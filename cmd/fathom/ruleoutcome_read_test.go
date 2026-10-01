package main

// ruleoutcome_read_test.go — SR-61's reader half (surface reckoning F-27/U4):
// the signal log the edit hook writes (ruleoutcome.go) is read back as
// semantic.RuleOutcomeEvent rows for `fathom obligations` and the serving
// explainer. The reader honors the writer's own rotation (path.1 then path),
// tells no log from an unreadable one, and counts what it could not decode
// instead of hiding it.

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/geoffmilleraz/fathom/internal/console"
	"github.com/geoffmilleraz/fathom/internal/semantic"
)

func TestReadRuleOutcomeLogRoundTripsTheWriter(t *testing.T) {
	path := filepath.Join(t.TempDir(), "rule-outcomes.jsonl")
	t.Setenv("FATHOM_RULE_OUTCOME_LOG", path)
	clock := time.Date(2026, 9, 28, 12, 0, 0, 0, time.UTC)
	orig := ruleOutcomeNow
	t.Cleanup(func() { ruleOutcomeNow = orig })
	ruleOutcomeNow = func() time.Time { return clock }

	recordRuleOutcome("claim-a", ruleOutcomeServed, "s1", "a.go")
	clock = clock.Add(time.Hour)
	recordRuleOutcome("claim-a", ruleOutcomeStillServed, "s1", "a.go")
	clock = clock.Add(time.Hour)
	recordRuleOutcome("claim-b", ruleOutcomeTopChanged, "s2", "b.go")

	got := readRuleOutcomeLog(path, clock)
	if got.Read.Err != nil || !got.Read.Present {
		t.Fatalf("read = %+v, want a present, clean log", got.Read)
	}
	if got.Read.Rows != 3 || got.Read.Skipped != 0 || len(got.Events) != 3 {
		t.Fatalf("rows %d skipped %d events %d, want 3/0/3", got.Read.Rows, got.Read.Skipped, len(got.Events))
	}
	if e := got.Events[1]; e.RuleID != "claim-a" || e.Event != semantic.RuleEventStillServed || e.Session != "s1" || !e.At.Equal(clock.Add(-time.Hour)) {
		t.Errorf("events[1] = %+v, want claim-a still_served in s1 at the writer's timestamp", e)
	}
	if want := time.Date(2026, 9, 28, 12, 0, 0, 0, time.UTC); !got.Read.Since.Equal(want) {
		t.Errorf("since = %s, want the earliest row %s", got.Read.Since, want)
	}
}

func TestReadRuleOutcomeLogReadsTheRotatedGenerationFirst(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "rule-outcomes.jsonl")
	now := time.Date(2026, 9, 28, 12, 0, 0, 0, time.UTC)
	old := `{"schema_version":1,"ts":"` + now.Add(-48*time.Hour).Format(time.RFC3339Nano) + `","rule_id":"claim-a","event":"served"}` + "\n"
	cur := `{"schema_version":1,"ts":"` + now.Add(-time.Hour).Format(time.RFC3339Nano) + `","rule_id":"claim-a","event":"complied"}` + "\n"
	if err := os.WriteFile(path+".1", []byte(old), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(cur), 0o644); err != nil {
		t.Fatal(err)
	}
	got := readRuleOutcomeLog(path, now)
	if len(got.Events) != 2 || got.Events[0].Event != semantic.RuleEventServed || got.Events[1].Event != semantic.RuleEventTopChanged {
		t.Fatalf("events = %+v, want the rotated generation's row first, then the live log's — a rotation hides no history", got.Events)
	}
}

func TestReadRuleOutcomeLogCountsWhatItCannotDecode(t *testing.T) {
	path := filepath.Join(t.TempDir(), "rule-outcomes.jsonl")
	now := time.Date(2026, 9, 28, 12, 0, 0, 0, time.UTC)
	good := `{"schema_version":1,"ts":"` + now.Add(-time.Hour).Format(time.RFC3339Nano) + `","rule_id":"claim-a","event":"served"}`
	body := strings.Join([]string{
		good,
		`not json at all`,
		`{"schema_version":1,"ts":"yesterday-ish","rule_id":"claim-a","event":"served"}`,
		`{"schema_version":1,"ts":"` + now.Add(-time.Hour).Format(time.RFC3339Nano) + `","rule_id":"","event":"served"}`,
		``,
	}, "\n")
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	got := readRuleOutcomeLog(path, now)
	if len(got.Events) != 1 || got.Read.Skipped != 3 {
		t.Fatalf("events %d skipped %d, want 1 good row and 3 skipped", len(got.Events), got.Read.Skipped)
	}
	v := console.RuleOutcomesView([]string{"claim-a"}, got.Events, got.Read, now)
	if v.State != console.StatePartial {
		t.Errorf("state = %s, want partial: a log with undecodable rows gives a lower bound", v.State)
	}
}

func TestReadRuleOutcomeLogDropsRowsOlderThanTheWidestWindow(t *testing.T) {
	path := filepath.Join(t.TempDir(), "rule-outcomes.jsonl")
	now := time.Date(2026, 9, 28, 12, 0, 0, 0, time.UTC)
	rows := `{"schema_version":1,"ts":"` + now.AddDate(0, 0, -200).Format(time.RFC3339Nano) + `","rule_id":"claim-a","event":"served"}` + "\n" +
		`{"schema_version":1,"ts":"` + now.AddDate(0, 0, -3).Format(time.RFC3339Nano) + `","rule_id":"claim-a","event":"served"}` + "\n"
	if err := os.WriteFile(path, []byte(rows), 0o644); err != nil {
		t.Fatal(err)
	}
	got := readRuleOutcomeLog(path, now)
	if len(got.Events) != 1 {
		t.Fatalf("events = %d, want only the row inside the 90-day window kept in memory", len(got.Events))
	}
	// Since still names the log's true first row: a 200-day-old log does
	// span the never-triggered window even though its old rows are dropped.
	if !got.Read.Since.Equal(now.AddDate(0, 0, -200)) {
		t.Errorf("since = %s, want the log's earliest row", got.Read.Since)
	}
}

func TestReadRuleOutcomeLogNoLogIsAbsentAndAnUnreadableOneIsAnError(t *testing.T) {
	dir := t.TempDir()
	missing := filepath.Join(dir, "rule-outcomes.jsonl")
	if got := readRuleOutcomeLog(missing, time.Now()); got.Read.Present || got.Read.Err != nil {
		t.Errorf("no log: %+v, want not present and no error", got.Read)
	}
	if got := readRuleOutcomeLog("", time.Now()); got.Read.Present || got.Read.Err != nil {
		t.Errorf("no resolvable path: %+v, want not present and no error", got.Read)
	}
	// A directory where the log should be: it exists and cannot be read.
	if err := os.Mkdir(missing, 0o755); err != nil {
		t.Fatal(err)
	}
	got := readRuleOutcomeLog(missing, time.Now())
	if got.Read.Err == nil {
		t.Fatalf("an unreadable log read as %+v, want an error — never a silent zero", got.Read)
	}
	if v := console.RuleOutcomesView([]string{"x"}, got.Events, got.Read, time.Now()); v.State != console.StateError {
		t.Errorf("state = %s, want error", v.State)
	}
}

func TestReadRuleOutcomeLogIsReusedUntilTheFilesChange(t *testing.T) {
	path := filepath.Join(t.TempDir(), "rule-outcomes.jsonl")
	t.Setenv("FATHOM_RULE_OUTCOME_LOG", path)
	now := time.Now().UTC()
	orig := ruleOutcomeNow
	t.Cleanup(func() { ruleOutcomeNow = orig })
	ruleOutcomeNow = func() time.Time { return now }

	recordRuleOutcome("claim-a", ruleOutcomeServed, "s", "a.go")
	first := cachedRuleOutcomeLog(path, now)
	second := cachedRuleOutcomeLog(path, now)
	if len(first.Events) != 1 || len(second.Events) != 1 {
		t.Fatalf("events = %d/%d, want 1/1", len(first.Events), len(second.Events))
	}
	recordRuleOutcome("claim-a", ruleOutcomeStillServed, "s", "a.go")
	third := cachedRuleOutcomeLog(path, now)
	if len(third.Events) != 2 {
		t.Errorf("after an append the cached read returned %d events, want 2 — the cache key must move with the file", len(third.Events))
	}
}
