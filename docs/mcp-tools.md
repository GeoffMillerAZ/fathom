# MCP Tools Reference

Fathom exposes a JSON-RPC 2.0 MCP server (stdio or HTTP). This page documents every
registered tool, prompt, and resource.

See [usage.md](usage.md) for how to connect a client and select a surface.
See [architecture.md](architecture.md) for how the tool groups map to the three-layer design.
Registry size: 54 tools after the 2026-08-23 surface prune plus `memory_search` (55),
plus `localize` (56, wave G, plan VB-26); the surface reckoning (ADR-053) has since
removed tools, `memory_search` among them (SR-13);
`TestMcpToolsDocMatchesRegistry` keeps the tables below in step with the registry.

---

## Where each part lives

This page is the index; the reference is split per group so parallel work adds
files instead of editing one. `TestMcpToolsDocMatchesRegistry` reads this page and every
file below, in this order.

| File | Holds |
|---|---|
| [surfaces](mcp-tools/surfaces.md) | Tool groups, profiles and surfaces: the profile table, the path-scoped HTTP surfaces and the plain HTTP endpoints |
| [protocol](mcp-tools/protocol.md) | The served protocol contract |
| [cost-classes](mcp-tools/cost-classes.md) | The cost-class table (mirrors the registry) |
| [shared](mcp-tools/shared.md) | Shared tools |
| [coding](mcp-tools/coding.md) | Coding tools |
| [investigation](mcp-tools/investigation.md) | Investigation tools |
| [platform](mcp-tools/platform.md) | Platform tools |
| [module-host](mcp-tools/module-host.md) | Module host (no MCP tools) |
| [symbolnode](mcp-tools/symbolnode.md) | The SymbolNode shape |
| [prompts](mcp-tools/prompts.md) | Prompts |
| [resources](mcp-tools/resources.md) | Resources |
