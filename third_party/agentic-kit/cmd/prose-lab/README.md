# prose-lab: opt-in headless evidence

An experimental development tool for the [HP proof](../../docs/designs/prose-headless-proof.md).
`prepare`/`capture` qualify **transport and accounting only**. The separate
`document-prepare`/`document-capture` commands produce checked positive document
candidates with the real engine and a distinct configured model support reviewer.
Neither path establishes independent acceptance, source truth, completed HP
qualification, publication authority or a production host. Remaining acceptance
is tracked in the [work order](../../docs/plans/prose-headless-proof.md).

`prepare` validates a synthetic study, exact lane/case and typed request without
reading credentials, creating a journal or calling a provider. `capture` additionally
requires `--live` and an explicit private journal directory. It uses the existing
Anthropic completion or OpenRouter/Jev typed decision adapter. There is no fallback,
automatic retry, streaming or installation. Cancellation uses the process context.

```sh
go run ./cmd/prose-lab prepare --study STUDY.json --lane generation --case canary --request REQUEST.json
go run ./cmd/prose-lab capture --study STUDY.json --lane generation --case canary --request REQUEST.json --journal PRIVATE_DIR --live
```

The caller resolves existing `ANTHROPIC_API_KEY` / `OPENROUTER_API_KEY` privately
into the child environment. Never put keys in arguments, study files, transcripts
or source control. Inputs must be owned public synthetic fixtures. Selecting
`dataset_kind: synthetic` is an explicit caller assertion, not a secret scanner.

Study data conforms to `schema.cue#CanaryStudy`. Prices are integer micro-USD per
million tokens and must include a pricing source. Model IDs/prices stay in selected
study data. Reservation rates and token ceilings must cover the selected model's
highest applicable price/context tier. The tool cannot establish a provider's
billing promise. Configure exact rates for the actual cohort, stop/review if usage
or pricing is uncertain, and retain provider invoices separately when available.

`usage_priced_micro_usd` is calculated from reported tokens and selected price
data, not a provider invoice. Input tiers price all token classes explicitly.
Resolved models must equal the selected model or an explicitly configured exact
`resolved_model` alias. Patterns/wildcards are refused; unknown identities cannot release spend.
Reserve the documented full model context at its highest applicable tier. The
wire-byte-plus-framing sanity check catches an obviously too-small reservation;
it does not certify a tokenizer or bound undocumented provider overhead. Jev's
profile requires documented free output, as its native wire has no output cap.

Before every wire attempt the journal exclusively creates and synchronizes a
reservation of the exact outgoing body and its parent directory. The exact body
and its digest are private, with no authentication headers. Completed captures price independently
present usage fields against the reservation's frozen lane. Absent/invalid usage,
transport errors, denied calls and uncertain attempts retain the full reservation
and stop the run. Zero reported tokens are distinct from missing fields.
Responses/usage and resolved model stay in mode0600 journal files. Successful
synthetic response bodies are private; provider error bodies are never persisted.
The ordinary output contains fixed status and aggregate accounting, not model text.

A single process owns `.lock`. A crash, unfinished reservation or overrun stops
automatic restart. Preserve the directory and inspect the prior process, request
and provider state before recovery. Removing `.lock` does not resolve a pending or
corrupt history. Changing study/budget data cannot reuse the same journal. A fresh
journal is a fresh accounting scope, so the owner must account for **all** journals
and Claude Code usage against the overall approved stage. A journal is not account
access control, a tamper-proof store or a provider-enforced spending cap.

The journal reserves space for a <=4 MiB outcome before dispatch and caps its
stored files at 64 MiB. Provider response bodies are bounded to 1 MiB; request
bodies to 64 KiB. At most 4,999 attempts fit the restart roster. Storage/count
refusal occurs before transport and cannot silently skip an attempted call.

The existing in-memory `llmclient/instrument.WithBudget` intentionally tracks
logical calls and reverses failed estimates. That contract cannot establish this
study's attempt-level, unknown-spend, crash/restart accounting. This command-local
journal is deliberately separate, not a replacement budget framework.

Exit0 means the selected preparation/capture completed. Exit2 means usage,
configuration, provider, budget, cancellation, journal or output prevented the
observation. No semantic quality verdict or exit1 quality result is claimed here.
App-kit AK-CLI-02's existing stdlib divergence applies: escaped plain JSON, fixed
diagnostics and no styled dependencies. Guidance came from its readable CLI guide
and maintainer checkout while the MCP kit-root entrypoint was unavailable. No new
shared renderer defect or app-kit source change is established by this utility.

Tests are entirely offline. They seed missing usage, error bodies, extra wire
attempts, wrong origins, concurrent owners, interrupted/replayed attempts, budget
changes, malformed history, symlink/private-directory and output/preflight cases.
These fixtures are not live-provider, actual-cost or generated-document proof.

## Positive document study

The frozen v1 [synthetic corpus](testdata/document-corpus/README.md) fixes six
document types, executive/technical audiences and three repeats: 36 positive cells.
Study `expected_case_ids` must exactly equal that manifest. Every fixture, goal,
required section/fact, negative seed and authored occurrence time is read and bound
before dispatch. Facts are source-author assertions in a fictional world, not real
records. Unknown source truth is not inferred from a support-model answer.

Choose two Anthropic lanes with disjoint exact requested/resolved model IDs.
The owner selects the stronger reviewer and records the model/pricing evidence;
a different name alone does not prove stronger quality. Generation currently uses
4096 output tokens per section; review uses the selected lane's output cap (1..8192 tokens), including provider reasoning. Each section is one
generated paragraph with scoped facts. This path does not qualify headings, lists,
tables, global/local change experiments, group publication or Jev comparisons.
Their contracts and tests remain separate acceptance work.

```sh
prose-lab document-prepare --study STUDY.json --corpus cmd/prose-lab/testdata/document-corpus --cell CELL_ID --generation-lane generation --review-lane review
prose-lab document-capture --study STUDY.json --corpus cmd/prose-lab/testdata/document-corpus --cell CELL_ID --generation-lane generation --review-lane review --prepared-digest PREPARED_SHA256 --journal PRIVATE_JOURNAL --artifacts PRIVATE_ARTIFACTS --live
```

Preparation emits the common `study_digest` and a separate `prepared_digest`
binding that study, exact cell and both lanes. Capture requires the reviewed
prepared digest before creating directories or constructing clients. A changed
cell/lane cannot reuse it. All cells share the common journal and aggregate budget;
do not create one journal per cell to reset spend. Freeze the actual binary/source
hash in the owner's execution record too. Protocol/template labels are not a
cryptographic assertion of which binary ran.

`PRIVATE_ARTIFACTS` is a distinct owned mode0700 directory. An exclusive permanent
cell marker prevents automatic replay, including failed attempts. The private
version JSON, Markdown and fixed receipt use mode0600 files with no overwrite.
A held attempt stays in the denominator. A deliberate bounded repair needs its
own explicitly recorded attempt/root and shares the total spend ledger. An absent
receipt or partially written artifact is unknown/held, never success.

For v1, `candidate_checked` means engine integrity, configured heuristic support,
required citation-ID presence and hard word-range checks completed. Citation coverage alone is
not semantic coverage. Every accepted output still needs independent evaluation
against all required facts, goals, contradictions, clarity and audience criteria.
`negative_cases_evaluated`, `independent_acceptance` and `publication_authority`
remain false. The available planted negatives are not executed by this command.
They require separate bound attempts/results before any negative-hold claim.

### Prospective length policy (v2)

The separate [v2 inputs](testdata/document-corpus-v2/README.md) preserve all six
scenarios, 36 cells and 43 available negative seeds. Five length ranges become
advisory; technical-design retains a required 1200..1600 range as a long-document
capability witness. This policy does not regrade or retry the failed v1 study.

V2 prepare and capture both require a separately reviewed goal-scope file and its
explicit raw SHA-256. There is no automatic pin selection. For these checked-in
inputs, the reviewed scope pin is:

```text
41988f2f526415763d64faa7193e477f1370c56bfd2a0e269427dbbf0c004403
```

```sh
prose-lab document-prepare --study STUDY.json --corpus cmd/prose-lab/testdata/document-corpus-v2 --cell CELL_ID --generation-lane generation --review-lane review --goal-scope cmd/prose-lab/testdata/document-scopes/v2-length-goals.json --goal-scope-digest 41988f2f526415763d64faa7193e477f1370c56bfd2a0e269427dbbf0c004403
```

Add the same two scope flags to the capture command above. Review the complete
study, six-goal metadata, long witness and prepared digest before opting into live
capture. A new scope or corpus invalidates the old preparation. V1 refuses either
scope flag, including an explicitly empty one. Scope hashes bind caller-reviewed
bytes; they do not authenticate organizational authority.

V2 uses runner protocol3, prepared receipt2 and run receipt2. Word counts use
`markdown-whitespace-fields/v1`: Unicode-whitespace fields in the full canonical
Markdown, including headings and citation markers. This is not a linguistic count
for every language. The runner verifies the rendered artifact before counting it.
It reports exact count, range and signed deviation. Advisory misses warn; required
misses hold. Byte, token, journal, artifact and spend bounds remain hard.

The private `<cell>.goals.json` contains canonical per-goal fitness profiles,
observations and assessments. Only length receives a deterministic observation.
Semantic goals require independent human evidence and remain unknown here.
`semantic_quality: unassessed`, `independent_acceptance: false` and
`publication_authority: false` prevent a length result from becoming acceptance.
No whole-assignment completion result is synthesized. A length warning can coexist
with `candidate_checked`; that status still requires separate semantic evaluation.

The checked-in v2 corpus qualifies offline policy behavior only. No v2 paid cohort,
negative-case evaluation, cost/quality improvement or UI-entry claim follows.

Receipts distinguish `cell_charged_micro_usd` from the shared journal's
`cumulative_charged_micro_usd`. The latter must never be summed across cells.
The accounting basis is usage-priced **or reserved uncertain** local spend, not an
invoice. A journal-open failure leaves cost unavailable, not zero. Reservations
include extra rounding headroom for separately priced input/cache buckets.
Actual wire usage, resolved model, attempts and latency remain in the journal;
core `GenerationRecord` counters alone are unqualified.

Authored synthetic `occurred_at` is separate from actual `observed_at` and source
acquisition. Repeats cannot change a March incident into an October event. Future,
missing or malformed authored times refuse. Actual source acquisition follows the
injected run clock; it does not prove a historical event or authorize publication.

## Canonical answer projection

The Anthropic study wraps both generation and review with a narrowly versioned
canonical-text view. The ordinary adapter retains unknown content for forward
compatibility. Default provider thinking therefore produced a real pre-review
hold despite valid JSON. The lab now removes only closed, validated `thinking`
and `redacted_thinking` metadata from the answer view after native capture. Empty
thinking text is valid for omitted display. Tool, refusal, malformed and genuinely
unknown blocks still hold. No reasoning text becomes a claim, support rationale,
source truth or authority. Raw responses and total output usage remain private.

The projection ID and runner protocol enter the study binding and receipts. A
protocol change cannot reuse an old prepared digest or journal. Prior failed calls
remain part of the owner's full denominator and aggregate stage budget. Turning
thinking off was rejected as a general solution: selected model families differ
in whether they allow it. [Provider contract](https://platform.claude.com/docs/en/build-with-claude/thinking)
and exact response receipts are separate from a workload-quality claim.


## Configured support-review study

`review-prepare|review-capture|review-report` exercise the existing support reviewer
against separately authored v2 controls. The reviewed scope has **67 cases**:
43 planted unsupported assertions and 24 supported controls (12 exact, six
paraphrases, six multi-fact syntheses). Always holding cannot qualify. This is a
configured assertion-support experiment, not full-document evaluation, source
truth, calibrated general accuracy or permission to publish.

Select the v2 corpus and both separately reviewed scopes for all three commands:

```sh
prose-lab review-prepare --study STUDY.json --corpus cmd/prose-lab/testdata/document-corpus-v2 --goal-scope cmd/prose-lab/testdata/document-scopes/v2-length-goals.json --goal-scope-digest 41988f2f526415763d64faa7193e477f1370c56bfd2a0e269427dbbf0c004403 --review-scope cmd/prose-lab/testdata/document-scopes/v2-support-review.json --review-scope-digest fe12741de1f91c0063a05172e4dd0e66bd580b40cf8afe937a4bd366086ecee8 --review-lane review --case CASE_ID
```

The [support-review contract](../../docs/designs/prose-review-proof.md) defines the
exact study ID and roster. Preparation uses the real reviewer and native encoder
without credentials or network. Its digest binds the exact selected request.
Capture takes the same common flags plus `--case CASE_ID --prepared-digest SHA256
--journal PRIVATE_JOURNAL --artifacts PRIVATE_ARTIFACTS --live`. It checks the
native wire against preparation before reserving or dispatching. All cases share
one protected journal and artifact root. Each case has a permanent attempt marker,
including holds. Stop on the first mismatch, incomplete observation or uncertain
cost. Do not change roots or retry to erase a failed denominator.

Report takes the common flags plus `--journal PRIVATE_JOURNAL --artifacts
PRIVATE_ARTIFACTS`; omit case, prepared digest and live flags. It requires existing
private roots, acquires a temporary cooperative journal lock, validates native
history and replays the real decoder, canonical text projection and reviewer
through an in-memory-only transport. It also verifies each completed marker,
receipt, rationale and charge against that replay. Typed receipts alone cannot
qualify a case. Reporting creates neither missing roots nor missing outcomes and
never reads credentials or calls a provider.

Missing cases, intact pending reservations, settled failed/unknown outcomes and
invalid attempts are distinct. Pending, failed and unknown spend retains the full
reservation. Invalid attempts retain known charges. Unreadable, locked or corrupt
history leaves cost unavailable rather than zero. Exit0 means every fixture
expectation matched, exit1 means a complete semantic mismatch, exit2 means an
incomplete/invalid observation. All outputs keep independent acceptance and
publication authority false. Scope pins establish byte identity, not trusted
ownership. Journal protection and accounting across every root remain the
operator's responsibility.

Offline fake-native tests prove runner and refusal behavior only. One separately
frozen [actual cohort](../../docs/evidence/prose-headless/2026-10-25-support-live/README.md)
matched all 67 controls with independently reconciled native usage and no retries.
That qualifies the selected support site only. Whole-document semantics and UI
entry remain open under HP-06 and HP-09.


## Offline document adjudication

The [HP-13 contract](../../docs/designs/prose-document-adjudication.md) prepares an
exact v2 document for review and replays caller-declared findings. It reads existing
version, Markdown, goals and receipt artifacts without provider access, credentials,
journal access, writes or repairs. A complete required-length-held candidate can
be inspected while retaining its hold. Partial/early holds refuse.

```sh
prose-lab document-review-prepare \
  --study STUDY.json --corpus cmd/prose-lab/testdata/document-corpus-v2 \
  --goal-scope cmd/prose-lab/testdata/document-scopes/v2-length-goals.json \
  --goal-scope-digest REVIEWED_SCOPE_SHA256 --cell CELL_ID \
  --generation-lane generation --review-lane review \
  --artifacts PRIVATE_ARTIFACTS --receipt-digest REVIEWED_RECEIPT_SHA256 \
  --at EVALUATION_UTC
```

Use the separately reviewed raw receipt hash. Packet digests bind all four artifact
files and the independently pinned criteria. Evaluation time is explicit canonical
UTC and does not change the immutable packet identity. Private read-only roots/files
are supported without chmod. Version integrity and canonical rendering establish
historical consistency; current source authorization and provider history are not
revalidated here.

Default JSON contains IDs, digests, length and criterion metadata. Only explicit
`--include-content` adds selected Markdown, facts, goals, claim coordinates, span
maps and review rationales. Protect that output as document content. It contains no
native provider response or hidden reasoning projection.

Report uses the same common flags with `document-review-report --packet-digest
REVIEWED_PACKET_SHA256`. Omit adjudication flags to report missing judgments as
unknown. Otherwise add both `--adjudication PRIVATE_FINDINGS.json` and
`--adjudication-digest REVIEWED_FINDINGS_SHA256`. The closed input names the packet,
declared reviewer ID/kind/relationship, and canonical fitness observations.
Observations must reference the document and Markdown digests and known semantic
criteria. Length is recomputed; an adjudicator cannot override its required hold.

Human-shaped declarations are still caller-declared evidence. Agent-as-human
claims refuse; heuristic, producer and unknown-independence findings cannot satisfy
the required independent-human methods. Missing, stale, future, wrong-subject or
wrong-definition findings remain unknown. Report exit0 means the selected declared
criteria pass, exit1 means a known required failure, and exit2 means unknown or
unusable evidence. Failure/unknown reports retain their per-goal states. Every
report keeps identity verification unperformed and independent acceptance and
publication authority false. The consumer owns trusted identity and any effect.

This command does not close HP-A14 document acceptance, negative execution or
HP-A18 UI entry. The next long-document pilot remains separately frozen and must
retain the full matrix's unexecuted cases and any unresolved human judgments.


## Paired support review (experimental)

`review-pair-prepare`, `review-pair-capture` and `review-pair-report` compare the
legacy and candidate review prompts over the pinned old 67 controls plus three
scope regressions. One shared study/journal binds 70 controls and 140 executions,
ordered lexically by control ID with baseline then candidate. All source facts
remain available. Source/schema qualification is not real model qualification.
The two pilot-derived labels are development judgments, not held-out human labels.

Offline preparation needs no key, journal or artifact directory:

```sh
prose-lab review-pair-prepare \
  --study cmd/prose-lab/testdata/paired-review-study.json \
  --corpus cmd/prose-lab/testdata/document-corpus-v2 \
  --goal-scope cmd/prose-lab/testdata/document-scopes/v2-length-goals.json \
  --goal-scope-digest 41988f2f526415763d64faa7193e477f1370c56bfd2a0e269427dbbf0c004403 \
  --base-review-scope cmd/prose-lab/testdata/document-scopes/v2-support-review.json \
  --review-scope cmd/prose-lab/testdata/document-scopes/v2-paired-support-review.json \
  --review-scope-digest f8d86a5366e2317f2a5dd6bc90fae074452a5d3c2176dccf9b1e589043ac6a49 \
  --regression-source docs/evidence/prose-headless/2026-10-25-long-pilot/regressions.json \
  --case architecture-decision--n-01 --variant baseline
```

The sample study's prior spend and prices are dated fixture data. Reconcile the
actual cumulative stage, select current prices and independently freeze exact
source/executable/study/scope/140-position order before any authorized live study.
The template and a valid preparation do not authorize calls or adopt the candidate.
This command qualifies only the exact reviewed scope/source bytes; a different
cohort needs a separately reviewed version, not an alternative caller-authored label file.

Capture adds the exact prepared digest, separate private `--journal`/`--artifacts`
roots and explicit `--live`. Only the next execution may run. A baseline semantic
mismatch still permits its paired candidate. A candidate mismatch/inconclusive,
native failure or uncertain accounting stops subsequent calls. A pending/unknown
attempt retains its full reservation and lock. Reports refuse that locked journal;
empty refused output is unknown, not zero cost. No recovery or retry is automatic.

Report takes the common binding flags and existing private roots, without case,
variant, prepared or live flags. It retains all 140 positions and separate baseline/
candidate counters. Exit 0 requires valid known completion of both variants and all
candidate expectations matched. A valid known candidate mismatch with only later
unexecuted positions exits 1. Integrity/accounting uncertainty, refusal or incomplete
work exits 2. Capture exits 0 for a valid baseline observation or matching candidate,
1 for known candidate mismatch, 2 for inconclusive/unknown/refusal.

JSON defaults omit prose, oracle labels, provenance and reasons. Explicit
`--include-content` discloses them for inspection. Encoding/cancellation errors write
no JSON. A short/error write returns 2; stdout bytes already delivered cannot be
revoked. Independent acceptance, publication authority and model/site adoption stay
false on every result. See the [contract](../../docs/designs/prose-review-pair-contract.md)
and [SC qualification plan](../../docs/designs/prose-review-scope-calibration.md).


## Offline oracle preparation and coverage (experimental)

HP-19's review-oracle-prepare and review-oracle-report inspect the fixed70
historical controls under cited-entailment/1 before further model tuning.
The [failed paired trial](../../docs/evidence/prose-headless/2026-10-25-paired-live/README.md)
and its labels remain unchanged. These commands execute no model, read no API key,
write no journal and grant no effect authority. They are an experiment-local
source/criteria audit, not an arbitrary-project semantic validator.

Use the same study/corpus/goal/base-scope/review-scope/regression binding flags from
the paired example above. Replace its command with review-oracle-prepare and omit
--case/--variant. The result contains all70 IDs and per-case digests. Explicit
--include-content adds assertions, historical labels, all facts and their exact
source pointers, and the embedded criterion text. Treat that projection as content.

Semantic packet/case identity excludes model, price, budget and filesystem paths.
A changed study is still recorded in study_digest. Source, criterion or premise
changes invalidate the packet; byte-identical evidence reused for another study
is not a new independent judgment or sample.

For a report, use those same common flags with review-oracle-report and add
--packet-digest REVIEWED_PACKET_SHA256 --assessments REVIEWED_ASSESSMENTS.json.
The closed prose-lab.oracle-assessments/1 input names the packet/criterion and
declared rows. Every row binds its case digest, disposition, interpretation, reason,
examined premise/counter IDs, basis, assessor kind/reference and prospective
citation IDs. Premises stay within originally cited facts; uncited context may
be considered as counterevidence, never silently substituted as positive support.

Assessors may record considered counterevidence and prospective suggestions without
erasing uncertainty. Only registered/document-scoped IDs are allowed. A complete
set of declared affirmations is still a declaration, not verified semantic truth.
Human-shaped declarations do not authenticate identity. The explicit false flags
for source/identity authentication, acceptance, publication, model adoption and
new-study admission remain false even on exit0.

Exit0 means70 structurally bound declared affirmations. Exit1 retains a declared
ambiguity or revision, including any other unreviewed cases. Exit2 means missing
coverage without a known issue, invalid input, cancellation or output failure.
Every valid report retains all70 rows and counts. Invalid input cannot be converted
to a partial success. Inputs are bounded regular leaf files; parent directories
are operator-trusted, not a filesystem sandbox. Content is opt-in.

The complete [synthetic held example](testdata/oracle-assessments-held.json)
uses packet digest
f5fc178e7fea3042e3e5ff8f70aa8b00046891b8f13f9961703a03908d198e65.
Pass that digest and example path with the common binding flags above. It exits1:
one declared ambiguity and69 unreviewed controls, with every authority flag false.
The example is a structural workflow demonstration, not HP-20's independent
whole-floor adjudication. Never reuse a packet digest after changing its source
or criterion.

For intrinsic JSON-path diagnostics before source binding, run:

    cue vet -c cmd/prose-lab/schema.cue REVIEWED_ASSESSMENTS.json -d '#OracleAssessments'

CUE checks shape/conditional laws; the Go report additionally checks the exact
registered roster, source, cited premises and digests. Neither proves the declared
basis is true. [0047 and the full contract](../../docs/designs/prose-oracle-qualification.md)
keep HP-20's independent whole-floor review, modal contrasts and any future paid
study separate. A valid schema, key availability or exit0 is not spending consent.
