// Package narrativeshift detects when a declarant changed their story over
// time — the deposition-prep "prior inconsistent statement" gold. It operates
// as a pure stream query over assertion.Assertion values with no external
// dependencies, no clocks, and no I/O.
//
// CUE contracts live in schema.cue; this file is the Go counterpart and must
// stay in sync with that schema.
//
// Detection algorithm:
//  1. Group assertions by (Declarant, PropositionID).
//  2. Within each group sort by StatedAt, ties broken by ID (determinism).
//  3. For each consecutive pair that is incompatible, emit a NarrativeShift.
//  4. Output sorted by (Declarant, PropositionID, From.StatedAt, From.ID).
//
// Incompatibility: recantation > stance_change > content_change (priority order).
// A pair where prev == next on both Stance and Content is compatible; no shift.
//
// Concurrency contract: Detect is a pure function with no shared mutable
// state; it is safe to call concurrently from multiple goroutines.
package narrativeshift

import (
	"fmt"
	"sort"

	assertion "github.com/geoffmilleraz/fathom/third_party/agentic-kit/foundations/evidence/assertion"
)

// ShiftKind classifies the nature of a declarant's story change.
//
// CUE mirror: #ShiftKind in schema.cue.
type ShiftKind string

const (
	// KindRecantation is emitted when the later assertion's Stance is a
	// retraction (Stance.IsRetraction() == true): the declarant explicitly
	// withdrew or recanted the prior statement.
	KindRecantation ShiftKind = "recantation"

	// KindStanceChange is emitted when the later assertion carries a different
	// Stance from the earlier one, but is not a retraction. For example,
	// the declarant moved from affirming to denying.
	KindStanceChange ShiftKind = "stance_change"

	// KindContentChange is emitted when Stance is the same between the two
	// assertions but the verbatim Content differs. The declarant maintained
	// the same epistemic posture but changed what they actually said.
	KindContentChange ShiftKind = "content_change"

	// KindEscalation is reserved for future use. A possible rule is: a
	// declarant who previously "qualifies" a proposition and later "affirms"
	// it without qualification (hardening). This kind is defined here for
	// completeness but is never emitted by the current Detect implementation.
	// Any future emission must be accompanied by a clearly-documented rule
	// and corresponding tests.
	KindEscalation ShiftKind = "escalation"
)

// validShiftKinds is the canonical set used by Validate.
var validShiftKinds = map[ShiftKind]struct{}{
	KindRecantation:   {},
	KindStanceChange:  {},
	KindContentChange: {},
	KindEscalation:    {},
}

// Validate returns an error if k is not a recognised ShiftKind.
func (k ShiftKind) Validate() error {
	if _, ok := validShiftKinds[k]; !ok {
		return fmt.Errorf("narrativeshift: unknown ShiftKind %q; must be one of recantation|stance_change|content_change|escalation", string(k))
	}
	return nil
}

// NarrativeShift records a single detected story change for one declarant on
// one proposition. From precedes To chronologically (From.StatedAt <= To.StatedAt).
//
// CUE mirror: #NarrativeShift in schema.cue.
type NarrativeShift struct {
	// Declarant is the EntityID of the real-world person whose story changed.
	Declarant string

	// PropositionID is the proposition both assertions address.
	PropositionID string

	// From is the earlier assertion — the prior statement.
	From assertion.Assertion

	// To is the later assertion — the changed statement.
	To assertion.Assertion

	// Kind classifies what type of story change occurred.
	Kind ShiftKind

	// GapDays is the number of whole calendar days between From.StatedAt and
	// To.StatedAt. Always >= 0. Computed as floor((To.StatedAt - From.StatedAt)
	// / 24h) using UTC, so a same-day pair yields 0.
	GapDays int
}

// Detect finds all narrative shifts in the given slice of assertions. The
// input slice is not modified. The result is deterministic regardless of input
// order.
//
// A shift is emitted for each consecutive (prev, next) pair within a
// (Declarant, PropositionID) group where the pair is incompatible — meaning
// the declarant changed their story. Assertions from different declarants are
// never compared; cross-declarant disagreement is a conflict (handled by the
// evidence-graph), not a shift.
//
// Priority for kind classification:
//  1. recantation   — next.Stance.IsRetraction()
//  2. stance_change — prev.Stance != next.Stance (and next is not a retraction)
//  3. content_change — Stances equal but Content differs
//
// A pair that is identical on both Stance and Content is compatible and does
// not produce a shift.
func Detect(assertions []assertion.Assertion) []NarrativeShift {
	// Group by (Declarant, PropositionID).
	type groupKey struct {
		Declarant     string
		PropositionID string
	}
	groups := make(map[groupKey][]assertion.Assertion)
	for _, a := range assertions {
		k := groupKey{a.Declarant, a.PropositionID}
		groups[k] = append(groups[k], a)
	}

	// Collect and sort the group keys so output order is independent of map
	// iteration order.
	keys := make([]groupKey, 0, len(groups))
	for k := range groups {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool {
		if keys[i].Declarant != keys[j].Declarant {
			return keys[i].Declarant < keys[j].Declarant
		}
		return keys[i].PropositionID < keys[j].PropositionID
	})

	var result []NarrativeShift

	for _, k := range keys {
		members := groups[k]
		if len(members) < 2 {
			continue // a single assertion has nothing to shift from
		}

		// Sort within group by StatedAt; ties broken by ID for determinism.
		sort.Slice(members, func(i, j int) bool {
			if !members[i].StatedAt.Equal(members[j].StatedAt) {
				return members[i].StatedAt.Before(members[j].StatedAt)
			}
			return members[i].ID < members[j].ID
		})

		// Walk consecutive pairs.
		for i := 0; i < len(members)-1; i++ {
			prev, next := members[i], members[i+1]

			kind, compatible := classify(prev, next)
			if compatible {
				continue
			}

			gapDays := int(next.StatedAt.UTC().Sub(prev.StatedAt.UTC()).Hours() / 24)
			if gapDays < 0 {
				gapDays = 0
			}

			result = append(result, NarrativeShift{
				Declarant:     k.Declarant,
				PropositionID: k.PropositionID,
				From:          prev,
				To:            next,
				Kind:          kind,
				GapDays:       gapDays,
			})
		}
	}

	// Final sort: (Declarant, PropositionID, From.StatedAt, From.ID).
	sort.Slice(result, func(i, j int) bool {
		if result[i].Declarant != result[j].Declarant {
			return result[i].Declarant < result[j].Declarant
		}
		if result[i].PropositionID != result[j].PropositionID {
			return result[i].PropositionID < result[j].PropositionID
		}
		if !result[i].From.StatedAt.Equal(result[j].From.StatedAt) {
			return result[i].From.StatedAt.Before(result[j].From.StatedAt)
		}
		return result[i].From.ID < result[j].From.ID
	})

	return result
}

// classify determines the ShiftKind for a consecutive (prev, next) pair, and
// whether the pair is compatible (no shift). compatible == true means the pair
// should be skipped.
func classify(prev, next assertion.Assertion) (kind ShiftKind, compatible bool) {
	if next.Stance.IsRetraction() {
		return KindRecantation, false
	}
	if prev.Stance != next.Stance {
		return KindStanceChange, false
	}
	if prev.Content != next.Content {
		return KindContentChange, false
	}
	// Stance and Content are identical — no story change.
	return "", true
}
