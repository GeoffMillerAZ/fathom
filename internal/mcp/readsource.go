// Package mcp — read_source: a bounded, read-only raw-text read over a
// service's indexed corpus (T5-4, docs/plans/trust-and-tokenomics.md).
//
// Motivation (the H3-1 home-bench row, docs/plans/home-bench.md): the agent
// surface had no raw-text read, so a task that needed the CURRENT text at a
// cited line cost 16 turns and 601k tokens of contortion (T13) — the bench's
// single most expensive call — where a plain `sed -n` would have done it in
// one. read_source is that escape hatch: line-ranged, size-capped, so an
// agent can pin exactly the lines it needs without ever pulling a whole
// large file through the agent surface. It is the program's SECOND net-new
// tool on the curated allowlist (surface.go's companionTools) — justified
// here by the measured H3-1 evidence, not added speculatively.
//
// "Indexed corpus" is enforced, not aspirational: the readable set mirrors
// what the connector's Discover walks — .fathomignore'd paths (the same
// compiled matcher, connector.LoadIgnore), the always-skipped directories
// (connector.IsSkippedDir), files over the indexer's extraction cap, and
// binaries are all refused; .env-pattern files are refused outright as
// defense in depth on the shared daemon (secrets-shaped files must never be
// servable through an agent tool, even in a repo that forgot to ignore
// them). Symlinks are followed but must RESOLVE inside the corpus root
// (filepath.EvalSymlinks containment on both sides) — a symlink pointing
// outside the corpus is refused, closing the lexical-only traversal hole.
//
// readSourceRange is the shared implementation: registerSymbolContext
// (nav_tools.go, T5-6) reuses it to fetch the definition's inline source
// snippet from the exact same safe-read path, so a citation produced by
// either tool was read identically.
package mcp

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"unicode/utf8"

	"github.com/geoffmilleraz/fathom/internal/connector"
	"github.com/geoffmilleraz/fathom/internal/corpus"
	"github.com/geoffmilleraz/fathom/internal/filter"
	"github.com/geoffmilleraz/fathom/internal/fusion"
	"github.com/geoffmilleraz/fathom/internal/gitro"
	"github.com/geoffmilleraz/fathom/internal/scope"
	"github.com/geoffmilleraz/fathom/internal/tenancy"
)

// sourceReadMaxLines and sourceReadMaxBytes bound a single read_source call:
// generous enough for a function body or a doc section, small enough that
// one call can never smuggle a whole large file through the agent surface.
// The byte cap reuses budget.go's defaultCharBudget precedent (16000) so
// there is exactly one "how big can one tool response be" number in this
// package, not two independently-chosen ones.
const (
	sourceReadMaxLines = 200
	sourceReadMaxBytes = defaultCharBudget
)

// sourceReadDefaultBytes is read_source's own DEFAULT per-call byte budget
// (SB-3, docs/plans/strengths-bench.md; O31/O32): the home bench found
// read_source the single largest contributor to references/read_source
// over-payload (123KB of 419KB across 38 calls) — half of the old 16000
// default is plenty for the citation this tool exists to serve (a cited
// range, not a whole file) and still overridable per call via max_bytes,
// up to the sourceReadMaxBytes ceiling that also bounds symbol_context's
// own snippet read.
const sourceReadDefaultBytes = 8000

// readSourceBudget resolves the effective byte budget for one read_source
// call: a non-positive/absent max_bytes uses the measured 8000-byte
// default; a caller-declared max_bytes is honored up to the
// sourceReadMaxBytes (16000) ceiling — read_source's escape hatch is for a
// cited range, never a mechanism to pull an entire large file through in
// one call.
func readSourceBudget(requested int) int {
	if requested <= 0 {
		return sourceReadDefaultBytes
	}
	if requested > sourceReadMaxBytes {
		return sourceReadMaxBytes
	}
	return requested
}

// lineWireBytes is what one returned line costs on the wire: its marshaled
// {"line":N,"text":"…"} row, JSON escaping included — the same measure
// budget.go's fitToBudget applies to every other budgeted tool's rows
// (BP-42). json.Marshal escapes HTML characters, so this can only
// over-count what the server actually sends, never under-count it.
func lineWireBytes(ln int, text string) int {
	b, err := json.Marshal(sourceLine{Line: ln, Text: text})
	if err != nil {
		return len(text) + 32
	}
	return len(b)
}

// runeFloor moves byte index n back to the start of the rune it falls in,
// so a cut never splits a multi-byte UTF-8 sequence.
func runeFloor(s string, n int) int {
	for n > 0 && n < len(s) && !utf8Start(s[n]) {
		n--
	}
	return n
}

// fitLineToWire returns the longest prefix of text, cut on a rune boundary,
// whose row fits in room bytes on the wire. Escaping only ever adds bytes,
// so no prefix longer than room can fit and the search is bounded by it.
// A room too small for even one rune still returns the first rune: the
// smallest readable answer, never an empty line (budget.go's BA-4 rule — a
// budget is a target, not a refusal).
func fitLineToWire(ln int, text string, room int) string {
	lo, hi := 0, len(text)
	if hi > room {
		hi = room
	}
	for lo < hi {
		mid := (lo + hi + 1) / 2
		if lineWireBytes(ln, text[:runeFloor(text, mid)]) <= room {
			lo = mid
		} else {
			hi = mid - 1
		}
	}
	cut := text[:runeFloor(text, lo)]
	if cut == "" && text != "" {
		_, size := utf8.DecodeRuneInString(text)
		cut = text[:size]
	}
	return cut
}

// sourceReadMaxFileBytes is the hard per-file ceiling checked via os.Stat
// BEFORE any read — a multi-GB corpus file must refuse cheaply, never be
// loaded into the shared daemon's memory just to slice 200 lines out of it.
// The value deliberately equals internal/fusion.MaxExtractSize (2MB, pinned
// by TestReadSource_FileCeilingMatchesExtractCap so the two can never
// silently drift): a file the indexer refuses to extract is not in the
// index, so refusing to read it keeps the "indexed corpus" contract exact
// rather than approximate.
const sourceReadMaxFileBytes = 2 * 1024 * 1024

// sourceLine is one line-numbered line of a bounded read, shared by
// read_source's "lines" field and symbol_context's "snippet" field.
type sourceLine struct {
	Line int    `json:"line"`
	Text string `json:"text"`
}

// sourceRange is the outcome of a bounded, line-numbered file read.
type sourceRange struct {
	Citation      citationStatus
	ContentSHA256 string
	Lines         []sourceLine
	StartLine     int // 1-based, first line actually returned (may exceed TotalLines when the request starts past EOF)
	EndLine       int // 1-based, last line actually returned (StartLine-1 when Lines is empty)
	TotalLines    int
	ClampedEOF    bool   // an explicit caller boundary (start_line / end_line) ran past EOF
	Note          string // human-readable clamp/truncation explanation, "" if none
	// Requested is the line count the caller's own range would have
	// returned absent any cap (naturalEnd-start+1) — 0 when the request
	// starts past EOF. Capped (SB-3) is true when either the line cap or
	// the byte cap actually cut the response short; registerReadSource
	// uses the two together to build the elided block (Requested-len(Lines)
	// elided of Requested), never when the range simply reached EOF on its
	// own.
	Requested int
	Capped    bool
}

// resolveCorpusRoot resolves service's on-disk corpus root via ServiceMeta,
// refusing — rather than silently resolving a relative path against the
// daemon's ambient CWD — when no root is recorded or the recorded root is
// relative. This is the O10/C4 discipline already established for the
// arch_conformance project-rules loader (internal/projectrules/resolve.go's
// ErrRelativeCorpusRoot): a relative ServiceMeta path predates the ADR-017
// absolute-anchor fix and means "re-ingest", never "resolve from here" — a
// relative root under a shared daemon could otherwise read a FOREIGN
// service's files and attribute them to this one.
func resolveCorpusRoot(ctx context.Context, backend QueryInterface, service string) (string, error) {
	if r, ok := clonePath(ctx, service); ok {
		return r.Root, nil
	}
	path, _, found, err := backend.ServiceMeta(ctx, service)
	if err != nil {
		return "", fmt.Errorf("service metadata: %w", err)
	}
	if !found || path == "" {
		return "", fmt.Errorf("no corpus root recorded for service %q — ingest it first", service)
	}
	if !filepath.IsAbs(path) {
		return "", fmt.Errorf("service %q's recorded corpus root %q is relative (pre-O10 metadata) and cannot be resolved safely — re-ingest the service so ServiceMeta records an absolute path", service, path)
	}
	return path, nil
}

// validateRelPath is the LEXICAL half of the safe-path discipline: it
// refuses an absolute file argument and ".."-traversal (checked both on the
// raw input and on the joined, cleaned result), returning the cleaned
// slash-form repo-relative path plus the joined absolute candidate. It does
// not touch the filesystem — resolveRealPath is the physical half.
func validateRelPath(corpusRoot, file string) (rel, full string, err error) {
	if filepath.IsAbs(file) {
		return "", "", fmt.Errorf("file must be repo-relative, not absolute: %q", file)
	}
	clean := filepath.Clean(filepath.FromSlash(file))
	if clean == "." || clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
		return "", "", fmt.Errorf("file path escapes the service corpus: %q", file)
	}
	full = filepath.Join(corpusRoot, clean)
	if !pathWithin(corpusRoot, full) {
		return "", "", fmt.Errorf("file path escapes the service corpus: %q", file)
	}
	return filepath.ToSlash(clean), full, nil
}

// resolveRealPath is the PHYSICAL half: filepath.EvalSymlinks BOTH the
// corpus root and the candidate path, requiring the RESOLVED path to sit
// inside the RESOLVED root. os.ReadFile follows symlinks, so without this a
// symlinked file (or a symlinked directory) inside the corpus could serve
// content from anywhere on the daemon's disk while lexically "inside" the
// root — defeating the cross-service isolation this tool documents. A
// symlink whose target stays inside the corpus resolves and is allowed (the
// connector doesn't index the link itself, but its target's content is
// corpus content); one resolving outside is refused.
//
// The returned path is the RESOLVED real path — callers stat/read that, so
// the containment verdict and the bytes served come from the same physical
// file.
func resolveRealPath(corpusRoot, full, file string) (string, error) {
	realRoot, err := filepath.EvalSymlinks(corpusRoot)
	if err != nil {
		return "", fmt.Errorf("service corpus root %q cannot be resolved: %v — re-ingest the service", corpusRoot, err)
	}
	realFull, err := filepath.EvalSymlinks(full)
	if err != nil {
		if os.IsNotExist(err) {
			return "", fmt.Errorf("file not found in service corpus: %q", file)
		}
		return "", fmt.Errorf("file path cannot be resolved: %q", file)
	}
	if !pathWithin(realRoot, realFull) {
		return "", fmt.Errorf("file path escapes the service corpus (symlink resolves outside the corpus root): %q", file)
	}
	return realFull, nil
}

// pathWithin reports whether candidate is root itself or a descendant of
// root, by cleaned-path prefix with a separator boundary (so "/a/bc" never
// passes as inside "/a/b").
func pathWithin(root, candidate string) bool {
	if candidate == root {
		return true
	}
	rootWithSep := root
	if !strings.HasSuffix(rootWithSep, string(filepath.Separator)) {
		rootWithSep += string(filepath.Separator)
	}
	return strings.HasPrefix(candidate, rootWithSep)
}

// notIndexedReason reports why relFile (slash-form, cleaned, repo-relative)
// is outside the INDEXED corpus even though it may exist on disk — "" when
// the path is readable. Three layers, mirroring what Discover excludes at
// ingest plus one defense-in-depth rule:
//
//   - .env-pattern basenames (".env", ".env.local", "prod.env", …) are
//     refused unconditionally: secrets-shaped files must never be servable
//     through an agent tool on the shared daemon, even in a repo that
//     forgot to .fathomignore them.
//   - any path segment the connector always skips (connector.IsSkippedDir:
//     vendor, .git, testdata, generated, node_modules, .idea, .vscode,
//     .claude) — those trees are never walked, so they are not corpus.
//     Other dot-paths (e.g. .github/workflows, .fathomignore itself) ARE
//     walked and indexed, and stay readable.
//   - any path the service's own .fathomignore excludes (the same compiled
//     matcher Discover applies, connector.LoadIgnore) — checked against the
//     file and every ancestor directory, mirroring the walk-time pruning.
func notIndexedReason(ctx context.Context, corpusRoot, service, relFile string) string {
	return notIndexedReasonOpts(ctx, corpusRoot, service, relFile, notIndexedOpts{})
}

// notIndexedOpts configures notIndexedReasonOpts's checks beyond
// notIndexedReason's default (every refusal applies).
type notIndexedOpts struct {
	// SkipProjectIgnore skips ONLY the project's own .fathomignore
	// corpus-membership refusal — set exclusively by applyCoverageProfiles's
	// build-artifact carve-out (internal/mcp/review_tool.go): a coverage
	// profile is typically a workspace/ build artifact a repo's
	// .fathomignore routinely excludes from the indexed corpus, and that
	// exclusion alone must not refuse the exact files this parameter
	// exists to read. Every OTHER refusal still applies UNCHANGED: .env,
	// always-skipped directories, a scoped-ignore/purge exclusion (user-
	// or service-scope — ignoreReason distinguishes it from a plain
	// project .fathomignore hit), and suppression.
	SkipProjectIgnore bool
}

// notIndexedReasonOpts is notIndexedReason with opts. See notIndexedOpts.
func notIndexedReasonOpts(ctx context.Context, corpusRoot, service, relFile string, opts notIndexedOpts) string {
	base := filepath.Base(filepath.FromSlash(relFile))
	if base == ".env" || strings.HasPrefix(base, ".env.") || strings.HasSuffix(base, ".env") {
		return fmt.Sprintf("refusing %q: .env-pattern files are never served (secrets-shaped; defense in depth)", relFile)
	}
	segs := strings.Split(strings.Trim(relFile, "/"), "/")
	for _, seg := range segs[:len(segs)-1] {
		if connector.IsSkippedDir(seg) {
			return fmt.Sprintf("%q is not in the indexed corpus: directory %q is always excluded from ingest", relFile, seg)
		}
	}
	if reason := ignoreReason(corpusRoot, service, relFile, segs, opts.SkipProjectIgnore); reason != "" {
		return reason
	}

	// SUPPRESSIONS apply to disk reads exactly as to store reads — a
	// query-layer filter that a disk-reading tool bypasses is the V3
	// defect one abstraction up. The refusal names the suppression (its
	// own disclosure), and the collector is fed for the envelope's block.
	if set, now := scope.NewSource(scope.PathFromEnv(), nil).Snapshot(); !set.Empty() {
		if sup, hit := set.Match(service, relFile, now); hit {
			scope.Record(ctx, service, &scope.Disclosure{
				Count:   1,
				Reasons: []string{sup.Reason}, IDs: []string{sup.ID},
			})
			return fmt.Sprintf("%q is suppressed for this service (%s: %s) — fathom scope unsuppress %s restores it", relFile, sup.ID, sup.Reason, sup.ID)
		}
	}
	return ""
}

// ignoreReason checks relFile (with its pre-split segs) against the
// service's SCOPED ignore rules — project .fathomignore UNION user-scope
// ~/.fathom/ignore UNION service-scope ~/.fathom/ignore.d/<service>
// (purge's write-side chokepoint, serviceignore.go) — the same mirror
// Discover applies at ingest. "" when nothing excludes it.
//
// When skipProjectIgnore is set and the match is found to be caused ONLY
// by the project's own .fathomignore (re-checked against
// connector.UserAndServiceOnlyIgnore, the project-free matcher), this
// returns "" too — the narrow carve-out applyCoverageProfiles needs. A
// match that ALSO holds under the project-free matcher (a genuine
// user-scope or service-scope/purge exclusion) still refuses regardless of
// skipProjectIgnore: MAJOR fix (review_tool.go finding 3) — that refusal
// must never be silently bypassed.
//
// An error loading either matcher REFUSES rather than serving — a broken
// exclusion mechanism must not fail toward disclosure of purged content
// (the V-review's read_source finding this function otherwise mirrors).
func ignoreReason(corpusRoot, service, relFile string, segs []string, skipProjectIgnore bool) string {
	ignore, _, ierr := connector.LoadIgnoreScopedFor(corpusRoot, service)
	if ierr != nil {
		return fmt.Sprintf("refusing %q: the service's exclusions could not be loaded (%v) — serving without them could disclose purged content", relFile, ierr)
	}
	if ignore == nil || !matchesFileOrAncestor(ignore, relFile, segs) {
		return ""
	}
	if skipProjectIgnore {
		scoped, serr := connector.UserAndServiceOnlyIgnore(service)
		if serr != nil {
			return fmt.Sprintf("refusing %q: the service's scoped exclusions could not be loaded (%v) — serving without them could disclose purged content", relFile, serr)
		}
		if scoped == nil || !matchesFileOrAncestor(scoped, relFile, segs) {
			// Excluded ONLY by the project's own .fathomignore — the
			// corpus-membership refusal this carve-out exists to skip.
			return ""
		}
	}
	return fmt.Sprintf("%q is not in the indexed corpus: excluded by the service's ignore rules (%s or a scoped exclusion)", relFile, connector.IgnoreFileName)
}

// matchesFileOrAncestor reports whether m excludes relFile itself or any
// ancestor directory (with the trailing slash Discover uses when pruning
// dirs mid-walk — a dir-only pattern like "workspace/" must exclude every
// file beneath it here exactly as it does at ingest).
func matchesFileOrAncestor(m *filter.Matcher, relFile string, segs []string) bool {
	if m.Match(relFile) {
		return true
	}
	for i := 1; i < len(segs); i++ {
		if m.Match(strings.Join(segs[:i], "/") + "/") {
			return true
		}
	}
	return false
}

// readSourceRange reads a bounded, line-numbered range of file (repo-relative
// to service's corpus root). startLine/endLine are 1-based and optional
// (<=0 means unset): with neither set, it returns a capped whole-file view
// starting at line 1; with only startLine set, it reads from there to EOF or
// the cap, whichever comes first; with both set, the EXPLICIT range is
// honored exactly unless it exceeds maxLines, which is refused (never
// silently narrowed — a caller who pinned exact boundaries would otherwise
// be misled about what was returned).
//
// ClampedEOF is reported ONLY when the caller's own boundary was out of
// range — start_line beyond EOF, or an explicit end_line beyond EOF — never
// merely because a no-end-given default read naturally stopped at a short
// file's last line (the caller can always tell it reached EOF by comparing
// the response's end_line to total_lines). Running past the line/byte cap
// is a SEPARATE, independent condition, reported via Note regardless of
// whether an explicit end_line was given. The byte cap holds even when the
// FIRST line alone exceeds it (the minified-JS case): that line comes back
// truncated to the budget with the note set — never whole, which would
// silently void the documented cap.
//
// readSourceRange counts each line's raw text against maxBytes: its callers
// (symbol_context's snippet, obligations' cited lines) re-shape the lines
// before they reach the wire. read_source itself reads through
// readSourceLines with onWire set, so its budget bounds the lines as they
// are sent (BP-42).
func readSourceRange(ctx context.Context, backend QueryInterface, service, file string, startLine, endLine, maxLines, maxBytes int) (sourceRange, error) {
	if reads, _ := ctx.Value(citationReadsKey{}).(*citationReads); reads != nil {
		return citationSourceRange(ctx, backend, service, file, startLine, endLine, maxLines, maxBytes)
	}
	return readSourceLines(ctx, backend, service, file, startLine, endLine, maxLines, maxBytes, false)
}

// readSourceLines is readSourceRange with the byte accounting chosen: raw
// text (onWire false), or the returned "lines" array as marshaled — its
// brackets, separators and each row's {"line","text"} envelope with JSON
// escaping (onWire true). The measured reason: at the 8000-byte default,
// run-20260913T094546Z's H1-T11 t2 cell received 11,189 bytes of
// read_source, 10,849 of them the lines array, because the raw count left
// the per-line envelope out.
func readSourceLines(ctx context.Context, backend QueryInterface, service, file string, startLine, endLine, maxLines, maxBytes int, onWire bool) (sourceRange, error) {
	data, status, contentHash, err := readSourceBytes(ctx, backend, service, file, false)
	if err != nil {
		return sourceRange{}, err
	}
	rng, err := sliceSourceLines(data, startLine, endLine, maxLines, maxBytes, onWire)
	rng.Citation = status
	if onWire {
		// Only read_source exposes this unsalted identity. Snippets already
		// checked these same bytes against the index's salted fingerprint.
		rng.ContentSHA256 = contentHash
	}
	return rng, err
}

// readSourceBytes enforces the shared source-read boundaries and checks the
// bytes once. Identity-only callers stream without retaining the file.
func readSourceBytes(ctx context.Context, backend QueryInterface, service, file string, identityOnly bool) ([]byte, citationStatus, string, error) {
	if identityOnly {
		_, status, err := streamCitationFile(ctx, backend, service, file, nil)
		return nil, status, "", err
	}
	rel, full, err := sourceReadPath(ctx, backend, service, file)
	if err != nil {
		return nil, citationStatus{}, "", err
	}
	// Through the corpus reader (RT-91): the same cap, re-checked at open,
	// and every byte charged to the request's admission ticket. full is the
	// EvalSymlinks-resolved, corpus-root-contained path from resolveRealPath.
	before, known := indexedSourceFingerprint(ctx, backend, service, rel)
	epoch, epochKnown := citationEpoch(ctx, backend, service, rel, before)
	data, ex, err := corpus.ReadFileCapped(ctx, full, sourceReadMaxFileBytes)
	if err != nil {
		if ctx.Err() != nil || !errors.Is(err, fs.ErrNotExist) {
			return nil, citationStatus{}, "", err
		}
		return nil, citationStatus{}, "", fmt.Errorf("file not found in service corpus: %q", file)
	}
	if ex != nil {
		switch ex.Reason {
		case corpus.ReasonBinary:
			return nil, citationStatus{}, "", fmt.Errorf("refusing to read binary file: %q", file)
		case corpus.ReasonOversize:
			return nil, citationStatus{}, "", fmt.Errorf("file %q is %d bytes, over the %d-byte read ceiling (files this large are excluded from the index by the same extraction cap)", file, ex.Size, sourceReadMaxFileBytes)
		}
		return nil, citationStatus{}, "", fmt.Errorf("file not found in service corpus: %q", file)
	}
	if connector.IsBinary(data) {
		return nil, citationStatus{}, "", fmt.Errorf("refusing to read binary file: %q", file)
	}
	hash, contentHash := fusion.FingerprintAndContentHash(data, epoch)
	status := checkedCitation(ctx, backend, service, rel, before, known, hash, epochKnown)
	return data, status, contentHash, nil
}

func sourceReadPath(ctx context.Context, backend QueryInterface, service, file string) (string, string, error) {
	corpusRoot, err := resolveCorpusRoot(ctx, backend, service)
	if err != nil {
		return "", "", err
	}
	// Order matters: lexical validation first (an absolute path or
	// traversal gets ITS refusal, never a confusing corpus-membership one),
	// then indexed-corpus membership, then physical symlink containment.
	rel, full, err := validateRelPath(corpusRoot, file)
	if err != nil {
		return "", "", err
	}
	if r, ok := clonePath(ctx, service); ok {
		if !cloneVisiblePath(ctx, rel) {
			return "", "", fmt.Errorf("file is not git-visible in resolved checkout")
		}
		if reason := notIndexedReason(ctx, r.IndexedRoot.Root, service, rel); reason != "" {
			return "", "", fmt.Errorf("%s", reason)
		}
	}
	if reason := notIndexedReason(ctx, corpusRoot, service, rel); reason != "" {
		return "", "", fmt.Errorf("%s", reason)
	}
	full, err = resolveRealPath(corpusRoot, full, file)
	if err != nil {
		return "", "", err
	}
	// Hard per-file ceiling BEFORE reading: a stat is O(1); loading a
	// multi-GB artifact into the shared daemon just to slice 200 lines out
	// of it is an availability hole. Files this large aren't extracted into
	// the index either (fusion.MaxExtractSize), so this is also honesty.
	info, err := os.Stat(full)
	if err != nil {
		return "", "", fmt.Errorf("file not found in service corpus: %q", file)
	}
	if !info.Mode().IsRegular() {
		return "", "", fmt.Errorf("not a regular source file: %q", file)
	}
	if info.Size() > sourceReadMaxFileBytes {
		return "", "", fmt.Errorf("file %q is %d bytes, over the %d-byte read ceiling (files this large are excluded from the index by the same extraction cap)", file, info.Size(), sourceReadMaxFileBytes)
	}
	return rel, full, nil
}

// sliceSourceLines is the line-ranged, capped view of one file's bytes that
// readSourceLines and readSourceAtRef share once each has read the bytes
// its own way: the same line numbering, EOF clamping, line and byte caps,
// and notes, so a read at a ref is shaped exactly as a working-tree read.
func sliceSourceLines(data []byte, startLine, endLine, maxLines, maxBytes int, onWire bool) (sourceRange, error) {
	var allLines []string
	if len(data) > 0 {
		allLines = strings.Split(string(data), "\n")
		if n := len(allLines); n > 0 && allLines[n-1] == "" && data[len(data)-1] == '\n' {
			allLines = allLines[:n-1] // trailing "\n" produces a bogus empty final element
		}
	}
	total := len(allLines) // 0 for an empty file, matching the extractor's countLines

	start := startLine
	if start <= 0 {
		start = 1
	}
	explicitEnd := endLine > 0

	if start > total {
		return sourceRange{
			StartLine: start, EndLine: start - 1, TotalLines: total, ClampedEOF: true,
			Note: fmt.Sprintf("start_line %d is beyond EOF (file has %d lines)", start, total),
		}, nil
	}

	// naturalEnd is the caller's meaningful boundary: their explicit
	// end_line (pulled back to total, flagging clampedEOF, if it overshot),
	// or simply "to EOF" when none was given — never the internal
	// start+maxLines-1 default, which is a cap artifact, not a claim about
	// where the file ends.
	clampedEOF := false
	naturalEnd := total
	if explicitEnd {
		naturalEnd = endLine
		if naturalEnd > total {
			clampedEOF = true
			naturalEnd = total
		}
	}

	if explicitEnd && (naturalEnd-start+1) > maxLines {
		return sourceRange{}, fmt.Errorf("requested range %d-%d (%d lines) exceeds the %d-line cap per call — narrow start_line/end_line or make repeated calls", start, naturalEnd, naturalEnd-start+1, maxLines)
	}
	end := naturalEnd
	lineCapped := false
	if (end - start + 1) > maxLines {
		end = start + maxLines - 1
		lineCapped = true
	}

	var out []sourceLine
	byteCount := 0
	if onWire {
		// The array's opening bracket; each row's +1 below is its
		// separator, or the closing bracket for the last row.
		byteCount = 1
	}
	truncatedForBytes := false
	for ln := start; ln <= end; ln++ {
		text := allLines[ln-1]
		cost := len(text) + 1
		if onWire {
			cost = lineWireBytes(ln, text) + 1
		}
		if byteCount+cost > maxBytes {
			if len(out) == 0 {
				// The very first line alone exceeds the budget (minified
				// JS, generated single-line JSON): return it TRUNCATED to
				// the budget with the note set — never whole (silently
				// voiding the documented cap), never empty (making the
				// line unreadable at any budget). The cut is chosen per
				// accounting: on the wire this branch also fires for a line
				// SHORTER than maxBytes raw (its row's envelope and escaping
				// push it over), so text[:maxBytes] would slice past its end.
				var cut string
				if onWire {
					cut = fitLineToWire(ln, text, maxBytes-byteCount-1)
				} else {
					cut = text[:maxBytes]
				}
				out = append(out, sourceLine{Line: ln, Text: cut})
			}
			truncatedForBytes = true
			break
		}
		byteCount += cost
		out = append(out, sourceLine{Line: ln, Text: text})
	}
	actualEnd := start - 1
	if len(out) > 0 {
		actualEnd = out[len(out)-1].Line
	}

	var notes []string
	if clampedEOF {
		notes = append(notes, fmt.Sprintf("end_line clamped to EOF (file has %d lines)", total))
	}
	if truncatedForBytes {
		notes = append(notes, fmt.Sprintf("truncated at the %d-byte cap", maxBytes))
	} else if lineCapped {
		notes = append(notes, fmt.Sprintf("truncated at the %d-line cap", maxLines))
	}

	return sourceRange{
		Lines:      out,
		StartLine:  start,
		EndLine:    actualEnd,
		TotalLines: total,
		ClampedEOF: clampedEOF,
		Note:       strings.Join(notes, "; "),
		Requested:  naturalEnd - start + 1,
		Capped:     truncatedForBytes || lineCapped,
	}, nil
}

// readSourceAtRef is read_source's `ref` read (WAE-3): file's bytes as
// committed at the commit ref names, read through git in the service's
// corpus root (gitro.BlobAt: no lock, no fetch, no prompt), never from the
// working tree. Every filter readSourceLines applies holds, in the same
// order — the lexical path checks, then corpus membership (.env files,
// always-skipped directories, the ignore rules, suppressions), then the
// read ceiling and the binary check — and each refusal names the filter
// that hit. At a ref a symlink or a submodule entry is refused by its mode,
// never followed (the working-tree read's containment check has nothing to
// resolve in a commit), and a ref that starts with '-' is refused before git
// sees it. The bytes are charged to the request's admission ticket as the
// corpus reader charges a disk read. It returns the commit ref resolved to.
func readSourceAtRef(ctx context.Context, backend QueryInterface, service, file, ref string, startLine, endLine, maxLines, maxBytes int, onWire bool) (sourceRange, string, error) {
	corpusRoot, err := resolveCorpusRoot(ctx, backend, service)
	if err != nil {
		return sourceRange{}, "", err
	}
	rel, _, err := validateRelPath(corpusRoot, file)
	if err != nil {
		return sourceRange{}, "", err
	}
	if strings.HasPrefix(ref, "-") {
		return sourceRange{}, "", fmt.Errorf("refusing ref %q: a ref may not start with '-' (git would read it as an option)", ref)
	}
	if r, ok := clonePath(ctx, service); ok {
		if !cloneVisiblePath(ctx, rel) {
			return sourceRange{}, "", fmt.Errorf("file is not git-visible in resolved checkout")
		}
		if reason := notIndexedReason(ctx, r.IndexedRoot.Root, service, rel); reason != "" {
			return sourceRange{}, "", fmt.Errorf("%s", reason)
		}
	}
	if reason := notIndexedReason(ctx, corpusRoot, service, rel); reason != "" {
		return sourceRange{}, "", fmt.Errorf("%s", reason)
	}
	if err := tenancy.ProducerAllowsCtx(ctx, tenancy.ClassPoint); err != nil {
		return sourceRange{}, "", err
	}
	data, commit, err := gitro.BlobAt(ctx, corpusRoot, ref, rel, sourceReadMaxFileBytes)
	if err != nil {
		if ctx.Err() != nil {
			return sourceRange{}, "", ctx.Err()
		}
		return sourceRange{}, "", refReadError(file, ref, err)
	}
	if err := tenancy.Charge(ctx, int64(len(data))); err != nil {
		return sourceRange{}, "", err
	}
	if connector.IsBinary(data) {
		return sourceRange{}, "", fmt.Errorf("refusing to read binary file: %q at ref %q", file, ref)
	}
	rng, err := sliceSourceLines(data, startLine, endLine, maxLines, maxBytes, onWire)
	sum := sha256.Sum256(data)
	rng.ContentSHA256 = hex.EncodeToString(sum[:])
	return rng, commit, err
}

// refReadError names the filter a ref read hit, from gitro.BlobAt's error.
func refReadError(file, ref string, err error) error {
	var entry *gitro.EntryError
	var size *gitro.SizeError
	switch {
	case errors.As(err, &entry):
		switch entry.Kind {
		case "symlink":
			return fmt.Errorf("refusing %q at ref %q: it is a symlink at that commit (a ref read never follows a symlink)", file, ref)
		case "submodule":
			return fmt.Errorf("refusing %q at ref %q: it is a submodule at that commit (its content is another repository)", file, ref)
		}
		return fmt.Errorf("refusing %q at ref %q: it is a %s at that commit, not a file", file, ref, entry.Kind)
	case errors.As(err, &size):
		return fmt.Errorf("file %q is %d bytes at ref %q, over the %d-byte read ceiling (files this large are excluded from the index by the same extraction cap)", file, size.Size, ref, size.Max)
	case errors.Is(err, gitro.ErrBadRevision):
		return fmt.Errorf("refusing ref %q: a ref may not carry whitespace, a control character or ':'", ref)
	case errors.Is(err, gitro.ErrNotRepository):
		return fmt.Errorf("ref %q cannot be read: the service's corpus root is not a git work tree", ref)
	case errors.Is(err, gitro.ErrUnknownRevision):
		return fmt.Errorf("ref %q names no commit in the service's repository", ref)
	case errors.Is(err, gitro.ErrPathAbsent):
		return fmt.Errorf("file not found at ref %q: %q", ref, file)
	case errors.Is(err, gitro.ErrBadPath):
		return fmt.Errorf("file path escapes the service corpus: %q", file)
	}
	return fmt.Errorf("file %q could not be read at ref %q: %v", file, ref, err)
}

// ─── read_source ───────────────────────────────────────────────────────────

func registerReadSource(s *Server, backend QueryInterface) {
	s.registerTool(toolDescriptor{
		Name:        "read_source",
		Description: "Reads a line range (or capped whole file), line-numbered for citation. Not when the index already answered: a find_symbol hit carries file and line. Reach for it when you need bytes a hit or snippet didn't carry. Required: service, file.",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"service":    map[string]any{"type": "string", "description": "Service / repo identifier"},
				"file":       map[string]any{"type": "string", "description": "Repo-relative or absolute clone/worktree file, no \"..\""},
				"start_line": map[string]any{"type": "integer", "description": "1-based start line (default 1)"},
				"end_line":   map[string]any{"type": "integer", "description": "1-based end line (default start_line+199, or EOF)"},
				"max_bytes":  map[string]any{"type": "integer", "description": "Per-call byte budget (default 8000, ceiling 16000)"},
				"ref":        map[string]any{"type": "string", "description": "Commit, tag or branch to read the file at, through git (default: the working tree)"},
			},
			"required": []string{"service", "file"},
		},
		group: GroupCoding,
	}, func(ctx context.Context, args json.RawMessage) (any, *rpcError) {
		if backend == nil {
			return nil, &rpcError{Code: ErrCodeInternal, Message: "no query backend configured"}
		}
		var p struct {
			Service   string `json:"service"`
			File      string `json:"file"`
			StartLine int    `json:"start_line"`
			EndLine   int    `json:"end_line"`
			MaxBytes  int    `json:"max_bytes"`
			Ref       string `json:"ref"`
		}
		if err := json.Unmarshal(args, &p); err != nil {
			return nil, &rpcError{Code: ErrCodeParams, Message: "read_source: invalid args"}
		}
		if p.Service == "" {
			return nil, &rpcError{Code: ErrCodeParams, Message: "read_source: service required"}
		}
		if p.File == "" {
			return nil, &rpcError{Code: ErrCodeParams, Message: "read_source: file required"}
		}

		maxBytes := readSourceBudget(p.MaxBytes)
		var (
			rng    sourceRange
			commit string
			err    error
		)
		if p.Ref != "" {
			rng, commit, err = readSourceAtRef(ctx, backend, p.Service, p.File, p.Ref, p.StartLine, p.EndLine, sourceReadMaxLines, maxBytes, true)
		} else {
			rng, err = readSourceLines(ctx, backend, p.Service, p.File, p.StartLine, p.EndLine, sourceReadMaxLines, maxBytes, true)
		}
		if err != nil {
			return nil, &rpcError{Code: ErrCodeParams, Message: "read_source: " + err.Error()}
		}

		lines := make([]any, len(rng.Lines))
		for i, ln := range rng.Lines {
			lines[i] = map[string]any{"line": ln.Line, "text": ln.Text}
		}
		resp := map[string]any{
			"file":        p.File,
			"lines":       lines,
			"start_line":  rng.StartLine,
			"end_line":    rng.EndLine,
			"total_lines": rng.TotalLines,
			"clamped_eof": rng.ClampedEOF,
		}
		if rng.Note != "" {
			resp["note"] = rng.Note
		}
		if p.Ref != "" {
			resp["ref"] = p.Ref
			resp["commit"] = commit
			resp["source_identity"] = map[string]any{"state": "commit", "commit": commit, "content_sha256": rng.ContentSHA256}
		} else {
			identity := map[string]any{"state": "working_tree", "content_sha256": rng.ContentSHA256}
			if rng.Citation.State == "" {
				identity["matches_index"] = true
			} else if rng.Citation.State == "stale" {
				identity["matches_index"] = false
			}
			resp["source_identity"] = identity
		}
		// elided (SB-3) fires only when actual LINES were dropped — the
		// line/byte cap cut the range short before it reached its natural
		// end (naturalEnd/lineCapped). A single line alone exceeding the
		// byte cap (the minified-JS case) still returns that one line,
		// truncated in content but not in count: `note` already discloses
		// that, and `elided`'s count/of contract (lines dropped, not bytes
		// cut within one) would be misleading there.
		if rng.Requested > len(rng.Lines) {
			resp["elided"] = elisionFor(len(rng.Lines), rng.Requested, "line order", "call again with a larger max_bytes, or narrow start_line/end_line")
		}
		return resp, nil
	})
}
