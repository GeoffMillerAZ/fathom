# Sextant Usage

How to ingest repos, query symbols, run the MCP server, and embed sextant in Go code.

See [architecture.md](architecture.md) for design context and [mcp-tools.md](mcp-tools.md)
for the full MCP tool reference.

---

## Prerequisites

| Requirement | Notes |
|---|---|
| Go 1.26+ | CGO must be enabled (DuckDB) |
| [CUE](https://cuelang.org) 0.16+ | Schema validation |
| [go-task](https://taskfile.dev) | `Taskfile.yml` is the only command runner |
| [lefthook](https://github.com/evilmartians/lefthook) | Git hooks (pre-commit gofmt, pre-push validate) |
| `../agentic-kit` | Sibling module, wired via `go.work` + `replace` |

Build and gate commands:

```sh
task            # list available tasks
task doctor     # toolchain + workspace health check
task validate   # full gate: fmt-check + vet + cue-vet + go test -race (CGO/DuckDB)
lefthook install
```

---

## workspace/ convention

All build artifacts, logs, temp files, and local databases go under `workspace/` at the
repo root. This directory is gitignored. Never write generated files elsewhere in the tree.

| Path | Contents |
|---|---|
| `workspace/db/` | DuckDB stores (default: `workspace/db/sextant.duckdb`) |
| `workspace/logs/` | Run and MCP call logs (`mcp-calls.json`) |
| `workspace/artifacts/` | Generated artifacts (`CLAUDE.md`, manifests, exports) |
| `workspace/tmp/` | Ephemeral scratch files |

`store.Open` auto-creates the parent directory when it does not exist.

---

## Quickstart

```sh
# 1. Ingest the current directory as a service named "my-service"
sextant ingest --service my-service --path .

# 2. List stored symbols
sextant inspect --service my-service

# 3. Query symbols relevant to the "auth" concern
sextant query --service my-service --concern auth --keywords token,validate

# 4. Start a stdio MCP server for your editor/agent
sextant serve-mcp --store workspace/db/sextant.duckdb
```

---

## CLI subcommands

All subcommands share the convention:

- `--store` defaults to `workspace/db/sextant.duckdb`
- `--mode` selects the connector: `local` (default), `faux` (fixture data), or `github`
- Exit 0 on success, 1 on any error

### `ingest`

Ingest a service into the knowledge store.

```sh
sextant ingest --service <id> [--path <dir>] [--store <path>] [--mode local|faux]
```

| Flag | Default | Description |
|---|---|---|
| `--service` | (required) | Logical service ID, e.g. `payment-service` |
| `--path` | `.` | Source root directory (local mode) |
| `--store` | `workspace/db/sextant.duckdb` | DuckDB store path |
| `--mode` | `local` | Connector mode: `local` or `faux` |

Output: `discovered`, `extracted`, `skipped`, `symbols` counts.

The pipeline is hash-gated: unchanged files (same SHA-256 as the stored fingerprint) are
skipped. Re-ingesting a large repo where few files changed is near-free.

```sh
sextant ingest --service payment-service --path /src/payment
# service:    payment-service
# discovered: 142
# extracted:  38
# skipped:    104
# symbols:    891
```

---

### `ingest-diff`

Ingest only files changed since a git ref. When `--base` is omitted, falls back to a full
ingest.

```sh
sextant ingest-diff --service <id> [--path <dir>] [--base <git-ref>] [--store <path>] [--mode local|faux]
```

| Flag | Default | Description |
|---|---|---|
| `--service` | (required) | Logical service ID |
| `--path` | `.` | Source root directory |
| `--base` | `""` | Git ref to diff against (e.g. `HEAD~1`, `main`). Empty = full ingest |
| `--store` | `workspace/db/sextant.duckdb` | DuckDB store path |
| `--mode` | `local` | Connector mode |

```sh
sextant ingest-diff --service payment-service --path /src/payment --base main
# service:    payment-service
# base:       main
# changed:    7 files
# discovered: 142
# ...
```

---

### `watch`

Poll for file changes and re-ingest automatically.

```sh
sextant watch --service <id> --path <dir> [--interval <duration>] [--store <path>] [--mode local|faux]
```

| Flag | Default | Description |
|---|---|---|
| `--service` | (required) | Logical service ID |
| `--path` | (required) | Source root to watch |
| `--interval` | `5s` | Polling interval, e.g. `5s`, `1m` |
| `--store` | `workspace/db/sextant.duckdb` | DuckDB store path |
| `--mode` | `local` | Connector mode |

`watch` uses mtime comparison. On any change it triggers a full `Ingest` for the service
and prints a summary line. Runs until interrupted.

```sh
sextant watch --service my-service --path . --interval 10s
# [watch] watching my-service at . (interval=10s)
# [watch] my-service: re-ingested (discovered=142 extracted=3 symbols=891)
```

---

### `inspect`

List stored symbols for a service.

```sh
sextant inspect --service <id> [--kind <SourceKind>] [--store <path>]
```

| Flag | Default | Description |
|---|---|---|
| `--service` | (required) | Service ID to inspect |
| `--kind` | `""` | Filter by SourceKind (e.g. `GoFunc`, `GoType`, `K8sWorkload`) |
| `--store` | `workspace/db/sextant.duckdb` | DuckDB store path |

```sh
sextant inspect --service my-service --kind GoFunc
# service: my-service
# symbols: 47
#
#   GoFunc      ValidateToken                            internal/auth/token.go
#   GoFunc      RefreshToken                             internal/auth/token.go
#   ...
```

---

### `query`

Query symbols for a concern. ConcernID is mandatory (ADR-006).

```sh
sextant query --service <id> --concern <concern-id> [--keywords <kw,...>] [--top <n>] [--store <path>]
```

| Flag | Default | Description |
|---|---|---|
| `--service` | (required) | Service ID |
| `--concern` | (required) | Concern ID to scope the query, e.g. `authentication`, `billing` |
| `--keywords` | `""` | Comma-separated keywords for relevance scoring |
| `--top` | `20` | Maximum results to return |
| `--store` | `workspace/db/sextant.duckdb` | DuckDB store path |

When the concern-scoped query returns no results, the command falls back to a keyword
substring search over all symbols for the service.

```sh
sextant query --service payment-service --concern auth --keywords token,validate --top 5
#   1  ValidateToken                             GoFunc  internal/auth/token.go
#      func ValidateToken(ctx context.Context, tok string) error
#   2  RefreshToken                              GoFunc  internal/auth/token.go
#   ...
```

If staleness is detected, a warning is printed at the end of the output:
`warning: staleness detected — re-ingest recommended`

---

### `serve-mcp`

Serve the MCP JSON-RPC 2.0 protocol over stdio. This is the mode used by editor and agent
clients (e.g. Claude Code). No network port is opened.

```sh
sextant serve-mcp --store <path> [--mode local|faux]
```

| Flag | Default | Description |
|---|---|---|
| `--store` | (required) | DuckDB store path |
| `--mode` | `local` | Connector mode |

The process reads JSON-RPC 2.0 messages from stdin and writes responses to stdout. See
[Connecting an MCP client](#connecting-an-mcp-client) below and
[mcp-tools.md](mcp-tools.md) for the full tool list.

---

### `serve-http`

Serve the MCP JSON-RPC protocol over HTTP. Suited for multi-client, platform, and CI use.

```sh
sextant serve-http --store <path> [--addr <host:port>]
```

| Flag | Default | Description |
|---|---|---|
| `--store` | (required) | DuckDB store path |
| `--addr` | `:7575` | Listen address |

HTTP endpoints:

| Method | Path | Description |
|---|---|---|
| `POST` | `/rpc` | Single JSON-RPC 2.0 request/response (4 MB body limit) |
| `GET` | `/healthz` | Liveness probe — returns `ok` |
| `GET` | `/events` | SSE stream — sends `hello` event then stays open |
| `GET` | `/metrics` | Text metrics: `sextant_uptime_seconds`, `sextant_requests_total` |
| `POST` | `/reload` | Reload the store (PlatformStore re-scan in platform mode) |

```sh
sextant serve-http --store workspace/db/sextant.duckdb --addr :7575
# serve-http: listening on :7575

# In another terminal:
curl -s http://localhost:7575/healthz
# ok

curl -s -X POST http://localhost:7575/rpc \
  -H 'Content-Type: application/json' \
  -d '{"jsonrpc":"2.0","id":1,"method":"tools/list","params":{}}'
```

---

### `platform-ingest`

Ingest all services under a root directory. Each immediate subdirectory is treated as one
service; each gets its own `<service-id>.duckdb` file in the output store directory.

```sh
sextant platform-ingest --root <dir> --store-dir <dir>
```

| Flag | Default | Description |
|---|---|---|
| `--root` | (required) | Directory containing service subdirectories |
| `--store-dir` | (required) | Output directory for per-service `.duckdb` files |

```sh
sextant platform-ingest --root /src/services --store-dir workspace/db/platform
# service:    auth-service
#   discovered: 89
#   extracted:  89
#   symbols:    421
# service:    billing-service
#   ...
# total: 4 services, 421 discovered, 1823 symbols
```

---

### `platform-callers`

Print cross-repo callers of a symbol. Builds the platform graph live from Go source on
disk without a store.

```sh
sextant platform-callers --repos <id=path,...> --symbol <name>
```

| Flag | Default | Description |
|---|---|---|
| `--repos` | (required) | Comma-separated `id=path` pairs, e.g. `svc-a=/src/a,svc-b=/src/b` |
| `--symbol` | (required) | Bare exported symbol name to look up |

```sh
sextant platform-callers \
    --repos payment-service=/src/payment,order-service=/src/order \
    --symbol ProcessPayment
# cross-repo callers of "ProcessPayment":
#
# FROM-SERVICE             FROM-FILE                                TO-SERVICE               TO-FILE
# ──────────────────────────────────────────────────────────────────────────────────────────────────
# order-service            internal/checkout/handler.go             payment-service          payment.go
#
# total: 1 caller(s)
```

---

### `gen-claude-md`

Generate a `CLAUDE.md` from stored symbols. The output is a Markdown file suitable for
dropping into an AI coding assistant's context.

```sh
sextant gen-claude-md --service <id> [--store <path>] [--output <path>]
```

| Flag | Default | Description |
|---|---|---|
| `--service` | (required) | Service ID |
| `--store` | `workspace/db/sextant.duckdb` | DuckDB store path |
| `--output` | `workspace/artifacts/CLAUDE.md` | Output file path |

Output is deterministic: symbols are sorted by kind then name. The file includes a
kind/count breakdown table and a full symbol listing.

```sh
sextant gen-claude-md --service my-service
# wrote workspace/artifacts/CLAUDE.md (891 symbols)
```

---

### `gen-platform-manifest`

Generate a platform manifest YAML from `*.duckdb` store files. The manifest lists service
IDs and their store paths, suitable for feeding to `serve-http` platform mode.

```sh
sextant gen-platform-manifest --store-dir <dir> [--output <path>]
```

| Flag | Default | Description |
|---|---|---|
| `--store-dir` | (required) | Directory containing `*.duckdb` files |
| `--output` | `workspace/artifacts/platform-manifest.yaml` | Output YAML path |

```sh
sextant gen-platform-manifest --store-dir workspace/db/platform
# wrote workspace/artifacts/platform-manifest.yaml (4 services)
```

---

### `version`

Print version information.

```sh
sextant version
```

---

## Go client (`pkg/client`)

`pkg/client` exposes a thin `Client` that wires together the connector, fusion pipeline,
query engine, and store. Import it without touching any `internal/` package.

```go
import "github.com/geoffmilleraz/sextant/pkg/client"

// Open (or create) a DuckDB store; wire a local-mode connector.
c, err := client.NewConstantClient(client.ModeLocal, "workspace/db/sextant.duckdb")
if err != nil { /* handle */ }
defer c.Close()

// Ingest a service.
report, err := c.Ingest(ctx, client.ServiceRef{ID: "my-service", Path: "."})
fmt.Println(report.Symbols) // number of symbols extracted

// Concern-scoped symbol query.
result, err := c.QuerySymbols(ctx, client.Query{
    ServiceID: "my-service",
    ConcernID: "authentication",
    Keywords:  []string{"token", "validate"},
    TopN:      20,
})
for _, n := range result.Nodes {
    fmt.Printf("%s %s %s\n", n.Kind, n.Name, n.FilePath)
}
if result.Staleness.ReingestRecommended {
    // re-ingest before trusting this data
}

// Find callers of a symbol.
callers, err := c.FindCallers(ctx, "my-service", "ValidateToken")

// Find a symbol definition.
nodes, err := c.FindSymbol(ctx, "my-service", "ValidateToken")

// List all stored symbols (no filtering).
all, err := c.ListAllSymbols(ctx, "my-service")

// List all ingested service IDs.
services, err := c.Services(ctx)

// Batch ingest.
reports, err := c.IngestBatch(ctx, []client.ServiceRef{
    {ID: "auth", Path: "/src/auth"},
    {ID: "billing", Path: "/src/billing"},
})
```

Available modes:

| Constant | Connector | Use |
|---|---|---|
| `client.ModeLocal` | `LocalConnector` | Reads from local directory path |
| `client.ModeFaux` | `FauxConnector` | Reads from `testdata/fauxtown/` fixture |
| `client.ModeGitHub` | `GitHubConnector` | Reads via `gh` CLI (requires `gh`) |

Pass `""` for `storePath` to use an in-memory DuckDB database (useful in tests).

---

## Connecting an MCP client

### Claude Code (stdio)

Add sextant to your Claude Code MCP configuration:

```json
{
  "mcpServers": {
    "sextant": {
      "command": "sextant",
      "args": ["serve-mcp", "--store", "/path/to/workspace/db/sextant.duckdb"]
    }
  }
}
```

Claude Code will launch the `sextant serve-mcp` process and communicate over stdio.

### Any MCP-compatible client (stdio)

Pipe JSON-RPC 2.0 messages to the `serve-mcp` process:

```sh
# Initialize
echo '{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"my-client","version":"1.0"}}}' \
  | sextant serve-mcp --store workspace/db/sextant.duckdb

# tools/list
echo '{"jsonrpc":"2.0","id":2,"method":"tools/list","params":{}}' \
  | sextant serve-mcp --store workspace/db/sextant.duckdb
```

### HTTP client

```sh
# Single JSON-RPC call via POST /rpc
curl -s -X POST http://localhost:7575/rpc \
  -H 'Content-Type: application/json' \
  -d '{
    "jsonrpc": "2.0",
    "id": 1,
    "method": "tools/call",
    "params": {
      "name": "query_symbols",
      "arguments": {
        "service": "my-service",
        "concern": "authentication",
        "keywords": ["token"],
        "top": 10
      }
    }
  }'
```

### Switching profiles

Once connected, call `set_profile` to filter the visible tool surface:

```json
{
  "jsonrpc": "2.0",
  "id": 3,
  "method": "tools/call",
  "params": {
    "name": "set_profile",
    "arguments": { "profile": "coding" }
  }
}
```

Valid profiles: `coding`, `investigation`, `shared`, `platform`.

See [mcp-tools.md](mcp-tools.md) for all tools, prompts, and resources.
