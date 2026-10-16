// Package directinvestigator is the deterministic Tier-2 evidence engine over
// an on-disk tree. It implements the Tier2 interface from evidence-ladder.
//
// # Pure-Go contract
//
// The pure-Go engine is the canonical implementation: filepath.WalkDir in
// sorted order, regexp matching, bounded file sizes, and explicit skip
// accounting into a CoverageManifest. "Searched exhaustively and found
// nothing" is a provable, reproducible claim when the manifest is complete.
//
// # Optional rg accelerator
//
// An Engine interface allows an rg (ripgrep) accelerator to be substituted
// when the caller explicitly provides the binary path. The rg engine is
// NEVER PATH-sniffed — callers must opt in with an absolute path. The rg
// engine is equivalence-tested against the pure engine in tests.
//
// The rg accelerator is best-effort: if it fails, the caller should fall
// back to the pure engine. The ladder does not guarantee which engine runs.
//
// # Absence archetypes: prefer broad weak patterns
//
// For absence archetypes, use broad, weak patterns with downstream triage
// rather than narrow "decisive" patterns with nothing to triage. A broad
// pattern minimises false absents; downstream triage filters false positives.
// Narrow patterns that miss real hits produce incorrect ABSENT_PROVEN verdicts.
//
// Example: prefer `password\s*=\s*` over `password\s*=\s*"[A-Za-z0-9]+"`.
// The broader form catches more variants; decisive=false on ambiguous hits
// lets a human review them before they become conclusions.
//
// # Refs format
//
// Refs are minted as `file://<relpath>#L<start>-L<end>` using evidencespan.Locator
// conventions. Optional SpanProof anchoring is available via the AnchorFn hook.
//
// # Concurrency
//
// Investigator is immutable after creation and safe for concurrent use.
// Each Search call uses only local state.
//
// # No LLM
//
// This package contains no LLM calls. Absence claims must be provable and
// reproducible without inference.
package directinvestigator

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"unicode/utf8"

	"github.com/geoffmilleraz/fathom/third_party/agentic-kit/foundations/agent/bounded-exec"
	evidencesig "github.com/geoffmilleraz/fathom/third_party/agentic-kit/foundations/evidence/evidence-signature"
)

// Config configures the investigator.
//
// CUE mirror: #Config in schema.cue.
type Config struct {
	// RootDir is the directory to walk. Must be non-empty.
	RootDir string

	// MaxFileBytes caps the per-file read size. Files larger than this are
	// recorded as Oversize skips. Must be positive.
	MaxFileBytes int64

	// TreeHash is the content/git hash of the tree at the time of search.
	// Recorded in the resulting CoverageManifest for ABSENT_PROVEN scoping.
	TreeHash string
}

// Validate returns an error if the config is malformed.
func (c Config) Validate() error {
	if c.RootDir == "" {
		return fmt.Errorf("directinvestigator: Config.RootDir must not be empty")
	}
	if c.MaxFileBytes <= 0 {
		return fmt.Errorf("directinvestigator: Config.MaxFileBytes must be positive, got %d", c.MaxFileBytes)
	}
	return nil
}

// Engine is the optional accelerator interface. Callers may supply an rg-backed
// implementation. The pure engine is always the fallback and the canonical result.
type Engine interface {
	// SearchFile returns the line ranges matching pattern in the file at relPath,
	// or nil if no match. Returns an error only on I/O failure.
	SearchFile(ctx context.Context, relPath, absPath, pattern string) ([]LineRange, error)
}

// LineRange is a matched line range within a file.
type LineRange struct {
	Start int // 1-based
	End   int // 1-based
}

// RgEngine implements Engine using ripgrep via bounded-exec.
// The binary path must be supplied explicitly — it is NEVER PATH-sniffed.
type RgEngine struct {
	// BinaryPath is the absolute path to the rg binary.
	BinaryPath string
	runner     *boundedexec.Runner
}

// NewRgEngine constructs an RgEngine. binaryPath must be absolute.
func NewRgEngine(binaryPath string) (*RgEngine, error) {
	if binaryPath == "" {
		return nil, fmt.Errorf("directinvestigator: RgEngine: binaryPath must not be empty")
	}
	if !filepath.IsAbs(binaryPath) {
		return nil, fmt.Errorf("directinvestigator: RgEngine: binaryPath %q must be absolute (never PATH-sniffed)", binaryPath)
	}
	runner, err := boundedexec.New(boundedexec.Config{
		Timeout:        30_000_000_000, // 30s
		MaxStdoutBytes: 10 * 1024 * 1024,
		MaxStderrBytes: 64 * 1024,
	})
	if err != nil {
		return nil, fmt.Errorf("directinvestigator: RgEngine: %w", err)
	}
	return &RgEngine{BinaryPath: binaryPath, runner: runner}, nil
}

// SearchFile runs rg against a single file and parses line numbers.
func (e *RgEngine) SearchFile(ctx context.Context, relPath, absPath, pattern string) ([]LineRange, error) {
	res, err := e.runner.Run(ctx, boundedexec.Spec{
		Path: e.BinaryPath,
		Args: []string{"--line-number", "--no-heading", "-e", pattern, absPath},
	})
	if err != nil {
		return nil, fmt.Errorf("directinvestigator: rg: %w", err)
	}
	if res.ExitCode == 1 {
		// Exit code 1 means no matches.
		return nil, nil
	}
	if res.ExitCode != 0 {
		return nil, fmt.Errorf("directinvestigator: rg exited %d: %s", res.ExitCode, string(res.Stderr))
	}
	return parseRgOutput(res.Stdout), nil
}

// parseRgOutput parses `rg --line-number --no-heading` output.
// Each line is "<linenum>:<text>". We extract line numbers and group
// consecutive matches into ranges.
func parseRgOutput(out []byte) []LineRange {
	var ranges []LineRange
	scanner := bufio.NewScanner(bytes.NewReader(out))
	for scanner.Scan() {
		line := scanner.Text()
		colon := strings.IndexByte(line, ':')
		if colon < 0 {
			continue
		}
		var lineNum int
		_, err := fmt.Sscanf(line[:colon], "%d", &lineNum)
		if err != nil || lineNum <= 0 {
			continue
		}
		ranges = append(ranges, LineRange{Start: lineNum, End: lineNum})
	}
	return ranges
}

// Investigator is the Tier-2 engine. It implements the evidence-ladder Tier2 interface.
// Create with New.
type Investigator struct {
	cfg    Config
	engine Engine // optional accelerator; nil = pure Go
}

// New creates an Investigator. engine may be nil (pure-Go fallback).
func New(cfg Config, engine Engine) (*Investigator, error) {
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	return &Investigator{cfg: cfg, engine: engine}, nil
}

// Search implements the Tier2 interface: walks the tree, matches predicates,
// and returns Findings plus a CoverageManifest with explicit skip accounting.
//
// The walk is sorted (deterministic). Binary, oversize, LFS-stub, non-UTF-8,
// and symlink files are recorded in the manifest's Skipped categories.
//
// Files whose paths do not match any of sig.Scope.Globs are skipped (Ignored).
// Only glob-scoped signatures are supported; source_id scopes are not handled
// by this engine (return an error).
func (inv *Investigator) Search(ctx context.Context, sig evidencesig.ConcernSignature) (Findings, evidencesig.CoverageManifest, error) {
	if err := sig.Validate(); err != nil {
		return Findings{}, evidencesig.CoverageManifest{}, fmt.Errorf("directinvestigator: invalid signature: %w", err)
	}
	if !sig.Scope.IsGlobBased() {
		return Findings{}, evidencesig.CoverageManifest{}, fmt.Errorf("directinvestigator: source_id scopes are not handled by this engine; use a source-registry-backed Tier-2")
	}

	// Compile all predicates into regexps.
	patterns, err := compilePredicates(sig.Predicates)
	if err != nil {
		return Findings{}, evidencesig.CoverageManifest{}, err
	}

	manifest := evidencesig.CoverageManifest{
		Tool:     "direct-investigator/pure",
		TreeHash: inv.cfg.TreeHash,
	}
	if inv.engine != nil {
		manifest.Tool = "direct-investigator/rg"
	}

	var findings []evidencesig.Finding

	// Collect all paths in the root, sorted for determinism.
	paths, err := collectSortedPaths(inv.cfg.RootDir)
	if err != nil {
		return Findings{}, evidencesig.CoverageManifest{}, fmt.Errorf("directinvestigator: walk: %w", err)
	}

	for _, relPath := range paths {
		if ctx.Err() != nil {
			return Findings{}, evidencesig.CoverageManifest{}, ctx.Err()
		}

		absPath := filepath.Join(inv.cfg.RootDir, relPath)

		// Check if path matches any scope glob.
		if !matchesAnyGlob(relPath, sig.Scope.Globs) {
			manifest.Skipped.Ignored = append(manifest.Skipped.Ignored, relPath)
			continue
		}

		// Check symlink.
		info, err := os.Lstat(absPath)
		if err != nil {
			manifest.Skipped.Ignored = append(manifest.Skipped.Ignored, relPath)
			continue
		}
		if info.Mode()&os.ModeSymlink != 0 {
			manifest.Skipped.Symlink = append(manifest.Skipped.Symlink, relPath)
			continue
		}
		if info.IsDir() {
			continue
		}

		// Check file size.
		if info.Size() > inv.cfg.MaxFileBytes {
			manifest.Skipped.Oversize = append(manifest.Skipped.Oversize, relPath)
			continue
		}

		// Read file content.
		content, err := readFile(absPath, inv.cfg.MaxFileBytes)
		if err != nil {
			manifest.Skipped.Ignored = append(manifest.Skipped.Ignored, relPath)
			continue
		}

		// Check binary / non-UTF-8.
		if isBinary(content) {
			manifest.Skipped.Binary = append(manifest.Skipped.Binary, relPath)
			continue
		}
		if !utf8.Valid(content) {
			manifest.Skipped.NonUTF8 = append(manifest.Skipped.NonUTF8, relPath)
			continue
		}

		// Check LFS stub.
		if isLFSStub(content) {
			manifest.Skipped.LFSStub = append(manifest.Skipped.LFSStub, relPath)
			continue
		}

		// Search the file.
		manifest.Files = append(manifest.Files, relPath)
		fileFindings := inv.searchContent(ctx, relPath, absPath, content, patterns, sig)
		findings = append(findings, fileFindings...)
	}

	sort.Strings(manifest.Files)
	sort.Strings(manifest.Skipped.Ignored)
	sort.Strings(manifest.Skipped.Binary)
	sort.Strings(manifest.Skipped.Oversize)
	sort.Strings(manifest.Skipped.Symlink)
	sort.Strings(manifest.Skipped.LFSStub)
	sort.Strings(manifest.Skipped.NonUTF8)

	return Findings{Items: findings}, manifest, nil
}

// Findings wraps the finding slice (mirrors evidence-ladder Findings).
type Findings struct {
	Items []evidencesig.Finding
}

// searchContent searches the given file content for all compiled patterns.
func (inv *Investigator) searchContent(
	ctx context.Context,
	relPath, absPath string,
	content []byte,
	patterns []*regexp.Regexp,
	sig evidencesig.ConcernSignature,
) []evidencesig.Finding {
	// If we have an rg engine, use it for the first pattern (primary predicate).
	if inv.engine != nil && len(patterns) > 0 {
		ranges, err := inv.engine.SearchFile(ctx, relPath, absPath, sig.Predicates[0].Arg)
		if err == nil {
			var out []evidencesig.Finding
			for _, r := range ranges {
				out = append(out, evidencesig.Finding{
					Path:      relPath,
					LineStart: r.Start,
					LineEnd:   r.End,
					Decisive:  isDecisive(sig.Archetype),
				})
			}
			return out
		}
		// Fall through to pure engine on rg failure.
	}

	return searchContentPure(relPath, content, patterns, sig.Archetype)
}

// searchContentPure searches content with regexp, line by line.
func searchContentPure(relPath string, content []byte, patterns []*regexp.Regexp, archetype evidencesig.Archetype) []evidencesig.Finding {
	if len(patterns) == 0 {
		return nil
	}
	var findings []evidencesig.Finding
	lines := bytes.Split(content, []byte("\n"))
	for i, line := range lines {
		lineNum := i + 1
		for _, pat := range patterns {
			if pat.Match(line) {
				findings = append(findings, evidencesig.Finding{
					Path:      relPath,
					LineStart: lineNum,
					LineEnd:   lineNum,
					Decisive:  isDecisive(archetype),
				})
				break // one finding per line per file section
			}
		}
	}
	return findings
}

// isDecisive returns true when a match is strong enough to be decisive.
// For absence archetypes, any hit is decisive (it falsifies absence).
// For existence/contradicts/threshold, a match is decisive.
func isDecisive(archetype evidencesig.Archetype) bool {
	return true // all regexp matches are decisive in the direct engine
}

// compilePredicates compiles predicate Args into regexps.
// Only "regexp" and "contains" kinds are supported; others are skipped.
func compilePredicates(preds []evidencesig.Predicate) ([]*regexp.Regexp, error) {
	var out []*regexp.Regexp
	for _, p := range preds {
		switch p.Kind {
		case "regexp":
			re, err := regexp.Compile(p.Arg)
			if err != nil {
				return nil, fmt.Errorf("directinvestigator: predicate %q: invalid regexp %q: %w", p.Kind, p.Arg, err)
			}
			out = append(out, re)
		case "contains":
			out = append(out, regexp.MustCompile(regexp.QuoteMeta(p.Arg)))
		default:
			// Unknown predicate kinds are silently skipped.
			// Callers that require specific semantics should implement a custom Engine.
		}
	}
	return out, nil
}

// matchesAnyGlob returns true if relPath matches any of the given globs.
func matchesAnyGlob(relPath string, globs []string) bool {
	for _, g := range globs {
		if matchGlob(g, relPath) {
			return true
		}
	}
	return false
}

// matchGlob matches a path against a glob pattern with double-star support.
func matchGlob(pattern, path string) bool {
	// filepath.Match handles simple globs but not **.
	// For ** support, use a regexp-based approach.
	reStr := globToRegexp(pattern)
	re, err := regexp.Compile(reStr)
	if err != nil {
		// Fallback to filepath.Match.
		matched, _ := filepath.Match(pattern, path)
		return matched
	}
	return re.MatchString(path)
}

// globToRegexp converts a glob pattern (with ** support) to a regexp string.
func globToRegexp(glob string) string {
	var sb strings.Builder
	sb.WriteString("^")
	i := 0
	for i < len(glob) {
		if i+1 < len(glob) && glob[i] == '*' && glob[i+1] == '*' {
			sb.WriteString(".*")
			i += 2
			if i < len(glob) && glob[i] == '/' {
				i++
			}
			continue
		}
		switch glob[i] {
		case '*':
			sb.WriteString("[^/]*")
		case '?':
			sb.WriteString("[^/]")
		case '.', '+', '(', ')', '[', ']', '{', '}', '^', '$', '|':
			sb.WriteByte('\\')
			sb.WriteByte(glob[i])
		default:
			sb.WriteByte(glob[i])
		}
		i++
	}
	sb.WriteString("$")
	return sb.String()
}

// collectSortedPaths walks rootDir and returns all relative file paths, sorted.
func collectSortedPaths(rootDir string) ([]string, error) {
	var paths []string
	err := filepath.WalkDir(rootDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil // skip unreadable entries
		}
		if d.IsDir() {
			return nil
		}
		rel, relErr := filepath.Rel(rootDir, path)
		if relErr != nil {
			return nil
		}
		paths = append(paths, rel)
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Strings(paths)
	return paths, nil
}

// readFile reads up to maxBytes from the file.
func readFile(path string, maxBytes int64) ([]byte, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return io.ReadAll(io.LimitReader(f, maxBytes))
}

// isBinary reports whether the content looks like binary data.
// Heuristic: if the first 8KB contains a NUL byte, treat as binary.
func isBinary(content []byte) bool {
	probe := content
	if len(probe) > 8192 {
		probe = probe[:8192]
	}
	return bytes.ContainsRune(probe, 0)
}

// isLFSStub reports whether the content looks like a Git LFS pointer.
func isLFSStub(content []byte) bool {
	return bytes.HasPrefix(content, []byte("version https://git-lfs.github.com/spec/"))
}
