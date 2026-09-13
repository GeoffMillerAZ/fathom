package scenariobench

import (
	"path/filepath"
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
