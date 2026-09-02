package mcp_test

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/mcp"
)

// verifyFakeBackend is a QueryInterface fake with a small fixture: a verified,
// tested symbol (DoWork), an untested one (Untested), an entrypoint (Router)
// that reaches DoWork, a contradicted-but-tested symbol (Conflicted), and the
// ADR-021 §3 O9 residue shape: RunAdapter dispatches to Complete on two
// receivers (AdapterA, AdapterB) — an ambiguous method name the syntax-only
// resolver refuses (no Calls edge is even attempted here), reachable ONLY
// through typedEdges.
type verifyFakeBackend struct {
	nodes      []core.SymbolNode
	contras    []core.Contradiction
	typedEdges []core.Edge
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
			{Service: "svc", Package: "work", Name: "RunAdapter", Kind: core.KindGoFunc, FilePath: "work/adapter.go", Line: 1, Tier: core.TierVerified},
			{Service: "svc", Package: "work", Name: "TestRunAdapter", Kind: core.KindGoFunc, FilePath: "work/adapter_test.go", Line: 1, Calls: []string{"RunAdapter"}, Tier: core.TierVerified},
			{Service: "svc", Package: "AdapterA", Name: "AdapterA.Complete", Kind: core.KindGoMethod, FilePath: "work/adaptera.go", Line: 1, Tier: core.TierVerified},
			{Service: "svc", Package: "AdapterB", Name: "AdapterB.Complete", Kind: core.KindGoMethod, FilePath: "work/adapterb.go", Line: 1, Tier: core.TierVerified},
		},
		contras: []core.Contradiction{{
			Symbol: "Conflicted",
			A:      core.Observation{SourceRef: "work/c.go"},
			B:      core.Observation{SourceRef: "docs/c.md"},
		}},
		typedEdges: []core.Edge{
			{FromService: "svc", FromFile: "work/adapter.go", FromSymbol: "svc::work/work.RunAdapter", ToSymbol: "svc::work/AdapterA.AdapterA.Complete", Kind: core.EdgeIfaceDispatch},
			{FromService: "svc", FromFile: "work/adapter.go", FromSymbol: "svc::work/work.RunAdapter", ToSymbol: "svc::work/AdapterB.AdapterB.Complete", Kind: core.EdgeIfaceDispatch},
		},
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

func (f *verifyFakeBackend) TypedEdges(_ context.Context, _ string) ([]core.Edge, error) {
	return f.typedEdges, nil
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

// TestVerifyPlan_Tool_TypedEdgeDispatch is the CT-4 residue-flip proof
// (ADR-021 §3), exercised through the real verify_plan MCP tool end to end
// (not just the internal/verify unit test): a claim on AdapterA.Complete —
// reachable ONLY via a typed interface-dispatch edge, never via the
// syntax-only graph (RunAdapter has no Calls entry naming "Complete" at
// all, mirroring the ambiguous-method-name refusal graph.go:218-240 always
// gave) — now verifies Pass, with the dispatch modality named in the detail.
func TestVerifyPlan_Tool_TypedEdgeDispatch(t *testing.T) {
	srv := mcp.NewServer(newVerifyBackend())
	resp := roundtrip(t, srv, request{
		JSONRPC: "2.0", ID: 3002, Method: "tools/call",
		Params: map[string]any{
			"name": "verify_plan",
			"arguments": map[string]any{
				"service": "svc",
				"tasks": []any{
					map[string]any{"id": "T-1", "title": "adapter residue", "symbol": "AdapterA.Complete", "claimed": true},
				},
			},
		},
	})
	p := xtraPayload(t, resp)
	tasks := p["tasks"].([]any)
	if len(tasks) != 1 {
		t.Fatalf("tasks = %d, want 1", len(tasks))
	}
	tm := tasks[0].(map[string]any)
	if got := tm["status"].(string); got != "pass" {
		t.Fatalf("status = %q, want pass: %+v", got, tm)
	}
	if got := tm["label"].(string); got != "verified" {
		t.Errorf("label = %q, want verified", got)
	}
	var detail string
	for _, ci := range tm["criteria"].([]any) {
		c := ci.(map[string]any)
		if c["name"] == "test_reaches" {
			detail = c["detail"].(string)
		}
	}
	if detail == "" {
		t.Fatal("no test_reaches criterion in output")
	}
	if !strings.Contains(detail, "reached via interface dispatch") {
		t.Errorf("test_reaches detail = %q, want it to mention interface dispatch", detail)
	}
	if !strings.Contains(detail, "2 implementer") {
		t.Errorf("test_reaches detail = %q, want it to mention 2 implementers", detail)
	}
	if !strings.Contains(detail, "TestRunAdapter") {
		t.Errorf("test_reaches detail = %q, want it to name the reaching test", detail)
	}
}

// TestVerifyPlan_VerdictOutcomeReachesCallLog is T5-7's real evidence
// (docs/plans/trust-and-tokenomics.md): the gate-catches instrument is the
// JSONL call log, not just the tool result — mirrors D1-2's
// TestBrief_ClaimsAttachedReachesCallLog (extra_tools_test.go) exactly,
// generalized from a scalar (claims_attached) to named counts
// (verdict_outcome). Covers both write paths: the top-level dispatch
// (loggedToolsCall, reading the field back out of the wrapped MCP content
// envelope) and the batch sub-call path (logSub, reading the raw handler
// result map).
func TestVerifyPlan_VerdictOutcomeReachesCallLog(t *testing.T) {
	// T-1 claimed+verified, T-2 claimed+failing, T-3 unclaimed (must not
	// count) — claimed:2 verified:1 failing:1 over exactly the tasks
	// internal/verify.Gate itself would gate on.
	tasks := []any{
		map[string]any{"id": "T-1", "title": "do work", "symbol": "DoWork", "claimed": true},
		map[string]any{"id": "T-2", "title": "untested", "symbol": "Untested", "claimed": true},
		map[string]any{"id": "T-3", "title": "ghost", "symbol": "Ghost"},
	}
	args := map[string]any{"service": "svc", "tasks": tasks}

	t.Run("top_level", func(t *testing.T) {
		var buf bytes.Buffer
		srv := mcp.NewServerWithLogger(newVerifyBackend(), mcp.NewCallLogger(&buf))
		resp := roundtrip(t, srv, request{
			JSONRPC: "2.0", ID: 3101, Method: "tools/call",
			Params: map[string]any{"name": "verify_plan", "arguments": args},
		})
		p := xtraPayload(t, resp)
		vo, ok := p["verdict_outcome"].(map[string]any)
		if !ok {
			t.Fatalf("result payload missing verdict_outcome (fixture broken?): %v", p)
		}
		if vo["claimed"] != float64(2) || vo["verified"] != float64(1) || vo["failing"] != float64(1) {
			t.Fatalf("result verdict_outcome = %v, want claimed:2 verified:1 failing:1", vo)
		}

		entry := callLogEntryFor(t, &buf, "verify_plan", "")
		logged, ok := entry["verdict_outcome"].(map[string]any)
		if !ok {
			t.Fatalf("logged entry missing verdict_outcome: %v", entry)
		}
		if logged["claimed"] != float64(2) || logged["verified"] != float64(1) || logged["failing"] != float64(1) {
			t.Errorf("logged verdict_outcome = %v, want claimed:2 verified:1 failing:1", logged)
		}
	})

	t.Run("via_batch", func(t *testing.T) {
		var buf bytes.Buffer
		srv := mcp.NewServerWithLogger(newVerifyBackend(), mcp.NewCallLogger(&buf))
		resp := roundtrip(t, srv, request{
			JSONRPC: "2.0", ID: 3102, Method: "tools/call",
			Params: map[string]any{
				"name": "batch",
				"arguments": map[string]any{
					"calls": []map[string]any{{"name": "verify_plan", "arguments": args}},
				},
			},
		})
		if resp["error"] != nil {
			t.Fatalf("unexpected error: %v", resp["error"])
		}
		entry := callLogEntryFor(t, &buf, "verify_plan", "batch")
		logged, ok := entry["verdict_outcome"].(map[string]any)
		if !ok {
			t.Fatalf("batch sub-call logged entry missing verdict_outcome: %v", entry)
		}
		if logged["claimed"] != float64(2) || logged["verified"] != float64(1) || logged["failing"] != float64(1) {
			t.Errorf("batch logged verdict_outcome = %v, want claimed:2 verified:1 failing:1", logged)
		}
	})

	t.Run("no_claimed_tasks_omits_field", func(t *testing.T) {
		var buf bytes.Buffer
		srv := mcp.NewServerWithLogger(newVerifyBackend(), mcp.NewCallLogger(&buf))
		resp := roundtrip(t, srv, request{
			JSONRPC: "2.0", ID: 3103, Method: "tools/call",
			Params: map[string]any{
				"name": "verify_plan",
				"arguments": map[string]any{
					"service": "svc",
					"tasks": []any{
						map[string]any{"id": "T-1", "title": "unclaimed", "symbol": "DoWork"},
					},
				},
			},
		})
		p := xtraPayload(t, resp)
		if _, present := p["verdict_outcome"]; present {
			t.Errorf("verdict_outcome should be absent with zero claimed tasks, got: %v", p["verdict_outcome"])
		}
		entry := callLogEntryFor(t, &buf, "verify_plan", "")
		if _, present := entry["verdict_outcome"]; present {
			t.Errorf("logged entry should have no verdict_outcome field, got: %v", entry)
		}
	})
}
