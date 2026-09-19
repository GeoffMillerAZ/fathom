# Fitness functions and assessment profiles

Legacy `Registry`/`Execute` score caller-supplied checks through `eval-gate`.
Registration and reads now retain independent check-slice snapshots. Use
`ExecuteTimedWithClock` for reproducible timing; `Execute` remains a pure projection.

For evidence-linked domain assessments, define a `Profile` and call `AssessProfile`.
A criterion has a stable ID, definition digest, required/scored kind, allowed evidence
methods and explicit N/A permission. Scored criteria have a positive bounded weight;
required criteria do not participate in weighted arithmetic.

An `Observation` binds the criterion definition, subject, outcome, method, reason,
artifact references and validity window. Retain the actual artifacts yourself.
Methods distinguish deterministic checks, empirical measurements, heuristics and human
review; allowing a method does not establish its calibration or authenticity.
Supply canonical UTC times (`t.UTC()`); non-UTC or monotonic-bearing timestamps are
rejected rather than producing a wire representation different from the input contract.

Required failures produce `fail`; missing, stale, mismatched or unknown required
evidence produces `unknown`. N/A needs explicit permission and justification. An
entirely inapplicable required profile cannot pass. Soft scoring is available only
after required checks pass and all applicable scored criteria are known. No applicable
scored criteria means a null score, not perfection.

The result includes a sorted rule trace and profile/subject/evidence fingerprints.
Same inputs and evaluation time reproduce the result. Digests identify typed canonical
Go JSON; they are not JCS, signatures, correctness proofs or external freshness checks.
This package executes no collectors, resolves no artifacts and authorizes no actions.

`Profile.Validate` and `Observation.Validate` enforce input contracts; malformed,
duplicate or foreign observations return typed errors. The corresponding CUE definitions
are in `profile.cue`. Existing stable `eval-gate` behavior is unchanged.

[Two-domain example](../../../examples/engineering-assessments/README.md) ·
[Design](../../../docs/designs/engineering-assessments.md)
