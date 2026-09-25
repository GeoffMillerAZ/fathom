# P11 synthetic analytical task fixture

The first-party fixture, projection, export and their mathematical tests moved to
[`apps/receipts`](../../../../apps/receipts/README.md) under
[ADR-0038](../../../adr/0038-runnable-receipts-reference.md). This assessment pointer
retains the earlier projection evidence. The runnable authored reference is not vendor
data, a port of Dashboard2, or a reusable financial backend; original fidelity stays separate.

`apps/receipts/fixture.cue` contains literal receipt/refund facts in integer cents and an independent
expected-outcome oracle. August supplies comparison data; September, October and
November represent complete, missing-source and partial-period selections. Observation
and evaluation times are fixed. These are synthetic historical fixtures, not live data.

From the app-kit root, using its existing pinned Go tools:

```sh
go tool cue vet -c apps/receipts/fixture.cue
go test ./apps/receipts/...
```

The example projects app-owned facts into the existing `analytics.Dataset`; CSV uses
`analytics.WriteCSV`. It does not add a catalog entry, runtime dependency, service,
shared selection framework or public metrics API. Do not import the example's private
business model into production. Real apps own facts, currency policy, authorization,
selection, observation and arithmetic; the kit validates declared presentation meaning.

Important counterexamples: Cedar has negative net receipts in September; Atlas has a
known zero in October while the other products have missing inputs. November's known
signed subtotal is not a lower bound and must not produce a full-month comparison.
The nonnegative bar renderer's refusal is intentional, not a failure to suppress.
Signed geometry and accessible chart interaction still require their own adapter proof.

`buildEvidenceCSV` composes receipt/refund/net evidence in one deterministic 35-column
CSV: current/previous series, selected product, measure, source/window/freshness and
the shared writer's values and declared role/meaning (ADR-0056; historical receipts used33 columns).
Measure completeness is evaluated after selection; a known
receipt survives an unknown refund. [ADR-0037](../../../adr/0037-share-internal-analytics-csv-sections.md)
shares internal section validation with Gallery, not domain arithmetic or a public API.
The reference now owns the download route and signed renderer; that later evidence
belongs in its README, not retroactively in the earlier projection result below.

No browser or benchmark runs follow from these commands. `task receipts:run` launches
the synthetic task at `http://127.0.0.1:8766`. Gallery `/analytics` remains a separate reliability
journey, not a new P11 port. Source/native and authored-task evidence must stay distinct.

## Observed gate — 2026-09-14

The focused package tests and final `go test -race` pass, as do scoped `go vet` and
CUE validation. The selection matrix is 36 overview plus 36 representative detail
cases. Additional checks compare both current/previous detail CSV rows against their
overview rows, preserve source/window/freshness, test positive/negative one-cent values,
and retain the existing nonnegative chart renderer's refusal. No shared renderer changed.

Independent contract review added receipt/refund expectations that catch equal errors
hidden by an unchanged net, explicit freshness boundaries and tied-decline semantics.
Code review fixed detail projections initially retaining whole-channel export rows.
The source-preservation check now serializes actual values, not pointer addresses;
the CSV reader asserts the real header/rectangular rows and rejects duplicate IDs.
These are synthetic projection checks, not complete P11 or consumer proof.

The consolidated exporter independently checks literal CUE receipt/refund facts and
derived net values for all 36 overview plus 36 representative detail selections,
including both periods, measure-specific completeness and detail/overview equality.
Tests preserve missing versus zero, signed cents, schema/order, exact-once spreadsheet
escaping, deterministic bytes and source immutability. Focused race tests, vet and CUE
validation pass. Gallery's extraction separately preserves all 144 actual exports:
length-framed corpus SHA-256 `f8747c6ec1f964bb8cb6d9af9a6def1ee94612c1f3ae4edcbd3b3962c9279afe`
before and after; `TestBuildAnalyticsCSVAllSelectionsDigest` carries the regression.
