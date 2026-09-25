# ADR-0061: Retain complete historical capture sets within explicit bounds

- **Status:** Accepted
- **Type:** architecture
- **Date:** 2026-09-15
- **Extends:** ADR-0042/0043/0055; no certification or source-access change

## Context

P09's retained original zoom run has34 images; its runtime-repair zoom run has52.
The existing32-slot/result bound refuses either complete run. The largest needs54
slots including its report and future full-gate review. Splitting images into
unrelated gates would misstate their meaning; selecting only passing captures would
lose the original failure history. The failed preparer wrote no bundle or index.

## Decision

Raise the internal contract's per-gate slots and receipt's per-result evidence limit
from32 to64, in both CUE and the pre-CUE Go guard. Test exact boundary admission,
over-limit refusal, schema/guard agreement and unchanged non-certification behavior.
Do not change the256KiB document,64MiB decoded content,92MiB encoded bundle, input
group, PNG, catalog pagination or index limits. Gate applicability, reasons, input
roles, independent pins, original membership and canonical encoding remain required.

Retain P09's four complete selected runs as retrospective evidence with exact report
and image hashes. Their selected source/output/harness closure is not a full replay.
The failed base remains failed; later scoped repair successes do not complete any
full gate. No retained source executes in Gallery and no images enter MCP metadata.

## Compatibility and migration

The JSON shape and schema_version1 stay unchanged. Previously accepted canonical
packets keep exactly the same bytes and identities and remain readable. This is a
**breaking reader-capacity expansion**: older binaries reject a new packet with more
than32 slots/evidence entries per gate. Before selecting that packet, rebuild the
private writer, Gallery and MCP readers from a revision implementing this decision.
Do not replace running user previews or globally restart agent integrations without
their scoped authorization. A separately built local preview can inspect the packet.
No rewriting, rehashing or deletion of old packets is a migration step.

## Alternatives considered

- Multiple smaller records: keeps old-reader compatibility but fragments one run's
  comparison history. Prefer it if complete independent runs exceed the new bounds.
- Reassign captures to unrelated gates: rejected because metadata would mislead.
- Omit captures or combine them into a new image: loses exact complete originals.
- Remove item limits: unnecessary evaluator/read amplification within byte bounds.

## Consequences

The evaluator may see up to twice as many slots per gate. Existing byte ceilings
still apply; actual packet metadata and served-image cost need scoped checks before
claiming usability. The reader upgrade requirement must remain visible in guidance.
No runtime API, dependency, consumer adoption or broader conversion support is added.

## Revisit when

- A complete selected run requires more than64 slots or exceeds unchanged byte limits.
- Measured packet reads impede comparison or bounded MCP responses no longer fit.
- An old packet changes identity, a larger packet weakens refusal, or historical
  observations become a compatibility claim.
