# app-kit UI components

The kit's reusable UI lives under `appkit/ui/`. Everything is **pure-Go,
server-rendered, token-themed (D7), deterministic, and accessible** — no npm
toolchain, no client framework. Data-viz components return an SVG `string`; HTML
components return an HTML `string`; both embed anywhere (in templ,
`templ.Raw(...)`).

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
| `chat` | **L2 + L3b** | `chat.js` is L2; the Go markup emits role utilities |
| `callout` | **L3b** | |
| `chrome` | **L3b** | |
| `docs` | **L1 + L3b** ² | `prose.css` is the L1 typography layer (`doc-prose`, the boxed `doc-toc`, the heading permalink hook); `toc.go`/`anchor.go` are L3b — `RenderToC`, `RenderToCPanel`, `RenderToCRail`, `RenderPermalink`. Needs `@source ".../appkit/ui/docs/*.go"` plus `@import ".../appkit/ui/docs/prose.css"` |
| `dashboard` | **L3b** | layout only — charts/KPIs/tables come from `viz`/`stat`/`table` |
| `form` | **L3b** | |
| `hitl` | **L3b** | |
| `stat` | **L3b** | composes `viz.Sparkline` (L3a) inside an L3b tile |
| `topology` | **L3b** | emits literal Tailwind utility strings nine times — the clearest L3b in the kit |
| `vizframe` | **L3b** | the provenance frame (ADR-0017 §6). Needs `@source ".../appkit/ui/vizframe/*.go"` |
| `explainer` | **L3b** ¹ | |
| `export` | **L3b** ¹ | |
| `card` | **L3b + L4** | templ |
| `kanban` | **L3b + L4** | templ |
| `table` | **L3b + L4** | templ |

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

`GradedRole(frac)` maps 0..1 → danger / caution / positive for consistent
status coloring across components.

## `appkit/ui/callout` — messaging (HTML)

| Component | Use it for |
|---|---|
| `Callout` | block message: info / success / caution / danger / note (ARIA note/status/alert + icon); `Body` (escaped) or `BodyHTML` (trusted rich/block) |
| `Disclaimer` | the standard "information only / not advice" caution callout |
| `Badge` | inline status pill tinted by role |

## `appkit/ui/stat` — KPI tiles (HTML)

`Stat` — a big value + label + optional colored delta + optional trend
sparkline (composes `viz.Sparkline`).

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

## `appkit/ui/form` — form & input vocabulary (HTML, no JS)

Token-themed, accessible, escaped, server-rendered. `Field` owns the label/help/
error plumbing; controls carry their own ARIA so a bare control stays consistent.

| Component | Use it for |
|---|---|
| `Field` | labeled wrapper: label + control slot + help/error (`aria-describedby`, `aria-invalid`); error takes priority over help |
| `Input` | single-line control of any text-like `Type` (text/email/number/date/…) |
| `Textarea` | multi-line text |
| `Select` | dropdown with optional placeholder prompt + selected option |
| `Checkbox` | single boolean with inline label |
| `RadioGroup` | mutually-exclusive options as a `<fieldset>` + `<legend>` (grouped requiredness) |
| `Button` | action control — `primary` / `ghost` / `danger`; `<button>` or `<a>` (with `Href`) |

## `appkit/ui/chrome` — structural creature comforts (HTML, no JS)

| Component | Use it for |
|---|---|
| `EmptyState` | "nothing here yet" panel (icon + heading + body + CTA) |
| `Breadcrumbs` | navigation trail |
| `Pagination` | windowed prev/next + page links (server-rendered) |
| `Tabs` | link-based view switcher (active underline) |
| `Skeleton` / `SkeletonCard` | loading placeholders (Tailwind `animate-pulse`) |
| `Disclosure` | collapsible "show more" section on native `<details>` (no JS) |
| `DescriptionList` | aligned term/detail metadata (`<dl>`) |

All server-rendered with plain links/markup, so they honor the loopback
invariant (D5) — no shell-specific JS.

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

## `appkit/ui/card`, `kanban`, `table`, `router` — composable layout (templ)

The templ-based composition primitives. `Card` is the single canonical bordered
surface (chrome no longer duplicates it); `kanban` composes it into a board.

| Component | Use it for |
|---|---|
| `card.Card` | the bordered-surface primitive: `Header` / `Body` / `Footer` slots (templ.Component); base for higher-level layouts |
| `kanban.Board` | server-rendered Kanban — `Column`s + `CardItem`s (StageID → Column.ID), composed from `card.Card`; emits `data-*` hooks for the app's hypermedia layer |
| `table.Table` | generic data table — typed `Column`s (optional sortable `<th>` links) + `Row`s; tolerant of cell/column length mismatch |
| `router` | embedded dependency-free History-API client router (`router.js`) for static-client surfaces (e.g. GitHub Pages SPAs); served via `Handler` or inlined via `Script`. No build step, no npm toolchain (ADR-0002 as amended by ADR-0017 §3) |

## `appkit/ui/hitl` — human-in-the-loop (HTML)

For agentic apps that need a person in the decision path and a window into the
run.

| Component | Use it for |
|---|---|
| `ApprovalCard` | a proposed agent action awaiting a decision: intent + risk badges, confidence rating, rich summary, **caller-supplied action HTML** (approve/reject/edit forms — the app wires the loopback POST) |
| `RunTrace` | a vertical event rail of an agent run (done / active / failed / pending) — "what's happening under the hood" |
| `DiffView` | before → after review of a proposed agent change (current vs proposed, per field; additions/removals) — pair with `ApprovalCard` |
| `ToolCall` | inspect one agent tool invocation: name + status badge + collapsible args/result (native `<details>`) — "what did the agent do" |

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

## `appkit/ui/tokens` — the design tokens

The CUE-authored token pipeline: color roles × light/dark, radius, fonts.
`appkit gen` emits `theme.css` (`@theme` + runtime scheme vars), DTCG JSON, and
Go role-name constants (`ColorNames`, `RadiusNames`, `FontNames`).

---

See the **Gallery** (`apps/gallery`, `task gallery:run`) for every component
rendered live, and `cmd/appkit-mcp`'s `get_kit_css_setup` for wiring the CSS in
a consuming app.
