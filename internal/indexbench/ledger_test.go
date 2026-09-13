package indexbench

import (
	"bufio"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestAppendCompetitorLedger_CreatesAndAppends(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "nested", "competitors-ledger.jsonl")

	row1 := CompetitorLedgerRow{
		Arm: "fathom", Version: "dev", Repo: "/repo", Commit: "abc123",
		Precision: 1, Recall: 0.9, Adjudicable: 10, Date: "2026-09-13",
	}
	if err := AppendCompetitorLedger(path, []CompetitorLedgerRow{row1}); err != nil {
		t.Fatalf("first append: %v", err)
	}
	row2 := CompetitorLedgerRow{
		Arm: "graphify", Version: "v0.9.61", Repo: "/repo", Commit: "abc123",
		Precision: 0.4, Recall: 0.3, Adjudicable: 4,
		DroppedKinds: map[string]int{"contains": 2}, Date: "2026-09-13",
	}
	if err := AppendCompetitorLedger(path, []CompetitorLedgerRow{row2}); err != nil {
		t.Fatalf("second append: %v", err)
	}

	f, err := os.Open(path)
	if err != nil {
		t.Fatalf("open ledger: %v", err)
	}
	defer f.Close()

	var rows []CompetitorLedgerRow
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		var r CompetitorLedgerRow
		if err := json.Unmarshal(sc.Bytes(), &r); err != nil {
			t.Fatalf("parse ledger line %q: %v", sc.Text(), err)
		}
		rows = append(rows, r)
	}
	if len(rows) != 2 {
		t.Fatalf("got %d rows, want 2: %+v", len(rows), rows)
	}
	if rows[0].Arm != "fathom" || rows[1].Arm != "graphify" {
		t.Errorf("rows out of order or wrong: %+v", rows)
	}
	if rows[1].DroppedKinds["contains"] != 2 {
		t.Errorf("dropped_kinds not round-tripped: %+v", rows[1])
	}
}

func TestAppendCompetitorLedger_MultipleRowsOneCall(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "ledger.jsonl")
	rows := []CompetitorLedgerRow{
		{Arm: "fathom", Precision: 1, Recall: 1},
		{Arm: "serena", Precision: 0.8, Recall: 0.9},
	}
	if err := AppendCompetitorLedger(path, rows); err != nil {
		t.Fatalf("append: %v", err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	lines := 0
	for _, b := range data {
		if b == '\n' {
			lines++
		}
	}
	if lines != 2 {
		t.Fatalf("got %d lines, want 2", lines)
	}
}
