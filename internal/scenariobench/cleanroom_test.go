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

// TestSessionEnv_WiredCellSetsFathomHTTP is O45 item 6: a wired cell's own
// scratch daemon URL must reach every subprocess the session spawns
// (fathom's SessionStart/PostToolUse hooks, any `fathom` CLI call), or
// they silently talk to the operator's real daemon instead.
func TestSessionEnv_WiredCellSetsFathomHTTP(t *testing.T) {
	wt := Worktree{CellDir: filepath.Join(t.TempDir(), "cell"), DaemonURL: "http://127.0.0.1:53219"}
	env, err := sessionEnv(wt, RunOptions{})
	if err != nil {
		t.Fatalf("sessionEnv: %v", err)
	}
	if env["FATHOM_HTTP"] != wt.DaemonURL {
		t.Errorf("FATHOM_HTTP = %q, want %q", env["FATHOM_HTTP"], wt.DaemonURL)
	}
}

// TestSessionEnv_UnwiredCellSetsNoFathomHTTP is the converse: an unwired
// cell (no DaemonURL) must not set FATHOM_HTTP at all, so it keeps
// whatever the environment (or nothing) already resolves to.
func TestSessionEnv_UnwiredCellSetsNoFathomHTTP(t *testing.T) {
	wt := Worktree{CellDir: filepath.Join(t.TempDir(), "cell")}
	env, err := sessionEnv(wt, RunOptions{})
	if err != nil {
		t.Fatalf("sessionEnv: %v", err)
	}
	if _, ok := env["FATHOM_HTTP"]; ok {
		t.Errorf("expected no FATHOM_HTTP for an unwired cell, got %q", env["FATHOM_HTTP"])
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
