package support

import (
	"os"
	"path/filepath"
	"testing"
)

// TestSupportLedgerAppendOnlySorted proves SM-8: every AppendLedger call
// adds one row without dropping any earlier one, and the file is rewritten
// sorted by (reading_date, kind, id, capability) after every write —
// whatever order the calls themselves arrived in.
func TestSupportLedgerAppendOnlySorted(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "support-ledger.jsonl")

	// Deliberately out of chronological order, mirroring two independent
	// callers (indexbench lang-oracle, fathom harness verify) writing
	// whenever their own reading lands.
	rows := []LedgerRow{
		{Kind: KindLanguage, ID: "typescript", Capability: "L1", LevelMet: L1, Grade: GradeGood, ReadingDate: "2026-09-27", Commit: "86d4c5e2"},
		{Kind: KindClient, ID: "claude-code", Capability: "mcp-tools", Grade: GradeExcellent, ReadingDate: "2026-09-25", Commit: "7f7752b1"},
		{Kind: KindLanguage, ID: "javascript", Capability: "L1", LevelMet: L1, Grade: GradeGood, ReadingDate: "2026-09-26", Commit: "92b13169"},
	}
	for i, r := range rows {
		if err := AppendLedger(path, r); err != nil {
			t.Fatalf("AppendLedger(#%d): %v", i, err)
		}
	}

	got, err := readLedger(path)
	if err != nil {
		t.Fatalf("readLedger: %v", err)
	}
	if len(got) != len(rows) {
		t.Fatalf("ledger has %d rows, want %d (append-only: none dropped)", len(got), len(rows))
	}

	wantDates := []string{"2026-09-25", "2026-09-26", "2026-09-27"}
	for i, r := range got {
		if r.ReadingDate != wantDates[i] {
			t.Fatalf("row %d date = %s, want %s — ledger is not sorted by date", i, r.ReadingDate, wantDates[i])
		}
	}

	// A fourth row on the SAME earliest date sorts by kind/id/capability
	// next, never by arrival order.
	if err := AppendLedger(path, LedgerRow{Kind: KindClient, ID: "claude-code", Capability: "obligations-post-edit", Grade: GradeGood, ReadingDate: "2026-09-25", Commit: "7f7752b1"}); err != nil {
		t.Fatalf("AppendLedger(#4): %v", err)
	}
	got, err = readLedger(path)
	if err != nil {
		t.Fatalf("readLedger: %v", err)
	}
	if len(got) != 4 {
		t.Fatalf("ledger has %d rows, want 4", len(got))
	}
	if got[0].Capability != "mcp-tools" || got[1].Capability != "obligations-post-edit" {
		t.Fatalf("same-date rows not sorted by capability: got %s then %s", got[0].Capability, got[1].Capability)
	}

	// A malformed row (a grade outside the closed vocabulary) is refused
	// and never reaches disk.
	before, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	err = AppendLedger(path, LedgerRow{Kind: KindRules, ID: "cursor-rules", Capability: "read", Grade: "great", ReadingDate: "2026-09-27", Commit: "86d4c5e2"})
	if err == nil {
		t.Fatal("expected AppendLedger to refuse an out-of-vocabulary grade")
	}
	after, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(before) != string(after) {
		t.Fatal("a rejected row still modified the ledger file")
	}
}
