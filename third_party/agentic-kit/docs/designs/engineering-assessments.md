# Composable engineering assessments

Status: implementation design, 2026-09-16. Decision: [0012](../intent/decisions/0012-composable-engineering-assessments.md),
extending [0011](../intent/decisions/0011-workload-selection-evidence.md).
Delivery and limitations: [verification](../evidence/engineering-assessments/2026-09-16/README.md).
This is one bounded incubation, reviewed by the kit maintainer on 2026-10-16 or
next release. It is not a commitment to populate every possible practice catalog.

## Product framing and findings

Agentic-kit should be an engineering decision system, not an encyclopedia of patterns.
For a workload, make it inexpensive to state requirements, test applicable alternatives,
inspect disqualifying failures, retain evidence, and revisit a decision after change.
The same lifecycle applies to model choice, retrieval, orchestration, safety and
other engineering concepts; their measurement methods and thresholds are not universal.

Existing ownership is useful and stays intact:

| Responsibility | Existing owner | Change in this slice |
|---|---|---|
| Hierarchy, facets, catalog identity and overlays | `knowledge/catalog` | no replacement |
| Provider/model facts and task escalation | model catalog and model ladder | no universal ordering |
| Named checks, profile projection | `quality/fitness-function-registry` | versioned profile and evidence envelope |
| Weighted boolean arithmetic | `quality/eval-gate` | reused, stable API unchanged |
| Trial accounting and paired uncertainty | `kits/eval-runner` | existing benchmark receipts consumed |
| Cheap topicality and public-endpoint admission | `governance/topic-gate`, `kits/endpoint-guard` | existing ownership preserved; topicality is not a safety classification |
| Artifact identity | `record-fingerprint` | reused; consumer retains artifacts |
| Tool authority and governed execution | existing security foundations and harness | screening must precede execution |
| Distribution and correction | lesson-transfer and consumer storage | no automatic policy propagation |

Current fitness checks are caller-supplied booleans, not executed tests. A function name,
tier or high weighted score does not establish that its declared invariant was measured.
The new assessment projection addresses that ambiguity without changing legacy scoring.

The existing topic gate already has a small-model judge, but its question is whether
text is on-topic, with a yes/no verdict and optional embedding shortcut. It is not a
versioned multi-category refusal policy. Keep public identity/rate/budget admission
before paid screening; compose or choose semantic gates based on measured benefit,
not duplicate every check. This slice does not change the stable topic-gate contract
or silently install another model call in an existing endpoint.

## Research and interpretation

NIST AI RMF profiles tailor risk-management priorities to use cases, requirements and
resources. The Playbook explicitly is not an all-actions checklist. This supports
applicable profiles, not a compulsory universal catalog.
[AI RMF profiles](https://airc.nist.gov/airmf-resources/airmf/6-sec-profile/),
[Playbook](https://airc.nist.gov/airmf-resources/playbook/).

OSCAL separates control catalogs, tailored profiles, implementations and assessments;
assessment results distinguish observations, evidence and findings. We borrow this
separation, not the entire platform, schema or a claim of OSCAL compliance.
[Layers](https://pages.nist.gov/OSCAL/learn/concepts/layer/),
[assessment results](https://pages.nist.gov/OSCAL/learn/concepts/layer/assessment/assessment-results/).

MITRE ATLAS publishes adversarial tactics, techniques, case studies and mitigations,
with separate content and format versions. Those identifiers can label a threat model
or test case; our inference is that a taxonomy mapping alone establishes neither attack
coverage nor mitigation effectiveness.
[ATLAS data](https://github.com/mitre-atlas/atlas-data/blob/main/README.md).

Model/RAG experiment methodology, grader bias, indirect-injection research and source
limitations remain in the [benchmark design](benchmark-evidence.md) and
[researched assessment](../assessments/2026-09-16-selection-evidence.md).

Anthropic's classifier research separates a policy constitution from trained classifiers,
and reports both attack outcomes and unnecessary refusals. Its later cascade sends
lightweight flags to a stronger check rather than treating every flag as final refusal.
Those trained/internal systems are not equivalent to this prompted adapter; no published
accuracy or overhead transfers to our code. This supports configurable review plus local
calibration, not confidence thresholds or claims of universal protection.
[Constitutional Classifiers](https://www.anthropic.com/research/constitutional-classifiers),
[next generation](https://www.anthropic.com/research/next-generation-constitutional-classifiers).

The provider's current guidance also describes lightweight input/tool-result screens
and least-privilege controls. This adapter uses the provider-neutral `LLMClient` seam;
it does not inherit the suggested model's quality or intercept tool results automatically.
[Prompt-injection mitigations](https://platform.claude.com/docs/en/test-and-evaluate/strengthen-guardrails/mitigate-jailbreaks).

## Assessment contract

A versioned profile identifies its domain and criterion definitions. Each criterion
declares required versus scored, allowed evidence methods, explicit N/A permission,
and a weight only when scored. A definition digest binds the collector/rubric/acceptance
meaning; the consumer must actually include these dependencies when producing it.

Findings bind criterion definition and subject digests, outcome, method, explanation,
artifact references, observation time and expiry. Methods distinguish deterministic,
empirical, heuristic and human evidence. Pass/fail/unknown/not-applicable are different.
Malformed envelopes, duplicates and unknown criterion IDs are input errors. Missing,
stale, future-dated or identity/method-mismatched findings produce explicit unknowns.

Required failure wins over unknown; otherwise any required unknown blocks readiness.
N/A needs permission, reason and evidence. At least one required criterion must be
applicable and pass; an empty or entirely inapplicable profile cannot look healthy.
A scoped soft score exists only after required checks pass and every applicable scored
criterion is known. Allowed N/A criteria are visibly excluded. No scored criteria means
no score, not 100%. Arithmetic delegates to the existing `Execute`/`eval-gate` path.

The result includes sorted rule-level outcomes, profile/subject/evidence identities,
caller-injected evaluation time, required status and nullable soft score. Reasons are
machine-identifiable. Identity is versioned, domain-separated canonical typed Go JSON;
it is not JCS, authentication, semantic equivalence, truth or external freshness.
Retain the profile, findings, subject manifest and referenced artifacts to reconstruct it.
Sensitive raw prompts need not be embedded in assessment records; retention and access
control remain consumer responsibilities. Hashes of low-entropy secrets are not redaction.

Bounds are part of the contract: finite positive scored weights, bounded collections,
bounded valid UTF-8 text, unambiguous identifiers and SHA-256 digests. Exact input
permutations canonicalize; materially different inputs change identity. No wall-clock,
network, randomness, model calls, storage or runtime authorization in this projection.

## Prompt screening and refusal standards

Treat an untrusted user's prompt as data for a small, eligible model to classify against
explicit categories and a trusted rubric. Do not let the request choose its trust level,
policy, classifier or exemptions. A cheap screen is an optional layer; it is not proof
that the request is safe and does not replace identity, authorization or output checks.

Policy carries a versioned identity, category definitions and deterministic actions.
Defaults must be useful without claiming universal coverage: attempted permission bypass,
protected-data extraction and instruction hijacking are refusal categories; uncertain
classification, invalid output, missing categories and classifier errors request review
and hold execution. Mere mention of security, harmful text in a quoted example, and
authorized defensive analysis must not be automatically refused as keyword matches.

Organizations may add domain/content restrictions, explicitly change category actions,
and choose a different error disposition within the documented safety boundary. A
model's self-reported confidence is not used as a calibrated decision threshold.
Results must identify the policy/input, category decisions and selected action without
requiring retention of private text or a persuasive generated rationale.

Callers supply an accessible model/client and explicit budget/timeout; no default public
model name silently becomes an eligible deployment. Screeners have no caller tools or
side effects. Required screening runs before expensive generation and protected effects;
parallel work cannot publish or act before checks complete. Native host coverage is
separate from this Go composition and must not be inferred from API support.

`PromptScreener` lives beside the existing `PreLLMChain` in `guardrails-pipeline`;
legacy optional-stage behavior is unchanged. Put data-classification/redaction/egress
controls before sending private text even to the screener. The supplied client's retry
or routing wrappers may make extra calls; one `Complete` invocation is not one billed
attempt. Preserve returned usage/attempt metadata and collect billing externally.
Timeout cancellation requires a cooperative client; no goroutine can undo an external
effect. CLI adapters' native capabilities and unenforced token limits do not become
screening guarantees. Defaults are security-specific, not comprehensive moderation;
provider policies and organization content standards remain independent constraints.

The opt-in `StructuredOutputs` configuration derives a closed schema from trusted
category definitions before request fingerprinting. It addresses an observed consumer
pilot format failure without relaxing the local parser. Schema support must be part of
deployment eligibility; the legacy default stays off. Changing this setting is a new
solution candidate, not a correction to earlier failed observations.

Evaluate false allows, false refusals, review/error rates, attack success, benign utility,
latency and all screening cost. Include adversarial attempts against the classifier,
indirect retrieval/tool content and cross-agent handoffs when those surfaces are in scope.
A detector alert is not a prevented action; assert real handler calls and released output.
Synthetic tests prove wiring and decision semantics, not production classification quality.

## First compositions and acceptance

1. Consume the retained resume_as_code synthetic benchmark receipt through the actual
   benchmark summarizer. Missing cost or non-live evidence stays visible; no model winner.
2. Exercise a security entry point with blocked and benign prompts, exact callback/handler
   counters, malformed responses and classifier failure. No blocked/reviewed request may
   reach the protected continuation. Include a deliberately disabled control as a negative
   control so a high benign score cannot compensate for an actual unauthorized action.
3. Regression cases cover required fail/unknown, stale/future findings, definition/subject
   mismatch, forbidden methods, permitted/disallowed N/A, all-N/A, duplicate/foreign IDs,
   nonfinite weights, bounds, deterministic identities, input immutability and CUE/Go wire
   compatibility. Full `task validate` and truthful downstream coverage remain required.

These references are intentionally small. They demonstrate independent domain collectors
feeding one assessment contract, not an endorsed suite of universal criteria.

## Next evidence, not speculative infrastructure

Next promotion evidence: a consumer records labeled benign/adversarial screening cases,
policy changes, reviewer agreement, all costs and an actual decision changed by assessment.
Then add a domain profile only when its applicability, evaluator and invalidation rules
are named. Compare simple/no-model baselines before adding architectural complexity.

Deferred: a hosted registry, universal optimization score, automatic provider discovery,
model auto-selection, authenticated evidence infrastructure, a red-team execution service,
hundreds of canned criteria, native-host interception and automatic lesson authority.
F5 and the other future triggers are not satisfied by this broader ambition alone.
