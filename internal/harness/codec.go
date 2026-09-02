package harness

import (
	"encoding/json"
	"fmt"
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
}

// HookOutput is what a hook verb wants the harness to do with its payload.
// Context is injected; Block (+ Reason) is honored only by the codec of a
// harness whose event can block, and only the stop verb behind --block
// ever sets it (§5.3 inject-only).
type HookOutput struct {
	Context string
	Block   bool
	Reason  string
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

// claudeEnvelope is the union of every stdin field fathom's hook verbs
// read, per event. Claude Code sends one flat object with hook_event_name
// naming the event; unknown fields are ignored, absent ones stay zero.
type claudeEnvelope struct {
	SessionID      string `json:"session_id"`
	TranscriptPath string `json:"transcript_path"`
	CWD            string `json:"cwd"`
	Source         string `json:"source"`
	Reason         string `json:"reason"`
	Trigger        string `json:"trigger"`
	ToolName       string `json:"tool_name"`
	ToolInput      struct {
		FilePath string `json:"file_path"`
		Command  string `json:"command"`
	} `json:"tool_input"`
	StopHookActive bool `json:"stop_hook_active"`
	// InstructionsLoaded / ConfigChange carry file_path at the top level
	// (hooks reference §7.2, §7.11), beside memory_type and load_reason.
	FilePath   string `json:"file_path"`
	MemoryType string `json:"memory_type"`
	LoadReason string `json:"load_reason"`
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
		ReEntrant:      env.StopHookActive,
		MemoryType:     env.MemoryType,
		LoadReason:     env.LoadReason,
	}
	if ev == EventInstructionsLoaded || ev == EventConfigChange {
		// Only these two events document a top-level file_path; a tool
		// event's file lives under tool_input, and the fallback is never
		// crossed (a field the documentation does not show is not read).
		in.FilePath = env.FilePath
	}
	return in, nil
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
	if out.Block {
		// Top-level decision: the Stop contract (also accepted on the other
		// blockable events). Only the stop verb behind --block ever asks.
		return json.Marshal(map[string]any{"decision": "block", "reason": out.Reason})
	}
	if out.Context == "" {
		return nil, nil
	}
	name, ok := claudeEventName(ev)
	if !ok {
		return nil, fmt.Errorf("claude-code: no binding for event %q", ev)
	}
	return json.Marshal(map[string]any{
		"hookSpecificOutput": map[string]any{
			"hookEventName":     name,
			"additionalContext": out.Context,
		},
	})
}
