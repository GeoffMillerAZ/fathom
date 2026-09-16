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

## Deferred (explicitly not v0 — see design §5/§6)

- [-] `cmd` oracle (`runtime_pass`) {id: D0-1, disposition: deferred, reason: "O6 just landed; measure skip-share first", trigger: "skip_test share still >30% after 4 weeks of O6"}
- [-] STATUS.cue persisted diff / transition events {id: D0-2, disposition: deferred, reason: "stretch item E; needs a first machine consumer", trigger: "pilot metrics green at week 4"}
- [-] lexical/eval/human oracle wiring {id: D0-3, disposition: deferred, reason: "named consumers not yet asking; eval-runner is Stage-1 only", trigger: "first gated plan making an absence or behavioral claim"}
