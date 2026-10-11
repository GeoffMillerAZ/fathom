# Scope: current boundaries

Part of the scope router, `../scope.md` (the goal index, the compatibility surface and every section's member).

## Current boundaries

- Languages: Go via `go/parser`; every tree-sitter language (Python, JS/TS, Ruby, Kotlin,
  Java, C, C++, C#, Rust, Scala, PHP, Bash, Lua, …) on wazero (`internal/extractor/tsengine`,
  ADR-048), no CGO (ADR-066); HCL, Kubernetes YAML, CUE, Markdown, `go.mod` natively; tool identity for
  Helm, kustomize, compose, OpenAPI, generated files. Evidence: `docs/architecture.md`
  "Extraction", ADR-021.
- Surfaces: 57 CLI verbs (`fathom help`, counted 2026-09-02; ADR-029 adds `harness`);
  MCP over stdio and HTTP with the full registry, an eight-tool default agent surface and a
  21-tool extended one (ADR-053), which
  now serve annotations and a generated `outputSchema` (`docs/mcp-tools.md`); Go client
  `pkg/client`; the Observatory console on app-kit (`cmd/fathom/console_*.go` over
  `internal/console` projections; `observatory*.go` renderers), plus
  opt-in loopback `theme-guide` over synthetic models (docs/app-kit.md). The guide is
  experimental, source-preserving, separate from daemon startup and not runtime settings.
- Store: pure-Go SQLite in WAL mode, the only on-disk backend; one shared fleet store
  served by one always-on loopback daemon (`serve-http`, `127.0.0.1:7575`) under launchd.
  Evidence: ADR-020, ADR-025, `Taskfile.yml` deploy.
- Harnesses: Claude Code (SessionStart, PreToolUse, PostToolUse, Stop, SessionEnd,
  PreCompact hooks) and Codex (AGENTS.md stanza, `fathom prime`, MCP) both verified live;
  Claude Code reads the AGENTS.md stanza natively (≥2.1.277, ADR-059).
  Under ADR-029 the harness is data in `internal/harness`: Gemini CLI ships `unverified`
  until a real session's beats verify it; Cursor and Copilot are data rows; tiers C/D are
  documented rows. Evidence: `docs/adoption/claude-code.md`, `docs/adoption/codex.md`,
  `docs/vision/25-harness-port.md`.
- Findings loop: an external review kit's findings are importable (`fathom findings
  import`, subject-bound, tier-capped by producer) and served at the edit alongside
  governing rules; fathom never approves, disputes, tailors, or closes one on its own
  authority — the review kit stays the organizational record. Evidence:
  `docs/vision/30-findings-loop.md`, ADR-040.
- Fleet: the repositories on this workstation, reconciled by `fathom fleet reconcile`
  (20 on 2026-09-02, 19 drifted on the dry run; nothing applied). Evidence:
  `CHANGELOG.md` 2026-08-13.
- Substrate: `agentic-kit` via `go.work`, or the byte-pinned in-tree mirror. Evidence: ADR-009, ADR-019.
- Platform: macOS only in practice (launchd daemon, TCC, code signing); no CI runner
  exists (`scripts/ci.sh` is the CI helper, nothing invokes it).
- Index snapshots and environment daemons: designed and parked (`docs/design/index-snapshots.md`, ADR-069
  decisions 11 to 15). Today no index snapshot is exported, imported, or served from a lineup, and fathom serves one
  loopback daemon per workstation. The owner prioritizes the work; the earliest start is after war1 lands and
  RT-M1 reads. Phase start triggers and kills: `docs/vision/38-proof-layer-roadmap.md` section 9.4. The environment
  daemon (F74) also waits on the owner's answer to U1. Its goal, provisionally G38, is opened with the next free
  goal id when phase 1 starts.
