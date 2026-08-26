# Scope

Where the edges are right now. Present tense; done goals are deleted, git remembers.
Computed progress: `fathom plans status --service fathom` (43 active plans, 623 gated rows on 2026-08-26).

## Current goals

**G2. The A/B becomes admissible.** Done when: `fathom flywheel` reports ≥20
arm-homogeneous units per arm (MI-5) and a keep/retract verdict is recorded under law 12.
Until then every production savings claim stays inadmissible
(`docs/vision/24-work-frontier.md` §7: ON arm had 0 admissible units on 2026-08-21).

**G3. The continuity-thread bet reaches its sample.** Done when: `threadBetCheck`
reports 20 units and the flywheel's continuity loop states keep or kill
(`docs/vision/21-continuity-threads.md`; 4 of 20 on 2026-08-21).

**G4. Ambient fathom closes.** Done when: `fathom plans status` shows
`docs/plans/ambient-fathom.md` with 0 open and 0 claimed-unproven rows. Open on
2026-08-25 evening: SH-9, SP-10, RG-5 (observatory UI), DO-3, IR-3/4/5 (LLM assess lanes,
undesigned), OP-1 (the A/B, G2), OP-2 (deploy), RG-9 (wiring row).

**G5. The custodian closes.** Done when: `docs/plans/custodian.md` shows 0 open and 0
claimed-unproven. Open on 2026-08-25 evening: CC-9 (`config propose`), CC-11/12/13
(corpus seam, packs, doctor convergence), DP-2/3/4, SD-2/3/4 (SDD panel).

**G6. Zero claimed-unproven rows fleet-wide, then CI fails closed.** Done when:
`fathom plans status` shows 0 claimed-unproven across every active plan (12 on
2026-08-25: impact-observatory-activation 4, observatory 3, custodian 2, ambient-fathom 2,
cpp-toolchain 1) and `fathom verify --strict` exits 0 for every top-level plan. Blocked on
the pending decision `th-c214dd26` (`fathom thread list`).

**G7. The evidence plane's wave 1 ships and its wave-2 gate is read (ADR-026).** Done when:
`fathom verify --service fathom --plan docs/plans/evidence-plane.md --require-all` exits 0
for wave 1 (`fathom intent audit`, the intent ledger, the doctor rows, the clock) and the
clock row records whether K0's demand signal (≥3 annotations across ≥2 repos) was met by
day 30 — either outcome closes the goal; `docs/plans/honesty-fixes.md` ships first.

## Current boundaries

- Languages: Go via `go/parser`; Python, JS/TS, Ruby via tree-sitter on wazero
  (`internal/extractor/tsengine`); Java, C, C++, C#, Rust, Scala, PHP, Bash, Lua via CGO
  tree-sitter; HCL, Kubernetes YAML, CUE, Markdown, `go.mod` natively; tool identity for
  Helm, kustomize, compose, OpenAPI, generated files. Evidence: `docs/architecture.md`
  "Extraction", ADR-021.
- Surfaces: 47 CLI verbs (`fathom help`; ADR-026 adds `intent`, honesty-fixes adds `post-commit-hook`); MCP over stdio and HTTP with 55 registered
  tools and a 17-tool agent surface (`docs/mcp-tools.md`); Go client `pkg/client`;
  the observatory web UI on app-kit (`cmd/fathom/observatory*.go`).
- Store: pure-Go SQLite in WAL mode, the only on-disk backend; one shared fleet store
  served by one always-on loopback daemon (`serve-http`, `127.0.0.1:7575`) under launchd.
  Evidence: ADR-020, ADR-025, `Taskfile.yml` deploy.
- Harnesses: Claude Code (SessionStart, PreToolUse, PostToolUse, Stop, SessionEnd,
  PreCompact hooks) and Codex (AGENTS.md stanza, `fathom prime`). Evidence:
  `docs/adoption/claude-code.md`, `docs/adoption/codex.md`.
- Fleet: the repositories on this workstation, reconciled by `fathom fleet reconcile`
  (17 on 2026-08-13). Evidence: `CHANGELOG.md` 2026-08-13.
- Substrate: `agentic-kit` via `go.work`, or the byte-pinned in-tree mirror. Evidence: ADR-009, ADR-019.
- Platform: macOS only in practice (launchd daemon, TCC, code signing); no CI runner
  exists (`scripts/ci.sh` is the CI helper, nothing invokes it).

## Compatibility surface

A change to any item is breaking: commit `!`, `### Breaking` in `CHANGELOG.md`, a
migration note (`docs/rename-migration.md` is the precedent; new ones go to
`docs/upgrades/<version>-<slug>.md`).

- Governed Store Format v0: additive within v0; a rename, retype, or removal is v1 and a
  `format_version` bump. Evidence: `docs/spec/store-format-v0.md`,
  `internal/store/format_v0_test.go` (`TestStoreFormatV0_RequiredSchema`).
- Expert Module v0 manifest. Evidence: `docs/spec/expert-module-v0.md`, `schema/module/schema.cue`,
  `internal/modspec/schema_test.go`.
- `module_query` wire contract v0 (`claim_id`, `anchor`, `watermark`). Evidence: commit 0df159d,
  `docs/handoff/repo-strategy.md` §3.
- Plan grammar v2: checkbox states, annotation keys, `id:`, dispositions, the
  checkbox-line rule. Evidence: `docs/plans/README.md`, `schema/plan/schema.cue`,
  `internal/verify/markdown.go`.
- `fathom verify` exit codes 0/1/2 and the `--strict`, `--require-all`, `--quiet` flags;
  every wired repo's `lefthook.yml` depends on them. Evidence: `cmd/fathom/verify.go`.
- Hook verbs and their stdin/stdout contracts: `session-hook`, `edit-hook [--pre]`,
  `stop-hook`, `session-end-hook`, `pre-compact-hook`; `fathom init` writes them into
  adopters' `~/.claude/settings.json`. Evidence: `docs/adoption/claude-code.md`.
- The `<!-- fathom:begin -->`…`<!-- fathom:end -->` stanza region in `CLAUDE.md` and
  `AGENTS.md` (fleet reconcile compares content, law 15). Evidence: `CHANGELOG.md` 2026-08-13.
- The 17-tool agent surface allowlist, tool names and schemas, `POST /rpc/agent` and the
  `/rpc/companion` alias. Evidence: `docs/mcp-tools.md`, `internal/mcp/surface.go`.
- The `.fathom/` files an adopter commits: `concerns.cue`, `guard.cue`, `capabilities.cue`,
  `modules.cue`, `layers/`, `rules/`, `observations.md`. Evidence: `schema/{binding,guard,config,semantic,rules}`.
- The `~/.fathom/` files: `config.toml`, `capabilities.cue`, `concerns.cue`,
  `verify-ledger.jsonl`, `ignore.d/<service>`. Evidence: `docs/configuration.md`, `CHANGELOG.md` 2026-08-14.
- Telemetry record shapes carrying `schema_version` and `fathom_version`: call log JSONL,
  beats, session ledger, rollups, `/statusz` JSON; the research bundle
  (`schema/research-bundle`). Evidence: `CHANGELOG.md` IO-11..13.
- `pkg/client` and `pkg/platform` exported Go API. Evidence: `pkg/client/client.go`.
- CLI verbs and their `--json` envelopes (law 10: every capability is a verb first).

Pre-1.0: breaking changes bump minor. The line still gets written.

## Non-goals

- **No domain connectors** (PDF, Confluence, portals). Why: liability-shaped error
  costs and "the unwinnable Glean game" (`docs/design/product-vision.md` §3). Revisit
  when: an external party ships a conforming module on the published spec with zero
  connectors from us.
- **No web-scale code search, IDE language services, or embeddings-first chat.** Why:
  lanes owned by others; the harness supplies chat better and free
  (`docs/vision/01-thesis.md` §2–3). Revisit when: never.
- **No hosted multi-tenant expert backend.** Why: store rewrite, tenancy, SLA, data
  governance on a bus factor of one (`docs/vision/07` R1). Revisit when: an external
  party runs local fathom four weeks and asks in writing for a hosted view, and law 9
  hardening is proven.
- **No first-party chat or incident command center.** Why: competes with the best
  distribution channel (`07` R2). Revisit when: ≥15–20 resolved findings exist and a
  consumer asks.
- **No dispatch loop.** Fathom never watches, schedules, retries, spawns, or kicks off;
  no task tracker, no workflow engine (`docs/vision/24` dispatch boundary, `09`). Revisit when: never.
- **No transformation or migration engine.** Why: the Moderne trap (`01` §3). Revisit when: never.
- **No tool joins a surface without call-log justification; no net tool growth** (law
  4; product-vision standing non-goals). Revisit when: never.
- **No grep-blocking.** Why: friction at the agent's most reflexive action is trust-fatal
  (`docs/design/coding-agent-excellence.md` §5.3). Revisit when: post-injection telemetry
  shows ≥30% of greps in indexed repos were index-answerable, and then only as a hint.
- **CUE is never an authoring surface or an execution engine** (ADR-018, `09`). Revisit when: never.
- **No separate SDD binary or repo.** Why: no consumer without the index (ADR-018). Revisit when: never.
- **No Node toolchain.** Why: the artifact is a Go binary; CSS via the pinned Tailwind
  binary (`.fathom/guard.cue`). Revisit when: never.
