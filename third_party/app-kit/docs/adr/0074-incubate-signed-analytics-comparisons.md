# ADR-0074: Incubate signed single-dataset analytics comparisons

- **Status:** Accepted
- **Type:** architecture
- **Date:** 2026-09-22
- **Extends:** ADR-0056, ADR-0069 and ADR-0072

## Context

The resumed core uplift reaches ADR-0069's missing-critical-task trigger:
`Dataset` and `WriteCSV` accept signed observations, but `RenderBars` intentionally
refuses them because the low-level primitive clamps negatives. Receipts already
needs signed comparison geometry. Its paired periods, currency calculation and
detail links remain app-owned; it is a need witness, not automatic adoption proof.

## Decision

Add experimental `analytics.RenderDivergingBars(Dataset, viz.BarsOptions, time.Time)`.
Keep existing `RenderBars` and `viz.Bars` contracts and output unchanged. Admit one
dataset and reporting window, no exported types, multi-series engine or app policy.
Reuse the same validation, truth context, readable table, role meanings and CSV.

Center zero within a symmetric domain derived from the largest absolute rounded
present value. A positive `Max` declares that absolute extent and must not clip any
rounded observation. All-zero selections use an explicit one-unit extent and zero
marks; missing rows have no mark. Label the domain and values directly. Sign does
not select a token role or imply improvement: app-authored meanings remain binding.

Use deterministic size limits and conservative label/value-space admission. When
space cannot preserve readable marks and labels, explain the table-only fallback;
never silently truncate or misrepresent an observation. Preserve native scrolling,
strict no-script/style CSP and token-based light/dark presentation.

Expose a synthetic Gallery specimen and matching CSV, and extend the existing
experimental analytics capability rather than create another catalog or server.
Changed selected capability metadata requires review of v2 pins; no consumer pin
or source mirror updates occur automatically. Rebuild selected CSS after adoption.

## Evidence and limits

Require independent signed/zero/missing geometry expectations, clipping refusal,
rounding and table/export agreement, hostile text, fallback and legacy regressions.
Browser checks cover the declared fixture, narrow/wide layouts and both themes,
including keyboard access and export. Record exact inputs and unverified layers.
Gallery integration is not consumer promotion, human usability, production-data
correctness or full analytics/core certification. No new runtime or dependency.

## Revisit when

- A named consumer cannot complete its task without paired series, extra dimensions
  or richer navigation; assess its seam before growing this API.
- Two independent consumers demonstrate the permanent-promotion bar.
- A measured layout/contrast/performance failure defeats these simpler constraints.
- Chart, table and export disagree, or an old renderer contract changes.
