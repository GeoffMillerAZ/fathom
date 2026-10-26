# 0037: Inventory and evidence compose with domain-owned reviews

Status: accepted
Type: architecture
Date: 2026-10-23

Request: "we have catalog kit but should that have a sub kit called inventory kit
and should be have evidence kits and what else?" (owner, 2026-10-23).

## Context

Catalog Kit owns definitions, composition and identity. Platform Review Kit already
collects native repository inventory. Agentic Kit has evidence primitives,
Investigation Kit, assessments, assignments, HITL/prose and learning contracts.
Missing workflow guarantees include independent observed coverage, claim-specific
support, temporal/disclosure semantics and honest correction/invalidation.
Two owner-requested Astra consultations informed this decision; their advice is
not runtime evidence. [Sources and dispositions](../../research/2026-10-23-inventory-evidence-review.md).

## Decision

Accept the [IER contracts](../../designs/inventory-evidence-review.md) and
[EDR workflow specifications](../../designs/evidence-driven-reviews.md) as design.
Observed Inventory and Evidence Assessment start as consumer-local profiles and
adapters over native owners. Inventory is not a Catalog sub-kit with observation
authority. Catalog-backed views and existing scan-go remain valid projections.

After separate admission, first prove one bounded deterministic review dossier,
independent expected scope and claim-specific evidence use through a qualified
native Review Kit receiving route. Existing inventory/sound producers do not
prove third-party ingress; no cross-module internal-package import is allowed.
The [IR work order](../../plans/inventory-evidence-review.md) records later work.

Use existing Investigation Kit for bounded hypothesis work, Assignment Kit or an
equivalent explicit completion contract, domain Review Kit for applicability and
findings, HITL/prose for exact changes and learning contracts for reviewed feedback.
Separate review completion, subject readiness, evidence support, inference confidence,
causal explanation, incident likelihood and action authority.

A later sibling Inventory Kit or Evidence Assessment Kit requires two independent
owners demonstrating shared behavior, retained native law, removed duplicated glue
and measured adoption benefit. Neither extraction is admitted by naming it here.
Keep assurance arguments with the domain review owner; no universal evidence store,
score, risk ledger, workflow engine or incident predictor.

This refines G9..G14 design, including G12 human interaction and G13 source/prose
correction, preserves 0004/0006/0012/0024/0025/0036 and
F17/F19/F20, and opens no implementation slot. No runtime, new public types, source
migration, provider/scanner, hook installation or consumer/global mutation is shipped.

## Alternatives rejected

- Observed Inventory sub-kit under Catalog: confuses definitions and measured state.
  Wins if: only a catalog view/descriptor is needed; that already fits Catalog Kit.
- New public evidence/inventory kits immediately: their shared behavior is unproven.
  Wins if: independent receiver qualification and extraction criteria above are met.
- Universal attestation, grade or incident-probability engine: hides native limits.
  Wins if: never as a substitute for domain evidence and qualified numerical methods.
- Move platform inventories or findings into Agentic Kit: duplicates active owners.
  Wins if: an owner-approved migration proves benefit and preserves native contracts.
- Treat source topology as deployed truth or model confidence as incident likelihood.
  Wins if: never; operational and calibrated support must be supplied independently.

## Revisit when

A named receiver admits S1; two owners request the same missing seam; a planted
omission/citation/correction case produces a supported false pass; native ingress
cannot preserve domain law; qualified evidence establishes a numerical risk need;
or 2026-11-21/next release arrives. Without new evidence, keep implementation queued
and revise or retire the hypothesis. No API or compatibility tier changes occur.
