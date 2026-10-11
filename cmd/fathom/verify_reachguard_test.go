package main

// verify_reachguard_test.go — RF-5 (ADR-067 D3, I57): the CLI's verify
// evidence holds the same guard as the daemon's verify_plan
// (internal/mcp's TestReachAndWiredIgnoreHeuristicEdges): reachable_from
// and wired pass on typed edges only, and a refusal names the heuristic
// edge and its tier.

import (
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/analysis/callgraph"
	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/verify"
)

func TestCLIVerifyEvidence_ReachIgnoresHeuristicEdges(t *testing.T) {
	fn := func(file, pkg, name string, calls ...string) core.SymbolNode {
		return core.SymbolNode{Service: "svc", Package: pkg, Name: name, Kind: core.KindGoFunc, FilePath: file, Line: 1, Tier: core.TierVerified, Calls: calls}
	}
	nodes := []core.SymbolNode{
		fn("cmd/app/main.go", "main", "main", "lib.Typed", "lib.Guess"),
		fn("internal/lib/lib.go", "lib", "Typed"),
		fn("internal/lib/lib.go", "lib", "Guess"),
		fn("internal/lib/lib_test.go", "lib", "TestReach", "Typed", "Guess"),
	}
	typed := []core.Edge{
		{FromService: "svc", FromSymbol: "svc::cmd/app/main.main", ToSymbol: "svc::internal/lib/lib.Typed", Kind: core.EdgeDirect},
		{FromService: "svc", FromSymbol: "svc::internal/lib/lib.TestReach", ToSymbol: "svc::internal/lib/lib.Typed", Kind: core.EdgeDirect},
	}
	g := callgraph.BuildBoth(nodes, typed)
	ev := &cliEvidence{graph: g, product: callgraph.ProductEntryReach(g)}
	var _ verify.HeuristicReachEvidence = ev

	if !ev.Reachable("TestReach", "Typed") {
		t.Error("TestReach -> Typed is a typed edge: Reachable must pass")
	}
	if ev.Reachable("TestReach", "Guess") {
		t.Error("TestReach -> Guess is syntax-only: Reachable must refuse it")
	}
	if h, ok := ev.HeuristicReach("TestReach", "Guess"); !ok || h.Tier != "heuristic" || !strings.HasSuffix(h.From, "TestReach") || !strings.HasSuffix(h.To, "Guess") {
		t.Errorf("HeuristicReach = (%+v, %v), want the edge TestReach -> Guess, tier heuristic", h, ok)
	}
	if _, ok := ev.ProductReachedBy("Typed"); !ok {
		t.Error("main -> Typed is a typed edge: ProductReachedBy must pass")
	}
	if _, ok := ev.ProductReachedBy("Guess"); ok {
		t.Error("main -> Guess is syntax-only: ProductReachedBy must refuse it")
	}
	if entry, h, ok := ev.ProductHeuristicReach("Guess"); !ok || entry != "svc::cmd/app/main.main" || h.Tier != "heuristic" {
		t.Errorf("ProductHeuristicReach = (%q, %+v, %v), want main via a heuristic edge", entry, h, ok)
	}
}
