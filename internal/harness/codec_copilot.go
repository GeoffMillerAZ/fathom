package harness

// codec_copilot.go — GL-8 (docs/plans/governance-learning.md, design doc
// §4): the GitHub Copilot codec. Copilot's hooks (docs.github.com
// copilot/reference/hooks-reference, github/docs b87a8cf 2026-09-01; the
// descriptor's notes) accept PascalCase event names with the snake_case
// "Claude-format" payload, so stdin decodes like Claude Code's, but the
// OUTPUT is flat:
//
//   - SessionStart and PostToolUse inject through a top-level
//     `additionalContext` (PostToolUse appends it to textResultForLlm,
//     capped at 10 KB by Copilot);
//   - PreToolUse answers flat permissionDecision/permissionDecisionReason —
//     rendered only for ADR-038's explicit Deny; the CLI documents no
//     context field there, so ordinary output is silence;
//   - Stop's `decision:"block"` + `reason` forces another turn, the same
//     contract the Claude codec renders for the stop verb's --block.
//
// tool_input's per-tool shape is UNSHOWN in the reference; file_path and
// command are read because the Claude-format payload names them — the one
// assumption here, named.

import (
	"encoding/json"
	"fmt"
)

// copilotHookCodec speaks GitHub Copilot's hook JSON.
type copilotHookCodec struct{}

// CopilotCodec returns the GitHub Copilot codec.
func CopilotCodec() Codec { return copilotCodec() }

// copilotCodec builds the GitHub Copilot codec — the one constructor every path
// (CodecFor, the exported CopilotCodec) goes through.
func copilotCodec() Codec { return copilotHookCodec{} }

func (copilotHookCodec) ID() string { return "copilot" }

// copilotEnvelope is the union of every stdin field fathom's hook verbs
// read from Copilot.
type copilotEnvelope struct {
	SessionID      string `json:"session_id"`
	TranscriptPath string `json:"transcript_path"`
	CWD            string `json:"cwd"`
	Source         string `json:"source"`
	Reason         string `json:"reason"`
	Trigger        string `json:"trigger"`
	ToolName       string `json:"tool_name"`
	ToolInput      struct {
		FilePath string `json:"file_path"`
		Path     string `json:"path"`
		Command  string `json:"command"`
	} `json:"tool_input"`
	StopHookActive bool `json:"stop_hook_active"`
}

func (copilotHookCodec) DecodeInput(ev Event, raw []byte) (HookInput, error) {
	var env copilotEnvelope
	if err := json.Unmarshal(raw, &env); err != nil {
		return HookInput{}, err
	}
	file := env.ToolInput.FilePath
	if file == "" {
		file = env.ToolInput.Path
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
		Tool:           ClassifyTool("copilot", env.ToolName),
		FilePath:       file,
		Command:        env.ToolInput.Command,
		ReEntrant:      env.StopHookActive,
	}, nil
}

// copilotContextEvent reports the canonical events whose Copilot output
// documents additionalContext.
func copilotContextEvent(ev Event) bool { return ev == EventSessionStart || ev == EventPostTool }

func (copilotHookCodec) EncodeOutput(ev Event, out HookOutput) ([]byte, error) {
	if out.View != nil {
		return nil, fmt.Errorf("copilot: a view is not supported")
	}
	d, _ := Lookup("copilot")
	if _, ok := d.Binding(ev); !ok {
		return nil, fmt.Errorf("copilot: no binding for event %q", ev)
	}
	if out.Deny {
		if ev != EventPreTool {
			return nil, fmt.Errorf("copilot: deny is only rendered on the pre-tool event")
		}
		return json.Marshal(map[string]any{
			"permissionDecision":       "deny",
			"permissionDecisionReason": out.DenyReason,
		})
	}
	if out.Block {
		if ev != EventStop {
			return nil, nil
		}
		return json.Marshal(map[string]any{"decision": "block", "reason": out.Reason})
	}
	if out.Context == "" || !copilotContextEvent(ev) {
		return nil, nil
	}
	return json.Marshal(map[string]any{"additionalContext": out.Context})
}
