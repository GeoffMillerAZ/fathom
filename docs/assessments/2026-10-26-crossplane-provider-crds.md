# Crossplane provider CRD calibration

## Verdict

The nine-subject native before/after probes agree exactly, including their raw
report digest. The product control changes actual extracted output between them. No new native score or support promotion is claimed. The
historical reading retains 352/354 matched definitions, 63/63 references and
27/27 cross-file references. P-T1 corrects the inherited definition tier from A
to C because the older Go oracle selects nested definition candidates with an
authored walk. Native schema validation accepts candidates but does not enumerate
that population. Provider schema acceptance does not prove reference or field binding.

## Scope and intent

IAC-19 / GAP1-34 uses provider CRDs as calibration-only oracle inputs. ADR-048
and I54 require measured support; I56 prohibits an implicit product downloader.
The owner authorized the provider-schema fetch trigger. F25 remains deferred.
No product engine, dependency layer, queue, generated board or plan box changes.
The sole canonical shard remains `calibration/corpus.d/crossplane.yaml`.

The original three canonical subjects are preserved. A legacy AWS platform,
Azure/GCP platforms and three older network composition/XRD subjects add
exploratory probes, each pinned before new measurement. The first six subjects
had zero matching Upbound bases; three network subjects were added and all initial
subjects were retained. `calibration/oracles/crossplane/pins-r1.json` records
identities, selection criteria and the peeled legacy tag. These are not holdouts.

## Native schema evidence

**Measured, 2026-10-26, before and after.** Crossplane CLI 2.5.0 `resource validate`
consumed local core CRDs, XRDs and provider CRDs with its image source disabled.
This is the current command corresponding to `beta validate`. Source archives for
Upbound AWS 2.0.0, Azure 1.10.0 and GCP 1.10.0 supplied 3,019 CRD files. Their
content does not establish OCI package equivalence. The pinned sources, tool
hashes and nine scopes are in `calibration/oracles/crossplane/pins-r1.json`;
the public counts and hashes are in the [calibration receipt](data/2026-10-26-crossplane-r1.json).

| Subject | YAML files | Compositions / XRDs | Literal bases | Native valid / invalid / missing schema |
|---|---:|---:|---:|---:|
| crossplane/crossplane | 316 | 34 / 34 | 8 | 201 / 26 / 52 |
| upbound/platform-ref-aws | 15 | 1 / 1 | 0 | 3 / 0 / 0 |
| upbound/configuration-aws-network | 11 | 1 / 1 | 0 | 3 / 0 / 0 |
| upbound/platform-ref-aws-legacy | 14 | 1 / 1 | 7 | 4 / 0 / 2 |
| upbound/platform-ref-azure | 15 | 1 / 1 | 0 | 3 / 0 / 0 |
| upbound/platform-ref-gcp | 16 | 1 / 1 | 0 | 3 / 0 / 0 |
| upbound/platform-ref-aws-network-resources | 2 | 1 / 1 | 16 | 17 / 1 / 0 |
| upbound/platform-ref-azure-network-resources | 2 | 1 / 1 | 3 | 4 / 1 / 0 |
| upbound/platform-ref-gcp-network-resources | 2 | 1 / 1 | 2 | 3 / 1 / 0 |
| **Total** | **393** | **42 / 42** | **36** | **241 / 29 / 54** |

This source inventory is separate from `lang-oracle` discovery and the historical
reading's 189 measured files. It includes schema checks of objects outside that
old scored file set; the 393 YAML files do not become new scored evidence.

The CLI made 282 validation calls and can emit multiple result rows per call;
the three status totals are result rows, not a 282-item partition. It also emitted
22 additional rows. Twenty-one literal bases matched provider CRDs (AWS 16,
Azure 3, GCP 2); all 21 validated. The other 15 bases had no matching Upbound
schema and remain in the denominator. The three legacy outer Compositions were
invalid under the 2.x Composition schema; all three remain counted. Three
unmodified provider examples validated, and all three unknown-field mutations
were rejected. These controls test schema sensitivity, not binding accuracy.

## Directional field paths

The probe treats `FromCompositeFieldPath` as reading the matching XRD schema and
`ToCompositeFieldPath` as reading the matching composed-resource schema. This is
an authored tier-C schema walk. Among 63 patches, it produced 41 directional
field-path diagnostics: 33 `declared`, one `notDeclared`, and seven `unmodeled`.
The `notDeclared` path is absent from the raw XRD; Crossplane can add fields to a
generated XR schema, so this is not a native invalid-field result. Twenty-two
patch forms were unsupported; one patchSet definition is counted separately.
Mixed files supply neither native input nor XRD schemas to this diagnostic. Neither the 41
diagnostics nor that counter proves a Function's runtime binding.

## Native oracle search, 2026-10-26

| Source group | Search / primary source | Result and disposition |
|---|---|---|
| Own toolchain | [CLI 2.5 command reference](https://docs.crossplane.io/cli/v2.5/command-reference/); [owner validation API](https://pkg.go.dev/github.com/crossplane/cli/v2@v2.5.0/pkg/validate) | Structured schema results, no static referent or patch-binding output. Native candidate under measurement. |
| SCIP / LSIF | [SCIP producer list](https://github.com/scip-code/scip#tools-using-scip), [Sourcegraph indexer guide](https://sourcegraph.com/docs/code-navigation/writing-an-indexer), [LSIF indexers](https://lsif.dev/); query `github scip-crossplane indexer` | No listed Crossplane producer found. GitHub repository search cache inaccessible. Bounded search miss, not proof of absence. |
| Language servers | [up xpls 0.34.2](https://pkg.go.dev/github.com/upbound/up@v0.34.2/cmd/up/xpls), [upstream feature issue](https://github.com/crossplane/crossplane/issues/4674); queries `site:github.com/upbound "language server" crossplane` and `site:docs.upbound.io "xpls"` | Existing Apache-2.0 candidate, untried here. Historical issue describes missing patch diagnostics; current behavior is not inferred from that issue. |
| Package registry / sibling APIs | [patch-and-transform input API](https://pkg.go.dev/github.com/crossplane-contrib/function-patch-and-transform/input/v1beta1), [0.8.2 native patch implementation](https://github.com/crossplane-contrib/function-patch-and-transform/blob/v0.8.2/patches.go) | Typed inputs and instance patch execution are candidates. They do not by themselves certify all static schema fields. Untried. |
| Runtime tracing / evaluation | Official CLI reference, `composition render` Development runtime and `resource trace` | Local gRPC Function evaluation can avoid containers, but is untried and executes a function. Cluster trace is outside the no-cluster probe. Neither is claimed unavailable. |

## Remaining ceiling

The native validator grades schema instances. It does not resolve XRD or Function
referents and does not execute directional patch expressions. The existing L2/L3
good ceiling is retained for the measured oracle, with the untried candidates
above explicitly open. A new probe of xpls or owner Function evaluation, with
native population and independent truth controls, is the next evidence trigger.
The existence of those tools is already known and is not a future trigger.

IAC-13's historical assumption that function runtimes require containers is too
broad: Development runtime can use a local process. This branch does not execute
that process or claim runtime evidence. A separate bounded evaluation can revisit
the deferral without adding product behavior.

## Reproduction and controls

See `calibration/oracles/crossplane/README.md` for committed reproduction commands.
Both reports include source and tool hashes. Archive-derived CRD digests match
the local provider manifests. All nine exact tracked-set manifests, including
symlinks and gitlinks, agree before and after; all untracked/ignored inventories
are empty. The public receipt contains only counts, hashes and repository pins.

The product control removed one actual CrossplaneStep while retaining one root
CrossplaneResource and seven composed resources. Subject bytes were unchanged.
The two native receipts and raw-report digests agree exactly. This proves response
to the chosen perturbation without claiming a native binding certificate.

Harness `lang-regrade` at `6db920e72` (full identity in the receipt) computed
L1/L2/L3 good after the tier correction. All historical counts, metrics, required
kinds, tenancy, date and extraction commit are conserved. This is a metadata
correction, so no new throughput measurement is represented. Only the Crossplane
reading changes among 185 reading files; the other 184 and all 31 baseline/parity
files remain byte-identical, with required kinds preserved for all 154 eligible
readings.

The population-matching mutation failed as expected: bypassing required report
identity made `test_report_requires_identity_and_reconciles` fail with
`AssertionError: Refusal not raised`. The restored suite passes. The oracle-tier
regression failed before the correction with `tier = "A", want C`; the full Go
oracle race suite then passed with the native offline integration test enabled.
The mixed-file schema regression also failed before its fix and passes afterward.
Ten Python tests, strict reading round-trip, exact X-R3, release and merge-debris
checks pass. The controls and log hashes are in the public receipt.

Native and Go jobs ran serially at verified nice 10, with launch load at most 9,
Go 1.27.3, GOMAXPROCS=2 and GOFLAGS=-p=2 on the shared Apple Silicon helper.
Concurrent host jobs were not recorded. The controller owns full merged-head
`task validate`, verification, Linux CI and integration. No plan box is changed.
