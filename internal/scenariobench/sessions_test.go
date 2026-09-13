package scenariobench

import (
	"context"
	"path/filepath"
	"testing"
)

// TestRunSessions_FakeHarnessReplays (SB-15) drives RunSessions' whole loop
// — both modes, cold sessions, plan-row reading — against committed
// transcript fixtures, with no spend.
func TestRunSessions_FakeHarnessReplays(t *testing.T) {
	t.Run("single mode runs Count cold sessions", func(t *testing.T) {
		wtDir := t.TempDir()
		task := Task{ID: "T1", Prompt: "do the thing", Sessions: Sessions{Mode: ModeSingle, Count: 2, MaxTurns: 10}}
		arm := Arm{}
		wt := Worktree{Path: wtDir, Arm: ArmFiles{SystemAppend: "keep notes"}}
		h := &FakeHarness{
			StreamsDir: streamsDir,
			TaskID:     "T1",
			ArmName:    "full",
			Streams:    map[string]string{"T1/full": "single-session.jsonl"},
		}
		opts := RunOptions{Trial: 1, Model: "sonnet", RawDir: filepath.Join(t.TempDir(), "raw")}

		results, err := RunSessions(context.Background(), h, task, "full", arm, wt, opts)
		if err != nil {
			t.Fatalf("RunSessions: %v", err)
		}
		if len(results) != 2 {
			t.Fatalf("expected 2 sessions, got %d", len(results))
		}
		if results[0].Role != "single" || results[0].Index != 0 {
			t.Errorf("session 0: role=%q index=%d, want single/0", results[0].Role, results[0].Index)
		}
		if results[1].Role != "single" || results[1].Index != 1 {
			t.Errorf("session 1: role=%q index=%d, want single/1", results[1].Role, results[1].Index)
		}
		// Both sessions replay the same fixture (a real cold rerun would
		// differ session to session, but the fixture identity is what this
		// test is checking, not the model's behavior).
		if results[0].Turns == 0 || results[1].Turns == 0 {
			t.Error("expected non-zero Turns from the replayed transcript")
		}
	})

	t.Run("planner-workers mode reads rows from the planner's plan file", func(t *testing.T) {
		wtDir := t.TempDir()
		// FakeHarness never touches disk (it only replays a canned
		// SessionResult), so the planner's own effect — NOTES.md — is
		// pre-placed here to stand in for what a real planner session
		// would have written.
		writeTestFile(t, filepath.Join(wtDir, "NOTES.md"),
			"# Plan\n\n- [ ] {id: row-1} do thing one\n- [ ] {id: row-2} do thing two\n")

		task := Task{ID: "T2", Prompt: "do the thing", Sessions: Sessions{Mode: ModePlannerWorkers, Workers: 2, MaxTurns: 10}}
		arm := Arm{}
		wt := Worktree{Path: wtDir}
		h := &FakeHarness{
			StreamsDir: streamsDir,
			TaskID:     "T2",
			ArmName:    "full",
			Streams: map[string]string{
				"T2/full/planner": "planner.jsonl",
				"T2/full/worker":  "worker.jsonl",
			},
		}
		opts := RunOptions{Trial: 1, Model: "sonnet", RawDir: filepath.Join(t.TempDir(), "raw")}

		results, err := RunSessions(context.Background(), h, task, "full", arm, wt, opts)
		if err != nil {
			t.Fatalf("RunSessions: %v", err)
		}
		if len(results) != 3 {
			t.Fatalf("expected 1 planner + 2 workers = 3 sessions, got %d", len(results))
		}
		if results[0].Role != "planner" || results[0].Index != 0 {
			t.Errorf("session 0: role=%q index=%d, want planner/0", results[0].Role, results[0].Index)
		}
		if results[1].Role != "worker" || results[1].RowID != "row-1" {
			t.Errorf("session 1: role=%q rowID=%q, want worker/row-1", results[1].Role, results[1].RowID)
		}
		if results[2].Role != "worker" || results[2].RowID != "row-2" {
			t.Errorf("session 2: role=%q rowID=%q, want worker/row-2", results[2].Role, results[2].RowID)
		}
	})

	t.Run("planner-workers mode caps workers at Sessions.Workers", func(t *testing.T) {
		wtDir := t.TempDir()
		writeTestFile(t, filepath.Join(wtDir, "NOTES.md"),
			"- [ ] {id: row-1} a\n- [ ] {id: row-2} b\n- [ ] {id: row-3} c\n")
		task := Task{ID: "T3", Prompt: "p", Sessions: Sessions{Mode: ModePlannerWorkers, Workers: 1, MaxTurns: 10}}
		wt := Worktree{Path: wtDir}
		h := &FakeHarness{
			StreamsDir: streamsDir, TaskID: "T3", ArmName: "full",
			Streams: map[string]string{"T3/full/planner": "planner.jsonl", "T3/full/worker": "worker.jsonl"},
		}
		opts := RunOptions{Trial: 1, RawDir: filepath.Join(t.TempDir(), "raw")}

		results, err := RunSessions(context.Background(), h, task, "full", Arm{}, wt, opts)
		if err != nil {
			t.Fatalf("RunSessions: %v", err)
		}
		if len(results) != 2 {
			t.Fatalf("expected 1 planner + 1 worker (capped by Workers=1) = 2, got %d", len(results))
		}
	})

	t.Run("no plan file means no workers, not an error", func(t *testing.T) {
		wtDir := t.TempDir()
		task := Task{ID: "T4", Prompt: "p", Sessions: Sessions{Mode: ModePlannerWorkers, Workers: 2, MaxTurns: 10}}
		wt := Worktree{Path: wtDir}
		// planner.jsonl's own transcript claims a Write to NOTES.md, but
		// FakeHarness never actually performs it — so with no plan file on
		// disk, readPlanRows correctly finds nothing to assign.
		h := &FakeHarness{
			StreamsDir: streamsDir, TaskID: "T4", ArmName: "full",
			Streams: map[string]string{"T4/full/planner": "planner.jsonl"},
		}
		opts := RunOptions{Trial: 1, RawDir: filepath.Join(t.TempDir(), "raw")}

		results, err := RunSessions(context.Background(), h, task, "full", Arm{}, wt, opts)
		if err != nil {
			t.Fatalf("RunSessions: %v", err)
		}
		if len(results) != 1 {
			t.Fatalf("expected only the planner session (no plan file to assign workers from), got %d", len(results))
		}
	})

	t.Run("unknown mode is an error", func(t *testing.T) {
		task := Task{ID: "T5", Sessions: Sessions{Mode: "bogus"}}
		if _, err := RunSessions(context.Background(), &FakeHarness{}, task, "full", Arm{}, Worktree{}, RunOptions{}); err == nil {
			t.Fatal("expected an error for an unknown session mode")
		}
	})
}
