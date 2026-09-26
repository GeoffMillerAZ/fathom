# Layered graph renderer (FATHOM-KIT-02)

## Contract

Originating consumer: Fathom's observatory console (vision 35, FATHOM-KIT-02). Its code
neighbourhood shows a symbol's callers and callees to depth 2 as a layered graph: solid
resolved edges, dashed heuristic edges, highlighted test-reaching paths, and a bounded
node count that aggregates the remainder and says so. Blast radius reuses the renderer.
Source identity: Fathom `docs/design/observatory-console.md` sha256
`b9630e3d2b6ec4667362cfd23e8911025093d2af3af923fce66f2b351e676272` at its `f8b40609`.

Kit boundary: L3a (I4/I5). One validated `viz.GraphLayout` renders `SVG()` and a
class-free `Table()` so picture and alternative cannot disagree (I11). Invalid input
fails before output (converted-core-v1 `typed-config`). No client script, layout
engine, pan/zoom or node traversal was added; ADR-0069 bounds the work to this task.

## Implementation

- `LayoutGraph`: longest-path ranks (DFS cycle breaking in input order; closing edges
  drawn right to left and counted) or `RankMode: given`; four barycenter sweeps per
  layout, ties by previous position then input order; forward-edge ports spread along
  node sides; horizontal arrowheads only, so output has no trigonometry.
- `LayoutEgo`: BFS both ways to `Depth` (default 2); the nearer side wins, callees on a
  tie; omitted nodes counted; an empty side is an explicit `None` column.
- Bounds: 2000 nodes, 8000 edges, 64 columns refused before layout; `MaxPerRank`
  (default 8) keeps focus, then emphasized, then degree, then input order, and pins
  one `+N more` node last. Collapsed labels appear in the table's Represents column.
- Accessibility: `role="img"`, `<title>`, `<desc>` (summary, empty columns, every note)
  over one `aria-hidden` drawing; SVG node links are `tabindex="-1"` pointer shortcuts.
  The table has captions, `scope` headers and ordinary links.

## Package evidence

`go test -race ./appkit/ui/viz ./appkit/catalog ./apps/gallery` passes. Goldens:
`graph_{layered,ego,bounded,empty,arcs}.svg.golden` and three `*_table.html.golden`.
Tests cover ranks, crossing removal, stable ties, cycles, self-loops, given/empty
columns, bounding notes, hidden internal edges, per-style aggregate lines, unique
table names for repeated labels, ego sides/depth/tie, 25 layout and 6 ego refusals,
hostile-text escaping, well-formed standalone XML, viewBox-contained geometry, legend
words, dashed plus width (not color-only) cues and determinism.
`TestGraph_RolePairsMeetContrastFloors` measures the default kit tokens: all text
pairs clear 4.5:1 and edge, focus-border and aggregate-border marks 3:1 against both
adjacent fills in light and dark. Normal node borders (`line-strong`, about 1.8:1)
are decorative; labels identify those nodes. The existing
`TestVizEmitsNoClassAttribute`/`NoLiteralColor`/`SpecimensCoverEveryRenderer` now
include `LayoutGraph` and `LayoutEgo`; the table has its own no-class/style/color test.

## Browser evidence

Gallery `/graph` (five synthetic specimens; not linked from the section nav because
every nav entry is paid for by the signed-analytics 20 KiB page budget, which a nav
entry broke). Harness bytes: `fixtures/layered-graph/browser-check.js`, sha256
`779608cbb3748fab09f7c170c6f4760606384fcfffca418293d7aab1c1ad97d6`, run through the
Playwright MCP `browser_run_code` tool in Chromium 153.0.8010.53 against the Gallery
binary sha256 `8696270b03c51b43796e444f16c1aa5f0771215f0bdf2568d02ce25cb6e5dbca`
(go1.26.3 darwin/arm64) on 127.0.0.1:8772, built after the review fixes below.
Inputs: `graph.go` `1c28ea96…aab1b`, Gallery `graph.go` `e85ede51…cd300`, `app.css`
`82189b11…5099` (pinned Tailwind rebuild adds only the three `[&_a]` link rules).

| Check (1280×900 and 390×844, light and dark) | Expected | Observed |
|---|---|---|
| Page status and horizontal page overflow | 200, none | 200, none in all four cells |
| 16 graph-page tab stops | no SVG descendant focused; 2px outline | 0 SVG stops; all outlined |
| Target size, all tables open | every table link, SVG node link and summary ≥ 44px high | 2 table links, 2 SVG node links, 5 summaries; all 44px, none below |
| Table alternative | Space opens; Tab reaches first link with outline | open; `api.Handle` link, solid 2px |
| AX tree, five images | name + description, no exposed children or links | as expected in every cell |
| 390 px diagram region | ArrowRight scrolls it | scrollLeft 80 after two presses |
| Destinations | pointer on SVG node; Enter on table link | both reach their `/graph?symbol=…` URLs |

The first run (harness `f7b09335…`, binary `2be390d6…`) passed every other row but
did not measure target size; the review below found table links at 17px and SVG node
links at 40px (28px without a group line). Twenty specimen captures (5 × 4 cells) were
reviewed then, and nine re-captures (neighbourhood, bounded and given columns at
1280 px light/dark and 390 px dark) after the fixes; all stay in the session scratch
directory, not the repository. Two defects found in the first visual review were
fixed before that run: Chromium exposed the SVG's children and links to assistive
technology despite `role="img"` (now `aria-hidden` drawing), and an edge was drawn
through an empty column's `None` box (placeholders now paint above edges). The same
browser instance was shared with another session; every result above reads elements
only `/graph` renders, and screenshots were checked to show this page.

## Review follow-up

A review of the first commit returned nine findings; all were reproduced and fixed
test-first.

| Finding | Fix | Pinned by |
|---|---|---|
| "+N more" edges took the OR of member styles and emphasis (one heuristic caller made eight read as heuristic) | edges into a generated aggregate merge only within one style and emphasis; each lane is drawn separately (spread ports forward, 4px apart otherwise) and has its own table row and count | `TestGraph_AggregateKeepsEachLineStyle`, `TestGraph_ParallelBackAndSameColumnLinesDoNotOverlap`, Gallery bounded specimen |
| Edges table named endpoints by label only | one name per node everywhere in the table: label, else label (group), else label (ID) | `TestGraph_TableNamesRepeatedLabelsUniquely` |
| Table links 17px, SVG node links 28/40px | node boxes are 44px once any node links; Gallery table links `inline-flex min-h-11`; the harness now measures targets | `TestGraph_LinkedNodesAreFullSizeTargets`, `TestGraphTableLinksOwn44pxTargets`, target row above |
| Catalog listed Fathom as a consumer after selection only | `consumers` is empty; the gap states the selection | `TestLayeredGraphStaysExperimentalL3a`, `TestGraphCatalogPageClaimsNoConsumer` |
| Search missed "code neighborhood", "neighborhood", "DAG" | task phrases in both spellings and DAG | `TestLayeredGraphFoundByConsumerWording`, two selection-corpus cases |
| Scope stated 35 catalog records | 36 | `TestScopeStatesCatalogSize` |
| No golden pinned self-loops or same-column arcs | `graph_arcs` golden plus viewBox and gap bounds over seven fixtures; the last-column reservation now grows with the arc (a 3-step arc's control points had reached 4px past the viewBox) | `TestGraph_Arcs_Golden`, `TestGraph_GeometryStaysInsideTheViewBox`, `TestGraph_ArcsStayInTheirGap`; mutations (loop controls at x+80, 200px bulge, fixed 24px reservation) each fail them |
| Aggregate box cues (fill 1.1:1, line-strong border 1.7–2.0:1) below 3:1 while the legend gives them meaning | the dashed aggregate border and its legend sample use ink-muted (≥ 6.7:1 on the sunken fill inside, ≥ 7.1:1 on the surface around it, light and dark) | `TestGraph_AggregateCueIsMeasuredRole`, contrast pairs |
| C0 controls passed validation and broke standalone XML | node and option text refuses control characters other than tab/LF/CR, and U+FFFE/U+FFFF | seven refusal cases, `TestGraph_SVGIsWellFormedXML` |

## Limits

Not measured: render time, memory, SVG size budgets, screen readers, other browsers,
forced colors, print, zoom, or app-owned themes. Long edges and cycle-closing edges
can pass behind intermediate nodes; labels are width-estimated, so a wider consumer
font may crowd a box. Wording (Focus, +N more, None) is English. Fathom has selected
the renderer but not integrated it; no second consumer exists, so it stays experimental.
