package certify

// git_test.go — CV-4's own test: TestReachUnchangedSince_CommittedAndWorktreeEdits
// (design doc §1.1, O114). Uses a real temporary git repository rather
// than a faked GitRunner: reachUnchangedSince's whole job is to see BOTH
// a committed edit and an uncommitted worktree edit in one read, and that
// is exactly the distinction a canned git-diff-output fake would have to
// get right by construction — proving it against real git is cheaper and
// more honest than proving a fake matches real git's behaviour.

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func mustGit(t *testing.T, run GitRunner, dir string, args ...string) []byte {
	t.Helper()
	out, err := run(dir, args...)
	if err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
	return out
}

func TestReachUnchangedSince_CommittedAndWorktreeEdits(t *testing.T) {
	dir := t.TempDir()
	run := ExecGit
	git := func(args ...string) { mustGit(t, run, dir, args...) }
	write := func(name, content string) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	git("init", "-q")
	git("config", "user.email", "test@example.com")
	git("config", "user.name", "test")

	write("a.go", "package a\nfunc A() {}\n")
	write("b.go", "package a\nfunc B() {}\n")
	git("add", "-A")
	git("commit", "-q", "-m", "c0")
	c0 := strings.TrimSpace(string(mustGit(t, run, dir, "rev-parse", "HEAD")))

	t.Run("nothing changed since HEAD is unchanged", func(t *testing.T) {
		ok, err := reachUnchangedSince(run, dir, c0, []string{"a.go", "b.go"})
		if err != nil {
			t.Fatal(err)
		}
		if !ok {
			t.Error("want unchanged, nothing has been touched since c0")
		}
	})

	// A COMMITTED edit to a.go.
	write("a.go", "package a\nfunc A() { _ = 1 }\n")
	git("add", "-A")
	git("commit", "-q", "-m", "c1: edit a.go")
	c1 := strings.TrimSpace(string(mustGit(t, run, dir, "rev-parse", "HEAD")))

	t.Run("a file changed in a LATER COMMIT is reported changed", func(t *testing.T) {
		ok, err := reachUnchangedSince(run, dir, c0, []string{"a.go", "b.go"})
		if err != nil {
			t.Fatal(err)
		}
		if ok {
			t.Error("want changed: a.go was edited and committed after c0")
		}
	})

	t.Run("a file not in the reach list is ignored even though it changed", func(t *testing.T) {
		ok, err := reachUnchangedSince(run, dir, c0, []string{"b.go"})
		if err != nil {
			t.Fatal(err)
		}
		if !ok {
			t.Error("want unchanged: only b.go is in the reach, and b.go never changed")
		}
	})

	// An UNCOMMITTED edit to b.go — never git add'ed, never committed.
	write("b.go", "package a\nfunc B() { _ = 2 }\n")

	t.Run("an uncommitted worktree edit is reported changed", func(t *testing.T) {
		ok, err := reachUnchangedSince(run, dir, c1, []string{"a.go", "b.go"})
		if err != nil {
			t.Fatal(err)
		}
		if ok {
			t.Error("want changed: b.go carries an uncommitted worktree edit")
		}
	})

	t.Run("an empty commit is refused rather than silently answered", func(t *testing.T) {
		if _, err := reachUnchangedSince(run, dir, "", []string{"a.go"}); err == nil {
			t.Error("want an error for an empty commit")
		}
	})
}

// TestExecGitContextStopsAtItsContext (RT-118, design §14 "git via
// CommandContext"): the runner a caller builds from its context stops git
// when that context ends — a console deadline reaches the git a conserve
// plane spawns.
func TestExecGitContextStopsAtItsContext(t *testing.T) {
	dir := t.TempDir()
	mustGit(t, ExecGit, dir, "init", "-q")
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := ExecGitContext(ctx)(dir, "rev-parse", "--git-dir"); err == nil {
		t.Fatal("a git run under an ended context answered")
	}
	if out, err := ExecGitContext(context.Background())(dir, "rev-parse", "--git-dir"); err != nil || !strings.Contains(string(out), ".git") {
		t.Fatalf("a live context: %q, %v", out, err)
	}
}
