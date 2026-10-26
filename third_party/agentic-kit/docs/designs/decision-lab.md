# Bounded decision-site utilities

Status: accepted implementation specification, 2026-10-23. Decision [0033](../intent/decisions/0033-decision-sites-extend-the-offline-reference.md).
Extends W1's offline reference under 0032, not a new implementation slot or runtime.

## Judgment, weighed

Use typed judgments for bounded semantic labels, source-support candidates, triage,
clarification and prose review. Keep required scope, source identity, authorization,
budget, freshness and effect gates deterministic. Defaults remain provider-none;
calibration belongs to the exact site/cohort/provider/resolved model. Existing
`decision.Decider`, `PrepareDecision`, cache/replay and readings stay the building blocks.
No new foundation, router, provider dispatcher, automatic promotion or production migration.

## Utility contract

DL-1. `cmd/decision-lab` is an offline-only CLI with `prepare`, `check`, and `study`.
It performs no network, key lookup, subprocess, cache mutation or effect. Reuse core
`decision.Validate`, `TemplateDigest`, `RequestKey`, `Confidence` and `NewReading`.
Initial scope is **text-only string-choice packs**: 1..32 questions, 2..16 distinct
options each, explicit `unknown`, observable instructions and independently declared
favorable options used only for diagnostic error counts. Other kinds/images are
explicitly unsupported here; the existing port still supports predicate/score/images.

DL-2. A separately trusted versioned pack names its ID/template version, questions,
expected provider and resolved model, policy epoch, owner, deterministic fallback and
baseline/kill criterion. It defines the expected question set independently of answers.
Any cohort, option, instruction, ordering, policy or identity change alters the binding.
An input names case/subject/revision and captured state. `prepare` validates both and
emits a versioned prepared envelope with full request, pack/input digests and RequestKey.
Explicit `--out PATH` creates a private file exclusively; `--out -` deliberately emits
the supplied state for a pipe. Otherwise print metadata only, never source text.
Digests establish byte identity, not producer authenticity or source truth.

DL-3. `check` consumes the trusted pack, input and a captured result. The capture pins
case ID, pack digest and RequestKey. A response is the **decision port JSON**, not raw
vendor wire. Validate exact provider/resolved model, nonnegative usage/latency, one
exclusive outcome per expected question, no foreign IDs, exact answer ID/kind, known
choice and complete finite [0,1] option probabilities with sum within 0.01 of one.
Choice must be a maximum-probability option (ties allowed). Recompute confidence with
the core recipe and reject contradictory supplied confidence. Refusal, invalid answer,
omission and semantic `unknown` are distinct; no missing answer becomes false/clear.
Transport/no-provider/consent/budget failures have typed captured causes, not raw errors.
Validate protocol before a captured answer contributes to any reading or candidate cache.
Cache provenance is per question: a batch can mix exact reused answers with new results.
Validate explicit hit IDs and declared dispatch-attempt counts; policy/no-provider/budget
refusals have zero dispatches. Counts are caller assertions, not authenticated billing.

DL-4. `study` consumes a trusted pack, **independently authored expected cases/labels**
and captures. Cases include the actual input so request identity can be recomputed.
Require 1..64 unique case and declared independent-unit IDs, exact labels for every
question, label provenance (human/computed/frontier), and dataset kind synthetic/observed.
Recompute each model RequestKey and reject duplicate exposures even under different
case/unit IDs. Contradictory labels for identical exposure need adjudication or missing
context, not extra confidence. This excludes exact repeats, not all correlated samples.
Captures cannot shrink expected coverage. Duplicate/foreign cases, changed bindings,
missing cases and invalid labels make a scoped report incomplete/nonzero. Keep all
denominators, missing/refused/invalid/unknown/typed-cause/cache counts, declared attempts,
unreported usage, per-class confusion,
false favorable/unfavorable counts and per-predicted-class `NewReading` results visible.
Abstention is separate from an established unfavorable prediction. Exclude only each
reused answer, not its fresh batch peers, from calibration N. Re-running a report
does not append trials. Declared unit IDs do not authenticate empirical independence.
The reading key binds the entire pack/cohort plus recipe, provider and resolved model.
No single overall excellence score, automatic threshold, calibrated permission or savings claim.

DL-5. Bound each JSON input at 1 MiB and nesting at 24; reject duplicate/unknown fields,
case aliases, malformed UTF-8, trailing data and inappropriate nulls. Require canonical
field names and bounded ASCII IDs. Iterate IDs canonically and retain exact input file
hashes separately from semantic/request hashes. Stable JSON and plain text have fixed
reason codes, no provider body/source/credential echo, and actionable next steps.
No implicit file reads from source references or images. Output errors are nonzero.
`check`: 0 means every expected classification is usable; a partial batch retains its
per-question answers but returns 2 for invalid/unknown/unusable members.
Usable answer details expose only validated option/confidence/reuse, never raw reasons.
`study`: 0 means a complete valid diagnostic report, even with bad predictions;
2 means malformed or incomplete data. All outputs say **no action authority**.

DL-6. Ship source-backed recipes for review triage, retrieval support and human/prose
clarification; fixtures include a deliberately false favorable result and missing-context
abstention. Positive and negative CLI tests cover omission/duplicates/foreign IDs,
model/cohort/input drift, invalid probability/choice/confidence, mixed refusal/invalid
outcomes, replay/cache inflation, independent expected-case loss, output failure and
private/exclusive output and mixed fresh/reused batches. Include a complete retrieval
cohort, not only a retrieval pack over unrelated triage inputs. AI-authored semantic
labels identify frontier provenance; computed provenance needs an actual stated oracle.
CUE owns pack/input/capture/study shapes; Go mirrors semantics.
An independent guide-only trial reaches a useful offline result within 15 minutes.

## Adoption and review

This is a W1 authoring/qualification aid, not full AE-1/AE-2/AE-7 completion. Existing
quality-gate retains repository-bound enforcement; consumers keep their domain laws.
Maintenance owner: kit maintainer. Existing W1 budget/review remains five engineering
days, zero provider spend, <=64 MiB corpus, review 2026-11-21 or next release. No new slot.
Require focused race/CUE/CLI checks, independent review and `task validate`. No exported
library API changes are intended. Wider native API features and live studies remain
explicit adoption decisions, not silently dropped capabilities or automatic paid work.
