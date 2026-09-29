# SDD reconciler v0 — plan grammar v2, wave gate, CUE shadow, Stop hook

Contract-bearing plan for ADR-018 / docs/design/sdd-reconciler.md. Gated by
`fathom verify` at pre-push. Every claim below names an index-provable symbol.

## Grammar v2 (internal/verify)

- [x] `[~]` parses as in-progress; `id:`/`disposition:`/`reason:`/`trigger:`/`skip:` annotation keys land on Task/Expect {id: G2-1, symbol: ParseMarkdownPlan, package: internal/verify, reachable_from: TestAdoptionServiceScoped}
- [x] Double-quoted annotation values may contain commas; quotes stripped {id: G2-2, symbol: parseAnnotation, package: internal/verify, reachable_from: TestBind_DriftedModule_FlagsRules}
- [x] `LintPlan` reports v2 convention violations deterministically (dup IDs, disposition without reason, disposition off `[-]`, trigger without deferred, skip_test without skip) {id: G2-3, symbol: LintPlan}
- [x] `GateRequireAll` fails open/in-progress tasks in addition to Gate semantics {id: G2-4, symbol: GateRequireAll}

## CLI verbs (cmd/fathom)

- [x] `verify --require-all` wave gate + lint warnings on stderr {id: V0-1, symbol: cmdVerify, package: cmd/fathom}
- [x] `gen-plan-cue` emits #Plan-conformant CUE and proves lossless round-trip {id: V0-2, symbol: cmdGenPlanCue}
- [x] Round-trip comparator rebuilds a Plan from emitted CUE and diffs task-for-task {id: V0-3, symbol: planFromCUE}
- [x] `stop-hook` computes newly-claimed tasks (working tree vs HEAD) {id: V0-4, symbol: newlyClaimedTasks}
- [x] `stop-hook` advisory/block output per design §4; silent exit 0 on operational failure {id: V0-5, symbol: cmdStopHook}

## Schema + docs

- [x] schema/plan #Task gains state/disposition/reason/trigger; #Expect gains package/skip_reason; valid/invalid testdata updated {id: S0-1, symbol: TestPlanSchema_Negatives, package: internal/verify, skip_test: true, skip: "the schema test IS the Test* anchor; nothing reaches a test"}
- [x] Grammar reference updated (docs/plans/README.md v2 keys); CHANGELOG entry; design doc + ADR-018 committed {id: S0-2, symbol: LintPlan, skip_test: true, skip: "docs deliverable anchored to the feature symbol per plans-README rule"}

## Grammar honesty defects (found 2026-09-29, fixed with the wiring wave)

Two defects that let a plan read healthier or emptier than it is. Neither is in force as a fix yet.

- [x] A wrapped row keeps its contract: `ParseMarkdownPlan` reads the annotation block (the braces) on a continuation line of a checkbox row, or `LintPlan` refuses a wrapped annotation by name and never lets it downgrade silently to a positional `md-NNN` row with no contract; the README's "Annotation placement" rule is updated to match {id: G2-5, symbol: ParseMarkdownPlan, package: internal/verify, reachable_from: TestParseMarkdownPlanWrappedAnnotationKeepsContract}
- [x] Deferred is not done: `depSatisfied` and the cross-plan branch of `rowEligibility` keep a row blocked while a needs target is a deferred `[-]` row, name a dependent of a refused row as needing re-planning (a superseded disposition would behave the same, should the grammar gain one), and never treat either as silently eligible; the README's `needs` row says so {id: G2-6, symbol: depSatisfied, package: cmd/fathom, reachable_from: TestDepSatisfiedDeferredBlocksAndRefusedNamesReplan}
- [x] (op) G2-7: the plans hit by G2-5, feature-map FM-1..FM-9 here and DP-1..DP-11 in agentic-kit, are re-verified after the fix and their moves from no-contract to proven, disproven or still unproven are counted in the reading {id: G2-7, oracle: lexical, path: docs/bench/readings/plan-grammar/wrapped-rows.md, pattern: "^G2-7 reading:", needs: "G2-5"}

## Deferred (explicitly not v0 — see design §5/§6)

- [-] `cmd` oracle (`runtime_pass`) {id: D0-1, disposition: deferred, reason: "O6 just landed; measure skip-share first", trigger: "skip_test share still >30% after 4 weeks of O6"}
- [-] STATUS.cue persisted diff / transition events {id: D0-2, disposition: deferred, reason: "stretch item E; needs a first machine consumer", trigger: "pilot metrics green at week 4"}
- [-] lexical/eval/human oracle wiring {id: D0-3, disposition: deferred, reason: "named consumers not yet asking; eval-runner is Stage-1 only", trigger: "first gated plan making an absence or behavioral claim"}
