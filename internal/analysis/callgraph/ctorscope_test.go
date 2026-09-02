package callgraph_test

import (
	"reflect"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/analysis/callgraph"
	"github.com/geoffmilleraz/fathom/internal/core"
)

// FIX 3 (resolver half): the Go extractor records a method call on a receiver
// variable whose initializer is a constructor-shaped selector call into an
// imported package as "<import-path>#.Method" (see GoExtractor.Extract),
// alongside the legacy "<var>.Method" form. The resolver scopes the method
// lookup to that package: by Go import mechanics the import path resolves to
// exactly one directory, so a node belongs to the package iff the import path
// ends with "/"+dir(FilePath). Exactly one method node in that package whose
// name ends in ".Method" → edge; zero or more than one → NOTHING (the honesty
// rule, package-scoped — never first-match-wins, never a fall-through to the
// global suffix rule for the encoded form). The legacy "<var>.Method" entry
// keeps the global unique-suffix method fallback as the secondary path.

// TestCtorScopedMethodEdge_UniqueInPackageResolves pins the real failing
// claim — evidence-spine {symbol: Run, package: foundations/evidence/
// evidence-ladder}: seven *.Run method nodes exist repo-wide so the global
// unique-suffix rule correctly refuses "l.Run", but within evidence-ladder
// there is exactly one, and the test's `l := ladder.New(...)` initializer
// proves which package the receiver came from.
func TestCtorScopedMethodEdge_UniqueInPackageResolves(t *testing.T) {
	nodes := []core.SymbolNode{
		{
			Service: "svc", Package: "Ladder", Name: "Ladder.Run",
			Kind: core.KindGoMethod, FilePath: "foundations/evidence/evidence-ladder/types.go",
		},
		{
			// A second .Run method elsewhere: the global unique-suffix
			// fallback must refuse, so only the package-scoped rule can
			// produce this edge.
			Service: "svc", Package: "Harness", Name: "Harness.Run",
			Kind: core.KindGoMethod, FilePath: "kits/agent-harness/harness.go",
		},
		{
			Service: "svc", Package: "evidenceladder_test", Name: "TestLadderRun",
			Kind: core.KindGoFunc, FilePath: "foundations/evidence/evidence-ladder/ladder_test.go",
			Calls: []string{
				"l.Run", // legacy form: globally ambiguous here, resolves to nothing
				"example.com/kit/foundations/evidence/evidence-ladder#.Run",
			},
		},
	}
	g := callgraph.Build(nodes)

	want := []string{"svc::foundations/evidence/evidence-ladder/evidenceladder_test.TestLadderRun"}
	if got := callgraph.TestImpact(g, "svc::Ladder.Ladder.Run"); !reflect.DeepEqual(got, want) {
		t.Errorf("TestImpact(Ladder.Run) = %v, want %v", got, want)
	}
	if got := callgraph.TestImpact(g, "svc::Harness.Harness.Run"); len(got) != 0 {
		t.Errorf("TestImpact(Harness.Run) = %v, want [] (constructor provenance points at evidence-ladder)", got)
	}
}

// TestCtorScopedMethodEdge_AmbiguousWithinPackageResolvesToNothing: two
// receivers in the SAME package both defining Run make the package-scoped
// lookup ambiguous — nothing, never first-match-wins.
func TestCtorScopedMethodEdge_AmbiguousWithinPackageResolvesToNothing(t *testing.T) {
	nodes := []core.SymbolNode{
		{
			Service: "svc", Package: "Ladder", Name: "Ladder.Run",
			Kind: core.KindGoMethod, FilePath: "foundations/evidence/evidence-ladder/types.go",
		},
		{
			Service: "svc", Package: "Replay", Name: "Replay.Run",
			Kind: core.KindGoMethod, FilePath: "foundations/evidence/evidence-ladder/replay.go",
		},
		{
			Service: "svc", Package: "evidenceladder_test", Name: "TestRun",
			Kind: core.KindGoFunc, FilePath: "foundations/evidence/evidence-ladder/ladder_test.go",
			Calls: []string{"example.com/kit/foundations/evidence/evidence-ladder#.Run"},
		},
	}
	g := callgraph.Build(nodes)
	if g.EdgeCount() != 0 {
		t.Errorf("EdgeCount() = %d, want 0 (two .Run methods within the package)", g.EdgeCount())
	}
}

// TestCtorScopedMethodEdge_ZeroInPackageResolvesToNothing: when the named
// package defines NO matching method, the encoded form resolves to nothing —
// it must NOT fall through to the global suffix rule (the constructor's
// return type is unknown at the AST level; guessing the sole global .Run
// would be a heuristic). The legacy "<var>.Method" entry — recorded alongside
// by the extractor — remains the sanctioned secondary path for that case.
func TestCtorScopedMethodEdge_ZeroInPackageResolvesToNothing(t *testing.T) {
	nodes := []core.SymbolNode{
		{
			Service: "svc", Package: "Harness", Name: "Harness.Run",
			Kind: core.KindGoMethod, FilePath: "kits/agent-harness/harness.go",
		},
		{
			Service: "svc", Package: "evidenceladder_test", Name: "TestRun",
			Kind: core.KindGoFunc, FilePath: "foundations/evidence/evidence-ladder/ladder_test.go",
			// Only the encoded form: the package it names has no .Run node.
			Calls: []string{"example.com/kit/foundations/evidence/evidence-ladder#.Run"},
		},
	}
	g := callgraph.Build(nodes)
	if g.EdgeCount() != 0 {
		t.Errorf("EdgeCount() = %d, want 0 (no .Run in the constructor's package; no global fall-through)", g.EdgeCount())
	}
}

// TestCtorScopedMethodEdge_SecondaryGlobalFallbackStillWorks: the legacy
// "<var>.Method" form recorded alongside the encoded one still resolves via
// the pre-existing global unique-suffix method fallback when it is genuinely
// unambiguous — the two paths compose instead of shadowing each other.
func TestCtorScopedMethodEdge_SecondaryGlobalFallbackStillWorks(t *testing.T) {
	nodes := []core.SymbolNode{
		{
			Service: "svc", Package: "Harness", Name: "Harness.Run",
			Kind: core.KindGoMethod, FilePath: "kits/agent-harness/harness.go",
		},
		{
			Service: "svc", Package: "harness_test", Name: "TestRun",
			Kind: core.KindGoFunc, FilePath: "kits/agent-harness/harness_test.go",
			Calls: []string{
				"h.Run", // globally unique .Run → secondary path binds it
				"example.com/kit/foundations/evidence/evidence-ladder#.Run", // nothing there
			},
		},
	}
	g := callgraph.Build(nodes)
	want := []string{"svc::kits/agent-harness/harness_test.TestRun"}
	if got := callgraph.TestImpact(g, "svc::Harness.Harness.Run"); !reflect.DeepEqual(got, want) {
		t.Errorf("TestImpact(Harness.Run) = %v, want %v", got, want)
	}
}
