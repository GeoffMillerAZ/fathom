# Command palette: experimental build and scoped Gallery evidence

## Contract

Originating consumer: Fathom, `FATHOM-KIT-01` (vision 35 observatory console;
its `docs/design/observatory-console.md` §2 asks for Mod+K over `overlay.RenderDialog`,
grouped `/search?q=` fragments, arrow/Enter/Escape with focus return, and the same
results as a no-JavaScript page). The installed appkit MCP (catalog revision
`3511030…`) returned no capability for `command palette`, so this is new first-party work
(ADR-0066/0069), not a conversion of a licensed original. A private command-palette
reference was located by metadata only; no licensed source was retrieved or copied.

Scope: `appkit/ui/palette` (L2 `palette.js` + L3b renderers), catalog
`interaction.command-palette` (experimental), Gallery `/palette` and `/palette/search`,
inventory `candidate.command-palette`. Base `dc77e66`, branch `fathom-kit-palette`.
Out of scope: changing `overlay`, Fathom integration, promotion, performance budgets.

## What the renderer and script guarantee

- Trigger is a real link to `Palette.Action`; primary click opens, modified click
  keeps the link. Action query parameters travel as hidden fields in both forms.
- Invalid configuration or results are refused before output (IDs, same-origin
  relative Action/ResultsURL, shortcut grammar, debounce/timeout bounds, blank labels,
  unsafe hrefs, Error with results, 32 groups/200 results).
- The fragment is plain headings, lists and links; the script alone adds listbox,
  group and option roles, so the no-JS page keeps link semantics.
- The script inserts a same-origin 2xx response of at most 512 KiB only when it is
  exactly one `[data-palette-fragment]` (whitespace and comments aside) whose
  `data-palette-query` is the query it asked for. Anything else (non-2xx, network
  error, timeout, cross-origin URL, a fragment for another query, or a full page such
  as a login redirect, including a page that renders this palette and so carries its
  idle fragment) shows the configured error template.
- Zero results with a `Notice` is `partial`, not `empty`: the announcement ends in
  "(partial)" and the default advice does not blame the query.
- The trigger renders no `aria-keyshortcuts` and a hidden, empty hint; the script
  adds both, in the viewer's platform notation, only to triggers whose palette is
  mounted. `AppKitPalette.init(scope)` accepts a `[data-palette]` root or any
  container, forgets palettes whose markup left the document (so a swapped-out
  instance cannot swallow the shortcut) and re-upgrades every trigger.
- The shortcut follows the typed Latin letter or digit; the physical key stands in
  only for other characters (non-Latin layouts, Option symbols, shifted digits).

## Deterministic checks

| Command | Result |
|---|---|
| `go test ./appkit/ui/palette` | pass: goldens (trigger, dialog, form, results), parsed-DOM ids/labels/hooks, all states including zero-result partial, no server-side shortcut promise, 29 configuration refusals (each checked on trigger, dialog and form) and 12 results refusals, escaping, no literal colors or `dark:` |
| `node --test appkit/ui/palette/palette_test.mjs` | 28/28 with DOM/timer/fetch doubles: debounce coalescing, immediate abort of a superseded request, late response ignored, timeout reported once, non-2xx/non-fragment/full-page/other-query/network/cross-origin errors, Enter fallback when stale or composing, focus return, platform Mod key, keyboard layouts, re-mount, late and orphan triggers, `init(root)` |
| `go test ./apps/gallery/...` | pass, including fragment/full-page split, `Vary`, 503 path, the `unsearched` partial state, no observed consumers on the catalog detail and handler release when the client aborts `slow` |
| `go test ./appkit/catalog ./cmd/appkit-mcp` | pass: no consumer claimed for the palette; common phrasings (keyboard shortcut, Cmd+K, Ctrl+K, global/site search, search modal, quick open, command menu) find it; shared selection corpus through catalog, MCP and Gallery |

The node doubles pinned one real defect before browser work: the footer
"Open search page" link lives outside the `data-palette` body, so the first
`syncPageLink` never found it.

## Browser run of record

Harness: [`fixtures/command-palette/browser_check.js`](fixtures/command-palette/browser_check.js)
(sha256 `e0a1c8ca0601…1847`), executed as a Playwright MCP `browser_run_code`
function after the second-pass review corrections (an earlier execution against a
binary built before the last catalog-prose edits observed the same values, stall
error at 5177 ms); output
[`results.json`](fixtures/command-palette/results.json) (`d0fd56d91fad…91b9`). Browser
`browser.version()` 153.0.8010.53, macOS arm64, Go 1.26.3, pinned Tailwind v4.3.0.
Gallery binary sha256 `6384a9291bb8…18cb` built from the committed tree on `127.0.0.1:8771`,
`app.css` `a024c9b91071…b00e`, `palette.js` `9dde452e3f6b…25f1`, `palette.go` `0350091cbafb…675f`. Synthetic data: Gallery
routes, the embedded catalog and reserved queries. Each scenario ran in its own
browser context because the MCP browser session was shared with another agent's
pages. The first run of record (harness `128807977fe0…afa8`, results
`9ffa2ca5a42d…0ec7`, binary `7789bdde6fac…153fe`) is superseded; its A–E
observations were reproduced unchanged apart from the hint and no-JavaScript fields
the review corrected.

| Scenario | Viewport / theme / input | Expected | Observed |
|---|---|---|---|
| A shortcut | 1440×900 light, Apple platform, keyboard | ⌘K opens from a focused page link and focuses the input; Ctrl+K does not | as expected; trigger 44px, `aria-haspopup=dialog`, `aria-keyshortcuts=Meta+K`, hint ⌘K revealed by the script |
| A debounce | typing `overlay` at 20 ms | one request with the header | `overlay:200` only |
| A selection | ArrowDown ×2, ArrowUp | activedescendant moves and wraps; options `tabindex=-1`, ≥44px | option-1, option-0, option-1; 44/52px; outline 2px + soft accent |
| A close/return | Escape; trigger click; Enter | closes, focus back on invoker; reopen keeps query selected; Enter follows | focus on "the full search page"; value selected; landed on `/catalog/interaction.overlays?theme=light` |
| A stale Enter | type `s` then Enter before results | native GET for exactly what was typed | `/palette/search?theme=light&q=overlays` |
| A dismissal | × button, backdrop, ⌘-click trigger | closes; modified click opens the page in a new tab | closed; new tab `/palette/search?theme=light` |
| B states | 1280×800 dark | empty, partial notice, 503 error, recovery | all states with correct `aria-expanded`/activedescendant |
| B lifecycle | `slow` then `slowly`; `stall` | superseded request aborted, busy row shown without shifting results; 5 s timeout error; clearing sends nothing | `slow` aborted, results top 252→252px; error at 5161 ms; 0 requests on clear |
| C narrow | 390×844 OS dark, `long` | no horizontal overflow; long identifiers wrap; 44px targets | no overflow; options 333×172 and 333×44; input/link 44px; key hints hidden |
| D non-Apple | Win32 emulation | Ctrl+K opens, ⌘K does not, hint Ctrl+K | as expected; hint visible |
| E no JavaScript | 390×844 dark, JS disabled | no shortcut promised; link reaches the page; GET form keeps theme; h1/h2; plain links; 44px; focus ring | no `aria-keyshortcuts` or `aria-haspopup`, hint hidden (`display: none`, empty); Ctrl+K/⌘K change nothing; `?theme=dark&q=overlay`; H1 + two H2; `role=null`; 44/44/44/56px; 2px outline; `fail` → 503 error state |
| F re-mount | 1280×800 light; palette markup replaced, then `init(document)` | before init ⌘K neither throws nor is swallowed; after init the new palette opens, focuses and returns focus | before: closed, not prevented; after: open, prevented, input focused; Escape returns focus to the invoker; no page errors |
| F full-page answers | route answers the palette request with 302 to `/palette` (renders the palette) and to `/overlays` | both show the error state and announce the failure | both `error` for `overlay`, status "Search failed. …" |
| F zero-result partial | `unsearched` | partial, count 0, announcement says partial, no spelling advice | `partial`, `0`, "No results for “unsearched” (partial)", "Matches may exist in sources that were not searched." |
| F late trigger | trigger markup from `/palette` inserted after load, then `init(document)` | hint, `aria-keyshortcuts` and popup added in Apple notation; click opens | before: hidden empty hint, no attributes; after: ⌘K, `Meta+K`, `dialog`; click opens |
| F `init(root)` | second palette appended; `init` on its `[data-palette]` element | `open(id)` false before, true after | false → true, dialog open |
| F layouts | synthetic ⌘ keydowns: `t`/KeyK (Dvorak), `k`/KeyV, `л`/KeyK | Dvorak's physical K does not open; the key typing k does; non-Latin falls back to the physical key | not opened/not prevented; opened; opened |

Retained failures and corrections, in order: an exploratory run showed the input's
reserved padding pushed the native clear button to mid-field (fixed with a separate
busy row); the Gallery header could not take a "Command palette" entry without
exceeding the signed-analytics 20 KiB page budget (specimen reached from Overlays
and the catalog instead); the first execution of the harness failed with
`ReferenceError: URL is not defined` in the MCP runner (corrected by stripping
origins by hand; the run of record is the second execution).

## Second-pass review corrections

A review of `887814f` reported nine findings. Each was reproduced by a failing test
before its fix (node doubles for the script, Go tests for renderers and the
catalog), then confirmed in the browser run above. None was skipped.

1. Re-mount: a swapped-out instance stayed first in the shortcut loop and its
   detached `showModal` threw `InvalidStateError`, so ⌘K stopped working (I12).
   `init` now forgets disconnected palettes, the key loop and `open` skip them.
2. A full page carrying the palette's idle fragment was inserted as an answer,
   showing idle copy after a redirect. Only a response that is exactly one fragment
   for the requested query is inserted; `Results.Query` must be the request's query.
3. The catalog listed Fathom as an observed consumer though it has not integrated
   the palette (I8/I15); `consumers` is now empty, matching the inventory.
4. Zero results with a `Notice` announced a definitive "No results" and advised
   checking the spelling; it is now `partial` with a partial announcement.
5. Triggers inserted after wiring kept the server hint and lacked `aria-haspopup`;
   every `init` now upgrades all triggers, and withdraws the shortcut from triggers
   whose palette is gone.
6. Without JavaScript the trigger advertised Control+K; the server now renders no
   `aria-keyshortcuts` and a hidden, empty hint that only the script reveals.
7. `init(root)` ignored the `[data-palette]` element itself; it now wires it.
8. Shortcut matching fell back to the physical key even when a different Latin
   letter was typed (Dvorak ⌘T opened the palette); the fallback now applies only
   to non-letter, non-digit keys.
9. Common phrasings (keyboard shortcut, Cmd+K, Ctrl+K, global/site search, search
   modal, quick open, command menu) found nothing; reviewed task phrases now cover
   them (ADR-0062) and the shared selection corpus gains four cases.

The Gallery gains a reserved `unsearched` query so the zero-result partial state
stays reproducible.

## Converted-core-v1 gate decisions

Partial: structure, visual, interaction, lifecycle, security, data, reproducibility
(each scoped to the rows above and the deterministic checks). Not run: performance
(no workload or budget measured), integration (Fathom has not adopted this revision).
None passed; no reviewer has accepted these results.

## Not verified

Screen readers and announcement timing; Firefox/WebKit; touch, IME and real
Windows/Linux keyboards (D is emulated) or real non-QWERTY layouts (F uses synthetic
events); browser zoom; contrast ratios (visual only); performance and memory;
multiple palettes in use on one page (F only opens a second one); Fathom's own routes, CSS scan and
decision-port "Suggested view" line (an app-owned group). `overlay.RenderDialog`'s
shared close button is a 24px target, below the kit's 44px convention; Escape and the
backdrop also close. Built-in secondary copy is English. Screenshots were inspected
locally and are not retained.
