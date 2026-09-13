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

// TestInstrumentTruthError covers the gate directly: every wantServers
// entry must show "connected" in res.MCPServers, in order, else the
// FIRST mismatch (absent or non-connected) is reported.
func TestInstrumentTruthError(t *testing.T) {
	cases := []struct {
		name        string
		wantServers []string
		got         map[string]string
		wantMsg     string
	}{
		{"no servers declared", nil, nil, ""},
		{"declared and connected", []string{"fathom"}, map[string]string{"fathom": "connected"}, ""},
		{"declared but absent from the transcript", []string{"fathom"}, map[string]string{}, "instrument: mcp server fathom absent"},
		{"declared but not connected", []string{"graphify"}, map[string]string{"graphify": "failed"}, "instrument: mcp server graphify failed"},
		{"first of two mismatches wins (deterministic order)", []string{"a", "b"}, map[string]string{"a": "connected"}, "instrument: mcp server b absent"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := instrumentTruthError(c.wantServers, SessionResult{MCPServers: c.got})
			if got != c.wantMsg {
				t.Errorf("instrumentTruthError = %q, want %q", got, c.wantMsg)
			}
		})
	}
}

// TestRunSessions_InstrumentTruthGate_LosesTheCell is the DEFECT fix's own
// end-to-end proof at the RunSessions layer: a wired arm's declared MCP
// server that the transcript's own init event never reports connected
// must not pass as a reading — the session's Error is set so CellLost
// reports the cell as lost, exactly as an every-session-errored cell
// already does.
func TestRunSessions_InstrumentTruthGate_LosesTheCell(t *testing.T) {
	wtDir := t.TempDir()
	task := Task{ID: "T1", Prompt: "do the thing", Sessions: Sessions{Mode: ModeSingle, Count: 1, MaxTurns: 10}}
	// wt.Arm.MCPServerNames is what RenderArm actually wrote — set
	// directly here, standing in for what RenderArm would have produced
	// for a wired arm declaring "fathom" and "graphify".
	wt := Worktree{Path: wtDir, Arm: ArmFiles{MCPServerNames: []string{"fathom", "graphify"}}}
	h := &FakeHarness{
		StreamsDir: streamsDir,
		TaskID:     "T1",
		ArmName:    "full",
		// mcp-init.jsonl's own init event reports fathom connected but
		// graphify failed.
		Streams: map[string]string{"T1/full": "mcp-init.jsonl"},
	}
	opts := RunOptions{Trial: 1, Model: "sonnet", RawDir: filepath.Join(t.TempDir(), "raw")}

	results, err := RunSessions(context.Background(), h, task, "full", Arm{}, wt, opts)
	if err != nil {
		t.Fatalf("RunSessions: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected the single (failing) session, got %d", len(results))
	}
	wantErr := "instrument: mcp server graphify failed"
	if results[0].Error != wantErr {
		t.Errorf("Error = %q, want %q", results[0].Error, wantErr)
	}
	if reason := CellLost(results); reason == "" {
		t.Error("expected CellLost to report this cell as lost")
	}
}

// TestRunSessions_InstrumentTruthGate_AllConnectedIsNotLost is the
// converse: every declared server connected must leave the session's
// Error untouched.
func TestRunSessions_InstrumentTruthGate_AllConnectedIsNotLost(t *testing.T) {
	wtDir := t.TempDir()
	task := Task{ID: "T1", Prompt: "do the thing", Sessions: Sessions{Mode: ModeSingle, Count: 1, MaxTurns: 10}}
	wt := Worktree{Path: wtDir, Arm: ArmFiles{MCPServerNames: []string{"fathom"}}}
	h := &FakeHarness{
		StreamsDir: streamsDir,
		TaskID:     "T1",
		ArmName:    "full",
		Streams:    map[string]string{"T1/full": "mcp-init.jsonl"},
	}
	opts := RunOptions{Trial: 1, Model: "sonnet", RawDir: filepath.Join(t.TempDir(), "raw")}

	results, err := RunSessions(context.Background(), h, task, "full", Arm{}, wt, opts)
	if err != nil {
		t.Fatalf("RunSessions: %v", err)
	}
	if len(results) != 1 || results[0].Error != "" {
		t.Errorf("expected one clean session, got %+v", results)
	}
}

// TestRunSessions_PropagatesMCPConfigPath proves item 1's plumbing through
// the whole RunSessions loop: every session's request carries
// wt.Arm.MCPConfigPath, not an empty string a harness would silently fall
// back from.
func TestRunSessions_PropagatesMCPConfigPath(t *testing.T) {
	wtDir := t.TempDir()
	task := Task{ID: "T1", Prompt: "do the thing", Sessions: Sessions{Mode: ModeSingle, Count: 2, MaxTurns: 10}}
	wt := Worktree{Path: wtDir, Arm: ArmFiles{MCPConfigPath: "/rendered/scenariobench-mcp.json"}}
	h := &FakeHarness{
		StreamsDir: streamsDir, TaskID: "T1", ArmName: "full",
		Streams: map[string]string{"T1/full": "single-session.jsonl"},
	}
	opts := RunOptions{Trial: 1, Model: "sonnet", RawDir: filepath.Join(t.TempDir(), "raw")}

	if _, err := RunSessions(context.Background(), h, task, "full", Arm{}, wt, opts); err != nil {
		t.Fatalf("RunSessions: %v", err)
	}
	if len(h.MCPConfigPaths) != 2 {
		t.Fatalf("expected 2 recorded MCPConfigPaths, got %d: %v", len(h.MCPConfigPaths), h.MCPConfigPaths)
	}
	for i, got := range h.MCPConfigPaths {
		if got != wt.Arm.MCPConfigPath {
			t.Errorf("session %d: MCPConfigPath = %q, want %q", i, got, wt.Arm.MCPConfigPath)
		}
	}
}

// recordingHarness wraps FakeHarness and remembers every SessionRequest.Prompt
// it was asked to run, in call order, so a test can assert on the exact
// prompt text RunSessions builds without a real (or fake-transcript-blind)
// harness caring what that text is.
type recordingHarness struct {
	*FakeHarness
	prompts []string
}

func (h *recordingHarness) RunSession(req SessionRequest) (SessionResult, error) {
	h.prompts = append(h.prompts, req.Prompt)
	return h.FakeHarness.RunSession(req)
}

// TestRunSessions_SingleModeFirstSessionGetsPlanFirstInstruction is SB-XX:
// a single-session task never asked for a plan, so the M2/M3 (plan
// coverage, false-done) readings had nothing to read. The FIRST session's
// prompt must now carry the same PlanFirstInstruction planner-workers'
// planner gets; later sessions keep the cold-continuation prompt
// untouched.
func TestRunSessions_SingleModeFirstSessionGetsPlanFirstInstruction(t *testing.T) {
	task := Task{ID: "T1", Prompt: "do the thing", Sessions: Sessions{Mode: ModeSingle, Count: 2, MaxTurns: 10}}
	h := &recordingHarness{FakeHarness: &FakeHarness{
		StreamsDir: streamsDir, TaskID: "T1", ArmName: "full",
		Streams: map[string]string{"T1/full": "single-session.jsonl"},
	}}
	opts := RunOptions{Trial: 1, Model: "sonnet", RawDir: filepath.Join(t.TempDir(), "raw")}

	if _, err := RunSessions(context.Background(), h, task, "full", Arm{}, Worktree{Path: t.TempDir()}, opts); err != nil {
		t.Fatalf("RunSessions: %v", err)
	}
	if len(h.prompts) != 2 {
		t.Fatalf("expected 2 recorded prompts, got %d", len(h.prompts))
	}
	wantFirst := task.Prompt + PlanFirstInstruction
	if h.prompts[0] != wantFirst {
		t.Errorf("session 0 prompt = %q, want %q", h.prompts[0], wantFirst)
	}
	if h.prompts[1] != scenarioContinuationPrompt {
		t.Errorf("session 1 prompt = %q, want the unchanged continuation prompt %q", h.prompts[1], scenarioContinuationPrompt)
	}
}
