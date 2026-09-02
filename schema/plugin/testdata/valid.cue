// valid.cue — positive fixtures for the plugin manifest schemas (the
// spec's own verbatim examples). Must pass `cue vet ./...` from schema/plugin.
package plugin

minimalPlugin: #AgentPlugin & {
	"$schema": "https://agent-plugins.org/schemas/1.0.0/plugin.schema.json"
	name:      "minimal-plugin"
}

fullPlugin: #AgentPlugin & {
	"$schema":   "https://agent-plugins.org/schemas/1.0.0/plugin.schema.json"
	name:        "plugin-name"
	version:     "1.2.0"
	description: "Brief plugin description"
	author: {name: "Author Name", email: "author@example.com", url: "https://example.com"}
	homepage:   "https://docs.example.com/plugin"
	repository: "https://github.com/example/plugin"
	license:    "MIT"
	keywords: ["keyword1", "keyword2"]
	extensions: {"com.example.client": {setting: true}}
}

mcp: #AgentMCP & {
	"$schema": "https://agent-plugins.org/schemas/1.0.0/mcp.schema.json"
	mcpServers: {
		"local-validator": {
			type:    "stdio"
			command: "./bin/validator"
			args: ["--data", "${PLUGIN_DATA}/validator"]
			env: {CONFIG: "${PLUGIN_ROOT}/config.json"}
			cwd: "${PLUGIN_ROOT}"
		}
		"deployment-api": {
			type: "streamable-http"
			url:  "https://deploy.example.com/mcp"
			headers: {"X-Tenant": "public-tenant"}
		}
		"legacy-events": {type: "sse", url: "https://legacy.example.com/sse"}
		"loopback": {type: "streamable-http", url: "http://localhost:7575/rpc/agent"}
	}
}

claude: #ClaudeManifest & {
	name:        "plugin-name"
	displayName: "Plugin Name"
	version:     "1.2.0"
	description: "Brief plugin description"
	author: {name: "Author Name"}
	keywords: ["keyword1"]
	metadata: {catalogId: "cat-123", tier: "pro"}
	skills:     "./custom/skills/"
	hooks:      "./config/hooks.json"
	mcpServers: "./mcp-config.json"
}
