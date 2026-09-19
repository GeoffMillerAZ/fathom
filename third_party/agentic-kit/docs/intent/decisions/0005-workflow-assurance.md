# 0005: Governance claims belong to a tested workflow and execution surface

**Status:** accepted
**Type:** architecture
**Date:** 2026-09-12

## Context

Importable safeguards are frequently optional: the harness permits a nil detector and
budget, and effect policy defaults to permissive. The CLI clients reject caller tools;
they are not integrations with the host coding agent's policy lifecycle. A green
`task validate` also coexists with violations of I12/I13 (INC-8). Package availability
cannot establish that a consumer's workflow is governed.

## Decision

- The product entry point becomes a small set of composed workflows with explicit
  profiles. Keep the existing piecemeal APIs and their compatibility contracts.
- Each claimed practice names its applicability, enforcing code, execution surface,
  failure behavior, test evidence, version, and remaining consumer responsibility.
  Use the vocabulary in [assurance.md](../assurance.md).
- A supported profile validates required wiring before work begins. Missing evidence
  or a skipped required probe produces `unknown` or `unsupported`, never a pass.
  Implementation is scope G5; this record does not claim the profile exists today.
- Distinguish model-provider adapters from Claude Code / Codex harness adapters.
  Generate native instructions, skills, hook configuration, and receipts from shared
  policy data where semantics match. Probe each supported host/version independently.
- Instructions and heuristic detectors are advisory. Blocking hooks cover only their
  demonstrated tool paths. Security authority resides in controlled dispatch, service
  credentials, native permissions, or an OS boundary, with uncovered paths stated.
- Test the failure mode through the consumer entry point: disabled guard, parse error,
  timeout, absent dependency, bypass path, and stale configuration. Static reachability
  proves wiring, not execution or semantic correctness.
- Spend claims distinguish estimates from reservations and measured actuals, including
  retries and child work. Unknown cost cannot be reported as measured zero.

## Consequences

Shared intent can travel across harnesses without pretending their semantics are equal.
Profiles add a small maintained surface and conformance matrix. New strict defaults are
introduced through explicit profiles; existing constructor behavior does not silently
change. No gate, dependency, stability tier, or external configuration changes here.

## Alternatives rejected

- More instructions alone: useful guidance, insufficient authority enforcement.
- Replace the host agent with a universal runtime: increases ownership and reduces
  compatibility; the kit remains composable.
- A single excellence score: can hide a critical control failure behind passing checks.

## Revisit when

- A supported host upgrade changes a tested hook or permission behavior.
- A required profile control can be bypassed through an advertised execution path.
- Two consumers need incompatible meanings for the same profile field.
