package scenariobench

import (
	"bytes"
	"context"
	"encoding/hex"
	"fmt"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"

	"github.com/geoffmilleraz/fathom/internal/wiring"
)

// gitFixtureRepo creates a local git repository with one commit (a file
// tracked at "marker.txt") and returns its path and that commit's sha —
// entirely local, so PrepareSubject's clone/worktree-add path is testable
// with no network.
func gitFixtureRepo(t *testing.T) (repoPath, commit string) {
	t.Helper()
	dir := t.TempDir()
	run := func(args ...string) string {
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		cmd.Env = append(
			gitEnv(),
			"GIT_AUTHOR_NAME=test", "GIT_AUTHOR_EMAIL=test@example.invalid",
			"GIT_COMMITTER_NAME=test", "GIT_COMMITTER_EMAIL=test@example.invalid",
		)
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("git %v: %v (%s)", args, err, out)
		}
		return strings.TrimSpace(string(out))
	}
	run("init", "-q", "-b", "main")
	writeTestFile(t, filepath.Join(dir, "marker.txt"), "hello from the fixture subject\n")
	run("add", "marker.txt")
	run("commit", "-q", "-m", "initial commit")
	sha := run("rev-parse", "HEAD")
	return dir, sha
}

// gitFixtureRepoWithFiles is gitFixtureRepo plus extra tracked files
// (relative path -> content) committed in the same initial commit — for
// tests that need an answer-key-shaped file already present in the
// subject before PrepareSubject ever runs.
func gitFixtureRepoWithFiles(t *testing.T, extra map[string]string) (repoPath, commit string) {
	t.Helper()
	dir := t.TempDir()
	run := func(args ...string) string {
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		cmd.Env = append(
			gitEnv(),
			"GIT_AUTHOR_NAME=test", "GIT_AUTHOR_EMAIL=test@example.invalid",
			"GIT_COMMITTER_NAME=test", "GIT_COMMITTER_EMAIL=test@example.invalid",
		)
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("git %v: %v (%s)", args, err, out)
		}
		return strings.TrimSpace(string(out))
	}
	run("init", "-q", "-b", "main")
	writeTestFile(t, filepath.Join(dir, "marker.txt"), "hello from the fixture subject\n")
	for rel, content := range extra {
		full := filepath.Join(dir, rel)
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatalf("mkdir for %s: %v", rel, err)
		}
		writeTestFile(t, full, content)
	}
	run("add", "-A")
	run("commit", "-q", "-m", "initial commit")
	sha := run("rev-parse", "HEAD")
	return dir, sha
}

// TestPrepareSubject_RemovesExcludedPathsBeforeSetup is O53 defect 3's
// prepare-time-removal proof: a Subject.ExcludePaths glob must be gone
// from the worktree BEFORE the subject's own Setup command runs (so a
// script cannot depend on the excluded file's presence), for an unwired
// arm exactly like a wired one — nothing about this removal is
// wired-arm-specific. A file NOT matching any exclude pattern must
// survive untouched.
func TestPrepareSubject_RemovesExcludedPathsBeforeSetup(t *testing.T) {
	repoPath, commit := gitFixtureRepoWithFiles(t, map[string]string{
		"testdata/contextbench/tasks-home.json": `{"H1-T01":{"ground_truth":"the answer key"}}`,
		"docs/bench/scenarios.md":               "# bench scenarios\n",
		"keep/me.txt":                           "not excluded\n",
	})
	root := t.TempDir()
	task := Task{
		ID: "T1",
		Subject: Subject{
			RepoURL:    repoPath,
			BaseCommit: commit,
			ExcludePaths: []string{
				"testdata/contextbench/**",
				"testdata/scenariobench/**",
				"docs/bench/**",
			},
			// If exclusion ran AFTER Setup instead of before, this command
			// (which fails when the excluded file is still present) would
			// catch it too — belt and suspenders on top of the direct
			// os.Stat assertions below.
			Setup: []string{"test ! -e testdata/contextbench/tasks-home.json"},
		},
	}
	arm := Arm{Instructions: "none"}

	wt, err := PrepareSubject(context.Background(), task, arm, root)
	if err != nil {
		t.Fatalf("PrepareSubject: %v", err)
	}
	defer StopWorktree(&wt)

	if _, err := os.Stat(filepath.Join(wt.Path, "testdata", "contextbench", "tasks-home.json")); !os.IsNotExist(err) {
		t.Errorf("expected the excluded answer-key file to be removed, got err=%v", err)
	}
	if _, err := os.Stat(filepath.Join(wt.Path, "docs", "bench")); !os.IsNotExist(err) {
		t.Errorf("expected docs/bench to be removed, got err=%v", err)
	}
	if _, err := os.Stat(filepath.Join(wt.Path, "keep", "me.txt")); err != nil {
		t.Errorf("expected a non-excluded file to survive: %v", err)
	}
	if _, err := os.Stat(filepath.Join(wt.Path, "marker.txt")); err != nil {
		t.Errorf("expected the subject's own file to survive: %v", err)
	}
}

// gitFixtureRepoWithFuture is gitFixtureRepo plus a SECOND commit after the
// base commit — the shape a repo with a merged fix has: the fix (or, as in
// O55, a control planner's discovery of it) lives strictly after base in
// `git log`. Returns the repo path, base's sha, and the future commit's own
// sha.
func gitFixtureRepoWithFuture(t *testing.T) (repoPath, base, future string) {
	t.Helper()
	dir, baseSHA := gitFixtureRepo(t)
	run := func(args ...string) string {
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		cmd.Env = append(
			gitEnv(),
			"GIT_AUTHOR_NAME=test", "GIT_AUTHOR_EMAIL=test@example.invalid",
			"GIT_COMMITTER_NAME=test", "GIT_COMMITTER_EMAIL=test@example.invalid",
		)
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("git %v: %v (%s)", args, err, out)
		}
		return strings.TrimSpace(string(out))
	}
	writeTestFile(t, filepath.Join(dir, "future.txt"), "PR #10124's merged fix\n")
	run("add", "future.txt")
	run("commit", "-q", "-m", "merge: PR #10124 the fix the task is measuring")
	futureSHA := run("rev-parse", "HEAD")
	return dir, baseSHA, futureSHA
}

// TestPrepareSubject_WorktreeHasNoFuture pins O55 item 1: a cell worktree
// built off a full clone must never let `git log --all` (or any other
// means) resolve a commit that only exists after base_commit — a control
// planner found the task's own merged fix that way. After PrepareSubject,
// the worktree's own history is exactly one commit and the future commit's
// sha is unresolvable inside it.
func TestPrepareSubject_WorktreeHasNoFuture(t *testing.T) {
	repoPath, base, future := gitFixtureRepoWithFuture(t)
	root := t.TempDir()
	task := Task{ID: "T1", Subject: Subject{RepoURL: repoPath, BaseCommit: base}}
	arm := Arm{Instructions: "none"}

	wt, err := PrepareSubject(context.Background(), task, arm, root)
	if err != nil {
		t.Fatalf("PrepareSubject: %v", err)
	}
	defer StopWorktree(&wt)

	out, err := exec.Command("git", "-C", wt.Path, "log", "--all", "--oneline").CombinedOutput()
	if err != nil {
		t.Fatalf("git log --all: %v (%s)", err, out)
	}
	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	if len(lines) != 1 || lines[0] == "" {
		t.Errorf("expected exactly one commit in the cell worktree's history, got %d: %q", len(lines), out)
	}
	if _, err := os.Stat(filepath.Join(wt.Path, "future.txt")); err == nil {
		t.Error("the future commit's own file must not be present in the cell worktree's tree")
	}
	resolve, err := exec.Command("git", "-C", wt.Path, "cat-file", "-e", future+"^{commit}").CombinedOutput()
	if err == nil {
		t.Errorf("expected the future commit's sha to be unresolvable in the cell worktree, but it resolved: %s", resolve)
	}

	// Reuse (the same subject+base_commit+arm cell prepared again) must
	// still land on a single-commit history, not accumulate one per call.
	wt2, err := PrepareSubject(context.Background(), task, arm, root)
	if err != nil {
		t.Fatalf("PrepareSubject (2nd): %v", err)
	}
	defer StopWorktree(&wt2)
	out2, err := exec.Command("git", "-C", wt2.Path, "log", "--all", "--oneline").CombinedOutput()
	if err != nil {
		t.Fatalf("git log --all (2nd): %v (%s)", err, out2)
	}
	lines2 := strings.Split(strings.TrimSpace(string(out2)), "\n")
	if len(lines2) != 1 || lines2[0] == "" {
		t.Errorf("expected exactly one commit after reuse, got %d: %q", len(lines2), out2)
	}
}

// gitFixtureRepoWithExportIgnore builds a local git repository whose
// .gitattributes marks one committed file export-ignore — `git archive`
// silently drops such a file from its own output, a real checkout
// (`git worktree add`, `checkout-index`) never does. Returns the repo path
// and the commit's sha.
func gitFixtureRepoWithExportIgnore(t *testing.T) (repoPath, commit string) {
	t.Helper()
	dir := t.TempDir()
	run := func(args ...string) string {
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		cmd.Env = append(
			gitEnv(),
			"GIT_AUTHOR_NAME=test", "GIT_AUTHOR_EMAIL=test@example.invalid",
			"GIT_COMMITTER_NAME=test", "GIT_COMMITTER_EMAIL=test@example.invalid",
		)
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("git %v: %v (%s)", args, err, out)
		}
		return strings.TrimSpace(string(out))
	}
	run("init", "-q", "-b", "main")
	writeTestFile(t, filepath.Join(dir, "normal.txt"), "normal content\n")
	writeTestFile(t, filepath.Join(dir, "secret.txt"), "export-ignore'd content\n")
	writeTestFile(t, filepath.Join(dir, ".gitattributes"), "secret.txt export-ignore\n")
	run("add", "-A")
	run("commit", "-q", "-m", "initial commit")
	sha := run("rev-parse", "HEAD")
	return dir, sha
}

// TestCheckoutCommitTree_KeepsExportIgnoredFiles pins O55 item 1's
// fix-first review directly: checkoutCommitTree's own read-tree +
// checkout-index must keep an export-ignore'd file that `git archive`
// would silently drop.
func TestCheckoutCommitTree_KeepsExportIgnoredFiles(t *testing.T) {
	repoPath, commit := gitFixtureRepoWithExportIgnore(t)
	wtPath := filepath.Join(t.TempDir(), "wt")
	if err := os.MkdirAll(wtPath, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := checkoutCommitTree(context.Background(), repoPath, commit, wtPath); err != nil {
		t.Fatalf("checkoutCommitTree: %v", err)
	}
	raw, err := os.ReadFile(filepath.Join(wtPath, "secret.txt"))
	if err != nil {
		t.Fatalf("expected secret.txt (export-ignore'd) to be present — git archive would have dropped it: %v", err)
	}
	if string(raw) != "export-ignore'd content\n" {
		t.Errorf("secret.txt content = %q, want the committed content", raw)
	}
	if _, err := os.Stat(filepath.Join(wtPath, "normal.txt")); err != nil {
		t.Errorf("expected normal.txt present: %v", err)
	}
	// checkoutCommitTree must never mutate the CLONE's own main index —
	// only a temporary one.
	if _, err := os.Stat(filepath.Join(repoPath, ".git", "index")); err != nil {
		t.Fatalf("the clone's own index vanished: %v", err)
	}
}

// TestPrepareSubject_ExportIgnoredFilesSurvive is the end-to-end proof:
// PrepareSubject's own cell worktree still carries a subject's
// export-ignore'd file, not just checkoutCommitTree in isolation.
func TestPrepareSubject_ExportIgnoredFilesSurvive(t *testing.T) {
	repoPath, commit := gitFixtureRepoWithExportIgnore(t)
	root := t.TempDir()
	task := Task{ID: "T1", Subject: Subject{RepoURL: repoPath, BaseCommit: commit}}
	arm := Arm{Instructions: "none"}

	wt, err := PrepareSubject(context.Background(), task, arm, root)
	if err != nil {
		t.Fatalf("PrepareSubject: %v", err)
	}
	defer StopWorktree(&wt)

	if _, err := os.Stat(filepath.Join(wt.Path, "secret.txt")); err != nil {
		t.Errorf("expected the export-ignore'd file to survive PrepareSubject's checkout, got: %v", err)
	}
	if _, err := os.Stat(filepath.Join(wt.Path, "normal.txt")); err != nil {
		t.Errorf("expected the ordinary file present: %v", err)
	}
}

func TestPrepareSubject_UnwiredArm(t *testing.T) {
	repoPath, commit := gitFixtureRepo(t)
	root := t.TempDir()
	task := Task{
		ID: "T1",
		Subject: Subject{
			RepoURL:    repoPath,
			BaseCommit: commit,
			Setup:      []string{"echo subject-setup-ran > subject-setup.marker"},
		},
	}
	arm := Arm{
		Wired:        false,
		Instructions: "none",
		Setup:        []string{"echo arm-setup-ran > arm-setup.marker"},
	}

	wt, err := PrepareSubject(context.Background(), task, arm, root)
	if err != nil {
		t.Fatalf("PrepareSubject: %v", err)
	}
	defer StopWorktree(&wt)

	if _, err := os.Stat(filepath.Join(wt.Path, "marker.txt")); err != nil {
		t.Errorf("expected the subject's file to be checked out: %v", err)
	}
	if _, err := os.Stat(filepath.Join(wt.Path, "subject-setup.marker")); err != nil {
		t.Errorf("expected the subject's Setup command to have run: %v", err)
	}
	if _, err := os.Stat(filepath.Join(wt.Path, "arm-setup.marker")); err != nil {
		t.Errorf("expected the arm's Setup command to have run: %v", err)
	}
	if wt.DaemonURL != "" {
		t.Errorf("expected no scratch daemon for an unwired arm, got DaemonURL=%q", wt.DaemonURL)
	}
	// RenderArm should have run: settings.json exists with no hooks key.
	if wt.Arm.SettingsPath == "" {
		t.Fatal("expected RenderArm to have run (SettingsPath set)")
	}
	raw, err := os.ReadFile(wt.Arm.SettingsPath)
	if err != nil {
		t.Fatalf("reading settings.json: %v", err)
	}
	if strings.Contains(string(raw), "hooks") {
		t.Errorf("unwired arm's settings.json should carry no hooks key: %s", raw)
	}

	if err := StopWorktree(&wt); err != nil {
		t.Errorf("StopWorktree on an unwired worktree should be a no-op, got: %v", err)
	}
}

func TestPrepareSubject_ReusesSharedClone(t *testing.T) {
	repoPath, commit := gitFixtureRepo(t)
	root := t.TempDir()
	task := Task{ID: "T1", Subject: Subject{RepoURL: repoPath, BaseCommit: commit}}
	arm := Arm{Instructions: "none"}

	wt1, err := PrepareSubject(context.Background(), task, arm, root)
	if err != nil {
		t.Fatalf("PrepareSubject (1st): %v", err)
	}
	defer StopWorktree(&wt1)

	wt2, err := PrepareSubject(context.Background(), task, arm, root)
	if err != nil {
		t.Fatalf("PrepareSubject (2nd): %v", err)
	}
	defer StopWorktree(&wt2)

	if wt1.SubjectClone != wt2.SubjectClone {
		t.Errorf("expected both cells to share one clone, got %q and %q", wt1.SubjectClone, wt2.SubjectClone)
	}
	// Same (subject, commit, arm): the worktree is SHARED and reset between
	// cells, so a compiled store is paid for once; a different arm gets its
	// own. Whatever the first cell left behind must be gone.
	if wt1.Path != wt2.Path {
		t.Errorf("expected cells on one (subject, commit, arm) to share a worktree, got %q and %q", wt1.Path, wt2.Path)
	}
	if _, err := os.Stat(filepath.Join(wt2.Path, "marker.txt")); err != nil {
		t.Errorf("expected the second worktree to also have the subject's file: %v", err)
	}
	if err := os.WriteFile(filepath.Join(wt2.Path, "leftover.txt"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	wt3, err := PrepareSubject(context.Background(), task, arm, root)
	if err != nil {
		t.Fatalf("PrepareSubject (3rd): %v", err)
	}
	defer StopWorktree(&wt3)
	if _, err := os.Stat(filepath.Join(wt3.Path, "leftover.txt")); !os.IsNotExist(err) {
		t.Errorf("a reused worktree must be cleaned between cells; leftover.txt survived (%v)", err)
	}
	other, err := PrepareSubject(context.Background(), task, Arm{Instructions: "plain", NotesFile: true}, root)
	if err != nil {
		t.Fatalf("PrepareSubject (other arm): %v", err)
	}
	defer StopWorktree(&other)
	if other.Path == wt1.Path {
		t.Error("a different arm must get its own worktree")
	}
}

// TestPrepareSubject_RefusesResetWithLiveSessionInWorktree is O57 finding
// 1's second half: a shared worktree whose own session pidfile names a
// STILL LIVE process refuses the reset (an error, which cmd/scenariobench
// turns into a LOST cell) rather than resetting out from under a live
// agent; once that pidfile names a DEAD process, the next prepare succeeds
// and clears it, exactly like a normal reused worktree.
func TestPrepareSubject_RefusesResetWithLiveSessionInWorktree(t *testing.T) {
	old := sessionCommandMarker
	sessionCommandMarker = "sleep"
	t.Cleanup(func() { sessionCommandMarker = old })

	repoPath, commit := gitFixtureRepo(t)
	root := t.TempDir()
	task := Task{ID: "T1", Subject: Subject{RepoURL: repoPath, BaseCommit: commit}}
	arm := Arm{Instructions: "none"}

	wt1, err := PrepareSubject(context.Background(), task, arm, root)
	if err != nil {
		t.Fatalf("PrepareSubject (1st): %v", err)
	}
	defer StopWorktree(&wt1)

	stray := exec.Command("sleep", "300")
	stray.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	if err := stray.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = stray.Process.Kill(); _, _ = stray.Process.Wait() })
	pidfile := sessionWorktreePIDFile(wt1.Path)
	if err := os.WriteFile(pidfile, []byte(fmt.Sprintf("%d\n", stray.Process.Pid)), 0o644); err != nil {
		t.Fatal(err)
	}

	if _, err := PrepareSubject(context.Background(), task, arm, root); err == nil {
		t.Fatal("expected PrepareSubject to refuse the reset with a live session pidfile in place")
	} else if !strings.Contains(err.Error(), "refusing to reset") {
		t.Errorf("PrepareSubject error = %v, want it to name the refusal", err)
	}

	// The refusal must not have touched the live process or its pidfile.
	if syscall.Kill(stray.Process.Pid, 0) != nil {
		t.Error("a live session must not be killed by a refused reset")
	}

	// Once the process is gone, the next prepare succeeds and clears the
	// stale pidfile.
	_ = stray.Process.Kill()
	waited := make(chan error, 1)
	go func() { _, err := stray.Process.Wait(); waited <- err }()
	select {
	case <-waited:
	case <-time.After(5 * time.Second):
		t.Fatal("stray session did not die")
	}
	wt2, err := PrepareSubject(context.Background(), task, arm, root)
	if err != nil {
		t.Fatalf("PrepareSubject after the session died: %v", err)
	}
	defer StopWorktree(&wt2)
	if _, err := os.Stat(pidfile); !os.IsNotExist(err) {
		t.Errorf("stale session pidfile should be cleared once prepare succeeds: %v", err)
	}
}

// TestPrepareSubject_ConcurrentCallsShareOneCloneWithoutRacing is the
// -parallel-arms clone race's own proof: two cells of the same subject
// (as -parallel-arms' batching would produce — distinct arms, one shared
// cloneDir keyed only on repo_url) calling PrepareSubject at the same
// moment must both succeed and end up sharing one valid clone, never one
// racing `git clone` into the other's half-written target directory.
func TestPrepareSubject_ConcurrentCallsShareOneCloneWithoutRacing(t *testing.T) {
	repoPath, commit := gitFixtureRepo(t)
	root := t.TempDir()
	task := Task{ID: "T1", Subject: Subject{RepoURL: repoPath, BaseCommit: commit}}

	const n = 8
	results := make([]Worktree, n)
	errs := make([]error, n)
	var wg sync.WaitGroup
	wg.Add(n)
	for i := 0; i < n; i++ {
		i := i
		go func() {
			defer wg.Done()
			// Distinct arms (as batchCellPlans would run concurrently) so
			// each gets its OWN worktree — only the shared clone is meant
			// to be contended.
			arm := Arm{Instructions: "none", Setup: []string{fmt.Sprintf("echo arm-%d > arm.marker", i)}}
			results[i], errs[i] = PrepareSubject(context.Background(), task, arm, root)
		}()
	}
	wg.Wait()

	var clone string
	for i, err := range errs {
		if err != nil {
			t.Errorf("PrepareSubject[%d]: %v", i, err)
			continue
		}
		defer StopWorktree(&results[i])
		if clone == "" {
			clone = results[i].SubjectClone
		} else if results[i].SubjectClone != clone {
			t.Errorf("PrepareSubject[%d]: SubjectClone = %q, want %q (shared)", i, results[i].SubjectClone, clone)
		}
	}
	if clone == "" {
		t.Fatal("no successful PrepareSubject call to check the clone against")
	}
	if out, err := exec.Command("git", "-C", clone, "rev-parse", "HEAD").CombinedOutput(); err != nil {
		t.Errorf("shared clone %s is not a valid git repo: %v (%s)", clone, err, out)
	}
}

func TestPrepareSubject_UnknownCommitIsAnError(t *testing.T) {
	repoPath, _ := gitFixtureRepo(t)
	root := t.TempDir()
	task := Task{ID: "T1", Subject: Subject{RepoURL: repoPath, BaseCommit: strings.Repeat("0", 40)}}
	if _, err := PrepareSubject(context.Background(), task, Arm{Instructions: "none"}, root); err == nil {
		t.Fatal("expected an error for a base_commit that does not exist")
	}
}

// TestPrepareSubject_LocalBaseUsableForInWorktreeGitDiffs is the
// fix-first review's own regression test: the fixture repo carries a
// commit AFTER task.Subject.BaseCommit — a "future" commit, standing in
// for the real source repo having moved on past this task's pin — so the
// test proves two things at once: PrepareSubject's worktree checks out
// ONLY base's own tree (never the future commit's), and
// task.Subject.BaseCommit is not even a valid git object inside that
// worktree at all (checkoutCommitIntoWorktree re-commits under a NEW
// sha) — which is exactly why every in-worktree git diff/checkout
// against "base" must go through wt.LocalBase instead: a change-present
// detector reading $SCENARIO_BASE_COMMIT (oracleEnv, via runDetector)
// and refusal.go's own changedPaths both prove it directly, end to end,
// through the real PrepareSubject.
func TestPrepareSubject_LocalBaseUsableForInWorktreeGitDiffs(t *testing.T) {
	dir := t.TempDir()
	run := func(args ...string) string {
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		cmd.Env = append(
			gitEnv(),
			"GIT_AUTHOR_NAME=test", "GIT_AUTHOR_EMAIL=test@example.invalid",
			"GIT_COMMITTER_NAME=test", "GIT_COMMITTER_EMAIL=test@example.invalid",
		)
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("git %v: %v (%s)", args, err, out)
		}
		return strings.TrimSpace(string(out))
	}
	run("init", "-q", "-b", "main")
	writeTestFile(t, filepath.Join(dir, "x.go"), "package x\n\nfunc Base() {}\n")
	run("add", "x.go")
	run("commit", "-q", "-m", "base commit")
	baseCommit := run("rev-parse", "HEAD")

	// The "future" commit: the source repo kept moving after this task
	// was pinned to baseCommit. PrepareSubject must still check out only
	// baseCommit's own tree.
	writeTestFile(t, filepath.Join(dir, "future.txt"), "the source repo kept moving after base_commit\n")
	run("add", "future.txt")
	run("commit", "-q", "-m", "a commit after base_commit")

	root := t.TempDir()
	task := Task{
		ID:      "T-localbase",
		Subject: Subject{RepoURL: dir, BaseCommit: baseCommit},
	}
	wt, err := PrepareSubject(context.Background(), task, Arm{Instructions: "plain"}, root)
	if err != nil {
		t.Fatalf("PrepareSubject: %v", err)
	}
	defer StopWorktree(&wt)

	if wt.LocalBase == "" {
		t.Fatal("expected wt.LocalBase to be set")
	}
	if wt.LocalBase == baseCommit {
		t.Errorf("wt.LocalBase = %q, want a sha DIFFERENT from baseCommit %q — checkoutCommitIntoWorktree re-commits under its own root", wt.LocalBase, baseCommit)
	}
	if _, err := os.Stat(filepath.Join(wt.Path, "future.txt")); !os.IsNotExist(err) {
		t.Errorf("expected future.txt (the future commit's own file) to be absent from the checked-out tree, got err=%v", err)
	}
	// The whole reason LocalBase exists: task.Subject.BaseCommit does not
	// resolve inside this worktree at all.
	if err := exec.Command("git", "-C", wt.Path, "cat-file", "-e", baseCommit+"^{commit}").Run(); err == nil {
		t.Error("expected task.Subject.BaseCommit to NOT resolve inside wt.Path — that gap is exactly what this fix addresses")
	}

	detector := Detector{
		Name:       "change-present",
		Command:    "git diff --name-only $SCENARIO_BASE_COMMIT -- . | grep -q x.go",
		ExpectExit: 0,
	}
	task.Oracle.Detectors = []Detector{detector}

	before, err := runDetector(context.Background(), wt, task, detector)
	if err != nil {
		t.Fatalf("runDetector (before edit): %v", err)
	}
	if before.Exit != 1 {
		t.Errorf("before any edit: detector exit = %d, want 1 (no diff against $SCENARIO_BASE_COMMIT yet)", before.Exit)
	}

	writeTestFile(t, filepath.Join(wt.Path, "x.go"), "package x\n\nfunc Base() {}\nfunc Added() {}\n")

	after, err := runDetector(context.Background(), wt, task, detector)
	if err != nil {
		t.Fatalf("runDetector (after edit): %v", err)
	}
	if after.Exit != 0 {
		t.Errorf("after editing x.go: detector exit = %d, want 0", after.Exit)
	}

	changed, err := changedPaths(context.Background(), wt, wt.LocalBase)
	if err != nil {
		t.Fatalf("changedPaths: %v", err)
	}
	found := false
	for _, p := range changed {
		if p == "x.go" {
			found = true
		}
	}
	if !found {
		t.Errorf("changedPaths = %v, want it to include x.go", changed)
	}
}

// TestCheckoutCommitIntoWorktree_OperatorSafecrlfDoesNotBreakCRLFSubjectFile
// is VB-27 prep's own regression test (2026-09-16): running the real
// scenariobench driver against joke2k/faker (a Long Code Arena
// bug-localization subject, docs/bench/scenarios.md "External corpus (LCA
// bug localization)") failed both cells with "fatal: CRLF would be
// replaced by LF in docs/make.bat" — checkoutCommitIntoWorktree's own `git
// add -A` inherits the OPERATOR's real global git config (SubjectEnv
// passes the full process environment through, HOME included), and this
// operator's config carries core.autocrlf=input, core.safecrlf=true
// (confirmed directly). That config has nothing to do with the bench: the
// whole point of checkoutCommitIntoWorktree, per its own doc comment
// above, is to preserve base's tree EXACTLY (checkout-index over `git
// archive`, specifically to avoid a silent byte-level rewrite) — an
// autocrlf/safecrlf policy from the operator's own unrelated dev
// environment must never veto or rewrite a subject's own line endings.
// This test never touches the real global config: GIT_CONFIG_GLOBAL
// points every git subprocess at a throwaway file carrying the same
// autocrlf=input/safecrlf=true the operator's own config had, so the
// failure reproduces hermetically, then proves the fix (explicit
// core.autocrlf=false/core.safecrlf=false on the throwaway worktree repo,
// before `git add -A`) both unblocks the commit AND keeps the CRLF bytes
// byte-for-byte.
func TestCheckoutCommitIntoWorktree_OperatorSafecrlfDoesNotBreakCRLFSubjectFile(t *testing.T) {
	dir := t.TempDir()
	// -c core.autocrlf=false -c core.safecrlf=false: fixture setup must be
	// hermetic regardless of THIS machine's own real global git config —
	// confirmed directly, this dev environment's own ~/.gitconfig already
	// carries safecrlf=true, which would otherwise fail this same `git add`
	// before the test even gets to simulate anything.
	run := func(args ...string) string {
		full := append([]string{"-c", "core.autocrlf=false", "-c", "core.safecrlf=false"}, args...)
		cmd := exec.Command("git", full...)
		cmd.Dir = dir
		cmd.Env = append(
			gitEnv(),
			"GIT_AUTHOR_NAME=test", "GIT_AUTHOR_EMAIL=test@example.invalid",
			"GIT_COMMITTER_NAME=test", "GIT_COMMITTER_EMAIL=test@example.invalid",
		)
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("git %v: %v (%s)", args, err, out)
		}
		return strings.TrimSpace(string(out))
	}
	run("init", "-q", "-b", "main")
	const crlfContent = "echo hello\r\necho world\r\n"
	if err := os.WriteFile(filepath.Join(dir, "make.bat"), []byte(crlfContent), 0o644); err != nil {
		t.Fatal(err)
	}
	run("add", "make.bat")
	run("commit", "-q", "-m", "base commit with a CRLF file")
	base := run("rev-parse", "HEAD")

	// From here on, GIT_CONFIG_GLOBAL simulates the OPERATOR's own real
	// config for checkoutCommitIntoWorktree's own subprocesses only — the
	// exact thing under test, unlike the hermetic fixture setup above.
	globalConfig := filepath.Join(t.TempDir(), "gitconfig-safecrlf")
	if err := os.WriteFile(globalConfig, []byte("[core]\n\tautocrlf = input\n\tsafecrlf = true\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("GIT_CONFIG_GLOBAL", globalConfig)

	wtPath := t.TempDir()
	if err := checkoutCommitIntoWorktree(context.Background(), dir, base, wtPath); err != nil {
		t.Fatalf("checkoutCommitIntoWorktree: %v (this is the exact failure joke2k/faker hit under the operator's real global git config)", err)
	}

	got, err := os.ReadFile(filepath.Join(wtPath, "make.bat"))
	if err != nil {
		t.Fatalf("reading checked-out make.bat: %v", err)
	}
	if string(got) != crlfContent {
		t.Errorf("make.bat bytes = %q, want the CRLF bytes preserved exactly (never normalized to LF)", got)
	}
}

// newFsckViolatingBareRepo builds a bare repo whose HEAD commit's tree
// carries a hand-crafted, zero-padded file mode ("0100644" rather than the
// "100644" `git write-tree` would ever produce) — exactly the class of
// malformed object git's fsck names zeroPaddedFilemode, and exactly what
// tootallnate/java-websocket's own real history carries (confirmed
// directly against lca-run-1's log, workspace/logs/lca-run-1.log,
// 2026-09-17: `error: object …: zeroPaddedFilemode: contains zero-padded
// file modes` / `fatal: fsck error in packed object`). `git hash-object -w`
// refuses to store a tree this malformed at all (its own fsck-on-write
// check) — `--literally` is git's own escape hatch for exactly this. The
// commit is built from LOOSE objects copied straight into a fresh bare
// repo (content-addressed, so a plain file copy is safe) and a ref/HEAD
// set directly with `update-ref`/`symbolic-ref`, never `git push` — a push
// would run the BARE repo's own receive.fsckObjects check, which nothing
// here has any reason to touch.
func newFsckViolatingBareRepo(t *testing.T) string {
	t.Helper()
	src := t.TempDir()
	run := func(args ...string) string {
		cmd := exec.Command("git", args...)
		cmd.Dir = src
		cmd.Env = append(
			gitEnv(),
			"GIT_AUTHOR_NAME=test", "GIT_AUTHOR_EMAIL=test@example.invalid",
			"GIT_COMMITTER_NAME=test", "GIT_COMMITTER_EMAIL=test@example.invalid",
		)
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("git %v: %v (%s)", args, err, out)
		}
		return strings.TrimSpace(string(out))
	}
	run("init", "-q", "-b", "main")
	if err := os.WriteFile(filepath.Join(src, "file.txt"), []byte("hello\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	blobSHA := run("hash-object", "-w", "file.txt")
	rawSHA, err := hex.DecodeString(blobSHA)
	if err != nil {
		t.Fatalf("decoding blob sha %q: %v", blobSHA, err)
	}
	var tree bytes.Buffer
	tree.WriteString("0100644 file.txt\x00")
	tree.Write(rawSHA)
	treePath := filepath.Join(src, "tree.bin")
	if err := os.WriteFile(treePath, tree.Bytes(), 0o644); err != nil {
		t.Fatal(err)
	}
	treeSHA := run("hash-object", "--literally", "-w", "-t", "tree", treePath)
	commitSHA := run("commit-tree", treeSHA, "-m", "bad mode commit")

	bare := t.TempDir()
	if out, err := exec.Command("git", "init", "-q", "--bare", bare).CombinedOutput(); err != nil {
		t.Fatalf("git init --bare %s: %v (%s)", bare, err, out)
	}
	srcObjects := filepath.Join(src, ".git", "objects")
	if err := filepath.WalkDir(srcObjects, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		rel, err := filepath.Rel(srcObjects, path)
		if err != nil {
			return err
		}
		dst := filepath.Join(bare, "objects", rel)
		if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
			return err
		}
		return copyFile(path, dst)
	}); err != nil {
		t.Fatalf("copying loose objects into the bare repo: %v", err)
	}
	if out, err := exec.Command("git", "--git-dir="+bare, "update-ref", "refs/heads/main", commitSHA).CombinedOutput(); err != nil {
		t.Fatalf("git update-ref: %v (%s)", err, out)
	}
	if out, err := exec.Command("git", "--git-dir="+bare, "symbolic-ref", "HEAD", "refs/heads/main").CombinedOutput(); err != nil {
		t.Fatalf("git symbolic-ref: %v (%s)", err, out)
	}
	return bare
}

// TestEnsureClone_OperatorGlobalFsckAndAutocrlfDontBreakSubjectClone is
// VB-27's own reproduction of lca-run-1's real defect
// (workspace/logs/lca-run-1.log, 2026-09-17): tootallnate/java-websocket
// failed to clone with `fatal: fsck error in packed object` under this
// machine's real environment. newFsckViolatingBareRepo reproduces that
// exact class of malformed object hermetically; this test proves a global
// config carrying fetch.fsckObjects=true and core.autocrlf=input (the same
// class of leak core.autocrlf/safecrlf's own per-key override already
// covers for CRLF subjects, checkoutCommitIntoWorktree) does not affect
// ensureClone's own clone of it.
func TestEnsureClone_OperatorGlobalFsckAndAutocrlfDontBreakSubjectClone(t *testing.T) {
	bare := newFsckViolatingBareRepo(t)

	// Sanity: a PLAIN clone (no -c overrides, no isolation) of this fixture
	// fails exactly the way tootallnate/java-websocket did — confirms the
	// fixture actually reproduces the defect before testing the fix
	// against it. file:// forces the same fetch-pack/index-pack transport
	// codepath a real network clone uses (never the local hardlink
	// optimization a plain path would take), matching ensureClone's own
	// clone of an https:// URL.
	plainDest := filepath.Join(t.TempDir(), "plain")
	out, err := exec.Command("git", "clone", "--no-single-branch", "file://"+bare, plainDest).CombinedOutput()
	if err == nil {
		t.Fatalf("fixture bug: a plain clone of the malformed bare repo succeeded; want the zeroPaddedFilemode fsck failure this test means to reproduce (%s)", out)
	}
	if !strings.Contains(string(out), "zeroPaddedFilemode") {
		t.Fatalf("fixture bug: plain clone failed for an unexpected reason: %s", out)
	}

	// From here on, GIT_CONFIG_GLOBAL simulates the OPERATOR's own real
	// config for ensureClone's own subprocess only — the exact thing under
	// test, unlike the hermetic fixture setup above (which never goes
	// through ensureClone/runInEnv at all).
	globalConfig := filepath.Join(t.TempDir(), "gitconfig-fsck")
	if err := os.WriteFile(globalConfig, []byte("[fetch]\n\tfsckObjects = true\n[core]\n\tautocrlf = input\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("GIT_CONFIG_GLOBAL", globalConfig)

	cloneDir := filepath.Join(t.TempDir(), "clone")
	if err := ensureClone(context.Background(), "file://"+bare, cloneDir, t.TempDir()); err != nil {
		t.Fatalf("ensureClone: %v (this is the exact failure tootallnate/java-websocket hit in lca-run-1 under the operator's real global git config)", err)
	}
	if _, err := os.Stat(filepath.Join(cloneDir, ".git")); err != nil {
		t.Fatalf("cloneDir has no .git after ensureClone: %v", err)
	}
}

// TestRequiresCompiledLayer covers the O45 item 1 default: a task's own
// RequiresLayer wins when set; otherwise it defaults true for the
// lookup-corpus categories whose prompts assume a compiled layer, false
// otherwise.
func TestRequiresCompiledLayer(t *testing.T) {
	yes, no := true, false
	cases := []struct {
		name string
		task Task
		want bool
	}{
		{"obligations category defaults true", Task{Category: "obligations"}, true},
		{"staleness category defaults true", Task{Category: "staleness"}, true},
		{"claim_provenance category defaults true", Task{Category: "claim_provenance"}, true},
		{"verify category defaults true", Task{Category: "verify"}, true},
		{"blast_radius category defaults false", Task{Category: "blast_radius"}, false},
		{"unrelated category defaults false", Task{Category: "cross_package_feature"}, false},
		{"explicit false overrides an obligations default", Task{Category: "obligations", RequiresLayer: &no}, false},
		{"explicit true overrides a non-default category", Task{Category: "blast_radius", RequiresLayer: &yes}, true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := requiresCompiledLayer(c.task); got != c.want {
				t.Errorf("requiresCompiledLayer = %v, want %v", got, c.want)
			}
		})
	}
}

// fakeObligationsDaemon returns an httptest.Server standing in for the
// scratch daemon's /rpc/agent, replying to a tools/call "obligations"
// request with rules (a JSON array of arbitrary objects, one per rule).
func fakeObligationsDaemon(t *testing.T, rules []string) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		rulesJSON := "[" + strings.Join(rules, ",") + "]"
		fmt.Fprintf(w, `{"jsonrpc":"2.0","id":1,"result":{"service":"fathom","file":"internal/core/model.go","rules":%s}}`, rulesJSON)
	}))
}

func TestProbeCompiledLayerServesRules_RulesPresent(t *testing.T) {
	srv := fakeObligationsDaemon(t, []string{`{"id":"R1","text":"some rule"}`})
	defer srv.Close()

	ok, err := probeCompiledLayerServesRules(context.Background(), srv.URL, "fathom", "internal/core/model.go")
	if err != nil {
		t.Fatalf("probeCompiledLayerServesRules: %v", err)
	}
	if !ok {
		t.Error("expected ok=true when the fake daemon returns a rule")
	}
}

func TestProbeCompiledLayerServesRules_ZeroRules(t *testing.T) {
	srv := fakeObligationsDaemon(t, nil)
	defer srv.Close()

	ok, err := probeCompiledLayerServesRules(context.Background(), srv.URL, "fathom", "internal/core/model.go")
	if err != nil {
		t.Fatalf("probeCompiledLayerServesRules: %v", err)
	}
	if ok {
		t.Error("expected ok=false when the fake daemon returns zero rules")
	}
}

func TestProbeCompiledLayerServesRules_DaemonError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"jsonrpc":"2.0","id":1,"error":{"code":-32000,"message":"service not indexed"}}`)
	}))
	defer srv.Close()

	if _, err := probeCompiledLayerServesRules(context.Background(), srv.URL, "fathom", "internal/core/model.go"); err == nil {
		t.Fatal("expected an error when the daemon's own response carries an error")
	}
}

// TestApplyPostWireSetup_RunsSetupThenIngest proves the order O45 item 4
// requires: PostWireSetup's own commands run BEFORE runIngest is called —
// a fake "daemon" here is just runIngest recording what it saw, so no
// real binary or daemon is needed.
func TestApplyPostWireSetup_RunsSetupThenIngest(t *testing.T) {
	dir := t.TempDir()
	orderFile := filepath.Join(dir, "order.log")
	if err := os.WriteFile(orderFile, []byte("ingest-would-have-run-here\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	wt := Worktree{Path: dir, CellDir: dir}
	task := Task{Subject: Subject{PostWireSetup: []string{"echo drift >> order.log"}}}

	var reingested bool
	runIngest := func(ctx context.Context) error {
		reingested = true
		raw, err := os.ReadFile(orderFile)
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(string(raw), "ingest-would-have-run-here\ndrift") {
			t.Errorf("expected post_wire_setup's drift line AFTER the earlier ingest marker, got %q", raw)
		}
		return nil
	}
	if err := applyPostWireSetup(context.Background(), wt, task, runIngest); err != nil {
		t.Fatalf("applyPostWireSetup: %v", err)
	}
	if !reingested {
		t.Error("expected runIngest to be called for a wired cell")
	}
}

func TestApplyPostWireSetup_UnwiredCellSkipsReingest(t *testing.T) {
	dir := t.TempDir()
	wt := Worktree{Path: dir, CellDir: dir}
	task := Task{Subject: Subject{PostWireSetup: []string{"true"}}}
	if err := applyPostWireSetup(context.Background(), wt, task, nil); err != nil {
		t.Fatalf("applyPostWireSetup: %v", err)
	}
}

func TestApplyPostWireSetup_EmptyIsNoop(t *testing.T) {
	called := false
	wt := Worktree{Path: t.TempDir(), CellDir: t.TempDir()}
	if err := applyPostWireSetup(context.Background(), wt, Task{}, func(context.Context) error { called = true; return nil }); err != nil {
		t.Fatalf("applyPostWireSetup: %v", err)
	}
	if called {
		t.Error("expected no re-ingest when PostWireSetup is empty")
	}
}

func TestStopWorktree_NilAndZeroValueAreNoOps(t *testing.T) {
	if err := StopWorktree(nil); err != nil {
		t.Errorf("StopWorktree(nil): %v", err)
	}
	wt := Worktree{}
	if err := StopWorktree(&wt); err != nil {
		t.Errorf("StopWorktree on a zero-value Worktree: %v", err)
	}
}

// TestInstrumentOwnedPaths_ReadsTheLedgerPlusStatics pins O55 item 4:
// every file internal/wiring's own ledger (.fathom/owned.json) records a
// KEY for (KindKey — a structured file like .claude/settings.json's own
// hooks block, or another lefthook.yml-shaped entry) is instrument-owned,
// plus the two paths that carry no ledger entry of their own —
// .fathomignore and the ledger file itself. CLAUDE.md/AGENTS.md's own
// fathom stanza is deliberately NOT exercised here — it is a KindREGION,
// which apply.go never records in this ledger at all (fix-first review,
// 2026-09-14); TestInstrumentRegionRanges_* below covers that path.
func TestInstrumentOwnedPaths_ReadsTheLedgerPlusStatics(t *testing.T) {
	dir := t.TempDir()
	ledger := wiring.NewLedger()
	ledger.Record(".claude/settings.json", wiring.OwnedKey{ID: "hooks", Digest: "sha256:y", Wrote: "2026-09-14T00:00:00Z"})
	ledger.Record(".claude/skills/foo/SKILL.md", wiring.OwnedKey{ID: "file", Digest: "sha256:z", Wrote: "2026-09-14T00:00:00Z"})
	if err := wiring.SaveLedger(dir, ledger); err != nil {
		t.Fatal(err)
	}

	owned, err := InstrumentOwnedPaths(Worktree{Path: dir})
	if err != nil {
		t.Fatalf("InstrumentOwnedPaths: %v", err)
	}
	for _, want := range []string{".claude/settings.json", ".claude/skills/foo/SKILL.md", ".fathomignore", ".fathom/owned.json"} {
		if !owned[want] {
			t.Errorf("expected %q to be instrument-owned, got %v", want, owned)
		}
	}
	if owned["real_agent_file.go"] {
		t.Error("a file the ledger never mentions must not be reported owned")
	}
	if owned["CLAUDE.md"] {
		t.Error("CLAUDE.md must never be reported owned via the ledger — its stanza is a KindRegion, never ledger-tracked")
	}
}

// TestInstrumentRegionRanges_RealRegionFromWiring pins O55 item 4
// (fix-first review): InstrumentRegionRanges must find a region internal
// wiring's OWN production code actually wrote — wiring.ApplyWiring, never
// a hand-crafted marker string standing in for it — and report exactly the
// lines its own markers (wiring.HTMLMarkers) delimit.
// TestGitIgnoredWorkspacePaths_RealGitignore pins O55 item 4 (fix-first
// review) end to end: a real .gitignore that ignores one workspace/ path
// but not another is respected exactly — the ignored path is reported,
// real subject source under workspace/ is not (ADR-011: workspace/ is
// this repo's own convention, never assumed of an arbitrary subject).
func TestGitIgnoredWorkspacePaths_RealGitignore(t *testing.T) {
	dir := t.TempDir()
	run := func(args ...string) {
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		cmd.Env = append(
			gitEnv(),
			"GIT_AUTHOR_NAME=test", "GIT_AUTHOR_EMAIL=test@example.invalid",
			"GIT_COMMITTER_NAME=test", "GIT_COMMITTER_EMAIL=test@example.invalid",
		)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v (%s)", args, err, out)
		}
	}
	run("init", "-q", "-b", "main")
	writeTestFile(t, filepath.Join(dir, ".gitignore"), "workspace/artifacts/\n")
	run("add", "-A")
	run("commit", "-q", "-m", "base")

	diff := []byte("diff --git a/workspace/artifacts/scratch.json b/workspace/artifacts/scratch.json\n" +
		"new file mode 100644\n" +
		"--- /dev/null\n" +
		"+++ b/workspace/artifacts/scratch.json\n" +
		"@@ -0,0 +1 @@\n+{}\n" +
		"diff --git a/workspace/pkg/real_source.go b/workspace/pkg/real_source.go\n" +
		"new file mode 100644\n" +
		"--- /dev/null\n" +
		"+++ b/workspace/pkg/real_source.go\n" +
		"@@ -0,0 +1 @@\n+package pkg\n")

	ignored := GitIgnoredWorkspacePaths(context.Background(), Worktree{Path: dir}, diff)
	if !ignored["workspace/artifacts/scratch.json"] {
		t.Errorf("expected workspace/artifacts/scratch.json to be reported gitignored, got %v", ignored)
	}
	if ignored["workspace/pkg/real_source.go"] {
		t.Errorf("expected workspace/pkg/real_source.go (real subject source, not gitignored) to NOT be reported, got %v", ignored)
	}
}

// TestGitIgnoredWorkspacePaths_NoGitRepoReportsNothing covers the safe
// default: a git failure (no repository at all) reports no paths ignored
// rather than erroring — never a reason to silently drop a subject's real
// source.
func TestGitIgnoredWorkspacePaths_NoGitRepoReportsNothing(t *testing.T) {
	diff := []byte(unifiedDiffFile("workspace/pkg/real.go", "@@ -1 +1 @@\n-a\n+b\n"))
	ignored := GitIgnoredWorkspacePaths(context.Background(), Worktree{Path: t.TempDir()}, diff)
	if len(ignored) != 0 {
		t.Errorf("expected no ignored paths when wt.Path is not a git repo at all, got %v", ignored)
	}
}

func TestInstrumentRegionRanges_RealRegionFromWiring(t *testing.T) {
	dir := t.TempDir()
	fixedNow := func() time.Time { return time.Date(2026, 9, 14, 0, 0, 0, 0, time.UTC) }
	spec := wiring.Spec{Regions: []wiring.Region{{
		File: "CLAUDE.md", Syntax: wiring.HTMLMarkers, Version: "v3",
		Body: "line one of the real stanza\nline two of the real stanza",
	}}}
	if _, err := wiring.ApplyWiring(dir, spec, wiring.ApplyOptions{Now: fixedNow}); err != nil {
		t.Fatalf("wiring.ApplyWiring: %v", err)
	}

	ranges := InstrumentRegionRanges(context.Background(), Worktree{Path: dir})
	rng, ok := ranges["CLAUDE.md"]
	if !ok {
		t.Fatalf("expected CLAUDE.md to carry a detected region, got %v", ranges)
	}
	if rng.oldStart != 0 || rng.oldEnd != 0 {
		t.Errorf("expected no base-side region (no git repo/base commit here), got old=[%d,%d]", rng.oldStart, rng.oldEnd)
	}
	if rng.newStart == 0 || rng.newEnd < rng.newStart {
		t.Fatalf("expected a real new-side region range, got %+v", rng)
	}
	raw, err := os.ReadFile(filepath.Join(dir, "CLAUDE.md"))
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(string(raw), "\n")
	if !strings.HasPrefix(lines[rng.newStart-1], wiring.HTMLMarkers.BeginPrefix) {
		t.Errorf("line %d = %q, want the begin marker", rng.newStart, lines[rng.newStart-1])
	}
	if !strings.HasPrefix(lines[rng.newEnd-1], wiring.HTMLMarkers.End) {
		t.Errorf("line %d = %q, want the end marker", rng.newEnd, lines[rng.newEnd-1])
	}
}

// TestInstrumentOwnedPaths_NoLedgerIsJustTheStatics covers the ordinary
// unwired-arm case: no .fathom/owned.json at all is not an error, and the
// two static entries are still reported.
func TestInstrumentOwnedPaths_NoLedgerIsJustTheStatics(t *testing.T) {
	owned, err := InstrumentOwnedPaths(Worktree{Path: t.TempDir()})
	if err != nil {
		t.Fatalf("InstrumentOwnedPaths: %v", err)
	}
	if len(owned) != 2 || !owned[".fathomignore"] || !owned[".fathom/owned.json"] {
		t.Errorf("expected just the two static entries, got %v", owned)
	}
}

// TestObligationsProbeServesRules_RealEnvelope pins O51: a real tools/call
// response carries the tool's JSON inside result.content[].text; the first
// probe read only result.rules and so voided every layer-required wired
// cell against a real daemon. Both shapes must be read.
func TestObligationsProbeServesRules_RealEnvelope(t *testing.T) {
	cases := []struct {
		name string
		raw  string
		want bool
	}{
		{"real envelope with rules", `{"jsonrpc":"2.0","id":1,"result":{"content":[{"type":"text","text":"{\"file\":\"internal/core/model.go\",\"rules\":[{\"rule\":\"Sort outputs.\",\"src\":\"internal/CLAUDE.md#L3\"}]}"}]}}`, true},
		{"real envelope with zero rules", `{"jsonrpc":"2.0","id":1,"result":{"content":[{"type":"text","text":"{\"file\":\"x.go\",\"rules\":[]}"}]}}`, false},
		{"legacy flat result", `{"jsonrpc":"2.0","id":1,"result":{"rules":[{"rule":"x"}]}}`, true},
		{"empty result", `{"jsonrpc":"2.0","id":1,"result":{}}`, false},
	}
	for _, c := range cases {
		got, err := obligationsProbeServesRules([]byte(c.raw))
		if err != nil {
			t.Fatalf("%s: %v", c.name, err)
		}
		if got != c.want {
			t.Errorf("%s: served=%v, want %v", c.name, got, c.want)
		}
	}
	if _, err := obligationsProbeServesRules([]byte(`{"jsonrpc":"2.0","id":1,"error":{"code":-32000,"message":"service not indexed"}}`)); err == nil {
		t.Error("an error envelope must surface as an error, never as zero rules")
	}
}
