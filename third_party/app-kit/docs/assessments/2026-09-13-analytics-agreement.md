# Analytics agreement: Growing Gardens BI

Source/fixture audit and subsequently owner-approved narrow repair, 2026-09-13. Evidence for
[G3](../intent/scope.md#g3--make-analytics-a-decision-tool-of-exceptional-quality), not a
consumer migration or validation of live business records.

## Confirmed mismatch

At consumer HEAD `7503dd1c270bf6ceb72d42c742f507b9513db0dc`,
`internal/finance/engine.go` adds only `pending` events to the pending-funds overlay.
DES accrual and projected receipt events are informational: `Liquid: false`, but
not recorded pending funds. `internal/xlsx/ledger.go` instead adds **every** non-liquid
event to its cumulative pending overlay. Forecast and spreadsheet can disagree.

Synthetic opening cash 100 and a DES accrual 10 produce forecast overlay 100 but
spreadsheet overlay 110. Adding the weekly projected DES receipt makes the exported
overlay 120. Recorded pending funds 25 correctly produce 125 in both paths; that
positive control must remain intact. The kit must not prescribe finance policy:
the consumer's current calculation is the contract being checked.

## Narrow candidate and installed repair

[ledger-fix.patch](fixtures/ggb-overlay/ledger-fix.patch) changes only the pending
accumulation branch to `else if e.Kind == "pending"`. Informational rows remain;
the live Excel cash-balance formulas remain. This candidate aligns export with the
existing engine, without changing engine policy, dates, UI or customer data.

[overlay_agreement_test.go.txt](fixtures/ggb-overlay/overlay_agreement_test.go.txt)
holds the complete synthetic regression test as inert review material. The `.txt`
extension prevents importing consumer dependencies into app-kit. It belongs in the
consumer's `internal/xlsx` test package, not in a shared finance package. The owner
subsequently approved this exact finance/export-only scope; installation is recorded below.

Test `TestAppKitSyntheticPendingOverlayAgreement20260913` fixes the clock at
2026-09-07 UTC and exercises nine cases: cash-only, normal tuition, DES accrual,
DES accrual/projected receipt, recorded pending, cleared funds, expenses/draws,
mixed pending/DES events, and the 30-day export cutoff. Independent literal fixtures
check event semantics, daily forecast totals, actual XLSX rows, inflow/outflow,
every live cash formula and its evaluated result, and pending-overlay values.
No seed store, real records, network, wall clock or running consumer service is used.

## Reproduction and observations

The Go overlay mechanism injected the test into the consumer package without writing
there. The fixed overlay additionally substituted the one-line candidate ledger.
Both used `GOWORK=off GOPROXY=off GOTOOLCHAIN=local`, `-mod=readonly`, and the same
test fixture. `go test ./internal/xlsx -run '^TestAppKitSyntheticPendingOverlayAgreement20260913$'
-count=1 -overlay=<manifest>` is the focused command shape; add `-race` for race checks.
Overlay manifests map absolute original paths to temporary replacement files.

| Source under test | Observed result |
|---|---|
| Original ledger + regression | 6 controls pass; 3 DES cases fail with the documented overstatement |
| Candidate ledger + identical regression | all 9 pass; focused race repeat passes (1.513s); package vet passes |
| `git apply --check` against original consumer | patch applies cleanly; **not applied** |

Artifact SHA-256 values:

```text
ledger-fix.patch
4b2c7c07164f4fd8fb56d002f57bddfc3abca2569ba35b4afadeeb66cb625286
overlay_agreement_test.go.txt
f2621dfa2cc7838073dc43f738dafd930690fdb4d501535dcd7f6b9eca59a496
```

Audited originals: ledger SHA-256
`9c516b4847d6f81a96f99202e7aa6e561f4e6a72698b0fc908c80e9e8581d1cb`;
engine `e61b003f935ba6d709461e039e2f9f5e71b44c8e05824c7ffdab8b4bc415cb70`.
The consumer's five modified UI/CSS files and untracked `render_test.go` were unchanged;
finance/export/module files stayed clean. Recheck source identity before applying.

## Disposition

The owner approved the narrow fix and tests. `internal/xlsx/ledger.go` now accumulates
only recorded pending events; `internal/xlsx/overlay_agreement_test.go` contains the
unchanged independent nine-case fixture. Before the fix, three DES cases failed and
six controls passed; afterward all nine passed. `go test ./internal/xlsx ./internal/finance
-count=1` and the corresponding `-race` command passed. Existing UI/CSS edits remained
untouched; no live store, data, deployment, module change or commit was involved.
[INC-8](../intent/incidents.md) is closed by this regression, not by a shared finance policy.
Shared markup checks alone cannot prove analytics: each app tests its own derivation,
table and export. Browser, live-data, spreadsheet-client recalculation and full
two-consumer adoption/task-performance proof remain unclaimed.
