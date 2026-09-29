package main

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/geoffmilleraz/fathom/internal/console"
)

// workRepo is a git repository whose history exercises /work's commit
// reads: c1 proves A-1 and is reworked by c2 a day later; c3 proves A-2
// and is reverted by c4 the next day; c5 touches c1's file after c1's
// 7-day window has closed.
type workRepo struct {
	root                   string
	base                   time.Time
	c1, c2, c3, c4, c5, c6 string
}

func newWorkRepo(t *testing.T) workRepo {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not on PATH")
	}
	r := workRepo{root: t.TempDir(), base: time.Date(2026, 9, 1, 10, 0, 0, 0, time.UTC)}
	write := func(name string, lines ...string) {
		p := filepath.Join(r.root, name)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(strings.Join(lines, "\n")+"\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	head := func() string {
		out, err := exec.Command("git", "-C", r.root, "rev-parse", "HEAD").Output()
		if err != nil {
			t.Fatal(err)
		}
		return strings.TrimSpace(string(out))
	}
	commit := func(at time.Time, msg string) string {
		gitAt(t, r.root, at, "add", "-A")
		gitAt(t, r.root, at, "commit", "-q", "-m", msg)
		return head()
	}
	gitAt(t, r.root, r.base, "init", "-q")
	write("internal/pay/a.go", "l1", "l2", "l3", "l4", "l5", "l6", "l7", "l8", "l9", "l10")
	write("web/b.go", "b1")
	r.c1 = commit(r.base, "pay: add a")
	write("internal/pay/a.go", "l1", "L2", "L3", "l4", "l5", "l6", "l7", "l8", "l9", "l10")
	r.c2 = commit(r.base.Add(24*time.Hour), "pay: rework a")
	write("web/c.go", "c1", "c2", "c3", "c4", "c5")
	r.c3 = commit(r.base.Add(48*time.Hour), "web: add c")
	gitAt(t, r.root, r.base.Add(72*time.Hour), "revert", "--no-edit", r.c3)
	r.c4 = head()
	write("internal/pay/a.go", "l1", "L2", "L3", "l4", "l5", "l6", "l7", "l8", "l9", "l10", "l11")
	r.c5 = commit(r.base.Add(10*24*time.Hour), "pay: later change")
	write("web/d.go", "d1")
	r.c6 = commit(r.base.Add(26*24*time.Hour), "web: add d, window still open at now")
	return r
}

// workRepoJoin is the join /work reads for newWorkRepo: A-1 proven at c1,
// A-2 proven at c3 (named abbreviated, as a ledger may) after one
// claimed-unproven reading, A-3 proven at c6, and A-4 at a commit this
// clone does not have.
func workRepoJoin(r workRepo) (console.JoinResult, console.History) {
	at := func(d int) string { return r.base.Add(time.Duration(d) * time.Hour).Format(time.RFC3339) }
	rows := []console.ProvenRow{
		{Service: "svc", Plan: "p.md", ID: "A-1", At: at(1), Commit: r.c1},
		{Service: "svc", Plan: "p.md", ID: "A-2", At: at(49), Commit: r.c3[:12]},
		{Service: "svc", Plan: "p.md", ID: "A-3", At: at(26*24 + 1), Commit: r.c6},
		{Service: "svc", Plan: "p.md", ID: "A-4", At: at(26*24 + 2), Commit: strings.Repeat("d", 40)},
	}
	h := console.History{}
	for _, pr := range rows {
		rh := console.RowHistory{Service: "svc", Plan: "p.md", ID: pr.ID, Readings: []console.RowReading{{At: at(0), State: console.HistOpen}}}
		if pr.ID == "A-2" {
			rh.Readings = append(rh.Readings, console.RowReading{At: at(40), State: console.HistUnproven})
		}
		rh.Readings = append(rh.Readings, console.RowReading{At: pr.At, Commit: pr.Commit, State: console.HistProven, Weight: 1})
		h.Rows = append(h.Rows, rh)
	}
	return console.JoinResult{Outcomes: []console.SessionOutcome{{SessionID: "s1", Rows: rows}}}, h
}

// WI-4w: /work reads rework within 7 days and the later commits that
// revert a proving commit with one read-only git log, beside the
// claimed-unproven retries its proof history carries; a part it could not
// read is named, never a zero.
func TestWorkReadsReworkAndReverts(t *testing.T) {
	r := newWorkRepo(t)
	join, h := workRepoJoin(r)
	now := r.base.Add(28 * 24 * time.Hour)
	facts, note := workCommitFacts(context.Background(), r.root, join, now)

	c1 := facts[r.c1]
	if !c1.ReworkKnown || c1.ReworkLines != 4 {
		t.Fatalf("c1's file was changed again by c2 within 7 days (2 added, 2 deleted); c5 is outside the window: %+v", c1)
	}
	if !c1.RevertsKnown || len(c1.Reverts) != 0 {
		t.Fatalf("nothing reverts c1, read as a measured zero: %+v", c1)
	}
	c3 := facts[r.c3[:12]]
	if !c3.ReworkKnown || c3.ReworkLines != 5 {
		t.Fatalf("the revert deleted c3's five lines within its window: %+v", c3)
	}
	if !c3.RevertsKnown || len(c3.Reverts) != 1 || c3.Reverts[0].Commit != r.c4 || c3.Reverts[0].At == "" {
		t.Fatalf("c4 reverts c3 (the abbreviated spelling the ledger used): %+v", c3)
	}
	if c6 := facts[r.c6]; c6.ReworkKnown || !strings.Contains(c6.ReworkReason, "still open") || !c6.RevertsKnown {
		t.Fatalf("c6's 7-day window is still open: rework unknown and named, reverts read: %+v", c6)
	}
	if _, ok := facts[strings.Repeat("d", 40)]; ok {
		t.Fatal("a commit this clone does not have is not read")
	}
	for _, want := range []string{"3 of 4 proving commits", "not in this clone", "git log", "1 revert"} {
		if !strings.Contains(note, want) {
			t.Errorf("the commits note names %q: %s", want, note)
		}
	}

	changes := console.EffortPerProvenChange(join, h, nil, facts, console.NewZones(nil, nil))
	byCommit := map[string]console.ProvenChange{}
	for _, c := range changes {
		byCommit[c.Commit] = c
	}
	if c := byCommit[r.c1]; !c.Rework.Known() || c.Rework.V != 4 || !c.Reverted.Known() || c.Reverted.V != 0 || c.Retries != 0 {
		t.Fatalf("A-1's change: rework 4, no revert, first attempt: %+v", c)
	}
	if c := byCommit[r.c3[:12]]; c.Rework.V != 4+1 || c.Reverted.V != 1 || c.Retries != 1 {
		t.Fatalf("A-2's change: rework 5, reverted once, one claimed-unproven retry: %+v", c)
	}
	if c := byCommit[r.c6]; c.Rework.Known() || !strings.Contains(c.Rework.Reason, "still open") {
		t.Fatalf("A-3's rework is unknown with its reason: %+v", c.Rework)
	}
	if c := byCommit[strings.Repeat("d", 40)]; c.Rework.Known() || c.Reverted.Known() || c.Reverted.Reason == "" {
		t.Fatalf("A-4's commit was not read: rework and reverts absent, named: %+v", c)
	}

	// A root git cannot read names the failure; nothing is a zero.
	bad, badNote := workCommitFacts(context.Background(), t.TempDir(), join, now)
	if len(bad) != 0 || !strings.Contains(badNote, "could not be read") {
		t.Fatalf("an unreadable repository is named: %v %q", bad, badNote)
	}
}
