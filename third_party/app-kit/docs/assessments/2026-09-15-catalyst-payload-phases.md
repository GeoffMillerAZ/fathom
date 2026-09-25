# P10 payload diagnostic: cancellation precedes teardown

Follow-up to the [partial acquisition](2026-09-15-catalyst-payload-observations.md),
under the [prospective diagnostic contract](../design/catalyst-settings-performance.md#diagnostic-and-delivery-boundaries).
This resolves chronology for one new observation, not the cause of cancellation,
a complete payload/performance gate, source fidelity or target certification.

## Actual observation

Private directory: `data/design-assets/p10-payload-zstjgT`.
Plan SHA256: `95076cec0b58dfc879fd967eca9628cc748224211b2f5a3b361a3eacabdf8a8c`.
Receipt SHA256: `6ec716b9632accd9912b83878c49b9e69ec8ebb230cefe88338ba4f6b242acfa`.
Sample SHA256: `d88046bc6e5cbe096229ca3a22f188f893daad35834a9faccd963c59b4d6d9b8`.

The optional diagnostic planned five fresh-context cold source observations at
1440px/light, stopping at the first failure. It ran one, failed, and left four not run;
terminal status is `failed-partial-diagnostic-desktop-observations`, exit1.
The default360-sample acquisition matrix was not rerun or reduced.

The source form and fonts became ready. All58 admitted resources remain recorded;
22 HTTP200 `text/x-component` responses ended with `net::ERR_ABORTED`. Their missing
body/transfer totals remain unknown, not zero. There were no policy blocks.

| Ordered event | Collector monotonic milliseconds |
|---|---:|
| Form/fonts readiness observed | 900.448 |
| Resource drain begins | 900.521 |
| First cancellation received | 903.562 |
| Last cancellation received | 926.174 |
| 200ms quiet observed | 1,131.538 |
| Context close called | 1,143.657 |
| Context close settled | 1,150.249 |
| Owned CDP listeners detached | 1,150.329 |
| Value snapshot finalized | 1,150.347 |

All22 cancellations were received217.484–240.095ms before close was called. None
arrived in the context-close phase. Thus this run's explicit context teardown did
not cause those already-observed events. CDP browser-clock timestamps are retained
separately; these differences use only the collector clock. They are not user latency.
The source's reason for canceling is unproven; do not call its host broken or disable
prefetch. This observation cannot retroactively timestamp the older PjAAZ6 receipt.

## Instrument correction and checks

`payload-browser.mjs` now records ordered event receipts and readiness/drain/close
markers. It preserves resources received during close, joins pending event work with
bounded timeouts, detaches only owned listeners, then takes value snapshots and
recomputes totals. Later events after detachment are outside the observation boundary.
Warm-prime resources are retained separately from reload accounting; no warm runtime
case was exercised here. The native/source renderers and cache headers were unchanged.

The older shared error sink mislabeled network/body/drain failures as blocked requests.
Current `blocked` records explicit origin/method/WebSocket refusals; `collector_errors`
records transport/body/drain/close failures. Both refuse the sample. Resource failure
and unavailable-body facts independently refuse it, even if an error array is lost.
Old receipts remain unchanged; their generic blocked label is not policy attribution.

Twelve source-only Node tests pass: nine payload checks and three lifecycle tests.
The actual collector functions are tested with fake CDP/browser contexts: failures
before/during close, late-resource retention, aligned totals, correct final rejection,
a valid positive control and a frozen post-detachment receipt. These are synthetic
collector checks, not twelve browser task checks. Both suites join `task check`;
the full maintainer gate was not rerun for this tooling-only change.
The broader controller/zoom/focus/payload Node selection passes26 tests. Root intent
budget/link/ADR-membership/stanza checks and `git diff --check` pass; this is a structural
doctor subset, not a fresh host-wiring or nested-component audit. No Go source changed.

Runner SHA256: `871cabbf6db2d7913a4b35da06763ce0c9358cf3e248fb0441b7561feeba15fd`.
Helper SHA256: `fb37d538462f4187dbdf04e7a782d3fec0eb1ad1f2b0eb16e0875db6dc118fe1`.
Chrome152.0.7977.84 / Playwright1.63.0 / Node26.3.0; Apple M4/arm64,32GiB,
Darwin25.6.0, loopback, no shaping, reduced motion, isolated context.
The collector revalidated source PID55043/root/packet and target PID46237/binary;
this source-only diagnostic did not navigate the target. Selected source files/assets
and the target binary match the prior packet; no new binary behavior proof is implied.

## Next evidence, not a waived gate

Trace the source's cancellation path and retain partial resource accounting without
turning canceled downloads into complete payloads. A separately reviewed cached static
delivery experiment is needed: the existing read-only host deliberately sets no-store,
confirmed in live source page/CSS HEAD responses. Preserve that independently runnable
baseline. Complete the named task, RTT/offline, visual and consumer gates separately.

Later separate evidence: [stream-cancel instrumentation](2026-09-15-catalyst-stream-cancel.md)
narrows the observed API calls without attributing a cause; [cached delivery](2026-09-15-catalyst-cached-static.md)
passes HTTP checks but fails actual font-URL membership. Neither replaces this receipt.
