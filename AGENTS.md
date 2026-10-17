# AGENTS.md — fathom

A **code-intelligence + evidence engine**: governed symbols/call edges/facts and concern-scoped queries for coding agents, cross-repo impact, review, incidents and fleet health.
Surfaces: Go (`pkg/client`), CLI (`cmd/fathom`), stdio/HTTP MCP. **agentic-kit** and **app-kit** vendored in-tree (Kits row); connector → fusion → query: `docs/architecture.md`, `ADR.md`. **Evaluating fathom rather than changing it?** Start at `README.md` and `docs/early-testers.md`. The rules below govern changes to this repository; the Autonomy line is for the maintainer's own agents.

## Non-negotiables (one line each — the pointer carries the full rules)

| Rule | Full instructions |
|---|---|
| **workspace/** — ALL build artifacts, logs, temp files, local DBs go under gitignored `workspace/` (`db/`, `logs/`, `artifacts/`, `tmp/`), never elsewhere in the tree (also gitignored: `ref/`, `tmp/`) | `docs/usage.md` §workspace/ convention |
| **Gate** — `task validate` (every step `Taskfile.yml`'s `validate:unleased` lists, ending in `go test -race`, with the perfgate timing test in its own non-race step) stays green, and CI runs it on Linux for every push and pull request (`.github/workflows/ci.yml`); `lefthook install` once (pre-commit fmt+cue, pre-push validate+verify) | `Taskfile.yml`, `lefthook.yml` |
| **Done means proven** — a `docs/plans/*.md` box flips to `[x]` only when `fathom verify` proves it, never on claim | `docs/AGENTS.md`, `docs/plans/README.md` |
| **Deploy = `task deploy`, NEVER a raw `cp`** — cp truncates a running executable, and an unsigned binary loses the daemon's macOS file-access grant (deploy signs with a local code-signing identity, then asserts the daemon serves HEAD) | `Taskfile.yml` deploy, `.fathom/observations.md` O10; `docs/dogfood-observations.md` O25 (the incident) |
| **UI = app-kit doctrine** — never hand-write observatory markup; role utilities only; CSS via `task ui:css` | `cmd/fathom/AGENTS.md` |
| **Org-agnostic (ADR-011)** — zero org-specific names/hosts/catalogs in code, docs, tests, fixtures; anything org-specific is configuration | `ADR.md` |

## Project intent

Router `docs/intent/README.md` (consult it before material product, design, domain or architecture work; load only what
it routes you to); vision `docs/design/product-vision.md`; decisions `ADR.md`, indexed at `docs/intent/decisions/README.md`.

Change records: conventional commit with a `Verified:` line, one commit per reviewable
unit, no WIP on main. A `CHANGELOG.md` line in the same change only for observable
behavior (compatibility surface: `docs/intent/scope.md`); `### Breaking` first; collapse
`[Unreleased]` to the delta, never the journey. Rules: `third_party/agentic-kit/docs/project-intent/changes.md`.

Autonomy (`docs/intent/autonomy.md`): commit and push to `main` freely (the integration
branch from v0.5.0), and roll the changelog and tag after a green wave; ask before deleting,
purging, spending LLM budget, or touching global agent config; never force-push `main`,
raw-`cp` the daemon, skip a gate, or claim done without `Verified:`.

## Working conventions

- **TDD**: failing `_test.go` first; golden fixtures (`internal/connector/testdata/fauxtown/`); always verify with `-race` (the gate does; some bugs surface only there).
- **Determinism is a contract** (sorted outputs, injected clocks — `internal/AGENTS.md`); **pure Go** (ADR-066): Linux builds `CGO_ENABLED=0`; darwin keeps cgo (`MACOSX_DEPLOYMENT_TARGET=14.0`) only for cmd/fathom's two proc readers; Linux `-race` needs cgo.
- **Domain model**: shared types in `internal/core`, extended additively; external re-exports in `pkg/client`.
- **Decision models**: every new ADR (from ADR-071) and design doc carries a `## Decision models` section
  (`**Decision models.**` in `ADR.md`): the typed questions it could ask through the hexagonal port, the deterministic
  fallback with provider `none`, the opt-out, and the measurement that proves or kills each, or one line saying why
  none applies (ADR-073; DP-41 enforces).

<!-- appkit:begin -->
## App-kit

App-kit adoption lives at docs/app-kit.md: selected layers/profile, design context,
revision, proof commands and divergences. Before material UI/CLI work, consult it and
the kit's intent router through appkit MCP or the pinned local docs. Select a fitting
capability/recipe; verify the actual task and states in the target UI. Record reusable
improvements or a reasoned divergence and return applicable fixes upstream.
<!-- appkit:end -->

Contribution routing and disposition evidence: `docs/app-kit.md#selected-metadata-and-reusable-contributions-2026-09-16`; shared contract via app-kit MCP `get_kit_decision("contribution-loop")`.

## Path-scoped instructions (nested AGENTS.md, each imported by a one-line CLAUDE.md beside it: Claude Code loads on demand, Codex root-down)

| Path | Carries |
|---|---|
| `internal/AGENTS.md` | determinism contract; adding extractors / analyzers / MCP tools; store purity |
| `docs/AGENTS.md` | plan-row traps and the verify gate (the grammar itself: `docs/plans/README.md`); docs upkeep |
| `cmd/fathom/AGENTS.md` | the observatory / app-kit UI doctrine |
| `schema/AGENTS.md` | CUE schema conventions (valid/invalid testdata pairs) |

## Source of truth

| Topic | Where |
|---|---|
| Decisions | `ADR.md` |
| Live programs (computed: `fathom plans status --service fathom`; largest: language breadth, the responsible tenant) | `docs/plans/language-breadth.md`, `docs/plans/responsible-tenant.md`; founding designs `docs/design/evidence-plane.md`, `docs/design/semantic-layers.md`, `docs/design/product-vision.md` |
| Roadmap (goals, the roadmap through 2027-01, computed progress) | `docs/intent/scope.md`, `docs/vision/38-proof-layer-roadmap.md`, `fathom plans status --service fathom` |
| Vision series (numbered; the README carries status) | `docs/vision/` |
| Plans — gated checklists + grammar | `docs/plans/`, `docs/plans/README.md` |
| Orientation (architecture, the MCP tools and surfaces catalog, adding a language or tool format) | `docs/architecture.md`, `docs/mcp-tools.md`, `docs/design/language-breadth.md` §13 (the worker recipe) |
| Kits (adopted kits, pins, adoption records) | `docs/app-kit.md`, `appkit.adoption.json`; `third_party/app-kit/`, `third_party/agentic-kit/` (pinned by each `.vendor-commit`, refreshed by `task sync-kit`); `schema/testkit/` (the test-kit mirror); the mirrors are packages of this module by import-path rewrite (`cmd/kitrewrite`, ADR-070), no kit in `go.mod`; ADR-063 (kit overlaps) |
| Fleet index, daemon ops, deploy, dogfood discipline | `docs/guides/fleet-index.md` |
| Cloud sessions (Claude Code on the web, Codex cloud) | `docs/adoption/cloud-sessions.md`, `docs/plans/cloud-sessions.md`, `scripts/cloud-setup.sh` |
| Release (the changelog, release narratives, the ritual: tag after a green wave) | `CHANGELOG.md`, `docs/changelog/`, `third_party/agentic-kit/.claude/skills/intent-release/SKILL.md`, `docs/intent/autonomy.md` |
| CLI/workspace; use-case guides (keep `## Maintained` current) | `docs/usage.md`, `docs/use-cases/` |
| Early testers: install, what works, known limits | `docs/early-testers.md` |
| Friction log (`fathom observe`; the narrative log is frozen) | `.fathom/observations/` (live, one file per entry; `.fathom/observations.md` the legacy log); `docs/dogfood-observations.md` (historical) |
<!-- fathom:begin v3 sha256:158432b6c8d66bbf889164f9f254db642c1263bee71b4bc23bd4209258b2d8ff -->
## fathom code intelligence

This repo is indexed by fathom as service "fathom". Route by
question shape, on this session's default eight-tool MCP registration:
where a symbol is defined is `find_symbol`; who-calls it, briefly
(definition, a capped caller/callee list, tests) is `symbol_context`;
what-breaks is `blast_radius`; a composed review verdict over a diff is
`review_diff`; what-governs this repository is `obligations` with no
path (the root conventions); is a plan's box proven is `verify_plan`;
the current text at a cited line is `read_source`. A literal string, a
config value, a log line, or a document-versus-document contradiction
is `search`'s job (grep with a receipt), not the other tools'.
Read files to *edit* them, not to discover them. The wider verb set
(the full resolved caller graph, staleness and what-was-known, and
more) lives one hop away at `/rpc/extended` (or its `/rpc/companion`
alias); register that surface instead of `/rpc/agent` for a session
that needs it.

The index refreshes itself: a read of a drifted service answers from
what it has and queues a re-ingest in the background: nothing to call.

Obligations — governing rules for the file you are about to touch, and
where `docs/intent/` exists its matched invariants — are injected on every
edit via the PostToolUse hook. Treat them as binding, not advisory.

Continuity: run `fathom prime` at session start if your harness runs no
SessionStart hook. It prints the same identity, freshness, live-example,
thread-brief, and custodian primer this hook injects on Claude Code.
File what the next session must know: `fathom thread note "…"`.

Done means proven: a plan task is only "done" when the index proves it, not
when it is claimed. Before checking off a `docs/plans/*.md` item, run:

    fathom verify --service fathom --plan docs/plans/<plan>.md

Exit 0 = every claimed task verifies. Exit 1 = a claimed task is unproven —
fix the code or uncheck the box. Exit 2 = operational (daemon down / index
stale) — fail-open, not a verification failure.
<!-- fathom:end -->
