// ruleremedies.go — GL-19 (docs/plans/governance-learning.md, design doc
// §5's class table): ruleRemedies is the filtered, evidence-carrying view
// over GL-11's RuleEffectiveness results — "ignored rules proposed for a
// hook, never-triggered rules proposed for pruning, each with its
// evidence." Of the table's four classes only two carry an ACTIONABLE
// remedy (ignored -> move to a hook or rewrite; never_triggered -> prune
// or re-scope); effective's remedy is "keep" and contested's is "human
// review" — neither is a change to propose, so neither class reaches this
// list. A proposal only (GL-X1/GL-X2): nothing here writes a rules file,
// flips a claim's status, or changes lifecycle — RuleEffectiveness (GL-11)
// already computed the SAME class and remedy string this reads; this file
// only filters and cites evidence, never reclassifies.
package semantic

import "fmt"

// RuleRemedy is one proposed, human-reviewable remedy for a served rule.
type RuleRemedy struct {
	RuleID string
	// Class is always RuleClassIgnored or RuleClassNeverTriggered — the
	// only two classes ruleRemedies ever returns.
	Class RuleEffectivenessClass
	// Remedy is RuleEffectivenessResult.Remedy verbatim, never re-derived.
	Remedy string
	// Evidence cites the counts that justified the proposal, so a reader
	// (or `fathom intent audit`, a later caller) sees the signal beside
	// the suggestion rather than having to recompute it.
	Evidence string
}

// ruleRemedies filters results to the ignored and never_triggered classes
// and attaches each one's evidence. Input order is already RuleID-sorted
// (RuleEffectiveness's own contract); filtering preserves that order.
func ruleRemedies(results []RuleEffectivenessResult) []RuleRemedy {
	var out []RuleRemedy
	for _, r := range results {
		switch r.Class {
		case RuleClassIgnored:
			ratio := 0.0
			if r.Served30 > 0 {
				ratio = 100 * float64(r.Violated30) / float64(r.Served30)
			}
			out = append(out, RuleRemedy{
				RuleID: r.RuleID, Class: r.Class, Remedy: r.Remedy,
				Evidence: fmt.Sprintf("violated %d/%d served (%.0f%%) over 30d, never blocked", r.Violated30, r.Served30, ratio),
			})
		case RuleClassNeverTriggered:
			out = append(out, RuleRemedy{
				RuleID: r.RuleID, Class: r.Class, Remedy: r.Remedy,
				Evidence: "not served in 90d",
			})
		}
	}
	return out
}
