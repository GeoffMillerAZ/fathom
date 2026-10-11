package main

// doctor_outcomewriter_test.go — GL-24's doctor row (INC-27): a writer that
// wrote nothing in 72 hours while the edit hook served claims is named, from
// the hooks log and the ledger, with an injected clock.

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// writeOutcomeWriterFixture writes a beat log holding one served,
// claim-bearing post-edit answer per hour over the 72 h before now, and a
// rule-outcome ledger whose newest row is lastRow (none when zero).
func writeOutcomeWriterFixture(t *testing.T, dir string, now, lastRow time.Time, claims string) (beatLog, ledger string) {
	t.Helper()
	beatLog = filepath.Join(dir, "hooks.jsonl")
	ledger = filepath.Join(dir, "rule-outcomes.jsonl")
	var b strings.Builder
	for h := 72; h >= 1; h-- {
		at := now.Add(-time.Duration(h) * time.Hour)
		meta := map[string]string{"emitted": "true", "phase": "post", "session": "s"}
		if claims != "" {
			meta["claims"] = claims
		}
		line, _ := json.Marshal(map[string]any{"channel": "edit-hook", "kind": "answered", "at": at.UTC().Format(time.RFC3339Nano), "meta": meta})
		b.Write(line)
		b.WriteByte('\n')
	}
	if err := os.WriteFile(beatLog, []byte(b.String()), 0o644); err != nil {
		t.Fatal(err)
	}
	if !lastRow.IsZero() {
		row := fmt.Sprintf(`{"schema_version":2,"ts":%q,"rule_id":"c1","event":"served"}`+"\n", lastRow.UTC().Format(time.RFC3339Nano))
		if err := os.WriteFile(ledger, []byte(row), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return beatLog, ledger
}

func TestDoctorNamesSilentRuleOutcomeWriter(t *testing.T) {
	now := time.Date(2026, 10, 10, 20, 0, 0, 0, time.UTC)

	// INC-27's shape: the hook served claims every hour for 72 h, the
	// ledger's newest row is ten days old.
	dir := t.TempDir()
	beatLog, ledger := writeOutcomeWriterFixture(t, dir, now, now.Add(-10*24*time.Hour), "3")
	t.Setenv("FATHOM_HOOKS_LOG", beatLog)
	t.Setenv("FATHOM_RULE_OUTCOME_LOG", ledger)
	c := checkRuleOutcomeWriterFromFiles(now)
	if c.Name != "rule_outcome_writer" || c.Status != doctorFail {
		t.Fatalf("72 h of claim-bearing answers and no row: %+v, want rule_outcome_writer FAIL", c)
	}
	for _, want := range []string{ledger, "72", "2026-09-30"} {
		if !strings.Contains(c.Reason, want) {
			t.Errorf("reason %q does not name %q", c.Reason, want)
		}
	}
	if c.Remedy == "" {
		t.Error("a FAIL row carries a remedy")
	}

	// No ledger at all: the same FAIL, naming that it was never written.
	dir = t.TempDir()
	beatLog, ledger = writeOutcomeWriterFixture(t, dir, now, time.Time{}, "1")
	t.Setenv("FATHOM_HOOKS_LOG", beatLog)
	t.Setenv("FATHOM_RULE_OUTCOME_LOG", ledger)
	if c := checkRuleOutcomeWriterFromFiles(now); c.Status != doctorFail || !strings.Contains(c.Reason, "no row") {
		t.Errorf("no ledger: %+v, want FAIL naming no row", c)
	}

	// A row an hour ago: the writer is live.
	dir = t.TempDir()
	beatLog, ledger = writeOutcomeWriterFixture(t, dir, now, now.Add(-time.Hour), "3")
	t.Setenv("FATHOM_HOOKS_LOG", beatLog)
	t.Setenv("FATHOM_RULE_OUTCOME_LOG", ledger)
	if c := checkRuleOutcomeWriterFromFiles(now); c.Status != doctorPass {
		t.Errorf("a row an hour ago: %+v, want PASS", c)
	}

	// Answers that served no claim (raw rows only) had nothing to write.
	dir = t.TempDir()
	beatLog, ledger = writeOutcomeWriterFixture(t, dir, now, time.Time{}, "0")
	t.Setenv("FATHOM_HOOKS_LOG", beatLog)
	t.Setenv("FATHOM_RULE_OUTCOME_LOG", ledger)
	if c := checkRuleOutcomeWriterFromFiles(now); c.Status != doctorPass {
		t.Errorf("answers with no claim: %+v, want PASS", c)
	}

	// Answers from a build before the claims count: unknown, a WARN.
	dir = t.TempDir()
	beatLog, ledger = writeOutcomeWriterFixture(t, dir, now, time.Time{}, "")
	t.Setenv("FATHOM_HOOKS_LOG", beatLog)
	t.Setenv("FATHOM_RULE_OUTCOME_LOG", ledger)
	if c := checkRuleOutcomeWriterFromFiles(now); c.Status != doctorWarn {
		t.Errorf("answers without a claims count: %+v, want WARN", c)
	}

	// The row is in doctor's checklist.
	found := false
	for _, c := range runDoctorChecks(t.TempDir(), "", "claude", "http://127.0.0.1:1") {
		if c.Name == "rule_outcome_writer" {
			found = true
		}
	}
	if !found {
		t.Error("rule_outcome_writer is not a doctor row")
	}
}
