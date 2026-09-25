# Public analytics renderer at its row boundary, 2026-09-15

## Result and scope

The existing `analytics.MaxRows == 200` contract now has deterministic, race and
actual-browser evidence at that boundary. No production renderer, public route,
dependency, token, consumer pin or default appearance changed. This is G3 capacity
evidence, not full G3 acceptance or promotion of a large-data renderer.

The test-owned Gallery handler calls the real `RenderBars` and `WriteCSV`. Four
390/1440 by1000 light/dark Chrome contexts use page scripts disabled, reduced motion
and synthetic data. Native Tab/PageUp/PageDown/ArrowRight/typing/Enter reach the first
and last table rows, expose Meaning, download CSV and change to the long-label fallback.
No scripted focus, scroll or form-value assignment substitutes for those tasks.

Final result: **268 checks, zero failures/errors; 28 viewport PNGs, eight CSVs and one
owned-browser record**. Each capture requires settled native scrolling, the named
label/meaning target fully inside the viewport before and after capture, stable
geometry, exact PNG dimensions, all decoded rows and nonblank pixels. The narrow
table is horizontally scrollable: label and Meaning captures are separate views,
not a claim that every column fits on screen simultaneously.
Check counts include repeated observations, not268 independent requirements.

## Independent fixture and contracts

- Ordered IDs `service-001` through `service-200`; value at zero-based index `i` is `i`.
- Eleven positive multiples of17 are missing with an explicit unavailable-source reason;
  they remain in the table and CSV, never as plotted zeroes.
- Row zero is a real zero. There are189 measured tracks and188 positive fills.
- Sixteen measured rows have explicit `i-.25` to `i+.25` intervals: positive multiples
  of13 and the last row. Missing rows have no invented interval.
- Even/odd rows declare API/Worker meanings; both table and CSV preserve those labels.
- September1 through October1 UTC, evaluated October5 at12:00 UTC with11:00 observation
  and a two-hour freshness budget; coverage is explicitly11 unavailable of200 selected.
- The long variant changes only the first label to18 repetitions of `Long service label `.
  Its explicit table-only fallback retains all200 rows without an orphan legend.
- A201st row is refused by validation/render/export before output. Rendering/export
  do not mutate the caller's selection. Integer values remain `0`, not forced `0.00`.

Go regressions: [maximum rows](../../appkit/ui/analytics/max_rows_test.go),
[test-only controller](../../apps/gallery/analytics_scale_browser_test.go).
Browser and invocation: [scale.mjs](fixtures/gallery-analytics/scale.mjs),
[probe guide](fixtures/gallery-analytics/README.md#public-renderers-200-row-boundary).

## Predeclared budgets and observations

Hardware: Apple M4,10 logical CPUs,32GiB RAM, macOS arm64; Chrome152.0.7977.84,
Node26.3.0, reviewed Playwright1.63.0. These are local synthetic/in-process results.

| Measurement | Declared budget | Observed |
|---|---:|---:|
| Full normal HTML |256KiB|172,842 bytes maximum|
| Full fallback HTML |256KiB|79,836 bytes maximum|
| Applied Gallery CSS |128KiB|51,129 bytes|
| DOM elements, normal/fallback |3,500|2,224 /1,272|
| Validating render,100 iterations |25ms/op;4MiB/op|0.844ms;1,329,297 bytes/op|
| Validating CSV,100 iterations |25ms/op;4MiB/op|0.095ms;60,126 bytes/op|
| Local DOM-ready p95 |500ms|14.3ms across eight observations|

The eight browser observations are four first-context loads and four native GET
navigations, not eight cold-cache samples. No speedup, LCP/INP, remote latency, human
task time/error, production workload or memory-residency claim follows. Allocation
figures are allocated bytes per operation, not retained heap. Timing is diagnostic,
not a timing-sensitive default CI gate; byte and row bounds remain deterministic tests.

The normal chart is6,420px tall. Capacity is not recommended dashboard density: apps
must select useful cohorts or aggregate deliberately, keep coverage/export scope clear
and not misrepresent top-N as complete. Native access to a very long table is not proof
of effortless analysis. This evidence does not justify speculative caching, WASM or a
new renderer; high-volume and real-consumer acceptance remain separate work.

## Retained proof and corrected capture oracle

Private root: `/Volumes/m2ext/appkit-analytics-scale.02FdbPYQ`.
The controller saves its selected test/renderer/CSS/harness bytes and a manifest before
browser work, pins expected response hashes and its executing test binary, validates
the persisted report and exact artifact inventory, and rechecks source identity afterward.
Only fixed loopback GET/HEAD fixture paths and CSS are admitted; no external/write/socket
requests occur. No signed-in browser profile, licensed sources or real app data are used.

1. `analytics-scale-3699113071/report.json`, SHA
   `c7fc9056aefdff70e9f07c7bc789fca282fe5be9a07010385b79fa6605ad8f94`:
   163 checks; task/DOM/export evidence, without owned-PID or decoded-PNG validation.
2. `analytics-scale-1377600222/report.json`, SHA
   `a50f237ccc504cebe18be3ccfaabbae66e212f33af62fbf87956dea5bdc0de67`:
   204 checks including cleanup/PNG shape, but main visual review found the narrow-dark
   first-Meaning capture showed the chart instead of its table target. A transient
   scroll position and an X-only horizontal check were insufficient. That image remains
   failed target-capture evidence, not a successful visual acceptance result.
3. `analytics-scale-332569846/report.json`, SHA
   `94b0168ab9feb7f98b2882185a55e2b21421de977debdd7c7cc3f3db498bd39c`:
   268 checks, with settled native scroll and immediate before/after target X/Y checks.
   Manifest SHA `963a6cd612db7fbef7bbf57f1c0767bcac89b378b8718b04bb88f0eb10f7b1e8`;
   executed harness SHA `cc709c708ce1f72bc0bdc006d31e8661aa574a2163b50ef400ebbaf2506fe6d0`.

Main independently rehashes the selected snapshots and37 artifacts and visually checks
narrow-dark first/last Meaning plus wide-light wrapped-label captures. The final owned
Chrome parent PID32016 exits after bounded close; a separate process check finds it
absent. A150-second watchdog precedes the controller's three-minute deadline. Descendant
processes were not exhaustively inventoried; existing user previews remain untouched.

[Four oracle regressions](fixtures/gallery-analytics/scale_test.mjs) exercise the actual
harness predicates without launching Chrome: off-screen X-correct targets, empty or
horizontal clipping, complete PNG decoding, and blank/wrong-width/truncated/invalid
captures. This closes the local INC-28 recurrence, not all historical capture gaps.

Final full-package race tests pass: analytics1.275s and Gallery23.861s. The final browser
test passes70.324s; ordinary source-free runs skip the explicitly opt-in browser test.
`benchmark.txt` SHA `1cd96d8238087e91625bfaaf1211f735dada9d28b092bf18f9c64969483ceb87`
retains the100-iteration benchmark output. No commit, release, deployment or consumer
upgrade is included. Existing small-journey, human and two-app outcome gates stay open.
The MCP suite passes6.096s after guidance updates. Structural checks pass ten living-doc
budgets,274 relative file links and the60-line Unreleased body, not full intent-doctor
or anchor/host validation. The working-tree whitespace check is clean.
