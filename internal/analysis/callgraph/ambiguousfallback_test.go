package callgraph_test

import (
	"context"
	"fmt"
	"reflect"
	"sort"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/analysis/callgraph"
	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/indexbench"
)

// SB-50 (docs/plans/strengths-bench.md): Build's ambiguous-name fallback
// (the suffix match and the method-call fallback in resolve) used to scan
// every node for every unresolved callee — O(calls * nodes). It now
// resolves through resolveAmbiguousCallees against a suffix index Build
// computes once. These are golden/equality tests: the edge set below was
// captured from the pre-refactor O(nodes)-scan algorithm and must still be
// exactly what the indexed algorithm produces, on the fauxtown fixture
// fleet and on a synthetic graph exercising both fallback branches at
// N > 2 candidates (not just the pairwise case the existing ambiguity
// tests already cover).

// TestBuildEdges_FauxtownGolden pins Build's complete edge set over the
// fauxtown fixture fleet's three Go services, merged into one graph the way
// a cross-repo caller would. The fixture doesn't happen to need the
// ambiguous-name fallback to resolve any of its own edges, so this test's
// job is proving the refactor leaves the OTHER resolution paths (exact,
// byPkgName, byDirName, same-package) untouched; TestResolveAmbiguousCallees_*
// below cover the fallback itself.
func TestBuildEdges_FauxtownGolden(t *testing.T) {
	ctx := context.Background()
	c, _, err := indexbench.NewFauxtownIndex(ctx)
	if err != nil {
		t.Fatalf("NewFauxtownIndex: %v", err)
	}
	var merged []core.SymbolNode
	for _, svc := range indexbench.FauxtownGoServices {
		nodes, err := c.ListAllSymbols(ctx, svc)
		if err != nil {
			t.Fatalf("ListAllSymbols(%s): %v", svc, err)
		}
		merged = append(merged, nodes...)
	}

	g := callgraph.Build(merged)

	var edges []string
	for _, n := range merged {
		for _, ce := range callgraph.CalleesDetail(g, n.Key()) {
			edges = append(edges, fmt.Sprintf("%s -> %s", n.Key(), ce.Key))
		}
	}
	sort.Strings(edges)

	want := []string{
		"ledger-service::ledger-service/ledger.Balance -> ledger-service::ledger-service/ledger.computeBalance",
		"ledger-service::ledger-service/ledger.Post -> ledger-service::ledger-service/ledger.applyEntry",
		"ledger-service::ledger-service/ledger.Post -> ledger-service::ledger-service/ledger.validateEntry",
		"payment-service::payment-service/payment.Charge -> payment-service::payment-service/payment.validateCharge",
	}
	if !reflect.DeepEqual(edges, want) {
		t.Errorf("Build(fauxtown).edges = %v, want %v", edges, want)
	}
	if got, wantN := len(merged), 13; got != wantN {
		t.Errorf("fauxtown node count = %d, want %d (fixture changed under this test)", got, wantN)
	}
	if got, wantN := g.EdgeCount(), len(want); got != wantN {
		t.Errorf("Build(fauxtown).EdgeCount() = %d, want %d", got, wantN)
	}
}

// TestResolveAmbiguousCallees_SuffixMatchManyPackages exercises the general
// suffix-match branch (not the method-call fallback) at N=3 candidates: a
// package name ("util") declared identically in three different
// directories makes both byPkgName and byDirName ambiguous, forcing
// resolution down to the raw component-boundary suffix scan
// resolveAmbiguousCallees now serves from the index. A fourth, uniquely
// named package proves the ambiguity in "util" doesn't collaterally break
// resolution elsewhere.
func TestResolveAmbiguousCallees_SuffixMatchManyPackages(t *testing.T) {
	nodes := []core.SymbolNode{
		{Service: "svc", Package: "util", Name: "Do", Kind: core.KindGoFunc, FilePath: "internal/a/util/util.go"},
		{Service: "svc", Package: "util", Name: "Do", Kind: core.KindGoFunc, FilePath: "internal/b/util/util.go"},
		{Service: "svc", Package: "util", Name: "Do", Kind: core.KindGoFunc, FilePath: "internal/c/util/util.go"},
		{Service: "svc", Package: "helper", Name: "Do", Kind: core.KindGoFunc, FilePath: "internal/helper/helper.go"},
		{
			Service: "svc", Package: "caller_test", Name: "TestAmbiguous", Kind: core.KindGoFunc,
			FilePath: "internal/caller/caller_test.go", Calls: []string{"util.Do"},
		},
		{
			Service: "svc", Package: "caller_test", Name: "TestUnique", Kind: core.KindGoFunc,
			FilePath: "internal/caller/caller_test.go", Calls: []string{"helper.Do"},
		},
	}
	g := callgraph.Build(nodes)

	if got := callgraph.TestImpact(g, "svc::internal/a/util/util.Do"); len(got) != 0 {
		t.Errorf("TestImpact(a/util.Do) = %v, want [] (3-way package-name ambiguity)", got)
	}
	if got := callgraph.TestImpact(g, "svc::internal/b/util/util.Do"); len(got) != 0 {
		t.Errorf("TestImpact(b/util.Do) = %v, want [] (3-way package-name ambiguity)", got)
	}
	if got := callgraph.TestImpact(g, "svc::internal/c/util/util.Do"); len(got) != 0 {
		t.Errorf("TestImpact(c/util.Do) = %v, want [] (3-way package-name ambiguity)", got)
	}
	want := []string{"svc::internal/caller/caller_test.TestUnique"}
	if got := callgraph.TestImpact(g, "svc::internal/helper/helper.Do"); !reflect.DeepEqual(got, want) {
		t.Errorf("TestImpact(helper.Do) = %v, want %v (unambiguous helper.Do must still resolve)", got, want)
	}
	if g.EdgeCount() != 1 {
		t.Errorf("EdgeCount() = %d, want 1 (only the unambiguous helper.Do call)", g.EdgeCount())
	}
}

// TestResolveAmbiguousCallees_MethodFallbackManyReceivers exercises the
// method-call fallback branch at N=4 candidates: four receiver types across
// four packages all define Close, so a "<var>.Close" callee must resolve to
// nothing — never first-match-wins, and never "ambiguous above 2". A fifth,
// uniquely named method proves the count==1 path still resolves and still
// applies the Name-suffix guard.
func TestResolveAmbiguousCallees_MethodFallbackManyReceivers(t *testing.T) {
	nodes := []core.SymbolNode{
		{Service: "svc", Package: "Alpha", Name: "Alpha.Close", Kind: core.KindGoMethod, FilePath: "alpha/alpha.go"},
		{Service: "svc", Package: "Beta", Name: "Beta.Close", Kind: core.KindGoMethod, FilePath: "beta/beta.go"},
		{Service: "svc", Package: "Gamma", Name: "Gamma.Close", Kind: core.KindGoMethod, FilePath: "gamma/gamma.go"},
		{Service: "svc", Package: "Delta", Name: "Delta.Close", Kind: core.KindGoMethod, FilePath: "delta/delta.go"},
		{Service: "svc", Package: "Solo", Name: "Solo.Shutdown", Kind: core.KindGoMethod, FilePath: "solo/solo.go"},
		{
			Service: "svc", Package: "caller_test", Name: "TestAmbiguousClose", Kind: core.KindGoFunc,
			FilePath: "caller/caller_test.go", Calls: []string{"x.Close"},
		},
		{
			Service: "svc", Package: "caller_test", Name: "TestUniqueShutdown", Kind: core.KindGoFunc,
			FilePath: "caller/caller_test.go", Calls: []string{"s.Shutdown"},
		},
	}
	g := callgraph.Build(nodes)

	for _, name := range []string{"Alpha.Close", "Beta.Close", "Gamma.Close", "Delta.Close"} {
		if got := callgraph.TestImpact(g, name); len(got) != 0 {
			t.Errorf("TestImpact(%s) = %v, want [] (4-way method-name ambiguity)", name, got)
		}
	}
	want := []string{"svc::caller/caller_test.TestUniqueShutdown"}
	if got := callgraph.TestImpact(g, "Solo.Shutdown"); !reflect.DeepEqual(got, want) {
		t.Errorf("TestImpact(Solo.Shutdown) = %v, want %v (unambiguous method must still resolve)", got, want)
	}
	if g.EdgeCount() != 1 {
		t.Errorf("EdgeCount() = %d, want 1 (only the unambiguous s.Shutdown call)", g.EdgeCount())
	}
}
