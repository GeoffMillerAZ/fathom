package mcp

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/memo"
)

// TestVerifyPlanGraphFallbackReadsUnknown pins O1162 (I38, I44): when the
// typed-edge read fails, verify_plan degrades to the syntax graph, names the
// fallback in a partial, and a reachability criterion that did not pass reads
// Unknown with the cause, never Fail. The branch that ran rides the
// verdict_outcome field.
func TestVerifyPlanGraphFallbackReadsUnknown(t *testing.T) {
	nodes := []core.SymbolNode{
		{Service: "svc", Package: "main", Name: "main", Kind: core.KindGoFunc, FilePath: "cmd/app/main.go", Line: 1, Tier: core.TierVerified},
		{Service: "svc", Package: "work", Name: "Run", Kind: core.KindGoFunc, FilePath: "work/run.go", Line: 1, Tier: core.TierVerified},
	}
	edges := []core.Edge{{FromService: "svc", FromFile: "cmd/app/main.go", FromSymbol: "svc::cmd/app/main.main", ToSymbol: "svc::work/work.Run", Kind: core.EdgeDirect}}
	call := func(t *testing.T, mark string, failTyped bool) map[string]any {
		t.Helper()
		prev := verifyTypedLoader
		t.Cleanup(func() { verifyTypedLoader = prev })
		if failTyped {
			verifyTypedLoader = func(QueryInterface, string) memo.TypedLoader {
				return func(context.Context) ([]core.Edge, error) { return nil, errors.New("refuse mark: read cut") }
			}
		}
		b := &coldGraphBackend{storeID: "o1162-" + t.Name(), mark: mark, nodes: nodes, edges: edges, typedReads: map[string]int{}}
		srv := NewServer(b)
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		raw, _ := json.Marshal(map[string]any{"jsonrpc": "2.0", "id": 1, "method": "tools/call",
			"params": map[string]any{"name": "verify_plan", "arguments": map[string]any{
				"service": "svc",
				"tasks":   []any{map[string]any{"id": "R-1", "symbol": "Run", "reachable_from": "main", "claimed": true, "skip_test": true, "skip": "fixture"}},
			}}})
		resp, ok := srv.HandleRequest(ctx, raw)
		if !ok {
			t.Fatal("no response")
		}
		var out rpcResp
		if err := json.Unmarshal(resp, &out); err != nil || out.Error != nil {
			t.Fatalf("%v %+v: %s", err, out.Error, resp)
		}
		content, _ := out.Result["content"].([]any)
		var m map[string]any
		if err := json.Unmarshal([]byte(content[0].(map[string]any)["text"].(string)), &m); err != nil {
			t.Fatal(err)
		}
		return m
	}
	edgeWired := func(m map[string]any) (status, detail string) {
		for _, tk := range m["tasks"].([]any) {
			for _, c := range tk.(map[string]any)["criteria"].([]any) {
				cm := c.(map[string]any)
				if cm["name"] == "edge_wired" {
					return cm["status"].(string), cm["detail"].(string)
				}
			}
		}
		return "", ""
	}

	typed := call(t, "w1", false)
	if st, d := edgeWired(typed); st != "pass" {
		t.Fatalf("typed graph: edge_wired = %s (%s), want pass", st, d)
	}
	if vo, _ := typed["verdict_outcome"].(map[string]any); vo == nil || vo["graph_fallback"] != float64(0) {
		t.Errorf("typed verdict_outcome = %v, want graph_fallback 0", typed["verdict_outcome"])
	}
	if _, ok := typed["partial"]; ok {
		t.Errorf("typed graph: unexpected partial %v", typed["partial"])
	}

	fb := call(t, "w2", true)
	st, d := edgeWired(fb)
	if st != "unknown" || !strings.Contains(d, "refuse mark: read cut") {
		t.Fatalf("fallback: edge_wired = %s (%s), want unknown naming the cause", st, d)
	}
	if p, _ := fb["partial"].(string); !strings.Contains(p, "typed call graph could not be read") || !strings.Contains(p, "refuse mark: read cut") {
		t.Errorf("fallback partial = %q", p)
	}
	if vo, _ := fb["verdict_outcome"].(map[string]any); vo == nil || vo["graph_fallback"] != float64(1) || vo["failing"] != float64(0) {
		t.Errorf("fallback verdict_outcome = %v, want graph_fallback 1, failing 0", fb["verdict_outcome"])
	}
	if agg, _ := fb["graph_fallback"].(map[string]any); agg == nil || agg["rows"] != float64(1) || agg["remedy"] == "" {
		t.Errorf("fallback aggregate = %v", fb["graph_fallback"])
	}
}
