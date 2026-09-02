package contextbench_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/contextbench"
)

func TestCorpusSHA256_IsTheFileHash(t *testing.T) {
	path := filepath.Join(t.TempDir(), "corpus.json")
	if err := os.WriteFile(path, []byte(`{"version":1,"tasks":[]}`), 0o644); err != nil {
		t.Fatal(err)
	}
	got, err := contextbench.CorpusSHA256(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 64 {
		t.Fatalf("want a 64-hex digest, got %q", got)
	}
	again, _ := contextbench.CorpusSHA256(path)
	if again != got {
		t.Fatal("hash must be deterministic")
	}
	if err := os.WriteFile(path, []byte(`{"version":1,"tasks":[] }`), 0o644); err != nil {
		t.Fatal(err)
	}
	changed, _ := contextbench.CorpusSHA256(path)
	if changed == got {
		t.Fatal("any byte change is a new corpus identity")
	}
	if _, err := contextbench.CorpusSHA256(filepath.Join(t.TempDir(), "absent")); err == nil {
		t.Fatal("a missing corpus must error, not hash to something")
	}
}

func TestLedgerRow_PB3FieldsAreAdditive(t *testing.T) {
	// A row written before per_task / corpus_sha256 / model_id / cost_usd /
	// dirty_tree existed must still parse, and a row that carries them
	// must round-trip them.
	path := filepath.Join(t.TempDir(), "ledger.jsonl")
	old := `{"run_id":"old","generated_at":"2026-09-01T00:00:00Z","commit":"abc","corpus":"c","model":"sonnet","treatment":"fathom","control":"grep","tasks":1,"min_trials":1,"treatment_successes":1,"control_successes":0,"treatment_trials":1,"control_trials":1,"aggregate_token_ratio":0.5,"median_token_ratio":0.5,"evaluation":{"criterion_id":"H2-1","verdict":"indeterminate","reason":"n=1"}}` + "\n"
	if err := os.WriteFile(path, []byte(old), 0o644); err != nil {
		t.Fatal(err)
	}
	r := row("new", contextbench.VerdictPass)
	r.PerTask = []contextbench.PairedTask{{TaskID: "T1", TreatmentTrials: 3, ControlTrials: 3, TreatmentSuccesses: 3, ControlSuccesses: 1}}
	r.CorpusSHA256 = strings.Repeat("a", 64)
	r.ModelID = "claude-sonnet-4-6"
	r.CostUSD = 12.5
	r.DirtyTree = true
	r.CellsLost = 2
	r.LeakedTasks = []string{"T9"}
	if err := contextbench.AppendLedger(path, r); err != nil {
		t.Fatal(err)
	}
	rows, err := contextbench.LoadLedger(path)
	if err != nil {
		t.Fatalf("an old row must still parse: %v", err)
	}
	if len(rows) != 2 || rows[0].CorpusSHA256 != "" || rows[0].PerTask != nil {
		t.Fatalf("old row must read as not-recorded, got %+v", rows[0])
	}
	got := rows[1]
	if got.CorpusSHA256 != r.CorpusSHA256 || got.ModelID != "claude-sonnet-4-6" || got.CostUSD != 12.5 || !got.DirtyTree || got.CellsLost != 2 || len(got.PerTask) != 1 || len(got.LeakedTasks) != 1 {
		t.Errorf("PB-3 fields did not round-trip: %+v", got)
	}
}

func TestBackfillLedgerRow_IsIndeterminateUnderTheFloorAndSaysSo(t *testing.T) {
	c := contextbench.Criterion{ID: "H2-1", RequireStrictlyMoreSuccesses: true, MaxTokenRatio: 1.0, MinTrials: 3}
	r := contextbench.BackfillLedgerRow(c, "20260724T170152Z", "2026-09-02T00:00:00Z", "bfb8010", "testdata/contextbench/tasks-home.json", "sonnet",
		13, 13, 11, 0.570, "backfilled from plan record, n=1, indeterminate")
	if r.Evaluation.Verdict != contextbench.VerdictIndeterminate {
		t.Fatalf("an n=1 run evaluated under a floor of 3 is indeterminate, got %s (%s)", r.Evaluation.Verdict, r.Evaluation.Reason)
	}
	if r.MinTrials != 1 || r.TreatmentSuccesses != 13 || r.ControlSuccesses != 11 || r.AggregateTokenRatio != 0.570 {
		t.Errorf("recorded numbers must be carried verbatim: %+v", r)
	}
	if r.CorpusSHA256 != "" || r.PerTask != nil {
		t.Error("a backfilled row has nothing to pair against — no corpus hash, no per-task data")
	}
	if !strings.Contains(r.Notes, "backfilled") {
		t.Error("notes must say the row was backfilled")
	}
}

// TestLedgerHasBackfilledRow pins PB-2: the committed ledger holds exactly
// one backfilled n=1 row, marked indeterminate, and nothing that claims
// more than the plan record it came from.
func TestLedgerHasBackfilledRow(t *testing.T) {
	rows, err := contextbench.LoadLedger("../../docs/bench/ledger.jsonl")
	if err != nil {
		t.Fatal(err)
	}
	var backfilled []contextbench.LedgerRow
	for _, r := range rows {
		if strings.Contains(r.Notes, "backfilled") {
			backfilled = append(backfilled, r)
		}
	}
	if len(backfilled) != 1 {
		t.Fatalf("want exactly one backfilled row, got %d", len(backfilled))
	}
	b := backfilled[0]
	if b.MinTrials != 1 || b.Evaluation.Verdict != contextbench.VerdictIndeterminate {
		t.Errorf("the backfilled row is an n=1 run and must read indeterminate under the floor: %+v", b)
	}
	if b.CorpusSHA256 != "" {
		t.Error("the backfilled row predates corpus hashing; a hash here would invite a false regression pairing")
	}
}
