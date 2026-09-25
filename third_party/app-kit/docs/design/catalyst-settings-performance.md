# P10 performance measurement contract

Prospective work for [P10 step5](catalyst-settings-adapter.md#implementation-sequence-and-proof),
within G1/G4's admitted conversion pilot. This is not a measurement, benchmark result,
completed acceptance gate or a new universal kit budget. Governing references:
[conversion matrix](conversion-pipeline.md#verification-matrix) and
[runtime screening targets](runtime-selection.md#deliberate-performance-experiments).
The [first partial acquisition](../assessments/2026-09-15-catalyst-payload-observations.md)
records90 usable390px cold samples and unresolved desktop-prefetch/warm-cache failures.
It does not complete this contract; no performance threshold or source baseline changed.

## Diagnostic and delivery boundaries

An optional, separately identified diagnostic isolates five cold1440px/light source
observations, stopping at the first failure. It does not change the default360-sample
acquisition plan or replace its missing samples.
Record request/response/completion/failure receipt times and readiness/drain/close
markers, ordered by sequence, on one Node monotonic clock; retain original CDP timestamps
separately. Mark context close called/settled and collection listener shutdown explicitly.
An event received during teardown does not establish why the browser canceled it.
Keep canceled and late resources, unavailable bytes and every failure in the terminal
receipt. Instrumentation must not disable prefetch or reinterpret an abort as success.
Earlier PjAAZ6 aborts remain unclassified; new timing cannot retroactively attribute them.
The [first phase diagnostic](../assessments/2026-09-15-catalyst-payload-phases.md)
records22 cancellations before close in one new observation; their cause remains open.
An additional, explicit `--diagnostic-stream-cancel` lane may observe stream/reader
cancel calls and bounded stacks. It changes native function identity and adds overhead:
never merge its results into passive timing samples or claim per-request causality.
Keep dropped/truncated observations explicit. Raw CDP cancellation/block/CORS facts and
allowlisted prefetch-purpose metadata supplement, but never replace, failed resources.
The [stream diagnostic](../assessments/2026-09-15-catalyst-stream-cancel.md) observes23 canceled prefetch responses and zero wrapped cancel calls; cause remains unproven.

The existing [read-only source host](../assessments/fixtures/catalyst-native/reference-server.mjs)
explicitly sets `Cache-Control: no-store`; live source CSS and page responses confirm
that policy. This is a rehearsal-host choice, not a discovered vendor performance
defect. The native companion/controller also use no-store. Do not rewrite these
baselines to manufacture warm-cache samples.

A representative cached-delivery experiment remains required and separate: pin a
new named listener/transport revision for each source/target while preserving the
independently runnable no-store reference. Admit only selected static assets through
an explicit URL/decoded-SHA/content-type allowlist; retain no-store for documents,
RSC/route data, mutations, errors and other dynamic responses. State cache freshness and encoding policies
before collecting; verify actual cache provenance and identical decoded assets.
Do not introduce a generic response-caching proxy, cache authority-bearing responses,
silently alter source-native routing/prefetch, or claim a whole-app speed comparison.
Synthetic refusal/identity checks and independent review precede that experiment.
No cached-delivery profile is implemented or admitted by this document alone.
The [v1 experiment](../assessments/2026-09-15-catalyst-cached-static.md) implements separate hosts and passes688 HTTP/32 exclusion checks, but fails actual font-URL coverage; it is not a warm-cache pass.

### Fixed static-asset delivery experiment

Implement `p10-cached-static-v1` on separate opt-in listeners, preserving defaults.
Its scope is the packet's43 CSS/font/SVG assets, not the source's runtime JavaScript
or a production cache profile. Native controller/companion/fixture CSS remain no-store;
the experiment cannot claim a fully warmed React runtime or complete delivery parity.
The source and target use the same declared policy, checked on actual responses:

| Property | Fixed contract |
|---|---|
| Admission | Exact clean unencoded URL, no query, selected decoded SHA/MIME/size, GET/HEAD, no RSC/route-prefetch headers; authority checks run first |
| Freshness | `Cache-Control: private, max-age=60`; never `immutable` for these URLs |
| Encoding | Identity or gzip level6, precomputed; honor quality zero, `Vary: Accept-Encoding`, no acceptable representation yields406/no-store |
| Identity | Strong ETag for the selected representation; gzip decodes to the selected original SHA; HEAD sends the same headers without a body |
| Exclusions | Documents, RSC/route data, mutations, errors, unknown URLs and unselected assets remain no-store; no proxy, route rewriting or source edits |

Freeze the asset/transport identities before browser collection. Synthetic HTTP tests
and an independently reviewed source/target cache smoke test precede a full acquisition.
Both old listeners stay independently usable; a new listener is not an upgrade of them.

### Exact CSS-declared font aliases

V1's browser smoke found that the pinned stylesheet requests fonts with `?v=4.1`,
while the packet identifies their query-free file URLs. Keep v1's failed receipt and
no-query policy unchanged. The separate `p10-cached-static-v2` must bind38 exact
aliases from40 CSS URL occurrences to the38 existing font pins, with the source CSS
SHA, each canonical URL, MIME, decoded SHA and size. Its [implementation and first smoke](../assessments/2026-09-15-catalyst-font-alias-delivery.md)
pass the HTTP and cold-page checks, not the complete gate. No alias may be inferred by stripping
an arbitrary request query. Other values, extra/duplicate parameters, encoding or path
variants stay uncached; six route-data headers and authority refusals still apply.
Both transports must check the same alias membership and byte identities, then pass
the direct HTTP and six-sample actual-browser smoke before wider measurements.
The reviewed v2 source composes the unchanged v1 host: an authorized exact alias
selects its canonical prepared representation using a fresh lookup object; the original
request and source delegate URL never change. `X-AppKit-Delivery-Experiment` identifies
v2 separately from v1's representation policy. Both hosts expose the same canonical
alias-manifest SHA; verify it on every static response. Plans distinguish43 decoded
assets,38 aliases and81 delivery URLs. V1 receipts and listeners remain independent.
Experiment/manifest headers identify selected-static handling, including406/no-store;
dynamic/query/veto/authority/method fallbacks omit them. The first v2 smoke passes
1,296 HTTP/62 exclusion checks and all three cold samples. Source warm records cache
hits, but CDP cannot supply two cached font bodies; it fails, with two samples not run.
Prime bytes are not observed warm bytes. Any alternative identity proof requires a
prospective contract and independent oracle review; existing failures remain unchanged.

### Supplemental warm representation identity <a id="supplemental-warm-representation-identity-prospective-only"></a>

Upstream [Blink body retrieval](https://chromium.googlesource.com/chromium/src/+/052942593bafb4a3e0169e2d0fbee8e50e4d2d9c/third_party/blink/renderer/core/inspector/inspector_network_agent.cc)
has a missing-content path consistent with the observed error. This is not attribution
to the exact installed Chrome build. [CDP cache events](https://chromedevtools.github.io/devtools-protocol/tot/Network/#event-requestServedFromCache)
report cache service separately from retrievable response bodies.
Retain the strict body-required audit unchanged. A separate supplemental
`validator-linked identity` record establishes only the following finite inference.
The [implemented checker and first actual result](../assessments/2026-09-15-catalyst-warm-font-identity.md)
link two source fonts; direct warm bodies and the strict gate remain incomplete.

- One selected WOFF2 warm request links to exactly one immediately preceding prime
  request in the same isolated context and frame. Record request IDs, navigation
  epochs/loader IDs and monotonic event sequence; prime completion precedes warm start.
  Evaluate each font independently, bounded by the selected38-font closure; duplicate
  candidates or ambiguous navigation/context linkage fail, never choose a convenient one.
- Prime decoded bytes, MIME and size match the selected asset pin. Warm is settled
  GET200 with an actual correlated `requestServedFromCache` event; no redirects,
  service workers, prefetch cache,304 or network/response failures qualify.
- Exact strong ETag, Content-Encoding, Content-Length, MIME, Vary and v2/alias headers
  match across prime, warm and the same-encoding raw-HTTP representation probe.
  Absent, malformed, truncated, multiply represented or mismatched metadata refuses.
- Only a structured, bounded, exact CDP missing-data error for that warm request may
  explain its unavailable body; substring matching and unrelated collector-error
  suppression are forbidden. If the harness cannot expose an unambiguous error kind,
  mark that prerequisite unavailable rather than manufacturing a protocol error code.
- Keep strict `sample.ok`, failure arrays, body availability, byte/timing totals and
  cache counts unchanged. A linked identity is not directly observed warm bytes,
  a complete cache/delivery audit, a replacement performance sample or certification.

Independent design review requires failure canaries for changed URL/context/frame/
epoch/ETag/encoding/MIME, missing prime bytes/cache event, duplicate candidates,
unrelated failures and header ambiguity before implementing that supplemental result.
Do not refetch, disable cache, inject prime bytes or retrofit older receipts. Bounded
allowlisted response headers, collection/frame/loader/event lineage and exact error
facts now feed the checker. Playwright1.63.0 omits the protocol numeric error code;
the reviewed wrapper hash and exact captured name/message/command identify that
limited error class. Missing/drifted wrapper proof makes only the supplemental result
unavailable; it must not stop the base smoke or change strict failures/byte totals.

## Compare the actual workloads

The retained hardened React `/settings` page includes navigation/account shell,
hydration and application chunks. The current Go specimen contains the complete
settings form but no equivalent shell. Do not present their whole-page bytes or first
view times as equivalent application implementations or an overall speedup.

| Measurement | Source | Target | Interpretation |
|---|---|---|---|
| Form task readiness | Ten editable source controls and actual fonts ready | Same ten native/enhanced controls and fonts ready | Same task boundary; record differing host/runtime overhead |
| Bio edit/country-region correction | Hardened source local behavior | Optional native companion; native-only typing measured separately | Same supplied values and expected correction; native server validation is a separate navigation task |
| Reset | Source's complete initial snapshot | Native HTTP Reset and enhanced local Reset | Record transport and focus/node differences, not identical mechanisms |
| First-page resource payload | Whole React route with shell | Go form-only host | Group bytes by responsibility; no equivalence or relative-speed headline |
| Shell-inclusive first useful view | Whole source page | Not applicable until equivalent Go shell exists | Explicitly incomplete comparison, not a zero-cost shell |

The source is the documented hardened derivative, not untouched purchased TSX.
Select explicit existing roots/origins, verify source/build manifest and target binary
identities, and revalidate processes before collection. Preparation/install/relaunch
is not implicit in the collector. Static source assets can be hash-checked; dynamic
Next HTML/RSC identity must be recorded as observed, not reproducible by assertion.

## Prospective collection

- Pin browser, Playwright, Node, OS, hardware, source/target manifests, runner and
  controller identities; record CPU/network shaping, viewport and cache mode.
- Start with390/dark and1440/light at100% zoom. Use30 fresh-context cold samples and30
  warm-cache samples per source/native/enhanced profile, serially with no concurrent
  benchmark work. Warm cache requires observing hits, not just reusing a context.
- Run named0/100/300ms added-RTT lanes separately; loopback0ms is not WAN/mobile proof.
  Record offline editing, correction/reset availability and recovery as behavior,
  not a latency number or a fabricated successful save.
- Record per resource URL/type/status, cache provenance, content encoding, transferred
  and decoded bytes where instrumented, and SHA of available bodies. Distinguish
  document, CSS, fonts, images, source runtime/chunks and native controller. A local
  gzip calculation is not served compression or network transfer.
- First useful form: all ten expected controls usable plus `document.fonts.ready`.
  Record navigation origin, task start, DOM readiness and observed font completion.
  Injected probe overhead and unsupported native/no-script timing stay explicit.
- Input/update: same bio text and country/region transition; local input-to-next-paint,
  native POST422/correction navigation and Reset are separate rows. Record p50/p95,
  all samples, long tasks above50ms, DOM count and heap where available. Do not call
  a fixed observation delay, selector wait or requestAnimationFrame alone interaction latency.

## Gates and unresolved admission

Native initial routes must send zero application JavaScript. Input-time native/enhanced
edits must send zero requests. No outside-origin traffic or unexpected background task
is allowed. Source-owned full-page overhead cannot be silently dropped from accounting.

The existing proposed operational screening targets are ≤50KiB compressed first-view
JavaScript, local input-to-next-paint p95≤100ms, and no task-blocking main-thread work
above50ms. Apply them only to a validated named workload and label the actual device;
a high-end local machine does not satisfy the proposed mid-range-device experiment.
First-view non-inferiority needs a source distribution and a separately reviewed
variance margin before target performance repairs. Initial collection is descriptive,
not permission to set an acceptance threshold equal to the observed result.

Initial18,005-byte controller/calculated5,128-byte gzip figures are historical; the
[newline repair](../assessments/2026-09-15-catalyst-textarea-transport.md) is18,280/5,235 bytes.
Both are artifact facts only. Native styling changes, complete asset payload, measured runtime
costs, network/offline lanes and consumer integration time remain to be collected.
