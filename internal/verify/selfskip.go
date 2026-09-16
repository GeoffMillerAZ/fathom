package verify

// selfskip.go — O56 defect 2: an agent may not grade its own row. A claimed
// row's skip_test escape must be reasoned (verifyTask's skip-without-reason
// check, unconditional and pure) AND it must not have been introduced by the
// same edit/commit that claims the row done. THAT half needs one prior
// version of the plan file to compare against — git history, which this
// package otherwise never touches (VerifyPlan is pure, docs/CLAUDE.md). The
// split mirrors the lexical oracle's own FileEvidence/DirFiles/OverlayLocal
// pattern exactly: the comparison is pure once handed bytes (AnnotateSelfSkip),
// and the ONE git read that supplies those bytes lives entirely in the
// caller (cmd/fathom's priorSelfSkipPlan, internal/mcp's verify_plan
// plan_path support) — never a git log -L walk or git blame, per O56's own
// instruction to keep this deterministic and bounded.

import "fmt"

// AnnotateSelfSkip returns a copy of current with SelfSkipped set on every
// row that is currently claimed ([x]), carries skip_test: true, AND whose
// skip was not already present in prior (matched by task ID).
//
// prior == nil means "no comparison is possible or applicable" (no git, or
// the caller determined the check does not apply here) — nothing is
// flagged; a skip is presumed to stand rather than be punished for an
// instrument that cannot read history from where it runs (fail-open, same
// discipline as an Unknown verdict never refusing). A non-nil prior with no
// row for a given ID (a genuinely new row, or an empty Plan{} when the
// prior git ref does not have the file at all) means that row's skip did
// NOT predate the claim — it is flagged, because it clearly was "added"
// relative to what came before.
//
// Deterministic; no I/O; task order and every other field are preserved.
func AnnotateSelfSkip(current Plan, prior *Plan) Plan {
	out := current
	if prior == nil {
		// Nothing to compare against — every row's flag is left at its
		// parsed zero value (false); still return a defensive copy so
		// callers can treat the result uniformly.
		out.Tasks = append([]Task(nil), current.Tasks...)
		return out
	}
	priorByID := make(map[string]Task, len(prior.Tasks))
	for _, t := range prior.Tasks {
		priorByID[t.ID] = t
	}
	out.Tasks = make([]Task, len(current.Tasks))
	for i, t := range current.Tasks {
		t.SelfSkipped = false
		if t.Claimed && t.Expect.SkipTest {
			pt, hadRow := priorByID[t.ID]
			hadSkip := hadRow && pt.Expect.SkipTest
			if !hadSkip {
				t.SelfSkipped = true
			}
		}
		out.Tasks[i] = t
	}
	return out
}

// OverlaySelfSkip is the HTTP-mode complement to AnnotateSelfSkip, exactly
// mirroring OverlayLocal's role for the lexical oracle: a daemon that
// computed rep parsed the plan markdown it was handed WITHOUT git access, so
// it cannot know a row was self-skipped — its verdict for that row may read
// Pass ("waived: <reason>"). This overlay applies the locally (or
// plan_path-resolved) computed AnnotateSelfSkip verdict from plan, forcing a
// flagged row to Fail/self-skipped regardless of what the daemon said. A row
// not flagged SelfSkipped in plan is left untouched byte for byte, and the
// summary is recomputed exactly as OverlayLocal does.
func OverlaySelfSkip(rep *Report, plan Plan) {
	byID := make(map[string]Task, len(plan.Tasks))
	for _, t := range plan.Tasks {
		byID[t.ID] = t
	}
	touched := false
	for i := range rep.Tasks {
		t, ok := byID[rep.Tasks[i].TaskID]
		if !ok || !t.SelfSkipped || !t.Expect.SkipTest {
			continue
		}
		touched = true
		detail := fmt.Sprintf(
			"skip_test/skip was added in the same edit or commit that claims this row done — an agent may not grade its own row (self-skipped); skip: %q",
			t.Expect.SkipReason,
		)
		rep.Tasks[i].Criteria = append(rep.Tasks[i].Criteria, Criterion{Name: "skip_test", Status: Fail, Detail: detail})
		rep.Tasks[i].Status = Fail
		rep.Tasks[i].Label = "self-skipped"
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
