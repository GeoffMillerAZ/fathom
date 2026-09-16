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
	// Wired: true so baselineIsolationEnv (O53 defect 4) adds nothing here
	// — this test is about the clean room's OWN merge behavior with no
	// CellDir, not the baseline isolation feature (covered separately in
	// baseline_test.go).
	env, err := sessionEnv(Arm{Wired: true}, wt, opts)
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
	env, err := sessionEnv(Arm{Wired: true}, wt, RunOptions{})
	if err != nil {
		t.Fatalf("sessionEnv: %v", err)
	}
	if env["FATHOM_HTTP"] != wt.DaemonURL {
		t.Errorf("FATHOM_HTTP = %q, want %q", env["FATHOM_HTTP"], wt.DaemonURL)
	}
}

// TestSessionEnv_UnwiredCellSetsUnroutableFathomHTTP is O53 defect 4's own
// proof at the sessionEnv layer: an unwired cell must NOT keep whatever
// FATHOM_HTTP the environment already resolves to (the defect — a live
// operator daemon at the default localhost:7575 was reachable from a
// baseline arm) — it must be pinned to the unroutable placeholder.
func TestSessionEnv_UnwiredCellSetsUnroutableFathomHTTP(t *testing.T) {
	wt := Worktree{CellDir: filepath.Join(t.TempDir(), "cell")}
	env, err := sessionEnv(Arm{Wired: false}, wt, RunOptions{})
	if err != nil {
		t.Fatalf("sessionEnv: %v", err)
	}
	if env["FATHOM_HTTP"] != unroutableFathomURL {
		t.Errorf("FATHOM_HTTP = %q, want %q", env["FATHOM_HTTP"], unroutableFathomURL)
	}
	if env["FATHOM_HTTP_URL"] != unroutableFathomURL {
		t.Errorf("FATHOM_HTTP_URL = %q, want %q", env["FATHOM_HTTP_URL"], unroutableFathomURL)
	}
}

func TestSessionEnv_MergesCleanRoomUnderExplicitEnv(t *testing.T) {
	wt := Worktree{CellDir: filepath.Join(t.TempDir(), "cell")}
	opts := RunOptions{
		Env: map[string]string{"FATHOM_THREADS": "explicit-wins", "OTHER": "kept"},
	}
	env, err := sessionEnv(Arm{Wired: true}, wt, opts)
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
