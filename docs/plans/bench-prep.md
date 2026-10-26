# Bench prep: vetted third-party subjects, a task bank, dry-run harnesses and pre-registered reads before any spend

Grammar: v2. Prefix: BX- (BX-n work rows, BX-Rn the readiness checklist, BX-Mn milestones and preregs).
Design: `docs/design/bench-prep.md`. Decisions applied: ADR-031 to ADR-034 (BX-40 amends ADR-034 decision 2),
ADR-029 decision 6 (the Codex arm waits), ADR-049 decision 2 (the table freeze), ADR-011.

Scope: bench data are files (manifests, corpora, cells, ledgers, readings, preregs). No store table, column, cache
or product background job. P0 and P1 spend $0. Every row that spends agent budget is `[-]` deferred until the owner
approves its envelope (`docs/intent/autonomy.md`, Held). Ids BX-1 to BX-31 and BX-M0 to BX-M3 keep the roadmap
draft's meaning as amended by the critique; new rows start at BX-32.

Lanes, the first word of each title: `codex` is a helper-Mac Codex lane working from a written brief (the kit rows
need the agentic-kit checkout and land through `task sync-kit`); `cloud` is a cloud lane; `ctl` is the controller
only. A library row is claimed in the same change as the row that wires it into `cmd/scenariobench`, never with
`library: true`. Readings live under `docs/bench/readings/bench-prep/`, preregs under `docs/bench/prereg/`,
subject manifests under `calibration/bench.d/`, raw receipts under `workspace/bench/`. Q1 to Q8 are the owner
questions of the design's section 15, not the ids of `docs/intent/questions.md`.

Verify before claiming any row: `fathom verify --service fathom --plan docs/plans/bench-prep.md`.

## P0: subjects, stratum yield, dry runs of the existing banks, method records ($0, starts today)

### Subjects

- [ ] codex: `schema/bench` CUE package with `#Subject` (corpus.d's repo entry unchanged plus the `bench:` block: toolchain with lockfile sha256, setup, test, suite_seconds, flake_runs, l0_reading, split, task_sources with dataset, revision and licence, contamination with age, popularity band and fresh_after) and `#Prereg`, valid and commented invalid testdata {id: BX-1, oracle: lexical, path: schema/bench/schema.cue, pattern: "^#Subject:"}
- [ ] codex: `LoadSubjectManifests` reads `calibration/bench.d/*.yaml` against `#Subject` and refuses a file name that is not the subject id, a missing sha and a licence off the C1 list; every negative in `invalid.cue` is asserted through the CUE Go API; claimed with BX-3 {id: BX-32, symbol: LoadSubjectManifests, package: internal/benchprep, needs: BX-1}
- [ ] codex: `scenariobench vet` runs `VetSubject` per manifest and writes `workspace/bench/vet/<subject>.json`: C1 licence at the pinned sha, C2 line band, C3 suite green 3 times with no network and its timings, C4 find_callers and resolved blast_radius precision and recall at the 0.95 floor against go/types or SCIP, C5 fix-commit yield (the post-cutoff count pending Q2), C6 fan-in from the truth graph, C7 independence; a failed criterion names its evidence {id: BX-3, symbol: VetSubject, package: internal/benchprep, needs: "BX-32,BX-33"}
- [ ] codex: `MineFixCommits` walks a full clone under `workspace/scenariobench/subjects/` and keeps commits whose touched tests fail at the parent and pass at the commit with the rest of the suite green, recording created_at and the revert-and-redo task shape {id: BX-33, symbol: MineFixCommits, package: internal/benchprep}
- [-] `calibration/clone.sh` takes a manifest directory {id: BX-2, disposition: refused, reason: "clone.sh clones --depth 1 and cannot mine fix commits; MineFixCommits uses scenariobench's full clones (BX-33) and the subject prep reads bench.d (critique 12)"}

### Strata

- [ ] codex: `StratifyTask` assigns an F1 task's stratum under both pre-declared definitions (D-a: no issue identifier occurs in a gold file at base; D-b: the gold file ranks outside a fixed BM25 top 10 over the issue's identifiers) and records lexical_rank; it reads the subject tree and the issue text, never fathom's index {id: BX-8, symbol: StratifyTask, package: internal/benchprep}
- [ ] codex: `HopDistance` is the shortest call-graph distance from any symbol the issue names to a gold function in the go/types or SCIP truth, with the buckets no-graph-path and no-oracle; a covariate, never a stratum {id: BX-34, symbol: HopDistance, package: internal/benchprep}

### Dry-run harnesses

- [ ] codex: `FetchGoldPatch` reads the gold patch from the pinned dataset revision the task's source names, checks it against the task's patch sha256, caches it under `workspace/` and never writes it into the tree {id: BX-35, symbol: FetchGoldPatch, package: internal/scenariobench}
- [ ] codex: `-harness null` (`NewNullHarness`) makes no edit and gives an empty answer {id: BX-19, symbol: NewNullHarness, package: internal/scenariobench}
- [ ] codex: `-harness oracle` (`NewOracleHarness`) applies the verified gold patch on F2 tasks and answers the gold files on F1 tasks {id: BX-20, symbol: NewOracleHarness, package: internal/scenariobench, needs: BX-35}
- [ ] codex: `-harness wrong-patch` (`NewWrongPatchHarness`) applies the test patch alone or a killed mutant on F2 tasks and answers a fixed non-gold file set on F1 tasks {id: BX-21, symbol: NewWrongPatchHarness, package: internal/scenariobench, needs: BX-35}
- [ ] codex: `scenariobench dryrun` prints `DryRunVerdict` over a corpus's null, oracle and wrong-patch cells at 3 trials: keep, or drop naming the reason (null passed, oracle failed, wrong patch passed, flaky) {id: BX-36, symbol: DryRunVerdict, package: internal/scenariobench, needs: "BX-19,BX-20,BX-21"}

### Statistics, upstream in agentic-kit and mirrored by `task sync-kit` (claimed with BX-63, which wires them)

- [ ] codex: exact McNemar test on paired binary outcomes, with the two-sided p and the discordant counts {id: BX-25, symbol: McNemarExact, package: third_party/agentic-kit/foundations/quality/reliability-stats}
- [ ] codex: Holm step-down adjustment over a family of p-values, input order preserved {id: BX-37, symbol: HolmAdjust, package: third_party/agentic-kit/foundations/quality/reliability-stats}
- [ ] codex: wild-cluster bootstrap interval with Webb six-point weights over caller-given clusters, seeded {id: BX-38, symbol: WildClusterBootstrapCI, package: third_party/agentic-kit/foundations/quality/reliability-stats}
- [ ] codex: paired equivalence by two one-sided tests (the 1 - 2 alpha interval inside a margin), returning the verdict and the interval {id: BX-39, symbol: EquivalenceTOST, package: third_party/agentic-kit/foundations/quality/reliability-stats}

### Method records

- [ ] ctl: prereg template `docs/bench/prereg/README.md`: hypotheses and a prediction per arm including the predicted nulls, strata, arms, primary metric and MEI, TOST margins, the three outcomes and the tie rule, the Holm family, n from the power table, fixed n, turn and cost caps, the no-read rules, exclusions, scoring by assignment with the zero-call line and a per-protocol secondary labelled biased toward fathom, the deviation log, the prereg sha on every ledger row {id: BX-24, oracle: lexical, path: docs/bench/prereg/README.md, pattern: "^## Three outcomes$", needs: BX-40}
- [ ] ctl: ADR-034 amended: a LOST cell stays an instrument defect; a surface_unused cell is a treatment outcome kept in a prereg's primary and counted on the zero-call line, and is still refused as an `oracle: ledger` anchor for a tool effect {id: BX-40, oracle: lexical, path: ADR.md, pattern: "^\*\*Amended 2026-[0-9-]+ \(bench prep: scoring by assignment\)"}

### P0 readings

- [ ] (op) cloud: licence audit of the datasets (Multi-SWE-bench, SWE-PolyBench, ContextBench, LCA, SWE-rebench, SWE-bench-Live) at their pins, naming what may be committed and what is kept as sha256 only {id: BX-41, oracle: lexical, path: docs/bench/readings/bench-prep/licences.md, pattern: "^BX-41 reading:"}
- [ ] (op) codex: vetting reading over 15 public candidates (5 each in Go, Python and TypeScript): every C1 to C7 receipt summarized, refusals with their reasons, the C4 refusal rate stated as a finding, C5's post-cutoff half marked pending Q2 {id: BX-4, oracle: lexical, path: docs/bench/readings/bench-prep/vetting.md, pattern: "^BX-4 reading: [0-9]+ of [0-9]+ candidates kept", needs: BX-3}
- [ ] (op) codex: stratum yield on LCA (40) and public-go (35) under D-a and D-b, with the hop-distance distribution and the no-graph-path share; the controller records which definition H4 uses before any agent spend {id: BX-42, oracle: lexical, path: docs/bench/readings/bench-prep/stratum-yield.md, pattern: "^BX-42 reading:", needs: "BX-8,BX-34"}
- [ ] (op) codex: dry-run reading of the existing banks (public-go 35, reuse-go 4, LCA 40): null passes 0, oracle passes every kept task, wrong-patch passes 0, no flake over 3 trials, every drop named {id: BX-43, oracle: lexical, path: docs/bench/readings/bench-prep/dry-run-existing.md, pattern: "^BX-43 reading:", needs: BX-36}
- [ ] (op) ctl: P0 exit: the plan verifies, BX-4, BX-42 and BX-43 are committed, and at least 10 candidates pass C1 to C4, C6 and C7 {id: BX-M0, oracle: lexical, path: docs/bench/readings/bench-prep/p0-exit.md, pattern: "^BX-M0 verdict: (PASS|FAIL)", needs: "BX-4,BX-42,BX-43"}
- [-] Run the PB-14 import into `tasks-public-go.json` {id: BX-6, disposition: refused, reason: "capped at 30 to 40 Go tasks by PB-14's own rule and cli/cli-heavy; its SWE-Explore half is CC-BY-NC-ND so nothing derived may be committed; Multi-SWE-bench tasks enter through BX-50 on vetted subjects (critique 10)"}

## P1: the bank, harness hardening, free readings, preregs and readiness ($0)

### Subjects and bank

- [ ] (op) codex: L0 per kept subject: fathom, gopls and Serena on the Go subjects, fathom and Serena on Python and TypeScript, against go/types or SCIP through `indexbench compare`, rows on `docs/bench/competitors-ledger.jsonl`; a subject under 0.95 is named and kept out of agent runs (ADR-032) {id: BX-5, oracle: lexical, path: docs/bench/readings/bench-prep/l0-subjects.md, pattern: "^BX-5 reading:", needs: BX-4}
- [ ] (op) codex: vetting continues to 6 kept subjects per P1 language (3 dev and 3 held-out by repository) from about 24 candidates, with the refusals and the C4 refusal rate updated {id: BX-44, oracle: lexical, path: docs/bench/readings/bench-prep/vetting.md, pattern: "^BX-44 reading: [0-9]+ subjects kept", needs: BX-4}
- [ ] (op) cloud: C3 re-validated on Linux for every kept subject: the suite green 3 times with no network, timings beside the macOS ones {id: BX-45, oracle: lexical, path: docs/bench/readings/bench-prep/linux-c3.md, pattern: "^BX-45 reading:", needs: BX-44}
- [ ] codex: `ImportPolyBench` reads the pinned SWE-PolyBench 500 subset (Python, TypeScript, Java) with its file and node localization truth and the licence recorded at the pin {id: BX-7, symbol: ImportPolyBench, package: internal/benchprep, needs: BX-41}
- [ ] codex: `ImportContextBenchGold` reads ContextBench gold contexts as F1 retrieval truth beside the fix-commit files {id: BX-46, symbol: ImportContextBenchGold, package: internal/benchprep, needs: BX-41}
- [ ] codex: the scenario JSON's additive bench fields (stratum, split, source with dataset, revision and licence, created_at, lexical_rank, hop_distance) are validated at load, and `LoadScenarioCorpus` refuses a held-out or non-CC0 task that carries a verbatim test patch or any gold patch {id: BX-47, symbol: validateBenchFields, package: internal/scenariobench}
- [ ] codex: `ResolveTestPatch` fetches a sha256-only test patch from the pinned dataset revision at run time and refuses on a mismatch; `RunOracle` and `CalibrateSubject` read through it {id: BX-48, symbol: ResolveTestPatch, package: internal/scenariobench, needs: BX-47}
- [ ] codex: `ImportLiveSlice` reads SWE-rebench and SWE-bench-Live tasks dated after the pinned model's training cutoff, on vetted repositories only {id: BX-49, symbol: ImportLiveSlice, package: internal/benchprep, needs: BX-41}
- [ ] codex: `scenariobench bank` runs `BuildTaskBank`: Multi-SWE-bench through `DeriveScenarioTasks`, LCA through `BuildLCACorpus`, BX-7, BX-46, BX-49 and BX-33's mined commits, keeping a task only on a repository with C1 and C4 receipts and stamping every bench field {id: BX-50, symbol: BuildTaskBank, package: internal/benchprep, needs: "BX-7,BX-46,BX-49,BX-47,BX-8,BX-34,BX-33"}
- [ ] codex: hit@3 on every localization reading (`HitAtK`) beside hit@1 and hit@5 {id: BX-51, symbol: HitAtK, package: internal/scenariobench}
- [ ] codex: `ScoreFunctionLocalization` scores function hit@5 against the fix commit's enclosing functions (`EnclosingFuncsForHunks`) {id: BX-52, symbol: ScoreFunctionLocalization, package: internal/scenariobench}
- [ ] codex: `GenerateFalseRows` builds F3: a true row per gold function and its test, false rows by construction (stub, deleted test, non-reaching test, unwired entry, rename), and refuted rows harvested from real agent plans in prior scenario runs with their run ids {id: BX-9, symbol: GenerateFalseRows, package: internal/benchprep}
- [ ] codex: `MineMutants` runs go-mutesting, mutmut or Stryker as each documents at a pinned version and keeps mutants at fan-in 3 or more that only a test outside their package kills {id: BX-10, symbol: MineMutants, package: internal/benchprep}
- [ ] codex: `BaselineCallers` lists the callers of a change's symbols by rg and by gopls through `ProbeReferencesViaMCP`, the H3 and H5 baseline {id: BX-53, symbol: BaselineCallers, package: internal/benchprep}
- [ ] codex: `SplitHeldOut` splits the bank by repository (3 dev and 3 held-out per P1 language) plus a post-cutoff time slice and writes the sha256 of each split's task-id list {id: BX-54, symbol: SplitHeldOut, package: internal/benchprep, needs: BX-50}
- [ ] codex: `RecordHeldOutRead`: `scenariobench run` refuses a held-out task unless `-heldout-read` names a signed prereg, appends the read to `docs/bench/heldout-reads.md`, and refuses a second read of a split sha until that split is demoted to dev {id: BX-29, symbol: RecordHeldOutRead, package: internal/benchprep, needs: BX-54}
- [ ] (op) ctl: the held-out split committed: each split's task-id sha256 in `docs/bench/heldout-reads.md` above an empty read log, after Q2 sets the post-cutoff slice {id: BX-11, oracle: lexical, path: docs/bench/heldout-reads.md, pattern: "^Held-out split sha256: [0-9a-f]{64}", needs: "BX-54,BX-29"}
- [ ] (op) codex: bank reading: tasks per family, stratum, language, split and licence against the design's targets, every shortfall named {id: BX-12, oracle: lexical, path: docs/bench/readings/bench-prep/bank.md, pattern: "^BX-12 reading:", needs: "BX-50,BX-9,BX-10,BX-54"}

### Harness hardening

- [ ] (op) codex: egress spike: Claude Code's Bash sandbox settings against a macOS sandbox profile on the agent's tool subprocesses, keeping the harness's API connection and loopback to the scratch daemon; one is chosen with evidence {id: BX-55, oracle: lexical, path: docs/bench/readings/bench-prep/egress-spike.md, pattern: "^BX-55 reading:"}
- [ ] codex: `RenderNetworkDeny` writes the chosen network denial into every cell's settings, loopback to the cell's own scratch daemon excepted {id: BX-13, symbol: RenderNetworkDeny, package: internal/scenariobench, needs: BX-55}
- [ ] codex: `ApplyOfflineEnv` pre-warms the module cache, wheelhouse and npm cache outside the cell from the manifest's sources, then sets GOPROXY=off, GOFLAGS=-mod=readonly, PIP_NO_INDEX=1 with PIP_FIND_LINKS and npm_config_offline=true inside it {id: BX-56, symbol: ApplyOfflineEnv, package: internal/scenariobench}
- [ ] codex: `RecordArmPins` stamps every ledger row with the exact model id, the claude CLI version, the fathom binary sha, the MCP surface, the tool list and the measured tool-schema tokens; `-model` given an alias (sonnet, opus, haiku) is refused {id: BX-14, symbol: RecordArmPins, package: internal/scenariobench}
- [ ] codex: `-max-usd` (`CheckSpendCap`) stops scheduling when the next cell's planned cost would pass the cap and records `capped: true` on the run {id: BX-15, symbol: CheckSpendCap, package: internal/scenariobench}
- [ ] codex: `MaxTurnsStops` counts max-turns stops per arm onto the ledger row, with the turn cap read from the prereg {id: BX-57, symbol: MaxTurnsStops, package: internal/scenariobench}
- [ ] codex: `InterleaveCells` interleaves arms within each task and shuffles task order from the prereg's recorded seed {id: BX-16, symbol: InterleaveCells, package: internal/scenariobench}
- [ ] codex: `ClassifyLostCell` retries infrastructure failures (a LOST server, a harness crash, API transport, oracle infrastructure) at most twice, never retries a model outcome, and counts lost cells per arm {id: BX-17, symbol: ClassifyLostCell, package: internal/scenariobench}
- [ ] codex: `LeakedScenarioTasks` ports contextbench's leak rule: an A0 hit@3 in 2 or more of 3 trials marks a task leaked, excluded from the verdict and listed, with the per-subject A0 pass rate on the row {id: BX-58, symbol: LeakedScenarioTasks, package: internal/scenariobench}
- [ ] codex: `PreregArms` declares A0, A1, A2, A3, A3m, A4, A1+V and A1+T as data; the H1 arms share one plan-first instruction (grammar v2 rows with symbol and package) and A4 carries a routing stanza of fathom's shape {id: BX-59, symbol: PreregArms, package: internal/benchprep}
- [ ] codex: `CheckArmParity` renders each arm and A1 through `RenderArm` and fails on any difference the arm does not declare: prompt, tools, settings, notes file, turn cap, model, worktree tree hash {id: BX-60, symbol: CheckArmParity, package: internal/scenariobench, needs: BX-59}
- [ ] codex: `-harness scripted-fathom` (`NewScriptedFathomHarness`) sends the issue's identifiers through search, find_symbol and symbol_context over the cell's MCP and proves surface_calls above 0 in every wired cell; a harness check, never a floor or a ceiling {id: BX-22, symbol: NewScriptedFathomHarness, package: internal/scenariobench}
- [ ] codex: `-harness scripted-grep` (`NewScriptedGrepHarness`) ranks files by rg over the issue's identifiers to prove the A1 pipeline end to end; its per-stratum scores are never reported because the same ranking defines the strata {id: BX-61, symbol: NewScriptedGrepHarness, package: internal/scenariobench}

### Pre-registration and analysis

- [ ] codex: `LoadPrereg` reads `docs/bench/prereg/<id>.json` against `#Prereg`; `scenariobench run -prereg` refuses a dirty tree or a file that differs from HEAD and stamps prereg_sha256 on every ledger row {id: BX-62, symbol: LoadPrereg, package: internal/benchprep, needs: BX-1}
- [ ] codex: `ClassifyOutcome` reads confirmed, refuted or inconclusive per primary (an interval inside 0 to MEI reads refuted), equivalent, harm or inconclusive per predicted-null stratum by TOST, and no-read for a capped run or one losing over 5% of an arm's cells; the primary interval is the wild-cluster bootstrap over repositories with the task-clustered bootstrap beside it and the more conservative deciding; Holm across the prereg's primaries {id: BX-63, symbol: ClassifyOutcome, package: internal/benchprep, needs: "BX-62,BX-25,BX-37,BX-38,BX-39,BX-15,BX-17"}
- [ ] codex: sample size for a paired mean difference with a design effect for clustering, and for discordant pairs {id: BX-64, symbol: PairedSampleSize, package: third_party/agentic-kit/foundations/quality/reliability-stats}
- [ ] codex: `PowerTable` gives n per primary from a run's measured standard deviation of task-mean differences, intra-repository correlation and false-done base rate, and says whether the bank can fill it {id: BX-26, symbol: PowerTable, package: internal/benchprep, needs: BX-64}
- [ ] codex: `RenderPreregResults` renders one table, losses first: per primary the estimate, interval and outcome; per-repository effects; cost and tokens with tool-schema tokens; each wired arm's zero-call rate on its own line; lost cells and max-turns stops per arm; A0 leaks; deviations; and a mandatory Where fathom lost section {id: BX-27, symbol: RenderPreregResults, package: internal/benchprep, needs: "BX-63,BX-57,BX-58"}
- [ ] codex: the fake harness drives plan, run, score, ledger and the results template end to end over committed streams for every arm `PreregArms` declares {id: BX-23, symbol: TestPreregPipeline_FakeHarnessEndToEnd, package: cmd/scenariobench, skip_test: true, skip: "the test is the anchor: it drives the whole pipeline over committed streams", needs: "BX-27,BX-59"}
- [ ] (op) cloud: second-opinion review of the H4 and H1 prereg drafts, every finding answered in the prereg's deviation log before sign-off {id: BX-65, oracle: lexical, path: docs/bench/readings/bench-prep/prereg-review.md, pattern: "^BX-65 reading:", needs: BX-24}
- [ ] (op) ctl: H4 prereg v1 signed: the stratum definition BX-42 chose, hop distance as a covariate, A3 against A1 as the primary, A2, A3m and A4 as secondaries, the A0 leak arm, MEI +10 points of hit@3 in the non-lexical stratum, a TOST margin of 5 points in the lexical stratum, cost non-inferiority at 1.3, n left to BX-71 {id: BX-M1a, oracle: lexical, path: docs/bench/prereg/h4-localization.md, pattern: "^Prereg H4 v1 signed", needs: "BX-24,BX-42,BX-59,BX-65"}
- [ ] (op) ctl: H1 prereg v1 signed: the free gate first (BX-69), then A1+V against A1 and A1+V against A1+T on the per-task false-done rate as primaries, F2P success held by a 5-point TOST margin, A3 against A1 as a secondary, n left to BX-71 {id: BX-M1b, oracle: lexical, path: docs/bench/prereg/h1-verify-gate.md, pattern: "^Prereg H1 v1 signed", needs: "BX-24,BX-59,BX-65,BX-69"}

### Free readings ($0, each against its baseline)

- [ ] (op) codex: H3 on F4: review names a killing caller or test outside the diff, with recall and false findings per change, against rg and gopls callers (BX-53) {id: BX-66, oracle: lexical, path: docs/bench/readings/bench-prep/h3-review.md, pattern: "^BX-66 reading:", needs: "BX-10,BX-53,BX-5"}
- [ ] (op) codex: H5 on F5: call-site recall and consumer build and test results from the cv-m4 apidiff pipeline on the kept subjects' public consumers, against grep, go.work and gopls {id: BX-67, oracle: lexical, path: docs/bench/readings/bench-prep/h5-cross-repo.md, pattern: "^BX-67 reading:", needs: "BX-53,BX-44"}
- [ ] (op) codex: H8 on F6: safety recall 1.0 and the share of the suite run, against the whole suite, Go's test cache and pytest-testmon {id: BX-68, oracle: lexical, path: docs/bench/readings/bench-prep/h8-test-selection.md, pattern: "^BX-68 reading:", needs: "BX-33,BX-44"}
- [ ] (op) codex: H1's free gate on F3: verify's precision and false-flag rate on constructed and harvested false rows against the coverage check (`FalseDoneRate`'s coverage join), bar precision 0.90 or more and false flags 10% or less {id: BX-69, oracle: lexical, path: docs/bench/readings/bench-prep/h1-fixtures.md, pattern: "^BX-69 reading:", needs: BX-9}
- [ ] codex: hermetic F3 fixtures and a small committed F4 set hold their verdicts under `task validate` {id: BX-30, symbol: TestHermeticBenchFixturesHoldTheirVerdicts, package: internal/benchprep, skip_test: true, skip: "the test is the anchor: it replays the committed F3 and F4 fixtures", needs: "BX-9,BX-10"}

### Readiness for the first approved run (one reading, one line per check)

- [ ] (op) codex: every kept task passes the null, oracle, wrong-patch and 3-trial flake checks {id: BX-R1, oracle: lexical, path: docs/bench/readings/bench-prep/readiness.md, pattern: "^R1 PASS:", needs: "BX-12,BX-36"}
- [ ] (op) codex: egress denial proven inside a cell: curl to github.com fails, go mod download of the subject at latest fails, pip download fails, and the suite passes from the cache {id: BX-R2, oracle: lexical, path: docs/bench/readings/bench-prep/readiness.md, pattern: "^R2 PASS:", needs: "BX-13,BX-56"}
- [ ] (op) ctl: the model ids and the claude CLI version are pinned in the run config (Q2) {id: BX-R3, oracle: lexical, path: docs/bench/readings/bench-prep/readiness.md, pattern: "^R3 PASS:", needs: BX-14}
- [ ] (op) ctl: the shakedown's -max-usd cap and turn cap are set {id: BX-R4, oracle: lexical, path: docs/bench/readings/bench-prep/readiness.md, pattern: "^R4 PASS:", needs: "BX-15,BX-57"}
- [ ] (op) ctl: the preregs are committed and the run config carries their sha256 {id: BX-R5, oracle: lexical, path: docs/bench/readings/bench-prep/readiness.md, pattern: "^R5 PASS:", needs: "BX-M1a,BX-M1b,BX-62"}
- [ ] (op) ctl: the held-out split is committed and its read log is empty {id: BX-R6, oracle: lexical, path: docs/bench/readings/bench-prep/readiness.md, pattern: "^R6 PASS:", needs: BX-11}
- [ ] (op) codex: the scripted fathom agent shows MCP calls in 100% of wired cells {id: BX-R7, oracle: lexical, path: docs/bench/readings/bench-prep/readiness.md, pattern: "^R7 PASS:", needs: BX-22}
- [ ] (op) codex: `scenariobench plan` cell counts equal the shakedown plan's for every arm {id: BX-R8, oracle: lexical, path: docs/bench/readings/bench-prep/readiness.md, pattern: "^R8 PASS:", needs: BX-23}
- [ ] (op) codex: arm parity passes for every arm {id: BX-R9, oracle: lexical, path: docs/bench/readings/bench-prep/readiness.md, pattern: "^R9 PASS:", needs: BX-60}
- [ ] (op) ctl: host headroom: one run at a time, on the helper Mac, within the heavy-process cap {id: BX-R10, oracle: lexical, path: docs/bench/readings/bench-prep/readiness.md, pattern: "^R10 PASS:"}
- [ ] (op) ctl: P1 exit: R1 to R10 read PASS, and the L0, bank and four free readings are committed {id: BX-M1, oracle: lexical, path: docs/bench/readings/bench-prep/readiness.md, pattern: "^BX-M1 verdict: PASS", needs: "BX-R1,BX-R2,BX-R3,BX-R4,BX-R5,BX-R6,BX-R7,BX-R8,BX-R9,BX-R10,BX-5,BX-12,BX-66,BX-67,BX-68,BX-69"}

## P2: owner-approved spend

- [-] (op) ctl: shakedown, no claim: 10 F1 dev tasks (A0 at 3 trials, A1, A3) and 10 F2 dev tasks (A1, A1+V, A1+T, A3) at 1 trial; measures the zero-call rate, lost cells, cost per task shape at p50 and p95, turns and max-turns stops, the variance of hit@3 and the false-done base rate; its tasks then leave the dev analysis set {id: BX-70, disposition: deferred, reason: "agent spend is held (autonomy.md); expected about $55 behind a $120 cap", trigger: "the owner approves the shakedown (Q1)", needs: BX-M1, oracle: lexical, path: docs/bench/readings/bench-prep/shakedown.md, pattern: "^BX-70 reading:"}
- [ ] (op) ctl: both preregs amended with n, the cost caps and the turn caps from the shakedown before any dev data exist (T5), or a hypothesis declared unpowered and not run {id: BX-71, oracle: lexical, path: docs/bench/readings/bench-prep/power.md, pattern: "^BX-71 reading:", needs: "BX-70,BX-26"}
- [-] (op) ctl: H4 dev run at the prereg's n: A1, A3 and A4 at 3 trials, A0 at 3, A2 and A3m as Q3 decides, read by `ClassifyOutcome` {id: BX-72, disposition: deferred, reason: "agent spend is held; the envelope is sized from BX-70", trigger: "the owner approves the H4 envelope", needs: BX-71, oracle: lexical, path: docs/bench/readings/bench-prep/h4-dev.md, pattern: "^BX-72 reading:"}
- [-] (op) ctl: H1 dev run at the prereg's n: A1, A1+V, A1+T and A3 at 3 trials, read by `ClassifyOutcome` {id: BX-73, disposition: deferred, reason: "agent spend is held; the envelope is sized from BX-70", trigger: "the owner approves the H1 envelope", needs: BX-71, oracle: lexical, path: docs/bench/readings/bench-prep/h1-dev.md, pattern: "^BX-73 reading:"}
- [ ] (op) ctl: P2 exit: the H4 and H1 dev readings published in `docs/bench/` either way, rendered by `RenderPreregResults`, a null as prominent as a win {id: BX-M2, oracle: lexical, path: docs/bench/readings/bench-prep/p2-exit.md, pattern: "^BX-M2 verdict:", needs: "BX-72,BX-73"}

## P3: tuning, held-out reads, the conditional and deferred work

- [ ] ctl: knob map `docs/bench/prereg/knobs.md`: each losing stratum named with the knobs that can move it (verify criteria, search receipts, blast_radius depth, surface membership, the review byte cap, obligations ranking) {id: BX-28, oracle: lexical, path: docs/bench/prereg/knobs.md, pattern: "^## Knob map$", needs: BX-M2}
- [ ] ctl: tuning discipline in the prereg template: dev only, every tuning commit cites its stratum and dev delta, agent spend for tuning needs its own prereg row {id: BX-31, oracle: lexical, path: docs/bench/prereg/README.md, pattern: "^## Tuning$", needs: BX-24}
- [ ] codex: the scheduled free layer over the corpus as a helper-Mac lane script, never a product job, filing `fathom observe` on a breached threshold {id: BX-74, oracle: lexical, path: scripts/bench-free-layer.sh, pattern: "fathom observe", needs: "BX-66,BX-67,BX-68,BX-69"}
- [-] (op) ctl: H4 held-out confirmatory read {id: BX-75, disposition: deferred, reason: "one clean read per split; agent spend is held", trigger: "a named release candidate (Q6) after BX-M2", needs: "BX-M2,BX-11", oracle: lexical, path: docs/bench/readings/bench-prep/h4-heldout.md, pattern: "^BX-75 reading:"}
- [-] (op) ctl: H1 held-out confirmatory read {id: BX-76, disposition: deferred, reason: "one clean read per split; agent spend is held", trigger: "a named release candidate (Q6) after BX-M2", needs: "BX-M2,BX-11", oracle: lexical, path: docs/bench/readings/bench-prep/h1-heldout.md, pattern: "^BX-76 reading:"}
- [-] codex: gopls and graphify agent arms beside A4 {id: BX-77, disposition: deferred, reason: "each arm adds about a third to a run", trigger: "the owner answers Q3"}
- [-] codex: a Codex harness behind the harness port (`codex exec --json`) {id: BX-18, disposition: deferred, reason: "ADR-029 decision 6 keeps measurement Claude-only until that clause is amended", trigger: "the owner answers Q4 yes and ADR-029 is amended"}
- [-] (op) ctl: H2 agent half on F2's high fan-in stratum {id: BX-78, disposition: deferred, reason: "conditional on a tool-driven H4 lift", trigger: "BX-72 reads confirmed and A2 or A3m carries the lift"}
- [-] (op) ctl: H5 agent half {id: BX-79, disposition: deferred, reason: "runs only if the free reading leads its baselines", trigger: "BX-67 reads fathom's call-site recall above grep and go.work and gopls"}
- [-] H6, rework avoided {id: BX-80, disposition: refused, reason: "two negative priors; the feature needs rework before a measurement"}
- [-] H7, the claims in a final answer are true {id: BX-81, disposition: deferred, reason: "its grader needs claim extraction: an LLM-judge risk on a primary metric", trigger: "a deterministic claim extractor exists"}
- [-] codex: Java and Rust subjects, 3 each {id: BX-82, disposition: deferred, reason: "P3 languages", trigger: "BX-M1 reads PASS and Python and TypeScript reach their bank targets"}
- [ ] (op) ctl: P3 exit: the held-out readings published either way {id: BX-M3, oracle: lexical, path: docs/bench/readings/bench-prep/p3-exit.md, pattern: "^BX-M3 verdict:", needs: "BX-75,BX-76"}
