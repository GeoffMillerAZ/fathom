package scenariobench

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestAbsorbInstrumentFiles (O105): files written after the root commit are
// folded into it, LocalBase follows the amended root, and a detector-style
// `git diff --name-only $LocalBase` is empty until the session edits.
func TestAbsorbInstrumentFiles(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "main.go"), []byte("package main\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	env := map[string]string{
		"GIT_AUTHOR_NAME": "bench", "GIT_AUTHOR_EMAIL": "bench@example.invalid",
		"GIT_COMMITTER_NAME": "bench", "GIT_COMMITTER_EMAIL": "bench@example.invalid",
	}
	for _, args := range [][]string{{"init", "-q", "-b", "bench"}, {"add", "-A"}, {"-c", "commit.gpgsign=false", "commit", "-q", "-m", "base deadbeef"}} {
		if out, err := runInEnv(ctx, dir, env, "git", args...); err != nil {
			t.Fatalf("git %v: %v (%s)", args, err, out)
		}
	}
	wt := Worktree{Path: dir}
	root, err := runIn(ctx, dir, "git", "rev-list", "--max-parents=0", "HEAD")
	if err != nil {
		t.Fatal(err)
	}
	wt.LocalBase = strings.TrimSpace(root)

	// The instrument writes its wiring after the root commit.
	if err := os.MkdirAll(filepath.Join(dir, ".claude"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, ".claude", "settings.json"), []byte("{}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if out, _ := runIn(ctx, dir, "git", "diff", "--name-only", wt.LocalBase); strings.TrimSpace(out) != "" && !strings.Contains(out, "settings.json") {
		t.Fatalf("precondition: unexpected diff %q", out)
	}
	if err := absorbInstrumentFiles(ctx, &wt); err != nil {
		t.Fatal(err)
	}
	if out, err := runIn(ctx, dir, "git", "diff", "--name-only", wt.LocalBase); err != nil || strings.TrimSpace(out) != "" {
		t.Fatalf("after absorb: diff against LocalBase = %q err=%v, want empty", out, err)
	}
	if out, _ := runIn(ctx, dir, "git", "status", "--porcelain"); strings.TrimSpace(out) != "" {
		t.Fatalf("after absorb: working tree not clean: %q", out)
	}
	head, _ := runIn(ctx, dir, "git", "rev-parse", "HEAD")
	if strings.TrimSpace(head) != wt.LocalBase {
		t.Fatalf("LocalBase %s != HEAD %s", wt.LocalBase, strings.TrimSpace(head))
	}
	// The session's own edit is what a detector now sees.
	if err := os.WriteFile(filepath.Join(dir, "main.go"), []byte("package main\n\nfunc X() {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if out, _ := runIn(ctx, dir, "git", "diff", "--name-only", wt.LocalBase); strings.TrimSpace(out) != "main.go" {
		t.Fatalf("session diff = %q, want main.go only", out)
	}
}
