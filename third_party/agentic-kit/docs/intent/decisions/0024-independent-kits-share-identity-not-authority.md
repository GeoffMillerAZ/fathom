# 0024: Independent kits share identity, not authority

Status: accepted
Type: architecture
Date: 2026-10-21

Request: "please continue to raise the excellence and intercoperation yet independence
of our kits. dev ex and capability and excellence are very high priorities. find
gaps and fill them and spc durably and implement until polished to excellence"

## Context

Catalog Kit has exact definition/evidence pins but adapters identify definitions by
stable keys while its selection/diff surfaces use path IDs. Ordinal pilot slugs
turn insertions into false identity churn. Current pilots project effective data
without proving all advertised composition seams. CLI flags can be silently
ignored, and kit-new can truncate files in an existing non-module destination.
These are concrete shipped-workflow gaps under 0023, not a new incubation.

## Decision

- Expose verified detached stable-key lookup and move-aware diffs in Catalog Kit.
  Keep exact path/digest-bound decision packets; moves never auto-refresh consent.
- Index identities for discovery and reject unused CLI configuration before I/O.
- Make scaffolding refuse existing outputs and exclusively create owned files.
- Prove actual domain composition and independent external-module adoption. Keep
  typed payloads, semantic floors, CUE unification and authority in their owners.
- Preserve recoverable drafts before the owner-requested workspace pruning.

Specification and acceptance: [kit-interoperability](../../designs/kit-interoperability.md).
No new public library package, provider, protocol, dependency, release tier or consumer mutation.

## Alternatives

- Universal kit adapter/lifecycle protocol: adds policy without a current shared
  contract. Wins if: two consumers demonstrate the same unsolved protocol boundary.
- Rename paths as delete/add: loses declared stable identity. Wins if: never for
  verified bundles whose stable keys are unchanged.
- Auto-rebind stale judgments after moves: confuses identity with exact evidence
  and authorization. Wins if: never without a separately reviewed reuse contract.

## Revisit when

A consumer cannot retain its typed domain law, independent release needs arise,
a stable key changes meaning in place, or metadata projection requires a shared
protocol that two consumers cannot implement through the existing boundaries.
