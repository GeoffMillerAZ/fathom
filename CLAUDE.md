# CLAUDE.md — fathom

A **code-intelligence + evidence engine**: it ingests source across one or
many repos, extracts symbols / call edges / facts as governed evidence
observations, and answers concern-scoped queries — for coding agents,
cross-repo blast-radius, change review, incident investigation, fleet health.
Surfaces: Go client (`pkg/client`), CLI (`cmd/fathom`), MCP server (stdio +
HTTP). Built on **agentic-kit** (`../agentic-kit`, via `go.work` + `replace`).
Three layers — connector → fusion → query: `docs/architecture.md`, `ADR.md`.

## Non-negotiables (one line each — the pointer carries the full rules)

| Rule | Full instructions |
|---|---|
| **workspace/** — ALL build artifacts, logs, temp files, local DBs go under gitignored `workspace/` (`db/`, `logs/`, `artifacts/`, `tmp/`), never elsewhere in the tree (also gitignored: `ref/`, `tmp/`) | `docs/usage.md` §workspace/ convention |
| **Gate** — `task validate` (fmt-check, vet, cue-vet, vendor-check, scripts-check, `go test -race`) stays green; `lefthook install` once (pre-commit fmt+cue, pre-push validate+verify) | `Taskfile.yml`, `lefthook.yml` |
| **Done means proven** — a `docs/plans/*.md` box flips to `[x]` only when `fathom verify` proves it, never on claim | `docs/CLAUDE.md`, `docs/plans/README.md` |
| **Deploy = `task deploy`, NEVER a raw `cp`** — cp truncates a running executable, and an unsigned binary loses the daemon's TCC grant (deploy signs with the `fathom-dev` identity, then asserts the daemon serves HEAD) | `Taskfile.yml` deploy, `.fathom/observations.md` O10 |
| **Index-first** — resolve where/what/who-calls via the fathom MCP tools (`find_symbol`, `references`, `symbol_context`, …) before any Grep/Read sweep; refresh via the `ingest` MCP tool, never a shell `fathom ingest` against the shared store | `docs/guides/fleet-index.md` |
| **UI = app-kit doctrine** — never hand-write observatory markup; role utilities only; CSS via `task ui:css` | `cmd/fathom/CLAUDE.md` |
| **Org-agnostic (ADR-011)** — zero org-specific names/hosts/catalogs in code, docs, tests, fixtures; anything org-specific is configuration | `ADR.md` |

## Working conventions

- **TDD**: write the failing `_test.go` first; prefer golden fixtures
  (`internal/connector/testdata/fauxtown/` is the deterministic fixture fleet).
- **Determinism is a contract**: sorted outputs, injected clocks — `internal/CLAUDE.md`.
- **CGO**: tests run `CGO_ENABLED=1` + `MACOSX_DEPLOYMENT_TARGET=14.0`
  (tree-sitter, ADR-013); `internal/store` stays pure Go (ADR-020/025).
- **The domain model is the keystone**: shared types live in `internal/core`,
  extended additively; `pkg/client` re-exports for external consumers.
- Always verify with `-race` — the gate uses it; several test bugs only
  surface under it.

## Path-scoped instructions (auto-loaded when you work there)

| Path | Carries |
|---|---|
| `internal/CLAUDE.md` | determinism contract; adding extractors / analyzers / MCP tools; store purity |
| `docs/CLAUDE.md` | plan grammar v2 essentials, anchor rules, the verify gate; docs upkeep |
| `cmd/fathom/CLAUDE.md` | the observatory / app-kit UI doctrine |
| `schema/CLAUDE.md` | CUE schema conventions (valid/invalid testdata pairs) |

## Source of truth

| Topic | Where |
|---|---|
| Decisions | `ADR.md` |
| Active program: semantic layers + expert modules | `docs/design/semantic-layers.md`, `docs/design/product-vision.md` |
| Vision series (thesis → observatory UI) | `docs/vision/` (01–17) |
| Plans — gated checklists + grammar | `docs/plans/`, `docs/plans/README.md` |
| Fleet index, daemon ops, deploy, dogfood discipline | `docs/guides/fleet-index.md` |
| MCP tools + surfaces catalog | `docs/mcp-tools.md` |
| CLI usage + workspace convention | `docs/usage.md` |
| Use-case guides (one doc per use case; keep `## Maintained` current) | `docs/use-cases/` |
| Friction log (record what you observe) | `docs/dogfood-observations.md` |
| Original design docs + roadmap checklist (gitignored, local-only) | `tmp/design_inspiration/files/`, `tmp/TRACKING.md` |

<!-- fathom:begin -->
## fathom code intelligence

This repo is indexed by fathom as service "fathom". Before any Grep/Read
sweep, resolve where/what/who-calls via the fathom MCP tools:
`find_symbol`, `references`, `symbol_context`, `outline`, `repo_map`,
`pack_context`, `verify_plan`. Read files to *edit* them, not to discover
them — the index replaces exploratory grep.

Refresh the index via the `ingest` MCP tool (`{"service":"fathom"}`) at
session start and after each commit; it is hash-gated, so a clean repo
refreshes in well under a second.

Obligations — governing rules for the file you are about to touch — are
injected automatically on every edit via the PostToolUse hook. Treat them
as binding, not advisory.

Done means proven: a plan task is only "done" when the index proves it, not
when it is claimed. Before checking off a `docs/plans/*.md` item, run:

    fathom verify --service fathom --plan docs/plans/<plan>.md

Exit 0 = every claimed task verifies. Exit 1 = a claimed task is unproven —
fix the code or uncheck the box. Exit 2 = operational (daemon down / index
stale) — fail-open, not a verification failure.
<!-- fathom:end -->
