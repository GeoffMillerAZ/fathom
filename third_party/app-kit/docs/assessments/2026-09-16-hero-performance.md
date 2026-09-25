# P01 performance: fast local rendering, oversized cold assets

Date: 2026-09-16. G1/G4 experimental evidence, not target certification.
The [maintained benchmark](fixtures/hero-adapter/PERFORMANCE.md) measures the retained
enhanced P01 fragment with its original image/CSS bytes and controlled Inter font.
No component, source asset, consumer, dependency or runtime API was changed.

A separate [font-delivery candidate](2026-09-16-hero-font-delivery.md) now passes
all eight cold cells at499,059 bytes with paired glyph/pixel evidence. The original
results below and all warm body failures remain unchanged; this is not a full pass.

## Contract and measurements

The prospective [contract](fixtures/hero-adapter/performance-contract.json) fixes
390/1440px by1000px, light/dark, two browser launches and cold/warm navigation:
16 observations. Apple M4, Darwin25.6.0 arm64, Chrome152.0.7977.84, Node26.3.0,
DPR1, reduced motion,10Mbps download/5Mbps upload,80ms simulated latency and4xCPU.
These are local browser-emulation conditions, not a physical mobile/network claim.

Final normal run `hero-performance-UXZZ4O`:

| Measurement | Cold range, eight observations | Warm range, eight observations | Contract |
|---|---|---|---|
| Uncached decoded bodies | 658,700 bytes | 2,553 observed document bytes | Cold524,288; warm16,384 bytes |
| First contentful paint | 288–304ms | 128–140ms | Cold1,000; warm600ms |
| Sampled largest contentful paint | 520–528ms | 128–140ms | Cold1,800; warm1,000ms |
| Readiness upper-bound observation | 764.4–788.4ms | 153.4–163.3ms | Cold2,000; warm1,200ms |
| JS heap, including instrumentation | 1,315,044–1,315,440 bytes | 2,036,252–2,038,576 bytes | 16,777,216 bytes |

Every cold lane fails payload:131.3KiB/25.6% above the512KiB target. The352,240-byte
font and288,581-byte photograph account for97.3% of cold decoded bodies. Exact cold
asset identities, seven requests, three decoded/two visible images, zero application
scripts, visible links and native Tab order pass. Timing and heap meet their budgets.
This is evidence about this specimen, not a performance claim for the whole catalog.

Every warm lane remains failed: Chrome reports six cache hits, fresh `navigate`
navigation and one uncached HTML request, but `Network.getResponseBody` cannot return
the cached font body. Timing and document bytes are observations, not a clean warm
pass. All warm server lanes have zero asset requests. Stored font bytes, a cold read,
or a request-unbound `Page.getResourceContent` would not repair that strict proof.
The [P10 precedent](2026-09-15-catalyst-warm-font-identity.md) has the same boundary.

## Fault detection and retained repairs

The oracle's16 Node tests cover fixed budgets/throttles, complete matrix coordinates,
fresh navigation, assets, exact links, malformed metrics and canary-specific failures.
Three final browser controls each use one cold390px/light observation:

- Missing font: exact404, failed font readiness and targeted asset failure detected.
- Delayed CSS: exact2,500ms injected delay plus FCP/readiness overruns detected.
- Oversized photo: exact614,400-byte growth and changed body identity detected.

An incomplete16-cell matrix or unrelated budget failure cannot count as detection.
Canary execution exit0 means detection plus cleanup, never a passing component.

Initial `hlU4D0` is a failed harness run: disabled JavaScript stalled instrument RAF
and timer callbacks; Chrome also requested an implicit favicon. The isolated
`/Volumes/m2ext/appkit-hero-instrumentation.0bFg2CA5/` control showed font readiness in
both modes, but RAF/timer callbacks timed out at500ms only with script execution off.
The corrected instrument enables those callbacks while CSP still prohibits application
scripts. An explicit empty data favicon adds31 wrapper bytes; original fragment,
styles and assets are unchanged. The actual measurements include that wrapper delta.

Initial missing-font `h7HZMM` also remains failed: its canary incorrectly required a
completed404, but Chrome aborts that font response with `net::ERR_ABORTED`. Independent
review admitted only that exact404/path/error, zero body/empty SHA, failed readiness
and targeted asset failure. Negative tests reject200 aborts, other paths/errors,
missing duration, nonempty bodies and successful readiness. Ordinary gates did not change.
The original normal `LCMnpv` and two initial detected controls remain historical runs;
the final runner counts observed cache hits even if response-body retrieval fails.

## Retained evidence

Private parent: `/Volumes/m2ext/appkit-hero-performance.Bqh5VTtv/`.
Each run retains the launcher/runner/oracle/test/contract before execution, copied
input assets, raw request graphs, metrics, captures, command logs and receipt hashes.
The launcher executes the private copies and checks source/snapshot identity afterward.
Chrome executable identity/version is recorded, not its entire framework dependency
closure; Playwright binds the selected entry module, not its transitive installation.

| Final directory | Receipt SHA-256 | Result |
|---|---|---|
| `hero-performance-UXZZ4O` | `4649670fd7a60f13a329ad80851fe5ac61d5e92b0de6d00e1008e14b9ca3ee4d` | Eight cold budget failures, eight warm identity failures |
| `hero-performance-L8dOaG` | `9668215ab9d7e77f861b4afbe1c9bf5048f0b2cd32cca584b95c5ffad51626d9` | Missing font detected |
| `hero-performance-FnmZQf` | `08ed58b1261b45e2d3f682ae687ef733b96b9f4a349c46f7b2ce56d4a5873c24` | Delayed CSS detected |
| `hero-performance-mZe77s` | `e26fc0e5d7a4f2b3548bb4d881b0ddf4d11f47c5ac72af4b62c49ef0e8357938` | Oversized image detected |

Final normal run retains16 screenshots; three controls add three. Main-agent inspection
of desktop-light and narrow-dark normal captures confirms readable composition, images
and visible focused link, not complete visual/accessibility certification. Every owned
browser exits0 and each loopback server closes; the first failed watchdog also cleaned up.
A separate retained `audit-final.mjs` replays the final oracle and checks all four
receipts, five executed-source snapshots and15 copied inputs per run, command-log
hashes,19 PNG identities/dimensions and actual absence of the five final owned PIDs.
Its `audit-final.json` SHA-256 is
`a6d0628ba424a16c3856963e672a0e39b0606c1f22e5dcde293e016cb3602cec`.
The scoped intent check passes14 documents/279 relative links/64 ADR records and root
line budgets; this is not a full host-wiring or semantic intent audit. Node oracle
tests run in `task check`; the full maintainer suite was not rerun for this fixture.

## Consequences and next work

ADR-0023's cost-budget revisit condition fires. Preserve the original and current
failure; the rendering profile remains plausible because its measured timing passes.
Next compare a separately declared asset-delivery candidate: same visual design and
glyph coverage, smaller font/image payload, original bytes retained, fixed budgets and
paired fidelity checks. A font subsetting tool is not installed in the checked default
Python/PATH; no dependency was installed or alternate font substituted to obtain green.
Solve or explicitly redesign the warm-body evidence contract with independent review;
do not silently replace a request-bound assertion with stored-byte or family-name proof.
Full performance, accessibility, consumer acceptance and target certification stay open.
