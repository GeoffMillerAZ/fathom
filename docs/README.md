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
| [use-cases/](use-cases/) | Per-concern how-tos once fathom is wired in; [coding-agents.md](use-cases/coding-agents.md) is the canonical short wiring summary the adoption guides expand on |
| [architecture.md](architecture.md) | Three-layer design, data model, store (SQLite-only, pure Go — ADR-020/ADR-025), fusion pipeline, MCP surface, cross-repo graph |
| [usage.md](usage.md) | Quickstart, every CLI subcommand with flags and examples, Go client API, MCP client connection |
| [configuration.md](configuration.md) | `~/.fathom/config.toml` / project config: LLM provider profiles, precedence, secrets doctrine, `config`/`providers`/`smoke` |
| [layers.md](layers.md) | Semantic-layer catalog (`obligations`, `findings`, `adr-rules`, `spec-bindings`), module-pack shape, the `fathom layers` verb |
| [mcp-tools.md](mcp-tools.md) | Full reference table of all MCP tools grouped by profile, plus prompts and resources |
| [why-duckdb.md](why-duckdb.md) | Why DuckDB replaced SQLite originally, and why ADR-020 reversed that — DuckDB itself was then deleted outright in SS-8/ADR-025 (superseded and removed — historical rationale) |

The authoritative decision log is [ADR.md](../ADR.md).
The implementation roadmap and status live in [tmp/TRACKING.md](../tmp/TRACKING.md) (gitignored).
