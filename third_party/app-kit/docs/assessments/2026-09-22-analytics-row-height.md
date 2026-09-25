# Analytical row-height readability

## Prospective bounded task

R1/R4, owner-funded core maintenance from app-kit
`a9e375fa566460eabdd4bd3e619443402482e5c6`. The clean checkout's high-level
`RenderBars` and `RenderDivergingBars` accept `RowH:1, Gap:1` while emitting 11px
text at 2px center spacing. A single 1px row also risks clipping even with a large
gap. Source review identified the lead; browser reproduction was pending when
this contract was frozen. Executed results follow below.

Task: read every selected label/value, including zero/missing meaning, and export
the same data when a caller chooses an overly dense layout. Preserve I11/I12/I17,
ADR-0069/0074, existing defaults, low-level `viz.Bars`, validation refusals, all data
and CSV. The existing positive-small-dimension non-error contract stays intact.
No app-specific policy, consumer writes, global configuration, dependency additions,
installed MCP refresh, licensed source changes or quality promotion.

Use the existing table-only fallback when the normalized row height is below
16 logical pixels. This is conservative space admission for the current fixed
11px SVG labels, not a measurement or guarantee for every consumer font. Checking
`RowH+Gap` alone cannot protect first/last-row edges. Explain the row-height limit
and recovery, retain every table row and suppress an unplotted legend. Defaults
and fitting row heights must remain byte-identical to the baseline. Empty/all-missing
selections keep their existing no-measured-values message, not a density warning.

Impeccable hardening uses the current PRODUCT.md and analytics experience contract.
No new visual identity, typography, theme or bitmap asset is designed. The scene
is an adopter reading a dense synthetic comparison on a desktop or narrow display;
both existing light and dark schemes remain relevant and unchanged.

## Frozen acceptance and cost

- Unit RED/GREEN: both renderers, tiny rows, a large-gap singleton, 15/16 boundary,
  normal/default and nonpositive-default options, empty/all-missing, unchanged
  table/CSV meanings and no legend for an omitted chart. Existing tests remain.
- One baseline/candidate Chrome acquisition: normal and signed renderers; tiny
  rows and singleton edges; 16px and default controls; light1440 and dark390.
  Inspect actual SVG text bounds/overlap, exact tables and independently parsed
  native CSV downloads. Read the narrow table via native keyboard and a settled
  postcondition, using the retained 750ms scroll cap and no-input control.
- The baseline must reproduce tiny-row overlap and singleton clipping. Every
  candidate chart must fit measured label/value bounds; omitted charts retain
  explained, keyboard-accessible tables. Fitting HTML and all exports must match
  baseline bytes. Inspect selected captures, not merely bounding boxes.
- Same frozen source/fixture/CSS/runtime, completed document/CSS bodies, exact
  loopback route allowlist, no application scripts/external assets, fresh profile,
  retained failed results and bounded owned process cleanup.
- At most60minutes; one browser pair under a120second owner bound, with at most
  one separately recorded correction for a demonstrated harness defect. A genuine
  UI failure remains failed; do not weaken geometry/data oracles. No full matrix,
  memory acquisition, human A/B or consumer acceptance claim.

Evidence root: `/Volumes/m2ext/appkit-evidence/appkit-row-height.c68n7FlL`. Freeze executed source,
driver/helper, commands and runtime identities before each run. Owner: app-kit
maintainer. Contribution is the shared fallback plus synthetic regressions and an
upgrade note; broader analytics acceptance and real-consumer propagation stay open.

## Result

The frozen baseline reproduces overlap in both multi-row renderers and clipping
in both large-gap singletons, in both lanes. The candidate passes **224 checks
across 32 cells**, with zero unexpected requests, in 10.7 seconds on Chrome 153.0.8010.53,
Playwright 1.62.1 and Node 26.3.0. No corrective acquisition was needed.

- All 32 tables and native CSV downloads match independent expected rows, including
  signed/zero/missing values and all ten data columns. All 16 baseline/candidate
  export pairs are byte-identical; all eight fitting HTML pairs are byte-identical.
- Candidate tiny/single charts are omitted with the exact recovery message and no
  legend. Floor/default text bounds fit the actual SVG and adjacent labels do not
  overlap under the unchanged 0.25px tolerance.
- Both narrow candidate tasks reach the table by Tab, retain visible focus during
  a 150ms no-input control, scroll with ArrowRight to Meaning and return left. No
  programmatic scroll substitutes for input; the 750ms observation budget is unchanged.
- Six of 14 retained captures were inspected: normal-tiny/signed-single failures,
  both readable-floor charts, narrow normal/right and signed/left fallbacks. The
  baseline clips visibly; fitting charts and fallback text/table remain readable.
- Document/CSS completion, exact CSS identity, restrictive CSP and no app scripts
  pass. Both fixture processes and Chrome exit 0 without forced termination; output
  capture completes and both server ports refuse subsequent requests.
- Identical frozen unit tests fail all eight fallback cases before the repair and
  pass afterwards. `go test -mod=readonly -count=1 -race -timeout=120s` passes
  `./appkit/ui/analytics ./appkit/ui/viz ./apps/gallery`; no source drift was observed.

## Retention and adoption

All paths below are relative to the private evidence root above. `baseline.tar`,
phase-specific source overlays, build source archives, runner bytes, commands,
runtime hashes, stdout/stderr and results are retained. These builds use warm local
caches with offline dependency resolution and `-buildvcs=false`, not cold/clean-Git
reproducibility. Only the two high-level renderer files differ between fixture builds.

| Artifact | SHA-256 |
|---|---|
| `build/baseline-fixture` | `32de708701bf21586fb65ba8507005be1205a9929409f214b83eb4ea7973b807` |
| `build/candidate-fixture` | `609d7d5a5a0a2d378a2a08c23df8217779862b09730ea9a2ac94ae36bafb6c0c` |
| `browser-inputs/manifest.json` | `61489d2ea2e0d178d411c77bb9176ff66a8a8863b1826e8150cc07cc3fa42dde` |
| `browser/report.json` | `924e058e300fad0ab7073efce845e6fc19170b5d52a0e9432e09eafc7f6385fa` |

[Fixture/replay instructions](fixtures/analytics-row-height/README.md) retain the
bounded task. [INC-67](../intent/incidents.md) closes the missing row-height
regression. Adopters rebuilding the high-level recipe receive this fallback; there
are no new dependencies, CSS classes or data/API fields. Review apps that deliberately
set positive row heights below 16; defaults do not change. No consumer was updated.
This is scoped readability evidence, not full accessibility/font/zoom, human A/B,
consumer acceptance or quality promotion; G1/G3/G4 and core readiness remain open.
