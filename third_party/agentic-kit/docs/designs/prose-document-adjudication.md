# Bound document review packets and declared adjudication

HP-13/14, 2026-10-25. Implements the missing review intake within0045/0046 and
[HP-A14/17](prose-headless-proof.md). HP-12 qualified one configured assertion
reviewer; whole-document purpose, completeness, clarity and audience remain open.
This command-local offline slice composes the existing document and fitness owners.
It adds no generic review engine, identity provider, approval authority or UI.

```mermaid
flowchart LR
  A[Pinned corpus + retained artifact set] --> B[Rebuilt snapshot and criteria]
  B --> C[Bound review packet]
  C --> D[Explicit human or agent declarations]
  D --> E[Canonical fitness replay]
  E --> F[Declared pass / fail / unknown]
  F --> G[Consumer-owned identity and effect gate]
```

The final consumer gate is not implemented by this lab command.

## Selected decisions and trust

- **DA-1:** Verify and expose one exact retained v2 document snapshot for review.
  A complete candidate or a complete required-length-held candidate is inspectable.
  Early/partial holds cannot become reviewable by inventing missing artifacts.
- **DA-2:** Rebuild criteria from pinned corpus/goal scope. Use canonical fitness
  profiles/AssessProfile, never trust supplied sidecar policy or invent a score.
- **DA-3:** Human/agent reviewer kind and independence are caller declarations.
  A human method string cannot authenticate a person or prove actual inspection.
  Output trust=caller-declared, identity_verification=not_performed,
  source_revalidation=not_performed, provider_history_verification=not_performed,
  independent_acceptance=false and publication_authority=false on every path.
- **DA-4:** Missing, stale, wrong-subject/definition and disallowed-method findings
  stay unknown. Known required failure dominates unknown in aggregate, retaining
  all per-goal states. Advisory length cannot override semantic failure/unknown.
- **DA-5:** Default output is metadata only. Explicit include-content releases
  selected Markdown/facts/goals/sections/claim mapping and declared rationales as
  escaped JSON. Never serialize native provider responses or hidden reasoning.
- **DA-6:** Report exit0 means all selected declared semantic and mandatory length
  checks pass;1 means a known required failure;2 means unknown/refused/unusable.
  None establishes independent human acceptance or authorizes publication.

0046's next-study review remains a gate before the pilot: preserve the required
long witness, all semantic floors and the failed v1 cell/repair. No admission or
compatibility boundary is expanded. Free option: packet digests and canonical
criteria can be projected by a future app-owned workbench without owning its UI,
identity or effects now.

## Commands and exact inputs

Add document-review-prepare and document-review-report to prose-lab. Common flags:

```
--study PATH --corpus DIR --goal-scope PATH --goal-scope-digest SHA256
--cell ID --generation-lane ID --review-lane ID --artifacts PRIVATE_DIR
--receipt-digest SHA256 --at CANONICAL_UTC [--include-content]
```

Report additionally requires --packet-digest SHA256. Optional adjudication input
requires both --adjudication PATH and --adjudication-digest SHA256; neither means
no semantic observations. Prepare refuses these report-only flags, even empty.
Reject live/journal/unknown/positional flags. Explicit at is the injected evaluation
clock: canonical RFC3339Nano UTC ending Z, four-digit year, not a hidden current-time
fallback. Packet identity excludes at, permitting later review/freshness evaluation.

Reuse existing strict study/v2 corpus/goal-scope loading, exact36 eligible-cell
roster, distinct selected Anthropic lanes and documentStudyBytesV2/prepared binding.
No credential reads, clients, provider calls, subprocesses, journal access, root
creation, artifact writing/repair or global changes in either command.

Open an existing non-symlink artifact directory with os.Root, permission0500 or0700.
Read-only0500 roots and0400 files are deliberately supported without chmod. Reuse
existing bounded read helpers, with regular non-symlink files of permission0400 or0600, <=4MiB each,
<=16MiB total for the four artifacts, full bounded reads. Preserve every input.
Read receipt bytes once and require the explicit raw receipt hash before trusting
its fields. Require version2, exact cell/corpus/scope/study/prepared digests, lanes,
models and text projection. Required authority flags are false; semantic_quality
is unassessed. Status is candidate_checked without an error, or held with exactly
required_length_held. Require all four files:

| File | Cross-check |
|---|---|
| cell.receipt.json | Pinned raw bytes and exact recomputed capture metadata |
| cell.version.json | VerifyDocument, matching document digest and fixture target |
| cell.md | Exact bytes from canonical RenderMarkdown, matching Markdown digest |
| cell.goals.json | Canonical goals digest; recompute all profiles/results/counts |

Rebuild documentTarget from the exact fixture/cell and the retained facts' common
ObservedAt. Require all fact acquisition times agree, are canonical and <=at.
Compare the whole canonical target, not only goal or source IDs. This binds purpose,
audience, goals, constraints, section spec, source bytes/status/locators and authored
event times. It checks historical consistency, not current source authorization.
Run documentStructuralCoverage and VerifyDocument/rerender. Use the retained goals'
original evaluated time to recompute and compare the entire canonical sidecar via
documentLengthAssess; it must not be after at. Recompute current deterministic
length at at for the report. Do not mutate original observations or receipt status.
Opaque charges are not independently audited here; provider history remains unverified.
Digests establish coherence of the bytes read, not an atomic filesystem snapshot or
hostile namespace confinement. The operator must protect the artifact roots.

## Packet contract

Version prose-lab.document-review-packet/1. Canonical packet identity binds protocol,
cell/document IDs, corpus/goal-scope/study/prepared digests, raw SHA256 of all four
files, canonical document/Markdown/subject digests, exact semantic profile digests
and timeless length criterion/class/range/count/deviation/status/definition identity.
Never include a fitness Assessment or observation/evaluation timestamps in packet
identity. Use an explicit struct and jsonBytes, excluding
packet_digest itself, at, include-content and any optional content projection.
Changing any reviewed artifact or criterion invalidates the old packet pin.

Default packet includes these identities, candidate status, length criterion/class/
range/count/deviation/status, per-semantic-goal ID/definition/profile digests and
fixed trust/authority labels. The subject digest is the existing documentLengthAssess
subject. No Markdown, fact text, goal statements, instructions, reviewer identity or
free-text reason appears by default.

Explicit content contains purpose/audience/voice/technical level, canonical Markdown,
rendered node span map, fixture facts/sections/required goals and per-node claim
coordinates/assertions/fact IDs. Select these fields; never dump entire generation
records. Bound complete JSON output to8MiB before writing any bytes. Output errors
or cancellation return2; diagnostics use fixed codes without raw paths/input/errors.

## Declared adjudication contract

Version prose-lab.document-adjudication/1:

```
{version, packet_digest,
 reviewer: {id, kind: human|agent, relationship: independent|producer|unknown},
 observations: [canonical fitness.Observation, ...]}
```

Strict closed objects, no duplicate/unknown/null fields; file owner-only regular
non-symlink permission0400 or0600 and <=1MiB. Reviewer ID uses the existing bounded safe identifier rule.
There are0..32 observations, at most one per known semantic goal. Foreign or
length criterion IDs refuse. Observations retain canonical fitness bounds, reason,
artifact digests and UTC times. Their artifact references must include the canonical
document and Markdown digests; this remains caller-declared reference binding.
Header packet digest must equal both the supplied and freshly recomputed packet.
An agent claiming method human refuses. Other disallowed methods remain unknown.
Producer/unknown relationship observations are retained but ineligible for required
independent semantic findings; do not silently relabel their method as human or heuristic.

Rebuild each one-goal profile from corpus bytes and evaluate eligible observations
using AssessProfile at explicit at. Missing/invalidated observations project to
canonical unknown traces. Rebuild a combined semantic profile, and a combined
selected-criteria profile adding deterministic length only when length is required.
No confidence averaging or human-identity inference. Default report includes
packet/input digests, declared_goals_status, declared_criteria_status, safe per-goal
outcome/rule/eligibility metadata, separate length result and all fixed authority
labels. Free-text traces/reviewer identity are explicit-content only. All-hard-fail
and unknown cases retain complete accounting; report exit follows DA-6. Valid
unknown/failure reports are still emitted. Malformed/refused input emits a fixed
diagnostic and no partial packet/report. No whole Assignment completion is minted.

## Proof cases

DA-A01: actual command pass/fail/unknown and full CUE shapes, including content mode.
DA-A02: changed receipt/version/Markdown/sidecar/scope/cell/purpose/facts cannot reuse pins.
DA-A03: changed sidecar profiles/counts/observations cannot replace rebuilt policy.
DA-A04: omitted/stale/future/wrong-definition/wrong-subject/heuristic observations stay
unknown; foreign/duplicate/length criteria and agent-as-human refuse.
DA-A05: declared human+independent passes may yield declared pass but never authenticated
identity, independent_acceptance or publication authority. Producer/unknown stays unknown.
DA-A06: a complete required-length-held candidate is inspectable and remains failed
under all-passing semantic declarations. Advisory warning does not fail alone.
DA-A07: default stdout/stderr exclude fixture prose, goals, identities and rationale;
explicit content is escaped/bounded. Broken output and cancellation refuse.
DA-A08: missing/private/symlink/partial roots/files do not get created or repaired;
all original artifact hashes and directory entries remain unchanged; zero model calls.
DA-A09: canonical Go/CUE constraints agree on required/null/unknown/Unicode and byte bounds.
DA-A10: full task validate and strict plan proof precede completion; unchanged library
roots imply no new consumer-runtime or UI claim.

## Next bounded long-document pilot

After HP-13 proof, predeclare one technical-design/technical cell as a separate pilot,
not a36-cell repeat. Freeze the v2 corpus/scope, selected eligible-cell universe and
one-cell execution roster, exact source/binary/lanes/prices/budget, section/purpose/
audience rubric and stop rule. Use existing document prepare/capture; retain any
hold, missing section/fact, useless padding, absent headings or unknown human goals.
No automatic repair or denominator substitution. Capture one attempt; produce a
bound packet and independent review intake, with missing actual human observations
remaining unknown. A pilot cannot close HP-A14/18 or relabel the failed v1 study.
The full36-cell campaign requires its separately frozen acceptance/adjudication and
negative-execution protocol before dispatch. None is executed by this offline slice.

Rejected: accepting model-shaped human rows, synthesizing actual human observations,
trusting a sidecar as policy, rewriting old candidate acceptance/status, always-zero
quality exits, a new identity/approval system, automatic provider fallback, heading/
context redesign before observing the bounded pilot, requiring chmod of already
private read-only evidence, and shared UI components now.
Revisit on a false declared pass, disclosure leak, stale-pin reuse, required floor
relaxation, target/protocol change, first pilot result or0045's budget/review trigger.

## First pilot outcome

The [frozen one-cell run](../evidence/prose-headless/2026-10-25-long-pilot/README.md)
stopped before a complete artifact set. The actual review preparation refused it.
The first-pilot-result and0045 failure triggers fired. No required floor is waived.
[HP-15](prose-review-scope-calibration.md) specifies prospective scope/reviewer
calibration before a new generation study; this failed pilot receives no retry.
