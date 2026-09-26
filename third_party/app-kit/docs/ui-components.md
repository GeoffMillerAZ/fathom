# app-kit UI components

The kit's reusable UI lives under `appkit/ui/`: Go renderers, token-themed CSS (D7)
and optional portable scripts. Consumers need no npm toolchain or client framework
for these packages. Data-viz components return SVG strings; HTML components return
HTML or templ components. Package tests cover selected semantics and deterministic
output; accessibility and interaction of a composed application need browser proof.

Colors are **token role names** (`accent`, `positive`, `caution`, `danger`,
`info`, `ink`, `line`, `surface`, …) resolved to `var(--ak-color-<role>)` at
render time, so every component re-themes light/dark with no rebuild. Invalid
roles fall back safely.

## Tiers — what a consumer has to run to use each package

The kit is layered (**D17 / [ADR-0017](adr/0017-multi-target-layered-kit.md)**)
and a consumer takes any prefix and stops. The tier is the whole answer to "what
do I have to set up for this to render correctly", so every package carries one.
`TestUIComponentsDocCoversEveryPackage` fails if a package enters `appkit/ui/`
without a row here — a package with no tier is a package whose consumer finds out
by rendering it wrong.

| Tier | What it costs a consumer |
|---|---|
| **L0** | nothing at runtime — CUE token sources, read by the generator only |
| **L1** | one `<link>` or `@import`. Plain CSS: `--ak-*` custom properties and fixed class names, no literal colors, ≤ one class of specificity, no preflight |
| **L2** | one `<script>`. Classic script, zero dependencies, no build step, one `window.*` global, `go:embed`ed with `Script()`/`Handler()` |
| **L3a** | a Go call, and **nothing else** — zero `class=` in the package, every color via `cssColor(role)` → `var(--ak-color-<role>)` on a presentation attribute. **No Tailwind, no `@source` glob, no CSS build.** Inject the returned SVG string anywhere, including into a React app |
| **L3b** | a Go call **plus** `appkit css` with an `@source` glob over the package's `*.go` — it emits Tailwind role utilities, which do not exist until the scanner has seen them |
| **L4** | L3b, plus templ (D4) and a Go server (D5) |

| Package | Tier | Notes |
|---|---|---|
| `tokens` | **L0**, emits **L1** | CUE token sets in, `theme.css` + DTCG JSON + Go role constants out; `Contrast`/`ContrastOver`/`ContrastRoles` measure WCAG in Go with no browser |
| `themes` | **L1** | shipped alternate palettes (`synthwave`); `theme.css` per theme, plus a Go `CSS()` embed so the file rides in `go mod vendor` |
| `motion` | **L1 + L2** | `motion.css` is the vocabulary (L1); `motion.js` is an optional style-flush helper (L2). The reduced-motion cancel is welded to `.ak-motion`, the same class that grants motion |
| `router` | **L2** | `router.js`, History-API client router for static-client surfaces only (D16 boundary still binding) |
| `viz` | **L3a** | the portable tier. Zero `class=`, color only through `var(--ak-color-*)`; asserted by `TestVizEmitsNoClassAttribute` and `TestVizEmitsNoLiteralColor` |
| `overlay` | **L2 + L3b** | `overlay.js` is L2; the Go markup emits role utilities |
| `palette` | **L2 + L3b** | experimental command palette: `palette.js` is L2 (`window.AppKitPalette`, `data-palette-*` hooks); the trigger, dialog and results renderers emit role utilities — scan `appkit/ui/palette/*.go` and `appkit/ui/overlay/*.go` |
| `appearance` | **L2 + L3b** | portable light/dark/system behavior and an optional native-select control; app chooses the storage key, persistence failures remain visible; scan `appkit/ui/appearance/*.go` for the control |
| `chat` | **L2 + L3b** | `chat.js` is L2; the Go markup emits role utilities |
| `live` | **L2 + L3b** | experimental; `live.js` is L2 (`window.AppKitLive`, no dependencies); the region/status helpers emit role utilities, so scan `appkit/ui/live/*.go` |
| `callout` | **L3b** | |
| `chrome` | **L3b** | `RenderPageHeading` renders actions through `form.RenderButton`: scan `appkit/ui/form/*.go` too |
| `docs` | **L1 + L3b** ² | `prose.css` is the L1 typography layer (`doc-prose`, the boxed `doc-toc`, the heading permalink hook); `toc.go`/`anchor.go` are L3b — `RenderToC`, `RenderToCPanel`, `RenderToCRail`, `RenderPermalink`. Needs `@source ".../appkit/ui/docs/*.go"` plus `@import ".../appkit/ui/docs/prose.css"` |
| `code` | **L3b** | experimental read-only source excerpt: numbered lines, labelled per-line state, `#L<n>` anchors, keyboard-scrollable viewport; no syntax highlighting. Needs `@source ".../appkit/ui/code/*.go"` |
| `dashboard` | **L3b** | layout only — charts/KPIs/tables come from `viz`/`stat`/`table` |
| `form` | **L3b** | |
| `hitl` | **L3b** | includes the experimental line-level `RenderUnifiedDiff` and its Myers `DiffText`; scan `appkit/ui/hitl/*.go` |
| `stat` | **L3b** | composes `viz.Sparkline` (L3a) inside an L3b tile |
| `topology` | **L3b** | emits literal Tailwind utility strings nine times — the clearest L3b in the kit |
| `vizframe` | **L3b** | the provenance frame (ADR-0017 §6). Needs `@source ".../appkit/ui/vizframe/*.go"` |
| `analytics` | **L3b** | opt-in typed comparison recipe with missing/freshness/coverage context, bars, visible table and CSV; scan both `ui/analytics/*.go` and `ui/vizframe/*.go` |
| `vizcheck` | **L3a test helper** | Go-only static HTML assertions; renders nothing and needs no Tailwind scan or browser runtime |
| `explainer` | **L3b** ¹ | |
| `export` | **L3b** ¹ | |
| `card` | **L3b + L4** | templ |
| `kanban` | **L3b + L4** | templ |
| `table` | **L3b + L4** | templ |
| `internal` | **L3a** (private implementation only) | Shared `webref` destination-shape checks for managed form/table/excerpt links and `linetext` line splitting plus hidden-character reveal for `code`/`hitl`; no rendering, class names, CSS scan or consumer API. Delivered through the importing Go package's dependency closure. |

¹ `explainer` and `export` are **self-contained by contract**: each renders a
single file carrying its own inline CSS/JS, so they neither consume the kit
theme layer nor compose `ui/motion`. Both exemptions are stated in their package
docs with the reason, and both carry their own reduced-motion escape. They are
listed as L3b because that is the strictest thing a consumer might assume; in
practice they need nothing at all.

² `docs` ships with **one** consumer (platform-engineering's portal), short of
D17's ordinary two-consumer entry bar for L3b — a named, on-the-record
exception (`docs/journals/app-kit.md`, platform-engineering, 2026-08-19),
because `docs/architecture/portal.md` §8 already named the component and
ADR-0019 already accepted its cost. Not exempt from D17's "zero consumers for
two releases is demoted" clause.

**The L3a/L3b split is the one that surprises people, and it existed
undocumented for a year.** A kit chart drops into a page with no Tailwind and no
build; a kit callout does not. When you add a component, pick the tier
deliberately — a `class="text-ink"` added to `viz` would silently demote the
whole package and break every consumer that took it without a scanner, with no
error, because the class simply would not resolve.

For L3b/L4 packages, the consuming app's CSS must include the kit theme layer
and `@source` the component package — see
[getting-started.md](getting-started.md) and the Assistant's `get_kit_css_setup`.

## `appkit/ui/viz` — data visualization (SVG)

At-a-glance charts. Every color is a token role; same inputs → byte-identical
SVG (golden-tested); each emits `role="img"` + `<title>`.

| Component | Use it for |
|---|---|
| `Sparkline` | inline trend line |
| `StackBar` | one horizontal stacked bar (composition) |
| `Bars` | labeled horizontal bar chart (comparison / distribution) |
| `Donut` | distribution ring |
| `Funnel` | vertical funnel (intake → outcome) |
| `MeterRow` | labeled value/max meter |
| `Gauge` | 180° score dial (case strength, likelihood, confidence); `Graded` auto-colors |
| `Rating` | n-of-M pips (evidence strength, confidence); `Graded` |
| `Steps` | lifecycle/progress stepper (done / current / upcoming) |
| `Timeline` | events + spans + markers; views: proportional / sequence / condensed (honest gap-elide) / swimlanes / density; plus `EventFilter` (window / lane / role / search) |
| **`RangeBars`** | low–high interval bars + most-likely marker (recovery ranges) |
| **`Waterfall`** | running-total columns (combined recovery → net; up/down/total) |
| **`CostReward`** | diverging reward-vs-cost bars from a center axis |
| **`Tornado`** | sorted ± swings around a baseline — which factors move the outcome most (sensitivity) |
| **`LayoutGraph`** / **`LayoutEgo`** | experimental layered directed graph (longest-path or given columns) and a focus node's callers/callees by depth; one layout renders `SVG()` plus a class-free `Table()` alternative |
| **`Heatmap`** | matrix intensity grid (coverage maps, activity by period); optional cell truth states keep a measured zero distinct from missing / unknown / error |

`GradedRole(frac)` maps 0..1 → danger / caution / positive for consistent
status coloring across components.

**Fluid width.** Every chart drawn at a fixed pixel width — `Sparkline`,
`StackBar`, `Bars`, `Donut` (`Size`), `Funnel`, `MeterRow`, `Gauge`, `Steps`,
`Timeline`, `RangeBars`, `Waterfall`, `CostReward`, `Tornado` — takes
`Fluid bool`. Unset, output is byte-identical to before. Set, the opening tag
becomes `width="100%"` with no `height`, the same `viewBox` and
`preserveAspectRatio="xMinYMin meet"`: the chart fills its container's width,
the browser derives the height from the viewBox ratio, and the drawing itself is
unchanged. No class or inline style is emitted, so it stays L3a and strict-CSP
safe. Fluid scales; it does not reflow. Text shrinks when the container is
narrower than `W` and grows when it is wider: a 560px `Bars` in a 340px column
draws its 11px labels at about 6.7px. Use it for text-free charts (a sparkline
across a card), set `W` near the narrowest container a text chart must stay
legible in and bound the container's width, or keep a fixed-width chart in a
keyboard-scrollable region beside a table alternative. `Heatmap`, `Rating` and
`LayoutGraph` size themselves from their content and have no Fluid option.
Gallery `/components` shows a fluid sparkline (FATHOM-F0-KIT-04,
[evidence](assessments/2026-09-25-fathom-f0-kit-returns.md)).

Experimental `LayoutGraph(nodes, edges, GraphOptions)` and `LayoutEgo(focus, nodes,
edges, EgoOptions)` validate before output (refusals wrap `ErrInvalidGraph`; inputs
bounded at 2000 nodes, 8000 edges, 64 columns; text must be valid UTF-8 without
control characters other than tab/LF/CR, so the SVG stays well-formed XML) and
return one `GraphLayout`.
Columns come from longest path (cycles broken deterministically, the closing edges
drawn right to left) or app-given ranks; barycenter sweeps with stable tie-breaks
order each column. `MaxPerRank` (default 8) collapses the rest of a column into
one "+N more" node; edges into it keep one line and table row per style and
emphasis, each with its own count. Every reduction appears in `Notes()`, the SVG
`<desc>` and the table. `SVG()` is `role="img"` with `<title>`/`<desc>` over an
`aria-hidden` drawing; its node links are pointer-only (`tabindex="-1"`), and once
any node links every node box is at least 44px high. `Table()` is plain
semantic HTML — no class or style — with captions, header scopes and ordinary links,
so apps style it (give its links 44px-high hit areas) and keyboard/assistive-technology
users reach nodes there; repeated labels are qualified by group, or by ID, so each
row and edge endpoint names one node. Name edge meanings with
`SolidLabel`/`DashedLabel`/`EmphasisLabel`; wrap wide graphs in a keyboard-scrollable
region. Catalog `analytics.graph`; Gallery `/graph`;
[scoped evidence](assessments/2026-09-25-layered-graph.md). Long edges are not
routed around intermediate nodes, and the originating consumer (Fathom) has not
integrated it yet.

### Heatmap truth states (experimental)

Set `HeatmapOptions.States` (`[][]HeatmapCell{State, Reason}`, row-major and
parallel to the rows; the grid spans both) or `HeatmapOptions.TruthStates` to
render every cell as one of five states. Absence is never drawn as a low value:

| State | Drawn as |
|---|---|
| value | the `Role` ramp (`0.12 + 0.85·norm`), unchanged; labels, titles and the range never round a nonzero value to `0` (below 0.005 it keeps two significant digits) |
| `CellZero` (or a measured `0`) | its ramp fill (the ramp floor when every measured cell is zero) plus a `0` glyph (cells ≥ 14px, or with `ShowValues`); below 14px without `ShowValues` only the fill remains, which the ramp's lowest values share |
| `CellMissing` (or `NaN`) | no fill; dashed outline and `/` hatch in `ink-muted` |
| `CellUnknown` (±Inf, unrecognized state) | no fill; dotted outline, cross-hatch and `?` |
| `CellError` | no fill; solid `danger` outline and cross |

Each cell carries a `<title>` naming row, column, state and reason; a `<desc>`
summarizes the counts, and a legend lists only the states present (with the
shared reason when every cell in a state has the same one). Missing, unknown and
error cells ignore any numeric entry and never stretch the value range. Hatches
are drawn as geometry, not `<pattern>` references, so the SVG needs no
document-unique IDs. `HeatmapOptions.Validate(rows)` reports contradictory input
(unrecognized states, a zero-state cell carrying a number, non-finite values, an
inverted range, invalid roles); the renderer itself never refuses. Leave both
fields unset and output is byte-identical to the previous renderer (golden-tested).
Titles are pointer-only inside `role="img"`: give keyboard and screen-reader users
the same cells as a table. The Gallery's `/components/heatmap` specimen shows the
table alternative, an all-missing grid and the legacy comparison. X labels and
`ShowValues` labels are not fitted to the cell (at the default 26px, `-0.004` or
`-12.25` runs into its neighbor), so size `Cell` for your labels.

## `appkit/ui/callout` — messaging (HTML)

| Component | Use it for |
|---|---|
| `Callout` | block message: info / success / caution / danger / note (ARIA note/status/alert + icon); `Body` (escaped) or `BodyHTML` (trusted rich/block) |
| `Disclaimer` | the standard "information only / not advice" caution callout |
| `Badge` | inline status pill tinted by role |

## `appkit/ui/stat` — KPI tiles (HTML)

`Stat` — a big value + label + optional colored delta + optional trend
sparkline (composes `viz.Sparkline`).

`Render` and `RenderBand` let captions wrap before squeezing short values/units;
oversized strings remain visible with emergency word wrapping. Four/five-item bands
use two columns at tablet width, reaching full density at larger breakpoints. Cell-owned
separators also support partial final rows. Rebuild
consumer CSS after upgrading: these are L3b scanner-visible layout changes. The
Gallery's `/layout#stat-reflow` exercises zero, lower bounds, absence, long source
identifiers and a delta/sparkline. See [geometry/contrast evidence](assessments/2026-09-14-stat-reflow.md);
this is not a guarantee that arbitrary labels or values fit every app-owned container.

## `appkit/ui/overlay` — dialog, drawer, toast (HTML + tiny vanilla JS)

Built on the native `<dialog>` element + a dependency-free `overlay.js` (served
like `router.js`). Loopback-safe (D5): markup works without JS; the script adds
open-by-id, light-dismiss, and client toasts.

| Component | Use it for |
|---|---|
| `Dialog` | centered modal (sm/md/lg); optional backdrop light-dismiss; Escape closes |
| `Drawer` | full-height slide-over anchored left/right |
| `Toast` / `RenderToastRegion` | notifications — server-rendered flash *or* client `AppKitOverlay.toast()`; roles info/success/caution/danger |
| `TriggerButton` / `OpenAttr` | open an overlay by id (`data-overlay-open`) |
| `Script` / `Handler` | inline or serve `overlay.js` |

## `appkit/ui/palette` — command palette (experimental, HTML + tiny vanilla JS)

Catalog `interaction.command-palette`. The trigger is a real link to the app's GET
search page, so everything works without JavaScript. `palette.js` upgrades it into
a native modal built on `overlay.RenderDialog` (overlay.js is not required): Mod+K
(Command on Apple platforms, Control elsewhere; configurable or `none`) opens it
from anywhere and focuses the input; typing fetches the app's results fragment after
a debounce and aborts stale requests; ArrowUp/ArrowDown move an
`aria-activedescendant` selection, Enter follows it, Escape closes and returns focus
to the invoker. Enter with no selection, or while the displayed results answer an
older query, submits the GET form, so the full page answers exactly what was typed.
The script inserts a response only when it is exactly one fragment for the query it
asked for; a full page (even one rendering the palette) or another query's fragment
shows the error state. Call `AppKitPalette.init(scope)` after swapping palette or
trigger markup: it wires new roots, drops swapped-out ones and re-upgrades triggers.

| Component | Use it for |
|---|---|
| `Palette` | typed configuration: ID, Action (search page; its query becomes hidden fields), ResultsURL, Param, Label, Placeholder, Shortcut, Debounce, Timeout, IdleText, ErrorText; invalid values are refused, not corrected |
| `RenderTrigger` | the 44px search link; the script adds the platform-localized shortcut hint and `aria-keyshortcuts` only once its palette is wired |
| `RenderDialog` | the dialog: combobox input, fixed-height results region, live status, failure template, key hints and an "Open search page" link |
| `RenderResults` / `Group` / `Result` | one fragment for both the palette and the full page: grouped links plus idle, empty, partial (`Notice`, with or without results) and error states; set `Query` to the request's query; `HeadingLevel` 2 on a page |
| `RenderSearchForm` | the full page's labeled GET form |
| `WantsFragment` / `FragmentHeader` | tell the palette request apart on one route; send `Vary: X-AppKit-Palette` |
| `Script` / `Handler` | inline or serve `palette.js` (revalidating, like `appearance`) |

The app owns the route, authorization, sources, ranking, destinations, the page
layout and shortcut conflicts. Built-in secondary copy is English. The Gallery's
`/palette` reproduces every state; `palette_test.mjs` pins the script lifecycle with
DOM doubles. Fathom originated it; it stays experimental until two consumers integrate it.

## `appkit/ui/form` — form & input vocabulary (HTML, no JS)

Token-themed, accessible, escaped, server-rendered. `Field` owns the label/help/
error plumbing; controls carry their own ARIA so a bare control stays consistent.

| Component | Use it for |
|---|---|
| `Field` | labeled wrapper: label + control slot + help/error (`aria-describedby`, `aria-invalid`); error takes priority over help |
| `Input` | single-line control of any text-like `Type` (text/email/number/date/…) |
| `Textarea` | multi-line text |
| `Select` | dropdown with optional placeholder prompt + selected option |
| `Checkbox` | single boolean with inline label; optional `Invalid`/`HasHelp` wire `ErrorID`/`HelpID` (caller renders matching messages) |
| `RadioGroup` | mutually-exclusive options as a `<fieldset>` + `<legend>`; grouped requiredness and associated help/error feedback |
| `Button` | action control — `primary` / `ghost` / `danger`; native `<button>`, safe enabled `<a>`, or noninteractive disabled link label |

`Select.Value` selects a matching option. A nonempty value absent from `Options`
instead selects one disabled empty-value prompt: the escaped `Placeholder`, or
`Selection unavailable` when none was supplied. The missing raw value is not reflected
and no valid option is selected. With an initially empty value and a placeholder, that
placeholder alone is selected; without a placeholder, native first-option behavior and
app-owned empty-option meaning remain unchanged.

For an unavailable optional value, native `FormData` omits the select entry; `Required`
blocks submission through native missing-value validation. The renderer does not infer
invalid/error state, add descriptions, change existing control attributes/classes or
choose a valid fallback. Apps must validate and explain stale values and explicitly set
a valid fallback when desired. Matching disabled options keep existing native semantics:
they can satisfy requiredness while being omitted from submission. Validate membership,
permissions and submitted values server-side; this missing-value repair is not a policy
for disabled choices. Rebuild, review snapshots and selected `interaction.forms` pins
([ADR-0083](adr/0083-render-missing-select-values-as-unavailable.md)).
The [assessment](assessments/2026-09-23-select-state.md) separates controlled-option
browser checks from the retained failed native-key choice and consumer acceptance.

Nonempty `Button.Href` selects link mode: relative references (including query/fragment)
and absolute HTTP(S) are allowed; malformed, protocol-relative, backslash/control-bearing,
surrounding-whitespace and other/opaque-scheme destinations are refused. Refused or
`Disabled` links retain an escaped label in a nonfocusable `span` with link role and
`aria-disabled="true"`, never a form submit. Apps own destination trust, authorization
and explanations for unavailability. Empty Href keeps the native button contract.
This tightens previous behavior: rebuild source/CSS and review markup snapshots;
non-web schemes need separately reviewed app-owned links ([ADR-0079](adr/0079-disable-unsafe-form-button-links.md)).

`RadioGroup` renders its active message and wires every radio with `aria-describedby`;
error takes priority over help and adds `aria-invalid="true"` plus danger focus styling.
Give each group a nonempty, document-unique `Name`. Feedback IDs encode its bytes as
`ak-radio-<hex>-help|error`; reserve those IDs along with existing `Name-index` control
IDs. Empty names retain no generated description relationship. Selection does not clear
an app-owned error: rerender after validation with `Error` empty to restore help.
Names/values, native keyboard grouping and disabled-fieldset inheritance stay unchanged;
rebuild scanned CSS and review markup snapshots. [Scoped evidence](assessments/2026-09-23-radio-feedback.md)
does not certify every theme, browser or assistive technology.

Requiredness belongs to the native radio group; the renderer marks its first member.
Disabling that member does not make enabled peers optional. Disabling the whole
fieldset instead removes its controls from constraint validation. Keep server-side
validation app-owned ([bounded native control](assessments/2026-09-23-radio-requiredness.md)).

Experimental `SettingsSection{HeadingID, Title, Description, Content}` and
`RenderSettingsSections` compose description/control rows, stacked below sm and two
columns above. Both columns wrap long words, including control labels/help/errors,
without rewriting native controls; explicit child text-flow styles can override this.
Place their h3 headings beneath your h2; use document-unique valid UTF-8
IDs without whitespace/control characters. Duplicate IDs or blank titles fail atomically;
an empty list renders nothing. Plain-text fields are escaped; `Content` is reviewed HTML.
The renderer adds no form, fieldset, label, submit action or script. Wrap controls with
`RenderField` as needed and keep validation, authorization, saving and recovery app-owned.
Scan `appkit/ui/form/*.go` for Tailwind and include theme CSS. The compiled
`ExampleRenderSettingsSections` and [shell guide](design/application-shell.md) show the
boundary; [ADR-0064](adr/0064-incubate-responsive-settings-sections.md) retains two-consumer promotion.

## `appkit/ui/dashboard` — shell, navigation and layout (HTML, no JS)

`RenderNav` accepts app-owned `NavGroup` and `NavItem` values. It escapes labels,
disables unsafe destinations and marks at most one best path match with
`aria-current="page"`; `MatchSubtree` respects route-segment boundaries. Icons are
trusted HTML and require caller review. The app still owns routing, permissions,
destinations and mobile composition.

`RenderShell` can place the same navigation in its desktop and native mobile
disclosure. An enabled skip link targets a programmatically focusable main region
without adding that region to the normal tab order. These are L3b renderers: scan
`appkit/ui/dashboard/*.go`, include the theme CSS, and verify keyboard focus, narrow
layout, long labels and theme contrast in the composed application.

Navigation items and the mobile disclosure carry a 44px minimum height and explicit
focus treatment. The current item uses the normal ink role on the soft accent surface;
consumers still verify delivered contrast and target width in their composed shell.

Experimental `AccountDestinations{Profile, Settings, Help, About}` produces a fresh
`Group(heading)` for `RenderNav`. Omit destinations with neither URL nor label; a supplied
URL gets its role's default label, while a label without a URL stays disabled. Raw model
fields are not validated URLs: custom quick/help links need their own destination policy;
template escaping alone does not reproduce RenderNav's disabled-item behavior.
`RenderNotice(Notice{Kind, Title, Body})` renders escaped status/error/conflict feedback;
empty bodies emit nothing and unknown kinds use alert treatment. The app determines
the outcome and owns actions, announcements/focus during updates, authorization and
save/recovery. These L3b helpers add no runtime or session backend. See the compiled
`ExampleAccountDestinations_Group`, [shell guide](design/application-shell.md) and
[incubation boundary](adr/0063-incubate-account-presentation-recipe.md).

## `appkit/ui/chrome` — structural creature comforts (HTML, no JS)

| Component | Use it for |
|---|---|
| `EmptyState` | "nothing here yet" panel (icon + heading + body + CTA) |
| `Breadcrumbs` | navigation trail; links are 44px targets with a focus-visible outline |
| `Pagination` | windowed prev/next + page links (server-rendered) |
| `Tabs` | link-based view switcher (active underline) |
| `Skeleton` / `SkeletonCard` | loading placeholders (Tailwind `animate-pulse`) |
| `Disclosure` | collapsible "show more" section on native `<details>` (no JS) |
| `DescriptionList` | aligned term/detail metadata (`<dl>`) |
| `PageHeading` / `RenderPageHeading` | experimental: the page's one `h1` with optional `Breadcrumbs` trail, eyebrow, meta line and up to four `form.Button` actions |

All server-rendered with plain links/markup, so they honor the loopback
invariant (D5) — no shell-specific JS.

`Breadcrumbs` links render the kit's 44px minimum target (`inline-flex min-h-11
min-w-11`) and a 2px `focus-visible` outline in the focus role; the current page
and the separators stay plain text. The trail row is therefore 44px tall: rebuild
scanned CSS and review snapshots when upgrading (FATHOM-F0-KIT-03,
[evidence](assessments/2026-09-25-fathom-f0-kit-returns.md)). `RenderPagination`
links are still about 28px high, below that floor.

Experimental `RenderPageHeading(PageHeading{ID, Breadcrumbs, Eyebrow, Title, Meta, Actions})`
composes the existing `Breadcrumbs` output and `form.RenderButton` actions unchanged
(unsafe action destinations stay inert labels; set `Type: "button"` for script-owned
actions). `Breadcrumbs` writes every crumb `Href` as a link, so the heading refuses a
crumb destination that is not relative or HTTP(S) (`javascript:`, `data:`,
protocol-relative, padded). Every field is escaped plain text; a blank title, control
characters, more than eight meta items or four actions, or an invalid `ID` fail before
output. The title column
shrinks and wraps long words; below `sm` the actions wrap onto their own row beneath
the title, with 44px minimum targets. Scan `appkit/ui/chrome/*.go` and `appkit/ui/form/*.go`. Gallery `/page-heading`
shows full, title-only and long-content variants; Fathom (FATHOM-KIT-07) is the
originating consumer, with integration and second-consumer promotion still open.

## `appkit/ui/live` — live freshness (L2 + L3b, experimental)

Tells a reader that a server-rendered panel has newer data **without re-rendering or
moving it**. `live.js` (served by `Handler` or inlined with `Script`) reads an app-owned
server-sent event stream with `fetch`, matches each event to elements carrying
`data-live-source` and swaps that region's reserved status line from the app's as-of
text to "… · Updated — refresh this panel". One polite live region announces the
change (debounced, once per region). Refresh reloads the page at the region and
returns focus to it; event data and region HTML are never inserted.

| Piece | Use it for |
|---|---|
| `RenderRegion(Region{ID, Sources, Label, AsOf, RefreshHref, Body})` | labelled, programmatically focusable section around reviewed app HTML plus its status line |
| `RegionAttrs` + `RenderStatus` | the same hooks on an app-owned wrapper (e.g. a dashboard widget footer) |
| `RenderPageStatus(Page{URL, Events, LastEventID})` | once per page: connection line, one stop/resume native button and the polite announcer; auto-starts the script from the render-time cursor |
| `AppKitLive.start/stop/state/mark` | explicit configuration (`sources(message)` maps custom payloads; without `lastEventId` it adopts the cursor of the page status with the same URL and connects once the document is parsed), the manual stop hook, diagnostics |

Events carry JSON `{"source": …}` or `{"sources": [...]}` naming app-owned keys; `*`
marks every region (send it when a replay gap cannot be honored). Reconnects resend
`Last-Event-ID` as the id's UTF-8 bytes (any id round-trips), back off exponentially
with equal jitter (8 attempts, 30 s cap; the first retry waits 1–1.5× the 1 s floor or
a larger server `retry`), treat 60 s of silence as lost, stop on 204 and do not retry
other 4xx except 408/429. A connection earns a fresh attempt budget only by staying
open 10 s or delivering an event that advances the cursor, so a server that accepts and
closes still exhausts the attempts. The stream pauses after the page is hidden for 5 s
and resumes on return. Without the script the markup says "Live updates are off.
Reload for newer data." Nothing moves at any width: the region status layers share one
grid cell, the connection line stacks an invisible copy of every message it can show,
and the stop/resume control keeps its box (invisible until usable, never offered when
it cannot act) and reserves its wider label; this relies on the scanned `invisible`
utility. A refocused region shows a focus-visible ring. The package doc states the
keyboard and connection contract. There is no event taxonomy, replay store,
authorization or portable vendoring. Gallery `/live` drives a synthetic fixture
(publish, gap, dropped streams, refused connections); Fathom (FATHOM-KIT-06) is the
originating consumer, not yet an adopter.

## `appkit/ui/appearance` — preference behavior and control

`BootstrapScript(Options{StorageKey: "your-existing-key"})` runs before CSS to restore
light/dark/system. `RenderControl` adds a native selector, status and retry.
The script emits `appkit:appearance-change`; app-owned canvas charts can redraw on
effective-theme changes. Empty storage key means session-only. This shares appearance
behavior, not an account/settings backend. Read the package contract/tests before adoption.

## `appkit/ui/analytics` — opt-in comparison recipe

`Dataset.Validate(now)`, `RenderBars(dataset, options, now)` and
`WriteCSV(writer, dataset, now)` use one app-supplied metric contract and row selection.
Experimental `WriteTableCSV(writer, dataset, now)` uses the same cells in one
27-column spreadsheet table with repeated context; empty selections have headers
only. Keep `WriteCSV` when empty-selection provenance must travel with the export.
Units, time window, source freshness, completeness, uncertainty and missing values
stay explicit. Chart, visible table and CSV use the same presentation rounding;
CSV is not a lossless source-data export. Bounds: 200 rows, 0–2 decimals, magnitude
up to 1e12. Negative values export but are rejected by this bars composition.
`RenderDivergingBars` is the experimental signed alternative. Both high-level
renderers retain all data in an explained table fallback for positive `RowH < 16`;
nonpositive row height still uses the 28px default. Low-level `viz.Bars` is unchanged.
Optional `Row.DetailHref` adds a native root-relative table-label link in both
renderers, including fallback/missing rows; empty preserves old markup and CSV
omits row-detail URLs (source links remain in context). Apps own routes, permission and return context. **Breaking for
positional Row literals:** migrate to named fields; keyed callers may omit the field.
See the [full contract](../appkit/ui/analytics/README.md); no query engine or domain
calculation is supplied. Consumer/browser adoption remains a separate G3 requirement.

## `appkit/ui/vizcheck` — static composition checks

`CheckHTML(markup, spec)` compares an explicit chart/table/source fixture and accounts
for every SVG under its selected root. Errors and findings both fail the test. No
CSS/runtime code is added; labels and name declarations are not geometry, computed
visibility, accessibility or source-truth proof. See the [contract and canaries](../appkit/ui/vizcheck/README.md).

## `appkit/ui/docs` — prose chrome for rendered documentation (L1 CSS + HTML)

Prose typography, a table of contents in two viewport treatments, and the
heading permalink that pairs with it. Promoted from platform-engineering's
portal (`docs/architecture/portal.md` §8; see the package doc for what did
and did not travel, including the two named parts — the honesty markers and
the document layout shell — that stayed in the portal on purpose).

| Component | Use it for |
|---|---|
| `Heading` | one heading: level, text, and an id the CALLER already assigned — this package never derives one from Text (see the package doc's "anchors are given" section) |
| `RenderToC` | boxed nested-list ToC, prepended to a document rendered inline (a printable bundle, a style-guide specimen) |
| `RenderToCPanel` | the outline as a native `<details>` disclosure — narrow-viewport treatment |
| `RenderToCRail` | the outline as a sticky column — wide-viewport treatment; both panel and rail render on every request and CSS picks one, since a zero-JS page cannot measure the viewport to choose |
| `RenderPermalink` | the "#" a reader clicks to copy a stable link to one heading — "anchor nav" |

`prose.css` (L1: plain CSS, `@import` it, no `@source` needed) carries the
markdown-body typography (`doc-prose`/`prose-atrium`), the permalink's
hover/focus treatment, and `RenderToC`'s boxed styling. `RenderToCPanel` and
`RenderToCRail` need none of it — they emit Tailwind role utilities only, so
`appkit/ui/docs/*.go` needs the ordinary L3b `@source` line instead.

## `appkit/ui/code` — code excerpt viewer (HTML, no JS)

Experimental. `RenderExcerpt(Excerpt{Label, Caption, SourceHref, AnchorPrefix, Lines})`
renders numbered source lines in a named, keyboard-scrollable viewport (`tabindex="0"`,
visible focus ring); code never wraps and the number/marker gutter stays pinned while
long lines scroll. Each `Line` carries an app-supplied `LineState` — normal, changed,
not executed or highlighted — shown as a role-tinted row, a text marker, a
screen-reader label and a legend entry with counts, so color is never the only
channel. `StateLabels` renames a state ("Uncovered"); the app decides what it means.

Line ids are `AnchorPrefix + "L" + number` (default `#L120`), and each line number is
a native permalink to its own anchor; a targeted line gets a focus-colored frame via
CSS `:target`. Give each excerpt on a page its own prefix. Numbers must increase; a
jump renders an explicit "Lines 122–129 not shown" gap. `SplitLines(start, text)`
numbers plain text. An empty excerpt says so; a blank label, non-increasing numbers,
unknown states, more than `MaxLines` lines or an unsafe `SourceHref` return an error
and no markup. Characters that draw nothing, reorder the line or change how it parses
render as visible code points in the lines, `Caption` and `SourceLabel`: controls, every
Unicode format character (bidi overrides and isolates, zero-width, soft hyphen, tag
characters), U+2028/U+2029 (a space to the browser, a line break to JavaScript),
default-ignorable fillers and variation selectors. Classification uses the Go release's
Unicode tables. The caption and link text are also isolated with `<bdi>`.

There is **no syntax-highlighting engine**: review excerpts render escaped monospace
text, which is what the originating task needs. Rows are 24px, so each line-number
permalink is a 48×24px target at a 24px pitch: WCAG 2.2 2.5.8's minimum, not the kit's
44px floor for standalone controls (a recorded divergence). Every line number is a tab
stop, so keep excerpts bounded. Scan `appkit/ui/code/*.go` and include the theme CSS. Gallery
`/agentic/code` shows every state, anchors, gap, revealed characters, empty and refused input.

## `appkit/ui/card`, `kanban`, `table`, `router` — composable layout (templ)

The templ-based composition primitives. `Card` is the single canonical bordered
surface (chrome no longer duplicates it); `kanban` composes it into a board.

| Component | Use it for |
|---|---|
| `card.Card` | the bordered-surface primitive: `Header` / `Body` / `Footer` slots (templ.Component); base for higher-level layouts |
| `kanban.Board` | server-rendered Kanban — `Column`s + `CardItem`s (StageID → Column.ID), composed from `card.Card`; emits `data-*` hooks for the app's hypermedia layer |
| `table.Table` | generic data table — typed `Column`s (optional sortable `<th>` links) + `Row`s; tolerant of cell/column length mismatch |
| `router` | embedded dependency-free History-API client router (`router.js`) for static-client surfaces (e.g. GitHub Pages SPAs); served via `Handler` or inlined via `Script`. No build step, no npm toolchain (ADR-0002 as amended by ADR-0017 §3) |

Managed `table.Row.Href` accepts unchanged relative web references and absolute
HTTP(S) destinations. Unsafe nonempty input renders an inert **Open unavailable**
label, not an actionable link; empty input adds no action. Enabled Open links have
44px minimum targets and explicit keyboard focus. Rebuild scanned CSS and review
row-density/snapshot changes when upgrading. Apps still own route authorization,
origin trust and explanatory context; custom `RowAction`/`HTMLCells`/`Attrs` and
row activation remain app-owned escape hatches. This is not sorting, selection or
whole-table accessibility certification ([ADR-0080](adr/0080-harden-table-row-destinations.md),
[scoped evidence](assessments/2026-09-23-table-link-safety.md)).
An action column is now inferred from any nonempty Href or a custom action slot;
`RowHrefs: true` can reserve it for empty/unlinked results. Every row gets that cell,
but a missing Href leaves it empty without inventing a control. Review selectors,
snapshots and app-authored footer spans when upgrading; inference does not mutate
caller data ([ADR-0081](adr/0081-derive-one-table-action-column.md)).
Every header cell carries `scope="col"`; the table has no row headers (body and
footer cells are `<td>`), so a first column that names its row needs app-owned
markup. The scroll wrapper is `relative`, which makes it the containing block
for the sr-only Select/Actions header labels: a table wider than a 390px screen
scrolls inside the wrapper instead of letting a hidden label widen the whole page.
The wrapper now also contains absolutely positioned content from custom cells;
an overlay that must escape it needs fixed positioning (FATHOM-F0-KIT-01/02,
[evidence](assessments/2026-09-25-fathom-f0-kit-returns.md)).

## `appkit/ui/hitl` — human-in-the-loop (HTML)

For agentic apps that need a person in the decision path and a window into the
run.

| Component | Use it for |
|---|---|
| `ApprovalCard` | a proposed agent action awaiting a decision: intent + risk badges, confidence rating, rich summary, **caller-supplied action HTML** (approve/reject/edit forms — the app wires the loopback POST) |
| `RunTrace` | a vertical event rail of an agent run (done / active / failed / pending) — "what's happening under the hood" |
| `DiffView` | before → after review of a proposed agent change (current vs proposed, per field; additions/removals) — pair with `ApprovalCard` |
| `ToolCall` | inspect one agent tool invocation: name + status badge + collapsible args/result (native `<details>`) — "what did the agent do" |
| `RenderUnifiedDiff` | experimental line-level review of a proposed text change: unified hunks, old/new line numbers, `+`/`-` markers as text, collapsed unchanged context (native `<details>`) — beside the field-level `DiffView` |
| `DiffText` / `LineDiff.Hunks` | deterministic Myers line diff and git-compatible hunk grouping, usable without rendering |

`RenderUnifiedDiff(UnifiedDiff{Label, Old, New})` compares two texts; `Hunks` accepts
precomputed hunks instead (pass `LineDiff.Replaced` as `Replaced` so the budget note
survives). They may be a subset of one comparison, so the lines between them are stated
per side as "not included" (never as unchanged, never invented); overlapping hunks and a
"no newline" mark anywhere but a side's last line are refused. Every line carries a spoken label ("Added, proposed line 9"),
rows are role-tinted with a left rule, the structure is lists (no table), and the
viewport is a named `tabindex="0"` region that scrolls long lines. Identical texts
render "No changes"; a change beyond the 1000-edit budget is shown as one replacement
block and says so. Hidden, format, tag, line-separator and direction-changing characters
and invalid bytes render as visible code points in the lines, label and side names (the
same classes as `ui/code`); names are isolated with `<bdi>`. Input is bounded by
`MaxDiffLines` per side; invalid input returns an error and no markup. There is no
word-level diff, side-by-side view or syntax highlighting. Apps own both texts,
line-ending normalization and whether the change is applied. Gallery `/agentic/code`.

## `appkit/ui/motion` — the motion vocabulary (L1 CSS + optional L2 JS)

Two-state interpolation, bounded staggering, and `prefers-reduced-motion` as an
invariant rather than an afterthought. `@import` `motion.css` after the theme
layer — **no `@source` entry needed**, the class names are plain CSS selectors,
not utilities.

| Piece | Use it for |
|---|---|
| `Root` (`ak-motion`) / `StateB` (`ak-state-b`) | grant transitions in a subtree and say which state it is showing; `RootClass(bool)` renders either state server-side |
| `WhenA` / `WhenB` / `Tone` | which state an element belongs to, or that it is present in both and changes color |
| `Fade` / `Tint` / `Shift` / `Anim` | pick exactly one per element — CSS transition lists do not merge across classes |
| `Step(i)` / `DelayStyle(d)` | index stagger (1…`MaxStep` = 8) or an arbitrary delay |
| `CSS` / `CSSHandler` | inline or serve `motion.css` |
| `Script` / `Handler` | inline or serve the optional `motion.js` (`AppKitMotion.flush/enter/toggle/frame/reduced`) |

**Both states must be legible without motion.** The escape is structural rather
than advisory: every `transition-*` declaration is scoped to `.ak-motion`, and
`.ak-motion` is the exact selector the reduced-motion block cancels — including
transitions the consumer wrote inside it. `motion_test.go` fails the build if a
future edit breaks that. `MaxStep = 8` makes an unbounded stagger
*inexpressible* rather than clamped.

## `appkit/ui/vizframe` — the provenance frame (L3b)

A chart is a claim, and an unsourced claim is worth less than no claim. The
frame carries a mandatory source line; `Render` has no code path that emits a
figure without saying where it came from. Adopted by
[ADR-0017](adr/0017-multi-target-layered-kit.md) §6 as a **sibling** package —
it imports nothing from `ui/viz`, `ui/viz` imports nothing from it, and zero of
`viz`'s call sites changed.

| Piece | Use it for |
|---|---|
| `Options.Validate() error` | inspect — wire the predicate into your own test or lint |
| `Render(body, o) string` | guard — returns `""` rather than render a figure that cannot say where it came from |
| `MustRender(body, o) string` | panic — dev builds and golden tests |
| `Published` / `Derived` / `Estimate` | the three (and only three) ways a number can be believed; `.Linked(href)`, `.Drafted()` |
| `CarriesMechanism` | the waiver, for a diagram that states no quantity — visible in a diff, in a grep, and in the DOM as `data-viz-carries="mechanism"` |

Silence is strict: `Options{}` is `CarriesFigure` and refuses. The kind labels
are deliberately not caller-overridable — an app that could relabel `Estimate`
as `Verified` would make the rule decorative. The source line uses `ink-muted`
and never `ink-faint` (hazard H-004: a caveat set quieter than the claim it
qualifies is dishonest), asserted by `TestSourceLineNeverUsesInkFaint`.

The trusted body slot does not choose a layout or resize its SVG. In a grid, let
cells shrink (`min-w-0`) and wrap a wide body in a named, focus-visible native scroll
region; keep the caption and source outside that region. Give code examples their
own keyboard-scrollable viewport and wrap long inline identifiers. Gallery's
[`/provenance` repair](assessments/2026-09-22-provenance-reflow.md) demonstrates this
composition without changing the primitive. Its values are synthetic: an illustrative
publication link is not evidence for made-up data.

## `appkit/ui/tokens` — the design tokens

The CUE-authored token pipeline: color roles × light/dark, radius, fonts.
`appkit gen` emits `theme.css` (`@theme` + runtime scheme vars), DTCG JSON, and
Go role-name constants (`ColorNames`, `RadiusNames`, `FontNames`).

---

See the **Gallery** (`apps/gallery`, `task gallery:run`) for every component
rendered live, and `cmd/appkit-mcp`'s `get_kit_css_setup` for wiring the CSS in
a consuming app.
