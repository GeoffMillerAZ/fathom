package contextbench_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/contextbench"
)

func row(id string, v contextbench.Verdict) contextbench.LedgerRow {
	return contextbench.LedgerRow{
		RunID:       id,
		GeneratedAt: "2026-09-01T00:00:00Z",
		Commit:      "abc1234",
		Corpus:      "testdata/contextbench/tasks-home.json",
		Model:       "sonnet",
		Treatment:   "fathom",
		Control:     "grep",
		MinTrials:   3,
		Evaluation:  contextbench.Evaluation{CriterionID: "H2-1", Verdict: v, Reason: "because"},
	}
}

func TestLedger_AppendThenLoadRoundTrips(t *testing.T) {
	path := filepath.Join(t.TempDir(), "ledger.jsonl")
	for _, r := range []contextbench.LedgerRow{row("r1", contextbench.VerdictFail), row("r2", contextbench.VerdictPass)} {
		if err := contextbench.AppendLedger(path, r); err != nil {
			t.Fatalf("AppendLedger: %v", err)
		}
	}
	got, err := contextbench.LoadLedger(path)
	if err != nil {
		t.Fatalf("LoadLedger: %v", err)
	}
	if len(got) != 2 || got[0].RunID != "r1" || got[1].RunID != "r2" {
		t.Fatalf("round trip lost or reordered rows: %+v", got)
	}
	if got[1].Evaluation.Verdict != contextbench.VerdictPass {
		t.Errorf("verdict = %q, want pass", got[1].Evaluation.Verdict)
	}
}

func TestLedger_MissingFileLoadsEmptyNotError(t *testing.T) {
	got, err := contextbench.LoadLedger(filepath.Join(t.TempDir(), "absent.jsonl"))
	if err != nil {
		t.Fatalf("a ledger that does not exist yet is empty, not an error: %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("want no rows, got %d", len(got))
	}
}

func TestLedger_CorruptLineFailsLoudlyAndNamesTheLine(t *testing.T) {
	// A ledger that silently skips what it cannot parse would report a
	// shorter, healthier-looking history than the truth (ADR-027).
	path := filepath.Join(t.TempDir(), "ledger.jsonl")
	if err := contextbench.AppendLedger(path, row("r1", contextbench.VerdictPass)); err != nil {
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

	if _, err := contextbench.LoadLedger(path); err == nil {
		t.Fatal("want an error for an unparseable ledger line, got nil")
	} else if !strings.Contains(err.Error(), "line 2") {
		t.Errorf("error should name the offending line, got: %v", err)
	}
}

func TestRenderLedgerMarkdown_ShowsVerdictAndTrialsNewestLast(t *testing.T) {
	md := contextbench.RenderLedgerMarkdown([]contextbench.LedgerRow{
		row("r1", contextbench.VerdictFail),
		row("r2", contextbench.VerdictIndeterminate),
	})
	if !strings.Contains(md, "fail") || !strings.Contains(md, "indeterminate") {
		t.Fatalf("both verdicts must appear verbatim:\n%s", md)
	}
	if strings.Index(md, "r1") > strings.Index(md, "r2") {
		t.Error("rows render in ledger order (append order), oldest first")
	}
}
