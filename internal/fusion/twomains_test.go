package fusion_test

// TestTwoMainsSameFuncNameNeverCollapse is PC-32 (docs/plans/proof-credibility.md,
// O21): the reachfix fixture carries two `package main` binaries (cmd/a,
// cmd/b), each declaring `func run`, with a test reaching only cmd/a's.
// Before symbol keys carried the package DIRECTORY, both collapsed to the
// single key "reachfix::main.run" in callgraph.Build's node map — last
// writer wins — so the verdict for either depended on ingest order, and a
// third binary added anywhere in the tree could refute rows it never
// touched. The fixture is ingested end to end (the real syntax pass and the
// real typed pass) and the assertions hold on the syntax graph alone, on the
// combined graph, and on the node slice in REVERSED order.

import (
	"context"
	"path/filepath"
	"sort"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/analysis/callgraph"
	"github.com/geoffmilleraz/fathom/internal/connector"
	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/fusion"
	"github.com/geoffmilleraz/fathom/internal/store"
)

func TestTwoMainsSameFuncNameNeverCollapse(t *testing.T) {
	dir, err := filepath.Abs(filepath.Join("testdata", "reachfix"))
	if err != nil {
		t.Fatalf("abs: %v", err)
	}
	ctx := context.Background()
	st := store.NewMemoryStore()
	pipe := fusion.New(connector.NewLocalConnector(), st, fusion.Config{})
	if _, err := pipe.Ingest(ctx, core.ServiceRef{ID: "reachfix", Path: dir}); err != nil {
		t.Fatalf("Ingest: %v", err)
	}
	obs, err := st.QueryObservations(ctx, "reachfix", "")
	if err != nil {
		t.Fatalf("QueryObservations: %v", err)
	}
	var nodes []core.SymbolNode
	for _, o := range obs {
		nodes = append(nodes, o.ToSymbolNode())
	}
	typed, err := st.TypedEdges(ctx, "reachfix")
	if err != nil {
		t.Fatalf("TypedEdges: %v", err)
	}

	runA := core.SymbolKey("reachfix", "cmd/a/main", "run")
	runB := core.SymbolKey("reachfix", "cmd/b/main", "run")
	testA := core.SymbolKey("reachfix", "cmd/a/main", "TestRunA")

	// The two run funcs are distinct stored symbols with distinct keys.
	keys := map[string]int{}
	for _, n := range nodes {
		if n.Name == "run" {
			keys[n.Key()]++
		}
	}
	if keys[runA] != 1 || keys[runB] != 1 {
		t.Fatalf("expected exactly one node each for %s and %s, got %v", runA, runB, keys)
	}

	// The typed pass keys its edges the same way the syntax pass keys nodes:
	// TestRunA -> cmd/a's run is a typed edge, and no typed edge reaches
	// cmd/b's run.
	var typedToA, typedToB int
	for _, e := range typed {
		switch e.ToSymbol {
		case runA:
			typedToA++
		case runB:
			typedToB++
		}
	}
	if typedToA == 0 {
		t.Errorf("no typed edge targets %s — the typed pass keys diverged from the node keys", runA)
	}
	if typedToB == 0 {
		t.Errorf("no typed edge targets %s (main -> run is a direct call)", runB)
	}

	check := func(t *testing.T, label string, g *callgraph.Graph) {
		t.Helper()
		if !callgraph.Reachable(g, testA, runA) {
			t.Errorf("%s: TestRunA must reach cmd/a's run", label)
		}
		if callgraph.Reachable(g, testA, runB) {
			t.Errorf("%s: TestRunA must never reach cmd/b's run", label)
		}
		if got := callgraph.TestImpact(g, runA); len(got) != 1 || got[0] != testA {
			t.Errorf("%s: TestImpact(%s) = %v, want [%s]", label, runA, got, testA)
		}
		if got := callgraph.TestImpact(g, runB); len(got) != 0 {
			t.Errorf("%s: TestImpact(%s) = %v, want empty", label, runB, got)
		}
		// Each main reaches only its own run.
		mainA := core.SymbolKey("reachfix", "cmd/a/main", "main")
		mainB := core.SymbolKey("reachfix", "cmd/b/main", "main")
		if !callgraph.Reachable(g, mainA, runA) || callgraph.Reachable(g, mainA, runB) {
			t.Errorf("%s: cmd/a main must reach only cmd/a run", label)
		}
		if !callgraph.Reachable(g, mainB, runB) || callgraph.Reachable(g, mainB, runA) {
			t.Errorf("%s: cmd/b main must reach only cmd/b run", label)
		}
		// The ambiguous package-name form names nothing — never whichever
		// binary was ingested last.
		if got := callgraph.TestImpact(g, "reachfix::main.run"); len(got) != 0 {
			t.Errorf("%s: TestImpact(reachfix::main.run) = %v, want empty (ambiguous)", label, got)
		}
	}

	// Syntax graph, in stored order.
	check(t, "syntax", callgraph.Build(nodes))
	// Syntax graph, reversed order: the verdict must not depend on which
	// binary's observations arrive last.
	rev := make([]core.SymbolNode, len(nodes))
	for i, n := range nodes {
		rev[len(nodes)-1-i] = n
	}
	check(t, "syntax-reversed", callgraph.Build(rev))
	// Combined graph — exactly what a real service's ingest produces.
	g := callgraph.Build(nodes)
	callgraph.AddTypedEdges(g, typed)
	check(t, "combined", g)

	// Determinism: two builds over the same input agree edge for edge.
	a := callgraph.TestImpact(callgraph.Build(nodes), runA)
	b := callgraph.TestImpact(callgraph.Build(rev), runA)
	sort.Strings(a)
	sort.Strings(b)
	if len(a) != len(b) || (len(a) > 0 && a[0] != b[0]) {
		t.Errorf("TestImpact differs by input order: %v vs %v", a, b)
	}
}
