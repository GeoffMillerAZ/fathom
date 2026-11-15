# CTR-10: facts, bindings, deltas and hazard contract

Status: DRAFT W1-5/#264. No implementation or RED claim.
Base: `468dc705813382d584926887b451cbea145c76ac`. Independent review branch.
Owns only this document. No companion fixtures land in this draft.
Landing dependencies: approved REC-1, CTR-4 path gate, ENG-2 and LIB-1 as authored.
Implementation/test fixtures described here are future approved handoffs only.

## Intent and decisions before design

Read intent router, P2/P3/P20/P22, scope compatibility/non-goals, ADR-091/092,
change-hazards design sections 8/9, rows HZ-3/4/6/9/10 and LIB-7. Catalogs are
operator-placed read-only facts. No plans/providers/downloads execute in product.
ResourceSet/Attr/Class/Span and Catalog/Contract remain the public types, mirrored
by existing iacfacts types through convert.go. No internal aliases at the door.
W2-1 answers ADR-092's fired LL-M1 trigger: door 80, consumer 81. No new imported
package allowed without removing one or ruling. F97's general cross-tool identity
and IAM models remain deferred. No future fields, new table, tool or execution.
Changes to public JSON/API are additive and require API/golden review, not an
implicit compatibility exemption. No evidence other revisit conditions fired.

Settled decisions (citations are governing text, not evidence of implementation):
- D1 reuse five classes and value-free checks: design change-hazards §8 says
  “All read class, contract and change. None reads a value.” Alternative numeric
  policy evaluation crosses that boundary and is excluded.
- D2 reuse binding resolver and catalog: HZ-4 explicitly requires the public
  resolver; ADR-091 places facts in catalogs at query time, not dependency/store.
  Alternative second resolver is rejected by E8 “Do not duplicate them.”
- D3 H2/H4/H5 before stateful catalogs: HZ-10 specifies these checks and H5 advice
  only. HZ-9 uses the person-accepted stateful list as declared context, never a
  provider fact. Alternative promoting the list to contract truth violates HZ-9.
- D4 no package growth: W2-1, alternative remove an existing package before adding.
- D5 real RED in implementation lane: W2-2 D5 precedent and PL-D1; alternative
  tagged contract tests. Landable default-build failures conflict with the gate.

Unsettled U1 (recommendation): approve the concrete additive shapes/signatures
below. Alternative stage only internal fixtures until public API review. HZ rows
settle behavior, not these exact field spellings. No self-approval for U1.
Unsettled U2: reserve the listed existing-file integrations serially after #164,
#166/#167/#168; alternative split integration into one architect lane. Directory
ownership alone cannot prove disjoint writes. No dispatch until exact grants land.

## Proposed shapes, no implementation

Existing types are incorporated unchanged by name. Public additions live only in
existing `pkg/lang/iac` and internal additions only in existing `internal/iacfacts`.
No package is created. Internal/public copies follow existing explicit conversion.
JSON names below are frozen lower snake case, never Go default field names.

```go
// pkg/lang/iac; additions alongside current ResourceSet/Attr.
type UnknownReason struct {
    Code string `json:"code"`
    Span Span `json:"span"`
    DecidableBy []string `json:"decidable_by"`
}
type ReadEvidence struct {
    Read []string `json:"read"`
    Unread []string `json:"unread"`
    Unknowns []UnknownReason `json:"unknowns"`
}
type Block struct {
    Kind string `json:"kind"`
    Name string `json:"name"`
    Attrs []Attr `json:"attrs"`
    Span Span `json:"span"`
}
// ResourceSet gains Evidence ReadEvidence and Blocks []Block only.
// Unknown Attrs are associated by exact Span, never by raw value.
type Binding struct {
    Status string `json:"status"`
    Source string `json:"source"`
    Version string `json:"version"`
    Catalog string `json:"catalog"`
    Candidates []string `json:"candidates"`
    Notes []string `json:"notes"`
}
func ResolveBinding(ctx context.Context, repo fs.FS, dir string,
    resource Resource, catalogs []*Catalog) (Binding, error)
type Snapshot struct { Set ResourceSet; Sources map[string][]byte }
type ResourceChange struct {
    Address string `json:"address"`
    Kind string `json:"kind"`
    Before *Resource `json:"before,omitempty"`
    After *Resource `json:"after,omitempty"`
    Attributes []AttrChange `json:"attributes"`
}
type AttrChange struct {
    Path string `json:"path"`
    Before Class `json:"before"`
    After Class `json:"after"`
    Kind string `json:"kind"`
}
type ChangeSet struct {
    Changes []ResourceChange `json:"changes"`
    Evidence ReadEvidence `json:"evidence"`
}
func DiffResources(base, head Snapshot) (ChangeSet, error)
```

Codes: `expression`, `syntax-error`, `unread-input`, `unbound-catalog`,
`unsupported-form`, `read-limit`. DecidableBy is a sorted subset of exactly
`plan`, `render`, `vendored-module`, `catalog-version`, `context-file` (HZ-3).
Empty subset means none of these can decide it, not safe. No unbounded prose
reason echoing secrets. Every unknown/expression attribute and unread/dropped
section contributes a reason, even with values disabled. Read/Unread list exact
logical input paths, sorted/disjoint, no absolute host paths. Empty input evidence
is not proof of a complete read. Read limits/refusals retain existing semantics.

Blocks.Kind closes over `variable`, `output`, `locals`, `required_providers`,
`moved`, `removed`, `import` for HZ-4. Block.Attrs reuse existing classes/refs/spans,
never invent a text/value field. Named selectors and references are structural
identifiers. Sensitive/contract-sensitive values stay withheld even WithValues.
Binding delegates to existing ResolveIaCBinding and converts its result, including
refusal/disclosures as Notes. Status is locked/constraint-matched/ambiguous/unbound.
No catalog facts attach to the latter two. Context/deadline/busy rules remain.

Snapshot.Sources is transient caller input to compare source expressions, never
marshaled. Address is `(tool, module directory, mode, type, name)` joined with NUL
internally, escaped as JSON string externally; no cross-tool correspondence.
No rename inference. Kind: added/removed/changed. ResourceChange copies must clear
all Attr.Value fields, regardless of input WithValues. Class transitions include
empty class only to denote absence. Same-class expression spelling changes are
compared from spans using Sources, following cfn_replace.go; whitespace-only
changes normalize the same way as the existing CFN comparator. Missing source,
opaque spans or ambiguous duplicate addresses yield unknown evidence, never
“unchanged”. Type change is remove/add. No secret value or source/hash is emitted.

```go
// internal/iacfacts: structural copies of ChangeSet and ReadEvidence above.
func BuildReadEvidence(set ResourceSet, read, unread []string) ReadEvidence
type Hazard struct {
    ID, Kind, Address, EvidenceClass string
    Span Span
    Reasons []UnknownReason
    Citations []IaCFactRef
}
func CheckRenameWithoutMoved(changes ChangeSet, blocks []Block) []Hazard
func CheckMultiplicityWithoutMoved(changes ChangeSet, blocks []Block) []Hazard
func CheckProvabilityLost(changes ChangeSet) []Hazard
func isStatefulType(tool, typ string, accepted []StatefulType) bool
type StatefulType struct { Tool, Type, Source, AcceptedBy string }
```

H2 is advice for compatible deleted/added declarations, never a proven rename.
Covering moved blocks suppress H2/H4 only for their explicit from/to pair. H4
requires changed count/for_each kind or key expression; H5 literal -> expression/
unknown is advice. All hazards sort by span/id/address. Stateful list entries
must have source + person acceptance; evidence class is `declared context`.
Existing CFNChange/CFNBinding/CFNReplacementFacts and IaCFactRef are reused rather
than replaced. Catalog IF-13 writes existing ContractAttr.Replacement, retains
provenance and keeps absent distinct from conditional/unknown. HZ-11/IF-17 and
cascade remain gated consumers, not extra checks implemented in HZ-10.

## Versioned hazard JSON and registered-handler handoff

Proposed `fathom-lang hazards --base DIR --head DIR --json` reads files only.
Exit 0 for a complete answer even with findings, 1 for read failure, 2 for misuse.
Array envelope: `{"schema_version":1,"hazards":[],"evidence":{"read":[],"unread":[],"unknowns":[]}}`.
Each hazard key is `id,kind,address,evidence_class,span,reasons,citations`.
Positive H2 example uses id H2/kind rename-without-moved/evidence_class advice;
H5 kind provability-lost. Empty arrays remain arrays, no literal values. Exact
positive fixture spans come from `main.tf` fixture bytes, not fabricated location.
API snapshot update and existing command switch integration are architect-owned.

Review dimension receives `Checked bool`, `Reason string`, `AsOf string` and
`Findings []Hazard` in a new internal `HazardDimension` projection. `Checked=false`
requires nonempty Reason; absent catalog/source or timeout cannot become a clean
empty answer. AsOf is the input revision supplied by caller, never wall clock.
#311 must exercise the registered review_diff handler, not a facade-only test.
Exact schema/projection wiring grants must be frozen with #167/#168 first.

## Exact issue ownership snapshot and serial transfer

These are the current issue allowlists, not grants to this draft author. Existing
files remain with the architect until a named serial handoff. Directory grants
are reproduced as authored and must be narrowed to the fixture files below before
dispatch. Tests in a contract batch transfer once, at its landed commit, to the
consumer. No concurrent shared-file owner.

### #303 Fact unknowns (HZ-3)

- `internal/iacfacts/resource.go` (exists).
- `internal/iacfacts/read_evidence.go` (new).
- `internal/iacfacts/read_terraform.go` (exists).
- `internal/iacfacts/read_route.go` (exists).
- `internal/iacfacts/read_cloudformation.go` (exists).
- `internal/iacfacts/read_pulumi.go` (exists).
- `internal/iacfacts/check_module.go` (exists).
- `pkg/lang/iac/types.go` (exists).
- `pkg/lang/iac/read.go` (exists).
- `pkg/lang/iac/convert.go` (exists).
- `internal/mcp/iac_facts.go` (exists).
- `internal/mcp/iac_cfn.go` (exists).
- `internal/iacfacts/read_evidence_test.go` (new).
- `pkg/lang/iac/read_evidence_test.go` (new).
- `cmd/fathom-lang/resources_evidence_test.go` (new).
- `internal/mcp/iac_read_evidence_test.go` (new).
- `pkg/lang/iac/read_test.go` (exists).

### #268 Resolve infrastructure bindings for portable facts (HZ-4)

- `pkg/lang/iac/binding.go` (new).
- `pkg/lang/iac/binding_test.go` (new).
- `pkg/lang/iac/types.go` (exists).

### #269 Compute infrastructure change sets (HZ-6)

- `pkg/lang/iac/diff.go` (new).
- `pkg/lang/iac/diff_test.go` (new).

### #308 Hazard free (HZ-10)

- `internal/iacfacts/hazard_free.go` (new).
- `internal/iacfacts/hazard_free_test.go` (new).

### #307 Hazard binary (LIB-7)

- `cmd/fathom-lang/hazard.go` (new).
- `cmd/fathom-lang/hazard_test.go` (new).
- `docs/bench/readings/library-roadmap/lib-7-hazard-entry.md` (new).

### #312 Hazard stateful (HZ-8, HZ-9, LIB-9)

- `internal/iacfacts/hazard_stateful.go` (new).
- `internal/iacfacts/hazard_stateful_test.go` (new).
- `calibration/iac/stateful-types.json` (new).
- `docs/bench/readings/library-roadmap/lib-9-stateful-candidates.md` (new).

### #313 Hazard upgrades (HZ-11)

- `internal/iacfacts/upgrade_hazards.go` (new).
- `internal/iacfacts/upgrade_hazards_test.go` (new).
- `docs/bench/readings/change-hazards/h6-upgrade-fixtures.md` (new).

### #311 Hazard review dimension (HZ-2)

- `internal/mcp/review_iac_dimension.go` (new).
- `internal/mcp/review_iac_dimension_test.go` (new).

## Frozen test and fixture handoff

Proposed companion drafts: `pkg/lang/iac/testdata/contracts/` files
`unknown.tf`, `bindings.tf`, `base.tf`, `head.tf`, `moved.tf`, `hazards-v1.json`;
`internal/iacfacts/testdata/contracts/stateful.json`. Each exact filename requires
an explicit added fixture grant before dispatch. Current contract author lands none.
Use fake provider `example_widget`, source `registry.example.test/demo/provider`.
No fixture literal secrets appear in expected JSON, even if source contains a canary.

| Consumer / exact test | Assertions and proposed command |
|---|---|
| HZ-3 #303 `pkg/lang/iac/read_test.go::TestLaneFactUnknowns` | unknown/expression each has reason and closed deciding action; read/unread disjoint; WithValues has no effect on completeness; Terraform, CFN, Pulumi and MCP registered routes retain evidence. `go test -race ./pkg/lang/iac -run '^TestLaneFactUnknowns$' -count=1` |
| HZ-4 #268 `pkg/lang/iac/binding_test.go::TestLaneFactBindings` | all seven block kinds, alias/multiple-root conflict, missing pin/catalog; no facts on ambiguous/unbound; secret canary absent under WithValues. `go test -race ./pkg/lang/iac -run '^TestLaneFactBindings$' -count=1` |
| HZ-6 #269 `pkg/lang/iac/diff_test.go::TestLaneFactDelta` | add/remove/class transition/same-class source change; reordered inputs stable; missing source unknown; all values cleared; unchanged CFN IF-C4 fixture answers through port. `go test -race ./pkg/lang/iac -run '^TestLaneFactDelta$' -count=1` |
| IF-13 `cmd/iaccatalog/replacement_test.go::TestLaneCatalogReplacement` | byte-identical rebuild; replace/conditional/absent distinct and provenance retained. `go test -race ./cmd/iaccatalog -run '^TestLaneCatalogReplacement$' -count=1` |
| HZ-10 #308 `internal/iacfacts/hazard_free_test.go::TestLaneHazardFree` | H2/H4 suppressed only by covering moved; H5 advice; reordered deterministic; incomplete read never clean. `go test -race ./internal/iacfacts -run '^TestLaneHazardFree$' -count=1` |
| LIB-7 #307 `cmd/fathom-lang/hazard_test.go::TestLaneHazardBinary` | exact v1 golden, canary absent, exits 0/1/2, no provider subprocess. `go test -race ./cmd/fathom-lang -run '^TestLaneHazardBinary$' -count=1` |
| HZ-9 #312 `internal/iacfacts/hazard_stateful_test.go::TestLaneHazardStateful` | accepted list required, declared-context evidence cannot become catalog fact. `go test -race ./internal/iacfacts -run '^TestLaneHazardStateful$' -count=1` |
| cascade `internal/iacfacts/cascade_test.go::TestLaneHazardCascade` | shared-reference closure stable, unresolved refs counted, no instance expansion. `go test -race ./internal/iacfacts -run '^TestLaneHazardCascade$' -count=1` |
| HZ-11 #313 `internal/iacfacts/upgrade_hazards_test.go::TestLaneHazardUpgrades` | exact two pinned catalogs, absent fact remains unknown; IF-17 owner hands catalog diff. `go test -race ./internal/iacfacts -run '^TestLaneHazardUpgrades$' -count=1` |
| HZ-2 #311 `internal/mcp/review_iac_dimension_test.go::TestLaneHazardReviewDimension` | registered handler, checked/reason invariant, absent/as_of propagation, catalog refs resolvable. `go test -race ./internal/mcp -run '^TestLaneHazardReviewDimension$' -count=1` |

Batch proposed tests: `pkg/lang/iac/hazard_contract_test.go::TestLaneContractsFacts`,
`internal/iacfacts/hazard_contract_test.go`, `cmd/fathom-lang/hazard_contract_test.go`.
Primary command: `go test -race ./pkg/lang/iac -run '^TestLaneContractsFacts$' -count=1`.
They validate fixtures/shapes, not observed scanner precision. Each consumer must
own its runnable failing test and capture base/head/command/exit before implementation.
No tests exist or ran RED because this draft describes them. Keep normal build green.

Reading consumers #270/#271/#302/#309/#310 and hazard-validity/replacement/upgrade
readings are not dispatched by fixture presence. They need independently pinned
inputs/native versions, executable scorer and complete denominators. HZ-1 bar is
0.85 precision, IF-CM3 agreement 0.85, HZ-M1 precision 0.9 as authored. Their separate
contracts must freeze these; do not substitute a positive fixture or invent floors.
CTR-10 does not claim them or authorize native/provider execution. Lowest-wave
reading contracts are the next W2-4 backlog, not hidden implementation here.

## Evidence, proof limits, decision models

Source: pkg/lang/iac/types.go/read.go/catalog.go/convert.go; internal/iacfacts/
binding.go/read_outputs.go/cfn_replace.go/catalog_build.go. CFN comparator already
retains changed-source semantics; current public ContractAttr has Replacement and
JSONPath. No second schema reader, registry or value evaluator is needed.
History source base is pinned above; `git log -1 -- <path>` is a review aid only.
`git diff --check` is the executed draft check. No race/RED/API snapshot/closure/
CUE/full gate run is claimed. CTR-10 and consumer rows remain unchecked.
Decision models: none. Deterministic fact transforms have provider none and no
judgment substitute. Manual review may opt out without claiming automation.
