package mcp_test

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/mcp"
)

// schemaFakeBackend serves every agent-surface tool well enough to produce
// a real result: symbols with edges, a corpus root on disk (read_source,
// memory_search), sources (references, staleness_report).
type schemaFakeBackend struct {
	reviewFakeBackend
}

func (s *schemaFakeBackend) FindSymbol(_ context.Context, _, name string) ([]core.SymbolNode, error) {
	var out []core.SymbolNode
	for _, n := range s.nodes {
		if n.Name == name {
			out = append(out, n)
		}
	}
	return out, nil
}

func (s *schemaFakeBackend) FindCallers(_ context.Context, _, symbol string) ([]core.SymbolNode, error) {
	var out []core.SymbolNode
	for _, n := range s.nodes {
		for _, c := range n.Calls {
			if c == "svc::pkg."+symbol || c == symbol {
				out = append(out, n)
			}
		}
	}
	return out, nil
}

// TestOutputSchemasMatchLiveResults (RP-11): every agent-surface tool
// advertises an outputSchema in tools/list, returns structuredContent
// beside its text block, and that live result CONFORMS to the schema it
// advertised — the spec's "Servers MUST provide structured results that
// conform to this schema", checked here rather than trusted.
func TestOutputSchemasMatchLiveResults(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "pkg"), 0o755); err != nil {
		t.Fatal(err)
	}
	src := []byte("package pkg\n\nfunc A() { B() }\nfunc B() {}\n")
	if err := os.WriteFile(filepath.Join(root, "pkg", "a.go"), src, 0o644); err != nil {
		t.Fatal(err)
	}
	fake := &schemaFakeBackend{reviewFakeBackend: reviewFakeBackend{
		extendedFakeBackend: extendedFakeBackend{nodes: extFixtureNodes(), services: []string{"svc"}, svcPath: root},
		sources:             map[string][]byte{"pkg/a.go": src},
	}}
	// list_services needs a fleet backend; the same fixture nodes serve it.
	srv := mcp.NewServerWithFleet(fake, &fakeFleet{svcs: map[string][]core.SymbolNode{"svc": extFixtureNodes()}})
	ctx := mcp.WithRequestScope(context.Background(), mcp.SurfaceAgent, true)

	listResp := scopedRoundtrip(t, srv, ctx, request{JSONRPC: "2.0", ID: 1, Method: "tools/list", Params: map[string]any{}})
	schemas := map[string]map[string]any{}
	for _, raw := range listResp["result"].(map[string]any)["tools"].([]any) {
		e := raw.(map[string]any)
		out, ok := e["outputSchema"].(map[string]any)
		if !ok {
			t.Errorf("%s: no outputSchema in tools/list", e["name"])
			continue
		}
		schemas[e["name"].(string)] = out
	}

	args := map[string]map[string]any{
		"capabilities":     {},
		"discoveries":      {"service": "svc"},
		"entrypoints":      {"service": "svc"},
		"find_callers":     {"service": "svc", "symbol": "B", "depth": 0},
		"find_symbol":      {"service": "svc", "name": "A"},
		"ingest":           {"service": "svc"},
		"list_services":    {},
		"memory_search":    {"service": "svc", "query": "A"},
		"obligations":      {"service": "svc", "file": "pkg/a.go"},
		"outline":          {"service": "svc", "file": "pkg/a.go"},
		"pack_context":     {"service": "svc", "concern": "billing"},
		"query_symbols":    {"service": "svc", "concern": "billing"},
		"read_source":      {"service": "svc", "file": "pkg/a.go"},
		"references":       {"query": "A", "word": true},
		"repo_map":         {"service": "svc"},
		"search":           {"service": "svc", "pattern": "func"},
		"staleness_report": {"service": "svc"},
		"symbol_context":   {"service": "svc", "symbol": "A"},
		"verify_plan":      {"service": "svc", "plan_markdown": "- [x] A exists {id: T1, symbol: A}"},
	}
	for _, name := range mcp.AgentSurfaceTools() {
		a, ok := args[name]
		if !ok {
			t.Errorf("%s: no call arguments in this test — add a row", name)
			continue
		}
		resp := scopedRoundtrip(t, srv, ctx, request{JSONRPC: "2.0", ID: 2, Method: "tools/call", Params: map[string]any{"name": name, "arguments": a}})
		if resp["error"] != nil {
			t.Errorf("%s: call failed: %v", name, resp["error"])
			continue
		}
		result := resp["result"].(map[string]any)
		sc, ok := result["structuredContent"].(map[string]any)
		if !ok {
			t.Errorf("%s: no structuredContent beside the text block", name)
			continue
		}
		// structuredContent and the text block are the same bytes.
		text := result["content"].([]any)[0].(map[string]any)["text"].(string)
		var fromText map[string]any
		if err := json.Unmarshal([]byte(text), &fromText); err != nil {
			t.Errorf("%s: text block not JSON: %v", name, err)
		}
		if scb, _ := json.Marshal(sc); string(scb) != mustCompact(t, text) {
			t.Errorf("%s: structuredContent differs from the text block", name)
		}
		if msg := mcp.ValidateAgainstSchemaForTest(schemas[name], sc); msg != "" {
			t.Errorf("%s: live result does not conform to its advertised outputSchema: %s\nresult: %s", name, msg, text)
		}
	}
}

func mustCompact(t *testing.T, s string) string {
	t.Helper()
	var v any
	if err := json.Unmarshal([]byte(s), &v); err != nil {
		t.Fatal(err)
	}
	b, _ := json.Marshal(v)
	return string(b)
}
