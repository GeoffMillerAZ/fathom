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

### Pushing (`task push`)

```sh
task push      # scripts/push-check.sh, then: git push origin HEAD:main
```

INC-20 (2026-09-19): two pushes reached `origin/main` ungated after a push clone's
`.git/hooks/pre-push` was wiped by an unrelated process — `task validate` never ran
before either push, and both push logs were two lines long, the tell of a gate that
never fired. `task push` refuses to push at all unless `scripts/push-check.sh` finds
lefthook's own pre-push hook in place first (invariant I40, ADR-041 decision 10):

- exit 0, `push-check: ok (lefthook pre-push present)` — the hook is present, push
  proceeds.
- exit 3, one line naming why — the hook is missing, empty, or present but not
  lefthook's (a plain `git init`'s own `*.sample` hooks, for instance). Run
  `lefthook install` and re-run `task push`.
- exit 2 — an operational error (not a git repository, `git` not on `PATH`), not a
  verdict about the hook.

The check resolves the hook path through `git rev-parse --git-path hooks/pre-push`,
so a repo with `core.hooksPath` pointed elsewhere, or a linked worktree, is checked
where its own hook actually lives — never a hardcoded `.git/hooks/pre-push`. `git
push` directly (skipping `task push`) still works and is never itself gated — the
gate is `task push`'s own contract, not a server-side or repo-level enforcement; use
`task push` so the gate actually runs. `scripts/push-check_test.sh` (wired into
`task scripts-check`) proves the check against disposable temp repos: a missing hook,
a non-lefthook hook, a real lefthook hook, and a `core.hooksPath` variant.

### Build-cache placement and disk guard

Fathom's supported build path (`task ...` and repository scripts) preserves the
machine's normal Go and temporary-directory settings by default. To give Fathom a
dedicated cache—on an external volume, for example—create the root first and export:

```sh
mkdir -p /path/on/the/intended/volume/fathom
export FATHOM_CACHE_ROOT=/path/on/the/intended/volume/fathom
export FATHOM_CACHE_MAX_MIB=8192 # optional; 0 disables automatic pruning
task validate
```

The root must be absolute, pre-existing, and writable. Requiring it to exist is
intentional: if a removable volume is absent, Fathom refuses instead of recreating the
mount path on the system disk. The build flow derives `GOCACHE`, `GOMODCACHE`,
`GOTMPDIR`, `TMPDIR`, and `UV_CACHE_DIR` beneath that root. Before a supported Go
invocation, a build cache larger than `FATHOM_CACHE_MAX_MIB` is cleared with
`go clean -cache`; module downloads are retained.

Raw `go` commands bypass repository policy. Use `task test`, `task build`, or the
corresponding repository script when cache placement matters.

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

This is fathom's own dev-loop convention — its `.fathom/config.toml` opts into it
(`[daemon] state_dir = "workspace"`). It is NOT the default for a repo fathom adopts
(O63 item 5, `internal/statedir`): a hook's own runtime state (session/observed-discovery
logs, retro/narrate artifacts, the receipts log, MCP call logs) defaults to OUTSIDE the
subject repository entirely, under `$XDG_STATE_HOME/fathom/<repo-id>/` (default
`~/.local/state/fathom/<repo-id>/`), so a fresh adopter's `git status` after `fathom init`
and normal use never shows fathom's own hook output. A repo opts back into the old
in-tree layout the same way this one does — `[daemon] state_dir = "workspace"` in its own
`.fathom/config.toml` — or `FATHOM_STATE_DIR` overrides the directory directly for one
invocation (hooks' fixed command lines can't pass a flag). The code index (`-store` /
`workspace/db/`) is unaffected either way — it is the artifact fathom exists to produce,
not runtime state, and stays under the repo's `workspace/` regardless.

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

- `--store` defaults to `<repo>/workspace/db/fathom.sqlite` — the enclosing git
  repository's `workspace/`, from any subdirectory — or `~/.fathom/db/fathom.sqlite`
  when the cwd is not inside a repository; never the bare cwd (invariant I31, O11).
  `[daemon].store` in `config.toml` and an explicit `--store` still win. The same
  rule resolves `--call-log` (`<repo>/workspace/logs/mcp-calls.jsonl`, else
  `~/.fathom/logs/mcp-calls.jsonl`; `FATHOM_CALL_LOG` and `[daemon].call_log` win)
- `--mode` selects the connector: `local` (default), `faux` (fixture data), or `github`
- Exit 0 on success, 1 on any error

### `ingest`

Ingest a service into the knowledge store.

```sh
fathom ingest --service <id> [--path <dir>] [--store <path>] [--mode local|faux] [--scip <path>]... [--json]
```

| Flag | Default | Description |
|---|---|---|
| `--service` | (required) | Logical service ID, e.g. `payment-service` |
| `--path` | `.` | Source root directory (local mode) |
| `--store` | `<repo>/workspace/db/fathom.sqlite` (see §CLI subcommands) | store path (.sqlite/.db) |
| `--mode` | `local` | Connector mode: `local` or `faux` |
| `--scip` | auto-detected | SCIP index file to import as typed edges (ADR-041 decision 5, SU-9; repeatable). Unset: auto-detects `index.scip` at the service root and every `.fathom/scip/*.scip` |
| `--json` | off | Emit the run as JSON: `{service, outcome: ok|degraded|refused, exit_code, discovered, extracted, skipped, symbols, read_failed, read_failures: [{path, err, errno?}], unclassified, oversize, classified_zero, unsupported_languages?, typed_edges?, scip_imports?, excluded?, error?, hint?}` — `errno` is the class (`EPERM`, `EACCES`, `ENOENT`, `EIO`, `EISDIR`) derived from the connector's text, so CI can assert `read_failed == 0`. `unsupported_languages` (O69, present only when non-empty) is `{language: file_count}` — e.g. `{"swift": 412}` — for source in a KNOWN programming language fathom has no registered extractor for; see "Unsupported languages" below. `scip_imports` (present only when at least one SCIP index was imported) is `[{path, producer, language?, documents, edges, direct, unresolved, unmapped}]` — see "Typed edges by import" below |

Output: `discovered`, `extracted`, `skipped`, `symbols` counts, plus non-zero-only rows
(`unclassified`, `oversize`, `classified-but-zero-symbols`, `unsupported languages (no extractor)`,
`typed-edges`, `typed_edges: … via <producer>` per imported SCIP index, `excluded:` pattern
counts, and `read-failed`).

**Typed edges by import (ADR-041 decision 5, SU-9).** Go gets typed call edges natively
(`internal/fusion.EnrichTypedEdges`, type-checking the source directly); every other
language gets them by IMPORTING an external indexer's SCIP output instead —
`internal/scip.ImportIndex` maps a SCIP index's definition/reference occurrences onto
fathom's `typed_edges` table, replaced per fathom package exactly like the native pass.
fathom never runs or vendors a per-language type-checker itself: produce the index
separately (`scip-typescript index`, `scip-python index`, …, per `docs/languages.md`'s
tracked delegate order) and either name it with `--scip PATH` (repeatable — one ingest
can import several indexes) or drop it at `index.scip` in the service root / anywhere
under `.fathom/scip/*.scip` for auto-detection. A reference occurrence resolves to
`core.EdgeDirect` when its symbol is defined somewhere in the same index, or
`core.EdgeUnresolved` when it names something outside the index (never silently
dropped). The summary line names the producer fathom read from the index's own
metadata, e.g. `typed_edges: 118 edge(s) (94 direct / 24 unresolved) via scip-typescript
0.3.16 [index.scip]` — `fathom capabilities --json`'s `typed_edge_producers` field names,
per language, which mechanism fathom's own code currently provides (`native` /
`scip:<tool>` / `file` / `syntax`).

Each definition is mapped to fathom's own symbol space by its POSITION — the same
service's already-extracted symbol table (the native or file-resolved pass's output for
the files the index covers, always present by the time `-scip` runs), matched by
document path + the definition's own line — falling back to parsing the SCIP symbol
string's own descriptor suffix only when that position match is ambiguous (O115: the
real, currently-published `scip-typescript`/`scip-python` releases never set a
definition's `DisplayName`, so a name reconstructed from it — the pre-fix behaviour —
never joined fathom's own symbol keys, and `find_callers`/`references`/`blast_radius`
silently resolved none of a SCIP-imported service's edges). A definition neither path can
place contributes NO edge, in either direction, and is counted in the summary's
`scip_imports[].unmapped` (JSON) — printed as a warning on stderr when non-zero, e.g.
`warning: scip import index.scip: 3 definition(s) could not be mapped to a fathom symbol
and contributed no edge (O115)` — never silently folded into `direct` or `unresolved`.

**Unsupported languages (O69).** `unclassified` lumps every file with no registered
extractor into one count, unnamed — a lockfile, an image, and a real Swift source file
all count the same way. When an unrecognized extension is also a KNOWN programming
language (`internal/connector.UnsupportedLanguageForPath` — currently: Swift, Dart,
Elixir, Clojure, Haskell, OCaml, F#, Erlang, Zig, Nim, Julia, Elm, Crystal, Groovy),
ingest names it and counts its files separately, so a repo whose primary language has no
extractor reads as "swift: 412 files, no extractor" instead of a suspiciously low symbol
count with no explanation. The same breakdown is in `list_services`' per-service
`unsupported_languages` and the `unsupported_languages` doctor row (WARN, not FAIL — a
capability gap, not a misconfiguration). None of the languages above have a tree-sitter
grammar or extractor wired into this repo today; adding one is a separate, larger change
(a real extractor/grammar, TDD'd against a calibration fixture) — this is disclosure only,
never a fake extraction. Kotlin was on this list through wave P1; wave P2 (O79) closed it
— `.kt`/`.kts` now extract via `internal/extractor.NewTSExtractor("kotlin")` (tree-sitter
via wazero/WASM, ADR-021 §4, `internal/extractor/tsengine/grammars/kotlin.wasm`) — the
first language added for new-language breadth rather than a CGO-incumbent swap.

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
| `--store` | `<repo>/workspace/db/fathom.sqlite` (see §CLI subcommands) | store path (.sqlite/.db) |
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
| `--store` | `<repo>/workspace/db/fathom.sqlite` (see §CLI subcommands) | store path (.sqlite/.db) |
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
| `--store` | `<repo>/workspace/db/fathom.sqlite` (see §CLI subcommands) | store path (.sqlite/.db) |

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
| `--store` | `<repo>/workspace/db/fathom.sqlite` (see §CLI subcommands) | store path (.sqlite/.db) |

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
| `--call-log` | `<repo>/workspace/logs/mcp-calls.jsonl` (else `~/.fathom/logs/mcp-calls.jsonl`) | JSONL tool-call log, resolved from the daemon working directory's repository (§CLI subcommands); empty disables |
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
| `GET` | `/events` | Live freshness (OC-13): SSE with an `id:` per event (`<epoch>-<n>`: the epoch is this daemon process's boot ID, named in the opening `hello` event), a `: keepalive` every 25 s, replay of the last 64 events after `Last-Event-ID` (or `?last_event_id=`) — an id from another epoch (a reconnect across a daemon restart) replays the whole ring, the restart event first — at most 16 subscribers (the 17th gets 503 + `Retry-After`). Kinds: `ingest` (a service's indexed content moved; a hash-gated no-op re-ingest is not one), `compile` (one per new run), `verify` (the verify ledger, polled every 2 s), `session` (the row's repo label mapped to the service whose root it names; a label that is not a name fires only the unscoped `sessions` source), `daemon`. Events carry names and IDs only, never content |
| `GET` | `/metrics` | Text metrics: `fathom_uptime_seconds`, `fathom_requests_total`, memory and memo gauges, `fathom_console_page_views_total{route=…}` (OC-15: HTML views per console route pattern since start; the durable 30-day record is `~/.fathom/console-views.jsonl`, `FATHOM_CONSOLE_VIEWS` overrides) and `fathom_console_event_subscribers` |
| `GET` | `/statusz` | Daemon health, per-service index freshness, channel liveness, last compile per layer, and (schema 2) the daemon's own build stamp `binary_commit` / `binary_dirty` / `built_at` — JSON (schema 3, unchanged) via `Accept: application/json`/`?format=json`; HTML on the console shell otherwise, where every service names its typed-edge state — run by this daemon, recorded by an earlier daemon process (replayed from the store), failed with its reason, **does not apply** (no `go.mod` at the service root), or **unknown** (a Go module with no pass recorded anywhere), never blank (OC-6; `docs/design/observability-surfaces.md` §8 item 6). Gated inside the `/rpc/modules` bearer-token allowlist when `--modules-token-env` is configured; open on the default loopback-only daemon |
| `POST` | `/reload` | Reload the store (PlatformStore re-scan in platform mode) |
| `GET` | `/`, `/fleet`, `/projects/{svc}`, `/impact`, `/sessions`, `/sessions/{id}`, `/brief`, `/wiring`, `/statusz`, `/search`, `/operate`, `/decide` | The Observatory console (see below): every page on one shell; `?format=json` returns the page's envelope (`fathom.console.page/v1`) |
| `GET` | `/observatory`, `/observatory/service`, `/observatory/impact`, `/observatory/story`, `/observatory/brief`, `/observatory/session`, `/observatory/wiring/` | Legacy routes (OC-7): an HTML request is a **308** to the page's new home; `?format=json` still returns the old JSON unchanged (`observatoryModel` v1 and its per-page slices) |
| `POST` | `/observatory/wiring/preview` · `apply` · `undo` | Law-15 wiring through `internal/wiring`: `preview` returns the plan + `plan_digest`; `apply` requires it (409 if the repo changed since preview) and returns a `receipt_digest`; `undo` requires that digest. All three refuse non-loopback `Host`, cross-origin `Origin`, unknown service ids, and paths that are not registered service roots (`requireLocalMutation`); a refusal, and a `GET` of any of the three, renders on the console shell with the reason and a way back to `/wiring` |

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
| `--store` | `<repo>/workspace/db/fathom.sqlite` (see §CLI subcommands) | store path, .sqlite/.db (language-agnostic source) |
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
| `--store` | `<repo>/workspace/db/fathom.sqlite` (see §CLI subcommands) | store path (.sqlite/.db) |
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
| `--store` | `<repo>/workspace/db/fathom.sqlite` (see §CLI subcommands) | store path (.sqlite/.db) |
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
fathom session-hook [--url http://localhost:7575] [--dir <project-dir>] [--event instructions-loaded]
```

| Flag | Default | Description |
|---|---|---|
| `--url` | `http://localhost:7575` | fathom `serve-http` base URL |
| `--dir` | `$CLAUDE_PROJECT_DIR`, else cwd | Project dir to resolve the service from |
| `--event` | off | Record a harness event instead of priming the session. `instructions-loaded` (the **InstructionsLoaded** hook, PC-23) writes one silent `instructions-loaded/loaded` beat `{file_path, memory_type, load_reason, session}` to `~/.fathom/hooks.jsonl` and prints nothing — the harness-side evidence `fathom doctor`'s `stanza_loaded` row reads. Never counts as a session start |

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

**Enforcement (ADR-038).** A `.fathom/guard.cue` guard or `layer_rules` entry marked
`enforce: true` DENIES a violating PreToolUse `Edit`/`Write`/`MultiEdit` or `Bash` call
outright — the tool call never runs, and Claude Code/Codex show a one-line reason (the rule,
its citation, the remedy) in place of it — instead of only advising after the fact. Loading an
`enforce: true` rule with no deterministic check behind it (a note-only guard, or a layer rule
of kind `no_cycles`/`cross_service_direction`) is refused, naming it. `FATHOM_ENFORCE=off`, or
`[guards] enforce = false` in `.fathom/config.toml` / `~/.fathom/config.toml`, turns every
enforced rule in this repo back to advisory.

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

Every plan it verifies leaves a `~/.fathom/verify-ledger.jsonl` row with
`origin: stop` (PC-22); `fathom verify` rows carry `origin: cli`, and the
edit hook's claim-moment rows carry `origin: claim-moment` — the PostToolUse
`edit-hook` runs the same verify for one plan the moment an edit adds a
`[x]` to a depth-1 `docs/plans/*.md` and injects the refusal (silent on a
clean claim or any operational failure).

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
  [--concern <id>] [--json | --format text|json|sarif] [--quiet] [--quiet-pass] \
  [--mutation <report>] [--strict-mutation] [--since <ref>]
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
| `--mutation` | `""` (falls back to `$FATHOM_MUTATION_REPORT`) | VB-34: a mutation-testing report (go-mutesting log output, or fathom's own JSON shape — see `docs/use-cases/test-coverage.md`); a claimed row whose symbol is `test_reaches`-covered but has mutation score 0 gets a `covered-not-asserted` note. `--store`/direct mode only — HTTP mode errors (exit 2) if given |
| `--strict-mutation` | `false` | Gate-fail a `covered-not-asserted` finding instead of noting it as advisory |
| `--since` | `""` | ADR-044 §1.4/CV-14 (I45): compare each `--plan` file against its version at this git ref. A claimed row whose `skip_test`/`skip`, `disposition`, `retires:`, `surface_change:`, or a raised `max:` was gained ANYWHERE inside `<ref>`..head (not merely the last edit/commit — the narrower thing no `--since` does) is reported `self-declared` and, for a gained skip, fails the gate exactly as the plain self-skip check does. Text output prints `self-declared: <keys>` under each affected row and, after the summary, `deleted since <ref> (<plan>): <ids>` for rows present at `<ref>` and missing now. Applies only inside a git work tree; a `<ref>` the plan file did not exist at yet counts as every current declaration being self-declared |

A plan row's `certified_by: <Test>` annotation (vision 33 / ADR-043 stage 1,
TC-9, `internal/verify.EvaluateCertifiedBy`) replaces the ordinary
`test_reaches`/`skip_test` requirement: the row verifies only when that test
has a `live` compiled certification (`internal/certify`) scoped to the row's
`symbol`, at or above `strength` (default `asserts`). See
[docs/plans/README.md](plans/README.md) for the full annotation grammar.

See [docs/plans/README.md](plans/README.md) for the plan grammar and
[docs/adoption/spec-driven-development.md](adoption/spec-driven-development.md) for the
full authoring-to-gate loop.

---

### `claim`

Human-judgment triage on a compiled semantic-layer claim, store-direct only (v0):

```sh
fathom claim add          --store <path> --service <id> --scope <glob> [--scope <glob> ...] --rule "..." [--remedy "..."] [--cite path#Lstart-Lend] [--json]
fathom claim refute       --store <path> --service <id> --claim <claim-id> --note "..." [--author <name>] [--dry-run] [--json]
fathom claim corroborate  --store <path> --service <id> --claim <claim-id> --note "..." [--author <name>]
```

`add` (SU-17, docs/plans/substrate.md §D) mints one content-addressed, human-authored
claim without waiting for a pattern to recur — "remember this" as a single utterance.
Producer is always `human.review`; tier is always capped at `sme-stated` — `--tier` is
parsed only to be refused (`--tier` is not a valid flag on this verb; there is no way to
raise the cap). `PromptVersion` is `human-authored/v1`, its own reconciliation domain:
distinct from native review's `review-verb/v1` and the governed import's
`external-import/v1`, so neither ever supersedes or is superseded by a `claim add` claim.
An `external_findings` row is written alongside it, exactly like a governed import
(`source id human:<sha256(rule)[:12]>`, `source version 1`); scope-fp and cites links are
written with the SAME helpers `fathom findings import` uses. Served by the pre-edit hook
like any finding. `--store`, `--service`, at least one `--scope`, and `--rule` are
required; `--remedy` and `--cite` are optional.

`refute` appends a `ClaimEvent{Verdict:"refuted"}` **and** flips the claim's status to
refuted — suppression is refutation with a note. `--dry-run` (SU-18) previews the
disposition instead of recording it: `would withhold N served row(s) across N file(s):
<first 5 files>` (JSON: `{claim_id, would_withhold, files, truncated}`), computed by
walking the claim's own scope glob(s) against the service's indexed files. Nothing is
written on a dry run; `--note` is not required with `--dry-run`. `corroborate` appends a
`ClaimEvent{Verdict:"corroborated"}` and never changes status or tier — corroboration is
provenance, not evidence-laundering. `--store`, `--service`, `--claim`, and `--note` (the
last waived by `--dry-run`) are all required; `--author` is optional.

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
fathom layers list    [--root <dir>] [--store <path> --service <id>] [--json]
fathom layers show    <name> [--root <dir>]
fathom layers install <dir>  [--root <dir>] [--force]
```

`list --json`: `{root, store_read, layers: [{name, source, overrides, kind,
tier, stratum, evaled, drift_tracked, corpus_include, load_error?, claims?,
last_compile?}], orphans?}` — `claims`/`last_compile` appear only when
`--store`/`--service` read the store (`store_read: true`); a broken project
pack is an entry with `load_error`, never dropped.

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
calling the LLM or writing anything, printing a real `cost_usd` range for the model
`--profile`/`FATHOM_COMPILE_MODEL` resolves when this binary knows a price for it, or
`cost unknown: no price for model <m>` when it does not — never a fabricated or
zero-shaped estimate.

```sh
fathom compile --service <id> [--layer obligations] [--dry-run=false] \
  [--store <path> [--profile <name>] [--force] [--unsafe-skip-eval-gate]] \
  [--url http://localhost:7575]
```

| Flag | Default | Description |
|---|---|---|
| `--service` | (required) | Service ID |
| `--layer` | `""` (the default compile set) | Layer to compile (must be defined in the manifest; `fathom layers list` shows what's visible). Empty compiles CV-41's default set — `semantic.DefaultLayersFor`: `obligations` always, plus `adr-rules`/`spec-bindings` when their own corpus matches at least one file under the service's root — one layer at a time, direct `--store` mode only. HTTP mode has no plumbing to resolve the service's root from the CLI, so an empty `--layer` there still compiles `obligations` alone, unchanged from before this row. |
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
fathom instrument --adoption [--service <id>] [--call-log <path>] [--store <path>] [--root <dir>] [--url U] [--json]
```

| Flag | Default | Description |
|---|---|---|
| `--log` | `~/.fathom/hooks.jsonl` | Hooks beat log (JSONL) |
| `--window` | `168h` (7 days) | Coverage window, measured back from now |
| `--adoption` | off | The flywheel gauge: adoption report from the MCP call log — read-error classification, a surface-scoped zero-call list (tools unused on the agent surface vs not exposed there) |
| `--service` | all | (`--adoption`) scope to one service's call-log rows; unattributed rows are excluded and counted |
| `--call-log` | `[daemon].call_log`, else `<repo>/workspace/logs/mcp-calls.jsonl` (see §CLI subcommands) | (`--adoption`) MCP tool-call log |
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

#### `report evidence` — per-service evidence density (CV-39, ADR-044 §5)

```sh
fathom report evidence [--service <id>] [--store DB] [--json]
```

Store-direct, never the daemon. `--store` defaults to `[daemon].store` in
config.toml, else the workspace default (`workspace/db/fathom.sqlite`); the
report names the store it read (`store` in JSON, the first text line). A
store that does not exist, indexes no service, or does not index the named
`--service` is refused (exit 1) naming the store — never an empty file
created on the spot and read back as a row of zeros. `--service` reports
one service; omitted, every service the store's observations table knows
about gets its own row, in `ListServiceIDs`' own alphabetical id order. For
each service:

- **typed share of call edges** — the store's own typed edges
  (`internal/store.TypedEdges`), split by `core.EdgeConfidenceOf` into
  resolved vs. heuristic, as a count and a ratio;
- **languages touched**, each with its edge tier (`typed`/`scip`/
  `file-resolved`/`syntax`, `internal/conserve.TiersByLanguage`) — read from
  the store's own indexed file paths, never a re-read of the corpus, so a
  service whose recorded root no longer exists on this machine still gets
  an honest breakdown;
- **certificate density** — live over declared targets
  (`internal/certify.Gaps`'s own summary), `0` when nothing is declared;
- **inventory item counts by kind** — `internal/analysis/inventory.Extract`
  over the service's own corpus (`api:`/`json:`/`route:`/`env:`/`rpc:`/
  `cli:`/configured-pattern keys), best-effort empty when the corpus is not
  readable from here;
- **whether receipts re-derive** (Go only) — samples the alphabetically
  first non-test Go file's first declaration, builds a
  `symbol_location` receipt against the corpus root's current HEAD, and
  re-derives it from a bare git checkout with `go/parser` alone
  (`internal/receiptverify.Verify`, VB-20): `"live"` when it reproduces,
  `"difference: …"` when it does not, `"unproven: …"`/`"not-applicable:
  …"` when the check could not even be attempted (no corpus root recorded,
  no Go sources, no `go.mod`, not a git checkout) — never defaulted to
  either true or false.

It ranks NO languages: neither the services themselves nor the per-service
language breakdown are ever reordered by how good a language's edge tier
is — both stay in their own alphabetical order regardless of which
service or language happens to have the best tooling support.

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
  [--harness claude-code|codex|gemini-cli|cursor|copilot|agents|none] \
  [--skills[=claude|codex|agents|all|codex-prompts]] [--seed-user-ignore] [--json]
```

| Flag | Default | Description |
|---|---|---|
| `--path` | cwd | Repo root to wire |
| `--service` | resolved from `--path` | Service ID — the same identity the hooks use |
| `--url` | `http://localhost:7575` | fathom `serve-http` base URL |
| `--bin` | `os.Executable()`, symlink-resolved | fathom binary path baked into the hook commands |
| `--harness` | `claude-code` | Which registry descriptor's vehicles to write (ADR-029); `none` = stanza + `.fathom/` only. Prints the descriptor's own `mcp add` line, never runs it |
| `--skills` | omit = skip | Install the nine shipped agent skills. Bare `--skills` (= `all`) writes `.agents/skills/` (the Agent Skills standard path, spec-only frontmatter) **and** `.claude/skills/`; `codex` and `agents` write `.agents/skills/`; `claude` writes `.claude/skills/`; `codex-prompts` writes the deprecated user-global `~/.codex/prompts/` twin — the only user-global write, explicit only |
| `--seed-user-ignore` | off | Seed `~/.fathom/ignore` with machine-junk defaults; prints exactly what it writes |
| `--harness` | `claude-code` | Which registry descriptor's vehicles to write (ADR-029): `claude-code` (hooks into `.claude/settings.json`), `codex` (hooks into the repo's `.codex/hooks.json`, run after `/hooks` trust), `gemini-cli` (hooks into the repo's `.gemini/settings.json`, ms timeouts), `cursor` / `copilot` (data rows: stanza + skills, no hook file), `agents` (AGENTS.md + MCP), `none` (stanza + `.fathom/` only). Non-Claude descriptors are *unverified on this machine*; every one prints its own `mcp add` line and never writes a user-global file; a descriptor 60 days unverified after its codec landed is refused |
| `--json` | off | Emit the run as JSON (below) instead of the narration |

**`--json` envelope.** `{service, root, exit_code, writes: [{path, action:
created|updated|ok|shim, detail}], blocked: [{path, action: "blocked",
detail}], receipt: {id, time, files} | null, manual_steps: [{text,
command?}], narration: [...], stderr: [...]}` — every write init narrates,
the receipt the wiring engine appended during the run (`fathom fleet undo
<id>` reverses it), and every `MANUAL:` step as data with its runnable line
in `command` where there is one. Exit code is init's own.

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

**Stanza line budget.** When `.fathom/concerns.cue`'s `max_lines` concern governs a stanza
file, `init`/`reconcile --apply` trim the stanza body — dropping the opening-moves worked
example first, then the continuity paragraph — to the smallest cut that fits every budgeted
file, applied identically to CLAUDE.md and AGENTS.md so the two never diverge; a trim
prints the note naming which tier it dropped. **This repo's own `CLAUDE.md` sits at exactly
its 120-line budget right now** — the next line of growth here drops the opening-moves
table first, before anything else in the stanza gives way.

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
fathom doctor [--harness claude|codex|gemini-cli|cursor|copilot|agents|auto] [--path <dir>] [--service <id>] [--url <base>] [--json]
```

| Flag | Default | Description |
|---|---|---|
| `--harness` | `auto` | `claude`, `codex`, any other registry id (`fathom harness list`), or `auto` (detects from each descriptor's presence markers in registry order — `.claude/settings.json`/`~/.claude.json`, `.codex/config.toml`, `.gemini/settings.json`, … — defaulting to `claude` with no evidence). A non-Claude harness adds the `harness_hooks` row, which reads the descriptor's hook files and reports `skipped: unverified` — never PASS |
| `--path` | cwd | Repo root to diagnose |
| `--service` | resolved from `--path` | Service ID — the same identity `fathom init` resolves |
| `--url` | `http://localhost:7575` | fathom `serve-http` base URL |
| `--json` | off | Machine-readable report instead of the human table |

The founding eleven checks: `mcp_registration`, `daemon_reachable`, `binary_fresh` (the running
binary's own build vs `git rev-parse HEAD` of its own source tree — the LOCAL binary only,
and the row says so; a dirty build surfaces `vcs.modified`, and both linked-worktree
caveats — mismatch inside one, stamp-poisoning by ones nested under the checkout — are
printed only when verified to apply), `daemon_fresh` (PC-4: the daemon's `/statusz`
`binary_commit` against this binary's own stamp — PASS on the same revision, FAIL naming
both, SKIP with the reason for a daemon that predates `/statusz` schema 2 or an
unstamped binary; never compared to the repo's HEAD), `hooks_installed` (any settings scope: project `settings.json`,
`settings.local.json`, or user `~/.claude/settings.json`; when nothing readable wires
them but the call log shows them firing, the verdict names that contradiction instead
of a duplicate-installing remedy) / `hooks_firing` (both Claude Code only — Codex
reports `skipped`, honestly, since it has no hook mechanism), `lefthook_gates`
(harness-agnostic), `stanza_present`, `stanza_loaded` (the harness-side twin
of `stanza_present`, PC-25: PASS when an `InstructionsLoaded` beat names this
repo's `CLAUDE.md`/`AGENTS.md`, SKIP naming the wiring date when none does —
never FAIL), `skills_installed`, `wiring_drift` (PC-29: hooks,
stanza regions, each harness's MCP server entry — argv and env/header NAMES, never values —
and `fathom:owned` skill files against the digests in `.fathom/owned.json`; PASS at zero
drift naming any entry without a baseline yet, FAIL naming each drift by kind and path,
SKIP when a file could not be read), `config_valid`,
`obligations_served` (service resolved the way the hooks resolve it — every candidate
identity intersected with the daemon's service list, with a distinct not-indexed
verdict and drift-flagged rules counted into the reason), `surface_traffic`
(process-lifetime counters — "since this daemon process started"). Later rows:
`capabilities` (red only when something switched on is failing to deliver), `concerns`
(the custodian — a load error or a violation is the only red), `user_scope_ignore`,
`user_scope_guard`, `guard_file` / `guard_dead_scopes` / `guard_include_modules_unbound`,
`binding_file` / `binding_modules_mounted` / `binding_module_readiness` (SKIP names the real
reason when `list_modules` is not registered: no `--module` store was mounted), `binary_on_path`,
`channels_env`, `ident_cache_env`, `metrics_tier`, `session_ledger_tokens`,
`cpp_toolchain` (CMake repos: `compile_commands.json` present), and
`decision_provider` (ADR-042, docs/vision/32-decision-port.md: `[decide] provider =
none` is `skipped` naming it off; `llm` pings the configured profile lane and, when
`decisions.jsonl` exists, names its last recorded `latency_ms`; any other value is a
`fail`). The stanza check accepts both the v1 and the v3 (`sha256:`-digested) begin
marker.

Exit `0` every check passed; `1` at least one check failed; `2` doctor itself could not
run (bad flags, unresolvable `--path`) — an operational error, not a verdict on the
repo's wiring. See the `fathom-doctor` skill for interpretation, the remedy-approval
discipline, and the tie-in/latency-budget checks doctor itself doesn't run.

### `plans` — computed SDD progress and archival

```sh
fathom plans status  --service <svc> [--plan P]... [--store DB | --url U] [--json]   # per-plan rollup: total/proven/claimed-unproven/measured-stale/open/deferred/other-skipped
fathom plans next    --service <svc> [--eligible-only] [--free] [--root DIR] [--json] # the ordered work frontier, with {id, symbol, package} anchors
fathom plans context --service <svc> --row <id> [--budget 4000]                       # render a row's ctx: context pack (elided beyond the byte budget)
fathom plans lease   --plan P <row-id> [--holder H] [--ttl 2h] [--release] [--root DIR] # TTL advisory lease on a row (.fathom/leases.jsonl)
fathom plans leases  [--root DIR] [--json]                                            # list live leases
fathom plans archive <plan> --service <svc> [--root DIR]   # retire a fully-proven plan to docs/plans/archive/ (out of gate scope)
fathom plans restore <plan> [--root DIR]                   # bring it back
```

State is always derived from the index by the verify engine, never asserted.
`measured-stale` (PC-5, ADR-027): when the daemon that answered is not this
binary's build (or, inside the fathom checkout, not HEAD), a claimed row it did
not prove is a coverage gap in that column — never `claimed-unproven` — and a
footer names why; `fathom intent audit` and doctor's `unproven_claims` apply
the same rule to the verify ledger's newest reading per plan.
`next --eligible-only` hides rows blocked on an unmet `needs:` target (the
derived `eligible` / `blocked_on` state); `--free` hides rows holding a fresh
lease. `archive` refuses unless every row id is lexically absorbed into
`CHANGELOG.md`, `ADR.md`, or `docs/changelog/*.md` (the per-release
narrative) — the refusal names all three. Heads-up: `plans status` currently
verifies every active plan serially over HTTP (minutes on a large plan set —
O29 in `docs/dogfood-observations.md` tracks the fix).

### `thread` — continuity across sessions (vision 21)

```sh
fathom thread brief   [--budget-bytes 2400] [--repo .] [--source startup|clear|resume|compact|fork] [--all-repos] [--json]
                                               # what a fresh session gets injected (startup/clear only; resume/compact get nothing)
                                               # --json: {kind:"brief", source, budget_bytes, bytes, text} — text is "" on resume/compact
fathom thread note    "text" [--thread T] [--kind K] [--completes-when …] [--repo .]   # file a handoff BEFORE /clear
fathom thread point   "what happened" [--session S] [--thread T] [--repo .]          # side-task pointer
fathom thread list    [--all-repos] [--all] [--thread T] [--repo .] [--json]   # this repo's open items; --all-repos: every repo; --all: also closed + dormant, every repo
                                               # --json: {kind:"list", items:[{id,status,kind,thread,text,repo,session_id,filed_at,closed_at,closed_by,commit,pointers}], hidden_other_repos}
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
fathom config audit [--repo .] [--json] [--format text|json|sarif] [--hook [--harness claude-code]]
fathom config audit [--repo .] --code-facts [--json] [--strict]
```

`--hook` is the harness **ConfigChange** hook (PC-24): it reads the event
envelope from stdin, audits the changed repo, writes one
`custodian/config-change` beat `{source, file_path, result, violated}` and
exits `0` always — never a block (a block on that event surfaces no message
anywhere in the harness); the next session's attention line carries the
violated count.

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

`--code-facts` (VB-28, *experimental*, **off by default**): audit
code-fact-shaped sentences across every auto-loaded file — CLAUDE.md,
AGENTS.md, `.cursor/rules/*`, `.windsurf/rules/*`, the Cline memory bank
(`memory-bank/*.md`), and Claude Code's per-project auto-memory
(`~/.claude/projects/<encoded-root>/memory/*.md`) — classifying each as
`anchored-fresh`, `anchored-drifted` (with a receipt naming what the tree
says now), `unanchorable-code-claim`, or `preference`. Advisory: exits `0`
regardless of findings unless `--strict` is also given, in which case any
`anchored-drifted` sentence exits `1`. A file this cannot read is named on
stderr and the run reports itself as a lower bound, never a silent clean
bill. Stays off by default until precision holds ≥ 0.8 on a real labelled
set (VB-29, `docs/vision/22-config-custodian.md` §3c).

### `daemon` — install, inspect and remove the serve-http agent

```sh
fathom daemon install [--label dev.fathom.serve-http] [--store PATH] [--addr :7575] [--bin PATH] [--force]
fathom daemon status  [--url http://localhost:7575] [--json]
fathom daemon uninstall [--label NAME]
```

`status --json`: `{platform, label?, plist?, url, reachable, detail?,
remedy?, exit_code}` — exit 1 when not reachable, same as the text path.

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
fathom fleet reconcile [--repo R]... [--apply [--rebaseline]] [--url U]      # dry-run by default; --apply: shim repoint + additive template merge + stanza + baseline digests
fathom fleet receipts  [--repo R] [--limit 20] [--json]                      # every write fathom makes in a repo is receipted…
fathom fleet undo      [--last | <receipt-id>] [--repo R] [--json]           # …and reversible
fathom fleet remedy    [--dir ~/.fathom/fleet] [--json]                      # time-to-remedy per repository from the report snapshots (SH-9)
```

`remedy` grades every dated `report-<date>.json` snapshot with the fleet page's
wiring grade: a gap opens at the first snapshot grading a repository below A and
closes at the first later one grading it A again. A gap already in the first
snapshot opened on or before it (its interval is a lower bound); a snapshot that
could not read the call log grades that repository unknown, never unused. The
2026-08-08 baseline (9 of 17 below A, unbounded) is recorded beside the median.

`reconcile --apply` writes stanza regions and hooks only through
`internal/wiring` — the same plan → apply → receipt path as `init`, with the
same law-15 refusal for a hand-edited tracked region (blocked and named;
other files still applied). The same ledger (`.fathom/owned.json`) baselines
each harness descriptor's MCP server entry (argv and env/header NAMES, never
values) and every `fathom:owned` skill file: dry-run prints a `DRIFT:` line by
kind and path for a changed entry and never merges it; `--apply` records a
baseline for entries that have none, and `--apply --rebaseline` re-records a
drifted one when the change is yours. Dry-run distinguishes `stale` from `up to date
(will be re-marked with a digest on apply; no prose changes)` for a v1
region awaiting its digest.

### `decide` — the decision port's log

```sh
fathom decide log [--site S] [--page N] [--per-page 50] [--readings DIR] [--call-log P] [--url U] [--json]
```

The provider and its sites (`[decide]`), the ruling log — `decisions.jsonl`
beside the resolved call log: site, kind, choice, confidence, latency, applied,
and a failed call's error — newest first, and the table of the newest
`docs/bench/readings/decision-port-calibration-*.md` under `--readings` or any
registered service's root (parsed, never recomputed: the log carries no gold
labels). A missing log or reading is named, never zero.

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

### `review` — the change-review verb

```sh
fathom review --service <id> (--since <git-ref> | --manifest FILE) [--store DB] [--mode report|persist] [--no-persist] [--idempotency-key K] [--expect-index-watermark W] [--budget-seconds N] [--call-log P] [--json] [--format text|github] [--out FILE]
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
affected_tests, test_gap}`), identical to `review_diff`'s, folded into a
versioned `ReviewResponse` envelope (FL-16, `docs/plans/findings-loop.md`):
`schema_version`, `producer{name,version,build}`, `inputs{manifest_digest,
source_watermark, index_watermark, rules_digest, plans_digest,
extractor_versions}`, the unchanged `dimensions[]`/`summary`/
`findings_persisted`, `coverage{complete, unknown}` (false the moment any
dimension is unchecked, or the run is truncated/stale/cancelled),
`truncated`/`stale_index`/`cancelled`, and `unanchored_paths` — a changed
path the index carries no symbol for (a doc, a workflow file) is accepted
as a valid process subject and listed here rather than given an invented
anchor.

Exactly one of `--since <ref>` or `--manifest FILE` (a JSON
`[{"path","sha256"}]` array) selects the changed-file set; with `--manifest`
every path's CURRENT content must hash to its declared sha256 or the run
refuses with `manifest mismatch: <path>` and exit 2 — never a silent
re-read.

`--since` (ADR-044 §1.6, CV-13) additionally resolves `<ref>` to a commit
sha and carries it, plus the service's own corpus root, on the review
engine's internal Input (`Base`/`Root`). This feeds the **`integrity`**
dimension (ADR-044 §1, CV-12): without `--since`, `[ ] integrity` prints
with reason `needs a base ref`; with it, `fathom review` runs
internal/conserve's `integrity` plane over base..head and reports any
guarded-test loss (a certifying or invariant-enforcing test that was
deleted, renamed, weakened, newly skipped, had its referenced golden
fixture change, or lost/moved its `// certifies:` marker) or a hand edit
to a generated file whose generator inputs did not change. The
**`conservation`** dimension (ADR-044 §2, CV-24) reads the same
`Base`/`Root` and reports the same "needs a base ref" reason without
`--since`; with it, `fathom review` (and `review_diff`) build a real
`internal/conserve.Input` via `conserve.NewInput` and run it through
`conserve.Run` over every registered plane (`conserve.Planes()`) — the
same composition `fathom conserve` itself uses — so `[x] conservation`
(`checked: true`) reports the same planes/findings `fathom conserve
--since <ref>` would over the identical range, with no separate command
needed. A ref that does not resolve fails open (every
dimension still runs over the changed-file set `--since` already
computed); `--manifest` runs never populate `Base` at all, since they
carry no ref.

`--mode persist` (the default) writes the deterministic-oracle findings to
the claims store; `--mode report` (`--no-persist` is an alias) never
writes anything — pinned by a store-state equality test including the
error path. `--idempotency-key K` (persist mode only) records the response
in an additive `review_runs` table: a repeated key over the SAME request
(service + mode + changed-file set) replays the recorded response and
writes nothing else; the same key over a DIFFERENT request refuses with
`idempotency key reused with a different payload` and exit 2.
`--expect-index-watermark W` compares W against the index's own watermark;
a mismatch sets `stale_index: true` (every dimension still runs — a report,
never a hard refusal) and always refuses persist regardless of `--mode`.
`--budget-seconds` is advisory — on excess it prints a named skip and exits
0; it requires `--mode report`/`--no-persist` (a bounded run must never be
a store writer).

`--format github` (VB-22, `docs/plans/verbs.md`) renders ONE deterministic
Markdown PR comment instead of the human summary or `--json`: changed
symbols with their callers out to depth 2 (review-recall-loop mechanism
(a), `docs/bench/readings/review-recall-loop-2026-09-18.md`) and each
caller's resolution tier plus depth (`[resolved, depth 1]`/`[heuristic,
depth 2]`), call-site completeness (DIRECT callers found vs. how many live
in this same diff — never inflated by a depth-2 row), governing rules with
cited lines, a
`### Verify` line naming `fathom verify --service … --plan …` whenever the
diff touches a `docs/plans/*.md` row's anchored symbol, affected tests and
the test gap, and a coverage block when a profile was supplied — every
finding line carries a stable claim id, and NO model prose appears anywhere
on this path (`internal/analysis/review/github.go`'s `renderGitHubReview`).
Opens with the fixed marker `<!-- fathom-review -->` so a caller can find
and update its own prior comment. `--out FILE` writes the report (any
format) to `FILE` instead of stdout — stdout then carries only a one-line
confirmation — for a CI step that captures the comment body and hands it to
a separate upsert step; see `docs/adoption/github-action.md` (VB-23) for the
full GitHub Action and the required-check wiring.

### `review-key` — one reuse-eligibility token (FL-17/FL-18/FL-19)

```sh
fathom review-key --service <id> (--since <git-ref> | --manifest FILE) [--store DB] [--prior KEY.json] [--json]
```

Computes one sha256 token (`internal/analysis/reviewkey`) over every input a
`fathom review` verdict depends on, so a caller (typically an external
review kit) can decide whether a prior review result is still safe to reuse
without a second bespoke hashing API:

- `manifest` — the changed-path digest set (from `--since`'s git diff,
  hashed off the working tree, or `--manifest FILE`'s own declared
  digests).
- `closure` — the typed import/call closure of the changed files, walked
  forward over the SAME typed call graph `blast_radius`/`diff_impact` use
  (never a new import parser), digested by each reached file's store
  fingerprint.
- `rules` — the project's `.fathom/rules/archconformance.cue` +
  `.fathom/guard.cue` + any project-local `.fathom/layers/*/layer.cue`.
- `plans` — every file under `docs/plans/`.
- `tooling` — fathom's own version identity.
- `index` — the service's connector watermark + store watermark.

```json
{ "schema_version": 1,
  "key": "<sha256>",
  "complete": false,
  "unknown": ["dependency closure: pkg/pkg has no typed edges"],
  "components": {
    "manifest": "<sha256>", "closure": "<sha256>", "rules": "<sha256>",
    "plans": "<sha256>", "tooling": "<sha256>", "index": "<sha256>" }
}
```

A component whose input could not be verified (an unreadable manifest path,
a changed package the typed pass never analyzed) never hashes as empty —
it is excluded from that component's own digest and named in `unknown[]`,
and `complete` is false; the key still computes over whatever WAS
resolved, so a caller may reuse narrower results or refuse — fathom only
reports. `--prior KEY.json` diffs a prior key against this run and adds
`changed_since_prior: [component, …]`. Identical inputs produce an
identical key across runs and machines. `pkg/client.ReviewKey` is the same
code path this CLI calls — a Go embedder gets byte-identical answers.

### `conserve` — the refactor-conservation report (ADR-044, CV-19)

```sh
fathom conserve --since <git-ref> --service <id> [--store DB] [--json] [--strict] [--plan FILE ...] [--residue-max N] [--coverage-meta FILE]
```

Runs every registered `internal/conserve.Plane` (`surface`, `certificates`,
`governance`, plus `integrity`/`absence` once landed) over base (`--since
<ref>`) versus head (the working tree, uncommitted edits included) and
reports each plane's verdict: `conserved` (checked, nothing undeclared
lost), `losses` (at least one undeclared finding), or `unproven` (an
input the plane needed was missing — never `conserved` from a missing
input, I44). The `surface` plane's own findings are the capability
inventory's base→head delta (`internal/analysis/inventory`: exported Go
declarations, JSON wire tags, routes, env keys, proto RPCs, plus any
`[[surface.pattern]]`-configured kind, `docs/configuration.md`) — a
removed or changed `route:` key additionally names any OTHER known
service whose client code still calls it (`consumer_loss`). `certificates`
reports a `// certifies:` marker declared at base and gone at head, plus
any compiled certification whose test or reach is in the changed set and
whose state is `unproven`/`broken`/`unreachable`. `governance` reports an
obligations/adr-rules/spec-bindings/findings claim whose scope glob(s)
matched a file at base and match none at head — a deliberate exception
that reads the full base/head file-path list (`git ls-tree`/`git
ls-files`), never file content, since a claim's scope can be orphaned by
files nowhere near this particular diff.

A finding is excused (its `declared` field is `"prior"`) only when a plan
row's `retires:`/`surface_change:` key already named it BEFORE the change
range started — `--plan` (repeatable) names which plan files to read those
declarations from; unset, every `docs/plans/*.md` at the repo root
(non-recursive, the same set the pre-push gate itself walks) is read. A
declaration added inside the range excuses nothing (I45) — it shows up as
`declared: "self"` instead, exactly like `fathom verify --since` renders a
self-declared row.

One kind is a deliberate exception: a `resurrection`/`moved` finding (an
ordinary, undeclared rename — design §3.3's resurrection honesty) always
carries `declared: "prior"` on the wire even though nothing was ever
declared, because "prior" is the only value that excuses a finding from
`losses` and nothing needed excusing. The text renderer and `review_diff`
never print that raw value for a `moved` finding — both say "no
declaration needed" instead, so the line never claims a declaration was
made when its own Detail says the opposite; a caller reading the JSON
directly still sees `declared: "prior"`.

```json
{"schema":"fathom.conserve/v1","service":"fathom","base":"<sha>","head":"worktree",
 "planes":[{"plane":"surface","verdict":"losses","missing":[],
   "findings":[{"plane":"surface","kind":"removed","key":"api:foo.Old",
     "detail":"func()","base":"foo.go#L3","declared":"none",
     "file":"foo.go","line":3,
     "remedy":"add `retires: api:foo.Old` to the row that removed it, or restore it"}]}],
 "tiers":[{"language":"Go","tier":"typed"},{"language":"TypeScript","tier":"file-resolved"}]}
```

Every finding also carries `file`/`line` (both `omitempty`) — the
repo-relative location of the finding's SUBJECT, at HEAD when it still
exists there, else at BASE — for a Review Kit citation that needs a real
source anchor rather than the human-readable `base`/`head` strings above.
Empty when no plane could place the subject at a location cheaply (a
`governance` claim's own scope glob names no single file, for instance);
never invented.

`--json` emits the versioned `fathom.conserve/v1` document above (the
compatibility surface: field names and verdict/declared strings are
stable); the default is a short text table, one line per plane plus its
findings and missing inputs indented beneath, and a trailing `tiers:` line
naming the edge tier (`typed`/`scip`/`file-resolved`/`syntax`) behind every
language among the changed files. `--strict` exits 1 when any plane's
verdict is `losses` (2 is reserved for an operational failure — a bad
`--since` ref, an unreadable store; 0 otherwise, --strict included, when
no plane reports losses).

#### The reading set (design §3.4, ADR-044 amended 2026-09-25, Q22)

`fathom conserve` prints a `reading set` section AFTER every plane's own
table: the changed functions in `base..head` whose behaviour no
commit-bound passing test vouches for — what a reviewer still has to open
once every plane has spoken. It is **not a plane**: it carries no verdict,
changes none of the planes' verdicts, and never affects `--strict`'s exit.

For each changed, non-test, non-generated `.go` file, every function
`added` (no function of that receiver-qualified name existed at base
anywhere in the same changed package directory) or `modified` (one did,
and the exact body text differs) is classed by walking the index's
resolved call edges back to `Test*` functions:

- `attested` — at least one reaching test's latest run passed, is bound to
  a commit, was made on a clean tree, and neither the test's file nor the
  changed function's file changed since that commit (the same O114 rule
  certificates use, applied here without a declared target);
- `unattested` — a test reaches it, but none has such a run;
- `unreached` — no test reaches it over resolved edges at all.

A moved-but-unchanged function (same body, different file in the same
package directory) is neither added nor modified, and a deleted function
is never in the reading set (the compiler proves it gone; the
resurrection/absence planes cover the rest). Items are ordered `unreached`
then `unattested`, each by resolved caller count descending then symbol —
`attested` functions are counted but never listed, since nothing there
needs a reviewer's attention. The list is capped (`--residue-max`,
default 200); the report names how many were cut. **No percentage, ratio
or threshold is ever computed (I43): counts and lists only.**

The reading set is `unproven` — classing nothing, only naming what was
missing — when the index is not at HEAD (the store's last-ingested commit
must equal git HEAD, working tree clean), the typed pass failed, or a
changed file's base content could not be read (I44). `changed` and
`unanalyzed` (every changed file that isn't Go, by path — the reading set
never extracts functions from any other language) are still reported in
that case: both are pure facts of the diff itself, computed without
touching the index, so a stale or partial store never taints them.

```json
{"changed":3,"attested":1,"unattested":1,"unreached":1,
 "items":[{"symbol":"Unreached","file":"widgets/extra.go","line":3,
   "change":"added","class":"unreached","tests":[],"callers":0},
  {"symbol":"Reached","file":"widgets/extra.go","line":9,
   "change":"modified","class":"unattested","tests":["TestReached"],"callers":1}],
 "truncated":0,"unanalyzed":[],"missing":[]}
```

Served two other places: `fathom.conserve/v1`'s additive `residue` object
(exactly the shape above), and `fathom review --since`'s `conservation`
dimension, which carries the same counts plus the first five `unreached`
items alongside its ordinary plane findings.

#### Execution evidence (CV-M13, design §3.5)

The reading set's original rule judged a function `attested` whenever a
passing, commit-bound test *statically reached* it — CV-M13
(`docs/bench/readings/cv-m13-reading-set-calibration-2026-09-25.md`)
measured that against real Go coverage and found it both too generous
(3.7% of `attested` functions never actually executed — branches no test
takes, a CLI verb switch, interface fan-out) and too strict (unreached
functions that DID execute, reached only through the standard library or a
runtime initializer, outside the first-party call graph).

`task tests:attest` now runs the suite ONCE as `go test -json
-coverpkg=./... -coverprofile=<results_dir>/coverage.out ./...` (the same
JSON output still feeds the envelope; `TESTKIT_COVER=0` turns the profile
off) and writes `<results_dir>/coverage.meta.json`:

```json
{"schema":"fathom.coverage/v1","commit":"<sha>","dirty":false,"race":false,
 "profile":"coverage.out","passed":true}
```

`fathom conserve` reads it from `--coverage-meta FILE`, or from `[tests]
results_dir/coverage.meta.json` when configured and neither is given. The
profile is used only when its meta names a commit, `dirty` is false and
`passed` is true (I44) — an unbound, dirty or failing-run profile is never
used, and the report names why (`coverage_unusable`). A profile whose file
keys map for some modules but not others (a second module's results left in
`results_dir`, a module renamed since the run) is still used, and the report
counts the unplaced keys with a sample (`coverage_unmapped`,
`coverage_unmapped_sample`): functions in those files have no execution
evidence and fall back to static reach. Per function, the
profile also counts only when neither the function's own file nor any
reaching test's file changed since that bound commit (the same O114 rule
certificates use) — a function whose profile has gone stale that way falls
back to static reach on its own, without disturbing any other item.

With a usable profile, a changed function's coverage blocks inside its own
span are mapped from the profile (`LoadCoverage`, module-path-aware,
duplicate blocks across a `-coverpkg` run's per-package sections merged by
MAXIMUM count) and its *changed blocks* — every block, for an added
function; only the blocks overlapping a `git diff -U0 base..head`-changed
line, for a modified one — decide the class purely from execution. A
function counts as modified only when its code tokens changed: a comment,
blank line or reflowed line inside the body (like a doc comment above it)
leaves it out of the reading set entirely.

- `attested` — every changed block executed (a modified function whose
  changed lines hold no block, such as a signature-only edit, clears);
- `partial` — the function executed (some block of it ran, changed or
  not), but some changed block never ran (`unexecuted_lines` names those
  ranges);
- `unexecuted` — no block of the function executed at all.

Without a usable profile — globally, or for one specific item — the
original static classes (`attested`/`unattested`/`unreached`) stand for
that item. Every item's `evidence` field names which rule classed it:
`"executed"` or `"static-reach"`. Items list every non-`attested` class,
in order `unexecuted`, `unreached`, `partial`, `unattested`, each by
resolved caller count then symbol — still counts and lists only (I43), no
percentage. `unanalyzed_functions` counts a changed function whose own
file the index never analyzed at all (vendor/, an ignore rule), so
`changed` always equals the sum of every class plus that count
(reconciliation, CV-71). Test-case identity: a case id carrying its own
package (`fathom tests ingest`'s join, `JoinCasesToSymbols`) disambiguates
a bare `Test*` name shared across packages, which used to leave both
unjoined (CV-M13's control measurement: 34 of 566 cases).

```json
{"schema":"fathom.coverage/v1", "...": "...",
 "residue":{"changed":2,"attested":0,"unattested":0,"unreached":0,
   "partial":1,"unexecuted":1,"unanalyzed_functions":0,
   "items":[
     {"symbol":"Half","file":"widgets/extra.go","line":9,"change":"modified",
      "class":"partial","tests":[],"callers":0,"evidence":"executed",
      "unexecuted_lines":["12-14"]},
     {"symbol":"Dead","file":"widgets/extra.go","line":20,"change":"added",
      "class":"unexecuted","tests":[],"callers":0,"evidence":"executed"}],
   "truncated":0,"unanalyzed":[],"missing":[],"coverage_unusable":""}}
```

The decision port has no role (deterministic execution evidence, ADR-044
decision 8's "never safety").

### `refactor` — refactors as programs (CV-33/CV-34, ADR-044 §4)

```sh
fathom refactor plan <symbol|package|import> [--out FILE] [--service <id>] [--store DB]
fathom refactor status --plan FILE [--json]
```

`refactor plan` writes a gate-ready, grammar-v2 plan for retiring ONE
target — a symbol name, an import path, or a repo-relative package
directory (converted to its import path via `go.mod`'s own `module` line).
The target may be written before, after, or between the flags; unlike the
rest of this CLI (`fathom receipt verify`'s own doc note above: Go's `flag`
package normally stops at the first bare argument), this command
recognizes its own flag names wherever they fall so the documented
`<target> [--out FILE] …` order works exactly as written. Store-direct,
reusing the same `internal/analysis/callgraph` queries `rename_impact`/
`safe_delete_check` already answer with for a resolved symbol, and a plain
Go import scan (`go/parser`, `ImportsOnly`) for an import/package target.
The generated plan carries, in order:

1. a `Baseline: <HEAD sha>` header line (`docs/plans/README.md`'s
   plan-level pin);
2. one **ratcheted `gone: pattern` row per package that still references
   the target today** — grouped from typed, resolved caller edges for a
   symbol target, or from files that still import the path for an
   import/package target — each `max:` seeded at today's own lexical
   match count scoped to that package via `paths:` (so the row already
   passes once `fathom verify --store` checks it — `cliEvidence`'s
   `GoneEvidence` adapter);
3. a **`certified_by:` row per live-certified test** `TestImpact` says
   reaches the target (symbol targets only), so the refactor cannot
   silently break a certified behaviour while it proceeds;
4. the **final, unscoped absence row** — `gone: callers <Symbol>
   {package: <its own package>}` for a symbol target, `gone: import
   <path>` for an import/package target — proving the WHOLE corpus, not
   just the packages seen today;
5. a closing instruction line naming `fathom conserve --since <baseline>
   --strict`.

A ratchet's `max:` must never come from a partial read: when the corpus
cannot be fully read (a locked directory, a fetch error), `refactor plan`
refuses outright and names what could not be read, rather than seeding
`max: 0` for a package its own typed call graph may already show has live
references today. It also refuses on a DIRTY working tree: `max:` is
seeded from live disk while `Baseline:` is the last commit, so an
uncommitted edit would seed a `max:` that disagrees with what `refactor
status` later reads for that very same baseline sha (`git show`) —
commit or stash first.

`refactor status` reads one such plan's ratchet rows (any `gone: pattern`
row with `max:` set — hand-written or `refactor plan`-generated) and
reports, per row, its live count against `max:`, its count AT the plan's
own `Baseline:` commit (`git show`/`git ls-tree`, never a second ingest —
P19: cost scales with the row's own `paths:`, not the repo), and **drift**:
`max(0, current-baseline)`, references that landed on the branch since the
baseline. A plan with no `Baseline:` header is refused by name, never
silently reported as zero rows.

```json
{"schema":"fathom.refactor.status/v1","plan":"docs/plans/retire-old.md",
 "baseline":"<sha>",
 "ratchets":[{"id":"old-g1","pattern":"\\bOldFunc\\b","paths":["internal/caller/"],
   "max":3,"current":4,"baseline":3,"drift":1,"over_max":true}]}
```

---

### `receipt` — a stranger's re-derivation (VB-20)

```sh
fathom receipt verify [--repo <path>] <receipt.json>
```

Flags precede the positional argument (Go's `flag` package stops parsing
flags at the first bare argument, the same convention `fleet undo` already
uses) — `--repo` before `<receipt.json>`, never after.

Re-derives a saved agent-surface answer's claim from a bare checkout of the
receipt's own `index_commit` (the envelope field VB-19 adds to `find_callers`,
`references`, `blast_radius`, `symbol_context`, `safe_delete_check`,
`rename_impact`, `test_impact` and `review_diff`) using
`golang.org/x/tools/go/packages` and `git` alone — it never opens `internal/
mcp`, a fathom store, or the daemon, so a stranger with the Go toolchain and
this repository's git history gets the same verdict fathom's own gate does
(ADR-034 receipts; ADR-035 decision 4, `docs/vision/29-verbs-not-lookups.md`
§3 wave E).

A receipt is a small JSON file the caller saves from an answer's envelope —
`kind` (`callers` | `references` | `symbol_location` | `certification`), the
queried `symbol` (bare name or `Recv.Method` — go/types' own vocabulary),
`index_commit`, and the facts to check (`callers`, `files`, `file`/`line`,
or `test`/`reached` for `certification`):

```json
{
  "kind": "callers",
  "tool": "find_callers",
  "symbol": "validateCharge",
  "index_commit": "a1b2c3d...",
  "callers": ["Charge"]
}
```

`--repo` names the git checkout to `git archive` from (default: the current
directory). Exit codes: **0** the claim reproduced; **1** it did not — stdout
names the difference (a missing/extra caller, a file no longer mentioning the
symbol, a moved definition line); **2** it could not run at all — no git
repository at `--repo`, the commit is missing, the checkout has no `go.mod`,
or the receipt file is missing/invalid — an operational refusal, never a
false pass or fail.

**`certification` receipts** (TC-13, vision 33 / ADR-043) carry `test` (the
certifying `Test*` function, `"dir/pkg.Name"`) and `reached` (the symbols its
certification claims that test reaches) instead of `callers`/`files`:

```json
{"kind": "certification", "symbol": "Charge", "test": "internal/billing.TestCharge_Idempotent",
 "index_commit": "a1b2c3d...", "reached": ["Charge", "validateCharge"]}
```

Reach is re-derived independently with go/types alone, exactly like the
other three kinds (`internal/receiptverify` + `internal/contextbench/publicloc`
— never `internal/certify`'s own indexed computation). The run itself is
reported as `pass: unverified (no run)` unless `--run-tests` is given, in
which case `fathom receipt verify` actually executes
`go test -run '^<Test>$' <pkg>` for that one test and reports `pass: true`
or `pass: false` — ADR-043 decision 5: a reached test is not a passing test,
so the verdict is never inferred from reach alone.

| Flag (`receipt verify`) | Default | Description |
|---|---|---|
| `--repo` | current directory | git checkout to re-derive from |
| `--run-tests` | `false` | `certification` kind only: actually run the named test instead of reporting `unverified (no run)` |

### `tests coverage` / `tests select` — coverage, mutation, and test selection

Full walkthrough: [docs/use-cases/test-coverage.md](use-cases/test-coverage.md).

```sh
fathom tests coverage --profile [layer=]<file> [--profile <more>...] \
  [--service <id>] [--store <path>] [--diff <ref>] [--mutation <report>] [--json]

fathom tests select --since <ref> [--profile [layer=]<file>...] \
  [--service <id>] [--store <path>] [--json]
```

`tests coverage` joins one or more `go test -coverprofile` files (each
labeled by test layer) to indexed symbols: a per-symbol table (one column
per layer, plus `ANY`), or `--diff <ref>` to narrow to changed symbols with
no covering test in any layer. `--mutation <report>` (VB-34) adds a
`MUTATION` score column (0.0-1.0) from a joined mutation-testing report — see
"Mutation" in the use-case doc for the two accepted report shapes.

`tests select --since <ref>` (VB-33) ranks `Test*` functions for a diff by
typed-edge reachability, corroborating statement coverage (when `--profile`
is given), and test layer (unit before integration before e2e), then emits
a `go test ./<pkg> -run '^(TestA|TestB)$'` invocation per package plus the
changed symbols no test reaches. A test that already certifies a target
whose scope intersects this diff (TC-23, vision 33 / ADR-043 stage 3) ranks
first, ahead of that reach ranking — advisory only, it never changes which
tests are selected. A replay ledger at
`workspace/artifacts/tests-select/replay.jsonl`, when present, adds a
measured predicted catch-rate — never a fabricated one.

| Flag (`tests coverage`) | Default | Description |
|---|---|---|
| `--profile` | (required, repeatable) | Go cover profile, as `[layer=]path`; a bare path defaults to layer `unit` |
| `--service` | resolved from cwd | Service ID |
| `--store` | `<repo>/workspace/db/fathom.sqlite` | store path |
| `--diff` | `""` | git ref: narrow to changed symbols with no covering test in any layer |
| `--mutation` | `""` | mutation-testing report; adds the `MUTATION` score column |
| `--json` | `false` | Emit JSON instead of a table |

| Flag (`tests select`) | Default | Description |
|---|---|---|
| `--since` | (required) | git ref: rank tests for the diff since this ref |
| `--profile` | (none, repeatable) | Go cover profile(s); enables the coverage ranking tie-break |
| `--service` | resolved from cwd | Service ID |
| `--store` | `<repo>/workspace/db/fathom.sqlite` | store path |
| `--json` | `false` | Emit JSON instead of a table |

### `tests ingest` / `tests status` — test-kit evidence (vision 33 / ADR-043 stage 0)

```sh
fathom tests ingest --manifest <dir> --results <file|dir> \
  [--service <id>] [--store <path>]

fathom tests status [--service <id>] [--store <path>] [--json]
```

`tests ingest` loads a service's declared `<dir>/tests/*.cue` manifests
(validated against the mirrored test-kit schema, `schema/testkit/`) and
attested `#TestResult` envelopes from `--results` (a single file, a
directory of `*.json` files, plain JSON, NDJSON, or a DSSE/in-toto-wrapped
envelope — the payload is decoded and its digest recorded as the run's
identity; fathom never verifies signatures itself) into the additive
`test_suites`/`test_runs`/`test_cases` store tables, joining each case to a
`Test*` symbol by exact name (`TestX/sub` joins to `TestX`). It also scans
`--manifest`'s whole tree for `// certifies: <kind> <id> [(<strength>)]`
markers above `Test*` functions, writing one declared fact per entry into
`certify_declarations` (a malformed marker is a warning naming its
file:line, never a silent skip). "no manifests found" and "no runs
ingested" print as named lines — never a silently clean run.

`tests status` prints, per declared suite, its class, tier, the last
ingested run's verdict and totals, and `joined N of M cases` — the
case-to-symbol join rate is a reported fact, never a pass/fail. A service
with no declared suites prints "no manifests found" instead of an empty
table.

fathom never runs a test itself (design doc §2): both verbs ingest
test-kit's own record. A run's `env.commit`/`env.dirty`/`env.race` (vision
34 / ADR-044 stage 1, O114) bind the run to the commit it was attested
against — the commit-blind half of `internal/certify`'s state ladder that
let a certificate read `live` after the code it certified had already
changed; an envelope with no commit (the common case for one produced
before this field existed, or by an adapter never told) leaves its
certificate `unproven`, never a silent guess. `task tests:attest` produces
a commit-bound envelope for this repository's own suite — see below.

**`task tests:attest`** re-attests this repository's own suite with a
commit-bound envelope: `go test -json ./...`, converted to a `#TestResult`
envelope by test-kit's go-test-json adapter (env `TESTKIT_GO_JSON` — the
path to a built test-kit `surveyor` binary; the task prints a clear skip,
never a failure, when it is unset), bound to `git rev-parse HEAD` / `git
status --porcelain`, and written under `[tests] results_dir`
(`docs/configuration.md`). The task never ingests the envelope itself — a
plain `fathom ingest`, or `fathom tests ingest --results
workspace/artifacts/test-results`, picks it up on its own next run.
`TESTKIT_RACE=1` adds `-race` to the run and `--race` to the envelope (off
by default — a full `-race ./...` run is slow, and the task's own job is
producing a commit-bound envelope, not deciding this repository's race
policy).

| Flag (`tests ingest`) | Default | Description |
|---|---|---|
| `--manifest` | `.` | service repo root: `tests/*.cue` live under `<dir>/tests/`, and `// certifies:` markers are scanned under `<dir>` |
| `--results` | `""` | attested `#TestResult` envelope file, or a directory of `*.json` envelopes |
| `--service` | resolved from cwd | Service ID |
| `--store` | `<repo>/workspace/db/fathom.sqlite` | store path |

| Flag (`tests status`) | Default | Description |
|---|---|---|
| `--service` | resolved from cwd | Service ID |
| `--store` | `<repo>/workspace/db/fathom.sqlite` | store path |
| `--json` | `false` | Emit JSON instead of a table |

### `certify` — the compiled certifications layer (vision 33 / ADR-043 stages 1-3)

```sh
fathom certify status [--service <id>] [--store <path>] [--json]
fathom certify gaps [--kind invariant|incident|spec|port|rule|behaviour] \
  [--service <id>] [--store <path>] [--json]
fathom certify manifest [--out FILE] [--check] [--service <id>] [--store <path>]
fathom certify orphans [--json] [--service <id>] [--store <path>]
fathom certify suggest [--limit N] [--out FILE] [--service <id>] [--store <path>]
```

`certify status` reports the compiled `certifications` layer's target/live/
gap counts (a target is a declared-or-discovered invariant, incident, spec,
rule or port) and the join rate. `certify gaps` lists every declared target
with no live certificate, one line per gap with a remedy; `--kind` narrows
to one target kind; `--json` emits the versioned `fathom.certify.gaps/v1`
shape the Review Kit adapter's `fathom-certify` parses (schema string, field
names, and sort order — kind then id — are the wire contract; do not change
them without a version bump). Each target additionally carries `source`
(`{"file":..., "line":...}`, omitempty) — where the target ITSELF is
declared: a `// certifies:` marker's own comment line, an invariant's row in
`docs/intent/invariants.md`, an incident heading in
`docs/intent/incidents.md`, or a port interface's own declaration — nil when
`Gaps` could not place it. `certify manifest` regenerates
`docs/certifications.md` from the layer; `--check` fails (exit 1) instead of
writing when the committed file differs — a hand edit to that file is a
defect. `certify orphans` (TC-22) lists `Test*` symbols that certify
nothing, reach no symbol changed in the last 180 days, and never failed or
flaked in any ingested run — a list, never an action; nothing is deleted or
flagged for deletion. `certify suggest` (TC-24) asks the decision port
(`[decide] provider = llm|typesafe`) one batched request per chunk of
unregistered `Test*` symbols — a strength choice plus an "asserts a real
behaviour" noul — and writes proceed-band answers to `--out`
(`workspace/artifacts/certify-suggestions.jsonl` by default, regenerated
fresh each run, never appended), rendered as a hint in `certify gaps`'s
table output; a suggestion is never a claim and is never written to the
store, and `provider = none` prints one line and exits 0.

| Flag (`certify gaps`) | Default | Description |
|---|---|---|
| `--kind` | `""` (all kinds) | filter to one target kind |
| `--service` | resolved from cwd | Service ID |
| `--store` | `<repo>/workspace/db/fathom.sqlite` | store path |
| `--json` | `false` | Emit the `fathom.certify.gaps/v1` JSON shape instead of a table |

| Flag (`certify manifest`) | Default | Description |
|---|---|---|
| `--out` | `docs/certifications.md` | path to write the generated artifact |
| `--check` | `false` | exit 1 when the committed file differs, without writing |
| `--service` | resolved from cwd | Service ID |
| `--store` | `<repo>/workspace/db/fathom.sqlite` | store path |

| Flag (`certify suggest`) | Default | Description |
|---|---|---|
| `--limit` | `0` (no cap) | cap the number of unregistered `Test*` symbols considered |
| `--out` | `workspace/artifacts/certify-suggestions.jsonl` | suggestions artifact path (regenerated fresh, never appended) |
| `--service` | resolved from cwd | Service ID |
| `--store` | `<repo>/workspace/db/fathom.sqlite` | store path |

`certify status` and `certify orphans` also take `--service`/`--store`
(resolved the same way) plus `--json`.

### `export` — interchange with other code-intel tooling (VB-32)

```sh
fathom export scip --service <id> [--store <path>] [-o index.scip] [--module <dir>]
```

Writes `service`'s symbol and typed-edge tables as a [SCIP](https://sourcegraph.com/blog/announcing-scip)
index, over the generated wire-format types vendored at `third_party/scip` (see that
directory's `README.md` for the exact upstream tag/commit and why only those files are
vendored rather than depending on the upstream module directly) — store-direct only,
mirroring `review`'s structure (no daemon round trip). Each symbol's resolution tier
(`core.EdgeConfidenceOf`: `resolved`/`heuristic`) travels on the CALLER's
`SymbolInformation.Documentation`, one line per typed-edge relationship, formatted
`fathom:resolution <target-symbol> = <tier>` — SCIP's `Relationship` message has no
free-form field, so this is the "Relationship-adjacent field" the tier lives in; a
definition's own occurrence carries no tier (tier is a property of an edge, never of a
definition site). An edge whose target isn't among the service's own indexed nodes is
skipped (SCIP's `ExternalSymbols` table would be the correct home for that; out of scope
here).

After writing, it round-trips: with `--module <dir>` naming a real Go module root and
`scip-go` on `PATH`, it runs `scip-go` independently over that module and compares bare
symbol names against the store's own list — a genuine cross-tool check. Otherwise (the
common case — most machines running this command don't have `scip-go` installed) it
decodes the just-written bytes with the bindings and compares the resulting symbol table
against the store's, printing a `round-trip (bindings-decode): N/N symbols matched — ok`
line and naming that the `scip-go` comparison is pending on a machine that has it.

| Flag | Default | Description |
|---|---|---|
| `--service` | (required) | Service to export |
| `--store` | `<repo>/workspace/db/fathom.sqlite` (see §CLI subcommands) | store path (.sqlite/.db) |
| `-o` | `index.scip` | Output path for the SCIP index |
| `--module` | (none) | A real Go module root on disk, for the `scip-go` round-trip comparison (ignored when `scip-go` is not on `PATH`) |

`fathom ingest --scip` — importing symbols FROM a SCIP index, rather than exporting TO
one — is a later, separate decision; not built here.

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
fathom metrics traffic       [--service S] [--by surface|tool|hour|day|caller] [--since DUR | --from … --to …] [--call-log P] [--url U] [--json]
fathom metrics channels      [--since DUR | --from … --to …] [--hooks-log P] [--json]
fathom metrics interventions [--since DUR | --from … --to …] [--store P] [--call-log P] [--hooks-log P] [--json]
fathom metrics series        <metric> --bucket hour|day [--since DUR] [--root D] [--json]
fathom metrics rollup        [--date YYYY-MM-DD] [--root D] [--store P] [--call-log P] [--hooks-log P] [--json]
fathom metrics rotate        [--call-log P] [--keep-days N] [--apply]
fathom metrics impact        [--call-log P] [--url U] [--json]
fathom metrics catches       [--kind K]... [--service S] [--days N] [--page N] [--per-page 20] [--call-log P] [--url U] [--json]
fathom metrics sessions      [--arm on|off] [--repo R] [--q TEXT] [--sort at|turns|first_edit|tokens|fathom_calls|catches] [--asc] [--page N] [--per-page 50] [--json]
fathom metrics sessions      --id SESSION [--channel C]... [--page N] [--per-page 500] [--json]
```

Design: `docs/design/observability-surfaces.md`. `traffic --by day` groups by UTC
day and `--by caller` by caller class — agent, edit hook, gate, refresh, plumbing or
unattributed, from each row's origin and surface — so plumbing never passes for
adoption. `impact`, `catches` and `sessions` print the console's Evidence projections
(vision 35, `internal/console`): the four questions with each arm as a range and "not
instrumented" instead of a zero; one timeline of verify refusals (the verify ledger),
conserve losses and integrity catches (`fathom review` rows in the call log) and guard
refusals (the edit hook's `denied` beats), each kind absent with its reason when its
instrument recorded nothing to count from; and the session table (one row per session)
or one session's replay, never more than 2,000 events per response.

### `findings` — findings as evidence (**experimental**; entry ≥15–20 resolved findings)

```sh
fathom findings near       <symbol|file> --store P --service S [--hops 2] [--include-superseded] [--as-of <RFC3339|sha>] [--json]
fathom findings status     --store P --service S [--json]
fathom findings import     --store P --service S --subject ORG/REPO/COMPONENT [--producer NAME] --file P [--dry-run] [--json]
fathom findings candidates --store P --service S [--min-sites 3] [--json]
```

`near` is the call-graph precedent lookup (mirror of the `findings_near`
MCP tool); `--as-of` (SU-19, ADR-041 decision 7) narrows to claims whose
bi-temporal validity window contains the resolved RFC3339 timestamp or
commit sha, BEFORE the superseded/refuted gate runs — "was this finding
true then", answered regardless of `--include-superseded` and regardless
of the claim's CURRENT status. `status` reports entry-condition progress
and, since the
findings loop (FL-23), `candidates`/`disagreements_by_rule` counts, plus
(SU-16) `rules_never_fired_30d` and `top_fired` — computed from the hooks
beat log every other `fathom metrics`/instrument reader shares
(`beatLogPath`/`readBeatsFor`). `candidates` derives rule candidates —
class, normalized shape, member citations, producers, code-change
resolutions, a drafted rule sentence, and the ADR-038 check family that
could enforce it — from patterns spanning at least `--min-sites` distinct
sites across native and external (review-kit-imported) findings alike.
Since SU-15, each candidate's score is sites × mean CODEOWNERS-ownership
weight of its member files (1.0 with no CODEOWNERS) × a decay applied once
per disputed (refuted) member; a caller-set cadence cap bounds how many
candidates are emitted per call, counting the rest as `deferred`; the
drafted rule carries a `promoted_from: <pattern id>, members: N,
producers: [...]` provenance tag. fathom never writes a rule; this is a
report a human decides whether to act on. Design:
`docs/design/findings-as-evidence.md`, `docs/vision/30-findings-loop.md`,
`docs/vision/31-substrate.md` §3 D8.

**The findings loop** (`docs/vision/30-findings-loop.md`, ADR-040;
`docs/plans/findings-loop.md`): `import` is the governed entry point for an
EXTERNAL review kit's own findings — validated against `.fathom/config.toml`'s
`[findings] subject` (no mapping, no import), deduplicated and ordered by a
per-claim cursor, capped at the producer's declared tier (`schema/semantic/
producers.cue`), and never accepted from a `fathom`-origin producer. The
receipt (`{schema_version, delivered, duplicate, stale, unclassified,
rejected: [{index, finding_id, reason}], truncated, cursors, dry_run}`)
exits 1 on any rejection. `candidates` derives rule candidates from
patterns recurring across `--min-sites` distinct sites (native and
imported findings together) — a report only; fathom never writes a rule.
When `.fathom/config.toml` names an `outbox`, the session hook imports it
once per session and reports the receipt in the brief — see
`docs/configuration.md`'s findings-loop section and
`docs/adoption/findings-loop.md` for the end-to-end guide. `fathom claim
add` (see the `claim` section above) is the loop's third verb: one
human-authored, sme-stated-capped claim minted directly, no recurrence or
import batch required; `fathom claim refute --dry-run` previews which
served rows a refutation would withhold before it is recorded.

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

### `plugin` — one tree, two manifests

```sh
fathom plugin render [--out DIR] [--mcp-url U] [--marketplace]
```

Renders the plugin tree into `--out` (the output dir belongs to the
generator, so re-rendering overwrites) from the same embedded sources
`init` installs — never hand-maintained:

| File | Loader |
|---|---|
| `plugin.json`, `mcp.json` | Agent Plugins 1.0 (VS Code, Cursor, Copilot, Codex, Kiro, …): the closed `$schema`-selected manifest and the `streamable-http` loopback registration |
| `.claude-plugin/plugin.json`, `.mcp.json`, `hooks/hooks.json` | Claude Code: its manifest, the `type: http` registration, the full hook template spelled via the shim-stable `fathom` |
| `skills/<name>/SKILL.md` | both — nine skills, spec-only frontmatter (the six Agent Skills fields are the shared floor) |

Both manifests vet against `schema/plugin/schema.cue` under `task cue-vet`
(`TestPluginManifestsVet`), the Agent Plugins pair is golden-pinned
(`cmd/fathom/testdata/plugin/`), and `scripts/ci.sh` runs `claude plugin
validate --strict` over the rendered tree when a `claude` binary is on PATH
(a named skip otherwise). `--marketplace` also writes
`.claude-plugin/marketplace.json` beside the plugin dir.

---

## The observatory console and the brief

The daemon serves the Observatory console (vision 35, ADR-045; wave F0 of
`docs/plans/observatory-console.md`). Every page renders on one app-kit shell:
a grouped sidebar (Now, Fleet, Proof, Knowledge, Evidence, System, Learn — a
group shows once a page registers in it), a breadcrumb trail, a page heading
with the question it answers and the verbs it renders as copyable chips, a
light/dark/system theme control (browser storage key `fathom-theme`), a skip
link, and a search field. Error pages (404, 400, an unknown project, an
unreadable store) render on the same shell with the reason and the remedy.

| Route | Page |
|---|---|
| `/` | Now — the front door (four questions, surfaces) |
| `/fleet` | Fleet tiles; the JSON carries `fathom fleet report --json` computed by the verb's own function |
| `/projects/{svc}` | One project (was `/observatory/service?id=S`) |
| `/impact` | The four questions, Q1–Q4 in order: each counterfactual arm a range (p25–p75 of real tokens per turn, median marked) with its session count; every tool's outcome "not instrumented" where no instrument exists, never a colored zero; and the catches timeline — verify refusals, conserve losses, integrity catches, guard refusals — filterable by kind, window (`?days=`) and project (`?svc=`), its rows the table. Renders `fathom metrics impact --json` and `fathom metrics catches --json` |
| `/sessions`, `/sessions/{id}` | A table of sessions (one row per session: arm, turns, first edit, tokens, fathom calls, catches), filterable (`?arm=`, `?repo=`, `?q=`), sortable (`?sort=`, `?dir=`), 50 to a page; and one session's replay, windowed — 200 events a page (`?per=`, never more than 2,000 per response), filterable by channel (`?channel=`). Renders `fathom metrics sessions [--id S] --json` |
| `/operate` | The daemon (version, commit, uptime, memory, goroutines); calls per day per surface as a heatmap whose missing day is hatched, never zero, with p50/p95/errors per day in its table; latency and errors by surface and by caller class; the tools by caller class and the zero-call list; the flywheel; `config --explain`; time-to-remedy; a project's capabilities (`?svc=`). Renders `metrics traffic`, `instrument --adoption`, `fleet remedy` and `config --explain` |
| `/decide` | The decision port: provider and sites, the ruling log beside the call log (`?site=`), and the latest calibration reading's table. Renders `fathom decide log --json` |
| `/brief` | The shareable brief; `?format=md` downloads it |
| `/wiring` | Per-repo Preview → Apply → Undo (the POSTs stay at `/observatory/wiring/*` behind the local gate) |
| `/statusz` | Daemon status (JSON schema 3 unchanged) |
| `/search?q=` | Projects, plans and plan rows, symbols, rules and records, verbs and pages, grouped; groups are listed by their best match (a tie goes to the smaller group, then that order) and every query word must match; a row or record ID matches exactly (`OC-1`, `ADR-045`); each group shows 8 of an exact total, and `&group=<slug>` (`projects`, `plans-and-rows`, `symbols`, `rules-and-records`, `verbs`, `pages`) pages through all of it, 50 at a time (`&page=N`). A query of more than 8 words or 200 characters is a 400. `?format=fragment` is the HTML fragment the command palette will load, `?format=json` the results |

- Every page answers `?format=json` (or `Accept: application/json`) with one
  envelope: `schema`, `page`, `route`, `title`, `group`, `question`, `verbs`,
  `metrics` (the shared metrics it shows, each a value with its truth state —
  `present`, `zero`, `absent`, `stale`, `partial`, `error` — reason, source and
  denominator), `sections` (each declared verb's `--json`) and `model`.
- Shared metrics are computed once (`internal/console`): `fleet.services`,
  `fleet.symbols`, `calls.total` (every row in the resolved call log —
  unattributed rows are named, never dropped), `sessions.total` (distinct
  sessions, never ledger rows) and `observations.open` (a partial count stays a
  "≥" lower bound everywhere, the brief included).
- The fleet page's grade distribution over 30 days (RG-7) is
  **experimental**: days without a snapshot are Absent — today's fold
  only, until daily snapshots persist (the page says so). Symbols/traffic/
  serves tiles render Absent, never 0, when the call log or store could
  not be read. A project's 30-day per-dimension series (RG-6) renders a day
  with no call-log rows as Absent and never draws a sparkline across it.
- `fathom brief [--print|--out DIR]` — the same impact model as a dated
  local markdown brief, with an explicit "Not measured" section.
  Sharing is a human verb (law 13): the verb writes a file and stops.

**The call log is resolved once** (OC-2). `serve-http` records itself in
`~/.fathom/daemons/<host>-<port>.json` (its absolute `--call-log`, store, pid and
commit) once its listener is bound, and renders its own pages from that log.
Every metrics, instrument and flywheel verb, `fleet report` and `research
report`/`bundle` then resolve the log the same way: `--call-log`, else
`FATHOM_CALL_LOG`, else the record of the daemon they talk to (`FATHOM_HTTP`,
default `localhost:7575`), else a named `--service`'s own root (J5), else
`[daemon].call_log`, else the legacy default path. A verb run from any directory
now reads the log the daemon writes; `metrics rotate` inspects that same log and
`review` appends its T5-7 gate row to it. A daemon started with `--call-log ""`
records logging off, and verbs talking to it then read no log (rule `none`)
rather than a stale legacy one. Every verb names the log and the rule that chose
it: a `call_log` object (`path`, `rule`, `note`) in its JSON (`fleet report`
keeps `call_log_path` beside it) and a `call log: <path> (rule …)` line in its
text; `research bundle` names it on stderr, never inside the shareable bundle.

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
HTTP a surface is selected by path — `POST /rpc/agent` is the curated 19-tool
coding surface (`internal/mcp/surface.go`; also served at `/rpc/companion` for
compatibility); bare `POST /rpc` is the full registry (54 tools plus
`memory_search`). Off-surface calls execute and are logged `off_surface:true`
rather than rejected; the hard gate exists only behind the server's
`strictSurfaces` option and on hosted `/rpc/modules`.

See [mcp-tools.md](mcp-tools.md) for all tools, prompts, and resources, and
[use-cases/coding-agents.md](use-cases/coding-agents.md) for the canonical
coding-agent wiring (hooks, gates, CLAUDE.md/AGENTS.md).
