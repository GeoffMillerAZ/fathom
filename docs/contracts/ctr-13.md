# CTR-13: cell ledger, readiness, cards, subjects and attested strength

Status: DRAFT W1-5/#262. Contract only, no product or bench implementation.
Base: `468dc705813382d584926887b451cbea145c76ac`. Independent draft branch.
Owns docs/contracts/ctr-13.md only. All fixtures below are proposed, not landed.
Landing prerequisites REC-1, CTR-4, ENG-2, LIB-1 remain unclaimed. BEN-1 precedes
readiness/cards/subjects, as proof-program says. CTR-11 and CTR-12 dependencies
below are interfaces to approved versions, not accidental branch stacking.

## Intent and settled choices before design

Read intent router, P2/P3/P7/P12/P20/P22, scope compatibility/non-goals,
ADR-032/085/101/109/110, proof-program BN-D1..6, bench-prep C1..7/D27,
wiring-proof WP-D1/2 and R2-1/BX-40 in #262. Newer ADRs are proposals, applied
as the rows' design context, not evidence a paid gate passed. No model primary
oracle, no silent missing input, no scheduler or product test runner. W2-1 answers
the fired LL-M1 trigger; this batch does not touch the language door. No powered
null/concept or environment-trigger reading is claimed. Keep external subject
truth and repository splits. Preserve borrowed control cells when their actual
inputs match, not speculative cross-version migration or new statistics library.

- D1 one bench command outside product, one ledger, freeze old driver: BEN-1,
  ADR-101 and proof-program consolidation. Alternative parallel live drivers
  cannot satisfy the row. #266's cell-key-only file list is insufficient.
- D2 six readiness controls before paid cells: BEN-2/BN-D12. Alternative a green
  ordinary unit suite is not the six-harness prerequisite.
- D3 fact/delivery/outcome ordered proofs and card closure: BEN-3/BN-D1/D2.
  Alternative switch-registry detection as proof confuses capability configuration
  with demonstrated fact correctness.
- D4 independent subject oracles, development/held-out splits, seven families:
  BEN-5/BN-D4 and ADR-085/109. Alternative own-fleet smoke in headline counts is
  excluded. ADR-109's explicit scale classes replace only C2's upper ceiling.
- D5 scripted preflight W0-W3/W6 precedes agent. W4/W5 non-use is an outcome,
  never exclusion: ADR-110/WP-D1/2, #262 R2-1, BX-40. This supersedes BN-D6's
  earlier “surface called at least once” validity wording.
- D6 strength uses attested results, no execution: CHG-4/ADR-100 d4 and current
  certifies marker ladder. Alternative authored strength string alone is not proof.

Unsettled U1: approve exact cell/ledger/readiness/card/admission/strength types and
fixtures below. Alternative split each named boundary into its own approval item.
Unsettled U2: approve exact BEN-1 consolidation slices/command ownership and serial
schema/certification grants below. Alternative narrow #266 to cell-key subtask and
keep BEN-1 unclaimed until separate consolidation lanes complete it.
Unsettled U3: implementing-lane RED with CTR-13 row amendment, recommended.
Alternative tagged batch RED with an explicit green-default command. REC-1 W2-2
D5 does not override CTR-13's current batch-owned runnable failing-proof bar.

## Stable cell key and unified ledger (U1)

Reuse existing contextbench.CellInputs and CellKey/Validate, not a second list
of those eight invariants. Reuse scenariobench task/oracle/session models rather
than another task schema. Additional key inputs below each have a current named
consumer: exact subject commit, tool schema/profile, preregistered task family and
criterion, and trial. No fields for future providers or scheduler state.

```go
// internal/benchledger/cellkey.go
type CellInputs struct {
    Base contextbench.CellInputs
    SubjectCommit, ToolSchemaSHA256, ProfileSHA256, Family, CriterionID string
    Trial int
}
func CellKey(in CellInputs) (string, error)
type CellRow struct {
    Schema string `json:"schema"`
    Key string `json:"key"`
    Inputs CellInputs `json:"inputs"`
    Outcome scenariobench.SessionResult `json:"outcome"`
    Preflight PairPreflight `json:"preflight"`
}
func AppendCell(dst io.Writer, existing []CellRow, row CellRow) error
func ReadCells(src io.Reader) ([]CellRow, error)
```

Schema fathom.bench-cell/v1. Nested input JSON names lower_snake_case, Base
encodes current contextbench field names by an explicit wire adapter matching
its existing labeled invariants. Validate Base, reject CR/LF in its scalar fields
(existing canonical hash uses LF labels); Trial >=1; full subject SHA; family one
of PLAN/HAZ/FIX/LOC/REV/LANE/ASK. Canonical key is SHA256 of fixed-order JSON
`{base_key,subject_commit,tool_schema_sha256,profile_sha256,family,criterion_id,trial}`.
base_key is existing contextbench.CellKey(Base); no hash of Go map iteration.
ToolSchema/Profile hashes bind actual effective treatment. Baseline arms declare
not-applicable treatment fields using empty ArmSurface/schema/profile consistently;
their own harness/prompt/corpus/subject/scorer pins remain required. Changing
fathom commit alone cannot invalidate an otherwise identical daemon-free control.

ReadCells rejects malformed/unknown schema, duplicate key or stored key not matching
recomputed inputs, and reports line numbers. AppendCell validates against complete
existing rows and writes one canonical LF JSON row, never overwrites an old key.
It owns no hidden path/clock/lock. Command owner serializes append/create under the
existing ledger lock pattern. Raw transcripts stay workspace artifacts; SessionResult
is serialized through its existing outcome fields without transcript payloads.
New v1 ledger lives docs/bench/proof-cells.jsonl. Old ledgers remain immutable legacy
inputs; neither history rewritten nor incomparable trials silently borrowed.
Conversion adapters are explicit read-only imports with original run ids retained.

## Complete BEN-1 consolidation handoff (U2)

Recommended one bench command: keep existing cmd/scenariobench as the external
command owner, extending its current subcommand registration to `cells/readiness`.
No cmd/fathom verb/runner. Three one-day slices with separate issues/commits:
B1 key+ledger leaf; B2 command/legacy adapter/freeze; B3 consolidation parity receipt.
No slice alone claims BEN-1. #266 can implement B1 only under its current two files;
architect must grant the exact additional files below or split before dispatch.
No second new runner or statistics library. Reuse existing contextbench.CellKey,
scenariobench execution/scoring/oracles and ledger serializers through adapters.

| Slice owner | Exact proposed paths |
|---|---|
| B1 #266 | internal/benchledger/cellkey.go, cellkey_test.go, ledger.go, ledger_test.go |
| B2 command integration owner | cmd/scenariobench/main.go, cmd/scenariobench/cells.go, cmd/scenariobench/cells_test.go, cmd/scenariobench/readiness.go, cmd/scenariobench/readiness_test.go |
| B2 legacy freeze owner (same serial turn) | cmd/contextbenchscore/main.go, cmd/contextbenchscore/freeze_test.go, cmd/benchledgerrender/main.go, cmd/benchledgerrender/freeze_test.go |
| B3 receipt/data owner | docs/bench/readings/proof-program/ben-1-consolidation.md, docs/bench/proof-cells.jsonl |

Freeze means old score/render commands still read/render existing artifacts but
refuse writing NEW outcome/ledger rows, with explicit diagnostic directing to
scenariobench cells; no old history deletion and no public API change inferred.
Existing cmd/reviewbench/indexbench remain specialized free oracle/cost tools,
read-only adapters to unified ledger, not new paid drivers. If they can append
outcomes at frozen base, B2 must inventory their exact write site and get an added
named grant before B3; do not claim consolidation by ignoring another writer.
B3's TestBenchDriverConsolidation must enumerate all current ledger-writing entrypoints,
prove each old write path refuses while read/render parity remains, and one canonical
new row is accepted from kept command. No ad-hoc script writes the canonical ledger.

## One preflight admission contract shared with CTR-12 (U1)

```go
// internal/benchledger/cellkey.go (initial shared definitions)
type PreflightReceipt struct {
    SessionID, ProductCommit, SubjectCommit, ProfileSHA256 string
    BeforeAgent bool
    W0, W1, W2, W3, W6 string
    Reason string
}
type PairPreflight struct {
    PairID, TreatmentSession, ControlSession string
    BeforeAgent, Valid bool
    Reason string
}
func AdmitPair(pairID string, treatment, control PreflightReceipt) (PairPreflight, error)
```

Each applicable rung is pass/fail/unknown; not-applicable permitted only where a
baseline's frozen applicability profile explicitly declares no product/daemon.
Receipt digests bind the scripted observed artifacts and environment identity
before startup, not a caller's boolean assertion: adapter validates those bytes
and expected pins before AdmitPair. Add exactly `EvidenceSHA256 string` to
PreflightReceipt for the captured artifact digest, required, compared by adapter.
All applicable W0-W3/W6 pass, identities match expected cell and BeforeAgent true
for both -> Valid. Any fail -> whole pair VOID, not just failing arm. Unknown or
missing receipts -> not admitted with reason, never counted as a product success.
The writer records decision before agent actions and verifies unchanged environment.
No agent call/use/action field enters this decision. CTR-12's labs.DeliveryFunnel
imports PairPreflight, not the other direction. Valid zero-use treatment remains
scored/non-use in assigned arms. Non-use-conditioned reading may be secondary and
labelled biased toward product, never headline or ruling. Genuine daemon-free arms
prove their own startup/tool absence/budget, no fabricated stamp (WP-D2).

## Six readiness harnesses (U1)

```go
// internal/benchready/readiness.go
type Control struct { Name string; Task scenariobench.Task }
type ControlResult struct { Name string; Passed bool; Reason string }
type Runner func(context.Context, Control) (scenariobench.SessionResult, error)
func RunReadiness(ctx context.Context, controls []Control, run Runner) ([]ControlResult, error)
```

Exactly six unique Names: null/oracle/wrong-patch/flake/scripted-wired/scripted-grep.
Runner is the existing scenario execution adapter; in unit tests it is scripted,
never a provider. Null proves expected null scoring. Oracle gets every key right.
Wrong patch fails the independent oracle despite agent claiming success. Flake
alternates fixed seeded outcomes and must not look confirmed. Scripted-wired runs
preflight answered surface probes then fixed agent fixture actions; scripted-grep
proves no treatment surface/hooks and grep-capable controls. All six results must
pass before any paid cell, no missing harness treated as skipped pass. All preflight
phase calls excluded from use/outcome totals. Unknown host/harness channel makes
that conformance claim unknown, not a synthetic pass or guessed WSL parity.

## Capability proof card shape/closure (U1)

Existing capability.Registry is configuration/switch detection, not a proof card.
Keep it. Add Card in existing internal/capability/card_closure.go, not a replacement
registry. CTR-11's eight head schemas do not own this data shape.

```go
type ProofRef struct { Path, InputSHA256 string }
type Card struct {
    ID, Claim string
    Surfaces []string
    EffectStratum, NullStratum string
    Fact, Delivery, Outcome ProofRef
}
func CheckCardClosure(shipped []string, cards []Card) ([]string, error)
```

Proposed schema/records/data.cue:
```cue
#ProofRef: close({path: string & !="", input_sha256: =~"^[0-9a-f]{64}$"})
#Capability: close({
    id: string & !="", claim: string & !=""
    surfaces: [string, ...string]
    effect_stratum: string & !="", null_stratum: string & !=""
    fact: #ProofRef, delivery: #ProofRef, outcome: #ProofRef
})
```

References may point at preregistered pending-proof records; existence is not PASS.
No authored fact-proven/live fields. Readings compute proof liveness/order; missing
proof cannot unlock next tier. Closure compares exact verb:/tool:/hook: surface
keys from current registries, sorted unique. Missing card, orphan key, duplicate
card id or multiple conflicting claims errors with exact key. No substring match,
no manufactured surface from prose. #299 owns calibration/capabilities/demo.cue
and schema/records/testdata/cards/{valid,invalid}.cue after REC-1 -> REC-2 namespace
handoffs in CTR-11; this batch proposes no other REC-10 standard/query/knob fields.
That broader REC-10 row remains unclaimed until its own contract, no future fields.

## Subject admission and attested strength (U1)

```go
// internal/benchledger/subjects.go
type SubjectInput struct {
    Subject benchprep.Subject
    Receipt benchprep.VetReceipt
    Split string
    Family string
    OracleSource, OracleCommit string
}
type Admission struct { Subject, State string; Reasons []string }
func AdmitSubject(in SubjectInput) (Admission, error)
// internal/attested/junit.go, normalization only, no runner.
type TestResult struct { Test, Status, Commit, RunDigest string }
func ReadJUnit(src io.Reader, commit, runDigest string) ([]TestResult, error)
// internal/certify/strength_floor.go
type MutationResult struct { Test, Mutant, Commit, RunDigest string; Killed bool }
func EnforceStrengthFloor(certificate Record, floor extractor.CertifyStrength,
    tests []attested.TestResult, mutations []MutationResult) (State, error)
```

SubjectInput reuses actual C1..7 receipt and its refused/pending/kept verdict.
Applications first, repository-level development/held-out separation, at most two
subjects/org, no own fleet, independently pinned oracle and exact task-family key.
C1/C3..7 remain. ADR-109's scale-vector admission is an additional receipt, not
silently implemented by this old C2 receipt: add `EnvironmentReceiptPath string`
to SubjectInput and require its pinned concept/scale/per-arm conformance validation
for any ADR-109 claim. Until the environment contract lands, state pending, never
“old C2 pass means scale admitted”. False-by-construction config tasks are development
only. Missing cutoff stays unknown; no invented cutoff or missing suite run hidden.
BEN-5 adds no new subject evaluator runner, benchprep.VetSubject remains instrument.

Strength order is existing extractor.CertifyStrengthRank, not a new ladder. Record
is existing certify.Record. Its target/test/scope/live-state and RunDigest must
match independently attested results at exact source commit. Missing/malformed,
failed/flaky/stale/wrong-commit result never upgrades. Mutation-killed requires the
named mutant was killed by the matching test, not any run or another test. JUnit
reader normalizes qualified test identity and skipped/failure/error/passed without
running tests; no partial invalid XML treated as pass. Existing testrun.ParseTestRun
already supports JUnit; reuse its parser and preserve identity/diagnostics, then
join explicit external commit/digest. Ingest adapter supplies those attestations;
certify computes floor, never executes a mutator/test/provider. Floor breach returns
existing StateUnproven, flaky returns StateFlaky, stale returns StateStale; unknown
floor is error. Current exported marker grammar and default asserts unchanged.

## Exact issue ownership snapshot and serial transfer

These are the current issue allowlists, not grants to this draft author. Existing
files remain with the architect until a named serial handoff. Directory grants
are reproduced as authored and must be narrowed to the fixture files below before
dispatch. Tests in a contract batch transfer once, at its landed commit, to the
consumer. No concurrent shared-file owner.

### #266 Give bench cells stable unique keys (BEN-1)

- `internal/benchledger/cellkey.go` (new).
- `internal/benchledger/cellkey_test.go` (new).

### #300 Bench readiness (BEN-2)

- `internal/benchready/readiness.go` (new).
- `internal/benchready/readiness_test.go` (new).

### #299 Bench cards (BEN-3, REC-10)

- `internal/capability/card_closure.go` (new).
- `internal/capability/card_closure_test.go` (new).
- `schema/records/data.cue` (new).
- `schema/records/testdata/` (new).
- `calibration/capabilities/` (new).
- `calibration/standards/` (new).

### #301 Bench subjects (BEN-5)

- `internal/benchledger/subjects.go` (new).
- `internal/benchledger/subjects_test.go` (new).
- `calibration/bench/subjects/` (new).
- `calibration/bench/tasks/` (new).

### #321 Test strength (CHG-4)

- `internal/certify/strength_floor.go` (new).
- `internal/certify/strength_floor_test.go` (new).
- `internal/attested/junit.go` (new).
- `internal/attested/junit_test.go` (new).

## Exact grants and red handoff (U2/U3)

#300 owns readiness.go/readiness_test.go only; command adapter belongs B2, preventing
false claim that a leaf-only helper reaches product. #301 owns subjects.go/
subjects_test.go plus exact calibration/bench/subjects/demo.json and
calibration/bench/tasks/{plan,haz,fix,loc,rev,lane,ask}-development.json fixtures.
No held-out tasks can be edited after outcomes. #299 owns card_closure.go/test,
data.cue and exact cards namespace above after head schema release.
#321 owns strength_floor.go/test and new attested/junit.go/test; proposed extra
existing-file integration grants internal/certify/compile.go and
internal/fusion/ingest.go require one engine owner turn after other active engine
handbacks. Exact ingestion site must be grounded before dispatch; no claim a leaf
function alone establishes the current CHG-4 ingest join. No new store schema.

| Consumer exact test | Assertions and command |
|---|---|
| internal/benchledger/cellkey_test.go::TestLaneBenchCells | every input changes key; fixed treatment-only product change leaves daemon-free control key; trial differs; CR/LF rejected; wrong stored key/duplicates refuse; same input deterministic. `go test -race ./internal/benchledger -run '^TestLaneBenchCells$' -count=1` |
| internal/benchledger/cellkey_test.go::TestPairPreflightAssignedArms | healthy probe then zero agent calls stays Valid; W4/W5 irrelevant; scripted W0-W3/W6 fail voids whole pair before startup; missing/late receipt unknown/unadmitted; baseline applicability not fabricated. `go test -race ./internal/benchledger -run '^TestPairPreflightAssignedArms$' -count=1` |
| cmd/scenariobench/cells_test.go::TestBenchDriverConsolidation | kept command writes one canonical row; old write paths refuse; all legacy artifacts still render; no whole BEN-1 claim from key-only slice. `go test -race ./cmd/scenariobench -run '^TestBenchDriverConsolidation$' -count=1` |
| internal/benchready/readiness_test.go::TestLaneBenchReadiness | six unique controls required; null/oracle/wrong/flake discrimination; scripted wired/grep preflight/adapters, zero-use pair retained; missing any one fails paid readiness. `go test -race ./internal/benchready -run '^TestLaneBenchReadiness$' -count=1` |
| internal/capability/card_closure_test.go::TestLaneBenchCards | shipped surface without a card errors; orphan and duplicate surfaces conflict; schema valid/invalid; pending proof never promoted from presence. `go test -race ./internal/capability -run '^TestLaneBenchCards$' -count=1` |
| internal/benchledger/subjects_test.go::TestLaneBenchSubjects | refused/pending C1..7 stays unadmitted; independent oracle required; development config only; repository split disjoint; missing ADR-109 receipt cannot satisfy scale claim. `go test -race ./internal/benchledger -run '^TestLaneBenchSubjects$' -count=1` |
| internal/certify/strength_floor_test.go::TestLaneTestStrength | matching live attestation/floor passes; wrong target/test/commit/digest, skip/fail/flaky/stale/missing mutation cannot upgrade; only exact mutant kill meets mutation floor. `go test -race ./internal/certify -run '^TestLaneTestStrength$' -count=1` |
| internal/attested/junit_test.go::TestJUnitAttestedIdentity | qualified JUnit cases preserve status + commit/digest; malformed/missing identity refuses, skipped not pass. `go test -race ./internal/attested -run '^TestJUnitAttestedIdentity$' -count=1` |

Batch primary internal/benchledger/lane_contract_test.go::TestLaneContractsBench;
`go test -race ./internal/benchledger -run '^TestLaneContractsBench$' -count=1`.
Other proposed batch paths internal/benchready/lane_contract_test.go,
internal/capability/lane_contract_test.go, internal/certify/lane_contract_test.go.
Proposed fixtures internal/benchledger/testdata/contracts/{cell-valid,cell-duplicate,
preflight-zero-use,preflight-failed}.json; internal/benchready/testdata/contracts/
controls.json; internal/attested/testdata/contracts/{passed,failed,skipped,malformed}.xml.
CTR-13 owns them only for approved contract landing; transfer per consumer serially.
Each test's declaration existence is checked before command; missing file/package
is absence, not observed RED. U3 decides where actual failing execution is captured.
No default-build compile break lands merely to say test-first.

## Evidence and proof limits

Reuse source: contextbench/cell.go (CellInputs, CellKey, Validate), scenariobench/types.go
Task/SessionResult and ledger.go, benchprep/manifest.go/vet.go, capability Registry,
certify.Record/State, extractor.CertifyStrengthRank, testrun.ParseTestRun JUnit path.
Old ledger freeze and command consolidation need B2/B3 proof; not implemented here.
Executed draft check: git diff --check; commit secret scan. No RED/race/readiness/
paid cell/CUE/full gate run, subject admission or plan claim. BEN-1..5/CHG-4 stay open.
Decision models: none. Primary oracles and admission/strength joins deterministic;
provider none supplies full behavior. A manual review cannot become a cell oracle.
