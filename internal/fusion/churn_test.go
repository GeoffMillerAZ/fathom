package fusion_test

import (
	"context"
	"testing"
	"time"

	"github.com/geoffmilleraz/fathom/internal/connector"
	"github.com/geoffmilleraz/fathom/internal/fusion"
	"github.com/geoffmilleraz/fathom/internal/store"
)

// TestFileChurnAtIngest proves TQ-1 (docs/plans/test-intelligence.md;
// design doc docs/design/test-intelligence.md §3 T1): churn per file —
// commits touching it in the 180 days before the ingest's own clock — is
// computed once, at ingest, through the injected git log (the same
// WithGitLog → parseGitLog → SetCommits path gitlog_pipeline_test.go
// exercises), never re-walked at request time, and stored so a later read
// (store.FileChurn) is a plain lookup. Commit dates are built relative to
// "now" at test time (internal/CLAUDE.md determinism: never mix a fixed
// fixture date with the real clock), so the test never crosses the 180-day
// horizon as calendar time passes.
func TestFileChurnAtIngest(t *testing.T) {
	now := time.Now().UTC()
	iso := func(daysAgo int) string { return now.AddDate(0, 0, -daysAgo).Format(time.RFC3339) }
	rec := func(hash string, daysAgo int, files ...string) string {
		s := "\x00" + hash + "\x1fAlice\x1f" + iso(daysAgo) + "\x00commit message\x00\n"
		for _, f := range files {
			s += f + "\n"
		}
		return s + "\n"
	}
	rawLog := rec("aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", 10, "pkg/x.go", "pkg/y.go") +
		rec("bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb", 100, "pkg/x.go") +
		rec("cccccccccccccccccccccccccccccccccccccccc", 200, "pkg/x.go", "pkg/z.go")

	conn := connector.NewFauxConnector()
	st := store.NewMemoryStore()
	p := fusion.New(conn, st, defaultCfg())
	p.WithGitLog(func(_ string) string { return rawLog })

	ctx := context.Background()
	if _, err := p.Ingest(ctx, paymentSvc); err != nil {
		t.Fatalf("Ingest error: %v", err)
	}

	churn, err := st.FileChurn(ctx, paymentSvc.ID)
	if err != nil {
		t.Fatalf("FileChurn: %v", err)
	}
	got := map[string]int{}
	for _, c := range churn {
		got[c.Path] = c.Commits180d
	}
	want := map[string]int{"pkg/x.go": 2, "pkg/y.go": 1}
	if len(got) != len(want) {
		t.Fatalf("FileChurn = %+v, want exactly %+v (the >180d-old commit's files must be excluded)", got, want)
	}
	for path, n := range want {
		if got[path] != n {
			t.Errorf("FileChurn[%s] = %d, want %d", path, got[path], n)
		}
	}
}

// TestFileChurnAtIngest_ReplacesOnReingest proves SetFileChurn's replace
// contract (mirroring SetCommits): a second ingest with a different git log
// replaces the prior churn set wholesale rather than accumulating counts
// across ingests.
func TestFileChurnAtIngest_ReplacesOnReingest(t *testing.T) {
	now := time.Now().UTC()
	iso := func(daysAgo int) string { return now.AddDate(0, 0, -daysAgo).Format(time.RFC3339) }
	rec := func(hash string, daysAgo int, files ...string) string {
		s := "\x00" + hash + "\x1fAlice\x1f" + iso(daysAgo) + "\x00commit message\x00\n"
		for _, f := range files {
			s += f + "\n"
		}
		return s + "\n"
	}

	conn := connector.NewFauxConnector()
	st := store.NewMemoryStore()
	p := fusion.New(conn, st, defaultCfg())
	ctx := context.Background()

	p.WithGitLog(func(_ string) string {
		return rec("aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", 5, "pkg/x.go")
	})
	if _, err := p.Ingest(ctx, paymentSvc); err != nil {
		t.Fatalf("first Ingest error: %v", err)
	}

	p.WithGitLog(func(_ string) string {
		return rec("bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb", 5, "pkg/y.go")
	})
	if _, err := p.Ingest(ctx, paymentSvc); err != nil {
		t.Fatalf("second Ingest error: %v", err)
	}

	churn, err := st.FileChurn(ctx, paymentSvc.ID)
	if err != nil {
		t.Fatalf("FileChurn: %v", err)
	}
	got := map[string]int{}
	for _, c := range churn {
		got[c.Path] = c.Commits180d
	}
	if _, ok := got["pkg/x.go"]; ok {
		t.Errorf("pkg/x.go from the first ingest's commit should be gone after re-ingest, got %+v", got)
	}
	if got["pkg/y.go"] != 1 {
		t.Errorf("pkg/y.go = %d, want 1", got["pkg/y.go"])
	}
}
