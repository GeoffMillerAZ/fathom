# Cached-font protocol diagnostic

Date: 2026-09-16. G1/G4 instrument investigation, not a component or performance pass.
The [P01 candidate](2026-09-16-hero-font-delivery.md) still lacks request-bound warm
font bytes. Eight Chrome recording configurations were tested; none recovered them.
No original, asset, cache policy, benchmark oracle, budget or failed receipt changed.

## Bounded experiment

The already-authorized isolated Playwright installation launches installed
Chrome152.0.7977.84 with a fresh unsigned profile and owned loopback server. Seven
resources are copied from the exact retained performance candidate, checked against
its request-graph hashes, and served with the same CSP and cache headers. There is
no application script, service worker or extra asset request. The first six modes
do not intercept requests; the two Fetch modes pause only the exact font response.
Each context clears cache only before cold navigation, then navigates normally for
warm. All eight final warm lanes retain seven completed browser requests, six cache hits,
zero static-asset server requests and one uncached document request.

| Configuration | Cold font bytes | Warm font bytes |
|---|---|---|
| Default, unthrottled | Exact via `getResponseBody` | Unavailable |
| Larger network buffers | Exact via `getResponseBody` | Unavailable |
| Legacy durable-message flag | Exact via `getResponseBody` | Unavailable |
| Streaming requested at response notification | Exact via `getResponseBody`; streaming request too late | Stream accepted but empty; body unavailable |
| Default, benchmark network throttling | Exact via `getResponseBody` | Unavailable |
| Current durable-message command plus streaming at request notification | Exact via stream and `getResponseBody` | Stream empty; body unavailable |
| Raw Fetch response interception, cold and warm | Exact through the paused response ID | No Fetch pause; body unavailable |
| Raw Fetch enabled only after ordinary cold navigation | Exact through Network in cold control | No Fetch pause; body unavailable |

The final four configurations use80ms latency and10/5Mbps network emulation, not CPU throttling or
the complete performance matrix. In its cold positive control, streaming is accepted
before the response:128 chunks reconstruct192,380 bytes with SHA
`a328e08221a9a70a27c193d3a2fc2fbcfa5b238d8360c3cc796341130b8b1eb0`.
For warm, the command is issued at request notification but its reply arrives after
loading finishes: zero chunks and zero buffered bytes. It cannot prove that an earlier
browser-internal activation would fail. Both cold/warm use the same collector; body
retrieval runs only after stream finalization in that mode.

## Fetch response-stage result

The materially different Fetch instrument uses raw CDP, not Playwright routing. It
keeps cache enabled and pauses only the selected font URL/type at response stage.
Successful body observations bind Fetch's network ID to the settled Network request,
exact URL, frame and current navigation loader before accepting bytes. One cold
positive control returns the exact192,380-byte font before continuing the response.
Enabling Fetch throughout or only after the cold navigation makes no warm font pause:
both warm lanes still have six actual cache hits and zero asset-server requests.
Thus this instrument also cannot supply warm bytes; it does not defeat the cache to
make a false warm pass. No performance oracle or failed benchmark changed.

The first two diagnostic receipts remain failed: a cold error-stage notification
preceded Network observation and had no HTTP status. Capturing its raw payload revealed
`responseErrorReason: Failed`, not `CacheMiss`. The corrected diagnostic retains it as
an unavailable-body observation, never calls the body API on it, and reconciles it
after navigation. A later distinct Fetch ID supplies the successful200 response for
that Network ID. This is not proof of a cache-miss/retry cause. The
[Fetch protocol](https://chromedevtools.github.io/devtools-protocol/1-3/Fetch/#event-requestPaused)
explicitly permits an error reason instead of HTTP status at response stage.

The primary protocol documents [durable storage](https://chromedevtools.github.io/devtools-protocol/tot/Network/#method-configureDurableMessages)
and [request-ID streaming](https://chromedevtools.github.io/devtools-protocol/tot/Network/#method-streamResourceContent),
but does not guarantee cached-font extraction. These results establish the limit of
the tested Chrome/protocol sequence, not a universal browser impossibility or a font
defect. Painted-font facts, cached files and earlier cold bodies remain insufficient
substitutes under the current strict request-body contract.

## Replay and retained evidence

The maintained [diagnostic](fixtures/hero-adapter/cache-probe.mjs) snapshots itself
before execution. Supply the retained candidate performance receipt, its independently
recorded SHA, and an existing private output parent:

```sh
APPKIT_PLAYWRIGHT_MODULE=/absolute/approved/playwright/index.mjs \
node docs/assessments/fixtures/hero-adapter/cache-probe.mjs \
  /absolute/hero-performance-NAME/receipt.json EXPECTED_SHA256 /absolute/private-parent [fetch]
```

Exit0 / `completed:true` means the diagnostic and cache controls completed, **not**
that warm verification passed. Inspect the per-request body/stream result. The
maintained script defaults to the final two Network configurations; `fetch` selects
the two Fetch configurations. The first four keep their historical snapshot. Neither
diagnostic is a replacement benchmark. `completed:true` permits explicitly unavailable
error-stage observations and absent warm bodies; it is not an asset-identity pass.

Private parent: `/Volumes/m2ext/appkit-hero-font-candidate.AGTWfHOp/`.

| Packet | Receipt SHA-256 |
|---|---|
| `cache-probe-cX8WvP` — eight unthrottled cold/warm lanes | `f0b42fe1f9d54a9517480957d85059b4a4654754ad45532b96f84fa9af7ce752` |
| `cache-probe-RwjPof` — four throttled cold/warm lanes | `357710e6f6cb0aca26f28ede4cf365e9887ca01e9c36083e423602ecb2ab0467` |
| `cache-probe-ePOowX` — four final Fetch cold/warm lanes | `71c5b6f2c0dca2f6d82c87e9777399b01c1bb06c3087a2e3b0a76e10d56ae0c6` |
| `cache-probe-unb4eC` — failed early-correlation assertion | `039a943216ff1466a5e9f8e5c1b1ee34737d6e744df55bd4407dc9222b76ec9c` |
| `cache-probe-MQZa3k` — retained failure with raw error event | `502e65f52cd391cb5286979422a2f9734057d57b955be63ce246fff9146128a3` |

All packets retain executed source, input receipt, seven input copies, observed body
files, request facts and launcher logs. Owned browser exits are0 and servers close.
The maintained script exactly matches the final Fetch snapshot SHA
`f5518467935fa2ec62620829dc77d7763cde4cd818005b9b2ad8bf3b65664156`;
the earlier Network snapshot remains
`709ef8de7d54476b6cbf395143d2c8a64e09c761c981931c976ac68a1049b2b6`.
Stop repeating these configurations unchanged. Revisit for a materially different
request-bound instrument or browser/protocol revision; preserve the failed warm gate
and continue the other G1–G6 acceptance work. No target promotion follows.
