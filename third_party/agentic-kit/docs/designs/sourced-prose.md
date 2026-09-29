# Sourced prose v1 contract

Implemented in `kits/governed-prose-kit/sourced_*.go`; CUE: `schema.cue`.
Parent design: [governed workflows](governed-workflows.md), §§2–3.
This is an additive strict path; legacy `Pipeline` remains a bibliography/policy
composition, not a claim-support or authorization guarantee.

## Data and trust

- `SourceRef` = ID, exact version, lowercase SHA-256 of canonical UTF-8 bytes.
  `SourceProvider.Resolve` authenticates the principal and returns that exact snapshot;
  `Current` independently supplies the current pointer. Both must enforce access and
  retention. A source can be public or restricted, expired or revoked.
- Base `Fact.Span` reuses canonical `evidence-span`: full provenance, source ID,
  record ID = source version, record hash = source digest. `byte-range` uses its
  existing **inclusive** end convention; excerpt bytes must exactly equal Span.Value.
  Other locator kinds fail closed. PDF/cell extraction is an adapter-owned transform
  into a separately versioned text snapshot, not a claim that text offsets locate PDF
  bytes. Additional native locator formats remain outside v1.
- Derived facts name earlier inputs only; cycles, duplicates, missing inputs, mixed
  units, division by zero and silent rounding fail. `arithmetic/v1` supports exact
  rational sum, difference and ratio. They inherit source lineage through inputs;
  fabricated direct-source metadata on a derived fact is rejected. Numeric inputs
  and results are at most 128 characters of signed base-10 integers, decimals or
  fractions; exponents and base prefixes are refused before arithmetic. Derived
  grades cannot exceed any input grade or predate its observation.
- A claim names precise support and contradiction IDs. Its assertion must equal the
  text at each unit's `[start,end)` UTF-8 byte range. Every non-whitespace character
  has coverage; overlapping ranges, dangling or unused claims fail.
- Non-factual text requires explicit independent classification review. The generator
  cannot declare itself reviewed. Reviewer input includes the whole authorized fact
  set, permitting detection of contradictions omitted by the generator.

`ExactSupportReviewer` is the conservative reference policy: one exact quotation from
a non-historical fact, no unresolved current contradiction. It rejects paraphrases
it cannot establish. Caller-reviewed exact headings may be allowlisted. Richer model
or human review is injected and identified in every decision; it remains a semantic
policy, not deterministic source truth. Historical facts, including transitive
arithmetic ancestors, require richer review.

## Entry points and bounds

`NewSourcedPipeline` requires a client, source provider, reviewer and versioned config.
All exported Go string fields in strict requests, drafts, provenance and copied/hashed
artifacts must contain valid UTF-8, including identity fields and map keys. Reject
malformed bytes before JSON can replace them or any downstream callback sees them;
valid Unicode (including a literal replacement rune) keeps its exact bytes. Config,
resolved snapshot metadata and reviewer decisions receive the same check at entry.
Wire input must also encode Unicode scalar values: unpaired surrogate escapes are
rejected before JSON decoding can turn an unknown fact ID into an existing U+FFFD ID.
Valid surrogate pairs, literal U+FFFD and escaped backslashes remain supported.
Restricted facts are refused before model calls unless the consumer explicitly sets
`AllowRestrictedGeneration` for this configured recipient; read access alone is not
egress authorization. Client wrappers can apply additional DLP and destination policy.
Clock is injected; nil defaults to UTC wall time. `Generate` resolves evidence before
one bounded client call, supplies actual facts and a structured-output JSON schema,
then rejects incomplete/refused/tool
output and malformed/duplicate/unknown/trailing JSON. `CheckDraft` is the same gate
for structured prose generated elsewhere. Each runs semantic review independently
and rechecks evidence afterward, before issuing a passing receipt.
Transport/retry spend belongs at the injected client's attempt boundary; this API
does not certify opaque client-internal retries. All operation limits are explicit;
aggregate source, input and output bytes are capped. No hidden retained run history.

The artifact receipt binds exact input facts, draft, principal/topic, config and
generation prompt. Model is the requested identity; the base LLM response contract
does not prove the actual backend model. Receipts are inspectable hashes, **not signed
credentials**, causal model explanations, or proof that a source is true.

## Correction, publication and confidentiality

`InvalidateSource` copies the artifact and marks the reverse fact/derivation/unit
closure, clears approval and support, and blocks export. Old artifacts remain intact.
Regeneration is a new Generate/CheckDraft operation; unchanged units can be resubmitted
but all publication checks rerun. Source corrections detected through Current block
publication even if the caller forgot to invoke invalidation.

`Export` checks artifact/config identity, reruns source and semantic checks, requires a
consumer-supplied publication authorizer, then observes evidence again after approval.
The non-mutating authorizer binds actor, artifact/evidence/config digests, audience and expiry to
the consumer's approval mechanism. No external effect is performed by Export; an
atomic write with a target version belongs inside the consumer transaction/HITL path.
Prepare the entire export before proposing/committing publication; never consume an
approval inside the authorizer, because later rendering or freshness checks can fail.
Source observations and an external publication cannot be made globally atomic by
this library; downstream destinations must enforce their own version/revocation rule.

Public exports replace entire units dependent on restricted facts, transitively. The
entire artifact is conservatively tainted when any input is restricted: declared
support edges and generated IDs cannot establish absence of private influence. Use a
new public-only evidence set and new review/approval to publish public sections. The
sidecar omits tainted text, IDs, excerpts and review metadata. Public-only units map exact
rendered Markdown ranges to selected claims/facts and arithmetic ancestors, including
per-claim ranges after Markdown escaping. `VerifyProseExport` checks these byte links
and the rendered digest; it does not authenticate the expected digests. Markdown
escapes active HTML/links. Digests and redaction counts remain linkable metadata, not
an anonymity guarantee. Export authorization uses freshly checked supplied-draft
provenance, never caller-editable claims about a prior model/prompt. The complete JSON
export (escaped text plus sidecar) has its own aggregate MaxBytes limit, checked while
building as well as before return. Restricted exports still require the authorizer and authorized
source reads. The consumer owns encryption, retention, storage and access to artifacts.

## Executable acceptance

`TestSourcedGenerationRejectsUnsupportedClaims` rejects 14 when sources say 12 and 10.
`TestSourcedEvidenceFailuresBeforeModel`, `TestSourcedDraftFailures`,
`TestSourcedStrictJSON`, `TestSourcedReviewerFailsClosedAndCannotMutate`,
`TestSourcedCorrectionAndRedaction`, `TestSourcedExportRechecksAfterAuthorization`,
`TestSourcedArithmeticAndTransitiveInvalidation`, and
`TestSourcedCancellationAndBounds` pin negative cases. These are deterministic
implementation evidence, not a live-model quality sample or consumer benefit score.
`TestSourcedPrincipalCannotChangeBeforeAuthorization` rejects malformed principals
before source/model/review/authorization callbacks in Generate, CheckDraft and Export.
`TestSourcedCopiesAndDigestsRejectInvalidUTF8` covers nested provenance, edges and map
keys; Unicode-preservation and invalidation/transport regressions close INC-12.
`TestSourcedModelCannotNormalizeMalformedFactID` and the CLI's
`TestProseCLIRejectsMalformedUnicodeIdentity` exercise the shared private wire guard
before typed decoding (INC-13); bypassing only that guard makes both tests fail.
