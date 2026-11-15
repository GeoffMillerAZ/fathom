# REC-1: standalone lane and record allocation contract

Status: W2-2 APPROVED choices applied for issue #252; exact-commit critic confirmation
pending. Documentation only, no implementation or dispatch readiness.
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
ADR-092's LL-M1 FAIL trigger fired. W2-1 answers it: door 80, consumer 81,
no package headroom. No language-door import is added.
No evidence that the syntax-trial or decision-file split trigger fired. No syntax
migration is proposed. Free option: keep records leaf-like; no export or kit move.

## Decisions settled by W2-2

- D1 recommended: exact repository-relative owned files, no patterns. Alternative:
  CODEOWNERS patterns plus parity fixtures. Exact paths satisfy PL-D5 now without
  copying its matcher or treating review routing as write permission.
- D2 recommended: pure allocator over an explicit inventory, with CLI providing
  serialized read/allocate/write. Alternative: allocator opens and writes files.
  An allocator call alone does not reserve an id and cannot promise cross-process
  uniqueness. Successful CLI writing, not a hidden counter, consumes the id.
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
type Definition struct { Family, ID string; At Location }
type IDRange struct { Family, Prefix string; First, Last int }
type LaneGrant struct {
    ID string
    OwnedPaths []string
    IDRanges []IDRange
}
type OwnershipViolation struct { Definition Definition; LaneID, Reason string }
type TrustedInventory struct {
    Revision string
    Lanes []LaneGrant
    Legacy []Definition
}
func Inventory(root string, cfg Config) ([]Definition, error)
func CheckDefinitions(defs []Definition, cfg Config) ([]Duplicate, error)
func CheckOwnership(defs []Definition, trusted TrustedInventory) ([]OwnershipViolation, error)
// Opaque result, constructed only by the shared checker. Zero value is invalid.
type ValidatedInventory struct { /* unexported state */ }
func ValidateInventory(defs []Definition, policy Config,
    trusted TrustedInventory) (ValidatedInventory, error)
func (v ValidatedInventory) Definitions() []Definition // defensive full copy

// internal/records; no store, daemon or language door import.
type IDRange = recordids.IDRange
type Lane struct {
    Kind, ID, Goal, Class, BaseCommit string
    Rows, OwnedPaths, ReadOnlyPaths, ContractFiles []string
    ProofCommand, Budget, HarnessClass string
    IDRanges []IDRange
}
func AllocateRecordID(lane Lane, family, target string,
    inventory recordids.ValidatedInventory) (string, error)
// cmd/fathom, existing args/writer convention
func cmdRecNew(args []string, stdout, stderr io.Writer) int
```

`Check` calls Inventory then CheckDefinitions preserving existing sorting,
plan parsing, archives and exact grandfather rules. Inventory must not collapse
repeated definitions. Extend Defaults with observation headings in the legacy
log and per-entry files. Match a definition heading only, not references. Also
read CUE lane declarations, validate all ranges and detect duplicate lane slugs.
Preserve existing grandfathered plan pairs, but never allow a new reservation
to reuse one of their ids. No grandfather bypass for new records. The caller loads the effective Config
from the trusted base using Defaults/Load, including exact Grandfather path sets
and CrossPlan, then calls ValidateInventory. That shared checker calls
CheckDefinitions with the effective policy and CheckOwnership with trusted grants.
Any checker/policy/ownership error returns an invalid result plus error. The caller
must stop before allocation. Successful validation preserves every definition,
including both members of permitted grandfather pairs. The opaque result carries
validity, and Definitions returns a copy without collapsing entries. No candidate
can weaken policy or add an exception.

The pure allocator accepts only that validated result, not raw definitions or
Config. It has no duplicate rule of its own. It refuses an id already defined,
grandfathered ones included, or outside the granted interval. The lowest free
in-range id wins. New duplicate detection belongs entirely to caller validation.
No hardcoded repository exceptions. This is W2-2's exact checker boundary.

Lane/range and target validation before allocation rejects malformed lane/range,
absent family, ambiguous family ranges, noncanonical target,
absolute path, traversal, or target not in exact OwnedPaths. This pure function
checks lexical containment only. Filesystem containment is a CLI boundary below.
Read-only paths never grant write permission. Shared checker duplicate diagnostics name both
locations, sorted by family/id/path/line. Lowest unused integer in the matching
range wins, independent of inventory order. Exhaustion/error returns `"", err`.
No mutation of inputs and no internal clock. Reject an invalid zero-value
ValidatedInventory before selecting a candidate. Repeated calls with unchanged
validated inventory intentionally return the same candidate. A successful caller adds the created definition and revalidates the complete
inventory before the next call. This is explicit in the repeat test.

`rec new --root DIR --lane SLUG --family NAME --path REL --title TEXT` is frozen.
No trusted-ref flag and no trust inferred from Lane.BaseCommit. The integrator
injects `FATHOM_LANE_TRUSTED_BASE` (full commit id) and `FATHOM_LANE_ID`; both are
required and the slug must match. In CI the controller supplies the immutable
trusted event base and lane id, never a candidate-branch field. Locally the
operator supplies these values as authority; this is not a sandbox against an
operator who deliberately changes its own environment. Missing/unreadable trust
fails closed. Read `docs/lanes/<slug>.cue` from that commit via git object reads.
Container is exactly `lane: #Lane & { ... }`, importing the standalone records
schema through the repository's CUE module. Discover only direct `.cue` entries
under trusted `docs/lanes/`, each filename equals lane.id. No recursive glob,
worktree import, lane-supplied base or additional declaration grants authority.
Decode against the schema bytes at that same trusted revision. Compare the
worktree manifest bytes to trusted bytes and reject any difference, including
self-broadening. ContractFiles/ReadOnlyPaths are never write grants.

`TrustedInventory.Legacy` is Inventory at the trusted revision, minus ids covered
by any reserved interval. Legacy definitions remain permitted at the same
family/full-id/path only (line numbers may shift). Moving a legacy definition is
an explicit integrator migration and otherwise fails. Every nonlegacy definition
must map to exactly one trusted lane by family/prefix/closed interval. Zero or
multiple lane matches fail. Its path must be an exact member of that lane's
OwnedPaths. No definition-authored lane field is trusted. Thus even a unique id
moved to another lane's file fails. CheckOwnership sorts violations by
family/id/path/line/lane/reason. Both `gate record-ids` and `rec new` consume the
same trusted inventory and CheckOwnership result. Existing Check signature stays
usable for legacy duplicate-only consumers; the gate gains ownership explicitly.

CLI uses `validateRecordTarget(root, rel string) error` in rec_new.go before
allocation and again before writing: resolve root and existing ancestors, reject
symlinks in target or parents, absolute/traversal/noncanonical names and escapes.
The symlink test belongs to rec_new_test.go, never the pure allocator. A clone-local
lock in `workspace/` covers fresh inventory, shared ValidateInventory, allocation and write.
Append to an existing owned aggregate or create a new exact owned file, using a
same-directory temp and atomic rename with existing mode preserved. This is not
a split of ADR.md. Outside-path and trust rejection happen before any write or id
consumption. Concurrency guarantee covers callers of this CLI in one clone only;
external writers must obey the lane's single writer contract. Across clones,
disjoint trusted ranges prevent collisions. No counter or allocation journal.

TEXT must be nonempty single-line text without controls, `{`, `}`, `|`, backticks
or CUE interpolation. These refusals keep the minimal emitter unambiguous. Emit
exact UTF-8 bytes (LF) below, substituting ID/TEXT. Append separator is two LF if
the existing file is nonempty, first trimming only trailing LF. A new file has no
separator. No derived fields or new lifecycle syntax. Per-family payload:

```text
adr:         ## ID: TEXT\n\n**Status.** Proposed\n
invariant:   | ID | TEXT | stated | |\n
future:      ## ID. TEXT\n
incident:    ## ID: TEXT\n
observation: ### ID: TEXT\n
plan:        - [ ] TEXT {id: ID}\n
```

These are literal escaped representations of emitted payload bytes, not six
prefixes to write. The checker adds exact trusted owned target paths to the
corresponding existing Family.Files and keeps its heading/plan parser. Observation
headings use `^#{2,6}\s+(O[0-9]+)\b`. A plan's prefix and target must agree with
its trusted range. No arbitrary regexp or file glob is taken from a candidate.
REC-2 subsequently vets these legacy-compatible authored headings; this draft
adds no speculative head fields. The trusted fixture repository includes one
initial file per family and docs/lanes/demo.cue, reserving the six ranges above.
Creation/appending followed by Inventory must recover exactly one new definition
with the same family/id/path and pass CheckOwnership. CLI error exit 1, misuse 2,
success 0. Print `ID REL\n` only after successful atomic write. Git object reads
are allowed; no provider, test execution or forge invocation. Architect owns
existing CLI registration/help integration with declareVerb.

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
separate files and expects caller validation to fail with both locations, without
calling the allocator; `duplicate-observation` similarly seeds O10000; `exhausted-range` seeds both ids and expects empty id plus exhaustion error;
`outside-path` expects empty id and unchanged inventory; `repeat` allocates 10000,
adds its definition, revalidates, then expects 10001; `families` allocates equal numbers under
six prefixes without collision; `reordered` permutes inventory and compares exact
id/error text; `grandfather-pair` supplies an exact two-plan grandfather pair to the shared
validator with effective policy, then expects the unrelated reserved id to allocate
from the validated result while preserving both original definitions; `grandfather-third-path` adds the same id in a third path
and expects caller validation error with no allocator call; `grandfather-local-duplicate` repeats
it inside an excepted plan and expects caller validation rejection; `grandfather-reuse` reserves a
range including the grandfathered suffix and expects the next free id, never
that occupied id; `stale-grandfather` removes one required pair member and expects
caller validation to fail with stale-policy error and no allocator call. No fixture hardcodes the repository's DF list:
use configurable DEMO pairs and separately exercise effective repository Config. No filesystem assertions in the pure allocator test.
`internal/recordids/recordids_test.go::TestLaneRecordIDInventory` preserves current
archive/grandfather cases and rejects duplicate CUE slugs and overlapping ranges.
`cmd/fathom/rec_new_test.go::TestRecNewRejectsUnownedPathWithoutAllocating` invokes
`run`, asserts nonzero, no created file, unchanged inventory and same next candidate.
`TestRecNewTrustedRoundTrip` tests all six exact emitted payloads, discovery at
trusted revision, Inventory round trip, append preservation, no duplicate on
re-inventory, self-broadened manifest rejection and no writes with missing trust.
`TestRecNewRejectsSymlinkEscape` rejects target and parent symlinks before mutation.
`TestRecordDefiningLaneOwnership` in recordids_test.go seeds an in-range definition
in another lane's owned file, a moved legacy definition, a unique unreserved id,
and overlapping grants. Each fails with its path and lane/reason, irrespective
of candidate manifest edits. A valid owned in-range definition passes. Test two
simultaneous CLI calls produce distinct ids and no partial target.

```sh
rg -n '^func TestLaneLaneRecords\(' internal/records/allocate_test.go
go test -race ./internal/records -run '^TestLaneLaneRecords$' -count=1
go test -race ./internal/recordids -run 'TestLaneRecordIDInventory|TestRepositoryRecordIDs' -count=1
go test -race ./cmd/fathom -run '^TestRecNew(RejectsUnownedPathWithoutAllocating|TrustedRoundTrip|RejectsSymlinkEscape)$' -count=1
go test -race ./internal/recordids -run '^TestRecordDefiningLaneOwnership$' -count=1
task cue-vet
```

Expected initial failure: missing proposed declarations or failing assertions after
tests exist, captured with exact base, head, command and exit. No test was run RED
here. No gate lease taken while the shared gate runs. This document needs only
`git diff --check`; code/CUE snippets are draft contracts, not verified builds.
REC-1 stays unchecked. After approved implementation, scratch-ingest its HEAD and
run `fathom verify --service fathom --plan docs/plans/typed-records.md` before claim.

## W2-2 ruling application (supersedes FIX round 2 boundary)

W2-2 verbatim semantics: “the caller validates the inventory with the existing
checker and its effective policy, and hands the allocator that validated result.
The allocator has no duplicate rule of its own. It refuses only an id that is
already defined, grandfathered ones included, or one outside the caller's granted
range.” The declarations and assertions above implement this contract in prose.
D1..D6 are settled by W2-2 explicitly, including the integration grant and real
implementation-lane RED. Prior commits are preserved. Critic must confirm this
exact revision before the approval marker is bound to its commit. No live ranges,
product implementation or passing gate is implied.

## FIX round 2 receipt (historical, boundary superseded by W2-2)

F4 initially gave AllocateRecordID effective Config; W2-2 above replaces that boundary.
The existing checker applies its exact grandfather policy to the full inventory.
Allowed pairs do not block unrelated allocation; all their ids remain occupied.
New third-path/local duplicates and stale policy fail. Named assertions above
cover each distinction without collapsed input or hardcoded exceptions. This is
the final FIX allowance. Any further unresolved revision must be held for ruling.

## FIX round 1 receipt

F1: pure lexical allocator and root-aware CLI containment now have separate inputs
and tests. F2: CheckOwnership receives independently trusted per-lane grants plus
legacy inventory, checking every definition and refusing moves or unreserved ids.
F3: concrete trusted CUE discovery/container, immutable external trust source,
exact CLI flags/payload bytes and six-family round-trip tests are specified.
These are recommendations, not observed RED or architect approval.

## Evidence and decision models

Source: `internal/recordids/recordids.go` Defaults/Check and its existing
`TestRepositoryRecordIDs`; `cmd/fathom/gate_recordids.go::cmdGateRecordIDs`.
History: `2a097224f` introduced the checker; `36f89ea5d` tightened exceptions.
No independent registry is needed. Missing observation checking is visible in
Defaults. Proposed `internal/records` is absent at base.
No decision model applies: paths, identities and intervals are computable.
Provider none needs no substitute. Opt-out is integrator-minted ids with manual
review, without an automation claim. Fixtures above are the proposed measurement.
