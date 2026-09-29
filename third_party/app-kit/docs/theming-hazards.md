# Theming and contrast hazards — the review checklist

Every hazard on this page shipped to production in a kit-adjacent app **and
survived code review**. None of them is exotic; each one is invisible in a diff
and obvious in a measurement. That asymmetry is the entire subject: theming bugs
do not look like bugs in source, so reading harder does not find them.

Run this list against any change that adds a color, a theme-scoped rule, an
opacity, or a mark that carries meaning. Each entry states **the hazard**, **how
it presents**, **why review misses it**, and **the fix**.

Conventions per [README.md](README.md): facts are dated, decisions are cited by
D-number. Measurements below were taken **2026-08-07** against token set
`appkit` ([../cue/tokens/kit.cue](../cue/tokens/kit.cue)) — re-derive them with
the commands in [§H3](#h3--measure-never-read), never quote them from here.

| # | Hazard | The 30-second check |
|---|---|---|
| [H1](#h1--a-theme-override-fires-under-every-palette-the-selector-does-not-name) | A theme override fires under palettes it doesn't name | `grep -rn 'data-theme="light"' appkit apps` → must be **only** the generated exclusion in `tokens.go` |
| [H2](#h2--equal-specificity-and-the-legend-that-disagrees-with-the-field) | Equal specificity; the generic rule wins because it is later | Does any later rule restate a property the state class owns? Does a legend restate a fill? |
| [H3](#h3--measure-never-read) | A contrast ratio was read, not measured | Does the claim name an instrument, a selector, a theme, and a number? |
| [H4](#h4--a-token-that-does-not-exist-fails-silently) | A role that does not exist, referenced anyway | `go test ./appkit/ui/tokens -run 'Dangling\|ThemeOnly'` |
| [H5](#h5--the-31-floor-and-why-opacity-encodings-fail-it) | A meaning-bearing mark below 3:1 once composited | `go test ./appkit/ui/tokens -run MeaningFloor -v` |
| [H6](#h6--a-caveat-set-quieter-than-the-claim-it-qualifies-is-dishonest) | The source line, the caveat or the draft badge is the faintest thing on the figure | `grep -rn 'ink-faint' appkit/ui/vizframe` → must be empty |

H4, the static half of H5, and H6 run inside `task check` today. H1, H2 and the
composited half of H3/H5 are human checks — do them.

---

## H1 — A theme override fires under every palette the selector does not name

**The hazard.** A rule scoped to a scheme selector (`[data-theme="light"] .mark
{ … }`) also applies under every *other* variant that remaps the palette without
touching that attribute. The rule was written for one palette and silently
governs all of them.

**How it presents.** Figure and ground invert. In the shipped case the chart's
ground measured **8.51:1** while the figure — the mark carrying the argument —
measured **3.39:1**: the background was more prominent than the data. It looks
"a bit washed out", which reads as a taste problem, not a defect, so it gets
deferred instead of fixed.

**Why review misses it.** The selector is *correct for the palette the reviewer
has in their head*. Nothing in the rule mentions the other variants, and the
other variants' code is somewhere else entirely — a different file, often a
different token set. Reviewing the rule in isolation can never surface the
interaction. It shipped twice.

**The fix, in the kit's own theming model.** The kit has no `data-aesthetic`
analogue. It has exactly one selector axis —
`data-theme ∈ {unset, light, dark}` — and the only light-side selector anywhere
is an *exclusion* generated at
[`appkit/ui/tokens/tokens.go`](../appkit/ui/tokens/tokens.go) (`EmitThemeCSS`):

```css
@media (prefers-color-scheme: dark) {
  :root:not([data-theme="light"]):not([data-theme="dark"]) { … }
}
```

The kit today has **zero** `[data-theme="light"] …` override rules. Keep it
that way, because the hazard already exists here without a new attribute:

> **A theme in the kit is a whole token set, not an override**
> ([`appkit/ui/themes/README.md`](../appkit/ui/themes/README.md)). An app swaps
> which `theme.css` it `@import`s, and *nothing in the DOM changes*.
> `appkit/ui/themes/synthwave` is dark-valued in **both** schemes — under it,
> `data-theme="light"` selects a **dark** palette. Any `[data-theme="light"]`
> color rule inverts figure and ground under synthwave the day it is written.

So the general rule — *a theme override must be gated against every variant that
remaps the palette* — has a sharper kit-native form, because the kit's
palette-remapping variants are **not selectable from CSS at all**:

> **Never key a color on a theme selector. Key it on a role token.**
> The correct number of scheme-gated color rules in kit code is zero.

| Instead of | Write |
|---|---|
| `[data-theme="light"] .mark { fill: #333 }` | `fill="var(--ak-color-ink)"` — the role already carries both scheme values |
| `[data-theme="light"] .panel { background: #fff }` | `class="bg-surface"` |
| a hand-written dark override | `class="text-ink-muted dark:text-ink"` — the `dark:` variant is generated *by the token set you shipped* (`@custom-variant dark`), so every theme defines it for itself |

There is deliberately **no `light:` variant**: the light values are `:root`, the
default. If you find yourself wanting one, you are about to write this bug.

**If an app does add an aesthetic axis** (the kit permits it — schema.cue's open
role map is the same "guide, don't dictate" hook), then every scheme-gated rule
must exclude every palette-remapping value, which is the fix that shipped:

```css
:root[data-theme="light"]:not([data-aesthetic="terminal"]):not([data-aesthetic="outrun"]) .mark { … }
```

Note the maintenance cost this creates: **every new aesthetic value must be
added to every such selector**, and forgetting one is silent. That cost is the
argument for keeping the count at zero.

---

## H2 — Equal specificity, and the legend that disagrees with the field

**The hazard.** Two rules of equal specificity; the later one wins. The later
one is usually the *generic* rule, because generic rules get appended last.

```css
.ublock.is-used              { opacity: 1;   }  /* (0,3,0) — the state that matters */
[data-theme="light"] .ublock { opacity: .24; }  /* (0,3,0) — later, so it wins */
```

**How it presents.** The load-bearing "used" mark rendered at **1.30:1**. The
entire point of the chart was invisible in light theme while every decorative
element around it was fine.

**Why review misses it.** Both rules are individually correct and read as
unrelated — one is about state, one is about theme. Specificity ties are
resolved by *source order*, which is not visible at either rule's location and
changes when files are reordered or concatenated. Nobody computes (0,3,0) twice
and compares while reading a diff.

**The fix (immediate).** Make the generic rule structurally unable to claim the
state the specific rule owns:

```css
.ublock:not(.is-used) { opacity: .24; }   /* the states are now disjoint, not racing */
```

**The fix (structural, and the one that actually holds).** The 1.30:1 mark had a
legend swatch that *looked* right, because the swatch restated its fill
independently of the field. **A legend must reuse the mark's classes — or be
produced by the same function as the mark — so that a key which disagrees with
the field is unrepresentable rather than merely unlikely.**

Kit-native form of both halves:

- **`ui/viz` marks carry presentation attributes** (`fill="var(--ak-color-…)"`,
  emitted by `cssColor` in
  [`appkit/ui/viz/svg.go`](../appkit/ui/viz/svg.go)). A presentation attribute
  has **specificity 0** — it loses to *any* stylesheet rule, including one an
  app writes months later (`.chart rect { fill: … }`). That is the same
  race, with the kit's marks pre-losing it. When an app must restyle a kit
  chart, it should choose the role at render time, not override the fill in CSS.
- **`ui/viz` has no legend component today.** When one is built, its swatch must
  call the *same* role/opacity function the mark calls — not a literal, not a
  parallel switch. The prior art is already exported for exactly this reason:
  `viz.GradedRole(frac)` ([`gauge.go`](../appkit/ui/viz/gauge.go)) maps 0..1 →
  `danger`/`caution`/`positive` and is public "so callers can color related
  elements consistently". A legend that calls `GradedRole` cannot disagree with
  a gauge that calls `GradedRole`.

**Review question:** for every mark that has a key, name the single expression
that produced both colors. If there are two expressions, the key can lie.

---

## H3 — MEASURE, NEVER READ

**The hazard.** Contrast is asserted from the source of a color rather than
computed from the pixels the browser actually produces.

**How it presents.** It doesn't. That is the whole problem. Two rounds of review
missed marks at **1.30:1** and **1.66:1** — both carrying the argument of their
chart. In one case a builder **reported validating a light-theme color ramp it
had never written**: the report described a ramp that did not exist in the
codebase, and the report was believed because it contained numbers.

**Why review misses it.** `oklch(0.70 0.14 80)` reads as "caution", not as
"2.60:1 on surface". Nobody composites alpha in their head. And a claim like
"validated in both themes" is indistinguishable, in a PR, from a claim that was
actually executed.

**The rule.** A contrast claim that does not name **the instrument, the
selector, the theme/token set, and the number** is not a claim. Re-measure;
never quote a table (including the ones on this page).

### Procedure A — static, no browser, no Node (D2)

For anything whose color is a token value composited over another token value:

```sh
# the kit's meaning-floor table, printed and asserted
go test ./appkit/ui/tokens -run TestKitPaletteMeaningFloor -v

# one pair, in code
ratio, err := tokens.ContrastRoles(tok, "caution", "surface", "light")   // 2.60
ratio, err := tokens.ContrastOver(accentLight, 0.12, surfaceLight)       // 1.19 — composited
```

[`appkit/ui/tokens/contrast.go`](../appkit/ui/tokens/contrast.go) implements
WCAG 2.1 over `oklch()` and hex, composites alpha the way browsers do (in the
gamma-encoded space, not linear), and **returns an error for anything it cannot
resolve** — `var(…)`, `color-mix(…)`, a named color, a translucent value with no
background. A measurement you could not make must fail loudly; silently
returning a passing number is the failure this file exists to prevent.

Measured floor table, token set `appkit`, 2026-08-07 (`caution` light is the one
role below the floor — see [Known open](#known-open-in-this-kit-keep-honest)):

| Role on `surface` | light | dark |
|---|---|---|
| `ink` | 16.97:1 | 15.80:1 |
| `ink-muted` | 7.12:1 | 7.27:1 |
| `ink-faint` | 3.78:1 | 4.00:1 |
| `accent` | 5.97:1 | 6.63:1 |
| `focus` | 4.81:1 | 7.70:1 |
| `positive` | 4.33:1 | 7.68:1 |
| **`caution`** | **2.60:1** | 9.62:1 |
| `danger` | 5.12:1 | 6.24:1 |
| `info` | 3.97:1 | 8.02:1 |

`line` (1.29:1 light) and `*-soft` (≈1.1–1.5:1) are **not** on this list on
purpose: they are hairlines and tints, which carry no meaning by themselves. If
a hairline starts carrying meaning, it needs a role that clears the floor.

### Procedure B — composited, in a real page

Procedure A cannot see ancestor `opacity`, `fill-opacity`, `color-mix()`, or
what is actually behind a mark. Those need the browser. Paste this into DevTools
on the Gallery (`task gallery:run`) — it measures **computed** styles, so the
browser has already resolved `var()`, `color-mix()`, and inheritance for you:

```js
// Composited contrast audit. Paste in DevTools; call with a selector.
// Reports every match against the first opaque background behind it.
(function (sel) {
  var px = document.createElement('canvas').getContext('2d', { willReadFrequently: true });
  function rgba(css) {                       // any CSS color -> [r,g,b,a] in 0..1, or null
    if (!css || css === 'none') return null;
    px.fillStyle = '#010203';                // sentinel: an invalid value leaves it in place,
    px.fillStyle = css;                      // so an unparseable color returns null, never black
    if (px.fillStyle === '#010203' && css.replace(/\s/g, '') !== '#010203') return null;
    px.clearRect(0, 0, 1, 1);
    px.fillRect(0, 0, 1, 1);
    var d = px.getImageData(0, 0, 1, 1).data;
    return [d[0] / 255, d[1] / 255, d[2] / 255, d[3] / 255];
  }
  function over(f, a, b) {                   // composite in the encoded space, as browsers do
    return [a * f[0] + (1 - a) * b[0], a * f[1] + (1 - a) * b[1], a * f[2] + (1 - a) * b[2], 1];
  }
  function lum(c) {
    var l = c.slice(0, 3).map(function (v) {
      return v <= 0.04045 ? v / 12.92 : Math.pow((v + 0.055) / 1.055, 2.4);
    });
    return 0.2126 * l[0] + 0.7152 * l[1] + 0.0722 * l[2];
  }
  function ratio(x, y) {
    var a = lum(x), b = lum(y), hi = Math.max(a, b), lo = Math.min(a, b);
    return (hi + 0.05) / (lo + 0.05);
  }
  function backdrop(el) {                    // walk up, compositing translucent backgrounds
    var stack = [], n = el.parentElement;
    for (; n; n = n.parentElement) {
      var c = rgba(getComputedStyle(n).backgroundColor);
      if (!c || c[3] === 0) continue;
      stack.push(c);
      if (c[3] === 1) break;
    }
    var base = stack.pop() || [1, 1, 1, 1];
    while (stack.length) { var t = stack.pop(); base = over(t, t[3], base); }
    return base;
  }
  var rows = [];
  document.querySelectorAll(sel).forEach(function (el) {
    var s = getComputedStyle(el);
    var isShape = el.ownerSVGElement && s.fill && s.fill !== 'none';
    var fg = rgba(isShape ? s.fill : s.color);
    if (!fg) return;
    var alpha = fg[3] * parseFloat(isShape ? s.fillOpacity : 1);
    for (var n = el; n; n = n.parentElement) alpha *= parseFloat(getComputedStyle(n).opacity); // the opacity chain
    if (alpha < 0.01) return;                // trap 2: skip the not-showing half of a two-state widget
    var bg = backdrop(el);
    var r = ratio(over(fg, alpha, bg), bg);
    rows.push({
      mark: el.tagName.toLowerCase() + (el.getAttribute('class') || el.id || ''), // trap 1: not el.className on SVG
      alpha: +alpha.toFixed(2), ratio: +r.toFixed(2),
      verdict: r >= 4.5 ? 'text ok' : r >= 3 ? 'mark ok' : 'FAILS 3:1'
    });
  });
  console.table(rows.sort(function (a, b) { return a.ratio - b.ratio; }));
})('svg [fill], .badge, .stat-delta');   // ← the marks that carry meaning
```

Run it **once per combination that ships**: `data-theme` unset / `light` /
`dark`, × every token set the app can load (kit default, `synthwave`, the app's
own), **× each interaction state a mark can be in** — the second state of an
`appkit/ui/motion` two-state widget, a hover, an expanded disclosure. The
Gallery switches `data-theme` server-side (`apps/gallery/render.go`), so the
theme loop is three URLs, not three builds; toggle `ak-state-b` (or the
interaction) between runs for the rest.

Two limits, stated honestly: it measures color math, not stacking — a mark
overlapped by a sibling, or drawn over an image or gradient, needs a screenshot
and a pixel sample instead. And `getComputedStyle` gives the *computed* value,
which is what you want, but a mark that is off-screen or `display:none` at
measurement time was not measured.

### Procedure B — the two traps in this snippet

Two mistakes are easy to make writing an audit like this, and both were live in
an earlier version of the snippet above. They interact — the fix for one does
not reveal the other — so both are called out here, each reproduced by running
the snippet verbatim on the Gallery.

1. **`el.className` is not a string on SVG.** On an HTML element `el.className`
   is the class string; on an **SVG** element it is an `SVGAnimatedString`
   *object*, whose `.baseVal` is `""` when the element has no class. The old
   label `el.className.baseVal || el.className` therefore fell through the empty
   `baseVal` to the object itself and printed `[object SVGAnimatedString]`. That
   is not cosmetic here: `appkit/ui/viz` marks carry presentation attributes and
   **no** classes, so on the Gallery — the target this doc names — most rows
   would share that one label and you could not tell which mark failed. Use
   `el.getAttribute('class') || el.id || ''`, which reads the same on both.

2. **A fully transparent mark scores a false FAIL.** The opacity chain folds in
   every ancestor's `opacity`, so the *not-currently-showing* half of any
   two-state widget (an `ak-state-b` group while state A shows) reaches `alpha
   ≈ 0`. Composited over its background at alpha 0 it *equals* the background,
   giving `ratio 1.00` → `"FAILS 3:1"` — a mark that is invisible, not
   unreadable. That produced six spurious rows per run, and is systemic now that
   `appkit/ui/motion` ships and every two-state widget emits an off-state. Skip
   `alpha ≈ 0` (the `if (alpha < 0.01) return;` line above), and measure each
   state in its *own* run rather than reading both at once — which is why the
   loop above now includes an interaction-state axis.

---

## H4 — A token that does not exist fails silently

**The hazard.** A reference to a role or custom property that nothing defines.
It fails in two directions, both quiet:

1. `var(--ak-color-typo)` with no fallback is **invalid at computed-value
   time**: the whole declaration is dropped and the property falls back to its
   initial value — a black `fill`, no border, no radius. (This is how the source
   site silently stripped every border and radius that referenced its undefined
   `--line-strong` and `--rad-md`.)
2. `text-neon-pink` where `neon-pink` is not in the active token set means
   Tailwind **never emits the utility at all**. The class is inert; the built
   CSS contains no trace of it to grep for.

**How it presents.** Nothing renders wrong enough to notice. No console error,
no invalid-property warning in DevTools' Styles pane for case 2 (there is no
rule to warn about), and the element is *almost* right.

**Why review misses it.** Case 1 looks like a correct `var()`. Case 2 looks like
a correct utility. Neither can be judged without the token set open beside the
component — and in case 2 the token set that defines the role *does exist*, just
not the one the app loads.

**State of this kit (verified 2026-08-07).**

- **No dangling custom properties.** In the built `apps/gallery/ui/app.css`,
  103 `var(--X)` references resolve against 110 definitions. Seven names are
  referenced without a definition and **all seven carry an explicit fallback**,
  which is correct usage, not a hazard: `var(--ak-font-scale, 1)` (the
  deliberate user hook), `var(--tw-duration, …)`, `var(--tw-ease, …)`, and four
  `--default-font-*` names — all Tailwind's own idiom.
- **One live instance of failure mode 2**, now pinned: see
  [Known open](#known-open-in-this-kit-keep-honest).

**The rule.** A component in `appkit/ui/` may reference only roles the **default
token set** defines (`tokens.ColorNames`). A role that only one theme defines is
an app-level choice, not a kit-level one. If a component must accept a
caller-supplied role, it emits a **fallback**, so an unknown role degrades to a
visible color instead of to `unset`.

**Detection.** Two tests, both in `task check`:

```sh
go test ./appkit/ui/tokens -run 'TestNoDanglingRoleReferences|TestNoThemeOnlyRoleUtilities'
```

- `TestNoDanglingRoleReferences` — every literal `var(--ak-color-<role>)` in
  kit and app source names a real role.
- `TestNoThemeOnlyRoleUtilities` — no `appkit/ui/` component styles itself with
  a role only a non-default theme defines. It carries a **ratchet**: a new
  occurrence fails, *and* removing a listed one fails too (with "delete the
  entry"), so the exception list can only shrink.

Ad-hoc, for a built stylesheet in any project:

```sh
comm -23 \
  <(grep -rhoE -- '--ak-color-[a-z0-9-]+' path/to/app.css | sed 's/--ak-color-//' | sort -u) \
  <(grep -oE   -- '--ak-color-[a-z0-9-]+:' appkit/ui/tokens/theme.css | tr -d ':' | sed 's/--ak-color-//' | sort -u)
```

Empty output is the pass. Anything printed is referenced but never defined.

---

## H5 — The 3:1 floor, and why opacity encodings fail it

**The rule.** **Anything that carries meaning clears 3:1** against the
background it is actually drawn on — a chart mark, a status dot, a graded fill,
a focus ring, a legend swatch, a threshold line. Text clears 4.5:1. The
constants are `tokens.MeaningFloor` and `tokens.TextFloor`; use them rather than
typing `3.0` into a new check.

**Why opacity encodings are the common failure.** Opacity is attractive for
intensity because it keeps a chart to one role and one hue (the kit does this
deliberately — `heatmap.go` modulates a single role, and that is a good
default). But **opacity is a move toward the background**: it reduces contrast
by construction, and it does so non-linearly in the encoded space. A role that
measures 5.97:1 at full strength does not measure 3:1 at half.

Measured, kit default token set, light scheme, 2026-08-07:

| `accent` over `surface` at | contrast |
|---|---|
| 1.00 | 5.97:1 |
| 0.70 | 3.29:1 |
| 0.55 | 2.46:1 |
| 0.50 | 2.25:1 |
| 0.24 | 1.44:1 |
| 0.12 | **1.19:1** |

`viz.Heatmap`'s ramp is `fill-opacity = 0.12 + 0.85·norm`. Its floor cell
measures **1.19:1**, and the ramp only clears 3:1 at **norm ≈ 0.63**
(`fill-opacity ≈ 0.66`) — so roughly the bottom two-thirds of the scale is below
the floor. `TestContrastOver_OpacityRampFallsBelowFloor` pins those numbers.

**How it presents.** The chart looks tasteful. Low values look "faint", which is
the intent — until you ask whether a reader can tell a low cell from an empty
one, or see it at all on a projector or a dimmed laptop.

**Why review misses it.** `fill-opacity="0.5"` reads as a styling choice, not as
a contrast decision. There is no number in the diff.

**The fix — pick one deliberately:**

1. **Decide whether the mark carries meaning.** A *track* behind a bar
   (`bars.go`, `rangebar.go`, both at `fill-opacity 0.5`) is scenery: it says
   "the rest of the scale", and nothing is lost if it is faint. A *cell* whose
   opacity **is** the value is load-bearing and must clear the floor. Same
   technique, opposite verdicts — the question is what the mark encodes.
2. **Floor the ramp.** Start it where it measures 3:1 rather than at 0.12
   (`ContrastOver` gives the crossing point per role and per scheme — it is not
   the same number in light and dark).
3. **Add a second channel.** Opacity plus a value label, a size, a position, or
   a border. Then opacity is redundancy rather than the sole carrier, and the
   floor applies to the channel that carries the meaning.
4. **Use distinct roles + a legend** — and per [H2](#h2--equal-specificity-and-the-legend-that-disagrees-with-the-field),
   build the legend from the same expression as the marks.

---

## H6 — A caveat set quieter than the claim it qualifies is dishonest

**The hazard.** A figure states a number in `text-ink`, and the line saying
where the number came from is set in `ink-faint` at `text-[0.625rem]`. Nothing
is false, and the page is still lying: the reader is being steered toward the
claim and away from its qualification, by typography rather than by words. The
same applies to a draft badge, a "not advice" disclaimer, an error state, and a
units label that changes what a number means.

**Measured**, kit default token set, 2026-08-07 (re-derive with
[§H3](#h3--measure-never-read); `vizframe`'s frame is `bg-surface-raised`):

| role | over `surface` | over `surface-raised` |
|---|---|---|
| `ink` | 16.97:1 | 17.46:1 |
| `ink-muted` | 7.12:1 | 7.33:1 |
| `ink-faint` | **3.78:1** | **3.89:1** |

Dark: `ink` 15.80 / 14.73, `ink-muted` 7.27 / 6.78, `ink-faint` **4.00 / 3.73**.
`ink-faint` is below `tokens.TextFloor` (4.5) on every one of those grounds. It
is a legitimate role for scenery — an axis tick, a placeholder, a row of
inactive metadata — and it is the wrong role for anything a reader needs in
order to judge what they are looking at.

**How it presents.** It looks tasteful. Quiet metadata is a real design idiom,
and this is what the idiom looks like when it is applied to the wrong sentence.

**Why review misses it.** `text-ink-faint` on a caption is indistinguishable in
a diff from `text-ink-faint` on an axis label. The judgement is not about the
class; it is about whether the sentence qualifies a claim, which the diff cannot
see. And the honest version reads as "shouting" to whoever wrote the quiet one.

**The rule.** A qualification is set no fainter than one step below the thing it
qualifies, and never in `ink-faint`. In practice: `ink-muted`. Smaller is fine —
the size difference is a legible hierarchy at any contrast; the contrast
difference is a legibility cliff.

**In this kit.** `appkit/ui/vizframe`'s source line is `text-ink-muted` and
`TestSourceLineNeverUsesInkFaint` fails if any part of the frame is set in
`ink-faint`. Provenance is the whole reason that package exists; a provenance
line the reader skips is worse than none, because it also confers the appearance
of rigour.

    grep -rn 'ink-faint' appkit/ui/vizframe    # must print nothing

`appkit/ui/callout`'s Details affordance was the same defect in a control: the only
label that reveals the elaboration, set in `ink-faint` (3.16:1 light and 2.88:1 dark
on `danger-soft`). It is `ink-muted` now, and `TestDetailsLabelClearsTheTextFloorOnEveryKind`
measures it over every kind's soft ground in both schemes (FATHOM-F0-KIT-06).

---

## Known open in this kit (keep honest)

Found by writing this page; **reported, not fixed**, because each is a
deliberate design change rather than a typo — and per the kit's own convention,
a doc must not claim more than the code does. Each is pinned by a test that
fails if it silently changes.

| # | Finding | Measured | Where | What a fix looks like |
|---|---|---|---|---|
| 1 | `caution` on `surface` fails the meaning floor in light | **2.60:1** | [`cue/tokens/kit.cue`](../cue/tokens/kit.cue) `caution.light = oklch(0.70 0.14 80)` | Darken the light-scheme caution — it crosses 3:1 at `oklch(0.66 0.14 80)` (3.03:1) and reaches 4.16:1 at `oklch(0.58 …)` — or stop using it alone for meaning. Token change → regen + Gallery rebuild; pinned in `knownBelowFloor`. |
| 2 | `hover:text-neon-pink` is inert under every theme but synthwave | n/a — the utility is never generated | [`appkit/ui/kanban/kanban.templ`](../appkit/ui/kanban/kanban.templ) card title | Use a default-set role (`hover:text-accent`) in the kit and let synthwave re-map it; pinned in `themeOnlyUsage`. Violates the H4 rule today. |
| 3 | `viz.cssColor` validates role *syntax*, never *existence*, and emits no fallback | — | [`appkit/ui/viz/svg.go`](../appkit/ui/viz/svg.go) | Emit `var(--ak-color-<role>, var(--ak-color-ink-muted))`. Note a strict membership check would break an app's legitimately added open role (`schema.cue`'s open map), so the **fallback** form is the one compatible with "guide, don't dictate". Touches every golden file — deliberate, not incidental. |
| 4 | `viz.Heatmap`'s opacity ramp is below 3:1 for ~⅔ of its range | 1.19:1 at the floor cell | [`appkit/ui/viz/heatmap.go`](../appkit/ui/viz/heatmap.go) | A `MinOpacity` option defaulting to the measured crossing point, or pairing the ramp with `ShowValues`. |

---

## The five-minute audit

```sh
# 1. No scheme-gated color rules outside the generated exclusion (H1).
grep -rn 'data-theme="light"' appkit apps --include='*.go' --include='*.templ' --include='*.js' --include='*.css'

# 2. Roles exist, and no component depends on a theme-only role (H4).
go test ./appkit/ui/tokens -run 'TestNoDanglingRoleReferences|TestNoThemeOnlyRoleUtilities'

# 3. The palette clears the meaning floor, and print the table (H3, H5).
go test ./appkit/ui/tokens -run TestKitPaletteMeaningFloor -v

# 4. The whole gate (includes 2 and 3).
task check
```

Then, by hand, for the change in front of you:

- [ ] **H1** — does any new rule name a scheme? If yes, it is wrong until proven
      otherwise; state which palettes it fires under.
- [ ] **H2** — does any later rule restate a property a state class owns? Does
      any legend/key restate a color instead of reusing the mark's expression?
- [ ] **H3** — for every meaning-bearing mark the change adds or touches, is
      there a measured number, from a named instrument, per theme **and** per
      token set? Composited marks measured in a browser, not in your head.
- [ ] **H4** — every role referenced is in the default token set, or the
      reference carries a fallback.
- [ ] **H5** — every meaning-bearing mark ≥ 3:1 (text ≥ 4.5:1) at its **lowest**
      opacity, not at full strength.

Agents get the condensed form of this page through the Assistant's
**`get_kit_theming`** tool (`cmd/appkit-mcp/hazards.go`), which is the surface
you are already at when you are choosing colors for adapted markup. The full
checklist is here; the wiring contract it depends on is `get_kit_css_setup`.
