# Gallery analytics: keyboard, targets and local performance

Follow-up to the [107-check fixture](2026-09-13-gallery-browser-smoke.md#analytics-journey-follow-up)
and [journey contract](../design/analytics-journey.md). This slice repairs native hit
areas and measures the fixed synthetic journey. G3 remains open; these checks do not
establish human task performance, whole-page accessibility or consumer acceptance.

## Same-oracle browser result

| Observation | Before | After |
|---|---:|---:|
| Passing assertions | 948 | 1,002 |
| Failing assertions | 54 | 0 |
| Page/console errors; blocked requests | 0; 0 | 0; 0 |
| Observed non-download responses (all 200) | 58 | 58 |
| Minimum sampled text contrast | 5.906:1 | 5.906:1 |
| HTML response range | 13,514–21,799 B | 13,528–21,929 B |
| Shared CSS | 47,636 B | 48,033 B |
| DOM-ready p95, five local samples | 23.2 ms | 19.9 ms |

Chrome 152.0.7977.84 and reviewed Playwright 1.63.0 used fresh contexts at
390/768/1440 CSS pixels in light/dark, reduced motion. The full scoped route uses keys
alone: actual skip focus → native select type-ahead → data-condition disclosure →
partial August/API/no-comparison selection → Apply → Gateway detail → Back → CSV.
There is no scripted `.focus()` or `.selectOption()` shortcut. Reached controls retain
native focus outlines. CSV checks independently expect Gateway 4 and Search 2, exact
partial windows, two rectangular records and 31 unique columns. Six downloaded files
per run are 2,712 bytes each; parsed values/context pass before and after the hit-area fix.

All 54 failures were selected interactive targets: summaries were 20px high and table
links 17px or 37px when wrapped. Parent row padding did not enlarge the anchor. Three
template-line changes add minimum heights/padding to native summaries and make each
table link own its hit area, preserving native details/table semantics and URL context.
Two Go regressions fail before and pass after. The pinned Tailwind 4.3.0 rebuild adds
397 CSS bytes from current scan inputs; this entire delta is not attributed to one utility.
No production JavaScript, dependency, data or runtime profile was added.

Root inspected final phone-dark overview, tablet-light detail and desktop-dark overview;
labels/content remain readable with no new outer overflow or visual target regression.
Automated geometry and opaque text/background samples cover the main task, not every
Gallery navigation/footer link. Counts include repeated observations, not 1,002 independent
requirements. Source correctness, screen-reader behavior and complete conformance are not proven.

## Predeclared local budgets and actual handlers

Budgets selected before measurement: HTML ≤32 KiB; local DOM-ready p95 ≤250ms;
in-process handler ≤5ms/op and ≤512 KiB allocated/op. Environment: Apple M4, 10 logical
CPUs, 32 GiB, darwin/arm64. Five 200ms benchmark samples per case; median values:

| Handler | Before µs/op | After µs/op | Before B/op | After B/op |
|---|---:|---:|---:|---:|
| Complete overview | 252.323 | 248.040 | 304,748 | 304,860 |
| Partial API, no comparison | 228.602 | 225.937 | 257,338 | 283,001 |
| Missing worker | 213.671 | 245.540 | 225,484 | 225,456 |
| Stale detail | 224.926 | 215.481 | 235,334 | 235,472 |
| CSV | 31.069 | 30.690 | 65,476 | 65,477 |
| Invalid-query recovery | 47.225 | 46.788 | 61,174 | 61,175 |

Every recorded sample fits those ceilings. The partial-case allocation increase is
retained, not hidden; its cause is unproven. These samples establish neither a speedup
nor statistical equivalence. Handler work includes validation, fixture construction,
HTML/SVG/CSV rendering, headers and copying into a reusable response buffer; it excludes
request construction, first buffer growth, networking and browser rendering. Timing
ceilings are diagnostic, not flaky CI gates. Current costs do not justify a speculative
render cache. DOM-ready is not LCP/INP or human completion time, and loopback is not a
remote/cold network or production-sized dataset.

## Reproducibility and private evidence

The [repo-owned probe and instructions](fixtures/gallery-analytics/README.md) retain
the exact oracle; it refuses non-loopback/write traffic and records actual downloaded
CSV bytes. Entry/package identities are checked before import; full transitive tool
closure is not claimed. The preliminary popup-arrow experiment did not change the
native select and submitted the form on Enter; those materials remain explicitly
noncanonical. Final before and after both use the same stronger type-ahead oracle.

| Identity | SHA-256 |
|---|---|
| Final browser harness | `548a7117005da366288d3484d8296e3f6b0e3fbb6cdcf88ea7d321d0a8f94afe` |
| Before browser receipt | `21329d45d97a02eba31c947350312e60407c5f27252225c31d8a66c0a64d239d` |
| After browser receipt | `8ee98e43c442f155dc5c9f7f7652278fa581b623b7bfaf93c01e7d68fa7f6824` |
| Private packet manifest | `04f18278f72477f55d6e2e32b7285f8835f818daba7070fbe86173b983f4e0dd` |

Packet: `data/design-assets/gallery-analytics-quality.PWc0oB/manifest.json`, 54 payload
files / 7,770,857 bytes. Root independently verified every file size/hash and receipt
harness identity. It retains both final reports, 24 PNGs, 12 CSVs, raw benchmark logs,
selected before/after template/CSS and tests, and labeled keyboard experiments. This
is selected evidence, not a full source/binary/dependency closure. The packet is private
and ignored; back it up separately. Red receipts remain intact.

## Artifact-store boundary

The full root Go suite caught a separate maintenance defect: archived Gallery/RAC
`.go` evidence under `data/design-assets` was treated as application packages, producing
undefined helper and absent RAC-module errors. Git ignore does not control Go discovery.
A minimal dependency-free `data/design-assets/go.mod` now creates an explicit module
boundary. Only that marker is unignored; all existing packet paths and bytes stay intact.
Never run build/test commands inside the data store. INC-19 records positive/negative
package-discovery and Git privacy canaries, including hostile inherited configuration.
Marker-only Gallery/MCP startup checks preserve first-party use without licensed data.
This is not an app dependency or exclusion of real kit code.

## Verification and remaining work

Full `go test ./...` and `go vet ./...` pass after the boundary repair, alongside focused
Gallery analytics and private-boundary race tests. Scoped intent checks cover 88 docs,
516 links, 33 ADRs, six goals and nine futures with no failure; 17 legacy ADRs remain
over budget. This is not the full intent-doctor or host-loading test. Both updated guides
match actual MCP tool/resource bytes. No release/tag/deployment,
live records, provider calls, vendor mirror refresh or consumer mutation occurred here.
Remaining: genuine browser zoom, native popup-arrow behavior, screen-reader/touch/device
checks, independent human task-time/error baselines, production workload budgets and
two app-owned acceptance journeys. The six active goals have not been checked off.
