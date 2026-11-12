# The evidence base for the 2026-11-12 realignment: what changes, what is missing, what could stand alone

Status: Proposed 2026-11-12 (architect ruling, owner review pending). Supports [vision 46](../vision/46-proof-at-the-moment-of-change.md) and ADR-093 to ADR-104. Owner questions (OQ1 to OQ14) live in vision 46, each with its default if unanswered.

## Verdict

Keep almost everything. Cut what a fired kill already condemned, or what has no reader. Uplift the proof layer so it is present at the moment of change, and merge the duplicated surfaces. The backlog is blocked on measurements nobody runs, not on building. Five months of evidence come from one operator, so one outside team running the gate is worth more than the next ten programs. Only the language library is a real door to release now.

Everything not listed below is kept as it is. The owner's 2026-09-27 scorecard of 86 features is the baseline. This file records what six more weeks of readings change.

Codes: DS-D decision, DS-R risk (disposition). GP-n gap, GP-D decision, GP-R risk (gaps). SP-F finding, SP-S candidate, SP-D decision, SP-R risk (spin-offs). GP-n is a gap, not a goal id. Plan ids (ENG, CHG, RLN, WIR, CUT, LIB, FCT, UND, INB, TUN, HZ, REC, FLT, BEN) and program ids (PG-n) are in [docs/plans/realignment.md](../plans/realignment.md).

Verdicts: CUT (delete under a `gone:` row with a reopening condition), UPLIFT, MERGE, PARK (no staffing, with the clock or trigger named), KEEP (listed only where a reader might expect a change). Each CUT says whether a kill fired or it is the architect's preference.

## Part 1. Disposition

### 1.1 Cuts where a kill fired

| Item | Kill | Note |
|---|---|---|
| Readouts: lossy lean rules, read-outline as a lossy view, saved raw output, dedupe view | RO-M0 CUT (0.535% against 4%), RO-M19 STOP (0.40% against 1%) | Was "deferred". Withdraw. The lexer, shell unwrap, claim check, gate preflight and tests-to-run stay. |
| Carry-across at compaction, compaction advice | CA-M1 STOP (0.5 re-reads against 2) | |
| `route` and `rank_within_tier` decision sites | 45.7% right at 0.9 confidence, minus 0.19 paired mean | The port stays. |
| `internal/concern`, `internal/graph`, `internal/lookup`, `internal/journal`, `internal/resultcache`, `internal/scenariobenchwired`, the cached call graph with no production caller | no product importer | `concern` is documented as the live engine and is not. |
| `gen-status-cue` | zero readers since July | |
| `gen-claude-md` alias, three one-time migration scripts | deprecated, ran once | |
| `docs/design/anchored-memory.md` and other designs for work already cut | the feature was deleted under ADR-053 | Mark retired. Vision 20 still praises it. |
| The home bench as a source of claims | a dev set near its ceiling, declared so | Keep as a smoke run. |

### 1.2 Cuts that are preference, for the owner (OQ6)

| Item | Evidence | Why |
|---|---|---|
| Assess lanes, the hidden `assess` and `research` verbs | Frozen since 2026-09-27 "until an owner pull". Their readings never ran. | Code with no reader. Git keeps it. |
| Goal G2: the ambient A/B, and `arm`, `rate`, `retro`, `flywheel` | Zero admissible treatment units since August. Review 18 days overdue. | ADR-080's deterministic holdouts are the production successor. The adoption gauge stays. Law 12 stays. |

### 1.3 Parked

| Item | Clock or trigger |
|---|---|
| Index snapshots | Parked by its own kill (first reading 0.81 against 0.6). Reopens on a named team consumer. |
| Incident maps, telemetry links stage B | No clock in the record. Parked until a consumer is named. |
| Concern-fact waves 3 and 4, the platform-engineering rows | Their own kill on 2027-01-31. |
| Portable receipts waves 3 to 5 | Their own consumer gates. |
| Claude Code mods phases 2 to 4 | Their own kill on 2027-01-31. Phase 1 (status line, scout) is kept. |
| The old `ask` spine | Its own kill on 2027-01-31. The question router is a new verb (PG-7). |

### 1.4 Uplifts and merges

#### Engine

| Item | Verdict | Reason |
|---|---|---|
| Call graph on the hook path | UPLIFT | RT-195 to RT-199. Specified, measured, unbuilt. |
| Proof verbs beyond Go | UPLIFT | `wired` reads not applicable outside Go. Call extraction is already 0.92 to 0.97 precise for Python, TypeScript and JavaScript. The gap is typed edges and reach, not extraction. |
| Incumbent extractors beside dialects | CUT at parity | Two implementations per language recur in the friction log. |
| Per-wave extractor epoch | UPLIFT | Every bump invalidates every cache. Derive per dialect. |
| System graph, dependency plane | UPLIFT | 12 library rows, 41 open, no ingest. Waits on the freeze amendment (ADR-095). |
| `internal/ownership` | UPLIFT (wire) | An owners parser with no caller. |
| `internal/vendored`, 41 hand-rolled "is this a test" rules | MERGE | One rule, one source. |
| Language breadth | KEEP in data lanes, capped | Largest measured-level program. Milestone numbers need a rewrite, not more work. Fired refusal triggers (one grammar now passes its gate) are unprocessed. |

#### Proof

| Item | Verdict | Reason |
|---|---|---|
| Verify ledger | UPLIFT | 5,489 of 15,329 rows sit outside a checkout root, mostly gate clones. Record them under the checkout they verify. |
| Receipts (4 Go-only kinds) | UPLIFT | The answer contract for graph tools and named queries. |
| Certifications | UPLIFT | Strength floors for agent-written tests, through attested results. |
| Tests selected by reads | KEEP on probation | Its trigger reading read 4 of 6 against a bar of 5. The 14-day boundary reading is unread and needs one open row first. |
| Test-run hook | UPLIFT | 83% of failing runs were invisible behind wrappers. |
| Findings loop | KEEP | The review kit ingests scanners. Fathom ranks imported findings by reach. No SARIF door. |
| Spec interop | KEEP | The proof layer under spec tools. First outside wedge (part 3). |
| Infrastructure contract checks | UPLIFT | The hazard engine (ADR-099, HZ plan). |

#### Intent and continuity

| Item | Verdict | Reason |
|---|---|---|
| Record heads for every kind | UPLIFT | ADR-087's unbuilt rows, generalized (typed records, ADR-094). |
| `internal/intent`, `internal/adr`, `internal/recordids`, the plan parser | MERGE behind one port | The parsers stay as dialects. Direct calls go. |
| `intent audit`, `intent doctrine`, `intent index`, `config audit`, parts of `doctor` | MERGE | Three surfaces answer "is the configuration true". |
| Custodian concerns | UPLIFT | Per-repository today. The same shape, run across the fleet, is fleet conformance. |
| Continuity threads | KEEP | Thousands of recent entries. Only the bet of goal G3 closes. |
| Exploration maps | UPLIFT, re-aimed | Parent-to-child explains 2.8% to 15% of re-reads. Sibling re-reads are 69.9%. Aim at a shared read map. |
| Friction log | UPLIFT | Closing verdicts. 201 of 209 adopter entries read open, fixed ones included. |
| Feature map (no code) | MERGE into the `capability` kind | |

#### Delivery

| Item | Verdict | Reason |
|---|---|---|
| Post-edit and prompt prose rules | KEEP under ADR-080 | Run AD-M1 now, build the holdout log, read AD-M4. Two zero-effect readings reopen policy. |
| Session, sub-agent and directory primers | UPLIFT | 18.3% and 17.5% of hook bytes. Hold each to a byte budget with a reading. |
| Compiled layers | KEEP small | Cheap ($27.50 for 4,025 claims). They go stale at about 7 commits a day. Serve on pull. |
| Four offline transcript readers (`readout replay`, `labs corpus`, `research report`, `flywheel`) | MERGE | One corpus reader under the proof program (ADR-101). |

#### Surfaces

| Item | Verdict | Reason |
|---|---|---|
| `blast_radius` | UPLIFT | 57% of calls returned an error in 30 days, mostly named declines on a cold graph. 38 agent-chosen calls. |
| `obligations` on the edit hook | UPLIFT | The workhorse: 41,693 of 47,974 calls come from the edit hook. 8.2% were abandoned in 30 days, the largest count of any tool (ENG-11). |
| `query_symbols` | MERGE into `find_symbol` on the hook path | Off the default surface, hook-driven, 30% abandoned. |
| Eleven extended tools with no or almost no calls | CUT from the extended surface, keep as CLI | `outline`, `find_callers`, `repo_map` (zero calls), `references`, `pack_context`, `localize`, `test_impact`, `staleness_report` (under 35 calls, half or more abandoned), `findings_near`, `rename_service`, `cross_repo_callers` (never in the log). Each returns on off-surface demand. |
| `ingest`, `compile_layer`, `remove_service`, `rename_service` on the full endpoint | MOVE to CLI only | Writes and spend with no auth. |
| `metrics` subverbs that duplicate hidden verbs | MERGE | Two entry points per instrument. |
| Five refresh triggers, five export paths | MERGE | One verb with modes each. |
| `pkg/client` | UPLIFT or demote | About 100 methods, internal types in signatures, no API snapshot. A door or not a promise. |
| `pkg/platform` | CUT from the compatibility surface | It takes an internal type. No outside caller can use it. |
| Doctor (45 files in the command package) | UPLIFT | Into `internal/wiring`, one check registry. |
| `cmd/fathom` as one package | PARK | Verbs already register per file. A split unbinds 563 claimed rows. Future F98 keeps its trigger. Lanes own files. |
| Console pages with no views in the 30-day page log | CUT | The log exists for this. |
| Console refresh | UPLIFT | A live region through the kit. Until then notify and refresh. |

#### Fleet, operations, bench, documents

| Item | Verdict | Reason |
|---|---|---|
| Wire stamp per session | UPLIFT (build) | No reading can say fathom was wired without it. |
| Exact-tree gate receipts | UPLIFT (turn on) | Built, off by default. |
| Managed daemon | UPLIFT | macOS only, while cloud lanes run Linux. |
| Harness matrix | UPLIFT | One harness's demotion clock expired and the page still prints tier A. Render it. |
| Bench programs (strengths, prep, labs, context, review) | MERGE | One program, one driver, one ledger. About 120,000 lines in six commands. |
| Readings as prose | UPLIFT | Reading records with input fingerprints. |
| Vision series 01 to 45 | MERGE | One living vision record. The series is history. |
| `docs/design/product-vision.md` | UPLIFT | Dated 2026-07-07 and still named as the vision. |
| Hand-typed numbers in README, overview, early-tester notes | UPLIFT | Rendered or tested. README says 37 and 63 where the board says 45 and 74. |
| The legacy single-file friction log | CUT after its entries are split | Two logs. |

### 1.5 ADR-051's joins

ADR-051 names sixteen joins: thirteen accepted, two deferred (U14, U15) and one covered by doc drift (U16). They are kept, and the fleet picture reuses them instead of rebuilding.

| Join | Verdict |
|---|---|
| U1 cost per proven outcome | KEEP. The cost side of the fleet board. |
| U12 proof regressions with the breaking commit | KEEP. It already sits on the progress page. |
| U2 debt interest rate, U3 friction map, U13 hotspots | KEEP as pages. No new work until a page-view reading. |
| U4 rules that learn | PARK until the rule-outcome writer has 30 days of clean signal. It wrote nothing for nine days once. |
| U5 model and harness scorecard | UPLIFT. It routes lane classes to the cheapest model that proves its rows. |
| U6 task preflight | PARK behind its own A/B. |
| U7 change risk receipt | MERGE into the landing bundle. |
| U8 traceability matrix | MERGE into `why`. |
| U9 migration tracker | KEEP. |
| U10 service catalog that cannot rot | UPLIFT. Built with the system graph (PG-5, FCT-9). Fleet conformance (PG-7) reads it. |
| U11 architecture drift radar | KEEP. |
| U14 onboarding path, U15 capability reach | PARK. Deferred in the record. Futures F30 and F31 hold their triggers. |
| U16 doc truth | KEEP. Covered by doc drift. |

### 1.6 Futures and refusals, revisited and combined

The ask was to revisit rejected and deferred ideas and combine backlog items into better ones. Six combinations, then the single verdicts.

| Combination | Members | Verdict |
|---|---|---|
| Fleet landing | future F82 (overlap, the semantic half), F60 (landed test), F28 (answer events), ADR-081's receipts, join U7 | PROMOTE as the landing half of the flagship (CHG-5 to CHG-7). F82's replay already found 63 same-function conflicts. F83, F84 and F92 stay futures. |
| Proof at the edit | future F90 (duplicate at birth), the pre-write delta card, conservation deltas | PROMOTE as what-if cards, replay first (CHG-1). F89 stays a future. |
| Shared read map | futures F50, F54, F77, F36 and F75 (anchors beside shell reads, which are about 44% of Bash bytes) | PROMOTE as PG-11, replay first. F75's own trigger has not fired (13.5% against 30%), so it starts as replay only. |
| The judged family | 18 futures behind a kill screen that never ran | ONE record. Run the screen once, or close the family on 2027-01-31. |
| Typed intent | future F9 (machine-evaluated triggers), F41, F53, F86 | F9 is promoted in part: date and record conditions are computed (ADR-094). The rest wait behind the kill screen. |
| Production signal to code | futures F56, F57, F59, F66, F71 | PARK as one item with one trigger: a named observability consumer. Log templates are built and stay. |

| Refusal or deferral | Revisit verdict |
|---|---|
| CUE as the authored source | Typed heads now, syntax by trial (ADR-094). |
| A state machine for specs | Lifecycle conditions computed at HEAD. No authored state, no engine. |
| The table freeze | Amend (ADR-095). |
| Embeddings and meaning search | Stay refused. A third-party arm on the bench. |
| Generated wiki, project overview | Stay refused. Computed cards on pull, after the bench reads. |
| Anchored memory, lossy compression, compaction carry | Stay cut. |
| Paid bench runs | Unblocked by readiness harnesses, not by more approvals. |
| Remote and hosted channels | Wait on the threat-model answer. |
| Token dashboard | Stays refused. Cost per proven row is allowed and used. |
| Splitting the decision file | Wait for AP-M1. |
| Dispatch loop, kanban write-back | Stay refused. Reading lane state is admitted (ADR-098). |
| Grep blocking | Not fired. |
| 246 refused plan rows with no reopening condition | Each gets a `review_by` or an explicit "never" in the one-time pass (REC-5). |
| The 2026-09-27 single-user block (futures F3, F8, F15, F27, F30) | Wait on OQ1 and the threat-model answer. |

### 1.7 Goals past review

| Goal | Verdict | Reason |
|---|---|---|
| G2 A/B admissible | CLOSE (preference, OQ6) | A review verdict, not a fired kill. Holdouts succeed it. |
| G3 thread bet | CLOSE the bet, keep threads | 4 of 20 units in three months. |
| G4 ambient fathom | CLOSE | 3 rows open. Move them, archive the plan. |
| G7 evidence plane waves 1 and 2 | REMOVE | Met in August. |
| G9 bench honest | CLOSE with its FAIL recorded | Succeeded by the proof program. |
| G11 receipts | MERGE into the answer contract | |
| G12 verbs not lookups | CLOSE | Freshness killed, lookups level. |
| G13 console closes | MERGE into the fleet picture | Browser proof has never run. Fix that first. |
| G14 responsible tenant | KEEP, re-scoped | The reading stays. The freeze link changes. |

### 1.8 Programs: every plan with open rows

Open-row counts are raw checkbox counts on 2026-11-12. "Readings" means the open rows are mostly measurements for a reading lane.

| Plan | Open | Verdict |
|---|---|---|
| responsible-tenant | 25 | KEEP. PG-1. |
| project-understanding | 66 | SPLIT. Lifecycle rows to PG-2, trace and router to PG-7, bench rows to PG-7. Kill stands. |
| system-graph, dependency-licenses | 41, 12 | PG-5, after the freeze amendment. |
| iac-facts, lang-library | 22, 1 | KEEP. PG-9 and PG-3. |
| iac-provider-schemas | 13 | CUT. Superseded in part. Fold survivors into iac-facts. |
| bench-prep, feature-labs, strengths-bench, bench-pyramid, bench-public, bench-confidence | 70, 37, 7, 2, 2, 1 | MERGE into PG-4. |
| language-breadth and its gap plan | 104, 52 | KEEP in data lanes, capped. |
| game-development-support | 32 | KEEP. A named-consumer program on data lanes. |
| test-intelligence, test-run-hook, test-certifications | 15, 4, 3 | KEEP. PG-3 (CHG-3, CHG-4). |
| landing-evidence, boundary-enforcement, multi-machine | 1, 5, 1 | KEEP. PG-3 landing. Read the first real round trip. |
| advice-delivery, wiring-receipts, reach, cloud-sessions | 12, 8, 9, 3 | KEEP. PG-8. |
| exploration-maps | 16 | UPLIFT. PG-11. |
| observatory-console, progress-and-economics, work-intelligence, work-signals | 10, 5, 7, 8 | MERGE into PG-6. Stall signals are computed at render and need no freeze amendment. |
| repository-facts | 17 | KEEP what survived its two cuts. PG-5. |
| readouts | 16 | SPLIT. Seam readings keep. Lossy rows cut. |
| receipts, proof-credibility, refactor-conservation, served-not-kept | 11, 2, 3, 2 | KEEP. Readings. |
| spec-interop | 7 | KEEP. The outside wedge. |
| surface-reckoning | 10 | KEEP. Read the off-surface reading now. |
| knowledge-retrieval, stewardship, support-matrix, efficiency-wave, context-bill, change-and-compliance, governance-learning, fathom-dir, decision-port, judgment-foundations, agents | 4, 4, 9, 7, 2, 7, 8, 7, 6, 3, 3 | KEEP. Mostly readings for reading lanes. Wire the two support-matrix callers. |
| judged-predicates, judged-telemetry | 5, 5 | MERGE into the one judged-family record. |
| feature-map | 14 | MERGE into the `capability` kind. |
| agentic-decisions, claude-code-mods, portable-receipts, platform-engineering, well-architected-evidence, gke-module-host | 6, 15, 26, 23, 16, 1 | PARK on their own clocks. |
| incident-maps, index-snapshots, telemetry-links | 24, 25, 1 | PARK. |
| carry-across | 2 | CUT. |
| ambient-fathom, verbs, intent-v2, append-points, expert-suite-o, resume-2026-10 | 3, 3, 1, 1, 1, 8 | CLOSE after their last reading. The resume file is a note, not a plan. |

Thirty more plans have no open rows. Archive them with the existing verb.

### 1.9 Decisions

- DS-D1 Adopt the verdicts above. Each CUT is a plan row with a `gone:` contract and a reopening condition.
- DS-D2 Run purge wave 1 (section 1.1) before any parallel build wave.
- DS-D3 Cuts by preference wait for OQ6. They are not bundled with fired kills.
- DS-D4 A killed reading deletes its code in the same change. The lossy readout track and carry-across outlived their kills.
- DS-D5 Lines per wave is a tracked number, not a goal. A wave that deletes tests to shrink is a defect.
- DS-D6 Every PARK names a clock or a trigger. "Later" is not a state.

### 1.10 Risks

- DS-R1 A cut removes something a quiet adopter uses. Mitigation: off-surface demand is logged and a tool returns at 5% of sessions. That reading must finally be taken.
- DS-R2 Merging bench programs stalls measurement. Mitigation: the free layers keep running.
- DS-R3 The programs table is from raw checkbox counts. Mitigation: `fathom plans status` before any row moves.

### 1.11 What matters most

Cuts, all with a fired kill or no reader:

1. The lossy readout track, carry-across and the dedupe view.
2. Two failed decision sites.
3. Seven dead packages and caches.
4. Eleven extended tools with no or almost no calls.
5. The home bench as a source of claims.

Uplifts:

1. Graph answers on the hook path.
2. Typed heads and computed conditions for every record kind.
3. Proof at the moment of change: what-if verdicts and the landing bundle.
4. Proof verbs beyond Go.
5. One proof program with capability cards and proof liveness.

For the owner to rule: assess lanes and the ambient A/B (OQ6).

## Part 2. Gaps: what the ask did not include and the plan needs

Each gap names its evidence and a move.

### 2.1 People, readings and decisions

- **GP-1 No outside user.** Every reading and ledger comes from one operator's machines. The question of product against force multiplier has been open in the intent corpus since July and gates licence, publication and six futures. Move: answer it (OQ1), then name one outside team for a four-week trial of the gate (OQ2), using the spec-tool check as the vehicle (SP-S3).
- **GP-2 Readings are the bottleneck, rulings second.** About 310 of 903 open plan rows are measurements. About 67 of those name the owner, a ruling or spend. The rest can be run today and nobody is assigned. On the queue, 4 of 108 queue items wait on a ruling. Separately, nine goals are past review with no verdict and 22 questions are open. Move: reading lanes from the first wave, and three classes of ruling. Class A, recorded by the architect and reversible by the owner: a reading's verdict against its preregistered bar, a goal review, a cut whose kill fired. Class B, a weekly batch of typed questions, each with a recommendation and a default that applies after seven days. Class C, owner only: spend above a cap, licence, publication, anything irreversible.
- **GP-3 Scheduled readings pass unread.** The intent-practice readings are due 2026-11-17 and a lifecycle reading 2026-12-15. Nine goal reviews are already past. Nothing computes "due". Move: ReviewDue as a condition on every goal, reading and record (ADR-094), shown on the attention list.
- **GP-4 The asks are not recorded.** Decisions cite "the owner's note" in prose. Move: an `ask` record with the raw words, the uplifted ask and links to what it spawned. Start with this realignment's eleven.
- **GP-5 One maintainer.** A second contributor is arriving. Move: the onboarding ladder and a contributor guide.
- **GP-6 One word in the ask is ambiguous.** "State machines" in ask 3 may mean record lifecycles, or state machines in the code under analysis (states and transitions as facts). This plan reads it as the first (OQ14).

### 2.2 Engine and product

- **GP-7 Presence has no stated objective.** There are budgets and ladders and no promise. Move: the bar of PG-1, read weekly and shown on the fleet view (goal G53). No paid proof starts before RT-M15 passes.
- **GP-8 Proof stops at Go.** `wired` reads not applicable for other languages, and typed edges need an index that is rarely present. Call extraction itself is already 0.92 to 0.97 precise for Python, TypeScript and JavaScript. Move: proof verbs beyond Go (PG-5, FCT-1, which waits on nothing).
- **GP-9 Linux is a second-class host.** The managed daemon is macOS only. Cloud lanes run Linux, and CI is the only Linux witness. Move: a service unit, a Linux reading, required CI (PG-8).
- **GP-10 Security of a tool that injects into every session.** Release archives carry a build-provenance attestation, and no stranger has read one yet. The full endpoint exposes writes and model spend with no auth. The threat-model question is still open. Move: WIR-6 and WIR-7.
- **GP-11 Silent success is still the commonest defect shape.** A failed read counted as empty, a vacuous pass, a ledger that wrote nothing for nine days. Each was fixed where found. Move: the answer contract as a type, so an empty result cannot be built without a reason.
- **GP-12 The scale envelope is unstated.** Cold ingest of fathom itself takes several minutes. Nobody has said how large a repository fathom promises to serve inside its budget. Move: state it from a reading and refuse by name above it (OQ13).

### 2.3 Process and economics

- **GP-13 Gate time is unmeasured.** It is the landing bottleneck by every account, and the repository holds no number for it. Move: measure wall time and lease wait from gate logs in wave 0.
- **GP-14 A complexity budget.** About 496,000 non-test lines and as many test lines in five months. Move: lines per wave as a tracked number, and each build program names what it deletes.
- **GP-15 The cost of the build fleet does not steer it.** Fathom already computes cost per proven row per harness and model (ADR-051's scorecard). Move: use it to route lane classes to the cheapest model that proves its rows.
- **GP-16 Some readings cannot be repeated by others.** They rest on private transcripts. Move: a public corpus behind every public claim, and a private label on the rest.
- **GP-17 Numbers in prose drift.** Tool counts, language counts and tier labels disagree across README, guides and pages. Move: render or test every number.
- **GP-18 Releases stalled.** The last tag is 2026-10-28, with 217 commits since and the language library untagged. Move: tag after each green wave, as the autonomy rule already says.
- **GP-19 The friction loop does not close.** 201 of 209 adopter reports read open, fixed ones included. Move: closing verdicts, an age signal, a reply path.
- **GP-20 Stale guidance in standing instructions.** One instruction sends agents to a docs reader that was removed. A hook header advertises an opt-out that a guard refuses. Move: fix both, and let the custodian's pointer check cover them.

### 2.4 Strategy

- **GP-21 Family boundaries.** The policy tool that would consume the library is outside this repository by the owner's choice. Move: record the split once. Fathom computes facts and hazards. Baselines and judgment live in the kits. The check kernel waits on the owner's ask (OQ3).
- **GP-22 The public statement.** It leads with code intelligence, and one published page still states savings that were later withdrawn. Move: one sentence (vision 46), and remove the page.
- **GP-23 What a harness absorbs.** Persistence, task lists and schema deferral became harness features this year. Move: each capability card names what a harness would have to ship to make it redundant.
- **GP-24 Retention and privacy.** Call logs, hook rows and the lane file grow, and paths move between the operator's machines. Move: one retention table and a private label on anything with paths.
- **GP-25 This plan needs its own kill conditions.** Move: three, stated now. If RT-M15 still fails after ENG-1 to ENG-6, the hook path gets only what answers in time and what-if verdicts are not built. If the syntax trial ties, records keep labelled heads and the question closes. If false-done reads null on the PLAN family with adequate power, the proof position itself is re-examined before any other program continues.

### 2.5 Decisions

- GP-D1 GP-1, GP-2 and GP-7 enter the realignment as goals with instruments: G51 one outside team runs the gate, G52 readings are staffed, G53 engine presence bar.
- GP-D2 Adopt the three ruling classes as a record (ADR-103). It is the revisit of the autonomy rules, whose trigger has fired.
- GP-D3 Add the `ask` kind and file this realignment's asks first.
- GP-D4 GP-8, GP-9 and GP-10 are rows in existing programs, not new programs.
- GP-D5 GP-13, GP-14 and GP-17 become tested numbers once measured.
- GP-D6 GP-25's three conditions go into the realignment record as its revisit text.

### 2.6 Risks

- GP-R1 Delegated rulings drift from what the owner wants. Mitigation: each is a record the owner can reverse, and the weekly batch lists them.
- GP-R2 An outside trial exposes rough edges. That is its purpose. One team, one capability, four weeks.
- GP-R3 An objective the engine misses for weeks is discouraging. Mitigation: it is program one and the board shows the trend.

### 2.7 The three that decide the rest

1. **GP-2, readings.** The backlog is not blocked on building. It is blocked on measurements nobody runs and, less often, on a ruling. Reading lanes and ruling classes fix both. More build lanes alone would only lengthen the queue.
2. **GP-7, presence.** The proof layer is only as good as its presence at the moment of change. State the bar, read it weekly, and start no paid proof before its first step passes.
3. **GP-1, a user outside the fleet.** Five months of evidence come from one operator. One outside team running the gate for four weeks will say more about what is world class than the next ten programs.

## Part 3. Spin-offs: which parts of fathom have value on their own

Sources in the repository: `docs/handoff/repo-strategy.md`, [the 2026-11-04 standalone-capabilities assessment](2026-11-04-standalone-capabilities.md), ADR-092, ADR-052, [docs/plans/lang-library.md](../plans/lang-library.md), [docs/plans/iac-facts.md](../plans/iac-facts.md).

### 3.1 The tests, and one addition

Three recorded rules decide this and are kept: the release-day test (could it ship on a different day, and would anyone notice), principle P20 (build for a named consumer, generalize from two) and principle P21 (split on a boundary that already exists and is enforced).

The addition: a spin-off has five forms, and a new repository is the last and most expensive.

| Form | Promise | Example |
|---|---|---|
| Door | a public package with a closure test and an API snapshot, in the same module | `pkg/lang` |
| Binary | a second command in the release archive | `fathom-lang` |
| Data | a versioned, digested artifact | provider contract catalogs |
| Spec | a published format with a verifier | receipts, the landing bundle |
| Packaging | the same engine delivered where people already work | a pull-request check for spec-tool tasks |

### 3.2 Findings

- SP-F1 One door is real. `pkg/lang` and its subpackages have their own types, a closure test, an API snapshot, no-dial and no-write tests, an outside-module build and fuzzing.
- SP-F2 It is unreleased. The last tag is 2026-10-28. The door landed 2026-11-04, and 173 commits have followed it. The one open library reading cannot run without a tag.
- SP-F3 It has no consumer yet. The named consumer is the owner's policy tool, outside this repository and unbuilt. ADR-092 does not count the product as the outside consumer.
- SP-F4 Its closure budget has no margin: 80 packages against a bar of 80, and an outside consumer links 81.
- SP-F5 Twelve internal packages are leaves with no fathom imports, about 15,100 lines. Each is one door away when a consumer appears.
- SP-F6 The evidence packages (receipts, their verifier, certifications, conservation) all reach the store, with closures of 41 to 61 packages. None is a door without a port.
- SP-F7 `pkg/client` and `pkg/platform` are on the compatibility surface and are not doors.
- SP-F8 The repository is source-available. A standalone library competes with permissively licensed tools. ADR-052's triggers have not fired.
- SP-F9 The market gaps are specific: no spec tool derives status from evidence, no product pairs test selection with coding agents, nobody publishes provider contracts with replacement facts as data, and no static tool reports Terraform change hazards without a plan.
- SP-F10 Every release archive since v0.13.0 embeds grammar blobs (55 since v0.23.0) and ships only the product licence and the readme. The licence notices for what the binaries embed are owed, and the product's own archive owes them too.
- SP-F11 A CloudFormation replacement finding is already served by `review_diff`, and its floor against `cdk diff` (HZ-1) has never been read. Whether the fleet holds infrastructure code is unverified, and a census through the fleet index settles it (UND-M3, `docs/plans/realignment.md`).

### 3.3 Candidates

| Id | Candidate | Form | Value alone | Boundary today | Verdict |
|---|---|---|---|---|---|
| SP-S1 | Language and tool understanding (`pkg/lang`, `iac`, `langtest`, `fathom-lang`), now with the change hazards H1, H2, H4 and H5 in the binary (library rows LIB-7, HZ-10) | door and binary | high: plan-free reads that report what they cannot know, graded against tool oracles | enforced by test | NOW: release it, after the licence notice (LIB-12) |
| SP-S2 | Provider contract catalogs with replacement facts and upgrade diffs | data | high and unique | builder exists, publishing does not | NEXT: rides IF-6 and IF-11. One upstream schema has no redistribution licence, so that catalog is built by the operator. |
| SP-S3 | Evidence-gated done for spec tools | packaging | the strongest outside wedge: proves tasks in four foreign formats | the verb exists, a check form does not | NEXT: a pull-request check that runs `verify` on touched rows |
| SP-S4 | The check kernel (`pkg/lang/check`) | door | a small evaluator with three-valued verdicts | unbuilt | ON A YES TO OQ3: the kernel is not built now, and the default on 2027-01-31 is no |
| SP-S5 | The landing bundle and receipts | spec and verifier | a stranger checks a landing with git and a toolchain | the verifier's closure is 49 packages | LATER: publish the format when a second consumer appears |
| SP-S6 | Typed records and lifecycle conditions | door, later a kit piece | intent as data with derived conditions, for any agent-built project | new, born as a leaf | DESIGN FOR IT: no store import from the first commit |
| SP-S7 | Tests selected by what they read | kit piece | unsold for agents | leaf | AFTER its reading passes |
| SP-S8 | Harness port, budgets and shedding, shell command parsing and hook guards | kit pieces | medium | leaves | ON TRIGGER: a second consumer each |
| SP-S9 | Grading harness for parsers | method and tool | "the grading method is what sets fathom apart" | closure of 42 | LATER: `langtest` is the first step |
| SP-S10 | Bench driver (cell keys, borrowing, void rules) | tool | medium | coupled | AFTER the bench merge |
| SP-S11 | The verify gate apart from the index, readouts, findings storage, the review engine | none | fail the release-day test | coupled at both ends | NEVER |
| SP-S12 | The hazard benchmark's labels and method (BEN-10): public infrastructure history labelled by a plan, with the bias of synthesized state stated | data and method | high: the one infrastructure row with a consumer, an outside oracle and no competitor | the labels are built from public history | NEXT: after the first hazard reading (HZ-M1), published with the scanner arms |
| SP-S13 | A pre-commit hook and a pull-request check for infrastructure changes | packaging | medium | the library binary | REGISTER: an outside repository named in writing, with HZ-9 at 0.9. The pull-request check also needs signed catalogs (IF-6). |
| SP-S14 | An evidence-class annotation for other scanners' findings | spec | medium | none | REGISTER: the review kit asks in writing, and a reading shows a precision gain against plan truth |
| SP-S15 | A class conformance suite | spec | medium | none | REGISTER: the library is released and one outside reader asks |
| SP-S16 | The three outside wedges the uses census ranks highest: the contract check outside the product, the module and chart interface diff, the catalog diff | packaging and data | each fails no clause but the consumer clause | the library binary and published catalogs | REGISTER: each waits on a named consumer |

### 3.4 Decisions

- SP-D1 Tag a release that contains the library, re-read its closure budget and run the open reading. An unreleased door is not a spin-off. The licence notice (LIB-12) comes first: no further archive ships without it.
- SP-D2 Moved to the register. The product reads through the door only when an outside importer exists and the binary and the outside-module reading have missed an API gap. Hazards are called directly, and the door has other readers that exercise it: its binary, the calibration tools and the outside-module reading. It would not satisfy ADR-092's outside-consumer condition either way.
- SP-D3 The catalog is the first data product. Publish it with an attestation when its two rows close.
- SP-D4 The first outside wedge is packaging: evidence-gated done as a check for teams that already write tasks in a spec tool. It turns the one proven capability into a short trial, and it is the vehicle for the outside team (OQ2).
- SP-D5 Build the records package as a leaf from the first commit.
- SP-D6 Fix or demote the two false doors. `pkg/client` gets its own types and a snapshot, or leaves the compatibility surface. `pkg/platform` leaves it.
- SP-D7 No new repository in this effort. Every trigger for one is recorded and none has fired.
- SP-D8 Spin-off steps, in order: the licence notice, then the closure re-read, the tag and the outside build reading. Then the hazard entry in the binary and the public hazard benchmark (BEN-10), the reading benchmark under its three rules (BEN-11), the licence audit (IF-11) and published catalogs (IF-6). Register entries open on their triggers, which OQ15 helps and does not replace, and the kernel and its draft acceptance test open on a yes to OQ3.

### 3.5 Decision models

None applies to the spin-offs. Each candidate is computed. The check kernel, if built, takes a caller's check and a caller's facts, and no model resolves an unknown.

### 3.6 Risks

- SP-R1 Public surface grows on a door with no outside user. The recorded sunset holds it: no outside importer by 2027-05-04 stops new surface.
- SP-R2 A pull-request check needs a fast cold start. Mitigation: prove only the rows a change touches, over an index of the changed closure.
- SP-R3 Licence keeps strangers away from the library and the catalog (OQ4).
- SP-R4 Spin-offs pull attention from engine presence. Mitigation: only SP-D1 is work in H0.

### 3.7 Ranked

1. **SP-S1, the language library.** The only real door. Release it, after the licence notice.
2. **SP-S3, evidence-gated done as a check under spec tools.** The proven capability, packaged where a large audience already works. The best first contact with outside users.
3. **SP-S2, provider contract catalogs as data.** The earliest artifact with value to tools that never run fathom.
   **SP-S12, the hazard benchmark's labels and method,** ranks beside it: it is the library's first public artifact with an audience.
4. **SP-S6, typed records.** Not a spin-off yet. Build it so it can become one.
5. Everything else waits for its named consumer.

## Decision models

None applies. This document records evidence and verdicts. It adds no site that asks a typed question. The decision models of each resulting change are stated in ADR-093 to ADR-104 and in [vision 46](../vision/46-proof-at-the-moment-of-change.md).
