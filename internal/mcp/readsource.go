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
	ClampedEOF bool   // the requested (or default) end ran past EOF
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

// resolveSafePath joins corpusRoot and file — a caller-supplied,
// expected-repo-relative path — refusing an absolute file argument or any
// path that would escape corpusRoot via "..". Defense in depth: the
// traversal check runs both on the raw (pre-Join) input and again on the
// joined, cleaned result.
func resolveSafePath(corpusRoot, file string) (string, error) {
	if filepath.IsAbs(file) {
		return "", fmt.Errorf("file must be repo-relative, not absolute: %q", file)
	}
	clean := filepath.Clean(filepath.FromSlash(file))
	if clean == "." || clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("file path escapes the service corpus: %q", file)
	}
	full := filepath.Join(corpusRoot, clean)
	rootWithSep := corpusRoot
	if !strings.HasSuffix(rootWithSep, string(filepath.Separator)) {
		rootWithSep += string(filepath.Separator)
	}
	if !strings.HasPrefix(full, rootWithSep) {
		return "", fmt.Errorf("file path escapes the service corpus: %q", file)
	}
	return full, nil
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
// whether an explicit end_line was given.
func readSourceRange(ctx context.Context, backend QueryInterface, service, file string, startLine, endLine, maxLines, maxBytes int) (sourceRange, error) {
	corpusRoot, err := resolveCorpusRoot(ctx, backend, service)
	if err != nil {
		return sourceRange{}, err
	}
	full, err := resolveSafePath(corpusRoot, file)
	if err != nil {
		return sourceRange{}, err
	}
	data, err := os.ReadFile(full) //nolint:gosec — full is corpus-root-confined by resolveSafePath above
	if err != nil {
		return sourceRange{}, fmt.Errorf("file not found in service corpus: %q", file)
	}
	if connector.IsBinary(data) {
		return sourceRange{}, fmt.Errorf("refusing to read binary file: %q", file)
	}

	allLines := strings.Split(string(data), "\n")
	if n := len(allLines); n > 0 && allLines[n-1] == "" && strings.HasSuffix(string(data), "\n") {
		allLines = allLines[:n-1] // trailing "\n" produces a bogus empty final element
	}
	total := len(allLines)

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
		byteCount += len(text) + 1
		if byteCount > maxBytes && len(out) > 0 {
			truncatedForBytes = true
			break
		}
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
		Description: "Read a line range (or a capped whole-file view) of one file from a service's indexed corpus. Read-only, line-numbered so results are pinnable as citations. Capped at 200 lines / 16000 bytes per call, whichever is hit first — narrow start_line/end_line for larger files or ranges; end_line == total_lines in the response means EOF was reached. Refuses an absolute file path, path traversal, a file outside the service corpus, a binary file, and an EXPLICIT range wider than the cap. An explicit start_line or end_line landing beyond EOF is clamped (not refused) and noted; running past the cap is also noted, on any request shape.",
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
