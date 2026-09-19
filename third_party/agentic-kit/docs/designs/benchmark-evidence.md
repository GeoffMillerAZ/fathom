# Workload selection evidence

Status: accepted direction; bounded first implementation. Decision:
[0011](../intent/decisions/0011-workload-selection-evidence.md). Findings and primary
sources: [assessment](../assessments/2026-09-16-selection-evidence.md).

## 1. Product and implementation boundary

Help a consumer say: **this eligible solution meets these requirements on this workload,
under these measured conditions, with these limitations**. Do not promise a universally
best model or deterministic semantic truth. Deterministic checks establish specified
properties; experiments estimate performance; assumptions remain named assumptions.

The first slice extends the experimental `kits/eval-runner`:

- Exact binding of supplied outputs to scenarios and producer configuration.
- Versioned, component-addressed `BenchmarkManifest` and bounded `BenchmarkEvidence`.
- All-submitted-trial accounting, explicitly incomplete/estimated costs, and latency
  distributions with their denominators.
- Fresh, exactly paired live acceptance comparisons using existing task-clustered
  `reliability-stats.PairedBootstrapCI`. Replay and synthetic bookkeeping cannot enter
  this statistical path. No automatic winner, production switch or live collection.

This change produces no live quality result, selection recommendation or production
adoption evidence. The comparator consumes supplied receipts; it performs no live calls.
Sections below specify the larger integration direction. They do **not** imply that
deployment discovery, automatic selection, all metric collectors or RAG engines ship.
Consumers may adopt receipts without the runner, and the runner without a new service.

## 2. Ownership: extend, do not duplicate

| Concern | Existing owner | Integration direction |
|---|---|---|
| Published model facts | `llmclient/model-catalog` | Versioned sourced assertions; no authoritative embedded rows |
| Approved deployments and environment | Consumer CUE config + injected `LLMClient` | Explicit eligible deployment set; do not create a central organization service |
| Ordered capability escalation | `llmclient.Ladder` | Task/role-specific, prefiltered rungs; caller evaluates signals |
| Provider outages | `llmclient.NewFailover`, typed errors | Separate failure policy and eligibility recheck |
| Evaluation | `kits/eval-runner`, `scenario-bank`, `qa-harness` | Solution/trial evidence alongside existing judge-call records |
| Judgment and audit | `eval-journal`, `ai-provenance`, evidence spans | Link exact output, rubric and source versions; no duplicate journal engine |
| Identity and invalidation | `record-fingerprint`, prompt hashes, correction ledger | Component hashes and dependency references; consumer-owned artifacts |
| Cost and operational signals | cost tracker, budget guard, `llminstrument`, telemetry | Collect every leaf call; aggregate without double counting |
| Statistics | `reliability-stats` | Paired task-level uncertainty; caller owns experiment design |
| Retrieval | retrieval metrics, evidence ranker, vector store, truth bridge | Diagnostic comparisons and provenance-preserving adapters |
| Workflow/effects | agent harness, effect gate, HITL, operator elicitation | Preserve authority, bounds and distinct recovery actions |

No root Go dependencies, new package or stability demotion is needed for the first
slice. New fields in stable CUE contracts require the project's migration ceremony.
The first manifest is evaluation-specific, not the deferred universal harness manifest.

## 3. Four layers of model information

Target consumer integration, not new V1 catalog/deployment fields. V1 carries component
digests; it does not materialize eligibility or execute selection policies.

1. **Provider claim.** Source URL/API, retrieval time, source digest, provider/model
   identifiers, requested alias versus exposed resolved version; lifecycle state;
   modalities, input/output limits, tool and structured-output support, supported
   reasoning/sampling settings and combinations. Unknown differs from unsupported.
2. **Deployment eligibility.** Organization/environment-local deployment ID; endpoint
   and API mode; region; credential *reference*, never credential; approved uses/data
   classes, permissions, contract/retention/residency restrictions, operational access
   observation and expiry. Distinguish approval from reachability and actual model access.
3. **Workload measurement.** Full solution fingerprint, dataset/strata, protocol,
   environment, grader, costs and outcomes. Unbenchmarked is an explicit state.
4. **Recommendation.** Policy version plus eligible/rejected configurations, evidence
   references, applicability, uncertainty, expiration and override/outcome records.

Each layer has its own source and freshness. A new globally known model is neither
approved nor accessible nor benchmarked. Missing facts cannot acquire permissive
defaults. Deprecated, retired, unavailable and unknown are distinct reasons to exclude
or review. An availability probe can itself require permission and spend.

Provider APIs supply unequal metadata. Preserve raw claim provenance and a normalized
projection rather than inventing uniform capabilities. A provider label or a custom
endpoint's model string does not establish training lineage or immutable version.
Native CLI entitlement is distinct from API entitlement and model/tool availability.

Pricing is a versioned reference, not `CostTier`: distinguish region, service tier,
input/output/cache/reasoning billing, batch/tool charges, discounts and effective dates.
Use the existing price table where it represents the contract; otherwise preserve an
external billing amount and its source. Never infer a missing price as free inference.

## 4. Candidate is the complete solution

Represent independent axes, not one exclusive architecture enum:

- Computation: deterministic/no-LLM, single call, fixed chain, adaptive loop.
- Roles and topology: producer, workers, reviewers, synthesis; edges and handoffs.
- Scheduling: sequential or bounded parallel, queue/cache state and cancellation.
- Context: prompt templates, assembly, compaction, retrieval and token limits.
- Model configuration per role, API mode, defaults, reasoning and output bounds.
- Tool contracts/behavior, permissions, retries, failover, escalation and stop policy.

Parallel execution may occur within a single agent or a multi-agent workflow. Extra
reviewers may improve error detection yet add correlated errors, synthesis loss and cost.
Always include the cheapest adequate deterministic baseline, then a simple single-call
baseline where meaningful. Vary one factor for diagnosis; benchmark complete bundles
for selection. Record intentional confounds rather than attributing a bundle's gain to
one model. Keep compute and cost-matched comparisons alongside unconstrained results.

## 5. Experimental protocol

Before inspecting candidate results, fix workload population, task strata/weights,
data rights, splits, primary acceptance metric, hard constraints, practically worthwhile
improvement/non-inferiority margins, comparisons, trial budget and stopping rules.

Use development cases for prompt/routing tuning, screening cases for cheap pruning,
and untouched final cases for the decision. Group near-duplicates and related entities
into one split; preserve a split digest. Synthetic cases expose mechanisms, but do not
establish production prevalence or difficulty. Legacy corpus Q5 remains unratified.

Run matched task/replicate pairs; randomize or interleave candidate execution order.
Reset mutable state between trials. Record endpoint, timestamp window, input/output
sizes, cache hit/miss/unknown, concurrency, tool environment and timeouts. Correlated
outages and repeated trials of one case are not new independent tasks. An unstarted
scheduled case gets an explicit disposition; silently dropping it invalidates coverage.

Report effect sizes and uncertainty, not just p-values. Use the existing task-clustered
paired bootstrap for equal-task-weighted acceptance differences. Report macro task
estimates separately from trial-weighted rates; production-weighted strata need their
own declared estimator. A degenerate interval from few or uniformly passing tasks is
not certainty. Size important studies from pilot variance and the minimum useful effect;
there is no universal minimum sample count or magic three-trial certificate.

Testing many candidates against the same holdout creates selection bias. Screen first,
confirm the selected comparison on a fresh holdout, and record all configurations tried.
Use planned multiplicity correction when making a family of confirmatory claims;
optional stopping needs a suitable sequential procedure, not repeated ordinary CIs.
The kit does not currently implement either correction or sequential inference.

Grader validation is part of the protocol:

- Prefer outcome/state/property tests when they capture the requirement; retain valid
  alternative solutions instead of overfitting to a prescribed trajectory.
- Semantic rubrics specify dimensions, evidence, abstention and disagreement handling.
  Calibrate against independently labeled references, including counterexamples.
- Check position swaps, verbosity/rubric perturbations, family/lineage correlation,
  misleading reference content and instructions embedded in the item being graded.
- Keep the final grader and hidden acceptance data outside the candidate's write scope.
  Validate graders against known-good and seeded-bad results before trusting a score.
- Separate answer-level calibrated confidence from an aggregate estimate's confidence
  interval. Self-reported certainty and cross-family labels establish neither.

## 6. Measurement contract

One logical task may include many attempts and many model/tool calls. The receipt binds
task/replicate and complete solution to the final accepted outcome. A judge-call record
is not another task trial. Preserve judge records in `eval-journal` and reference their
artifact digest from the consumer's evidence; do not duplicate their text in every row.

| Metric | Meaning and denominator |
|---|---|
| First visible token | Client request to first user-visible content; exclude hidden reasoning; report missing/censored count |
| First useful result | First content satisfying the workload's versioned usefulness check; acknowledgment is not useful |
| Generation throughput | Declared token count and generation interval; report tokenizer/provider convention and missing usage |
| Terminal duration | Task admission to terminal outcome, including failed/timeout/canceled tasks; not a success-latency claim |
| Accepted completion | Task admission to verified acceptance, including review/wait; conditional distribution plus acceptance rate |
| First/eventual success | Accepted without retry versus accepted before stopping; all attempted logical tasks remain denominator |
| Cost per attempted task | All attributable costs divided by attempted logical tasks, with unknown coverage visible |
| Cost per accepted task | All attributable costs, including failures, divided by accepted tasks; undefined if none accepted |
| Recovery/intervention | Task-level retry/escalation/failover/abandonment/timeout/review incidence plus counts and operator time |
| Overhead | Retrieval/tool/queue/orchestration/screening/verification/synthesis durations and charges, with overlap explicit |

Record p50/p95 and sample counts, not just means. Conditional accepted latency must not
be sold as unconditional p95 time-to-success: when too few tasks succeed, that quantity
is not established. Failures are not zero-duration successes. First-result timestamps
missing because a task failed are missing/censored, not fast results.

Charge every leaf operation once: routers, retries hidden inside adapters, cached calls,
screeners, judges, stability probes, failed calls, canceled/speculative workers and
synthesis. Parent spans aggregate children but are not additional billed calls. Reconcile
late usage after cancellation; until then retain unknown cost/reservations. Separate
online verification from experiment-only grading cost, reporting both and their total.

Cost basis is **observed billing**, **usage-calculated**, **estimated**, or **unknown**.
Mixed totals retain amounts/counts by basis, warnings and coverage; receipt claims of complete
cost still require a real collector. An absent collector cannot establish zero. V1
summaries calculate over submitted receipts, not automatically discovered calls; the
consumer must check planned-case and leaf-call coverage before operational use.

Every V1 report labels evidence **caller-declared**. Even `mode: live` is an unverified
external attestation, not proof a call happened. A trusted consumer collector must bind
the actual configuration, planned/submitted case counts, leaf-call coverage, dataset
split and grader checks before using the arithmetic as empirical evidence. Neither a
hash nor this comparator authenticates those claims. The operational report additionally
needs missing/estimated counts, task/replicate counts, dependency freshness, N/A reasons
and coverage/grader limitations; omissions block recommendation, not arithmetic alone.
First-visible/useful timings are optional raw trial facts in V1, not standardized
summary metrics. Its latency quantiles use linear interpolation over all terminal
durations, and separately over accepted-completion durations. First-attempt success
means terminal acceptance with exactly one attempt, not a successful internal call.
Complete estimated cost may yield an explicitly estimated cost-per-task; estimates
never become mathematical lower bounds. Inspect basis totals, not the aggregate alone.
The accounting model is non-negative attributable charges, not a net invoice with
credits/discounts; partial observed/usage sums are only lower bounds under that model.

The paired estimand is terminal workload acceptance, not model-only answer accuracy.
Known errors, timeouts, cancellations and abandonment remain non-accepted tasks: dropping
them rewards unreliable workflows. An **unknown** terminal outcome blocks comparison;
it remains in bookkeeping denominators, so that summary is observed acceptance only.
The protocol must fix the terminal/deadline policy. No submitted labels establish that
all planned trials were collected, nor that individual task clusters are independent.

## 7. Identity, provenance and reuse

V1 manifests require explicit component digests for solution code, prompt, context,
model/deployment configuration, parameters, tools, workflow and retrieval; evaluation
digests cover dataset, acceptance, grader, protocol, environment, pricing and useful
result definition. A no-LLM/no-RAG component hashes an explicit not-applicable marker.
Component values are SHA-256 digests, not paths or credentials. Bound map sizes and IDs.

Canonicalization is versioned Go JSON serialization of typed fields and component
arrays sorted by name, with a domain separator and existing
`record-fingerprint.HashContent`; it is not JCS
or semantic normalization. Materialization must resolve defaults before hashing. Store
the referenced component artifacts under consumer retention controls so the hashes can
be reconstructed. Hashing an opaque label or omitting a dependency defeats attribution;
format validation cannot discover those omissions.

Each trial additionally binds an immutable trial-spec digest (input, task identity,
replicate seed/schedule and controlled conditions); paired labels with different specs
are refused. Accepted/rejected outcomes reference the retained acceptance-check artifact
by digest. Receipt fingerprints cover the manifest identity, declared mode/validity and
the complete trial ledger sorted by task/replicate, with UTC timestamps. Summary and
comparison reports carry these receipt identities; comparisons also carry seed,
resampling count, interval level and evaluation time. Callers retain and resolve receipts
by digest to reconstruct arithmetic. This library provides no storage, signature or
authentication. Losing the receipts or referenced artifacts loses auditability.
The trial spec excludes intentionally varied solution components: a model or prompt
under comparison belongs in the solution manifest, not a falsely shared trial identity.

The model component includes requested alias, exposed resolved identity (or unknown),
endpoint/API mode/region and role configuration. External tools and indexes need
observed revisions and validity intervals. A local hash does not detect a silent remote
change. Reusing a historical measurement does not make another independent observation.

| Change | Permissible reuse, after authority/freshness checks |
|---|---|
| Exact inputs, deterministic code and acceptance artifacts | Reuse exact deterministic result; preserve original trial ID |
| Identical output/input + identical deterministic grader | Reuse grade; do not increment independent trial count |
| Grader/rubric/acceptance change | Regrade retained outputs where sufficient; no regeneration needed solely for grading |
| Price-only change | Recalculate from complete usage using new price version; retain original billing; quality may remain useful |
| Prompt/model/tools/workflow/context/retrieval change | Rerun affected solution tests; retain unaffected artifact identities |
| Corpus/index/ACL/freshness change | Recheck affected retrieval, support and authorization; conservative full rerun if dependencies unknown |
| Unobservable alias/remote behavior drift or expiry | Fresh live assessment; unchanged local digest is insufficient |
| Semantic/approximate cache match | Separate approximate experiment with error/privacy metrics; never exact reuse |

V1 strict comparison intentionally refuses changed evaluation fingerprints, including
prices. It does not yet implement dependency-aware partial reruns or a grade cache.
Consumers may record an explicit harmonized re-analysis later; silently mixing old and
new evaluation conditions is forbidden. Replayed traces test implementation, not fresh
quality, latency or billing. Corrections/revocations invalidate downstream recommendations;
existing correction-ledger and reviewed lesson distribution remain the authority path.

Raw traces are not mandatory. Keep output/source locators and digests where content is
sensitive; scope access, encryption, redaction and retention. Hashes of guessable private
values can themselves disclose information. A deleted artifact makes later reproduction
unavailable, not successful. Existing append-only stores do not gain deletion semantics.

## 8. Deterministic decision policy (next consumer integration)

Designed here, not executed by the V1 comparator. A returned interval is not a selection.

Policies filter before optimizing: authorization/data constraints → availability and
required capabilities → hard acceptance/safety requirements → fresh applicable evidence
→ cost/latency/operational-complexity objectives. Unknown requirements do not pass.
An explicit research-only unbenchmarked candidate can enter screening, not production.

The consumer compiles admitted role-specific candidates into existing `Ladder` rungs.
Initial choice is not necessarily the cheapest model. For a two-stage policy, expected
cost is first-stage cost plus the probability of escalation times conditional later
cost, including validation and false-acceptance risk. Measure the complete routing
policy against strong-first; do not estimate it from isolated model averages alone.

Keep actions distinct and bounded:

| Observed condition | Policy action |
|---|---|
| Transient provider failure, safe retry identity | Retry/backoff within attempt and deadline bounds |
| Deployment outage and equivalent authorized alternate | Provider failover; recheck capability/data constraints |
| Missing evidence, retrieval budget available | Retrieve more/change query; do not claim a stronger model supplies facts |
| Supported evidence but failed reasoning/format criterion | Capability/configuration escalation; preserve prior evidence |
| Missing authorization or contested/irreversible judgment | Human review/authorization, not model escalation |
| Unknown external side-effect outcome | Reconcile destination state before retrying |
| Exhausted budget/deadline, revoked access, no admissible candidate | Stop/abstain with explicit cause |

A selection record contains version, decision ID/time, workload+requirements digest,
policy digest, org/environment/deployment snapshot, all considered candidates, ordered
rule results and rejection reasons, evidence fingerprints/validity/uncertainty, chosen
action and limits. Later outcomes reference the decision ID; overrides name their
authority and do not rewrite the original record. Explanatory prose is a rendering of
this record, never the authorization input. Store through existing JSONL/event/journal
facilities and versioned artifacts, not a new decision database.

## 9. RAG selection and diagnostic experiments

Extend the existing practices/roadmap and consumer retrieval configuration, not a
second strategy catalog. Lexical, dense, hybrid, metadata/ACL-filtered and reranked
retrieval are independently composable choices. Query rewrite/multi-query, hierarchy,
graph and iterative retrieval require evidence that their extra work pays off.

The solution includes parsing/chunking/overlap, metadata, embedding model, index version,
freshness/permissions, ranking/reranking, context assembly and citation handling. Test
retrieval against query relevance labels separately from generation. Reuse precision,
recall, MRR, coverage and false-absence metrics; add nDCG only when graded relevance is
available, in the existing metrics package with its declared convention.

Compare: no retrieval; normal retrieval; oracle relevant evidence; normal evidence with
oracle context assembly; generation from fixed evidence. These ablations localize recall,
selection/truncation and generation failures. Include contradictions, stale documents,
answerable-but-missed questions, genuine scoped absence and appropriate abstention.
Track citation correctness/support/coverage separately from ID validity. Unlabeled
documents are not automatically irrelevant, and top-K failure is not proof of absence.

Measure indexing/refresh/embedding cost amortized over a declared workload horizon,
retrieval and reranking latency, maintenance and permission leakage. Record the corpus
and query conditions supporting a recommendation; no global claim that dense or graph
retrieval is best. The first slice adds no index, graph engine or general context ranker.

## 10. Screening and trust boundaries

A small policy classifier may screen explicit policy categories, with uncertain cases
sent to a stronger classifier or human. Measure class-conditional misses, false blocks,
prevalence, adaptive attacks, cost and latency on held-out cases. A trained specialized
classifier's research result is not evidence that a prompted small general model works.

Authorization, tool permissions and protected output release remain outside the
classifier. Untrusted wrappers are defense-in-depth. Test malicious content through
retrieval, tool results, compaction and cross-agent handoffs; retain trust/provenance
through every transform. In controlled, authorized tests, use a fake destination and
synthetic secret canary: attempted unauthorized mutations must leave its state/effect
count unchanged; protected output, journals and egress must not contain the canary.
Test outcomes, not merely detector alerts, and include benign utility controls. This
does not authorize probing real credentials, private data or production destinations.

Speculative work may run only where independently authorized, read-only and bounded.
Protected output is buffered until required checks pass; streaming acknowledgment does
not count as a useful result. No mutation or disclosure can race ahead of the approval
or output check. Existing effect/HITL paths govern commits and ambiguous recovery.

## 11. Prospective consumer integration and offline proof

Chosen consumer: resume_as_code `7a6c56a26dd10a7cf80128d23ef13bab90377ae8`, inspected on disk.
`cmd/generate.go:buildRedTeamPanel`, `cmd/ops.RedTeam` and `cmd/redteam` already have
CUE-configured model roles, a two-round bound, structured critics, per-lane usage and
request-scoped provider-family checks. Preserve these, including UNVERIFIED results.
Do not change its production model or factual résumé data to build the pilot.

Next consumer adapter is local to that workflow: materialize eligible deployments from
consumer CUE; validate known models/lineage with existing catalog; wrap leaf calls and
verification to emit these benchmark receipts. Do not label three slots from one
provider an independent panel merely because they have different names. An unavailable
required reviewer blocks the claim, not the entire consumer's unrelated workflows.

Start with synthetic fixtures and the existing offline red-team tests to prove plumbing.
Then, after data and spend approval, compare deterministic fact/coverage checks alone,
the current bounded red-team bundle, and one explicitly eligible alternative. Reserve
separate development/held-out tasks stratified by unsupported claims, contradictions,
absence, benign supported prose and malicious source/judge instructions. Small screening
is triage; final sample size follows pilot variance and the decision's consequences.

Implementation acceptance:

1. Duplicate/foreign outputs and prompt/parameter mismatches refuse before grading.
2. Manifest map order is irrelevant; every material component change changes identity.
3. Unknown/partial cost and zero acceptance cannot render a false cheap success.
4. Failed, timed-out and canceled work stays in submitted-trial denominators and cost.
5. Stale/future-dated, replay/synthetic, unknown-outcome, mismatched protocol/spec and unpaired data refuse
   statistical comparison; repeated trials retain task-cluster grouping.
6. Seeded comparison is reproducible; CUE/Go serialization and negative cases agree.
7. Existing replay behavior and downstream consumers remain compatible, with actual
   revisions/skip/failure results recorded. No new model calls or dependencies.

Consumer promotion additionally needs planned-case and leaf-call coverage, retained
artifact provenance, grader calibration and a measured collection budget. It is not
established by package tests or a synthetic receipt. Record the consumer commit and
gate disposition for integration; actual workflow use for adoption; compared outcomes
and costs for measured benefit. These remain separate assurance axes. Review owner:
kit maintainer; review due 2026-10-16 or next release. If only offline plumbing remains,
revise with new evidence and a bounded review or park; do not call that promotion.

The reproducible [overlay bridge](../../examples/benchmark-evidence/README.md) exercises
that consumer's actual red-team package without changing source, vendor or model choices.

## 12. What to build after evidence, not before it

The owner has now commissioned the bounded operational follow-through under
[0013](../intent/decisions/0013-operational-selection-evidence.md). Its
[completion ledger](../plans/2026-09-operational-evidence.md) retains the required
consumer, collection, policy, screening and measured-decision exits; the following
sequence is active work, not an assertion of completion or automatic promotion.

Priority: consumer receipt collector → eligibility/materialized catalog adapter →
fixed-policy selection record → targeted live comparisons → current-model/deployment
change assessment. Model discovery triggers a budgeted eligibility/screen/compare/confirm
proposal identifying affected workloads and expected decision value, never preference.
Changed price may justify recalculation only; unknown aliases may justify a fresh canary.

Defer adaptive optimizers/bandits, automatic model switching, a judge pool, distributed
workers, provider batch APIs, graph retrieval, central evidence storage and semantic
caches until a consumer measures the need. Every promotion retains rollback, baseline
and a reevaluation trigger. The goal is fewer unjustified choices, not a larger platform.

## 13. Reuse planner boundary (0013)

`PlanBenchmarkReuse` composes the existing experimental manifests and evidence.
The request binds each source unit to its actual trial-spec digest, classifies every
manifest dependency as locally retained or externally observed, and supplies versioned
verification, determinism and usage receipt references. External observations name the
exact source/target component values and their validity window. The entire request,
source, target and proposed plan have canonical identities.

Actions distinguish retained deterministic output, retained grades, regrading, usage
repricing, unit reruns and fresh live measurement. Combined grader/price changes retain
both actions. A live target cannot substitute retained output/grade for a new observation;
all proposed actions add zero independent trials and execute nothing.

The planner validates caller assertions; it neither reads retained bytes nor proves a
receipt truthful. The consumer verifies bytes and controls retention. Missing dependency
bindings, stale/unknown remote observations and missing artifacts conservatively prevent
the corresponding reuse. Exact output reuse requires all-local bindings and both source
and target determinism receipts; a remote model name is not a determinism certificate.

Current dependency coverage is the complete manifest per unit. This deliberately does
not claim inferred minimal test subsets: dependency-selective execution still needs a
consumer-derived, version-bound dependency closure. The operational goal remains open
until its actual consumer reuse/invalidation path and all other required exits are proven.
