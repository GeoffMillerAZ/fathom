package console

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/geoffmilleraz/fathom/internal/intent"
	readmanifest "github.com/geoffmilleraz/fathom/third_party/agentic-kit/foundations/telemetry/read-manifest"
)

// TestIntentView_UnreadableCorpusIsNeverAbsent pins I38 on the intent
// tab's projection: a repository with no router and no decision records has
// no corpus (absent — a choice, not a defect), but one whose router EXISTS
// and cannot be read is an error naming the file, never "no corpus"; a
// corpus read in part is a partial. The corpus's own reads decide it (a
// source outside the corpus that could not be read makes the counts a lower
// bound, not the corpus missing).
func TestIntentView_UnreadableCorpusIsNeverAbsent(t *testing.T) {
	root := t.TempDir()
	now := time.Date(2026, 9, 26, 4, 0, 0, 0, time.UTC)

	none := IntentView(intent.Corpus{Root: root, Tier: intent.TierNone}, intent.Audit{Root: root}, nil, now)
	if none.Corpus.State != StateAbsent || !strings.Contains(none.Corpus.Reason, "a choice, not a defect") {
		t.Errorf("no corpus at all = %+v, want absent (a choice)", none.Corpus)
	}

	router := filepath.Join(root, "docs", "intent", "README.md")
	if err := os.MkdirAll(filepath.Dir(router), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(router, []byte("# router\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(router, 0); err != nil || os.Getuid() == 0 {
		t.Skip("cannot make a file unreadable here")
	}
	t.Cleanup(func() { _ = os.Chmod(router, 0o644) })
	var reads readmanifest.Set
	_, _ = reads.ReadFile(router)
	cov := reads.Manifest()
	blind := IntentView(intent.Corpus{Root: root, Tier: intent.TierNone, Coverage: cov}, intent.Audit{Root: root, Coverage: cov}, nil, now)
	if blind.Corpus.State != StateError {
		t.Fatalf("an unreadable router = %+v, want an error — never absent", blind.Corpus)
	}
	if strings.Contains(blind.Corpus.Reason, "a choice, not a defect") || !strings.Contains(blind.Corpus.Reason, "README.md") || !strings.Contains(blind.Corpus.Reason, "denied") {
		t.Errorf("the error names the unreadable file and its errno: %q", blind.Corpus.Reason)
	}

	partial := IntentView(intent.Corpus{Root: root, Tier: intent.TierDecisionsOnly, Coverage: cov}, intent.Audit{Root: root, Coverage: cov}, nil, now)
	if partial.Corpus.State != StatePartial || partial.Corpus.V != string(intent.TierDecisionsOnly) || !strings.Contains(partial.Corpus.Reason, "README.md") {
		t.Errorf("a corpus read in part = %+v, want a partial naming what could not be read", partial.Corpus)
	}
	whole := IntentView(intent.Corpus{Root: root, Tier: intent.TierRouter}, intent.Audit{Root: root}, nil, now)
	if whole.Corpus.State != StatePresent || whole.Corpus.V != string(intent.TierRouter) {
		t.Errorf("a corpus read whole = %+v", whole.Corpus)
	}
}

// TestIntentView_FuturesFamilyStaysFutures (K8): a future's condition read
// from a member of the split futures.md is still a future on the intent
// tab, never "other".
func TestIntentView_FuturesFamilyStaysFutures(t *testing.T) {
	now := time.Date(2026, 10, 9, 4, 0, 0, 0, time.UTC)
	c := intent.Corpus{Root: t.TempDir(), Tier: intent.TierRouter, Conditions: []intent.Condition{
		{ID: "F1#1", Record: "F1", Label: "Promote when", Text: "a", File: "docs/intent/futures.md", Line: 9},
		{ID: "F2#1", Record: "F2", Label: "Promote when", Text: "b", File: "docs/intent/futures/f01-f09.md", Line: 4},
		{ID: "G2#1", Record: "G2", Label: "Done when", Text: "c", File: "docs/intent/scope/goals-1.md", Line: 3},
	}}
	m := IntentView(c, intent.Audit{Root: c.Root}, nil, now)
	if len(m.Futures) != 2 || len(m.Other) != 1 || m.Other[0].ID != "G2#1" {
		t.Errorf("futures = %+v, other = %+v: want both futures' conditions as futures", m.Futures, m.Other)
	}
}
