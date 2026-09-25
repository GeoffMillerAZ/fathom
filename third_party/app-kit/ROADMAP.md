# app-kit roadmap

Outcome gates, not dates. [Scope](docs/intent/scope.md) is the only active goal register
and owns Done when criteria. [Vision](VISION.md) sets the bar; the
[assessment](docs/assessments/2026-09-13-app-kit.md) explains the baseline.
ADR-0019/0020/0021 replace the old sequence that blocked UI work behind an unbuilt spine.

## Current priority: land, then improve as needed

[ADR-0072](docs/adr/0072-stabilize-and-resume-on-demand.md) replaces the open-ended
implementation sequence with bounded stabilization and demand-driven continuation.
Do not complete every remaining experiment as a prerequisite for stopping this campaign.
The [resumption guide](docs/design/stabilization-and-resumption.md) owns the six
packets, exact evidence/spec pointers, activation triggers, budgets and stopping rules.
The owner resumed bounded **adoption/MCP/contribution, analytics, reusable UI/themes
and catalog/core** work. [Current scope](docs/intent/scope.md#current-priority-stabilization-then-demand-driven-work)
owns active priorities and consumer boundaries; consult it before selecting a packet.
[RAC CSV](docs/assessments/2026-09-22-analytics-adoption-integration.md) and
[Fathom theme-guide](docs/assessments/2026-09-22-fathom-guide-only.md) receipts record
their scoped working-tree integrations, not consumer commits, releases or full
acceptance. Human A/B remains owner-deferred. This roadmap grants no new consumer
write authority, and Conjure evidence does not retire the selected-app gates.

| Priority | Work | Exit / activation |
|---|---|---|
| Baseline | Coherent source handoff, truthful prepared/unexecuted evidence, agent/MCP routing, focused checks | [Handoff verified](docs/assessments/2026-09-17-stabilization-handoff.md); no promotion or release inferred |
| On a real need | One catalog/core improvement, theme guide, analytical journey, consumer propagation or native task | Named adopter/task or explicit owner-funded maintenance batch; fixed acceptance and cost limit |
| Later options | Additional platforms, component families and specialized runtimes | Existing future promotion triggers; no speculative build orders |

Car Assistant and Growing Gardens BI adoption is on hold under
[ADR-0032](docs/adr/0032-defer-car-assistant-and-growing-gardens-adoption.md).
No writes or repeat prompts for that deferred pair. The owner separately selected
Resume-as-Code and Fathom UI work; this does not complete adoption evidence.

ADR-0069 retains five starter areas and a
tested [JIT enhancement path](docs/design/just-in-time-conversion.md). The twelve
historical pilots keep their evidence; remaining variants and deep optimization are
demand-led or owner-batched, not current prerequisites. [Versioned standards](docs/design/converted-component-standard.md)
and a CUE inventory distinguish implementation, gate evidence and consumer adoption.
No deferred failure becomes a pass. [G2 shell behavior/adoption](docs/assessments/2026-09-16-shell-acceptance-matrix.md)
and [G5 terminal interaction/adoption](docs/assessments/2026-09-16-terminal-acceptance-matrix.md)
are accepted; broader analytics, guide, core and desktop acceptance remains open.
The last weighted estimate earns
no extra credit from this priority change.

ADR-0029 retains personal-site identities and app-owned living guides. Presets and the
local experimental loop exist under ADR-0031; remaining consumer and acceptance work
resumes through a selected packet and its current evidence. Keep optional neon
separate from the palette and font/density/layout choices separate from domain behavior.

## Each resumed slice

Choose one useful task, review the existing implementation and receipts, freeze the
applicable gates and budget, verify that task and return its reusable improvement.
Do not silently expand to every remaining component or another consumer.
Each slice must leave a useful artifact even if the next one is deferred.

ADR-0033 makes browser core tasks first-class for selected Wails profiles. G6's
[parity matrix](docs/design/web-desktop-parity.md) names local/remote topology, native
exceptions and independent browser/macOS proof. No remote deployment or fleet rewrite is implied.

## Promotion and retirement

ADR-0017/0018 govern layer-specific promotion; ADR-0019 admits bounded experiments.
Do not raise a recipe into a permanent configurable component until its consumers
prove the boundary. Repeated divergences set priority. Unused abstractions follow the
recorded retirement condition rather than accumulating indefinitely.

## Parked work

[Future directions](docs/intent/futures.md) holds CA/GGB adoption, native mobile, WASM acceleration,
large-data renderers, distributed sync and native framework packages with promotion
and drop conditions. Old platform and Gallery designs retain useful detail but are
subordinate to current profiles, scope and implementation evidence.
