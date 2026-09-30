package intent_test

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/geoffmilleraz/fathom/internal/intent"
	"github.com/geoffmilleraz/fathom/internal/ledgerroots"
)

// TestIntentLedgerReadersIgnoreUnregisteredRoots pins PE-25: every reader
// of the intent ledger — the audit's debt and baseline, the kill clock,
// learning announcements, a condition's first-fired moment — reads only
// rows whose repo_root is a registered service root (PE-23's rule), names
// how many it ignored, keeps every row when the registry is unread, and
// never rewrites the file. The writers' dedup reads stay raw.
func TestIntentLedgerReadersIgnoreUnregisteredRoots(t *testing.T) {
	base := t.TempDir()
	reg := filepath.Join(base, "repos", "app")
	scratch := filepath.Join(base, "scratch")
	clone := filepath.Join(scratch, "clone")
	worktree := filepath.Join(reg, ".claude", "worktrees", "w1")
	other := filepath.Join(base, "elsewhere", "other")
	link := filepath.Join(base, "links", "app")
	for _, d := range []string{reg, clone, worktree, other, filepath.Dir(link)} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.Symlink(reg, link); err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	at := func(h int) string { return now.Add(-time.Duration(h) * time.Hour).Format(time.RFC3339) }
	rows := []intent.LedgerRow{
		{Kind: intent.KindClock, RepoRoot: reg, At: at(50), BaselineDays: []int{3, 5}, BaselineMedian: 4},
		{Kind: intent.KindDebt, RepoRoot: reg, At: at(40), ByKind: map[string]int{"U1": 2}},
		{Kind: intent.KindLearning, RepoRoot: link, At: at(30), ID: "c1", Verdict: "active"},
		{Kind: intent.KindOracle, RepoRoot: reg, At: at(20), ID: "F1#1", Verdict: "fired", Commit: "c2"},
		// Unregistered: a scratch clone's earlier firing, a worktree's debt,
		// another root's clock and learning.
		{Kind: intent.KindOracle, RepoRoot: clone, At: at(60), ID: "F1#1", Verdict: "fired", Commit: "c1"},
		{Kind: intent.KindDebt, RepoRoot: worktree, At: at(10), ByKind: map[string]int{"U1": 9}},
		{Kind: intent.KindClock, RepoRoot: other, At: at(45), BaselineNote: "none"},
		{Kind: intent.KindLearning, RepoRoot: other, At: at(5), ID: "c9", Verdict: "retired"},
	}
	path := filepath.Join(base, "intent-ledger.jsonl")
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	for _, r := range rows {
		r.SchemaVersion = intent.LedgerSchemaVersion
		if err := enc.Encode(r); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(path, buf.Bytes(), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("FATHOM_INTENT_LEDGER", path)
	roots := ledgerroots.RegisteredRootsWith([]string{reg, clone}, []string{scratch})

	kept, ign := intent.ReadRegisteredLedger(roots)
	if len(kept) != 4 || ign.Rows != 4 || ign.ScratchRows != 1 || len(ign.Roots) != 3 {
		t.Fatalf("kept %d, ignored %+v; want 4 kept, 4 ignored under 3 roots (1 scratch)", len(kept), ign)
	}
	if p := ign.Phrase("intent-ledger row"); p != "4 intent-ledger rows under 3 unregistered roots (1 under a scratch dir) ignored" {
		t.Fatalf("phrase %q", p)
	}

	// The audit's debt and baseline, the clock, learning, first-fired.
	if d, ok := intent.NewestDebt(kept, worktree); ok {
		t.Errorf("a worktree's debt row was read: %+v", d)
	}
	if d, ok := intent.NewestDebt(kept, reg); !ok || d.ByKind["U1"] != 2 {
		t.Errorf("the registered root's debt = %+v, %v", d, ok)
	}
	if _, ok := intent.Clock(kept, other); ok {
		t.Error("an unregistered root's clock was read")
	}
	if b, ok := intent.Baseline(kept, reg); !ok || b.BaselineMedian != 4 {
		t.Errorf("the registered root's baseline = %+v, %v", b, ok)
	}
	if l := intent.RecentLearning(kept, other, now.Add(-7*24*time.Hour)); len(l) != 0 {
		t.Errorf("an unregistered root's learning was announced: %+v", l)
	}
	if first, ok := intent.FirstSeen(kept, "F1#1", "fired"); !ok || first.Commit != "c2" {
		t.Errorf("first fired = %+v, %v; a scratch clone's earlier firing must not date it", first, ok)
	}
	if again, _ := intent.RegisteredRows(rows, roots); len(again) != 4 {
		t.Errorf("RegisteredRows over rows already read kept %d, want 4", len(again))
	}

	// An unread registry keeps every row and says why.
	all, uign := intent.ReadRegisteredLedger(ledgerroots.Unread("the daemon did not answer"))
	if len(all) != len(rows) || uign.Rows != 0 || !strings.Contains(uign.Phrase("intent-ledger row"), "the daemon did not answer") {
		t.Fatalf("unread: kept %d of %d, ignored %+v", len(all), len(rows), uign)
	}

	// The writers' dedup stays raw: a baseline for the other root exists in
	// the file, so RecordBaseline appends nothing for it.
	if err := intent.RecordBaseline(other, "c9", at(1), nil, 0, "again"); err != nil {
		t.Fatal(err)
	}
	after, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(after, buf.Bytes()) {
		t.Fatal("the intent ledger changed: a reader rewrote it, or a writer's dedup read the filtered rows")
	}
}
