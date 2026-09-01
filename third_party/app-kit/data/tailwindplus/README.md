# TailwindPlus dataset

Private dump of the author's TailwindPlus (Tailwind UI) subscription,
produced by `tailwindplus-downloader` v3.0.0. **Licensed content — never
publish, never commit to a public remote.**

## Current state

- Source of truth: `tailwindplus-downloader/output/tailwindplus-components.json`
  (sibling repo), ~336MB, dump version `2026-02-02-230729`.
- **657 components** in a four-level taxonomy:
  `Section.Group.Category.Name` (e.g.
  `Application UI.Elements.Badges.Flat`), across sections
  **Application UI**, **Ecommerce**, **Marketing**.
- Each component fans out into 18 variants (6 for Ecommerce): framework ×
  Tailwind version (`3` | `4`) × mode (`light` | `dark` | `system`; Ecommerce
  is `none`). **10,452 variants total.**
- **The real per-snippet payload** (verified 2026-06-15, not the idealized
  shape the design doc first assumed) is
  `{code, language, name, mode, version, supportsDarkMode, preview}`, where:
  - `name` is the **framework**: `"html"` | `"vue"` | `"react"` (despite the
    field name). `language` is only the syntax — `"html"` backs both html and
    vue templates, `"jsx"` backs react — so framework is NOT derivable from
    `language`.
  - `version` is the integer **Tailwind major** (`3` | `4`), not a string.
  - `mode` is JSON `null` for Ecommerce (the pipeline maps it to `"none"`).
  - `full_name` and a separate `framework` field are **not** in the dump; the
    pipeline derives `full_name` from the taxonomy path and `framework` from
    `name`.
  - `preview` is a full HTML document **inline on every snippet** — 258MB of
    the 336MB. The pipeline drops it from the queryable index (it's the source
    for a future `get_preview`).

The raw JSON lives here (git-ignored — not LFS) as
`tailwindplus-components.json`. If the file is missing, copy it per "How the
dump gets here" below.

## Pipeline (built 2026-06-15)

`appkit data build` compiles the dump into one SQLite index
(`index/components.sqlite`, ~37MB, git-ignored): CUE-validated typed records
→ structural columns + contentless FTS5 (BM25) + zstd-compressed,
content-addressed code blobs. Code is 63MB → 14MB zstd; build ≈ 15s.
`appkit-mcp` serves it (see `cmd/appkit-mcp/README.md`). Semantic/vector
search is not wired yet — BM25 + structural is the value floor and has no
runtime dependency; the schema reserves room for embeddings.

## How the dump gets here

From the repo root:

```sh
cp /Volumes/m2ext/code/geoffmilleraz/tailwindplus-downloader/output/tailwindplus-components.json data/tailwindplus/
```

The original copy attempt was blocked by macOS volume permissions (TCC) on
the m2ext volume — if the copy fails, grant your terminal access to the
volume and retry.

## Where this is going

See `docs/adr/0010-component-data-pipeline.md` and
`docs/design/component-pipeline.md` (and its 2026-06-15 implementation note)
for the full design and how the shipped MVP deviates from it (SQLite-first,
no Parquet layer yet, no embeddings yet, ncruces `ext/fts5` instead of the
C sqlite-vec extension). The existing `tailwindplus` MCP server keeps working
off the JSON until `appkit-mcp` reaches parity (D11/G2).
