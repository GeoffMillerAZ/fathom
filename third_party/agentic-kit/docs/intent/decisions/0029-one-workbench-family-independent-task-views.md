# 0029: One workbench family, independent task views

Status: accepted
Type: architecture
Date: 2026-10-22

## Context

The owner requests advanced backendless layouts, compact assistance through full
workspaces, and reusable prototype support contributed to app-kit. HITLMAN and
prose share evidence, selection, exact changes and recovery, but prose owns a
document graph and HITL owns effect authorization. 0024, 0027 and 0028 retain
independent kit authority, bound transactions and immutable input-derived prose.
The existing W3 implementation pilot remains queued. No runtime promotion or
measured-benefit trigger has fired. This admits a bounded design experiment.

## Decision

Use one workbench family with a shared context/selection/inspection vocabulary.
Review, Prose, Evidence, Runs, Catalog and Learning are task views, not mandatory
tabs in every consumer. An embedded task can use only the smallest useful view.
Inline assistance, side panel, split workspace and full workspace are presentations
of the selected subject. Expansion retains selection and local task state.

Keep semantic ownership separate. Prose supplies graph, source mapping, variants
and rebuild closure. HITL supplies proposals, authority, exact decisions and effect
receipts. The shell cannot implement or replace either authority. Consumers own
identity, storage, routes, effects and allowed capabilities. Shared UI stays optional.

Author original static synthetic prototypes in `prototypes/<slug>/`, with a v1
manifest, README, design questions, scenario inventory and review/promote/retire
conditions. They have no production imports, backend, provider, credential flow or
persistence. Label every presentation visibly. Simulated roles and approvals are
never security or transaction evidence. No screenshots/build products in Git.

Contribute generic create/check/preview tooling to app-kit, not agentic workflows.
Keep it experimental, standalone and free of a chosen product framework. Return
reusable observed defects; a prototype does not become a stable component by
changing status. The app and kit promotion gates still apply independently.

The [experience design](../../designs/agentic-engineering/workbench-experience.md)
owns layout questions, scenario coverage and observed proof. Existing HW/PH
acceptance and AE-8..11 runtime rows remain open.

## Alternatives

- Separate complete HITLMAN and prose shells: duplicate evidence and navigation.
  Wins if: attended tasks show incompatible selection or interaction needs.
- One universal editor with all capabilities always visible: excessive attention
  and coupling. Wins if: a named task requires that complete view and proves value.
- Promote the mock directly into a production application: simulated authority
  and synthetic data would be misleading. Wins if: never by prototype status alone.
- Keep temporary designs in untracked scratch: loses questions and decisions.
  Wins if: throwaway work has no durable design question or review value.

## Revisit when

Attended trials show increased review effort, an embedded task needs a different
selection model, shell reuse crosses an authority boundary, the prototype lacks
an owner at review, or two consumers prove a reusable visual/accessibility contract.
Change the smallest owning layer. Do not pre-build a general editor framework.
