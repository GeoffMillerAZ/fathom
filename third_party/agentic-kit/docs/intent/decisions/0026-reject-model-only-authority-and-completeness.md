# 0026: Reject model-only authority and completeness decisions

Status: rejected
Type: security
Date: 2026-10-22

Request (paraphrase, owner discussion 2026-10-22): consider decision models as
security/prompt bouncers and as a way to decide which prose needs regeneration.
The rejected alternative is making those judgments the sole safety or
completeness control. The useful decision-assistance applications are accepted.

## Context

A valid typed answer or high confidence does not establish permission, semantic
truth, completeness or calibrated risk. Existing 0017/0018 already require
site-specific calibration and provider-optional behavior. Removing a dependency
because a judge missed it can publish a stale summary; allowing a tool because
a screen labels text safe can bypass deterministic authorization.

## Decision

Reject a model result as the sole authorizer, exhaustive conflict detector,
publication permit, or oracle that can shrink a declared dependency closure.
Wins if: never as the only control over a required deterministic property.

Use models for bounded classification, prioritization, suggested support links,
additional dependency candidates and uncertainty detection. An unfavorable or
uncertain answer may hold work for review; a favorable answer cannot lift an
authority, evidence, freshness, data-disclosure or budget floor. Named controlled
entrypoints must enforce those floors independently of the model.

Catalog thresholds with their site, question/definition/model versions, labels,
calibration scope, error tradeoffs, expiry, owner and fallback. A native
probability, an evidence grade and a human confidence assessment remain different
values. Explanations cite evidence and rule traces, not hidden chain of thought.

## Consequences

Prompt screens remain defense in depth. Unknown, refused, malformed, timed-out
and unsupported answers retain distinct causes. Cheap-first escalation never
routes a prohibited action to a more permissive provider. Deterministic lineage
invalidates known dependents; semantic checks may widen the affected set and
hold publication when coverage is unknown.

## Revisit when

A site has calibrated new evidence: reconsider its routing/abstention policy,
not this authority boundary. Reopen the boundary only with an independently
enforced proof system for the exact property, not a better model confidence score.
