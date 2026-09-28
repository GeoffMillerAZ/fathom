package harness

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"
)

// decodeJSON parses encoded codec output into a map; nil bytes are nil.
func decodeJSON(t *testing.T, b []byte) map[string]any {
	t.Helper()
	if b == nil {
		return nil
	}
	var m map[string]any
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatalf("output is not a JSON object: %s", b)
	}
	return m
}

// TestCursorCodecRoundTrip (GL-7): Cursor's flat snake_case stdin decodes
// into the neutral envelope and the neutral output encodes back into
// Cursor's own top-level snake_case fields.
func TestCursorCodecRoundTrip(t *testing.T) {
	if wired, ok := CodecFor("cursor"); !ok || wired.ID() != "cursor" {
		t.Fatal("cursor must have its codec")
	}
	c := cursorCodec()

	// afterFileEdit: conversation_id is the session, workspace_roots[0]
	// the cwd, the top-level file_path and edits[] the change.
	in, err := c.DecodeInput(EventPostTool, []byte(`{"conversation_id":"conv-1","generation_id":"g","hook_event_name":"afterFileEdit","workspace_roots":["/w/proj"],"transcript_path":"/t.jsonl","file_path":"/w/proj/a.go","edits":[{"old_string":"x","new_string":"y"},{"old_string":"p","new_string":"q"}]}`))
	if err != nil {
		t.Fatal(err)
	}
	if in.SessionID != "conv-1" || in.Cwd != "/w/proj" || in.FilePath != "/w/proj/a.go" || in.TranscriptPath != "/t.jsonl" || in.NewContent != "y\nq" || in.OldContent != "x\np" {
		t.Errorf("afterFileEdit decoded = %+v", in)
	}
	// preToolUse: tool_input.file_path, tool classified from the descriptor.
	in, err = c.DecodeInput(EventPreTool, []byte(`{"conversation_id":"c","cwd":"/w","tool_name":"Write","tool_input":{"file_path":"b.go"}}`))
	if err != nil || in.FilePath != "b.go" || in.Tool != ToolFileEdit || in.Cwd != "/w" {
		t.Errorf("preToolUse decoded = %+v, %v", in, err)
	}
	// beforeShellExecution: top-level command.
	in, _ = c.DecodeInput(EventPreTool, []byte(`{"conversation_id":"c","command":"go test ./..."}`))
	if in.Command != "go test ./..." || in.Tool != ToolShell {
		t.Errorf("beforeShellExecution decoded = %+v", in)
	}
	// sessionStart names session_id itself; stop's loop_count marks re-entry.
	in, _ = c.DecodeInput(EventSessionStart, []byte(`{"session_id":"s","conversation_id":"c"}`))
	if in.SessionID != "s" {
		t.Errorf("sessionStart session = %q, want s", in.SessionID)
	}
	in, _ = c.DecodeInput(EventStop, []byte(`{"conversation_id":"c","status":"completed","loop_count":1}`))
	if !in.ReEntrant {
		t.Error("stop with loop_count>0 must be re-entrant")
	}

	// Context: additional_context on sessionStart and afterFileEdit only.
	for _, ev := range []Event{EventSessionStart, EventPostTool} {
		out, err := c.EncodeOutput(ev, HookOutput{Context: "rules"})
		if err != nil || !reflect.DeepEqual(decodeJSON(t, out), map[string]any{"additional_context": "rules"}) {
			t.Errorf("%s context = %s, %v", ev, out, err)
		}
	}
	for _, ev := range []Event{EventPreTool, EventStop, EventSessionEnd} {
		out, err := c.EncodeOutput(ev, HookOutput{Context: "rules"})
		if err != nil || out != nil {
			t.Errorf("%s has no context channel: got %s, %v; want silence", ev, out, err)
		}
	}
	// Block never becomes followup_message (a forced continue).
	if out, err := c.EncodeOutput(EventStop, HookOutput{Block: true, Reason: "r"}); err != nil || out != nil {
		t.Errorf("stop block = %s, %v; want silence", out, err)
	}
	// ADR-038 Deny: Cursor's flat permission shape, pre-tool only.
	out, err := c.EncodeOutput(EventPreTool, HookOutput{Deny: true, DenyReason: "rule X"})
	if err != nil || !reflect.DeepEqual(decodeJSON(t, out), map[string]any{"permission": "deny", "user_message": "rule X", "agent_message": "rule X"}) {
		t.Errorf("deny = %s, %v", out, err)
	}
	if _, err := c.EncodeOutput(EventPostTool, HookOutput{Deny: true}); err == nil {
		t.Error("deny off the pre-tool event must be an error (cannot answer)")
	}
	if _, err := c.EncodeOutput(EventSubagentStart, HookOutput{Context: "x"}); err == nil {
		t.Error("an unbound event must be an error")
	}
}

// TestCopilotCodecRoundTrip (GL-8): Copilot's Claude-format stdin decodes
// into the neutral envelope and the output encodes into its flat fields.
func TestCopilotCodecRoundTrip(t *testing.T) {
	if wired, ok := CodecFor("copilot"); !ok || wired.ID() != "copilot" {
		t.Fatal("copilot must have its codec")
	}
	c := copilotCodec()
	in, err := c.DecodeInput(EventPostTool, []byte(`{"session_id":"s1","cwd":"/w","transcript_path":"/t","tool_name":"edit","tool_input":{"path":"a.go"}}`))
	if err != nil || in.SessionID != "s1" || in.Cwd != "/w" || in.FilePath != "a.go" || in.Tool != ToolFileEdit {
		t.Errorf("PostToolUse decoded = %+v, %v", in, err)
	}
	in, _ = c.DecodeInput(EventSessionStart, []byte(`{"session_id":"s1","cwd":"/w","source":"startup"}`))
	if in.Source != "startup" {
		t.Errorf("SessionStart source = %q", in.Source)
	}
	in, _ = c.DecodeInput(EventStop, []byte(`{"session_id":"s1","cwd":"/w","stop_hook_active":true}`))
	if !in.ReEntrant {
		t.Error("stop_hook_active must mark re-entry")
	}
	in, _ = c.DecodeInput(EventPreTool, []byte(`{"session_id":"s1","cwd":"/w","tool_name":"bash","tool_input":{"command":"ls"}}`))
	if in.Command != "ls" || in.Tool != ToolShell {
		t.Errorf("PreToolUse bash decoded = %+v", in)
	}

	for _, ev := range []Event{EventSessionStart, EventPostTool} {
		out, err := c.EncodeOutput(ev, HookOutput{Context: "rules"})
		if err != nil || !reflect.DeepEqual(decodeJSON(t, out), map[string]any{"additionalContext": "rules"}) {
			t.Errorf("%s context = %s, %v", ev, out, err)
		}
	}
	if out, err := c.EncodeOutput(EventPreTool, HookOutput{Context: "rules"}); err != nil || out != nil {
		t.Errorf("PreToolUse has no context field: %s, %v", out, err)
	}
	out, err := c.EncodeOutput(EventStop, HookOutput{Block: true, Reason: "claim unproven"})
	if err != nil || !reflect.DeepEqual(decodeJSON(t, out), map[string]any{"decision": "block", "reason": "claim unproven"}) {
		t.Errorf("stop block = %s, %v", out, err)
	}
	out, err = c.EncodeOutput(EventPreTool, HookOutput{Deny: true, DenyReason: "rule X"})
	if err != nil || !reflect.DeepEqual(decodeJSON(t, out), map[string]any{"permissionDecision": "deny", "permissionDecisionReason": "rule X"}) {
		t.Errorf("deny = %s, %v", out, err)
	}
}

// TestRenderFlatHooks: both flat shapes, one entry per binding, calling
// the given binary; a nested-shape descriptor is not flat.
func TestRenderFlatHooks(t *testing.T) {
	cur, _ := Lookup("cursor")
	raw, ok, err := RenderFlatHooks(cur, "/opt/fathom")
	if !ok || err != nil {
		t.Fatalf("cursor flat render: ok=%v err=%v", ok, err)
	}
	var doc struct {
		Version int                          `json:"version"`
		Hooks   map[string][]cursorHookEntry `json:"hooks"`
	}
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatal(err)
	}
	if doc.Version != 1 || len(doc.Hooks["sessionStart"]) != 1 || doc.Hooks["sessionStart"][0].Command != "/opt/fathom session-hook --harness cursor" || doc.Hooks["sessionStart"][0].Timeout != 10 {
		t.Errorf("cursor hooks = %s", raw)
	}
	if strings.Contains(string(raw), `"type"`) || strings.Contains(string(raw), `"hooks": [`) {
		t.Errorf("cursor entries must be flat, no nested hooks group: %s", raw)
	}
	if e := doc.Hooks["preToolUse"]; len(e) != 1 || e[0].Matcher != "Write" {
		t.Errorf("preToolUse entry = %+v, want matcher Write", e)
	}

	cop, _ := Lookup("copilot")
	raw, ok, err = RenderFlatHooks(cop, "fathom")
	if !ok || err != nil {
		t.Fatalf("copilot flat render: ok=%v err=%v", ok, err)
	}
	var cdoc struct {
		Version int                           `json:"version"`
		Hooks   map[string][]copilotHookEntry `json:"hooks"`
	}
	if err := json.Unmarshal(raw, &cdoc); err != nil {
		t.Fatal(err)
	}
	if e := cdoc.Hooks["SessionStart"]; cdoc.Version != 1 || len(e) != 1 || e[0].Type != "command" || e[0].Bash != "fathom session-hook --harness copilot" || e[0].TimeoutSec != 10 {
		t.Errorf("copilot hooks = %s", raw)
	}

	cc, _ := Lookup(ClaudeCode)
	if _, ok, _ := RenderFlatHooks(cc, "fathom"); ok {
		t.Error("claude-code's settings shape is nested, not flat")
	}
}
