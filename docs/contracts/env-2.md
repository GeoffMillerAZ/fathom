# ENV-2: leaf environment coverage contract

Historical pre-W16 status (provenance only): UNAPPROVED final round-2 repair for an independent critic. T3-1 through T3-4
settle the four prior choices; LAND is still required. Contract only.
No implementation, admission, plan completion, effect verdict or spend approval.
This lane owns this new document only. Later ENV-2 owns exactly
`internal/benchenv/coverage.go` and `internal/benchenv/coverage_test.go`.

## W18-1/W18-2 unpublished keyed-inventory peer amendment

Normative schema source: `docs/contracts/env-1-core.md` at
`24904c52a66ec9703522cca60b4e14fc939f4eea`,
SHA-256 `f63a59fe2bfee66b0c102698749eac2a043a99bec5e0cbdfcda17590dfe2f07e`.
T12-1 governs typed expectations and the pure two-value evaluator; W16-2 and
T3-1 govern the single inventory and Card.ID join; W3-4 governs the
named generator version. W2-3 requires independent LAND for this exact amendment.
W18-1/W18-2 govern one keyed CUE concept struct in one inventory file, with
T12-1/FIX1 expectations per value and no redundant authored key/id. The Go
projection is map[string]ConceptRecord over EVALUATED CUE field names, not raw
source label spellings; empty inventory JSON uses concepts:{}.
Normal correction1 follows existing CUE field semantics: NFD e\u0301 evaluates
to NFC \u00e9 as a field name; string reference values are not normalized and
must equal the actual decoded key exactly. Compatible declarations may collapse;
conflicting30/31 content at that evaluated field refuses in either order. No ENV
normalizer, alternate identity rule or reference rewriting is added. The stored
invalid.cue negative stays fully commented; tests extract/uncomment its exact
witness into owned scratch for explicit vet/source-validator refusal. Core's
CUEFieldSemantics and CommentedNegative tests pin these exact cases.
CUE/schema plus the valid-invalid pair and cue vet pin conflicting content;
compatible repeated declarations may unify. One exact-file lane path grant,
enforced by pathcheck, owns the inventory. It needs nothing from the record id
checker. Other authored ENV records retain their existing record-id laws.
W18-3 disposition: the existing data shape/schema/checks suffice; use them,
without new machinery. No classifier, CUE-literal discovery, opaque-key grants,
numeric policy or RecordOwnership prerequisite applies to concept keys. The
shared concept-record extension is deferred to REC-6 under its existing owner.
Historical source pins below remain provenance, not normative alternatives.
The preceding W16 core was `2ada35b791d8115fc05e563ba3af571e624a9895`,
SHA-256 `2ac44ac36988aacc9783e76e676b5d2abdd5006afb4ded36241f775fb7047981`
(provenance only).
The old core approval and held peer review do not approve these W18/T12 bytes.
Fresh independent review and a new handoff are required; F1 is not self-closed.
This amendment supplies no implementation grant, inventory data or path grant,
runtime/native evidence, gate result or plan-row proof. Other authored ENV
records retain their separate REC discovery/grants prerequisite.
Integration remains HOLD pending the controller's exact-source final review.

The historical R3/T8 Matching repair remains provenance; its Dn(1,30) before
Cn(1,30) union is unchanged. This amendment's review is separate and confers no
broader publication, implementation or integration approval.

## Authority and intent

BE-D2/D3 require independent native evidence, matched negatives and unique origins;
BE-D4 requires scale vectors and independent environments; BE-D9 derives adequacy
from inputs (`docs/design/bench-environments.md:79-144,237-245`). P2/P3/P17
require deterministic computation and visible unknowns
(`docs/intent/principles.md:12-17,63-64`). I56/I62 prohibit implicit foreign
inputs and model proof (`docs/intent/invariants.md:85,90`). The validity-trigger
reopening is already documented at `docs/design/bench-environments.md:23-28`.
No additional trigger fires here. No future is built and no existing compatibility
surface changes. ADR-109 remains proposed (`ADR.md:7132-7137`).

Apply W3-4: named closed enums, explicit lower snake case JSON, comparable keys,
non-nil result collections, typed errors and no context on a pure call. W3-2
assigns the actual RED tests to the later implementing lane. W2-3 requires a LAND
critic and governing text, with ungoverned points escalated. These ruling IDs
refer to the runner's supplied authority, not new repository files.

The original immutable peer is ENV-1 commit
`7f22be5ac0bdff0ddfc3f09f61a2d57d83489321`. Its final round-2 peer is
`44d4f45184996c505e0bcc96d5ab5a8c65da56e4`, still UNAPPROVED with final critic
pending. Peer references below use `docs/contracts/env-1.md@44d4f4518:<lines>`.
The entire T3 mapping/port/trust/subtype/result section is byte-identical between
those commits. Historical pre-T12: all four leaf declaration blocks matched both peers exactly after
whitespace normalization and removal of this document's package/import preamble.
Their historical stable combined SHA-256 is
`0d66ea3561f8d5bc0051ca5407cc7b94c9b6c96355ccd35f246bb1a16c00d030`
(join normalized blocks with LF, no final LF). Peer R2 adds NewModeError and
changes downstream CLI/error contracts; none changed that historical leaf's inputs or port. T12-1 now amends the
pure evaluator inputs explicitly, while conserving every native port declaration.
This is compatibility evidence, not peer approval. Authored inputs, T3 ports and
derived results remain specified dependencies, not delivered APIs. T3 supersedes
ENV-1/U2/U3 and ENV-2/U1/U2. Downstream peer policies remain their owners' work,
never additional leaf prerequisites. No native adapter is implemented here. Final independent critic remains required
before approval; remaining defects return HOLD, with no automatic third repair.

## Grounded decisions

| Point | Contract | Authority |
|---|---|---|
| ENV-2/D1 | A pure leaf, no generator, CLI, model, store, filesystem, process, network or registry ownership. No new dependency and no import from the language door. | ENV-2 ownership, `docs/plans/bench-environments.md:36-37`; BE-D9, `docs/design/bench-environments.md:237-245` |
| ENV-2/D2 | Check full language/vector/prediction tuples separately. Common minima are the authored minima, each at least 30. Matched negatives and supplementary floors cannot be replaced by surplus elsewhere. | BE-D2, `docs/design/bench-environments.md:79-109` |
| ENV-2/D3 | Derive cell counts and portfolio sufficiency; unknown, stale, clone-inflated and absent inputs cannot pass. Coverage is necessary, not BEN-5 admission. | BE-D3, `docs/design/bench-environments.md:118-125`; BEN-5, `docs/plans/proof-program.md:42` |
| ENV-2/D4 | Public and constructed coverage are separate requests. Smoke contributes nothing. Deduplicate pins/replicas and require two independent environments for each claimed axis/class. Public outcomes also require two public repository origins. | BE-D1/D4/D5, `docs/design/bench-environments.md:70-77,127-144,185-195` |
| ENV-2/D5 | Catalog closure includes every BE-D2 closure domain even for deferred targets, without implementing those targets. No parallel BEN card or admission registry. | `docs/design/bench-environments.md:111-116`; BN-D1, `docs/design/proof-program.md:52-55` |
| ENV-2/D6 | Retain assigned-arm populations. No W4/W5 use, action, model or WPF outcome input reaches this leaf. WPF preflight and reading liveness remain consumer gates. | `docs/design/bench-environments.md:53-60,210-215`; `docs/plans/bench-environments.md:23-30` |

## Exact T3-selected interface

Except the core-owned Metric vocabulary, all declarations in the following Go blocks belong in
`internal/benchenv/coverage.go`. The sole non-standard import is the ENV-1 authored
`schema/benchenv` data package. Shared authored data stay there. Leaf-specific
ports/results stay here; no public-door re-export is requested. The `env` prefix
means that schema package, not the existing `internal/capability` extension API.

```go
package benchenv

import (
    "time"
    env "github.com/geoffmilleraz/fathom/schema/benchenv"
)

type CoveragePurpose string
const (
    PurposeCoverage CoveragePurpose = "coverage"
    PurposePublicOutcome CoveragePurpose = "public_outcome"
)

type CoverageRequest struct {
    Records env.EnvironmentRecords `json:"records"`
    Catalog env.Ref `json:"catalog"`
    Tier env.Tier `json:"tier"`
    Purpose CoveragePurpose `json:"purpose"`
    At time.Time `json:"at"`
}

// Caller-selected, independently verified, immutable in-memory evidence.
// NOT implemented by decoding authored ENV records or trusting their labels.
type CoverageEvidence interface {
    Catalog(ref env.Ref) (CatalogProjection, error)
    Environment(head env.EnvironmentHead) (EnvironmentProjection, error)
    Concept(head env.EnvironmentHead, requirement env.Requirement,
        witnesses []env.Witness, at time.Time) (ConceptProjection, error)
}

func CheckEnvironmentCoverage(inventory env.ConceptInventory, snapshot CoverageSnapshot) (CoverageResult, error)

type CatalogDomain string
const (
    DomainCapability CatalogDomain = "capability"
    DomainFamily CatalogDomain = "family"
    DomainCard CatalogDomain = "card"
    DomainPrimitive CatalogDomain = "primitive"
    DomainLifetime CatalogDomain = "lifetime"
    DomainBeyond CatalogDomain = "beyond"
)
type CatalogKey struct {
    Domain CatalogDomain `json:"domain"`
    ID string `json:"id"`
}
type CatalogEntry struct {
    Key CatalogKey `json:"key"`
    Requirements []env.Ref `json:"requirements"`
}
type ConceptObligation struct {
    Requirement env.Ref `json:"requirement"`
    Groups []MeasureGroup `json:"groups"`
}
type CatalogProjection struct {
    Source env.Ref `json:"source"`
    Inventory []CatalogKey `json:"inventory"`
    Entries []CatalogEntry `json:"entries"`
    Concepts []ConceptObligation `json:"concepts"`
}

type EnvironmentProjection struct {
    Head env.Ref `json:"head"`
    Families []string `json:"families"`
    PublicRepositories []string `json:"public_repositories"`
    Scale env.Scale `json:"scale"`
}

type OriginKey struct {
    Family string `json:"family"`
    Root string `json:"root"`
}
type SampleKey struct {
    Origin OriginKey `json:"origin"`
    Site env.Anchor `json:"site"`
}
type ProjectedSample struct {
    WitnessID string `json:"witness_id"`
    Key SampleKey `json:"key"`
    Match OriginKey `json:"match"`
    Polarity env.Polarity `json:"polarity"`
}
// Group is a tuple, never a delimiter-joined string.
type MeasureGroup struct {
    Tool string `json:"tool"`
    Kind string `json:"kind"`
    Language string `json:"language"`
    Format string `json:"format"`
}
type ProjectedMeasure struct {
    Metric Metric `json:"metric"`
    Group MeasureGroup `json:"group"`
    Members []SampleKey `json:"members"`
    Value *int64 `json:"value"`
}
type ConceptProjection struct {
    Environment env.Ref `json:"environment"`
    Requirement env.Ref `json:"requirement"`
    Checker env.Pin `json:"checker"`
    Receipts []env.Ref `json:"receipts"`
    Samples []ProjectedSample `json:"samples"`
    Measures []ProjectedMeasure `json:"measures"`
}
```

`SampleKey`, `CatalogKey`, `OriginKey` and `MeasureGroup` are comparable
structs under ENV-1 Anchor/RepositoryPin definitions
(`docs/contracts/env-1.md@44d4f4518:188-194,249-255`). No NUL or
other sentinel encoding is used for identity. The exported structs are typed port values, not additional authored
CUE records. No constructor accepts a JSON claim of verification.

### T12-1 pure evaluation and measured snapshot

T12-1 supersedes the former CheckEnvironmentCoverage(req, evidence) function
signature, not the unchanged CoverageEvidence interface or any projection type.
The existing native port remains the only projection interface. Its owner still
verifies bytes/history before preparation; the pure leaf never receives a port.
These new value declarations and preparation helper belong only to the later
`internal/benchenv/coverage.go` owner, with tests in coverage_test.go. No new file,
package, native transport, implementation or present execution is granted.

```go
type SnapshotConcept struct {
    ClaimID string `json:"claim_id"`
    Requirement env.Ref `json:"requirement"`
    Projection ConceptProjection `json:"projection"`
}
type CoverageSnapshot struct {
    Request CoverageRequest `json:"request"`
    InventorySource env.Ref `json:"inventory_source"`
    Catalog CatalogProjection `json:"catalog"`
    Environments []EnvironmentProjection `json:"environments"`
    Concepts []SnapshotConcept `json:"concepts"`
    Refusals []EvidenceRefusal `json:"refusals"`
    Invalid *CoverageError `json:"invalid"`
}
func PrepareCoverageSnapshot(inventory env.ConceptInventory, req CoverageRequest,
    inventorySource env.Ref, evidence CoverageEvidence) (CoverageSnapshot, error)
```

The normative evaluator is
`CheckEnvironmentCoverage(inventory env.ConceptInventory, snapshot CoverageSnapshot)`
with exactly those two value inputs and the existing (CoverageResult, error)
return. No context, callback, port, function, raw error interface, lazy ref resolver,
clock, filesystem, hashing of fingerprints or I/O is hidden in the snapshot.
Inventory is supplied independently, never reconstructed from measured metrics.
The schema owns expectation data and its one canonical metric/unit/minimum
relation. The leaf's Metric and constants below are aliases, not another enum.

Explicit owner data flow:

1. The data owner's exact-file lane path grant is enforced by pathcheck. The
   owner validates the one keyed CUE struct with the core schema/cue vet before
   decode and supplies that critic-reviewed inventory plus source Ref. Conflicting
   content at one key refuses; compatible declarations may unify. No contents,
   grant or recordchecker prerequisite for concept keys is supplied here. Existing
   record-id laws for other authored ENV records remain independently mandatory.
2. Native/catalog preparation resolves that source's exact bytes and every
   Card/checker/catalog/native receipt/history dependency; it verifies all refs
   by ID/path/SHA256. InventorySource.Path is the one proposed production path;
   its ID is opaque, not a new family. Catalog.Source binds this complete source
   and its referenced mapping/pins. Changing inventory expectation bytes without
   rebinding the source is stale_pin; unavailable bytes are unknown_evidence.
   Existing CheckCardClosure receives actual shipped keys and approved Cards,
   then ValidateEnvironmentConceptJoin receives all approved Card.IDs. Failure
   stops before projections. Native owners verify each independent expectation
   set, units, minima and aggregate/subtype selection against the cited design and
   exact Card/CheckerContract. These are acceptance prerequisites, not hidden
   semantic work delegated to the pure snapshot.
3. PrepareCoverageSnapshot validates typed Request, inventory and InventorySource
   and nonnil port first, using core ValidateConceptInventoryValue and the same
   pure validation rules as evaluation. It calls only existing Catalog, Environment and Concept methods,
   in the exact stage/binding order below, once per complete binding; it deep
   copies request, all projections and refusals. Catalog refuses before later
   calls exactly as before. Expected refusals are stored in Refusals with their
   exact existing scoped Path; unknown port errors become Invalid with typed
   evidence_unavailable and safe path, no raw error text/cause. A known structural
   projection violation becomes Invalid with invalid_projection. A nonnil Invalid
   stops preparation and later stages; return the same typed error along with
   the captured snapshot. Expected refusals alone return nil error. Invalid
   input similarly produces the typed invalid_input value and no calls. Every
   slice is nonnil, Invalid is null on successful preparation. No writer, native
   execution or source read is performed here. It uses the independent inventory
   to validate each measured stage before a later port call, preserving every
   existing first-error/zero-later-call assertion. It never derives an expectation
   from observations. Assembly and evaluation share the same pure stage checks.
4. The caller passes its independently validated deep-copied inventory and that
   exact snapshot to CheckEnvironmentCoverage. Snapshot has no expected-metric
   field, pass bit, authored truth or second inventory; InventorySource is
   provenance, never authority to read a file. Constructors accepting authored
   JSON as a verified snapshot are forbidden. Trust is the same operator-selected
   native-owner boundary as T3-2; a pure value signature cannot prove byte reads.
   The leaf calls core ValidateConceptInventoryValue for inventory
   shape/units/minima/duplicates, then checks
   present requirement/group reference consistency without parsing CUE or
   calling a recordchecker. The actual complete Card closure and pure core join have already
   run under their owner before projection, as required above. The evaluator does
   not rerun full claim-completeness join as a precheck: its existing missing
   requirement/ref diagnostics remain computed missing_concept, never converted
   into invalid_input. Present requirements must resolve Concept in inventory and
   match its Card/CheckerContract group; unknown Concept gives invalid_input at
   /records/requirements/i/concept, mismatched group ref at its existing member.
   Native owner CheckCardClosure and complete core join remain mandatory; fake
   omission tests prove only the unchanged aggregation/refusal behavior.
   For catalog refusal/Invalid snapshots the existing earlier-stage result wins;
   do not fabricate a join from an unavailable catalog. The leaf rechecks every
   available projection binding, member, shape and captured error/refusal; missing
   or extra snapshot slots, contradictory success/refusal/Invalid values, or
   duplicate bindings are invalid_projection at the canonical evidence binding.
   Snapshot.Invalid is validated for code/path and reported at its recorded stage;
   an earlier-stage invalid request always wins. No native calls occur in evaluation.

All later port-call/fake assertions apply to PrepareCoverageSnapshot followed by
pure evaluation; they are not permission for the evaluator to call an interface.
The staged diagnostics and exact JSON/Matching unions remain unchanged. Copies,
refusals and Invalid encode only measured values or typed failures, never a claim
that the inventory bytes or native provenance were verified by the leaf.
A snapshot containing successful projections and a refusal for a DIFFERENT binding
is valid; a catalog refusal has no later projections/refusals. Successful Concepts identify claim and exact requirement; refusals identify
the same binding through their scoped Path and have no fabricated projection.
Missing binding values are represented only by that binding's scoped refusal,
not zeros.

For each requirement, resolve its opaque Concept byte-for-byte in inventory,
then use ALL that record's Expectations. Groups still come from the unchanged
CatalogProjection.Concepts, using the existing metric-specific group applicability
law. The expected set is this inventory metric set crossed with its applicable
native-declared groups, never the returned metric list or parsed checker-ref ID.
No group can disappear because no observations were returned. Each present measure
must be in that exact set, with the existing Members/Value law; unknown/unrelated,
extra, duplicate or missing metric/group is invalid_projection. The canonical
metric's unit determines identity counting, with no scalar conversion. Cardinality
uses the unchanged native normalization/deduplication rules. Effective Required
is the inventory Minimum, subject to its canonical lower bound (history200 and
cited aggregate presence1/repository3/symbol3 included); common sides use their
unchanged authored minima>=30. Never reduce a requirement because measured data
are short or unknown. Known empty Members gives0; missing native backing gives
unknown_evidence/null, never0. All other BE-D2 predicates and T3-3 subtype rules
are unchanged. A claimed output subtype cannot reuse aggregate presence1 records;
its selected two metrics each require30 in separately reviewed inventory data.

Inventory validation errors become *CoverageError{Code: ErrorInvalidInput,
Path: "/inventory" + local pointer}; exact CUE/Go validation reasons and pointers
are core-owned. Join errors preserve their existing /records or /inventory
pointers. /inventory is the typed source-order diagnostic view (expectations
retain source indices), not /evidence/catalog/inventory, which is the unrelated
closed-domain catalog roster. Validate request first, then inventory shape,
identity/reference rules, then measured stages; within each use the existing
pointer/reason law. No omission can yield PASS. No result aliases caller or port
memory; snapshots and results may be read concurrently after assembly. Sort
private copies; permutations of semantic sets preserve byte-identical result JSON,
while diagnostics on malformed source inventory use its explicit source indices.

### T12-1 future implementing assertions (W3-2)

These are future semantic tests, never run-Go proof in this document amendment.
Use the existing two-environment F fixture, common30/30, G0 and all unrelated
requirements/portfolio floors healthy. P and Q retain their exact diagnostic
meaning below. K and L are supplied arbitrary distinct keys, not allocations.
In inventory pointers, <K> means the supplied key escaped as one RFC6901
segment, never an array index. Core owns TestConceptInventory_MapKeys, TestConceptInventory_CUEFieldSemantics
and TestConceptInventory_CommentedNegative.
Core owns TestValidateConceptInventory_Expectations,
TestValidateConceptInventoryValue and TestValidateEnvironmentConceptJoin_ExpectationScope. Native/catalog owner extends
TestEnvironmentCatalog_InventoryJoin under a separate grant: verify real inventory
bytes and cited expectations, actual CheckCardClosure then full-ID pure join, then
projection then assembly. Changed expectation bytes with stale Source refuse
stale_pin at /evidence/catalog; unavailable inventory bytes give unknown_evidence
there, both with zero later calls. A fully repinned separately reviewed source
must deliver its changed expectations, not a cache from the old mapping. Preserve
all existing native-conformance/predicate assertions and raw manifests.

| Exact test in coverage_test.go | Exact assertions |
|---|---|
| `TestEnvironmentCoverage_ExpectationBinding` | Give K the aggregate output expectation set from T3-3 (output_cases30, question_cases30, long/short1, route/rank/table1, each unit and exact exception citation from core), L cross_file_paths30/paths, and the same G0. Returning only cross_file_paths30 for K refuses *CoverageError ErrorInvalidProjection at P/measures/cross_file_paths/0, normalized UNFIT Cells:[] Issues:[], no partial counts. Bind the same measured cross-file set to L with all other floors met: PASS/nil, counts30/30, MeasureResult cross_file_paths observed30 required30. Remove output_cases while all other expected metrics pass, leaving no unexpected metric: ErrorInvalidProjection at P/measures/output_cases/0, same empty error result. Also supply unrelated known cross_file_paths30 with output_cases still missing: the unrelated metric is refused even though it has30. Consistently permute K/L through inventory, groups, records, catalog, witness refs and snapshot bindings, preserving each expectation's semantic attachment: keep claim/requirement/witness IDs fixed and repin all affected refs: result JSON is byte-identical. Swap only the two expectation attachments: the formerly valid cross-file snapshot refuses its now-unexpected cross_file_paths. Rename arbitrary checker-ref IDs consistently without changing inventory: result bytes unchanged, no dispatch on those IDs. Every check uses zero evaluator port calls. |
| `TestEnvironmentCoverage_ExpectationMinima` | Omitted cross_file_paths minimum validates to30. Measured29 -> exact existing C(cross_file_paths,29,30,G0) and N(...,1), INCOMPLETE/nil; measured30 healthy twin PASS. Direct Go Minimum0/29 (even with unrelated valid citation) -> ErrorInvalidInput /inventory/concepts/<K>/expectations/0/minimum, empty error result. Every canonical below30 metric uses its exact matching cited floor and unit: at that floor healthy PASS; at floor-1 exact existing C(metric,floor-1,floor,G0) and N(...,1), common30/30 retained. Remove a required typed citation or supply a wrong typed citation/unit -> core-owned invalid_input pointer, no silent default. Raw null/untyped members are core source-validator cases and must refuse before this typed API is called. History199 -> C(history_commits,199,200,G0), history200 PASS. Raising inventory cross-file minimum to31 makes measured30 -> C(cross_file_paths,30,31,G0). Selected subtype metrics29 still refuse required30, aggregate presence1 never lowers them. Missing native backing stays R(unknown_evidence,P/receipts/0) and null counts, while present empty members produce C(metric,0,required,G0); neither can PASS. |
| `TestEnvironmentCoverage_SnapshotBoundary` | Capture exact old Catalog->Environment->Concept order, once per binding, private per-call copies and existing early-refusal/first-error assertions using PrepareCoverageSnapshot. Then replace the fake with a panic-on-call sentinel and evaluate only inventory+snapshot: no calls or I/O. Repeated and concurrent evaluations yield byte-identical JSON and unchanged input bytes, even after mutating the fake's original backing slices after assembly. Inspect all snapshot fields recursively: values only, no port/interface/callback/error interface/context or authored pass fields. Missing/duplicate/extra success slot, success plus refusal for SAME binding, malformed/out-of-scope refusal and contradictory catalog failure/later slot each refuse ErrorInvalidProjection at that exact canonical evidence root. Expected refusal for a distinct binding retains the existing complete issue union. Unexpected native error is normalized to typed evidence_unavailable at its invocation root, raw cause absent; zero later preparation calls, no partial result. nil/empty typed Expectations refuse /inventory/concepts/<K>/expectations before measured evaluation. Missing/null/untyped raw expectation fields must first refuse in core source validation, not be fabricated as unrepresentable typed Go values. |

### Trust boundary, references and normalization (ENV-1/U3)

Settled T3-2: the caller supplies an operator-selected native-evidence adapter,
built over an already read, immutable snapshot. Its methods are deterministic,
non-writing and perform no I/O. I/O and native execution occur in the owning
adapter's preparation stage, which may use context. PrepareCoverageSnapshot calls
only this port; the evaluator receives only inventory and measured value snapshots. No default implementation synthesizes projections from `Witness`,
`C6Row.Qualifies`, `VetReceipt.Verdict`, an oracle name or a claimed digest.
A malicious trusted adapter is outside this trust boundary; authored records are
inside the untrusted boundary. A digest proves identity only after independent
resolution and verification of the referenced bytes.

Before returning a projection the adapter MUST:

1. Resolve references by ID, canonical relative path AND SHA-256 of exact bytes.
   Bind the environment head and requirement bytes, all repository full commits,
   source policy, licence inventory/member receipts, toolchain/image pins,
   checker version/digest and native receipt digests. Rehash every dependency;
   cache only under the complete immutable binding. A changed binding is stale,
   absent/unreadable bytes are unknown. Referenced native evidence must describe
   the same measured source, not whatever checkout happens to exist now.
2. Verify independent provenance by the selected native tool, native format key,
   subject tests or confirmed gold anchors required in the table below. Reject
   fathom-produced truth and model primary labels. A `producer` field alone is
   insufficient. Seed truth must predate ingest and have the specified native
   checks. Reuse the original oracle; do not build a new ENV oracle.
3. Normalize lineage from verified history/clone/template receipts. `Family` is
   the canonical public repository origin, or constructed topology/template
   ancestry. `Root` is the earliest independent task/site origin in that family,
   stable across descendant fixes, moved/copied clones, padding and pin changes.
   A positive and its safe/absent matched negative have distinct sample origins
   but share a verified `Match` pair origin. Strings are opaque verified keys,
   not witness IDs, authors' `Origin` refs, content hashes alone or random seeds.
   An unverifiable ancestry refuses; never guess independence.
4. Project the exact per-concept predicates and metric memberships below from
   the native facts. A sample with an absent prerequisite is omitted, not marked
   `qualified:true`. Supplementary event keys are independently normalized task,
   site, commit, symbol or question identities, as the table specifies. Metrics
   with different populations need not use common positive/negative anchors.
   They still carry normalized `SampleKey` identities and native receipt support.
5. For foreign-input ages use the supplied `at`, the producer's declared bound
   and the format's frozen native rule. No ambient clock. ENV's 30-calendar-day
   *reading* freshness rule is not applied here. A deliberately stale envelope
   is a valid negative example if its negative classification was independently
   checked; a stale evidence receipt that purports to prove that example is not.

The leaf checks the returned binding against its exact request, ensures each
projected common sample names one supplied witness with the same anchor/polarity,
and accepts no extra or multiply bound witness. Reference mismatches produce
`stale_pin`; missing or unsupported evidence produces `unknown_evidence`.
Projection shape/contract violations produce typed `invalid_projection` errors.
The adapter supplies independently checked metric memberships, not an authored
aggregate count. `Value` is nil for all metrics except `history_commits`; that
metric has nonnegative verified reachable-unique count and empty Members.

Environment `Families` contains every member's ancestry root. Public repository
keys use the existing URL normalization law
(`internal/benchprep/vet.go:428-448`); an origin repair/rename is resolved by the
adapter, never guessed from different strings. Overlapping Families make
connected components across environments. Only one representative per component
counts. Constructed families need independently verified different topologies and
templates: different seed, pin or wrapper ID is insufficient.

### W16 single inventory and owner wiring dependency

`calibration/environments/concepts.cue` is the sole proposed production inventory,
with `inventory: #ConceptInventory & {...}` exactly as the pinned core defines.
Its concepts member is one CUE struct with opaque #Text field names as the
sole authored concept ids after existing CUE field evaluation; values carry
T12/FIX1 expectations and Card or
BE-D2/BE-D9 citations. It also holds the read-only Card.ID/checker-contract groups.
Those groups reference the same struct keys; they are not a second inventory.
Core projects Concepts as map[string]ConceptRecord, never an array with a Key/id
field. Map serialization/traversal orders evaluated map keys by UTF-8 bytes;
error pointers escape those actual keys with RFC6901 (~ to ~0, / to ~1).
References are string values, compared exactly to decoded keys without silent
normalization: source label e\u0301 evaluates to \u00e9, an NFC reference resolves,
and an NFD reference does not. The Go map preserves evaluated names, never a
promise of raw source-label bytes or declaration multiplicity. Thus
key a/~ λ has /inventory/concepts/a~1~0 λ as its leaf/join pointer base. No fixed length, contract
roster, generated key enum, fallback inventory or new Card API is permitted.
Adding a key is a critic-reviewed data change, not a contract amendment. The
existing BE-D2 supplemental metric predicates, counts and eligible strata remain
mandatory for their cited obligations. Named cases below are obligation examples
and bindings, never a universal enum or required inventory roster. The data critic
checks each key's justification and the actual native obligation binding; changing
its spelling cannot remove that obligation or lower its floor.

Native/catalog preparation resolves and verifies the actual complete approved
Card and CheckerContract bytes and mapping pins, reuses unchanged
`CheckCardClosure(shipped []string, cards []Card) ([]string, error)`, then calls
core's pure `ValidateEnvironmentConceptJoin(records, inventory, cardIDs)` with the
complete exact approved Card.ID set before any native/catalog projection. Use the
core-owned `EnvironmentRecords`, `ConceptInventory` and `[]string` parameters
exactly. No title/surface aliases or filtered IDs. A missing-card/orphan-surface/
duplicate-card/conflicting-claim closure error stops before the join; a join error
stops before projection. The join checks complete Card/group refs and every
required `(CheckerContract.ID, concept key)` once in each claim's exact stratum;
it verifies no native truth, group semantics, counts or admission.
CatalogProjection.Source binds the complete single mapping and its referenced
Card/catalog/checker-contract refs and pins. Unavailable bytes remain
unknown_evidence; readable verified missing declarations are absent_capability.
Neither a shape-valid inventory nor empty arrays establish actual closure.

W18 removes the concept inventory's former WR-CORE-REC dependency. The one
separately granted data file is checked by the existing lane pathcheck; a changed
or new second/ungranted inventory path refuses. Its keyed CUE struct/schema and
exact valid/fully-commented-invalid testdata pair pins conflicting content by
extracting/uncommenting the negative into owned scratch for explicit cue vet. Compatible
repetition is not a duplicate-record error. No recordchecker extension,
representation, discovery, effective numeric policy or RecordOwnership result is
needed for concept keys. Other authored ENV records retain their REC checks.
The data owner still needs that one exact-file path grant and critic review; no
contents, grant or actual production keys are supplied by these contracts.
WR-CORE-CATALOG then requires native/catalog owners to supply
actual pinned Card/checker bytes, accepted native fixture manifests and the
closure -> pure join -> projection proof. In particular, the native/catalog
owner must verify the inventory's typed expectations against the cited BE-D2
obligations and exact native/group/subtype predicates in its pinned checker
contract, with complete expected projection bytes. The inventory alone supplies
expected metric/minimum/unit values to the leaf. No key-name switch, checker-ref
lookup, inferred observation or hidden side channel substitutes for owner proof.
Missing inputs stay unproven. No command DTO, private execution field,
context side channel or port is changed.

### Catalog projection and closure (ENV-1/U2)

Settled T3-1: ENV owns only a pinned read-only concept/group mapping keyed by
approved CTR-13 Card.ID. Native preparation loads the authoritative shipped
surfaces and cards and reuses
`CheckCardClosure(shipped []string, cards []Card) ([]string, error)` before
projecting CatalogProjection. Reuse exactly Card and ProofRef with amended JSON
and typed errors (`docs/contracts/ctr-13.md:212-249,492-539`). The extension
`internal/capability.Registry` stays configuration policy, never a replacement
card model (`internal/capability/capability.go:5-18`). Implementation availability
is explicitly pending; the approved card contract is settled.

Requirement.Capability and CoverageClaim.Capability equal Card.ID, never a surface
or title. Requirement.Card is the canonical ENV-1 Ref with ID equal to Capability;
Requirement.CheckerContract is a separate Ref to the native obligations and
normalization/subtype metadata. Checker pins code; CheckerContract pins the
contract bytes. These SHA-256 values need not equal. ConceptObligation.Requirement
resolves to that exact Requirement, including both refs. CatalogProjection.Source
binds the complete mapping and all referenced card/checker-contract pins.
DomainCapability.ID is the exact Card.ID. Other domains retain publisher keys and
join through the same requirement refs, never an alias card registry.
ENV-1 owns these authored inputs
(`docs/contracts/env-1.md@44d4f4518:230-248,300-330,597-637`).
Native preparation checks raw card, mapping and checker-contract bytes before
Catalog succeeds; a shape validator cannot do so. Unavailable dependency bytes
refuse Catalog with unknown_evidence, never absent_capability. A readable verified
source with a required checker declaration known absent instead refuses Catalog
with absent_capability. Both stop before Environment/Concept; the exact one-issue
results are frozen below. The pure leaf compares only successfully returned
catalog sets/bindings.

BEN-5 alone consumes coverage through approved SubjectInput/AdmitSubject,
including EnvironmentReceiptPath and amended TaskFamily/Split/AdmissionState
(`docs/contracts/ctr-13.md:250-296,492-539`). ENV dev explicitly maps to
benchledger.SplitDevelopment (development), held-out to SplitHeldOut. Admission
is downstream, never a prerequisite for computing its own coverage receipt.
Respect REC-1 -> heads -> #299 card-data serial grants and retained BEN/hook turns
(`docs/contracts/bench-consolidation.md:33-82`;
`docs/contracts/records-integration.md:36-60`;
`docs/contracts/command-reservations.md:39-69`). None grants an edit here.

Catalog entries are **all** BEN-3 capabilities; the seven families PLAN, HAZ, FIX,
LOC, REV, LANE, ASK; the eleven ordered card rows including both what-if tiers;
the added guardrail and lifetime cards; primitives 1 through 8; RL-1 through
RL-12; and every V-N/A-N target in beyond-determinism section 3. Preserve the
publisher's opaque card keys, including the V-N10/A-N1/A-N2 relationship. Do not
invent new BEN card IDs from titles or double-count the relationship. Sources:
`docs/design/proof-program.md:85-99,204-223` and
`docs/design/beyond-determinism.md:33-53`.

Inventory is the complete independently verified source roster. Entries is its
requirement projection. The leaf requires exact key-set equality (no missing,
extra or duplicated keys); deleting an Entry while retaining its Inventory key
is absent_capability. Changing or truncating the source Inventory itself must
be refused by the trusted catalog adapter against the pinned source roster.
This roster projection is not a separately maintained ENV registry.

Every entry has at least one exact requirement ref; every ref resolves to a
Requirement with concept, both minima, full stratum and checker pin. References
may be shared across domains. Every concept/group/expectation required by the single
critic-reviewed inventory must exist in the requirements projection, without a
contract-fixed cardinality or list of keys. The catalog publishes the per-capability mappings;
ENV never infers those mappings from the illustrative Cards column. The adapter
refuses known missing source/checker declarations and unavailable dependency bytes
at Catalog, distinguishing absence from unknown. After Catalog succeeds, the leaf
rejects missing entries, empty mappings and references, and claim capability absent
from the pinned projection. An arbitrary subset supplied as the whole catalog cannot pass.
A deferred target needs its requirement and checker contract, not runtime delivery.

For a claim, select the authoritative capability entry's requirements matching
its full stratum. Its `Requirements` set must equal that set, with at least one
member. Omitted refs are `missing_concept`; extra or wrong-stratum refs are
`invalid_input`. If no matching mapping exists, report `absent_capability`.
A catalog-complete inventory does not require coverage *claims* for all targets.
Only requested cells are evaluated, so an unscheduled target does not block
an unrelated card's coverage. Missing closure still prevents overall PASS.

## Exact metric vocabulary and per-concept projection

`Metric` aliases the schema-owned named string and exported constants below. The core owns the complete closed vocabulary and metric/unit relation; these
aliases preserve existing port spelling, never arbitrary map keys. Metrics are local
coverage dimensions, not another BEN capability catalog. First-column names are
BE-D2 obligation cases/bindings, not normative inventory keys. Actual opaque keys
and Card.ID/group mappings come only from the single reviewed inventory. The
single inventory supplies typed expected metric/minimum/unit obligations for each key;
renaming or adding a key never waives a cited BE-D2 predicate, count or stratum.
No new metric or native projection law is inferred for an unbound key: its owner
must supply the exact existing obligation binding and native evidence first.

Every row additionally requires 30 independent positive and 30 distinct matched
negative samples, in the full requested stratum. All table counts are inclusive
floors and count unique normalized members. Raising an authored common minimum
raises that side's floor. A minimum below 30 is invalid_input, matching the
ENV-1 schema contract; direct Go construction cannot bypass that bound. Each explicitly claimed
tool/kind, language or format is a separate required group, never a group inferred
only from observed evidence. The authoritative requirement/checker projection must
supply those group obligations even if it has zero observations. T3-selected
representation: CatalogProjection.Concepts has one ConceptObligation per
requirement, with the complete group set resolved from its bound native checker
contract. For ungrouped concepts Groups is the singleton all-empty tuple.
ConceptProjection includes a measure for each required group and metric. The
leaf compares those sets exactly. Omitting a group is invalid_projection, never
an empty pass. A measure with empty Members is known zero, not missing evidence.
Only hazards uses Tool+Kind, polyglot-generated uses Language, guardrails uses
Kind, and foreign-inputs uses Format. All unused group fields must be empty.
Polyglot boundary_sites is ungrouped; its language_sites has the language groups.
Guardrail deletion metrics are ungrouped; rule_labels has the Kind groups. All
other metrics are ungrouped. The catalog must include the empty tuple where an
ungrouped metric applies. Every group-requiring concept has nonempty declared
groups; polyglot has at least three distinct nonempty languages. No wildcard
group or optional observed-only group is allowed.

| Obligation case/binding (not a key roster) | Metrics and exact native predicate before membership; independent source |
|---|---|
| `false-plan` | `absent_symbols` >=1 and `unwired_entries` >=1 within 30 false rows; 30 true rows are matched negatives. Seed manifest plus compiler/test oracle. |
| `cross-file` | `cross_file_paths` >=30, each depth >=3 and >=3 distinct product files; positives are these paths, negatives disconnected controls. go/types, independent SCIP or native compiler. |
| `cross-repository` | `repository_links` >=30; union of their verified endpoints gives `linked_repositories` >=3 and `linked_services` >=3. Matched non-links. Lockfiles/protocol contracts plus independent link key. |
| `fan-in-dispatch` | `fanin_symbols` >=3, each >=5 unique product call sites in >=3 outside units under D24/D26; `dispatch_sites` >=30, dispatch OR re-export sites with native resolution. Common 30/30 still applies. D26 recount plus compiler receipts. |
| `hazards` | `hazardous_changes` >=30 and `safe_changes` >=30 for EACH declared (tool, kind). Base-state-to-head plan/native diff, not a scanner's own hazard label. |
| `modules-unknowns-upgrades` | `module_expressions`, `missing_context`, `provider_transitions` each >=30. Native evaluator/provider contract diffs. |
| `stale-rationale` | `stale_instructions`, `live_instructions`, `governing_links` each >=30 with explicit anchored governing records. Seed key or confirmed public-record anchors. |
| `fixes-mutants` | `public_fixes` >=30, each subject test fails at parent and passes at fix; `collateral_mutants` >=30 each independently killed/survived labelled. Common matched safe controls remain mandatory. Native subject tests and public history. |
| `localization` | `nonlexical_issues` and `lexical_issues` each >=30, each with base file present, D-a token rule and independent fix key. D-b is diagnostic only and cannot relabel D-a. |
| `landing` | `conflicting_pairs` and `clean_pairs` each >=30 with actual merge/gate logs and overlap evidence, including checked absence for clean pairs. |
| `history` | `history_commits` >=200 reachable unique commits from parents, plus `relevant_changes` >=30 independently labelled. Merge messages/shallow checkout length do not count. |
| `polyglot-generated` | `boundary_sites` >=30, generated/handwritten boundary verified from generator inputs; `language_sites` >=30 for each declared language, >=3 languages. Native compiler/format keys. |
| `data-subprocess` | `data_reads`, `subprocess_reads`, `uncovered_changes` each >=30. Attested file-read logs and deliberate test failures. |
| `lifetime` | `acquire_release_pairs`, `missing_release_fixes`, `escapes` each >=30 with cross-function identities. SSA, public parent/fixed tests and incumbent linters. Escape is an observed category, not proof of safe release. |
| `growth-shutdown` | `bounded_sites`, `unbounded_sites`, `shutdown_cases` each >=30; reachable bounds plus spawn/join OR timer-loop cases. Independent audit and runtime leak profiles. |
| `shared-map` | `multisession_tasks` >=30, each repeated across sessions, with independent held-out gold/T2 anchors and transcript reads. Repeated attempts do not multiply tasks. |
| `guardrails` | `declared_deletions` and `undeclared_deletions` each >=30; `rule_labels` >=30 for each declared rule kind. Native pipeline execution, incident fix and derived mutants with independent labels. |
| `pins-contracts` | `pinned_inputs`, `floating_inputs`, `interface_changes` each >=30, checked through lock parsers, native API diff and catalog versions. |
| `foreign-inputs` | `valid_envelopes` and `invalid_envelopes` each >=30 per declared format. Independently verified producer/commit/age checks distinguish valid from stale/mismatched examples. |
| `runtime-identity` | `resolvable_bindings` and `ambiguous_bindings` each >=30; every observation preserves declared versus observed values and export provenance. Operator-placed runtime export. No runtime/account join is built. |
| `output-judgment` | `output_cases` >=30 total with `long_outputs` >=1 and `short_outputs` >=1; `question_cases` >=30 total covering `route_questions`, `rank_questions`, `table_questions` each >=1. Lossless bytes, native evaluation or independent confirmed/frozen gold key. T3-3 scopes subtype floors as specified below. |

```go
type Metric = env.Metric
const (
    MetricNone = env.MetricNone
    MetricPositive = env.MetricPositive
    MetricNegative = env.MetricNegative
    MetricAbsentSymbols = env.MetricAbsentSymbols
    MetricUnwiredEntries = env.MetricUnwiredEntries
    MetricCrossFilePaths = env.MetricCrossFilePaths
    MetricRepositoryLinks = env.MetricRepositoryLinks
    MetricLinkedRepositories = env.MetricLinkedRepositories
    MetricLinkedServices = env.MetricLinkedServices
    MetricFaninSymbols = env.MetricFaninSymbols
    MetricDispatchSites = env.MetricDispatchSites
    MetricHazardousChanges = env.MetricHazardousChanges
    MetricSafeChanges = env.MetricSafeChanges
    MetricModuleExpressions = env.MetricModuleExpressions
    MetricMissingContext = env.MetricMissingContext
    MetricProviderTransitions = env.MetricProviderTransitions
    MetricStaleInstructions = env.MetricStaleInstructions
    MetricLiveInstructions = env.MetricLiveInstructions
    MetricGoverningLinks = env.MetricGoverningLinks
    MetricPublicFixes = env.MetricPublicFixes
    MetricCollateralMutants = env.MetricCollateralMutants
    MetricNonlexicalIssues = env.MetricNonlexicalIssues
    MetricLexicalIssues = env.MetricLexicalIssues
    MetricConflictingPairs = env.MetricConflictingPairs
    MetricCleanPairs = env.MetricCleanPairs
    MetricHistoryCommits = env.MetricHistoryCommits
    MetricRelevantChanges = env.MetricRelevantChanges
    MetricBoundarySites = env.MetricBoundarySites
    MetricLanguageSites = env.MetricLanguageSites
    MetricDataReads = env.MetricDataReads
    MetricSubprocessReads = env.MetricSubprocessReads
    MetricUncoveredChanges = env.MetricUncoveredChanges
    MetricAcquireReleasePairs = env.MetricAcquireReleasePairs
    MetricMissingReleaseFixes = env.MetricMissingReleaseFixes
    MetricEscapes = env.MetricEscapes
    MetricBoundedSites = env.MetricBoundedSites
    MetricUnboundedSites = env.MetricUnboundedSites
    MetricShutdownCases = env.MetricShutdownCases
    MetricMultisessionTasks = env.MetricMultisessionTasks
    MetricDeclaredDeletions = env.MetricDeclaredDeletions
    MetricUndeclaredDeletions = env.MetricUndeclaredDeletions
    MetricRuleLabels = env.MetricRuleLabels
    MetricPinnedInputs = env.MetricPinnedInputs
    MetricFloatingInputs = env.MetricFloatingInputs
    MetricInterfaceChanges = env.MetricInterfaceChanges
    MetricValidEnvelopes = env.MetricValidEnvelopes
    MetricInvalidEnvelopes = env.MetricInvalidEnvelopes
    MetricResolvableBindings = env.MetricResolvableBindings
    MetricAmbiguousBindings = env.MetricAmbiguousBindings
    MetricOutputCases = env.MetricOutputCases
    MetricLongOutputs = env.MetricLongOutputs
    MetricShortOutputs = env.MetricShortOutputs
    MetricQuestionCases = env.MetricQuestionCases
    MetricRouteQuestions = env.MetricRouteQuestions
    MetricRankQuestions = env.MetricRankQuestions
    MetricTableQuestions = env.MetricTableQuestions
)
```

MetricNone/MetricPositive/MetricNegative are output-only and cannot occur in
ProjectedMeasure. All other metrics may occur only for the independent typed expectation
bound to that inventory key and its required native-declared group;
unrelated known metrics are invalid_projection. Named cases below are obligation
bindings, never a switch on permitted concept key strings. The exact native
binding and projection bytes remain the owner wiring prerequisite above.

The adapter normalizes **identity and native facts**; ENV owns only joins,
set cardinalities and the stated floors. Common sample projection never infers
metrics from a witness's claimed concept. A witness belonging to another
requirement, language, vector or predicted effect/null cannot fill this cell.

### D24/D26 normalization obligations

Reuse `calibration/bench/tools/recount.py:8-61` as the landed D26 policy, including
its support-path distinctions, and independently bind the compiler/SCIP receipts.
`benchprep.C6Row` (`internal/benchprep/vet.go:63-70`) loses site provenance and
cannot by itself prove product filtering, unit uniqueness or native identity.
`vetC6` counts supplied Calls/Units (`internal/benchprep/vet.go:392-426`);
its success is not a second independent witness. Do not feed it invented rows.

For Go, units are import packages, with external `<pkg>_test` merged into the
corresponding defining directory/package before exclusion. For Python and
TypeScript, units are directories. Defining units never count. Calls are unique
(path,line,column) product sites, not repeated receipt rows or function counts.
Three distinct qualifying symbols each need five calls in three distinct outside
units. Five calls in two units, or three repeated strings pretending to be three
units, fail. Definition-side product eligibility must also be independently
checked; a support-only symbol is not a product fan-in subject.

Exclude test, example, benchmark, sample, documentation and tool directories,
test files and defining-package tests in every language (D26 at
`docs/design/bench-prep.md:173`). Preserve landed product-path exceptions such as
`assert/assertions.go`, `mock/mock.go`, `src/_pytest/fixtures.py` and
`packages/toolkit/src/query/core/apiState.ts`; lexical similarity to an excluded
category is insufficient (`calibration/bench/tools/test_recount.py:12-40`).
Other languages require their native import-unit/test-file policy frozen by the
oracle owner; until supported they give unknown_evidence, never a guessed D26
pass. This is a normalizer wiring requirement, not another Go compiler in ENV.

## Counting, scale, ordering and results

1. Validate the already schema-validated typed request defensively for empty or
   duplicate IDs, unknown enums, invalid paths/refs/pins, malformed strata,
   minima below 30, empty scale vectors, mixed claimed tiers and zero At.
   Unclaimed inventory heads may belong to other tiers; they contribute nothing. No exported function is allowed to
   bypass safety by constructing Go values directly. Do not rerun CUE or record
   ownership discovery here. Schema parsing stays core-owned; concept path grants
   use pathcheck, and other authored ENV records retain their REC boundary.
2. PrepareCoverageSnapshot resolves the exact pinned catalog, environments and
   concepts through the unchanged trusted port, once per binding on private deep
   copies. The pure evaluator reads those immutable values only. Build expected
   requirement/group sets from the catalog, and metric/minimum/unit sets solely
   from the independently supplied inventory, not claim refs or observed measures.
3. Deduplicate each requirement's samples using equivalence components: same
   verified OriginKey OR same physical anchor at its bound repository/commit
   denotes one site. A chain of overlapping identities remains one component.
   Conflicting polarity/match/binding for the same identity is invalid_projection.
   Count one positive per component. Count one negative per distinct verified
   Match that has a qualified positive and a distinct qualified negative;
   enforce one-to-one pairing, so one positive cannot sponsor thirty negatives.
   Unmatched positives still count positive; unmatched negatives count zero.
   Match IDs from authored witnesses never establish this relation on their own.
4. Deduplicate metric Members by the same identity law within the metric/group;
   linked_repositories and linked_services instead count distinct canonical
   repository/service OriginKeys, not their potentially shared evidence anchors;
   they can have their own native event population. They need not be present in
   Samples but must be native-supported by the bound receipts. One metric may
   intentionally share a member with another; no cross-metric sum is made.
   Common positives cannot substitute for a missing supplementary population.
5. Preserve every failure. Each cell's PositiveCount/NegativeCount is the minimum
   qualifying count across its authoritative required concepts, not their sum.
   RequirementResult exposes the separate counts and every metric/group count.
   RequirementResults include missing authoritative requirements, with null counts,
   so an omitted claim ref never hides its missing-concept issue.
   A missing projection has unknown counts, not measured zero. Cell minimum is
   nil if any required concept is unknown; a known empty set yields pointer zero.
6. Classify scale from independently measured values, with unknown pointers
   staying unknown. Require all four excluded categories exactly once on typed heads (otherwise
   invalid_input), including
   known-zero generated counts, and the bound source-policy receipt. Compare
   the measured Scale to the head; changed values/digests are stale_pin.
   Lines/files exclude vendored, generated, tests and build output; history is
   reachable unique commits, not shallow length. No scalar max or summing axes.
7. For each capability/full-stratum/axis/class, collect locally qualifying
   environments of the requested tier and count independent ancestry components.
   An environment in a larger class does not fill an exact smaller-class cell.
   Two are required per claimed axis/class. A claim with no scale axes is
   invalid_input, matching ENV-1's nonempty scale vector. For public_outcome the tier
   must be public and at least two independent canonical public repository
   origins must occur in qualifying environments. Repeated pins, mixed tiers,
   smoke and two constructed replicas cannot fill either floor. A multi-repo
   environment alone does not satisfy the two-environment floor.

Exact classes, BE-D4 (`docs/design/bench-environments.md:127-144`):

| Axis | S | M | L | XL |
|---|---|---|---|---|
| lines | 5,000..49,999 | 50,000..249,999 | 250,000..999,999 | >=1,000,000 |
| files | 50..499 | 500..2,499 | 2,500..9,999 | >=10,000 |
| repositories | 1 | 2..4 | 5..12 | >=13 |
| history | 200..999 | 1,000..4,999 | 5,000..19,999 | >=20,000 |

Below each S floor is tiny for that axis and supports no S claim. Null means
unknown; zero means measured below floor. Unsupported classes are untested.
Do not retrospectively admit old subjects or amend C2 in this leaf. BEN-5/ENV-C6
own C1/C3-C7, prior refusals and the C2 resource-policy amendment. Coverage PASS
alone must not be rendered ADMITTED (`docs/design/bench-environments.md:138-144`;
`internal/benchprep/vet.go:150-173`).

Settled T3-4: these nullable coverage results and all port types live ONLY in
`internal/benchenv/coverage.go`. ENV-1 retains authored inputs, with no
schema alias importing the leaf. The following result declarations are identical to
`docs/contracts/env-1.md@44d4f4518:497-572`.

```go
type CoverageVerdict string
const (
    CoveragePass CoverageVerdict = "PASS"
    CoverageUnfit CoverageVerdict = "UNFIT"
    CoverageIncomplete CoverageVerdict = "INCOMPLETE"
)
type CoverageFailure string
const (
    CoverageMissingConcept CoverageFailure = "missing_concept"
    CoverageMissingCount CoverageFailure = "missing_count"
    CoverageStalePin CoverageFailure = "stale_pin"
    CoverageCloneInflation CoverageFailure = "clone_inflation"
    CoverageAbsentCapability CoverageFailure = "absent_capability"
    CoverageUnknownEvidence CoverageFailure = "unknown_evidence"
    CoverageInsufficientEnvironments CoverageFailure = "insufficient_environments"
    CoverageScaleMismatch CoverageFailure = "scale_mismatch"
)
type CoverageErrorCode string
const (
    ErrorInvalidInput CoverageErrorCode = "invalid_input"
    ErrorInvalidProjection CoverageErrorCode = "invalid_projection"
    ErrorEvidenceUnavailable CoverageErrorCode = "evidence_unavailable"
)
type CoverageError struct {
    Code CoverageErrorCode `json:"code"`
    Path string `json:"path"`
}
func (e *CoverageError) Error() string
// The port uses this for an expected evidence refusal. Unknown errors are
// normalized to evidence_unavailable without forwarding native error text.
type EvidenceRefusal struct {
    Reason CoverageFailure `json:"reason"`
    Path string `json:"path"`
}
func (e *EvidenceRefusal) Error() string

type CoverageIssue struct {
    ClaimID string `json:"claim_id"`
    RequirementID string `json:"requirement_id"`
    Catalog *CatalogKey `json:"catalog"`
    Metric Metric `json:"metric"`
    Group MeasureGroup `json:"group"`
    Scale *env.ScaleClaim `json:"scale"`
    Reason CoverageFailure `json:"reason"`
    Observed *int64 `json:"observed"`
    Required int64 `json:"required"`
}
type MeasureResult struct {
    Metric Metric `json:"metric"`
    Group MeasureGroup `json:"group"`
    Observed *int64 `json:"observed"`
    Required int64 `json:"required"`
}
type RequirementResult struct {
    RequirementID string `json:"requirement_id"`
    PositiveCount *int64 `json:"positive_count"`
    NegativeCount *int64 `json:"negative_count"`
    Measures []MeasureResult `json:"measures"`
    Issues []CoverageIssue `json:"issues"`
}
type CoverageCell struct {
    ClaimID string `json:"claim_id"`
    PositiveCount *int64 `json:"positive_count"`
    NegativeCount *int64 `json:"negative_count"`
    Requirements []RequirementResult `json:"requirements"`
    Issues []CoverageIssue `json:"issues"`
}
type CoverageResult struct {
    Verdict CoverageVerdict `json:"verdict"`
    Cells []CoverageCell `json:"cells"`
    Issues []CoverageIssue `json:"issues"`
}
```

Issues use MetricPositive/MetricNegative for common counts, a table metric for
supplementary counts and MetricNone for non-count failures. All issues have
explicit keys, including null Catalog/Scale/Observed when inapplicable or unknown.
No omitted keys, arbitrary reason strings or embedded native output are allowed.
A known zero is `0`, never null. Class shortages attach Scale. Catalog omissions
attach Catalog, with empty ClaimID/RequirementID for a global omission.

A successful computation returns nil error and a derived verdict. PASS requires
all requested cells and portfolio/closure floors with no issues. Empty claims or
zero locally qualifying cells returns UNFIT. Any locally qualifying cell plus
remaining cell/portfolio/closure failures returns INCOMPLETE. A locally qualifying
cell has verified binding, all required concepts/groups/counts and matching scale.
Known catalog set omissions after successful Catalog do not erase independently
measured local counts; they prevent overall PASS. Catalog refusal occurs earlier
and leaves all cell counts null, as frozen below. Neither is an effect
estimate. A class shortage is insufficient_environments with observed independent
component count and required 2; public repository shortage uses the same reason,
MetricNone and null Scale. Scale mismatch attaches the failing Scale. Missing
concept means a required ref/concept is absent; present but short counts mean
missing_count. An unknown receipt means unknown_evidence, not a known zero.
Clone deduplication is always performed; emit clone_inflation alongside the
corresponding shortage only when nominal multiplicity would have met a floor
that the unique count misses. Harmless duplicates with enough unique evidence
do not themselves turn PASS into failure.

Port EvidenceRefusal accepts only stale_pin, unknown_evidence or
absent_capability, scoped to the requested binding. Other reasons are
invalid_projection. Structural invalidity returns *CoverageError and a normalized
UNFIT result with empty non-nil Cells/Issues, never partial counts or PASS. Choose
errors by the exact staged pointer law and diagnostic tables below; prepare the
snapshot through the port in the specified order; pure evaluation calls no port. No arbitrary resource limit
is introduced by this contract. If implementation needs one, return for a typed
limit contract instead of silently truncating evidence.

All returned slices, nested slices included, are allocated and marshal as `[]`,
never null. No returned slice/pointer aliases caller or adapter memory. Sort cells
by ClaimID, requirements by RequirementID, measures by metric then group fields
(tool, kind, language, format), and issues lexicographically by claim, requirement,
catalog domain/id (nil first), metric, group tuple, scale axis/class (nil first),
reason, observed (nil first), required. Deduplicate identical issues. Global
Issues is the sorted union of cell issues and portfolio/catalog issues. Cell
Issues is the union of requirement issues plus claim issues. Sort sets on private
copies. Semantic ordered inputs are not rewritten. Given identical verified
snapshot and At, shuffled set inputs give byte-identical JSON. No ambient time,
map order, pointer identity or error text affects the output.

### W16 inventory propagation assertions (W3-2)

Core owns the exact `TestValidateConceptInventory_OpenKeys`,
`TestValidateEnvironmentConceptJoin`, `_Requirements` and `_GroupClosure`
assertions in the pinned core; do not substitute a fixed-key or fixed-count test.
The native/catalog owner must add `TestEnvironmentCatalog_InventoryJoin` in its
separately granted test file. Start with supplied K and supplied L, distinct
opaque nonempty evaluated CUE field names, not production allocations. Verify actual pinned Card and
CheckerContract bytes, successful unchanged CheckCardClosure, then capture the
complete Card.ID set passed to ValidateEnvironmentConceptJoin and prove the call
precedes projection. Add L with BE-D9 citation and extend its mapped group; add
the matching requirement in each claim's exact stratum. After separate data
critic review, closure/join/projection pass without contract/schema/enum edits.
Renaming K to another evaluated name with exact matching reference values also
passes; source-label normalization never rewrites references or fixes cardinality.
Remove that new requirement while retaining another: the pure join refuses
ValidationReference at /records/coverage/0/requirements, before projection.
Duplicate a group/concept requirement under a distinct ID: ValidationIdentity at
/records/coverage/0/requirements/1/id. Change mapped Card/CheckerContract path or
sha256: exact ValidationReference at that /records/requirements/0/card or
/checker_contract member. Undefined or wrong-group concept refuses at
/records/requirements/0/concept. These are the core's exact join assertions,
not new leaf port error constants. Assert no projection call after any failure.

Separately remove actual Card or CheckerContract bytes: global Catalog
unknown_evidence with zero Environment/Concept calls; never fabricate closure
from IDs or shape. A readable verified missing checker declaration gives global
Catalog absent_capability. Change only the mapping or a referenced pin without
repinning CatalogProjection.Source: global stale_pin; a fully repinned valid
mapping uses its new source binding. Native payload verification and the
Card/checker byte fixtures remain the native owner's acceptance, not a fake's
proof. Conflicting concept declarations, ungranted/second inventory paths and
path grants are the core KeyConflict/CompatibleDeclarations/PathGrant assertions,
with MapKeys proving escaped pointers and deterministic map order. The path grant
is the future data lane's existing pathcheck proof, not a schema filesystem test.
RecordOwnership remains solely for other authored ENV records, not concept keys.

## Every implementing test and exact assertions

Later ENV-2 writes these tests in `internal/benchenv/coverage_test.go`, observes
the relevant real failure before implementation, then runs
`go test -race ./internal/benchenv -run TestEnvironmentCoverage` under the lane's
package-test lease. No implementation tests are claimed by this contract lane.
All are new tests, not assertions that a named test already exists.

The trusted fake port is explicit in leaf unit tests. It returns pinned fixture
projections or typed refusals, never silently treats Witness JSON as truth.
Native adapter conformance is a separate wiring acceptance, described below.
The independent floor mutations below are FAKE port aggregation cases. A raw
native prerequisite deletion may also invalidate a common positive and its
matched negative; it must not preserve them artificially. The requested native
owner must freeze complete normalized projection bytes, including those joined
common populations and their exact additional shortages, with each raw-fixture
manifest before implementation. Until that exact manifest grant is accepted,
raw inventory-wide concept conformance is explicitly unproven, not implied by the floor twins.
A fake passing projection proves the leaf's aggregation only, not real receipt
verification. Every negative twin below starts from two independent exact-floor
healthy environments and changes one condition, keeping unrelated counts high.
Common positive/negative counts are coupled by the leaf's one-to-one matching law
even for a fake port: negative_count <= positive_count. The complete common-count
twins below apply to every positive-short/clone case, including subtype and D26.

| Exact top-level test name | Exact positive and negative assertions |
|---|---|
| `TestEnvironmentCoverage` | Two independent eligible environments, all authoritative requirements, 30 positive/30 matched negative each and exact supplementary/scale floors: PASS, nil error, each cell's counts 30, never 60. Reduce to zero claims: UNFIT with non-nil empty cells, never PASS. |
| `TestEnvironmentCoverage_InventoryKeys` | Inject supplied K and supplied L with exact existing metric bindings and complete Card/group mapping. Extending the reviewed inventory, requirements and claim refs consistently leaves the pure leaf law unchanged; no contract roster/cardinality check. Removing each authoritative ref still gives M at its sorted expected slot, with the exact unchanged MissingConcept unions. Named BE-D2 floor cases all remain mandatory. The fake proves aggregation only; TestEnvironmentCatalog_InventoryJoin separately proves real preparation order/byte binding under its owner grant. |
| `TestEnvironmentCoverage_MissingConcept` | Each authoritative inventory concept ref can be removed independently from the claim while all others exceed floor: missing_concept naming the claim/ref, no PASS. All present passes. All local cells unfit gives UNFIT; one healthy cell plus a failing cell gives INCOMPLETE. |
| `TestEnvironmentCoverage_OneShort` | Remove one positive from30 pairs ->29/29 and BOTH shortages. Remove only one negative ->30/29 and negative shortage only. Assert the complete ordered common-count twins below, including side-specific and joint minima31. Thirty passes; authored29 returns invalid_input. Extra other-language/vector/prediction witnesses cannot fill a gap. |
| `TestEnvironmentCoverage_Matching` | Thirty distinct verified 1:1 pairs pass. Thirty unpaired negatives, one positive reused by 30 negatives, and forged authored MatchID each cannot give 30 negative count. Distinct verified matched origins count. Conflicting bindings on one origin produce invalid_projection. |
| `TestEnvironmentCoverage_StalePin` | Independently mutate environment ref/digest, repository commit, source policy/licence/toolchain/image binding, witness/requirement ref, native receipt digest, checker version and checker digest: typed port refusal yields stale_pin on exact cell/ref, no count from it. Exact pinned twins pass. Absent/unreadable evidence yields unknown_evidence with null counts, not zero. |
| `TestEnvironmentCoverage_TrustedBoundary` | Nil port is invalid_input. Complete self-labelled authored witnesses without independent projection cannot pass; unknown source refuses. A source with a claimed producer only cannot qualify through the conforming adapter fixture. Correct independently bound fixture can qualify. Crossed binding IDs, extra witness, bad member identity, scalar on non-history metric, unknown metric, duplicate/conflicting measure group: invalid_projection at the fixed diagnostic slot; same-ID changed digests use stale_pin. Unexpected port error becomes evidence_unavailable without raw text. |
| `TestEnvironmentCoverage_CloneInflation` | New witness IDs at identical anchors, same-origin moved clones, descendant fixes and padding leave unique count unchanged. Nominal positive30/unique29 gives29/29, BOTH missing_count issues and positive clone_inflation; a negative-only clone gives30/29 with negative clone_inflation and shortage. Assert the complete ordered twins below. Enough distinct origins passes even with harmless duplicates. Transitive clone components count once. Alternate pins and seed-only variants count one environment, while independently verified families count two. |
| `TestEnvironmentCoverage_CapabilityClosure` | Authoritative complete pinned projection passes. Independently delete each domain Entry while retaining Inventory; separately empty a requirement mapping, remove a checker declaration from readable verified contract bytes, make checker-contract bytes unavailable, and omit each authoritative inventory concept. Known declaration absence is global Catalog R(absent_capability); unavailable bytes is global Catalog R(unknown_evidence), not K. Known entry/mapping omissions use K and concept omissions use M, with exact results below. A malformed/empty typed checker pin is invalid_input. Include all seven families, all card rows/additions, primitives1..8, RL-1..12, and all V-N/A-N targets. A reduced source snapshot is refused by the adapter: changed bytes give stale_pin, a genuine source closure omission gives absent_capability. Neither can masquerade as the pinned full catalog. Deferred but declared targets need no coverage claims or runtime implementation. |
| `TestEnvironmentCoverage_GroupClosure` | Each catalog-declared tool/kind, language, format and rule-kind group appears with every required metric: exact floors pass. Zero members gives missing_count observed0. Missing entire group/metric is invalid_projection. An observed extra group, wildcard or unknown metric cannot backfill or silently widen catalog obligations. |
| `TestEnvironmentCoverage_ConceptSpecificFloors` | Run every exact subtest below. Each exact-floor healthy twin passes; each named mutation fails the exact requirement/metric/group. Common 30/30 remains in force even when a supplementary minimum is 1, 3 or 200. |
| `TestEnvironmentCoverage_D26` | Run every exact D26 subtest below against frozen normalizer fixtures and their projections. Healthy 3 symbols x5 calls x3 outside units plus dispatch/common floors passes; excluded/duplicate sites never repair a shortage. Leaf fake fixtures assert resulting counts/refusals; requested adapter conformance separately proves the raw native projection. |
| `TestEnvironmentCoverage_Scale` | Run all 16 (axis,class) subtests named below with floor-1, floor and upper-bound values; floor and upper classify exactly, floor-1 cannot fill that class. Null/zero below-floor never becomes S. Each axis independent; large lines cannot hide short files/history. A valid head with unavailable native exclusions/receipt gives unknown_evidence; changed bound receipt gives stale_pin. Missing/duplicate exclusion categories on typed heads are invalid_input. |
| `TestEnvironmentCoverage_IndependentEnvironments` | For each claimed axis/class, two independent locally eligible environments pass, one gives insufficient_environments observed1/required2 and class untested. Two pins, overlapping-repository components and seed-only replicas still count1. Two independent constructed families pass constructed coverage, fail public_outcome. Two public repo origins are additionally required for public_outcome. Smoke never contributes; mixed-tier request is invalid_input. |
| `TestEnvironmentCoverage_NoMutation` | Deep-snapshot req and fake source; call twice, deep-equal originals including nested pointers/slices. Mutating returned result cannot change request, source or earlier result. A hostile fake modifying passed argument copies cannot mutate caller data. |
| `TestEnvironmentCoverage_Determinism` | Four fixed shuffled orders for environments, requirements, claims, witnesses, catalog inventory/entries/groups, samples, measure members and adapter-return slices: same sorted byte-identical JSON at fixed At. Multiple simultaneous calls with read-only snapshot match under -race. Nil/empty equivalent input sets normalize identically where valid. |
| `TestEnvironmentCoverage_JSONVocabulary` | Golden full result and normalized error contain exactly declared lower snake case keys and named enum strings. All result collections including nested empty issues/measures/requirements are [], never null. Unknown count is null, known zero0. No sentinel identity strings, raw native output, secrets or native paths. Unknown purpose/tier/reason/metric enum refuses. |
| `TestEnvironmentCoverage_PhaseSeparation` | Same evidence and assigned cells, different external W4/W5/use/action outcomes: byte-identical result because those fields are not inputs. No WPF or model interface is called. No native execution, filesystem, git, network or ambient-clock dependency enters the leaf. Fixed At controls foreign-input age fixture; an unrelated WPF/freshness seam cannot block leaf compilation/proof. |

### Exact concept subtests

Names below are appended to `TestEnvironmentCoverage_ConceptSpecificFloors/`.
The concept table defines the independently verified source and membership; these
assertions test each floor, not an authored success boolean.

| Subtest | Exact healthy boundary and independent negative mutations |
|---|---|
| `false-plan` | 30 false/30 true, >=1 absent symbol and >=1 unwired entry. Remove either subtype: observed0/required1 failure. |
| `cross-file` | 30 depth>=3 paths across >=3 product files/30 disconnected controls. Replace any required path with depth2 or two-file path: qualifying paths29, fails. |
| `cross-repository` | 30 links/30 non-links, >=3 repositories and >=3 services. Separately reduce links29, repos2, services2: exact metric fails. |
| `fan-in-dispatch` | >=3 symbols each5 calls/3 outside units and dispatch sites30 plus common30/30. Symbols2, one symbol4 calls or2 units, or dispatch29 each fails independently. |
| `hazards` | Each declared tool/kind30 hazardous/30 safe. Either side29 in any group fails despite another group's surplus. |
| `modules-unknowns-upgrades` | Expressions30, missing-context30, transitions30. Independently29 each fails. |
| `stale-rationale` | Stale30, live30, explicit governing links30. Independently29 or a missing required anchor fails. |
| `fixes-mutants` | Native fail-before/pass-after public fixes30 and independently labelled mutants30. Either29, missing parent test, index-derived label or absent killed/survived label cannot count and fails. |
| `localization` | D-a nonlexical30/lexical30 with base files and fix keys. Either29 or absent base file fails; changing D-b alone cannot change D-a counts. |
| `landing` | Conflicting30/clean30 with real merge/gate/overlap-or-absence evidence. Either29 or missing evidence fails. |
| `history` | Reachable unique200 and labelled changes30. Commits199 or changes29 fails. Duplicate parent paths or merge-message proxies cannot inflate count. |
| `polyglot-generated` | Boundary30 plus >=3 declared languages each30. Boundary29, languages2, one language29 or missing generator/native binding fails. |
| `data-subprocess` | Data reads30, subprocess reads30, uncovered changes30. Independently29 or missing read/failure receipt fails. |
| `lifetime` | Cross-function pairs30, missing-release fixes30, escapes30. Independently29 or missing SSA/test/incumbent support fails. Escapes never become safe-release positives by inference. |
| `growth-shutdown` | Reachable bounded30/unbounded30 and shutdown cases30. Independently29 or missing reachability/leak evidence fails. |
| `shared-map` | Repeated multi-session tasks30 with independent gold/T2 anchors. Tasks29, absent gold, or repeating one task30 times fails. |
| `guardrails` | Declared30/undeclared30 and independent labels30 for each rule kind. Either deletion29, one kind29, absent pipeline/fix/mutant support fails. |
| `pins-contracts` | Pinned30/floating30/interface changes30 with native lock/API/catalog receipts. Independently29 or missing binding fails. |
| `foreign-inputs` | Each declared format valid30/invalid30, stale or mismatch negatives independently verified. Either29 or a missing group fails; genuinely stale backing evidence cannot count even for a deliberately stale negative. |
| `runtime-identity` | Resolvable30/ambiguous30 with declared/observed separation and operator export. Either29, conflated values or absent export fails. |
| `output-judgment` | Outputs30 including long>=1/short>=1, questions30 including route/rank/table>=1 each. Outputs29, questions29, any subtype0, changed lossless bytes or absent confirmed/native/frozen key fails. |

### Exact D26 subtests and adapter fixture obligations

Names appended to `TestEnvironmentCoverage_D26/`:

| Subtest | Exact assertion |
|---|---|
| `GoUnits` | Three distinct outside import packages count; multiple files in one package count one unit. Defining package and its `<pkg>_test` package contribute zero units/sites. One fewer qualifying outside unit fails. |
| `PythonUnits` | Three outside directories count; several modules in one directory count one. Defining-directory product and test calls contribute zero. Healthy five calls in three outside directories passes; two directories fails. |
| `TypeScriptUnits` | Three outside directories count, not three exports/files from one directory. Defining-directory calls and its tests contribute zero. Five calls/three directories passes; two directories fails. |
| `ProductOnly` | For each Go/Python/TypeScript fixture independently, exclude test/example/benchmark/sample/docs/tool directories and test files, including nested support paths and defining-unit tests. Each fixture has 4 product calls plus support calls: cannot reach5. Healthy5 true product calls qualifies. |
| `LandedPathPolicy` | Every excluded path in ProductCallers.test_excludes_support_in_every_language remains excluded. Every product path in test_keeps_product remains eligible subject to native membership. Broad substring filters that exclude those product paths fail conformance. |
| `UniqueSitesAndUnits` | Repeated identical native site rows count once, repeated units count once, five distinct sites across three units count5/3. Repetition cannot repair 4/2. |
| `DefiningProductSymbol` | A support-only target does not qualify even with outside product callers. Identical product target with native binding qualifies. Unknown definition provenance refuses. |
| `DispatchAndCommonFloor` | Three qualifying fan-in symbols alone cannot replace dispatch30 or common30/30. Each missing component fails its own metric. All three conditions pass together. |
| `UnknownNativeTruth` | Named producer, C6Row.Qualifies=true, or VetReceipt kept alone cannot count. Missing or fathom-derived compiler/SCIP input yields unknown_evidence. Bound independent native truth passes. |

The adapter-owning integration lane MUST replay these raw D26 fixtures against
its actual normalizer and demonstrate equal projections. That is a wiring proof,
not supplied by a fake leaf port or a second ENV recount implementation. Until
that owner/file grant exists, native integration remains unproven. The contract
gives the exact T3-selected interface, normalization and requested acceptance.

### Exact scale subtests

The exact subtest paths under `TestEnvironmentCoverage_Scale/` are:

| Axis | Exact subtest paths |
|---|---|
| lines | `lines/S`, `lines/M`, `lines/L`, `lines/XL` |
| files | `files/S`, `files/M`, `files/L`, `files/XL` |
| repositories | `repositories/S`, `repositories/M`, `repositories/L`, `repositories/XL` |
| history | `history/S`, `history/M`, `history/L`, `history/XL` |

For each finite class test lower-1, lower, upper, upper+1. XL tests lower-1,
lower and lower+1. Also test per-axis nil, known0, missing/duplicated excluded
category, missing receipt, and independent weak-axis failure as top-level
assertions with the exact failure categories specified above. Run each class with two distinct
qualifying environment families; absence of the second cannot be hidden by
another axis or an above-class value. No arbitrary upper XL cutoff is added.

## T3-3 exact subtype metadata flow and assertions

Use only ENV-1's existing Card/CheckerContract refs and Stratum.Subtype. Native
preparation resolves Requirement.Card to CTR-13 Card, checks Card.ID equals both
Capability fields, then resolves CheckerContract to the pinned obligation binding
for that exact Card. It verifies the exact Card.Claim and EffectStratum (effect)
or NullStratum (null) bytes against that binding and its exact env.Stratum.
The binding identifies the allowed subtype selector explicitly, never by parsing
Claim prose, title, surface, filename, observed sample or a substring. The schema
validator checks shape/reference consistency only. No new field/type is needed.
A checker-contract binding that cannot establish the exact selector refuses Catalog
with unknown_evidence before cell evaluation; never infer an aggregate/subtype claim.

Flow is Card.ID -> Requirement.Card + CheckerContract -> exact Requirement.Stratum
-> CatalogEntry.Requirements/ConceptObligation.Requirement -> equal claim Stratum
-> Concept's selected witnesses. Subtype is part of that full equality before
counting. The port returns native-supported membership for that requirement, so
it needs no parallel subtype/group field. Aggregate selector is exactly "";
nonempty selectors are opaque card-declared strings, not a new schema enum.

For the synthetic output fixture only, explicitly bind selectors long, short,
route, rank and table through pinned checker contracts. Use Card.ID
opaque-fixture-card, Surfaces [verb:flow], Claim fixture-claim, EffectStratum
fixture-effect and NullStratum fixture-null. The native fixture contract binds
those exact bytes to the selected env.Stratum, including its Subtype. Changing
only a title or surface cannot change the selector. Changing bound claim/stratum
bytes without repinning refuses Catalog with stale_pin; independently pinned but
unsupported selector evidence refuses Catalog with unknown_evidence. A returned catalog requirement that
does not authorize a claim's nonempty subtype gives invalid_input at
/records/coverage/0/stratum/subtype. Repinning all authored bytes cannot itself
establish native authorization.

Aggregate output-judgment requires output_cases30, question_cases30,
long_outputs1, short_outputs1, route_questions1, rank_questions1, table_questions1,
and common positive30/negative30. An explicitly claimed long subtype requires
output_cases30 and long_outputs30 in that subtype; short uses output_cases and
short_outputs. Route/rank/table each require question_cases and the corresponding
question metric at30. Other aggregate presence metrics are not required in a
single subtype stratum. The independent common30/30 is required within EACH
selected subtype. An opaque selector not supported by native metadata remains
unknown_evidence. No borrowing from an aggregate, another subtype or another
language/vector/prediction is permitted.

MeasureResult has no verdict. Aggregate PASS creates only the requested aggregate
cells and never creates subtype claims, cells or a subtype PASS. A subtype count
below30 is labelled exactly `descriptive only` by consumers, even if another
scope's verdict is PASS; the leaf's result conveys count plus requested scope,
not a fabricated status field. Even a subtype count>=30 does not create a claim.
An explicitly claimed subtype with29 fails its own floor; the failed claim's
UNFIT/INCOMPLETE is an adequacy refusal, never a subtype effect verdict.

| Exact additional test | Mutations and exact assertions |
|---|---|
| `TestEnvironmentCoverage_CardMapping` | Exact opaque Card.ID (different from verb:flow), refs and complete mapping passes. Capability set to verb:flow -> E(input,/records/requirements/0/card/id). Mutated Card bytes -> global R(stale_pin,/evidence/catalog), unavailable Card bytes -> global R(unknown_evidence,/evidence/catalog); same independently for checker_contract. Card.Claim/selected-stratum changed without repinning -> global R(stale_pin,/evidence/catalog); unavailable selector binding -> global R(unknown_evidence,/evidence/catalog). Each is the one-issue early-refusal result below, with zero Environment/Concept calls. These are fake refusal propagation tests, not native verification. |
| `TestEnvironmentCoverage_SubtypeFloors` | Aggregate long1+short29, route1+rank1+table28, common30/30 -> PASS/nil; output_cases=30, question_cases=30, exactly the requested two aggregate environment cells, no subtype cell/verdict. For EACH of five subtype metrics independently mutate to0 -> C(metric,0,1,G0); mutate output_cases and question_cases independently to29 -> C(metric,29,30,G0); common-positive removal ->29/29 with BOTH shortages; negative-only removal ->30/29 with negative shortage. Apply ALL complete common-count twins below, including positive/negative clones and side-specific/joint minima31, with their ordered unions and exact counts. Every below30 subtype remains descriptive only, including in failure output. |
| `TestEnvironmentCoverage_SubtypeClaims` | For EACH long/short/route/rank/table, exact native-selected requirement+claim Subtype with own common30/30 and selected two metrics30 -> PASS/nil. Each selected metric29 -> C(metric,29,30,G0), common-positive removal ->29/29 with BOTH shortages, negative-only removal ->30/29 with negative shortage. EACH of five selectors runs ALL common-count twins below, including clones and side-specific/joint minima31, retaining its two supplementary metrics30. Long29+short1 and route29+rank1 cannot repair the selected29. Empty aggregate and nonempty subtype claims never merge. Unclaimed nonempty selector -> E(input,/records/coverage/0/stratum/subtype). Returned witnesses from another requirement -> E(projection,P/samples/0/witness_id). No inferred per-subtype PASS or effect verdict at29. |

## Exact diagnostic grammar and mutation tuples (ENV-2/F2)

These are prospective assertions, not executed tests. They supplement every test
above. The notation below is a complete expansion rule, not a choice of outcomes.
A test MUST assert the entire expanded tuple plus exact result/error JSON, not
substring error text. No change to canonical CoverageIssue/CoverageError types is
introduced. Path is carried by CoverageError/EvidenceRefusal only; the tuple's
Path for a computed issue is its assertion location in the diagnostic view,
never an extra CoverageIssue JSON field.

### Fixture and paths

F has two independent eligible public environments e-a/e-b, claims c-a/c-b,
requirement r-x for the subtest's concept (all other requirements have IDs
starting z- so the mutated requirement is canonical index0), exact common30/30, native-backed
supplementary floors and the four exact S claims files/history/lines/repositories.
Each twin has one full claimed stratum, portfolio index0, shared by c-a/c-b.
Other authoritative requirements remain healthy. Counts and diagnostics below mutate only e-a/c-a;
e-b/c-b stays locally eligible. A coverage issue therefore gives INCOMPLETE/nil
error unless the row explicitly removes all eligible claims or mutates global
catalog/requirements shared by both cells. Those exact global cases follow the
closure-status rule below. The common-count table freezes side-specific and joint
minimum31 twins;
all unaffected requirements have common counts at least31 for those twins.
Only e-a is mutated. Every other required concept/metric and e-b remain healthy,
so the table's requirement counts are also the exact c-a cell minima.
Each fixture has unique full pins, anchors, pair origins and disjoint ancestry families. Use
2026-11-15T00:00:00Z for At. G0 is the all-empty MeasureGroup tuple.

JSON pointers have RFC6901 token escaping (~ -> ~0 then / -> ~1), leading slash,
no URI-fragment or percent decoding, and decimal array indices without leading
zeroes. Empty pointer denotes root only. All malformed fixtures place the mutated sample/member at canonical index0;
count tables expand the actual expected slot for every removed concept/group.
Ref.Path is separate: apply the exact
ENV-1 canonical relative slash-path law
(`docs/contracts/env-1.md@44d4f4518:280-285`), never reinterpret it as a pointer.

Pointers address a canonical diagnostic view, explicitly independent of caller
set order. /records has the ENV-1 shape. Sort copies of environments,
requirements, witnesses and coverage by ID; duplicate-ID ties by canonical typed
JSON (fixed struct field order, recursively sorted set children). Refs sort by
ID/path/SHA256, pins by Name/Version/SHA256, repositories by ID/URL/Commit,
scale claims by Axis/Class and exclusions by Kind. Ordered semantic lists stay
ordered. Empty IDs sort before nonempty. Validation never mutates caller input.

/evidence/catalog is CatalogProjection plus diagnostic bindings keyed by exact
Requirement.ID (each the canonical Requirement). Inventory/entries are sorted by
(domain,id), concepts by Requirement ref, groups lexicographically by
(tool,kind,language,format). /evidence/environments/<environment-id> holds the
EnvironmentProjection plus a diagnostic-only bindings child equal to the canonical
ENV-1 EnvironmentHead. This child makes repository/policy/native pin dependency
paths precise without adding a port field. /evidence/concepts/<claim-id>/<requirement-id> holds the
ConceptProjection plus diagnostic-only card and checker_contract children equal
to the selected Requirement refs. Its samples sort by WitnessID then complete
comparable key; measure Members sort by complete comparable SampleKey fields.
For diagnostics only, its measures are keyed by exact Metric then indexed by the
sorted union of declared and returned groups (including missing required slots).
No opaque ID is sentinel encoded. Diagnostic measure paths end
/measures/<metric>/<group-index>/members (or /value for history_commits).
These keyed diagnostic views are not additional exported Go/CUE types.

For any explicit-subtype fixture, its authoritative requirement ref selects only
that subtype; metadata mismatch paths are the same typed request fields.
For F, P = /evidence/concepts/c-a/r-x. Replace r-x with the exact concept key
prefixed r- in concept-table cases (for example r-history). Q(m,g) =
P/measures/<m>/<g>/members; Q(history_commits,0) ends /value instead.
Missing array members use their slot in sorted expected-union-actual keys;
malformed present members use their canonical present slot. This rule fixes paths
after removal, duplicate insertion and set shuffling. Extra IDs with / or ~ are
escaped, never interpreted as path traversal. A port refusal Path MUST fall under
its requested catalog/environment/concept prefix and use this grammar; outside
prefix or malformed escapes -> E(projection,the invoked binding root).

### Complete tuple notation

Tuple order is (Code, Reason, Path, ClaimID, RequirementID, Catalog, Metric, Group,
Scale, Observed, Required). The literal null means inapplicable/unknown, not0.

* E(input,p) = (invalid_input,null,p,"","",null,none,G0,null,null,0).
  E(projection,p) substitutes invalid_projection, E(unavailable,p) substitutes
  evidence_unavailable. These are typed errors with NO CoverageIssue, and return
  exactly {"verdict":"UNFIT","cells":[],"issues":[]}.
* C(m,o,n,g) = (null,missing_count,Q(m,g),c-a,r-x,null,m,the exact group g,null,o,n).
  It is an issue, nil error; the measure and requirement expose the same known
  count. Common sides instead use P/samples, metric positive/negative, G0.
* R(reason,p) = (null,reason,p,c-a,r-x,null,none,G0,null,null,0), nil error.
  Unknown/stale binding counts are null, not0; no projection from that binding
  contributes. For catalog-wide refusal use empty claim/requirement and
  /evidence/catalog; for environment refusal use c-a, empty requirement and
  /evidence/environments/e-a. All affected requirement/cell counts become null for unknown whole environment
  or concept bindings. For scale-only unknown/stale refusal, concept common
  counts independently known at that source remain30; only the affected scale
  measurement is null. Scale failure still prevents local eligibility.
* K(domain,id,p) = (null,absent_capability,p,"","",{domain,id},none,G0,null,null,0).
  M(p) = (null,missing_concept,p,c-a,r-x,null,none,G0,null,null,0).
* S(axis,class,o) = (null,scale_mismatch,/evidence/environments/e-a/scale/<axis>,
  c-a,"",null,none,G0,{axis,class},o,class-lower-bound).
  Unknown measured scale substitutes unknown_evidence, Observed:null, Required:0.
* N(axis,class,o) = (null,insufficient_environments,/portfolio/opaque-fixture-card/
  0/<axis>/<class>,"","",{capability,opaque-fixture-card},none,G0,
  {axis,class},o,2). Public-origin shortage uses /portfolio/opaque-fixture-card/
  0/public_repositories, Scale:null. Portfolio index0 is the first canonical full stratum, ordered by language,
  prediction, subtype and complete axis/class vector. Different subtype scopes
  get distinct indices; this never collapses two full strata into one key. Portfolio tuples have no RequirementID.

The short strings input/projection/unavailable are notation, not new enums.
Computed issue Path is used to verify the bound request/diagnostic slot only.
Repeated known count shortages additionally create clone_inflation with identical
issue keys/counts when and only when nominal multiplicity met that floor.
Structural/operational errors have no Reason/metric/counts in their two-key JSON.
For each tuple template expand only explicitly overridden fixture IDs, fields and
indices; do not accept an alternate Code/Reason or a regex-matched Path.
Every requirement issue is present once in its RequirementResult.Issues, once
in its cell union, and once in the global union; a portfolio/global closure
issue appears only globally. A claim-wide scale/binding issue appears in that
cell and globally, not in an unrelated requirement. All union comparisons use
the complete typed keys and documented sorting, with no duplicate issue.
CoverageError.Error() formats exactly `<code> at <path>`; EvidenceRefusal.Error()
formats `<reason> at <path>`. Native error text never enters either output.

### Complete common-count twins and arithmetic proof

This table is mandatory for OneShort, CloneInflation (EACH listed identity
mutation), SubtypeFloors, SubtypeClaims (EACH of five selectors), and
D26/DispatchAndCommonFloor. It overrides no matching/lineage law. All supplementary
metrics remain healthy. For minima31 twins all other required common populations
are >=31; e-b meets the same authored minima and stays locally eligible.

Construct concrete samples p-01..p-k and n-01..n-k at disjoint anchors/origins,
with p-i/n-i sharing verified pair-i. "drop-p/n" removes only the last sample
of that polarity. "clone-p/n" replaces its last sample by a copy of its first
with the SAME key and Match (a fresh witness ID does not change either).
"append-p/n" adds that harmless duplicate without removing an original. Moved,
descendant/padding and transitive aliases use the same normalized component and
consistent Match. A conflicting Match is invalid_projection, never this twin.

Cp(o,r)/Cn(o,r) expand exactly C(positive/negative,o,r,G0). Dp/Dn expand the
identical complete tuple with Reason clone_inflation. Arrays below are ORDERED
requirement and cell issue unions, not alternative tuples. Every row's known
counts apply to r-x AND c-a; c-b remains known at its healthy k/k. Empty union
means global [], PASS/nil. Otherwise global order is [N(axis,class,1) for EACH
claimed axis/class in canonical order, then the listed local union],
INCOMPLETE/nil. Counts never become null in these known-population twins.
Portfolio issues have empty ClaimID and therefore sort before c-a issues;
negative sorts before positive, clone_inflation before missing_count. Paths are
P/samples for all local tuples. The declared tuple grammar supplies ALL other
fields. Unchanged requirements have Issues:[]; no extra clone issue is inferred
for unmatched negatives after a positive clone.

| Twin | k; mutation; positive/negative minima | Known positive/negative | Ordered requirement/cell union |
|---|---|---|---|
| healthy30 | 30; keep;30/30 | 30/30 | [] |
| positive-short | 30; drop-p;30/30 | 29/29 | [Cn(29,30),Cp(29,30)] |
| negative-short | 30; drop-n;30/30 | 30/29 | [Cn(29,30)] |
| positive-clone | 30; clone-p;30/30 | 29/29 | [Cn(29,30),Dp(29,30),Cp(29,30)] |
| negative-clone | 30; clone-n;30/30 | 30/29 | [Dn(29,30),Cn(29,30)] |
| healthy-positive-clone | 30; append-p;30/30 | 30/30 | [] |
| healthy-negative-clone | 30; append-n;30/30 | 30/30 | [] |
| positive-min31-healthy | 31; keep;31/30 | 31/31 | [] |
| negative-min31-healthy | 31; keep;30/31 | 31/31 | [] |
| positive-min31 | 31; drop-p;31/30 | 30/30 | [Cp(30,31)] |
| negative-min31 | 31; drop-n;30/31 | 31/30 | [Cn(30,31)] |
| both-min31-healthy | 31; keep;31/31 | 31/31 | [] |
| both-min31-positive-short | 31; drop-p;31/31 | 30/30 | [Cn(30,31),Cp(30,31)] |
| both-min31-negative-short | 31; drop-n;31/31 | 31/30 | [Cn(30,31)] |
| both-min31-positive-clone | 31; clone-p;31/31 | 30/30 | [Cn(30,31),Dp(30,31),Cp(30,31)] |
| both-min31-negative-clone | 31; clone-n;31/31 | 31/30 | [Dn(30,31),Cn(30,31)] |

This independent Python set check derives counts and complete ordered union
labels from concrete sample/pair keys and compares EVERY table row, including
unchanged healthy twins. Run from the repository root. It proves documentation
arithmetic only, not Go RED/GREEN, lineage normalization or native conformance.
The normalized-key fixtures have no conflicting identities or many-to-one
positive sponsorship; negative membership is the qualified pair intersection.

```python
from pathlib import Path
import re
text = Path("docs/contracts/env-2.md").read_text()
rows = re.findall(r"^\| ([\w-]+) \| (\d+); ([\w-]+);(\d+)/(\d+) "
                  r"\| (\d+)/(\d+) \| (\[.*?\]) \|$", text, re.M)
assert len(rows) == 16
for name, k, mutation, pf, nf, ep, en, expected in rows:
    k, pf, nf = map(int, (k, pf, nf))
    p = [(f"p-{i:02}", f"pair-{i:02}") for i in range(1, k+1)]
    n = [(f"n-{i:02}", f"pair-{i:02}") for i in range(1, k+1)]
    side = p if mutation.endswith("-p") else n
    if mutation.startswith("drop-"):
        side.pop()
    elif mutation.startswith("clone-"):
        side[-1] = side[0]
    elif mutation.startswith("append-"):
        side.append(side[0])
    pm = {pair for key, pair in p}
    matched = [(key, pair) for key, pair in n if pair in pm]
    pc, nc = len(set(p)), len({pair for key, pair in matched})
    assert nc <= pc and (pc, nc) == (int(ep), int(en)), name
    issues = []
    for suffix, count, floor, nominal in (
            ("n", nc, nf, len(matched)), ("p", pc, pf, len(p))):
        if count < floor:
            if nominal >= floor:
                issues.append(f"D{suffix}({count},{floor})")
            issues.append(f"C{suffix}({count},{floor})")
    local = "[" + ",".join(issues) + "]"
    assert local == expected, (name, local, expected)
    portfolio = [f"N({axis},S,1)" for axis in ("files", "history", "lines", "repositories")]
    global_union = portfolio + issues if issues else []
    print(name, (pc, nc), local, global_union)
print("PASS: 16 concrete common-count twins and ordered unions")
```

### First-error and refusal ordering

Stages are (1) defensive typed request validation, (2) catalog, (3) environment
by canonical ID, (4) concept by claim ID then requirement ID. A structural error
in an earlier stage wins without calling a later stage. Request validation subphases are field shape/path/enum/bounds, identity, then
cross-reference consistency, in that order. This prevents a malformed field from
being hidden by its downstream mismatched ref. Missing expected requirement
content is a computed M issue after catalog resolution, not a reference-syntax
error. Within each subphase (and each later projection-validation stage) gather
all independently checkable structural errors, select lexicographic pointer then
code. If an earlier binding operation returns an unexpected error, stop there;
no partial result escapes. Expected EvidenceRefusal values are issues. A Catalog
refusal stops stage2 with
its one global issue and no Environment/Concept calls or portfolio evaluation.
Only after Catalog succeeds do environment/concept refusals accumulate independent
issues and portfolio shortages. Binding-local structural
violations beat that binding's returned data; simultaneous valid-data shortages
all survive. Port calls use private copies in this exact order, once per identical
complete binding. A reused concept projection may serve multiple equal requests,
but is never reused across distinct stratum/subtype bindings.

TestEnvironmentCoverage_Determinism/CompetingInputErrors starts F with
Purpose="bad" AND requirements[0].PositiveMinimum=29. Both orderings of all set
inputs select E(input,/purpose), exact error
{"code":"invalid_input","path":"/purpose"}; port call count0. Repair Purpose
only: E(input,/records/requirements/0/positive_minimum). Add unknown negative
minimum29 as well: /negative_minimum sorts first. Canonical requirement ID remains
r-x despite set shuffle. Test /CompetingProjectionErrors returns scalar Value=1
on metric cross_file_paths AND a forged sample's witness_id. It selects
E(projection,P/measures/cross_file_paths/0/value), before P/samples/0/witness_id,
for all four fixed set shuffles. /CompetingBindings returns unexpected native
errors for e-b AND e-a; E(unavailable,/evidence/environments/e-a) wins, raw text
absent. /EscapedIDs uses c/a~b, requires /evidence/concepts/c~1a~0b/r-x and
rejects refusal path /evidence/concepts/c/a~b/r-x as malformed/out-of-binding.

TestEnvironmentCoverage_Determinism/CatalogUnknownWithLaterErrors makes
checker-contract bytes unavailable while every configured Environment and Concept
would return invalid_projection. Catalog's R(unknown_evidence,/evidence/catalog)
wins with the exact one-issue/null-cell result below; Catalog calls1,
Environment/Concept calls0. /CatalogKnownAbsentWithLaterErrors substitutes a
readable verified contract with a required declaration absent: identical result
shape/call counts, sole Reason absent_capability. Neither adds K or N.
/InvalidRequestBeforeCatalog combines unknown Purpose with unavailable checker
bytes: E(input,/purpose), normalized empty error result, ALL port calls0.
These outcomes hold across the same four set shuffles. /ConceptUnknownAfterCatalog
uses a successful complete Catalog and only unavailable r-x native receipt bytes:
local R(unknown_evidence,P/receipts/0) plus N(...,1), as frozen below; no K.

### Request, port and closure mutations by test

All healthy twins return PASS/nil with no issues. Each row is a separate negative
subtest; comma-separated lists expand to one mutation per named field, never a
diagnostic alternative. Computed shortages also cause N(...,1) when e-a alone
ceases locally qualifying for its exact claimed class. Assert that union, never
silently discard portfolio issues. With both environments similarly ineligible,
N(...,0) accompanies UNFIT. Removing all claims returns UNFIT, cells:[], issues:[].

| Test suffix / exact mutation | Exact tuple |
|---|---|
| defensive input / empty environment, requirement, witness, claim ID | E(input,/records/environments/0/id), /records/requirements/0/id, /records/witnesses/0/id, /records/coverage/0/id respectively; each isolated. Duplicate IDs use the same collection's second equal-ID slot /1/id. |
| defensive input / invalid tier, purpose, zero At | E(input,/tier), E(input,/purpose), E(input,/at), separately. |
| defensive input / malformed stratum language, subtype whitespace, prediction, empty vector, duplicate axis | E(input,/records/requirements/0/stratum/language), /stratum/subtype, /stratum/prediction, /stratum/scale, /stratum/scale/1/axis, respectively, under that same requirement prefix. |
| defensive input / invalid axis or class | E(input,/records/coverage/0/stratum/scale/0/axis), /class under the same scale element, separately. |
| defensive input / bad canonical ref path, SHA256, empty ref ID, malformed full repository commit | E(input,/catalog/path), E(input,/catalog/sha256), E(input,/catalog/id), E(input,/records/environments/0/repositories/0/commit), separately. |
| defensive input / unknown polarity, negative scale measurement, reversed anchor line range | E(input,/records/witnesses/0/polarity), E(input,/records/environments/0/scale/lines), E(input,/records/witnesses/0/anchor/end_line), separately. |
| defensive input / claim extra ref, wrong-stratum ref | E(input,/records/coverage/0/requirements/0) EACH, never missing_concept. |
| base / zero claims | No tuple; literal {"verdict":"UNFIT","cells":[],"issues":[]}, nil error. |
| MissingConcept / each authoritative claim requirement ref omitted | M(/records/coverage/0/requirements/0); absent slot is sorted expected ref position for that concept. Required result remains visible with null counts. |
| MissingConcept / required concept definition absent while catalog ref persists | M(/records/requirements/0) in each affected claim; exact global closure union below, not a stale native receipt. |
| OneShort / remove positive or negative from30 pairs | Positive removal: common twin positive-short,29/29; negative-only removal: negative-short,30/29. Exact ordered nested/global unions below. |
| OneShort / minimum31 with30 | Run positive-min31, negative-min31, both-min31-positive-short and both-min31-negative-short below; floors and resulting unions differ by side. |
| OneShort / minimum29 | E(input,/records/requirements/0/positive_minimum); negative minimum mutation uses /negative_minimum. |
| OneShort / surplus other language, vector, prediction | No local mutation: add valid witnesses in another requirement; prior complete common-twin union and29/29 or30/29 counts remain unchanged. A mismatched claim ref is separately E(input,/records/coverage/0/requirements/0). |
| Matching / no verified pairs; reuse1 positive for30 negatives; forged authored match labels with unpaired native projection | C(negative,0,30,G0); [Dn(1,30),Cn(1,30)]; C(negative,0,30,G0), respectively. |
| Matching / same normalized identity with conflicting polarity or match | E(projection,P/samples/0/polarity); E(projection,P/samples/0/match), separately. Conflict path is first canonical identity representative's field. |
| StalePin / same-ID changed environment ref/digest, repository commit, source_policy, licence, toolchain version/digest, image version/digest | R(stale_pin,/evidence/environments/e-a/head) for the head ref/digest. Other mutations use /evidence/environments/e-a/bindings followed by /repositories/0/commit, /source_policy, /licence, /toolchains/0/version, /toolchains/0/sha256, /images/0/version, /images/0/sha256, respectively. |
| StalePin / changed witness ref, requirement ref, native receipt digest, checker version, checker digest | R(stale_pin,P/samples/0/witness_id), R(stale_pin,P/requirement), R(stale_pin,P/receipts/0), R(stale_pin,P/checker/version), R(stale_pin,P/checker/sha256), separately. |
| StalePin / corresponding bytes missing/unreadable | For EVERY stale mutation above, identical Path with Reason unknown_evidence and Observed:null, Required:0. No stale tuple is accepted for missing bytes. |
| TrustedBoundary / nil port | E(input,/evidence); zero port calls. |
| TrustedBoundary / self-labelled witnesses, producer-only, unsupported source | R(unknown_evidence,P), each, with null counts. Fake refusal tests only; actual provenance replay belongs to native conformance. |
| TrustedBoundary / wrong projection environment ID, requirement ID, checker name | E(projection,P/environment/id), E(projection,P/requirement/id), E(projection,P/checker/name), separately. Same-ID changed digests instead use the StalePin rows. |
| TrustedBoundary / extra witness, empty normalized member Origin.Root, nonhistory Value, negative history Value, unknown metric, conflicting duplicate measure | E(projection,P/samples/0/witness_id), E(projection,Q(cross_file_paths,0)/0/origin/root), E(projection,P/measures/cross_file_paths/0/value), E(projection,P/measures/history_commits/0/value), E(projection,P/measures/unknown/0/metric), E(projection,P/measures/cross_file_paths/0), separately. |
| TrustedBoundary / expected refusal with reason missing_count, malformed pointer, out-of-binding pointer | E(projection,P) for EACH; only stale_pin/unknown_evidence/absent_capability accepted. |
| TrustedBoundary / unexpected port error | E(unavailable,P), sanitized two-key JSON and normalized empty result. |
| CloneInflation / duplicated anchor IDs, moved same-origin, descendant, padding, transitive alias chain | Each positive clone uses the consistent original pair identity: positive-clone in the common-count table,29/29 and ordered negative shortage, positive clone_inflation, positive shortage. Negative-only clones use negative-clone,30/29. Transitive components retain one consistent Match; conflicting Match uses Matching structural error. Harmless duplicates use healthy-positive-clone/healthy-negative-clone in that table. |
| CapabilityClosure / delete Entry, empty mapping after Catalog succeeds | For EACH domain and each inventory member, K(domain,id,/evidence/catalog/entries/0); empty mapping path appends /requirements. Slot follows expected key, not always index0; exact known-set omission unions below. |
| CapabilityClosure / readable verified checker contract lacks required declaration | Catalog returns R(absent_capability,/evidence/catalog), one global issue, null cells, no N; exact early-refusal result below. |
| CapabilityClosure / checker-contract dependency bytes unavailable | Catalog returns R(unknown_evidence,/evidence/catalog), one global issue, null cells, no K/N; exact early-refusal result below. Never infer absence from unavailable bytes. |
| CapabilityClosure / required concept removed from catalog requirements universe | M(/records/requirements/0) with ClaimID/RequirementID empty for a global inventory concept omission, plus affected-claim M and portfolio union specified below. Concept-specific claim omission uses MissingConcept row. |
| CapabilityClosure / extra Entry, duplicate Entry, duplicate Inventory key, absent ConceptObligation | E(projection,/evidence/catalog/entries/0), E(projection,/evidence/catalog/entries/0/key), E(projection,/evidence/catalog/inventory/0), E(projection,/evidence/catalog/concepts/0), respectively, at offending canonical slot. |
| CapabilityClosure / claim unknown capability; no exact-stratum mapping | K(capability,unknown,/records/coverage/0/capability); K(capability,opaque-fixture-card,/records/coverage/0/stratum), with ClaimID c-a (claim-scoped), separately. |
| CapabilityClosure / changed source bytes; missing member in native authoritative closure | R(stale_pin,/evidence/catalog); R(absent_capability,/evidence/catalog), global empty claim/requirement IDs. Native K with known missing member uses its exact key instead. |
| CapabilityClosure / malformed checker pin, Card ref, CheckerContract ref | E(input,/records/requirements/0/checker/sha256), E(input,/records/requirements/0/card/path), E(input,/records/requirements/0/checker_contract/sha256), respectively. |
| GroupClosure / empty required members | C(the metric,0,its exact floor,the required group). |
| GroupClosure / missing metric/group, extra group, wildcard group, unknown metric | E(projection,P/measures/<metric>/<canonical-group-index>); wildcard appends /group, unknown metric appends /metric, separately. Never a count shortage for a missing measure. |
| GroupClosure / polyglot catalog declares only2 languages | E(projection,/evidence/catalog/concepts/0/groups), not an invented language-count metric. Missing a returned declared language instead uses missing-measure row. |
| IndependentEnvironments / remove e-a claim; overlapping origins; alternate pins; seed replicas; transitive overlap | N(each claimed axis,class,1); no synthetic witness shortage. One environment with several repos still gives1. |
| IndependentEnvironments / public origin count1 with otherwise2 components | N(public_repositories,null,1) using public-origin expansion; exact scale cells remain eligible. |
| IndependentEnvironments / constructed request with public_outcome | E(input,/purpose). Mixed claimed tier -> E(input,/records/coverage/0/environment). Claimed smoke -> E(input,/records/coverage/0/environment); unclaimed smoke contributes0 without error. |
| NoMutation / repeat/private-copy/hostile fake mutations | Deep-equal originals; repeat bytes equal; mutate result without aliasing prior result/source. A hostile copy mutation returned as wrong binding is E(projection,P/environment/id), while original input stays equal. |
| Determinism / four fixed shuffles and simultaneous read-only calls | Same result and exact issue order; competing errors use fixtures above, all failure result arrays []. Concurrency is future -race acceptance, unrun here. |
| JSONVocabulary / unknown purpose, tier, metric, refusal reason | E(input,/purpose), E(input,/tier), E(projection,P/measures/unknown/0/metric), E(projection,P), respectively. Unknown enum is never an arbitrary reason string. |
| PhaseSeparation / change external W4/W5 outcomes | No tuple; byte-identical healthy PASS, no call to WPF/model/I/O/ambient clock. Fixed-At foreign-age below/equal/above native bound gives exact native valid/invalid membership; stale supporting receipt separately R(stale_pin,P/receipts/0). |

### Exact global closure status and issue union

Global catalog refusal R(stale_pin,/evidence/catalog),
R(unknown_evidence,/evidence/catalog) or R(absent_capability,/evidence/catalog)
prevents authoritative resolution for ALL claims: UNFIT, every requested cell's
counts null, requirements:[] and cell issues:[], with that ONE global R issue.
No per-concept or portfolio counts are computable, so no synthetic N is added.

A returned complete-source Inventory with one Entry missing instead permits
independent set comparison. Deleting a non-capability domain Entry, or an
unrequested capability Entry, gives ONE global K, other cells retain locally
measured30/30 and overall INCOMPLETE. Deleting the requested DomainCapability
opaque-fixture-card Entry, or emptying its mapping, gives the global K plus
claim-scoped K for c-a and c-b at their /records/coverage/<index>/capability,
null cell counts and requirements:[], N(each claimed axis,class,0), UNFIT.
Unavailable native checker-contract bytes, including a dependency shared by both
cells, cannot yield a successful Catalog. They take the ONE global
R(unknown_evidence,/evidence/catalog) early-refusal branch, never K, per-cell R
or N. Readable verified contract bytes with a required declaration known absent
take the ONE global R(absent_capability,/evidence/catalog) branch instead. An
absent/malformed authored CheckerContract Ref is E(input,...) before the port;
that is neither of these native-source cases. Missing ConceptObligation from an
otherwise returned projection is structural E(projection,...).

For these early refusals, stage2 calls Catalog once; Environment/Concept calls0,
nil error, exactly two requested cells c-a/c-b with PositiveCount/NegativeCount
null, Requirements:[], Issues:[], and the single R globally. The ordered global
union is [R(reason,/evidence/catalog)] with empty claim/requirement, Catalog:null,
Metric:none, G0, Scale:null, Observed:null, Required:0. This follows the verified
source law above and the unchanged peer law that "absent/unreadable bytes are unknown"
(`docs/contracts/env-1.md@44d4f4518:763-770`). After successful Catalog,
unavailable concept-only receipt bytes instead give R(unknown_evidence,
P/receipts/0), c-a counts null, its r-x counts null and Measures:[], the R in
requirement/cell unions, c-b30/30, and ordered global [N(...,1),R], INCOMPLETE.
That local receipt case adds no absent-capability issue.

Removing one required concept definition from the shared Records while its
catalog ref persists gives M at its expected-union /records/requirements/<slot>
for EACH c-a/c-b with that exact RequirementID, null required counts and
N(...,0), UNFIT. If the catalog requirement universe itself lacks an authoritative inventory
concept, add one global M (empty ClaimID/RequirementID) at that same slot;
local authoritative requirement refs still generate their c-a/c-b M. Removing
only a claim ref affects c-a alone and gives its M plus N(...,1), INCOMPLETE.
These rules specify complete unions and distinguish a malformed port shape
from complete but inadequate coverage; they never allow catalog omissions PASS.

### Literal failure JSON and collection assertions

TestEnvironmentCoverage_JSONVocabulary asserts these exact standalone values:

```json
{"verdict":"UNFIT","cells":[],"issues":[]}
```

```json
{"code":"invalid_input","path":"/purpose"}
```

```json
{"code":"invalid_projection","path":"/evidence/concepts/c-a/r-x/measures/cross_file_paths/0/value"}
```

```json
{"code":"evidence_unavailable","path":"/evidence/environments/e-a"}
```

The same test freezes these exact known-zero and unknown measurement/issue
values, using full explicit keys and no Path added to CoverageIssue:

```json
{"metric":"long_outputs","group":{"tool":"","kind":"","language":"","format":""},"observed":0,"required":1}
```

```json
{"claim_id":"c-a","requirement_id":"r-x","catalog":null,"metric":"none","group":{"tool":"","kind":"","language":"","format":""},"scale":null,"reason":"unknown_evidence","observed":null,"required":0}
```

The following FULL early-refusal results additionally freeze unavailable catalog
dependency bytes and known missing checker declarations, respectively. Both have
nil error; they are the complete ordered results, not fragments. No per-cell R,
K, portfolio N or partial measurement is present.

```json
{"verdict":"UNFIT","cells":[{"claim_id":"c-a","positive_count":null,"negative_count":null,"requirements":[],"issues":[]},{"claim_id":"c-b","positive_count":null,"negative_count":null,"requirements":[],"issues":[]}],"issues":[{"claim_id":"","requirement_id":"","catalog":null,"metric":"none","group":{"tool":"","kind":"","language":"","format":""},"scale":null,"reason":"unknown_evidence","observed":null,"required":0}]}
```

```json
{"verdict":"UNFIT","cells":[{"claim_id":"c-a","positive_count":null,"negative_count":null,"requirements":[],"issues":[]},{"claim_id":"c-b","positive_count":null,"negative_count":null,"requirements":[],"issues":[]}],"issues":[{"claim_id":"","requirement_id":"","catalog":null,"metric":"none","group":{"tool":"","kind":"","language":"","format":""},"scale":null,"reason":"absent_capability","observed":null,"required":0}]}
```

These are result and separate errors, never an undocumented combined envelope.
Assert exact field sets for every type in the Go declarations. Initialize ALL
nested requirements/measures/issues and receipt coverage/artifacts/pairs/outcomes/
samples/diagnostics as []. Input snapshot projections also allocate every exported
slice, including empty Families/Repositories/Receipts/Samples/Measures/Members/
Groups/Inventory/Entries/Concepts. Null remains legal for measured pointers and
inapplicable Catalog/Scale only, with known-zero Observed=0. No sentinel identities,
raw native output/error, local path, inferred subtype verdict or extra field.

## Assertion-complete concept mutation expansion

Every row expands under TestEnvironmentCoverage_ConceptSpecificFloors/<concept>.
Each listed metric has a separate exact-floor healthy twin (PASS/nil), then the
listed observed/required C tuple and resulting N(...,1). P's r-x becomes the exact
r-<concept> ID. Ungrouped index0 means G0. Hazards use index0=(tool:terraform,kind:delete)
and index1=(tool:terraform,kind:replace). Polyglot uses languages go/python/
typescript at indices0/1/2; guardrails uses kinds explicit/implicit at0/1;
foreign inputs uses formats json/yaml at0/1. Other tuple fields are empty.
Indexing includes the required empty tuple only for metrics that use it; for
grouped metric paths it indexes that metric's applicable sorted group set. Each group is
mutated independently with other groups above floor. Missing whole native backing
bytes always R(unknown_evidence,P/receipts/0), never a fabricated count0.
A missing prerequisite for ONE native member excludes just that member. Those raw
cases target the specific supplementary counts below; exact complete raw
projection/matched-count assertions require the native manifest acceptance just
specified. Projection-only twins prove aggregation without native verification.

| Obligation case/binding (not inventory membership) | Independent mutation -> exact C(metric,observed,required,group) or E |
|---|---|
| false-plan | absent_symbols0/1; unwired_entries0/1, G0 each. |
| cross-file | cross_file_paths29/30 G0, separately depth2 and only2 product files on one member. |
| cross-repository | repository_links29/30; linked_repositories2/3; linked_services2/3, G0 each. Nonlinks29 uses C(negative,29,30,G0). |
| fan-in-dispatch | fanin_symbols2/3 for two symbols, one symbol with4 calls, one with2 units, separately; dispatch_sites29/30; G0. |
| hazards | hazardous_changes29/30; safe_changes29/30, separately for EACH tool/kind group. |
| modules-unknowns-upgrades | module_expressions29/30; missing_context29/30; provider_transitions29/30, G0 each. |
| stale-rationale | stale_instructions29/30; live_instructions29/30; governing_links29/30, G0. Remove one governing anchor -> governing_links29/30. |
| fixes-mutants | public_fixes29/30; collateral_mutants29/30, G0. Remove one parent/test binding -> public_fixes29/30; one absent killed/survived label -> collateral_mutants29/30; index-derived replacement -> public_fixes29/30. |
| localization | nonlexical_issues29/30; lexical_issues29/30, G0. Missing base file/fix key on one nonlexical member -> nonlexical_issues29/30 separately. D-b change alone gives identical PASS bytes. |
| landing | conflicting_pairs29/30; clean_pairs29/30, G0. Missing merge, gate, overlap evidence on one conflicting member -> conflicting_pairs29/30 separately; unchecked clean absence -> clean_pairs29/30. |
| history | history_commits199/200; relevant_changes29/30, G0. Duplicate parent paths/message proxies leave199/29 unchanged, never inflate. Missing entire parent graph -> R(unknown_evidence,P/receipts/0). |
| polyglot-generated | boundary_sites29/30 G0; language_sites29/30 EACH declared language. Missing generator binding on one boundary -> boundary_sites29/30. Catalog with2 languages -> E(projection,/evidence/catalog/concepts/0/groups); missing third returned group -> E(projection,P/measures/language_sites/2). |
| data-subprocess | data_reads29/30; subprocess_reads29/30; uncovered_changes29/30, G0. Remove one read receipt from data/subprocess or one failure receipt from uncovered -> corresponding29/30. |
| lifetime | acquire_release_pairs29/30; missing_release_fixes29/30; escapes29/30, G0. Remove SSA binding on one pair, subject-test binding on one fix, incumbent support on one escape -> corresponding29/30, separately. Escape never fills positive release count. |
| growth-shutdown | bounded_sites29/30; unbounded_sites29/30; shutdown_cases29/30, G0. Remove one reachability binding for each site class or one runtime-leak binding for shutdown -> corresponding29/30. |
| shared-map | multisession_tasks29/30 G0. Remove one gold/T2/transcript binding ->29/30 separately. Repeat1 task30 times -> C(multisession_tasks,1,30,G0) plus clone_inflation same1/30. |
| guardrails | declared_deletions29/30; undeclared_deletions29/30 G0; rule_labels29/30 EACH kind. Remove one pipeline binding -> declared_deletions29/30; fix/mutant support on one label -> rule_labels29/30 for that kind, separately. |
| pins-contracts | pinned_inputs29/30; floating_inputs29/30; interface_changes29/30, G0. Remove one lock/API/catalog binding -> corresponding29/30, separately. |
| foreign-inputs | valid_envelopes29/30; invalid_envelopes29/30 EACH format. Remove one producer/commit/age binding -> valid_envelopes29/30 separately; a verified deliberately stale negative still qualifies invalid_envelopes. Stale backing receipt -> R(stale_pin,P/receipts/0). |
| runtime-identity | resolvable_bindings29/30; ambiguous_bindings29/30 G0. Conflate one declared/observed pair -> resolvable_bindings29/30; remove one ambiguous member's operator export -> ambiguous_bindings29/30. |
| output-judgment | output_cases29/30; question_cases29/30; EACH long_outputs/short_outputs/route_questions/rank_questions/table_questions0/1 G0 for aggregate. Change one output's lossless bytes -> output_cases29/30; remove one confirmed/native/frozen question key -> question_cases29/30. All per-subtype floor30 twins are required under SubtypeClaims above. |

## Exact D26 native-conformance wiring request and leaf tuples

T3-2 authorizes the port law, not an adapter/file grant. Request an explicit
native-owner serial grant for ONLY these proposed additional files:
`internal/benchenvnative/snapshot.go`, `internal/benchenvnative/snapshot_test.go`,
`internal/benchenvnative/testdata/d26/manifest.json`, and the subject/receipt files
listed next. No implementation exists here, and none is written in this repair.
The snapshot preparation owner may use I/O; internal/benchenv remains pure and
imports no adapter. This is a request needing owner acceptance, not ownership
claimed through this document.

Exact prospective normalizer test is TestNativeSnapshotNormalizer_D26 in
snapshot_test.go, with all nine subtest names below. Fixture roots are
`internal/benchenvnative/testdata/d26/go`, `.../python`, `.../typescript`.
Requested files are go/go.mod, go/target/target.go, go/a/a.go, go/b/b.go,
go/c/c.go, go/target/target_test.go; python/src/target/__init__.py,
python/src/a/a.py, python/src/b/b.py, python/src/c/c.py,
python/src/target/test_self.py; typescript/tsconfig.json,
typescript/src/target/index.ts, typescript/src/a/a.ts,
typescript/src/b/b.ts, typescript/src/c/c.ts,
typescript/src/target/index.test.ts; and each root's native-receipt.json.
The manifest binds all fixture file bytes, full source commits, native compiler/
SCIP pin/digest and raw output digest. Actual native payloads must be generated
by those pinned independent tools, never authored to match expected projections.
Raw receipts must preserve resolved target/site/unit and definition provenance;
C6Row-only or a fake projection is insufficient. Pinning real raw payload format
and its independent generation receipt is acceptance required of that owner;
unavailable payloads stay UNPROVEN. No invented native schema is accepted here.

The exact additional support-path fixtures are the union of strings in
ProductCallers.test_excludes_support_in_every_language and test_keeps_product at
`calibration/bench/tools/test_recount.py:12-40`. Request a file at EACH of those
literal relative paths under each applicable language root, with controlled
call/no-call bytes. The union is an exact source-pinned manifest expansion, not a
directory grant. Owner must enumerate bytes/hashes and accept the expanded list
before writing. Those tests are reused as path policy, not native identity proof.

TestNativeSnapshotNormalizer_Concepts in snapshot_test.go is additionally requested
for every authoritative inventory concept and all named BE-D2 obligation cases,
with the predicate/mutation projections specified above. Fixture names bind those
obligations; they do not enumerate production concept keys.
The owner supplies independently produced native fixtures for those concepts in
a separate exact manifest grant before implementation; no catch-all testdata
ownership or presumed payload proof follows. Until then, the named fake leaf twins prove
aggregation only and raw concept conformance is unproven.

D26 healthy raw fixture has3 product target symbols, each five distinct product
sites distributed2/2/1 across outside units a/b/c, plus30 dispatch/common pairs.
For EACH D26 test below, normalize raw receipt and assert exact site/unit sets
and fanin_symbols count; compare projection bytes with the fixed leaf fixture.
Then TestEnvironmentCoverage_D26/<name> asserts the indicated C/R tuple. Site
names/order are the fixture's canonical (path,line,column) keys; self/test sites
are absent, not just subtracted from an aggregate number. Mutate one target only
so two other qualifying symbols remain. Assertions below are fixed outcomes.

| D26 subtest (all9) | Raw/normalized assertion and exact leaf outcome |
|---|---|
| GoUnits | Native packages a/b/c ->5 sites/3 import-package units per target, fanin_symbols3 PASS. Move c's site to a different file in b ->5/2, target unqualified, C(fanin_symbols,2,3,G0). Defining and external target_test callers add0 sites/units; cannot repair2. |
| PythonUnits | a/b/c directories ->5/3, PASS. Several modules in a still1 unit; replace c with another b module ->5/2, C(fanin_symbols,2,3,G0). Defining-directory and tests add0. |
| TypeScriptUnits | a/b/c directories ->5/3, PASS. Additional exports/files in b add no new unit; replacing c ->5/2, C(fanin_symbols,2,3,G0). Defining/test calls add0. |
| ProductOnly | EACH Go/Python/TypeScript separately:4 product sites + EACH support-path caller ->4 product sites, target unqualified -> C(fanin_symbols,2,3,G0). Restore5 actual product calls across3 units ->PASS. Nested support and defining tests contribute0. |
| LandedPathPolicy | EVERY excluded source-pinned path -> product_path=false; EVERY retained product exception ->true with native membership checked. Replacing the fifth call with each excluded path ->C(fanin_symbols,2,3,G0); using each retained product path as fifth site across3 units ->PASS. Broad substring exclusion fails conformance. |
| UniqueSitesAndUnits | Duplicate native site rows leave4 sites/2 units ->C(fanin_symbols,2,3,G0);5 distinct sites/3 units ->PASS. Same identity conflicts ->E(projection,P/samples/0/match), never choose arbitrary row. |
| DefiningProductSymbol | Support-only definition excludes target despite5/3 callers ->C(fanin_symbols,2,3,G0); product definition with identical5/3 ->PASS. Unknown definition provenance ->R(unknown_evidence,P/receipts/0). |
| DispatchAndCommonFloor | Qualifying fanin_symbols3 alone with dispatch29 ->C(dispatch_sites,29,30,G0). Positive removal ->29/29 and BOTH shortages; negative-only removal ->30/29 and negative shortage. Run ALL common-count twins in the common-count table (including clones, side-specific/joint minima31), with fanin_symbols3 and dispatch30 unchanged; exact ordered unions/counts apply. Full3/30/30/30 ->PASS. |
| UnknownNativeTruth | Producer-only, C6Row.Qualifies alone, VetReceipt kept alone, missing compiler receipt, index-derived replacement, unsupported language EACH ->R(unknown_evidence,P/receipts/0), null counts. Independently bound native truth ->PASS. |

Actual native normalizer RED/GREEN and -race proof stays separate and unproven
until the requested files/raw fixture manifest are explicitly granted and those
tests run. Naming them, gofmt or a fake projection cannot establish that proof.

## Exact scale mutations for all16 paths

For each TestEnvironmentCoverage_Scale/<axis>/<class> in the existing16-path table,
use both independent environments measured at the same test value v. Hold all
other axes/metrics healthy. Finite-class v=lower,upper ->PASS/nil and exact class;
v=lower-1,upper+1 -> S(axis,class,v) for EACH environment and N(axis,class,0),
UNFIT. XL v=lower,lower+1 ->PASS; v=lower-1 ->S for each plus N(...,0), UNFIT.
Below S is tiny, never reclassified S. Being above a requested finite class is
also a scale mismatch, never borrowed into it. Required on S is always that
class's lower bound, even when observed is above its upper bound. The full class
limits in the existing table fix the upper-bound comparison; no arbitrary XL cap.

For EACH of16 paths independently, v=null -> R(unknown_evidence,
/evidence/environments/e-a/scale/<axis>) in e-a, c-a, empty requirement,
Observed:null/Required:0, plus N(...,1), INCOMPLETE. v=0 ->S(axis,class,0) and
N(...,1), known0 (other environment stays healthy). Large line values cannot
repair independently short files/repositories/history. Each weak axis gets its
own S plus N, exact axis/class attached. Null known/unknown is never merged.

For EACH axis/class, omit native scale receipt bytes ->R(unknown_evidence,
/evidence/environments/e-a/scale/receipt); change receipt bytes/digest ->
R(stale_pin,same path); both with null scale measurement, preserved independently known concept30/30,
and N(...,1). Missing typed receipt
Ref ->E(input,/records/environments/0/scale/receipt/id). Missing a typed excluded
category ->E(input,/records/environments/0/scale/excluded); duplicate category ->
E(input,/records/environments/0/scale/excluded/0/kind) at canonical duplicate
slot. Test each vendored/generated/test/build separately. Present measured-zero
excluded counts are accepted and preserved; different independently measured
Scale versus head ->R(stale_pin,/evidence/environments/e-a/scale/<axis>).
Removing the second eligible environment gives only N(...,1); repeated pins,
seed variants or a second environment joined through overlap still give1.
These16 subtests plus each null/zero/receipt/exclusion mutation assert exact
sorted global and cell issue unions under the diagnostic expansion, not just
verdict or reason presence.

## T3 dispositions and exact wiring requests

| Prior point | Settled disposition | Governing authority |
|---|---|---|
| ENV-1/U2 | Read-only exact Card.ID mapping; reuse Card/ProofRef/CheckCardClosure. No card writer or second API. | T3-1; CTR-13 capability contract cited above; BE-D2 closure. |
| ENV-1/U3 | One immutable snapshot port, lineage and native metric law reproduced above. Owners verify bytes/history before pure joins/counts. No native adapter now. | T3-2; BE-D2/D3/D9. |
| ENV-2/U1 | Floor30 per claimed stratum; aggregate30 and each subtype>=1 when aggregate only. Below30 descriptive only. | T3-3; subtype section below. |
| ENV-2/U2 | All nullable coverage/results/port types leaf-owned; authored inputs ENV-1-owned. | T3-4; exact committed peer declarations. |

The minimum input dependency is exactly ENV-1's EnvironmentRecords/Head,
Requirement (including Card and CheckerContract), Stratum (including Subtype),
Witness, Ref, Pin, RepositoryPin, Anchor, Scale/ScaleClaim, ConceptInventory,
Metric/MetricUnit/MinimumCitation/MetricExpectation and associated enums
(`docs/contracts/env-1.md@44d4f4518:127-330`). No parallel definitions or
additional schema fields beyond the T12-1 expectation amendment are requested. Implementation waits for ENV-1's
contract/implementation dependency; peer critic approval is still pending.

EnvironmentReceipt is leaf-owned to avoid a schema/package cycle. Under the
future handed coverage.go grant, carry this exact peer declaration
(`docs/contracts/env-1.md@44d4f4518:1456-1465`):

```go
type EnvironmentReceipt struct {
    Mode env.EnvironmentMode `json:"mode"`
    State env.ModeState `json:"state"`
    Coverage []CoverageResult `json:"coverage"`
    Artifacts []env.Ref `json:"artifacts"`
    Pairs []env.Ref `json:"pairs"`
    Outcomes []env.AgentOutcome `json:"outcomes"`
    Samples []env.SnapshotSample `json:"samples"`
    Diagnostics []env.ModeDiagnostic `json:"diagnostics"`
}
```

C0's separately owned integration must import schema inputs and internal/benchenv,
return benchenv.EnvironmentReceipt and assert nested [] in TestEnvironmentDispatch.
No C0 path or shared schema grant follows from this document. ENV-1's downstream
availability/policy/wiring limits remain theirs, not new leaf prerequisites.
Its R2 CLI/error repairs and retained legacy plan forms do not widen this leaf.
BEN-5 keeps admission; existing loaders stay read-only
(`internal/benchprep/manifest.go:76-80`; `internal/benchprep/split.go:145-160`).
Native adapter ownership/test requests below need an explicit serial grant.
WPF APIs, BEN-9 calendar/fingerprint, CLI registry, generator, model adapter and
paid gates do not block this pure coverage computation. A fake-port leaf PASS
still means native integration UNPROVEN. No coverage result authorizes actions,
effects, trial counts, holdout access or spending.

## Decision models

None. Coverage uses deterministic native receipts and set counts with provider
`none`. Model primary labels are refused. Agents remain experimental subjects;
no new judged site, scheduler or oracle is created
(`docs/design/bench-environments.md:269-274`).
