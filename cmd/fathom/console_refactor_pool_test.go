package main

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
)

// TestRefactorReportBoundedPool (RT-118, design §14): /refactors computes
// conserve through the console's one job pool — a range waits in its queue
// while the workers are busy, and asking again for the same range joins the
// flight instead of starting another — refuses a diff wider than
// conserve_max_files by name instead of running conserve over it, and runs
// git under a deadline: a git that hangs is killed and named, never waited
// on.
func TestRefactorReportBoundedPool(t *testing.T) {
	f := newConsoleFixture(t)
	root := t.TempDir()
	gitInitTempRepo(t, root)
	runGitFootprint(t, root, "config", "user.email", "t@example.com")
	runGitFootprint(t, root, "config", "user.name", "t")
	mustWrite(t, filepath.Join(root, "a.go"), "package a\n")
	runGitFootprint(t, root, "add", ".")
	runGitFootprint(t, root, "commit", "-qm", "base")
	for i := 0; i < 5; i++ {
		mustWrite(t, filepath.Join(root, "f"+strconv.Itoa(i)+".go"), "package a\n")
	}
	runGitFootprint(t, root, "add", ".")
	runGitFootprint(t, root, "commit", "-qm", "wide")
	sr := serviceRoot{Service: "proof-service", Root: root}
	pc := plansFor(f.deps)
	resetConserveCache()
	t.Cleanup(resetConserveCache)

	// The pool's two workers are busy: the range is queued, not started.
	release := make(chan struct{})
	pool := f.deps.jobPool()
	for i := 0; i < consoleJobWorkers; i++ {
		if _, err := pool.SubmitJob("", "blocker-"+strconv.Itoa(i), func() { <-release }); err != nil {
			t.Fatal(err)
		}
	}
	waitUntil(t, func() bool { return pool.Stats().Running == consoleJobWorkers })
	prevMax := consoleConserveMaxFiles
	consoleConserveMaxFiles = 3
	t.Cleanup(func() { consoleConserveMaxFiles = prevMax })
	if _, ok := refactorReport(pc, sr, "HEAD~1", 20*time.Millisecond); ok {
		t.Fatal("a range computed while every worker was busy: it did not go through the pool")
	}
	if _, ok := refactorReport(pc, sr, "HEAD~1", 20*time.Millisecond); ok {
		t.Fatal("still pending")
	}
	if st := pool.Stats(); st.Queued != 1 {
		t.Fatalf("pool = %+v; want the range queued once (a second ask joins its flight)", st)
	}
	close(release)
	e, ok := refactorReport(pc, sr, "HEAD~1", 10*time.Second)
	if !ok {
		t.Fatal("the range never finished")
	}
	if !strings.Contains(e.err, "conserve_max_files") || !strings.Contains(e.err, "5 changed files") {
		t.Fatalf("a diff over the cap = %q; want the refusal naming conserve_max_files and the 5 changed files", e.err)
	}

	// A git that hangs is killed at the deadline and named.
	bin := t.TempDir()
	mustWrite(t, filepath.Join(bin, "git"), "#!/bin/sh\nexec sleep 30\n")
	if err := os.Chmod(filepath.Join(bin, "git"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	prevGit := consoleGitTimeout
	consoleGitTimeout = 200 * time.Millisecond
	t.Cleanup(func() { consoleGitTimeout = prevGit })
	start := time.Now()
	e, ok = refactorReport(pc, serviceRoot{Service: "proof-service", Root: t.TempDir()}, "HEAD~1", 10*time.Second)
	if took := time.Since(start); took > 5*time.Second {
		t.Fatalf("a hung git held the page %s", took)
	}
	if !ok || !strings.Contains(e.err, "git") {
		t.Fatalf("a hung git = ok %v err %q; want a named git failure", ok, e.err)
	}
}

// TestPlanGitHistoryHonoursItsContext (RT-118): the plan history panel's
// git reads run under the page's context — a git that does not answer in
// time is stopped and named, and the console does not keep that answer.
func TestPlanGitHistoryHonoursItsContext(t *testing.T) {
	root := t.TempDir()
	gitInitTempRepo(t, root)
	mustWrite(t, filepath.Join(root, "docs", "plans", "p.md"), "# P\n")
	runGitFootprint(t, root, "add", ".")
	runGitFootprint(t, root, "-c", "user.email=t@example.com", "-c", "user.name=t", "commit", "-q", "-m", "one")
	revs, _, note := planGitHistory(context.Background(), root, "docs/plans/p.md")
	if len(revs) != 1 || note != "" {
		t.Fatalf("history = %d revisions, note %q", len(revs), note)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	revs, _, note = planGitHistory(ctx, root, "docs/plans/p.md")
	if len(revs) != 0 || !strings.Contains(note, "stopped") {
		t.Fatalf("under an ended context = %d revisions, note %q; want none and the stop named", len(revs), note)
	}
}

// TestRefactorReportDeadlineIsNamedNotClean (RT-118): a computation cut
// off by consoleConserveDeadline is a named failure, never a report that
// shows its unfinished planes clean and is kept until HEAD moves.
func TestRefactorReportDeadlineIsNamedNotClean(t *testing.T) {
	f := newConsoleFixture(t)
	root := t.TempDir()
	gitInitTempRepo(t, root)
	runGitFootprint(t, root, "config", "user.email", "t@example.com")
	runGitFootprint(t, root, "config", "user.name", "t")
	mustWrite(t, filepath.Join(root, "docs", "plans", "p.md"), "# P\n")
	mustWrite(t, filepath.Join(root, "a.go"), "package a\n")
	runGitFootprint(t, root, "add", ".")
	runGitFootprint(t, root, "commit", "-qm", "base")
	mustWrite(t, filepath.Join(root, "a.go"), "package a\n\nfunc A() {}\n")
	runGitFootprint(t, root, "add", ".")
	runGitFootprint(t, root, "commit", "-qm", "two")
	realGit, err := exec.LookPath("git")
	if err != nil {
		t.Skip("no git")
	}
	bin := t.TempDir()
	mustWrite(t, filepath.Join(bin, "git"), "#!/bin/sh\nfor a in \"$@\"; do if [ \"$a\" = show ]; then sleep 3; fi; done\nexec "+realGit+" \"$@\"\n")
	if err := os.Chmod(filepath.Join(bin, "git"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	prev := consoleConserveDeadline
	consoleConserveDeadline = 500 * time.Millisecond
	t.Cleanup(func() { consoleConserveDeadline = prev })
	resetConserveCache()
	t.Cleanup(resetConserveCache)
	pc := plansFor(f.deps)
	e, ok := refactorReport(pc, serviceRoot{Service: "proof-service", Root: root}, "HEAD~1", 30*time.Second)
	if !ok {
		t.Fatal("the range never finished")
	}
	if e.err == "" || !strings.Contains(e.err, "not computed within") {
		t.Fatalf("a computation cut off at its deadline = err %q, %d planes; want a named 'not computed within' failure", e.err, len(e.report.Planes))
	}
}
