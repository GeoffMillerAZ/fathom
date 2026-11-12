# Typed records: heads with schemas, conditions at HEAD, ids allocated, a trial for syntax

Status: Proposed 2026-11-12 (architect ruling, owner review pending).

Grammar v2 (`docs/plans/README.md`). Prefix: REC- (REC-n work, REC-T the syntax trial, REC-Mn readings). Design: `docs/design/typed-records.md` (CU- codes are its). Decision: ADR-094. Invariants: I70, I72, I75. Owner question OQ10 (the trial's spend cap) lives in `docs/vision/46-proof-at-the-moment-of-change.md`, default if unanswered: no paid spend, and the trial does not run.

Every row below is open. Symbols and packages name what the change will create. A row flips only through `fathom verify --service fathom --plan docs/plans/typed-records.md`. Readings live under `docs/bench/readings/typed-records/`, one file per row, each opening with a verdict line. `fathom verify` keeps its exit codes (CU-K7). The markdown plan parser stays (CU-K6).

## Phase 1: ids, schemas, parse, conditions

- [ ] Lane kind, `fathom rec new` and id allocation from a lane range on a standalone schema, as an extension of the existing record-id check: observation ids, lane ranges, and the clause that a record key is defined in one file inside its lane's owned paths. Main thread, wave 0a {id: REC-1, symbol: AllocateRecordID, package: internal/records}
- [ ] Head schemas for every record kind (lane `record-heads` once REC-8 adds the key), each with valid and invalid testdata pairs (CU-D1, CU-D2) {id: REC-2, oracle: lexical, path: schema/records/heads.cue, pattern: "^#(Decision|Invariant|Goal|Future|Question|Incident|Observation|Ask)Head:", min: 8, needs: "REC-1"}
- [ ] Typed parse of the labelled heads people write today, for every kind (ADR-087's rows, generalized). Proof: unparsed count per kind reads zero {id: REC-3, symbol: ParseHead, package: internal/records, needs: "REC-2"}
- [ ] Named conditions at HEAD: SupersederResolves, OutcomeResolves, ReviewRecorded, ReviewDue, LinkResolves, EnforcementHolds, ConditionFired, with a fixture per condition. PU-M4's bar applies to the first three {id: REC-4, symbol: EvaluateCondition, package: internal/records, needs: "REC-3"}
- [ ] One-time tool pass that gives every condition a `review_by`, or an explicit "never" for a refusal. Zero undated conditions afterwards {id: REC-5, oracle: lexical, path: docs/bench/readings/typed-records/review-by-pass.md, pattern: "^REC-5 undated: 0", needs: "REC-4"}

## Phase 2: one loader, vet, plan keys, reading verbs

- [ ] `internal/records` as a leaf behind the dialect port, no store import. Consumers route through it {id: REC-6, symbol: LoadRecords, package: internal/records, needs: "REC-3"}
- [ ] (op) Direct parse calls that bypass the port are gone: a `gone:` pattern row is written once REC-6 names the bypassing call sites, and this reading lists them until then {id: REC-M6, oracle: lexical, path: docs/bench/readings/typed-records/port-bypass.md, pattern: "^REC-M6 bypass calls: 0", needs: "REC-6"}
- [ ] `fathom rec vet` as its own step of pre-commit and the gate: shape per kind, links resolve, ids defined once, no derived field authored (I70), no condition cycle. Verify's exit codes unchanged {id: REC-7, symbol: cmdRecVet, package: cmd/fathom, needs: "REC-6"}
- [ ] Plan grammar keys `lane`, `wave`, `feature`, `gate` and `capability`: the closed plan schema gains them, the console stops reading `(op)` from titles, and about 400 titles are backfilled by tool in one main-thread batch. Zero rows keep a field in the title {id: REC-8, symbol: ParseAnnotationKeys, package: internal/verify, needs: "REC-6"}
- [ ] `fathom rec query`, `fathom rec show` and `fathom why` over typed links, with `--json` and goldens (CU-D9). No new MCP tool {id: REC-9, symbol: cmdRecQuery, package: cmd/fathom, needs: "REC-7"}
- [ ] Data kinds in CUE (lane `record-data` once REC-8 adds the key): capabilities, standards, the named-query catalog, reading heads, the knob map, with valid and invalid pairs {id: REC-10, oracle: lexical, path: schema/records/data.cue, pattern: "^#(Lane|Capability|Standard|NamedQuery|ReadingHead|Knob)[A-Za-z]*:", min: 6, needs: "REC-2"}
- [ ] Closing verdicts on the friction log. Language ceiling notes move to reading records. Proof: open share by age {id: REC-11, symbol: ObservationVerdict, package: internal/records, needs: "REC-6"}

## Phase 3: the syntax trial and its follow-up

- [ ] (op) The syntax trial (CU-D10): arms A0 to A3, 30 records per kind, a deny log at the vet, the control run twice for the noise floor, the written prediction recorded before data. Starts when the 2026-11-17 readings are read. Needs the owner's spend cap (OQ10). Does not wait on the engine readings {id: REC-T, oracle: lexical, path: docs/bench/readings/typed-records/syntax-trial.md, pattern: "^REC-T verdict:", needs: "REC-7"}
- [ ] Move heads to the winning syntax, kind by kind, only if a non-control arm wins. Per-kind parity and consumer tests {id: REC-12, symbol: MigrateHead, package: internal/records, needs: "REC-T"}

## Readings (CU-M)

- [ ] (op) REC-M1 typed coverage: unparsed heads read zero for every kind, with file and line when not. If a kind cannot reach zero without rewriting its records, it keeps its labelled lines and a counted unparsed share {id: REC-M1, oracle: lexical, path: docs/bench/readings/typed-records/coverage.md, pattern: "^REC-M1 verdict:", needs: "REC-3"}
- [ ] (op) REC-M2 the trial's result under CU-D10's decision rule {id: REC-M2, oracle: lexical, path: docs/bench/readings/typed-records/trial-result.md, pattern: "^REC-M2 verdict:", needs: "REC-T"}
- [ ] (op) REC-M3 gate cost: vet of the full corpus under 2 seconds cold and under 200 milliseconds for one changed file {id: REC-M3, oracle: lexical, path: docs/bench/readings/typed-records/gate-cost.md, pattern: "^REC-M3 verdict:", needs: "REC-7"}
- [ ] (op) REC-M4 size and reading cost: record size and tokens read per answered question in each arm, answering ADR-074's stated costs with numbers {id: REC-M4, oracle: lexical, path: docs/bench/readings/typed-records/size-and-reading.md, pattern: "^REC-M4 verdict:", needs: "REC-T"}
- [ ] (op) REC-M5 collisions on record paths: the overlap replay script restricted to record files {id: REC-M5, oracle: lexical, path: docs/bench/readings/typed-records/collisions.md, pattern: "^REC-M5 verdict:"}

## Refused for now

- [-] Split the decision file into one file per record {id: REC-X1, disposition: deferred, reason: "its trigger is conflict share, the reading is unread, and a split costs 109 Go readers, 21 lexical rows and the kit's own skills", trigger: "the post-fix conflict-share reading shows record-file conflicts as a material share of merges"}
- [-] A `metric` condition kind with a fixed comparator {id: REC-X2, disposition: deferred, reason: "added only when a reading needs it", trigger: "a reading row needs a numeric comparator that `reading` cannot express"}
- [-] Edge judgment between git endpoints for record statuses {id: REC-X3, disposition: deferred, reason: "every measured gap is checkable at HEAD (ADR-087 decision 5), fathom writes no transition history", trigger: "PU-M5 reads 2 or more illegal edges in 12 months"}
