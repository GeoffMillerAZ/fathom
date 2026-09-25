# ADR-0042: Bind target evidence without self-certifying compatibility

- **Status:** Accepted
- **Type:** architecture
- **Date:** 2026-09-14
- **Extends:** ADR-0023; existing syntax and adapter formats remain unchanged

## Context

Five typed candidates have separate replay/browser receipts but no shared target
receipt checker. P06 is a concrete first integration. Its useful historical evidence
does not satisfy every gate; hashing a report must not turn its assertions into proof.

## Decision

Implement a separate internal CUE-authored contract/receipt integrity layer. A contract
identifies the source, target profile/variant, output, input roles, nine applicability
gates and expected evidence slots/roles. Receipt observations bind its exact digest,
source revision and artifact hash. Callers supply independently selected expected pins
and a bounded exact byte inventory; receipt-provided expectations cannot prove freshness.

Check schemas, exact canonical JSON, identities, evidence-slot admission and content
hashes deterministically. Missing, stale, contradictory, duplicate or unexpected material
is an error. Failed/not-run gates remain valid historical observations with reasons;
they are never passes. N/A requires a contract reason and an explicit review assertion,
not a receipt author's shortcut. Reviewer names are assertions, not authenticated approvals.

Return `integrity=matched`, claimed gate statuses and incomplete gates. Always retain
`verification=not-verified`, including an all-passed receipt. This checker does not parse
the meaning of a report, run its commands, prove archive membership, authenticate review,
or establish platform/consumer compatibility. No generic runner/plugin registry is added.

Historical imports explicitly say `retrospective`; they cannot claim that their contract
was frozen before the run. Prospective applicability/oracle review and family-specific
semantic checks remain required for certification under ADR-0023. Current P06 integration
retains open gates, private source bytes and source limitations.

## Consequences

The integrity layer makes evidence attributable and tampering visible, not trustworthy
by itself. Before verified-target discovery ships, add report-specific semantic refusal
canaries, reviewed applicability/baseline changes, consumer history and a separate
promotion path; then expose identical target records through Gallery/MCP. Original
retrieval and legacy `not-verified` envelopes stay independent and usable throughout.
No new catalog profile strings, public runtime dependency, consumer changes or license
rights follow from a receipt. The [guide](../design/target-evidence.md) owns current APIs.

## Alternatives rejected

- Treat a hash-valid all-pass report as certification: lets a forged or unrelated report
  certify a component. Wins only with independent semantic and review admission.
- Extend syntax receipts with compatibility flags: confuses grammar success with target
  proof and changes existing historical envelopes unnecessarily.

## Revisit when

- A family-specific semantic verifier and actual consumer meet every applicable gate.
- A seeded stale, role-swapped or self-authored report advertises target compatibility.
- A named integration requires authenticated multi-party review or remote storage.
- Two real candidates cannot express their provenance or evidence with the narrow format.
