package console

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/semantic"
	"github.com/geoffmilleraz/fathom/internal/store"
)

var ruleNow = time.Date(2026, 9, 28, 12, 0, 0, 0, time.UTC)

func ruleClaim(svc, id, layer, rule string, status core.ClaimStatus, lifecycle core.ClaimLifecycle) core.SemanticClaim {
	return core.SemanticClaim{
		ClaimID: id, ServiceID: svc, Layer: layer, LayerVersion: "v1", Kind: "obligation", Subject: "CLAUDE.md#" + id,
		Rule: rule, Scope: []string{"**"}, Tier: core.TierDocClaimed, TierProfile: "engineering", Status: status,
		Lifecycle: lifecycle, PromptVersion: "p", ReproKey: id, InputDigest: "d", CompiledAtWatermark: "w", CompiledAt: ruleNow.Add(-200 * 24 * time.Hour),
	}
}

func ruleEvents(id, kind string, n, daysAgo int) []semantic.RuleOutcomeEvent {
	var out []semantic.RuleOutcomeEvent
	for i := 0; i < n; i++ {
		out = append(out, semantic.RuleOutcomeEvent{RuleID: id, Event: kind, At: ruleNow.Add(-time.Duration(daysAgo)*24*time.Hour + time.Duration(i)*time.Minute)})
	}
	return out
}

func ruleFixture(t *testing.T, extra ...core.SemanticClaim) KnowledgeSource {
	t.Helper()
	st := store.NewMemoryStore()
	claims := []core.SemanticClaim{
		ruleClaim("alpha", "r-ignored", "obligations", "Never call time.Now in internal/.", core.ClaimFresh, ""),
		ruleClaim("alpha", "r-effective", "obligations", "Keep the store pure.", core.ClaimFresh, ""),
		ruleClaim("alpha", "r-contested", "obligations", "Every claim cites a line.", core.ClaimStale, ""),
		ruleClaim("alpha", "r-never", "obligations", "Never used rule.", core.ClaimFresh, ""),
		ruleClaim("alpha", "r-quiet", "obligations", "Rarely served rule.", core.ClaimFresh, ""),
		ruleClaim("beta", "r-beta", "obligations", "Beta rule.", core.ClaimFresh, ""),
		// Not governing rules: a claims-role layer, a retired rule, a superseded one.
		ruleClaim("alpha", "c-binding", "findings", "A plan-row binding.", core.ClaimFresh, ""),
		ruleClaim("alpha", "r-retired", "obligations", "Retired rule.", core.ClaimFresh, core.LifecycleRetired),
		ruleClaim("alpha", "r-superseded", "obligations", "Superseded rule.", core.ClaimSuperseded, ""),
	}
	claims = append(claims, extra...)
	if err := st.PutClaims(context.Background(), claims, nil); err != nil {
		t.Fatal(err)
	}
	return KnowledgeSource{Store: st}
}

func ruleSignal(daysBack int) RuleSignal {
	var ev []semantic.RuleOutcomeEvent
	ev = append(ev, ruleEvents("r-ignored", "served", 30, 5)...)
	ev = append(ev, ruleEvents("r-ignored", "violated", 20, 5)...)
	ev = append(ev, ruleEvents("r-effective", "served", 25, 4)...)
	ev = append(ev, ruleEvents("r-effective", "complied", 24, 4)...)
	ev = append(ev, ruleEvents("r-contested", "served", 5, 3)...)
	ev = append(ev, ruleEvents("r-contested", "refuted", 3, 3)...)
	ev = append(ev, ruleEvents("r-quiet", "served", 3, 2)...)
	ev = append(ev, ruleEvents("r-beta", "served", 2, 2)...)
	// One old event fixes how far back the log reaches, and belongs to a
	// rule nothing since has served.
	ev = append(ev, ruleEvents("r-gone", "served", 1, daysBack)...)
	return RuleSignal{State: StatePresent, Source: "~/.fathom/rule-outcomes.jsonl", Events: ev}
}

func rowFor(v RuleEffectivenessView, id string) (RuleEffectivenessRow, bool) {
	for _, r := range v.Rows {
		if r.RuleID == id {
			return r, true
		}
	}
	return RuleEffectivenessRow{}, false
}

func tallyOf(v RuleEffectivenessView, class string) int {
	for _, c := range v.Classes {
		if c.Class == class {
			return c.Count
		}
	}
	return -1
}

func TestListRuleEffectiveness_ClassesRatesAndRemedies(t *testing.T) {
	src := ruleFixture(t)
	v, err := ListRuleEffectiveness(context.Background(), src, ruleSignal(100), RuleEffectivenessFilter{}, 1, ruleNow)
	if err != nil {
		t.Fatal(err)
	}
	if v.Rules.V != 6 || v.Rules.State != StatePresent {
		t.Errorf("Rules = %+v, want 6 governing rules (the claims-role, retired and superseded ones are not rules)", v.Rules)
	}
	if v.Signal.State != StatePresent || v.SpanDays.V < 99 {
		t.Errorf("Signal = %+v, SpanDays = %+v", v.Signal, v.SpanDays)
	}
	want := map[string]string{
		"r-ignored": "ignored", "r-effective": "effective", "r-contested": "contested",
		"r-never": "never_triggered", "r-quiet": "unclassified", "r-beta": "unclassified",
	}
	if len(v.Rows) != len(want) {
		t.Fatalf("rows = %d, want %d: %+v", len(v.Rows), len(want), v.Rows)
	}
	for id, class := range want {
		r, ok := rowFor(v, id)
		if !ok || r.Class != class {
			t.Errorf("%s = %+v, want class %s", id, r, class)
		}
	}
	// Order: what needs a human first, then by how much it was served.
	var order []string
	for _, r := range v.Rows {
		order = append(order, r.Class)
	}
	if got := strings.Join(order, ","); got != "ignored,contested,never_triggered,unclassified,unclassified,effective" {
		t.Errorf("row order = %s", got)
	}
	// Counts and rates carry their n; a rate needs 20 serves.
	ign, _ := rowFor(v, "r-ignored")
	if ign.Served30 != 30 || ign.Violated30 != 20 || ign.ViolationRate == nil || *ign.ViolationRate < 0.66 || *ign.ViolationRate > 0.67 {
		t.Errorf("r-ignored = %+v", ign)
	}
	quiet, _ := rowFor(v, "r-quiet")
	if quiet.Served30 != 3 || quiet.ComplianceRate != nil || quiet.ViolationRate != nil {
		t.Errorf("r-quiet = %+v: below 20 serves shows n and no rate", quiet)
	}
	// Tallies count every class; the retired, superseded and claims-role
	// claims are in none.
	if tallyOf(v, "ignored") != 1 || tallyOf(v, "effective") != 1 || tallyOf(v, "contested") != 1 || tallyOf(v, "never_triggered") != 1 || tallyOf(v, "unclassified") != 2 {
		t.Errorf("classes = %+v", v.Classes)
	}
	// Remedies are proposals for the two actionable classes, with evidence.
	if v.RemediesTotal != 2 || len(v.Remedies) != 2 {
		t.Fatalf("remedies = %+v (total %d), want the ignored and the never-triggered rule", v.Remedies, v.RemediesTotal)
	}
	byClass := map[string]RuleRemedyRow{}
	for _, r := range v.Remedies {
		byClass[r.Class] = r
	}
	if r := byClass["ignored"]; r.RuleID != "r-ignored" || !strings.Contains(r.Remedy, "hook") || !strings.Contains(r.Evidence, "20/30") || r.Statement == "" {
		t.Errorf("ignored remedy = %+v", r)
	}
	if r := byClass["never_triggered"]; r.RuleID != "r-never" || r.Remedy != "prune or re-scope" || !strings.Contains(r.Evidence, "90d") {
		t.Errorf("never-triggered remedy = %+v", r)
	}
	// The log names a rule the store does not hold: said, not dropped.
	if !strings.Contains(strings.Join(v.Notes, "|"), "1 rule ids in the signal log are not claims in this store") {
		t.Errorf("notes = %v", v.Notes)
	}
}

// TestListRuleEffectiveness_NeverTriggeredNeedsTheWindow: a log that reaches
// back ten days cannot say a rule was not served in ninety, so the rule is
// left unclassified with the reason and no prune is proposed (I38).
func TestListRuleEffectiveness_NeverTriggeredNeedsTheWindow(t *testing.T) {
	v, err := ListRuleEffectiveness(context.Background(), ruleFixture(t), ruleSignal(10), RuleEffectivenessFilter{}, 1, ruleNow)
	if err != nil {
		t.Fatal(err)
	}
	r, _ := rowFor(v, "r-never")
	if r.Class != "unclassified" || r.Remedy != "" || !strings.Contains(r.Note, "90 days") {
		t.Errorf("r-never = %+v, want unclassified with the window named", r)
	}
	if tallyOf(v, "never_triggered") != 0 {
		t.Errorf("never_triggered tally = %d, want 0 until the log covers 90 days", tallyOf(v, "never_triggered"))
	}
	for _, rm := range v.Remedies {
		if rm.Class == "never_triggered" {
			t.Errorf("a prune was proposed on a 10-day signal: %+v", rm)
		}
	}
	if !strings.Contains(strings.Join(v.Notes, "|"), "not judged yet") {
		t.Errorf("notes = %v, want the withheld judgement named", v.Notes)
	}
	// The ratio classes still stand: they read 30 days and need n.
	if ign, _ := rowFor(v, "r-ignored"); ign.Class != "ignored" {
		t.Errorf("r-ignored = %+v", ign)
	}
}

// TestListRuleEffectiveness_NoSignalClassifiesNothing: no log is not a log
// of nothing. Absent and unreadable signals leave the rows empty and say why;
// the governing rules are still counted.
func TestListRuleEffectiveness_NoSignalClassifiesNothing(t *testing.T) {
	for name, sig := range map[string]RuleSignal{
		"absent":     {State: StateAbsent, Reason: "no rule-outcome log at ~/.fathom/rule-outcomes.jsonl"},
		"empty":      {State: StatePresent, Source: "log"},
		"unreadable": {State: StateError, Reason: "permission denied"},
	} {
		v, err := ListRuleEffectiveness(context.Background(), ruleFixture(t), sig, RuleEffectivenessFilter{}, 1, ruleNow)
		if err != nil {
			t.Fatal(name, err)
		}
		if v.Signal.Known() {
			t.Errorf("%s: Signal = %+v, want absent or error, never a count", name, v.Signal)
		}
		if err := v.Signal.Validate(); err != nil {
			t.Errorf("%s: %v", name, err)
		}
		if len(v.Rows) != 0 || len(v.Remedies) != 0 || v.Total != 0 {
			t.Errorf("%s: rows %d remedies %d: nothing is classified without a signal", name, len(v.Rows), len(v.Remedies))
		}
		if v.Rules.V != 6 {
			t.Errorf("%s: Rules = %+v, want the 6 governing rules still counted", name, v.Rules)
		}
		for _, c := range v.Classes {
			if c.Count != 0 {
				t.Errorf("%s: class %s counts %d", name, c.Class, c.Count)
			}
		}
		if len(v.Classes) != 5 {
			t.Errorf("%s: classes = %+v, want all five named even when empty", name, v.Classes)
		}
	}
}

func TestListRuleEffectiveness_FilterAndPaging(t *testing.T) {
	var many []core.SemanticClaim
	for i := 0; i < 120; i++ {
		many = append(many, ruleClaim("alpha", fmt.Sprintf("bulk-%03d", i), "obligations", fmt.Sprintf("Bulk rule %03d.", i), core.ClaimFresh, ""))
	}
	src := ruleFixture(t, many...)
	ctx := context.Background()
	v, err := ListRuleEffectiveness(ctx, src, ruleSignal(100), RuleEffectivenessFilter{Service: "alpha"}, 1, ruleNow)
	if err != nil {
		t.Fatal(err)
	}
	if v.Total != 125 || v.Pages != 3 || v.PerPage != RulesPerPage || len(v.Rows) != RulesPerPage {
		t.Errorf("alpha total/pages/rows = %d/%d/%d", v.Total, v.Pages, len(v.Rows))
	}
	if _, ok := rowFor(v, "r-beta"); ok {
		t.Error("a project filter must leave the other project's rules out")
	}
	last, _ := ListRuleEffectiveness(ctx, src, ruleSignal(100), RuleEffectivenessFilter{Service: "alpha"}, 99, ruleNow)
	if last.Page != 3 || len(last.Rows) != 25 {
		t.Errorf("a page past the end is the last: page %d rows %d", last.Page, len(last.Rows))
	}
	ign, _ := ListRuleEffectiveness(ctx, src, ruleSignal(100), RuleEffectivenessFilter{Class: "ignored"}, 1, ruleNow)
	if ign.Total != 1 || ign.Rows[0].RuleID != "r-ignored" {
		t.Errorf("class filter = %+v", ign.Rows)
	}
	if tallyOf(ign, "effective") != 1 {
		t.Error("the tally counts every class of the filtered projects, before the class filter")
	}
	if _, err := ListRuleEffectiveness(ctx, src, ruleSignal(100), RuleEffectivenessFilter{Class: "bogus"}, 1, ruleNow); err == nil {
		t.Error("an unknown class must be an error, not an empty page")
	}
	// Deterministic: the same inputs are the same bytes.
	a, _ := json.Marshal(v)
	b, _ := ListRuleEffectiveness(ctx, src, ruleSignal(100), RuleEffectivenessFilter{Service: "alpha"}, 1, ruleNow)
	bj, _ := json.Marshal(b)
	if string(a) != string(bj) {
		t.Error("the projection is not deterministic")
	}
}
