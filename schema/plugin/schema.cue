// Package plugin vendors the two plugin manifests `fathom plugin render`
// emits over one skills/ tree (docs/plans/reach-parity.md RP-6/RP-7,
// docs/vision/25-harness-port.md §1):
//
//   - #AgentPlugin / #AgentMCP — Agent Plugins Specification 1.0.0
//     (agent-plugins.org, spec/1.0.0.md published 2026-07-24; JSON schemas
//     schemas/1.0.0/{plugin,mcp}.schema.json), the root plugin.json and
//     mcp.json every listed client (VS Code, Cursor, Copilot, Codex, Kiro,
//     …) loads. Both are CLOSED schemas in the spec and closed here.
//   - #ClaudeManifest — Claude Code's .claude-plugin/plugin.json
//     (code.claude.com/docs/en/plugins-reference, fetched 2026-09-02).
//     Only `name` is required; the component-path fields must start with
//     "./" (a "../" escapes the plugin root and the component is dropped).
//
// Standards are adopted as vendored schemas under `task cue-vet`, never
// as hand-written conformance tests; cmd/fathom's TestPluginManifestsVet
// renders the tree and unifies every manifest with these definitions.
package plugin

import "strings"

// ─── Agent Plugins 1.0.0 ────────────────────────────────────────────────

#AgentPluginSchemaURL: "https://agent-plugins.org/schemas/1.0.0/plugin.schema.json"
#AgentMCPSchemaURL:    "https://agent-plugins.org/schemas/1.0.0/mcp.schema.json"

// #PluginName: 1–64 chars; a-z, 0-9, "-", "."; alphanumeric first and last;
// no "--" and no "..". (The spec's lookahead pattern, spelled as CUE
// conjunctions — RE2 has no lookahead.)
#PluginName: string & =~"^[a-z0-9](?:[a-z0-9.-]*[a-z0-9])?$" & !~"--" & !~"\\.\\." & strings.MaxRunes(64)

#Author: {
	name?:  string
	email?: string
	url?:   string
}

// #AgentPlugin is plugin.json §5: closed — inline component config
// (mcpServers, hooks, agents, commands, lspServers) is a schema violation.
#AgentPlugin: {
	"$schema":    #AgentPluginSchemaURL
	name:         #PluginName
	version?:     string
	description?: string
	author?:      #Author
	homepage?:    string
	repository?:  string
	license?:     string
	keywords?: [...string]
	// keys are reverse-domain client namespaces; values MUST be objects.
	extensions?: {[=~"^[a-z0-9]+(\\.[a-z0-9-]+)+$"]: {...}}
}

#StdioServer: {
	type:    "stdio"
	command: string & strings.MinRunes(1)
	args?: [...string]
	// env keys must not be PLUGIN_ROOT / PLUGIN_DATA (the client sets those last).
	env?: {[!~"^(PLUGIN_ROOT|PLUGIN_DATA)$"]: string}
	// plugin-relative, or exactly ${PLUGIN_ROOT}/${PLUGIN_DATA} (with an optional subpath).
	cwd?: string & =~"^(?:\\./|\\$\\{PLUGIN_ROOT\\}(?:/|$)|\\$\\{PLUGIN_DATA\\}(?:/|$))"
}

#HTTPServer: {
	type: "streamable-http" | "sse"
	// absolute http(s); http only for localhost / loopback literals.
	url: string & (=~"^https://[^\\s#@]+$" | =~"^http://(localhost|127\\.[0-9.]+|\\[::1\\])(:[0-9]+)?(/[^\\s#@]*)?$")
	headers?: {[string]: string}
}

// #AgentMCP is mcp.json §7.2: closed, `$schema` version must match plugin.json's.
#AgentMCP: {
	"$schema": #AgentMCPSchemaURL
	mcpServers: {[string]: #StdioServer | #HTTPServer}
}

// ─── Claude Code .claude-plugin/plugin.json ─────────────────────────────

#PluginRelPath: string & =~"^\\./" & !~"\\.\\./"

// #ClaudeManifest is the documented schema; unrecognized top-level fields
// are ignored at load and warned by `claude plugin validate` (errors under
// --strict), so the struct is closed here to keep the rendering warning-free.
#ClaudeManifest: {
	name:           string & =~"^[a-z0-9]([a-z0-9-]*[a-z0-9])?$"
	"$schema"?:     string
	displayName?:   string
	version?:       string
	description?:   string
	author?:        #Author
	homepage?:      string
	repository?:    string
	license?:       string
	keywords?: [...string]
	metadata?: {...}
	defaultEnabled?: bool
	skills?:         #PluginRelPath | [...#PluginRelPath] | "."
	commands?:       #PluginRelPath | [...#PluginRelPath]
	agents?:         #PluginRelPath | [...#PluginRelPath]
	workflows?:      #PluginRelPath | [...#PluginRelPath]
	outputStyles?:   #PluginRelPath | [...#PluginRelPath]
	hooks?:          #PluginRelPath | [...#PluginRelPath] | {...}
	mcpServers?:     #PluginRelPath | [...#PluginRelPath] | {...}
	lspServers?:     #PluginRelPath | [...#PluginRelPath] | {...}
	userConfig?: {...}
	channels?: [...]
	dependencies?: [...(string | {name: string, version?: string})]
	experimental?: {...}
}
