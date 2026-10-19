package harness

import (
	"os"
	"path/filepath"
)

// ClaudeConfigRoot is Claude Code's per-user config root, the directory that
// holds settings.json, CLAUDE.md and the projects/ transcript and memory
// tree: $CLAUDE_CONFIG_DIR when set, else home/.claude; "" when neither
// resolves. Every reader of that tree resolves it here (O1194), so none
// reads ~/.claude while Claude Code writes elsewhere.
func ClaudeConfigRoot(home string) string {
	if d := os.Getenv("CLAUDE_CONFIG_DIR"); d != "" {
		return d
	}
	if home == "" {
		return ""
	}
	return filepath.Join(home, ".claude")
}

// ClaudeUserConfigFile is Claude Code's user config file, which carries the
// user- and local-scope MCP registrations: $CLAUDE_CONFIG_DIR/.claude.json
// when the variable is set, else home/.claude.json (beside the root, not in
// it); "" when neither resolves.
func ClaudeUserConfigFile(home string) string {
	if d := os.Getenv("CLAUDE_CONFIG_DIR"); d != "" {
		return filepath.Join(d, ".claude.json")
	}
	if home == "" {
		return ""
	}
	return filepath.Join(home, ".claude.json")
}
