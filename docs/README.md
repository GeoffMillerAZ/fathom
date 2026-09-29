# Fathom Docs

Engineer-facing documentation for the fathom code-intelligence + evidence engine.

**New to fathom? Start at [adoption/](adoption/)** — a zero-to-working guide
per AI coding assistant ([claude-code.md](adoption/claude-code.md),
[codex.md](adoption/codex.md),
[spec-driven-development.md](adoption/spec-driven-development.md)), each
self-contained end-to-end. Once wired in, [use-cases/](use-cases/) covers
what to *do* with it — one how-to per concern (coding context, cross-repo
blast radius, spec verification, incident investigation, change review,
fleet health).

| Document | What it covers |
|---|---|
| [adoption/](adoption/) | Zero-to-working setup guides, per client — the first stop for a newcomer |
| [adoption/README.md](adoption/README.md) | **Start here** — the 15-minute path, what is proven, and what to push back on |
| [use-cases/](use-cases/) | Per-concern how-tos once fathom is wired in; [coding-agents.md](use-cases/coding-agents.md) is the canonical short wiring summary the adoption guides expand on |
| [architecture.md](architecture.md) | Three-layer design, data model, store (SQLite-only, pure Go — ADR-020/ADR-025), fusion pipeline, MCP surface, cross-repo graph |
| [usage.md](usage.md) | Quickstart, every CLI subcommand with flags and examples, Go client API, MCP client connection |
| [configuration.md](configuration.md) | `~/.fathom/config.toml` / project config: LLM provider profiles, precedence, secrets doctrine, `config`/`providers`/`smoke` |
| [layers.md](layers.md) | Semantic-layer catalog (`obligations`, `findings`, `adr-rules`, `spec-bindings`), module-pack shape, the `fathom layers` verb |
| [mcp-tools.md](mcp-tools.md) | Full reference table of every registered MCP tool by group and HTTP surface (the 19-tool agent allowlist), plus prompts and resources — test-synced to the registry |
| [why-duckdb.md](why-duckdb.md) | Why DuckDB replaced SQLite originally, and why ADR-020 reversed that — DuckDB itself was then deleted outright in SS-8/ADR-025 (superseded and removed — historical rationale) |
| [spec/](spec/) | The publishable Governed Store Format v0 + Expert Module v0 specs (product-vision.md §4 H2) — normative, falsifiability-indexed, separate from this directory's engineer-facing how-tos |
| [intent/](intent/) | **Project intent** — the router ([intent/README.md](intent/README.md)): principles, invariants, domain, [scope](intent/scope.md) (goals, non-goals, compatibility surface), futures, decisions index, autonomy, incidents, open questions |
| [assessments/](assessments/) | Dated research summaries (market, neighbouring tools, peer stacks, the feature scorecard, the language atlas, the Jev levels, the semantic-layers evidence), each opening with a plain-language verdict and citing the decisions it drove; data beside it in `assessments/data/` ([assessments/README.md](assessments/README.md) has the conventions) |
| [plans/](plans/) | Gated work orders (grammar v2: [plans/README.md](plans/README.md)); `fathom verify` proves them, `fathom plans status` computes progress |
| [guides/fleet-index.md](guides/fleet-index.md) | Fleet index, daemon ops, `task deploy`, dogfood discipline |

The authoritative decision log is [ADR.md](../ADR.md), indexed at [intent/decisions/README.md](intent/decisions/README.md).
Implementation status is computed, never asserted: `fathom plans status --service fathom`;
scope and roadmap live in [intent/scope.md](intent/scope.md).
