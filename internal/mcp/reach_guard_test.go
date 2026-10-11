package mcp

// reach_guard_test.go — RF-5 (docs/plans/repository-facts.md; ADR-067 D3;
// I57 extended to call edges): verify's `reachable_from` (edge_wired) and
// `wired` pass on no edge of resolution tier heuristic, and a refusal
// names the edge and its tier. The edge is still served with its tier
// named: symbol_context and blast_radius answer exactly as before.

import (
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/core"
)

// reachGuardServer: one Go service with a binary.
//
//	cmd/app main -> lib.Wire -> lib.Typed   (typed: direct)
//	cmd/app main -> lib.Guess               (syntax only: heuristic)
//	lib.Guess    -> lib.Deep                (typed)
//	TestReach    -> lib.Typed               (typed)
//	TestReach    -> lib.Guess               (syntax only: heuristic)
func reachGuardServer(t *testing.T) *Server {
	t.Helper()
	fn := func(file, pkg, name string, calls ...string) core.SymbolNode {
		return core.SymbolNode{Service: "svc", Package: pkg, Name: name, Kind: core.KindGoFunc, FilePath: file, Line: 1, Tier: core.TierVerified, Calls: calls}
	}
	e := func(from, to string) core.Edge {
		return core.Edge{FromService: "svc", FromSymbol: "svc::" + from, ToSymbol: "svc::" + to, FromFile: "", Kind: core.EdgeDirect}
	}
	return NewServer(&multiSvcBackend{svcs: map[string]*svcFixture{
		"svc": {
			nodes: []core.SymbolNode{
				fn("cmd/app/main.go", "main", "main", "lib.Wire", "lib.Guess"),
				fn("internal/lib/lib.go", "lib", "Wire", "Typed"),
				fn("internal/lib/lib.go", "lib", "Typed"),
				fn("internal/lib/guess.go", "lib", "Guess", "Deep"),
				fn("internal/lib/guess.go", "lib", "Deep"),
				fn("internal/lib/lib_test.go", "lib", "TestReach", "Typed", "Guess"),
			},
			typedEdges: []core.Edge{
				e("cmd/app/main.main", "internal/lib/lib.Wire"),
				e("internal/lib/lib.Wire", "internal/lib/lib.Typed"),
				e("internal/lib/lib.Guess", "internal/lib/lib.Deep"),
				e("internal/lib/lib.TestReach", "internal/lib/lib.Typed"),
			},
			indexCommit: "guardcommit",
			path:        "/repo",
		},
	}})
}

func criterionOf(t *testing.T, task map[string]any, name string) map[string]any {
	t.Helper()
	for _, c := range task["criteria"].([]any) {
		if m := c.(map[string]any); m["name"] == name {
			return m
		}
	}
	t.Fatalf("task %v has no %s criterion: %v", task["id"], name, task["criteria"])
	return nil
}

func TestReachAndWiredIgnoreHeuristicEdges(t *testing.T) {
	srv := reachGuardServer(t)
	got := callTool(t, srv, "verify_plan", map[string]any{
		"service": "svc",
		"tasks": []any{
			map[string]any{"id": "H-1", "claimed": true, "symbol": "Guess", "reachable_from": "TestReach"},
			map[string]any{"id": "H-2", "claimed": true, "symbol": "Typed", "reachable_from": "TestReach"},
		},
	})
	tasks, _ := got["tasks"].([]any)
	if len(tasks) != 2 {
		t.Fatalf("verify_plan returned %d tasks, want 2: %v", len(tasks), got)
	}
	byID := map[string]map[string]any{}
	for _, x := range tasks {
		m := x.(map[string]any)
		byID[m["id"].(string)] = m
	}

	// H-1: reached only through heuristic edges — refused, the edge and
	// its tier named, on both criteria.
	h1 := byID["H-1"]
	if h1["status"] == "pass" {
		t.Errorf("H-1 passed on heuristic edges alone: %v", h1)
	}
	ew := criterionOf(t, h1, "edge_wired")
	if ew["status"] != "fail" {
		t.Errorf("H-1 edge_wired = %v, want fail", ew)
	}
	if d, _ := ew["detail"].(string); !strings.Contains(d, "heuristic") || !strings.Contains(d, "lib.TestReach") || !strings.Contains(d, "lib.Guess") {
		t.Errorf("H-1 edge_wired detail %q must name the heuristic edge TestReach -> Guess and its tier", d)
	}
	w := criterionOf(t, h1, "wired")
	if w["status"] == "pass" {
		t.Errorf("H-1 wired = %v, want refused", w)
	}
	if d, _ := w["detail"].(string); !strings.Contains(d, "heuristic") || !strings.Contains(d, "main.main") || !strings.Contains(d, "lib.Guess") {
		t.Errorf("H-1 wired detail %q must name the heuristic edge main -> Guess and its tier", d)
	}

	// H-2: the same shape over typed edges passes.
	h2 := byID["H-2"]
	if c := criterionOf(t, h2, "edge_wired"); c["status"] != "pass" {
		t.Errorf("H-2 edge_wired = %v, want pass over typed edges", c)
	}
	if c := criterionOf(t, h2, "wired"); c["status"] != "pass" {
		t.Errorf("H-2 wired = %v, want pass over typed edges", c)
	}
	if h2["status"] != "pass" {
		t.Errorf("H-2 status = %v, want pass: %v", h2["status"], h2)
	}

	// The heuristic edge is still served, its tier named: symbol_context
	// lists main and TestReach as Guess's callers and tallies them
	// heuristic; Deep (a typed callee) tallies typed.
	sc := callTool(t, srv, "symbol_context", map[string]any{"service": "svc", "symbol": "Guess", "snippet": false})
	callers := map[string]bool{}
	for _, c := range sc["callers"].([]any) {
		callers[c.(string)] = true
	}
	if !callers["svc::cmd/app/main.main"] || !callers["svc::internal/lib/lib.TestReach"] {
		t.Errorf("symbol_context(Guess) callers = %v, want main and TestReach still served", sc["callers"])
	}
	if typed, heuristic := resolutionOf(t, sc); typed != 1 || heuristic != 2 {
		t.Errorf("symbol_context(Guess) resolution = {typed:%d heuristic:%d}, want {typed:1 heuristic:2}", typed, heuristic)
	}

	// blast_radius still walks the heuristic edge and says so.
	br := callTool(t, srv, "blast_radius", map[string]any{"service": "svc", "changed_files": []any{"internal/lib/guess.go"}, "include_heuristic": true})
	if _, heuristic := resolutionOf(t, br); heuristic == 0 {
		t.Errorf("blast_radius(guess.go) resolution names no heuristic edge: %v", br["resolution"])
	}
}
