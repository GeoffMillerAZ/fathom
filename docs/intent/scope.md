# Scope

Where the edges are right now. Present tense; done goals are deleted, git remembers.
Computed progress: `fathom plans status --service fathom` (51 active plans, 779 gated rows on 2026-09-02).

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
`fathom plans status` shows 0 claimed-unproven across every active plan (0 on
2026-09-02, with 10 rows `measured-stale` behind a daemon predating HEAD -- a coverage
gap, not a failure) and `fathom verify --strict` exits 0 for every top-level plan (the
sweep over all 51 reads 672 claimed / 662 verified / 0 refuted on 2026-09-02; the 10
unverified are claimed rows with no contract the gate can refute, each naming why).
Path: store provenance on verdicts and the lexical oracle shipped in 0.8.0 (ADR-030,
`docs/plans/proof-credibility.md` PC-1..5, PC-17..19); PC-20 is what is left.

**G9. A stranger can re-run the bench.** Done when: `docs/plans/bench-public.md` PB-1..18
are proven (truth at HEAD, H2-2 registered, the derived public corpus reproduces
bit-for-bit from a clean clone -- all proven in 0.8.0) and the first replicated home run
(BC-14) has a ledger row, verdict honest either way (ADR-031). BC-14 has NOT been run:
the ledger holds one backfilled n=1 row, indeterminate under the 3-trial floor. The
public run (PB-19) and any publication stay operator decisions (Q4, F2).

**G7. The evidence plane's waves 1–2 ship and its oracle gate is read (ADR-026).** Waves 1
and 2 shipped 2026-08-27: the verb, the ledger, the doctor rows, the clock, the shared
attention line, the capped primer, the guard proposal with provenance, U12/U13, the fleet
column, and K5's pair with K1's measured baseline (2 records, median 78 days). Done when:
the clock row records whether K0's demand signal (≥3 annotations across ≥2 repos) was met
by day 30 — **either outcome closes the goal**, and EP-16/17/18 are the only rows still
waiting on it.

## Current boundaries

- Languages: Go via `go/parser`; Python, JS/TS, Ruby via tree-sitter on wazero
  (`internal/extractor/tsengine`); Java, C, C++, C#, Rust, Scala, PHP, Bash, Lua via CGO
  tree-sitter; HCL, Kubernetes YAML, CUE, Markdown, `go.mod` natively; tool identity for
  Helm, kustomize, compose, OpenAPI, generated files. Evidence: `docs/architecture.md`
  "Extraction", ADR-021.
- Surfaces: 57 CLI verbs (`fathom help`, counted 2026-09-02; ADR-029 adds `harness`);
  MCP over stdio and HTTP with 55 registered tools and a 17-tool agent surface, which
  now serve annotations and a generated `outputSchema` (`docs/mcp-tools.md`); Go client
  `pkg/client`; the observatory web UI on app-kit (`cmd/fathom/observatory*.go`).
- Store: pure-Go SQLite in WAL mode, the only on-disk backend; one shared fleet store
  served by one always-on loopback daemon (`serve-http`, `127.0.0.1:7575`) under launchd.
  Evidence: ADR-020, ADR-025, `Taskfile.yml` deploy.
- Harnesses: Claude Code (SessionStart, PreToolUse, PostToolUse, Stop, SessionEnd,
  PreCompact hooks; verified on this machine) and Codex (AGENTS.md stanza, `fathom prime`,
  MCP). Under ADR-029 the harness is data in `internal/harness`: Codex and Gemini CLI
  descriptors carry codecs and ship `unverified` until a real session's beats verify
  them; Cursor and Copilot are data rows; tiers C/D are documented rows. Evidence:
  `docs/adoption/claude-code.md`, `docs/adoption/codex.md`, `docs/vision/25-harness-port.md`.
- Fleet: the repositories on this workstation, reconciled by `fathom fleet reconcile`
  (20 on 2026-09-02, 19 drifted on the dry run; nothing applied). Evidence:
  `CHANGELOG.md` 2026-08-13.
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
  `stop-hook`, `session-end-hook`, `pre-compact-hook`, each with an additive
  `--harness <id>` (absent ⇒ `claude-code`); `fathom init` writes them into adopters'
  `~/.claude/settings.json`, under `--harness=codex` into the repo's `.codex/hooks.json`, and
  under `--harness=gemini-cli` into the `hooks` key of the repo's `.gemini/settings.json`.
  Evidence: `docs/adoption/claude-code.md`, ADR-029.
- `fathom harness list|capabilities --json` and the exported descriptor shape
  (`schema/harness`). Evidence: ADR-029.
- Plan grammar v2's lexical oracle keys on plan rows (`oracle: lexical`, `path`,
  `pattern`). Evidence: ADR-030, `schema/plan/schema.cue`.
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
