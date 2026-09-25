# P07/P08 private evidence retention, 2026-09-15

## Delivered boundary

Explicitly retained two historical packets under the existing ADR-0043/0055 private
store and image viewer. No new runtime, renderer, public API, certification rule or
automatic import. The original P06 packets and original archive catalog are unchanged.
Four immutable records now contain 120 PNG slots: P06's24, P07's52 and P08's44.
The conversion count remains **eight typed adapters, zero certified targets**.

| Packet | Artifacts | Images | Bundle SHA-256 (also `evidence@` ID) |
|---|---:|---:|---|
| P07 menu | 73 | 52 | `66aff1381147060cae5488e8c26a3c4491fb37109ce33a78391fba1a5d5c926f` |
| P08 drawer | 67 | 44 | `1ce15a0ce6e58db27a03c87654f3c8a97580c10111483c8cc5489159364a53dc` |

Both contracts are retrospective, `go-web`, `original-typed-repair-history`.
Their interaction gate stays **failed**, eight others stay **not-run**, all nine stay
incomplete, and verification stays `not-verified`. Reports retain scoped later repair
passes without replacing the failed base. Capture labels are historical metadata,
not independently authenticated behavior, viewport or input claims.

## Independently selected identities and content

Both source members belong to retained archive
`application-ui@b26cbf6e9529c2219452de32b1a9c49ca12d7bc63bca5e48cd286088521ba0e8`.

| Pin | P07 | P08 |
|---|---|---|
| Contract | `0075e974db6688ab3fc43634c009dd235cf0dab96c914e32d62ef0d4e145a995` | `3653355be39bc7872b04d0f39a6b71ace05eefb140bf70a9078f0d916a242ddc` |
| Source | `10ed82d0c337ad246f1ab8687d73cc0a7dc5d834bb7b3ec46a01bc7c5d5f144a` | `1fbde54a0be874618cc295bf3b837fee4d89f38a1543dd9a6e2374bbd1d13415` |
| Base artifact | `7917bf4253fe81b0ef97da581a39a92cc834a024b21c59656e5b36385f0a3ea0` | `34644520c31c87de479b4c61472b6ea3fb0e759f7c757db2fbb291fd9b2883ca` |

Source-free maintainer specs in `internal/designreceipts/overlay_{menu,drawer}_spec_test.go`
pin each selected source/output/runtime/transformer/harness/report. The test-only
exporter checks raw report SHA, exact capture count, each PNG SHA/size/full decoding,
canonical contract and incomplete-gate boundaries before writing no-clobber bundles.
It needs explicit private roots; ordinary tests skip the live exporter when unrequested.
Synthetic tests cover deterministic output, preserved failure, tampering and path bounds.
This is not a general importer or a hostile-same-user filesystem guarantee.

P07 retains four reports: base302 scoped checks/20 PNGs, baseline zoom23 passes/two
failures/two PNGs, repair314 checks/20 PNGs, delivered repair zoom210 checks/ten PNGs.
P08 retains base229 checks/24 PNGs, baseline zoom144 passes/30 failures/eight PNGs,
repair360 checks/eight PNGs and repair zoom182 checks/four PNGs.
These are historical report contents, not newly executed component tasks.

P07's early browser/zoom harness bytes are not recovered; current snapshots do not
retroactively close that dependency gap. P08 repair browser reports bind base artifact
plus repair CSS; a separate retained delivery report connects those bytes to the nested
repair artifact. P08's missing original photo and synthetic fixture remain explicit.
Neither packet retains complete historical execution/dependency closure.

## Publication and replay

Work root: `/Volumes/m2ext/appkit-evidence/appkit-overlay-retention.ooQsuIsc`.
Its independently built `appkit` SHA is
`744fe2fba251eda3c76d49ba9b7d54ac8060882e6c9e4aa645b59f47cb4a8f2f`.
Four separate CLI checks return0: P07/P08 base deliveries and both explicit repairs.
These are current deterministic delivery replays, not fresh browser execution.
The existing `assets-evidence-save` CLI then admitted each prepared bundle with the
independent contract/source/artifact pins above, rechecking original membership.

Original catalog SHA remains
`23a1c87e21181fb944abf551c293b0154953c682ff915e7965332cf381482b84`.
Both old P06 bundle hashes still equal their immutable IDs. New evidence index SHA:
`2db0f878c9113de42a265e5237c5e87953912aa8010f4c37f57ae752bd3f9cdb`.
Private bundle bodies stay under `data/design-assets/evidence/entries/`, excluded
from source distribution. Temporary source/repair evidence was not deleted.

## Viewer and protocol proof

[Read-only probe](fixtures/overlay-evidence/check-live.mjs), SHA
`e1ea7c03f24d09f06d43c897b3f2effda3546f9b3e6760048c68165cbfb59519`, compares **all96 HTTP
PNG bodies** with independently named bundle bytes, hashes and sizes. Fresh isolated
Chrome152.0.7977.84 contexts run with JavaScript disabled and loopback-only requests.
Eight cases cover both families at390/1440px and light/dark: catalog→subject→record,
keyboard type-ahead/Tab/Enter selection, original/typed and failed-baseline/repair pairs,
preserved theme, decoded proportional stacked images and no document horizontal overflow.
Menu baseline zoom exists only in light; dark Gallery cases preserve those light pixels.

Result: **120 checks,96 matching image bodies,eight screenshots,zero errors**.
Final receipt: `inspection-16a9bt/receipt.json` beneath the work root, SHA
`4978301d4cf075c33fd9aecedc4d56ba87efa628ff9d8878a25f7fac94208726`.
Main rehashed four inputs, the separate binary snapshot and all eight screenshots.
They match the first pass's screenshots exactly; desktop-menu/light and mobile-drawer/dark
controls were visually inspected. This is viewer proof, not a repeat
of the historical component keyboard/zoom behavior or complete visual acceptance.
The first pass remains at `inspection-c516o0/receipt.json`. Final harness adds checked
real/current-user/0700 output-parent refusal and30s HTTP timeouts, plus explicit separation
of its caller-selected URL from the binary-file snapshot (not process attestation).
A separate `lsof` check observes PID65722 using the binary named in the
[viewer assessment](2026-09-15-private-image-inspection.md); no immutable process attestation is claimed.

The opt-in `TestRetainedOverlayEvidenceMCPAndGalleryParity` passes with the actual private
root. It uses a real MCP SDK client/server over in-memory transport and the Gallery
handler; exact subject search/get match direct store metadata. Largest response21,229
bytes, below64KiB; no content bytes, nine incomplete gates, failed interaction and
matched original membership are checked. Focused race run passes19.183s.
This does not prove an installed MCP host restarted or loaded these tools.

All four scoped package suites pass: `internal/designreceipts`, `internal/designevidence`,
`cmd/appkit-mcp`, `apps/gallery`; the final MCP rerun passes7.289s, other suites cache
their earlier0.819/1.706/10.534s passes. The exporter race suite passes2.583s; Node's
output-parent refusal test passes1/1. Local document links/budgets and paired versioned
Fathom stanzas pass. No repository-wide or real-consumer suite is implied.

## Repeat and limits

Set `APPKIT_OVERLAY_EVIDENCE_ROOT` to the absolute private asset root for the MCP test.
For the maintained browser probe additionally set `APPKIT_OVERLAY_URL` to the running
loopback Gallery, `APPKIT_OVERLAY_OUTPUT_PARENT` to an explicit private directory,
`APPKIT_OVERLAY_BINARY` to that server's binary and `APPKIT_PLAYWRIGHT` to the existing
isolated Playwright module. It creates only its own private inspection directory.

No consumer/dependency/global configuration changes, new commits or deployment.
RAC's preview remains HTTP200. Scope/guide/changelog now reflect retained availability;
ADR-0043/0055 already cover it. Full pilot gates, other adapter retention, consumer
acceptance and native delivery remain open; Windows stays owner-deferred.
