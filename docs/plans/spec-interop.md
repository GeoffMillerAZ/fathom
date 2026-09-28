# Spec interop and the certified tier: the plan (vision 38, ADR-049)

Grammar v2 (`docs/plans/README.md`). A row flips to `[x]` only when
`fathom verify --service fathom --plan docs/plans/spec-interop.md` proves it.
Design: `docs/design/spec-interop.md` (section numbers below are its). Goal G19 (SI-M1..M4).
Promotes futures F20. The test-tied tier is the existing `certified_by:` (ADR-043); `tests:`
stays the `gone: callers` modifier (ADR-044) and is not reused.

## Wave 1: the port, Spec Kit, OpenSpec, the converge report, the grammar keys

- [x] The `SpecDialect` port and deterministic detection by path and content shape (§3) {id: SI-1, symbol: RegisterSpecDialect, package: internal/specdialect, reachable_from: TestSpecDialectRegistryDetect}
- [x] The native dialect wraps `ParseMarkdownPlan` with zero output change on every plan in `docs/plans/` {id: SI-2, symbol: nativeDialectParse, package: internal/specdialect, reachable_from: TestNativeDialectMatchesParseMarkdownPlan, needs: "SI-1"}
- [x] Spec Kit adapter: `specs/<feature>/tasks.md` tasks with `T###` ids, `[x]`/`[X]`, anchors from named paths and inline annotations; positions reported in the foreign file {id: SI-3, symbol: specKitParse, package: internal/specdialect, reachable_from: TestSpecKitAdapterInPlace, needs: "SI-1"}
- [x] OpenSpec adapter: `openspec/changes/<id>/tasks.md` and requirement ids from spec deltas {id: SI-4, symbol: openSpecParse, package: internal/specdialect, reachable_from: TestOpenSpecAdapterInPlace, needs: "SI-1"}
- [x] `fathom verify --plan <foreign file>` dispatches to the claiming dialect; `--dialect` overrides; exit codes 0/1/2 unchanged; foreign files byte-identical after the run {id: SI-9, symbol: dialectForPlan, package: cmd/fathom, reachable_from: TestVerifyForeignPlanInPlace, needs: "SI-3"}
- [x] The converge report (`verify --converge`): claimed-unproven, unanchored, and unclaimed code on the spec's branch range (§5) {id: SI-5, symbol: Converge, package: internal/specdialect, reachable_from: TestConvergeThreeSections, needs: "SI-9"}
- [x] `--write-fixups` appends tasks only inside the fathom converge block, in the dialect's own checkbox shape; text outside the markers never changes (law 15) {id: SI-6, symbol: writeConvergeBlock, package: internal/specdialect, reachable_from: TestWriteFixupsOnlyInsideMarkers, needs: "SI-5"}
- [x] `weight:` joins the grammar: positive integer, default 1, lint refuses anything else; parsed into `Task.Weight` {id: SI-7, symbol: parseWeight, package: internal/verify, reachable_from: TestParseWeightKey}
- [x] `accept:` joins the grammar and `annotationKeys` (I22): quoted acceptance text, never gating by itself {id: SI-8, symbol: parseAccept, package: internal/verify, reachable_from: TestParseAcceptKey}

## Wave 4: more dialects, the certified tier everywhere, Jev's proposals

- [x] Kiro adapter: `.kiro/specs/<name>/tasks.md` with `_Requirements:_` references resolved into `requirements.md` (EARS) {id: SI-10, symbol: kiroParse, package: internal/specdialect, reachable_from: TestKiroAdapterRequirements, needs: "SI-1"}
- [ ] BMAD adapter: story files with acceptance criteria and a task list {id: SI-11, symbol: bmadParse, package: internal/specdialect, reachable_from: TestBMADAdapterStories, needs: "SI-1"}
- [x] Task Master adapter: `.taskmaster/tasks/tasks.json` tasks and subtasks, `status: done` as claimed {id: SI-12, symbol: taskMasterParse, package: internal/specdialect, reachable_from: TestTaskMasterAdapterJSON, needs: "SI-1"}
- [x] GitHub task-list adapter over an issue or PR body exported to a file the user passes {id: SI-13, symbol: ghTaskListParse, package: internal/specdialect, reachable_from: TestGitHubTaskListAdapter, needs: "SI-1"}
- [x] The `Proof: certified` plan header: every `symbol:` row must carry `certified_by:` or is refused `certificate-absent` {id: SI-14, symbol: parseProofHeader, package: internal/verify, reachable_from: TestProofCertifiedHeaderRequiresCertificates}
- [ ] Foreign requirement-to-test references map to `certified_by:` on synthesized rows (Kiro, Spec Kit test paths, Task Master `testStrategy`) {id: SI-15, symbol: mapForeignTestRefs, package: internal/specdialect, reachable_from: TestForeignTestRefsBecomeCertifiedBy, needs: "SI-14,SI-10"}
- [ ] The adjudicated anchor sample: at least 60 foreign tasks with human-chosen anchors, committed as a fixture {id: SI-16, oracle: lexical, path: internal/specdialect/testdata/anchors/adjudicated.jsonl, pattern: "adjudicated_by", min: 60}
- [ ] Anchor proposals through the decision port (a Choice over at most eight `find_symbol`/`pack_context` candidates), shown only as proposals and only when SI-M4 passed; zero calls with `[decide] provider = none` (I42) {id: SI-17, symbol: proposeAnchors, package: internal/specdialect, reachable_from: TestProposeAnchorsGatedByCalibration, needs: "SI-5,SI-16"}

## Refused (recorded so they are not re-proposed)

- [-] Adopt a foreign spec format as canonical {id: SI-X1, disposition: refused, reason: "none carries anchors that make done computable (ADR-049 decision 6)"}
- [-] Reuse the `tests:` key for the test-tied tier {id: SI-X2, disposition: refused, reason: "`tests:` is a gone: callers modifier in the compatibility surface (ADR-044); certified_by: is the tier (ADR-043)"}
- [-] A spec authoring workflow or templates of our own {id: SI-X3, disposition: refused, reason: "the scaffolds own authoring; fathom proves (no separate SDD binary)"}
- [-] A model marking a task done {id: SI-X4, disposition: refused, reason: "ADR-049 decision 12; proposals are never proof"}

## Measurements (op)

- [ ] (op) SI-M1: 5 dialects verify in place on committed fixture projects, foreign files byte-identical after the run {id: SI-M1, oracle: lexical, path: docs/bench/readings/spec-interop/dialects.md, pattern: "^SI-M1 verdict: PASS", needs: "SI-3,SI-4,SI-10,SI-11,SI-12"}
- [ ] (op) SI-M2: the converge report on a real Spec Kit or OpenSpec project, section counts and a human check of 20 items {id: SI-M2, oracle: lexical, path: docs/bench/readings/spec-interop/converge.md, pattern: "^SI-M2 verdict: PASS", needs: "SI-5"}
- [ ] (op) SI-M3: a plan under `Proof: certified` refuses a row whose certificate is unproven and accepts it once live {id: SI-M3, oracle: lexical, path: docs/bench/readings/spec-interop/certified.md, pattern: "^SI-M3 verdict: PASS", needs: "SI-14"}
- [ ] (op) SI-M4: Jev anchor calibration on the adjudicated sample: top choice right at least nine in ten at the shown confidence, or the drop recorded {id: SI-M4, oracle: lexical, path: docs/bench/readings/spec-interop/anchor-calibration.md, pattern: "^SI-M4 verdict: (PASS|DROPPED)", needs: "SI-16"}
