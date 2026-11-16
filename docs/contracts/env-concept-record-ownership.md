# ENV concept ownership: deferred checker extension

Status: DEFERRED by architect W18-1 (2026-11-16). The checker-extension proposal
is not approved and is not an implementation or dispatch contract.
Retained proposal source: 27f825061050307c03a4ce3d14e11f5d2652e7fc,
following 01c9711ec9debaac6abc7593a2d1efc89d04b47d.

## Current decision

W18-1 chooses the existing DATA approach: concept IDs are field names of one CUE
struct in the single inventory file, `calibration/environments/concepts.cue`.
Each key denotes one concept; conflicting content for the same key is a CUE
conflict reported by `cue vet`; W18-1 requires the schema's valid/invalid
testdata pair to pin that behavior. No executed schema/vet proof is claimed here.
Ownership uses the lane's single-file path grant, enforced by the existing path
check. This status record creates no inventory, schema, fixture or grant.
W18-2 retains T12's typed expectation on each record; it requires nothing from
the record-id checker for these concept keys.

Existing-mechanism check (W18-3): the keyed data shape, schema/vet conflict checks
and single-file path grant/check solve the current identity and ownership need.
Use that approach. No classifier, literal-CUE discovery, opaque-key grant or
second ID system is authorized now; no such machinery is implemented or
re-proposed by this status record.

Revisit when: REC-6 makes concepts typed records, and not before.
Revisit is a review trigger, not an implementation or dispatch grant.

## Retained proposal: historical and nonnormative

Everything below is the unchanged proposal body at the retained source above.
Its old gap/prerequisite claims, UNSETTLED alternatives, classifier/discovery/grant
shapes, future paths and prospective tests are historical, superseded by W18-1.
They are not current requirements, pending work orders, approvals or reasons to
hold the chosen DATA approach for a checker extension. Do not implement or
re-propose them before the sole REC-6 revisit condition. The existing shared
checker and allocator are unchanged.

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
// Label emitted by the new CUE adapter, NOT a reservation of an arbitrary name.
const FamilyConcept FamilyName = "concept"
// Add to Definition. Existing regex/Markdown definitions leave this false.
CUEConcept bool `json:"cue_concept,omitempty"`
// Add to Config. Omitted/false retains the exact legacy discovery behavior.
Concepts bool `json:"concepts,omitempty"`
// Add to BOTH LaneDeclaration and LaneGrant, and copy through validated grants.
ConceptPaths []string `json:"concept_paths,omitempty"`
```

UNSETTLED FIX1 disambiguation: CUEConcept is a discovery discriminator, not a
new identity namespace or authority. Existing Family.Name is arbitrary, including
"concept" (R:29-34,116-154,1000-1003). Never infer this marker or new permission
from Family alone, reserve/rename a family string, or relabel existing definitions.
Only the shared raw CUE adapter sets it on the new AST occurrences. Authored
fields/JSON supplied by a candidate do not establish discovery or grants; production
callers use the shared Inventory output without dropping or synthesizing markers.
Existing definitions' omitted/false marker preserves their wire representation.
The marker survives trusted snapshots, copies and supported checker serialization;
a lossy transport cannot be used to turn CUE ownership proof into legacy proof.

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
are not definitions. Each occurrence yields FamilyConcept with CUEConcept:true,
the decoded exact string and the key token's original repository-relative
path/1-based line. Existing regex/Markdown discoveries retain CUEConcept:false,
including an arbitrary family literally named "concept". If enabled AST and legacy
regex discovery overlap, retain both occurrences and let the same duplicate check
refuse; do not coalesce, relabel, ignore one source or weaken collision policy.
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
Only when all old tuple fields tie, the new marker sorts false before true;
legacy-only ordering is unchanged. The marker never changes duplicate identity.
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
UNSETTLED FIX1 activation semantics: the trusted Config.Concepts flag controls
new CUE discovery/proof, never the interpretation of an existing family name.

| Effective flag and discovered definition | Required behavior |
|---|---|
| Omitted/false; CUEConcept:false, any Family.Name including "concept" | Original discovery, Legacy matching, shared validation and generic gate behavior unchanged. No CUE scan/grant inference from the name. |
| True; CUEConcept:false, including legacy "concept" | Existing checker/Legacy path still applies, with no implicit CUE grant requirement. |
| True; CUEConcept:true | Require the trusted CUE policy and exact-file grant; never Legacy-exempt. |
| Omitted/false; CUEConcept:true passed to ValidateInventory | ErrPolicy even if trusted.Legacy contains an identical tuple. Do not erase the marker. |

Active ConceptPaths with the flag false also refuse ErrPolicy. LoadTrusted must
carry only the effective trusted policy/grants into this flow. A generic legacy
check/gate may still succeed for an unchanged regex family named "concept" with
the flag omitted/false; that success is NOT ENV ownership evidence. An ENV consumer
must independently require trusted Config.Concepts:true and the shared marked CUE
inventory before treating gate/validation success as its RecordOwnership proof.
Missing activation/trust cannot fall back to a legacy success for that purpose.
No new CLI verb/flag or family-name-based mode inference is proposed. Legacy Check
remains duplicate-only for every configured family, not only numeric families.
Grounds: C:143-202,232-236; R:29-34,414-424,647-727,811-820,919-930,1110-1127.

ValidateInventory still invokes the SAME CheckDefinitions across the entire
inventory and then the SAME CheckOwnership, producing the same opaque validated
result. No family-qualified collision escape or second duplicate algorithm is
added. Exact grandfathered numeric plan pairs remain permitted, but a concept
reusing one of their full IDs or any new duplicate refuses. Existing copies,
stable ordering, full-ID collisions and typed refusals remain (R:263-279,299-311,
452-489; C:143-173; W2-2).

UNSETTLED ownership branch: only for CUEConcept:true, require exactly one trusted
concept grant and its exact path, BEFORE the existing legacy exemption. The marker
must also have the adapter's FamilyConcept label; an inconsistent marked tuple
refuses ErrPolicy. CheckOwnership enforces that grant even if a marked definition
is manually present in trusted.Legacy. LoadTrusted excludes marked CUE definitions
from unrestricted Legacy, but keeps existing unmarked definitions of EVERY family,
including "concept", under their original family/full-id/path rules. Numeric range
matching is unchanged. ValidateInventory/LoadTrusted enforce activation; the
CheckOwnership signature and its existing unmarked ownership role are unchanged.
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
  additive policy/grant/discovery-marker fields, bounded AST discovery, shared
  trust/ownership and defensive copies. No generic discovery plug-in or public door package.
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

### FIX1 exact compatibility control, prospective and UNRUN

Source analysis of accepted R:29-34,1000-1003,834-852,414-424 predicts acceptance
of this witness; native confirmation remains a future implementing obligation:
a trusted Config.Families entry has Name:"concept", Files:["legacy.md"],
Pattern:"^record: (.+)$"; the file has the single line `record: K`, where K is a
supplied nonempty synthetic fixture key. With a real trusted fixture commit and
no ranges/grants, LoadTrusted discovers that record and retains it in Legacy.
Re-inventorying the unchanged family/full-id/path yields valid ValidateInventory
and successful generic trusted gate behavior. This is an existing-behavior control,
not a new key, live config, grant or an executed test. Run both missing Concepts
and explicit Concepts:false. Neither case may require a concept grant, inspect the
CUE envelope or return ErrPolicy merely because the family string is "concept".
Then keep that record while enabling the new adapter with a separate valid CUE
fixture/grant: the legacy record stays unmarked/valid; newly marked CUE occurrences
remain grant-required and cannot use either genuine or injected Legacy tuples.
An overlapping regex/AST occurrence must still fail the shared duplicate check.

| Exact proposed test and file | Required assertions |
|---|---|
| `TestConceptRecordDiscovery` in `internal/recordids/recordids_test.go` | Supplied nonempty K in literal inventory data yields CUEConcept:true with its exact bytes/path/key line; legacy regex/Markdown output stays false regardless of family name. Two declarations of K in separate discovered files with disjoint fields yield TWO definitions and both original locations before unification; shared validation returns ErrDuplicate with both. Repeat in one file and on one line; retain multiplicity. Ignore comments/group references. Unsupported computed/imported/alias/list forms and unreadable/symlink paths refuse rather than omit. Reordering inputs preserves deterministic diagnostics. |
| `TestConceptRecordOwnership` in the same file | One valid trusted data-only ConceptPaths+single OwnedPaths grant with IDRanges: [] accepts supplied K only at the sole inventory path. A mixed numeric/concept lane refuses ErrShape. A unique K in a second discovered file, a missing/mismatched/duplicate grant, read-only-only path or numeric-only grant returns ErrOwnership/ErrShape as applicable, never healthy zero. Put a marked CUE definition in the trusted snapshot or inject it into Legacy: moving it or omitting the concept grant still refuses; no Legacy escape. An unmarked legacy regex "concept" keeps its original exemption. |
| `TestConceptRecordTrust` in the same file | Candidate flag/manifest/schema edits cannot broaden trusted discovery/grants; modified/extra/missing lane bytes refuse. A real raw-blob snapshot with export-ignore/export-subst/replacement-object attempts retains the accepted trust protections. Exercise the exact FIX1 regex Name:"concept" control with omitted and false flag: trusted reload, validation and output remain unchanged, with no CUE path reads/grant requirement. Candidate marker/flag fields cannot manufacture or erase the shared AST classification. Marked CUE definitions with flag false still refuse even if injected into Legacy. Copies/serialization retain markers; mutating returned ConceptPaths cannot mutate the validated grant. |
| `TestConceptRecordNumericCompatibility` in `internal/records/allocate_test.go` | With the effective grandfathered pair present, shared validation and an unrelated existing numeric fixture grant in a separate lane still allocate its lowest unused id. A new duplicate, concept collision with a grandfathered full ID and attempted grandfather reuse all refuse through existing shared checks/allocator rules. Numeric signatures and emitted payload bytes are unchanged; asking the numeric allocator for concept refuses without writes. No exceptions are hardcoded from this repository. Test optional field omission/empty array and reject explicit null, wrong type or unknown labels through JSON and CUE. |
| `TestGateConceptRecordOwnership` in `cmd/fathom/rec_new_test.go` | Requested concept proof with no trusted base or missing canonical inventory refuses ErrTrust; trusted exact grant plus existing inventory succeeds; second-file/duplicate/candidate-policy mutations fail through shared ValidateInventory. A legacy duplicate-only success cannot satisfy ENV ownership. Run the exact FIX1 Name:"concept" Legacy control through the generic trusted gate with flag omitted/false: same successful verdict/output as accepted REC. That verdict cannot satisfy an ENV ownership request without true trusted activation/marked discovery. Existing legacy gate exits/output are preserved for all family names. |
| `TestConceptRecordCorpusParity` in `internal/recordids/recordids_test.go` | On identical immutable real-corpus bytes, compare accepted REC source and candidate with concepts omitted/false: full Definition tuples, duplicate/ownership diagnostics and native record-id verdicts unchanged. Also compare the exact FIX1 custom regex Name:"concept" control, both flag omissions/false and its coexistence with a separately activated CUE fixture. Then enable only in isolated supplied fixtures for feature proofs. Publish source/tree/config/digests and every difference; any existing real-corpus verdict change stops and escalates. No count-only parity or borrowed old native approval. |

Future guarded proof commands (not run here):
`go test -race ./internal/recordids -run '^TestConceptRecord' -count=1`;
`go test -race ./internal/records -run '^(TestConceptRecordNumericCompatibility|TestLaneLaneRecords)$' -count=1`;
`go test -race ./cmd/fathom -run '^TestGateConceptRecordOwnership$' -count=1`.
The implementer first guards declarations, runs relevant existing REC regression
packages under the approved lease, supplies real native gate/corpus receipts and
hands off to the sole wave critic. The foreman owns the full gate. ENV's actual
RecordOwnership consumer proof and reviewed DATA remain separate prerequisites.

## Disposition

WR-CORE-REC/U1 is UNSETTLED as the complete family, syntax/discovery, FIX1
classification/activation and trusted opaque-key grant policy. Architect acceptance
is required before amendment of existing Config/schema/interfaces or dispatch. Core bounded publication, local
join validation, this draft and historical native evidence do not answer U1 or
activate a required gate. REC-1 acknowledgement and current-source critic review
remain prerequisites, not presumed facts. No key, range or grant is reserved.

## Decision models

None. Discovery, identity, trust and ownership are deterministic;
no model selects keys, grants, exceptions or validation outcomes.
