# Fathom Docs

Engineer-facing documentation for the fathom code-intelligence + evidence engine.

| Document | What it covers |
|---|---|
| [architecture.md](architecture.md) | Three-layer design, data model, DuckDB store, fusion pipeline, MCP surface, cross-repo graph |
| [usage.md](usage.md) | Quickstart, every CLI subcommand with flags and examples, Go client API, MCP client connection |
| [mcp-tools.md](mcp-tools.md) | Full reference table of all MCP tools grouped by profile, plus prompts and resources |
| [why-duckdb.md](why-duckdb.md) | Why DuckDB replaced SQLite and what the CGO trade-off means in practice |

The authoritative decision log is [ADR.md](../ADR.md).
The implementation roadmap and status live in [tmp/TRACKING.md](../tmp/TRACKING.md) (gitignored).
