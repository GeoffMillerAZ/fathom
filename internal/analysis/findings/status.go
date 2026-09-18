package findings

// status.go — §3.7's progress-bar report (docs/design/findings-as-
// evidence.md, §6 item 7): "the blocked [incident-investigation] module's
// entry condition, made visible... the bar is read from data, not
// asserted." The entry bar itself is docs/design/product-vision.md §3's own
// row: "Entry: >=15-20 resolved findings" for the incident-investigation
// module. A pure function over already-fetched claims/events, like every
// other file in this package — `fathom findings status` (cmd/fathom) does
// the I/O and calls this.

import (
	"sort"

	"github.com/geoffmilleraz/fathom/internal/core"
)

// EntryBarLow/EntryBarHigh are product-vision.md §3's own entry-condition
// range for the incident-investigation module ("Entry: >=15-20 resolved
// findings"). A range, not a single number, because the design itself
// states it as one — StatusReport reports against BOTH ends rather than
// picking one and hiding the range.
const (
	EntryBarLow  = 15
	EntryBarHigh = 20
)

// StatusReport is `fathom findings status`'s whole output — every number
// computed fresh from the claims/events the caller fetched, never asserted.
type StatusReport struct {
	// ResolvedFindingCount is the number of DISTINCT finding-claims with a
	// recorded, independent code-change resolution (the same definition
	// PatternMember.HasIndependentCodeChangeResolution uses) — a
	// refutation or scope-disappearance is not counted here: neither is
	// "this failure class recurs and gets fixed this way" precedent, the
	// exact bar product-vision.md §3 is measuring.
	ResolvedFindingCount int `json:"resolved_finding_count"`
	// DistinctClasses is the sorted set of core.FindingClass values seen
	// across every finding-claim passed in (any status — a class the
	// vocabulary is actually being used for, not just resolved ones).
	DistinctClasses []core.FindingClass `json:"distinct_classes"`
	// PatternsWithThreeResolvedMembers is the count of derived patterns
	// (DerivePatterns) whose ResolvedCodeChangeCount() >= 3 — the exact
	// promotion-rule threshold (§3.5), surfaced here as a standing count
	// rather than requiring a separate call.
	PatternsWithThreeResolvedMembers int `json:"patterns_with_three_resolved_members"`

	// Candidates (FL-23, §3.6) is the count of RuleCandidates at the
	// promotion-relevant min-sites floor of 3 — the entry condition for
	// "a recurring finding is worth lifting into a rule", read from data.
	Candidates int `json:"candidates"`
	// DisagreementsByRule (FL-23, §4 honesty rule 5: "disagreements are
	// shown, never resolved by fathom") counts groups of same-normalized-
	// rule-shape, same-anchor-path claims where a served member from one
	// producer disagrees with another producer's member superseded via an
	// "external-closure" resolution. Named "by_rule" rather than the plan's
	// bare "disagreements": core.SemanticClaim carries no per-claim rule id
	// of its own at this layer (that field lives only on the served
	// semantic.Obligation, populated by a join the concurrent provenance-
	// substrate wave owns) — NormalizeRuleShape(Rule) is the closest
	// available proxy, named honestly rather than passed off as exact.
	DisagreementsByRule int `json:"disagreements_by_rule"`

	EntryBarLow  int `json:"entry_bar_low"`
	EntryBarHigh int `json:"entry_bar_high"`
	// EntryConditionMet is true once ResolvedFindingCount >= EntryBarLow —
	// the LOW end of the range, the earliest point the module's entry
	// condition could be read as satisfied; Note always states where in
	// the range the count actually falls.
	EntryConditionMet bool   `json:"entry_condition_met"`
	Note              string `json:"note"`
}

// BuildStatus computes the §3.7 report from claims (expected already
// filtered to kind=="finding") and events (typically the store-wide
// claim_events population — ClaimEventsSince over a wide window).
func BuildStatus(claims []core.SemanticClaim, events []core.ClaimEvent) StatusReport {
	eventsByClaim := make(map[string][]core.ClaimEvent, len(events))
	for _, ev := range events {
		eventsByClaim[ev.ClaimID] = append(eventsByClaim[ev.ClaimID], ev)
	}

	classSeen := map[core.FindingClass]bool{}
	resolved := 0
	for _, cl := range claims {
		if cl.Class != "" {
			classSeen[cl.Class] = true
		}
		if cl.Status != core.ClaimSuperseded {
			continue
		}
		if kind, _ := latestSupersedeResolution(eventsByClaim[cl.ClaimID]); kind == core.ResolutionCodeChange {
			resolved++
		}
	}

	classes := make([]core.FindingClass, 0, len(classSeen))
	for c := range classSeen {
		classes = append(classes, c)
	}
	sort.Slice(classes, func(i, j int) bool { return classes[i] < classes[j] })

	patterns := DerivePatterns(claims, events)
	threePlus := 0
	for _, p := range patterns {
		if p.ResolvedCodeChangeCount() >= 3 {
			threePlus++
		}
	}

	r := StatusReport{
		ResolvedFindingCount:             resolved,
		DistinctClasses:                  classes,
		PatternsWithThreeResolvedMembers: threePlus,
		Candidates:                       len(RuleCandidates(claims, events, 3)),
		DisagreementsByRule:              disagreementsByRule(claims, events),
		EntryBarLow:                      EntryBarLow,
		EntryBarHigh:                     EntryBarHigh,
		EntryConditionMet:                resolved >= EntryBarLow,
	}
	switch {
	case resolved >= EntryBarHigh:
		r.Note = "entry condition met: at or above the top of the 15-20 resolved-finding range (product-vision.md §3)"
	case resolved >= EntryBarLow:
		r.Note = "entry condition met: within the 15-20 resolved-finding range, below its top (product-vision.md §3)"
	default:
		r.Note = "entry condition not yet met: below the 15-resolved-finding floor (product-vision.md §3)"
	}
	return r
}
