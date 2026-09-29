# Changelog — app-kit

One entry per tag once tagging starts (D13); pre-tag work logged by date.

## [Unreleased]

Release impact: minor (pre-1.0: breaking bumps minor)

### Added

- `dashboard.Shell` takes `PageHeading` and `MobileBrand`. `PageHeading` hands the page's one `h1` to `Content` (for example `chrome.RenderPageHeading`): the top bar writes `Title` as plain text, or nothing when it is empty, instead of a second `h1`. `MobileBrand` repeats `Brand` at the start of the top bar below `md`, where the sidebar that carries it is hidden. Both default off, and the default output is byte-identical (`TestShell_DefaultTopBarIsUnchanged`). Returned for Fathom's console stage 0, which could not adopt `chrome.RenderPageHeading` without two `h1`s (FATHOM-S0-KIT-01).
- `viz` charts drawn at a fixed pixel width (`Sparkline`, `StackBar`, `Bars`, `Donut`, `Funnel`, `MeterRow`, `Gauge`, `Steps`, `Timeline`, `RangeBars`, `Waterfall`, `CostReward`, `Tornado`) take an opt-in `Fluid` option: `width="100%"`, no `height`, the same `viewBox` and `preserveAspectRatio="xMinYMin meet"`, so the chart fills its container at its own aspect ratio. The drawing is unchanged and no class or inline style is emitted (L3a, strict-CSP safe). Unset, output is byte-identical; existing goldens are unchanged. Fluid scales, it does not reflow: text shrinks when the container is narrower than `W` and grows when it is wider. Gallery `/components` adds a fluid sparkline card, and the INC-39 `components_reflow.mjs` caption list now has 19 entries. Returned for Fathom's console wave F0 (FATHOM-F0-KIT-04); Fathom has not adopted it ([evidence](docs/assessments/2026-09-25-fathom-f0-kit-returns.md)).
- Experimental `appkit/ui/palette` (`interaction.command-palette`, L2 + L3b) adds a command palette for Fathom's observatory console: a real search link, an `overlay.RenderDialog` modal opened by Mod+K, debounced server-rendered grouped fragments with stale-request abort, arrow/Enter/Escape keyboard contract and the same results on a no-JavaScript GET page. Only a response that is exactly one fragment for the requested query is inserted (a redirect to a full page shows the error state); the shortcut hint and `aria-keyshortcuts` appear only once the script wires the trigger; call `AppKitPalette.init` after swapping palette or trigger markup. Additive; consumers add `@source` for `appkit/ui/palette/*.go` and `appkit/ui/overlay/*.go` and serve one search route. Gallery `/palette` reproduces every state; evidence is one Chromium build and Fathom has not integrated it ([assessment](docs/assessments/2026-09-25-command-palette.md)).
- Experimental `viz.LayoutGraph` and `viz.LayoutEgo` (catalog `analytics.graph`, L3a) lay out a bounded directed graph or one node's callers/callees deterministically and render `SVG()` plus a class-free `Table()` alternative from one layout. Refused input returns `ErrInvalidGraph` before output; per-column overflow collapses into a stated "+N more" node that keeps each edge style's own line and count. Additive: no existing renderer changes; Gallery `/graph` is linked from `/components`, not the section nav. Fathom selected it (FATHOM-KIT-02) but has not integrated it; screen readers, performance and a second consumer remain open ([evidence](docs/assessments/2026-09-25-layered-graph.md)).
- Experimental `viz.Heatmap` truth states: `HeatmapOptions.States`/`TruthStates` keep a measured zero (ramp fill plus `0`) distinct from missing (dashed, hatched), unknown (dotted, cross-hatched, `?`) and error (danger cross) cells, with per-cell reason titles, a `<desc>` summary, a legend of present states and an opt-in `HeatmapOptions.Validate`. Absent cells never fill or widen the range. Unset, output is byte-identical (legacy goldens frozen before the change); keyed option literals are unaffected. A measured zero with no value span sits at the ramp floor, and truth-state labels, titles and ranges never round a nonzero value to `0`. Gallery `/components/heatmap` pairs the chart with a table alternative. Requested for Fathom's coverage matrix, but no published Fathom revision records the selection and no app consumes it yet; integration, screen-reader and performance acceptance remain open ([evidence](docs/assessments/2026-09-25-heatmap-truth-states.md)).
- Thirteen catalog records for shipped but uncatalogued packages: the viz family (heatmap, sparkline, composition, scores, stages, outcome), provenance frame, agent run inspection, motion, topology, explainer, export toolbar and chrome disclosures/description lists, each with declared tasks, honest maturity (experimental without a Gallery specimen) and consumers limited to apps that call the package (Fathom for the sparkline, scores, provenance frame and agent-trace records). Existing entries and their capability pins are unchanged; the whole-catalog revision changes, so restart searches ([evidence](docs/assessments/2026-09-25-heatmap-truth-states.md)).
- Experimental `ui/code` excerpt viewer (`content.code-excerpt`) and `hitl.RenderUnifiedDiff` with a deterministic Myers `hitl.DiffText` (`work.unified-diff`), requested by Fathom (FATHOM-KIT-04/05) and shown on Gallery `/agentic/code`. Numbered lines carry text-labelled app-supplied states, `#L<n>` permalinks and keyboard-scrollable, non-wrapping viewports; unified hunks use text `+`/`-` markers, spoken line labels and native collapsed context. Characters that draw nothing, reorder text or change how it parses (controls, format and tag characters, U+2028/U+2029, default-ignorable fillers, variation selectors) show as code points in lines, captions and names. Precomputed hunks may be a subset: skipped ranges are stated per side, never as unchanged; `UnifiedDiff.Replaced` keeps the edit-budget note. 24px excerpt rows give WCAG 2.2 2.5.8 permalink targets, below the kit's 44px floor. L3b: scan `appkit/ui/code/*.go` (new) and `appkit/ui/hitl/*.go`, rebuild CSS. No syntax highlighting; Fathom integration, second consumer and full accessibility/performance evidence remain open.
- Experimental `appkit/ui/live` (L2 + L3b, catalog `interaction.live-freshness`): a dependency-free `AppKitLive` script follows an app-owned server-sent event stream and marks matching `data-live-source` regions updated in reserved space with one polite announcement and a refresh link, never re-rendering or moving content. Jittered backoff whose attempt budget a server that accepts and closes cannot reset, `Last-Event-ID` resume (sent as UTF-8 bytes) from a render-time cursor that explicit starts also adopt, hidden-page pause and a manual stop/resume control; `RenderRegion`/`RegionAttrs`/`RenderStatus`/`RenderPageStatus` render the markup, reserving every connection message so nothing moves even at 320 px. Experimental `chrome.RenderPageHeading` (`interaction.page-heading`) renders the page's one `h1` with breadcrumbs (non-web destinations refused), eyebrow, meta and `form.RenderButton` actions that wrap below the title at narrow width; `chrome` now imports `form`, so scan `appkit/ui/form/*.go` too. Gallery `/live` (synthetic SSE fixture) and `/page-heading` specimens; Fathom is the originating consumer, not yet an adopter ([evidence](docs/assessments/2026-09-25-live-freshness-and-page-heading.md)).
- Gallery `state=error` demonstrates a failed analytics source separately from zero, missing observations and valid-empty selection. It withholds values/detail/CSV before building data, returns non-attachment 503 HTML, and offers selection-preserving complete-fixture recovery. No shared analytics API or consumer change ([task](docs/assessments/2026-09-23-analytics-source-error.md)).
- **Breaking rendered-contract correction:** missing nonempty `Select.Value` now shows a disabled empty prompt, never an accidental valid fallback; a configured placeholder alone owns initially empty selection. Apps explicitly choose fallbacks and retain validation/disabled-choice policy. Rebuild, review snapshots and changed `interaction.forms` pins. Scoped browser/reset/submission checks pass with controlled option changes; the failed native-key choice remains open ([ADR-0083](docs/adr/0083-render-missing-select-values-as-unavailable.md), [assessment](docs/assessments/2026-09-23-select-state.md)).
- JSON-only `appkit adoption search` and `capability` expose the shared first-party catalog and guarded exact metadata/pins without MCP, Gallery, consumer reads or network. Review before drafting; success is retrieval, not adoption proof. Existing commands/schemas remain unchanged; rebuild the CLI for these additive commands ([ADR-0082](docs/adr/0082-discover-adoption-capabilities-without-a-host.md)). Roadmap routing now reflects scoped RAC/Fathom working-tree integration rather than stale pending approvals.
- Opt-in isolated MCP host acquisition and offline trace verification retain exact current-source identities, synthetic source, revision refusal/recovery and partial-exit canaries. Claude passes the nine-call flow; both default Codex trials remain incomplete despite exit0. Disabling its code-mode host is a retained failed experiment, not a fallback; explicit host errors also fail acceptance. No server/installed configuration or consumer changes ([host evidence](docs/assessments/2026-09-23-current-mcp-hosts.md), [presentation diagnosis](docs/assessments/2026-09-23-codex-tool-presentation.md)).
- Gallery analytics adds an explicit empty fixture group, no-results explanation and selection-preserving recovery on the comparison/renderer pages. Empty CSV exports keep their schemas and disclose header-only output; current-period context remains available separately. No shared query API or consumer migration ([task](docs/assessments/2026-09-23-analytics-empty-selection.md)).
- Quality-work MCP/Gallery discovery distinguishes matched, filtered and untracked work. MCP list/get add optional `expected_catalog_revision`; both transports return independent `catalog_revision` without changing the inventory hash. Rebuild for these additive fields; empty results and authored readiness are not acceptance or executed proof ([selection](docs/assessments/2026-09-22-quality-selection-guidance.md), [revision contract](docs/assessments/2026-09-23-quality-catalog-revision.md), [lookup](docs/assessments/2026-09-16-quality-lookup.md)).
- Theme-guide lifecycle instrumentation, independent structural joins and memory-v2 controls retain false passes and failed checkpoints while adding fail-closed attachment and metric controls. Instrumentation remains opt-in; retention, attribution and browser-memory/full-contract acceptance remain unverified ([instrument](docs/assessments/2026-09-22-theme-lifecycle-instrument.md), [join](docs/assessments/2026-09-22-theme-lifecycle-join.md), [controls](docs/assessments/2026-09-22-theme-memory-controls.md), [ADR-0073](docs/adr/0073-separate-memory-metrics-from-attribution.md)).
- The bounded Fathom guide integration adds its exact mirror/module graph, synthetic guide, CSS diagnostics and missing-data display repairs. Focused build/browser/export evidence passes; separate light/dark200 chart captures resolve the raster omission. Dark-guide selection also fails on a minimal native control; park input permutations, preserving guide/human/full-quality and consumer-delivery gaps ([integration](docs/assessments/2026-09-22-fathom-guide-only.md), [zoom](docs/assessments/2026-09-22-fathom-large-zoom.md), [light diagnosis](docs/assessments/2026-09-22-native-zoom-paint.md), [dark result](docs/assessments/2026-09-23-fathom-dark-guide-paint.md), [control](docs/assessments/2026-09-23-native-select-guide-resume.md)).
- Static export cold replay now reproduces the exporter, CSS and eleven files from two empty caches with frozen file-only dependencies. The failed preparation preflight remains preserved; this is same-host CGO-disabled proof, not consumer/host/full-quality acceptance ([preparation](docs/assessments/2026-09-22-static-cold-delivery.md), [replay](docs/assessments/2026-09-22-static-cold-replay.md)).
- Analytical bars add explained table-only fallbacks for subpixel positive marks and positive `RowH < 16`, preserving values, CSV and ordinary charts. Rebuild adopters and review changed guide pins; native-scroll and browser evidence are scoped, not full quality promotion ([row-height](docs/assessments/2026-09-22-analytics-row-height.md), [resolution](docs/assessments/2026-09-22-bar-resolution-and-guide-discovery.md)).
- Experimental `analytics.WriteTableCSV` adds a rectangular spreadsheet export beside unchanged report CSV, with a secondary Gallery download; empty selections contain headers only ([ADR-0077](docs/adr/0077-incubate-rectangular-analytics-export.md)). Experimental `analytics.RenderDivergingBars` adds signed bars and [accurate fallback guidance](docs/assessments/2026-09-23-adopter-guidance.md). Optional `Row.DetailHref` adds native table-label links with atomic URL validation and unchanged CSV ([ADR-0075](docs/adr/0075-incubate-app-owned-analytics-detail-links.md)); **breaking for positional Row literals:** migrate to named fields. The independent [missing-reason width repair](docs/assessments/2026-09-23-analytics-state-width.md) changes linked/unlinked missing-row HTML, preserving complete-data output and CSV. Review snapshots and `analytics.comparison` pins, rebuild CSS; no consumer migration or promotion is automatic ([ADR-0074](docs/adr/0074-incubate-signed-analytics-comparisons.md), [ADR-0076](docs/adr/0076-preserve-readable-missing-state-columns.md)).
- Exact MCP capability lookup and Gallery JSON expose the canonical v2 `capability_revision` used by `appkit adoption pins`. Rebuild the server; metadata identity is not source, runtime or adoption proof ([adoption contract](docs/design/adoption-contract.md#explicit-version2-capability-pins)).
- Conjure's bounded dashboard pilot covers truthful failure/empty/stale/recovery states while retaining indicator defects; its owning session remains separate and human A/B is deferred ([evidence](docs/assessments/2026-09-21-conjure-dashboard-validity.md)).
- RAC aggregate CSV and readiness returns have scoped synthetic/native-browser proof without a consumer commit; Fathom export/CSS/race follow-ups pass within frozen inputs. Historical failures and full guide/G3/G4 acceptance remain open ([RAC](docs/assessments/2026-09-22-analytics-adoption-integration.md), [readiness](docs/assessments/2026-09-22-analytics-readiness-return.md), [Fathom export](docs/assessments/2026-09-22-fathom-native-export.md)).
- [Stabilization and demand-driven resumption](docs/design/stabilization-and-resumption.md) replaces the open-ended campaign with six bounded packets and preserved failures. The [JIT rehearsal](docs/assessments/2026-09-17-jit-contribution-rehearsal.md) is synthetic contract/receipt proof, not adoption, second-app propagation, promotion or a release.
- Gallery CSS ETags/revalidation and theme-guide transport controls preserve CSS/preview isolation and bounded cancellation behavior. Candidate cache reuse, memory, browser cancellation and consumer acceptance remain partial or failed ([CSS evidence](docs/assessments/2026-09-17-gallery-css-revalidation.md), [transport](docs/assessments/2026-09-16-theme-guide-transport.md)).
- Analytics comparison, report-first filters and hostile HTML/CSV/source-link checks retain bounded keyboard/detail/return/export evidence. Human results, CUE security and G3 acceptance remain open ([comparison](docs/assessments/2026-09-16-analytics-comparison-pair.md), [selection](docs/assessments/2026-09-22-analytics-selection-disclosure.md), [composition](docs/assessments/2026-09-16-analytics-composition-safety.md)).
- [G2 shell acceptance](docs/assessments/2026-09-16-shell-acceptance-matrix.md) reconciles Gallery and two adopters, including shared stat/layout repairs; rebuild scanned CSS. Performance evidence retains failed attempts, and broader starter/native/release quality remains partial ([performance](docs/assessments/2026-09-16-shell-performance.md)).
- The Gallery static-content starter and deterministic build-only export add native reading/FAQ, truthful states, theme variants and prefix-aware 404 recovery. Rebuild CSS; retained failures and core/accessibility/performance/hosting adoption remain separate ([starter](docs/assessments/2026-09-16-static-content-starter.md), [export](docs/assessments/2026-09-16-static-export.md)).
- Experimental private macOS/Wails work retains topology, media/recovery, exact-pair, assembly, lifecycle and AX evidence. Cmd+N/audio, scrolling, signature, rollback, installation/signing and Windows remain open ([parity evidence](docs/assessments/2026-09-16-native-profile-workflow.md)).
- Private Gallery evidence adds exact-subject capture inspection and typed P07/P08/P09 repair records while preserving originals and failed gates. **Breaking reader capacity:** rebuild private CLI/Gallery/MCP for packets over 32 entries; ADR-0061 raises the bounded limit to 64 without changing old identities ([ADR-0061](docs/adr/0061-bound-larger-historical-capture-sets.md), [retention](docs/assessments/2026-09-15-navbar-evidence-retention.md)).
- Private P01 hero and P09 navbar add typed export/replay and explicit repair variants with retained source/performance failures. Font/cache and runtime evidence remains scoped; no v1 rewrite, account mutation, consumer acceptance or certification is implied ([hero](docs/design/hero-adapter.md), [navbar](docs/design/responsive-navbar-adapter.md), [ADR-0060](docs/adr/0060-deliver-explicit-navbar-runtime-repair.md)).
- Private candidate preparation and exact-server/browser replay reproduce selected outputs without sibling source or network access; current replay migrates stale CSV8 fixtures to CSV10. Bounded private-bundle proof does not complete native/release acceptance or establish installed/signed distribution ([ADR-0045](docs/adr/0045-exact-pins-and-private-candidate-preparation.md), [ADR-0068](docs/adr/0068-prepare-private-unsigned-macos-delivery-first.md), [bundle runtime](docs/assessments/2026-09-16-private-bundle-runtime.md)).
- Opt-in CLI reporting and the Charm terminal reference support bounded G5 acceptance, including the reviewed Bosun/Surveyor unknown-input exit 1→2 migration. Historical lifecycle failures remain; installed, graphical and accessibility certification is not claimed ([matrix](docs/assessments/2026-09-16-terminal-acceptance-matrix.md), [ADR-0070](docs/adr/0070-opt-in-cli-invocation-classification.md)).
- Experimental typed adapters P02–P06 and the separate P10 settings derivative retain immutable delivery/replay and scoped native-browser evidence. They are not certified targets, installed routes or inferred public APIs ([adapter evidence](docs/assessments/2026-09-14-navigation-typed-adapter.md), [P10](docs/assessments/2026-09-15-catalyst-textarea-transport.md)).
- The runnable P11 receipts reference adds signed analytics tasks; private P12 retains the typed React/Tiptap lifecycle and pinned Datastar morph/zoom lanes. No public editor API, save backend, root Node dependency, consumer adoption or target certification follows ([P11](apps/receipts/README.md), [P12](docs/assessments/2026-09-16-client-editor-island.md)).
- App-owned theme review previews immutable CUE and exports acknowledged ZIPs without applying changes; cancellation, lifecycle, memory and performance failures remain retained ([guide](docs/design/app-owned-styleguide.md), [memory](docs/assessments/2026-09-17-theme-guide-memory.md)).
- Named Synthwave, Indigo, Copper, Lagoon and Orchard identities use existing Gallery/Go/vendor paths with deterministic scoped replay. Preset availability and generation do not prove whole-app fidelity or acceptance ([ADR-0029](docs/adr/0029-named-themes-and-app-owned-style-guides.md), [replay](docs/assessments/2026-09-16-paired-themes.md)).
- Private HTML/Elements translation retains legacy diagnostics and adds runtime-pinned `assets-translate-elements` receipts with frozen compiler/DOM/browser replay. Missing imagery, small controls and certification remain open ([guide](docs/design/elements-translation.md), [evidence](docs/assessments/2026-09-14-elements-conversion.md)).
- Gallery Activity adds opt-in same-origin Datastar snapshots with bounded assets/writes, isolation, redaction, reconnect and cancellation. Evidence is scoped and does not establish full browser/native conformance ([observer](docs/assessments/2026-09-13-datastar-observer.md), [stream](docs/assessments/2026-09-13-streaming-proof.md)).
- Synthetic Codex/Claude trials retain exact MCP/consumer contracts and refusal cases; they do not prove real registration, UI quality, release pins or fleet enforcement ([host evidence](docs/assessments/2026-09-13-agent-host-routing.md)).
- Private MatDash and Catalyst references preserve originals, failed audits and hardened derivatives; strict/cold/font evidence remains scoped, with warm/cancellation/reproducibility gaps and no save backend or certified target ([MatDash](docs/assessments/2026-09-14-matdash-strict-build.md), [Catalyst](docs/assessments/2026-09-15-catalyst-font-alias-delivery.md)).
- Opt-in `ui/vizcheck` adds static composition checks with planted-failure canaries; Fathom adds bounded display/read repairs with separate browser evidence. Static checks do not replace browser proof, and neither completes dashboard/G3/G4 acceptance ([ADR-0025](docs/adr/0025-static-chart-composition-evidence.md), [Fathom adoption](docs/assessments/2026-09-16-fathom-display-adoption.md)).
- Read-only adoption `inspect/check/pins/router` validates explicit contracts; v2 pins avoid unrelated catalog invalidation. New `appkit adoption draft` prints explicit v2 choices with metadata/file hashes and optional module/router requirements; review/save/check remain separate, never automatic upgrade approval. Rebuild the CLI; no runtime or host proof is inferred ([ADR-0065](docs/adr/0065-pin-selected-adoption-capabilities.md), [ADR-0078](docs/adr/0078-draft-explicit-adoption-selections.md)).
- Intent/adoption/contribution/JIT guidance and MCP resources share reviewed documents; all root-router design guides now resolve, including Catalyst settings, Fathom maintenance and web/macOS parity. Rebuild/reconnect MCP; documentation availability is not target support or consumer propagation ([router proof](docs/assessments/2026-09-23-mcp-router-guides.md), [ADR-0066](docs/adr/0066-close-consumer-contribution-loops.md), [ADR-0069](docs/adr/0069-prioritize-starter-patterns-and-just-in-time-conversion.md)).
- The shared CUE catalog, bounded MCP search/exact-get and Gallery JSON/HTML share IDs/revisions; Gallery quality review now preserves independent catalog/inventory guards and refuses malformed queries or stale catalog handoffs. Rebuild Gallery; no pins are updated ([handoff](docs/assessments/2026-09-23-gallery-quality-review.md)). **Breaking search correction:** match declared tasks or capability ID/name, not explanatory prose or unrelated combinations; restart cursors and review changed pins ([ADR-0062](docs/adr/0062-search-declared-capability-tasks.md), [evaluation](docs/assessments/2026-09-16-catalog-selection.md)).
- Portable light/dark/system appearance includes failure/retry/events; `appkit vendor --include appearance` supplies it without a Go runtime.
- Analytics derives charts, tables and CSV from one validated dataset with bounded rows, truth states and readiness checks. **Breaking:** CSV appends `role`/`role_label`; migrate shared 8→10, Gallery 31→33 and Receipts 33→35 columns and rebuild CSS; full accessibility, human, performance and G3 acceptance remain open ([ADR-0056](docs/adr/0056-preserve-analytics-role-meaning.md), [journeys](docs/assessments/2026-09-17-analytics-truth-state-journeys.md)).
- Private ZIP inventory, immutable evidence and saved syntax attempts share bounded CLI/MCP/Gallery discovery and exact identity checks; retrieval never executes projects or certifies targets. Originals stay private, failures remain, and translation outputs remain explicitly unverified ([ADR-0022](docs/adr/0022-private-design-references-and-fidelity.md), [ADR-0043](docs/adr/0043-private-historical-evidence-catalog.md), [translation](docs/design/html-translation.md)).
- Experimental account/notice and responsive settings recipes replace Gallery duplication and [wrap long control text](docs/assessments/2026-09-22-settings-control-reflow.md), keeping routing, save, authorization and form authority app-owned. RAC evidence is scoped working-tree adoption, not an authentication service or released consumer pin ([ADR-0063](docs/adr/0063-incubate-account-presentation-recipe.md), [ADR-0064](docs/adr/0064-incubate-responsive-settings-sections.md)).

### Changed

- **Breaking rendered-contract correction:** a `callout.Render` disclosure's `DetailsLabel` affordance (and its chevron) is `text-ink-muted` instead of `text-ink-faint`. It is the only control that reveals `Details`, and ink-faint measured 2.62 to 3.41:1 on the five kinds' soft grounds (danger-soft 3.16:1 light, 2.88:1 dark), below the 4.5:1 text floor; ink-muted measures 4.76 to 6.43:1 (`TestDetailsLabelClearsTheTextFloorOnEveryKind`). Affected: callouts with `Details` (Fathom's blocked-change preview wraps a `hitl.DiffView` in one, labelled "Show both versions"). Migration: rebuild scanned CSS and review snapshots. Returned for Fathom's console wave F0, which filed it against `hitl.DiffView` (FATHOM-F0-KIT-06).
- **Breaking rendered-contract correction:** `chrome.RenderPagination` page links, Prev and Next render the 44px minimum target (`inline-flex min-h-11 min-w-11 items-center justify-center rounded-md px-3`) with the 2px `focus-visible` outline in the focus role, the current page and disabled ends keep that height, and the row is `flex-wrap`. Before, links were `px-2 py-1` or `px-2.5 py-1` (about 28px high) with only the browser's focus ring, and a 20-page window did not fit a 390px screen. Affected: apps calling `chrome.RenderPagination` (Fathom's console wraps every pager locally). Migration: rebuild scanned CSS for `appkit/ui/chrome/*.go` (the Gallery build needed no new utility), review snapshots and changed `interaction.chrome` pins, and drop local pager-target wrappers. Returned for Fathom's console (FATHOM-F0-KIT-05: left open by wave F0's returns, and the pager part of FATHOM-C-KIT-05).
- **Breaking rendered-contract correction:** the `overlay` dialog and drawer backdrop is the new `scrim` token role at 50% (`backdrop:bg-scrim/50`) instead of the palette color `backdrop:bg-black/40`, so no kit component paints a color outside the role vocabulary (`TestComponentsUseRoleColorsOnly` now holds every `appkit/ui` package to that). `scrim` is an optional schema role: `kit.cue` authors `oklch(0.30 0.015 260)` for light and `oklch(0.10 0.010 260)` for dark, and `tokens.Load` gives a set that omits it the kit value for each branch's measured surface polarity, so older app token sets load unchanged and dark-only sets get the darker veil in both branches. In every shipped set and both schemes the scrim at 50% darkens the surface at least as much as black/40 did (default set: 2.91:1 against 2.83:1 light, 1.03:1 both dark). Generated `theme.css`, DTCG JSON, `ColorNames`/`ColorScrim` and the terminal palette (`RoleScrim`, now 24 roles) gain the role; `palette.RenderDialog` inherits the class. Affected: apps rendering `overlay.RenderDialog`, `overlay.RenderDrawer` or `palette.RenderDialog` (Fathom's console). Migration: rerun `appkit gen` for app-owned token sets (nothing to author), rebuild scanned CSS, and review snapshots; until both run the class has no rule and the browser's default backdrop shows. The veil relies on `::backdrop` inheriting custom properties from its dialog, which current engines do; where it does not, the backdrop is transparent. Returned for Fathom's console stage 0 (FATHOM-S0-KIT-02).
- **Breaking rendered-contract correction:** `chrome.Breadcrumbs` links render the 44px minimum target (`inline-flex min-h-11 min-w-11 items-center justify-center rounded-sm`) and a 2px `focus-visible` outline in the focus role. The trail row is now 44px tall; before, links were about 17px high with only the browser's default focus ring. `chrome.RenderPageHeading` inherits the change. Affected: apps calling `chrome.Breadcrumbs` or `chrome.RenderPageHeading` (the catalog records no `interaction.chrome` consumer; Fathom's unpublished console is one). Migration: rebuild scanned CSS for `appkit/ui/chrome/*.go`, review snapshots and changed `interaction.chrome` pins, and drop local crumb-target wrappers. `RenderPagination` links are unchanged at about 28px. Returned for Fathom's console wave F0 (FATHOM-F0-KIT-03) ([evidence](docs/assessments/2026-09-25-fathom-f0-kit-returns.md)).
- **Breaking rendered-contract correction:** `table.Table` header cells carry `scope="col"`, and the scroll wrapper is `relative`, so the sr-only Select/Actions header labels resolve against it. At 390px the Gallery `/layout` specimen no longer widens the page by 54px; Fathom measured an 861px document on its statusz page. The wrapper now also contains absolutely positioned content from custom cells, so an overlay that must escape it needs fixed positioning. The table still has no row-header option. Affected: `data.table` consumers (Throughline in the catalog; Fathom's unpublished console). Migration: rebuild templ and scanned CSS, review snapshots, cell overlays and changed `data.table` pins, and drop local scope/overflow workarounds. Returned for Fathom's console wave F0 (FATHOM-F0-KIT-01/02) ([evidence](docs/assessments/2026-09-25-fathom-f0-kit-returns.md)).
- **Breaking rendered-contract correction:** tables infer one action column from actual row links or a custom slot; `RowHrefs` remains an explicit reservation for empty/unlinked results. Rows without links get empty action cells, not controls. Rebuild Go/templ, review snapshots/selectors, app-authored footer spans and `data.table` metadata pins. No new API, JavaScript, automatic consumer upgrade or table certification ([ADR-0081](docs/adr/0081-derive-one-table-action-column.md), [evidence](docs/assessments/2026-09-23-table-action-column.md)).
- **Breaking:** managed table `Row.Href` now refuses unsafe/non-web destinations as inert Open unavailable labels and adds 44px native link targets/focus. Rebuild templ/scanned CSS, review row density/snapshots and selected `data.table` metadata pins. Custom action slots remain app-owned; form-button output is unchanged by sharing its existing internal URL predicate. No automatic consumer migration or table certification ([ADR-0080](docs/adr/0080-harden-table-row-destinations.md), [evidence](docs/assessments/2026-09-23-table-link-safety.md)).
- Untracked quality-work guidance now names the consumer-owned contract/issue handoff and preserves original discovery context plus both revisions; MCP lookups remain read-only. Rebuild to receive the clarified next action; no metadata pins or gate statuses change ([handoff](docs/assessments/2026-09-23-untracked-quality-handoff.md)).
- Static-reading queries now resolve the fixed `content.docs` example and MCP delivery guide without changing profile/maturity. Rebuild MCP, restart searches and review pins rather than repinning automatically ([evidence](docs/assessments/2026-09-16-catalog-selection.md#static-reading-discovery-follow-up-2026-09-22)).
- Installed MCP refresh delivers revision guards and current quality metadata through the existing executable; registrations are unchanged and sessions must reconnect. This is protocol delivery, not a fresh model-driven adoption trial ([evidence](docs/assessments/2026-09-22-mcp-installed-refresh.md#revision-guard-follow-up-completed-within-scope)).
- MCP `get_capability` accepts optional `expected_revision` and refuses malformed/stale revisions before metadata or v2 pins; omission retains current lookup. Gallery binds detail links and offers the same HTML/JSON guard: stale409 recovery preserves valid filters/theme, explains invalid-filter resets, and refuses malformed/duplicate guards with400. Rebuild affected servers; pins and unguarded access stay unchanged ([contract](docs/design/adoption-contract.md#explicit-version2-capability-pins), [Gallery evidence](docs/assessments/2026-09-23-gallery-catalog-revision.md)).
- Analytics refuses `Window.TimeZone: "Local"` before chart/table/CSV output; select an IANA zone or `UTC`. Boundary regressions preserve macOS snapshot symlink refusal ([evidence](docs/assessments/2026-09-22-adoption-boundary-hardening.md)).
- Gallery navigation/provenance/CSS closure moves owned styles into explicit scans, improves narrow keyboard scrolling and preserves synthetic disclosures. Rebuild CSS; retained same-CSS pixel differences and scoped evidence do not promote quality or migrate consumers ([payload](docs/assessments/2026-09-22-gallery-nav-payload.md), [closure](docs/assessments/2026-09-22-gallery-css-closure.md), [provenance](docs/assessments/2026-09-22-provenance-reflow.md)).
- The optional Jev assessment records alpha transport, fallback and receipt constraints; no provider is installed or invoked and deterministic behavior is unchanged ([assessment](docs/assessments/2026-09-22-jev-adoption-assessment.md)).
- App-kit MCP setup distinguishes Claude JSON from Codex TOML and hardens identity/protocol annotations. **Breaking admission/output tightening:** commercial search/list enforce declared axes/limits; previews default to 32 KiB (64 KiB max) and require paging plus optional `expected_sha256` for complete use ([migration](docs/assessments/2026-09-21-mcp-adoption-setup.md)).
- **Breaking:** form-button unsafe or disabled links become noninteractive labels, never submits; relative and HTTP(S) destinations remain supported. Rebuild source/scanned CSS and review snapshots ([ADR-0079](docs/adr/0079-disable-unsafe-form-button-links.md)). Core rendering rejects malformed analytics text/label overflow, filters missing-only legends and refuses duplicate readiness revisions without changing table/CSV meaning. Radio groups now associate help/errors, expose invalid state and use danger focus styling; native values/selection stay unchanged. Rebuild scanned CSS and review markup snapshots; no inventory gate is promoted ([analytics](docs/assessments/2026-09-22-analytics-adoption-integration.md), [radio feedback](docs/assessments/2026-09-23-radio-feedback.md)).
- Reconciled navigation returns RAC/Fathom artifact-pin/refusal proof while preserving prior failures; the original G4 checklist is proven, but guide/core/JIT and human analytics remain open ([reconciliation](docs/assessments/2026-09-16-propagation-reconciliation.md)).
- Fathom guards/config guidance preserve paid designs and app-selected profiles; semantic serving and runtime adoption remain separate, with no net token-savings claim ([maintenance](docs/design/fathom-maintenance.md), [journal](docs/journals/2026-09-16-fathom-dogfood.md)).
- Browser-first Wails profiles retain explicit native exceptions and target-specific evidence; web/macOS are current, while CA/GGB and Windows remain deferred without waived conformance ([ADR-0033](docs/adr/0033-web-first-workflows-and-native-enhancements.md), [ADR-0027](docs/adr/0027-web-and-macos-now-windows-on-demand.md)).
- Private comparison/browser controls retain original frames, native fallback and recorded failures; repaired smoke checks do not certify conversion fidelity, full tasks or native delivery ([browser evidence](docs/assessments/2026-09-13-gallery-browser-smoke.md), [ADR-0030](docs/adr/0030-isolated-comparison-workspace-controls.md)).
- Gallery loopback now cancels active requests before bounded shutdown and reports its bound address; no public runtime/native-host API is introduced.
- MCP CSS setup uses real flags, explicit source closure and app-local motion vendoring; consumer build/asset proof remains separate ([canaries](docs/assessments/2026-09-14-css-source-closure.md), [rehearsal](docs/assessments/2026-09-13-clean-consumer-build.md)). ADR-0023 permits profile-appropriate build-time templ or justified client ownership without forced migration.
- Agent guidance scopes layers/runtime profiles per consumer; missing optional commercial artifacts no longer block first-party MCP startup, while explicit broken paths still fail. MatDash indexing derives Tailwind major and refuses unknown versions; rebuild old indexes with `appkit data matdash-build`.
- The maintainer gate validates catalog CUE and appearance tests with Node; consumers still need no Node runtime.
- **Breaking:** `get_kit_theming` preserves class input and returns review notes instead of inferred palette/dark-class rewrites. Callers must define contextual mappings and verify rendering ([ADR-0022](docs/adr/0022-private-design-references-and-fidelity.md)).
- **Breaking:** custom `appkit gen -tokens` no longer emits a terminal palette implicitly; CLI consumers must pass `-terminal OUTPUT`. Default generation and CSS-only consumers are unchanged.

## 2026-09-14 (ui) — dashboard navigation accessibility repair

Raises shared navigation items and the mobile Menu disclosure to a 44px minimum
height, adds an explicit Menu focus outline, and uses the ink role for readable
current-page text on the soft accent surface. No theme or runtime contract changes.

## 2026-09-14 (ui) — typed dashboard navigation and skip-target focus

Adds experimental `dashboard.RenderNav`, `NavGroup`, `NavItem` and `MatchPath` for
escaped grouped links, route-boundary matching and one current-page indicator. Unsafe
destinations render disabled, while long labels wrap and links expose visible-focus
utilities. `RenderShell` skip destinations now accept programmatic focus without
entering the normal tab order. Consumers retain routing, permissions and icon review,
must scan the L3b package, and still need composed keyboard/narrow/theme verification.

## 2026-06-17 (ui) — hitl.ToolCall: agent tool-call inspector

Rounds out the human-in-the-loop / "under the hood" vocabulary with
`hitl.ToolCall` — inspect one agent tool invocation: tool name, a status badge
(ok / error / running / pending), optional summary + duration, and the arguments
& result under a native `<details>` (no JS). The "what did the agent actually do"
view. Deterministic, token-themed, escaped. Gallery Agentic page showcases it.
The kit's agentic-HITL set is now ApprovalCard · RunTrace · DiffView · ToolCall.

## 2026-06-17 (ui) — viz.Tornado: sensitivity / "what moves the outcome"

Completes the outcome-viz vocabulary (RangeBars · Waterfall · CostReward ·
**Tornado**). `viz.Tornado` renders one bar per factor spanning its Low→High
outcome values across a central baseline, widest swing on top — the canonical
"which factors move the outcome most" sensitivity chart. Portion above the
baseline is positive-role, below danger-role (role-overridable). Deterministic
(stable sort by swing), golden-tested, escaped, token-themed. Gallery outcomes
showcase includes it.

## 2026-06-17 (ui) — hitl.DiffView: before/after change review

Extends the human-in-the-loop vocabulary with `hitl.DiffView` — before → after
review of a proposed agent change, field by field (current value danger-tinted,
proposed positive-tinted; a missing side reads as an addition or removal). Pairs
with `ApprovalCard` so a person can see exactly what an agent wants to change and
approve or reject it. Deterministic, token-themed, escaped, no JS. Showcased on
the Gallery's Agentic page.

## 2026-06-17 (ui) — chrome: Disclosure + DescriptionList

Two more structural creature comforts in `appkit/ui/chrome`:

- **`Disclosure`** — collapsible "show more" section on the native
  `<details>`/`<summary>` (zero JS; CSS-only chevron). Replaces hand-rolled
  expand/collapse panels.
- **`DescriptionList`** — aligned term/detail metadata as a `<dl>` two-column grid.

Both deterministic, token-themed, escaped. Showcased on the Gallery's "Agentic &
chrome" page.

## 2026-06-17 (ui) — overlays: dialog, drawer, toast (loopback-safe JS)

New `appkit/ui/overlay` package: modal `Dialog`, slide-over `Drawer`, and `Toast`
notifications on the native `<dialog>` element + a dependency-free `overlay.js`
(served via `Script`/`Handler` like `router.js`; no Node, no framework — D5/ADR-0002).

- **`Dialog`** — centered modal (sm/md/lg), optional backdrop light-dismiss; native
  Escape-to-close for free.
- **`Drawer`** — full-height slide-over anchored left/right.
- **`Toast`** / **`RenderToastRegion`** — server-rendered after a POST-redirect
  (the flash pattern, no JS) *and* spawned client-side via `AppKitOverlay.toast()`;
  identical role classes (info/success/caution/danger) on both paths.
- **`TriggerButton`** / **`OpenAttr`** — open any overlay by id (`data-overlay-open`).
- Gallery: new "Overlays" page with working dialog/drawer/toast demos.

## 2026-06-17 (ui) — forms: the input vocabulary every app needs

The kit had rich data *display* but no form/input vocabulary — every consuming
app hand-rolled `<input>`/`<label>`/`<select>` markup. New `appkit/ui/form`
package (HTML-string, like `chrome`): token-themed, accessible, escaped,
deterministic, no Node.

- **`Field`** — labeled wrapper with help/error text; owns the ARIA plumbing
  (`<label for>`, `aria-describedby` → help/error ids via `HelpID`/`ErrorID`,
  error takes visual priority over help).
- **`Input`** (any text-like type), **`Textarea`**, **`Select`** (with optional
  placeholder + selected option), **`Checkbox`**, **`RadioGroup`** (fieldset +
  legend, grouped requiredness). Each carries `required`/`aria-required`,
  `disabled`, and `aria-invalid` + danger styling when `Invalid`.
- **`Button`** — `primary` / `ghost` / `danger` variants; renders `<button>` or,
  with `Href`, an `<a>` styled as a button.
- Gallery: new "Forms" page showcasing the full vocabulary live.

## 2026-06-16 (ui) — kit unification: composable layout packages + one canonical Card

Unified the two parallel UI lines onto `main`, keeping all value from both: the
outcome-viz / `hitl` / `chrome` suites already here, plus the composable layout
packages that had been developed on a separate branch.

- **`appkit/ui/card`** (new) — the single canonical bordered-surface primitive:
  `Header`/`Body`/`Footer` slots as `templ.Component`. `appkit/ui/chrome` no
  longer defines its own `Card` (the duplicate `chrome.RenderCard` was unused and
  is removed) — one Card concept across the kit.
- **`appkit/ui/kanban`** (new) — server-rendered Kanban board composed from
  `card.Card`; `Column`s + `CardItem`s (StageID → Column.ID), emits `data-*`
  hooks for the consuming app's hypermedia layer. Presentational only.
- **`appkit/ui/table`** (new) — generic token-driven data table: typed `Column`s
  with optional sortable header links, length-tolerant `Row`s.
- **`appkit/ui/router`** (new) — embedded dependency-free History-API client
  router (`router.js`) for static-client surfaces; no build step, no Node
  (ADR-0002). Served via `Handler` or inlined via `Script`.
- Full gate green (`task check`): vet, all package tests, cue vet, byte-identical
  token regen.

## 2026-06-16 (ui) — outcome visualization + human-in-the-loop

Outcome-focused viz (visualize likely outcomes and rewards/costs) and the first
agentic human-in-the-loop UI. All pure-Go, token-themed, deterministic
golden-tested, accessible.

- **`viz.RangeBars`** — labeled low–high interval bars with an optional
  most-likely point marker (recovery low/high per theory); `$`/`%` units via
  Prefix/Suffix.
- **`viz.Waterfall`** — running-total column chart with absolute Total
  checkpoints (combined recovery → net): up=positive, down=danger, total=accent.
- **`viz.CostReward`** — diverging reward-vs-cost bars from a center axis
  (recovery vs filing fees / time).
- **`appkit/ui/hitl`** (new package) — human-in-the-loop UI for agentic apps:
  `ApprovalCard` (a proposed action awaiting a decision — intent + risk badges,
  confidence rating, rich summary, caller-supplied loopback action HTML) and
  `RunTrace` (a vertical done/active/failed/pending event rail — the
  "what's happening under the hood" view).
- **`appkit/ui/chrome`** (new package) — structural creature-comfort UI:
  `EmptyState`, `Breadcrumbs`, windowed `Pagination` (prev/next + page links),
  `Tabs` (link-based view switcher), and `Skeleton`/`SkeletonCard` loading
  placeholders. All server-rendered, no JS — loopback-friendly. (The bordered
  `Card` surface lives in its own `appkit/ui/card` package — see the unification
  entry above.)
- Gallery: outcomes showcase section (RangeBars / Waterfall / CostReward).

  Outcome viz respects Throughline's no-prediction ethos: it renders statutory
  recovery ranges and the user's own weighting, never invented probabilities.

## 2026-06-16 (ui) — at-a-glance suite II: Rating, Bars, Steps, Stat tiles

More at-a-glance vocabulary for Throughline-class dashboards, generally useful.

- **`viz.Rating`** — a row of pips (`filled` of Max) for a compact strength /
  quality indicator (evidence strength, confidence). Graded mode colors by the
  fraction.
- **`viz.Bars`** — a horizontal labeled bar chart for comparisons, probabilities,
  or an expected-outcome distribution (per-bar role + value labels, proportional
  to Max or auto).
- **`viz.Steps`** — a case/workflow lifecycle stepper (filed → discovery → trial
  → resolved) with done / current / upcoming states; completed connectors take
  the done color.
- **`appkit/ui/stat`** (new package) — KPI tiles: a big value + label + optional
  colored delta + optional trend sparkline (composes `viz.Sparkline`).
- **Gallery** Messaging page: a "Key figures" KPI row, and the case-summary
  composite now shows an evidence-strength Rating, the case-status Steps, and a
  labeled outcome Bars chart. `input.css` `@source`s the stat package too.

## 2026-06-16 (ui) — at-a-glance suite: Gauge, callouts, timeline density + filter

Toward Throughline's at-a-glance experience (case status / strength / expected
outcomes) and trustworthy messaging — built generally useful.

- **`viz.Gauge`** — 180° semicircular score dial for case strength, likelihood,
  or confidence. `Graded` mode auto-colors by the value fraction
  (`GradedRole`: danger / caution / positive); center percent + caption; value
  clamped; deterministic golden-tested.
- **`viz.Timeline` density view** (`ViewDensity`) — bins the range into
  equal-time cells shaded by event count (a heatmap); one row per lane when
  laned. `DensityBuckets` overrides the cell count. Brings the Timeline to five
  views (proportional / sequence / condensed / swimlanes / density).
- **`viz.EventFilter`** — the kit's selection toolkit so apps drive a timeline
  from UI controls: a date window (`From`/`To`, overlap for spans), `Lanes` /
  `Roles` facets, and a case-insensitive `Query` (search). `MatchEvent` /
  `MatchSpan` / `FilterEvents` / `FilterSpans` / `Apply`. Rendering stays pure —
  filter, then render.
- **`appkit/ui/callout`** (new package) — messaging as deterministic
  server-rendered HTML themed by status role: `Callout`
  (info / success / caution / danger / note, with ARIA note/status/alert and
  inline icons), `Disclaimer` (the "information only" caveat a regulated app
  shows), and `Badge` (inline status pill). Pure Go, escaped.
- **Gallery** — new **Messaging** page with an at-a-glance case-summary
  composite (Gauge + outcome StackBar + evidence MeterRow + Badges +
  Disclaimer), the five callout kinds, and a status-badge row; two Gauge cards
  and the density/filtered timelines on Components. `input.css` now `@source`s
  the callout package — the multi-package class-scanning pattern a consuming app
  uses for each kit component package it renders.

## 2026-06-16 (viz) — Timeline primitive (proportional / sequence / condensed / swimlanes)

- **`appkit/ui/viz.Timeline`** — a reusable event-over-time component in the viz
  family (pure-Go SVG, token-themed, deterministic golden-tested, accessible).
  Events (dots), Spans (auto-packed lanes), and Markers (vertical reference
  lines — "today", deadlines) as overlays that work in every view. Three views
  via a piecewise time scale:
  - `proportional` (default) — true elapsed-time spacing;
  - `sequence` — distinct instants spaced evenly (reading order);
  - `condensed` — proportional, but empty stretches longer than
    `CondenseFraction` (15% default) of the span collapse to a fixed-width break
    **labeled with the omitted duration** ("3 mo") plus a "from – to omitted"
    title and a `//` axis glyph. Honest condensation — compressed, never hidden;
    span-covered and event-bearing stretches are never elided, and markers
    extend the range without fragmenting it.
  - **swimlanes** — set `Event.Lane`/`Span.Lane` and the timeline groups into
    one labeled row per distinct lane (sharing the time axis; markers span all
    rows). Lane-free data stays single-axis (byte-identical). Composes with all
    of the above. For Throughline: group by declarant, evidence type, or source.
  Shown under all four views in the Gallery's components page. Next view
  candidates (Throughline-driven): density/heatmap, focus+context. Filtering is
  a caller concern (the renderer draws the set it's given); click-to-expand-gap
  is a loopback-friendly interaction for a later Gallery prototype.

## 2026-06-16 (gallery) — the living style guide v0 + adoption correctness

Owner-directed: a living style guide, and making the kit + TailwindPlus easy and
*correct* to consume. All cgo-free, no Node, `task check` green.

- **The Gallery (D12, VISION #2)** — `apps/gallery`: a kit app, built on the kit,
  that renders every design token (color roles, radius scale, font specimens) as
  live utilities and every `appkit/ui/viz` primitive, with server-side
  light/dark/OS switching via `data-theme` — flip it and roles re-bind through
  `--ak-color-*` with no rebuild. One `http.Handler` (D5); `cmd/gallery` is a
  loopback-only shell. CSS via the pinned Tailwind binary; `input.css` uses
  `@source inline(...)` to force-generate the token vocabulary (utilities are
  built by name at render time). Pages, vocabulary coverage, viz rendering, the
  theme contract, and the safelist's drift are all tested. v0 scope is tokens +
  viz; knob-gen, service exercisers, and the Playwright baseline come with the
  Phase 1 spine (Gate G3).
- **`get_kit_css_setup` (Assistant)** — closes the gap `get_kit_theming` opened:
  the CSS-layer + build contract a consuming project needs for kit role
  utilities to actually render — the no-Node build (`appkit gen` → `theme.css`,
  `appkit css`), the `input.css` content, the `@source` scanning requirement
  (the #1 inert-utility cause), the `data-theme` contract, and the full utility
  vocabulary. Reads `tokens.ColorNames/RadiusNames/FontNames`, so it can't drift
  from what `appkit gen` emits.
- **Token vocabulary** — the generator now exports `RadiusNames` and `FontNames`
  alongside `ColorNames` (single source via `radiusValues`/`fontValues`, also
  feeding `EmitThemeCSS`; `theme.css` byte-identical). The theming guide now
  points at `get_kit_css_setup`.

## 2026-06-15/16 (twplus) — component data pipeline + the Assistant (D10/D11)

Owner-directed ahead of Phase 1 sequence, to get value from the licensed
TailwindPlus dump now. All cgo-free (no Node, no native libs, no daemon);
`task check` green; adversarially reviewed (one D14 fix applied).

- **Pipeline `internal/twplus/`** — `appkit data build` compiles the 336MB dump
  into one SQLite corpus (~37MB): streaming parse (drops 258MB of inline
  previews) → per-component CUE validation (`schema/tailwindplus.cue`, D7) →
  normalize → structural columns + contentless FTS5 BM25 + zstd
  content-addressed code blobs. 657 components / 10,452 variants / 9,765 blobs,
  ~15s. Schema corrected from the real dump (component-pipeline.md §9): the
  snippet `name` field is the framework (`html`|`vue`|`react`), `version` is
  the Tailwind major, Ecommerce `mode` is null→`none`.
- **Assistant `cmd/appkit-mcp`** — MCP server (official go-sdk v1.6.1; stdio
  default + loopback-only HTTP with origin protection, D14). Tools:
  `search_components`, `get_component`, `get_component_preview`,
  `list_components`, `list_sections`, plus the kit-guidance family
  (`get_kit_guidance`, `get_kit_theming`, `list_kit_decisions`,
  `get_kit_decision`) and ADRs as `kit://adr/<id>` resources (D11/D15). Adopt
  via `.mcp.json` from any project.
- **Theming bridge** — `get_kit_theming` adapts a TailwindPlus component to the
  kit's semantic tokens: palette utilities → role utilities
  (`bg-indigo-600`→`bg-accent`, `text-gray-900`→`text-ink`), `dark:` color
  variants dropped (the role token auto-themes). Deterministic + tested; emitted
  roles are validated against the kit's generated `tokens.ColorNames` so a role
  rename can't silently produce a dead class. `get_component` stays byte-faithful
  — the agent applies the guidance.
- **Previews** — `appkit data previews` → `previews.sqlite` (~12MB; 10,452
  previews dedupe to 1,743 blobs) backs `get_component_preview`.
- **Quality gate** — golden known-item suite
  (`data/tailwindplus/golden-queries.json`, `appkit data eval`,
  `task twplus:eval`): **15/15 in top-3**.
- **Integrity + fidelity proof** — `appkit data verify` / `task twplus:verify`:
  every blob decompresses and hashes back to its content address, FTS rows ==
  variants, no dangling refs, manifest counts match, and (when the licensed dump
  is present) all 10,452 variants are confirmed byte-identical to the source.
  Exits non-zero on any mismatch — turns the parity doc's byte-identical claim
  into a checked invariant.
- **Legacy parity** — `docs/design/tailwindplus-parity.md`: `get_component`
  returns byte-identical code, search superseded (ranked vs unranked);
  retirement owner-gated.
- **Adoption docs** — root README (Assistant headline + quickstart) +
  `docs/getting-started.md` + `cmd/appkit-mcp/README.md`. `CLAUDE.md` updated
  (current truth + the new sextant shared-daemon model).
- **Semantic search** — built behind a pluggable embedder (`internal/twplus/embed`
  Ollama client + `internal/twplus/index` vec1 store, RRF fusion,
  `SearchHybrid`); `appkit data embed` / `task twplus:embed` writes a separate
  gitignored `embeddings.sqlite`, and `search_components` (MCP) and `data search
  -semantic` (CLI) fuse BM25 + vector when it's present. Strictly additive
  (ADR-0010): default stays BM25, every query-time embed failure falls back.
  Tested end-to-end with a deterministic fake embedder; **generating real
  vectors needs a local Ollama** (`qwen3-embedding-0.6b`). Golden suite still
  15/15. Cgo-free KNN via ncruces `ext/vec1` (`spikes/twplus-vec-check`).
- New deps: `ncruces/go-sqlite3` (SQLite over wazero), `klauspost/compress`,
  `modelcontextprotocol/go-sdk`.

## 2026-06-10 (viz) — ui/viz dashboard primitives

- **`appkit/ui/viz` package shipped** — 5 pure-Go server-rendered SVG
  components for dashboard and investigation screens:
  - `Sparkline` — inline trend line; NaN-filtered; area-fill option; handles
    empty/single/all-equal inputs without panicking.
  - `StackBar` — horizontal stacked bar (coverage: scanned/pending/failed);
    zero-total renders empty-state track using the "line" role.
  - `Donut` — distribution ring (evidence by tier); optional center text;
    1px gap between segments; empty-state circle.
  - `Funnel` — vertical funnel (proposed→imported/excluded); widths
    proportional to first stage; zero-safe; inline count labels.
  - `MeterRow` — labeled confidence/coverage meter; optional percentage
    label; value/max clamping.
- **Theming:** all colors resolve to `var(--ak-color-<role>)` — components
  re-theme with light/dark automatically. Role strings validated
  `[a-z0-9-]+`; invalid roles fall back to `ink-muted` (no injection).
- **Accessibility:** every component emits `role="img"` + `aria-label` +
  `<title>`.
- **Determinism:** same inputs → byte-identical output; golden-file tests in
  `testdata/*.svg.golden` enforce this; `UPDATE_GOLDEN=1 go test` regenerates.
- Shared SVG helpers (`svg.go`): `cssColor`, `esc`, `fmtNum`, `svgOpen`,
  `svgTitle`; `fmtNum` trims trailing zeros at max 2 decimals.
- First consumer planned: Throughline doc-35 phase 5 dashboards; second
  expected: alaska_bidding.

## 2026-06-10 (later) — first consumer wired

- Throughline's UI build chain now rides the kit (its doc 34): tokens.cue →
  `appkit gen` → theme.css/Go constants; CSS via `appkit css`; Node removed
  from its toolchain. Display contract (data-theme + --ak-font-scale)
  consumed by a real Appearance settings surface, browser-verified.
- `EmitGoNames` output gofmt-formatted (caught by Throughline's fmt gate).
- Kit backlog from the wiring session recorded in docs/NEXT-SESSION.md
  (settings pattern, brand-remap emitter, CLI distribution, templ pinning).

## 2026-06-10 — repo born + the CSS/token chain

- Imported the staging corpus: VISION, ROADMAP, decision register (D1–D15),
  15 ADRs, design docs. TailwindPlus dump placed (gitignored, licensed).
- **CSS/token chain shipped** (Spike 0.3 scope, graduated into kit packages;
  verdict in `spikes/0.3-tokens-css/README.md`):
  - `internal/tailwind` — Tailwind v4.3.0 standalone pinned by SHA-256 per
    platform in an embedded CUE manifest; fetch/verify/run; checksum
    mismatch is a hard failure; end-to-end conformance test.
  - `appkit/ui/tokens` — #Tokens CUE schema (semantic color roles ×
    light/dark, fonts, radius), cuecontext-validated Load, deterministic
    emitters: Tailwind `@theme inline` CSS with runtime `data-theme`
    switching + OS fallback + `--ak-font-scale` display hook; DTCG JSON;
    Go role-name constants.
  - `cue/tokens/kit.cue` — the kit default theme.
  - `cmd/appkit` — `css` (pinned-binary passthrough) and `gen`
    (token regeneration, flag-pointable so apps reuse it).
  - Taskfile vocabulary established: `task gen` / `task css:build` /
    `task check` (vet + tests + cue vet + regeneration cleanliness).
- Layout deltas vs repo-layout.md recorded in CLAUDE.md (embedded manifest
  and schema locations).
