# 0010: Accept bounded, pinned experimental guardrail adoption

**Status:** accepted
**Type:** compatibility
**Date:** 2026-09-13

## Context

Throughline already consumes experimental `kits/guardrails-pipeline`. The owner's
delegated architecture/implementation work now replaces its findings workaround and
validates the actual vendored upgrade (`a007075`, kit `188c53c`). A production import
does not prove a stable public contract; neither does a green synthetic suite.

## Decision

Keep the classification experimental and accept the existing consumer's upgrade risk
for this bounded pilot. Throughline pins a specific module revision and tracked vendor
snapshot; upgrades remain deliberate changes with its full default gate, structured
finding/advisory regressions and documented rollback to the preceding pin. No live
deployment, silent dependency update or broader compatibility promise is authorized.

The maintainer owns review at the next release, before the next production upgrade,
or by 2026-10-12, whichever comes first. Evaluate a stable promise only with exercised
consumer contracts and a named maintenance commitment. A second named pilot alone is
not evidence for promotion. Keep upgrade/version evidence in the consumer's
`docs/upgrades/governed-kit-delivery.md`, not duplicated compatibility counts.

## Alternatives

Immediate promotion would hide unresolved API evolution under a promise. Removing an
existing working consumer would discard useful evidence and is unnecessary when the
consumer's exact dependency and tests already make change explicit.

## Revisit when

- The next release, production upgrade or 2026-10-12 arrives.
- A guardrail change breaks the pinned consumer or loses structured findings.
- A second exercised consumer requests the same compatibility commitment.
