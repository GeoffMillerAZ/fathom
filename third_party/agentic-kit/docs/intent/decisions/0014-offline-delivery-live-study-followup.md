# 0014: Finish operational delivery without treating deferred live trials as evidence

**Status:** accepted
**Type:** process
**Date:** 2026-09-21
**Extends:** 0013's delivery scope; its evidence and authority rules remain binding.

## Context

The owner explicitly requests implementation and offline completion now, with the
remaining paid study documented as a TODO for later testing and optimization.
The earlier campaign retained a bounded classifier study, failed provider calls and
unknown-charge reservations. Cross-provider confirmation and live red-team execution
were not completed. Provider access, approval and evidence validity have since expired.

## Decision

- Complete consumer integration, evidence reuse, reconstructable decisions, independent
  controls, offline negative tests, documentation, review and clean-checkout verification.
  These remain required work, not deferred under the study exception.
- Defer only further live model calls, cross-provider comparisons and empirical
  optimization/confirmation. No provider calls, metadata probes or credential access are
  needed for this delivery. Preserve existing campaign accounting; do not renew approval,
  reset the ledger, rewrite failed runs or refresh evidence timestamps.
- Retain the historical study and label its limitations, failures and expired validity.
  A small challenge-set result is not production safety, current model quality or a
  universal architecture ranking. No production defaults or selection change.
- The consumer owns an executable follow-up specification at
  `resume_as_code/docs/evaluation/live-study-todo.md`: authority prerequisites, immutable
  inputs, budgets, stages, hard thresholds, graders, comparisons, failure accounting,
  artifacts, stopping rules and acceptance. Link it from the completion ledger.
- Delivery may be called implementation-complete only when the amended ledger's
  required offline exits pass. Deferred live coverage stays explicitly unexecuted;
  it is neither a passing check nor a waiver of any repository gate.

## Rationale

Paying for more samples is unnecessary to repair deterministic software and evidence
plumbing. It is necessary to support new empirical claims. Separate those obligations
instead of blocking useful implementation or manufacturing benchmark confidence.

## Revisit when

- The owner authorizes a new live campaign or explicitly renews the original one with
  its retained spend, current deployments, expiry and public/synthetic data scope.
- A consumer proposes production screening/routing, or a model/prompt/tool/corpus change
  makes a quality or performance claim depend on fresh measurements.
- The 0011–0013 review date (2026-10-16) or next release: inspect the TODO and determine
  whether to run, revise or explicitly retain the deferral. No automatic promotion.
