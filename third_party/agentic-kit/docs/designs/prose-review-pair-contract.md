# Paired support review command contract

Implements the offline portion of [HP-15](prose-review-scope-calibration.md).
This contract was selected before implementation; [offline qualification](../evidence/prose-headless/2026-10-25-paired-review/README.md) now records source proof. No live qualification or generation
wrapper is delivered by this source unit. Stdlib only, command-local and experimental.

## Inputs and identities

Add review-pair-prepare, review-pair-capture and review-pair-report to prose-lab.
Use runReviewPair/runReviewPairWith and the existing reviewDeps seam for deterministic
clock/httptest injection. Existing review-* and document-* paths stay unchanged.

Common flags: --study, --corpus, --goal-scope, --goal-scope-digest,
--base-review-scope, --review-scope, --review-scope-digest and --regression-source.
Prepare/capture require --case LOGICAL_ID and --variant baseline|candidate.
Capture also requires --prepared-digest, --journal PRIVATE_DIR,
--artifacts PRIVATE_DIR and --live. Report requires only common flags plus existing
private journal/artifact roots. --include-content explicitly discloses source and
oracle/rationale fields; default output excludes them. Report refuses case/variant,
prepared/live flags even explicitly empty. Prepare refuses effect flags even empty.
Exact help is key-free, succeeds without inputs, and never masks an unknown flag.

Study version prose-lab.paired-review-study/1 has required fields:
version, id, dataset_kind=synthetic, budget_micro_usd, prior_direct_micro_usd,
direct_stage_budget_micro_usd, review (existing lane), variants and
expected_execution_ids. No optional/unknown/null keys. Variants are exactly ordered
[{id:baseline,protocol:prose-lab.support-review/1},
 {id:candidate,protocol:prose-lab.support-review/2}]. One shared Anthropic lane
means identical model/settings/prices. Require valid lane, output1..8192,
0<budget<=5,000,000, 0<=prior, 0<stage<=10,000,000 and prior+budget<=stage.
Prior spend is caller-declared frozen input, not an authenticated account balance.

Scope version prose-lab.review-scope/2 is the independent root-owned fixture
testdata/document-scopes/v2-paired-support-review.json: exact base_scope_digest,
corpus/goal-scope digests and three extensions. Load the pinned legacy scope through
its existing strict owner. Keep all old67 controls unchanged. Each extension has
id, document_id, kind, role, fact_ids, assertion, expected and provenance.
Provenance is exactly source_digest, method and reason. The explicit bounded
regression-source file is the closed prose-lab.observed-review-regressions/1
artifact retained with the pilot. For the two observed extensions its raw digest
must match source_digest. Match each source row ID (extension ID is document ID
plus -- plus source ID), assertion, fact IDs, reference_judgment, method and basis
against its extension. Require source fixture_digest equal the raw registered
fixture digest and source fact records equal corpus facts. Its human_labeling,
changes_pilot_status and site_promotion remain false. The authored contrast instead
references the raw registered fixture digest and fixture-author method. Bind the
regression-source digest into common preparation/journal/replay. These checks prove
byte/declared-label lineage, never source or human authenticity. Require valid/unique bounded
IDs, registered document/cited facts, nonblank bounded UTF-8 text, exact one of each
role, one supported/two unsupported expectations, and two observed-development/
one authored-contrast entries. Roles are rhetorical-framing (supported),
document-scope-overclaim (unsupported) and material-scope-exclusion (unsupported).
Never accept caller-supplied replacement source facts. Provenance is declared,
not authenticated human identity. Default output excludes kind, role, expected,
provenance, assertions, facts and free-text reasons.

Logical cases sort lexically by ID. For each case emit baseline--CASE then
candidate--CASE as execution IDs, yielding an exact ordered140-entry manifest.
Reject duplicates, omission, foreign IDs, wrong order or invalid derived IDs.
This experiment enforces the exact root-reviewed scope and regression-source raw
pins, not an arbitrary caller-authored new cohort. Identical portable copies are
valid; a changed cohort needs a separately reviewed source/protocol revision.

The common study binding covers raw study, corpus, both scopes, both protocol
identities/exact prompt bytes and the whole manifest; it never depends on the
selected variant. Preparation displays requested model/protocol, corpus/goal/base/
review/regression-source pins and the declared budget/prior/stage caps.
Per-execution preparation binds that common digest, case,
variant/execution ID and exact real-encoder native request digest. Labels, control
and execution identities stay out of the provider payload. All corpus facts remain
available for contradiction checks. Baseline requests reuse reviewRequest unchanged.
Candidate changes only versioned support guidance. No effort/cache/model override.

## Execution, replay and output

Reuse bounded private regular-file readers, exclusive artifact markers, studyHTTP,
attemptRunner, locked journal and native replay. No subprocess, auth/config change,
automatic retry or provider fallback. Only capture --live creates roots/calls.
Before dispatch, reconstruct the valid completed prefix, native requests/responses,
receipts, model/usage/price and marker bindings. Reject a wrong next execution,
duplicate, missing/pending/unknown/invalid outcome or uncertain spend. Baseline
semantic mismatch permits its paired candidate. Any candidate mismatch or
inconclusive result stops later capture. Native/accounting failure in either stops.
Check new-journal and prior+cumulative+full-reservation stage caps before every call.
One selected journal mutex/lock serializes variants. Never overlap reservations.
An uncertain attempt retains its full reservation and journal lock. Reports refuse
that locked journal rather than silently clearing it or inventing zero cost; the
held capture/reservation retain the charge. No recovery/removal is automatic.

Report opens existing roots only and audits without a writer port. It retains
position, case, variant, execution ID and state for all 140 entries, including missing
positions. Keep per-variant complete/missing/false-support/false-hold/inconclusive/
pending/failed/unknown/invalid counts and token/cost/latency accounting. Global totals
must reconcile, and unknown usage stays charged/reserved. Detect extra artifact
files, partial writes, replaced pins, wrong marker/receipt/native data and out-of-order
history. Fixed diagnostics exclude raw input/path/native errors. Encode bounded
JSON fully before writing. Cancellation/encoding failures write no JSON; short or
failed writes return 2, but stdout bytes already accepted by a writer cannot be revoked.

Capture exits0 for a valid baseline observation or matching candidate,1 for a
known candidate mismatch,2 for inconclusive/unknown/refusal. Retain the private
receipt before a semantic nonzero exit. A baseline known mismatch is still an
observation; no command grants authority.

Integrity or accounting uncertainty dominates semantic mismatch. Corrupt or
unverifiable evidence, uncertain spend or missing required outcome/artifact for an
attempted execution exits2 and cannot claim qualification. A valid known candidate
mismatch with merely unexecuted later positions exits1. Never use a corrupt receipt
as a known failure oracle.

Report exit0 only when both variants have all70 known valid completions and the
candidate matches every oracle with no inconclusive/invalid/missing/pending/failed/
unknown result. Baseline semantic mismatches are retained observations. Exit1 for
a known candidate mismatch even with later missing rows. Exit2 for unknown/refused/
incomplete without a known candidate mismatch. Valid incomplete/failing reports
still emit JSON. No per-row oracle/match field is emitted by default; aggregate
comparison counts and native status enums are metadata. Include-content may expose
the oracle, source and rationale. All output always keeps independent_acceptance,
publication_authority and model/site adoption false. Cohort qualification is an
observation; no automatic workflow changes or publication follows.

Every data contract and actual output has CUE shape/refusal parity. Tests use fake
native transport and injected clocks, never providers/keys/network/sleeps. Parent
acceptance drives command entrypoints with independent cases. Preserve all 67 old
wire digests and replay results. Prove complete fake 140, baseline false-hold/candidate
pass, candidate failure/stop, pending/unknown spend, reordered/extra/mutated artifacts,
privacy/content, help, cancellation and broken output. Fake passes prove plumbing
only. Full gate/strict source proof precede source completion; the [actual paid trial](../evidence/prose-headless/2026-10-25-paired-live/README.md)
failed qualification; prospective generation remains unimplemented. Oracle
semantics must be qualified under0047 before further tuning.
