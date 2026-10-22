# Landing evidence

Decision: [ADR-081](../../ADR.md#adr-081-landing-evidence-for-many-agents-and-machines-exact-tree-receipts-git-note-provenance-and-computed-candidates-2026-10-22), accepted by the owner-architect, 2026-10-22.
Plan: `docs/plans/landing-evidence.md` (LE-). Boundary work: `docs/plans/boundary-enforcement.md` (BE-). Extends [multi-machine development](multi-machine-development.md), ADR-078 D2: receipts move evidence. ADR-081 amends D4's "reuse, no new verb" clause with file-based gate-receipt verbs; its deferred store columns stay deferred.

## Problem

The scrubbed 2026-10-22 [baseline reading](../bench/readings/landing-evidence/2026-10-22-baseline.md) inventory finds test outcomes and commit/dirty/race fields in `test_runs` and `test_cases`, but no git tree hash, command, host label, toolchain, exit code or log checksum for gate reuse. `cmd/fathom/testrunhook.go` records a working-diff digest in `tree`; that is not a git tree object. Existing `fathom export receipt` supports symbol-location and caller evidence, not a gate result. `gatepreflight.go` has advisory same-session reuse only, and `internal/readout/testdelta.go` detects possible same-tree flakes only within a session. Source pointers were checked at HEAD; the inventory is not a new implementation claim.

The same reading's 21-day boundary audit reports operational conserve skips in 19 of 21 gate logs. Of the last 100 main CI runs, 39 failed, 42 passed and 7 were cancelled; its summary does not classify the remaining 12. It reports 126 of 3,516 non-merge commits lacking `Verified:` since September 1 and a conflict marker in ADR.md. The marker was grounded at the starting HEAD and removed in this docs correction; the recurrence check remains BE-1. Conserve stderr is captured at HEAD but hidden by its generic operational-skip branch; BE-4 diagnoses why, rather than assume a missing store. The same reading's census found 20 task ids shared across plans. Existing intent-index and plan lint already reject ADR/index drift and within-plan duplicates; reuse them rather than claim uniqueness is wholly absent.

## Mechanism

1. **d1, exact-tree receipt files (LE-1 and LE-2).** A gate run writes git tree hash, dirty flag, validate task name and a digest of Taskfile plus lefthook configuration, toolchain, os/arch, operator-named host label, exit, log digest, start and end. Refuse a dirty tree. Names `fathom gate receipt write|check` are provisional. A receipt is stored under the fathom home or a repo-configured directory and carried by the operator. With `[gate] accept_receipts` opted in, pre-push checks a successful receipt's exact tree and command/configuration/toolchain/platform identity under the repository's declared gate requirements before accepting prior evidence. A changed task or hook digest cannot match. Accept only when the gate log is present beside the receipt, re-hashes to the recorded log digest, and ends with the gate's own success line; dirty trees are never accepted. `[gate] accept_receipts` stays off in this repository until BE-11 is decided. A repo-local `.fathom/guard.cue` guard in BE-8 scope denies agent writes to the receipt directory. Host labels name provenance; they are supplied, never read from the machine. CI remains the independent Linux witness, so a helper result cannot erase a missing or red CI result. No new table is required.
2. **d2, provenance notes (LE-3).** `fathom provenance note` writes harness, model, operator host label, session and lane to `refs/notes/fathom/provenance`. It adds no commit trailer and never pushes notes. The operator's refspec decides whether notes travel. These are declarations, not authenticated identity. File and note fields avoid any dependency on MM-3's deferred `test_runs` columns; if an implementation needs those store fields, that portion waits for MM-3 and RT-M1 PASS.
3. **d3, flake ledger (LE-4).** Read hook logs and imported receipt files across sessions. A named test must have failed and then passed on the same git tree hash; an aggregate passing gate without per-test evidence is not proof that a particular test passed. Rank by recorded gate time lost, with stable tie ordering. Existing hook `tree` diff digests cannot be equated to git tree objects; unbound legacy runs stay unknown. No new table, collection daemon or test timing experiment is added.
4. **d4, culprit candidates (LE-5).** Read a red commit range, failing-test reach and blast radius; rank candidates deterministically and print `git bisect` commands for the operator. Name missing graph evidence, never infer no reach from an unreadable index. This is a candidate list, not a culprit verdict. Fathom never runs a build or test.
5. **d5, unique ids (LE-6).** Gate configurable record families: ADR headings, invariants, futures, incidents and per-plan task ids. Reuse existing ADR/index and plan-id checks; reject each new duplicate with its locations. Cross-plan checks are opt-in, grandfathering the 20 already-shared ids by an explicit list. Exclude `docs/plans/archive/**` from the scan; AD-1 through AD-4 recur there and are not active-plan duplicates. Do not silently widen the exemption and do not change plan grammar v2.

## Failure and fail-open behavior

A missing, malformed, failed, dirty or identity-mismatched receipt cannot replace a gate run. Without an acceptable receipt, use the existing gate path and its existing exit semantics; receipt lookup never turns infrastructure failure into a pass. Default-off acceptance preserves the current rerun behavior. The trust model is same-operator trust, unsigned: matching identity and a checked log do not authenticate an author. An external signer is the path to stronger trust under portable-receipts; fathom signs nothing and verifies no signature. A matching receipt proves only the named run on the identified tree and tool closure; it neither verifies a signature nor supplies CI's independent result.

Hooks retain I9 and I53: operational failures fail open within their deadlines and are not retried past the reserve. Boundary enforcement BE-4 makes the first operational error visible without turning it into a blocking claim. Notes are optional evidence and absent notes say nothing about the code. Unknown tree/outcome evidence cannot classify a flake, and missing test reach is disclosed in candidate ranking. No receipt can bypass a blocking failure or change `fathom verify` exit codes.

## Generality

Any adopter can move files from a helper machine, CI, teammate or cloud session using existing carriers. The receipt is harness-independent; a harness with the needed post-command hook event can contribute run evidence. Without that event, an operator-invoked receipt command supplies evidence explicitly. Without per-test outcomes, only the gate result is available and the flake ledger degrades to unknown for individual tests. Without an index, culprit ranking names missing reach and still prints the operator's commands. Provenance is useful with any harness/model declaration and git notes support; note transfer remains an operator choice.

Each machine keeps its own loopback daemon, if it needs one (ADR-078 D1/D7). There is no central daemon or daemon-to-daemon RPC. The non-goal [“No dispatch loop”](../intent/scope/non-goals.md) remains binding: no scheduling, retrying, watching, spawning, placement or machine control. [Portable receipts](../plans/portable-receipts.md) says “signs nothing, dials nothing”: fathom also verifies no signature, posts nothing and fetches nothing (I56). File inspection adds neither transport nor signing. F92's placement-evidence promotion trigger and MM-3's deferred store columns stay unchanged.

## Compatibility surface

This document accepts work, without implementing it. Store format v0, plan grammar v2, verify flags/exit codes and existing hook contracts stay unchanged. The planned CLI verbs, `[gate] accept_receipts` setting, receipt representation and pre-push behavior intersect `docs/intent/scope.md`'s compatibility surface and need implementation-time review, a breaking fragment and migration note where that contract changes. Acceptance remains opt-in, and CI remains independent. The docs lane's Changed fragment records the Stop-hook contract clarification under ADR-080; this landing-evidence design changes no runtime behavior. The table freeze prohibits any new table, column, cache or background job until its existing RT-M1 condition; no tenancy or governor policy changes are implied.

## Decision models

None applies: matching identity, ordering evidence, detecting duplicate ids and ranking reach are computed with provider `none`. A future flake-versus-real classification of failure text could be a sampled opt-in audit, never a code fact or a run verdict. It would require a labelled deterministic baseline, calibrated-band policy, provider `none` unknown fallback, explicit opt-out and a keep/kill reading before activation. No model spend is authorized.

## Measurements

All new keep/kill thresholds below are set here, not measured results; the 20-id grandfather count is a supplied inventory count.

- **LE-M1, end-to-end reuse.** A helper-produced receipt is accepted for a real exact-tree push and CI independently agrees. Record the tree, command/config digest, toolchain/platform, log digest, gate exit, receipt check result, push result and CI result. Missing CI or identity is incomplete, never agreement. Keep receipt reuse only if that reading succeeds; refusal cases must still rerun or refuse under the existing gate rather than accept stale evidence.
- **LE-1 and LE-2, identity controls.** Deterministic fixtures must reject dirty trees, altered tree/config/toolchain/platform identities, nonzero exits, missing gate logs, mismatched log digests, logs without the gate's own final success line and corrupted log evidence; default-off takes the existing gate path. These are functional assertions, never wall-clock or CPU-time tests.
- **LE-3, provenance boundary.** Verify the named note contains the declared fields, commit text has no added trailer and no note push is invoked. A need for store host fields defers only that part behind MM-3; do not add columns during the freeze.
- **LE-4, flake evidence.** Replay failed-then-passed same-tree test evidence across sessions and imported files, preserving recorded cost order. Different or unknown trees and aggregate-only results must not label a test flaky. Kill any unsupported classification, keeping unknown visible.
- **LE-5, candidates without execution.** A known reach fixture ranks implicated commits ahead of unrelated commits; ties are deterministic, missing graph inputs are named and no build/test is invoked. Failure of these controls rejects the ranking implementation, not the operator's ability to run bisect.
- **LE-6, record boundaries.** Mutating a unique fixture to duplicate each configured family must fail with both locations; the explicit 20-id cross-plan grandfather list passes, a new shared id fails when opted in, and opt-out leaves existing per-plan checks intact. No plan grammar amendment is needed.

Readings live under `docs/bench/readings/landing-evidence/`. Reopen a server-side VCS only with more than 20 concurrent agents (set here) landing to one branch and measured push retries, or more than 500 lands/day (set here) (ADR-081 O1). No such contention reading was supplied; this design claims none. The committed 2026-10-22 baseline preserves supplied observations; none of the implementation outcome readings is completed by writing the record.

## As implemented (LE-1, LE-2)

ADR-081's local receipt path records one clean git tree, framed gate log and command
identity without a store table, signing, transport, gate dispatch or model call.
The wrapper emits the commit, tree and clean state at START and PASS, fails if they
change or either endpoint is dirty, logs the Go environment, and calls the receipt
writer only after PASS. Write and check bind both endpoints to the receipt tree.

Anyone who can run `fathom gate receipt write` as you can create an accepted receipt;
the guard stops editor-tool tampering only; CI is the independent witness.

The repository's enforced command guard denies agent receipt-writer invocations;
the enforced file guard protects its receipt directory. These controls do not
authenticate unsigned evidence. Acceptance stays off unless the project sets
`[gate] accept_receipts = true` and the user sets
`[gate] allow_receipt_acceptance = true`. Either alone does nothing; doctor names
half-set consent. This repository leaves acceptance off.

The checker requires the logged Go version and os/arch to match local `go env` and
`go version`, and requires `GOWORK=off`. Command identity reads gate definitions
from the requested git tree. For HEAD, the tree hash already covers tracked gate
definitions and scripts. Pre-push checks every pushed non-delete local SHA and
falls back to validate if any receipt refuses. CI remains independent.
