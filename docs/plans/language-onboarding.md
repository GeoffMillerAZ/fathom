# Language onboarding: feasible replay before implementation

Q1 revision proposal for issue #16. Review the [design](../design/language-onboarding.md),
[assessment](../assessments/2026-10-25-language-onboarding.md),
[mechanical census](../assessments/data/2026-10-25-language-onboarding-mechanical.json), and
[v2 criterion](../assessments/data/2026-10-25-language-onboarding-criteria.json)
before any scorer or runtime predicate is written. Every row stays open in this lane.

Baseline: 743a914307baa286ccc5a6d8b92069910617b706

## Evidence contracts

- [ ] Publish the reviewed v2 freeze declaration for input/index/checker hashes, exact command arrays, all 19 explicitly cleaned controls and scoring criteria {id: LH-1, oracle: lexical, path: docs/bench/readings/language-onboarding-replay.json, pattern: "\"inputs_frozen\": true", accept: "Freeze public source pairs and isolated defect hunks before code or replay; changes require a new criterion version"}
- [ ] Record B's computed validity evaluation and three paired timing samples per required control in the v2 ledger {id: LH-2, oracle: ledger, path: docs/bench/language-onboarding-replay.jsonl, run_id: language-onboarding-replay-v2, task: submission-hygiene, arm: baseline, min_trials: 3, accept_verdict: any, needs: LH-1}
- [ ] Publish the pre-oracle scheduling feasibility decision with its supported ceiling and revival condition, without inventing replay trials {id: LH-3, oracle: lexical, path: docs/design/language-onboarding.md, pattern: "eligible ceiling is 1 defect in 1 addition, below the unchanged 3-defect/2-addition bar"}
- [ ] Record N's independently computed KEEP/KILL evaluation, case/control counts and three paired timing samples per required control in the v2 ledger {id: LH-4, oracle: ledger, path: docs/bench/language-onboarding-replay.jsonl, run_id: language-onboarding-replay-v2, task: submission-hygiene, arm: new-predicates, min_trials: 3, accept_verdict: any, needs: LH-2}
- [ ] Publish N's scorer-rendered outcome count line, preserving a failed result and its conditions rather than typing an outcome word {id: LH-5, oracle: lexical, path: docs/design/language-onboarding.md, pattern: "Outcome recorded: submission-hygiene (KEEP|KILL) \\(incremental catches [0-9]+, additions [0-9]+, false blockers [0-9]+, controls [0-9]+\\)", needs: LH-4}

LH-3 is independent of B/N: the pre-oracle experiment is not scheduled because
its supported ceiling is insufficient. It cannot block N or acquire a fictitious
three-trial row. N is a separately scoped pre-submission hypothesis, not S plus
new predicates. E2 harness-rule changes and shared integration defects do not
supply its per-addition catch counts.

The post-approval replay scorer must compute `evaluation.{criterion_id,verdict,reason}`
for B and N from frozen criteria and receipts. The v2 criterion file already
records S's arithmetic feasibility result, without treatment timings. The scorer
also emits report/ledger digests and renders LH-5's count line. Existing scenario
bench criteria do not implement these bars, and the ledger oracle does not
recalculate them. The controller reviews scorer tests and receipt arithmetic.

`accept_verdict: any` permits a measured failed experiment to close; it does not
permit KEEP after failure. Missing input, omitted control or invalid baseline is
`indeterminate`/NOT_MEASURED. The scorer emits only a diagnostic report and no claimable ledger row for such an arm,
even when timing alone completed three pairs. The generic ledger oracle would
otherwise accept indeterminate with `accept_verdict: any`; the scorer must not
write that shape. Disposition its row with the precise ceiling and trigger if operationally
blocked. Do not fabricate trials or a zero false-blocker count. A missing
applicable checker is not a pass.

The 10-second gate applies to every control's median of three paired N-minus-B
wall differences, measured from fresh serial pre-commit process entry to all
selected checks returning, before any commit object. Oracle work, downloads,
tool builds and warmup are excluded. The real hook is parallel; this experiment
makes only a conservative serial-overhead claim. Minimum trials is the thinnest
control's paired count, never a subprocess total.

## Integration and guidance remain separate

Any KEEP requires a separately reviewed implementation contract for the matching
pre-submission check seam. There is no approved `runLangLoop` pre-oracle stage.
A failed hygiene experiment records the null and removes experimental checks.
Confirmed stale guidance may be corrected in a separate reviewed unit covering
dependencies, guarded writes, actual verify evidence, ADR-075 fragments, and a
skill trigger check without a paid API. No importer or workflow engine is presumed.

Before any claim:

```sh
fathom verify --service fathom --plan docs/plans/language-onboarding.md
```

This checkout is not an indexed service in the current session. No daemon
verification is claimed, no box flips here, and no code or runtime behavior is
changed by this revision. LH task IDs allocate no ADR/F/I/O record identifiers.
