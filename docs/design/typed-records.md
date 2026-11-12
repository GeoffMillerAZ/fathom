# Typed records: intent, plans, backlog and progress as validated, queryable, lifecycle-driven records

Status: Proposed 2026-11-12 (architect ruling, owner review pending). Decision: ADR-094 (the new record). Plan: `docs/plans/typed-records.md` (REC-). Vision: `docs/vision/46-proof-at-the-moment-of-change.md` (owner questions OQ1 to OQ14 live there, this design's is OQ10). Re-judges ADR-018, ADR-074 R1, ADR-087 decision 1 and the July 2026 verdict "build a reconciler, not a machine". Proposed invariants: I70, I72, I75.

The first draft of this design reversed those rulings outright. A critic assigned to defend them showed the reversal overreached. This is the position that survived.

Codes carry the prefix CU: CU-F finding, CU-K kept from the old rulings, CU-C critique correction, CU-D decision, CU-M measurement, CU-R risk. Sources in the repository: `ADR.md` (ADR-018, ADR-026, ADR-074, ADR-075, ADR-087), `docs/vision/09-deterministic-sdd.md`, `docs/plans/README.md`, `schema/plan/`, `schema/intent/`, `docs/plans/project-understanding.md`, `docs/plans/intent-v2.md`.

Reading order: the verdict is the last section.

## 1. The ask, split in two

The owner asked for specs, plans, decisions, backlog and progress to move from markdown to CUE: machine actionable, discoverable, validated, kept current deterministically, queried instead of grepped, driven by CUE state machines. The owner also wrote, in ADR-074: "I really don't think we should be doing things because I've said so."

The ask has two parts, and they deserve different answers.

- **Part 1: typed, validated, queryable, lifecycle-driven.** Yes, for every record kind, now. Most of it was accepted on 2026-10-28 and never built.
- **Part 2: the authored bytes are CUE.** Yes today for data that has no prose. For records that carry prose, a measurement should choose the syntax, and no instrument that could choose it has ever existed.

## 2. Intent that applies, named before designing

- **ADR-087** decided a CUE status vocabulary, three lifecycle conditions at HEAD, an unparsed-status count and a structured-share count. Its wins-if governs authored syntax. This design amends it (CU-D12).
- **ADR-074** is the practice that kept markdown records and set the wins-if. Its R1 is re-judged here. Its stated costs ("short records three times larger", "readers are models") get numbers in CU-M4.
- **ADR-026** owns the annotated-condition scheme that nobody used in 30 days. Typed conditions (CU-D4) replace the annotation request with a tool pass.
- **ADR-018** keeps its core: SDD state is derived, never driven. CU-K1 to CU-K8 carry it forward unchanged.
- **I62:** no model sets a ruling or a condition.
- **I70** (proposed): no derived field is authored. A record that writes a phase or a condition fails vet.
- **I72** (proposed): a record link resolves. An id is allocated from a lane range and defined in exactly one file.
- **I75** (proposed): the lifecycle evaluator has no side effect and no clock of its own.
- **ADR-011:** vendor names in data are not org names. No org-specific name enters a schema or a fixture.
- **Revisit triggers.** ADR-087's second trigger, more than five unparsed statuses a month, has no instrument. The keep-or-kill readings of the intent practice are due 2026-11-17. The lifecycle-condition reading PU-M4 is due 2026-12-15. The decision-file split trigger is conflict share, and its reading is unread.

## 3. The re-judgment

### 3.1 Kept from the old rulings

- CU-K1 Phases are derived, never authored and never stored. A declared state can lie.
- CU-K2 Conditions are named and implemented in Go. No expression language.
- CU-K3 Rationale stays prose.
- CU-K4 Fathom writes no transition history. No edge is judged until the backtest PU-M5 reads.
- CU-K5 No dispatch. A lifecycle is evaluated. It starts nothing.
- CU-K6 Markdown plan grammar v2 is a permanent dialect. Foreign dialects synthesize it, the default tool surface accepts it as a parameter, and adopters author it.
- CU-K7 `fathom verify` keeps its exit codes and their meaning.
- CU-K8 The kit pattern: CUE declares the vocabulary, Go owns the law.

### 3.2 What is true today

- CU-F1 The accepted design is unbuilt. ADR-087 decided a CUE status vocabulary, three lifecycle conditions at HEAD, an unparsed-status count and a structured-share count. None has code. The plan that carries them holds 0 of 66 rows, fifteen days on.
- CU-F2 The wins-if for CUE authoring was read once by hand on 2026-10-28 and did not fire: goals 28% structured bytes, futures 7.5% to 13.7%, plan rows 38%, against 50%. Its second trigger, more than five unparsed statuses a month, has no instrument.
- CU-F3 That test measures prose volume. It cannot tell whether typed heads should exist, because a decision's head is always a small part of its bytes while every machine reader reads only the head.
- CU-F4 Two scheduled readings bear on this and neither has been read: the keep-or-kill readings of the intent practice (due 2026-11-17) and the lifecycle-condition reading PU-M4 (due 2026-12-15).
- CU-F5 Plans and futures already have CUE schemas. The plan round trip is lossless for the row model and held by a test. It drops plan prose, so it is not a complete copy of a plan file.
- CU-F6 Real costs of prose as the machine source, by kind:

| Kind | Cost | State |
|---|---|---|
| Plan rows | A bespoke one-line language with 38 keys. Escaped quotes and a missing space once dropped rows in silence. | Parse silence is fixed and held by a test |
| Plan rows | Fields that are not keys: 94 rows carry `(lane: x)` in the title, about 310 carry `(op)`. | Open |
| Decisions | Statuses in four spellings. Superseded-by links unchecked. | Open, assigned to the unbuilt rows |
| Goals | 9 of 41 past review with no verdict, nothing computes "overdue". | Open, same |
| Conditions | About 300, none evaluable. An annotation scheme was offered and nobody used it in 30 days. | Open |
| Refusals | 246 of 252 refused plan rows carry no reopening condition. | Open |
| Friction log | Free-text status, no closing verdict, about half open. | Open |
| Invariants | The enforcement rung is typed by hand. Two rows say VIOLATED while the rows that enforce them have landed. | Open |
| Queue | 108 queue items live as issue labels and comments. | Outside the repository |
| Reach | 14 of 69 invariants reach an edit, through a generated guard file. | Partly fixed |

- CU-F7 Merge collisions are a weak argument here. The 247 append-point conflicts of the September replay were mostly the changelog and generated boards, before fixes that have since landed. The decision file conflicted in 6 of 444 merges. The reading after the fixes is unread.
- CU-F8 No surveyed spec tool authors plans in a typed language, and no public measurement exists of how well agents author CUE against YAML or markdown.

### 3.3 What the critique corrected in the first draft

- CU-C1 "The wins-if cannot fire." It was read by hand and did not fire.
- CU-C2 "The CUE form is a complete copy of every plan." Only of the row model.
- CU-C3 "Unification is a duplicate-id gate for free." Two files that set different fields on one id unify in silence. Only conflicting values fail.
- CU-C4 "Delete five prose readers." The markdown plan parser has 43 call sites, feeds every foreign dialect and backs a parameter on the default tool surface. It stays.
- CU-C5 "A generated decision index keeps old references working." Twenty-one plan rows match sentences in decision bodies, and 109 Go files read the decision file.
- CU-C6 "Verdict equivalence is the migration proof." It passes by construction. The consumers that would break (self-skip against history, `--since` ranges, the claim gate, the stop hook, archive and restore) go untested by it.
- CU-C7 "The kill is binding." No measurement in the draft could have kept markdown as the source.
- CU-C8 The cheaper design was missing: typed heads in a markdown file, vetted by CUE schemas.
- CU-C9 "The Kubernetes lesson was applied halfway." Kubernetes specs are authored in a human syntax and checked against a schema. That is what fathom does with plans.

## 4. The design

### CU-D1 Type every head

A record kind's head is typed when at least one machine consumer reads its fields. Every kind below passes. This replaces the structured-bytes test for the question "should a head be typed". It does not decide the syntax the head is authored in.

### CU-D2 Three containers

| Container | Kinds | Authored as |
|---|---|---|
| Data with no prose | lanes, capabilities, standards, the named-query catalog, the knob map, reading heads, lifecycle declarations | CUE files, now. Hand-authored CUE data is already in scope (the non-goal was narrowed on 2026-10-17). |
| Records with prose | decisions, invariants, goals, futures, questions, incidents, observations, asks | One typed head and a markdown body in one file. Until the trial reads, the head is the labelled lines people already write, parsed and vetted. |
| Plan rows | plan rows | Markdown grammar v2, with new keys (CU-D8). |

### CU-D3 Lifecycles: a declared vocabulary and named conditions, computed at HEAD

This extends ADR-087 decisions 2 and 3 to every kind and builds them.

- CUE declares, per kind, the authored values and which named conditions apply.
- Go implements each condition. Each is computed at the current commit.
- A regression is a difference between two verify ledger rows, which an existing view already shows. It is not a lifecycle phase.

The line between authored and derived:

| Kind | Authored (a person or agent decides) | Derived (computed, never written) |
|---|---|---|
| plan row | row claim (open, in progress, done, skipped), disposition, reason | blocked, proven, wired, stale proof |
| decision | ruling (proposed, accepted, rejected, deferred, withdrawn), `supersedes` | superseded (another record supersedes it), SupersederResolves, RevisitDue |
| invariant | statement, `enforced_by` | rung: the test exists, a gate reaches it, the last attested run passed |
| goal | bet: instrument, threshold, review date | met (the reading says so), ReviewDue, ReviewRecorded |
| future | promote and drop conditions | ConditionFired, ReviewDue |
| incident | outcome links | OutcomeResolves |
| observation | verdict (fix, wontfix, duplicate, ceiling) | open, aged, fixed (a fix-proof commit resolves) |
| reading | instrument, input fingerprint, verdict | live, stale |
| lane | owned paths, contract files, proof command, id range | active, idle, landed (from git and receipts) |
| capability | surface keys, claim | fact-proven, delivery-proven, outcome-proven |

### CU-D4 Typed conditions

| Kind | Meaning | Evaluated from |
|---|---|---|
| `date` | after a date | the clock passed to the evaluator |
| `record` | another record has a ruling or condition | the record set |
| `reading` | a named reading has a verdict | reading records |
| `manual` | only a person can judge, with a `review_by` date | becomes ReviewDue after the date |

A one-time tool pass gives every existing condition a `review_by`. That makes all of them at least date-evaluable on day one, with no hand annotation. The earlier annotation scheme failed because it asked people to annotate. A `metric` kind with a fixed comparator is added only when a reading needs it. Conditions that reference records form a graph, and a Go law refuses a cycle.

### CU-D5 One loader behind the dialect port

`internal/records` loads every kind into one model: the markdown dialects through their existing parsers, CUE files through the CUE API. Consumers reach records only through this port. The markdown plan parser is the permanent markdown dialect behind it. What is deleted is direct calls that bypass the port, proven by a `gone:` row. The package is a leaf with no store import, so it can move to the kit when a second evaluator exists.

### CU-D6 Ids are allocated, and defined once

- `fathom rec new` takes the next id from the lane's reserved range.
- The existing record-id check already refuses duplicate decision, invariant, future, incident and plan ids. It is extended: observation ids, lane ranges, and a clause that each record key is defined in exactly one file inside the defining lane's owned paths. This is the duplicate gate. Unification is not (CU-C3).
- The integrator mints ids until this lands. After that lanes allocate, decision ids included.

### CU-D7 Vet is its own gate step

`fathom rec vet` runs at pre-commit and as a step of the gate: shape per kind, links resolve, ids defined once, no derived field authored, no condition cycle. `fathom verify` is unchanged. A broken decision record never makes verify exit 1, which every adopter's hook reads as "a claimed row is unproven".

### CU-D8 Plan grammar gains the missing keys

`lane`, `wave`, `feature`, `gate` (replaces `(op)` in titles) and `capability` become annotation keys. The parser already tolerates unknown keys. Three things still change: the plan schema is closed and gains the keys, the console stops reading `(op)` from titles, and about 400 titles are backfilled by tool in one main-thread batch. Adopters' plans are untouched. The fleet picture gets its progress bars from these keys.

### CU-D9 Reading: query, show, why

- `fathom rec query` selects by kind, ruling, condition, link, lane, wave, text. `--json` always.
- `fathom rec show <id>` renders a record with its derived conditions.
- `fathom why <id-or-path>` walks typed links from a record to plan rows to symbols to receipts.
- No new MCP tool. `obligations`, when asked, also returns the records that govern a path. Nothing new is pushed at the edit except checkable guards.

### CU-D10 The syntax trial

A preregistered, paired trial on shadow copies decides how heads of prose-bearing records are authored.

| Arm | Head | Body |
|---|---|---|
| A0 control | labelled lines in markdown, as today | same file |
| A1 | YAML front matter | same file |
| A2 | a CUE block at the top of the markdown file | same file |
| A3 | a `.cue` file | a separate markdown file |

- Subjects: 30 existing records per kind, converted by tool into each arm and spot-checked.
- Tasks, run by agents on each harness that builds fathom: author a record from a prose ask, edit a field, add a condition, supersede a record, answer ten questions that need fields, resolve a two-lane merge.
- Instrument: a deny log at the vet, so first-write failures are counted, not just fixed.
- Primary metrics: silent errors (valid but wrong) and first-write validity. Secondary: tokens read per answered question, repair turns, diff size.
- Noise floor: the control arm is run twice. The difference between the two runs is the floor.
- Decision rule: a non-control arm is adopted only when its reduction in silent errors clears the floor at the preregistered power, and it is not worse than control on first-write validity by more than two points. Anything less goes to control and nothing migrates. Between A1 and A2, A2 wins a tie because its head vets against the schema with no translation.
- It starts when the 2026-11-17 readings are read. PU-M4 informs the reading if it exists by then and is not a precondition.
- It is paid agent work with its own cap (OQ10). It does not wait on the engine readings.
- **Prediction, written before data:** A1 and A2 beat A0 on silent errors, A3 loses on first-write validity, and A2 is adopted for decisions, goals and futures. Plans stay grammar v2.

### CU-D11 Files stay where they are until a trigger fires

One file per record already holds for observations, change notes and epoch bumps. The decision file is not split now. Its recorded trigger is conflict share, the reading is unread, and a split costs 109 Go readers, 21 lexical rows and the kit's own skills.

## 5. Measurements

- CU-M1 Typed coverage. Unparsed heads read zero for every kind, with file and line when not.
- CU-M2 The trial (CU-D10).
- CU-M3 Gate cost. Vet of the full corpus under 2 seconds cold and under 200 milliseconds for one changed file.
- CU-M4 Size and reading cost. Measure record size and tokens read per answered question in each arm. This answers ADR-074's stated costs ("short records three times larger", "readers are models") with numbers.
- CU-M5 Collisions on record paths: its own run of the overlap replay script restricted to record files, since AP-M1 counts every append point.
- **Stop:** if CU-M1 cannot reach zero for a kind without rewriting its records, that kind keeps its labelled lines and a counted unparsed share, as ADR-087 already specified.

## Decision models

One candidate site: `condition_kind`, a choice among the condition kinds of CU-D4 for an existing prose condition. With provider `none` every condition is `manual` with a `review_by`, which is the day-one state. Opt-out: the existing switches. Measurement: share of proposals a person accepts unchanged, kill below 50% on 40. Drafting a record head from a prose ask is generation, outside the port's kinds, and stays an authoring aid whose acceptance is the vet. No judgment sets a ruling or a condition (I62).

## 6. Decisions

- CU-D1 to CU-D11 as above.
- CU-D12 Amend ADR-087: its wins-if governs authored syntax only and gains a third trigger, "the trial reads a win for a non-control arm". Typing heads for every kind no longer waits on it.
- CU-D13 Build ADR-087's unbuilt rows first, generalized to every kind (REC-2 to REC-5).
- CU-D14 The work queue's facts (lanes, owned paths, ranges) become lane records in CUE. Issue labels stay the forge's view of the queue, and fathom writes none of them.
- CU-D15 The feature map of ADR-055, which has no code, becomes the `capability` kind.
- CU-D16 The friction log gains closing verdicts. Language ceiling notes move to reading records.

## 7. Risks

- CU-R1 Stage 1 is read as "markdown again". It is not: every head gets a schema, a vet at commit, derived conditions, queries and allocated ids, and every new data kind is CUE.
- CU-R2 The trial is small and its tasks are artificial. Mitigation: both harnesses, real records, a written prediction, and the rule that a tie changes nothing.
- CU-R3 Typed conditions stay `manual` forever. Mitigation: ReviewDue is computed for all of them from day one, and the count of `manual` is a tracked number.
- CU-R4 The lifecycle grows into a workflow engine. CU-K2 and CU-K5 hold it. A test asserts the evaluator has no side effect and no clock of its own (I75).
- CU-R5 Prose in CUE strings has its own traps (`\(` is interpolation, quotes need escapes). They are loud, not silent. The trial's deny log will count them.
- CU-R6 The kit declined CUE as a source (its decision 0021) and its skills read markdown records. Stage 1 keeps them working. A syntax change after the trial is a kit change too.

## 8. Verdict

**Type everything now. Build what was already accepted. Let a trial choose the syntax.**

The July verdict said reconciler, not machine, and it was right about the part that matters: nobody may write a state. What was missing is everything around that rule. Heads without schemas, conditions nobody can evaluate, refusals with no way back, a queue that left the repository. All of that can be fixed today without reversing a ruling, because ADR-087 already accepted most of it and no one built it. Data that has no prose is CUE from birth. For records that carry prose, the syntax question has been argued three times and measured never. This plan measures it once, with a rule that a tie changes nothing, and then stops arguing.
