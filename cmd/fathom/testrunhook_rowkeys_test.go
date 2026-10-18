package main

// testrunhook_rowkeys_test.go — RO-35 (docs/plans/readouts.md): the
// test-run row carries `head`, `tree` and `scope`, additive meta a delta
// reads, on every matched run and inside the hook's deadline. A miss leaves
// the key absent, never empty.

import (
	"context"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"
)

// commitAll commits everything in repo and returns the commit's sha.
func commitAll(t *testing.T, repo string) string {
	t.Helper()
	for _, args := range [][]string{
		{"add", "-A"},
		{"-c", "user.name=t", "-c", "user.email=t@example.test", "-c", "commit.gpgsign=false", "commit", "-q", "-m", "init"},
		{"rev-parse", "HEAD"},
	} {
		cmd := exec.Command("git", append([]string{"-C", repo}, args...)...)
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
		if args[0] == "rev-parse" {
			return strings.TrimSpace(string(out))
		}
	}
	return ""
}

var treeDigestRe = regexp.MustCompile(`^[0-9a-f]{64}$`)

func lastTestRunRow(t *testing.T) map[string]string {
	t.Helper()
	rows := testRunBeats(t)
	if len(rows) == 0 {
		t.Fatal("no test-run row")
	}
	return rows[len(rows)-1]
}

func TestTestRunRowCarriesHeadTreeAndScope(t *testing.T) {
	srv, _ := countingDaemon(t)
	repo := testRunRepo(t, true)
	writeRepoFile(t, repo, "money/money.go", "package money\n\nfunc Round(n int) int { return n }\n")
	head := commitAll(t, repo)
	// a Go working-tree change, so the diff has text
	writeRepoFile(t, repo, "money/money.go", "package money\n\nfunc Round(n int) int { return n + 1 }\n")

	const cmd = "go test ./money/ -run TestRound -skip Slow"
	if code, _ := runTestRunHook(t, "test-run", repo, srv.URL, testRunPostPayload(repo, cmd, testRunPassingOutput)); code != 0 {
		t.Fatalf("exit %d", code)
	}
	pass := lastTestRunRow(t)
	if pass["result"] != "pass" {
		t.Fatalf("row = %v, want a pass", pass)
	}
	if pass["head"] != head {
		t.Errorf("pass head = %q, want %q", pass["head"], head)
	}
	if !treeDigestRe.MatchString(pass["tree"]) {
		t.Errorf("pass tree = %q, want a sha256", pass["tree"])
	}
	if want := "./money/ run=TestRound skip=Slow"; pass["scope"] != want {
		t.Errorf("pass scope = %q, want %q", pass["scope"], want)
	}

	// the same tree hashes the same; a failing run carries the keys too
	if code, _ := runTestRunHook(t, "failed-test-run", repo, srv.URL, testRunFailurePayload(repo, cmd, testRunFailingOutput)); code != 0 {
		t.Fatalf("exit %d", code)
	}
	fail := lastTestRunRow(t)
	if fail["result"] != "fail" || fail["head"] != head || fail["tree"] != pass["tree"] || fail["scope"] != pass["scope"] {
		t.Errorf("fail row = %v, want the pass row's head, tree and scope", fail)
	}

	// an edit changes the tree
	writeRepoFile(t, repo, "money/money.go", "package money\n\nfunc Round(n int) int { return n + 2 }\n")
	runTestRunHook(t, "test-run", repo, srv.URL, testRunPostPayload(repo, cmd, testRunPassingOutput))
	if edited := lastTestRunRow(t); edited["tree"] == pass["tree"] || !treeDigestRe.MatchString(edited["tree"]) {
		t.Errorf("tree after an edit = %q, was %q", edited["tree"], pass["tree"])
	}
}

func TestTestRunRowTreeCountsAnEditedUntrackedGoFile(t *testing.T) {
	srv, _ := countingDaemon(t)
	repo := testRunRepo(t, true)
	origDir := editSetDir
	editSetDir = filepath.Join(t.TempDir(), "edit-set")
	defer func() { editSetDir = origDir }()
	writeRepoFile(t, repo, "money/money.go", "package money\n")
	commitAll(t, repo)

	run := func() string {
		runTestRunHook(t, "test-run", repo, srv.URL, testRunPostPayload(repo, "go test ./...", testRunPassingOutput))
		return lastTestRunRow(t)["tree"]
	}
	clean := run()
	// a new file the diff against HEAD cannot see, not in the session's edit set
	writeRepoFile(t, repo, "money/new.go", "package money\n\nvar X = 1\n")
	if got := run(); got != clean {
		t.Errorf("an untracked file outside the edit set changed the tree: %q vs %q", got, clean)
	}
	markFileEdited("tr-session", "money/new.go", time.Now())
	withNew := run()
	if withNew == clean || !treeDigestRe.MatchString(withNew) {
		t.Errorf("tree with the edited untracked file = %q, clean %q", withNew, clean)
	}
	writeRepoFile(t, repo, "money/new.go", "package money\n\nvar X = 2\n")
	if got := run(); got == withNew {
		t.Errorf("tree did not follow the untracked file's content")
	}
}

func TestTestRunRowKeysAbsentOnAMiss(t *testing.T) {
	srv, _ := countingDaemon(t)

	// no commit: no HEAD, and no diff against it
	repo := testRunRepo(t, true)
	runTestRunHook(t, "test-run", repo, srv.URL, testRunPostPayload(repo, "go test ./...", testRunPassingOutput))
	row := lastTestRunRow(t)
	for _, k := range []string{"head", "tree"} {
		if v, ok := row[k]; ok {
			t.Errorf("repository with no commit: row[%s] = %q, want the key absent", k, v)
		}
	}

	// a deadline already past: nothing is computed, nothing is written empty
	repo2 := testRunRepo(t, true)
	writeRepoFile(t, repo2, "a.go", "package a\n")
	commitAll(t, repo2)
	orig := testRunHookContext
	testRunHookContext = func() (context.Context, context.CancelFunc) {
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		return ctx, cancel
	}
	defer func() { testRunHookContext = orig }()
	runTestRunHook(t, "test-run", repo2, srv.URL, testRunPostPayload(repo2, "go test ./...", testRunPassingOutput))
	row = lastTestRunRow(t)
	for _, k := range []string{"head", "tree"} {
		if v, ok := row[k]; ok {
			t.Errorf("past deadline: row[%s] = %q, want the key absent", k, v)
		}
	}
	if row["result"] != "pass" {
		t.Errorf("row = %v, the run still gets its row", row)
	}
}

func TestTestRunScopeLexing(t *testing.T) {
	for _, tc := range []struct{ command, want string }{
		{"go test ./...", "./..."},
		{"go test", ""},
		{"go test ./a/ ./b/ ./a/", "./a/,./b/"},
		{"go test -race -count=1 ./money/", "./money/"},
		{"go test -count 1 -timeout 5m ./money/", "./money/"},
		{"go test -run TestA -skip=TestB ./x", "./x run=TestA skip=TestB"},
		{"go test -run=^TestA$ ./x", "./x run=^TestA$"},
		{"cd x && GOFLAGS=-mod=mod go test ./y | tail -n 5", "./y"},
		{"go test ./z -args -run foo", "./z"},
		{"FATHOM_X=1 go test -run 'TestA|TestB' ./q", "./q run=TestA|TestB"},
		{"task test", ""},
	} {
		if got := testRunScope(tc.command, "go test"); tc.command != "task test" && got != tc.want {
			t.Errorf("scope(%q) = %q, want %q", tc.command, got, tc.want)
		}
	}
	if got := testRunScope("task test", "task test"); got != "" {
		t.Errorf("scope of a task invocation = %q, want absent", got)
	}
}
