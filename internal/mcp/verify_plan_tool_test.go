package mcp_test

import (
	"context"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/mcp"
)

// verifyFakeBackend is a QueryInterface fake with a small fixture: a verified,
// tested symbol (DoWork), an untested one (Untested), an entrypoint (Router)
// that reaches DoWork, and a contradicted-but-tested symbol (Conflicted).
type verifyFakeBackend struct {
	nodes   []core.SymbolNode
	contras []core.Contradiction
}

func newVerifyBackend() *verifyFakeBackend {
	return &verifyFakeBackend{
		nodes: []core.SymbolNode{
			{Service: "svc", Package: "work", Name: "DoWork", Kind: core.KindGoFunc, FilePath: "work/do.go", Line: 10, Tier: core.TierVerified},
			{Service: "svc", Package: "work", Name: "TestDoWork", Kind: core.KindGoFunc, FilePath: "work/do_test.go", Line: 5, Calls: []string{"DoWork"}, Tier: core.TierVerified},
			{Service: "svc", Package: "api", Name: "Router", Kind: core.KindGoFunc, FilePath: "api/router.go", Line: 3, Calls: []string{"DoWork"}, Tier: core.TierVerified},
			{Service: "svc", Package: "work", Name: "Untested", Kind: core.KindGoFunc, FilePath: "work/untested.go", Line: 1, Tier: core.TierVerified},
			{Service: "svc", Package: "work", Name: "Conflicted", Kind: core.KindGoFunc, FilePath: "work/c.go", Line: 1, Tier: core.TierVerified},
			{Service: "svc", Package: "work", Name: "TestConflicted", Kind: core.KindGoFunc, FilePath: "work/c_test.go", Line: 1, Calls: []string{"Conflicted"}, Tier: core.TierVerified},
		},
		contras: []core.Contradiction{{
			Symbol: "Conflicted",
			A:      core.Observation{SourceRef: "work/c.go"},
			B:      core.Observation{SourceRef: "docs/c.md"},
		}},
	}
}

func (f *verifyFakeBackend) QuerySymbols(_ context.Context, _ core.Query) (core.QueryResult, error) {
	return core.QueryResult{Contradictions: f.contras}, nil
}

func (f *verifyFakeBackend) FindCallers(_ context.Context, _, _ string) ([]core.SymbolNode, error) {
	return nil, nil
}

func (f *verifyFakeBackend) FindSymbol(_ context.Context, _, name string) ([]core.SymbolNode, error) {
	var out []core.SymbolNode
	for _, n := range f.nodes {
		if n.Name == name {
			out = append(out, n)
		}
	}
	return out, nil
}

func (f *verifyFakeBackend) ListAllSymbols(_ context.Context, _ string) ([]core.SymbolNode, error) {
	return f.nodes, nil
}

func (f *verifyFakeBackend) Services(_ context.Context) ([]string, error) {
	return []string{"svc"}, nil
}

func (f *verifyFakeBackend) Sources(_ context.Context, _ string) ([]core.SourceFile, error) {
	return nil, nil
}
func (f *verifyFakeBackend) IngestService(_ context.Context, _ string) error { return nil }
func (f *verifyFakeBackend) ServiceMeta(_ context.Context, _ string) (string, string, bool, error) {
	return "", "", false, nil
}

func (f *verifyFakeBackend) ListCommits(_ context.Context, _ string) ([]mcp.CommitInfo, error) {
	return nil, nil
}

func TestVerifyPlan_Tool(t *testing.T) {
	srv := mcp.NewServer(newVerifyBackend())
	resp := roundtrip(t, srv, request{
		JSONRPC: "2.0", ID: 3001, Method: "tools/call",
		Params: map[string]any{
			"name": "verify_plan",
			"arguments": map[string]any{
				"service": "svc",
				"concern": "work",
				"tasks": []any{
					map[string]any{"id": "T-1", "title": "do work", "symbol": "DoWork"},
					map[string]any{"id": "T-2", "title": "untested", "symbol": "Untested"},
					map[string]any{"id": "T-3", "title": "ghost", "symbol": "Ghost"},
					map[string]any{"id": "T-4", "title": "wired", "symbol": "DoWork", "reachable_from": "Router"},
					map[string]any{"id": "T-5", "title": "conflict", "symbol": "Conflicted"},
					// T-6: claimed done — should surface claimed=true in output.
					map[string]any{"id": "T-6", "title": "claimed done", "symbol": "DoWork", "claimed": true},
					// T-7: skipped — should surface skipped=true in output.
					map[string]any{"id": "T-7", "title": "skipped item", "symbol": "DoWork", "skipped": true},
				},
			},
		},
	})
	p := xtraPayload(t, resp)

	// Index tasks by id.
	byID := map[string]map[string]any{}
	for _, ti := range p["tasks"].([]any) {
		tm := ti.(map[string]any)
		byID[tm["id"].(string)] = tm
	}

	want := map[string][2]string{
		"T-1": {"pass", "verified"},
		"T-2": {"fail", "present-but-untested"},
		"T-3": {"fail", "claimed-but-absent"},
		"T-4": {"pass", "verified"},
		"T-5": {"fail", "contradicted"},
		"T-6": {"pass", "verified"},
		"T-7": {"pass", "verified"},
	}
	for id, exp := range want {
		tm, ok := byID[id]
		if !ok {
			t.Fatalf("task %s missing from result", id)
		}
		if got := tm["status"].(string); got != exp[0] {
			t.Errorf("%s: status = %q, want %q", id, got, exp[0])
		}
		if got := tm["label"].(string); got != exp[1] {
			t.Errorf("%s: label = %q, want %q", id, got, exp[1])
		}
	}

	// T-6 must emit claimed=true; T-7 must emit skipped=true.
	if byID["T-6"]["claimed"] != true {
		t.Errorf("T-6 should have claimed=true in output, got %v", byID["T-6"]["claimed"])
	}
	if byID["T-7"]["skipped"] != true {
		t.Errorf("T-7 should have skipped=true in output, got %v", byID["T-7"]["skipped"])
	}
	// T-1 has no claimed/skipped set — neither key should be present.
	if _, ok := byID["T-1"]["claimed"]; ok {
		t.Errorf("T-1 should not have 'claimed' key when false")
	}

	// runtime_pass is always reported as unknown — the honesty boundary.
	t1 := byID["T-1"]
	var sawRuntimeUnknown bool
	for _, ci := range t1["criteria"].([]any) {
		c := ci.(map[string]any)
		if c["name"] == "runtime_pass" && c["status"] == "unknown" {
			sawRuntimeUnknown = true
		}
	}
	if !sawRuntimeUnknown {
		t.Error("T-1 must report runtime_pass=unknown")
	}

	summary := p["summary"].(map[string]any)
	if summary["total"].(float64) != 7 {
		t.Errorf("summary.total = %v, want 7", summary["total"])
	}
}
