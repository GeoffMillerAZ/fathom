# Fathom Usage

How to ingest repos, query symbols, run the MCP server, and embed fathom in Go code.

See [architecture.md](architecture.md) for design context and [mcp-tools.md](mcp-tools.md)
for the full MCP tool reference.

---

## Prerequisites

| Requirement | Notes |
|---|---|
| Go 1.26+ | CGO must be enabled — required by the tree-sitter extractor bindings (ADR-013), not the store (pure Go since SS-8/ADR-025) |
| [CUE](https://cuelang.org) 0.16+ | Schema validation |
| [go-task](https://taskfile.dev) | `Taskfile.yml` is the only command runner |
| [lefthook](https://github.com/evilmartians/lefthook) | Git hooks (pre-commit gofmt, pre-push validate) |
| `../agentic-kit` | Sibling module, wired via `go.work` + `replace` |

Build and gate commands:

```sh
task            # list available tasks
task doctor     # toolchain + workspace health check
task validate   # full gate: fmt-check + vet + cue-vet + go test -race (CGO/tree-sitter)
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
globs, trailing-slash dir-only rules, and leading-slash root anchoring
(`/fathom` matches only the root-level entry, never `cmd/fathom/`):

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

Build artifacts (compiled binaries, generated bundles) belong in `bin/` or
`workspace/` — both already ignored by convention above — never at the repo
root, where they get walked like any other file. As defense in depth,
connectors also detect binary content directly (a NUL byte in the first 8000
bytes, ADR-021 §1): such files are excluded from extraction, from cloc's
line counts (reported instead in a files/bytes-only `binary` bucket), and
from the regex-based scan tools. `.fathomignore` should still name known
build-artifact paths explicitly with a root-anchored entry (e.g. `/fathom`
for a stray `go build -o fathom` at repo root): an ignored path is never
discovered or fetched into memory at all, whereas the content sniff runs
only after the bytes have already been read.

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
| `--store` | `workspace/db/fathom.sqlite` | store path (.sqlite/.db) |
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
| `--store` | `workspace/db/fathom.sqlite` | store path (.sqlite/.db) |
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
| `--store` | `workspace/db/fathom.sqlite` | store path (.sqlite/.db) |
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
| `--store` | `workspace/db/fathom.sqlite` | store path (.sqlite/.db) |

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
| `--store` | `workspace/db/fathom.sqlite` | store path (.sqlite/.db) |

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
| `--store` | (required) | store path (.sqlite/.db) |
| `--mode` | `local` | Connector mode |
| `--auto-refresh` | `true` | Re-ingest a service when its git HEAD has drifted from the indexed commit, before answering the query (hash-gated, debounced per commit). Removes the manual staleness→ingest loop. Pass `--auto-refresh=false` to disable. |

If a second `serve-mcp` on the same store hits SQLite's busy/locked conflict (a write
contending with another writer past `busy_timeout`, ADR-020), it refuses with actionable
guidance rather than a bare driver error. The process reads JSON-RPC 2.0 messages from stdin and writes responses to stdout. See
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
| `--store` | (required) | store path (.sqlite/.db) |
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
| `--store` | `workspace/db/fathom.sqlite` | store path, .sqlite/.db (language-agnostic source) |
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
| `--store` | `workspace/db/fathom.sqlite` | store path (.sqlite/.db) |
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
| `--store` | `workspace/db/fathom.sqlite` | store path (.sqlite/.db) |
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

Generate a platform manifest YAML from `*.sqlite`/`*.db` store files (a stray `*.duckdb`
file is still listed for visibility if it has no `.sqlite` sibling, but opening it will
fail loudly — DuckDB stores are no longer supported, SS-8). The manifest lists service
IDs and their store paths, suitable for feeding to `serve-http` platform mode.

```sh
fathom gen-platform-manifest --store-dir <dir> [--output <path>]
```

| Flag | Default | Description |
|---|---|---|
| `--store-dir` | (required) | Directory containing `*.sqlite`/`*.db` files |
| `--output` | `workspace/artifacts/platform-manifest.yaml` | Output YAML path |

```sh
fathom gen-platform-manifest --store-dir workspace/db/platform
# wrote workspace/artifacts/platform-manifest.yaml (4 services)
```

---

### `migrate-store` (removed, SS-8)

**No longer exists in this or any later build.** It ported an existing
DuckDB store's eight non-derivable tables (`semantic_claims`, `claim_links`,
`claim_events`, `compile_runs`, `service_meta`, `commits`, `knowledge_cards`,
`doc_summaries`) into a freshly created SQLite store (ADR-020). Once the
fleet had migrated and the daemon ran a full day green on SQLite, SS-8
(`docs/plans/store-sqlite.md`; ADR-025) deleted the command along with the
DuckDB driver it depended on — the trigger fired 2026-07-23. The historical
runbook (what it did, in order) is kept as a historical note in
[configuration.md](configuration.md); a genuine need to read an old
`.duckdb` file requires a pre-SS-8 build of fathom.

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

### `edit-hook`

Claude Code **PostToolUse** hook (`Edit`/`Write`/`MultiEdit`). Reads the hook JSON from
stdin, resolves the edited file, calls the `obligations` MCP tool for it, and injects
governing rules (≤150 tokens, ≤6 rules, each with a `src:` citation, evidence tier, and a
`code drifted since compile` flag when applicable) plus budget-permitting spec-reference
lines. Silent on a clean file and silent on any error (daemon down, file not indexed,
layer never compiled) — it never blocks a write and always exits 0.

```sh
fathom edit-hook [--url http://localhost:7575]
```

Wire it under `hooks.PostToolUse` with matcher `Edit|Write|MultiEdit` (see
[adoption/claude-code.md](adoption/claude-code.md) §7.2 for the full JSON snippet). The
`obligations` layer must be compiled for the service at least once
(`fathom compile --service <id> --dry-run=false`) before this hook has anything to inject.

---

### `stop-hook`

Claude Code **Stop** hook (ADR-018 §4). Diffs which `docs/plans/*.md` tasks were newly
claimed `[x]` **during this session** (working tree vs. the file's state at git HEAD) and
re-verifies just those against the index. Silent (exit 0) on `stop_hook_active`, no git,
no plans, or any operational failure. If any newly-claimed task fails verification, it
emits advisory `additionalContext` by default, or `{"decision":"block"}` with `--block`.

```sh
fathom stop-hook [--url http://localhost:7575] [--block]
```

| Flag | Default | Description |
|---|---|---|
| `--url` | `defaultFathomURL()` | fathom `serve-http` base URL |
| `--block` | `false` | Block session end on a disproven claim instead of just surfacing it |

---

### `verify`

The **done-means-proven gate** (ADR-018): re-parses one or more `docs/plans/*.md`
checklists, queries the live index (HTTP daemon by default, or a local `--store` for
CI/no-daemon), and returns per-task verdicts with file:line evidence. Exit `0` — every
claimed task verifies; exit `1` — a claimed `[x]` task failed verification (block the
push); exit `2` — operational error (daemon unreachable, service unindexed, empty index)
— fail-open, never a false "all claims failed."

```sh
fathom verify --service <id> --plan docs/plans/<slug>.md [--plan <more>...] \
  [--store <path> | --url http://localhost:7575] [--strict] [--require-all] \
  [--concern <id>] [--json] [--quiet-pass]
```

| Flag | Default | Description |
|---|---|---|
| `--plan` | (required, repeatable) | Markdown plan file(s) to verify |
| `--service` | (required) | Service ID to verify against |
| `--store` | `""` | store path (direct mode, bypasses the daemon; required for CI) |
| `--url` | `defaultFathomURL()` | fathom `serve-http` base URL (HTTP mode; ignored when `--store` is set) |
| `--strict` | `false` | Treat claimed-but-Unknown tasks as failures too (recommended for CI) |
| `--require-all` | `false` | Wave gate: additionally fail on any open/in-progress task (ADR-018 §2) |
| `--concern` | `""` | Optional concern for contradiction scoping |
| `--json` | `false` | Emit a JSON report + gate result instead of human text |
| `--quiet-pass` | `false` | Suppress output when the gate passes (exit 0) |

See [docs/plans/README.md](plans/README.md) for the plan grammar and
[docs/adoption/spec-driven-development.md](adoption/spec-driven-development.md) for the
full authoring-to-gate loop.

---

### `claim`

Human-judgment triage on a compiled semantic-layer claim, store-direct only (v0):

```sh
fathom claim refute      --store <path> --service <id> --claim <claim-id> --note "..." [--author <name>]
fathom claim corroborate --store <path> --service <id> --claim <claim-id> --note "..." [--author <name>]
```

`refute` appends a `ClaimEvent{Verdict:"refuted"}` **and** flips the claim's status to
refuted — suppression is refutation with a note. `corroborate` appends a
`ClaimEvent{Verdict:"corroborated"}` and never changes status or tier — corroboration is
provenance, not evidence-laundering. `--store`, `--service`, `--claim`, and `--note` are
all required; `--author` is optional.

---

### `remap`

Point a service's recorded corpus root at a new location (ADR-017). Because every anchor
in the store is repo-relative, moving a corpus (or the whole store to another machine)
needs exactly this one metadata update — queries, source fetching, freshness watermarks,
and re-ingest all resolve through the remapped path immediately.

```sh
fathom remap --store <path> --service <id> --path <new-corpus-root>
```

`--store`, `--service`, and `--path` are all required; `--path` must be an existing
directory.

---

### `layers`

List, show, or install semantic-layer packs — the embedded catalog (`obligations`,
`findings`, `adr-rules`, `spec-bindings`) unioned with a project's own `.fathom/layers/`.
Presence of a pack **is** its enable state for compilation; there is deliberately no
enable/disable verb.

```sh
fathom layers list    [--root <dir>] [--store <path> --service <id>]
fathom layers show    <name> [--root <dir>]
fathom layers install <dir>  [--root <dir>] [--force]
```

`list` prints every visible pack (name, source, kind, tier, eval status, drift, corpus,
and — with `--store`/`--service` — compiled claim counts and last-compile date, so
eval-vs-production is never conflated). `show <name>` prints one pack's full resolved
definition and prompt provenance. `install <dir>` validates a directory against `#Layer`
and copies it into `<root>/.fathom/layers/<name>/`; `--force` replaces an
already-installed pack of the same name (whole-directory replace, never a merge). See
[docs/layers.md](layers.md) for the catalog reference.

---

### `compile`

Compile a semantic layer for a service — an LLM pass over the layer's corpus that emits
governed claims (ADR-016). `dry_run` defaults to `true`: it estimates cost without
calling the LLM or writing anything.

```sh
fathom compile --service <id> [--layer obligations] [--dry-run=false] \
  [--store <path> [--profile <name>] [--force] [--unsafe-skip-eval-gate]] \
  [--url http://localhost:7575]
```

| Flag | Default | Description |
|---|---|---|
| `--service` | (required) | Service ID |
| `--layer` | `obligations` | Layer to compile (must be defined in the manifest; `fathom layers list` shows what's visible) |
| `--dry-run` | `true` | Estimate cost without spending; `--dry-run=false` to actually compile |
| `--store` | `""` | Direct mode — bypasses the daemon, compiles against a local store |
| `--url` | `defaultFathomURL()` | HTTP mode base URL (used when `--store` is empty; the daemon compiles with its own configured profile) |
| `--profile` | `""` | Named LLM profile from `~/.fathom/config.toml` (direct `--store` mode only) |
| `--force` | `false` | Override the idempotency guard (same service/layer/watermark/prompt already compiled) — direct `--store` mode only |
| `--unsafe-skip-eval-gate` | `false` | Override the refusal to compile a layer with no passing eval record — direct `--store` mode only |

Both guard-override flags are logged into the resulting `compile_runs` row, never applied
silently. See [docs/layers.md](layers.md) and [docs/configuration.md](configuration.md)
for the provider-lane and layer-catalog references.

---

### `config`

Print the effective configuration: user + project `.fathom/config.toml` layers merged,
the active profile resolved (flag `>` env `>` project file `>` user file `>` built-in
default), printed as human text or JSON.

```sh
fathom config [--profile <name>] [--explain] [--json]
```

`--explain` is the flagship flag — it annotates every resolved key with its provenance
(which layer supplied it, and the exact source file or env var). Pure local file reads,
no daemon dependency.

---

### `providers`

Auth-health probe across configured LLM profiles — **health, not spend**: each
selected profile's client runs one `Ping` (zero/near-zero tokens). Contrast with `smoke`,
which spends real tokens on a real completion.

```sh
fathom providers [--profile <name>] [--json]
```

Exit `0` when every probeable lane is healthy; exit `1` if any probed lane failed or any
profile could not even be constructed. A profile whose adapter doesn't implement `Ping`
is listed as "not probeable," never silently dropped.

---

### `smoke`

One tiny real completion per selected profile — reports model, latency, tokens, and cost.
Because this spends real money, `--profile` or `--all` is **required**; there is no
implicit "smoke everything" default.

```sh
fathom smoke --profile <name> [--json]
fathom smoke --all [--json]
```

Cost is estimated at the same flat per-KTok rates `compile` uses, so a smoke run's
reported cost is directly comparable to a compile run's `cost_usd`.

---

### `instrument`

Channel-health report over the Claude Code hooks beat log: injection coverage
(seen/answered), a per-day rollup, and liveness alarms (e.g. a session that fired
`session-hook` but never saw an `edit-hook` within a week). Advisory only — exit 0
always, even on a missing or empty log; a broken instrument must never gate anything.

```sh
fathom instrument [--log <path>] [--window 168h]
```

| Flag | Default | Description |
|---|---|---|
| `--log` | `beatLogPath()` | Hooks beat log (JSONL) |
| `--window` | `168h` (7 days) | Coverage window, measured back from now |

---

### `report`

Write an analyzer's output as a Markdown finding document — the deterministic bridge
that lets analyzer results flow through the ordinary ingest → compile → claims pipeline
(the `findings` layer) instead of a second persistence path.

```sh
fathom report --store <path> --service <id> [--analyzer security_rollup] [--out findings]
```

v0 supports exactly two analyzers: `security_rollup` (default; `secret_scan` +
`shell_out` + `http_client_calls` over the service's sources) and `authorship` (the
Co-Authored-By trailer ledger joined with staleness and complexity). Output is exactly
one file, `<service>-<analyzer>.md`, inside `--out`; generation is a pure function of the
corpus working tree, so re-running against an unchanged tree reproduces identical bytes.

---

### `init`

One-command, idempotent, additive-only repo wiring (§5.5 of
[coding-agent-excellence.md](design/coding-agent-excellence.md)): writes
`.fathomignore` if absent, ingests the repo via the daemon and confirms it indexed,
wires the SessionStart/PostToolUse/Stop hooks into `.claude/settings.json`, stamps the
fathom operating-contract stanza into both `CLAUDE.md` and `AGENTS.md`, and
write-if-absent scaffolds `~/.fathom/config.toml`.

```sh
fathom init [--path <dir>] [--service <id>] [--url http://localhost:7575] [--bin <path>]
```

| Flag | Default | Description |
|---|---|---|
| `--path` | cwd | Repo root to wire |
| `--service` | resolved from `--path` | Service ID — the same identity the hooks use |
| `--url` | `defaultFathomURL()` | fathom `serve-http` base URL |
| `--bin` | `os.Executable()`, symlink-resolved | fathom binary path baked into the hook commands |

Exit `0` — fully wired and the daemon confirms the service is indexed; exit `2` — wiring
is complete but the daemon is unreachable or the service still shows zero symbols
(operational, mirrors `verify`'s fail-open doctrine); exit `1` is never returned for an
operational reason. Prints the steps it deliberately does not do for you: interactive MCP
registration and the lefthook pre-push snippet. See
[docs/adoption/claude-code.md](adoption/claude-code.md) for the full walkthrough.

---

### `doctor`

Deterministic wiring-diagnosis checklist for a fathom-adopting repo — the one-command
answer to "is fathom actually wired here, end to end?" Every row is `{name, status,
reason, remedy}`; `status` is `pass`, `fail`, or `skipped` (an uncheckable state, e.g. an
unreadable config or an unreachable daemon, is always `skipped` with a reason — never a
guessed `pass`). `remedy` is a single command, present on every `fail`. `doctor` is
strictly read-only: it never writes a file, calls a mutating MCP tool, or restarts
anything.

```sh
fathom doctor [--harness claude|codex|auto] [--path <dir>] [--service <id>] [--url <base>] [--json]
```

| Flag | Default | Description |
|---|---|---|
| `--harness` | `auto` | `claude`, `codex`, or `auto` (detects from `.claude/settings.json`/`~/.claude.json` vs `.codex/config.toml`, defaulting to `claude` with no evidence either way) |
| `--path` | cwd | Repo root to diagnose |
| `--service` | resolved from `--path` | Service ID — the same identity `fathom init` resolves |
| `--url` | `defaultFathomURL()` | fathom `serve-http` base URL |
| `--json` | off | Machine-readable report instead of the human table |

Eleven checks: `mcp_registration`, `daemon_reachable`, `binary_fresh` (the running
binary's own build vs `git rev-parse HEAD` of the checked repo — the local proxy for
daemon staleness; every pass states the daemon's installed copy is unverified, a dirty
build surfaces `vcs.modified`, and both linked-worktree caveats — mismatch inside one,
stamp-poisoning by ones nested under the checkout — are printed only when verified to
apply), `hooks_installed` (any settings scope: project `settings.json`,
`settings.local.json`, or user `~/.claude/settings.json`; when nothing readable wires
them but the call log shows them firing, the verdict names that contradiction instead
of a duplicate-installing remedy) / `hooks_firing` (both Claude Code only — Codex
reports `skipped`, honestly, since it has no hook mechanism), `lefthook_gates`
(harness-agnostic), `stanza_present`, `skills_installed`, `config_valid`,
`obligations_served` (service resolved the way the hooks resolve it — every candidate
identity intersected with the daemon's service list, with a distinct not-indexed
verdict and drift-flagged rules counted into the reason), `surface_traffic`
(process-lifetime counters — "since this daemon process started").

Exit `0` every check passed; `1` at least one check failed; `2` doctor itself could not
run (bad flags, unresolvable `--path`) — an operational error, not a verdict on the
repo's wiring. See the `fathom-doctor` skill for interpretation, the remedy-approval
discipline, and the tie-in/latency-budget checks doctor itself doesn't run.

---

## Go client (`pkg/client`)

`pkg/client` exposes a thin `Client` that wires together the connector, fusion pipeline,
query engine, and store. Import it without touching any `internal/` package.

```go
import "github.com/geoffmilleraz/fathom/pkg/client"

// Open (or create) a SQLite store; wire a local-mode connector.
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

Pass `""` for `storePath` to use an in-memory SQLite database (useful in tests).

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
curated 16-tool coding surface (also served at `/rpc/companion` for
compatibility); bare `POST /rpc` is the full registry. Off-surface calls
execute and are logged `off_surface` rather than rejected; hard gating is
available via the server's strict-surfaces option.

See [mcp-tools.md](mcp-tools.md) for all tools, prompts, and resources, and
[use-cases/coding-agents.md](use-cases/coding-agents.md) for the canonical
coding-agent wiring (hooks, gates, CLAUDE.md/AGENTS.md).
