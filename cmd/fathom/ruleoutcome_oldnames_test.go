package main

// ruleoutcome_oldnames_test.go — JF-2 (docs/plans/judgment-foundations.md,
// docs/design/judgment-foundations.md section 5): the tracker's events were
// renamed complied -> top_changed and violated -> still_served, and a ledger
// on disk holds both generations. Every reader of the ledger reads the old
// rows under the new names, never dropping one, and the counts a user sees
// carry the new words.

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/geoffmilleraz/fathom/internal/console"
	"github.com/geoffmilleraz/fathom/internal/semantic"
)

// TestRuleOutcomeReadersMapOldEventNames writes a ledger the way a binary
// from before JF-1 did (schema_version 1, complied and violated), appends rows
// through the current recorder, and reads the one file through each reader.
func TestRuleOutcomeReadersMapOldEventNames(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "rule-outcomes.jsonl")
	t.Setenv("FATHOM_RULE_OUTCOME_LOG", path)

	now := time.Date(2026, 9, 28, 12, 0, 0, 0, time.UTC)
	origNow := ruleOutcomeNow
	ruleOutcomeNow = func() time.Time { return now.Add(-time.Minute) }
	t.Cleanup(func() { ruleOutcomeNow = origNow })

	oldRow := func(hoursAgo int, rule, event, session, file string) string {
		return fmt.Sprintf(`{"schema_version":1,"fathom_version":"0.9.0","ts":%q,"rule_id":%q,"event":%q,"session":%q,"file":%q}`,
			now.Add(-time.Duration(hoursAgo)*time.Hour).Format(time.RFC3339Nano), rule, event, session, file)
	}
	old := strings.Join([]string{
		oldRow(10, "claim-a", "served", "s1", "a.go"),
		oldRow(9, "claim-a", "served", "s1", "a.go"),
		oldRow(9, "claim-a", "violated", "s1", "a.go"), // the repeat edit
		oldRow(8, "claim-b", "served", "s1", "a.go"),
		oldRow(8, "claim-a", "complied", "s1", "a.go"), // the top claim changed
		oldRow(7, "claim-c", "blocked", "s1", "b.go"),
	}, "\n") + "\n"
	if err := os.WriteFile(path, []byte(old), 0o644); err != nil {
		t.Fatal(err)
	}
	// The current binary appends under the new names (JF-1).
	recordRuleOutcome("claim-a", ruleOutcomeServed, "s2", "a.go")
	recordRuleOutcome("claim-a", ruleOutcomeStillServed, "s2", "a.go")
	const rows = 8

	count := func(names []string) map[string]int {
		m := map[string]int{}
		for _, n := range names {
			m[n]++
		}
		return m
	}
	// want is the whole ledger by event: 3 old and 1 new served rows, the one
	// violated and the one new still_served, the complied and the blocked.
	want := map[string]int{"served": 4, "still_served": 2, "top_changed": 1, "blocked": 1}
	check := func(reader string, names []string) {
		t.Helper()
		got := count(names)
		if len(names) != rows {
			t.Errorf("%s read %d of %d rows: a row of an older ledger was dropped", reader, len(names), rows)
		}
		for _, retired := range []string{"complied", "violated"} {
			if got[retired] != 0 {
				t.Errorf("%s still returns the retired event name %q: %v", reader, retired, got)
			}
		}
		for ev, n := range want {
			if got[ev] != n {
				t.Errorf("%s: %s = %d, want %d (%v)", reader, ev, got[ev], n, got)
			}
		}
	}

	// The report reader (internal/console RuleOutcomesView's input).
	log := readRuleOutcomeLog(path, now)
	if log.Read.Err != nil || log.Read.Skipped != 0 || log.Read.Rows != rows {
		t.Fatalf("read = %+v, want %d clean rows", log.Read, rows)
	}
	var names []string
	for _, e := range log.Events {
		names = append(names, e.Event)
	}
	check("readRuleOutcomeLog", names)

	// The effectiveness reader (`fathom claim effectiveness`, /rules).
	sig := readRuleOutcomeSignal(path)
	if sig.State != console.StatePresent || sig.Skipped != 0 {
		t.Fatalf("signal = %+v, want present and clean", sig)
	}
	names = nil
	for _, e := range sig.Events {
		names = append(names, e.Event)
	}
	check("readRuleOutcomeSignal", names)

	// The work page's reader (friction, search runs ended by an edit).
	ruleEvents, _ := workRuleEvents(dir)
	names = nil
	for _, e := range ruleEvents {
		names = append(names, e.Event)
	}
	check("workRuleEvents", names)
	// workRuleEdits and friction also read a raw old-name event they are
	// handed, so no path to them can reintroduce one.
	if got := workRuleEdits([]console.RuleEvent{{Rule: "r", Event: "violated", Session: "s", File: "a.go", At: now.Format(time.RFC3339)}}); len(got) != 1 {
		t.Errorf("workRuleEdits dropped a violated row (an edit-hook event ends a search run): %+v", got)
	}

	// The counts the CLI and the console show: the same rows, new words.
	v := console.RuleOutcomesView([]string{"claim-a", "claim-b"}, log.Events, log.Read, now)
	if !v.Known() {
		t.Fatalf("outcomes = %s (%s)", v.State, v.Reason)
	}
	a, ok := v.V.ByClaim("claim-a")
	if !ok || a.Served != 3 || a.TopChanged != 1 || a.StillServed != 2 {
		t.Errorf("claim-a = %+v, want served 3, top_changed 1, still_served 2 (violated 1 + one new row)", a)
	}
	if got, want := ruleOutcomeText(v, "claim-a"), "hook, last 30 days: served 3 · top changed 1 · still served 2"; got != want {
		t.Errorf("ruleOutcomeText = %q, want %q", got, want)
	}
	if strings.Contains(ruleOutcomeText(v, "claim-a"), "complied") || strings.Contains(ruleOutcomeText(v, "claim-a"), "violated") {
		t.Errorf("the CLI line still says complied or violated: %q", ruleOutcomeText(v, "claim-a"))
	}

	// The effectiveness projection folds both generations into one count.
	for _, r := range semantic.RuleEffectiveness([]string{"claim-a"}, sig.Events, now) {
		if r.RuleID == "claim-a" && (r.Served30 != 3 || r.TopChanged30 != 1 || r.StillServed30 != 2) {
			t.Errorf("claim-a effectiveness = %+v, want served 3, top_changed 1, still_served 2", r)
		}
	}
}
