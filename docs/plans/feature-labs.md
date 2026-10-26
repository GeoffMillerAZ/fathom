# Feature labs: lab cards, the firing funnel, the results ledger and the report-to-regression loop

Grammar: v2. Prefix: LAB- (LAB-n work rows, LAB-Mn readings and kill criteria, LAB-Dn deferred options, LAB-Rn
refusals). Design: `docs/design/feature-labs.md` (section numbers below are its). Issue:
https://github.com/GeoffMillerAZ/fathom/issues/13. Decisions applied: ADR-049 decision 2 (the table freeze), ADR-011,
ADR-029 decision 6 (measurement stays Claude-only), ADR-034 decision 2, ADR-046 and I46, ADR-073, ADR-080.

Scope: cards, cases, corpora, ledgers and readings are files. No store table, column, cache or background job. P0
spends $0.

P1 adds the spend the owner has approved, recorded in `docs/intent/autonomy.md` (Held) in this change:

- decision-model experiments through the port, which need no ask since 2026-10-22;
- agent sessions, experiments and benchmark sessions, which may spend agent budget since 2026-10-26 in a fixed
  order: harness first, then the no-claim shakedown, then preregistered runs.

Both rulings were given in session, so the controller confirms them from the owner's words before merge. Agent
budget goes first to parallel sessions that build the harness rows (LAB-1, LAB-2, LAB-12, LAB-13, LAB-15, LAB-19,
LAB-20, LAB-21, LAB-33), then to the no-claim shakedown (LAB-37). P2's preregistered runs spend inside an envelope
the owner sizes from LAB-37's prices, and those rows stay deferred until then. K1 to K6 are the design's kill criteria (section 5). Q1 to Q8 are the design's
owner questions, not the ids of `docs/intent/questions.md`.

Lanes, the first word of each title:

- `codex`: a helper-machine Codex lane working from a written brief.
- `cloud`: a cloud lane.
- `api`: an API-funded agent session under the 2026-10-26 lift, or a provider session through the decision
  port.
- `ctl`: the controller only.

The kit rows (LAB-13, LAB-15) go in one brief with bench-prep's BX-25, BX-37, BX-38, BX-39 and BX-64. They are the
same package, need the agentic-kit checkout beside fathom's, and land through one `task sync-kit`. A kit statistics
row is claimed with the row that wires it into a fathom entry point, never with `library: true`.

Shared with bench-prep (`bench-prep.md`, on its own branch), one record each and never a labs copy:

- pins (BX-14 `RecordArmPins`);
- the prereg template (BX-24);
- the knob map file (BX-28);
- the held-out read log (BX-29);
- the hermetic fixture set (BX-30);
- arm declaration and parity (BX-59, BX-60);
- the shakedown reading (BX-70);
- the scheduled lane (BX-74).

A row that waits on a bench-prep row cites it in its title, not as a `needs:` link, because `bench-prep.md` is not on
this branch and lint refuses a missing sibling. Whichever plan merges second turns each citation into a
`bench-prep.md#BX-n` link.

Where files live:

- readings: `docs/bench/readings/labs/`;
- cards and cases: `calibration/labs.d/`;
- results: `docs/bench/labs-ledger.jsonl`;
- fix proofs: `docs/bench/readings/labs/fix-proofs.jsonl`;
- local corpora, labels, real prefixes and raw runs: `workspace/labs/`.

Verify before claiming any row: `fathom verify --service fathom --plan docs/plans/feature-labs.md`.

## P0: registry, corpus, firing reading, ledger and the loop's first links ($0, starts now)

### Registry (sections 4.2 and 4.3)

- [ ] codex: `schema/labs` CUE package with `#Lab` (id, features for cross-reference, surface, firing with its should and should-not rules and grader, helped with metric, the minus-one-feature arm, direction and MEI, quality naming the instrument that owns it, fixtures, harness, kill with a reading each, knobs with kind and location, owners), `#Case` (the kit scenario-bank `#Scenario` plus lab, should_fire, split as train, select or confirm, regression, observation, theme and derivation) and `#Result` (one row per run, task and arm, the funnel stages as fields, section 4.6), valid and commented invalid testdata {id: LAB-1, oracle: lexical, path: schema/labs/schema.cue, pattern: "^#Lab: "}
- [ ] codex: `LoadLabs` reads `calibration/labs.d/*.yaml` against `#Lab` and refuses a file name that is not the card id, an unknown surface key, a firing block with no should-not rule, a helped arm that removes more than its own feature and a card with no kill criterion; every negative in `invalid.cue` is asserted through the CUE Go API {id: LAB-2, symbol: LoadLabs, package: internal/labs, needs: LAB-1}
- [ ] codex: `CheckLabClosure`: every shipped surface the caller passes in appears in a card's `surface` or in `no-lab.yaml` with an instrument or a reason, and an unknown surface key in either is refused; scorecard and FM-1 feature ids are carried for cross-reference and never decide closure {id: LAB-3, symbol: CheckLabClosure, package: internal/labs, needs: LAB-2}
- [ ] ctl: the 16 seed cards of section 4.3 committed with their firing, helped, quality and kill choices, indexed one table row each in `calibration/labs.d/README.md`; a test loads every committed card through `LoadLabs` and asserts at least 16, each with a should-not rule, a minus-one-feature arm and a kill {id: LAB-4, symbol: LoadLabs, package: internal/labs, reachable_from: TestCommittedSeedCardsLoad, needs: LAB-2}
- [ ] codex: `shippedLabSurfaces` lists what fathom ships (`registeredToolNames`, `hookSpecsFor`, the skills `fathom plugin render` writes from `skillsdist.Names`); `calibration/labs.d/no-lab.yaml` names each shipped surface no seed card covers with its instrument (`indexbench` L0, `reviewbench`, `langbench`, a bench-prep family) or `none` with the reason a firing question does not apply; a test runs `CheckLabClosure` over the committed files and fails on any shipped surface with neither {id: LAB-5, symbol: shippedLabSurfaces, package: cmd/fathom, reachable_from: TestCommittedLabsCloseOverShippedSurfaces, needs: "LAB-3,LAB-4"}
- [ ] codex: the labs section of `fathom instrument --adoption` (`adoptionLabsSection`) prints the cards, closure gaps, the newest ledger row per lab, case counts by split, fixed observations without a fix proof and observation links, in the text and `--json` reports; it reads files only, never the daemon, and never spends; there is no `fathom labs status` verb {id: LAB-6, symbol: adoptionLabsSection, package: cmd/fathom, reachable_from: cmdInstrumentAdoption, needs: "LAB-5,LAB-12,LAB-16"}

### Session corpus and the eligibility classifier (section 4.4)

- [ ] codex: `DecodeTranscriptEvents` in a shared `internal/transcript` package: the line and block walk of `fathom readout replay` moved out of `cmd/fathom` with its tests unchanged, returning ordered events (`tool_use` name and input shape class, `tool_result` error flag and byte count, `hook_additional_context` with its hook event, Skill invocations, subagent boundaries, and each line's `cwd`, `gitBranch` and timestamp); no text is kept, events are deduplicated by tool-use id, and malformed or unreadable lines are counted and named (I38); readout replay and the labs both read it, so no fourth decoder exists; Claude-only (ADR-029 decision 6) {id: LAB-7, symbol: DecodeTranscriptEvents, package: internal/transcript, reachable_from: cmdReadoutReplay}
- [ ] codex: `ExtractDecisionPoints` cuts the events into decision points: a session hash from `bundle.LoadOrCreateSalt` and `bundle.HashService`, the turn index, the preceding calls, the call made and its shape class, and the next three calls {id: LAB-8, symbol: ExtractDecisionPoints, package: internal/labs, needs: LAB-7}
- [ ] codex: `ClassifyDecisionPoint`, the eligibility table v0 for the choices the agent makes: for each agent-surface verb and shipped skill a point is should-fire, should-not or unclassified, never guessed (hook seams read AD-20's eligible log instead); the definition check is a go/parser declaration scan of the tree at the commit on the session's `gitBranch` nearest its timestamp (pure Go, ADR-066), never a fathom answer (section 7 item 8); a tree that cannot be recovered leaves the point unclassified and counted (I38); every rule has a table test with a near-miss negative {id: LAB-9, symbol: ClassifyDecisionPoint, package: internal/labs, needs: LAB-8}
- [ ] codex: `fathom labs corpus` (`cmdLabsCorpus`) walks this repository's local transcripts (every project only on Q3's answer) into `workspace/labs/sessions/` and prints aggregates only {id: LAB-10, symbol: cmdLabsCorpus, package: cmd/fathom, needs: LAB-8}
- [ ] codex: `FiringReading` computes the funnel per lab: eligible (from `ClassifyDecisionPoint` for verbs and skills, from AD-20's log for hook seams where it exists, else "not measured" with the reason), fired, delivered, acted on (from AD-21 or RO-M4's next-three-calls rule where they exist, else "not measured" with the reason), missed points by substitute tool and false fires at should-not points; `fathom labs fire --write` appends one row per (run, surface, arm `replay`) {id: LAB-11, symbol: FiringReading, package: internal/labs, needs: "LAB-9,LAB-12"}

### Results ledger (section 4.6)

- [ ] codex: `AppendLabResult` validates a `#Result` row and appends it to `docs/bench/labs-ledger.jsonl` with the ledger oracle's keys first (run_id, task_id, arm, trials, evaluation, surface_unused, instrument_defect); one row per (run_id, task_id, arm), a duplicate key refused, the funnel stages as fields of that row and `evaluation.criterion_id` naming the one criterion the run was preregistered to decide; it is the only writer, reached from `fathom labs fire --write` and `labbench`, and a test reads an appended row back through `internal/verify`'s ledger oracle by run, task and arm and asserts a second append under the key is refused {id: LAB-12, symbol: AppendLabResult, package: internal/labs, needs: LAB-1}

### Statistics (each claimed with the row that wires it)

- [ ] codex: `PassHatK` returns the unbiased pass^k from n trials with c passes, C(c, k) / C(n, k), beside `PassPowK`, upstream in the kit batch with BX-25, BX-37, BX-38, BX-39 and BX-64; wired by LAB-11 and LAB-20 {id: LAB-13, symbol: PassHatK, package: third_party/agentic-kit/foundations/quality/reliability-stats}
- [ ] codex: `BinaryRates` returns TPR, TNR, PPV and NPV with intervals from `ask.Wilson` for a confusion table, NaN on an empty margin; it stays in fathom because the kit cannot import `internal/ask`; wired by LAB-M1a through LAB-34 {id: LAB-14, symbol: BinaryRates, package: internal/labs}
- [ ] codex: `RoganGladen` corrects an observed positive rate for a judge's TPR and TNR, clamped to 0 and 1, with a seeded bootstrap interval, and returns NaN when TPR plus TNR is at most 1, upstream in the same kit batch; wired by LAB-34 {id: LAB-15, symbol: RoganGladen, package: third_party/agentic-kit/foundations/quality/reliability-stats}

### The loop's first links (section 4.7)

- [ ] codex: `ParseObservationLabLinks` reads the optional `- **Lab:** <lab>/<case>` bullet from `.fathom/observations/O<n>.md` and the legacy log; an absent bullet reads unlinked, never an error; `fathom observe --lab <lab>/<case>` writes it at filing and every existing reader of the entry still parses it; a link is a pointer, and no proof bullet is ever written by hand {id: LAB-16, symbol: ParseObservationLabLinks, package: internal/labs}
- [ ] ctl: `calibration/labs.d/feedback-seed.md` maps each of the 25 themes of `docs/bench/readings/feedback/adopter-triage-2026-10-20.md` to a lab case or a named disposition (fixed and proved by its fix commit's test, not fathom's, no case possible) {id: LAB-17, oracle: lexical, path: calibration/labs.d/feedback-seed.md, pattern: "^\| T[0-9]{2} \|", min: 25}
- [ ] codex: the first fixture cases from the open themes T01, T02, T03, T05, T06, T07, T08 and T15 on fauxtown or a public subject, each with `regression: true`, a deterministic grader and its observation id; the deadline themes (T01, T02) run on an injected clock or a slow fixture store, never a cold service; `LoadLabCases` loads them and a test asserts each names its theme and observation and grades as an ordinary Go test {id: LAB-18, symbol: LoadLabCases, package: internal/labs, reachable_from: TestFeedbackSeedCasesLoad, needs: "LAB-1,LAB-17"}

### The fork harness (section 4.5)

- [ ] codex: `RenderLabEvalSuite` writes a lab's cases into a rendered plugin copy's evals directory under `workspace/labs/runs/` with free graders only (`tool_used`, `tool_order`, `regex`), 3 runs, a turn cap and `context.history_file` for a prefix, beside a second copy with that one feature removed (its verb left out of `--allow-tools`, or its hook or skill deleted from the copy); an LLM grader is refused by name, and the run command it emits always carries `--no-publish` and `--ablation none`, with `--mocks off` against a scratch daemon for a helped suite {id: LAB-19, symbol: RenderLabEvalSuite, package: internal/labs, needs: LAB-1}
- [ ] codex: `PluginEvalDelta` reads two `claude plugin eval --json` runs, the plugin and the plugin minus one feature, into one ledger row per (run, case, arm): with-only indicators fill the fired stage, Δ over outcome graders fills helped, pass^k per case; a helped row without its minus-one-feature pair, or from a run on recorded mocks, is refused, and `--threshold` is never read; its fixture follows the documented shape and is re-pinned at the shakedown (LAB-37) {id: LAB-20, symbol: PluginEvalDelta, package: internal/labs, needs: "LAB-12,LAB-13"}

### Method records

- [ ] ctl: the lab prereg addendum `docs/bench/prereg/labs.md`: a firing criterion and a helped metric on every card, the minus-one-feature arm, cases sampled from every stratum including should-not points and points where the substitute succeeded, the seeded train, select and confirm split with every select and confirm read logged through BX-29, one rewrite round, the exact McNemar test on paired case outcomes with Holm across the round's surfaces and n from `PairedSampleSize`, the A/A instrument check, pass^k beside the mean, Δ computed by fathom, scoring by assignment with the zero-call line, pins from `RecordArmPins`, `--no-publish`, and the spend label; it extends bench-prep's template (BX-24) and repeats none of it {id: LAB-21, oracle: lexical, path: docs/bench/prereg/labs.md, pattern: "^## Firing and helping$"}
- [ ] ctl: an ADR records the program's decisions (cards and results as files, one ledger row per key, the product verb never spends, firing never counts as helping, helped is the plugin minus one feature, replay tunes no knob before K2 reads, a fix is proved by its own commit's test, the lab description override is refused by the managed daemon), its number taken free at merge with `fathom gate record-ids` {id: LAB-22, oracle: lexical, path: ADR.md, pattern: "^## ADR-[0-9]+: Feature labs"}

### P0 readings

- [ ] (op) ctl: the offline firing baseline on this machine's transcripts (this repository, others on Q3): per agent-surface verb eligible, fired, missed with the substitute tool and false fires; skills invoked against trigger-eligible requests; hook context delivered per hook event, with eligible from AD-20's log where it exists; the unclassified share named; aggregates only, population and window named {id: LAB-M0a, oracle: lexical, path: docs/bench/readings/labs/firing-baseline.md, pattern: "^LAB-M0a reading:", needs: "LAB-10,LAB-11"}
- [ ] (op) ctl: P0 exit: the plan verifies, `CheckLabClosure` reads every shipped surface mapped, LAB-M0a is committed and the ledger holds a row the ledger oracle reads {id: LAB-M0, oracle: lexical, path: docs/bench/readings/labs/p0-exit.md, pattern: "^LAB-M0 verdict: (PASS|FAIL)", needs: "LAB-3,LAB-4,LAB-5,LAB-12,LAB-M0a"}

## P1: the harness and shakedown, validation, fix proofs, text knobs and decision readings

### The shakedown and decision comparisons (owner rulings of 2026-10-22 and 2026-10-26)

- [ ] (op) api: the no-claim shakedown: 2 labs (find-symbol, skills), 10 cases, 3 runs, both arms (the plugin and the plugin minus the feature), `--no-publish`; prices a run per case shape, measures the discordant rate `PairedSampleSize` needs and re-pins `PluginEvalDelta`'s fixture against real output; recorded in bench-prep's shakedown reading (BX-70) beside its own line under one spend label {id: LAB-37, oracle: lexical, path: docs/bench/readings/bench-prep/shakedown.md, pattern: "^LAB-37 reading:", needs: "LAB-19,LAB-20,LAB-21,LAB-33"}
- [ ] (op) api: per-site decision comparisons: Jev and OpenAI Decisions against provider `none` on LAB-35's label sets through `labbench decide`, EXP-D2's question read through the ledger, no cross-vendor cascade, each reading's spend recorded and keys read from files {id: LAB-38, oracle: lexical, path: docs/bench/readings/labs/decision-sites.md, pattern: "^LAB-38 reading:", needs: "LAB-33,LAB-34,LAB-35"}

### Validate before steering (K1)

- [ ] (op) api: classifier labels: at least 40 points of each class (should-fire and should-not) per agent-surface verb and skill, by a seeded stratified sample from `workspace/labs/sessions/`, labelled by two independent labelers blind to the classifier and to each other; Cohen's κ per surface, every disagreement adjudicated by the owner, and a surface under 0.8 labelled by the owner or left to counts (Q4); a surface short of 40 in either class is named; labels stay in `workspace/labs/labels/` {id: LAB-23, oracle: lexical, path: docs/bench/readings/labs/classifier-labels.md, pattern: "^LAB-23 reading: [0-9]+ points labelled", needs: LAB-M0a}
- [ ] (op) ctl: K1, the classifier reading: per surface TPR and TNR with Wilson bounds through `BinaryRates` on at least 40 points per class; a surface whose lower bound is under 0.80 on either, or whose labelers' κ is under 0.8 without owner labels, reports counts only, never rates; the unclassified residue share is stated {id: LAB-M1a, oracle: lexical, path: docs/bench/readings/labs/classifier.md, pattern: "^LAB-M1a verdict: (PASS|FAIL)", needs: "LAB-23,LAB-14"}

### Fix proofs and the loop

- [ ] codex: `ProveObservationFix` behind `labbench regress`: for an observation whose `Status:` reads fixed, it takes the fix sha from that line (else the commit whose message names the O-id), lists the `Test` functions that commit added, runs them once in a scratch worktree at the parent (they must fail) and at the fix (they must pass), and appends the pair to `docs/bench/readings/labs/fix-proofs.jsonl`; a fixed entry with no recoverable sha or no added test is named unproven, never skipped; deterministic lab cases are ordinary Go tests under `go test -race ./...` and recorded-mock cases join bench-prep's hermetic set (BX-30), so no labs runner enters the gate {id: LAB-24, symbol: ProveObservationFix, package: internal/labs, needs: "LAB-16,LAB-18"}
- [ ] (op) codex: the seed cases' reading: each LAB-18 case red at HEAD (reproduced) or named not reproduced, and each fix since then proved through `ProveObservationFix` {id: LAB-M1c, oracle: lexical, path: docs/bench/readings/labs/feedback-cases.md, pattern: "^LAB-M1c reading:", needs: LAB-24}
- [ ] codex: `checkLoopLabs`, one `fathom flywheel` check: fixed observations with and without a fix proof, open themes with no case and red cases, each count from a file it names (P7) {id: LAB-25, symbol: checkLoopLabs, package: cmd/fathom, needs: "LAB-16,LAB-24"}
- [ ] codex: `AdopterFixedSince` behind `fathom instrument --adoption --adopter <journal dir>` reads an adopter journal read-only and lists the entries whose theme's case is green at a named release; it never writes the journal (P13, P15, Q6) {id: LAB-26, symbol: AdopterFixedSince, package: internal/labs, needs: "LAB-6,LAB-16,LAB-24"}
- [ ] codex: `hook-path-harness.py --lab <lab>` writes fired and delivered rows (answered, abandoned and latency per hook) to a file that `fathom labs ledger import` appends, beside its existing summary {id: LAB-27, oracle: lexical, path: docs/bench/readings/efficiency-wave/hook-path-harness.py, pattern: "^ +ap\.add_argument\(.--lab.", needs: LAB-12}

### Text knobs (section 4.5)

- [ ] codex: `loadLabToolDescriptions` behind `serve-http --lab-tool-descriptions <file>` replaces description text in `tools/list` on a scratch daemon with names and schemas unchanged (`TestMcpToolsDocMatchesRegistry` still holds); the managed daemon's identity and the user's own `FATHOM_HOME` refuse it with exit 2 and a named reason; there is no config key {id: LAB-28, symbol: loadLabToolDescriptions, package: cmd/fathom}
- [ ] codex: scenariobench `instructions: stanza-text` on a wired arm replaces the text between the `fathom:begin` and `fathom:end` markers `fathom init` wrote with `instructions_text` (empty text removes the block, the no-stanza arm) and keeps the hooks; `text` mode already appends to the subject's root CLAUDE file or AGENTS.md (ADR-059) and is unchanged; the arms are declared through `PreregArms` (BX-59) and pass `CheckArmParity` (BX-60) with the block as the only declared difference {id: LAB-29, symbol: replaceStanzaBlock, package: internal/scenariobench}
- [ ] codex: `ImportSkillEvalResults` turns the JSON of skill-creator's `run_eval.py` and `run_loop.py`, run as they are, into ledger rows; the eval sets, one per shipped or project skill (12) in skill-creator's format with positives and near-miss negatives sharing the skill's keywords on public fixtures, are sized by `PairedSampleSize` from the shakedown's discordant rate and split by `SplitLabCases`, and a test refuses a set short of its n in either class; neither script is rebuilt {id: LAB-30, symbol: ImportSkillEvalResults, package: internal/labs, needs: "LAB-12,LAB-32"}
- [ ] codex: `BuildForkPrefix` rebuilds a fork prefix on fauxtown or a public subject from a decision point sampled from every stratum (should-fire missed, should-fire hit, should-not, substitute succeeded), at least 20 cases per agent-surface verb or the n `PairedSampleSize` gives, each tagged `production-derived-redacted` with its derivation (the point's shape and stratum) {id: LAB-31, symbol: BuildForkPrefix, package: internal/labs, needs: "LAB-M0a,LAB-19"}
- [ ] codex: `SplitLabCases` assigns train, select or confirm by a seeded hash of the case id; every select and confirm read goes through bench-prep's held-out read log (`RecordHeldOutRead`, BX-29), which demotes a split on its second read {id: LAB-32, symbol: SplitLabCases, package: internal/labs, needs: LAB-1}

### The bench binary

- [ ] codex: `cmd/labbench` with `fork`, `aa`, `decide` and `regress`; every spending verb refuses to start without `-max-usd`, `-spend-label` and a committed prereg whose sha it stamps on each row, `fork` and `aa` refuse a plugin eval command without `--no-publish`, a binding cap writes a no-read, and `-dry-run` renders and validates suites (`claude plugin validate`) at $0 {id: LAB-33, symbol: runFork, package: cmd/labbench, needs: "LAB-19,LAB-20,LAB-21"}

### Decision readings (section 4.6)

- [ ] codex: `DecisionLabReading` turns labeled samples per site, template version, provider and resolved model into ledger rows through `decision.NewReading` (bands, Wilson, Brier, AUROC) with TPR and TNR (`BinaryRates`), coverage, abstention and a Rogan-Gladen estimate (`RoganGladen`), against the provider `none` row of the same run {id: LAB-34, symbol: DecisionLabReading, package: internal/labs, needs: "LAB-12,LAB-14,LAB-15"}
- [ ] (op) codex: the decision backfill: E1, E2 and E3 (`docs/bench/readings/decide/`), the route calibration of 2026-09-24 and the decision-port A/B of 2026-09-23 re-expressed as ledger rows from their committed readings, with no new calls {id: LAB-M1b, oracle: lexical, path: docs/bench/readings/labs/decision-backfill.md, pattern: "^LAB-M1b reading:", needs: LAB-34}
- [ ] codex: label sets for the sites with computable truth (EXP-2's four E1 templates, `certify_suggest`) under `calibration/labs.d/decide/` over public spans, each with its provider `none` rows {id: LAB-35, oracle: lexical, path: calibration/labs.d/decide/README.md, pattern: "^\| [a-z_]+ \| [0-9]+ labels \|", min: 2, needs: LAB-34}

### P1 exit and K5

- [ ] (op) ctl: P1 exit: LAB-M1a read per surface, LAB-M1c committed, LAB-M1b and LAB-38 committed, LAB-37 read, every fork and eval set passing `labbench fork -dry-run`, and the P2 envelope sized from LAB-37's prices {id: LAB-M1, oracle: lexical, path: docs/bench/readings/labs/p1-exit.md, pattern: "^LAB-M1 verdict: (PASS|FAIL)", needs: "LAB-M1a,LAB-M1b,LAB-M1c,LAB-24,LAB-33,LAB-37,LAB-38"}
- [ ] (op) ctl: K5, the loop's value, 60 days after LAB-M1: the share of observations fixed in the window with a fix proof from `ProveObservationFix`; PASS at half or more, else the loop shrinks to what was used {id: LAB-M1d, oracle: lexical, path: docs/bench/readings/labs/loop-value.md, pattern: "^LAB-M1d verdict: (PASS|FAIL)", needs: "LAB-M1,LAB-25"}

## P2: live forks and the bench's cells (preregistered runs inside the owner's sized envelope)

- [-] (op) api: the A/A instrument check per lab: the unchanged variant against itself at 3 seeds; an exact McNemar separation of a variant from itself means the harness is too noisy to read {id: LAB-D2, disposition: deferred, reason: "preregistered spend follows the harness and the shakedown, inside an envelope the owner sizes from LAB-37's prices", trigger: "LAB-37 read and the owner's sized envelope covers it", needs: LAB-37, oracle: lexical, path: docs/bench/readings/labs/aa-floor.md, pattern: "^LAB-D2 reading:"}
- [-] (op) api: K2, replay predicts live: at least 60 real prefixes from this repository's sessions forked locally through `context.history_file` at the commit nearest each session (the commit-point method, never committed); κ between the replay prediction and the live first call, PASS at 0.6 or more; below it replay ranks where to look and never selects or keeps a knob {id: LAB-M2a, disposition: deferred, reason: "preregistered spend follows the harness and the shakedown, inside an envelope the owner sizes from LAB-37's prices", trigger: "LAB-M1a read and the owner's sized envelope covers it", needs: "LAB-D2,LAB-M1a", oracle: lexical, path: docs/bench/readings/labs/replay-live.md, pattern: "^LAB-M2a verdict: (PASS|FAIL)"}
- [-] (op) api: K3 for skills: 12 skills through `run_loop.py` as is, one improvement iteration chosen on the select split, the confirm split untouched until LAB-D4; recall, false fires and pass^3, exact McNemar per skill on paired query outcomes, Holm across the round's 20 surfaces with LAB-M2c {id: LAB-M2b, disposition: deferred, reason: "preregistered spend follows the harness and the shakedown, inside an envelope the owner sizes from LAB-37's prices", trigger: "LAB-D2 read and the owner's sized envelope covers it", needs: "LAB-D2,LAB-30,LAB-32", oracle: lexical, path: docs/bench/readings/labs/skills.md, pattern: "^LAB-M2b verdict: (PASS|FAIL)"}
- [-] (op) api: K3 for the agent-surface verbs: the shipped descriptions against one rewrite each through LAB-28's override, chosen on the select split, exact McNemar per verb with Holm across the round's 20 surfaces, Δ fired and Δ helped (the plugin minus the verb) side by side {id: LAB-M2c, disposition: deferred, reason: "preregistered spend follows the harness and the shakedown, inside an envelope the owner sizes from LAB-37's prices", trigger: "LAB-D2 read and the owner's sized envelope covers it", needs: "LAB-D2,LAB-28,LAB-31,LAB-32", oracle: lexical, path: docs/bench/readings/labs/descriptions.md, pattern: "^LAB-M2c verdict: (PASS|FAIL)"}
- [-] (op) api: stanza variants through the `stanza-text` mode: the shipped stanza, an opening-moves-only stanza, a reordered stanza and the block removed, arms declared through BX-59 and parity-checked by BX-60; tool choice in the first three calls per task shape, the task outcome against the block removed, and the AG-OP1 clause 2 line from one fresh session {id: LAB-M2d, disposition: deferred, reason: "preregistered spend follows the harness and the shakedown, inside an envelope the owner sizes from LAB-37's prices", trigger: "LAB-D2 read and the owner's sized envelope covers it", needs: "LAB-D2,LAB-29", oracle: lexical, path: docs/bench/readings/labs/stanza.md, pattern: "^LAB-M2d verdict: (PASS|FAIL)"}
- [-] (op) api: K4 across every knob LAB-M2b to LAB-M2d kept or reverted: a knob that raised firing and left the helped metric harmed or inconclusive toward harm is reverted {id: LAB-M2e, disposition: deferred, reason: "reads the P2 readings, which wait on spend", trigger: "LAB-M2b, LAB-M2c and LAB-M2d read", needs: "LAB-M2b,LAB-M2c,LAB-M2d", oracle: lexical, path: docs/bench/readings/labs/firing-vs-helping.md, pattern: "^LAB-M2e verdict: (PASS|FAIL)"}
- [ ] (op) ctl: the firing funnel over the bench's A3 cell transcripts (live wired sessions with known tasks), read through `FiringReading` with each task's gold tool as eligible, at no extra spend; waits on the first bench cells (BX-70 or later) {id: LAB-39, oracle: lexical, path: docs/bench/readings/labs/bench-cells-firing.md, pattern: "^LAB-39 reading:", needs: LAB-11}
- [ ] (op) ctl: P2 exit: K2 and K3 read and at least one knob decision recorded either way, a null as prominent as a win {id: LAB-M2, oracle: lexical, path: docs/bench/readings/labs/p2-exit.md, pattern: "^LAB-M2 verdict: (PASS|FAIL)", needs: "LAB-M2a,LAB-M2b,LAB-M2c,LAB-M2e"}

## P3: tuning, release-candidate reruns and the deferred options

- [ ] ctl: every kept or reverted knob with its lab, ledger run_id and select-split Δ, written under `## Lab knobs` in bench-prep's knob map file (BX-28), one record with the bench's knobs {id: LAB-36, oracle: lexical, path: docs/bench/prereg/knobs.md, pattern: "^## Lab knobs$", needs: LAB-M2}
- [-] (op) api: confirm-split reruns of every kept knob at a named release candidate, each split read once through BX-29's log {id: LAB-D4, disposition: deferred, reason: "one clean read per split, at a release candidate; its spend is preregistered inside the owner's sized envelope", trigger: "a named release candidate after LAB-M2", needs: "LAB-M2,LAB-32"}
- [-] codex: a labs stage in bench-prep's scheduled lane script (BX-74): live reruns on a helper-machine lane, never a product job, filing `fathom observe` on a breached lab threshold; one lane, not a second {id: LAB-D5, disposition: deferred, reason: "recurring agent spend", trigger: "the owner approves a standing envelope after LAB-M2 and BX-74 lands"}
- [-] codex: the residue predicate site for unclassified decision points (design, Decision models) {id: LAB-D6, disposition: deferred, reason: "build only if the residue is large enough to matter", trigger: "LAB-M1a reads the unclassified residue above 20% of points"}
- [-] codex: a Codex arm for the fork and stanza labs through the `harness` field {id: LAB-D7, disposition: deferred, reason: "ADR-029 decision 6 keeps measurement Claude-only", trigger: "the owner answers Q7 yes and ADR-029 is amended"}
- [-] codex: OTLP export of ledger rows through the kit's `otel-genai` sink {id: LAB-D8, disposition: deferred, reason: "no consumer yet (P20)", trigger: "an operator asks to send lab rows to a collector"}
- [-] codex: a judged helped metric recorded through the kit's `eval-journal`, binary, validated by TPR and TNR on at least 100 labeled cases, never primary {id: LAB-D9, disposition: deferred, reason: "F12 is unpromoted", trigger: "F12's Promote when fires"}
- [-] codex: the stop hook's session-ledger fold and the transcript cost reader move onto `internal/transcript` (LAB-7 already shares the replay walk with the labs) {id: LAB-D10, disposition: deferred, reason: "both run in the Stop hook, a hot path, and work today", trigger: "a transcript format change breaks one of them"}
- [ ] (op) ctl: K6, the program's keep-or-kill, after the confirm reads (LAB-D4): at least one knob change holds on the confirm split, else the program stops at the P1 instruments (cards, ledger, fix proofs, firing reading) and P3 is refused; if K6 has not read by 2027-01-31 the program stops there the same way (Q8) {id: LAB-M3, oracle: lexical, path: docs/bench/readings/labs/program.md, pattern: "^LAB-M3 verdict: (KEEP|STOP)", needs: "LAB-M2,LAB-D4"}
- [-] An LLM judge on any primary firing or helped metric {id: LAB-R1, disposition: refused, reason: "every primary is graded deterministically (design section 3); a judge enters only through LAB-D9"}
- [-] Model classification of observations into themes {id: LAB-R2, disposition: refused, reason: "F49 says not to build it; a hand link costs one line per fix"}
- [-] A labs MCP tool {id: LAB-R3, disposition: refused, reason: "P4 and ADR-053: the labs are a CLI verb and a bench binary"}
- [-] A store table for lab results {id: LAB-R4, disposition: refused, reason: "ADR-049 decision 2 holds until RT-M1 reads PASS; the ledger file already has a stable key"}
- [-] A plugin-against-no-plugin helped arm in the labs {id: LAB-R5, disposition: refused, reason: "bench H4 (A3 against A1) owns it, and a whole-plugin baseline attributes nothing to one feature"}
- [-] A labs regression runner or time budget inside `task validate` {id: LAB-R6, disposition: refused, reason: "a deterministic case is a Go test under go test -race, and BX-30 holds the hermetic fixture set"}
