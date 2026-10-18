package harness

// codec_gemini.go — the Gemini CLI codec (HP-19, ADR-029 §5). Gemini's
// hooks (geminicli.com/docs/hooks, reference.md e09410b 2026-04-10;
// release v0.58.0 2026-09-01; a local install exists on this workstation)
// are Claude-shaped in nesting and in the hookSpecificOutput.additionalContext
// injection, with three documented differences this codec encodes:
//
//   - The decision is FLAT: {"decision":"allow"|"deny","reason":…} at the
//     top level, never under hookSpecificOutput. fathom never emits deny
//     (§5.3 inject-only): a --block verdict on AfterAgent is rendered as
//     a systemMessage — advisory, shown to the user — never a forced retry.
//   - Parse failure means ALLOW: "The CLI will default to Allow and treat
//     the entire output as a systemMessage." Recorded as the descriptor's
//     failMode; the codec's own contract is therefore "valid JSON or
//     nothing" — nil, nil is the only silence.
//   - additionalContext is documented for SessionStart, BeforeAgent and
//     AfterTool only. On BeforeTool and AfterAgent it is UNSHOWN, so the
//     guard's and the stop verb's text goes out as systemMessage (the one
//     output field every event documents) — the user's terminal, not the
//     model. The descriptor's notes say exactly that.
//
// Per-tool tool_input shapes are UNSHOWN in the hooks docs; file_path is
// read because the built-in write_file / replace tools take that argument
// name (docs/tools/file-system) — the one assumption here, named.

import (
	"encoding/json"
	"fmt"
)

// geminiCodec speaks Gemini CLI's hook JSON.
type geminiCodec struct{}

// GeminiCodec returns the Gemini CLI codec.
func GeminiCodec() Codec { return geminiCodec{} }

func (geminiCodec) ID() string { return GeminiCLI }

// geminiEnvelope is the union of every stdin field fathom's hook verbs
// read from Gemini CLI (reference.md base schema + per-event fields).
type geminiEnvelope struct {
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
}

func (c geminiCodec) DecodeInput(ev Event, raw []byte) (HookInput, error) {
	var env geminiEnvelope
	if err := json.Unmarshal(raw, &env); err != nil {
		return HookInput{}, err
	}
	return HookInput{
		Event:          ev,
		SessionID:      env.SessionID,
		Cwd:            env.CWD,
		TranscriptPath: env.TranscriptPath,
		Source:         env.Source,
		Reason:         env.Reason,
		Trigger:        env.Trigger,
		ToolName:       env.ToolName,
		Tool:           ClassifyTool(GeminiCLI, env.ToolName),
		FilePath:       env.ToolInput.FilePath,
		Command:        env.ToolInput.Command,
		ReEntrant:      env.StopHookActive,
	}, nil
}

// geminiContextEvents are the canonical events whose Gemini output
// documents hookSpecificOutput.additionalContext (SessionStart, AfterTool).
// Every other event gets systemMessage: documented on all of them.
var geminiContextEvents = map[Event]bool{EventSessionStart: true, EventPostTool: true}

func geminiEventName(ev Event) (string, bool) {
	d, _ := Lookup(GeminiCLI)
	if b, ok := d.Binding(ev); ok {
		return b.Harness, true
	}
	return "", false
}

func (c geminiCodec) EncodeOutput(ev Event, out HookOutput) ([]byte, error) {
	if out.View != nil {
		return nil, fmt.Errorf("gemini-cli: a view is not supported")
	}
	if _, ok := geminiEventName(ev); !ok {
		return nil, fmt.Errorf("gemini-cli: no binding for event %q", ev)
	}
	if out.Deny {
		// Gemini CLI's BeforeTool leaves additionalContext UNSHOWN (this
		// file's own doc comment) and its documented flat
		// {"decision":"deny",...} shape belongs to AfterAgent, not a tool
		// gate — ADR-038's deny path is not extended here. An error is the
		// same "cannot answer" ⇒ silence contract every other codec-cannot-
		// render case already uses; it never silently downgrades a denial
		// to an advisory the agent could miss.
		return nil, fmt.Errorf("gemini-cli: deny is not supported")
	}
	if out.Block {
		// Never deny (§5.3): a proven-failing claim is SHOWN, not forced
		// into a retry loop. Gemini's AfterAgent deny would re-prompt the
		// model with reason; fathom's stop verdict stays advisory here.
		return json.Marshal(map[string]any{"systemMessage": out.Reason})
	}
	if out.Context == "" {
		return nil, nil
	}
	if geminiContextEvents[ev] {
		name, _ := geminiEventName(ev)
		return json.Marshal(map[string]any{
			"hookSpecificOutput": map[string]any{
				"hookEventName":     name,
				"additionalContext": out.Context,
			},
		})
	}
	return json.Marshal(map[string]any{"systemMessage": out.Context})
}
