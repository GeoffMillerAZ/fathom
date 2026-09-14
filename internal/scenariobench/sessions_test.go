package scenariobench

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
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

	// O55 item 3: a worker, once it finishes its row, is followed by a
	// new worker session on the next unchecked row — Workers=1 no longer
	// caps how many ROWS get a session, only that the driver runs them
	// one at a time. With MaxWorkersTotal unset, the default cap
	// (Workers*3 = 3) exactly covers this plan's 3 rows, so all three get
	// assigned in row order.
	t.Run("planner-workers mode reassigns a worker to every row, not just Sessions.Workers of them", func(t *testing.T) {
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
		if len(results) != 4 {
			t.Fatalf("expected 1 planner + 3 workers (all rows assigned) = 4, got %d", len(results))
		}
		wantRowIDs := []string{"row-1", "row-2", "row-3"}
		for i, want := range wantRowIDs {
			got := results[i+1]
			if got.Role != "worker" || got.RowID != want {
				t.Errorf("session %d: role=%q rowID=%q, want worker/%s", i+1, got.Role, got.RowID, want)
			}
		}
		// O55 item 3 (fix-first review): reassignment is tracked in memory
		// now, never by writing into the plan file — FakeHarness never
		// touches disk, so every row is STILL literally unchecked on disk
		// even though all three were assigned. Writing any of them would
		// be the very regression this test now guards against.
		n, err := RemainingPlanRows(wtDir)
		if err != nil {
			t.Fatalf("RemainingPlanRows: %v", err)
		}
		if n != 3 {
			t.Errorf("expected the driver to leave the plan file untouched (3 still unchecked on disk), got %d", n)
		}
	})

	// The explicit fixture from the finding: a 5-row plan and workers=2
	// assigns all 5 (default MaxWorkersTotal = 2*3 = 6 >= 5).
	t.Run("a 5-row plan and workers=2 assigns all 5", func(t *testing.T) {
		wtDir := t.TempDir()
		writeTestFile(t, filepath.Join(wtDir, "NOTES.md"),
			"- [ ] {id: row-1} a\n- [ ] {id: row-2} b\n- [ ] {id: row-3} c\n- [ ] {id: row-4} d\n- [ ] {id: row-5} e\n")
		task := Task{ID: "T3b", Prompt: "p", Sessions: Sessions{Mode: ModePlannerWorkers, Workers: 2, MaxTurns: 10}}
		wt := Worktree{Path: wtDir}
		h := &FakeHarness{
			StreamsDir: streamsDir, TaskID: "T3b", ArmName: "full",
			Streams: map[string]string{"T3b/full/planner": "planner.jsonl", "T3b/full/worker": "worker.jsonl"},
		}
		opts := RunOptions{Trial: 1, RawDir: filepath.Join(t.TempDir(), "raw")}

		results, err := RunSessions(context.Background(), h, task, "full", Arm{}, wt, opts)
		if err != nil {
			t.Fatalf("RunSessions: %v", err)
		}
		if len(results) != 6 {
			t.Fatalf("expected 1 planner + 5 workers = 6, got %d", len(results))
		}
		workers := 0
		for _, r := range results {
			if r.Role == "worker" {
				workers++
			}
		}
		if workers != 5 {
			t.Errorf("expected all 5 rows assigned a worker, got %d", workers)
		}
	})

	// MaxWorkersTotal, when set, caps total worker sessions below what the
	// plan's own row count would otherwise get — the row(s) past the cap
	// stay unassigned, and RunSessions reports no error (a budget cap is
	// not a failure).
	t.Run("MaxWorkersTotal caps total worker sessions", func(t *testing.T) {
		wtDir := t.TempDir()
		writeTestFile(t, filepath.Join(wtDir, "NOTES.md"),
			"- [ ] {id: row-1} a\n- [ ] {id: row-2} b\n- [ ] {id: row-3} c\n")
		task := Task{ID: "T3c", Prompt: "p", Sessions: Sessions{Mode: ModePlannerWorkers, Workers: 1, MaxWorkersTotal: 2, MaxTurns: 10}}
		wt := Worktree{Path: wtDir}
		h := &FakeHarness{
			StreamsDir: streamsDir, TaskID: "T3c", ArmName: "full",
			Streams: map[string]string{"T3c/full/planner": "planner.jsonl", "T3c/full/worker": "worker.jsonl"},
		}
		opts := RunOptions{Trial: 1, RawDir: filepath.Join(t.TempDir(), "raw")}

		results, err := RunSessions(context.Background(), h, task, "full", Arm{}, wt, opts)
		if err != nil {
			t.Fatalf("RunSessions: %v", err)
		}
		if len(results) != 3 {
			t.Fatalf("expected 1 planner + 2 workers (capped by MaxWorkersTotal=2) = 3, got %d", len(results))
		}
		wantRowIDs := []string{"row-1", "row-2"}
		for i, want := range wantRowIDs {
			got := results[i+1]
			if got.Role != "worker" || got.RowID != want {
				t.Errorf("session %d: role=%q rowID=%q, want worker/%s", i+1, got.Role, got.RowID, want)
			}
		}
		// The driver never writes into the plan file (O55 item 3, fix-first
		// review) — all 3 rows are still unchecked on disk regardless of
		// the cap; row-3 was simply never assigned, which the session
		// count above already proves.
		n, err := RemainingPlanRows(wtDir)
		if err != nil {
			t.Fatalf("RemainingPlanRows: %v", err)
		}
		if n != 3 {
			t.Errorf("expected the driver to leave the plan file untouched (3 still unchecked on disk), got %d", n)
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

// checkingHarness wraps a FakeHarness and, after a worker session, checks
// off the first still-unchecked row in the plan file — standing in for a
// REAL harness's own file edits (FakeHarness itself never touches disk):
// the worker prompt (workerRowPromptFmt) instructs the agent to mark its
// own row `[x]` when finished, exactly what a real agent following that
// instruction would do to the tree. This is what
// TestRunSessions_WorkerCheckingItsOwnRowIsNeverReassignedOrAnError
// exercises — the O55 (fix-first review) regression: a first attempt at
// item 3 had the DRIVER also rewrite that same box, which errored when a
// worker had already checked it, and that error was not errSubjectPrep, so
// `scenariobench run` hard-exited 2 and discarded every other cell.
type checkingHarness struct {
	t *testing.T
	*FakeHarness
}

func (h *checkingHarness) RunSession(req SessionRequest) (SessionResult, error) {
	res, err := h.FakeHarness.RunSession(req)
	if err != nil || req.Role != "worker" {
		return res, err
	}
	checkFirstUncheckedRow(h.t, req.Worktree)
	return res, nil
}

// checkFirstUncheckedRow marks the plan file's own first unchecked row
// `[x]` — the real-harness-shaped side effect checkingHarness simulates.
func checkFirstUncheckedRow(t *testing.T, worktree string) {
	t.Helper()
	rows, err := readPlanRows(worktree)
	if err != nil {
		t.Fatalf("readPlanRows: %v", err)
	}
	if len(rows) == 0 {
		return
	}
	row := rows[0]
	path := filepath.Join(worktree, filepath.FromSlash(row.file))
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading plan file %s: %v", path, err)
	}
	lines := strings.Split(string(raw), "\n")
	for i, line := range lines {
		m := planRowRE.FindStringSubmatch(line)
		if m == nil || strings.TrimSpace(m[1]) != row.text {
			continue
		}
		idx := strings.Index(line, "[ ]")
		if idx < 0 {
			continue
		}
		lines[i] = line[:idx] + "[x]" + line[idx+3:]
		if err := os.WriteFile(path, []byte(strings.Join(lines, "\n")), 0o644); err != nil {
			t.Fatalf("writing plan file %s: %v", path, err)
		}
		return
	}
	t.Fatalf("row %q not found verbatim in %s", row.text, path)
}

// TestRunSessions_WorkerCheckingItsOwnRowIsNeverReassignedOrAnError pins
// the O55 fix-first review defect directly: a worker that does what its
// own prompt instructs (checks its row `[x]`) must never make
// runPlannerWorkers error, and the row it checked must never be handed to
// a second worker — the driver reads the tree's own state (readPlanRows)
// rather than writing to it.
func TestRunSessions_WorkerCheckingItsOwnRowIsNeverReassignedOrAnError(t *testing.T) {
	wtDir := t.TempDir()
	writeTestFile(t, filepath.Join(wtDir, "NOTES.md"),
		"- [ ] {id: row-1} a\n- [ ] {id: row-2} b\n")
	task := Task{ID: "T6", Prompt: "p", Sessions: Sessions{Mode: ModePlannerWorkers, Workers: 2, MaxTurns: 10}}
	wt := Worktree{Path: wtDir}
	fake := &FakeHarness{
		StreamsDir: streamsDir, TaskID: "T6", ArmName: "full",
		Streams: map[string]string{"T6/full/planner": "planner.jsonl", "T6/full/worker": "worker.jsonl"},
	}
	h := &checkingHarness{t: t, FakeHarness: fake}
	opts := RunOptions{Trial: 1, RawDir: filepath.Join(t.TempDir(), "raw")}

	results, err := RunSessions(context.Background(), h, task, "full", Arm{}, wt, opts)
	if err != nil {
		t.Fatalf("RunSessions: %v", err)
	}
	if len(results) != 3 {
		t.Fatalf("expected 1 planner + 2 workers = 3, got %d", len(results))
	}
	wantRowIDs := []string{"row-1", "row-2"}
	for i, want := range wantRowIDs {
		got := results[i+1]
		if got.Role != "worker" || got.RowID != want {
			t.Errorf("session %d: role=%q rowID=%q, want worker/%s", i+1, got.Role, got.RowID, want)
		}
	}
	// Both rows were genuinely checked (by the harness, standing in for
	// the agent) — nothing left unchecked, and no row was ever assigned
	// twice (which len(results)==3 and the two distinct rowIDs above
	// already rule out).
	n, err := RemainingPlanRows(wtDir)
	if err != nil {
		t.Fatalf("RemainingPlanRows: %v", err)
	}
	if n != 0 {
		t.Errorf("expected both rows checked by the harness, %d still unchecked", n)
	}
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

// TestHooksInstrumentError covers hooksInstrumentError directly: only a
// wired session with no SessionStart entry in HooksFired is an instrument
// error; an unwired arm never is, regardless of HooksFired.
func TestHooksInstrumentError(t *testing.T) {
	cases := []struct {
		name    string
		wired   bool
		fired   map[string]int
		wantMsg string
	}{
		{"unwired, no hooks at all", false, nil, ""},
		{"wired, SessionStart fired", true, map[string]int{"SessionStart": 1}, ""},
		{"wired, nil HooksFired", true, nil, "instrument: no SessionStart hook fired"},
		{"wired, HooksFired present but no SessionStart", true, map[string]int{"PostToolUse": 1}, "instrument: no SessionStart hook fired"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := hooksInstrumentError(c.wired, SessionResult{HooksFired: c.fired})
			if got != c.wantMsg {
				t.Errorf("hooksInstrumentError = %q, want %q", got, c.wantMsg)
			}
		})
	}
}

// TestRunSessions_WiredArmNoSessionStartHook_LosesTheCell is the hooks
// instrument-truth gate's own end-to-end proof: a wired arm whose
// transcript never shows a SessionStart hook_response must not pass as a
// reading, exactly like an absent MCP server.
func TestRunSessions_WiredArmNoSessionStartHook_LosesTheCell(t *testing.T) {
	wtDir := t.TempDir()
	task := Task{ID: "T1", Prompt: "do the thing", Sessions: Sessions{Mode: ModeSingle, Count: 1, MaxTurns: 10}}
	wt := Worktree{Path: wtDir}
	h := &FakeHarness{
		StreamsDir: streamsDir,
		TaskID:     "T1",
		ArmName:    "full",
		// single-session.jsonl carries no hook_response event at all.
		Streams: map[string]string{"T1/full": "single-session.jsonl"},
	}
	opts := RunOptions{Trial: 1, Model: "sonnet", RawDir: filepath.Join(t.TempDir(), "raw")}

	results, err := RunSessions(context.Background(), h, task, "full", Arm{Wired: true}, wt, opts)
	if err != nil {
		t.Fatalf("RunSessions: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected the single (failing) session, got %d", len(results))
	}
	wantErr := "instrument: no SessionStart hook fired"
	if results[0].Error != wantErr {
		t.Errorf("Error = %q, want %q", results[0].Error, wantErr)
	}
	if results[0].InstrumentError != wantErr {
		t.Errorf("InstrumentError = %q, want %q", results[0].InstrumentError, wantErr)
	}
	if reason := CellLost(results); reason == "" {
		t.Error("expected CellLost to report this cell as lost")
	}
}

// TestRunSessions_WiredArmSessionStartHookFired_NotLost is the converse: a
// wired arm whose transcript shows the SessionStart hook firing must leave
// the session clean.
func TestRunSessions_WiredArmSessionStartHookFired_NotLost(t *testing.T) {
	wtDir := t.TempDir()
	task := Task{ID: "T1", Prompt: "do the thing", Sessions: Sessions{Mode: ModeSingle, Count: 1, MaxTurns: 10}}
	wt := Worktree{Path: wtDir}
	h := &FakeHarness{
		StreamsDir: streamsDir,
		TaskID:     "T1",
		ArmName:    "full",
		Streams:    map[string]string{"T1/full": "hooks-fired.jsonl"},
	}
	opts := RunOptions{Trial: 1, Model: "sonnet", RawDir: filepath.Join(t.TempDir(), "raw")}

	results, err := RunSessions(context.Background(), h, task, "full", Arm{Wired: true}, wt, opts)
	if err != nil {
		t.Fatalf("RunSessions: %v", err)
	}
	if len(results) != 1 || results[0].Error != "" || results[0].InstrumentError != "" {
		t.Errorf("expected one clean session, got %+v", results)
	}
}

// TestCellLost_AnySessionInstrumentErrorLosesCell is finding 3's own proof:
// an instrument error on ONE session (here, session 0 lost its server)
// must lose the whole cell even though session 1 ran cleanly with turns —
// the pre-fix code only lost a cell when EVERY session errored.
func TestCellLost_AnySessionInstrumentErrorLosesCell(t *testing.T) {
	sessions := []SessionResult{
		{Error: "instrument: mcp server fathom absent", InstrumentError: "instrument: mcp server fathom absent"},
		{Turns: 3},
	}
	if reason := CellLost(sessions); reason == "" {
		t.Error("expected CellLost to report this cell as lost when any session carries an InstrumentError")
	}
}

// TestCellLost_NoInstrumentErrorUnaffected is the converse: with no
// InstrumentError anywhere, CellLost's pre-existing behavior (every
// session errored, or no turns at all) is unchanged.
func TestCellLost_NoInstrumentErrorUnaffected(t *testing.T) {
	sessions := []SessionResult{{Turns: 1}, {Error: "some in-band failure", Turns: 1}}
	if reason := CellLost(sessions); reason != "" {
		t.Errorf("CellLost = %q, want empty (not every session errored, no instrument error)", reason)
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

// TestRunSessions_SingleModeFirstSessionGetsPlanFirstInstruction is SB-XX,
// narrowed by O45 item 2: a single-session CHANGE task (here, one naming
// Gold.Files) never asked for a plan, so the M2/M3 (plan coverage,
// false-done) readings had nothing to read. The FIRST session's prompt
// must now carry the same PlanFirstInstruction planner-workers' planner
// gets; later sessions keep the cold-continuation prompt untouched.
func TestRunSessions_SingleModeFirstSessionGetsPlanFirstInstruction(t *testing.T) {
	task := Task{
		ID: "T1", Prompt: "do the thing",
		Sessions: Sessions{Mode: ModeSingle, Count: 2, MaxTurns: 10},
		Gold:     Gold{Files: []string{"foo.go"}},
	}
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

// TestRunSessions_SingleModeOneSessionPlansThenImplements: a change-shaped
// task with ONE session must not be told that later sessions will implement
// its rows — it gets PlanThenImplementInstruction instead (clean room v3's
// dry cell, 2026-09-13: the wired arm wrote a plan and stopped).
func TestRunSessions_SingleModeOneSessionPlansThenImplements(t *testing.T) {
	task := Task{
		ID: "T1", Prompt: "do the thing",
		Sessions: Sessions{Mode: ModeSingle, Count: 1, MaxTurns: 10},
		Gold:     Gold{Files: []string{"a.go"}},
	}
	for _, wired := range []bool{false, true} {
		h := &recordingHarness{FakeHarness: &FakeHarness{
			StreamsDir: streamsDir, TaskID: "T1", ArmName: "full",
			Streams: map[string]string{"T1/full": "single-session.jsonl"},
		}}
		opts := RunOptions{Trial: 1, Model: "sonnet", RawDir: filepath.Join(t.TempDir(), "raw")}
		arm := Arm{Wired: wired}
		if _, err := RunSessions(context.Background(), h, task, "full", arm, Worktree{Path: t.TempDir()}, opts); err != nil {
			t.Fatalf("wired=%v: RunSessions: %v", wired, err)
		}
		if len(h.prompts) != 1 {
			t.Fatalf("wired=%v: %d prompts, want 1", wired, len(h.prompts))
		}
		if !strings.HasPrefix(h.prompts[0], task.Prompt+PlanThenImplementInstruction) {
			t.Errorf("wired=%v: prompt = %q, want the plan-then-implement instruction", wired, h.prompts[0])
		}
		if strings.Contains(h.prompts[0], "later sessions") {
			t.Errorf("wired=%v: a one-session task was told later sessions exist", wired)
		}
		if wired != strings.Contains(h.prompts[0], "fathom verify") {
			t.Errorf("wired=%v: grammar addendum presence wrong", wired)
		}
	}
}

// TestRunSessions_SingleModeLookupTaskSkipsPlanFirstInstruction is O45 item
// 2's own converse: a pure-lookup task (Oracle.AnswerAllOf, no Gold files,
// no test patch, no detectors — SB-32's lookups-vs.json shape) has nothing
// to plan, so its first session's prompt must be the task prompt verbatim
// — appending PlanFirstInstruction here only spent the turn budget writing
// a plan the answer-only oracle never reads.
func TestRunSessions_SingleModeLookupTaskSkipsPlanFirstInstruction(t *testing.T) {
	task := Task{
		ID: "T1", Prompt: "what does FreshnessMarkerValue return?",
		Sessions: Sessions{Mode: ModeSingle, Count: 1, MaxTurns: 10},
		Oracle:   Oracle{AnswerAllOf: []string{"42"}},
	}
	h := &recordingHarness{FakeHarness: &FakeHarness{
		StreamsDir: streamsDir, TaskID: "T1", ArmName: "full",
		Streams: map[string]string{"T1/full": "single-session.jsonl"},
	}}
	opts := RunOptions{Trial: 1, Model: "sonnet", RawDir: filepath.Join(t.TempDir(), "raw")}

	if _, err := RunSessions(context.Background(), h, task, "full", Arm{}, Worktree{Path: t.TempDir()}, opts); err != nil {
		t.Fatalf("RunSessions: %v", err)
	}
	if len(h.prompts) != 1 {
		t.Fatalf("expected 1 recorded prompt, got %d", len(h.prompts))
	}
	if h.prompts[0] != task.Prompt {
		t.Errorf("session 0 prompt = %q, want the task prompt verbatim %q (no plan-first instruction for a lookup task)", h.prompts[0], task.Prompt)
	}
}

// TestPrimerInstrumentError covers primerInstrumentError directly: only a
// wired session whose SessionStart primer either names "not an indexed
// service" or fails to name any indexed service at all is an instrument
// error; an unwired arm never is, and a wired session with no SessionStart
// hook at all defers to hooksInstrumentError (returns "" here).
func TestPrimerInstrumentError(t *testing.T) {
	cases := []struct {
		name    string
		wired   bool
		fired   map[string]int
		primer  string
		wantMsg string
	}{
		{"unwired, any primer text", false, map[string]int{"SessionStart": 1}, "this repo (\"x\") is not an indexed service", ""},
		{"wired, no SessionStart hook at all", true, nil, "", ""},
		{
			"wired, primer names the indexed service", true,
			map[string]int{"SessionStart": 1},
			`fathom code-intelligence is live — this repo is the indexed service "fathom" (100 symbols)`, "",
		},
		{
			"wired, primer reports not indexed", true,
			map[string]int{"SessionStart": 1},
			`fathom code-intelligence is live, but this repo ("1b8765d-abc123") is not an indexed service — its symbols aren't in the index`,
			"instrument: SessionStart primer reports this worktree is not an indexed service",
		},
		{
			"wired, primer names no service at all (daemon down)", true,
			map[string]int{"SessionStart": 1},
			"fathom code-intelligence daemon is not responding",
			"instrument: SessionStart primer named no indexed service",
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := primerInstrumentError(c.wired, SessionResult{HooksFired: c.fired, SessionStartContext: c.primer})
			if got != c.wantMsg {
				t.Errorf("primerInstrumentError = %q, want %q", got, c.wantMsg)
			}
		})
	}
}

// TestRunSessions_WiredArmNotIndexedPrimer_LosesTheCell is the primer
// instrument-truth gate's own end-to-end proof (O45 item 7): a wired
// session whose transcript's own SessionStart hook_response says the
// worktree is not an indexed service must not pass as a reading.
func TestRunSessions_WiredArmNotIndexedPrimer_LosesTheCell(t *testing.T) {
	dir := t.TempDir()
	notIndexedEnvelope := mustJSON(map[string]any{
		"hookSpecificOutput": map[string]any{
			"hookEventName":     "SessionStart",
			"additionalContext": `fathom code-intelligence is live, but this repo ("wt") is not an indexed service — its symbols aren't in the index`,
		},
	})
	path := writeRawLines(t, dir, "session.jsonl", []string{
		mustJSON(map[string]any{
			"type": "system", "subtype": "hook_response", "hook_event": "SessionStart", "output": notIndexedEnvelope,
		}),
		mustJSON(map[string]any{
			"type": "result", "is_error": false, "num_turns": 1, "duration_ms": 10, "total_cost_usd": 0.001,
		}),
	})
	streamName := filepath.Base(path)
	task := Task{ID: "T1", Prompt: "do the thing", Sessions: Sessions{Mode: ModeSingle, Count: 1, MaxTurns: 10}}
	wt := Worktree{Path: t.TempDir(), Arm: ArmFiles{MCPServerNames: nil}}
	h := &FakeHarness{
		StreamsDir: dir, TaskID: "T1", ArmName: "full",
		Streams: map[string]string{"T1/full": streamName},
	}
	opts := RunOptions{Trial: 1, Model: "sonnet", RawDir: filepath.Join(t.TempDir(), "raw")}

	results, err := RunSessions(context.Background(), h, task, "full", Arm{Wired: true}, wt, opts)
	if err != nil {
		t.Fatalf("RunSessions: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 session, got %d", len(results))
	}
	if !strings.Contains(results[0].Error, "not an indexed service") {
		t.Errorf("Error = %q, want it to mention the not-indexed primer", results[0].Error)
	}
	if reason := CellLost(results); reason == "" {
		t.Error("expected CellLost to report this cell as lost")
	}
}

// mustJSON marshals v — panics on error rather than failing a *testing.T,
// since every call site here passes a fixed, known-good literal.
func mustJSON(v any) string {
	b, err := json.Marshal(v)
	if err != nil {
		panic(err)
	}
	return string(b)
}

// TestPostToolUseInstrumentError covers postToolUseInstrumentError
// directly (clean room v3): only a wired session that edited a file with
// no PostToolUse entry in HooksFired is an instrument error; an unwired
// arm never is, regardless of edits, and a wired session that never
// edited anything has no governed-or-not edit for the hook to have
// answered, so it is not measurable either way.
func TestPostToolUseInstrumentError(t *testing.T) {
	cases := []struct {
		name        string
		wired       bool
		filesEdited []string
		fired       map[string]int
		wantMsg     string
	}{
		{"unwired, edited a file, no PostToolUse", false, []string{"CLAUDE.md"}, nil, ""},
		{"wired, no edits at all", true, nil, nil, ""},
		{"wired, edited a file, PostToolUse fired", true, []string{"CLAUDE.md"}, map[string]int{"PostToolUse": 1}, ""},
		{"wired, edited a file, nil HooksFired", true, []string{"CLAUDE.md"}, nil, "instrument: no PostToolUse hook fired despite an edit"},
		{"wired, edited a file, HooksFired present but no PostToolUse", true, []string{"CLAUDE.md"}, map[string]int{"SessionStart": 1}, "instrument: no PostToolUse hook fired despite an edit"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := postToolUseInstrumentError(c.wired, SessionResult{FilesEdited: c.filesEdited, HooksFired: c.fired})
			if got != c.wantMsg {
				t.Errorf("postToolUseInstrumentError = %q, want %q", got, c.wantMsg)
			}
		})
	}
}

// TestRunSessions_WiredArmEditNoPostToolUseHook_LosesTheCell is the
// PostToolUse instrument-truth gate's own end-to-end proof (clean room
// v3): a wired session whose transcript shows a working SessionStart
// primer (so hooksInstrumentError/primerInstrumentError both pass) but
// edits a file with no PostToolUse hook_response anywhere in the stream —
// exactly the pre-`--include-hook-events` defect ("dry-r4-change") — must
// not pass as a reading.
func TestRunSessions_WiredArmEditNoPostToolUseHook_LosesTheCell(t *testing.T) {
	wtDir := t.TempDir()
	task := Task{ID: "T1", Prompt: "do the thing", Sessions: Sessions{Mode: ModeSingle, Count: 1, MaxTurns: 10}}
	wt := Worktree{Path: wtDir}
	h := &FakeHarness{
		StreamsDir: streamsDir,
		TaskID:     "T1",
		ArmName:    "full",
		Streams:    map[string]string{"T1/full": "sessionstart-edit-no-posttool.jsonl"},
	}
	opts := RunOptions{Trial: 1, Model: "sonnet", RawDir: filepath.Join(t.TempDir(), "raw")}

	results, err := RunSessions(context.Background(), h, task, "full", Arm{Wired: true}, wt, opts)
	if err != nil {
		t.Fatalf("RunSessions: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected the single (failing) session, got %d", len(results))
	}
	wantErr := "instrument: no PostToolUse hook fired despite an edit"
	if results[0].Error != wantErr {
		t.Errorf("Error = %q, want %q", results[0].Error, wantErr)
	}
	if results[0].InstrumentError != wantErr {
		t.Errorf("InstrumentError = %q, want %q", results[0].InstrumentError, wantErr)
	}
	if reason := CellLost(results); reason == "" {
		t.Error("expected CellLost to report this cell as lost")
	}
}

// TestAllocateRawAttemptDir_TwoAttemptsGetDistinctDirs is O53 defect 2's
// low-level proof: a second allocation against the SAME cell directory
// must never return the directory the first allocation already claimed,
// and content written into the first attempt must survive the second
// allocation untouched.
func TestAllocateRawAttemptDir_TwoAttemptsGetDistinctDirs(t *testing.T) {
	cellDir := filepath.Join(t.TempDir(), "T1-full-t1")

	first, err := allocateRawAttemptDir(cellDir)
	if err != nil {
		t.Fatalf("first allocateRawAttemptDir: %v", err)
	}
	marker := filepath.Join(first, "session-0-single.stream.jsonl")
	if err := os.WriteFile(marker, []byte("attempt one's transcript"), 0o644); err != nil {
		t.Fatalf("writing marker: %v", err)
	}

	second, err := allocateRawAttemptDir(cellDir)
	if err != nil {
		t.Fatalf("second allocateRawAttemptDir: %v", err)
	}
	if second == first {
		t.Fatalf("second attempt reused the first attempt's directory: %s", second)
	}

	entries, err := os.ReadDir(cellDir)
	if err != nil {
		t.Fatalf("reading cell dir: %v", err)
	}
	if len(entries) != 2 {
		t.Fatalf("expected 2 attempt dirs under %s, got %d: %v", cellDir, len(entries), entries)
	}

	raw, err := os.ReadFile(marker)
	if err != nil || string(raw) != "attempt one's transcript" {
		t.Errorf("first attempt's transcript did not survive the second allocation: %q, err=%v", raw, err)
	}
}

// TestRunSessions_RepeatedCellKeyGetsDistinctAttemptDirs is O53 defect 2's
// end-to-end proof: two RunSessions calls for the EXACT SAME cell key
// (task, arm, trial, run-id) — a LOST retry or a resume re-running the
// same cell — must land their sessions in two different attempt-<k>
// directories on disk rather than one call's raw transcript silently
// overwriting the other's (the defect: 32 of 140 cells' on-disk transcript
// was a different session than the one the cell record scored).
func TestRunSessions_RepeatedCellKeyGetsDistinctAttemptDirs(t *testing.T) {
	wtDir := t.TempDir()
	task := Task{ID: "T1", Prompt: "do the thing", Sessions: Sessions{Mode: ModeSingle, Count: 1, MaxTurns: 10}}
	wt := Worktree{Path: wtDir}
	rawDir := filepath.Join(t.TempDir(), "raw")
	opts := RunOptions{Trial: 1, Model: "sonnet", RawDir: rawDir, RunID: "run-1"}

	h1 := &FakeHarness{StreamsDir: streamsDir, TaskID: "T1", ArmName: "full", Streams: map[string]string{"T1/full": "single-session.jsonl"}}
	if _, err := RunSessions(context.Background(), h1, task, "full", Arm{}, wt, opts); err != nil {
		t.Fatalf("first RunSessions: %v", err)
	}

	h2 := &FakeHarness{StreamsDir: streamsDir, TaskID: "T1", ArmName: "full", Streams: map[string]string{"T1/full": "single-session.jsonl"}}
	if _, err := RunSessions(context.Background(), h2, task, "full", Arm{}, wt, opts); err != nil {
		t.Fatalf("second (retried) RunSessions: %v", err)
	}

	cellDir := filepath.Join(rawDir, "run-1", "T1-full-t1")
	entries, err := os.ReadDir(cellDir)
	if err != nil {
		t.Fatalf("reading cell dir %s: %v", cellDir, err)
	}
	var attemptNames []string
	for _, e := range entries {
		attemptNames = append(attemptNames, e.Name())
	}
	if len(attemptNames) != 2 {
		t.Fatalf("expected 2 attempt dirs for the repeated cell key, got %v", attemptNames)
	}
	if attemptNames[0] == attemptNames[1] {
		t.Errorf("the two RunSessions calls shared one attempt dir: %v", attemptNames)
	}
}
