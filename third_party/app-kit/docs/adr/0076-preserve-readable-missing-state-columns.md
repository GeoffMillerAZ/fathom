# ADR-0076: Reserve readable width for missing-data explanations

- **Status:** Accepted
- **Type:** ux
- **Date:** 2026-09-23
- **Supersedes:** ADR-0075 only where its byte-preservation wording would prohibit a separately reviewed layout repair

## Context

At baseline `8cc2a7c`, the shared analytical table compresses its missing-reason
column to 59.45px and produces 297px rows at 320/390px viewports. Native scrolling
can reach the full text, but reading 65 characters should not require that height.
The [frozen baseline](../assessments/2026-09-23-analytics-state-width.md) fails
the prospective width/row-height criteria. This fires ADR-0069's missing acceptance
and ADR-0075's wrapping/compatibility revisit triggers.

## Decision

Reserve a 12rem minimum for missing-reason cells in the shared table, retaining
wrapping, complete text and the existing native horizontal-scroll region. Do not
widen all-present datasets, hide explanations, introduce scripts or replace native
table semantics. Verify mixed rows, keyboard travel, exact data/CSV and both themes.
The minimum is relative to the app's root font; no arbitrary literal color is added.

This independent repair intentionally changes missing-row HTML, with or without
`DetailHref`. ADR-0075's byte identity remains true for its frozen link-introduction
delta, not a timeless prohibition on explicit layout bug fixes. Link policy,
validation, app-owned routing, public signatures, data and CSV remain unchanged.
Historical failures and original/candidate bytes remain available; no old result
is relabeled and no quality oracle is weakened.

## Compatibility and alternatives

Rebuild selected L3b CSS and review HTML snapshots plus selected capability pins.
This is an observable markup/CSS compatibility change, not consumer propagation.
Extra horizontal travel is accepted only with proven keyboard reach/return.
`nowrap` would create unbounded width; a global table-width increase needlessly
affects complete data; truncation or a disclosure hides essential truth context.
No consumer, installed MCP binary or dependency changes occur automatically.

## Revisit when

- The measured minimum fails with app fonts, zoom, translations or a named viewport.
- Native scrolling cannot expose the entire reason and return to the selected row.
- A named app needs a different reading layout or measured density budget.
- Data, CSV, detail destinations or all-present output change under this repair.
