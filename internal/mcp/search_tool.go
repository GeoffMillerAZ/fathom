// Package mcp — search: grep with a receipt (RC-24, docs/plans/receipts.md;
// ships SB-48, docs/plans/strengths-bench.md).
//
// search is deliberately NOT a smarter grep: it runs ripgrep against the
// WORKING TREE when `rg` is on PATH, falling back to an equivalent Go
// regexp walk with an identical output shape when it is not — for the
// recall, exactness and freshness only a live read gives (the index may
// not have re-ingested uncommitted edits yet, vision 28 §3.3). What makes
// it more than grep is the receipt: each hit is annotated, from the index,
// with the enclosing symbol's name and kind, `test`/`generated`/`vendored`
// flags when they apply, and an `ident`/`comment`/`string` classification
// — so the Read that followed 27% of Greps in the home bench (vision 28
// F8) is not needed.
//
// Receipt scope, honestly: the enclosing symbol (sym/kind) requires a
// `service` (it is an index lookup — backend.ListAllSymbols); flags and
// classification are pure filesystem/lexical facts and are populated even
// for a bare `root` call with no backend at all. Classification is
// same-line lexical evidence (a preceding line-comment token, or a
// same-line quoted span) for a small set of well-known languages — it
// never guesses across a block comment or a multi-line string it cannot
// see the rest of; the schema's typed edges carry no per-occurrence line
// position to classify against directly, so this is the closest
// unambiguous substitute (skip, never guess, is the constraint SB-48
// actually asks for).
package mcp

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/geoffmilleraz/fathom/internal/connector"
	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/filter"
)

// searchDefaultLimit bounds the default hit count (SB-48: rendered ≤ ~120B
// a hit, so 50 stays well inside the 16KB shared budget with room for
// receipts and context).
const searchDefaultLimit = 50

// searchMaxFileBytes mirrors fusion.MaxExtractSize's 2MB extraction cap
// (kept as a local literal rather than an import: this package does not
// otherwise depend on internal/fusion, and the number is a shared
// convention, not a coupled constant) — a file larger than this is never
// grepped by the Go-regexp fallback (ripgrep has no such limit and needs
// none; it streams).
const searchMaxFileBytes = 2 * 1024 * 1024

// searchHitMaxBytes is the per-hit wire budget (SB-48: "~120B a hit" —
// looser than references' 100B/60-char occurrence since a search hit also
// carries the receipt fields).
const searchHitMaxBytes = 120

// searchTextMaxChars bounds one rendered line (hit or context) before the
// byte-fit loop, mirroring references' occTextMaxChars.
const searchTextMaxChars = 80

// searchHardScanCap bounds the Go-regexp fallback's total match count (a
// safety valve against a wide-open pattern over a huge tree with no rg
// installed) — ripgrep needs no such cap; it is fast enough to finish and
// report an honest total itself.
const searchHardScanCap = 20000

// searchExtraSkipDirs are directory names search always prunes beyond
// connector.IsSkippedDir's ingest-time set (vendor, .git, testdata,
// generated, node_modules, .idea, .vscode, .claude): "workspace" is this
// repo's own build-artifact convention (docs/usage.md), reachable only
// through .fathomignore at ingest time, but search reads the raw
// filesystem and has no per-service ignore file to fall back on when
// called with a bare `root` — so it is pruned unconditionally here,
// exactly as the RC-24 spec requires ("vendored and workspace/ trees
// excluded by default").
var searchExtraSkipDirs = map[string]bool{"workspace": true}

func searchSkipDir(name string) bool {
	return connector.IsSkippedDir(name) || searchExtraSkipDirs[name]
}

// searchVendorSegments are path segments search flags "vendored" —
// broader than searchSkipDir's "vendor"/"node_modules" (which are pruned
// from every walk) because a caller-given `root` may point AT or INSIDE
// one of these trees on purpose, in which case pruning does not apply
// (root itself is never pruned) but the flag still should.
var searchVendorSegments = map[string]bool{"vendor": true, "node_modules": true, "third_party": true}

// searchHit is one search result, rendered compact for the wire (SB-48,
// mirroring references' SB-1 / find_callers' SB-47): a combined
// "file:line" location, the matched line trimmed to fit, and the receipt
// — sym/kind (enclosing symbol, index-backed), flags (comma-joined
// test/generated/vendored), and class (ident/comment/string) — each
// omitted rather than guessed when it cannot be determined.
type searchHit struct {
	Loc   string   `json:"loc"`
	Text  string   `json:"text"`
	Sym   string   `json:"sym,omitempty"`
	Kind  string   `json:"kind,omitempty"`
	Flags string   `json:"flags,omitempty"`
	Class string   `json:"class,omitempty"`
	Ctx   []string `json:"ctx,omitempty"`
}

// searchResult is search's top-level answer.
type searchResult struct {
	Hits   []searchHit `json:"hits"`
	Engine string      `json:"engine"`
	Elided *elided     `json:"elided,omitempty"`
}

// searchRawHit is one match before receipts are attached: what either
// engine (ripgrep or the Go-regexp walk) produces, in the same shape.
type searchRawHit struct {
	File   string // repo/root-relative, slash-form
	Line   int    // 1-based
	Text   string
	Before []string
	After  []string
}

// registerSearch wires the "search" tool into the investigation group.
func registerSearch(s *Server, backend QueryInterface) {
	s.registerTool(toolDescriptor{
		Name:        "search",
		Description: "Grep with a receipt — the hit and the symbol it sits in, so the follow-up read is not needed. Runs ripgrep against the WORKING TREE when it is on PATH (falls back to an equivalent Go-regexp walk with an identical output shape otherwise) — for the recall, exactness and freshness a live read gives that the index, not yet re-ingested, may not (vision 28 §3.3). Each hit renders {loc: \"file:line\", text} plus a receipt: sym/kind (the enclosing func/method/type/const/var, from the index — requires service) and, from the filesystem/line itself (no service needed): flags (comma-joined test/generated/vendored when they apply) and class (ident/comment/string from same-line lexical evidence, omitted rather than guessed when a block comment or multi-line string on an earlier line makes it undecidable). Vendored and workspace/ trees are excluded by default. Hits are sorted by path then line; over limit (default 50) or the 16KB response budget, elided names how many more exist and how to get them. Give exactly one of service (searches its indexed corpus root; enables sym/kind) or root (an explicit directory; sym/kind omitted, flags/class still render). Reach for it when you have a literal string, a config value, or a log line to find; for where/who-calls questions use find_symbol/references instead — this result IS the answer for a text search, never a reason to re-grep the same file yourself. Required: pattern.",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"pattern": map[string]any{
					"type":        "string",
					"description": "Regular expression to search for (Go regexp syntax — must compile; the same pattern text is handed to ripgrep when it runs the search).",
				},
				"service": map[string]any{
					"type":        "string",
					"description": "Service / repo identifier — searches its indexed corpus root and enables sym/kind receipts. Give exactly one of service or root.",
				},
				"root": map[string]any{
					"type":        "string",
					"description": "Absolute filesystem directory to search directly (no index lookup, so sym/kind are omitted; flags/class still render). Give exactly one of service or root.",
				},
				"path": map[string]any{
					"type":        "string",
					"description": "Optional glob (gitignore-style, e.g. \"internal/mcp/**/*.go\") narrowing which files are searched.",
				},
				"case": map[string]any{
					"type":        "boolean",
					"description": "Case-sensitive match. Default true (grep/ripgrep's own default); false for case-insensitive.",
				},
				"limit": map[string]any{
					"type":        "integer",
					"description": "Maximum hits returned (default 50).",
				},
				"context": map[string]any{
					"type":        "integer",
					"description": "Lines of context before and after each hit (default 0).",
				},
			},
			"required": []string{"pattern"},
		},
		group: GroupInvestigation,
	}, func(ctx context.Context, args json.RawMessage) (any, *rpcError) {
		var p struct {
			Pattern string `json:"pattern"`
			Service string `json:"service"`
			Root    string `json:"root"`
			Path    string `json:"path"`
			Case    *bool  `json:"case"`
			Limit   int    `json:"limit"`
			Context int    `json:"context"`
		}
		if err := json.Unmarshal(args, &p); err != nil {
			return nil, &rpcError{Code: ErrCodeParams, Message: "search: invalid args"}
		}
		if p.Pattern == "" {
			return nil, &rpcError{Code: ErrCodeParams, Message: "search: pattern required"}
		}
		if (p.Service == "") == (p.Root == "") {
			return nil, &rpcError{Code: ErrCodeParams, Message: "search: give exactly one of service or root"}
		}
		caseSensitive := true
		if p.Case != nil {
			caseSensitive = *p.Case
		}
		reSrc := p.Pattern
		if !caseSensitive {
			reSrc = "(?i)" + reSrc
		}
		re, reErr := regexp.Compile(reSrc)
		if reErr != nil {
			return nil, &rpcError{Code: ErrCodeParams, Message: "search: invalid pattern: " + reErr.Error()}
		}
		if p.Path != "" {
			// Fail fast on an unusable glob rather than silently matching
			// nothing (filter.New never errors, so validate shape here:
			// an empty compiled matcher from a garbage pattern is
			// indistinguishable from a legitimate "matches nothing"
			// glob, so this is a best-effort shape check only).
			_ = filter.New([]string{p.Path})
		}

		root := p.Root
		var byFile map[string][]core.SymbolNode
		var ignore *filter.Matcher
		var ignorePatterns []string
		if p.Service != "" {
			if backend == nil {
				return nil, &rpcError{Code: ErrCodeInternal, Message: "no query backend configured"}
			}
			r, err := resolveCorpusRoot(ctx, backend, p.Service)
			if err != nil {
				return nil, &rpcError{Code: ErrCodeInternal, Message: err.Error()}
			}
			root = r
			if nodes, err := backend.ListAllSymbols(ctx, p.Service); err == nil {
				byFile = groupSymbolsByFile(nodes)
			}
			// CRITICAL (V-review): a `service` search must mirror the
			// INDEXED corpus exactly as read_source does — the same
			// scoped .fathomignore loader, the same fail-closed posture
			// (readsource.go's notIndexedReason, born of the purged-
			// credential incident). Without this, a file ingest and
			// read_source both refuse is still served here: search reads
			// the raw filesystem and had no per-service exclusion at all.
			// An error loading the exclusions REFUSES the search rather
			// than searching without them — a broken exclusion mechanism
			// must not fail toward disclosure.
			ig, pats, _, ierr := connector.LoadIgnoreScopedForWithPatterns(root, p.Service)
			if ierr != nil {
				return nil, &rpcError{Code: ErrCodeInternal, Message: fmt.Sprintf("search: refusing to search service %q: its exclusions could not be loaded (%v) — searching without them could disclose purged or excluded content", p.Service, ierr)}
			}
			ignore, ignorePatterns = ig, pats
		} else {
			info, err := os.Stat(root)
			if err != nil || !info.IsDir() {
				return nil, &rpcError{Code: ErrCodeParams, Message: fmt.Sprintf("search: root %q is not a readable directory", root)}
			}
		}

		limit := p.Limit
		if limit <= 0 {
			limit = searchDefaultLimit
		}
		contextN := p.Context
		if contextN < 0 {
			contextN = 0
		}

		rawHits, engine, err := searchRun(ctx, root, p.Pattern, p.Path, caseSensitive, contextN, ignore, ignorePatterns)
		if err != nil {
			return nil, &rpcError{Code: ErrCodeInternal, Message: "search: " + err.Error()}
		}
		sort.Slice(rawHits, func(i, j int) bool {
			if rawHits[i].File != rawHits[j].File {
				return rawHits[i].File < rawHits[j].File
			}
			return rawHits[i].Line < rawHits[j].Line
		})

		total := len(rawHits)
		limited := rawHits
		if len(limited) > limit {
			limited = limited[:limit]
		}
		flagsCache := map[string][]string{}
		linesCache := map[string][]string{}
		hits := make([]searchHit, 0, len(limited))
		for _, rh := range limited {
			hits = append(hits, renderSearchHit(rh, re, byFile, flagsCache, linesCache, root))
		}
		kept, truncated := fitToBudget(hits, 0)
		var el *elided
		if truncated || total > len(kept) {
			el = elisionFor(len(kept), total, "path, then line", "narrow path or pattern")
		}
		return searchResult{Hits: kept, Engine: engine, Elided: el}, nil
	})
}

// searchRun runs ripgrep when it is on PATH, falling back to searchWalk
// (an equivalent Go-regexp walk, identical output shape) on any rg
// plumbing failure or absence — the fallback promise the RC-24 spec makes.
// ignore/ignorePatterns are the SAME service-scoped .fathomignore load
// (connector.LoadIgnoreScopedForWithPatterns) applied to both engines:
// ignore (the compiled Matcher) drives searchWalk's own tree walk;
// ignorePatterns (the identical raw globs) become ripgrep's own `-g`
// excludes, so a `service` call never serves what ingest/read_source
// would refuse, regardless of which engine answers it. Both nil for a
// bare `root` call, which has no per-service ignore file to load.
func searchRun(ctx context.Context, root, pattern, pathGlob string, caseSensitive bool, contextN int, ignore *filter.Matcher, ignorePatterns []string) ([]searchRawHit, string, error) {
	if _, err := exec.LookPath("rg"); err == nil {
		if hits, err := searchRipgrep(ctx, root, pattern, pathGlob, caseSensitive, contextN, ignorePatterns); err == nil {
			return hits, "ripgrep", nil
		}
		// Any rg plumbing failure (an unexpected build's flag set, a
		// transient exec error) falls through to the Go walk rather than
		// surfacing an engine implementation detail as a tool error.
	}
	hits, err := searchWalk(root, pattern, pathGlob, caseSensitive, contextN, ignore)
	return hits, "regexp", err
}

// ignorePatternToRipgrepGlob converts one raw .fathomignore line (gitignore
// syntax — the exact same line filter.New compiles into ignore's Matcher)
// into the equivalent ripgrep -g glob. ripgrep's glob sense is INVERTED
// from gitignore's: a bare glob means "search files matching this", so an
// ignore-file EXCLUDE line needs a "!" added; a gitignore RE-INCLUDE line
// ("!pattern") becomes a plain rg include glob (bang stripped) so a
// project's own negation still overrides an earlier exclude, exactly as
// Matcher's last-match-wins does when both are handed the same lines in
// the same order. "" for a blank or comment line — not a pattern at all.
func ignorePatternToRipgrepGlob(line string) string {
	t := strings.TrimSpace(line)
	if t == "" || strings.HasPrefix(t, "#") {
		return ""
	}
	if strings.HasPrefix(t, "!") {
		return t[1:]
	}
	return "!" + t
}

// searchRipgrep shells out to rg. Exit code 1 (no matches) is success with
// zero hits; any other non-zero exit is a real error.
func searchRipgrep(ctx context.Context, root, pattern, pathGlob string, caseSensitive bool, contextN int, ignorePatterns []string) ([]searchRawHit, error) {
	args := []string{
		"--line-number", "--no-heading", "--with-filename", "--color=never", "--sort", "path",
		"-g", "!vendor/**", "-g", "!node_modules/**", "-g", "!workspace/**",
		// MINOR (V-review): searchWalk already refuses a .env-pattern
		// basename via isEnvPatternFile — the ripgrep path had no
		// equivalent, so a secrets-shaped file was servable through this
		// engine alone depending on which one answered a given call.
		"-g", "!*.env", "-g", "!.env", "-g", "!.env.*",
	}
	for _, p := range ignorePatterns {
		if g := ignorePatternToRipgrepGlob(p); g != "" {
			args = append(args, "-g", g)
		}
	}
	if !caseSensitive {
		args = append(args, "-i")
	}
	if contextN > 0 {
		args = append(args, "-C", strconv.Itoa(contextN))
	}
	if pathGlob != "" {
		args = append(args, "-g", pathGlob)
	}
	args = append(args, "--", pattern, ".")
	cmd := exec.CommandContext(ctx, "rg", args...)
	cmd.Dir = root
	var out, errBuf bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &errBuf
	if err := cmd.Run(); err != nil {
		if ee, ok := err.(*exec.ExitError); ok && ee.ExitCode() == 1 {
			return nil, nil
		}
		return nil, fmt.Errorf("ripgrep: %v: %s", err, strings.TrimSpace(errBuf.String()))
	}
	return parseRipgrepOutput(out.String(), contextN), nil
}

var (
	rgMatchLineRe   = regexp.MustCompile(`^(.*?):(\d+):(.*)$`)
	rgContextLineRe = regexp.MustCompile(`^(.*?)-(\d+)-(.*)$`)
)

// parseRipgrepOutput parses rg's plain-text output (--line-number
// --no-heading --with-filename), grouping -C context lines onto the hit
// they precede/follow: the first contextN context lines after a match
// become its After; earlier lines (from the previous group, or leading a
// file) are buffered as the NEXT match's Before, bounded to the last
// contextN lines. A "--" line (rg's between-groups separator) resets both.
func parseRipgrepOutput(raw string, contextN int) []searchRawHit {
	lines := strings.Split(strings.TrimRight(raw, "\n"), "\n")
	var hits []searchRawHit
	var beforeBuf []string
	openIdx := -1
	afterCount := 0
	reset := func() { openIdx = -1; afterCount = 0 }
	for _, ln := range lines {
		if ln == "" {
			continue
		}
		if ln == "--" {
			beforeBuf = nil
			reset()
			continue
		}
		if m := rgMatchLineRe.FindStringSubmatch(ln); m != nil {
			lineNo, _ := strconv.Atoi(m[2])
			// rg emits "./pkg/a.go" for the "." target this package always
			// passes; strip the prefix so both engines agree on the same
			// repo/root-relative, slash-form path.
			file := strings.TrimPrefix(filepath.ToSlash(m[1]), "./")
			h := searchRawHit{File: file, Line: lineNo, Text: m[3]}
			if len(beforeBuf) > 0 {
				h.Before = append([]string(nil), beforeBuf...)
			}
			hits = append(hits, h)
			beforeBuf = nil
			openIdx = len(hits) - 1
			afterCount = 0
			continue
		}
		if contextN > 0 {
			if m := rgContextLineRe.FindStringSubmatch(ln); m != nil {
				text := m[3]
				if openIdx >= 0 && afterCount < contextN {
					hits[openIdx].After = append(hits[openIdx].After, text)
					afterCount++
					if afterCount >= contextN {
						reset()
					}
					continue
				}
				beforeBuf = append(beforeBuf, text)
				if len(beforeBuf) > contextN {
					beforeBuf = beforeBuf[len(beforeBuf)-contextN:]
				}
				continue
			}
		}
	}
	return hits
}

// searchIgnoreMatch reports whether ignore excludes path — tested
// directly, then against every ancestor directory with a trailing slash
// appended (mirroring notIndexedReason's ancestor walk, readsource.go, so
// search's own mirror of the INDEXED corpus stays consistent with
// read_source's): a dir-only .fathomignore pattern ("workspace/") needs
// that trailing slash to match the directory segment, not a same-named
// file. A caller testing a DIRECTORY node itself should pass path with
// its own trailing slash already appended (nil ignore never matches —
// the bare `root` call path, which has no per-service ignore to load).
func searchIgnoreMatch(ignore *filter.Matcher, path string) bool {
	if ignore == nil {
		return false
	}
	if ignore.Match(path) {
		return true
	}
	trimmed := strings.Trim(strings.TrimSuffix(path, "/"), "/")
	if trimmed == "" {
		return false
	}
	segs := strings.Split(trimmed, "/")
	for i := 1; i < len(segs); i++ {
		if ignore.Match(strings.Join(segs[:i], "/") + "/") {
			return true
		}
	}
	return false
}

// searchWalk is the Go-regexp fallback: same exclusions, same output
// shape, used when rg is not on PATH. ignore (nil for a bare `root` call)
// is the service-scoped .fathomignore Matcher search's caller loaded via
// connector.LoadIgnoreScopedForWithPatterns — see searchRun's doc comment.
func searchWalk(root, pattern, pathGlob string, caseSensitive bool, contextN int, ignore *filter.Matcher) ([]searchRawHit, error) {
	reSrc := pattern
	if !caseSensitive {
		reSrc = "(?i)" + reSrc
	}
	re, err := regexp.Compile(reSrc)
	if err != nil {
		return nil, err
	}
	var glob *filter.Matcher
	if pathGlob != "" {
		glob = filter.New([]string{pathGlob})
	}
	var hits []searchRawHit
	stop := fmt.Errorf("search: scan cap reached")
	walkErr := filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil // best-effort: skip unreadable entries
		}
		rel, rerr := filepath.Rel(root, p)
		if rerr != nil {
			return nil
		}
		rel = filepath.ToSlash(rel)
		if rel != "." && d.Type()&fs.ModeSymlink != 0 {
			// MINOR (V-review): never follow a symlink — it can resolve
			// outside root entirely, exactly what read_source refuses
			// ("symlinks resolving outside the corpus root"); a grep
			// tool must not become a wider door than the read tool it
			// hands out a receipt for.
			return nil
		}
		if d.IsDir() {
			if rel != "." && (searchSkipDir(d.Name()) || searchIgnoreMatch(ignore, rel+"/")) {
				return filepath.SkipDir
			}
			return nil
		}
		if glob != nil && !glob.Match(rel) {
			return nil
		}
		if isEnvPatternFile(d.Name()) {
			return nil
		}
		if searchIgnoreMatch(ignore, rel) {
			return nil
		}
		info, ierr := d.Info()
		if ierr != nil || info.Size() > searchMaxFileBytes {
			return nil
		}
		data, rerr2 := os.ReadFile(p)
		if rerr2 != nil || connector.IsBinary(data) {
			return nil
		}
		fileLines := strings.Split(string(data), "\n")
		for i, line := range fileLines {
			if !re.MatchString(line) {
				continue
			}
			h := searchRawHit{File: rel, Line: i + 1, Text: line}
			if contextN > 0 {
				for b := i - contextN; b < i; b++ {
					if b >= 0 {
						h.Before = append(h.Before, fileLines[b])
					}
				}
				for a := i + 1; a <= i+contextN && a < len(fileLines); a++ {
					h.After = append(h.After, fileLines[a])
				}
			}
			hits = append(hits, h)
			if len(hits) >= searchHardScanCap {
				return stop
			}
		}
		return nil
	})
	if walkErr != nil && walkErr != stop {
		return hits, walkErr
	}
	return hits, nil
}

// isEnvPatternFile reports whether base looks like a .env-pattern secrets
// file (defense in depth, mirroring notIndexedReason's first check) — a
// grep tool must not become a way to read what read_source already
// refuses.
func isEnvPatternFile(base string) bool {
	return base == ".env" || strings.HasPrefix(base, ".env.") || strings.HasSuffix(base, ".env")
}

// groupSymbolsByFile buckets nodes by FilePath, each bucket sorted
// ascending by Line, for enclosingSymbol's scan.
func groupSymbolsByFile(nodes []core.SymbolNode) map[string][]core.SymbolNode {
	byFile := map[string][]core.SymbolNode{}
	for _, n := range nodes {
		byFile[n.FilePath] = append(byFile[n.FilePath], n)
	}
	for f, syms := range byFile {
		sort.Slice(syms, func(i, j int) bool { return syms[i].Line < syms[j].Line })
		byFile[f] = syms
	}
	return byFile
}

// enclosingSymbol returns the innermost symbol (from a file's Line-sorted
// symbol list) whose span contains line: EndLine when the extractor set
// it, else bounded by the next symbol's start line (or open-ended for the
// file's last symbol) — the SAME candidate selection as before this fix,
// so a real EndLine's nesting behavior (an outer symbol still wins when a
// later, more specific one does not itself reach line) is unchanged.
//
// MAJOR fix (V-review): the Go extractor never sets EndLine, so that
// fallback boundary is a GUESS — "the next symbol starts here" is not "this
// symbol's body ends here". A doc comment or a blank run between two
// symbols sits inside the naive boundary and was silently attributed to
// the symbol ABOVE it. Once the naive-boundary winner is chosen, this now
// re-checks ONLY that winner (never a candidate that already lost to a
// more specific real-EndLine nested match) against lines — the file's
// content, nil when unreadable — narrowing its trusted span to the first
// blank line or comment-only run before the next symbol (narrowFallbackSpan).
// A hit past that narrowed point is honestly unattributable: this returns
// (_, false) rather than keep the naive guess. lines==nil (can't narrow)
// or a real EndLine (nothing to narrow) leaves the naive answer untouched.
// Returns (_, false) when no symbol's span covers line at all — never a
// guess at the nearest one.
func enclosingSymbol(syms []core.SymbolNode, line int, lines []string) (core.SymbolNode, bool) {
	var best core.SymbolNode
	found := false
	bestIdx := -1
	for i, n := range syms {
		if n.Line > line {
			break
		}
		upper := n.EndLine
		if upper <= 0 {
			if i+1 < len(syms) {
				upper = syms[i+1].Line - 1
			} else {
				upper = line
			}
		}
		if line <= upper {
			best, found, bestIdx = n, true, i
		}
	}
	if found && best.EndLine <= 0 && bestIdx+1 < len(syms) {
		naive := syms[bestIdx+1].Line - 1
		if narrowed := narrowFallbackSpan(best.Line, naive, lines, symbolFileExt(best)); line > narrowed {
			return core.SymbolNode{}, false
		}
	}
	return best, found
}

// narrowFallbackSpan trims naiveUpper — the next symbol's start line minus
// one, the boundary enclosingSymbol falls back to when a symbol's EndLine
// is unset — down to the last line of startLine's block that scanning
// forward still looks unambiguously like code: the first blank line, or
// the start of a comment-only run that continues uninterrupted up to
// naiveUpper (almost always the NEXT symbol's own doc comment, not this
// symbol's body), whichever comes first. A plain run of code all the way
// to naiveUpper, a nil lines (file unreadable — nothing to narrow
// against), or an extension with no known line-comment token leaves
// naiveUpper unchanged.
func narrowFallbackSpan(startLine, naiveUpper int, lines []string, ext string) int {
	if lines == nil {
		return naiveUpper
	}
	upper := naiveUpper
	for ln := startLine; ln <= naiveUpper; ln++ {
		if ln < 1 || ln > len(lines) {
			break
		}
		if strings.TrimSpace(lines[ln-1]) == "" {
			upper = ln - 1
			break
		}
	}
	if tok, known := searchLineCommentTokens[ext]; known {
		commentStart := naiveUpper + 1
		for ln := naiveUpper; ln >= startLine; ln-- {
			if ln < 1 || ln > len(lines) {
				break
			}
			text := strings.TrimSpace(lines[ln-1])
			if text == "" || !strings.HasPrefix(text, tok) {
				break
			}
			commentStart = ln
		}
		if commentStart-1 < upper {
			upper = commentStart - 1
		}
	}
	return upper
}

// symbolFileExt returns n's lowercased file extension, for
// narrowFallbackSpan's searchLineCommentTokens lookup.
func symbolFileExt(n core.SymbolNode) string {
	return strings.ToLower(filepath.Ext(n.FilePath))
}

// symbolKindLabel maps a core.SourceKind to the RC-24 vocabulary
// (func/method/type/const/var) — ok is false for a kind this vocabulary
// does not cover (infra kinds: Terraform, K8s, Docker, …), which callers
// must omit rather than mislabel.
func symbolKindLabel(k core.SourceKind) (string, bool) {
	s := string(k)
	switch {
	case strings.Contains(s, "Method"):
		return "method", true
	case strings.HasSuffix(s, "Func") || strings.HasSuffix(s, "Fn"):
		return "func", true
	case strings.Contains(s, "Const"):
		return "const", true
	case strings.Contains(s, "Var"):
		return "var", true
	case strings.Contains(s, "Class"), strings.Contains(s, "Type"), strings.Contains(s, "Interface"),
		strings.Contains(s, "Struct"), strings.Contains(s, "Enum"), strings.Contains(s, "Record"),
		strings.Contains(s, "Module"), strings.Contains(s, "Trait"):
		return "type", true
	default:
		return "", false
	}
}

// isTestPath reports whether rel looks like a test file by common
// per-language naming conventions, or sits under a test/tests directory.
func isTestPath(rel string) bool {
	base := rel
	if i := strings.LastIndex(rel, "/"); i >= 0 {
		base = rel[i+1:]
	}
	switch {
	case strings.HasSuffix(base, "_test.go"),
		strings.HasSuffix(base, "_test.py"),
		strings.HasPrefix(base, "test_") && strings.HasSuffix(base, ".py"),
		strings.HasSuffix(base, "_spec.rb"),
		strings.Contains(base, ".test."),
		strings.Contains(base, ".spec."):
		return true
	}
	for _, seg := range strings.Split(rel, "/") {
		if seg == "test" || seg == "tests" || seg == "testdata" {
			return true
		}
	}
	return false
}

// hasPathSegment reports whether rel (slash-form) has seg as one of its
// path components.
func hasPathSegment(rel, seg string) bool {
	for _, s := range strings.Split(rel, "/") {
		if s == seg {
			return true
		}
	}
	return false
}

// searchComputeFlags computes the test/vendored/generated flags for one
// repo-relative file: test and vendored are pure path facts (no I/O);
// generated additionally reads up to 4KB of the file (connector.IsGenerated's
// own scan bound) for the "Code generated ... DO NOT EDIT" marker. Returns
// the flags sorted (test, then generated, then vendored — a fixed,
// deterministic order) for a stable "flags" join.
func searchComputeFlags(root, rel string) []string {
	var flags []string
	if isTestPath(rel) {
		flags = append(flags, "test")
	}
	generated := hasPathSegment(rel, "generated")
	if !generated {
		if f, err := os.Open(filepath.Join(root, filepath.FromSlash(rel))); err == nil {
			buf := make([]byte, 4096)
			n, _ := io.ReadFull(f, buf)
			f.Close()
			if n > 0 && connector.IsGenerated(rel, buf[:n]) {
				generated = true
			}
		}
	}
	if generated {
		flags = append(flags, "generated")
	}
	for seg := range searchVendorSegments {
		if hasPathSegment(rel, seg) {
			flags = append(flags, "vendored")
			break
		}
	}
	return flags
}

// searchLineCommentTokens is a small, LOCAL table of same-line comment
// tokens (kept separate from connector's private langCommentTokens — this
// is a narrower, deliberately conservative subset for classifyHit, not a
// connector API change) for languages whose line-comment syntax is enough,
// on its own, to say "this position opens a comment" without needing
// block-comment/multi-line-string context this classifier never guesses.
var searchLineCommentTokens = map[string]string{
	".go": "//", ".ts": "//", ".tsx": "//", ".js": "//", ".jsx": "//",
	".java": "//", ".cs": "//", ".rs": "//", ".proto": "//", ".cue": "//",
	".c": "//", ".cpp": "//", ".h": "//", ".hpp": "//",
	".py": "#", ".rb": "#", ".sh": "#", ".yaml": "#", ".yml": "#", ".toml": "#",
}

// classifyHit returns the ident/comment/string classification for the
// match at line[start:end] in a file with this extension, or ("", false)
// when the single line's lexical evidence is not conclusive. Restricted to
// searchLineCommentTokens' known extensions: an unknown extension always
// returns unknown rather than guessing "ident" by default.
func classifyHit(file, line string, start, end int) (string, bool) {
	ext := strings.ToLower(filepath.Ext(file))
	tok, known := searchLineCommentTokens[ext]
	if !known {
		return "", false
	}
	prefix := line[:start]
	if idx := strings.Index(prefix, tok); idx >= 0 {
		if !anyOddQuoteBefore(prefix[:idx]) {
			return "comment", true
		}
		return "", false // the token itself looks quoted — ambiguous
	}
	for _, q := range []byte{'"', '\'', '`'} {
		if quotedSpan(line, start, end, q) {
			return "string", true
		}
	}
	if anyOddQuoteBefore(prefix) {
		return "", false // an unterminated quote precedes the match on this line
	}
	return "ident", true
}

func quoteCountBefore(s string, q byte) int {
	n := 0
	for i := 0; i < len(s); i++ {
		if s[i] == q {
			n++
		}
	}
	return n
}

func anyOddQuoteBefore(prefix string) bool {
	for _, q := range []byte{'"', '\'', '`'} {
		if quoteCountBefore(prefix, q)%2 == 1 {
			return true
		}
	}
	return false
}

// quotedSpan reports whether [start,end) sits strictly between two q
// characters on the SAME line: an odd count of q before start (an open
// q-run), and at least one more q at or after end (it closes on this
// line).
func quotedSpan(line string, start, end int, q byte) bool {
	if quoteCountBefore(line[:start], q)%2 != 1 {
		return false
	}
	return strings.IndexByte(line[end:], q) >= 0
}

// searchHitWireSize is the marshaled byte size of one rendered hit.
func searchHitWireSize(h searchHit) int {
	b, err := json.Marshal(h)
	if err != nil {
		return searchHitMaxBytes + 1
	}
	return len(b)
}

// renderSearchHit renders one raw match into its wire shape: the trimmed
// location/text, the enclosing-symbol receipt (when byFile is non-nil and
// covers this file — enclosingSymbol's own fallback-narrowing needs the
// file's lines, read at most once per file via linesCache, nil when
// unreadable), the test/generated/vendored flags (cached per file —
// searchComputeFlags reads the file at most once regardless of hit count),
// and the ident/comment/string classification (re-locating the match in
// the returned line text with re, so classification is consistent
// regardless of which engine produced the hit).
func renderSearchHit(rh searchRawHit, re *regexp.Regexp, byFile map[string][]core.SymbolNode, flagsCache map[string][]string, linesCache map[string][]string, root string) searchHit {
	h := searchHit{
		Loc:  rh.File + ":" + strconv.Itoa(rh.Line),
		Text: truncateForWire(rh.Text, searchTextMaxChars),
	}
	if syms, ok := byFile[rh.File]; ok {
		lines, cached := linesCache[rh.File]
		if !cached {
			if data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(rh.File))); err == nil {
				lines = strings.Split(string(data), "\n")
			}
			linesCache[rh.File] = lines
		}
		if sym, found := enclosingSymbol(syms, rh.Line, lines); found {
			h.Sym = sym.Name
			if k, ok := symbolKindLabel(sym.Kind); ok {
				h.Kind = k
			}
		}
	}
	flags, cached := flagsCache[rh.File]
	if !cached {
		flags = searchComputeFlags(root, rh.File)
		flagsCache[rh.File] = flags
	}
	if len(flags) > 0 {
		h.Flags = strings.Join(flags, ",")
	}
	if re != nil {
		if loc := re.FindStringIndex(rh.Text); loc != nil {
			if cls, ok := classifyHit(rh.File, rh.Text, loc[0], loc[1]); ok {
				h.Class = cls
			}
		}
	}
	if len(rh.Before) > 0 || len(rh.After) > 0 {
		ctx := make([]string, 0, len(rh.Before)+len(rh.After))
		for _, b := range rh.Before {
			ctx = append(ctx, truncateForWire(b, searchTextMaxChars))
		}
		for _, a := range rh.After {
			ctx = append(ctx, truncateForWire(a, searchTextMaxChars))
		}
		h.Ctx = ctx
	}
	// Fit the base render (no context) to the per-hit budget by shrinking
	// Text only — Loc/Sym/Kind/Flags/Class are exact, cheap facts never
	// trimmed for size. A caller who asked for context (Ctx non-empty)
	// accepted the larger per-hit cost and is left alone here; the
	// response-level 16KB budget (fitToBudget over the whole hit list)
	// still applies regardless.
	if len(h.Ctx) == 0 {
		for searchHitWireSize(h) > searchHitMaxBytes && len(h.Text) > 0 {
			n := utf8.RuneCountInString(h.Text) - 1
			if n < 0 {
				n = 0
			}
			h.Text = truncateForWire(h.Text, n)
		}
	}
	return h
}
