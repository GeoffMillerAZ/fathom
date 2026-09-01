# The Gallery — the kit's living style guide (D12)

A kit app, built on the kit, that *is* the proof of the kit (VISION #2): every
design token visualized and every `appkit/ui/viz` primitive rendered live,
themed through the kit's token CSS. When you want to know how the kit looks or
that a component works, this is the catalog.

It is one `http.Handler` (the Loopback Invariant, D5) — `gallery.New()`. The
`cmd/gallery` runner is the thinnest possible shell around it, bound to
loopback only.

## Run it

```sh
task gallery:css     # build the CSS with the pinned Tailwind binary (D2 — no Node)
task gallery:run     # serve on http://127.0.0.1:8765
```

`gallery:css` only needs re-running when the token set or the page templates
change; the built `ui/app.css` is committed so the app runs out of the box.

## What's here (v0)

- **Tokens** — every color role (`tokens.ColorNames`) rendered as a live swatch
  (fill + text + border), the radius scale (`rounded-*`), and font specimens.
  The page is generated from the token vocabulary, so adding a role in CUE and
  regenerating makes it appear here automatically (a test enforces coverage).
- **Components** — the five `ui/viz` primitives (Sparkline, StackBar, Donut,
  Funnel, MeterRow) with representative data and the call that produced each.
- **Theme switch** — light / dark / OS, applied server-side via `data-theme`
  on `<html>`. Flip it and watch every role re-bind through `--ak-color-*` with
  no rebuild — the token system's whole point, made visible.

## How the CSS works (the pattern every kit app uses)

`ui/input.css` is the canonical shape: `@import "tailwindcss"`, then the
generated kit layer (`appkit/ui/tokens/theme.css`), then the sources to scan.
The Gallery builds role utilities *by name* at render time (`bg-` + role), which
the Tailwind scanner can't see as literals — so `input.css` uses
`@source inline(...)` (brace-expanded) to force-generate exactly the kit's token
vocabulary. A test (`TestInputCSSCoversTokenVocabulary`) keeps that safelist in
sync with the generated token names. This is the same contract the Assistant's
`get_kit_css_setup` tool documents for any consuming project.

## Not yet (arrives with the Phase 1 spine)

Knob controls generated from typed templ parameters, service exercisers (bus /
store / jobs / dialogs), vendored interactive components, and the Playwright
visual-regression baseline (ROADMAP Phase 3 / Gate G3). This v0 stands up the
catalog and proves the token + viz + CSS chain end to end.
