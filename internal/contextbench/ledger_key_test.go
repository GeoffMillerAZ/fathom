package contextbench_test

import (
	"bytes"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/contextbench"
)

// NP-B9 / BX-87: the bench ledger is keyed by (run_id,
// evaluation.criterion_id). One run may be scored under several criteria,
// so the run id alone is not the key; the pair is.

func keyed(run, criterion string) contextbench.LedgerRow {
	r := row(run, contextbench.VerdictFail)
	r.Evaluation.CriterionID = criterion
	return r
}

func TestCheckLedgerKeys_NamesEmptyAndRepeatedKeys(t *testing.T) {
	lines := []contextbench.LedgerLine{
		{Line: 1, Row: keyed("r1", "H2-1")},
		{Line: 2, Row: keyed("r1", "H2-3")},
		{Line: 4, Row: keyed("", "H2-1")},
		{Line: 5, Row: keyed("r2", " ")},
		{Line: 7, Row: keyed("r1", "H2-1")},
	}
	err := contextbench.CheckLedgerKeys(lines)
	if err == nil {
		t.Fatal("want an error naming the empty and repeated keys, got nil")
	}
	msg := err.Error()
	for _, want := range []string{"line 4", "empty run_id", "line 5", "empty evaluation.criterion_id", "line 7", "(r1, H2-1)", "line 1"} {
		if !strings.Contains(msg, want) {
			t.Errorf("error should contain %q, got: %s", want, msg)
		}
	}
	if strings.Contains(msg, "line 2") {
		t.Errorf("r1/H2-3 is a distinct key and must not be named: %s", msg)
	}
	if strings.Index(msg, "line 4") > strings.Index(msg, "line 7") {
		t.Errorf("offenders are named in line order: %s", msg)
	}

	if err := contextbench.CheckLedgerKeys(lines[:2]); err != nil {
		t.Errorf("one run under two criteria is two keys: %v", err)
	}
	if err := contextbench.CheckLedgerKeys(nil); err != nil {
		t.Errorf("an empty ledger has no bad key: %v", err)
	}
}

func TestAppendLedger_RefusesARepeatedKeyAndAcceptsANewCriterion(t *testing.T) {
	path := filepath.Join(t.TempDir(), "ledger.jsonl")
	for _, r := range []contextbench.LedgerRow{keyed("r1", "H2-1"), keyed("r1", "H2-3")} {
		if err := contextbench.AppendLedger(path, r); err != nil {
			t.Fatalf("AppendLedger(%s, %s): %v", r.RunID, r.Evaluation.CriterionID, err)
		}
	}
	before, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	err = contextbench.AppendLedger(path, keyed("r1", "H2-1"))
	if err == nil {
		t.Fatal("want a refusal for a repeated (run_id, criterion_id), got nil")
	}
	if !strings.Contains(err.Error(), "(r1, H2-1)") || !strings.Contains(err.Error(), "line 1") {
		t.Errorf("refusal should name the key and the line that holds it: %v", err)
	}
	after, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(before, after) {
		t.Errorf("a refused append must leave the ledger byte for byte unchanged\nbefore:\n%s\nafter:\n%s", before, after)
	}
}

func TestAppendLedger_RefusesAnEmptyKey(t *testing.T) {
	path := filepath.Join(t.TempDir(), "ledger.jsonl")
	if err := contextbench.AppendLedger(path, keyed("r1", "")); err == nil {
		t.Fatal("a row without a criterion id has no key and must be refused")
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Errorf("a refused append must not create the ledger (stat: %v)", err)
	}
}

func TestCheckLedgerKeys_ComparesTrimmedKeys(t *testing.T) {
	err := contextbench.CheckLedgerKeys([]contextbench.LedgerLine{
		{Line: 1, Row: keyed("r1", "H2-1")},
		{Line: 2, Row: keyed("r1 ", " H2-1")},
	})
	if err == nil || !strings.Contains(err.Error(), "line 2") {
		t.Fatalf("a padded copy of a key is the same key, want line 2 named, got %v", err)
	}
}

func TestAppendLedger_RefusesOverACorruptLineAndNamesIt(t *testing.T) {
	path := filepath.Join(t.TempDir(), "ledger.jsonl")
	if err := contextbench.AppendLedger(path, keyed("r1", "H2-1")); err != nil {
		t.Fatal(err)
	}
	f, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.WriteString("{not json\n"); err != nil {
		t.Fatal(err)
	}
	f.Close()
	before, _ := os.ReadFile(path)
	err = contextbench.AppendLedger(path, keyed("r2", "H2-1"))
	if err == nil || !strings.Contains(err.Error(), "line 2") {
		t.Fatalf("want a refusal naming line 2, got %v", err)
	}
	after, _ := os.ReadFile(path)
	if !bytes.Equal(before, after) {
		t.Error("a refused append must leave the ledger unchanged")
	}
}

func TestAppendLedger_TerminatesAnUnterminatedLastLine(t *testing.T) {
	path := filepath.Join(t.TempDir(), "ledger.jsonl")
	if err := contextbench.AppendLedger(path, keyed("r1", "H2-1")); err != nil {
		t.Fatal(err)
	}
	b, _ := os.ReadFile(path)
	if err := os.WriteFile(path, bytes.TrimRight(b, "\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := contextbench.AppendLedger(path, keyed("r2", "H2-1")); err != nil {
		t.Fatal(err)
	}
	rows, err := contextbench.LoadLedger(path)
	if err != nil || len(rows) != 2 {
		t.Fatalf("want 2 rows after appending past an unterminated line, got %d (%v)", len(rows), err)
	}
}

func TestCommittedLedgerIsKeyedByRunAndCriterion(t *testing.T) {
	// An unreadable committed ledger fails this test; it never skips.
	lines, err := contextbench.LoadLedgerLines("../../docs/bench/ledger.jsonl")
	if err != nil {
		t.Fatalf("reading the committed ledger: %v", err)
	}
	if len(lines) == 0 {
		t.Fatal("the committed ledger read as empty")
	}
	if err := contextbench.CheckLedgerKeys(lines); err != nil {
		t.Fatalf("the committed ledger is not keyed by (run_id, criterion_id): %v", err)
	}
	var crits []string
	for _, l := range lines {
		if l.Row.RunID == "20260913T002832Z" {
			crits = append(crits, l.Row.Evaluation.CriterionID)
		}
	}
	sort.Strings(crits)
	if strings.Join(crits, ",") != "H2-1,H2-3" {
		t.Errorf("run 20260913T002832Z must keep exactly its H2-1 and H2-3 rows, got %v", crits)
	}
}
