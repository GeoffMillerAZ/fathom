### `serve-mcp`

Serve the MCP JSON-RPC 2.0 protocol over stdio. This is the mode used by editor and agent
clients (e.g. Claude Code). No network port is opened.

```sh
fathom serve-mcp --store <path> [--mode local|faux] [--auto-refresh]
fathom serve-mcp --url <daemon rpc url> [--wait 30s]     # stdio front for a running serve-http
```

| Flag | Default | Description |
|---|---|---|
| `--store` | (required) | store path (.sqlite/.db) |
| `--mode` | `local` | Connector mode |
| `--auto-refresh` | `true` | Re-ingest a service when its git HEAD has drifted from the indexed commit, before answering the query (hash-gated, debounced per commit). Removes the manual staleness→ingest loop. Pass `--auto-refresh=false` to disable. |
| `--url` | (none) | Stdio front for a running `serve-http` daemon: each JSON-RPC line read from stdin is POSTed to this URL (e.g. `http://127.0.0.1:7575/rpc/agent`) and the answer is written back as one line; a notification gets no line. Opens no store, so any number of stdio clients share the daemon's one writer. Exclusive with `--store`. |
| `--wait` | `30s` | With `--url`: how long to wait for the daemon's `/healthz` before the first forward. A request with an id is always answered: when the daemon cannot be reached, with a JSON-RPC error naming its URL. |

If a second `serve-mcp` on the same store hits SQLite's busy/locked conflict (a write
contending with another writer past `busy_timeout`, ADR-020), it refuses with actionable
guidance rather than a bare driver error. The process reads JSON-RPC 2.0 messages from stdin and writes responses to stdout. See
[Connecting an MCP client](../usage.md#connecting-an-mcp-client) and
[mcp-tools.md](../mcp-tools.md) for the full tool list.

---
