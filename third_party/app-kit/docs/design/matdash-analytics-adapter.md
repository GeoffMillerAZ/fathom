# P11 analytical composition: task contract

Status: independently reviewed task contract and [runnable synthetic reference](../../apps/receipts/README.md) with [scoped evidence](../assessments/2026-09-14-receipts-reference.md), not a converted or certified P11 target.
Governed by [ADR-0023](../adr/0023-profile-specific-conversion-and-client-compute.md),
I7/I8/I11/I12/I13/I17 and the frozen [pilot](../assessments/2026-09-13-conversion-pilot.md).
Original Dashboard2 and its native runtime remain independent reference choices.

## User and decision

A product owner reviews a monthly report on a laptop, sometimes in daylight and
sometimes in a dim room. Preserve independently selectable light/dark presentation.
They choose month/channel, identify the product contributing most to a change in
net receipts, inspect its receipts/refunds and source facts, then export that same
selection. This is synthetic cash-flow evidence, not accounting revenue, profit,
forecasting, growth causality or a production financial service.

## Original versus authored behavior

The P11 grid has 13 panels, not one coherent domain model. Its RevenueForcastChart
uses signed series and USD-looking totals without declared units or derivation.
AnnualProfit combines conversion, users and monetary labels. RevenueByProduct has
an unbound month selector and client tabs that reorder literal product rows.
Do not infer a metric contract from those names or count that as working filtering.

Two representations stay separate. Source-faithful work must retain all applicable
panel structure, fonts, assets, palette, responsive behavior and native affordances;
missing semantics remain disclosed. The authored task uses explicit fixtures below,
real month/channel selection, exact values, explanation/detail/export and truth states.
Changing labels, data, charts or panel selection is a reviewed adaptation, not fidelity.
The original 13-panel baseline and full P11 gates remain required for the port.

## Synthetic data contract

The CUE fixture under `apps/receipts/` is first-party test
data, not extracted vendor facts. Three products have fixed IDs and app/mobile channels.
Each monthly product fact contains nonnegative receipt/refund cents or explicit null
with a missing reason. Net receipts = receipts minus refunds, calculated in integer
cents before bounded presentation as USD with two-decimal precision. The existing
CSV writer omits insignificant trailing zeros; negative net is valid.
No exchange rates, cross-currency sum, implicit thousands or accounting claims.

Selected periods are half-open UTC calendar months; August 2024 is comparison input and
September/October/November are selectable. A snapshot names coverage end and source
observation separately. The fixed evaluation clock is 2024-12-05 12:00 UTC; freshness
uses a 24-hour maximum source age; exactly 24 hours is fresh, older is stale.
A stale variant uses the fixture's explicit older observation and changes age only.
Partial November covers November 1–20 inclusive, never extrapolated to month end.
Its analytics dataset window ends at the exclusive coverage end, November 21;
the selection still names November and the coverage note discloses the incomplete
calendar month. Do not relabel that observed interval as a full month.

Keep a row when either amount is missing, but net is missing, not zero. Each period's
exact total requires its full temporal coverage and every selected net known. Delta
and largest-decline attribution require both periods to meet that condition.
The sum of available signed values is only a
known subtotal: unknown refunds can move the final result either way. It is not a
lower bound. Compare=none removes comparison data and claims. Channel selection
precedes aggregation; detail narrows by stable ID without changing source/window.
Largest decline means the most negative product delta, not the largest magnitude
of either sign. Return all tied IDs in product order. Flat/all-increasing selections
have no declining product. This describes contributions, never causal attribution.

September all-channel expected net is USD 1,550.00 versus August USD 2,300.00:
delta -750.00, of which Atlas accounts for -600.00 and Cedar -150.00. Beacon is
unchanged. App-only totals are 650.00 versus 1,400.00; mobile is 900.00 in both.
October contains a complete zero for Atlas and missing inputs for Beacon/Cedar.
November has measured-to-date values but incomplete monthly coverage. Neither
permits an exact monthly total, comparison delta or ranked cause claim.

## Implementation boundary and interaction mapping

Use existing `analytics.Dataset`, validation and `WriteCSV`; preserve separate current
and previous windows. The initial executable fixture is app-owned projection/example
code, not a new kit metrics API. Do not copy Gallery's reliability backend into it.
The reference routes use canonical native GET selection/detail/back/export and preserve
the selected theme. The completed evidence export must retain receipt/refund/net
amounts, product identity and selection/source/coverage context. Net-only CSV checks
are a useful projection gate, not that completed consolidated export.
Invalid, duplicate and filtered-out selection needs explicit refusal
and recovery. No JavaScript is needed for those navigation tasks; no fake live state.

The signed source series and negative synthetic Cedar value cannot be passed to
`analytics.RenderBars`, which deliberately refuses negatives. Preserve that refusal.
The private authored SVG renderer follows [ADR-0038](../adr/0038-runnable-receipts-reference.md);
its paired common scale and native detail links need their own geometry/keyboard proof.
A faithful source adapter still needs source parity. Do not abs/clamp values,
drop the negative row, substitute a table and claim chart conversion, or add a generic
chart-engine abstraction. Accessible tables supplement the chosen chart.

## Gates and limits

The consolidated CSV uses one row per period/product/measure, ordered current then
previous, product order, then receipts/refunds/net. Its eight selection/measure columns
precede the shared writer's 17 context and ten value columns (35 total). Empty
numeric cells remain missing, true zero stays numeric, and all USD values retain
two-decimal precision. Raw receipt/refund completeness is evaluated separately from
net completeness; a missing refund must not erase a known receipt. Channel/detail
selection applies before every measure. Shared cells retain their original spreadsheet
protection. [ADR-0056](../adr/0056-preserve-analytics-role-meaning.md) appends role/meaning
without changing existing columns; update exact-width readers. [ADR-0037](../adr/0037-share-internal-analytics-csv-sections.md) owns internal
section reuse; no new public formatter or general ingestion API follows. ADR-0038
separately admits the example's bounded loopback HTTP surface.

Before UI implementation, tests must verify exact cents, filtered totals, known versus
missing zero, per-product contribution, separate windows, stale age, partial signed
subtotals, detail consistency and CSV agreement. Literal expected fixtures are an
independent oracle; modified amounts, dropped rows, zero-filled missing data and
incorrect full-month comparisons must fail it. CUE validity alone is insufficient.
Receipt and refund amounts have their own literal oracle: equal errors in both
amounts must fail even when net/delta remain unchanged.

UI proof must cover native month/channel/apply, explanation, detail/back and actual
CSV download at 390/1440px, light/dark, keyboard/pointer, 200% zoom and long labels.
Source comparison retains all original deficiencies and uses matching source/runtime/
font/assets. Separate browser tests must inspect chart values/geometry, not only page
overflow. Three-product task targets: HTML <=32 KiB, no JS for navigation, handlers
<=5ms/op and <=512KiB allocated/op locally; runtime-selection owns broader browser/
network budgets. Targets are predeclared, not measured performance claims.

The model fixture alone establishes no browser, full composition, private delivery,
actual consumer, task-time improvement or P11 certification. Keep the
twelve-case denominator and the three existing typed-adapter count unchanged.

## Revisit when

An upstream source change invalidates the baseline; a task needs forecast/accounting
semantics, more than one currency, live/offline behavior or a measured richer client;
or a chosen chart cannot represent signed values and preserve required interaction.
Review the mapping and applicable gates before changing outputs or expectations.
