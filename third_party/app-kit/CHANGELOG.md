# Changelog — app-kit

One entry per tag once tagging starts (D13); pre-tag work logged by date.

## 2026-09-14 (ui) — stat reflow and runtime theme identity

Shared stat bands preserve metric content at narrow widths and delay dense column
layouts until sufficient space is available. Generated themes now route font and
radius utilities through runtime aliases so named themes can change identity tokens
without rebuilding utility CSS. Existing semantic roles and palette values are unchanged.

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
