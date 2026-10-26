# 0027: Human workbench and prose share bound transactions

Status: accepted
Type: architecture
Date: 2026-10-22

Request: "we should also build a human in the loop workbench kit."

## Context

The existing HITL committer, source-linked prose, token records and cueform are
useful substrates, not a complete workbench. app-kit's approval primitive
presents pending/review context; it grants neither permission nor execution.
0008 already binds approval to exact actor/operation/state. The new workbench
must work for both human-reviewed changes and explicitly authorized autonomy.

## Decision

- Use one proposal/validation/authorization/commit/reconciliation protocol for
  human and agent clients. Policy chooses execution mode. Presentation cannot
  bypass the server's effect gate, and a suggestion is not consent.
- Separate resolving facts, annotating presentation, approving effects, overriding
  a specific overrideable rule, reidentifying tokens and exporting artifacts.
  RBAC roles combine with target, purpose, clearance and current policy checks.
- Canonical prose inputs are sources, claims, annotations, structure and policies.
  Generated versions are immutable projections. Editing output creates a tracked
  proposed input/annotation or a distinct human-authored artifact, never silent drift.
- Regenerate the known dependency closure, including summaries and variants.
  Broader semantic impact can require more review; isolated section regeneration
  cannot be promised when other sections depend on the change.
- Agentic-kit owns portable contracts and deterministic transaction behavior;
  app-kit owns reusable visual recipes/accessibility; apps retain authentication,
  persistence, domain semantics, actual effects and destination reconciliation.
- Keep simple adoption possible: an existing form and evidence panel can use the
  protocol before a full workbench exists. HITLMAN remains a working product name,
  not a new mandatory runtime, package tree or second approval engine.

Acceptance: [human workbench](../../designs/agentic-engineering/human-workbench.md)
and [prose harness](../../designs/agentic-engineering/prose-harness.md).

## Alternatives

- Tokenization as authorization: a token can still leak associations or grant
  unintended access. Wins if: never without independent access control.
- All red findings can be overridden by an administrator: some policy floors are
  absolute. Wins if: never for a rule declared non-overrideable.
- Color-only warnings: excludes users and hides status in exports. Wins if: never.
- New shared editor/framework first: exceeds the proven gap. Wins if: a consumer
  pilot shows the existing app-kit/editor seams cannot satisfy the interaction.

## Revisit when

An actual destination cannot reconcile unknown outcomes, a consumer cannot retain
domain authority, the question burden exceeds measured benefit, or two consumers
demonstrate a missing reusable visual or transaction contract. No production or
visual/accessibility acceptance is inferred from this design.
