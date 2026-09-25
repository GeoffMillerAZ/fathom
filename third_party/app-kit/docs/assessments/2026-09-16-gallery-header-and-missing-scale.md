# Gallery navigation and missing-data truth — 2026-09-16

Scoped G1/G3 improvement, not completion of the goals, whole-Gallery accessibility,
conversion certification or consumer adoption. No consumer, dependency, public API,
data/CSV contract, commit, deployment or global configuration changed.

## Result

Below 1024 CSS pixels the outer header now offers a native, initially closed Sections
disclosure with the current section named. It uses `dashboard.RenderNav` and the same
app-owned destinations/selection queries as desktop navigation. Desktop navigation
remains expanded. Header links and the disclosure own 44px minimum targets and visible
keyboard outlines; theme links visibly identify the selected setting, and home retains
theme. Native tab order follows the visual brand, theme, then navigation order.
No new script, navigation framework, shared API or authentication behavior is involved.

The analytics comparison only states a numeric scale when it emits a chart. All-missing
overview/detail states explain that nothing is available to plot; the measured-zero
worker still has its SVG, numeric zero and bounded 0-to-1 display scale. Previous-period
reference values, domain calculations, export schemas and public renderers are unchanged.

## Evidence

The [maintained probe](fixtures/gallery-catalog/header-journey.mjs) owns a source-free
Gallery process and fresh Chrome contexts. Identical harness bytes test both retained
binaries at 320/390/768/1440 CSS pixels, light/dark, 1000px high and reduced motion.
Application scripts/service workers are disabled; only allowlisted same-origin GETs
are admitted. Each navigation binds served HTML/CSS, status and the 32 KiB HTML budget.

| Observation | Before | After |
|---|---:|---:|
| Assertions | 1,916 | 1,496 |
| Failed observations | 598 | 0 |
| Header target failures | 480 | 0 |
| Missing-state numeric-scale failures | 16 | 0 |
| Retained HTML documents | 48 | 48 |
| Full-page captures | 32 | 38 |
| Maximum observed HTML bytes | 22,389 | 30,873 |
| Served CSS bytes | 51,129 | 51,373 |

The denominator differs because the old header exposes all destinations while the new
narrow header is closed; the candidate additionally exercises opening/resizing it.
The other 102 baseline failures cover disclosure visibility, visible navigation/current
inventory, selected-theme emphasis and tab order. No assertion threshold was lowered.

Native Tab/Enter proves skip-to-main, opening Sections, choosing Analytics, opening the
data-condition disclosure, applying a missing fixture, detail and selection-preserving
return. Native select **type-ahead** chooses Missing; popup-arrow interaction is not
claimed. Resizing an open menu to desktop and back preserves one visible current link
and native open state. Header normal-text contrast is at least 6.808:1 in the repaired
observations; this is opaque text/background sampling, not a universal contrast audit.
Narrow-dark closed/open and desktop-light missing captures were visually reviewed.

Gallery/dashboard package suites pass with `-race`. The two new header tests and
missing/zero tests run against retained old renderer sources via `baseline-overlay.json`:
36 missing-scale assertions and both header cases fail; measured zero already passes.
The same regressions pass after repair. Existing analytics disclosure assertions now
scope the main landmark; both header navigation trees independently retain the exact
selected query. Browser visibility proof remains separate from these class/DOM tests.
The unchanged catalog main-content probe also passes 357 checks on the final binary.
Final review added visible theme emphasis and brand/theme/navigation DOM order; the
expanded test fails both checks against the retained first candidate via
`header-v1-overlay.json`, then passes the final implementation.

Six existing handler benchmarks at 100 iterations on Apple M4/Darwin-arm64 observed
55,624–557,150 ns/op and 66,832–403,036 B/op, below the existing 5ms/512KiB local ceilings.
These are warm synthetic observations, not speedup, network, LCP/INP or device claims.
Responsive duplicate navigation increases HTML by 8,484 bytes in the largest sampled
page; the 32KiB ceiling is unchanged. No speculative cache was added.

## Retained execution

Private root: `/Volumes/m2ext/appkit-gallery-flow.wmCjBqzF/`.
`before/` retains 11 selected source/build inputs; `candidate-v1/` and `final/` each retain
13 including tests and their executed probe. These snapshots/build commands are operator association,
not complete hermetic closure, cold reproducibility or an immutable release.

- Before: `gallery-before`, `header-journey-i9Qvwd`.
- After: `gallery-final`, `header-journey-n7KrQp`.
- Additional unchanged catalog regression: `catalog-targets-Ae6NVQ`.
- Earlier candidate: `gallery-after`, `header-journey-M49YDt` (1,472 passing checks),
  paired with `header-journey-OVDG7n`. This predates the final tab-order/theme checks;
  `catalog-targets-cNXMo0` is its separate main-content regression, not final-build proof.
- Failed collector attempts remain: `header-journey-hjz7QB` (selection stayed complete),
  `header-journey-atzdUd` and `header-journey-YryuQ2` (popup-key sequence navigated during
  focus traversal). Their exact harnesses/results remain failed; type-ahead corrected
  input delivery before the final same-harness baseline/candidate pair.

Each final header run retains exact executed source, served CSS/HTML, images, process
logs and receipt. All 168 HTML/image/CSS artifacts were independently rehashed, plus
the two harness copies and binaries. Both owned Gallery/Chrome pairs exited 0 without signals and
all four PIDs were independently absent; unrelated previews were untouched.

| Artifact | SHA-256 |
|---|---|
| Final harness, both | `e59d721a615c656c77e5c46fed6fa5e7907c633d1c0e6aa667b5355a4f8059ee` |
| Before receipt | `55615f292670b9abd16d2c82103d068b28820c680c65bf9cf1de07af7991a83c` |
| After receipt | `229bc7fc3c61826e4f834790708e275ea9e4a6eee0933f35d56f1dfc01dd9ccc` |
| Before binary | `aafc2035e63c71d53f1e74fed1d68e397c051d209220f4020afb945066222c39` |
| After binary | `83e80305194e0a5fd0b17a0bda1f2e0cbc36f03bedcaeb6f4249fc869c58e29c` |
| Rebuilt CSS | `94589496d9aa1e0e72c629b316df2c2629104af3dec2dda27e5a73726179a01b` |

Build: `task gallery:css`, then `go build -o BINARY ./apps/gallery/cmd/gallery`.
The equivalent pinned CSS command was invoked directly. Tests: `go test -race
./apps/gallery ./appkit/ui/dashboard -count=1`; benchmark: `go test ./apps/gallery
-run '^$' -bench '^BenchmarkAnalytics' -benchmem -benchtime=100x -count=1`.
Warm offline caches, `GOENV=off GOWORK=off GOTOOLCHAIN=local GOPROXY=off GOSUMDB=off`
were used. [Runner instructions](fixtures/gallery-catalog/README.md) describe invocation.
Actual browser zoom, physical touch, screen readers, other engines, human task outcomes,
full native/consumer acceptance and release propagation remain unproven by this slice.
