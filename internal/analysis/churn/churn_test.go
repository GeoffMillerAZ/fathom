package churn_test

import (
	"math"
	"testing"
	"time"

	"github.com/geoffmilleraz/fathom/internal/analysis/churn"
)

// fixed "now" for deterministic tests.
var epoch = time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)

// ─── Hotspots ────────────────────────────────────────────────────────────────

func TestHotspots_Basic(t *testing.T) {
	// b.go: 100 lines changed 6 months ago (weight ≈ 0.5, score ≈ 50).
	// a.go: 18 lines changed 1-2 days ago (weight ≈ 1.0, score ≈ 18).
	// b.go ranks first due to higher absolute churn despite older age.
	commits := []churn.Commit{
		{File: "a.go", Additions: 10, Deletions: 2, Author: "alice", When: epoch.AddDate(0, 0, -1)},
		{File: "a.go", Additions: 5, Deletions: 1, Author: "bob", When: epoch.AddDate(0, 0, -2)},
		{File: "b.go", Additions: 100, Deletions: 0, Author: "carol", When: epoch.AddDate(0, -6, 0)},
	}

	hs := churn.Hotspots(commits, epoch)
	if len(hs) != 2 {
		t.Fatalf("expected 2 hotspots, got %d", len(hs))
	}

	// b.go has much higher churn so even with decay it outscores a.go.
	if hs[0].File != "b.go" {
		t.Errorf("expected b.go first (highest weighted score), got %s", hs[0].File)
	}
	if hs[1].File != "a.go" {
		t.Errorf("expected a.go second, got %s", hs[1].File)
	}
}

func TestHotspots_CommitCount(t *testing.T) {
	commits := []churn.Commit{
		{File: "x.go", Additions: 1, Deletions: 0, Author: "alice", When: epoch},
		{File: "x.go", Additions: 1, Deletions: 0, Author: "alice", When: epoch},
		{File: "x.go", Additions: 1, Deletions: 0, Author: "bob", When: epoch},
	}
	hs := churn.Hotspots(commits, epoch)
	if len(hs) != 1 {
		t.Fatalf("expected 1 hotspot")
	}
	if hs[0].Commits != 3 {
		t.Errorf("expected 3 commits, got %d", hs[0].Commits)
	}
	if hs[0].Authors != 2 {
		t.Errorf("expected 2 authors, got %d", hs[0].Authors)
	}
	if hs[0].Churn != 3 {
		t.Errorf("expected churn 3, got %d", hs[0].Churn)
	}
}

func TestHotspots_ScoreDecaysWithAge(t *testing.T) {
	// Two files with equal churn but different ages; recent should score higher.
	recent := churn.Commit{File: "new.go", Additions: 50, Deletions: 50, Author: "a", When: epoch.AddDate(0, 0, -1)}
	old := churn.Commit{File: "old.go", Additions: 50, Deletions: 50, Author: "a", When: epoch.AddDate(-2, 0, 0)}
	hs := churn.Hotspots([]churn.Commit{recent, old}, epoch)
	if hs[0].File != "new.go" {
		t.Errorf("expected new.go to rank first, got %s", hs[0].File)
	}
	if hs[0].Score <= hs[1].Score {
		t.Errorf("recent score (%f) should exceed old score (%f)", hs[0].Score, hs[1].Score)
	}
}

func TestHotspots_FutureDateClampedToZeroAge(t *testing.T) {
	// When commit is in the "future" relative to now, age is clamped to 0.
	c := churn.Commit{File: "future.go", Additions: 10, Deletions: 0, Author: "a", When: epoch.AddDate(0, 0, 10)}
	hs := churn.Hotspots([]churn.Commit{c}, epoch)
	if len(hs) != 1 {
		t.Fatalf("expected 1 hotspot")
	}
	// weight at age=0 is exp(0)=1, so score == churn.
	expected := 10.0
	if math.Abs(hs[0].Score-expected) > 1e-6 {
		t.Errorf("expected score %f, got %f", expected, hs[0].Score)
	}
}

func TestHotspots_Empty(t *testing.T) {
	hs := churn.Hotspots(nil, epoch)
	if len(hs) != 0 {
		t.Errorf("expected empty slice, got %v", hs)
	}
}

func TestHotspots_TieBrokenByFile(t *testing.T) {
	// Same score → sorted by File name.
	c1 := churn.Commit{File: "z.go", Additions: 5, Deletions: 0, Author: "a", When: epoch}
	c2 := churn.Commit{File: "a.go", Additions: 5, Deletions: 0, Author: "a", When: epoch}
	hs := churn.Hotspots([]churn.Commit{c1, c2}, epoch)
	if hs[0].File != "a.go" {
		t.Errorf("expected a.go first on tie, got %s", hs[0].File)
	}
}

func TestHotspots_Determinism(t *testing.T) {
	commits := []churn.Commit{
		{File: "a.go", Additions: 10, Deletions: 2, Author: "alice", When: epoch.AddDate(0, 0, -1)},
		{File: "b.go", Additions: 5, Deletions: 1, Author: "bob", When: epoch.AddDate(0, 0, -2)},
	}
	a := churn.Hotspots(commits, epoch)
	b := churn.Hotspots(commits, epoch)
	if len(a) != len(b) {
		t.Fatalf("non-deterministic length")
	}
	for i := range a {
		if a[i] != b[i] {
			t.Errorf("hotspot[%d] differs: %+v vs %+v", i, a[i], b[i])
		}
	}
}

// ─── ParseGitLog ─────────────────────────────────────────────────────────────

const sampleGitLog = `author:alice@example.com
date:2024-01-01T10:00:00+00:00
10	2	internal/foo/foo.go
5	0	internal/foo/bar.go

author:bob@example.com
date:2023-07-01T08:00:00+00:00
0	3	internal/foo/foo.go
-	-	binary.bin
`

func TestParseGitLog_Basic(t *testing.T) {
	commits := churn.ParseGitLog([]byte(sampleGitLog))
	// Expected: 3 parseable numstat lines (binary skipped).
	if len(commits) != 3 {
		t.Fatalf("expected 3 commits, got %d: %+v", len(commits), commits)
	}
}

func TestParseGitLog_Authors(t *testing.T) {
	commits := churn.ParseGitLog([]byte(sampleGitLog))
	if commits[0].Author != "alice@example.com" {
		t.Errorf("expected alice, got %s", commits[0].Author)
	}
	if commits[2].Author != "bob@example.com" {
		t.Errorf("expected bob, got %s", commits[2].Author)
	}
}

func TestParseGitLog_Counts(t *testing.T) {
	commits := churn.ParseGitLog([]byte(sampleGitLog))
	if commits[0].Additions != 10 || commits[0].Deletions != 2 {
		t.Errorf("unexpected counts: %+v", commits[0])
	}
}

func TestParseGitLog_Dates(t *testing.T) {
	commits := churn.ParseGitLog([]byte(sampleGitLog))
	want := time.Date(2024, 1, 1, 10, 0, 0, 0, time.UTC)
	if !commits[0].When.Equal(want) {
		t.Errorf("expected %v, got %v", want, commits[0].When)
	}
}

func TestParseGitLog_BinarySkipped(t *testing.T) {
	commits := churn.ParseGitLog([]byte(sampleGitLog))
	for _, c := range commits {
		if c.File == "binary.bin" {
			t.Error("binary file should be skipped")
		}
	}
}

func TestParseGitLog_EmptyInput(t *testing.T) {
	commits := churn.ParseGitLog([]byte{})
	if len(commits) != 0 {
		t.Errorf("expected empty slice for empty input")
	}
}

func TestParseGitLog_Determinism(t *testing.T) {
	a := churn.ParseGitLog([]byte(sampleGitLog))
	b := churn.ParseGitLog([]byte(sampleGitLog))
	if len(a) != len(b) {
		t.Fatalf("non-deterministic length")
	}
	for i := range a {
		if a[i] != b[i] {
			t.Errorf("commit[%d] differs", i)
		}
	}
}

// ─── Round-trip: parse → hotspots ────────────────────────────────────────────

func TestParseAndHotspots_RoundTrip(t *testing.T) {
	commits := churn.ParseGitLog([]byte(sampleGitLog))
	now := time.Date(2024, 2, 1, 0, 0, 0, 0, time.UTC)
	hs := churn.Hotspots(commits, now)
	if len(hs) == 0 {
		t.Fatal("expected at least one hotspot")
	}
	// foo.go was touched twice; should appear once in hotspots.
	found := false
	for _, h := range hs {
		if h.File == "internal/foo/foo.go" {
			found = true
			if h.Commits != 2 {
				t.Errorf("expected 2 commits for foo.go, got %d", h.Commits)
			}
			if h.Authors != 2 {
				t.Errorf("expected 2 authors for foo.go, got %d", h.Authors)
			}
		}
	}
	if !found {
		t.Error("internal/foo/foo.go not found in hotspots")
	}
}
