# 0009: Transfer reviewed lessons through receiver-owned checks

**Status:** accepted
**Type:** architecture
**Date:** 2026-09-13

## Context

G6/G8's synthetic sourced-prose scenario now passes the integrated `task validate`:
claim-level review and export, correction invalidation, reconstructed approval and
ambiguous-delivery reconciliation. Their declared exit tests are met, not production
adoption. Under 0004, G7 may occupy the released implementation slot alongside G5.

## Decision

Implement the bounded [lesson-transfer contract](../../designs/lesson-transfer.md)
as one experimental kit and a non-executing `kit-workflow lesson` front door. Reuse
the existing engineering and provider-boundary tests as independent receivers.

Versioned public-synthetic payload, reproducer and attestation files have explicit
byte/depth limits. Domain-separated raw-byte digests exclude attestations and derived
digest fields, avoiding circular self-hashing. Hashes bind bytes, not author identity.
Receiver-owned review acceptance, reproduction, current-snapshot and revocation
callbacks are mandatory for a reproduced adoption receipt. Copy values across callback
boundaries and recheck freshness; callback errors and unknown evidence cannot pass.

The initial transfer links the kit's engineering coverage defect to resume_as_code's
unknown-provider-family failure. Both receivers seed a regression and use independent
local oracles. This is retrospective extraction, not a claim that sharing caused the
already-made repairs. Imported labels, files and receipts grant no execution authority.

No shared memory store, command runner, signature service, synchronization daemon,
automatic remediation or new dependency. Local persistence and trust remain caller
owned. CLI assertions are pending receiver claims, never independently reproduced.

## Review

Owner: kit maintainer. Review by 2026-10-12 or next release, whichever comes first.
Retain only with two-repository seeded-regression and revocation evidence; otherwise
revise or park under 0004. Compatibility remains experimental; no tier promotion.

## Revisit when

- A receiver cannot express its applicability or safely bound reproduction.
- A harmful transfer, forged acceptance or revocation race defeats these checks.
- Two consumers require authenticated distribution or durable revocation delivery.
