# Assessment: evidence-based workload selection

Reviewed 2026-09-16. Baseline kit `2e1b1bbfd6277bc4f8835928e200b7ef516ff1be`;
resume_as_code `7a6c56a26dd10a7cf80128d23ef13bab90377ae8`, verified on disk.
Independent audits covered catalog/consumer selection, evaluation/accounting and
retrieval/security; exact source reads checked the important index findings.

## Finding

The right next product is **selection evidence for complete workload solutions**, not
a larger list of models or another agent framework. The kit has valuable primitives,
but does not yet connect their observations into defensible runtime recommendations.
[Decision 0011](../intent/decisions/0011-workload-selection-evidence.md) accepts this
direction. [Design and acceptance](../designs/benchmark-evidence.md) distinguish the
bounded first implementation from the larger researched integration proposal.

## Repository evidence

| Existing capability | What it establishes | Missing integration |
|---|---|---|
| `model-catalog.Catalog`, `ValidateIndependence` | Caller data, family/lineage and wire-capability checks; no authoritative rows | Sourced/fresh provider facts, tri-state unknown capability, organization approval and deployment access |
| `llmclient.Ladder`, `NewFailover`, `ProviderInfo` | Caller-ordered escalation; provider failure handling; descriptive request-specific identity | Task-specific evidence and eligibility before selection; identity is not reachability or independent training |
| `evalrunner.New/Run` | Supplied-output judging, independence checks, budget wrapper, journal and probe wiring | Complete solution/trial identity, observed producer lifecycle and all-call accounting |
| `scenario-bank`, `eval-journal` | Provenance-gated cases and append-only judge-call records | Case version/output binding in records; corpus Q5 remains owner-unratified |
| `Regression`, `determinism-gate`, replay | Baseline verdict/score changes, observed agreement, offline path checks | Not fresh quality evidence; legacy pairing is scenario/concern, not complete experiment identity |
| `reliability-stats` | Seeded task-clustered paired CI, pass metrics, effect-size/test arithmetic | Only pass^3 was wired into eval-runner; no experiment-design or multiplicity guarantees |
| Cost tracker, budget guard, instrumentation | Caller-priced usage totals, bounded calls and optional telemetry | Nil prices can yield zero in legacy accounting; no bill/estimate/unknown receipt or complete workflow denominator |
| `record-fingerprint`, prompt hashes, AI provenance | Content identity, reproduction/source references | No shared full-solution manifest or automatic dependency-aware validity |
| Retrieval metrics/ranker/vector store/truth bridge | P/R/MRR, false absence/coverage, ranking and embedding/retrieval seams | No evidence-backed retrieval-strategy selector or end-to-end retrieval security/outcome baseline |
| Harness/guardrails/effects/HITL | Tested controlled-path refusals, bounds, approval transactions | Optional controls do not imply universal deployment coverage; model screening efficacy is unmeasured |

Source anchors: `kits/eval-runner/{runner,types,judge}.go`;
`foundations/llmclient/{ladder,provider,failover}.go` and `model-catalog/`;
`foundations/quality/{eval-journal,reliability-stats,scenario-bank}/`;
`docs/practices.md` §§8,15,18,20,21 and `docs/threat-model.md`.
No central `OrgConfig` implementation was found: organization-specific choices live
in consumer CUE configuration and injected collaborators. Do not invent a parallel one.

Concrete input-binding defect: `Run` built a map that silently overwrote duplicate
outputs, accepted outputs outside the bank and compared producer model but not prompt/
parameter hashes. The implementation repairs this with pre-side-effect refusal tests
and records INC-14. The stale deferred-eval-runner row in TRACKING is also corrected;
the runner already ships.

## Research findings and implications

These are supporting sources, not evidence that agentic-kit has measured the same gains.
All links checked 2026-09-16; paper versions are pinned where possible.

- **Measure multiple objectives under comparable conditions.** HELM evaluates broad
  scenarios and exposes quality/robustness/efficiency tradeoffs. Inference for this kit:
  compare whole configurations on the same workload, not unrelated public scores.
  [HELM, v2](https://arxiv.org/abs/2211.09110v2).
- **Architecture benefit depends on the task.** A controlled study across 260
  configurations reports both gains and degradation from coordination. It is evidence
  for testing task/topology fit, not a universal multi-agent scaling law.
  [Scaling agent systems, v3](https://arxiv.org/abs/2512.08296v3).
- **Grade outcomes, not claims of completion.** Anthropic distinguishes tasks, trials,
  transcripts and final environment state; it recommends deterministic checks and
  calibrated semantic graders. This supports preserving the kit's existing seams while
  adding trial accounting. [Agent evaluation guidance](https://www.anthropic.com/engineering/demystifying-evals-for-ai-agents).
- **Selecting among noisy estimates creates bias.** Cawley and Talbot show overfitting
  the model-selection criterion can materially distort evaluation. Reserve final data,
  log configurations tried and predeclare comparisons/stopping rules.
  [Model-selection bias](https://www.jmlr.org/papers/v11/cawley10a.html).
- **A judge needs evaluation too.** MT-Bench documents position, verbosity and
  self-enhancement biases. Cross-family checks are useful but not proof of independent
  or correct judgment. [LLM-as-judge study, v4](https://arxiv.org/abs/2306.05685v4).
- **Retrieval has strong simple baselines.** BEIR's heterogeneous evaluation finds
  lexical retrieval a robust baseline and documents effectiveness/compute tradeoffs.
  Its historical ranking is not a current universal recommendation.
  [BEIR, v4](https://arxiv.org/abs/2104.08663v4).
- **Separate retrieval, faithful context use and answer quality.** RAGAs proposes
  distinct reference-free metrics. Adopt the diagnostic separation, not the assumption
  that model-generated metrics replace labeled acceptance or source truth.
  [RAGAs](https://aclanthology.org/2024.eacl-demo.16/).
- **Test adversarial effects and benign utility.** AgentDojo evaluates tool-using agents
  exposed to untrusted external data and adaptive attacks. A detector alert alone is
  not a prevented action. [AgentDojo, v3](https://arxiv.org/abs/2406.13352v3).
- **Cheap screening is a hypothesis worth measuring.** Anthropic reports a lightweight
  first-stage classifier cascading to a stronger one. This is specialized classifier
  research, not proof that any cheap prompted model is a safe authorization gate.
  [Classifier cascade](https://www.anthropic.com/research/next-generation-constitutional-classifiers).
- **Use compatible operational vocabulary without importing an SDK.** OpenTelemetry
  distinguishes operation/tool duration and token usage, including failed inference
  calls counted once per invocation. Map existing telemetry to these concepts; pin the
  mapping version and add workload-specific accepted/useful completion metrics.
  [GenAI metric conventions](https://github.com/open-telemetry/semantic-conventions-genai/blob/main/docs/gen-ai/gen-ai-metrics.md).
- **Model discovery is not an organization policy.** OpenAI's model API and Claude's
  model/capability API expose different information. Normalize sourced claims but keep
  approval, actual endpoint access and measured fitness distinct.
  [OpenAI model API](https://developers.openai.com/api/reference/ruby/resources/models),
  [Claude model API](https://platform.claude.com/docs/en/api/models).
- **Continuous evaluation is an engineering loop.** OpenAI recommends workload goals,
  datasets, metrics, comparisons and continuous evaluation. Reuse the kit's portable
  evaluator rather than tie the core to a hosted evaluation product.
  [Evaluation guidance](https://developers.openai.com/api/docs/guides/evaluation-best-practices).

## Recommendations and bounded implementation

Keep the stable catalog and journal compatible. Put the first manifest, trial summaries
and strict comparison path in the existing experimental evaluator. Reuse the existing
hash/statistics primitives and consumer persistence. No package expansion is necessary.
The implementation reports caller-declared evidence; it cannot prove model calls,
hidden-call coverage, representative data or grader correctness from submitted JSON.

The prospective real consumer is resume_as_code's bounded red-team workflow:
`cmd/generate.go:buildRedTeamPanel`, `cmd/ops/redteam.go` and `cmd/redteam/provider_test.go`.
It already has configured role models, usage accounting, two-round stopping and
request-scoped family checks. Next integration should instrument that real path and
materialize its eligible deployments—not replace its judgment loop or factual inputs.
Offline plumbing and compatibility are separate from production adoption and benefit.

## Assumptions, open decisions and deferrals

Implemented and verified: exact runner input binding; versioned manifests and receipt
fingerprints; bounded terminal trials with acceptance-artifact references; failure-inclusive
summaries with explicit cost bases; reproducible task-clustered comparisons that refuse
incompatible, stale, unknown or non-live declarations. No second evaluator was added.
The synthetic real-consumer bridge and retained receipt pass, as do the kit gate and
Throughline compatibility checks. Fathom/GGB retain their known unrelated test failures.
[Commands, revisions, negative control and intent health](../evidence/benchmarks/2026-09-16/README.md).

Assumptions: a trusted collector can observe all material calls; sufficiently independent
tasks and useful acceptance labels can be obtained; retained artifacts may be inspected
under consumer policy. Each must be checked in the pilot, not promoted to a guarantee.

Open: authorized held-out corpus/data rights, affordable live campaign, usefulness
definition, cost/latency/false-block tradeoffs and who adjudicates semantic disagreements.
Q5 and Q7 remain unresolved. No live model winner is established by this assessment.

Defer automatic routing/discovery, graph retrieval, adaptive experimentation, worker
pools, central stores and judge panels. Their existing subscriber triggers remain.
Review the evidence incubation by 2026-10-16 or next release; offline plumbing alone
does not establish promotion.

## Fathom during this exercise

Useful for exact definitions, file outlines, typed callers and enforcement entrypoints;
these avoided broad source loading. Less useful for broad conceptual searches: model/
retrieval concerns returned irrelevant or empty results despite existing code, and
generic names such as `Ladder` were ambiguous. `query_symbols` reported old/stale sources
immediately after a successful ingest while HEAD-aligned queries reported current code.
Direct bounded reads resolved behavior and consumer-revision uncertainty.

Judgment: useful navigation aid, not a substitute for data-flow review, runtime tests or
external research. The highest-value improvements are consistent freshness semantics,
qualified symbol resolution and reliable exact-versus-fallback search behavior. No
measured token saving is claimed. Firecrawl failed authentication; primary sources were
retrieved through web search without changing credentials or global tool configuration.
