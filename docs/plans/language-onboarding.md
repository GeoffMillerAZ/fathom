# Language onboarding: replay before implementation

Proposal for issue #16. Awaiting maintainer review of the
[design](../design/language-onboarding.md),
[assessment](../assessments/2026-10-25-language-onboarding.md), and
[case inventory](../assessments/data/2026-10-25-language-onboarding-cases.json).
No code is authorized until that review. Every row remains open in this lane.
Stable LH task IDs are plan contracts, not allocated ADR/F/I/O record IDs.

Baseline: 743a914307baa286ccc5a6d8b92069910617b706

## Evidence contracts

- [ ] Publish the reviewed replay-input freeze marker with each retained bad/control source and exact input/tool-tree hashes {id: LH-1, oracle: lexical, path: docs/bench/readings/language-onboarding-replay.json, pattern: "\"inputs_frozen\": true", accept: "The controller reviews each input identity and at least one repaired control per counted catch; unresolved cases are unmeasured"}
- [ ] Record the baseline arm's three replay trials in the experiment ledger {id: LH-2, oracle: ledger, path: docs/bench/language-onboarding-replay.jsonl, run_id: language-onboarding-replay-v1, task: prerequisite-checks, arm: baseline, min_trials: 3, accept_verdict: any, needs: LH-1}
- [ ] Record the reordered-existing-check arm's three replay trials in the experiment ledger {id: LH-3, oracle: ledger, path: docs/bench/language-onboarding-replay.jsonl, run_id: language-onboarding-replay-v1, task: prerequisite-checks, arm: reordered, min_trials: 3, accept_verdict: any, needs: LH-2}
- [ ] Record the new-predicate arm's three replay trials in the experiment ledger {id: LH-4, oracle: ledger, path: docs/bench/language-onboarding-replay.jsonl, run_id: language-onboarding-replay-v1, task: prerequisite-checks, arm: new-predicates, min_trials: 3, accept_verdict: any, needs: LH-3}
- [ ] Publish the experiment outcome marker in the design, including KILL when the bars fail {id: LH-5, oracle: lexical, path: docs/design/language-onboarding.md, pattern: "Outcome recorded: (KEEP|KILL)", needs: LH-4}

The lexical gates prove publication of a specific declaration, not its empirical
truth. The ledger gates prove matching recorded trial counts with no voided
instrument; `accept_verdict: any` deliberately permits a failed experiment to
close. They do not prove KEEP, time saved, or controls passed. The controller must
review the frozen identities and raw per-case results before claiming LH-1 or
accepting the LH-5 outcome. A skipped arm has no three-trial evidence: disposition
its row with the exact ceiling and trigger, rather than writing fabricated trials.

The report records all 20 source groups, the five candidate groups, the tested
subset, repaired controls per tested group, catches by arm/scope, false blockers,
three alternating paired cost trials, command exits, and missing inputs. Every
counted catch needs a repaired control. Missing controls make a case unmeasured.
Cases requiring repo-wide acceptance cannot count as per-item preflight catches.
The reviewed design fixes the KEEP bars; the result records scheduling and new
predicates separately. An instrument defect voids the affected result.

## No integration on a failed experiment

This plan stops at evidence and a reviewed KEEP/KILL result. A KEEP result gets a
separate reviewed implementation unit with exact product symbols, required
certification tests and scope, before any runtime integration. A KILL result
leaves the current harness in place and records rejected-for-now wins-if
conditions. No importer, new command, validator or workflow engine is presumed.

Confirmed stale guidance can be corrected as a separate reviewed documentation
unit, even after KILL. That unit must cover dependency states, scratch versus
committed writes, actual verify evidence, ADR-075 fragments, and a skill trigger
check with no paid model/API call. It must describe only implemented checks.

Before any claim, run:

```sh
fathom verify --service fathom --plan docs/plans/language-onboarding.md
```

This checkout is not an indexed service in the current session. These are
non-code evidence contracts and no daemon verification is claimed. All boxes
remain open. Runtime work, if later justified, requires failing tests first and
race tests on touched packages. This branch does not push or allocate record IDs.
