# Architecture Decision Records — sextant

Sextant is a code-intelligence + evidence engine: it ingests source across one or
many repos, extracts symbols/edges/facts, stores them as **evidence
observations**, and answers concern-scoped queries (for AI coding assistants,
cross-repo blast-radius, investigations, fleet health). It exposes a Go client,
a CLI, and an MCP server.

This file records the decisions that shape the implementation, each with the
"why" rooted in a design lesson or constraint. Status: `accepted` unless noted.

---

## ADR-001 — Three-layer architecture

**Decision.** Strictly separate three layers:

1. **Connector SPI** — abstract source (`Discover`/`Fetch`); produces `SourceRef`s.
2. **Source Fusion Pipeline** — `SourceRef → bytes → extractors → Observation` records in the store; hash-gating, parallel extraction, lazy mode.
3. **Knowledge Query Layer** — `store → typed results` per query/tool; concern-scoped filtering, staleness assessment, optional hybrid semantic search.

**Why.** Each layer is independently testable and replaceable. The query layer
never touches a network; the pipeline never knows where bytes came from.

## ADR-002 — Connector SPI with a deterministic Faux connector

**Decision.** `Connector{ Discover(ctx, ServiceRef) ([]SourceRef, error); Fetch(ctx, SourceRef) ([]byte, error) }` with three impls: `LocalConnector` (disk, no network), `GitHubConnector` (gh CLI), `FauxConnector` (wraps `testdata/fauxtown/`).

**Why.** Tying ingest to a live VCS API created dev friction and non-deterministic
tests. With Faux, the entire fusion + query stack is tested against fixed fixture
data — no network, no credentials, golden-file stable. **Faux is the backbone of
the TDD strategy.**

## ADR-003 — Storage: DuckDB (supersedes the original SQLite choice)

**Decision.** Persist observations and derived tables in **DuckDB** (`github.com/marcboeker/go-duckdb/v2`).

**Context / change.** The original design used `modernc.org/sqlite` (pure-Go,
CGO-free) specifically because the project had a no-CGO constraint. **That
constraint has been lifted.** DuckDB is a columnar analytical engine that fits
sextant's workload far better: large `observations` scans, group-bys for
hotspots/metrics/fleet rollups, and (future) Parquet export for the analytical
substrate. CGO build cost is accepted.

**Consequences.** Tests and CI run with `CGO_ENABLED=1`. The store package hides
DuckDB behind an interface (`Store`) so the engine is swappable and unit-testable
with an in-memory fake. Schema tables: `observations` (main evidence),
`knowledge_cards`, `cross_ref_index`, `doc_summaries`, `runbooks`; fingerprints
in a separate store (cheap to update / clear independently).

## ADR-004 — CGO-free tree-sitter via wazero/WASM; Go via native `go/parser`

**Decision.** Multi-language extraction runs tree-sitter grammars compiled to
**WASM, executed in the pure-Go `wazero` runtime** (no CGO, no Docker, golden
testable). **Go** uses the stdlib `go/parser`/`go/ast` directly (fastest, most
complete). HCL, k8s YAML, and CUE use their native Go parsers.

**Why.** Even with CGO available for DuckDB, the WASM tree-sitter path keeps the
extractor layer deterministic and free of native grammar build chains. Go AST is
strictly better than tree-sitter for Go, so we don't pay WASM cost there.

## ADR-005 — Hash-gated, deterministic 3-pass fusion pipeline

**Decision.** Pipeline runs: **Pass 1 Filter** (sequential, ignore vendor/
testdata/generated), **Pass 2 Fetch** (N parallel workers into pre-allocated
slots, first error via atomic CAS, no mid-flight cancel), **Pass 3 post-fetch**
(sequential, in input order: SHA-256 fingerprint → hash-gate → purge stale →
budget gate → pending). Then bucket pending by language and extract.

**Why.** Re-ingesting a 15k-file repo where 14.9k files are unchanged must be
near-free (hash compares only). Determinism (same repo → same result) requires
the post-fetch pass to re-impose input order even though fetch was parallel.
Budget gating prevents runaway cost on unexpectedly large repos.

## ADR-006 — Mandatory concern-scoped queries

**Decision.** Every query carries a `concern_id`; the engine filters observations
to that concern, then scores by keyword/semantic overlap and truncates to TopN.

**Why.** Dumping all symbols for a service into an LLM context was expensive and
noisy (~measured 77% token reduction by scoping). Forcing a concern makes callers
specific and keeps results relevant and cheap.

## ADR-007 — Evidence tiers reused from agentic-kit

**Decision.** Observations carry an **evidence tier** from agentic-kit's
`foundations/review/scoring` foundation: `verified` (code, ceiling 5),
`pre-scan` (4), `doc-claimed` (2), `sme-stated` (2), `assumed` (1). The query
engine respects ceilings when scoring — a doc claim never outranks a code fact.

**Why.** sextant is a downstream consumer of the agentic-kit platform; evidence
governance is exactly what that platform provides. We import it rather than
reinvent. See ADR-009.

## ADR-008 — Staleness is always reported

**Decision.** Every query result includes a staleness assessment (which sources,
when last ingested, whether re-ingest is recommended).

**Why.** Callers must distinguish "data is fresh" from "re-ingest before trusting
this." Silent staleness is a correctness trap for AI consumers.

## ADR-009 — Depend on agentic-kit as the platform substrate

**Decision.** sextant imports `github.com/geoffmilleraz/agentic-kit` (resolved
locally via `go.work` + a `replace` directive during co-development). Reuse:
`review/scoring` (evidence tiers), `evidence/*` (observation/contradiction
shapes), `telemetry`, `llmclient` (knowledge-card enrichment), `security`
(token redaction / DLP posture), `contracts`.

**Why.** Avoids duplicating governed-evidence primitives; keeps the two projects
coherent. Changes to agentic-kit are made there when a shared primitive needs to
evolve.

## ADR-010 — MCP server: stdio + HTTP, tool groups, profiles

**Decision.** Expose tools over MCP: `serve-mcp` (stdio JSON-RPC 2.0, zero ports,
for editor/agent clients) and `serve-http` (JSON-RPC + SSE, multi-client,
platform mode loads all stores at startup). Tools are grouped
(`coding`/`investigation`/`shared`/`platform`); clients select a profile via
`set_profile` to filter the surface. A `resultcache` memoizes expensive derived
graphs and is cleared on any ingest.

**Why.** Editors want a local stdio server with no ports; platforms want a shared
HTTP server. Tool groups keep the surface relevant to the task at hand.

## ADR-011 — Generalized / org-agnostic, open-source-ready

**Decision.** No organization-specific names, hosts, catalogs, or product
references anywhere in code, docs, tests, or fixtures. Anything org-specific is
configuration. The `testdata/fauxtown/` fixtures use invented, neutral names.

**Why.** The project is intended to be open-sourced and adopted by any org.

## ADR-012 — Test-driven development with golden fixtures

**Decision.** Build TDD: write the failing test (often against `FauxConnector` /
`testdata/fauxtown/`) before the implementation; extractor and query outputs are
asserted with golden files; determinism is an explicit, tested property.

**Why.** A code-intelligence engine is only trustworthy if its outputs are exact
and reproducible. Tests encode that contract.
