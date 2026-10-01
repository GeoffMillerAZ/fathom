### `serve-mcp`

Serve the MCP JSON-RPC 2.0 protocol over stdio. This is the mode used by editor and agent
clients (e.g. Claude Code). No network port is opened.

```sh
fathom serve-mcp --store <path> [--mode local|faux] [--auto-refresh]
```

| Flag | Default | Description |
|---|---|---|
| `--store` | (required) | store path (.sqlite/.db) |
| `--mode` | `local` | Connector mode |
| `--auto-refresh` | `true` | Re-ingest a service when its git HEAD has drifted from the indexed commit, before answering the query (hash-gated, debounced per commit). Removes the manual staleness→ingest loop. Pass `--auto-refresh=false` to disable. |

If a second `serve-mcp` on the same store hits SQLite's busy/locked conflict (a write
contending with another writer past `busy_timeout`, ADR-020), it refuses with actionable
guidance rather than a bare driver error. The process reads JSON-RPC 2.0 messages from stdin and writes responses to stdout. See
[Connecting an MCP client](#connecting-an-mcp-client) below and
[mcp-tools.md](../mcp-tools.md) for the full tool list.

---
