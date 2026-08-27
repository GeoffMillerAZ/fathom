// Package mcp_test — TDD tests for source-scanning analyzer tools.
//
// All tests use a fake backend that returns canned SourceFiles so no
// on-disk connector, DuckDB, or network is required.
package mcp_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/analysis/cloc"
	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/mcp"
)

// ─── sourcesFakeBackend ───────────────────────────────────────────────────────

// sourcesFakeBackend embeds the existing extendedFakeBackend and adds a Sources
// implementation that returns canned SourceFiles.
type sourcesFakeBackend struct {
	extendedFakeBackend
	sources []core.SourceFile
}

func (s *sourcesFakeBackend) Sources(_ context.Context, _ string) ([]core.SourceFile, error) {
	return s.sources, nil
}

// compile-time check: sourcesFakeBackend must implement QueryInterface.
var _ mcp.QueryInterface = (*sourcesFakeBackend)(nil)

// ─── canned source fixtures ───────────────────────────────────────────────────

// goSrc is a tiny Go file with a shell-out, a secret assignment, and a TODO.
const goSrc = `package foo

import (
	"os/exec"
	"os"
)

// TODO(ABC-123): clean this up
func Run() {
	cmd := exec.Command("ls", "-la")
	_ = cmd
	token := "SuperSecretPassword123456789"
	_ = os.Getenv("DATABASE_URL")
	ioutil.ReadAll(nil)
}
`

// pySrc is a tiny Python file with an env key and a route.
const pySrc = `import os

DATABASE_URL = os.environ["DATABASE_URL"]

@app.route('/health', methods=['GET'])
def health_check():
    pass
`

// jsSrc is a tiny JS file with an HTTP client call and a process.env read.
const jsSrc = `const axios = require('axios');

const apiKey = process.env.API_KEY;

async function fetchData() {
  return axios.get('/api/v1/users');
}
`

func newSourcesSrv(files []core.SourceFile) *mcp.Server {
	be := &sourcesFakeBackend{
		extendedFakeBackend: extendedFakeBackend{
			nodes:    extFixtureNodes(),
			services: []string{"svc"},
		},
		sources: files,
	}
	return mcp.NewServer(be)
}

// sourcesToolPayload is a helper that extracts the JSON payload from a
// tools/call response.
func sourcesToolPayload(t *testing.T, resp map[string]any) map[string]any {
	t.Helper()
	return extToolPayload(t, resp)
}

// ─── secret_scan ─────────────────────────────────────────────────────────────

func TestSecretScan_Basic(t *testing.T) {
	files := []core.SourceFile{
		{Path: "foo/run.go", Content: []byte(goSrc), Lang: "go"},
	}
	srv := newSourcesSrv(files)
	resp := roundtrip(t, srv, request{
		JSONRPC: "2.0", ID: 1001, Method: "tools/call",
		Params: map[string]any{
			"name":      "secret_scan",
			"arguments": map[string]any{"service": "svc"},
		},
	})
	p := sourcesToolPayload(t, resp)
	findings, ok := p["findings"].([]any)
	if !ok {
		t.Fatalf("secret_scan: findings missing: %v", p)
	}
	// goSrc contains a hardcoded token literal → expect at least one finding.
	if len(findings) == 0 {
		t.Error("secret_scan: expected at least one finding for the hardcoded token")
	}
}

// TestSecretScan_BinaryFileSkipped: source scanners must never chew a
// binary-classified file (ADR-021 §1 / LT-2 follow-through) — before this
// guard, a 103MB root build artifact was regex-scanned by every scan tool on
// every call. The fixture is binary content that CONTAINS a would-be secret;
// the scanner must report nothing for it.
func TestSecretScan_BinaryFileSkipped(t *testing.T) {
	binaryWithSecret := append([]byte{0x00, 0x01, 0x02}, []byte("\naws_key = \"AKIAIOSFODNN7EXAMPLE\"\n")...)
	files := []core.SourceFile{
		{Path: "blob", Content: binaryWithSecret, Lang: core.LangBinary},
	}
	srv := newSourcesSrv(files)
	resp := roundtrip(t, srv, request{
		JSONRPC: "2.0", ID: 3300, Method: "tools/call",
		Params: map[string]any{
			"name":      "secret_scan",
			"arguments": map[string]any{"service": "svc"},
		},
	})
	p := sourcesToolPayload(t, resp)
	findings, ok := p["findings"].([]any)
	if !ok {
		t.Fatalf("findings missing: %v", p)
	}
	if len(findings) != 0 {
		t.Errorf("binary file was scanned: %d finding(s): %v", len(findings), findings)
	}
}

func TestSecretScan_MissingService(t *testing.T) {
	srv := newSourcesSrv(nil)
	resp := roundtrip(t, srv, request{
		JSONRPC: "2.0", ID: 1002, Method: "tools/call",
		Params: map[string]any{
			"name":      "secret_scan",
			"arguments": map[string]any{},
		},
	})
	errObj, ok := resp["error"].(map[string]any)
	if !ok {
		t.Fatalf("expected error, got result: %v", resp)
	}
	if code := errObj["code"].(float64); code != float64(mcp.ErrCodeParams) {
		t.Errorf("code = %v, want %v", code, mcp.ErrCodeParams)
	}
}

func TestSecretScan_NoFindings(t *testing.T) {
	files := []core.SourceFile{
		{Path: "clean.go", Content: []byte(`package foo
func Hello() string { return "hello" }
`), Lang: "go"},
	}
	srv := newSourcesSrv(files)
	resp := roundtrip(t, srv, request{
		JSONRPC: "2.0", ID: 1003, Method: "tools/call",
		Params: map[string]any{
			"name":      "secret_scan",
			"arguments": map[string]any{"service": "svc"},
		},
	})
	p := sourcesToolPayload(t, resp)
	findings, ok := p["findings"].([]any)
	if !ok {
		t.Fatalf("secret_scan NoFindings: findings missing: %v", p)
	}
	if len(findings) != 0 {
		t.Errorf("secret_scan NoFindings: expected 0 findings, got %d", len(findings))
	}
}

// ─── shell_out ────────────────────────────────────────────────────────────────

func TestShellOut_Basic(t *testing.T) {
	files := []core.SourceFile{
		{Path: "foo/run.go", Content: []byte(goSrc), Lang: "go"},
	}
	srv := newSourcesSrv(files)
	resp := roundtrip(t, srv, request{
		JSONRPC: "2.0", ID: 1010, Method: "tools/call",
		Params: map[string]any{
			"name":      "shell_out",
			"arguments": map[string]any{"service": "svc"},
		},
	})
	p := sourcesToolPayload(t, resp)
	findings, ok := p["findings"].([]any)
	if !ok {
		t.Fatalf("shell_out: findings missing: %v", p)
	}
	// goSrc has exec.Command → expect at least one finding.
	if len(findings) == 0 {
		t.Error("shell_out: expected at least one finding for exec.Command")
	}
}

func TestShellOut_MissingService(t *testing.T) {
	srv := newSourcesSrv(nil)
	resp := roundtrip(t, srv, request{
		JSONRPC: "2.0", ID: 1011, Method: "tools/call",
		Params: map[string]any{
			"name":      "shell_out",
			"arguments": map[string]any{},
		},
	})
	errObj, ok := resp["error"].(map[string]any)
	if !ok {
		t.Fatalf("expected error, got result: %v", resp)
	}
	if code := errObj["code"].(float64); code != float64(mcp.ErrCodeParams) {
		t.Errorf("code = %v, want %v", code, mcp.ErrCodeParams)
	}
}

// ─── http_client_calls ────────────────────────────────────────────────────────

func TestHTTPClientCalls_Basic(t *testing.T) {
	files := []core.SourceFile{
		{Path: "client.js", Content: []byte(jsSrc), Lang: "js"},
	}
	srv := newSourcesSrv(files)
	resp := roundtrip(t, srv, request{
		JSONRPC: "2.0", ID: 1020, Method: "tools/call",
		Params: map[string]any{
			"name":      "http_client_calls",
			"arguments": map[string]any{"service": "svc"},
		},
	})
	p := sourcesToolPayload(t, resp)
	findings, ok := p["findings"].([]any)
	if !ok {
		t.Fatalf("http_client_calls: findings missing: %v", p)
	}
	// jsSrc has axios.get → expect at least one finding.
	if len(findings) == 0 {
		t.Error("http_client_calls: expected at least one finding for axios.get")
	}
}

func TestHTTPClientCalls_MissingService(t *testing.T) {
	srv := newSourcesSrv(nil)
	resp := roundtrip(t, srv, request{
		JSONRPC: "2.0", ID: 1021, Method: "tools/call",
		Params: map[string]any{
			"name":      "http_client_calls",
			"arguments": map[string]any{},
		},
	})
	errObj, ok := resp["error"].(map[string]any)
	if !ok {
		t.Fatalf("expected error, got result: %v", resp)
	}
	if code := errObj["code"].(float64); code != float64(mcp.ErrCodeParams) {
		t.Errorf("code = %v, want %v", code, mcp.ErrCodeParams)
	}
}

// ─── cloc ─────────────────────────────────────────────────────────────────────

func TestCloc_Basic(t *testing.T) {
	files := []core.SourceFile{
		{Path: "foo/run.go", Content: []byte(goSrc), Lang: "go"},
		{Path: "bar/app.py", Content: []byte(pySrc), Lang: "py"},
	}
	srv := newSourcesSrv(files)
	resp := roundtrip(t, srv, request{
		JSONRPC: "2.0", ID: 1050, Method: "tools/call",
		Params: map[string]any{
			"name":      "cloc",
			"arguments": map[string]any{"service": "svc"},
		},
	})
	p := sourcesToolPayload(t, resp)
	byLang, ok := p["by_lang"].(map[string]any)
	if !ok {
		t.Fatalf("cloc: by_lang missing: %v", p)
	}
	if byLang["go"] == nil {
		t.Error("cloc: expected 'go' entry in by_lang")
	}
	if byLang["py"] == nil {
		t.Error("cloc: expected 'py' entry in by_lang")
	}
	if p["total"] == nil {
		t.Fatal("cloc: total missing")
	}
}

func TestCloc_MissingService(t *testing.T) {
	srv := newSourcesSrv(nil)
	resp := roundtrip(t, srv, request{
		JSONRPC: "2.0", ID: 1051, Method: "tools/call",
		Params: map[string]any{
			"name":      "cloc",
			"arguments": map[string]any{},
		},
	})
	errObj, ok := resp["error"].(map[string]any)
	if !ok {
		t.Fatalf("expected error, got result: %v", resp)
	}
	if code := errObj["code"].(float64); code != float64(mcp.ErrCodeParams) {
		t.Errorf("code = %v, want %v", code, mcp.ErrCodeParams)
	}
}

// ─── source tool profiles ─────────────────────────────────────────────────────

func TestSourceToolsInCorrectProfile(t *testing.T) {
	srv := newSourcesSrv(nil)

	// HF-5 deleted the path-scoped group surfaces (ADR-014 amendment 2); the
	// group TAXONOMY they were named after still classifies every tool, and
	// that is what this test was always really asserting.
	for _, tc := range []struct {
		name  string
		group mcp.ToolGroup
	}{
		{"secret_scan", mcp.GroupShared},
		{"shell_out", mcp.GroupShared},
		{"http_client_calls", mcp.GroupShared},
		{"cloc", mcp.GroupCoding},
	} {
		if got := srv.GroupOfForTest(tc.name); got != tc.group {
			t.Errorf("tool %q registered in group %q, want %q", tc.name, got, tc.group)
		}
	}
}

// ─── nil backend guard ────────────────────────────────────────────────────────

func TestSourceTools_NilBackend(t *testing.T) {
	srv := mcp.NewServer(nil)
	for _, toolName := range []string{
		"secret_scan", "shell_out", "http_client_calls", "cloc",
	} {
		resp := roundtrip(t, srv, request{
			JSONRPC: "2.0", ID: 9000, Method: "tools/call",
			Params: map[string]any{
				"name":      toolName,
				"arguments": map[string]any{"service": "svc"},
			},
		})
		errObj, ok := resp["error"].(map[string]any)
		if !ok {
			t.Errorf("%s with nil backend: expected error, got result: %v", toolName, resp)
			continue
		}
		if code := errObj["code"].(float64); code != float64(mcp.ErrCodeInternal) {
			t.Errorf("%s nil backend: code = %v, want %v", toolName, code, mcp.ErrCodeInternal)
		}
	}
}

// ─── JSON shape verification ──────────────────────────────────────────────────

// TestFindingJSONShape verifies the shape of individual Finding objects returned
// by security tools (file, line, rule, message fields).
func TestFindingJSONShape(t *testing.T) {
	files := []core.SourceFile{
		{Path: "foo/run.go", Content: []byte(goSrc), Lang: "go"},
	}
	srv := newSourcesSrv(files)
	resp := roundtrip(t, srv, request{
		JSONRPC: "2.0", ID: 2000, Method: "tools/call",
		Params: map[string]any{
			"name":      "secret_scan",
			"arguments": map[string]any{"service": "svc"},
		},
	})
	p := sourcesToolPayload(t, resp)
	findings, ok := p["findings"].([]any)
	if !ok || len(findings) == 0 {
		t.Skip("no findings to inspect shape")
	}
	f := findings[0].(map[string]any)
	for _, field := range []string{"file", "rule", "message"} {
		if f[field] == nil {
			t.Errorf("finding missing field %q: %v", field, f)
		}
	}
}

// TestClocJSONShape verifies the shape of the cloc by_lang entry.
func TestClocJSONShape(t *testing.T) {
	files := []core.SourceFile{
		{Path: "foo/run.go", Content: []byte(goSrc), Lang: "go"},
	}
	srv := newSourcesSrv(files)
	resp := roundtrip(t, srv, request{
		JSONRPC: "2.0", ID: 2010, Method: "tools/call",
		Params: map[string]any{
			"name":      "cloc",
			"arguments": map[string]any{"service": "svc"},
		},
	})
	p := sourcesToolPayload(t, resp)
	byLang := p["by_lang"].(map[string]any)
	goStats, ok := byLang["go"].(map[string]any)
	if !ok {
		t.Fatalf("cloc: by_lang.go missing or wrong type: %v", byLang)
	}
	for _, field := range []string{"blank", "comment", "code"} {
		if goStats[field] == nil {
			t.Errorf("cloc by_lang.go missing field %q", field)
		}
	}
	// JSON numbers come as float64.
	_ = goStats["code"].(float64)
}

// ─── sources tool: sorted output ─────────────────────────────────────────────

// TestSecretScanSortedOutput verifies results are sorted by file/line.
func TestSecretScanSortedOutput(t *testing.T) {
	const src2 = `package bar
// TODO
token := "AnotherHighEntropyToken12345678"
`
	files := []core.SourceFile{
		{Path: "z_last.go", Content: []byte(goSrc), Lang: "go"},
		{Path: "a_first.go", Content: []byte(src2), Lang: "go"},
	}
	srv := newSourcesSrv(files)
	resp := roundtrip(t, srv, request{
		JSONRPC: "2.0", ID: 3000, Method: "tools/call",
		Params: map[string]any{
			"name":      "secret_scan",
			"arguments": map[string]any{"service": "svc"},
		},
	})
	p := sourcesToolPayload(t, resp)
	findings, ok := p["findings"].([]any)
	if !ok || len(findings) < 2 {
		t.Skip("need at least 2 findings to check order")
	}
	// First finding's file should be <= second finding's file.
	f0 := findings[0].(map[string]any)["file"].(string)
	f1 := findings[1].(map[string]any)["file"].(string)
	if f0 > f1 {
		t.Errorf("findings not sorted by file: %q > %q", f0, f1)
	}
}

// ─── JSON marshalling sanity ──────────────────────────────────────────────────

// TestClocTotalConsistency verifies that total.code == sum(by_lang[*].code).
func TestClocTotalConsistency(t *testing.T) {
	files := []core.SourceFile{
		{Path: "foo/run.go", Content: []byte(goSrc), Lang: "go"},
		{Path: "bar/app.py", Content: []byte(pySrc), Lang: "py"},
	}
	srv := newSourcesSrv(files)
	resp := roundtrip(t, srv, request{
		JSONRPC: "2.0", ID: 3100, Method: "tools/call",
		Params: map[string]any{
			"name":      "cloc",
			"arguments": map[string]any{"service": "svc"},
		},
	})
	p := sourcesToolPayload(t, resp)

	// Re-marshal and unmarshal for cleaner access.
	raw, _ := json.Marshal(p)
	var typed struct {
		ByLang map[string]struct {
			Blank   int `json:"blank"`
			Comment int `json:"comment"`
			Code    int `json:"code"`
		} `json:"by_lang"`
		Total struct {
			Code int `json:"code"`
		} `json:"total"`
	}
	if err := json.Unmarshal(raw, &typed); err != nil {
		t.Fatalf("unmarshal typed: %v", err)
	}
	sumCode := 0
	for _, s := range typed.ByLang {
		sumCode += s.Code
	}
	if typed.Total.Code != sumCode {
		t.Errorf("total.code = %d, want sum-of-langs %d", typed.Total.Code, sumCode)
	}
}

// TestCloc_BinaryBucket is the LT-3 regression: a NUL-bearing file
// contributes zero code (or blank/comment) lines to cloc's output —
// regardless of how many "lines" its raw bytes would otherwise split into —
// and is instead reported as a files/bytes-only "binary" bucket, never
// silently folded into by_lang or total.
func TestCloc_BinaryBucket(t *testing.T) {
	binaryContent := []byte("\x00\x01\x02\nnot real code\n\nmore garbage\n")
	files := []core.SourceFile{
		{Path: "foo/run.go", Content: []byte(goSrc), Lang: "go"},
		{Path: "blob", Content: binaryContent, Lang: core.LangBinary},
	}
	srv := newSourcesSrv(files)
	resp := roundtrip(t, srv, request{
		JSONRPC: "2.0", ID: 3200, Method: "tools/call",
		Params: map[string]any{
			"name":      "cloc",
			"arguments": map[string]any{"service": "svc"},
		},
	})
	p := sourcesToolPayload(t, resp)

	byLang, ok := p["by_lang"].(map[string]any)
	if !ok {
		t.Fatalf("cloc: by_lang missing: %v", p)
	}
	if _, ok := byLang[core.LangBinary]; ok {
		t.Error("binary content must not appear as a by_lang entry")
	}

	binary, ok := p["binary"].(map[string]any)
	if !ok {
		t.Fatalf("cloc: binary bucket missing: %v", p)
	}
	if files, ok := binary["files"].(float64); !ok || files != 1 {
		t.Errorf("binary.files = %v, want 1", binary["files"])
	}
	if bytesGot, ok := binary["bytes"].(float64); !ok || int(bytesGot) != len(binaryContent) {
		t.Errorf("binary.bytes = %v, want %d", binary["bytes"], len(binaryContent))
	}

	// total must reflect only the non-binary (go) file's lines — the
	// binary file's raw-byte "lines" never leak into total.code.
	total, ok := p["total"].(map[string]any)
	if !ok {
		t.Fatalf("cloc: total missing: %v", p)
	}
	goStats := cloc.Count("go", goSrc)
	if got := total["code"].(float64); int(got) != goStats.Code {
		t.Errorf("total.code = %v, want %d (the go file's code lines only)", total["code"], goStats.Code)
	}
}

// TestCloc_GeneratedBucket verifies that a file whose header matches the
// "Code generated ... DO NOT EDIT" convention (ADR-021 §2 / TI-5) is reported
// as a files/bytes-only "generated" bucket — same mechanics as the binary
// bucket above — and never inflates by_lang or total.
func TestCloc_GeneratedBucket(t *testing.T) {
	generatedContent := []byte("// Code generated by protoc-gen-go. DO NOT EDIT.\npackage foo\n\nfunc X() {}\n")
	files := []core.SourceFile{
		{Path: "foo/run.go", Content: []byte(goSrc), Lang: "go"},
		{Path: "foo/foo.pb.go", Content: generatedContent, Lang: "go", Generated: true},
	}
	srv := newSourcesSrv(files)
	resp := roundtrip(t, srv, request{
		JSONRPC: "2.0", ID: 3202, Method: "tools/call",
		Params: map[string]any{
			"name":      "cloc",
			"arguments": map[string]any{"service": "svc"},
		},
	})
	p := sourcesToolPayload(t, resp)

	generated, ok := p["generated"].(map[string]any)
	if !ok {
		t.Fatalf("cloc: generated bucket missing: %v", p)
	}
	if files, ok := generated["files"].(float64); !ok || files != 1 {
		t.Errorf("generated.files = %v, want 1", generated["files"])
	}
	if bytesGot, ok := generated["bytes"].(float64); !ok || int(bytesGot) != len(generatedContent) {
		t.Errorf("generated.bytes = %v, want %d", generated["bytes"], len(generatedContent))
	}

	// total must reflect only the hand-written go file's lines — the
	// generated file's lines never leak into total.code.
	total, ok := p["total"].(map[string]any)
	if !ok {
		t.Fatalf("cloc: total missing: %v", p)
	}
	goStats := cloc.Count("go", goSrc)
	if got := total["code"].(float64); int(got) != goStats.Code {
		t.Errorf("total.code = %v, want %d (the hand-written file's code lines only)", total["code"], goStats.Code)
	}
}

// TestCloc_UnclassifiedBucket verifies that a file with no registered
// extractor (an empty Lang, as a defensively-handled edge case) is reported
// under the explicit core.LangUnclassified bucket rather than "other".
func TestCloc_UnclassifiedBucket(t *testing.T) {
	files := []core.SourceFile{
		{Path: "notes.xyz", Content: []byte("some content\nmore content\n"), Lang: ""},
	}
	srv := newSourcesSrv(files)
	resp := roundtrip(t, srv, request{
		JSONRPC: "2.0", ID: 3201, Method: "tools/call",
		Params: map[string]any{
			"name":      "cloc",
			"arguments": map[string]any{"service": "svc"},
		},
	})
	p := sourcesToolPayload(t, resp)
	byLang, ok := p["by_lang"].(map[string]any)
	if !ok {
		t.Fatalf("cloc: by_lang missing: %v", p)
	}
	if _, ok := byLang[core.LangUnclassified]; !ok {
		t.Errorf("expected %q entry in by_lang, got %v", core.LangUnclassified, byLang)
	}
	if _, ok := byLang["other"]; ok {
		t.Error(`"other" bucket must never appear — unclassified files get an explicit label`)
	}
}
