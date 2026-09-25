# Source-reviewed catalog task selection — 2026-09-16

G1 retrieval improvement, not completion of G1 or conversion/consumer certification.
[ADR-0062](../adr/0062-search-declared-capability-tasks.md) owns the search contract and migration.

## Result

The [shared corpus](../../internal/catalogtest/testdata/selection.json) contains65 cases:
47 positive tasks and18 unavailable/incompatible refusals, covering all34 catalog IDs.
The exact same final corpus fails11 cases against the saved old engine/metadata and
passes all65 against the repair. Eight supported phrases were missed; three queries
wrongly recommended unavailable behavior:

- Missing: dark mode, help page, command configuration, key performance indicator,
  horizontal bar graph, activity history, task board and sortable records.
- False recommendations: account backend → shell composition; automatic source patch
  → local theme review; email delivery → the private email-field source adapter.

Search now requires a complete match in the ID/name or one declared positive task.
It does not combine unrelated tasks or index explanatory/negative prose. Reviewed CUE
phrases retain useful related facets, including mobile/current-route navigation and
theme/storage recovery. IDs, response schemas, implementation anchors, runtime profiles
and maturity are unchanged; task metadata changes the catalog revision.
Version1 cursors are refused with restart guidance, including unchanged custom catalogs.

## Verification and limits

| Surface | Evidence |
|---|---|
| Shared catalog | `TestTaskSelectionEvaluation`:65 source-reviewed cases, every record covered |
| MCP protocol | `TestCatalogTaskCorpusAcrossMCPAndGallery`: actual in-memory MCP calls, matching structured/text responses |
| Gallery JSON and HTML | The same corpus checks `/api/catalog/search` parity and parsed `/catalog` result-link order on every page |
| Boundaries | Unsupported/backend/renderer/layer cases stay empty; page size, response shape/bytes, revision, duplicates and forbidden IDs checked through final continuation |
| Direct regressions | `TestSearchRequiresCoherentPositiveTask` preserves normalization/prefix matches while refusing prose/task conflation; `TestSearchRejectsPriorMatcherCursor` refuses an otherwise valid old cursor |
| Existing behavior | Catalog, MCP, Gallery and terminal package suites pass with the race detector; existing adapter/analytics inventory expectations retained, not relaxed |

Run from the kit root with the existing pinned dependencies:

```sh
go test -race ./appkit/catalog ./cmd/appkit-mcp ./apps/gallery ./apps/terminal -count=1
```

Observed environment: Go1.26.3, Darwin/arm64, existing warm module/build caches;
`GOWORK=off GOPROXY=off GOSUMDB=off GOENV=off GOTOOLCHAIN=local`.
This is library/protocol/HTTP-render evidence, not browser input, visual quality,
installed agent-host refresh, consumer adoption, clean-build or performance proof.
The existing running previews and consumer pins were not replaced.

## Oracle review and retained replay

The initial60-case draft had12 failures, but independent source review rejected some
expectations: `viz.Bars` is horizontal, not a vertical column chart; a native record
table is not an editable grid; bare navigation does not imply primary-over-local
ordering. Terminal configuration was narrowed to application command configuration.
These were test-specification corrections, not capabilities implemented by the repair.
Column charts and editable grids now have refusal cases; actual horizontal bars,
sortable records and explicit shell navigation have supported cases. Empty-query
pagination checks the existing stable-ID order. A second review added two related-facet
queries to catch over-narrow phrase matching. No old expected ranking was manufactured.

Private retained directory: `/Volumes/m2ext/appkit-catalog-selection.a59RZaZv/`.
It includes initial60, corrected63 and final65 case corpora; old engine/metadata;
the schema, test runner and final source snapshots; each phase's logs; and the Go
overlay used for same-final-corpus old-source replay without rolling back the worktree.

| Artifact | SHA-256 |
|---|---|
| `selection-final.json` | `409a5426e71f90c29809a55d85c9b032e5621cb7e644b998b9dabe21f734b72b` |
| `final-before.log` (11 failed /54 passed) | `eeb6df4f9c36590e8ffc6b3a3d935dfffea1a401c5032bdf8c137b895eb80176` |
| `final-selection.log` (catalog and MCP/Gallery pass) | `e7193c99df764ef69866926b67c51cc5f04369143120fd2505dae85859967d78` |

The full failure history is retained, including intermediate recall regressions.
This bounded authored corpus does not prove arbitrary-language understanding or
universal recommendation quality. New real-task misses must extend both source-reviewed
vocabulary and the shared evaluation, without broadening a renderer's actual capability.

## Client-editor guidance follow-up

This follow-up extended the corpus to68 cases: the original65 plus three refusal cases for a shipped
rich-text editor in `go-web`, `react-web` and `static-web`. The34 catalog records and
their revision are unchanged by this follow-up. P12 remains a private rehearsal,
not a public widget; the exported integration-anchor gate was not weakened to list it.

The rebuilt MCP server exposes `get_kit_decision("client-editor-island")` and
`kit://client-editor-island`, serving the exact [ownership guide](../design/client-editor-island.md).
That guide now routes to private fixture prerequisites/build/browser commands.
Existing protocol tests compare current file bytes with tool text/structured output
and listed/read resources. `TestClientEditorIslandThroughMCPWithoutLicensedCorpus`
separately supplies only a synthetic guidance document in a temporary kit root;
the document resolves without any commercial corpus. A missing root still refuses it.

The command above reruns this shared corpus across library/MCP/Gallery JSON/HTML alongside
existing package tests. This adds no tool, schema, dependency or paid source body.
Rebuild/restart the MCP server to adopt its new route; no installed-server refresh,
real agent-host invocation, new catalog capability or editor certification is claimed.

## Settings-section discovery follow-up

[ADR-0064](../adr/0064-incubate-responsive-settings-sections.md) adds the experimental
`shell.settings-sections` recipe with exported renderer anchors, app-owned form authority
and no consumer claim. The catalog now has35 records and the shared corpus70 cases:
all preceding68 remain, plus a positive Go-web layout task and a React-profile refusal.
This is a new presentation capability, not broader account-backend or editor availability.
The [section evidence](2026-09-16-settings-section-recipe.md) retains passing race and
cross-surface selection logs plus the actual Gallery response; no installed MCP restart.

## Exact analytics discovery follow-up

The `analytics.comparison` preview now opens `/analytics/recipe`, the actual public
`analytics.RenderBars` specimen, instead of the separate app-composed `/analytics`
journey. The latter remains linked; no renderer, profile, API or maturity changed.
`TestCatalogAnalyticsPreviewUsesAdvertisedRendererAndCSV` first failed for OS/light/dark
against the previous metadata. It now follows the actual catalog anchor, checks the
exact renderer once, follows its direct `analytics.WriteCSV` download, and verifies
theme preservation without leaking catalog filters. Existing exact-renderer selection
checks remain separate from this discovery regression. HTTP200 alone did not prove fit.

This changes the catalog revision and the selected `analytics.comparison` metadata pin.
Review that record when selected; v1 whole-catalog pins also require review. Rebuild
Gallery/MCP binaries to load embedded metadata; no running service or consumer pin was
changed. This is handler/protocol evidence, not a new visual or host-adoption claim.

Verification: full Gallery, MCP and catalog suites pass with `-race -count=1`; pinned
CUE catalog validation passes. The first broad run exposed the inventory test's old
preview expectation; that exact path assertion was updated, not removed. Read-only
current-checker v2 checks pass for RAC and Fathom, which select other capabilities;
no consumer file was written. Evidence: `/Volumes/m2ext/appkit-analytics-discovery.7PsYdRNq/`
retains old inputs, the three-theme failure excerpt, original broad-run failure and
final passing log plus both static consumer reports. Contribution-review follow-up
also retains five missing-guidance assertions before the repair; root guidance now
matches canonical discovery/pre-workaround triggers and separates all three delivery
states. Targeted structural checks pass222 local links, document/Unreleased budgets
and paired intent/contribution-routing sections; not a full host/configuration audit.
