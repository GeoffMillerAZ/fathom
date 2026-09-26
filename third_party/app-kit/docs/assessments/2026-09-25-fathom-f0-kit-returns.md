# Fathom console wave F0: four returned kit defects (2026-09-25)

Fathom's observatory console wave F0 (vision 35 / ADR-045, the console shell over the
`dc77e66` kit mirror) moved every page onto kit packages and found four shared
defects. Fathom records them in its `docs/app-kit.md`, section "Observatory console
wave F0", under "Kit defects observed", each with a local workaround. That wave was not
on Fathom `main` (`a683d602`) when this change was made, so no Fathom revision is pinned
here. This receipt covers only the kit side: the fixes, their tests and one Gallery
browser run. Fathom has adopted none of them yet.

## Contribution records

```text
ID / originating app and user task: FATHOM-F0-KIT-01, Fathom console /statusz services
  table (a screen-reader user reads a service row against its column headers).
Observed / expected result / affected kit and app revisions: table.Table rendered every
  <th> without scope; expected scope="col" on column headers (scope="row" on row
  headers, which the table does not render). Kit 54f5fac and earlier; Fathom wave F0.
Evidence: TestTable_EveryHeaderCellHasColumnScope (six header shapes); Gallery /layout
  browser check below (4 of 4 headers scoped; 0 of 4 before).
Ownership: kit L3b (appkit/ui/table); Throughline is the other catalog consumer.
Disposition: returned.
Next action and owner: Fathom drops renderKitTable's scope injection when it adopts
  this revision (Fathom maintainer). Revisit if a consumer needs row headers.
Delivery: this change on app-kit main; Fathom fork retained until adoption.
Verification: kit Gallery only; Fathom and second-consumer results unverified.

ID / originating app and user task: FATHOM-F0-KIT-02, Fathom /statusz on a 390px phone
  (read the services table without the whole page scrolling sideways).
Observed / expected result / affected kit and app revisions: the sr-only "Actions"
  header label is absolutely positioned; its containing block was outside the
  overflow-x-auto wrapper, so it widened the document (Fathom measured an 861px
  document; Gallery /layout 444px at 390px). Expected: the table scrolls inside its own
  wrapper. Kit 54f5fac and earlier; Fathom wave F0.
Evidence: TestTable_ScrollWrapperContainsHiddenHeaderLabels; the Gallery browser check
  below (page overflow 54px before, 0 after, at 390px in both themes).
Ownership: kit L3b (appkit/ui/table).
Disposition: returned.
Next action and owner: Fathom removes the `relative overflow-hidden` wrapper in
  console_statusz.go on adoption (Fathom maintainer).
Delivery: this change; Fathom fork retained until adoption.
Verification: kit Gallery only; Fathom and second-consumer results unverified.

ID / originating app and user task: FATHOM-F0-KIT-03, Fathom console page heading
  (keyboard and touch users move up the breadcrumb trail).
Observed / expected result / affected kit and app revisions: chrome.Breadcrumbs links
  were about 17px high with no focus-visible treatment beyond the browser default;
  expected the kit's 44px target floor and its 2px focus-role outline. Kit 54f5fac and
  earlier; Fathom wave F0.
Evidence: TestBreadcrumbs_LinksHaveTargetSizeAndVisibleFocus; page heading golden
  regenerated; the Gallery browser check below.
Ownership: kit L3b (appkit/ui/chrome); chrome.RenderPageHeading composes it.
Disposition: returned.
Next action and owner: Fathom drops renderConsoleHeading's [&_a] wrapper on adoption
  (Fathom maintainer). RenderPagination's ~28px links are a separate candidate, not
  fixed here.
Delivery: this change; Fathom fork retained until adoption.
Verification: kit Gallery only; Fathom and second-consumer results unverified.

ID / originating app and user task: FATHOM-F0-KIT-04, Fathom console charts on a 390px
  phone (traffic sparkline, 560px tool-load bars, 900px session timeline).
Observed / expected result / affected kit and app revisions: every viz chart drew at a
  fixed pixel width; expected a width-responsive option. Kit 54f5fac and earlier.
Evidence: TestFluid_SameDrawingWidthFollowsContainer (17 renderings over 13 charts),
  bars_fluid and sparkline_fluid goldens, every pre-existing golden unchanged; Gallery
  /components fluid sparkline in the browser check below.
Ownership: kit L3a (appkit/ui/viz).
Disposition: returned, with a stated limit: Fluid scales the drawing and does not
  reflow it, so text still shrinks below W. Fathom's scroll-region-plus-table
  alternative for its text-dense charts stays the right local choice.
Next action and owner: Fathom may replace [&>svg]:max-w-full on the sparkline with
  Fluid (Fathom maintainer). Revisit when a consumer needs reflowing text charts.
Delivery: this change; opt-in, so no consumer output changes until adopted.
Verification: kit Gallery only; Fathom and second-consumer results unverified.
```

## Changes

- `table.Table`: `scope="col"` on every header cell (select, data and action columns);
  the scroll wrapper gains `relative`. There are no row headers, so no `scope="row"`.
- `chrome.Breadcrumbs`: links get `inline-flex min-h-11 min-w-11 items-center
  justify-center rounded-sm` and `focus-visible:outline-2 outline-offset-2
  outline-focus`. The page heading golden changes accordingly.
- `viz`: `Fluid bool` on the options of the 13 fixed-width charts, through one
  `chartOpen` helper. Set, the tag is `width="100%"` without `height`, with the same
  `viewBox` and `preserveAspectRatio="xMinYMin meet"`, and nothing after the tag
  changes. Unset, output is byte-identical to before.
- Gallery `/components` adds "Sparkline — fluid (fills its container)"; the INC-39
  `components_reflow.mjs` caption list and README count follow (19).
- No new CSS utilities were needed: the rebuilt Gallery `app.css` (pinned Tailwind
  4.3.0) is byte-identical to the committed file.

## Package evidence

Each new test failed on the `54f5fac` sources before its fix (the Fluid test did not
compile). `go test ./appkit/ui/... ./apps/gallery ./appkit/catalog` passes, and so do
all existing viz goldens. `TestFathomF0KitReturnsAreServed` pins the served
Gallery markup and the built CSS rules the browser check depends on.

## Browser evidence

| Artifact | sha256 |
|---|---|
| Gallery binary (`go build ./apps/gallery/cmd/gallery`, go1.26.3 darwin/arm64, uncommitted tree; a rebuild after the last comment-only edit is byte-identical) | `330c2cb7e84cc73e8677f52f3af2db967a99e231b3b8c21d6386ae81d128f6de` |
| Baseline Gallery binary at `54f5fac` | `90ac5cb82ff501912f7e0d211a3d8ac2a76dc08ccbae39fcd174976bf70aea5e` |
| `appkit/ui/table/table_templ.go` | `af04fa779fb7557c0727f9dff0699dcf0357168fbd927a1fdc7fc76e858a5f95` |
| `appkit/ui/chrome/chrome.go` | `c4b6d42757f7860c5803236336a3eb1da47cd1a1393f595f35d5a5a6079b816b` |
| `appkit/ui/viz/svg.go` / `sparkline.go` | `19a9f9bd3595ab44647b536ced90822b7df67c71ddfe7db53e2e166abd3b6df9` / `722e8c3e21b6dde292b6e59a641ceea06f0682a99ad0909a242722b024a010cb` |
| `apps/gallery/render.go` | `acf2f80d8d0cbceb38a313b5ec8c763852277309564ace44a83417838e7bfe00` |
| `apps/gallery/ui/app.css` (unchanged) | `54dc0b14b856f74d75ea9a54ea62192f039d4d0660df874c86d0f5985328b9ad` |
| `appkit/catalog/catalog.cue` (embedded) | `134bdc12bb0f9106ec6a745aaef1d23e22348a4b2368170e1b1024bf9349187d` |
| [Harness](fixtures/fathom-f0-kit/browser-check.js) | `ed064f0b95bc111e81ef777a4f433e8f1d708d7f24ac31b551050166a13f7aac` |

The harness ran through the Playwright MCP `browser_run_code` tool in Chromium 153
(user agent `Chrome/153.0.0.0`), macOS 26.6.2, against loopback Gallery binaries with
`-design-assets=` (no private assets). The committed bytes point at the candidate's port;
the baseline run changed only that port. Screenshots stayed in a private scratch
directory: the 390px table specimen, the focused dark trail and the fluid card at 390
light and 1280 dark were reviewed by eye.

| Check (390×844 and 1280×900, light and dark) | Baseline `54f5fac` | Candidate |
|---|---|---|
| `/layout` document overflow at 390px | 54px in both themes | 0 |
| `/layout` document overflow at 1280px | 0 | 0 |
| Kit wrapper position / contains the sr-only "Actions" label | `static` / no (offset parent `body`) | `relative` / yes |
| Table scrolls inside its wrapper at 390px (1280px: fits) | yes | yes |
| Header cells with `scope="col"` | 0 of 4 | 4 of 4 |
| `/agentic` crumb links "Cases", "Doe v. Roe" (w×h) | 39.6×16.5, 68.7×16.5 | 44×44, 68.7×44 |
| First crumb reached by Tab (19th stop at 390, 36th at 1280) | `:focus-visible`, UA ring `auto 1px` | `:focus-visible`, `solid 2px`, offset 2px, focus role (`oklch(0.55 0.19 262)` light, `oklch(0.72 0.15 262)` dark) |
| `/components` fluid sparkline, width vs card content box | absent | 284 = 284 at 390; 406 = 406 at 1280 |
| Fluid height from the 320×48 viewBox | absent | 42.6 / 60.9 (ratio 6.668 / 6.667) |
| Fluid chart inside its figure; `/components` overflow | absent | yes; 0 |

## Limits

- The Gallery table is one three-column specimen with one hidden label. Other tables,
  zoom, forced colors, screen readers and engines other than Chromium are unchecked.
- The positioned wrapper now contains, and scroll-clips, absolutely positioned content
  from custom cells that used to escape it. No Gallery specimen has such content.
- The browser run reached breadcrumbs by keyboard, not by touch; 44px is measured
  geometry, not a touch-accuracy result.
- Fluid does not keep text legible. A 560px chart in a 340px column draws 11px text at
  about 6.7px. The Gallery shows Fluid only on a text-free sparkline.
- `RenderPagination`'s ~28px links and `hitl.DiffView`'s ink-faint toggle, which the
  same Fathom record lists, were not changed here.
- The opt-in `components_reflow.mjs` harness was not run; its caption list was updated
  and `TestComponentsReflowHarnessPinsEveryCard` passes.
