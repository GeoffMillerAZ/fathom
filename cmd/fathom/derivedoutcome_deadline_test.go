package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

// TestDerivedCommitsGitDeadlineAndRetry (RT-115, I38): the model's
// commits-in-span git read answers within consoleGitTimeout or is stopped
// (never holding the model's one rebuild flight), a stopped read is not
// kept, and a failed read is kept only for consolePlansRetry — an EPERM
// wedge on the volume clears, and the unit returns to the outcome medians
// without waiting for HEAD to move.
func TestDerivedCommitsGitDeadlineAndRetry(t *testing.T) {
	realGit, err := exec.LookPath("git")
	if err != nil {
		t.Skip("no git")
	}
	root := t.TempDir()
	gitInitTempRepo(t, root)
	mustWrite(t, filepath.Join(root, "a.go"), "package a\n")
	runGitFootprint(t, root, "add", ".")
	runGitFootprint(t, root, "-c", "user.email=t@example.com", "-c", "user.name=t", "commit", "-qm", "one")
	derivedCommitsMemo.Lock()
	derivedCommitsMemo.m = nil
	derivedCommitsMemo.Unlock()
	t.Cleanup(func() { derivedCommitsMemo.Lock(); derivedCommitsMemo.m = nil; derivedCommitsMemo.Unlock() })
	clock := time.Now()
	prevNow := derivedCommitsNow
	derivedCommitsNow = func() time.Time { return clock }
	t.Cleanup(func() { derivedCommitsNow = prevNow })
	prevGit := consoleGitTimeout
	consoleGitTimeout = 300 * time.Millisecond
	t.Cleanup(func() { consoleGitTimeout = prevGit })
	start, end := time.Now().Add(-time.Hour), time.Now().Add(time.Hour)

	bin := t.TempDir()
	wrapper := filepath.Join(bin, "git")
	mustWrite(t, wrapper, "#!/bin/sh\nexec sleep 30\n")
	if err := os.Chmod(wrapper, 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	began := time.Now()
	if _, ok := derivedCommitsInSpan(root, start, end); ok {
		t.Fatal("a hung git answered")
	}
	if took := time.Since(began); took > 5*time.Second {
		t.Fatalf("a hung git held the model's read %s", took)
	}

	// A git that fails (the volume's wedge): not an answer, kept briefly.
	mustWrite(t, wrapper, "#!/bin/sh\necho 'fatal: Operation not permitted' >&2\nexit 128\n")
	if _, ok := derivedCommitsInSpan(root, start, end); ok {
		t.Fatal("a failing git answered")
	}
	// The wedge clears.
	mustWrite(t, wrapper, "#!/bin/sh\nexec "+realGit+" \"$@\"\n")
	clock = clock.Add(consolePlansRetry)
	if n, ok := derivedCommitsInSpan(root, start, end); !ok || n != 1 {
		t.Fatalf("after the wedge cleared = %d, %v; want the one commit (a failure is not kept until HEAD moves)", n, ok)
	}
}
