# Decision models: project opportunities and limits

Read-only source survey, 2026-10-23. These are adoption candidates, not measurements
of accuracy, savings or permission to migrate a consumer. Utility: [decision-lab](../designs/decision-lab.md).

## Current source snapshots

| Project | Inspected revision | Relevant source state |
|---|---|---|
| Platform engineering | `821c509035a8` | Review judgment sources clean |
| Fathom (`sextant` checkout) | `fd42197ffcb6` | Decision-backed rule-ordering sources clean |
| consumer-c | `04be1238bae1` | Relevant code clean; local `docs/intent/scope.md` edits preserved |
| app-kit | `6883a60d7863` | Quality-inventory source checked locally after stale index response |
| Consumer-d | `2d95c9795d11` | Future-gated triage, no adoption implied |
| agentic-kit | `da35dea` | Existing port, quality gate, practice-review reference |

Fathom supplied source locations; its raw/incomplete obligations and partial reviews
do not certify any runtime behavior. No other project's files, pins, hooks or policies
were changed. Exact code references below name the inspected commit; later work may differ.

## Prioritized sites

| ID / current state | Bounded question and code seam | Floor and baseline | Measure / stop |
|---|---|---|---|
| DS-01 Review criteria, **implemented optional lane** | `platform-repo/kits/review-consumer/internal/judgment/judgment.go` `Prepare`/`Review`: criterion-authored semantic options with required unknown; support/conflict is one possible pack | Existing off/replay/Jev/OpenRouter, domain types, source tiers, exact identity and coverage; retain mandatory Standard checks | Held-out false satisfactions, unknowns, reviewer minutes and total attempts; stop provider use if quality floor or paired benefit misses |
| DS-02 Governing-rule order, **implemented optional lane; protection partly unproven** | `sextant/internal/semantic/decisionrank.go` `rerankWithinTiers`: is this rule relevant to the proposed edit? | Preserve tiers, lexical fallback and per-site opt-in. I62's calibration rule is accepted but its serving-bar test remains pending; relevance grants no security authority | Prove pending `TestJudgmentNeverPrunesBelowR3Bar` before expansion; compare applicable-rule recall, omissions, latency/token cost and task gain |
| DS-03 Retrieval labels, **best new offline pilot** | `consumer-c/cmd/ops/evaluation_retrieval.go` `EvaluationRelevanceLabel`/`ComputeEvaluationRetrievalDiagnostic`: separate relevance, support and contradiction labels, preserving nullable unknown | Existing labels and observed retrieval controls; preserve resolver/access/freshness/closed scope and mixed/conflicting evidence; live evaluation deferred | Human agreement and diagnostic localization versus current labels; stop if annotation/disagreement cost erases benefit |
| DS-04 Requirement matching, **existing model proposals** | `consumer-c/cmd/ops/assess.go` `ComputeAssessWith`/`MatchesFromProposals`: meets, partial, or unknown? | Evidence IDs must resolve; absent support cannot become meets; use present proposal path as baseline | Uncited affirmatives, human agreement and cost per accepted proposal; typed adapter migration earns its cost first |
| DS-05 Clarification routing, **proposed additional stage** | `consumer-c/cmd/llm/client.go` `ClassifyDemands` already classifies likely_have/partial/missing and generates an interview question; declared question-family routing would be new | Human answers/source records create facts; classification does not write the answer or infer missing experience | Accepted sourceable answers per operator minute; keep the existing combined prompt if an extra stage adds no gain |
| DS-06 Work queue triage, **proposed advisory use** | `app-kit/internal/qualityplan/reader.go` `Snapshot.List`: which authored work item merits inspection next? | Exact catalog/revision and untracked/filtered-out states remain; no model certification of gates/adoption | Time to useful selection and missed authored core items versus existing order; stop reordering without gain |
| DS-07 Prose review, **proposed shadow use** | `kits/governed-prose-kit/sourced_pipeline.go` `CheckDraft` already invokes a consumer-owned SupportReviewer and `sourced_export.go` `Export` rechecks support; new sites target audience mismatch or conceptual drift | Existing support, freshness and publication checks remain; new judgments may add concerns/dependents, never shrink required rebuild or authorize export | Human-adjudicated misses, false warnings and review burden versus existing review; stop when burden wins |
| DS-08 Lesson intake, **pattern awaiting receiver seam/W4** | Lesson-transfer has no shared candidate store; duplicate/reusable/insufficient labeling needs a named receiver-owned corpus | Receiver owns intake history, sanitation, reproduction, adoption and revocation; issues remain untrusted data | Independent recurrence history and accepted regressions versus direct intake; do not build before receiver seam/admission |
| DS-09 Failure triage, **pattern awaiting named workflow** | Implementation/test/environment/insufficient is a useful question shape, not an inspected project site | Parse exit/status and stack locations deterministically; a later admitted classifier suggests investigation only | Name a workflow and baseline first; then compare correct first investigation, time to verified fix and cost; no edit/command authority |
| DS-10 Consumer-d diagnosis, **deferred** | `consumer-d/internal/doctor/doctor.go` `doctor.Run`: likely explanation or next read-only probe | Existing deterministic severity/remedies; F7 waits for G3/UI-C, trigger not proven | Do not implement before admission; later compare diagnosis/recovery and reject unsafe advice |

Start with DS-01's existing lane and DS-03's public/synthetic offline labels. DS-02
should first prove its pending serving-bar protection. DS-04 is a migration; DS-05
must beat the existing combined prompt. DS-06/07 need bounded shadow studies;
DS-08/09 need named receiver seams and DS-08/10 retain wave/future prerequisites.
The lab creates no adoption.

## Rejected applications

- Replace exact BM25/RRF, schema checks, test status, counts/dates or permissions with
  model opinions. Keep their deterministic operators; model relevance can be a later
  measured signal over the eligible residue, never evidence that required work ran.
- Add an “AI-written” score to résumé style. `consumer-c/cmd/audit/style.go`
  explicitly rejects that unvalidated score; retain actionable structural checks.
- Let a semantic “safe” answer waive security, privacy or source-policy floors; let
  a model certify complete coverage or remove known prose dependencies. Rejected by 0026.
- Share thresholds or aggregate class accuracy across sites/providers. Preserve an
  exact cohort and per-predicted-class readings; cached/replayed rows are not new trials.

## Current official API review

Checked 2026-10-23: OpenAI documents general availability, typed predicate/choice/score
answers, up to 400 questions, shared-input prompt caching, question images and native
JSON input. It recommends thresholds derived from application labels. Shared prefix
reuse and exact answer reuse are different mechanisms. See the [Decisions guide](https://developers.openai.com/api/docs/guides/decisions).

The [API reference](https://developers.openai.com/api/reference/resources/decisions/methods/create)
documents `prompt_cache_key`/options and cached/write token counters. Cache token
details are included in input totals. Current kit `Usage` retains only input/output
totals, and the portable text/image request does not expose every native input/control.
The lab adds no adapter claim: native feature work needs explicit conformance and a
named adopter. No provider call, account availability, current pricing guarantee,
retention eligibility or live quality measurement was established by this research.
