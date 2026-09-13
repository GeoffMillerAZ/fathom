package scenariobench

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// TestPrepareCalibrationWorktree_ClearsStaleRegistration: a killed run
// leaves the calibration worktree registered in git but missing on disk;
// `git worktree add` then refuses with "missing but already registered".
// The helper prunes the registration first (reuse-pilot-2, 2026-09-13).
func TestPrepareCalibrationWorktree_ClearsStaleRegistration(t *testing.T) {
	repo, sha := gitFixtureRepo(t)
	wt := filepath.Join(t.TempDir(), "wt-stale")
	if out, err := exec.Command("git", "-C", repo, "worktree", "add", "--detach", wt, sha).CombinedOutput(); err != nil {
		t.Fatalf("seeding worktree: %v (%s)", err, out)
	}
	if err := os.RemoveAll(wt); err != nil {
		t.Fatal(err)
	}
	if out, err := prepareCalibrationWorktree(context.Background(), repo, wt, sha); err != nil {
		t.Fatalf("expected the stale registration to be pruned, got %v (%s)", err, out)
	}
	if _, err := os.Stat(filepath.Join(wt, "marker.txt")); err != nil {
		t.Fatalf("expected a fresh checkout at %s: %v", wt, err)
	}
	_ = exec.Command("git", "-C", repo, "worktree", "remove", "--force", wt).Run()
}
