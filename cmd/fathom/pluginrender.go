package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	skillsdist "github.com/geoffmilleraz/fathom/dist/agents/skills"
	"github.com/geoffmilleraz/fathom/internal/buildinfo"
	"github.com/geoffmilleraz/fathom/internal/harness"
	"github.com/geoffmilleraz/fathom/internal/mcp"
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
const pluginVersion = "0.3.12"

// pluginDescription is the one description both manifests carry.
const pluginDescription = "Code intelligence for coding agents: routing by question shape, governing rules at the edit, and done-means-proven gates. Skills, hooks and the MCP registration in one install."

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
func renderAgentPluginManifest(mcpURL, defaultSurfaceHeader string) (pluginJSON, mcpJSON map[string]any) {
	pluginJSON = map[string]any{
		"$schema":     agentPluginSchemaURL,
		"name":        "fathom",
		"version":     pluginVersion,
		"description": pluginDescription,
		"author":      map[string]any{"name": "fathom"},
		"license":     "FSL-1.1-ALv2", // ADR-052: fathom is source-available under FSL-1.1-ALv2
		"keywords":    []string{"code-intelligence", "mcp", "skills", "verify-gate"},
	}
	mcpJSON = map[string]any{
		"$schema": agentMCPSchemaURL,
		"mcpServers": map[string]any{
			// No alwaysLoad here: this is the PORTABLE agent-plugins.org
			// manifest, and schema/plugin/schema.cue's #AgentMCP does not
			// admit the field — it is Claude Code's own, and it rides the
			// Claude plugin's .mcp.json below (BP-22).
			//
			// The header documents the default surface (RH-5,
			// docs/design/reach.md §4): #AgentPlugin is a closed schema
			// with no metadata field, so `headers` — the one open field
			// #HTTPServer admits — carries it here instead. Informational
			// only; the daemon does not read it back.
			"fathom": map[string]any{
				"type":    "streamable-http",
				"url":     mcpURL,
				"headers": map[string]any{"X-Fathom-Default-Surface": defaultSurfaceHeader},
			},
		},
	}
	return pluginJSON, mcpJSON
}

// renderPluginSurface returns the default agent surface's tool names
// (sorted) and the named opt-in groups this render stamps into the plugin
// tree (RH-5, docs/design/reach.md §4): "the plugin bundle... render[s]
// the default surface and group settings into both manifests". Read
// straight from the registry (internal/mcp.DefaultSurfaceTools/
// ExtendedSurfaceTools) rather than hand-copied, so an installed plugin
// can never document a surface the daemon does not actually serve.
//
// "extended" is the one opt-in group the registry serves today (POST
// /rpc/extended, and /rpc/companion as its compatible alias) — the design
// doc's named groups (review/explore/memory/system/admin, docs/design/
// reach.md §3) are RH-2's `[surface] groups` config, not yet implemented;
// this documents what the surface reckoning actually shipped (SR-33..35),
// not the not-yet-built config surface.
func renderPluginSurface() (defaultTools []string, groups map[string][]string) {
	defaultTools = mcp.DefaultSurfaceTools()
	groups = map[string][]string{
		"extended": mcp.ExtendedSurfaceTools(),
	}
	return defaultTools, groups
}

// pluginTemplateDigestPin is TestPluginTemplateDigestPinned's golden value
// for pluginVersion "0.3.12" (0.3.12: INTENT.md retired from the activate and custodian skills, the plan skeleton opens with a Request line, ADR-074; release 0.10.0: the onboard, custodian and plan
// skills carry cloud sessions, the import-only CLAUDE.md, ingest cost and
// exit, and plan archival; 0.3.9: the doctor skill names both agents-md plugin ids;
// 0.3.10: the onboard skill builds Linux pure Go, ADR-066; 0.3.11: the
// onboard, doctor and rewire skills are cut to the v0.11.0 install path and
// every skill names only tools a surface serves, the doctor skill keeps both
// agents-md plugin ids and the onboard skill the question-shape routing). Regenerate with
// pluginTemplateDigest() (see that test) and bump pluginVersion in the
// SAME change whenever this pin moves — the digest is what makes a
// template edit visible; the version bump is the deliberate human act it
// demands (docs/design/reach.md §4).
const pluginTemplateDigestPin = "b5eeb717f30e2e30"

// pluginTemplateDigest is a stable sha256 (hex, first 16 chars) over every
// deterministic input `plugin render` turns into the shipped tree: the
// skill sources, the hook spec table, and the default surface/groups
// renderPluginSurface stamps into the manifests. Deliberately EXCLUDES
// pluginVersion and buildinfo.Version() (varying build metadata that would
// make the digest chase its own tail). internal/AGENTS.md's determinism
// contract holds: no clock, no map-iteration order (every input is sorted
// first).
func pluginTemplateDigest() (string, error) {
	h := sha256.New()

	names := append([]string(nil), skillsdist.Names...)
	sort.Strings(names)
	for _, name := range names {
		content, err := renderSkill(skillsTargetAgents, name)
		if err != nil {
			return "", fmt.Errorf("skill %q: %w", name, err)
		}
		fmt.Fprintf(h, "skill:%s:%d:", name, len(content))
		h.Write(content)
	}

	specs := append([]hookSpec(nil), hookSpecsFor(harness.ClaudeCode, hookOptInsAt("")...)...) // RO-38: the user layer's opt-ins
	sort.Slice(specs, func(i, j int) bool {
		if specs[i].event != specs[j].event {
			return specs[i].event < specs[j].event
		}
		if specs[i].matcher != specs[j].matcher {
			return specs[i].matcher < specs[j].matcher
		}
		return specs[i].sub < specs[j].sub
	})
	for _, s := range specs {
		fmt.Fprintf(h, "hook:%s:%s:%s:%d:%v:", s.event, s.matcher, s.sub, s.timeout, s.async)
	}

	defaultTools, groups := renderPluginSurface()
	fmt.Fprintf(h, "default:%v:", defaultTools)
	groupNames := make([]string, 0, len(groups))
	for name := range groups {
		groupNames = append(groupNames, name)
	}
	sort.Strings(groupNames)
	for _, name := range groupNames {
		fmt.Fprintf(h, "group:%s:%v:", name, groups[name])
	}

	return hex.EncodeToString(h.Sum(nil))[:16], nil
}

// pluginRootTokenPattern matches the Agent Plugins standard's two
// plugin-relative substitution variables.
var pluginRootTokenPattern = regexp.MustCompile(`\$\{PLUGIN_(ROOT|DATA)\}`)

// noPluginRootTokens walks v (as decoded JSON: map[string]any,
// []any, string, ...) and errors if any string value depends on
// ${PLUGIN_ROOT} or ${PLUGIN_DATA} — the two Agent Plugins substitution
// variables Cursor is documented NOT to expand (cursor.com/docs/
// reference/plugins, fetched 2026-09-28), unlike every other Agent
// Plugins client. fathom's render never needs them (a loopback URL, no
// plugin-relative command), so this is a guard against a future render
// regressing into relying on them for the Cursor manifest specifically.
func noPluginRootTokens(v any) error {
	switch t := v.(type) {
	case string:
		if pluginRootTokenPattern.MatchString(t) {
			return fmt.Errorf("value %q depends on a plugin-relative token Cursor does not expand", t)
		}
	case map[string]any:
		keys := make([]string, 0, len(t))
		for k := range t {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			if err := noPluginRootTokens(t[k]); err != nil {
				return err
			}
		}
	case []any:
		for _, e := range t {
			if err := noPluginRootTokens(e); err != nil {
				return err
			}
		}
	}
	return nil
}

// renderCursorPlugin returns Cursor's own plugin manifest (RH-8,
// docs/design/reach.md §4: "a --client cursor render where Cursor's
// plugin shape differs"): .cursor-plugin/plugin.json, whose `mcpServers`
// field is a path STRING pointing at a sibling mcp.json rather than an
// inline server map — the one documented structural difference from the
// portable Agent Plugins pair `fathom plugin render` already writes
// (cursor.com/docs/reference/plugins, fetched 2026-09-28). It reuses that
// already-rendered mcp.json (mcpJSON, for the ${PLUGIN_ROOT}/${PLUGIN_DATA}
// guard only — see noPluginRootTokens) instead of duplicating its content.
func renderCursorPlugin(mcpJSON map[string]any) (map[string]any, error) {
	if err := noPluginRootTokens(mcpJSON); err != nil {
		return nil, fmt.Errorf("cursor manifest: %w", err)
	}
	return map[string]any{
		"$schema":     agentPluginSchemaURL,
		"name":        "fathom",
		"version":     pluginVersion,
		"description": pluginDescription,
		"mcpServers":  "./mcp.json",
	}, nil
}

func cmdPluginRender(args []string, stdout, stderr io.Writer) int {
	fs := newFlagSet("plugin render", stderr)
	out := fs.String("out", "fathom-plugin", "output directory (created/overwritten — it belongs to the generator)")
	mcpURL := fs.String("mcp-url", "http://localhost:7575/rpc/agent", "daemon endpoint the plugin registers")
	marketplace := fs.Bool("marketplace", false, "also write .claude-plugin/marketplace.json beside the plugin dir")
	client := fs.String("client", "", `additional client-specific manifest to render alongside the portable pair ("cursor")`)
	if err := fs.Parse(args); err != nil {
		return 1
	}
	if *client != "" && *client != "cursor" {
		fmt.Fprintf(stderr, "error: --client %q is not supported (only \"cursor\")\n", *client)
		return 1
	}

	// RH-5 (docs/design/reach.md §4): the default surface and its opt-in
	// groups, read from the live registry — stamped into every manifest
	// below rather than hand-copied.
	defaultTools, groups := renderPluginSurface()
	defaultSurfaceHeader := strings.Join(defaultTools, ",")

	// .claude-plugin/plugin.json — Claude Code's manifest.
	manifest := map[string]any{
		"name":        "fathom",
		"displayName": "fathom",
		"version":     pluginVersion,
		"description": pluginDescription,
		"author":      map[string]any{"name": "fathom"},
		"metadata": map[string]any{
			"fathomVersion":  buildinfo.Version(),
			"defaultSurface": defaultTools,
			"surfaceGroups":  groups,
		},
	}
	if err := writeJSON(filepath.Join(*out, ".claude-plugin", "plugin.json"), manifest); err != nil {
		fmt.Fprintf(stderr, "error: %v\n", err)
		return 1
	}

	// plugin.json + mcp.json — the Agent Plugins 1.0 pair (RP-6).
	agentPlugin, agentMCP := renderAgentPluginManifest(*mcpURL, defaultSurfaceHeader)
	if err := writeJSON(filepath.Join(*out, "plugin.json"), agentPlugin); err != nil {
		fmt.Fprintf(stderr, "error: %v\n", err)
		return 1
	}
	if err := writeJSON(filepath.Join(*out, "mcp.json"), agentMCP); err != nil {
		fmt.Fprintf(stderr, "error: %v\n", err)
		return 1
	}

	// .cursor-plugin/plugin.json — Cursor's own manifest (RH-8), opt-in via
	// --client cursor: its shape differs from both dialects already
	// rendered above (renderCursorPlugin).
	if *client == "cursor" {
		cursorManifest, err := renderCursorPlugin(agentMCP)
		if err != nil {
			fmt.Fprintf(stderr, "error: %v\n", err)
			return 1
		}
		if err := writeJSON(filepath.Join(*out, ".cursor-plugin", "plugin.json"), cursorManifest); err != nil {
			fmt.Fprintf(stderr, "error: %v\n", err)
			return 1
		}
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
	specs := hookSpecsFor(harness.ClaudeCode, hookOptInsAt("")...) // RO-38: a plugin is user-global, so the user layer's opt-ins decide
	for _, spec := range specs {
		entry := map[string]any{
			"type":    "command",
			"command": hookCommand("fathom", spec.sub),
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
			"fathom": map[string]any{
				"type":       "http",
				"url":        *mcpURL,
				"alwaysLoad": true,
				// RH-5: the same default-surface header the portable
				// mcp.json carries above (renderAgentPluginManifest).
				"headers": map[string]any{"X-Fathom-Default-Surface": defaultSurfaceHeader},
			},
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

	clientNote := ""
	if *client == "cursor" {
		clientNote = " + .cursor-plugin/plugin.json"
	}
	fmt.Fprintf(stdout, "plugin rendered: %s (v%s, %d skills, %d hook groups, MCP → %s; .claude-plugin/plugin.json + Agent Plugins 1.0 plugin.json/mcp.json%s over one skills/ tree) — validate with `claude plugin validate --strict %s`\n",
		*out, pluginVersion, len(skillsdist.Names), len(specs), *mcpURL, clientNote, *out)
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

// runPlugin dispatches `fathom plugin render`.
func runPlugin(args []string, stdout, stderr io.Writer) int {
	if len(args) > 0 && args[0] == "render" {
		return cmdPluginRender(args[1:], stdout, stderr)
	}
	fmt.Fprintln(stderr, "usage: fathom plugin render [--out DIR] [--mcp-url U] [--marketplace]")
	return 1
}

// pluginRunner runs `fathom plugin` (registered below; EW-45).
type pluginRunner struct{}

func (pluginRunner) execVerb(args []string, _ io.Reader, stdout, stderr io.Writer) int {
	return runPlugin(args, stdout, stderr)
}

func init() {
	declareVerb(verb{Name: "plugin", Group: "govern", Order: 120, Run: pluginRunner{},
		Summary: "plugin render [--out DIR] [--mcp-url U] [--marketplace]: render one plugin tree carrying both the Claude Code and the Agent Plugins 1.0 manifests over one skills/ dir"})
}
