# CLAUDE.md — sextant

Guidance for AI agents (and humans) working in this repo.

## What sextant is

A **code-intelligence + evidence engine**. It ingests source across one or many
repos, extracts symbols / call edges / facts, stores them as governed **evidence
observations**, and answers *concern-scoped* queries — for AI coding assistants,
cross-repo blast-radius, change review, incident investigation, and fleet health.
Surfaces: a Go client (`pkg/client`), a CLI (`cmd/sextant`), and an **MCP server**
(stdio + HTTP).

It builds on **agentic-kit** (`../agentic-kit`, wired via `go.work` + `replace`)
for governed-evidence primitives: evidence tiers (`review/scoring`), `llmclient`,
`embedding`, `security`, `telemetry`, `contracts`.

## ⚠️ workspace/ convention (read this first)

**All build artifacts, logs, temp files, and local databases go under
`workspace/` at the repo root, which is gitignored.** Never write generated files
elsewhere in the tree.

- `workspace/db/`        — DuckDB stores (CLI default: `workspace/db/sextant.duckdb`)
- `workspace/logs/`      — run/test logs
- `workspace/artifacts/` — generated artifacts (gen-claude-md output, manifests, exports)
- `workspace/tmp/`       — ephemeral scratch

`store.Open` auto-creates the parent dir. When adding code or commands that emit
files, default their output under `workspace/`.

Other gitignored top-level dirs: `ref/` (reference clones of external tools, see
`ref/README.md` — inspiration only, never imported/built) and `tmp/` (the scanned
design-doc conversion: `tmp/design_inspiration/files/` + `tmp/TRACKING.md`).

## Deterministic-excellence gate

Everything must stay green:

```sh
task validate   # fmt-check + vet + cue-vet + go test -race  (CGO/DuckDB)
task test-unit  # -short
task doctor     # toolchain + workspace health
lefthook install
```

- **lefthook**: pre-commit = gofmt + cue-vet on staged files; pre-push = `task validate`.
- **go-task**: `Taskfile.yml` is the only command runner. Tests run with `CGO_ENABLED=1`
  (DuckDB) and `MACOSX_DEPLOYMENT_TARGET=14.0`.
- **Always verify with the race detector** (`go test -race ./...`) — the gate uses it;
  several test bugs only surface under `-race` (e.g. reading a server's output buffer
  while a `Serve` goroutine writes it — run `Serve` synchronously against finite input).

## Architecture (three layers — see ADR.md)

```
internal/connector   SPI: Discover/Fetch (Local / Faux / GitHub)
        ↓
internal/fusion      3-pass pipeline: filter → parallel fetch → deterministic
                     post-fetch (sha256 hash-gate, budget gate, lazy) → extract → store
        ↓
internal/query       concern-scoped lookup, evidence-tier ranking (agentic-kit),
                     staleness, contradictions; hybrid semantic via internal/vector
```

Supporting: `internal/extractor` (Go via `go/parser`; Python/JS/Java/Ruby/HCL/k8s/
CUE/Markdown/go.mod via deterministic heuristics — tree-sitter/WASM is the
documented future path, ADR-004), `internal/store` (DuckDB + memory fake, behind a
`Store` interface), `internal/mcp` (JSON-RPC server, tool groups/profiles),
`internal/analysis/*` (callgraph, complexity, apisurface, archconformance),
`internal/platformlink` + `pkg/platform` (cross-repo graph), `internal/depgraph`,
`internal/{adr,ownership,sdd}`, `internal/{llm,llmsec,vector}` (agentic-kit enrichment).

## Conventions

- **TDD**: write the failing `_test.go` first; assert determinism explicitly; prefer
  golden fixtures against `testdata/fauxtown/` (the deterministic fixture fleet).
- **Determinism is a contract**: sort outputs; inject clocks (`now func() time.Time`),
  never call `time.Now()` in query/analysis logic.
- **Org-agnostic / open-source-ready (ADR-011)**: NO organization-specific names,
  hosts, catalogs, or product references anywhere — in code, docs, tests, or fixtures.
  Anything org-specific is configuration. (Zero such references is a hard requirement.)
- **The domain model is the keystone**: shared types live in `internal/core`; extend
  it additively; `pkg/client` re-exports for external consumers.
- New CUE schemas: a `schema.cue` package with `testdata/valid.cue` (positive) and a
  commented `testdata/invalid.cue`; negatives are asserted by Go tests, not live-vetted.

### Adding things
- **Extractor**: implement `extractor.Extractor`, register by extension in
  `fusion.New()`, add its ext→kind in `connector`, golden-test it.
- **Analyzer**: a pure `internal/analysis/<name>` package over `[]core.SymbolNode`
  /`[]core.Observation`; deterministic; tested.
- **MCP tool**: register in `internal/mcp` with a group; extend the backend interface
  minimally if it needs new data; cover with a fake-backend test.

## Source of truth

- Design docs (drive the implementation): `tmp/design_inspiration/files/` (13 docs).
- Roadmap + status (exhaustive checklist): `tmp/TRACKING.md`.
- Decisions: `ADR.md`. Overview/usage: `README.md`. Deeper docs: `docs/`.

## Dogfood: the fleet index

Sextant indexes itself (with throughline, agentic-kit, app-kit) into
`workspace/db/fleet.duckdb`, served to Claude Code sessions via the
user-scoped `sextant` MCP server. When working in this repo, use those tools
(`find_symbol`, `query_symbols`, `find_callers`, `outline`, `repo_map`) for
discovery before any Grep/Read sweep, and refresh via the MCP `ingest` tool
(`{"service":"sextant"}`) at session start and after commits. The DuckDB
store is single-writer: while a session's server holds the lock, CLI
`ingest` fails — that is expected; a best-effort post-commit lefthook covers
no-session commits. Repo-specific index excludes live in `.sextantignore`
(`ref/`, `tmp/`, `workspace/`, `bin/`).

**Dogfood discipline (we are sextant's first consumer — keep it sharp for the
rest of the fleet):**
- **Log what you observe.** Recall gaps, contract mismatches, latency, pleasant
  surprises → `docs/dogfood-observations.md`, with a reproduction (probe a
  known-good case beside the failing one and root-cause the difference). When you
  fix sextant, record it in `CHANGELOG.md` and close the observation.
- **Staleness is the silent failure.** `serve-mcp` is a long-running process that
  can outlive several commits; a committed fix does **not** reach it until the
  binary is rebuilt *and* the server restarted. If a tool misbehaves, first run
  `sextant version` and compare to `git rev-parse HEAD` — a mismatch means stale,
  not buggy. The `post-commit` hook rebuilds `bin/sextant` on Go changes, but a
  running server still needs a restart (reload the MCP config / session) to load
  it. Reap duplicate `serve-mcp` processes — only one can hold the write lock.
