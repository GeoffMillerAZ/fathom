# ENV-G2: bounded governance preset contract

Status: **DRAFT, W18/T12 preset alignment. Fresh independent critic and W2-3 approval required.** This is the
current ENV-G2 data contract, not a reading. No implementation, materialization,
native result, count, independence, admission, plan completion or effect verdict
is claimed. In particular, no `ENV-G2 verdict:` line is emitted here. This lane owns only the graph and governance
preset documents. Local documentation evidence
and the handback live under ignored `workspace/tmp/`.

## Authority, intent and frozen dependencies

The work order is [ENV-G2](../plans/bench-environments.md), Seed presets. Its
scope is false/true plan rows, stale/live anchored instructions and written
rationale with explicit governing record links. [BE-D2](../design/bench-environments.md),
Inventory concepts, supplies the common independent 30/30 floor and supplementary
counts. BE-D5 supplies seeded truth before ingest, fixture conservation,
concept-removal twins and the seed-replica rule. BE-D3/D4 supply native evidence,
deduplication, scale vectors and the two-independent-environment rule. BE-D9
keeps status computed. None grants another concept, product analyzer or oracle.

Intent-design consulted the [router](../intent/README.md),
[principles](../intent/principles.md), [invariants](../intent/invariants.md),
[scope and compatibility surface](../intent/scope.md),
[non-goals](../intent/scope/non-goals.md), [decision index](../intent/decisions/README.md)
and [future index](../intent/futures.md). P2/P3/P15/P17 mean deterministic inputs,
visible unavailable evidence, owned writes and computed status. I43 requires
live attestation, I54 forbids unsupported language claims, I56 requires
operator-named local inputs and I62 excludes model judgment from proof. The
invariants' stated enforcement limits remain as documented, not newly enforced
by this contract. ADR-018 governs plan verification and product wiring. ADR-085
governs independent anchors and primary truth. ADR-086's trace is a consumer,
not an independent key. Proposed ADR-108/109/110 are the specified benchmark
design context, not assertions that their implementation exists.

The benchmark-validity evidence trigger is already recorded in the design's
Intent before design paragraph. No additional fired trigger or conflicting
accepted decision was found. No dispatch loop, new oracle, model-labelled key,
future feature or new compatibility behavior is introduced. The free option is
to retain separate existing references and the approved port, with no new field
or runtime work. No future is promoted. This document creates no ADR or change
to the shared intent corpus.

The recovered publication `8b5c6ffb3`, replayed byte-for-byte as
`f05c1b0cf9c0332fff74f30e4c8bab00b1198253`, was a DRAFT. It transfers no
independent approval. The prior W16-2 amendment subsequently received its first
independent LAND and W2-3 approval at the exact preset baseline below; that
approval does not transfer to this peer-source amendment. No T8-2 repair round is used. The lane's integration base remains
`17d930d2f1b96b707b03ec84d5f87a3a5333c24d`.

The following immutable sources define the dependencies within the stated
bounds. Exact file bytes were checked against their committed Git objects.
Approval of a document never establishes integration or implementation.

| Dependency and normative bound | Commit and repository-relative path | SHA-256 of file |
|---|---|---|
| Historical W16 amended core: superseded concept shape; retained named version and other schema provenance | `2ada35b791d8115fc05e563ba3af571e624a9895:docs/contracts/env-1-core.md` | `2ac44ac36988aacc9783e76e676b5d2abdd5006afb4ded36241f775fb7047981` |
| Historical full ENV-1: command/loader declarations only; its older fixed concept roster is superseded by the amended core | `8bc356c285b9557f4265497c52e9c00f195903a3:docs/contracts/env-1.md` | `99480d95ad7fbbe0abea9ac3e6c5deba98390bab0b2f4c1fb040c9c9e615792f` |
| Historical ENV-2: immutable port, metric/count/diagnostic and native-conformance laws; its all-21 contract roster is superseded by W16-2 | `b59a8c3f67ef8115097af431f581502c5802dd8d:docs/contracts/env-2.md` | `d9d77b1f5c8bad20c35e9700d12a1466885da7b8cccb45b98ec2ec1082f6c152` |
| Historical ENV-3: producer protocol; string-typed GeneratorVersion reproduction is superseded by the exact peer amendment below | `4d5e8cff0defdbb29366f20f8517f10f16d439a2:docs/contracts/env-3-fleet.md` | `28c590df7926f56b1d1bcc9cd7a3a1284552ad77c256fc92207e08afd700180d` |

### W18/T12 cleared-source precedence amendment

W18-3: existing keyed CUE/schema/vet and exact-file pathcheck suffice. Use them;
no concept checker extension, classifier, CUE-literal discovery, normalizer,
second identity policy, opaque-key grant or parallel inventory is needed.
W18-1/W18-2 supersede the W16 concept-REC prerequisite; the shared proposal is
deferred to REC-6 under its existing owner. Other authored ENV records retain
their existing record-id laws, discovery/effective policy and serial grants.
Concept keys need no REC discovery, numeric policy, opaque grants or RecordOwnership.

The following exact sources have bounded documentary LAND/W2-3 approval in the
supplied combined T12/FIX1/W18 review. That source approval does not approve this
preset amendment. A fresh independent critic and the controller's exact-source
final review remain required. This table overrides historical core/peer shape,
expectation and evaluator text; historical line citations locate conserved laws.
No floating branch, historical roster or old approval supplies authority.

| Bounded normative source | Exact commit and path | File SHA-256 |
|---|---|---|
| env-1-core.md | `24904c52a66ec9703522cca60b4e14fc939f4eea:docs/contracts/env-1-core.md` | `f63a59fe2bfee66b0c102698749eac2a043a99bec5e0cbdfcda17590dfe2f07e` |
| env-1.md | `df851b75fa5aad8c0f5edae1875bc35ce852845d:docs/contracts/env-1.md` | `8509e9f63ac1f24191e7a30144fa57f65fb123cbb53c0b46f0717073c32807cf` |
| env-2.md | `df851b75fa5aad8c0f5edae1875bc35ce852845d:docs/contracts/env-2.md` | `61b2d77e45de9695176628cd8c00bd34dcdf03077de2105177d7548d97e0c753` |
| env-3-fleet.md | `df851b75fa5aad8c0f5edae1875bc35ce852845d:docs/contracts/env-3-fleet.md` | `e31d008bf91572f7f3b7e7ac87660b8f46fbca17dbc58681d7e679bd894534a1` |

The one proposed `calibration/environments/concepts.cue` has container
`inventory: #ConceptInventory & {...}`. Its concepts member is one struct whose
concept identities are evaluated CUE field names. Values carry independent typed
metric/minimum/unit expectations and citations. Go projects those fields into
map[string]ConceptRecord, with no redundant authored key/id; empty inventory JSON
is {"concepts":{},"groups":[]}. No actual inventory contents, keys or path grant
are supplied. Preset selectors are conditional data requirements, never a roster.

Existing CUE semantics makes an NFD source label e\u0301 evaluate to NFC \u00e9.
Compatible declarations may unify; conflicting30/31 content refuses in either
order. Group and requirement reference string values are unchanged: compare them
exactly against decoded keys, without repairing or normalizing them. An NFD
reference does not resolve against the evaluated NFC key. Direct typed Go maps
use exact supplied key/value equality and prove no authored-source provenance.
The stored invalid.cue remains fully commented; tests extract only its exact
commented conflict witness to owned scratch for explicit vetting, never rely on
./... discovery. Other record-id ownership rules are unaffected.

Inventory expectations are independent of measured snapshots. Observations, key
spelling, checker IDs and digests never choose obligations. T12-1 retains default30,
common30/30, cited lower exceptions, explicit history200 and T3-3 aggregate/subtype
law. The unchanged BE-D2 predicates and unknown/null versus measured-zero law
remain mandatory. Missing expectations refuse validation, never silently pass.
Each expected metric is crossed with its applicable native-declared groups.
Missing, extra, duplicate or unrelated metric/group projections refuse; expected
sets cannot shrink when observations are absent. Higher authored minima remain.
The canonical metric/unit/minimum relation belongs only to the pinned core.

Caller flow is exact: validate the one inventory with core schema/vet and the
shared value validator; resolve its bytes and actual Card/checker/catalog refs;
run actual CheckCardClosure with shipped surfaces and approved Cards, then the
pure ValidateEnvironmentConceptJoin with all approved Card.IDs, never titles or
a preset subset. Native owners verify bytes/history, expectations and complete
bindings before projection. PrepareCoverageSnapshot assembles deep-copied measured
values/refusals using the unchanged native CoverageEvidence port. The caller then
passes its independently validated inventory and immutable snapshot to the pure
evaluator, with no port/callback/resolver/I/O or expected-metric copy in the snapshot.
Stale expectation/source binding gives stale_pin; unavailable backing stays
unknown_evidence/null. Existing staged errors, zero-later-call behavior, missing
concept computation, Matching unions and initialized result JSON stay unchanged.
The evaluator checks available bindings without replacing missing_concept with a
full-claim-completeness precheck. Go semantic and native conformance tests remain
future work owned by their existing implementation lanes.

The superseded core approval delivered as `104db845` and earlier W16 peer pins
below are historical provenance only. Full ENV-1 CLI/provenance publication remains
**HELD**. Historical T8 repair LAND cannot clear that hold. Whole T8 remains:
command-private per-call inputs in both CLI forms, exact fake captures/options,
privacy and zero-call assertions; Matching retains Dn(1,30) before Cn(1,30),
positive30/negative1/nominal30. T6 retains sole BEN-9 hashing, inclusive UTC0..30,
immediate relevant-change staleness, ignored workspace materialization, neutral
.invalid fixture identity and the complete fleet protocol. No wire value changes.

Actual ACK7 is LANDED17b1bc703 and was processed once. ENV was excluded from frozen8;
its published gate result proves only frozen8, not these held sources. ACK8 is absent.
ENV remains OUT8; no wave9 assembly or integration grant is inferred. Source approval
is documentary only, with no implementation, native/data/acceptance, admission,
plan-row or C5 grant. All104 ENV boxes remain open. No approval or wave handoff is
performed by this preset author.

### Historical W16 peer-source provenance

The exact older peer pins below are superseded provenance, not normative
alternatives. Their conserved recipe/native laws remain subject to the cleared
W18/T12 source table above.

| Historical peer source | Candidate commit and path | File SHA-256 |
|---|---|---|
| ENV-1 inventory/group and named-version propagation only | `55066828d3893631846a0344ec80f3dee5b4033b:docs/contracts/env-1.md` | `e92f14dd5f27cb77abe317479f38e284f590a8a7e91749664a73da877a358ec1` |
| ENV-2 open inventory/group closure; conserved immutable port and metric/count/diagnostic laws | `55066828d3893631846a0344ec80f3dee5b4033b:docs/contracts/env-2.md` | `64961e035fe2805fc24dd09de59f8df51e1305afbd2a76c1bdf64197835e1386` |
| ENV-3 named-version reproduction; conserved producer protocol | `55066828d3893631846a0344ec80f3dee5b4033b:docs/contracts/env-3-fleet.md` | `f1f28f3d3bae345cc916d758423a71eb9ba483112aa5d9e0fb9046123387b52e` |

The old peer/source review and core-publication receipt cover their exact historical
bytes only; they cannot approve this amendment or clear full ENV-1. Native bytes,
inventory data/path grants and C5 implementation remain unavailable.

The prior preset approval binds `7a48eb6af45cd87b92d5c396cbaa923d26bee997`, file SHA-256
`c4a8793362e6c7bd85baa94a08ecfe687d715d6aed55f200fcd2fc4ac692d919`.
It does not approve these amended bytes. A fresh independent critic is required
for this new user-directed pin/precedence amendment, not another G1 repair round
or a T8-2 exception. No implementation dispatch or plan-row claim is granted.

Historical provenance only: the draft's old core pin
`cccb365ae4479c6ceec3c7c57ced83c663a79789:docs/contracts/env-1-core.md`,
SHA-256 `1e965b2fe8d73c5a9ded7d7a5a0bec7c05530964be215e676196388681863b83`,
is not the normative core dependency. Its fixed vocabulary cannot constrain a
reviewed inventory data addition. These are different bytes and different source
identities, not a relabelling of the old pin as W16.

Approval provenance: W2-3, W3-2/W3-4, W18-1/2/3, T12-1 and T3/T6/T8
govern the bounded amendment above. Its exact four-file source received independent
LAND; this two-preset alignment still needs its own fresh critic. Historical full
ENV-1 HOLD and later T8 repair clearances remain separate. No peer bytes, production
inventory or shared REC proposal change here. Superseded pins remain provenance.

| Decision | Bounded choice | Governing text |
|---|---|---|
| ENV-G2/D1 | Data only for `false-plan` and `stale-rationale`. Other concepts remain unclaimed. | ENV-G2 plan row; BE-D2 false-plan and stale-instructions/rationale rows |
| ENV-G2/D2 | Existing FleetManifest, SeedInput, source references and immutable native port. No alternate loader API, card or identity registry. | Approved core, Fleet data declarations and Request declarations; ENV-1, Fake registry seam; T3-1/T3-2/T3-4 |
| ENV-G2/D3 | At least 30 false/30 true,30 stale/30 live,30 governing links, plus common30/30 independently per selected full stratum. | BE-D2; ENV-2, Exact metric vocabulary and Counting; T3-3 |
| ENV-G2/D4 | Pin independently checked seed truth, compiler/test/entrypoint evidence and record anchors before ingest. Never obtain truth from the index. | BE-D2/BE-D5; ENV-2, Trust boundary; ADR-085 |
| ENV-G2/D5 | Seed7/8 are replicas. Only independently verified different templates/topologies can fill two environments. No padding as tasks or public/synthetic pooling. | BE-D1/BE-D3/BE-D4/BE-D5; ENV-2, IndependentEnvironments |
| ENV-G2/D6 | Other authored ENV records retain shared record checker/effective policy and REC serial grants; concept keys use schema/vet and exact-file pathcheck. No new registry, allocated record ID or exception. | [REC-1](rec-1.md), Shared checker prerequisite and allocation; [records integration](records-integration.md), Exact ownership sequence; W2-2; W18-1/2/3 |
| ENV-G2/D7 | Generated repositories, truth and all proof roots are workspace-only and uncommitted, with the approved neutral Git identity. | T6-2; ENV-3, Workspace and Commit payload |
| ENV-G2/D8 | Use keyed ConceptRecord values with independent typed expectations; read one cited inventory and preserve the chosen two-concept preset scope as data. Resolve actual Card bytes, reuse CheckCardClosure, then the pure concept join. No fixed universal roster. | W18-1/2/3; T12-1; W16-2; T3-1; BE-D2/BE-D9; amended core, Concept record shape and single inventory / Read-only Card.ID and group join |
| ENV-G2/D9 | All member licence receipts coexist with native role outputs in Receipts; Licence is the separate complete inventory. Freeze finite native-owner binding requests and exact refusal assertions, with no new native envelope. | BE-D9; amended core, Representation rules (member licences); frozen ENV-2, Trust boundary points 1/2; T3-2 |

The recipe and path spellings below specialize these governed choices.
The native output paths are requests to their owner, never grants or new truth.
Missing bytes, pins, approved card selectors and adapter conformance remain
execution dependencies; they supply neither owner grants nor admission waivers.

## Exact later data ownership

This is a future implementing data lane's **NEW-file list**, not permission for
this author to create those files. Prefix `G` in this section means exactly
`calibration/environments/fleet/governance/`. No directory wildcard is granted.
The two template families are candidate development environments, not a sealed
holdout or a declaration of independent ancestry.

| Exact new path relative to G | Contents and ownership limit |
|---|---|
| `recipe.md` | Human-readable frozen recipe, source identity inventory, required native bindings and expected relationships in this contract. No status field or second registry. |
| `links.answer.json` | Exact unchanged fauxtown key identified below. Copy raw bytes, never parse/re-marshal. Not governance truth. |
| `direct/seed-7.cue` | Full existing EnvironmentRequest/SeedInput for direct seed7, only after all required bytes/pins exist. |
| `direct/seed-8.cue` | Same direct recipe at seed8 with independently updated expansion digests, a replica. |
| `direct/sources/module.txt` | Raw `go.mod` source bytes. |
| `direct/sources/main.txt` | Raw `cmd/app/main.go` fixed direct-entry source. |
| `direct/sources/policy.txt` | Raw `policy/policy.go` source for independent task sites. |
| `direct/sources/policy-base.txt` | Raw c0 `policy/policy.go`, preserving the independently frozen prior behavior. |
| `direct/sources/policy-test.txt` | Raw `policy/policy_test.go` native subject tests. Fixture data, not an ENV checker. |
| `direct/sources/policy-test-base.txt` | Raw c0 `policy/policy_test.go` tests for that prior behavior. |
| `direct/sources/plan.txt` | Raw `docs/plans/governance.md` plan grammar v2 rows. |
| `direct/sources/instructions.txt` | Raw `docs/instructions.md` anchored stale/live instructions. |
| `direct/sources/records.txt` | Raw `docs/decisions.md` independently grounded written rationales and governing records. |
| `direct/sources/records-base.txt` | Raw c0 `docs/decisions.md`, with prior definitions and exact supersession bindings retained. |
| `direct/truth/native.json` | Unmodified operator-placed native seed-key/receipt bytes in the accepted native owner's existing format. No authored substitute JSON shape. |
| `direct/expected.json` | Exact existing FleetResult JSON expected from fully frozen inputs. No made-up native counts, coverage or admission status. |
| `split/seed-7.cue` | Full existing EnvironmentRequest/SeedInput for split seed7, subject to the same dependencies. |
| `split/seed-8.cue` | Same split recipe at seed8, independently updated digests, a replica. |
| `split/sources/module.txt` | Raw `go.mod` source bytes. |
| `split/sources/main.txt` | Raw `cmd/app/main.go` explicit finite entry source. |
| `split/sources/dispatch.txt` | Raw `internal/dispatch/dispatch.go` fixed dispatch source. |
| `split/sources/policy.txt` | Raw `internal/policy/policy.go` independently authored task sites. |
| `split/sources/policy-base.txt` | Raw c0 `internal/policy/policy.go` prior behavior. |
| `split/sources/policy-test.txt` | Raw `internal/policy/policy_test.go` native subject tests. |
| `split/sources/policy-test-base.txt` | Raw c0 `internal/policy/policy_test.go` prior-behavior tests. |
| `split/sources/plan.txt` | Raw `docs/plans/governance.md` rows. |
| `split/sources/instructions.txt` | Raw `docs/instructions.md` anchored stale/live instructions. |
| `split/sources/records.txt` | Raw `docs/decisions.md` grounded rationales and governing records. |
| `split/sources/records-base.txt` | Raw c0 `docs/decisions.md` prior governing definitions/bindings. |
| `split/truth/native.json` | Unmodified operator-placed native key/receipt in that same accepted native format. |
| `split/expected.json` | Exact existing FleetResult JSON, issued only after its frozen input bytes exist. |

Two further exact new paths are
`docs/bench/readings/environments/seed-governance.md` and
`docs/bench/readings/environments/seed-governance.json`. Only a later released
reading lane may create them from actual ENV-C5 output. It must retain refused,
unknown and missing prerequisites and distinguish local cell counts from overall
portfolio sufficiency. This draft creates neither file and assigns no verdict.

No existing file, shared card/mapping/catalog, schema, allocator, checker,
generator, CLI, native adapter, plan checkbox or changelog is owned here. These
paths do not grant native tooling or record creation. If the native owner needs
additional receipt/fixture files or a different raw format, request an exact
manifest/grant upstream before adding paths. No catch-all source directory,
central loader or registry is authorized.

## Existing APIs and loader boundary

The following signatures and the SeedInput block are reproduced unchanged from
the frozen approved sources. They are specified APIs, absent at this base, not
new declarations or evidence that they compile. Schema types remain solely in
`schema/benchenv`; derived coverage/port types solely in `internal/benchenv`.

```go
func ValidateEnvironmentRecords(source []byte) (EnvironmentRecords, error)
func ValidateFleetManifest(source []byte) (FleetManifest, error)
func ValidateEnvironmentRequest(source []byte) (EnvironmentRequest, error)
```

```go
type SeedInput struct {
    Manifest FleetManifest `json:"manifest"`
    Sources []Ref `json:"sources"`
    Limits FleetLimits `json:"limits"`
    Requirements []Requirement `json:"requirements"`
    Catalog Ref `json:"catalog"`
}
```

```go
func GenerateBenchmarkFleet(ctx context.Context, root string,
    manifest benchenv.FleetManifest, sources map[string][]byte,
    limits benchenv.FleetLimits) (benchenv.FleetResult, error)
```

```go
type CoverageEvidence interface {
    Catalog(ref env.Ref) (CatalogProjection, error)
    Environment(head env.EnvironmentHead) (EnvironmentProjection, error)
    Concept(head env.EnvironmentHead, requirement env.Requirement,
        witnesses []env.Witness, at time.Time) (ConceptProjection, error)
}

func CheckEnvironmentCoverage(inventory env.ConceptInventory, snapshot CoverageSnapshot) (CoverageResult, error)
```

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

CoverageEvidence remains the preparation port; the evaluator takes exactly the
independent inventory and immutable measured snapshot, as the caller flow above.

```go
type environmentInputLoader func(context.Context, environmentInvocation, *benchenv.EnvironmentReceipt) (env.EnvironmentRequest, environmentInvocation, error)
func (r *environmentRegistry) registerInput(mode env.EnvironmentMode, load environmentInputLoader) error
```

ENV-C5 owns `cmd/scenariobench/environment_seed.go` and
`environment_seed_test.go`, `runEnvironmentSeed`, and registration of its
mode-owned loader/handler through ENV-C0's existing private registry. The public
proof entrypoint is only, after ENV-C5 lands:

```sh
go run ./cmd/scenariobench env-check -seed governance
```

That invocation is not run here. SeedInput has no native-path, Sources map,
EnvironmentRecords, projection, chosen card or pass field to fill in. Sources
are exact existing Ref values, not a new bundle API. C5 must verify their bytes
and bind their IDs to the producer's `sources[ContentID]`, preserving its explicit
private per-call sourceRoot/outputRoot handoff. Missing bytes must not fall back
to a template registry, ambient path, environment, download or fabricated key.
The data lane cannot implement this join. Structured and flag forms retain the
approved ENV-1 validation, privacy, ModeUnavailable and error-precedence laws.

C5 must prepare existing EnvironmentRecords/EnvironmentHead, Requirement,
Witness and CoverageClaim inputs from the approved card/mapping plus generated
FleetResult and verified native snapshot. It cannot add those fields to
SeedInput. Its request to the leaf uses the existing CoverageRequest with
Records, Catalog, TierConstructed, PurposeCoverage and fixed supplied At. BEN-5
admission remains downstream. Coverage is never its own admission prerequisite.

Requirement.Capability is an exact approved CTR-13 Card.ID, not `governance`, a
title, tool name or fabricated alias. Requirement.Card and CheckerContract are
different exact Ref values. Unrelated deferred targets may be in catalog closure
without coverage claims or new implementations.

### Single authoritative inventory and read-only owner seam

W16-2 replaces the old fixed universal 21-key contract roster with open keys in
one authoritative DATA input, proposed exactly
`calibration/environments/concepts.cue`, container
`inventory: #ConceptInventory & {...}`. The path is an upstream proposal, not
an ENV-G2-owned file or a current owner grant. No contents, production keys,
card IDs or group assignments are supplied here. Every key must cite the exact
capability Card or BE-D2/BE-D9 design row requiring it; a new cited key is a
critic-reviewed data change, never a schema/contract enum amendment.

The two chosen selectors `false-plan` and `stale-rationale` remain this preset's
data scope. They are not a complete catalog or schema vocabulary. They may be
used in executable requests only after the authoritative inventory actually
contains and justifies them and the exact Card/checker-contract groups select
them. Do not create a preset-local registry, copied maintained inventory or
fallback list. An additional required group concept stays missing/unavailable;
this preset cannot trim catalog closure or invent that concept's fixtures.

The following exact declarations are reproduced from the cleared core. Metric,
MetricUnit and MinimumCitation are the core's closed vocabularies; the one canonical
metric/unit/minimum relation is defined there, never inferred from observations.
The schema owns these data, not this preset or the leaf. Default omission emits30;
Go Minimum0 refuses. A lower minimum needs its exact BE-D2 citation and floor;
history requires explicit200. Aggregate output presence1 cannot satisfy an
explicitly claimed subtype's selected metrics30 or lower common30/30.

```go
type MetricExpectation struct {
    Metric Metric `json:"metric"`
    Minimum int64 `json:"minimum"`
    Unit MetricUnit `json:"unit"`
    LowerMinimum MinimumCitation `json:"lower_minimum,omitempty"`
}
```

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

Evaluated field identities, exact unchanged reference values and the complete
caller flow are governed above. Empty Concepts is a non-nil map, Groups a non-nil
slice, never null. Map keys traverse in UTF-8 byte order; pointers escape the
actual decoded key as RFC6901, not a source-array index. Repeated compatible CUE
fields may unify, while conflicting concrete content refuses before decode.
Shape/local join success proves no semantic citation justification or native truth.

Other authored ENV records still need their own U1 checker/discovery/effective
policy/grants. Those prerequisites do not apply to concept fields or their exact
inventory path. No shared proposal, second inventory or new checker is authored.

### Named generator version, same wire value

The exact amended schema-owned declaration is:

```go
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
```

Its exact existing CUE manifest excerpt is:

```cue
#GeneratorVersion: "env-fleet/v1"
#FleetManifest: close({
    seed!: int & >=0 & <=18446744073709551615
    generator_version!: #GeneratorVersion, template!: #Text, size!: #FleetSize
    repositories!: [#FleetRepository, ...#FleetRepository]
    truth!: [#FleetFile, ...#FleetFile]
})
```

Use `benchenv.GeneratorVersionV1`, not a new version or bare exported string
type. The generator_version JSON key and env-fleet/v1 wire value are unchanged.
SeedInput's nested manifest uses the same #GeneratorVersion. The exact peer
candidate's WR-CORE-FLEET reproduction now carries this named type and refreshed
source/block/file hashes; its older string reproduction is historical only.
The cleared source has bounded documentary approval; this preset
still needs fresh independent review and exact-source final integration review.
GenerateBenchmarkFleet's signature above stays unchanged; the generator never
copies or aliases schema declarations locally.

### Exact amended-source block receipts

For each row, hash the UTF-8 bytes **between** fences, with LF and one final LF,
excluding the fences. These are document block identities, not BEN-9 fingerprints
or runtime receipts. The new tables distinguish exact core excerpts, the peer snapshot excerpt and
historical receipts; hashes never select metric obligations.

| Cleared reproduced block | SHA-256 |
|---|---|
| MetricExpectation Go declarations | `26a16eb44dbc322b68f2fd4b1812ce33a45e1957275bdfacc0a2ecba7d558934` |
| Concept Go declarations | `50129617bf6afe99621f54b54b720c82141eb0ea3ee2ba6b08f7831a0a979207` |
| Concept CUE declarations | `a1d36d42546c3bdbdf4f3a75c65e8f3590e338848cc9417f6b54f951a4e8ba5e` |
| Measured snapshot Go declarations (ENV-2) | `da51239c8136b869b3ae08a8214e7217c2b08fb7e5ddade5a5928a5a18713981` |

GeneratorVersion receipts below remain unchanged. The two old concept block
receipts below identify historical W16 excerpts only, superseded by the table
above; their list/key identity is not normative. Snapshot bytes are from the
cleared ENV-2 peer, not the core. All other listed excerpts are core substrings.

| Historical concept / unchanged version block | SHA-256 |
|---|---|
| Concept Go declarations | `ab17e19af88615a8d9da799424c436e8cfcee9c2c913efe68a1639127cb2edc7` |
| Concept CUE declarations | `e5ac464e08eb589919baa3d76da801ea731cdcab2f3508825e704fc1cfe21bda` |
| GeneratorVersion and FleetManifest Go declarations | `4645fb472fc9797bff131da6e85a11583a38fe34f465315a10aeb29fea8ee5f9` |
| GeneratorVersion CUE manifest excerpt | `f9f422aa093fd5b38fcd7b67ccbc02ec22cc904f17f0711ff44cea5f4ffea39b` |

## Minimum current recipe and identities

Use `GeneratorVersion=benchenv.GeneratorVersionV1` (wire `"env-fleet/v1"`). Direct has Template
`governance-direct-v1`, one repository ID `governance-direct`; split has Template
`governance-split-v1`, one repository ID `governance-split`. Each has complete
snapshots `c0` then `c1`, Parents `[]` then `["c0"]`, Head `c1`. These are
producer-local IDs, not allocated record identifiers or native origin keys.
Changing them cannot manufacture lineage independence. Sources, results and
errors use the approved closed types without additions.

Both candidates use seeds **7 and 8**. For each family, seed7 twice in fresh
owned ignored workspace roots must yield identical checkout/truth bytes, modes,
Git IDs/DAG and canonical FleetResult JSON. Reversed successful input set order
has the same result. Seed8 changes only explicit `@ENV_SEED@` expansions and
their dependent digests, trees and commits. It does not change task identity,
polarity, matches, governing record relations or normalized origins. Nonseeded
key and fixture bytes stay identical. A seed-only variant never adds an
environment or independent task. No PRNG, default seed or environment lookup.

Every raw source map key is fixed: `module`, `main`, `policy`, `policy-test`,
`policy-base`, `policy-test-base`, `plan`, `instructions`, `records`,
`records-base`, `native`, `links`; split additionally uses `dispatch`.
They bind the exact raw files in the ownership table, with `links`
bound to G/links.answer.json. Map keys do not prove truth. Each FleetFile must
have actual SourceSHA256, actual post-expansion SHA256, explicit Mode, Class
and Seeded. Mode is 100644 for this recipe. Seeded=true is restricted to an
explicit seed marker in main source; every other listed file is Seeded=false.
The producer replaces all exact tokens and requires the independently supplied
digests; no normalization or second expansion engine belongs to this lane.

In each complete snapshot, module is `go.mod`/build; main and policy are the
product paths listed above; split dispatch is also product. policy-test is test.
Plan, instructions and records are generated fixture documentation and carry
class generated, not product padding. c0 and c1 each declare every file, even
unchanged ones. At c0, policy, policy-test and records paths use their respective
`-base` ContentIDs; at c1 they use the unsuffixed ContentIDs. All other paths
reuse their named ContentID in both snapshots. Native owners must freeze the
actual behavioral/governing transition, not merely edit a timestamp or heading.
Native truth and the unchanged link key are external truth,
not files in any generated repository. In Manifest.Truth use respectively
Path `governance/native.json`, ContentID `native`, and Path `links.answer.json`,
ContentID `links`, both mode100644, class generated, Seeded=false. FleetResult
then has `truth/governance/native.json` and `truth/links.answer.json` Ref paths.
Do not confuse those producer-root paths with source-root Ref paths in SeedInput.

Every generated source/plan/instruction anchor is bound to the returned full
repository commit for the selected snapshot, exact slash path and inclusive
positive line interval. A name, line guessed from this recipe or logical c1
is not a RepositoryPin.Commit. No generated commit is invented in this draft.
Source-native truth references must bind both snapshots, actual raw receipt
bytes, compiler/test/entrypoint pins, record definitions and anchored scopes.
They must exist and be independently verified before immutable projection or
ingest. The native key is NOT derived from an ENV coverage result or index.
The same nonseeded native.json must explicitly bind the seed7 AND seed8 product
commits/anchors and preserve the shared task origins. Missing either binding
blocks that replica; do not reuse a seed7 commit as seed8 evidence. Native
owners prepare and verify those exact inputs independently, then C5 rechecks
them against the actual producer result before projection. Keep content refs
acyclic: raw product/record/tool/receipt leaves first, native key referencing
those leaves/commit identities, seed manifest referencing the key, then derived
ENV records/receipts. A native key never contains its own digest or the digest
of a manifest/receipt that already includes it. No self-referential placeholder
hash or second canonicalizer can repair such a cycle.

For each candidate the known recipe shape is one repository/two reachable
declared commits. Direct has two HEAD product files, split three. Product line
counts and excluded line counts require the actual unavailable raw sources.
No guessed size or zero replaces them. Limits have no default: freeze exact
positive MaxRepositories/MaxCommits/MaxFiles/MaxBytes against all expanded full
snapshots plus truth using ENV-3's accounting before any runnable seed file is
issued. Git encoding overhead is excluded. Repeated snapshots count again.
Size equality, cap exhaustion, overflow and error order remain ENV-3's tests,
not a new data-lane hasher/counter. BEN-9 alone owns fingerprint canonicalization.
The known entry arithmetic is direct 7 files per snapshot x2 +2 truth=16;
split 8 x2 +2=18. Equality caps for repository/commit/file entries are therefore
1/2/16 and 1/2/18 respectively; the byte cap is unavailable until raw and expanded
bytes are frozen. HEAD excluded file counts are generated 3/test 1/build 1/vendored 0
for both; excluded line counts are still unavailable. These shape calculations
are not measured source size or a materialization result.

Candidate coverage is deliberately narrow: Go, aggregate Subtype `""`,
repositories/S only. Neither line/file/history classes nor another language is
claimed. Two commits are below history/S. Each prediction selector must be
approved by the exact card/checker contract before issuing a Requirement or
CoverageClaim. No effect/null label is inferred from this document. For every
selected full tuple (language, scale vector, prediction, subtype), provision its
own disjoint populations below. If the supplied card also requires a null,
subtype or wider concept stratum, the absent population remains unavailable;
do not borrow counts, narrow the card, add another claimed concept or silently
expand this file grant. Request its exact data amendment first.

Direct and split are candidate different product/governance topologies: fixed
direct entry calls versus a fixed finite dispatcher in a separate package.
They require independently authored task bodies, keys, templates and verified
ancestry. A rearranged copy of direct is not split. Native Family/Root evidence
must establish whether they are independent; their two names are not evidence.
If they share template/clone ancestry, ENV-2 counts one environment. This draft
claims neither two eligible environments nor a populated stratum. No public or
smoke input may repair a constructed shortage. Holdout sealing is ENV-C7/S2's
separate work, not permission from a second seed or this development preset.

### Task populations, independent truth and rationale

The minimum per selected stratum PER candidate is the following recipe, not
observed counts. An ordinal is a locator, never an independence proof. Native
owners must freeze exact task-specific behavior, source/record anchors and raw
truth before generation; thirty renamed copies of one task do not satisfy it.

The following 30 slots freeze the subject behaviors and governing transitions.
They are fixture requirements, not an authored native answer key. Each cell
states c0 rule -> c1 rule. Direct and split have different task bodies and
governing topics; do not generate split by moving/renaming direct's functions.
Use only local standard-library subject code, with no external service, time,
locale, network, dynamic loading or dependency. Numeric inputs are bounded by
the specified fixture cases, not host int width. Native owners bind concrete
input/output boundary cases and real receipts to these requirements before
claiming a site. An inability to verify a slot leaves that slot unavailable.

| Slot | Direct task and governing transition | Split task and governing transition |
|---|---|---|
|01 | Leading/trailing ASCII space: preserve -> trim. | Slash-separated path: preserve repeated separators -> collapse them. |
|02 | ASCII letters: preserve case -> lower case. | Backslash path separators: preserve -> convert to slash. |
|03 | Decimal integer text: allow leading plus -> reject leading plus. | Binary digits: allow empty input -> reject empty input. |
|04 | Port text: allow zero -> reject zero, retain1..65535. | Percentage text: allow values above100 -> reject them, retain0..100. |
|05 | Comma list: preserve empty items -> omit empty items. | Semicolon list: preserve duplicates -> stable first-occurrence uniqueness. |
|06 | Empty string: treat as present -> reject missing value. | Tab-only field: treat as nonempty -> reject whitespace-only value. |
|07 | Suffix match: case sensitive -> ASCII case insensitive. | Prefix match: accept an empty prefix -> require a nonempty prefix. |
|08 | Key ordering: descending -> ascending byte order. | Stable partition: nonempty values last -> nonempty values first. |
|09 | LF-terminated text: preserve final LF -> remove exactly one final LF. | CRLF text: preserve CRLF -> normalize CRLF to LF. |
|10 | Decimal fraction: truncate two digits -> round half away from zero to two digits. | Integer division: truncate quotient -> return quotient and remainder. |
|11 | URL query value: preserve literal space -> percent-encode space. | HTML text: preserve angle brackets -> escape them. |
|12 | Base64 text: accept unpadded input -> require canonical padding. | Hex text: accept odd digit count -> reject odd digit count. |
|13 | UTF-8 text length: count bytes -> count Unicode code points. | ASCII control bytes: accept -> reject bytes below32 except LF. |
|14 | CSV field: leave embedded comma unquoted -> quote and double embedded quotes. | Tab-delimited field: preserve embedded tab -> reject embedded tab. |
|15 | Path extension: match `.go` only -> also accept uppercase `.GO`. | Filename validation: permit slash -> reject any slash. |
|16 | Signed decimal: accept surrounding spaces -> reject surrounding spaces. | Timestamp text: accept missing zone -> require an explicit numeric zone or Z. |
|17 | Boolean text: accept `1`/`0` -> accept only `true`/`false`. | Positive-count text: accept negative sign -> reject negatives and retain zero. |
|18 | Repeated key assignment: last value wins -> first value wins. | Unknown named field: ignore -> return an explicit error. |
|19 | Line prefix: remove first occurrence anywhere -> remove only at line start. | Line suffix: remove first occurrence anywhere -> remove only at line end. |
|20 | Nonempty substring: replace first occurrence -> replace all occurrences. | Repeated delimiter: split every occurrence -> split only the first occurrence. |
|21 | Range endpoint: upper bound exclusive -> inclusive. | Slice window endpoint: end inclusive -> end exclusive. |
|22 | Text palindrome: compare case sensitive -> ignore ASCII case. | Adjacent duplicate text items: preserve -> coalesce runs only. |
|23 | Tab expansion: retain tabs -> expand each tab to four spaces. | Space indentation: preserve -> remove exactly one two-space leading unit. |
|24 | Empty list join: return one delimiter -> return empty string. | Delimiter-only input: emit empty fields -> emit an empty list. |
|25 | Signed zero text: preserve `-0` -> canonicalize to `0`. | Decimal leading zeros: preserve -> canonicalize without leading zeros. |
|26 | Line endings: accept lone CR -> reject lone CR, retain LF/CRLF. | NUL-containing value: accept -> reject any NUL byte. |
|27 | Key equality: trim both sides -> compare exact bytes. | Membership: compare exact bytes -> trim outer ASCII space on the query only. |
|28 | Bounded decimal count: clamp values above9 -> reject values above9. | Fixed-width decimal code: left-pad shorter codes -> require exactly four digits. |
|29 | Trailing delimiter: always append -> append only when absent. | Leading delimiter: always prepend -> prepend only when absent. |
|30 | Last-field extraction: choose first colon -> choose last colon. | First-field extraction: choose last slash -> choose first slash. |

For each slot NN, plan targets are exactly `CaseNNFalse` and `CaseNNTrue` in
the family's policy package, with package-qualified annotations on the same
checkbox line. Both rows are claimed fixture rows. At slots01..15 the false
target is absent from every product and test definition, independently probed
by the compiler in native-owner workspace scratch. At slots16..30 it exists
and is directly unit-tested but absent from the complete product entry routes.
Every true target exists, is directly unit-tested and is called by the product
route `case-NN`. The fixed finite product router has exactly those 30 true
routes, an explicit error for every other route, and no reflection, plugin,
callback registration or alternative entry path. The split router delegates
through internal/dispatch; direct routes call policy functions directly.
This finite source contract makes complete entrypoint evidence possible; a
coverage log from some routes alone is still insufficient proof.

Each slot also has two disjoint instruction source anchors at c1: stale text
still directs the c0 rule in that table cell, live text directs the c1 rule.
Each binds its actual governing record/definition and applicable policy symbol,
with a written reason explaining the precise rule transition. The governing
link metric uses the 30 live instruction-to-record relations, one per slot;
stale relations retain their full evidence too and cannot be relabelled live.
Each instruction pair has one independently verified Match, separate from its
plan pair. Slot numbers, function names and route strings are not record IDs,
verified origin keys, allocations or gold labels. The native owner must verify
actual scoped records and distinct site ancestry rather than trusting this table.

| Population | Frozen minimum and pairing | Independent evidence required |
|---|---|---|
| False plan positives |30 distinct claimed false rows.15 target an absent symbol,15 an existing tested function not reached by any declared product entry. | Fixed seed manifest; native compiler symbol resolution/failed absent-target probes; native subject tests; complete fixed product entry/route evidence for unwired sites. |
| True plan negatives |30 distinct true rows, one safe partner for each false row under verified Match. Each target exists, is reached by its native test and product entry, and meets its actual row contract. | Independently bound compiler, passing subject-test and fixed-entry evidence. An unchecked row or merely open checkbox is not proof of true. |
| Stale instruction positives |30 distinct anchored instructions whose stated scoped rule contradicts the governing record at c1, with the exact prior c0 relation retained. | Independently frozen seed key with both instruction and governing record anchors, revision/supersession and scope evidence. |
| Live instruction negatives |30 distinct live anchored instructions, one matched safe partner per stale instruction, whose scoped directive agrees with its explicit governing record at c1. | Same source/record verification; known age or unchanged wording alone is insufficient. |
| Written governing links |30 distinct explicit instruction-to-record links with written rationale, independently normalized as30 governing-link members. | Exact source anchor, record identity/definition anchor, scope and written reason, all pinned to bytes and history. A mention, unresolved fragment or duplicate link is not a member. |

Both the false and true cohorts use real grammar-v2 rows with contracts on the
checkbox line. Tests target the named function and exact package where needed.
No library waiver, skip_test, test-only wrapper or absent contract makes a false
claim true. Existing verifier behavior for an unwired function is library-only /
Unknown and strict refusal, not necessarily Fail. Seed truth calls the claim
false on independent product-wiring evidence; it never relabels that product
verdict or uses it as the oracle. Compiler existence alone cannot prove wiring.
A successful build of a repository lacking an absent symbol is not the required
negative symbol-resolution probe. Partial execution does not prove absence of
all product paths. Missing complete entrypoint evidence is unknown_evidence.

Staleness is a semantic source/record relationship, not a birthday, changed
prose or stale index. At c1 both old and new instructions must be readable at
their exact current source anchors. The stale member retains its prior binding
and the now-conflicting governing record; the live partner binds the still
applicable rule. Fixed native truth must reject reversed labels, a record outside
scope, a link to the wrong revision, a moved anchor without repinning and a
missing reason. An unrelated prose change, identical age or seed marker change
alone must leave labels unchanged. These are native fixture obligations under
the existing stale-rationale concept, not a new natural-language oracle.

Use existing record identities only when they actually identify those source
records. A borrowed real ADR number whose content does not govern the fixture
cannot be a governing link. No fabricated ADR, reserved range, auto-allocation
or new grandfather exception is supplied here. If fixture records need new
recognized IDs, obtain REC's effective validated inventory, integrator grants
and serialized allocation first. Unavailable grants block records/source/key
freezing. Do not solve that dependency with unrecognized aliases in a second
registry. Local opaque FleetFile/Match locators never substitute for record IDs.

## Finite native receipt bindings and member licences

The recovered draft did not freeze Head.Receipts membership. The analogous
representation failure is prevented here by an exhaustive binding request,
not by importing another preset's graph layout. Under BE-D9, amended core's
Representation rules and frozen ENV-2's Trust boundary point 1, inventory and
all member receipts must be independently resolvable alongside native outputs.

For this request F is exactly `direct` or `split`, S exactly `7` or `8`, and
K exactly `licence-inventory`, `member-licence`, `source-policy`, `scale`,
`lineage`, `false-plan` or `stale-rationale`. Request precisely these **28**
existing-format native output leaves:
`workspace/artifacts/benchenv/governance/{F}/seed-{S}/{K}.raw`.
This finite expansion fixes paths, not any unavailable bytes, digest, native
format or approved Ref.ID. The native snapshot preparation owner must accept
this exact output/verification manifest under its existing owner seam before
use. It is not permission to add a native package, adapter, checker, driver,
wrapper envelope or tracked file. Unsupported incumbent formats block DEP3;
return for an exact owner grant amendment instead of inventing serialization.

Each Ref uses the corresponding canonical expanded path and whole-file SHA-256;
its actual ID comes from that owner's accepted immutable manifest and must
select exactly that artifact. Logical K names here are roles, not allocated
record IDs or invented native keys. Different roles cannot alias IDs, paths or
payloads to hide an unavailable output. Reuse in the specific fields below is
intentional. Each F/S head selects only its own S leaves; native.json retains
both seeds' bindings as already required. The source-owned native.json remains
one unmodified incumbent-format key, not a new ENV envelope collecting payloads.
It references independently verified raw leaves without containing its own hash.

| Role K | Binding into existing types; native-owner obligation |
|---|---|
| licence-inventory | EnvironmentHead.Licence only: complete inventory of this head's repository member and applicable source/dependency/toolchain licence facts. Not a borrowed per-member receipt. |
| member-licence | In EnvironmentHead.Receipts: independently verified receipt for the sole repository `governance-direct` or `governance-split`, at this seed's exact returned full commit, covering its applicable inventory licence facts. Not the inventory or a compiler output. |
| source-policy | EnvironmentHead.SourcePolicy and in Receipts: exact source/exclusion policy bytes verified against this head. |
| scale | EnvironmentHead.Scale.Receipt and in Receipts: measured product/exclusion counts and reachable history for this head and source policy. No guessed lines/bytes or change to the repository/S-only claim. |
| lineage | In Receipts and existing Witness.Origin resolution: independent template/history/clone/matched-pair evidence for both selected concepts; no seed-derived independent origin. |
| false-plan | In Receipts and existing false-plan ConceptProjection.Receipts/Witness.Receipt resolution: independent compiler/subject-test/complete-entrypoint evidence for the fixed false/true rows, not index truth. |
| stale-rationale | In Receipts and existing stale-rationale ConceptProjection.Receipts/Witness.Receipt resolution: independently verified source/record scope, revision, rationale and governing-link evidence. |

Thus each Head.Receipts has exactly six distinct refs in recipe order:
member-licence, source-policy, scale, lineage, false-plan, stale-rationale.
Licence is the seventh, distinct inventory ref, not a seventh Receipts entry
or a substitute for member-licence. Existing Oracles and checker-contract refs
continue identifying their actual approved oracle/contract bytes; no receipt
is relabelled as a card or oracle definition. This is a finite recipe choice,
not a restriction on the core's general Receipts collection.

Each candidate has one repository member, no image member (Images=[]), and its
pinned toolchain/dependencies as enumerated by the actual inventory. The native
owner verifies that complete membership and licence coverage; a newly discovered
member or a requirement for additional separate licence receipts needs an exact
binding/grant amendment. Do not silently omit it or borrow another role's output.
The inventory, native format, applicable licences and every output's real pin,
bytes, ID, digest and independent verification are unavailable DEP3 inputs.
A path spelling or this role table is not a licence or native assertion.

All 28 requested leaves stay ignored and uncommitted. They are upstream native
outputs, not extra FleetManifest.Truth members or generated repository files;
the 16/18 entry arithmetic and tracked data-file list above are unchanged.
The acyclic resolution order is product/source/tool/record leaves and actual
Git commits, then these native outputs, then the existing native key referencing
them, then seed manifests/derived ENV records. A licence/native output cannot
hash a head, seed manifest or key that already references it. Actual reference
resolution and count are still outside this data author's authority.

### Exact receipt assertions in existing owner tests

These names are subcases in already requested tests, not new test files/APIs.
Run each independently for F/S with the other eligible family healthy, exact
At=2026-11-15T00:00:00Z, aggregate Go/repositories-S and actual approved prediction/
Card/claim/requirement refs. Assert complete sorted result JSON, all global,
cell and requirement issues, initialized collections and nulls, never a substring.
The exact diagnostic grammar is frozen ENV-2, Exact diagnostic grammar. The
following tuples identify its changed cell and precise refused Ref; no new code,
metric or diagnostic slot is introduced.

| Exact existing test/subcase | Exact assertion |
|---|---|
| TestEnvironmentSeed/ENV-G2/LicenceHealthy | For each F/S the loader resolves the seven distinct roles; Licence equals licence-inventory and Receipts equals the six refs in the exact order above. Native validation binds the sole member/full commit plus all inventory toolchain/dependency licence facts. Both concepts retain known common30/30; absent_symbols15, unwired_entries15, stale_instructions30, live_instructions30, governing_links30. Two independently verified healthy families give PASS and an empty issue union; seed7/8 replicas together still count one environment and give insufficient_environments1/2, class untested. Assert all generated files/truth/digests conserve the existing recipe. This is transport/native/coverage composition, never an admission assertion. |
| TestEnvironmentCoverage_TrustedBoundary/ENV-G2/LicenceHealthy | With the same accepted independently prepared immutable fixture, each head's Licence, all six Receipts and measured Scale resolve exactly; common and supplementary counts above survive. The fake leaf test proves joins/counts only; source/raw verification remains native conformance. Self-labelled inventory/receipt bytes alone cannot pass. |
| TestEnvironmentCoverage_StalePin/ENV-G2/LicenceMissing | Independently remove or make unreadable licence-inventory or member-licence bytes, keeping every other role healthy. Return R(unknown_evidence,p) at the exact inventory/member path below. Both affected requirement and cell counts are null, never measured zero or retained common30/30. Other family's cell stays healthy; global INCOMPLETE/nil with N(repositories,S,1) and the affected cell's binding issue. No missing_count is invented. |
| TestEnvironmentCoverage_StalePin/ENV-G2/LicenceStale | Keep the selected Ref.ID stable; independently change its Path, SHA256, actual bound repository commit, raw member identity, or applicable toolchain/dependency binding against the original snapshot. Return R(stale_pin,p) at the exact changed binding path below, no affected count. The healthy family stays eligible; global INCOMPLETE/nil with N(repositories,S,1). Cross-role substitution is a changed binding, never repaired by aliasing inventory/member/native output. Restore exact bytes/bindings to recover healthy output. Valid digest syntax alone still passes shape and proves no external equality. Wrong returned projection identity is separately the existing structural invalid_projection case, whose entire result is {"verdict":"UNFIT","cells":[],"issues":[]}, not a partial INCOMPLETE. |
| TestNativeSnapshotNormalizer_Concepts/false-plan and /stale-rationale, ENV-G2 licence cases | Before projection the native owner tests the identical healthy/missing/stale inventory/member cases on raw bytes/history for both F/S. Assert exact source/head/Ref bindings and unknown_evidence or stale_pin at the corresponding path below. Missing card bytes instead uses the existing early Catalog refusal and never produces a local portfolio issue. No native format, key contents or successful native run is authored here. |

For these licence subcases H is the actual affected EnvironmentHead.ID, C each
claim using H, A that claim's approved Card.ID, and i the native member-licence
Ref's canonical index in H.Receipts (sort by ID/Path/SHA256 on a private copy,
not the recipe's serialized order). Escape every pointer token by RFC6901.
Use exactly these refusal paths p:

| Mutation | Exact p in the existing diagnostic view |
|---|---|
| Inventory bytes missing/unreadable, same-ID path/digest changed, or its internal membership/licence facts changed | /evidence/environments/H/bindings/licence |
| Member receipt bytes missing/unreadable, same-ID path/digest changed, or its internal member/licence facts changed | /evidence/environments/H/bindings/receipts/i |
| Repository commit changed | /evidence/environments/H/bindings/repositories/0/commit |
| Toolchain version or digest changed | /evidence/environments/H/bindings/toolchains/j/version or /sha256 respectively; j is that actual toolchain's canonical Name/Version/SHA256 slot |

R(reason,p) expands to (Code:null, Reason:reason, Path:p, ClaimID:C,
RequirementID:"", Catalog:null, Metric:none, Group:all-empty,
Scale:null, Observed:null, Required:0), nil error. It appears once in each
affected cell and global union, not in requirement Issues; all affected
requirement/cell counts become null for these whole-environment binding refusals.
N(repositories,S,1) expands to (Code:null, Reason:insufficient_environments,
Path:/portfolio/A/k/repositories/S, ClaimID:"", RequirementID:"",
Catalog:{domain:capability,id:A}, Metric:none, Group:all-empty,
Scale:{axis:repositories,class:S}, Observed:1, Required:2); k is that canonical
full stratum's portfolio index. It occurs once per affected A/full-stratum in
the global union only. Issue Path is an assertion location, never a new public
CoverageIssue field. Global issues sort before cell issues by the frozen law;
no other issues survive this single-mutation fixture. The other family's
counts/metric rows remain exactly healthy. Missing exact roles in authored
records is not a successful shape check of their external bytes.

The healthy fixture is conditional on complete actual Card/group closure in
this scope, measured scale, valid owner inputs and independently verified ancestry.
If the authoritative card requires another concept, these tests cannot fabricate
a healthy source: DEP2/3 stays blocked and its missing-concept evidence remains.

These licence assertions supplement every existing native/common/metric twin;
none lowers a count floor or supplies an unknown prerequisite. DEP3 acceptance
must bind the assertions to real incumbent native bytes and format. W3-2 leaves
real semantic RED with each implementing owner. This lane runs only document
checks, no Go RED, typecheck, -race, native test, materialization or full gate.

## Exact coverage obligations and negative twins

Use the preset-selected data keys `false-plan` and `stale-rationale` only,
subject to the authoritative inventory and complete group join above. Each
authoritative requirement has PositiveMinimum=30 and NegativeMinimum=30 or
higher card-authorized minima. Its native group is exactly MeasureGroup with
all four fields empty. The immutable port returns ProjectedSample with actual
WitnessID, comparable SampleKey, verified Match OriginKey and Polarity. The
author's MatchID or Origin Ref does not establish lineage. Positive/negative
origins are distinct, with one verified matched-pair origin. Equivalence
components and physical-anchor overlap deduplicate transitively as ENV-2 requires.

Supplementary measures use ProjectedMeasure.Members, Value=nil. False-plan
requires absent_symbols>=1 and unwired_entries>=1 within its 30 false rows
(this recipe supplies 15 of each). stale-rationale requires stale_instructions>=30,
live_instructions>=30 and governing_links>=30. No sum across concepts, cards,
tuples, environments or metrics is made. CoverageCell counts are minima across
requirements, not60 when each concept has30. Unknown is null, measured zero0.
T3-3 makes a subtype its own stratum only when the pinned card claims it. An
aggregate is not permission to issue subtype verdicts below30.

Native preparation, by its owner, verifies raw bytes, real pins, scope,
record links and history before building the immutable in-memory snapshot.
CoverageEvidence methods do no I/O and supply no self-labelled verification.
Catalog binds actual Card/CheckerContract bytes and the complete mapping;
Environment supplies measured scale and verified Families;
Concept supplies only independently supported samples and memberships.
Unsupported prerequisites omit a sample, unavailable whole backing refuses
unknown_evidence, changed binding/digest refuses stale_pin. ENV performs joins,
set counts and floors only. An instrument or projection error is not proof that
the environment lacks a concept. Raw native format and normalizer conformance
are ENV-G2/DEP3, not filled with an author-produced JSON fixture.

For each family and selected stratum retain healthy sources/key, and independently
construct the following twins in owned uncommitted workspace during the future
released test. They are ephemeral mutations of the existing data, not additional
tracked files or source grants. Keep unaffected populations above their floors.

| Twin | Exact requirement/result obligation |
|---|---|
| Remove false-plan or stale-rationale from a claim | Authoritative catalog retains it: missing_concept for that requirement. It cannot disappear from closure. Removing all eligible claims never produces an empty PASS. |
| Remove all native samples/metrics for one concept | A known absent concept cannot qualify. Preserve the distinction between a complete empty native set, missing projection (invalid_projection) and unavailable backing (unknown_evidence). No false measured-zero success. |
| Remove absent-symbol membership only | With common30/30 and other metrics healthy: absent_symbols0/required1. Separately unwired_entries0/required1. Removing just one of this recipe's15 members does not falsely trigger a floor1 failure. |
| Remove one stale, live or governing-link metric member | Mutate each independently: the selected measure29/required30, missing_count at its exact metric slot. Removing a governing anchor gives governing_links29/30; other metrics cannot hide it. |
| Cross-bind source, record, checker or commit | Refuse the precise changed binding as stale_pin or malformed projection per ENV-2, never preserve a passing count from it. Missing entire raw evidence gives null counts/unknown_evidence. |
| Duplicate or move a task, clone a fix, repeat a seed | Normalize ancestry/anchors; no extra independent site or environment. A changed witness ID cannot repair a short population. |
| Delete written rationale or repoint link | Native stale-rationale conformance excludes the affected governing-link member unless independently supported; no text-similarity or index-derived replacement. |

For common samples adopt ENV-2's entire 16-twin table unchanged, independently
for each concept. Let k independent p-i/n-i pairs share verified pair-i. Drop
the last side for drop-p/n, replace it by the first component for clone-p/n,
append a harmless first-component duplicate for append-p/n. Supplementary
members stay healthy. The known results are:

| Twin | k; mutation; positive/negative minima | Known positive/negative | Ordered local issue union |
|---|---|---|---|
| healthy30 |30; keep;30/30 |30/30 |[] |
| positive-short |30; drop-p;30/30 |29/29 |[Cn(29,30),Cp(29,30)] |
| negative-short |30; drop-n;30/30 |30/29 |[Cn(29,30)] |
| positive-clone |30; clone-p;30/30 |29/29 |[Cn(29,30),Dp(29,30),Cp(29,30)] |
| negative-clone |30; clone-n;30/30 |30/29 |[Dn(29,30),Cn(29,30)] |
| healthy-positive-clone |30; append-p;30/30 |30/30 |[] |
| healthy-negative-clone |30; append-n;30/30 |30/30 |[] |
| positive-min31-healthy |31; keep;31/30 |31/31 |[] |
| negative-min31-healthy |31; keep;30/31 |31/31 |[] |
| positive-min31 |31; drop-p;31/30 |30/30 |[Cp(30,31)] |
| negative-min31 |31; drop-n;30/31 |31/30 |[Cn(30,31)] |
| both-min31-healthy |31; keep;31/31 |31/31 |[] |
| both-min31-positive-short |31; drop-p;31/31 |30/30 |[Cn(30,31),Cp(30,31)] |
| both-min31-negative-short |31; drop-n;31/31 |31/30 |[Cn(30,31)] |
| both-min31-positive-clone |31; clone-p;31/31 |30/30 |[Cn(30,31),Dp(30,31),Cp(30,31)] |
| both-min31-negative-clone |31; clone-n;31/31 |31/30 |[Dn(30,31),Cn(30,31)] |

Cp/Cn mean missing_count, Dp/Dn clone_inflation for positive/negative, with the
same complete tuples, canonical paths and ordering as ENV-2, Exact diagnostic
grammar and Complete common-count twins. Here substitute the actual pinned
capability, claim, requirement and repository/S stratum identities, never its
opaque-fixture-card example as a live key. At=2026-11-15T00:00:00Z for these
fixed test vectors, not a claimed run time. Where the other independently
eligible family remains healthy, a local shortage also gives the portfolio
insufficient_environments1/2 issue first, global INCOMPLETE/nil. If none remains
eligible, use ENV-2's UNFIT law. Known counts never become null merely because
they miss a floor. All actual global/cell/requirement issues and sorted JSON,
not just a verdict substring, must match the approved diagnostic grammar.

The reused-positive matching attack also stays exact: nominal30 negatives
sharing one qualified positive yield positive30/negative1 with ordered
`[Dn(1,30),Cn(1,30)]`, plus the portfolio issue when applicable. This is the
approved ENV-2 T8-1 correction, not30 matches. Entirely unpaired negatives
count0. A false Match conflict is invalid_projection, not benign deduplication.

## Proof inventory, existing consumers and exact wiring requests

The data lane owns no Go test, CLI, checker, compiler adapter or generator
source. Existing product tests below are reuse evidence only. Tests specified
by approved ENV contracts but absent here are labelled obligations, never run
results. W3-2 assigns real semantic RED to each future implementation owner.

| Assertion and exact test | Exact later command | Present evidence limit / owner |
|---|---|---|
| Plan syntax: TestParseMarkdownPlan, TestParseMarkdownPlan_CheckboxSemantics | `go test -race ./internal/verify -run '^TestParseMarkdownPlan(_CheckboxSemantics)?$'` | Existing fake/static tests in markdown_test.go:11,68. Reuse grammar; they do not load governance data or establish native truth. |
| Plan result semantics: TestVerifyPlan_Verified, TestVerifyPlan_ClaimedButAbsent, TestWired_TestOnlyRowIsLibraryOnlyAndFailsStrict | `go test -race ./internal/verify -run '^(TestVerifyPlan_(Verified|ClaimedButAbsent)|TestWired_TestOnlyRowIsLibraryOnlyAndFailsStrict)$'` | Existing verify_test.go:44,69 and wired_test.go:66 use fake evidence. Pin their behavior, never use their index-shaped output as seed truth. |
| Anchor/scope refusal: TestCompile_RejectsBadAnchorsAndScopes | `go test -race ./internal/semantic -run '^TestCompile_RejectsBadAnchorsAndScopes$'` | Existing compile_test.go:143 checks compiler behavior; no governance-native key or semantic stale/live label follows. |
| Record link/reference existence: TestADRReferencesResolveConcern | `go test -race ./internal/custodian -run '^TestADRReferencesResolveConcern$'` | Existing adrrefs_test.go:17 checks real/phantom/unreadable references. Existence alone does not establish scope, applicability, rationale or30 independent links. |
| Shared record identity/effective policy: TestRepositoryRecordIDs, TestPlansArchiveAndGrandfather, TestGrandfatherRejectsStalePairs, TestRepositoryEnforcesCrossPlanUniqueness | `go test -race ./internal/recordids -run '^(TestRepositoryRecordIDs|TestPlansArchiveAndGrandfather|TestGrandfatherRejectsStalePairs|TestRepositoryEnforcesCrossPlanUniqueness)$'` | Existing recordids_test.go:104,51,151,165. Check(root,cfg) and Defaults/Load are reused; ENV CUE ownership/effective grants remain upstream REC work. No duplicate check/exception here. |
| Source shape/refs/subtype/ownership: TestValidateEnvironmentRecords_Valid, TestValidateEnvironmentRecords_NegativeShape, TestValidateEnvironmentRecords_NoAuthoredPass, TestValidateEnvironmentRecords_References, TestValidateEnvironmentRecords_Paths, TestValidateEnvironmentRecords_PriorRead, TestValidateEnvironmentRecords_ErrorOrder, TestValidateEnvironmentRecords_RecordOwnership, TestValidateEnvironmentRecords_T3Metadata, TestValidateEnvironmentRecords_T3Subtype, TestEnvironmentRecords_JSONArrays, TestValidateFleetManifest, TestEnvironmentRequest_Negatives | `go test -race ./schema/benchenv` | Approved core Failing-test inventory; implementation absent. Preserve every exact suffix assertion and RecordOwnership dependency. Shape alone cannot check external digest equality. |
| Raw bytes, repeatability, snapshot DAG, size/caps, T6, source27 conservation: TestBenchmarkFleet, TestBenchmarkFleet_InvalidManifest, TestBenchmarkFleet_SourceBytes, TestBenchmarkFleet_CanonicalDAG, TestBenchmarkFleet_FullSnapshots, TestBenchmarkFleet_TreesAndModes, TestBenchmarkFleet_SizeAndExclusions, TestBenchmarkFleet_Limits, TestBenchmarkFleet_Overflow, TestBenchmarkFleet_WorkspaceRoot, TestBenchmarkFleet_PathCollisions, TestBenchmarkFleet_PublicationCollision, TestBenchmarkFleet_Cancelled, TestBenchmarkFleet_IO, TestBenchmarkFleet_JSONArrays, TestBenchmarkFleet_InputConservation, TestBenchmarkFleet_IndependentBoundary | `go test -race ./internal/benchfleet -run '^TestBenchmarkFleet'` | All 17 approved ENV-3 tests remain producer-owner obligations. Generic baseline tests alone do not exercise this preset's unavailable sources. DEP4 requests governance fixture integration in C5, not producer API changes. |
| Common counts/matches/clones: TestEnvironmentCoverage_OneShort, TestEnvironmentCoverage_Matching, TestEnvironmentCoverage_CloneInflation | `go test -race ./internal/benchenv -run '^TestEnvironmentCoverage_(OneShort|Matching|CloneInflation)$'` | Approved ENV-2 tests, absent implementation. Every common twin above and reused-positive correction must be retained. Fake-port arithmetic is not native conformance. |
| Concept removal/floors: TestEnvironmentCoverage_MissingConcept, TestEnvironmentCoverage_ConceptSpecificFloors/false-plan, TestEnvironmentCoverage_ConceptSpecificFloors/stale-rationale | `go test -race ./internal/benchenv -run '^TestEnvironmentCoverage_(MissingConcept|ConceptSpecificFloors)$/^(false-plan|stale-rationale)$'` | Existing approved test obligations, absent implementation. Full assertion expansion in ENV-2 applies; raw normalization is separately DEP3. |
| Catalog/card/unknown/stale boundaries: TestEnvironmentCoverage_CapabilityClosure, TestEnvironmentCoverage_CardMapping, TestEnvironmentCoverage_GroupClosure, TestEnvironmentCoverage_TrustedBoundary, TestEnvironmentCoverage_StalePin | `go test -race ./internal/benchenv -run '^TestEnvironmentCoverage_(CapabilityClosure|CardMapping|GroupClosure|TrustedBoundary|StalePin)$'` | Approved obligations, not a data-owned substitute catalog. Every unavailable byte and changed binding remains visible. |
| Scale, independent environments, purity/arrays: TestEnvironmentCoverage_Scale, TestEnvironmentCoverage_IndependentEnvironments, TestEnvironmentCoverage_NoMutation, TestEnvironmentCoverage_Determinism, TestEnvironmentCoverage_JSONVocabulary, TestEnvironmentCoverage_PhaseSeparation | `go test -race ./internal/benchenv -run '^TestEnvironmentCoverage_(Scale|IndependentEnvironments|NoMutation|Determinism|JSONVocabulary|PhaseSeparation)$'` | Approved leaf obligations. One multi-repo family or seed replicas cannot fill two environments, no public-outcome inference. |
| Native source/lineage/record/semantic projection: TestNativeSnapshotNormalizer_Concepts/false-plan and /stale-rationale | `go test -race ./internal/benchenvnative -run '^TestNativeSnapshotNormalizer_Concepts$/^(false-plan|stale-rationale)$'` | Already requested by approved ENV-2, Exact D26 native-conformance wiring request. DEP3 requires accepted raw fixture manifest, pin/format and exact native proof before it can run. Not an existing implemented oracle. |
| Flag/structured loader/transport: TestEnvironmentDispatch_PlanForms/ENV-G2 and TestEnvironmentDispatch_ExecutionInputs | `go test -race ./cmd/scenariobench -run '^TestEnvironmentDispatch_(PlanForms|ExecutionInputs)$'` | Approved ENV-C0 obligations. Its zero-native-call fake seam proves transport only. No source-root or private execution value in public receipts. |
| Preset data load, generator-before-native-before-coverage, all twins, conservation: TestEnvironmentSeed | `go test -race ./cmd/scenariobench -run '^TestEnvironmentSeed$'` | Approved ENV-C5 obligation. DEP4 requests a governance case using the exact data files above, both families/seeds and this full assertion inventory. No consumer exists at this base. |

### Cleared-schema assertions retained verbatim

The following proposed assertions belong to the existing schema/data/native
owners. They remain future implementing obligations, not executed Go proof.

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
| `TestValidateConceptInventory_KeyConflict` | Run cue vet -c with the existing schema and exact stored valid.cue plus fully commented invalid.cue: exit0. Extract only the commented conceptInventoryConflict example into owned scratch, removing exactly one leading // plus one space from each shown line; assert the resulting bytes equal the shown30/31 witness without comments. Explicitly vet the schema plus that scratch negative: nonzero with a conflict at conceptInventoryConflict.concepts.K.expectations.0.minimum, concrete30 versus31. Validate its extracted inventory source: ValidationShape /concepts/K/expectations/0/minimum and exact empty-map failure JSON; reverse the two declarations: identical reason/path/result, not CUE diagnostic-order dependence. Never require the stored invalid.cue to fail vet or infer full-gate failure from scratch package inclusion. No source key discovery/checker is called. |
| `TestValidateConceptInventory_CompatibleDeclarations` | Repeat K with the same complete content, then split required_by and expectations across two K declarations: both cue vet -c and source validation succeed and export the same single K map value as the valid fixture. Do not assert that CUE rejects compatible repetition. The distinct other-record-id RecordOwnership tests remain mandatory. |
| `TestConceptInventory_PathGrant` | In the future data lane, supply its one exact-file path grant to the existing pathcheck. Only that proposed inventory path passes. A changed/new second inventory path and a changed/new ungranted path fail, even with otherwise valid CUE; the path check need not inspect content or allocate keys. An empty or permissive glob is not an exact-file grant. This is pathcheck proof, not a pure schema/Go validator filesystem test; the data lane owns it, no schema test imports a path checker. No actual grant is made here. |
| `TestConceptInventory_MapKeys` | Use each of slash, tilde, space, Unicode λ and combined `a/~ λ` as supplied valid field labels; these particular spellings evaluate unchanged, so their decoded map keys round-trip exactly. General Unicode field evaluation is tested separately below, not promised raw-byte preservation. For the combined evaluated key omit expectations: ValidationShape /concepts/a~1~0 λ/expectations; duplicate expectation metric: ValidationIdentity /concepts/a~1~0 λ/expectations/1/metric. Repeat in the join: exact /inventory prefix. Empty label refuses at /concepts/ and whitespace-only label at its escaped member. Nil Concepts refuses /concepts. Permute CUE field order and Go map insertion order, also placing errors under distinct keys: identical JSON and selected lowest escaped pointer/reason. With missing expectations under both a/b and a~b, choose /concepts/a~0b/expectations (escaped-pointer order), in either source/map order; its join twin chooses /inventory/concepts/a~0b/expectations. Traverse evaluated map keys by UTF-8 bytes, then select errors by the existing escaped-pointer/reason law. No trimming, ENV normalization or raw key in Error(). |
| `TestConceptInventory_CUEFieldSemantics` | Use literal source label NFD e\u0301, codepoints U+0065 U+0301, UTF-8 65cc81, and NFC \u00e9, U+00E9, UTF-8 c3a9; record input bytes/codepoints and raw export stdout/stderr/exit. Assert raw exported field-label UTF-8 is c3a9 (U+00E9), not65cc81, while each raw exported reference string retains its supplied codepoints/bytes. SourceNFD and SourceNFC each pass cue vet -c and concrete export; exact decoded Concepts map is {"\u00e9": record}, size1, key UTF-8 c3a9, never a retained NFD key. Decode to the Go ConceptInventory: exactly that NFC key/unchanged record, initialized Groups. CompatibleNFDFirst and CompatibleNFCFirst declare both raw spellings with the same complete record: vet/export/source validation pass, one map entry, output equal to the single NFC case, never a duplicate-id error. ConflictingNFDFirst and ConflictingNFCFirst attach minimum30 to NFD and31 to NFC, changing only declaration order: vet/export fail with conflicting30/31 at the evaluated field; source validator returns ValidationShape /concepts/é/expectations/0/minimum and exact empty-map failure JSON in both orders. ReferenceNFC and ReferenceNFD cross EACH label spelling with a group's concept string value NFC or NFD: all four CUE vet/export cases pass shape, exported label is NFC, reference VALUE bytes stay respectively c3a9 or65cc81. NFC reference passes source/value validation; NFD reference refuses ValidationReference /groups/0/concepts/0, join twin /inventory/groups/0/concepts/0, without mutation or reference rewriting. With valid NFC group mapping, an NFD Requirement.Concept refuses join ValidationReference /records/requirements/0/concept; NFC requirement resolves against the same complete Card/checker refs. Decoded map key and each retained reference value must match exact codepoints and bytes above. For direct typed Go maps, no CUE parse/normalizer runs: exact matching supplied key/value strings resolve, mismatched strings refuse at the same existing group/requirement pointer; this local equality test claims no authored-source/provenance acceptance. Repeat the two single-label cases in a schema-free CUE struct {keys: {label:30}, reference:label}: fields evaluate NFC while reference values retain their original NFD/NFC bytes. Existing CUE field semantics is the only identity mechanism. |
| `TestConceptInventory_CommentedNegative` | Read stored invalid.cue: every nonblank line begins with //, including the complete conceptInventoryConflict example; no active stored negative. The exact shown example uses // followed by one space on each line. Copy only that example, strip exactly that prefix per line, and assert the bytes equal the unchanged30/31 witness. Write only owned workspace scratch with the existing package/schema context. Explicit cue vet -c of schema+stored valid+commented invalid returns0; explicit schema+extracted scratch negative returns nonzero at conceptInventoryConflict.concepts.K.expectations.0.minimum. Source-validator refusal is the exact KeyConflict assertion above. This extraction is test-only reuse of the commented-negative convention, no production parser or new fixture format. Do not rely on ./... to discover testdata, move schema paths or claim a full-gate result. |

### T12 pure evaluator assertions retained verbatim

The following exact rows are the cleared ENV-2 obligations. P/Q and G0 retain
that peer's tuple/diagnostic grammar, using its two-environment F fixture with
unrelated floors healthy. Synthetic K/L are independently supplied, not production
keys. Native/catalog owners verify real expectations and exact source refs before
snapshot preparation. They retain TestEnvironmentCatalog_InventoryJoin: changed
expectation bytes with stale Source refuse stale_pin at /evidence/catalog;
unavailable inventory bytes give unknown_evidence there, both with zero later
calls. A repinned reviewed source must deliver its changed expectations.

| Exact test in coverage_test.go | Exact assertions |
|---|---|
| `TestEnvironmentCoverage_ExpectationBinding` | Give K the aggregate output expectation set from T3-3 (output_cases30, question_cases30, long/short1, route/rank/table1, each unit and exact exception citation from core), L cross_file_paths30/paths, and the same G0. Returning only cross_file_paths30 for K refuses *CoverageError ErrorInvalidProjection at P/measures/cross_file_paths/0, normalized UNFIT Cells:[] Issues:[], no partial counts. Bind the same measured cross-file set to L with all other floors met: PASS/nil, counts30/30, MeasureResult cross_file_paths observed30 required30. Remove output_cases while all other expected metrics pass, leaving no unexpected metric: ErrorInvalidProjection at P/measures/output_cases/0, same empty error result. Also supply unrelated known cross_file_paths30 with output_cases still missing: the unrelated metric is refused even though it has30. Consistently permute K/L through inventory, groups, records, catalog, witness refs and snapshot bindings, preserving each expectation's semantic attachment: keep claim/requirement/witness IDs fixed and repin all affected refs: result JSON is byte-identical. Swap only the two expectation attachments: the formerly valid cross-file snapshot refuses its now-unexpected cross_file_paths. Rename arbitrary checker-ref IDs consistently without changing inventory: result bytes unchanged, no dispatch on those IDs. Every check uses zero evaluator port calls. |
| `TestEnvironmentCoverage_ExpectationMinima` | Omitted cross_file_paths minimum validates to30. Measured29 -> exact existing C(cross_file_paths,29,30,G0) and N(...,1), INCOMPLETE/nil; measured30 healthy twin PASS. Direct Go Minimum0/29 (even with unrelated valid citation) -> ErrorInvalidInput /inventory/concepts/<K>/expectations/0/minimum, empty error result. Every canonical below30 metric uses its exact matching cited floor and unit: at that floor healthy PASS; at floor-1 exact existing C(metric,floor-1,floor,G0) and N(...,1), common30/30 retained. Remove a required typed citation or supply a wrong typed citation/unit -> core-owned invalid_input pointer, no silent default. Raw null/untyped members are core source-validator cases and must refuse before this typed API is called. History199 -> C(history_commits,199,200,G0), history200 PASS. Raising inventory cross-file minimum to31 makes measured30 -> C(cross_file_paths,30,31,G0). Selected subtype metrics29 still refuse required30, aggregate presence1 never lowers them. Missing native backing stays R(unknown_evidence,P/receipts/0) and null counts, while present empty members produce C(metric,0,required,G0); neither can PASS. |
| `TestEnvironmentCoverage_SnapshotBoundary` | Capture exact old Catalog->Environment->Concept order, once per binding, private per-call copies and existing early-refusal/first-error assertions using PrepareCoverageSnapshot. Then replace the fake with a panic-on-call sentinel and evaluate only inventory+snapshot: no calls or I/O. Repeated and concurrent evaluations yield byte-identical JSON and unchanged input bytes, even after mutating the fake's original backing slices after assembly. Inspect all snapshot fields recursively: values only, no port/interface/callback/error interface/context or authored pass fields. Missing/duplicate/extra success slot, success plus refusal for SAME binding, malformed/out-of-scope refusal and contradictory catalog failure/later slot each refuse ErrorInvalidProjection at that exact canonical evidence root. Expected refusal for a distinct binding retains the existing complete issue union. Unexpected native error is normalized to typed evidence_unavailable at its invocation root, raw cause absent; zero later preparation calls, no partial result. nil/empty typed Expectations refuse /inventory/concepts/<K>/expectations before measured evaluation. Missing/null/untyped raw expectation fields must first refuse in core source validation, not be fabricated as unrepresentable typed Go values. |

`TestFleetManifest_GeneratorVersion` must reflect the named GeneratorVersion
field, GeneratorVersionV1 exactly "env-fleet/v1", and unchanged generator_version
JSON key. The constant fixture passes. Missing, null, empty, unknown-string and
non-string versions each refuse FleetShape at /generator_version with the exact
initialized failure JSON from the core; nested SeedInput mutations refuse
ModeShape at /seed/manifest/generator_version. No arbitrary future version passes.

These are dependencies of their existing owners, not additions to this data
lane's write grant. TestValidateEnvironmentRecords_RecordOwnership concerns only
other authored ENV records and retains their existing U1 grants/checker laws,
including grandfathered-pair/new-duplicate controls. It is not a concept-field
validation, discovery, numeric grant or prerequisite. The future data lane proves
its single exact-file inventory path grant through existing pathcheck; no actual
grant or filesystem test is supplied here.

No command in this table has been run in this documentation lane. No model,
native binary, generator, gate, fleet, benchmark or readout was executed.
The controller releases capacity for later bounded -race proofs and owns the
full gate. Listing a prospective command is neither a test result nor a grant
to create its package. `fathom verify --service fathom --plan
docs/plans/bench-environments.md` is required before changing a plan claim;
no checkbox changes here.

| Dependency | Exact missing input or wiring, with disposition |
|---|---|
| ENV-G2/DEP1 | The exact cleared W18/T12 core and peer source has bounded documentary LAND/W2-3 approval only. This preset alignment needs fresh independent review and the controller exact-source final review before integration. Schema/leaf/producer/mode implementation is absent at this base and needs later grants. Retain native ports, producer signature, complete T6/T8 and metric/count laws. Full ENV-1 CLI/provenance remains HELD; old104 is superseded, ENV excluded from frozen8, ACK8 absent. No self-approval or integration grant. |
| ENV-G2/DEP2 | The data owner must supply its one exact-file inventory path grant and critic-reviewed keyed CUE contents with independent expectations/citations, actual Card/checker bytes and complete groups. Existing schema/vet/pathcheck suffice; no concept REC discovery, numeric policy or RecordOwnership prerequisite. Other authored ENV records retain their separate U1 laws. BEN/native catalog owner runs actual CheckCardClosure then pure ValidateEnvironmentConceptJoin with all Card.IDs before projection/snapshot preparation. No roster, content or grant is invented. |
| ENV-G2/DEP3 | Native owner accepts the exact 28-leaf finite output manifest and licence assertions above plus an exact raw fixture/receipt manifest for the named direct/split sources and native.json inputs, actual compiler/test/entrypoint tool pins and outputs, source/record/lineage identities and the two existing TestNativeSnapshotNormalizer_Concepts subtests. Cover every population, semantic counterexample and common/supplementary twin above. Native source bytes, record keys/grants, hashes, complete entrypoint evidence and independent template ancestry are currently unavailable. Do not serialize a fake native key or define its format here. |
| ENV-G2/DEP4 | C5 owner wires its existing mode-owned loader, source Ref-to-ContentID binding, private roots, exact SeedInput to producer, native preparation, derived EnvironmentRecords, actual CheckCardClosure, complete full-ID inventory/group join, PrepareCoverageSnapshot and pure inventory+snapshot leaf call. Include ENV-G2/LicenceHealthy, LicenceMissing and LicenceStale composition cases without a duplicate native checker. Request the governance case in TestEnvironmentSeed with every assertion above, actual preset bytes/digests and preserved source27. C0 owns transport tests. No data-owner code edits or central switch. |
| ENV-G2/DEP5 | After DEP1..4 and actual immutable inputs exist, data owner freezes exact seed manifests, both source/output digests per FleetFile, actual FleetResult JSON and actual measured size/caps using the approved producer protocol. Independent reviewer checks them against native bytes/history before any count or reading. No stub value is legal in the meantime. Two independently eligible candidates and exact card strata remain unproven. |
| ENV-G2/DEP6 | Fresh independent critic must review this new peer-source amendment; prior exact preset approval does not transfer and no T8-2 round is used; track owner applies W2-3 only after LAND and governing-choice accounting. Publication/integration, source-bound controller gate, later reading and any BEN-5 admission remain distinct. This author does not self-award approval or use an unrun reading as PASS/UNFIT. |

DEP1..5 are ordinary planned implementation/input dependencies. No genuinely
unsettled policy decision was identified. If a required source format, record
grant or card claim is incompatible with this recipe, stop at that exact
boundary and return the conflict; do not silently expand ownership or reopen a
settled T3/T6 choice. There is no speculative alternate API in this contract.

## Conservation, workspace and documentation evidence

The original 27 fauxtown files, including the hidden worktree fixture, are a
conservation set only. They establish no governance task or wider concept
count. Reuse BuildFleet's explicit copy pattern
(`internal/mcp/costtest/fleet.go:46-64`), never its ingest/store/client imports or
bulk padding as independent work. The original link key is 3564 exact bytes,
SHA-256 `6ef02cfb613bac1985e17eef1de72885f5642dbc0a160d8c521250c867492f51`.
The source set is pinned to this lane's immutable base above. An original key
copy is external truth for conservation only, not governance-native proof.

Every future generated checkout, Git control fixture, truth output, healthy
twin, failed attempt and comparison root must be physically below owned ignored
`workspace/`, uncommitted, using ENV-3's containment/publication/error rules.
No ambient external TempDir, source checkout modification or generated Git data
in tracked paths. The generated author and committer are exactly
`Environment Fixture <fixture@example.invalid>`, UTC `946684800+k` in canonical
topological order per repository. Source commits retain the source owner's
identity. No remote write/restart, fetch, global config or second fingerprint
implementation. Every proof example in this document inherits these conditions.

Documentation proof here is limited to immutable source hashes, exact API/text
conservation, existing citation/test-name checks, file ownership, Markdown/JSON
shape and common-count arithmetic. Such a check can pass while DEP1..5 remain
unavailable. It is not API typechecking, semantic Go RED/GREEN, -race, actual
independence, native count, admission or a source-bound full gate.

### Frozen source27 inventory

All paths below are relative to `internal/connector/testdata/fauxtown/` at
the lane base. All 27 modes are 100644. Check the complete set and each exact
byte digest, including the hidden worktree member, before and after later proof.
Do not count copied files as new governance tasks.

| Path | Bytes | SHA-256 |
|---|---|---|
| `catalog-service/catalog.go` | 1449 | `180f8e2eb12212fa0cc93fc64acdf1bac0ecdc946943f3fc7dfcc5972ad16bec` |
| `checkout-service/checkout.go` | 1274 | `485db68c54aae3db1318e470f0ceb089c521d0cfa77e7e29753b776461d8580d` |
| `fulfillment-worker/gen/inventory/v1/inventory_grpc.pb.go` | 500 | `651859ecf36f42526e359ce5de93276cf79e0defbf3ba3d29cbb398eef8b942c` |
| `fulfillment-worker/worker.go` | 621 | `9e200fd4bad295e9a673aeded62ebb88fdb694a47fe378e4be1fe9dbaf16589d` |
| `inventory-grpc/gen/inventory/v1/inventory_grpc.pb.go` | 500 | `651859ecf36f42526e359ce5de93276cf79e0defbf3ba3d29cbb398eef8b942c` |
| `inventory-grpc/proto/inventory.proto` | 355 | `e42686fc316d4c06cde69c7466eff33fa1443633da5ca7e0b17224807276a47e` |
| `inventory-grpc/server.go` | 194 | `27d8cfe80eaae7b35ddfac3355ca03c7cee5938bf77ee19d13d016381adbfd5d` |
| `ledger-service/errors.go` | 213 | `9b158befb5f604243ed576e3a5fb478fc899c61372a91800dd6f6290f576a107` |
| `ledger-service/ledger.go` | 1240 | `434741486d5966fb5d796afadac5a77e7f26609806fe4daee62e6aec63c797a7` |
| `links.answer.json` | 3564 | `6ef02cfb613bac1985e17eef1de72885f5642dbc0a160d8c521250c867492f51` |
| `orders-api/main.go` | 854 | `0591cfa716cfcd428f05b2cb55ee463663272c653c7240427e0f2c56f0193957` |
| `orders-api/openapi.yaml` | 331 | `6a3e79db44f848f4d81826945e2ee6b655fb56095b7546cacc9ce87cdccb1290` |
| `payment-service/errors.go` | 223 | `57eac2fcb8e61b211bf977f1ece1096d200a012da17e6a92f9773e37846be5dc` |
| `payment-service/payment.go` | 1079 | `5c42a32f7fba0f6931a7729502ec72637b894920a7561112689c009fcbb53fca` |
| `polyglot-service/App.java` | 194 | `c77f5ba8caad85e0e38088a5241bf7c56ca7275db5bbc10b835be5fda6d57fac` |
| `polyglot-service/README.md` | 197 | `b261fc02f620fdfc0a77212efabb6bba6892aa30f5bfa49393713140cb41fe2f` |
| `polyglot-service/app.js` | 208 | `c343de3b67198dab591384ddaefbb36d5d51fd8c7f401dd11cccbbfe25c01208` |
| `polyglot-service/app.py` | 179 | `5b5b19b13b3214d0adb2e116f753fa889093c86683c4262bec12304699117e1a` |
| `polyglot-service/app.rb` | 120 | `1994a55230c7fed5065243d276e2c7120fa364c9f43bb94d2eed6a0068f959b7` |
| `polyglot-service/deploy.yaml` | 192 | `ce349a6a8499da1279bc21e24022766bbd5be57d736b0f419a1af6a70c8475e3` |
| `polyglot-service/go.mod` | 113 | `19df527e7d356b8a873961282415a0f93a11a339cbf894d138767eb1198ee85b` |
| `polyglot-service/main.tf` | 147 | `0d680bdbc5794896015f3d28fa2ba1a60df796af879eaff406ee749d90a7293c` |
| `polyglot-service/schema.cue` | 115 | `399b60da866dd165cc7e498ab23694060a14c256748d243d0f490cbac7722fbe` |
| `storefront-web/src/api.ts` | 801 | `4e8e40c7531db8cbe577b41e0b6aa77634963bacdb97c3371dd89ca42d8f7c05` |
| `storefront-web/src/gen/inventory_grpc_web_pb.ts` | 541 | `47c1145fcb2949469227f3e6da452c52f7f4ab25f854513cbf15ccdafb99dd22` |
| `worktree-service/.claude/worktrees/agent-0001/main.go` | 268 | `1aff886990460b04ca55eeda5d4e304a3b2d1bc7302b293fb47aeacf9530b18a` |
| `worktree-service/main.go` | 300 | `95ab95a134e0f613e2ce504c1ae6cb18600aeaeca2bb14556a3f29b04fe6bda4` |

## Decision models

None applies. This preset supplies deterministic native-evidence inputs with
provider `none`. No judgment site, model call, model-labelled oracle or model
key is introduced. Experimental agents, if separately authorized later, remain
subjects and cannot supply the primary truth.
