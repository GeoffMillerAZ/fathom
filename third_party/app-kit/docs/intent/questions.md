# Open questions

These require evidence during implementation, not approval to author this intent.
The owner has already authorized ambitious strategy and bounded design decisions.

| Question | Why it changes the design | How to resolve |
|---|---|---|
| Which attribution method and numeric thresholds should implement the selected app-owned-growth memory contract? | The owner chose bounded app-owned growth plus reported total browser cost in [ADR-0072](../adr/0072-stabilize-and-resume-on-demand.md); [blank controls](../assessments/2026-09-17-theme-guide-memory.md) exceed the old fixed ceiling | deferred until R3 in the resumption guide is selected; fix a new versioned contract and independent controls before acquisition; preserve v1 failures, never retrospectively change its limits |
| Do Today review and Observatory evidence tasks improve completion time/error without hiding uncertainty? | validates the selected G3 workflows beyond visual/static checks | each consumer now has PRODUCT.md and docs/app-kit.md; [RAC](../assessments/2026-09-13-resume-today-uplift.md) and [Fathom](../assessments/2026-09-13-fathom-observatory-uplift.md) record scoped baselines; compare synthetic task outcomes, preserving their distinct themes and data policy |
| What exact MatDash license/channel/tier was purchased? | determines which source/adaptations can be exposed or distributed | inspect purchase record before a distribution decision; no entitlement inference from files |
| Which apps can currently build in isolation and consume a private release pin? | determines the first release migration and delivery work | clean build plus artifact-integrity check per selected reference; report missing credentials separately |
| Which native desktop capabilities are required first? | keeps G6 focused on real shell value | prove Gallery browser/macOS parity under ADR-0033 before choosing an actual consumer's native enhancement; Windows remains deferred under ADR-0027 |

No reserved ADR IDs. A resolved question leaves this file; its decision or source
evidence remains in the owning home. Do not let this become a second backlog.
