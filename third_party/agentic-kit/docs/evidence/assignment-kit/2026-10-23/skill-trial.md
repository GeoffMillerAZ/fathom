# Assignment Brief guide-only trial, 2026-10-23

Boundary: I had already reviewed the source and tests, so this is not blind usability evidence. The trial followed only `docs/assignment-kit.md` and `.claude/skills/assignment-brief/SKILL.md` for commands and interpretation. It used synthetic repository fixtures. No provider, network, private input, hook, or external effect was used.

## Trigger and instructions

The skill description triggers on authoring or assessing a bounded assignment and completion disputes. It expressly excludes ordinary task lists and replacing repository intent. Its procedure requires independent expected scope, typed assignment/scope/input, exact evidence and revision pins, finite budget/deadline, actual `Evaluate`, no action authority, and retained partial receipts. I found no instruction to run a model, install a hook, contact a person, or treat `complete` as permission. This is a metadata/instruction review, not a measured implicit-trigger test in another agent session.

## Executed from the guide

| Guide step | Observed result |
|---|---|
| `go run ./examples/assignment-kit` | Current JSON is a wrapper with `synthetic:true`, an explicit offline evidence claim, and eight named entries under `snapshots`. Statuses: `command/complete=complete`, `command/missing-required-unknown=continue`, `investigation/complete-stretch-failed=complete`, `investigation/unresolved-ambiguity=blocked`, `investigation/resolved-complete=complete`, `investigation/no-progress-pivot=pivot-required`, `investigation/ambiguity-escalation=needs-review`, `investigation/exhausted-budget-stop=stopped`. All eight report `action_authority=false` and nonempty assignment/scope/evidence pins. |
| `go test -race ./kits/assignment-kit ./examples/assignment-kit` | Passed for both packages. |
| `cue vet ./kits/assignment-kit/...` | Passed. |
| Negative fixture: `go test -count=1 -run 'TestAcceptanceAndPins|TestEvidenceRules|TestConfidenceBindings|TestConcernsBudgetsAndRules' ./kits/assignment-kit` | Passed. These tests assert changed/omitted pins refuse and missing/stale/foreign evidence or unqualified confidence cannot complete; budget/deadline equality stops. This reuses authored tests, so it is weaker than an independently written consuming app. |

## Adoption finding

The guide gives a usable command path and accurately explains all six statuses and invalid input. It distinguishes completion, trust and action authority. The short Go snippet assumes typed values already authored; the runnable examples supply those values. No live caller, independently authored receiver scope, actual authenticated source, or skill-trigger telemetry was exercised. The full AS-A10 independent trial therefore remains a qualification claim for the root owner to decide, not a measured adoption claim.

The new example tests prove omitted privacy and counterevidence goals reject against explicit scope rosters and changed/moved catalog definitions do not reuse old pins. `go test -count=1 -race ./examples/assignment-kit` passed. The trial remains synthetic and cannot prove a consuming application's independent authorship or retained artifact truth.
