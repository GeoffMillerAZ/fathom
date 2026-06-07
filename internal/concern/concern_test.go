package concern_test

import (
	"testing"

	"github.com/geoffmilleraz/sextant/internal/concern"
	"github.com/geoffmilleraz/sextant/internal/core"
)

// ---------------------------------------------------------------------------
// PropagateConcern tests
// ---------------------------------------------------------------------------

func TestPropagateConcern_Empty(t *testing.T) {
	got := concern.PropagateConcern(nil, "auth", "pkg.Login")
	if got != nil {
		t.Errorf("expected nil for empty symbols, got %v", got)
	}
}

func TestPropagateConcern_EmptySeed(t *testing.T) {
	syms := []core.SymbolNode{{Package: "pkg", Name: "Login"}}
	got := concern.PropagateConcern(syms, "auth", "")
	if got != nil {
		t.Errorf("expected nil for empty seed, got %v", got)
	}
}

func TestPropagateConcern_SeedNotInGraph(t *testing.T) {
	syms := []core.SymbolNode{
		{Package: "pkg", Name: "AuthCheck"},
	}
	got := concern.PropagateConcern(syms, "auth", "pkg.Unknown")
	// seed not in graph but concern matches nothing reachable from seed.
	if len(got) != 0 {
		t.Errorf("expected no results for seed absent from graph, got %v", got)
	}
}

func TestPropagateConcern_DirectMatch(t *testing.T) {
	syms := []core.SymbolNode{
		{Package: "auth", Name: "Validate", Calls: nil},
	}
	got := concern.PropagateConcern(syms, "auth", "auth.Validate")
	if len(got) != 1 || got[0] != "auth.Validate" {
		t.Errorf("expected [auth.Validate], got %v", got)
	}
}

func TestPropagateConcern_BackwardWalk(t *testing.T) {
	// handler → service → authCheck
	// Concern = "auth"; seed = "auth.Check"
	// Expected: auth.Check (direct match), caller service.Run if name contains "auth"
	syms := []core.SymbolNode{
		{Package: "auth", Name: "Check", Calls: nil},
		{Package: "service", Name: "Run", Calls: []string{"auth.Check"}},
		{Package: "handler", Name: "ServeHTTP", Calls: []string{"service.Run"}},
	}
	got := concern.PropagateConcern(syms, "auth", "auth.Check")
	// Only "auth.Check" matches the concern substring.
	if len(got) != 1 || got[0] != "auth.Check" {
		t.Errorf("expected [auth.Check], got %v", got)
	}
}

func TestPropagateConcern_MultipleMatches(t *testing.T) {
	// All three symbols contain "auth" in their package or name.
	syms := []core.SymbolNode{
		{Package: "auth", Name: "Login", Calls: nil},
		{Package: "auth", Name: "Logout", Calls: []string{"auth.Login"}},
		{Package: "authz", Name: "Check", Calls: []string{"auth.Logout"}},
	}
	got := concern.PropagateConcern(syms, "auth", "auth.Login")
	// All three are reachable backward from auth.Login and all contain "auth".
	want := map[string]bool{
		"auth.Login":  true,
		"auth.Logout": true,
		"authz.Check": true,
	}
	if len(got) != len(want) {
		t.Fatalf("expected %d results, got %d: %v", len(want), len(got), got)
	}
	for _, g := range got {
		if !want[g] {
			t.Errorf("unexpected result %q", g)
		}
	}
}

func TestPropagateConcern_Deterministic(t *testing.T) {
	syms := []core.SymbolNode{
		{Package: "auth", Name: "B", Calls: nil},
		{Package: "auth", Name: "A", Calls: []string{"auth.B"}},
		{Package: "auth", Name: "C", Calls: []string{"auth.A"}},
	}
	first := concern.PropagateConcern(syms, "auth", "auth.B")
	second := concern.PropagateConcern(syms, "auth", "auth.B")
	if len(first) != len(second) {
		t.Fatalf("non-deterministic: %v vs %v", first, second)
	}
	for i := range first {
		if first[i] != second[i] {
			t.Errorf("non-deterministic at [%d]: %q vs %q", i, first[i], second[i])
		}
	}
}

func TestPropagateConcern_NoCycleInfiniteLoop(t *testing.T) {
	// Mutual recursion: a calls b, b calls a.
	syms := []core.SymbolNode{
		{Package: "auth", Name: "A", Calls: []string{"auth.B"}},
		{Package: "auth", Name: "B", Calls: []string{"auth.A"}},
	}
	// Must terminate.
	got := concern.PropagateConcern(syms, "auth", "auth.A")
	if len(got) == 0 {
		t.Error("expected at least one result for cyclic concern graph")
	}
}

// ---------------------------------------------------------------------------
// EvidenceSummary tests
// ---------------------------------------------------------------------------

func TestEvidenceSummary_Empty(t *testing.T) {
	got := concern.EvidenceSummary(nil)
	if got == nil {
		t.Error("expected non-nil map for empty input")
	}
	if len(got) != 0 {
		t.Errorf("expected empty map, got %v", got)
	}
}

func TestEvidenceSummary_SingleTier(t *testing.T) {
	obs := []core.Observation{
		{EvidenceTier: core.TierVerified},
		{EvidenceTier: core.TierVerified},
		{EvidenceTier: core.TierVerified},
	}
	got := concern.EvidenceSummary(obs)
	if got[core.TierVerified] != 3 {
		t.Errorf("expected 3 verified, got %d", got[core.TierVerified])
	}
}

func TestEvidenceSummary_MultipleTiers(t *testing.T) {
	obs := []core.Observation{
		{EvidenceTier: core.TierVerified},
		{EvidenceTier: core.TierPreScan},
		{EvidenceTier: core.TierDocClaimed},
		{EvidenceTier: core.TierDocClaimed},
		{EvidenceTier: core.TierAssumed},
	}
	got := concern.EvidenceSummary(obs)
	checks := map[core.EvidenceTier]int{
		core.TierVerified:   1,
		core.TierPreScan:    1,
		core.TierDocClaimed: 2,
		core.TierAssumed:    1,
	}
	for tier, want := range checks {
		if got[tier] != want {
			t.Errorf("tier %q: want %d, got %d", tier, want, got[tier])
		}
	}
}

func TestEvidenceSummary_Deterministic(t *testing.T) {
	obs := []core.Observation{
		{EvidenceTier: core.TierVerified},
		{EvidenceTier: core.TierPreScan},
	}
	a := concern.EvidenceSummary(obs)
	b := concern.EvidenceSummary(obs)
	for tier := range a {
		if a[tier] != b[tier] {
			t.Errorf("non-deterministic for tier %q: %d vs %d", tier, a[tier], b[tier])
		}
	}
}
