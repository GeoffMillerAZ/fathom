package console

import (
	"errors"
	"testing"
	"time"

	"github.com/geoffmilleraz/fathom/internal/semantic"
)

// ruleOutcomesNow is the fixed clock every case reads at.
var ruleOutcomesNow = time.Date(2026, 9, 28, 12, 0, 0, 0, time.UTC)

func outcomeEvent(id, event string, daysAgo int) semantic.RuleOutcomeEvent {
	return semantic.RuleOutcomeEvent{RuleID: id, Event: event, At: ruleOutcomesNow.AddDate(0, 0, -daysAgo)}
}

func TestRuleOutcomesViewCountsServedCompliedViolatedPerClaim(t *testing.T) {
	events := []semantic.RuleOutcomeEvent{
		outcomeEvent("claim-a", semantic.RuleEventServed, 1),
		outcomeEvent("claim-a", semantic.RuleEventServed, 2),
		outcomeEvent("claim-a", semantic.RuleEventServed, 3),
		outcomeEvent("claim-a", semantic.RuleEventComplied, 3),
		outcomeEvent("claim-a", semantic.RuleEventViolated, 2),
		// Older than the 30-day window: not counted.
		outcomeEvent("claim-a", semantic.RuleEventServed, 45),
		outcomeEvent("claim-b", semantic.RuleEventServed, 4),
		outcomeEvent("claim-b", semantic.RuleEventRefuted, 4),
		// A claim nobody asked about never appears.
		outcomeEvent("claim-z", semantic.RuleEventServed, 1),
	}
	read := RuleOutcomeRead{Present: true, Rows: len(events), Since: ruleOutcomesNow.AddDate(0, 0, -45)}

	v := RuleOutcomesView([]string{"claim-a", "claim-b", "claim-c", ""}, events, read, ruleOutcomesNow)
	if !v.Known() {
		t.Fatalf("state = %s (%s), want a measured value", v.State, v.Reason)
	}
	if v.V.WindowDays != 30 {
		t.Errorf("window = %d days, want 30", v.V.WindowDays)
	}
	if len(v.V.Rules) != 3 {
		t.Fatalf("rules = %+v, want the three named claims in the order asked (the blank id is skipped)", v.V.Rules)
	}
	a, b, c := v.V.Rules[0], v.V.Rules[1], v.V.Rules[2]
	if a.ClaimID != "claim-a" || a.Served != 3 || a.Complied != 1 || a.Violated != 1 {
		t.Errorf("claim-a = %+v, want served 3, complied 1, violated 1 in the window", a)
	}
	if b.ClaimID != "claim-b" || b.Served != 1 || b.Refuted != 1 {
		t.Errorf("claim-b = %+v, want served 1, refuted 1", b)
	}
	// A measured zero: the log is there and this claim is in it nowhere.
	if c.ClaimID != "claim-c" || c.Served != 0 || c.Complied != 0 || c.Violated != 0 {
		t.Errorf("claim-c = %+v, want a measured zero", c)
	}
}

func TestRuleOutcomesViewNamesTheClassAfterTheWindowIsCovered(t *testing.T) {
	var events []semantic.RuleOutcomeEvent
	for i := 0; i < 24; i++ {
		events = append(events, outcomeEvent("ignored", semantic.RuleEventServed, 1+i%20))
		if i < 14 {
			events = append(events, outcomeEvent("ignored", semantic.RuleEventViolated, 1+i%20))
		}
	}
	read := RuleOutcomeRead{Present: true, Rows: len(events), Since: ruleOutcomesNow.AddDate(0, 0, -100)}
	v := RuleOutcomesView([]string{"ignored", "quiet"}, events, read, ruleOutcomesNow)
	if got := v.V.Rules[0].Class; got != string(semantic.RuleClassIgnored) {
		t.Errorf("ignored rule class = %q, want %q (violated 14 of 24 served)", got, semantic.RuleClassIgnored)
	}
	if got := v.V.Rules[1].Class; got != string(semantic.RuleClassNeverTriggered) {
		t.Errorf("never-served rule class = %q, want %q once the log spans the 90-day window", got, semantic.RuleClassNeverTriggered)
	}
}

func TestRuleOutcomesViewNeverCallsARuleUntriggeredBeforeTheLogSpansTheWindow(t *testing.T) {
	// A log that began nine days ago cannot say a rule has not been served
	// in 90: never_triggered would be a claim the instrument cannot make.
	events := []semantic.RuleOutcomeEvent{outcomeEvent("other", semantic.RuleEventServed, 2)}
	read := RuleOutcomeRead{Present: true, Rows: 1, Since: ruleOutcomesNow.AddDate(0, 0, -9)}
	v := RuleOutcomesView([]string{"young"}, events, read, ruleOutcomesNow)
	if got := v.V.Rules[0].Class; got != "" {
		t.Errorf("class = %q, want none: the log is 9 days old, never_triggered needs 90", got)
	}
}

func TestRuleOutcomesViewTruthStates(t *testing.T) {
	ids := []string{"claim-a"}

	absent := RuleOutcomesView(ids, nil, RuleOutcomeRead{Present: false}, ruleOutcomesNow)
	if absent.State != StateAbsent || absent.Reason == "" || absent.Known() {
		t.Errorf("no log: %+v, want absent with a reason — never a zero", absent)
	}

	errored := RuleOutcomesView(ids, nil, RuleOutcomeRead{Err: errors.New("permission denied")}, ruleOutcomesNow)
	if errored.State != StateError || errored.Reason == "" {
		t.Errorf("unreadable log: %+v, want error naming why", errored)
	}

	partial := RuleOutcomesView(ids, []semantic.RuleOutcomeEvent{outcomeEvent("claim-a", semantic.RuleEventServed, 1)},
		RuleOutcomeRead{Present: true, Rows: 2, Skipped: 1, Since: ruleOutcomesNow.AddDate(0, 0, -1)}, ruleOutcomesNow)
	if partial.State != StatePartial || partial.Reason == "" || partial.V.Rules[0].Served != 1 {
		t.Errorf("a log with a malformed row: %+v, want a partial lower bound that still counts the good rows", partial)
	}

	none := RuleOutcomesView(nil, nil, RuleOutcomeRead{Present: true}, ruleOutcomesNow)
	if none.State != StateAbsent {
		t.Errorf("no served rule to count: state %s, want absent", none.State)
	}
}
