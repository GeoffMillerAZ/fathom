package contextbench

import (
	"strings"
	"testing"
)

func TestRenderVerdict_PlainLanguageBeforeAnyRatio(t *testing.T) {
	rows := []LedgerRow{
		{
			RunID: "20260913T002832Z", Treatment: "fathom", Control: "grep",
			TreatmentSuccesses: 31, ControlSuccesses: 28, TreatmentTrials: 33, ControlTrials: 33,
			AggregateTokenRatio: 1.443545060837286, AggregateCostRatio: 1.4646445896968614,
			Evaluation: Evaluation{CriterionID: "H2-1", Verdict: "fail", Reason: "aggregate token ratio 1.444× exceeds the pre-registered cap of 1.000×"},
		},
	}
	out := RenderVerdict(rows)
	if !strings.HasPrefix(out, VerdictHeading) {
		t.Fatalf("must open with the heading:\n%s", out)
	}
	if !strings.Contains(out, "success 31/33 vs 28/33") {
		t.Errorf("must report treatment vs control successes:\n%s", out)
	}
	if !strings.Contains(out, "tokens 1.444×") {
		t.Errorf("must report the token ratio:\n%s", out)
	}
	if !strings.Contains(out, "cost 1.465×") {
		t.Errorf("must report the cost ratio:\n%s", out)
	}
	if !strings.Contains(out, "fathom's arm beat grep on success (31 v 28) at 1.444× the tokens") {
		t.Errorf("must render the plain-language sentence:\n%s", out)
	}
}

func TestRenderVerdict_LostToAndMatched(t *testing.T) {
	lost := RenderVerdict([]LedgerRow{{RunID: "r1", Control: "grep", TreatmentSuccesses: 1, TreatmentTrials: 3, ControlSuccesses: 3, ControlTrials: 3}})
	if !strings.Contains(lost, "fathom's arm lost to grep") {
		t.Errorf("a lower success rate must render as lost to:\n%s", lost)
	}
	matched := RenderVerdict([]LedgerRow{{RunID: "r2", Control: "grep", TreatmentSuccesses: 2, TreatmentTrials: 3, ControlSuccesses: 2, ControlTrials: 3}})
	if !strings.Contains(matched, "fathom's arm matched grep") {
		t.Errorf("equal success rates must render as matched:\n%s", matched)
	}
}

// TestRenderVerdict_IndeterminateDoesNotClaimBeat (review finding 4): a
// row the criterion calls indeterminate — an n=1 run below the
// replication floor, in this case — must never render "beat"/"lost to"
// from the raw success rates; it must say the run did not measure enough
// and still show the criterion's own verdict and reason.
func TestRenderVerdict_IndeterminateDoesNotClaimBeat(t *testing.T) {
	rows := []LedgerRow{{
		RunID: "20260724T170152Z", Treatment: "fathom", Control: "grep",
		TreatmentSuccesses: 13, ControlSuccesses: 11, TreatmentTrials: 13, ControlTrials: 13,
		AggregateTokenRatio: 0.570,
		Evaluation:          Evaluation{CriterionID: "H2-1", Verdict: VerdictIndeterminate, Reason: "thinnest cell ran 1 trial(s), below the pre-registered floor of 3"},
	}}
	out := RenderVerdict(rows)
	if strings.Contains(out, "fathom's arm beat") || strings.Contains(out, "fathom's arm lost to") {
		t.Errorf("an indeterminate row must never claim beat/lost to from raw rates:\n%s", out)
	}
	if !strings.Contains(out, "did not run enough trials to decide against grep") {
		t.Errorf("must say the run did not measure enough:\n%s", out)
	}
	if !strings.Contains(out, "registered criterion H2-1: indeterminate (thinnest cell ran 1 trial(s), below the pre-registered floor of 3)") {
		t.Errorf("must still render the criterion's own verdict and reason:\n%s", out)
	}
}

func TestRenderVerdict_NoRows(t *testing.T) {
	out := RenderVerdict(nil)
	if !strings.Contains(out, VerdictHeading) || !strings.Contains(out, "No runs recorded") {
		t.Errorf("an empty ledger must render a sentence, not a blank block:\n%q", out)
	}
}
