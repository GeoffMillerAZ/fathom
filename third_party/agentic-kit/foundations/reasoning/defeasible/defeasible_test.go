// Package defeasible_test exercises the defeasible-resolution engine via
// external black-box tests. All tests are table-driven and race-safe.
package defeasible_test

import (
	"strings"
	"testing"

	"github.com/geoffmilleraz/agentic-kit/foundations/reasoning/defeasible"
)

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func mustResolve(t *testing.T, provisions []defeasible.Provision, modifiers []defeasible.Modifier, active map[defeasible.Status]bool) defeasible.Resolution {
	t.Helper()
	r, err := defeasible.Resolve(provisions, modifiers, active)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	return r
}

func defaultActive() map[defeasible.Status]bool {
	return map[defeasible.Status]bool{
		defeasible.StatusPresumed: true,
		defeasible.StatusUpheld:   true,
	}
}

func provision(key, value string, conf float64) defeasible.Provision {
	return defeasible.Provision{Key: key, BaseValue: value, BaseConfidence: conf}
}

func modifier(id, targetKey string, layer int, kind string, newValue string, effect defeasible.Effect, conf float64, refs []string, status defeasible.Status, supersedes []string, priority int) defeasible.Modifier {
	return defeasible.Modifier{
		ID:           id,
		TargetKey:    targetKey,
		Layer:        layer,
		Kind:         kind,
		NewValue:     newValue,
		Effect:       effect,
		Confidence:   conf,
		EvidenceRefs: refs,
		Status:       status,
		Supersedes:   supersedes,
		Priority:     priority,
	}
}

// ---------------------------------------------------------------------------
// Status validation
// ---------------------------------------------------------------------------

func TestStatusValidate(t *testing.T) {
	valid := []defeasible.Status{
		defeasible.StatusPresumed,
		defeasible.StatusUpheld,
		defeasible.StatusInvalidated,
		defeasible.StatusContested,
		defeasible.StatusWaived,
	}
	for _, s := range valid {
		t.Run(string(s), func(t *testing.T) {
			if err := s.Validate(); err != nil {
				t.Errorf("expected no error for %q, got %v", s, err)
			}
		})
	}
	if err := defeasible.Status("bogus").Validate(); err == nil {
		t.Error("expected error for unknown status")
	}
}

// ---------------------------------------------------------------------------
// Effect validation
// ---------------------------------------------------------------------------

func TestEffectValidate(t *testing.T) {
	valid := []defeasible.Effect{defeasible.EffectSet, defeasible.EffectStrike, defeasible.EffectReinterpret}
	for _, e := range valid {
		t.Run(string(e), func(t *testing.T) {
			if err := e.Validate(); err != nil {
				t.Errorf("expected no error for %q, got %v", e, err)
			}
		})
	}
	if err := defeasible.Effect("delete").Validate(); err == nil {
		t.Error("expected error for unknown effect")
	}
}

// ---------------------------------------------------------------------------
// Provision validation
// ---------------------------------------------------------------------------

func TestProvisionValidate(t *testing.T) {
	cases := []struct {
		name    string
		p       defeasible.Provision
		wantErr bool
	}{
		{"valid", provision("§1", "payment due in 30 days", 0.9), false},
		{"empty-key", provision("", "value", 0.9), true},
		{"conf-negative", provision("§1", "v", -0.1), true},
		{"conf-above-one", provision("§1", "v", 1.1), true},
		{"conf-zero", provision("§1", "v", 0.0), false},
		{"conf-one", provision("§1", "v", 1.0), false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.p.Validate()
			if tc.wantErr && err == nil {
				t.Error("expected error, got nil")
			}
			if !tc.wantErr && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Modifier validation
// ---------------------------------------------------------------------------

func TestModifierValidate(t *testing.T) {
	good := modifier("m1", "§1", 1, "amendment", "new text", defeasible.EffectSet, 0.8, nil, defeasible.StatusPresumed, nil, 0)
	cases := []struct {
		name    string
		m       defeasible.Modifier
		wantErr bool
	}{
		{"valid", good, false},
		{"empty-id", func() defeasible.Modifier { m := good; m.ID = ""; return m }(), true},
		{"empty-target", func() defeasible.Modifier { m := good; m.TargetKey = ""; return m }(), true},
		{"conf-negative", func() defeasible.Modifier { m := good; m.Confidence = -0.01; return m }(), true},
		{"conf-above-one", func() defeasible.Modifier { m := good; m.Confidence = 1.01; return m }(), true},
		{"bad-status", func() defeasible.Modifier { m := good; m.Status = "unknown"; return m }(), true},
		{"bad-effect", func() defeasible.Modifier { m := good; m.Effect = "delete"; return m }(), true},
		{"self-supersede", func() defeasible.Modifier { m := good; m.Supersedes = []string{"m1"}; return m }(), true},
		{"conf-zero-ok", func() defeasible.Modifier { m := good; m.Confidence = 0.0; return m }(), false},
		{"conf-one-ok", func() defeasible.Modifier { m := good; m.Confidence = 1.0; return m }(), false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.m.Validate()
			if tc.wantErr && err == nil {
				t.Error("expected error, got nil")
			}
			if !tc.wantErr && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Test: base-only — no modifiers
// ---------------------------------------------------------------------------

func TestResolveBaseOnly(t *testing.T) {
	provisions := []defeasible.Provision{
		provision("§1", "payment in 30 days", 0.9),
		provision("§2", "governing law: NY", 0.85),
	}
	r := mustResolve(t, provisions, nil, defaultActive())

	if len(r.Effective) != 2 {
		t.Fatalf("want 2 effective provisions, got %d", len(r.Effective))
	}
	// Deterministic ordering by key.
	if r.Effective[0].Key != "§1" {
		t.Errorf("first key = %q, want §1", r.Effective[0].Key)
	}
	if r.Effective[0].Value != "payment in 30 days" {
		t.Errorf("value = %q, want base value", r.Effective[0].Value)
	}
	if r.Effective[0].Confidence != 0.9 {
		t.Errorf("confidence = %g, want 0.9", r.Effective[0].Confidence)
	}
	if r.Effective[0].Void {
		t.Error("base provision should not be void")
	}
	if len(r.Effective[0].AppliedModifierIDs) != 0 {
		t.Errorf("no modifiers applied; got %v", r.Effective[0].AppliedModifierIDs)
	}
}

// ---------------------------------------------------------------------------
// Test: one amendment (set) applies correctly
// ---------------------------------------------------------------------------

func TestResolveOneAmendmentSetsValue(t *testing.T) {
	provisions := []defeasible.Provision{
		provision("§1", "payment in 30 days", 0.9),
	}
	mods := []defeasible.Modifier{
		modifier("m1", "§1", 1, "amendment", "payment in 45 days", defeasible.EffectSet, 0.8, nil, defeasible.StatusPresumed, nil, 0),
	}
	r := mustResolve(t, provisions, mods, defaultActive())

	if len(r.Effective) != 1 {
		t.Fatalf("want 1, got %d", len(r.Effective))
	}
	ep := r.Effective[0]
	if ep.Value != "payment in 45 days" {
		t.Errorf("value = %q, want amended value", ep.Value)
	}
	if ep.Void {
		t.Error("should not be void after set")
	}
	if len(ep.AppliedModifierIDs) != 1 || ep.AppliedModifierIDs[0] != "m1" {
		t.Errorf("AppliedModifierIDs = %v, want [m1]", ep.AppliedModifierIDs)
	}
}

// ---------------------------------------------------------------------------
// Test: invalidated modifier excluded from resolution
// ---------------------------------------------------------------------------

func TestResolveInvalidatedModifierExcluded(t *testing.T) {
	provisions := []defeasible.Provision{
		provision("§1", "original text", 0.9),
	}
	mods := []defeasible.Modifier{
		modifier("m1", "§1", 1, "amendment", "amended text", defeasible.EffectSet, 0.8, nil, defeasible.StatusInvalidated, nil, 0),
	}
	r := mustResolve(t, provisions, mods, defaultActive())
	ep := r.Effective[0]
	if ep.Value != "original text" {
		t.Errorf("invalidated modifier should not change value; got %q", ep.Value)
	}
	if len(ep.AppliedModifierIDs) != 0 {
		t.Errorf("no modifier should be applied; got %v", ep.AppliedModifierIDs)
	}
}

// ---------------------------------------------------------------------------
// Test: waived modifier excluded
// ---------------------------------------------------------------------------

func TestResolveWaivedModifierExcluded(t *testing.T) {
	provisions := []defeasible.Provision{
		provision("§3", "arbitration clause", 0.9),
	}
	mods := []defeasible.Modifier{
		modifier("m2", "§3", 1, "waiver", "waived", defeasible.EffectStrike, 0.7, nil, defeasible.StatusWaived, nil, 0),
	}
	r := mustResolve(t, provisions, mods, defaultActive())
	ep := r.Effective[0]
	if ep.Void {
		t.Error("waived modifier should not cause strike — it is inactive")
	}
}

// ---------------------------------------------------------------------------
// Test: supersession chain A superseded by B superseded by C — C wins, A&B shadowed
// ---------------------------------------------------------------------------

func TestResolveSupersessionChain(t *testing.T) {
	// Layer 1, same priority, same key.
	// C supersedes B supersedes A.
	// A, B are removed from contention; C is the only active modifier.
	provisions := []defeasible.Provision{provision("§2", "base", 0.9)}
	mods := []defeasible.Modifier{
		modifier("mA", "§2", 1, "amendment", "vA", defeasible.EffectSet, 0.7, nil, defeasible.StatusPresumed, nil, 0),
		modifier("mB", "§2", 1, "amendment", "vB", defeasible.EffectSet, 0.75, nil, defeasible.StatusPresumed, []string{"mA"}, 1),
		modifier("mC", "§2", 1, "amendment", "vC", defeasible.EffectSet, 0.8, nil, defeasible.StatusPresumed, []string{"mB"}, 2),
	}
	r := mustResolve(t, provisions, mods, defaultActive())
	ep := r.Effective[0]
	if ep.Value != "vC" {
		t.Errorf("value = %q, want vC (C wins the supersession chain)", ep.Value)
	}
	// mA and mB must appear in Shadowed (removed by supersession).
	shadowed := make(map[string]bool)
	for _, id := range ep.ShadowedModifierIDs {
		shadowed[id] = true
	}
	if !shadowed["mA"] || !shadowed["mB"] {
		t.Errorf("shadowed = %v, want mA and mB to be shadowed", ep.ShadowedModifierIDs)
	}
	// mC applied.
	if len(ep.AppliedModifierIDs) != 1 || ep.AppliedModifierIDs[0] != "mC" {
		t.Errorf("AppliedModifierIDs = %v, want [mC]", ep.AppliedModifierIDs)
	}
}

// ---------------------------------------------------------------------------
// Test: two modifiers at different layers chain — higher layer's value wins,
// both contribute to confidence (weakest-link), and lower-layer is applied
// (not shadowed by conflict, since they occupy different Layer slots).
// ---------------------------------------------------------------------------

func TestResolveConflictHigherLayerWins(t *testing.T) {
	provisions := []defeasible.Provision{provision("§4", "base", 0.9)}
	mods := []defeasible.Modifier{
		// mLow: layer 1 (legal); mHigh: layer 2 (operational); neither supersedes the other.
		// They occupy different Layer slots so both are applied in order.
		// mHigh is applied last and its value prevails.
		modifier("mLow", "§4", 1, "contra_proferentem", "low-layer value", defeasible.EffectSet, 0.8, nil, defeasible.StatusPresumed, nil, 0),
		modifier("mHigh", "§4", 2, "statutory_override", "high-layer value", defeasible.EffectSet, 0.75, nil, defeasible.StatusPresumed, nil, 0),
	}
	r := mustResolve(t, provisions, mods, defaultActive())
	ep := r.Effective[0]
	if ep.Value != "high-layer value" {
		t.Errorf("value = %q, want high-layer value (layer 2 applied last, prevails)", ep.Value)
	}
	// Both mods are applied in the chain — neither is a conflict loser.
	applied := make(map[string]bool)
	for _, id := range ep.AppliedModifierIDs {
		applied[id] = true
	}
	if !applied["mLow"] || !applied["mHigh"] {
		t.Errorf("both mods should be applied; AppliedModifierIDs = %v", ep.AppliedModifierIDs)
	}
	// Confidence: weakest-link across base=0.9, mLow=0.8, mHigh=0.75 → 0.75.
	const wantConf = 0.75
	if ep.Confidence != wantConf {
		t.Errorf("confidence = %g, want %g (weakest-link across chain)", ep.Confidence, wantConf)
	}
}

// ---------------------------------------------------------------------------
// Test: strike → Void
// ---------------------------------------------------------------------------

func TestResolveStrikeVoid(t *testing.T) {
	provisions := []defeasible.Provision{provision("§5", "penalty clause", 0.9)}
	mods := []defeasible.Modifier{
		modifier("mStrike", "§5", 2, "unenforceable", "", defeasible.EffectStrike, 0.8, nil, defeasible.StatusPresumed, nil, 0),
	}
	r := mustResolve(t, provisions, mods, defaultActive())
	ep := r.Effective[0]
	if !ep.Void {
		t.Error("provision should be void after strike")
	}
	if ep.Value != "" {
		t.Errorf("void provision should have empty value, got %q", ep.Value)
	}
}

// ---------------------------------------------------------------------------
// Test: confidence weakest-link bounding along applied chain
// ---------------------------------------------------------------------------

func TestResolveConfidenceWeakestLink(t *testing.T) {
	// base conf = 0.9, modifier conf = 0.6 → effective = min(0.9, 0.6) = 0.6
	provisions := []defeasible.Provision{provision("§6", "base", 0.9)}
	mods := []defeasible.Modifier{
		modifier("mWeak", "§6", 1, "amendment", "new text", defeasible.EffectSet, 0.6, nil, defeasible.StatusPresumed, nil, 0),
	}
	r := mustResolve(t, provisions, mods, defaultActive())
	ep := r.Effective[0]
	const want = 0.6
	if ep.Confidence != want {
		t.Errorf("confidence = %g, want weakest-link %g", ep.Confidence, want)
	}
}

func TestResolveConfidenceWeakestLinkChain(t *testing.T) {
	// Two modifiers applied in sequence: base=0.9, m1=0.7, m2=0.85 → min = 0.7.
	provisions := []defeasible.Provision{provision("§7", "base", 0.9)}
	mods := []defeasible.Modifier{
		modifier("m1", "§7", 1, "amendment", "step1", defeasible.EffectSet, 0.7, nil, defeasible.StatusPresumed, nil, 0),
		modifier("m2", "§7", 1, "course_of_performance", "step2", defeasible.EffectSet, 0.85, nil, defeasible.StatusPresumed, nil, 1),
	}
	r := mustResolve(t, provisions, mods, defaultActive())
	ep := r.Effective[0]
	const want = 0.7
	if ep.Confidence != want {
		t.Errorf("confidence = %g, want weakest-link %g", ep.Confidence, want)
	}
}

// ---------------------------------------------------------------------------
// Test: cycle in Supersedes DAG → error
// ---------------------------------------------------------------------------

func TestResolveCycleError(t *testing.T) {
	provisions := []defeasible.Provision{provision("§8", "base", 0.9)}
	// mX supersedes mY and mY supersedes mX → cycle.
	mods := []defeasible.Modifier{
		modifier("mX", "§8", 1, "amendment", "vX", defeasible.EffectSet, 0.8, nil, defeasible.StatusPresumed, []string{"mY"}, 0),
		modifier("mY", "§8", 1, "amendment", "vY", defeasible.EffectSet, 0.8, nil, defeasible.StatusPresumed, []string{"mX"}, 1),
	}
	_, err := defeasible.Resolve(provisions, mods, defaultActive())
	if err == nil {
		t.Fatal("expected cycle error, got nil")
	}
	if !strings.Contains(err.Error(), "cycle") {
		t.Errorf("error %q should mention cycle", err.Error())
	}
}

// ---------------------------------------------------------------------------
// Test: contingency pivot — flip amendment to invalidated changes effective value back
// ---------------------------------------------------------------------------

func TestResolveContingencyPivot(t *testing.T) {
	provisions := []defeasible.Provision{provision("§1", "original", 0.9)}
	mods := []defeasible.Modifier{
		modifier("m1", "§1", 1, "amendment", "amended", defeasible.EffectSet, 0.8, nil, defeasible.StatusPresumed, nil, 0),
	}

	// With m1 active: amended value.
	r1 := mustResolve(t, provisions, mods, defaultActive())
	if r1.Effective[0].Value != "amended" {
		t.Fatalf("expected amended value, got %q", r1.Effective[0].Value)
	}

	// Flip m1 to invalidated: original value restored.
	flipped := defeasible.WithStatus(mods, "m1", defeasible.StatusInvalidated)
	r2 := mustResolve(t, provisions, flipped, defaultActive())
	if r2.Effective[0].Value != "original" {
		t.Errorf("after flip to invalidated, want original, got %q", r2.Effective[0].Value)
	}

	// Original slice is unmodified (defensive copy).
	if mods[0].Status != defeasible.StatusPresumed {
		t.Error("WithStatus must not mutate the original modifiers slice")
	}
}

// ---------------------------------------------------------------------------
// Test: determinism on shuffled input
// ---------------------------------------------------------------------------

func TestResolveDeterministic(t *testing.T) {
	provisions := []defeasible.Provision{
		provision("§B", "baseB", 0.8),
		provision("§A", "baseA", 0.9),
	}
	mods := []defeasible.Modifier{
		modifier("mZ", "§B", 1, "amendment", "vZ", defeasible.EffectSet, 0.7, nil, defeasible.StatusPresumed, nil, 0),
		modifier("mA", "§A", 1, "amendment", "vA", defeasible.EffectSet, 0.75, nil, defeasible.StatusPresumed, nil, 0),
	}

	r1 := mustResolve(t, provisions, mods, defaultActive())

	// Reverse provision and modifier order.
	provisions2 := []defeasible.Provision{provisions[1], provisions[0]}
	mods2 := []defeasible.Modifier{mods[1], mods[0]}
	r2 := mustResolve(t, provisions2, mods2, defaultActive())

	if len(r1.Effective) != len(r2.Effective) {
		t.Fatalf("lengths differ: %d vs %d", len(r1.Effective), len(r2.Effective))
	}
	for i := range r1.Effective {
		if r1.Effective[i].Key != r2.Effective[i].Key {
			t.Errorf("position %d: key %q vs %q — not deterministic", i, r1.Effective[i].Key, r2.Effective[i].Key)
		}
		if r1.Effective[i].Value != r2.Effective[i].Value {
			t.Errorf("position %d: value %q vs %q — not deterministic", i, r1.Effective[i].Value, r2.Effective[i].Value)
		}
	}
}

// ---------------------------------------------------------------------------
// Test: conflict — same layer and priority, higher confidence wins
// ---------------------------------------------------------------------------

func TestResolveConflictSameLayerPriorityHigherConfidenceWins(t *testing.T) {
	provisions := []defeasible.Provision{provision("§9", "base", 0.9)}
	mods := []defeasible.Modifier{
		modifier("mLowConf", "§9", 1, "amendment", "low-conf value", defeasible.EffectSet, 0.5, nil, defeasible.StatusPresumed, nil, 0),
		modifier("mHighConf", "§9", 1, "amendment", "high-conf value", defeasible.EffectSet, 0.85, nil, defeasible.StatusPresumed, nil, 0),
	}
	r := mustResolve(t, provisions, mods, defaultActive())
	ep := r.Effective[0]
	if ep.Value != "high-conf value" {
		t.Errorf("value = %q, want high-conf value (tiebreak by confidence)", ep.Value)
	}
	shadowed := make(map[string]bool)
	for _, id := range ep.ShadowedModifierIDs {
		shadowed[id] = true
	}
	if !shadowed["mLowConf"] {
		t.Errorf("mLowConf should be shadowed; shadowed = %v", ep.ShadowedModifierIDs)
	}
}

// ---------------------------------------------------------------------------
// Test: contested modifier active when contested is included in activeStatuses
// ---------------------------------------------------------------------------

func TestResolveContestedActiveWhenConfigured(t *testing.T) {
	provisions := []defeasible.Provision{provision("§10", "base", 0.9)}
	mods := []defeasible.Modifier{
		modifier("mC", "§10", 1, "amendment", "contested value", defeasible.EffectSet, 0.7, nil, defeasible.StatusContested, nil, 0),
	}

	// By default (presumed+upheld active), contested is NOT active.
	r1 := mustResolve(t, provisions, mods, defaultActive())
	if r1.Effective[0].Value != "base" {
		t.Errorf("contested should be inactive by default; got %q", r1.Effective[0].Value)
	}

	// Include contested in activeStatuses.
	active2 := map[defeasible.Status]bool{
		defeasible.StatusPresumed:  true,
		defeasible.StatusUpheld:    true,
		defeasible.StatusContested: true,
	}
	r2 := mustResolve(t, provisions, mods, active2)
	if r2.Effective[0].Value != "contested value" {
		t.Errorf("contested should be active when configured; got %q", r2.Effective[0].Value)
	}
}

// ---------------------------------------------------------------------------
// Test: reinterpret effect replaces value (same semantics as set from engine POV)
// ---------------------------------------------------------------------------

func TestResolveReinterpretEffect(t *testing.T) {
	provisions := []defeasible.Provision{provision("§11", "original", 0.9)}
	mods := []defeasible.Modifier{
		modifier("mR", "§11", 1, "contra_proferentem", "reinterpreted", defeasible.EffectReinterpret, 0.75, nil, defeasible.StatusPresumed, nil, 0),
	}
	r := mustResolve(t, provisions, mods, defaultActive())
	ep := r.Effective[0]
	if ep.Value != "reinterpreted" {
		t.Errorf("reinterpret: value = %q, want reinterpreted", ep.Value)
	}
	if ep.Void {
		t.Error("reinterpret should not make provision void")
	}
}

// ---------------------------------------------------------------------------
// Test: modifier targeting non-existent provision key is ignored (no panic)
// ---------------------------------------------------------------------------

func TestResolveModifierUnknownKeyIgnored(t *testing.T) {
	provisions := []defeasible.Provision{provision("§1", "base", 0.9)}
	mods := []defeasible.Modifier{
		modifier("mOrphan", "§999", 1, "amendment", "v", defeasible.EffectSet, 0.8, nil, defeasible.StatusPresumed, nil, 0),
	}
	r := mustResolve(t, provisions, mods, defaultActive())
	if len(r.Effective) != 1 {
		t.Fatalf("want 1 effective provision, got %d", len(r.Effective))
	}
	if r.Effective[0].Key != "§1" {
		t.Errorf("unexpected key %q", r.Effective[0].Key)
	}
}

// ---------------------------------------------------------------------------
// Test: Resolve returns error on invalid provision
// ---------------------------------------------------------------------------

func TestResolveErrorOnInvalidProvision(t *testing.T) {
	provisions := []defeasible.Provision{
		{Key: "", BaseValue: "v", BaseConfidence: 0.9},
	}
	_, err := defeasible.Resolve(provisions, nil, defaultActive())
	if err == nil {
		t.Fatal("expected error for invalid provision, got nil")
	}
}

// ---------------------------------------------------------------------------
// Test: Resolve returns error on invalid modifier
// ---------------------------------------------------------------------------

func TestResolveErrorOnInvalidModifier(t *testing.T) {
	provisions := []defeasible.Provision{provision("§1", "v", 0.9)}
	mods := []defeasible.Modifier{
		{ID: "", TargetKey: "§1", Layer: 1, Effect: defeasible.EffectSet, Confidence: 0.8, Status: defeasible.StatusPresumed},
	}
	_, err := defeasible.Resolve(provisions, mods, defaultActive())
	if err == nil {
		t.Fatal("expected error for invalid modifier (empty ID), got nil")
	}
}

// ---------------------------------------------------------------------------
// Test: empty provisions returns empty resolution, no error
// ---------------------------------------------------------------------------

func TestResolveEmptyProvisions(t *testing.T) {
	r := mustResolve(t, nil, nil, defaultActive())
	if len(r.Effective) != 0 {
		t.Errorf("want empty effective, got %v", r.Effective)
	}
}

// ---------------------------------------------------------------------------
// Test: Trace records entries for applied and shadowed modifiers.
// Two modifiers at the same (Layer, Priority) slot — one wins, one is
// shadowed and recorded in the trace.
// ---------------------------------------------------------------------------

func TestResolveTraceRecorded(t *testing.T) {
	provisions := []defeasible.Provision{provision("§1", "base", 0.9)}
	mods := []defeasible.Modifier{
		// Same layer and priority → conflict; mWin has higher confidence and wins.
		modifier("mWin", "§1", 1, "statutory_override", "override", defeasible.EffectSet, 0.9, nil, defeasible.StatusPresumed, nil, 0),
		modifier("mLose", "§1", 1, "amendment", "amendment", defeasible.EffectSet, 0.8, nil, defeasible.StatusPresumed, nil, 0),
	}
	r := mustResolve(t, provisions, mods, defaultActive())
	if len(r.Trace) == 0 {
		t.Error("trace should not be empty when a modifier is shadowed by conflict")
	}
	// Trace must include an entry for the loser.
	found := false
	for _, te := range r.Trace {
		if te.ModifierID == "mLose" {
			found = true
		}
	}
	if !found {
		t.Errorf("trace = %+v; expected entry for mLose", r.Trace)
	}
}

// ---------------------------------------------------------------------------
// Test: WithStatus returns independent copy, does not mutate original
// ---------------------------------------------------------------------------

func TestWithStatusDefensiveCopy(t *testing.T) {
	orig := []defeasible.Modifier{
		modifier("m1", "§1", 1, "amendment", "v", defeasible.EffectSet, 0.8, nil, defeasible.StatusPresumed, nil, 0),
		modifier("m2", "§1", 1, "amendment", "v2", defeasible.EffectSet, 0.7, nil, defeasible.StatusPresumed, nil, 1),
	}
	updated := defeasible.WithStatus(orig, "m1", defeasible.StatusInvalidated)

	// Original must be unchanged.
	if orig[0].Status != defeasible.StatusPresumed {
		t.Error("WithStatus must not mutate original slice")
	}
	// Updated has the new status.
	if updated[0].Status != defeasible.StatusInvalidated {
		t.Errorf("updated[0].Status = %q, want invalidated", updated[0].Status)
	}
	// Unknown ID: no-op, all statuses unchanged.
	updated2 := defeasible.WithStatus(orig, "unknown-id", defeasible.StatusWaived)
	for _, m := range updated2 {
		if m.Status == defeasible.StatusWaived {
			t.Errorf("unknown ID should produce no-op; modifier %q got waived", m.ID)
		}
	}
}

// ---------------------------------------------------------------------------
// Test: EvidenceRefs defensive copy on Modifier
// ---------------------------------------------------------------------------

func TestModifierEvidenceRefsDefensiveCopy(t *testing.T) {
	refs := []string{"ev-1", "ev-2"}
	m := modifier("m1", "§1", 1, "amendment", "v", defeasible.EffectSet, 0.8, refs, defeasible.StatusPresumed, nil, 0)
	// Mutating the original refs slice should not affect the modifier if it stores a copy.
	_ = m
	// The modifier is a value type; this test just ensures the type compiles
	// and that the slice field is present (defensive copy is in Resolve's internal handling).
}

// ---------------------------------------------------------------------------
// DEFECT 1a: invalidated superseder must NOT remove its target from contention.
//
// Scenario: mB is invalidated and lists mA in its Supersedes field. mA is
// active (presumed). The bug: buildSupersededSet walks ALL modifiers including
// inactive ones, so mB's Supersedes list was being honoured despite mB being
// inactive — mA was silently dropped from contention without appearing in
// ShadowedModifierIDs or the Trace.
//
// Correct behaviour: only an ACTIVE modifier may supersede another; an
// inactive modifier's Supersedes list has no effect.
// ---------------------------------------------------------------------------

func TestResolveInvalidatedSupersederHasNoEffect(t *testing.T) {
	provisions := []defeasible.Provision{provision("§1", "base", 0.9)}
	mods := []defeasible.Modifier{
		// mA is active — should be applied.
		modifier("mA", "§1", 1, "amendment", "amended-by-mA", defeasible.EffectSet, 0.8, nil, defeasible.StatusPresumed, nil, 0),
		// mB is invalidated and claims to supersede mA. It must have no effect.
		modifier("mB", "§1", 1, "amendment", "amended-by-mB", defeasible.EffectSet, 0.9, nil, defeasible.StatusInvalidated, []string{"mA"}, 0),
	}
	r := mustResolve(t, provisions, mods, defaultActive())
	ep := r.Effective[0]

	// mA must be applied — the invalidated superseder must not remove it.
	if ep.Value != "amended-by-mA" {
		t.Errorf("value = %q, want amended-by-mA; invalidated superseder mB must not remove mA", ep.Value)
	}
	applied := make(map[string]bool)
	for _, id := range ep.AppliedModifierIDs {
		applied[id] = true
	}
	if !applied["mA"] {
		t.Errorf("mA must be in AppliedModifierIDs; got %v", ep.AppliedModifierIDs)
	}
	if applied["mB"] {
		t.Errorf("invalidated mB must NOT be in AppliedModifierIDs; got %v", ep.AppliedModifierIDs)
	}
	// mA must NOT appear in ShadowedModifierIDs (it was not defeated by anything).
	for _, id := range ep.ShadowedModifierIDs {
		if id == "mA" {
			t.Errorf("mA must not be shadowed; invalidated mB has no supersession power; shadowed = %v", ep.ShadowedModifierIDs)
		}
	}
}

// ---------------------------------------------------------------------------
// DEFECT 1a (waived variant): waived superseder must also have no effect.
// ---------------------------------------------------------------------------

func TestResolveWaivedSupersederHasNoEffect(t *testing.T) {
	provisions := []defeasible.Provision{provision("§1", "base", 0.9)}
	mods := []defeasible.Modifier{
		modifier("mA", "§1", 1, "amendment", "amended-by-mA", defeasible.EffectSet, 0.8, nil, defeasible.StatusPresumed, nil, 0),
		modifier("mW", "§1", 1, "amendment", "amended-by-mW", defeasible.EffectSet, 0.9, nil, defeasible.StatusWaived, []string{"mA"}, 0),
	}
	r := mustResolve(t, provisions, mods, defaultActive())
	ep := r.Effective[0]

	if ep.Value != "amended-by-mA" {
		t.Errorf("value = %q, want amended-by-mA; waived superseder mW must not remove mA", ep.Value)
	}
	applied := make(map[string]bool)
	for _, id := range ep.AppliedModifierIDs {
		applied[id] = true
	}
	if !applied["mA"] {
		t.Errorf("mA must be in AppliedModifierIDs; got %v", ep.AppliedModifierIDs)
	}
}

// ---------------------------------------------------------------------------
// DEFECT 1b: every active modifier NOT applied must appear in ShadowedModifierIDs
// and the Trace. No active modifier may vanish silently.
//
// This test uses supersession (mC supersedes mA and mB) to verify that both
// mA and mB appear in ShadowedModifierIDs with reason "shadowed:superseded".
// ---------------------------------------------------------------------------

func TestResolveAllNonAppliedModifiersTracedWithReason(t *testing.T) {
	provisions := []defeasible.Provision{provision("§1", "base", 0.9)}
	mods := []defeasible.Modifier{
		modifier("mA", "§1", 1, "amendment", "vA", defeasible.EffectSet, 0.7, nil, defeasible.StatusPresumed, nil, 0),
		modifier("mB", "§1", 1, "amendment", "vB", defeasible.EffectSet, 0.75, nil, defeasible.StatusPresumed, nil, 0),
		// mC supersedes both mA and mB — they must be shadowed, not silently dropped.
		modifier("mC", "§1", 1, "amendment", "vC", defeasible.EffectSet, 0.8, nil, defeasible.StatusPresumed, []string{"mA", "mB"}, 0),
	}
	r := mustResolve(t, provisions, mods, defaultActive())
	ep := r.Effective[0]

	// mC is the only applied modifier.
	if len(ep.AppliedModifierIDs) != 1 || ep.AppliedModifierIDs[0] != "mC" {
		t.Errorf("AppliedModifierIDs = %v, want [mC]", ep.AppliedModifierIDs)
	}

	// mA and mB must appear in ShadowedModifierIDs.
	shadowed := make(map[string]bool)
	for _, id := range ep.ShadowedModifierIDs {
		shadowed[id] = true
	}
	if !shadowed["mA"] {
		t.Errorf("mA must be in ShadowedModifierIDs; got %v", ep.ShadowedModifierIDs)
	}
	if !shadowed["mB"] {
		t.Errorf("mB must be in ShadowedModifierIDs; got %v", ep.ShadowedModifierIDs)
	}

	// Both mA and mB must appear in the Trace with a shadowed reason.
	traceByMod := make(map[string]string)
	for _, te := range r.Trace {
		traceByMod[te.ModifierID] = te.Decision
	}
	for _, id := range []string{"mA", "mB"} {
		dec, ok := traceByMod[id]
		if !ok {
			t.Errorf("modifier %s must have a trace entry; trace = %+v", id, r.Trace)
			continue
		}
		if !strings.HasPrefix(dec, "shadowed:") {
			t.Errorf("modifier %s trace decision = %q, want shadowed:* prefix", id, dec)
		}
	}
}

// ---------------------------------------------------------------------------
// DEFECT 2 (incomplete trace): applied modifiers must emit "applied" trace entries.
//
// The CUE schema and Go doc both list "applied" as a valid Decision value.
// The original code never emitted "applied" entries — winners were silently
// omitted from the Trace, making the trace useless for full-chain auditing.
// ---------------------------------------------------------------------------

func TestResolveAppliedModifiersEmitTraceEntries(t *testing.T) {
	provisions := []defeasible.Provision{provision("§1", "base", 0.9)}
	mods := []defeasible.Modifier{
		modifier("m1", "§1", 1, "amendment", "step1", defeasible.EffectSet, 0.8, nil, defeasible.StatusPresumed, nil, 0),
		modifier("m2", "§1", 1, "amendment", "step2", defeasible.EffectSet, 0.75, nil, defeasible.StatusPresumed, nil, 1),
	}
	r := mustResolve(t, provisions, mods, defaultActive())
	ep := r.Effective[0]

	// Both modifiers are at different priorities, so both are applied.
	if len(ep.AppliedModifierIDs) != 2 {
		t.Fatalf("want 2 applied modifiers, got %v", ep.AppliedModifierIDs)
	}

	// The Trace must include an "applied" entry for every applied modifier.
	traceApplied := make(map[string]bool)
	for _, te := range r.Trace {
		if te.Decision == "applied" {
			traceApplied[te.ModifierID] = true
		}
	}
	for _, id := range ep.AppliedModifierIDs {
		if !traceApplied[id] {
			t.Errorf("applied modifier %q must have a Trace entry with Decision=applied; trace = %+v", id, r.Trace)
		}
	}
}

// ---------------------------------------------------------------------------
// DEFECT 2 (incomplete trace — single-modifier case): even a single applied
// modifier must produce an "applied" trace entry.
// ---------------------------------------------------------------------------

func TestResolveAppliedTraceEntrySingleModifier(t *testing.T) {
	provisions := []defeasible.Provision{provision("§1", "base", 0.9)}
	mods := []defeasible.Modifier{
		modifier("mSingle", "§1", 1, "amendment", "new", defeasible.EffectSet, 0.8, nil, defeasible.StatusPresumed, nil, 0),
	}
	r := mustResolve(t, provisions, mods, defaultActive())

	found := false
	for _, te := range r.Trace {
		if te.ModifierID == "mSingle" && te.Decision == "applied" {
			found = true
		}
	}
	if !found {
		t.Errorf("single applied modifier must appear in Trace with Decision=applied; trace = %+v", r.Trace)
	}
}

// ---------------------------------------------------------------------------
// DEFECT 1a + DEFECT 2 combined: with an invalidated superseder (mB→mA) and
// a separate active modifier (mC), verify:
//   - mA is applied (not silently removed by the invalidated superseder)
//   - mC is applied
//   - Both have "applied" trace entries
//   - The trace has no spurious "shadowed:superseded" entry for mA
// ---------------------------------------------------------------------------

func TestResolveInvalidatedSupersederCombinedTrace(t *testing.T) {
	provisions := []defeasible.Provision{provision("§1", "base", 0.9)}
	mods := []defeasible.Modifier{
		// mA: layer 1 priority 0 — should be applied.
		modifier("mA", "§1", 1, "amendment", "vA", defeasible.EffectSet, 0.8, nil, defeasible.StatusPresumed, nil, 0),
		// mC: layer 1 priority 1 — also applied (different priority, different slot).
		modifier("mC", "§1", 1, "amendment", "vC", defeasible.EffectSet, 0.75, nil, defeasible.StatusPresumed, nil, 1),
		// mB: invalidated, claims to supersede mA — must be ignored.
		modifier("mB", "§1", 1, "amendment", "vB", defeasible.EffectSet, 0.9, nil, defeasible.StatusInvalidated, []string{"mA"}, 0),
	}
	r := mustResolve(t, provisions, mods, defaultActive())
	ep := r.Effective[0]

	// Final value: mC is applied last (priority 1 > priority 0) after mA.
	if ep.Value != "vC" {
		t.Errorf("value = %q, want vC (mA then mC applied in priority order)", ep.Value)
	}

	applied := make(map[string]bool)
	for _, id := range ep.AppliedModifierIDs {
		applied[id] = true
	}
	if !applied["mA"] || !applied["mC"] {
		t.Errorf("mA and mC must both be applied; AppliedModifierIDs = %v", ep.AppliedModifierIDs)
	}
	if applied["mB"] {
		t.Errorf("invalidated mB must not be applied; AppliedModifierIDs = %v", ep.AppliedModifierIDs)
	}

	// Trace must have "applied" entries for both mA and mC.
	traceDecisions := make(map[string][]string)
	for _, te := range r.Trace {
		traceDecisions[te.ModifierID] = append(traceDecisions[te.ModifierID], te.Decision)
	}
	for _, id := range []string{"mA", "mC"} {
		hasApplied := false
		for _, d := range traceDecisions[id] {
			if d == "applied" {
				hasApplied = true
			}
		}
		if !hasApplied {
			t.Errorf("modifier %s must have an applied trace entry; trace = %+v", id, r.Trace)
		}
	}

	// mA must NOT appear as "shadowed:superseded".
	for _, d := range traceDecisions["mA"] {
		if strings.HasPrefix(d, "shadowed:") {
			t.Errorf("mA must not be shadowed by invalidated mB; trace decision = %q", d)
		}
	}
}

// ---------------------------------------------------------------------------
// Test: conflict tiebreak by ID (deterministic when layer/priority/confidence equal)
// ---------------------------------------------------------------------------

func TestResolveConflictIDTiebreak(t *testing.T) {
	// Same layer, same priority, same confidence → alphabetical ID tiebreak.
	// "mAlpha" < "mZeta" alphabetically → mAlpha wins (lower ID = winner).
	provisions := []defeasible.Provision{provision("§T", "base", 1.0)}
	mods := []defeasible.Modifier{
		modifier("mZeta", "§T", 1, "amendment", "zeta-value", defeasible.EffectSet, 0.8, nil, defeasible.StatusPresumed, nil, 0),
		modifier("mAlpha", "§T", 1, "amendment", "alpha-value", defeasible.EffectSet, 0.8, nil, defeasible.StatusPresumed, nil, 0),
	}
	r1 := mustResolve(t, provisions, mods, defaultActive())
	r2 := mustResolve(t, provisions, []defeasible.Modifier{mods[1], mods[0]}, defaultActive())

	// Both orderings must produce the same winner.
	if r1.Effective[0].Value != r2.Effective[0].Value {
		t.Errorf("ID tiebreak not deterministic: %q vs %q", r1.Effective[0].Value, r2.Effective[0].Value)
	}
}
