// checkablerule.go — SR-45 (docs/plans/surface-reckoning.md, design doc §5
// row SR-45, F-26): a served LayerRule (VB-13, layerrule.go) is
// "checkable" exactly when its kind has a deterministic pre-write check —
// the SAME boundary EvaluateLayerRuleEnforcement already draws (enforce.go:
// only import_direction and boundary can be evaluated against a single
// proposed file's own text; no_cycles and cross_service_direction need the
// full index/fleet graph). guardFromCheckableRule compiles a checkable
// rule into an enforce:true guard so a caller (a layer-pack compiler, a
// `fathom rules` verb) can promote a served, advisory rule to enforcement
// without hand-editing `.fathom/guard.cue`; a rule this package cannot
// mechanically check is reported not-checkable (ok=false) instead of
// silently enforced — the "lint output" a caller renders for it.
package guardspec

// LayerRuleIsCheckable reports whether rule's kind has a deterministic
// pre-write check (EvaluateLayerRuleEnforcement) — the vocabulary the
// obligations answer marks a served rule with (SR-45): import_direction
// and boundary are; no_cycles and cross_service_direction are not.
func LayerRuleIsCheckable(rule LayerRule) bool {
	switch rule.Kind {
	case ImportDirection, Boundary:
		return true
	default:
		return false
	}
}

// guardFromCheckableRule compiles rule into an enforce:true LayerRule when
// it is checkable (LayerRuleIsCheckable), matching LoadLayerRules' own
// refusal for any other kind (ADR-038: "enforce:true is not supported for
// kind %q"). ok is false for a not-checkable rule; compiled is then the
// zero LayerRule, never a partially-enforced one. Idempotent: a rule
// already carrying Enforce:true compiles to itself unchanged.
func guardFromCheckableRule(rule LayerRule) (compiled LayerRule, ok bool) {
	if !LayerRuleIsCheckable(rule) {
		return LayerRule{}, false
	}
	compiled = rule
	compiled.Enforce = true
	return compiled, true
}
