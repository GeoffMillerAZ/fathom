package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"

	skillsdist "github.com/geoffmilleraz/fathom/dist/agents/skills"
	"github.com/geoffmilleraz/fathom/internal/buildinfo"
	"github.com/geoffmilleraz/fathom/internal/harness"
)

// fathom plugin render (vision 16 R1, plan IO-25; RP-6 two manifests):
// generate the plugin directory — skills (all ten, namespaced
// /fathom:<name> in Claude Code), the full hook template as Claude plugin
// hooks, and the MCP registration in BOTH manifest dialects — from the
// SAME embedded sources init installs. A build artifact, never
// hand-maintained: a hand-edited plugin is the schema-copy bug class, so
// re-rendering overwrites unconditionally (the output dir belongs to the
// generator). Distribution: commit the rendered tree to a marketplace repo;
// bump pluginVersion to ship an update (the reconcile channel).
//
// Layout (one tree, two loaders):
//
//	<out>/plugin.json                 Agent Plugins 1.0 (root; $schema-selected)
//	<out>/mcp.json                    Agent Plugins 1.0 (streamable-http, loopback)
//	<out>/.claude-plugin/plugin.json  Claude Code
//	<out>/.mcp.json                   Claude Code (type: http)
//	<out>/hooks/hooks.json            Claude Code hook template
//	<out>/skills/<name>/SKILL.md      shared: spec-only frontmatter

// pluginVersion is the plugin's semver — bumped deliberately per template/
// skill change that should propagate to installed fleets; the embedded
// fathom build version rides in metadata for provenance.
const pluginVersion = "0.2.0"

// pluginDescription is the one description both manifests carry.
const pluginDescription = "Governed code intelligence: index-first discovery, obligations at edit time, done-means-proven gates — skills, hooks, and the MCP registration in one install."

// Agent Plugins 1.0 schema URLs (agent-plugins.org, spec 1.0.0 published
// 2026-07-24). plugin.json's `$schema` MUST equal the first (clients select
// the loading rules by it and reject an unsupported version); mcp.json's
// version MUST match plugin.json's. schema/plugin/schema.cue vendors both.
const (
	agentPluginSchemaURL = "https://agent-plugins.org/schemas/1.0.0/plugin.schema.json"
	agentMCPSchemaURL    = "https://agent-plugins.org/schemas/1.0.0/mcp.schema.json"
)

// renderAgentPluginManifest renders the Agent Plugins 1.0 pair (RP-6,
// docs/plans/reach-parity.md): the root `plugin.json` (closed schema —
// no inline component config, no metadata field) and `mcp.json` pointing
// at the loopback daemon as a `streamable-http` server. Both sit beside
// `.claude-plugin/plugin.json` over ONE `skills/` tree, so a directory
// rendered once loads in Claude Code (which reads its own manifest and
// auto-discovers `skills/`) and in every Agent Plugins client (VS Code,
// Cursor, Copilot, Codex, Kiro, …). Deterministic: no build stamp.
func renderAgentPluginManifest(mcpURL string) (pluginJSON, mcpJSON map[string]any) {
	pluginJSON = map[string]any{
		"$schema":     agentPluginSchemaURL,
		"name":        "fathom",
		"version":     pluginVersion,
		"description": pluginDescription,
		"author":      map[string]any{"name": "fathom"},
		"license":     "MIT",
		"keywords":    []string{"code-intelligence", "mcp", "skills", "verify-gate"},
	}
	mcpJSON = map[string]any{
		"$schema": agentMCPSchemaURL,
		"mcpServers": map[string]any{
			// No alwaysLoad here: this is the PORTABLE agent-plugins.org
			// manifest, and schema/plugin/schema.cue's #AgentMCP does not
			// admit the field — it is Claude Code's own, and it rides the
			// Claude plugin's .mcp.json below (BP-22).
			"fathom": map[string]any{"type": "streamable-http", "url": mcpURL},
		},
	}
	return pluginJSON, mcpJSON
}

func cmdPluginRender(args []string, stdout, stderr io.Writer) int {
	fs := newFlagSet("plugin render", stderr)
	out := fs.String("out", "fathom-plugin", "output directory (created/overwritten — it belongs to the generator)")
	mcpURL := fs.String("mcp-url", "http://localhost:7575/rpc/agent", "daemon endpoint the plugin registers")
	marketplace := fs.Bool("marketplace", false, "also write .claude-plugin/marketplace.json beside the plugin dir")
	if err := fs.Parse(args); err != nil {
		return 1
	}

	// .claude-plugin/plugin.json — Claude Code's manifest.
	manifest := map[string]any{
		"name":        "fathom",
		"displayName": "fathom",
		"version":     pluginVersion,
		"description": pluginDescription,
		"author":      map[string]any{"name": "fathom"},
		"metadata":    map[string]any{"fathomVersion": buildinfo.Version()},
	}
	if err := writeJSON(filepath.Join(*out, ".claude-plugin", "plugin.json"), manifest); err != nil {
		fmt.Fprintf(stderr, "error: %v\n", err)
		return 1
	}

	// plugin.json + mcp.json — the Agent Plugins 1.0 pair (RP-6).
	agentPlugin, agentMCP := renderAgentPluginManifest(*mcpURL)
	if err := writeJSON(filepath.Join(*out, "plugin.json"), agentPlugin); err != nil {
		fmt.Fprintf(stderr, "error: %v\n", err)
		return 1
	}
	if err := writeJSON(filepath.Join(*out, "mcp.json"), agentMCP); err != nil {
		fmt.Fprintf(stderr, "error: %v\n", err)
		return 1
	}

	// skills/ — ONE tree, spec-only frontmatter (agentsVariant): the six
	// Agent Skills fields are the shared floor every client and Claude
	// Code load unchanged.
	for _, name := range skillsdist.Names {
		content, err := renderSkill(skillsTargetAgents, name)
		if err != nil {
			fmt.Fprintf(stderr, "error: embedded skill %q: %v\n", name, err)
			return 1
		}
		dest := filepath.Join(*out, "skills", name, "SKILL.md")
		if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
			fmt.Fprintf(stderr, "error: %v\n", err)
			return 1
		}
		if err := os.WriteFile(dest, content, 0o644); err != nil {
			fmt.Fprintf(stderr, "error: %v\n", err)
			return 1
		}
	}

	// hooks/hooks.json — the SAME template init writes (the registry's
	// Claude Code bindings, ADR-029), spelled via the shim-stable `fathom`
	// (never a checkout path).
	hooksObj := map[string]any{}
	specs := hookSpecsFor(harness.ClaudeCode)
	for _, spec := range specs {
		entry := map[string]any{
			"type":    "command",
			"command": "fathom " + spec.sub,
			"timeout": spec.timeout,
		}
		if spec.async {
			entry["async"] = true // RT-169
		}
		group := map[string]any{"hooks": []any{entry}}
		if spec.matcher != "" {
			group["matcher"] = spec.matcher
		}
		arr, _ := hooksObj[spec.event].([]any)
		hooksObj[spec.event] = append(arr, group)
	}
	if err := writeJSON(filepath.Join(*out, "hooks", "hooks.json"), map[string]any{"hooks": hooksObj}); err != nil {
		fmt.Fprintf(stderr, "error: %v\n", err)
		return 1
	}

	// .mcp.json — the daemon registration.
	if err := writeJSON(filepath.Join(*out, ".mcp.json"), map[string]any{
		"mcpServers": map[string]any{
			// alwaysLoad (BP-22, ADR-032 §5): per Claude Code's MCP docs,
			// startup WAITS for this server to connect before the session
			// begins, so fathom's tools are present on turn one instead of
			// arriving after the agent has already reached for Grep. It is a
			// startup-ordering flag, NOT a deferral control — measured
			// 2026-09-12 on claude 2.1.270, a session pays zero ToolSearch
			// turns with it and without it. Claude Code's own .mcp.json
			// shape, not the portable agent-plugins.org one above, which
			// does not admit the field.
			"fathom": map[string]any{"type": "http", "url": *mcpURL, "alwaysLoad": true},
		},
	}); err != nil {
		fmt.Fprintf(stderr, "error: %v\n", err)
		return 1
	}

	if *marketplace {
		mp := map[string]any{
			"name":  "fathom-plugins",
			"owner": map[string]any{"name": "fathom"},
			"plugins": []any{map[string]any{
				"name":        "fathom",
				"source":      "./" + filepath.Base(*out),
				"description": manifest["description"],
				"version":     pluginVersion,
			}},
		}
		if err := writeJSON(filepath.Join(filepath.Dir(*out), ".claude-plugin", "marketplace.json"), mp); err != nil {
			fmt.Fprintf(stderr, "error: %v\n", err)
			return 1
		}
	}

	fmt.Fprintf(stdout, "plugin rendered: %s (v%s, %d skills, %d hook groups, MCP → %s; .claude-plugin/plugin.json + Agent Plugins 1.0 plugin.json/mcp.json over one skills/ tree) — validate with `claude plugin validate --strict %s`\n",
		*out, pluginVersion, len(skillsdist.Names), len(specs), *mcpURL, *out)
	return 0
}

func writeJSON(path string, v any) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(data, '\n'), 0o644)
}
