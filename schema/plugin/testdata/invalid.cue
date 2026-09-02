// invalid.cue — negative fixtures for the plugin manifest schemas,
// documented commented-out so live `cue vet` stays green (repo convention).
// Each case is asserted to actually fail by
// cmd/fathom/pluginschema_test.go (TestPluginSchema_Negatives).
package plugin

// 1. the wrong $schema version:
// bad1: #AgentPlugin & {"$schema": "https://agent-plugins.org/schemas/1.1.0/plugin.schema.json", name: "x"}

// 2. an uppercase / double-hyphen / double-dot name:
// bad2: #AgentPlugin & {"$schema": #AgentPluginSchemaURL, name: "My-Plugin"}
// bad3: #AgentPlugin & {"$schema": #AgentPluginSchemaURL, name: "has--double"}
// bad4: #AgentPlugin & {"$schema": #AgentPluginSchemaURL, name: "too.many..dots"}

// 5. inline component config (closed schema):
// bad5: #AgentPlugin & {"$schema": #AgentPluginSchemaURL, name: "x", mcpServers: {}}

// 6. an MCP server mixing variants / a non-loopback http URL:
// bad6: #AgentMCP & {"$schema": #AgentMCPSchemaURL, mcpServers: {x: {type: "stdio", command: "x", url: "https://a"}}}
// bad7: #AgentMCP & {"$schema": #AgentMCPSchemaURL, mcpServers: {x: {type: "streamable-http", url: "http://example.com/mcp"}}}

// 8. a Claude component path escaping the plugin root:
// bad8: #ClaudeManifest & {name: "x", skills: "../elsewhere"}
