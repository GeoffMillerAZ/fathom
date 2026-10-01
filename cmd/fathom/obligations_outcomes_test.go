package main

// obligations_outcomes_test.go — SR-61's read side (surface reckoning
// F-27/U4, design §6): `fathom obligations` and the console's serving
// explainer show, beside each served rule, what the edit hook recorded of it
// — served, top_changed, still_served over the last 30 days — read from the signal
// log at request time. The agent-facing MCP payload is untouched (its claim
// id and counts ride nowhere an agent pays for; obligations_claimid_test.go).

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/geoffmilleraz/fathom/internal/console"
)

type obligationsOutcomesJSON struct {
	Rules []struct {
		Rule    string `json:"rule"`
		ClaimID string `json:"claim_id"`
	} `json:"rules"`
	Outcomes struct {
		State  console.State `json:"state"`
		Reason string        `json:"reason"`
		V      struct {
			WindowDays int `json:"window_days"`
			Rules      []struct {
				ClaimID     string `json:"claim_id"`
				Served      int    `json:"served"`
				TopChanged  int    `json:"top_changed"`
				StillServed int    `json:"still_served"`
			} `json:"rules"`
		} `json:"v"`
	} `json:"outcomes"`
}

func TestObligationsShowsTheHooksRuleOutcomes(t *testing.T) {
	f := newConsoleFixture(t)
	logPath := t.TempDir() + "/rule-outcomes.jsonl"
	t.Setenv("FATHOM_RULE_OUTCOME_LOG", logPath)
	orig := ruleOutcomeNow
	t.Cleanup(func() { ruleOutcomeNow = orig })
	ruleOutcomeNow = func() time.Time { return f.now.Add(-time.Hour) }

	verb := []string{"obligations", "--service", "payment-service", "--file", "payment.go"}

	// No hook has recorded anything: absent with its reason, never a zero.
	out, errb, code := runVerb(t, f, append(append([]string{}, verb...), "--json")...)
	if code != 0 {
		t.Fatalf("obligations --json: %d %s", code, errb)
	}
	var none obligationsOutcomesJSON
	if err := json.Unmarshal([]byte(out), &none); err != nil {
		t.Fatalf("%v\n%s", err, out)
	}
	if none.Outcomes.State != console.StateAbsent || none.Outcomes.Reason == "" {
		t.Fatalf("before any hook fired outcomes = %+v, want absent with a reason", none.Outcomes)
	}
	if len(none.Rules) == 0 || none.Rules[0].ClaimID == "" {
		t.Fatalf("the CLI's served rules carry no claim id: %+v", none.Rules)
	}
	text, _, _ := runVerb(t, f, verb...)
	if !strings.Contains(text, "hook outcomes: no edit hook has recorded") || strings.Contains(text, "hook, last 30 days") {
		t.Errorf("text with no log must say so once and print no counts:\n%s", text)
	}

	// The hook served the top rule three times, it was addressed once and
	// ignored once.
	top := none.Rules[0].ClaimID
	for _, ev := range []string{ruleOutcomeServed, ruleOutcomeServed, ruleOutcomeServed, ruleOutcomeTopChanged, ruleOutcomeStillServed} {
		recordRuleOutcome(top, ev, "sess-1", "payment.go")
	}
	out, _, _ = runVerb(t, f, append(append([]string{}, verb...), "--json")...)
	var got obligationsOutcomesJSON
	if err := json.Unmarshal([]byte(out), &got); err != nil {
		t.Fatalf("%v\n%s", err, out)
	}
	if got.Outcomes.State != console.StatePresent && got.Outcomes.State != console.StateZero {
		t.Fatalf("outcomes state = %s (%s), want a measurement", got.Outcomes.State, got.Outcomes.Reason)
	}
	if got.Outcomes.V.WindowDays != 30 || len(got.Outcomes.V.Rules) != len(got.Rules) {
		t.Fatalf("outcomes = %+v over %d rules", got.Outcomes.V, len(got.Rules))
	}
	first := got.Outcomes.V.Rules[0]
	if first.ClaimID != top || first.Served != 3 || first.TopChanged != 1 || first.StillServed != 1 {
		t.Errorf("top rule outcomes = %+v, want served 3, top_changed 1, still_served 1", first)
	}
	for _, r := range got.Outcomes.V.Rules[1:] {
		if r.Served != 0 || r.StillServed != 0 {
			t.Errorf("a rule the hook never served reads %+v, want a measured zero", r)
		}
	}
	text, _, _ = runVerb(t, f, verb...)
	if !strings.Contains(text, "hook, last 30 days: served 3 · top changed 1 · still served 1") {
		t.Errorf("text does not show the top rule's record:\n%s", text)
	}

	// The console's serving explainer shows the same record, read at
	// request time (the explanation itself is cached).
	env := f.pageJSON(t, "/explain?svc=payment-service&file=payment.go")
	if len(env.Verbs) != 1 {
		t.Fatalf("explain verbs = %v", env.Verbs)
	}
	var section obligationsOutcomesJSON
	if err := json.Unmarshal(env.Sections[env.Verbs[0]], &section); err != nil {
		t.Fatalf("explain section: %v", err)
	}
	if section.Outcomes.State != console.StatePresent && section.Outcomes.State != console.StateZero {
		t.Fatalf("explain outcomes state = %s (%s)", section.Outcomes.State, section.Outcomes.Reason)
	}
	_, _, html := f.get(t, "/explain?svc=payment-service&file=payment.go")
	if !strings.Contains(html, `data-rule-outcomes`) || !strings.Contains(html, "3 served") || !strings.Contains(html, "1 still served") {
		t.Errorf("the explainer's served list does not show the hook's record:\n%.600s", html)
	}
}
