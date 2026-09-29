# Jev opportunities for agentic-kit and Review Kit

Created: 2026-09-21. Updated: 2026-09-22 after independent Astra review, implementation and a synthetic OpenRouter smoke.
Status: implemented optional Review Kit lane; OpenRouter connectivity demonstrated; review quality and savings unproven.
[Design and criterion examples](../designs/typed-judgment-review.md) ·
[work order](../plans/2026-09-jev-review-and-routing.md).

## Implementation follow-through

The owner has authorized implementation for clients with and without Jev. Review Kit's
[owning design](../../../platform-engineering/kits/review-kit/docs/design/typed-judgment.md)
and [delivery ledger](../../../platform-engineering/docs/plans/review-typed-judgment.md)
now distinguish available code, offline protocol proof and remaining live evaluation.
The research recommendation below remains conditional on measured quality/economics;
provider access does not block implementing or fixture-testing the optional adapters.

## OpenRouter access update — 2026-09-22

The owner supplied access after offline delivery. Two tiny synthetic requests through
the actual Review Kit CLI reached OpenRouter's alpha Decisions API. Requested
`typesafe/jev-1.13` returned `typesafe/jev-1.13-20260917`; the first call was withheld
by exact-model validation, then the adapter gained explicit requested/returned pins.
The second call produced a valid advisory report, inspection and no-call reuse.
OpenRouter reported $0.000062244 across both requests. The
[retained evidence](../../../platform-engineering/kits/review-kit/docs/assessments/2026-09-22-openrouter-jev-smoke.md)
includes both attempts and the complete report; the key and repository source were
not transmitted as prompt content or retained in artifacts.

**The important negative result:** both responses marked a captured but uninformative
note as satisfied when the authored criterion intended unknown. An unknown option
alone does not produce reliable abstention. Missing-source capture already prevents
dispatch; missing semantic information inside captured text remains a model problem.
The next study must explicitly test applicability/sufficiency decisions, preserve
confident errors, and measure any extra-stage cost. No confidence threshold or
production policy changes on this three-question integration smoke.

The product recommendation is unchanged: invest in the criterion/evidence system and
compare replaceable backends. Access is now established for this account/profile;
quality, calibration, invoice reconciliation, retention for client data and routing
benefit remain separate proof obligations. The older unrelated funded campaign stays
deferred. Official [Decisions contract](https://github.com/OpenRouterTeam/go-sdk/blob/main/decisions.go)
and [endpoint identity/price](https://openrouter.ai/api/v1/models/typesafe/jev-1.13/endpoints)
were inspected on this date.

## Recommendation

**Make catalog-defined, typed judgment a first-class design concept, with Jev as the
first candidate backend.** The valuable product is a cheap, repeatable assessment of
specific criteria that agents and people can inspect and act on. Review Kit is the
natural first consumer. Agentic-kit's broader benefit is the reusable boundary between
a question, its evidence, the model's distribution, and the policy that uses it.

The owner's central hypothesis is strong: many recurring reviews need an actionable
checklist rather than a newly generated explanation. A catalog already supplies the
expectation, consequence, remedy and closure requirement. Jev can classify a captured
code or document slice against those expectations; code can produce useful text and
source links from the result. Escalation is an option when that feedback does not
resolve the issue, or when the rule needs deeper analysis. It need not follow every
failed criterion.

Research **two tracks in parallel**: criterion review and task/model/path routing.
Defect detection and routing are different abilities. Either might work when the other
does not. Share evidence collection and evaluation infrastructure, while measuring
their outcomes independently.

## What changed from the first assessment

| Earlier limitation | Revised conclusion |
|---|---|
| Narrow advisory-review pilot was the main opportunity | The main investment is an executable criterion catalog; review is the first application, with routing an independent second experiment. |
| Source-linked justification appeared to require another reviewer | Captured spans, typed reason codes and catalog-authored explanations can produce useful source-linked feedback in one decision stage. Semantic support remains a fallible judgment. |
| Routing waited for successful code review | Start routing design and offline preparation alongside review. Production switching still requires its own measured outcome and authority. |
| Existing Standard/profile contracts appeared almost sufficient | They supply important ownership and evidence foundations, but have no complete typed-question/answer contract. The actual gap is described below. |
| The comparison used one generative reviewer baseline | Compare the same criterion decomposition on Jev and a structured-output LLM, plus ordinary freeform review and deterministic-only controls. |
| The example centered on destructive dispatch control flow | Start with local semantic relationships. Multi-file security and concurrency remain difficult evaluation strata that normally escalate. |

## Verified provider facts

TypeSafe released Jev in early access on 2026-09-15. Its API accepts state and typed
questions: Noul gives a yes-probability, Choice a selected option and distribution,
and Score an ordered-rubric position. It does not generate new prose.
[Release](https://typesafe.ai/blog/introducing-system-one-models-and-jev),
[primitives](https://docs.typesafe.ai/primitives).

The documented Jev 1.13 input price is $0.042 per million tokens; outputs are free.
Limits are 64k tokens for the request and 32k for state plus the longest question.
Input is text. Pin a configured version before a trial and retain the returned
identity; a moving alias invalidates fixed-threshold assumptions.
[Model specifications](https://docs.typesafe.ai/models).

Question IDs are bookkeeping, not model-visible instructions. Choice allows at most
255 options; Score allows 2–10 levels. Questions in a request share state but cannot
consume each other's answers. The native endpoint returns typed answers and usage.
[API reference](https://docs.typesafe.ai/api).

Choice/Score confidence summarizes distribution concentration; Noul has no separate
confidence field. Neither concentration nor an explicit unknown option establishes
calibration on our code corpus. Thresholds belong to the exact question, option set,
model and workload. [Confidence](https://docs.typesafe.ai/confidence).

The vendor documents difficulties with arithmetic, dates, indirection, irrelevant
context and adversarial source content. Its published workflow comparisons use model
consensus references rather than independently adjudicated code defects. These support
investigation, not a claim of production review accuracy.
[Limitations](https://docs.typesafe.ai/model-jaggedness/jev-1.13),
[evaluation method](https://evals.typesafe.ai/).

## Actual repository fit

Inspected: agentic-kit `6c21e29` plus the pre-existing dirty intent edits, and
platform-engineering `6c1f9d7`. Fathom located definitions; targeted source reads
established the shapes below at the initial inspection. Agentic-kit's `task validate`
passed then. Subsequent Review Kit implementation and the narrow live smoke are
recorded above and in the delivery audit; the table is the original fit analysis.

| Existing surface | What it actually supplies | Proposed use / missing piece |
|---|---|---|
| [Review Kit core](../../../platform-engineering/kits/review-kit/internal/core/core.go) | Requirements have a check adapter and binding; finding definitions carry rationale, remedy, class and enforcement | Add a versioned semantic binding/profile under this owner. Existing finding rows are not already executable Jev questions. |
| [Review Kit finding events](../../../platform-engineering/kits/review-kit/internal/findings/findings.go) | Model producer kind, anchored source, payload digest, outcome, coverage, rule text/remedy/closure and history | Reference a retained rich decision receipt. Preserve model producer/tier; a content digest is not authority. |
| [Assessment profiles](../../foundations/quality/fitness-function-registry/profile.go) | Criterion definition digests, admissible methods, pass/fail/unknown/N/A and artifact references | Map evaluated results to heuristic observations; retain probability vectors and input lineage in the referenced receipt. |
| [Knowledge catalog](../../foundations/knowledge/catalog/schema.cue) | Authored taxonomy, stable keys, facets and source metadata | Preserve taxonomy integrity. Attach domain-owned criterion definitions; do not replace identity/merge validation with a model. |
| [LLMClient](../../foundations/llmclient/types.go) | Generative Complete and Stream request/response contract | A native decision call has different semantics. Propose a small local typed decision port, with Jev and structured-LLM adapters; review a shared export separately. |
| [Ladder](../../foundations/llmclient/ladder.go) | Caller-ordered escalation driven by a caller signal | Jev can inform the signal or task features; existing consumer policy retains eligibility and selection. Ladder is not a learned router. |
| [Review economics](../../../platform-engineering/kits/review-kit/docs/design/economics.md) | Manifest selection, freshness, reuse rules, measured work; no enabled paid model lane | Extend the optional lane with reservation, usage, cancellation and complete decision dependencies. |
| [Fathom integration](../../../platform-engineering/kits/review-kit/docs/design/fathom.md) | Code intelligence, report import, finding projection and contextual retrieval | Supply bounded packets and serve results. The real projection/edit-hook/recurrence tracer remains a separate proof obligation. |

Cross-repository links assume the sibling checkouts. Review Kit's leaf-module boundary
forbids casually importing the whole agentic-kit engine. Start with a local adapter
and portable receipts; extract a shared leaf only when a second concrete consumer
needs the same contract and conformance proves the fit.

## Opportunity portfolio

P1 means investigate now; P2 means a strong next use; P3 means retain the hypothesis
until its named condition is met. These rank expected value, not measured performance.

| Priority | Opportunity | Why Jev could matter | Success measure / boundary |
|---|---|---|---|
| P1 | Criteria-driven code and engineering review | Repeated semantic checks become cheap typed observations with authored feedback | Defect recall, actionable precision, repair time and total cost; retain discovery of uncataloged defects separately |
| P1 | Model and review-path routing | Classify task demands or select an eligible workflow before paying for it | Quality-constrained cost, routing regret, missed escalation and overhead versus fixed/rule baselines |
| P1 | Source support and evidence localization | Judge claim/span pairs or select supplied witness candidates | Support and localization accuracy; links resolve to captured bytes even when the judgment is wrong |
| P1 | Fast agent feedback through Review Kit/Fathom | A failed criterion plus exact source and remedy may be enough for the coding agent to repair it | Time to useful feedback, repeated-error rate, accepted fixes and distraction/duplicate burden |
| P2 | Retrieval relevance and context selection | Rank bounded candidate passages, rules, tools or skills without a generative explanation | End-to-end answer quality and retrieval recall; false rejection must remain visible |
| P2 | Recovery and orchestration | Classify missing context, provider failure, task ambiguity or insufficient capability | Useful completion under budget versus a simple fixed recovery policy; retries remain bounded |
| P2 | Catalog enrichment and health | Assign facets and candidate rule mappings; find ambiguity or likely duplicates | Maintainer time and accepted edits; catalog merge, floors and publication stay deterministic |
| P2 | Finding recurrence and lesson applicability | Judge whether a known failure/remedy applies to a new bounded context | Correct match/transfer rate; preserve originals and counterexamples |
| P2 | Evaluation triage | Identify likely failure class and select cases needing expensive adjudication | Misses in audited unflagged samples, reviewer time and disagreement rate |
| P2 | Operational/document consistency | Compare a specific runbook/ADR claim with supplied state or a named trigger | Correct, source-linked discrepancies; dates, counts and freshness computed in code |
| P3 | Prompt, tool-result and egress risk screening | Cheap narrow policy features ahead of independent controls | False allow/refusal and actual protected effect outcomes; adversarial input is an explicit test stratum |
| P3 | Broad security/concurrency or architecture verdicts | Decompose known patterns into local questions, escalating the remaining reasoning | Only pursue where packets and labeled outcomes support it; no global safety/absence claim |

The defensible asset is the curated criterion corpus, context recipes, outcome data and
evidence lineage. Jev is replaceable. A better backend should be able to answer the
same questions without rewriting organizational policy.

There is relevant public prior art. The authors of
[jev-review](https://github.com/NiazMorshed2007/jev-review) describe a local MCP wrapper
that sends focused context to Jev and returns dimension scores for the coding agent to
act on. [jev-router](https://github.com/gargpratyush/jev-router) describes routing at
fresh-turn boundaries, retained decision explanations and prompt-cache-aware policy.
These READMEs are design references, not audited integration or quality evidence.
Inspect and selectively reuse useful patterns before implementing equivalent glue;
our proposed value is organization-owned atomic criteria, exact-source records and
measured policy outcomes. No plugin was installed or executed.

## Source links and explanations are available at the cheap stage

There are three distinct products:

1. **Input provenance:** code binds a criterion result to exact captured files, ranges,
   revision and question/model versions.
2. **Structured explanation:** catalog-authored reason and remedy text is combined
   with a typed result. Jev can select supplied witness IDs or classify individual
   spans as support/contradiction/insufficient. No invented line number is needed.
3. **Established defect or closure:** a relevant test, authoritative producer or further
   adjudication supplies the strength required by the rule.

The first two are useful without the third. They must say whether a span was merely
supplied, judged supportive, or independently verified. A second LLM explanation can
help with a novel relationship; it does not automatically improve evidence strength.
TypeSafe's citation example uses deterministic quote matching with model support
judgments, illustrating this composition. Its example is not our validation.
[Citation cookbook](https://docs.typesafe.ai/cookbooks/citation_check).

The [design](../designs/typed-judgment-review.md) includes actual Review Kit rule IDs,
a structured request, an illustrative rendered finding, and the feedback/escalation loop.

## Economics to optimize

At the documented rate, 20,000 input tokens cost $0.00084 in model usage. Large overall
savings are plausible only if the approach reduces expensive review or increases
useful coverage enough to justify its overhead.

For (n) questions sharing a state of (S) tokens, with (Q) total question tokens:

- Separate calls send approximately (nS + Q) input tokens.
- One coherent batch sends approximately (S + Q).
- Concurrent individual calls can reduce latency but still repeat the state.

Illustration: 24 questions of 120 tokens over 6,000 state tokens cost approximately
$0.00617 separately or $0.000373 batched, excluding protocol overhead. This is arithmetic
about repeated input, not a measured quality or end-to-end speedup. Batch related
questions and possible routing/support questions speculatively when their inputs are
already present; use another request when an answer changes the needed evidence or
options. Avoid diluting a small question in a whole-repository packet.
[Batching pattern](https://docs.typesafe.ai/patterns/fan-out),
[qualified serial/concurrent comparison](https://docs.typesafe.ai/cookbooks/parallel_questions).

For a cascade, compare `Jev cost + extra preparation + escalations + audit samples`
with the direct review cost at matched quality. Track engineering/review labor separately
from API charges. The cheapest answer is still an eligible unchanged observation reused
with its original time. Missing price or uncertain attempts remain unknown cost.

## Evaluation that can change the recommendation

Review arms share the same captured changes: deterministic only; ordinary generative
review; the same typed criteria answered by a generative model; Jev; Jev plus selective
escalation. This distinguishes better decomposition from better model economics.
Record whether probabilities are native outputs, generated self-reports, or unavailable;
do not manufacture a distribution for an LLM adapter. Compare verdicts directly and
calibrate each backend separately.
Also compare supplied complete context with retrieved packets to isolate retrieval
misses from judgment errors.

Freeze development/holdout groups by independent change or repository. Adjudicate
positive, benign, missing-context, adversarial and uncataloged-defect cases. Audit a
sample of “no issue” outputs and the cascade's non-escalated cases. Prefer complete
independent adjudication of the frozen holdout; sampled operational audits need recorded
inclusion probabilities and appropriate weighting. Seeded challenge sets do not estimate
production prevalence. Grade cascade outcomes against independent truth; an escalated
reviewer's answer is an output under test, not the label. Freeze whether that reviewer
sees Jev's answer, and inspect anchoring with a blinded comparison when needed.
Report per-rule and per-severity misses, actionable precision, abstention, source localization/support,
calibration, p50/p95 latency, total usage, review effort and accepted repairs. Correlated
questions and repeated replay are not independent trials. Zero observed misses in a
small challenge set cannot establish a low production miss rate.

Routing gets its own corpus and controls: fixed inexpensive, fixed strong, a deterministic
task policy and Jev-assisted selection. Measure achieved task outcomes and regret against
the best eligible observed path, with full counterfactual runs only in an authorized
bounded study. Start with task-property classification plus deterministic policy;
evaluate direct Choice routing as an alternative. Selection confidence alone is not
evidence that the downstream model succeeds.

## Intent, current authorization and next move

Apply agentic-kit I1/I5/I11, P1/P7/P8/P12/P13 and decisions 0004–0006/0011–0014,
plus Review Kit R2/R4/R9/R10/R12/R18/R21 and ADR-0095/0098–0103. These require
domain ownership, honest evidence, bounded spend and preserved mandatory coverage.

The request authorizes this research and design, including a routing proposal.
Historical runtime-routing deferrals are not a prohibition on studying it. Review Kit
F4's shared catalog leaf and agentic-kit F7's health/cost routing are not automatically
promoted; a later accepted implementation decision must name the actual consumer and
evidence. A shared native decision interface would also need to reconcile the current
LLMClient “all model calls” wording and the platform's leaf dependency boundary.

Decision [0014](../intent/decisions/0014-offline-delivery-live-study-followup.md) now
explicitly defers live studies while allowing authorized offline delivery. The existing
operational-evidence implementation has advanced since the first assessment; its live
study remains a separate obligation. At that offline checkpoint no Jev calls, account
probes or source transmission had been made; the later authorized synthetic smoke is
recorded above. The immediate implementation proposal at that point
is a small offline criterion pack, native/fake decision seam and replayable feedback
example, with routing packets prepared alongside it. The [work order](../plans/2026-09-jev-review-and-routing.md)
states the separate evidence needed to enable each path.

### Offline delivery — 2026-09-22

Review Kit now implements the optional criterion lane in its working tree: no-provider
feedback, exact replay, native Jev and strict Chat Completions adapters, independent
egress policy, bounded attempt receipts, exact reuse, journal/outbox integration,
shared workbench artifact compatibility, bounded edit batches, and separate offline
review/routing studies. Twelve example criteria have 48 development cases.

The [delivery audit](../../../platform-engineering/kits/review-kit/docs/assessments/2026-09-22-typed-review-delivery.md)
records the actual gates, native loopback workflow and consumer proof. The approved
portal dependency repair is applied and its normal tests, vet, build and dependency
seams passed. The subsequent OpenRouter smoke establishes that narrow live path;
measured accuracy, savings, calibrated confidence, native Fathom hook delivery or
production model routing are not claimed.
This strengthens implementation readiness; it does not change the assessment's
conditional economic or quality conclusions.
