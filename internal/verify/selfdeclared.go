package verify

// selfdeclared.go — CV-10/CV-35 (ADR-044 §1.4, I45): "a declaration added
// inside the change range (skip, disposition, retires:, surface_change:, a
// raised max:) or a plan row deleted in it never excuses a loss; it is
// reported self-declared." This generalises O56's self-skip check
// (selfskip.go) from "the one edit/commit that claimed the row" to "any
// point across a base..head range" — the shape every wave-1+ conservation
// row (integrity, ledger, absence) needs to know whether a declaration
// that WOULD excuse a finding was actually present before the change
// started, or was added by the same change it excuses.
//
// AnnotateSelfSkip stays exactly as it is: "Keep AnnotateSelfSkip as is for
// callers without a base" (ADR-044 wave-0 instructions) — a caller with no
// base ref at all (the ordinary `fathom verify`, no --since) still gets
// the narrower, git-history-free edit/commit check. AnnotateSelfDeclared is
// additive, for callers that DO have a base plan version (`--since <ref>`,
// `fathom conserve`).

import (
	"fmt"
	"sort"
	"strings"
)

// Self-declaration keys, in I45's own order of mention — sorted
// alphabetically wherever a row carries more than one (AnnotateSelfDeclared
// always emits them sorted, so callers never depend on gain order).
const (
	declaredSkip          = "skip"
	declaredDisposition   = "disposition"
	declaredRetires       = "retires"
	declaredSurfaceChange = "surface_change"
	declaredMax           = "max"
)

// AnnotateSelfDeclared returns a copy of current with each row's
// SelfDeclared set to the sorted list of declaration keys it gained
// relative to atBase — present now and absent at base, or (max only)
// present now and higher than at base. A claimed row that gained "skip" is
// ALSO marked SelfSkipped, the same field O56's narrower check
// (AnnotateSelfSkip) sets, so a single downstream fail path (OverlaySelfSkip
// OR OverlaySelfDeclared — either reads the same field) always sees it.
//
// atBase == nil means "no comparison is possible" (mirrors AnnotateSelfSkip's
// own nil convention exactly): nothing is flagged, DeletedRows is nil, every
// row's SelfDeclared is nil — still a defensive copy, so callers can treat
// the result uniformly either way.
//
// current.Baseline and atBase.Service/etc. are never consulted here — which
// two plan VERSIONS to compare is entirely the caller's job (cmd/fathom's
// --since glue, internal/conserve's Declarations); this function is pure
// and deterministic: task order and every other field are preserved,
// DeletedRows is sorted.
func AnnotateSelfDeclared(current Plan, atBase *Plan) Plan {
	out := current
	if atBase == nil {
		out.Tasks = append([]Task(nil), current.Tasks...)
		out.DeletedRows = nil
		return out
	}
	baseByID := make(map[string]Task, len(atBase.Tasks))
	for _, t := range atBase.Tasks {
		baseByID[t.ID] = t
	}
	headIDs := make(map[string]bool, len(current.Tasks))
	out.Tasks = make([]Task, len(current.Tasks))
	for i, t := range current.Tasks {
		headIDs[t.ID] = true
		bt, hadRow := baseByID[t.ID]

		// Bool-only, matching AnnotateSelfSkip's own criterion exactly
		// (verifyTask's native fail path also keys on Expect.SkipTest
		// alone): a bare skip: reason with no skip_test:true does nothing
		// in either check, so it is not "gaining a skip" on its own.
		gainedSkip := t.Expect.SkipTest && !(hadRow && bt.Expect.SkipTest)

		gainedDisposition := t.Disposition != "" && !(hadRow && bt.Disposition != "")
		gainedRetires := len(t.Retires) > 0 && !(hadRow && len(bt.Retires) > 0)
		gainedSurfaceChange := len(t.SurfaceChange) > 0 && !(hadRow && len(bt.SurfaceChange) > 0)
		raisedMax := t.Max != nil && (!hadRow || bt.Max == nil || *t.Max > *bt.Max)

		var keys []string
		if gainedSkip {
			keys = append(keys, declaredSkip)
		}
		if gainedDisposition {
			keys = append(keys, declaredDisposition)
		}
		if gainedRetires {
			keys = append(keys, declaredRetires)
		}
		if gainedSurfaceChange {
			keys = append(keys, declaredSurfaceChange)
		}
		if raisedMax {
			keys = append(keys, declaredMax)
		}
		sort.Strings(keys)
		t.SelfDeclared = keys
		if gainedSkip && t.Claimed {
			t.SelfSkipped = true
		}
		out.Tasks[i] = t
	}

	var deleted []string
	for _, bt := range atBase.Tasks {
		if !headIDs[bt.ID] {
			deleted = append(deleted, bt.ID)
		}
	}
	sort.Strings(deleted)
	out.DeletedRows = deleted
	return out
}

// OverlaySelfDeclared is the report-side complement to AnnotateSelfDeclared,
// mirroring OverlaySelfSkip's role for a wider set of declarations: a
// self-declared "skip" — t.SelfSkipped, which AnnotateSelfDeclared sets on
// EXACTLY the rows a gained skip_test flags, the same field AnnotateSelfSkip
// sets for its narrower one-edit/commit check — is forced to Fail/
// self-skipped, regardless of what the underlying verdict said, because a
// skip declared anywhere inside the range excuses nothing (I45), exactly as
// one declared in the very last commit does (O56). This overlay is
// self-sufficient: a caller that runs AnnotateSelfDeclared/OverlaySelfDeclared
// alone (never AnnotateSelfSkip/OverlaySelfSkip — the base-aware path is a
// full replacement, not an addition, for a caller that HAS a base) still
// gets the fail correctly; cmd/fathom's --since glue happens to call both,
// harmlessly idempotent since the second call finds the row already Fail.
//
// Every OTHER self-declared row (disposition/retires/surface_change/max
// alone, no skip) is left at its computed Status: I45 says these are
// "reported," never "failed" — a self-declared retirement or ratchet is
// significant information for a HUMAN reviewer, but the row's own evidence
// contract still stands or falls on its own criteria. Its self-declared
// keys are recorded as an informational `self_declared` criterion on every
// touched row (including a failed one, so its failure detail's context is
// visible without a second lookup), so a renderer (fathom verify --since's
// "self-declared: <keys>" line, CV-14) never has to re-read the plan.
//
// A row not present in plan, or with an empty SelfDeclared, is left
// completely untouched.
func OverlaySelfDeclared(rep *Report, plan Plan) {
	byID := make(map[string]Task, len(plan.Tasks))
	for _, t := range plan.Tasks {
		byID[t.ID] = t
	}
	touched := false
	for i := range rep.Tasks {
		t, ok := byID[rep.Tasks[i].TaskID]
		if !ok || len(t.SelfDeclared) == 0 {
			continue
		}
		detail := fmt.Sprintf("self-declared: %s", strings.Join(t.SelfDeclared, ", "))
		if t.SelfSkipped {
			touched = true
			failDetail := fmt.Sprintf(
				"skip_test/skip was declared inside the change range — a declaration added inside the range excuses nothing (I45); %s; skip: %q",
				detail, t.Expect.SkipReason,
			)
			rep.Tasks[i].Criteria = append(rep.Tasks[i].Criteria, Criterion{Name: "skip_test", Status: Fail, Detail: failDetail})
			rep.Tasks[i].Status = Fail
			rep.Tasks[i].Label = "self-skipped"
			continue
		}
		rep.Tasks[i].Criteria = append(rep.Tasks[i].Criteria, Criterion{Name: "self_declared", Status: NA, Detail: detail})
	}
	if !touched {
		return
	}
	sum := Summary{Total: len(rep.Tasks)}
	for _, v := range rep.Tasks {
		switch v.Status {
		case Pass:
			sum.Pass++
		case Fail:
			sum.Fail++
		default:
			sum.Unknown++
		}
	}
	rep.Summary = sum
}
