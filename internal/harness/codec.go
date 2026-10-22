package harness

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
)

// HookInput is the harness-neutral envelope every hook verb consumes.
// A codec fills what the harness's event documents; everything else is
// the zero value, and the verbs already treat "" as "unknown, carry on"
// (guardstamp.go's leakage bound documents why an empty session id is a
// safe, honest fallback).
type HookInput struct {
	Event          Event
	SessionID      string
	Cwd            string
	TranscriptPath string
	// Source is the session-start source in the harness's own vocabulary
	// (Claude Code: startup|resume|clear|compact|fork); the primer's
	// startup/clear gate reads it.
	Source string
	// Reason is the session-end reason (Claude Code: clear|logout|exit|
	// prompt_input_exit|other).
	Reason string
	// Trigger is the compaction trigger (manual|auto).
	Trigger   string
	ToolName  string
	ToolInput string
	Tool      ToolClass
	// FilePath is the edited file on tool events (tool_input.file_path)
	// and the loaded or changed file on the instructions-loaded and
	// config-change events (top-level file_path).
	FilePath string
	Command  string
	// MemoryType and LoadReason are the instructions-loaded event's own
	// fields (Claude Code: "User"|"Project"|"Local"|"Managed";
	// session_start|nested_traversal|path_glob_match|include|compact).
	MemoryType string
	LoadReason string
	// ReEntrant is Claude Code's stop_hook_active: the stop hook is already
	// continuing the session and must not loop.
	ReEntrant bool
	// AgentType is the subagent's type on the subagent-start event
	// (Claude Code: agent_type); "" when the harness does not name one.
	AgentType string
	// AgentID is the new subagent's own id on the subagent-start event
	// (Claude Code: agent_id). It is the only moment fathom sees the id: the
	// subagent's Bash environment does not carry it, and a subagent that
	// files exploration-map entries needs it for `--agent` (XM-8, XM-14).
	// It is also read on the subagent-stop event and on the post-tool and
	// tool-failure events fired inside a subagent (RO-V1), where the test-run
	// row records it. "" on every other event, in the main agent, and when
	// the harness does not name one.
	AgentID string
	// AgentTranscriptPath is the subagent's own transcript on the
	// subagent-stop event (Claude Code: agent_transcript_path, read by RO-V1
	// on 2.1.302); the claim check reads the agent's last tool call from it
	// (O1230). "" on every other event and when absent.
	AgentTranscriptPath string
	// LastMessage is the agent's final assistant message on the stop and
	// subagent-stop events (Claude Code: last_assistant_message, read by
	// RO-V1 on 2.1.302); "" on every other event and when absent.
	LastMessage string
	// ToolError is the failure message on the tool-failure event.
	ToolError string
	// ToolOutput is the text a shell tool call printed (TR-2, TR-V1):
	// Claude Code's PostToolUse tool_response stdout then stderr, or its
	// PostToolUseFailure error text after the leading "Exit code N" line.
	// "" for a tool whose result is not shell output.
	ToolOutput string
	// ExitCode is the shell command's exit status, set only when the
	// envelope carries it (Claude Code: PostToolUseFailure's leading
	// "Exit code N"); nil otherwise — PostToolUse never carries one.
	ExitCode *int
	// OutputTruncated reports that the harness cut ToolOutput and kept the
	// rest elsewhere (Claude Code: tool_response.persistedOutputPath).
	OutputTruncated bool
	// Shell is a shell tool call's output envelope as the harness carries
	// it, the streams kept apart (the readout hook reads it; ToolOutput is
	// the joined text every other reader takes). Zero when the event
	// carries no shell output.
	Shell ShellOutput
	// ToolUseID is the harness's id for this tool call (Claude Code:
	// tool_use_id on PreToolUse, PostToolUse and PostToolUseFailure); ""
	// when absent.
	ToolUseID string
	// Path is the directory an event is ABOUT rather than the session's
	// own cwd: the new worktree's root (worktree-create), the root the
	// session moved to or added (cwd-changed), or a Grep/Glob tool call's
	// own tool_input.path argument (RC-56, the file/dir the call searched
	// — never its pattern, which is not a location).
	Path string
	// Prompt is the user's own submitted text on the user-prompt-submit
	// event (Claude Code: UserPromptSubmit) — O57 items 4/5. Empty on
	// every other event; the harness does not document it elsewhere.
	Prompt string
	// NewContent is the text a file-edit tool call is about to WRITE
	// (ADR-038, wave G1's enforced pre-write checks): Write's whole
	// tool_input.content, or Edit/MultiEdit's new_string(s) concatenated
	// with "\n" — enough for a deterministic content check without
	// reconstructing the whole post-edit file. Empty on every event that
	// is not a file-edit tool call.
	NewContent string
	// OldContent is the text Edit/MultiEdit is about to REPLACE
	// (old_string(s), concatenated the same way as NewContent) — "" for
	// Write, which replaces the whole file. Lets a check see something
	// being REMOVED that NewContent alone cannot show (e.g. a deleted doc
	// section).
	OldContent string
	// ReadRange reports that a Read tool call names a range rather than
	// the whole file (Claude Code: tool_input.offset or tool_input.limit
	// present and not null). Read on PreToolUse only (H8's outline hint).
	ReadRange  bool
	ReadOffset int
	ReadLimit  int
}

// ShellOutput is a shell tool call's result with its streams apart (Claude
// Code's Bash tool_response: stdout, stderr, interrupted,
// persistedOutputPath). On a failure event the harness carries one joined
// text, so Stdout holds it and Stderr is "".
type ShellOutput struct {
	Stdout      string
	Stderr      string
	Interrupted bool
	// Persisted is the file the harness saved an over-cap output to.
	Persisted string
	// IsImage is Bash's isImage flag, carried so a view can copy the whole
	// output envelope.
	IsImage bool
}

// HookOutput is what a hook verb wants the harness to do with its payload.
// Context is injected; Block (+ Reason) is honored only by the codec of a
// harness whose event can block, and only the stop verb behind --block
// ever sets it. Deny (+ DenyReason) is ADR-038's narrow amendment to §5.3
// inject-only: only the PreToolUse guard hook, only for a file/command
// governed by an enforce:true guard or layer rule, ever sets it — every
// other output on every other verb and event stays inject-only, unchanged.
type HookOutput struct {
	Context string
	Block   bool
	Reason  string
	// Deny asks the codec to render a PERMISSION decision (Claude Code /
	// Codex: hookSpecificOutput.permissionDecision:"deny" +
	// permissionDecisionReason) instead of an advisory injection. Only
	// meaningful on EventPreTool; a codec that does not support it (or is
	// asked for it on any other event) returns an error, which the calling
	// verb treats as "cannot answer" — i.e. silence, never a crash and
	// never a fallback to advisory (see edithook_pre.go).
	Deny bool
	// DenyReason is the one-line reason shown in place of the tool call —
	// the rule, its citation, and the remedy (design decision, ADR-038
	// item 3: never a wall of text).
	DenyReason string
	// View replaces the shell tool's result (RO-19): Claude Code renders it
	// as hookSpecificOutput.updatedToolOutput on PostToolUse, with the full
	// Bash envelope (stdout, stderr, interrupted, isImage) taken from the
	// caller's copy of the input and only Stdout and Stderr replaced.
	// Persisted is never emitted. Every other event and codec refuses it
	// with an error, which the calling verb treats as silence. Nothing sets
	// it yet.
	View *ShellOutput
	// UpdatedInput is a Claude Code PreToolUse input rewrite. It makes no permission decision.
	UpdatedInput json.RawMessage
}

// Codec is the ONLY per-harness code: envelope in, envelope out.
type Codec interface {
	// ID is the registry id this codec speaks.
	ID() string
	// DecodeInput parses the harness's stdin JSON for a canonical event.
	DecodeInput(ev Event, raw []byte) (HookInput, error)
	// EncodeOutput renders the harness's stdout JSON for a canonical
	// event. nil, nil means "print nothing" — the verbs' silence contract.
	EncodeOutput(ev Event, out HookOutput) ([]byte, error)
}

// CodecFor returns the codec wired for a registry id. ok is false for an
// unknown id AND for a descriptor that is data only (no codec landed):
// the hook verbs treat both as I9 fail-open — one stderr line, exit 0.
func CodecFor(id string) (Codec, bool) {
	d, ok := Lookup(id)
	if !ok {
		return nil, false
	}
	switch d.Codec {
	case "claude":
		return ClaudeCodec(), true
	case "codex":
		return CodexCodec(), true
	case "gemini":
		return GeminiCodec(), true
	case "cursor":
		return CursorCodec(), true
	case "copilot":
		return CopilotCodec(), true
	}
	return nil, false
}

// claudeCodec speaks Claude Code's hook JSON — the de-facto interchange
// envelope (vision 25 §1). Field names are the verified contract of
// 2026-09-02 (hooks reference, v2.1.251 text). Its encoders build the
// SAME map shapes the four pre-refactor sites built, so encoding/json's
// sorted-key output is byte-identical to what the goldens pinned.
type claudeCodec struct{}

// ClaudeCodec returns the Claude Code codec.
func ClaudeCodec() Codec { return claudeCodec{} }

func (claudeCodec) ID() string { return ClaudeCode }

// claudeToolInput is a tool_use event's own tool_input object — the union
// of every shape fathom's hook verbs read across Bash, Grep/Glob, and the
// three file-edit tools (Write/Edit/MultiEdit).
type claudeToolInput struct {
	FilePath string `json:"file_path"`
	Command  string `json:"command"`
	// Path is Grep/Glob's own tool_input.path (RC-56) — the file or
	// directory argument, never Pattern (unread: a search pattern is not a
	// location, docs/vision/28-receipts.md §4).
	Path string `json:"path"`
	// Content is Write's whole new file body. OldString/NewString are
	// Edit's replaced/replacement text; Edits is MultiEdit's own list of
	// the same pair (ADR-038, wave G1's enforced pre-write checks — see
	// HookInput.NewContent/OldContent).
	Content   string `json:"content"`
	OldString string `json:"old_string"`
	NewString string `json:"new_string"`
	Edits     []struct {
		OldString string `json:"old_string"`
		NewString string `json:"new_string"`
	} `json:"edits"`
	// Offset and Limit are Read's own range (H8's outline hint), kept raw
	// so a present key and an absent one differ.
	Offset json.RawMessage `json:"offset"`
	Limit  json.RawMessage `json:"limit"`
}

// claudeEnvelope is the union of every stdin field fathom's hook verbs
// read, per event. Claude Code sends one flat object with hook_event_name
// naming the event; unknown fields are ignored, absent ones stay zero.
type claudeEnvelope struct {
	SessionID      string          `json:"session_id"`
	TranscriptPath string          `json:"transcript_path"`
	CWD            string          `json:"cwd"`
	Source         string          `json:"source"`
	Reason         string          `json:"reason"`
	Trigger        string          `json:"trigger"`
	ToolName       string          `json:"tool_name"`
	ToolInput      claudeToolInput `json:"tool_input"`
	StopHookActive bool            `json:"stop_hook_active"`
	// InstructionsLoaded / ConfigChange carry file_path at the top level
	// (hooks reference §7.2, §7.11), beside memory_type and load_reason.
	FilePath   string `json:"file_path"`
	MemoryType string `json:"memory_type"`
	LoadReason string `json:"load_reason"`
	// The wave-3 advisory events' own fields (hooks reference, fetched
	// 2026-09-12 against Claude Code 2.1.270). Each is read ONLY on the
	// event that documents it — a field the documentation does not show
	// on an event is not read there.
	AgentID   string `json:"agent_id"`   // SubagentStart, SubagentStop, a subagent's tool events (RO-V1)
	AgentType string `json:"agent_type"` // SubagentStart, SubagentStop
	// Error is PostToolUseFailure's failure text (TR-V1 read the key from
	// Claude Code 2.1.284's hook-input construction); ToolError is the
	// key this codec read before that reading, kept as a fallback.
	Error        string          `json:"error"`          // PostToolUseFailure
	ToolError    string          `json:"tool_error"`     // PostToolUseFailure (pre-TR-V1 key)
	ToolUseID    string          `json:"tool_use_id"`    // PostToolUseFailure
	ToolResponse json.RawMessage `json:"tool_response"`  // PostToolUse
	WorktreePath string          `json:"worktree_path"`  // WorktreeCreate
	PreviousCwd  string          `json:"previous_cwd"`   // CwdChanged
	DirPath      string          `json:"directory_path"` // DirectoryAdded
	HowAdded     string          `json:"how_added"`      // DirectoryAdded
	TriggeredBy  string          `json:"triggered_by"`   // Setup
	Prompt       string          `json:"prompt"`         // UserPromptSubmit
	// LastAssistantMessage is Stop's and SubagentStop's final message
	// (RO-V1, docs/bench/readings/readouts/stop-facts.md).
	LastAssistantMessage string `json:"last_assistant_message"` // Stop, SubagentStop
	AgentTranscriptPath  string `json:"agent_transcript_path"`  // SubagentStop
}

func (c claudeCodec) DecodeInput(ev Event, raw []byte) (HookInput, error) {
	var env claudeEnvelope
	if err := json.Unmarshal(raw, &env); err != nil {
		return HookInput{}, err
	}
	in := HookInput{
		Event:          ev,
		SessionID:      env.SessionID,
		Cwd:            env.CWD,
		TranscriptPath: env.TranscriptPath,
		Source:         env.Source,
		Reason:         env.Reason,
		Trigger:        env.Trigger,
		ToolName:       env.ToolName,
		Tool:           ClassifyTool(ClaudeCode, env.ToolName),
		FilePath:       env.ToolInput.FilePath,
		Command:        env.ToolInput.Command,
		Path:           env.ToolInput.Path,
		ReEntrant:      env.StopHookActive,
		NewContent:     claudeProposedNewContent(env.ToolInput),
		OldContent:     claudeProposedOldContent(env.ToolInput),
		MemoryType:     env.MemoryType,
		LoadReason:     env.LoadReason,
	}
	switch ev {
	case EventInstructionsLoaded, EventConfigChange, EventFileChanged:
		// Only these events document a top-level file_path; a tool event's
		// file lives under tool_input, and the fallback is never crossed
		// (a field the documentation does not show is not read).
		in.FilePath = env.FilePath
	case EventSubagentStart:
		in.AgentType = env.AgentType
		in.AgentID = env.AgentID
		in.Source = env.AgentID
	case EventStop:
		in.LastMessage = env.LastAssistantMessage
	case EventSubagentStop:
		// RO-V1: SubagentStop carries the subagent's own agent_id and
		// agent_type and its final message.
		in.LastMessage = env.LastAssistantMessage
		in.AgentType = env.AgentType
		in.AgentID = env.AgentID
		in.AgentTranscriptPath = env.AgentTranscriptPath
	case EventToolFailure:
		in.AgentID = env.AgentID // RO-V1: a hook fired inside a subagent carries agent_id
		in.ToolError = env.Error
		if in.ToolError == "" {
			in.ToolError = env.ToolError
		}
		in.ToolOutput, in.ExitCode = claudeFailureOutput(in.ToolError)
		in.Shell = ShellOutput{Stdout: in.ToolOutput}
		in.ToolUseID = env.ToolUseID
	case EventPostTool:
		in.AgentID = env.AgentID // RO-V1: a hook fired inside a subagent carries agent_id
		in.ToolOutput, in.OutputTruncated = claudeShellOutput(env.ToolResponse)
		in.Shell = claudeShellStreams(env.ToolResponse)
		in.ToolUseID = env.ToolUseID
	case EventWorktreeCreate:
		in.Path = env.WorktreePath
	case EventCwdChanged:
		// One canonical event, two harness events: CwdChanged names the
		// new root in cwd (previous_cwd is the old one), DirectoryAdded
		// names it in directory_path and says how it arrived.
		if env.DirPath != "" {
			in.Path = env.DirPath
			in.Source = env.HowAdded
		} else {
			in.Path = env.CWD
			in.Reason = env.PreviousCwd
		}
	case EventSetup:
		in.Trigger = env.TriggeredBy
	case EventUserPromptSubmit:
		in.Prompt = env.Prompt
	case EventPreTool:
		var toolEnvelope struct {
			Input json.RawMessage `json:"tool_input"`
		}
		_ = json.Unmarshal(raw, &toolEnvelope)
		if env.ToolName == "Agent" || env.ToolName == "Task" {
			in.ToolInput = string(toolEnvelope.Input)
		}
		in.AgentID = env.AgentID
		_ = json.Unmarshal(env.ToolInput.Offset, &in.ReadOffset)
		_ = json.Unmarshal(env.ToolInput.Limit, &in.ReadLimit)
		in.ReadRange = claudeRawSet(env.ToolInput.Offset) || claudeRawSet(env.ToolInput.Limit)
		in.ToolUseID = env.ToolUseID
	}
	return in, nil
}

// claudeRawSet reports whether a raw envelope field is present and not null.
func claudeRawSet(v json.RawMessage) bool {
	s := strings.TrimSpace(string(v))
	return s != "" && s != "null"
}

// claudeShellOutput reads a Bash tool_response ({stdout, stderr,
// interrupted, isImage, ..., persistedOutputPath}): stdout, then stderr
// on its own line, and whether the harness persisted a cut output
// elsewhere. Any other tool's result reads as no shell output.
func claudeShellOutput(raw json.RawMessage) (string, bool) {
	if len(raw) == 0 || raw[0] != '{' {
		return "", false
	}
	var r struct {
		Stdout              *string `json:"stdout"`
		Stderr              string  `json:"stderr"`
		PersistedOutputPath string  `json:"persistedOutputPath"`
	}
	if json.Unmarshal(raw, &r) != nil || r.Stdout == nil {
		return "", false
	}
	out := *r.Stdout
	if r.Stderr != "" {
		if out != "" {
			out += "\n"
		}
		out += r.Stderr
	}
	return out, r.PersistedOutputPath != ""
}

// claudeShellStreams reads the same Bash tool_response with its streams
// apart; any other tool's result reads as the zero ShellOutput.
func claudeShellStreams(raw json.RawMessage) ShellOutput {
	if len(raw) == 0 || raw[0] != '{' {
		return ShellOutput{}
	}
	var r struct {
		Stdout              *string `json:"stdout"`
		Stderr              string  `json:"stderr"`
		Interrupted         bool    `json:"interrupted"`
		IsImage             bool    `json:"isImage"`
		PersistedOutputPath string  `json:"persistedOutputPath"`
	}
	if json.Unmarshal(raw, &r) != nil || r.Stdout == nil {
		return ShellOutput{}
	}
	return ShellOutput{Stdout: *r.Stdout, Stderr: r.Stderr, Interrupted: r.Interrupted, IsImage: r.IsImage, Persisted: r.PersistedOutputPath}
}

// claudeFailureOutput splits PostToolUseFailure's error text: a leading
// "Exit code N" line is the exit status (the only place Claude Code
// carries one, TR-V1) and the rest is the command's output; without that
// line the whole text is the output and the exit code is unknown.
func claudeFailureOutput(errText string) (string, *int) {
	first, rest, _ := strings.Cut(errText, "\n")
	if n, ok := strings.CutPrefix(strings.TrimSpace(first), "Exit code "); ok {
		if code, err := strconv.Atoi(n); err == nil {
			return rest, &code
		}
	}
	return errText, nil
}

// claudeProposedNewContent returns the text a Claude Code Edit/Write/
// MultiEdit tool_input is about to WRITE: Write's whole content, Edit's
// new_string, or MultiEdit's edits[].new_string joined with "\n" — in that
// preference order, so a tool_input carrying more than one shape (should
// not happen; defensive) still yields a sensible answer. "" when the
// envelope carries none of the three.
func claudeProposedNewContent(ti claudeToolInput) string {
	if ti.Content != "" {
		return ti.Content
	}
	if len(ti.Edits) > 0 {
		parts := make([]string, len(ti.Edits))
		for i, e := range ti.Edits {
			parts[i] = e.NewString
		}
		return strings.Join(parts, "\n")
	}
	return ti.NewString
}

// claudeProposedOldContent is claudeProposedNewContent's mirror for the
// text being REPLACED: "" for Write (which replaces the whole file, not a
// fragment), Edit's old_string, or MultiEdit's edits[].old_string joined
// with "\n".
func claudeProposedOldContent(ti claudeToolInput) string {
	if len(ti.Edits) > 0 {
		parts := make([]string, len(ti.Edits))
		for i, e := range ti.Edits {
			parts[i] = e.OldString
		}
		return strings.Join(parts, "\n")
	}
	return ti.OldString
}

// claudeEventName is the hookEventName Claude Code expects back for each
// canonical event fathom answers on.
func claudeEventName(ev Event) (string, bool) {
	d, _ := Lookup(ClaudeCode)
	if b, ok := d.Binding(ev); ok {
		return b.Harness, true
	}
	return "", false
}

func (c claudeCodec) EncodeOutput(ev Event, out HookOutput) ([]byte, error) {
	if out.View != nil {
		// RO-19: updatedToolOutput exists on PostToolUse only. A value that
		// does not match the tool's schema is ignored by the harness, so the
		// whole Bash envelope is always written.
		if ev != EventPostTool || out.Deny || out.Block {
			return nil, fmt.Errorf("claude-code: a view is only valid alone on PostToolUse, got %q", ev)
		}
		return json.Marshal(map[string]any{
			"hookSpecificOutput": map[string]any{
				"hookEventName": "PostToolUse",
				"updatedToolOutput": map[string]any{
					"stdout":      out.View.Stdout,
					"stderr":      out.View.Stderr,
					"interrupted": out.View.Interrupted,
					"isImage":     out.View.IsImage,
				},
			},
		})
	}
	if out.Deny {
		// ADR-038: hookSpecificOutput.permissionDecision:"deny" +
		// permissionDecisionReason — the documented PreToolUse deny shape
		// (see internal/harness/testdata/codecs/codex/pre-tool.deny.json,
		// verbatim; Claude Code's own hooks reference documents the
		// identical field names). Only PreToolUse supports a permission
		// decision at all; asking for Deny on any other event is a caller
		// bug, reported as an error (the verb's own "cannot answer" ⇒
		// silence contract), never a decision Claude Code would reject.
		if ev != EventPreTool {
			return nil, fmt.Errorf("claude-code: deny is only valid on PreToolUse, got %q", ev)
		}
		return json.Marshal(map[string]any{
			"hookSpecificOutput": map[string]any{
				"hookEventName":            "PreToolUse",
				"permissionDecision":       "deny",
				"permissionDecisionReason": out.DenyReason,
			},
		})
	}
	if out.Block {
		// Top-level decision: the Stop contract (also accepted on the other
		// blockable events). Only the stop verb behind --block ever asks.
		return json.Marshal(map[string]any{"decision": "block", "reason": out.Reason})
	}
	if len(out.UpdatedInput) > 0 {
		if ev != EventPreTool {
			return nil, fmt.Errorf("claude-code: updatedInput requires PreToolUse")
		}
		var input map[string]json.RawMessage
		if json.Unmarshal(out.UpdatedInput, &input) != nil || input == nil {
			return nil, fmt.Errorf("claude-code: updatedInput requires an object")
		}
		specific := map[string]any{"hookEventName": "PreToolUse", "updatedInput": input}
		if out.Context != "" {
			specific["additionalContext"] = out.Context
		}
		return json.Marshal(map[string]any{"hookSpecificOutput": specific})
	}
	if out.Context == "" {
		return nil, nil
	}
	if claudeStdoutIsData[ev] {
		// On WorktreeCreate the harness reads the hook's STDOUT as the
		// worktree path — any byte we print becomes a path it then refuses
		// (O34, second form). Nothing may be emitted; the advisory goes to
		// stderr in the caller.
		return nil, nil
	}
	name, ok := claudeEventName(ev)
	if !ok {
		return nil, fmt.Errorf("claude-code: no binding for event %q", ev)
	}
	if !claudeContextEvents[ev] {
		// Claude Code validates hookSpecificOutput per event and REJECTS
		// additionalContext where it is undocumented — on WorktreeCreate
		// that rejection blocked every worktree-isolated agent spawn
		// (O34, 2026-09-13). The top-level systemMessage is accepted on
		// every event and reaches the user, which is what an advisory is
		// for; the model never sees it, and these events have no model
		// turn to inject into anyway.
		return json.Marshal(map[string]any{"systemMessage": out.Context})
	}
	return json.Marshal(map[string]any{
		"hookSpecificOutput": map[string]any{
			"hookEventName":     name,
			"additionalContext": out.Context,
		},
	})
}

// claudeStdoutIsData names the events whose stdout the harness parses as a
// value rather than as a hook envelope (WorktreeCreate: the custom worktree
// path). The codec emits nothing for them.
var claudeStdoutIsData = map[Event]bool{
	EventWorktreeCreate: true,
}

// claudeContextEvents are the events on which Claude Code documents
// hookSpecificOutput.additionalContext (2.1.270: SessionStart,
// UserPromptSubmit, PreToolUse, PostToolUse, PostToolBatch,
// PostToolUseFailure, SubagentStart, Stop, SubagentStop; RO-V1 read Stop's
// and SubagentStop's reaching the model on 2.1.302). Every other event
// gets systemMessage. Extending this set without the harness documenting
// the field re-creates O34.
var claudeContextEvents = map[Event]bool{
	EventSessionStart:     true,
	EventPreTool:          true,
	EventPostTool:         true,
	EventStop:             true,
	EventSubagentStart:    true,
	EventToolFailure:      true,
	EventUserPromptSubmit: true,
	EventSubagentStop:     true,
}
