# Jev criteria and routing work order

Status: consumer implementation delivered; evaluation work remains proposed. Revised 2026-09-22 after Astra review and OpenRouter smoke. Owner: kit maintainer, with
Review Kit as the first review consumer and the existing consumer selection workflow
as the routing study home.
[Assessment](../assessments/2026-09-21-jev-opportunities.md) ·
[design and draft rule mapping](../designs/typed-judgment-review.md).

The owner subsequently authorized offline-first implementation and configurable
provider support for clients with and without Jev. Review Kit now owns that delivery:
[consumer work order](../../../platform-engineering/docs/plans/review-typed-judgment.md),
[contract](../../../platform-engineering/kits/review-kit/docs/design/typed-judgment.md).
JEV-01/03–05/08 map to consumer TJ-01–07/09; JEV-02/09 map to TJ-08. Preserve the
independent live-study rows below. Local consumer implementation does not by itself
promote a shared agentic-kit API or prove model quality. This research-level checklist
stays open until each broader acceptance scope is met; use the consumer ledger for
implemented symbols and runtime proof.
Decision [0014](../intent/decisions/0014-offline-delivery-live-study-followup.md)
preserves the live-study deferral while allowing authorized offline delivery.
The owner subsequently authorized OpenRouter access for a separate, two-request
synthetic integration smoke. [Its receipt](../../../platform-engineering/kits/review-kit/docs/assessments/2026-09-22-openrouter-jev-smoke.md)
records API cost, a model-identity refusal and a missing-information error. It does
not resume the earlier campaign or satisfy JEV-06/07. Further quality/routing studies
require their own frozen scope and budget; retained earlier reservations are not a
new Jev budget.

## Objective and sequencing

Test whether catalog-driven typed judgment increases useful review coverage and reduces
cost/time, and independently whether Jev improves task/model/path selection. Share packet,
receipt and evaluation mechanics. Neither track requires the other to succeed.

```text
JEV-01 criterion inventory ----+--> JEV-03 packets --> JEV-04 adapters --> JEV-05 offline proof
JEV-02 evaluation protocols --+                              |
JEV-09 routing design --------+                              |
                                                             v
                                JEV-06 authorized live study / separate track protocols
                                         |                         |
                                JEV-07 review decision      JEV-09 routing decision
                                         |
                                JEV-08 optional integration
                                         |
                                JEV-10 delivery / shared-contract disposition
```

A small engineering pilot may reject a poor fit quickly. Production-quality claims
need enough representative independent observations for their stated error bound.
Do not impose production statistical claims on an offline prototype, or promote a
small successful prototype as proof of those claims.

## Work and evidence

- [ ] **JEV-01 Criterion inventory.** Refine the ten existing rule mappings in the
  design into reviewed atomic definitions. Start with the six local semantic cases;
  retain the four harder cases as separate strata. For each child criterion, supply
  positive, benign, incomplete and adversarial examples, context recipe, outcome
  mapping, authored reason/remedy and closure requirement. Retain deterministic-only
  controls. Output: a versioned Review Kit-owned draft pack and a parent-rule coverage
  map that cannot turn one passing child into a whole-rule pass.

- [ ] **JEV-02 Two frozen protocols.** Define separate review and routing corpora,
  development/holdout groups, case/label provenance, adjudication, sample counts,
  permitted data, stopping/budget rules and practical targets. Review compares
  deterministic only, ordinary generative review, structured LLM criteria, Jev criteria
  and Jev plus escalation. Routing compares fixed inexpensive/strong paths, deterministic
  policy, Jev task features plus policy, and optional direct Choice. Freeze thresholds
  before holdout. Include source-injection, confidently wrong, missing-context and
  uncataloged-defect cases. Output: protocols with uncertainty and cluster treatment,
  label disagreements and an explicit distinction between pilot and promotion evidence.
  Prefer full independent holdout adjudication. For sampled operational audits, record
  inclusion probabilities and the weighting/uncertainty estimator; seeded challenge
  performance and production prevalence estimates remain separate. Freeze the inputs
  seen by escalated reviewers and grade their outputs against independent truth.

- [ ] **JEV-03 Packets, source support and receipts.** Bind subject/revision, exact
  captured and transmitted bytes, source/witness IDs, question/catalog/policy digests,
  visibility, redaction, dependency/index state, model identity, observations, usage and
  timestamps. Keep input anchors, model-judged support and independently verified proof
  distinct. Raw distributions belong in retained receipts referenced by existing
  finding payload/profile artifact digests. Output: CUE-owned bounded shapes and
  mutation fixtures for stale source, wrong question, missing closure, altered witness,
  permission change and invalid reuse. Exact model aliases or unknown remote state
  cannot silently qualify historical results as current.

- [ ] **JEV-04 Native decision experiment.** Add the smallest consumer-local typed
  decision port with fake/cassette, Jev and structured-LLM adapters. Native decisions do
  not need invented streaming semantics. Verify missing/extra question IDs, wrong
  types/model identity, invalid probability distributions, unknown option, oversize
  response, deadline, cancellation, 429, bounded retry and unknown-cost accounting.
  Wire required egress and reserve-before-dispatch controls explicitly; existing
  LLMClient wrappers do not automatically cover the new port. Output: deterministic
  conformance and failure fixtures, no paid calls in tests or core offline review.
  Preserve native/generated/unavailable probability provenance; a structured LLM's
  missing probabilities do not justify fabricated distributions. Compare verdicts
  across backends and calibrate any confidence-based policy separately.

- [ ] **JEV-05 Offline end-to-end prototype.** Produce captured-source checklist,
  authored feedback, witness links, disagreement and targeted escalation records
  through normal consumer paths using fake responses. Prove pass/fail/unknown/N/A
  semantics, mandatory-check preservation and model-tier preservation. Simulate an
  agent repair and recheck against the original closure requirement. Compare serial,
  concurrent-single and coherent-batch request construction; measure state duplication
  and packet work without asserting live speed or quality. Output: reproducible
  fixtures, complete call ledger, no-op zero-call proof, changed-input invalidation,
  and one human/agent-readable example that requires no generated explanation.

- [ ] **JEV-06 Authorized live studies.** When authorized, confirm current access,
  model/version and price, data/retention policy and spend ceiling before dispatch.
  Use the original campaign ledger only if the owner explicitly renews it; preserve
  all prior reservations. Run development, freeze revisions, then untouched holdout.
  Account for failed/canceled calls, retries, escalations, audit samples and unknown
  bills. Use complete-context versus retrieved-context and batching ablations.
  Review and routing may run or conclude independently. Output: retained trial manifests,
  original response/usage records and both protocols' complete denominators.

- [ ] **JEV-07 Review decision.** Report per-rule/severity recall and actionable
  precision, unknown/abstention, wrong-but-confident cases, source localization/support,
  duplicate burden, accepted repairs, review labor, API cost and p50/p95 total latency.
  Independently inspect sampled no-issue and non-escalated outputs; distinguish retrieval,
  judgment and policy failures. Compare against the structured-LLM arm as well as
  freeform review. Show uncertainty by independent change clusters and coverage at each
  threshold. Output: retain/revise/reject per criterion, exact approved configuration
  and limits. No pooled score may hide a critical miss.

- [ ] **JEV-08 Optional Review Kit feedback integration.** For evaluated criteria,
  expose the same scoped checklist and source-linked authored feedback to agents and
  people through existing artifacts. Bind any new displayed fields through versioned
  producer/consumer contracts. Use relevant edits or explicit batch completion,
  debounce/coalesce, and suppress stale responses for current feedback. Keep complete
  history and zero-execution read paths. Verify independent/DATUM rendering through
  app-kit guidance if UI changes are needed. Prove one real Fathom import/serve/edit/
  repair/recurrence tracer and its unsupported-host behavior. Output: advisory normal
  workflow, useful-action measurement, and explicit evidence needed for any later
  CI enforcement proposal.

- [ ] **JEV-09 Independent routing study.** Begin design alongside JEV-01/02; do not
  gate it on review accuracy. Select a named consumer task distribution and eligible
  configurations from the existing 0011–0013 workflow. Code filters authorization,
  capability, context and spend constraints. Evaluate task-feature classification
  feeding existing policy, with direct Choice as an alternative. Measure actual task
  success, quality-constrained total cost, avoidable escalation, routing overhead,
  route stability and regret against eligible observed outcomes. Include prompt-cache
  loss and context conversion, with tool-loop
  continuations pinned at a declared task/turn boundary. Bounded paired
  counterfactual execution requires JEV-06 authorization; shadow logging alone cannot
  establish every unchosen path's performance. Output: an independently supported
  retain/revise/reject decision, fallback policy and any proposal to reopen runtime
  selection deferrals. No automatic production-default change.

- [ ] **JEV-10 Contract and delivery disposition.** Keep catalog semantics and
  closure with Review Kit, selection policy with its consumer, and source intelligence
  with Fathom. If a second concrete consumer needs the same native question/receipt
  mechanics, review an independently consumable shared leaf against the existing
  profile/evaluator/catalog contracts. Reconcile the current LLMClient wording and
  platform leaf-module boundary in a decision before exporting a new shared API.
  Otherwise retain the local port. Output: relevant ADRs/intent, exact migration and
  consumer evidence, focused conformance tests and required repository gates. Findings
  or static tests alone do not demonstrate field adoption.

## Decision thresholds

Set numeric thresholds in JEV-02 from the intended use and deterministic/current-review
baseline. Required protocol fields are maximum miss rate by severity, acceptable
false-alarm burden, minimum useful coverage, escalation/audit rate, practical cost/time
improvement, sample size and uncertainty method. Record which are pilot screening
targets and which can support broader deployment.

For advisory use, false positives cost attention and missed defects limit coverage.
For skipping deeper reviews, misses become more consequential: audit non-escalated
cases and demonstrate the appropriate risk/coverage tradeoff. A typed or high-confidence
answer alone clears neither bar. Use stronger evidence whenever the existing rule
requires it. A positive result in routing does not validate code review, or vice versa.

## Immediate deliverable and completion discipline

The next proposed implementation unit is JEV-01/03/04/05 narrowed to a six-criterion
offline tracer, plus JEV-02/09 routing protocol and packet preparation. It has an
engineering exit without live funding: typed definitions, fake provider, exact sources,
authored feedback, correct unknown handling and replay. No claim of Jev quality follows
from that exit.

The broad JEV work boxes retain their full-track acceptance conditions. Review Kit
consumer delivery has its own implementation/test contracts and proof below; it does
not establish live quality or cross-consumer adoption. Before marking broader tracks,
use actual symbol/package verification contracts and run
`fathom verify --service agentic-kit --plan docs/plans/2026-09-jev-review-and-routing.md`.
Keep unprovable claims open; record documentation review separately. Required
`task validate`, affected consumer checks and host/runtime tests apply to their
implementation surfaces. Do not weaken a gate to make a plan pass.

Review on 2026-10-16 or next release, alongside decisions 0011–0014. Retain, revise or
park each track using its own evidence; elapsed time does not create a measured result.

### Consumer delivery checkpoint — 2026-09-22

Review Kit TJ-01–09 have runtime and static proof. The owner-approved portal dependency
repair is applied; normal consumer tests, vet, build and dependency seams passed. The
[delivery audit](../../../platform-engineering/kits/review-kit/docs/assessments/2026-09-22-typed-review-delivery.md)
and [adoption guide](../../../platform-engineering/kits/review-kit/docs/design/typed-judgment-adoption.md)
replace speculative implementation instructions for this consumer. Broader JEV rows
remain open where they require held-out outcomes, other live provider profiles,
invoice reconciliation, native hook certification or production routing. The later
OpenRouter smoke verifies only connectivity/API cost and actual artifact handling;
its semantic error strengthens JEV-02's missing-information requirements. The earlier
paid campaign has not resumed.
