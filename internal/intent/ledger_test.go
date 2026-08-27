package intent_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/geoffmilleraz/fathom/internal/intent"
)

// TestIntentLedgerIsSeparateFromVerifyLedger (EP-6) is the important one.
// The verify ledger's readers — the flywheel's verify-gate loop,
// instrument --adoption, the SDD trend — decode EVERY line as a gate run
// and count it. An intent row landing in that file would decode as a
// zero-task verify and silently corrupt the program's headline instrument.
// Two kinds of record, two files, and this test proves the separation
// rather than asserting it in a comment.
func TestIntentLedgerIsSeparateFromVerifyLedger(t *testing.T) {
	dir := t.TempDir()
	intentPath := filepath.Join(dir, "intent-ledger.jsonl")
	verifyPath := filepath.Join(dir, "verify-ledger.jsonl")
	t.Setenv("FATHOM_INTENT_LEDGER", intentPath)

	// A verify-ledger row, as cmd/fathom writes them.
	if err := os.WriteFile(verifyPath,
		[]byte(`{"schema_version":1,"at":"2026-08-26T00:00:00Z","repo_root":"/r","service":"fathom","plan":"docs/plans/x.md","total":3,"claimed":3,"verified":3,"failing":0}`+"\n"),
		0o644); err != nil {
		t.Fatal(err)
	}

	if err := intent.AppendLedger(intent.LedgerRow{
		Kind: intent.KindOracle, RepoRoot: "/r", ID: "F9#1", Verdict: "fired", Commit: "abc1234",
	}); err != nil {
		t.Fatalf("AppendLedger: %v", err)
	}

	// The intent row went to the intent file …
	rows := intent.ReadLedger()
	if len(rows) != 1 || rows[0].ID != "F9#1" || rows[0].Kind != intent.KindOracle {
		t.Fatalf("intent ledger = %+v", rows)
	}
	if rows[0].SchemaVersion != intent.LedgerSchemaVersion {
		t.Errorf("row must carry its schema version, got %d", rows[0].SchemaVersion)
	}

	// … and the verify ledger is byte-for-byte untouched.
	after, err := os.ReadFile(verifyPath)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(after), "F9#1") || strings.Count(string(after), "\n") != 1 {
		t.Errorf("the verify ledger must never receive an intent row: %s", after)
	}

	// A verify-ledger reader decoding the intent file would see zeros — the
	// corruption this separation prevents. Prove the shape differs.
	var asVerify struct {
		Total   int `json:"total"`
		Claimed int `json:"claimed"`
		Plan    string
	}
	line, _ := os.ReadFile(intentPath)
	_ = json.Unmarshal([]byte(strings.SplitN(string(line), "\n", 2)[0]), &asVerify)
	if asVerify.Plan != "" || asVerify.Total != 0 {
		t.Errorf("an intent row must not look like a verify row: %+v", asVerify)
	}
}

// TestRecordTransitionDedupes: an oracle verdict is recorded the FIRST time
// it is seen at a commit, so an hourly audit does not fill the file while
// every real transition is still captured.
func TestRecordTransitionDedupes(t *testing.T) {
	t.Setenv("FATHOM_INTENT_LEDGER", filepath.Join(t.TempDir(), "l.jsonl"))
	row := intent.LedgerRow{RepoRoot: "/r", ID: "ADR-014#1", Verdict: "fired", Commit: "deadbee"}

	if wrote, err := intent.RecordTransition(intent.ReadLedger(), row); err != nil || !wrote {
		t.Fatalf("first transition must be recorded (wrote=%v err=%v)", wrote, err)
	}
	if wrote, _ := intent.RecordTransition(intent.ReadLedger(), row); wrote {
		t.Error("the same (id, verdict, commit) must not be recorded twice")
	}
	// A NEW commit is a new observation of the same verdict — recorded.
	row.Commit = "cafe123"
	if wrote, _ := intent.RecordTransition(intent.ReadLedger(), row); !wrote {
		t.Error("the same verdict at a new commit is a new transition")
	}
	// A different verdict is always new.
	row.Verdict = "holds"
	if wrote, _ := intent.RecordTransition(intent.ReadLedger(), row); !wrote {
		t.Error("a changed verdict must be recorded")
	}

	if _, ok := intent.FirstSeen(intent.ReadLedger(), "ADR-014#1", "fired"); !ok {
		t.Error("FirstSeen must find the earliest fired row")
	}
	if _, ok := intent.FirstSeen(intent.ReadLedger(), "nope#1", "fired"); ok {
		t.Error("FirstSeen must not invent a row")
	}
}

// TestIntentClockWrittenOnce (EP-12): law 6 — an activation step that
// leaves no stamp cannot be measured later. The stamp records the consumer
// population present when the clock started, and never moves.
func TestIntentClockWrittenOnce(t *testing.T) {
	t.Setenv("FATHOM_INTENT_LEDGER", filepath.Join(t.TempDir(), "l.jsonl"))

	if _, ok := intent.Clock(intent.ReadLedger(), "/r"); ok {
		t.Fatal("no clock should exist yet")
	}
	if err := intent.StartClock("/r", "abc1234", []string{"conjure", "fathom", "agentic-kit"}); err != nil {
		t.Fatalf("StartClock: %v", err)
	}
	first, ok := intent.Clock(intent.ReadLedger(), "/r")
	if !ok {
		t.Fatal("clock must be readable after StartClock")
	}
	if len(first.Population) != 3 || first.Population[0] != "agentic-kit" {
		t.Errorf("population must be recorded and sorted: %v", first.Population)
	}

	// A second call is a no-op: the clock's whole value is that it does not
	// move when someone re-runs the verb.
	if err := intent.StartClock("/r", "ffff999", []string{"fathom"}); err != nil {
		t.Fatal(err)
	}
	again, _ := intent.Clock(intent.ReadLedger(), "/r")
	if again.Commit != first.Commit || len(again.Population) != 3 {
		t.Errorf("the clock must not restart: %+v", again)
	}
}

// TestDebtDeltaNeedsAnOlderRow: "+k since" is only meaningful against a row
// from a previous session — two back-to-back audits must not report a
// triumphant +0.
func TestDebtDeltaNeedsAnOlderRow(t *testing.T) {
	t.Setenv("FATHOM_INTENT_LEDGER", filepath.Join(t.TempDir(), "l.jsonl"))
	now := time.Now().UTC()

	_ = intent.AppendLedger(intent.LedgerRow{
		Kind: intent.KindDebt, RepoRoot: "/r",
		At: now.Add(-2 * time.Minute).Format(time.RFC3339), ByKind: map[string]int{"U2": 12},
	})
	if _, ok := intent.DebtBefore(intent.ReadLedger(), "/r", now.Add(-24*time.Hour)); ok {
		t.Error("a row from two minutes ago is not a baseline for a session delta")
	}

	_ = intent.AppendLedger(intent.LedgerRow{
		Kind: intent.KindDebt, RepoRoot: "/r",
		At: now.Add(-48 * time.Hour).Format(time.RFC3339), ByKind: map[string]int{"U2": 15},
	})
	old, ok := intent.DebtBefore(intent.ReadLedger(), "/r", now.Add(-24*time.Hour))
	if !ok || old.ByKind["U2"] != 15 {
		t.Errorf("the newest row older than the cutoff is the baseline, got %+v", old)
	}
	if newest, ok := intent.NewestDebt(intent.ReadLedger(), "/r"); !ok || newest.ByKind["U2"] != 12 {
		t.Errorf("NewestDebt = %+v", newest)
	}
}
