# cmd/fathom — the observatory UI is an app-kit app (never hand-write markup)

The web surfaces — the Observatory console (vision 35, ADR-045: `/`, `/fleet`,
`/projects/{svc}`, `/impact`, `/sessions`, `/brief`, `/wiring`, `/statusz`,
`/search`, served by serve-http; the legacy `/observatory*` routes redirect) —
are rendered by Go files here on **app-kit** (Path A, Go server-rendered).
**Every console page and component lives in its own `console_<name>.go` and
registers in `init()` (`declarePage` → `registerPage`, `console_registry.go`):
the one table drives routing, the grouped sidebar, search and the JSON parity
test.** Page projections that a CLI verb also shows live in `internal/console`
(no UI imports; values carry a truth state). Kit parts: `appkit/ui/dashboard`
for the shell and grouped nav, `chrome` for breadcrumbs/description lists,
`appearance` for the theme (key `fathom-theme`), `table` for tables, `stat` for
KPIs (`stat.Absent` for uninstrumented metrics — law 3; `valueStat` maps a
projected value), `viz` for charts; a panel is `renderPanel` (question, verb
chip, source/denominator/freshness footer). The doctrine, binding:

1. **Search before writing.** Query the `appkit` MCP (`search_components`,
   TailwindPlus + MatDash) and the kit packages before authoring ANY
   markup — hand-writing a component the corpus or kit already has is the
   named failure mode, and this repo already deleted one hand-rolled tile
   set the same day it was written (2026-08-08).
2. **Role utilities only** (`bg-surface-raised`, `text-ink-muted`,
   `text-caution`…): never a palette color, never an inline color, never
   `dark:` color variants — the kit tokens carry light/dark.
3. **CSS via `task ui:css`** — the checksum-pinned Tailwind binary; no
   Node, no package.json, ever (the Taskfile passes `-terminal ""` to
   `appkit gen` to skip the kit's CLI palette). The build reads
   `cmd/fathom/ui/input.css` whose **@source globs are load-bearing**: any
   new file that renders kit utilities must be listed there or its classes
   silently never render. `console_*.go` is scanned by glob, so a new
   console page file needs no edit there — a new kit package or a renderer
   outside that glob does (and `scripts/ui-css.sh`'s audited closure with it).
4. **A 200 is not a rendering.** Open the pages in a browser (or
   Playwright-screenshot both themes) before claiming UI work done.
5. **Promote upward (D13).** A generalizable piece moves into app-kit with
   a rationale — `stat.Absent` (the honest-absence tile) is the precedent.
6. **Absent is never 0.** No call-log rows for a day, or an unreadable
   store/log, renders `stat.Absent` with a reason; sparklines never draw
   across an absent day (`observatory_series.go`); JSON carries no zero for it.
7. **Mutations stay local.** `/observatory/wiring/*` (`observatory_apply.go`)
   writes only via `internal/wiring` behind `requireLocalMutation` — loopback
   Host, same-origin, registered roots, the digest the previous step returned.
