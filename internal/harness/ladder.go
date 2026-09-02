package harness

// ladder.go — the degradation ladder's tier B/C/D rows (HP-22, vision 25
// §2.3): harnesses fathom reaches with stanza + MCP + skills and NO
// injection. They are documented rows in the generated table, not
// descriptors with codecs: no `init --harness`, no hook file, no vehicle
// table — just the honest facts an adopter needs and the primary source
// each row was read from (E2 lane report, 2026-09-01). Tier C = hooks exist
// only as in-process JS/TS plugins (the no-Node non-goal refuses a host);
// tier D = no hook mechanism at all; tier B = a block-only hook path with
// no stdout/context contract (§5.3 inject-only refuses emulating a guard
// through it).

import (
	"fmt"
	"strings"
)

// LadderRow is one tier B/C/D harness as data.
type LadderRow struct {
	ID          string `json:"id"`
	Display     string `json:"display"`
	Tier        string `json:"tier"`
	Instruction string `json:"instruction_file"`
	Skills      string `json:"skills_path"`
	MCPConfig   string `json:"mcp_config"`
	// MCPAddLine is the harness's own registration line; {{url}} is the
	// daemon base URL.
	MCPAddLine string `json:"mcp_add_line"`
	// Injection is the honest "no injection here" line: why fathom cannot
	// push context into a session on this harness.
	Injection string `json:"injection"`
	Source    string `json:"source"`
}

// MCPLine renders the row's registration line for url.
func (r LadderRow) MCPLine(url string) string {
	return strings.ReplaceAll(r.MCPAddLine, "{{url}}", strings.TrimRight(url, "/"))
}

// ladder is the fixed-order row list (tier, then the E2 report's order).
var ladder = []LadderRow{
	{
		ID: "windsurf", Display: "Windsurf / Cascade (Devin Desktop)", Tier: "B",
		Instruction: "AGENTS.md (root always-on; subdirectory files glob-activated); .windsurf/rules/*.md",
		Skills:      ".agents/skills/, .windsurf/skills/ (+ ~/.agents/skills/, ~/.codeium/windsurf/skills/)",
		MCPConfig:   "~/.codeium/windsurf/mcp_config.json (user only — no project-level file)",
		MCPAddLine:  "add to ~/.codeium/windsurf/mcp_config.json: {\"mcpServers\":{\"fathom\":{\"serverUrl\":\"{{url}}/rpc/agent\"}}}",
		Injection:   "no injection here: 12 snake_case hooks (.windsurf/hooks.json) block on exit 2 and have no stdout JSON contract — a fathom guard would be block-only, which §5.3 refuses",
		Source:      "https://docs.devin.ai/desktop/cascade/hooks (2026-09-01)",
	},
	{
		ID: "opencode", Display: "OpenCode", Tier: "C",
		Instruction: "AGENTS.md (project, walks parents; ~/.config/opencode/AGENTS.md); CLAUDE.md fallback",
		Skills:      ".opencode/skills/, .claude/skills/, .agents/skills/ (+ ~/.config/opencode/skills/, ~/.agents/skills/)",
		MCPConfig:   "opencode.json(c) mcp.<name> {type:\"remote\", url}",
		MCPAddLine:  "add to opencode.json: {\"mcp\":{\"fathom\":{\"type\":\"remote\",\"url\":\"{{url}}/rpc/agent\"}}}",
		Injection:   "no injection here: hooks are in-process JS/TS plugins (.opencode/plugins/); no stdin/stdout contract and no session-start context hook — a plugin host is the no-Node non-goal",
		Source:      "https://opencode.ai/docs/plugins/ (v1.18.26, 2026-09-01)",
	},
	{
		ID: "amp", Display: "Amp (Sourcegraph)", Tier: "C",
		Instruction: "AGENTS.md > AGENT.md > CLAUDE.md (cwd and parents; ~/.config/amp/AGENTS.md)",
		Skills:      ".agents/skills/, .claude/skills/ (+ ~/.agents/skills/, ~/.config/amp/skills/)",
		MCPConfig:   "amp.mcpServers in ~/.config/amp/settings.json or .amp/settings.json (needs `amp mcp approve`)",
		MCPAddLine:  "amp mcp add fathom {{url}}/rpc/agent",
		Injection:   "no injection here: hooks are TS/JS plugins (.amp/plugins/, session.start/tool.call/agent.end) with no shell-script contract — a plugin host is the no-Node non-goal",
		Source:      "https://ampcode.com/docs/customize/plugins (2026-09-01)",
	},
	{
		ID: "zed", Display: "Zed (ACP client)", Tier: "D",
		Instruction: "first match of .rules, .cursorrules, .windsurfrules, .clinerules, .github/copilot-instructions.md, AGENT.md, AGENTS.md, CLAUDE.md, GEMINI.md — first wins, no merge",
		Skills:      ".agents/skills/ (+ ~/.agents/skills/, v1.4.0+)",
		MCPConfig:   "settings.json context_servers.<name> {url, headers}; forwarded over ACP into session/new",
		MCPAddLine:  "add to Zed settings.json: {\"context_servers\":{\"fathom\":{\"url\":\"{{url}}/rpc/agent\"}}}",
		Injection:   "no injection here: Zed has no hooks and ACP has no hook concept (a client interposes only at session/request_permission, fs/*, terminal/*); fathom reaches ACP agents through MCP passthrough",
		Source:      "https://agentclientprotocol.com/protocol/overview (ACP v1; Zed v1.17.2, 2026-09-01)",
	},
	{
		ID: "junie", Display: "JetBrains Junie", Tier: "D",
		Instruction: "AGENTS.md (root) + .junie/playbook.md + .junie/rules/*.md; ~/.junie/AGENTS.md",
		Skills:      "config.json skill-locations (no default path documented)",
		MCPConfig:   ".junie/mcp/mcp.json (project), ~/.junie/mcp/mcp.json (user)",
		MCPAddLine:  "add to .junie/mcp/mcp.json: {\"mcpServers\":{\"fathom\":{\"url\":\"{{url}}/rpc/agent\"}}}",
		Injection:   "no injection here: config.json `hooks` names a SessionStart example but the event list and the stdin/stdout contract are undocumented — nothing to encode",
		Source:      "https://junie.jetbrains.com/docs/junie-cli-configuration.html (2026-09-01)",
	},
	{
		ID: "warp", Display: "Warp Agent CLI", Tier: "D",
		Instruction: "AGENTS.md (WARP.md legacy wins if both), nested dirs; ~/.agents/AGENTS.md",
		Skills:      ".agents/skills/, .warp/skills/ (+ ~/.agents/skills/)",
		MCPConfig:   ".warp/.mcp.json (app, approval required), ~/.warp/.mcp.json; the CLI reads only ~/.warp_cli/.mcp.json",
		MCPAddLine:  "add to ~/.warp_cli/.mcp.json: {\"mcpServers\":{\"fathom\":{\"url\":\"{{url}}/rpc/agent\"}}}",
		Injection:   "no injection here: no hooks are documented",
		Source:      "https://docs.warp.dev/agents/cli/configuration/ (2026-09-01)",
	},
	{
		ID: "continue", Display: "Continue", Tier: "D",
		Instruction: "root AGENTS.md loaded as a rule; .continue/rules/*.md; config.yaml rules",
		Skills:      "not documented (prompts block = slash prompts)",
		MCPConfig:   "config.yaml mcpServers[{name, type: streamable-http, url}] or .continue/mcpServers/*.yaml",
		MCPAddLine:  "add to config.yaml: mcpServers: [{name: fathom, type: streamable-http, url: {{url}}/rpc/agent}]",
		Injection:   "no injection here: no hooks are documented",
		Source:      "https://docs.continue.dev/customize/deep-dives/mcp (2026-09-01)",
	},
	{
		ID: "aider", Display: "Aider", Tier: "D",
		Instruction: "CONVENTIONS.md via --read / .aider.conf.yml read:; no AGENTS.md auto-load in the options reference",
		Skills:      "none",
		MCPConfig:   "not in the official options reference on 2026-09-01 (a --mcp-server flag is claimed by third parties, unverified)",
		MCPAddLine:  "no documented MCP registration — pass fathom's `pack_context` output with --read instead",
		Injection:   "no injection here: no hooks (only --lint-cmd/--test-cmd post-edit automation); MCP unverified",
		Source:      "https://aider.chat/docs/config/options.html (2026-09-01)",
	},
}

// LadderRows returns a copy of every tier B/C/D row in fixed order.
func LadderRows() []LadderRow {
	out := make([]LadderRow, len(ladder))
	copy(out, ladder)
	return out
}

// RenderLadderTable renders LadderRows as a Markdown table — the ONE
// source a generated adoption doc's ladder section is pinned to. url fills
// each row's mcp add line ("" leaves the {{url}} template visible).
func RenderLadderTable(url string) string {
	var sb strings.Builder
	sb.WriteString("| Harness | Tier | Instruction file | Skills | MCP | Injection |\n|---|---|---|---|---|---|\n")
	for _, r := range LadderRows() {
		mcp := r.MCPConfig + " — `" + r.MCPAddLine + "`"
		if url != "" {
			mcp = r.MCPConfig + " — `" + r.MCPLine(url) + "`"
		}
		fmt.Fprintf(&sb, "| %s (`%s`) | %s | %s | %s | %s | %s — source: %s |\n",
			r.Display, r.ID, r.Tier, mdCell(r.Instruction), mdCell(r.Skills), mdCell(mcp), mdCell(r.Injection), mdCell(r.Source))
	}
	return sb.String()
}

// mdCell escapes the one byte a table cell cannot carry.
func mdCell(s string) string { return strings.ReplaceAll(s, "|", "\\|") }
