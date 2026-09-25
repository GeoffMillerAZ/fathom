# Analytical mark resolution and returned guide adoption

## Bounded resumption contract

- R1/R5, owner-funded core/adoption maintenance after `7f9f054`.
- Need: a positive observation must not become a zero-width graphical mark;
  discovery must not deny the scoped guide integrations already recorded.
- Scope: high-level `analytics.RenderBars`, fixed synthetic regressions and a
  representative local browser task; `recipe.styleguide.local` and its existing
  quality-inventory record. No consumer edits, source conversions, new APIs,
  dependencies, provider calls, installed MCP refresh, deployment or live data.
- Preserve I11/I12/I17/I18, ADR-0056/0065/0069/0072/0074. Low-level `viz.Bars`,
  existing validation and row-height defaults, zero/missing distinctions, source
  meaning, roles and all CSV bytes remain unchanged. No quality tier is promoted.
- Implementation: if a rounded positive bar would occupy less than one chart
  coordinate pixel, use an explicitly explained table-only fallback. Never inflate
  the bar, change the declared scale, drop the observation or label it zero.
  Existing fitting layouts remain byte-identical. The table retains every value,
  bounds, missing reason and role meaning; absent plots have no chart legend.
- Discovery: record exact Fathom artifact/source evidence and historical RAC evidence
  without claiming current full-app adoption or released propagation. Keep failed
  performance and partial integration states. Full-record selected-metadata pins
  change under ADR-0065; old consumer selections are not silently rewritten.
- Verify: retain old code plus new test bytes before RED; unchanged tests must pass
  after repair, including subpixel, one-pixel, measured-zero and readable controls.
  Run focused package/race and catalog/MCP/quality checks. Browser uses fixed source,
  CSS and synthetic values with native table access, two schemes and narrow/wide
  layouts; no font download or runtime dependency installation.
- Budget: two scoped fixes, at most 60 minutes after this contract; at most one
  baseline/candidate browser pair, with one separately recorded correction only for
  a demonstrated harness defect. Keep genuine failures; stop at a useful handoff.
- Evidence root: `/Volumes/m2ext/appkit-evidence/appkit-bar-resolution.zA3lsjzJ`; exact executed
  source/harness/logs are retained before revisions. Owner: app-kit maintainer.
- Contribution: shared analytics fix plus synthetic regression; FATHOM-GUIDE-01
  returns source-bound consumer evidence through the existing catalog/inventory.
  Consumer pin review, installed-host delivery and remaining guide/analytics gates
  are separate work, not inferred from these checks.

## Result

Both scoped changes are implemented. Source/race/MCP checks pass. The browser
comparison demonstrates the semantic repair, but its overall result is **failed**:
four narrow keyboard-scroll checks remain unresolved. No quality tier is promoted.

### Analytical mark resolution

The unchanged primitive reproduces a zero-width positive mark for values1 and1e12.
The new high-level tests fail against the baseline on both automatic and explicit
maximum cases, then pass unchanged after the guard. Measured zero, comfortably
readable values and the exact one-pixel boundary remain charts. Missing reasons,
partial coverage, role meanings and CSV truth remain intact.

- `go test ./appkit/ui/analytics -run '^TestBarResolution' -count=1 -timeout=60s`: pass.
- `go test -race ./appkit/ui/analytics ./appkit/ui/viz ./apps/gallery -count=1`: pass.
- `go test ./appkit/catalog ./internal/qualityplan ./cmd/appkit-mcp -count=1`: pass
  after the final metadata clarification; exact results retained in `final-catalog-tests.txt`.
- `go vet ./appkit/ui/analytics ./appkit/catalog ./internal/qualityplan`: pass.
- Static Fathom verification:40 claimed tasks verified across four plans; not runtime proof.

The baseline is7f9f054750ee722c66335d7acc53f1b08ddc6261. Private candidate source is
that archive plus only `appkit/ui/analytics/render.go`, with the same synthetic
fixture and exact CSS. Node26.3.0, Chrome153.0.8010.53 and Playwright1.62.1 run fresh,
script-disabled contexts:1440×1000 light and390×844 dark, spread/readable selections,
both baseline and candidate. No signed-in profile or external asset is involved.

RUN-01 stopped after one completed cell: the driver used `innerText` on SVG and
guessed CSV header names `lower_bound/upper_bound`. The one admitted harness
correction uses `textContent` and source-reviewed `lower/upper`, without changing
the application, expected values or semantic fallback requirement. Both executed
drivers and failed reports are retained in the [portable fixture packet](fixtures/bar-resolution-2026-09-22/README.md).

RUN-02 completes eight cells and74 checks:68 pass, six fail. Two are the required
baseline semantic failures; the candidate has the explained fallback with no
unplotted legend in both lanes. The other four are unexpected narrow keyboard
failures in both binaries and both selections: Tab reaches the overflowing table
with a2px outline, but the immediate observation after ArrowRight remains at zero.
Neither application behavior nor harness timing is isolated as the cause. There
is no second correction, retry or acceptance waiver in this batch.

All independent table/role/missing-value and CSV-row checks pass. Baseline/candidate
CSV bytes match for both selections; readable-document hashes match within each
lane. Exact document/CSS completion, restrictive CSP, zero app scripts and clean
local transport checks pass. All owned servers/Chrome exit without forced stops;
ports refuse afterward, with no timeout or capture/close error. Wide fallback,
narrow focused table and wide readable-chart screenshots were inspected; this
does not establish horizontal keyboard access or whole-screen accessibility.

### Returned adoption evidence

The local-guide catalog now records RAC/Fathom scoped history and links Fathom's
exact integration receipt. `core.themes-style-guide` records artifact
`52666a587cc44dfc146064cd58d5003d34d79d319592d8d16d6394c1ff454198`
as `verified-scoped`. Maturity remains experimental, integration partial and
performance failed. Preview-only script/form restrictions are distinguished from
the guide's own native editing/export forms.

The new inventory regression was retained RED/GREEN in
`/Volumes/m2ext/appkit-evidence/appkit-styleguide-return.BXK7OE`. An old catalog test required zero
consumers and failed against the corrected history; its original bytes/failure are
in `stale-discovery-oracle.tar/.stdout`. Its replacement preserves experimental,
runtime and promotion refusals while requiring the bounded consumer history.
That oracle maintenance is not counted as an unchanged RED/GREEN repair.

Existing MCP tests compare current embedded CUE with exact `get_capability`, Gallery,
canonical pins and `get_quality_work` output. No duplicate protocol harness was added.
Read-only `appkit adoption pins` observes the new guide metadata revision
`820a6558235601f4c217ceac0f0751fb4915c8c2f0a5989943a7faebf5cd8f7f`;
Fathom's selected old revision remains
`565b6155b2a834cb954d8b948876923852f510b3cd8efc23456e14ea5de1ec56`.
The shell pin is unchanged. Review metadata deliberately under ADR-0065; an exact
old-mirror consumer check and drift against a newer catalog answer different questions.
No consumer file or installed MCP binary was changed in this batch. The installed
470c9cf server requires a separate rebuild/restart to expose these updated records.

### Retained identities

All paths below are relative to the private evidence root above.

| Artifact | SHA-256 |
|---|---|
| `red-source.tar` | `ce4620caa388bcd50c8839eee7a0591fdf91bdf05f15d513393b634b32482d98` |
| `green-source.tar` | `4d91d85ce9d0935826bc032effc167ec88f43005b3b451897196ad32c10f79df` |
| `browser-harness-01.tar` | `fda6ce4a0d52629c50320633c21095b96d5243090702b6ae0889dc40d28f6dea` |
| `browser-harness-02.tar` | `00e45606cd6dff150def5bbc3b3594c5b11348576025d310c67c1a45edd3dad4` |
| `baseline-fixture` | `3ae6d4f90b7b9ff4042b04b2965a7cb4c83c8809c400a1a74f315aed67912990` |
| `candidate-fixture` | `68e5cde8d5b43893bdcca518c303258c5de8998aa91645b23e5f575431dd270b` |
| shared `app.css` | `ae56bfc2ef6cb042bd4305caa0bd421d906681dbaaf209f7c3c9a46144028297` |
| `browser-run-01/report.json` | `4d4a6592eef879b5534c8cc36b766ddaa456ac99d44c8b9836c860576bd934e3` |
| `browser-run-02/report.json` | `9c0b6d3902841b7cbbcc9a77603cbbff808231c77aa1c067a55507c081b2bce5` |

### Stop and resume

INC-62 links the mechanical defect to its regression. Intent-prune compresses closed
INC-60 (prior detail in13fc281) and the new record; root intent budgets remain within
their limits. No invariant, future, decision or completion criterion changed.
The changelog's earlier pending Fathom status is corrected to the already-authorized
integration, not a new consumer change.

Stop at these two scoped repairs. Next keyboard work must first inspect native
scroll/event settling against the preserved symmetric failure under a new bounded
contract, with the same semantic/value oracle. Keep these failed receipts unchanged.
Consumer metadata review, installed-host delivery, Fathom large-number geometry,
actual zoom, human A/B and broader accessibility/performance/G3/G4 acceptance remain
separate open gates. This is incremental core/adoption value, not goal completion.

Subsequent [separately contracted scroll observation](2026-09-22-analytics-scroll-settling.md)
verifies all four native right-edge/return tasks on these exact retained binaries.
Immediate samples remain zero; positive movement appears during bounded observation.
That later scoped pass resolves this local keyboard question, not the historical
failed report or broader acceptance gates above.
