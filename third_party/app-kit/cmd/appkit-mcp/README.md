# appkit-mcp

The kit's MCP server. One stdio (or HTTP) endpoint that gives any project's
coding agent two things:

1. **The TailwindPlus corpus, queryable** — the 657-component / 10,452-variant
   library you licensed, compiled into a fast local index (D10). Search by
   intent, filter by framework/version/mode, fetch exact source.
2. **app-kit itself, legible** — the hard rules and the ADRs behind them
   (D11/D15), so an agent writing code for a kit app builds it the kit way
   (no Node, CUE source of truth, the loopback invariant) instead of guessing.

It is cgo-free and pure Go: no Node, no native libraries, no daemon.

## One-time setup

Build the corpus index from the licensed dump (needs
`data/tailwindplus/tailwindplus-components.json` present — see
`data/tailwindplus/README.md`):

```sh
task twplus:build      # → data/tailwindplus/index/components.sqlite (~37MB)
task twplus:previews   # optional → data/tailwindplus/index/previews.sqlite (~12MB), for get_component_preview
task mcp:build         # → bin/appkit-mcp
```

The index and the dump are git-ignored (licensed content, D10); each machine
builds its own with `task twplus:build`.

## Use it from another project

Add an `.mcp.json` to the consuming repo. **Recommended** (built binary, explicit root):

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

**No-build alternative** (compiles on launch; mirrors how Throughline calls the
`appkit` CLI from a sibling checkout):

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

`APPKIT_ROOT` lets the server find the ADRs and the index regardless of the
host's working directory. The server also auto-discovers both by walking up
from the working directory for the kit's `go.mod`, so inside the app-kit
checkout no env is needed.

### Resolution & flags

| What | Flag | Env | Default |
|---|---|---|---|
| Corpus index | `-index PATH` | `APPKIT_TWPLUS_INDEX` | `<kit-root>/data/tailwindplus/index/components.sqlite` |
| Previews (optional) | `-previews PATH` | `APPKIT_TWPLUS_PREVIEWS` | `<kit-root>/data/tailwindplus/index/previews.sqlite` |
| Embeddings (optional) | `-embeddings PATH` | `APPKIT_TWPLUS_EMBEDDINGS` | `<kit-root>/data/tailwindplus/index/embeddings.sqlite` |
| Ollama (query embedding) | `-ollama URL` | `OLLAMA_HOST` | `http://127.0.0.1:11434` |
| Kit checkout (ADRs/guidance) | `-kit-root PATH` | `APPKIT_ROOT` | discovered via go.mod walk |
| HTTP transport | `-http :PORT` | — | stdio |

If the index is missing the server exits with a build hint; if the kit root is
wrong the component tools still work and the guidance tools report it.

**Semantic search is optional and automatic.** When `embeddings.sqlite` exists
(built by `appkit data embed` / `task twplus:embed`, needs a local Ollama),
`search_components` fuses BM25 + vector KNN (Reciprocal Rank Fusion); otherwise,
and on any query-time embed failure, it is pure BM25. Never a dependency
(ADR-0010). The startup log states which mode is active.

**HTTP mode serves licensed content.** `-http` binds **loopback only** by
default (a bare `:PORT` becomes `127.0.0.1:PORT`) and refuses a non-loopback
host unless you pass `-http-public` (D10/D14). The handler keeps the SDK's
Origin / DNS-rebinding protection on. Never expose this port to a public
interface.

## Tools

| Tool | Purpose |
|---|---|
| `search_components` | Ranked search over taxonomy/names/classes — BM25 + structural, or hybrid (BM25 ⊕ vector, RRF) when embeddings are present. Follow with `get_component`. |
| `get_component` | Full source for one variant by `full_name` + framework/tailwind_version/mode. Lists available variants if ambiguous. |
| `get_component_preview` | The rendered standalone preview HTML for a variant (visual reference). Needs the previews artifact — `task twplus:previews`; reports how to build it if absent. |
| `list_components` | Browse the catalog structurally with filters. |
| `list_sections` | Sections (with counts) or the groups within a section. |
| `get_kit_guidance` | The hard rules and how to build on app-kit — read before generating kit-app code. |
| `get_kit_theming` | How to adapt a TailwindPlus component to the kit's semantic tokens — palette utilities → role utilities (`bg-indigo-600`→`bg-accent`), `dark:` color variants dropped. No args → full guide; `classes` → per-class rewrite. |
| `list_kit_decisions` / `get_kit_decision` | The ADRs, by id or slug (`0005`, `loopback`, `component-data-pipeline`). Also exposed as `kit://adr/<id>` resources. |

Axes: framework ∈ {html, react, vue}; tailwind_version ∈ {3, 4}; mode ∈
{light, dark, system} (none for Ecommerce).

## Refresh

When the dump updates, re-run `task twplus:build`; content-addressed code
blobs mean only changed variants cost work. Semantic (vector) search is not
yet wired — search is BM25 + structural today; the schema reserves room to add
embeddings without a query-API change.
