# 0023: Catalog Kit composes existing foundations and preserves domain laws

**Status:** accepted
**Type:** architecture
**Date:** 2026-10-20
**Request:** owner-directed consolidation of repeated CUE catalogs, hierarchical
composition, team/project tailoring, decision-model support and deterministic tooling.

## Context

The existing catalog foundation already owns hierarchy, facets, search, rendering and
drift; the decision foundation already owns typed questions and optional provider
adapters. Review Kit has stricter semantic floors and pinned-source synchronization.
Env Kit uses contradiction-refusing CUE unification. A second implementation of these
foundations, or an unconditional substitution of one kit's merge law for another,
would increase duplication or weaken behavior.

The current owner request reopens the v1 no-federation boundary in
`docs/designs/catalog-kit.md` for a concrete general/team/project composition workflow.
It does not reopen the no-network/no-server boundary of the catalog foundation.

## Decision

1. Put the experimental workflow under `kits/catalog-kit`, in agentic-kit. Compose
   `foundations/knowledge/catalog` and `foundations/decision`; preserve their APIs and
   legacy hash semantics. Do not create another decision port or remote provider client.
2. Resolve selected, already-loaded catalog layers through an explicit dependency DAG.
   CUE validates authored contracts. Go applies explicit transformations, bounds and
   preconditions. Replacement is never described as CUE unification.
3. Receiver-owned field grants, exact target digests and authored reasons govern
   mechanical overrides. Imported records cannot grant themselves authority. Consumers
   retain final CUE validation, semantic floors, exceptions, admission and effects.
4. Keep criterion definitions, captured evidence, model observations and policy outcomes
   distinct. Prepare bounded requests only against exact catalog and evidence versions.
   Preparation binds the exact complete domain-definition bytes, not just a version label.
   It does not enable a provider, spend money, close a rule or authorize an effect.
5. Keep analytics exports derived. Parquet support is an optional external format
   adapter; the root Go module remains stdlib-only. Full bundles retain the audit record.
6. Receiver-owned named profiles select root layers plus their dependency closure.
   They never filter grants or redefine domain laws. Provide stable CLI search,
   diagnostics and explain output for both humans and agents.
7. Migrate consumers by differential tests against real pinned catalogs and refusal
   cases. Local fixtures establish implementation behavior, not adoption or model quality.

## Decision models

Site: a consumer's atomic semantic criterion over a captured passage. Shape:
predicate, choice or score through the existing decision port. Default/fallback:
no provider; deterministic checks and the existing review path continue. A missing
required witness prevents request preparation. Choice includes unknown. Reuse binds
question/evidence/catalog/provider/model/adapter/policy versions.

The lane earns use only through consumer-specific holdout evaluation against the
deterministic baseline and current reviewer, including missed-defect audits, coverage,
abstention, cost and operator time. A measured failure to meet the consumer's accepted
margin disables the lane without disabling catalog resolution. No universal threshold
or transfer of calibration across providers is introduced.

## Consequences and limits

The new workflow is experimental until real consumers establish compatibility and
adoption. Review Kit and Env Kit are named migration targets, not completed migrations.
Source authentication, signed distribution, lifecycle/alias protocols, domain outcome
aggregation and live model quality remain separate work. The old design remains the
record for the foundation's original scope; this decision extends composition above it.

## Revisit when

- A real consumer cannot preserve its semantic floor or unification law through this seam.
- Two external consumers require an independent module/release boundary.
- Measured catalog sizes exceed the documented operational limits or query budgets.
- Native provider behavior changes enough to require a decision-port or confidence migration.
- The workflow reaches its next release without a completed real-consumer pilot.

Assessment: `docs/assessments/2026-10-20-catalog-kit.md`.
Specification: `docs/designs/catalog-workflow.md` (owner-directed uplift, 2026-10-21).
Work order: `docs/plans/catalog-kit-uplift.md`.
