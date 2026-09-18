package harness

// codec_codex.go — the Codex CLI codec (HP-17, ADR-029 §5). Codex's hook
// input envelope is Claude-shaped (hooks.md, fetched 2026-09-02, no version
// stamp; local codex-cli 0.146.0), but output is event-specific: Stop uses
// the common systemMessage / decision fields while tool and session events
// use hookSpecificOutput. The Codex-specific differences are encoded here,
// nowhere else:
//
//   - apply_patch reports the whole patch in tool_input.command, not a
//     file_path. The first `*** Update File:` / `*** Add File:` /
//     `*** Delete File:` header names the file the guard and the
//     obligations verb act on (a multi-file patch is guarded by its first
//     file only — documented, not hidden).
//   - additionalContext is capped at ~2,500 tokens per handler
//     (additionalContextLimit); over the cap Codex spills the text to a
//     temp file and shows the model a head-and-tail preview. fathom cuts
//     BEFORE the cap with a named cut (the I12 discipline), so the model
//     sees a coherent head and knows what was dropped, never a spill path.
//
// Everything decoded here is a field the documentation shows; a field it
// does not show is not read. The descriptor stays `unverified` until
// doctor hooks_firing sees beats from a real Codex session.

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"
)

// CodexAdditionalContextCap is the byte budget fathom keeps a Codex
// additionalContext payload under: Codex's default additionalContextLimit
// is ~2,500 tokens and a spilled payload reaches the model as a preview
// plus a temp-file path, which is worse than a named cut. 8,000 bytes is
// ~2,000 tokens at four bytes per token — under the limit with margin for
// the envelope and non-ASCII payload glyphs.
const CodexAdditionalContextCap = 8000

// codexCutMarker names the cut, like the primer's (I12): the reader knows
// text was dropped and why, and the hooks beat log can count it.
const codexCutMarker = "\n… [fathom: cut %d bytes to stay under Codex's additionalContext limit]"

// codexCodec speaks Codex CLI's hook JSON.
type codexCodec struct{}

// CodexCodec returns the Codex CLI codec.
func CodexCodec() Codec { return codexCodec{} }

func (codexCodec) ID() string { return Codex }

// codexEnvelope is the union of every stdin field fathom's hook verbs read
// from Codex, per hooks.md §"Hook input". transcript_path is `string |
// null`; a JSON null leaves the string empty, which is the honest zero.
// tool_input is a JSON value: an object for Bash / apply_patch (command),
// the raw arguments for MCP and local function tools — decoded loosely so
// a non-object never fails the whole envelope.
type codexEnvelope struct {
	SessionID      string          `json:"session_id"`
	TranscriptPath string          `json:"transcript_path"`
	CWD            string          `json:"cwd"`
	Source         string          `json:"source"`
	Reason         string          `json:"reason"`
	Trigger        string          `json:"trigger"`
	ToolName       string          `json:"tool_name"`
	ToolInput      json.RawMessage `json:"tool_input"`
	StopHookActive bool            `json:"stop_hook_active"`
}

func (c codexCodec) DecodeInput(ev Event, raw []byte) (HookInput, error) {
	var env codexEnvelope
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
		Tool:           ClassifyTool(Codex, env.ToolName),
		ReEntrant:      env.StopHookActive,
	}
	command := codexToolCommand(env.ToolInput)
	switch in.Tool {
	case ToolShell:
		in.Command = command
	case ToolFileEdit:
		// apply_patch: the patch IS the command; its first file header is
		// the file the edit verbs act on, resolved against the session cwd
		// (hooks run with the session cwd as their working directory).
		in.Command = command
		if p := ApplyPatchFirstFile(command); p != "" {
			if !filepath.IsAbs(p) && env.CWD != "" {
				p = filepath.Join(env.CWD, p)
			}
			in.FilePath = p
		}
		in.NewContent, in.OldContent = ApplyPatchContent(command)
	}
	return in, nil
}

// codexToolCommand extracts tool_input.command when tool_input is an
// object carrying one; "" for any other shape (MCP/local tool arguments).
func codexToolCommand(raw json.RawMessage) string {
	if len(raw) == 0 {
		return ""
	}
	var obj struct {
		Command string `json:"command"`
	}
	if err := json.Unmarshal(raw, &obj); err != nil {
		return ""
	}
	return obj.Command
}

// applyPatchFileHeaders are the file-naming lines of the apply_patch
// grammar (OpenAI's apply_patch tool: `*** Begin Patch`, then one of these
// per file, then hunks, then `*** End Patch`).
var applyPatchFileHeaders = []string{"*** Update File: ", "*** Add File: ", "*** Delete File: "}

// ApplyPatchFirstFile returns the path named by the first file header of
// an apply_patch patch, or "" when the text carries none. Exported so the
// edit verbs' tests can pin the grammar without going through a codec.
func ApplyPatchFirstFile(patch string) string {
	for _, line := range strings.Split(patch, "\n") {
		line = strings.TrimRight(line, "\r")
		for _, h := range applyPatchFileHeaders {
			if strings.HasPrefix(line, h) {
				return strings.TrimSpace(strings.TrimPrefix(line, h))
			}
		}
	}
	return ""
}

// ApplyPatchContent extracts the added and removed text from an
// apply_patch patch (ADR-038, wave G1's enforced pre-write checks): every
// hunk line beginning with '+' (never "+++") joins `added`; every line
// beginning with '-' (never "---") joins `removed`. Deliberately a plain
// line scan, not a full patch parser — apply_patch's own grammar
// (ApplyPatchFirstFile's applyPatchFileHeaders) is simple enough that this
// is exact for the shapes fathom's checks care about, and a malformed or
// unrecognized patch degrades to two empty strings rather than an error.
func ApplyPatchContent(patch string) (added, removed string) {
	var add, rem []string
	for _, line := range strings.Split(patch, "\n") {
		switch {
		case strings.HasPrefix(line, "+++") || strings.HasPrefix(line, "---"):
			continue
		case strings.HasPrefix(line, "+"):
			add = append(add, strings.TrimPrefix(line, "+"))
		case strings.HasPrefix(line, "-"):
			rem = append(rem, strings.TrimPrefix(line, "-"))
		}
	}
	return strings.Join(add, "\n"), strings.Join(rem, "\n")
}

// codexEventName is the hookEventName Codex expects back for a canonical
// event, from the descriptor's own bindings.
func codexEventName(ev Event) (string, bool) {
	d, _ := Lookup(Codex)
	if b, ok := d.Binding(ev); ok {
		return b.Harness, true
	}
	return "", false
}

func (c codexCodec) EncodeOutput(ev Event, out HookOutput) ([]byte, error) {
	if out.Deny {
		// ADR-038: verbatim against
		// internal/harness/testdata/codecs/codex/pre-tool.deny.json —
		// Codex is Claude-shaped here too (this file's own doc comment).
		if ev != EventPreTool {
			return nil, fmt.Errorf("codex: deny is only valid on PreToolUse, got %q", ev)
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
		// Stop: {"decision":"block","reason":…} creates a continuation
		// prompt from reason (hooks.md §"Stop"). Only the stop verb behind
		// --block ever asks; no permissionDecision is ever emitted (§5.3).
		return json.Marshal(map[string]any{"decision": "block", "reason": out.Reason})
	}
	if out.Context == "" {
		return nil, nil
	}
	if ev == EventStop {
		// Stop accepts the common systemMessage field, not the
		// hookSpecificOutput.additionalContext shape used by context-bearing
		// events. Returning the latter is schema-invalid and makes Codex report
		// "invalid stop hook JSON output" even though the bytes are JSON.
		return json.Marshal(map[string]any{"systemMessage": CapCodexContext(out.Context)})
	}
	name, ok := codexEventName(ev)
	if !ok {
		return nil, fmt.Errorf("codex: no binding for event %q", ev)
	}
	return json.Marshal(map[string]any{
		"hookSpecificOutput": map[string]any{
			"hookEventName":     name,
			"additionalContext": CapCodexContext(out.Context),
		},
	})
}

// CapCodexContext truncates ctx to CodexAdditionalContextCap bytes with a
// named cut. The cut lands on a rune boundary so a multi-byte glyph is
// never split; the marker is charged against the cap, so the result is
// always ≤ the cap. Deterministic and pure.
func CapCodexContext(ctx string) string {
	if len(ctx) <= CodexAdditionalContextCap {
		return ctx
	}
	// Reserve room for the marker with the largest cut count it could name.
	reserve := len(fmt.Sprintf(codexCutMarker, len(ctx)))
	keep := CodexAdditionalContextCap - reserve
	if keep < 0 {
		keep = 0
	}
	for keep > 0 && !isRuneStart(ctx[keep]) {
		keep--
	}
	head := ctx[:keep]
	return head + fmt.Sprintf(codexCutMarker, len(ctx)-len(head))
}

func isRuneStart(b byte) bool { return b&0xC0 != 0x80 }
