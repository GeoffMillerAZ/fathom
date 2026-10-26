# Language onboarding: measure the remaining rounds

Proposal for [issue #16](https://github.com/GeoffMillerAZ/fathom/issues/16).
Status: research and proposed experiment, awaiting maintainer review. No implementation is authorized by this document alone.
Baseline: `743a914307baa286ccc5a6d8b92069910617b706`.

## The decision to make

Improve the point at which a worker receives evidence that its addition is incomplete. Do not build another language-creation framework. The current recipe already searches upstream queries and offers `lang-pin`, `lang-linguist`, `lang-draft`, `lang-probe`, `lang-gap`, `lang-loop` and `lang-tracker`. A new importer has to beat those tools on a measured failure class.

The proposed first experiment is cheap, offline prerequisite checks in the existing harness, using existing checks wherever possible. It must demonstrate earlier useful failures on historical submissions before becoming a default stage. Query import, new resolution engines, oracle installers and a workflow service are outside this proposal.

## Governing intent

G15 and ADR-048 require measured support, independent tiered oracles and generated tracking. I54 binds every support claim to its committed reading. I55 and ADR-048 decision 6 retain the source, exact pin and allowlisted license for borrowed files. GPL tools remain separate measurement processes, never vendored parsers. I38 requires incomplete evidence to be disclosed. I62 excludes judgments from proof.

ADR-048 already places the queue loop in the harness and excludes the decision port from extraction, classification, measurement and tracking. ADR-075 requires change-note fragments and generated-file regeneration. The proposed work changes neither support thresholds nor reading/manifest contracts. It does not reopen the general resolver rejection. No new applicable revisit trigger has been established by this census. The shared-strategy trigger in ADR-048 was addressed by its existing amendment.

F68's reference maps remain a future. Keeping ordinary pinned source links preserves that option without introducing a catalog subsystem, cache or background job. Reusable research and experiment guidance can be proposed to agentic-kit after it has earned a result. The checks themselves depend on fathom's manifests, reading schema and measurement entry point and belong here.

## Evidence and its limits

The dated [assessment](../assessments/2026-10-25-language-onboarding.md) owns the sample, source coverage, finding counts and upstream inventory. It records 21 observed revision/review rounds across six GPU-wave lanes using controller-authorized aggregate evidence, distinguishes requested rounds from landed outcomes, and leaves end-to-end elapsed time unmeasured. Named-round counts outside those six lanes remain unknown; individual reviewer-pass counts remain unmeasured in every lane. Commit counts are not substituted for rounds. A review comment is evidence of a reported finding, not independent proof that its proposed fix is correct.

Historical replay can show that a check would have caught a particular defect earlier. It cannot establish worker-hours saved or rounds avoided. Those outcomes require a prospective comparison. The initial experiment therefore claims only reproducible catches, false blockers and pre-oracle cost.

## Candidate boundaries

| Candidate | Existing baseline | Proposed disposition |
|---|---|---|
| Upstream starter importer | Recipe step 11 and `lang-draft`; existing grammar, Aider and Neovim reuse | Do not build now. Require repeated source/capture-conversion misses beyond current goldens/probe before reconsidering. |
| Offline preflight | `runLangLoop`, reading validation, tracker checks, grammar-lock checks and integration tests | Replay first. Reuse checks and move useful failures before oracle work. Add a missing predicate only when a measured event needs it. |
| Plan-row generator | Existing LB-Q contracts and lexical-oracle grammar | Do not build now. A generated row cannot make the underlying evidence independent or representative. |
| Recipe and skill corrections | Conflicting prose already identified | Propose a separate documentation unit after review. Keep one authoritative workflow and explicit limits. |
| Semantic or security auto-review | Independent oracle design and dedicated adversarial review | Keep human review. A mechanical preflight cannot prove representative corpus selection, oracle equivalence or resource safety. |

## Experiment contract

Freeze the replay cases, their bad heads, repaired controls, finding labels and expectations in the reviewed assessment before implementation. The [inventory](../assessments/data/2026-10-25-language-onboarding-cases.json) currently contains 20 reported groups, five candidates and zero documented repaired controls for those candidates. No check has passed a keep bar. Obtain a repaired control per positive case before measuring it. If controls cannot be obtained, report the case unmeasured and do not count it toward KEEP. Hold the baseline fixed at the commit above. Use only public source and repo-relative paths in committed evidence. Keep fetched subjects and tools under `workspace/`; use a fresh subprocess, never the daemon. Missing commits, archives or tools are `not measured`, not passing controls.

Keep the harness executable fixed at the baseline tree for every arm. Export the reviewed bad and repaired data/fixtures into isolated scratch roots and record their tree/content hashes. If a defect lives in harness code rather than inputs, define and review the exact minimal defect patch and paired repair applied to the same baseline, record both resulting tool-tree hashes, and use that identical pair across arms. Do not compare a current tool to an unrelated historical executable. A patch that cannot be isolated without changing the behavior under test makes the case unmeasured.

Each case also declares its scope: subject-file selection, per-item input, or repository-wide acceptance. E2-R1/R2/R3 require a recursive reading set; E2-R5 concerns plan/readings and cannot be satisfied by a per-item schema check or guessed prose parser. Only count a check when the replay demonstrably examines its required scope. Use existing repo-wide acceptance tests for repo-wide defects. A per-item loop stage cannot take credit for them by inspecting only its selected reading.

Compare three arms on the same cases:

1. Current `lang-loop` and the recipe's existing acceptance checks, with their current order.
2. Those same checks scheduled before the expensive oracle stage.
3. Arm 2 plus only the missing predicates justified by the frozen cases.

Record the defect caught, command, exit, stage, elapsed CPU/wall time, and whether the repaired control passes. Count each defect once even when several checks catch it. A test that was already required and catches the defect in arm 1 is a scheduling win at most, never a new detection. Report denominators separately for all events, mechanical events and events with replayable inputs. A warning, skipped dependency or missing input is not a catch.

Keep the reordered stage only if it catches at least three distinct mechanical defects across at least two lanes before oracle execution, falsely blocks zero repaired controls with at least one control for every counted defect, and adds no more than 10 seconds median warm-run overhead over the current cheap stage on the same machine (three alternating paired runs). Otherwise retain the current order and publish the null result. These are proposed bars to freeze at review, not measured outcomes.

Keep a new predicate only if arm 3 catches at least two distinct frozen defects that arms 1 and 2 miss, with zero false blockers on all repaired controls and at least one repaired control per counted defect. Otherwise remove it. Do not stretch one failure into multiple defects or adjust the cases/bar after results. A useful reorder does not justify a new command or validator.

A later claim about fewer rounds needs prospective evidence: at least five new additions with the same review protocol, reporting all findings and worker/reviewer time against an explicitly selected baseline. Until then, call savings a hypothesis.

## Proposed implementation after the experiment passes

Choose the integration seam from the retained check’s proven scope. For a retained per-item/subject check it is `cmd/indexbench/langloop.go`'s existing `runLangLoop` path, before `ops.measure` on fresh measurements. For a repository-wide check it is the existing acceptance-test path, not a new per-item claim. A per-item preflight leaves committed readings and generated files untouched, and performs no installation, network fetch, daemon call or oracle execution. Reading-only replay retains its explicit skip disclosure and cannot claim a fresh preflight passed. Tests inject the measurement operation and prove that a failing prerequisite never invokes it.

Prefer existing checks for manifest/reading shape, dispatch wiring and generated consistency. Verify source archive identity only if the actual pinned archive is available locally. A syntactically valid digest is not proof that it hashes the intended archive. Do not reimplement the parser, grade rules, license policy or the full validation gate. Report unreadable inputs as errors and unavailable optional evidence as unmeasured. Keep output bounded, deterministic and actionable with paths and the existing recipe step.

Any new metadata, test name or output change must be specified in the reviewed replay result before integration. No new store schema, cache, memo file, scheduled work or persistent runtime catalog is permitted. The committed replay measurement is a research artifact, not a runtime state file.

## Guidance changes proposed for review

Correct the initial dependency paragraph so a deferred `[-]` prerequisite never means done. Describe scratch measurement followed by guarded `WRITE=1` instead of writing a committed reading directly. Require an actual successful verify run before a controller claims a row, rather than saying it "would pass". Replace the old changelog-edit instruction with a unique ADR-075 fragment.

Keep the existing upstream search order, and make workers record why each candidate was accepted or rejected: pinned source, file-specific license, capture/predicate compatibility and independence from the chosen oracle. The Bash Aider adaptation shows why a query that compiles can still emit no names. Fixtures/goldens and `lang-probe` remain the evidence of capture behavior.

## Proposed decision record, without an allocated ID

Prefer replaying existing checks earlier in the current language loop. Reject a general query importer and a second onboarding framework for now. Wins if repeated onboarding findings concern query conversion that the current draft/probe path cannot diagnose, or the frozen replay demonstrates a missing mechanical check at the declared bar. Reject the preflight too if the replay fails its keep criteria. Record that null result in this design and the assessment before stopping.

This is a draft for the maintainer to accept, amend or reject. No ADR, future, invariant or observation ID is allocated here.

## Decision models

None applies. Finding counts, source identity, capture compatibility, schema validity and replay outcomes are deterministic evidence questions. Provider `none` is the complete path. No model call, judged grade or paid API is introduced.

## Review and proof

The [proposed plan](../plans/language-onboarding.md) keeps every row open. The maintainer reviews this design, assessment, frozen cases and keep/kill bars before any implementation. If approved, use failing tests first and race tests on touched packages. An accepted design is distinct from implemented behavior. Historical findings remain reported findings until replay reproduces them.
