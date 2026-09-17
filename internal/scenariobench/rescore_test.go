package scenariobench

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCellRawBase_MatchesRawDirRunIDTaskArmTrial(t *testing.T) {
	got := CellRawBase("/root/raw", "vs-pilot-6", "H1-T01", "full", 2)
	want := filepath.Join("/root/raw", "vs-pilot-6", "H1-T01-full-t2")
	if got != want {
		t.Errorf("CellRawBase = %q, want %q", got, want)
	}
}

func TestCellRawBase_EmptyRunID(t *testing.T) {
	got := CellRawBase("/root/raw", "", "T1", "control", 1)
	want := filepath.Join("/root/raw", "T1-control-t1")
	if got != want {
		t.Errorf("CellRawBase = %q, want %q", got, want)
	}
}

// writeRawStream writes lines (already-JSON-encoded event lines, one per
// entry) as one raw session stream file under dir.
func writeRawStream(t *testing.T, path string, lines []string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	content := strings.Join(lines, "\n") + "\n"
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

const completeResultLine = `{"type":"result","is_error":false,"num_turns":1,"duration_ms":1,"total_cost_usd":0,"result":"the answer is fusion.New","usage":{"input_tokens":1,"output_tokens":1}}`

func TestIsCompleteSessionStream(t *testing.T) {
	dir := t.TempDir()
	complete := filepath.Join(dir, "complete.jsonl")
	writeRawStream(t, complete, []string{
		`{"type":"assistant","message":{"content":[{"type":"text","text":"working"}],"usage":{"input_tokens":1,"output_tokens":1}}}`,
		completeResultLine,
	})
	incomplete := filepath.Join(dir, "incomplete.jsonl")
	writeRawStream(t, incomplete, []string{
		`{"type":"assistant","message":{"content":[{"type":"text","text":"working"}],"usage":{"input_tokens":1,"output_tokens":1}}}`,
	})

	if ok, err := IsCompleteSessionStream(complete); err != nil || !ok {
		t.Errorf("IsCompleteSessionStream(complete) = %v, %v; want true, nil", ok, err)
	}
	if ok, err := IsCompleteSessionStream(incomplete); err != nil || ok {
		t.Errorf("IsCompleteSessionStream(incomplete) = %v, %v; want false, nil", ok, err)
	}
}

// TestLatestCompleteAttempt_PicksHighestComplete covers a retried cell
// key: attempt-1 is complete, attempt-2 is a killed driver's own partial
// leftover — the LATEST attempt must still win when it is itself
// complete, never an older one merely because it is safer.
func TestLatestCompleteAttempt_PicksHighestComplete(t *testing.T) {
	dir := t.TempDir()
	writeRawStream(t, filepath.Join(dir, "attempt-1", "session-0-single.stream.jsonl"), []string{completeResultLine})
	writeRawStream(t, filepath.Join(dir, "attempt-2", "session-0-single.stream.jsonl"), []string{completeResultLine})

	got, ok, err := LatestCompleteAttempt(dir)
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Fatal("expected an attempt to be found")
	}
	if want := filepath.Join(dir, "attempt-2"); got != want {
		t.Errorf("LatestCompleteAttempt = %q, want %q (the highest-numbered complete attempt)", got, want)
	}
}

// TestLatestCompleteAttempt_SkipsIncompleteHighestAttempt: attempt-2 was
// cut off mid-session (no terminal result event) — the scan must fall
// back to attempt-1 rather than rescoring half a session as if it were
// whole.
func TestLatestCompleteAttempt_SkipsIncompleteHighestAttempt(t *testing.T) {
	dir := t.TempDir()
	writeRawStream(t, filepath.Join(dir, "attempt-1", "session-0-single.stream.jsonl"), []string{completeResultLine})
	writeRawStream(t, filepath.Join(dir, "attempt-2", "session-0-single.stream.jsonl"), []string{
		`{"type":"assistant","message":{"content":[{"type":"text","text":"still working"}],"usage":{"input_tokens":1,"output_tokens":1}}}`,
	})

	got, ok, err := LatestCompleteAttempt(dir)
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Fatal("expected attempt-1 to be found even though attempt-2 is incomplete")
	}
	if want := filepath.Join(dir, "attempt-1"); got != want {
		t.Errorf("LatestCompleteAttempt = %q, want %q", got, want)
	}
}

// TestLatestCompleteAttempt_NoneQualify covers the all-incomplete case
// (a driver killed mid-attempt with no earlier successful try): false,
// never a guess.
func TestLatestCompleteAttempt_NoneQualify(t *testing.T) {
	dir := t.TempDir()
	writeRawStream(t, filepath.Join(dir, "attempt-1", "session-0-single.stream.jsonl"), []string{
		`{"type":"assistant","message":{"content":[{"type":"text","text":"still working"}],"usage":{"input_tokens":1,"output_tokens":1}}}`,
	})
	_, ok, err := LatestCompleteAttempt(dir)
	if err != nil {
		t.Fatal(err)
	}
	if ok {
		t.Error("expected no complete attempt to be found")
	}
}

// TestLatestCompleteAttempt_MissingDirIsFalseNotError covers a cell that
// never ran at all under this run-id: no raw dir exists yet.
func TestLatestCompleteAttempt_MissingDirIsFalseNotError(t *testing.T) {
	_, ok, err := LatestCompleteAttempt(filepath.Join(t.TempDir(), "never-existed"))
	if err != nil {
		t.Fatal(err)
	}
	if ok {
		t.Error("expected false for a raw base that does not exist")
	}
}

func TestNeedsLiveWorktree(t *testing.T) {
	cases := []struct {
		name string
		task Task
		want bool
	}{
		{"answer-only (lookups-vs.json shape)", Task{Oracle: Oracle{AnswerAllOf: []string{"fusion.New"}}}, false},
		{"plain task with no oracle checks", Task{}, false},
		{"test patch", Task{Gold: Gold{TestPatch: "diff --git a/x b/x"}}, true},
		{"fail_to_pass names", Task{Oracle: Oracle{FailToPass: []string{"TestFoo"}}}, true},
		{"pass_to_pass names", Task{Oracle: Oracle{PassToPass: []string{"TestBar"}}}, true},
		{"detectors", Task{Oracle: Oracle{Detectors: []Detector{{Name: "d1"}}}}, true},
		{"refusal-aware", Task{Oracle: Oracle{Refusal: &Refusal{ForbiddenPaths: []string{"x"}}}}, true},
		{"localization-only", Task{Oracle: Oracle{Localization: true}}, false},
	}
	for _, c := range cases {
		if got := NeedsLiveWorktree(c.task); got != c.want {
			t.Errorf("%s: NeedsLiveWorktree = %v, want %v", c.name, got, c.want)
		}
	}
}

func TestHistoricalWorktreePathAndCellDir_MatchPrepareSubjectFormula(t *testing.T) {
	task := Task{ID: "H1-T01", Subject: Subject{RepoURL: "https://example.invalid/repo.git", BaseCommit: "deadbeef"}}
	arm := Arm{Wired: true}
	root := "/root"

	wtPath := HistoricalWorktreePath(root, task, arm)
	wantWT := filepath.Join(root, "workspace", "scenariobench", "subjects", shortHash(task.Subject.RepoURL)+"-worktrees", task.Subject.BaseCommit+"-"+armContentHash(arm))
	if wtPath != wantWT {
		t.Errorf("HistoricalWorktreePath = %q, want %q", wtPath, wantWT)
	}

	cellDir := HistoricalCellDir(root, task, arm, 2)
	wantCellDirBase := filepath.Join(root, "workspace", "artifacts", "scenariobench", "worktrees", task.ID+"-"+filepath.Base(wtPath))
	wantCellDir := filepath.Join(wantCellDirBase, "trial-2")
	if cellDir != wantCellDir {
		t.Errorf("HistoricalCellDir = %q, want %q", cellDir, wantCellDir)
	}

	subjectsDir := HistoricalSubjectsDir(root)
	wantSubjectsDir := filepath.Join(root, "workspace", "scenariobench", "subjects")
	if subjectsDir != wantSubjectsDir {
		t.Errorf("HistoricalSubjectsDir = %q, want %q", subjectsDir, wantSubjectsDir)
	}
	// wantWT is subjectsDir/<hash>-worktrees/<commit-armhash> —
	// HistoricalWorktreePath's own cloneDir sits directly under
	// HistoricalSubjectsDir, exactly as PrepareSubject's live formula
	// does (subject.go).
	if !strings.HasPrefix(wtPath, subjectsDir+string(filepath.Separator)) {
		t.Errorf("HistoricalWorktreePath %q is not nested under HistoricalSubjectsDir %q", wtPath, subjectsDir)
	}
}

// TestSubjectsDirFromClone_MatchesHistoricalSubjectsDir proves the LIVE
// path's own formula (SubjectsDirFromClone, sessions.go's own safe-dir
// wiring — filepath.Dir(Worktree.SubjectClone)) agrees with the rescore
// path's no-I/O formula (HistoricalSubjectsDir) for the SAME (root, repo)
// pair: PrepareSubject's own cloneDir is subjectsDir/shortHash(repoURL),
// so Dir(cloneDir) must equal subjectsDir exactly, on both paths.
func TestSubjectsDirFromClone_MatchesHistoricalSubjectsDir(t *testing.T) {
	root := "/root"
	repoURL := "https://example.invalid/repo.git"
	cloneDir := filepath.Join(HistoricalSubjectsDir(root), shortHash(repoURL))
	got := SubjectsDirFromClone(cloneDir)
	want := HistoricalSubjectsDir(root)
	if got != want {
		t.Errorf("SubjectsDirFromClone(%q) = %q, want %q", cloneDir, got, want)
	}
	if SubjectsDirFromClone("") != "" {
		t.Error(`SubjectsDirFromClone("") = non-empty, want "" (never a guess with no real Worktree)`)
	}
}

// TestRescoreSessionsFromRaw_ReconstructsSessionAndSuppressesFixedFalsePositive
// is wave N18 defect 2's own core proof: a raw attempt dir holding one
// complete session stream file, whose own Bash `cd` into the cell's
// subject-worktree directory would have false-positived the PRE-FIX
// SandboxEscapeScan (it names "/benchroot" as a literal substring of
// "benchroot2", exactly like defect 1) — RescoreSessionsFromRaw must
// reconstruct the session AND correctly clear it, given the same safe
// directories a live run would have known.
func TestRescoreSessionsFromRaw_ReconstructsSessionAndSuppressesFixedFalsePositive(t *testing.T) {
	dir := t.TempDir()
	root := "/private/tmp/claude-501/-Volumes-m2ext-code-geoffmilleraz-sextant/e8652946-4037-4a56-8b48-3336a3490e6f/scratchpad/benchroot2"
	subjectWorktreeDir := root + "/workspace/scenariobench/subjects/abc123-worktrees/deadbeef-feedface"
	cellDir := root + "/workspace/artifacts/scenariobench/worktrees/H1-T01-deadbeef-feedface/trial-1"
	rawBase := CellRawBase(filepath.Join(dir, "raw"), "vs-pilot-6", "H1-T01", "full", 1)
	attemptDir := filepath.Join(rawBase, "attempt-1")

	writeRawStream(t, filepath.Join(attemptDir, "session-0-single.stream.jsonl"), []string{
		`{"type":"system","subtype":"init","mcp_servers":[{"name":"fathom","status":"connected"}],"tools":["mcp__fathom__find_symbol"]}`,
		`{"type":"system","subtype":"hook_response","hook_event":"SessionStart","hook_name":"SessionStart:startup","output":"this repo (fathom) is the indexed service"}`,
		`{"type":"assistant","message":{"content":[{"type":"tool_use","id":"tu0","name":"Bash","input":{"command":"cd ` + subjectWorktreeDir + ` && git status"}}],"usage":{"input_tokens":5,"output_tokens":5}}}`,
		`{"type":"user","message":{"content":[{"type":"tool_result","tool_use_id":"tu0","content":"On branch main\nnothing to commit, working tree clean\n"}]}}`,
		`{"type":"result","is_error":false,"num_turns":1,"duration_ms":1,"total_cost_usd":0.001,"result":"the answer is fusion.New","usage":{"input_tokens":5,"output_tokens":5}}`,
	})
	mcpPath := filepath.Join(attemptDir, "session-0-mcp.json")
	if err := os.WriteFile(mcpPath, []byte(`{"mcpServers":{"fathom":{"alwaysLoad":true}}}`), 0o644); err != nil {
		t.Fatal(err)
	}

	sessions, err := RescoreSessionsFromRaw(attemptDir, nil, true, subjectWorktreeDir, []string{subjectWorktreeDir, cellDir, rawBase})
	if err != nil {
		t.Fatalf("RescoreSessionsFromRaw: %v", err)
	}
	if len(sessions) != 1 {
		t.Fatalf("expected 1 reconstructed session, got %d", len(sessions))
	}
	s := sessions[0]
	if s.FinalAnswer != "the answer is fusion.New" {
		t.Errorf("FinalAnswer = %q, want the parsed result text", s.FinalAnswer)
	}
	if s.SandboxEscape {
		t.Error("expected no sandbox escape: the cd targets the cell's own subject-worktree directory, given as a safe dir")
	}
	if s.AnswerKeyLeak {
		t.Error("expected no answer-key leak: no excludePaths were given")
	}
	if s.Error != "" {
		t.Errorf("Error = %q, want no instrument error (fathom connected, matching the captured mcp.json)", s.Error)
	}
}

// TestRescoreSessionsFromRaw_GenuineEscapeStaysFlagged is the converse:
// a real sandbox escape (naming the operator's own push clone, outside
// every safe dir) must still be caught after reconstruction — rescoring
// must never be a looser bar than the live path.
func TestRescoreSessionsFromRaw_GenuineEscapeStaysFlagged(t *testing.T) {
	dir := t.TempDir()
	root := "/private/tmp/claude-501/-Volumes-m2ext-code-geoffmilleraz-sextant/e8652946-4037-4a56-8b48-3336a3490e6f/scratchpad"
	escaped := root + "/pushclone/testdata/scenariobench/guards-fathom.json"
	subjectWorktreeDir := root + "/benchroot2/workspace/scenariobench/subjects/abc123-worktrees/deadbeef-feedface"
	cellDir := root + "/benchroot2/workspace/artifacts/scenariobench/worktrees/H1-T05-deadbeef-feedface/trial-1"
	attemptDir := filepath.Join(dir, "raw", "vs-pilot-6", "H1-T05-full-t1", "attempt-1")

	writeRawStream(t, filepath.Join(attemptDir, "session-0-single.stream.jsonl"), []string{
		`{"type":"assistant","message":{"content":[{"type":"tool_use","id":"tu0","name":"Bash","input":{"command":"find /private/tmp/claude-501 -maxdepth 8 -iname \"*fathom*\""}}],"usage":{"input_tokens":5,"output_tokens":5}}}`,
		`{"type":"user","message":{"content":[{"type":"tool_result","tool_use_id":"tu0","content":"` + escaped + `\n"}]}}`,
		completeResultLine,
	})

	sessions, err := RescoreSessionsFromRaw(attemptDir, nil, false, subjectWorktreeDir, []string{subjectWorktreeDir, cellDir})
	if err != nil {
		t.Fatalf("RescoreSessionsFromRaw: %v", err)
	}
	if len(sessions) != 1 {
		t.Fatalf("expected 1 reconstructed session, got %d", len(sessions))
	}
	if !sessions[0].SandboxEscape {
		t.Error("expected the genuine escape to stay flagged after reconstruction")
	}
}

// TestRescoreSessionsFromRaw_SerenaActiveProjectMessageStaysScorable is
// wave N19 defect 1, built from the EXACT raw-dir shape of a real
// vs-pilot-6 cell the rescore-from-raw pass silently dropped
// (H1-T04/serena/t1, docs/bench/readings/vs-pilot-6-second-reading.md
// §6): one attempt-1/session-0-<role>.stream.jsonl plus its sibling
// session-0-mcp.json, exactly as ClaudeHarness itself writes them — the
// stream's structure copied verbatim, never its real content. Serena's
// own session-start tool_result narrates the worktree path in prose
// immediately followed by a period and a newline with no space
// ("...at <path>.\nActive language servers: go."), which used to glue
// that period onto an otherwise-exact safe-dir token and lose the whole
// cell to a false sandbox_escape despite a complete session with a real
// final answer (is_error: false, a genuine result event). This also
// covers wave N19 defect 2 in the same pass: the result event carries
// modelUsage but no top-level total_cost_usd, so the reconstructed
// session's own CostUSD must still come out nonzero.
func TestRescoreSessionsFromRaw_SerenaActiveProjectMessageStaysScorable(t *testing.T) {
	dir := t.TempDir()
	root := "/private/tmp/claude-501/-Volumes-m2ext-code-geoffmilleraz-sextant/e8652946-4037-4a56-8b48-3336a3490e6f/scratchpad/benchroot2"
	subjectWorktreeDir := root + "/workspace/scenariobench/subjects/cc9873a2-worktrees/1b8765d-d13524f3c445"
	cellDir := root + "/workspace/artifacts/scenariobench/worktrees/H1-T04-1b8765d-d13524f3c445/trial-1"
	subjectsDir := root + "/workspace/scenariobench/subjects"
	rawBase := CellRawBase(filepath.Join(dir, "raw"), "vs-pilot-6", "H1-T04", "serena", 1)
	attemptDir := filepath.Join(rawBase, "attempt-1")

	activeProjectMsg := "Created and activated a new project with name '1b8765d-d13524f3c445' at " +
		subjectWorktreeDir + ".\\nActive language servers: go.\\nFile encoding: utf-8.\\n"
	writeRawStream(t, filepath.Join(attemptDir, "session-0-single.stream.jsonl"), []string{
		`{"type":"system","subtype":"init","mcp_servers":[{"name":"serena","status":"connected"}],"tools":["mcp__serena__activate_project"]}`,
		`{"type":"assistant","message":{"content":[{"type":"tool_use","id":"tu0","name":"mcp__serena__activate_project","input":{}}],"usage":{"input_tokens":5,"output_tokens":5}}}`,
		`{"type":"user","message":{"content":[{"type":"tool_result","tool_use_id":"tu0","content":"` + activeProjectMsg + `"}]}}`,
		`{"type":"result","is_error":false,"num_turns":1,"duration_ms":1,` +
			`"modelUsage":{"claude-sonnet-5":{"costUSD":0.065024}},` +
			`"result":"cmdVerify exists at cmd/fathom/verify.go:37","usage":{"input_tokens":5,"output_tokens":5}}`,
	})
	mcpPath := filepath.Join(attemptDir, "session-0-mcp.json")
	if err := os.WriteFile(mcpPath, []byte(`{"mcpServers":{"serena":{"alwaysLoad":true}}}`), 0o644); err != nil {
		t.Fatal(err)
	}

	attemptGot, ok, err := LatestCompleteAttempt(rawBase)
	if err != nil || !ok {
		t.Fatalf("LatestCompleteAttempt: ok=%v err=%v", ok, err)
	}

	sessions, err := RescoreSessionsFromRaw(attemptGot, nil, false, subjectWorktreeDir,
		[]string{subjectWorktreeDir, cellDir, rawBase, subjectsDir})
	if err != nil {
		t.Fatalf("RescoreSessionsFromRaw: %v", err)
	}
	if len(sessions) != 1 {
		t.Fatalf("expected 1 reconstructed session, got %d", len(sessions))
	}
	s := sessions[0]
	if s.SandboxEscape {
		t.Error("expected no sandbox escape: Serena's own startup message names this cell's own subject-worktree, just with a trailing sentence period")
	}
	if reason := CellLost([]SessionResult{s}); reason != "" {
		t.Errorf("CellLost = %q, want \"\" (a complete, correctly-answered raw session must stay scorable)", reason)
	}
	if s.CostUSD != 0.065024 {
		t.Errorf("CostUSD = %v, want 0.065024 (summed from modelUsage since total_cost_usd is absent)", s.CostUSD)
	}
}

// TestRescoreSessionsFromRaw_MultipleSessionsInIndexOrder covers a
// planner-workers-shaped raw dir: session files are reconstructed in
// session-index order, not directory-listing order.
func TestRescoreSessionsFromRaw_MultipleSessionsInIndexOrder(t *testing.T) {
	dir := t.TempDir()
	attemptDir := filepath.Join(dir, "attempt-1")
	writeRawStream(t, filepath.Join(attemptDir, "session-1-worker.stream.jsonl"), []string{completeResultLine})
	writeRawStream(t, filepath.Join(attemptDir, "session-0-planner.stream.jsonl"), []string{completeResultLine})

	sessions, err := RescoreSessionsFromRaw(attemptDir, nil, false, "", nil)
	if err != nil {
		t.Fatalf("RescoreSessionsFromRaw: %v", err)
	}
	if len(sessions) != 2 {
		t.Fatalf("expected 2 sessions, got %d", len(sessions))
	}
	if sessions[0].Index != 0 || sessions[0].Role != "planner" {
		t.Errorf("sessions[0] = index %d role %q, want index 0 role \"planner\"", sessions[0].Index, sessions[0].Role)
	}
	if sessions[1].Index != 1 || sessions[1].Role != "worker" {
		t.Errorf("sessions[1] = index %d role %q, want index 1 role \"worker\"", sessions[1].Index, sessions[1].Role)
	}
}

// TestRescoreSessionsFromRaw_NoStreamFilesIsAnError covers a raw attempt
// dir that exists but holds nothing to rescore (an empty or foreign
// directory) — an explicit error, never zero sessions silently treated
// as a valid (and therefore CellLost "no sessions ran") cell.
func TestRescoreSessionsFromRaw_NoStreamFilesIsAnError(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := RescoreSessionsFromRaw(dir, nil, false, "", nil); err == nil {
		t.Error("expected an error for an attempt dir with no session stream files")
	}
}
