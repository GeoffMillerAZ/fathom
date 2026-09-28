package harness

import (
	"encoding/json"
	"fmt"
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
	Trigger  string
	ToolName string
	Tool     ToolClass
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
	// ToolError is the failure message on the tool-failure event.
	ToolError string
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
	AgentID      string `json:"agent_id"`       // SubagentStart
	AgentType    string `json:"agent_type"`     // SubagentStart
	ToolError    string `json:"tool_error"`     // PostToolUseFailure
	ToolUseID    string `json:"tool_use_id"`    // PostToolUseFailure
	WorktreePath string `json:"worktree_path"`  // WorktreeCreate
	PreviousCwd  string `json:"previous_cwd"`   // CwdChanged
	DirPath      string `json:"directory_path"` // DirectoryAdded
	HowAdded     string `json:"how_added"`      // DirectoryAdded
	TriggeredBy  string `json:"triggered_by"`   // Setup
	Prompt       string `json:"prompt"`         // UserPromptSubmit
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
		in.Source = env.AgentID
	case EventToolFailure:
		in.ToolError = env.ToolError
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
	}
	return in, nil
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
// PostToolUseFailure, SubagentStart, Stop, SubagentStop). Every other event
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
}
