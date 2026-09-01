# Legacy `tailwindplus` MCP server — parity & retirement checklist

The G2 gate (ROADMAP, Phase 2) retires the existing `tailwindplus` MCP server
once the kit's Assistant (`appkit-mcp`, D11) **matches or consciously
supersedes** every tool and real usage it provides. This is that checklist.
"Retirement is the gate, parity is the proof."

The legacy server serves the raw 336MB JSON dump directly; `appkit-mcp` serves
the compiled corpus (`internal/twplus/`, D10). Both can run side-by-side in a
`.mcp.json` until retirement is signed off.

## The legacy tool surface

Five tools (captured from the live server, 2026-06-16):

| # | Legacy tool | Shape |
|---|---|---|
| 1 | `list_component_names` | all component names grouped by category |
| 2 | `search_component_names(search_term)` | case-insensitive substring match over names; **flat, unranked** list |
| 3 | `get_component_by_full_name(full_name, framework, tailwind_version, mode)` | the component's source code + metadata |
| 4 | `get_component_preview_by_full_name(…)` | the rendered preview HTML |
| 5 | `list_tailwindplus_information` | dump metadata: `{version, component_count: 657, downloader_version, …}` |

## Parity matrix

| Legacy | `appkit-mcp` coverage | Verdict | Notes |
|---|---|---|---|
| `list_component_names` | `list_components` (+ `list_sections`) | **supersede** | structural browse with variant counts and `section`/`framework`/`tw`/`mode` filters; `list_sections` adds taxonomy facets with counts |
| `search_component_names` | `search_components` | **supersede** | BM25 + structural ranking vs. unranked substring. Verified: legacy `search("pricing")` returns 16 names with "Three tiers" 6th; `appkit` ranks it **#1**, and the [golden suite](../../data/tailwindplus/golden-queries.json) holds 15/15 known-items in top-3 |
| `get_component_by_full_name` | `get_component` | **match (+)** | **byte-identical code verified corpus-wide** — `appkit data verify` re-walks the licensed dump and confirms all 10,452 variants' stored code hashes equal the source (not just a spot check). Same required axes; `appkit` also returns the full `available_variants` list for disambiguation |
| `get_component_preview_by_full_name` | `get_component_preview` | **match** | same rendered HTML, from the `previews.sqlite` artifact (`appkit data previews`) |
| `list_tailwindplus_information` | manifest + `list_sections` + server `Instructions` | **match** | dump `version`/`component_count`/`downloader_version` live in `data/tailwindplus/manifest.json`; live counts via `list_sections`/internal `Counts()`; axis vocabulary stated in the server instructions |

Every legacy capability is covered; two are strict improvements (ranked search,
faceted browse). `appkit-mcp` additionally exposes what the legacy server never
did: the kit's ADRs and conventions (`get_kit_guidance`,
`list_kit_decisions`, `get_kit_decision`, `kit://adr/<id>` resources) — D15.

## Axis compatibility

Both take the same axes: `framework` ∈ {html, react, vue}, `tailwind_version`
∈ {3, 4}, `mode` ∈ {light, dark, system} (none for Ecommerce). `appkit-mcp`
treats them as optional filters (returning available variants when
under-specified) rather than all-required — a superset of the legacy contract.

## Retirement criteria (G2 exit)

1. **Coverage** — every legacy tool shape mapped above. ✅ done.
2. **Verified equivalence** — `get_component` code is byte-identical to the
   source across the **entire corpus** (`appkit data verify` / `task
   twplus:verify`: 10,452/10,452 variants, 9,765 blobs hash-checked), plus the
   ranked-search improvement verified above. ✅ done.
3. **Replay-diff (recommended final check, not yet run)** — capture a set of
   real tool calls actually made against the legacy server and replay them
   against both, diffing results, to certify no real usage regresses. This is
   the rigorous proof the gate language calls for; it should run before the
   flip.
4. **Flip** — remove the `tailwindplus` entry from every consuming `.mcp.json`,
   archive the legacy server's repo, and demote the raw JSON to archival input
   (kept until the in-kit downloader makes it obsolete — component-pipeline.md
   §7 Future #1).

## Status

Parity is achieved at the tool-shape level and spot-verified on real output.
**Retirement is owner-gated** and pending the replay-diff in (3). Until then,
both servers can coexist; nothing forces the cutover.
