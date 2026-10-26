# HP-13 document review: differential and trust-boundary review

Baseline af4d6548829612312a99c0fb060fec3a17aed767. The exact source manifest and
executed evidence are in [HP-13 qualification](../evidence/prose-headless/2026-10-25-adjudication/README.md).
Recommendation: admit this bounded offline review aid. The full local gate passed; no
confirmed source security blocker remains in the final independent review. This
is not authenticated human review or a production acceptance assessment.

## Change and blast radius

Six production files add two CLI verbs, bounded artifact reads, rebuilt canonical
fitness replay, platform-specific no-follow flags and CUE contracts. Two test files
exercise command entry points and trust/disclosure boundaries. No exported library,
dependency, hook, global configuration or existing generation protocol changes.

| Surface | Blast radius / risk |
|---|---|
| runDocumentReview | New runLab dispatch for two verbs; private read/output and declared-state interpretation |
| loadDocumentReviewPacket | Four retained files plus exact study/corpus/scope; no writer/client/journal port |
| assessDocumentReview | Reuses AssessProfile; its caller-declared trust cannot authenticate a human |
| read helpers | New command-local Unix/no-Unix flag selection; no existing reader changed |
| schema.cue | New closed packet/adjudication/report definitions; global #ID unchanged |

The existing documentLengthAssess from b24e54f and prior documentTarget/VerifyDocument/
RenderMarkdown owners remain unchanged. Profile replay uses the old sidecar time
for consistency and explicit review time for current findings. Source and provider
history are deliberately not revalidated. Canonical digests identify bytes rather
than authenticity or an atomic filesystem state.

## Confirmed findings and repairs

| Ref | Finding and counterexample | Outcome |
|---|---|---|
| DA-F1 | Actual content packet's s-01-body#c1 failed new CUE plain-ID constraint | Dedicated claim ID type matches existing document_wire.go producer; actual packet CUE regression passes |
| DA-F2 | syscall.O_NOFOLLOW was undefined on Windows | Tiny platform flags; Windows/Linux pure-Go cross-builds pass; native runtime remains unqualified |
| DA-F3 | CUE accepted agent-kind with human-method declarations | Intrinsic method/kind condition agrees with runtime refusal; independent negative fixture retained |
| DA-F4 | CUE accepted criteria-pass with a required length hold | Cross-field aggregate constraints agree with canonical runtime; independent negative fixture retained |
| DA-F5 | Eligible initially meant only observed-pass, hiding admitted unknown and producer context | Separate eligibility_rule and outcome preserve admitted unknown and independence-not-declared diagnosis |

No critical/high runtime authorization bypass was confirmed. These issues were
caught before commit. The trust-sensitive code keeps independent_acceptance and
publication_authority false even for a caller-declared criteria pass. Parent tests
show that self-consistent sidecar/receipt replacement cannot become policy.

## Method, tests and limits

Applied differential-review and sharp-edges to current source, baseline history and
known helper contracts. Root-cause review used systematic-debugging for the actual
schema and compiler failures. Fathom review_diff was stale for uncommitted files;
its unrelated baseline findings were not treated as this change's verdict. Final
independent source critic found no blocker after the repairs and did not run tests.
Actual test/build/CUE results belong to the parent's separate qualification receipt.
No coverage percentage, fuzz campaign, provider authenticity, human identity,
source freshness, platform-native qualification or UI quality was measured.

Local roots require operator custody. Unix rejects final-component symlinks;
other platforms retain pre-Lstat/opened-file identity checks before reading and
must be runtime-qualified before adoption. Metadata defaults avoid prose, private
identity and free-text reason disclosure; explicit content is a deliberate export.
A future consumer must verify identity/policy and revalidate exact effect inputs.
No report, digest, declared human method or exit0 grants publication authority.

## Required next evidence

Run the separately frozen one-cell long-document pilot only after its source,
executable, scope, rubric, prices, finite accounting and stop rule are selected.
Record actual human judgments after generation, never invent them before dispatch.
Full matrix, structural negatives, group/selection/disclosure and HP-A18 stay open.
