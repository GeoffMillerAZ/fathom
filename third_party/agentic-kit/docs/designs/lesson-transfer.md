# Lesson transfer: a bounded, receiver-controlled G7 contract

Status: implemented bounded v1 on 2026-09-13; two-repository delivery evidence lives
in the [delivery assessment](../assessments/2026-09-13-governed-workflows.md), not a
production-effectiveness claim. Authority: decision 0009 and decisions
[0005](../intent/decisions/0005-workflow-assurance.md) and
[0006](../intent/decisions/0006-evidence-and-lessons.md), and §4 of
[governed workflows](governed-workflows.md).

## Present hypothesis and boundary

Transfer one **public synthetic** lesson: unknown or partial evidence must remain
`unknown`/`fail`, never be promoted to `pass`. Its origin fixture is the strict
engineering coverage defect; its named receiver fixture is resume_as_code's unknown
provider-family path. This is retrospective extraction from already repaired defects,
not a claim that a lesson caused either fix. The evidence of transfer is independently
seeded negatives prevented in both repositories, not a count of lessons or a claim of
provider quality.

This is a portable-file protocol, not memory. A lesson never runs a command, changes a
receiver's policy, writes a receiver tree, grants credentials, or makes publication
authority. The receiver chooses whether to load it, supplies the reproduction callback,
and records any local remedy separately. Git/Fathom may distribute files but do not
authenticate their author or authorize execution.

Existing shapes help, but none is the lesson contract: `scenario-bank.Scenario` is a
useful versioned synthetic scenario; `eval-runner.RegressionResult` can report a local
before/after regression; `improvement-flywheel.Proposal` is only an advisory candidate.
They lack origin, redaction, receiver adoption, and revocation bindings, so G7 must not
serialize a flywheel proposal as a lesson.

## `lesson-transfer/v1` portable files

A bundle has three input files: `lesson.json`, `reproducer.json`, and
`attestations.json`. Supporting provenance/receiver reports are separate. All inputs
are UTF-8, single JSON values with duplicate and unknown keys rejected. JSON strings
must contain Unicode scalar values: unpaired surrogate escapes are refused before
decoding can change IDs, reviewer labels or invariant text. Valid pairs and literal
U+FFFD remain valid; raw-byte hashes still bind the original encoding. Maximum bytes
are 64 KiB, 256 KiB, and 64 KiB respectively; nesting is at
most 32. `lesson.json` is the payload; it contains no derived digest or attestation.
Its payload digest is `SHA-256(UTF8("agentic-kit.lesson-transfer.payload/v1") ||
0x00 || exact lesson.json UTF-8 bytes)`. `reproducer.json` is independently bound as
`SHA-256(UTF8("agentic-kit.lesson-transfer.reproducer/v1") || 0x00 || exact raw UTF-8 bytes)`.
`attestations.json` may name those two computed digests, but is excluded from both;
there is no self-referential bundle digest.

`lesson.json` contains:

| Field | Required binding |
| --- | --- |
| `version`, `id`, `revision`, `status` | `lesson-transfer/v1`; immutable ID/revision; candidate, reviewed, superseded, or revoked (adoption belongs to each receiver receipt) |
| `classification` | only `public-synthetic` for the first transfer; reject unclassified/private input |
| `origin` | workload name, immutable revision/snapshot digest, incident ID or bounded incident summary digest, and origin scenario/result digest |
| `applicability`, `exclusions` | exact supported conditions and contexts in which the receiver must not apply it |
| `reproducer` | schema ID/version, file name/raw digest, independently specified expected outcome, and resource bounds; never a shell command or executable path |
| `remedies` | one or more immutable remedy IDs/revisions plus exact artifact/patch/test digests; bytes are not auto-applied |
| `supersedes`, `revocation` | explicit prior revision labels; active/revoked state, optional reason, and required receiver recheck |

`attestations.json` holds the payload/reproducer digests, reviewer label, RFC3339
review time and scope. Reviewer strings are evidence labels, not authority:
only a receiver's trusted review-acceptance callback can accept a bundle for adoption.

`reproducer.json` is data only: `schema`, `version`, a closed input of
`{"case":"unknown-family"}` or `{"case":"unobserved-control"}`, expected
classification (`fail` or `unknown`), and named invariant. Unknown fields and arbitrary
input payloads are refused, not screened by a secret-shaped-string blocklist. Descriptive
labels remain untrusted; the receiving reviewer verifies rights and public-synthetic
classification before acceptance. No supplied string becomes an instruction or command.

Hashes establish exact byte linkage only. Origin snapshot, incident, reviewer,
provider-family, and receiver outcome are assertions of their named host; a hash does
not authenticate a host, prove an incident occurred, or establish semantic authority.
The receiving repository decides whether its local trust policy accepts those
assertions.

## Minimal implementation contract

```go
type Bundle struct { /* lesson-transfer/v1 fields above */ }
type ReceiverSnapshot struct { Workload, Revision, ConfigDigest string }
type ReproductionOutcome struct { Classification, Evidence, Detail string }
type Reproduce func(context.Context, Bundle, ReceiverSnapshot) (ReproductionOutcome, error)
type ReviewAcceptance func(context.Context, Bundle, ReceiverSnapshot) error
type CurrentReceiver func(context.Context, ReceiverSnapshot) (ReceiverSnapshot, error)
type CurrentRevocation func(context.Context, Bundle) (RevocationStatus, error)

func ValidateBundle(lesson, reproducer, attestations []byte) (Bundle, error)
func CheckAdoptionAt(ctx context.Context, b Bundle, receiver ReceiverSnapshot, accept ReviewAcceptance, current CurrentReceiver, revocation CurrentRevocation, run Reproduce, now func() time.Time) (AdoptionReceipt, error)
```

`ValidateBundle` is pure validation: bounds, strict JSON/CUE shape, lifecycle,
domain-separated raw-byte hashes and canonical reference digests. Required arrays cannot
be null; candidate/revoked/superseded records can be parsed but cannot be adopted.
`ValidateApplicability` checks exact workload/configuration inclusion and exclusions.
`CheckAdoptionAt` requires all four receiver-owned callbacks and an injected nonzero
clock (`CheckAdoption` is the production wall-clock convenience): it copies the
bundle and snapshot before every callback and copies callback output before retaining
it. It first checks current receiver identity/configuration and revocation, obtains
trusted review acceptance, invokes only the receiver-provided reproducer, then rechecks
current receiver snapshot and revocation. It fails closed on any callback error,
cancellation, missing evidence, unexpected outcome, changed snapshot, or
revoked/superseded bundle. Callback mutation affects only its owned copy, never the
validated internal value. It returns an owned receipt value; it does not persist it.
No callback or command is constructed from bundle data. A receipt may say `fresh` only
when both final rechecks ran; otherwise it is bound-only/unknown, never fresh.

The matching CLI is intentionally non-executing:

```text
kit-workflow lesson validate --lesson lesson.json --reproducer reproducer.json \
  --attestations attestations.json
kit-workflow lesson adoption-check --lesson lesson.json --reproducer reproducer.json \
  --attestations attestations.json \
  --receiver receiver-snapshot.json --assertion receiver-outcome.json
```

The second command validates bindings and emits a **pending, receiver-asserted**
receipt. It cannot label an adoption reproduced or measured because it did not invoke
the receiver callback. A receiver's Go test (or another locally owned integration)
uses `CheckAdoption`; local persistence, reviewer approval, and any remedy application
remain outside this CLI.

An `AdoptionReceipt` binds payload digest (which covers ID/revision/applicability), raw
reproducer digest, receiver snapshot, callback outcome and checked time. It records
separate `integrity` and `reproduction` statuses plus point-in-time `fresh`; outcome
evidence is a canonical SHA-256 digest, with detail bounded to 4096 UTF-8 bytes. It has
no persisted counters or measurement verdict: receiver test reports hold those claims.
Supersession/revocation cannot rewrite an old value; the receiver must recheck current
status before later recommendation/use. Receipt bytes are not an authorization token.

## Candidate acceptance fixtures

All public fixtures are synthetic. The origin fixture seeds a coverage receipt with a
required engineering control absent or unknown and expects non-pass in
`kits/lesson-transfer/agent_harness_integration_test.go`. The receiver fixture seeds an unknown or
partial provider family in `resume_as_code`'s kit-client/provider test and expects it
cannot qualify as an independent passing family. Each test must prove the seeded bad
state fails before the remedy and passes only after the receiver-owned remedy; neither
test may delegate its oracle to mutable bundle text.

Required negative cases: malformed/oversized/duplicate/trailing bundle JSON; payload,
raw-reproducer, or attestation binding mismatch; origin mismatch;
non-synthetic/private field; absent applicability or excluded receiver; forged
reviewer/host assertion treated as untrusted; missing/rejecting review acceptance;
callback mutation, cancellation, or error; unexpected `pass`; incomplete evidence;
altered receiver revision/config; stale final receiver/revocation check; revoked and
superseded bundle; and invalid evidence digests or clocks. Receiver tests must not
read a real incident, invoke a network/model, or execute a bundled command.
`TestLessonBundleRejectsMalformedUnicodeBeforeIdentityBinding` covers all three input
files with correctly rebound raw digests; disabling only the Unicode wire guard makes
it fail, so a hash mismatch cannot conceal the decoder defect (INC-13).

Measurements are bounded counters, not quality scores: seeded failures attempted,
non-pass outcomes before remedy, prevented false passes after remedy, false blocks,
adoption/review time, callback duration, and known/reserved/unknown cost separately.
`eval-runner` may attach its local regression result by digest, but repeated replay is
one implementation observation, not independent effectiveness evidence.

## Admission and limits

Start as one bounded `experimental` `kits/lesson-transfer` package: validation and
receiver-callback adoption only. Add CUE plus matched valid/invalid fixtures; do not
add a service, dependency, background sync, command runner, signature system, or
automatic remediation. The actual two-repository acceptance is independent seeded
negative receivers in agentic-kit and resume_as_code. If a receiver needs authenticated
producer identity or durable revocation delivery, that is a new authority/distribution
decision, not a v1 field.
