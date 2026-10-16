package metrics

// interventions.go — Q-12 (docs/plans/expert-suite-q.md, docs/design/
// observability-surfaces.md §1): the design's own Interventions table,
// mechanism-labeled, no inferred agent behavior ever. Five rows, in the
// design's own order:
//
//  1. unproven-done refusals   — verify_plan calls' "failing" verdict count
//  2. review findings superseded — claim_events with Verdict "superseded"
//  3. non-conforming artifacts refused — ALWAYS "not measurable" (no
//     persistent log of module-doctor promotion refusals or
//     store.MountReadOnly conformance failures exists today; the design's
//     own §1 table already omits a "live number" for this row)
//  4. obligations served      — the channels aggregate's ObligationsServed
//  5. guarded pre-warns       — the channels aggregate's GuardPreServed
//
// Rows 4/5 reuse AggregateChannels (channels.go) internally rather than
// reimplementing the beat accounting a second time.

import (
	"time"

	"github.com/geoffmilleraz/fathom/internal/core"

	heartbeat "github.com/geoffmilleraz/fathom/third_party/agentic-kit/foundations/telemetry/heartbeat"
)

// InterventionRow is one mechanism-labeled row of the design's own table.
// Count is nil (with an explanatory Note) exactly when the mechanism has
// no durable evidence source today — never a fabricated zero.
type InterventionRow struct {
	Name      string `json:"name"`
	Mechanism string `json:"mechanism"`
	Count     *int   `json:"count"`
	Note      string `json:"note,omitempty"`
}

// InterventionsReport is AggregateInterventions' output.
type InterventionsReport struct {
	Since string            `json:"since"`
	Until string            `json:"until"`
	Rows  []InterventionRow `json:"rows"`
}

// CallVerdictOutcome is the minimal call-log shape AggregateInterventions
// needs from a verify_plan/review row's verdict_outcome field — decoupled
// from cmd/fathom's adoptionCallEntry the same way CallEntry (traffic.go)
// is.
type CallVerdictOutcome struct {
	Tool    string
	Outcome map[string]int
}

// AggregateInterventions builds the design §1 table from already-parsed
// inputs: calls (verify_plan verdict outcomes), beats (the hooks beat
// log, for rows 4/5 via AggregateChannels), and claimEvents (the
// claim_events log, for row 2). claimEventsQueried distinguishes "no
// --store given, this mechanism was never attempted" from "queried a real
// store that genuinely has zero superseded events this window" — a nil
// vs. empty claimEvents slice cannot make this distinction on its own
// (Store.ClaimEventsSince also returns a nil slice for a genuinely empty
// result), so the caller states which case it is explicitly.
func AggregateInterventions(callVerdicts []CallVerdictOutcome, beats []heartbeat.Beat, claimEvents []core.ClaimEvent, claimEventsQueried bool, since, until time.Time) InterventionsReport {
	r := InterventionsReport{
		Since: since.UTC().Format(time.RFC3339),
		Until: until.UTC().Format(time.RFC3339),
	}

	// Row 1: unproven "done" claims refused.
	unprovenDone := 0
	sawVerifyPlan := false
	for _, cv := range callVerdicts {
		if cv.Tool != "verify_plan" {
			continue
		}
		sawVerifyPlan = true
		unprovenDone += cv.Outcome["failing"]
	}
	row1 := InterventionRow{
		Name:      "unproven_done_claims_refused",
		Mechanism: "verify_plan returned a failing verdict on a row an author/agent had checked off",
	}
	if sawVerifyPlan {
		n := unprovenDone
		row1.Count = &n
	} else {
		row1.Note = "not measurable: no verify_plan calls with a verdict_outcome in window"
	}
	r.Rows = append(r.Rows, row1)

	// Row 2: review findings raised then superseded.
	superseded := 0
	for _, ev := range claimEvents {
		if ev.Verdict == "superseded" && !ev.At.Before(since) && !ev.At.After(until) {
			superseded++
		}
	}
	row2 := InterventionRow{
		Name:      "review_findings_superseded",
		Mechanism: "a review-minted finding persisted, then reconciled as fixed on a later push — recorded as a \"superseded\" claim_event (fathom review's fixed-finding reconcile)",
	}
	if !claimEventsQueried {
		row2.Note = "not measurable: no --store given — claim_events was never queried this run"
	} else {
		n := superseded
		row2.Count = &n
	}
	r.Rows = append(r.Rows, row2)

	// Row 3: non-conforming artifacts refused — ALWAYS not measurable
	// today (no code path persists this event anywhere in fathom).
	r.Rows = append(r.Rows, InterventionRow{
		Name:      "non_conforming_artifacts_refused",
		Mechanism: "store.MountReadOnly conformance refusal, module-doctor promotion denial",
		Note:      "not measurable: no persistent log of module-doctor promotion refusals or store.MountReadOnly conformance failures exists today — both are computed fresh per invocation and never recorded to a beat, log, or store row",
	})

	// Rows 4/5: reuse the channels aggregate — no second implementation.
	channels := AggregateChannels(beats, since, until)
	row4 := InterventionRow{
		Name:      "governed_edits_with_rules_delivered",
		Mechanism: "obligations served on an edit (delivery is the fact; obedience is not claimed)",
	}
	if channels.InjectionSeen == 0 {
		row4.Note = "not measurable: no edit-hook beats observed in window"
	} else {
		n := channels.ObligationsServed
		row4.Count = &n
	}
	r.Rows = append(r.Rows, row4)

	row5 := InterventionRow{
		Name:      "guarded_critical_edits_pre_warned",
		Mechanism: "pre-hook served before the tool ran on a guarded scope",
	}
	if channels.GuardPreSeen == 0 {
		row5.Note = "not measurable: no PreToolUse guard-hook beats observed in window"
	} else {
		n := channels.GuardPreServed
		row5.Count = &n
	}
	r.Rows = append(r.Rows, row5)

	return r
}
