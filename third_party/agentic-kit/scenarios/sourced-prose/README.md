# Sourced prose, approved once

Run the offline integration proof:

```sh
go test -race ./scenarios/sourced-prose -count=1
# Print the historical export and complete source map:
go test -v ./scenarios/sourced-prose -run TestRunSourcedProseApprovalAndRecovery
```

`sourcedprose.Run(ctx)` returns an inspectable `Report`. Its model, authenticated
editor, reviewer, clock, IDs and destination are synthetic; no network, secrets,
sleep or real publication occurs. Concurrent runs share no mutable state.

The independent fixture rejects **14** even when it cites current **12** and
historical **10**. It accepts “Count increased from 10 to 12, a difference of 2.”
only under the dedicated comparison reviewer. This is deliberately richer than
`ExactSupportReviewer`; historical comparisons are not an exemption from review.
An independent status sentence supplies the unaffected-unit control.

The composed path is:

1. `Generate`, `CheckDraft`, then `Export` with a non-mutating authorizer.
2. Atomically install the checked export only if the initial source/evidence,
   configuration and target version still match.
3. HITL `Propose`, JSON serialize/reconstruct the transaction store, and `Commit`.
4. Reject replay. Atomically correct the count to 13, invalidate the count facts,
   derived difference, dependent prose and pending approval; preserve status wording.
5. In an independent remote-publication run: `Prepare`, reconstruct, `Send` once,
   lose the receipt, reconstruct again, refuse resend, then `Reconcile` without
   another effect. Exactly one attempt survives; no generic exactly-once claim.

`Report.Export.SourceMap` maps rendered Markdown byte ranges to claims, exact
versioned facts, locators and derivation closure. Markdown escapes punctuation,
so rendered offsets differ from original assertion offsets. The comparison maps
to count v2 and historical v1; the status unit maps only to status v1.
`VerifyProseExport` checks transport integrity, not semantic support or truth.
`Report.Export` is the historical publication; `Report.Invalidated` records its
current invalidation. Historical export bytes are not fresh publication authority.

All sources and current pointers live in the same `TransactionStore.Target.Value`
as preparation, publication history and local effect count. The source provider
has no second mutable map. Correction changes pointers/facts, clears preparation,
invalidates the artifact and advances target/evidence versions in one transaction.

Bounds: four factual inputs, two prose units, three mock generation attempts per
Run, one source correction, one local effect and one remote effect. The configured
profile permits at most 8 facts, 4 units/claims, 64 KiB and 512 output tokens per
call; the reference store retains at most 16 decisions and 16 MiB per snapshot.
Production randomness, persistence, retention and authorization are host duties.

This proves an integrated synthetic workflow, **not** real consumer adoption,
source truth, model calibration, disk durability, or delivery guarantees at an
arbitrary destination. See [the adoption guide](../../docs/adopting-sourced-prose.md).
