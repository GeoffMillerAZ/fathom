# Quality-boundary excellence review (2026-10-22)

Scope: read-only source and test review of the diff from `d9191ec` in `cmd/quality-gate/**`, the adoption guide/skill, design `QB-9..14`, decision 0031, and `DW-1..9`. No source edits or commits were made by this reviewer.

## Initial finding and disposition

**F1 · P1 · Shadow-only observation initially collected no model evidence. Resolved in current source.** The first pass set exit 2 for zero required coverage and skipped `model`, including under `--observe`. `main.go` now preserves the enforcement unknown while allowing an explicit shadow observation after structural checks. `TestShadowOnlyObserveCollectsWithoutEnforcing` proves `check` and `pre-push` collect a model receipt only in observe mode.

**F2 · P1 · Deletion-only push initially passed an empty required boundary. Resolved in current source.** The deletion branch now applies the independent zero-required-coverage floor and exits unknown; the shadow-only regression covers an all-deletion push.

**F3 · P2 · Observe reason initially misstated a clean ref. Resolved in current source.** The final pre-push loop now derives each report's own enforcement exit, and `TestPushObservationRetainsEachRefOutcome` proves a clean and held ref retain distinct reasons.

**F4 · P2 · Required source unknown initially arrived after push model dispatch. Resolved for the required boundary; advisory shadow behavior accepted.** `PrepareOnly` validates source, request shape and input cap before environment, cache or provider work. `TestPushPreflightsRequiredModelSourceBeforeAnyDispatch` proves a later invalid required source blocks all dispatch and marks an unchecked required ref unknown. Decision 0031 and QB-12 now explicitly keep a shadow input unknown advisory: it cannot clear a required floor, and a valid other ref may still collect bounded shadow evidence. `TestShadowSourceUnknownIsAdvisoryAndDoesNotDispatchItself` uses the real evaluator seam and proves one call for the valid ref, none for the invalid ref, with the latter reported unknown. The rejected alternative is recorded in decision 0031.

## Covered areas

- Git executable resolution, candidate-path and symlink rejection, raw-OID push rows, exact commit/tree capture, bounded Git output, and policy-ref recheck.
- Full model recipe/cohort hash, canonical question and source ordering, provider model attestation, rounded probability confidence, calibration binding, advisory shadow cache, HTTP body cap, and invocation-shared batch budget.
- Push-wide structural preflight, required site receipt enumeration, empty required boundary handling, applicability and unselected-path reporting, plus CUE/Go policy bounds.
- Documentation separates workflow blueprints from implemented integration; decision 0031 rejects a universal accuracy floor and acknowledges owner-selected calibration bars.

`go test -race ./cmd/quality-gate -timeout 90s` passed after the final F4 scope/test delta (12.896s). The default policy passed `cue vet -c cmd/quality-gate/schema.cue cmd/quality-gate/default-policy.json -d '#Policy'`; `gofmt -l cmd/quality-gate/*.go` was empty and scoped `git diff --check` clean before this final delta. Fathom `review_diff` over the changed paths reported a medium band but only 5/8 dimensions checked, with a stale index and incomplete obligation compilation; those are scoping inputs, not source proof.

## Remaining limits

This review did not run live providers, install hooks, or verify protected CI status in an adopting repository. A trusted host/Git installation and a genuinely trusted policy ref remain external requirements. Focused source/tests do not prove exhaustive CUE/Go equivalence or model accuracy under owner-chosen bars.
