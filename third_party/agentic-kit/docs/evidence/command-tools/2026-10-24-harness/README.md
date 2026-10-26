# Signed harness configuration: scoped qualification

GT8/HC-1..4, 2026-10-24. [0041](../../../intent/decisions/0041-harness-config-authenticates-selected-sources.md)
and [HC](../../../designs/harness-control.md) define the profile. This qualifies the
selected-source config checker, not native grants, state recovery or installed hooks.

| Instrument | Actual result |
|---|---|
| [Source pins](source-pins.json) | Exact command, test, schema, helper, skill and design bytes based on `82c6495`. No protected library root or module dependency changed. |
| [Native oracle](native-trial.json) | 47/47 owned synthetic Mac arm64 cases passed, using Go 1.26.3. Python cryptography independently signs DSSE bytes and computes the frozen snapshot digest; it is a qualification dependency, not a Go/runtime dependency. |
| [Report shapes](report-schema-trial.json) | All 47 actual native reports passed the closed CUE report contract, including failure privacy/code/count/ordinal constraints. |
| [Helper pass](helper-trial.json) | Exact 17-file source module snapshot passed focused race/vet on a separate Mac with Go 1.27.3; source bytes remained unchanged. This is not a full repository or other-OS gate. |
| [Independent security review](security-review.md) | Source review, failing counterexamples and final integration rereads closed the confirmed findings. Shared Fathom review was resource-refused; current source was inspected directly. |
| [Guide trial](guide-trial.md) | Both projects, all five authority kinds, mandatory/advisory results, documented negatives, help and overwrite refusal passed using only public guidance/schema. |
| Executable gate | `task validate` passed with format/vet/CUE/constitution and full race suites. Focused command coverage was 84.0%, a statement count rather than assurance of every behavior. |
| Plan proof | Fresh private-store ingestion and strict candidate verification: six claimed GT rows verified, zero failures. Static symbol/test reachability is separate from executed tests/native results. |
| Cross-builds | Windows amd64 and Linux amd64 builds passed. Their filesystem/native behavior was not executed or qualified. |

Final native binary SHA-256:
`911d8f0b526ee5c3eb8b24564e292fecf1e6f9946ab577d5ed1668f6f2ad25d1`.

The native corpus covers two projects with shared installation/org/team/personal
sources and separate project sources, all supported base64 forms, ignored/absent key
hints, wrong signature/key state/type/hash/target/revision, missing/expired/future/old
sources, mandatory weakening, unknown tuners, losing invalid preferences, empty range
and default conflicts, required organization/coverage/priority errors, case-alias roots
and candidate-owned registration, strict malformed signed payloads, calendar/registry
shape failures and in-project signed sources/tampering. Every run retained unchanged
before/after fixture content/mode/path inventories and empty stderr.

The independent digest oracle implements the exact HC-4 binary preimage rather than
calling the command's digest function. The signed malformed-payload cases retain valid
signatures/pins so they test decoding after authentication, rather than failing early
on unrelated signature errors. Earlier alias and unselected-shape failures are retained.

Raw synthetic fixtures, scripts, reports, source snapshots, old failing trials, native
and helper logs, guide receipts and gate logs are retained privately in the dated
`2026-10-24-harness-control` maintainer evidence directory. Public receipts omit host
paths and raw source bodies. No real keys, credentials, hook installation, global
configuration edit, provider call or paid inference was used.

## Claim limits

An operator must independently protect the registration, trust enrollment and
dispatcher. Code checks cannot attest that OS boundary or the actual actor. Signed
sources can be candidate-controlled for availability; changed bytes cannot become
authoritative without the independently pinned content/key/target. No full DSSE/PKI,
live revocation, complete organization-policy inventory or source-truth claim follows.

Existing roots are identity checked; case-fold aliases are conservatively rejected,
and absent unselected roots are permitted. Snapshot rereads cannot establish atomic
capture or detect ABA. Endpoint clock checks use wall time after stripping Go's native
monotonic metadata. Injected-clock tests and stdlib source inspection cover that fix;
no OS clock was changed. Persistent rollback/reboot resistance is still GT10 work.

Failure counts and ordinals are selected-configuration metadata for authorized
operators, not automatically public telemetry. Signature success is not permission
to publish or forward it. A writer failure may leave an already-verified prefix.
One-shot corpus timings are not production p95, peak-memory or savings benchmarks.
GT9..11 still own native approval, durable maintenance and actual host interception;
GT4/5/7/12..17 and the broader goal remain incomplete. PX workbench/UI acceptance
continues to be the flagship priority under 0038.
