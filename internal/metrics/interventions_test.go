package metrics

import (
	"testing"
	"time"

	"github.com/geoffmilleraz/fathom/internal/core"

	heartbeat "github.com/geoffmilleraz/fathom/third_party/agentic-kit/foundations/telemetry/heartbeat"
)

func interventionsWindow() (time.Time, time.Time) {
	return time.Unix(0, 0).UTC(), time.Unix(1<<32, 0).UTC()
}

func TestAggregateInterventions_AllFiveRowsPresent(t *testing.T) {
	since, until := interventionsWindow()
	report := AggregateInterventions(nil, nil, nil, false, since, until)
	if len(report.Rows) != 5 {
		t.Fatalf("rows = %+v, want 5", report.Rows)
	}
	wantNames := []string{
		"unproven_done_claims_refused",
		"review_findings_superseded",
		"non_conforming_artifacts_refused",
		"governed_edits_with_rules_delivered",
		"guarded_critical_edits_pre_warned",
	}
	for i, want := range wantNames {
		if report.Rows[i].Name != want {
			t.Errorf("row %d name = %q, want %q", i, report.Rows[i].Name, want)
		}
		if report.Rows[i].Mechanism == "" {
			t.Errorf("row %d (%s) has no Mechanism", i, report.Rows[i].Name)
		}
	}
}

func TestAggregateInterventions_UnprovenDoneRefusals_SumsFailingVerdicts(t *testing.T) {
	since, until := interventionsWindow()
	calls := []CallVerdictOutcome{
		{Tool: "verify_plan", Outcome: map[string]int{"claimed": 10, "verified": 7, "failing": 3}},
		{Tool: "verify_plan", Outcome: map[string]int{"claimed": 5, "verified": 5, "failing": 0}},
		{Tool: "review", Outcome: map[string]int{"spec_coverage": 2}}, // not verify_plan — excluded
	}
	report := AggregateInterventions(calls, nil, nil, false, since, until)
	row := report.Rows[0]
	if row.Count == nil || *row.Count != 3 {
		t.Fatalf("row1 Count = %v, want 3", row.Count)
	}
}

func TestAggregateInterventions_ReviewSupersededCount_FromClaimEvents(t *testing.T) {
	since, until := interventionsWindow()
	events := []core.ClaimEvent{
		{ClaimID: "c1", Verdict: "superseded", At: time.Unix(100, 0).UTC()},
		{ClaimID: "c2", Verdict: "refuted", At: time.Unix(100, 0).UTC()},
		{ClaimID: "c3", Verdict: "superseded", At: time.Unix(200, 0).UTC()},
	}
	report := AggregateInterventions(nil, nil, events, true, since, until)
	row := report.Rows[1]
	if row.Count == nil || *row.Count != 2 {
		t.Fatalf("row2 Count = %v, want 2", row.Count)
	}
	if row.Note != "" {
		t.Errorf("row2 Note = %q, want empty when queried", row.Note)
	}
}

func TestAggregateInterventions_ReviewSupersededCount_NotQueried_HonestNote(t *testing.T) {
	since, until := interventionsWindow()
	report := AggregateInterventions(nil, nil, nil, false, since, until)
	row := report.Rows[1]
	if row.Count != nil {
		t.Fatalf("row2 Count = %v, want nil when not queried", row.Count)
	}
	if row.Note == "" {
		t.Error("row2 Note is empty, want an honest not-queried note")
	}
}

// TestAggregateInterventions_NonConformingRow_AlwaysNotMeasurable is the
// load-bearing "no inferred agent behavior" proof — this row must NEVER
// carry a count, regardless of input.
func TestAggregateInterventions_NonConformingRow_AlwaysNotMeasurable(t *testing.T) {
	since, until := interventionsWindow()
	calls := []CallVerdictOutcome{{Tool: "verify_plan", Outcome: map[string]int{"failing": 99}}}
	beats := []heartbeat.Beat{{Channel: "edit-hook", Kind: "seen", At: time.Unix(1, 0).UTC()}}
	events := []core.ClaimEvent{{ClaimID: "c1", Verdict: "superseded", At: time.Unix(1, 0).UTC()}}
	report := AggregateInterventions(calls, beats, events, true, since, until)
	row := report.Rows[2]
	if row.Name != "non_conforming_artifacts_refused" {
		t.Fatalf("row[2] = %+v, want non_conforming_artifacts_refused", row)
	}
	if row.Count != nil {
		t.Fatalf("Count = %v, want nil — this row has no evidence source today", row.Count)
	}
	if row.Note == "" {
		t.Error("Note is empty, want an honest not-measurable note")
	}
}

func TestAggregateInterventions_ObligationsAndGuardRows_MatchChannelsAggregate(t *testing.T) {
	since, until := interventionsWindow()
	at := time.Unix(100, 0).UTC()
	beats := []heartbeat.Beat{
		{Channel: "edit-hook", Kind: "seen", At: at},
		{Channel: "edit-hook", Kind: "answered", At: at, Meta: map[string]string{"emitted": "true"}},
		{Channel: "edit-hook", Kind: "seen", At: at, Meta: map[string]string{"phase": "pre"}},
		{Channel: "edit-hook", Kind: "answered", At: at, Meta: map[string]string{"phase": "pre", "emitted": "true"}},
	}
	report := AggregateInterventions(nil, beats, nil, false, since, until)
	channels := AggregateChannels(beats, since, until)

	row4 := report.Rows[3]
	if row4.Count == nil || *row4.Count != channels.ObligationsServed {
		t.Fatalf("row4 Count = %v, want %d (matching AggregateChannels)", row4.Count, channels.ObligationsServed)
	}
	row5 := report.Rows[4]
	if row5.Count == nil || *row5.Count != channels.GuardPreServed {
		t.Fatalf("row5 Count = %v, want %d (matching AggregateChannels)", row5.Count, channels.GuardPreServed)
	}
}

func TestAggregateInterventions_EmptyInput_AllRowsPresentWithNotes(t *testing.T) {
	since, until := interventionsWindow()
	report := AggregateInterventions(nil, nil, nil, false, since, until)
	for _, row := range report.Rows {
		if row.Count == nil && row.Note == "" {
			t.Errorf("row %q has neither Count nor Note — must have one", row.Name)
		}
	}
}
