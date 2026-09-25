# Dashboards & MatDash

Current routing: [experience system](experience-system.md), [adoption](adoption.md)
and [scope](../intent/scope.md). The historical sequence below records initial
integration, not current fleet-wide proof. The source audit found Tailwind 3 in the
main package manifest while the extractor labels records Tailwind 4 without conversion;
verify dependencies before adaptation. See the [assessment](../assessments/2026-09-13-app-kit.md).

Status: in progress (2026-06-17, owner-directed). **MatDash React/Tailwind/Vite
Pro is purchased and on disk** at `data/matdash/` (gitignored — licensed, D6/D10).
This doc records what it is, how it's stored, and how each kind of app uses it so
the money lands across the fleet.

## What it is (verified from the package)

MatDash Pro is a **React + Vite + TypeScript + Tailwind + Flowbite-React**
dashboard template — *not* plain HTML. Monorepo under `data/matdash/packages/`:

| Package | What it is |
|---|---|
| `starterkit` | Minimal app (layout shell + routes + a couple views) — the **base for a new React app** |
| `main` | Full kitchen-sink app: `components/dashboards/{Dashboard1,Dashboard2,dashboard3}`, all `views/`, every widget |
| `dark` / `horizontal` / `mini-sidebar` / `rtl` | Layout/theme variants of the shell |
| `jwt-firebase` | Auth-wired variant |

Each app is `src/{api,components,context,css,layouts,routes,types,utils,views}` +
`tailwind.config.ts` + `postcss.config.js`. Also ships `MatDash.fig` (Figma) and
`docs/` (HTML usage docs). It gives the thing TailwindPlus and kit `ui/viz`
don't: a **cohesive dashboard shell + layout system** (sidebar/topbar/widget grid)
plus ready dashboard pages.

## Storage, backup, and licensing

- **Location:** `data/matdash/` — original, full, on disk. Mirrors
  `data/tailwindplus/`: **gitignored**, never committed, never redistributed
  (D6/D10). Its tooling will live under `internal/` so importers can't reach it.
- **Backup and entitlement:** gitignored data is not backed up by this repository.
  The purchase channel, tier and re-download availability must be verified against
  the actual purchase record. Do not infer the vendor account or redistribution
  entitlement from an installed directory. Keep any licensed backup private.

## How each app uses it (two paths)

**Path B — React/Node apps (pumpkin, future React apps): use it directly.**
This is the native fit — MatDash *is* React+Vite+Tailwind+Flowbite.
- New app → copy `packages/starterkit` as the base (shell + routing), then pull
  dashboard pages/widgets from `packages/main` (`components/dashboards/*`,
  `views/*`). Restyle via the app's own `tailwind.config.ts`.
- Existing React app (pumpkin) → lift the layout shell + the widgets it needs.
- It uses **Flowbite-React** under the hood — keep that in mind when pulling
  components (they expect Flowbite + its Tailwind plugin).

**Path A — Go server-rendered apps (Throughline, new Go apps): mine → adapt.**
The Flowbite-React components don't run in a no-Node Go app, but their **layout +
Tailwind structure** translate. Workflow (same as TailwindPlus):
- Inspect the source/dependencies and authorized previews; adapt the applicable
  shell/widget behavior to kit roles and the chosen target. A corpus reference
  is not a validated Go or Tailwind-version conversion. Kit build rules remain
  governed by ADR-0017; no new npm execution exception is introduced here.
- Build it into **`appkit/ui/dashboard`**: `Shell` (sidebar + topbar + content
  region, slots for nav/user/actions), `WidgetGrid` (responsive card grid),
  `KpiWidget` (composing existing `stat` + `viz`). Charts stay kit `ui/viz` (SVG)
  for the Go path. MatDash's React package does include ApexCharts/react-apexcharts;
  those have a different runtime and need explicit adaptation or profile selection.

## Discovery funnel — making it reachable fleet-wide

The global `~/.claude/CLAUDE.md` UI-stack directive already points every project at
the `appkit` MCP and names MatDash. To make MatDash *searchable* like TailwindPlus:

- **Index it as a second corpus in `appkit-mcp`.** Reuse `internal/twplus`
  (parse → normalize → SQLite FTS5 + zstd blobs). New piece: `internal/matdash/
  extract` walks `data/matdash/packages/main/src` (`components/`, `views/`,
  `layouts/`), emitting one record per `.tsx` component (Section/Group/Name/Code,
  framework `react`). Index into the existing store with a `source=matdash` column
  (or `data/matdash/index/`, gitignored). `search_components` then returns MatDash
  components by intent; `get_component` fetches the `.tsx`.
- Until indexed, `get_kit_guidance` + this doc are the pointers.

## Sequencing

1. ✅ Placed at `data/matdash/` (gitignored); `ref/` cleanup done.
2. ✅ `internal/matdash` extractor + `appkit data matdash-build` → index
   (`data/matdash/index/components.sqlite`, 762 records); `cmd/appkit-mcp` serves
   it as a second corpus via the `source` param on `search_components`/
   `get_component`. Verified (build/vet/test, FTS, startup `matdash=true`).
   Commit `9c6acb9`.
3. ✅ `appkit/ui/dashboard` (`RenderShell` + `RenderWidget`/`RenderWidgetGrid`),
   adapted from MatDash's FullLayout; gallery Dashboard page (commit `09da02e`);
   recipe documented in `CLAUDE.md` (surfaced by `get_kit_guidance`).
4. ✅ Adopt: Throughline (path A) — `/dashboard` page frames the existing
   coverage/outcomes/deadlines projections into the kit's `RenderWidget`/
   `RenderWidgetGrid` inside Throughline's own shell (UPL guardrail honored:
   counts/ranges "if established", no prediction). Both editions build; verified
   in browser against the Imported Case (Throughline commit `055fa84`). Pumpkin
   (path B) reuses `data/matdash/packages/starterkit` + `main` directly — enabled
   by the global UI-stack directive; no app-kit code needed.

**Plan complete (2026-06-18).** All four steps done; MatDash is searchable
fleet-wide via the appkit MCP, the kit has a turnkey dashboard, and Throughline
consumes it.

Relates to: `component-pipeline.md` (the twplus pipeline this reuses),
`agentic-ux.md` (observability dashboards share the viz), and the global
`~/.claude/CLAUDE.md` UI-stack directive (the funnel this feeds).
