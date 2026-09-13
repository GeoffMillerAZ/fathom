// Package mcp_test — TDD tests for the search MCP tool (RC-24,
// docs/plans/receipts.md; ships SB-48).
package mcp_test

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/mcp"
)

// writeSearchFixture lays out a small tree under a temp root:
//
//	pkg/a.go        — func A() { B() } / func B() {}
//	pkg/a_test.go   — a Go test file (for the "test" flag)
//	vendor/v.go     — matches the pattern but must never be returned
//	workspace/w.go  — matches the pattern but must never be returned
func writeSearchFixture(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	files := map[string]string{
		"pkg/a.go":       "package pkg\n\nfunc A() {\n\tB()\n}\n\nfunc B() {\n}\n",
		"pkg/a_test.go":  "package pkg\n\nimport \"testing\"\n\nfunc TestA(t *testing.T) {\n\tA()\n}\n",
		"vendor/v.go":    "package vendor\n\nfunc A() {}\n",
		"workspace/w.go": "package workspace\n\nfunc A() {}\n",
	}
	for rel, content := range files {
		full := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

func searchFixtureNodes() []core.SymbolNode {
	return []core.SymbolNode{
		{Service: "svc", Package: "pkg", Name: "A", Kind: core.KindGoFunc, FilePath: "pkg/a.go", Line: 3, EndLine: 5},
		{Service: "svc", Package: "pkg", Name: "B", Kind: core.KindGoFunc, FilePath: "pkg/a.go", Line: 7, EndLine: 8},
	}
}

func newSearchServer(t *testing.T) (*mcp.Server, string) {
	t.Helper()
	root := writeSearchFixture(t)
	srv := mcp.NewServer(&extendedFakeBackend{
		nodes:    searchFixtureNodes(),
		services: []string{"svc"},
		svcPath:  root,
	})
	return srv, root
}

func TestSearchWithReceiptsFromService(t *testing.T) {
	srv, _ := newSearchServer(t)
	payload := callTool(t, srv, "search", map[string]any{"service": "svc", "pattern": `func [AB]\(`})
	hits, _ := payload["hits"].([]any)
	if len(hits) != 2 {
		t.Fatalf("got %d hits, want 2: %v", len(hits), payload)
	}
	engine, _ := payload["engine"].(string)
	if engine != "ripgrep" && engine != "regexp" {
		t.Fatalf("engine = %q, want ripgrep or regexp", engine)
	}
	first := hits[0].(map[string]any)
	if got := first["loc"]; got != "pkg/a.go:3" {
		t.Errorf("hits[0].loc = %v, want pkg/a.go:3", got)
	}
	if got := first["sym"]; got != "A" {
		t.Errorf("hits[0].sym = %v, want A (enclosing symbol from the index)", got)
	}
	if got := first["kind"]; got != "func" {
		t.Errorf("hits[0].kind = %v, want func", got)
	}
	second := hits[1].(map[string]any)
	if got := second["sym"]; got != "B" {
		t.Errorf("hits[1].sym = %v, want B", got)
	}
}

func TestSearchRootOnlyOmitsSymKindButKeepsFlags(t *testing.T) {
	_, root := newSearchServer(t)
	srv := mcp.NewServer(nil) // no backend at all — root mode must not need one
	payload := callTool(t, srv, "search", map[string]any{"root": root, "pattern": `func TestA`})
	hits, _ := payload["hits"].([]any)
	if len(hits) != 1 {
		t.Fatalf("got %d hits, want 1: %v", len(hits), payload)
	}
	h := hits[0].(map[string]any)
	if _, has := h["sym"]; has {
		t.Errorf("root-only search carries sym %v, want omitted (no index to ask)", h["sym"])
	}
	flags, _ := h["flags"].(string)
	if !strings.Contains(flags, "test") {
		t.Errorf("hits[0].flags = %q, want it to contain \"test\" (pkg/a_test.go)", flags)
	}
}

func TestSearchExcludesVendorAndWorkspaceByDefault(t *testing.T) {
	_, root := newSearchServer(t)
	srv := mcp.NewServer(nil)
	payload := callTool(t, srv, "search", map[string]any{"root": root, "pattern": `func A\(\)`})
	hits, _ := payload["hits"].([]any)
	for _, raw := range hits {
		h := raw.(map[string]any)
		loc, _ := h["loc"].(string)
		if strings.HasPrefix(loc, "vendor/") || strings.HasPrefix(loc, "workspace/") {
			t.Errorf("hit %v came from an excluded tree", loc)
		}
	}
	// Exactly the two non-excluded definitions of "func A()"-shaped lines:
	// pkg/a.go's "func A() {" — vendor/v.go and workspace/w.go both also
	// match the pattern and MUST be pruned.
	if len(hits) != 1 {
		t.Fatalf("got %d hits, want exactly 1 (vendor/workspace pruned): %v", len(hits), hits)
	}
}

func TestSearchPathGlob(t *testing.T) {
	_, root := newSearchServer(t)
	srv := mcp.NewServer(nil)
	payload := callTool(t, srv, "search", map[string]any{"root": root, "pattern": "package", "path": "pkg/*_test.go"})
	hits, _ := payload["hits"].([]any)
	if len(hits) != 1 {
		t.Fatalf("got %d hits, want 1 (path glob scoped to pkg/*_test.go): %v", len(hits), hits)
	}
	h := hits[0].(map[string]any)
	if loc, _ := h["loc"].(string); loc != "pkg/a_test.go:1" {
		t.Errorf("loc = %v, want pkg/a_test.go:1", loc)
	}
}

func TestSearchCaseInsensitive(t *testing.T) {
	_, root := newSearchServer(t)
	srv := mcp.NewServer(nil)
	// Default (case omitted) is case-sensitive: "FUNC A" must not match.
	payload := callTool(t, srv, "search", map[string]any{"root": root, "pattern": "FUNC A", "path": "pkg/a.go"})
	if hits, _ := payload["hits"].([]any); len(hits) != 0 {
		t.Fatalf("case-sensitive default matched %d hits for \"FUNC A\", want 0: %v", len(hits), hits)
	}
	payload = callTool(t, srv, "search", map[string]any{"root": root, "pattern": "FUNC A", "path": "pkg/a.go", "case": false})
	if hits, _ := payload["hits"].([]any); len(hits) == 0 {
		t.Fatalf("case: false found 0 hits for \"FUNC A\", want at least 1")
	}
}

func TestSearchContext(t *testing.T) {
	_, root := newSearchServer(t)
	srv := mcp.NewServer(nil)
	payload := callTool(t, srv, "search", map[string]any{"root": root, "pattern": `func A\(\)`, "path": "pkg/a.go", "context": 1})
	hits, _ := payload["hits"].([]any)
	if len(hits) != 1 {
		t.Fatalf("got %d hits, want 1: %v", len(hits), hits)
	}
	h := hits[0].(map[string]any)
	ctx, _ := h["ctx"].([]any)
	if len(ctx) != 2 {
		t.Fatalf("ctx = %v, want 2 lines (1 before + 1 after)", ctx)
	}
}

func TestSearchLimitAndElision(t *testing.T) {
	root := t.TempDir()
	var sb strings.Builder
	const total = 75
	for i := 0; i < total; i++ {
		sb.WriteString(fmt.Sprintf("needle line %d\n", i))
	}
	if err := os.WriteFile(filepath.Join(root, "many.txt"), []byte(sb.String()), 0o644); err != nil {
		t.Fatal(err)
	}
	srv := mcp.NewServer(nil)
	payload := callTool(t, srv, "search", map[string]any{"root": root, "pattern": "needle"})
	hits, _ := payload["hits"].([]any)
	if len(hits) != 50 {
		t.Fatalf("got %d hits, want the default limit 50", len(hits))
	}
	el, ok := payload["elided"].(map[string]any)
	if !ok {
		t.Fatalf("elided missing, want a block naming the %d hits past the limit: %v", total-50, payload)
	}
	if got := int(el["count"].(float64)); got != total-50 {
		t.Errorf("elided.count = %d, want %d", got, total-50)
	}
	if got := int(el["of"].(float64)); got != total {
		t.Errorf("elided.of = %d, want %d", got, total)
	}
	if el["how_to_get_more"] == "" {
		t.Error("elided.how_to_get_more is empty")
	}
	// limit narrows it directly.
	payload = callTool(t, srv, "search", map[string]any{"root": root, "pattern": "needle", "limit": 5})
	hits, _ = payload["hits"].([]any)
	if len(hits) != 5 {
		t.Fatalf("got %d hits with limit:5, want 5", len(hits))
	}
}

func TestSearchHitsSortedByPathThenLine(t *testing.T) {
	root := t.TempDir()
	files := map[string]string{
		"z.txt": "needle\n",
		"a.txt": "needle\nneedle\n",
	}
	for name, content := range files {
		if err := os.WriteFile(filepath.Join(root, name), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	srv := mcp.NewServer(nil)
	payload := callTool(t, srv, "search", map[string]any{"root": root, "pattern": "needle"})
	hits, _ := payload["hits"].([]any)
	if len(hits) != 3 {
		t.Fatalf("got %d hits, want 3", len(hits))
	}
	var locs []string
	for _, raw := range hits {
		locs = append(locs, raw.(map[string]any)["loc"].(string))
	}
	want := []string{"a.txt:1", "a.txt:2", "z.txt:1"}
	for i, w := range want {
		if locs[i] != w {
			t.Errorf("locs = %v, want %v", locs, want)
		}
	}
}

func TestSearchRequiresPatternAndExactlyOneOfServiceRoot(t *testing.T) {
	srv := mcp.NewServer(nil)
	cases := []map[string]any{
		{"root": "/tmp"}, // missing pattern
		{"pattern": "x"}, // neither service nor root
		{"pattern": "x", "service": "svc", "root": "/tmp"}, // both
	}
	for _, args := range cases {
		resp := roundtrip(t, srv, request{JSONRPC: "2.0", ID: 1, Method: "tools/call", Params: map[string]any{"name": "search", "arguments": args}})
		if resp["error"] == nil {
			t.Errorf("args %v: want an error, got success: %v", args, resp)
		}
	}
}

func TestSearchOnAgentSurface(t *testing.T) {
	on := false
	for _, name := range mcp.AgentSurfaceTools() {
		if name == "search" {
			on = true
		}
	}
	if !on {
		t.Fatal("search is not on the agent surface allowlist")
	}
}
