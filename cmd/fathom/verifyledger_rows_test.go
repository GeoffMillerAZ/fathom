package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/verify"
)

// PE-1 (docs/design/progress-and-economics.md §3): schema 3 carries a
// per-row verdict array, written only when the plan's row digest changed
// since that plan's previous reading; every row carries the digest; v1 and
// v2 rows still parse.
func TestVerifyLedgerSchema3RowsOnDigestChange(t *testing.T) {
	p := filepath.Join(t.TempDir(), "verify-ledger.jsonl")
	t.Setenv("FATHOM_VERIFY_LEDGER", p)

	// A v1 and a v2 row already in the ledger still parse.
	legacy := `{"schema_version":1,"at":"2026-09-01T00:00:00Z","plan":"docs/plans/x.md","total":2,"claimed":1,"verified":1}` + "\n" +
		`{"schema_version":2,"at":"2026-09-02T00:00:00Z","plan":"docs/plans/x.md","total":2,"claimed":1,"verified":1,"origin":"cli"}` + "\n"
	if err := os.WriteFile(p, []byte(legacy), 0o644); err != nil {
		t.Fatal(err)
	}

	rep := verify.Report{Tasks: []verify.TaskVerdict{
		{TaskID: "T-1", Claimed: true, Status: verify.Pass, Weight: 3},
		{TaskID: "T-2", Claimed: true, Status: verify.Fail, Criteria: []verify.Criterion{
			{Name: "symbol_present", Status: verify.Pass},
			{Name: "test_reaches", Status: verify.Fail, Detail: "no test reaches Foo"},
		}},
		{TaskID: "T-3"},
		{TaskID: "T-4", Claimed: true, Status: verify.Unknown},
		{TaskID: "T-5", Skipped: true},
	}}
	plans := []string{"docs/plans/x.md"}
	appendVerifyLedger(plans, "svc", []verify.Report{rep}, verifyProvenance{})
	appendVerifyLedger(plans, "svc", []verify.Report{rep}, verifyProvenance{}) // unchanged tree

	rows := readVerifyLedger()
	if len(rows) != 4 {
		t.Fatalf("want 4 ledger rows (2 legacy + 2 new), got %d", len(rows))
	}
	if rows[0].SchemaVersion != 1 || rows[1].SchemaVersion != 2 || rows[0].Total != 2 || rows[1].Rows != nil {
		t.Fatalf("legacy rows must still parse with counts only: %+v %+v", rows[0], rows[1])
	}
	first, second := rows[2], rows[3]
	if first.SchemaVersion != 3 || first.Digest == "" {
		t.Fatalf("new rows are schema 3 with a digest: %+v", first)
	}
	want := map[string]verifyLedgerRowVerdict{
		"T-1": {ID: "T-1", State: "claimed-proven", Weight: 3},
		"T-2": {ID: "T-2", State: "claimed-unproven", Criterion: "test_reaches: no test reaches Foo", Weight: 1},
		"T-3": {ID: "T-3", State: "open", Weight: 1},
		"T-4": {ID: "T-4", State: "unknown", Weight: 1},
		"T-5": {ID: "T-5", State: "dispositioned", Weight: 1},
	}
	if len(first.Rows) != len(want) {
		t.Fatalf("first reading carries every row, got %+v", first.Rows)
	}
	for _, r := range first.Rows {
		if r != want[r.ID] {
			t.Errorf("row %s = %+v, want %+v", r.ID, r, want[r.ID])
		}
	}
	if second.Digest != first.Digest || second.Rows != nil {
		t.Fatalf("an unchanged digest appends counts without the rows array: %+v", second)
	}
	if second.Total != 5 || second.Claimed != 3 {
		t.Fatalf("counts are still written: %+v", second)
	}

	// A changed verdict changes the digest and writes the array again.
	rep.Tasks[1].Status = verify.Pass
	rep.Tasks[1].Criteria = nil
	appendVerifyLedger(plans, "svc", []verify.Report{rep}, verifyProvenance{})
	rows = readVerifyLedger()
	third := rows[len(rows)-1]
	if third.Digest == first.Digest || len(third.Rows) != 5 {
		t.Fatalf("a changed plan writes a new digest and the array: %+v", third)
	}
	data, _ := os.ReadFile(p)
	if !strings.Contains(string(data), `"rows":[`) {
		t.Fatalf("rows array not on disk: %s", data)
	}
}
