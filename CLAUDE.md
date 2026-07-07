# CLAUDE.md — fathom

Guidance for AI agents (and humans) working in this repo.

## What fathom is

A **code-intelligence + evidence engine**. It ingests source across one or many
repos, extracts symbols / call edges / facts, stores them as governed **evidence
observations**, and answers *concern-scoped* queries — for AI coding assistants,
cross-repo blast-radius, change review, incident investigation, and fleet health.
Surfaces: a Go client (`pkg/client`), a CLI (`cmd/fathom`), and an **MCP server**
(stdio + HTTP).

It builds on **agentic-kit** (`../agentic-kit`, wired via `go.work` + `replace`)
for governed-evidence primitives: evidence tiers (`review/scoring`), `llmclient`,
`embedding`, `security`, `telemetry`, `contracts`.

## ⚠️ workspace/ convention (read this first)

**All build artifacts, logs, temp files, and local databases go under
`workspace/` at the repo root, which is gitignored.** Never write generated files
elsewhere in the tree.

- `workspace/db/`        — DuckDB stores (CLI default: `workspace/db/fathom.duckdb`)
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
- **Adoption / use-case guides:** `docs/use-cases/` — one how-to per use case
  (getting-started + coding-context, cross-repo-blast-radius, spec-verification,
  incident-investigation, change-review, fleet-health) for repos wiring fathom
  into Claude Code. Convention: every use case has exactly one doc here; when an
  MCP tool or CLI command is added/renamed/removed, update the relevant doc's
  `## Maintained` line (and the catalog in `docs/mcp-tools.md`).

## Dogfood: the fleet index

Fathom indexes the fleet (agentic-kit, app-kit, fathom; platform-kit once
it's added) into `workspace/db/fleet.duckdb`, served by a **single always-on
HTTP daemon** — launchd `com.geoffmilleraz.fathom-http` running `serve-http`
at `http://localhost:7575/rpc`, registered user-scope as an **HTTP MCP**. Every
session shares one index and one store, so there is no per-session server, no
single-writer lock contention, and no "Failed to connect." If a session doesn't
see the fathom tools (`find_symbol`, `repo_map`, …), it started before the
switch — restart it once.

When working in this (or any fleet) repo:
1. **Index first.** Resolve where / what / who-calls via fathom (`find_symbol`,
   `query_symbols`, `references`, `find_callers`, `symbol_context`, `outline`,
   `repo_map`, `pack_context`) before any Grep/Read sweep — `references` replaces
   fleet-wide grep, `symbol_context` gives def+callers+callees+tests in one call.
   Read files to *edit* them, not to discover them. The index is fleet-wide, so
   cross-repo questions are one query. (These are the core of the 14-tool
   `companion` surface, which the daemon also serves standalone at
   `POST /rpc/companion` — see docs/mcp-tools.md.)
2. **Refresh through the server, never the CLI.** The daemon holds the
   single-writer DuckDB lock, so a shell `fathom ingest` fails with a lock
   error. Call the `ingest` MCP tool (`{"service":"<this-repo>"}`) at session
   start and after each commit (hash-gated, ~0.2s when clean). Because the store
   is shared, your refresh updates the index for *every* session.
3. **Honesty.** After heavy *uncommitted* edits, call `staleness_report` before
   trusting results, or just read the file you're about to change.
4. **Scope.** Only fleet repos are indexed. A repo that isn't a fathom service
   yet must be ingested as a new service before its symbols appear.

Repo-specific index excludes live in `.fathomignore` (`ref/`, `tmp/`,
`workspace/`, `bin/`).

**Dogfood discipline (we are fathom's first consumer — keep it sharp for the
rest of the fleet):**
- **Log what you observe.** Recall gaps, contract mismatches, latency, pleasant
  surprises → `docs/dogfood-observations.md`, with a reproduction (probe a
  known-good case beside the failing one and root-cause the difference). When you
  fix fathom, record it in `CHANGELOG.md` and close the observation.
- **Staleness — two kinds.** *Data* staleness is handled automatically:
  `serve-http` re-ingests a service on git-HEAD drift before answering
  (`--auto-refresh`, default on), and you can force it via the `ingest` MCP tool.
  *Binary* staleness is the one to watch — the daemon runs a built binary, so a
  committed Go fix doesn't reach it until `bin/fathom` is rebuilt **and** the
  daemon restarted (`launchctl kickstart -k gui/$(id -u)/com.geoffmilleraz.fathom-http`).
  The `post-commit` hook rebuilds `bin/fathom` on Go changes; if a tool
  misbehaves, compare `fathom version` to `git rev-parse HEAD` — a mismatch
  means stale, not buggy.
- **Rebuilding the fleet index.** If the index is stale despite auto-refresh,
  has duplicate/path-like services, or is bloated (originally ingested without
  `.fathomignore`), rebuild it cleanly with `scripts/reingest-fleet.sh` (run
  from a real terminal — it stops/starts the daemon). It re-ingests each fleet
  repo with a clean `--service`/`--path`, recording the `ServiceMeta`
  path+commit that auto-refresh, `staleness_report`, and the `ingest` tool all
  depend on.
