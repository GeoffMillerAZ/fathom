# P12: actual 200% browser zoom

Scoped follow-up to the [Go-hosted editor](2026-09-16-client-editor-island.md)
and [actual Datastar morph](2026-09-16-client-editor-datastar.md), under the
[prospective zoom contract](../design/client-editor-island.md#actual-browser-zoom-probe).
No editor, stylesheet, runtime or build-output bytes changed. This adds evidence,
not a converted component, public API or certified target.

## Result

The retained browser harness supports an additional `--zoom=200` trial. Each light/dark
view owns a fresh private Chrome profile and sets Appearance zoom to literal200%.
Runtime checks independently require outer1440px, inner720x500 CSSpx, DPR2,
visualViewport.scale1 and document width720 before tasks, during the large-document
capture and after tasks. No signed-in profile or global browser setting is used.

| Evidence | Actual200% | Ordinary100% regression |
|---|---|---|
| Views | light/dark, measured720x500 CSSpx | light/dark at1440/390px |
| Task/global assertions |119 passed,0 failed |218 passed,0 failed |
| Asset observations |44 passed over ten URLs |82 passed over ten URLs |
| Captures |12 |24 |
| Live Go host probes |10 passed |10 passed |
| Cleanup | owned host exits0 on SIGTERM | owned host exits0 on SIGTERM |

Both lanes also check the honest no-script notice separately at390px/100%.
Zoom tasks include existing toolbar/format/link/image/code/preview behavior, keyboard
focus, context/draft/history preservation, atomic refusal, two-instance isolation,
disconnected typing, disposal and remount. The frozen65,535-byte document still has
30 measured key inputs and ten mount/edit/dispose cycles per theme. Actual Datastar
morph preservation and explicitly injected failure/cancellation add18 checks.

Large-document two-frame input-proxy p95 is31.4ms light/30.9ms dark, below the unchanged
100ms screen; no input-window long task exceeded50ms. Small-document p95 is32.4/31.9ms.
Mount diagnostics are12.6/15.7ms, with no invented acceptance threshold. Explicit-GC
deltas retain12 nodes, zero listeners and820,532/809,660 heap bytes; not leak-free proof.

Full captures use exact CDP DIP bounds and validate PNG dimensions, CRCs, decompression
and all rows. The long-document captures explicitly show only the1440x1000 viewport,
with full document geometry retained. Light-code, dark-morph and dark-large-document
images were inspected. This is not original-paired fidelity or pixel-stability proof;
CDP captures may retain transient caret/animation state. An independently run source
check, separate from the browser receipts below, passes all15 shared capture/setup
tests, including malformed/truncated PNG and setup-cleanup failures:

```sh
node --test docs/assessments/fixtures/gallery-analytics/zoom-contract_test.mjs docs/assessments/fixtures/catalyst-settings-adapter/zoom-capture_test.mjs
```

## Retained identity and replay

Private root: `data/design-assets/p12-client-island.UM4cXe/build-GL7iOz/`.
The unchanged output manifest remains
`648d2f0f415813a847464115f67d79c37dba5d2c2be5f98c52cca86ba34bf351`.

- Zoom `trial-YAnrMa/run-receipt.json`: `28e953bd633da0ea0d69ad8f426d0e4f8cd395181ed6aa45b3c4e5fc213c187e`.
- Zoom `browser-RUhlff/receipt.json`: `2c199505150b8ba0adb97111ed572e51e9d2a9f3ea49bf3a48ca68482d72e48c`.
- Ordinary `trial-mV2bhU/run-receipt.json`: `4c5c7589618df76129d1b9574aca867a95381a0823a81f31ca03e4665828d0f9`.
- Ordinary `browser-W2zQL6/receipt.json`: `76cb57961e9d5912bbb9b135ff846b0609acd62f6adcfb031228b52af02e287c`.
- Browser source: `a796c1d95b852f14ca42a6a6879cba0b1703d6d7aa285dab6eb3c0ddc0da0435`.
- Runner source: `0258867ceef9afa3298fbe0ebeeb10f4877d1ce20033ecdd095f74225217671f`.

Each trial retains its exact source/helpers and the browser verifies the input envelope
before execution. Chrome152.0.7977.84, existing Playwright1.63.0 and local Go1.26.3;
machine/configuration and commands are in the receipts. [Replay instructions](fixtures/client-editor/README.md#actual-browser-zoom).

## Still open

Other browsers/zoom levels, native-device input, IME, clipboard, assistive technology,
full source fidelity, large-draft morphing, persistence and real-consumer adoption.
Scale and morph are separate tasks. No blanket accessibility/performance claim;
the certified-target count remains zero. Earlier failures and receipts are unchanged.
