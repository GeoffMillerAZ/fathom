# app-kit agent guidance

app-kit makes each app better to use and the next easier to build.
It guides, it does not dictate. Consumers keep their own product intent and authority.

## Project intent

Durable intent lives at `docs/intent/README.md` (router). Vision: `VISION.md`.
Decisions: `docs/adr/` (general decision records; index: `docs/adr/README.md`).
Consult the router before material product, design, domain or architecture work;
load only its routed context. Record durable decisions and state fired triggers
before designing around them. Existing design sketches are not availability claims.

Change records: conventional commits with an honest `Verified:` line, one reviewable
unit per commit, no WIP on main. Update `CHANGELOG.md` for observable consumer changes
(compatibility: `docs/intent/scope.md`); Breaking first, Unreleased is the delta.
Practice: `../agentic-kit/docs/project-intent/changes.md`.

Autonomy (`docs/intent/autonomy.md`): execute scoped work and verification; existing
session authorization counts. Publishing, global config, real data and destructive
actions need scope-specific authority. Preserve unrelated work; never bypass gates.

## Consumer routing and layers

Current priority and handoff status: `docs/design/stabilization-and-resumption.md` (MCP: `get_kit_decision("stabilization")`, ADR-0072). Resume one named need with fixed acceptance and a budget. Do not automatically resume all G1–G6 work. Deferred is not passed; preserve evidence and unrelated work.

Use `docs/design/adoption.md` (MCP: `get_kit_decision("adoption")`) for setup and proof.
At discovery, before duplicating a workaround, and at material UI/CLI handoff, use `docs/design/contribution-loop.md` (MCP: `get_kit_decision("contribution-loop")`) to record reusable findings and refresh consumer intent/agent guidance; no automatic cross-repo writes.
Use `docs/design/experience-system.md` for recipes and analytics/interaction quality.
Use `get_kit_decision("intent")` to discover the same intent docs through MCP.

ADR-0017: **Stopping is a complete adoption**. Choose the applicable layers; do not
impose the Go runtime on portable consumers. ADR-0021 scopes runtime profiles.

| Layer / target | Contract |
|---|---|
| L0 | CUE token source and semantic roles |
| L1 | portable CSS/custom properties; app-owned palette/identity |
| L2 | dependency-free browser JS with documented globals/hooks |
| L3a | Go SVG/rendered output using vars; no Tailwind scanner |
| L3b/L4 | utility-themed Go/templ UI; consumer must scan kit sources for CSS |
| L5 | discovery/guidance, not an application runtime dependency |
| Terminal | appkit/cli; read docs/design/cli-kit.md (ADR-0018) |

Use `appkit vendor` for selected portable artifacts and integrity checks; use Go
imports for runtime packages. Promotion follows ADR-0017/0018; documented divergence
is valid. First-party UI APIs: `docs/ui-components.md`. Commercial source retrieval
does not prove compatibility or constitute an implemented kit recipe.

For material UI work, choose a complete visual reference separately from the renderer.
Private templates, Catalyst and dependency context: `docs/design/private-references.md`
(MCP: `get_kit_decision("design-references")`, `search_design_assets`).
Preserve reference hierarchy, typography, spacing and interaction states; theming is
contextual review, not automatic palette replacement. Compare original and adaptation.
For conversion/agent repair, read `docs/design/conversion-pipeline.md`; choose static,
Go/Datastar or client-heavy boundaries with `docs/design/runtime-selection.md` (same MCP IDs).
Read `docs/design/just-in-time-conversion.md` and `docs/design/converted-component-standard.md` for core-first standards, CUE implementation/gate/adoption tracking and bounded on-demand work; never upgrade raw originals or weaken failed gates.
Originals remain first-class choices under ADR-0022; target compatibility needs proof.
Bounded HTML translation and saved attempts exist; see docs/design/html-translation.md.
General interactive conversion and target certification remain incomplete. Never weaken
an oracle or silently switch runtimes. ADR-0023 scopes older restrictions.

Selected UI references: Resume-as-Code and Fathom, not automatic migration orders. CA/GGB and Windows stay deferred.
For Wails work, read docs/design/web-desktop-parity.md (ADR-0033): browser core tasks
remain independent; native exceptions and local/remote topology are explicit.
Repo Fathom configuration and evidence: docs/design/fathom-maintenance.md. Use its
bounded guards and config checks; stale compiled claims are not current authority.

## Maintainer-only workflow (this app-kit checkout)

The following applies to work in this repository, not the caller's consumer repository.

- `task check`: vet/tests/CUE validation and generated-artifact cleanliness.
- `task gen`: default tokens; `task gen:synthwave`: shipped alternate theme.
- `task gallery:css` / `task gallery:run`: build CSS / launch the existing catalog.
- Pinned tools: `go tool templ`, `go tool cue`; Tailwind via `appkit css`.
- CUE owns generated tokens. Do not hand-edit generated CSS/Go/DTCG outputs.
- No npm build in this kit; ADR-0017 scopes the consumer boundary and ADR-0020
  retains the isolated browser-test exception. Licensed data is private input.
- Never bypass checksum verification, silently change public contracts or describe
  a planned runtime as shipped. Invariants and evidence: `docs/intent/invariants.md`.
- `go test ./cmd/appkit-mcp` checks served docs/index and actual tool/resource parity.
- Pre-push runs `decision-index` for both ADR registers/MCP guidance and `quality-evidence` for receipt/readiness. Update both decision registers with an ADR; review changed standards/evidence fixture pins, never waive hashes. Static Fathom verification cannot replace these runtime checks.

<!-- fathom:begin v3 sha256:6ed8d0796d88b63860afac0b2ef0878f9263b12d26cc1e78a797fe8e0f908129 -->
## fathom code intelligence

This repo is indexed by fathom as service "app-kit". Route by
question shape (SB-7, docs/vision/27-strengths-bench.md §8 — the home
bench found "index before any grep" over-broad): where / who-calls /
what-breaks resolve via the fathom MCP tools (`find_symbol`,
`references`, `symbol_context`, `outline`, `repo_map`, `pack_context`,
`verify_plan`); what-governs this repository is `obligations` with no
path (the root conventions); is this still true / what was known is
`staleness_report` (RC-23, docs/vision/28-receipts.md §3.5). A literal
string, a config value, a log line, or a document-versus-document
contradiction is `search`'s job (grep with a receipt, RC-24), not the
other tools'.
Read files to *edit* them, not to discover them.

Opening moves by task shape (first 2-3 calls, then edit):
- debug/trace: find_symbol -> find_callers/references -> symbol_context
- feature/change: repo_map -> pack_context(concern) -> references
- review/verify: verify_plan -> obligations -> references on changed symbols
- refactor: references -> outline -> symbol_context

Refresh the index via the `ingest` MCP tool (`{"service":"app-kit"}`) at
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

    fathom verify --service app-kit --plan docs/plans/<plan>.md

Exit 0 = every claimed task verifies. Exit 1 = a claimed task is unproven —
fix the code or uncheck the box. Exit 2 = operational (daemon down / index
stale) — fail-open, not a verification failure.
<!-- fathom:end -->
