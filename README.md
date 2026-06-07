# sextant

**Code-intelligence + evidence engine.** Sextant ingests source across one or
many repositories, extracts symbols, call edges, and facts, stores them as
**governed evidence observations**, and answers *concern-scoped* queries — for AI
coding assistants, cross-repo blast-radius, change review, incident
investigation, and fleet-wide health.

It ships three surfaces: a Go client (`pkg/client`), a CLI (`cmd/sextant`), and
an **MCP server** (stdio + HTTP) that editors and agents connect to.

> General-purpose and org-agnostic — no vendor-specific names or catalogs;
> anything org-specific is configuration (see `ADR.md`, ADR-011).

## Three-layer architecture (ADR-001)

```
Connector SPI        Discover/Fetch sources   (Local / GitHub / Faux)
        │
        ▼
Source Fusion        SourceRef → bytes → extractors → Observation records
Pipeline             hash-gated, parallel, deterministic, lazy
        │
        ▼
Knowledge Query      store → typed results per query/tool
Layer                concern-scoped · evidence-tiered · staleness-aware
```

- **Go** is parsed natively (`go/parser`); other languages via tree-sitter
  grammars run in the pure-Go **wazero/WASM** runtime (ADR-004).
- Storage is **DuckDB** (ADR-003).
- Evidence tiers (`verified` › `pre-scan` › `doc-claimed` › `sme-stated` ›
  `assumed`) come from **[agentic-kit](../agentic-kit)** — sextant builds on that
  governed-evidence platform (ADR-009).

## Quickstart

```sh
task            # list tasks
task doctor     # toolchain + workspace health
task validate   # full gate: fmt-check + vet + cue-vet + go test -race (CGO/DuckDB)
lefthook install
```

Requires: Go 1.26+, **CGO enabled** (DuckDB), [CUE](https://cuelang.org) 0.16+,
[go-task](https://taskfile.dev), [lefthook](https://github.com/evilmartians/lefthook).
The sibling `../agentic-kit` module is wired via `go.work` + a `replace`.

## Usage (planned surfaces)

```go
// Go client
c := client.NewConstantClient(client.ModeLocal)
c.Ingest(ctx, client.ServiceRef{ID: "my-service", Path: "."})
res, _ := c.QuerySymbols(ctx, client.Query{ServiceID: "my-service", ConcernID: "auth", Keywords: []string{"token"}})
callers, _ := c.FindCallers(ctx, "my-service", "ValidateToken")
```

```sh
sextant ingest    --service my-service --path .
sextant inspect   --service my-service --kind GoFunc
sextant serve-mcp                       # stdio MCP server for editors/agents
```

## Docs

| | |
|---|---|
| [docs/architecture.md](docs/architecture.md) | Three-layer design, data model, fusion pipeline, DuckDB store, MCP surface |
| [docs/usage.md](docs/usage.md) | Quickstart, all CLI subcommands, Go client API, connecting an MCP client |
| [docs/mcp-tools.md](docs/mcp-tools.md) | Full MCP tool reference: all tools, prompts, and resources |
| [docs/why-duckdb.md](docs/why-duckdb.md) | Why DuckDB replaced SQLite and the CGO trade-off |

## Layout

```
cmd/sextant/   CLI entry point (ingest, inspect, serve-mcp, watch, gen-*)
pkg/           Public API surface importable by other repos (client, platform)
internal/      All implementation (connector, fusion, store, extractor, query,
               mcp, analysis, platformlink, depgraph, llm, vector, sdd, adr, …)
testdata/      fauxtown/ — deterministic fixtures for tests (no secrets)
docs/          Explainers
tmp/           Scratch: converted design docs + TRACKING.md (gitignored)
```

Implementation status and the full roadmap live in `tmp/TRACKING.md`; the
design docs that drive it are in `tmp/design_inspiration/files/`. Key decisions
are in `ADR.md`.
