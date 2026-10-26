# Language onboarding: measure the remaining rounds

Proposal for [issue #16](https://github.com/GeoffMillerAZ/fathom/issues/16).
Status: revised after controller review; awaiting Q1 approval. No implementation is authorized.
Census cutoff and reference checkset: `743a914307baa286ccc5a6d8b92069910617b706`.

## The decision to make

Keep upstream reuse in the existing harness and reject a general importer for now. The recipe already searches grammar queries, Aider and Neovim and provides `lang-pin`, `lang-linguist`, `lang-draft`, `lang-probe`, `lang-gap`, `lang-loop` and `lang-tracker`. None of the 20 GPU review groups concerns query conversion.

The revised census separates two hypotheses. Moving existing checks before an addition's oracle has insufficient eligible historical cases, so that experiment is not run. Two public hygiene repairs justify a different, narrower experiment: add checks at **pre-submission**, after oracle work and before commit/review. Its possible benefit is fewer mechanical review escapes, not less oracle work. No runtime change follows merely from approving this research proposal.

## Governing intent

G15, I54 and ADR-048 bind support claims to independent tiered readings and generated tracking. I55 and ADR-048 retain source, exact pin and license for borrowed files. I38 requires incomplete evidence to be disclosed. I62 keeps judgments out of proof. ADR-048 already places the queue loop in the harness and excludes decision models from classification, extraction, measurement and tracking. ADR-075 requires change-note fragments and generated-file regeneration.

The proposed checks change no support thresholds, manifest/reading contracts or resolution strategy. F68's reference maps remain a future; ordinary pinned links preserve that option without building a catalog. No newly applicable revisit trigger is established here. Reusable guidance may be proposed to agentic-kit after a measured result; the current research does not claim an upstream contribution.

## Evidence and source boundaries

The [assessment](../assessments/2026-10-25-language-onboarding.md) reports 15 additions, nine follow-up defects (seven mechanical, two semantic), a separate 20-group GPU review sample, and 21 named review/revision rounds across six GPU lanes. It lists findings per addition and public bad/fixed pairs. Requested rounds are distinguished from landed outcomes. Individual reviewer-pass counts and end-to-end worker time remain unmeasured.

LH-R1 authorizes aggregate private rounds, outcomes, finding classes and numerical readings, including historical per-round checkpoints. Only those aggregates are published. The defect-pair census is independently public git evidence. PR #12 and #23 review/inline endpoints were retried successfully (zero entries each; two discussion comments each), closing the earlier access gap. Historical review completeness and causal time savings are not inferred from that response.

## Feasibility before any replay

The [mechanical inventory](../assessments/data/2026-10-25-language-onboarding-mechanical.json) records each defect's phase, attribution, bad parent, repair and check availability. Required integrator regeneration is excluded. E2-R1..R5 are repository-wide harness-rule changes outside the onboarding experiment; C5-R2 is a CUDA-specific measured-set adapter. Neither supplies generic onboarding catch credit. The shared receipt-golden defect and late `exec-noctx` gate pair remain in the census with their actual integration scope, rather than being hidden or assigned to a language.

| Arm/hypothesis | Eligible cases and scope | Public controls | Maximum before replay | Bar and disposition |
|---|---|---|---|---|
| B: existing pre-submission checks, reference | Same staged snapshots used by N; compare the full frozen mechanical checkset as a separate baseline validity check | Objective-C and PgBouncer repairs; other clean controls specified below | 2 candidate bad/control pairs; actual baseline acceptance unmeasured | Reference validity can pass or fail. It has no intervention KEEP bar. |
| S: existing checks before first oracle | Nix format only. Thrift format/backup were merge-introduced; WGSL heading appeared after first reading; golden/exec are shared integration | Nix bad/fixed pair available | 1 eligible defect, 1 addition; confirmed from source, no arm replay | Original bar >=3 defects across >=2 additions is infeasible. Record the feasibility null now; do not spend trials to obtain a predetermined KILL. |
| N: new pre-submission hygiene predicates | Objective-C literal machine-path fallback; PgBouncer tracked bytecode. Both staged-source scope, after oracle and before review | 2 public bad/fixed pairs; checker and specificity trials still unmeasured | Conditional ceiling 2 incremental defects across 2 additions | Requires >=2 incremental catches across >=2 additions, zero false blockers and the defined cost bar. It can win or lose. |

A public repair pair establishes an input/control identity, not proof that all baseline checks accept it or that a new predicate is sound. If B already detects one of N's two defects, N fails the novelty bar. Missing tools, invalid controls or uninterpretable patches make the result indeterminate. Do not add a third case after seeing results to rescue the bar.

## Scheduling null and rejected-for-now decision, without an allocated ID

Reject the **pre-oracle reordering experiment on this population** for now: its eligible ceiling is 1 defect in 1 addition, below the unchanged 3-defect/2-addition bar. This is a feasibility null, not a measured treatment failure or evidence against earlier integration gates in general. The computed feasibility result is in the [criterion file](../assessments/data/2026-10-25-language-onboarding-criteria.json).

Wins if a future census supplies at least three addition-specific defects across at least two additions, each demonstrably present before oracle work, with an existing required detector and a public bad/fixed pair. Integration-only defects require their own scheduling question and population. Do not relabel them to satisfy this trigger. No trial ledger row is written for S, and its future cost or false-blocker count is not reported as zero.

The general importer is also rejected for now. Wins if repeated query-conversion defects remain after the existing draft/probe path and a separate measured comparison demonstrates useful incremental detection. No new ADR/F/I/O identifier is allocated by either draft decision.

## Active experiment: B versus N at pre-submission

Freeze `language-onboarding-replay-v2` before implementation. The [criterion specification](../assessments/data/2026-10-25-language-onboarding-criteria.json) records source cases, independent arm definitions, thresholds, scoring rules and the current unmeasured state. The reviewed freeze must add exact staged-index/tree hashes, checker revisions, command argument arrays and all controls. Changing any after replay requires a new criterion version and a new run, never rescoring a failed v2 as success.

For each case, use one common clean replay base and isolate only the reviewed defect/repair hunks from the public pair. The two fixes bundled in `441b225e6` must be split: N's Objective-C case cannot include the Solidity chronology repair. Materialize identical staged-index snapshots as working files for B and N, since existing gofmt/CUE hook commands read working files while debris checks read the index. Record common base, source pair, patch, index and checker hashes. If isolation changes the defect's meaning, mark the case unmeasured. No historical private path literal or binary artifact is copied into committed fixtures; fetched source stays under `workspace/`, with public revision references and digests in the report.

B's timed stage is the existing `lefthook.yml` **pre-commit** command set at the reference revision: `merge-debris --staged`, `go-fmt-check` on staged Go files, and `cue-vet` on applicable staged CUE files. Freeze their expanded commands and selected paths. B contains no new `git diff --check` step. The actual hook declares `parallel: true`; the replay conservatively runs this same set serially in a fixed order in both arms, then N's new predicates. Claim only serial-stage overhead. Changing the real hook's parallel schedule is a separate integration decision.

N preserves B's checks and adds only:

1. Inspect staged Python oracle source under `calibration/oracles/` for `os.environ.get` or `os.getenv` with a constant absolute environment fallback rooted in a personal-home or mounted-volume directory. Match syntax and root category, not the historical path or variable name. Standard system roots, relative/default-free forms and dynamic expressions are separate benign or uncovered cases. Diagnostics show repo-relative path, line and rule only, never the literal private value.
2. Reject staged added/copied/modified Python bytecode (`.pyc`/`.pyo`) under `calibration/oracles/`. Read index paths, not filesystem leftovers; do not flag deleted paths or artifacts kept outside the submitted tree.

Baseline novelty is checked against the frozen **mechanical acceptance set**, not an invented claim that the whole repository validation gate ran: the pre-commit commands above, `TestNoOrgSpecificNames`, `TestCmdExportReceiptRedactGoldens`, `TestTenancyRules` and `changelog-check`. Existing named checks at both historical bad and reference revisions are recorded separately. Run the selected checks offline on isolated cases; do not run the forbidden full `task validate`. A selected check that cannot execute leaves novelty unmeasured. A source inspection saying a check skips a path is not a passing replay.

Controls include each public repair, explicitly cleaned staged snapshots for the other 13 census additions, and synthetic benign controls for system-library fallback, a relative/default-free form, a dynamic fallback, and a deleted bytecode path. Freeze exact control revisions, selected staged paths, index hashes and expected B acceptance at review. Do not use an uncorrected first-reading or merge tree as a clean control: Nix needs its format repair, Thrift its merge repairs and WGSL its heading repair. Prefer the cleaned reference-cutoff tree for the other snapshots. B acceptance must be demonstrated, not assumed; a failed control after freeze invalidates the run rather than permitting a replacement chosen after results. Every counted bad-case catch needs its paired repaired control; all controls must be accepted by B. An N rejection of any such control is a false blocker. Unexpected baseline-control failure is an invalid experiment input, not an N success.

## The 10-second stage and computed criteria

On each repaired/benign control, warm both command sets once without retaining timing samples. Then run three alternating B/N pairs (reverse order on the second pair) with the same staged index, environment and warmed caches. The wall-clock interval starts at entry to the fresh serial pre-commit check process and ends when all selected checks return, before a commit object is created. It includes child-process startup and check execution; excludes oracle work, source downloads, tool builds and the prior warmup. Record raw wall and CPU samples. Bad-tree fast exits do not prove overhead.

For each control compute `median(N_wall[i] - B_wall[i])` over its three pairs. **Every** control median must be <=10 seconds. Report the maximum of those medians; do not use a pooled median that can hide a slow control. `trials` in an arm ledger row is the minimum paired timing count over all required controls, never the total number of subprocesses.

After approval, a small pure replay scorer must derive counts and `evaluation.{criterion_id,verdict,reason}` from the frozen criteria and case receipts. No current bench evaluator is claimed to implement these bars: `scenariobench` scores success/token criteria; the existing verify ledger oracle checks row identity, nonvoid status, trial floor and declared verdict, not the raw arithmetic. Reuse its JSONL envelope, not an incompatible scoring policy.

The scorer must implement:

- **B validity:** pass only when all frozen controls execute and pass and the mechanical baseline outcome is recorded for both bad pairs. Missing input/receipt or invalid control yields `indeterminate`.
- **N KEEP:** all required receipts valid; >=2 distinct bad cases across >=2 additions rejected only by N; each B bad case accepted by the frozen mechanical set; every control accepted by B and N; at least three paired timing samples per control; every median delta <=10 seconds. A fully observed failed threshold computes `fail`/KILL. Missing or invalid evidence computes `indeterminate`/NOT_MEASURED in the diagnostic report and emits **no claimable ledger row**, even if three timing pairs happened to finish. Counts never multiply by repeated trials.
- **S feasibility:** fail/KILL_FEASIBILITY from eligible ceiling 1<3 and additions 1<2. No replay trials, timing, catches or false blockers are invented.

Only complete pass/fail results emit ledger rows; `accept_verdict: any` is reserved for a fully observed KILL, not incomplete functional evidence. The report and ledger share criterion/receipt digests. The scorer renders the design's outcome count line from the report; nobody types KEEP into it. Tests after approval must refute a forged verdict, one missing repaired control, a baseline-detected case counted as new, an omitted benign control, duplicate cases counted twice, and a single slow control hidden by a pooled median. No scorer or runtime predicate is implemented in this research branch.

## Guidance and integration boundaries

Correct stale recipe guidance as a separate reviewed documentation unit: deferred dependency states, scratch measurement versus guarded `WRITE=1`, actual verify evidence, and ADR-075 fragments. Preserve the existing upstream search order and source/license/oracle-independence checks. A query that compiles still needs hand-written goldens and `lang-probe`.

Any KEEP gets a separately reviewed implementation contract at the matching pre-submission hook/check seam. The active N proposal is not a `runLangLoop` pre-oracle stage and cannot claim oracle-time savings. It adds no store table, cache, memo file, daemon, scheduler, installer or model call. A KILL removes the experimental predicates and records rejected-for-now conditions. A later claim of fewer review rounds needs prospective evidence from at least five additions with a fixed review protocol; historical catches alone do not establish saved hours.

## Decision models

None applies. Source identity, staged artifacts, syntax predicates, counts and timing criteria are deterministic. Provider `none` is the complete path; no judgment or paid API enters proof.

## Review and proof

The [revised plan](../plans/language-onboarding.md) leaves every row open. Q1 approval is required before the experiment's scorer or predicates are written. The S feasibility null is already recorded as research; B/N outcomes remain unmeasured. Exact tests and product integration require later contracts. No runtime behavior or support level has changed.
