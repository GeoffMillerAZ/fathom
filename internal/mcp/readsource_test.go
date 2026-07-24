// Package mcp_test — TDD tests for read_source (T5-4) and symbol_context's
// inline definition snippet (T5-6), docs/plans/trust-and-tokenomics.md.
package mcp_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/extractor"
	"github.com/geoffmilleraz/fathom/internal/mcp"
)

// writeCorpusFile writes content under root/relPath, creating parent dirs,
// and returns the absolute path written.
func writeCorpusFile(t *testing.T, root, relPath, content string) string {
	t.Helper()
	full := filepath.Join(root, filepath.FromSlash(relPath))
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", relPath, err)
	}
	return full
}

// readSourceCall issues a read_source tools/call and returns the payload.
func readSourceCall(t *testing.T, srv *mcp.Server, args map[string]any) map[string]any {
	t.Helper()
	resp := roundtrip(t, srv, request{
		JSONRPC: "2.0", ID: 2001, Method: "tools/call",
		Params: map[string]any{"name": "read_source", "arguments": args},
	})
	return xtraPayload(t, resp)
}

// readSourceCallError issues a read_source tools/call expected to fail and
// returns the error object.
func readSourceCallError(t *testing.T, srv *mcp.Server, args map[string]any) map[string]any {
	t.Helper()
	resp := roundtrip(t, srv, request{
		JSONRPC: "2.0", ID: 2002, Method: "tools/call",
		Params: map[string]any{"name": "read_source", "arguments": args},
	})
	return xtraError(t, resp)
}

func lineTexts(t *testing.T, p map[string]any) []string {
	t.Helper()
	raw, ok := p["lines"].([]any)
	if !ok {
		t.Fatalf("lines missing or wrong type: %v", p)
	}
	out := make([]string, len(raw))
	for i, l := range raw {
		m, ok := l.(map[string]any)
		if !ok {
			t.Fatalf("lines[%d] not an object: %v", i, l)
		}
		out[i], _ = m["text"].(string)
	}
	return out
}

const tenLineFile = "line1\nline2\nline3\nline4\nline5\nline6\nline7\nline8\nline9\nline10\n"

// ─── read_source: happy paths ────────────────────────────────────────────────

func TestReadSource_WholeFileCapped(t *testing.T) {
	root := t.TempDir()
	writeCorpusFile(t, root, "pkg/foo.go", tenLineFile)
	srv := mcp.NewServer(&extendedFakeBackend{services: []string{"svc"}, svcPath: root})

	p := readSourceCall(t, srv, map[string]any{"service": "svc", "file": "pkg/foo.go"})
	if got := p["start_line"]; got != float64(1) {
		t.Errorf("start_line = %v, want 1", got)
	}
	if got := p["end_line"]; got != float64(10) {
		t.Errorf("end_line = %v, want 10", got)
	}
	if got := p["total_lines"]; got != float64(10) {
		t.Errorf("total_lines = %v, want 10", got)
	}
	if p["clamped_eof"] != false {
		t.Errorf("clamped_eof = %v, want false", p["clamped_eof"])
	}
	texts := lineTexts(t, p)
	if len(texts) != 10 || texts[0] != "line1" || texts[9] != "line10" {
		t.Errorf("lines = %v, want line1..line10", texts)
	}
}

func TestReadSource_ExplicitRange_LineNumbersCorrect(t *testing.T) {
	root := t.TempDir()
	writeCorpusFile(t, root, "pkg/foo.go", tenLineFile)
	srv := mcp.NewServer(&extendedFakeBackend{services: []string{"svc"}, svcPath: root})

	p := readSourceCall(t, srv, map[string]any{"service": "svc", "file": "pkg/foo.go", "start_line": 3, "end_line": 5})
	if got := p["start_line"]; got != float64(3) {
		t.Errorf("start_line = %v, want 3", got)
	}
	if got := p["end_line"]; got != float64(5) {
		t.Errorf("end_line = %v, want 5", got)
	}
	raw, _ := p["lines"].([]any)
	if len(raw) != 3 {
		t.Fatalf("lines count = %d, want 3: %v", len(raw), raw)
	}
	wantLine := 3
	wantText := []string{"line3", "line4", "line5"}
	for i, l := range raw {
		m := l.(map[string]any)
		if int(m["line"].(float64)) != wantLine {
			t.Errorf("lines[%d].line = %v, want %d", i, m["line"], wantLine)
		}
		if m["text"] != wantText[i] {
			t.Errorf("lines[%d].text = %v, want %q", i, m["text"], wantText[i])
		}
		wantLine++
	}
	if p["note"] != nil {
		t.Errorf("note = %v, want absent for an in-range request", p["note"])
	}
}

func TestReadSource_StartLineOnly_ReadsToCapOrEOF(t *testing.T) {
	root := t.TempDir()
	writeCorpusFile(t, root, "pkg/foo.go", tenLineFile)
	srv := mcp.NewServer(&extendedFakeBackend{services: []string{"svc"}, svcPath: root})

	p := readSourceCall(t, srv, map[string]any{"service": "svc", "file": "pkg/foo.go", "start_line": 8})
	texts := lineTexts(t, p)
	if want := []string{"line8", "line9", "line10"}; len(texts) != len(want) || texts[0] != want[0] || texts[2] != want[2] {
		t.Errorf("lines = %v, want %v", texts, want)
	}
	// No end_line was given, so reaching the file's natural last line is NOT
	// a clamp — the caller never claimed a boundary that got pulled back.
	// end_line == total_lines is how a caller tells "I reached EOF".
	if p["clamped_eof"] != false {
		t.Errorf("clamped_eof = %v, want false (no explicit end_line was given)", p["clamped_eof"])
	}
	if got := p["end_line"]; got != float64(10) {
		t.Errorf("end_line = %v, want 10", got)
	}
	if got := p["total_lines"]; got != float64(10) {
		t.Errorf("total_lines = %v, want 10", got)
	}
	if p["note"] != nil {
		t.Errorf("note = %v, want absent (3 lines is well under the cap)", p["note"])
	}
}

// ─── read_source: EOF clamp (never a refusal) ────────────────────────────────

func TestReadSource_EndLineBeyondEOF_ClampedNotRefused(t *testing.T) {
	root := t.TempDir()
	writeCorpusFile(t, root, "pkg/foo.go", tenLineFile)
	srv := mcp.NewServer(&extendedFakeBackend{services: []string{"svc"}, svcPath: root})

	p := readSourceCall(t, srv, map[string]any{"service": "svc", "file": "pkg/foo.go", "start_line": 8, "end_line": 500})
	if got := p["end_line"]; got != float64(10) {
		t.Errorf("end_line = %v, want 10 (clamped)", got)
	}
	if p["clamped_eof"] != true {
		t.Errorf("clamped_eof = %v, want true", p["clamped_eof"])
	}
	note, _ := p["note"].(string)
	if !strings.Contains(note, "EOF") {
		t.Errorf("note = %q, want it to mention EOF", note)
	}
}

func TestReadSource_StartLineBeyondEOF_EmptyNotRefused(t *testing.T) {
	root := t.TempDir()
	writeCorpusFile(t, root, "pkg/foo.go", tenLineFile)
	srv := mcp.NewServer(&extendedFakeBackend{services: []string{"svc"}, svcPath: root})

	p := readSourceCall(t, srv, map[string]any{"service": "svc", "file": "pkg/foo.go", "start_line": 999})
	raw, _ := p["lines"].([]any)
	if len(raw) != 0 {
		t.Errorf("lines = %v, want empty", raw)
	}
	if p["clamped_eof"] != true {
		t.Errorf("clamped_eof = %v, want true", p["clamped_eof"])
	}
	note, _ := p["note"].(string)
	if !strings.Contains(note, "beyond EOF") {
		t.Errorf("note = %q, want it to mention beyond EOF", note)
	}
}

// ─── read_source: refusals ───────────────────────────────────────────────────

func TestReadSource_AbsolutePath_Refused(t *testing.T) {
	root := t.TempDir()
	writeCorpusFile(t, root, "pkg/foo.go", tenLineFile)
	srv := mcp.NewServer(&extendedFakeBackend{services: []string{"svc"}, svcPath: root})

	errObj := readSourceCallError(t, srv, map[string]any{"service": "svc", "file": "/etc/passwd"})
	msg, _ := errObj["message"].(string)
	if !strings.Contains(msg, "absolute") {
		t.Errorf("message = %q, want it to mention absolute", msg)
	}
}

func TestReadSource_PathTraversal_Refused(t *testing.T) {
	root := t.TempDir()
	writeCorpusFile(t, root, "pkg/foo.go", tenLineFile)
	// A file OUTSIDE root that traversal must not reach.
	outside := t.TempDir()
	writeCorpusFile(t, outside, "secret.txt", "top secret\n")

	srv := mcp.NewServer(&extendedFakeBackend{services: []string{"svc"}, svcPath: root})

	cases := []string{"../secret.txt", "pkg/../../secret.txt", ".."}
	for _, f := range cases {
		errObj := readSourceCallError(t, srv, map[string]any{"service": "svc", "file": f})
		msg, _ := errObj["message"].(string)
		if !strings.Contains(msg, "escapes") {
			t.Errorf("file %q: message = %q, want it to mention escapes", f, msg)
		}
	}
}

func TestReadSource_BinaryFile_Refused(t *testing.T) {
	root := t.TempDir()
	full := filepath.Join(root, "bin.dat")
	if err := os.WriteFile(full, []byte("PK\x00\x00binary-ish-content"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	srv := mcp.NewServer(&extendedFakeBackend{services: []string{"svc"}, svcPath: root})

	errObj := readSourceCallError(t, srv, map[string]any{"service": "svc", "file": "bin.dat"})
	msg, _ := errObj["message"].(string)
	if !strings.Contains(msg, "binary") {
		t.Errorf("message = %q, want it to mention binary", msg)
	}
}

func TestReadSource_ExplicitRangeExceedsCap_Refused(t *testing.T) {
	root := t.TempDir()
	var sb strings.Builder
	for i := 1; i <= 300; i++ {
		sb.WriteString("line\n")
	}
	writeCorpusFile(t, root, "big.txt", sb.String())
	srv := mcp.NewServer(&extendedFakeBackend{services: []string{"svc"}, svcPath: root})

	errObj := readSourceCallError(t, srv, map[string]any{"service": "svc", "file": "big.txt", "start_line": 1, "end_line": 300})
	msg, _ := errObj["message"].(string)
	if !strings.Contains(msg, "cap") {
		t.Errorf("message = %q, want it to mention the cap", msg)
	}
}

func TestReadSource_FileNotFound_Refused(t *testing.T) {
	root := t.TempDir()
	srv := mcp.NewServer(&extendedFakeBackend{services: []string{"svc"}, svcPath: root})

	errObj := readSourceCallError(t, srv, map[string]any{"service": "svc", "file": "does/not/exist.go"})
	msg, _ := errObj["message"].(string)
	if !strings.Contains(msg, "not found") {
		t.Errorf("message = %q, want it to mention not found", msg)
	}
}

func TestReadSource_NoServiceMeta_Refused(t *testing.T) {
	srv := mcp.NewServer(&extendedFakeBackend{services: []string{"svc"}}) // svcPath unset
	errObj := readSourceCallError(t, srv, map[string]any{"service": "svc", "file": "pkg/foo.go"})
	msg, _ := errObj["message"].(string)
	if !strings.Contains(msg, "ingest") {
		t.Errorf("message = %q, want it to mention ingest", msg)
	}
}

// TestReadSource_RelativeServiceMetaPath_Refused mirrors
// TestArchConformance_RelativeServiceMetaPath_NotChecked (extra_tools_test.go):
// a pre-O10 relative ServiceMeta path must never be resolved against the
// daemon's ambient CWD — refuse and name re-ingest, never silently read a
// foreign repo's files under this service's name.
func TestReadSource_RelativeServiceMetaPath_Refused(t *testing.T) {
	srv := mcp.NewServer(&extendedFakeBackend{services: []string{"svc"}, svcPath: "."})
	errObj := readSourceCallError(t, srv, map[string]any{"service": "svc", "file": "pkg/foo.go"})
	msg, _ := errObj["message"].(string)
	if !strings.Contains(msg, "re-ingest") {
		t.Errorf("message = %q, want it to mention re-ingest", msg)
	}
}

func TestReadSource_ServiceRequired(t *testing.T) {
	srv := mcp.NewServer(&extendedFakeBackend{services: []string{"svc"}})
	errObj := readSourceCallError(t, srv, map[string]any{"file": "pkg/foo.go"})
	msg, _ := errObj["message"].(string)
	if !strings.Contains(msg, "service") {
		t.Errorf("message = %q, want it to mention the missing service parameter", msg)
	}
}

func TestReadSource_FileRequired(t *testing.T) {
	srv := mcp.NewServer(&extendedFakeBackend{services: []string{"svc"}, svcPath: t.TempDir()})
	errObj := readSourceCallError(t, srv, map[string]any{"service": "svc"})
	msg, _ := errObj["message"].(string)
	if !strings.Contains(msg, "file") {
		t.Errorf("message = %q, want it to mention the missing file parameter", msg)
	}
}

// ─── read_source: on the agent surface ───────────────────────────────────────

func TestReadSource_OnAgentSurface(t *testing.T) {
	if !companionTools_contains(t, "read_source") {
		t.Error("read_source must be on the companion/agent allowlist (T5-4)")
	}
}

// companionTools_contains is a tiny local check independent of
// TestCompanionSurfaceToolsList's exact-match assertion, so this file keeps
// its own direct evidence that read_source is on the curated surface.
func companionTools_contains(t *testing.T, name string) bool {
	t.Helper()
	for _, n := range companionWant {
		if n == name {
			return true
		}
	}
	return false
}

// ─── symbol_context: inline definition snippet (T5-6) ───────────────────────

func TestSymbolContext_Snippet_Present(t *testing.T) {
	root := t.TempDir()
	writeCorpusFile(t, root, "pkg/alpha.go", "package pkg\n\nfunc Alpha() {\n\treturn\n}\n")

	be := &extendedFakeBackend{
		nodes: []core.SymbolNode{
			{Service: "svc", Package: "pkg", Name: "Alpha", Kind: core.KindGoFunc, FilePath: "pkg/alpha.go", Line: 3, Signature: "func Alpha()"},
		},
		services: []string{"svc"},
		svcPath:  root,
	}
	srv := mcp.NewServer(be)
	resp := roundtrip(t, srv, request{
		JSONRPC: "2.0", ID: 3001, Method: "tools/call",
		Params: map[string]any{"name": "symbol_context", "arguments": map[string]any{"service": "svc", "symbol": "Alpha"}},
	})
	p := xtraPayload(t, resp)
	raw, ok := p["snippet"].([]any)
	if !ok {
		t.Fatalf("snippet missing or wrong type: %v", p)
	}
	if len(raw) == 0 {
		t.Fatal("snippet is empty")
	}
	first := raw[0].(map[string]any)
	if int(first["line"].(float64)) != 3 {
		t.Errorf("snippet[0].line = %v, want 3 (the definition line)", first["line"])
	}
	if first["text"] != "func Alpha() {" {
		t.Errorf("snippet[0].text = %v, want %q", first["text"], "func Alpha() {")
	}
}

func TestSymbolContext_Snippet_CappedAt40Lines(t *testing.T) {
	root := t.TempDir()
	var sb strings.Builder
	sb.WriteString("package pkg\n\n")
	sb.WriteString("func Big() {\n")
	for i := 0; i < 100; i++ {
		sb.WriteString("\t_ = 1\n")
	}
	sb.WriteString("}\n")
	writeCorpusFile(t, root, "pkg/big.go", sb.String())

	be := &extendedFakeBackend{
		nodes: []core.SymbolNode{
			{Service: "svc", Package: "pkg", Name: "Big", Kind: core.KindGoFunc, FilePath: "pkg/big.go", Line: 3, Signature: "func Big()"},
		},
		services: []string{"svc"},
		svcPath:  root,
	}
	srv := mcp.NewServer(be)
	resp := roundtrip(t, srv, request{
		JSONRPC: "2.0", ID: 3002, Method: "tools/call",
		Params: map[string]any{"name": "symbol_context", "arguments": map[string]any{"service": "svc", "symbol": "Big"}},
	})
	p := xtraPayload(t, resp)
	raw, ok := p["snippet"].([]any)
	if !ok {
		t.Fatalf("snippet missing: %v", p)
	}
	if len(raw) != 40 {
		t.Errorf("snippet length = %d, want 40 (capped)", len(raw))
	}
	first := raw[0].(map[string]any)
	if int(first["line"].(float64)) != 3 {
		t.Errorf("snippet[0].line = %v, want 3", first["line"])
	}
}

func TestSymbolContext_Snippet_AbsentForUnreadableCorpus(t *testing.T) {
	be := &extendedFakeBackend{
		nodes: []core.SymbolNode{
			{Service: "svc", Package: "pkg", Name: "Alpha", Kind: core.KindGoFunc, FilePath: "pkg/alpha.go", Line: 3, Signature: "func Alpha()"},
		},
		services: []string{"svc"}, // svcPath unset — no corpus root recorded
	}
	srv := mcp.NewServer(be)
	resp := roundtrip(t, srv, request{
		JSONRPC: "2.0", ID: 3003, Method: "tools/call",
		Params: map[string]any{"name": "symbol_context", "arguments": map[string]any{"service": "svc", "symbol": "Alpha"}},
	})
	p := xtraPayload(t, resp)
	if _, present := p["snippet"]; present {
		t.Errorf("snippet should be absent (honest absence, never a fake) when the corpus is unreadable, got %v", p["snippet"])
	}
	// The rest of symbol_context must still succeed — an unreadable corpus
	// degrades the additive snippet field only, never the whole call.
	def, ok := p["def"].(map[string]any)
	if !ok || def["name"] != "Alpha" {
		t.Errorf("def should still be present and correct: %v", p["def"])
	}
}

func TestSymbolContext_DefEndLine(t *testing.T) {
	be := &extendedFakeBackend{
		nodes: []core.SymbolNode{
			{Service: "svc", Package: "pkg", Name: "Alpha", Kind: core.KindGoFunc, FilePath: "pkg/alpha.go", Line: 3, EndLine: 9, Signature: "func Alpha()"},
		},
		services: []string{"svc"},
	}
	srv := mcp.NewServer(be)
	resp := roundtrip(t, srv, request{
		JSONRPC: "2.0", ID: 3004, Method: "tools/call",
		Params: map[string]any{"name": "symbol_context", "arguments": map[string]any{"service": "svc", "symbol": "Alpha"}},
	})
	p := xtraPayload(t, resp)
	def, ok := p["def"].(map[string]any)
	if !ok {
		t.Fatalf("def missing: %v", p)
	}
	if got := def["end_line"]; got != float64(9) {
		t.Errorf("def.end_line = %v, want 9", got)
	}
}

// ─── T5-5: DocChunk line offsets end-to-end (extractor → serving), the
// T08-class fixture ────────────────────────────────────────────────────────
//
// T08 (docs/plans/home-bench.md H3-1): fathom derived "line 20" by
// arithmetic off an L18 heading; the truth was L29 — a DocChunk's single
// heading Line forced a downstream guess at the section's extent. This test
// proves the fix end-to-end: the REAL MarkdownExtractor computes EndLine per
// heading, the fusion→store→serving path carries it (ListAllSymbols here,
// standing in for the store round trip proven separately by
// internal/store's own tests), and read_source over [Line, EndLine] — the
// intended agent flow: find_symbol/symbol_context for the section's
// [start,end], then read_source that exact range — recovers the section's
// real text, including the target sentence's REAL line number, never a
// guess.
func TestT08Class_DocSectionLineRecovery(t *testing.T) {
	const doc = `# Introduction

This is the intro paragraph.

## Installation

Run the install command.

### Advanced Setup

For advanced users.

## Usage

Basic usage examples.
`
	root := t.TempDir()
	writeCorpusFile(t, root, "README.md", doc)

	nodes, err := extractor.NewMarkdownExtractor().Extract("svc", "README.md", []byte(doc))
	if err != nil {
		t.Fatalf("Extract: %v", err)
	}

	be := &extendedFakeBackend{nodes: nodes, services: []string{"svc"}, svcPath: root}
	srv := mcp.NewServer(be)

	// Step 1 (find_symbol): resolve the "Installation" section's real extent.
	resp := roundtrip(t, srv, request{
		JSONRPC: "2.0", ID: 4001, Method: "tools/call",
		Params: map[string]any{"name": "find_symbol", "arguments": map[string]any{"service": "svc", "name": "Installation"}},
	})
	p := xtraPayload(t, resp)
	symbols, ok := p["symbols"].([]any)
	if !ok || len(symbols) == 0 {
		t.Fatalf("find_symbol(Installation): no symbols: %v", p)
	}
	sym := symbols[0].(map[string]any)
	startLine := int(sym["line"].(float64))
	endLine := int(sym["end_line"].(float64))
	if startLine != 5 || endLine != 8 {
		t.Fatalf("Installation section = [%d,%d], want [5,8]", startLine, endLine)
	}

	// Step 2 (read_source): pull the real, line-numbered section text.
	rsResp := roundtrip(t, srv, request{
		JSONRPC: "2.0", ID: 4002, Method: "tools/call",
		Params: map[string]any{
			"name":      "read_source",
			"arguments": map[string]any{"service": "svc", "file": "README.md", "start_line": startLine, "end_line": endLine},
		},
	})
	rsp := xtraPayload(t, rsResp)
	raw, _ := rsp["lines"].([]any)
	var foundLine int
	for _, l := range raw {
		m := l.(map[string]any)
		if m["text"] == "Run the install command." {
			foundLine = int(m["line"].(float64))
		}
	}
	// The T08 bug guessed line 20 off an L18 heading when the truth was L29
	// — i.e. it invented an offset instead of reading it. Here the real
	// line (7) comes straight from read_source's line numbering, not
	// arithmetic on the heading line (5).
	if foundLine != 7 {
		t.Errorf("\"Run the install command.\" reported at line %d, want 7 (the real line, not a heading-relative guess)", foundLine)
	}
}
