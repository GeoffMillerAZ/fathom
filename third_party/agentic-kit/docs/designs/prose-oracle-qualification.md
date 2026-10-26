# Source-bound prose oracle qualification

Selected by0047 after the [failed paired trial](../evidence/prose-headless/2026-10-25-paired-live/README.md).
HP-19 commands and the embedded criterion pass the full local gate and
[offline compiled qualification](../evidence/prose-headless/2026-10-25-oracle/README.md).
Fresh source verification proves the symbol, 28 statically reaching tests and
entrypoint wiring. HP-20 semantic adjudication remains unimplemented.
This is an offline addition inside0045's existing
prose-lab incubation, not another kit, provider study or UI. Existing document,
support-review and paired requests, schemas, digests and replay remain unchanged.

## Problem and law

A valid transport receipt cannot validate the expected semantic answer. The
runbook control's permission-after versus only-after disagreement exposed that
difference. Preserve the old fixed-label mismatch as measured, while recording
that the label's strict entailment is ambiguous. Do not train toward a convenient
answer before deciding what the evidence actually permits.

**OQ-1:** Positive support uses the cited premise set. Other exposed authoritative
facts remain available for contradiction and scope checks. A reviewer that needs
an uncited premise reports the gap; it does not silently repair the citation.
Source truth, source integrity, semantic support and effect authority stay separate.

**OQ-2:** Under cited-entailment/1, faithful paraphrase preserves actor, quantity,
time, modality, polarity, conditions, causal strength and authority. Wording
differences are allowed; adding a necessity, sufficiency, sole condition or
document-wide scope requires its own support. No unstated closed-world assumption.
Only-after expresses necessity, not a claim that expiry is the sole requirement.
Permission after a condition does not, by itself, formally prohibit every earlier
permission. Ambiguous normative language is unresolved.

**OQ-3:** All70 old controls remain in a source-bound inventory. Their original
labels are immutable history, not asserted new truth. A future revision carries
a one-to-one disposition and preserves defects/ambiguities as required controls.

~~~mermaid
flowchart LR
  A[Frozen corpus and old70 controls] --> B[Deterministic oracle packet]
  B --> C[Declared source-linked assessments]
  C --> D[Coverage and binding report]
  D --> E{All premises and criteria resolved?}
  E -->|no| F[Held with exact ambiguity or proposed revision]
  E -->|yes| G[Independent floor review and a new study freeze]
  G --> H[Separately measured provider qualification]
~~~

## Command contract

Add review-oracle-prepare and review-oracle-report to cmd/prose-lab. Reuse the
existing paired-input loader, bounded private readers, real source/fact resolution
and exact70-case pins. No new library, catalog owner, HTTP client or experiment
framework. One logical control per row, lexical order, independent of variants.

Common inputs are the existing study/corpus/goal/base-review/review/regression
paths and pins. Add an explicit criterion identity cited-entailment/1, owned by
this command version; bind its exact definition bytes. Prepare requires no key,
clock, journal, artifacts, live flag, network or writes. Reject effect/case/variant
flags even explicitly empty. Both commands have exact key-free help.

Prepare emits prose-lab.oracle-packet/1: packet_digest, source/study/corpus/scope/
criterion digests, planned70, row identifiers and per-row binding digests.
Semantic identity excludes the separate observed study digest. The packet
digest covers the complete canonical packet, exact assertion, historical
expected label, cited facts, every context fact, source/label provenance and
criterion bytes. Changing presentation cannot change its identity.
Criterion bytes are the complete UTF-8/LF/no-BOM contents of
cmd/prose-lab/oracle-criteria/cited-entailment-v1.txt, including its single final LF.
HP-19 embeds that file in the binary; no caller override or installed-source path
is consulted. Its SHA256 is bb12ed77442b05cc9912247881fce489a35d5de4749fb1efe364c5c2011acaac. A wording change
requires a new criterion identity/file and new packet; the old file is retained.

Label lineage is derived, not invented: old positives name the raw base-scope
digest and /positive_controls/i plus positive-control-means-supported/1; old
negatives name /negative_case_ids/i plus negative-control-means-unsupported/1,
and the registered corpus fixture /negative_cases/j/mutated_assertion.
Retain its /negative_cases/j/expected value and the explicit mapping rule too. Extensions retain
their existing source_digest/method/reason and /extensions/i/expected location.
Assertion lineage names its actual owner: a positive uses the raw base-scope
digest and /positive_controls/i/assertion; a negative uses its raw fixture digest
and /negative_cases/j/mutated_assertion; an extension uses the raw paired-scope
digest and /extensions/i/assertion. Observed extensions additionally retain their
bound regression-source lineage. Each fact uses its raw fixture digest and
/facts/k pointer. Indices are zero-based and resolved by registered ID. Lineage
records artifact_kind, raw artifact_digest, json_pointer and mapping_rule when
a label is derived. Absolute paths and fabricated human provenance stay excluded.

Use existing jsonBytes/json.Marshal compact UTF-8 encoding, including normal Go
HTML escaping and no trailing LF. Canonical case field order is case_id,
document_id, historical_expected, assertion, cited_fact_ids, cited_facts,
context_facts, label_lineage, assertion_lineage. Fact records retain every current
field, ordered by ID; cited_fact_ids retains its declared order.
Intrinsic case shape requires unique cited/context record IDs, equality of the
cited-ID and cited-record sets, and inclusion of cited IDs in context. Full fact
content, source membership and digests remain Go binding checks.
Case digest excludes its own digest. Canonical packet version is prose-lab.oracle-canonical-packet/1.
Canonical packet field order is version,
criterion_id, criterion_sha256, corpus_digest, goal_scope_digest,
base_review_scope_digest, review_scope_digest, regression_source_digest, rows.
Rows are the full canonical cases in lexical ID order. Packet digest excludes
itself and row-digest display fields. Study/model/prices and filesystem paths are
trace metadata outside semantic identity, so unchanged evidence can be reused
under a different study without pretending to have new adjudication.

### Wire projections

Prepare metadata fields are version, packet_digest, criterion_id,
criterion_sha256, study_digest, corpus_digest, goal_scope_digest,
base_review_scope_digest, review_scope_digest, regression_source_digest,
planned and rows. Each row has case_id, document_id and case_digest, with an
optional content containing its full canonical case. Both packet and report
also carry all six false authority/authentication/admission flags named below.

Explicit content projection adds criterion_text at the root, exactly the embedded
bytes, and full row content. Criterion_text is a display projection; canonical
identity already binds its exact bytes through criterion_sha256. It is never a
caller override and adds no extra canonical field. Report content additionally
includes each declared assessment, absent for an unreviewed row.

Default output omits assertions, facts, historical labels, free-text criteria,
reasons and assessor metadata. Explicit --include-content exposes the full packet
for review. Same digest in either projection; no dropped or replacement cases.

Report takes the same bound inputs plus --packet-digest and --assessments PATH,
opens existing bounded regular input files only, never creates an output root.
Limit assessments to1MiB and encoded output to8MiB. IDs reuse the paired-loader
ASCII bounds; assessor_ref is1..256 UTF-8 bytes and basis1..4096 UTF-8 bytes.
Each fact-ID list is unique and no longer than its document fact roster.
CUE and Go enforce byte lengths, not accidentally different rune limits.
The closed prose-lab.oracle-assessments/1 root has required version,
packet_digest, criterion_id, criterion_sha256 and rows (at most70).
Every listed row field, including the three fact-ID lists, is required. Each row has:

| Field | Constraint |
|---|---|
| case_id, case_digest | Exact registered case and prepared binding |
| disposition | affirmed, ambiguous or revise |
| interpretation | literal, faithful-paraphrase, normative-direction or unresolved |
| reason_code | sufficient-premises, missing-premise, scope-ambiguity, modality-ambiguity, contradiction or citation-mismatch |
| premise_fact_ids, counter_fact_ids | Unique registered IDs, tied to this document |
| basis | Nonblank bounded UTF-8 justification; no source replacement |
| assessor_kind, assessor_ref | declared human or agent, bounded identity reference; unauthenticated |
| proposed_fact_ids | Unique registered IDs for a prospective correction only; cannot rewrite the old case |

Every assessment names at least one premise or counter fact as structured source
basis; contradiction requires a counter fact. Premise_fact_ids for every disposition/label must be
drawn from the original cited set, while counter_fact_ids may use all context.
A proposed citation never silently supplies positive support. Affirmed rows have
no proposed_fact_ids; citation-mismatch revisions need a nonempty proposed set.
Ambiguous rows may carry prospective citation suggestions and remain unresolved.
Overlap between examined premises and counter facts is allowed for composite
claims, including affirmations; the tool does not infer an unresolved conflict
from list membership. The declared basis must explain the assessment. An actual
unresolved material conflict still violates the criterion; this structural tool
cannot establish whether that explanation is true.

Affirmed requires nonempty premises for supported controls, a resolved
interpretation and sufficient-premises for those positives; negative controls
need contradiction/citation-mismatch/missing-premise with an explicit source basis.
Ambiguous requires unresolved interpretation and scope/modality ambiguity or
missing premise. Revise requires a source-linked defect and proposed premise set
when citation changes. CUE enforces intrinsic shape/conditional laws; Go additionally resolves the
registered historical expectation, cited set, fact/document membership and raw
source bindings. Neither validator proves the semantic basis correct. Source-defined facts remain data,
not executable reviewer instructions.

Bind assessments to the packet before grading coverage. The command cannot prove
prior reviewer exposure or blinding; declarations about those are external
review evidence. Reject duplicate,
foreign, stale, wrong-document or extra fields/IDs, missing required values,
invalid UTF-8, final-component symlinks/nonregular files, oversized inputs and\nmismatched hashes. Parent directories remain operator-trusted; this is not\nfilesystem confinement, ownership verification or source authentication. Missing rows
remain unreviewed in the report, never inferred affirmed.

Report emits prose-lab.oracle-report/1 with the same identity metadata, raw
assessments_digest, planned, assessed, affirmed, ambiguous, revise and unreviewed.
Counters satisfy assessed=affirmed+ambiguous+revise and planned=assessed+unreviewed
=70. Every report row has case_id, document_id, case_digest and state
(affirmed/ambiguous/revise/unreviewed), plus optional content with canonical case
and declared assessment. Counts must also agree with all70 row states. It records declared_assessments_complete
only when all70 are present, and floor_declared_ready only when all70 affirm the
criterion without unresolved/revision needs. These are declared review coverage,
not authenticated truth, independent model calibration or spending authority.
Always keep human_identity_authenticated, source_authenticated,
model_site_adoption, independent_acceptance, publication_authority and
new_study_admission false. Default metadata privacy matches prepare.

Exit0: all70 declared affirmations structurally bind and no gaps remain.
Exit1: valid report with ambiguous/revise rows, including later unreviewed rows.
Exit2: unresolved coverage without a known defect, malformed/unverifiable input,
cancellation or output failure. Encode bounded JSON before writing; short writes
return2 without claiming already-delivered bytes were revoked.

## Independent floor review before any new paid study

HP-20 examines every old control against OQ-1/2 with source-linked declared agent
or actual human assessments. Independent review must consider the source rather
than copy the first assessor. Missing human evidence remains missing. No numerical
confidence or automatic majority vote makes a premise valid.

Freeze an old-to-new disposition for every control. An ambiguous old positive
becomes a mandatory uncertainty case; add an unambiguous positive instead of
silently weakening the denominator. Keep old negative obligations. Record all
changed citations/labels/criteria and why in a new source/version.
The current pair command remains exact70; do not feed it a revised roster or
resume its failed journal. A revised suite needs a separately specified intake.

Required contrast dimensions: permission after versus explicit prohibition before
confirmation; necessary versus sufficient condition; necessary versus sole
condition; expired versus confirmed expired; cited support versus uncited
context; normative definition versus observed success. Include the original
disputed row. No contrast's label is inferred from a lexical keyword detector.

### HP-20 review receipt

The lab maintainer owns prose-lab.oracle-floor-review/1, delivered with HP-20,
outside the HP-19 intake. It binds old packet/criterion/assessment-report digests,
the future scope and every raw proposed fixture digest. Include declared reviewer
kind/reference/role and outcome-exposure statements, never authenticated identity.

Its old_case_dispositions covers all70 old IDs exactly once, with old case digest,
retain/uncertainty/replace disposition, prospective IDs, source-linked basis and
every changed label/citation/criterion. No old required negative can be omitted.
Its required contrast manifest is exactly:

| ID | Discriminator |
|---|---|
| OQ-C01 | Permission after versus explicit prohibition before confirmation |
| OQ-C02 | Necessary versus sufficient condition |
| OQ-C03 | Necessary versus sole condition |
| OQ-C04 | Expired versus confirmed expired |
| OQ-C05 | Cited positive premises versus uncited context |
| OQ-C06 | Normative definition versus observed success |
| OQ-C07 | Original disputed runbook row and an unambiguous positive replacement |

Each contrast contains2..4 cases, each supplying pinned synthetic facts, assertion,
cited IDs, declared expected enum, source basis and independent review. A case may
reuse an old ID but must name it; each side of the discriminator must appear.
Unknown labels remain held. Record remaining ambiguities
and a declared held/ready-for-new-study-design verdict, with provider_qualified,
identity_authenticated and publication_authority always false. Limit the receipt
to1MiB, seventy disposition rows and these seven contrast rows. HP-20 supplies
closed CUE shape, complete-roster/binding checks and actual independently reviewed
data together; HP-19 does not validate or execute a future model study.

## Acceptance and implementation order

- **OQ-A01:** Prepare covers exact70 logical cases with stable source/criterion/case
  digests; explicit content projection changes no binding.
- **OQ-A02:** Report rejects changed packet, source, criterion, duplicated/foreign
  rows or fact IDs and invalid disposition/reason combinations.
- **OQ-A03:** All70 states/counters survive partial, ambiguous and revised inputs;
  missing data never yields floor_declared_ready.
- **OQ-A04:** Exact help, privacy, key-free operation, cancellation, bounds and
  broken-output behavior have actual entrypoint tests and CUE refusal parity.
- **OQ-A05:** Owner independently plants a missing-premise record and changed
  citation; the report holds both without altering the original scope or trial.
- **OQ-A06:** Full local gate and fresh strict plan/source proof precede HP-19
  completion. Offline/schema passes cannot close HP-20's semantic judgments.
- **OQ-A07:** Existing67 wires,70-case pair requests and retained reports replay
  unchanged. No altered default reviewer, new provider calls or UI.
- **OQ-A08:** A complete independently reviewed floor, old-to-new disposition and
  explicit new protocol/source contract precede a new paid freeze.

Implement OQ-A01..07 as one command-local unit; then perform HP-20 independently.
No new paid experiment is admitted by these commands.

## Evidence and alternatives

This design applies the separation of tasks, graders, transcripts and outcomes in
[Anthropic's evaluation engineering guide](https://www.anthropic.com/research/demystifying-evals-for-ai-agents),
which also warns that faulty tasks and graders can dominate results. Its
[success-criteria guide](https://platform.claude.com/docs/en/test-and-evaluate/develop-tests)
calls out ambiguous test cases. These sources guide the design, not a measured
qualification of this kit.

Reject prompt-only patches, retrospective label changes, removing the disputed
control, shrinking context to save tokens, model self-certification and a universal
formal-logic engine. Optional model suggestions remain held observations under0047;
no new classifier/provider integration is part of HP-19.
