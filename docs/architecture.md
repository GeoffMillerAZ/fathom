# Fathom Architecture

Fathom is a code-intelligence + evidence engine. It ingests source across one or many
repositories, extracts symbols, call edges, and facts, stores them as governed evidence
observations, and answers concern-scoped queries — for AI coding assistants, cross-repo
blast-radius, change review, incident investigation, and fleet health.

See also: [ADR.md](../ADR.md) for all decisions; [why-duckdb.md](why-duckdb.md) for the
original (now historical — SQLite is the only engine, ADR-025) storage rationale;
[mcp-tools.md](mcp-tools.md) for the MCP surface reference; `docs/intent/README.md` for
the project-intent router.

---

## Three-layer architecture (ADR-001)

```
┌───────────────────────────────────────────────────────────────────┐
│  Layer 1 — Connector SPI                                          │
│  Discover / Fetch sources → []SourceRef                           │
│  Impls: LocalConnector · GitHubConnector · FauxConnector          │
└───────────────────────────┬───────────────────────────────────────┘
                            │ []SourceRef
                            ▼
┌───────────────────────────────────────────────────────────────────┐
│  Layer 2 — Source Fusion Pipeline                                 │
│  SourceRef → bytes → extractors → Observation records in store    │
│  3-pass: filter → parallel fetch → deterministic post-fetch       │
│  hash-gate · budget-gate · lazy mode · parallel extraction        │
└───────────────────────────┬───────────────────────────────────────┘
                            │ []Observation (store: SQLite — the only on-disk backend, ADR-025)
                            ▼
┌───────────────────────────────────────────────────────────────────┐
│  Layer 3 — Knowledge Query Layer                                  │
│  store → typed results per query / MCP tool call                  │
│  concern-scoped · keyword+tier ranked · staleness-aware           │
│  cross-repo graph verbs walk typed edges in memory                │
└───────────────────────────────────────────────────────────────────┘
```

Each layer is independently testable and replaceable (ADR-001). The query layer never
touches a network; the pipeline never knows where bytes came from.

---

## Layer 1 — Connector SPI (ADR-002)

```go
type Connector interface {
    Discover(ctx context.Context, svc ServiceRef) ([]SourceRef, error)
    Fetch(ctx context.Context, ref SourceRef) ([]byte, error)
}
```

Three implementations:

| Implementation | What it does |
|---|---|
| `LocalConnector` | Reads from a local directory tree. No network. Fast. Default for dev. |
| `GitHubConnector` | Fetches via the `gh` CLI; requires `gh` to be installed. `svc.Repo` must be set. |
| `FauxConnector` | Wraps `testdata/fauxtown/` fixture data. Deterministic, no secrets. Used in all integration tests. |

`FauxConnector` is the backbone of the TDD strategy (ADR-002, ADR-012): the whole fusion
+ query stack is exercised against fixed fixture data with no network or credentials.

---

## Layer 2 — Source Fusion Pipeline (ADR-005)

`internal/fusion` — 3-pass, deterministic, hash-gated.

### Pipeline passes

**Pass 1 — Filter (sequential)**
Skips `vendor/`, `testdata/`, generated files (`.pb.go`), and anything matching
`DefaultIgnorePatterns`. Uses `**`-glob matching with negation support (`internal/filter`).

**Pass 2 — Fetch (parallel)**
N concurrent workers fetch raw bytes into pre-allocated slots. First error via atomic CAS;
no mid-flight cancellation (workers drain cleanly).

**Pass 3 — Post-fetch (sequential, input order)**
Re-imposes input order to guarantee determinism despite parallel fetch, then for each file:

1. SHA-256 fingerprint of content.
2. **Hash-gate**: compare against stored fingerprint. Skip unchanged files. This makes
   re-ingest of a large repo near-free when few files changed.
3. Purge stale observations for changed files.
4. **Budget gate**: abort if total pending bytes would exceed the configured budget.
5. Queue as pending for extraction.

Pending files are bucketed by language and extracted.

### Extraction (ADR-004, ADR-013, ADR-021 §4)

| Language | Extractor |
|---|---|
| Go | Native `go/parser` / `go/ast` — fastest, most complete, extracts symbols + call edges + cyclomatic complexity |
| Python, JavaScript/TypeScript/JSX/TSX (`.js`/`.jsx`/`.mjs`/`.cjs`/`.ts`/`.tsx`/`.mts`/`.cts`), Ruby | Tree-sitter via **wazero/WASM** (`internal/extractor/tsengine`, ADR-021 §4) — tree-sitter's own core C runtime + one grammar per language, compiled to a self-contained wasm32-wasi module, no CGO. Calibration-gated against the CGO incumbent per language (`docs/plans/extraction-treesitter.md`); query-driven (`.scm` files), not a hand-rolled walk. `.mjs`/`.cjs` (ESM/CommonJS) share the JS grammar; `.mts`/`.cts` share the TS grammar (J1, app-kit O18/O32/O33). TypeScript/JavaScript and Python additionally get a per-file import/module-scope resolver when no SCIP index is present for the service (ADR-041 decision 5, `internal/fusion.ResolveFileEdges`, SU-11): parsed imports plus same-file module-scope names resolve call sites to `core.EdgeFileResolved` edges — tier pre-scan, always heuristic confidence, never presented as typed. |
| Kotlin (`.kt`/`.kts`) | Also tree-sitter via **wazero/WASM** (`internal/extractor/tsengine`, wave P2/O79) — but *new-language breadth*, not a calibration swap: fathom had no Kotlin extractor at all before this (O69), so there is no CGO incumbent to gate against (`fwcd/tree-sitter-kotlin`, no field() names — see `internal/extractor/dialects/kotlin/tags.scm`'s header for how that changes the query patterns). |
| Java, C, C++, C#, Rust, Scala, PHP, Bash, Lua | Tree-sitter via the official **CGO** Go bindings (ADR-013) — not yet migrated to the wazero engine (Java additionally gained field/import symbol coverage in wave P2/O79, still on this CGO path) |
| HCL/Terraform | Native HCL parser → `TerraformResource` nodes |
| Kubernetes YAML | Native k8s decoder → workload kind/name/namespace |
| CUE | `cue/ast` → definitions and constraints; typed edges (imports, `#A: #B & {...}` references) additionally resolved package-wide via `cue/load` (ADR-041 decision 5, `internal/fusion.EnrichCUEEdges`, SU-10) |
| Markdown | Overlapping 2000-char chunks with nearest-heading context |
| `go.mod` | Module dependency versions |

Go AST is strictly better than tree-sitter for Go code, so no WASM/CGO cost is paid
there. The *store* backend needs no CGO at all — it is pure-Go SQLite (ADR-020), the
DuckDB backend having been deleted outright once the fleet migrated (ADR-025, SS-8,
`docs/plans/store-sqlite.md`). Extraction and store are independent CGO surfaces, so
today CGO is required for the full `fathom` binary solely by the Java/C/C++/C#/Rust/
Scala/PHP/Bash/Lua row above — migrating a language's extractor off CGO tree-sitter
onto the wazero engine does not, by itself, make the whole binary CGO-free until every
row in that table has moved (`internal/extractor/tsengine` itself already builds and
tests clean under `CGO_ENABLED=0` in isolation; `internal/store` now does too).

**Typed edges beyond syntax (ADR-041 decision 5).** The table above is what every
extractor produces from syntax alone — a symbol table plus call-site TEXT
(`SymbolNode.Calls`), resolved into a best-effort call graph by
`internal/analysis/callgraph`. A second, independent kind of edge exists for Go only,
today: `internal/fusion.EnrichTypedEdges` type-checks the source directly
(`go/packages` + `go/types`) and writes compiler-grade `typed_edges` rows. Every other
language gains the same typed_edges table by IMPORTING an external indexer's SCIP
output instead of fathom running a type-checker of its own: `internal/scip.ImportIndex`
maps a SCIP index's definition/reference occurrences onto fathom's typed edges, keyed
with the SAME package/qualifier convention the language's own extractor already uses
(derived from SCIP's occurrence geometry — `EnclosingRange` containment — never trusted
from the foreign indexer's own symbol grammar), replaced per fathom package exactly like
the native Go pass. `fathom ingest --scip PATH` (repeatable, or auto-detected
`index.scip` / `.fathom/scip/*.scip`) runs the import; `docs/languages.md` tracks which
language has which level (`native` / `scip:<tool>` / `file` — a per-file, non-typed
fallback resolver, tracked but not yet built — / `syntax`, the baseline every table row
above already has) and `fathom capabilities --json`'s `typed_edge_producers` field
reports the same table from code.

### Lazy mode

When `Lazy: true` is set on the pipeline config, files are queued but not extracted until
the first query arrives for that service. Platform mode can therefore load many repos at
startup without paying extraction cost upfront; each service warms on first use.

---

## The data model (internal/core)

All layers share types from `internal/core/model.go`. `pkg/client` re-exports them for
external consumers.

### Observation

`Observation` is the atomic evidence record persisted in the store. Every symbol, fact, or
chunk extracted from a source produces one or more `Observation` rows.

```go
type Observation struct {
    ServiceID    string
    ConcernID    string       // concern scope; "" = unscoped
    SourceRef    string       // file[:line]
    SourceKind   SourceKind   // GoFunc, GoType, PythonClass, K8sWorkload, …
    EvidenceTier EvidenceTier // verified | pre-scan | doc-claimed | sme-stated | assumed
    Symbol       string       // symbol name, e.g. "ValidateToken"
    Claim        string       // human-readable fact, e.g. "func ValidateToken(ctx, string) error"
    ContentHash  string       // SHA-256 of source content (hash-gate fingerprint)
    Calls        []string     // callee symbol keys
    Complexity   int          // cyclomatic complexity (0 = not applicable)
    IngestedAt   time.Time
}
```

### SymbolNode

`SymbolNode` is the query-layer view of a symbol — produced by extractors and returned by
queries and MCP tools.

```go
type SymbolNode struct {
    Service    string
    Package    string
    Name       string
    Kind       SourceKind
    FilePath   string
    Line       int
    Signature  string
    Calls      []string     // callee keys
    Tier       EvidenceTier
    Complexity int
}
```

### Evidence tiers (ADR-007, ADR-009)

Fathom inherits evidence tiers from agentic-kit's `review/scoring` foundation. A code fact
never outranks a doc claim in the query layer's confidence ceilings.

| Tier | Origin | Confidence ceiling |
|---|---|---|
| `verified` | Extracted directly from code | 5 |
| `pre-scan` | Pattern-matched, not fully confirmed | 4 |
| `doc-claimed` | Stated in documentation | 2 |
| `sme-stated` | Said by a human expert | 2 |
| `assumed` | Inferred, no direct evidence | 1 |

### SourceKind constants

Well-known kinds include: `GoFunc`, `GoMethod`, `GoType`, `GoVar`, `GoConst`,
`GoInterface`, `PythonFunc`, `PythonClass`, `JSFunc`, `JSClass`, `JSMethod`,
`JavaClass`, `JavaInterface`, `JavaMethod`, `RubyClass`, `RubyModule`, `RubyMethod`,
`TerraformResource`, `K8sWorkload`, `CUEDefinition`, `DocChunk`, `ModuleDep`.
The set is open-ended; new extractors add new kinds.

---

## Layer 3 — Knowledge Query Layer (ADR-006, ADR-008)

`internal/query` — concern-scoped, evidence-tiered, staleness-aware.

### Mandatory concern scoping (ADR-006)

Every `Query` carries a `ConcernID`. The engine filters observations to that concern, then
scores by keyword overlap and evidence tier and truncates to `TopN`. Forcing a concern keeps
results relevant and reduces LLM token cost significantly (measured ~77% token reduction vs.
dumping all symbols).

```go
type Query struct {
    ServiceID string
    ConcernID string       // mandatory
    Keywords  []string
    Kinds     []SourceKind // optional filter
    TopN      int          // 0 → default 20
}
```

### Ranking: keyword overlap + evidence tier

`scoreAndRank` (`internal/query/engine.go`) is the whole of the ranking path: each
observation's score is `keywordOverlap*10 + tierRank`, ties broken by symbol name then
source file for determinism. There is no vector or embedding component — "semantic" in
fathom means a compiled claim (ADR-016's semantic stratum, below), never a vector.

A prior optional hybrid path (`internal/vector`, the query engine's `WithVector` option,
a `SymbolIndexer` blended against `agentic-kit`'s embedding client) was deleted as dead
code: no production caller ever wired an indexer into the engine (ADR-041 decision 2).
Revisit only through vision 31 §5's kill criterion (`docs/vision/31-substrate.md`): a
controlled result on a labelled concept-query set that lexical plus graph cannot match at
≤ 2× tokens. `internal/query`'s `TestQueryEngineHasNoVectorOption` fails the build if a
`*Vector*`/`*Embedding*` option is reintroduced without going through that revisit.

Concept and localization queries that need more than keyword+tier reach for the graph
instead: `find_callers`/`references`/`blast_radius` walk typed call edges, and `localize`
expands seeds through the typed neighbourhood (vision 31 §3 D4) — never a dense one-shot
ranking pass.

### Staleness reporting (ADR-008)

Every `QueryResult` includes a `StalenessReport`:

```go
type StalenessReport struct {
    Sources             []SourceStatus
    ReingestRecommended bool
}
```

Callers must never silently trust stale data. Silent staleness is a correctness trap for AI
consumers — fathom surfaces it explicitly on every query response.

### Contradiction detection

The query engine surfaces `Contradiction` pairs: two observations asserting incompatible
claims about the same symbol, using agentic-kit evidence semantics.

---

## Store (ADR-003, ADR-020, ADR-025)

`internal/store` — pluggable behind a `Store` interface. **SQLite (pure-Go,
`modernc.org/sqlite`, WAL mode) is the only on-disk backend** since ADR-020 made it the
default and SS-8 (ADR-025, `docs/plans/store-sqlite.md`) then deleted the DuckDB backend
(`github.com/marcboeker/go-duckdb/v2` and its bindings) outright — the trigger ("operator
runs migrate-store on the fleet DB and the daemon runs a full day green on sqlite") fired
2026-07-23. See [why-duckdb.md](why-duckdb.md) for the original DuckDB rationale (now
historical) and the ADR-020/ADR-025 entries in `ADR.md` for why SQLite replaced it and
why the removal was safe: cross-process concurrent readers beside a live writer — the
property DuckDB's single-writer lock never gave the daemon — outweighed DuckDB's
columnar-scan advantage for this workload, and once every store had migrated there was no
remaining reason to keep the driver (or the CGO it required) compiled in.

The `Store` interface is implemented by two types:

| Type | Use |
|---|---|
| `SQLiteStore` | The only on-disk backend — persists to a `.sqlite`/`.db` file, WAL mode, uncapped read pool. Auto-creates parent directory. |
| `MemoryStore` | Tests — in-process fake; shares the contract via `contract_test.go`. |

`store.Open` (and the lower-level `store.BackendForPath`) dispatch to `SQLiteStore` by
file extension (`.sqlite`/`.db`). A `.duckdb` path is still *recognized* — so a leftover
pre-SS-8 file still participates in `PlatformStore`'s same-stem collision detection
instead of silently vanishing — but `BackendForPath` refuses to ever open one: it returns
a loud, typed error naming the file and pointing at a pre-SS-8 build, never a silent
fallback. `PlatformStore`, `gen-platform-manifest`, and `platform-ingest` all share this
dispatch, so a stale `.duckdb` sitting beside a `.sqlite` sibling loses the collision
cleanly; a lone `.duckdb` with no sibling fails loudly the moment it would be the winner.

Schema tables (SQLite-only now; the DDL no longer needs to port across backends):

| Table | Contents |
|---|---|
| `observations` | Main evidence table — all `Observation` rows including complexity scores |
| `fingerprints` | SHA-256 hash-gate index (stored separately; cheap to clear independently) |
| `knowledge_cards` | LLM-generated structured summaries per symbol (opt-in enrichment) |
| `doc_summaries` | Document-level summaries for Markdown sources |
| `service_meta`, `commits`, `semantic_claims`, `claim_links`, `claim_events`, `compile_runs`, plus `knowledge_cards`/`doc_summaries` above | Non-derivable tables (the eight the historical migration ported): corpus root + watermark, git history, and the semantic stratum's claims (cards/summaries are LLM-derived enrichment a re-ingest does not rebuild); `observations`/`fingerprints` always rebuild by re-ingest |
| `typed_edges`, `typed_edge_fingerprints` | Go type-checked call edges (`direct` / `iface-dispatch`, ADR-021 §3), hash-gated per package |

The on-disk format is specified in `docs/spec/store-format-v0.md` (`application_id`
`0x46415448`, `user_version` ≥ 1 — the markers `serve-http --module` checks at mount).

Default store path: `workspace/db/fathom.sqlite` (gitignored; all artifacts go under
`workspace/` — see the workspace convention in [usage.md](usage.md)).

---

## MCP server (ADR-010)

`internal/mcp` — JSON-RPC 2.0 over stdio or HTTP. See [mcp-tools.md](mcp-tools.md) for
the full tool reference.

### Server modes

| Mode | Command | Transport | Use case |
|---|---|---|---|
| stdio | `serve-mcp` | JSON-RPC 2.0 on stdin/stdout | Editor / agent clients (Claude Code, etc.) — zero ports |
| HTTP | `serve-http` | POST `/rpc` (full surface), POST `/rpc/{surface}` (scoped), GET `/events` SSE, GET `/healthz`, GET `/metrics`, GET `/statusz`, POST `/reload` | Multi-client, platform / CI use |

### Tool groups and surfaces

Tools are grouped into five internal groups. There is no profile-switching tool (the old
`set_profile` was removed — surfaces-week1 SW-2, amending ADR-014's stdio clause): a
**stdio** session always sees the full registry; over **HTTP** each request selects an
immutable surface via the path (`internal/mcp/surface.go`). `POST /rpc/{surface}` filters
`tools/list` only — an off-surface `tools/call` still **executes** and is logged
`off_surface:true` (execute-and-flag, ADR-014 amendment, 2026-07-16); surfaces are
ergonomics and measurement, never a security boundary. The pre-amendment hard gate
survives behind `Server.strictSurfaces` only (the hosted `/rpc/modules` surface went with SR-17).
`agent` (`/rpc/agent`; alias `companion`) is the curated 19-tool coding allowlist; the
group surfaces mirror the groups plus `GroupShared`; the registry was 54 tools after the 2026-08-23 prune and the surface reckoning
(ADR-053) has removed more since (`memory_search` by SR-13).

| Surface | Tool group | Purpose |
|---|---|---|
| `coding` | `GroupCoding` | Symbol navigation, complexity, call-graph, impact analysis |
| `investigation` | `GroupInvestigation` | Cross-service, incident, callers/callees, test impact |
| `shared` | `GroupShared` | Always available: `find_symbol`, `obligations`, `ingest`, meta tools, security scans |
| `platform` | `GroupPlatform` | Fleet-wide: list services, cross-service search, drift, cross-repo callers |
| `modules` | `GroupModules` | Read-only expert-module host (`serve-http --module`; hosted mode narrows it to an explicit allowlist) |

### Result cache

`internal/resultcache` memoizes expensive derived graphs (callgraphs, impact trees) within
a warm server session. The cache is invalidated on any ingest event so tools never return
stale derived data.

### Call logging

`serve-http` appends one JSONL record per tool call (tool, service, group, surface,
latency, error; `batch` sub-calls are logged individually with `via:"batch"`) to the
`--call-log` path, default `logs/mcp-calls.jsonl` under the repo's state directory
(`internal/statedir`, O63 item 5) — outside the repo tree unless that repo's own
`.fathom/config.toml` sets `[daemon] state_dir = "workspace"` (fathom's own repo does;
see `docs/usage.md` §workspace/ convention). The writer is injectable for testing.

---

## Cross-repo platform graph (ADR-009)

`pkg/platform` + `internal/platformlink`

`platform.Build(repos map[string]string)` walks all Go source files in each named repo,
extracts exported symbols and cross-package references, and joins them into directed
caller→callee edges. It is a pure function: same repos → same graph, no shared state, no
network.

The atomic unit is `CrossRepoEdge`:

```go
type CrossRepoEdge struct {
    FromService string
    FromFile    string
    ToService   string
    ToKey       string
    ToFile      string
    Kind        CrossRepoEdgeKind // "" (call) or "service_ref" (VB-30)
}
```

The `platform-callers` CLI subcommand and the `cross_repo_callers` / `platform_callers`
MCP tools use this graph to answer "which services call this symbol?"

### Edge kinds (vision 02 §4.7: one at a time)

`CrossRepoEdge.Kind` classifies the relationship an edge records. It is additive —
every edge produced before the field existed carries the zero value — and grows one kind
at a time:

| Kind | Value | Producer | Join key | Tier |
|---|---|---|---|---|
| call | `""` (`core.CrossRepoEdgeCall`) | `platformlink.BuildFromSymbols` | bare `pkg.Name` string equality | heuristic (`core.CrossRepoEdgeConfidenceOf`) |
| service_ref | `"service_ref"` (`core.CrossRepoEdgeServiceRef`) | `internal/analysis/serviceref.ServiceRefEdges` (VB-30) | normalized `(HTTP method, path template)` | heuristic, never typed |

`service_ref` (docs/vision/29-verbs-not-lookups.md §I) is the edge that lets blast radius
cross a *process* boundary rather than only a *repo* boundary within one call graph: a
client in one service issues an HTTP request whose method + path template matches a
route another service registers, and `internal/analysis/serviceref` joins the two sides
across the whole fleet (never within the same service — that is an ordinary intra-service
call, not a cross-repo reference). `ToKey` carries `"METHOD /normalized/template"` (the
same `"METHOD path"` convention `internal/mcp`'s `traverse` tool already uses for
`graph.KindRoute`), never a bare symbol name.

Extraction is two-sided:

- **Server side** — `internal/analysis/routes.Extract` (already used by the `endpoints`
  analyzer and the `traverse` MCP tool): `net/http`, chi, gin and echo route
  registrations, language-agnostic across Go/JS/TS/Python.
- **Client side** — `internal/analysis/serviceref.ExtractClientCalls`: Go-only for VB-30
  (language coverage for a brand-new edge kind grows one language at a time, same rule as
  the edge kind itself), parsed with `go/parser` and walked as real `ast.CallExpr` nodes
  (never a byte-level regex over raw source — a call written inside a `//` comment is
  plain text to the parser, never an AST node, so it can never manufacture a hop)
  detecting `http.NewRequest[WithContext]` / `http.Get` / `http.Post` / `http.Head` /
  `http.PostForm` call sites. The method argument accepts a quoted literal ("GET") or a
  net/http `Method*` constant (`http.MethodGet`); the URL argument accepts a string
  literal or an inline `fmt.Sprintf("template", args...)` call, whose `%`-verbs
  `NormalizeTemplate` collapses to `"{}"`.

`NormalizeTemplate` collapses `:param` (chi/gin/express, segment-anchored: the colon must
be the first character of its segment, so `/events/at:noon` stays literal), `{param}`
(gorilla/echo, including a `{param:regex}` constraint), a `*wildcard` segment (anywhere
in the path, not only the last segment), and a printf verb (`%s`, `%d`, `%02d`, …) into
one placeholder, `"{}"`; strips a leading scheme+host ONLY when the string starts with
one (an unanchored search would be fooled by a decoy like
`/redirect?next=http://evil.com/path`); strips a query string/fragment; and normalises a
trailing slash — so `"/items/:id"`, `"/items/"` and `"http://host/items/%s"` all compare
equal to `"/items/{}"` / `"/items"` as appropriate.

`blast_radius`, `cross_repo_blast_radius` and `review_diff` each report matching
`service_ref` hops as an additive `service_ref_hops` array (one object per hop, always
`"kind":"service_ref"`, `"tier":"heuristic"`) — computed fleet-wide via
`serviceref.BuildFromStore`, the same no-caching-layer precedent
`cross_repo_blast_radius` already sets for the `call`-kind graph. A `(method, template)`
key registered by more than one service fans out to all of them (edges are never dropped
for being ambiguous) and each such hop carries `"ambiguous":true` plus the full sorted
`"candidates"` list, so a consumer can tell a genuine fan-out from a unique join.
`docs/bench/` documents how VB-31 (precision/recall ≥ 0.90 against a hand-verified route
table) will be measured.

`PlatformStore` (`internal/store/platform_store.go`) loads many store files
(`*.sqlite`/`*.db`, plus `*.duckdb` recognized only so a pre-SS-8 leftover can lose a
same-stem collision rather than vanish silently — see the Store section above), routes
queries by service ID, and supports atomic `Reload()` — used by the HTTP server's
`POST /reload` endpoint.

---

## The semantic stratum (ADR-016) — claims, compile, sweep, obligations

Above the symbol index sits `internal/semantic`: a **layer pack** (`docs/layers.md`;
`schema/semantic/schema.cue` `#Layer`, with a `stratum` field per ADR-024) compiles a
governing corpus (AGENTS.md and CLAUDE.md, ADR.md, plans, findings) into **governed claims** —
citation-grounded, evidence-tiered, content-addressed by prompt version — stored in
`semantic_claims` / `claim_links` / `claim_events` / `compile_runs`. `fathom compile`
(dry-run by default; eval-gated) and the `compile_layer` tool write them; the daemon's
post-refresh **sweep** (`internal/semantic/sweep.go`) re-anchors citations and flags
drift/contradiction rather than falsifying; `fathom claim refute|corroborate` is the
human triage. Serving is the `obligations` tool (rules governing a file at decision time,
capped and displacement-disclosed) — injected by the PostToolUse edit hook and, for
`.fathom/guard.cue` scopes, the PreToolUse guard (`docs/design/guarded-edits.md`). The
`findings` layer adds review/audit conclusions with remediation drift
(`docs/design/findings-as-evidence.md`, `review_diff`, `findings_near`); expert modules
package a compiled store for read-only mounting (`docs/design/expert-modules.md`,
`docs/spec/expert-module-v0.md`; **experimental**, law 9). Founding design:
`docs/design/semantic-layers.md`.

## The 2026-08 subsystems — instrument, not hand

Each is a small package with one verb and one design doc; none edits the user's prose
(fathom reports, the session decides):

| Subsystem | Package | Surface | Design |
|---|---|---|---|
| Capability registry — one list of what is on and why anything is off; `capabilities.cue` user/project (law 13) with the `budgets` block (BA-9, experimental) | `internal/capability` | `fathom capabilities`, doctor row, observatory | `docs/design/capability-registry.md`, `docs/design/budgeted-answers.md` |
| Config custodian — falsifiable concerns over AGENTS.md/CLAUDE.md; six check kinds, one of which (`stanza_tools_served`) asks the daemon and degrades honestly | `internal/custodian` | `fathom config audit`, doctor `concerns`, session-brief line | `docs/design/config-custodian.md` |
| Continuity threads — the work, not the session; repo-scoped, byte-budgeted brief (bet: experimental, 4/20) | `cmd/fathom/thread.go` (`~/.fathom/threads.jsonl`) | `fathom thread …`, SessionEnd/PreCompact hooks, `fathom prime` | `docs/design/continuity-threads.md` |
| Wiring + receipts (law 15) — plan → apply → receipt for every file fathom writes in a repo; hand-edited tracked regions blocked, `undo` reverses | `internal/wiring` | `fathom init`, `fleet reconcile --apply`, `fleet receipts|undo`, `/observatory/wiring/` | `docs/design/self-healing-fleet.md` |
| Scope + purge — reversible suppression disclosed in every envelope; irreversible purge with a byte probe | `internal/scope` | `fathom scope …`, `fathom purge …` | `docs/design/scope-and-purge.md` |
| Observability + research — call-log/hooks-log metrics, the observatory (30-day series, grade history experimental), the sanitized research bundle | `internal/metrics`, `internal/research`, `cmd/fathom/observatory*.go` | `fathom metrics …`, `/observatory`, `fathom research …` | `docs/design/observability-surfaces.md`, `docs/design/research-flywheel.md` |

---

## Supporting packages

| Package | Role |
|---|---|
| `internal/analysis/*` | Pure analysis packages (callgraph, complexity, cognitive, apisurface, antipattern, deps, security, cloc, review, findings, …) |
| `internal/depgraph` | Import-cycle detection (Tarjan SCC) |
| `internal/filter` | `**`-glob ignore patterns with negation |
| `internal/resultcache` | Memoisation of expensive derived graphs |
| `internal/verify`, `internal/sdd` | The done-means-proven gate (ADR-018) and the plan grammar (`ctx:`, `needs:`, leases) |
| `internal/llm` | Knowledge-card enrichment via agentic-kit `llmclient` |
| `internal/llmsec` | Token redaction via agentic-kit `security` |
| `pkg/client` | Public Go API — wires connector, pipeline, store, query engine; re-exports `ReadFailuresError` |
| `pkg/platform` | Public cross-repo graph API |
