# 0041: Harness configuration authenticates selected sources

Status: accepted
Type: security
Date: 2026-10-24

## Context

GT6 qualifies controlled Git reads. GT8..11 still lack protected policy composition,
native advisory-maintenance grants and host integration. Existing agent-hooks is
economic guidance, not authority. The lexical evidence-signature contract is not a
digital signature verifier. Names or candidate-owned files cannot authenticate policy.

## Decision

Admit [HC-1..4](../../designs/harness-control.md) as command-local `harnessctl` within
0039's existing W1 slot, budget and no-installation boundary. No new foundation,
dependency, daemon, third slot or consumer/global mutation. The kit's synthetic
source-review receiver exercises independently enrolled operator/org/project sources.

Use one protected operator registration for selected projects, exact expected source
rosters/revisions/digests, enrolled Ed25519 keys and target context. Require the fixed
DSSE/Ed25519 profile for every selected layer before composing it. Validate current
selected snapshot/time; do not claim remote revocation or OS/actor attestation.
Signed layer data may be checked into a candidate project: only the protected
registration/keys establish issuer authority. Candidate availability or filenames do
not. Registration and dispatcher remain outside candidate write authority.

Mandatory floors intersect; advisory preferences affect only specifically delegated
sample rates within every issuer's constraints. Source priority selects among valid
preferences, never weakens a floor. Failure reports cannot disclose source bodies or
identities. JSON explanation is evidence, not execution authority or an allow cache.

Keep native user presence, approver policy, exact maintenance grants and durable
per-boundary state separate in GT9/10. A biometric/authentication Boolean is insufficient.
GT8's wall-clock check cannot establish reboot/rollback-safe leases. Existing agent-hooks
fault behavior is preserved until an admitted, tested GT11 integration.

No model site belongs in source/signature/mandatory-policy validation: those questions
are deterministic. Optional judgment may later add a concern under GT12; it cannot clear
a failure. App-kit keeps presentation/accessibility ownership and the existing stdlib
CLI divergence. This work supports the anchor workflows without claiming PX completion.

## Alternatives rejected

- Universal policy/PKI/update engine or Warden fork. Wins if: a measured named receiver
  needs capabilities beyond this closed profile and a separate admission proves reuse.
- Unsigned files because public keys exist, or filenames imply issuer identity.
  Wins if: never for this profile; a separately specified protected-source profile would
  need equivalent independent authority and failure evidence.
- Canonicalize arbitrary JSON and call its hash authorization. Wins if: never; exact
  DSSE payload verification establishes selected source authenticity, not effect consent.
- Let a global switch, model verdict or authenticated gesture disable mandatory checks.
  Wins if: never under 0039; exact advisory maintenance remains separately controlled.
- Forbid in-repo signed snapshots of every kind. It blocks project extensions/org
  caches without improving the independent signature/pin boundary. Wins if: a named
  deployment forbids candidate-controlled availability and selects protected sources.
- Collapse every failure into a constant count. Fixed reason codes, truthful counts
  and bounded source ordinals aid repair; their metadata still needs recipient authority.

## Revisit when

An unverified or stale source contributes policy; a preference weakens a floor;
selected target/issuer/revision changes without a hold; a source leaks on failure;
operator storage protection or native time/identity assumptions fail; two consumers
prove a missing shared seam; or 2026-11-21/next release arrives. Keep GT9..11 and the
full goal open until their native receiver evidence exists.
