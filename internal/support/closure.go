package support

// closure.go — CheckClosure (SM-7, design §3.1): the closure test between a
// kind's registry and its target list, generic over the two target shapes
// the repo actually has (a #Target row for the six shared kinds, or a
// language-queue row for language/tool, LB-F22) via the shared ClosureRef.

import (
	"fmt"
	"sort"
)

// ClosureRef is the minimal (kind, id, plan_row) triple CheckClosure needs
// from a target-shaped row, whichever of the two target files it came from.
type ClosureRef struct {
	Kind    Kind
	ID      string
	PlanRow string
}

// ClosureViolation is one closure failure design §3.1 names.
type ClosureViolation struct {
	Kind   Kind
	ID     string
	Reason string
}

func (v ClosureViolation) String() string {
	return fmt.Sprintf("%s/%s: %s", v.Kind, v.ID, v.Reason)
}

// closureKey joins (kind, id) into the join key design §2 names.
func closureKey(k Kind, id string) string { return string(k) + "/" + id }

// CheckClosure is TestSupportRegistriesMatchTargets's engine (design §3.1):
//
//   - a registry entry with no target row at all fails (support nobody
//     planned or measured);
//   - a target row with no registry entry fails ONLY when its owning plan
//     row is claimed (claimedRows[plan_row] == true) — a target row whose
//     plan row is still open is `declared`, not a failure, so planning
//     ahead stays possible;
//   - a reading naming a (kind, id) in neither the registry nor the
//     targets fails.
//
// claimedRows maps a plan row id to whether it is claimed ([x]); a row
// absent from the map is treated as not claimed (open). The result is
// sorted by (kind, id, reason); nil means a clean closure.
func CheckClosure(registry []RegistryEntry, targets []ClosureRef, readings []ClosureRef, claimedRows map[string]bool) []ClosureViolation {
	regKeys := make(map[string]bool, len(registry))
	for _, r := range registry {
		regKeys[closureKey(r.Kind, r.ID)] = true
	}
	targetKeys := make(map[string]bool, len(targets))
	for _, t := range targets {
		targetKeys[closureKey(t.Kind, t.ID)] = true
	}

	var violations []ClosureViolation

	for _, r := range registry {
		if !targetKeys[closureKey(r.Kind, r.ID)] {
			violations = append(violations, ClosureViolation{r.Kind, r.ID, "registry entry has no target row (support nobody planned or measured)"})
		}
	}

	for _, t := range targets {
		if regKeys[closureKey(t.Kind, t.ID)] {
			continue
		}
		if claimedRows[t.PlanRow] {
			violations = append(violations, ClosureViolation{t.Kind, t.ID, fmt.Sprintf("target row names no registry entry, and its plan row %s is claimed", t.PlanRow)})
		}
		// Else: declared (planned, not yet built) — not a failure.
	}

	for _, rd := range readings {
		k := closureKey(rd.Kind, rd.ID)
		if !regKeys[k] && !targetKeys[k] {
			violations = append(violations, ClosureViolation{rd.Kind, rd.ID, "reading names an entry in neither the registry nor the targets"})
		}
	}

	sort.Slice(violations, func(i, j int) bool {
		a, b := violations[i], violations[j]
		if a.Kind != b.Kind {
			return a.Kind < b.Kind
		}
		if a.ID != b.ID {
			return a.ID < b.ID
		}
		return a.Reason < b.Reason
	})
	return violations
}

// targetRefs converts #Target rows (the shared six-kind target list) into
// ClosureRefs.
func targetRefs(targets []Target) []ClosureRef {
	refs := make([]ClosureRef, 0, len(targets))
	for _, t := range targets {
		refs = append(refs, ClosureRef{Kind: t.Kind, ID: t.ID, PlanRow: t.PlanRow})
	}
	return refs
}
