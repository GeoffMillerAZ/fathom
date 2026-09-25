# ADR-0039: Separate explicitly reported errors from exit classification

- **Status:** Accepted
- **Type:** architecture
- **Date:** 2026-09-14
- **Follow-up:** ADR-0070 resolves the invocation-migration triggers below; this
  reporting decision remains accepted and its historical failure evidence unchanged.

## Context

The bounded Bosun/Surveyor [runtime probe](../assessments/2026-09-14-cli-consumer-errors.md)
found silent exit 1 for unknown flags and commands in both actual binaries. Their
no-op Fang handlers avoid duplicating curated leaf diagnostics but also discard
framework errors. I10 requires distinct failure/could-not-check outcomes; the
existing `Code` contract deliberately maps ordinary errors to 1, never guessed 2.
Pinned Cobra v1.10.2 returns untyped discovery/required-flag/flag-group errors;
Fang v2.0.1 calls its error handler before returning, and creates commands internally.

## Decision

Add opt-in `command.Reported(err)` and `command.UnreportedErrorHandler(next)`.
`Reported` acknowledges an already-rendered diagnostic, preserving error identity,
text and explicit exit codes. Nil remains nil. The handler delegates an unreported
error once; nil `next` selects Fang's default handler. It never mutates `Execute`,
overrides consumer options or infers acknowledgement from a nonzero exit status.

Single-error wrapping retains acknowledgement. A join is acknowledged only when all
non-nil children are acknowledged, or the caller explicitly marks the entire join.
A mixed join must render: one reported child cannot hide another child's failure.
Rendering the intact mixed error may repeat a reported child; losing an unreported
diagnostic is worse. Callers must not mark an error before actually reporting it.

Keep `Code`/`Exit`/`Execute` compatibility unchanged. Usage classification is a
separate open requirement, not fixed by this handler. App-owned usage refusals can
already carry `Exit(2, err)`; automatic Cobra classification needs typed provenance
at the error origin and must preserve existing explicit exits and callback policy.

## Alternatives rejected

- Suppress every Fang error: hides actionable syntax failures, as measured.
- Treat every nonzero exit as reported: exit status says nothing about rendering.
- Find any reported descendant with `errors.As`: hides unreported joined errors.
- Classify by message text or whether `RunE` started: confuses application/pre-hook,
  generated-command and invocation failures; it cannot prove the 0/1/2 contract.
- Recreate Cobra execution phases or fork dependencies in this slice: larger and
  riskier than the independent reporting fix. No cache patch or pin change is made.

## Consequences and outstanding work

The low-layer helper addresses a repeated need without a new CLI framework or
dependency. Consumers must opt in and mark only their curated, already-rendered
failures. Synthetic package tests do not establish their adoption. The retained
consumer probe continues to require nonempty stderr and exit 2 for invalid syntax;
its current four failures remain G5 work, not accepted success.

## Revisit when

- An authorized consumer migration can adopt explicit reporting and demonstrate its
  successful, failed-check, usage, early-callback and machine-output branches.
- A pinned Cobra candidate supplies typed invocation-error origins, or a bounded
  reviewed patch can prove those branches without replaying callbacks or changing
  caller policy. Evaluate dependency cost and consumer compatibility before adoption.
- A real error composition needs acknowledgement beyond the documented chain/join
  rule; preserve unreported diagnostics before broadening suppression.
