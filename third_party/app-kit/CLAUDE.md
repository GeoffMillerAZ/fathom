# CLAUDE.md — app-kit

This is **the Kit**: the personal product platform every app of Geoff's
imports. Read in this order: [VISION.md](VISION.md) (north star),
[docs/DECISIONS.md](docs/DECISIONS.md) (the D-register, binding),
[ROADMAP.md](ROADMAP.md) (gates), [docs/design/repo-layout.md](docs/design/repo-layout.md)
(where everything lives). ADRs in `docs/adr/` carry the reasoning — cite them
by number.

## Hard rules (the ones agents trip on)

1. **No npm toolchain, ever (D2, as split by D17).** CSS comes from the
   checksum-pinned Tailwind standalone binary via `appkit css` /
   `task css:build`. A `package.json` anywhere in **this repo** outside
   `apps/gallery/visreg/` is a build failure by policy. What that rule does
   **not** say: it does not forbid hand-authored kit browser JS (`chat.js`,
   `overlay.js`, `router.js`, `motion.js` are all L2 artifacts — ADR-0017 §3
   retired ADR-0002 §3's first clause), and it does not bind a consumer repo
   that is not a Go app. A React/Vite app importing `theme.css` is a
   compliant consumer.
2. **CUE is the source of truth (D7).** Schemas/tokens/manifests are authored
   in CUE; generated artifacts are committed; `task check` fails if
   regeneration isn't byte-identical. Never hand-edit a `*_gen.go`,
   `theme.css`, or `tokens.dtcg.json`.
3. **Checksums are load-bearing.** `internal/tailwind/manifest.cue` pins the
   binary per platform from the release's official `sha256sums.txt`. A
   mismatch must stay a hard failure — never "skip verification".
4. **Apps import the kit; nothing is copied sideways (D13).** A component
   used by two apps moves UP into `appkit/ui/<component>/`. D17 indexes that
   bar to the layer: two consumers is the L3b/L4 bar and is unchanged, while
   a token, a CSS class, a browser script or an L3a renderer needs one
   consumer plus its fixture. Promote at the lowest layer that carries it.
5. **The Loopback Invariant (D5)** governs everything runtime: every kit app
   is one Go `http.Handler`; shells are chrome. No shell-specific JS bindings
   in app code, ever.
6. **Licensed data stays caged.** `data/tailwindplus/*.json` is licensed,
   gitignored, never redistributed; its tooling lives under `internal/twplus/`
   so importers can't reach it.

## Layout deltas from repo-layout.md (deliberate, recorded)

- The Tailwind pin lives at `internal/tailwind/manifest.cue` (embedded in the
  binary — apps run `appkit css` from the module, so the manifest must ship
  inside it), not `tools/tailwind/manifest.cue`.
- The token schema lives at `appkit/ui/tokens/schema.cue` (embedded, so app
  token sets validate through the module), with the kit's default values at
  `cue/tokens/kit.cue`.

## Workflow

- `task gen` — regenerate token artifacts. `task check` — the full gate
  (vet, tests, cue vet, regen cleanliness). `task css:build` — kit CSS.
- Every commit that changes generated outputs includes both the CUE source
  and the regenerated artifacts (the check enforces it).
- Tool invocations: `go tool templ ...`, `go tool cue ...` (pinned via
  go.mod tool directives — never a globally installed version).
- The Throughline rebuild protocol is
  [docs/design/throughline-rebuild.md](docs/design/throughline-rebuild.md);
  Throughline feedback lands here as kit backlog items, built in the kit
  when reusable.

## Current truth (keep honest)

Pre-Phase-1 on the platform spine. Done: the CSS/token chain (Spike 0.3,
verdict in `spikes/0.3-tokens-css/README.md`). Not done: spikes 0.1/0.2,
appkit/app, shell, bus, store, web.

**Gallery v0 is built (owner-directed 2026-06-16, ahead of Phase 3):**
`apps/gallery` — the living style guide (D12), one `http.Handler` (D5)
rendering every token role/radius/font and every `ui/viz` primitive themed,
with server-side `data-theme` switching, CSS via the pinned binary
(`task gallery:css` / `gallery:run`). v0 = tokens + viz only; knob-gen from
templ params, service exercisers, and the Playwright baseline (Gate G3) still
need the Phase 1 spine. The token generator now exports
`RadiusNames`/`FontNames` (not just `ColorNames`); the Assistant gained
`get_kit_css_setup` (the no-Node CSS-layer contract a consumer needs for role
utilities to render) and `get_kit_theming`. `ui/viz` gained a `Timeline`
(events/spans/markers; proportional/sequence/condensed views, the condensed one
eliding empty gaps with honest duration labels) — built for Throughline-class
timeline needs.

**Out of roadmap sequence (owner-directed 2026-06-15): the D10 component
pipeline + D11 Assistant MVP are built and working.** `internal/twplus/`
(parse → CUE-validate → normalize → SQLite index: structural + FTS5 BM25 +
zstd code blobs) compiles the licensed dump via `appkit data build`;
`cmd/appkit-mcp` serves it plus the kit's ADRs over MCP (stdio/HTTP) for any
sibling project. `appkit data verify` proves the index byte-faithful to the
dump (10,452/10,452). Semantic search is **built** (`internal/twplus/embed` +
vec1 hybrid RRF in the index; `appkit data embed`, auto-used by
`search_components`) but ships dormant: BM25 + structural is the live default
until you run `data embed` against a local Ollama (additive, never a dependency,
ADR-0010). NOT yet done: Parquet durable layer; real embedding vectors in this
env (no Ollama); parity-retirement of the legacy `tailwindplus` server. The
corrected dump schema and MVP deviations live in
`docs/design/component-pipeline.md` §9 (semantic specifics + deviations in §10).
Do not claim beyond this in any doc or tracker. Next steps:
`docs/NEXT-SESSION.md`.

## Dashboards (appkit/ui/dashboard + MatDash)

Turnkey dashboard layout lives in `appkit/ui/dashboard`: `RenderShell` (responsive
sidebar + topbar + content area; sidebar collapses to a no-JS `<details>` "Menu"
below `md`) + `RenderWidget` / `RenderWidgetGrid` (responsive card grid, column
spans). Token-themed, pure-Go, loopback-safe (D5). Shown on the gallery Dashboard
page.

The dashboard package owns **layout only**. Charts, KPIs, and tables inside widgets
come from existing kit packages: `ui/viz` (SVG charts), `ui/stat` (KPI band/tiles),
`ui/table`.

- **Go server-rendered dashboards (L3b/L4, e.g. Throughline):** build with
  `appkit/ui/dashboard` + the primitives above. The shell was adapted from MatDash
  and re-themed to kit tokens (no npm toolchain, D2).
- **React dashboards (e.g. pumpkin):** use MatDash directly — base the app on
  `data/matdash/packages/starterkit`, pull dashboards/widgets from `packages/main`
  (React + Vite + Tailwind + Flowbite-React) — and take the kit at L0/L1 so it
  themes identically. See the layers below.

MatDash (purchased) lives at `data/matdash/` (gitignored, licensed — D6/D10). Its
React components are searchable via `search_components` (source=matdash). Full plan:
`docs/design/dashboards-and-matdash.md`.

## The layers — the kit is multi-target and layered; it guides, it does not dictate (D17 / ADR-0017)

**The kit publishes artifacts in layers. A consumer takes any prefix and stops.
Stopping is a complete adoption, not a partial one.** Adoption is measured by
layer-weighted reach — six of eight consumers are at L1 today — never by
component-import counts.

| L | What | Contract | Who can take it |
|---|---|---|---|
| **L0** | CUE token sets (`cue/tokens/*`, `ui/tokens/schema.cue`) | `#Tokens`: 23 required roles + the open-role rule | the generator |
| **L1** | Portable CSS (`ui/tokens/theme.css`, `ui/themes/*/theme.css`, `ui/motion/motion.css`) | `--ak-*` custom properties, fixed class names, **no literal colours**, ≤ one class of specificity, no preflight | anything with a `<link>` |
| **L2** | Portable browser JS (`ui/{router,overlay,chat,motion}/*.js`) | zero deps, no build step, one `window.*` global, `go:embed` + `Script()`/`Handler()` | any `<script>` |
| **L3a** | Var-themed Go renderers (`ui/viz`) | `func(data, Options) string`; **zero `class=`**; colour only via `cssColor(role)` → `var(--ak-color-*)`; **needs no Tailwind scanner** | any Go program, incl. a build-time tool in a non-Go repo |
| **L3b** | Utility-themed Go renderers (`ui/{callout,card,chrome,dashboard,explainer,export,form,hitl,kanban,stat,table,topology,vizframe}`) | emits Tailwind role utilities; the consumer **must** run `appkit css` with an `@source` glob over the kit | Go apps running the kit CSS build |
| **L4** | templ components (`*.templ` in the L3b packages) | D4, D5 | Go server-rendered kit apps only |
| **L5** | Discovery (`cmd/appkit-mcp`, `docs/`) | knowledge queryable mid-task (D15) | agents |

**The L3a/L3b split is the one an agent gets wrong.** `ui/viz` emits zero
`class=` and themes purely through custom properties, so a kit chart drops into
a page with **no Tailwind and no build** — inject the SVG string. Everything
else emits role utilities and needs the scanner. When you add a component,
pick a tier deliberately and put its row in `docs/ui-components.md`;
`TestVizEmitsNoClassAttribute` is what stops `viz` drifting into L3b by
accident.

**Guide, do not dictate — as constraints on the kit, not on apps.** Never
require a consumer to run Node or emit an artifact only a bundler can consume
(emitted CSS is valid in a `<link>`, emitted JS is a classic script, emitted
data is JSON, emitted markup is a string). Never emit a literal colour.
Never exceed one class of specificity, use `!important` outside an
accessibility floor, or ship preflight. Never enforce an editorial rule inside
a shared renderer — opinions live in wrappers a consumer can refuse
(`ui/vizframe`, not a `Source` field on all fifteen `viz.*Options`).

**Distribution to a non-Go consumer:** `appkit vendor -out DIR --include
tokens,motion,router,overlay,chat [--dark-variant none|attribute] [--check]`.
It writes only L0/L1/L2 artifacts plus a `kit.lock` (module version + SHA-256
per file); `--check` exits 1 on drift. Use `--dark-variant none` (the vendor
default) for any app with its own `dark:` utilities — `@custom-variant dark`
re-keys Tailwind's built-in `dark:` from `prefers-color-scheme` to an
attribute that app may not set, silently deleting its dark mode with no error.
`appkit gen` keeps `attribute`, so the Go consumers are byte-unchanged.

**Divergence is the system working.** A component that declines a kit
equivalent carries a `DIVERGENCE:` header naming what it declined and why.
Two divergences with the same reason are a kit bug with a queue position.

## fathom fleet index (code intelligence — use it first)

The fleet index (agentic-kit · app-kit · fathom, plus the wider fleet) is
served by a single always-on launchd **daemon** — `fathom serve-http` at
`http://localhost:7575/rpc`, registered user-scope as an HTTP MCP server. Every
session shares one store, so there is no single-writer lock contention or
"Failed to connect" between sessions (the old per-session `serve-mcp` model is
retired). If a session doesn't see the tools, it started before the daemon was
registered — restart it once.

Resolve "where/what/who-calls" via its tools (`find_symbol`, `query_symbols`,
`find_callers`, `outline`, `repo_map`, `pack_context`) before any Grep/Read
sweep — read files to edit them, not to discover them. Cross-repo questions are
one query because the index is fleet-wide.

Refresh **through the server, never the CLI**: the daemon holds the
single-writer lock on the SQLite store, so a shell `fathom ingest` fails with a lock error.
Call the `ingest` MCP tool (`{"service":"app-kit"}`) at session start and after
each commit (hash-gated, ~0.2s when clean); because the store is shared, your
refresh updates the index for every session. After heavy uncommitted edits run
`staleness_report` before trusting results (or just read the file you're about
to change). Only fleet repos are indexed — a repo that isn't a fathom service
yet must be ingested as a new service first.

<!-- fathom:begin -->
## fathom code intelligence

This repo is indexed by fathom as service "app-kit". Before any Grep/Read
sweep, resolve where/what/who-calls via the fathom MCP tools:
`find_symbol`, `references`, `symbol_context`, `outline`, `repo_map`,
`pack_context`, `verify_plan`. Read files to *edit* them, not to discover
them — the index replaces exploratory grep.

Refresh the index via the `ingest` MCP tool (`{"service":"app-kit"}`) at
session start and after each commit; it is hash-gated, so a clean repo
refreshes in well under a second.

Obligations — governing rules for the file you are about to touch — are
injected automatically on every edit via the PostToolUse hook. Treat them
as binding, not advisory.

Continuity: run `fathom thread brief` at session start if your harness has
no SessionStart hook (Codex does not) — it prints the open threads, or
nothing. File what the next session must know: `fathom thread note "…"`.

Done means proven: a plan task is only "done" when the index proves it, not
when it is claimed. Before checking off a `docs/plans/*.md` item, run:

    fathom verify --service app-kit --plan docs/plans/<plan>.md

Exit 0 = every claimed task verifies. Exit 1 = a claimed task is unproven —
fix the code or uncheck the box. Exit 2 = operational (daemon down / index
stale) — fail-open, not a verification failure.
<!-- fathom:end -->
