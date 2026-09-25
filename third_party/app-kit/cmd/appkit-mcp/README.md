# appkit-mcp

One stdio/Streamable HTTP endpoint for first-party discovery, guidance and licensed references.
Start with the consumer's own intent and selected layers/profile; adopting portable
assets does not require adopting the Go application runtime (ADR-0017/0018/0021).

The [adoption guide](../../docs/design/adoption.md) defines the current workflow.
The embedded CUE-validated catalog covers first-party tokens, primitives, recipes and
CLI-tool references—an initial selection, not every kit API or completed workflow.
Automated consumer profile generation, doctor/audit and upgrade receipts remain planned;
the explicit read-only adoption CLI and authored quality inventory are implemented.
The server is cgo-free Go; no Node or daemon required.

## Build and optional source setup

Build the server; first-party discovery needs no commercial corpus:

```sh
task mcp:build         # → bin/appkit-mcp, including the first-party catalog
```

To enable licensed references, build the optional local artifacts from your source:

```sh
task twplus:build      # needs data/tailwindplus/tailwindplus-components.json
task twplus:previews   # optional rendered TailwindPlus reference previews
appkit data matdash-build  # optional MatDash index; accepts -src and -out
appkit data assets-import -src /path/to/official-tailwind-plus-zip-downloads
```

Source stays private/Git-ignored; building/serving grants no redistribution rights (ADR-0010/0022).

## Use it from another project

Use [agent setup](../../docs/design/agent-setup.md) for both hosts. Claude Code's
project configuration is `.mcp.json`; Codex uses `config.toml`. **Recommended**
(built binary, explicit root; Claude project example):

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

Build explicitly before registration; do not compile/download on each host launch.

`APPKIT_ROOT` locates documents and optional artifacts independently of the host's
working directory. In this checkout, the server discovers the kit's `go.mod` by
walking upward. The catalog itself is embedded in the binary.

### Resolution & flags

| What | Flag | Env | Default |
|---|---|---|---|
| TailwindPlus index (optional) | `-index PATH` | `APPKIT_TWPLUS_INDEX` | `<kit-root>/data/tailwindplus/index/components.sqlite` |
| MatDash index (optional) | `-matdash-index PATH` | `APPKIT_MATDASH_INDEX` | `<kit-root>/data/matdash/index/components.sqlite` if discovered |
| Previews (optional) | `-previews PATH` | `APPKIT_TWPLUS_PREVIEWS` | `<kit-root>/data/tailwindplus/index/previews.sqlite` |
| Embeddings (optional) | `-embeddings PATH` | `APPKIT_TWPLUS_EMBEDDINGS` | `<kit-root>/data/tailwindplus/index/embeddings.sqlite` |
| Ollama (query embedding) | `-ollama URL` | `OLLAMA_HOST` | `http://127.0.0.1:11434` |
| Kit checkout (ADRs/guidance) | `-kit-root PATH` | `APPKIT_ROOT` | discovered via go.mod walk |
| Private design archives (optional) | `-design-assets DIR` | `APPKIT_DESIGN_ASSETS` | `<kit-root>/data/design-assets` |
| HTTP transport | `-http :PORT` | — | stdio |

Missing or unreadable implicitly discovered commercial artifacts log a diagnostic and
leave first-party discovery available. A broken artifact explicitly selected by flag
or environment fails startup with a repair/build hint; remove that setting for use
without the artifact. This applies to indexes, previews, embeddings and design archives. An invalid
explicit root directory also fails; missing document files report unavailable guidance.

**Semantic search is optional and automatic.** When `embeddings.sqlite` exists
(built by `appkit data embed` / `task twplus:embed`, needs a local Ollama),
`search_components` fuses BM25 + vector KNN (Reciprocal Rank Fusion); otherwise,
and on any query-time embed failure, it is pure BM25. Never a dependency
(ADR-0010). Startup logs report loaded sources and optional embeddings.

**MCP HTTP mode can serve licensed content.** `-http` binds **loopback only** by
default (a bare `:PORT` becomes `127.0.0.1:PORT`) and refuses a non-loopback
host unless you pass `-http-public` (D10/D14). The handler applies
Origin / DNS-rebinding protection. Never expose this port to a public
interface.

## Tools

| Tool | Purpose |
|---|---|
| `search_capabilities` | First-party discovery by task, profile/layer, kind and maturity; bounded summaries with IDs, revision and continuation cursor. |
| `get_capability` | One first-party record by exact `id`, with source anchors, usage, dependencies, app-owned seams, proof entrypoints and gaps. |
| `list_quality_work` / `get_quality_work` | Bounded authored standards/gate/adoption inventory; independent inventory and catalog revision guards. Never live checking or certification. |
| `search_components` | Ranked search over taxonomy/names/classes — BM25 + structural, or hybrid (BM25 ⊕ vector, RRF) when embeddings are present. Follow with `get_component`. |
| `get_component` | Full source for one variant by `full_name` + framework/tailwind_version/mode. Lists available variants if ambiguous. |
| `get_component_preview` | Exact UTF-8 pages of captured preview HTML: default 32 KiB, max 64 KiB; follow `has_more`/`next_offset` with `expected_sha256`. External assets are not bundled; a partial page is not a renderable whole preview. Needs `task twplus:previews`. |
| `list_components` | Browse TailwindPlus with filters, limit (default/max 200) and offset; continue using `has_more`/`next_offset` with unchanged filters/index/session. No snapshot guarantee. |
| `list_sections` | TailwindPlus sections (with counts) or the groups within a section. |
| `get_kit_guidance` | Layer/profile-aware routing into current intent and adoption docs. Maintainer-only sections do not govern a consumer checkout. |
| `get_kit_css_setup` | Required CSS setup and kit source scanning for the chosen rendering layer. |
| `get_kit_theming` | Contextual adaptation guide; `classes` returns preservation/review notes with exact original input in `rewritten`. No automatic palette mapping or dark-variant deletion. |
| `search_design_assets` / `get_design_asset` | Bounded private whole-project archive discovery; exact get verifies archive SHA and returns manifest/dependency context. |
| `list_design_asset_files` / `get_design_asset_file` | Bounded file metadata and verified original text, without extraction/execution. Source reads default to 100 lines, max 400/32 KiB per response. |
| `list_design_conversions` / `get_design_conversion` | Private saved syntax attempts with original links; exact reads check integrity/current replay, page diagnostics and source, never execute or certify. See [contract](../../docs/design/html-translation.md#save-and-discover-private-attempts). |
| `search_design_evidence` / `get_design_evidence` | Private historical evidence metadata; search is recorded, exact get rechecks retained bytes and original membership, not today's transformer or semantic claims. No bundle bytes, writes or certification. See [contract](../../docs/design/target-evidence.md). |
| `list_kit_decisions` / `get_kit_decision` | The ADRs, by id or slug (`0005`, `loopback`, `component-data-pipeline`). Also exposed as `kit://adr/<id>` resources. |

Document IDs: `agent-setup`, `intent`, `intent/{principles,invariants,domain,scope,futures,questions,autonomy,incidents,repo}`,
`adoption`, `adoption-contract`, `stabilization`, `experience-system`, `analytics-journey`, `viz-checks`, `application-shell`,
`design-references`, `conversion-pipeline`, `html-translation`, `elements-translation`, `faq-adapter`, `hero-adapter`, `target-evidence`, `runtime-selection`,
`ui-components`, `cli-kit`, `terminal-gallery`, `vision`, `decisions`, `hazards`, `claude`. Matching resources use
`kit://<id>`. Resources and tools serve the same stored bodies.

All existing commercial and guidance tool names remain available. Commercial search/get
default to `source=tailwindplus`; use `source=matdash` for MatDash. Calls to unavailable
sources return tool errors. First-party records use `get_capability`, not `get_component`.

## Shared catalog and Gallery

`kit://catalog` serves the full versioned first-party JSON snapshot. MCP and Gallery
use the same embedded records and SHA-256 revision; rebuild both after catalog changes.
Launch `task gallery:run` for `/catalog` and stable `/catalog/{id}` detail pages.
The Gallery JSON endpoints are `/api/catalog`, `/api/catalog/{id}` and
`/api/catalog/search`; these are distinct from the MCP server's `-http` transport.

Search accepts `q`, `profile`, `layer`, `kind`, `maturity`, `limit`, `cursor` over Gallery
HTTP; MCP uses `query` in place of `q`. Profiles: `portable`, `go-web`, `static-web`,
`react-web`, `cli`; layers: `L0`, `L1`, `L2`, `L3a`, `L3b`, `L4` (omit for CLI runtimes).
Kinds: `token`, `primitive`, `recipe`, `reference`; maturity: `experimental`, `available`,
`proven`. Text requires all query words to match positive capability metadata.
Valid unmatched filters return an empty page; unknown values fail. Limit defaults to
10, bounded 1–50. Continue with `next_cursor` and the same query/filters/limit; changed
queries or revisions require a new search. Search omits full usage and source bodies.

For the six cataloged private adapters, search `query="private adapter"`,
`profile="cli"`, `kind="reference"`, then get `reference.pricing-adapter`,
`reference.directory-adapter`, `reference.form-adapter`, `reference.navigation-adapter`
`reference.faq-adapter`, or `reference.hero-adapter`.
These are metadata-only first-party CLI capabilities,
not licensed source bodies or certified web components. Exact records give frozen
original requirements, JSON/export/check usage and evidence gaps without reading a
private store. Their empty web layers and CLI-only profile intentionally exclude
them from `go-web`/`static-web`/`react-web` compatibility results. Private artifacts
are still obtained through the explicit CLI, not a new MCP generation/execution tool.
The [P05 email-field candidate](../../docs/assessments/2026-09-14-form-typed-adapter.md)
links to `get_kit_decision("form-adapter")` for five-file delivery, CSS scanning of
both `form.go` and `form.templ`, and app-owned validation/save/retry boundaries.
The [P06 page-navigation candidate](../../docs/assessments/2026-09-14-navigation-typed-adapter.md)
links to `get_kit_decision("navigation-adapter")` for five-file delivery, Reference/GET/Links
variants, scanning both `navigation.go` and `navigation.templ`, and app-owned routing,
authorization and submitted-key allowlists. Its additive record changes the global
catalog revision; existing consumer selections/pins are not automatically refreshed.

Maturity is explicit per entry; consumer history and proof entrypoints do not
establish completed workflows. The experimental [terminal reference](../../docs/design/terminal-gallery.md)
uses these same first-party IDs/revision, with no private-source or conversion browser.
Other APIs: `ui-components`/`cli-kit`; private archive
workflow and limitations: [design-references](../../docs/design/private-references.md).

Documents load at process startup; restart for file changes and rebuild for code/catalog
changes. Building does not register either host. Fresh Claude Code/Codex invocation
and consumer adoption still require the [adoption evidence](../../docs/design/adoption.md).

Commercial variant axes: framework ∈ {html, react, vue}; tailwind_version ∈ {3, 4}; mode ∈
{light, dark, system} (none for Ecommerce).

## Refresh

When the TailwindPlus dump updates, re-run `task twplus:build`. For MatDash, rebuild with
`appkit data matdash-build`: the importer now reads each source file's nearest package
manifest and preserves its declared Tailwind major. Current local manifests declare
`tailwindcss: ^3.4.16`, yielding version 3; old version-4-normalized indexes need rebuilding.
Missing/ambiguous declarations fail instead of inventing compatibility. Source bytes
are unchanged; a React/Tailwind 3 reference is not a Go or Tailwind 4 conversion.
Commercial search uses BM25 + structural by default; optional embedding artifacts enable hybrid search. Neither mode proves
task fit, integration correctness or visual quality.
