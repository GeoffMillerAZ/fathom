# Runnable analytics comparison pair

Status: automated preparation passes; human usability results remain pending. This
closes the missing runnable-pair prerequisite, not G3 or general UI certification.
The [protocol](fixtures/analytics-usability/README.md) retains its fixed task and
answer key. No consumer, production dataset, installed browser profile or kit UI
implementation changed during this preparation.

## Controlled comparison

Private root: `/Volumes/m2ext/appkit-evidence/appkit-analytics-pair.u19A9Z`.
`prepared/receipt.json` records two independent 1,268-file first-party source copies.
A substitutes only the retained prior `apps/gallery/analytics_templates.go`; B keeps
the current composition. Both use the same current runtime, data, shared shell and
pinned Tailwind 4.3.0 compiler. CSS is rebuilt separately and is byte-identical across
A/B; the complete source inventories differ only in that template.

This is a **reconstructed-composition comparison**, not historical runtime replay.
The original selected-source packet did not contain a runnable historical closure.
Six preparation commands pass: snapshot, extraction, each CSS build and each Gallery
build. The Go builds use a local module proxy and private module cache; this is not an
OS-sandbox or fully archived Go/browser dependency-closure claim. One build per variant
is not a two-cold-build reproducibility result. Neither binary is a consumer release.

| Retained artifact | SHA-256 |
|---|---|
| `prepared/receipt.json` | `1667a6d42b7c663ebd3e14e9ab41f5cabb83a267a837d7b22c9e53bc67e4e5d8` |
| A Gallery binary | `7374819296b5df8470c867a9bf07fe3e7583f90d16f5cfff115a8286f3ad62f3` |
| B Gallery binary | `0d5319c91f72fb1cd44147c24349cab6227dfeba80b96508d208e7292bd7d4ef` |
| Each generated CSS | `8a2bc5884b90b664db4bb8793f33f78a9f85bf50f1737ce565b88def3a7d0d2d` |

## Executed task proof

Final run: `probe-exact-values.pqHocB/browser-run/` under the private root. Its nested
browser evidence is `browser/analytics-usability-pair-Uvqh6w/`.
Chrome 152.0.7977.84 and Playwright 1.62.1 ran eight fresh, script-disabled,
cache-disabled cases: A/B × 390/1440 CSS-pixel widths × light/dark, height 1,000.
All eight pass. Navigation uses native keyboard actions, not injected value changes.

- Select August 2026, API services, no comparison and partial data.
- Verify exactly Gateway and Search, values **At least 4** and **At least 2**, both
  partial/lower-bound counts, with the visible full-month limitation.
- Open Gateway detail, retain selection/theme, and verify only Gateway's **At least 4**
  with its partial state and lower-bound explanation.
- Return with all four selections and download the overview CSV, not detail CSV.
- Check its ordered 33-column schema, exact two service/value rows, August UTC window,
  selected parameters and partial context. All eight downloads have identical bytes.

Five separate mutations of the actual CSV validator are refused: renamed header,
reordered header, wrong window, wrong value and missing row; an unmodified synthetic
control passes. These are validator controls, not transport/parser mutation tests.
Sixteen full-document captures and raw target dimensions are retained. A still has
20px summaries and 17px table links in all four cases. B's sampled controls meet the
44px dimension check, but target measurements are observations, not a quality gate
in this semantic-readiness harness. Baseline quality failures remain failures.

Scoped screenshot review inspected A/B 390-dark overviews and 1440-light Gateway
details: selected values, coverage explanation and source context remain readable,
without observed overlap or clipping. The narrow table intentionally scrolls its
state column horizontally. B's added renderer link wraps without collision. This
four-capture review is not a complete visual/accessibility audit of the matrix.

The browser guard observed only the allowed loopback GET routes and exact CSS bytes,
with no blocked requests. All eight owned browser PIDs were proven absent before
their temporary profiles were removed. Both Gallery processes exited zero and their
ports refused connections after shutdown. These are local task/lifecycle results,
not proof of all browser traffic containment or machine-wide process cleanup.

| Final receipt | SHA-256 |
|---|---|
| `run-report.json` | `346c624398f101b7b8e4558277b334b6838bdf624eafb6d227fdb0d6c44fc8bf` |
| Nested `receipt.json` | `b77c795d192d0e8e366bd2a4c9c53a5abf83c162375278a0280750d2d72abce4` |
| Each downloaded CSV | `d91a29b9bbd31d96f4227ca6c8eceef395bc3d98cfa0ef8cf1d1c960ff5a4712` |

## Failures and oracle changes retained

Every executed runner/configuration was frozen before execution. No failed packet
was edited or replaced, and the A/B binaries and CSS remained unchanged across runs.

1. `browser-run/`: all eight cases failed because the probe read the section's heading
   rather than its explanatory paragraph. Nested receipt SHA
   `8023e199e2e8653e4d6e060f15e428664bc46edd90e5bc41ee0f2d6e02c19281`.
2. `probe-retry.Rjq4da/browser-run/`: all eight failed when focus instrumentation raced
   native document navigation. Correct paragraph values were observed in six cases.
   Nested receipt SHA `4b17e33417921c8cec4ad41291978a757d193efff2ba734516a61637fa689893`.
3. `probe-navigation.mFWgWm/browser-run/`: 8/8 pass after awaiting the pre-armed expected
   destination before inspecting focus. Nested receipt SHA
   `1e4d3455b9bb16289599620111227a4d60200efe9870542055e6987ed1b0c6be`.
4. Final run above adds explicit refusal of extra overview rows and verifies the detail
   value/state/explanation. This strengthens the oracle; it is not an unchanged-oracle
   replay. It passes 8/8 without changing the app artifacts.

The maintained entrypoints are [prepare.mjs](fixtures/analytics-usability/prepare.mjs),
[run-pair.mjs](fixtures/analytics-usability/run-pair.mjs) and
[verify-pair.mjs](fixtures/analytics-usability/verify-pair.mjs). The protocol includes
replay and manual staging instructions. No test servers remain running.

## Remaining acceptance

Recruit a participant, use neutral A/B labels, record order/assistance and manual
task time/errors as predeclared. With one participant, report descriptive observations
only. Automated navigation timings are diagnostics, not human speed or error rates.
This fixed pair does not establish full accessibility, performance, other analytics
states, consumer acceptance or the entire historical redesign's benefit. Existing
performance and broader-state receipts remain separate; G3 stays open.
