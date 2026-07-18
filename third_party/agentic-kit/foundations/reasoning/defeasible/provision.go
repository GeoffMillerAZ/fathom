package defeasible

import (
	"fmt"
	"sort"
)

// resolveProvision applies eligible modifiers to a single provision and
// returns the effective provision and trace entries.
//
// eligible: active modifiers for this provision that are NOT globally superseded.
// supersededIDs: active modifier IDs removed from contention by supersession;
// included in ShadowedModifierIDs and the trace for audit purposes.
//
// # Conflict model
//
// Modifiers are grouped by (Layer, Priority). Within each group, the modifier
// with the highest Confidence wins (alphabetically lower ID breaks ties).
// Losers in the same group are "conflict shadowed". Modifiers in different
// groups form an ordered application chain (ascending Layer, then Priority).
//
// # Confidence
//
// Weakest-link: min(BaseConfidence, each chain-member's Confidence).
func resolveProvision(p Provision, eligible []Modifier, supersededIDs map[string]bool) (EffectiveProvision, []TraceEntry) {
	var trace []TraceEntry

	// Record superseded modifiers in the trace.
	for id := range supersededIDs {
		trace = append(trace, TraceEntry{
			ProvisionKey: p.Key,
			ModifierID:   id,
			Decision:     "shadowed:superseded",
		})
	}

	if len(eligible) == 0 {
		return EffectiveProvision{
			Key:                 p.Key,
			Value:               p.BaseValue,
			Confidence:          p.BaseConfidence,
			AppliedModifierIDs:  []string{},
			ShadowedModifierIDs: sortedKeys(supersededIDs),
		}, trace
	}

	// Sort by (Layer asc, Priority asc, Confidence asc, ID asc).
	// This ordering groups same-(Layer,Priority) members together and ensures
	// the within-group winner is selected consistently.
	sorted := make([]Modifier, len(eligible))
	copy(sorted, eligible)
	sort.Slice(sorted, func(i, j int) bool {
		a, b := sorted[i], sorted[j]
		if a.Layer != b.Layer {
			return a.Layer < b.Layer
		}
		if a.Priority != b.Priority {
			return a.Priority < b.Priority
		}
		if a.Confidence != b.Confidence {
			return a.Confidence < b.Confidence
		}
		return a.ID < b.ID
	})

	winners, conflictLosers := selectWinners(sorted)

	// Record conflict losers in the trace.
	for _, loser := range conflictLosers {
		winnerID := findGroupWinner(winners, loser.Layer, loser.Priority)
		trace = append(trace, TraceEntry{
			ProvisionKey: p.Key,
			ModifierID:   loser.ID,
			Decision: fmt.Sprintf(
				"shadowed:conflict:beaten by %s (layer=%d priority=%d confidence=%g)",
				winnerID, loser.Layer, loser.Priority, loser.Confidence,
			),
		})
	}

	// Apply winners in chain order; track weakest-link confidence.
	conf := p.BaseConfidence
	value := p.BaseValue
	void := false
	var appliedIDs []string

	for _, w := range winners {
		conf = minConfidence(conf, w.Confidence)
		switch w.Effect {
		case EffectStrike:
			void = true
			value = ""
		case EffectSet, EffectReinterpret:
			if !void {
				value = w.NewValue
			}
		}
		appliedIDs = append(appliedIDs, w.ID)
		trace = append(trace, TraceEntry{
			ProvisionKey: p.Key,
			ModifierID:   w.ID,
			Decision:     "applied",
		})
	}

	// Combine superseded + conflict loser IDs into a single sorted list.
	shadowedIDs := make([]string, 0, len(supersededIDs)+len(conflictLosers))
	for id := range supersededIDs {
		shadowedIDs = append(shadowedIDs, id)
	}
	for _, l := range conflictLosers {
		shadowedIDs = append(shadowedIDs, l.ID)
	}
	sort.Strings(shadowedIDs)

	// Stable trace sort.
	sort.Slice(trace, func(i, j int) bool {
		if trace[i].ModifierID != trace[j].ModifierID {
			return trace[i].ModifierID < trace[j].ModifierID
		}
		return trace[i].Decision < trace[j].Decision
	})

	return EffectiveProvision{
		Key:                 p.Key,
		Value:               value,
		Void:                void,
		Confidence:          conf,
		AppliedModifierIDs:  appliedIDs,
		ShadowedModifierIDs: shadowedIDs,
	}, trace
}

// selectWinners partitions sorted modifiers into per-(Layer,Priority) group
// winners and conflict losers. Input must already be sorted by
// (Layer, Priority, Confidence, ID).
func selectWinners(sorted []Modifier) (winners, losers []Modifier) {
	type groupKey struct {
		Layer    int
		Priority int
	}

	// Preserve group order from the sorted slice.
	var groupOrder []groupKey
	groupMembers := make(map[groupKey][]Modifier)
	seenGroup := make(map[groupKey]bool)

	for _, m := range sorted {
		gk := groupKey{m.Layer, m.Priority}
		if !seenGroup[gk] {
			seenGroup[gk] = true
			groupOrder = append(groupOrder, gk)
		}
		groupMembers[gk] = append(groupMembers[gk], m)
	}

	for _, gk := range groupOrder {
		members := groupMembers[gk]
		if len(members) == 1 {
			winners = append(winners, members[0])
			continue
		}
		// Pick winner: highest Confidence; alphabetically lower ID as tiebreak.
		best := members[0]
		for _, m := range members[1:] {
			if m.Confidence > best.Confidence || (m.Confidence == best.Confidence && m.ID < best.ID) {
				best = m
			}
		}
		winners = append(winners, best)
		for _, m := range members {
			if m.ID != best.ID {
				losers = append(losers, m)
			}
		}
	}
	return winners, losers
}

// findGroupWinner returns the winner ID in the given (Layer, Priority) group
// from the winners slice. Returns "" if not found (should never happen in
// normal operation).
func findGroupWinner(winners []Modifier, layer, priority int) string {
	for _, w := range winners {
		if w.Layer == layer && w.Priority == priority {
			return w.ID
		}
	}
	return ""
}

// minConfidence returns the smaller of a and b (weakest-link bounding).
func minConfidence(a, b float64) float64 {
	if b < a {
		return b
	}
	return a
}

// sortedKeys returns the keys of a bool map as a sorted string slice.
// Returns a non-nil empty slice when m is empty.
func sortedKeys(m map[string]bool) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
