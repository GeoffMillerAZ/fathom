# Inspect the retained P09 history

This maintainer-only probe checks the existing private Gallery. It does not execute
vendor source, certify a component, install dependencies or change the evidence store.
Read the [assessment](../../2026-09-15-navbar-evidence-retention.md) for independently
reviewed pins and the [reader migration](../../../adr/0061-bound-larger-historical-capture-sets.md).

Select an already running, current loopback Gallery and an existing private0700 output
parent. Use the already-approved isolated Playwright installation, not a signed-in profile:

```sh
APPKIT_PLAYWRIGHT=/absolute/isolated/node_modules/playwright/index.mjs \
  node docs/assessments/fixtures/navbar-evidence/check-live.mjs \
  http://127.0.0.1:PORT /absolute/private/design-assets \
  1cb9f80bebbd697520e24c9e85992eaef3f285a0ba57241483e6f1b7b6188361 \
  /absolute/private-output-parent
node --test docs/assessments/fixtures/navbar-evidence/check-live.test.mjs
```

The probe retains its exact harness/configuration before requests, all image response
hashes, selection/geometry checks and separate controls/displayed-image screenshots.
Failures get a new `failure.json`; no old receipt or image is overwritten. Cleanup
checks the owned Chrome parent, not every descendant. Browser tasks use page JavaScript
disabled, keyboard/native forms and same-origin PNG navigation. These observations
are not component-behavior, full accessibility, performance or host-adoption proof.

The separate protocol test uses an actual MCP SDK client/server over in-memory
transport plus the Gallery handler and direct store, with no installed-host claim:

```sh
APPKIT_NAVBAR_EVIDENCE_ROOT=/absolute/private/design-assets \
APPKIT_NAVBAR_EVIDENCE_SHA=1cb9f80bebbd697520e24c9e85992eaef3f285a0ba57241483e6f1b7b6188361 \
  go test -race ./cmd/appkit-mcp -run '^TestRetainedNavbarEvidenceMCPAndGalleryParity$' -count=1 -v
```

For packet preparation, `TestNavbarOverlayEvidencePrepare` in
`internal/designreceipts/navbar_overlay_live_test.go` requires explicit reviewed input
roots and an existing private output directory. It prepares only; CLI saving is a
separate action requiring independently selected contract/source/artifact hashes.
Ordinary tests skip opt-in private preparation/protocol checks, never call them passed.
