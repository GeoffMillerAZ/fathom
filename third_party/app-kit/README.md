# app-kit

**One person. Many apps. Zero starting-from-scratch.**

The personal product platform under every app from here on: a Go module
(the Kit), a living style guide (the Gallery), and an MCP design server
(the Assistant). Read [VISION.md](VISION.md) first, then
[docs/DECISIONS.md](docs/DECISIONS.md) (the binding register, D1–D15) and
[ROADMAP.md](ROADMAP.md) (gates, not deadlines).

## What works today

### The Assistant — `appkit-mcp`

The kit's MCP server (D11/D15). One endpoint that gives any project's coding
agent two things: **your licensed TailwindPlus library, queryable**, and **the
kit's own decisions, legible** — so an agent builds *the kit way* instead of
guessing. See [cmd/appkit-mcp/README.md](cmd/appkit-mcp/README.md) and the
[getting-started guide](docs/getting-started.md).

```sh
task twplus:build   # licensed dump → data/tailwindplus/index/components.sqlite (37MB, ~15s)
task mcp:build      # → bin/appkit-mcp   (cgo-free: no Node, no native libs)
```

Then point any sibling project at it via `.mcp.json` ([Adopt it](#adopt-it-in-your-project)).
The agent gets `search_components` → `get_component` over **657 components /
10,452 variants** (html·vue·react × Tailwind 3·4 × light·dark·system), plus
`get_kit_guidance` and all 15 ADRs (also exposed as `kit://adr/<id>`
resources). Pipeline + server: [internal/twplus/](internal/twplus/),
[docs/design/component-pipeline.md](docs/design/component-pipeline.md) §9.

### The CSS/token chain — the slice Throughline's UI rebuild rides on

```sh
task gen        # CUE tokens → @theme CSS + DTCG JSON + Go constants (committed)
task css:build  # checksum-pinned Tailwind v4 standalone builds app CSS (no Node)
task check      # vet + tests + cue vet + regeneration cleanliness
```

- `appkit/ui/tokens` — the design-token pipeline: CUE schema (embedded,
  validates every app's token set), deterministic emitters, runtime
  light/dark switching via `data-theme` with an OS-preference fallback, and
  the `--ak-font-scale` display-settings hook.
- `cue/tokens/kit.cue` — the kit default theme. Apps author their own
  `tokens.cue` against the same schema and run
  `appkit gen -tokens ... -css ... -go ...`.
- `internal/tailwind` — the D2 rig: Tailwind v4.3.0 standalone pinned by
  SHA-256 per platform (embedded CUE manifest), fetched to the user cache,
  verified on every run, mismatch = hard failure. The conformance test
  builds real CSS over a templ fixture.

An app's `assets/input.css` is three lines:

```css
@import "tailwindcss";
@import "<path to generated theme.css>";
@source "../**/*.templ";
```

## ui/viz — dashboard primitives

`appkit/ui/viz` is the kit's first visualization component library. All
functions return a plain SVG string — pure Go, stdlib only, no framework
dependency.

```go
import "github.com/geoffmilleraz/app-kit/appkit/ui/viz"

// In a templ component:
@templ.Raw(viz.Sparkline(values, viz.SparklineOptions{}))

// In html/template:
{{ template.HTML(viz.MeterRow(65, 100, viz.MeterOptions{Label: "coverage", ShowPct: true})) }}
```

All user-supplied text is HTML-escaped internally. All colors resolve to
`var(--ak-color-<role>)` from the kit token pipeline — components re-theme
with light/dark automatically.

| Function | Purpose | Key options |
|---|---|---|
| `Sparkline(values []float64, o SparklineOptions)` | Inline trend line | W, H, StrokeRole, FillRole |
| `StackBar(segments []Segment, o StackBarOptions)` | Horizontal coverage/funnel bar | W, H, Radius |
| `Donut(segments []Segment, o DonutOptions)` | Distribution ring | Size, Thickness, CenterText |
| `Funnel(stages []Stage, o FunnelOptions)` | Vertical funnel (proposed→final) | W, StageH, Gap |
| `MeterRow(value, max float64, o MeterOptions)` | Labeled confidence meter | Label, ShowPct, FillRole |

Edge cases handled for all: zero-total, empty input, NaN-filtered, negative
values clamped. Invalid role names fall back to `ink-muted` (no injection).
Every component emits `role="img"` + `aria-label` + `<title>`.

Golden files in `appkit/ui/viz/testdata/`. To regenerate after intentional
output changes: `UPDATE_GOLDEN=1 go test ./appkit/ui/viz/...`.

## Adopt it in your project

The Assistant is the fastest way to *use* the kit from another repo. Build the
corpus once (above), then drop an `.mcp.json` in the consuming project:

```json
{
  "mcpServers": {
    "appkit": {
      "command": "/abs/path/to/app-kit/bin/appkit-mcp",
      "env": { "APPKIT_ROOT": "/abs/path/to/app-kit" }
    }
  }
}
```

Now any agent in that project can `search_components` / `get_component` over
your TailwindPlus library and `get_kit_guidance` / `get_kit_decision` to build
the kit way. Full walkthrough, the no-build `go run` alternative, and the
loopback-only HTTP mode: [docs/getting-started.md](docs/getting-started.md)
and [cmd/appkit-mcp/README.md](cmd/appkit-mcp/README.md).

To consume the kit's **Go packages** (tokens, `ui/viz`) directly, import
`github.com/geoffmilleraz/app-kit/...` as a normal module dependency.

## Status

Pre-Phase-1 on the platform spine. Built and working:

- **The CSS/token chain** (Spike 0.3, graduated into kit packages) — verdict in
  [spikes/0.3-tokens-css/README.md](spikes/0.3-tokens-css/README.md).
- **`ui/viz`** dashboard primitives (see above).
- **The component pipeline + Assistant** (D10/D11), built ahead of roadmap
  sequence at the owner's direction (2026-06-15): `internal/twplus/` compiles
  the licensed TailwindPlus dump into a queryable SQLite corpus and
  `cmd/appkit-mcp` serves it plus the kit's ADRs over MCP. Search is BM25 +
  structural by default; hybrid semantic search (BM25 ⊕ vector) is built and
  turns on automatically once `appkit data embed` runs against a local Ollama.
  A Parquet durable layer and retirement of the legacy `tailwindplus` server
  are tracked but not done — see
  [docs/design/component-pipeline.md](docs/design/component-pipeline.md) §9–§10.

Not yet built: Spikes 0.1 (loopback SSE in shells) and 0.2 (embedded NATS),
and the Phase 1 platform foundation (`appkit/app`, `web`, `shell`, `bus`,
`store`) downstream of them. Next: [docs/NEXT-SESSION.md](docs/NEXT-SESSION.md).
