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
	// MCPServerNames is the sorted, deduped set of server names actually
	// written into MCPConfigPath's mcpServers block — including a wired
	// arm's merged-in "fathom" entry (wireScratchDaemon runs before
	// RenderArm), regardless of whether AllowedTools was ever set. This is
	// what the instrument-truth gate (RunSessions) checks a session's own
	// SessionResult.MCPServers against, and what AllowedTools' own
	// "mcp__<server>" augmentation below is derived from.
	MCPServerNames []string `json:"mcp_server_names,omitempty"`
	// AllowedTools / DisallowedTools are sorted copies of the arm's lists
	// — AllowedTools additionally carries "mcp__<server>" for every name
	// in MCPServerNames not already present, so a non-empty allow-list
	// always covers the MCP surface this arm was also handed (never a
	// silent gap between what was wired and what was permitted).
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
//   - .claude/settings.json: for a wired arm, `fathom init` (PrepareSubject's
//     wireScratchDaemon, which runs BEFORE RenderArm) has already written
//     the REAL hook stack there — SessionStart/PreToolUse/PostToolUse/Stop —
//     and RenderArm only asserts it is present (assertWiredHooksPresent),
//     NEVER replaces it with a stand-in; an unwired arm gets an empty `{}`
//     (StripFathom's own "hooks" removal is what undoes a wired one);
//   - an MCP config file holding arm.MCP's mcpServers block, written even
//     when it is empty;
//   - sorted copies of the allowed/disallowed tool lists;
//   - the instruction-file variant selected by arm.Instructions: "plain"
//     leaves the subject's own CLAUDE.md untouched, "none" removes it,
//     "stanza" requires arm.Wired (only a wired arm's `fathom init`
//     run puts a real stanza there — RenderArm just asserts the
//     precondition and reports the path), and "text" writes
//     arm.InstructionsText verbatim (a competitor arm's own routing
//     stanza, RC-8);
//   - the notes-file instruction text, when arm.NotesFile is set.
func RenderArm(arm Arm, worktree string) (ArmFiles, error) {
	var out ArmFiles

	claudeDir := filepath.Join(worktree, ".claude")
	if err := os.MkdirAll(claudeDir, 0o755); err != nil {
		return ArmFiles{}, fmt.Errorf("rendering arm: creating %s: %w", claudeDir, err)
	}

	settingsPath := filepath.Join(claudeDir, "settings.json")
	if arm.Wired {
		// `fathom init` (PrepareSubject's wireScratchDaemon) already wrote
		// the real hook stack here, BEFORE RenderArm runs — replacing it
		// with a stand-in (the defect this closes) left every wired cell
		// running with none of fathom's actual hooks. Assert it, never
		// rewrite it.
		if err := assertWiredHooksPresent(settingsPath); err != nil {
			return ArmFiles{}, fmt.Errorf("rendering arm: %w", err)
		}
	} else if err := writeJSONFile(settingsPath, map[string]any{}); err != nil {
		return ArmFiles{}, fmt.Errorf("rendering arm: %w", err)
	}
	out.SettingsPath = settingsPath

	mcpServers := normalizeMCPServers(arm.MCP)
	mcpPath := filepath.Join(claudeDir, "scenariobench-mcp.json")
	if err := writeJSONFile(mcpPath, map[string]any{"mcpServers": mcpServers}); err != nil {
		return ArmFiles{}, fmt.Errorf("rendering arm: %w", err)
	}
	out.MCPConfigPath = mcpPath

	serverNames := make([]string, 0, len(mcpServers))
	for name := range mcpServers {
		serverNames = append(serverNames, name)
	}
	sort.Strings(serverNames)
	out.MCPServerNames = serverNames

	out.AllowedTools = sortedCopy(arm.AllowedTools)
	if len(out.AllowedTools) > 0 {
		have := map[string]bool{}
		for _, t := range out.AllowedTools {
			have[t] = true
		}
		for _, name := range serverNames {
			tool := "mcp__" + name
			if !have[tool] {
				have[tool] = true
				out.AllowedTools = append(out.AllowedTools, tool)
			}
		}
		sort.Strings(out.AllowedTools)
	}
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
	case "text":
		// A competitor arm's own routing stanza (RC-8): arm.InstructionsText
		// verbatim, so its tool is never measured as unused for want of a
		// name (vision 28 §2 F2) — the same mechanism "stanza" gives a
		// wired arm, without requiring `fathom init`.
		if arm.InstructionsText == "" {
			return ArmFiles{}, fmt.Errorf("rendering arm: instructions %q requires arm.InstructionsText", arm.Instructions)
		}
		if err := os.WriteFile(claudeMD, []byte(arm.InstructionsText), 0o644); err != nil {
			return ArmFiles{}, fmt.Errorf("rendering arm: writing %s: %w", claudeMD, err)
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

// assertWiredHooksPresent reads the settings.json a wired cell's `fathom
// init` (PrepareSubject's wireScratchDaemon) already wrote at path — BEFORE
// RenderArm ever runs — and errors, naming path, unless it carries a
// "hooks" key with at least one SessionStart entry: RenderArm's own proof
// that the real hook stack (SessionStart/PreToolUse/PostToolUse/Stop) is
// what a wired cell's harness will actually see, never a stand-in RenderArm
// invented itself (the defect this closes: every wired cell used to run
// under a single fake PostToolUse hook naming a fathom subcommand that does
// not exist, with no SessionStart/PreToolUse/Stop hooks at all).
//
// It also refuses a hook command naming a bare binary on PATH rather than
// an absolute path: `fathom init`'s default binPath resolution
// (os.Executable(), cmd/fathom/init.go's resolveBinPath) already produces
// the absolute path of whatever binary is running it — the cell's own
// <binDir>/fathom, since wireScratchDaemon execs that binary directly,
// never through PATH — so a bare command here means something upstream
// changed that contract; erroring loudly beats silently guessing at the
// intended absolute path.
func assertWiredHooksPresent(path string) error {
	raw, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("a wired arm requires %s to already exist (PrepareSubject's wireScratchDaemon runs `fathom init` before RenderArm): %w", path, err)
	}
	var settings map[string]any
	if err := json.Unmarshal(raw, &settings); err != nil {
		return fmt.Errorf("%s: parsing: %w", path, err)
	}
	hooksObj, _ := settings["hooks"].(map[string]any)
	sessionStart, ok := hooksObj["SessionStart"].([]any)
	if !ok || len(sessionStart) == 0 {
		return fmt.Errorf("%s: a wired arm's settings.json must carry a hooks.SessionStart entry from `fathom init` (found none — was `fathom init` run before RenderArm?)", path)
	}
	if bad := firstBareHookCommand(hooksObj); bad != "" {
		return fmt.Errorf("%s: hook command %q names a bare binary on PATH, not the cell's own fathom binary (fathom init's binPath resolution should have written an absolute path)", path, bad)
	}
	return nil
}

// firstBareHookCommand walks every hook entry in a Claude Code
// hooks-object (event -> [{matcher, hooks:[{command,...}]}]) and returns
// the first command whose leading token has no "/" — a bare name that
// would resolve through PATH at run time instead of naming a specific
// binary. "" when every command is a path.
func firstBareHookCommand(hooksObj map[string]any) string {
	for _, groupsRaw := range hooksObj {
		groups, _ := groupsRaw.([]any)
		for _, g := range groups {
			gm, ok := g.(map[string]any)
			if !ok {
				continue
			}
			hs, _ := gm["hooks"].([]any)
			for _, h := range hs {
				hm, ok := h.(map[string]any)
				if !ok {
					continue
				}
				cmd, _ := hm["command"].(string)
				fields := strings.Fields(cmd)
				if len(fields) == 0 {
					continue
				}
				if !strings.Contains(fields[0], "/") {
					return cmd
				}
			}
		}
	}
	return ""
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

// normalizeMCPServers returns a fresh mcpServers map (never mutating
// arm.MCP's own nested maps — they may be shared by another cell/trial):
// nil becomes an empty map (RenderArm always writes an MCP config), and
// every entry shaped as a map gets "alwaysLoad": true when it does not
// already carry that key. Claude Code 2.1.x defers MCP tool schemas
// behind ToolSearch unless an entry says otherwise, so a competitor arm
// connected but never called (docs/bench/scenarios.md "Instrument truth")
// is this default's second defect, not a stanza problem. An explicit
// alwaysLoad — true or false — is never overwritten.
func normalizeMCPServers(mcp map[string]any) map[string]any {
	out := make(map[string]any, len(mcp))
	for name, v := range mcp {
		entry, ok := v.(map[string]any)
		if !ok {
			out[name] = v
			continue
		}
		copied := make(map[string]any, len(entry)+1)
		for k, vv := range entry {
			copied[k] = vv
		}
		if _, has := copied["alwaysLoad"]; !has {
			copied["alwaysLoad"] = true
		}
		out[name] = copied
	}
	return out
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
