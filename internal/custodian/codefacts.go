package custodian

// codefacts.go — VB-28 (docs/plans/verbs.md, wave H; vision 29 §2 thesis
// 3; ADR-035 decision 7): the custodian audits code-fact-shaped sentences
// in every auto-loaded file, not only the fathom stanza — the R8 gap
// named in ADR-035's context ("the harness's own memory carried a code
// claim the custodian never saw"). Behind `fathom config audit
// --code-facts`, default OFF until the labelled-set precision bar (VB-29,
// M14) holds: precision first, in doubt classify preference (§ below).
//
// Four classes, never a silent fifth:
//   - anchored-fresh:          a path/symbol anchor resolves at HEAD and
//                               any stated count still holds
//   - anchored-drifted:        it resolved once but does not hold now —
//                               the receipt names what the tree says
//   - unanchorable-code-claim: shaped like a code fact (a trigger phrase,
//                               a bare count, or a check this audit could
//                               not complete) but names nothing this audit
//                               can confirm
//   - preference:               not a code fact — an instruction, a
//                               principle, an imperative
//
// "At HEAD" here means the working tree at root, read directly (git
// blob access is not needed: a checked-out worktree already IS HEAD for
// a clean repo, and a dirty one is the more honest thing to check
// against — an agent auditing its own instructions cares what the files
// say right now).
//
// ADR-027's lesson applies unchanged: every read goes through the kit's
// read-manifest (agentic-kit foundations/telemetry/read-manifest, the
// same mechanism internal/intent/audit.go and internal/wiring use), so an
// unreadable file is NAMED and every count derived from an incomplete
// read is rendered as a lower bound (readcoverage.RenderCount) — never a
// bare total that reads as a clean bill over a directory this audit could
// not even list.
import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	readmanifest "github.com/geoffmilleraz/agentic-kit/foundations/telemetry/read-manifest"
)

// CodeFactClass is one of the four audit outcomes for a sentence.
type CodeFactClass string

const (
	ClassAnchoredFresh   CodeFactClass = "anchored-fresh"
	ClassAnchoredDrifted CodeFactClass = "anchored-drifted"
	ClassUnanchorable    CodeFactClass = "unanchorable-code-claim"
	ClassPreference      CodeFactClass = "preference"
)

// Sentence is one classifiable unit: a line (or line fragment) from an
// auto-loaded file, with its provenance.
type Sentence struct {
	File string `json:"file"` // display path: repo-relative, or "~/..." for auto-memory
	Line int    `json:"line"`
	Text string `json:"text"`
}

// CodeFactFinding is one sentence's classification.
type CodeFactFinding struct {
	File     string        `json:"file"`
	Line     int           `json:"line"`
	Class    CodeFactClass `json:"class"`
	Sentence string        `json:"sentence"` // excerpt, capped
	Receipt  string        `json:"receipt,omitempty"`
}

// AutoLoadedFile is one file a coding harness loads into context
// automatically, discovered by AutoLoadedFiles.
type AutoLoadedFile struct {
	// Path is repo-relative for files inside root, or a "~/…" display
	// path for harness-global files (Claude Code's per-project
	// auto-memory lives under the user's home directory, outside the
	// repo) — never the real absolute path, which would put the
	// operator's home directory into a report (ADR-011).
	Path string
	Kind string // agent-config | cursor-rule | windsurf-rule | memory-bank | claude-auto-memory
}

const excerptCap = 160

// AuditCodeFacts is the `--code-facts` entry point: discover every
// auto-loaded file for root, split each into sentences, and classify
// them. Every read — each auto-loaded file's content, and each
// directory this walks to find them — goes through a *readmanifest.Set,
// so an unreadable file or directory is NAMED in the returned coverage,
// never dropped silently from the count (ADR-027); the caller renders
// any count derived from findings through readcoverage.RenderCount so an
// incomplete read is shown as a lower bound, not a clean bill.
//
// home is the harness home directory (os.UserHomeDir() in production,
// injectable for tests); "" skips Claude Code auto-memory discovery
// rather than erroring.
func AuditCodeFacts(root, home string) (findings []CodeFactFinding, coverage readmanifest.Manifest, err error) {
	reads := readmanifest.NewSet()
	files, ferr := AutoLoadedFiles(root, home, reads)
	if ferr != nil {
		return nil, reads.Manifest(), ferr
	}
	var sentences []Sentence
	for _, f := range files {
		abs := f.Path
		if !filepath.IsAbs(abs) {
			abs = filepath.Join(root, f.Path)
		}
		data, ok := readTracked(reads, f.Path, abs)
		if !ok {
			continue
		}
		for _, rs := range splitSentences(string(data)) {
			sentences = append(sentences, Sentence{File: f.Path, Line: rs.Line, Text: rs.Text})
		}
	}
	return auditCodeFactSentences(root, sentences), reads.Manifest(), nil
}

// readTracked reads absPath and records the outcome under displayPath —
// the path a finding would show — rather than letting the manifest
// derive its own key, since a Claude Code auto-memory path is outside
// root and must stay in its anonymized "~/…" form (never the operator's
// real home directory) even when the read fails.
func readTracked(reads *readmanifest.Set, displayPath, absPath string) ([]byte, bool) {
	data, err := os.ReadFile(absPath)
	switch {
	case err == nil:
		reads.Record(displayPath, readmanifest.Read(len(data)))
		return data, true
	case errors.Is(err, fs.ErrNotExist):
		reads.Record(displayPath, readmanifest.Absent())
		return nil, false
	default:
		reads.Record(displayPath, readmanifest.Unreadable(err))
		return nil, false
	}
}

// auditCodeFactSentences classifies each given sentence against root —
// the anchor this row builds (VB-28). Split out from AuditCodeFacts so
// the classifier's precision/recall can be measured directly against a
// labelled set (codefacts_test.go), independent of file discovery and
// sentence splitting.
func auditCodeFactSentences(root string, sentences []Sentence) []CodeFactFinding {
	out := make([]CodeFactFinding, 0, len(sentences))
	for _, s := range sentences {
		class, receipt := classifySentence(root, s.File, s.Text)
		out = append(out, CodeFactFinding{
			File:     s.File,
			Line:     s.Line,
			Class:    class,
			Sentence: excerpt(s.Text),
			Receipt:  receipt,
		})
	}
	return out
}

// excerpt trims and caps a sentence at excerptCap RUNES (not bytes) so a
// multibyte character is never split in half.
func excerpt(s string) string {
	s = strings.TrimSpace(s)
	r := []rune(s)
	if len(r) <= excerptCap {
		return s
	}
	return string(r[:excerptCap-1]) + "…"
}

// AutoLoadedFiles discovers every file a coding harness auto-loads for
// root, sorted for determinism: the agent-config corpus (CLAUDE.md /
// AGENTS.md at any depth — AgentConfigFiles, which already covers
// `.claude/**/CLAUDE.md`), Cursor project rules (`.cursor/rules/*`),
// Windsurf rules (`.windsurf/rules/*`), the Cline memory bank
// (`memory-bank/*.md`), and — when home is non-empty — Claude Code's
// per-project auto-memory (`~/.claude/projects/<encoded-root>/memory/*.md`).
//
// Each directory is listed with os.ReadDir through reads: an absent
// directory (the common case — most repos have no `.cursor/`) is not a
// failure, but a directory that exists and cannot be LISTED (permission
// denied) is recorded unreadable rather than silently yielding zero
// files, which would read as "no Cursor rules" instead of "could not
// check for Cursor rules".
func AutoLoadedFiles(root, home string, reads *readmanifest.Set) ([]AutoLoadedFile, error) {
	var out []AutoLoadedFile

	agentFiles, err := AgentConfigFiles(root)
	if err != nil {
		return nil, err
	}
	for _, f := range agentFiles {
		out = append(out, AutoLoadedFile{Path: f, Kind: "agent-config"})
	}

	dirs := []struct {
		rel, kind string
		match     func(name string) bool
	}{
		{".cursor/rules", "cursor-rule", anyFile},
		{".windsurf/rules", "windsurf-rule", anyFile},
		{"memory-bank", "memory-bank", isMarkdownFile},
	}
	for _, d := range dirs {
		dirAbs := filepath.Join(root, filepath.FromSlash(d.rel))
		for _, name := range listDirFiles(reads, dirAbs, d.rel, d.match) {
			out = append(out, AutoLoadedFile{Path: filepath.ToSlash(filepath.Join(d.rel, name)), Kind: d.kind})
		}
	}

	if home != "" {
		memDir := ClaudeCodeMemoryDir(root, home)
		memDisplay := displayHomePath(home, memDir)
		for _, name := range listDirFiles(reads, memDir, memDisplay, isMarkdownFile) {
			out = append(out, AutoLoadedFile{Path: filepath.ToSlash(filepath.Join(memDisplay, name)), Kind: "claude-auto-memory"})
		}
	}

	return out, nil
}

func anyFile(string) bool             { return true }
func isMarkdownFile(name string) bool { return strings.HasSuffix(name, ".md") }

// listDirFiles lists dirAbs's regular files matching match, sorted by
// name. A directory that genuinely does not exist yields no files and no
// failure — most repos have no `.cursor/`, `.windsurf/`, or
// `memory-bank/`. Any OTHER read error (permission denied, not a
// directory) is recorded against dirDisplay in reads as unreadable —
// this is the fix for the failure class where an unreadable directory
// silently looked identical to an absent one.
func listDirFiles(reads *readmanifest.Set, dirAbs, dirDisplay string, match func(name string) bool) []string {
	entries, err := os.ReadDir(dirAbs)
	switch {
	case err == nil:
		var names []string
		for _, e := range entries {
			if e.IsDir() {
				continue
			}
			if match(e.Name()) {
				names = append(names, e.Name())
			}
		}
		sort.Strings(names)
		return names
	case errors.Is(err, fs.ErrNotExist):
		return nil
	default:
		reads.Record(dirDisplay, readmanifest.Unreadable(err))
		return nil
	}
}

// ClaudeCodeMemoryDir resolves the per-project auto-memory directory the
// way Claude Code does: the project's absolute path with every "/"
// replaced by "-" (observed convention).
func ClaudeCodeMemoryDir(root, home string) string {
	abs, err := filepath.Abs(root)
	if err != nil {
		abs = root
	}
	encoded := strings.ReplaceAll(filepath.ToSlash(abs), "/", "-")
	return filepath.Join(home, ".claude", "projects", encoded, "memory")
}

func displayHomePath(home, abs string) string {
	if rel, err := filepath.Rel(home, abs); err == nil && !strings.HasPrefix(rel, "..") {
		return filepath.ToSlash(filepath.Join("~", rel))
	}
	return abs
}

// --- sentence splitting -----------------------------------------------

type rawSentence struct {
	Line int
	Text string
}

var (
	fenceRe         = regexp.MustCompile("^```")
	sentenceSplitRe = regexp.MustCompile(`([.!?])\s+`)
	// abbreviationEndRe matches a candidate sentence fragment ending in a
	// known abbreviation ("e.g.", "i.e.", …) — a period there is not a
	// sentence boundary, so the split folds into the next fragment.
	abbreviationEndRe = regexp.MustCompile(`(?i)\b(?:e\.g|i\.e|etc|approx|cf)\.$`)
	mdLeadRe          = regexp.MustCompile(`^(\s*[-*]\s+|\s*\|\s*|#+\s+|\d+\.\s+)+`)
)

// splitSentences breaks file content into (line, sentence-fragment)
// pairs: markdown fences are skipped, blank lines dropped, leading list /
// heading / table markup stripped, and each remaining line further split
// into sentences on `. ` / `! ` / `? ` boundaries — guarded against the
// common abbreviations ("e.g. ", "i.e. ") that would otherwise fragment a
// path like `CLAUDE.md` is already excluded, since that period is never
// followed by whitespace.
func splitSentences(text string) []rawSentence {
	var out []rawSentence
	inFence := false
	for i, line := range strings.Split(text, "\n") {
		trimmed := strings.TrimSpace(line)
		if fenceRe.MatchString(trimmed) {
			inFence = !inFence
			continue
		}
		if inFence || trimmed == "" {
			continue
		}
		content := mdLeadRe.ReplaceAllString(trimmed, "")
		content = strings.TrimSpace(content)
		if content == "" {
			continue
		}
		for _, sent := range splitLineSentences(content) {
			sent = strings.TrimSpace(sent)
			if sent != "" {
				out = append(out, rawSentence{Line: i + 1, Text: sent})
			}
		}
	}
	return out
}

func splitLineSentences(line string) []string {
	locs := sentenceSplitRe.FindAllStringSubmatchIndex(line, -1)
	if len(locs) == 0 {
		return []string{line}
	}
	var out []string
	start := 0
	for _, loc := range locs {
		// loc[2]:loc[3] is the punctuation char; loc[1] is where the
		// following whitespace run ends.
		candidate := line[start:loc[3]]
		if abbreviationEndRe.MatchString(candidate) {
			continue // not a sentence boundary — fold into the next split
		}
		out = append(out, line[start:loc[3]])
		start = loc[1]
	}
	out = append(out, line[start:])
	return out
}
