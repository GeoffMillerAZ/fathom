package fusion_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/connector"
	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/fusion"
	"github.com/geoffmilleraz/fathom/internal/store"
)

func writeTrendFile(t *testing.T, root, rel, body string) {
	t.Helper()
	p := filepath.Join(root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

// TestViolationTrendAtIngest (CX-20, docs/design/change-and-compliance.md
// §5 step 2): each ingest of a repository with an archconformance rule pack
// records the violation count per deny rule, stamped with the ingest's
// commit; the reading names the previous reading's commit and the change,
// so the commits that added violations are named from stored rows. An
// unchanged re-ingest at the same commit records nothing; a repository with
// no rule pack records nothing (no rules is not zero violations).
func TestViolationTrendAtIngest(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	writeTrendFile(t, root, "a/a.go", "package a\n\nimport \"example.test/b\"\n\nfunc Run() { b.Do() }\n")
	writeTrendFile(t, root, "b/b.go", "package b\n\nfunc Do() {}\n\nfunc More() {}\n")
	writeTrendFile(t, root, "c/c.go", "package c\n\nimport \"example.test/b\"\n\nfunc Use() { b.Do() }\n")

	st := store.NewMemoryStore()
	svc := core.ServiceRef{ID: "trend-svc", Path: root}
	commit := "c1"
	p := fusion.New(connector.NewLocalConnector(), st, defaultCfg())
	p.WithGitHead(func(string) string { return commit })
	p.WithGitLog(func(string) string { return "" })

	// No rule pack: nothing is recorded.
	if _, err := p.Ingest(ctx, svc); err != nil {
		t.Fatalf("Ingest: %v", err)
	}
	if rs, err := st.ArchViolationTrend(ctx, svc.ID); err != nil || len(rs) != 0 {
		t.Fatalf("no rule pack: readings %+v, err %v; want none", rs, err)
	}

	writeTrendFile(t, root, ".fathom/rules/archconformance.cue",
		"rules: [\n\t{from_pkg: \"a\", to_pkg: \"b\", allow: false},\n\t{from_pkg: \"c\", to_pkg: \"a\", allow: false},\n]\n")
	commit = "c2"
	writeTrendFile(t, root, "a/doc.go", "package a\n")
	if _, err := p.Ingest(ctx, svc); err != nil {
		t.Fatalf("Ingest: %v", err)
	}
	rs, err := st.ArchViolationTrend(ctx, svc.ID)
	if err != nil {
		t.Fatal(err)
	}
	want := []store.ArchViolationReading{
		{Seq: 1, Commit: "c2", Rule: "a -> b", Count: 1},
		{Seq: 1, Commit: "c2", Rule: "c -> a", Count: 0},
	}
	assertReadings(t, "first reading", rs, want)

	// Unchanged re-ingest at the same commit: no new reading.
	if _, err := p.Ingest(ctx, svc); err != nil {
		t.Fatalf("Ingest: %v", err)
	}
	if rs2, _ := st.ArchViolationTrend(ctx, svc.ID); len(rs2) != len(want) {
		t.Fatalf("unchanged re-ingest recorded a reading: %+v", rs2)
	}

	// A commit that adds a violation: the reading names it and the delta.
	commit = "c3"
	writeTrendFile(t, root, "a/more.go", "package a\n\nimport \"example.test/b\"\n\nfunc Also() { b.More() }\n")
	if _, err := p.Ingest(ctx, svc); err != nil {
		t.Fatalf("Ingest: %v", err)
	}
	rs, _ = st.ArchViolationTrend(ctx, svc.ID)
	want = append(want,
		store.ArchViolationReading{Seq: 2, Commit: "c3", PrevCommit: "c2", Rule: "a -> b", Count: 2, Delta: 1},
		store.ArchViolationReading{Seq: 2, Commit: "c3", PrevCommit: "c2", Rule: "c -> a", Count: 0, Delta: 0},
	)
	assertReadings(t, "second reading", rs, want)

	var added []string
	for _, r := range rs {
		if r.Delta > 0 {
			added = append(added, r.Rule+"@"+r.PrevCommit+".."+r.Commit)
		}
	}
	if len(added) != 1 || added[0] != "a -> b@c2..c3" {
		t.Errorf("commits that added violations = %v, want [a -> b@c2..c3]", added)
	}
}

func assertReadings(t *testing.T, label string, got, want []store.ArchViolationReading) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("%s: got %d readings %+v, want %d %+v", label, len(got), got, len(want), want)
	}
	for i := range want {
		g, w := got[i], want[i]
		if g.RecordedAt == 0 {
			t.Errorf("%s: reading %d carries no time", label, i)
		}
		g.RecordedAt = 0
		if g != w {
			t.Errorf("%s: reading %d = %+v, want %+v", label, i, g, w)
		}
	}
}
