# REC-1: standalone lane and record allocation contract

Status: DRAFT for W1-5 / issue #252. Not approved, implemented or dispatch-ready.
Base: `468dc705813382d584926887b451cbea145c76ac` (refreshed origin/main).
Dependencies: none for review. Landing and implementation require architect approval.
This branch owns only `docs/contracts/rec-1.md`. No companion fixtures land here.

## Intent before design

Read `docs/intent/README.md`, principles P2/P3/P15/P17/P20/P22,
`scope.md` compatibility, `scope/non-goals.md`, the decision index, ADR-087,
ADR-092 and ADR-094, `docs/design/typed-records.md` CU-D2/D5/D6/D7 and
`docs/design/parallel-lanes.md` PL-D1..6. ADR-087 is accepted. ADR-094 and
I70/I72/I75 are proposals, used as this draft's constraints, not shipping laws.
No authored phase, transition history, dispatch, store or new MCP tool. Existing
markdown and verify 0/1/2 semantics survive. Record vet is a separate gate.
ADR-092's LL-M1 FAIL trigger fired (W1-2). No language-door import is added.
No evidence that the syntax-trial or decision-file split trigger fired. No syntax
migration is proposed. Free option: keep records leaf-like; no export or kit move.

## Decisions to approve

- D1 recommended: exact repository-relative owned files, no patterns. Alternative:
  CODEOWNERS patterns plus parity fixtures. Exact paths satisfy PL-D5 now without
  copying its matcher or treating review routing as write permission.
- D2 recommended: pure allocator over an explicit inventory, with CLI providing
  serialized read/allocate/create. Alternative: allocator opens and writes files.
  An allocator call alone does not reserve an id and cannot promise cross-process
  uniqueness. Successful CLI creation, not a hidden counter, consumes the id.
- D3 recommended: the existing `recordids` checker owns inventory and uniqueness.
  Alternative: a new independent records registry (reject: CU-D6).
- D4 recommended: numeric intervals are closed, per family and prefix, minted by
  the integrator in trusted-base lane files. Alternative: global auto-reservation
  (reject: grants workers authority over shared ranges).
- D5 recommended: draft first, then author tests and capture RED in the approved
  implementation branch before implementation. Alternative: build-tagged contract
  tests landed separately with an explicitly tagged RED command. Default-build
  failing tests cannot land while the mandatory full gate must remain green.
- D6 recommended: approve the checker integration allowlist below before dispatch.
  Alternative: split REC-1 into checker-integration then allocator/CLI lanes.

## Standalone CUE, proposed `schema/records/lane.cue`

```cue
package records
#Family: "adr" | "invariant" | "future" | "incident" | "observation" | "plan"
#Range: close({
    family: #Family
    prefix: string & =~"^[A-Z][A-Z0-9]*-?$"
    first: int & >=1
    last: int & >=first
})
#Lane: close({
    kind: "lane"
    id: string & =~"^[a-z][a-z0-9-]*$"
    goal: string & !=""
    rows: [string, ...string]
    class: "L-D" | "L-R" | "L-P" | "L-F" | "L-E" | "L-S"
    base_commit: string & =~"^[0-9a-f]{40}$"
    owned_paths: [string, ...string]
    read_only_paths: [...string]
    contract_files: [string, ...string]
    proof_command: string & !=""
    budget: string & !=""
    harness_class: string & !=""
    id_ranges: [...#Range]
})
```

`budget` and `harness_class` are required by PL-D4, operator declarations only.
No host, account, session, scheduling, runtime status or future fields. Runtime
phase is derived elsewhere. No schema version is needed before a first consumer.
CUE checks shape. Go checks normalized paths, pairwise overlap, ids and trust.
Use `schema/records/schema.cue` as the package entrypoint for the existing cue-vet
and staged-CUE discovery rules. This new file requires the schema handoff too.

Proposed fixture reservation (not a reservation in the live repository):

| Family | Prefix | First..last |
|---|---|---|
| adr | ADR- | 10000..10001 |
| invariant | I | 10000..10001 |
| future | F | 10000..10001 |
| incident | INC- | 10000..10001 |
| observation | O | 10000..10001 |
| plan | DEMO- | 10000..10001 |

For live lanes the integrator scans the trusted inventory and all still-reserved
lane intervals, then commits an explicit unused closed interval for each family
needed by that lane. No default interval and no implicit family permission.
ADR renders decimal with minimum width 3 (`ADR-001`), others no padding. Plan
prefixes must be an existing plan's author-owned prefix. No new id family for
lanes: lane `id` is a unique slug. Equal numbers in different prefixes do not
collide. Duplicate full ids in different families are invalid too. Overlapping
intervals for the same prefix fail even if their family names differ.

## Go contracts (declarations only)

All names below are proposed except `recordids.Location`, `Family`, `Config`,
`Duplicate`, `Defaults`, `Load` and `Check`, which already exist.

```go
// internal/recordids: single identity discovery/checking implementation.
type Definition struct {
    Family string
    ID string
    At Location
}
func Inventory(root string, cfg Config) ([]Definition, error)
func CheckDefinitions(defs []Definition, cfg Config) ([]Duplicate, error)

// internal/records; no store, daemon or language door import.
type IDRange struct { Family, Prefix string; First, Last int }
type Lane struct {
    Kind, ID, Goal, Class, BaseCommit string
    Rows, OwnedPaths, ReadOnlyPaths, ContractFiles []string
    ProofCommand, Budget, HarnessClass string
    IDRanges []IDRange
}
func AllocateRecordID(lane Lane, family, target string,
    inventory []recordids.Definition) (string, error)
// cmd/fathom, existing args/writer convention
func cmdRecNew(args []string, stdout, stderr io.Writer) int
```

`Check` calls Inventory then CheckDefinitions preserving existing sorting,
plan parsing, archives and exact grandfather rules. Inventory must not collapse
repeated definitions. Extend Defaults with observation headings in the legacy
log and per-entry files. Match a definition heading only, not references. Also
read CUE lane declarations, validate all ranges and detect duplicate lane slugs.
Preserve existing grandfathered plan pairs, but never allow a new reservation
to reuse one of their ids. No grandfather bypass for new records.

Allocator rejects malformed lane/range, absent family, ambiguous family ranges,
any duplicate in inventory (including observations), noncanonical target,
absolute path, traversal, symlink escape, or target not in exact OwnedPaths.
Read-only paths never grant write permission. Duplicate diagnostics name both
locations, sorted by family/id/path/line. Lowest unused integer in the matching
range wins, independent of inventory order. Exhaustion/error returns `"", err`.
No mutation of inputs and no internal clock. Repeated calls with unchanged
inventory intentionally return the same candidate. A successful caller must add
the created definition before the next call. This is explicit in the repeat test.

`rec new --root DIR --lane FILE --family NAME --path REL` reads a lane declaration
from the trusted base, not a broadened worktree version. Creation writes a minimal
new definition file using exclusive create. Outside path rejection precedes writes
and consumes nothing. CLI serializes competing calls under a clone-local lock in
`workspace/`, rereads inventory under lock, creates once, and releases the lock.
Cross-clone collisions are prevented by disjoint trusted reservations. Error exit
1, misuse 2, success 0. Print the allocated id and path only after successful
creation. No subprocess/provider/forge invocation. `rec` verb registration remains
an architect-owned integration edit, using the existing declareVerb convention.

## Exact ownership and serial handoff

Issue #252 implementation owns `internal/records/lane.go`, `allocate.go`,
`allocate_test.go`, `schema/records/lane.cue`, `schema/records/testdata/`,
`cmd/fathom/rec_new.go`, `cmd/fathom/rec_new_test.go`.
Recommended architect integration grant, separate from that current allowlist:
`internal/recordids/recordids.go`, `internal/recordids/recordids_test.go`,
`cmd/fathom/gate_recordids.go`, `schema/records/schema.cue`.
Checker extracts Inventory first, then allocator/CLI consumes it. Architect retains
existing CLI/help registration files and gate configuration. No lane dispatch
until these exact edits and the base supplying them are recorded in its manifest.
CTR-3 receives only trusted exact path lists. CTR-11 receives Lane and inventory,
not a second schema or allocator. Reserved ids are committed before workers start.

## Proposed fixtures and executable proof handoff

All fixtures below are NEW proposals, not files present on this branch.
`schema/records/testdata/valid.cue` contains the lane and six fixture ranges above.
`invalid.cue` stays commented, with named cases `shape`, `derived-phase`,
`inverted-range`, `overlap-range`, `duplicate-key`, `outside-path`,
`exhausted-range`. Go tests instantiate each invalid case and assert an error.
Positive CUE fixtures are live, negative ones never fail `cue vet ./...` by design.

`internal/records/allocate_test.go::TestLaneLaneRecords` subtests:
`shape` rejects unknown field `phase`; `duplicate-key` seeds ADR-10000 twice in
separate files and expects both locations; `duplicate-observation` similarly seeds
O10000; `exhausted-range` seeds both ids and expects empty id plus exhaustion error;
`outside-path` expects empty id and unchanged inventory; `repeat` allocates 10000,
adds its definition, then expects 10001; `families` allocates equal numbers under
six prefixes without collision; `reordered` permutes inventory and compares exact
id/error text; `symlink-escape` rejects a path whose parent resolves outside root.
`internal/recordids/recordids_test.go::TestLaneRecordIDInventory` preserves current
archive/grandfather cases and rejects duplicate CUE slugs and overlapping ranges.
`cmd/fathom/rec_new_test.go::TestRecNewRejectsUnownedPathWithoutAllocating` invokes
`run`, asserts nonzero, no created file, unchanged inventory and same next candidate.
Also test two simultaneous valid creates produce distinct ids and no partial file.

```sh
rg -n '^func TestLaneLaneRecords\(' internal/records/allocate_test.go
go test -race ./internal/records -run '^TestLaneLaneRecords$' -count=1
go test -race ./internal/recordids -run 'TestLaneRecordIDInventory|TestRepositoryRecordIDs' -count=1
go test -race ./cmd/fathom -run '^TestRecNewRejectsUnownedPathWithoutAllocating$' -count=1
task cue-vet
```

Expected initial failure: missing proposed declarations or failing assertions after
tests exist, captured with exact base, head, command and exit. No test was run RED
here. No gate lease taken while the shared gate runs. This document needs only
`git diff --check`; code/CUE snippets are draft contracts, not verified builds.
REC-1 stays unchecked. After approved implementation, scratch-ingest its HEAD and
run `fathom verify --service fathom --plan docs/plans/typed-records.md` before claim.

## Evidence and decision models

Source: `internal/recordids/recordids.go` Defaults/Check and its existing
`TestRepositoryRecordIDs`; `cmd/fathom/gate_recordids.go::cmdGateRecordIDs`.
History: `2a097224f` introduced the checker; `36f89ea5d` tightened exceptions.
No independent registry is needed. Missing observation checking is visible in
Defaults. Proposed `internal/records` is absent at base.
No decision model applies: paths, identities and intervals are computable.
Provider none needs no substitute. Opt-out is integrator-minted ids with manual
review, without an automation claim. Fixtures above are the proposed measurement.
