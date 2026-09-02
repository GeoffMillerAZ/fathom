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
task validate   # full gate: fmt-check + vet + cue-vet + vendor-check + store-purego + scripts-check + go test -race (CGO/tree-sitter)
lefthook install
```

`store-purego` (invariant I2, ADR-025) runs `CGO_ENABLED=0 go test -race ./internal/store/`
on every validate/push — the store stays pure Go; only the tree-sitter extractors need CGO.

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

Output: `discovered`, `extracted`, `skipped`, `symbols` counts, plus non-zero-only rows
(`unclassified`, `oversize`, `classified-but-zero-symbols`, `typed-edges`, `excluded:`
pattern counts, and `read-failed`).

The pipeline is hash-gated: unchanged files (same SHA-256 as the stored fingerprint) are
skipped. Re-ingesting a large repo where few files changed is near-free.

**Ingest honesty (IT-14/IT-15, O16).** A file the connector cannot *read*
(EPERM/EACCES/EIO, or ENOENT for a file deleted mid-run) is skipped and named:
`read-failed: N file(s)` with path + errno (first 5 inline, `... and N more`), its stored
evidence left untouched; every readable file is still indexed; the run exits 1 with
`error: ingest degraded: …`. A run that could read **nothing** while the store already
holds observations for the service **refuses to write** (no watermark advance, no purge):
`error: ingest refused: …`, exit 1, and on "operation not permitted" adds
`hint: external-volume TCC wedge? tccutil reset SystemPolicyRemovableVolumes <terminal bundle id>`.
Transport errors (context cancellation, connector failure) still abort before any write.
The daemon's `ingest` MCP tool returns the same cause + hint as a JSON-RPC error rather
than `ok:true`; `pkg/client` re-exports `client.ReadFailuresError` (`Refused`, `Existing`,
`Hint()`).

**Worktree re-root refusal (PC-33, I30, O22).** When the store already records a root
for `--service` and `--path` is a linked git worktree of that root (`git worktree add`,
or any path under `.claude/worktrees/`), the ingest is refused with
`error: ingest failed: ingest: worktree re-root refused: service "<id>" stays rooted at
<recorded>; <path> is a git worktree (...)`, exit 1, nothing written. The daemon's
`ingest` MCP tool applies the same check (`internal/worktree.CheckReroot`) and its
`ok` envelope names the `root` it refreshed. Moving a service from a worktree back to
its main checkout is allowed. Register a worktree under its own service id if you
mean to index it.

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
fathom serve-http --store <path> [--addr <host:port>] [--module <store>]... [--modules-token-env <NAME>]
```

| Flag | Default | Description |
|---|---|---|
| `--store` | (required, or `[daemon].store` in config.toml) | store path (.sqlite/.db) |
| `--addr` | `127.0.0.1:7575` | Listen address — loopback by default; surfaces are not a security boundary (ADR-014 amendment) |
| `--auto-refresh` | `true` | Re-ingest a service on git-HEAD drift before answering |
| `--call-log` | `workspace/logs/mcp-calls.jsonl` | JSONL tool-call log, resolved against the daemon working directory; empty disables |
| `--module` | — | Path to a conforming expert-module store, mounted READ-ONLY on `POST /rpc/modules`; repeatable; a non-conforming store aborts startup |
| `--modules-token-env` | unset | NAME of an env var holding the bearer token for `POST /rpc/modules` (hosted mode); unset = no auth |

The daemon also reads the `budgets` block of `.fathom/capabilities.cue` /
`~/.fathom/capabilities.cue` (project → user → built-in) for class-budgeted answers
(`list` = `query_symbols`/`references`, `composite` = `pack_context`, `map` = `repo_map`;
BA-9, **experimental**) — see [configuration.md](configuration.md). Stdio `serve-mcp`
still uses the built-in defaults (documented gap).

HTTP endpoints:

| Method | Path | Description |
|---|---|---|
| `POST` | `/rpc` | Single JSON-RPC 2.0 request/response (4 MB body limit) — full tool surface |
| `POST` | `/rpc/{surface}` | Same protocol scoped to a named surface (`agent`, `companion`, `coding`, `investigation`, `shared`, `platform`, `modules`): filters `tools/list`; an off-surface `tools/call` still **executes** and is logged `off_surface:true` (execute-and-flag, ADR-014 amendment) — the hard gate exists only behind `Server.strictSurfaces` and on hosted `/rpc/modules` |
| `GET` | `/healthz` | Liveness probe — returns `ok` |
| `GET` | `/events` | SSE stream — sends `hello` event then stays open |
| `GET` | `/metrics` | Text metrics: `fathom_uptime_seconds`, `fathom_requests_total` |
| `GET` | `/statusz` | Daemon health, per-service index freshness, channel liveness, last compile per layer — JSON via `Accept: application/json`/`?format=json`, HTML otherwise (`docs/design/observability-surfaces.md` §8 item 6). Gated inside the `/rpc/modules` bearer-token allowlist when `--modules-token-env` is configured; open on the default loopback-only daemon |
| `POST` | `/reload` | Reload the store (PlatformStore re-scan in platform mode) |
| `GET` | `/observatory`, `/observatory/service`, `/observatory/impact`, `/observatory/story`, `/observatory/brief`, `/observatory/session`, `/observatory/wiring/` | The observatory (see below); `?format=json` returns the model |
| `POST` | `/observatory/wiring/preview` · `apply` · `undo` | Law-15 wiring through `internal/wiring`: `preview` returns the plan + `plan_digest`; `apply` requires it (409 if the repo changed since preview) and returns a `receipt_digest`; `undo` requires that digest. All three refuse non-loopback `Host`, cross-origin `Origin`, unknown service ids, and paths that are not registered service roots (`requireLocalMutation`) |

```sh
fathom serve-http --store workspace/db/fathom.sqlite --addr 127.0.0.1:7575
# serve-http: listening on 127.0.0.1:7575

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
plan (`.cue` matching `schema/plan/schema.cue`, `.json`, or a `.md` checklist), verifies
each task against the index (the same checks as the `verify_plan` MCP tool), and writes a
`STATUS.cue`.

The plan file is the human-owned source of truth; `STATUS.cue` is regenerated, never
hand-edited — there is no bidirectional sync and no plan state stored in the database.

```sh
fathom gen-status-cue --plan <file> --service <id> [--store <path>] [--output <path>] [--concern <c>]
```

| Flag | Default | Description |
|---|---|---|
| `--plan` | (required) | Path to a `.cue`, `.json`, or `.md` checklist plan file |
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
fathom edit-hook [--url http://localhost:7575] [--pre]
```

| Flag | Default | Description |
|---|---|---|
| `--url` | `http://localhost:7575` | fathom `serve-http` base URL |
| `--pre` | off | Run as the **PreToolUse guard hook** (`docs/design/guarded-edits.md`, `.fathom/guard.cue` + `~/.fathom/guard.cue`) instead of the PostToolUse obligations hook |

Wire it under `hooks.PostToolUse` with matcher `Edit|Write|MultiEdit` (see
[adoption/claude-code.md](adoption/claude-code.md) §7.2 for the full JSON snippet). The
`obligations` layer must be compiled for the service at least once
(`fathom compile --service <id> --dry-run=false`) before this hook has anything to inject.

When `<repo>/.fathom/memories.jsonl` exists, the payload also carries a separate
`🧠 fathom memory — <file>:` block (AM-6, **experimental**) listing memories anchored to
the edited file, each `[tier, state]`, ordered tier → specificity → id, inside the same
150-token whole-payload budget (the displaced count is named); a memory RPC failure emits
an `edit-hook/error` beat, never a block.

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
  [--concern <id>] [--json | --format text|json|sarif] [--quiet] [--quiet-pass]
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
| `--json` | `false` | Emit a JSON report + gate result instead of human text (alias of `--format json`) |
| `--format` | `text` | `text`, `json`, or `sarif` — SARIF 2.1.0 of the refusals: one rule per criterion (`symbol_present`, `test_reaches`, `lexical_oracle`, …), each result located at the plan file and row line, so code scanning and editor problem panes render them. `fathom intent audit --format sarif` and `fathom config audit --format sarif` share the emitter. Exit codes are unchanged |
| `--quiet` | `false` | Print only failing rows (and, with `--strict`, unknown rows) plus the summary line -- the gate mode |
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

`list` prints every visible pack (name, source, kind, tier, stratum, eval status, drift,
corpus, and — with `--store`/`--service` — compiled claim counts and last-compile date,
so eval-vs-production is never conflated). `show <name>` prints one pack's full resolved
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
fathom instrument --adoption [--service <id>] [--call-log <path>] [--store <path>] [--root <dir>] [--json]
```

| Flag | Default | Description |
|---|---|---|
| `--log` | `~/.fathom/hooks.jsonl` | Hooks beat log (JSONL) |
| `--window` | `168h` (7 days) | Coverage window, measured back from now |
| `--adoption` | off | The flywheel gauge: adoption report from the MCP call log — read-error classification, a surface-scoped zero-call list (tools unused on the agent surface vs not exposed there) |
| `--service` | all | (`--adoption`) scope to one service's call-log rows; unattributed rows are excluded and counted |
| `--call-log` | `[daemon].call_log`, else `workspace/logs/mcp-calls.jsonl` | (`--adoption`) MCP tool-call log |
| `--store` | — | (`--adoption`) build the registered-tool list the way `serve-http` does |
| `--root` / `--json` | `.` / off | (`--adoption`) project root for `.fathom/observations.md`; JSON output |

---

### `report`

Write an analyzer's output as a Markdown finding document — the deterministic bridge
that lets analyzer results flow through the ordinary ingest → compile → claims pipeline
(the `findings` layer) instead of a second persistence path.

```sh
fathom report --store <path> --service <id> [--analyzer security_rollup] [--out findings]
```

Analyzers: `security_rollup` (default; `secret_scan` + `shell_out` + `http_client_calls`
over the service's sources), `authorship` (the Co-Authored-By trailer ledger joined with
staleness and complexity), and `reverse-drift`. Output is exactly
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
fathom init [--path <dir>] [--service <id>] [--url http://localhost:7575] [--bin <path>] \
  [--skills[=claude|codex|all]] [--seed-user-ignore]
```

| Flag | Default | Description |
|---|---|---|
| `--path` | cwd | Repo root to wire |
| `--service` | resolved from `--path` | Service ID — the same identity the hooks use |
| `--url` | `http://localhost:7575` | fathom `serve-http` base URL |
| `--bin` | `os.Executable()`, symlink-resolved | fathom binary path baked into the hook commands |
| `--skills` | omit = skip | Install the shipped agent skills (`fathom-activate`, `-custodian`, `-doctor`, `-feedback`, `-impact`, `-onboard`, `-plan`, `-rewire`, `-verify`) for `claude`, `codex`, or `all` (bare `--skills` = all) |
| `--seed-user-ignore` | off | Seed `~/.fathom/ignore` with machine-junk defaults; prints exactly what it writes |

Exit `0` — fully wired and the daemon confirms the service is indexed; exit `2` — wiring
is complete but the daemon is unreachable or the service still shows zero symbols
(operational, mirrors `verify`'s fail-open doctrine). Prints the steps it deliberately
does not do for you: interactive MCP registration and the lefthook pre-push snippet. See
[docs/adoption/claude-code.md](adoption/claude-code.md) for the full walkthrough.

**Law 15 (SH-7/SH-8).** The CLAUDE.md/AGENTS.md stanza regions and `.claude/settings.json`
hooks are written only through `internal/wiring`: one plan → apply → one receipt per repo
(`.fathom/receipts.jsonl`, gitignored; `fathom fleet undo` reverses it). A tracked region
(`<!-- fathom:begin v3 sha256:… -->`) that a human edited is **blocked** (exit 1, file
byte-identical, the other files still applied, one line per file distinguishing
"drifted (hand-edited) region" from "could not be read/parsed"); a v1 region
(`<!-- fathom:begin -->`, no digest) is adopted and upgraded to v3 on the first write —
not drift. `init` continues its other steps around a blocked file and lists the blocked
files at the end.

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
| `--url` | `http://localhost:7575` | fathom `serve-http` base URL |
| `--json` | off | Machine-readable report instead of the human table |

The founding eleven checks: `mcp_registration`, `daemon_reachable`, `binary_fresh` (the running
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
(process-lifetime counters — "since this daemon process started"). Later rows:
`capabilities` (red only when something switched on is failing to deliver), `concerns`
(the custodian — a load error or a violation is the only red), `user_scope_ignore`,
`user_scope_guard`, `guard_file` / `guard_dead_scopes` / `guard_include_modules_unbound`,
`binding_file` / `binding_modules_mounted` / `binding_module_readiness`, `binary_on_path`,
`channels_env`, `ident_cache_env`, `metrics_tier`, `session_ledger_tokens`, and
`cpp_toolchain` (CMake repos: `compile_commands.json` present). The stanza check accepts
both the v1 and the v3 (`sha256:`-digested) begin marker.

Exit `0` every check passed; `1` at least one check failed; `2` doctor itself could not
run (bad flags, unresolvable `--path`) — an operational error, not a verdict on the
repo's wiring. See the `fathom-doctor` skill for interpretation, the remedy-approval
discipline, and the tie-in/latency-budget checks doctor itself doesn't run.

### `plans` — computed SDD progress and archival

```sh
fathom plans status  --service <svc> [--plan P]... [--store DB | --url U] [--json]   # per-plan rollup: total/proven/claimed-unproven/open/deferred/other-skipped
fathom plans next    --service <svc> [--eligible-only] [--free] [--root DIR] [--json] # the ordered work frontier, with {id, symbol, package} anchors
fathom plans context --service <svc> --row <id> [--budget 4000]                       # render a row's ctx: context pack (elided beyond the byte budget)
fathom plans lease   --plan P <row-id> [--holder H] [--ttl 2h] [--release] [--root DIR] # TTL advisory lease on a row (.fathom/leases.jsonl)
fathom plans leases  [--root DIR] [--json]                                            # list live leases
fathom plans archive <plan> --service <svc> [--root DIR]   # retire a fully-proven plan to docs/plans/archive/ (out of gate scope)
fathom plans restore <plan> [--root DIR]                   # bring it back
```

State is always derived from the index by the verify engine, never asserted.
`next --eligible-only` hides rows blocked on an unmet `needs:` target (the
derived `eligible` / `blocked_on` state); `--free` hides rows holding a fresh
lease. `archive` refuses unless every row id is lexically absorbed into
`CHANGELOG.md`, `ADR.md`, or `docs/changelog/*.md` (the per-release
narrative) — the refusal names all three. Heads-up: `plans status` currently
verifies every active plan serially over HTTP (minutes on a large plan set —
O29 in `docs/dogfood-observations.md` tracks the fix).

### `thread` — continuity across sessions (vision 21)

```sh
fathom thread brief   [--budget-bytes 2400] [--repo .] [--source startup|clear|resume|compact|fork] [--all-repos]
                                               # what a fresh session gets injected (startup/clear only; resume/compact get nothing)
fathom thread note    "text" [--thread T] [--kind K] [--completes-when …] [--repo .]   # file a handoff BEFORE /clear
fathom thread point   "what happened" [--session S] [--thread T] [--repo .]          # side-task pointer
fathom thread list    [--all-repos] [--all] [--thread T] [--repo .]   # this repo's open items; --all-repos: every repo; --all: also closed + dormant, every repo
fathom thread close   <id> [--why …]
fathom thread promote <thread> [--out docs/plans/<thread>.md] [--repo .]   # scaffold a thread's history into a plan
fathom thread handoff [--repo .]               # derived in-flight state for a handoff
```

Store: `~/.fathom/threads.jsonl` — user-central, append-only,
latest-row-wins; never shared (law 13). Deposits from the lifecycle hooks
(`session-end-hook`, `pre-compact-hook` — wired by `fathom init`) are
mechanical facts only. `--kind` is one of `decision-pending`, `spec-unbuilt`,
`in-flight`, `side-task`, `handoff-note`, `session-end`, `pre-compact`
(default `handoff-note`). Items with `completes_when`
(`file_exists:<path>` / `plan_row_checked:<plan>#<ROW-ID>`) close themselves
when reality catches up. The brief is byte-budgeted and enforced; overflow is
named (`and N more`), never silently dropped.

**Repo-scoped (TH-6).** `note`/`point` file on the current repo's
daemon-resolved service id by default (`general` only when nothing resolves;
`--thread` wins). `list` and `brief` show this repo's items and print one
line `N open item(s) in other repos — fathom thread list --all`; untagged
legacy items are never hidden. The continuity-thread bet itself is
**experimental** (4/20 units collected).

### `config audit` — the config custodian (vision 22, "instructed means true")

```sh
fathom config audit [--repo .] [--json]
```

Audits registered concerns (`~/.fathom/concerns.cue` user defaults;
`.fathom/concerns.cue` repo scope, governs) against the repo: per-concern
`holds` / `violated (file:line evidence)` / `unable-to-check (reason)` /
`off (named)`. Exit `1` iff violated; zero concerns exits `0` with the
absence named. Check kinds: `forbidden_pattern`,
`command_documented_and_exists`, `max_lines`, `pointers_resolve`, plus
(CC-14/CC-15, **experimental**) `stanza_tools_served` — every tool the
fathom stanza names must be served by the daemon's `/rpc/agent`
`tools/list` (the one kind that asks the daemon; daemon down → `unable to
check` naming the URL, never violated) — and
`handoff_cites_instruments` (`days: N`) — a handoff thread note claiming
shipped work must cite a commit sha, an active plan row id, `fathom
verify`, or a `workspace/artifacts/retros/` path — and
`adr_references_resolve` — every path a decision record cites must exist,
scoped to this repo's own id shape so a sibling repo's records are named
rather than reported missing. Fathom reports and never edits your prose —
the session applies fixes. Full model: `docs/design/config-custodian.md`.

### `daemon` — install, inspect and remove the serve-http agent

```sh
fathom daemon install [--label dev.fathom.serve-http] [--store PATH] [--addr :7575] [--bin PATH] [--force]
fathom daemon status  [--url http://localhost:7575]
fathom daemon uninstall [--label NAME]
```

macOS/launchd. `install` writes `~/Library/LaunchAgents/<label>.plist` with
**your** resolved binary (symlinks followed, so launchd runs the stable
installed copy rather than a `bin/` build output a rebuild would truncate),
store at `~/.fathom/db/fathom.sqlite`, call log at
`~/.fathom/logs/mcp-calls.jsonl`, and a PATH carrying your Go toolchain —
without `go` the daemon's auto-refresh degrades Go services to syntax-only
edges, so `install` says so when it cannot find one. It refuses to stack a
second agent on the same port, naming the one already there and how to keep
or replace it. `uninstall` leaves the store and logs alone.

`status` prints the installed label and whether the daemon is actually
serving — an installed agent that is not serving is worse than none, because
everything downstream assumes it.

Every "the daemon is down" message in fathom (doctor's remedy, the
SessionStart line, `init`'s closing note) names the label **discovered on
this machine**, never a fixed one. Off macOS, all of them fall back to
`fathom serve-http` under your own supervisor.

### `intent` — the intent corpus, audited (ADR-026, **experimental**)

```sh
fathom intent audit  [--root .] [--json] [--kind conditions|invariants|unproven] [--fresh] [--top N]
fathom intent export --guard [--root .]
```

Reads a repo's Project Intent corpus (`docs/intent/` router, invariants,
conditions, sensitive surfaces, and the decision records the router points
at) and reports three things: **what it declares**, **what an instrument can
show**, and **what nothing shows**. It never edits the corpus, never
evaluates a condition an author did not annotate, and never scores a
repository.

Three tiers, all normal: a router-bearing repo gets the full audit; a repo
with decision records and no router gets the conditions from their
`Revisit when` trailers; a repo with neither is simply not adopting the
practice, which is a choice.

`audit` reports unproven claims by kind, each with a remedy:

| Kind | What it names |
|---|---|
| U1 / U2 | plan rows the verify gate proved failing, or claimed with no contract (read from the newest recorded gate run — never a fresh verify, which costs ~70 s here) |
| U3 | invariants at rung `VIOLATED`. `stated` and `reviewed` are listed as UNENFORCED, not as debt — turning an honest rung into debt is how rungs get faked |
| U4 | an invariant naming a test or gate step that does not resolve |
| U5 | conditions that fired and were never decided |
| U6 | incidents open past 30 days with no outcome |
| U11 | a record citing a path that has moved |
| U12 | a row claiming done inside a sensitive surface while pinning no named test |
| U14 | a sensitive-surface row whose globs match no file — it has silently stopped guarding |
| U13 | a guard entry whose intent provenance no longer matches the router, or a sensitive surface no guard covers |

Every source it could NOT compute is named with the reason — an audit that
silently yields nothing is indistinguishable from a healthy repo. A file that
cannot be READ (as opposed to one that is absent) makes every count a named
lower bound, `≥ N (k source(s) unreadable: <path>: <errno>)`, and `--json`
carries `coverage.complete` (ADR-027).

`export --guard` PROPOSES `.fathom/guard.cue` entries from the router's
sensitive-surface rows, stamped with per-row provenance so staleness stays
computable. It prints them; **fathom never writes that file** (law 15).

Two `fathom doctor` rows report the corpus and its debt, and neither can
FAIL: not adopting the practice is a choice, and honest debt is not a broken
repo. Switch both off with the `intent-corpus` capability — the verb still
answers on demand. Full design: `docs/design/evidence-plane.md`.

### `scope` / `purge` — reversible suppression and verified removal

```sh
fathom scope list      [--all] [--service S] [--json]
fathom scope suppress  '<glob>' --reason R [--service S] [--until 7d]
fathom scope unsuppress <id>
fathom scope preview   '<glob>' [--service S] [--store DB] [--sample 10] [--json]
fathom purge '<glob>' --service S --reason '<why>' [--store DB] [--dry-run | --yes] \
     [--all] [--probe <literal>]... [--include-history --force] [--json]
fathom purge history [--json]
```

`scope suppress` hides matching paths from query results (reversible,
reason-carrying, disclosed in every MCP envelope's `suppressed` block);
`purge` removes matching content from the store irreversibly, appends a
service-scoped ignore so re-ingest cannot resurrect it, and its verdict is
tri-state with a post-purge byte probe — exit `2` means the SQL rows are
gone but the raw store bytes were left unchecked. `--all` is the glob
`**`; `--include-history` also deletes `claim_events` (breaks claim-history
integrity) and needs `--force` as a second key. See
`docs/use-cases/fleet-health.md` and `docs/design/scope-and-purge.md`.

### `retro` — did the firings actually help? (mechanical, free)

```sh
fathom retro [--session S | (latest stamped)] [--project DIR] [--json] [--write]
```

Joins a session's intervention firings (guard fires, obligation
injections, custodian attention) against what is on disk afterward:
committed / modified-uncommitted / untouched per fired file, concern
remediation now-state, silent-on-clean counts. Unjoinable (pre-stamp)
firings are counted and named; OFF-arm windows say channels were silent
by design. Outcome-shaped evidence, never causality (the arm carries
that — law 12). `--write` lands the retro under
`workspace/artifacts/retros/`. The priced judgment lane is `assess`
(IR-3, not yet built). Design: `docs/design/intervention-outcomes.md`.

### `rate` — the session outcome, in one number

```sh
fathom rate <1-5> [--note "..."] [--repo .] [--session S]
fathom rate --list
```

Appends to the local ratings log — the outcome half of the measurement
loop (`observe` files the specific friction; `rate` files the verdict).
**Experimental**: an optional human override; the A/B criterion cannot be
evaluated until ratings exist.

### `brief` / `flywheel` — the value story and the pipes it rides on

```sh
fathom brief [--print | --out ~/.fathom/briefs] [--url U]   # the shareable markdown value brief, honesty covenant included ("Not measured" is a section, not a gap)
fathom flywheel [--json] [--url U]                          # every feedback loop: CLOSED / OPEN / IDLE — exit 1 if any loop is OPEN
```

Run `flywheel` before quoting any number `brief` renders: a figure
downstream of an OPEN loop is not evidence. `fathom capabilities` (below)
explains why any surface reads zero. The A/B counterfactual behind the
savings figures is **experimental** — the ON arm is still collecting (≥20
units per arm needed; every savings claim is inadmissible until then, law
12). `fathom arm [on | off --for D --sessions N]` switches the arm;
`fathom narrate [on --for D --sessions N | off]` arms self-expiring story
capture (neither is listed in `fathom help`).

### `fleet` — report, reconcile, receipts, undo (law 15)

```sh
fathom fleet report    [--json | --out ~/.fathom/fleet] [--url U]           # wiring/freshness across every indexed repo
fathom fleet reconcile [--repo R]... [--apply] [--url U]                     # dry-run by default; --apply: shim repoint + additive template merge + stanza
fathom fleet receipts  [--repo R] [--limit 20] [--json]                      # every write fathom makes in a repo is receipted…
fathom fleet undo      [--last | <receipt-id>] [--repo R] [--json]           # …and reversible
```

`reconcile --apply` writes stanza regions and hooks only through
`internal/wiring` — the same plan → apply → receipt path as `init`, with the
same law-15 refusal for a hand-edited tracked region (blocked and named;
other files still applied). Dry-run distinguishes `stale` from `up to date
(will be re-marked with a digest on apply; no prose changes)` for a v1
region awaiting its digest.

### `prime` — the Codex pull tier

```sh
fathom prime [--dir <project-dir>] [--url http://localhost:7575]
```

Prints the full session primer — exactly what the SessionStart hook composes
into `additionalContext` — as plain text, for a harness with no SessionStart
hook (Codex). Always exits 0 (daemon down degrades to an honest status
line). `--dir` defaults to `$CLAUDE_PROJECT_DIR`, else cwd.

### `observe` — file a friction observation

```sh
fathom observe --symptom "..." [--expected "..."] [--actual "..."] [--repro "<verbatim command>"] [--tool <name>] [--root .]
```

Appends a structured entry to `<root>/.fathom/observations.md`. `--symptom`
is required.

### `review` — the change-review verb (not listed in `fathom help`)

```sh
fathom review --service <id> --since <git-ref> [--store DB] [--no-persist] [--budget-seconds N] [--call-log P] [--json]
```

CLI mirror of the `review_diff` MCP tool: diff impact + obligations/findings
+ architecture conformance + spec coverage in one cited verdict, followed by
the test-impact block (BD-5, `docs/plans/blast-depth.md`) — the `Test*`
functions that statically reach a changed symbol (`run …`, cited file:line,
with the symbols each reaches) and the changed symbols no test reaches
(`gap …`):

```
[x] test_impact: 2 affected test(s), 1 changed symbol(s) with no test
  - run svc::pkg.TestFoo (pkg/a_test.go#L1-L1) reaches svc::pkg.A, svc::pkg.B
  - gap svc::pkg.Orphan (pkg/orphan.go#L1-L1): no test reaches it
```

`--json` carries the same block as `test_impact` (`{checked, reason?,
affected_tests, test_gap}`), identical to `review_diff`'s. Persists
the deterministic-oracle findings to the claims store unless `--no-persist`
(the pre-push summary hook uses it). `--budget-seconds` is advisory — on
excess it prints a named skip and exits 0; it requires `--no-persist` (a
bounded run must never be a store writer).

### `gen-plan-cue`

```sh
fathom gen-plan-cue --plan docs/plans/<slug>.md --service <id> [--output <path>]
```

Emits the generated CUE shadow of a Markdown plan (round-trip verified);
stdout by default.

### `research` — the research flywheel

```sh
fathom research report [--since 168h] [--service S] [--root .] [--out DIR] [--call-log P] [--hooks-log P] [--bench-dir D] [--no-write] [--json]
fathom research bundle [--since 168h] [--service S] [--root .] [--out DIR] [--dry-run] ...
```

`report` aggregates local evidence (call log, hooks log, context-bench
results, observations) into `workspace/artifacts/research/report.{md,json}`;
`bundle` renders the sanitized, shareable summary (schema
`schema/research-bundle/`) — sharing stays a human verb. Design:
`docs/design/research-flywheel.md`.

### `module` — expert-module lifecycle (**experimental**, law 9: read-only before writable)

```sh
fathom module init    --name N --description T --include GLOB[,…] [--exclude …] [--dir D] [--root D] [--tier-profile P] [--license L] [--force] [--json]
fathom module sources --manifest module.cue [--root D] [--json]
fathom module map     --manifest module.cue [--root D] [--store P --service S] [--json]
fathom module compile --manifest module.cue --store P [--service S] [--dry-run=true|false] [--profile P | --mock --mock-entries-file F] [--force] [--unsafe-skip-eval-gate] [--json]
fathom module eval    --manifest module.cue [--fixtures D] [--profile P | --mock --mock-entries-file F] [--record P] [--json]
fathom module doctor  --manifest module.cue [--root D] [--store P --service S] [--pack-dir D] [--require-level L] [--json]
fathom module pack    --manifest module.cue --store P --out D [--force] [--json]
fathom module runs    --manifest module.cue --store P [--service S] [--limit 20] [--json]
```

Specs: `docs/spec/expert-module-v0.md`, `docs/spec/store-format-v0.md`;
design `docs/design/expert-modules.md`. A packed module is mounted read-only
by `serve-http --module`; no external consumer exists yet.

### `metrics` — observability surfaces

```sh
fathom metrics spend         --store P [--service S] [--module LAYER] [--by layer|day|model] [--since DUR | --from RFC3339 --to RFC3339] [--root D] [--json]
fathom metrics traffic       [--service S] [--by surface|tool|hour] [--since DUR | --from … --to …] [--call-log P] [--json]
fathom metrics channels      [--since DUR | --from … --to …] [--hooks-log P] [--json]
fathom metrics interventions [--since DUR | --from … --to …] [--store P] [--call-log P] [--hooks-log P] [--json]
fathom metrics series        <metric> --bucket hour|day [--since DUR] [--root D] [--json]
fathom metrics rollup        [--date YYYY-MM-DD] [--root D] [--store P] [--call-log P] [--hooks-log P] [--json]
fathom metrics rotate        [--call-log P] [--keep-days N] [--apply]
```

Design: `docs/design/observability-surfaces.md`.

### `findings` — findings as evidence (**experimental**; entry ≥15–20 resolved findings)

```sh
fathom findings near   <symbol|file> --store P --service S [--hops 2] [--include-superseded] [--json]
fathom findings status --store P --service S [--json]
```

`near` is the call-graph precedent lookup (mirror of the `findings_near`
MCP tool); `status` reports entry-condition progress. Design:
`docs/design/findings-as-evidence.md`.

### `memory` — anchored memory (**experimental**)

```sh
fathom memory add    --store P --service S --claim TEXT --anchor KIND:REF [--anchor …] --evidence KIND:REF [--evidence …] [--tier T] [--confidence N] [--by ACTOR] [--root D] [--json]
fathom memory list   --store P --service S [--state fresh|suspect|orphaned|retired] [--root D] [--json]
fathom memory review --store P --service S [--id ID --resolve still-true|no-longer-true|rewrite --by ACTOR [--claim TEXT] [--note TEXT] [--commit C]] [--root D] [--json]
fathom memory show   --id ID [--root D] [--json]
```

Memories live in `<root>/.fathom/memories.jsonl`. Anchor kinds, sharpest
first: `symbol | file | commit | claim | path`; evidence kind sets the tier
(`machine-check` → verified, `index` → pre-scan, `doc` → doc-claimed,
`human` → sme-stated, `model` → assumed). A memory with no anchor is
refused — that is a preference, not a memory about this codebase. Anchor
drift moves a memory to `suspect` (tier dropped one step) or `orphaned`;
retired memories never surface. Served by the `memory_search` MCP tool
(agent surface) and the edit-hook memory block. Design:
`docs/design/anchored-memory.md`.

### `plugin` (not listed in `fathom help`)

```sh
fathom plugin render [--out DIR] [--mcp-url U] [--marketplace]
```

Renders the Claude Code plugin tree (hooks, skills, MCP registration) into
`--out`; the output dir belongs to the generator, so re-rendering overwrites.

---

## The observatory and the brief

The daemon serves a value-realization UI (vision 17) next to `/statusz`:

- `GET /observatory` — fleet tiles: wiring, freshness, traffic, open
  observations; every warning names its remedy command. `?format=json`
  returns the model.
- `GET /observatory/service?id=S` — one repo's adoption gauge, hook
  wiring, freshness with the exact re-ingest command, and (RG-6) 30-day
  per-dimension series (queried, discovering, preventing, gating) from
  real call-log rows — a day with no call-log rows at all renders Absent
  ("no call-log rows for <date>"), a day the log was alive but this
  service saw nothing is a measured 0, and sparklines never draw across
  an absent day.
- The fleet page's grade distribution over 30 days (RG-7) is
  **experimental**: days without a snapshot are Absent — today's fold
  only, until daily snapshots persist (the page says so). Symbols/traffic/
  serves tiles render Absent, never 0, when the call log or store could
  not be read.
- `GET /observatory/wiring/` — per-repo wiring plans; `preview` → `apply`
  → `undo` (see the `serve-http` endpoint table).
- `GET /observatory/impact` — verify catches and discovery actions with
  their denominators, and the counterfactual pilot as a state machine
  (a savings figure exists only as an arm range — law 12). Story rows
  render under an explicit lens banner (law 14).
- `fathom brief [--print|--out DIR]` — the same impact model as a dated
  local markdown brief, with an explicit "Not measured" section.
  Sharing is a human verb (law 13): the verb writes a file and stops.

## Capabilities — what is on here, and why anything is off

`fathom capabilities` answers the most common adoption question in one
place. A capability is reported **on only when it is observably
operating** — a config file saying `guards: on` with no rules compiled
reports off, with the reason (law 3 applied to configuration).

```sh
fathom capabilities [--repo .] [--service S] [--url U]   # table: state, evidence, and the reason for every gap
fathom capabilities --cost              # what each one costs when on
fathom capabilities --json              # the model doctor and the observatory render
fathom capabilities explain guards      # what it does, what it costs, what evidence it produces
fathom capabilities set narrate on      # writes .fathom/capabilities.cue
fathom capabilities set narrate on --scope user
```

`set` splices only the `capabilities` block and preserves everything else in
the file byte-for-byte (a `budgets` block, comments); it refuses when the
file cannot be round-tripped. Non-identifier ids such as `verify-gate` are
written as quoted labels. The same file's optional `budgets` block (BA-9,
**experimental**) is documented in [configuration.md](configuration.md).

Two scopes, resolved built-in → user → project, with the winning scope
shown beside every value (law 13):

| File | May do |
|---|---|
| `~/.fathom/capabilities.cue` | set a **default** for repositories that have not spoken |
| `<repo>/.fathom/capabilities.cue` | **govern** — the final word for that repository |

A user-scope file that tries to set a project-only capability (a gate, a
guard) is **refused with the key named**, never silently ignored. An
unknown id is refused with the nearest known id suggested. `on?: false`
— which in CUE declares the field may exist rather than setting it, so
the line does nothing — is refused with the fix in the message.

`fathom doctor` carries one `capabilities` row. It fails **only** when a
capability is switched on and something is failing to deliver it (rules
compiled but no hook fires; an index behind HEAD). A capability whose
inputs are simply absent here — no module bound, no layer pack installed
— is not a failure and does not red-mark the row.

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

Pipe JSON-RPC 2.0 messages to the `serve-mcp` process. `initialize` echoes
your `protocolVersion` when it is one of `2025-11-25`, `2025-06-18`,
`2025-03-26`, `2024-11-05`, and answers `2025-11-25` otherwise; every
agent-surface tool in `tools/list` carries `annotations` and an
`outputSchema`, and `tools/call` returns `structuredContent` beside the text
block (`docs/mcp-tools.md` § Protocol contract):

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

There is no profile-switching tool (the old `set_profile` was removed; ADR-014
amendment, 2026-07-16): a stdio session always sees the full registry, and over
HTTP a surface is selected by path — `POST /rpc/agent` is the curated 17-tool
coding surface (`internal/mcp/surface.go`; also served at `/rpc/companion` for
compatibility); bare `POST /rpc` is the full registry (54 tools plus
`memory_search`). Off-surface calls execute and are logged `off_surface:true`
rather than rejected; the hard gate exists only behind the server's
`strictSurfaces` option and on hosted `/rpc/modules`.

See [mcp-tools.md](mcp-tools.md) for all tools, prompts, and resources, and
[use-cases/coding-agents.md](use-cases/coding-agents.md) for the canonical
coding-agent wiring (hooks, gates, CLAUDE.md/AGENTS.md).
