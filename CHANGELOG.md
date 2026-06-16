# Changelog

All notable changes to sextant. Format loosely follows
[Keep a Changelog](https://keepachangelog.com/); dates are ISO-8601.

## [Unreleased]

_Nothing yet._

## [0.2.0] — 2026-06-16

Enhancements driven by mining throughline's real Claude Code sessions (841 sextant calls,
1666 shell greps, a heavy manual freshness loop).

### Added
- **`references` MCP tool** — evidence-tiered, fleet-wide occurrence search (substring /
  whole-word / regex). Returns text matches at the `pre-scan` tier plus symbol definitions
  cross-referenced from the index at their real tier. Turns the ~658 repo-by-repo greps a
  session does into one structured call.
- **`verify_plan` accepts a Markdown checklist** (`plan_markdown`), and `gen-status-cue`
  accepts a `.md` plan file: each `- [ ]/[x] title [SPEC-ID] {symbol: Foo}` line becomes a
  verifiable task. A `/verify` slash command runs the loop — surfacing `[x]`-but-not-verified
  items before "done" is trusted. (throughline's sessions were hand-rolling this.)

### Changed
- **`serve-mcp --auto-refresh` (default on)** — a query for a service whose git HEAD has
  drifted from the indexed commit transparently re-ingests (hash-gated, debounced per
  commit) before answering. Removes the manual staleness→ingest loop (49 ingests / 19
  staleness checks per 3 sessions). A second `serve-mcp` on the same store now refuses with
  actionable guidance instead of DuckDB's raw single-writer lock error.

## [0.1.0] — 2026-06-15

First tagged milestone: a multi-language code-intelligence + evidence engine with an MCP
server, a language-agnostic cross-repo graph, and an evidence-backed verification layer.

### Added
- **Multi-language extraction (~25 languages/tools).** Tree-sitter via CGO (ADR-013) for
  general languages — Python, JS/TS, Java, Ruby, Rust, C#, C, C++, Scala, PHP, Lua — and
  pure-Go parsers for config/IaC/tooling: Terraform/HCL, Kubernetes, Helm, Dockerfile,
  docker-compose, GitHub Actions, Protocol Buffers, GraphQL, CUE, Markdown, SQL, TOML,
  JSON. 93 `SourceKind`s. Interim intra-file call edges for the tree-sitter languages.
  Calibration corpus + harness.
- **Language-agnostic cross-repo blast radius.** `internal/platformlink` builds the
  cross-repo graph from stored multi-language observations instead of a Go-only filesystem
  walk; `pkg/platform.BuildFromStore`; the `platform-callers` CLI defaults to the store
  path (`--repos` is a labelled legacy Go-only mode). Cross-repo edges now resolve for
  every language whose extractor populates call edges.
- **Evidence-backed definition-of-done.** `internal/verify` engine + the `verify_plan` MCP
  tool: a per-task verdict computed from code evidence (the claimed symbol exists at a
  sufficient tier ∧ a test statically reaches it ∧ a required call-edge is wired ∧ no
  contradiction ∧ declared specs are covered), each with cited evidence. The
  `gen-status-cue` CLI renders a deterministic, one-directional `STATUS.cue` from a
  human-owned plan file (`schema/plan` CUE schema). **Static by construction**:
  `runtime_pass` is always `unknown` — a static check never claims a test passes.
- **Use-case adoption docs** under `docs/use-cases/` — a getting-started guide plus six
  how-tos (coding-context, cross-repo blast radius, spec verification, incident
  investigation, change review, fleet health) for wiring sextant into Claude Code, with a
  one-doc-per-use-case maintenance convention.
- **Build-identity / staleness signal.** `sextant version` reports the embedded VCS
  revision, commit time, and dirty flag (Go `-buildvcs`, no ldflags); `serve-mcp` logs it
  at startup, so a long-running server that has outlived commits is self-evident — compare
  against `git rev-parse HEAD`. (O3.)
- Earlier foundation in this release: typed edge-graph `traverse` + commit `history`
  ingestion; `spec_refs` and `brief` tools; git-HEAD auto-staleness; line numbers carried
  through to MCP results; `.sextantignore` repo-specific excludes; the Claude Code
  integration playbook and assessment.

### Fixed
- **`SymbolNode.Package` persists through the store.** It was silently dropped in the
  `SymbolNode → Observation` mapping, degrading `SymbolKey` reconstruction to `"svc::.Name"`
  and breaking precise cross-repo / call-graph matching off stored evidence. Added a
  `package` column with an idempotent migration.
- **The call graph resolves cross-package selector calls.** A callee recorded as
  `"pkg.Name"` (e.g. an external `*_test` package calling the symbol under test) never
  matched the node key `"svc::pkg.Name"`, so `test_impact` / `find_callers` /
  `rename_impact` under-counted in the common Go external-test-package pattern. Fixed with
  a precise `package.Name` index (a receiver-method call is never mistaken for a package
  selector).
- **`outline` honors its repo-relative contract.** `symbolnav.Outline` matched the stored
  absolute `FilePath` against the `file` arg by exact string, so a repo-relative path —
  the documented contract — returned `{"symbols":[]}`. Now resolves a relative query by
  path-segment suffix (absolute still exact). Found via fleet dogfooding (throughline).
  (O1.)

### Changed
- **The `post-commit` hook rebuilds `bin/sextant` on Go changes** before re-indexing, so
  the on-disk binary tracks HEAD. A running `serve-mcp` still needs a restart to load a new
  binary. (O3.)
