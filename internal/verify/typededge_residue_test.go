package verify_test

// TestO9Residue_RealFixtureIngestAndVerify is the CT-4 full-stack proof
// (ADR-021 §3): fusion.Pipeline.Ingest (real extraction) + fusion.
// EnrichTypedEdges (real go/packages resolution) + callgraph.AddTypedEdges +
// verify.VerifyPlan, end to end against a real Go module fixture — no
// hand-built core.Edge values anywhere in this test. It reproduces the
// three real O9 residue shapes (adapter-style Complete, Store
// contract-suite dispatch, Ping) and asserts each flips from the old
// syntax-only refusal (graph.go:218-240) to a typed-dispatch reach.

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/analysis/callgraph"
	"github.com/geoffmilleraz/fathom/internal/connector"
	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/fusion"
	"github.com/geoffmilleraz/fathom/internal/store"
	"github.com/geoffmilleraz/fathom/internal/verify"
)

// residueEvidence adapts a real store + callgraph to verify.DetailedEvidence
// — a minimal from-scratch implementation (no MCP/CLI wiring needed) so this
// test depends only on internal/{core,store,fusion,analysis/callgraph,verify}.
type residueEvidence struct {
	nodes []core.SymbolNode
	graph *callgraph.Graph
}

func (e *residueEvidence) FindSymbol(name string) []core.SymbolNode {
	var out []core.SymbolNode
	for _, n := range e.nodes {
		if n.Name == name {
			out = append(out, n)
		}
	}
	return out
}

func (e *residueEvidence) TestReaches(symbol string) []string {
	return callgraph.TestImpact(e.graph, symbol)
}

func (e *residueEvidence) TestReachesDetail(symbol string) []verify.ReachDetail {
	details := callgraph.TestImpactDetail(e.graph, symbol)
	out := make([]verify.ReachDetail, len(details))
	for i, d := range details {
		out[i] = verify.ReachDetail{Key: d.Key, ViaDispatch: d.ViaDispatch, Implementers: d.Implementers}
	}
	return out
}

func (e *residueEvidence) Reachable(from, to string) bool {
	return callgraph.Reachable(e.graph, from, to)
}
func (e *residueEvidence) Contradictions() []core.Contradiction    { return nil }
func (e *residueEvidence) SpecImplementors(specID string) []string { return nil }

var _ verify.DetailedEvidence = (*residueEvidence)(nil)

func TestO9Residue_RealFixtureIngestAndVerify(t *testing.T) {
	dir, err := filepath.Abs(filepath.Join("..", "fusion", "testdata", "typedfixture"))
	if err != nil {
		t.Fatalf("abs: %v", err)
	}

	ctx := context.Background()
	st := store.NewMemoryStore()
	pipe := fusion.New(connector.NewLocalConnector(), st, fusion.Config{})

	svc := core.ServiceRef{ID: "typedfixture", Path: dir}
	if _, err := pipe.Ingest(ctx, svc); err != nil {
		t.Fatalf("Ingest: %v", err)
	}

	obs, err := st.QueryObservations(ctx, "typedfixture", "")
	if err != nil {
		t.Fatalf("QueryObservations: %v", err)
	}
	if len(obs) == 0 {
		t.Fatal("no observations stored — Ingest found no Go source at the fixture path")
	}
	nodes := make([]core.SymbolNode, len(obs))
	for i, o := range obs {
		nodes[i] = o.ToSymbolNode()
	}

	// Before the typed pass: the syntax-only graph refuses every ambiguous
	// method name (graph.go:218-240) — prove the OLD behavior really was a
	// refusal, so the "flip" below is meaningful, not vacuous.
	syntaxOnly := callgraph.Build(nodes)
	for _, target := range []string{"AdapterA.Complete", "MemStore.PutClaims", "LocalPing.Ping"} {
		if reached := callgraph.TestImpact(syntaxOnly, target); len(reached) != 0 {
			t.Fatalf("syntax-only graph reaches %s = %v; want empty (the O9 refusal this test proves gets dissolved)", target, reached)
		}
	}
	// O40's own shape (direct call on a concrete receiver, not through the
	// interface): TestCallMemStoreDirect -> CallMemStoreDirect ->
	// MemStore.PutClaims. The syntax-only graph must not reach it either —
	// same ambiguous-suffix refusal, one hop closer to a real caller.
	if !callgraph.Reachable(syntaxOnly, "TestCallMemStoreDirect", "CallMemStoreDirect") {
		t.Fatal("syntax-only graph does not even reach CallMemStoreDirect from its own Test* caller — fixture wiring is broken, not the thing under test")
	}
	if callgraph.Reachable(syntaxOnly, "TestCallMemStoreDirect", "MemStore.PutClaims") {
		t.Fatal("syntax-only graph reaches MemStore.PutClaims from TestCallMemStoreDirect; want false (the O40 refusal this test proves gets dissolved)")
	}

	typed, err := st.TypedEdges(ctx, "typedfixture")
	if err != nil {
		t.Fatalf("TypedEdges: %v", err)
	}
	if len(typed) == 0 {
		t.Fatal("no typed edges stored — EnrichTypedEdges did not run (check Ingest's Go-source/go.mod gate)")
	}

	graph := callgraph.Build(nodes)
	callgraph.AddTypedEdges(graph, typed)
	ev := &residueEvidence{nodes: nodes, graph: graph}

	// O40: after AddTypedEdges, the direct concrete-receiver call resolves
	// too — plain reachability (edge_wired's own check), not test_reaches'
	// dispatch-detail path, since this is a direct edge, not dispatch.
	if !ev.Reachable("TestCallMemStoreDirect", "MemStore.PutClaims") {
		t.Fatal("typed graph does not reach MemStore.PutClaims from TestCallMemStoreDirect; want true (O40 fixed)")
	}

	cases := []struct {
		name   string
		symbol string
		test   string
	}{
		{"adapter-style Complete", "AdapterA.Complete", "TestRunAdapter"},
		{"Store contract-suite dispatch", "MemStore.PutClaims", "TestRunContract"},
		{"Ping", "LocalPing.Ping", "TestCheckPing"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			plan := verify.Plan{Tasks: []verify.Task{{
				ID: "T", Title: tc.name, Claimed: true,
				Expect: verify.Expect{Symbol: tc.symbol},
			}}}
			rep := verify.VerifyPlan(plan, ev)
			tv := rep.Tasks[0]
			if tv.Status != verify.Pass {
				t.Fatalf("%s: status = %v, want Pass: %+v", tc.symbol, tv.Status, tv)
			}
			var detail string
			for _, c := range tv.Criteria {
				if c.Name == "test_reaches" {
					detail = c.Detail
				}
			}
			if detail == "" || !strings.Contains(detail, "reached via interface dispatch") || !strings.Contains(detail, tc.test) {
				t.Errorf("%s: test_reaches detail = %q, want it to mention interface dispatch and %s", tc.symbol, detail, tc.test)
			}
		})
	}
}
