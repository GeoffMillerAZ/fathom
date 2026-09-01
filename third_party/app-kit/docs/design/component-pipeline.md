# Component data pipeline — design

The build-time pipeline that turns the 336MB TailwindPlus JSON dump into
the kit's queryable component corpus: CUE-validated typed records →
canonical zstd Parquet shards → one generated SQLite hybrid index with
pinned-model embeddings. Decision and alternatives:
`../adr/0010-component-data-pipeline.md` (D10). Consumers: the Assistant
(`mcp-design-assistant.md`, ADR-0011) and the Gallery (ADR-0012).

Dataset ground truth (verified against the live MCP server, 2026-06-10):
657 components, dump version `2026-02-02-230729`, downloader v3.0.0,
taxonomy `Section.Group.Category.Name` across sections **Application UI**,
**Ecommerce**, **Marketing**; variant axes framework (`html`|`react`|`vue`)
× tailwind_version (`3`|`4`) × mode (`light`|`dark`|`system`|`none` —
Ecommerce uses `none` only); variant payload `{version, full_name, name,
code, framework, language, tailwind_version, mode, supportsDarkMode}`.
Raw JSON source-of-truth location and copy instructions:
`../../data/tailwindplus/README.md` (the
m2ext volume is currently TCC-blocked — we don't duplicate those
instructions here). **Licensed content: everything downstream of the dump
is private-only, forever.**

---

## 1. CUE schemas

Per D7, the schema is authored once, in CUE, in a Go-representable subset.
It lives at `cue/tailwindplus/tailwindplus.cue` (kit CUE packages live
under `cue/`, one directory per package) and is `go:embed`-ed into the
pipeline binary so validation never depends on file-system layout.

```cue
package tailwindplus

#Framework:       "html" | "react" | "vue"
#TailwindVersion: 3 | 4
#Mode:            "light" | "dark" | "system" | "none"
#Section:         "Application UI" | "Ecommerce" | "Marketing"

#Variant: {
	version:          string & !=""
	full_name:        string & !=""   // "Application UI.Elements.Badges.Flat"
	name:             string & !=""
	code:             string & !=""
	framework:        #Framework
	language:         string & !=""   // "html" | "jsx" | "vue" as emitted by the dump
	tailwind_version: #TailwindVersion
	mode:             #Mode
	supportsDarkMode: bool
}

#Component: {
	section:   #Section
	group:     string & !=""
	category:  string & !=""
	name:      string & !=""
	full_name: string & !=""          // joined taxonomy, must equal the parts
	variants: [#Variant, ...#Variant] // at least one

	// Ecommerce ships unthemed; everything else never ships "none".
	if section == "Ecommerce" {
		variants: [...{mode: "none"}]
	}
	if section != "Ecommerce" {
		variants: [...{mode: "light" | "dark" | "system"}]
	}
}
```

Notes:

- The enums are *closed disjunctions of literals* — they validate at
  runtime via cuecontext, and `cue exp gengotypes` (experimental as of CUE
  v0.16.1, 2026-04-08 —
  https://cuelang.org/docs/howto/generate-go-types-from-cue-definitions/)
  degrades them to `string`/`int` in generated Go. That is acceptable per
  D7: generated types are shape, cuecontext is law. Generated types are
  committed at `internal/twplus/schema/cue_types_tailwindplus_gen.go` with
  a CI check that regeneration is clean.
- The conditional mode rules are validation-only — they don't affect the
  Go shape, so they cost nothing in gengotypes terms.
- The artifact manifest schema (`#Manifest`, section 7) lives in the same
  CUE package.

## 2. Pipeline stages and code layout

The pipeline is a `kit` CLI subcommand family — `appkit data` — not a
separate binary; D13's monorepo gives us one module and one CLI.

```
cmd/appkit/                      # the kit CLI (D13)
  data.go                        # `appkit data {build,parquet,embed,index,verify}`
cue/tailwindplus/tailwindplus.cue
internal/twplus/
  schema/                        # embedded CUE + generated Go types
  parse/                         # streaming dump decode → raw records
  validate/                      # per-component cuecontext validation
  normalize/                     # variant explosion, content hashing, dedup
  parquetio/                     # shard writer/reader (parquet-go)
  enrich/                        # embed-text builder: taxonomy + class extraction
  embed/                         # Ollama /api/embed client, Voyage client, cache
  index/                         # SQLite DDL + builder + hybrid query helpers
  manifest/                      # manifest assembly + verification
```

`internal/` is deliberate: within the monorepo, `cmd/appkit-mcp` and the
Gallery can import `internal/twplus/index` for querying, but downstream
apps (which consume `appkit` as a module) cannot touch licensed-data
tooling.

Subcommands compose the stages:

| Command | Stages | Needs |
|---|---|---|
| `appkit data parquet` | parse → validate → normalize → parquet write | the raw JSON |
| `appkit data embed` | enrich → embed | Parquet shards + Ollama (or `--embedder=voyage`) |
| `appkit data index` | index build → manifest | Parquet shards + embedding cache |
| `appkit data build` | all of the above | everything |
| `appkit data verify` | re-run hashing, compare to manifest | artifacts only |

### Stage: parse

A streaming `json.Decoder` walks the dump's tree
(section → group → category → component → variants) and yields one raw
component at a time — we never hold 336MB of decoded structures at once.
Payloads that are not `#Variant`-shaped (the dump also carries preview
payloads) are passed through opaquely to a side shard (section 3) and
excluded from v1 indexing.

### Stage: validate

Each raw component is unified against `#Component` individually — 657
small unifications, not one giant one — so a failure reports
`component[full_name]: <cue error>` instead of an opaque wall. cuecontext
is stable and concurrency-safe as of CUE v0.16.x
(https://github.com/cue-lang/cue/releases/tag/v0.16.0); we validate on a
worker pool. cuelang.org/go is pinned exactly (v0.16.1, per D7).

### Stage: normalize

Variant explosion and dedup. One output row per variant, plus
content-addressed code blobs:

```go
package normalize

type VariantRow struct {
	// Taxonomy (denormalized onto every row; Parquet columns compress this away)
	Section  string `parquet:"section,dict"`
	Group    string `parquet:"group,dict"`
	Category string `parquet:"category,dict"`
	Name     string `parquet:"name,dict"`
	FullName string `parquet:"full_name"`

	// Variant axes
	Framework       string `parquet:"framework,dict"`
	Language        string `parquet:"language,dict"`
	TailwindVersion int32  `parquet:"tailwind_version"`
	Mode            string `parquet:"mode,dict"`
	SupportsDark    bool   `parquet:"supports_dark_mode"`
	Version         string `parquet:"version,dict"`

	// Content addressing
	ContentHash string `parquet:"content_hash"` // hex sha256 of raw code bytes
	Code        string `parquet:"code,zstd"`
}

// Key returns the canonical sort key: full taxonomy, then axes.
func (r VariantRow) Key() string {
	return r.FullName + "\x00" + r.Framework + "\x00" +
		strconv.Itoa(int(r.TailwindVersion)) + "\x00" + r.Mode
}
```

`ContentHash = sha256(code)` serves two purposes: dedup (where the dump
repeats identical code bytes across mode axes — common when
`supportsDarkMode` theming is carried by `dark:` classes in one payload —
all such variants share one stored blob and one embedding; the measured
dedup ratio is recorded in the manifest, not assumed here) and
content-addressing for the embedding cache (stage: embed).

### Stage: parquet write

parquet-go v0.30.1 (2026-05-18, pure Go —
https://github.com/parquet-go/parquet-go/releases), `GenericWriter`, zstd:

```go
func writeShard(path string, rows []normalize.VariantRow) error {
	slices.SortFunc(rows, func(a, b normalize.VariantRow) int {
		return cmp.Compare(a.Key(), b.Key())
	})
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	w := parquet.NewGenericWriter[normalize.VariantRow](f,
		parquet.Compression(&zstd.Codec{}), // fixed default level — never tweak per-run
	)
	if _, err := w.Write(rows); err != nil {
		return err
	}
	return w.Close()
}
```

Sharding is by `section/group` (slugified). If a group's shard would
exceed ~15MB, it splits deterministically by category
(`elements-00.parquet`, `elements-01.parquet`, …). The ~15MB ceiling keeps
every shard under the ~20MB upload limits agent tools impose, with margin.

### Stage: enrich

Builds the text that gets embedded — *not* the raw code. One embed-text
per variant:

```
Application UI > Elements > Badges > Flat
react · tailwind v4 · dark · supports dark mode
classes: inline-flex items-center rounded-md px-2 py-1 text-xs font-medium ring-1 ...
```

Line 1 is the taxonomy (the strongest signal — these names are
human-curated). Line 2 is the axes. Line 3 is extracted Tailwind classes:
`class=`/`className=`/`:class=` attribute values parsed from the code,
frequency-ranked, deduplicated, capped (~120 tokens) so boilerplate
(`flex`, `p-4`) doesn't drown distinctive utilities (`grid-cols-7`,
`aria-checked:`). Qwen3-Embedding's 32k context
(https://github.com/QwenLM/Qwen3-Embedding) would fit whole code bodies,
but embedding curated descriptions beats embedding markup soup for
retrieval relevance — and keeps embed cost trivial. The embed-text is
stored in the index (it's also useful display material for search results).

### Stage: embed

- **Default: local Ollama** (v0.30.7, 2026-06-07 —
  https://github.com/ollama/ollama/releases), model
  `qwen3-embedding-0.6b`, output **768 dims**, batched through
  `POST /api/embed` (the endpoint accepts an input array). Qwen3-Embedding
  supports user-defined output dimensions
  (https://github.com/QwenLM/Qwen3-Embedding); if the serving API doesn't
  expose a dims parameter, the client truncates to 768 (MRL) and
  re-normalizes. Vectors are L2-normalized at write time either way.
- **The model digest is pinned**: at first run the tool records the Ollama
  model digest into the manifest; subsequent runs refuse to embed if the
  local digest differs (`--repin` to consciously migrate, which invalidates
  the cache).
- **Optional flag: `--embedder=voyage`**, model `voyage-code-3`
  (2048/1024/512/256 dims; 200M free tokens per account covers this corpus
  many times over — https://docs.voyageai.com/docs/pricing). Flagged, not
  default, because query-time then needs the same API key — the Ollama path
  keeps query-time search fully local and offline.
- **Content-addressed and resumable.** Cache key =
  `sha256(model_digest ‖ dims ‖ embed_text)` → 768 float32s, stored in
  `data/tailwindplus/embeddings/<model>-<dims>/cache.sqlite`. A re-run
  embeds only rows whose key is absent — a dump update touching 20
  components costs 20×~12 embeddings, not 8k. The cache is committed to
  the private repo so CI and clean machines rebuild the index with no
  Ollama at all.

### Stage: index build

One SQLite file, built via ncruces/go-sqlite3 v0.34.x (cgo-free, FTS5 +
sqlite-vec capable — https://github.com/ncruces/go-sqlite3/releases) with
the sqlite-vec ncruces WASM bindings
(https://github.com/asg017/sqlite-vec-go-bindings). DDL:

```sql
CREATE TABLE component (
  id                 INTEGER PRIMARY KEY,
  section            TEXT NOT NULL,
  "group"            TEXT NOT NULL,
  category           TEXT NOT NULL,
  name               TEXT NOT NULL,
  full_name          TEXT NOT NULL UNIQUE,
  supports_dark_mode INTEGER NOT NULL
);

CREATE TABLE blob (                      -- content-addressed, deduped code
  content_hash TEXT PRIMARY KEY,         -- hex sha256
  code_zstd    BLOB NOT NULL,            -- klauspost/compress zstd
  raw_len      INTEGER NOT NULL
);

CREATE TABLE variant (
  id               INTEGER PRIMARY KEY,
  component_id     INTEGER NOT NULL REFERENCES component(id),
  framework        TEXT NOT NULL CHECK (framework IN ('html','react','vue')),
  tailwind_version INTEGER NOT NULL CHECK (tailwind_version IN (3,4)),
  mode             TEXT NOT NULL CHECK (mode IN ('light','dark','system','none')),
  language         TEXT NOT NULL,
  version          TEXT NOT NULL,
  content_hash     TEXT NOT NULL REFERENCES blob(content_hash),
  embed_text       TEXT NOT NULL,
  UNIQUE (component_id, framework, tailwind_version, mode)
);
CREATE INDEX variant_axes ON variant (framework, tailwind_version, mode);

-- Contentless FTS5: BM25 index only, no stored text. The code text exists
-- only transiently at build time; storing it again here would balloon the
-- file. We never need snippet()/highlight() — get_component returns full
-- code from blob. rowid == variant.id.
CREATE VIRTUAL TABLE fts_variant USING fts5(
  full_name, taxonomy, embed_text, classes,
  content='',
  tokenize="unicode61 tokenchars '-_:'"   -- keep 'grid-cols-3', 'dark:' whole
);

-- sqlite-vec brute-force KNN. Partition keys make the common filtered
-- searches scan only their slice; no ANN at ~8k vectors, on purpose.
CREATE VIRTUAL TABLE vec_variant USING vec0(
  variant_id INTEGER PRIMARY KEY,         -- == variant.id
  embedding  FLOAT[768] distance_metric=cosine,
  framework  TEXT PARTITION KEY,
  section    TEXT PARTITION KEY,
  +full_name TEXT
);
```

The hybrid query is the documented sqlite-vec RRF pattern
(https://alexgarcia.xyz/blog/2024/sqlite-vec-hybrid-search/index.html),
one statement, FULL OUTER JOIN (SQLite ≥ 3.39):

```sql
WITH vec_matches AS (
  SELECT variant_id, row_number() OVER (ORDER BY distance) AS rn
  FROM vec_variant
  WHERE embedding MATCH :query_vec
    AND k = :k
    AND framework = :framework            -- partition-key prune
),
fts_matches AS (
  SELECT rowid AS variant_id, row_number() OVER (ORDER BY rank) AS rn
  FROM fts_variant
  WHERE fts_variant MATCH :query_text
  LIMIT :k
),
fused AS (
  SELECT COALESCE(v.variant_id, f.variant_id) AS variant_id,
         COALESCE(1.0/(:rrf_k + v.rn), 0.0)
       + COALESCE(1.0/(:rrf_k + f.rn), 0.0) AS score
  FROM vec_matches v
  FULL OUTER JOIN fts_matches f USING (variant_id)
)
SELECT c.full_name, va.framework, va.tailwind_version, va.mode, fused.score
FROM fused
JOIN variant   va ON va.id = fused.variant_id
JOIN component c  ON c.id = va.component_id
WHERE (:section IS NULL OR c.section = :section)
  AND (:tw      IS NULL OR va.tailwind_version = :tw)
  AND (:mode    IS NULL OR va.mode = :mode)
ORDER BY score DESC
LIMIT :limit;
```

`:rrf_k` defaults to 60 (the standard constant), `:k` to 64. When no
embedder is reachable, `vec_matches` is skipped and the query runs
BM25-only with the same structural WHERE clauses — graceful degradation is
a code path, not an error.

### Stage: manifest

Assembled last, validated against `#Manifest`, written as
`data/tailwindplus/manifest.json` (section 7).

## 3. Artifact layout

```
data/tailwindplus/
  README.md                          # provenance + copy instructions (exists)
  tailwindplus-components.json       # raw dump, git-ignored (336MB)
  parquet/
    application-ui/
      elements.parquet               # ≤ ~15MB each, zstd, sorted rows
      forms.parquet
      ...
    ecommerce/...
    marketing/...
    _previews/                       # opaque pass-through payloads, unindexed
  embeddings/
    qwen3-embedding-0.6b-768/
      cache.sqlite                   # content-addressed vectors, committed
  index/
    components.sqlite                # THE index; go:embed-ed into appkit-mcp
  manifest.json                      # CUE-validated; the artifact contract
```

Parquet shards, the embedding cache, and the manifest are committed (this
is a private repo; the corpus is ~tens of MB compressed and diffable). The
raw JSON is git-ignored — the Parquet layer *is* its durable typed form.

## 4. Determinism

**Guarantee: same dump bytes + same pipeline version → byte-identical
Parquet shards and manifest.** Enforced by construction — rows sorted by
`VariantRow.Key()`, fixed compression codec and level, fixed row-group
sizing, no timestamps or hostnames anywhere in the output — and verified
by test: `appkit data verify` rebuilds from the raw JSON and compares
SHA-256s against the manifest; CI runs it on every pipeline change.

**The SQLite index is held to logical, not byte, equivalence.** SQLite
file bytes depend on page-allocation history; chasing byte-stability there
buys nothing. `verify` instead compares a canonical dump (ordered SELECTs
over all tables, hashed) — same logical content, pass.

**Embeddings are exempt from byte-determinism, deliberately.** Float
results vary across GPU/CPU backends and runtime versions; they are
reproducible only per pinned runtime, not bit-exact across machines. This
is fine because (a) the cache is committed, so in practice vectors are
computed once and reused everywhere — determinism by storage, not by
recomputation; (b) the manifest pins the model digest and dims, so any
regeneration happens in a consciously chosen, internally consistent
embedding space; (c) vectors only *rank* results — a ±1e-6 wobble cannot
change what `get_component` returns, only (immeasurably) the order of
near-ties. The invariant that matters — never mix vectors from different
models/dims in one index — is enforced by the manifest check, not by bit
equality.

## 5. Manifest

```cue
#Manifest: {
	schema_version: 1
	source: {
		dump_version: string        // "2026-02-02-230729"
		downloader:   string        // "tailwindplus-downloader v3.0.0"
		sha256:       string        // of the raw JSON
		bytes:        int
	}
	counts: {
		components: int             // 657
		variants:   int             // measured at build
		blobs:      int             // post-dedup; dedup ratio falls out
		vectors:    int
	}
	parquet: shards: [...{path: string, sha256: string, bytes: int, rows: int}]
	embedding: {
		provider: "ollama" | "voyage"
		model:    string            // "qwen3-embedding-0.6b"
		digest:   string            // exact Ollama model digest — THE pin
		dims:     768
	}
	index: {path: string, sha256: string, bytes: int, logical_hash: string}
	pipeline_version: string        // module version of the builder
}
```

No `built_at` field: timestamps are the classic determinism leak, and git
already knows when artifacts changed. The manifest is the contract the
Assistant checks at startup (model digest match before attempting semantic
queries) and the thing `verify` verifies.

## 6. Integration

- **The Assistant** (`mcp-design-assistant.md`, ADR-0011): `appkit-mcp`
  `go:embed`s `components.sqlite`, opens it read-only via
  `internal/twplus/index`, and serves the converged search-then-get tool
  shape — `search_components` runs the hybrid RRF query (BM25-only when no
  embedder is reachable), `get_component` decompresses one blob by
  `full_name` + axes. Query embeddings come from the same Ollama model the
  manifest pins; on digest mismatch or daemon absence the Assistant logs
  once and degrades to BM25 + structural. Never a catalog dump (D11).
- **The Gallery** (ADR-0012): browsing is pure structural SQL — sections,
  groups, counts, axes facets — through the same `internal/twplus/index`
  package. Same monorepo, same file, no second data path. The Gallery's
  TailwindPlus browser is reference material for building kit components;
  it renders previews from blob code on demand.
- **DuckDB, dev-only:** ad-hoc analysis over the Parquet shards
  (`SELECT framework, count(*) ... FROM 'parquet/**/*.parquet'`) via the
  DuckDB CLI on the workstation. It ships in nothing and no kit code
  imports it (ADR-0010).

## 7. Future

1. **Reimplement the downloader in-kit.** `tailwindplus-downloader` v3.0.0
   emits the 336MB JSON; its successor (`appkit data pull`) will emit
   `normalize.VariantRow` records straight to Parquet — the JSON stage
   disappears, the CUE schema validates at the source, and dump updates
   become shard-level diffs by construction.
2. **The kit's own components join the corpus.** Add a `corpus` column
   (`tailwindplus` | `kit`) through schema, Parquet, and index; the
   Gallery's component registry (typed templ parameters, D12) emits the
   same `VariantRow` shape + embed-texts. One hybrid search across
   everything — "do we already have one of these?" answered in one query,
   which is the propagation vision (VISION §3) applied to UI.
3. **Preview images.** The dump's preview payloads ride along unindexed in
   `parquet/_previews/`; rendering them (and Gallery screenshots of kit
   components) into thumbnails served by an Assistant `get_preview` tool is
   deferred until the Assistant's text tools prove out.

## 8. Migration

The existing `tailwindplus` MCP server keeps serving agents directly off
the raw JSON — it works today and costs nothing to keep. It retires when
the Assistant reaches **parity**, defined as: `search_components` /
`get_component` / preview cover every query shape the old server handles
(list/search/get by full_name with framework/tailwind_version/mode
parameters), verified by replaying a captured set of real tool calls
against both and diffing results. Until then, both run; after that,
`.mcp.json` entries flip to `appkit-mcp` and the old server's repo is
archived. The raw JSON then demotes to archival input — kept until the
in-kit downloader (Future #1) makes it obsolete entirely.

## Pins (as decided here, all verified 2026-06-10)

| Dependency | Pin | Why it's safe enough |
|---|---|---|
| parquet-go | v0.30.1 (2026-05-18) | pure Go, active releases |
| ncruces/go-sqlite3 | v0.34.x (v0.34.4, 2026-06-04) | D9's driver; FTS5 + sqlite-vec |
| sqlite-vec | v0.1.10-alpha.4 (2026-05-18) + ncruces bindings | pre-1.0: pin exact, vendor WASM blob |
| klauspost/compress (zstd) | pinned in go.mod | blob + shard compression |
| cuelang.org/go | v0.16.1 exact (D7) | upgrades treated like compiler upgrades |
| Ollama | v0.30.7+, model digest pinned in manifest | digest is the real pin, not the daemon |
| Embedding model | qwen3-embedding-0.6b @ 768 dims | 32k ctx, code-retrieval training |

## 9. Implementation note (2026-06-15 — what was actually built)

The first shipped slice (`internal/twplus/`, `cmd/appkit data build`,
`cmd/appkit-mcp`) is the **value floor**, faithful to D2/D7/D9 but deviating
from §§1–8 in recorded ways:

**Schema reality vs. §1.** The §1 `#Variant` was written from the normalized
MCP view, not the raw dump. Verified against the 336MB file:

- The snippet's `name` field **is the framework** (`html` | `vue` | `react`);
  `language` (`html` | `jsx`) is only the syntax (html backs both html and
  vue; jsx backs react). Framework is taken from `name`, not derived from
  `language`. There IS vue (it just rides `language:"html"`).
- The snippet `version` is the integer **Tailwind major**, not a string; the
  dump-version string is top-level only.
- Ecommerce `mode` is JSON `null`; `normalize` maps it to `"none"` before
  validation. There is no `framework` or `full_name` field in the dump — both
  are derived. `preview` is an inline full-HTML field per snippet (258MB of
  336MB), dropped from the index.

The corrected schema lives at `internal/twplus/schema/tailwindplus.cue`
(embedded next to its Go package, matching the kit's tokens-schema precedent,
not `cue/tailwindplus/`). It is validated at runtime via cuecontext;
`gengotypes` is not used (the D7-documented handwritten-types fallback).

**Storage vs. §§2–4.** SQLite-first. The Parquet "durable truth" layer is
**not built yet** — the SQLite index is the artifact, rebuilt from the
git-ignored JSON via `appkit data build`. Determinism is therefore by-rebuild,
not yet byte-stable shards.

**Engine vs. §6.** ncruces v0.35 moved FTS5 to a registerable extension
(`ext/fts5`, per-connection via `driver.Open(dsn, fts5.Register)`); the embed
package is deprecated. **Semantic search is not wired** — search is FTS5 BM25
+ structural filters only (graceful degradation IS the default path today).
Notably ncruces ships `ext/vec1`, a **Go-native** ANN vector extension — a
cleaner embeddings path than the asg017/sqlite-vec C alpha ADR-0010 assumed;
prefer it when embeddings land.

**Lesson:** FTS5 query syntax injects — `-`, `:`, `"` are operators. User text
is tokenized and double-quoted before `MATCH` (`index.ftsQuery`); proven by
the `rounded-md` failure in `spikes/twplus-sqlite-check`.

**Tool surface (D11).** `appkit-mcp` ships `search_components`,
`get_component`, `list_components`, `list_sections`, plus the kit-guidance
family (`get_kit_guidance`, `list_kit_decisions`, `get_kit_decision`) and
ADRs as `kit://adr/<id>` resources. Parity with the legacy `tailwindplus`
server is now documented and spot-verified in
[tailwindplus-parity.md](tailwindplus-parity.md) (retirement owner-gated).

## 10. Semantic search — design & status (2026-06-15/16)

BM25 + structural is the shipped search and meets the G2 bar (15/15 golden
known-items in top-3, `data/tailwindplus/golden-queries.json`). Semantic search
is the planned enhancement — **never a dependency** (ADR-0010): query-time it
degrades to BM25 whenever no embedder is reachable, which is already the live
code path.

**Mechanics — both halves de-risked, cgo-free:**

- FTS5 BM25: `spikes/twplus-sqlite-check`.
- Vector KNN via ncruces **`ext/vec1`** (Go-native, registered per-connection
  like fts5): `spikes/twplus-vec-check`. This **replaces the asg017/sqlite-vec
  C alpha** ADR-0010 assumed — no vendored WASM blob, no pre-1.0 C dependency.
  Proven SQL: `CREATE VIRTUAL TABLE v USING vec1` + `INSERT … VALUES('rebuild',
  '{index:"flat"}')` (brute force — correct at ~8k vectors, no ANN), insert
  with `vec1_from_json(?)`, and KNN as a **table-valued function**:
  `SELECT rowid, distance FROM v(vec1_from_json(?)) LIMIT k` (nearest-first).

**Status: built 2026-06-16, behind a pluggable embedder.** The whole path
ships and is tested end-to-end with a deterministic fake embedder; the only
thing gating *live* use is running `appkit data embed` against a local Ollama
(absent in the build env, so the actual vectors aren't generated here). What
exists:

1. `internal/twplus/embed/` — `Embedder` interface + an Ollama `POST /api/embed`
   client (default model `qwen3-embedding-0.6b`, L2-normalized, dims/count
   validated, loopback default). Unit-tested against `httptest`, no live model.
   *(Deviations from the original plan: the content-addressed embed cache is not
   built — `embeddings.sqlite` is itself the rebuildable artifact — and it is
   **gitignored, not committed**, because it derives from licensed content
   (D10). Model-digest pinning is stubbed (meta stores model+dims; digest empty)
   pending an `/api/show` lookup.)*
2. `appkit data embed` (`task twplus:embed`) writes a **separate**
   `embeddings.sqlite` (a `vec1` flat KNN table, rowid = variant id, model+dims
   pinned in `meta`) — separate so `data build` stays deterministic and
   embedder-free and re-embedding never rewrites the corpus. `EmbedText` comes
   straight from `normalize` (`VariantRow.EmbedText`).
3. Hybrid query: `Index.SearchHybrid` runs BM25 and vec1-KNN, fuses by
   Reciprocal Rank Fusion (k=60), applies the same structural filters, returns
   top-N. Degrades to BM25 on a nil store, empty query vector, or dims mismatch.
   The MCP `search_components` tool auto-enables it when `embeddings.sqlite` is
   present and embeds the query with the store's pinned model; CLI `data search
   -semantic` does the same. On any query-time embed failure it logs once and
   serves BM25.
4. The golden suite is the regression guard: still **15/15** in top-3 on the
   BM25 path. Re-run after generating real embeddings to confirm semantic adds
   recall without dropping any known-item.
