# ADR-0010: Compile the TailwindPlus corpus into Parquet shards and one SQLite hybrid index

- **Status:** Accepted
- **Date:** 2026-06-10
- **Decision drivers:**
  - A 336MB JSON dump is unqueryable, undiffable, and over every ~20MB
    upload limit an agent tool imposes; the Assistant (D11) needs the corpus
    queryable in microseconds, not scrollable.
  - The shipped query path must be pure Go, cgo-free, and cross-compilable
    (D2, D9) — one binary, no native libs, no daemons required to *search*.
  - At this corpus size (~8k variant rows) brute-force vector search is
    sub-millisecond; any ANN machinery is pure liability.
  - Determinism: same inputs must produce the same canonical artifact bytes
    (VISION, "Where does determinism actually live?").

## Context

The kit owns a private dump of Geoff's TailwindPlus subscription:
**657 components**, dump version `2026-02-02-230729`, produced by
`tailwindplus-downloader` v3.0.0 (verified against the live MCP server,
2026-06-10). Taxonomy is `Section.Group.Category.Name` across sections
**Application UI**, **Ecommerce**, **Marketing**. Each component fans out
across variant axes framework (`html`|`react`|`vue`) × tailwind_version
(`3`|`4`) × mode (`light`|`dark`|`system`; Ecommerce uses `none`) — up to 18
combinations for Application UI/Marketing and 6 for Ecommerce, averaging ~12
variants per component, ~8k variant rows total. A variant payload is
`{version, full_name, name, code, framework, language, tailwind_version,
mode, supportsDarkMode}`. The raw JSON is 336MB at
`/Volumes/m2ext/code/geoffmilleraz/tailwindplus-downloader/output/tailwindplus-components.json`
(currently TCC-blocked; copy instructions live in
`../../data/tailwindplus/README.md`). **This is licensed content: private
only,
never published, never committed to a public remote.**

What the corpus must support: the Assistant's hybrid component search
(structural filters + BM25 + semantic, per D11), the Gallery's catalog
browsing (D12), and eventually one search across TailwindPlus *and* the
kit's own components. Binding constraints from prior decisions: Go + CUE
only (D2), every schema authored in CUE (D7), SQLite via the cgo-free
ncruces/go-sqlite3 driver (D9), agents as first-class consumers (D15).

External facts that shape the solution space (all verified 2026-06-10):

- parquet-go v0.30.1 (2026-05-18) is pure Go, actively released, with a
  typed `GenericWriter`/`GenericReader` API and zstd codecs —
  https://github.com/parquet-go/parquet-go/releases
- ncruces/go-sqlite3 v0.34.4 (2026-06-04) runs SQLite as wazero WASM —
  cgo-free, cross-compilable, supports FTS5 and sqlite-vec —
  https://github.com/ncruces/go-sqlite3/releases. modernc.org/sqlite
  v1.52.0 (2026-06-06) cannot load sqlite-vec —
  https://pkg.go.dev/modernc.org/sqlite
- sqlite-vec v0.1.10-alpha.4 (2026-05-18): `vec0` virtual tables do
  brute-force KNN with partition-key and metadata columns; official Go
  bindings ship an `ncruces` package with a sqlite-vec-enabled WASM build —
  https://github.com/asg017/sqlite-vec/releases,
  https://github.com/asg017/sqlite-vec-go-bindings. ~8k × 768-dim float32
  vectors ≈ 24MB; brute force at this scale is well under a millisecond.
- The hybrid FTS5-BM25 + vec0 + Reciprocal Rank Fusion single-query pattern
  is documented by sqlite-vec's author —
  https://alexgarcia.xyz/blog/2024/sqlite-vec-hybrid-search/index.html
- DuckDB's Go story churned: marcboeker/go-duckdb was archived 2025-10-20;
  the official github.com/duckdb/duckdb-go (v2.10503.1, 2026-06-03,
  bundling DuckDB v1.5.3) requires cgo with per-platform static libs. The
  VSS extension hides HNSW persistence behind
  `hnsw_enable_experimental_persistence` with documented data-loss risk
  (https://duckdb.org/docs/current/core_extensions/vss), and its FTS index
  must be manually rebuilt on change
  (https://duckdb.org/docs/current/core_extensions/full_text_search).
- Local embeddings: Ollama v0.30.7 (2026-06-07); Qwen3-Embedding-0.6B
  offers 32k context, code-retrieval training, and user-definable output
  dimensions — https://github.com/QwenLM/Qwen3-Embedding,
  https://ollama.com/dengcao/Qwen3-Embedding-0.6B. API option: Voyage's
  voyage-code-3 (2048/1024/512/256 dims), with 200M free tokens per account
  covering the whole corpus — https://docs.voyageai.com/docs/pricing.

## Decision

We build a **build-time Go tool** (a `kit` CLI subcommand, `appkit data`)
that compiles the dump in three layers:

1. **JSON → CUE-validated typed records.** The dump is streamed, and every
   component is validated against `#Component`/`#Variant` CUE schemas
   (taxonomy enums, mode rules, payload shape) via embedded cuecontext —
   per D7. Anything that fails validation fails the build loudly.
2. **Typed records → canonical zstd Parquet, sharded by section/group.**
   One row per variant, rows sorted, compression fixed, no timestamps — so
   the same input produces the same bytes. No shard exceeds ~15MB: under
   the ~20MB upload limits agent tools impose, diffable shard-by-shard,
   durable as the typed source of truth. parquet-go v0.30.1
   `GenericWriter` does the writing.
3. **Parquet + embeddings → ONE generated SQLite index file.** Structural
   columns (section/group/category/name, framework, tailwind_version,
   mode, supportsDarkMode); contentless FTS5 for BM25 over
   names/taxonomy/descriptions/extracted classes (contentless because the
   code text must not be duplicated into the index); a sqlite-vec `vec0`
   table of float32[768] vectors with framework/section partition keys —
   brute force, **no ANN, deliberately**; and content-addressed
   zstd-compressed code blobs (variants whose code bytes are identical
   across modes share one blob and one embedding).

**Embeddings** are produced locally by Ollama (qwen3-embedding-0.6B at 768
dims), with the exact model digest pinned in the artifact manifest; the run
is content-addressed and resumable, so re-runs only embed changed rows.
Voyage voyage-code-3 is an optional `--embedder=voyage` flag — but then
query-time embedding needs the same API, so Ollama stays the default. At
query time the Assistant **degrades gracefully to BM25 + structural
filters** when no embedder is reachable; semantic search is an enhancement,
never a dependency.

**The index ships inside the binary** via `go:embed` into `appkit-mcp`.
**DuckDB is permitted as a dev-time convenience** for ad-hoc queries over
the Parquet shards; it ships nowhere (cgo, archived-driver churn,
experimental VSS persistence).

This ADR deliberately does **not** cover: the Assistant's tool surface
(ADR-0011 / `../design/mcp-design-assistant.md`), the in-kit reimplementation
of the downloader (future work, see the design doc), or preview images.
Full design: `../design/component-pipeline.md`.

## Alternatives considered

- **Keep the single JSON file.** Zero pipeline work; the existing
  `tailwindplus` MCP server already runs off it. Lost because 336MB is over
  every upload limit, unqueryable without loading the world into memory,
  undiffable, and untyped — every consumer re-derives structure ad hoc.
- **Generated Go source shards ("a series of smaller Go files / layered
  imports").** The original brief's instinct: emit the data as `.go` files,
  import only what you need. Attractive because the data would be typed and
  compiler-checked with no runtime parsing. Lost on three hard costs:
  (1) *compile time and tooling* — hundreds of megabytes of string literals
  that gopls must index and the compiler must rechew on every toolchain or
  package change; (2) *binary bloat* — string literals land uncompressed in
  the binary's data section, and layered imports only trade bloat for
  package sprawl; (3) *no query engine* — you still hand-roll filtering,
  BM25, and vector scoring over Go slices. `go:embed` of a compressed
  SQLite index delivers the same ships-in-the-binary property *plus* SQL,
  FTS5, and vec0, without recompiling data.
- **DuckDB as the shipped store.** Best-in-class analytical SQL directly
  over Parquet. Lost because the official driver is cgo with per-platform
  static libs (duckdb-go v2.10503.1, 2026-06-03), the previous community
  driver was archived mid-flight (2025-10-20) showing layer churn, VSS
  persistence is experimental with documented corruption risk, and the FTS
  index is rebuild-only. Retained exactly where it's good: dev-time ad-hoc
  queries.
- **chromem-go (pure-Go embedded vector DB).** Zero deps, exhaustive
  search — fine at this scale. Lost because it is vectors-only (structural
  filters and BM25 need a second engine and hand-rolled joins) and its last
  tagged release v0.7.0 is from 2024-09-01, 21 months stale despite recent
  commits — https://github.com/philippgille/chromem-go.
- **bleve.** Real BM25 since v2.5.0 (2025-04), v2.6.0 current
  (2026-04-30). Lost because vector search requires the `vectors` build tag
  plus FAISS via cgo
  (https://github.com/blevesearch/bleve/blob/master/docs/vectors.md) —
  disqualifying for the pure-Go binary — and bleve-BM25 alongside a
  separate vector store means two index lifecycles instead of one file.

**What the winner costs, honestly:** two artifact formats to maintain
(Parquet for truth, SQLite for query) plus a pipeline to keep them
coherent; sqlite-vec is pre-1.0 with an alpha cadence and effectively one
maintainer (pin exact versions, vendor the WASM blob); semantic search
keeps a soft runtime dependency on a local Ollama daemon; and wazero's
JIT is unavailable on iOS (interpreter fallback — a known D9 caveat,
deferred to the mobile phase).

## Consequences

- **Easier:** the Assistant answers component queries from one embedded
  file in microseconds; agents get typed, machine-legible data (D15); the
  corpus is rebuildable from source with one command; shards are diffable
  so future dump updates produce reviewable deltas; the kit's own
  components can join the same index later.
- **Harder:** the pipeline is real code with real stages to maintain; CUE
  validation errors on a 336MB dump need good per-component error reporting
  (CUE diagnostics are a known weak spot — Discussion #3723); embedding
  runs require a local Ollama install. Mitigations: per-component
  validation with indexed error paths; content-addressed embedding cache
  committed to the private repo so rebuilds don't need Ollama.
- **Risks accepted, with tripwires:**
  - *sqlite-vec stalls or breaks pre-1.0.* Tripwire: a kit Go upgrade or
    ncruces bump that the pinned bindings can't follow → re-evaluate
    (fallback: plain-Go dot products over ~24MB of vectors is trivial).
  - *Index outgrows `go:embed` taste.* Tripwire: `components.sqlite`
    exceeds ~100MB → ship it beside the binary (content-addressed download)
    instead of inside.
  - *Embedding model rot.* Vectors are invalid if the model changes; the
    digest is pinned in the manifest. Tripwire: pinned model unavailable in
    current Ollama → re-embed under a new manifest entry, never mix spaces.
  - *Licensed-content leakage.* The index embeds licensed code into
    `appkit-mcp`. Tripwire: any part of the kit being open-sourced →
    licensed artifacts must be split out first (D13 vendoring rules).

## Revisit when

- The corpus grows past ~100k vectors (other sources joined the index) —
  brute force stops being free; reconsider ANN then, not before.
- sqlite-vec ships 1.0 (re-pin and simplify) or goes unmaintained for 12
  months (swap in plain-Go scoring).
- parquet-go stops releasing for 12 months, or the dump format changes
  (downloader v4) — re-validate stage 1 assumptions.
- The mobile phase arrives — wazero-on-iOS interpreter performance gets
  measured against the D9 plan-B drivers.
- The Assistant reaches parity and the legacy `tailwindplus` MCP server
  retires — the JSON demotes from "serving" to "archival input".
