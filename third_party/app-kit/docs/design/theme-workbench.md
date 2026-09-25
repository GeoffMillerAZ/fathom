# Named themes and app-owned living style guides

Contract: [ADR-0029](../adr/0029-named-themes-and-app-owned-style-guides.md).
Observed implementation/test boundaries: [assessment](../assessments/2026-09-13-personal-themes.md).
Named identity and color scheme are separate. Choosing a preset does not make an app a
resume dashboard, change its data, or imply a light version of a dark-only design.

## Immediate implementation

The personal `geoff-synthwave` and `geoff-indigo` presets live under `cue/tokens/`.
Three first-party identities, `copper`, `lagoon` and `orchard`, add deliberately
different light and dark mappings: warm mineral/orange, cool teal and olive/leaf.
Their Go packages under `appkit/ui/themes/` embed generated stylesheets. Existing
`appkit` and `synthwave` stay independent, and the Gallery's default appearance stays
unchanged.
The Gallery `/themes` workbench previews actual token, component, form and dashboard
specimens with a bounded GET selection. It is not an arbitrary CSS editor or an app
preferences backend. Browser fidelity and consumer adoption remain separate evidence.

Go delivery: serve `geoffsynthwave.CSSHandler()` after the compiled utility stylesheet, or
import its `theme.css` after Tailwind in the app's CSS input. Portable delivery:

```sh
appkit vendor -out ./vendor/appkit --include tokens --theme copper
appkit vendor -out ./vendor/appkit --include tokens --theme copper --check
```

Use a fresh output or explicitly review replacement of an existing selected theme.
Keep `kit.lock`; do not hand-edit generated files. The default theme remains `appkit`.
An explicit `--theme ... --check` also compares the selected theme's current bytes;
without `--theme`, the existing disk-versus-lock integrity contract stays unchanged.
Vendoring still defaults to `--dark-variant none`; an app that uses the kit's attribute
strategy can select `attribute`. A named dark-only palette does not convert structural
`dark:` branches compiled for another strategy. Review those separately.

Color, font and radius utilities bind to `--ak-color-*`, `--ak-font-*` and
`--ak-radius-*`. Rebuild existing utilities once to gain font/radius indirection;
an already-built stylesheet with literal fonts/radii cannot learn the new bindings
from a palette swap. Consumers can use the variables directly without Tailwind.
Font scale remains `--ak-font-scale`; it is a separate preference, not an identity.

The three first-party palettes are CUE-authored in OKLCH and generated with the
existing command, with no runtime color parser or new dependency:

```sh
go run ./cmd/appkit gen --tokens cue/tokens/lagoon/lagoon.cue \
  --css appkit/ui/themes/lagoon/theme.css --dtcg '' --go '' --terminal ''
```

Their package test checks deterministic emitted CSS, embedded delivery and selected
text, control, status/chart and focus contrast pairs in both schemes. The
[frozen browser replay](../assessments/2026-09-16-paired-themes.md) adds 61 scoped
checks and 12 captures, not whole-screen accessibility or consumer adoption proof.

## Geoff source and adaptations

The owner-selected primary reference is committed `src/styles/global.css` at
`375defbac2c9e5c3fa01cada634464b95936f877`, SHA-256
`cb2d45d42bd73a3862ee01e7a9efec1be52272cceb7f2c315c21c22c0ebb51b3`.
`BaseLayout.astro` imports it and requests Inter from Google Fonts; the kit does not
copy that remote request. Deep purple `#241b2f`, pink `#f92aad`, pale magenta `#ff7edb`
and white `#ffffff` retain source values. Exact white is an explicit owner-requested
reference exception. Status/strong-boundary/soft-background roles are documented
semantic adaptations, not original site contracts. Existing kit Synthwave is untouched.

Optional `geoffsynthwave.NeonCSS()` / `NeonHandler()` supplies `.ak-neon-title`: static
3/6/9px glow bound to semantic ink. It is separate from palette CSS, has no animation
or font download, and switches off for print, forced colors and increased contrast.
Use on selected headings; avoid glowing tables, body copy and control labels. This
effect is not yet an `appkit vendor` group; Go can serve it or materialize NeonCSS().

Secondary reference checkout: `/Users/geoff/code/geoffmilleraz/geoff`, resolving to
`/Volumes/m2ext/code/geoffmilleraz/geoff`. The active Next layout imports `globals.css`
and renders `ResumeDashboard`. HEAD `375defbac2c9e5c3fa01cada634464b95936f877` does **not**
contain the current untracked migration files; these hashes identify the inspected input:

| Source member | SHA-256 |
|---|---|
| `apps/web/tailwind.config.js` | `19044ad6bca723d8a9fdfb3d6eb2a18ab4dfdc785f06d8a3a5222f8db86feabb` |
| `apps/web/src/components/react/dashboard/ResumeDashboard.jsx` | `21ab1c35df43e5d2dfae7458cbaa04b580d85becf9bce47e1814bce9df8a4800` |
| `packages/ui/src/components/application-ui/shells/SidebarLayout.jsx` | `436a2cc7368b454fdf347f70a46b9fff27c0841b36556c11d2d0b7c400b2d66a` |

The source uses Tailwind 3.4.17 gray-900/800 surfaces, indigo-400/500 accents,
Inter/system sans, 6/8/12px medium/large/extra-large corners and generous responsive
gutters. No active Inter font loader was found: the preset declares a stack, not a
font download or exact typography baseline. The kit owns font delivery explicitly.

Semantic adaptation is intentional: gray-50 replaces source white; gray-400 supports
readable secondary/faint text; dark ink on indigo preserves button-label contrast;
gray-500 provides a stronger control boundary. Status foreground/background pairs
complete kit roles but are kit assignments, not recovered website semantics. Preserve
canonical color values here instead of mechanically mapping all grays to one role.
Computed selected-pair contrast is not whole-screen accessibility certification.

The old `theme.yaml` pink/cyan/Orbitron configuration is not the active Next entrypoint
and does not override the committed older CSS's Inter body rule. Photos, personal records,
Unsplash fallback, Headless UI/Heroicons code and website dependencies are not imported.
The site's README license claim is not a verified license for every template or asset.

## App guide outcome, experimental local implementation

An app guide should make this loop straightforward:

1. Load the app's pinned kit selection, token source and real recipe/state fixtures.
2. Inspect default and candidate identity at narrow/wide sizes and supported schemes.
3. Preview explicit overrides without changing saved preferences or source files.
4. Compare color meaning, typography, shape, focus, density, missing/error states and
   analytics encoding; show font/assets and contrast failures, not just swatches.
5. Reset, validate and export a reviewable token diff with source/revision identity.
6. Rebuild in the app, verify its tasks, then contribute a reusable improvement back.

Keep app identity, route/auth/data policy and app-specific styles app-owned. Reuse
catalog IDs, specimen recipes and validation instead of creating another metadata copy.
The guide must work without paid corpora, an LLM, a live MCP or a mandatory frontend
runtime. Fonts need local dependency/license records; previewing a font must not fetch
it implicitly. Different layout/density/motion settings remain independent choices.
The [local app-owned guide](app-owned-styleguide.md) now implements bounded token
preview/reset/checks and acknowledged snapshot export, with app-owned callbacks.
Gallery opts in through `-theme-source`. This is experimental static/system-font
inspection; actual consumer adoption and browser task evidence remain G1/G4 work.

## Website patterns worth evaluating next

- Sidebar + sticky header + narrow-screen navigation: compare with the shared shell,
  retaining app-owned navigation and a complete keyboard/mobile contract.
- Search/result-count/faceted filter bar: combine with the analytical journey and URL
  state, not a second filter engine.
- Persona/context selection coordinating summary and detail: first prove a non-resume
  task, stable state ownership and accessible selection.
- Divided expandable rows with compact dates and inset detail: preserve composition,
  add proper expanded-state semantics; do not copy the source's missing `aria-expanded`.

These are documented candidates. No website component has been promoted or vendored.
Promotion/drop conditions live in [futures](../intent/futures.md).
