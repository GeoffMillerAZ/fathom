# Parallel lanes: splitting the build across harnesses and contributors without merge drama

Status: Proposed 2026-11-12 (architect ruling, owner review pending). Decisions: PL-D1 to PL-D15 below, recorded as
ADRs in the realignment set (ADR-093 to ADR-104). Owner questions live in `docs/vision/46-proof-at-the-moment-of-change.md`
(OQ1..OQ14). Plans: no plan file here. The rows live in `docs/plans/realignment.md` and the other plans (HZ, REC, FLT, BEN).

Revised after a feasibility critique that read the code. Codes carry the prefix PL: PL-F finding, PL-D decision,
PL-R risk. Lane classes are L-D, L-R, L-P, L-F, L-E, L-S.

Harness and machine assignment is operator configuration and lives outside the repository (ADR-011). This design
names lane classes, owned files, proofs and caps. It never names which harness, host or account runs a lane.

Reading order: the wave plan and what stays on the main thread are the last sections.

## 1. Findings

- PL-F1 Collisions were on shared text. In the 30 days to 2026-10-19, 263 of 523 merges conflicted, 247 on
  append points: the changelog (118), a generated board (56), one large plan (64). Fixes have landed since
  (change-note fragments, regenerate-on-merge, sibling plan files, per-wave epoch files). The reading after
  the fixes, AP-M1, is unread.
- PL-F2 Sixty-three conflicts were inside one function, mostly in the daemon's serve file and the console
  registry. Nothing forecasts those.
- PL-F3 The gate is serial per machine behind a lease: 19 steps, then the tests. Its wall time today is not
  recorded anywhere in the repository. One assessment outside it put a gated push near 35 minutes. Measure
  before changing it.
- PL-F4 Linux CI is the only cross-platform witness. It failed 39 of its last 100 runs on main as of
  2026-10-22, and a local gate has passed over a red CI more than once.
- PL-F5 Ids are checked after the fact and reserved by hand in briefs. Two workers once minted the same
  ids. Observation ids are not checked.
- PL-F6 The command package is one 144,000-line `package main`, but verbs already register in their own
  files. A lane can own files there today. A package split would unbind 563 claimed plan rows and cannot
  keep tests unchanged, so it is not a precondition for parallel work.
- PL-F7 Shared files remain: the hook files (ten planned rows touch them), the config struct, the tool
  registry, the schema directory, the task and hook configuration.
- PL-F8 The queue is issue labels and CLAIM comments: 108 open queue items, 70 of them language items, 15
  blocked, 4 waiting on a ruling. The queue's own helper task does not exist yet.
- PL-F9 About 310 open plan rows are measurements. Most can be run by a lane today. Nobody is assigned.
- PL-F10 Test selection by reads is not proven. Its trigger reading read 4 of 6 against a bar of 5, its
  boundary reading is unread, and it needs a 290 MB observation that matches the lane head. An
  affected-tests gate would miss the data and subprocess reads that data lanes produce.
- PL-F11 Hosts starve. Four waves at once produced load of 20 to 60 and file-access faults that killed
  gates. Standing caps: four heavy processes per machine, one engine lane at a time.
- PL-F12 What already works: a clone and scratch per worker, the fixes in PL-F1, exact-tree gate receipts
  (built, off), provenance notes, the stacked-work check, integrator-minted ids.
- PL-F13 There is no contributor guide. A newcomer's reading path is spread over ten files.

## 2. Principles

- PL-D1 **Contract first.** The main thread lands interfaces, schemas, fixtures and failing proof tests
  before a lane starts. A lane begins red on its own proof and ends green. No lane designs.
- PL-D2 **A lane owns files.** Its record lists them. Where a file must be shared, one lane owns it and
  the rows that need it run through that lane in sequence.
- PL-D3 **Ids are allocated.** A lane record carries a reserved range per id family. `fathom rec new`
  hands out the next one. A law checks that each id is defined in exactly one file.
- PL-D4 **A lane is a record.** Goal, owned files, read-only files, contract files, proof command, rows,
  id range, base commit, budget, harness class. It is CUE data with a standalone schema, so it exists
  before the records program does. The lane's id is set in the environment of its clone, where the
  provenance verb and the hooks read it. The forge's issue stays the place people claim work. Fathom
  writes no issue.
- PL-D5 **Path ownership is a gate.** A lane's diff must sit inside its owned files plus the per-lane
  files every lane may add (its change note, its sibling plan file, its epoch file). A touch outside names
  the owning lane.
- PL-D6 **The lane gate is not lighter yet.** It runs affected tests, every test package the lane owns,
  the boundary readers, vet, record vet, the path check and the lane's proof command. A lighter gate waits
  on the selection reading passing and on ADR-090's own trigger, a CI-gated merge queue.
- PL-D7 **A landing carries its proof.** The bundle of plan row CHG-5 in `docs/plans/realignment.md`. The
  integrator verifies a bundle instead of re-running an identical tree. CI samples full runs. Red CI on
  main stops merges.
- PL-D8 **Engine changes are serial.** Fusion, extraction, the store, the core types, ingest. One at a
  time, with a regrade.
- PL-D9 **Forecast before merge.** The integrator orders merges by lane overlap: shared files, shared
  functions, intersecting reach. The second lane to land resolves on its own branch.
- PL-D10 **Workers return, they do not widen.** A lane that finds its brief false stops and reports.
- PL-D11 **Ground every brief.** One cheap check before dispatch: the named symbols and tests exist, and
  the rows it depends on are where the brief says. Four of nine briefs from backlog prose once had a false
  premise.
- PL-D12 **Readings are lanes.** A reading lane runs one preregistered measurement and files it with its
  verdict. It touches a readings directory and nothing else.

## 3. Lane classes

| Class | What | Touches | Proof | Runs | Risk |
|---|---|---|---|---|---|
| L-D Data | a dialect, hazard fixtures, a bench subject, a record backfill | data directories, its own plan file | a harness reading or fixtures | many at once | lowest |
| L-R Reading | one preregistered measurement | one reading file | the reading and its verdict line | many at once | lowest |
| L-P Leaf package | a new package behind a frozen interface, or a new schema package | one directory | closure and contract tests | many at once | low |
| L-F Files in a shared package | named files in the command package or another shared package | only those files | the lane's proof tests | many at once, never two on one file | medium |
| L-E Engine | fusion, extraction, store, core types, ingest | shared engine files | readings regraded | one at a time | high |
| L-S Surface | existing schemas, the tool registry, hook contracts, decisions, the config struct, release | shared contracts | the contract tests | main thread only | highest |

Config keys are added by the main thread in one batch per wave. A lane asks for its keys in its record.

## 4. Lanes for the next effort

Row ids are plan rows: LIB, RL and LP in `docs/plans/library-roadmap.md` (the separate library roadmap), IF in `docs/plans/iac-facts.md`, HZ in `docs/plans/change-hazards.md`, REC in `docs/plans/typed-records.md`, FLT in
`docs/plans/fleet-picture.md`, BEN in `docs/plans/proof-program.md`, and ENG, CHG, RLN, WIR, CUT, LIB, FCT, UND, INB, TUN
in `docs/plans/realignment.md`.

| Lane | Class | Rows | Owns | Handed | Proof |
|---|---|---|---|---|---|
| hazard-oracle | L-R | HZ-1 with its small harness (it runs the live check and `cdk diff` on template pairs), then HZ-0, IF-M1 | reading files, a bench harness | the review's floor for HZ-1 | the readings. HZ-1 under its floor stops the hazard lanes and drops the live CloudFormation flag to advice wording. Wave 1, HZ-1 first. |
| lib-facts (was hazard-facts) | L-F: named files in a package the hazard lanes share | HZ-3, HZ-4, HZ-6 (HZ-5 on entry, HZ-7 is a register entry) | the infrastructure reader's fact files | fact shapes, API snapshot | reader tests |
| hazard-free | L-F | HZ-10, LIB-7 | hazard files and the library binary's subcommand | the JSON shape of the hazard entry, a wave 0c contract | JSON goldens, then the first replay reading. Wave 2, after lib-facts. |
| hazard-checks | L-F | HZ-9 with LIB-9, HZ-11, IF-15 | the hazard files | the fact shapes, replay fixtures | replay readings. HZ-9 starts after IF-13 and hazard-free. Wave 2. |
| hazard-surface | L-F | HZ-2 first. IF-14 and IF-16 after HZ-9 reads 0.9. | the product's review files | the fact shapes | the review dimension and flag tests. Wave 2 for HZ-2, 3 for the rest. |
| hazard-replay | L-R | the HZ-M1 harness with the bench plan reader, then its readings for HZ-10 and HZ-9, IF-M2, IF-M3, IF-CM3 | reading files, a bench harness | the plan reader | each reading on its own needs. Wave 2. |
| decidability | L-R | HZ-17 | one reading file | the bench plan reader | the reading. Wave 2. |
| record-heads | L-P | REC-2, REC-3 | schema packages for each kind, the head parsers | kind list, sample records | unparsed count |
| record-conditions | L-P | REC-4, REC-5 | the lifecycle package | condition names, fixtures | condition fixtures |
| record-port | L-P then L-E | REC-6 | `internal/records`. Its ingest hookup takes the engine slot. | loader interface | closure test |
| record-verbs | L-F | REC-7, REC-9 | the `rec` verb files | verb JSON goldens | vet and query tests |
| plan-keys | L-S | REC-8 | the plan schema, the console's title reader, plan files by backfill tool. One main-thread batch. | key list | zero fields left in titles |
| landing-bundle | L-F | CHG-5, CHG-6 | gate receipt files | bundle schema | bundle verifies on a clean checkout |
| whatif-replay | L-R then L-F | CHG-1, CHG-2, CHG-9, and LIB-13 as an internal package | replay cards, then the pre-edit verdict files and the delta package | card schema | replay readings |
| hooks | L-F, sequential | ENG-8, ENG-9, WIR-1, CHG-3, BEN-4 in waves 1 and 2. Then CHG-8 (the E20 half, after BEN-4 and ahead of FLT-3), FLT-3, CHG-2, WIR-2, INB-1, INB-2. | the hook files. Every row that edits one runs through this lane, in this order. | stamp, attribution and holdout-log schemas | coverage readings |
| engine | L-E, sequential | ENG-3, ENG-4, ENG-6, ENG-11 after ENG-1 and ENG-2 | the graph store, ingest publish, admission | cost-row lint, re-warm counters | ENG-5 as a reading lane after ENG-4 |
| ledger | L-F | ENG-10 | the verify ledger writer | ledger schema | the ledger reading |
| test-strength | L-F | CHG-4 with the mutation join and the JUnit reader | certificate and ingest files | strength ladder fixtures | certificate fixtures |
| friction | L-D | REC-11 | observation files, by tool | verdict vocabulary | open share by age |
| fleet-ground | L-F | FLT-1, ENG-7 | new console projection files | panel-equals-verb test | parity |
| overlap | L-P | CHG-7 | overlap package | replay corpus | forecast reading |
| bench-merge | L-P | BEN-1 | the bench driver | cell-key and ledger schema | key uniqueness |
| readiness | L-P | BEN-2 | scripted harnesses | harness interface | all six pass |
| cards | L-D | BEN-3, REC-10 | capability and standard files | kind schemas | closure check |
| catalogs | L-D | IF-13, IF-11, IF-12. LIB-9 is built with HZ-9 in hazard-checks (wave 2). IF-6 later, on its entry (the library tag, IF-11, signed releases, OQ4). | the catalog builder and pins | catalog format | byte-identical rebuild. Wave 1. |
| closure-reading | main thread | LIB-11 | one reading file | the closure bar | Wave 0b, before the tag and before HZ-4 or HZ-6 add to the door. |
| notice | main thread, with the release | LIB-12 | release files | the grammar lock | the release check fails without the notice. Wave 0b, before the tag. |
| pairing-reading | L-R | FCT-10 (TQ-M3) | one reading file | the committed readings | the reading. Wave 1. |
| latency-reading | L-R | CHG-M2, the cold-latency half of CHG-8 | one reading file | the hook budget | the reading. Wave 1. |
| census | L-R | UND-M3 (the file-presence slice of UND-1), CHG-10's ranking of served prose rules | reading files | the fleet index | the readings. Wave 1. |
| bench-subjects | L-D | BEN-5, with false-by-construction rows for configuration dialects | subject and task-bank files | admission rules | admission reading. Wave 2. |
| rules | L-F | CHG-10's port of the tenancy lint (RL-9), then CHG-11 and CHG-12 | the lint's rule data, the replay package | rule kind schema | the rule fires on the causing commit, every derived mutant killed. Waves 2 and 3. |
| lifetime-sites | L-P | RL-2 | the Go reader's function walk | the site fact shapes | RL-M0 against the tenancy lint. Wave 3, with the rules lane's port. |
| benchmark | L-R | BEN-10, BEN-11 with LIB-14 | reading files, fixtures | preregistration | the readings. Wave 3, behind the paid gate for the model arms. |
| purge-N | L-F | CUT-1 items | the files being removed | `gone:` contract | absence proven |
| reading-N | L-R | open measurement rows | one reading file each | the preregistered bar | verdict line |
| dialects | L-D | language queue | dialect data | the existing recipe | harness readings |

Main thread, before fan-out: ENG-1 and ENG-2 (engine slot), the realignment records, REC-1, the path
check, the licence notice and the closure re-read, contracts and failing proofs for wave 1.

Library lanes are cheap parallel capacity. Fact rows, hazard computations and their readings touch no daemon file and no hook
file, and they still start in wave 1, after the lane kind and the wave's contracts exist. No library lane touches a daemon
file or a hook file. The first wave grows by three reading lanes: pairing, latency and census. Easy library lanes must not pull
attention from engine presence, so ENG-1 and ENG-2 stay on the main thread before fan-out.

Caps that stay: one engine lane at a time, at most five language lanes claimed, at most four heavy test or
oracle processes per machine.

## 5. Onboarding a second contributor

A ladder. Each step has a proof.

| Step | Task | Proof | Time |
|---|---|---|---|
| 0 | Read five things: the root instructions, the intent router, the plan row traps, the wave guide, the queue rules | none | an hour |
| 1 | The stranger path: install, wire a sample repository, run `verify`, read one receipt | doctor green on the sample | an hour |
| 2 | A reading lane: re-run one committed reading and compare | numbers match | half a day |
| 3 | A second reading lane: run one open measurement row and file it | a verdict line | a day |
| 4 | A data lane: one dialect through the language recipe | the harness reading | a day |
| 5 | A fixtures lane: hazard fixtures with mutants | every mutant killed | a day |
| 6 | A leaf package lane under review | contract tests | a few days |

Guardrails for steps 2 to 6: the lane record names the only files that may change, the path check enforces
it, a critic reads every pull request before the integrator, and no step touches existing schemas, hooks,
the store, the config struct or the tool registry.

To write first: a contributor guide that is this ladder plus the install pins, an owners file that matches
lane ownership, and the queue helper the queue already names.

## 6. Risks

- PL-R1 Contract-first makes the main thread the bottleneck. Mitigation: wave 0 is small and typed, and
  lanes may add new schema packages themselves.
- PL-R2 The path check blocks honest cross-cutting work. Mitigation: a lane may declare a shared file. The
  declaration is the review trigger.
- PL-R3 A full lane gate is slow. Accepted until the selection reading passes. The landing bundle removes
  the second run of an identical tree, which is the waste that can be cut safely today.
- PL-R4 Red CI stalls every merge. Mitigation: fixing main's CI is a standing first lane, and its pass
  rate is on the fleet view.
- PL-R5 Hosts starve again. Mitigation: caps per machine, the gate lease, readings taken on a host with no
  swap pressure.
- PL-R6 A newcomer needs context that lives in private notes. Mitigation: lane records carry everything a
  lane needs.
- PL-R7 A lane proven under one harness fails under another. Mitigation: proofs are commands, not harness
  behaviour.

## 7. Decisions

- PL-D1 to PL-D12 as in section 2.
- PL-D13 No package split of the command package in this effort.
- PL-D14 Measure gate wall time and lease wait from gate logs in wave 0, and put both on the fleet view.
- PL-D15 Staff reading lanes from the first wave. They are the cheapest way to shrink the open backlog and
  the best first lane for a newcomer.

## 8. The wave plan

| Wave | Who | What lands | Exit |
|---|---|---|---|
| 0a | main thread | realignment records, ENG-1, ENG-2, REC-1 (lane kind and allocation), the path check, the gate-time measurement | records gate green, hook-path harness re-read |
| 0b | main thread with file-owned workers | purge wave 1, the licence notice (LIB-12), the closure re-read (LIB-11), the release tag | absences proven, tag cut |
| 0c | main thread | contracts and failing proofs for wave 1, config keys in one batch | each lane starts red |
| 1 | six lanes and five reading lanes | hazard-oracle (HZ-1 first) and the pairing, latency and census reading lanes, lib-facts (HZ-4 and HZ-6), record-heads, record-conditions, landing-bundle, hooks (through BEN-4), BEN-1 bench merge, catalogs | each lane's proof. The HZ-1 reading is in. |
| 2 | eight to ten lanes | hazard-free, hazard-checks, hazard-replay, decidability, hazard-surface (HZ-2), HZ-3 facts after HZ-2, bench-subjects, record-port (engine slot), record-verbs, plan-keys (main thread), whatif-replay, the hooks lane's CHG-8 turn, rules, fleet-ground after REC-8, readiness, cards, test-strength, friction, more readings | unparsed heads at zero, hazard replay read |
| 3 | by reading, after the freeze amendment | engine (ENG-3, ENG-4, then ENG-5), overlap, the what-if verdicts that passed replay, lifetime-sites, hazard-surface (the Terraform flag and cascade, after HZ-9 reads 0.9), the rules lane's CHG-11 and CHG-12, table programs, the first paid proof, the benchmark lane behind the paid gate | RT-M15 read, first outcome reading |

A wave ends with one integration batch, a green CI, a tag and a consolidation pass.

## 9. What stays on the main thread

- Decisions, invariants, goals and every record that governs others.
- Existing schemas and interfaces, the answer contract, the lane kind.
- The store format, the tool registry, the hook contracts, the config struct.
- The engine slot's order, and ENG-1 and ENG-2 themselves.
- Integration, the release ritual and tags.
- Spend approvals and preregistrations.
- Grounding briefs and closing lanes.

Everything else can be handed to a lane with a contract and a proof.

## Decision models

None applies. Lane classes, path ownership, id allocation, merge ordering and wave scheduling are deterministic
bookkeeping over files, diffs and records, so a model could only guess what the data already states. Provider `none`
is the whole design, and opting out means not creating a lane record. Which harness or model runs a lane is operator
configuration outside the repository (ADR-011), not a decision this design asks a model to make.
