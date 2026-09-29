# 0007: Intent follows ownership; its practice is a pinned dependency

**Status:** accepted
**Type:** process
**Date:** 2026-09-13

## Context

Platform-engineering has multiple independently useful kits and root-only review-kit
intent. The owner requested a component product charter and reusable monorepo guidance.
Routing by cwd can miss component intent; a global checkout-following skill install is
not a reproducible team dependency. Decision 0005 already separates instructions from
tested workflow enforcement.

## Decision

- Root owns repository membership, shared constraints, integration and delegated
  authority. A product with its own audience, cadence or roadmap owns local intent.
  A package or vendored dependency does not automatically become a product.
- Resolve affected paths, load applicable ancestor constraints once and then the
  nearest owning router. Cross-boundary work consults both owners. Existing ADR IDs,
  homes and append-only rules survive adoption; local records never silently override
  parent constraints. Sibling-specific decisions do not bind unrelated siblings.
- Share authority, incidents and change records by pointers unless a genuinely
  independent ownership/release boundary needs its own home. Goals belong to one
  owner; root goals link integration outcomes, not copies of product roadmaps.
- Prefer reviewed, project-local snapshots of practice/skills/templates with an
  upstream base commit and exact digests. A modified source checkout is explicitly a
  working-tree snapshot, not a fabricated tag. Same bytes serve both hosts; upgrades
  are deliberate diffs. The existing personal installer stays available by opt-in.
- Deterministic budget/link/index/stanza/integrity checks complement, never replace,
  evidence-backed audits. Each check states its coverage. Installation, parse success
  and documentation review do not establish live host routing or enforcement.
- Preserve explicit accepted future design without asserting that code implements it.
  State current evidence and missing enforcement; use testable exits for admitted work.

## Consequences

The practice works in standalone repos and monorepos without duplicating policy or
autoloading whole documentation trees. A small offline snapshot updater is maintained
with the init skill for the named platform-engineering adopter; no runtime dependency,
global installation, remote fetch or new mandatory gate is introduced.

## Alternatives rejected

- One monorepo-wide product vision: hides independently useful products and owners.
- Full template trees per directory: duplicates authority and inflates context.
- Mutable global symlinks as the team pin: updates silently and depends on one machine.
- A new central intent service: files and reviewed manifests meet the current need.

## Revisit when

- A root-started or cross-component task misses its governing router despite the route.
- A consumer cannot pin/update the practice without local edits or a global install.
- A component gains a genuinely independent release or authorization boundary.
- Host upgrades change native instruction loading, discovery or precedence.
