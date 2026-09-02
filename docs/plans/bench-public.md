# Bench public — truth at HEAD, method the field respects, a corpus a stranger can re-run

Design: `docs/vision/26-proof-credibility.md` §2. Decision: ADR-031. Extends
`bench-confidence.md` (BC-14/15/16 stay there, re-anchored by PB-7).

Runs spend real tokens and stay operator-initiated (autonomy: ask before LLM
spend). Everything else here is free to run and lands in `task validate`.
Statistics come from agentic-kit `foundations/quality/reliability-stats`
(the kit owns them: fathom hand-built paired statistics twice).

Verify: `fathom verify --service fathom --plan docs/plans/bench-public.md`.

## Truth at HEAD

- [x] Home corpus re-sweep revision: every ground truth re-derived at a named commit (the moved workspace rule, the inverted ADR-010 truth, the moved premises, the `29` substring hole); `machine_criterion` untouched; `TestHomeCorpusTruthAtHead` asserts each all-of token still appears in the file its derivation names, so corpus drift is a gate failure {id: PB-1, symbol: TestHomeCorpusTruthAtHead, package: internal/contextbench, skip_test: true, skip: "the test is the anchor"}
- [x] `docs/bench/ledger.jsonl` exists with one backfilled row from the in-tree 2026-07-24 artifact (`BackfillLedgerRow`, marked n=1 / indeterminate under the replication floor); ADR-028 and the CHANGELOG say "created by" rather than "committed" for rows that do not exist {id: PB-2, symbol: BackfillLedgerRow, package: internal/contextbench}
- [x] `LedgerRow` gains `per_task`, `corpus_sha256` (`CorpusSHA256`), `model_id`, `cost_usd`, `dirty_tree` — additive, omitempty, old rows parse (`LoadLedger`) {id: PB-3, symbol: LoadLedger, package: internal/contextbench}
- [x] Runner `--resume <run-dir>` merges completed cells (`MergeResumedResults`), counts `cells_lost`, and records the resolved model id and `total_cost_usd` from the envelope {id: PB-4, symbol: MergeResumedResults, package: internal/contextbench}
- [x] Runner subject parameterization: `--subject-root`, `--service`, `--fathom-url`, `--prompt-subject`, opt-in `--scratch-daemon` (ingests the subject into a store under `workspace/`, never the fleet store); the `memory_search` exclusion documented in `docs/bench/README.md` {id: PB-5, symbol: cmdTasks, package: cmd/contextbenchscore}
- [x] `TestCmdLedger_ExitCodes` pins 0 pass / 1 fail / 3 indeterminate {id: PB-6, symbol: TestCmdLedger_ExitCodes, package: cmd/contextbenchscore, skip_test: true, skip: "the test is the anchor"}
- [x] BC-14, BC-15, BC-16 in `bench-confidence.md` re-anchored to `TestLedgerHasReplicatedRow`, `RequiresSatisfied`, `cmdRegress` so the gate cannot pass them before the work exists {id: PB-7, symbol: cmdRegress, package: cmd/contextbenchscore, skip_test: true, skip: "plan edit; evidence is fathom verify --plan bench-confidence.md --require-all reporting BC-14 unproven (TestLedgerHasReplicatedRow is deliberately unwritten until the row exists) and BC-16 open behind needs: BC-14; BC-15 verifies on RequiresSatisfied and carries the PB-19 condition in prose"}

## Method

- [x] Fenced answer block + `max_predicted` (default `max(5, 2·|gold|)`); `ScoreWithPrecision` returns pass/fail plus precision and recall; test files are excluded from gold; registered as criterion H2-2 in the corpus beside H2-1, whose recorded verdicts stand (T5) {id: PB-8, symbol: ScoreWithPrecision, package: internal/contextbench}
- [x] A `none` arm (all tools disallowed) and a leak rule committed in `machine_criterion`: a task the none arm passes in ≥2 of 3 trials is `leaked` (`LeakedTasks`), excluded from the verdict and listed in the row {id: PB-9, symbol: LeakedTasks, package: internal/contextbench}
- [ ] `ComputeReliability` fills the row's `ci_success_diff` (seeded, task-clustered paired bootstrap 95% CI), `wilcoxon_p`, `a12`, `pass_at_1`, `pass_pow_3` from the kit's `reliability-stats`; reported, never gating {id: PB-10, symbol: ComputeReliability, package: internal/contextbench}
- [x] `FirstGoldHit` parses transcripts for tokens-to-first-gold-hit and tool-call counts per cell {id: PB-11, symbol: FirstGoldHit, package: internal/contextbench}
- [x] `RequiresSatisfied(task, env)`: a task may declare `requires: [compiled-layer]`; the home corpus is split so a deterministic subset runs without the LLM-compiled obligations layer {id: PB-12, symbol: RequiresSatisfied, package: internal/contextbench}

## Regression (BC-16)

- [x] `contextbenchscore regress -ledger [-baseline run_id]`: newest vs previous row with the same `corpus_sha256`; a task whose treatment success rate fell is flagged; flaky tasks listed not failed; added tasks are not regressions; exit 0/1/3; `task bench-regress` {id: PB-13, symbol: cmdRegress, package: cmd/contextbenchscore}

## The public corpus (BC-15)

- [x] `contextbenchscore import`: derives `testdata/contextbench/tasks-public-go.json` from two pinned sources — SWE-Explore's Go subset (line-level truth) and Multi-SWE-bench Go JSONL (CC0 gold patches) — with no LLM, under the pre-registered selection rule (gold patch touches 1–5 non-test files; sort by `instance_id`; first 30–40), emitting per-task `derivation`, `requires`, `source`, and `subject{repo_url, base_commit, service}`; each source's license recorded beside its pin {id: PB-14, symbol: cmdImport, package: cmd/contextbenchscore}
- [x] `DeriveLocalizationTasks`: `file_localization` (diff paths, all-of with cap) and `function_localization` (`EnclosingFuncsForHunks` via `go/parser` at `base_commit`) {id: PB-15, symbol: DeriveLocalizationTasks, package: internal/contextbench/publicloc}
- [x] `find_callers` truth from a committed `go/types` tool (`cmd/loctruth`), never from fathom's own index {id: PB-16, symbol: CallersTruth, package: cmd/loctruth}
- [x] `TestDerivationIsDeterministic`: the same pinned inputs produce the same `corpus_sha256` twice, and a shuffled input order produces the same corpus {id: PB-17, symbol: TestDerivationIsDeterministic, package: internal/contextbench/publicloc, skip_test: true, skip: "the test is the anchor"}
- [x] `docs/bench/public-loc.md`: the runbook (pin, derive, snapshot, run, score, publish transcripts), the cost statement (~35M tokens at n=3 across three arms), and the pre-registered criterion for run 1 {id: PB-18, skip_test: true, skip: "runbook; evidence is the file"}
- [ ] (op) First three-arm public run; ledger row with `corpus_sha256`; verdict honest either way {id: PB-19, symbol: TestLedgerHasPublicRow, package: internal/contextbench, skip_test: true, skip: "operator-run spend; the test asserts a public-corpus row exists and is replicated"}
- [-] Register the fathom-equipped harness as an explorer in SWE-Explore's Go subset {id: PB-20, disposition: deferred, reason: "a registry listing F2 forbids until promoted; needs the binary in a stranger's hands (Q4)", trigger: "F2 promoted or the owner ratifies a release"}
