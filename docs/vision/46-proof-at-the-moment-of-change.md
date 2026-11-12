# 46 - Proof at the moment of change: fathom re-envisioned from first principles

Status: Proposed 2026-11-12 (architect ruling, owner review pending). The owner asked the architect to decide, and will
inspect before acceptance. Records: ADR-093 to ADR-107. Designs: `docs/design/change-hazards.md`, `docs/design/library-roadmap.md`, `docs/design/beyond-determinism.md`,
`docs/design/typed-records.md`, `docs/design/fleet-picture.md`, `docs/design/proof-program.md`,
`docs/design/parallel-lanes.md`. Plan: `docs/plans/realignment.md` (the roadmap, horizons and critical path of
section 14 live there too). Assessment: `docs/assessments/2026-11-12-realignment.md`. Position: `docs/vision/38-proof-layer-roadmap.md`
(this vision sharpens it and does not replace it).

The ask was "make fathom the one-stop shop for agentic development, with natural-language understanding on a
deterministic engine". The architect worked from a 49-lane read-only inventory, a first draft, an adversarial critique
by five independent critics (88 findings, 10 of them blockers), a rewrite, and a second pass by two critics (42
findings, no blockers, applied). Section 11 lists what the critique overturned, because those corrections are the
most useful part.

Codes in this file carry the prefix V: V-U uplift, V-F finding, V-T thesis holds, V-N thesis fails, V-D decision, V-R
risk, V-C critique correction, V-M measure, OQ owner question. Repository ids are written with their kind: goal G2,
future F82, invariant I14. Proposed record ids: invariants I70 to I77, principles P25 to P27, goals G51 (one outside
team runs the gate), G52 (readings are staffed) and G53 (the engine presence bar). Decisions are ADR-093 to ADR-107. OQ15 is the one new owner question of the second round.

## 1. Words used here

- **Proof layer**: the product. The name is vision 38's and is kept.
- **Record**: an authored, typed unit of intent (a decision, a plan row, a goal).
- **Row claim**: the authored assertion that a plan row is done. Not the `claim` verb.
- **Ruling**: an authored decision on a record (accepted, rejected). **Phase**: a derived state.
- **Reading**: a committed measurement with a verdict.
- **Lane**: one unit of parallel work with owned paths and a proof. Not a session, a machine or a queue item.
- **Named query**: a registered, parameterized computation over facts that answers in the answer contract.
- **Hazard engine**: value-free change and contract checks, computed in the language library's implementation package and
  binary and delivered by the product. A **hazard fact** is what the library computes and a **hazard finding** is what
  the product delivers at a channel. **Check kernel**: an optional evaluator in the language library, not built now and
  scheduled only on a yes to OQ3.

## 2. The uplifted ask

The better question:

> Where does a deterministic engine change the outcome of agent-built software, and how do we put the whole product
> there?

What the ask did not name:

- V-U1 Engine reliability is the first feature. An answer that arrives after the deadline is not an answer.
- V-U2 A rule for where determinism wins (section 4), so any idea can be judged in one line.
- V-U3 One answer contract for every verb: state, basis, evidence, unknowns with reasons, read set.
- V-U4 A flagship: proof at the moment of change, at the edit and at the landing, computed from contracts, facts and
  intent together.
- V-U5 One registry of named queries behind checks, lifecycle conditions, question shapes and fleet views.
- V-U6 Typed intent with derived phases, and a trial that lets a measurement choose the syntax.
- V-U7 Proof liveness: every reading and claim is bound to its inputs and goes stale when they change.
- V-U8 Reading lanes. The backlog is unread measurements, and reading is parallel work.
- V-U9 A users question. All evidence today comes from one operator.

## 3. Findings: where fathom stands

| # | Finding | Source |
|---|---|---|
| V-F1 | Proof works. The gate caught claimed-over-verified in 180 of 436 runs in five days. Strict verify read 2,117 of 2,117 claimed rows on main. Receipts re-derived 50 of 50. Go call edges grade 1.00 and 1.00 against go/types. One clean mechanism win on the bench: the verify task at $0.06 against $0.33 to $0.51. | ADR-018 pilot verdict, `docs/bench/README.md` |
| V-F2 | No agent-outcome win exists. The newest home bench reads 41 of 47 against grep's 44 of 47 at 1.81 times the tokens, on a dev set near its ceiling. Change tasks and bug localization are nulls. On one lookup pilot fathom was ahead of grep at 0.76 times the tokens and behind the best language-server tool, and that pilot "would not survive a clean complete run". No public-benchmark reading exists. | `docs/bench/ledger.jsonl`, `docs/bench/readings/vs-pilot-6-second-reading.md` |
| V-F3 | Those readings were taken on an engine that timed out. Deadline abandons inflate tokens and failed cells, and the plan's own words are that every measurement before the fix measures the bug. | `docs/assessments/2026-10-26-next-phase-plan.md` |
| V-F4 | Pushed advice has not shown benefit. A first sample found 12 of 211 served rules relevant, a corrected sample 9 of 24, and the reading's verdict is that benefit cannot be judged until delivery is fixed. In the guards pilot the rule was served before the edit in 4 of 8 tasks and violated as often as control. | `docs/bench/readings/advice-delivery/2026-10-22-baseline.md` |
| V-F5 | Lossy output compression is closed by measurement: 0.535% of Bash bytes against a 4% bar. | RO-M0 |
| V-F6 | Model judgment over code is closed for now. Stage 0 stopped after $0.0272 with the model at or below a lexical baseline. Eighteen futures wait on a screen that never ran. | EXP-M4 |
| V-F7 | Graph-backed answers are declined on the hook path. In the committed harness reading no hook call was abandoned, 48.5% of graph calls were answered, all of them `verify_plan`, and pre-edit `blast_radius` was declined by name 50 of 50 times, because a 500 ms hook cannot start a 17 to 25 second build. In the call log `blast_radius` returned an error on 57% of calls in 30 days, and `obligations`, the edit loop's workhorse, was abandoned on 8.2%. One function is 96% of the cold build. An index over it brings the build under one second. That fix is specified and unbuilt (RT-195 to RT-199). | `docs/bench/readings/tenancy/rt-m14-2026-10-19.md`, `graph-patch-2026-10-26.md`, the call log |
| V-F8 | The table freeze waits on a tenancy reading that failed twice on host swap. Fathom caused no refuse minute in either window. The second reading put the question to the owner: should the freeze gate on fathom's own clauses alone. System graph tables, the dependency plane, stall detection and the graph re-warm wait behind it. | `docs/bench/readings/tenancy/rt-m1-2026-10-26.md` |
| V-F9 | The backlog is unread readings. About 310 of 903 open plan rows are measurement rows. A keyword pass suggests about 67 of those name the owner, a ruling or spend. Nine of 41 goals are past review with no verdict. The understanding program has 0 of 66 rows done. | `docs/plans/*.md`, `docs/intent/scope/` |
| V-F10 | Intent is typed in pieces and the accepted next step was never built. Plans and futures have CUE schemas. The status vocabulary and three lifecycle checks accepted on 2026-10-28 have no code. Statuses appear in four spellings and no condition is evaluable. | ADR-087, `schema/intent/` |
| V-F11 | The codebase is large. About 496,000 non-test Go lines in five months, 144,000 of them in one `package main`. Bench machinery is about 120,000 lines with tests. | line counts at HEAD |
| V-F12 | Parallel work collided on shared text. In the 30 days before the append-point fixes, 263 of 523 merges conflicted, 247 on append points (mostly the changelog and generated boards) and 63 inside one function. The reading that would show the rate after the fixes is unread. | `docs/bench/readings/f82-overlap-replay-2026-10-19.md`, ADR-075, reading AP-M1 |
| V-F13 | Wiring is deep in one harness. A second harness has 8 hand-pasted hook entries and 16 of 28 capabilities absent, and measurement is single-harness by decision. | ADR-029 decision 6 |
| V-F14 | Outside evidence is split. One preprint found a code-graph server lowered resolve rate, and a study found context files added cost without success. Other papers report gains from graph-anchored retrieval inside the agent's own search loop. All of it reached this review as search summaries, unverified at source. No surveyed spec tool derives status from evidence, and no product ties fleet state to a plan and to proof. | search summaries, not primary reads |

## 4. The thesis, tested

The thesis: because fathom has a deterministic engine, it can do natural-language understanding of code, projects and
organizations better, cheaper and more reliably than tools that run a model at query time.

Verdict: true for verdicts, unproven for context, and the evidence so far cannot tell a bad idea from a bad delivery.

**The rule (V-U2).** Four cases, in order of evidence.

1. **Verdicts and computed sets at forced channels: win.** Proven (V-F1).
2. **Structure inside the agent's own action: untested here.** Anchors on its own search and read results, a suspect
   set on its own red test, a shared read map for sibling agents. Outside results are encouraging. Default off until a
   reading wins.
3. **Optional tools the agent may choose: level.** Agents grep first. Fathom is level with the best lookup tools.
4. **Prose pushed into context: no benefit shown.** Its judge is the holdout design already accepted in ADR-080.

Holds:

- V-T1 Is it done. Claims checked against the index at a blocking gate.
- V-T2 What does this change do. Blast radius, conservation, and infrastructure hazards from versioned contracts joined
  to a diff of classed facts, with no plan and no value read (`docs/design/change-hazards.md`).
- V-T3 What must run. Tests selected by what they read. Sound in design. Its readings are under their bars.
- V-T4 What was intended. Which record governs a path, why, what is overdue. Needs typed links
  (`docs/design/typed-records.md`).
- V-T5 What is happening. Lanes, what they touch, which will collide. Nobody sells it.
- V-T6 Does the fleet meet its own standards. One typed check evaluated over every indexed repository.
- V-T7 Compile once. A check or a named query proven by fixtures costs nothing per use and never drifts.
- V-T8 Evidence a stranger can check. A model answer is never a receipt.

Fails or unproven:

- V-N1 Lookups. Level, not ahead.
- V-N2 Orientation prose and overviews. No gain here or in the literature.
- V-N3 Rationale nobody wrote down.
- V-N4 Pushed prose advice (V-F4).
- V-N5 Output compression (V-F5).
- V-N6 Meaning search. Pair with an outside tool as a measured arm.
- V-N7 Model judgment over code at today's models (V-F6).
- V-N8 Facts that live outside repositories: people, process, runtime. Standards that live inside repositories (CI,
  lint and base-image configuration, owners files, agent instructions, decisions) are inside facts and belong to V-T6.
- V-N9 Proof beyond Go. Call extraction now grades 0.92 to 0.97 precision for Python, TypeScript and JavaScript
  against compiler-backed oracles. The proof verbs are still Go-shaped: `wired` reads not applicable elsewhere, and
  typed edges need an index that is rarely present.
- V-N10 Breadth of infrastructure rules and value resolution on module-heavy code. Free scanners and a plan both beat
  fathom (`docs/design/change-hazards.md`).

These two lists are targets and not refusals (ADR-105). Each of the fifteen targets (V-N1 to V-N10 here and A-N1 to
A-N6 of the policy review, where V-N10 has two halves, A-N1 and A-N2) gets a path of the owner's ladder, a measurement, and a scheduled
row or a trigger. Section 14c counts them.

Consequence for natural language: language in, facts out. A question matches one of a closed set of named queries. The
answer is rendered from cited rows in the answer contract. A model may classify the question through the decision port
once that site passes calibration. A model never writes the answer.

## 5. What fathom becomes

Fathom stays the proof layer of vision 38 (ADR-093). This vision sharpens it in five ways.

1. **Four planes, one engine.**

| Plane | Question | Content | Today |
|---|---|---|---|
| Facts | What is true | symbols, edges, files, config, infrastructure, dependencies, links, owners | strong for Go, cross-service unbuilt |
| Intent | What is meant | asks, decisions, invariants, goals, plans, standards, lanes as typed records | typed in pieces |
| Work | What is happening | lanes, leases, sessions, gate receipts, provenance, overlap | pieces, no single picture |
| Proof | What holds | verdicts joining the other three | the proven core |

   The planes are a data taxonomy. The three code layers of ADR-001 stay as they are.

2. **One answer contract.** Every answer carries a state (holds, violated, unknown), the weakest evidence class it
   rests on, spans, unknowns with reasons and the read set. The hazard design's verdict shape is this contract.
   ADR-069's answer basis is its first field.
3. **One registry of named queries.** A named query has typed parameters, a cost class, an oracle test and an answer in
   the contract. Verify oracles, lifecycle conditions, question shapes, fleet standards and attention items all
   reference named queries. There is no query language.
4. **Decide at forced channels. Inform inside the agent's own action, and only after a reading.** A new channel that
   pushes prose is never on by default without a won reading (ADR-097).
5. **Models at authoring time only.** Behind the decision port, default none. The run-time path is deterministic. A
   drafted check or rule is admitted only through the acceptance rule (ADR-106, I76): it agrees with labels fathom did
   not derive, it kills every mutant derived from it with the predicted transition, and a person accepts it and writes
   it into the repository. There are two drafters and no judge. At run time the caller is the model, and fathom
   scaffolds it with a cited set, the unknowns and a bounded question. For batch authoring the drafter is the layer
   compiler's pipeline. A paired or hybrid source never raises an evidence class (I77).

**The flagship: proof at the moment of change (V-U4, ADR-100).** Two forced channels, one program.

- At the edit: what-if verdicts on the proposed text before it is written. A conserved surface lost, a wiring break, a
  replacement hazard, a duplicate of an existing symbol. Each speaks only on a transition. The edit half has two tiers.
  The delta tier is an internal fact delta for the code card kinds, starting with a removed exported symbol or route. It
  needs no graph and waits on the hook prerequisites (CHG-8). The reach tier needs the graph and waits on RT-M15.
- At the landing: a lane's push carries its proof. Verify verdicts, an exact-tree gate receipt, the affected-test set
  and its overlap with what main reaches. The integrator accepts in seconds and CI samples full runs as the outside
  oracle.

Every part exists or is specified: conservation, the pre-edit hook, the pre-write delta card in the hazard design,
exact-tree receipts, tests selected by reads, receipts. Delivery at the edit is table stakes, since scanners already
ship edit hooks. What is not is what the verdict is computed from, and that it speaks only on a transition. Each
edit-time verdict must show lower noise than the post-write signal for the same hazard before it is built. The landing
half needs no harness parity.

"One-stop shop" is reframed. Fathom is the one layer the other tools plug into. It reads what others compute (SCIP
indexes, coverage, attested test results, provider catalogs, foreign spec files, findings through the review kit) and
emits what only it can prove.

## 6. Capability map

Everything not listed is kept as it is. Program ids (PG-n) are the programs of section 14 and
`docs/plans/realignment.md`. Hazard rows keep the ids HZ-n of `docs/plans/change-hazards.md`.

| Plane | Capability | Today | Move | Program |
|---|---|---|---|---|
| Facts | Graph answers on the hook path | declined or late | uplift: RT-195 to RT-199 | PG-1 |
| Facts | Proof verbs beyond Go, typed edges by default | Go-shaped | uplift | PG-5 |
| Facts | Cross-service links, dependencies, owners | library only, unbuilt, unwired | build after the freeze amendment | PG-5 |
| Facts | Infrastructure contracts and replacement facts | CloudFormation built, Terraform specified | uplift | library roadmap (LP-3) |
| Intent | Typed heads for every record kind, lifecycle conditions | accepted, unbuilt | build | PG-2 |
| Intent | Head syntax (labelled lines, YAML, CUE) | undecided | trial | PG-2 |
| Intent | Lanes, capabilities, standards, named queries as data | none | build as CUE from birth | PG-2 |
| Work | Lane picture, overlap forecast, landing bundle | separate verbs | uplift | PG-3, PG-6 |
| Proof | Verify gate, wired check, conservation, certifications | proven | keep | |
| Proof | Hazard engine (value-free change and contract checks) | half built | build: HZ-0 to HZ-12, with HZ-8 checking the library roadmap's prerequisites (LP-3) | PG-3 |
| Proof | What-if verdicts at the edit | a labs card | prove by replay, then build | PG-3 |
| Proof | Proof-carrying landing | receipts built, off | build | PG-3 |
| Proof | Mutation-killed strength on agent-written tests | ladder exists, unused | uplift through attested results | PG-3 |
| Proof | Fleet conformance | custodian concerns per repository | uplift to the fleet | PG-7 |
| Proof | Proof liveness | certificates only | build | PG-4 |
| In-band | Shared read map for sibling agents, anchors on the agent's own search | unbuilt | replay first, then an arm | PG-11 |
| Delivery | Post-edit prose rules | no benefit shown | ADR-080's holdouts decide | PG-4 |
| Delivery | Lossy readouts, compaction carry | measured STOP | cut | PG-10 |
| Delivery | Question shapes, `why` | unbuilt | build small, judge on the bench | PG-7 |
| Delivery | Observatory | 39 pages, notify and reload | ground view first | PG-6 |

## 7. Slices: take in, pair with, refuse

Admission test. All five must hold, for outside slices and for fathom's own programs alike:

1. Computable from facts fathom holds, or from attested results it ingests. Fathom still runs nothing.
2. The answer is a verdict or a set, not prose.
3. It rides an existing forced channel or verb. No new tool on the default surface.
4. An oracle fathom does not own can grade it.
5. It has a named consumer, or it deletes more than it adds.

Clause 5 is applied row by row, to old rows and new ones alike. A row marked FUTURE is a register entry with a trigger and
not a roadmap row. Scanners stay "pair". Where fathom cannot prove, it pairs with what can and proves the borrowed
answer.

| Slice sold elsewhere | Verdict | Why |
|---|---|---|
| Evidence-gated done under spec tools | take in, lead with it | No spec tool derives status from evidence. Four dialects are already read. |
| Infrastructure change hazards | take in as a hazard engine | Contracts joined to a classed diff. No scanner holds both. No rule corpus. |
| Fleet picture tied to plan and proof | take in | Session boards are common. None shows verified progress or overlap. |
| Work tracking with dependencies | take in as typed records with derived phases | Status by assertion stays refused. |
| Test selection for agents | take in after it meets its bar | Unsold for agents. |
| Catalog and scorecards | take in the computed half | Services, owners, dependencies, adoption of standards. Judgment stays in the review kit. |
| Dependency upgrade impact | take in | Reach-restricted impact over the used closure. |
| Scanners | pair | The review kit ingests them. Fathom ranks their findings by what changed code reaches. |
| Meaning search, memory, review bots, library docs | pair | Measured arms and peers. |
| Wikis, overviews, chat | refuse | V-N2. |
| Output compression, token dashboards | refuse | V-N5. |
| Scheduling, placement, session management | refuse | Fathom reports and never acts. |

## 8. The measure of world class

By the middle of 2027, with fathom building fathom first. Each measure has an oracle fathom does not own. The method
for each is in `docs/design/proof-program.md`.

- V-M1 False-done. On tasks where some plan rows are false by construction, the gated arm ends with fewer false claims
  than the ungated arm, preregistered, on repositories fathom does not own.
- V-M2 Presence. At least 97% of calls answered inside the class deadline and under 1% abandoned, with each hook-path
  tool reported apart. The first step is the bar in PG-1.
- V-M3 The steering view shows local state within 2 seconds and shows the age of every remote number
  (`docs/design/fleet-picture.md`).
- V-M4 Repeated exploration. The share of sub-agent reads that hit a file an earlier agent already read falls by half
  from 69.9%. Tokens per proven row is reported per harness.
- V-M5 One public benchmark reading where the fathom arm beats the grep arm on a preregistered primary at no more than
  1.25 times the tokens. A powered null withdraws the claim in public.
- V-M6 The thesis itself. On the named-query question set, fathom against a query-time model tool: accuracy by anchor
  match, cost per question, and the same answer on a second run.
- V-M7 One team outside the owner's fleet runs the gate for four weeks and files what broke (goal G51).
- V-M8 Minutes per landing are measured, then halved.

## 9. Decisions

Each decision lands as a record. The mapping is in section 13.

- V-D1 Adopt the four-case rule of section 4, and the five-clause admission test of section 7, for every program and
  future.
- V-D2 Keep "proof layer" as the position. Sharpen it with the five points of section 5.
- V-D3 Make proof at the moment of change the flagship.
- V-D4 Engine presence is program one. Start with the unfrozen fixes.
- V-D5 Type every record head now. Choose head syntax by trial. New data kinds are CUE from birth.
- V-D6 Amend the freeze to gate on fathom's own tenancy clauses plus a per-change cost row.
- V-D7 Admit the work plane as read-only observation from operator-moved files.
- V-D8 Build the hazard engine in the library's implementation package and binary. The product delivers it. The check
  kernel is not built now and waits on a yes to OQ3, read on 2027-01-31.
- V-D9 Natural language is an interface over named queries. The understanding bench and its kill stand.
- V-D10 Inform in-band as labs cards with replay first. Nothing in-band is default-on without a reading.
- V-D11 Subtraction has the rank of a build program.
- V-D12 Reading lanes are a lane class. Unread measurements are staffed like code.
- V-D14 The win ladder. Every loss on the two lists of section 4 has a path, a measurement and a date (ADR-105). A paired
  or hybrid source never raises an evidence class, and a draft is never served until it passes the acceptance rule
  (ADR-106).
- V-D13 Order: rulings and unfrozen engine fixes, then four core programs in parallel lanes, then proof, then
  default-on and tuning. Wiring plumbing for the proof channels runs from the start. Tuning is last, as asked.

## 10. Risks

- V-R1 Second system. Mitigation: four core programs, the rest entry-conditioned.
- V-R2 The proof position reads as "fathom does less". Mitigation: the flagship is something nobody else can do, and
  V-M5 and V-M6 are published either way.
- V-R3 The in-band case becomes pushed advice by another name. Mitigation: default off, replay first, a preregistered
  arm, the same holdout rule.
- V-R4 The fleet picture drifts into a tracker. Mitigation: no view writes a phase (ADR-049 decision 4).
- V-R5 Engine work is deferred again. The fix has sat specified and unbuilt for seventeen days. Mitigation: it is on
  the main thread before fan-out.
- V-R6 The understanding bench reads null. Then the product is the proof layer without the understanding claim.
  Planned for.
- V-R7 Outside evidence in V-F14 was read from summaries. Mitigation: a primary read before any of it is cited in a
  record.

## 11. What the critique overturned

- V-C1 "Informing loses." The evidence supports "this delivery lost". The rule is now four cases with an in-band case
  that gets its own arm.
- V-C2 TypeScript 0.52 and JavaScript 0.19. A month stale. The committed readings are 0.97.
- V-C3 "Reverse three rulings and author everything in CUE." The premise overreached, the kill could not fire, and a
  cheaper design was missing. Now: type every head, build what was already accepted, and let a paired trial choose the
  syntax.
- V-C4 "The freeze is the gate on the engine." The largest latency fix does not wait on it.
- V-C5 "Cut pushed advice on a clock." ADR-080 already has an effect holdout that never auto-disables.
- V-C6 "Adopt the hazard review as written", while adding a SARIF door, plan files in the product and an unconditional
  kernel. The review refuses all three. Its real verdict is the hazard engine.
- V-C7 "Split the 144,000-line command package before fan-out." Verbs already register per file, and the split would
  unbind 563 claimed rows.
- V-C8 "Rulings are the bottleneck." Readings are. Most gated rows are measurements nobody has run.
- V-C9 "Fathom pulls lane state over ssh and syncs issue labels." Both break kept invariants. The operator's tools move
  bytes. Fathom reads local files and writes no issue.
- V-C10 Withdrawing index snapshots and folding threads into lanes. The first stays parked as its own kill prescribes.
  The second was a preference with no fired kill.

## 12. Decisions for the owner

Each row gives the options, the recommendation, and what happens if there is no answer. The owner asked the architect
to decide, so every row has a default and the default is what the plan assumes until the owner answers.

| # | Question | Options | Recommendation | Default if unanswered |
|---|---|---|---|---|
| OQ1 | Product or force multiplier (open in the intent corpus as question Q4) | product, force multiplier | product, proven on the fleet first | product |
| OQ2 | Name one outside team for a four-week trial of the gate. A second vehicle is possible: a pull-request check for infrastructure changes, if the named team has infrastructure code | name one with either vehicle, fleet only | name one by 2027-01-31, either vehicle | record "fleet only" on that date |
| OQ3 | Restated as a yes or no. Where the checks live is answered: the library. Build the check kernel in the library: yes or no? Yes schedules HZ-13 to HZ-16, HZ-18 and the acceptance test (LIB-10) after HZ-3, HZ-4 and HZ-6. "The product never reads a value" stands, since the owner did not speak to it (see `docs/design/change-hazards.md`) | yes, no | yes once the policy tool has a first design that evaluates through the library | no on 2027-01-31, hazards only |
| OQ4 | Licence for the library door and the catalog format, and whether the catalog is published | permissive at split time, keep | decide at split time, publish the catalog. Unchanged, and IF-6 (catalog attestations) waits on it | current licence stays, no publication |
| OQ5 | Freeze amendment: gate on fathom's own tenancy clauses plus a per-change cost row | yes, no | yes | the engine re-warm and the table programs wait |
| OQ6 | The cuts that are preference, not fired kills: assess lanes, the ambient A/B goal | cut, keep frozen | cut | stay frozen |
| OQ7 | The nine overdue goal reviews: may the architect record the verdicts | yes, owner reviews each | yes | recorded as architect rulings, reversible |
| OQ8 | Record head syntax: decide by the paired trial | trial, rule now for CUE, rule now for markdown | trial, after the two scheduled readings | trial |
| OQ9 | Ruling classes, a protected main with required CI, a second integrator | each yes or no | yes, yes, none yet | classes adopted, main unprotected |
| OQ10 | Spend: the bench shakedown cap, a separate cap for the record syntax trial, and which harnesses are arms, and spend for the model arms of the hazard benchmark and the reading benchmark (neither calls the daemon) | amounts | $120 for the shakedown, a small cap for the trial, both build harnesses, and a small cap for each model arm | no paid spend, and the trial does not run, the hazard benchmark runs with scanner arms only and the reading benchmark does not run |
| OQ11 | First fan-out size | four to ten lanes | six lanes plus two reading lanes | six plus two |
| OQ12 | Fleet transport between your machines | synced directory, git refs, your own ssh pipe | synced directory | synced directory |
| OQ13 | Largest repository fathom promises to serve | a number | the architect proposes one from a reading | architect's number |
| OQ14 | Ask 3 said "state machines". Record lifecycles, or also state machines in the code under analysis | lifecycles, both | lifecycles now, the second as a future | lifecycles |
| OQ15 | New. Name one outside adopter with infrastructure code, or confirm a fleet repository from the census (UND-1). It satisfies T-IaC. Each register entry that names T-IaC still needs its own second condition | name one, confirm a fleet repository, neither | read the census first, then name one by 2027-01-31, together with OQ2 | the entries that need T-IaC stay closed, and hazards are proven by replay and the public benchmark only |

## 13. Records and where the detail lives

| Code | Record | Subject |
|---|---|---|
| NA-1 | ADR-093 | the proof layer sharpened: four planes, the four-case rule, the admission test |
| NA-2 | ADR-094 | typed records with derived phases, syntax by trial |
| NA-3 | ADR-095 | freeze amendment: gate on fathom's own tenancy clauses plus a cost row |
| NA-4 | ADR-096 | engine presence as program one |
| NA-5 | ADR-097 | delivery default-off until a reading wins |
| NA-6 | ADR-098 | the work plane, read-only |
| NA-7 | ADR-099 | the hazard engine |
| NA-8 | ADR-100 | proof at the moment of change |
| NA-9 | ADR-101 | one proof program |
| NA-10 | ADR-102 | every harness measured |
| NA-11 | ADR-103 | ruling classes |
| NA-12 | ADR-104 | subtraction as a build program |
| NA-13 | ADR-105 | the win ladder: targets with paths, weakest evidence class wins (I77), three evidence classes, the paid gate clause |
| NA-14 | ADR-106 | the acceptance rule for drafted checks (I76) |
| NA-15 | ADR-107 | whole-program resource evidence for Go, F90 amended for RL-3 and RL-4 only |

Designs: hazards in `docs/design/change-hazards.md`, record heads in `docs/design/typed-records.md`, the fleet view in
`docs/design/fleet-picture.md`, measurement in `docs/design/proof-program.md`, lanes and waves in
`docs/design/parallel-lanes.md`. Rows, horizons and the critical path: `docs/plans/realignment.md`, with the hazard,
record, fleet and proof programs in `docs/plans/change-hazards.md`, `docs/plans/typed-records.md`,
`docs/plans/fleet-picture.md` and `docs/plans/proof-program.md`. The library side of the plan is `docs/design/library-roadmap.md`, and the win paths for the two lists of section 4 are in `docs/design/beyond-determinism.md`. The assessment with the evidence behind the findings
is `docs/assessments/2026-11-12-realignment.md`.

## 14. The order: horizons and the critical path

The ask: add capabilities, prove them, then wire and tune. The order keeps that shape and moves two things forward.

- **Engine presence first.** Graph-backed answers are declined on the hook path. Every capability rides the same graph,
  and a measurement taken on it measures the fault. The fix is specified and small.
- **Plumbing for the proof channels from the start.** A wire stamp per session and parity for the claim gate on every
  harness that builds fathom. Tuning and default-on stay last.

Four programs are the core: PG-1 engine presence, PG-2 typed records, PG-3 proof at the moment of change, PG-4 the
proof program. The other eight (PG-5 facts wider, PG-6 fleet picture, PG-7 understanding and organization, PG-8
wiring, PG-9 library and catalogs (now the library roadmap, section 14a), PG-10 subtraction, PG-11 in-band structure, PG-12 default-on and tuning) start when
their entry condition holds. Rows and entry conditions: `docs/plans/realignment.md`.

| Horizon | Waves | Exit events |
|---|---|---|
| H0 Realign and unblock | 0a, 0b, 0c | (1) the realignment records are landed and OQ7, OQ9 and OQ11 are answered or defaulted. OQ5 gates only the rows that need the freeze amendment, and OQ6 only the preference cuts. (2) RT-195 and RT-197 are deployed. (3) The hook-path harness is re-read. (4) Purge wave 1 is proven gone. (5) The lane kind, id allocation and the path check exist. (6) Wave 1 lanes have contracts and failing proofs. (7) A release tag contains the language library. |
| H1 Core capabilities | 1, 2 | each core program's done-when |
| H2 Proof | 3 and the paid readings | capability cards read, the first combined run, the understanding bench verdict |
| H3 Default-on, tuning, outside use | later | V-M1 to V-M8 |

No date is set for H0. Its limit is attention and rulings, not commit throughput: the engine fix has sat specified for
seventeen days and main has had no commit since 2026-11-06. Existing kill dates stay as anchors: 2027-01-31 (the old
`ask` spine, mods, concern-fact waves, labs), 2027-03-31 (the understanding program), 2027-05-04 (the library door
sunset).

Two gates for proof work, stated once:

- Free steps (fact proofs, forward logging, readiness harnesses) start at the H0 exit.
- Paid steps need all of: a wire stamp on every cell, the readiness harnesses passing, RT-M15 reading PASS, a signed
  preregistration and an approved spend row. A reading that never calls the daemon is excused from RT-M15 and the wire
  stamp, and only from those (ADR-105).

The critical path:

```
records and rulings (ADR-093 to ADR-107, OQ7, OQ9, OQ11)
  -> lanes can start: REC-1 lane kind, allocation, path check, contracts and failing proofs

ENG-1, ENG-2 (not frozen)
  -> hook-path harness re-read -> fan-out

OQ5 freeze amendment + fresh tenant-clause week (runs beside wave 1)
  -> ENG-4 re-warm -> ENG-5 RT-M15 reading -> paid proof steps
  -> system graph tables, dependency plane

REC-2 to REC-9 typed heads, conditions, queries -> fleet bars, why, named queries
WIR-1 wire stamp + BEN-2 readiness harnesses -> paid proof steps
```

Three gates decide what follows them.

1. **ENG-1.** One indexed lookup is 96% of the cold build. It is specified, measured and unbuilt, and it waits on
   nothing.
2. **The freeze amendment.** It unblocks the re-warm and every table program. The reading that failed asked the owner
   this question on 2026-10-26.
3. **RT-M15.** Until at least half of hook-path graph calls are answered, no paid proof is valid and no reach-tier what-if
   verdict can be trusted at the edit. The delta tier does not wait on it.

If only one thing is done this month, it is ENG-1.

## 14a. Two roadmaps, one product

The main roadmap (`docs/plans/realignment.md`) and the library roadmap (`docs/design/library-roadmap.md`) are one module and
one release train. The library roadmap owns computation in the library's packages and binary, and the checks that go hand
in hand with it. The main roadmap owns delivery, joins and proof. The owner's instinct that checks belong with the base is
right at the package level: same module, same implementation package, same binary, same release. It is early at the API
level, because a public promise with no user is a cost with no reader. Rule corpora stay outside, because every rule change
would become a library release and breadth would drift to the mean.

**The main roadmap consumes the library.**

| Main row | Needs from the library | Effect |
|---|---|---|
| PG-3 done-when: the hazard engine reads on Terraform and CloudFormation | LP-4 | A program on one roadmap closes on the other (joint 2) |
| PG-3 IF-14: the Terraform replacement flag in `review_diff` | HZ-9 at 0.9 | No Terraform hazard finding before the reading |
| PG-3 HZ-2: `iac` as a review dimension, with a stated absence when no catalog is placed | nothing new | Phase one |
| The live CloudFormation flag (IF-C4) | HZ-1 at 0.85 | Under it the flag drops to advice wording |
| PG-3 IF-16: cascade in `blast_radius` | IF-15 | |
| PG-3 HZ-19: the pre-write infrastructure card | HZ-6, CHG-8 | Gated by ADR-091 decision 7 and the IF-M6 trigger |
| PG-4 BEN-10: the public hazard benchmark | HZ-9, HZ-10, LIB-7 | Joint 3 |
| PG-4 BEN-9 with FCT-8: liveness | catalog digest and version binding on a hazard fact | |
| PG-7 UND-1: the census | HZ-4 for provider bindings | |
| PG-7 UND-7: infrastructure question shapes | HZ-9 read | |
| PG-5 FCT-11: `referenced` for configuration | a cross-file resolver for config dialects | Entry: the census |
| PG-3 CHG-10: rules compiled once | RL-2 where a rule reads site facts | |

**The library waits on the main roadmap.**

| Library row | Waits on | Why |
|---|---|---|
| Every library lane | wave 0a (REC-1, the lane kind and id allocation) and wave 0c (contracts and failing proofs) | Typed records gate every lane |
| LIB-1, then LL-M7 | LIB-12, LIB-11, one green wave on main, the release ritual | No tag contains the door, its closure reading is stale, and the archives owe a licence notice |
| IF-6 | signed releases with provenance (WIR-6), and OQ4 | Attestations and licence |
| IF-M2 | an amendment to its `needs` (ADR-099) | The plan binds it to IF-14 and IF-16, which open only after 0.9 |
| The library half of BEN-10 | BEN-1, BEN-2, BEN-5 | The driver, the readiness harnesses, subject admission |
| RL-3 | the engine presence bar (PG-1), ADR-107, a cost reading | It runs in the typed pass |
| RL-4 | ADR-107, the store's cost gate (ENG-3), a cost reading at ingest | "No delete anywhere" is a whole-program fact |
| RL-1, RL-6 | FCT-6 | Attested analyzer output and runtime profiles |
| RL-5 | the operator turning on the runtime's leak profile in tests | Fathom runs nothing, and no leak detector is in use in the fleet today |
| LIB-13 | CHG-1's replay cards | The delta is built inside the what-if work |
| LIB-14 | BEN-5 | Its fixtures are built first. BEN-11 then waits on LIB-14 and the paid gate |
| Any agent surface for hazards | PG-3's delivery rows and ADR-091 decision 7 | The library computes, the product delivers |
| The model arm of BEN-10 | readiness harnesses, a signed preregistration, a spend row | Not RT-M15 and not the wire stamp: it never calls the daemon (ADR-105) |
| Register rows | each entry's own trigger | OQ15 satisfies T-IaC. An entry that names a second condition still needs it |
| The kernel | nothing on the main roadmap | Only OQ3 and its own fact rows |

**Five joints.**

1. **The tag.** Main's release unblocks the library's outside reading. Before it, LL-M1 is re-read.
2. **PG-3 closes on LP-4.** The flagship's done-when names hazards on two tools, and that work sits on the library
   roadmap, gated per tool: HZ-1 for CloudFormation and HZ-9 for Terraform.
3. **The hazard benchmark (BEN-10).** It needs HZ-9 from the library and BEN-5 from main. It is the one infrastructure row
   with a consumer, an outside oracle and no competitor.
4. **The hook prerequisites (CHG-8).** Found from the library side and owned by main. Every what-if card needs them, and
   the delta tier then does not wait on engine presence.
5. **Attested evidence (FCT-6).** Analyzer output and runtime profiles (RL-1, RL-6) enter through it.

## 14b. The critical path across both roadmaps

1. **Main: ENG-1, then ENG-2.** Unchanged. The engine fix is still the first thing on the main thread.
2. **Both, in wave 0b: the licence notice, a re-read of LL-M1, then one green wave and the tag (joint 1).** Then LL-M7.
   Until then nothing outside can use the library, and ADR-092's revisit date is under six months away. No further archive
   should ship without the notice.
3. **Wave 1 readings: HZ-1 first, then HZ-0, TQ-M3, the census and the cold-latency reading.** HZ-1 comes first because its
   finding is already served to agents. It needs a small harness. The others need no new code.
4. **Library: HZ-3, HZ-4, HZ-6, then HZ-10 and LIB-7, with IF-13 built beside them.** Catalog-free hazards give the first
   Terraform replay reading before the replacement scan lands.
5. **Main, in the same waves: the hook half of CHG-8 and the build parts of the pairings.** They have a consumer today and
   no dependency on the engine.
6. **Library: HZ-9 at 0.9 (joint 2).** Then main delivers the Terraform flag, and the benchmark is published (joint 3).
7. **Beyond hazards, as their entries open:** the fact delta inside the what-if work, site facts with the tenancy lint
   ported onto them, then the two benchmarks.
8. **On the owner's answers:** OQ15 satisfies T-IaC for the infrastructure register. OQ3 schedules the kernel.

If only one thing is done this month on the library side, it is the notice and the tag. If two, add HZ-1. The main
roadmap's one thing is still ENG-1. The library roadmap adds what runs beside it, and the honest size of that roadmap: its
potential is eight primitives and 67 judged uses, its schedule is the part that has a reader today, and the first wave
finds out who else will use it.

## 14c. Beyond determinism

The owner asked to win where determinism does not win today. `docs/design/beyond-determinism.md` answers it for the fifteen
targets of section 4: ten have scheduled work, one is refused with a wins-if (V-N5, lossy output compression), and four
wait on a trigger (V-N8, A-N1 which is the first half of V-N10, A-N4 and A-N6), three of them on T-IaC. Its laws are recorded in
ADR-105 (the ladder, the weakest evidence class wins) and ADR-106 (the acceptance rule for drafts). Three moves matter
most: compile one rule from one incident, test the thesis on ground fathom does not own (the hazard benchmark, then the
reading benchmark), and let outside oracles grade fathom before they feed it, starting with HZ-1 and TQ-M3.

## 15. Decision models

No verdict in this vision calls a model at run time. The sites it names, with the typed question each could ask:

| Site | Typed question | Fallback with provider none | Opt-out | Kill measurement |
|---|---|---|---|---|
| Question shape (UND-6, ADR-093) | which closed named query does this question match | pattern match by keyword and anchor, and an unmatched question gets a refusal that names the nearest shapes | the existing switches, provider none by default | the site stays off until calibration passes, and the understanding bench reading (kill 2027-03-31) decides the rest |
| Condition kind (REC-4, ADR-094) | which condition kind fits an existing prose condition | every condition is `manual` with a `review_by`, the day-one state | the existing switches | under 50% of proposals accepted unchanged on 40 |
| Secondary grader on the understanding bench (BEN-M9, ADR-101) | does the answer match the anchor when the match is not exact | the primary anchor match stands alone | off by default | agreement under 0.6 with person labels |

The syntax trial (REC-T) is a paid agent trial that measures authoring, not a decision site. Drafting a record head from a prose
ask is generation, outside the port's kinds, and its acceptance is the vet. Everything else (hazard verdicts, the answer
contract, named queries, the fleet picture) is deterministic and asks no typed question, because a verdict that a stranger must
re-check cannot rest on a model (V-T8). A model never resolves an unknown or judges a security property (ADR-099). ADR-105 to ADR-107 add no site and no kind to
the port: drafting a check or a record from prose is generation, outside the port's kinds, and ends at the acceptance
rule (ADR-106).

## 16. The vision in one paragraph

Agents now write most of the code. They are fast, they forget, they overclaim, and a fleet of them collides. What they
cannot supply for themselves is ground truth. Fathom is that ground truth: it computes facts about the code, holds
intent as typed records, observes the work, and joins the three into verdicts that carry their evidence. Its strongest
move is proof at the moment of change. Before an edit is written it says what the edit would break. When a lane lands,
the push carries its own proof. It answers questions when asked, through named queries with a fixed contract that says
what holds, what is violated and what is unknown. Whether it should also inform an agent inside its own search is an
open question this plan measures instead of assuming. Scanners, spec tools, reviewers and memory products plug into it.
The ambition is to be the layer nothing else can replace, to prove that on tasks and repositories fathom does not own,
and to withdraw any claim a powered reading does not support.
