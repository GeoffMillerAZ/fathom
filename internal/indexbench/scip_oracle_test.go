package indexbench

// scip_oracle_test.go — SU-12's required offline test (docs/plans/
// substrate.md): runs ScipOracle over the SU-9 fixture (internal/scip/
// testdata/tsfixture) — fathom's own tsextract SYNTAX edges (no SCIP or
// native typed pass exists for TypeScript in this clone; the syntax call
// graph is what an agent's find_callers/blast_radius actually reads for a
// TypeScript service today) scored against the fixture's hand-built SCIP
// truth — and pins the numbers, so a regression in either ScipOracle's
// scoring or the fixture itself fails loudly here rather than silently
// drifting a bench artifact nobody re-derives by hand.

import (
	"os"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/analysis/callgraph"
	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/extractor"
	fscip "github.com/geoffmilleraz/fathom/internal/scip"
)

const scipOracleFixtureService = "tsfixture-svc"

// fathomSyntaxEdgesForFixture extracts the SU-9 TypeScript fixture with the
// REAL tsextract extractor and returns its syntax call graph as
// []core.Edge (Kind "" — no typed pass has run) — exactly what
// find_callers/blast_radius fall back to for a language with no typed
// edges yet (ADR-021 §3's documented fallback).
func fathomSyntaxEdgesForFixture(t *testing.T) []core.Edge {
	t.Helper()
	e, err := extractor.NewTSExtractor("javascript")
	if err != nil {
		t.Fatalf("NewTSExtractor: %v", err)
	}
	var nodes []core.SymbolNode
	for _, f := range []string{"math.ts", "greeter.ts", "main.ts"} {
		src, err := os.ReadFile("../scip/testdata/tsfixture/" + f)
		if err != nil {
			t.Fatalf("read %s: %v", f, err)
		}
		ns, err := e.Extract(scipOracleFixtureService, f, src)
		if err != nil {
			t.Fatalf("extract %s: %v", f, err)
		}
		nodes = append(nodes, ns...)
	}
	g := callgraph.Build(nodes)
	var edges []core.Edge
	for _, n := range nodes {
		for _, ci := range callgraph.CalleesDetail(g, n.Key()) {
			edges = append(edges, core.Edge{FromSymbol: n.Key(), ToSymbol: ci.Key, Kind: ci.Kind})
		}
	}
	return edges
}

func scipTruthEdgesForFixture(t *testing.T) []core.Edge {
	t.Helper()
	raw, err := os.ReadFile("../scip/testdata/tsfixture/index.scip")
	if err != nil {
		t.Fatalf("read index.scip: %v", err)
	}
	// O115: ImportIndex now maps every definition against the service's
	// own extracted symbol table rather than the SCIP index's own
	// DisplayName, so the fixture's real tsextract nodes are required here
	// too (fathomSyntaxEdgesForFixture's own extraction, reused).
	var nodes []core.SymbolNode
	e, err := extractor.NewTSExtractor("javascript")
	if err != nil {
		t.Fatalf("NewTSExtractor: %v", err)
	}
	for _, f := range []string{"math.ts", "greeter.ts", "main.ts"} {
		src, err := os.ReadFile("../scip/testdata/tsfixture/" + f)
		if err != nil {
			t.Fatalf("read %s: %v", f, err)
		}
		ns, err := e.Extract(scipOracleFixtureService, f, src)
		if err != nil {
			t.Fatalf("extract %s: %v", f, err)
		}
		nodes = append(nodes, ns...)
	}
	imp, err := fscip.ImportIndex(scipOracleFixtureService, raw, nodes)
	if err != nil {
		t.Fatalf("ImportIndex: %v", err)
	}
	var edges []core.Edge
	for _, pkg := range imp.Packages {
		edges = append(edges, pkg.Edges...)
	}
	return edges
}

// TestScipOracle_TSFixture pins ScipOracle's numbers over the SU-9 fixture.
//
// Fathom's syntax pass finds 2 edges: Greeter.greet->add (the call
// site tsextract's query captures) and run->Greeter.greet (the
// "g.greet(...)" method call) — but NOT run->Greeter, because
// "new Greeter()" is a constructor reference tsextract's syntax query
// never captures as a call site. The SCIP truth has 3 direct edges (those
// two, plus run->Greeter) and 1 unresolved edge (Greeter.greet's
// console.log reference, external to the fixture's own index) — 4 truth
// edges total, so the fixture's honest recall gap (2/3 on the direct row)
// is real, not a bug: it is exactly what "fathom has no typed pass for
// this language yet" looks like against an independent truth.
func TestScipOracle_TSFixture(t *testing.T) {
	fathomEdges := fathomSyntaxEdgesForFixture(t)
	if len(fathomEdges) != 2 {
		t.Fatalf("test setup: fathom syntax edges = %d, want 2 (fixture/extractor drifted)", len(fathomEdges))
	}
	truthEdges := scipTruthEdgesForFixture(t)
	if len(truthEdges) != 4 {
		t.Fatalf("test setup: scip truth edges = %d, want 4 (fixture drifted)", len(truthEdges))
	}

	rep := ScipOracle([]ScipSubject{{Language: "typescript", FathomEdges: fathomEdges, Truth: truthEdges}})
	rep.Corpus = "tsfixture"

	if rep.Oracle != "internal/indexbench/scip-oracle" {
		t.Errorf("Oracle = %q", rep.Oracle)
	}
	if len(rep.Rows) != 2 {
		t.Fatalf("Rows = %+v, want 2 (direct, unresolved)", rep.Rows)
	}

	direct := rep.Rows[0]
	unresolved := rep.Rows[1]
	if direct.EdgeKind != "direct" {
		direct, unresolved = unresolved, direct
	}

	want := ScipOracleRow{
		Language: "typescript", EdgeKind: "direct",
		Truth: 3, Predicted: 2, Correct: 2,
		Precision: 1.0, Recall: round4(2.0 / 3.0), UnresolvedRate: 0.25,
	}
	if direct != want {
		t.Errorf("direct row = %+v, want %+v", direct, want)
	}

	wantUnresolved := ScipOracleRow{
		Language: "typescript", EdgeKind: "unresolved",
		Truth: 1, Predicted: 0, Correct: 0,
		Precision: 0, Recall: 0, UnresolvedRate: 0.25,
	}
	if unresolved != wantUnresolved {
		t.Errorf("unresolved row = %+v, want %+v", unresolved, wantUnresolved)
	}
}
