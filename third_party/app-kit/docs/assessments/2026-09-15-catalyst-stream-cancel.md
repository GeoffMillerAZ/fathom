# P10 stream-cancel diagnostic: no observed prototype calls

Follow-up to the [phase diagnostic](2026-09-15-catalyst-payload-phases.md), under the
[separate diagnostic contract](../design/catalyst-settings-performance.md#diagnostic-and-delivery-boundaries).
This is an instrumented failure investigation, not a performance sample or source defect attribution.

## Observation and provenance

Private directory: `data/design-assets/p10-payload-fefBfM`.
Plan SHA256: `8c0d3faa4efbc1ebfb19295b791e06be20456c7452e12734bbd71fbd8d8101cf`.
Receipt SHA256: `8e3f4ccc679139b2436c0d51654ff30bab79bacfb0fd7b07211d2cd73760ba3f`.
Sample SHA256: `fe173d0d7b401fb681ec3dcffc1640fa62983ebc55526ccbfdc8646248871019`.

The explicit `--diagnostic-stream-cancel` run planned at most five source-only
1440px/light cold observations. It stopped after the first failed observation;
four remain not run, exit1. The unchanged source listener was PID55043 at55516.
Target PID46237/binary was attested but not navigated. No source prefetch was disabled.

| Observed fact | Result |
|---|---|
| Form controls and fonts | ready; ten editable controls |
| Retained requests | 58 |
| Failed responses | 23 HTTP200 `text/x-component`, `net::ERR_ABORTED` |
| CDP cancellation flag | true for all23; no supplied block/CORS reason |
| Request purpose | all23 carry `rsc:1` and `next-router-prefetch:1`;20 have nonempty segment-prefetch values |
| Explicit policy refusals | 0 |
| Wrapped stream/reader cancel calls | 0; observer installed,0 dropped |
| Resource bodies for canceled responses | unavailable; no invented digest/byte totals |

First/last failures arrived at collector monotonic896.198/920.398ms, before
context-close-start1142.537ms. This run's explicit close cannot cause its already
received failures. Browser and collector clocks remain separate; these are not task latencies.

The init script wraps only `ReadableStream.prototype.cancel` and
`ReadableStreamDefaultReader.prototype.cancel`, preserving receiver, arguments,
return/throw and descriptors. It records bounded call metadata, not reason/body/request
identity. Installation changes function identity and adds overhead; this run is never
merged into passive samples. Zero observed calls is not proof that nothing canceled:
browser internals, other APIs/realms and discarded response bodies remain outside this probe.

The recorded initiator points to the selected Next chunk's fetch path. Local inspection
found conditional clone/reader cancellation paths in the installed Next runtime, but
the diagnostic did not observe their public prototype calls. Their presence is not
causal evidence. No source bug, harmless-cancellation exception or complete-payload pass follows.

Runner SHA256: `117aa9067a5c9b036629c3a4862c975ba286dc727d173b83aa9326b9fdb35c94`.
Payload helper: `b14f7f938acae35e4f6205b0c6dd47a7e734a7be834a9691cb0df70820277937`.
Observer: `efc5873a47abcab887b7519bf252226375ade93f2e93d841bf286c43d4be6d2e`.
Later exported collector helpers change the current runner hash; this receipt binds
only the runner above. Chrome152.0.7977.84, Playwright1.63.0, Node26.3.0,
Apple M4/arm64,32GiB, Darwin25.6.0; loopback, no shaping, reduced motion.

## Checks and next action

Sixteen source-only tests passed before acquisition:11 payload fact/refusal checks,
three collector lifecycle checks and two stream-observer mock tests. They distinguish
missing/false cancellation facts, bound purpose/initiator metadata and verify exact
delegation/refusal/restoration. They are not native-browser cancellation coverage.
The observer tests join `task check`; failed resources remain fail-closed independently
of the error arrays. Continue with a bounded source/response-lifecycle investigation
only when it can distinguish the remaining causes; repeating this same probe adds no proof.
