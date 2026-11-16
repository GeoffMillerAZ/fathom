# ENV-1: schema-core contract

Status: unpublished W18-1/W18-2 keyed-inventory amendment, retaining T12-1/FIX1,
with normal correction1 for evaluated CUE field semantics and commented negatives,
of the W16-2 **core only** publication at
source `c3e8988b9133649ccf432778493cc36c60c5a90b`, held by integration review.
The extracted core's provenance remains
`44d4f45184996c505e0bcc96d5ab5a8c65da56e4`. Independent amendment review is
required. The original full contract remains **HOLD**. Earlier approval applies only to
the exact previous core blob, never these amended bytes. This is
contract extraction, not implementation, admission, consumer dispatch, plan-row
completion or a waiver of the record-id laws for other authored ENV records.

## Scope and authority

Only `docs/contracts/env-1-core.md` is published here. The exact future core
implementation owned files are:

* `schema/benchenv/schema.cue`
* `schema/benchenv/validate.go` (including the embedded schema)
* `schema/benchenv/validate_test.go`
* `schema/benchenv/testdata/valid.cue`
* `schema/benchenv/testdata/invalid.cue`

No wider implementation grant is made. The core consists of the reproduced
authored Go/CUE data declarations, pure local validators, error constructor law,
fixtures and schema-test assertions. The source identity identifies extraction
provenance, not a dependency on an unpublished parent. All operative core text is
included here. The extraction manifest describes the pre-amendment blocks;
W16-2 changes concept shape/inventory/join text and W3-4 corrects GeneratorVersion.
W18-1/W18-2 replace the concept array with one keyed CUE struct, retaining
T12-1/FIX1 typed expectations; prior extraction/block identities remain
historical provenance. Fresh independent review and a new source-bound handoff
are required. Parent source assembly is not wave integration.
D-number references retain their source decision identities. Future inventory
data has one proposed owned path, `calibration/environments/concepts.cue`, in a
separate critic-reviewed data lane. That path is read-only to core consumers;
neither its contents nor a write grant is supplied by this publication.

Governing public sources are [CU-D2/D3 and CU-D6/D7](../design/typed-records.md),
[BE-D9 and the benchmark design](../design/bench-environments.md),
[ENV-1 and the fleet handoff](../plans/bench-environments.md),
[REC-1](rec-1.md) and [the serial integration grants](records-integration.md).
W2-3 approves by rule after independent LAND with governing citations. W3-2 leaves
real assertion RED to the implementer. W3-4 supplies the API house rules preserved
below. T3-1..4 keep authored inputs here and derived coverage and native ports in
their separate owner. Ruling names identify the supplied approval provenance,
not files adopters must obtain to interpret these declarations.

T6 has settled U5/U6, without expanding this publication grant: ENV defines the
ordered fingerprint artifact; BEN-9 alone owns its hasher and canonical bytes.
Materialized fleets belong only in gitignored workspace storage and are never
committed; generated Git identity uses a neutral `.invalid` domain. Those are
execution ownership boundaries, not added core execution obligations. This
publication includes no fingerprint/age verdict, hasher, generator signature or
seed-expansion/Git/size/limit/materialization execution protocol. Fleet DTOs,
schema, fixture values and pure manifest/DAG validation remain in scope.

CLI transport, registration and dispatch, the held F4 seam, leaf coverage and
native ports/results, and EnvironmentReceipt remain excluded. Core NewModeError
stays here; C0's ModeErrorCause test remains C0's unchanged obligation.
RecordOwnership retains its upstream REC dependency for other authored ENV
records only. Concept struct validation and its single-file path grant need
nothing from the record id checker (W18-1/W18-2). There is no waiver for other records.

## Intent preserved from the source

The intent router is `docs/intent/README.md`, sections Routing and Before designing.
P2/P3/P6/P7/P8/P12/P17 require deterministic, visible, stamped evidence and a
counterfactual (`docs/intent/principles.md:12-17,25-33,45-47,63-64`). I43, I54,
I56 and I62 apply with their stated enforcement limits
(`docs/intent/invariants.md:73,83,85,90`). No dispatch loop or new product analyzer
is introduced (`docs/intent/scope/non-goals.md:27-31`;
`docs/design/bench-environments.md:111-116,239-245`).

The benchmark-validity evidence trigger was already raised by the governing design
(`docs/design/bench-environments.md:23-28`). It reopens unsupported empirical
conclusions, not ownership, licence boundaries or feature activation. No new
trigger firing is established here. ADR-108/109/110 remain proposed records, used
as this lane's specified design context, not represented as delivered policy
(`ADR.md:7061-7067,7132-7137,7202-7205`). No documented future is implemented.
Separate keys and receipt references preserve later additive consumers at no extra
runtime cost. No existing compatibility surface changes in this documentation lane.

## Preserved decision identities

| Point | Choice and boundary | Governing text |
|---|---|---|
| ENV-1/D1 | Standalone CUE data, additive Go mirrors in `schema/benchenv/validate.go`. No prose migration, central records loader or new registry. | CU-D2, `docs/design/typed-records.md:89-95`; ENV-1 ownership, `docs/plans/bench-environments.md:33-34` |
| ENV-1/D2 | Authored heads, requirements and measured witnesses are inputs. No authored adequate, admitted, valid, fresh or passing coverage status. Pure evaluators derive results. | BE-D9, `docs/design/bench-environments.md:237-245`; CU-D3, `docs/design/typed-records.md:97-118` |
| ENV-1/D3 | Count independently checked anchors, matched negatives and deduplicated origins separately in each full stratum. Thirty per side is a floor, not a power calculation. | BE-D2/D3, `docs/design/bench-environments.md:79-125`; ADR-109, `ADR.md:7148-7152` |
| ENV-1/D4 | Explicit four-axis scale claims, two independent eligible environments per claimed axis/class, and two public repositories for public outcome claims. No smoke contribution. | BE-D1/D4, `docs/design/bench-environments.md:70-77,127-144` |
| ENV-1/D5 | Reuse subject manifests, vet receipts, D26 rows, split calculation, scenario cell identity and preregistration checking. Their existing limitations remain visible. | BE-D3/D6, `docs/design/bench-environments.md:120-125,197-208`; `internal/benchprep/{manifest.go,vet.go,split.go}` and `internal/scenariobench/{types.go,cell.go}` |
| ENV-1/D6 | Preflight references and agent outcomes are separate types. ENV does not define WPF's ladder or its gate. Both arms pass preflight before either agent starts. Zero use stays assigned. | BE-D7, `docs/design/bench-environments.md:210-215`; WP-D1/D2, `docs/design/wiring-proof.md:23-45`; correction 1, `docs/design/bench-prep.md:36-42,371-375` |
| ENV-1/D7 | Fresh-only re-test envelopes preserve raw digests and original provenance. Audit history cannot be relabelled as a fresh run. All validity flags survive precedence. | BV-D1/D2, `docs/design/bench-environments.md:32-68`; ENV-C4/C8, `docs/plans/bench-environments.md:61-74`; `ADR.md:7100-7104` |
| ENV-1/D9 | Named string enums, explicit lower snake case JSON, typed refusals, no context on pure calls, non-nil exported result slices. No language-door dependency or new module dependency. | Architect W3-4, `RULINGS-W3.md`, lines 5-15 and 25-29 in the runner's governing context; `internal/AGENTS.md:3-18` |
| ENV-1/D10 | Other authored ENV records retain shared record-id checks. Concept ids are solely the field names of one CUE struct in one file: conflicting content fails CUE validation; the lane exact-file path grant is enforced by pathcheck. No concept recordchecker dependency or second identity. | W18-1/W18-2 supersede the concept prerequisite; CU-D6/D7 and W2-2 remain for other records |
| ENV-1/D12 | Typed metric/minimum/unit expectations in each inventory record, default30 with exact cited smaller supplementary floors; inventory plus measured snapshot is the pure leaf boundary. | T12-1; T3-3; W3-4; BE-D2/BE-D9 |
| ENV-1/D11 | Contract the concept record shape; keep open keys and Card.ID/group mappings in one critic-reviewed inventory data file. No contract key list or new card API. | Architect W16-2; T3-1; BE-D2/BE-D9, `docs/design/bench-environments.md` |

D8's materializer behavior is outside this core. References to native adapters,
the T3 mapping and supplementary concept conditions in the schema rules below
identify external evidence boundaries, not a port or coverage API supplied here.
The rules' reference to the T3 port points to that separate native owner; it does
not require a declaration omitted from this core to validate local shape.

## Other authored ENV records: shared record checker prerequisite

* `recordids.Check(root string, cfg Config) ([]Duplicate, error)` exists
  (`internal/recordids/recordids.go:18-35,50-72,91-95`). Its default families
  describe Markdown, not ENV CUE keys. REC-1 is approved **contract only**
  (`docs/contracts/rec-1.md:3-8,100-140`): `ValidatedInventory`, its constructor,
  `AllocateRecordID`, `schema/records` and `internal/records` are absent at this
  base. ENV must not create placeholder versions or claim that `Check` already
  validates CUE ownership.

| Durable point | Exact prerequisite and upstream disposition |
|---|---|
| ENV-1/U1 | Other authored ENV records retain REC-1 implementation/effective grants and their discovery prerequisite. Serial turns remain REC-1 -> heads -> BEN-3; no guessed allocator or exceptions. W18-1/W18-2 remove concept inventory keys, validation and path ownership from this prerequisite. `docs/contracts/rec-1.md:100-175`; `docs/contracts/records-integration.md:36-60`. |

The RecordOwnership test below concerns only other authored ENV records and stays
blocked by U1 until their existing shared checker/discovery/grants are supplied.
It does not discover, classify or validate concept struct keys, and is not a
prerequisite to validating or using a separately granted concept inventory.
W18-1 defers the shared concept-record extension to REC-6 under its existing
owner; this contract supplies no extension, alternate checker or proposal edit.

W18-3 disposition: the existing keyed CUE struct/schema, valid-invalid pair and
`cue vet`, plus one exact-file lane path grant enforced by pathcheck, suffice.
Use them; no new machinery is required.

## Exact data vocabulary

Authored declarations in this section stay in package `benchenv` at
`schema/benchenv/validate.go`. Card/CheckerContract refs and Subtype metadata
make the authored inputs to the T3-settled mapping/port explicit; implementation
is pending. Identifiers are opaque nonempty keys, not new
record-id families or reservations. Digests are lowercase 64-hex SHA-256 of the
referenced exact file bytes. Commits are full lowercase 40-hex git commits, as
the existing subject schema requires (`schema/bench/schema.cue:12-25`).
Ordered lists retain authored semantic order. Sets are normalized at result time.
The type named `Ref` is a content reference, never evidence that bytes were checked.
The shape validator cannot compare a reference digest to external file bytes.

```go
type Tier string
const (
    TierPublic Tier = "public"
    TierConstructed Tier = "constructed"
    TierSmoke Tier = "smoke"
)
type Split string
const (
    SplitDevelopment Split = "dev"
    SplitHeldOut Split = "held-out"
)
type Axis string
const (
    AxisLines Axis = "lines"
    AxisFiles Axis = "files"
    AxisRepositories Axis = "repositories"
    AxisHistory Axis = "history"
)
type ScaleClass string
const (
    ClassS ScaleClass = "S"
    ClassM ScaleClass = "M"
    ClassL ScaleClass = "L"
    ClassXL ScaleClass = "XL"
)
type Prediction string
const (
    PredictionEffect Prediction = "effect"
    PredictionNull Prediction = "null"
)
type Polarity string
const (
    PolarityPositive Polarity = "positive"
    PolarityNegative Polarity = "negative"
)
type ExclusionKind string
const (
    ExclusionVendored ExclusionKind = "vendored"
    ExclusionGenerated ExclusionKind = "generated"
    ExclusionTest ExclusionKind = "test"
    ExclusionBuild ExclusionKind = "build"
)
type Ref struct {
    ID string `json:"id"`
    Path string `json:"path"`
    SHA256 string `json:"sha256"`
}
type RepositoryPin struct {
    ID string `json:"id"`
    URL string `json:"url"`
    Commit string `json:"commit"`
}
type Pin struct {
    Name string `json:"name"`
    Version string `json:"version"`
    SHA256 string `json:"sha256"`
}
type ExcludedCount struct {
    Kind ExclusionKind `json:"kind"`
    Lines int64 `json:"lines"`
    Files int64 `json:"files"`
}
type Scale struct {
    Lines *int64 `json:"lines"`
    Files *int64 `json:"files"`
    Repositories *int64 `json:"repositories"`
    History *int64 `json:"history"`
    Excluded []ExcludedCount `json:"excluded"`
    Receipt Ref `json:"receipt"`
}
type ScaleClaim struct {
    Axis Axis `json:"axis"`
    Class ScaleClass `json:"class"`
}
type EnvironmentHead struct {
    ID string `json:"id"`
    Tier Tier `json:"tier"`
    Repositories []RepositoryPin `json:"repositories"`
    Subjects []Ref `json:"subjects"`
    Licence Ref `json:"licence"`
    Toolchains []Pin `json:"toolchains"`
    Images []Pin `json:"images"`
    SourcePolicy Ref `json:"source_policy"`
    Scale Scale `json:"scale"`
    Split Split `json:"split"`
    PriorReads []Ref `json:"prior_reads"`
    Oracles []Ref `json:"oracles"`
    Receipts []Ref `json:"receipts"`
}
type Stratum struct {
    Subtype string `json:"subtype"`
    Language string `json:"language"`
    Scale []ScaleClaim `json:"scale"`
    Prediction Prediction `json:"prediction"`
}
type Requirement struct {
    ID string `json:"id"`
    Capability string `json:"capability"`
    Card Ref `json:"card"`
    CheckerContract Ref `json:"checker_contract"`
    Concept string `json:"concept"`
    Stratum Stratum `json:"stratum"`
    PositiveMinimum int `json:"positive_minimum"`
    NegativeMinimum int `json:"negative_minimum"`
    Checker Pin `json:"checker"`
}
type Anchor struct {
    Repository RepositoryPin `json:"repository"`
    Path string `json:"path"`
    StartLine int `json:"start_line"`
    EndLine int `json:"end_line"`
}
type Witness struct {
    ID string `json:"id"`
    Environment Ref `json:"environment"`
    Requirement Ref `json:"requirement"`
    Anchor Anchor `json:"anchor"`
    Polarity Polarity `json:"polarity"`
    MatchID string `json:"match_id"`
    Origin Ref `json:"origin"`
    Checker Pin `json:"checker"`
    Receipt Ref `json:"receipt"`
}
type CoverageClaim struct {
    ID string `json:"id"`
    Capability string `json:"capability"`
    Environment Ref `json:"environment"`
    Stratum Stratum `json:"stratum"`
    Requirements []Ref `json:"requirements"`
}
type EnvironmentRecords struct {
    Environments []EnvironmentHead `json:"environments"`
    Requirements []Requirement `json:"requirements"`
    Witnesses []Witness `json:"witnesses"`
    Coverage []CoverageClaim `json:"coverage"`
}
```

Representation rules are D1-D5/D9. `Ref.Path` and anchor paths are canonical,
repository-relative slash paths, never URLs, colon-bearing segments, absolute paths, `.` or `..` segments,
backslashes, NULs or controls. A reference resolves by both ID and SHA-256, not
filename alone. Native path parameters for ENV-C9 are a separate local input,
never stored in these public records. Repository URLs must identify public
repositories for the public tier, have no credentials/query/fragment and match
the referenced manifest's normalized identity. Constructed repository URLs are
empty strings, with IDs and commit hashes supplied by the fleet manifest.
Smoke records remain local and are rejected by public export and coverage claims.

Missing measured counts are JSON `null` / CUE `null`, represented by nil pointers,
not zero. They are well-shaped unknown input and cannot pass admission. A known
zero uses a pointer to zero. Every excluded category is reported once, including
zero, and includes generated counts. The scale receipt supplies the exclusion
policy and reachable-unique commit calculation (BE-D4). Images may be `[]` for
a native environment; inventing a container image is forbidden. No map of secret
values, setup commands or transcript text is added. An environment references all
member licence receipts through `Receipts`; `Licence` identifies the licence
inventory, not one licence borrowed for every member.

Strata are full tuples: language, axis/class vector, predicted effect/null and
Subtype. Empty Subtype is aggregate, never unknown. Nonempty Subtype is the exact
card-declared selector from the pinned claim/checker contract (T3-3), never an
inferred category or a new enum. Whitespace-only Subtype refuses as shape.
Witness membership is inherited from the exact requirement reference. A matched
positive/negative pair shares `MatchID` within environment/requirement. `Origin`
points to independent lineage evidence, not a worker-selected clone counter.
The checker must inspect that evidence before counting. Duplicated anchors,
fix descendants, generated padding, repeated pins or seed-only variants do not
create new independent sites or environments (BE-D2/D3/D5).

Requirement.Capability and CoverageClaim.Capability are exact approved CTR-13
Card.ID keys, never titles or Surface strings. Requirement.Concept is an open
data key resolved through the single inventory below, never a contract enum or
a fixed-size list. Requirement.Card
references the exact frozen Card bytes, with Card.ID equal to Capability.
CheckerContract references the native contract that freezes grouped obligations,
normalization version and subtype selectors; Checker still pins the actual
checker version/digest. Their digests need not equal: code and contract are
different artifacts. Shape validation checks their syntax and Card.ID/Capability
identity only, never external bytes. T3-1/2 adapters verify both references.
They bind a checker version/digest. Supplementary BE-D2 conditions are checked
by that concept's independent oracle receipt, not by raising a generic authored
`qualified` flag. Each inventory key cites the capability card or BE-D2/BE-D9
design row requiring it. The BE-D2 table supplies cited obligations, not a closed
key vocabulary or a fixed inventory cardinality. Adding a key changes reviewed
inventory data, not this contract. The common floor and cited supplementary
obligations remain mandatory.
In particular, 30 shallow paths cannot stand in for the cross-file depth/file
minimum, and 30 links within two repositories cannot satisfy the three-repository
and three-service requirement. All other row-specific subcounts remain mandatory.
T3-2 settles the raw-oracle-to-witness immutable snapshot port and normalization
law (external native-owner boundary). Adapter implementation and native conformance remain unproven, not
unresolved representation authority.

## T12-1 typed expected-metric ownership

T12-1 amends W16's record shape only. The single inventory is the independent
source of expected metric obligations, not an observation or a checker-ref lookup.
`schema/benchenv` owns Metric, MetricUnit, MinimumCitation and MetricExpectation
alongside ConceptRecord. This relocates the existing closed Metric vocabulary
without changing any member or wire value. `internal/benchenv` aliases that one
vocabulary; schema never imports the leaf. Native ports, projections, predicate
normalization, group selectors and their owners are unchanged. There is no second
canonical metric mapping and no switch on opaque concept keys or checker refs.

```go
type Metric string
const (
    MetricNone Metric = "none"
    MetricPositive Metric = "positive"
    MetricNegative Metric = "negative"
    MetricAbsentSymbols Metric = "absent_symbols"
    MetricUnwiredEntries Metric = "unwired_entries"
    MetricCrossFilePaths Metric = "cross_file_paths"
    MetricRepositoryLinks Metric = "repository_links"
    MetricLinkedRepositories Metric = "linked_repositories"
    MetricLinkedServices Metric = "linked_services"
    MetricFaninSymbols Metric = "fanin_symbols"
    MetricDispatchSites Metric = "dispatch_sites"
    MetricHazardousChanges Metric = "hazardous_changes"
    MetricSafeChanges Metric = "safe_changes"
    MetricModuleExpressions Metric = "module_expressions"
    MetricMissingContext Metric = "missing_context"
    MetricProviderTransitions Metric = "provider_transitions"
    MetricStaleInstructions Metric = "stale_instructions"
    MetricLiveInstructions Metric = "live_instructions"
    MetricGoverningLinks Metric = "governing_links"
    MetricPublicFixes Metric = "public_fixes"
    MetricCollateralMutants Metric = "collateral_mutants"
    MetricNonlexicalIssues Metric = "nonlexical_issues"
    MetricLexicalIssues Metric = "lexical_issues"
    MetricConflictingPairs Metric = "conflicting_pairs"
    MetricCleanPairs Metric = "clean_pairs"
    MetricHistoryCommits Metric = "history_commits"
    MetricRelevantChanges Metric = "relevant_changes"
    MetricBoundarySites Metric = "boundary_sites"
    MetricLanguageSites Metric = "language_sites"
    MetricDataReads Metric = "data_reads"
    MetricSubprocessReads Metric = "subprocess_reads"
    MetricUncoveredChanges Metric = "uncovered_changes"
    MetricAcquireReleasePairs Metric = "acquire_release_pairs"
    MetricMissingReleaseFixes Metric = "missing_release_fixes"
    MetricEscapes Metric = "escapes"
    MetricBoundedSites Metric = "bounded_sites"
    MetricUnboundedSites Metric = "unbounded_sites"
    MetricShutdownCases Metric = "shutdown_cases"
    MetricMultisessionTasks Metric = "multisession_tasks"
    MetricDeclaredDeletions Metric = "declared_deletions"
    MetricUndeclaredDeletions Metric = "undeclared_deletions"
    MetricRuleLabels Metric = "rule_labels"
    MetricPinnedInputs Metric = "pinned_inputs"
    MetricFloatingInputs Metric = "floating_inputs"
    MetricInterfaceChanges Metric = "interface_changes"
    MetricValidEnvelopes Metric = "valid_envelopes"
    MetricInvalidEnvelopes Metric = "invalid_envelopes"
    MetricResolvableBindings Metric = "resolvable_bindings"
    MetricAmbiguousBindings Metric = "ambiguous_bindings"
    MetricOutputCases Metric = "output_cases"
    MetricLongOutputs Metric = "long_outputs"
    MetricShortOutputs Metric = "short_outputs"
    MetricQuestionCases Metric = "question_cases"
    MetricRouteQuestions Metric = "route_questions"
    MetricRankQuestions Metric = "rank_questions"
    MetricTableQuestions Metric = "table_questions"
)
type MetricUnit string
const (
    MetricUnitSymbols MetricUnit = "symbols"
    MetricUnitEntryPoints MetricUnit = "entry_points"
    MetricUnitPaths MetricUnit = "paths"
    MetricUnitLinks MetricUnit = "links"
    MetricUnitRepositories MetricUnit = "repositories"
    MetricUnitServices MetricUnit = "services"
    MetricUnitSites MetricUnit = "sites"
    MetricUnitChanges MetricUnit = "changes"
    MetricUnitExpressions MetricUnit = "expressions"
    MetricUnitCases MetricUnit = "cases"
    MetricUnitTransitions MetricUnit = "transitions"
    MetricUnitInstructions MetricUnit = "instructions"
    MetricUnitFixes MetricUnit = "fixes"
    MetricUnitMutants MetricUnit = "mutants"
    MetricUnitIssues MetricUnit = "issues"
    MetricUnitPairs MetricUnit = "pairs"
    MetricUnitCommits MetricUnit = "commits"
    MetricUnitReads MetricUnit = "reads"
    MetricUnitEscapes MetricUnit = "escapes"
    MetricUnitTasks MetricUnit = "tasks"
    MetricUnitDeletions MetricUnit = "deletions"
    MetricUnitLabels MetricUnit = "labels"
    MetricUnitInputs MetricUnit = "inputs"
    MetricUnitEnvelopes MetricUnit = "envelopes"
    MetricUnitBindings MetricUnit = "bindings"
    MetricUnitOutputs MetricUnit = "outputs"
    MetricUnitQuestions MetricUnit = "questions"
)
type MinimumCitation string
const (
    MinimumFalsePlan MinimumCitation = "BE-D2:False plan claims"
    MinimumCrossRepository MinimumCitation = "BE-D2:Cross-repository dependencies"
    MinimumFanin MinimumCitation = "BE-D2:Fan-in and dispatch"
    MinimumOutput MinimumCitation = "BE-D2:Output fidelity and judgments"
)
type MetricExpectation struct {
    Metric Metric `json:"metric"`
    Minimum int64 `json:"minimum"`
    Unit MetricUnit `json:"unit"`
    LowerMinimum MinimumCitation `json:"lower_minimum,omitempty"`
}
```

The following is the sole canonical metric/unit/minimum relation. Unit labels
name the existing BE-D2 populations, not conversion factors or new counters.
The unchanged ENV-2 predicate table defines qualification and normalization for
those populations. All counts apply independently in every claimed full stratum
and applicable native-declared group. Common positive/negative minima remain
at least30 and are not lowered by a supplementary expectation. MetricNone,
MetricPositive and MetricNegative are output-only, never inventory expectations.

| Metric | Unit | Smallest permitted minimum | Citation required below30 |
|---|---|---|---|
| `absent_symbols` | `symbols` | 1 | `BE-D2:False plan claims` |
| `unwired_entries` | `entry_points` | 1 | `BE-D2:False plan claims` |
| `cross_file_paths` | `paths` | 30 | none permitted |
| `repository_links` | `links` | 30 | none permitted |
| `linked_repositories` | `repositories` | 3 | `BE-D2:Cross-repository dependencies` |
| `linked_services` | `services` | 3 | `BE-D2:Cross-repository dependencies` |
| `fanin_symbols` | `symbols` | 3 | `BE-D2:Fan-in and dispatch` |
| `dispatch_sites` | `sites` | 30 | none permitted |
| `hazardous_changes` | `changes` | 30 | none permitted |
| `safe_changes` | `changes` | 30 | none permitted |
| `module_expressions` | `expressions` | 30 | none permitted |
| `missing_context` | `cases` | 30 | none permitted |
| `provider_transitions` | `transitions` | 30 | none permitted |
| `stale_instructions` | `instructions` | 30 | none permitted |
| `live_instructions` | `instructions` | 30 | none permitted |
| `governing_links` | `links` | 30 | none permitted |
| `public_fixes` | `fixes` | 30 | none permitted |
| `collateral_mutants` | `mutants` | 30 | none permitted |
| `nonlexical_issues` | `issues` | 30 | none permitted |
| `lexical_issues` | `issues` | 30 | none permitted |
| `conflicting_pairs` | `pairs` | 30 | none permitted |
| `clean_pairs` | `pairs` | 30 | none permitted |
| `history_commits` | `commits` | 200 | none permitted |
| `relevant_changes` | `changes` | 30 | none permitted |
| `boundary_sites` | `sites` | 30 | none permitted |
| `language_sites` | `sites` | 30 | none permitted |
| `data_reads` | `reads` | 30 | none permitted |
| `subprocess_reads` | `reads` | 30 | none permitted |
| `uncovered_changes` | `changes` | 30 | none permitted |
| `acquire_release_pairs` | `pairs` | 30 | none permitted |
| `missing_release_fixes` | `fixes` | 30 | none permitted |
| `escapes` | `escapes` | 30 | none permitted |
| `bounded_sites` | `sites` | 30 | none permitted |
| `unbounded_sites` | `sites` | 30 | none permitted |
| `shutdown_cases` | `cases` | 30 | none permitted |
| `multisession_tasks` | `tasks` | 30 | none permitted |
| `declared_deletions` | `deletions` | 30 | none permitted |
| `undeclared_deletions` | `deletions` | 30 | none permitted |
| `rule_labels` | `labels` | 30 | none permitted |
| `pinned_inputs` | `inputs` | 30 | none permitted |
| `floating_inputs` | `inputs` | 30 | none permitted |
| `interface_changes` | `changes` | 30 | none permitted |
| `valid_envelopes` | `envelopes` | 30 | none permitted |
| `invalid_envelopes` | `envelopes` | 30 | none permitted |
| `resolvable_bindings` | `bindings` | 30 | none permitted |
| `ambiguous_bindings` | `bindings` | 30 | none permitted |
| `output_cases` | `outputs` | 30 | none permitted |
| `long_outputs` | `outputs` | 1 | `BE-D2:Output fidelity and judgments` |
| `short_outputs` | `outputs` | 1 | `BE-D2:Output fidelity and judgments` |
| `question_cases` | `questions` | 30 | none permitted |
| `route_questions` | `questions` | 1 | `BE-D2:Output fidelity and judgments` |
| `rank_questions` | `questions` | 1 | `BE-D2:Output fidelity and judgments` |
| `table_questions` | `questions` | 1 | `BE-D2:Output fidelity and judgments` |

Each citation is an exact row selector in
[BE-D2](../design/bench-environments.md):
False plan claims (line89, inclusion of both categories), Cross-repository
dependencies (line91, three repositories/services), Fan-in and dispatch (line92,
three qualifying symbols), or Output fidelity and judgments (line109, aggregate
presence, interpreted by T3-3). ENV-2's unchanged predicates make inclusion at
least1; these citations never reduce common30/30. The output presence exception
applies only to aggregate Subtype:"". An explicitly claimed subtype retains its
own common30/30 and its selected metrics30. History keeps its explicit200 floor.
A larger authored minimum is allowed, never clamped downward.

CUE defaults an omitted `minimum` to30; validation emits that concrete int64.
An explicitly supplied null, zero, negative, fractional or overflowing value is
invalid, never defaulted. A history expectation must explicitly reach200.
Typed Go input has no absence marker: Minimum0 is invalid, not shorthand for30.
A below30 minimum requires the exact matching row selector above and must still
meet its smaller floor. At30 or above LowerMinimum must be absent (Go zero value).
Unknown citations, metric/unit combinations and extra fields refuse shape.

A ConceptRecord has a nonempty Expectations array. One record denotes one
independently expected metric set. Requirements with different aggregate/subtype
sets use separate critic-reviewed records through the existing opaque Concept
field; no selector API or production keys are introduced. Every expectation
applies to every requirement referencing that record, in the requirement's full
stratum. For an explicit subtype, the pure join rejects any output-presence
expectation below30 at its `/inventory/concepts/<key>/expectations/j/minimum` with
ValidationReference. The data critic and native owner verify the selected set
against the cited design and exact Card.Claim/CheckerContract bytes. Schema shape
cannot establish this semantic justification or supply missing native fixtures.
No metric is inferred from whatever the measured projection happens to contain.

## Concept record shape and single inventory

These additive declarations belong only in `schema/benchenv/validate.go`; their
CUE projection belongs only in `schema/benchenv/schema.cue`. Fixtures and every
test below use the already owned `testdata/{valid,invalid}.cue` and
`validate_test.go`. No new implementation file or data-file write is authorized.
`calibration/environments/concepts.cue` is the sole proposed production inventory,
with container exactly `inventory: #ConceptInventory & {...}`. Its `concepts`
member is ONE CUE struct keyed by concept id; each value carries its T12 typed
expectations and citations. The same file holds the read-only T3 Card.ID/group
mapping. Group concept members are references, never a second identity roster.
No second inventory,
compiled-in fallback, generated key enum or contract table defines its members.
Any concept-key examples are non-normative and cannot close the vocabulary.

```go
type ConceptDesignRow string
const (
    ConceptDesignBED2 ConceptDesignRow = "BE-D2"
    ConceptDesignBED9 ConceptDesignRow = "BE-D9"
)
type ConceptCitation struct {
    Card *Ref `json:"card,omitempty"`
    DesignRow ConceptDesignRow `json:"design_row,omitempty"`
}
type ConceptRecord struct {
    RequiredBy []ConceptCitation `json:"required_by"`
    Expectations []MetricExpectation `json:"expectations"`
}
type ConceptGroup struct {
    Card Ref `json:"card"`
    CheckerContract Ref `json:"checker_contract"`
    Concepts []string `json:"concepts"`
}
type ConceptInventory struct {
    Concepts map[string]ConceptRecord `json:"concepts"`
    Groups []ConceptGroup `json:"groups"`
}
func ValidateConceptInventory(source []byte) (ConceptInventory, error)
func ValidateConceptInventoryValue(inventory ConceptInventory) error
func ValidateEnvironmentConceptJoin(records EnvironmentRecords,
    inventory ConceptInventory, cardIDs []string) error
```

Concept ids are the evaluated CUE struct field names, not raw source label
spellings. Existing CUE evaluation determines that identity; ENV introduces no
parser, normalizer, alternate identity policy, prefix or numeric range. For
example decomposed source label `e\u0301` (U+0065 U+0301, UTF-8 65cc81) evaluates
to precomposed field name `\u00e9` (U+00E9, UTF-8 c3a9). Compatible declarations
of both spellings are one evaluated field; conflicting concrete content at that
field is a CUE conflict. There is no extra collision classifier or id authority.

The Go Concepts map projects those evaluated field names exactly. Group concept
members and Requirement.Concept are string VALUES: CUE does not normalize them
with field labels. Resolve them by exact Go string equality against actual map
keys; never silently repair, normalize or rewrite references. Thus an NFD label
with an NFC reference resolves, while an NFD reference remains decomposed and
fails against the evaluated NFC key. Both remain shape-valid CUE; the existing
value validator/join supplies the reference refusal. For typed Go input those
pure functions compare the supplied map keys and string values exactly, without
CUE evaluation or a second normalization pass. A manually supplied Go map is not
proof of the authored source's evaluation or provenance. Keys/references are
opaque nonempty #Text data, not record-id families.
Keys stay open; the expectation metric/unit/citation vocabularies above are closed.
The citation's design-row selector is BE-D2 or BE-D9 in
`docs/design/bench-environments.md`. A citation supplies exactly one non-null
Card reference OR one design-row selector, never both. Each record has at least
one citation. A Card citation refers to the exact frozen CTR-13 Card bytes; its
ID is the approved Card.ID. Design citations do not invent an entry in that card
catalog. Native/caller reference resolution verifies those bytes and whether the
cited claim/row requires the key; neither pure validator judges prose or proves
that semantic justification. The data critic reviews it before a key is added.
No production keys or inventory contents are specified or created in this lane.

Each group binds one exact Card reference and one exact CheckerContract reference
to a nonempty set of concept keys. Its identity is the pair
`(Card.ID, CheckerContract.ID)`, not a new group enum or ID family. There is one
group per such pair; multiple groups may belong to one Card.ID. All groups for
the same Card.ID use the same complete Card reference. Every group key resolves
to exactly one ConceptRecord value by its map key. Repeated keys within a
group, repeated citation identities (Card.ID or design-row selector), and repeated
group pairs refuse in the value validator. A CUE field name is the sole authored
concept identity: no redundant key/id field, shadow list, numeric grant or enum.
Different concrete content for the same field unifies to a CUE conflict; `cue vet`
reports it. Compatible repeated declarations, including disjoint fields that
unify, are valid CUE and are not claimed to be duplicate-record errors. Compare
complete refs by ID, Path and SHA256 when identity matches;
conflicting Card refs refuse rather than selecting one. Cited design-only keys
may remain unused by a card until a reviewed data change maps them.

```cue
#MetricUnit: "symbols" | "entry_points" | "paths" | "links" | "repositories" | "services" | "sites" | "changes" | "expressions" | "cases" | "transitions" | "instructions" | "fixes" | "mutants" | "issues" | "pairs" | "commits" | "reads" | "escapes" | "tasks" | "deletions" | "labels" | "inputs" | "envelopes" | "bindings" | "outputs" | "questions"
#MinimumCitation: "BE-D2:False plan claims" | "BE-D2:Cross-repository dependencies" | "BE-D2:Fan-in and dispatch" | "BE-D2:Output fidelity and judgments"
#ExpectedMetric: "absent_symbols" | "unwired_entries" | "cross_file_paths" | "repository_links" | "linked_repositories" | "linked_services" | "fanin_symbols" | "dispatch_sites" | "hazardous_changes" | "safe_changes" | "module_expressions" | "missing_context" | "provider_transitions" | "stale_instructions" | "live_instructions" | "governing_links" | "public_fixes" | "collateral_mutants" | "nonlexical_issues" | "lexical_issues" | "conflicting_pairs" | "clean_pairs" | "history_commits" | "relevant_changes" | "boundary_sites" | "language_sites" | "data_reads" | "subprocess_reads" | "uncovered_changes" | "acquire_release_pairs" | "missing_release_fixes" | "escapes" | "bounded_sites" | "unbounded_sites" | "shutdown_cases" | "multisession_tasks" | "declared_deletions" | "undeclared_deletions" | "rule_labels" | "pinned_inputs" | "floating_inputs" | "interface_changes" | "valid_envelopes" | "invalid_envelopes" | "resolvable_bindings" | "ambiguous_bindings" | "output_cases" | "long_outputs" | "short_outputs" | "question_cases" | "route_questions" | "rank_questions" | "table_questions"
#MetricExpectation: close({
    metric!: #ExpectedMetric
    unit!: #MetricUnit
    minimum: *30 | (int & >=1 & <=9223372036854775807)
    lower_minimum?: #MinimumCitation
}) & (
    // Projection of the sole canonical relation above, not another policy.
    {metric!: ("absent_symbols" | "fanin_symbols"), unit!: "symbols"} |
    {metric!: ("unwired_entries"), unit!: "entry_points"} |
    {metric!: ("cross_file_paths"), unit!: "paths"} |
    {metric!: ("repository_links" | "governing_links"), unit!: "links"} |
    {metric!: ("linked_repositories"), unit!: "repositories"} |
    {metric!: ("linked_services"), unit!: "services"} |
    {metric!: ("dispatch_sites" | "boundary_sites" | "language_sites" | "bounded_sites" | "unbounded_sites"), unit!: "sites"} |
    {metric!: ("hazardous_changes" | "safe_changes" | "relevant_changes" | "uncovered_changes" | "interface_changes"), unit!: "changes"} |
    {metric!: ("module_expressions"), unit!: "expressions"} |
    {metric!: ("missing_context" | "shutdown_cases"), unit!: "cases"} |
    {metric!: ("provider_transitions"), unit!: "transitions"} |
    {metric!: ("stale_instructions" | "live_instructions"), unit!: "instructions"} |
    {metric!: ("public_fixes" | "missing_release_fixes"), unit!: "fixes"} |
    {metric!: ("collateral_mutants"), unit!: "mutants"} |
    {metric!: ("nonlexical_issues" | "lexical_issues"), unit!: "issues"} |
    {metric!: ("conflicting_pairs" | "clean_pairs" | "acquire_release_pairs"), unit!: "pairs"} |
    {metric!: ("history_commits"), unit!: "commits"} |
    {metric!: ("data_reads" | "subprocess_reads"), unit!: "reads"} |
    {metric!: ("escapes"), unit!: "escapes"} |
    {metric!: ("multisession_tasks"), unit!: "tasks"} |
    {metric!: ("declared_deletions" | "undeclared_deletions"), unit!: "deletions"} |
    {metric!: ("rule_labels"), unit!: "labels"} |
    {metric!: ("pinned_inputs" | "floating_inputs"), unit!: "inputs"} |
    {metric!: ("valid_envelopes" | "invalid_envelopes"), unit!: "envelopes"} |
    {metric!: ("resolvable_bindings" | "ambiguous_bindings"), unit!: "bindings"} |
    {metric!: ("output_cases" | "long_outputs" | "short_outputs"), unit!: "outputs"} |
    {metric!: ("question_cases" | "route_questions" | "rank_questions" | "table_questions"), unit!: "questions"} ) & (
    *{minimum: >=30, lower_minimum?: _|_} |
    {metric!: ("absent_symbols" | "unwired_entries"), minimum: >=1 & <30, lower_minimum!: "BE-D2:False plan claims"} |
    {metric!: ("linked_repositories" | "linked_services"), minimum: >=3 & <30, lower_minimum!: "BE-D2:Cross-repository dependencies"} |
    {metric!: ("fanin_symbols"), minimum: >=3 & <30, lower_minimum!: "BE-D2:Fan-in and dispatch"} |
    {metric!: ("long_outputs" | "short_outputs" | "route_questions" | "rank_questions" | "table_questions"), minimum: >=1 & <30, lower_minimum!: "BE-D2:Output fidelity and judgments"} ) & ({metric!: !="history_commits"} | {metric!: "history_commits", minimum: >=200})
#ConceptDesignRow: "BE-D2" | "BE-D9"
#ConceptCitation: close({card!: #Ref}) | close({design_row!: #ConceptDesignRow})
#ConceptRecord: close({
    required_by!: [#ConceptCitation, ...#ConceptCitation]
    expectations!: [#MetricExpectation, ...#MetricExpectation]
})
#ConceptGroup: close({
    card!: #Ref
    checker_contract!: #Ref
    concepts!: [#Text, ...#Text]
})
#ConceptInventory: close({concepts!: {[#Text]: #ConceptRecord, [string & !~"\\S"]: _|_}, groups!: [...#ConceptGroup]})
```

ValidateConceptInventory accepts CUE data or its JSON subset, embeds the same
schema, decodes concrete defaults, then uses ValidateConceptInventoryValue for
the canonical local shape/path/identity/reference rules without I/O. The value
validator also belongs in validate.go, returns only nil or *ValidationError and
never parses CUE or defaults zero-valued Go fields. It never mutates input. The
leaf calls this shared value validator, never copies its metric/unit/minimum
relation or adds a second validation table. The core join uses this same check.
The Go map is the smallest lossless projection of that one EVALUATED CUE struct,
not of its raw source spellings or declaration multiplicity, and is not a
parallel inventory. Empty non-nil Concepts map and Groups slice are valid shape,
never proof of closure; nil Concepts refuses ValidationShape at /concepts in typed
input, just as null source concepts does. Every returned map and slice is non-nil.
A present value cannot have empty Expectations. No map iteration order is semantic:
traverse evaluated concept/map keys in ascending UTF-8 byte order. No ENV layer
adds Unicode normalization. Lists retain authored order and source indices. JSON map
members serialize in ascending key-byte order, yielding identical output under
CUE field reorderings or Go map insertion permutations. Do not materialize an
ordered shadow roster. On any failure return
`ConceptInventory{Concepts: map[string]ConceptRecord{}, Groups: []ConceptGroup{}}`
and `*ValidationError`, whose exact JSON result is `{"concepts":{},"groups":[]}`.
Use the existing ValidationReason constants and RFC6901/shape-first/first-error
law below. `<K>`, `<L>` and `<key>` in inventory pointers mean that exact
evaluated map key escaped as one RFC6901 segment: replace ~ with ~0, then /
with ~1, retaining the evaluated key's spaces and Unicode bytes. They are
parameter substitutions, not literal labels or
indices. For example key `a/~ λ` has base pointer `/concepts/a~1~0 λ`; its missing
expectations reports `/concepts/a~1~0 λ/expectations`. Error() exposes no key or
rejected value; the typed Path carries the escaped pointer. Invalid empty or
whitespace-only map labels refuse ValidationShape at their escaped member
pointer. Missing a citation arm reports its citation pointer; two arms report
that same pointer; a null card arm reports its `/card` member; any other invalid
single arm reports its deepest field. No raw key or rejected value enters
Error(). Missing/null/empty/untyped expectations refuse ValidationShape at
`/concepts/<key>/expectations`; missing/invalid members report the deepest
`/concepts/<key>/expectations/j/{metric,unit,minimum,lower_minimum}`. For pointer
attribution, first validate required fields/types/individual enums
and integer bounds, then the canonical metric/unit/minimum relation, then the
below30 citation binding. Only independently checkable candidates in the same
phase use pointer/reason ordering. Thus cross_file_paths29 with a syntactically
known but inapplicable citation reports /minimum before testing that binding.
CUE diagnostic order is never the public pointer oracle. Missing required
minimum citation reports `/lower_minimum`; forbidden or wrong citation reports
that same member. Unknown metric/unit combinations report `/unit` after both
enums are individually valid. Duplicate metric expectations refuse
ValidationIdentity at the later `/concepts/<key>/expectations/j/metric`. Local semantic
errors use the same pointer/reason ordering as all other inventory candidates.
The closed record shapes forbid authored
pass/status fields here too. No hashing or fingerprint canonicalization is added;
BEN-9 remains the sole owner of those operations.

### Read-only Card.ID and group join

The caller first resolves the frozen Card refs, then calls the existing CTR-13
`capability.CheckCardClosure(shipped []string, cards []capability.Card) ([]string, error)`
in `internal/capability`, using actual registry surface keys and approved cards.
Its existing missing-card/orphan-surface/duplicate-card/conflicting-claim errors
stop the call; ENV neither replaces it with an ID-only closure test nor adds a
card API. Pass all exact cards' IDs, not titles, Surfaces or a filtered subset, as
cardIDs to ValidateEnvironmentConceptJoin. This is an explicit caller obligation,
not something a pure schema signature can attest. Core schema tests use supplied
ID values; the real CheckCardClosure call and byte-resolution proof remain with
the caller's implementation owner, not a new schema import or duplicate checker.

The pure join revalidates records and inventory by their local rules before
joining; prefix their errors with `/records` or `/inventory`. Validate cardIDs as
nonempty #Text members with no duplicates (empty list is allowed); invalid member
is ValidationShape at `/card_ids/i`, later duplicate ValidationIdentity there.
Group Card.IDs must be exactly the cardIDs set: no orphan group, and every card
has at least one group. A Card citation must also resolve to that set and match
the complete Card ref used by that card's groups. No requirement or coverage
claim may name a capability outside it. Each requirement's Card and
CheckerContract must exactly match a group, and its Concept must belong to that
group and the inventory. Each coverage claim's referenced requirements must
contain every `(CheckerContract.ID, concept key)` in all groups for its Card.ID
in that claim's exact stratum, once each. This checks mapping closure, never
witness counts, native grouping semantics or admission. No mapping is inferred
from capability names or the BE-D2 table. No inputs are mutated or repaired.

Local validation runs records, then inventory, then cardIDs, stopping at the
first failing stage. For the inventory's semantic diagnostics, group Card refs
are compared to their first source-order occurrence, then citations to that
card's group ref. A repeated group pair reports ValidationIdentity at the later
group's `/checker_contract/id`, a repeated
group concept at that list member, and a repeated citation at its selected
`/card/id` or `/design_row`. Unknown group concept keys report
ValidationReference at that list member. Card ref conflicts report
ValidationReference at each conflicting `/path` or `/sha256` member, never echoing
values. Missing groups do not fabricate a reference to compare against.

After local validation, collect all join errors and select the lowest pointer
then reason using the same law below. Missing card mapping reports
ValidationReference at `/card_ids/i`; an orphan group/citation at
`/inventory/groups/i/card/id` or `/inventory/concepts/<key>/required_by/j/card/id`.
Conflicting Card/CheckerContract fields report ValidationReference at the exact
changed id/path/sha256 member. Unknown requirement capability, claim capability
or concept reports ValidationReference at `/records/requirements/i/capability`,
`/records/coverage/i/capability` or `/records/requirements/i/concept` respectively.
A missing requirement group reports ValidationReference at
`/records/requirements/i/checker_contract/id`. A concept outside a resolved group
reports ValidationReference at `/records/requirements/i/concept`. Missing a
required group/concept in a claim reports ValidationReference at
`/records/coverage/i/requirements`; a later repeated group/concept reports
ValidationIdentity at `/records/coverage/i/requirements/j/id`. Suppress cascades
from unavailable targets. Do not check a claim's group/concept completeness or
repetition when one of its referenced requirements failed the join. Nil error
establishes only this declared join.

The one inventory file is selected by its lane exact-file path grant, enforced
by pathcheck. The proposed `calibration/environments/concepts.cue` path is data,
not a write grant from this contract. No second production inventory is merged or
discovered. The data lane's pathcheck must refuse a new/changed inventory file
outside its one exact granted path, including a second inventory path; it need
not inspect CUE literals or concept keys. Within that file schema validation and
`cue vet` enforce the keyed struct and conflicting-content law. JSON/Go maps
cannot retain duplicate source declarations; that CUE source proof belongs before
decode. These checks need no recordchecker representation, discovery, effective
numeric policy, grants or RecordOwnership result. Other authored ENV records
retain U1 independently. Adding, removing or remapping a concept is a critic-reviewed
data change, not a contract revision. No inventory contents or data path grant is
supplied here, and the deferred REC-6 proposal remains its existing owner's work.

## Exact CUE projection and fixtures

`schema/benchenv/schema.cue` has package `benchenv`. The following definitions,
including closed structs and required fields, are the normative projection of the
Go declarations. `#Text` excludes empty/whitespace-only values. Go additionally
checks paths, URL policy and local references/identities. Other authored ENV
records retain shared record-id checks; concept keys use only the keyed CUE
struct/schema and the separate exact-file path grant enforced by pathcheck.
Shape validation is not evidence validation. Grounds: D1/D2/D9/D10 and the
existing CUE pattern at `schema/bench/schema_test.go:15-42,44-65`.

```cue
package benchenv

#Text: string & =~"\\S"
#Digest: string & =~"^[0-9a-f]{64}$"
#Commit: string & =~"^[0-9a-f]{40}$"
#Tier: "public" | "constructed" | "smoke"
#Split: "dev" | "held-out"
#Axis: "lines" | "files" | "repositories" | "history"
#ScaleClass: "S" | "M" | "L" | "XL"
#Prediction: "effect" | "null"
#Polarity: "positive" | "negative"
#ExclusionKind: "vendored" | "generated" | "test" | "build"
#Ref: close({id!: #Text, path!: #Text, sha256!: #Digest})
#RepositoryPin: close({id!: #Text, url!: string, commit!: #Commit})
#Pin: close({name!: #Text, version!: #Text, sha256!: #Digest})
#Int64: int & >=0 & <=9223372036854775807
#Int32: int & >=0 & <=2147483647
#ExcludedCount: close({kind!: #ExclusionKind, lines!: #Int64, files!: #Int64})
#Scale: close({
    lines!: null | #Int64
    files!: null | #Int64
    repositories!: null | #Int64
    history!: null | #Int64
    excluded!: [...#ExcludedCount]
    receipt!: #Ref
})
#ScaleClaim: close({axis!: #Axis, class!: #ScaleClass})
#EnvironmentHead: close({
    id!: #Text
    tier!: #Tier
    repositories!: [#RepositoryPin, ...#RepositoryPin]
    subjects!: [...#Ref]
    licence!: #Ref
    toolchains!: [#Pin, ...#Pin]
    images!: [...#Pin]
    source_policy!: #Ref
    scale!: #Scale
    split!: #Split
    prior_reads!: [...#Ref]
    oracles!: [#Ref, ...#Ref]
    receipts!: [#Ref, ...#Ref]
})
#Stratum: close({
    subtype!: "" | #Text
    language!: #Text
    scale!: [#ScaleClaim, ...#ScaleClaim]
    prediction!: #Prediction
})
#Requirement: close({
    id!: #Text
    capability!: #Text
    card!: #Ref
    checker_contract!: #Ref
    concept!: #Text
    stratum!: #Stratum
    positive_minimum!: #Int32 & >=30
    negative_minimum!: #Int32 & >=30
    checker!: #Pin
})
#Anchor: close({
    repository!: #RepositoryPin
    path!: #Text
    start_line!: #Int32 & >=1
    end_line!: #Int32 & >=start_line
})
#Witness: close({
    id!: #Text
    environment!: #Ref
    requirement!: #Ref
    anchor!: #Anchor
    polarity!: #Polarity
    match_id!: #Text
    origin!: #Ref
    checker!: #Pin
    receipt!: #Ref
})
#CoverageClaim: close({
    id!: #Text
    capability!: #Text
    environment!: #Ref
    stratum!: #Stratum
    requirements!: [#Ref, ...#Ref]
})
#EnvironmentRecords: close({
    environments!: [...#EnvironmentHead]
    requirements!: [...#Requirement]
    witnesses!: [...#Witness]
    coverage!: [...#CoverageClaim]
})
```

Empty lists are permitted only where shown and never mean passing coverage. No
default writes a status or repairs missing evidence. JSON keys and CUE labels are
identical. JSON numeric counts must be integral and fit Go's declared type.
All authored record shapes reject undeclared fields, including `status`, `pass`,
`admitted`, `adequate`, `valid` and `fresh` at every record level.

Fixtures reserved for ENV-1 are exactly:

* `schema/benchenv/testdata/valid.cue`: concrete `records: #EnvironmentRecords &
  {...}` with two independent public dev environments, one complete claim per
  environment/stratum, 30 matched positive/negative anchors each, and exact refs.
  Also named `unknownCounts` (null measurements, structurally valid) and
  `constructed` (empty URL, manifest-backed commit, native `images: []`).
* `schema/benchenv/testdata/invalid.cue`: fully commented negative examples, with
  one example per named schema mutation in the test table below.

No third fixture format or live invalid CUE package. Tests remove comments or
mutate the valid decoded value in memory. This follows `schema/AGENTS.md:3-11`.
Opaque referenced oracle artefacts in the valid shape fixture are synthetic
shape data, not proof of actual independent native qualification.

## Schema validator and derived checker boundary

```go
// schema/benchenv/validate.go; source is CUE data or its JSON subset.
func ValidateEnvironmentRecords(source []byte) (EnvironmentRecords, error)

type ValidationError struct {
    Path string `json:"path"`
    Reason ValidationReason `json:"reason"`
}
func (e *ValidationError) Error() string
type ValidationReason string
const (
    ValidationShape ValidationReason = "shape"
    ValidationPath ValidationReason = "path"
    ValidationReference ValidationReason = "reference"
    ValidationIdentity ValidationReason = "identity"
)
```

Return an initialized empty `EnvironmentRecords` with a typed error on failure,
including failures before decoding. Its exact JSON is
`{"environments":[],"requirements":[],"witnesses":[],"coverage":[]}`.
Never return a partially decoded record or a zero Go struct. No file, clock,
git, network or model access. Embed the schema in `validate.go`, not an unowned
extra `embed.go`. Compilation/shape errors use `ValidationShape`; semantic path,
reference and identity errors use the other named reasons. Choose the first
failure using the exact path/selection law in the negative-fixture table below,
with no raw values.
The validator validates declaration shape and in-bundle ID/stratum/pin consistency,
never claims external receipt digests were checked or a native checker executed.
The caller owns external reference resolution. Other authored ENV record-id
ownership remains blocked on REC-1, not replaced by local shape success. Concept
struct validation and exact-file pathcheck are independent under W18-1/W18-2.

## Referenced evidence input shapes

```go
type PreflightInput struct {
    PairID string `json:"pair_id"`
    Arm string `json:"arm"`
    Environment Ref `json:"environment"`
    ProfileSHA256 string `json:"profile_sha256"`
    Receipt Ref `json:"receipt"`
}
type AgentOutcome struct {
    PairID string `json:"pair_id"`
    Arm string `json:"arm"`
    CellKey string `json:"cell_key"`
    RunID string `json:"run_id"`
    Raw Ref `json:"raw"`
}
type FreshReadingInput struct {
    ID string `json:"id"`
    RunID string `json:"run_id"`
    MeasuredAt string `json:"measured_at"`
    Environment Ref `json:"environment"`
    Matrix Ref `json:"matrix"`
    Fingerprint Ref `json:"fingerprint"`
    Raw Ref `json:"raw"`
    Preflights []PreflightInput `json:"preflights"`
    Outcomes []AgentOutcome `json:"outcomes"`
}
```

Their proposed CUE projection is exactly closed structs with the same
required labels: `#PreflightInput` has `pair_id/arm: #Text`,
`environment/receipt: #Ref`, `profile_sha256: #Digest`; `#AgentOutcome` has
`pair_id/arm/run_id: #Text`, `cell_key: #Digest`, `raw: #Ref`;
`#FreshReadingInput` has `id/run_id: #Text`, `measured_at` a concrete RFC3339 UTC
timestamp, `environment/matrix/fingerprint/raw: #Ref`,
`preflights: [#PreflightInput, ...#PreflightInput]`,
`outcomes: [#AgentOutcome, ...#AgentOutcome]`. This describes bytes only.

## Fleet data declarations and pure manifest API

```go
// Proposed schema/benchenv/validate.go, except the named leaf signature.
type FileMode string
const (
    FileRegular FileMode = "100644"
    FileExecutable FileMode = "100755"
)
type FileClass string
const (
    FileProduct FileClass = "product"
    FileVendored FileClass = "vendored"
    FileGenerated FileClass = "generated"
    FileTest FileClass = "test"
    FileBuild FileClass = "build"
)
type FleetFile struct {
    Path string `json:"path"`
    ContentID string `json:"content_id"`
    SourceSHA256 string `json:"source_sha256"`
    SHA256 string `json:"sha256"`
    Mode FileMode `json:"mode"`
    Class FileClass `json:"class"`
    Seeded bool `json:"seeded"`
}
type FleetCommit struct {
    ID string `json:"id"`
    Parents []string `json:"parents"`
    Files []FleetFile `json:"files"`
}
type FleetRepository struct {
    ID string `json:"id"`
    Head string `json:"head"`
    Commits []FleetCommit `json:"commits"`
}
type FleetSize struct {
    Lines int64 `json:"lines"`
    Files int64 `json:"files"`
    Repositories int64 `json:"repositories"`
    History int64 `json:"history"`
}
type GeneratorVersion string
const (
    GeneratorVersionV1 GeneratorVersion = "env-fleet/v1"
)
type FleetManifest struct {
    Seed uint64 `json:"seed"`
    GeneratorVersion GeneratorVersion `json:"generator_version"`
    Template string `json:"template"`
    Size FleetSize `json:"size"`
    Repositories []FleetRepository `json:"repositories"`
    Truth []FleetFile `json:"truth"`
}
type FleetLimits struct {
    MaxRepositories int64 `json:"max_repositories"`
    MaxCommits int64 `json:"max_commits"`
    MaxFiles int64 `json:"max_files"`
    MaxBytes int64 `json:"max_bytes"`
}
type FleetCommitResult struct {
    ID string `json:"id"`
    Commit string `json:"commit"`
    Parents []string `json:"parents"`
}
type FleetRepositoryResult struct {
    ID string `json:"id"`
    Head string `json:"head"`
    Commits []FleetCommitResult `json:"commits"`
}
type FleetResult struct {
    Repositories []FleetRepositoryResult `json:"repositories"`
    Truth []Ref `json:"truth"`
    Size FleetSize `json:"size"`
    Excluded []ExcludedCount `json:"excluded"`
}
type FleetReason string
const (
    FleetShape FleetReason = "shape"
    FleetPath FleetReason = "path"
    FleetDuplicate FleetReason = "duplicate"
    FleetParent FleetReason = "parent"
    FleetCycle FleetReason = "cycle"
    FleetDigest FleetReason = "digest"
    FleetSizeMismatch FleetReason = "size_mismatch"
    FleetTarget FleetReason = "target"
    FleetIO FleetReason = "io"
    FleetCancelled FleetReason = "cancelled"
)
type FleetError struct {
    Path string `json:"path"`
    Reason FleetReason `json:"reason"`
}
func (e *FleetError) Error() string
type FleetLimit string
const (
    LimitRepositories FleetLimit = "repositories"
    LimitCommits FleetLimit = "commits"
    LimitFiles FleetLimit = "files"
    LimitBytes FleetLimit = "bytes"
)
type FleetLimitError struct {
    Path string `json:"path"`
    Kind FleetLimit `json:"kind"`
    Limit int64 `json:"limit"`
    Observed int64 `json:"observed"`
}
func (e *FleetLimitError) Error() string
func ValidateFleetManifest(source []byte) (FleetManifest, error)

```

`ValidateFleetManifest` is pure shape/paths/identity/DAG validation; missing raw
sources is NOT a shape error. It returns initialized empty repositories/truth
on failure. Its exact failure JSON is
`{"seed":0,"generator_version":"","template":"","size":{"lines":0,"files":0,"repositories":0,"history":0},"repositories":[],"truth":[]}`.

No checker, ingest, store, client, model, network or compiler invocation is inside
this pure API.

Repository/commit/content IDs match `[a-z][a-z0-9-]*`. Repositories sort by ID,
commit snapshots use a lexically tie-broken topological order, files sort by path,
and truth sorts by path. Input permutation does not alter output. Parent order
is authored and retained, because it affects git identity. No duplicate parent,
missing parent, cycle, unreachable commit or absent Head is permitted. The Head
must reach every declared commit. Empty root snapshots are allowed.

### Fleet CUE projection

```cue
#FleetID: string & =~"^[a-z][a-z0-9-]*$"
#FleetFile: close({
    path!: #Text, content_id!: #FleetID
    source_sha256!: #Digest, sha256!: #Digest
    mode!: "100644" | "100755"
    class!: "product" | "vendored" | "generated" | "test" | "build"
    seeded!: bool
})
#FleetCommit: close({id!: #FleetID, parents!: [...#FleetID], files!: [...#FleetFile]})
#FleetRepository: close({id!: #FleetID, head!: #FleetID, commits!: [#FleetCommit, ...#FleetCommit]})
#FleetSize: close({lines!: #Int64, files!: #Int64, repositories!: #Int64, history!: #Int64})
#GeneratorVersion: "env-fleet/v1"
#FleetManifest: close({
    seed!: int & >=0 & <=18446744073709551615
    generator_version!: #GeneratorVersion, template!: #Text, size!: #FleetSize
    repositories!: [#FleetRepository, ...#FleetRepository]
    truth!: [#FleetFile, ...#FleetFile]
})
#FleetLimits: close({
    max_repositories!: #Int64 & >0, max_commits!: #Int64 & >0
    max_files!: #Int64 & >0, max_bytes!: #Int64 & >0
})
```

All `int64` CUE members also have upper bound 9223372036854775807. No authored
result schema accepts an adequacy/pass flag. Go validates paths and cross-object
semantics; CUE compilation alone is not those checks.

## Request, diagnostic, sample and error declarations

```go
type EnvironmentMode string
const (
    ModeUnselected EnvironmentMode = "" // Output only when input mode is unknown.
    ModeFixtures EnvironmentMode = "fixtures"
    ModePortfolio EnvironmentMode = "portfolio"
    ModeBaseline EnvironmentMode = "baseline"
    ModeAudit EnvironmentMode = "audit"
    ModeSeed EnvironmentMode = "seed"
    ModeCandidate EnvironmentMode = "candidate"
    ModeSeal EnvironmentMode = "seal"
    ModeReading EnvironmentMode = "reading"
    ModeSnapshots EnvironmentMode = "snapshots"
    ModeModelRerun EnvironmentMode = "model-rerun"
    ModeLandedReplay EnvironmentMode = "landed-replay"
)
type FixturesInput struct {
    Records EnvironmentRecords `json:"records"`
    Catalog Ref `json:"catalog"`
    Evidence []Ref `json:"evidence"`
}
type PortfolioInput struct {
    Records EnvironmentRecords `json:"records"`
    Catalog Ref `json:"catalog"`
    Subjects []Ref `json:"subjects"`
    Evidence []Ref `json:"evidence"`
}
type BaselineInput struct {
    Matrix Ref `json:"matrix"`
    Preflights []PreflightInput `json:"preflights"`
    Outcomes []AgentOutcome `json:"outcomes"`
}
type AuditInput struct {
    Sources []Ref `json:"sources"`
    Runs []Ref `json:"runs"`
    Rulings []Ref `json:"rulings"`
}
type SeedInput struct {
    Manifest FleetManifest `json:"manifest"`
    Sources []Ref `json:"sources"`
    Limits FleetLimits `json:"limits"`
    Requirements []Requirement `json:"requirements"`
    Catalog Ref `json:"catalog"`
}
type CandidateInput struct {
    SubjectInput Ref `json:"subject_input"`
    Records EnvironmentRecords `json:"records"`
    Catalog Ref `json:"catalog"`
    Evidence []Ref `json:"evidence"`
}
type SealInput struct {
    Admission Ref `json:"admission"`
    Environment Ref `json:"environment"`
    PriorReads []Ref `json:"prior_reads"`
    Topology Ref `json:"topology"`
    OracleKeys []Ref `json:"oracle_keys"`
}
type ReadingInput struct {
    Reading FreshReadingInput `json:"reading"`
    ClaimTime string `json:"claim_time"`
    CurrentFingerprint Ref `json:"current_fingerprint"`
    ExistingRuns []string `json:"existing_runs"`
    ExistingCellKeys []string `json:"existing_cell_keys"`
}
type SnapshotInput struct {
    ProductCommit string `json:"product_commit"`
    BinarySHA256 string `json:"binary_sha256"`
    Commits []string `json:"commits"`
    Service string `json:"service"`
    WallCapMS int64 `json:"wall_cap_ms"`
}
type ModelRerunInput struct {
    ReadingID string `json:"reading_id"`
    Preregistration Ref `json:"preregistration"`
    Approval Ref `json:"approval"`
    Spend Ref `json:"spend"`
    CellNamespace string `json:"cell_namespace"`
    ExistingNamespaces []string `json:"existing_namespaces"`
    MaxUSDMicros int64 `json:"max_usd_micros"`
    WallCapMS int64 `json:"wall_cap_ms"`
}
type LandedReplayInput struct {
    Input Ref `json:"input"`
    WallCapMS int64 `json:"wall_cap_ms"`
}
type EnvironmentRequest struct {
    Mode EnvironmentMode `json:"mode"`
    Fixtures *FixturesInput `json:"fixtures,omitempty"`
    Portfolio *PortfolioInput `json:"portfolio,omitempty"`
    Baseline *BaselineInput `json:"baseline,omitempty"`
    Audit *AuditInput `json:"audit,omitempty"`
    Seed *SeedInput `json:"seed,omitempty"`
    Candidate *CandidateInput `json:"candidate,omitempty"`
    Seal *SealInput `json:"seal,omitempty"`
    Reading *ReadingInput `json:"reading,omitempty"`
    Snapshots *SnapshotInput `json:"snapshots,omitempty"`
    ModelRerun *ModelRerunInput `json:"model_rerun,omitempty"`
    LandedReplay *LandedReplayInput `json:"landed_replay,omitempty"`
}
type ModeState string
const (
    ModePass ModeState = "pass"
    ModeRefused ModeState = "refused"
    ModeUnknown ModeState = "unknown"
)
type ModeReason string
const (
    ModeShape ModeReason = "shape"
    ModeUnknownName ModeReason = "unknown_mode"
    ModeDuplicate ModeReason = "duplicate_mode"
    ModeUnavailable ModeReason = "unavailable"
    ModeDependency ModeReason = "dependency"
    ModeCancelled ModeReason = "cancelled"
    ModeLimit ModeReason = "limit"
    ModeIO ModeReason = "io"
)
type ModeError struct {
    Path string `json:"path"`
    Reason ModeReason `json:"reason"`
    cause error // Private, never JSON or rendered raw.
}
func NewModeError(path string, reason ModeReason, cause error) *ModeError
func (e *ModeError) Error() string
func (e *ModeError) Unwrap() error
type ModeDiagnostic struct {
    Path string `json:"path"`
    Reason ModeReason `json:"reason"`
    Evidence []Ref `json:"evidence"`
}
type SnapshotCase string
const (
    SnapshotCold SnapshotCase = "cold"
    SnapshotWarm SnapshotCase = "warm"
    SnapshotCacheFrom SnapshotCase = "cache-from"
)
type SnapshotState string
const (
    SnapshotPassed SnapshotState = "passed"
    SnapshotFailed SnapshotState = "failed"
    SnapshotDegraded SnapshotState = "degraded"
    SnapshotTimeout SnapshotState = "timeout"
    SnapshotCancelled SnapshotState = "cancelled"
)
type SnapshotSample struct {
    Commit string `json:"commit"`
    Case SnapshotCase `json:"case"`
    State SnapshotState `json:"state"`
    WallMS *int64 `json:"wall_ms"`
    Evidence []Ref `json:"evidence"`
}

func ValidateEnvironmentRequest(source []byte) (EnvironmentRequest, error)
```

### Closed request CUE projection and source-only validation

The exact additional closed input CUE definitions are below. Calendar validity,
canonical paths, reference consistency and cross-list identity are Go semantic
checks, not attested by successful CUE compilation.

```cue
#UTC: string & =~"^[0-9]{4}-[0-9]{2}-[0-9]{2}T[0-9]{2}:[0-9]{2}:[0-9]{2}Z$"
#PreflightInput: close({
    pair_id!: #Text, arm!: #Text, environment!: #Ref
    profile_sha256!: #Digest, receipt!: #Ref
})
#AgentOutcome: close({
    pair_id!: #Text, arm!: #Text, cell_key!: #Digest, run_id!: #Text, raw!: #Ref
})
#FreshReadingInput: close({
    id!: #Text, run_id!: #Text, measured_at!: #UTC
    environment!: #Ref, matrix!: #Ref, fingerprint!: #Ref, raw!: #Ref
    preflights!: [#PreflightInput, ...#PreflightInput]
    outcomes!: [#AgentOutcome, ...#AgentOutcome]
})
#FixturesInput: close({records!: #EnvironmentRecords, catalog!: #Ref, evidence!: [...#Ref]})
#PortfolioInput: close({
    records!: #EnvironmentRecords, catalog!: #Ref, subjects!: [...#Ref], evidence!: [...#Ref]
})
#BaselineInput: close({
    matrix!: #Ref, preflights!: [#PreflightInput, #PreflightInput, ...#PreflightInput]
    outcomes!: [...#AgentOutcome]
})
#AuditInput: close({
    sources!: [#Ref, ...#Ref], runs!: [#Ref, ...#Ref], rulings!: [#Ref, ...#Ref]
})
#SeedInput: close({
    manifest!: #FleetManifest, sources!: [#Ref, ...#Ref], limits!: #FleetLimits
    requirements!: [...#Requirement], catalog!: #Ref
})
#CandidateInput: close({
    subject_input!: #Ref, records!: #EnvironmentRecords, catalog!: #Ref, evidence!: [...#Ref]
})
#SealInput: close({
    admission!: #Ref, environment!: #Ref, prior_reads!: [...#Ref]
    topology!: #Ref, oracle_keys!: [...#Ref]
})
#ReadingInput: close({
    reading!: #FreshReadingInput, claim_time!: #UTC, current_fingerprint!: #Ref
    existing_runs!: [...#Text], existing_cell_keys!: [...#Digest]
})
#SnapshotInput: close({
    product_commit!: #Commit, binary_sha256!: #Digest
    commits!: [#Commit, #Commit, #Commit, #Commit, #Commit, #Commit, #Commit,
        #Commit, #Commit, #Commit, #Commit, #Commit, #Commit, #Commit,
        #Commit, #Commit, #Commit, #Commit, #Commit, #Commit, #Commit]
    service!: #Text, wall_cap_ms!: #Int64 & >0
})
#ModelRerunInput: close({
    reading_id!: #Text, preregistration!: #Ref, approval!: #Ref, spend!: #Ref
    cell_namespace!: #Text, existing_namespaces!: [...#Text]
    max_usd_micros!: #Int64 & >0, wall_cap_ms!: #Int64 & >0
})
#LandedReplayInput: close({input!: #Ref, wall_cap_ms!: #Int64 & >0})
#EnvironmentRequest:
    close({mode!: "fixtures", fixtures!: #FixturesInput}) |
    close({mode!: "portfolio", portfolio!: #PortfolioInput}) |
    close({mode!: "baseline", baseline!: #BaselineInput}) |
    close({mode!: "audit", audit!: #AuditInput}) |
    close({mode!: "seed", seed!: #SeedInput}) |
    close({mode!: "candidate", candidate!: #CandidateInput}) |
    close({mode!: "seal", seal!: #SealInput}) |
    close({mode!: "reading", reading!: #ReadingInput}) |
    close({mode!: "snapshots", snapshots!: #SnapshotInput}) |
    close({mode!: "model-rerun", model_rerun!: #ModelRerunInput}) |
    close({mode!: "landed-replay", landed_replay!: #LandedReplayInput})
```

The request is a closed tagged union. Exactly the selected non-null payload is
present; all other payload keys are absent, not null. `model-rerun` selects
`model_rerun`; `landed-replay` selects `landed_replay`. CUE is the union of
eleven closed structs, each containing literal
`mode!: "<mode>"` and required `<payload>!: #<Input>`. Every `#<Input>` is a
closed struct of exactly the required labels above, recursively applying existing
definitions. Lists are required, possibly `[]` except: snapshot commits exactly
21 unique full commits (seed plus 20 measured), baseline preflights at least two,
and audit sources/runs/rulings and seed sources each nonempty. Refs use #Ref,
digests #Digest, commits #Commit, opaque identifiers #Text, timestamps RFC3339
UTC with no fractional seconds, integer caps positive int64. Candidate subject_input
resolves exactly the amended CTR-13 SubjectInput, including its existing TaskFamily
type, rather than an ENV duplicate. Generic source bytes are valid CUE or JSON,
never arbitrary
`any` payloads. Add no authored outcome status to an input.

`ValidateEnvironmentRequest` returns typed `*ModeError` and a zero request on
failure; this type has no returned slices. It checks ONLY ENV shape and local
consistency. It never attempts shared checker/native/spend interfaces or asserts
that evidence resolves. Thus a baseline request can be shape-valid while WPF
admission remains unavailable. Envelope definitions are ENV-owned now, rather
than waiting for those consumers. `PreflightInput`/`AgentOutcome`/`FreshReadingInput`
are included in this union's CUE definitions with the earlier exact labels.

### Core NewModeError constructor law

F3 / W3-4: `NewModeError(path string, reason ModeReason, cause error) *ModeError`
is the sole cause-bearing construction API. It returns a fresh pointer, stores the
original cause without wrapping/copying, and `Unwrap()` returns that exact error
(or nil). This is present adapter use, with no context/I/O or future abstraction.
Valid reasons are the declared ModeReason constants; an unknown reason normalizes
to ModeShape. Path is a canonical RFC6901 pointer (empty root or `/`-prefixed,
with only `~0`/`~1` escapes for literal tilde/slash); preserve it exactly, including
escaped unknown field names. A noncanonical pointer normalizes to the empty
pointer. This reuses the validator's pointer law, without a second field-name
vocabulary. Paths locate structural fields/indices, never rejected values or
CLI text. Constructor stores the normalized Path/Reason in exported fields.
`Error()` is exactly `environment: <reason>` with no newline, using shape for an
unknown reason even on a manually built error. It renders neither Path nor cause,
so field names cannot inject text into diagnostics. JSON exports only Path and
Reason, never cause. Adapters use the constructor and existing RFC6901 pointer
builder; no raw cause text or rejected values enter JSON/Error.

For a nested leaf refusal, the wrapper's Path prefixes the original validator
pointer with `/fixtures/records`, `/portfolio/records` or `/candidate/records`;
its Reason is ModeDependency. The original typed ValidationError and its Path
and Reason survive unchanged through errors.As and pointer identity. Diagnostics
hold the outer reason and only independently digest-bound leaf artifacts in
Evidence, never raw cause text. `%w` may wrap ModeError further but cannot replace
the constructor or lose either errors.As target.

## Concrete schema fixtures and mutation laws

### ENV-1 fixture bytes and first-error law

`valid.cue`'s named records value is the following deterministic construction.
Tests extract that value before passing its JSON subset to the validator, not
its enclosing `records:` fixture label. Every omitted optional-looking field in
this recipe MUST still be present using its declared empty list, not omitted.
Numbers and strings here are literal, not samples chosen after a failure.

* Two heads in order e-a/e-b, public/dev. Repository IDs r-a/r-b, URLs
  `https://example.invalid/fixtures/a` and `/fixtures/b` on the same origin;
  commits are 40 `a` and 40 `b` respectively. These are synthetic lexical public
  URL fixtures, never proof these hosts/repositories exist or are public.
* `R(id)` means Ref{id:id,path:`fixtures/<id>.json`,sha256:64 `0` characters}.
  Licence R(lic-a/b), source policy R(policy-a/b), scale receipt R(scale-a/b),
  subjects [R(subject-a/b)], oracles [R(oracle-a/b)], receipts [R(receipt-a/b)].
  Toolchains [{name:"go",version:"1.0.0",sha256:64 `1`}], images/prior_reads [].
  Scale 5000/50/1/200, excluded ordered vendored/generated/test/build, all 0/0.
* One requirement q-flow: capability copied from an explicitly injected test Card.ID `opaque-fixture-card`
  (synthetic only, no production card ID allocation), Card R(opaque-fixture-card),
  CheckerContract R(native-flow-contract), concept copied from an injected
  well-shaped concept struct field name (synthetic test input, not a production key), language
  `go`, scale [{axis:"lines",class:"S"}], prediction `effect`, subtype "", both minima 30,
  checker {name:"native-flow",version:"1",sha256:64 `2`}.
* For each head a/b and i=001..030, append negative THEN positive witness, IDs
  `w-<head>-<i>-n/p`. Environment R(e-a/b), requirement R(q-flow), match_id
  `m-<head>-<i>`, origin R(origin-<head>-<i>-n/p), receipt R(raw-<head>-<i>-n/p),
  checker as q-flow. Anchor repository equals that head's pin, path
  `src/<i>-n/p.go`, lines 1..3. There are exactly 120 witnesses. These origin
  labels do not certify independence: no native/oracle proof is fabricated.
* Claims c-a/c-b have that same injected Card.ID, environment R(e-a/b), the exact
  q-flow stratum, requirements [R(q-flow)]. All external 64-zero digests are
  shape fixtures. The pure validator neither verifies nor repairs them.
* `unknownCounts` changes the four scale numbers to null in both heads.
  `constructed` changes tier to constructed and URLs to "", retains other
  fields including images:[], with manifest reference in receipts. Both decode.
  Known zero uses numeric 0, not null. Neither fixture is an admission PASS.

Fields Path in typed errors use RFC6901 JSON pointers: root is empty string,
indices decimal with no leading zeros, `~` -> `~0`, `/` -> `~1`. No CUE source
coordinates or raw input bytes appear in Error(). Syntax/non-concrete top-level
failure reports root. Semantic validation never runs on an invalid shape.
Collect shape errors at their deepest identifiable field (missing parent reports
that parent only); sort pointers by UTF-8 bytes, then serialized reason text.
After shape success, collect all semantic errors and use the same order. For a
missing reference, do not cascade errors from its unavailable target. First
occurrence defines a local ID/set key, subsequent occurrence is the duplicate.
Arrays retain input indices for pointers; normalization must not move diagnostics.
These representations follow D9's determinism/refusal rules; their exact grammar
is the recommended ENV-owned choice rather than a native CUE error contract.

Every row below starts from the fixture above, changing ONLY the stated field.
`P` in a parameterized row is exactly the declared field's pointer obtained by
walking the matching instance; this binds every member of that finite schema,
not a choice among reason classes. Each subtest asserts errors.As to
*ValidationError, exact Reason AND Path, exact initialized failure JSON, and no
mutated value in the diagnostic.

| Test / mutation | Exact ValidationReason | Exact Path |
|---|---|---|
| NegativeShape: malformed source or non-concrete root | shape | empty string |
| NegativeShape: delete EACH required label in EVERY #EnvironmentRecords descendant, including #Ref/#Pin/#Scale/#Anchor/#Stratum | shape | P of the deleted label |
| NegativeShape: set EACH array to null or scalar | shape | P of that array |
| NegativeShape: set repositories/toolchains/oracles/receipts/stratum.scale/claim.requirements to [] | shape | P of that array |
| NegativeShape: unknown tier/split/axis/class/prediction/polarity/exclusion.kind | shape | P of that exact enum field |
| NegativeShape: all empty or whitespace-only #Text members | shape | P of that member |
| NegativeShape: 39/41/nonhex/uppercase commit; 63/65/nonhex/uppercase digest, independently at every instance | shape | P of that exact commit/digest |
| NegativeShape: checker.version empty | shape | /requirements/0/checker/version |
| NegativeShape: positive_minimum or negative_minimum 29, fractional, negative or >2147483647 | shape | /requirements/0/positive_minimum or /requirements/0/negative_minimum, exactly the mutated member |
| NegativeShape: negative/fractional/>9223372036854775807 scale/excluded int64 member | shape | P of that member |
| NegativeShape: anchor.start_line 0/fractional/>2147483647 | shape | /witnesses/0/anchor/start_line |
| NegativeShape: anchor.end_line 0, below start_line, fractional or >2147483647 | shape | /witnesses/0/anchor/end_line |
| NoAuthoredPass: inject each of status/pass/admitted/adequate/valid/fresh/unknown into EVERY closed record, with false and true separately | shape | P of the injected key |
| NoAuthoredPass: inject top-level key x/y~z | shape | /x~1y~0z |
| References: set witness environment.id to missing-e | reference | /witnesses/0/environment/id |
| References: set witness requirement.id to missing-q | reference | /witnesses/0/requirement/id |
| References: set claim environment.id to missing-e | reference | /coverage/0/environment/id |
| References: set claim requirements[0].id to missing-q | reference | /coverage/0/requirements/0/id |
| References: anchor repository id -> r-missing or commit -> 40 c characters | reference | /witnesses/0/anchor/repository/id or /witnesses/0/anchor/repository/commit, exactly the mutated member |
| References: witness checker name -> other, version -> 2, or sha256 -> 64 3 characters | reference | /witnesses/0/checker/name, /version or /sha256, exactly the mutated member |
| References: claim stratum language -> python or prediction -> null (string) | reference | /coverage/0/stratum/language or /coverage/0/stratum/prediction, exactly the mutated member |
| References: claim stratum subtype -> long while requirement subtype is empty | reference | /coverage/0/stratum/subtype |
| References: requirement.card.id -> different-fixture-card | reference | /requirements/0/card/id |
| References: claim stratum scale[0].class -> M | reference | /coverage/0/stratum/scale/0/class |
| References: claim capability -> different-fixture-card | reference | /coverage/0/capability |
| References: duplicate requirement stratum axis appended at index1 | identity | /requirements/0/stratum/scale/1/axis |
| References: duplicate exclusion appended at index4 | identity | /environments/0/scale/excluded/4/kind |
| References: remove one of four required exclusion categories | shape | /environments/0/scale/excluded |
| References: duplicate head/requirement/witness/claim id by appending a full copy | identity | /environments/2/id, /requirements/1/id, /witnesses/120/id, /coverage/2/id respectively |
| References: duplicate repository id appended to head e-a | identity | /environments/0/repositories/1/id |
| References: duplicate claim requirement appended | identity | /coverage/0/requirements/1/id |
| Paths: set any Ref.path or Anchor.path to /x, ../x, a/../x, ./x, a/./x, a//b, a/, a\\b, NUL/control-bearing text | path | P of that path |
| Paths: repository URL userinfo, query, fragment, non-HTTPS, or empty public URL | path | /environments/0/repositories/0/url |
| Paths: constructed URL nonempty | path | /environments/0/repositories/0/url |
| PriorRead: split held-out and prior_reads [R(read-a)], same-pin OR changed-pin variant | reference | /environments/0/prior_reads |

Check reciprocal membership rather than any same-named repository: anchor pin
must match its referenced environment. Require exact checker equality. No tests
pretend a changed external SHA is detectable without bytes. Valid 64-hex digest
mutation of an external receipt continues shape-success, but downstream resolution
must reject actual mismatch. Distinct MatchID/Origin labels are not an independent
lineage check. Other authored ENV record ownership remains the REC checker test
under U1, not filesystem behavior in ValidateEnvironmentRecords. Concept keys
are governed separately by the single-file/schema/pathcheck law above.

`TestValidateEnvironmentRecords_ErrorOrder` mutates witness0 anchor path to ../x
and checker version to different value. Expect ValidationPath and
`/witnesses/0/anchor/path` in both CUE-field orders. Add an unknown top-level key
`z`: shape wins first at `/z`, semantic errors never replace it. Remove that key
and original expectation returns. For two shape errors at `/coverage` and
`/environments`, `/coverage` wins. CUE's incidental error-list order is not proof.

`TestEnvironmentRecords_JSONArrays` additionally marshals every error-return
variant before inspecting err, with the exact four-array JSON already stated.

Schema tests retain only schema-defined collections: ConceptInventory (including
ConceptRecord.RequiredBy/Expectations and ConceptGroup.Concepts), EnvironmentRecords,
FleetManifest, schema-defined FleetResult (root Parents:[], empty Truth),
ModeDiagnostic.Evidence and SnapshotSample.Evidence.

T3-4 does not move fleet types;
no schema test or production file imports internal/benchenv. Errors.As through
the CLI dependency wrapper belongs to C0, not schema tests.

### Fleet manifest fixture (data and pure validation only)

`product` = bytes `package p\n// seed=@ENV_SEED@\n`; `key` = the exact existing
fauxtown links.answer.json bytes, copied unchanged. Source digests come from these
literal bytes, expected expanded digest from literal `package p\n// seed=7\n`,
independently in the test (not from the generator).
Manifest seed7/version env-fleet/v1/template `chain-pair`, two repositories a/b,
each commits c0 (no parents, empty files) and c1 (parent c0, one file p.go,
mode100644/class product/seeded true/content_id product). Head c1. Truth is one
file links.answer.json, content_id key, source/output digest equal original key,
mode100644/class generated/seeded false. Size 4 lines/2 files/2 repos/4 commits.
Limits 2 repos/4 commits/3 entries/(2*len(expanded product)+len(key)) bytes.

The literal source and expanded-byte strings above identify the fixture's
digest inputs only. The limits are request fixture values. They do not authorize
expansion, size measurement, cap enforcement or materialization in the pure API.

Each negative starts from that exact manifest; all FleetError assertions use
errors.As and exact Reason/Path. Shape validator dispatchability never requires
sources or an output root. Generator checks remain a distinct subsequent stage.

| Mutation | Exact typed error / reason | Path |
|---|---|---|
| Missing required manifest/commit/file member, unknown enum/version, negative/fractional/overflow size, seed outside uint64 | FleetError / shape | pointer to mutated member |
| p.go -> ../p.go, /p.go, a//p.go, .git/config, backslash/control path | FleetError / path | /repositories/0/commits/1/files/0/path |
| append repository a | FleetError / duplicate | /repositories/2/id |
| append c1 in repository a | FleetError / duplicate | /repositories/0/commits/2/id |
| append p.go in c1 | FleetError / duplicate | /repositories/0/commits/1/files/1/path |
| c1 parent -> missing | FleetError / parent | /repositories/0/commits/1/parents/0 |
| c0 parent -> c1 (cycle) | FleetError / cycle | /repositories/0/commits/0/parents/0 |
| append repeated parent c0 to c1 | FleetError / duplicate | /repositories/0/commits/1/parents/1 |
| head -> missing | FleetError / parent | /repositories/0/head |
| head -> c0, leaving c1 unreachable | FleetError / parent | /repositories/0/head |
| add path p.go/child beside p.go | FleetError / path | /repositories/0/commits/1/files/1/path |

Use source input index for all manifest pointers, including the competing-error
case; process shape before semantic validation. For semantic candidates sort the
same pointer/reason law as ENV validation.

`TestValidateFleetManifest` in `schema/benchenv/validate_test.go` accepts the
exact fixture above without source bytes, rejects every manifest shape/path/DAG/
identity mutation in that table with its exact FleetError reason/pointer, and
asserts the initialized FleetManifest failure JSON. It deliberately cannot detect
valid-hex wrong digests or measured-size mismatch without sources, nor root/IO/
limit exhaustion. Those are generator tests. Missing raw sources never becomes
schema failure, and successful shape validation never calls an oracle.

### Complete request fixture data

`FixturesInput` containing the concrete shape fixture,
R(catalog), evidence:[]

Per-mode fixture values beyond FixturesInput use these exact replacements:
portfolio subjects [R(subject-a),R(subject-b)], catalog R(catalog), evidence [];
baseline matrix R(matrix), preflights two entries pair p1/arms treatment,control,
environment R(e-a), profile 64 `3`, receipts R(pre-t),R(pre-c), outcomes one
original pair p1/treatment/run new-run/cell_key 64 `4`/raw R(outcome);
audit sources/runs/rulings [R(source)]/[R(run)]/[R(ruling)]; seed the fleet fixture,
sources [R(product),R(key)], requirement q-flow, catalog R(catalog); candidate
R(subject-input-a) referencing exact CTR-13 SubjectInput, records fixture, catalog
R(catalog), evidence []; seal R(admission),R(e-a), prior_reads [], R(topology),
oracle_keys [R(key)]; reading uses the ReadingInput fixture below with existing_runs/cell_keys [];
snapshots 21 hashes fmt.Sprintf("%040x",i), i=1..21, product commit 40 `c`, binary
64 `d`, service fixture, wall_cap_ms 1000; model-rerun reading-id target,
R(prereg)/R(approval)/R(spend), namespace fresh, existing_namespaces [], max_usd_micros
100, wall_cap_ms 1000; landed-replay input R(landed-input), wall_cap_ms 1000.
These are references/shapes, never signed approvals.

ReadingInput fixture: id fresh-1/run new-run/measured_at `2026-11-01T23:59:59Z`,
environment R(e-a), matrix R(matrix), fingerprint R(fp), raw R(raw-new), the
baseline preflights/outcomes above, claim_time `2026-12-01T00:00:00Z`, current
fingerprint R(fp), empty existing inventories.

### Request mutation assertions

`TestEnvironmentRequest_Negatives` (ENV-1 validate_test.go) walks each of eleven
payload fixtures using ONLY ValidateEnvironmentRequest(source). Missing OR null
selected payload -> ModeShape at its payload pointer. With a valid selected
payload, add another -> ModeShape at that extra payload pointer. Change the tag
to another known mode while retaining the original payload -> ModeShape at the
new selected missing payload pointer; this is source union consistency, not an
outer CLI comparison. Unknown mode -> ModeUnknownName at /mode; repeated mode
label -> ModeDuplicate at /mode (same/different values, JSON and CUE). Unknown
member -> ModeShape at its exact pointer. Remove each required descendant label,
mutate its enum/digest/commit/count/path or Ref canonical path -> ModeShape at that
member pointer. All failures return the zero EnvironmentRequest, with exact safe
ModeError and no raw value. Schema tests assert no process output/exit/handler
behavior because their API receives no CLI mode or command adapter.

## Failing-test inventory

The implementing lane must demonstrate real assertion RED before implementation.
A missing API compile failure is not that demonstration. Every core assertion
below remains mandatory, including RecordOwnership for other authored ENV records.
Concept inventory validation and ownership have no RecordOwnership prerequisite. No failing Go tests
are landed by this publication. The concrete fixtures and mutation tables above
bind this inventory. C0's cause-identity/process tests are not moved here.

### Concept inventory shape, ownership and join assertions

Except the separately owned data-lane PathGrant proof, tests here are proposed
in `schema/benchenv/validate_test.go`, not executed
by this publication. The fixture uses the records fixture above and an injected
nonempty, non-whitespace key K, with no production vocabulary assumption. Its
inventory has Concepts: map[string]ConceptRecord{K: {RequiredBy: one Card
citation equal to q-flow.Card, Expectations: [{MetricCrossFilePaths, 30,
MetricUnitPaths, ""}]}}, and one ConceptGroup{Card: q-flow.Card, CheckerContract:
q-flow.CheckerContract, Concepts: [K]}. cardIDs contains q-flow.Capability.
In `valid.cue`, the synthetic inventory fixture has label `conceptInventory`;
it is not a second production inventory. The exact future schema testdata pair
is `schema/benchenv/testdata/valid.cue` (the conceptInventory value) and
`schema/benchenv/testdata/invalid.cue` (the fully commented
conceptInventoryConflict example below; every stored negative remains commented).
Tests extract the valid container before validation. For the negative, select
only that exact commented example, strip one leading `// ` from each line, and
write those unchanged uncommented bytes to owned scratch under workspace, using
the existing schema/package for explicit vetting. Validate the extracted
conflicting inventory value, never an always-active stored testdata negative.
A second key
L is supplied independently, differs from K as an evaluated field name, and is
never a contract member. Distinct raw label spellings alone do not establish
distinct concept identities.

Each negative below asserts errors.As to *ValidationError, exact reason and
pointer, safe Error() with no rejected value, unchanged inputs, and (for
ValidateConceptInventory) exact failure JSON `{"concepts":{},"groups":[]}`.
Inventory pointers below are local to that validator unless prefixed explicitly.

| Exact test name | Exact assertions |
|---|---|
| `TestValidateConceptInventory_Valid` | The synthetic fixture decodes, retains evaluated field name K exactly and round-trips only declared JSON fields. Replace its citation with each ConceptDesignBED2/ConceptDesignBED9 selector, then use both as separate citations: all decode. Card and design-row citations marshal only their selected arm, never a null optional arm. Empty concepts/groups decode to non-nil empty map/slice. No I/O, clock, native checker or model call. |
| `TestValidateConceptInventory_OpenKeys` | Add the supplied L with a BE-D9 citation and add L to the group: inventory and join pass after adding an otherwise identical requirement with distinct ID and Concept L and referencing it in both claims. Changing K consistently to another supplied evaluated #Text field name, with exactly matching reference string values, passes without editing schema, constants or contract; raw source spellings are not the equality oracle. Whitespace-only struct field name refuses ValidationShape at /concepts/<K>. No test pins inventory length or matches a list of permitted keys. |
| `TestValidateConceptInventory_Shape` | Missing each required field, null/scalar/array concepts, null/scalar group array, empty required_by or group concepts, malformed digest and unknown member each refuse ValidationShape at the mutated field pointer. For missing required label report its own pointer. No citation arm or both citation arms refuse at /concepts/<K>/required_by/0; null card refuses at /concepts/<K>/required_by/0/card; unknown design_row refuses at /concepts/<K>/required_by/0/design_row. Unknown status/pass/admitted/adequate/valid/fresh fields, with true and false, refuse at their exact pointer in every closed inventory record. Empty/whitespace Ref text and unknown citation labels also refuse at their exact field pointer. |
| `TestValidateConceptInventory_Expectations` | For EVERY canonical metric except history_commits, omit both minimum and lower_minimum with the exact matching unit: concrete CUE export/decode succeeds, emits Minimum30 with no lower_minimum, and explicit30 is byte-equal after validation. Use exact subtest names Defaults/<metric>, expanded for every such canonical metric; explicitly include absent_symbols, unwired_entries, fanin_symbols, linked_repositories, linked_services, long_outputs, short_outputs, route_questions, rank_questions and table_questions, plus the ordinary cross_file_paths control. An unresolved disjunction or merely shape-valid incomplete value fails each assertion. Omitted history minimum intentionally refuses ValidationShape /concepts/<K>/expectations/0/minimum; explicit history200 passes. Remove/null/empty/scalar expectations -> ValidationShape /concepts/<K>/expectations; scalar entry -> that entry pointer. Missing/null/empty/unknown metric or unit -> its exact member. positive/negative/none refuse /metric. Known cross_file_paths+commits refuses /unit. minimum0/-1/30.5/overflow/null refuses /minimum. cross_file_paths29 refuses /minimum even with an unrelated valid citation. For EACH below30 relation in the canonical table, its exact cited floor passes, absent/wrong/null/empty/unknown citation refuses /lower_minimum, and floor-1 refuses /minimum (when floor1, zero is shape). History199 refuses /minimum,200 passes. Minimum30 with lower_minimum present refuses /lower_minimum. Unknown field refuses its pointer. Duplicate metric entry refuses ValidationIdentity at /concepts/<K>/expectations/1/metric. Assert exact error, safe text, unchanged inputs and initialized empty-inventory JSON for EVERY refusal. |
| `TestValidateConceptInventoryValue` | Run every expressible typed mutation from the inventory shape/identity/reference/expectation tables through the value validator: same *ValidationError reason and local pointer as source validation, with unchanged input. A validated decoded default30 passes; direct Minimum0 refuses /concepts/<K>/expectations/0/minimum. nil Expectations refuses /concepts/<K>/expectations; empty nonnil Expectations also refuses there. No CUE parse, I/O, native call or duplicated metric policy. Nil Concepts refuses ValidationShape /concepts; empty nonnil Concepts/Groups pass without a closure claim. |
| `TestValidateEnvironmentConceptJoin_ExpectationScope` | For each long/short/route/rank/table metric, bind a cited aggregate presence1 expectation to Subtype:"": join passes; bind the same record to an explicitly claimed nonempty subtype: ValidationReference /inventory/concepts/<K>/expectations/0/minimum. Use a separately reviewed synthetic record with the selected two metrics30: join passes. Do not infer selector from key, Card title or checker-ref ID. An expectation cannot lower Requirement.PositiveMinimum/NegativeMinimum below30. |
| `TestValidateConceptInventory_Identity` | Conflicting concept declarations are the KeyConflict source test, never a typed map duplicate. Repeat group key: ValidationIdentity /groups/0/concepts/1. Append a group copy: ValidationIdentity /groups/1/checker_contract/id. Repeat Card citation: ValidationIdentity /concepts/<K>/required_by/1/card/id; repeat BE-D2 citation: ValidationIdentity /concepts/<K>/required_by/1/design_row. Compatible repeated concept declarations remain valid CUE, tested below; other record-id duplicate laws are unchanged. |
| `TestValidateConceptInventory_References` | Replace group key with L without defining L: ValidationReference /groups/0/concepts/0. Add a second group with a distinct CheckerContract.ID but same Card.ID and changed card.sha256: ValidationReference /groups/1/card/sha256. Change its card.path instead: ValidationReference /groups/1/card/path. Card citations matching a group ID but conflicting in path/sha256 refuse at the changed citation member. Apply every Ref.path mutation from the Paths table to both citation and group references: ValidationPath at that exact path pointer. |
| `TestValidateEnvironmentConceptJoin` | Valid fixture returns nil. Empty cardIDs plus empty records/inventory returns nil without claiming closure; a nonempty cardIDs set with no groups refuses ValidationReference /card_ids/0. Remove all groups from the valid inventory: ValidationReference /card_ids/0. Add an otherwise unused card ID at cardIDs[1]: ValidationReference /card_ids/1. With records empty and the concept using a BE-D2 citation, remove cardIDs[0]: orphan group refuses ValidationReference /inventory/groups/0/card/id. Add a concept citing a card outside cardIDs: ValidationReference /inventory/concepts/<L>/required_by/0/card/id. Invalid cardIDs[0] refuses ValidationShape /card_ids/0; duplicate at index1 refuses ValidationIdentity /card_ids/1. Replace the sole ID with a distinct supplied title or surface string: ValidationReference /card_ids/0. |
| `TestValidateEnvironmentConceptJoin_Requirements` | With records coverage/witnesses empty to avoid dependent local errors, change requirement capability and its Card.ID together to an unknown ID: ValidationReference /records/requirements/0/capability. Change only claim capability to an unknown ID: the existing local check refuses ValidationReference /records/coverage/0/capability. Change requirement Concept to undefined L, or defined L absent from its group: ValidationReference /records/requirements/0/concept. Change CheckerContract.ID to an unmapped ID: ValidationReference /records/requirements/0/checker_contract/id. Change either mapped Card/CheckerContract path or sha256: ValidationReference at that exact /records/requirements/0/card or /checker_contract member. Changes to Card.ID inconsistent with capability remain the existing local ValidationReference /records/requirements/0/card/id. |
| `TestValidateEnvironmentConceptJoin_GroupClosure` | Add a second group with the same Card ref, distinct CheckerContract.ID and Concepts [K]; shape passes, join refuses ValidationReference /records/coverage/0/requirements until both claims include a new requirement for that exact group/stratum. Then join passes. Add supplied L with design citation and extend either group's concepts: missing L similarly refuses until both claims reference matching requirements. Removing one required member from the first claim refuses at /records/coverage/0/requirements (leave another member so local nonempty shape passes). Two distinct requirement IDs with the same group/concept referenced by a claim refuse ValidationIdentity /records/coverage/0/requirements/1/id. No group-specific oracle obligation or count is reported as verified. |
| `TestConceptInventory_JSONArrays` | Marshal validated empty inventory as exactly {"concepts":{},"groups":[]}; every validator failure returns that same JSON before err is inspected. Concepts remains a JSON object; populated RequiredBy, Expectations, Groups and group Concepts retain arrays. No null slice or undeclared key. Populated Expectations marshal as arrays; empty/null Expectations on a present record refuse. Missing optional citation arms and a zero-valued LowerMinimum are omitted. |
| `TestConceptInventory_ErrorOrder` | Unknown top-level z and a semantic bad group key: ValidationShape /z wins. Remove z: ValidationReference /groups/0/concepts/0. Two shape errors at /concepts and /groups choose /concepts in either CUE field order. Join prefixes local inventory errors with /inventory and record errors with /records; invalid records are checked before inventory, inventory before cardIDs, then all join candidates use pointer/reason ordering. Two semantic errors, missing cardIDs[1] mapping and unknown requirement concept, choose /card_ids/1. No mutation, map iteration or CUE diagnostic ordering changes that result. |

These test fixtures exercise the shape and join without allocating production
keys or claiming the caller's Card approval/byte-check/CheckCardClosure proof.
No RecordOwnership or recordchecker result is needed for these concept tests.
Other authored ENV record ownership remains separately tested below.

The valid/invalid pair has exactly this synthetic concept content; K is a fixture
label, never an allocated production id. Retain the schema definitions above.
In `schema/benchenv/testdata/valid.cue`:

```cue
conceptInventory: #ConceptInventory & {
    concepts: {"K": {
        required_by: [{card: {id: "opaque-fixture-card", path: "fixtures/opaque-fixture-card.json", sha256: "0000000000000000000000000000000000000000000000000000000000000000"}}]
        expectations: [{metric: "cross_file_paths", unit: "paths", minimum: 30}]
    }}
    groups: [{
        card: {id: "opaque-fixture-card", path: "fixtures/opaque-fixture-card.json", sha256: "0000000000000000000000000000000000000000000000000000000000000000"}
        checker_contract: {id: "native-flow-contract", path: "fixtures/native-flow-contract.json", sha256: "0000000000000000000000000000000000000000000000000000000000000000"}
        concepts: ["K"]
    }]
}
```

In `schema/benchenv/testdata/invalid.cue` (fully commented conflicting example):

```cue
// conceptInventoryConflict: #ConceptInventory & {
//     concepts: {
//         "K": {
//             required_by: [{design_row: "BE-D2"}]
//             expectations: [{metric: "cross_file_paths", unit: "paths", minimum: 30}]
//         }
//         "K": {expectations: [{metric: "cross_file_paths", unit: "paths", minimum: 31}]}
//     }
//     groups: []
// }
```

| Exact test name | Exact assertions |
|---|---|
| `TestValidateConceptInventory_KeyConflict` | Run cue vet -c with the existing schema and exact stored valid.cue plus fully commented invalid.cue: exit0. Extract only the commented conceptInventoryConflict example into owned scratch, removing exactly one leading // plus one space from each shown line; assert the resulting bytes equal the shown30/31 witness without comments. Explicitly vet the schema plus that scratch negative: nonzero with a conflict at conceptInventoryConflict.concepts.K.expectations.0.minimum, concrete30 versus31. Validate its extracted inventory source: ValidationShape /concepts/K/expectations/0/minimum and exact empty-map failure JSON; reverse the two declarations: identical reason/path/result, not CUE diagnostic-order dependence. Never require the stored invalid.cue to fail vet or infer full-gate failure from scratch package inclusion. No source key discovery/checker is called. |
| `TestValidateConceptInventory_CompatibleDeclarations` | Repeat K with the same complete content, then split required_by and expectations across two K declarations: both cue vet -c and source validation succeed and export the same single K map value as the valid fixture. Do not assert that CUE rejects compatible repetition. The distinct other-record-id RecordOwnership tests remain mandatory. |
| `TestConceptInventory_PathGrant` | In the future data lane, supply its one exact-file path grant to the existing pathcheck. Only that proposed inventory path passes. A changed/new second inventory path and a changed/new ungranted path fail, even with otherwise valid CUE; the path check need not inspect content or allocate keys. An empty or permissive glob is not an exact-file grant. This is pathcheck proof, not a pure schema/Go validator filesystem test; the data lane owns it, no schema test imports a path checker. No actual grant is made here. |
| `TestConceptInventory_MapKeys` | Use each of slash, tilde, space, Unicode λ and combined `a/~ λ` as supplied valid field labels; these particular spellings evaluate unchanged, so their decoded map keys round-trip exactly. General Unicode field evaluation is tested separately below, not promised raw-byte preservation. For the combined evaluated key omit expectations: ValidationShape /concepts/a~1~0 λ/expectations; duplicate expectation metric: ValidationIdentity /concepts/a~1~0 λ/expectations/1/metric. Repeat in the join: exact /inventory prefix. Empty label refuses at /concepts/ and whitespace-only label at its escaped member. Nil Concepts refuses /concepts. Permute CUE field order and Go map insertion order, also placing errors under distinct keys: identical JSON and selected lowest escaped pointer/reason. With missing expectations under both a/b and a~b, choose /concepts/a~0b/expectations (escaped-pointer order), in either source/map order; its join twin chooses /inventory/concepts/a~0b/expectations. Traverse evaluated map keys by UTF-8 bytes, then select errors by the existing escaped-pointer/reason law. No trimming, ENV normalization or raw key in Error(). |
| `TestConceptInventory_CUEFieldSemantics` | Use literal source label NFD e\u0301, codepoints U+0065 U+0301, UTF-8 65cc81, and NFC \u00e9, U+00E9, UTF-8 c3a9; record input bytes/codepoints and raw export stdout/stderr/exit. Assert raw exported field-label UTF-8 is c3a9 (U+00E9), not65cc81, while each raw exported reference string retains its supplied codepoints/bytes. SourceNFD and SourceNFC each pass cue vet -c and concrete export; exact decoded Concepts map is {"\u00e9": record}, size1, key UTF-8 c3a9, never a retained NFD key. Decode to the Go ConceptInventory: exactly that NFC key/unchanged record, initialized Groups. CompatibleNFDFirst and CompatibleNFCFirst declare both raw spellings with the same complete record: vet/export/source validation pass, one map entry, output equal to the single NFC case, never a duplicate-id error. ConflictingNFDFirst and ConflictingNFCFirst attach minimum30 to NFD and31 to NFC, changing only declaration order: vet/export fail with conflicting30/31 at the evaluated field; source validator returns ValidationShape /concepts/é/expectations/0/minimum and exact empty-map failure JSON in both orders. ReferenceNFC and ReferenceNFD cross EACH label spelling with a group's concept string value NFC or NFD: all four CUE vet/export cases pass shape, exported label is NFC, reference VALUE bytes stay respectively c3a9 or65cc81. NFC reference passes source/value validation; NFD reference refuses ValidationReference /groups/0/concepts/0, join twin /inventory/groups/0/concepts/0, without mutation or reference rewriting. With valid NFC group mapping, an NFD Requirement.Concept refuses join ValidationReference /records/requirements/0/concept; NFC requirement resolves against the same complete Card/checker refs. Decoded map key and each retained reference value must match exact codepoints and bytes above. For direct typed Go maps, no CUE parse/normalizer runs: exact matching supplied key/value strings resolve, mismatched strings refuse at the same existing group/requirement pointer; this local equality test claims no authored-source/provenance acceptance. Repeat the two single-label cases in a schema-free CUE struct {keys: {label:30}, reference:label}: fields evaluate NFC while reference values retain their original NFD/NFC bytes. Existing CUE field semantics is the only identity mechanism. |
| `TestConceptInventory_CommentedNegative` | Read stored invalid.cue: every nonblank line begins with //, including the complete conceptInventoryConflict example; no active stored negative. The exact shown example uses // followed by one space on each line. Copy only that example, strip exactly that prefix per line, and assert the bytes equal the unchanged30/31 witness. Write only owned workspace scratch with the existing package/schema context. Explicit cue vet -c of schema+stored valid+commented invalid returns0; explicit schema+extracted scratch negative returns nonzero at conceptInventoryConflict.concepts.K.expectations.0.minimum. Source-validator refusal is the exact KeyConflict assertion above. This extraction is test-only reuse of the commented-negative convention, no production parser or new fixture format. Do not rely on ./... to discover testdata, move schema paths or claim a full-gate result. |

### T3 metadata and subtype assertions

| Test | Required assertions |
|---|---|
| TestValidateEnvironmentRecords_T3Metadata | Concrete Card.ID-bound fixture and aggregate subtype:"" decode; remove card/checker_contract/subtype -> ValidationShape at /requirements/0/card, /requirements/0/checker_contract, /requirements/0/stratum/subtype respectively; wrong card.id -> ValidationReference at /requirements/0/card/id; card path traversal -> ValidationPath at /requirements/0/card/path; checker_contract bad digest -> ValidationShape at /requirements/0/checker_contract/sha256. Every failure returns the unchanged exact four-array JSON. |
| TestValidateEnvironmentRecords_T3Subtype | Claim subtype long versus aggregate requirement -> ValidationReference /coverage/0/stratum/subtype. Matching subtype values are shape-valid with no Card-claim verification or native call; whitespace-only subtype -> ValidationShape at that mutated subtype pointer. Shape-only success cannot certify a per-subtype Card claim. |

### ENV-1, `schema/benchenv/validate_test.go`

| Exact test name | Required assertions |
|---|---|
| `TestValidateEnvironmentRecords_Valid` | Both public and constructed named fixtures decode. Null scale counts survive as unknown, known zero survives as zero. All fields round-trip with exactly the declared JSON keys. No file/model/network access. |
| `TestValidateEnvironmentRecords_NegativeShape` | Separate subtests remove each required field from every definition, set each enum to an unknown value, use 39/41-character or nonhex commits, 63/65-character or nonhex digests, empty checker version, minimum 29, negative/fractional counts, line zero and reversed line range. Each returns `*ValidationError` with `ValidationShape` and the offending field path. |
| `TestValidateEnvironmentRecords_NoAuthoredPass` | Inject each of `status`, `pass`, `admitted`, `adequate`, `valid`, `fresh` into each authored record type, plus an unrelated unknown field. Every mutation refuses, including `false` values. |
| `TestValidateEnvironmentRecords_References` | Missing in-bundle environment/requirement ID, wrong anchor environment pin, incompatible requirement/claim stratum, duplicate axis, duplicate exclusion category and incomplete exclusions all refuse with the exact reason/path in the mutation table. External oracle/receipt refs are checked for shape only. A syntactically valid wrong external digest is not falsely reported as verified. No default repairs missing evidence. |
| `TestValidateEnvironmentRecords_Paths` | Absolute, traversal, dot, repeated-slash noncanonical, backslash, NUL/control paths, credential URL, URL query/fragment and public empty URL refuse; constructed empty URL passes. The mutation table fixes ValidationPath and its precise pointer; no secret value is echoed. |
| `TestValidateEnvironmentRecords_PriorRead` | Nonempty prior reads plus held-out split refuses. A receipt-backed development head passes. Keeping a prior-read ref and changing the repository pin still refuses; cross-record exposure discovery is the T3-2 native adapter, never hidden I/O in this pure function. |
| `TestValidateEnvironmentRecords_RecordOwnership` | For OTHER authored ENV records only, after REC-1 lands, two declarations of the same record id in different owned files refuse even with disjoint fields that CUE could unify. An out-of-grant record definition refuses. Existing grandfathered record inventory remains valid, an unrelated granted ID is permitted, a newly duplicated ID fails, and reuse of a grandfathered ID refuses. Load trusted effective policy and grants and call shared REC ValidateInventory; no local substitute, new record-id family, hardcoded repository exceptions or waived assertions. Discovery/grants for those records remain U1. Exclude concept struct keys: their CUE conflicts and single-file pathcheck are the separate tests above, with no REC prerequisite. |
| `TestEnvironmentRecords_JSONArrays` | Schema-defined collections only: EnvironmentRecords, FleetManifest, FleetResult (including FleetRepositoryResult.Commits/FleetCommitResult.Parents), ModeDiagnostic.Evidence and SnapshotSample.Evidence. Assert [] and exact initialized validator failure literals. Optional union members omitted. No CoverageResult, EnvironmentReceipt or other leaf import/assertion; those belong to TestEnvironmentCoverage_JSONVocabulary. |
| `TestEnvironmentRequest_Negatives` | Exact eleven-payload source union/tag consistency and ModeError assertions above. No outer CLI mode, exit or leaf-result assertion. |
| `TestValidateFleetManifest` | Exact fleet shape/path/DAG/identity fixture mutations and initialized FleetManifest failure literal above, without source I/O or materialization claims. |
| `TestFleetManifest_GeneratorVersion` | Reflect FleetManifest.GeneratorVersion as the named GeneratorVersion type, with GeneratorVersionV1 exactly "env-fleet/v1"; round-trip via the unchanged generator_version JSON key. The valid fixture using that constant passes. Missing, null, empty, unknown string and non-string versions each refuse FleetShape at /generator_version with exact initialized failure JSON. SeedInput's nested manifest uses the same CUE #GeneratorVersion: each corresponding mutation refuses ModeShape at /seed/manifest/generator_version. No arbitrary future version is accepted. |

## Proof boundary

This contract does not implement or type-check its proposed APIs. Future core
implementation proof remains `go test -race ./schema/benchenv` and
`cue vet ./schema/benchenv`, followed by source-bound integration validation.
Publication checks establish amendment conservation and document/schema consistency
only. No plan box flips, runtime admission or full-contract LAND is claimed.
An independent publication critic is still required.

## Decision models

None. Shape, lineage, counts, preflight and freshness are deterministic evidence
checks with provider `none`. No model assigns a primary label or promotes a
status. Experimental agents remain subjects, with no paid call authorized by
this document (`docs/design/bench-environments.md:269-274`).
