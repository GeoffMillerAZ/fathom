# Getting started — adopting app-kit

This is the end-to-end path from a fresh clone to **using the kit's Assistant
in another project**: an MCP server that lets any coding agent search your
licensed TailwindPlus library and read the kit's decisions so it builds the
kit way.

If you only want the kit's Go packages (design tokens, `ui/viz`), skip to
[Consuming the Go packages](#consuming-the-go-packages).

## Prerequisites

- **Go 1.26+** (`go version`). That's it for the Assistant — the whole stack
  is cgo-free (SQLite runs as WebAssembly via ncruces/go-sqlite3), so there is
  **no Node, no native library, no daemon** to install (D2).
- **[Task](https://taskfile.dev)** (`task`) for the convenience targets, or run
  the underlying `go` commands directly.
- The **licensed TailwindPlus dump** at
  `data/tailwindplus/tailwindplus-components.json`. It is git-ignored and
  never redistributed (D10); see
  [data/tailwindplus/README.md](../data/tailwindplus/README.md) for how to
  place it. Without it you can still build everything except the corpus index.

## 1. Clone and sanity-check

```sh
git clone <app-kit remote> app-kit && cd app-kit
task check     # go vet + tests + cue vet + regeneration cleanliness — all green
```

`task check` is the full gate and a good smoke test that your toolchain
matches the kit's pins.

## 2. Build the corpus index

This compiles the 336 MB dump into one queryable SQLite file
(`data/tailwindplus/index/components.sqlite`, ~37 MB, also git-ignored):

```sh
task twplus:build
# appkit data build: 657 components, 10452 variants, 9765 blobs in ~15s
# code 63MB → 14MB zstd · index 37MB → data/tailwindplus/index/components.sqlite
```

What it does: streams the dump (dropping 258 MB of inline preview HTML),
validates every component against the CUE schema (D7), normalizes it, and
writes structural columns + a contentless FTS5 BM25 index + zstd
content-addressed code blobs. Details:
[component-pipeline.md](design/component-pipeline.md).

Smoke-test it without the server:

```sh
go run ./cmd/appkit data search -framework react -tw 4 pricing three tiers
go run ./cmd/appkit data get -framework react -tw 4 -mode light \
  "Marketing.Page Sections.Pricing Sections.Three tiers"
go run ./cmd/appkit data stats
```

Confirm your built index is sound and faithful to the dump (every blob
decompresses and hashes to its content address; counts match the manifest; and,
when the dump is present, all 10,452 variants are byte-identical to the source):

```sh
go run ./cmd/appkit data verify   # or: task twplus:verify
```

It exits non-zero on any mismatch, so it's safe to gate a rebuild on. The
fidelity pass is skipped automatically if you don't have the licensed dump;
the integrity and manifest checks still run.

### Optional: semantic search

Search is BM25 + structural by default and meets the quality bar (15/15 golden
known-items in top-3). If you run a local [Ollama](https://ollama.com) you can
add hybrid (BM25 ⊕ vector) ranking for better intent matching:

```sh
ollama pull qwen3-embedding-0.6b
go run ./cmd/appkit data embed        # or: task twplus:embed
# → writes data/tailwindplus/index/embeddings.sqlite (git-ignored)
```

The server and CLI pick it up automatically: `search_components` fuses BM25 and
vector KNN (Reciprocal Rank Fusion) when `embeddings.sqlite` is present, and
falls back to BM25 whenever Ollama is unreachable. Point at a non-default daemon
with `-ollama`/`OLLAMA_HOST`. Semantic is never a dependency (ADR-0010) — the
kit works fully without it.

## 3. Build the server

```sh
task mcp:build      # → bin/appkit-mcp
```

`bin/appkit-mcp` is a single self-contained binary. By default it speaks MCP
over **stdio** (what an agent host launches); `-http :PORT` switches to the
Streamable HTTP transport (loopback-only — see
[the server README](../cmd/appkit-mcp/README.md)).

## 4. Wire it into another project

In the consuming repo, add `.mcp.json` (this is what Claude Code and other MCP
hosts read). **Recommended** — built binary, explicit kit root:

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

**No-build alternative** — compiles on launch, mirrors how Throughline already
calls the `appkit` CLI from a sibling checkout:

```json
{
  "mcpServers": {
    "appkit": {
      "command": "go",
      "args": ["run", "-C", "/abs/path/to/app-kit", "./cmd/appkit-mcp"],
      "env": { "APPKIT_ROOT": "/abs/path/to/app-kit" }
    }
  }
}
```

`APPKIT_ROOT` lets the server find the corpus index and the ADRs no matter
what directory the host launches it from. Inside the app-kit checkout itself
no env is needed — it discovers both by walking up to the kit's `go.mod`.

Restart the host so it connects the server. (MCP servers attach at session
start; a config added mid-session won't load until you relaunch.)

## 5. Use it

Once connected, the agent has these tools:

| Tool | Use it to… |
|---|---|
| `search_components` | find components by intent/keywords (BM25 over taxonomy, names, classes) with optional `section`/`framework`/`tailwind_version`/`mode` filters |
| `get_component` | get the exact source for one variant by `full_name` + axes |
| `list_components` / `list_sections` | browse the catalog structurally |
| `get_kit_guidance` | read the hard rules before generating kit-app code |
| `get_kit_theming` | adapt a pulled component to the kit's semantic tokens (palette → role utilities; drop `dark:` color variants) |
| `list_kit_decisions` / `get_kit_decision` | read the ADR behind any decision (by id or slug) |

The intended flow is **search → get → (adapt)**:
`search_components("testimonial grid")` then `get_component(full_name,
framework="react", tailwind_version=4, mode="light")`, and — when the consuming
project themes through app-kit tokens — `get_kit_theming(classes=…)` to turn the
component's palette utilities into role utilities. Frameworks are `html` / `vue`
/ `react`; versions `3` / `4`; modes `light` / `dark` / `system` (`none` for
Ecommerce).

A concrete check that adoption worked: in the consuming project, ask the agent
to "find a pricing section with three tiers and give me the React Tailwind-v4
code" — it should call `search_components` then `get_component` and return real
source, with no human explanation.

## Keeping it fresh

When the dump updates, re-run `task twplus:build`. Content-addressed code blobs
mean only changed variants cost work. The index and dump stay git-ignored
(licensed); each machine builds its own.

## Consuming the Go packages

The kit's reusable Go lives under `appkit/`. Import it as a normal module:

```go
import (
    "github.com/geoffmilleraz/app-kit/appkit/ui/tokens"
    "github.com/geoffmilleraz/app-kit/appkit/ui/viz"
)
```

- `appkit/ui/tokens` — the design-token pipeline. Author your app's
  `tokens.cue` against the embedded schema and generate CSS/DTCG/Go with
  `appkit gen` (see the root [README](../README.md)).
- `appkit/ui/viz` — pure-Go SVG dashboard primitives (Sparkline, StackBar,
  Donut, Funnel, MeterRow), themed via the token CSS variables.

Licensed-data tooling (`internal/twplus/`) is deliberately under `internal/`,
so downstream modules can't import it — they reach the corpus only through the
Assistant.

## Troubleshooting

- **`open corpus index … no such file`** — run `task twplus:build` first, or
  point `-index` / `APPKIT_TWPLUS_INDEX` at the SQLite file.
- **Guidance tools say docs not found** — set `-kit-root` / `APPKIT_ROOT` to
  the app-kit checkout; component tools still work without it.
- **The server isn't listed in the host** — MCP servers attach at launch;
  relaunch the host after editing `.mcp.json`.
- **`-http` refuses to start** — by design it binds loopback only; pass a
  loopback host or `-http-public` (it serves licensed content, D10/D14).

Where to go next: [docs/README.md](README.md) is the map of the whole corpus
(VISION, DECISIONS, ADRs, design docs).
