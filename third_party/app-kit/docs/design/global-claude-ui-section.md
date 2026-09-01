# Global CLAUDE.md — UI stack section (verbatim copy)

> **This is a copy of a file that lives outside every repo.** The live text is
> the "UI/UX stack (all projects)" section of `/Users/geoff/.claude/CLAUDE.md`,
> which nothing versions. This copy exists so the section is reviewable,
> diffable and recoverable; it is not loaded by anything.
>
> Landed 2026-08-07 with ADR-0017 and `app-kit/CLAUDE.md`. It says the same
> thing as the kit's own CLAUDE.md on purpose: `cmd/appkit-mcp/kitdocs.go`
> returns `app-kit/CLAUDE.md` **verbatim** as `get_kit_guidance`, so a session
> whose global file and MCP guidance disagree gets the older of the two.
>
> Verify the copy is current:
>
>     diff <(sed -n '/^## UI\/UX stack/,$p' ~/.claude/CLAUDE.md) \
>          <(sed -n '/^## UI\/UX stack/,$p' docs/design/global-claude-ui-section.md)

## UI/UX stack (all projects)

The licensed assets (TailwindPlus, MatDash) and app-kit only pay off if UI work
flows through one discovery funnel. **Common to every project: before writing UI
markup, query the `appkit` MCP** (registered user-scope; restart once if a
session doesn't see the tools). `search_components` returns the licensed
**TailwindPlus** and **MatDash** corpora; `get_component` fetches exact source;
`get_kit_guidance` / `get_kit_css_setup` / `get_kit_theming` / `list_kit_decisions`
return app-kit's own rules and ADRs. Reach for these before inventing anything.
**MatDash React/Tailwind/Vite Pro** lives at `app-kit/data/matdash/` (gitignored)
— `packages/starterkit` is the base for a new React app, `packages/main` has the
dashboards/widgets (Flowbite-React).

**app-kit is multi-target and layered (ADR-0017). Take any layer and stop —
stopping is a complete adoption, not a partial one.**

- **L0/L1 — tokens and CSS. Framework-neutral, every project.** `theme.css`
  (roles, radius, fonts, light/dark) and `motion.css` (two-state vocabulary,
  bounded stagger, the reduced-motion cancel) are **plain CSS with no Go in
  them**. A React app `@import`s them exactly as a Go app does. This is the
  default level of participation.
- **L2 — browser JS, any project without its own framework.** `router.js`,
  `overlay.js`, `chat.js`, `motion.js` — classic scripts, one global each, zero
  deps, no build step.
- **L3a — var-themed Go renderers (`appkit/ui/viz`).** Zero `class=`; colour via
  `var(--ak-color-*)` only, so a kit chart renders correctly themed with **no
  Tailwind and no build** — inject the SVG string.
- **L3b/L4 — utility-themed Go components + templ (`appkit/ui/*`). Go
  server-rendered apps only.** These emit Tailwind role utilities, so the
  consumer must run `appkit css` with an `@source` glob over the kit.

Get L0–L2 with `appkit vendor -out <dir> --include …`, which writes
`kit.lock`; run `appkit vendor --check` in CI or the copy drifts silently.
Vendor *into* the project root — Vite refuses CSS resolved outside it, and a
sibling-checkout relative path assumes a clone at a fixed depth.

**"No Node" was four rules. Keep them apart, because conflating them is what
produced the line this replaces:**
- The kit's own CSS comes from the SHA-256-pinned Tailwind standalone binary,
  re-verified every call — never npm. **Stands.** *This is what the rule
  protects: a clean-machine build that produces the same bits on any day.*
- No `package.json` in the app-kit monorepo or in a Go app whose primary
  artifact is a binary or an `http.Handler` tree. **Stands** — a rule about
  *those repos*.
- Kit-authored browser JS is allowed at escape-ladder rung L1 (ephemeral, no
  durable state, no build step, `go:embed`-ed, one global). **Amended by
  ADR-0017** — the kit already ships four such files. Third-party JS still
  enters only vendored, pinned and checksummed.
- *"React apps can't use the kit."* **Deleted. It was never true**, and never
  what the rule meant: ADR-0002's own Boundaries paragraph permits consuming
  Node-built artifacts, `spa/router.js` has been byte-identical kit code in
  production for months, and `geoffmiller-cloud` has no `package.json` anywhere.

**Path A — Go server-rendered (Throughline, `racli serve`, new Go work):**
L0–L4. Purchased templates are pattern sources, not runtime deps: pull
(framework `html`) → strip hardcoded colors → kit role utilities
(`get_kit_theming`: `bg-indigo-600`→`bg-accent`) → pure-Go render fn. CSS via
`appkit css` (`get_kit_css_setup`). Dashboards = MatDash shells + kit `ui/viz`
charts, re-themed.
**Path B — React/Node (pumpkin, MatDash-based):** L0–L3a. `@import` the kit's
`theme.css` and `motion.css` in your Tailwind entry so you theme and animate
identically to every Go app; generate or vendor with `--dark-variant none` or
your existing `dark:` utilities silently stop following the OS. Use
TailwindPlus/MatDash in their native React variants with the app's own Tailwind
config. **The kit's Go components don't apply — the kit does.**

**Promotion.** Build it in the app first, then promote **at the lowest layer
that carries it**: a colour is a token, a lesson is a row in
`app-kit/docs/theming-hazards.md`, an interaction is CSS plus a class
vocabulary, a calculation is a Go function — and only a whole rendered thing is
a Go component, which still needs **two Go consumers (D13)**. At L0–L3a one
consumer plus a written rationale is enough. Divergence is the system working:
document it with a `DIVERGENCE:` header naming what you declined and why.
