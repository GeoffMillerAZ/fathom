package findings

import (
	"testing"
	"time"

	"github.com/geoffmilleraz/fathom/internal/core"
)

// neighborhoodNodes: a chain A -> B -> C -> D (A calls B calls C calls D),
// plus an unrelated E with no edges to the chain at all.
//
//	A (pkg/a.go)  --calls-->  B (pkg/b.go)  --calls-->  C (pkg/c.go)  --calls--> D (pkg/d.go)
//	E (pkg/e.go)  (isolated)
func neighborhoodNodes() []core.SymbolNode {
	return []core.SymbolNode{
		{Service: "svc", Package: "pkg", Name: "A", Kind: core.KindGoFunc, FilePath: "pkg/a.go", Calls: []string{"svc::pkg.B"}},
		{Service: "svc", Package: "pkg", Name: "B", Kind: core.KindGoFunc, FilePath: "pkg/b.go", Calls: []string{"svc::pkg.C"}},
		{Service: "svc", Package: "pkg", Name: "C", Kind: core.KindGoFunc, FilePath: "pkg/c.go", Calls: []string{"svc::pkg.D"}},
		{Service: "svc", Package: "pkg", Name: "D", Kind: core.KindGoFunc, FilePath: "pkg/d.go"},
		{Service: "svc", Package: "pkg", Name: "E", Kind: core.KindGoFunc, FilePath: "pkg/e.go"},
	}
}

func TestFileNeighborhood_BidirectionalHopDistances(t *testing.T) {
	nodes := neighborhoodNodes()
	hops := FileNeighborhood(nodes, nil, "svc::pkg.B", 3)
	want := map[string]int{
		"pkg/b.go": 0, // subject itself
		"pkg/a.go": 1, // caller of B
		"pkg/c.go": 1, // callee of B
		"pkg/d.go": 2, // callee of C, 2 hops from B
	}
	for f, h := range want {
		if got, ok := hops[f]; !ok || got != h {
			t.Errorf("hops[%q] = %d (ok=%v), want %d", f, got, ok, h)
		}
	}
	if _, ok := hops["pkg/e.go"]; ok {
		t.Errorf("pkg/e.go (isolated) should not appear in the neighborhood, got hops=%+v", hops)
	}
}

func TestFileNeighborhood_HopBudgetRespected(t *testing.T) {
	nodes := neighborhoodNodes()
	hops := FileNeighborhood(nodes, nil, "svc::pkg.A", 1)
	if _, ok := hops["pkg/c.go"]; ok {
		t.Errorf("pkg/c.go is 2 hops from A, must not appear with maxHops=1: %+v", hops)
	}
	if _, ok := hops["pkg/b.go"]; !ok {
		t.Errorf("pkg/b.go is 1 hop from A, should appear: %+v", hops)
	}
}

func TestFileNeighborhood_SubjectIsFilePath_ExpandsToItsSymbols(t *testing.T) {
	nodes := neighborhoodNodes()
	hops := FileNeighborhood(nodes, nil, "pkg/b.go", 3)
	if hops["pkg/b.go"] != 0 {
		t.Errorf("hops[pkg/b.go] = %d, want 0 (subject file itself)", hops["pkg/b.go"])
	}
	if hops["pkg/a.go"] != 1 {
		t.Errorf("hops[pkg/a.go] = %d, want 1", hops["pkg/a.go"])
	}
}

func TestFileNeighborhood_UnresolvableSubject_Nil(t *testing.T) {
	nodes := neighborhoodNodes()
	hops := FileNeighborhood(nodes, nil, "svc::pkg.NoSuchSymbol", 3)
	if hops != nil {
		t.Errorf("hops = %+v, want nil for an unresolvable subject", hops)
	}
}

func TestRankNear_ProximityThenRecency(t *testing.T) {
	t0 := time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC)
	claims := []core.SemanticClaim{
		{ClaimID: "far-old", Scope: []string{"pkg/d.go"}, Status: core.ClaimFresh, CompiledAt: t0},
		{ClaimID: "near-old", Scope: []string{"pkg/a.go"}, Status: core.ClaimFresh, CompiledAt: t0},
		{ClaimID: "near-new", Scope: []string{"pkg/a.go"}, Status: core.ClaimFresh, CompiledAt: t0.Add(time.Hour)},
	}
	fileHops := map[string]int{"pkg/a.go": 1, "pkg/d.go": 2}
	got := RankNear(claims, fileHops, false)
	if len(got) != 3 {
		t.Fatalf("got %d entries, want 3: %+v", len(got), got)
	}
	// near-new (hop 1, newest) first, then near-old (hop 1, older), then
	// far-old (hop 2) last.
	wantOrder := []string{"near-new", "near-old", "far-old"}
	for i, id := range wantOrder {
		if got[i].Claim.ClaimID != id {
			t.Errorf("position %d = %q, want %q (order: %v)", i, got[i].Claim.ClaimID, id, got)
		}
	}
}

func TestRankNear_ScopeOutsideNeighborhood_Excluded(t *testing.T) {
	claims := []core.SemanticClaim{
		{ClaimID: "outside", Scope: []string{"pkg/zzz.go"}, Status: core.ClaimFresh},
	}
	got := RankNear(claims, map[string]int{"pkg/a.go": 0}, false)
	if len(got) != 0 {
		t.Errorf("got %+v, want empty (scope not in neighborhood)", got)
	}
}

// TestRankNear_SupersededExcludedByDefaultAlwaysLabeledWhenIncluded is §4
// honesty rule 4's direct proof: a superseded finding is invisible unless
// asked, and NEVER unlabeled when included.
func TestRankNear_SupersededExcludedByDefaultAlwaysLabeledWhenIncluded(t *testing.T) {
	claims := []core.SemanticClaim{
		{ClaimID: "live", Scope: []string{"pkg/a.go"}, Status: core.ClaimFresh},
		{ClaimID: "fixed", Scope: []string{"pkg/a.go"}, Status: core.ClaimSuperseded},
		{ClaimID: "wrong", Scope: []string{"pkg/a.go"}, Status: core.ClaimRefuted},
	}
	fileHops := map[string]int{"pkg/a.go": 0}

	excluded := RankNear(claims, fileHops, false)
	if len(excluded) != 1 || excluded[0].Claim.ClaimID != "live" {
		t.Fatalf("default (includeSuperseded=false) = %+v, want only 'live'", excluded)
	}

	included := RankNear(claims, fileHops, true)
	if len(included) != 3 {
		t.Fatalf("includeSuperseded=true = %+v, want all 3", included)
	}
	for _, e := range included {
		wantSuperseded := e.Claim.ClaimID != "live"
		if e.Superseded != wantSuperseded {
			t.Errorf("entry %q: Superseded = %v, want %v", e.Claim.ClaimID, e.Superseded, wantSuperseded)
		}
	}
}

func TestRankNear_EmptyInput_EmptyOutput(t *testing.T) {
	if got := RankNear(nil, nil, false); len(got) != 0 {
		t.Errorf("got %+v, want empty", got)
	}
}
