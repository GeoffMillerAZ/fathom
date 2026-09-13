// SB-11: an Arm is data, never code (types.go). RenderArm is the one place
// that turns that data into the files a harness actually reads, and
// StripFathom is its inverse — removing a wired worktree's fathom wiring so
// the SAME clone can serve as the control, instead of a second clone that
// could drift from it.
package scenariobench

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/geoffmilleraz/fathom/internal/wiring"
)

// ArmFiles is what RenderArm materialized into one worktree for one arm:
// the harness inputs a SessionRequest is built from, recorded so a caller
// (or a test) can see exactly what was written without re-deriving it.
type ArmFiles struct {
	// SettingsPath is .claude/settings.json; it carries a "hooks" key only
	// when the arm is wired.
	SettingsPath string `json:"settings_path"`
	// MCPConfigPath is a json file holding the mcpServers block — always
	// written, even when arm.MCP is empty, so a harness invocation always
	// has a --mcp-config path to pass (context-bench.sh's EMPTY_MCP_CONFIG
	// pattern: an explicit empty config, never an absent flag).
	MCPConfigPath string `json:"mcp_config_path"`
	// AllowedTools / DisallowedTools are sorted copies of the arm's lists,
	// handed to the harness verbatim (SessionRequest carries the Arm
	// itself, so these are also derivable from it — kept here so a caller
	// can read them without re-deriving).
	AllowedTools    []string `json:"allowed_tools,omitempty"`
	DisallowedTools []string `json:"disallowed_tools,omitempty"`
	// InstructionsPath is the worktree's CLAUDE.md path; empty when
	// Instructions == "none" removed it.
	InstructionsPath string `json:"instructions_path,omitempty"`
	// SystemAppend is the arm-declared text every session's system prompt
	// carries in addition to the task prompt: the notes-file instruction
	// when arm.NotesFile is set, else empty. This — and nothing else — is
	// what may differ between arms in what the agent is told (SB-13).
	SystemAppend string `json:"system_append,omitempty"`
}

// notesInstructionText is the SAME "keep notes" instruction every arm with
// NotesFile gets, word for word, so the bench measures fathom's continuity
// against a real alternative and not "having any notes" (vision 27 §2).
const notesInstructionText = "Keep a running log of your plan and progress in NOTES.md at the repo root, and consult it at the start of every session before doing anything else."

// RenderArm turns arm into the harness inputs a worktree needs:
//   - .claude/settings.json, carrying a "hooks" key only when arm.Wired
//     (a wired arm's REAL hooks are written by `fathom init` in
//     PrepareSubject; this stanza is what marks a directly-rendered arm
//     as wired for RenderArm's own callers and tests, and is itself
//     exactly what StripFathom removes);
//   - an MCP config file holding arm.MCP's mcpServers block, written even
//     when it is empty;
//   - sorted copies of the allowed/disallowed tool lists;
//   - the instruction-file variant selected by arm.Instructions: "plain"
//     leaves the subject's own CLAUDE.md untouched, "none" removes it,
//     and "stanza" requires arm.Wired (only a wired arm's `fathom init`
//     run puts a real stanza there — RenderArm just asserts the
//     precondition and reports the path);
//   - the notes-file instruction text, when arm.NotesFile is set.
func RenderArm(arm Arm, worktree string) (ArmFiles, error) {
	var out ArmFiles

	claudeDir := filepath.Join(worktree, ".claude")
	if err := os.MkdirAll(claudeDir, 0o755); err != nil {
		return ArmFiles{}, fmt.Errorf("rendering arm: creating %s: %w", claudeDir, err)
	}

	settings := map[string]any{}
	if arm.Wired {
		settings["hooks"] = scenarioHooksStanza()
	}
	settingsPath := filepath.Join(claudeDir, "settings.json")
	if err := writeJSONFile(settingsPath, settings); err != nil {
		return ArmFiles{}, fmt.Errorf("rendering arm: %w", err)
	}
	out.SettingsPath = settingsPath

	mcpServers := arm.MCP
	if mcpServers == nil {
		mcpServers = map[string]any{}
	}
	mcpPath := filepath.Join(claudeDir, "scenariobench-mcp.json")
	if err := writeJSONFile(mcpPath, map[string]any{"mcpServers": mcpServers}); err != nil {
		return ArmFiles{}, fmt.Errorf("rendering arm: %w", err)
	}
	out.MCPConfigPath = mcpPath

	out.AllowedTools = sortedCopy(arm.AllowedTools)
	out.DisallowedTools = sortedCopy(arm.DisallowedTools)

	claudeMD := filepath.Join(worktree, "CLAUDE.md")
	switch arm.Instructions {
	case "none":
		if err := os.Remove(claudeMD); err != nil && !os.IsNotExist(err) {
			return ArmFiles{}, fmt.Errorf("rendering arm: removing %s: %w", claudeMD, err)
		}
	case "plain":
		if _, err := os.Stat(claudeMD); err == nil {
			out.InstructionsPath = claudeMD
		} else if !os.IsNotExist(err) {
			return ArmFiles{}, fmt.Errorf("rendering arm: statting %s: %w", claudeMD, err)
		}
	case "stanza":
		if !arm.Wired {
			return ArmFiles{}, fmt.Errorf("rendering arm: instructions %q requires a wired arm (only `fathom init` writes a real stanza)", arm.Instructions)
		}
		out.InstructionsPath = claudeMD
	default:
		return ArmFiles{}, fmt.Errorf("rendering arm: unknown instructions %q (want stanza, plain, or none)", arm.Instructions)
	}

	if arm.NotesFile {
		out.SystemAppend = notesInstructionText
	}

	return out, nil
}

// scenarioHooksStanza is the minimal wired-arm hooks block RenderArm
// writes. It is deliberately small: the point is that the file carries a
// "hooks" key at all when wired and none when not, so StripFathom's
// removal and the control comparison are unambiguous. A cell prepared by
// PrepareSubject for a wired arm calls `fathom init` first — which writes
// the real hooks — and RenderArm's own settings.json write happens before
// that, so the real wiring is what a rendered wired worktree ends up with.
func scenarioHooksStanza() map[string]any {
	return map[string]any{
		"PostToolUse": []any{
			map[string]any{
				"matcher": "Edit|Write|MultiEdit",
				"hooks": []any{
					map[string]any{"type": "command", "command": "fathom obligations-hook"},
				},
			},
		},
	}
}

// StripFathom removes fathom's wiring from a worktree PrepareSubject wired
// for a treatment arm, so the same worktree can serve as the control
// instead of a second clone that could drift from it: the marked stanza in
// CLAUDE.md/AGENTS.md (wiring.HTMLMarkers), the "hooks" key in
// .claude/settings.json, and the "fathom" entry in <worktree>/.mcp.json —
// the project MCP registration `fathom init` writes for Claude Code. Absent
// files or keys are not an error: a worktree that was never wired has
// nothing to strip.
func StripFathom(worktree string) error {
	for _, name := range []string{"CLAUDE.md", "AGENTS.md"} {
		if err := stripMarkedRegion(filepath.Join(worktree, name)); err != nil {
			return fmt.Errorf("stripping fathom: %w", err)
		}
	}
	if err := stripSettingsHooks(filepath.Join(worktree, ".claude", "settings.json")); err != nil {
		return fmt.Errorf("stripping fathom: %w", err)
	}
	if err := stripMCPEntry(filepath.Join(worktree, ".mcp.json"), "fathom"); err != nil {
		return fmt.Errorf("stripping fathom: %w", err)
	}
	return nil
}

// stripMarkedRegion removes the fathom-owned region of path (marker format
// v3, HTML comment syntax), consuming one surrounding blank line so the
// removal does not leave one behind. A file with no marker, or that does
// not exist, is left alone. A file that is empty after stripping is
// removed entirely.
func stripMarkedRegion(path string) error {
	raw, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("reading %s: %w", path, err)
	}
	text := string(raw)
	i := strings.Index(text, wiring.HTMLMarkers.BeginPrefix)
	if i < 0 {
		return nil
	}
	rel := strings.Index(text[i:], wiring.HTMLMarkers.End)
	if rel < 0 {
		return fmt.Errorf("%s: a begin marker with no matching end marker", path)
	}
	j := i + rel + len(wiring.HTMLMarkers.End)
	if j < len(text) && text[j] == '\n' {
		j++
	}
	if i > 0 && text[i-1] == '\n' {
		i--
	}
	remaining := text[:i] + text[j:]
	if strings.TrimSpace(remaining) == "" {
		return os.Remove(path)
	}
	return os.WriteFile(path, []byte(remaining), 0o644)
}

// stripSettingsHooks removes the top-level "hooks" key from the
// Claude Code settings.json at path. Absent file or key: no-op. A file
// left with no keys at all is removed rather than kept as "{}".
func stripSettingsHooks(path string) error {
	raw, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("reading %s: %w", path, err)
	}
	var settings map[string]any
	if err := json.Unmarshal(raw, &settings); err != nil {
		return fmt.Errorf("parsing %s: %w", path, err)
	}
	if _, ok := settings["hooks"]; !ok {
		return nil
	}
	delete(settings, "hooks")
	if len(settings) == 0 {
		return os.Remove(path)
	}
	return writeJSONFile(path, settings)
}

// stripMCPEntry removes the named entry from the mcpServers block of the
// Claude Code project MCP file at path. Absent file or entry: no-op. A
// file left with no servers at all is removed.
func stripMCPEntry(path, name string) error {
	raw, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("reading %s: %w", path, err)
	}
	var doc struct {
		MCPServers map[string]any `json:"mcpServers"`
	}
	if err := json.Unmarshal(raw, &doc); err != nil {
		return fmt.Errorf("parsing %s: %w", path, err)
	}
	if _, ok := doc.MCPServers[name]; !ok {
		return nil
	}
	delete(doc.MCPServers, name)
	if len(doc.MCPServers) == 0 {
		return os.Remove(path)
	}
	return writeJSONFile(path, doc)
}

func sortedCopy(in []string) []string {
	if len(in) == 0 {
		return nil
	}
	out := append([]string(nil), in...)
	sort.Strings(out)
	return out
}

func writeJSONFile(path string, v any) error {
	raw, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling %s: %w", path, err)
	}
	return os.WriteFile(path, append(raw, '\n'), 0o644)
}
