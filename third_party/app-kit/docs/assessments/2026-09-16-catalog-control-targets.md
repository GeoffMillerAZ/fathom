# Catalog control targets — 2026-09-16

G1 usability repair, not completion of G1, whole-Gallery accessibility or conversion
certification. [INC-18](../intent/incidents.md) now records this recurrence outside the
analytics page: padding a surrounding row does not enlarge its interactive element.

## Result

Standalone catalog controls now own a 44px minimum height using existing utilities.
Search/select fields, submit/reset, result links, pagination, empty/error recovery,
detail return/specimen links and both disclosure summaries retain native semantics.
Summaries keep their list-item marker; inline prose links retain text flow. No labels,
routes, data, catalog metadata, scripts, dependencies or CSS bytes changed.

`TestCatalogRenderedControlTargets` parses actual handler HTML and guards the relevant
classes and native summary layout. With the retained original renderer supplied through
a Go overlay, the final test fails 32 class assertions. The same test passes after the
repair; Gallery, catalog and MCP package suites pass with the race detector. This class
canary does not substitute for computed browser geometry.

## Same-oracle browser evidence

The [repo-owned probe](fixtures/gallery-catalog/README.md) launches each exact binary
with paid assets disabled and fresh Chrome contexts. Five pages × two widths × two
schemes cover index, filtered, empty, malformed cursor and shell detail. Application
scripts are disabled; all browser requests are local GETs. Native Tab/Enter searches,
opens a result, returns with theme/query intact and expands the disclosure.

| Observation | Original renderer | Repaired renderer |
|---|---:|---:|
| Checks | 357 | 357 |
| Failed 44px standalone/disclosed target observations | 156 | 0 |
| Other failures | 0 | 0 |
| Native keyboard-task assertions | 20 pass | 20 pass |
| Full-page captures | 20 | 20 |

Both runs use byte-identical probe source, Playwright entry module and 51,129-byte CSS;
Chrome is 152.0.7977.84. The matrix uses 390/1440 CSS-pixel widths, 1000px viewport height,
light/dark and reduced motion. HTTP status and exact stylesheet bytes are checked on
matrix and keyboard navigations. A closed disclosure excludes its hidden JSON link;
after native expansion that link must be visible and meet the same 44px floor.

The initial v1 baseline (`catalog-targets-uc3HIz`) remains failed: 236 checks/156 target
failures. Independent review found caller-supplied server identity, absent explicit
CSS/network proof, a viewport/target-width field collision and incomplete disclosure
visibility/cleanup evidence. V2 fixed those measurement gaps before either final run;
it owns binary launch, retains the served CSS and waits for process/stdio closure.
The original renderer was replayed under the same strengthened oracle as the repair.
No target threshold was relaxed and no old receipt was relabeled.

Narrow-dark index and desktop-light detail captures were visually inspected against
the existing layout: hierarchy, text wrapping and disclosure markers remain readable.
This is scoped operator review, not a complete visual or accessibility certification.
Shared header targets, physical touch, assistive technology, actual browser zoom, other
engines, performance and consumer acceptance are outside this probe.

## Retained artifacts and reproduction

Private root: `/Volumes/m2ext/appkit-catalog-review.3ebNBEYt/`.
It retains original/final renderer source, final parsed test, CSS/input CSS, module
files, the old-source overlay, binaries and final red/race logs. The first incomplete
candidate binary `gallery-after` is not the final tested artifact. Final runs:

- Original: `catalog-targets-u1B9Lf`, binary `gallery-old-final`.
- Repair: `catalog-targets-4Keit3`, binary `gallery-final`.

Each run retains its executed probe, CSS, 20 PNGs, process logs and JSON receipt.
Post-run review rehashed all 40 images, binaries, probe and CSS plus drained logs;
receipt hashes are below. Both browser and Gallery processes exited 0
without signals, and all four PIDs were independently absent. The earlier owned
baseline server's exit 0 is separately operator-transcribed from its command/session
results in `exploratory-server-shutdown.json`, not a claim of the v1 browser receipt.
Unrelated previews were not touched.

| Artifact | SHA-256 |
|---|---|
| Final probe | `1a512d4add28c490338e3ce1d17cb01f4849a421485ee03d92a6f0e44598fa3b` |
| Original receipt | `1b4d7497d9257967f3bdab1aa9f9ee9c09debc58aa0e197e8042e79ddf56cb61` |
| Repaired receipt | `3acc202057a7ca24a23b13dad0b3c16b72e5a6f3437331873b27e5a4ce2e4a1e` |
| Original binary | `798aafbc518bfffedfab26ad59d0c941c03b10e5a550a3d2d316ba3c0582c829` |
| Repaired binary | `aafc2035e63c71d53f1e74fed1d68e397c051d209220f4020afb945066222c39` |
| Served CSS, both | `812d7e1a50e65f6bed881893cc291f8501c48cb3260ac556c05a71310df6cab7` |

From the working checkout, builds used `go build -o BINARY ./apps/gallery/cmd/gallery`;
the original additionally used `-overlay ROOT/red-overlay.json`, mapping only
`apps/gallery/catalog.go` to the retained original. Verification used:

```sh
go test -overlay ROOT/red-overlay.json ./apps/gallery -run '^TestCatalogRenderedControlTargets$' -count=1
go test -race ./apps/gallery ./appkit/catalog ./cmd/appkit-mcp -count=1
```

`ROOT`/`BINARY` are explicit operator placeholders. Go 1.26.3/Darwin-arm64, Node 26.3.0,
warm offline caches and `GOWORK=off GOPROXY=off GOSUMDB=off GOENV=off GOTOOLCHAIN=local`
were used. Build commands/source association are operator evidence, not hermetic input
closure, cold reproducibility or a clean immutable release. No commit, consumer pin,
deployment, signed-in browser, real data or global configuration changed.
