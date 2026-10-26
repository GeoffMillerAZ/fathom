# Source-reviewed oracle floor receipt

HP-20/OQ-A08,2026-10-26. Implements0047 within0045's existing incubation and budget.
This is a command-local audit artifact, not a semantic engine, model calibration,
authentication or admission to spend. Old studies, labels and failures stay immutable.

## Inputs and command

`prose-lab review-floor-report` takes all existing `oracleCommonUsage` flags plus
`--packet-digest SHA256 --assessments PATH --floor-review PATH --future-scope PATH
--future-scope-digest SHA256 --fixtures DIR --review-dir DIR [--include-content]`.
All pins are explicit. It builds the current exact70 packet with existing loaders,
validates owner assessments using HP-19, and computes the metadata-only oracle report
with `oracleMakeReport`; its digest is SHA256 of compact Go JSON without trailing LF.
The floor receipt binds both raw assessments digest and that report digest.

Read only named bounded regular non-final-symlink inputs. Parent directories remain
operator-trusted. Read fixture/reviewer names only from validated ASCII IDs, never
caller paths inside JSON. No directory scans, output directories, clients, keys,
network, model calls, journal changes, clocks or effect authority.

Limits: receipt/scope/reviewer/fixture input<=1MiB each, <=8MiB aggregate bytes,
<=32fixtures, <=128future cases, <=256facts/case, <=8cited facts, <=8reviewers.
Text/reasons<=4096 UTF-8 bytes; refs<=256, IDs use existing validLabID.
Strict JSON: reject duplicate/unknown/missing/null fields, invalid UTF-8 and overflow.

## Exact wire contracts

Use closed structs/CUE and required literal fields in original JSON. All arrays are
required, including empty ones. All IDs within each registry are unique.

`prose-lab.oracle-floor-scope/1`: version,scope_id,criterion_id,criterion_sha256,
expected_case_ids,fixtures. Criterion is an embedded cited-entailment/1 or /2 with its exact pin.
The original packet/reviewer reports retain /1; the prospective scope may explicitly
select /2. A changed criterion invalidates prospective reviews and requires a
criterion entry in every affected old disposition; retain is then prohibited. `fixtures` contains {id,sha256}; bytes load from `<id>.json`
under --fixtures. SHA256 is over raw bytes, not a projection. Expected IDs equal
the complete union of fixture cases. No ignored extra/missing case is permitted.

`prose-lab.oracle-proposed-fixture/1`: version,id,cases. Each case is:
{case_id,document_id,assertion,cited_fact_ids,facts,expected,role,old_case_ids,basis,reviews}.
Facts contain {id,text,occurred_at,kind,required}, preserving all old fact data except
old artifact lineage (the scope supplies a new raw-file lineage). Kind uses existing
registered oracle fact kinds. IDs/citations resolve within the same case; no source
normalization or missing facts. Expected is supported|unsupported|inconclusive. Role is
ordinary|uncertainty-control|modal-control. An uncertainty control still requires
a held/unsupported strict-support answer, not silent removal from the workload.
`old_case_ids` names registered old cases; it may be empty for a new contrast.

Reviews contain {reviewer_ref,criterion_id,criterion_sha256,case_digest,status,
expected,premise_fact_ids,counter_fact_ids,basis}. Criterion and case pins must match
the prospective scope and actual case. Status is determinate|unresolved; expected is
supported|unsupported|inconclusive. Positive premises must be cited,
counters may use any case fact; at least one structured source basis is required.
Supported reviews require at least one cited positive premise. Fact kind is typed
data bound into the case digest, not inferred from a required prose prefix.
Unresolved review status or disagreeing answers hold the floor, never majority-vote
it ready. Two determinate Inconclusive answers can qualify an explicitly inconclusive
reference case; they cannot qualify a supported case or grant publication.
At least two distinct registered independent reviewers assess every proposed case.
These remain declared observations, not attested independence or identity.

Case digest uses compact Go JSON in the field order criterion_id,criterion_sha256,
case_id,document_id,assertion,
cited_fact_ids,facts,expected,role,old_case_ids. It excludes basis/reviews; fact field
order is id,text,occurred_at,kind,required. Retain supplied list order.

`prose-lab.oracle-floor-review/1` fields:
version,packet_digest,criterion_id,criterion_sha256,assessments_digest,
assessment_report_digest,future_scope_digest,reviewers,old_case_dispositions,
contrasts,remaining_ambiguities,verdict,provider_qualified,identity_authenticated,
publication_authority,new_study_admission. Four authority/admission flags are false.

Reviewers contain {ref,kind,role,exposure,assessments_digest,assessment_report_digest}.
Kind is human|agent; role is independent-source-reviewer. At least two distinct refs
are required. Exposure is an explicit nonblank statement, never verified by parsing.
Read `<ref>.json` under --review-dir as existing HP-19 assessments. Require complete70,
all row assessor refs/kinds equal that registered reviewer, packet/criterion binding,
raw file hash and re-derived metadata-only report hash. A reviewed old defect may
remain revise/ambiguous; complete review does not mean the old floor was ready.
With the unchanged /1 criterion, a positive marked ambiguous by the primary or any
registered reviewer must retain the original semantic input as a required
uncertainty control under an explicit uncertainty disposition. A new /2 criterion
instead requires the explicit changed-criterion disposition and complete newly
pinned reviews. It does not mechanically turn every old ambiguity into Unsupported.

Each old disposition contains {case_id,case_digest,disposition,prospective_ids,
changed_fields,basis,reviewer_refs}. Exactly all70 old IDs once, correct digests.
Disposition is retain|uncertainty|replace. Prospective IDs1..4 resolve in scope,
name the old ID in old_case_ids and have two registered reviewer refs.
Changed fields are unique label|citation|criterion|source|assertion. Retain requires
one prospective case with identical document/assertion/citation/fact values and
expected label, empty changed_fields. Other dispositions require nonempty changes
that cover every actual difference. Every criterion difference must be recorded explicitly; historical packet/report
identity still uses /1, never reinterpret the old failed run as a /2 result.
Uncertainty requires an uncertainty-control prospective case. Every old unsupported
obligation retains at least one unsupported prospective case and cannot be waived
into uncertainty. No old negative disappears through replacement or extra IDs.

Contrasts exactly OQ-C01..07, once each; {id,cases,reviewer_refs}. Case refs are
{case_id,case_digest},2..4 per contrast, unique and matching canonical future cases.
Each contrast has supported and unsupported sides (and may add an inconclusive
case), role modal-control (the original
disputed uncertainty control may also appear in OQ-C07), and two registered refs.
The names/discriminators come from prose-oracle-qualification.md, not keyword code.

Remaining ambiguities contain {case_id,basis}; registered old/future IDs, unique.
Verdict is held|ready-for-new-study-design. Ready requires no remaining ambiguities,
all proposed case reviews agreeing with the declared expected value, complete70
dispositions, all7 contrasts and complete independently declared review coverage.
The validator proves these structural/source-binding facts, not the semantic basis.

## Output and refusal

Default output contains version prose-lab.oracle-floor-report/1, input digests,
counts (old_cases,prospective_cases,contrasts,reviewers,remaining_ambiguities),
declared_floor_ready and the four false authority flags. Full content is opt-in.
Never emit private paths, assertions, facts, reasons or identities by default.
Exit0 means structurally complete declared ready-for-new-study-design, exit1 a valid
held receipt, exit2 malformed/incomplete/binding/IO/cancel/output failure.
Encode before writing; bounded output<=8MiB. Calls share no mutable state.

## Acceptance and execution

Owner independent tests plant omitted/duplicate old negatives, wrong case/source/
criterion/report hashes, missing contrast side, forged reviewer refs, uncited
positive premises, contradictory review/ready, stale future pins and budget/
authority flags. Go/CUE parity and actual CLI help/IO/privacy/refusals are required.
First source-review pass uses fresh independent readers with historical labels and
outcomes withheld; exposure remains declared, not a technical isolation guarantee.
All70 dispositions and7 reviewed contrasts precede new study design. The existing
exact70 runner is unchanged; a revised model cohort needs a separately frozen
intake/protocol and aggregate spend reservation. No UI entry follows from HP-20.

## Retained criterion defect and rejected binary floor

Independent v1 source review of all70 found69 matching historical labels and one
wrong positive. Both15-case contrast passes then returned an ambiguous confirmation
case, but one reader was unsure of the reference criterion while the other confidently
selected Inconclusive. Preserve both inputs/results and the held proposal.
Criterion2 makes clear epistemic strengthening Unsupported and unresolved material
source conflict Inconclusive. It does not assert the underlying fact is false.
The proposed floor now admits an explicit inconclusive reference target, separate
from an unresolved reviewer. A binary floor was rejected because it conflates those
two uncertainties. Re-review every prospective case under /2, not only the exposed
case. Keep the old packet, criterion, reports, failed trials and spend unchanged.

The complete reviewed prospective input is checked in at
`cmd/prose-lab/testdata/oracle-floor-v2/`. Its wire formats remain /1; its selected
semantic criterion is /2. Both separately instructed readers declare prior own /1
exposure and no technical identity/isolation attestation. Their agreement is not
model diversity, a human judgment, source authentication or calibrated accuracy.
