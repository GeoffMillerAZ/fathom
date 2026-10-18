package harness

// codec_cursor.go — GL-7 (docs/plans/governance-learning.md, design doc
// §4): the Cursor codec. Cursor's hooks (cursor.com/docs/hooks.md, read
// 2026-09-02; the descriptor's notes) are NOT Claude-shaped: the hook file
// `.cursor/hooks.json` is FLAT — {version:1, hooks:{<camelCase event>:
// [{command, matcher, timeout}]}} with no nested "hooks" group — and every
// output field is snake_case at the top level:
//
//   - sessionStart and afterFileEdit/postToolUse inject through
//     `additional_context` (whether afterFileEdit honors it is UNSHOWN; the
//     descriptor says so);
//   - preToolUse / beforeShellExecution answer a permission, flat:
//     {"permission":"deny","user_message":…,"agent_message":…} — rendered
//     only for ADR-038's explicit Deny, never for an ordinary output, and
//     no context channel exists there (§5.3), so ordinary output is silence;
//   - stop's only output, `followup_message`, auto-continues the agent,
//     which fathom never asks for: a --block verdict is silence here.
//
// Stdin carries conversation_id as the session identity (session_id only on
// sessionStart), workspace_roots in place of a single cwd on most events,
// afterFileEdit's top-level file_path, and preToolUse's tool_input.

import (
	"encoding/json"
	"fmt"
)

// cursorHookCodec speaks Cursor's flat snake_case hook JSON.
type cursorHookCodec struct{}

// CursorCodec returns the Cursor codec.
func CursorCodec() Codec { return cursorCodec() }

// cursorCodec builds the Cursor codec — the one constructor every path
// (CodecFor, the exported CursorCodec) goes through.
func cursorCodec() Codec { return cursorHookCodec{} }

func (cursorHookCodec) ID() string { return "cursor" }

// cursorEnvelope is the union of every stdin field fathom's hook verbs read
// from Cursor.
type cursorEnvelope struct {
	SessionID      string   `json:"session_id"`
	ConversationID string   `json:"conversation_id"`
	TranscriptPath string   `json:"transcript_path"`
	CWD            string   `json:"cwd"`
	WorkspaceRoots []string `json:"workspace_roots"`
	FilePath       string   `json:"file_path"`
	Command        string   `json:"command"`
	ToolName       string   `json:"tool_name"`
	ToolInput      struct {
		FilePath string `json:"file_path"`
		Command  string `json:"command"`
	} `json:"tool_input"`
	Edits []struct {
		OldString string `json:"old_string"`
		NewString string `json:"new_string"`
	} `json:"edits"`
	Status    string `json:"status"`
	LoopCount int    `json:"loop_count"`
}

func (cursorHookCodec) DecodeInput(ev Event, raw []byte) (HookInput, error) {
	var env cursorEnvelope
	if err := json.Unmarshal(raw, &env); err != nil {
		return HookInput{}, err
	}
	session := env.SessionID
	if session == "" {
		session = env.ConversationID
	}
	cwd := env.CWD
	if cwd == "" && len(env.WorkspaceRoots) > 0 {
		cwd = env.WorkspaceRoots[0]
	}
	file := env.FilePath
	if file == "" {
		file = env.ToolInput.FilePath
	}
	cmd := env.Command
	if cmd == "" {
		cmd = env.ToolInput.Command
	}
	in := HookInput{
		Event:          ev,
		SessionID:      session,
		Cwd:            cwd,
		TranscriptPath: env.TranscriptPath,
		ToolName:       env.ToolName,
		Tool:           ClassifyTool("cursor", env.ToolName),
		FilePath:       file,
		Command:        cmd,
		// A stop hook already inside a follow-up loop is re-entrant.
		ReEntrant: ev == EventStop && env.LoopCount > 0,
	}
	if in.ToolName == "" && cmd != "" {
		in.Tool = ToolShell
	}
	for i, e := range env.Edits {
		if i > 0 {
			in.NewContent += "\n"
			in.OldContent += "\n"
		}
		in.NewContent += e.NewString
		in.OldContent += e.OldString
	}
	return in, nil
}

// cursorContextEvent reports the canonical events whose Cursor output
// documents additional_context.
func cursorContextEvent(ev Event) bool { return ev == EventSessionStart || ev == EventPostTool }

func (cursorHookCodec) EncodeOutput(ev Event, out HookOutput) ([]byte, error) {
	if out.View != nil {
		return nil, fmt.Errorf("cursor: a view is not supported")
	}
	d, _ := Lookup("cursor")
	if _, ok := d.Binding(ev); !ok {
		return nil, fmt.Errorf("cursor: no binding for event %q", ev)
	}
	if out.Deny {
		if ev != EventPreTool {
			return nil, fmt.Errorf("cursor: deny is only rendered on the pre-tool event")
		}
		return json.Marshal(map[string]any{
			"permission":    "deny",
			"user_message":  out.DenyReason,
			"agent_message": out.DenyReason,
		})
	}
	if out.Block {
		// followup_message would auto-continue the agent (§5.3 refuses a
		// forced retry); Cursor documents no advisory stop output.
		return nil, nil
	}
	if out.Context == "" || !cursorContextEvent(ev) {
		return nil, nil
	}
	return json.Marshal(map[string]any{"additional_context": out.Context})
}
