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

**G14. Fathom is a responsible tenant (ADR-047, vision 36).** The owner's 2026-09-26 priority:
the daemon keeps to a declared `phys_footprint` budget, refuses what cannot fit with a named
reason, never pays repository-sized costs on a request, and cannot relaunch or leak its way
back into the machine. Done when: `docs/plans/responsible-tenant.md` has 0 open non-op rows
and RT-M1 (seven days within budget, no exit rung) and RT-M3 (the stress harness on the
wave-B and post-wave-D binaries) read PASS.
{bet: responsible-tenant, instrument: "fathom plans status", threshold: "docs/plans/responsible-tenant.md at 0 open non-op rows; RT-M1 and RT-M3 read PASS", review: 2026-10-26}

**G15. Language breadth and depth, measured (ADR-048, vision 37).** The owner's 2026-09-27
directive: excellent, measured support for the languages and tool formats people use, most
common first. A language counts at a level only when its committed reading proves it.
Done when: the foundation rows of `docs/plans/language-breadth.md` are proven and LB-M6
reads milestone M1 (20 languages at L1 excellent, 12 at L2, 8 at L3, 6 at L4; 15 tool
formats at target) with LB-M3 (no footprint rise) PASS.
{bet: language-breadth, instrument: "fathom plans status", threshold: "docs/plans/language-breadth.md LB-F rows proven; LB-M6 reads milestone M1; LB-M3 PASS", review: 2026-11-27}
Named 2026-09-27 (ADR-049, vision 38 §9.3): after M1 the ladder continues to vision 37 §5's M2
(45 languages at L1 with 35 excellent, 25 at L2, 14 at L3, 10 at L4; 30 tool formats at
target; LB-M6-M2) and M3 (60+ at L1, 35 at L2, 20 at L3, 12 at L4; 40+ tool formats;
LB-M6-M3). M2 and M3 are the next bets once M1 is read; this goal closes on M1. Today no
language has a committed reading and only Go (and partly SQL) fill `Signature`, so every
current level is an unmeasured estimate until LB-F3 and LB-M1 land.

**G16. Reach: fathom installs in one step and serves the clients people use (ADR-049,
vision 38).** The owner's 2026-09-27 wave 1. Done when: `docs/plans/reach.md` and the wave-1
rows of `docs/plans/governance-learning.md` are proven and RH-M1 (a stranger reaches a first
proven answer in at most 5 minutes, timed by `stranger-check`), RH-M2 (the plugin installs in
Claude Code and one other client), RH-M3 (default agent surface at most 8 tools), GL-M1
(rules read from at least 6 client formats) and GL-M4 (hooks live in at least 3 clients) read
PASS.
{bet: reach, instrument: "fathom plans status", threshold: "RH-M1 at most 5 minutes install to first proven answer; RH-M2 plugin installs in 2 clients; RH-M3 default surface at most 8 tools; GL-M1 at least 6 rules formats; GL-M4 hooks live in at least 3 clients", review: 2026-11-27}

**G17. The system graph: proof crosses service boundaries (ADR-050).** Done when: the
wave-2 and wave-3 rows of `docs/plans/system-graph.md` are proven and SG-M1 (route, rpc, gql,
topic, table and env links on the fixture fleet at precision at least 0.95 and recall at least
0.85 for the spec and typed tiers), SG-M2 (`contract_drift` catches at least 90% of seeded
breaking changes) and SG-M3 (`system_impact` answered on the home fleet with every link's
tier named) read PASS.
{bet: system-graph, instrument: "fathom plans status", threshold: "SG-M1 precision at least 0.95 and recall at least 0.85 per kind for spec+typed tiers; SG-M2 drift catches at least 90% of seeded breaks; SG-M3 home-fleet reading", review: 2027-01-15}

**G18. Dependencies: upgrade impact from what is already on disk (ADR-050).** Done when: the
dependency rows of `docs/plans/system-graph.md` are proven and SG-M4 (upgrade impact for Go,
npm, PyPI, Maven and Cargo at precision at least 0.9 on the seeded-upgrade corpus) and SG-M5
(the dependency API index is shared fleet-wide, one index per package version, within the
memory budget, with no network access) read PASS.
{bet: dependencies, instrument: "fathom plans status", threshold: "SG-M4 upgrade-impact precision at least 0.9 in 5 ecosystems; SG-M5 one shared index per package version, footprint within budget, zero dials", review: 2027-01-31}

**G19. Spec interop and the certified proof tier (ADR-049).** Done when:
`docs/plans/spec-interop.md` is proven through wave 4 and SI-M1 (5 spec dialects verify in
place), SI-M2 (the converge report on a real Spec Kit or OpenSpec project), SI-M3 (a plan
under `Proof: certified` verifies only on live certificates) and SI-M4 (Jev anchor inference
passes calibration, or is dropped with the reading) read PASS or record the drop.
{bet: spec-interop, instrument: "fathom plans status", threshold: "SI-M1 5 dialects verify in place; SI-M2 converge report read; SI-M3 certified tier live; SI-M4 calibration at least 9 in 10 or the drop recorded", review: 2026-12-15}

**G20. Knowledge retrieval: doc questions answered at grep's rate or better (ADR-049).**
Done when: `docs/plans/knowledge-retrieval.md` wave 2 is proven and KR-M1 (the doc-question
bench slice passes at least grep's rate at no more than 1.0x grep's bytes) and KR-M2 (doc drift
reported by `staleness_report` on seeded symbol changes, recall at least 0.9) read PASS.
{bet: knowledge-retrieval, instrument: "fathom plans status", threshold: "KR-M1 pass rate at least grep's at no more than 1.0x bytes; KR-M2 doc-drift recall at least 0.9", review: 2026-12-15}

**G21. Governance that learns and reaches reviewers (ADR-049).** Done when:
`docs/plans/governance-learning.md` is proven and GL-M2 (promotions and retirements under
the published thresholds, fewer than 1 in 10 promotions refuted within 30 days), GL-M3
(`as_of` answers across claims, memories and learned-rule states) and GL-M5 (SARIF or the
check payload consumed by one external reviewer) read PASS.
{bet: governance-learning, instrument: "fathom plans status", threshold: "GL-M2 fewer than 1 in 10 promotions refuted in 30 days; GL-M3 as_of reading; GL-M5 one external reviewer consumes the output", review: 2027-01-15}

**G22. Progress and economics, computed (ADR-049).** Done when:
`docs/plans/progress-and-economics.md` is proven and PE-M1 (every number on the progress
and economics pages traces to an instrument), PE-M2 (the proof history ledger reconstructs
burn-up and proof regressions for 30 days), PE-M3 (tokens per proven row joined for Claude
Code and Codex sessions) and PE-M4 (the pages measured like G13's UX-M1) read PASS, and PE-M5 (the ledger-pollution
reading: rows under unregistered scratch roots counted and every figure recomputed without them) is published. The plan
gained cost per proven weight point (PE-22) and registered-roots ledger hygiene (PE-23 to PE-29) on 2026-09-29
(ADR-056 decision 10, observation O121).
{bet: progress-economics, instrument: "fathom plans status", threshold: "PE-M1 every number traced; PE-M2 30-day ledger reconstruction; PE-M3 cost per proven row for 2 harnesses; PE-M4 UX reading PASS; PE-M5 ledger-pollution reading published", review: 2026-12-31}

**G23. Test intelligence: the tests that matter, ranked and selected (ADR-051).** Done when:
`docs/plans/test-intelligence.md` rows for T1 to T5 are proven and TQ-M1 (risk-weighted
coverage ranked on the home fleet), TQ-M2 (certifying-first selection re-proves every touched
certificate while running at most 40% of the suite on a replayed month of diffs) and TQ-M3
(coverage ingested for at least 5 languages) read PASS.
{bet: test-intelligence, instrument: "fathom plans status", threshold: "TQ-M1 home-fleet ranking read; TQ-M2 every touched certificate re-proved at no more than 40% of the suite; TQ-M3 coverage for at least 5 languages", review: 2027-01-15}

**G24. Work intelligence: effort, friction and cost joined to proof (ADR-051).** Done when:
`docs/plans/work-intelligence.md` is proven and WI-M1 (U1 to U6 on the console, every number
from an instrument), WI-M2 (U2's interest rate computed on at least 3 repositories with at
least 30 proven rows each) and WI-M4 (U6's preflight A/B read, keep or drop) read PASS or
record the drop.
{bet: work-intelligence, instrument: "fathom plans status", threshold: "WI-M1 U1-U6 traced to instruments; WI-M2 interest rate on at least 3 repos with at least 30 proven rows each; WI-M4 preflight A/B keep-or-drop recorded", review: 2027-01-31}

**G25. Change and compliance receipts (ADR-051).** Done when:
`docs/plans/change-and-compliance.md` is proven and CX-M1 (the risk receipt on every PR in at
least 2 repositories with SARIF or check output), CX-M2 (an evidence pack re-verifies
byte-identically), CX-M3 (one real migration tracked to proven completion), SG-M6 (the
catalog covers every fleet service) and CX-M4 (the drift radar live) read PASS.
{bet: change-compliance, instrument: "fathom plans status", threshold: "CX-M1 receipts on every PR in at least 2 repos; CX-M2 byte-identical re-verify; CX-M3 one migration proven complete; SG-M6 catalog for every fleet service; CX-M4 radar live", review: 2027-01-31}

**G26. The feature map is measured against the owner's own scorecard (ADR-055).** Done when:
`docs/plans/feature-map.md`'s FM-M1 (evidence columns match the hand-built 86-feature
scorecard on at least 90% of cells) reads PASS. FM-M2 (verdict-proposal agreement at least 0.8 at
n at least 3) is deferred with FM-10 and FM-11 by the owner's decision of 2026-09-29: verdicts stay
human, and model proposals return only after J0 passes and a calibration against the 86 labeled
verdicts reaches the bar (F51).
{bet: feature-map, instrument: "fathom plans status", threshold: "FM-M1 at least 90% cell match against the 86-feature scorecard; verdicts stay human, FM-M2 deferred until J0 passes", review: 2027-01-31}

**G27. Level 10 through fathom: the test-run hook is measured against control and raw level 10 (ADR-056).**
Done when: `docs/plans/test-run-hook.md` TR-M1 reads PASS (arm B, the hook at tiers 0 and 1 with provider none,
uses at least 20 percent fewer tokens-to-green than control, the paired CI excludes 0, correctness does not drop)
or reads FAIL with the reading published and the hook left off, and TR-M2 (raw level 10 read against B) is
published either way.
{bet: test-run-hook, instrument: "fathom plans status", threshold: "TR-M1 arm B at least 20 percent fewer tokens-to-green than control with the paired CI excluding 0 and no correctness drop, or the FAIL reading published with the hook left off; TR-M2 raw level 10 read either way", review: 2027-01-31}

**G28. Does a typed judgment model read code? Decided before any site reads code with one (ADR-056).**
Done when: `docs/plans/judged-predicates.md` JP-M1 (the J0 kill screen) reads PASS or KILL. A KILL drops F34, F36
and F37; a PASS is followed by the 40-handlers cell JP-M3, read against arm A (arm C at least 40 percent fewer
tokens at recall and precision within 0.05, CI excluding 0). The showcase property is never a security property.
{bet: judged-predicates, instrument: "fathom plans status", threshold: "JP-M1 J0 screen reads PASS or KILL before any code-reading site is built; if PASS, JP-M3 arm C at least 40 percent fewer tokens than control at recall and precision within 0.05 with the CI excluding 0", review: 2027-02-15}

**G29. Judgments are calibrated, not assumed (ADR-056).** Done when: `docs/plans/judgment-foundations.md`
JF-1 to JF-12 and JF-23 are proven (the renamed rule-outcome events, per-site opt-in, `route` off by default, the
deterministic certify ladder, calibration readings, bands read from the latest reading), JF-M1 (a first calibration reading) exists, and
`docs/plans/judged-telemetry.md` JT-M1 reads PASS (`failure_kind` judged versus settled or human agreement at
least 0.8 on at least 100 adjudicated items across at least 3 sittings) or the template is dropped.
{bet: judged-calibration, instrument: "fathom plans status", threshold: "JF-1 to JF-12 and JF-23 proven; JF-M1 first calibration reading; JT-M1 judged versus human agreement at least 0.8 on at least 100 adjudicated items across at least 3 sittings, or the template dropped", review: 2027-03-15}

**G30. Exploration maps cut sub-agent tokens without losing accuracy, or are not built (ADR-056).**
Done when: `docs/plans/exploration-maps.md` XM-M1 records BUILD or KILL from the 14-day re-derivation reading, and
if BUILD, XM-M2 (the A/B on identical briefs) reads at least 25 percent fewer sub-agent tokens, no accuracy loss
and under 5 percent stale entries.
{bet: exploration-maps, instrument: "fathom plans status", threshold: "XM-M1 records BUILD or KILL; if BUILD, XM-M2 at least 25 percent fewer sub-agent tokens with no accuracy loss and under 5 percent stale entries", review: 2027-01-31}

**G31. Context survives compaction, measured, or the deliberate silence stands (ADR-056).** Done when:
`docs/plans/carry-across.md` CA-V1 verifies the hook, CA-M1 (Reading 0) records BUILD or STOP, and if BUILD,
CA-M2 reads re-reads of the pre-compaction set down at least 30 percent with the CI excluding 0 and tokens-to-proof
not worse.
{bet: carry-across, instrument: "fathom plans status", threshold: "CA-V1 verified; CA-M1 records BUILD or STOP; if BUILD, CA-M2 re-reads of the pre-compaction set down at least 30 percent with the CI excluding 0 and tokens-to-proof not worse", review: 2027-01-15}

**G32. Work signals: stalls detected without a model, weights estimated with a published error (ADR-056).**
Done when: `docs/plans/work-signals.md` WS-M1 (candidate stall episodes checked against abandoned sessions and later
green runs), WS-M2 (the nudge A/B, keep or drop), WS-M3 (the estimated-weight error published) and WS-M4 (the
preflight model against a fitted baseline, keep or drop) are read.
{bet: work-signals, instrument: "fathom plans status", threshold: "WS-M1 episodes checked against abandoned sessions and green runs; WS-M2 nudge keep or drop recorded; WS-M3 estimated-weight error published; WS-M4 preflight model keep or drop recorded", review: 2027-02-28}

**G33. A lost pin is caught in the turn that lost it, or the line stays a record (ADR-057).**
Done when: `docs/plans/served-not-kept.md` PR-M0 (the $0 replay over the guards-pilot 6 and 8 diffs) reads PASS or
FAIL (a FAIL means the check is fixed before PR-3 is built, K1), PR-M1 (30 days of record mode on owner traffic,
every fire adjudicated by the owner) reads PASS or FAIL, and PR-M2 (the bench, about $76, the owner's approval
first) reads PASS (with the pin line on, at most half the line-off oracle violations on the pin tasks, the
interval excluding 0, tokens per clean success and tool calls at most 1.10 of the line-off arm) or FAIL with
the line left record-only. PR-8 (block a silent loss at pre-push, accepted 2026-09-29, built by PR-9) is not part of this bet.
{bet: served-not-kept, instrument: "fathom plans status", threshold: "PR-M2 part B: pin line on at most half the line-off oracle violations on the pin tasks with the interval excluding 0 and tokens per clean success and tool calls at most 1.10 of line-off (K5), else the line stays record-only; PR-M0 and PR-M1 read either way", review: 2027-01-31}

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

## Compatibility surface

A change to any item is breaking: commit `!`, `### Breaking` in `CHANGELOG.md`, a
migration note (`docs/rename-migration.md` is the precedent; new ones go to
`docs/upgrades/<version>-<slug>.md`).

- Governed Store Format v0: additive within v0; a rename, retype, or removal is v1 and a
  `format_version` bump. Evidence: `docs/spec/store-format-v0.md`,
  `internal/store/format_v0_test.go` (`TestStoreFormatV0_RequiredSchema`).
- Expert Module v0 manifest. Evidence: `docs/spec/expert-module-v0.md`, `schema/module/schema.cue`,
  `internal/modspec/schema_test.go`.
- Plan grammar v2: checkbox states, annotation keys, `id:`, dispositions, the
  checkbox-line rule. Evidence: `docs/plans/README.md`, `schema/plan/schema.cue`,
  `internal/verify/markdown.go`.
- `fathom verify` exit codes 0/1/2 and the `--strict`, `--require-all`, `--quiet` flags;
  every wired repo's `lefthook.yml` depends on them. Evidence: `cmd/fathom/verify.go`.
- Hook verbs and their stdin/stdout contracts: `session-hook` (`--event
  instructions-loaded|subagent-start|tool-failure|cwd-changed|test-run|failed-test-run`),
  `edit-hook [--pre]`,
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
- The spec-interop plan keys (ADR-049): `weight:` (a positive integer, default 1) and
  `accept:` (quoted acceptance text, never gating by itself). Evidence:
  `docs/design/spec-interop.md` §4, `schema/plan/schema.cue`.
- The `<!-- fathom:begin -->`…`<!-- fathom:end -->` stanza region in `AGENTS.md`, and the
  fathom-marked `@AGENTS.md` import region in a root `CLAUDE.md`, `.claude/CLAUDE.md` or
  `CLAUDE.local.md` that fathom did not create (fleet reconcile compares content, law 15).
  Evidence: ADR-059.
- The default agent surface (8 tools, named in ADR-053: `find_symbol`, `search`,
  `read_source`, `symbol_context`, `obligations`, `verify_plan`, `blast_radius`,
  `review_diff`) at `POST /rpc/agent`; the `extended` opt-in surface (21 tools) at
  `POST /rpc/extended`; tool names and schemas; the `/rpc/companion` alias (→ `extended`).
  Evidence: `docs/mcp-tools.md`, `internal/mcp/surface.go`.
- The `.fathom/` files an adopter commits: `concerns.cue`, `guard.cue`, `capabilities.cue`,
  `modules.cue`, `layers/`, `rules/`, `observations.md`. Evidence: `schema/{binding,guard,config,semantic,rules}`.
- The `~/.fathom/` files: `config.toml`, `capabilities.cue`, `concerns.cue`,
  `verify-ledger.jsonl`, `ignore.d/<service>`. Evidence: `docs/configuration.md`, `CHANGELOG.md` 2026-08-14.
- Telemetry record shapes carrying `schema_version` and `fathom_version`: call log JSONL,
  beats, session ledger, rollups, `/statusz` JSON; the research bundle
  (`schema/research-bundle`). Evidence: `CHANGELOG.md` IO-11..13.
- The daemon's admission wire format (ADR-047 §3.4): HTTP 503 with `Retry-After`,
  `X-Fathom-Busy-Scope` (`class` | `daemon`) and JSON-RPC `-32050` (`data: reason, class,
  level, retry_after_ms, in_flight`), the named `abandoned` outcome, `budget` (`partial`,
  `elided`, `bytes_read`, `reason`), and the managed daemon's 409 to a test caller.
  Evidence: `internal/mcp/admission.go` (`TestAdmissionReturnsNamed503WithRetryAfter`),
  `cmd/fathom/admission_http.go` (`TestTestCallerRefusedByManagedDaemon`).
- `pkg/client` and `pkg/platform` exported Go API. Evidence: `pkg/client/client.go`.
- CLI verbs and their `--json` envelopes (law 10: every capability is a verb first).

Pre-1.0: breaking changes bump minor. The line still gets written.

## Non-goals

- **No domain connectors** (PDF, Confluence, portals). Why: liability-shaped error
  costs and "the unwinnable Glean game" (`docs/design/product-vision.md` §3). Revisit
  when: an external party ships a conforming module on the published spec with zero
  connectors from us.
  - Note (2026-09-29, ADR-058): incident maps respect this non-goal. Their outward reach is a rendered link and a
    typed parameter only (a deployed SHA, a tag window); fathom never fetches, polls or stores an ops system's data,
    and the attach and connector forms are refused (`docs/design/incident-maps.md` section 6).
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
  - Note (2026-09-29, ADR-058): incident maps respect this non-goal. An incident map is a committed declaration served
    read-only; drills, drill receipts, a timeline, incident status and an incident-tool export are refused
    (`docs/plans/incident-maps.md` IM-X4), and fathom never mitigates.
- **No dispatch loop.** Fathom never watches, schedules, retries, spawns, or kicks off;
  no task tracker, no workflow engine (`docs/vision/24` dispatch boundary, `09`). Progress
  views (weighted tables, a derived kanban, burn-up) are not a tracker: they read verify's
  verdicts and write nothing; no status is set by a drag, a click or an assignment (ADR-049
  decision 4). Revisit when: never.
- **No productivity score for people.** Fathom measures work, code and agents, never ranks
  people: every per-author figure is an agent identity (model, harness, effort), and no view
  keys a row by a human (I58, ADR-051). Predictive estimates without a calibrated reading and
  auto-applied remedies are refused with it. Revisit when: never.
- **No generic token-usage dashboard.** Why: the clients and their usage tools own raw
  tokens, sessions and spend; a copy is commodity. Fathom shows cost only joined to what was
  proven (cost per proven row, ADR-049 decision 5). Revisit when: a client stops exposing its
  own usage and an adopter asks in writing.
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
