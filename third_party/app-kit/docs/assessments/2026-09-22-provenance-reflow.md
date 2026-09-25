# Provenance specimen responsive repair

## Bounded resumption

- Packet: R4/R1, owner-funded core maintenance; Gallery's provenance/diagram reading task.
- Need: at390px the existing `/provenance` page renders570px wide, observed during the
  retained [CSS comparison](2026-09-22-gallery-css-closure.md). Keep source, uncertainty,
  diagram labels and copyable examples readable, with contained keyboard scrolling.
  Hard-coded sample values also need an explicit synthetic-data notice: the illustrative
  published citation and billing derivation do not substantiate these demo numbers.
- Inputs: clean `c8f4baf`, existing Gallery/vizframe APIs, pinned Tailwind4.3.0,
  Chrome153.0.8010.53, Playwright1.62.1; synthetic first-party data only.
  Subsequent capture builds include the stated experiment's dirty tests/docs; no clean
  binary or independently installed consumer is claimed.
- Authority: kit-only composition/tests/docs; no consumer writes, public API expansion,
  provider, live data, global config, runtime switch or deployment.
- Acceptance: page contained at320/390/1280 in light/dark; unchanged source meanings and
  figure text; focus-visible keyboard access and actual horizontal scrolling where needed;
  keep caption/source outside chart scrolling and retain intrinsic SVG dimensions;
  visibly distinguish API demonstrations from real survey/billing measurements.
- Bound: one incomplete diagnostic, one bounded diagnostic, then one red and one green
  acceptance acquisition. Stop on a new unexplained failure; do not relax the oracle.
- Owner/return: app-kit maintainer; fix this composed example and document the reusable
  integration lesson. No full G3, pixel-fidelity or consumer-adoption promotion.
- Evidence: `/Volumes/m2ext/appkit-evidence/appkit-provenance-reflow.qkzAAe`; executed inputs and failures
  retained before any repair. Human A/B remains separately deferred.

## Investigation

The first diagnostic retained its320px screenshot but stalled at `page.addStyleTag` in
a script-disabled context. The exact owned Chrome PID was resolved from the test's Node
parent and terminated; the tool session exited1, and both owned PIDs disappeared.
The incomplete run is not a pass. The second diagnostic uses direct DOM style changes
only to localize layout contributions, with a30s browser-closing watchdog; this is not
production CSS or acceptance evidence.

The bounded diagnostic completed: at390px, releasing grid-cell minimums reduced the
page from570px to493px; containing the chart body then restored390px. At320px the
same steps left324px, with the final long inline test identifier requiring a break
opportunity. Wide1280px remained1280px throughout. These interventions localize the
composition defect; they do not alter production or replace the fixed browser oracle.

The chosen repair stays in the Gallery caller. Public `vizframe` accepts arbitrary
trusted body content, and analytics callers already supply their own scroll regions;
changing its default would risk nested focus stops and needlessly affect consumers.
The reusable lesson is documented at the public API's integration boundary.

## Repair and verification

`apps/gallery/provenance.go` gives only its chart bodies named native scroll regions,
retaining320px bars and the460px mechanism. Both specimen-grid children can shrink;
six code examples have named focus-visible keyboard regions, and the final long
identifier can wrap. Captions and sources stay outside the SVG viewport. A prominent
notice states that values are synthetic and illustrative citations do not substantiate
them. No public renderer, data value, dependency or consumer changed.

The frozen [browser driver](fixtures/provenance-reflow/browser.mjs) reads HTML/CSS
captured from actual `gallery.New()` handlers by retained `capture.go`, not a replacement
page. It fulfills only those two GET routes in isolated Chrome contexts with page
JavaScript disabled and other requests denied. This is not OS network containment.

| Check | Original | Repaired |
|---|---|---|
| Static composition regression | Missing notice/regions | Passes, including grid/code/identifier checks |
| Light/dark at320,390,1280px | Six cells fail; four overflow | Six pass;532 checks total |
| Keyboard | Required chart/code regions absent |66 visits by Tab;46 overflowing regions scroll on ArrowRight |
| Figure content | Source metadata preserved | Values/source copy/kinds and dimensions preserved; caption/source glyphs fit |
| Pinned CSS rebuild |43,446 bytes | Identical; required utilities already existed |
| Race regressions | Not reacquired | Gallery, viz, vizframe and analytics pass |

The driver was frozen before both acceptance runs. Reports record tool/input hashes,
six initial and six post-task screenshots, and successful bounded cleanup. The dark320px
post-task capture was also visually inspected for contained code/chart scrolling and
exposed focus. No full accessibility, pixel equivalence, performance, installed-consumer
or human-usability acceptance is inferred.

SHA-256 identities, relative to the retained root above:

- `browser.mjs`: `068f0c3d3fbdcb1232cba1781a0e95fabc2b88c85850df6b46afb98e32b54d0b`.
- `browser-red/report.json`: `f33eb06fbfa0ce05e924611769cb35183a47793f10e268a1e98482728f5cee06`.
- `browser-green/report.json`: `95c25b3abbf426bedc092335537fe2aa91b4cf1c6749f5b60aef35f1b902213f`.
- Shared CSS: `0abe503d969b308d005c7d6632386638fe75f92ea53ff4ba9d65770fe41b7a33`.
- Repaired light HTML: `ce7a445ae6dc40122d85bf8ce31034a89519c3307b10acbc561b008c647d4718`;
  dark: `53df8c14bada3475d0dd7ff1740cb689076cbcdf9400c925d9a8dc606e3e2489`.
- `capture-after`: `79b9c181287dc7f0a3eb25efa2a102b36ea02ad851ac760d38e875646f9d79c4`.

Replay: `node browser.mjs <playwright-entry> <chrome-executable> <input-root> <new-output-root>`.
Use the exact tools recorded in either report; output directory must not exist.
The owner has a120s deadline with separately bounded cleanup. Original failed static,
browser and stalled diagnostic receipts remain failures.
Scoped regression: `go test -mod=readonly -race ./apps/gallery ./appkit/ui/viz ./appkit/ui/vizframe ./appkit/ui/analytics`
with `GOWORK=off GOPROXY=off GOSUMDB=off GOMAXPROCS=2`.

## Review follow-up, separately bounded

Independent review found an existing copy error after the successful acquisition:
the page claimed every figure had a source line, though mechanism figures carry an
explicit waiver. The existing Gallery coverage test intentionally accepts that waiver.
Retain this green source and inputs, add a red/green copy regression, then correct that
sentence. Owner choice: one additional six-cell replay of the unchanged browser oracle
against newly frozen final HTML, because changed wrapping could affect containment.
No new behavior, relaxed assertion or open-ended acquisition is authorized by this step.
The retained `waiver-red.log` fails on the exact false claim. Corrected final HTML
passes all six cells/532 checks with the same driver and CSS; final light390px initial
capture was visually inspected. `browser-final/report.json` SHA-256 is
`1487f9bfaa22fe172d86436e622deac1c5f9967629dc4da99eb2220a247c3f20`;
it binds the new light/dark inputs. `capture-final` SHA-256 is
`e1f678c201a4f26bbe78fa10d0a002b8c0aff5e9afc9bad3a8f7aab33f56fc11`.
MCP/document and quality-plan package tests pass; final source race results are retained
in `regressions-race-final.log`. Broader R1/R4 and human A/B gates stay open.
