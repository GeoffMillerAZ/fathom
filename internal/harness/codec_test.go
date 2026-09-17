package harness

import (
	"encoding/json"
	"strings"
	"testing"
)

// The stdin fixtures are verbatim examples from the Claude Code hooks
// reference (https://code.claude.com/docs/en/hooks, fetched 2026-09-02),
// trimmed to the fields that appear in the doc's own examples.

const claudePreToolUseBash = `{
  "session_id": "abc123",
  "prompt_id": "550e8400-e29b-41d4-a716-446655440000",
  "transcript_path": "/home/user/.claude/projects/.../transcript.jsonl",
  "cwd": "/home/user/my-project",
  "permission_mode": "default",
  "hook_event_name": "PreToolUse",
  "tool_name": "Bash",
  "tool_input": {
    "command": "npm test",
    "description": "Run test suite",
    "timeout": 120000,
    "run_in_background": false
  },
  "tool_use_id": "toolu_01ABC123..."
}`

const claudePostToolUseWrite = `{
  "session_id": "abc123",
  "transcript_path": "/Users/.../.claude/projects/.../00893aaf-19fa-41d2-8238-13269b9b3ca0.jsonl",
  "cwd": "/Users/...",
  "permission_mode": "default",
  "hook_event_name": "PostToolUse",
  "tool_name": "Write",
  "tool_input": {
    "file_path": "/path/to/file.txt",
    "content": "file content"
  },
  "tool_response": {
    "filePath": "/path/to/file.txt",
    "success": true
  },
  "tool_use_id": "toolu_01ABC123...",
  "duration_ms": 12
}`

const claudeStop = `{
  "session_id": "abc123",
  "transcript_path": "~/.claude/projects/.../00893aaf-19fa-41d2-8238-13269b9b3ca0.jsonl",
  "cwd": "/Users/...",
  "permission_mode": "default",
  "hook_event_name": "Stop",
  "stop_hook_active": true,
  "last_assistant_message": "I've completed the refactoring. Here's a summary..."
}`

const claudeSessionStart = `{
  "session_id": "abc123",
  "transcript_path": "/Users/.../.claude/projects/.../00893aaf-19fa-41d2-8238-13269b9b3ca0.jsonl",
  "cwd": "/Users/...",
  "hook_event_name": "SessionStart",
  "source": "resume",
  "model": "claude-opus-5"
}`

const claudeSessionEnd = `{
  "session_id": "abc123",
  "transcript_path": "/Users/.../.claude/projects/.../00893aaf-19fa-41d2-8238-13269b9b3ca0.jsonl",
  "cwd": "/Users/...",
  "hook_event_name": "SessionEnd",
  "reason": "other"
}`

const claudePreCompact = `{
  "session_id": "abc123",
  "transcript_path": "/Users/.../.claude/projects/.../00893aaf-19fa-41d2-8238-13269b9b3ca0.jsonl",
  "cwd": "/Users/...",
  "hook_event_name": "PreCompact",
  "trigger": "manual",
  "custom_instructions": null
}`

func TestClaudeCodecDecodesDocumentedFixtures(t *testing.T) {
	c := ClaudeCodec()
	if c.ID() != ClaudeCode {
		t.Fatalf("ID = %q", c.ID())
	}

	in, err := c.DecodeInput(EventPreTool, []byte(claudePreToolUseBash))
	if err != nil {
		t.Fatal(err)
	}
	if in.Event != EventPreTool || in.SessionID != "abc123" || in.Cwd != "/home/user/my-project" || in.ToolName != "Bash" || in.Tool != ToolShell || in.Command != "npm test" || in.FilePath != "" {
		t.Errorf("PreToolUse Bash decoded wrong: %+v", in)
	}

	in, err = c.DecodeInput(EventPostTool, []byte(claudePostToolUseWrite))
	if err != nil {
		t.Fatal(err)
	}
	if in.Tool != ToolFileEdit || in.FilePath != "/path/to/file.txt" || in.ToolName != "Write" || in.TranscriptPath == "" {
		t.Errorf("PostToolUse Write decoded wrong: %+v", in)
	}

	in, err = c.DecodeInput(EventStop, []byte(claudeStop))
	if err != nil {
		t.Fatal(err)
	}
	if !in.ReEntrant || in.SessionID != "abc123" || in.TranscriptPath != "~/.claude/projects/.../00893aaf-19fa-41d2-8238-13269b9b3ca0.jsonl" {
		t.Errorf("Stop decoded wrong: %+v", in)
	}

	in, err = c.DecodeInput(EventSessionStart, []byte(claudeSessionStart))
	if err != nil {
		t.Fatal(err)
	}
	if in.Source != "resume" || in.SessionID != "abc123" {
		t.Errorf("SessionStart decoded wrong: %+v", in)
	}

	in, err = c.DecodeInput(EventSessionEnd, []byte(claudeSessionEnd))
	if err != nil {
		t.Fatal(err)
	}
	if in.Reason != "other" {
		t.Errorf("SessionEnd decoded wrong: %+v", in)
	}

	in, err = c.DecodeInput(EventPreCompact, []byte(claudePreCompact))
	if err != nil {
		t.Fatal(err)
	}
	if in.Trigger != "manual" {
		t.Errorf("PreCompact decoded wrong: %+v", in)
	}

	// Malformed and empty stdin: an error the verbs turn into silence.
	if _, err := c.DecodeInput(EventPostTool, []byte("not json")); err == nil {
		t.Error("malformed stdin must error")
	}
	if _, err := c.DecodeInput(EventPostTool, nil); err == nil {
		t.Error("empty stdin must error (json.Unmarshal of nothing)")
	}
	in, err = c.DecodeInput(EventPostTool, []byte("{}"))
	if err != nil || in != (HookInput{Event: EventPostTool, Tool: ToolOther}) {
		t.Errorf("empty object must decode to the zero input: %+v %v", in, err)
	}
}

// TestClaudeCodecEncodesPreSeamBytes pins the exact bytes the four
// pre-refactor encode sites produced (json.Marshal of the same map
// shapes: sorted keys, HTML escaping on).
func TestClaudeCodecEncodesPreSeamBytes(t *testing.T) {
	c := ClaudeCodec()
	cases := []struct {
		ev   Event
		out  HookOutput
		want string
	}{
		{EventSessionStart, HookOutput{Context: "✅ live"}, `{"hookSpecificOutput":{"additionalContext":"✅ live","hookEventName":"SessionStart"}}`},
		{EventPostTool, HookOutput{Context: "⚖️ fathom obligations — a.go:"}, `{"hookSpecificOutput":{"additionalContext":"⚖️ fathom obligations — a.go:","hookEventName":"PostToolUse"}}`},
		{EventPreTool, HookOutput{Context: "⚖️ fathom guard — command: x"}, `{"hookSpecificOutput":{"additionalContext":"⚖️ fathom guard — command: x","hookEventName":"PreToolUse"}}`},
		{EventStop, HookOutput{Context: "⛔ done-means-proven"}, `{"hookSpecificOutput":{"additionalContext":"⛔ done-means-proven","hookEventName":"Stop"}}`},
		{EventStop, HookOutput{Block: true, Reason: "T-1 fails"}, `{"decision":"block","reason":"T-1 fails"}`},
		// HTML escaping exactly as json.Marshal always did it.
		{EventPostTool, HookOutput{Context: "a <b> & c"}, `{"hookSpecificOutput":{"additionalContext":"a \u003cb\u003e \u0026 c","hookEventName":"PostToolUse"}}`},
	}
	for _, tc := range cases {
		got, err := c.EncodeOutput(tc.ev, tc.out)
		if err != nil {
			t.Fatalf("%s: %v", tc.ev, err)
		}
		if string(got) != tc.want {
			t.Errorf("%s:\n got %s\nwant %s", tc.ev, got, tc.want)
		}
		if !json.Valid(got) {
			t.Errorf("%s: not valid JSON: %s", tc.ev, got)
		}
	}

	// Silence contract: nothing to say ⇒ nothing to print.
	if got, err := c.EncodeOutput(EventPostTool, HookOutput{}); err != nil || got != nil {
		t.Errorf("empty output = %q, %v; want nil, nil", got, err)
	}
	// An event Claude Code has no binding for cannot be answered.
	if _, err := c.EncodeOutput(Event("nope"), HookOutput{Context: "x"}); err == nil {
		t.Error("unknown event must error")
	}
}

// TestClaudeCodecNeverEmitsDeny pins §5.3 inject-only: no encoded output
// ever carries a permission decision.
func TestClaudeCodecNeverEmitsDeny(t *testing.T) {
	c := ClaudeCodec()
	for _, ev := range []Event{EventSessionStart, EventPreTool, EventPostTool, EventStop} {
		for _, out := range []HookOutput{{Context: "x"}, {Block: true, Reason: "r"}} {
			got, err := c.EncodeOutput(ev, out)
			if err != nil {
				t.Fatal(err)
			}
			if strings.Contains(string(got), "permissionDecision") || strings.Contains(string(got), "deny") {
				t.Errorf("%s: %s", ev, got)
			}
		}
	}
}

// TestClaudeCodecEncodesDenyBytes pins ADR-038's amendment to §5.3
// inject-only, byte-for-byte: a PreToolUse Deny renders EXACTLY the
// documented shape (internal/harness/testdata/codecs/codex/pre-tool.deny.json,
// which this repo's own hooks reference confirms is Claude-shaped too),
// and Deny on any other event is refused rather than silently rendered.
func TestClaudeCodecEncodesDenyBytes(t *testing.T) {
	c := ClaudeCodec()
	got, err := c.EncodeOutput(EventPreTool, HookOutput{Deny: true, DenyReason: "Destructive command blocked by hook."})
	if err != nil {
		t.Fatal(err)
	}
	want := `{"hookSpecificOutput":{"hookEventName":"PreToolUse","permissionDecision":"deny","permissionDecisionReason":"Destructive command blocked by hook."}}`
	if string(got) != want {
		t.Errorf("Deny bytes:\n got %s\nwant %s", got, want)
	}
	if !json.Valid(got) {
		t.Errorf("not valid JSON: %s", got)
	}
	for _, ev := range []Event{EventPostTool, EventStop, EventSessionStart} {
		if _, err := c.EncodeOutput(ev, HookOutput{Deny: true, DenyReason: "x"}); err == nil {
			t.Errorf("%s: Deny must be refused off PreToolUse", ev)
		}
	}
}

// TestClaudeCodecDecodesProposedContent pins the Write/Edit/MultiEdit
// content extraction ADR-038's enforced checks read (HookInput.NewContent/
// OldContent).
func TestClaudeCodecDecodesProposedContent(t *testing.T) {
	c := ClaudeCodec()

	writeIn := `{"session_id":"s","cwd":"/w","hook_event_name":"PreToolUse","tool_name":"Write","tool_input":{"file_path":"/w/a.go","content":"package a\n"}}`
	in, err := c.DecodeInput(EventPreTool, []byte(writeIn))
	if err != nil {
		t.Fatal(err)
	}
	if in.NewContent != "package a\n" || in.OldContent != "" {
		t.Errorf("Write: NewContent=%q OldContent=%q", in.NewContent, in.OldContent)
	}

	editIn := `{"session_id":"s","cwd":"/w","hook_event_name":"PreToolUse","tool_name":"Edit","tool_input":{"file_path":"/w/a.go","old_string":"old","new_string":"new"}}`
	in, err = c.DecodeInput(EventPreTool, []byte(editIn))
	if err != nil {
		t.Fatal(err)
	}
	if in.NewContent != "new" || in.OldContent != "old" {
		t.Errorf("Edit: NewContent=%q OldContent=%q", in.NewContent, in.OldContent)
	}

	multiIn := `{"session_id":"s","cwd":"/w","hook_event_name":"PreToolUse","tool_name":"MultiEdit","tool_input":{"file_path":"/w/a.go","edits":[{"old_string":"o1","new_string":"n1"},{"old_string":"o2","new_string":"n2"}]}}`
	in, err = c.DecodeInput(EventPreTool, []byte(multiIn))
	if err != nil {
		t.Fatal(err)
	}
	if in.NewContent != "n1\nn2" || in.OldContent != "o1\no2" {
		t.Errorf("MultiEdit: NewContent=%q OldContent=%q", in.NewContent, in.OldContent)
	}

	// A non-edit tool call carries neither.
	in, err = c.DecodeInput(EventPreTool, []byte(claudePreToolUseBash))
	if err != nil {
		t.Fatal(err)
	}
	if in.NewContent != "" || in.OldContent != "" {
		t.Errorf("Bash: NewContent=%q OldContent=%q, want both empty", in.NewContent, in.OldContent)
	}
}

func TestCodecFor(t *testing.T) {
	if c, ok := CodecFor(ClaudeCode); !ok || c.ID() != ClaudeCode {
		t.Error("claude-code must have a codec")
	}
	for _, id := range []string{Codex, GeminiCLI} {
		if c, ok := CodecFor(id); !ok || c.ID() != id {
			t.Errorf("%q must have its wave-2 codec", id)
		}
	}
	for _, id := range []string{"cursor", "copilot", Agents, "nope", ""} {
		if _, ok := CodecFor(id); ok {
			t.Errorf("%q must have no codec (data row / pull tier / unknown)", id)
		}
	}
}

// TestClaudeFieldMapsAgreeWithCodec keeps the descriptor's documented
// field maps and the codec's envelope struct from drifting apart: every
// path a Claude Code binding names is one the codec actually reads or
// writes.
func TestClaudeFieldMapsAgreeWithCodec(t *testing.T) {
	reads := map[string]bool{
		"session_id": true, "cwd": true, "transcript_path": true, "source": true, "reason": true,
		"trigger": true, "tool_name": true, "tool_input.file_path": true, "tool_input.command": true,
		"stop_hook_active": true, "file_path": true, "memory_type": true, "load_reason": true,
		// Wave 3 (HP-26..HP-31), each read only on the event that
		// documents it (claudeEnvelope's per-event switch).
		"agent_id": true, "agent_type": true, "tool_error": true, "tool_use_id": true,
		"worktree_path": true, "previous_cwd": true, "directory_path": true,
		"how_added": true, "triggered_by": true,
		// O57 items 4/5.
		"prompt": true,
	}
	writes := map[string]bool{
		"hookSpecificOutput.additionalContext": true, "hookSpecificOutput.hookEventName": true,
		"decision": true, "reason": true,
	}
	d, _ := Lookup(ClaudeCode)
	for _, b := range d.Hooks {
		for k, v := range b.Input {
			if !reads[v] {
				t.Errorf("%s/%s: input %s → %q is not a path the codec reads", b.Harness, b.Matcher, k, v)
			}
		}
		for k, v := range b.Output {
			if !writes[v] {
				t.Errorf("%s/%s: output %s → %q is not a path the codec writes", b.Harness, b.Matcher, k, v)
			}
		}
		if _, ok := d.Binding(b.Event); !ok {
			t.Errorf("Binding(%s) not found", b.Event)
		}
	}
}

// O34: Claude Code rejects additionalContext on events where it is not
// documented (WorktreeCreate's schema accepts only worktreePath), and the
// rejection blocked every worktree-isolated agent spawn. Advisory events
// encode as the top-level systemMessage instead; the documented events keep
// additionalContext byte-for-byte.
func TestClaudeCodecUsesSystemMessageWhereAdditionalContextIsUndocumented(t *testing.T) {
	c, ok := CodecFor(ClaudeCode)
	if !ok {
		t.Fatal("no claude-code codec")
	}
	// WorktreeCreate's stdout IS the worktree path: nothing may be printed.
	if out, err := c.EncodeOutput(EventWorktreeCreate, HookOutput{Context: "advice"}); err != nil || len(out) != 0 {
		t.Errorf("WorktreeCreate must emit nothing on stdout, got %q (%v)", out, err)
	}
	for _, ev := range []Event{EventCwdChanged, EventSetup, EventFileChanged, EventConfigChange, EventInstructionsLoaded} {
		out, err := c.EncodeOutput(ev, HookOutput{Context: "advice"})
		if err != nil {
			t.Fatalf("%s: %v", ev, err)
		}
		var got map[string]any
		if err := json.Unmarshal(out, &got); err != nil {
			t.Fatalf("%s: %v", ev, err)
		}
		if _, has := got["hookSpecificOutput"]; has {
			t.Errorf("%s: emitted hookSpecificOutput, which Claude Code rejects on this event", ev)
		}
		if got["systemMessage"] != "advice" {
			t.Errorf("%s: want systemMessage advice, got %s", ev, out)
		}
	}
	for _, ev := range []Event{EventSessionStart, EventSubagentStart, EventToolFailure, EventPostTool} {
		out, err := c.EncodeOutput(ev, HookOutput{Context: "primer"})
		if err != nil {
			t.Fatalf("%s: %v", ev, err)
		}
		if !strings.Contains(string(out), `"additionalContext":"primer"`) {
			t.Errorf("%s: documented event must keep additionalContext, got %s", ev, out)
		}
	}
}
