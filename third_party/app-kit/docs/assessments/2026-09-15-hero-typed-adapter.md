# P01 typed hero: source fidelity and private delivery

Date: 2026-09-15. Working-tree evidence, not a release or certified target.
Contract: [hero guide](../design/hero-adapter.md), ADR-0023/0034/0036.

## Delivered behavior

- Frozen P01 now produces typed Go/templ copy, three app-owned action destinations,
  three app-owned image URLs and explicit decorative/nondecorative image semantics.
- Both constructors validate bounded plain text/URLs before output, snapshot values,
  escape content, reject canceled rendering before output and propagate writer errors.
- Original classes, structure and whitespace remain in `NewHero`. `NewHeroEnhanced`
  adds one wrapper and opt-in CSS for wrapping, 44px controls, arrow spacing and focus.
- `assets-adapt-hero` defaults to private JSON. Explicit `-out` creates six immutable
  private files; `-check` uses fresh original-source replay. No implicit download,
  dependency installation, compilation, real route/backend or app-kit runtime import.
- Metadata-only `reference.hero-adapter` joins MCP and Gallery CLI discovery. The
  shared catalog has 31 records and six private typed adapters, not six certified UIs.
  Legacy 439/657 syntax/compiler/DOM coverage and five saved syntax attempts are unchanged.

## Verification

| Evidence | Observed result | Boundary |
|---|---|---|
| Synthetic source/model tests | Reject shape/identity changes, unsafe URLs, invalid byte limits; exact-limit acceptance, all six bindings and image-alt semantics pass | Does not prove the purchased source visually |
| Actual frozen source | Repeated generation, exact replay and compiled DOM match before/after templ formatting | Only the six declared URLs differ; original archive/catalog unchanged |
| Actual CLI | Eight invocations exercise JSON, compiler-export match, new export, check, no-clobber, missing delivery, invalid flags and deliberate drift | 0/1/2 exits and silent export/check; no app backend |
| Affected package suites | `go test -race ./internal/designadapt ./appkit/catalog ./cmd/appkit ./cmd/appkit-mcp ./apps/gallery -count=1` passes | Paid-source/browser tests are separate opt-ins |
| Initial browser screening | 198/198 checks, 36 captures and six exact source/typed screenshot pairs | Chromium 152.0.7977.84, 390/768/1440 by 1000 CSS pixels, light/dark, DPR1 |
| Native journeys | 48 pointer destination visits; 16 keyboard focus checks and six keyboard Enter/Back journeys | Synthetic loopback destinations, no account/purchase/data mutation |
| Enhanced focus | Opaque 2px rings, contrast 6.46:1 light and 5.67:1 dark; tested viewport/ancestor clipping checks pass | Computed geometry/colors plus reviewed captures, not full accessibility certification |
| Actual 200% zoom | 92/92 checks and eight captures at 1440 outer/720 inner CSS pixels, DPR2 and visual scale 1 | One automated headless-Chrome desktop size, not physical input, screen-reader or cross-browser certification |
| Retained-asset canary | Wrong expected manifest digest is refused before browser launch | Source URLs plus independently supplied retained-manifest SHA are required for replay |
| Standalone consumer | Two cold private builds run 24 bounded commands; generated Go, faithful/enhanced renders and test binaries are byte-identical | Synthetic macOS sandbox rehearsal, not real consumer adoption or target certification |

Browser pages have no application script elements, enforce `script-src 'none'` and
serve exact local image/font/CSS resources. The test's browser evaluation is instrument
code, not application JavaScript. Network policy blocks external page requests.
All three original image URLs were captured once, then reused by exact byte identity.

The source baseline retains a 40px primary link and 24px secondary link; source-default
and typed-default output agree. Faithful maximum-length synthetic copy overflows in all
six viewport/scheme cases. Enhanced output has no horizontal overflow and all expected
visible links meet 44px in the tested baseline/long-copy cases. No hidden masks, changed
goldens, truncated content or source edits turn these inherited failures into passes.
The announcement is deliberately absent below the source's 640px breakpoint.

Main-agent visual review inspected desktop reference, narrow dark enhanced, a focused
desktop control and narrow long-copy containment. The enhanced inline-flex treatment
initially lost the gap before the arrow; a scoped 0.25em gap repaired it. Faithful output
was not changed. The large source image and scheme-swapped logos are actually present,
not replaced with blank rectangles.

## Standalone synthetic consumer

`standalone.mjs` copied only the delivered template, model and defaults plus the exact
typed oracle, pinned module files and a generated first-party consumer test into each of
two new private system-temp roots. Both cold runs used the pinned offline public module
ZIP cache to generate `hero_templ.go`, compile a test binary and execute the named
`TestDeliveredHeroConsumer` task. Removing generated Go then failed with `undefined:`.

Both runs produced identical `hero_templ.go` (`21faed5b...cd45`), faithful HTML
(`ab6cba0e...d421`), enhanced HTML (`5c1a1789...9a2c`) and test binary
(`affea782...b7c0`). The full faithful and enhanced hashes equal the corresponding input
hashes in the final browser receipt; faithful output also equals the retained
`typed.html` byte oracle. `go.mod`, `go.sum`, copied delivery inputs, the pristine
six-file delivery, built `appkit`, archive and private catalog remained unchanged.

The macOS sandbox canaries denied network, kit-source and pristine-delivery reads in
both runs. The actual-used compiler/consumer closure contained no app-kit/fleet module
or replacement. This proves reproducible isolated compilation and rendering of a
synthetic task, not CSS/browser behavior, a real consumer integration or certification.

## Final enhanced-CSS and browser follow-up

The final source/compiler/DOM replay, eight-command CLI exercise and five affected Go
package race suites pass with the original template, model and faithful render bytes
unchanged. Enhanced CSS now gives arrow-bearing links a shrinkable grid text track and
an independent arrow track, plus a scoped scroll margin for keyboard focus at the
viewport edge. The normal browser rerun adds 12 link-content containment assertions to
the initial 198 checks and passes 210/210 across six viewport/scheme pairs.

| Final evidence | SHA-256 |
|---|---|
| Source/compiler receipt | `5e2e7e96f556cd5df3988abfba1b28d55e4dcd87e2924f577becacefa01184bf` |
| Eight-command CLI receipt | `99615ac11f9bc239c7f0a2e092ca70e6f71c27f12080e137132d279e8e3bbf31` |
| 210-check normal browser receipt | `4ba0ff93f8d0524373d701f2618dc336e273878f11fc2c112f4b953e716ac1bd` |
| 92-check actual-zoom receipt | `8d867bc6319eabd070eea17f1fb3c5a05cc58ebaab3b00651724972279fc6857` |
| Actual-zoom harness | `019fe9c290c34d611852d902bc99c3a7620ea99be5a98358b2db665497d9445b` |
| Fresh standalone receipt | `dc997a4cba7507823099e69c0ea3ba9187daa83d3cc7ae87b6f8da6644d4200d` |

The separate actual-zoom run is hash-bound to that 210-check receipt. Chrome reports
actual 2x zoom with 1440 outer/720 inner CSS pixels, DPR2 and visual scale 1. Its 92
checks cover 12 native Enter/Back journeys, 24 contrasted and unclipped focus checks,
eight reverse-Tab checks, four full-document DIP captures and four painted viewport
focus captures. Every capture has its own nonblank guard.
Back/forward cache is enabled by removing Playwright's one disabling default flag;
there is no application JavaScript. Main-agent visual review covered final normal
desktop/narrow-long views plus selected full baseline and painted light-baseline and
dark-long-focus zoom captures, not every full-page state or a complete accessibility
review.

Same-harness regression inputs beneath external parent
`/Volumes/m2ext/appkit-evidence/appkit-hero-proof.o6s0RX5G7G` retain the pre-grid link overflow at
`regression-overflow/zoom-juXkz2/failure.json` (`5d07342e...ddd18`) and the
pre-scroll-margin bottom-edge focus failure at
`regression-focus/zoom-d8oeDN/failure.json` (`a379e21a...fa7b`). Both exit 1, record
successful cleanup and have no surviving owned Chrome PID. The earlier 88-check
`zoom-tqP6yb` receipt had correct
DOM evidence but blank Playwright focus screenshots; it is not image proof. The final
run uses CDP viewport capture plus the nonblank canary rather than reinterpreting those
old images.

## Retained private evidence and identities

Initial root: `/private/var/folders/2r/2zrkv4fd549bnlc5vwfzd4b00000gn/T/appkit-hero-final.1PMYuX1yvT`.
`delivery/` is pristine; `cli-delivery/` intentionally retains the drift canary and is
not a consumer input. Licensed originals and derivatives remain outside public fixtures.
Final root: `/Volumes/m2ext/appkit-evidence/appkit-hero-proof.o6s0RX5G7G/probe.D2hJLGunWJ`.
Fresh standalone root: `/Volumes/m2ext/appkit-evidence/appkit-hero-proof.o6s0RX5G7G/hero-standalone-xC7mNT`.

### Initial implementation identities

| Artifact | SHA-256 |
|---|---|
| Original P01 member, 2463 bytes | `356c19f1095d82529cb54014e3b94a67b7531489bb26b96b5fba8686d94beba9` |
| Composite adaptation rule | `69dba0f1c25327a276770692cfda0def86c309ce5b4e85f99e1ba538b243361a` |
| Generated template | `44af63390279edc24b72415b6f6e32d03af6bafcafccc1ed722060124b5e85b1` |
| Generated model | `78315cfafe0cbce84b121704a58ee3e9488a287d3674e8b0ef1f0954d0f70a75` |
| Enhanced CSS | `9df47c304846e49e290dea915fed9b5cfaf85b3c9c211c85bba6da4e7dbc3cab` |
| `compile-receipt.json` | `a910f65bde471a4e3c2a06e363def14044314d34c87906677118f28f7774fc37` |
| `cli-receipt.json` | `409433cb4f59955443aa9ce0e4fbda02c92a1879f8e6f9aad014ebd256628c20` |
| `browser-nMhzKZ/receipt.json` | `7a38dbf3bd0333f4cf21851a7b2b18fd0ee76c32dd960bae0d4f9a9c274dd0cc` |
| Browser harness | `6d205ac5f574b6ff1f78db48676356cd5cfe6bfa57a14231ae257d52f746fc15` |
| Standalone receipt | `fde554ddf8b3947c57d51a492f9664fe781fe234c685c4fbca5da39ae5a5e709` |
| Standalone harness | `7fbacbdd4b123552e3aab26a44cd5256a2db1a533024366edc57aa38dd9b2cc5` |
| Captured light logo, 1057 bytes | `6266da6bd8587743482853fb382bd4a4dfe46e87afc8f67c6af1468ec53f0ac5` |
| Captured dark logo, 1057 bytes | `14b0469b127db7af9743953b234c6cde55ab147ca583fe8f544e638030b8e292` |
| Captured photograph, 288581 bytes | `fccb93f9b47a296e29d8e594b891b7206b530565ef467286ca21e8d71d2d51a4` |

The source asset manifest is retained at sibling root `appkit-hero-browser.mo54YnnFQ9/browser/assets.json`,
SHA `6203a33dd42a369bbe39be41b5169b75dfda5f5825a0af0dcc9b1b0658e06d28`.
The browser harness verifies the local Tailwind 4.3.0 binary and the retained font source
document. InterVariable is 352240 bytes, SHA
`693b77d4f32ee9b8bfc995589b5fad5e99adf2832738661f5402f9978429a8e3`.
These are controlled rendering conditions, not recovered portal font/CSS provenance.

The final CSS is `0dc8b35bc170f6d987c2eaf2c9a46c84ba1b981a0bc3ab10d33082703f85fa13`;
the composite rule is `3e52ad3e819920b1b9bd8f69727a1d9348e4b47b86fb7d50e33510593c14d306`
and artifact is `05093df0dfcdaed5c61b22a2685eec0d783ca8b690fc74e16c0adc9f8eff8570`.
The fresh standalone receipt's two runs agree internally, and its four output hashes
match the initial receipt.

One disk-full standalone attempt stopped during the second compiler pass. Its partial
move to external storage was reconciled: every remaining source entry exists identically
at the destination and known failure/first-run evidence is retained, but no pre-move
manifest exists, so completeness of that failed attempt is not claimed. It is separate
from the successful fresh standalone receipt. The retained destination is
`/Volumes/m2ext/appkit-evidence/appkit-hero-proof.o6s0RX5G7G/retained-disk-full-attempt`; its 142 MiB
source residue remains at
`/private/var/folders/2r/2zrkv4fd549bnlc5vwfzd4b00000gn/T/hero-standalone-sw4rSx`.

The first browser attempt remains at `appkit-hero-browser.mo54YnnFQ9/browser/failure.json`:
Back ran before destination load settled. The harness now waits for the destination URL
and network-idle state. Earlier 151/197-check runs remain historical; the initial
198-check receipt binds its then-current harness and independent asset manifest. The
wrong-digest canary
is retained at `browser-rmGcDJ/failure.json`. Normal browser-host and server closure is
recorded in `browser-nMhzKZ/cleanup.json`; forced-kill descendant cleanup is not claimed.

## Remaining work

No real consumer adoption, exhaustive full-page visual review, screen-reader,
touch/cross-browser, adverse-network or production performance evidence is claimed.
The full Inter font is not payload optimization. General family conversion, automatic
repair and target gates remain open; this source-specific candidate does not certify
other hero designs.

Intent/design skills kept original identity and usability improvements separate. No new
runtime, architectural exception, plan completion, release, deployment or consumer edit.
