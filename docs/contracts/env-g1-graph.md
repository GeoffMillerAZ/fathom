# ENV-G1: bounded graph-preset data contract

Status: DRAFT W18/T12 preset alignment; fresh independent criticism and W2-3 approval required.
This author owns this preset document only. This draft freezes the data handoff and
minimum recipe below. Missing native artifacts and consumer wiring are explicit
prerequisites, not invented payloads. It is not an executable preset, an ENV-G1
reading, an admission, or an implementation grant. No plan box changes.

## Intent and frozen dependencies

The [intent router](../intent/README.md) routes this work to
[scope](../intent/scope.md), [non-goals](../intent/scope/non-goals.md),
[principles](../intent/principles.md), [invariants](../intent/invariants.md), and
[decisions](../intent/decisions/README.md). P2/P3/P15/P17 require deterministic,
visible, owned, computed evidence. I43 requires attested proof, I54 forbids
unsupported language claims, I56 forbids automatic downloads, and I62 excludes
model judgment from proof. Their documented enforcement limits do not change.
The pinned intent-design procedure was applied before selecting this recipe.

[BE-D2](../design/bench-environments.md) fixes per-stratum floors and independent
truth. BE-D5 fixes fixture conservation, truth before ingest, distinct constructed
topologies, and separation from public effects. BE-D4 requires two independently
eligible environments per claimed axis/class. [ENV-G1](../plans/bench-environments.md)
is the only work order addressed. Its current concepts are cross-file,
fan-in-dispatch, and cross-repository. The illustrative card titles in BE-D2 are
not Card.ID mappings. No other preset, WPF ladder, FTU, native oracle, analyzer,
registry, model site, or public outcome is added.

The benchmark-validity trigger was already raised in the design's Intent before
design paragraph. No new fired trigger or intent conflict is established by this
document. ADR-085 supplies independent anchors and power discipline. ADR-108/109
remain proposed records, used as the supplied design context. There is no change
to the existing compatibility surface. No documented future is implemented or
pre-built. Existing references and immutable evidence preserve later consumers
without an extra field or abstraction.

These exact artifacts, not a floating peer branch, identify provenance and
pending dependencies. Historical review is not approval of a superseded roster:

| Dependency | Frozen commit and document SHA-256 | Observed review boundary |
|---|---|---|
| Historical ENV-1 schema core, `docs/contracts/env-1-core.md` | `c3e8988b9133649ccf432778493cc36c60c5a90b`; `1e965b2fe8d73c5a9ded7d7a5a0bec7c05530964be215e676196388681863b83` | Publication review LAND on byte-identical `cccb365ae4479c6ceec3c7c57ced83c663a79789`. Retained provenance; W16-2 requires the amendment below before dispatch. REC ownership/grants and implementation remain dependencies. |
| Historical W16-2 amended core, same path | `2ada35b791d8115fc05e563ba3af571e624a9895`; `2ac44ac36988aacc9783e76e676b5d2abdd5006afb4ded36241f775fb7047981` | Independent track and wave LAND/W2-3 approval covers bounded publication only. Open concept record/inventory and Card.ID/group join replace fixed-key closure; integration/implementation and inventory data/grants remain dependencies. |
| Single authoritative concept inventory | `calibration/environments/concepts.cue`; exact reviewed bytes/digest not supplied | Proposed sole input path in the amended core's Concept record shape and single inventory section. Each key must cite its requiring capability card or BE-D2/BE-D9 design row. No inventory contents or data grant is invented here. |
| Historical ENV-2, `docs/contracts/env-2.md` | `b59a8c3f67ef8115097af431f581502c5802dd8d`; `d9d77b1f5c8bad20c35e9700d12a1466885da7b8cccb45b98ec2ec1082f6c152` | APPROVED-env-2 records independent LAND and W2-3 approval. No coverage implementation or native adapter follows. |
| Historical ENV-3 producer, `docs/contracts/env-3-fleet.md` | `4d5e8cff0defdbb29366f20f8517f10f16d439a2`; `28c590df7926f56b1d1bcc9cd7a3a1284552ad77c256fc92207e08afd700180d` | Prior `d5442905a4441ff1d77d7009b8f62d2c60f09646` received FIX/F1. R1 repairs the missing-checkout test clause. Separate R1 critic not supplied at this authoring snapshot; approval remains pending. |
| Historical ENV-1 command contract, `docs/contracts/env-1.md` | `8bc356c285b9557f4265497c52e9c00f195903a3`; `99480d95ad7fbbe0abea9ac3e6c5deba98390bab0b2f4c1fb040c9c9e615792f` | Fresh T8 critic says LAND, F4 closed. Earlier full-contract HOLD is not silently promoted by core publication. Integration and actual command implementation are unproved. |

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

The prior preset approval binds `cc47fce75726e44b211af782e66422daecc44d16`, file SHA-256
`0f8ef62946dd043ea498d5de346da2d723a2b88cacb6342f52fb8660e3216e5f`.
It does not approve these amended bytes. A fresh independent critic is required
for this new user-directed pin/precedence amendment, not another G1 repair round
or a T8-2 exception. No implementation dispatch or plan-row claim is granted.

The historical core was extracted from `44d4f45184996c505e0bcc96d5ab5a8c65da56e4`.
Its fleet declarations/validator are core lines 506-655; authored records are
lines 124-310; seed/request declarations are lines 659-917. ENV-2's operative
port is lines 55-236, catalog law 237-311, metric/D26/count law 313-540,
assertions 661-832 and 1248-1392. ENV-3's operative expansion/Git protocol is
lines 290-461; assertions 735-761. ENV-1's seed payload is lines 1334-1340,
loader/transport 1723-1873, and TestEnvironmentSeed at line 2547. Immutable
commit plus file path identifies every peer citation without requiring that a
held contract be stacked into this branch. Supplied approval/review receipts
establish documentation status only, not runtime evidence.

W16-2 keeps keys open; W18/T12 supersede its list shape, concept-REC dependency
and former expected-metric gap through the exact sources above. The three G1
selectors remain conditional preset scope. Complete Card/checker requirements,
including non-G1 concepts, cannot be trimmed to manufacture a passing graph card.
A cited design-only key may remain unmapped. Native/catalog and C5 owners must
supply the real source bindings and measured snapshot; no G1 validator, alternate
roster or invented native payload is authorized.

## Governed choices

| Decision | Bounded selection | Governing text |
|---|---|---|
| ENV-G1/D1 | Only cross-file, fan-in-dispatch and cross-repository data; standard-library Go native route, no product feature or oracle. | ENV-G1 plan lines100-101; BE-D2 lines79-109 and BE-D5 lines185-195. |
| ENV-G1/D2 | Full-stratum30/30 plus the three supplementary metric sets; exact Card.ID/selector binding, no inferred subtypes. | BE-D2 lines81-93; T3-1/3; approved ENV-2 lines237-311,346-348,776-832. |
| ENV-G1/D3 | Two candidate three-repository Go topologies; repositories=M only; different seeds remain replicas, native ancestry controls independence. | BE-D4/5 lines127-144,185-195; approved ENV-2 lines466-540. Recipe labels, filenames and source layout are ordinary data choices, not new evidence policy. |
| ENV-G1/D4 | Caller-supplied raw sources, complete snapshots, existing fleet/request/port shapes, seeds7/8, explicit caps and all output below ignored workspace. | T6-2 and frozen ENV-3 lines290-461; core lines506-655,659-917. |
| ENV-G1/D5 | Native owners verify raw truth/history before projection; C5 joins generator and checker; retain refusal, no local-PASS shortcut. | T3-2; BE-D3 lines118-125; ENV-C5 plan lines64-65; approved ENV-2 lines174-236,466-540; command contract lines1603-1686,2547. |

## Future data-lane ownership: exact new files

Only the following new paths are selected under ENV-G1's allowed roots. They
are a prospective data grant, not files created in this authoring lane. All
prefixes in this table expand to `calibration/environments/fleet/graph/`.
There are no wildcard grants and no checker, loader, test, or generator source
files. A missing upstream artifact does not authorize another path.

| Exact relative file | Data responsibility |
|---|---|
| `seed-7.json` | Existing EnvironmentRequest, mode seed, existing SeedInput; embeds the complete seed-7 FleetManifest. |
| `seed-8.json` | Same existing shapes for repeatability counterseed 8. |

The exact per-file source grant is this finite expansion. For EACH prefix
`linear` and `fork`, own `module-a.mod`, `module-b.mod`, `module-c.mod`,
`chain-entry.go`, `chain-step.go`, `chain-bridge.go`, `chain-gold.go`,
`chain-control.go`, `fan-target.go`, `fan-a.go`, `fan-b.go`, `fan-c.go`,
`dispatch-sites.go`, `dispatch-controls.go`, `client-a.go`, `server-a.go`,
`client-b.go`, `server-b.go`, `client-c.go`, `server-c.go`,
`protocol-a.yaml`, `protocol-b.yaml`, `protocol-c.yaml`, and `seed.txt`,
all beneath `sources/<prefix>/`. These 48 literal paths carry raw bytes, not an
executable template language. The three module inputs map to a/b/c go.mod with
module paths `example.invalid/<prefix>-a`, `example.invalid/<prefix>-b`, and
`example.invalid/<prefix>-c` respectively. They use the locally pinned Go
toolchain and standard library only, with no fetched or vendored dependencies.

Additionally own exactly `truth/linear-records-7.json`,
`truth/linear-records-8.json`, `truth/fork-records-7.json`,
`truth/fork-records-8.json`, `truth/linear-links.answer.json`,
`truth/fork-links.answer.json`, `truth/source-policy.md`, `truth/licences.md`,
and `expected.md`. Records use existing EnvironmentRecords. Link keys reuse the
existing fauxtown answer-key shape, never a new pass/qualified format.
`expected.md` is a human-readable identity/count ledger, not parsed runtime
configuration or an ENV oracle. It contains actual hashes when bytes are frozen,
never dummy digest literals. Raw tool output remains owned native workspace
evidence referenced by existing Ref; these paths grant no new receipt format.
The earlier F1 repair adds the eight exact binding paths and twenty instrument
output requests below. Review F2 adds twelve member licence output requests
under the same native owner. Neither adds a FleetFile, checkout file or truth entry.

The only reading paths are
`docs/bench/readings/environments/seed-graph.md` and
`docs/bench/readings/environments/seed-graph.json`. JSON uses existing derived
EnvironmentReceipt with its exact nested CoverageResult, never a new graph
reading DTO. Markdown links the exact request, result and native receipts and
reports scopes/counts/limits. Its plan verdict line is written only after actual
execution. No placeholder PASS or UNFIT line is authored now.

## Existing APIs only

These declarations are reproduced unchanged. Both `env` and the generator signature's `benchenv`
name schema/benchenv. They are frozen contracts, not available
symbols at this base. No data lane owns their implementation.

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
func ValidateFleetManifest(source []byte) (FleetManifest, error)
func ValidateEnvironmentRecords(source []byte) (EnvironmentRecords, error)
func ValidateEnvironmentRequest(source []byte) (EnvironmentRequest, error)
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

The snapshot and preparation helper above are exact cleared ENV-2 declarations,
owned by internal/benchenv. The preceding pure evaluator takes inventory and
snapshot only; CoverageEvidence remains the unchanged preparation port.

`FleetManifest` has exactly Seed, GeneratorVersion, Template, Size,
Repositories and Truth. `FleetFile` has exactly Path, ContentID, SourceSHA256,
SHA256, Mode, Class, Seeded. There is no registry key, source bundle, operation,
service declaration, truth status, optional future field, or generator-side
native call. GeneratorVersion has the core-owned named GeneratorVersion type;
use benchenv.GeneratorVersionV1, with unchanged generator_version JSON key and
"env-fleet/v1" wire value. The exact peer fleet reproduction above carries that
binding; no local alias or schema copy. Template is a nonblank label, never a source-discovery mechanism.
`SeedInput.Sources` supplies existing Ref values whose IDs equal ContentIDs;
C5 resolves their canonical repository-relative paths and exact SHA-256 into
caller-only raw `map[string][]byte`. No ambient directory scan supplies bytes.
Unknown IDs cannot fall back to disk or a template registry.

The seed-7 and seed-8 requests each select one complete explicit manifest:
version `env-fleet/v1`, template `graph`, Seed 7 or 8. The minimum complete
bank comprises two candidate topology families, linear and fork, each with
three repository IDs `<family>-a`, `<family>-b`, `<family>-c` and three separately
bound service identities. Both families occur in the manifest; their two
EnvironmentHeads select their own three returned repository pins. This supplies
two candidates, not an assertion of independent environments. Native history
and template verification alone can establish that distinction. A second seed
of either family always remains the same ancestry component.

Each repository has c0 (empty root) and c1 (parent c0, full final snapshot),
Head c1. Repositories sort by ID, snapshots by path, logical commit IDs are
scoped per repository, authored parent order is retained. Six repositories and
twelve reachable repository-scoped commits are the recipe arithmetic. Native
records bind returned full 40-hex commits, never logical c1 or assumed zeros.
Expected commit identities and seed-specific records must be frozen from the
source bytes before execution, using ENV-3's exact object law. The generator
does not substitute logical IDs into record files.

## Minimum graph bank and eligibility

Only Go is proposed. The smallest claim vector is repositories=M (each head
has three repositories); line/file/history measurements still report all four
axes, with exclusions, and make no unsupported S claim. Split is dev and tier
constructed, repository URLs are empty, no public outcome claim. Prediction
and aggregate/subtype selectors MUST come from the exact approved Card and
CheckerContract, not this author's interpretation of the word graph.

The minimum bank below applies to EACH candidate environment and EACH full
stratum actually selected by that binding. It is not a pooled 60/60 count.
If both effect and null are claimed, each needs its own complete bank. Different
language/vector/prediction/subtype witnesses cannot repair a shortage. Empty
Subtype is permitted only for an explicitly bound aggregate claim; any claimed
subtype needs its own 30/30. Unclaimed below-floor subtype counts are descriptive
only (T3-3 and ENV-2 SubtypeClaims). No subtype enum is introduced.

| Exact concept key | Required native population in each environment/stratum | Minimum supplementary metrics |
|---|---|---|
| `cross-file` | 30 independently qualified positive paths, each depth >=3 forward edges across >=3 distinct product files, and 30 paired disconnected controls. Linear recipe uses four successive definitions in four product files; fork recipe has two branches reconverging before the gold definition, without a shorter route. | cross_file_paths=30. A disconnected control must resolve its source and gold, then yield no-graph-path, not no-oracle. |
| `fan-in-dispatch` | 30 independently qualified dispatch or re-export positives with 30 matched safe/absent controls, AND three distinct product target symbols. Each target has five distinct product (path,line,column) callers distributed 2/2/1 over three outside Go import-package units. | fanin_symbols=3, dispatch_sites=30; the three symbols and fifteen calls do not replace common30/30. |
| `cross-repository` | 30 independently checked protocol links with 30 paired non-links. Ten positive links on each of a->b, b->c, c->a within the family bind three repos and three services. Non-links preserve the controlled source/provider context and independently check absence. | repository_links=30, linked_repositories=3, linked_services=3. |

Thus a single full stratum needs 90 positive memberships and 90 matched negative
memberships per environment, 180/180 across the two environments, without
assuming those are 180 distinct tasks or power units. Metric populations may
share verified sites under ENV-2's rules; no cross-metric sum is a verdict.
Thirty renamed copies of one chain, one dispatch site, or one link remain one
origin if the native lineage says so. Numbered declarations and changed seed
comments never prove independence. A compiler proves resolution, not lineage.
An independently checked origin/match inventory is mandatory before claiming
these floors. No source payload here is claimed to already meet them.

The file mapping for each family is exact: module-a/b/c.mod -> a/b/c go.mod
(FileBuild); seed.txt -> each repo seed.txt (FileBuild); client-a/b/c.go and
server-a/b/c.go -> the corresponding repo service/client.go and service/server.go
(FileProduct); protocol-a/b/c.yaml -> corresponding repo protocol/openapi.yaml
(FileProduct). In repo a, chain-entry/step/bridge/gold/control.go map respectively
to chain/entry.go, chain/step.go, chain/bridge.go, chain/gold.go, chain/control.go;
fan-target.go -> fan/target/target.go, fan-a/b/c.go -> fan/a/a.go, fan/b/b.go,
fan/c/c.go; dispatch-sites/controls.go -> dispatch/sites.go, dispatch/controls.go.
All those Go files are FileProduct. Every file is mode100644. No imports cross
repository modules: protocol links are service boundaries, not Go import edges.
The G1 data owner authors the listed source/key inputs. Native owners must
verify this layout and complete payloads before any runnable proof. In particular, the three fan caller files are
distinct import packages; chain files must have the required forward paths.
The source list above deliberately does not authorize extra files to fix a
native compilation or independence defect. Missing exact payloads and verified package
resolution are dependency gaps listed below, not a new oracle or
permission to materialize an approximation.

## Bytes, truth identities, repeatability and resource accounting

Every source Ref binds actual raw bytes by ID, canonical relative Path and
lowercase SHA-256. Each FleetFile separately binds raw SourceSHA256 and expanded
SHA256. Raw source ContentIDs are `linear-` or `fork-` plus the source filename with
its final extension removed (for example linear-module-a, linear-chain-entry,
fork-protocol-c). Truth ContentIDs are linear-records-7/8, fork-records-7/8,
linear-links, fork-links, source-policy, licences, and fauxtown-key; each request
uses its selected seed-specific record ID. Ref.Path for these raw sources is
the exact repository-relative source path selected above. The source-ID/path/hash
enumeration is a required freeze artifact in expected.md. It is currently absent.
A filename/ContentID selection alone is not an exact byte identity. Mandatory missing bytes/pins remain
unknown and block execution. There are no zero-filled hashes or invented pins.

Only `seed.txt` is Seeded=true, raw bytes `seed=@ENV_SEED@\n` (one actual LF).
It maps to `seed.txt`, FileBuild, mode100644 in each repository. All Go, module,
protocol, key and record source files are Seeded=false and copied literally.
For seeds7/8 it expands to `seed=7\n` / `seed=8\n`, seven bytes each.
Its raw SHA-256 is `9795ce7914153e664e4c109894b9695dfc56bfa4764ebd24e98735c746b4dd44`.
The expanded seed-7 hash is `e106b3b43e65032853bbc22b9ef08cbf46f2a205ccd01236e35cab0836adbba5`;
seed-8 is `dccd1e7a86544624263af6736e14fd0bb1545367af286981bfd194168533f14f`.
These literal-byte calculations are documentation arithmetic only. Product bytes and protocol keys
are unchanged across seeds; c1 hashes and bound record commits differ, c0
hashes and topology families do not. Size and excluded counts stay identical.
Seed-specific record bytes are separately supplied, not invented token expansion.
Head, requirement, origin and receipt refs use the F1 layout below. Their actual
source/native bytes, SHA-256 values and tool/code pins remain UNPROVEN unavailable
inputs. DEP2/DEP4 supply and verify those inputs; they do not choose the layout.

Truth FleetFiles are generated-class, unseeded, mode100644 and outside all
repositories. Paths `linear/records.json` and `fork/records.json` use the matching
seed-specific records; `linear/links.answer.json` and `fork/links.answer.json`
use the frozen new independent link keys; `source-policy.md` and `licences.md`
bind native source/exclusion and licence inventories; `fauxtown/links.answer.json`
is the unchanged original key with ContentID `fauxtown-key` and source/output
SHA-256 `6ef02cfb613bac1985e17eef1de72885f5642dbc0a160d8c521250c867492f51`.
The existing key is read directly from the preserved source27 path, not rewritten.
Result refs are the existing `{id:ContentID,path:"truth/"+Path,sha256:SHA256}`.
C5 consumes exactly the role/path/ID bindings below. Its implementation remains a
wiring request, not a new registry or an available adapter.

Compare seed 7 twice in fresh absent children under ignored workspace, then seed 8
in another child. Compare every checkout/truth byte and mode, canonical result
JSON, blob/tree/commit IDs, parent order and returned refs. Ignore compression,
inode/mtime and staging names only as ENV-3 permits. All generated repositories,
objects and raw outputs remain uncommitted under workspace. ENV-3's generated
identity is exactly `Environment Fixture <fixture@example.invalid>` for author
and committer, UTC timestamp946684800+k, not a real person's identity. This is
the frozen neutral .invalid identity, not authority to invent another domain.

Size uses HEAD product LF-counting and excluded vendored/generated/test/build
rows, including measured zero. History is repository-scoped reachable unique
commits. Truth contributes to neither source nor excluded counts. FleetLimits
are explicit positive int64 caps: six repositories, twelve commits, all full
snapshot entries plus truth entries, expanded bytes over all entries. Equality
passes, one-short refuses. The fixed mapping gives 20 product files and six build files per family
(14/3/3 product files across a/b/c), hence FleetSize.Files=40 and
FleetSize.Repositories=6, FleetSize.History=12. Per-head measured files/history
are20/6; only repositories=M is claimed. Empty c0 plus complete c1 snapshots
mean52 snapshot entries; seven truth entries make MaxFiles=59 at equality.
MaxRepositories=6 and MaxCommits=12 are equality caps. Product line counts,
build line counts other than the six one-line seed outputs, and MaxBytes CANNOT
be filled before native source/key/record bytes freeze. expected.md
must enumerate them with independent arithmetic, not a guessed scale label or
borrowed cost-harness bulk count. No hasher duplicating BEN-9 is introduced.

## ENV-G1/F1: frozen acyclic Ref bindings

This preserves the first normal F1 repair of source `3f5022c7c1e196b9dd6c0815c3b2067e4f22e305`.
It freezes representation, resolution and finite ownership, not evidence values.
Let G be exactly `calibration/environments/fleet/graph`, F be exactly `linear`
or `fork`, S be exactly `7` or `8`, and K be exactly `scale`, `lineage`,
`cross-file`, `fan-in-dispatch` or `cross-repository`. These Cartesian products
are exhaustive finite expansions, not directory grants. All paths below are
canonical repository-relative Ref paths; G is expanded literally in serialized
Refs. The two requests each select only their S. No alternate component layout
or extra dependency path is implicitly permitted.
For the member licence bindings only, M is exactly `a`, `b` or `c`, selecting
the existing repository member `{F}-{M}` and its returned full commit.

| Exact additional path | Serialized existing type or raw bytes; ID and role | Sole prospective owner |
|---|---|---|
| `G/bindings/requirements.json` | EnvironmentRecords with only Requirements populated; Environments/Witnesses/Coverage are []. Each reference ID is the exact contained Requirement.ID, selecting exactly one member; whole-file SHA-256, never an element digest. Contains the complete authoritative SeedInput.Requirements, including required non-G1 concepts, joined to the single cited inventory and all Card.ID/CheckerContract groups. | G1 data owner copies the DEP3 authoritative requirements without renaming IDs, trimming the inventory or changing strata. |
| `G/bindings/cards.json` | JSON array of existing CTR-13 Card values with approved JSON names. Requirement.Card.ID is that exact Card.ID, equal to Capability; each selects exactly one member, with the whole-file digest. This is not a new card/envelope type. | G1 data owner copies the authoritative shipped Card collection supplied by BEN-3/CTR-13 owner. |
| `G/bindings/checker-contract.raw` | Exact native-owner contract document bytes covering the complete requirement set, including grouped obligations, normalization and subtype metadata; Ref.ID is `graph-checker-contract`. Every Requirement.CheckerContract uses this one binding. No ENV wrapper or guessed native format. | Native snapshot owner supplies its existing contract bytes; G1 data owner preserves them literally. |
| `G/bindings/catalog.raw` | Exact existing T3 read-only mapping bytes for the complete catalog/requirements/cards/contracts derived from the single authoritative inventory, never a second maintained inventory. Ref.ID is `graph-catalog`. SeedInput.Catalog and CatalogProjection.Source are this identical Ref. No serialized CatalogProjection with a self-referential Source. | T3 mapping owner supplies the W16-compatible mapping and exact inventory pin; G1 data owner preserves its bytes. The upstream inventory is a read-only dependency, not a G1-owned file. |
| `G/bindings/{F}/head-{S}.json` | One existing EnvironmentHead, ID exactly `{F}-head-{S}`; environment role only. Four files. No enclosing EnvironmentRecords, witness, claim, request or self digest. | G1 data owner, after native verification of source commits, scale and pins. |
| `workspace/artifacts/benchenv/graph/{F}/seed-{S}/{K}.raw` | Exact existing independent native instrument output bytes; ID exactly `{F}-{K}-{S}`. Twenty files: scale receipt, verified history/clone/template lineage, compiler/path/product truth, D26/dispatch truth, protocol/non-link truth respectively. Origin role resolves only to lineage; concept receipt role resolves only to its concept K. No authored native record/envelope. | Precise request to ENV-2's native snapshot preparation owner under its existing snapshot.go/snapshot_test.go grant request. Owner acceptance is required, not presumed. Output stays ignored and uncommitted. |
| `workspace/artifacts/benchenv/graph/{F}/seed-{S}/licence-{M}.raw` | Exact existing native-verified member licence receipt bytes, ID exactly `{F}-licence-{M}-{S}`. Twelve files, one per F,S,M; binds that repository member and full source commit to its licence evidence, with whole-file SHA-256. Member licence receipt role only, distinct from inventory and every K instrument output. No G1 native schema or authored verification flag. | Same native snapshot preparation owner must accept the precise output/verification request under its existing grant request. Actual native format, bytes, pins and verification remain missing dependencies; outputs stay ignored and uncommitted. |

The first five rows add exactly eight future data files under G. The last two
rows request twenty instrument outputs plus twelve member licence outputs
(thirty-two workspace outputs total) and graph acceptance in the already
requested `internal/benchenvnative/snapshot.go` and `snapshot_test.go`; it grants
no new native source file, fixture directory, producer or format. If the actual
incumbent format cannot supply these bindings, retain DEP4 as unavailable and
return for an explicit grant amendment, never concatenate unrelated outputs,
borrow a policy/licence envelope, invent bytes, or choose another resolver.
Actual Card.ID/Requirement.ID values and all native payload/format/pin/digest
values are unavailable owner inputs. Their equality rules and physical paths
are fixed here. The finite path grant does not depend on their eventual count.
A missing authoritative requirement or non-G1 concept is not permission to trim
catalog closure. All REC definition ownership/discovery/grants remain DEP1:
inline serialized copies are not permission to register duplicate definitions
or bypass the shared checker. No REC exception is created.

The three repository members per head are exactly those already selected by
this recipe. The native owner verifies complete member coverage against the
licence inventory, including each member's applicable source/dependency licence
facts. Licence remains the inventory, not a substitute receipt. A per-member
output must be an existing native receipt capable of binding those facts; no
author assembles a new envelope or promotes a licence text to verified truth.
The native owner must also bind the pinned toolchain's applicable licence facts
under the existing inventory/native contract; no unaccounted member or licence
obligation may be silently omitted. If the authoritative inventory/native format
requires additional receipt files or cannot resolve this finite set, DEP4 stays
unavailable pending an explicit grant amendment. No extra path is implicit.

**Resolution is role-scoped and exact.** Preparation reads only the selected
request, its Sources and the exact paths above or the preselected truth paths.
It compares ID, literal canonical path, existing type/member identity and raw
whole-file SHA-256 together, without scanning directories or guessing by basename,
ID prefix, file extension or the index. A file containing several Requirements
or Cards uses ID selection inside that existing collection, not a new envelope,
new per-object digest or arbitrary JSON-pointer/fragment in Ref.Path. Every
supplied ID must select one member; none or several is a refusal. No ID alias,
request-inline fallback, generated-file fallback or cross-role substitution.
The native owner verifies the bytes and provenance before immutable projection;
decoding any of these authored files alone supplies no verified evidence.

| Existing Ref occurrence / role | Exact resolution and equality assertion |
|---|---|
| Witness.Environment, CoverageClaim.Environment, EnvironmentProjection.Head, ConceptProjection.Environment | `{F}-head-{S}` -> `G/bindings/{F}/head-{S}.json` -> EnvironmentHead. Aggregate Environments contains the same typed head, field for field, including every pin/ref/scale. |
| Witness.Requirement, CoverageClaim.Requirements, CatalogEntry.Requirements, ConceptObligation.Requirement, ConceptProjection.Requirement | Original authoritative Requirement.ID -> `G/bindings/requirements.json` -> that Requirement. Aggregate Requirements and SeedInput.Requirements equal the complete file collection. Seed 7 and 8 share these exact refs/bytes. |
| Requirement.Card | Capability/Card.ID -> `G/bindings/cards.json` -> that existing Card; native preparation compares its actual shipped claim and selectors. No invented graph Card.ID. |
| Requirement.CheckerContract | `graph-checker-contract` -> `G/bindings/checker-contract.raw`; all required native metadata verified, Checker remains the distinct actual code Pin. |
| SeedInput.Catalog, CatalogProjection.Source | `graph-catalog` -> `G/bindings/catalog.raw`; CheckCardClosure and the complete pinned mapping law still apply. |
| EnvironmentHead.Subjects | Exactly the family's 24 source refs selected above, each to `G/sources/{F}/<literal-filename>` with its existing source ContentID and RAW digest. These are authored bytes, not generated checkout aliases. Native preparation verifies the specified expansion/full Git commits separately. |
| EnvironmentHead.Licence / SourcePolicy | `licences` -> `G/truth/licences.md`; `source-policy` -> `G/truth/source-policy.md`. Only their stated inventory/policy roles, no head/requirement/native envelope. |
| EnvironmentHead.Scale.Receipt | `{F}-scale-{S}` -> the scale K output above, independently bound to the exact source/full commits. |
| EnvironmentHead.PriorReads | [] for this constructed dev recipe, subject to native exposure/history verification; no held-out or clean-history qualification inferred. |
| EnvironmentHead.Oracles | Exactly the three concept K output refs plus `{F}-links` -> `G/truth/{F}-links.answer.json`, reusing the independent compiler/D26/protocol instruments and independent link key. No product/index-derived key. |
| EnvironmentHead.Receipts | Exactly the five K output refs for F,S plus all three member licence receipt Refs `{F}-licence-{M}-{S}` at the exact paths above, sorted by ID/path/hash: eight Refs per head. A repeated K Ref in Oracles/Receipts means identical native bytes in the explicitly stated roles. Licence inventory, member receipts and K output roles cannot substitute for one another. |
| EnvironmentHead.Receipts member licence role | For every M, `{F}-licence-{M}-{S}` resolves only its `workspace/artifacts/benchenv/graph/{F}/seed-{S}/licence-{M}.raw` bytes. Native preparation verifies the same `{F}-{M}` repository/full commit and all applicable licence facts against the inventory before EnvironmentProjection.Head succeeds. No seed/family/member alias, concept/origin role or generated truth copy is accepted. |
| Witness.Origin | `{F}-lineage-{S}` -> the lineage K output. Native lineage resolves each witness anchor/polarity and verified match to its existing OriginKey/Match; Ref.ID alone is not an origin or independence proof. |
| Witness.Receipt, ConceptProjection.Receipts | Witness.Receipt is exactly the selected concept K output Ref. ConceptProjection.Receipts contains that Ref and the lineage output Ref, sorted by ID/path/hash. Raw payloads must resolve every projected witness and metric membership, not just assert a count. |
| FleetResult.Truth aggregate role | `{F}-records-{S}` -> `truth/{F}/records.json` at generated root -> EnvironmentRecords copied from `G/truth/{F}-records-{S}.json`. Exactly one selected aggregate per family. Native/C5 compare its head and requirements to their standalone bindings before use. |
| FleetResult.Truth key/policy/licence roles | `{F}-links` -> `truth/{F}/links.answer.json`; `source-policy` -> `truth/source-policy.md`; `licences` -> `truth/licences.md`; `fauxtown-key` -> `truth/fauxtown/links.answer.json`. Exactly these five plus the two aggregate refs. They retain the original source paths/bytes/digests and designated roles. |

All Refs inside requests/records/standalone components use sourceRoot, including
paths beginning G and `workspace/`. The caller's explicit sourceRoot is their
only filesystem base. The seven Refs returned specifically in FleetResult.Truth
use the generated root, exactly as ENV-3 specifies; C5 dispatches by that typed
field plus the exact seven ID/path pairs, never by the `truth/` prefix alone.
For seed S, their raw source map binds precisely the existing ContentIDs above
through SeedInput.Sources. The new binding files/native workspace outputs are
preparation dependencies, not additional FleetFiles or truth-map entries.
Copies of keys/policy/licences under generated truth must equal their authored
source bytes; the head's sourceRoot Refs are not rewritten to generated paths.
The structured and flag forms use the same existing private execution roots.
No local root, source bytes or role-map DTO is added to public JSON or APIs.

**Bytes-hash order, with no back edge.** An arrow below means the left artifact
contains a digest/binding of the right; ID labels without digests do not add a
hash dependency. Construction/hashing proceeds right to left:

1. Source/policy/licence-inventory/key/Card/native-contract bytes and real tool/code pins
   are leaves. Compute raw-file SHA-256 only after actual bytes exist. Expand
   the seed and compute Git identities by ENV-3's existing protocol; product
   commits contain no binding, truth, request or receipt files. Existing Card
   ProofRefs and native contract dependencies must also terminate in independent
   upstream input bytes, never any downstream graph artifact. DEP3 must supply
   compatible authoritative inputs; do not edit cards or discard proof refs.
2. The upstream concept inventory -> its cited Cards/native contracts; it never
   binds G1 requirements, catalog or later artifacts. Requirements and catalog
   preparation bind its exact bytes and complete group join, without adding a
   field to Requirement. Requirements -> Cards/native contract. Catalog mapping ->
   Requirements, inventory, Cards/native contract. Native outputs -> source/full
   Git commits, independent keys/policy/licences, real pins and
   Requirements/native contract as actually used. They may name head/claim/witness
   IDs and source commits, but MUST NOT
   contain hashes of head files, aggregate records, either seed request, generated
   truth/result, expected.md or reading outputs. No enclosing-file digest field
   is invented in any raw payload. No native output hashes itself or another
   native output in the thirty-two-output request; specifically, no member receipt
   hashes a head, aggregate, request or another native output. Member receipts
   bind source/full commits, licence inventory and real pins only. The catalog
   never binds its own digest or a native/head/aggregate/request/result/reading digest. Unsupported
   native format or upstream back binding remains unavailable, not permission
   to strip native data or change any authoritative card/contract/mapping.
3. Head files -> source/key/policy/licence/native output leaves. They never bind
   an aggregate or a request. Requirements never bind heads/native outputs.
4. Aggregate records -> head files, requirements file and native outputs.
   Inline heads/requirements are exact typed copies; their Ref digests are whole
   standalone file digests. Neither aggregate is a Ref target from itself or
   another aggregate. No witness/claim uses the inline seed-request requirements.
5. Requests -> completed aggregate/key/policy/licence truth bytes, raw sources,
   Requirements and catalog. Generated truth copies have identical unseeded
   bytes; FleetResult -> those copies. Reading/expected ledgers may bind all
   preceding inputs/results but nothing preceding binds their digests.

Thus self-hash R->R and two-file R->Q->R have no topological order, whereas this
fixed dependency order is acyclic for both families/seeds even when aggregate
records share a requirements file. Seed changes cannot create ancestry roots.
No canonical-object hash, second BEN-9 fingerprint hasher, new oracle, port,
record type, general cycle-error API or self-digest fixed-point search is needed.

**Frozen implementing assertions, not delivered tests.** Attach the following
subtests to ENV-2's already requested `TestNativeSnapshotNormalizer_Concepts/GraphRefBindings`
in snapshot_test.go and C5's already frozen `TestEnvironmentSeed/GraphRefBindings`.
Run every binding role above for each applicable F,S and each authoritative
requirement/witness, including shuffled collection order. These are required
assertions of those existing requested entrypoints, not new proof consumers.
Healthy/twin cases use isolated snapshots at the same declared relative paths,
with their own actual byte hashes; they do not mutate originals or add paths.
DEP3 must supply requirements using the frozen paths, with no silent Ref rewrite,
and the complete cited inventory/group join after CheckCardClosure. Its owner
must exercise the amended core's TestValidateConceptInventory_OpenKeys,
TestValidateConceptInventory_References, TestValidateEnvironmentConceptJoin,
TestValidateEnvironmentConceptJoin_Requirements and
TestValidateEnvironmentConceptJoin_GroupClosure assertions, including a reviewed
cited key added through inventory data only, unchanged contract bytes, and
group/claim closure. These remain pending owner tests, not tests delivered by G1.

- Healthy: resolve every exact ID/path/role/file-hash, compare the selected typed
  head/requirement/Card and complete arrays, verify source/generated copies,
  source commits, native pins and witness/match/member provenance. Assert exact
  EnvironmentProjection.Head, ConceptProjection.Environment/Requirement/Receipts
  and CatalogProjection.Source. Assert only seven FleetResult.Truth refs and
  unchanged size/caps; seed 7 repeats, seed 8 selects only its own heads/native
  outputs/aggregates while retaining authoritative requirements and ancestry.
- Member licence healthy: resolve all twelve bindings across both F,S, exactly
  three per head, and assert complete inventory-to-member/full-commit coverage,
  native byte/pin verification and exactly eight sorted Head.Receipts. Compare
  EnvironmentProjection.Head field for field, including all three member Refs;
  Licence still resolves only the inventory. Assert the five K Refs and both
  concept projection receipt Refs unchanged. Canonical receipt order is
  cross-file, cross-repository, fan-in-dispatch, licence-a, licence-b, licence-c,
  lineage, scale, each with the F prefix and S suffix. The member error paths
  are `/bindings/receipts/3`, `/bindings/receipts/4`, `/bindings/receipts/5`
  under the selected environment prefix, not the inventory's `/bindings/licence`.
- Member licence missing: separately omit each required member receipt from a
  consistently rebound, shape-valid head/aggregate twin, remove its file, make
  it unreadable, or leave its required native member binding unresolved.
  Environment returns existing
  *EvidenceRefusal{Reason:CoverageUnknownEvidence,Path:P} at that member's
  expected receipt slot. Counts remain unknown and existing R(reason,P) keeps
  null observations; neither a licence inventory nor healthy K outputs repairs it.
- Member licence stale: separately return a different member, family, seed,
  full commit, native pin, ID, canonical path or role; change actual bytes with
  the old digest or supply a shape-valid wrong digest. Include inventory-as-receipt
  and K-output-as-receipt substitutions even when bytes coincide. Each returns
  existing *EvidenceRefusal{Reason:CoverageStalePin,Path:P} at that same slot.
  A missing binding remains unknown; a resolved mismatched binding is stale.
  Malformed Refs retain shape precedence. Exercise head/aggregate equality and
  both CLI forms through the existing GraphRefBindings entrypoints, preserving
  original R/result/exit/error precedence and without manufacturing native proof.
- Missing: separately remove/unreadably bind each required file or selected
  member ID, including a valid-shape ID with no resolved member. Native method
  returns existing *EvidenceRefusal{Reason:CoverageUnknownEvidence,Path:P}.
  Counts are unknown, never known zero or a native failure claim.
- Wrong: keep the shape-valid request intact and separately supply a resolved
  binding for a different ID, different canonical path, wrong role (even with
  identical bytes), valid-shape wrong
  SHA-256, or changed actual bytes with old digest; also mutate an inline copy
  against its standalone member. Each is existing *EvidenceRefusal with
  CoverageStalePin (`stale_pin`) at P. Known ID absence uses the unknown case;
  a returned different member is this stale case. No new refusal vocabulary.
- P follows ENV-2's existing diagnostic view and first-error stages. Catalog,
  requirement-file, Card and checker-contract resolution first uses
  `/evidence/catalog` (one global issue, zero Environment/Concept calls).
  After successful Catalog, environment head identity uses
  `/evidence/environments/<head-id>/head`; subject i, policy, licence and
  environment receipt i use that environment prefix plus `/bindings/subjects/i`,
  `/bindings/source_policy`, `/bindings/licence`, `/bindings/receipts/i`.
  Oracle i uses `/bindings/oracles/i`; scale receipt uses `/bindings/scale/receipt`.
  Indices are the existing canonical Ref sort, with a missing member's expected
  slot retained. Concept Environment/Requirement uses
  `/evidence/concepts/<claim-id>/<requirement-id>/environment` or `/requirement`;
  concept receipt uses that concept prefix plus `/receipts/0`; origin/lineage
  uses `/receipts/1` (the lineage Ref follows the concept Ref in canonical order).
  Escape actual IDs by the existing RFC6901 rule. Existing R(reason,P) preserves
  exact scope/reason and null observations for the refused binding. Independently
  verified concept counts survive a scale-only refusal. An omitted authored
  requirement instead remains the original missing_concept/input-reference law,
  not a reclassification of an external artifact mutation. Malformed Ref syntax
  remains the original ValidationError/ModeError shape/path law.
- C5 exercises mutations reaching preparation in BOTH CLI forms, with separate
  sourceRoot/generated root and repeated basenames to catch root confusion.
  Generator precedes native preparation, which precedes the pure checker;
  no ingest is a truth source. Expected EvidenceRefusal becomes the original
  R(reason,P) issue/nil error from CheckEnvironmentCoverage, not an invented
  thrown error. Completed semantic refusal uses the existing dependency-refused
  receipt/exit1 law and preserves its complete CoverageResult. Where an existing
  typed leaf error is actually returned, retain the exact cause through the
  existing NewModeError/pointer-prefix law and initialized-error clearing law.
  Earlier request/source I/O, unavailable implementation and generator refusal
  retain their ORIGINAL ModeIO/ModeUnavailable/FleetError precedence, pointers
  and process mapping; never force later native calls after an earlier failure.
  Normalizer role mutations are checked independently even when a corresponding
  C5 input fails earlier. Do not infer an exit from a string or call a fake port
  native proof. Keep all concept-removal, one-short, match and closure twins.

Actual native preparation, the named tests, C5 producer/loader/checker wiring,
all source/native bytes/hashes, pins, observations and qualification remain
UNPROVEN. This freezes their acceptance and ownership now, without claiming a
current producer, native result, plan completion or ENV-G1 approval.

## Existing native truth work and conservation

The preserved source27 inventory is every tracked fauxtown file at base
`54bca03154ea35c39bf08acba4c3572e843da56a`, including the hidden worktree fixture.
Keep all 27 paths, bytes and modes identical, not just the visible service files.
The original key has seven links and six negative descriptions. It neither
fills30/30 nor names independent origins for a new graph bank.

Reuse the copying pattern in [BuildFleet](../../internal/mcp/costtest/fleet.go),
lines49-140, without importing its MCP/store/client harness or its ingest step.
Its bulk functions are scale padding, never labelled tasks. The existing
[TestFauxtownFleetAnswerKey](../../internal/syslink/fleet_test.go), lines70-179,
checks product link matching against the old hand-authored links; it reads Links,
not Negatives, and has a named TypeScript gRPC-web gap. It cannot be the new
independent protocol oracle or a 30-negative acceptance test. OpenAPI/proto
product tests are regression evidence, not independent truth.

[HopDistance](../../internal/benchprep/hop.go), lines19-164 and 173-220, already
uses Go packages/types, SSA and CHA with downloads disabled. Its SCIP alternative
requires an independent producer and actual enclosing ranges
([hop_scip.go](../../internal/benchprep/hop_scip.go), lines46-141).
HopDistance proves a shortest forward static distance, not runtime execution,
product-file cardinality, lineage, fan-in or protocol-link truth. Its result
cannot alone establish the entire cross-file projection. The native adapter
must bind the missing path/product facts before immutable projection, using
existing compiler/SCIP facts rather than a second ENV graph oracle.

[D26 recount](../../calibration/bench/tools/recount.py), lines8-61, supplies
product/support filtering and outside-unit policy; its
[policy tests](../../calibration/bench/tools/test_recount.py), lines10-40, keep
product exceptions. C6Row/vetC6 at internal/benchprep/vet.go:63-70,392-426
lose source-site provenance and cannot establish native truth alone. ENV-2's
requested TestNativeSnapshotNormalizer_D26 is still a separate native-owner
grant/fixture dependency. No author-qualified flag replaces it.

T3-2 assigns raw-byte, pin, history/clone, product membership, protocol and key
verification to native owners BEFORE projection. PrepareCoverageSnapshot uses the unchanged CoverageEvidence port to capture
verified projections; pure inventory+snapshot evaluation then performs the
deterministic joins/counts without I/O. Fathom's index never
supplies truth, and product syslink output never validates its own gold key.

## Exact assertion ownership and future proof commands

Commands below are acceptance commands, NOT executed in this documentation lane.
Named tests distinguish existing code from frozen-but-unimplemented tests.
The data lane adds none of their source files or a substitute local checker.

| Assertion | Exact existing test/proof entrypoint and required result | Current limitation / request |
|---|---|---|
| Preserve source27/key and caller bytes on success, changed seed, negatives and faults | Frozen ENV-3 TestBenchmarkFleet_InputConservation and TestBenchmarkFleet_IndependentBoundary; `go test -race ./internal/benchfleet -run '^TestBenchmarkFleet_(InputConservation\|IndependentBoundary)$'` | Producer is not implemented. Add graph data as consumer fixtures only through the producer owner's explicit acceptance, no G1 source ownership. |
| Same input same bytes/DAG/result; seed8 differences and no second environment | Frozen TestBenchmarkFleet, TestBenchmarkFleet_CanonicalDAG, TestBenchmarkFleet_SourceBytes; `go test -race ./internal/benchfleet -run '^TestBenchmarkFleet($\|_CanonicalDAG$\|_SourceBytes$)'` | ENV-3 approval/implementation and exact graph payload/hash freeze required. |
| Source/raw/output digests, full snapshots, size/exclusions, caps, workspace, neutral Git metadata, typed failures | Frozen TestBenchmarkFleet_FullSnapshots, TestBenchmarkFleet_SizeAndExclusions, TestBenchmarkFleet_Limits, TestBenchmarkFleet_WorkspaceRoot, TestBenchmarkFleet_JSONArrays; `go test -race ./internal/benchfleet -run '^TestBenchmarkFleet'` | Generic producer assertions bind this recipe. They cannot prove concept membership, ancestry independence or admission. |
| Old link-key regression without claiming new independent truth | Existing TestFauxtownFleetAnswerKey, TestProtoProvidesAndStubConsumes, TestOpenAPIProvidesSpecTier; `go test -race ./internal/syslink -run '^(TestFauxtownFleetAnswerKey\|TestProtoProvidesAndStubConsumes\|TestOpenAPIProvidesSpecTier)$'` | TestFauxtownFleetAnswerKey does not consume old negatives. Native owner must supply independently checked new30/30 link bank and protocol-to-projection adapter; no existing independent protocol-bank acceptance was located. |
| Directed compiler paths, missing truth and independent SCIP | Existing TestHopDistanceGoTypesCHA, TestHopDistanceMissingOracleAndTruth, TestHopDistanceSCIPReferences, TestHopDistanceSCIPRefusesMissingRanges, TestHopDistanceSCIPRequiresIndependentProducer; `go test -race ./internal/benchprep -run '^TestHopDistance'` | Existing fixtures prove these API semantics only. Graph30-path native receipt/three-product-file adapter acceptance is missing. |
| D26 filter, self/test exclusion and product exceptions | Existing ProductCallers.test_excludes_support_in_every_language, .test_units_exclude_self_and_all_test_callers, .test_keeps_product; `PYTHONDONTWRITEBYTECODE=1 python3 -m unittest discover -s calibration/bench/tools -p test_recount.py` | Recount consumes supplied resolved sites. ENV-2's requested TestNativeSnapshotNormalizer_D26, native bytes/projection fixtures and owner grant remain required; no native proof from a mocked test. |
| Per-concept floors and one-short/depth/files/units/repo/service mutations | Frozen TestEnvironmentCoverage_ConceptSpecificFloors/cross-file, /fan-in-dispatch, /cross-repository and TestEnvironmentCoverage_D26; `go test -race ./internal/benchenv -run '^TestEnvironmentCoverage_(ConceptSpecificFloors\|D26)$'` | Leaf fake-port projection tests are not native conformance. Exact raw mutation projections require native-owner acceptance. |
| Matched30/30, clones, stale/unknown, closure, full stratum, two independent environments and deterministic JSON | Frozen TestEnvironmentCoverage_OneShort, _Matching, _CloneInflation, _StalePin, _CapabilityClosure, _CardMapping, _SubtypeClaims, _IndependentEnvironments, _Determinism, _JSONVocabulary; `go test -race ./internal/benchenv -run '^TestEnvironmentCoverage'` | W18/T12 inventory expectations/group closure, actual Card.ID/mapping/selector/pins and immutable native adapter are missing. Frozen twenty-one-key prose is superseded, not a test roster. |
| Generator then native preparation then leaf; exact graph healthy and removal twins; loader and both CLI forms | Frozen TestEnvironmentSeed/GraphRefBindings and TestNativeSnapshotNormalizer_Concepts/GraphRefBindings; `go test -race ./cmd/scenariobench -run '^TestEnvironmentSeed$'` and `go test -race ./internal/benchenvnative -run '^TestNativeSnapshotNormalizer_Concepts$'`; frozen C0 TestEnvironmentDispatch_PlanForms / _ExecutionInputs; `go test -race ./cmd/scenariobench -run '^TestEnvironmentDispatch_(PlanForms\|ExecutionInputs)$'` | ENV-C5/native owners must accept the exact role/root/records/result and twelve member licence bindings, with eight receipts per head and healthy/missing/stale assertions above. Generic spy reachability alone cannot prove the graph recipe. |
| Runnable bounded reading after all dependencies land | ENV-G1 entrypoint `go run ./cmd/scenariobench env-check -seed graph` with explicit ignored-workspace output root accepted by the frozen command grammar | Unavailable now. C5 requires implemented frozen invocation transport and native receipts; do not run, materialize, stamp a verdict or treat the data ledger as a checker. |

For EACH environment/full stratum, C5's TestEnvironmentSeed acceptance must
exercise three independent concept-removal twins with healthy controls, using
the same pinned catalog and authoritative requirements. Remove only the chosen
concept from its claim to require missing_concept. Remove its independently
verified population to require unknown_evidence or the exact known-zero counts
according to what was actually checked; never preserve fabricated good samples.
These raw native twin projections must be supplied by the native owner, not
inferred from aggregate fake-port counts.

Separately freeze exact one-short projections: dropping one common positive
from30 pairs yields29/29 and both shortages; dropping one negative yields30/29
and only the negative shortage. cross_file_paths29, depth2 and only2 product
files each fail that concept. fanin_symbols2 (including a target with4 calls or
only2 units) and dispatch_sites29 each fail. repository_links29,
linked_repositories2, linked_services2 and nonlinks29 each fail independently.
Keep unrelated populations healthy, preserve the one-to-one matching coupling,
and assert all simultaneous shortages using ENV-2's exact diagnostic grammar.
Replicas, altered pins, moved aliases and seed changes cannot inflate origins
or environment count. The raw bank is not executable until those exact
projections and byte bindings have native acceptance.

Full coverage cannot PASS by omitting authoritative inventory/group requirements or by treating the
two seed runs as two environments. Both candidate families must independently
qualify under ENV-2's verified ancestry law. If a selected Card's authoritative
requirements include any non-G1 concept, this bank alone cannot pass that card.
Never remove those requirements to manufacture a graph-only card. A locally healthy concept result
with insufficient environments remains the actual INCOMPLETE/UNFIT coverage
result, with all reasons retained. This draft grants no local-PASS shortcut.
A witnessed execution may eventually support the plan's PASS/UNFIT line under
C5; until the result-to-reading adapter is frozen, no new fold/status rule is
created here. Coverage is never BEN-5 admission or a public effect verdict.

## Exact dependency and wiring requests

| Durable request | Required upstream deliverable before runnable graph proof |
|---|---|
| ENV-G1/DEP1 | The exact cleared W18/T12 source has bounded documentary LAND/W2-3 approval only. Obtain fresh independent review of this two-preset alignment and controller exact-source final review before any integration. Implement the schema/leaf/producer/C5 dependencies only under later grants. Concept inventory uses existing schema/vet and its exact-file path grant, without concept REC machinery; other authored ENV records retain U1. Full ENV-1 CLI/provenance remains HELD; old104 is superseded and ENV remains OUT8. |
| ENV-G1/DEP2 | G1 data owner authors the exact two-family per-file inputs and native owners verify bytes, package resolution, source-ID/path/hash map, independent origin/match inventory and full commit/truth hashes. Supply the already frozen F1 component paths/bindings; no layout or ownership decision remains delegated here. No copied template gets an assumed independent root. |
| ENV-G1/DEP3 | Authoritative inventory owner supplies the one cited `calibration/environments/concepts.cue` with reviewed per-key card/design citations, complete Card.ID/CheckerContract groups and exact byte pin, under DEP1's trusted grant. BEN-3/CTR-13 owner supplies actual Card.ID, Card/CheckerContract/Catalog bytes and digests, approved full strata, checker pins and complete T3 mapping with CheckCardClosure and the amended core inventory/join validators. Requirements/catalog close over that entire source, never just G1's three selected concepts or a second roster. No delivered inventory/card/mapping was found at the frozen base; graph/impact/fleet titles cannot substitute. |
| ENV-G1/DEP4 | Native owners accept ENV-2's already requested snapshot normalizer grant/fixtures and graph-specific raw compiler/path/product, D26, protocol/non-link and lineage acceptance. Accept the twenty original K output paths plus twelve exact per-member licence receipt paths, complete inventory/member licence coverage, and all GraphRefBindings assertions in the existing requested normalizer files. Supply existing native format bytes, actual pins/digests and immutable projections for healthy, missing/stale licence and every removal/one-short twin. Missing bytes or owner grants remain dependencies; no new ENV oracle or author-labelled native schema is authorized. |
| ENV-G1/DEP5 | ENV-C5 owner implements the frozen F1 role/root/assertion table in its command-local graph loader, binding seed-7.json by default, seed-8.json in repeatability proof, Sources->raw map, exact Truth role IDs->EnvironmentRecords, CoverageRequest{Tier:constructed, Purpose:coverage, At:2000-01-01T00:00:00Z}, roots/limits, two distinct EnvironmentHeads, native preparation, actual CheckCardClosure/full-ID pure join, PrepareCoverageSnapshot and pure CheckEnvironmentCoverage(inventory, snapshot). Retain existing handler/loader signatures, both CLI forms, typed errors and original CoverageResult. Freeze exact result/artifact-to-seed-graph.md/json mapping. No new public type or source edit is granted here. |
| ENV-G1/DEP6 | After DEP1-5, including approved amended core and reviewed pinned inventory, data owner completes hashes, arithmetic, all refs/receipts and expected.md under the exact granted file list, then executes only the released C5/Linux reading proof. Native actual counts, conformance, immutable bindings and independent environments stay UNPROVEN until that run. |

These are dependencies and missing artifacts, not evidence that native tools fail.
There is no newly identified ungoverned policy choice requiring an invented
alternative. In particular30/30, subtype treatment, two independent families,
workspace containment and neutral Git identity already have governing text.
Any later genuine ungoverned choice must be escalated with decision,
recommendation, one alternative and governing gap in <=300 characters; it cannot
be settled by a new field or an author oracle. The exact per-file grant does not waive
those upstream dependencies.

## Documentation proof

This lane runs document conservation, cited-source/API checks, source27/key
conservation and recipe shape arithmetic only. It runs no Go/native test, no
compiler/SCIP/protocol producer, no generator or env-check command, no admission,
no heavy/full gate, no model call and no remote write/restart. Named acceptance
commands above are future obligations. No reading or actual count is certified.
A separate critic must assess this draft and its explicit gaps before W2-3
approval; the author does not self-award LAND.

## Decision models

None applies. Existing deterministic native facts and
verified snapshots provide truth with provider none. No judgment site or model
primary label is introduced.
