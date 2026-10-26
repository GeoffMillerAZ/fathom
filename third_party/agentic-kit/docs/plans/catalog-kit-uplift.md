# Catalog Kit uplift

Owner request: 2026-10-20, expanded 2026-10-21. Decision: 0023.
Specification: [catalog-workflow](../designs/catalog-workflow.md).
Assessment: `docs/assessments/2026-10-20-catalog-kit.md`.

## Delivery status

The source contribution at `89fbe33` is integrated and strengthened with exact
criterion-definition binding, named profiles, strict Unicode decoding and CLI
search/doctor/help. Boxes below require Fathom proof plus the repository gate.
Supporting evidence includes schema/skill checks and isolated Review Kit/Env Kit
pilots. Production migration and model quality are separate from this kit delivery.

## Implementation work order

- [x] Explicit DAG resolution, source pins, stable identity, bounds and contradiction refusal [CK-S1] {id: CK-1, symbol: Resolve, package: kits/catalog-kit}
- [x] Receiver grants, compare-and-set patches, zero values and change provenance [CK-S2] {id: CK-2, symbol: Resolve, package: kits/catalog-kit}
- [x] Strict input decoding and complete bundle integrity [CK-S4] {id: CK-3, symbol: DecodeBundle, package: kits/catalog-kit}
- [x] Exact definition/evidence-bound preparation [CK-S5] {id: CK-4, symbol: PrepareDecision, package: kits/catalog-kit}
- [x] Version/provider/model/adapter/policy-bound reuse identity [CK-S6] {id: CK-5, symbol: DecisionCacheKey, package: kits/catalog-kit}
- [x] CLI resolve, explain, query, diff and prepare refusal paths [CK-S7] {id: CK-6, symbol: run, package: cmd/catalogctl}
- [x] Deterministic analytics projection [CK-S8] {id: CK-7, symbol: ExportNDJSON, package: kits/catalog-kit}
- [x] Existing Go inventory exact source coverage and permutation determinism [CK-S8] {id: CK-16, symbol: ResolveBundle, package: cmd/internal/catalog}
- [x] Named profile dependency closure and complete loaded-input validation [CK-S3] {id: CK-17, symbol: ResolveProfile, package: kits/catalog-kit}
- [x] Search, doctor, help, exit codes and full provenance explanation [CK-S7] {id: CK-18, symbol: execute, package: cmd/catalogctl}

CK-8 is the supporting skills deliverable: four structurally validated skills and
an independent read-only diagnostic trial, with no enforcement claim. CK-9 and
CK-10 are bounded reference pilots delivered under `kits/catalog-kit/pilots/`:
actual domain loaders, metadata/sidecar reconciliation and refusal suites at an
exact recorded platform-repo revision. These are external test templates,
so their execution receipts carry proof rather than the kit's Go call graph.

## Outside this delivery

- [-] Production Review Kit decision-transport migration [CK-S6] {id: CK-11, disposition: deferred, reason: "consumer-owned adoption is separate from kit delivery", trigger: "Review Kit admits its transport migration with consent and baseline cases"}
- [-] Provider confidence/version migration {id: CK-12, disposition: deferred, reason: "decision 0018 remains binding; no provider migration is needed here", trigger: "a consumer workload evaluation supports a separately reviewed contract migration"}
- [-] Shared alias/lifecycle/relation/exception protocol {id: CK-13, disposition: deferred, reason: "domain responsibilities and extension triggers are documented in the specification", trigger: "a named consumer demonstrates a shared link history or exception failure"}
- [-] Signed distribution and atomic activation {id: CK-14, disposition: deferred, reason: "local source composition requires no registry or publishing protocol", trigger: "a named consumer requires authenticated distribution beyond local pins"}
- [-] Paid native decision-lane quality study {id: CK-15, disposition: deferred, reason: "offline kit delivery grants no live-spend or calibration claim", trigger: "a consumer authorizes its budget and independently adjudicated workload study"}

## Verification

Current evidence: [integration record](../evidence/catalog-kit/2026-10-21/verification.md).
Original candidate evidence is historical: `../evidence/catalog-kit/2026-10-20/`.

```sh
cue vet ./kits/catalog-kit/...
go test -race ./kits/catalog-kit ./cmd/catalogctl ./cmd/internal/catalog ./foundations/knowledge/catalog
python3 -m unittest discover -s kits/catalog-kit/scripts -p '*_test.py'
bash kits/catalog-kit/pilots/run.sh
task validate
task downstream-check
fathom verify --service agentic-kit --plan docs/plans/catalog-kit-uplift.md --strict --require-all
```

The optional Python lane requires PyArrow. Downstream checks run in disposable
sibling snapshots because the existing consumer-c task re-pins/re-vendors its consumer.
Missing consumers remain untested. An unavailable/stale index is not a proof pass.
