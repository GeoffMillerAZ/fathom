# Operational evaluation and selection

Status: operational implementation delivered 2026-09-21; further live study deferred. Owner: kit maintainer; agents execute the owner's
bounded completion objective. Baseline: `3b5b3c8` (0011/0012 offline slices).
This is the single requirement/evidence ledger. Under the owner's explicit 2026-09-21
direction and [0014](../intent/decisions/0014-offline-delivery-live-study-followup.md),
only further live study is deferred; all implementation and offline exits remain required.

## Contract and authority

Turn the existing benchmark and assessment APIs into a consumer-used evaluation,
eligibility, decision and reevaluation path. Preserve production defaults and consumer
authority. No universal score, parallel registry/router/evaluator/store, new root Go
dependency, unrelated consumer repair, gate weakening, release, tag or main merge.
Existing owners and methodology: [benchmark design](../designs/benchmark-evidence.md),
[assessments](../designs/engineering-assessments.md), decisions 0011/0012.
Necessary opt-in changes to resume_as_code/throughline are explicitly authorized by
the new objective; their local rules still apply. Prior dirty files remain untouched.

## Requirements → gaps → proof

- **OE-01 Consumer collection.** Existing RAC overlay exercises real red-team
  code, but not normal adoption. Add an opt-in normal-path collector/dependency upgrade;
  bind resolved configuration, planned tasks and every leaf attempt to retained receipts.
  Proof: actual CLI/integration tests, missing/duplicate-call negatives, failed/canceled
  work accounted, consumer revision and executed opt-in run. Missing costs stay unknown.
- **OE-02 Eligibility and decisions.** Catalog, ladder and failover already exist.
  Add consumer-owned approved deployments and deterministic hard-before-soft policy;
  separate claims/access/measurement, with bounded retry/failover/escalate/retrieve/
  review/reconcile/stop actions. Proof: reconstructed rule records, expired/unauthorized/
  unavailable candidates denied, no automatic production switch, outcome linked to decision.
- **OE-03 Identity and change.** Component fingerprints and strict comparisons
  exist. Materialize defaults; retain referenced artifacts; add dependency-aware proposals
  for exact deterministic reuse, regrading, price-only recalculation, affected reruns and
  fresh live measurements. Proof: dependency mutation table, alias/remote freshness and
  missing-artifact negatives; reuse never increments independent observations.
- **OE-04 Screening integration.** PromptScreener exists. Compose eligible client,
  admission, budget, cancellation, versioned refusal policy and independent effect/egress/
  output guards on the consumer path. Proof: malformed/error/uncertain holds, forbidden
  effects and canary disclosure absent, removed-control negatives and benign controls;
  races/speculative execution tested where present, otherwise explicit non-applicability.
- **OE-05 Evaluation protocol and labels.** Existing fixtures prove plumbing only.
  Freeze representative strata, label provenance, development/held-out split, graders,
  hard thresholds, practical margins, comparisons and budget/stopping rules before results.
  Proof: retained protocol/data/label digests, grader counterexamples and disagreement
  records, no holdout tuning or fabricated human labels/independent trials.
- **OE-06 Historical decision and study handoff.** Reconstruct the retained
  simple/no-model and classifier study; specify remaining live comparison/confirmation
  as the explicit 0014 TODO. Proof: historical live receipts; all-task
  success and latency distributions/usefulness timing, attempts, complete-or-unknown costs,
  false allows/refusals, review/error rates and actual effect outcomes. A supported retain,
  reject or inconclusive decision is valid. Deferred cross-provider/red-team trials
  remain unexecuted, not passing; further live work requires renewed explicit authority.
- **OE-07 Retrieval composability.** Reuse a named consumer retrieval need and
  existing metrics. Add bounded no-retrieval/normal/oracle-context diagnostics that
  distinguish retrieval, context assembly and generation failure, with permission,
  stale/citation/absence cases. Proof: a second domain's artifacts reconstruct through
  the shared assessment lifecycle; no speculative retrieval engine.
- **OE-08 Provider coverage.** Exercise supported Claude/OpenAI request/response,
  usage, failure and cancellation contracts with hermetic tests. Proof: explicit surface/
  version matrix; API access, CLI entitlement and native-host enforcement remain distinct.
  Unsupported or unobserved native behavior cannot be reported passing.
- **OE-09 Delivery.** Update intent/decisions and consumer instructions, compatibility,
  tracking and changelog with implementation. Resolve independent security/correctness
  review; run focused race/CUE tests, full required gates and clean-checkout verification.
  Record affected consumer revisions and baseline failures; conventional commits carry
  `Verified:`. Run Fathom verification before checking plan items; index proof supplements,
  never replaces runtime evidence. No unapproved merge/release.

## Shared-code verification

- [x] Historical derivatives remain available beside required rerun/live proposals without manufacturing trials. {symbol: PlanBenchmarkReuse, package: kits/eval-runner, reachable_from: TestPlanBenchmarkReuseGenerationChangeKeepsOnlyHistoricalDerivatives}

OE-01–09 are cross-repository operational requirements, not kit-symbol proxies.
Following GW-14's precedent, their delivery evidence is recorded below; consumer
symbols are independently verified in Fathom's `resume_as_code` service.

## Execution and token economics

One main integrator; bounded scouts/builders/reviewers, typically two or three workers.
Read routed sources once; short path/symbol/test reports, no full-history worker forks.
Separate worktrees for writers and disjoint ownership; batch coherent implementation
units, focused tests during edits, full gates at required commit checkpoints.
Reuse prior primary-source research; fetch only unresolved/version-sensitive questions.
Retain exact artifacts/grades where valid; no made-up savings or sample multiplication.

## Completion evidence

Consumer code is committed at `d233b5359f7cfcd5f46d6670a40385ca272f4618`;
shared kit code at `335c8fc40f1f57c746092de0af850d6a7d9c6642` is consumed through
the normal vendor pin. [Consumer delivery and exact artifacts](../../../resume_as_code/docs/evaluation/2026-09-21/README.md)
own the commands, identities, limits and reproducible analysis. Original live receipts
remain private/create-once, with public projections; no original run was rewritten.

| Requirement | Implemented path and executed proof | Status / limits |
|---|---|---|
| OE-01 | `RunStoredEvaluation` → collector/store; CLI, closure/duplicate/missing-call, error/cancellation and persistent spend tests; retained actual 67 dispatched campaign calls | PASS integration; 11 bills remain unknown, not free |
| OE-02 | `EvaluateResearchAdmission`, `RecommendShadow`, `DecideRecovery`; expiry/access/capability negatives, canonical leaf binding and JSON reconstruction | PASS; distinct bounded proposals, no production apply |
| OE-03 | Shared `PlanBenchmarkReuse`, consumer `PlanStoredEvaluationReuse` / `ApplyEvaluationReuseDerivatives`; mutation matrix and actual CLI regrade/reprice | PASS; four historical grades, hypothetical +5% cost $0.00295365, zero new trials; rerun/live actions still unexecuted |
| OE-04 | `RunEvaluationWork` composes versioned prompt screening, authority, no-tools and output guards; removed-control and provider error/cancellation tests | PASS implementation; CLI has no release callback or speculative effects; classifier effectiveness remains limited |
| OE-05 | Retained protocol/amendment, case/spec/label digests; `ReportEvaluationCollection`, grader counterexamples and task-cluster comparison | PASS protocol plumbing; authored synthetic labels, not human calibration or representative traffic |
| OE-06 | `BuildEvaluationOfflineDecision`; original 48-trial heldout verifies, 46 accepted, one false allow and one error; hard criterion reconstructs reject and two actual-cost STOP records | PASS historical decision/handoff; current-as-of reconstruction refuses expiry; additional live study DEFERRED |
| OE-07 | `ComputeEvaluationRetrievalDiagnostic` and existing RAC resolver; four no/normal/oracle controls, permission/stale/citation/absence negatives | PASS deterministic second-domain composition; no production RAG quality claim |
| OE-08 | `NewEvaluationAPIFactory` and actual OpenAI Responses / Anthropic Messages wire tests: schema/limit/usage/429/cancellation | PASS hermetic contracts; OpenAI live quota failure retained, native hosts untested here |
| OE-09 | Independent security/correctness review; full clean kit validation, full isolated consumer tests, focused race/CUE, separate clean build and CLI analyses; normal commit hooks | PASS scoped delivery; fleet-wide/main gates below remain non-passes |

The historical zero-false-allow condition failed. Aggregate success cannot buy away
that failure. The new typed threshold is honestly labeled post-study analysis; it is
not parsed from prose or backdated. Historical executable metadata says
`vcs.modified=true`; its exact hash is retained, and a clean-source claim was corrected.
Final offline analysis uses a separate clean checkout with matching revision and
`vcs.modified=false` (INC-17), not a stamp override.

No new model calls, metadata probes, credential reads or paid activity occurred in
this offline delivery. Campaign `operational-evidence-20260916` remains $0.040875
usage-calculated cost plus $0.395 unknown reservations = $0.435875 against its original
$5 cap; authority expired 2026-09-18. Original stages include failed OpenAI and Anthropic
calls, plain-JSON development holds, structured development and heldout, and no-model
controls. Complete details: [historical study](../../../resume_as_code/docs/evaluation/2026-09-16/README.md).

## Verification and remaining boundaries

- `task validate`: PASS on clean kit `335c8fc`, including full race/CUE/constitution.
- Consumer `go test -mod=vendor ./...`: PASS; focused race across ops/LLM/CLI/lint:
  PASS. Independent checkout build and ops/CLI/integration tests: PASS.
- Review repairs: exact deployment/model/provider/role/endpoint binding; invalid recovery
  bounds; JSON-decoded decision requests; canceled no-leaf CUE shape; historical
  derivation versus pending execution. Removed-control tests detect the bypasses.
- [Fathom evidence](../../../resume_as_code/docs/evaluation/2026-09-21/fathom-verification.json):
  eight consumer and two shared-kit static anchors pass. No runtime pass is inferred
  from static reachability. The unannotated operational list initially yielded unknown,
  not proof; this ledger now separates the symbol-backed kit row from external evidence.
- Throughline `a007075fab6ac156be44de46df941cce4a0915ed`: build stops at readonly
  module drift, as in the earlier checkpoint; no compatibility pass.
- Growing Gardens BI `7503dd1c270bf6ceb72d42c742f507b9513db0dc`: build stops at
  app-kit's missing `uniseg` checksum, also previously observed; no compatibility pass.
- Fathom/sextant `c58adaa2bf865e41c52eb00df92987cc96a089b1`: current build stops
  at an unrequired replaced evidence-producer module and a missing temporary embedded
  CSS asset. The former is newly observed current dependency drift; the missing asset
  was already reported. These failures occur before a useful compatibility result.
- RAC's normal vendored build and kit integrity/skew tests pass. Whole-vendor regeneration
  against the current unrelated app-kit sibling hits the same missing checksum; no
  unrelated dependency repair was smuggled in. Owner UI/artifact bookkeeping stays separate.
- The prior broad RAC race run's CLI walk `strings.Builder` race remains a non-pass.
  The file was byte-identical at baseline `7a6c56a` / `15f7306`; baseline reproduction
  stopped earlier, so independent reproduction is not claimed. Focused changed-code
  race coverage passes. Owner root theme-guide lint failures remain unrelated and untouched.

No gate is waived: `TRACKING.md` stays partial for fleet-wide readiness and main remains
protected. This does not defer required scoped implementation: those exits above pass.
Further empirical work alone is the owner's explicit
[study TODO](../../../resume_as_code/docs/evaluation/live-study-todo.md), reviewed
2026-10-16/next release or before a production/changed-solution claim. Cross-provider,
live red-team, new final holdouts and optimization remain unexecuted. No automatic model
watcher, new retrieval engine, worker platform or universal catalog was introduced.

## Intent and tool assessment

Scoped intent audit: 0013's missing-dependency revisit fired and the consumer binding
regressions close it. 0014 records the owner's deferral; 0011/0012's dated promotion
review remains due, not automatically satisfied by a small rejecting study. F1/F7/F8/F14
remain parked. Root dependency, CUE/Go ownership and experimental boundaries are intact.

| Scoped intent-doctor check | Result |
|---|---|
| Router/living budgets, required root docs | PASS: router 80 lines; living docs within their budgets |
| Local decision index, status/revisit shape, relative links | PASS: 14 local records indexed, including the owner's separate 0007 work |
| Native host version verification | SKIP: installed Claude 2.1.278 / Codex 0.155.1 differ from recorded verification; no new host enforcement claim |
| Whole-repository/owner's nested-intent migration audit | SKIP: separate work; this is not a full doctor certification |

Fathom was useful for narrow symbol locations, cross-service reuse callers, obligations,
continuity and static wiring. It did not replace reading the actual collector or running
tests. Costs: unknown unannotated operational checks, broad interface-derived test paths,
occasional slow responses and a heuristic builtin-`append` callee that did not appear in
the typed edge list. Prefer the typed edge view; keep cross-repository operational
receipts distinct from index verdicts. No measured token-savings claim is made.
