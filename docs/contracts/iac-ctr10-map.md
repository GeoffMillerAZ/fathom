# IaC approved-contract reuse and reservations: #303/#308/#311/#312/#313

Status: complete contract draft for independent criticism, not dispatch approval.
Source base: `d9eef4794afdbc200d1b8697ce121d9d688fedaf`. Only this contract is changed.
No implementation, raw population, native run, measured RED, gate or row claim.
`READY` means the immutable draft is ready to review. Input/grant holds below remain.

## Governing intent

Router: `docs/intent/README.md`; P2/P3/P20/P22; I38/I56/I62;
ADR-091 decisions 2/4/6/7, ADR-092 decisions 7/8/10, ADR-099,
ADR-106 decisions 1/2. Catalogs are local read-only inputs, values remain
withheld in the product, plans are bench-only, no added door package, table,
cache, hook or tool. No kernel or future evaluator is commissioned. W2-1
settles the fired LL-M1 trigger at 80 door / 81 consumer packages with no
headroom. No evidence here fires the other revisit triggers. F97's broader
identity stays deferred. Additive API/JSON changes retain compatibility review.
W3-1/4 house rules apply to additions: named closed types/constants, comparable
structured identity, explicit snake_case JSON, recursively nonnil result slices,
context only for I/O and typed refusal/limit errors. No future fields.

## Approved source, authority and decisions

Reuse `docs/contracts/ctr-10.md` at exact commit
`26ffbfedae78c6486229b4558c14849421d52664`, branch
`lane/contract-ctr-10`, base `468dc705813382d584926887b451cbea145c76ac`.
Source: contracts clone's current CONTRACT-BOARD entry and exact READY marker,
`out/CONTRACT-ctr-10.md`, `out/APPROVED-ctr-10`, W3-1/2/3/4. This version is
W3-approved with the C1-C5 changes. It is a contract, not landed implementation.
Read it with immutable `git show`, not whatever branch is checked out.
This document supplies the requested reuse map, not a competing API draft.

- D1 Recommended: incorporate approved shapes, ports, four route proofs and
  serial reservations by exact commit. Alternative: create another uncertainty,
  change or hazard API, rejected by W3-1 and design §9 E8.
- D2 Recommended: HZ-2 integrated/proven before HZ-3; return maintenance after
  HZ-3 without making that return a dependency. Alternative reversed grant
  creates a prerequisite cycle. Governing: CTR-10 Topological handoff assertion,
  W3-1 U2 and HZ-3 needs HZ-2.
- D3 Recommended: H2/H5 remain advice, accepted stateful entries declared context,
  conditional replacement unknown. Alternative: promote inference/list/conditional
  to a proven contract fact, rejected by W3-1 U4, HZ-9 and ADR-106.
- D4 Recommended: keep HZ-8 as IF-13 alias and IF-17 as H6 input owner.
  Alternative: rebuild the catalog scan or upgrade diff in hazard lanes,
  rejected by design §9 E8 and HZ-8/HZ-11 authored rows.
- D5 Recommended: preserve every existing review dimension and add iac exactly
  once. Alternative: remove dimensions to force the old phrase seventh,
  rejected by additive W3-4. At this base review.Run already builds eight
  dimensions; the row's ordinal is stale, not permission to drop any.

## Coverage map: exact approved sections, remaining prerequisites

| Gap | Exact CTR-10 coverage | Remaining dispatch prerequisites |
|---|---|---|
| #303 HZ-3 | Proposed shapes UnknownReason/ReadEvidence; Concrete internal copies ReadObservation/BuildReadEvidence and conversions; Four mandatory HZ-3 route proofs; shared-file table | Land approved contract and implement HZ-2 first; independent exact source/file handback from #164/#167/#168; implementer real RED for all four routes and TestLaneFactUnknowns; reader evidence not yet implemented. |
| #308 HZ-10 | CheckRenameWithoutMoved/CheckMultiplicityWithoutMoved/CheckProvabilityLost; structured Address; H2/H5 advice; catalog-free hazard JSON and TestLaneHazardFree | Proven HZ-4/#268 and HZ-6/#269; exact change types and source spans; LIB-7 binary owner #307 separate; no need to wait for IF-13; no source grant to #164 CLI or MCP files. |
| #311 HZ-2 | HazardDimension projection Checked/Reason/AsOf/Findings; registered-handler handoff; TestLaneHazardReviewDimension; HZ-2-before-HZ-3 topology | HZ-1 result/stop cleared; #167/#168 facts/replacement turn handback; exact schema/projection and affected test grant; architect-owned change note; no wait for HZ-3. |
| #312 HZ-8/HZ-9/LIB-9 | StatefulType/isStatefulType/CheckStatefulReplacement; ReplacementFact and typed class; stateful fixture; TestLaneHazardStateful | Actual IF-13 fact/source-scan handoff from #168, HZ-6, LIB-7 as issue requires; person-accepted baseline with provenance and independent agreement reading; HZ-M1 separately gates surfaces at 0.9. |
| #313 HZ-11 | VersionFacts/UpgradeHazards; same-source explicit versions; TestLaneHazardUpgrades; #168 owns IF-17 preparation | IF-8/IF-17 handback, two real local pinned catalogs and actual attributes-in-use input; retain fixture reading owner; no dependency on publication IF-6. |

No row is checked and no reservation is inferred installed. All five gaps are
contract-covered by the cited source. Their missing implementation/input/grant
work is not silently closed. IF-13 and IaCUpgradeImpact are absent in this base;
new source signatures for those upstream implementations cannot be fabricated.
The approved ReplacementFact/VersionFacts ports are consumer contracts; #168
must supply the actual adapters and exact tests through its own handback.

## Exclusive paths and serial turns

Full normative table is CTR-10 Proposed exact shared-file reservation manifest.
This is its application to the five gaps, with ownership still conditional on
integrator confirmation against the actual #164/#166/#167/#168 manifests:

- `internal/iacfacts/resource.go`: #164 -> #303 -> #268 -> #269.
  Existing `read_terraform.go`, `read_route.go`, `read_cloudformation.go`,
  `read_pulumi.go`: #164 -> #303 -> #268, all under internal/iacfacts.
- `internal/iacfacts/check_module.go`: #167 -> #303. #303 solely adds
  read evidence; it cannot rewrite checks. `read_evidence.go` and
  `read_evidence_test.go` are #303's new exact files.
- `pkg/lang/iac/types.go`, `read.go`, `convert.go`: #303 -> #268 -> #269.
  `read_test.go` test transfer is CTR-10 -> #303, read-only thereafter.
  #303 owns new `pkg/lang/iac/read_evidence_test.go` and
  `cmd/fathom-lang/resources_evidence_test.go`, and an explicit route-proof
  turn in existing `cmd/fathom-lang/resources.go`.
- `internal/mcp/iac_facts.go`, `iac_cfn.go`: #167 -> #168 -> #311
  integrated/proven -> #303 evidence -> #311 maintenance return.
  `internal/mcp/review_tool.go`: #167 -> #168 -> #311; #303 reads that
  registered route without a second handler write turn.
  #311 owns new `review_iac_dimension.go` and its `_test.go`; #303 owns
  new `internal/mcp/iac_read_evidence_test.go`.
- #166 retains `internal/iacfacts/binding.go`, `binding_hcl.go`,
  `binding_json.go`; these five consumers receive no write permission.
- #168 retains `internal/iacfacts/catalog_build.go`,
  `cmd/iaccatalog/build.go`, `export.go`, and the approved future
  `internal/iacfacts/cascade.go`/`cascade_test.go`. #312/#313 read the
  handback only. IF-17 actual preparation files remain a required #168 grant,
  not a guessed filename. No lane adds a duplicate scan/upgrade resolver.
- #269 owns new `internal/iacfacts/change_types.go`, `pkg/lang/iac/diff.go`
  and `diff_test.go`. #308 consumes immutable ChangeSet/Block types in its
  only new files `internal/iacfacts/hazard_free.go`/`hazard_free_test.go`.
- #312 owns only `internal/iacfacts/hazard_stateful.go`/`hazard_stateful_test.go`,
  `calibration/iac/stateful-types.json`, and
  `docs/bench/readings/library-roadmap/lib-9-stateful-candidates.md`, plus
  the explicitly transferred CTR-10 fixture
  `internal/iacfacts/testdata/contracts/stateful.json`.
- #313 owns only `internal/iacfacts/upgrade_hazards.go`/`upgrade_hazards_test.go`
  and `docs/bench/readings/change-hazards/h6-upgrade-fixtures.md`.
- #307 separately owns the hazard entry; `cmd/fathom-lang/main.go` switch
  turn follows the existing resources owner. No command/hook transfer to
  #308/#312/#313. Exact fixtures stay with CTR-10's named owners; consumers
  cannot create another testdata namespace by implication.

Source grounding: existing review_tool.go registerReviewDiff invokes
reviewIaCFindings and reviewCFNFindings only outside as_of; current missing
catalog returns no iac payload. Existing review.Dimension, Verdict.Payload
and Dimension.payload already carry checked/reason/findings, so reuse that
projection instead of a parallel engine schema. Existing Run's order is
impact, obligations, arch_conformance, spec_coverage, test_coverage,
certifications, integrity, conservation. Preserve it and append one iac.
Summary counts must derive from the resulting slice. #311's new adapter can
translate CTR-10 HazardDimension to existing Dimension; no internal/analysis/
review source grant is inferred. Existing fact findings remain in the iac
payload while the dimension states whether that computation ran.

Request A's exclusive integration/test turn for existing
`internal/mcp/review_tool_test.go`, `iac_facts_test.go`, `iac_cfn_test.go` and
`review_compat_test.go` if their assertions need updating. No test is renamed
or dropped to force seven. Schema/change-note/API snapshot edits remain with
the architect; absence of a named schema file is not a grant to invent one.
The exact requests are `out/REQUEST-iac-reservations.md`.

## Exact acceptance assertions by consumer

The approved test names below are incorporated, with every assertion retained.
They are prospective and unrun. The implementing critic checks each assertion,
not only whether a matching test function exists.

| File / exact name | Every required assertion |
|---|---|
| internal/iacfacts/read_evidence_test.go / TestBuildReadEvidence | Every actual input, including defaults-only, appears; reasons and closed actions exact; Read/Unread sorted/disjoint; reordering byte-identical; incomplete syntax in Unread with reason, never closed completeness. |
| pkg/lang/iac/read_evidence_test.go / TestResourceReadEvidence | Terraform file/module, CloudFormation and Pulumi unresolved fixtures preserve all paths/reasons/actions through public convert; values absent by default. |
| cmd/fathom-lang/resources_evidence_test.go / TestResourcesCarryReadEvidence | Same pinned module via supervised and in-process command paths produces equal evidence, exact relative inputs including unchanged defaults-only file and all unknown reasons/actions; no default values. |
| internal/mcp/iac_read_evidence_test.go / TestReviewDiffCarriesReadEvidence | Registered review_diff through proven HZ-2 keeps both module/template evidence; defaults-only file remains even outside changed_files; no attribute values. Deliberately dropping a projection must fail this proof. |
| pkg/lang/iac/read_test.go / TestLaneFactUnknowns | Every unknown/expression has reason and only closed action; read/unread disjoint; WithValues cannot change completeness; all library/binary/MCP routes preserve evidence. |
| internal/mcp/review_iac_dimension_test.go / TestLaneHazardReviewDimension | Registered handler, exactly one appended iac, all old dimensions retained; checked=false implies reason; absent catalog and as_of explicitly unchecked with nonempty reason and caller revision; complete current input checked with empty reason; returned fact citations resolve to exact catalog bytes; timeout/unreadable cannot be clean; all empty findings arrays []; preserve byte cap and no values. |
| internal/iacfacts/hazard_free_test.go / TestLaneHazardFree | H2 candidate rename advice; H2/H4 suppressed only for exact covering moved pair; unrelated moved never suppresses; count/for_each kind or key-expression change yields H4; literal->expression/unknown H5 advice; order deterministic; incomplete/missing-source evidence never clean. |
| internal/iacfacts/hazard_stateful_test.go / TestLaneHazardStateful | H1 needs definite replacement AND accepted matching tool/type; rejected/unaccepted/wrong-tool/nonstateful yields no proven H1; conditional/unknown never proven; source and person acceptance retained; evidence class declared context cannot become catalog fact; source/list agreement reading separate from replay. |
| internal/iacfacts/upgrade_hazards_test.go / TestLaneHazardUpgrades | Two exact same-source catalog versions, exact attrs-in-use and citations; changed relevant replacement fact yields H6, unused changes don't; missing version/fact or source mismatch yields unknown, never nearest-version inference; #168 preparation routed into approved port; deterministic, no values. |
| pkg/lang/iac/hazard_contract_test.go / TestContractsFactsAPIHouseRules | Named constants; errors.As refusal/limit; Address comparable map key and exact five-key object, module-sensitive field order; Sources omitted with canary; every recursive result slice [] not null; reordered exact JSON; no added door package. |
| internal/iacfacts/hazard_contract_test.go / TestContractFactsHandoffDAG | All baseline/hz2-green/hz3-green/maintenance nodes sort; HZ-2 before every #303 route proof; no hz3->hz2 path; injected reversed edge causes cycle error; checker cannot discard plan prerequisite. |

Commands use `go test -race <own-package> -run '^<exact-name>$' -count=1`
for every listed name under the shared lease, then the actual consumer route
and plan verify on an isolated source-built scratch index. Implementers write
real failing assertions first under W3-2. Public API implementations are
architect class with go doc snapshot diff <=80 lines per W3-3; documentation
approval cannot substitute. No measured RED/GREEN is claimed here.

## Boundary and input holds

No fresh generic API choice is open for these five issues. CTR-10 resolves
the shapes. Exact upstream manifests/committed handbacks, independently proven
entry rows, accepted stateful input and actual IF-17 adapter remain missing.
Their delivery is operational work; a new approval marker cannot make it true.
No raw facts, accepted entries or replay precision have been invented.

## Decision models

None. Fact projection and declared provenance are deterministic; provider none.
Opt out by withholding the input and reporting why no answer was earned.
