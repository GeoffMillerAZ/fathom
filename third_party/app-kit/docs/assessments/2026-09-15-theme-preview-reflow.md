# Theme preview: visible components and actual browser zoom

G1/G4 hardening under I8/I12/I15 and ADR-0029/0031. No consumer migration,
new editor framework, theme change or conversion certification.

## Escaped layout defect

The existing review/export journey passed47 checks, but visual inspection showed
blank Sparkline/StackBar areas in the narrow component specimen. Their SVGs existed
and had opaque semantic paint. The implicit grid column instead took a long code
sample's minimum content width: a1,193.05px figure inside a388px preview centered
the160px sparkline at x532.52..692.52, entirely outside the visible pane.

`componentsHTML` now gives each figure `min-w-0`. Code samples keep their own
horizontal scroller. Original SVG bytes, figure names, chart inputs, themes and
application CSS remain unchanged. The same builder serves `/components` and the
theme guide; browser evidence below exercises the guide, not every Gallery route.

`TestComponentFiguresShrinkWithoutDroppingChartsOrCode` failed before repair
(zero shrinkable figures, expected17), then passes with the served CSS contract.
The first test draft had an undefined asset identifier; its compile failure was
not the red regression. Tailwind's actual zero utility uses `calc(var(--spacing) * 0)`;
the test accepts that equivalent emitted form as well as literal zero.

## Paired browser evidence

Fresh isolated Chrome152.0.7977.84, approved Playwright1.63.0, macOS arm64.
The repo-owned `apps/styleguide/browser/components_reflow.mjs` runs without page
JavaScript, admits only selected loopback guide/CSS routes, and rejects WebSockets,
outside requests, unexpected console/page errors and incomplete assertion sets.
Native selection/submission settles the new document before measuring it.

Six cases: outer widths320/390/1440, both scheme selections, narrow preview.
The dark-only Geoff Synthwave source intentionally supplies the same palette to
both scheme branches; these are not separate light/dark identity certifications.
Each case requires the exact17-caption inventory, contained positive-size figure
and SVG rectangles, plus the Sparkline's computed stroke/visibility. Screenshots
supplement those assertions; geometry alone does not prove every mark's legibility.

| Run | Result | Receipt SHA-256 |
|---|---|---|
| Baseline `XwK15F` | 45 checks,18 failures | `0cbdff5b332869833b54bb61ffb2c471fd158fcce5b213b58a616073232583bb` |
| Candidate `IJ1IDc` | 45 checks,0 failures | `da2c9fd5def6e2fe0b055cec141176ee6ec37467769132c5965dfb5ba8890932` |

Both use the same final probe SHA-256
`fcdab3813ff5a736d2e846c185332f197a12852dbba885cfe213b55ae6beeca3`.
At388px the repaired figure is356px wide; the Sparkline spans x114..274.
At286px it is254px wide; the Sparkline spans x63..223. Document scroll width equals
the frame width in all six cases. Root visually inspected the388px and286px
candidate captures: Sparkline and StackBar paint is visible, code samples remain.
This is not a pixel-difference, all-chart readability or keyboard-scrolling proof.
Exact HTTP comparison also checked `/components` and both unedited light/dark
component previews: all response bytes match after only the17 figure-class additions.
Thus preservation of SVG/code bytes is separately checked, not inferred solely
from a test comparing two calls to the current `vizCards()` implementation.

Earlier `rqf1NA`/`rqs5Nl` runs refused ambiguous test selectors before collecting
geometry; `Pjsjtz` first reproduced the defect with the smaller33-check oracle.
Those failures remain local and are not counted as product acceptance.
Receipts/screenshots live under the system temporary directory's
`appkit-theme-components-<suffix>` folders, not a permanent evidence archive.

## Actual200% browser keyboard lane

`usability.mjs --zoom=200` retains the normal30 checks and adds a separate private
0700 Chrome profile. The browser Appearance setting is2, then setup leaves WebUI
before activating application request restrictions. It uses no CSS zoom or injected
focus/DOM changes. CDP1440-DIP/720-CSS widths agree with DPR2, innerWidth720,
outerWidth1440, clientWidth720 and visualViewport.scale1.

Tab/Enter/type exercises invalid radius →422/focused error → repair link → corrected
1.5rem candidate → source view preserving edits → reset clearing edits/digest.
Four expanded-state inventories check horizontal overflow, control containment and
44px primary targets. Only viewport screenshots are captured, not complete zoomed
documents; root inspected the error and edited-section captures. This is not an
all-controls traversal, zoomed chart legibility or screen-reader certification.

Final probe SHA-256: `96ef3901ade4f2a9c121615a5b9ced8d0e67dddb989d8db5072a0fcf9e5a96c1`.
The receipt also binds the existing zoom helper. Both owned browsers are closed
even if one closure fails, and late response/cleanup errors precede the success gate.

| Candidate run | Result | Receipt SHA-256 |
|---|---|---|
| `appkit-theme-usability-WTuaus` | 56/56 including actual zoom | `4df9faf1e6c727f0f14e96c05ee4bc29a08242cdca491912af6522a493ea5a98` |
| `appkit-theme-usability-jnThzu` | unchanged normal30/30 | `817befb3266427f6a5de06f414261c9474874dd73f2aa32a200a94386cf1459a` |
| `appkit-theme-review-loop-ksFW6K` | existing export journey47/47 | `3ac57c79d438e6cea89e3b41ce85470d996dd4365610dba5658dfaab5b0bb23e` |

These folders are system-temporary, local synthetic evidence. A first intermediate
zoom probe sampled a previous document; the final lane waits for main-frame
navigation. No application workaround or relaxed expectation was introduced.

## Exact inputs and boundaries

Build packet: `/tmp/appkit-theme-zoom.SgNSrU`, with both retained executables and
`baseline-selected-source.tgz`. These are selected working-tree inputs, not a
complete clean release closure. No existing server was replaced.

- Baseline executable: `25e7f8ff6d4b4bc6c7c12114426a4c374b4c6eb04643ea65f33891ce501fb815`.
- Candidate executable: `f0f79d06729d4f2db9f697d5f57509769dcda407410f22203a15ef239cb2aa34`.
- Unchanged Gallery CSS: `9e2d822864db965307afe0d87828208d63c7082773cb8e57a1aecc7c1bc60d46`.
- Unchanged source CUE: `228a32cb3be5943124020fd791858571112cbc546c20810cbb8fb44eb92d8f08`.

Fresh race tests pass for styleguide, Gallery and its command. No public API,
schema, dependency, paid source, app preferences or consumer files changed.
At this assessment's baseline RAC integration awaited approval; the later authorized
[guide delivery](2026-09-15-resume-theme-guide.md) and [badge/zoom follow-up](2026-09-15-resume-theme-contrast-and-zoom.md) record its implementation and remaining limits.
Actual fonts, all-chart semantics/readability, assistive technology, human task
time/error, production-scale performance and real consumer acceptance remain open.
