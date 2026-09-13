// The clean room (RC-6, vision 28 §2 F4): every session runs with a
// scratch CLAUDE_CONFIG_DIR carrying no operator hooks, plugins, agents or
// commands, so the second reading's own defect — "every session inherited
// the operator's config: ~90 slash commands, plugins, and fathom's own
// SessionStart primer on the control arm" — cannot recur. A fresh
// CLAUDE_CONFIG_DIR already has no commands/, plugins/ or agents/
// subdirectory (those are what carried the slash commands and plugins),
// so the only files this package needs to seed are ~/.claude.json (the
// account-level config: auth, onboarding, MCP registrations Claude Code
// reads by default) and ~/.claude/settings.json (hooks, enabled plugins).
//
// The FIRST design here stripped mcpServers/projects from the copied
// .claude.json and wrote a blank settings.json ({}); the smoke test this
// plan row required (RC-6) found that broke authentication ("Not logged
// in · Please run /login") even though oauthAccount/onboarding fields
// were kept — macOS's keychain lookup for this CLI did not tolerate the
// scratch config dir once other .claude.json fields were removed. The
// fallback this file implements instead: copy ~/.claude.json nearly
// verbatim (mcpServers is harmless to keep — every real session passes
// `--strict-mcp-config`, which ignores it regardless) and source
// settings.json from the operator's own ~/.claude/settings.json, stripping
// only "hooks" and the plugin-enablement key. Hooks/plugins are the
// mechanism F4 names; mcpServers/projects are not.
package scenariobench

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// cleanRoomStrippedClaudeJSONKeys are removed from the copied
// ~/.claude.json — empty in every install this package has been verified
// against (hooks/plugins live in settings.json, not here), kept as a
// defensive strip in case a future Claude Code version moves them.
var cleanRoomStrippedClaudeJSONKeys = []string{"hooks", "plugins"}

// cleanRoomStrippedSettingsKeys are removed from the copied
// ~/.claude/settings.json: "hooks" (PostToolUse/SessionStart/... beats
// that would otherwise reach every cell) and "enabledPlugins" (plugin
// slash commands) — the two F4 names. "plugins" is stripped defensively
// alongside enabledPlugins in case a version spells the key differently.
var cleanRoomStrippedSettingsKeys = []string{"hooks", "enabledPlugins", "plugins"}

// cleanRoomEnv returns the environment overrides one session's clean room
// needs (RC-6): CLAUDE_CONFIG_DIR pointing at a per-run scratch dir under
// runDir holding a settings.json (copied from homeDir/.claude/settings.json
// with cleanRoomStrippedSettingsKeys removed) and a .claude.json (copied
// from homeDir/.claude.json with cleanRoomStrippedClaudeJSONKeys removed),
// and FATHOM_THREADS pointing at a per-cell threads.jsonl under cellDir.
// runDir is shared (and idempotently rewritten) across every cell of one
// run; cellDir is this cell's own Worktree.CellDir. homeDir is the
// operator's home directory (os.UserHomeDir() at the call site, injectable
// here for tests) — homeDir carrying neither source file is not an error:
// the clean room then simply starts with nothing copied forward (and, per
// the smoke test above, likely unauthenticated — a fresh CLAUDE_CONFIG_DIR
// needs its own `claude /login` once, same as any other machine).
func cleanRoomEnv(runDir, cellDir, homeDir string) (map[string]string, error) {
	configDir := filepath.Join(runDir, "claude-config")
	if err := os.MkdirAll(configDir, 0o755); err != nil {
		return nil, fmt.Errorf("clean room: creating %s: %w", configDir, err)
	}
	if err := copyStrippedJSON(filepath.Join(homeDir, ".claude", "settings.json"), filepath.Join(configDir, "settings.json"), cleanRoomStrippedSettingsKeys); err != nil {
		return nil, fmt.Errorf("clean room: settings.json: %w", err)
	}
	if err := copyStrippedJSON(filepath.Join(homeDir, ".claude.json"), filepath.Join(configDir, ".claude.json"), cleanRoomStrippedClaudeJSONKeys); err != nil {
		return nil, fmt.Errorf("clean room: .claude.json: %w", err)
	}
	if err := os.MkdirAll(cellDir, 0o755); err != nil {
		return nil, fmt.Errorf("clean room: creating %s: %w", cellDir, err)
	}
	return map[string]string{
		"CLAUDE_CONFIG_DIR": configDir,
		"FATHOM_THREADS":    filepath.Join(cellDir, "threads.jsonl"),
	}, nil
}

// copyStrippedJSON parses src as a JSON object, deletes strip's keys from
// it, and writes the result to dest. A homeDir of "" or a src that does
// not exist is not an error: dest is simply never written (an absent file
// is not the same as an empty {} — a caller that cares can stat for it).
func copyStrippedJSON(src, dest string, strip []string) error {
	if src == "" {
		return nil
	}
	raw, err := os.ReadFile(src)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("reading %s: %w", src, err)
	}
	var doc map[string]any
	if err := json.Unmarshal(raw, &doc); err != nil {
		return fmt.Errorf("parsing %s: %w", src, err)
	}
	for _, k := range strip {
		delete(doc, k)
	}
	return writeJSONFile(dest, doc)
}
