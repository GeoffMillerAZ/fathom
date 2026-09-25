# P09 private evidence retention, 2026-09-15

## Delivered boundary

P09 now has one immutable historical packet in the existing private Gallery/CLI/MCP
evidence catalog. It retains147 artifacts, including114 complete selected PNG captures.
Together with the unchanged P06/P07/P08 packets, five records retain234 images.
The first-party catalog remains34 records, nine typed candidates and zero certified targets.

The contract is retrospective, `go-web`, `original-typed-repair-history`.
Interaction stays failed; eight other gates stay not-run; all nine stay incomplete.
Exact bytes and original membership are checked, not authenticated report meaning,
full historical dependency closure, present runtime behavior or consumer acceptance.

| Selected run | Captures | Retained report SHA-256 |
|---|---:|---|
| Original/reference/typed base |20| `2dfad48ffd46e6187ab4809b3a7dd65675a1af48fdbf1702c1e01ca1102dbe48` |
| Failed base actual200% |34| `d537109542046601845d0e53f45dbf6dea92efc74abe0138490cd1da731e412f` |
| Runtime repair actual200% |52| `ef85abc90d63e79ed4b47d1ce1920d0870862308cd755e6252c542b77311c533` |
| Delivered repair normal100% |8| `bc8190f608f7d53d172650481cf7ebce34b5390f318ca4c110c1151af131b7a0` |

These are complete capture lists from four independently selected reports, not every
P09 experiment. The [navbar assessment](2026-09-15-navbar-typed-adapter.md) retains the
other runs and limitations, including the lost early harness and rejected cropped
captures. None of those rejected captures becomes successful visual evidence here.
Different variants and viewports are separate observations, not automatic fidelity pairs.

## Identity and capacity

- Bundle/record: `evidence@1cb9f80bebbd697520e24c9e85992eaef3f285a0ba57241483e6f1b7b6188361`.
- Contract: `ee38b2bd5a46060dabca0b8d15be94ad0d92f4c688191e6233ee208c2a17427c`.
- Receipt: `8164cd2961bdcf5a205641c6bfe96210bcf2065323045788a823b547a036ae4b`.
- Source: `79cdbf1cd163a13cb77cc96c4cf8046a088ad8f18757658408372d437400b9d7`.
- Delivered artifact: `67f015c1efa6c41b2acb38d56ac00ecc24b4c7d406729655d8330d425a5a6137`.
- Encoded bundle9,470,276 bytes; decoded supplied content7,070,776 bytes.
- Contract14,693 bytes; receipt20,908 bytes; maximum gate inventory54 slots.

The previous32-entry gate bound refused this packet without writing output.
[ADR-0061](../adr/0061-bound-larger-historical-capture-sets.md) raises it to64 in Go/CUE,
with exact32/33/64 admission,65 refusal and unchanged non-certification tests. Byte
ceilings are unchanged. **Rebuild private writers/readers before selecting this packet**;
old binaries reject its larger gate inventory. Old packets need no migration/rewrite.

## Preparation and saving

The source-free pinned spec and opt-in exporter live in
[navbar_overlay_spec_test.go](../../internal/designreceipts/navbar_overlay_spec_test.go)
and [navbar_overlay_live_test.go](../../internal/designreceipts/navbar_overlay_live_test.go).
They reuse the existing bounded exporter without repartitioning captures into unrelated
gates. Selected inputs retain all nine delivered files, original Elements/font,
transformer/schema snapshots, maintained fixtures, the generated runtime runner and
the final executed delivery harness. This is not reconstructed execution closure.

Work root: `/Volumes/m2ext/appkit-navbar-retention.mBbpWeLh`.
The successful private preparation is `prepare.eWTIHE1k/navbar.json`; the earlier
pre-limit `prepare.2PS31sfW` remains empty. Two encodings match exactly. Main independently
rehashes147 artifacts and the separately selected contract/source/output/report pins.

The newly built CLI SHA is
`fb5808e5676689b1afcd99f149bc62aaf8a7c13080d814502d0106748a7bc60a`.
Its source-replayed nine-file repair check returns0. All four older records remain
readable with matched original membership. Explicit `assets-evidence-save` admits
the reviewed packet using the independent contract/source/artifact hashes above.
The0600 bundle remains in the ignored private `data/design-assets/evidence/entries/`.
No original or temporary source evidence is removed.

Original catalog SHA remains
`23a1c87e21181fb944abf551c293b0154953c682ff915e7965332cf381482b84`.
The new evidence index SHA is
`cc27395a5be9e3da60fd4fed41da62e24b220cc95e74efb25546c85d39571001`.
The four previous bundle hashes still equal their immutable IDs.

## Viewer and protocol verification

The maintained [viewer probe](fixtures/navbar-evidence/check-live.mjs) checks all114
served image bodies, complete capture choices, keyboard comparison and full-size/Back
navigation at390/1440px in light/dark with page JavaScript disabled. It retains executed
harness/configuration bytes before requests and gives image fetching a180-second
aggregate deadline. Caller-selected URL identity is distinct from binary/process proof.
The source-free refusal test checks invalid origin, missing reviewed pin and nonprivate
or symlinked output parents before evidence/browser access.

A separately built Gallery includes the new reader; pre-existing previews are not
restarted. Its binary SHA is
`d0b1701e7126cd8d9c2eed47f00893418a8e13784623b61b81b5399d4df3a357`.
The separately observed running process is PID27903 at `http://127.0.0.1:64952`.
The earlier pre-capacity test process created in this turn was stopped; the three
pre-existing Gallery previews remain running. This is not immutable process attestation.

The first viewer receipt `inspection-prof8p/receipt.json`, SHA
`46e67ca6ed86c4d58176ccc6d123098b91e9494adfa485a9f800e59b79d42dae`, passes96 checks,
114 exact image bodies and four controls captures. Main inspection found those
screenshots barely showed the selected images. Their control evidence remains valid,
but they are not claimed as a visual inspection of the images themselves.

The capture-complete follow-up retains full measured viewport screenshots of each
selected image, with explicit full-image in-viewport bounds, without changing pixels,
page layout, viewport dimensions or earlier task assertions. Final result:
**104 checks,114 exact image bodies,12 screenshots,zero errors**. Four captures show
controls and eight show the complete displayed selected image. The final receipt is
`inspection-e1ActG/receipt.json`, SHA
`45196266b7cc1fe502b5eb2b7c600b1772a9ffc50081aea5b37c21fc8c7c96b3`.
Executed harness SHA:
`cc794a79bd65b3d07718d1d01520a217ea52775e0cbd7e91889204a3fbc44200`.
Configuration SHA:
`a2b0a0f6eba19ccc72f63c250a657a8076f95f892cf395c26d501c05d30cd8ed`.

Main rehashed the harness/configuration and all12 screenshots, verified exact390/1440
by1000 dimensions, and inspected wide-light account plus narrow-dark account/minimal
views. Chrome152.0.7977.84's owned parent PID28571 is absent. Descendants were not
inventoried; no blanket all-process cleanup claim follows. Local image request/body
measurements were185ms median,195ms p95,249ms maximum and21.3s total across114 requests;
this is one same-host run, not a controlled cold-cache or product performance gate.

`TestRetainedNavbarEvidenceMCPAndGalleryParity` passes over an actual SDK in-memory
client/server and the Gallery handler. Direct store, MCP and Gallery agree: search
1,291 bytes and exact metadata37,925 bytes, below64KiB with no content payload.
Source/output selection,114 images, original membership and nine incomplete gates
are checked. The combined race/protocol/document run passes12.690s. This does not
prove an installed MCP host restarted or loaded the larger-packet reader.

Default source-free race suites pass: designreceipts3.711s, designevidence10.461s,
CLI5.485s and Gallery25.034s. The opt-in preparer passed separately; its ordinary
skip is not live proof. Node authority-refusal checks pass1/1. No consumer, dependency,
global configuration, commit, tag, push or deployment change is included.
