package guardspec

import "testing"

// TestCheckableRuleCompilesToGuard is SR-45's anchor test (docs/plans/
// surface-reckoning.md, design §5 row SR-45, F-26): a served LayerRule
// whose kind has a deterministic pre-write check (import_direction or
// boundary — EvaluateLayerRuleEnforcement's own vocabulary) compiles to an
// enforce:true LayerRule via guardFromCheckableRule; a rule whose kind has
// no such check (no_cycles, cross_service_direction) is reported
// not-checkable instead of silently enforced — the "lint output" a caller
// renders for a served rule it cannot mechanically guard.
func TestCheckableRuleCompilesToGuard(t *testing.T) {
	importRule := LayerRule{
		ID: "no-store-in-cli", Kind: ImportDirection, Sentence: "cmd/fathom may not import internal/store directly",
		Severity: SeverityError,
		From:     []string{"cmd/fathom"}, To: []string{"internal/store"},
	}
	compiled, ok := guardFromCheckableRule(importRule)
	if !ok {
		t.Fatalf("guardFromCheckableRule(%q): ok = false, want true (import_direction is checkable)", importRule.ID)
	}
	if !compiled.Enforce {
		t.Errorf("compiled.Enforce = false, want true")
	}
	if compiled.ID != importRule.ID || compiled.Kind != importRule.Kind {
		t.Errorf("compiled = %+v, want the same rule (id/kind unchanged) with Enforce flipped on", compiled)
	}

	boundaryRule := LayerRule{
		ID: "ledger-boundary", Kind: Boundary, Sentence: "only internal/payment may import internal/ledger",
		Severity: SeverityError,
		Allow:    []string{"internal/payment"}, Guarded: []string{"internal/ledger"},
	}
	if compiled, ok := guardFromCheckableRule(boundaryRule); !ok || !compiled.Enforce {
		t.Errorf("guardFromCheckableRule(boundary) = %+v, ok=%v, want a compiled enforce:true guard", compiled, ok)
	}

	// no_cycles and cross_service_direction have no deterministic
	// pre-write check (EvaluateLayerRuleEnforcement's own doc comment: they
	// need the full index/fleet graph a single proposed file cannot
	// supply) — neither compiles.
	uncheckable := []LayerRule{
		{ID: "no-cycle", Kind: NoCycles, Sentence: "s", Severity: SeverityError, Scope: []string{"internal/**"}},
		{ID: "cross-svc", Kind: CrossServiceDirection, Sentence: "s", Severity: SeverityError, FromService: "a", ToService: "b"},
	}
	for _, rule := range uncheckable {
		if compiled, ok := guardFromCheckableRule(rule); ok {
			t.Errorf("guardFromCheckableRule(%q, kind %s): ok = true, want false (not mechanically checkable) — got %+v", rule.ID, rule.Kind, compiled)
		}
	}

	// A rule already enforced compiles to itself unchanged (idempotent).
	already := importRule
	already.Enforce = true
	if compiled, ok := guardFromCheckableRule(already); !ok || !compiled.Enforce || compiled.ID != already.ID {
		t.Errorf("guardFromCheckableRule on an already-enforced rule = %+v, ok=%v, want the same rule unchanged", compiled, ok)
	}
}

// TestLayerRuleIsCheckable proves the checkable/not-checkable split the
// obligations answer would mark a served rule with (SR-45's "obligations
// answer marks it checkable" half): exactly the two kinds
// EvaluateLayerRuleEnforcement has a deterministic check for.
func TestLayerRuleIsCheckable(t *testing.T) {
	cases := []struct {
		kind LayerRuleKind
		want bool
	}{
		{ImportDirection, true},
		{Boundary, true},
		{NoCycles, false},
		{CrossServiceDirection, false},
		{LayerRuleKind("unknown_kind"), false},
	}
	for _, c := range cases {
		if got := LayerRuleIsCheckable(LayerRule{Kind: c.kind}); got != c.want {
			t.Errorf("LayerRuleIsCheckable(kind=%s) = %v, want %v", c.kind, got, c.want)
		}
	}
}
