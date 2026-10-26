# 0028: Compositions bind snapshots and policy epochs

Status: accepted
Type: architecture
Date: 2026-10-22

Request: "is there anything I asked for that didn't get addressed or rejected
and recorded durably? let's also just give this design another excellence pass"

## Context

The first design mapped the request's topics but left material composition
contracts loose: commit/revoke races, delegated authority, complete generation
context, multi-output readiness, profile overrides and progress projections.
Independent completeness and adversarial reviews identify these gaps in the
[second-pass assessment](../../assessments/2026-10-22-agentic-engineering-design-review.md).
0024-0027 still govern ownership, model limits and immutable bound transactions.

## Decision

- Controlled local effects compare target state, approval consumption and trusted
  policy/evidence authorization epochs at one atomic commit point. If the adapter
  cannot supply that fence, it must hold the advertised strict operation.
- Remote dispatch has a last authorization boundary, then explicit unknown-result
  reconciliation. Revocation cannot retroactively undo an already dispatched effect.
- Identity includes trusted delegation and controlling principal; two credentials
  controlled by one actor do not satisfy a two-party approval requirement.
- Effective purpose comes from trusted host authority. Approval pins destination,
  disclosure scope and publish mode; equal bytes cannot transfer consent between
  audiences. Reconciliation never renews a consumed decision (0008).
- Provider-hosted tools require proved pre-effect interception or enforceable,
  bounded preauthorization before the outer call. Otherwise they are unsupported;
  a later tool trace cannot supply the missing boundary.
- Exposed generation/review context is a dependency, even if uncited. Pin a whole
  exposed snapshot when finer read tracking is unavailable; no semantic completeness
  guarantee is inferred from citations or a model judge.
- Simultaneous variants share an explicit input/policy cut and publish mode.
  Independent variant readiness and all-required group release differ; partial
  multi-destination delivery never implies atomic success.
- Default prose criteria, authorized profile overlays, catalog migrations and
  optional-off/waiver semantics are explicit data. Tightened floors invalidate active
  old judgments/approvals; historical bindings never become fresh by key matching.
- Goal/capability views join exact authoritative sources and receipts, displaying
  independent implementation/integration/adoption/measurement states. They cannot
  write progress or infer readiness from a checkbox or missing receipt.
  Expected capabilities use a scope-owner-authored manifest independent of TRACKING
  and receipts; disappearance from output cannot retire expected scope.
- Pages are private visual explanations linked to repository sources. They carry
  design status and do not become a competing specification or progress database.

The existing specifications and work order are amended. This is design-only:
no new runtime, public API, provider invocation, dependency or consumer migration.

## Alternatives

- Best-effort reads called atomic authorization: races remain possible. Wins if:
  an explicitly weaker advisory surface is all the consumer claims.
- Whole-prompt context omitted to improve cache hits: hides actual dependencies.
  Wins if: never for a claimed exact reuse or provenance boundary.
- Output group called atomic across arbitrary destinations: no shared transaction
  exists. Wins if: the destination supplies and proves that exact transaction.
- Pages as another editable progress authority: conflicts with intent/TRACKING.
  Wins if: a separately decided migration replaces the authority rather than forks it.

## Revisit when

A consumer cannot supply the declared fence or snapshot, a policy migration loses
domain law, source-view joins disagree, or attended trials show unnecessary
rebuild/review burden. Revisit the smallest contract; do not weaken its guarantee
silently. Exact per-site scenarios are in HW-15..20, EL-A15..17, PC-A13..16,
PH-19..27, HW-21..23, PC-A17, PV-17 and SH-14..17.
