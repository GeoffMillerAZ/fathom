package scenariobench

import (
	"path/filepath"
	"strings"
	"testing"
)

const streamsDir = "../../testdata/scenariobench/streams"

func TestParseSessionStream_SingleSession(t *testing.T) {
	r, err := ParseSessionStream(filepath.Join(streamsDir, "single-session.jsonl"))
	if err != nil {
		t.Fatalf("ParseSessionStream: %v", err)
	}
	if r.Turns != 4 {
		t.Errorf("Turns = %d, want 4", r.Turns)
	}
	if r.ToolSearchTurns != 1 {
		t.Errorf("ToolSearchTurns = %d, want 1", r.ToolSearchTurns)
	}
	if r.FirstEditTurn != 3 {
		t.Errorf("FirstEditTurn = %d, want 3", r.FirstEditTurn)
	}
	if r.TokensToFirstEdit != 27 {
		t.Errorf("TokensToFirstEdit = %d, want 27", r.TokensToFirstEdit)
	}
	if len(r.FilesRead) != 1 || r.FilesRead[0] != "/repo/foo.go" {
		t.Errorf("FilesRead = %v, want [/repo/foo.go]", r.FilesRead)
	}
	if len(r.FilesEdited) != 1 || r.FilesEdited[0] != "/repo/foo.go" {
		t.Errorf("FilesEdited = %v, want [/repo/foo.go]", r.FilesEdited)
	}
	wantBytesRead := int64(len("package foo\n\nfunc Foo() {}\n"))
	if r.BytesRead != wantBytesRead {
		t.Errorf("BytesRead = %d, want %d", r.BytesRead, wantBytesRead)
	}
	if r.ToolCalls["ToolSearch"] != 1 || r.ToolCalls["Read"] != 1 || r.ToolCalls["Edit"] != 1 {
		t.Errorf("ToolCalls = %v, want ToolSearch=1 Read=1 Edit=1", r.ToolCalls)
	}
	if len(r.BoxChecks) != 0 {
		t.Errorf("BoxChecks = %v, want none (not a markdown file)", r.BoxChecks)
	}
	if r.TotalTokens != 39 {
		t.Errorf("TotalTokens = %d, want 39 (26 in + 13 out)", r.TotalTokens)
	}
	if r.CostUSD != 0.0123 {
		t.Errorf("CostUSD = %v, want 0.0123", r.CostUSD)
	}
	if r.Error != "" {
		t.Errorf("Error = %q, want empty", r.Error)
	}
}

func TestParseSessionStream_WorkerBoxCheck(t *testing.T) {
	r, err := ParseSessionStream(filepath.Join(streamsDir, "worker.jsonl"))
	if err != nil {
		t.Fatalf("ParseSessionStream: %v", err)
	}
	if len(r.BoxChecks) != 1 {
		t.Fatalf("BoxChecks = %v, want exactly 1", r.BoxChecks)
	}
	bc := r.BoxChecks[0]
	if bc.RowID != "row-1" {
		t.Errorf("BoxChecks[0].RowID = %q, want row-1", bc.RowID)
	}
	if bc.File != "/repo/NOTES.md" {
		t.Errorf("BoxChecks[0].File = %q, want /repo/NOTES.md", bc.File)
	}
	if bc.Turn != 2 {
		t.Errorf("BoxChecks[0].Turn = %d, want 2", bc.Turn)
	}
	if bc.Text != "- [x] {id: row-1} do thing one" {
		t.Errorf("BoxChecks[0].Text = %q, want the raw checklist line", bc.Text)
	}
}

func TestParseSessionStream_PlannerWriteNoBoxCheck(t *testing.T) {
	// The planner's Write creates rows that are UNCHECKED ("- [ ]"); a
	// freshly-written checklist with no checked rows must not be
	// mis-detected as a box check.
	r, err := ParseSessionStream(filepath.Join(streamsDir, "planner.jsonl"))
	if err != nil {
		t.Fatalf("ParseSessionStream: %v", err)
	}
	if len(r.BoxChecks) != 0 {
		t.Errorf("BoxChecks = %v, want none (rows are unchecked)", r.BoxChecks)
	}
	if len(r.FilesEdited) != 1 || r.FilesEdited[0] != "/repo/NOTES.md" {
		t.Errorf("FilesEdited = %v, want [/repo/NOTES.md]", r.FilesEdited)
	}
}

// TestParseSessionStream_MCPInitEvent is the defect fix's own proof at the
// parser layer: a system/init event's mcp_servers and tools must land on
// SessionResult.MCPServers/MCPTools, and the mcp-namespaced tool_use calls
// it precedes still count into ToolCalls exactly as any other tool would.
func TestParseSessionStream_MCPInitEvent(t *testing.T) {
	r, err := ParseSessionStream(filepath.Join(streamsDir, "mcp-init.jsonl"))
	if err != nil {
		t.Fatalf("ParseSessionStream: %v", err)
	}
	wantServers := map[string]string{"fathom": "connected", "graphify": "failed"}
	if len(r.MCPServers) != len(wantServers) {
		t.Fatalf("MCPServers = %v, want %v", r.MCPServers, wantServers)
	}
	for name, status := range wantServers {
		if r.MCPServers[name] != status {
			t.Errorf("MCPServers[%q] = %q, want %q", name, r.MCPServers[name], status)
		}
	}
	if r.MCPTools != 2 {
		t.Errorf("MCPTools = %d, want 2 (mcp__fathom__find_symbol, mcp__fathom__references)", r.MCPTools)
	}
	if r.ToolCalls["mcp__fathom__find_symbol"] != 1 || r.ToolCalls["mcp__fathom__references"] != 1 {
		t.Errorf("ToolCalls = %v, want mcp__fathom__find_symbol=1 mcp__fathom__references=1", r.ToolCalls)
	}
}

// TestParseSessionStream_NoInitEventLeavesMCPServersNil covers the
// no-MCP/control case: a transcript with no system/init event at all must
// report a nil MCPServers, never an empty-but-non-nil map that could be
// mistaken for "every declared server absent".
func TestParseSessionStream_NoInitEventLeavesMCPServersNil(t *testing.T) {
	r, err := ParseSessionStream(filepath.Join(streamsDir, "single-session.jsonl"))
	if err != nil {
		t.Fatalf("ParseSessionStream: %v", err)
	}
	if r.MCPServers != nil {
		t.Errorf("MCPServers = %v, want nil (no init event in this fixture)", r.MCPServers)
	}
	if r.MCPTools != 0 {
		t.Errorf("MCPTools = %d, want 0", r.MCPTools)
	}
}

// TestParseSessionStream_HookResponseEvents covers RC-hooks: a
// "system"/"hook_response" event's own hook_event must land on
// SessionResult.HooksFired, counted per event name — the real shape found
// in docs/bench/scenarios.md's clean-room smoke test and this repo's own
// committed raw transcripts (workspace/artifacts/scenariobench/raw).
func TestParseSessionStream_HookResponseEvents(t *testing.T) {
	r, err := ParseSessionStream(filepath.Join(streamsDir, "hooks-fired.jsonl"))
	if err != nil {
		t.Fatalf("ParseSessionStream: %v", err)
	}
	want := map[string]int{"SessionStart": 1, "PostToolUse": 1}
	if len(r.HooksFired) != len(want) {
		t.Fatalf("HooksFired = %v, want %v", r.HooksFired, want)
	}
	for name, n := range want {
		if r.HooksFired[name] != n {
			t.Errorf("HooksFired[%q] = %d, want %d", name, r.HooksFired[name], n)
		}
	}
}

// TestParseSessionStream_ObligationsInjectedCount is O45 item 8: a
// PostToolUse hook_response whose own additionalContext carries the
// obligations marker increments the synthetic "obligations_injected" key
// alongside the real "PostToolUse" count; a SessionStart hook_response
// (present in the same fixture, with no such marker) must not.
func TestParseSessionStream_ObligationsInjectedCount(t *testing.T) {
	r, err := ParseSessionStream(filepath.Join(streamsDir, "obligations-injected.jsonl"))
	if err != nil {
		t.Fatalf("ParseSessionStream: %v", err)
	}
	if r.HooksFired["obligations_injected"] != 1 {
		t.Errorf("HooksFired[obligations_injected] = %d, want 1", r.HooksFired["obligations_injected"])
	}
	if r.HooksFired["PostToolUse"] != 1 {
		t.Errorf("HooksFired[PostToolUse] = %d, want 1", r.HooksFired["PostToolUse"])
	}
}

// TestParseSessionStream_SessionStartContextCaptured proves
// SessionResult.SessionStartContext is the FIRST SessionStart
// hook_response's own decoded additionalContext.
func TestParseSessionStream_SessionStartContextCaptured(t *testing.T) {
	r, err := ParseSessionStream(filepath.Join(streamsDir, "hooks-fired.jsonl"))
	if err != nil {
		t.Fatalf("ParseSessionStream: %v", err)
	}
	if !strings.Contains(r.SessionStartContext, "is the indexed service") {
		t.Errorf("SessionStartContext = %q, want it to contain %q", r.SessionStartContext, "is the indexed service")
	}
}

// TestParseSessionStream_NoHookResponseLeavesHooksFiredNil mirrors
// TestParseSessionStream_NoInitEventLeavesMCPServersNil: a transcript with
// no hook_response event at all must report nil, never an empty-but-non-nil
// map that could be mistaken for "every hook counted, zero times".
func TestParseSessionStream_NoHookResponseLeavesHooksFiredNil(t *testing.T) {
	r, err := ParseSessionStream(filepath.Join(streamsDir, "single-session.jsonl"))
	if err != nil {
		t.Fatalf("ParseSessionStream: %v", err)
	}
	if r.HooksFired != nil {
		t.Errorf("HooksFired = %v, want nil (no hook_response event in this fixture)", r.HooksFired)
	}
}

// TestParseSessionStream_HooksFiredByName is wave n13's own proof: the
// same "hooks-fired.jsonl" fixture's two hook_response events (each
// already carrying its own real hook_name) must ALSO land on
// SessionResult.HooksFiredByName, keyed by that full "Event:matcher"
// string rather than the bare event name HooksFired uses.
func TestParseSessionStream_HooksFiredByName(t *testing.T) {
	r, err := ParseSessionStream(filepath.Join(streamsDir, "hooks-fired.jsonl"))
	if err != nil {
		t.Fatalf("ParseSessionStream: %v", err)
	}
	want := map[string]int{"SessionStart:startup": 1, "PostToolUse:Edit|Write|MultiEdit": 1}
	if len(r.HooksFiredByName) != len(want) {
		t.Fatalf("HooksFiredByName = %v, want %v", r.HooksFiredByName, want)
	}
	for name, n := range want {
		if r.HooksFiredByName[name] != n {
			t.Errorf("HooksFiredByName[%q] = %d, want %d", name, r.HooksFiredByName[name], n)
		}
	}
}

// TestParseSessionStream_HooksFiredByNameShowsMCPGap is H1-T06's own
// defect, pinned as a regression: a session that calls two
// mcp__fathom__* tools and one Grep call, where only Grep's PreToolUse
// fires a hook_response, must show "PreToolUse:Grep": 1 in
// HooksFiredByName with NO "PreToolUse:mcp__fathom__…" key at all — the
// gap vs-pilot-5's second reading found, now directly visible rather
// than hidden inside a same-looking "PreToolUse": 1 total.
func TestParseSessionStream_HooksFiredByNameShowsMCPGap(t *testing.T) {
	r, err := ParseSessionStream(filepath.Join(streamsDir, "hooks-fired-mcp-gap.jsonl"))
	if err != nil {
		t.Fatalf("ParseSessionStream: %v", err)
	}
	if r.HooksFiredByName["PreToolUse:Grep"] != 1 {
		t.Errorf("HooksFiredByName[PreToolUse:Grep] = %d, want 1", r.HooksFiredByName["PreToolUse:Grep"])
	}
	for name := range r.HooksFiredByName {
		if strings.Contains(name, "mcp__fathom__") {
			t.Errorf("HooksFiredByName unexpectedly carries a PreToolUse entry for an mcp__fathom__ call: %q (fixture has none — this pins the gap, not a fix to the fixture)", name)
		}
	}
	// The bare hook_event count (HooksFired) still shows only 1 PreToolUse
	// fire in total — exactly the number that hid the gap before this field
	// existed.
	if r.HooksFired["PreToolUse"] != 1 {
		t.Errorf("HooksFired[PreToolUse] = %d, want 1 (the pre-existing, coarser count)", r.HooksFired["PreToolUse"])
	}
}

func TestParseSessionStream_MissingFile(t *testing.T) {
	if _, err := ParseSessionStream(filepath.Join(streamsDir, "does-not-exist.jsonl")); err == nil {
		t.Fatal("expected an error for a missing transcript")
	}
}

func TestParseSessionStream_NoResultEvent(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "no-result.jsonl")
	writeTestFile(t, path, `{"type":"assistant","message":{"content":[{"type":"text","text":"hi"}],"usage":{"input_tokens":1,"output_tokens":1}}}`+"\n")
	r, err := ParseSessionStream(path)
	if err != nil {
		t.Fatalf("ParseSessionStream: %v", err)
	}
	if r.Error == "" {
		t.Error("expected Error to be set when no terminal result event is present")
	}
}

// TestParseSessionStream_FinalAnswer proves the result event's own
// "result" text lands on SessionResult.FinalAnswer (SB-32's answer-only
// oracle) — the fixture's fake stream fixture whose result text carries a
// token.
func TestParseSessionStream_FinalAnswer(t *testing.T) {
	r, err := ParseSessionStream(filepath.Join(streamsDir, "answer-token.jsonl"))
	if err != nil {
		t.Fatalf("ParseSessionStream: %v", err)
	}
	const want = "The answer is fusion.New and also golden-test appear here."
	if r.FinalAnswer != want {
		t.Errorf("FinalAnswer = %q, want %q", r.FinalAnswer, want)
	}
	if r.Error != "" {
		t.Errorf("Error = %q, want empty", r.Error)
	}
	if r.AnswerFrom != "result" {
		t.Errorf("AnswerFrom = %q, want %q", r.AnswerFrom, "result")
	}
}

// TestParseSessionStream_FinalAnswerFallsBackToLastTextOnBudgetHit is O45
// item 3: a session that stated its answer and THEN ran out of turns
// (error_max_turns) must not score as blank — the terminal result event's
// own "result" text is empty on error_max_turns, so FinalAnswer falls back
// to the last assistant text block, and AnswerFrom records the fallback.
func TestParseSessionStream_FinalAnswerFallsBackToLastTextOnBudgetHit(t *testing.T) {
	r, err := ParseSessionStream(filepath.Join(streamsDir, "answer-budget-fallback.jsonl"))
	if err != nil {
		t.Fatalf("ParseSessionStream: %v", err)
	}
	const want = "The answer is fusion.New and also golden-test appear here."
	if r.FinalAnswer != want {
		t.Errorf("FinalAnswer = %q, want %q", r.FinalAnswer, want)
	}
	if r.AnswerFrom != "last_text" {
		t.Errorf("AnswerFrom = %q, want %q", r.AnswerFrom, "last_text")
	}
	if !r.BudgetHit {
		t.Error("expected BudgetHit true for an error_max_turns result")
	}
	if r.Error != "" {
		t.Errorf("Error = %q, want empty (a turn-budget hand-off is not an error)", r.Error)
	}
}

// TestParseSessionStream_MalformedPathSanitizedToFirstLine covers a
// malformed tool_use whose path-shaped input field carries an entire file
// body instead of a path (public pilot 1, 2026-09-13): FilesEdited and
// FilesRead must record only the first line, never the whole body.
func TestParseSessionStream_MalformedPathSanitizedToFirstLine(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "malformed.jsonl")
	writeTestFile(t, path,
		`{"type":"assistant","message":{"content":[`+
			`{"type":"tool_use","id":"tu_1","name":"Write","input":{"file_path":"/repo/BROKEN.md\npackage foo\n\nfunc Foo() {}\n"}},`+
			`{"type":"tool_use","id":"tu_2","name":"Read","input":{"file_path":"/repo/OTHER.md\nsome body text\nmore body\n"}},`+
			`{"type":"tool_use","id":"tu_3","name":"mcp__fathom__read_source","input":{"file":"internal/x.go\nsome body text\n"}}`+
			`],"usage":{"input_tokens":1,"output_tokens":1}}}`+"\n"+
			`{"type":"result","is_error":false,"num_turns":1,"duration_ms":1,"total_cost_usd":0,"usage":{"input_tokens":1,"output_tokens":1}}`+"\n")

	r, err := ParseSessionStream(path)
	if err != nil {
		t.Fatalf("ParseSessionStream: %v", err)
	}
	if len(r.FilesEdited) != 1 || r.FilesEdited[0] != "/repo/BROKEN.md" {
		t.Errorf("FilesEdited = %v, want [/repo/BROKEN.md] (sanitized to the first line)", r.FilesEdited)
	}
	wantRead := []string{"/repo/OTHER.md", "internal/x.go"}
	if len(r.FilesRead) != len(wantRead) {
		t.Fatalf("FilesRead = %v, want %v", r.FilesRead, wantRead)
	}
	for _, w := range wantRead {
		found := false
		for _, got := range r.FilesRead {
			if got == w {
				found = true
			}
		}
		if !found {
			t.Errorf("FilesRead = %v, missing sanitized path %q", r.FilesRead, w)
		}
	}
}
