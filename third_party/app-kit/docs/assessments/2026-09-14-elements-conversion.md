# Elements original/compiled conversion evidence — 2026-09-14

Four frozen interactive Tailwind Plus specimens now translate to constant-only templ,
compile before/after formatting with exact DOM equality, and pass paired Chrome tasks.
**842/842 browser assertions pass; all 34 screenshot pairs are byte-identical.**
This is scoped proof for four fixtures, not catalog-wide compatibility or certification.

## What changed

[ADR-0040](../adr/0040-explicit-elements-syntax-family.md) introduces the separate
`html-elements-to-templ-v1` rule and `elements-syntax` receipt. The read-only command
`appkit data assets-translate-elements` emits deterministic JSON, installs nothing and
accepts no `-save`. [Usage and limits](../design/elements-translation.md) are served
through MCP as `elements-translation`. Gallery/MCP share source-free experimental CLI
catalog ID `reference.elements-translator`; the catalog now contains 29 records.
Existing conversion list/get routes still serve only legacy saved attempts.

The new grammar preserves literal native HTML/basic SVG plus five exact Elements tags,
menu placement and local command bindings. Unsupported input has diagnostics without
partial output. Runtime installation, routes, data, authorization, repeated-instance
IDs and app integration remain host-owned. All receipts remain `not-verified`.

## Frozen denominator and conditions

Sources and their complete hashes remain in the immutable
[source-native baseline](2026-09-14-elements-source-reference.md) and
[original pilot](2026-09-13-conversion-pilot.md). No source was swapped or repaired.

| Case | Preserved behavior exercised in both lanes |
|---|---|
| P03 FAQ | Six disclosure states, initial open panel, keyboard/pointer toggles, sibling independence |
| P07 menu | Focus entry/movement/return, pointer opening, Escape and outside dismissal |
| P08 nested dialog/menu | Dialog opening, nested menu, ordered Escape dismissal and return focus |
| P09 mobile navigation | Narrow disclosure, keyboard/pointer control, account menu and resize |

Every case runs at 390/1440 CSS-pixel widths, light/dark, height 1000 and reduced motion.
Each lane exercises three remounts of its own fragment, not an original substituted
for compiled output. Runtime-withheld controls reject the same FAQ toggle in both lanes.
Open-state geometry/focus is observed; matching screenshot states have no masks,
pixel tolerance, style overrides or baseline replacement.

The host supplies Elements 1.0.22, the same pinned CSS and Inter font as the source
reference. Actual tooling: Go 1.26.3, templ v0.3.1020, Playwright 1.63.0 and installed
Chrome 152.0.7977.84. Browser contexts use fresh isolated profiles, no signed-in portal,
random-path loopback GET-only serving, exact Host/Origin checks, blocked service workers,
no network runtime dependencies and CSP-blocked external images/forms.

## Reproducibility and observed output

The [preparer](fixtures/elements-conversion/README.md) verifies the fixed source
manifest, all 18 declared files and all four actual archive members before conversion.
It replays each receipt, generates/compiles/renders before and after templ formatting,
then compares namespace-aware element/text/comment nodes and ordered attribute values.
Class/text whitespace is not normalized to make the comparison pass.

Two independent preparations produce **48/48 identical files**, including the manifest.
Outputs are private exclusive files beneath random private directories; publication is
manifest-last, with checked retained-directory identities and no replacement.
The browser requires the reviewed pair-manifest hash before reading/serving artifacts.

| Identity | SHA-256 |
|---|---|
| Frozen source manifest | `2fb9ae848b8040f8aee05f74a741253592a53a331201baca299dd813dc121f48` |
| Elements rule | `2cfaf2a258aa4be609834e6453e796effd7d8d68ee227ed3b596e1b5e0e450e5` |
| Pair manifest, both preparations | `2b21ebf83272db1a96799ead455b436912a947dc662133fe8f1f24c77d71883f` |
| Final browser report | `603fb9ba242d9f61cb2bbb02622105f37f897ecb2cfa5b9366fd1bded79c43c6` |
| Preparer | `30a420d99812d76d269a633e9728b35f330e7ad0e9820aa4e2cf2ce060e81add` |
| Final browser harness | `bd872637bfe73f84efcc3e5515d326c766a8ef5055c412fa47c8220117ca6023` |
| Final browser contract | `cdaee9dc33a3767410cccdda2ea84238c1bc16ee6116f9583669f99b67c75b07` |

Private artifacts are retained, Git-ignored, under
`data/design-assets/reference-assets/elements-conversion-1.0.22-pilot/`:
`elements-conversion-a6d89f092f3103dc31828e918ceacf21/` contains both browser runs;
`elements-conversion-d7db6b5b9861040fcd9eeb135511b253/` is the identical preparation replay.
The final report is `browser-CNYzW7/report.json`; earlier `browser-arShTP/report.json`
is retained with SHA `a5d6aa83abe8b565ba72f610b389101b381720ef444fd3fc1c66a3ec760dff90`.

Independent review found that the first harness trusted manifest lane names: a
rehashed manifest could alias compiled links to originals. Actual prepared links
were correct. A new preflight binds frozen source metadata/docs, distinct compiled
fragments and exact wrapper substitution; eleven alias/misbinding mutations now fail.
The unchanged browser tasks pass again after the fix. Independent core/CLI/catalog
and fixture reviews report no remaining actionable findings in their bounded scopes.

The browser report contains 842 assertions, 68 screenshots, 34 equal image pairs,
68 geometry observations, 36 focus observations, 136 admitted local requests and
no runtime exceptions or refused routed requests. CSP blocks the declared external
imagery before routing. Timings are diagnostic second-animation-frame observations,
not INP, presented-frame timing or performance approval.

Main visually inspected the compiled desktop FAQ and dark narrow menu, nested drawer
and open mobile navigation. The FAQ outline, selected menu item and mobile trigger
outline are visible. Missing drawer/profile imagery remains conspicuous. Pixel equality
establishes parity under these conditions, not that either source is fully accessible.

## Verification and limits

- Root `GOPROXY=off go test ./...` passes; focused CLI/catalog/MCP/Gallery races,
  Go vet, Elements/catalog CUE checks and six Node contract checks pass without skips.
- Converter tests include exact replay, rehashed-output/runtime tampering, grammar and
  size refusals, literal-only templ, old-rule refusal and the unchanged legacy digest.
- Fixture canaries reject changed class spacing/text/comments, ambiguous replacement,
  clobber/replacement paths and unreviewed manifests or unexpected denied local requests.
- Catalog/Gallery/MCP tests prove shared identity, source-free discovery and refusal
  to advertise CLI translation tooling as a proven web component.
- The intent-doctor subset passes budgets, ADR membership/shape, relative links,
  changelog shape and matching root intent stanzas. It does not prove host configuration.

Legacy source/schema bytes and rule digest
`0db161e2e0e567ae491661614e0115f4042c3ea76b188f867c8242e44e90eb7c`
remain unchanged. Historical **439/657 legacy passes, five saved attempts and four
typed candidates** remain their own counts. These four new syntax results are not
four additional typed components or proof for the other custom-element inputs.

P08/P09 images remain unpinned/blocked; small source controls, incomplete visible-focus
screening, placeholder actions and missing app backends remain findings. There is no
touch, screen-reader, true-zoom, cross-browser, Datastar-morph, leak, native-host or real
consumer proof. **Zero converted targets are certified.** Next work must earn typed
instance/app-owned integration and applicable family/asset gates before promotion.
