<!-- fathom:begin -->
## fathom code intelligence

This repo is indexed by fathom as service "fathom". Before any Grep/Read
sweep, resolve where/what/who-calls via the fathom MCP tools:
`find_symbol`, `references`, `symbol_context`, `outline`, `repo_map`,
`pack_context`, `verify_plan`. Read files to *edit* them, not to discover
them — the index replaces exploratory grep.

Refresh the index via the `ingest` MCP tool (`{"service":"fathom"}`) at
session start and after each commit; it is hash-gated, so a clean repo
refreshes in well under a second.

Obligations — governing rules for the file you are about to touch — are
injected automatically on every edit via the PostToolUse hook. Treat them
as binding, not advisory.

Done means proven: a plan task is only "done" when the index proves it, not
when it is claimed. Before checking off a `docs/plans/*.md` item, run:

    fathom verify --service fathom --plan docs/plans/<plan>.md

Exit 0 = every claimed task verifies. Exit 1 = a claimed task is unproven —
fix the code or uncheck the box. Exit 2 = operational (daemon down / index
stale) — fail-open, not a verification failure.
<!-- fathom:end -->

## UI: the observatory is an app-kit app (never hand-write markup)

The web surfaces (`/`, `/observatory*`, served by serve-http) are built on
**app-kit** (Path A, Go server-rendered): `appkit/ui/dashboard` for layout,
`ui/stat` for KPIs (`stat.Absent` for uninstrumented metrics — law 3),
`ui/viz` for charts (Sparkline, Gauge, Bars). The doctrine, binding:

1. **Search before writing.** Query the `appkit` MCP (`search_components`,
   TailwindPlus + MatDash) and the kit packages before authoring ANY
   markup — hand-writing a component the corpus or kit already has is the
   named failure mode, and this repo already deleted one hand-rolled tile
   set the same day it was written (2026-08-08).
2. **Role utilities only** (`bg-surface-raised`, `text-ink-muted`,
   `text-caution`…): never a palette color, never an inline color, never
   `dark:` color variants — the kit tokens carry light/dark.
3. **CSS via `task ui:css`** — the checksum-pinned Tailwind binary; no
   Node, no package.json, ever. The build reads `cmd/fathom/ui/input.css`
   whose **@source globs are load-bearing**: any new file that renders kit
   utilities must be listed there or its classes silently never render.
4. **A 200 is not a rendering.** Open the pages in a browser (or
   Playwright-screenshot both themes) before claiming UI work done.
5. **Promote upward (D13).** A generalizable piece moves into app-kit with
   a rationale — `stat.Absent` (the honest-absence tile) is the precedent.
