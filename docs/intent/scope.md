# Scope

Where the edges are right now. Present tense; done goals are deleted, git remembers.
Computed progress: `fathom plans status --service fathom` (51 active plans, 779 gated rows on 2026-09-02).

## Current goals

A goal is a pre-registered bet — `{bet, instrument, threshold, review}` on its `Done when:`,
the instrument a `fathom` verb, a Taskfile target or a repo script — or `fathom intent
doctrine` refuses it at load and names what it lacks (DP-4, ADR-026 amendment 2026-09-25).

**G2. The A/B becomes admissible.** Done when: `fathom flywheel` reports ≥20
arm-homogeneous units per arm (MI-5) and a keep/retract verdict is recorded under law 12.
Every production savings claim stays inadmissible until then (0 admissible ON-arm units
on 2026-08-21, `docs/vision/24-work-frontier.md` §7).
{bet: arm-bench, instrument: "fathom flywheel", threshold: "≥20 arm-homogeneous units per arm (MI-5), then a keep/retract verdict under law 12", review: 2026-10-25}

**G3. The continuity-thread bet reaches its sample.** Done when: `threadBetCheck`
reports 20 units and the flywheel's continuity loop states keep or kill
(`docs/vision/21-continuity-threads.md`; 4 of 20 on 2026-08-21).
{bet: thread-bet, instrument: "fathom flywheel", threshold: "20 fresh-with-brief units (threadBetCheck), then the continuity loop states keep or kill", review: 2026-10-25}

**G4. Ambient fathom closes.** Done when: `fathom plans status` shows
`docs/plans/ambient-fathom.md` with 0 open and 0 claimed-unproven rows. Open: SH-9,
SP-10, RG-5, RG-9 (observatory UI/wiring), OP-1 (the A/B, G2), OP-2 (deploy). DO-3 and
IR-3/4/5 (the assess lanes) are designed (`docs/design/assess-lanes.md`, ADR-046) and
proven 2026-09-25.
{bet: ambient-closes, instrument: "fathom plans status", threshold: "docs/plans/ambient-fathom.md at 0 open and 0 claimed-unproven rows", review: 2026-10-25}

**G5. The custodian closes.** Done when: `docs/plans/custodian.md` shows 0 open and 0
claimed-unproven. Open: SD-2/3/4 (SDD panel); DP-2 is refused (superseded by EP-9).
{bet: custodian-closes, instrument: "fathom plans status", threshold: "docs/plans/custodian.md at 0 open and 0 claimed-unproven rows", review: 2026-10-25}

**G9. The bench is honest, cheap to re-run, and the 2026-09-13 reading is the baseline.**
BP-1..25 proved (L0 index oracle, L1 payload replay, L2 corpus gate, per-key cell memoization,
H2-3 registered). The first replicated home run (run-20260913T002832Z) is FAIL on cost
either way: fathom passed 31/33 to grep's 28/33 but returned 1.90× the bytes (O31), and
every headless cell paid a removable ToolSearch turn (O32). Done when: the next L4 (BP-44)
borrows every grep/none cell by key, re-runs only fathom's, and records payload ≤ 1.0× or
why not, after BP-41..43 land. Publication stays an operator decision (Q4, F2).
{bet: bench-payload, instrument: "fathom plans status", threshold: "BP-44 proven in docs/plans/bench-pyramid.md with fathom's payload ≤ 1.0× grep's, or the reading names why not", review: 2026-10-25}

**G13. The Observatory console closes.** Done when: `fathom plans status` shows
`docs/plans/observatory-console.md` with 0 open non-op rows, UX-M1 and UX-M2 are measured
PASS, and every contribution row names its app-kit delivery (vision 35, ADR-045).
{bet: console-closes, instrument: "fathom plans status", threshold: "docs/plans/observatory-console.md at 0 open non-op rows; UX-M1 and UX-M2 read PASS", review: 2026-10-25}

**G11. A reading, a hand-off and an answer carry their evidence (ADR-034).** The first
round's 74 rows were voided by an instrument defect (the wired arm never had the tools; O41
voided G10, ADR-033's ledger goal, on 2026-09-13 and it folds in here);
every cell now carries its server status and surface calls, published readings are plan
rows the gate verifies against the ledger, every tool answers under a budget with a
receipt, discoveries reach workers and the next session as fresh-resolved pointers, and
coverage is a code fact by layer. Done when: the reruns in `docs/plans/receipts.md` wave 5
carry readings with `surface_calls > 0` and the discoveries kill criteria are decided.
{bet: receipts-rerun, instrument: "fathom plans status", threshold: "docs/plans/receipts.md wave 5 proven with surface_calls > 0 on every reading; the discoveries kill criteria decided", review: 2026-10-25}

**G12. The surface leads with the answers grep cannot give, and every answer is falsifiable
(ADR-035).** The four impact verbs and a safe-delete check join the agent surface,
obligations names the governing rule, a layer rule compiles to a test, receipts carry a
re-derivable resolution tier, and fathom is measured on a corpus it did not write.
Done when: `docs/plans/verbs.md` waves A–D are proven and M10–M13 carry readings, honest
either way. **2026-09-17:** waves A–D proven (VB-1..36); M7's freshness kill fired (VB-18 —
freshness leaves positioning); M11/M11b and M8 held; M10 is on hold pending ADR-037's
raw-guide re-measure; M13's governance pilot and M15's historical-PR review stay open;
M12/M12b and M14 are a published null or stay advisory. Governance now bites at the edit
for checkable rules (ADR-037, ADR-038); fathom reads level with the best tools at lower
cost, not a token-savings win (`docs/bench/README.md` "Where fathom stands").
{bet: verbs-readings, instrument: "fathom plans status", threshold: "docs/plans/verbs.md M10 and M13 carry a reading (M11 and M12 are read), honest either way", review: 2026-10-25}

**G7. The evidence plane's waves 1–2 ship and its oracle gate is read (ADR-026).** Waves 1
and 2 shipped 2026-08-27: the verb, the ledger, the doctor rows, the clock, the shared
attention line, the capped primer, the guard proposal with provenance, U12/U13, the fleet
column, and K5's pair with K1's measured baseline (2 records, median 78 days). Done when:
the clock row records whether K0's demand signal (≥3 annotations across ≥2 repos) was met
by day 30 — either outcome closes the goal, and EP-16/17/18 are the only rows still waiting
on it.
{bet: evidence-plane-k0, instrument: "fathom intent audit", threshold: "the clock row records K0's demand signal (≥3 annotations across ≥2 repos) met or missed at day 30", review: 2026-09-27}

## Current boundaries

- Languages: Go via `go/parser`; Python, JS/TS, Ruby, Kotlin via tree-sitter on wazero
  (`internal/extractor/tsengine`); Java, C, C++, C#, Rust, Scala, PHP, Bash, Lua via CGO
  tree-sitter; HCL, Kubernetes YAML, CUE, Markdown, `go.mod` natively; tool identity for
  Helm, kustomize, compose, OpenAPI, generated files. Evidence: `docs/architecture.md`
  "Extraction", ADR-021.
- Surfaces: 57 CLI verbs (`fathom help`, counted 2026-09-02; ADR-029 adds `harness`);
  MCP over stdio and HTTP with 55 registered tools and a 19-tool agent surface, which
  now serve annotations and a generated `outputSchema` (`docs/mcp-tools.md`); Go client
  `pkg/client`; the Observatory console on app-kit (`cmd/fathom/console_*.go` over
  `internal/console` projections; `observatory*.go` renderers), plus
  opt-in loopback `theme-guide` over synthetic models (docs/app-kit.md). The guide is
  experimental, source-preserving, separate from daemon startup and not runtime settings.
- Store: pure-Go SQLite in WAL mode, the only on-disk backend; one shared fleet store
  served by one always-on loopback daemon (`serve-http`, `127.0.0.1:7575`) under launchd.
  Evidence: ADR-020, ADR-025, `Taskfile.yml` deploy.
- Harnesses: Claude Code (SessionStart, PreToolUse, PostToolUse, Stop, SessionEnd,
  PreCompact hooks) and Codex (AGENTS.md stanza, `fathom prime`, MCP) both verified live.
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
- Hook verbs and their stdin/stdout contracts: `session-hook` (`--event
  instructions-loaded|subagent-start|tool-failure|cwd-changed`), `edit-hook [--pre]`,
  `stop-hook`, `session-end-hook`, `pre-compact-hook`, `config audit --hook [--event
  file-changed]`, `doctor --brief`, each with an additive `--harness <id>` (absent ⇒
  `claude-code`). `fathom init` writes them into `~/.claude/settings.json` and, under
  `--harness=gemini-cli`, into `.gemini/settings.json`'s `hooks` key; it writes no Codex
  hook file (codex-cli only loads a user-global one — `fathom harness hooks --harness
  codex` prints the entries instead). Evidence: `docs/adoption/claude-code.md`, ADR-029.
- `fathom harness list|capabilities|hooks|ladder|matrix` (`--json` where offered) and
  the exported descriptor shape, including each descriptor's event catalogue
  (`schema/harness`). Evidence: ADR-029.
- The `<!-- fathom:intent-playbook -->` region of a root `INTENT.md`, written by
  `fathom intent render --playbook` and held to existence by `config audit`'s
  `pointers_resolve`. Evidence: `cmd/fathom/intent_playbook.go`.
- The conservation contracts (ADR-044): `fathom conserve --json`'s `fathom.conserve/v1` shape
  (schema string, plane and kind names, field names, plane-kind-key sort order), the plan keys
  `gone:`, `retires:`, `surface_change:`, `baseline:` and `tests:`, and `.fathom/config.toml`'s
  `[[surface.pattern]]` block. Evidence: `docs/design/refactor-conservation.md`,
  `cmd/fathom/conserve.go` (`TestCmdConserve_JSONSchemaV1`).
- The certification contracts (ADR-043): the `// certifies: <kind> <id> [(<strength>)]` marker
  grammar, the `certified_by:` plan key, and `fathom certify gaps --json`'s
  `fathom.certify.gaps/v1` shape (schema string, field names, kind-then-id sort order) that
  Review Kit's `fathom-certify` adapter parses. Evidence: `docs/design/test-certifications.md`,
  `cmd/fathom/certify_gaps.go` (`TestCmdCertifyGaps_RemedyLinesAndJSON`).
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
