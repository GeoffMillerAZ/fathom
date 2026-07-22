# Fathom Usage

How to ingest repos, query symbols, run the MCP server, and embed fathom in Go code.

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
| `workspace/db/` | store files (default: `workspace/db/fathom.sqlite`) |
| `workspace/logs/` | Run and MCP call logs (`mcp-calls.jsonl`) |
| `workspace/artifacts/` | Generated artifacts (`CLAUDE.md`, manifests, exports) |
| `workspace/tmp/` | Ephemeral scratch files |

`store.Open` auto-creates the parent directory when it does not exist.

---

## Quickstart

```sh
# 1. Ingest the current directory as a service named "my-service"
fathom ingest --service my-service --path .

# 2. List stored symbols
fathom inspect --service my-service

# 3. Query symbols relevant to the "auth" concern
fathom query --service my-service --concern auth --keywords token,validate

# 4. Start a stdio MCP server for your editor/agent
fathom serve-mcp --store workspace/db/fathom.sqlite
```

---

## Ignoring files: `.fathomignore`

`LocalConnector` always skips `vendor`, `.git`, `testdata`, `generated`,
`node_modules`, `.idea`, and `.vscode`. For repo-specific exclusions, place a
`.fathomignore` file at the service root (the `--path` directory). It uses
gitignore-style patterns — one per line, `#` comments, `!` negation, `**`
globs, and trailing-slash dir-only rules:

```gitignore
# Mirrored third-party source — not our code.
ref/
# Scratch and local stores.
tmp/
workspace/
# Generated code: index the sources, not the codegen.
*_templ.go
```

Matched directories are pruned from the walk; matched files are dropped.
Sensitive data directories (user content, case data, secrets) should always be
excluded here — the index is queryable by any connected MCP client and is not
behind an egress/DLP gate.

---

## CLI subcommands

All subcommands share the convention:

- `--store` defaults to `workspace/db/fathom.sqlite`
- `--mode` selects the connector: `local` (default), `faux` (fixture data), or `github`
- Exit 0 on success, 1 on any error

### `ingest`

Ingest a service into the knowledge store.

```sh
fathom ingest --service <id> [--path <dir>] [--store <path>] [--mode local|faux]
```

| Flag | Default | Description |
|---|---|---|
| `--service` | (required) | Logical service ID, e.g. `payment-service` |
| `--path` | `.` | Source root directory (local mode) |
| `--store` | `workspace/db/fathom.sqlite` | store path (.duckdb or .sqlite/.db) |
| `--mode` | `local` | Connector mode: `local` or `faux` |

Output: `discovered`, `extracted`, `skipped`, `symbols` counts.

The pipeline is hash-gated: unchanged files (same SHA-256 as the stored fingerprint) are
skipped. Re-ingesting a large repo where few files changed is near-free.

```sh
fathom ingest --service payment-service --path /src/payment
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
fathom ingest-diff --service <id> [--path <dir>] [--base <git-ref>] [--store <path>] [--mode local|faux]
```

| Flag | Default | Description |
|---|---|---|
| `--service` | (required) | Logical service ID |
| `--path` | `.` | Source root directory |
| `--base` | `""` | Git ref to diff against (e.g. `HEAD~1`, `main`). Empty = full ingest |
| `--store` | `workspace/db/fathom.sqlite` | store path (.duckdb or .sqlite/.db) |
| `--mode` | `local` | Connector mode |

```sh
fathom ingest-diff --service payment-service --path /src/payment --base main
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
fathom watch --service <id> --path <dir> [--interval <duration>] [--store <path>] [--mode local|faux]
```

| Flag | Default | Description |
|---|---|---|
| `--service` | (required) | Logical service ID |
| `--path` | (required) | Source root to watch |
| `--interval` | `5s` | Polling interval, e.g. `5s`, `1m` |
| `--store` | `workspace/db/fathom.sqlite` | store path (.duckdb or .sqlite/.db) |
| `--mode` | `local` | Connector mode |

`watch` uses mtime comparison. On any change it triggers a full `Ingest` for the service
and prints a summary line. Runs until interrupted.

```sh
fathom watch --service my-service --path . --interval 10s
# [watch] watching my-service at . (interval=10s)
# [watch] my-service: re-ingested (discovered=142 extracted=3 symbols=891)
```

---

### `inspect`

List stored symbols for a service.

```sh
fathom inspect --service <id> [--kind <SourceKind>] [--store <path>]
```

| Flag | Default | Description |
|---|---|---|
| `--service` | (required) | Service ID to inspect |
| `--kind` | `""` | Filter by SourceKind (e.g. `GoFunc`, `GoType`, `K8sWorkload`) |
| `--store` | `workspace/db/fathom.sqlite` | store path (.duckdb or .sqlite/.db) |

```sh
fathom inspect --service my-service --kind GoFunc
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
fathom query --service <id> --concern <concern-id> [--keywords <kw,...>] [--top <n>] [--store <path>]
```

| Flag | Default | Description |
|---|---|---|
| `--service` | (required) | Service ID |
| `--concern` | (required) | Concern ID to scope the query, e.g. `authentication`, `billing` |
| `--keywords` | `""` | Comma-separated keywords for relevance scoring |
| `--top` | `20` | Maximum results to return |
| `--store` | `workspace/db/fathom.sqlite` | store path (.duckdb or .sqlite/.db) |

When the concern-scoped query returns no results, the command falls back to a keyword
substring search over all symbols for the service.

```sh
fathom query --service payment-service --concern auth --keywords token,validate --top 5
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
fathom serve-mcp --store <path> [--mode local|faux] [--auto-refresh]
```

| Flag | Default | Description |
|---|---|---|
| `--store` | (required) | store path (.duckdb or .sqlite/.db) |
| `--mode` | `local` | Connector mode |
| `--auto-refresh` | `true` | Re-ingest a service when its git HEAD has drifted from the indexed commit, before answering the query (hash-gated, debounced per commit). Removes the manual staleness→ingest loop. Pass `--auto-refresh=false` to disable. |

A second `serve-mcp` on the same store refuses with actionable guidance (DuckDB is
single-writer). The process reads JSON-RPC 2.0 messages from stdin and writes responses to stdout. See
[Connecting an MCP client](#connecting-an-mcp-client) below and
[mcp-tools.md](mcp-tools.md) for the full tool list.

---

### `serve-http`

Serve the MCP JSON-RPC protocol over HTTP. Suited for multi-client, platform, and CI use.

```sh
fathom serve-http --store <path> [--addr <host:port>]
```

| Flag | Default | Description |
|---|---|---|
| `--store` | (required) | store path (.duckdb or .sqlite/.db) |
| `--addr` | `:7575` | Listen address |
| `--auto-refresh` | `true` | Re-ingest a service on git-HEAD drift before answering |
| `--call-log` | `workspace/logs/mcp-calls.jsonl` | JSONL tool-call log, resolved against the daemon working directory; empty disables |

HTTP endpoints:

| Method | Path | Description |
|---|---|---|
| `POST` | `/rpc` | Single JSON-RPC 2.0 request/response (4 MB body limit) — full tool surface |
| `POST` | `/rpc/{surface}` | Same protocol scoped to a named surface (`companion`, `coding`, `investigation`, `shared`, `platform`): filters `tools/list` **and** hard-gates `tools/call` |
| `GET` | `/healthz` | Liveness probe — returns `ok` |
| `GET` | `/events` | SSE stream — sends `hello` event then stays open |
| `GET` | `/metrics` | Text metrics: `fathom_uptime_seconds`, `fathom_requests_total` |
| `POST` | `/reload` | Reload the store (PlatformStore re-scan in platform mode) |

```sh
fathom serve-http --store workspace/db/fathom.sqlite --addr :7575
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
service; each gets its own `<service-id>.sqlite` file in the output store directory.

```sh
fathom platform-ingest --root <dir> --store-dir <dir>
```

| Flag | Default | Description |
|---|---|---|
| `--root` | (required) | Directory containing service subdirectories |
| `--store-dir` | (required) | Output directory for per-service `.sqlite` files |

```sh
fathom platform-ingest --root /src/services --store-dir workspace/db/platform
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

Print cross-repo callers of a symbol. By default it reads the symbols and call edges
already ingested into the store, so callers are found for **every language** whose
extractor populated them — not just Go.

```sh
fathom platform-callers --store <path> --symbol <name> [--services a,b,c]
```

| Flag | Default | Description |
|---|---|---|
| `--symbol` | (required) | Bare symbol name to find cross-repo callers of |
| `--store` | `workspace/db/fathom.sqlite` | store path, .duckdb or .sqlite/.db (language-agnostic source) |
| `--services` | (all in store) | Comma-separated service IDs to include |
| `--repos` | — | Legacy Go-only mode: comma-separated `id=path` pairs; walks Go source on disk and overrides `--store` when set |

```sh
fathom platform-callers --store workspace/db/fleet.sqlite --symbol ProcessPayment
# cross-repo callers of "ProcessPayment" (store workspace/db/fleet.sqlite (all languages)):
#
# FROM-SERVICE             FROM-FILE                                TO-SERVICE               TO-FILE
# ──────────────────────────────────────────────────────────────────────────────────────────────────
# order-service            internal/checkout/handler.go             payment-service          payment.go
#
# total: 1 caller(s)
```

Cross-repo edges require the index to carry call edges; re-ingest with a current build so
`Package` and `Calls` are populated.

---

### `gen-claude-md`

Generate a `CLAUDE.md` from stored symbols. The output is a Markdown file suitable for
dropping into an AI coding assistant's context.

```sh
fathom gen-claude-md --service <id> [--store <path>] [--output <path>]
```

| Flag | Default | Description |
|---|---|---|
| `--service` | (required) | Service ID |
| `--store` | `workspace/db/fathom.sqlite` | store path (.duckdb or .sqlite/.db) |
| `--output` | `workspace/artifacts/CLAUDE.md` | Output file path |

Output is deterministic: symbols are sorted by kind then name. The file includes a
kind/count breakdown table and a full symbol listing.

```sh
fathom gen-claude-md --service my-service
# wrote workspace/artifacts/CLAUDE.md (891 symbols)
```

---

### `gen-status-cue`

Render an evidence-backed plan-verification status to a deterministic CUE file. Loads a
plan (`.cue` matching `schema/plan/schema.cue`, or `.json`), verifies each task against the
index (the same checks as the `verify_plan` MCP tool), and writes a `STATUS.cue`.

The plan file is the human-owned source of truth; `STATUS.cue` is regenerated, never
hand-edited — there is no bidirectional sync and no plan state stored in the database.

```sh
fathom gen-status-cue --plan <file> --service <id> [--store <path>] [--output <path>] [--concern <c>]
```

| Flag | Default | Description |
|---|---|---|
| `--plan` | (required) | Path to a `.cue` or `.json` plan file |
| `--service` | (required) | Service ID to verify against |
| `--store` | `workspace/db/fathom.sqlite` | store path (.duckdb or .sqlite/.db) |
| `--output` | `workspace/artifacts/STATUS.cue` | Output CUE file path |
| `--concern` | — | Optional concern to scope contradiction detection |

```sh
fathom gen-status-cue --plan plan.cue --service my-service
# wrote workspace/artifacts/STATUS.cue: 4 tasks, 2 verified, 1 failing, 1 unknown
```

Output is deterministic and `cue vet`-able. `runtime_pass` is always `unknown` — a static
check never claims a test passes.

---

### `gen-platform-manifest`

Generate a platform manifest YAML from `*.duckdb`/`*.sqlite` store files. The manifest lists service
IDs and their store paths, suitable for feeding to `serve-http` platform mode.

```sh
fathom gen-platform-manifest --store-dir <dir> [--output <path>]
```

| Flag | Default | Description |
|---|---|---|
| `--store-dir` | (required) | Directory containing `*.duckdb`/`*.sqlite` files |
| `--output` | `workspace/artifacts/platform-manifest.yaml` | Output YAML path |

```sh
fathom gen-platform-manifest --store-dir workspace/db/platform
# wrote workspace/artifacts/platform-manifest.yaml (4 services)
```

---

### `migrate-store`

Port an existing DuckDB store's eight non-derivable tables (`semantic_claims`,
`claim_links`, `claim_events`, `compile_runs`, `service_meta`, `commits`,
`knowledge_cards`, `doc_summaries`) into a freshly created SQLite store
(ADR-020). The source is opened **read-only** and left byte-identical; every
ported table is verified by re-reading it from the target and comparing
content hashes (a mismatch fails the migration). Refuses an existing target
that already has data. Only `observations` and `fingerprints` are left
behind — they rebuild from source, and the command prints the exact
re-ingest invocation per migrated service.

```sh
fathom migrate-store --from <path>.duckdb --to <path>.sqlite
```

| Flag | Default | Description |
|---|---|---|
| `--from` | (required) | Source DuckDB store path (must end in `.duckdb`) |
| `--to` | (required) | Target SQLite store path (must end in `.sqlite`; must be absent or empty) |

```sh
fathom migrate-store --from workspace/db/fleet.duckdb --to workspace/db/fleet.sqlite
# migrated workspace/db/fleet.duckdb -> workspace/db/fleet.sqlite (source opened read-only, unmodified)
# table            rows  spot-hash (source==target verified)
# ...
```

The full operator sequence (stop daemon → migrate → repoint → restart →
re-ingest → delete the old `.duckdb`) is the ADR-020 runbook in
[configuration.md](configuration.md).

---

### `version`

Print version information.

```sh
fathom version
```

---

### `session-hook`

Emit Claude Code **SessionStart** hook JSON that primes the session to use fathom's index
before falling back to Grep/Read. It pings the shared serve-http daemon, maps the project
dir (`$CLAUDE_PROJECT_DIR`, else cwd) to a service, and prints a one-line `additionalContext`
— or an honest "daemon down / repo not indexed" signal. Always exits 0; never stalls startup.

```sh
fathom session-hook [--url http://localhost:7575] [--dir <project-dir>]
```

Wire it into `~/.claude/settings.json` (alongside any existing SessionStart hook):

```json
{
  "hooks": {
    "SessionStart": [
      { "matcher": "startup|resume|clear|compact",
        "hooks": [ { "type": "command", "command": "/abs/path/bin/fathom session-hook", "timeout": 10 } ] }
    ]
  }
}
```

The daemon URL can also be set with `FATHOM_HTTP`.

---

## Go client (`pkg/client`)

`pkg/client` exposes a thin `Client` that wires together the connector, fusion pipeline,
query engine, and store. Import it without touching any `internal/` package.

```go
import "github.com/geoffmilleraz/fathom/pkg/client"

// Open (or create) a DuckDB store; wire a local-mode connector.
c, err := client.NewConstantClient(client.ModeLocal, "workspace/db/fathom.sqlite")
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

Add fathom to your Claude Code MCP configuration:

```json
{
  "mcpServers": {
    "fathom": {
      "command": "fathom",
      "args": ["serve-mcp", "--store", "/path/to/workspace/db/fathom.sqlite"]
    }
  }
}
```

Claude Code will launch the `fathom serve-mcp` process and communicate over stdio.

### Any MCP-compatible client (stdio)

Pipe JSON-RPC 2.0 messages to the `serve-mcp` process:

```sh
# Initialize
echo '{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"my-client","version":"1.0"}}}' \
  | fathom serve-mcp --store workspace/db/fathom.sqlite

# tools/list
echo '{"jsonrpc":"2.0","id":2,"method":"tools/list","params":{}}' \
  | fathom serve-mcp --store workspace/db/fathom.sqlite
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

### Selecting a tool surface

The `set_profile` tool was retired (ADR-014 amendment, 2026-07-16): surfaces
superseded profiles. Select a surface by path — `POST /rpc/agent` is the
curated 15-tool coding surface (also served at `/rpc/companion` for
compatibility); bare `POST /rpc` is the full registry. Off-surface calls
execute and are logged `off_surface` rather than rejected; hard gating is
available via the server's strict-surfaces option.

See [mcp-tools.md](mcp-tools.md) for all tools, prompts, and resources, and
[use-cases/coding-agents.md](use-cases/coding-agents.md) for the canonical
coding-agent wiring (hooks, gates, CLAUDE.md/AGENTS.md).
