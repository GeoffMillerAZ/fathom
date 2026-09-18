package findings

// disagreements.go — FL-23 (docs/vision/30-findings-loop.md §3.6, §4
// honesty rule 5, ADR-040 decision 4 rule 5): "contradictions are shown,
// never resolved by fathom … the pair is counted in `findings status` as
// `disagreements`." A disagreement here is a served (fresh/stale/
// contradicted) finding-kind claim from one producer sharing a normalized
// rule shape and anchor path with ANOTHER producer's claim that was
// superseded specifically via an "external-closure" resolution (ADR-040
// decision 2's own vocabulary: the review kit closed it, not a native
// re-derivation) — i.e. two producers disagree about whether the same
// thing is still true.
//
// "Rule id" and "anchor path" are the plan's own words; core.SemanticClaim
// carries neither at this layer (both live only on the served
// semantic.Obligation, populated by a join the concurrent provenance-
// substrate wave owns) — this uses the closest available proxies
// (NormalizeRuleShape(Rule), the first Scope entry) and is named
// DisagreementsByRule throughout, never bare "disagreements", so the
// approximation is visible wherever the count is read.

import "github.com/geoffmilleraz/fathom/internal/core"

// disagreementGroupKey groups claims the same way a rule id + anchor path
// pair would: normalized rule shape, plus the claim's primary (first)
// scoped path.
type disagreementGroupKey struct {
	shape string
	path  string
}

// disagreementsByRule computes the count BuildStatus reports as
// disagreements_by_rule.
func disagreementsByRule(claims []core.SemanticClaim, events []core.ClaimEvent) int {
	eventsByClaim := make(map[string][]core.ClaimEvent, len(events))
	for _, ev := range events {
		eventsByClaim[ev.ClaimID] = append(eventsByClaim[ev.ClaimID], ev)
	}

	groups := map[disagreementGroupKey][]core.SemanticClaim{}
	for _, cl := range claims {
		path := ""
		if len(cl.Scope) > 0 {
			path = cl.Scope[0]
		}
		key := disagreementGroupKey{shape: NormalizeRuleShape(cl.Rule), path: path}
		groups[key] = append(groups[key], cl)
	}

	const externalClosure = "external-closure" // ADR-040 decision 2; core.Resolution* constants land with FL-3
	count := 0
	for _, members := range groups {
		servedProducers := map[string]bool{}
		externallyClosedProducers := map[string]bool{}
		for _, cl := range members {
			prod := producerOf(cl.Subject)
			switch cl.Status {
			case core.ClaimFresh, core.ClaimStale, core.ClaimContradicted:
				servedProducers[prod] = true
			case core.ClaimSuperseded:
				if kind, _ := latestSupersedeResolution(eventsByClaim[cl.ClaimID]); kind == externalClosure {
					externallyClosedProducers[prod] = true
				}
			}
		}
		disagree := false
		for served := range servedProducers {
			for closed := range externallyClosedProducers {
				if served != closed {
					disagree = true
				}
			}
		}
		if disagree {
			count++
		}
	}
	return count
}
