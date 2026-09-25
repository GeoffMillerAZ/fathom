# Existing-capability and signed-chart guidance

## Bounded resumption

R1/R4/R5 owner-funded maintenance; baseline `5107d437b17f506d9924802efa0ea0fe5abe4872`.
The task is to select the existing signed renderer, understand a table-only comparison,
and find the contribution record from primary MCP guidance. Initial inspection suggested:

- `RenderBars` still recommends an app-owned signed representation only, despite
  experimental `analytics.RenderDivergingBars` being available under ADR-0074.
- A subpixel signed mark sets the label-space fallback, incorrectly blaming a lack
  of48 pixels on each side of zero even with a2000px chart and short labels.
- Primary MCP guidance mentions the contribution loop without its callable lookup
  (initial hypothesis, disproved by the baseline MCP test below).

Repair these messages/routes only. Preserve negative-value refusal, all validation,
plotted geometry, table/CSV values, role meaning, CSS, dependency graph and tool names.
I8/I11/I17/I18 and ADR-0066/0074 apply; no future or architectural boundary is promoted.
No consumer/global/installed-server changes, real data, restart, deployment or release.

Acceptance: frozen RED/GREEN for the two analytical messages and MCP route; signed
positive/negative subpixel cases keep every table/CSV value with no chart/legend;
ordinary and genuinely cramped charts retain their existing behavior. Package/race
regressions and a bounded original/candidate browser comparison of explanation,
exact table values and native keyboard access. Existing CSS and synthetic data only.
Maximum20 minutes implementation/acquisition, one browser acquisition plus one
separately retained correction only for a demonstrated harness defect. No broad
browser matrix, full accessibility/consumer acceptance or quality-status promotion.

Evidence root: `/Volumes/m2ext/appkit-evidence/appkit-analytics-guidance.WHf513po`.

## Results

The two analytics regressions fail on the original source and pass after repair:
`TestNegativeBarsRecommendExistingSignedRenderer` and
`TestDivergingSubpixelGuidancePreservesData`. The latter preserves both signed
values in table/CSV, chart/legend omission, actual label-space fallback and ordinary
signed geometry. Only refusal/explanation text changes; signatures, admission,
data, plotted geometry and CSS remain unchanged. Consumers rebuilding the recipe
receive the new diagnostics, not an automatic mirror or metadata-pin upgrade.

**Corrected hypothesis:** `TestIntentDocumentsThroughMCP` already passed on the
baseline because primary guidance embeds `CLAUDE.md`, which routes the contribution
lookup. The added `contribution-loop` assertion is test-only protection, not a fixed
MCP defect. `kitdocs.go`, tool names, registration and installed servers are unchanged.

The analytics, viz, Gallery and MCP race suites pass with no selected-source drift.
Both exact fixture builds pass using Go1.26.3/darwin-arm64, cached dependencies,
`GOWORK=off`, `GOPROXY=off`, `GOSUMDB=off`, CGO disabled and no VCS stamping. This
is a warm-cache synthetic build pair, not cold-build or consumer delivery proof.

One frozen Chrome153.0.8010.53 acquisition passes **60 checks across four cells**:
baseline/candidate at1440×1000/light and390×844/dark, scripts disabled, native Tab.
Each document retains the exact ±0.01 and1e12 rows, roles and table focus. The
candidate differs only in its two subpixel explanations; every other HTML byte
matches, including ordinary SVG and long-label fallback. All four captures were
inspected: explanations wrap, tables retain their existing narrow-screen scrolling,
and focus remains visible. No unexpected requests/errors/timeouts; owned Chrome
exit/close is observed without forced termination. Node26.3.0, Playwright1.62.1,
actual framework, driver/helper, CSS and document identities are retained.

Two preparer syntax failures occurred before Go execution: JavaScript replacement
string expansion duplicated runner text. `units-syntax-failed.mjs` and `units.mjs`
retain those sources; `units-fixed.mjs` passed parsing before the RED/GREEN runs.
No failed test or browser oracle was weakened; no second browser acquisition ran.
Implementation/acquisition completed within the selected20-minute window; later
status review and documentation closeout are separate. Independent small-model
code/evidence review found no blocker.

Documentation closeout added the missing `[Unreleased]` budget assertion to the
existing private check runner. It found86 lines in both baseline and candidate,
before MCP execution; `final-checks/result.json` retains that failure. Consolidation
folds overlapping unreleased entries into their owning features; released sections
must stay byte-identical. The unchanged assertions run again under
`final-checks-after-prune/`, with fresh pre-execution sources and no result overwrite.
INC-22's multiline evidence is compacted without removing its outcome; `5107d43`
retains the prior wording. This check covers selected links/budgets/stanza and the
MCP package, not a full intent-doctor or installed-agent audit.
Final result: MCP tests,410 relative links, agent-stanza equality and all selected
budgets pass; `[Unreleased]` is60 lines. Released changelog sections are byte-identical.
This result summary is appended after that frozen check; production/test inputs are unchanged.

### Retained identities and replay

Paths below are relative to the evidence root above. Each test phase saves its
executed runner and selected source bytes before execution; both complete fixture
source archives and the original prospective contract remain in the private packet.
The final results text is not substituted into the acquisition's contract identity.

| Artifact | SHA-256 |
|---|---|
| `baseline.tar` | `598ed83de6fe386b8e140e2facf2b75f4e6efdff44a4fee4226aa997a13ff390` |
| `build/baseline-source.tar` | `781e52fd8e446df2d728fbfb379374714b9f72bf87dd16115c0990e3751975f7` |
| `build/candidate-source.tar` | `8837578b851a73ddf2f1ee5312fc4a5c317a0d74150ef114bd858f3dde6dc09e` |
| `red/result.json` | `25363e3acb9cdd03d577b96e7a0c4dba8d1d82dfa0b365c1068fa539f5afff5d` |
| `green/result.json` | `caa1aafd9c6ff7fd0d41d831a5506dffc47da8f490f027a422792f2bc5fc9d1a` |
| `regressions/result.json` | `42ba9311ab611694f2e2224f618179fe39fe1ddf22ce391cc8684feca9b91de9` |
| `build/result.json` | `3986f85d09ced594fc7941762ad3ea1eab1c169be39f49397dd00dd56c93078a` |
| `browser-inputs/manifest.json` | `dbf0c0463dfc61568cf1e2418ddad023e6989a237c0d5b1cb4cf6eba4a69e6e0` |
| `browser/report.json` | `3504eae90520d74c4c1954d7b96098cb2975bbb4f972d532c2d68ed11bc7928d` |
| `final-checks/result.json` (retained budget failure) | `9eb7db1f03fd2457fe9fa112119a144d307e6ee2774aa169e0ad82613014f898` |
| `final-checks-after-prune/result.json` | `4aaaf42b68013e85e155182306e311ac5de027d5920836308d3cd8ea297f6735` |

[Replay instructions](fixtures/analytics-guidance/README.md) describe the frozen
fixture/manifest and boundaries. CSV agreement is unit evidence, not a native
download in this acquisition. Full accessibility, actual zoom, performance, human
understanding and consumer adoption remain unproven; no G1/G3/G4 or core promotion.
