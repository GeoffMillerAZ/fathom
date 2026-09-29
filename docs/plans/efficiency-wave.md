# The efficiency wave: the plan (ADR-049 amendment of 2026-09-29)

Grammar v2 (`docs/plans/README.md`). A row flips to `[x]` only when
`fathom verify --service fathom --plan docs/plans/efficiency-wave.md` proves it. Decision: ADR-049's
amendment of 2026-09-29 (the efficiency wave goes first). No design doc: the build work lives in its home
plans and is referenced here by id; this plan adds only what no plan carries (five defects, a test sweep,
a latency reading), the adoption steps, the two stopping points and the measurements that say whether the
wave paid.

Order: stage 0 builds one line, stage 1 removes per-call waste, stage 2 adds the session tools, stop 1
deploys and adopts them, stage 3 builds the maps slice, stop 2 adopts it. Nothing here adds a table, cache
or background job (ADR-049 decision 2, ADR-056 R5): maps live in thread fields, test-run rows in the hook
log, docs search in the FTS5 index KR-1 created. No row needs a provider; XM-M2 alone spends model budget
and is asked for first. Adoption rows are proven by the artifact they change: `.fathom/config.toml`, or
`docs/guides/agent-waves.md`, the worker-brief contract written at stop 1.

## Stage 0: one line to build on

Also in this stage: XM-V1, then the XM-M1 (BUILD) and CA-M1 (STOP) scans of 2026-09-28 land at their rows'
oracle paths as statistics only.

- [ ] (op) EW-0: the integration reading: the four unmerged lines (the push line carrying the wired audit, the judgment plans, the roadmap wave and the language wave) merged onto one line that keeps the push line's state for every row the wired audit reopened, `plans status` naming each of them open {id: EW-0, oracle: lexical, path: docs/bench/readings/efficiency-wave/stops.md, pattern: "^EW-0 reading:"}

## Stage 1: per-call waste (small, deterministic, parallel)

Also in this stage: sdd-reconciler-v0.md G2-5 and G2-6 (then G2-7), so dispatch stops offering blocked
rows; progress-and-economics.md PE-26 to PE-28, so scratch runs stop writing the real ledgers;
surface-reckoning.md SR-M1, which pins the default surface's schema bytes.

- [x] The Bash-write hook serves at most one line, once per session, when it cannot name a write target (a quoted `>`, a `$VAR` target, a writer named inside a string), and every post-bash emission is a payload-cost beat {id: EW-1, symbol: cmdEditHookBashWrite, package: cmd/fathom, reachable_from: TestEditHookBashWriteDoubtServesAtMostOneLine}
- [x] The session-end git-concurrency test waits on a progress count with a large safety bound, not a two-second timer {id: EW-2, symbol: mechanicalSessionFacts, package: cmd/fathom, reachable_from: TestSessionEndFactsBarrierCountsProgress}
- [x] `max_file_ms` is the slowest file's CPU time, so the tenancy floor and TestCmdLangParity read no wall clock {id: EW-3, symbol: measureSubject, package: cmd/indexbench, reachable_from: TestMeasureSubjectMaxFileIsCPUTime}
- [x] The build-cache guard never runs `go clean -cache` on a root another build may hold (O124): it trims by age or prunes only a root it created {id: EW-4, oracle: lexical, path: scripts/lib/build-cache.sh, pattern: "clean -cache", eq: 0}
- [x] Every tool the rendered CLAUDE.md and AGENTS.md stanza names is on the surface the registration serves, and its refresh line names a path that works on the default surface {id: EW-5, symbol: stanzaPlan, package: cmd/fathom, reachable_from: TestStanzaNamesOnlyRegisteredSurfaceTools}
- [x] (op) EW-7: the wall-bound test sweep: each of the 38 wall-clock test assertions (12 at millisecond scale) is moved to CPU time, a progress count or a large safety bound, or kept with a named reason {id: EW-7, oracle: lexical, path: docs/bench/readings/efficiency-wave/wall-bounds.md, pattern: "^EW-7 reading:"}
- [x] Worker scratch runs are isolated: the guide sets `FATHOM_HOME` and a private build-cache root per worker {id: EW-8, oracle: lexical, path: docs/guides/agent-waves.md, pattern: "FATHOM_HOME", needs: "progress-and-economics.md#PE-28,EW-4"}

## Stage 2: the session tools

- [x] This repository turns on the test-run hook (tiers 0 and 1, provider none) {id: EW-9, oracle: lexical, path: .fathom/config.toml, pattern: "^test_run = true", needs: "test-run-hook.md#TR-7,test-run-hook.md#TR-8,test-run-hook.md#TR-10,test-run-hook.md#TR-11"}
- [x] Workers ask docs search (`search` with `kind: docs`) or a row's context pack before opening ADR.md, CHANGELOG.md, Taskfile.yml or a design doc whole {id: EW-10, oracle: lexical, path: docs/guides/agent-waves.md, pattern: "kind: docs", needs: "knowledge-retrieval.md#KR-8,knowledge-retrieval.md#KR-9"}
- [x] Workers run `fathom tests select`'s certifying-first set before any whole-package run {id: EW-11, oracle: lexical, path: docs/guides/agent-waves.md, pattern: "tests select", needs: "test-intelligence.md#TQ-15w,test-intelligence.md#TQ-16w,surface-reckoning.md#SR-50"}
- [x] (op) EW-6: session-hook and stop-hook cost read as CPU and wall time from the hook-latency log on a loaded and an idle machine (p50 895 ms and 2,261 ms under load on 2026-09-29), naming whether it is work or load; a fix row follows if it is work {id: EW-6, oracle: lexical, path: docs/bench/readings/efficiency-wave/hook-latency.md, pattern: "^EW-6 reading:"}
- [ ] The hook-latency record carries each invocation's CPU time (user plus system) beside its wall time, so a slow hook reads as work or load from the log alone; EW-6's two work findings (the session hook near 0.9 s at any load, the stop hook 1.6 s at an idle machine) are then re-read as CPU {id: EW-29, symbol: recordHookLatency, package: cmd/fathom, reachable_from: TestRecordHookLatencyCarriesCPUTime, needs: "EW-6"}

## Stop 1: push, deploy, adopt

- [ ] Every wave's gate runs `fathom verify --strict` with the integrated binary, whose wired check refuses a library-only claim, then one push per wave {id: EW-12, oracle: lexical, path: docs/guides/agent-waves.md, pattern: "verify --strict", needs: "EW-0"}
- [x] The pre-push verify step runs `--strict --since $(git merge-base HEAD origin/main)`, so a `library: true` or `skip_test` waiver added in the pushed range is self-declared and refused (I45); with no merge-base it verifies without `--since` and says so, exit 2 still fails open, and waivers already on main are untouched (ADR-018 note 2026-09-29) {id: EW-14, oracle: lexical, path: lefthook.yml, pattern: "merge-base HEAD origin/main"}
- [x] Wired-check hardening from the 2026-09-29 re-audit (an adversarial corpus of fixture binaries): a blank-identifier reference (`var _ = f`) is no product reach, an entry-point `main` under a build constraint the default build excludes is not an entry, a method matches only its own receiver type, and interface dispatch is cross-checked against an RTA reachable set {id: EW-15, symbol: ProductEntryReach, package: internal/analysis/callgraph, reachable_from: TestProductReach_HardeningCorpus}
- [x] The pre-push verify reads its `--since` base from the remote sha lefthook passes on stdin (the ref the push replaces), so a stale local `origin/main` neither blocks a legitimate push nor widens the range; a new branch with no remote sha falls back to the merge-base and says so {id: EW-16, oracle: lexical, path: lefthook.yml, pattern: "use_stdin: true"}
- [x] `task deploy`'s health wait outlasts launchd's respawn throttle and, on a failed spawn, prints launchd's last exit reason instead of a bare timeout (O129) {id: EW-17, oracle: lexical, path: Taskfile.yml, pattern: "last exit reason"}
- [x] A long `ingest` over /rpc never answers with an empty body: past the server's write budget it answers with the refresh queue's state, as /refresh-request does (O128) {id: EW-18, symbol: registerIngest, package: internal/mcp, reachable_from: TestIngestLongRunAnswersQueueState}
- [x] An `ingest` with `remap` refuses to re-point a service whose current root is a live checkout of the same repository onto a different path unless the caller names that root (`remap_from`), so a worker's scratch clone cannot capture the owner's service (O127) {id: EW-19, symbol: registerIngest, package: internal/mcp, reachable_from: TestIngestRemapRefusesScratchCloneOfLiveRoot}
- [x] `fathom doctor` names a registered service whose root no longer exists, with the `ingest` call that re-points it (O127) {id: EW-20, symbol: cmdDoctor, package: cmd/fathom, reachable_from: TestDoctorNamesServiceWithMissingRoot}
- [x] The prompt hook drops a path candidate that names no file in the index or the working tree, and serves nothing for a cross-agent relay message (O126) {id: EW-21, symbol: extractPromptCandidates, package: cmd/fathom, reachable_from: TestPromptCandidatesDropNonexistentPaths}
- [x] The first `verify_plan` after an ingest or daemon start answers within its deadline: the product-reach memo the wired check reads is warmed when the watermark moves, or an in-flight walk is awaited rather than abandoned (O130) {id: EW-22, symbol: verifyPlanWithDeadline, package: internal/mcp, reachable_from: TestVerifyPlanFirstCallAfterIngestAnswers}
- [ ] (op) EW-S1: the stop-1 reading: the integrated line passed the gate and was pushed, the owner ran `task deploy`, `fathom doctor` on the main checkout shows the daemon serving HEAD and the stanza's tools on the served surface, and the next wave used the guide {id: EW-S1, oracle: lexical, path: docs/bench/readings/efficiency-wave/stops.md, pattern: "^EW-S1 reading:", needs: "EW-0,EW-1,EW-2,EW-3,EW-4,EW-5,EW-7,EW-8,EW-9,EW-10,EW-11,EW-12,sdd-reconciler-v0.md#G2-5,sdd-reconciler-v0.md#G2-6,surface-reckoning.md#SR-M1", weight: 2}

## Stage 3: the maps slice, then stop 2

The build is exploration-maps.md XM-1 to XM-9 and XM-14 (thread-backed), licensed by XM-M1's BUILD; XM-11
follows because XM-M2 needs it.

- [ ] Every worker brief carries one map id: the wave map the orchestrator mints before dispatch, composed with the task's own map, which workers append to with provenance {id: EW-13, oracle: lexical, path: docs/guides/agent-waves.md, pattern: "map id", needs: "exploration-maps.md#XM-8,exploration-maps.md#XM-9,exploration-maps.md#XM-14,EW-M3"}
- [ ] (op) EW-S2: the stop-2 reading: the maps slice deployed and the next two waves dispatched with a map id in every brief {id: EW-S2, oracle: lexical, path: docs/bench/readings/efficiency-wave/stops.md, pattern: "^EW-S2 reading:", needs: "EW-13,EW-M1", weight: 2}

## Measurement: did the wave pay

- [ ] (op) EW-M1: sub-agent tool_result plus hook-injected tokens per proven row (rows `[x]` at a window's end and not at its start, on the integrated line) in the 14 days after stop 1 are at least 15 percent below the 14 days before it (about 74k tool_result tokens per row: 49.69M over 671 rows), the wave-cluster bootstrap interval excluding 0 {id: EW-M1, oracle: lexical, path: docs/bench/readings/efficiency-wave/payoff.md, pattern: "^EW-M1 verdict: (PASS|FAIL)", needs: "EW-S1", weight: 3}
- [ ] (op) EW-M2: push attempts per landed push after stop 1 are at most 1.5 over at least 6 landings (10 attempts for 3 landings on 2026-09-28), and no gate failure traces to a test EW-2, EW-3 or EW-7 fixed or to the cache race EW-4 removes {id: EW-M2, oracle: lexical, path: docs/bench/readings/efficiency-wave/payoff.md, pattern: "^EW-M2 verdict: (PASS|FAIL)", needs: "EW-S1"}
- [ ] (op) EW-M3: XM-M1's scan re-run over the 14 days after stop 1, fleet and since-last-compaction variants (69.9 and 15.1 percent before), naming the map shape to adopt: a shared wave map, or per-child handoff when the since-compaction interval sits above 15 percent {id: EW-M3, oracle: lexical, path: docs/bench/readings/efficiency-wave/rederivation-after.md, pattern: "^EW-M3 reading:", needs: "EW-S1"}
- [ ] (op) EW-M4: after stop 2, tokens per proven row at least 30 percent below the EW-M1 baseline, read beside XM-M2 on our own worker briefs {id: EW-M4, oracle: lexical, path: docs/bench/readings/efficiency-wave/payoff.md, pattern: "^EW-M4 verdict: (PASS|FAIL)", needs: "EW-S2,exploration-maps.md#XM-M2", weight: 3}

## Not in the wave (recorded so they are not re-proposed)

- [-] Carry-across (carry-across.md CA-1 to CA-5) {id: EW-D1, disposition: deferred, reason: "CA-M1 read STOP: median 0.5 pre-set files re-read after compaction against a bar of 2, no different from 94 control points (p 0.39), and 0.64 compactions a day", trigger: "compactions reach 2 a day over 14 days and a CA-M1 re-scan reads BUILD"}
- [-] Per-child handoff as the default map delivery {id: EW-D2, disposition: deferred, reason: "2.8 percent of sub-agent reads hit a file the parent touched in the hour before spawning and 15.1 percent (interval 11.2 to 19.5) one it touched since its last compaction, from one parent session", trigger: "EW-M3 reads the since-compaction interval above 15 percent"}
- [-] Work-signal stall detectors (work-signals.md WS-1 to WS-6) {id: EW-D3, disposition: deferred, reason: "the detectors are a daemon fold, a background job ADR-049 decision 2 holds back", trigger: "responsible-tenant.md RT-M1 reads PASS"}
- [-] Surface groups (reach.md RH-1, RH-2) {id: EW-D4, disposition: deferred, reason: "RH-3's call log holds 7 rows so the default cannot be read yet; EW-5 removes the wasted calls without groups", trigger: "RH-3 reads"}
- [-] Rule retirement from outcomes (governance-learning.md GL-13, GL-14, GL-19) {id: EW-D5, disposition: deferred, reason: "needs SR-61 merged and 30 days of served, complied and violated signal", trigger: "30 days after EW-S1"}
- [-] The A/Bs that spend model budget other than XM-M2 (TR-M1, CA-M2, WS-M2, ambient-fathom.md OP-1) {id: EW-D6, disposition: deferred, reason: "they spend LLM budget and none gates the wave", trigger: "the owner approves the spend"}
