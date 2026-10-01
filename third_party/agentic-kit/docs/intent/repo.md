# Repository charter

Intent about the container: why agentic-kit is one repository, what belongs in it, how
the parts relate, and which parts are expected to leave or arrive. Practice:
`docs/project-intent/monorepo.md`. Package-level classification is `docs/compat.md`;
this file carries what a lint cannot: the evolution intent.

## Why one repository

The kit's target is verified workflows with reusable lessons (VISION one-sentence
test). Keeping one gate (`task validate`), one
constitution (`cmd/constitution-lint`), one compatibility audit (`compat.md`), one
catalog, and consumer checks over the same revision supports that target. Splitting would
multiply gates, let sibling packages drift apart between tags, and break atomic
cross-package change. The root's zero-dependency boundary (I1) limits dependency conflicts;
package maintenance and composition still have a cost. Live counts belong in the catalog.

## What belongs here, what does not

- Belongs when: it supplies a reusable agentic contract, control, workflow, adapter,
  scenario, or engineering practice; it has a consumer or meets incubation decision
  0004. Root Go stays stdlib-only; package concurrency and data contracts are explicit.
- Reviewed when: an incubation expires, a consumer leaves, or maintenance exceeds
  measured value. Retirement follows compatibility and autonomy policy; needing a
  dependency requires a nested-module decision (D1), not an exception in the root.
- Never here: application `main()`s beyond `cmd/` tools and examples (I18); reference
  sources except under `_reference/`; anything org-specific (I2).

## Component map

| Component | Kind | Intent | Consumers | Evolution |
|---|---|---|---|---|
| `foundations/**` | library | root | kits; fathom, throughline, growing-gardens-bi, resume_as_code directly | stays; review unused packages under 0004; extraction follows D8 |
| `kits/agent-harness` | kit | root | `examples/agent-cli`, `scenarios/governed-docextract-hitl`, `cmd/kit-new` output; Conjure `heal` (future) | stays; flagship |
| `kits/docextract` | kit | root | growing-gardens-bi | stays |
| `kits/docextract/pdf`, `kits/docextract/xlsx` | kit (nested modules) | root | growing-gardens-bi via `docextract` | split when: a consumer outside the fleet pins either module independently, or its release cadence diverges from the root for two tags (`compat.md` §v1 record) |
| `kits/endpoint-guard` | kit | root | `cmd/endpoint-guard` sidecar | stays |
| `kits/eval-runner` | kit | root | this repo's own `task eval` (uplift D9) | stays; Stages 2–3 are `futures.md` F14 |
| `kits/expertchat` | kit | root | `examples/expertchat-service`; gke-expertchat wave | stays; own intent when a second deployment names it |
| `kits/hitl` | kit | root | `scenarios/governed-docextract-hitl`, `scenarios/sourced-prose`; growing-gardens-bi assessed | stays |
| `kits/investigation` | kit | root | `scenarios/governed-investigation` | stays |
| `kits/guardrails-pipeline` | kit | root | throughline (`app/chat/untrusted.go`) | stays; pinned experimental risk accepted under 0010 |
| `kits/governed-prose-kit` | kit | root | `scenarios/sourced-prose`, `cmd/kit-workflow`; no production prose adoption | stays; value review 2026-10-12 or next release |
| `kits/lesson-transfer` | kit | root | strict-harness regression and resume_as_code provider-family receiver | stays; bounded transfer review 2026-10-12 or next release |
| `kits/command-center`, `kits/mcp-server` | kit | root | local tests; downstream composition not established by this assessment | stays; evidence review pending, no automatic demotion or retirement |
| `runtimes/credresolve` | runtime | root | throughline (`adapters/claudeauth`) | stays |
| `runtimes/policy` | runtime | root | no consumer confirmed in this assessment | retire when: a bounded value review recommends it and compatibility policy permits |
| `surfaces/forms/cueform`, `surfaces/editor` | surface | root | `surfaces/editor` ← `cueform`; app-kit adjacent | stays |
| `toolchains/jsonschema`, `toolchains/materialization/*` | toolchain | root | `llmclient` (`OutputSchema`), each other | stays |
| `toolchains/schemas` | contract | root | adoption unverified | merge into consuming packages when: the value review establishes redundant ownership |
| `cmd/*` | tooling | root | the gate, releases, scaffolding | stays; not library surface |
| `internal/jsonwire` | private helper | root | strict prose, lesson-transfer and workflow CLI decoders | stays private; Unicode wire integrity, not a general JSON validator |
| `examples/*`, `scenarios/*` | example | root | readers; the composition proofs | stays; not library surface |
| `fathom-pack/` | contract | root | every consumer via fathom guards and layers | stays |
| `docs/project-intent/`, `.claude/skills/intent-*` | tooling | root | every repo in the fleet | stays; inbound from adopters when: an adopter's `refresh` finds a rule the practice lacks |

Inbound: fathom doctrine laws become primitives here (heartbeat `CheckExistence` was
"promoted from fathom doctrine law 7"). Conjure's CLI kit and structured-edit engine are
named for app-kit, not here (Conjure ADR-010). Lessons may arrive from any workload;
ownership of its application code does not move with the lesson (0006).

## Dependency direction

Foundations import stdlib and sibling foundations; kits import foundations; runtimes,
surfaces, and toolchains sit above kits; `cmd/`, `examples/`, `scenarios/` consume
everything; nothing imports `cmd/`. Do not rely on workspace mode to make root imports
of nested modules appear dependency-free. `TestFoundationsImportDownward` checks the
production foundation imports; shared telemetry constants now live in `telemetry/event` (I13, INC-8).
Root `internal/` helpers are private kit/command implementation, not a sixth public
library root; foundations still import only stdlib and sibling foundations.

## Adding, promoting, extracting, retiring

- **Add**: junk gate (contract, `-race` tests, non-guarantees, consumer or 0004 incubation);
  `TRACKING.md` row; `compat.md` row; `task docs-index`; `CHANGELOG.md` line. Read two
  sibling packages first (P9).
- **Promote**: `experimental` → `stable` is one-way, recorded in `compat.md` and
  `CHANGELOG.md`, and requires a real consumer through `downstream-check`.
- **Extract**: nested module first (`go.mod`, `go.work` `use`, own tag cadence, no
  imports from sibling kits); a repository split only when the map's trigger fires.
  Nothing else is built for it.
- **Retire**: value review with an explicitly scoped consumer search; authorization
  per `autonomy.md`; `CHANGELOG.md` Removed, `TRACKING.md` disposition, `compat.md` update.

## Ecosystem

- Downstream: fathom, throughline, growing-gardens-bi, racli (`../resume_as_code`,
  vendored — `racli-check` re-pins it), checked by `task downstream-check` against kit
  HEAD; missing siblings are skipped, and a skip is not a pass. Verification should use
  isolated checkouts when vendoring would mutate the consumer.
- Upstream: fathom (doctrine laws → primitives; guards and layers ship back through
  `fathom-pack/`); app-kit (design decisions and the UI stack).
- Shared decisions inherited: app-kit `D2` (Go + CUE only, no npm), `D17` (multi-target
  layered kit), `D18` (cli-kit), cited by repository name; this repo's own D-series is
  `decisions/README.md` and is unrelated in numbering.
