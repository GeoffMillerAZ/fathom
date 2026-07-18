package defeasible

import (
	"fmt"
	"sort"
)

// DefaultActiveStatuses is the standard set of modifier statuses treated as
// active when callers do not need custom configuration. Invalidated and waived
// modifiers are never active regardless of the map.
var DefaultActiveStatuses = map[Status]bool{
	StatusPresumed: true,
	StatusUpheld:   true,
}

// Resolve computes the effective state of every provision by applying all
// active modifiers in precedence order.
//
// # Active modifiers
//
// A modifier is active when (a) its Status key maps to true in activeStatuses
// AND (b) its Status is not StatusInvalidated or StatusWaived — those two are
// hard-excluded regardless of the map.
//
// # Application order
//
// For each provision, the eligible (active, non-superseded) modifiers are
// sorted ascending by (Layer, Priority, Confidence asc, ID asc) and grouped
// by (Layer, Priority). Within each group only one modifier is active (the
// highest-Confidence one; alphabetically lower ID breaks ties). Modifiers at
// different (Layer, Priority) slots form an ordered application chain.
//
// # Supersession
//
// Before conflict resolution, the full Supersedes DAG is traversed. Any
// modifier whose ID is transitively named by another active modifier's
// Supersedes list is removed from contention. DAG cycles are an error.
//
// # Conflict resolution
//
// Two modifiers "conflict" when they share the same (Layer, Priority) slot.
// The winner is the one with the highest Confidence; alphabetically lower ID
// is the final deterministic tiebreak. The loser is recorded as Shadowed with
// a reason.
//
// # Confidence
//
// EffectiveConfidence is the weakest-link minimum across the base provision
// confidence and the confidence of every modifier in the applied chain.
//
// # Determinism
//
// Resolve is pure: given the same inputs in any order it produces identical
// output. Input slices are never mutated.
func Resolve(provisions []Provision, modifiers []Modifier, activeStatuses map[Status]bool) (Resolution, error) {
	for i, p := range provisions {
		if err := p.Validate(); err != nil {
			return Resolution{}, fmt.Errorf("defeasible: provisions[%d]: %w", i, err)
		}
	}
	for i, m := range modifiers {
		if err := m.Validate(); err != nil {
			return Resolution{}, fmt.Errorf("defeasible: modifiers[%d]: %w", i, err)
		}
	}

	if err := detectSupersessionCycle(modifiers); err != nil {
		return Resolution{}, err
	}

	supersededGlobal := buildSupersededSet(modifiers, activeStatuses)
	supersededPerTarget := computeSupersededPerTarget(modifiers, activeStatuses)

	// Sort provision keys for deterministic output order.
	provKeys := make([]string, len(provisions))
	for i, p := range provisions {
		provKeys[i] = p.Key
	}
	sort.Strings(provKeys)

	provByKey := make(map[string]Provision, len(provisions))
	for _, p := range provisions {
		provByKey[p.Key] = p
	}

	// Group active, non-globally-superseded modifiers by TargetKey.
	// Only active modifiers may supersede others; inactive supersession
	// links are ignored by buildSupersededSet.
	modsByTarget := make(map[string][]Modifier)
	for _, m := range modifiers {
		if !isActive(m, activeStatuses) || supersededGlobal[m.ID] {
			continue
		}
		modsByTarget[m.TargetKey] = append(modsByTarget[m.TargetKey], m)
	}

	var effective []EffectiveProvision
	var trace []TraceEntry

	for _, key := range provKeys {
		ep, entries := resolveProvision(provByKey[key], modsByTarget[key], supersededPerTarget[key])
		effective = append(effective, ep)
		trace = append(trace, entries...)
	}

	return Resolution{Effective: effective, Trace: trace}, nil
}

// isActive reports whether a modifier should be treated as active. Hard
// exclusions: StatusInvalidated and StatusWaived are never active regardless
// of the activeStatuses map.
func isActive(m Modifier, activeStatuses map[Status]bool) bool {
	if m.Status == StatusInvalidated || m.Status == StatusWaived {
		return false
	}
	return activeStatuses[m.Status]
}

// WithStatus returns a defensive copy of modifiers with the named modifier's
// Status replaced by newStatus. If no modifier has the given id, all statuses
// are unchanged. The original slice is never mutated.
func WithStatus(modifiers []Modifier, id string, newStatus Status) []Modifier {
	out := make([]Modifier, len(modifiers))
	for i, m := range modifiers {
		m.EvidenceRefs = copyStrings(m.EvidenceRefs)
		m.Supersedes = copyStrings(m.Supersedes)
		if m.ID == id {
			m.Status = newStatus
		}
		out[i] = m
	}
	return out
}

// copyStrings returns a copy of s, or nil if s is nil.
func copyStrings(s []string) []string {
	if s == nil {
		return nil
	}
	c := make([]string, len(s))
	copy(c, s)
	return c
}
