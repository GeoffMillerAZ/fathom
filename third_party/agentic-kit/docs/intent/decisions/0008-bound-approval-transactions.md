# 0008: Bind durable approvals inside the existing HITL transaction

**Status:** accepted
**Type:** architecture
**Date:** 2026-09-13

## Decision

Extend `kits/hitl`, not the effect taxonomy or harness loop, with a trusted
transaction store and authenticated approval binding. Bind actor, operation digest,
resource/state version, evidence/config digests, expiry and revocation. Verify and
consume the stored decision together with the pure local application and receipt.
The memory reference serializes for reconstruction but is not durable storage.

Prepare external delivery durably before I/O. One decision permits at most one
send; unresolved delivery blocks further effects until destination reconciliation.
An idempotency key is a destination contract, not a generic exactly-once claim.
Prepare export bytes before requesting approval; prose authorization callbacks
do not consume decisions or perform publication.

## Why now and boundaries

The owner authorized implementation of the governed-workflow design, whose G6/G8
requires approval across interruption. This deliberately extends 0002's rejection
of a separate approval token: decisions live in the existing HITL composition and
are looked up in trusted storage, never accepted as model-supplied bearer authority.
The synchronous harness `Approver` and all existing defaults remain unchanged.
No distributed authorization service, automatic policy propagation or new dependencies.

## Alternatives and evidence

Conversation checkpoints do not authorize effects. Signed bearer artifacts add key
management without coordinating the target transaction. Caller-owned transactional
storage supplies that boundary; JSON hashes alone never do.
Contract and executable cases: [approval/commit](../../designs/approval-commit.md).

## Revisit when

A consumer cannot atomically coordinate target state with its decision store; two
consumers need shared authorization; or a destination cannot reconcile an ambiguous
outcome. Refuse that path until its actual authority/delivery boundary is specified.
