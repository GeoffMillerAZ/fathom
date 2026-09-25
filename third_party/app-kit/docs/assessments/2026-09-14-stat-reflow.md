# Shared stat readability and pinned-build follow-up

G3 improvement returned from Fathom to the existing `stat.Render` / `RenderBand`,
not a new metric model or chart API. The task is reading a value, its unit and source
reason on phone/tablet/desktop without misleading wraps, clipping or low-contrast text.
Existing Tailwind Plus shared-border stats and the rendered Fathom page are the
references; no licensed source body is copied into either consumer.

## Implemented and verified

The value/caption group now wraps between items instead of squeezing a short unit
onto another line. Long identifiers may break within a word; no truncation, hidden
reason, forced nowrap or smaller type makes the test pass. Four-item bands use
two tablet columns/four large-screen columns; five-item bands use two/three/five.
Tiles also allow the sparkline to move to another row while retaining its size.
Each band cell now owns its bottom/right separators inside a clipped wrapper, so
multi-row and partial-row bands retain continuous borders without doubled edges.
Public Go fields/signatures and app-owned metric semantics remain unchanged.
Consumers must rebuild their L3b CSS after upgrading.

The Gallery `/layout#stat-reflow` has band/tile zero, lower-bound and absent-source
cases plus a delta/sparkline tile. The same browser-side geometry/contrast function
checks this specimen and Fathom's inert export. A separate five-item specimen at
`/layout#stat-dense-reflow` and the existing four-item band cover divider topology,
including three columns at 1024px. All fixture values remain literal.

| Evidence | Result |
|---|---|
| Fathom original, 60 width/theme/state cases | 188 failures: 108 value wraps, 80 value/caption bounds failures |
| First layout candidate, same 5,684 assertions | 16 remaining failures: `~0 tok` split at 768px beside the sidebar |
| Second stat candidate, same assertions | **5,684/5,684 pass**: 540 metric instances and 1,020 label/caption instances; review subsequently caught misplaced dividers outside this oracle |
| Final candidate, strengthened typography/divider suite | **6,467/6,467 pass**: exactly nine metrics per page, 180 bands/380 internal separators; filter-aware contrast and removed-divider canary |
| Existing semantic/evidence journeys, unchanged suite | **5,560/5,560 pass** against the final candidate; no error or unexpected request |
| Gallery initial 456 assertions | 3 light-theme positive-delta contrast failures, all geometry checks pass |
| Gallery contrast repair, same assertions | **456/456 pass** across 320/768/1440px, both themes, seven band/tile/spark metrics |
| Gallery final, strengthened suite | **848/848 pass** across 320/768/1024/1440px and both themes; exact values/labels/captions plus 3/4/5-cell divider topology |
| Go verification | root `go test ./...` and `go vet ./...`; scoped stat/tokens/terminal-theme/Gallery/styleguide race tests pass |
| Generation | default CUE generation replay byte-identical; Gallery CSS rebuilt |
| Full maintainer gate | `task check` passes tests/schema/Node/generation steps but exits nonzero at its final Git-cleanliness check because token source/tests/artifacts are uncommitted; not a full gate pass |

Default light `positive` changes only its OKLCH lightness, 0.55 to 0.51. Hue155,
chroma0.14, dark positive and named themes are unchanged. The Go contrast oracle was
RED before repair; new ratios on surface/raised/positive-soft are5.0701/5.2166/4.5191
in light and7.6828/7.1654/5.2876 in dark. Browser small delta contrast was4.4629:1,
below the unchanged4.5:1 floor; its final check passes. Terminal default light positive
regenerates from `#00884d` to `#007a44`. These are selected roles/surfaces, not a
complete palette or named-theme accessibility certification.
The full gate's generation leaves all three named-theme CSS hashes unchanged.

Chrome152.0.7977.84 / isolated Playwright1.63.0 on macOS arm64, DPR1, 100% zoom,
reduced motion. Range bounds, value/caption separation and computed composited color
are measured; unsupported opacity/backgrounds/filters fail. Planted wrapping, overlap,
low-contrast and missing-divider mutations trip their oracles and restore the original.
The original typography oracle did not prove separator layout: independent review
caught that gap, prompting the added checks rather than relabeling the old pass.
Final phone Gallery, tablet Fathom and 768/1024px partial-row Gallery screenshots were
visually reviewed. Divider probes check borders, clipping, rows and segment counts/bounds;
the selected screenshots verify continuity, not an arbitrary-grid continuity proof.
True200% zoom, touch,
screen readers, arbitrary consumer containers and human task-time acceptance remain open.

## Delivery: rehearsal, isolated commit and pinned consumer

The initial Fathom rehearsal retained `63b3a4f`. Its candidate was compiled using an explicit Go
overlay for **only** `stat.go` and generated CSS, then exported by its existing
synthetic exporter. The real mirror, renderers, palette, adoption manifest and module
pins were unchanged. Those earlier results prove a rehearsal, not propagation.

The standalone CSS investigation found two independent gaps:

1. Fathom runs the whole kit CLI in its own module context, missing indexing-module
   checksums. Building the CLI in its pinned mirror avoids new consumer dependencies.
2. The selected theme was generated by the newer, uncommitted kit emitter. The pinned
   emitter removes runtime font/radius aliases. Separately, implicit Tailwind source
   discovery makes CSS depend on unrelated checkout contents: two isolated copies
   agree with each other but not the full checkout.

An attempted Taskfile/source-glob repair compiled offline in two sibling-free copies,
but did not reproduce selected artifacts. It was **reverted**, not counted as done.
Fathom's original Taskfile, CSS input, token CSS and app CSS hashes were restored.
No checksum dependency additions, manual mirror patches or reduced oracle accepted it.

The owner authorized the extension. Isolated commit
`bcfeb8e94ce4132e1198a69f4cf10dc6764bd353` now contains only the stat repair,
font/radius emitter and regression, generated default/synthwave CSS, and its changelog
(seven files). It preserves the baseline positive0.55 palette; the separate0.51
contrast change remains outside this delivery. Its full `task check`, targeted race
tests and repeated generation pass with a clean worktree. This gate covers the isolated
commit, not the unrelated uncommitted root features. The original checkout/index stay
untouched at63b3a4f. Branch `appkit-stat-font-radius-delivery-20260914` retains the commit;
worktree: `/private/tmp/app-kit-stat-font-radius.O7BuDr/worktree`.

The authorized Fathom refresh now advances the exact mirror from63b3a4f tobcfeb8e.
Before applying the seven-file delta, the existing mirror matched the old commit's
complete archive; afterwards it matched the new archive. Both `task vendor-check`
mirrors pass, including unchanged agentic-kit3db571b. No unrelated mirror edits were
overwritten, no dependency declarations changed, and no consumer commit or deployment
occurred. The isolated kit commit remains distinct from the dirty app-kit root.

The new build script compiles the CLI inside the mirror with `GOWORK=off`, stages the
six Fathom renderer files and seven selected kit packages, and disables implicit
Tailwind scanning. The selected token CSS is6,753 bytes, SHA256
`e5b8a4aa803dd81d37bc18414c3415f486283d8e4a03a67d305b43b6fe82c291`;
app CSS is33,159 bytes, SHA256
`0e0b5e5408c5e2484f0f2adcdba6a7c9491214d3528051ef7b43300dc52acaa1`.
Positive0.55 and font/radius aliases remain selected. No hand-edited generated CSS.

Actual-mirror `GOWORK=off` synthetic exports, with no Go overlay, now pass the unchanged
6,467 metric-layout,5,560 evidence-journey and1,207 navigation browser assertions.
They cover both themes,320/768/1440px, exact absent/partial/zero/positive states,
metric/divider canaries and keyboard evidence/detail/back/navigation tasks.
Tablet-light and narrow-dark impact screenshots were inspected. These local Chrome
fixtures do not exercise live handlers/data or certify full accessibility.

Build review caught an extra-import bypass in the first source validator. The fixed
validator requires the exact audited CSS entry, hashes the complete321-file mirror,
rejects redirected sources and serializes builders. Nine synthetic build/refusal checks
pass, including missing/unreadable sources, extra imports and mid-build mutation.
Each output replacement is atomic, not the pair; `--check` detects interrupted pairs.
No directory lock proves a process is still live after an abnormal termination.

The first independent source-copy run correctly failed because the harness omitted
`dist/agents/skills`; the copy allowlist was corrected, not the build oracle. A second
run rebuilt identical CSS in both copies and produced identical79,960,354-byte CLI
binaries, then failed the broader race suite: its existing observation-coverage helper
matched obsolete stat classes and could select another row. Its exact semantic assertions
remain unchanged after a widget-local DOM lookup repair and adversarial helper tests.
The first passing replay was followed by a full command-package race run, which exposed
the same obsolete-class lookup in two service/series helpers. Those test-only helpers
now bind the exact tile/dimension and reject missing, duplicate and neighboring values;
the original service/series/sparkline assertions remain unchanged. Independent review
found no remaining issue. The full `GOWORK=off` command-package race suite passes
(67.098s); no production metric or renderer code changed in these repairs.

The final expanded source-copy replay passes all 15 commands: both copies regenerate
and compare the actual CSS, resolve both mirrors within their own snapshots, build
`./...`, produce byte-identical CLI binaries, and pass the selected Observatory,
app-kit, service-page and dimension-series race tests (1.865s). The snapshots contain
3,174 regular files and 8 symlinks (31,290,944 source bytes). Its 9,578 checks primarily
prove source identity, not 9,578 functional tests. The CLI is 79,960,354 bytes, SHA256
`0b1778b69bb45a88eedf9aaa3f2085025efc438a6b6542bbd542a2d64f40f552`.
Go 1.26.3 darwin/arm64, cached dependencies, `GOWORK=off`, `GOPROXY=off` and
`-mod=readonly -buildvcs=false` bound this result. No sibling kit checkout is used.

These are Git-visible working-tree snapshots, not a clean Git clone, cold-cache build,
installed host, release or deployment. The stock tracked-only clean-clone gate still
fails because the new mirrored `dashboard/nav.go` is untracked; no staging or gate
waiver hides it. Preserve the earlier failed receipts (`standalone`, `standalone-final`)
and the narrower passing `standalone-dom`; `standalone-complete` owns the final proof.
The unchanged pinned adoption checker passes21/21 checks at catalog2a98eb25 after binding
the new CSS and selected stat/emitter/CUE/stamp/build-script hashes. That is static
agreement, not full delivery or clean-Git proof.
After the consumer handoff document was finalized, the same checker was rerun:
`appkit-propagation.4RVZy1/adoption.final.json` records 21/21 passes, SHA256
`cac28adaac612c3518617435fed357490feb8a7f321a1e258dfa16295200203a`.
The checker binary remains `52185ef494dcc0329f5eea84c52a1cf297038b035e80e35ea4adbac57088cc9d`;
the selected contract digest remains `6920a6289b93bb44bc5966fd57c1f9163ecf1982a6eef54a6a160267f79b8323`.

## Retained evidence

All local reports below are under `sextant/workspace/tmp/` (ignored, not a backup).

| Report | Directory | SHA256 |
|---|---|---|
| Original typography | `appkit-metrics-browser.XTo1lS` | `afc36fbd7ded27b59f53a24de785d709ed96e153ac11ec4b142121c981d91d79` |
| First candidate | `appkit-stat-candidate-browser.YDoXi0` | `d485561f8946921a1e5617ed404c58c4d5733b5f5dc9dd89c632f9eec0023c67` |
| Second candidate typography | `appkit-stat-v2-browser.hAN4L4` | `b00789dbf4d5af1855863544fd76fcba2b8d14719a5ac436a45d1c8958d0ee57` |
| Second candidate journeys | `appkit-stat-v2-journeys.ZlRtEL` | `71c3075047ad9569630710ef2ba16a47b3a5bdbba0bf620a10936a8cabadd759` |
| Gallery contrast RED | `appkit-stat-gallery-browser.hL76JZ` | `94000ebdcb9c452ce09a940f482216372960d77fd004c67b14ff9a48a9b8d957` |
| Gallery contrast repair | `appkit-stat-gallery-final.UePqHq` | `3822a7419b050337e33e7c42c8ecba4c874cf8a59fda379aaba8e6a500795a6e` |
| Final typography/dividers | `appkit-stat-v3-metrics.IwrK0R` | `293807b4e3c90a896645d7dfa4c8a9be4ed404b0ad0ebf6c6254e012d3b3c793` |
| Final journeys | `appkit-stat-v3-journeys.KhwOtc` | `7c6b2aa63d017194cf17c810e7268a8b7673d3603d19e44423612b9c1ae55760` |
| Gallery final typography/dividers | `appkit-stat-gallery-v3.xDmZPA` | `0d8596ccca97365b4451b6ddaf1c1f02e1a7ccb136c1598dbde875eadeba9562` |
| Pinned-mirror typography/dividers | `appkit-propagation.4RVZy1/metrics` | `5fa4ce0ee775b4c69eac3866cab64efaf413b73473706e69a057eaac31955a47` |
| Pinned-mirror journeys | `appkit-propagation.4RVZy1/journeys` | `d1af81b932efdbfc8c0c58c1c06d4ab0c1daaeb6b058f720fd2beda0a6c386b1` |
| Pinned-mirror navigation | `appkit-propagation.4RVZy1/navigation` | `cfe2b4aea3073679bbe5aee8f3caf300084cb63cb25774fea938390361a61f23` |
| Final standalone source replay | `appkit-propagation.4RVZy1/standalone-complete` | `ff08356e5dcf9a83f50bfd51c2e175112bc72600c674feaa88b3215f9b8cbbbe` |

Final Fathom export: `appkit-stat-v3-export.y8bx5x`; overlay, candidate CSS,
final `stat-v3.go.txt` and pre-change artifacts: `appkit-stat-reflow.jYAvt9`. Failed-generator exports remain
separate in `appkit-css-pinned-export.xG98Vv`; isolated generator copies are under
`/tmp/appkit-css-standalone.8obPn2`. Do not use those as selected/passing delivery.
Stat source SHA256: `bf39b9412dce4764bcd1815a5b5ec6aad8963664069dc49bf418c4640d93b0f5`;
candidate Fathom CSS: `ad07423dcf751f65e4dc6d1a169b5a17eec24d5f7cb21285c9e4b204631de425`;
final Gallery CSS: `3e4f3ee10c531d163e5ad2cc78203f33fe8dc3290fbc6787f798cc5aa0f98668`.

Scoped intent structure review: root router77/80 lines, incident log147/150,
Unreleased stays within60 lines,38 indexed ADRs match38 records,177 relative file links resolve and
the Project Intent blocks/selected skill bodies agree. Anchor/content and host invocation
are separate. Claude2.1.271 is newer than the router's verified2.1.270; this remains a
warning, not an updated host-proof claim. Codex0.154.0 matches. MCP current-doc tests pass.

Replay with [consumer metrics](fixtures/fathom-observations/metrics.mjs),
[shared oracle](fixtures/fathom-observations/metric-geometry.mjs) and
[Gallery probe](../../apps/gallery/browser/stat_reflow.mjs); each takes explicit local
inputs and a new empty report directory. G1–G6 remain active. Windows/CA/GGB stay deferred.
The [standalone source harness](fixtures/fathom-navigation/standalone-css.mjs) takes the
Fathom source root and a new report directory under its ignored `workspace/`; it refuses
omitted CSS-selected inputs and never relabels a working-tree snapshot as a clean clone.
