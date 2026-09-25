# P01 typed split-hero adapter

Status: implemented private experimental CLI candidate with frozen-source, compiler,
DOM, exact export/replay, 210-check normal browser, 92-check actual-zoom and two-run
standalone synthetic evidence in the [assessment](../assessments/2026-09-15-hero-typed-adapter.md).
Real-consumer integration, full target verification and task certification remain open.
This is not a certified capability or a catalog-profile promotion. Delivery follows
[ADR-0034](../adr/0034-source-specific-private-component-delivery.md) and
[ADR-0036](../adr/0036-share-private-delivery-assurance.md).

## Frozen source and CLI

P01 accepts only archive
`marketing@8863da80a161b3dd0f3ee2991657edbc29855ff8dc23915dda1f520fd6859675`,
member `html/sections/heroes/09-split-with-image.html`, source SHA-256
`356c19f1095d82529cb54014e3b94a67b7531489bb26b96b5fba8686d94beba9`.
The original stays independently retrievable and immutable.

`appkit data assets-adapt-hero -root PRIVATE_DIRECTORY` emits private JSON. Add
`-out NEW_PRIVATE_DIRECTORY` to create exactly `hero.templ`, `hero.go`, `hero.css`,
`defaults.json`, `USAGE.md` and `artifact.json`, or `-check PRIVATE_DIRECTORY` for
read-only exact replay. Export and check are mutually exclusive and silent on success.
Exit 1 means observed drift; exit 2 means invalid invocation, source or could-not-check.
Existing destinations are refused, never replaced or repaired. Generation and replay
do not fetch source assets, install dependencies or compile output.

## Typed contract and application ownership

`Hero` carries three action destinations (`AnnouncementURL`, `PrimaryURL`,
`SecondaryURL`) and three image locations (`LogoLightURL`, `LogoDarkURL`, `ImageURL`).
All six are intentionally blank in defaults and required by both constructors. Each is
at most 2048 bytes and must be an explicit local absolute path or HTTPS URL; malformed,
opaque, userinfo, control-bearing, encoded-control and scheme-relative forms are refused.
Original source asset URLs are provenance only: they are not downloaded or bundled.
The app owns replacements, rights, pinning, delivery, routes and destination behavior.

Copy limits are bytes: `Heading` 160, `Introduction` 1200, `Announcement` 600,
`AnnouncementLabel`, `PrimaryLabel` and `SecondaryLabel` 100 each, `LogoAlt` 256 and
`ImageAlt` 512. Text must be valid UTF-8, nonblank and control-free. Set
`ImageDecorative` explicitly: decorative images require exactly empty `ImageAlt`;
non-decorative images require bounded plain alt text. Constructor errors must be handled
before response output. Inputs are snapshotted and escaped as text.

| Constructor | Contract |
|---|---|
| `NewHero` | Preserves the original responsive, dark and source-class composition. |
| `NewHeroEnhanced` | Explicitly opts into `hero.css`, loaded after source CSS. |

Compile a COPY in a private app-owned module with pinned `templ` v0.3.1020. The app
owns its Go module, Tailwind 4 compiler/hash, scans of `hero.templ` and `hero.go`, source
CSS, fonts and optional companion CSS. Generated runtime code imports `templ`, not
app-kit. Keep the licensed source and derivatives private; never compile inside the
immutable source store or delivery directory.

## Evidence and private replay

Synthetic source/schema/compiler/DOM/delivery coverage:

```sh
go test ./internal/designadapt -run TestHero
```

The opt-in live replay requires an existing private app-kit checkout. To retain its
0600 artifacts, first create an empty 0700 directory beneath the system temp directory:

```sh
APPKIT_PROBE_CONVERSION_KIT_ROOT=/absolute/private/app-kit \
APPKIT_PROBE_HERO_OUTPUT=/tmp/empty-private-hero-output \
go test ./internal/designadapt -run TestAdaptHeroLive -count=1
```

That probe rechecks the archive/catalog before and after use, runs fresh adaptation and
replay, writes the six-file delivery, and compiles source/typed/enhanced private output.
The original source remains unchanged. A matching result proves those bounded facts,
not browser behavior, accessibility, visual fidelity, task quality or adoption.

The separate browser probe is explicit because it downloads the three recorded source
asset URLs into the named private test output only:

```sh
node docs/assessments/fixtures/hero-adapter/browser.mjs \
  /absolute/app-kit /tmp/private-hero-probe --fetch-assets
```

For later runs use `--assets-from PRIVATE_ASSETS_JSON EXPECTED_SHA256`, supplying
the retained manifest's independently recorded digest. This reuses exact bytes without
network asset fetches; every run creates a new private `browser-*` evidence directory.

Use a fresh unsigned test profile only. The controlled local font is a test condition,
not portal provenance. No signed-in profile, consumer data or source-store write is
permitted. Browser completion still would not establish standalone real-consumer use,
full target verification or task certification.

The standalone fixture takes four absolute inputs: the kit checkout, a retained P01
probe containing pristine `delivery/`, `typed.html` and its current built `appkit`, the
explicit Go binary, and a public Go module ZIP-cache directory:

```sh
node docs/assessments/fixtures/hero-adapter/standalone.mjs \
  /absolute/app-kit /tmp/private-p01-probe \
  /absolute/go /absolute/go-module-cache/download
```

It is a macOS `sandbox-exec` proof. Two new private system-temp consumers start with
cold module/build caches, use only the pinned offline file proxy, and compile copies of
`hero.templ`, `hero.go` and `defaults.json` with a generated first-party synthetic test.
The sandbox denies network plus reads from kit source and the pristine delivery; the
built probe CLI checks that delivery before and after. The task requires exact faithful
bytes against `typed.html`, renders the enhanced wrapper, checks the actual-used module
closure and seeds a missing-generated-file failure. Commands and outputs are bounded and
retained privately. Matching runs prove standalone synthetic compilation/replay, not a
CSS/browser pass, real consumer adoption or target certification.

All probes may use a task-owned private temporary parent selected with command-scoped
`TMPDIR`; they do not require or change global configuration. The probe and retained
asset/receipt paths must resolve beneath that same chosen parent; the explicit public Go
module ZIP cache need not. The actual-zoom probe requires the final normal browser
receipt and its independently recorded SHA-256:

```sh
TMPDIR=/absolute/private-temp-parent \
node docs/assessments/fixtures/hero-adapter/zoom.mjs \
  /absolute/private-p01-probe \
  /absolute/private-p01-probe/browser-NAME/receipt.json \
  EXPECTED_NORMAL_RECEIPT_SHA256
```

It uses Chrome's actual 2x zoom rather than CSS scaling and automates native Enter,
Back and reverse-Tab journeys. Its selected CDP viewport captures have a bounded
nonblank canary. This is one desktop geometry in headless Chrome, not physical input,
screen-reader, cross-browser, exhaustive visual or full target certification.

## Revisit when

Revisit for source drift, failed replay, asset provenance/delivery ambiguity, inability
to preserve the composition, or a named consumer requiring a certified integration.

The [performance experiment](../assessments/2026-09-16-hero-performance.md) triggers the
cost-budget review: cold bodies are658,700 bytes against524,288; timing/heap pass the
named local workload, while cached-font body proof remains failed. Its
[snapshot-first runner](../assessments/fixtures/hero-adapter/PERFORMANCE.md) detects
missing font, delayed CSS and oversized-image controls. Preserve the original and
fixed budgets; a separate asset-delivery candidate needs glyph/fidelity proof. This
does not promote the adapter or establish full performance/consumer acceptance.

The separate [font-delivery experiment](../assessments/2026-09-16-hero-font-delivery.md)
preserves tested glyphs/pixels and reduces cold bodies to499,059 bytes (24.2% less).
All eight cold cells pass; eight warm font-body failures remain. Original-font fallback
and mixed-script cost are explicit; [replay](../assessments/fixtures/hero-adapter/FONT.md)
requires its private pinned tooling and does not change the six-file adapter delivery.
