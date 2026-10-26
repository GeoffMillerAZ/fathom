# Prose support-review qualification

Current semantic-floor readiness is reopened by [0047](../intent/decisions/0047-qualify-oracle-semantics-before-prompt-tuning.md).
Historical fixed-label matches below remain recorded, but cannot establish
current oracle validity or present model-site adoption.

HP-11/12, 2026-10-25. Authority:0045/0046 and [HP-A03/14/17](prose-headless-proof.md).
This extends the admitted headless lab, not the command platform or a third slot.
It qualifies one configured support-review site before further positive documents.
It cannot close document quality, factual truth, publication or UI entry.

## Frozen scope and acceptance

Use the existing v2 corpus and separately pinned goal scope without changing either.
An additional owner-authored review scope pins the exact corpus and goal-scope
digests, all43 document-qualified negative IDs, and24 positive controls per the fixed
roster: two exact assertions, one paraphrase and one multi-fact synthesis per
document. Every negative uses its frozen mutated assertion
and sole target fact as its cited evidence; all fixture facts remain visible for
contradiction review. Controls bind their independently reviewed assertion and exact cited fact IDs.
Exact controls must equal their sole fact; paraphrase and synthesis truth is an
owner-reviewed oracle, never inferred from the generator or equality alone.
Independent review must confirm the oracles before paid dispatch.

The closed review scope is `{version: prose-lab.review-scope/1, id, corpus_digest,
goal_scope_digest, negative_case_ids, positive_controls}`. Each positive control is
`{id, document_id, kind, fact_ids, assertion}`, with kind `exact|paraphrase|synthesis`. Negative IDs are `document-id--negative-id`.
Require all negatives exactly once, exactly four positive controls per document with the stated kind counts,
unique globally safe case IDs, valid references and an exact67-case study roster. Bounds: positive assertions1..4096 UTF-8 bytes,
1..8 unique cited IDs (one for exact, at least two for synthesis), and at most
256 total cases. Positive IDs must use `document-id--p-` plus a safe suffix and
cannot collide with any negative. Different controls may cite the same fact.
Both scope files require explicit raw SHA-256 pins; no automatic pin selection.
Read bounded regular non-symlink files once and evaluate those same pinned bytes.
Scope is caller-reviewed data, not authenticated policy authority.

Expect `unsupported` for each planted unsupported assertion and `supported` for
each control. Inconclusive remains a safe hold but cannot satisfy this fixture's
specificity bar. Qualification requires all67 distinct planned cases completed and
matching, zero false support, zero false control holds and zero inconclusives.
Always holding fails. This small cohort does not establish population accuracy or
calibrated confidence. Structural-node classification, source truth, full-document
semantic goals, and actual effect authorization remain separate HP cases.

The first selected provider is Anthropic, one explicit reviewer lane. Validate
current official model identity, full-context reservation and price evidence before
freezing. The first run has a$5 local journal ceiling within0045's remaining$10
direct stage; prior failed/canceled/uncertain spend still counts. No automatic
retry, repair, alternate model/provider or fallback. Stop the cohort on the first
mismatch, incomplete transport, unknown usage, accounting refusal or interrupted
process. Retain all planned missing cases. A later protocol/model is a separately
frozen experiment, not a retry hidden in this denominator.

## Command contracts

Add `review-prepare`, `review-capture`, `review-report` to `prose-lab`. Common inputs:
study, v2 corpus, goal scope and pin, review scope and pin, selected Anthropic lane.
Prepare/capture additionally select a case. Validate all inputs and the full roster
before credentials, directories or clients. Require a1..8192 selected output cap.

Protocol: `prose-lab.support-review/1`. Compute the required study ID as `review-`
plus SHA-256 of a JSON string map with keys `protocol`, `corpus_digest`,
`goal_scope_digest`, `review_scope_digest`, `lane_id`, lexically sorted keys and
no insignificant whitespace or trailing newline. Existing `CanaryStudy`
shape stays unchanged; its ID and exact roster bind the reviewed fixture to the
raw study bytes used by the existing journal. Prices/model remain study data.

Prepare has no effects and prints fixed selected metadata, scope/study/request
digests and reservation. Its selected preparation digest binds the study, case,
lane and exact native request body. Obtain the request through the actual existing
`documentReviewer` and Anthropic encoder with in-memory capturing clients; never
copy or independently maintain their prompt/wire recipe. No real network or
credential discovery is permitted in preparation.

Capture requires the exact prepared digest, `--live`, a private aggregate journal
and private artifact root. Use the existing reserve-before-attempt client, native
usage/stop validation and strict reasoning-to-text projection. Check the exact native
request digest inside the attempt boundary before reservation or transport; retain
the post-response binding check too. A permanent exclusive
case marker prevents automatic replay, including held attempts. Before another
live call, crosscheck every prior marker, receipt and rationale against native
replay, exact preparation/pins, status, authority flags and per-case/cumulative
charges. A post-response artifact failure must prevent further dispatch. Persist fixed
receipt and private rationale; public output never prints assertion/fact/rationale
text or native reasoning. Reuse the actual reviewer, not a second interpretation
of its model output. Exit0=expected observation,1=semantic mismatch,2=incomplete.
Every result has independent acceptance/publication authority false. Partial write
or absent receipt is unknown/held. Failed and uncertain costs stay charged.

Report additionally requires `--artifacts` for an existing private artifact root;
it never creates that root. It is offline locked verification of an existing journal. Implement
a no-create audit opener: hold an existing private root, acquire its cooperative
lock, then read and validate the existing header under that lock. Never create a
missing directory/header or repair an outcome. Reuse a single factored history
reader for capture and audit, preserving strict writer refusal of unfinished
attempts. The audit may retain an intact reservation lacking an outcome as
pending/held, charged at its full frozen reservation; it cannot authorize another
call. Return an audit snapshot without a writer port. A locked or corrupt journal
remains unavailable with cost unknown, never zero, and no automatic unlock. No
provider credentials or real transport. Reuse all existing history/accounting checks. Verify each reservation/outcome: exact selected
lane/prices, case ID, one attempt per case, exact native request hash, completed
HTTP response, present usage, resolved model, frozen pricing and actual cost.
Re-run the actual Anthropic decoder, text projection and `documentReviewer` over
each retained native response with an in-memory transport that checks the incoming
body against its reservation and cannot reach the network. Do not trust a typed
receipt's claimed status instead of the captured native response.

Crosscheck completed artifacts against their native response; a forged typed
status cannot replace that response, and missing artifacts cannot qualify. Pending
reservations do not require a completed rationale/receipt. Only an absent outcome is
pending. Settled failed and unknown outcomes have distinct terminal statuses and
retain their full reservation; none permits another automatic call. Expose invalid attempts
as `invalid`, not `missing`, and retain their known charge. Output bounded fixed
per-case IDs/statuses and aggregate planned/completed/missing/invalid,
false-support/false-hold/inconclusive/pending/failed/unknown counts, total local usage-priced or reserved
spend and `matched_fixture_expectations`. Missing/foreign/duplicate/invalid attempts
cannot qualify. Exit2=incomplete/invalid,1=complete mismatch,0=complete match. The
report's local lock is explicit; it grants no tamper-proof or provider-signed
authenticity. Artifact/journal protection remains the operator's responsibility. A fresh journal
is a fresh accounting scope, not a study-wide replay fence or a reset of the
approved stage budget. The owner must retain all roots and attempts.

## Proof and exclusions

CUE owns the closed scope/receipt/report shape; Go owns validation and execution.
Tests use fake transports only. Parent-owned oracles cover omitted/duplicate/changed
scope, shortened roster, changed selected pin before effects, replay, all-hold,
false support, inconclusive, missing/foreign/duplicate attempts, forged typed status,
native request/response/cost/model tampering, cancellation, private output and zero
real network in prepare/report. Test header disappearance under the acquired lock,
intact pending reservations charged in full by audit, unchanged writer refusal,
and existing-lock refusal without clearing it. Match all actual emitted shapes with CUE, including
early failures. Full gate precedes commits, strict plan proof precedes checkbox claims.

Owner freezes source/binary/input/rubric digests before actual calls, reserves stage
headroom and records every attempt and stop. Public evidence contains sanitized
counts and exact identities; raw model/rationale/usage stays private. Rejected:
transport exit0 as semantic success, always-hold qualification, receipt-only
aggregation, semantic self-confidence as truth, silent replacement of bad oracles,
and bypassing mandatory deterministic or authority floors.

Official pricing source: [Anthropic pricing](https://platform.claude.com/docs/en/about-claude/pricing).
Reviewed2026-10-25: Sonnet5.5 input$2/output$10 per million tokens; this is source
price data, not an invoice or an account balance. Actual selected rates stay in the
frozen study. Revisit on any mismatch, cost uncertainty, protocol/model/input change,
new publication/quality claim or0045's budget/review trigger.

## First frozen observation, 2026-10-25

[HP-12 actual evidence](../evidence/prose-headless/2026-10-25-support-live/README.md)
records 67/67 matches, including all 24 supported controls and all 43 unsupported
assertions, with no missing/inconclusive/failed/unknown/invalid observations. The
source, executable, full scope, native request digests, order and stop rubric were
frozen before dispatch. Independent native re-decoding and repricing reconciled
all charges to $0.271122. The configured site qualifies for this frozen cohort.
No general accuracy, whole-document quality, source-truth, publication or UI claim
is promoted. Revisit triggers and subsequent experiment boundaries above remain.
