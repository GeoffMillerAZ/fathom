package fusion_test

// TestReachabilityFixtureEndToEnd is VS-10 (docs/plans/verify-spine.md): the
// regression harness every future reachability claim runs against. It
// ingests internal/fusion/testdata/reachfix end to end (the real syntax
// extraction pass + the real EnrichTypedEdges pass), builds the combined
// graph (Build + AddTypedEdges — exactly what a real service's ingest
// produces), and asserts the reachability shapes VS-5/VS-6/VS-7 exist to
// guarantee: value-referenced HTTP handlers, a closure-only call, and two
// same-named methods that must never cross-reach.
//
// This test is itself the acceptance evidence for the row (a test cannot be
// reached by a test — docs/plans/verify-spine.md's VS-10 row is
// skip_test:true for exactly that reason).

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/analysis/callgraph"
	"github.com/geoffmilleraz/fathom/internal/connector"
	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/fusion"
	"github.com/geoffmilleraz/fathom/internal/store"
)

func TestReachabilityFixtureEndToEnd(t *testing.T) {
	dir, err := filepath.Abs(filepath.Join("testdata", "reachfix"))
	if err != nil {
		t.Fatalf("abs: %v", err)
	}

	ctx := context.Background()
	st := store.NewMemoryStore()
	pipe := fusion.New(connector.NewLocalConnector(), st, fusion.Config{})

	svc := core.ServiceRef{ID: "reachfix", Path: dir}
	if _, err := pipe.Ingest(ctx, svc); err != nil {
		t.Fatalf("Ingest: %v", err)
	}

	obs, err := st.QueryObservations(ctx, "reachfix", "")
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

	typed, err := st.TypedEdges(ctx, "reachfix")
	if err != nil {
		t.Fatalf("TypedEdges: %v", err)
	}
	if len(typed) == 0 {
		t.Fatal("no typed edges stored — EnrichTypedEdges did not run (check Ingest's Go-source/go.mod gate)")
	}

	graph := callgraph.Build(nodes)
	callgraph.AddTypedEdges(graph, typed)

	key := func(pkg, name string) string { return core.SymbolKey("reachfix", pkg, name) }
	contains := func(ss []string, want string) bool {
		for _, s := range ss {
			if s == want {
				return true
			}
		}
		return false
	}

	// VS-6: every handler registered as a VALUE (bare selector arg, keyed
	// route-table literal, type-conversion arg, middleware arg, method
	// value) is reached by the static server-building test.
	for _, tc := range []struct {
		pkg, name string
	}{
		{"reachfix", "handleA"},
		{"reachfix", "handleB"},
		{"reachfix", "handleC"},
		{"reachfix", "handleD"},
		{"server", "server.handleE"},
	} {
		reached := callgraph.TestImpact(graph, key(tc.pkg, tc.name))
		if !contains(reached, key("reachfix", "TestServerServes")) {
			t.Errorf("TestImpact(%s) = %v, want to contain TestServerServes", tc.name, reached)
		}
	}

	// VS-7: a call made only inside a t.Run closure is attributed to the
	// enclosing named test function.
	reachedClosure := callgraph.TestImpact(graph, key("reachfix", "onlyInClosure"))
	if !contains(reachedClosure, key("reachfix", "TestClosureOnly")) {
		t.Errorf("TestImpact(onlyInClosure) = %v, want to contain TestClosureOnly", reachedClosure)
	}

	// VS-5: same-named methods on different receivers never cross-reach.
	reachedA := callgraph.TestImpact(graph, key("A", "A.Extract"))
	if !contains(reachedA, key("reachfix", "TestAExtract")) {
		t.Errorf("TestImpact(A.Extract) = %v, want to contain TestAExtract", reachedA)
	}
	reachedB := callgraph.TestImpact(graph, key("B", "B.Extract"))
	if len(reachedB) != 0 {
		t.Errorf("TestImpact(B.Extract) = %v, want empty — TestAExtract must never reach B.Extract", reachedB)
	}
}
