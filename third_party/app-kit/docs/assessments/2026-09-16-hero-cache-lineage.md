# P01 prospective cache-lineage acquisition: failed evidence gate

2026-09-16. [ADR-0067](../adr/0067-prove-isolated-warm-cache-lineage.md) and the
[prospective contract](fixtures/hero-adapter/CACHE-LINEAGE.md) remain separate from
the unchanged, failed v1 body-identity benchmark. This is not a v2 performance pass,
full conversion, target certification, consumer adoption or field-performance claim.

## Implemented and exercised

- Pure bounded cache-lineage refusal checker and raw-graph/numeric oracle. Counts,
  bytes, cache facts and resource failures are derived, not trusted summary fields.
- Separate launcher snapshots its import/test graph before execution, reruns retained
  tests, copies exact pinned historical inputs and retains available response bodies,
  raw CDP events, server traces, PNGs and owned-process cleanup.
- Unique loopback server and fresh isolated browser context per pair; two owned
  Chrome152.0.7977.84 launches,390/1440×1000, light/dark, DPR1, reduced motion,
  10/5Mbps,80ms latency,4xCPU. No signed-in profile, downloads or consumer changes.
- 71 retained pure tests passed before acquisition. Two follow-up refusal tests
  specifically preserve the newly observed incomplete-header/unmatched-event cases.

## Result: 16 numeric observations, zero accepted lineage pairs

Every observation met the unchanged numeric budgets. These measured ranges are
diagnostic only; they cannot override the failed identity/event gate:

| Measured fact | Cold: eight observations | Warm: eight observations |
|---|---:|---:|
| Directly observed uncached decoded bytes | 499,059 | 2,553 |
| Observed request starts | 7 | 7 |
| FCP | 296–308ms | 128–156ms |
| Sampled LCP | 516–532ms | 128–156ms |
| Readiness observation | 628.2–655.2ms | 157.4–172.5ms |
| Instrumented JS heap | 1,395,176–1,403,756 bytes | 2,207,616–2,229,496 bytes |

All eight pairs retained equal PNGs/geometry and expected painted Inter facts. Every
cold font body matched the pinned192,380 bytes; each warm font retained its own cache
event, no static server response and the declared unavailable-body error. Both phases
retained the same seven font response headers, including strong ETag and cache policy.

Nevertheless every warm font request exposed only `{"referer":""}`, whereas its cold
request exposed Origin, referrer, user-agent and client-hint headers. There was no warm
font extra-info event supplying those missing facts. The required observed header-map
equality therefore fails; do not fill it from the cold request or source configuration.

Each navigation also emitted an unmatched `Network.loadingFailed`, type `Other`,
`net::ERR_CACHE_MISS`, with no corresponding observed request start. In the first cold
trace it follows the actual font's finished event by about261ms. This chronology does
not establish the failed resource URL or cause. The collector retained it as unexpected;
warm diagnostics also retain preceding cold failures, rather than discarding them.

The full report has eight header-binding refusals and16 unexpected-event sample
refusals, plus the matrix's repeated16 sample refusals. No budget was widened, header
check removed, failed packet reclassified or unknown event suppressed. The cache-clear,
representation-policy and original-asset live control modes exist but were not run:
an accepted full acquisition is absent, and extra unrelated failures would prevent
the strict control oracle from claiming a clean, targeted detection.

## Retained identity and cleanup

Private root: `/Volumes/m2ext/appkit-evidence/appkit-hero-font-candidate.AGTWfHOp/hero-lineage-Py3wTx`.
No licensed inputs or rendered assets are copied into this tracked assessment.

| Artifact | SHA-256 |
|---|---|
| `receipt.json` | `dc7c80279f048eb5bb7dcb753f4c790207a5bbd704e04e74449e666fbd6cc747` |
| `snapshot.json` | `52a9b57c8d2ab62ed541fddad24b17f1566aaf9a89997ebeef40e382b68807bb` |
| `launcher-receipt.json` | `bcbf17835a8819d284a0ada21abffbf7b2003998c0526ff6183b2e7aaddc5353` |
| Retained `lineage-run.mjs` | `849246607df6cebd2db311b26f238d99c9b61d8dcc4f08679143adc37029f726` |
| Retained `lineage-oracle.mjs` | `2c10c37781e40c8a4c72715845af842c9f84d55c5104c730127dbac079b9166b` |
| Retained `cache-lineage.mjs` | `5e8efb191f35e42ead7ab8876adbcb34260ceeceb14015cfccb4eb165936a67a` |

Owned Chrome PIDs69989/70022 exited0 without a signal; all eight owned servers closed.
Independent review rehashed the snapshot, runner/input/log identities, all16 PNGs and
104 directly observed body files; the eight absent bodies are exactly the warm fonts.
Both browser PIDs were independently confirmed absent. The maintained four-suite
test command passes73/73 after adding the two incident refusals.
The frozen packet remains replayable even as maintained tests gain refusal coverage.

## Next boundary

Do not repeat this matrix unchanged or remove the refusal conditions to obtain green.
A new instrument must first establish where the unmatched event comes from and whether
the required warm representation-request facts are observable without changing caching.
If they cannot be observed, this prospective method is not available on this instrument;
any different proof requires explicit review, applicability and a newly frozen contract.
V1, the original658,700-byte failure, target/consumer gates and all other pilots remain
unchanged. [INC-47](../intent/incidents.md) records the durable refusal outcome.
