# Why DuckDB?

Sextant uses **DuckDB** (`github.com/marcboeker/go-duckdb/v2`) as its storage engine.
This page explains the reasoning for engineers who wonder why an embedded analytical
database is the right fit, and what the CGO trade-off means in practice.

See [ADR-003](../ADR.md) for the formal decision record.

---

## The original design: pure-Go SQLite

The first design used `modernc.org/sqlite` — a pure-Go, CGO-free SQLite port. The reason
was simple: the project had a hard no-CGO constraint at the time (it was intended to ship
as a single static binary with no native dependencies).

That constraint has since been lifted. When the workload was analyzed honestly, SQLite was
the wrong tool.

---

## Sextant's workload is analytical, not transactional

A typical ingested repo produces tens of thousands of `Observation` rows. The queries
sextant needs to answer efficiently are not transactional point-lookups but analytical
scans:

- **Hotspot detection**: `GROUP BY symbol ORDER BY SUM(complexity) DESC LIMIT 10` over
  the entire `observations` table for a service.
- **Concern-scoped search**: full scan + filter over `(service_id, concern_id)` with
  keyword scoring applied to the result set.
- **Fleet rollups**: `GROUP BY service_id, source_kind COUNT(*)` across all services in a
  platform store, in response to `list_services` and `fleet_drift` tool calls.
- **Cross-service queries**: `query_cross_service` does a substring match over every
  symbol name and claim across all services simultaneously.

SQLite uses a row-oriented B-tree storage format. Reading 50,000 rows to compute a
group-by means materializing every row's worth of bytes, most of which are irrelevant
columns. DuckDB uses **columnar storage**: when computing `SUM(complexity)` it reads only
the complexity column, skipping the symbol, claim, source_ref, and calls columns
entirely. For analytical scans over wide tables this is a fundamental efficiency
advantage.

---

## Concrete reasons DuckDB fits sextant

### 1. Columnar scans over observations

The `observations` table is wide (service_id, concern_id, source_ref, source_kind,
evidence_tier, symbol, claim, content_hash, calls, complexity, ingested_at). Most queries
touch only two or three columns. Columnar reads eliminate the IO and CPU cost of
deserializing irrelevant columns on every row.

### 2. Aggregations for hotspots and fleet metrics

Complexity hotspot ranking, kind-count breakdowns, and fleet-level drift reports all
reduce to SQL aggregations. DuckDB's vectorized execution engine processes these in
batches of 1024 values at a time using SIMD operations. The same GROUP-BY that takes
milliseconds in DuckDB would take noticeably longer in row-oriented SQLite on a large
observation set.

### 3. Parquet-readiness

DuckDB can read and write Apache Parquet natively. This means the same `observations`
table can be exported to Parquet for downstream analytical pipelines (BI tools, dbt,
Spark) without any ETL step. This is a documented future capability (ADR-003: "Parquet
export for the analytical substrate").

### 4. Multi-file platform store

`PlatformStore` loads one `*.duckdb` file per service and routes queries by service ID.
DuckDB handles concurrent read access to multiple files cleanly. The `Reload()` method
performs an atomic hot-swap of the loaded store set without restarting the HTTP server.

---

## The CGO trade-off (ADR-003, ADR-004)

DuckDB requires CGO. That means:

- **Tests and CI run with `CGO_ENABLED=1`** and `MACOSX_DEPLOYMENT_TARGET=14.0`.
- The binary is not fully static. It links against the system's C runtime.
- Cross-compilation to a different OS/architecture requires a cross-compiler toolchain
  pointed at the target's C libraries.

This is accepted. The benefits of columnar storage outweigh the build complexity for this
workload.

Importantly, even though CGO is used for DuckDB, **the extractor layer stays CGO-free and
deterministic** (ADR-004). Multi-language extraction uses heuristic parsers today, with
tree-sitter grammars compiled to WASM run via the pure-Go `wazero` runtime as the
documented future path. The extractor layer never needs CGO regardless of the storage
decision.

### Build command

```sh
CGO_ENABLED=1 go build ./...
CGO_ENABLED=1 go test -race ./...
```

Or via task:

```sh
task validate   # runs with CGO_ENABLED=1 automatically
```

---

## Store interface: swappable under the hood

`internal/store` hides DuckDB behind a `Store` interface:

```go
type Store interface {
    PutObservations(ctx, []Observation) error
    QueryObservations(ctx, serviceID, concernID string) ([]Observation, error)
    Fingerprint(ctx, serviceID, sourceRef string) (hash string, found bool, err error)
    SetFingerprint(ctx, serviceID, sourceRef, hash string) error
    PurgeSource(ctx, serviceID, sourceRef string) error
    PutKnowledgeCards(ctx, []KnowledgeCard) error
    KnowledgeCards(ctx, serviceID string) ([]KnowledgeCard, error)
    PutDocSummaries(ctx, []DocSummary) error
    DocSummaries(ctx, serviceID string) ([]DocSummary, error)
    ListServiceIDs(ctx) ([]string, error)
    Close() error
}
```

`DuckStore` is the production implementation. `MemoryStore` is an in-process fake used in
unit tests — both are contract-tested against the same `contract_test.go` suite so they
stay consistent. If DuckDB ever needed to be swapped (e.g. for a read-replica or a
cloud-native columnar store), only the `DuckStore` struct changes; every caller is
insulated.
