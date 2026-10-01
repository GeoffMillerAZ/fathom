package main

// console_rules_test.go — GL-18: the rule-effectiveness page (/rules) and its
// verb (`fathom claim effectiveness`). The page states the class and the
// proposal for each rule with its n, never invents a class from an absent
// signal, holds "never triggered" until the log reaches 90 days, and serves
// as its JSON section exactly what the verb prints.

import (
	"encoding/json"
	"net/http"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/geoffmilleraz/fathom/internal/console"
)

// ruleLogLines is a rule-outcome log in the writer's own row shape: n rows
// of event for id, daysAgo before now.
func ruleLogLines(now time.Time, id, event string, n, daysAgo int) string {
	var b strings.Builder
	for i := 0; i < n; i++ {
		e := ruleOutcomeEvent{
			SchemaVersion: ruleOutcomeSchemaVersion, FathomVersion: "test",
			TS:     now.Add(-time.Duration(daysAgo)*24*time.Hour + time.Duration(i)*time.Second).UTC().Format(time.RFC3339Nano),
			RuleID: id, Event: event, Session: "s-1",
		}
		raw, _ := json.Marshal(e)
		b.Write(raw)
		b.WriteByte('\n')
	}
	return b.String()
}

// writeRuleLog points the signal log at a fresh file holding the fixture
// fleet's history: payment-service's wrap rule ignored, its second wrap rule
// effective, its idempotency rule contested, its ADR rule never served, and
// one old row that fixes how far back the log reaches.
func writeRuleLog(t *testing.T, f *consoleFixture, reachDays int) string {
	t.Helper()
	return writeRuleLogNames(t, f, reachDays, ruleOutcomeStillServed, ruleOutcomeTopChanged)
}

// writeRuleLogNames is writeRuleLog with the names of the two tracker events
// chosen: a ledger written before JF-1 says violated and complied.
func writeRuleLogNames(t *testing.T, f *consoleFixture, reachDays int, stillServed, topChanged string) string {
	t.Helper()
	path := filepath.Join(f.dir, "rule-outcomes.jsonl")
	t.Setenv("FATHOM_RULE_OUTCOME_LOG", path)
	n := f.now
	log := ruleLogLines(n, "kc-gone", "served", 1, reachDays) +
		ruleLogLines(n, consoleFixtureClaimID, "served", 30, 5) + ruleLogLines(n, consoleFixtureClaimID, stillServed, 20, 5) +
		ruleLogLines(n, "kc-payment-wrap-2", "served", 25, 4) + ruleLogLines(n, "kc-payment-wrap-2", topChanged, 24, 4) +
		ruleLogLines(n, "kc-payment-idem", "served", 5, 3) + ruleLogLines(n, "kc-payment-idem", "refuted", 3, 3)
	mustWrite(t, path, log)
	return path
}

// ruleBadgeMarkup is a class badge as the page draws it: the word inside its
// role-tinted pill (the filter's option labels carry the same words, so a
// bare ">ignored<" would match the select).
func ruleBadgeMarkup(role, word string) string {
	return `font-medium text-` + role + `">` + word + `</span>`
}

// TestConsoleRuleEffectivenessPage is GL-18's gate.
func TestConsoleRuleEffectivenessPage(t *testing.T) {
	f := newConsoleFixture(t)
	pinConsoleVerbClocks(t, f)
	writeRuleLog(t, f, 100)

	// Registered where the sidebar, the palette and the JSON parity read it.
	p, ok := defaultConsoleRegistry().page("rules")
	if !ok || p.Group != "Evidence" || !p.Nav || len(p.Verbs) != 1 {
		t.Fatalf("the rules page is not registered as a navigable Evidence page with its verb: %+v", p)
	}

	code, _, body := f.get(t, "/rules")
	if code != http.StatusOK {
		t.Fatalf("GET /rules = %d: %s", code, body)
	}
	for _, want := range []string{
		"Rule effectiveness",
		"Proposed changes", "Governing rules",
		"proposals: fathom never edits a rules file",
		// The four classes, in words.
		ruleBadgeMarkup("caution", "ignored"), ruleBadgeMarkup("caution", "contested"), ruleBadgeMarkup("info", "never triggered"), ruleBadgeMarkup("positive", "effective"),
		// Each proposal with its evidence.
		"move to a hook (deny) or rewrite with the violating example", "still served on a repeat edit 20/30 served (67%) over 30d, never blocked",
		"prune or re-scope", "not served in 90d",
		// Counts carry their n; a rate needs 20 serves.
		"n = 5 (rate needs 20)", "67% <span", "of 30",
		// The rule statements open their claim pages.
		"Wrap every charge error with the payment context.", `href="/claims/kc-payment-wrap?svc=payment-service"`,
		// The verb the page renders.
		`data-console-verb="fathom claim effectiveness --json"`,
	} {
		if !strings.Contains(body, want) {
			t.Errorf("/rules missing %q", want)
		}
	}
	// Role utilities only: no palette color, no dark: variant.
	for _, banned := range []string{"dark:", "bg-red-", "text-red-", "bg-green-", "text-green-", "bg-yellow-", "text-yellow-", "style="} {
		if strings.Contains(body, banned) {
			t.Errorf("/rules carries %q: role utilities only", banned)
		}
	}

	// The JSON section is the verb's output.
	env := f.pageJSON(t, "/rules")
	if len(env.Verbs) != 1 || env.Verbs[0] != "fathom claim effectiveness --json" {
		t.Fatalf("verbs = %v", env.Verbs)
	}
	parity := func(verb string, section json.RawMessage) {
		t.Helper()
		var fromPage any
		if err := json.Unmarshal(section, &fromPage); err != nil {
			t.Fatal(err)
		}
		if fromVerb := runVerbJSON(t, f, verb); !reflect.DeepEqual(fromVerb, fromPage) {
			a, _ := json.MarshalIndent(fromVerb, "", " ")
			b, _ := json.MarshalIndent(fromPage, "", " ")
			t.Errorf("page disagrees with `%s`:\nverb: %s\npage: %s", verb, a, b)
		}
	}
	parity(env.Verbs[0], env.Sections[env.Verbs[0]])
	var view console.RuleEffectivenessView
	if err := json.Unmarshal(env.Sections[env.Verbs[0]], &view); err != nil {
		t.Fatal(err)
	}
	// The page body is this view rendered: every rule's statement and class
	// word, and the same verb chip.
	rendered := renderRuleEffectiveness(view, env.Verbs[0])
	for _, r := range view.Rows {
		if !strings.Contains(rendered, r.Statement) {
			t.Errorf("the rendered body lacks the rule %q", r.Statement)
		}
	}
	if !strings.Contains(rendered, "fathom claim effectiveness --json") {
		t.Error("the rendered body lacks its verb chip")
	}
	classes := map[string]string{}
	for _, r := range view.Rows {
		classes[r.RuleID] = r.Class
	}
	for id, want := range map[string]string{consoleFixtureClaimID: "ignored", "kc-payment-wrap-2": "effective", "kc-payment-idem": "contested", "kc-payment-adr": "never_triggered"} {
		if classes[id] != want {
			t.Errorf("%s class = %q, want %q (rows %v)", id, classes[id], want, classes)
		}
	}

	// A filter is a URL, and the verb carries it.
	env = f.pageJSON(t, "/rules?svc=payment-service&class=ignored")
	if len(env.Verbs) != 1 || env.Verbs[0] != "fathom claim effectiveness --service payment-service --class ignored --json" {
		t.Fatalf("filtered verbs = %v", env.Verbs)
	}
	parity(env.Verbs[0], env.Sections[env.Verbs[0]])
	_, _, ignoredOnly := f.get(t, "/rules?svc=payment-service&class=ignored")
	if !strings.Contains(ignoredOnly, "Wrap every charge error with the payment context.") || strings.Contains(ignoredOnly, "Keep refunds idempotent.") {
		t.Error("the class filter must keep the ignored rule and drop the others from the table")
	}

	// Refusals name themselves.
	if code, _, _ := f.get(t, "/rules?class=bogus"); code != http.StatusBadRequest {
		t.Errorf("an unknown class = %d, want 400", code)
	}
	if code, _, _ := f.get(t, "/rules?svc=nope-service"); code != http.StatusNotFound {
		t.Errorf("an unknown project = %d, want 404", code)
	}
}

// TestConsoleRuleEffectivenessPage_HonestAboutTheSignal: no log is not a log
// of nothing, and a log too short cannot say "never triggered".
func TestConsoleRuleEffectivenessPage_HonestAboutTheSignal(t *testing.T) {
	f := newConsoleFixture(t)
	pinConsoleVerbClocks(t, f)

	t.Setenv("FATHOM_RULE_OUTCOME_LOG", filepath.Join(f.dir, "no-such-log.jsonl"))
	code, _, none := f.get(t, "/rules")
	if code != http.StatusOK {
		t.Fatalf("GET /rules without a log = %d", code)
	}
	for _, want := range []string{`data-truth-state="absent"`, "not measured", "No rule is classified yet", "Nothing to propose without a signal"} {
		if !strings.Contains(none, want) {
			t.Errorf("no log: /rules missing %q", want)
		}
	}
	for _, banned := range []string{ruleBadgeMarkup("info", "never triggered"), ruleBadgeMarkup("caution", "ignored"), "prune or re-scope"} {
		if strings.Contains(none, banned) {
			t.Errorf("no log: /rules classified something (%q) from no signal", banned)
		}
	}

	writeRuleLog(t, f, 10)
	_, _, short := f.get(t, "/rules")
	if strings.Contains(short, ruleBadgeMarkup("info", "never triggered")) || strings.Contains(short, "prune or re-scope") {
		t.Error("a 10-day signal must not classify or propose pruning a never-triggered rule")
	}
	for _, want := range []string{"not judged yet", "never triggered needs 90 days"} {
		if !strings.Contains(short, want) {
			t.Errorf("short log: /rules missing %q", want)
		}
	}
	if !strings.Contains(short, ruleBadgeMarkup("caution", "ignored")) {
		t.Error("the 30-day classes stand on a short log")
	}
}

// TestRenderRuleEffectiveness renders a projected view straight through the
// page body: stat band, both panels, notes.
func TestRenderRuleEffectiveness(t *testing.T) {
	now := time.Date(2026, 9, 28, 12, 0, 0, 0, time.UTC)
	v := console.RuleEffectivenessView{
		Filter: console.RuleEffectivenessFilter{Class: console.RuleClassAll}, AsOf: now.Format(time.RFC3339),
		Signal:   console.Measured(70, "log"),
		SpanDays: console.Measured(100, "log"),
		Rules:    console.Measured(2, "store"),
		Classes:  []console.RuleClassTally{{Class: "ignored", Count: 1, Means: "m"}, {Class: "contested"}, {Class: "never_triggered"}, {Class: "unclassified", Count: 1}, {Class: "effective"}},
		Total:    2, Page: 1, Pages: 1, PerPage: 50,
		Rows: []console.RuleEffectivenessRow{
			{RuleID: "r1", Service: "svc", Layer: "obligations", Statement: "Do <b>not</b> ignore me.", Class: "ignored", Served30: 30, StillServed30: 20, Served90: 30, Remedy: "move to a hook"},
			{RuleID: "r2", Service: "svc", Layer: "obligations", Statement: "Quiet.", Class: "unclassified", Served30: 2},
		},
		Remedies:      []console.RuleRemedyRow{{RuleID: "r1", Service: "svc", Statement: "Do <b>not</b> ignore me.", Class: "ignored", Remedy: "move to a hook", Evidence: "still served 20/30"}},
		RemediesTotal: 1, Services: []string{"svc"}, Notes: []string{"a <script> note"},
	}
	out := renderRuleEffectiveness(v, "fathom claim effectiveness --json")
	if strings.Contains(out, "<b>not</b>") || strings.Contains(out, "<script>") {
		t.Error("a rule statement and a note are escaped, never markup")
	}
	for _, want := range []string{"Proposed changes", "Governing rules", "move to a hook", "still served 20/30", "data-rule-notes", "rule effectiveness at a glance"} {
		if !strings.Contains(out, want) {
			t.Errorf("rendered body missing %q", want)
		}
	}
}

// TestReadRuleOutcomeSignal: a missing log is absent, an unreadable one an
// error, a bad row is counted and not silently dropped, and a log past the
// read bound is read from its newest bytes without its cut first row.
func TestReadRuleOutcomeSignal(t *testing.T) {
	dir := t.TempDir()
	if sig := readRuleOutcomeSignal(filepath.Join(dir, "nope.jsonl")); sig.State != console.StateAbsent || sig.Reason == "" || len(sig.Events) != 0 {
		t.Errorf("a missing log = %+v, want absent with a reason", sig)
	}
	if sig := readRuleOutcomeSignal(""); sig.State != console.StateAbsent {
		t.Errorf("no path = %+v, want absent", sig)
	}
	if sig := readRuleOutcomeSignal(dir); sig.State != console.StateError {
		t.Errorf("a directory = %+v, want an error", sig)
	}

	now := time.Date(2026, 9, 28, 12, 0, 0, 0, time.UTC)
	path := filepath.Join(dir, "log.jsonl")
	good := ruleLogLines(now, "r1", "served", 3, 2)
	mustWrite(t, path, good+"not json\n"+`{"rule_id":"","event":"served","ts":"2026-09-28T00:00:00Z"}`+"\n"+`{"rule_id":"r1","event":"served","ts":"yesterday"}`+"\n")
	sig := readRuleOutcomeSignal(path)
	if sig.State != console.StatePresent || len(sig.Events) != 3 || sig.Skipped != 3 {
		t.Errorf("events %d skipped %d state %s, want 3 events and 3 skipped rows", len(sig.Events), sig.Skipped, sig.State)
	}
	if e := sig.Events[0]; e.RuleID != "r1" || e.Event != "served" || e.Session != "s-1" || e.At.IsZero() {
		t.Errorf("event = %+v", e)
	}

	// Past the bound: only the newest whole rows, and the row the bound cuts
	// is dropped, not counted as a bad one.
	prev := ruleOutcomeReadMax
	defer func() { ruleOutcomeReadMax = prev }()
	newest := ruleLogLines(now, "new", "served", 5, 1)
	mustWrite(t, path, ruleLogLines(now, "old", "served", 50, 20)+newest)
	ruleOutcomeReadMax = int64(len(newest)) + 40 // the new rows plus the tail of one old row
	sig = readRuleOutcomeSignal(path)
	if sig.Skipped != 0 {
		t.Errorf("skipped = %d: the row cut by the bound is dropped, not counted as a bad row", sig.Skipped)
	}
	if n := len(sig.Events); n != 5 || sig.Events[n-1].RuleID != "new" {
		t.Errorf("events = %d, want exactly the 5 newest rows: %+v", n, sig.Events)
	}
}

// TestConsoleRuleEffectivenessReadsAnOldNameLedger (JF-2): a ledger written
// before JF-1 holds violated and complied rows. /rules and `fathom claim
// effectiveness --json` read it exactly as the same history in the new names,
// carry the renamed fields, and no longer name a count after compliance.
func TestConsoleRuleEffectivenessReadsAnOldNameLedger(t *testing.T) {
	read := func(stillServed, topChanged string) (console.RuleEffectivenessView, string) {
		t.Helper()
		f := newConsoleFixture(t)
		pinConsoleVerbClocks(t, f)
		writeRuleLogNames(t, f, 100, stillServed, topChanged)
		env := f.pageJSON(t, "/rules")
		raw := env.Sections[env.Verbs[0]]
		var v console.RuleEffectivenessView
		if err := json.Unmarshal(raw, &v); err != nil {
			t.Fatal(err)
		}
		return v, string(raw)
	}
	oldView, oldJSON := read("violated", "complied")
	newView, _ := read(ruleOutcomeStillServed, ruleOutcomeTopChanged)

	if !reflect.DeepEqual(oldView.Rows, newView.Rows) || !reflect.DeepEqual(oldView.Classes, newView.Classes) || !reflect.DeepEqual(oldView.Remedies, newView.Remedies) {
		t.Errorf("an old-name ledger reads differently from the same history in the new names:\n old %+v\n new %+v", oldView.Rows, newView.Rows)
	}
	var ignored, effective console.RuleEffectivenessRow
	for _, r := range oldView.Rows {
		switch r.RuleID {
		case consoleFixtureClaimID:
			ignored = r
		case "kc-payment-wrap-2":
			effective = r
		}
	}
	if ignored.Class != "ignored" || ignored.Served30 != 30 || ignored.StillServed30 != 20 || ignored.TopChanged30 != 0 || ignored.StillServedRate == nil {
		t.Errorf("the violated rows were not counted as still_served: %+v", ignored)
	}
	if effective.Class != "effective" || effective.TopChanged30 != 24 || effective.StillServed30 != 0 || effective.TopChangedRate == nil {
		t.Errorf("the complied rows were not counted as top_changed: %+v", effective)
	}
	for _, want := range []string{`"still_served_30d"`, `"top_changed_30d"`, `"still_served_rate"`, `"top_changed_rate"`} {
		if !strings.Contains(oldJSON, want) {
			t.Errorf("the JSON section lacks %s", want)
		}
	}
	for _, retired := range []string{`"violated_30d"`, `"complied_30d"`, `"violation_rate"`, `"compliance_rate"`} {
		if strings.Contains(oldJSON, retired) {
			t.Errorf("the JSON section still carries %s: a count of repeat edits is not compliance", retired)
		}
	}
}
