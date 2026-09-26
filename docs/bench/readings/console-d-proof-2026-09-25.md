# Console wave D — the Knowledge pages' browser proof (2026-09-25)

The doctrine's rule 4 (`cmd/fathom/CLAUDE.md`: a 200 is not a rendering) and the
plan's gate (`docs/plans/observatory-console.md`: every page row needs a browser
proof in both themes at 1440 and 390 px, recorded in the wave's reading), for
OC-27..OC-34: `/layers`, `/layers/{layer}`, `/claims/{id}`, `/packs` and
`/explain`.

## Verdict, plainly

**Wave D renders, and each page says what its verb says.** Five pages (plus the
explainer's picker and a stale claim) were opened in a real browser at 1440×900 and
390×844, in light and dark: all 28 views pass the shell checks — exactly one `h1`,
the skip link first and moving focus into `main`, the Knowledge group in the nav with
the current page marked (inside the mobile Menu at 390), breadcrumbs with
`aria-current`, no horizontal page overflow, no element past its panel's edge, and no
console errors, failed requests or ≥400 responses. On the same daemon each page's
JSON section equals its CLI verb's output (6 of 6), and `fathom obligations` serves
exactly the list the MCP `obligations` tool serves for `internal/conserve/conserve.go`
— once, not twice, for the rule the store holds in two copies (OC-33).

The first pass was not clean. It found five defects, fixed before this reading's
final run (below, "What the pass found").

## How it was run

- **Binary:** `bin/fathom` from branch `cp-wave-D` at `422fa98c`, the module's Go env
  (`GOWORK=off`, CGO on).
- **Daemon:** `serve-http` on `127.0.0.1:7673`, `--auto-refresh=false`,
  `FATHOM_SELF_RESTART=off`, `HOME` a scratch home (the daemon record, page-view log
  and verify ledger stayed there). The live daemon on 7575 and its store were not
  touched.
- **Data:** a `sqlite3 .backup` of the fleet store taken through a read-only URI
  (18 projects, 137,630 observations, 3,130 current claims), plus copies of the call
  log, session ledger, verify ledger and hooks beat log.
- **Browser:** Chromium through the Playwright MCP, each width in its own fresh
  browser context (the MCP's shared page was being driven by other waves' agents at
  the same time). The theme was set with the page's own appearance control — the
  sidebar select at 1440, the one inside the Menu disclosure at 390 — and every
  check ran in the page: `h1` count, the first Tab stop and where Enter moved focus,
  the nav's and the trail's current item, `scrollWidth` against `clientWidth`, every
  element's right edge against its panel's (outside the kit's scroll regions), the
  body ground (light `oklch(0.985 0.005 260)`, dark `oklch(0.16 0.012 260)`), and the
  console, page-error, failed-request and response listeners.
- **Keyboard journey:** on `/explain` with no project, Tab reaches the project select
  (16 stops), the file field follows it, typing a path and pressing Enter submits the
  GET form to `/explain?svc=fathom&file=internal%2Fconserve%2Fconserve.go&layer=rules`,
  which renders its 13 candidate cards; Enter on "Every candidate as a table" opens
  it; the verb chip's copy button takes focus. On `/layers` the heatmap's scroll region
  is a Tab stop (34 presses from the top) with the console's focus outline. A claim's
  `#cite-L6` line anchor resolves.

## Per-page results

| Page | Route | h1 | Height 1440 / 390 | Warm render | HTML |
|---|---|---|---|---|---|
| Semantic layers | `/layers` | Semantic layers | 3408 / 6461 | 37 ms | 117 KB |
| Layer | `/layers/obligations` | obligations | 7920 / 10647 | 35 ms | 107 KB |
| Claim | `/claims/0f8b475a…` | Claim 0f8b475a | 1422 / 2350 | 18 ms | 48 KB |
| Claim, stale | `/claims/fb47bebc…` | Claim fb47bebc | 1462 / 2582 | 18 ms | 45 KB |
| Packs | `/packs` | Packs | 4067 / 5499 | 60 ms | 77 KB |
| Serving explainer | `/explain?svc=fathom&file=internal/conserve/conserve.go` | Serving explainer | 3567 / 5821 | 9 ms (cached) | 166 KB |
| Explainer picker | `/explain` | Serving explainer | 900 / 1008 | 8 ms | 31 KB |

Warm render is the best of five `curl` fetches against the daemon. An explanation for
a file not yet explained costs about 380 ms: the raw-fallback and coverage
enrichments read the project's whole corpus from disk. It is then cached per daemon
while the event bus's cursor holds (30 s at most), so the budget is met warm, not on
the first view of a file. Every page is under the 250 KB HTML budget.

What the pages show on this fleet: 9 layers (3 compiled anywhere: adr-rules,
obligations, spec-bindings; 6 never compiled, each listed with where it is installed),
3,130 current claims in 16 of 18 projects, 77 real compile runs that spent $26.31 and
5 runs refused on their estimate (their $1.18 estimate is not spend — the mockup's
$27.50 counted it), 4,051 claims emitted and 5,168 rejected, 1,738 re-anchored and 63
contradicted verdicts. 27 registered languages, 14 indexed in the fleet, 3 with a
published precision/recall reading (Go, Python, TypeScript); the other 24 say
"unmeasured".

Screenshots are `<page>-<width>-<theme>.png` (28 full pages) plus eight `crop-*`
panel crops from the first pass, in the proof's scratch directory `cp-D/shots/`; they
are not committed.

## What the pass found, and what changed

1. **The explainer's served list was unreadable at 390 px.** Five table columns
   squeezed each rule to a word per line (`[overflow-wrap:anywhere]` let the column
   shrink to a letter). It is now the kit table from `lg` up and one stacked entry per
   rule below it, as `/statusz`'s services are; table cells no longer break names
   anywhere, so a wide table scrolls in the kit's wrapper instead.
2. **The coverage heatmap's column labels ran into each other** — the kit draws a
   column label under its cell without fitting it. Columns carry short keys (`adr`,
   `oblig`, `spec`) with a key line above the chart. Its row labels are shortened past
   eleven characters by the kit's fixed 70 px gutter; the table alternative carries
   every name whole (returned to the kit as FATHOM-D-KIT-01).
3. **A never-compiled layer drew three absence lines**; it now draws one ("not
   compiled — …") and where it is installed.
4. **`/explain` took 420–520 ms warm on fathom**; explanations are now cached as
   above, and `/layers` reads the store once for its strata and its coverage.
5. **A real claim's unbroken `asserted by` identity pushed its description list past
   the panel's edge**; long identifiers wrap inside it.

## Parity and serving on the real fleet

- `fathom layers strata --json`, `layers coverage --json`, `layers show obligations
  --json`, `claim show <id> --json`, `report packs --json` and `obligations --service
  fathom --file internal/conserve/conserve.go --layer rules --explain --json`, run with
  `--url http://127.0.0.1:7673` (the store resolved through the daemon's record, rule
  `daemon`), each equal their page's JSON section.
- `fathom obligations --json` for `internal/conserve/conserve.go` and the MCP
  `obligations` tool on the same daemon serve the same eight rules in the same order,
  both `displaced: 178`. The store holds "Inject clocks …" twice (with and without
  backticks); both surfaces serve it once, and the explanation names the second copy
  a duplicate of the rule ranked #4 — served fourth among the compiled rules, sixth in
  what the agent reads after two raw rows — instead of letting it take a slot.
- Not caught, by design: rules 7 and 8 for that file are two different sentences
  compiled from the same lines (`internal/CLAUDE.md#L8-L12`, "Never mix fixed fixture
  dates …"). OC-33 dedupes identical normalized statements only; a near-duplicate
  from one citation is a candidate follow-up, not a defect of this row.

## Cleanup

The daemon on 7673 was stopped. The scratch store copies, the call-log and ledger
copies and the scratch home were deleted. The shots were kept.
