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
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/geoffmilleraz/fathom/internal/connector"
	"github.com/geoffmilleraz/fathom/internal/scope"
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
	Lines      []sourceLine
	StartLine  int // 1-based, first line actually returned (may exceed TotalLines when the request starts past EOF)
	EndLine    int // 1-based, last line actually returned (StartLine-1 when Lines is empty)
	TotalLines int
	ClampedEOF bool   // an explicit caller boundary (start_line / end_line) ran past EOF
	Note       string // human-readable clamp/truncation explanation, "" if none
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
	// SERVICE-SCOPED exclusions (the V-review's read_source finding): the
	// mirror this function claims — "the readable set mirrors what
	// Discover walks" — was enforced with the service-BLIND loader, so for
	// exactly a purged service the mirror silently diverged: Discover
	// excluded secrets/**, read_source served it, and one MCP call
	// returned a purged credential verbatim after a purge printed
	// VERIFIED. The scoped loader is the SAME one Discover now uses; an
	// error loading it REFUSES rather than serving — a broken exclusion
	// mechanism must not fail toward disclosure of purged content.
	ignore, _, ierr := connector.LoadIgnoreScopedFor(corpusRoot, service)
	if ierr != nil {
		return fmt.Sprintf("refusing %q: the service's exclusions could not be loaded (%v) — serving without them could disclose purged content", relFile, ierr)
	}
	if ignore != nil {
		if ignore.Match(relFile) {
			return fmt.Sprintf("%q is not in the indexed corpus: excluded by the service's ignore rules (%s or a scoped exclusion)", relFile, connector.IgnoreFileName)
		}
		// Ancestor directories, with the trailing slash Discover uses when
		// pruning dirs mid-walk — a dir-only pattern ("workspace/") must
		// exclude every file beneath it here exactly as it does at ingest.
		for i := 1; i < len(segs); i++ {
			if ignore.Match(strings.Join(segs[:i], "/") + "/") {
				return fmt.Sprintf("%q is not in the indexed corpus: excluded by the service's ignore rules (%s or a scoped exclusion)", relFile, connector.IgnoreFileName)
			}
		}
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
func readSourceRange(ctx context.Context, backend QueryInterface, service, file string, startLine, endLine, maxLines, maxBytes int) (sourceRange, error) {
	corpusRoot, err := resolveCorpusRoot(ctx, backend, service)
	if err != nil {
		return sourceRange{}, err
	}
	// Order matters: lexical validation first (an absolute path or
	// traversal gets ITS refusal, never a confusing corpus-membership one),
	// then indexed-corpus membership, then physical symlink containment.
	rel, full, err := validateRelPath(corpusRoot, file)
	if err != nil {
		return sourceRange{}, err
	}
	if reason := notIndexedReason(ctx, corpusRoot, service, rel); reason != "" {
		return sourceRange{}, fmt.Errorf("%s", reason)
	}
	full, err = resolveRealPath(corpusRoot, full, file)
	if err != nil {
		return sourceRange{}, err
	}
	// Hard per-file ceiling BEFORE reading: a stat is O(1); loading a
	// multi-GB artifact into the shared daemon just to slice 200 lines out
	// of it is an availability hole. Files this large aren't extracted into
	// the index either (fusion.MaxExtractSize), so this is also honesty.
	info, err := os.Stat(full)
	if err != nil {
		return sourceRange{}, fmt.Errorf("file not found in service corpus: %q", file)
	}
	if info.Size() > sourceReadMaxFileBytes {
		return sourceRange{}, fmt.Errorf("file %q is %d bytes, over the %d-byte read ceiling (files this large are excluded from the index by the same extraction cap)", file, info.Size(), sourceReadMaxFileBytes)
	}
	data, err := os.ReadFile(full) //nolint:gosec — full is the EvalSymlinks-resolved, corpus-root-contained path from resolveRealPath
	if err != nil {
		return sourceRange{}, fmt.Errorf("file not found in service corpus: %q", file)
	}
	if connector.IsBinary(data) {
		return sourceRange{}, fmt.Errorf("refusing to read binary file: %q", file)
	}

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
	truncatedForBytes := false
	for ln := start; ln <= end; ln++ {
		text := allLines[ln-1]
		if byteCount+len(text)+1 > maxBytes {
			if len(out) == 0 {
				// The very first line alone exceeds the budget (minified
				// JS, generated single-line JSON): return it TRUNCATED to
				// the budget with the note set — never whole (silently
				// voiding the documented cap), never empty (making the
				// line unreadable at any budget).
				out = append(out, sourceLine{Line: ln, Text: text[:maxBytes]})
			}
			truncatedForBytes = true
			break
		}
		byteCount += len(text) + 1
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
	}, nil
}

// ─── read_source ───────────────────────────────────────────────────────────

func registerReadSource(s *Server, backend QueryInterface) {
	s.registerTool(toolDescriptor{
		Name:        "read_source",
		Description: "Read a line range (or a capped whole-file view) of one file from a service's INDEXED corpus. Read-only, line-numbered so results are pinnable as citations. Capped at 200 lines / 16000 bytes per call, whichever is hit first — narrow start_line/end_line for larger files or ranges; end_line == total_lines in the response means EOF was reached; a single line longer than the byte cap comes back truncated to it, with a note. Readable = what ingest walks: refuses .fathomignore'd paths, always-skipped directories (vendor, .git, testdata, generated, node_modules, .idea, .vscode, .claude), .env-pattern files, files over the 2MB extraction cap, binary files, absolute paths, path traversal, symlinks resolving outside the corpus root, and an EXPLICIT range wider than the line cap. An explicit start_line or end_line landing beyond EOF is clamped (not refused) and noted.",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"service":    map[string]any{"type": "string", "description": "Service / repo identifier"},
				"file":       map[string]any{"type": "string", "description": "Repo-relative file path (never absolute, never containing \"..\")"},
				"start_line": map[string]any{"type": "integer", "description": "1-based start line (default 1)"},
				"end_line":   map[string]any{"type": "integer", "description": "1-based end line, inclusive (default: start_line + 199, or EOF). An explicit range wider than the cap is refused, not narrowed."},
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

		rng, err := readSourceRange(ctx, backend, p.Service, p.File, p.StartLine, p.EndLine, sourceReadMaxLines, sourceReadMaxBytes)
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
		return resp, nil
	})
}
