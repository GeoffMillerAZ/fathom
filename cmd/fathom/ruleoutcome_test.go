// ruleoutcome_test.go — GL-10 (docs/plans/governance-learning.md, design
// doc §5): the signal source's own recording primitive, tested in
// isolation from any real hook invocation.
package main

import (
	"bufio"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestRuleOutcomeEventsRecorded(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "rule-outcomes.jsonl")
	t.Setenv("FATHOM_RULE_OUTCOME_LOG", path)

	fixed := time.Date(2026, 9, 28, 12, 0, 0, 0, time.UTC)
	old := ruleOutcomeNow
	ruleOutcomeNow = func() time.Time { return fixed }
	t.Cleanup(func() { ruleOutcomeNow = old })

	recordRuleOutcome("GL-11", ruleOutcomeServed, "sess-1", "internal/semantic/foo.go")
	recordRuleOutcome("GL-11", ruleOutcomeTopChanged, "sess-1", "internal/semantic/foo.go")
	// Both of these are dropped: an event needs both a rule id and a kind.
	recordRuleOutcome("", ruleOutcomeStillServed, "sess-1", "x.go")
	recordRuleOutcome("GL-11", "", "sess-1", "x.go")

	rows := readRuleOutcomeRows(t, path)
	if len(rows) != 2 {
		t.Fatalf("want 2 recorded rows, got %d: %+v", len(rows), rows)
	}
	if rows[0].RuleID != "GL-11" || rows[0].Event != ruleOutcomeServed {
		t.Errorf("row 1 = %+v, want rule_id=GL-11 event=served", rows[0])
	}
	if rows[0].TS != fixed.Format(time.RFC3339Nano) {
		t.Errorf("ts = %q, want %q", rows[0].TS, fixed.Format(time.RFC3339Nano))
	}
	if rows[0].Session != "sess-1" || rows[0].File != "internal/semantic/foo.go" {
		t.Errorf("row1 session/file = %q/%q", rows[0].Session, rows[0].File)
	}
	if rows[0].SchemaVersion != ruleOutcomeSchemaVersion {
		t.Errorf("schema_version = %d, want %d", rows[0].SchemaVersion, ruleOutcomeSchemaVersion)
	}
	if rows[1].Event != ruleOutcomeTopChanged {
		t.Errorf("row2 event = %q, want %q", rows[1].Event, ruleOutcomeTopChanged)
	}
}

// TestRuleOutcomeLogRotates proves GL-10's 16 MiB rotation bound: a log
// already over the (test-shrunk) cap is rotated to path+".1" before the
// next row is appended — the beat-log precedent (cmd/fathom/beats.go,
// internal/dlog.RotateIfNeeded), never left to grow unbounded.
func TestRuleOutcomeLogRotates(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "rule-outcomes.jsonl")
	t.Setenv("FATHOM_RULE_OUTCOME_LOG", path)

	oldCap := ruleOutcomeLogRotateBytes
	ruleOutcomeLogRotateBytes = 100
	t.Cleanup(func() { ruleOutcomeLogRotateBytes = oldCap })

	if err := os.WriteFile(path, make([]byte, 200), 0o644); err != nil {
		t.Fatal(err)
	}
	recordRuleOutcome("GL-11", ruleOutcomeServed, "", "")
	if _, err := os.Stat(path + ".1"); err != nil {
		t.Fatalf("expected a rotated backup at %s.1: %v", path, err)
	}
}

// TestRuleOutcomeNeverPanicsWithoutHome proves the silent-failure contract
// (matches emitBeat's, cmd/fathom/beats.go): an unresolvable log path never
// panics or blocks the caller.
func TestRuleOutcomeNeverPanicsWithoutHome(t *testing.T) {
	t.Setenv("FATHOM_RULE_OUTCOME_LOG", "")
	t.Setenv("HOME", "")
	recordRuleOutcome("GL-11", ruleOutcomeServed, "", "") // must not panic
}

type ruleOutcomeRow struct {
	SchemaVersion int    `json:"schema_version"`
	FathomVersion string `json:"fathom_version"`
	TS            string `json:"ts"`
	RuleID        string `json:"rule_id"`
	Event         string `json:"event"`
	Session       string `json:"session"`
	File          string `json:"file"`
}

func readRuleOutcomeRows(t *testing.T, path string) []ruleOutcomeRow {
	t.Helper()
	f, err := os.Open(path)
	if err != nil {
		t.Fatalf("open %s: %v", path, err)
	}
	defer f.Close()
	var rows []ruleOutcomeRow
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := sc.Bytes()
		if len(line) == 0 {
			continue
		}
		var row ruleOutcomeRow
		if err := json.Unmarshal(line, &row); err != nil {
			t.Fatalf("unmarshal row %q: %v", line, err)
		}
		rows = append(rows, row)
	}
	if err := sc.Err(); err != nil {
		t.Fatalf("scan %s: %v", path, err)
	}
	return rows
}
