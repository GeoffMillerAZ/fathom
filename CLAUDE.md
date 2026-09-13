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
| **Deploy = `task deploy`, NEVER a raw `cp`** — cp truncates a running executable, and an unsigned binary loses the daemon's TCC grant (deploy signs with the `fathom-dev` identity, then asserts the daemon serves HEAD) | `Taskfile.yml` deploy, `.fathom/observations.md` O10; `docs/dogfood-observations.md` O25 (the incident) |
| **Question-shape routing (SB-7)** — where/who-calls/what-breaks/what-governs/what-was-known resolve via the fathom MCP tools (`find_symbol`, `references`, `symbol_context`, …); a literal string, a config value, or a log line is grep's job; refresh via the `ingest` MCP tool, never a shell `fathom ingest` against the shared store | `docs/guides/fleet-index.md` |
| **UI = app-kit doctrine** — never hand-write observatory markup; role utilities only; CSS via `task ui:css` | `cmd/fathom/CLAUDE.md` |
| **Org-agnostic (ADR-011)** — zero org-specific names/hosts/catalogs in code, docs, tests, fixtures; anything org-specific is configuration | `ADR.md` |

## Project intent

Durable intent lives at `docs/intent/README.md` (router). Vision: `docs/design/product-vision.md`.
Decisions: `ADR.md` (general decision records, not only architecture; index at
`docs/intent/decisions/README.md`). Consult the router before material product, design, domain, or
architecture work and load only what it routes you to. Record decisions that pass its
threshold; state fired revisit triggers before designing around them.

Change records: conventional commit with a `Verified:` line, one commit per reviewable
unit, no WIP on main. A `CHANGELOG.md` line in the same change only for observable
behavior (compatibility surface: `docs/intent/scope.md`); `### Breaking` first; collapse
`[Unreleased]` to the delta, never the journey. Rules: `agentic-kit/docs/project-intent/changes.md`.

Autonomy (`docs/intent/autonomy.md`): commit and push to `main` freely (the integration
branch from v0.5.0), and roll the changelog and tag after a green wave; ask before deleting,
purging, spending LLM budget, or touching global agent config; never force-push `main`,
raw-`cp` the daemon, skip a gate, or claim done without `Verified:`.

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
| Active program: the evidence plane (intent corpus, honesty debt, change records) | `docs/design/evidence-plane.md`, plan `docs/plans/evidence-plane.md`; founding designs `docs/design/semantic-layers.md`, `docs/design/product-vision.md` |
| Vision series (numbered; the README carries status) | `docs/vision/` |
| Plans — gated checklists + grammar | `docs/plans/`, `docs/plans/README.md` |
| Fleet index, daemon ops, deploy, dogfood discipline | `docs/guides/fleet-index.md` |
| MCP tools + surfaces catalog | `docs/mcp-tools.md` |
| CLI usage + workspace convention | `docs/usage.md` |
| Use-case guides (one doc per use case; keep `## Maintained` current) | `docs/use-cases/` |
| Friction log (`fathom observe`; the narrative log is frozen) | `.fathom/observations.md` (live); `docs/dogfood-observations.md` (historical) |
| Original design docs (gitignored, local-only); roadmap = `fathom plans status` + `docs/intent/scope.md` | `tmp/design_inspiration/files/` |

<!-- fathom:begin v3 sha256:937d65ff005bd70f39830922abd1cef8d36cc2748037de3fa3d535066c703b2f -->
## fathom code intelligence

This repo is indexed by fathom as service "fathom". Route by
question shape (SB-7, docs/vision/27-strengths-bench.md §8 — the home
bench found "index before any grep" over-broad): where / who-calls /
what-breaks resolve via the fathom MCP tools (`find_symbol`,
`references`, `symbol_context`, `outline`, `repo_map`, `pack_context`,
`verify_plan`); what-governs this repository is `obligations` with no
path (the root conventions); is this still true / what was known is
`staleness_report` (RC-23, docs/vision/28-receipts.md §3.5). A literal
string, a config value, a log line, or a document-versus-document
contradiction is grep's job, not the index's.
Read files to *edit* them, not to discover them.

Opening moves by task shape (first 2-3 calls, then edit):
- debug/trace: find_symbol -> find_callers/references -> symbol_context
- feature/change: repo_map -> pack_context(concern) -> references
- review/verify: verify_plan -> obligations -> references on changed symbols
- refactor: references -> outline -> symbol_context

Refresh the index via the `ingest` MCP tool (`{"service":"fathom"}`) at
session start and after each commit; it is hash-gated, so a clean repo
refreshes in well under a second.

Obligations — governing rules for the file you are about to touch — are
injected automatically on every edit via the PostToolUse hook. Treat them
as binding, not advisory.

Continuity: run `fathom prime` at session start if your harness has no
SessionStart hook (Codex does not) — it prints the same identity,
freshness, live-example, thread-brief, and custodian primer this hook
injects on Claude Code. File what the next session must know:
`fathom thread note "…"`.

Done means proven: a plan task is only "done" when the index proves it, not
when it is claimed. Before checking off a `docs/plans/*.md` item, run:

    fathom verify --service fathom --plan docs/plans/<plan>.md

Exit 0 = every claimed task verifies. Exit 1 = a claimed task is unproven —
fix the code or uncheck the box. Exit 2 = operational (daemon down / index
stale) — fail-open, not a verification failure.
<!-- fathom:end -->
