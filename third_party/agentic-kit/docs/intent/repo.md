# Repository charter

Intent about the container: why agentic-kit is one repository, what belongs in it, how
the parts relate, and which parts are expected to leave or arrive. Practice:
`docs/project-intent/monorepo.md`. Package-level classification is `docs/compat.md`;
this file carries what a lint cannot: the evolution intent.

## Why one repository

The kit's promise is "import it and be compliant by default" (VISION one-sentence
test). That promise is only checkable when one gate (`task validate`), one
constitution (`cmd/constitution-lint`), one compatibility audit (`compat.md`), one
catalog, and one downstream check run over every package at once. Splitting would
multiply gates, let sibling packages drift apart between tags, and break atomic
cross-package change. Zero external dependencies (I1) is what makes 172 packages in one
module cheap: nothing here has a version to negotiate.

## What belongs here, what does not

- Belongs when: it is stdlib-only Go or CUE, has a named consumer or a named future
  workload (VISION junk gate), states its concurrency contract, and ships `schema.cue`
  where shape is data.
- Leaves when: zero consumers verified at a wave end (uplift D2, D5), or it needs a
  dependency (then it becomes a nested module, uplift D1).
- Never here: application `main()`s beyond `cmd/` tools and examples (I18); reference
  sources except under `_reference/`; anything org-specific (I2).

## Component map

| Component | Kind | Intent | Consumers | Evolution |
|---|---|---|---|---|
| `foundations/**` (148 pkgs, 30 categories) | library | root | every kit; fathom, throughline, growing-gardens-bi directly | stays; retire per D5 when a package has zero consumers |
| `kits/agent-harness` | kit | root | `examples/agent-cli`, `scenarios/governed-docextract-hitl`, `cmd/kit-new` output; Conjure `heal` (future) | stays; flagship |
| `kits/docextract` | kit | root | growing-gardens-bi | stays |
| `kits/docextract/pdf`, `kits/docextract/xlsx` | kit (nested modules) | root | growing-gardens-bi via `docextract` | split when: a consumer outside the fleet pins either module independently, or its release cadence diverges from the root for two tags (`compat.md` §v1 record) |
| `kits/endpoint-guard` | kit | root | `cmd/endpoint-guard` sidecar | stays |
| `kits/eval-runner` | kit | root | this repo's own `task eval` (uplift D9) | stays; Stages 2–3 are `futures.md` F14 |
| `kits/expertchat` | kit | root | `examples/expertchat-service`; gke-expertchat wave | stays; own intent when a second deployment names it |
| `kits/hitl` | kit | root | `scenarios/governed-docextract-hitl`; growing-gardens-bi assessed | stays |
| `kits/investigation` | kit | root | `scenarios/governed-investigation` | stays |
| `kits/guardrails-pipeline` | kit | root | throughline (`app/chat/untrusted.go`) | stays; classification decision open (`questions.md` Q2) |
| `kits/command-center`, `kits/governed-prose-kit`, `kits/mcp-server` | kit | root | none in-fleet ("composition unproven") | retire when: still no consumer at the next wave end (uplift D2) |
| `runtimes/credresolve` | runtime | root | throughline (`adapters/claudeauth`) | stays |
| `runtimes/policy` | runtime | root | none | retire when: no consumer at the next wave end |
| `surfaces/forms/cueform`, `surfaces/editor` | surface | root | `surfaces/editor` ← `cueform`; app-kit adjacent | stays |
| `toolchains/jsonschema`, `toolchains/materialization/*` | toolchain | root | `llmclient` (`OutputSchema`), each other | stays |
| `toolchains/schemas` | contract | root | none yet | merge into the consuming packages' `schema.cue` when: still unconsumed at the next wave end |
| `cmd/*` (8 tools) | tooling | root | the gate, releases, scaffolding | stays; not library surface |
| `examples/*`, `scenarios/*` | example | root | readers; the composition proofs | stays; not library surface |
| `fathom-pack/` | contract | root | every consumer via fathom guards and layers | stays |
| `docs/project-intent/`, `.agents/skills/intent-*` | tooling | root | every repo in the fleet | stays; inbound from adopters when: an adopter's `refresh` finds a rule the practice lacks |

Inbound: fathom doctrine laws become primitives here (heartbeat `CheckExistence` was
"promoted from fathom doctrine law 7"). Conjure's CLI kit and structured-edit engine are
named for app-kit, not here (Conjure ADR-010); nothing from Conjure is expected inbound.

## Dependency direction

Foundations import stdlib and sibling foundations; kits import foundations; runtimes,
surfaces, and toolchains sit above kits; `cmd/`, `examples/`, `scenarios/` consume
everything; nothing imports `cmd/`. The root module cannot import a nested module (I1
makes it impossible). Enforcement: stated (I13); a `go/parser` test is the cheap upgrade
(`questions.md` Q6).

## Adding, promoting, extracting, retiring

- **Add**: junk gate (contract, `-race` tests, non-guarantees stated, named consumer);
  `TRACKING.md` row; `compat.md` row; `task docs-index`; `CHANGELOG.md` line. Read two
  sibling packages first (P9).
- **Promote**: `experimental` → `stable` is one-way, recorded in `compat.md` and
  `CHANGELOG.md`, and requires a real consumer through `downstream-check`.
- **Extract**: nested module first (`go.mod`, `go.work` `use`, own tag cadence, no
  imports from sibling kits); a repository split only when the map's trigger fires.
  Nothing else is built for it.
- **Retire**: zero consumers verified (`downstream-check` plus grep), `CHANGELOG.md`
  Removed, `TRACKING.md` `[-]` with disposition, `compat.md` row dropped.

## Ecosystem

- Downstream: fathom, throughline, growing-gardens-bi, checked by `task
  downstream-check` against kit HEAD (fail-open when a sibling checkout is absent).
- Upstream: fathom (doctrine laws → primitives; guards and layers ship back through
  `fathom-pack/`); app-kit (design decisions and the UI stack).
- Shared decisions inherited: app-kit `D2` (Go + CUE only, no npm), `D17` (multi-target
  layered kit), `D18` (cli-kit), cited by repository name; this repo's own D-series is
  `decisions/README.md` and is unrelated in numbering.
