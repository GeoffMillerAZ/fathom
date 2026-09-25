# Catalog control-target probe

Local, synthetic Chrome evidence for the catalog's main-content controls. This is not
whole-Gallery accessibility, physical touch, browser-engine parity or certification.
The [assessment](../../2026-09-16-catalog-control-targets.md) retains the failed baseline
and before/after results. The shared header and inline prose links are outside this
standalone-control inventory.

Build the intended Gallery binary, then use the existing approved isolated Playwright
installation. The runner pins its entry-module digest and Chrome version; it installs
nothing and never uses a signed-in browser profile.

```sh
go build -o /absolute/private/directory/gallery ./apps/gallery/cmd/gallery
APPKIT_PLAYWRIGHT_MODULE=/absolute/approved/playwright/index.mjs \
  node docs/assessments/fixtures/gallery-catalog/targets.mjs \
  /absolute/private/evidence-parent /absolute/private/directory/gallery
```

The runner creates an exclusive evidence directory, freezes its own source, hashes
the executable and launches that executable on an ephemeral loopback port with paid
design assets disabled. It retains the observed origin, process identity, drained
stdout/stderr, exact served CSS and full-page captures. Only local catalog/CSS GETs
are admitted. Fresh contexts disable application scripts and service workers.

At 390/1440 CSS pixels in light/dark, it checks search, filtered, empty, bad-cursor and
detail pages. Only visible standalone actions are measured; the hidden JSON action
must be excluded while closed, then measured after native disclosure activation.
Native Tab/Enter tasks search, open a result, return with selection intact and expand
the disclosure. HTTP/CSS/network failures and abnormal process cleanup fail the run.

For a before/after comparison, retain both binaries and run identical harness bytes.
Compare `sourceSHA`, `playwrightEntrySHA`, `browserVersion` and `css.sha256` explicitly;
a passing individual receipt does not prove equal inputs. Bind source/build commands
separately and preserve all earlier failed runs. The runner does not prove complete
build-input closure, cold reproducibility, an immutable release or consumer adoption.

The parsed Go regression is a fast class/structure canary, not pixel measurement:

```sh
go test -race ./apps/gallery -run '^TestCatalog' -count=1
```

## Outer-header and analytics task probe

`header-journey.mjs` separately covers the outer header at 320/390/768/1440 in both
themes, native Sections disclosure/current-page behavior and 44px controls, keyboard
skip/navigation/select type-ahead/apply/detail/return, and missing-versus-zero analytics.
The [assessment](../../2026-09-16-gallery-header-and-missing-scale.md) retains old-source
failures and the repaired result. It is not full accessibility or actual-zoom proof.

```sh
APPKIT_PLAYWRIGHT_MODULE=/absolute/approved/playwright/index.mjs \
  node docs/assessments/fixtures/gallery-catalog/header-journey.mjs \
  /absolute/private/evidence-parent /absolute/private/directory/gallery
```

This runner also retains each served HTML document and checks the existing 32KiB
budget. Its request allowlist covers only the catalog index, analytics and kit CSS;
it performs no mutation or export download. Both before/after use the same probe,
but CSS legitimately differs when header utilities require a pinned rebuild. Compare
the recorded identities, not an assumption of identical CSS. Native type-ahead coverage
does not imply popup-arrow coverage; preserve failed collector attempts separately.
