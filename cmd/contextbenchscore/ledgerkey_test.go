package main

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

// NP-B9 / BX-87: both verbs that append to the bench ledger refuse a
// repeated (run_id, criterion_id) with exit 2 and leave the file unchanged,
// while the same run under another criterion still files.
func TestCmdLedgerAndBackfill_RefuseARepeatedRunAndCriterion(t *testing.T) {
	dir := t.TempDir()
	tasks := writeGatedCorpus(t, dir)
	results := filepath.Join(dir, "results")
	writeCells(t, results, concat(threeTrials("T1", "fathom", true, 100), threeTrials("T1", "grep", false, 200)))
	ledger := filepath.Join(dir, "ledger.jsonl")
	file := func(criterion string) int {
		return run([]string{"ledger", "-tasks", tasks, "-results", results, "-ledger", ledger, "-run-id", "r1", "-commit", "abc", "-criterion", criterion})
	}
	if rc := file("H2-1"); rc != 0 {
		t.Fatalf("first filing: exit %d", rc)
	}
	if rc := file("H2-2"); rc != 0 {
		t.Fatalf("the same run under another criterion is a new key: exit %d", rc)
	}
	before, err := os.ReadFile(ledger)
	if err != nil {
		t.Fatal(err)
	}
	if rc := file("H2-1"); rc != 2 {
		t.Errorf("ledger: a repeated (run_id, criterion_id) must exit 2, got %d", rc)
	}
	backfill := run([]string{"backfill", "-tasks", tasks, "-ledger", ledger, "-run-id", "r1", "-commit", "abc", "-task-count", "1", "-notes", "backfilled from a test"})
	if backfill != 2 {
		t.Errorf("backfill: a repeated (run_id, criterion_id) must exit 2, got %d", backfill)
	}
	after, _ := os.ReadFile(ledger)
	if !bytes.Equal(before, after) {
		t.Error("a refused filing must leave the ledger unchanged")
	}
	if rc := run([]string{"backfill", "-tasks", tasks, "-ledger", ledger, "-run-id", "r0", "-commit", "abc", "-task-count", "1", "-notes", "backfilled from a test"}); rc != 0 {
		t.Errorf("backfill of a new run: exit %d", rc)
	}
}
