# Fathom Architecture

Fathom is a code-intelligence + evidence engine. It ingests source across one or many
repositories, extracts symbols, call edges, and facts, stores them as governed evidence
observations, and answers concern-scoped queries — for AI coding assistants, cross-repo
blast-radius, change review, incident investigation, and fleet health.

See also: [ADR.md](../ADR.md) for all decisions; [why-duckdb.md](why-duckdb.md) for the
storage rationale; [mcp-tools.md](mcp-tools.md) for the MCP surface reference.

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
                            │ []Observation (DuckDB)
                            ▼
┌───────────────────────────────────────────────────────────────────┐
│  Layer 3 — Knowledge Query Layer                                  │
│  store → typed results per query / MCP tool call                  │
│  concern-scoped · evidence-tier ranked · staleness-aware          │
│  optional hybrid semantic search (agentic-kit embedding)          │
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

### Extraction (ADR-004)

| Language | Extractor |
|---|---|
| Go | Native `go/parser` / `go/ast` — fastest, most complete, extracts symbols + call edges + cyclomatic complexity |
| Python, JS/TS, Java, Ruby | Deterministic heuristic extractors (tree-sitter/WASM is the documented future path) |
| HCL/Terraform | Native HCL parser → `TerraformResource` nodes |
| Kubernetes YAML | Native k8s decoder → workload kind/name/namespace |
| CUE | `cue/ast` → definitions and constraints |
| Markdown | Overlapping 2000-char chunks with nearest-heading context |
| `go.mod` | Module dependency versions |

Even with CGO available for DuckDB, the extractor layer stays CGO-free and deterministic
(ADR-004). Go AST is strictly better than tree-sitter for Go code, so no WASM cost is paid
there.

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
scores by keyword/semantic overlap and truncates to `TopN`. Forcing a concern keeps results
relevant and reduces LLM token cost significantly (measured ~77% token reduction vs.
dumping all symbols).

```go
type Query struct {
    ServiceID  string
    ConcernID  string       // mandatory
    Keywords   []string
    Kinds      []SourceKind // optional filter
    TopN       int          // 0 → default 20
    SemanticWt float64      // 0 → keyword-only
}
```

### Hybrid semantic search

When `SemanticWt > 0`, the engine runs a keyword query and a vector search in parallel,
then merges results by blended score. Embeddings are produced via agentic-kit's
`embedding` package.

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

## DuckDB store (ADR-003)

`internal/store` — DuckDB-backed (`github.com/marcboeker/go-duckdb/v2`) behind a `Store`
interface. See [why-duckdb.md](why-duckdb.md) for the full rationale.

The `Store` interface is implemented by two types:

| Type | Use |
|---|---|
| `DuckStore` | Production — persists to a `.duckdb` file. Auto-creates parent directory. |
| `MemoryStore` | Tests — in-process fake; shares the contract via `contract_test.go`. |

Schema tables in the DuckDB store:

| Table | Contents |
|---|---|
| `observations` | Main evidence table — all `Observation` rows including complexity scores |
| `fingerprints` | SHA-256 hash-gate index (stored separately; cheap to clear independently) |
| `knowledge_cards` | LLM-generated structured summaries per symbol (opt-in enrichment) |
| `doc_summaries` | Document-level summaries for Markdown sources |

Default store path: `workspace/db/fathom.duckdb` (gitignored; all artifacts go under
`workspace/` — see the workspace convention in [usage.md](usage.md)).

---

## MCP server (ADR-010)

`internal/mcp` — JSON-RPC 2.0 over stdio or HTTP. See [mcp-tools.md](mcp-tools.md) for
the full tool reference.

### Server modes

| Mode | Command | Transport | Use case |
|---|---|---|---|
| stdio | `serve-mcp` | JSON-RPC 2.0 on stdin/stdout | Editor / agent clients (Claude Code, etc.) — zero ports |
| HTTP | `serve-http` | POST `/rpc` (full surface), POST `/rpc/{surface}` (scoped), GET `/events` SSE, GET `/healthz`, GET `/metrics`, POST `/reload` | Multi-client, platform / CI use |

### Tool groups, profiles, and surfaces

Tools are grouped into four profiles. On stdio, clients call `set_profile` to filter the
visible surface (valid values: the four groups plus `companion`); over HTTP `set_profile`
is rejected and each request selects an immutable surface via the path — `POST
/rpc/{surface}` filters `tools/list` and hard-gates `tools/call` (`companion` is a curated
14-tool coding hot loop; the group surfaces mirror the profiles):

| Profile | Tool group | Purpose |
|---|---|---|
| `coding` | `GroupCoding` | Symbol navigation, complexity, call-graph, impact analysis |
| `investigation` | `GroupInvestigation` | Cross-service, incident, callers/callees, test impact |
| `shared` | `GroupShared` | Always available: `set_profile`, `find_symbol`, meta tools, security scans |
| `platform` | `GroupPlatform` | Fleet-wide: list services, cross-service search, drift, cross-repo callers |

### Result cache

`internal/resultcache` memoizes expensive derived graphs (callgraphs, impact trees) within
a warm server session. The cache is invalidated on any ingest event so tools never return
stale derived data.

### Call logging

`serve-http` appends one JSONL record per tool call (tool, service, group, surface,
latency, error; `batch` sub-calls are logged individually with `via:"batch"`) to the
`--call-log` path, default `workspace/logs/mcp-calls.jsonl`. The writer is injectable
for testing.

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
}
```

The `platform-callers` CLI subcommand and the `cross_repo_callers` / `platform_callers`
MCP tools use this graph to answer "which services call this symbol?"

`PlatformStore` (`internal/store/platform_store.go`) loads many `*.duckdb` files, routes
queries by service ID, and supports atomic `Reload()` — used by the HTTP server's
`POST /reload` endpoint.

---

## Supporting packages

| Package | Role |
|---|---|
| `internal/analysis/*` | 33 pure analysis packages (callgraph, complexity, cognitive, apisurface, antipattern, routes, deps, security, markers, cloc, …) |
| `internal/depgraph` | Import-cycle detection (Tarjan SCC) |
| `internal/filter` | `**`-glob ignore patterns with negation |
| `internal/resultcache` | Memoisation of expensive derived graphs |
| `internal/llm` | Knowledge-card enrichment via agentic-kit `llmclient` |
| `internal/vector` | Embedding store via agentic-kit `embedding` |
| `internal/llmsec` | Token redaction via agentic-kit `security` |
| `pkg/client` | Public Go API — wires connector, pipeline, store, query engine |
| `pkg/platform` | Public cross-repo graph API |
