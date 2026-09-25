# Gallery navigation payload repair

## Bounded resumption

- R1/R4 owner-funded maintenance, base `a4cea64`: finish the signed analytics
  specimen's existing full-document budget without removing analytical context.
- Need: the retained [signed acquisition](2026-09-22-signed-analytics.md) fails
  `<20KiB` HTML at25,346B light/25,305B dark; header accounts for14,131B. This is a
  payload target, not proof of slow user tasks. HTML+CSS already passes128KiB.
- Choice: deduplicate Gallery-owned desktop/theme/home/disclosure styling into its
  compiled CSS. Preserve all DOM elements, non-class attributes, text, destinations,
  theme/current semantics, native disclosure and keyboard focus. Keep mobile
  `dashboard.RenderNav` byte-for-byte unchanged; no public API or consumer change.
- Intent: I3/I4/I7/I11/I12/I17 and ADR-0020/0069/0074. The measured-budget and funded
  maintenance triggers apply; no richer renderer, provider or future is promoted.
- Scene/register: Geoff inspects first-party recipes on desktop and narrow laptop
  windows in either light/dark surroundings; preserve the current product design.
- Authority/exclusions: kit source/tests/docs/CSS and scoped main landing only;
  no dependencies, external assets, real data, consumers, host config or deployment.
- Acceptance: unchanged signed four-cell browser oracle including `<20KiB` HTML,
  `<128KiB` HTML+CSS, chart/table/CSV truth, geometry, contrast and native export.
  Separately compare12 before/after cells: signed/catalog/quality ×1280/390 ×light/dark.
  Require DOM/content/non-class-attribute and computed style/geometry parity, 44px
  targets, native keyboard disclosure, tested2px focus and hover-state parity.
  Screenshot review is supplemental, not a new pixel-equivalence certification or
  reinterpretation of the earlier CSS-closure pixel failures.
- Bounds: one static red and green; one12-cell paired shell acquisition and one
  four-cell signed acceptance replay. At most one retained harness-only correction
  after a demonstrated instrumentation error. Stop on a new application failure.
- Retain original/final sources, CSS, producer/driver and tool identities before
  acquisitions under `/Volumes/m2ext/appkit-evidence/appkit-gallery-nav-payload.ZYKIFn`.
  Owner: kit maintainer. Return the measured composition lesson, not an automatic
  standard promotion. Human A/B and full G1/G3/G4/core gates remain separate.

## Result

The existing signed-page target now passes, without changing its oracle or removing
content. Gallery-local `g-nav`, `g-theme`, `g-home` and `g-summary` classes hoist the
original utilities into flat `@layer components` rules in `ui/input.css`. Existing
`aria-current` attributes select current states. Shared `RenderNav` is unchanged.

| Delivered bytes | Before | After | Delta |
|---|---:|---:|---:|
| Signed HTML, light | 25,346 | 20,382 | −4,964 (19.6%) |
| Signed HTML, dark | 25,305 | 20,341 | −4,964 (19.6%) |
| Gallery CSS | 43,446 | 46,106 | +2,660 |
| Light HTML + CSS | 68,792 | 66,488 | −2,304 (3.35%) |

The strict `<20,480B` HTML ceiling leaves only 97B light/138B dark of additional
passing capacity. This is a measured source-payload repair, not a latency, memory,
compression or cache-reuse claim. The same 4,964B HTML reduction occurs in catalog
and quality specimens; catalog's larger document is not claimed to pass this target.

- `TestSignedAnalyticsFullDocumentPayloadBudget` fails on both original documents
  and passes against the actual final `New()` handler with the unchanged strict
  limit. The original failing source, test and log remain retained.
- Header tests still check destinations, current states, hostile-input escaping,
  minimum targets/focus and exact shared mobile-renderer bytes. Class assertions
  now resolve the alias's exact selector and declared `@apply` utility; absent,
  wrong/prefixed selectors and removed utilities fail negative controls. These are
  source checks, not evidence of the runtime cascade.
- The unchanged signed browser driver passes **4/4** cases: 1280/390 × light/dark,
  full HTML and HTML+CSS budgets, signed direction and 2:1 magnitudes, zero/missing
  meaning, glyph bounds, contrast, body fit, actual keyboard chart/table scrolling
  and native Enter CSV downloads. All four 1,317B exports match. No blocked requests
  or failures; shutdown acknowledged204, the fixture exited PASS, and its port
  subsequently refused connections. Historical signed failures remain failures.
- The new [paired driver](fixtures/gallery-nav-payload/browser.mjs) passes **384
  checks in 12 paired cases** across signed/catalog/quality. Hash-bound before/after
  documents preserve every DOM node, non-class attribute and normalized direct text,
  measured computed styles/geometry, 44px targets, actual Tab/focus, native mobile
  Enter disclosure and noncurrent-link hover states. Scripts are disabled; unexpected
  requests fail. No failures/blocked requests or cleanup errors were reported.
- The 24 viewport captures are supplemental, not a pixel-equivalence gate. Visual
  inspection covered wide-light signed analytics and narrow-dark quality's open menu.
  Independent review found no blocking oracle defect; static state-selector coverage
  is supplemented by the paired browser's initial/post-task style comparisons.
- Scoped race suites pass for Gallery, dashboard, analytics, MCP and qualityplan.
  After final documentation/inventory edits, uncached Gallery/MCP/qualityplan tests
  and scoped vet pass. Repeating the pinned CSS build into a separate output is
  byte-identical; this is checkout repeatability, not a clean consumer-build claim.

No human A/B, actual zoom, assistive-technology, all-theme, OS network containment,
general process-attestation, consumer or release acceptance is established. Earlier
CSS-closure pixel failures remain unchanged. No readiness gate or maturity is promoted.

## Retention and reproduction

The private packet above contains the original tracked-source archive, before/after
producer and six-document manifests, CSS, red/green test logs, frozen executed tests,
both driver copies/reports, captures/exports and scoped race log. Browser inputs used
Chrome153.0.8010.53 and Playwright1.62.1 without an install or upgrade. The CSS compiler
is the existing checksum-verified Tailwind4.3.0 pin.

| Artifact | SHA-256 |
|---|---|
| Final Gallery CSS | `ae56bfc2ef6cb042bd4305caa0bd421d906681dbaaf209f7c3c9a46144028297` |
| Final light signed HTML | `3e15f7862116610aa372b03e2ae147cb32808f236b17a1c386b6cc9c58ce845f` |
| Final dark signed HTML | `29fd90bd0f67df1fd4f7ab2ddaf8b45249ce47aba7fccbe14ad99b074fffe827` |
| Signed fixture binary | `92318ef3421c7bb4009a89e2faf280f0782136b8f53462ef2df187fd1f18f4d6` |
| Unchanged signed driver | `66a494c5e70c6fd0fba87fa5935bb2893cf0c34d6fc20e6073c4e93ffc7a71d1` |
| Signed report | `b0c69b51d52b5743f4961b16f5d79a3ecad01c6ba6c3e62d1210f4c8b64374c6` |
| Paired driver | `af7682fb3c844e2182ecc86268913bbb16775833c145f5d2e643c5df079be45c` |
| Paired report | `bd21aab1d072e8a82976a32174af1155cc8d5c1811b48a280026f4d4e1d972ea` |
| Each CSV export | `07bb8a04f9a3aedebbe744d38296d12489b16e7d57fbc14cbd8e090c79b0f1fb` |

Run `go test ./apps/gallery -run '^(TestHeader|TestSignedAnalytics)' -count=1` for
static regressions. Rebuild CSS through `task gallery:css`, not a hand edit. The
[signed assessment](2026-09-22-signed-analytics.md#reproduce) specifies fixture startup
and explicit shutdown. Paired driver arguments are installed Playwright entry, Chrome
executable, retained before directory, after directory and a new output directory.
It uses retained local bytes without an app server; preserve both input manifests.

Composition lesson: attribute a failed **full document** budget first. Repeated
app-owned chrome utilities can move into selected CSS while retaining semantic DOM,
native controls and all analytical context. Measure the CSS tradeoff and cold total,
then verify actual keyboard/state/geometry parity. This does not require a new public
navigation API, consumer migration or automatic catalog-wide rewrite.
