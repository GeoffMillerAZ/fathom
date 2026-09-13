package scenariobench

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
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
			os.Environ(),
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

func TestPrepareSubject_UnknownCommitIsAnError(t *testing.T) {
	repoPath, _ := gitFixtureRepo(t)
	root := t.TempDir()
	task := Task{ID: "T1", Subject: Subject{RepoURL: repoPath, BaseCommit: strings.Repeat("0", 40)}}
	if _, err := PrepareSubject(context.Background(), task, Arm{Instructions: "none"}, root); err == nil {
		t.Fatal("expected an error for a base_commit that does not exist")
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
