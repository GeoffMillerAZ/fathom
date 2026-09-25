# P10 payload observations: useful partial data, incomplete benchmark

First acquisition slice of the [measurement contract](../design/catalyst-settings-performance.md).
No performance gate, comparative speedup, warm-cache proof or certification passes.
The source is the whole hardened React page with navigation/account shell; the native
and enhanced targets are complete form-only hosts. They are not equivalent applications.
The [subsequent phase diagnostic](2026-09-15-catalyst-payload-phases.md) observes22
cancellations before teardown in a new source run; this receipt's missing chronology
and incomplete benchmark remain unchanged.

## Collected data

Final private directory: `data/design-assets/p10-payload-PjAAZ6`.
Plan SHA256: `dc0081b1ed56395c1f514aba443920f94d8cbf7a473e6f48b06399529c422d26`.
Receipt SHA256: `6051f9d4b59d3ed98be8d241bfa9f13b17cd8283d2258133472f2fc70e52a863`.
There are90 usable cold observations:30 each for source/native/enhanced at390px dark.
The first1440px light source observation fails;269 of360 planned samples were not run.
Overall status remains `failed-partial-descriptive-observations`, not a green benchmark.

Median CDP encoded-transfer bytes for the30 usable samples per profile:

| Resource responsibility | Source whole page | Native form | Enhanced form |
|---|---:|---:|---:|
| Document, including any inline source data/scripts | 10,072 | 30,110 | 30,715 |
| CSS | 24,626 | 173,606 | 173,606 |
| Fonts | 227,064 | 226,394 | 226,394 |
| Images | 19,969 | 0 | 0 |
| Source runtime/chunks | 260,566 | 0 | 0 |
| Local controller | 0 | 0 | 18,481 |
| Other, including bodyless fixture favicon | 0 | 122 | 122 |

These are instrument transfer observations, not gzip estimates or decoded payload
sizes. Decoded source runtime totals855,506 bytes; the target controller is18,280
bytes. Source/target fonts both decode to226,080 bytes; source CSS172,209 and target
CSS172,989 bytes. The source serves compressed CSS; the current Go fixture does not.
Inline source scripts remain in document accounting rather than silently disappearing.
Native routes expose zero script elements and load zero application JavaScript.

Inference from this fixture only: CSS/font delivery is the largest remaining target
payload cost. Compression and selected-asset optimization deserve measurement before
micro-optimizing the small controller. Preserve design/font/glyph fidelity; this does
not authorize changing source assets or imply a deployed application's budget result.

## Instrument and boundaries

Chrome152.0.7977.84, Playwright1.63.0, Node26.3.0; Apple M4/arm64,32GiB RAM,
Darwin25.6.0. This is not the proposed mid-range/mobile device. Fresh isolated contexts,
local loopback, reduced motion, no signed-in profile, zero added RTT. Source-only HTTP
verification briefly overlapped initial collection; raw timings are not a controlled
performance distribution and are not used above.

The collector verifies the selected private packet and all declared retained files,
43 source build assets and the hardened manifest. Read-only process/listener checks
bind the selected source root and target executable to their provided local PIDs.
Per-profile origin/method restrictions and WebSocket refusal preserve isolation.
Selected file identities and observed response hashes remain separate facts.

Readiness requires the ten actual controls, excluding the Headless checkbox's hidden
transport input; target enhancement must mount. After form/fonts readiness, resource
collection admits requests through a200ms quiet window, bounded at5s. This wait and
automation-observed readiness are not paint/FUV/input latency. Later background traffic
is outside this finite observation. Every completed sample is retained separately;
unknown bytes, uncompleted requests, unusable responses and teardown errors stay visible.

## Failures retained, not normalized into passes

| Earlier directory suffix (`p10-payload-*`) | What happened |
|---|---|
| `ivhW21` | Source readiness incorrectly counted the hidden transport checkbox. Two30s timeouts preceded intentionally terminating only owned Chrome PID44768; the final partial receipt records four failed observations. No source was changed. |
| `gEiZ8j` | 30 source cold samples; first warm attempt had no observed cache hits. A direct source CSS HEAD request confirmed `Cache-Control: no-store`. Caching policy was not changed. |
| `VNL9nx` | 30 source cold samples; target's automatic favicon request returned404. |
| `HQZKVN` | After explicit no-icon response,49 samples pass; the next target favicon was still in flight at the old immediate observation boundary. |
| `PjAAZ6` | 90 phone cold samples pass; first desktop source case records22 aborted RSC prefetches (`text/x-component`, HTTP200, incomplete bodies). The receipt lacks event/close-phase timestamps, so before-versus-during-teardown causality is unproven. |

The source host is not proven defective by those aborts. Do not disable prefetch or
discard canceled transfers to manufacture a favorable source/target comparison.
Next: record event/observation/close phases, keep cancellation and partial-byte accounting
separate, then independently review the finite workload boundary. Warm-cache samples
remain unavailable until a separately justified representative caching setup exists.

## Fixture-only completion and replay

The form-only target now explicitly answers `/favicon.ico` GET/HEAD with204/no body,
no-store and nosniff; POST405 and wrong-host421 remain enforced. The route is reserved
from packet asset collisions. `TestFixtureExplicitlyHasNoApplicationIcon` covers this.
HTTP bodyless responses are recorded as such, not as fabricated missing-body digests.

Modules `p10-go-4253964458` and `p10-go-3505914506` match all16 recorded files.
Manifest SHA256: `6436ced9d647e2d94edd3e16023e69182bfd2f51543f69b3b9877b8041e35224`.
Binary SHA256: `37044595f35ecfe825768012423d8bcae02d300e5989d98330e7f148a3b61535`.
Compared with the [newline candidate](2026-09-15-catalyst-textarea-transport.md), only
server source/tests and the executable differ; all13 other files match. Its1,196
browser assertions remain bound to that earlier binary, not silently re-attested here.
Both new private compile/HTTP/race/build rehearsals pass; the actual source-only
`TestCatalystSyntheticHTTPFixture` passes under race. An earlier guessed test selector
matched no tests and is not counted. Seven payload-helper tests pass and join `task check`.
Full source fidelity, input/update latency, offline/RTT lanes, native delivery and
consumer adoption remain open; no dependency/global/consumer/publish changes occurred.
