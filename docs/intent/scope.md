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

**G9. The bench is honest, cheap to re-run, and the 2026-09-13 reading is the baseline.**
BP-1..25 proved (L0 index oracle, L1 payload replay and L2 corpus gate in `task validate`;
every cell remembered by its key; targeted runs; H2-3 registered) and the first replicated
home run (run-20260913T002832Z, BC-14 / BP-25) is on the ledger under both H2-1 and H2-3:
FAIL on cost either way — fathom passed 31/33 to grep's 28/33 but returned 1.90× the bytes
(O31), and every headless fathom cell paid a ToolSearch turn the config can remove (O32,
BP-39). Done when the next L4 (BP-44) borrows every grep and none cell by key, re-runs
only the fathom cells, and records payload ≤ 1.0× or why not — after BP-41..43 land and a
targeted run over the losing tasks shows them. Before ADR-032 the single three-arm bench
could only test what grep can also do and measured the harness more than the index
(O23–O26); the public run (PB-19) and any publication stay operator decisions (Q4, F2).

**G10. Every claim fathom makes is on a ledger, against its alternative (ADR-033).** Waves
1–5 proved (41 of 49 rows) and the pilots for M1, M3, M4, M5 and M6 are on
`docs/bench/scenarios-ledger.jsonl` (74 rows) with their predictions beside their verdicts:
M1 zero violations on either arm across 64 cells, M4 refuted as run, M5 confirmed on
overlap without a token win, M6 equal accuracy at a third of grep's cost for grep. Two rows
are honestly unmeasured and re-run next: M2 (no agent wrote a contract-bearing row, SB-53)
and M7 (a setup defect, SB-51). Done when those two rows carry a reading and SB-48
(search with a receipt) has a targeted run beside the lookup tasks grep won — the cost story
the whole matrix points at. M8 and M9 stay registered behind that. **2026-09-13:** every
pilot named above is voided by O41 (G11) — read this paragraph as history, not proof.

**G11. A reading, a hand-off and an answer carry their evidence (ADR-034).** The first
round's 74 rows are voided by an instrument defect (the wired arm never had the tools) and
say so on the ledger; every cell now carries its server status and surface calls, published
readings are plan rows the gate verifies against the ledger, every tool answers under a
budget with a receipt, discoveries reach workers and the next session as fresh-resolved
pointers, and coverage is a code fact by layer. Done when the reruns in `docs/plans/receipts.md`
wave 5 carry readings with `surface_calls > 0` and the discoveries kill criteria are decided.

**G12. The surface leads with the answers grep cannot give, and every answer is falsifiable (ADR-035).** The
four impact verbs and a safe-delete check join the agent surface, obligations names the rule that
governs, a layer rule compiles to a test, receipts carry a resolution tier a stranger can
re-derive, and fathom is measured on a corpus it did not write. Done when `docs/plans/verbs.md`
waves A–D are proven and M10, M11, M12 and M13 carry readings, honest either way.
**2026-09-17:** waves A–D are proven (VB-1..36) and every row carries a reading. M7's freshness
kill clause fired (VB-18) — freshness leaves positioning; M11/M11b (VB-19..21) and M8 (VB-30/31)
held; M10 (VB-12) is on hold until ADR-037's raw-guide fallback is re-measured; M13's
gated-versus-advisory governance pilot (VB-16) and M15's historical-PR review (VB-24, 5/42
strict-new hits) stay open; M12/M12b (VB-27, VB-35) and M14 (VB-29) are a published null or stay
advisory. Governance now bites at the edit for checkable rules (ADR-037, ADR-038); on the lookup
pilot fathom reads level with the best tools at lower cost, not a token-savings win
(`docs/bench/README.md` "Where fathom stands").

**G7. The evidence plane's waves 1–2 ship and its oracle gate is read (ADR-026).** Waves 1
and 2 shipped 2026-08-27: the verb, the ledger, the doctor rows, the clock, the shared
attention line, the capped primer, the guard proposal with provenance, U12/U13, the fleet
column, and K5's pair with K1's measured baseline (2 records, median 78 days). Done when:
the clock row records whether K0's demand signal (≥3 annotations across ≥2 repos) was met
by day 30 — **either outcome closes the goal**, and EP-16/17/18 are the only rows still
waiting on it.

## Current boundaries

- Languages: Go via `go/parser`; Python, JS/TS, Ruby, Kotlin via tree-sitter on wazero
  (`internal/extractor/tsengine`; Kotlin added wave P2/O79 as new-language breadth, no
  CGO incumbent); Java, C, C++, C#, Rust, Scala, PHP, Bash, Lua via CGO tree-sitter; HCL,
  Kubernetes YAML, CUE, Markdown, `go.mod` natively; tool identity for Helm, kustomize,
  compose, OpenAPI, generated files. Evidence: `docs/architecture.md` "Extraction",
  ADR-021.
- Surfaces: 57 CLI verbs (`fathom help`, counted 2026-09-02; ADR-029 adds `harness`);
  MCP over stdio and HTTP with 55 registered tools and a 19-tool agent surface, which
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
- Hook verbs and their stdin/stdout contracts: `session-hook` (with `--event
  instructions-loaded|subagent-start|tool-failure|cwd-changed`),
  `edit-hook [--pre]`, `stop-hook`, `session-end-hook`, `pre-compact-hook`,
  `config audit --hook [--event file-changed]` and `doctor --brief`, each with an
  additive `--harness <id>` (absent ⇒ `claude-code`); `fathom init` writes them into
  adopters' `~/.claude/settings.json` and, under `--harness=gemini-cli`, into the
  `hooks` key of the repo's `.gemini/settings.json`. It writes NO Codex hook file:
  codex-cli 0.153.1 loads hooks only from the user-global `$CODEX_HOME/hooks.json`,
  which fathom does not edit unasked — `fathom harness hooks --harness codex` prints
  the entries instead. Evidence: `docs/adoption/claude-code.md`, ADR-029.
- `fathom harness list|capabilities|hooks|ladder|matrix` (`--json` where offered) and
  the exported descriptor shape, including each descriptor's event catalogue
  (`schema/harness`). Evidence: ADR-029.
- The `<!-- fathom:intent-playbook -->` region of a root `INTENT.md`, written by
  `fathom intent render --playbook` and held to existence by `config audit`'s
  `pointers_resolve`. Evidence: `cmd/fathom/intent_playbook.go`.
- Plan grammar v2's lexical oracle keys on plan rows (`oracle: lexical`, `path`,
  `pattern`). Evidence: ADR-030, `schema/plan/schema.cue`.
- The `<!-- fathom:begin -->`…`<!-- fathom:end -->` stanza region in `CLAUDE.md` and
  `AGENTS.md` (fleet reconcile compares content, law 15). Evidence: `CHANGELOG.md` 2026-08-13.
- The 19-tool agent surface allowlist, tool names and schemas, `POST /rpc/agent` and the
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
