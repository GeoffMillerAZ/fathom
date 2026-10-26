# Prose support-review differential security review, 2026-10-25

**Recommendation:** approve the bounded offline runner change. No confirmed
blocker remains in the reviewed source. Paid semantic qualification, production
custody and document acceptance remain open. This is a high-risk change surface
because it governs provider dispatch and spend; a source review is not runtime
or account-level assurance.

| Severity | Confirmed and repaired | Open confirmed blockers |
|---|---:|---:|
| Critical/high | 0 | 0 |
| Medium | 3 | 0 |
| Low | 2 | 0 |

## Change and history

Baseline:307e7abca76727e66377307789ae356db0eb71bc. Exact reviewed command/test/input
bytes and executed gates are in the [qualification receipt](../evidence/prose-headless/2026-10-25-review/receipt.json).
The change adds review preparation, opt-in capture and offline replay; it factors
journal reading without exporting a new library or adding dependencies.

| Changed production file | Security surface |
|---|---|
| cmd/prose-lab/review.go | Scope/roster binding, dispatch guard, prior-case checks, fixed report states |
| cmd/prose-lab/review_artifacts.go | Native replay versus marker, typed receipt, private rationale and charge |
| cmd/prose-lab/provider.go | Optional exact prepared-wire fence before reservation/transport |
| cmd/prose-lab/ledger.go | Shared history validation and no-create locked audit |
| cmd/prose-lab/main.go | Explicit command routing/help |
| cmd/prose-lab/schema.cue | Closed scope/capture/report shapes and held-state consistency |

Git blame attributes baseline journal open/load behavior (lines81..220) to
93df0cf1548c5ba1963d5e1161902cfeaf65ee44. No authorization or safety check was
removed to pass this change. Baseline validateOutcome already validated pricing,
model and reservation arithmetic for every settled outcome. The early pending
reservation check therefore does not create the alleged old-settled-journal
compatibility regression. Existing writer refusal of unfinished history remains.

## Repaired counterexamples

| Ref | Severity and concrete failure | Repair and evidence |
|---|---|---|
| F1 | Medium: a native match followed by failed artifact persistence could permit the next paid case | reviewPriorAttempts now replays native response and crosschecks all marker/receipt/rationale/charge fields through reviewArtifactValidate; TestReviewArtifactFailureStopsCohortAndReportsCharge confirms no second call |
| F2 | Low: a charged invalid attempt appeared missing, obscuring the actual denominator | runReviewReport exposes invalid_count and invalid case status with charge retained; independent forged-receipt test expects invalid1, missing0 and charge134 |
| F3 | Medium: request drift was detected only after provider dispatch | attemptRunner.call checks expectedRequestDigest in bindWire before reserve/inner transport, provider.go:269; wrapper mutation test observes zero inner calls, reservations and charge |
| F4 | Medium: a failed private rationale write could leave supported/matched true in a held receipt | runReviewWith persist closure resets held/false on every error, review.go:439; artifact-failure regression checks the serialized private receipt; CUE held/matched combination refuses |
| F5 | Low: settled failed/unknown outcomes were labeled pending | runReviewReport preserves distinct terminal statuses/counters and full reservation, review.go:574; native-journal tests exercise both states |

Conditional packet-review concerns about missing usage/model/price checks are
closed by validateOutcome/readUsage, not duplicated checks. Unknown support
statuses, empty reasons, decode failures and non-text/refusal blocks are refused
by the existing documentReviewer and canonical text adapter. Helpers were inspected
before rejecting those conditional findings. Semantic inconclusive remains a safe
hold and cannot satisfy this fixture's qualification bar.

## Tests and blast radius

The independent owner test reads separate fixture data, not the runner's scope
loader, to supply all67 native-response oracles. It covers complete matching,
false support, always-hold specificity, inconclusive, intact pending accounting,
forged artifacts and weakened intake. Implementation tests cover replay, prior
artifact failure, wire drift, failed/unknown outcomes, no-create/locked audit and
Go/CUE boundaries. Full command race tests passed in38.773s; task validate passed
in67.889s. Exact hashes and scope are in the receipt. No coverage percentage was
measured and no provider ran in these tests.

At baseline307e7ab, Fathom reports two runtime openJournal callers (runCanary and
runDocumentWith), seven related tests and one test helper. New runReviewWith adds
one runtime caller. The shared readHistory path therefore affects three
command-local runtime flows. The optional prepared digest is set only by the new
review path; empty defaults preserve canary/document request behavior. Existing
journal/provider/document tests ran with the full gate. All five exported roots
and go.mod are byte-identical to baseline, so no downstream rerun or new consumer
compatibility claim is made. Prior consumer evidence remains explicitly scoped.

## Method and limitations

Surgical differential review of six production files, both new test files,
review controls and selected design. Used baseline blame, typed caller evidence,
current-source review, independent authored controls, concrete tampering and
failure scenarios, a standard implementation review and a final deep source critic.
Fathom review_diff had stale/incomplete dimensions; it was not treated as a pass.
Two Opus packet reviews supplied additional ordinary-text findings, but both
invocations exited1 at soft CLI budgets. The original packet review covered only
part of its packet. The final source critic inspected the repaired current files
without running tests or providers. Actual runtime claims come from separate gates.

Operator-owned private roots and cooperative locks are assumptions, not hostile
filesystem confinement. Lstat/OpenRoot namespace races and root tampering are not
qualified. Hashes prove byte identity rather than source authority. A fresh journal
is a fresh accounting scope; it cannot reset the approved stage's cumulative spend
or establish study-wide replay prevention across arbitrary roots. The owner must
retain every attempted root. No account-wide cap, provider invoice, real model
accuracy, source truth, publication authority, production host or UI is established.

## Required next evidence

HP-12 must independently freeze the exact source/binary, complete67-case roster,
model/pricing and stop rubric before opt-in live calls. Retain the first mismatch,
unknown and every missing planned case without automatic retries. HP-06/09 must
separately establish whole-document goals, selective regeneration, human authority
and UI-entry readiness. Those are planned, unimplemented qualification work, not
approval conditions silently satisfied by this source report.
