# HSE source and boundary review

2026-10-24. Scope: [0044/HSE](../../../designs/harness-spawn-economics.md),
against base `002ed135fdbb582be56b519806639d34bf18a2d1`.
Methods: differential-review, sharp-edges, independent read-only protocol review,
owner source review, injected receiver tests and actual pinned-process fixtures.
Publication acceptance and final measurements live in the adjacent receipt.

## Threat and authority

The candidate can supply delivered native event data and repeat attempts. It
cannot legitimately select the protected controller, profile, policy, evaluator,
approver roster or receiver state. Deployment must exclude candidate writes to
those roots and native hook argv. Matching hashes or Unix modes do not establish
that separation. Administrator/same-UID isolation, actual host interception and
live native enrollment are outside this source qualification.

Reviewed paths: pure evaluator startup/policy/decision/output, shared strict wire,
native normalization, sampling, protected receiver entry/exit, controller optional
references, child snapshot/process/cleanup, public report and CUE branch laws.
No foundation/provider API was changed. Native approval remains HMT-owned.

## Findings and dispositions

| ID | Counterexample and impact | Disposition / regression proof |
|---|---|---|
| HSE-R1 | Main resolved home before pure dispatch. A supposedly input-only peer still performed ambient discovery | `runMain` bypasses home lookup for every peer path; panic home/environment seams cover valid, malformed, help and flag cases |
| HSE-R2 | Legacy policy `json.Unmarshal` collapsed duplicate require_model keys and replaced malformed Unicode | Strict exact-byte policy preflight before unchanged parsePolicy; duplicate nested policy and Unicode/null/depth/trailing negatives |
| HSE-R3 | Treating every nonzero child exit as an execution fault lost valid complete denials | Fixed 0/1/2 decision binding, independent process-completion counter; actual process status fixtures and receiver wrong-exit tests |
| HSE-R4 | One-second WaitDelay could outlive a short selected child deadline if a descendant held pipes | 1 ms drain reserved inside the selected deadline; too-small budgets hold before invocation; actual inherited-pipe fixture refuses without the former one-second drain |
| HSE-R5 | Comparing only normalized state after another check could accept an intervening storage sequence | Exact pre-evaluation reload comparison precedes post-child HMT check/write; state drift regression holds |
| HSE-R6 | CUE branch constraints could supply omitted required false/zero values | Required-field-preserving branch constraints and generated omission parity for wire/report/controller references under CUE 0.16/0.17 |
| HSE-R7 | TrimSpace accepted blank-line prefixes/suffixes outside the promised peer frame | Exact object plus one LF framing; prefix, extra LF and trailing JSON negatives |
| HSE-R8 / INC-25 | A failed factory returned a nil pointer wrapped in a nonnil interface. Cleanup overwrote the child diagnosis; invalid native pin produced actual panic stderr | Explicit nil interfaces on all three preparation errors; three red/green factory assertions and actual native-negative CLI retest |

R1..7 were in the new development path and were repaired before source
publication. INC-25's native/store factory pattern was already present in
`a95225f9`; the actual private trial observed a panic, not an authorization grant
or source disclosure. R1's legacy startup originates in `8af29393`, retained for ordinary
commands. The original HC8 scanner in `07e28b00` was consolidated into
`internal/jsonwire` with unchanged legacy rejection behavior proved by HC8/HMT
tests. HMT's immutable bootstrap and authority logic originate in `a95225f9` and
remain the authority for this integration.

Fathom review_diff reported 185 transitive impact edges, 79 typed and 106 heuristic,
across the selected twelve-file analysis. Its response was truncated, the working
tree had uncommitted changes, and coverage, architecture rules and certifications
were not configured. That is a partial static impact receipt, not 185 runtime
callers or a security verdict. Legacy full-package regression and direct new-path
tests supply runtime evidence separately. The new wire is private to its two
command consumers and the HC8 adapter, with no public kit extraction.

## Misuse checks

Required work has no sampling flag. Optional controls cannot select another model
ladder, waive model/effort/fork/tier checks, change bootstrap or turn silence into
an allow token. Native run disables advice observations until a recipient exists.
Policy parse failure, unknown reply, drift, clocks, locked state, timeout, cleanup
ambiguity and output faults cannot yield a complete semantic result.

Current tests cover complete and malformed native used fields, unused nulls,
fork/resume/context-fork distinctions, repeated denials, advice rates, active
maintenance, exact source/state drift, clock-write failure, child exit/framing,
stdout/stderr caps, cancellation and inherited pipes. Native process tests use
the explicitly pinned test executable; their alternate dispatch exists only in
test source. A test-only race-runtime exit drain and macOS FIFO polling limitation
were diagnosed and corrected without changing production process policy.

Residual qualification: host delivery/crash/missing-hook behavior, protected
deployment/bootstrap, live user-presence/key authority, other-OS native approval,
physical power-loss behavior, production traffic and cost. Do not mark GT9..11 or
the complete governed-command objective done from these scoped proofs.
