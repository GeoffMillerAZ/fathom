# ENV concept ownership through the shared record checker

Status: UNAPPROVED PROPOSAL ONLY. WR-CORE-REC/U1 requires an architect ruling
and the sole wave critic's review. No policy, schema, API, file grant, inventory,
implementation or dispatch is authorized by this document.
Base: 17d930d2f1b96b707b03ec84d5f87a3a5333c24d.
This change owns only this new contract. The implementation paths below are a
requested future allowlist, not present ownership or permission to write them.

## Governing sources and the existing gap

- REC is accepted source 3d53bd73a892aa6a5ce2b3909f7da60654ed982e,
  `internal/recordids/recordids.go` (R below). Acceptance is not a claim that its
  implementation prerequisite is on main. It awaits the integration acknowledgement.
- The approved `docs/contracts/rec-1.md` (C), especially lines 143-202 and 232-236,
  and `docs/contracts/records-integration.md` (I), lines 38-60 and 150-155,
  are read at frozen source 23f40f055d63f5f9212f111eb80f07319083ef6e.
  Architect W2-2 settles shared validated inventory, trusted exact paths, existing
  numeric allocation and grandfather behavior. CU-D2/6/7 are
  `docs/design/typed-records.md:89-95,135-145`: CUE data, one duplicate checker,
  separate record vet, unchanged verify semantics.
- ENV is `docs/contracts/env-1-core.md` at
  104db84571e83848ec73e09d5878dce817cdb89d (E). Its bounded-publication review
  does not land its implementation. E:88,109-116,472-482,1393 make shared
  discovery, source locations, trusted policy and file grants prerequisites to
  RecordOwnership. E:331-367 and W16-2 require one reviewed DATA inventory with
  open, byte-exact keys and Card/BE-D2/BE-D9 citations, not a contract key list.

P15/P20 (`docs/intent/principles.md:57-58,72-73`) require exact ownership and a
named consumer. I8/I38 (`docs/intent/invariants.md:40,68`) preserve static verify
and honest read coverage. CUE DATA is already permitted; no prose-authoring or
syntax-trial trigger is asserted. No future is promoted. F25 is speculative,
not a reason to prebuild migration machinery. This proposal adds no store,
resident structure, public door package, execution engine or generic head system.

Existing shared signatures remain the seams (R:107,283,406,452,649):

```go
func Inventory(root string, cfg Config) ([]Definition, error)
func CheckDefinitions(defs []Definition, cfg Config) ([]Duplicate, error)
func CheckOwnership(defs []Definition, trusted TrustedInventory) ([]OwnershipViolation, error)
func ValidateInventory(defs []Definition, policy Config, trusted TrustedInventory) (ValidatedInventory, error)
func LoadTrusted(ctx context.Context, root, revision string) (TrustedState, error)
```

`Definition{Family, ID, At: Location{Path, Line}}` preserves occurrences and
locations (R:48-50,221-225,1000-1035). Present discovery is line-regex or Markdown;
CUE parsing is for direct lane manifests (R:533-635). Effective policy at the
frozen source has no family override and no concept discovery. There is no live
concept inventory or grant. Nonlegacy ownership first requires a canonical
numeric range, then exact OwnedPaths (R:337-363,406-449). Merely freezing that
behavior cannot discharge ENV's open-string ownership requirement.

## U1: UNSETTLED architect choice

Recommend option A below as one bounded upstream extension for this present
consumer. E:1393's prohibition on an ENV-local invented record family remains
binding: this proposal explicitly asks the architect to settle that upstream
policy before either owner implements it. It is not an implied exception.

Option A wins if the named ENV RecordOwnership consumer needs this seam before
REC-6 and the complete discovery/trust/compatibility assertions below can be
proved. Option B is to defer the representation and ownership decision to REC-6
and keep ENV RecordOwnership/dispatch held. B wins if this narrow upstream policy
is not approved or the consumer can wait for that loader. Neither option is
selected or authorized here. There is no local duplicate-checker substitute.

### U1a: proposed additive shape, not a reservation

All additions in this section are UNSETTLED. Extend the existing internal types,
not a second registry or new exported package:

```go
// One non-allocatable identity family; IDRange's numeric vocabulary is unchanged.
const FamilyConcept FamilyName = "concept"
// Add to Config. Omitted/false retains the exact legacy discovery behavior.
Concepts bool `json:"concepts,omitempty"`
// Add to BOTH LaneDeclaration and LaneGrant, and copy through validated grants.
ConceptPaths []string `json:"concept_paths,omitempty"`
```

The proposed standalone `#Lane` addition is only
`concept_paths?: [...string]`. Omitted means no concept permission; when authored,
JSON and CUE require a non-null array of strings. The six existing numeric
`#Family` members and `#Range` remain unchanged. No concept numeric interval,
synthetic prefix, allocator branch, production key or fixed vocabulary is added.
`internal/records.Lane` remains the existing alias; numeric allocator and CLI
signatures, emitted numeric payloads and legacy Check behavior stay unchanged.
Grounds: C:49-69,143-173; R:175-184,221-236; W2-2 and E:365-367.

### U1b: proposed discovery and declaration boundary

UNSETTLED: when the trusted effective Config.Concepts is true, Inventory adds one
bounded scan of direct `calibration/environments/*.cue` files. This is a discovery
envelope, NEVER an ownership pattern. Only `calibration/environments/concepts.cue`
can receive the concept file grant. The envelope includes other direct files so a
second or ungranted inventory is visible, not silently skipped. No whole-repository
or recursive discovery claim follows. When enabled, the canonical inventory must
exist as a regular readable file; missing inventory refuses ErrTrust, never a
healthy empty total. When omitted/false, this adapter performs no extra reads.
The core consumer may load only the sole canonical inventory path; a different
path cannot be treated as its owned input.

Use the existing CUE parser to inspect authored AST BEFORE unification, following
only literal top-level `inventory` structs (including the normative
`inventory: #ConceptInventory & {...}`) and their `concepts` list members' `key`
fields. Repeated inventory/struct declarations and same-line key declarations
are all retained, not merged. References in groups, citations and other records
are not definitions. Each occurrence yields FamilyConcept, the decoded exact
string and the key token's original repository-relative path/1-based line.
Keys must satisfy ENV's existing #Text requirement; invalid blank keys refuse
ErrShape rather than entering ownership. Preserve accepted bytes without trimming.
No case folding, slugification, trimming, normalization or numeric parsing.
Multiplicity is retained even when two locations share the same line.

UNSETTLED syntax admission: only literal string key values and literal list/struct
membership are ownership-discoverable in this first adapter. Computed keys,
interpolation, imported values, aliases, comprehensions, embedding or open lists
on that declaration path refuse ErrShape with a source location, rather than
producing an incomplete inventory. The fixed head reference is not evaluated
for discovery. Comments, unrelated definitions and reference-only fields are not
collected. CUE shape/join success alone still establishes no ownership. The core
owner must reconcile this explicit ownership-admission subset before dispatch;
this does not change its pure validators or pretend all CUE expressions work.

Unreadable files, parse failures and nonregular/symlink files or ancestors refuse;
none becomes absent, zero definitions or healthy coverage. Paths and output order
use existing canonicalization and deterministic family/id/path/line sorting.
Grounds: E:109-116,331-367,472-482,1393; R:107-162,1000-1035; I38.

### U1c: proposed trusted grant and shared checking

UNSETTLED: the integrator may later supply one trusted lane with ConceptPaths
containing exactly `calibration/environments/concepts.cue`, also present in that
lane's OwnedPaths. This data-only lane has exactly that one OwnedPaths entry and
IDRanges: []; it cannot append numeric payloads to the concept inventory through
the unchanged allocator. Unrelated numeric grants belong to other lanes.
Zero grants cannot authorize keys; multiple concept writers, mixed numeric/concept
grants, other paths, patterns and paths outside OwnedPaths refuse. The grant covers open
keys supplied by that lane's separately reviewed inventory DATA. It contains no
key enumeration, key-generation rule or duplicate registry. Citations/data review
remain the ENV owner's separate duties, not checks this adapter invents.

LoadTrusted must read the flag, schema and lane from the SAME operator-selected
immutable commit, using the accepted raw blob/limit/no-replacement machinery.
It includes the fixed discovery envelope in that snapshot as well as the existing
numeric discovery. Candidate config, claimed lane, BaseCommit, imports and altered
manifest bytes never broaden it. Missing policy/schema/grant fails closed.
If concept ownership is requested, gate record-ids must require the trusted path;
it cannot claim that proof through its legacy duplicate-only fallback. Within
trusted validation, concept definitions or active ConceptPaths with the flag false
refuse ErrPolicy; they cannot be discarded or activated by candidate fields. Legacy
numeric-only Check remains duplicate-only. Existing unconfigured callers do not
opt in accidentally. Grounds: C:177-202,232-236; R:647-727,811-820,919-930,1110-1127.

ValidateInventory still invokes the SAME CheckDefinitions across the entire
inventory and then the SAME CheckOwnership, producing the same opaque validated
result. No family-qualified collision escape or second duplicate algorithm is
added. Exact grandfathered numeric plan pairs remain permitted, but a concept
reusing one of their full IDs or any new duplicate refuses. Existing copies,
stable ordering, full-ID collisions and typed refusals remain (R:263-279,299-311,
452-489; C:143-173; W2-2).

UNSETTLED ownership branch: for FamilyConcept, require exactly one trusted concept
grant and its exact path, BEFORE the existing legacy exemption. Do not classify
concept definitions as unrestricted Legacy when building TrustedInventory.
Existing numeric Legacy matching and closed-range matching are unchanged.
OwnedPaths alone, or a numeric range for another family, is not concept authority.
Deep-copy ConceptPaths at every existing grant-copy boundary. Preserve
ValidatedInventory's checker-only constructor and defensive Definitions/Grant.
An unrelated approved numeric allocation still uses AllocateRecordID unchanged;
concept allocation through it has no matching range and refuses. The shared
inventory still marks every defined full ID used, grandfathered ones included.

### U1d: proposed exact future path handoffs

UNSETTLED requested implementation allowlist, only AFTER a ruling, exact-source
review, acknowledgement of REC-1 and fresh implementer assignment:

- `internal/recordids/recordids.go` and `internal/recordids/recordids_test.go`:
  additive policy/grant fields, bounded AST discovery, shared trust/ownership and
  defensive copies. No generic discovery plug-in or public door package.
- `internal/records/lane.go` and `internal/records/allocate_test.go`:
  optional-field JSON presence/null validation and numeric compatibility tests;
  `internal/records/allocate.go` remains read-only.
- `schema/records/lane.cue`, `schema/records/testdata/valid.cue` and
  `schema/records/testdata/invalid.cue`: the optional field and paired rejection
  cases only. Invalid CUE stays commented and is exercised by the Go tests,
  never as live gate-breaking input (`schema/AGENTS.md`).
  `schema/records/schema.cue` stays the existing package entrypoint.
- `cmd/fathom/gate_recordids.go` and existing `cmd/fathom/rec_new_test.go`:
  require existing trusted validation for concept proof; no verb or gate owner
  rewrite, new exit protocol or implementation of rec vet.

The serial REC-1 -> heads -> parser/evaluator -> consumer/gate ownership in I:38-60
still governs. This proposal grants no simultaneous writer of lane schema.
Activation is a SEPARATE integrator turn, with the exact trusted revision, one
concrete lane id/manifest pathname, and `.fathom/record-ids.json` named before any
write. No placeholder slug or pattern here is an allowlist. The DATA owner then
needs its own exact grant for `calibration/environments/concepts.cue`; no inventory
contents are supplied here. ENV's `schema/benchenv/validate_test.go` RecordOwnership
assertion belongs to the core owner after shared machinery/grants exist, not this
allowlist. No heads, REC-6 loader, bench schema or production DATA is co-owned.

## Prospective RED and acceptance, all UNRUN

These are a future implementer's obligations, not passing tests or API completion.
After approval, add only the additive shape/scaffolding needed to compile tests,
then capture actual assertion RED before behavior. Expected two raw definitions
versus zero, or expected authorized ownership versus refusal, is a semantic RED;
a missing-symbol/unknown-field compile error is not. Regression assertions that
already pass must be distinguished from new-feature RED. Apply W3-2/W2-2's real
RED/GREEN and independent assertion-by-assertion review; never land failing tests
or promote a fixture/schema/list-presence check to native proof.

| Exact proposed test and file | Required assertions |
|---|---|
| `TestConceptRecordDiscovery` in `internal/recordids/recordids_test.go` | Supplied nonempty K in literal inventory data yields its exact bytes/path/key line. Two declarations of K in separate discovered files with disjoint fields yield TWO definitions and both original locations before unification; shared validation returns ErrDuplicate with both. Repeat in one file and on one line; retain multiplicity. Ignore comments/group references. Unsupported computed/imported/alias/list forms and unreadable/symlink paths refuse rather than omit. Reordering inputs preserves deterministic diagnostics. |
| `TestConceptRecordOwnership` in the same file | One valid trusted data-only ConceptPaths+single OwnedPaths grant with IDRanges: [] accepts supplied K only at the sole inventory path. A mixed numeric/concept lane refuses ErrShape. A unique K in a second discovered file, a missing/mismatched/duplicate grant, read-only-only path or numeric-only grant returns ErrOwnership/ErrShape as applicable, never healthy zero. Put a concept in the trusted snapshot: moving it or omitting the concept grant still refuses; no Legacy escape. |
| `TestConceptRecordTrust` in the same file | Candidate flag/manifest/schema edits cannot broaden trusted discovery/grants; modified/extra/missing lane bytes refuse. A real raw-blob snapshot with export-ignore/export-subst/replacement-object attempts retains the accepted trust protections. Omitted flag leaves old discovery byte-for-byte unchanged. Mutating returned ConceptPaths cannot mutate the validated grant. |
| `TestConceptRecordNumericCompatibility` in `internal/records/allocate_test.go` | With the effective grandfathered pair present, shared validation and an unrelated existing numeric fixture grant in a separate lane still allocate its lowest unused id. A new duplicate, concept collision with a grandfathered full ID and attempted grandfather reuse all refuse through existing shared checks/allocator rules. Numeric signatures and emitted payload bytes are unchanged; asking the numeric allocator for concept refuses without writes. No exceptions are hardcoded from this repository. Test optional field omission/empty array and reject explicit null, wrong type or unknown labels through JSON and CUE. |
| `TestGateConceptRecordOwnership` in `cmd/fathom/rec_new_test.go` | Requested concept proof with no trusted base or missing canonical inventory refuses ErrTrust; trusted exact grant plus existing inventory succeeds; second-file/duplicate/candidate-policy mutations fail through shared ValidateInventory. A legacy duplicate-only success cannot satisfy ENV ownership. Existing numeric-only gate exits/output stay unchanged. |
| `TestConceptRecordCorpusParity` in `internal/recordids/recordids_test.go` | On identical immutable real-corpus bytes, compare accepted REC source and candidate with concepts omitted/false: full Definition tuples, duplicate/ownership diagnostics and native record-id verdicts unchanged. Then enable only in isolated supplied fixtures for feature proofs. Publish source/tree/config/digests and every difference; any existing real-corpus verdict change stops and escalates. No count-only parity or borrowed old native approval. |

Future guarded proof commands (not run here):
`go test -race ./internal/recordids -run '^TestConceptRecord' -count=1`;
`go test -race ./internal/records -run '^(TestConceptRecordNumericCompatibility|TestLaneLaneRecords)$' -count=1`;
`go test -race ./cmd/fathom -run '^TestGateConceptRecordOwnership$' -count=1`.
The implementer first guards declarations, runs relevant existing REC regression
packages under the approved lease, supplies real native gate/corpus receipts and
hands off to the sole wave critic. The foreman owns the full gate. ENV's actual
RecordOwnership consumer proof and reviewed DATA remain separate prerequisites.

## Disposition

WR-CORE-REC/U1 is UNSETTLED as the complete family, syntax/discovery and trusted
opaque-key grant policy. Architect acceptance is required before amendment of
existing Config/schema/interfaces or dispatch. Core bounded publication, local
join validation, this draft and historical native evidence do not answer U1 or
activate a required gate. REC-1 acknowledgement and current-source critic review
remain prerequisites, not presumed facts. No key, range or grant is reserved.

## Decision models

None. Discovery, identity, trust and ownership are deterministic;
no model selects keys, grants, exceptions or validation outcomes.
