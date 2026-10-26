# 0036: Assignments compose goals, evidence and stopping rules

Status: accepted
Type: architecture
Date: 2026-10-23

Request: "whatever you decide on these kits fully spec out and implement to completion"

## Context

The owner asks for catalogable assignments with goals, stretch goals, concerns,
confidence bars, pivot/give-up criteria and evidence of completion. Project Intent
already owns durable repo goals and conditions. 0021 rejects another repository
record kind. Catalog Kit owns definitions/identity, not assignment execution.
AssessProfile already handles required evidence, unknowns, freshness and soft scores.
The missing composition is a scoped assignment agreement and pure stopping/readiness
assessment. No condition justifies a new universal requirements or policy engine.

## Decision

Implement one experimental `kits/assignment-kit` under [AS-1..8](../../designs/assignment-kit.md).
Reference durable intent and reusable catalog definitions; reuse AssessProfile.
Required goals are conjunctive, stretch goals optional, scopes independent and pins
exact. Concerns, finite budgets/deadline, calibrated-confidence bindings and explicit
escalate/pivot/stop outcomes prevent a partial result from silently becoming success.

Complete this library, two runnable offline reference workflows, tests and adoption
guidance. No executor, new record store, provider, UI, daemon or installation is implied.
The application retains authentication, authority, evidence truth and real enforcement.

Amend W1's existing second reference slot (0032/0033/0035) for this owner-sponsored
assignment composition. Keep the same maintainer, five engineering days, zero paid
calls, <=64 MiB reference corpus and review 2026-11-21 or next release. This is not a
third active incubation or a promotion of older parked runtime work. The completion
claim is AS-A01..10, not whole G9/G11/G14 achievement or production adoption.

## Rejected alternatives

- Three separate goals/requirements/intent kits: duplicate identity/status ownership.
  Wins if: two consumers demonstrate independent laws the single contract cannot preserve.
- Put assignment state in Catalog Kit: confuses definitions with observed outcomes.
  Wins if: never; a read-only catalog projection of assignment templates is valid.
- Another mandatory intent file or goal-progress ledger: conflicts with 0021 and
  existing plans/TRACKING. Wins if: 0021's measured revisit conditions fire and are reviewed.
- Let self-reported confidence or a model's "done" close a task: cannot establish
  evidence, calibration or authority. Wins if: never as the sole completion proof.
- Build a workflow engine, permission service or automatic pivot executor here:
  duplicates existing owners. Wins if: two adopted consumers prove the missing seam.

## Revisit when

A planted omission completes; stretch blocks required success; exhausted work continues
through a claimed enforcement path; confidence is mistaken for truth; a consumer needs
shared lifecycle/grant semantics; W1 exceeds its budget; or its review date arrives.
Park further expansion without new evidence. Existing tiers and gates remain intact.
