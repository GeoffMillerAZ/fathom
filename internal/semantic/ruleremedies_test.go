// ruleremedies_test.go — GL-19 (docs/plans/governance-learning.md, design
// doc §5): ruleRemedies filters RuleEffectiveness's four classes down to
// the two design doc §5 proposes an ACTIONABLE remedy for (ignored ->
// move to a hook or rewrite; never_triggered -> prune or re-scope) and
// attaches human-readable evidence to each — effective's remedy is "keep"
// and contested's is "human review", neither a change to propose, so
// neither rule reaches this filtered view.
package semantic

import (
	"testing"
	"time"
)

func TestRuleRemediesAreProposals(t *testing.T) {
	now := time.Date(2026, 9, 28, 0, 0, 0, 0, time.UTC)
	recent := now.Add(-24 * time.Hour)

	servedEvents := func(ruleID string, n int, at time.Time) []RuleOutcomeEvent {
		out := make([]RuleOutcomeEvent, 0, n)
		for i := 0; i < n; i++ {
			out = append(out, RuleOutcomeEvent{RuleID: ruleID, Event: RuleEventServed, At: at})
		}
		return out
	}
	kindEvents := func(ruleID, kind string, n int, at time.Time) []RuleOutcomeEvent {
		out := make([]RuleOutcomeEvent, 0, n)
		for i := 0; i < n; i++ {
			out = append(out, RuleOutcomeEvent{RuleID: ruleID, Event: kind, At: at})
		}
		return out
	}

	var events []RuleOutcomeEvent
	// ignored: 20 served, 10 violated (50%), never blocked.
	events = append(events, servedEvents("ignored-rule", 20, recent)...)
	events = append(events, kindEvents("ignored-rule", RuleEventStillServed, 10, recent)...)
	// effective: keeps, never a remedy proposal.
	events = append(events, servedEvents("effective-rule", 20, recent)...)
	events = append(events, kindEvents("effective-rule", RuleEventTopChanged, 20, recent)...)
	// contested: human review, never a remedy proposal.
	events = append(events, servedEvents("contested-rule", 5, recent)...)
	events = append(events, kindEvents("contested-rule", RuleEventRefuted, 3, recent)...)
	// never-triggered-rule: zero events at all (known only via ruleIDs).

	ruleIDs := []string{"ignored-rule", "effective-rule", "contested-rule", "never-triggered-rule"}
	results := RuleEffectiveness(ruleIDs, events, now)

	got := ruleRemedies(results)
	if len(got) != 2 {
		t.Fatalf("ruleRemedies returned %d proposals, want 2 (ignored + never_triggered only): %+v", len(got), got)
	}

	// Sorted by RuleID: "ignored-rule" < "never-triggered-rule".
	if got[0].RuleID != "ignored-rule" || got[0].Class != RuleClassIgnored {
		t.Errorf("got[0] = %+v, want ignored-rule/ignored", got[0])
	}
	if got[0].Remedy != "move to a hook (deny) or rewrite with the violating example" {
		t.Errorf("ignored-rule remedy = %q", got[0].Remedy)
	}
	if got[0].Evidence == "" {
		t.Error("ignored-rule proposal carries no evidence")
	}

	if got[1].RuleID != "never-triggered-rule" || got[1].Class != RuleClassNeverTriggered {
		t.Errorf("got[1] = %+v, want never-triggered-rule/never_triggered", got[1])
	}
	if got[1].Remedy != "prune or re-scope" {
		t.Errorf("never-triggered-rule remedy = %q", got[1].Remedy)
	}
	if got[1].Evidence == "" {
		t.Error("never-triggered-rule proposal carries no evidence")
	}

	// effective-rule and contested-rule must never appear: their remedies
	// ("keep", "human review") are not proposals to change anything.
	for _, p := range got {
		if p.RuleID == "effective-rule" || p.RuleID == "contested-rule" {
			t.Errorf("ruleRemedies proposed a change for a %s rule: %+v", p.RuleID, p)
		}
	}
}

func TestRuleRemediesEmptyWhenNoneClassify(t *testing.T) {
	now := time.Date(2026, 9, 28, 0, 0, 0, 0, time.UTC)
	results := RuleEffectiveness([]string{"unclassified-rule"}, []RuleOutcomeEvent{
		{RuleID: "unclassified-rule", Event: RuleEventServed, At: now.Add(-time.Hour)},
	}, now)
	if got := ruleRemedies(results); len(got) != 0 {
		t.Errorf("ruleRemedies = %+v, want empty (no class applies)", got)
	}
}
