package semantic

// decisionrank.go — DP-7 (docs/plans/decision-port.md, ADR-042 decision 2,
// vision 32 §2 item 2): the obligations engine's decision-port re-rank.
// ADR-035's governing-scope tier law (governingscope.go) stays untouchable —
// "an exact-file rule outranks a package rule outranks a root rule, and no
// model may change that" (vision 32 §2). A decision.Decider may only
// re-order rules STRICTLY INSIDE one tier, by how confidently it answers
// "does this rule govern this edit" for each — never across tiers, never by
// dropping one.

import (
	"context"
	"sort"
	"strings"

	"github.com/geoffmilleraz/agentic-kit/foundations/decision"
	"github.com/geoffmilleraz/fathom/internal/core"
)

// rerankWithinTiers asks d one batched decision.KindNoul question per
// matched claim — "does this rule govern the edit described in the state" —
// and returns matched re-ordered ONLY inside each rule's own
// governing-scope tier (scored.governing.tier): a rule never crosses a
// tier and never drops (every input claim is present in the output,
// exactly once). Within a tier, a rule whose noul answer clears I42's
// unsure-band floor (usableNoul, decision.UnsureBelow) is ordered by
// descending noul, stable on ties; every other rule in that tier (no
// answer at all, or an answer below the floor) keeps its EXACT original
// position — I42: "a decision in the unsure band never changes served
// content."
//
// d is assumed non-nil and RankWithinTier-gated by the caller
// (obligationsCore never calls this otherwise); a nil d (or an empty
// matched) still degrades safely — matched returned unchanged, no call —
// so a direct unit test may pass one without special-casing it. A
// provider error (including decision.ErrNoProvider, e.g. a decision.None
// Decider) is returned as err and matched is returned UNCHANGED —
// obligationsCore swallows that error into an untouched, RankedBy:"none"
// envelope (ADR-042 decision 3: "serving must not fail because a ranker
// did"); this function itself never hides it.
//
// relevance carries every claim ID the Decider answered (its RAW noul
// value, regardless of the unsure-band floor — that floor gates ORDERING
// only, never whether the served payload discloses the answer) for
// obligationsCore to attach as each served Obligation.Relevance. records
// is resp.Records for the caller (DP-3) to log to the call log.
func rerankWithinTiers(ctx context.Context, d decision.Decider, file, hunk string, matched []scored) (reordered []scored, relevance map[string]float64, records []decision.Record, err error) {
	if d == nil || len(matched) == 0 {
		return matched, nil, nil, nil
	}
	req := decision.Request{State: rerankState(file, hunk)}
	for _, m := range matched {
		req.Questions = append(req.Questions, decision.Question{
			ID:           m.cl.ClaimID,
			Kind:         decision.KindNoul,
			Instructions: "Does this rule govern the edit described in the state — would an editor of this file have to follow it here?",
			TrueCriteria: rerankCriteria(m.cl),
		})
	}
	resp, derr := d.Decide(ctx, req)
	if derr != nil {
		return matched, nil, nil, derr
	}
	relevance = make(map[string]float64, len(resp.Answers))
	for id, a := range resp.Answers {
		if a.Kind == decision.KindNoul {
			relevance[id] = a.Noul
		}
	}
	return reorderByNoulWithinTier(matched, relevance), relevance, resp.Records("obligations"), nil
}

// rerankState renders the State text a rerank Request's questions are asked
// about: the edited path, plus the hunk when the hook supplied one.
func rerankState(file, hunk string) string {
	if hunk == "" {
		return "file: " + file
	}
	return "file: " + file + "\n\nhunk:\n" + hunk
}

// rerankCriteria is one claim's noul-question TrueCriteria: the rule text
// and its scope, so the Decider judges against the actual governing text
// rather than the claim ID alone.
func rerankCriteria(cl core.SemanticClaim) string {
	return "Rule: " + cl.Rule + "\nScope: " + strings.Join(cl.Scope, ", ")
}

// usableNoul reports id's noul answer from relevance and whether it clears
// I42's unsure-band floor (decision.UnsureBelow, 0.5) — the only answers
// rerankWithinTiers' own reorder (and obligationsCore's less()) ever act
// on. An absent answer, or one below the floor, always reports ok=false:
// the caller's claim never moves because of it.
func usableNoul(id string, relevance map[string]float64) (float64, bool) {
	v, ok := relevance[id]
	if !ok || v < decision.UnsureBelow {
		return 0, false
	}
	return v, true
}

// reorderByNoulWithinTier permutes matched ONLY among index positions that
// share the same governing.tier, and only among the USABLE-noul subset of
// that tier: every unusable-noul rule (no answer, or an unsure-band
// answer) is left at its EXACT original index; the usable rules are
// stable-sorted by descending noul and written back into the (usable-only)
// positions they already occupied, in that new order.
//
// This is deliberately NOT a single whole-tier comparator sort: mixing
// "no meaningful answer, never move me" items into an ordinary pairwise
// comparator produces an ill-defined (non-transitive) relation once a
// tier holds more than one usable item interleaved with an unusable one.
// Extracting the usable subsequence and sorting only among itself keeps
// this a plain, well-defined total order over floats.
func reorderByNoulWithinTier(matched []scored, relevance map[string]float64) []scored {
	out := append([]scored(nil), matched...)

	byTier := map[int][]int{}
	var tierOrder []int
	for i, m := range out {
		t := m.governing.tier
		if _, seen := byTier[t]; !seen {
			tierOrder = append(tierOrder, t)
		}
		byTier[t] = append(byTier[t], i)
	}

	for _, t := range tierOrder {
		idxs := byTier[t]
		var usableIdxs []int
		var usableItems []scored
		for _, idx := range idxs {
			if _, ok := usableNoul(out[idx].cl.ClaimID, relevance); ok {
				usableIdxs = append(usableIdxs, idx)
				usableItems = append(usableItems, out[idx])
			}
		}
		if len(usableItems) < 2 {
			continue // 0 or 1 usable rule in this tier: nothing to reorder.
		}
		sort.SliceStable(usableItems, func(i, j int) bool {
			ni, _ := usableNoul(usableItems[i].cl.ClaimID, relevance)
			nj, _ := usableNoul(usableItems[j].cl.ClaimID, relevance)
			return ni > nj
		})
		for k, idx := range usableIdxs {
			out[idx] = usableItems[k]
		}
	}
	return out
}
