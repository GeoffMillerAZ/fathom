package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// The core plugin's Claude Code mods (docs/design/claude-code-mods.md §3, §7): the scout agent, rendered from the
// registry's default surface. Pure Go and markdown: no module, no model turn.

// pluginScopedToolPrefix is the name Claude Code gives a tool of this plugin's own MCP server,
// mcp__plugin_<plugin>_<server>__<tool>. scripts/plugin-install-check.sh reads the server as plugin:fathom:fathom
// (claude 2.1.302); the tool prefix itself is unverified until the mods plan's reading records it, so render holds
// it here alone.
const pluginScopedToolPrefix = "mcp__plugin_fathom_fathom__"

// scoutAgentPath is the scout's file, relative to the plugin root.
const scoutAgentPath = "agents/fathom-scout.md"

// scoutRoute is the question shape each default-surface tool answers, as the AGENTS.md stanza routes it. A tool with
// no shape here returns false, so a registry change fails the render instead of shipping a scout that cannot name it.
func scoutRoute(tool string) (string, bool) {
	switch tool {
	case "blast_radius":
		return "what breaks when a file or symbol changes: a change, a rename or a delete", true
	case "find_symbol":
		return "where a symbol is defined", true
	case "obligations":
		return "what governs a file, or the repository's root rules with no path", true
	case "read_source":
		return "the current text at a cited line", true
	case "review_diff":
		return "a composed review verdict over a set of changed files", true
	case "search":
		return "a literal string, a config value, a log line, or one document against another (grep with a receipt)", true
	case "symbol_context":
		return "who calls a symbol, briefly: its definition, a capped caller and callee list, its tests", true
	case "verify_plan":
		return "whether a plan's box is proven", true
	}
	return "", false
}

// renderScoutAgent renders agents/fathom-scout.md (§7.2): tools are defaultTools (renderPluginSurface's default
// surface, sorted) by their plugin-scoped names; omitClaudeMd and low effort; a body of routing and the answer
// contract only. Deterministic: the output depends on defaultTools and the text below alone.
func renderScoutAgent(defaultTools []string) ([]byte, error) {
	scoped := make([]string, 0, len(defaultTools))
	var routing strings.Builder
	for _, tool := range defaultTools {
		shape, ok := scoutRoute(tool)
		if !ok {
			return nil, fmt.Errorf("scout: no routing line for default-surface tool %q", tool)
		}
		scoped = append(scoped, pluginScopedToolPrefix+tool)
		fmt.Fprintf(&routing, "- `%s`: %s.\n", tool, shape)
	}

	var b strings.Builder
	b.WriteString("---\n")
	b.WriteString("name: fathom-scout\n")
	b.WriteString("description: Answers one question about indexed code from fathom's index (where a symbol is defined, who calls it, what breaks, what governs a file, whether a plan's box is proven) with a file:line receipt per claim. Use it to locate or scope code before reading files.\n")
	b.WriteString("tools: " + strings.Join(scoped, ", ") + "\n")
	b.WriteString("omitClaudeMd: true\n")
	b.WriteString("effort: low\n")
	b.WriteString("---\n")
	b.WriteString("\n")
	b.WriteString("You are fathom's scout. Answer the one question you were given from fathom's index, then stop. You have\n")
	b.WriteString("fathom's tools and no others.\n")
	b.WriteString("\n")
	b.WriteString("Route by question shape:\n")
	b.WriteString("\n")
	b.WriteString(routing.String())
	b.WriteString("\n")
	b.WriteString("Every tool takes `service`: use the one the question names, else the one the session's fathom line names\n")
	b.WriteString("(`this repo is the indexed service ...`); when neither names one, say so instead of guessing.\n")
	b.WriteString("\n")
	b.WriteString("Answer contract:\n")
	b.WriteString("\n")
	b.WriteString("- At most 40 lines.\n")
	b.WriteString("- A `file:line` receipt on every claim, taken from a tool result; a claim without one is left out.\n")
	b.WriteString("- Name the index's freshness as the results report it (the indexed commit, or stale); when no result\n")
	b.WriteString("  reports it, write `freshness unknown`, never fresh.\n")
	return []byte(b.String()), nil
}

// subagentStatusLineCommand is the core plugin's subagent status line: `fathom statusline --subagent` when the binary
// is on PATH, else a line saying it is not, spelled via the shim-stable `fathom` like every hook.
const subagentStatusLineCommand = "command -v fathom >/dev/null 2>&1 && fathom statusline --subagent || echo 'fathom: not on PATH'"

// renderPluginSettings renders the core plugin's settings.json (§3, §7.3): subagentStatusLine alone, since a
// plugin's settings take effect for `agent` and `subagentStatusLine` only and a plugin cannot ship `statusLine`.
func renderPluginSettings() map[string]any {
	return map[string]any{
		"subagentStatusLine": map[string]any{
			"type":    "command",
			"command": subagentStatusLineCommand,
		},
	}
}

// writePluginFile writes data at path, creating its directory.
func writePluginFile(path string, data []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}
