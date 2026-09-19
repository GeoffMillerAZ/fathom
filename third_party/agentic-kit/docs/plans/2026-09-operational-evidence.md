# Operational evaluation and selection

Status: in progress, 2026-09-16. Owner: kit maintainer; agents execute the owner's
bounded completion objective. Baseline: `3b5b3c8` (0011/0012 offline slices).
This is the single requirement/evidence ledger; open items are not deferred scope.

## Contract and authority

Turn the existing benchmark and assessment APIs into a consumer-used evaluation,
eligibility, decision and reevaluation path. Preserve production defaults and consumer
authority. No universal score, parallel registry/router/evaluator/store, new root Go
dependency, unrelated consumer repair, gate weakening, release, tag or main merge.
Existing owners and methodology: [benchmark design](../designs/benchmark-evidence.md),
[assessments](../designs/engineering-assessments.md), decisions 0011/0012.
Necessary opt-in changes to resume_as_code/throughline are explicitly authorized by
the new objective; their local rules still apply. Prior dirty files remain untouched.

## Requirements → gaps → proof

- [ ] **OE-01 Consumer collection.** Existing RAC overlay exercises real red-team
  code, but not normal adoption. Add an opt-in normal-path collector/dependency upgrade;
  bind resolved configuration, planned tasks and every leaf attempt to retained receipts.
  Proof: actual CLI/integration tests, missing/duplicate-call negatives, failed/canceled
  work accounted, consumer revision and executed opt-in run. Missing costs stay unknown.
- [ ] **OE-02 Eligibility and decisions.** Catalog, ladder and failover already exist.
  Add consumer-owned approved deployments and deterministic hard-before-soft policy;
  separate claims/access/measurement, with bounded retry/failover/escalate/retrieve/
  review/reconcile/stop actions. Proof: reconstructed rule records, expired/unauthorized/
  unavailable candidates denied, no automatic production switch, outcome linked to decision.
- [ ] **OE-03 Identity and change.** Component fingerprints and strict comparisons
  exist. Materialize defaults; retain referenced artifacts; add dependency-aware proposals
  for exact deterministic reuse, regrading, price-only recalculation, affected reruns and
  fresh live measurements. Proof: dependency mutation table, alias/remote freshness and
  missing-artifact negatives; reuse never increments independent observations.
- [ ] **OE-04 Screening integration.** PromptScreener exists. Compose eligible client,
  admission, budget, cancellation, versioned refusal policy and independent effect/egress/
  output guards on the consumer path. Proof: malformed/error/uncertain holds, forbidden
  effects and canary disclosure absent, removed-control negatives and benign controls;
  races/speculative execution tested where present, otherwise explicit non-applicability.
- [ ] **OE-05 Evaluation protocol and labels.** Existing fixtures prove plumbing only.
  Freeze representative strata, label provenance, development/held-out split, graders,
  hard thresholds, practical margins, comparisons and budget/stopping rules before results.
  Proof: retained protocol/data/label digests, grader counterexamples and disagreement
  records, no holdout tuning or fabricated human labels/independent trials.
- [ ] **OE-06 Measured decision.** Run eligible simple/no-model and candidate solutions
  through staged pilot/comparison/confirmation. Proof: actual live receipts; all-task
  success and latency distributions/usefulness timing, attempts, complete-or-unknown costs,
  false allows/refusals, review/error rates and actual effect outcomes. A supported retain,
  reject or inconclusive decision is valid; unexecuted live work is not completion.
- [ ] **OE-07 Retrieval composability.** Reuse a named consumer retrieval need and
  existing metrics. Add bounded no-retrieval/normal/oracle-context diagnostics that
  distinguish retrieval, context assembly and generation failure, with permission,
  stale/citation/absence cases. Proof: a second domain's artifacts reconstruct through
  the shared assessment lifecycle; no speculative retrieval engine.
- [ ] **OE-08 Provider coverage.** Exercise supported Claude/OpenAI request/response,
  usage, failure and cancellation contracts with hermetic tests. Proof: explicit surface/
  version matrix; API access, CLI entitlement and native-host enforcement remain distinct.
  Unsupported or unobserved native behavior cannot be reported passing.
- [ ] **OE-09 Delivery.** Update intent/decisions and consumer instructions, compatibility,
  tracking and changelog with implementation. Resolve independent security/correctness
  review; run focused race/CUE tests, full required gates and clean-checkout verification.
  Record affected consumer revisions and baseline failures; conventional commits carry
  `Verified:`. Run Fathom verification before checking plan items; index proof supplements,
  never replaces runtime evidence. No unapproved merge/release.

## Execution and token economics

One main integrator; bounded scouts/builders/reviewers, typically two or three workers.
Read routed sources once; short path/symbol/test reports, no full-history worker forks.
Separate worktrees for writers and disjoint ownership; batch coherent implementation
units, focused tests during edits, full gates at required commit checkpoints.
Reuse prior primary-source research; fetch only unresolved/version-sensitive questions.
Retain exact artifacts/grades where valid; no made-up savings or sample multiplication.

## Current evidence / blockers

- Initial inspection: kit HEAD `3b5b3c8`; baseline implementation unchanged. Owner's
  separate intent-distribution, Taskfile and workspace edits are dirty and excluded.
- 0011/0012 promotion remains unproven; this objective advances their consumer work,
  not every future catalog. Record the narrower scope evolution before implementation.
- Prior $10 authorization covered another synthetic host exercise; retained report shows
  $0.0604956 then spent and explicitly requires fresh authorization for reruns. No new
  live calls authorized by possessing credentials. Owner approved a new, separate $5
  total campaign ceiling on 2026-09-16, public/synthetic cases only and approved available
  deployments. All screening/reviewer/retry/failed calls count; unknown charges retain
  reservations. No private résumé data or assumed human-reviewed labels enter the study.
  The new allowance begins unspent; prior host-study spend is not charged to or reused
  as this allowance. Persist cumulative expenditure before any paid dispatch.
- Known prior downstream failures: Fathom schema-size ceiling; Growing Gardens missing
  private financial fixture. Recheck unchanged baselines; neither waives a merge gate.
- Current operational checkpoint: kit reuse commit `e115d0b`; RAC topic branch has the
  normal-vendor collector, persistent campaign reservations, eligibility/recovery rules,
  private create-once artifacts and `evaluate` CLI implemented but not yet committed.
  Independent review repairs include approval-bound API factories, fresh request clones
  preserving absent provider fields, and failure-inclusive cost denominators.
- Actual public/synthetic pilots: 48 timed no-model held-out baseline trials; OpenAI
  generation reports quota exhaustion despite successful metadata access. Anthropic
  generation succeeds after a request-cloning defect was fixed, but four development
  responses contain fenced JSON and are correctly held by the strict classifier parser.
  Failed runs remain retained. As of this checkpoint, usage-computed cost is $0.004255
  and unknown-call reservations are $0.395 against the same $5 ceiling; neither is an
  invoice. No model-heldout or live red-team result has been observed yet.
- Opt-in `PromptScreenConfig.StructuredOutputs` now derives a closed schema before
  fingerprinting; focused race/CUE tests and `task validate` pass. This is a new candidate,
  requiring a protocol amendment before evaluation, not rewritten pilot evidence.
- Owner approved exactly the two official provider API hosts for RAC's source-domain
  gate; exact/lookalike and existing forbidden-source negative tests pass. Other gates
  are unchanged. Rechecked downstream commands currently stop earlier on unrelated
  dirty-workspace/dependency state: Fathom temporary embedded CSS missing; Throughline
  readonly module drift; Growing Gardens missing transitive `go.sum` entry. These are
  non-passes, not evidence of compatibility or a reason to weaken checks.

## Completion audit

For each OE row, retain implementation entrypoint, exact test/command and outcome,
revision, artifact references and limits. Final report separates implemented/integrated/
adopted/measured. Required incomplete or blocked rows keep the overall goal open.
