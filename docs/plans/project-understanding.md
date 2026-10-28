# Project understanding: the evidence layer, measured per question type (PU-)

Grammar: v2. Prefix: PU- (PU-D records, PU-L intent statuses, PU-B the bench, PU-W the intent trace and the why card,
PU-C common-name guards, PU-I the ID3 signal, PU-V review measurement, PU-R risk, PU-U the use case, PU-M readings,
PU-G goal thresholds, PU-X deferred and refused). Design: `docs/design/project-understanding.md` (section numbers below
are its). Vision: `docs/vision/45-the-understanding-layer.md`. Decisions: ADR-084 to ADR-089. Goals: G46, G47, G48.
Roadmap: `docs/vision/38-proof-layer-roadmap.md` section 9.17. A row flips to `[x]` only when
`fathom verify --service fathom --plan docs/plans/project-understanding.md` proves it.

Gates. No row adds a table, column, index, cache, memo file or background job while the freeze holds (ADR-049
decision 2). No row adds an MCP tool (law 4): answers ride CLI verbs and additive parameters of registered tools. No
row fetches (I56): the bench binary clones benchmark repositories from operator-named sources at pinned commits. No row
spends model budget outside the `api` lane and the owner's cap (Q39). Every bar is fixed in its prereg before the first
run, every grader fails both stubs and passes PU-B4 before a reading counts, a reading short of its minimum detectable
effect reads UNDERPOWERED, and no agent reading runs before NP-P0's deadline-abandon work lands and NP-B1 to NP-B8 hold
(`docs/assessments/2026-10-26-next-phase-plan.md`). `fathom intent index --check` already runs in `task validate`, so
the record rows below claim only what their oracles read. Hosted channels wait on Q5 and are not rows (ADR-084
decision 6). Lanes: controller, owner, cloud, codex, api.

## P0: records, intent statuses and the bench (2026-10-28 to 2026-11-17)

### The records

- [ ] Six decision records ADR-084 to ADR-089 appended to `ADR.md`, each with Context, Decision, Decision models, Alternatives rejected with a Wins if per alternative, Consequences and Revisit when (lane: controller) {id: PU-D1, oracle: lexical, path: ADR.md, pattern: "^## ADR-08[4-9]: ", min: 6}
- [ ] Their six rows in the decisions index (lane: controller) {id: PU-D2, oracle: lexical, path: docs/intent/decisions/README.md, pattern: "^[|] ADR-08[4-9] [|] accepted", min: 6}
- [ ] ADR-018 carries the dated note pointing at ADR-087's status vocabulary (lane: controller) {id: PU-D3, oracle: lexical, path: ADR.md, pattern: "^[*][*]Amended 2026-10-28 [(]ADR-087[)]: a status vocabulary"}
- [ ] The assessment that condenses the research, the verdict and the open questions (lane: controller) {id: PU-D4, oracle: lexical, path: docs/assessments/2026-10-28-project-understanding.md, pattern: "^## Verdict$"}
- [ ] The assessment's row in the assessments index (lane: controller) {id: PU-D5, oracle: lexical, path: docs/assessments/README.md, pattern: "2026-10-28-project-understanding[.]md"}
- [ ] The design doc with its `## Decision models` section (lane: controller) {id: PU-D6, oracle: lexical, path: docs/design/project-understanding.md, pattern: "^## Decision models$"}
- [ ] Vision 45 (lane: controller) {id: PU-D7, oracle: lexical, path: docs/vision/45-the-understanding-layer.md, pattern: "^# 45 - The understanding layer"}
- [ ] Vision 45's row in the vision index (lane: controller) {id: PU-D8, oracle: lexical, path: docs/vision/README.md, pattern: "^45[.] [*][*][[]45 - The understanding layer"}
- [ ] Product vision's comprehension verdict amended with its date and record (lane: controller) {id: PU-D9, oracle: lexical, path: docs/design/product-vision.md, pattern: "Amended 2026-10-28 [(]ADR-084[)]: staffed at decision moments"}
- [ ] Vision 38 section 9.17: the phases, their exit tests and the program kill (lane: controller) {id: PU-D10, oracle: lexical, path: docs/vision/38-proof-layer-roadmap.md, pattern: "^### 9[.]17 Amended 2026-10-28"}
- [ ] Goals G46, G47 and G48, each with a bet block whose threshold a null reading fails (lane: controller) {id: PU-D11, oracle: lexical, path: docs/intent/scope/goals-3.md, pattern: "^[*][*]G4[678][.] ", min: 3}
- [ ] The three goals' router rows (lane: controller) {id: PU-D12, oracle: lexical, path: docs/intent/scope.md, pattern: "^[*][*]G4[678][.] .*scope/goals-3[.]md", min: 3}
- [ ] Futures F93 to F96, F94 the deferred project map with its Promote when on the bench's T2 reading (lane: controller) {id: PU-D13, oracle: lexical, path: docs/intent/futures/f92-f97.md, pattern: "^## F9[3-6][.] ", min: 4}
- [ ] The four futures' index rows (lane: controller) {id: PU-D14, oracle: lexical, path: docs/intent/futures.md, pattern: "^[|] F9[3-6] [|]", min: 4}
- [ ] Questions Q38, Q39 and Q40 (lane: controller) {id: PU-D15, oracle: lexical, path: docs/intent/questions.md, pattern: "^[*][*]Q(38|39|40)[.] ", min: 3}
- [ ] Q5 records the hosted channels it blocks (lane: controller) {id: PU-D16, oracle: lexical, path: docs/intent/questions.md, pattern: "every hosted channel"}
- [ ] Q4 records its effect on ADR-084's frame (lane: controller) {id: PU-D17, oracle: lexical, path: docs/intent/questions.md, pattern: "keeps ADR-084's"}
- [ ] ADR-074 carries the dated note, in its Status line and as a paragraph, that R1's wins-if counts unparsed statuses (lane: controller) {id: PU-D18, oracle: lexical, path: ADR.md, pattern: "Amended 2026-10-28 [(]ADR-087[)]: R1's wins-if counts unparsed statuses", min: 2}
- [ ] The CUE non-goal's Revisit when reads the unparsed-status count, not field-vet failures (lane: controller) {id: PU-D19, oracle: lexical, path: docs/intent/scope/non-goals.md, pattern: "exceeds five a month for two months [(]ADR-087 d4[)]"}

### Intent statuses (ADR-087, design section 7)

- [ ] `schema/intent/status.cue`: the closed status set per kind (decision, incident, future, goal, at most six each), the conditions that are never states, and the agentic-kit decision-lifecycle commit it was taken from, with valid and invalid testdata pairs the gate vets (lane: codex) {id: PU-L1, oracle: lexical, path: schema/intent/status.cue, pattern: "^#DecisionStatus:"}
- [ ] Go loads the embedded vocabulary; a test holds the embedded copy byte-identical to the source and refuses an unknown kind by name (lane: codex) {id: PU-L2, symbol: LoadStatusVocabulary, package: internal/intent, reachable_from: TestStatusVocabularyEmbedMatchesSource, needs: PU-L1}
- [ ] The normalizer folds every status spelling to one state of its kind, reads "amended <date>" as the Amended condition and never rewrites a file (lane: codex) {id: PU-L3, symbol: normalizeRecordStatus, package: internal/intent, reachable_from: TestNormalizeRecordStatusFoldsSpellings, needs: PU-L2}
- [ ] Unparsed status lines are counted per kind and named with file and line in `fathom intent audit --json` (lane: codex) {id: PU-L4, symbol: countUnparsedStatuses, package: internal/intent, reachable_from: TestUnparsedStatusesNamedWithFileAndLine, needs: PU-L3}
- [ ] SupersederResolves: a superseded record whose successor id has no heading reads False, naming both; a sibling-repository successor reads Unknown (lane: codex) {id: PU-L5, symbol: checkSupersederResolves, package: internal/intent, reachable_from: TestSupersederResolvesFlagsMissingSuccessor, needs: PU-L3}
- [ ] OutcomeResolves: a closed incident whose Outcome names no invariant, test, decision or plan-row id that resolves reads False; prose with no id reads Unknown, never guessed (lane: codex) {id: PU-L6, symbol: checkOutcomeResolves, package: internal/intent, reachable_from: TestOutcomeResolvesNeedsAResolvingID, needs: PU-L3}
- [ ] ReviewRecorded: a goal past its bet's review date with no dated keep or kill line on or after it reads False; a refused bet reads Unknown (lane: codex) {id: PU-L7, symbol: checkReviewRecorded, package: internal/intent, reachable_from: TestReviewRecordedNeedsADatedKeepOrKill, needs: PU-L3}
- [ ] `fathom intent audit --kind lifecycle` prints the three conditions and `--format sarif` gives each False condition its rule with file and line; nothing gates (lane: codex) {id: PU-L8, symbol: lifecycleSARIFRules, package: cmd/fathom, reachable_from: TestIntentAuditKindLifecycleSARIF, needs: "PU-L5,PU-L6,PU-L7"}
- [ ] The structured-bytes share per kind (bet blocks for goals, rating and condition lines for futures, annotation blocks for plan rows, status lines for decisions) in `fathom intent audit --json`, the instrument for ADR-074 R1's wins-if (lane: codex) {id: PU-L9, symbol: structuredByteShare, package: internal/intent, reachable_from: TestStructuredByteSharePerKind}

### The understanding bench (ADR-085, design section 8)

- [ ] (op) The understanding prereg `docs/bench/prereg/understanding.md`: each benchmark's licence, version, key provenance and pinned commits; types T1 to T5; arms A0 to A3 with A3's server named; one primary type per arm (A1: T4, A2: T3, A3: T1); a power analysis on LAB-37's variance with each minimum detectable effect; Holm across the other types; the typed-language stratum primary; the anchor format, both stubs, the leak audit, $/answered and the spend cap; drafted for the owner's sign-off (lane: cloud, then owner) {id: PU-B0, oracle: lexical, path: docs/bench/prereg/understanding.md, pattern: "^## Power analysis$", needs: "feature-labs.md#LAB-37"}
- [ ] Pinned public benchmark files (RepoProbe, MULocBench, ORCA-bench, GitHub ReviewBench, ApacheJIT) load into one item shape (type, question, repository and commit, key anchors, key provenance); an item with no pinned commit, licence or key source is refused by name; nothing is fetched (lane: codex) {id: PU-B1, symbol: loadBenchmarkItems, package: internal/benchprep, reachable_from: TestLoadBenchmarkItemsRefusesUnpinned}
- [ ] The gap-bank format `#GapQuestion` in `schema/bench/understanding.cue`: T3 or T5 only, the question, its public source at a commit, checklist items each with an anchor set, and a person's confirmation without which an item never counts toward a primary metric; valid and invalid testdata (lane: codex) {id: PU-B2, oracle: lexical, path: schema/bench/understanding.cue, pattern: "^#GapQuestion:"}
- [ ] The anchor grader: a checklist item is met when the answer cites an anchor the independent oracle (SCIP or ctags, never fathom) resolved from the reference answer; a review finding is met when its file and line fall inside the gold hunk; answers past the anchor cap are graded on their first cap anchors; the empty-answer and cite-the-central-symbols stubs both fail; the grader imports no fathom engine package (lane: codex) {id: PU-B3, symbol: gradeByAnchorMatch, package: internal/benchprep, reachable_from: TestAnchorGraderFailsBothStubs, needs: "PU-B1,PU-B2"}
- [ ] (op) PU-B4: the grader's agreement with person labels on at least 100 items drawn by seed across the benchmarks; PASS at Cohen's kappa of at least 0.6; FAIL voids every PU reading until the grader is fixed (lane: owner, then controller) {id: PU-B4, oracle: lexical, path: docs/bench/readings/understanding/grader-validation.md, pattern: "^PU-B4 verdict: (PASS|FAIL)", needs: PU-B3}

### $0 pre-readings

- [ ] (op) PU-M0: precision of 100 live intent doc links sampled by seed, each labelled by a person, and coverage on a seeded sample of product functions; PASS at a Wilson 95% lower bound of precision of at least 0.85; FAIL keeps doc links a labelled secondary and holds the why card (lane: owner, then controller) {id: PU-M0, oracle: lexical, path: docs/bench/readings/understanding/link-precision.md, pattern: "^PU-M0 verdict: (PASS|FAIL)"}
- [ ] (op) PU-M5: the status-edge backtest over 12 months of git history of `ADR.md` and `docs/intent/`, every transition classed legal or illegal against the vocabulary; 2 or more illegal edges revive PU-X20 (lane: codex) {id: PU-M5, oracle: lexical, path: docs/bench/readings/intent/status-edge-backtest.md, pattern: "^PU-M5 reading: ", needs: PU-L3}

## P1: the intent trace and the baselines (2026-11-17 to 2026-12-15)

### The intent trace (ADR-086, design section 5.1)

- [ ] Plan rows that cite a record (by title, spec id or the plan's header) are listed with their ids, checkbox states and contracts (lane: codex) {id: PU-W1, symbol: recordPlanRows, package: internal/intent, reachable_from: TestRecordPlanRowsReadsTitlesSpecIDsAndHeaders}
- [ ] Each listed row's last recorded verdict is read from the verify ledger with the commit it was recorded at, `not recorded` when absent; `VerifyPlan` is never called on this path (lane: codex) {id: PU-W2, symbol: recordLedgerVerdicts, package: internal/intent, reachable_from: TestRecordLedgerVerdictsNeverRecompute}
- [ ] `BuildIntentTrace`: record, rows, symbols by point query, rules citing the record, labelled doc links last, receipt commands; absence reads "no binding row, rule or link found" with the counts searched and the commit; an unreadable corpus reads not computed (lane: codex) {id: PU-W3, symbol: BuildIntentTrace, package: internal/intent, reachable_from: TestBuildIntentTraceAbsenceNamesReadSet, needs: "PU-W1,PU-W2,PU-C2"}
- [ ] `fathom why <record-id> [--json]` prints the trace (lane: codex) {id: PU-W4, symbol: cmdWhy, package: cmd/fathom, reachable_from: TestCmdWhyRecordTraceJSON, needs: PU-W3}
- [ ] The observatory's intent tab renders a record's trace beside the audit, with app-kit role utilities and CSS through `task ui:css` (lane: codex) {id: PU-W5, symbol: intentTabTrace, package: cmd/fathom, reachable_from: TestIntentTabRendersTrace, needs: PU-W4}

### Guards and the ID3 signal (design sections 3 and 6)

- [ ] Callers reached only through heuristic edges to a name defined in at least the configured count of places are set apart as `common-name` in `blast_radius` and `symbol_context`, counted and never dropped (lane: codex) {id: PU-C1, symbol: commonNameSetApart, package: internal/query, reachable_from: TestCommonNameHeuristicCallersSetApartAndCounted}
- [ ] Doc-link targets from intent units are package-qualified where the unit names a package, and a name-only target defined in at least the configured count of places is labelled `name-only`, never dropped (lane: codex) {id: PU-C2, symbol: qualifyIntentLinkTargets, package: internal/docindex, reachable_from: TestIntentLinkTargetsQualifiedAndLabelled}
- [ ] ID3: a path the router's sensitive-surface table names that changed in a range with no governing record linked is named with the path, the range and the units searched; `fathom intent audit` lists it (lane: codex) {id: PU-I1, symbol: ungovernedSensitiveChanges, package: internal/intent, reachable_from: TestUngovernedSensitiveChangesNamesPathAndRange}
- [ ] `review_diff` and `fathom review --since` carry ID3 as information, never a gate, with `docs/mcp-tools.md` updated in the same change (lane: codex) {id: PU-I2, symbol: reviewSensitiveSurfaceDimension, package: internal/review, reachable_from: TestReviewSensitiveSurfaceDimensionNeverGates, needs: PU-I1}
- [ ] The use case `docs/use-cases/project-understanding.md` (I36) with its `## Maintained` line naming `fathom why`, and its row in the use-case index (lane: cloud) {id: PU-U1, oracle: lexical, path: docs/use-cases/project-understanding.md, pattern: "^## Maintained", needs: PU-W4}

### Readings

- [ ] (op) PU-M1: the comprehension baseline, A1 against A0 on a RepoProbe slice, MULocBench's config subset and the confirmed gap bank, per type under the prereg; KEEP when A1 beats A0 on T4 at its minimum detectable effect, Holm-adjusted, with tokens at most 1.25 times A0's; each other type reads won, level or lost; results go to the labs ledger (lane: api, on the owner's cap) {id: PU-M1, oracle: lexical, path: docs/bench/readings/understanding/baseline.md, pattern: "^PU-M1 verdict: (KEEP|CUT|UNDERPOWERED)", needs: "PU-B0,PU-B4"}
- [ ] (op) PU-M4: the three lifecycle conditions' first runs over fathom's corpus and the vendored agentic-kit corpus, every report checked by a person; KEEP when the conditions find all 10 defects listed by hand on 2026-10-28 (design section 1: 9 overdue goal reviews and 1 pending incident outcome, each fixed since read at that day's commit) and at least 2 person-confirmed defects outside that list, with at most 20% false reports; KILL removes the conditions and keeps the vocabulary as documentation (lane: controller, then owner) {id: PU-M4, oracle: lexical, path: docs/bench/readings/intent/lifecycle-conditions.md, pattern: "^PU-M4 verdict: (KEEP|KILL)", needs: PU-L8}
- [ ] (op) PU-M9: cold and warm p95 and abandons for `localize`, `fathom why`, `review --since` and `blast_radius` on a public repository of at least 1,000 files right after an ingest, on a machine other than the operator's working host, with the host load recorded; PASS at cold p95 at most 3 s, warm at most 1 s and 0 abandons; until PASS no doc says real-time (lane: codex) {id: PU-M9, oracle: lexical, path: docs/bench/readings/understanding/latency.md, pattern: "^PU-M9 verdict: (PASS|FAIL)", needs: PU-W4}
- [ ] (op) PU-M10: install, cold ingest and first answer of the `fathom` CLI on a hosted CI runner for a public repository of at least 1,000 files, p95 over 10 runs, against a review's time budget; reading only (lane: codex) {id: PU-M10, oracle: lexical, path: docs/bench/readings/understanding/ci-runner.md, pattern: "^PU-M10 reading: "}
- [ ] (op) PU-M11: TypeScript and JavaScript call precision and recall on two public repositories, the no-SCIP fallback and imported SCIP each against SCIP truth; reading only, level claims follow I54 (lane: codex) {id: PU-M11, oracle: lexical, path: docs/bench/readings/understanding/ts-js-calls.md, pattern: "^PU-M11 reading: "}
- [ ] (op) PU-M12: ID3 on 10 seeded sensitive-path edits and 20 unchanged paths; KEEP at 9 of 10 found and 0 false on unchanged; CUT removes ID3 under a `gone:` row (lane: codex) {id: PU-M12, oracle: lexical, path: docs/bench/readings/intent/sensitive-changes.md, pattern: "^PU-M12 verdict: (KEEP|CUT)", needs: PU-I2}

## P2: the decisive readings (2026-12-15 to 2027-01-31)

### The why card, on its trigger (design section 5.2)

- [ ] `BuildWhyCard` for a symbol: binding plan rows with their last recorded verdicts and commits, certifying and covering tests, one pointer line to `obligations`, opt-in provenance (at most 3 commits, never an author); never calls `VerifyPlan` and never re-runs rule selection; built only when PU-M1 shows a T3 or T4 gap and PU-M0 reads PASS, else refused with both readings cited (lane: codex) {id: PU-W6, symbol: BuildWhyCard, package: internal/intent, reachable_from: TestBuildWhyCardReadsStoredVerdictsOnly, needs: "PU-M1,PU-M0,PU-W2"}
- [ ] `symbol_context` gains `include: ["why"]`, additive and inside its byte cap, its schema byte cost stated against ADR-053's default-surface cap, with `docs/mcp-tools.md` updated in the same change (lane: codex) {id: PU-W7, symbol: symbolContextIncludeWhy, package: internal/mcp, reachable_from: TestSymbolContextIncludeWhyAdditiveAndCapped, needs: PU-W6}
- [ ] The lab card `calibration/labs.d/lab-why-card.yaml`: the plugin with and without `include: why`, a firing criterion and the helped metric (ADR-082 decision 5) (lane: controller) {id: PU-W8, oracle: lexical, path: calibration/labs.d/lab-why-card.yaml, pattern: "^id: lab-why-card$", needs: PU-W7}

### Understanding and incidents

- [ ] (op) PU-M2: ORCA-bench's code-bearing tasks, one general agent with and without fathom on a local channel; KEEP when the anchor match against the benchmark's root-cause key beats the arm without fathom at its minimum detectable effect; the benchmark's own score is a labelled secondary (lane: api, on the owner's cap) {id: PU-M2, oracle: lexical, path: docs/bench/readings/understanding/orca-ablation.md, pattern: "^PU-M2 verdict: (KEEP|CUT|UNDERPOWERED)", needs: "PU-B0,PU-B4"}
- [ ] (op) PU-M3: arms A2 (A1 plus the intent trace and, if built, the why card) on T3 and A3 (A1 plus the named meaning-search server) on T1, each against A1; A2 KEEP at T3's minimum detectable effect, else `include: why` leaves under a `gone:` row; A3 above A1 on T1 promotes F95 (docs only) (lane: api, on the owner's cap) {id: PU-M3, oracle: lexical, path: docs/bench/readings/understanding/arms.md, pattern: "^PU-M3 verdict: (KEEP|CUT|UNDERPOWERED)", needs: "PU-M1,PU-W4"}

### Review (ADR-088)

- [ ] (op) The review prereg `docs/bench/prereg/review.md`: GitHub ReviewBench at a pinned revision with its licence, version and key provenance, Martian's offline set at a pinned key as a secondary, 100 clean merged PRs (no revert or fix within 30 days) and 100 PRs with one seeded structural gap from 3 public repositories after the models' cutoff, the anchor-match primary, a power analysis, Holm across finding kinds and both stubs (lane: cloud, then owner) {id: PU-V1, oracle: lexical, path: docs/bench/prereg/review.md, pattern: "^## Power analysis$"}
- [ ] The lab card `calibration/labs.d/lab-reviewer.yaml`: one public reviewer model and harness, named, with and without fathom's MCP server on a local channel, on ReviewBench at the pinned revision, same prompts, paired by PR (lane: controller) {id: PU-V2, oracle: lexical, path: calibration/labs.d/lab-reviewer.yaml, pattern: "^id: lab-reviewer$", needs: PU-V1}
- [ ] (op) PU-M6: `review_diff` precision per finding kind on the prereg's clean and seeded PRs; a kind keeps default prominence at a Wilson lower bound of precision of at least 0.80 and at most 0.1 false findings per clean PR, and is advisory by default otherwise (lane: codex, then owner) {id: PU-M6, oracle: lexical, path: docs/bench/readings/review/precision-floor.md, pattern: "^PU-M6 verdict: ", needs: "PU-V1,platform-engineering.md#RVU-1,platform-engineering.md#RVU-2,platform-engineering.md#RVU-3,platform-engineering.md#RVU-4,platform-engineering.md#RVU-5"}
- [ ] (op) PU-M7: the reviewer with and without fathom, only PU-M6's kept kinds on; KEEP when anchor-matched precision beats the arm without fathom at its minimum detectable effect with recall non-inferior by the prereg margin; false positives per PR and the key revision reported (lane: api, on the owner's cap) {id: PU-M7, oracle: lexical, path: docs/bench/readings/review/reviewer-ab.md, pattern: "^PU-M7 verdict: (KEEP|CUT|UNDERPOWERED)", needs: "PU-V2,PU-M6"}

### Risk (ADR-088 decision 5)

- [ ] `fathom review --since B --risk-features FILE` writes one row per touched symbol: fan-in, cross-service reach with its tier, covering and certifying tests touched or not, concern deltas, governing records touched, basis and head commit; information only, never a band, score or gate, never an author field (lane: codex) {id: PU-R1, symbol: riskFeatureRows, package: internal/review, reachable_from: TestRiskFeatureRowsInformationOnlyAndAuthorFree}
- [ ] (op) PU-M8: on ApacheJIT, a preregistered logistic regression fitted on a train split in the bench only and never shipped; KEEP when fathom's features plus size capture more defect-inducing commits at the riskiest 10% than size alone at the minimum detectable effect on the held-out split; CUT removes PU-R1 under a `gone:` row (lane: codex) {id: PU-M8, oracle: lexical, path: docs/bench/readings/review/risk-capture.md, pattern: "^PU-M8 verdict: (KEEP|CUT|UNDERPOWERED)", needs: PU-R1}

## Goal thresholds (each verifies only on a winning reading)

- [ ] G46's threshold: the baseline reads KEEP; on a powered CUT this row is refused with the reading cited (lane: controller) {id: PU-G46, oracle: lexical, path: docs/bench/readings/understanding/baseline.md, pattern: "^PU-M1 verdict: KEEP"}
- [ ] G47's threshold: the lifecycle conditions read KEEP; on KILL this row is refused with the reading cited (lane: controller) {id: PU-G47, oracle: lexical, path: docs/bench/readings/intent/lifecycle-conditions.md, pattern: "^PU-M4 verdict: KEEP"}
- [ ] G48's threshold: the reviewer reading reads KEEP; on a powered CUT this row is refused with the reading cited (lane: controller) {id: PU-G48, oracle: lexical, path: docs/bench/readings/review/reviewer-ab.md, pattern: "^PU-M7 verdict: KEEP"}

## Deferred and refused (recorded so they are not re-proposed)

- [-] A generated code wiki or architecture prose {id: PU-X1, disposition: refused, reason: "narrative is a lens (P14); stale prose without citation validation; ADR-084"}
- [-] A committed project-map file {id: PU-X2, disposition: refused, reason: "K5 rejects committed derived data; F65 stays speculative"}
- [-] An embeddings or vector store for meaning search {id: PU-X3, disposition: refused, reason: "ADR-041 and ADR-049 decision 8; arm A3 is a third-party server"}
- [-] The project map, with component clustering as its later refinement {id: PU-X4, disposition: deferred, reason: "overviews do not help agents until fathom's own reading says otherwise (ADR-086 decision 4)", trigger: "F94: PU-M1 or PU-M3 shows A1 losing or tying A0 on T2 at adequate power with at least half the misses naming the wrong component"}
- [-] A new MCP tool for any answer in this plan {id: PU-X5, disposition: refused, reason: "law 4 and ADR-053; CLI verbs and additive parameters serve every answer"}
- [-] A decision-port site proposing intent links for prose mentions {id: PU-X6, disposition: deferred, reason: "ADR-086 decision models", trigger: "a Wilson lower bound of precision of at least 0.90 on 100 person-labelled pairs and an A2 gain on T3"}
- [-] Finding-kind suppression by reviewer acceptance data {id: PU-X7, disposition: deferred, reason: "no acceptance data yet; PU-M6's floor decides prominence", trigger: "a local reviewer integration records acceptance for 30 days"}
- [-] A risk model, band or gate in fathom {id: PU-X8, disposition: refused, reason: "ADR-069 decision 5 and ADR-088 decision 5; the adopter's tierer owns calibration"}
- [-] Compliance catalogs, crosswalks, SSP or POA&M output, or scanners of fathom's own {id: PU-X9, disposition: refused, reason: "ADR-088 decision 4; the policy kit and review kit own them"}
- [-] Executing, proposing or recording a mitigation {id: PU-X10, disposition: refused, reason: "the incident command center non-goal, ADR-069 decision 8, IM-X3, IM-X9 and ADR-089 decision 5"}
- [-] A rollback-safety review dimension {id: PU-X11, disposition: deferred, reason: "ADR-069 decision 8", trigger: "incident-maps.md#IM-M0 reads BUILD"}
- [-] A portal fact collector (Backstage actions, Soundcheck, Cortex, Port) {id: PU-X12, disposition: deferred, reason: "no named consumer (P20)", trigger: "an adopter names a portal in the operator's private pack"}
- [-] A team glossary {id: PU-X13, disposition: refused, reason: "domain docs plus FTS docs search answer it; no measured gap"}
- [-] A read-only support endpoint {id: PU-X14, disposition: deferred, reason: "ADR-069 decision 14 is parked", trigger: "RT-M1 PASS and an index-snapshot phase 3 consumer"}
- [-] Promoting the pre-written MCP prompts {id: PU-X15, disposition: deferred, reason: "no call-log evidence of use", trigger: "a lab card shows helped"}
- [-] A question budget line in the AGENTS.md stanza {id: PU-X16, disposition: deferred, reason: "every always-loaded text change is A/B tested", trigger: "a lab card shows helped"}
- [-] Splitting big diffs by blast radius {id: PU-X17, disposition: refused, reason: "no evidence; the review cap already elides by name"}
- [-] Meeting the review kit's closure contract {id: PU-X18, disposition: deferred, reason: "the contract is unpublished", trigger: "the review kit publishes its closure contract"}
- [-] CUE-authored intent records {id: PU-X19, disposition: refused, reason: "ADR-074 R1 and ADR-087 decision 1; wins if a kind passes 50% structured bytes or 5 unparsed statuses a month for two months"}
- [-] Edge judgment between git endpoints for intent statuses {id: PU-X20, disposition: deferred, reason: "every measured gap is checkable at HEAD (ADR-087 decision 5)", trigger: "PU-M5 reads 2 or more illegal edges in 12 months"}
- [-] Transition rows in the intent ledger {id: PU-X21, disposition: refused, reason: "ADR-074 rejected review stamps in the intent ledger; ADR-087 decision 6"}
- [-] A field-vet failure instrument {id: PU-X22, disposition: refused, reason: "refused failures leave no trace; PU-L4 and PU-L9 measure the wins-if instead"}
- [-] ID5: a goal's cited rows going from proven to unproven since a base {id: PU-X23, disposition: deferred, reason: "the verify ledger admitted scratch-plan rows on 2026-10-28", trigger: "the verify ledger admits only registered roots, shown by an intent audit with no unregistered-root rows"}
- [-] PR review against a merge-base index overlay {id: PU-X24, disposition: deferred, reason: "F70", trigger: "index-snapshots.md#IXS-M5 reads KEEP"}
- [-] A new reading for drifted and dangling intent bindings {id: PU-X25, disposition: refused, reason: "staleness_report's doc_drift serves them and knowledge-retrieval.md#KR-M2 measures them (ADR-086 decision 3)"}
- [-] The why card's governing section built from doc links {id: PU-X26, disposition: refused, reason: "ADR-077's selector answers what governs a file; the card points at obligations (ADR-086 decision 2)"}
