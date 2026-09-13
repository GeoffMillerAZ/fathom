package scenariobench

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCleanRoomEnv_SetsFathomThreadsAndCreatesCellDir(t *testing.T) {
	cellDir := filepath.Join(t.TempDir(), "cell")

	env, err := cleanRoomEnv(cellDir)
	if err != nil {
		t.Fatalf("cleanRoomEnv: %v", err)
	}
	wantThreads := filepath.Join(cellDir, "threads.jsonl")
	if env["FATHOM_THREADS"] != wantThreads {
		t.Errorf("FATHOM_THREADS = %q, want %q", env["FATHOM_THREADS"], wantThreads)
	}
	if len(env) != 1 {
		t.Errorf("expected only FATHOM_THREADS, got %v", env)
	}
	if _, err := os.Stat(cellDir); err != nil {
		t.Errorf("expected cellDir to be created: %v", err)
	}
}

func TestSessionEnv_EmptyCellDirSkipsCleanRoom(t *testing.T) {
	wt := Worktree{} // no CellDir — every test fixture that predates RC-6
	opts := RunOptions{Env: map[string]string{"FOO": "bar"}}
	env, err := sessionEnv(wt, opts)
	if err != nil {
		t.Fatalf("sessionEnv: %v", err)
	}
	if len(env) != 1 || env["FOO"] != "bar" {
		t.Errorf("expected opts.Env unchanged when CellDir is empty, got %v", env)
	}
}

func TestSessionEnv_MergesCleanRoomUnderExplicitEnv(t *testing.T) {
	wt := Worktree{CellDir: filepath.Join(t.TempDir(), "cell")}
	opts := RunOptions{
		Env: map[string]string{"FATHOM_THREADS": "explicit-wins", "OTHER": "kept"},
	}
	env, err := sessionEnv(wt, opts)
	if err != nil {
		t.Fatalf("sessionEnv: %v", err)
	}
	if env["FATHOM_THREADS"] != "explicit-wins" {
		t.Errorf("expected opts.Env to win on collision, got %q", env["FATHOM_THREADS"])
	}
	if env["OTHER"] != "kept" {
		t.Errorf("expected opts.Env's other keys preserved, got %v", env)
	}
}
