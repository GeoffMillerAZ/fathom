# Prepared P02 pricing comparison

Fixed private static comparison under [ADR-0028](../adr/0028-prepared-private-static-comparisons.md),
not an uploaded-HTML executor, recovered portal baseline or certified conversion.

## Inputs and preparation

Preparer source SHA-256: `00abe475137c427bb350472e9de9912716d7360687da2630b5a76bef353206d1`.
Synthetic test SHA-256: `14ad2229d29d5a26a1bec4150234a066ecffeada01f70b3e010db2288056645c`.
Final preparer synthetic race suite passed13.370s, including effective subprocess
telemetry-off, actual compiler/DOM and cached CSS repeatability canaries.

Attempt: `syntax@902c0695ff832fde0c73bb979cf73aa496098778061dffcff0276c11a6176043`.
Original: marketing archive `8863da80a161b3dd0f3ee2991657edbc29855ff8dc23915dda1f520fd6859675`,
`html/sections/pricing/05-single-price-with-details.html`, 4,710 bytes, SHA-256
`f545f273689b9a4312e5d0eeb66c15dc21d7d318e102e37c39b40f494e017e0f`.

The [explicit preparer](fixtures/html-preview/prepare.go) checks current replay and
original/artifact identities, compiles actual templ and compares namespace, text,
comments and attributes before/after formatting. It builds identical CSS twice using
cached checksum-pinned Tailwind4.3.0 and embeds verified local InterVariable/license.
Go caches, temporary work and effective telemetry-off mode are task-private; no global
configuration change or input download. Total bound declared before build:1MiB/document.
CSS patch/font defaults are missing from the archive: these are chosen conditions,
not a claim about original portal typography or layout.

Two early runs refused before publication: target `templ` instead of actual `go-templ`,
then the original SVG's inert `data-slot`. Actual-translator and exact-attribute canaries
cover both corrections. Resource/behavior refusals and the exact-DOM oracle remain.
Review also corrected effective Go telemetry isolation and receipt/descriptor agreement.
No failed check was counted as a pass or repaired by accepting a new visual baseline.

## Retained result and replay

Two successful preparations returned the same immutable receipt directory:
`data/design-assets/previews/p02-pricing/62a91c913f3860527b9d77b0b2227595d21b35ae1d06fb37a03a55091a30c8b8`.
The second rebuilt with fresh scratch and verified existing output without replacement.
Metadata is embedded in `apps/gallery/pricing_reference.cue`; licensed bodies remain
ignored/private, four documents mode0600 in a mode0700 bundle.

| Document | Bytes | SHA-256 |
|---|---:|---|
| original-light | 493525 | `5d3a5473878298809b4e3c6d0a142e8379c56ffc974ee1a7dcadce0614ca04ad` |
| converted-light | 493545 | `ba9c18b67e564ab335b3a72a51b77779ec76eb1432c191c8f571e14560ff9872` |
| original-dark | 493524 | `6b0352450e4f1672e6e42e64e8c0471a0461430bf9dcf4bf3e89cb0e5c8f148a` |
| converted-dark | 493544 | `544ed6ebb4bc71e4f0b0cc1966a4e4eb8fb7453ddfea131612d62e9e27f4c344` |

Independent review checked receipt/CUE agreement, file/style/font identities, unchanged
original body and equal converted bodies across schemes. Catalogs remained unchanged:
original `23a1c87e21181fb944abf551c293b0154953c682ff915e7965332cf381482b84`;
attempts `2e02af894cc17708636160034006b3317ff48a3453ae9ad820a6cc86ee252cd7`.

## Gallery evidence and limits

Open `/comparisons/p02-pricing`; select `viewport=390|1440`, `preview_theme=light|dark`.
Gallery `theme` is independent. Frames are1,000px high; static links do not purchase.
Serving rechecks replay, complete receipt agreement and exact hashes. Only the successful
outer route admits frames. Documents receive an empty-permission sandbox, no scripts,
forms/remote resources, exact inline-style hash and data-font permission. Existing
source/error routes retain inert/frame-denying policies. Headers are not browser proof.

Synthetic tests cover mismatch/corruption/symlink/size/query/origin/peer/method refusal,
matched frames and no input mutation. Fresh compiled HTTP on port18766 returned200 for
light/wide and dark/narrow, served all four exact hashes and refused external Origin403.
Gallery race suite passed22.753s. Font loading, containment, layout, keyboard behavior
and visual fidelity remain unverified because the in-app browser is unavailable.
One prepared case does not promote a target or make all439 compiler cases previews.

## Owner-reported clipping and comparison workspace

The owner's subsequent screenshot showed the 1440px specimens clipped in narrow
side-by-side columns. This is evidence of a viewer defect, not a source/converted
layout mismatch. [INC-10](../intent/incidents.md) records the feedback;
[ADR-0030](../adr/0030-isolated-comparison-workspace-controls.md) separates richer
first-party viewing controls from unchanged inert prepared documents.

The same route now defaults to an edge-to-edge original/converted flip view. Additional
query controls: `layout=flip|side|stack`, `scale=fit|actual`, `active=original|converted`.
Existing viewport, preview theme and Gallery theme queries remain supported.
Fit keeps the selected 390/1440 by1000 CSS-pixel layout viewport and scales the frame
and surrounding box together. Actual size is100%; browser zoom remains independent.
Both frames stay mounted while flipping; scroll positions are independent, not linked.
Browser fullscreen is an explicit progressive enhancement, including the toolbar.
Without the script, GET forms/side links work with actual-size scrollable previews.

Parent controls admit only the exact first-party script digest through CSP and SRI.
Prepared documents, receipt and all four body/style identities above are unchanged.
No frame-content access, source writes, runtime execution or target promotion is added.
Go checks cover server defaults/modes/refusals, script identity and unchanged child
policy; Node fake-DOM checks cover form/flip/resize/geometry and fullscreen rejection.
The authorized in-app browser still reports unavailable. These checks do not prove
real layout, keyboard, fullscreen or CSP enforcement; owner inspection remains needed.

Final checks: Gallery race suite19.202s, then post-review pricing race checks3.914s;
7 Node tests passed with zero skips, scoped vet and intent links/budgets passed.
Review fixes reserve a stable scrollbar gutter and show an actual-size fallback label
even when enabled JavaScript fails to load. Live binary SHA-256
`e2cc411b75018e0df372ada623e5431172de6a83e177408748a106c603ae67a5`
on127.0.0.1:18766 served all3 layouts, matched script/CSP/SRI and four unchanged
document hashes, refused external Origin403 and kept `/themes` and `/catalog`200.
