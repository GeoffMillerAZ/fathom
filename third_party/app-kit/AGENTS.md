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

## Component Libraries & UI Tools (app-kit, MatDash, TailwindPlus)

### MatDash & Dashboard UI
- **Location:** Monorepo at `data/matdash/packages/` (React + Vite + TypeScript + Tailwind + Flowbite-React).
- **Packages:** `starterkit` (turnkey template), `main` (full UI components/views/dashboards), `dark`, `horizontal`, `mini-sidebar`, `rtl`, `jwt-firebase`.
- **Searchable Index:** Indexed in `data/matdash/index/components.sqlite` (762+ records) and queried via the `appkit` MCP server (`search_components`, `get_component`).
- **Go/HTML Primitives:** Pure Go layout primitives in `appkit/ui/dashboard` (`Shell`, `WidgetGrid`) adapted from MatDash's `FullLayout`.
- **Design Spec:** `docs/design/dashboards-and-matdash.md`.

### TailwindPlus MCP Server
- **MCP Server Name:** `tailwindplus`
- **Key Tools:**
  - `list_component_names` / `search_component_names`: Discover Tailwind UI / TailwindPlus components (pass `search_term`).
  - `get_component_by_full_name`: Retrieve full component source code (HTML/React/Vue, TW3/TW4, Light/Dark).
  - `get_component_preview_by_full_name`: Get HTML preview snippet.
  - `list_tailwindplus_information`: Inspect metadata and system info.

### Core Assets & Tools in `app-kit`
- **`appkit` MCP Server (`cmd/appkit-mcp`):** Integrates TailwindPlus + MatDash + Kit guidance via `search_components`, `get_component`, `list_sections`, and `get_kit_css_setup`.
- **Design Token System (`appkit/theme`):** Role-based CSS and Tailwind design tokens for cohesive, dark/light-mode theming.
- **Gallery App (`apps/gallery`):** Interactive component gallery and layout preview application.
- **CLI Tooling (`cmd/appkit`):** Utilities for indexing component corpora (`matdash-build`) and managing design assets.
- **cli-kit (`appkit/cli/*`):** The terminal target (ADR-0018/D18) — token-themed cobra+fang chrome, output discipline, layered CUE config, TUI runner. Building or reviewing any fleet CLI: read `docs/design/cli-kit.md` first; reference consumer is env-kit's `bosun`.

