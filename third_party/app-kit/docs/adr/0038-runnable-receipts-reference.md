# ADR-0038: Exercise the analytical contract in a runnable receipts reference

- **Status:** Accepted
- **Type:** architecture
- **Date:** 2026-09-14
- **Decision drivers:** an inspectable task, retained numerical oracles, bounded ownership

## Context

The P11 authored task has tested integer-cent projections and consolidated CSV but
no runnable interaction. Existing bar APIs either refuse or clamp negative values;
the other signed charts model different quantities. The purchased Dashboard2's
13-panel composition is not this synthetic cash-flow domain model.

## Decision

Move the first-party fixture, projection, export and their unchanged mathematical
oracles from the assessment directory into `apps/receipts`. Keep an assessment pointer;
update provenance text to the actual fixture path. This is a bounded reference app,
not a reusable financial backend, public metrics API or fourth typed conversion.

Build native GET period/channel/comparison/theme selection, linked product detail,
context-preserving back navigation and actual CSV download over one validated model.
The standalone command binds loopback; there are no stores, providers, mutations,
live data, remote deployment or client scripts. Missing and partial values remain
unknown or measured subtotals, never fabricated monthly totals or lower bounds.

The app owns a small paired, signed SVG representation with a common zero-centered
scale, visible values and keyboard-accessible detail targets. Reuse shared tokens,
shell/navigation and provenance framing. Preserve `analytics.RenderBars`' negative
refusal; do not add a generic chart engine or promote this app renderer to a permanent
kit API before consumer evidence earns that boundary.

The complete original MatDash composition remains a separate source-faithful task.
This app may inform its reviewed usability adaptation but cannot certify its fonts,
assets, panel fidelity, native runtime or interactions. Keep the original available.

## Alternatives considered

- Import documentation fixtures from production packages: reverses ownership and
  leaves the example without a coherent launch/build surface.
- Duplicate the model into Gallery: creates independent arithmetic and oracle drift.
- Clamp, take absolute values, or substitute a table for signed geometry: changes
  meaning or omits a required capability. Tables supplement the chart.
- Promote a new shared chart engine immediately: one bounded task needs a renderer,
  not a new platform abstraction or client dependency.

## Consequences

The reference becomes runnable and independently testable while the old mathematical
tests move with it. Assessment command/source paths change and are documented; no
released API or catalog ID changes. Its HTTP, browser, payload and keyboard proof
must be earned under the existing P11 task budgets, not inferred from this decision.

## Revisit when

- Two real consumers need the same signed presentation contract.
- The three-product task exceeds its declared interaction or payload budget.
- A browser cannot preserve signed geometry or native keyboard inspection.
- Original/adapted lineage becomes ambiguous in discovery or usage.
