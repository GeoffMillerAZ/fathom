# App-kit adoption: evidence workspace

Consumer [intent](intent/README.md), [product context](../PRODUCT.md) and the
[UI doctrine](../cmd/fathom/CLAUDE.md) govern this selection. It executes existing
architecture; no metric model, daemon behavior or future has been promoted.

## Selected surface and design

- `go-web`, layers L1/L3b/L4, selecting `shell.layout` and experimental
  `recipe.styleguide.local`. Existing stat/viz
  integrations remain; this manifest is a bounded selection, not every kit import.
- Operational readers need to distinguish wired, used and paying off, then reach
  the evidence/remedy. Preserve current role-token identity in both light/dark themes.
- The shared MatDash-derived `dashboard.RenderShell` supplies the sidebar, top bar,
  mobile disclosure and landmarks. App-owned routes, leads, explainers and evidence
  remain inside it. No paid source or browser runtime was copied; the guide's
  reviewed module changes are disclosed below.
- Missing/unreadable is not zero; instruments, denominators and time windows remain
  explicit. Navigation composition must not change metrics, narratives or write policy.
- Existing kit callouts name unavailable configuration and absent comparable exploration;
  the latter links to session evidence without implying zero cost or savings. The service
  share uses existing `vizframe` for wrapping prose, denominator and snapshot provenance,
  not a long SVG caption or another nested widget. Measured derivations are unchanged.
- Fleet/impact/story/home use one app-owned observation projection: none known is absent,
  partial coverage is a named lower bound, complete coverage permits measured zero.
  Registry failures cannot turn a listed subtotal into a fleet total. Story separates
  the current backlog snapshot from weekly session metrics; home preserves unavailable
  call-log and verify-catch evidence. No observation reader or wire model changed.
- Compose the fleet as three indexing metrics plus two evidence-status metrics; the story's
  snapshot has its own band. Keep the lower-bound sign with its number and give service
  detail links a keyboard outline and 44px target. Do not patch the mirror in place.

## Mirror d9dd789: the console's kit contributions land (2026-09-25)

The mirror moves from `dc77e66` to app-kit main `d9dd78919099fccc3e3b70b06eeefff8bff5b72e`
(`scripts/sync-vendor.sh app-kit` pointed at a clone of the kit at that commit; the 116
changed kit files match `git diff --name-status dc77e66 d9dd789` one for one). The 26 kit
commits in between are Fathom's own vision-35 contributions and wave F0's returns. The
mirror's `go.mod`/`go.sum`, `cue/tokens/kit.cue` (so `guide-baseline.cue` and
`theme.css` stay byte-identical) and the dashboard, stat, vizframe and styleguide files
the manifest pins are unchanged; `shell.layout` and `recipe.styleguide.local` keep their
metadata pins. Existing call sites are unaffected: every new viz option is opt-in and
the table and breadcrumb changes are additive markup.

**F0 stopgaps removed** now that the kit carries each fix:

- `renderKitTable` (`console_panel.go`) only renders the templ component; the kit writes
  `scope="col"` itself (FATHOM-F0-KIT-01).
- `/statusz` drops its `relative overflow-hidden` wrapper: the kit's scroll wrapper is
  positioned (FATHOM-F0-KIT-02).
- `renderConsoleHeading` drops its `[&_a]` breadcrumb wrapper: the kit's trail links are
  44 px with a focus-visible outline (FATHOM-F0-KIT-03).
- The fixed-width charts take the kit's opt-in `Fluid` drawing (FATHOM-F0-KIT-04): the
  fleet traffic sparkline (its `[&>svg]:max-w-full` scaling hack is gone) and each service
  tile's sparkline fill their cards. Fluid scales text with the drawing (the kit's stated
  limit), so the text-dense tool-load bars are drawn twice, 560 px from `md` up (capped at
  that native width) and 320 px below it, one displayed at a time; the 900 px session
  timeline is Fluid from `md` up and keeps its native width in its own scroll region below.
  `chrome.RenderPagination`'s ~28 px links and `hitl.DiffView`'s ink-faint toggle were not
  changed by the kit, so `/search`'s pagination wrapper stays.

**Not adopted, with the reason:** `chrome.RenderPageHeading` (FATHOM-KIT-07) writes the
page's `h1`, and `dashboard.RenderShell`'s top bar already writes one, so composing both
would give every page two `h1`s. `renderConsoleHeading` stays app-local until the shell
can omit its top-bar title (returned as FATHOM-S0-KIT-01 below). `analytics.graph`,
`viz.heatmap` truth states, `content.code-excerpt` and `work.unified-diff` wait for the
pages that need them (waves C–E); none is imported yet, so none is scanned.

## Console wave G: the Learn pages (2026-09-25)

OC-46..OC-48 (`cmd/fathom/console_learn*.go`, projections `internal/console/learn.go`,
`ask.go`, `guides.go`) adopt two more kit pieces at mirror `d9dd789`; no mirror file
changed:

- `viz.LayoutGraph` (FATHOM-KIT-02, `analytics.graph`) draws `/learn`'s pipeline: given
  ranks (one column per stage, the stage name as the column heading), the stage's live
  count and unit in its box, each box linking to the stage's page. The SVG sits in a
  keyboard-scrollable region (it is ~1,030 px wide; it fits at 1440 and scrolls at 390)
  and the kit's own `Table()` alternative sits behind `chrome.RenderDisclosure`, its
  links given 44 px targets by the wrapper, as the kit asks.
- `docs` (`content.docs`): the guides render as `doc-prose` with `docs.RenderToCRail` from
  `xl` up (placed first in the document so the outline precedes the prose for keyboard
  and screen-reader readers) and `docs.RenderToCPanel` below it. `cmd/fathom/ui/input.css`
  imports `appkit/ui/docs/prose.css` (L1) and scans `appkit/ui/docs/*.go`;
  `scripts/ui-css.sh`'s audited closure lists both, and `task ui:css` regenerated
  `app.css`.
- Also composed: `form.RenderSelect`/`RenderField`/`RenderButton` (the project picker, a
  GET form), `form.RenderInput` (the try-it input), `chrome.RenderDescriptionList`,
  `table.Table` and `renderPanel` throughout.

Returned: FATHOM-G-KIT-01 (graph node truth states) and FATHOM-G-KIT-02 (the ToC panel
always open) as candidates, and FATHOM-G-KIT-03 as the `docs` package's second-consumer
evidence (records below). The browser run is
`docs/bench/readings/console-g-proof-2026-09-25.md`.

The wave's fix pass (same day) found the rail never stuck and its entries were 20 px
targets: the dashboard shell's `<main>` is a scroll container that never scrolls. Fathom
works around it without touching the mirror — a page that positions something sticky
carries `data-console-sticky`, and the console's `<body>` frees `main` for it
(`overflow-x: clip; overflow-y: visible`); the guide wraps the related links and
`docs.RenderToCRail` in one bounded sticky block with 44 px entries — and returns it as
FATHOM-G-KIT-04. The pipeline's graph nodes now carry the stage's name with the count
(`Source: 2,093`), since the kit's `Table()` names each node and each hand-off by its
label alone.

## Console stage 0: the ⌘K palette and live freshness (2026-09-25)

OC-12 adopts `interaction.command-palette` (FATHOM-KIT-01) at mirror `d9dd789`, pinned in
`appkit.adoption.json` (layer L2 added for its script; `palette.go` and `palette.js`
pinned as artifacts). `cmd/fathom/console_palette.go` configures one palette
(`console-palette`, Action `/search`, ResultsURL `/search?format=fragment`, Mod+K):

- The shell's top bar carries `palette.RenderTrigger` in place of the F0 search field;
  every page (error pages included) renders `palette.RenderDialog` once and loads the
  kit script from `/assets/palette.js` (`palette.Handler`). The dialog's title is an
  `h2`, so each page keeps one `h1`.
- `GET /search` answers the palette's fragment for `?format=fragment` or the script's
  `X-AppKit-Palette` header with `palette.RenderResults` (`Vary: X-AppKit-Palette`): the
  same `console.SearchWith` groups and order as the page and the JSON; a capped group
  names its exact total and ends with a "Show all N" link to its group page; a result with
  no page of its own (a verb) opens its search page; index notes become the kit's partial
  notice; a query past the bounds gets the kit's error state naming the bound.
- Without JavaScript the trigger is a link to `/search`, whose page now leads with
  `palette.RenderSearchForm`. The full page keeps fathom's richer rendering (verb chips,
  group pages, pagination) over the same results.

OC-14 adopts `interaction.live-freshness` (FATHOM-KIT-06), pinned likewise (`live.go`,
`live.js`). `cmd/fathom/console_live.go` wires the kit's helper to `GET /events`:

- Every `renderPanel` panel names the sources it follows twice on one element:
  fathom's `data-console-source` and the kit's region attributes (`live.RegionAttrs`,
  `data-live-source`, focusable, with a focus-visible ring), plus the kit's reserved
  status line (`live.RenderStatus`), which now carries the panel's freshness instead of
  the footer. A page whose body is not yet built from panels (the pages F0 moved over
  unchanged) is one region, `page-live`, following the sources its model reads
  (`consolePage.Live`; the project page fills in its own service), status line first.
- A page with regions renders `live.RenderPageStatus` under its heading, with the
  stream position it was rendered at (`EventBus.Cursor`), and loads `/assets/live.js`
  (`live.Handler`). The kit script follows the five event kinds and `gap`.
- The bus side (OC-13, additive): each event also names a fleet-wide key (`index`,
  `layers`, `plans`), so a panel summarizing every service follows one key instead of one
  per service (18 services exceed the kit's 16-source bound); and a reconnect whose cursor
  the replay cannot cover (another process's id, or older than the 64-event ring) is sent
  a `gap` event naming `*` first, the kit's contract for a server that cannot replay.

`cmd/fathom/ui/input.css` and `scripts/ui-css.sh` add `overlay/*.go`, `palette/*.go` and
`live/*.go`. Tests: `TestConsolePalette_OverEveryPageWithFragmentAndNoJSFallback`,
`TestConsolePalette_ResultsNameTotalsAndRealDestinations`,
`TestConsoleLive_PanelsCarrySourcesAndMarkWithoutMoving` and
`TestEventBus_GapNamesEverySource`. The keyboard and connection contracts are the kit's
(its script tests and Gallery runs); fathom's browser run is in
`docs/bench/readings/console-stage0-proof-2026-09-25.md`.

## Observatory console wave F0 (2026-09-25)

Vision 35 / ADR-045 moves every Observatory page onto one console shell
(`cmd/fathom/console_*.go`, plan `docs/plans/observatory-console.md` section A) on the
same `dc77e66` mirror; no mirror file changed. Packages the console now uses, beyond the
existing dashboard/stat/viz/vizframe/callout/form/hitl set:

- `dashboard.RenderNav` with app-owned `NavGroup`s (Now, Fleet, Proof, Knowledge,
  Evidence, System, Learn — a group appears once a page registers in it; no placeholder
  destinations) inside `RenderShell` with `SkipLink` and one `ContentID`.
- `appearance`: `BootstrapScript({StorageKey: "fathom-theme"})` inline before the
  stylesheet, and `RenderControl` beneath the sidebar groups (also in the mobile
  disclosure; the control's wrapping label needs no unique IDs). Browser-checked:
  selecting Dark sets `data-theme="dark"`, saves `fathom-theme`, and the token ground
  switches.
- `chrome.Breadcrumbs` (the trail under the top bar) and `chrome.RenderDescriptionList`
  (the statusz daemon panel).
- `table.Table` (statusz services, row drill-down to `/projects/{svc}`), which makes
  `github.com/a-h/templ` a direct root requirement at the mirror's own pin, v0.3.1020.
- `form.RenderInput` for the top bar's `GET /search` field; `form.RenderButton` (ghost,
  link mode) for the brief's "Download markdown" action and `/search`'s "Show all"
  links; `chrome.RenderEmptyState` for `/search`'s no-query and no-result states;
  `chrome.RenderPagination` for a result group's own page.

`cmd/fathom/ui/input.css` scans `../console_*.go` by glob and adds the `appearance`,
`chrome` and `table` packages; `scripts/ui-css.sh`'s audited closure matches it and
`task ui:css` regenerated `app.css`.

**Built locally as the originating consumer (vision 35 §6), to return:**

- FATHOM-KIT-07 page heading: `renderConsoleHeading` composes the trail, the question,
  a meta line and verb chips/actions under the kit top bar's `h1` from role utilities.
  It stays app-local until the kit accepts a heading slot.
- The verb chip (`renderVerbChip`): selectable command text with a copy button that
  `cmd/fathom/ui/console.js` reveals only when the Clipboard API exists — the no-JS
  reading is the plain command.

**Kit defects observed (return with fixtures; local workarounds named).** Returned as FATHOM-F0-KIT-01..04 and fixed in the kit at `c2996cc`, `4b7182c` and `75c9d0b`; the workarounds are gone since mirror `d9dd789` (above), except pagination and the DiffView toggle, which the kit left for later:

- `table.Table` renders `<th>` without `scope="col"`, and its `sr-only` "Actions"
  header is absolutely positioned inside an `overflow-x-auto` wrapper that is not a
  containing block — at 390 px the hidden label sits at the table's right edge and
  widens the whole document (measured: 861 px scroll width on `/statusz`). Workarounds:
  `console_statusz.go` wraps the table in `relative overflow-hidden`, and every kit table
  renders through `renderKitTable` (`console_panel.go`), which adds `scope="col"` to the
  template's header cells (its only `<th>` elements, all in `<thead>`). The kit fix is a
  `relative` scroll wrapper plus `scope="col"`.
- `chrome.Breadcrumbs` links have no focus-visible treatment and no 44 px target
  (measured 17 px). Workaround: `renderConsoleHeading` wraps the trail in
  `[&_a]:inline-flex [&_a]:min-h-11` with a `focus-visible` outline.
- `chrome.RenderPagination` links are `px-2 py-1` (about 28 px). Workaround: `/search`'s
  group pages wrap it in `[&_a]:min-h-11 [&_span]:min-h-11`.
- `viz.Sparkline`/`viz.Bars`/`viz.Timeline` take a fixed pixel width. Scaling the SVG
  down (`[&>svg]:max-w-full`) keeps `main` from scrolling but shrinks text: the 560 px
  tool-load bars drew 11 px labels at ~6 px on a 390 px screen. The traffic sparkline
  (no text) keeps the scaling; the tool bars and the 900 px session timeline now scroll
  inside their own keyboard-reachable `overflow-x-auto` region, and the tool bars carry
  a `table.Table` alternative behind `chrome.RenderDisclosure`. The kit fix is a
  width-responsive chart or a built-in scroll region plus table alternative.
- `hitl.DiffView`'s "Show both versions" toggle is `text-ink-faint` (measured 3.15:1
  light, 2.88:1 dark; `docs/theming-hazards.md` H6 puts ink-faint below the text floor).
  No local workaround — the toggle is kit markup; return it as `text-ink-muted`.
- Fathom's own absence notes had the same H6 defect (`metricCell`, session and timeline
  notes, the not-recorded column): they now use `text-ink-muted`, as `stat.Absent`
  does for its reason; only a bare decorative "—" stays ink-faint.

**Waiting on contributions (then):** the ⌘K palette dialog (OC-12, FATHOM-KIT-01 over
`overlay`) and the live "updated — refresh this panel" affordance (OC-14,
FATHOM-KIT-06). Their server sides shipped in F0; both landed in the kit and are
integrated in stage 0 (above).

**Proof (2026-09-25):** `go test -race` over `internal/console`, `cmd/fathom`,
`internal/mcp`, `internal/metrics`, `internal/scope`; `task ui:css`,
`scripts/ui-css_test.sh` 12/12 and `scripts/ui-css.sh --check`; a scratch `serve-http`
over a `.backup` copy of the fleet store (18 services) rendered every page — warm renders
0.3–34 ms, HTML 21–83 KB — with headless Chrome screenshots in both themes at 1440 px
and a 390 px frame, and an in-browser 390 px layout probe reporting no document-level
horizontal overflow on any page. This is not UX-M2's certification: keyboard paths and
contrast were spot-checked, not measured page by page.

## Observatory console wave B: Now and the fleet (2026-09-25)

Wave B (plan section B, OC-17..OC-20) rebuilds `/` and `/fleet` on the `d9dd789`
mirror; no mirror file changed. New kit package in the console: `topology.Render`
(the fleet's health rollup, groups collapsed, each group's name a filter link),
which joins `cmd/fathom/ui/input.css` and `scripts/ui-css.sh`'s audited closure.
Composed from what the console already used: `stat.RenderBand` (both pages' bands;
Now's keeps two columns below `md`, as the phone mockup does), `renderPanel`
(every panel names its verb and sources), `table.Table` for the pulse, the fleet
and every sparkline's per-day table alternative, `chrome.RenderDisclosure` (the
rest of the queue, earlier changes, the table alternatives, the machine surfaces),
`chrome.RenderEmptyState`, `chrome.RenderPagination`, `callout.Badge` (areas and
health), `form.RenderSelect`/`RenderInput`/`RenderField`/`RenderButton` (the fleet's
GET filter form).

**Built locally as the originating consumer, to return (candidates below):** the
shared-scale traffic sparkline (FATHOM-B-KIT-01), link-mode sortable table headers
(FATHOM-B-KIT-03) and the one-rendering card table (FATHOM-B-KIT-04). The rung
pips (the realization ladder as five labelled marks) and the attention list stay
fathom's own domain views (design §5).

## Observatory console wave C — the Proof pages (2026-09-25)

Wave C (`/plans`, `/plans/{svc}/{plan}`, `/plans/{svc}/{plan}/rows/{id}`,
`/certifications`, `/refactors`, the project overview's ladder; OC-21..OC-26) stays on
mirror `d9dd789`; no mirror file changed. Packages it adds to the console:

- `kanban.Board` (with `card`) for a plan's rows by state — read-only through
  `readOnlyBoard` (FATHOM-C-KIT-01 below); each lane shows its first 12 cards and the
  table beside it lists every row; an empty lane is named under the board rather than
  drawn (claimed-not-proven is always drawn).
- `overlay.RenderDrawer` for a row's evidence over the board: the card link fetches the
  row page's `?format=fragment` into the drawer (a small inline script, `showModal`,
  focus back to the card on close); without JavaScript the card is a link to the page.
- `code.RenderExcerpt` (FATHOM-KIT-04) for the line a lexical oracle matched
  (`LineHighlighted`, labelled "First match") and a partial function's unexecuted changed
  lines on `/refactors` (`LineNotExecuted`, labelled "Changed, never ran").
- `viz.Heatmap` truth states (FATHOM-KIT-03) for the certifications matrix: a zero count
  is `CellZero`, drawn as a zero, with the matrix as a `table.Table` behind
  `chrome.RenderDisclosure`. The coverage matrix (OC-28) is still to come.
- `viz.StackBar` (fluid) for proof bars and, one per row on a shared scale, for the
  burn-up and the ledger trends — the chart is its own table (FATHOM-C-KIT-02).
- `stat.Absent` for the trend until the verify ledger holds 14 days of rows (SD-3),
  `chrome.RenderPagination`, `chrome.RenderEmptyState`, `chrome.RenderDescriptionList`,
  and `form.RenderSelect`/`RenderInput`/`RenderButton` for `/refactors`' GET chooser.

`cmd/fathom/ui/input.css` and `scripts/ui-css.sh` add `card`, `code` and `kanban`.
The repository filter on `/plans` and `/certifications` is a wrapping list of links
composed from role utilities (44 px targets, `aria-current`) rather than `chrome.Tabs`,
which does not wrap (14 repositories overflow a 390 px screen) and marks links as
`role="tab"` without panels (FATHOM-C-KIT-03).

## Observatory console wave F (2026-09-26)

Wave F (`/impact`, `/sessions`, `/sessions/{id}`, `/operate`, `/decide`; OC-41..OC-45)
uses the `d9dd789` mirror unchanged and no new kit package, so `input.css` and
`scripts/ui-css.sh` are untouched. Newly adopted parts of packages already scanned:
`viz.RangeBars` (the counterfactual arms, a wide and a narrow drawing), `viz.Timeline`
swimlanes and its `ViewDensity` strip past 150 catches, `viz.Heatmap` with truth states
(`CellMissing` for a day with no call-log rows, `CellZero` for a silent surface),
`viz.StackBar` (a tool's calls by caller class), `viz.Bars` (the calibration buckets),
`chrome.RenderDisclosure`/`RenderPagination`/`RenderEmptyState`/`RenderDescriptionList`,
`form.RenderField`/`RenderSelect`/`RenderInput`/`RenderButton` (GET filter forms), and
`table.Table` with `HeaderHTML` sort links. Candidates to return:

- **FATHOM-F-KIT-01 (candidate):** the heatmap's truth-state legend is sized to the grid
  and truncates a reason with "…"; Fathom keeps reasons to a few words and puts the full
  reason in the table alternative. Expected: a legend that wraps, or its own width.
- **FATHOM-F-KIT-02 (candidate):** `table.Table`'s `Sortable` header is a Datastar
  button; a server-rendered table needs a link, and `aria-sort` belongs on the `<th>`,
  which `HeaderHTML` cannot reach. Fathom writes the direction as `sr-only` text.
- `chrome.RenderPagination` links are still about 28 px (F0's open item): wave F wraps
  them in `[&_a]:inline-flex [&_a]:min-h-11 [&_a]:items-center`.
- **FATHOM-F-KIT-03 (candidate, from the wave F fix round):** `table.Table` has no
  phone layout. At 390 px its scroll wrapper keeps the page from overflowing, but the
  columns that carry a table's answer (an outcome, p95, a ruling's choice) sit off-screen
  with no cue. Fathom restates each such table below `md` as a stacked list of
  `chrome.RenderDescriptionList` cards (`evTableOrStack`, `console_evidence.go`).
  Expected: an opt-in `table.Table` stacked mode that restates each row as terms and
  details below a breakpoint and keeps the real table from it up.
- A Tailwind class built at run time never reaches the stylesheet: wave F's chart
  wrapper wrote `min-w-[%dpx]` through `fmt.Sprintf`, Tailwind emitted that literal as an
  invalid rule, and the real class had none. The fix-round wrapper takes the class as a
  literal the caller writes (`evChart`), and `TestConsoleWaveF_OperatePanelsNameTheVerbsThatPrintThem`
  holds `app.css` to it. (Fathom-local; the kit's own classes are all literals.)

## Current mirror and palette convergence (2026-09-25)

FATHOM-GUIDE-01 landed on main at mirror `f7ab0fbd`, then took one reviewed bump to
app-kit main `dc77e660e378727cb062fd76e2a8770fd8a4540c` (`task sync-kit` archive,
file list checked against the maintainer checkout). The 71 kit commits in between touch
analytics, form, table, `internal/webref`, the adoption CLI/catalog and maintainer-only
styleguide browser tooling. Dashboard, stat, viz, vizframe, callout, hitl, tokens and the
styleguide Go API are unchanged; the mirror's `go.mod` is unchanged and root tidy is a no-op.
Breaking kit corrections, checked against fathom's call sites:

- ADR-0079 turns a form button with an unsafe or disabled `Href` into an inert label.
  Fathom's two link buttons (`/observatory/story`, `/observatory/wiring/`) are
  root-relative and pass `webref.Allowed`, so they still render anchors; the scanned
  `form/*.go` adds only the inert state's `opacity-50`/`cursor-not-allowed` utilities.
- ADR-0083 select prompt, radio feedback and settings wrap: fathom renders none of them.
- Analytics `Row.DetailHref`, `WriteTableCSV`, diverging bars and the `Window.TimeZone:
  "Local"` refusal, plus table row hrefs/action column: fathom imports neither package.

The palette divergence is resolved by adoption. `cmd/fathom/ui/guide-baseline.cue` is a
byte copy of the mirror's `cue/tokens/kit.cue`: light `positive` moves from
`oklch(0.55 0.14 155)` to the kit's contrast-safe `oklch(0.51 0.14 155)` (dark is
unchanged), and the generated `cmd/fathom/ui/tokens/theme.css` is byte-identical to the
kit's `appkit/ui/tokens/theme.css`. After a later bump, copy the mirror's `kit.cue` over
the baseline (or record a divergence here), then run `task ui:css`. The manifest pins both
files, so silent drift fails the adoption check.

`recipe.styleguide.local` is re-pinned `565b6155…` → `820a6558…`. The reviewed catalog
delta (app-kit `95151b4`) only records this consumer's return: usage cites the Fathom
receipt, gaps restate the scoped Fathom/RAC evidence, and consumers become Resume as Code
and Fathom. Maturity (experimental), layers, profiles, dependencies, source symbols,
states, app ownership and proof are unchanged; `shell.layout` stays `34006292…`.
`artifacts` no longer pins high-churn consumer files (`main.go`, `verbs.go`,
`observatory_render.go`, `go.mod`, `go.sum`), which failed the check on every unrelated
edit. It keeps the CSS/input/theme outputs, baseline plus `kit.cue`, the CSS builder and
its test, the guide source/test and the mirror anchors. `observatoryGuideRevision` names
the new mirror; `TestThemeGuideKitRevisionMatchesVendoredMirror` refuses a bump that
forgets it.

## Local theme guide integration (2026-09-22)

The owner-authorized working-tree upgrade selected the exact app-kit mirror
`f7ab0fbd3496e9dc3da8c97b6c4290f09a71ec50` (bumped since; see above). Its reviewed metadata kept
`shell.layout` unchanged and adds the experimental local-guide contract, not an
application settings backend. The root module adds only indirect uniseg v0.4.7;
the mirror also selects MCP SDK 1.8.0 and x/time v0.15.0 in the module graph.
Agentic-kit's `beb53a4fb901` pin/mirror and service JSON stay unchanged.

Build with `GOWORK=off`; run the resulting private binary's
`theme-guide -addr 127.0.0.1:18767`, then open `/dev/theme` at that origin.
This separate command does not mount routes in the daemon or read the registry,
call log or session ledger. It renders fixed complete/partial/missing/error models
through the actual Observatory renderers. Preview links are disabled explicitly.
Preview, reset and export never rewrite source or preferences; review the exported
candidate before an explicit source change. At `f7ab0fbd` the authored baseline kept the
positive `oklch(0.55 0.14 155)` palette; it now tracks the kit (above). Eight selected contrast checks
are not whole-screen accessibility certification.

Registry coverage failure now renders absence or a named lower bound. Call-log and
ledger failures independently suppress their own dependent counts, while readable
zero and unrelated valid sources remain intact (I38/ADR-027). No reader, calculation,
service-JSON field, live data, daemon restart, consumer commit or deployment changed.

Verification and current limitations are recorded in app-kit's
`docs/assessments/2026-09-22-fathom-guide-only.md`. The exact private source passes
35 top-level / 96 test-and-subtest race checks, vet, standalone build, 12 CSS
builder regressions, actual CSS build/check and the selected static adoption checks.
The current CSS-bearing binary also passes the bounded fresh-Chrome light/wide and
dark/narrow journey: preview/refusal/repair/reset plus pointer/Enter ZIP exports with
identical reviewed bytes and clean owned-process shutdown. Large-number chart geometry,
zoom, full accessibility, memory/cancellation and human usability remain open; the guide
remains experimental. Earlier failed receipts remain intact.
The existing identical CLAUDE.md/AGENTS.md routers already route here; no host settings
change is needed. **FATHOM-GUIDE-01:** adopt the shared guide, keep domain validity
repairs here, and return sanitized missingness/build-reliability tests and lessons to
app-kit. Revisit shared promotion only after independent consumer task evidence;
this integration does not close G3/G4, human usability or the memory/cancellation gates.

## Historical delivery identity and navigation handoff

`go.mod` declares app-kit `v0.0.0` with an in-tree replacement. ADR-019 requires its
exact stamped mirror: `third_party/app-kit/.vendor-commit` is now
`d9dd78919099fccc3e3b70b06eeefff8bff5b72e` (earlier `bcfeb8e`, `f7ab0fbd`, then `dc77e66`).
Local `go.work` instead resolves the sibling.
The adoption checker observes declarations/hashes, not effective Go resolution;
verify both modes. The owner authorized this scoped mirror refresh on 2026-09-14
while the other Claude Code session was paused. Its unrelated edits, agentic-kit
pin/mirror and existing token identity remain unchanged. No release or deployment.

`obsNavigation` maps the same seven app-owned destinations and active keys into
the mirrored `dashboard.RenderNav`; unknown keys select none. `Shell.SkipLink`
now targets the sole focusable `observatory-main`, replacing the app-owned skip
link/nested focus workaround. Shared links and Menu supply 44px minimum targets
and explicit keyboard focus treatment; tokens, routes and data semantics do not change.

`appkit.adoption.json` binds reviewed `shell.layout` metadata, generated CSS and
selected source bytes (21 passing static checks plus the version2 whole-catalog
not-applicable result), not a released kit version, transitive source pin, runtime
proof or the entire dependency graph. `task ui:css` now builds the CLI
inside the exact mirror with `GOWORK=off`, reads its CUE tokens, and stages explicit
Tailwind sources with `source(none)`. It needs no sibling checkout or new dependency.
`bash scripts/ui-css.sh --check` regenerates and compares both committed CSS outputs;
it does not treat embedding an old stylesheet as freshness proof.

## Verification and isolated preview

```sh
task ui:css
bash scripts/ui-css_test.sh
bash scripts/ui-css.sh --check
CGO_ENABLED=1 MACOSX_DEPLOYMENT_TARGET=14.0 go test -race ./cmd/fathom -run 'TestObservatory' -count=1
GOWORK=off CGO_ENABLED=1 MACOSX_DEPLOYMENT_TARGET=14.0 go test -race ./cmd/fathom -run 'TestObservatory' -count=1
GOWORK=off CGO_ENABLED=1 MACOSX_DEPLOYMENT_TARGET=14.0 go test -race ./cmd/fathom -run 'ThemeGuide' -count=1
(cd third_party/app-kit && GOWORK=off go run ./cmd/appkit adoption check -root ../..)
```

`TestObservatoryUsesSharedAccessibleShell` pins one main/heading/skip link/focus
target, both navigation regions and preserved content/degraded text.
`TestObservatoryNavigationRoutesAndUnknownActive` pins exact destinations and an
unknown key's absence of current-page state. Existing fleet/service/impact tests
remain required. Source-level evidence does not establish keyboard behavior.
`observatory_empty_test.go` pins unknown/known configuration, absent/measured exploration,
escaped reasons, readable share provenance and suppression when the call log is unreadable.
`observatory_observation_coverage_test.go` pins exact, absent, partial and degraded totals
across readers, bounded to the correct metric/widget rather than neighboring digits.

`cmd/fathom/appkit_preview_test.go` exports only the fixed synthetic `alpha` model.
Set `APPKIT_PREVIEW_DIR` to an existing empty child of `workspace/tmp`, then run
`go test ./cmd/fathom -run '^TestExportAppkitPreview$' -count=1` with the CGO settings
above. It renders fleet/impact/service/story in both themes, strips scripts/forms and
adds restrictive CSP. It starts no server and reads no live daemon/provider data.
Its inert transformation is not a general-purpose hostile-HTML sanitizer.
The fixed story model makes the evidence-link task inspectable without reading a ledger.

For observation triage, set `APPKIT_OBSERVATION_PREVIEW_DIR` to a different existing empty
`workspace/tmp` child and run `TestExportAppkitObservationPreview`. Its ten fixed cases
cover unknown/partial/zero/positive observations, unavailable/partial registry,
unavailable call logs and measured zero/positive catches. It exports home/fleet/impact/story plus alpha/beta/gamma detail
only where rows exist. No missing service is replaced with a fabricated service page.
Use the app-kit fixture `docs/assessments/fixtures/fathom-observations/browser.mjs` with
the export path, a separate empty report path and an isolated Playwright module path.
The server exposes only these fixed GET/HEAD documents and CSS, never operational routes.

Browser and delivery receipt: app-kit
`docs/assessments/2026-09-13-fathom-observatory-uplift.md`. Do not infer live route,
mutation, accessibility certification or installed-daemon proof from those fixtures.
Observation-triage follow-up: `docs/assessments/2026-09-14-fathom-observation-triage.md`
in app-kit records the synthetic browser matrix, baselines, budgets and remaining gaps.

Evidence-journey follow-up: app-kit `docs/assessments/2026-09-14-fathom-evidence-journey.md`
records the ten-scenario matrix and INC-19's missing-cost repair. Home/evidence actions
compose the pinned `form.RenderButton` as native anchors; a direct-anchor 44px rule
belongs to this app's composition and is included by the existing CSS source globs.
Failed call-log reads invalidate dependent cost/outcome values, not independently
measured schema tax. The subsequent owner-authorized stat/emitter upgrade now uses
the exact `bcfeb8e` mirror, not a synthetic Go overlay. It preserves this app's
positive0.55 palette and runtime font/radius aliases while correcting metric reflow
and multi-row dividers. The independent positive0.51 kit change was not included then;
the 2026-09-25 bump adopts it.
Actual-mirror exports pass 6,467 metric-layout, 5,560 evidence-journey and 1,207
navigation browser checks; tablet-light and narrow-dark screenshots were inspected.
These are synthetic local Chrome tasks, not real data, full accessibility or deployment.
Kit assessment `docs/assessments/2026-09-14-stat-reflow.md` owns the upgrade receipt,
including source-build checks and known limitations. Catalog `2a98eb25…`, module
declarations and agentic-kit's mirror remain unchanged; the newer catalog needs its
own review. All paused non-UI work remains independently owned and uncommitted.

Navigation handoff: set `APPKIT_NAVIGATION_PREVIEW_DIR` to another empty
`workspace/tmp` child and run `TestExportAppkitNavigationPreview`. It exports the real
shell for all seven active keys in both themes with an inert synthetic body, not
operational route handlers. App-kit's `docs/assessments/2026-09-14-navigation-delivery.md`
records scoped mirror, standalone-build and browser evidence and their limits.
The earlier navigation handoff passed 1,207 synthetic browser checks, workspace and snapshot
race tests, scoped vet, both mirror checks and repeated identical standalone builds.
Local evidence and the pre-change backup are under `workspace/tmp/appkit-navigation.1DsKeZ/`.

Before a consumer commit, include the new mirrored `dashboard/nav.go` and its tests;
the tracked-files-only clean-clone gate currently excludes them and fails. Keep
unrelated paused work separately reviewed. The earlier navigation catalog selection was
reconciled against an exact pinned checker (`52185ef4…`): all 16 checks pass for
`2a98eb25…`. Only `catalog_revision` changed; profile, layers, capability, artifacts,
module and routers did not. The old `0b528f5b…` catalog bytes could not be recovered,
so this is a prospective review of the current `shell.layout` contract against the
actual integration and its evidence, not an asserted old-to-new catalog comparison.
The checker/schema remain strict and unchanged. App-kit retains the checker, catalog,
1,248 compile-input hashes and before/after results in
`data/design-assets/adoption-checker.DJ6MuD/`; its navigation receipt records identities.
The stat upgrade adds five selected stat/emitter/CUE/stamp/build artifacts while keeping
that catalog/profile/layer selection. Coverage and series test helpers now bind the
actual component DOM, preserving exact value/reason assertions and rejecting decoys;
the complete command-package race suite passes. No production metric code changed.
Two sibling-free working-tree snapshots also regenerate identical CSS, build all Go
packages and yield byte-identical CLI binaries; this is not a clean-Git release proof.
No gate was waived.

## Selected metadata and reusable contributions (2026-09-16)

The owner-authorized version2 migration replaces the whole-catalog revision with the
reviewed `shell.layout` metadata pin. The current record still fits `go-web`/L3b and
the real responsive shell integration; it preserves app ownership of routes, current
path, permitted destinations, identity, authorization and content. Profile, layers,
capability, artifacts, module, contract and routers are otherwise unchanged. The old
`2a98eb25…dc43c` selected record differs from the current record by the already-reviewed
task phrase “mobile application shell navigation with current route and grouped links”;
the migration is prospective acceptance of the current complete record, not a claim
that old and new metadata bytes match.

Keep Fathom measurements, readers, evidence semantics, routes and daemon policy here.
Send a shared renderer defect to app-kit immediately with a failing sanitized fixture
or minimal regression. Propose repeated workarounds and reusable lessons upstream with
the originating task and evidence; incubate a one-app component locally until actual
consumer use establishes a durable API boundary. Record a local or deferred disposition
with its reason and concrete revisit condition. Returning a fix/test/lesson to the kit
is separate from adopting it here: Fathom integration needs a reviewed versioned
upgrade and proof of the original task. Second-consumer proof is required for promotion
only where the layer contract requires it. Retrieve app-kit MCP
`get_kit_decision("contribution-loop")` or a reviewed documentation snapshot; the
`dc77e66` mirror carries `docs/design/contribution-loop.md`. If unavailable, retain
the local candidate and mark canonical review pending rather than inventing policy.

### Contribution records

**FATHOM-KIT-01..08 — the console's just-in-time kit pieces (vision 35 §6, ADR-045)**

Fathom is the originating consumer of each; each stays experimental in the kit until a
second consumer integrates it (ADR-0066). Delivery is the app-kit main commit, all
inside mirror `d9dd789`:

| ID | Kit capability | Delivered at | Fathom integration |
|---|---|---|---|
| FATHOM-KIT-01 | `interaction.command-palette` (`appkit/ui/palette`) | `d52acea`, fixes `843a669` `77b25b9`, merge `9dc6f5d` | OC-12, `cmd/fathom/console_palette.go` |
| FATHOM-KIT-02 | `analytics.graph` (`viz.LayoutGraph`, `viz.LayoutEgo`) | `dc8f035`, fix `209249f`, merge `6ed6ea5` | OC-37, the code neighborhood (`cmd/fathom/console_project_index.go`, `LayoutEgo` + `Table()`, wave E); OC-46, the Learn pipeline (`cmd/fathom/console_learn_pipeline.go`, `LayoutGraph`, wave G) |
| FATHOM-KIT-03 | `viz.heatmap` truth states | `0f45523`, fix `8f663ab`, merge `b861d49` | the certifications matrix (OC-24, `console_certify.go`, wave C); the `/layers` coverage matrix (OC-28, `cmd/fathom/console_layers.go`, wave D) |
| FATHOM-KIT-04 | `content.code-excerpt` (`code.RenderExcerpt`) | `94e6815`, fix `5bf90cd`, merge `fd47ef0` | the refactor page and a row's oracle match (OC-26, OC-23, wave C); the `/claims/{id}` citation (OC-30, `cmd/fathom/console_claim.go`, wave D) |
| FATHOM-KIT-05 | `work.unified-diff` (`hitl.RenderUnifiedDiff`) | `94e6815`, fix `5bf90cd`, merge `fd47ef0` | pending: review surfaces |
| FATHOM-KIT-06 | `interaction.live-freshness` (`appkit/ui/live`) | `e7d66c9`, fix `f965851`, merge `54f5fac` | OC-14, `cmd/fathom/console_live.go` |
| FATHOM-KIT-07 | `interaction.page-heading` (`chrome.RenderPageHeading`) | `e7d66c9`, fix `f965851`, merge `54f5fac` | not adopted: the shell's top bar already writes the `h1` (FATHOM-S0-KIT-01) |
| FATHOM-KIT-08 | catalog records for shipped, uncatalogued packages | `a80d967`, merge `b861d49` | none needed (catalog metadata) |

- **Evidence:** the kit's receipts `docs/assessments/2026-09-25-command-palette.md`,
  `-layered-graph.md`, `-heatmap-truth-states.md`, `-code-review-renderers.md` and
  `-live-freshness-and-page-heading.md` in the mirror.
- **Disposition:** `delivered` in the kit; Fathom adoption per row above. Next action:
  Fathom maintainer integrates each piece with the wave whose page needs it.

**FATHOM-F0-KIT-01..04 — wave F0's four kit defects**

- **Origin / task:** console wave F0 (`/statusz` services table, the page heading's trail,
  the fleet and session charts at 390 px); observed in `docs/bench/readings/console-f0-proof-2026-09-25.md`.
- **Observed / expected:** 01 `table.Table` header cells without `scope`; 02 its sr-only
  "Actions" label widened the document at 390 px (861 px measured); 03 breadcrumb links
  17 px with no focus-visible treatment; 04 every viz chart fixed-width.
- **Evidence:** kit receipt `docs/assessments/2026-09-25-fathom-f0-kit-returns.md`
  (Gallery run before/after, `TestFathomF0KitReturnsAreServed`).
- **Disposition:** `returned`, fixed at `c2996cc` (01/02), `4b7182c` (03), `75c9d0b` (04,
  opt-in `Fluid`, which scales text rather than reflowing it).
- **Delivery / verification:** adopted at mirror `d9dd789`; the local stopgaps are
  removed and `TestConsoleA11y_ScopeTargetsChartsAndOverflow` refuses their return.

**FATHOM-S0-KIT-01 — the shell and the page heading cannot compose (candidate)**

- **Origin / task:** console stage 0 adopting FATHOM-KIT-07.
- **Observed / expected:** `dashboard.RenderShell` always writes its `Title` as the top
  bar's `h1`, and `chrome.RenderPageHeading` writes the page's `h1`; a page using both has
  two. Below `md` the top bar also shows no brand (the brand lives in the hidden sidebar),
  so Fathom adds a `md:hidden` brand link to its `Actions` slot.
- **Disposition:** `candidate` — a shell option to render the title as non-heading text
  (or omit it) and to show the brand in the mobile top bar. Next action: app-kit
  maintainer; revisit when a second consumer composes the page heading inside the shell.

**FATHOM-S0-KIT-02 — the overlay dialog's scrim is a palette color (candidate)**

- **Origin / task:** console stage 0 composing `palette.RenderDialog` (over
  `overlay.RenderDialog`) on every page.
- **Observed / expected:** the dialog's class list carries `backdrop:bg-black/40`, the
  one palette-color utility in the console's rendered markup; Fathom's doctrine and its
  `assertRoleUtilitiesOnly` check expect role utilities only. Expected: a scrim role
  token (or a documented exception in the kit's theming hazards). The dialog's close
  button is also a 24 px target (already listed in the kit's palette record).
- **Disposition:** `candidate`, not patched locally; the check names the kit-owned
  utility as its one exception. Next action: app-kit maintainer.

**FATHOM-S0-KIT-03 — `table.Table`'s "Open →" action wraps its arrow (candidate)**

- **Origin / task:** `/statusz`'s services table at 1440 px (F0 proof, item 5).
- **Observed / expected:** in a squeezed action column the row link's text wraps, the
  arrow on its own line; expected `whitespace-nowrap` on the action. Fathom now makes the
  service name the row's drill-down link and renders no action column, which also frees a
  column's width at 390 px.
- **Disposition:** `candidate`. Next action: app-kit maintainer.

**FATHOM-B-KIT-01 — a sparkline on a shared scale that never draws across a gap (candidate)**

- **Origin / task:** console wave B, the fleet's calls per day (OC-20: sparklines across
  projects share one scale and never draw across an absent day).
- **Observed / expected:** `viz.Sparkline` scales every call to its own min and max and
  drops a `NaN` point by joining its neighbours, so rows cannot share a scale (13 calls
  looked like 22k, the survey's defect) and a day the instrument never observed is
  drawn through. Expected: `SparklineOptions` with an explicit value domain (`Min`,
  `Max`) and gap segments (a missing point splits the line and is drawn as a gap).
- **Local build:** `sharedSparkline` (`cmd/fathom/console_fleetviz.go`) keeps viz's L3a
  discipline — no class attribute, role colors as `var(--ak-color-*)`, `role="img"`
  with a title — and every chart has its per-day table.
- **Disposition:** `candidate`. Next action: app-kit maintainer.

**FATHOM-B-KIT-02 — `topology.Render`'s links and summaries are text-sized (candidate)**

- **Origin / task:** the fleet's health rollup.
- **Observed / expected:** drill-through links and group summaries have no 44 px target
  or focus-visible outline, the console's floor. Fathom's wrapper adds both with
  descendant variants (`[&_a]:min-h-11`, `[&_summary]:min-h-11`, focus outlines).
  Expected: the kit's own `min-h-11` and focus treatment, as `chrome.Breadcrumbs` got in
  FATHOM-F0-KIT-03.
- **Disposition:** `candidate`. Next action: app-kit maintainer.

**FATHOM-B-KIT-03 — `table.Table`'s sortable header is a script-driven button (candidate)**

- **Origin / task:** the fleet table's sort (design §2: filters and sort are URL state;
  pages work without JavaScript).
- **Observed / expected:** a `Sortable` column renders `<button data-table-sort>` for a
  Datastar handler, and the header cell carries no `aria-sort`. Fathom renders each
  sortable header through `HeaderHTML` as a link to the sorted route that names the
  current direction in words (`sortHeader`). Expected: a link mode (`SortHref`) and
  `aria-sort` on the active `<th>`.
- **Disposition:** `candidate`. Next action: app-kit maintainer.

**FATHOM-B-KIT-04 — a table that reads as cards below a breakpoint, from one rendering (candidate)**

- **Origin / task:** the fleet table and Now's pulse at 390 px; wave F0's `/statusz`
  rendered its rows twice (a table and a stacked list), which at the fleet's size
  pushed the page past half its 250 KB budget.
- **Observed / expected:** fathom wraps the kit table in descendant variants
  (`cardTableLG`/`cardTableXL`: `thead` hidden, rows and cells as blocks, a per-cell
  label shown only below the breakpoint). Expected: a kit option (`Cards: "lg"`) with the
  labels drawn from the column headers.
- **Disposition:** `candidate`. Next action: app-kit maintainer.

**FATHOM-C-KIT-01 — `kanban.Board` has no read-only mode (candidate)**

- **Origin / task:** console wave C, a plan's rows by state (`console_plan.go`).
- **Observed / expected:** every card is `draggable="true"` with a grab cursor, and its
  title link hovers to `text-neon-pink`, a token only the synthwave theme defines. A
  ledger the page cannot change must not look movable. Expected: a read-only option
  (no drag hooks or cursor) and a role-token hover. The lane also has no bound or "and N
  more" affordance; Fathom caps lanes itself.
- **Disposition:** `candidate`; local workaround `readOnlyBoard` strips the three
  affordances, pinned by `TestConsolePlanPage_BoardDrawerAndBurnup`. Next action:
  app-kit maintainer.
- **Also observed (browser proof):** lanes are a fixed `w-64`, so a six-lane board is
  wider than a 1440 px content column and its last lanes sit behind the board's own
  horizontal scroll. Fathom draws a lane only when it has rows (claimed-not-proven
  always) and names the empty ones under the board — which still left five lanes
  (1,344 px) in a 1,094 px column; the lane layout is now FATHOM-C-KIT-05.

**FATHOM-C-KIT-02 — no multi-series or time-axis chart (candidate)**

- **Origin / task:** the burn-up (scope and claimed per commit) and the ledger trends.
- **Observed / expected:** `viz` has single-series `Sparkline` (auto-scaled, no axis)
  and horizontal `StackBar`; a burn-up needs scope and done on one time axis. Fathom
  draws one `StackBar` per point on a shared scale inside a table row — honest and
  accessible, but long for a long history (bounded to 30 commits and 14 days).
- **Disposition:** `candidate` — a line/area chart with a shared scale, a time axis and
  a table alternative. Next action: app-kit maintainer; revisit when a second consumer
  needs a trend with more than one series.

**FATHOM-C-KIT-03 — a wrapping filter nav (candidate)**

- **Origin / task:** the repository filter on `/plans` and `/certifications`.
- **Observed / expected:** `chrome.Tabs` does not wrap and is a `tablist` of links with
  no tab panels; with 14 repositories it overflows 390 px. Fathom composes a wrapping
  link list with 44 px targets and `aria-current`.
- **Disposition:** `candidate`. Next action: app-kit maintainer.

**FATHOM-C-KIT-04 — `dashboard.Widget`'s header does not stack when narrow (candidate)**

- **Origin / task:** the wave C browser proof at 390 px.
- **Observed / expected:** the widget draws its title and its action side by side at
  every width, so a panel whose action is a long verb chip (`fathom plans next --service
  fathom --plan docs/plans/observatory-console.md --json`) squeezes its title to one
  word per line ("Rows / by / state"). Expected: title and action stack below `sm`.
  Nothing overflows; it reads badly.
- **Disposition:** `candidate`; no local workaround (the header is the kit's). Next
  action: app-kit maintainer.

**FATHOM-C-KIT-05 — controls under 44 px, and fixed-width kanban lanes (candidate)**

- **Origin / task:** console wave C's review (defects 14 and 15): the Proof pages'
  pagers, the `/refactors` chooser and a plan's board.
- **Observed / expected:** `chrome.RenderPagination`'s links are `px-2.5 py-1` (about
  30 px tall; the kit's F0 record already noted it for `/search`), `form.RenderSelect`
  and `RenderInput` are `py-2` with a border (38 px) and `RenderButton` 36 px, and a
  kanban card's title link is `text-sm` with no minimum height or focus ring — under
  the 44 px targets fathom's design requires. `kanban.Board`'s lanes are `min-w-[16rem]
  w-64 flex-shrink-0`, so lanes never share a row: five need 1,344 px, and at 1440 the
  fifth sat 6 px into view behind the board's own scroll. Expected: 44 px targets (or a
  size option) on the pager, the form controls and the card link, and a lane layout
  that fits the container (lanes that share the row, or wrap).
- **Disposition:** `candidate`; local workarounds, each pinned by a test:
  `consolePager` wraps every console pager with `[&_a]:min-h-11`
  (`TestConsolePager_FortyFourPixelTargets`), the refactor form lifts its controls with
  `[&_select|input|button]:min-h-11`, and `readOnlyBoard` makes the card link a 44 px
  target with a focus ring and, from `md` up, turns the board into a grid of lanes at
  least 12rem wide that wraps instead of scrolling (`TestConsolePlanBoard_LanesAreBounded`
  fails if the kit renames the classes it rewrites). Next action: app-kit maintainer.

**FATHOM-D-KIT-01 — the heatmap's row-label gutter is fixed at 70 px (candidate)**

- **Origin / task:** console wave D's coverage matrix (OC-28): projects × layers with
  truth states, rows labeled by project name.
- **Observed / expected:** `viz.Heatmap` reserves a 70 px left gutter whenever `YLabels`
  is set, so a label longer than about eleven characters at the 10 px label size is
  clipped by the SVG viewport, and each cell's `<title>` names the row by that same
  label. Expected: an option to size the gutter to its labels (or a separate full name
  for the title). Fathom shortens a label past eleven characters with an ellipsis and
  carries every name whole in the table alternative beside the chart.
- **Disposition:** `candidate`, not patched locally beyond the shortened label. Next
  action: app-kit maintainer; revisit when a second consumer labels heatmap rows with
  identifiers longer than the gutter.

**FATHOM-E-KIT-01 — `chrome.Tabs` on a phone and at 44 px (candidate)**

- **Origin / task:** console wave E, the project workspace's nine link tabs (OC-35).
- **Observed / expected:** the tab bar is one `flex` row with no overflow handling (nine
  tabs are ~700 px, a 390 px screen would pan the page) and each tab is ~36 px high;
  its `role="tablist"`/`role="tab"` links carry `aria-current` but no tab panels, so a
  screen reader announces tabs that behave as links. A scrolling row also leaves the
  current tab past the strip's edge at 390 px (the survey measured Wiring at x 623–689
  in a strip ending at 359). Expected: an opt-in scrolling row that opens on the
  current tab, 44 px targets with a focus-visible outline, and either link semantics
  (`nav` + `aria-current`) or full tab semantics.
- **Local workaround:** `renderProjectTabs` wraps the bar in its own `overflow-x-auto`
  region and gives each link `min-h-11`, `whitespace-nowrap` and a focus outline through
  arbitrary variants on the wrapper; `ui/console.js` scrolls that region (never the
  page) so the `aria-current` tab is in view on load; the kit markup is untouched.
- **Disposition:** `candidate`. Next action: app-kit maintainer.

**FATHOM-E-KIT-02 — `dashboard.RenderWidget`'s header row never wraps (candidate)**

- **Origin / task:** console wave E; a panel's verb chip naming a `--store` or `--root`
  path (the index and wiring tabs).
- **Observed / expected:** the widget header is `flex items-center justify-between`
  with no wrap, so a long action squeezes the `h2` to a word per line (at 390 px and in
  a half-width column at 1440). Expected: the row wraps, the action dropping under the
  title.
- **Local workaround:** the workspace tabs render their panels inside a wrapper with
  `[&_section>div:first-child]:flex-wrap` (`projectBody`); shared `renderPanel` is
  unchanged.
- **Disposition:** `candidate`. Next action: app-kit maintainer.

**FATHOM-G-KIT-01 — graph nodes have no truth state (candidate)**

- **Origin / task:** console wave G, `/learn`'s pipeline (`viz.LayoutGraph`, one node per
  stage with its live count).
- **Observed / expected:** a stage whose instrument did not answer can only be drawn as a
  normal box labelled "—"; the absence reads the same as a measured value in the
  picture, and only the table alternative and the stage panel say why. Expected: the
  heatmap's truth states (FATHOM-KIT-03) for nodes — missing/unknown/error drawn
  distinctly (dashed or hatched box, a legend entry, the reason in the node's title).
- **Disposition:** `candidate`; Fathom keeps "—" and names the reason on the stage panel.
  Next action: app-kit maintainer.

**FATHOM-G-KIT-02 — the narrow ToC panel is always open (candidate)**

- **Origin / task:** console wave G, `/learn/guides/{slug}` at 390 px
  (`docs.RenderToCPanel`).
- **Observed / expected:** the disclosure renders `open`, and each entry is a 44 px row,
  so a guide with 22 outline entries (`getting-started`) spends its first 1,155 px on the
  outline before any prose. Expected: an `Options` field to render it closed (or closed
  above a heading count).
- **Disposition:** `candidate`; not patched (the panel is kit markup). Next action:
  app-kit maintainer.

**FATHOM-G-KIT-03 — `docs` has its second consumer (evidence)**

- **Origin / task:** console wave G renders the use-case guides, the glossary and the laws
  with `docs.RenderToCRail`, `docs.RenderToCPanel` and `prose.css` (`doc-prose`, the
  `doc-link--unresolved` treatment for a link with no page).
- **Observed:** `docs` shipped with one consumer (platform-engineering's portal) under a
  named D17 exception; Fathom is the second. Heading ids stay the caller's (goldmark's
  auto ids, or `{#law-N}`/`{#term-N}` given explicitly), as the package requires.
- **Disposition:** `evidence` for the kit's D17 two-consumer bar. Next action: app-kit
  maintainer records the second consumer.

**FATHOM-G-KIT-04 — the shell's main pins sticky content; the ToC rail's entries are 20 px (candidate)**

- **Origin / task:** console wave G fix pass, `/learn/guides/{slug}` at 1440 px
  (`dashboard.RenderShell` with `docs.RenderToCRail`).
- **Observed / expected:** the shell writes `<main class="flex-1 overflow-y-auto …">`
  inside a `min-h-screen` flex column with no height bound, so `main` grows with its
  content and never scrolls — yet as a scroll container it is the sticky reference for
  every descendant: the rail's `sticky top-6` scrolled away with the 14,000 px guide (top
  417 px, then −2,583 px after `scrollTo(0, 3000)`; with `main` not a scroll container it
  stays at 24 px). The rail's links are `block` text rows, 243 × 20 px, below 44 px
  targets and with no focus-visible ring. Expected: a shell whose `main` is not a scroll
  container unless the shell bounds it to the viewport, and rail entries that meet the
  kit's own 44 px floor (as `RenderToCPanel`'s do).
- **Disposition:** `candidate`; worked around in fathom (`data-console-sticky` and the
  body's `:has()` classes in `console_shell.go`; the guide's own bounded sticky block with
  `[&_nav_a]:min-h-11` in `console_learn.go`), the mirror untouched. Next action: app-kit
  maintainer.

**FATHOM-ADOPT-01 — selected metadata migration**

- **Origin / task:** Fathom; retain the reviewed responsive shell selection without
  invalidation from unrelated catalog records.
- **Observed / expected:** v1 failed only its whole-catalog revision; v2 should bind the
  reviewed current `shell.layout` record and preserve every other requirement.
- **Evidence:** app-kit ADR-0065 and
  `docs/assessments/2026-09-16-fathom-display-adoption.md#consumer-v2-metadata-migration`;
  the retained v2 checker reports 21 pass, one catalog not-applicable and zero failures.
- **Ownership:** app-kit owns the pin contract/checker; Fathom owns selected IDs,
  artifacts, module/mirror declarations, delivery evidence and runtime acceptance.
- **Disposition:** `no-candidate` — this adopts the existing ADR-0065 contract and does
  not propose another API, schema or automation layer.
- **Next action / owner / revisit:** Fathom maintainer reviews and updates the pin only
  when selected metadata changes; revisit on pin failure or an exact released-module
  upgrade, not on unrelated catalog changes.
- **Delivery / verification:** version2 manifest applied; non-migration fields compare
  identically with the retained v1 report. This is static proof, not effective module
  resolution or runtime acceptance.

**FATHOM-UI-01 — freshness and diagnostic display lesson**

- **Origin / task:** Fathom Observatory; represent incomplete freshness truthfully and
  keep diagnostics/grade geometry usable at the intrinsic narrow width.
- **Observed / expected:** positive language required complete known coverage; partial
  values required a lower bound and reason; long diagnostics needed left-aligned wrap;
  grade bars needed intrinsic 256px geometry.
- **Evidence:** app-kit
  `docs/assessments/2026-09-16-fathom-display-adoption.md`, including 13 freshness
  states, long/escaped diagnostics and the real-handler synthetic probe.
- **Ownership:** Fathom owns registry semantics and the app-local composition repair;
  app-kit owns the sanitized reusable lesson/test fixture.
- **Disposition:** `returned` lesson/test with an app-owned repair; no universal
  freshness API or stamped-mirror change.
- **Next action / owner / revisit:** app-kit maintainer retains the fixture and
  coverage-before-conclusion lesson. Revisit when a second analytical consumer needs
  the same missingness or containment cases; its verification is currently unperformed.
- **Delivery / verification:** repaired Fathom source plus durable regression passes the
  scoped race suite, 88 HTTP/412 browser checks, 9/9 CSS checks and CSS freshness.
  Second-consumer propagation remains unverified.

**FATHOM-GUIDE-01 — local theme guide and honest-absence repairs**

- **Origin / task:** Fathom Observatory; review theme and contrast changes against the
  real renderers without live data, and keep failed reads from rendering as zero.
- **Observed / expected:** the guide needs an app-owned CUE source, one synthetic model per
  coverage state and disabled operational links. Guide-only race runs exposed three
  "Absent is never 0" violations (fleet service count, obligation serves, ledger sessions)
  that must render an absence or a named lower bound.
- **Evidence:** app-kit `docs/assessments/2026-09-22-fathom-guide-only.md` and its fixture
  packet; `cmd/fathom/appkit_styleguide_test.go`,
  `observatory_observation_coverage_test.go` and the 12 regressions in
  `scripts/ui-css_test.sh`.
- **Ownership:** app-kit owns `apps/styleguide` (`recipe.styleguide.local`) and the
  returned lessons; Fathom owns the synthetic models, baseline selection, display repairs
  and CSS builder.
- **Disposition:** `returned`. The kit records Fathom as a consumer of the experimental
  recipe (catalog `95151b4`, `docs/design/contribution-loop.md`) without promoting it;
  the display repairs stay app-owned.
- **Next action / owner / revisit:** Fathom maintainer. Revisit on the next reviewed mirror
  bump, on a `recipe.styleguide.local` pin failure, or when the kit's zoom,
  memory/cancellation or G3/G4 evidence lands.
- **Delivery / verification:** landed on Fathom main 2026-09-25 at `f7ab0fbd`, then bumped
  to `dc77e66`. No fork: the mirror is unmodified and the guide consumes the kit package.
  Verified in the originating app with build, race suite, CSS builder/check, adoption
  check and real-store screenshots of home, fleet, service and impact in both themes at
  1440px and 390px. The second consumer's (RAC) evidence is the kit's; it was not replayed here.

### Project-local host configuration review

Reviewed on 2026-09-16: `.claude/settings.json` parses as a hooks-only object with
`ConfigChange`, `CwdChanged`, `DirectoryAdded`, `FileChanged`, `InstructionsLoaded`,
`PostToolUseFailure`, `PreToolUse`, `Setup` and `SubagentStart` event groups. A single
app-kit/MCP text match is only the `PostToolUseFailure` matcher, not server registration.
`.claude/settings.local.json` parses as a permissions object with an `allow` key; its
values were not recorded or changed. `.codex/hooks.json`, `.mcp.json` and
`.codex/config.toml` are absent; `.claude/commands/verify.md` was inventoried as a
project command, not host registration. No global configuration was inspected.
No change is warranted: this handoff does not require registration, the existing hooks
and local permissions are independently owned, and installing configuration without a
fresh-host test would not prove invocation. Fresh Claude/Codex retrieval remains
unverified.

## Observatory display handoff (2026-09-16)

The owner-authorized current-source candidate improves display only: thirteen freshness
states reserve positive success language for complete evidence, render partial evidence
as a lower bound with its reason, keep long diagnostics left-aligned and wrapping, and
grade the intrinsic 256px panel instead of only wide layouts. Existing I38 and ADR-027
already govern those semantics, so no new Fathom decision is needed. The real-handler
synthetic probe passed 88 HTTP and 412 browser checks against this checkout. This is
fixture evidence, not live-daemon, production-data or full accessibility proof. A
scoped race suite plus the 9/9 CSS harness and exact CSS freshness check pass; the
display implementation changed no dependency, generated CSS, theme or app-kit mirror.
The durable `TestAppkitGradeDistributionGeometry` additionally refuses the retained
420px original and passes the256px current source under `-race`; geometry no longer
depends only on the optional browser probe. Evidence and limitations live in app-kit
`docs/assessments/2026-09-16-fathom-display-adoption.md`.

## CSS fixture and artifact handoff (2026-09-16)

Owner-authorized repair: the synthetic CSS fixture now copies the existing
`scripts/lib/build-cache.sh` execution dependency. The adoption manifest selects that
helper and reconciles only the reviewed `scripts/ui-css.sh` hash; ADR-039/I39 cache
policy, helper/builder behavior, UI/CSS, dependencies and the kit mirror are unchanged.

The ordinary isolated replay passes all nine unchanged CSS assertions, separate shell
syntax checks, the active checkout's read-only `--check-sources`, and the retained
adoption checker (22 pass, one not applicable). Copied-fixture checks reject a modified
or missing helper by its exact path; restoring it passes again. Fresh private caches
were used with cleanup disabled, without accessing or pruning real caches.

Evidence is retained under `workspace/tmp/appkit-css-pin-repair.KvsMu3K7/`, including
the original missing-helper failure and an unexpected first post-repair synthetic
source-read failure. Subsequent traced and ordinary runs passed unchanged; the latter
failure's cause is not established. App-kit's
`docs/assessments/2026-09-16-propagation-reconciliation.md` records exact receipts.
This is not a new Tailwind/CSS freshness run, full Fathom gate, browser proof or release;
no daemon restart, commit, deployment or paused-work integration was performed.

## Next acceptance and return

Next: analytical hierarchy, meaningful populated/absent/stale/partial/error cases,
drill-down/context retention, synthetic task completion, contrast/focus/target sizes
and measured responsiveness. Avoid decorative charts or invented success metrics.
Return observed generic failures to app-kit with adversarial fixtures; retain app
domain calculations here. The mirror mismatch demonstrates why adoption needs
standalone runtime proof, not just a passing workspace or static manifest check.
