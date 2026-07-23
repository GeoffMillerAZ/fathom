# Adoption docs (owner directive 2026-07-23, vision 11 companion)

"Make sure it gets documented very well for how to adopt — especially for
Claude Code and Codex use cases and spec driven development."

Design (normative):
- Three definitive guides under docs/adoption/, each a complete
  zero-to-working path a newcomer can follow without reading any other doc,
  each ending with a verification checklist ("you know it works when…").
  docs/use-cases/ keeps its per-concern how-tos; adoption/ owns first wiring.
- **Claude Code guide** (docs/adoption/claude-code.md): install/build;
  daemon vs one-shot serve; MCP registration (HTTP, /rpc/agent surface,
  restart caveat); the hook stack (session brief, PostToolUse obligations
  injection, Stop-hook, pre-push verify gate) with copy-paste
  settings.json/lefthook blocks; per-project .fathom/ (layers, rules);
  provider lanes incl. claude-cli subscription (no API key needed); the
  15-tool agent surface with when-to-use-what; troubleshooting (stale
  binary vs stale data, staleness_report).
- **Codex guide** (docs/adoption/codex.md): fathom as MCP server for Codex
  clients; codex-cli provider lane (Ping tripwire) for compiles/evals; the
  same gates via plain git hooks where Codex's harness differs from Claude
  Code's; explicit table of which Claude-Code-specific features have Codex
  equivalents and which don't (honesty over symmetry).
- **SDD guide** (docs/adoption/spec-driven-development.md): the full loop —
  markdown specs (ADR-018), plan grammar ({id, symbol, package} rows),
  done-means-proven verify gate, computed progress (plans status/next as
  E3 lands), archival lifecycle (E4), spec-bindings layer pack, oracle
  classes. Written against this very program as the worked example.
- Maintenance contract: each guide carries a `## Maintained` line per repo
  convention; every phase that adds/renames a verb updates the guides in
  the same slice (enforced by review, recorded in each phase plan's critic
  pass).

## Tasks

- [x] docs/adoption/claude-code.md — complete zero-to-working guide + verification checklist {id: AD-1, symbol: registerServerStats, package: internal/mcp, skip_test: true, skip: "docs row anchored to the served surface it documents; accuracy enforced by the critic pass against live config"}
- [x] docs/adoption/codex.md — Codex wiring + honest capability table {id: AD-2, symbol: registerServerStats, package: internal/mcp, skip_test: true, skip: "docs row, same anchor rationale as AD-1"}
- [x] docs/adoption/spec-driven-development.md — the SDD loop with this program as the worked example {id: AD-3, symbol: cmdVerify, package: cmd/fathom, skip_test: true, skip: "docs row anchored to the verify CLI at the loop's center"}
- [x] README + docs index route newcomers to adoption/ first; use-cases cross-linked {id: AD-4, symbol: registerServerStats, package: internal/mcp, skip_test: true, skip: "docs routing row, same anchor rationale as AD-1"}
