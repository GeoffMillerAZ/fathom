# motion-provenance — what shipped, what is held back, what the verify pass left open

The motion vocabulary, the reduced-motion contract, the hazard checklist and the
provenance frame are **built and green** (`go test ./appkit/ui/motion/…
./appkit/ui/vizframe/… ./appkit/ui/tokens/… ./cmd/appkit-mcp/…
./appkit/ui/explainer/…` all `ok`). This plan does not re-plan them. It records
the claim so the index can gate it, states what is deliberately unwired, and
lists the seven things an independent verify pass found still open.

Gate: `fathom verify --service app-kit --plan docs/plans/motion-provenance.md`

---

## Decisions taken

1. **`vizframe` stays unwired until ADR-0017 is accepted.** Its package doc
   already says so; adopting it before the ADR would make the ADR a formality
   over a fait accompli. Its Gallery page and coverage test are owned by
   `multi-target-kit.md` (H-1, H-2). *Reverse:* move H-1/H-2 here and drop U-1.
2. **The `flush()` SVG defect is fixed with `getBoundingClientRect()`, and
   `motion_test.go:309`'s `offsetWidth` assertion is rewritten** rather than the
   helper being kept API-compatible. The test currently pins the bug in place.
   *Reverse:* revert both edits together — they are one change.
3. **The `caution` role's 2.60:1 in light is left failing and pinned**, not
   fixed. Changing a palette hue is Geoff's call, not a plan's; the test that
   records it is more valuable than a guess. *Reverse:* flip F-7 to `[ ]` and
   pick a hue.
4. **`explainer`'s `animation:none` is kept and its divergence from
   `motion.css`'s 1ms is documented, not reconciled.** Its loops are infinite and
   decorative with nothing awaiting a `transitionend`, which is the one case
   where `none` is safe. *Reverse:* change one line in `baseCSS` to `1ms`.
5. **No new hazard entries.** H-001…H-005 exist with measured evidence; adding
   more from a verify pass that could not run the build would be unmeasured
   prose. *Reverse:* add H-006 with its measurement.

---

## Shipped and green

- [x] `appkit/ui/motion` ships the two-state vocabulary with a **bounded** stagger — `MaxStep = 8` makes "648ms of tail across 216 nodes" inexpressible rather than clamped {id: M-1, symbol: Step, package: appkit/ui/motion}
- [x] The reduced-motion cancel is **welded to the class that grants motion**: every `transition-*` declaration is scoped to `.ak-motion`, which is the exact selector the reduced-motion block cancels — including transitions the consumer wrote inside it {id: M-2, symbol: CSS, package: appkit/ui/motion}
      → measured in a real browser with `page.emulateMedia({reducedMotion})`: `.ak-tint` 0.45s → 0.001s; a consumer's own transition *inside* `.ak-motion` 0.9s → 0.001s; the same rule *outside* `.ak-motion` left at 0.9s, untouched. State opacities and fills identical in both modes — only timing changes, never state or colour. The `1ms`-not-`0s` choice is empirically justified: `transitionend` fires at 7ms under `reduce`, and **never fires at all** under a counterfactual `0s`, hanging any JS awaiting it.
- [x] `motion.js` ships as a classic script registering one global, `//go:embed`-ed with `Script()`/`Handler()` {id: M-3, symbol: Script, package: appkit/ui/motion}
- [x] `appkit/ui/vizframe` exposes one predicate at three strengths — `Validate() error` (inspect), `Render() string` (guard, returns `""` rather than lie), `MustRender() string` (panic) — as a sibling package importing nothing from `appkit/ui/viz`, so 0 of 41 `viz` call sites are touched {id: M-4, symbol: Options.Validate, package: appkit/ui/vizframe}
- [x] `Options{}` (zero value) is `CarriesFigure` and strict; the `CarriesMechanism` waiver is visible in a diff, in a grep, and in the DOM as `data-viz-carries="mechanism"` — silence is strict, the escape hatch is one you have to write down {id: M-5, symbol: Render, package: appkit/ui/vizframe}
- [x] WCAG contrast is measurable in Go with no browser and no Node; values that cannot be resolved return an error rather than a ratio {id: M-6, symbol: Contrast, package: appkit/ui/tokens}
      → independently reproduced in a browser through a real composited canvas, agreeing to ±0.06 on all ten roles in both schemes and **exactly** on all six points of the accent/surface opacity ramp (1.00→5.97, 0.70→3.29, 0.55→2.46, 0.50→2.25, 0.24→1.44, 0.12→1.19).
- [x] `ContrastOver` covers the composited case (alpha over a known ground), which is where the opacity-encoding failures live {id: M-7, symbol: ContrastOver, package: appkit/ui/tokens}
- [x] `docs/theming-hazards.md` exists with H1–H5, each carrying its measured evidence and the command that catches it — the file `tokens/contrast.go:11` was already citing before it existed {id: M-8, symbol: "Theming and contrast hazards — the review checklist", package: docs, min_tier: doc-claimed, skip_test: true, skip: "markdown checklist; M-9 asserts the Go surface stays in sync with it"}
      → H1 confirmed empirically, not just argued: under synthwave with `data-theme="light"` forced, `--ak-color-surface` resolves to `#0e0b1a`, luminance 0.0041 — a *dark* palette under a light attribute.
- [x] The hazard checklist rides along with `get_kit_theming` rather than waiting behind a tool an agent has no reason to call, and its floors come from `tokens` so the two cannot disagree about a number {id: M-9, symbol: themingHazards, package: cmd/appkit-mcp}
- [x] `appkit/ui/explainer` carries a documented exemption from `ui/motion` plus its own reduced-motion escape in `baseCSS` {id: M-10, symbol: Render, package: appkit/ui/explainer}

## Deliberately unwired

- [-] Adopt `vizframe` — Gallery page, `@source` entry, `docs/ui-components.md` row, coverage assertion {id: U-1, disposition: deferred, reason: "the package doc defers adoption to ADR-0017; the work is owned by docs/plans/multi-target-kit.md tasks H-1 and H-2", trigger: "ADR-0017 accepted (multi-target-kit.md B-1)"}

## Follow-ups the verify pass left open

- [x] Fix `AppKitMotion.flush()`: `motion.js:52-54` reads `offsetWidth`, an `HTMLElement` property that is `undefined` on `SVGElement`, so reading it forces no style recalculation and the helper does nothing on the package's own primary use case; replace with `getBoundingClientRect().width` and rewrite the `strings.Contains(js, "offsetWidth")` assertion at `motion_test.go:309` that currently pins the bug {id: F-1, symbol: TestScript_FlushForcesLayoutOnSVG, package: appkit/ui/motion, skip_test: true, skip: "the deliverable IS the assertion; Script already exists, so anchoring on it would pass before the fix"}
      → **proves it:** measured in Chrome — `flush(svgG)` and `enter(svgG)` **silently did not run**; `flush()` with no arg and `frame()` did; all five ran on an HTML `<div>`. `typeof (SVG <g>).offsetWidth === "undefined"`. The verified fix flips both SVG cases to RAN. This bites the documented example at `motion.go:26-29`, which uses `<g class="…">`. New assertion: the script contains `getBoundingClientRect` and does not contain `offsetWidth`.

- [x] Add `/motion` to the Gallery — `buildMotion` rendering one SVG grid and one HTML grid with a toggle, both states legible with motion off — and add `@import "../../../appkit/ui/motion/motion.css";` to `apps/gallery/ui/input.css` after the theme import, then rebuild and commit `apps/gallery/ui/app.css` {id: F-2, symbol: buildMotion, package: apps/gallery}
      → **proves it:** `TestPagesServe` covers `/motion`. **No `@source` entry for `motion/*` —** empirically unnecessary: the rules are plain class selectors and were verified to resolve with no Tailwind scanner involved at all. This is also ADR-0012's gate, which needs no amendment: a page rendering a static specimen of `motion.css` *is* a Gallery page.

- [x] Add a motion section to `get_kit_css_setup` — the `@import` line, the class vocabulary, `RootClass`/`Step`/`DelayStyle`, and the fact that no `@source` entry is needed; `cssSetupGuide()` currently ends at "Verify it worked" with zero motion content {id: F-3, symbol: TestCSSSetupGuideCoversMotion, package: cmd/appkit-mcp, skip_test: true, skip: "the deliverable IS the assertion; cssSetupGuide already exists"}
      → **proves it:** a test asserts the guide contains `motion.css` and `ak-motion`. Together with F-4 this is the entire agent-facing discovery story; without it an agent cannot find any of this work.

- [x] Register the hazard doc with the decision loader — `loadExtra("hazards", filepath.Join("docs", "theming-hazards.md"))` in `loadKitDocs`, so `get_kit_decision("hazards")` resolves {id: F-4, symbol: TestFindDecision_Hazards, package: cmd/appkit-mcp, skip_test: true, skip: "the deliverable IS the assertion; loadKitDocs already exists"}
      → **proves it:** a test asserts `findDecision("hazards")` returns a non-empty doc. Then `task mcp:build` — the running MCP server is on a stale binary, so even the hazards work that *is* wired is unreachable until it is rebuilt and the client restarted.

- [x] Fix both traps in `docs/theming-hazards.md` §Procedure B and record them in a new "Procedure B — the two traps in this snippet" subsection: (1) the mark label `el.className.baseVal || el.className` yields `[object SVGAnimatedString]` for any SVG element with no class — `baseVal` is `""` → falsy → falls through to an `SVGAnimatedString` object; use `el.getAttribute('class') || el.id || ''`. (2) the opacity chain multiplies in the element's own `opacity`, so the not-currently-showing half of any two-state widget yields `alpha 0 → ratio 1.00 → "FAILS 3:1"`; skip `alpha ≈ 0`, and run once per interaction state rather than only once per `data-theme` × token set {id: F-5, symbol: "Procedure B — the two traps in this snippet", package: docs, min_tier: doc-claimed, skip_test: true, skip: "DevTools snippet inside a markdown doc; no Go symbol can reach it, and the new subsection only exists after the fix"}
      → **proves it:** both reproduced by running the snippet verbatim. Trap 1 is not cosmetic — `appkit/ui/viz` marks carry presentation attributes and **no** classes, so on the Gallery (the target the doc names) most rows would be labelled `[object SVGAnimatedString]` and you could not tell which mark failed. Trap 2 produced six spurious rows per run, and is systemic now that `appkit/ui/motion` ships: every two-state widget emits false failures. The two contributions interact and neither author accounted for it.

- [-] Fix `caution` at 2.60:1 against `surface` in light {id: F-7, disposition: deferred, reason: "real, independently reproduced by two instruments, and correctly pinned by TestKitPaletteMeaningFloor; changing a palette hue is Geoff's call", trigger: "Geoff picks a caution hue that clears 3:1 in light without breaking the dark branch (currently 9.62:1)"}

- [x] Assert `explainer`'s reduced-motion escape so a future edit to `baseCSS` cannot silently drop it, and record in the package doc why it uses `animation:none` where `motion.css` argues for `1ms` — the loops are infinite and decorative with nothing awaiting a `transitionend` {id: F-8, symbol: TestRender_ReducedMotionEscape, package: appkit/ui/explainer, skip_test: true, skip: "the deliverable IS the assertion; explainer.Render already exists"}
      → **proves it:** `TestRender_ReducedMotionEscape` asserts the output contains `prefers-reduced-motion:reduce` and that `.live` and `.dash` still carry their static fallbacks (`background:var(--green)` + `border-radius:50%`, `stroke-dasharray:6 6`) so both marks stay legible. Today nothing pins any of this.

- [x] Check whether the `appkit-mcp` binary at the repo root is gitignored; if it is not, remove it from tracking {id: F-9, symbol: TestNoCommittedBinaries, package: cmd/appkit-mcp, skip_test: true, skip: "the deliverable IS the assertion"}
      → the binary was **already** gitignored (`.gitignore:20` `/appkit-mcp`) and nothing binary is tracked; the deliverable is the guard test `TestNoCommittedBinaries`, which walks every git-tracked file and fails on any executable magic (ELF/Mach-O/PE), so a future committed build output is caught.
      → **proves it:** the test walks the repo root and fails on any tracked file matching a Go build output. The stale binary is also why the live MCP serves pre-contribution guidance (see F-4).
