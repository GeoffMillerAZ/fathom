// Package mcp_test — TDD tests for source-scanning analyzer tools.
//
// All tests use a fake backend that returns canned SourceFiles so no
// on-disk connector, DuckDB, or network is required.
package mcp_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/geoffmilleraz/sextant/internal/core"
	"github.com/geoffmilleraz/sextant/internal/mcp"
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

// ─── antipatterns ─────────────────────────────────────────────────────────────

func TestAntipatterns_Basic(t *testing.T) {
	files := []core.SourceFile{
		{Path: "foo/run.go", Content: []byte(goSrc), Lang: "go"},
	}
	srv := newSourcesSrv(files)
	resp := roundtrip(t, srv, request{
		JSONRPC: "2.0", ID: 1030, Method: "tools/call",
		Params: map[string]any{
			"name":      "antipatterns",
			"arguments": map[string]any{"service": "svc"},
		},
	})
	p := sourcesToolPayload(t, resp)
	if p["findings"] == nil {
		t.Fatalf("antipatterns: findings missing: %v", p)
	}
}

func TestAntipatterns_MissingService(t *testing.T) {
	srv := newSourcesSrv(nil)
	resp := roundtrip(t, srv, request{
		JSONRPC: "2.0", ID: 1031, Method: "tools/call",
		Params: map[string]any{
			"name":      "antipatterns",
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

// ─── cognitive ────────────────────────────────────────────────────────────────

func TestCognitive_Basic(t *testing.T) {
	files := []core.SourceFile{
		{Path: "foo/run.go", Content: []byte(goSrc), Lang: "go"},
	}
	srv := newSourcesSrv(files)
	resp := roundtrip(t, srv, request{
		JSONRPC: "2.0", ID: 1040, Method: "tools/call",
		Params: map[string]any{
			"name":      "cognitive",
			"arguments": map[string]any{"service": "svc"},
		},
	})
	p := sourcesToolPayload(t, resp)
	if p["functions"] == nil {
		t.Fatalf("cognitive: functions missing: %v", p)
	}
}

func TestCognitive_MissingService(t *testing.T) {
	srv := newSourcesSrv(nil)
	resp := roundtrip(t, srv, request{
		JSONRPC: "2.0", ID: 1041, Method: "tools/call",
		Params: map[string]any{
			"name":      "cognitive",
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

// ─── deprecation ─────────────────────────────────────────────────────────────

func TestDeprecation_Basic(t *testing.T) {
	files := []core.SourceFile{
		// goSrc calls ioutil.ReadAll which is in DefaultDeprecated.
		{Path: "foo/run.go", Content: []byte(goSrc), Lang: "go"},
	}
	srv := newSourcesSrv(files)
	resp := roundtrip(t, srv, request{
		JSONRPC: "2.0", ID: 1060, Method: "tools/call",
		Params: map[string]any{
			"name":      "deprecation",
			"arguments": map[string]any{"service": "svc"},
		},
	})
	p := sourcesToolPayload(t, resp)
	findings, ok := p["findings"].([]any)
	if !ok {
		t.Fatalf("deprecation: findings missing: %v", p)
	}
	// goSrc has ioutil.ReadAll.
	if len(findings) == 0 {
		t.Error("deprecation: expected at least one finding for ioutil.ReadAll")
	}
}

func TestDeprecation_MissingService(t *testing.T) {
	srv := newSourcesSrv(nil)
	resp := roundtrip(t, srv, request{
		JSONRPC: "2.0", ID: 1061, Method: "tools/call",
		Params: map[string]any{
			"name":      "deprecation",
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

// ─── env_keys ─────────────────────────────────────────────────────────────────

func TestEnvKeys_Basic(t *testing.T) {
	files := []core.SourceFile{
		{Path: "foo/run.go", Content: []byte(goSrc), Lang: "go"},
		{Path: "bar/app.py", Content: []byte(pySrc), Lang: "py"},
		{Path: "client.js", Content: []byte(jsSrc), Lang: "js"},
	}
	srv := newSourcesSrv(files)
	resp := roundtrip(t, srv, request{
		JSONRPC: "2.0", ID: 1070, Method: "tools/call",
		Params: map[string]any{
			"name":      "env_keys",
			"arguments": map[string]any{"service": "svc"},
		},
	})
	p := sourcesToolPayload(t, resp)
	keys, ok := p["keys"].([]any)
	if !ok {
		t.Fatalf("env_keys: keys missing: %v", p)
	}
	// goSrc has DATABASE_URL, pySrc has DATABASE_URL, jsSrc has API_KEY.
	if len(keys) == 0 {
		t.Error("env_keys: expected at least one env key")
	}
}

func TestEnvKeys_MissingService(t *testing.T) {
	srv := newSourcesSrv(nil)
	resp := roundtrip(t, srv, request{
		JSONRPC: "2.0", ID: 1071, Method: "tools/call",
		Params: map[string]any{
			"name":      "env_keys",
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

// ─── markers / test_debt ──────────────────────────────────────────────────────

func TestMarkers_Basic(t *testing.T) {
	files := []core.SourceFile{
		// goSrc has a TODO comment.
		{Path: "foo/run.go", Content: []byte(goSrc), Lang: "go"},
	}
	srv := newSourcesSrv(files)
	resp := roundtrip(t, srv, request{
		JSONRPC: "2.0", ID: 1080, Method: "tools/call",
		Params: map[string]any{
			"name":      "markers",
			"arguments": map[string]any{"service": "svc"},
		},
	})
	p := sourcesToolPayload(t, resp)
	markers, ok := p["markers"].([]any)
	if !ok {
		t.Fatalf("markers: markers missing: %v", p)
	}
	// goSrc has a TODO → expect at least one marker.
	if len(markers) == 0 {
		t.Error("markers: expected at least one marker (TODO in goSrc)")
	}
}

func TestTestDebt_Basic(t *testing.T) {
	files := []core.SourceFile{
		{Path: "foo/run.go", Content: []byte(goSrc), Lang: "go"},
	}
	srv := newSourcesSrv(files)
	resp := roundtrip(t, srv, request{
		JSONRPC: "2.0", ID: 1081, Method: "tools/call",
		Params: map[string]any{
			"name":      "test_debt",
			"arguments": map[string]any{"service": "svc"},
		},
	})
	p := sourcesToolPayload(t, resp)
	if p["markers"] == nil {
		t.Fatalf("test_debt: markers missing: %v", p)
	}
}

func TestMarkers_MissingService(t *testing.T) {
	srv := newSourcesSrv(nil)
	resp := roundtrip(t, srv, request{
		JSONRPC: "2.0", ID: 1082, Method: "tools/call",
		Params: map[string]any{
			"name":      "markers",
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

// ─── routes / api_routes ──────────────────────────────────────────────────────

func TestRoutes_Basic(t *testing.T) {
	files := []core.SourceFile{
		{Path: "bar/app.py", Content: []byte(pySrc), Lang: "py"},
	}
	srv := newSourcesSrv(files)
	resp := roundtrip(t, srv, request{
		JSONRPC: "2.0", ID: 1090, Method: "tools/call",
		Params: map[string]any{
			"name":      "routes",
			"arguments": map[string]any{"service": "svc"},
		},
	})
	p := sourcesToolPayload(t, resp)
	routes, ok := p["routes"].([]any)
	if !ok {
		t.Fatalf("routes: routes missing: %v", p)
	}
	// pySrc has @app.route('/health') → expect one route.
	if len(routes) == 0 {
		t.Error("routes: expected at least one route from pySrc")
	}
}

func TestAPIRoutes_Basic(t *testing.T) {
	files := []core.SourceFile{
		{Path: "bar/app.py", Content: []byte(pySrc), Lang: "py"},
	}
	srv := newSourcesSrv(files)
	resp := roundtrip(t, srv, request{
		JSONRPC: "2.0", ID: 1091, Method: "tools/call",
		Params: map[string]any{
			"name":      "api_routes",
			"arguments": map[string]any{"service": "svc"},
		},
	})
	p := sourcesToolPayload(t, resp)
	if p["routes"] == nil {
		t.Fatalf("api_routes: routes missing: %v", p)
	}
}

func TestRoutes_MissingService(t *testing.T) {
	srv := newSourcesSrv(nil)
	resp := roundtrip(t, srv, request{
		JSONRPC: "2.0", ID: 1092, Method: "tools/call",
		Params: map[string]any{
			"name":      "routes",
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

// ─── endpoint_inventory ───────────────────────────────────────────────────────

func TestEndpointInventory_Basic(t *testing.T) {
	files := []core.SourceFile{
		{Path: "bar/app.py", Content: []byte(pySrc), Lang: "py"},
	}
	srv := newSourcesSrv(files)
	resp := roundtrip(t, srv, request{
		JSONRPC: "2.0", ID: 1100, Method: "tools/call",
		Params: map[string]any{
			"name":      "endpoint_inventory",
			"arguments": map[string]any{"service": "svc"},
		},
	})
	p := sourcesToolPayload(t, resp)
	endpoints, ok := p["endpoints"].([]any)
	if !ok {
		t.Fatalf("endpoint_inventory: endpoints missing: %v", p)
	}
	if len(endpoints) == 0 {
		t.Error("endpoint_inventory: expected at least one endpoint from pySrc")
	}
}

func TestEndpointInventory_MissingService(t *testing.T) {
	srv := newSourcesSrv(nil)
	resp := roundtrip(t, srv, request{
		JSONRPC: "2.0", ID: 1101, Method: "tools/call",
		Params: map[string]any{
			"name":      "endpoint_inventory",
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

// ─── dependencies / deps ──────────────────────────────────────────────────────

const goSumContent = `github.com/pkg/errors v0.9.1 h1:abc
github.com/pkg/errors v0.9.1/go.mod h1:def
`

func TestDependencies_Basic(t *testing.T) {
	files := []core.SourceFile{
		{Path: "go.sum", Content: []byte(goSumContent), Lang: "go"},
	}
	srv := newSourcesSrv(files)
	resp := roundtrip(t, srv, request{
		JSONRPC: "2.0", ID: 1110, Method: "tools/call",
		Params: map[string]any{
			"name":      "dependencies",
			"arguments": map[string]any{"service": "svc"},
		},
	})
	p := sourcesToolPayload(t, resp)
	deps, ok := p["dependencies"].([]any)
	if !ok {
		t.Fatalf("dependencies: dependencies missing: %v", p)
	}
	if len(deps) == 0 {
		t.Error("dependencies: expected at least one dependency from go.sum")
	}
}

func TestDeps_Basic(t *testing.T) {
	files := []core.SourceFile{
		{Path: "go.sum", Content: []byte(goSumContent), Lang: "go"},
	}
	srv := newSourcesSrv(files)
	resp := roundtrip(t, srv, request{
		JSONRPC: "2.0", ID: 1111, Method: "tools/call",
		Params: map[string]any{
			"name":      "deps",
			"arguments": map[string]any{"service": "svc"},
		},
	})
	p := sourcesToolPayload(t, resp)
	if p["dependencies"] == nil {
		t.Fatalf("deps: dependencies missing: %v", p)
	}
}

func TestDependencies_MissingService(t *testing.T) {
	srv := newSourcesSrv(nil)
	resp := roundtrip(t, srv, request{
		JSONRPC: "2.0", ID: 1112, Method: "tools/call",
		Params: map[string]any{
			"name":      "dependencies",
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

	toolsInProfile := func(profile string) map[string]bool {
		setResp := roundtrip(t, srv, request{
			JSONRPC: "2.0", ID: 1200, Method: "tools/call",
			Params: map[string]any{
				"name":      "set_profile",
				"arguments": map[string]any{"profile": profile},
			},
		})
		if setResp["error"] != nil {
			t.Fatalf("set_profile(%s): %v", profile, setResp["error"])
		}
		listResp := roundtrip(t, srv, request{
			JSONRPC: "2.0", ID: 1201, Method: "tools/list",
		})
		if listResp["error"] != nil {
			t.Fatalf("tools/list: %v", listResp["error"])
		}
		result := listResp["result"].(map[string]any)
		tools := result["tools"].([]any)
		m := make(map[string]bool, len(tools))
		for _, raw := range tools {
			m[raw.(map[string]any)["name"].(string)] = true
		}
		return m
	}

	sharedTools := toolsInProfile("shared")
	for _, name := range []string{"secret_scan", "shell_out", "http_client_calls"} {
		if !sharedTools[name] {
			t.Errorf("shared profile missing tool %q", name)
		}
	}

	codingTools := toolsInProfile("coding")
	for _, name := range []string{"antipatterns", "cognitive", "cloc", "deprecation", "env_keys"} {
		if !codingTools[name] {
			t.Errorf("coding profile missing tool %q", name)
		}
	}

	investigationTools := toolsInProfile("investigation")
	for _, name := range []string{"markers", "test_debt", "routes", "api_routes", "endpoint_inventory", "dependencies", "deps"} {
		if !investigationTools[name] {
			t.Errorf("investigation profile missing tool %q", name)
		}
	}
}

// ─── nil backend guard ────────────────────────────────────────────────────────

func TestSourceTools_NilBackend(t *testing.T) {
	srv := mcp.NewServer(nil)
	for _, toolName := range []string{
		"secret_scan", "shell_out", "http_client_calls",
		"antipatterns", "cognitive", "cloc", "deprecation", "env_keys",
		"markers", "test_debt", "routes", "api_routes", "endpoint_inventory",
		"dependencies", "deps",
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

// TestMarkerJSONShape verifies Marker fields.
func TestMarkerJSONShape(t *testing.T) {
	files := []core.SourceFile{
		{Path: "foo/run.go", Content: []byte(goSrc), Lang: "go"},
	}
	srv := newSourcesSrv(files)
	resp := roundtrip(t, srv, request{
		JSONRPC: "2.0", ID: 2020, Method: "tools/call",
		Params: map[string]any{
			"name":      "markers",
			"arguments": map[string]any{"service": "svc"},
		},
	})
	p := sourcesToolPayload(t, resp)
	markers, ok := p["markers"].([]any)
	if !ok || len(markers) == 0 {
		t.Skip("no markers to inspect shape")
	}
	m := markers[0].(map[string]any)
	for _, field := range []string{"file", "line", "kind", "text"} {
		if m[field] == nil {
			t.Errorf("marker missing field %q: %v", field, m)
		}
	}
}

// TestRoutesJSONShape verifies Route fields.
func TestRoutesJSONShape(t *testing.T) {
	files := []core.SourceFile{
		{Path: "bar/app.py", Content: []byte(pySrc), Lang: "py"},
	}
	srv := newSourcesSrv(files)
	resp := roundtrip(t, srv, request{
		JSONRPC: "2.0", ID: 2030, Method: "tools/call",
		Params: map[string]any{
			"name":      "routes",
			"arguments": map[string]any{"service": "svc"},
		},
	})
	p := sourcesToolPayload(t, resp)
	routes, ok := p["routes"].([]any)
	if !ok || len(routes) == 0 {
		t.Skip("no routes to inspect shape")
	}
	r := routes[0].(map[string]any)
	for _, field := range []string{"method", "path", "file", "line"} {
		if r[field] == nil {
			t.Errorf("route missing field %q: %v", field, r)
		}
	}
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
