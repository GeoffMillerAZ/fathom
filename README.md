# fathom

**Code-intelligence + evidence engine.** Fathom ingests source across one or
many repositories, extracts symbols, call edges, and facts, stores them as
**governed evidence observations**, and answers *concern-scoped* queries — for AI
coding assistants, cross-repo blast-radius, change review, incident
investigation, and fleet-wide health.

It ships three surfaces: a Go client (`pkg/client`), a CLI (`cmd/fathom`), and
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
- Storage is pure-Go **SQLite** (WAL mode) — the only on-disk backend
  (ADR-020); DuckDB (ADR-003) was deleted outright once the fleet migrated
  (ADR-025, SS-8). The store needs no CGO at all; the CGO build requirement
  below is entirely the tree-sitter extractor bindings (ADR-013).
- Evidence tiers (`verified` › `pre-scan` › `doc-claimed` › `sme-stated` ›
  `assumed`) come from **[agentic-kit](../agentic-kit)** — fathom builds on that
  governed-evidence platform (ADR-009).

## Adopting fathom (start here)

Wiring fathom into an AI coding assistant is a first-class, definitive path —
not an afterthought bolted onto the quickstart below. Pick your client:

| | |
|---|---|
| [docs/adoption/README.md](docs/adoption/README.md) | **Start here** — the 15-minute path, what this project has actually proven, what it has NOT, and where a reviewer should push hardest |
| [docs/adoption/claude-code.md](docs/adoption/claude-code.md) | **Claude Code** — build/install, the daemon, MCP registration, the full hook stack (copy-paste blocks), provider lanes, the 19-tool agent surface, troubleshooting, and a verification checklist |
| [docs/adoption/codex.md](docs/adoption/codex.md) | **Codex** — MCP server registration, the `codex-cli` provider lane, git-native gates, and an honest table of which Claude-Code-specific features have no Codex equivalent yet |
| [docs/adoption/spec-driven-development.md](docs/adoption/spec-driven-development.md) | **Spec-driven development** — the full markdown-spec → gated-plan → done-means-proven verify loop, using this repo as the worked example |

Each guide is self-contained end-to-end; `docs/use-cases/` (below) then
covers what to *do* with fathom once it's wired in, one how-to per concern.

The fast path in either guide is `fathom init --skills[=claude|codex|all]`:
one command wires the repo (ingest, hooks, the `CLAUDE.md`/`AGENTS.md`
operating-contract stanza) **and** installs the nine shipped agent skills
(`fathom-activate`/`custodian`/`doctor`/`feedback`/`impact`/`onboard`/`plan`/
`rewire`/`verify`, `dist/agents/skills/`) for your harness. If anything about the wiring
looks off afterward, `fathom doctor` (not the Taskfile's `task doctor`
below — a different, CLI-native check) is the one-command diagnosis: MCP
registration, daemon health, binary freshness, hooks, stanza, skills,
config, obligations, and traffic, each with a one-command remedy.

## Quickstart

```sh
task            # list tasks
task doctor     # toolchain + workspace health
task validate   # full gate: fmt-check + vet + cue-vet + vendor-check + store-purego + go test -race
task deploy     # build, sign, install, restart the daemon, assert it serves HEAD — never a raw cp (O25)
lefthook install
```

Requires: Go 1.26+, **CGO enabled** (tree-sitter extractor bindings, ADR-013 —
not the store, which is pure Go), [CUE](https://cuelang.org) 0.16+,
[go-task](https://taskfile.dev), [lefthook](https://github.com/evilmartians/lefthook).
The sibling `../agentic-kit` module is wired via `go.work` + a `replace` for
local dev. Don't have `../agentic-kit`? Nothing to do — `go.mod`'s `replace`
falls back to an in-tree mirror (`third_party/agentic-kit`) automatically;
see [docs/distribution.md](docs/distribution.md).

## Usage (the three surfaces)

```go
// Go client (pkg/client) — SQLite store path, "" for in-memory
c, err := client.NewConstantClient(client.ModeLocal, "workspace/db/fathom.sqlite")
rep, err := c.Ingest(ctx, client.ServiceRef{ID: "my-service", Path: "."}) // rep.ReadFailed names unreadable files
res, _ := c.QuerySymbols(ctx, client.Query{ServiceID: "my-service", ConcernID: "auth", Keywords: []string{"token"}})
callers, _ := c.FindCallers(ctx, "my-service", "ValidateToken")
```

```sh
fathom ingest    --service my-service --path .          # exit 1 + `read-failed: N file(s)` when files could not be read
fathom inspect   --service my-service --kind GoFunc
fathom verify    --service my-service --plan docs/plans/<plan>.md [--quiet|--strict|--require-all]
fathom serve-mcp  --store workspace/db/fathom.sqlite     # stdio MCP server (full registry)
fathom serve-http --store workspace/db/fathom.sqlite     # the daemon, 127.0.0.1:7575; agents use POST /rpc/agent
```

Full CLI reference: [docs/usage.md](docs/usage.md); MCP tools and surfaces:
[docs/mcp-tools.md](docs/mcp-tools.md).

## Docs

| | |
|---|---|
| [docs/what-is-fathom.html](docs/what-is-fathom.html) | **Start here** — visual explainer of what fathom is and how it works |
| [docs/intent/README.md](docs/intent/README.md) | **Project intent** router — principles (laws 1–15), invariants, domain, [scope](docs/intent/scope.md) (goals, non-goals, compatibility surface), futures, decisions index over `ADR.md`, autonomy, incidents, open questions |
| [docs/adoption/](docs/adoption/) | **Wire it in** — [README.md](docs/adoption/README.md) is the start-here; zero-to-working guides per AI coding assistant: [claude-code.md](docs/adoption/claude-code.md), [codex.md](docs/adoption/codex.md), [spec-driven-development.md](docs/adoption/spec-driven-development.md) |
| [docs/use-cases/](docs/use-cases/) | **Use it** — one how-to per concern once you're wired in: coding context, cross-repo blast radius, spec verification, incident investigation, change review, fleet health, intent-corpus audit |
| [docs/tokenomics-explainer.html](docs/tokenomics-explainer.html) | Visual deep-dive: Claude Code tokenomics, how fathom bends the curve, tool comparisons (observed vs speculative, labeled) |
| [docs/architecture.md](docs/architecture.md) | Three-layer design, data model, fusion pipeline, store, MCP surface |
| [docs/usage.md](docs/usage.md) | Quickstart, CLI subcommand reference, Go client API, connecting an MCP client |
| [docs/configuration.md](docs/configuration.md) | `~/.fathom/config.toml` / project config: LLM provider profiles (incl. AWS GovCloud, Azure Government), precedence, secrets doctrine, `config`/`providers`/`smoke` |
| [docs/layers.md](docs/layers.md) | Semantic-layer catalog — `obligations`, `findings`, `adr-rules`, `spec-bindings`, module-pack shape, the `fathom layers` verb |
| [docs/mcp-tools.md](docs/mcp-tools.md) | Full MCP tool reference: every registered tool, the HTTP surfaces (19-tool agent allowlist), prompts, and resources — test-synced to the registry |
| [docs/distribution.md](docs/distribution.md) | Building fathom **without agentic-kit access** — the in-tree `third_party/agentic-kit` mirror, air-gapped `go mod vendor`, the `task sync-kit` ritual |
| [docs/why-duckdb.md](docs/why-duckdb.md) | Why DuckDB replaced SQLite, then ADR-020 reversed that and ADR-025/SS-8 removed DuckDB outright — historical rationale |
| [docs/claude-code-integration.md](docs/claude-code-integration.md) | Superseded for wiring by `docs/adoption/claude-code.md` — kept for its measured-assessment content ([assessment-claude-code-integration.md](docs/assessment-claude-code-integration.md)) |

## Layout

```
cmd/fathom/   CLI entry point (ingest, verify, plans, thread, init, doctor,
               serve-mcp, serve-http, …) and the observatory UI (app-kit)
pkg/           Public API surface importable by other repos (client, platform)
internal/      All implementation (connector, fusion, store, extractor, query,
               mcp, analysis, verify, wiring, custodian, memory, sdd, adr, …)
internal/connector/testdata/fauxtown/  deterministic fixture fleet (no secrets)
docs/          Adoption guides, use cases, architecture, gated plans, intent
tmp/, ref/, workspace/  Scratch, references, build artifacts (all gitignored)
```

Implementation status is computed, never asserted: `fathom plans status
--service fathom` (the verify gate over every active `docs/plans/*.md`).
Scope and roadmap: `docs/intent/scope.md` via the router `docs/intent/README.md`.
Key decisions are in `ADR.md` (indexed at `docs/intent/decisions/README.md`).
