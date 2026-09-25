# Private retained-image inspection, 2026-09-15

## Delivered boundary

[ADR-0055](../adr/0055-private-retained-image-inspection.md) extends only ADR-0043's
Gallery image-access boundary. Catalog reference pages now link by exact capability
ID to private target evidence. A record offers native capture selection, optional
stacked comparison and full-size PNG inspection. Existing metadata shapes and catalog
profiles remain unchanged; no CLI reference became a certified web component.

`ReadImage` reuses the exact checked bundle snapshot behind `Read`, including catalog,
all declared content and original membership. Only receipt-declared `role=image` slots
can return bytes. PNG signature, dimensions and full decoding must pass within 8 MiB,
8192-pixel edges and 16 million pixels. Response bytes match the retained image exactly.
The metadata API and MCP remain byte-free. Missing/non-image slots, unsupported media,
corruption and invalid private requests do not return an image. Invalid images retain
their recorded hash/size and recovery text in the HTML view.

HTML permits same-origin images only on a successfully rendered evidence detail. Raw
validated PNG responses alone allow inline styling for Chrome's generated image document;
scripts/forms/frames remain denied. Errors and other private routes keep their existing
policy. No source execution, new browser runtime, network proxy or automatic import.

## Private availability at viewer validation

P06's existing immutable record contains 24 retained screenshots:
`evidence@994e15283cbaa57570986429816bd42fadf91455531985c69c4ca48b7c7bbe1e`.
Its other historical record has no image slots. Both retain nine incomplete gates and
`verification=not-verified`. Other adapters' temporary captures are not silently indexed.
Eight typed adapters and zero certified targets remain the current conversion count.

Subsequent [P07/P08 retention](2026-09-15-overlay-evidence-retention.md) explicitly adds
96 menu/drawer captures; the original viewer run below still concerns P06's 24 images.

Reviewed binary: `/Volumes/m2ext/appkit-evidence-images.6Z1gZ3jP/gallery-v2`, SHA-256
`0b78a501e74fd8b52d52a4481a8bf1dbaaad509aa27f0fdaab85580c56441307`.
It serves an isolated loopback preview at `http://127.0.0.1:55031/target-evidence` while
PID65722 is running. This is not deployment or permanent service configuration.
The earlier test server PID64465 was stopped; RAC PID88496/port56589 remained HTTP200.

## Verification

- All tests in `./apps/gallery`, `./cmd/appkit-mcp`, `./internal/designevidence` and
  `./appkit/catalog` pass on the final source (10.471/6.041/1.699/1.096 seconds).
  This is four package suites, not the repository or consumer suite. Private optional
  tests retain their normal skip semantics; direct private proof is separately below.
- Focused Gallery/image-store race tests pass (5.130/6.203 seconds), including exact PNG
  bytes, bounds/CRC, source/bundle tampering, non-image refusal, method/origin/Host/query
  rejection, unchanged metadata, unavailable-image recovery and eight catalog links.
- Catalog/MCP parity, fixed discovery evaluations, evidence metadata protocol, decision
  index and current guide tests pass. Existing public catalog IDs/profiles/revision
  remain unchanged; document content identities change normally with updated guidance.
- Pinned Tailwind4.3.0 regenerates Gallery CSS. No source CSS or screenshot was recolored.

Maintained [browser probe](fixtures/evidence-images/browser.mjs), SHA-256
`e7fe9aaa0719927b0ec1223ebbe2ebcdadc925b6cd4d242f6b086f739ec69c8f`, runs installed Chrome
152.0.7977.84 headless in fresh contexts, no signed-in profile. JavaScript is disabled;
only loopback requests are admitted. It passes **89 checks**, with **eight captures**
and **zero browser errors**, at 390×1000 and 1440×1000 in light/dark.

Tasks: catalog → exact subject records → capture; keyboard type-ahead chooses actual
native options, Tab moves focus, Enter submits GET; the selected pair stacks with
undistorted dimensions and no document horizontal overflow; full-size PNG hash matches;
Back and theme changes preserve selection/pixels; returning to the list drops capture
parameters while retaining filter context. Focus ring width/button height are measured.
Mouse/touch names inside historical capture IDs remain authored metadata, not input proof
from this viewer run. Native popup arrow navigation and physical keyboard use are not proven.

Browser receipt: `.../browser-fjgJDf/receipt.json`, SHA-256
`55c54f20ecb5aa8031aeb383a4b94ac17ffd5b098ea0c643f580d4dbdfb8d199`.
Main independently rehashed all12 recorded inputs and all8 captures; visually inspected
390-light controls/figure and 1440-dark figure. Automated task elapsed was4.42–4.99s,
not human task time, a payload budget or performance qualification.

The separate [live byte checker](fixtures/evidence-images/check-live.mjs), SHA-256
`68bdf4e94c08b1a8c1197c3e384a5c8afd4e3161c8f54e61de2f622cbf7cc97d`, independently hashes
the named private bundle and compares **all24 HTTP image bodies** byte-for-byte with its
retained PNGs. All match, with no errors and unchanged bundle identity. Receipt:
`.../http-Z2u9yS/receipt.json`, SHA-256
`cb807c1983837155ca0bc18d95169ee229e5963597f2de1728ff0f9a71164a0f`.
All abbreviated receipt paths use `/Volumes/m2ext/appkit-evidence-images.6Z1gZ3jP/`.

## Failures retained, not rewritten

- `browser-PBssvQ/failure.json` (`02ef0a53d85010476259ed3857ff4da558c398a69a9b02c10639d394a12cc0d7`):
  seven checks then uncommitted arrow-key option selection. Tab had not changed values.
- `browser-ZiC6nj/failure.json` (`6def9cfe0af42d0acd067b59cc06563c025ebb38c0d48efa67ec3d069ffad7b5`):
  four checks then a Space/Home/Enter focus-sequence failure. Direct diagnosis found
  keyboard type-ahead works; final proof uses it rather than programmatic option selection.
- `browser-Txs6d6/failure.json` (`60c7c75e31b79d0a8fc4a02c95071efb605be55ff9aaa6af82b7f2def0c995ac`):
  fifteen checks, wrong-case theme locator and three Chrome image-document inline-style
  CSP errors. Corrected selector and the reviewed PNG-only policy precede final rerun.

## Intent and limits

Updated ADR indexes, target-evidence guide, scope and changelog in this change. Corrected
stale application-shell guidance using the current Fathom reader/test hashes matching
its [authorized adoption](2026-09-15-fathom-handler-read-failure.md#authorized-reader-adoption);
freshness/clipping remain open, with no consumer edits or new runtime claim.

Impeccable guided native task controls, unclipped single-column captures and preserved
source pixels, not a new visual baseline. No commits, dependency/global changes or real
app data. Root Go pins are unchanged. G1–G6 remain active: this viewer does not complete
live component availability, other adapter retention, full accessibility/zoom, target
fidelity, performance, consumer adoption or native delivery gates. Windows stays deferred.
