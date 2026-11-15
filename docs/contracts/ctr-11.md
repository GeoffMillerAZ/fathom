# CTR-11: typed heads, conditions and record read port

Status: Approved 2026-11-15 under W2-3 and W4-2. Contract only; implementation proof pending.
Base: `468dc705813382d584926887b451cbea145c76ac`. Independent of other draft branches.
Owns only docs/contracts/ctr-11.md. No fixture or product file lands here.
Landing needs approved REC-1, CTR-4, ENG-2, LIB-1; implementations remain gated by
REC-2 -> REC-3 -> REC-4/REC-6 -> REC-7 -> REC-9, as typed-records rows state.
REC-1's approved validated-inventory law is referenced, never redefined here.

## Intent before design and settled choices

Read intent router, principles P2/P3/P17/P20/P22, scope compatibility/non-goals,
ADR-087 and ADR-094, typed-records CU-D1..9, parallel-lanes PL-D1..6. ADR-087 is
accepted. ADR-094/I70/I72/I75 remain proposals except the REC-1 choices settled by
W2-2. Markdown stays authored, Go computes, CUE validates. No derived status is
written. No transition history, scheduler, store import or new MCP tool. Existing
verify 0/1/2 and plan grammar remain. No syntax-trial or split trigger proven fired.
ADR-092 LL-M1 trigger is answered by W2-1. This batch imports no language package.
Free option: leaf port and current parsers preserve later kit extraction without
implementing it or paying for a new generalized document engine.

- D1 keep labelled heads and current containers until trial: CU-D2/D10/D11 and
  ADR-087 d1. Alternative CUE-frontmatter migration is deferred, not authorized.
- D2 seven named HEAD conditions, injected clock and no effects: REC-4,
  CU-D3/D4 and ADR-094 d3/4/7. Alternative expression language violates CU-K2.
- D3 one leaf record loader and permanent plan parser: REC-6, CU-D5/CU-K6.
  Alternative parallel ad-hoc parser entrypoints are explicitly removed by REC-M6.
- D4 separate vet from verify: CU-D7/CU-K7, REC-7. Alternative using verify exit 1
  for malformed records would violate the compatibility contract.
- D5 read verbs without new MCP: CU-D9/REC-9. Alternative new MCP tool is excluded.

API mechanics are settled by W3-4, and semantic field/evaluator choices cite
D1-D5 and reused current intent models. No generic API approval requested.
U2 settled by W4-2: exact serial schema/parser/CLI grants below, one owner per
turn and separate backfill. No backfill wildcard grant follows.

U3 settled by W3-2: implementing lane writes/runs real failing tests, independent
critic compares every named assertion, weaker tests are FIX. CTR-11 governing
clause amendment is a separate shared docs commit. No RED claim on this branch.

## Eight concrete head schemas (U1)

Proposed schema/records/heads.cue, package records. These are parsed head shapes,
not a replacement authored syntax. Body prose never moves into CUE. The title is
the heading text, id is the existing key. Empty optional lists encode absent
labels. Unknown labels/fields are unparsed diagnostics, never ignored input.

```cue
#Text: string & !=""
#Review: string & (=~"^[0-9]{4}-[0-9]{2}-[0-9]{2}$" | "never")
#Condition: close({
    label: "Done when" | "Promote when" | "Drop if" | "Revisit when" | "If false"
    kind: "date" | "record" | "reading" | "manual"
    text: #Text
    review_by: #Review
    target?: #Text
    want?: #Text
})
#DecisionHead: close({
    kind: "decision", id: =~"^ADR-[0-9]+$", title: #Text
    ruling: "proposed" | "accepted" | "rejected" | "deferred" | "withdrawn"
    supersedes: [...#Text], conditions: [...#Condition]
})
#InvariantHead: close({
    kind: "invariant", id: =~"^I[0-9]+$", statement: #Text
    enforced_by: [...#Text], conditions: [...#Condition]
})
#GoalHead: close({
    kind: "goal", id: =~"^G[0-9]+$", title: #Text
    bet: #Text, instrument: #Text, threshold: #Text
    review: #Review, conditions: [...#Condition]
})
#FutureHead: close({
    kind: "future", id: =~"^F[0-9]+$", title: #Text
    confidence: "likely" | "plausible" | "speculative"
    horizon: "next" | "later" | "someday"
    value?: close({level: "low" | "medium" | "high" | "very-high", why: #Text})
    basis?: close({kind: "measured" | "argued" | "guess", cite?: #Text})
    conditions: [...#Condition]
})
#QuestionHead: close({
    kind: "question", id: =~"^Q[0-9]+$", title: #Text
    conditions: [...#Condition]
})
#IncidentHead: close({
    kind: "incident", id: =~"^INC-[0-9]+$", title: #Text
    outcome: [...#Text], conditions: [...#Condition]
})
#ObservationHead: close({
    kind: "observation", id: =~"^O[0-9]+$", title: #Text
    verdict?: "fix" | "wontfix" | "duplicate" | "ceiling"
    outcome: [...#Text], conditions: [...#Condition]
})
#AskHead: close({
    kind: "ask", id: #Text, title: #Text, conditions: [...#Condition]
})
```

Go laws beyond CUE shape: real calendar dates; never only for a terminal refusal;
record/reading condition requires target + want; date/manual forbids target/want;
measured Future basis requires a citation per existing schema/intent #Citation.
No metric comparator added (REC-X2). No inferred ruling or condition from prose.
Existing unstructured records that cannot supply a required label are reported
with kind/path/line and counted, not rewritten to make a zero-unparsed claim.
Missing review_by stays an explicit REC-5 undated diagnostic until its one-time
pass; no clock or default date fabricated by ParseHead.

## Exact Go heads and loader port (U1)

Declarations are proposals, not compiling code on this branch. These live in
internal/records/heads.go. Current intent.Condition/Future/Goal/Invariant are reuse
inputs. Existing read-manifest records coverage. Current verify.Plan/Task stay the
plan model. Source positions use recordids.Location, no new coordinate model.

```go
type Identity struct { ID, Title string }
type ConditionSpec struct {
    Source intent.Condition // ID/Record/Label/Text/File/Line from existing parser
    Kind ConditionKind `json:"kind"`
    ReviewBy string `json:"review_by"`
    Target string `json:"target,omitempty"`
    Want string `json:"want,omitempty"`
}
type DecisionHead struct { Identity; Ruling Ruling; Supersedes []string; Conditions []ConditionSpec }
type InvariantHead struct { ID, Statement string; EnforcedBy []string; Conditions []ConditionSpec }
type GoalHead struct { Identity; Bet, Instrument, Threshold, Review string; Conditions []ConditionSpec }
type FutureHead struct { Future intent.Future; Conditions []ConditionSpec }
type QuestionHead struct { Identity; Conditions []ConditionSpec }
type IncidentHead struct { Identity; Outcome []string; Conditions []ConditionSpec }
type ObservationHead struct { Identity; Verdict FrictionVerdict; Outcome []string; Conditions []ConditionSpec }
type AskHead struct { Identity; Conditions []ConditionSpec }
// Closed tagged union: exactly one matching head pointer is nonnil.
type Head struct {
    Kind RecordKind `json:"kind"`
    Decision *DecisionHead; Invariant *InvariantHead; Goal *GoalHead
    Future *FutureHead; Question *QuestionHead; Incident *IncidentHead
    Observation *ObservationHead; Ask *AskHead
}
type Record struct { Head Head; At recordids.Location }
type ParseResult struct { Records []Record; Unparsed []intent.Unparsed }
func ParseHead(kind RecordKind, path string, source []byte) (ParseResult, error)
type LoadResult struct {
    Records []Record
    Plans []verify.Plan
    Unparsed []intent.Unparsed
    Coverage readmanifest.Manifest
}
func LoadRecords(root string) (LoadResult, error)
```

JSON heads flatten only the matching pointer to CUE snake_case fields. Never
marshal eight optional variants. Future adapter reuses all existing Future fields
but replaces Conditions with typed ConditionSpec in the flattened encoding. Invariant
adapter reuses intent parsing of statement/enforcement claims, never authors Rung.
Goal adapter reuses LoadBets parsing for bet/instrument/threshold/review. Existing
condition labels and family routing are reused from intent.Load, not rescanned with
an unrelated regexp catalog. Ask id is its existing authored key, no allocated ASK
family introduced by this batch. No fields for future syntax or scheduler identity.

Port direction: records imports the existing intent parser, verify parser and
read-manifest; these packages never import records. Fusion's new records.go adapts
LoadResult. Architect serially redirects existing consumers only after parity tests;
REC-M6 first enumerates exact bypass sites, then adds a gone row. This draft grants
no deletion based on an invented bypass list. Plan markdown remains lossless for
its row model, not for all prose. Unreadable member is coverage-unreadable, never
an empty healthy corpus. Record sort is kind/id/path/line, diagnostic sort path/line.

## Named condition evaluator (U1)

```go
type DatedReview struct { RecordID string `json:"record_id"`; Date string `json:"date"`; Verdict ReviewVerdict `json:"verdict"`; At recordids.Location `json:"at"` }
type ConditionFacts struct {
    Records []Record
    Coverage readmanifest.Manifest
    Links map[string]bool
    Enforcement map[string]intent.Rung
    Readings map[string]string
    Bets []intent.GoalBet
    Reviews []DatedReview
}
type TargetConditionResult struct { Target string `json:"target"`; State ConditionState `json:"state"`; Reason string `json:"reason"` }
type ConditionResult struct { ID string `json:"id"`; Name ConditionName `json:"name"`; Record string `json:"record"`; State ConditionState `json:"state"`; Reason string `json:"reason"`; Targets []TargetConditionResult `json:"targets"` }
type ConditionTarget struct { RecordID string `json:"record_id,omitempty"`; ExpiryID string `json:"expiry_id,omitempty"` }
func EvaluateCondition(name ConditionName, target ConditionTarget, facts ConditionFacts,
    now time.Time) (ConditionResult, error)
func ObservationVerdict(head ObservationHead, facts ConditionFacts) (FrictionVerdict, error)
```

State closes over holds/violated/unknown. Map absence means unknown, not false.
Selection has two disjoint addresses. Expiry checks use target.ExpiryID,
the existing intent.Condition.ID (record#ordinal). Head checks use target.RecordID
and the named check; exactly one selector field is set. Missing/duplicate record
or expiry id returns a typed RefusalError, never a first-match fallback.

Head-addressed SupersederResolves reads DecisionHead.Supersedes; OutcomeResolves
reads IncidentHead/ObservationHead.Outcome; EnforcementHolds reads
InvariantHead.EnforcedBy. They never require an expiry condition. LinkResolves at
head aggregates these explicitly typed links; ReviewRecorded is head-addressed
for GoalHead/GoalBet. ReviewDue accepts either a goal head (bet date) or exact expiry
(review_by). ConditionFired is expiry-only. Invalid name/address combination
refuses. No synthetic Revisit/Done condition is fabricated for a head field.

A head check reads every target in its named field and returns target-level
results as well as aggregate. Head result ID is a derived display label
record#head:CheckName, never an authored expiry ID or a record-registry key.
Expiry result ID remains Source.ID. Extend ConditionResult with
Targets []TargetConditionResult, where TargetConditionResult has Target string,
State ConditionState, Reason string (explicit target/state/reason JSON tags).
Exactly duplicated field targets fail vet; sorting targets is deterministic.
Empty field is not applicable (holds with explicit reason), except a closed
incident claiming resolution without an outcome is violated under complete coverage.
Per record, evaluate all applicable head checks plus every (expiry ID, check)
pair, retaining all results. Aggregate is violated if any definite violation,
else unknown if any unknown, else holds. Unknowns are never dropped from totals.

Coverage is the existing manifest carried by LoadRecords, not a boolean authored
by a record. Any absent target is violated only when its owning routed member was
successfully read and is complete at HEAD. Unreadable/skipped/unknown target member
or unmapped ownership is unknown with coverage reason. Unrelated unreadable members
do not invalidate a fully read, specifically resolved target. This applies to
supersedes/outcomes/links/reviews/readings and enforcement facts alike.

SupersederResolves: every head supersedes target resolves to one record under
complete coverage. OutcomeResolves: every head outcome target resolves similarly. Prose with no typed target stays unknown. ReviewRecorded
uses its enclosing goal's accepted GoalBet, not the existence of any note. If the
bet is refused/missing or its review date invalid, return unknown with reason
(project-understanding §7 explicitly refuses the bet). If injected now is before
that date, the implication holds with “review not due”. At/after date, require an
explicit keep/kill line for that goal whose parsed Date >= bet.Review and <= now's
UTC date. Arbitrary notes, undated notes, old verdicts and future-dated verdicts do
not satisfy it. Missing timely verdict is violated only with complete goal coverage.
No other verdict counts. A timely keep/kill means recorded, not proof the goal won.
ReviewDue is date reached with no such timely review, never is not due. Changing
ReviewBy does not rewrite the original GoalBet review boundary used by this check.

LinkResolves checks exact typed target as above. EnforcementHolds checks authored
enforcement references with a checked gate/proven rung and complete coverage;
absent evidence is unknown. ConditionFired evaluates the exact selected expiry only:
date against injected now; record against explicit ruling; reading against exact
verdict; manual unknown until due then ReviewDue, never guessed true. No LLM/prose
predicate, internal time.Now, filesystem, git, store, provider or process. Unknown
condition name errors. Record-link cycles, including self-cycle, fail vet. All
result states are derived, never accepted as authored head fields.

Proposed TestReviewRecordedDatedGoal cases in conditions_test.go: old keep before
bet date fails; arbitrary timely note fails; timely keep and timely kill each
hold; future due date holds with not-due reason; refused bet is unknown; missing
review with unreadable goal is unknown. TestConditionIdentityAndCoverage creates
two different authored conditions on one record, selects each unique Source.ID,
asserts independent results, rejects duplicate ids, and makes a target member
unreadable to require unknown instead of violated or healthy. Aggregate retains
both conditions and named unknown reasons. Commands:
`go test -race ./internal/records -run '^TestReviewRecordedDatedGoal$' -count=1`
and `go test -race ./internal/records -run '^TestConditionIdentityAndCoverage$' -count=1`.
These are proposed acceptance tests, not executed evidence.

## CLI JSON contract and schema handoffs

`cmdRecVet(args []string, stdout, stderr io.Writer) int`, `cmdRecQuery(...) int`
and `cmdRecShow(...) int` follow existing command convention in new rec_*.go.
`rec query --kind KIND [--ruling VALUE] [--condition NAME] [--link ID] [--lane SLUG]
[--wave N] [--text TEXT] --json` selects LoadRecords plus separate lane/plan metadata.
Do not add lane/wave fields to every prose head. Invalid combination returns 2.
`rec show ID --json` errors on ambiguity, never picks first duplicate.
Envelope: `{"schema":"fathom.records/v1","records":[],"unparsed":[]}`;
each record is `{"head":{...},"path":"...","line":1,"conditions":[]}`.
Condition entries keys id/name/record/state/reason. Exact empty result stays arrays.
Vet JSON uses `{"schema":"fathom.rec-vet/v1","violations":[],"unparsed":[]}`.
Exit 0 clean, 1 schema/link/id/cycle violation, 2 unreadable/unavailable evidence.
`why` consumes existing typed plan/ADR links and verify receipts (ADR-086), not
prose links promoted into proof. No mutation. CLI uses existing flag and diagnostic
conventions; app-kit terminal guidance checked, no runtime/dependency migration.

Proposed fixture namespace, exact files:
`schema/records/testdata/heads/{decision,invariant,goal,future,question,incident,observation,ask}-{valid,invalid}.cue`.
Positive fixtures instantiate their schema, negative fixtures are commented and
Go tests compile their payloads expecting rejection. Named negative mutations per
kind: missing id, unknown derived phase, wrong kind, bad enum, missing required
head label. All eight reject `phase` and authored condition verdicts. No active
invalid CUE enters cue-vet. REC-2 receives exclusive heads namespace after REC-1
releases testdata/valid.cue and invalid.cue. No broad directory co-ownership.
CTR-13 owns card closure shape proposal, #299 owns future data.cue/card fixture
namespace only after this head turn, never modifies these eight head fixtures.

## Exact issue ownership snapshot and serial transfer

These are the current issue allowlists, not grants to this draft author. Existing
files remain with the architect until a named serial handoff. Directory grants
are reproduced as authored and must be narrowed to the fixture files below before
dispatch. Tests in a contract batch transfer once, at its landed commit, to the
consumer. No concurrent shared-file owner.

### #294 Record head schema (REC-2)

- `schema/records/heads.cue` (new).
- `schema/records/testdata/` (new).

### #293 Record head parser (REC-3, REC-M1)

- `internal/records/heads.go` (new).
- `internal/records/heads_test.go` (new).
- `docs/bench/readings/typed-records/coverage.md` (new).

### #292 Record conditions (REC-4)

- `internal/records/conditions.go` (new).
- `internal/records/conditions_test.go` (new).

### #295 Record review dates (REC-5)

- `internal/records/reviewdates.go` (new).
- `internal/records/reviewdates_test.go` (new).
- `docs/bench/readings/typed-records/review-by-pass.md` (new).

### #318 Record port (REC-6, REC-M6)

- `internal/records/load.go` (new).
- `internal/records/load_test.go` (new).
- `internal/fusion/records.go` (new).
- `docs/bench/readings/typed-records/port-bypass.md` (new).

### #319 Record vet (REC-7, REC-M3)

- `cmd/fathom/rec_vet.go` (new).
- `cmd/fathom/rec_vet_test.go` (new).
- `Taskfile.yml` (exists).
- `lefthook.yml` (exists).
- `docs/bench/readings/typed-records/gate-cost.md` (new).

### #317 Plan keys (REC-8)

- `internal/verify/annotation_keys.go` (new).
- `internal/verify/annotation_keys_test.go` (new).
- `schema/plan/schema.cue` (exists).
- `cmd/fathom/observatory_progress.go` (new).

### #299 Bench cards (BEN-3, REC-10)

- `internal/capability/card_closure.go` (new).
- `internal/capability/card_closure_test.go` (new).
- `schema/records/data.cue` (new).
- `schema/records/testdata/` (new).
- `calibration/capabilities/` (new).
- `calibration/standards/` (new).

### #305 Friction verdicts (REC-11)

- `internal/records/observation_verdict.go` (new).
- `internal/records/observation_verdict_test.go` (new).
- `.fathom/observations/` (exists).

## Explicit integration reservations (U2)

These are the complete proposed additional shared-file grants. Prior owners must
hand back a committed base before the next turn. Unlisted files stay unowned.

| Exact paths | Exclusive serial owner / handoff |
|---|---|
| schema/records/schema.cue, schema/records/lane.cue, schema/records/testdata/valid.cue, schema/records/testdata/invalid.cue | REC-1 -> #294 schema import/read-only consumer; no lane schema rewrite |
| schema/records/heads.cue and the 16 exact heads fixture files above | #294 -> #293 read-only -> #292 read-only |
| internal/records/heads.go, internal/records/heads_test.go | #293 only, with CTR-11 TestLaneRecordHeadParser transferred at contract landing |
| internal/records/conditions.go, internal/records/conditions_test.go, internal/records/reviewdates_test.go | #292 evaluator -> #295 dates (test file addition explicitly granted) |
| internal/records/load.go, internal/records/load_test.go, internal/fusion/records.go | #318 only; existing call-site redirection requires REC-M6 exact inventory first |
| cmd/fathom/rec_vet.go, cmd/fathom/rec_vet_test.go, Taskfile.yml, lefthook.yml | #319 one architect integration turn, after REC-1 and #164 command baseline |
| cmd/fathom/rec_query.go, cmd/fathom/rec_query_test.go, cmd/fathom/rec_show.go | record-verbs lane, explicit added rec_show.go grant |
| internal/verify/annotation_keys.go, internal/verify/annotation_keys_test.go, schema/plan/schema.cue, internal/verify/markdown.go, internal/verify/verify.go | #317 architect schema/parser turn only |
| cmd/fathom/plans_facts.go, cmd/fathom/plans_facts_test.go, internal/console/plans.go, internal/console/plans_test.go | #317 actual planTaskFact/PlanRowFact consumer turn; replace IsOpTitle use with typed Gate field |
| schema/records/data.cue, schema/records/testdata/cards/valid.cue, schema/records/testdata/cards/invalid.cue | #299 after head-schema handoff; CTR-13 supplies card shape |

REC-8 backfill is a separate architect-owned batch after parser/schema parity.
Generate its exact plan-path manifest from matching titles and freeze it before
any edit; this draft cannot grant “about 400 rows” as an exact allowlist. Actual current reader is internal/console/plans.go::IsOpTitle, called by
cmd/fathom/plans_facts.go::planTaskFact. #317 explicitly owns these four files,
not speculative project.go/plans_lines.go edits. Add Task/PlanRowFact fields
Lane, Wave, Feature, Capability strings and Gate bool (JSON lane/wave/feature/
capability/gate, omitempty for compatibility); parser accepts gate true/false only.
`planTaskFact` assigns Op from Task.Gate, never title prefix, and copies the other
four fields. Existing Op JSON presentation stays as legacy output alias for Gate,
not a second authored source. Remove IsOpTitle after its callers are gone. Titles
may retain literal “(op)” before backfill but cannot affect computed Op.
Proposed cmd/fathom/plans_facts_test.go::TestPlanTaskFactUsesGateKey parses one
plain title with gate:true and one “(op)” title with gate:false, calls the real
planTaskFact and asserts Op=true/false plus the other key fields survive. Feed
those facts into existing console RollupPlan/ProofBarOf projections and assert measurement
counts follow Gate, not title spelling. internal/console/plans_test.go adds
TestPlanProjectionUsesTypedGate with the same crossed fixture. Commands:
`go test -race ./cmd/fathom -run '^TestPlanTaskFactUsesGateKey$' -count=1` and
`go test -race ./internal/console -run '^TestPlanProjectionUsesTypedGate$' -count=1`.
The exact backfill path list remains separately frozen, not required to make this
consumer proof concrete. No facade-only proof substitutes for planTaskFact.
REC-M3 timing uses the full trusted LoadRecords corpus, cold process and one changed
fixture; five trials, median, same fixed fixture clock, elapsed monotonic duration.
Keep authored bars <2 s cold / <200 ms changed. W4-2 requires publishing EVERY
trial with fixture/binary/source identity and units, plus the five-trial median.
Flag any single cold trial >4 s or changed-file trial >400 ms beside the verdict
(the unchanged corresponding bar times two), even when median passes. Missing
trial makes timing incomplete; never drop an outlier. Test presence is not timing proof.

## Proposed test names, assertions and commands

| Consumer | Exact proof handoff |
|---|---|
| #294 REC-2 | proposed internal/records/lane_contract_test.go::TestLaneHeadSchemas compiles all eight valid/invalid pairs against schema bytes; missing/wrong/derived fields rejected, positive fields round-trip. `go test -race ./internal/records -run '^TestLaneHeadSchemas$' -count=1` |
| #293 REC-3 | internal/records/heads_test.go::TestLaneRecordHeadParser parses labelled examples of all eight kinds, counts every unparsed path/line, preserves current Future/Goal/Invariant fields and family members. `go test -race ./internal/records -run '^TestLaneRecordHeadParser$' -count=1` |
| #292 REC-4 | internal/records/conditions_test.go::TestLaneRecordConditions has holds/violated/unknown per seven names, missing evidence never healthy, cycle rejected, fixed date, no side effects. `go test -race ./internal/records -run '^TestLaneRecordConditions$' -count=1` |
| #295 REC-5 | internal/records/reviewdates_test.go::TestLaneRecordReviewDates date boundary, invalid calendar, missing date and never-only-refusal cases. `go test -race ./internal/records -run '^TestLaneRecordReviewDates$' -count=1` |
| #318 REC-6 | internal/records/load_test.go::TestLaneRecordPort mixed Markdown/CUE/plan corpus, unreadable family member, no store import, deterministic output and old-reader parity. `go test -race ./internal/records -run '^TestLaneRecordPort$' -count=1` |
| #319 REC-7 | cmd/fathom/rec_vet_test.go::TestLaneRecordVet malformed shape, unresolved link, duplicate incl observation, cycle, derived field and unreadable corpus; exact 0/1/2; uses REC-1 shared checker. `go test -race ./cmd/fathom -run '^TestLaneRecordVet$' -count=1` |
| REC-9 | cmd/fathom/rec_query_test.go::TestLaneRecordQuery query/show JSON goldens, unknown id and duplicate ambiguity, no writes, typed links only. `go test -race ./cmd/fathom -run '^TestLaneRecordQuery$' -count=1` |
| #317 REC-8 | internal/verify/annotation_keys_test.go::TestLanePlanKeys all five keys accepted by closed schema/parser and survive round-trip; old v2 unchanged; title-only metadata removed only in explicit backfill. `go test -race ./internal/verify -run '^TestLanePlanKeys$' -count=1` |
| #299 BEN-3 | internal/capability/card_closure_test.go::TestLaneBenchCards follows CTR-13 card contract, missing shipped surface card fails; no duplicate head/card schema. `go test -race ./internal/capability -run '^TestLaneBenchCards$' -count=1` |
| #305 REC-11 | internal/records/observation_verdict_test.go::TestLaneFrictionVerdicts exact four verdicts, unresolved fix/duplicate links unknown, ceiling does not imply fixed. `go test -race ./internal/records -run '^TestLaneFrictionVerdicts$' -count=1` |

Batch primary: internal/records/lane_contract_test.go::TestLaneContractsRecords;
`go test -race ./internal/records -run '^TestLaneContractsRecords$' -count=1`.
CLI aggregate `cmd/fathom/rec_contract_test.go` asserts envelope/exit/zero-write parity.
W3-2 assigns actual failing execution to each implementing lane. Missing test/package alone is absence, not observed
RED. Fixtures above and names are proposals, not present/executed on this branch.

## Evidence limits and decision models

Existing source reuse: internal/intent/corpus.go/parse.go/family.go/futures.go/
bets.go/triggers.go, schema/intent/schema.cue, verify.ParseMarkdownPlan/
ParseAnnotation and recordids.Check. intent.Family/coverage prevents unreadable
members disappearing. Current capability.Registry is switch policy, not a proof-card
schema; card closure must not relabel switch detection as fact proof.
Only draft diff check and commit hook run here. No CUE, RED, race, timing or full
gate result claimed. CTR-11 and consumers remain unchecked. No bypass deletion,
backfill, CLI registration or gate edit is implemented.
Decision models: none in these ports. Prose condition-kind suggestions from CU's
separate candidate site are unbuilt and cannot set facts/rulings. Provider none is
fully deterministic. Manual records remain counted unparsed rather than guessed.

## Head-check assertions and W3-4 exported contract rules

Proposed TestRecordHeadChecksWithoutExpiry in internal/records/conditions_test.go
creates: a superseding decision with Supersedes and Conditions empty; a closed
incident with Outcome and Conditions empty; an invariant with EnforcedBy and
Conditions empty. Invoke each named head check with RecordID and no ExpiryID.
Resolvable complete targets hold; missing target in a completely read member
violates; unreadable/sibling/unmapped target ownership is unknown with reason.
Every target is retained in Targets. Two targets with one unknown cannot become
healthy. A missing head does not manufacture a Revisit condition. Preserve
TestConditionIdentityAndCoverage with two distinct expiries on one record, and
reject using a head-only name with an expiry-only address or vice versa.
Command: go test -race ./internal/records -run '^TestRecordHeadChecksWithoutExpiry$'
-count=1 (one shell line). These are future real RED assertions under W3-2.

Closed vocabularies for the new exported declarations, W3-4:
~~~go
type RecordKind string
const (
    KindDecision RecordKind = "decision"
    KindInvariant RecordKind = "invariant"
    KindGoal RecordKind = "goal"
    KindFuture RecordKind = "future"
    KindQuestion RecordKind = "question"
    KindIncident RecordKind = "incident"
    KindObservation RecordKind = "observation"
    KindAsk RecordKind = "ask"
)
type ConditionKind string
const (
    ConditionDate ConditionKind = "date"
    ConditionRecord ConditionKind = "record"
    ConditionReading ConditionKind = "reading"
    ConditionManual ConditionKind = "manual"
)
type ConditionName string
const (
    CheckSupersederResolves ConditionName = "SupersederResolves"
    CheckOutcomeResolves ConditionName = "OutcomeResolves"
    CheckReviewRecorded ConditionName = "ReviewRecorded"
    CheckReviewDue ConditionName = "ReviewDue"
    CheckLinkResolves ConditionName = "LinkResolves"
    CheckEnforcementHolds ConditionName = "EnforcementHolds"
    CheckConditionFired ConditionName = "ConditionFired"
)
type ConditionState string
const (
    ConditionHolds ConditionState = "holds"
    ConditionViolated ConditionState = "violated"
    ConditionUnknown ConditionState = "unknown"
)
type Ruling string
const (
    RulingProposed Ruling = "proposed"
    RulingAccepted Ruling = "accepted"
    RulingRejected Ruling = "rejected"
    RulingDeferred Ruling = "deferred"
    RulingWithdrawn Ruling = "withdrawn"
)
type FrictionVerdict string
const (
    VerdictFix FrictionVerdict = "fix"
    VerdictWontFix FrictionVerdict = "wontfix"
    VerdictDuplicate FrictionVerdict = "duplicate"
    VerdictCeiling FrictionVerdict = "ceiling"
)
type ReviewVerdict string
const (
    ReviewKeep ReviewVerdict = "keep"
    ReviewKill ReviewVerdict = "kill"
)
type RefusalError struct { Path string `json:"path"`; Line int `json:"line"`; Reason string `json:"reason"` }
func (e *RefusalError) Error() string
type LimitError struct { Limit int64 `json:"limit"`; Observed int64 `json:"observed"` }
func (e *LimitError) Error() string
~~~

All compact declaration fields above acquire explicit lower_snake_case JSON tags:
Identity id/title, Source source, Head's eight pointers decision/invariant/goal/
future/question/incident/observation/ask (omitempty), Record head/at, LoadResult
records/plans/unparsed/coverage, ConditionFacts records/coverage/links/enforcement/
readings/bets/reviews. Per-head field tags exactly match the eight CUE schemas;
source-only positions are json:- when serializing parsed authored head data.
This is an explicit field-to-wire map, not Go's default capitalized encoding.
No bare string for a new closed vocabulary. Existing intent types remain additive
reuse inputs and are flattened through the declared adapter, never retyped.

Pure ParseHead/EvaluateCondition/ObservationVerdict have no context. LoadRecords
reads I/O and may accept context.Context as first argument; freeze that signature
as LoadRecords(ctx context.Context, root string) (LoadResult, error). Shape/
selection refusals are RefusalError, bounded-input failures LimitError, errors.As
works. Every exported result slice is nonnil recursively, including empty targets/
records/plans/unparsed/conditions/outcome/enforced_by; JSON [] never null.
No future fields, package on the language door, public type removal or default
value exposure. This is the W3-4 rule applied to these concrete current ports.

Proposed TestRecordContractsAPIHouseRules in internal/records/lane_contract_test.go
asserts constants/types, exact head/condition/envelope JSON names, empty slices [],
refusal/limit errors.As, context only loader I/O, and unchanged legacy parser
inputs/verify codes. Run go test -race ./internal/records -run
'^TestRecordContractsAPIHouseRules$' -count=1 as one line. Independent lane critic
compares every listed assertion under W3-2. No executed RED or green claim here.

## Approved W4-2 timing and grants

CTR-11/U2 one-owner-per-turn grants and separate backfill are settled by W4-2.
REC-M3 uses the trusted complete corpus and five trials per cold/changed-file
condition. Publish all ten individual durations and the two medians with explicit
units, binary/corpus identity and invocation. Every trial is retained. A trial
strictly above twice its own bar is flagged beside the verdict, never hidden by
the median. Exactly twice is not “over”. Median bars remain <2 s and <200 ms.
No new aggregation choice is pending and no timing was measured by this author.

Proposed TestRecordVetTimingReport in cmd/fathom/rec_vet_test.go: five complete
cold durations and five complete changed-file durations produce exact median and
all trial rows; one cold 4.001 s trial and changed 400.001 ms trial each emit flag
beside otherwise passing median; exact 4 s/400 ms do not emit over-twice flag;
missing trial is incomplete, cannot drop outlier or report five-trial PASS.
Retain original bar and record every raw value. Command go test -race ./cmd/fathom
-run '^TestRecordVetTimingReport$' -count=1 (one line), proposed/unrun. Implementing
critic compares each assertion under W3-2. No implementation or timing proof is
claimed. Contract approval does not activate reservations or satisfy prerequisites.
