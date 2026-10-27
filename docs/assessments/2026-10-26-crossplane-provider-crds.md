# Crossplane provider CRD calibration

## Verdict

The nine-subject native before/after probes agree exactly, including their raw
report digest. The product control changes actual extracted output between them.
No new native score or support promotion is claimed. P-T1 replaces the inherited
189-file, validator-filtered tier-A population with a full independent tier-C
walk over 244 canonical files: 428/428 definitions, 86/86 references and 33/33
cross-file references, with L1/L2/L3 good. Native schema validation accepts
instances but does not enumerate nested definitions or prove reference or field
binding. The L2/L3 native-oracle question remains open; no ceiling is established.

## Scope and intent

IAC-19 / GAP1-34 uses provider CRDs as calibration-only oracle inputs. ADR-048
and I54 require measured support; I56 prohibits an implicit product downloader.
The owner authorized the provider-schema fetch trigger. F25 remains deferred.
No product engine, dependency layer or generated board changes. The Crossplane
queue records the independent tier-C walk as its oracle. P-T1 withdraws the
historical LB-F157 tier-A claim and uses a separate, unchecked qualification row.
The sole canonical shard remains `calibration/corpus.d/crossplane.yaml`.

The original three canonical subjects are preserved. A legacy AWS platform,
Azure/GCP platforms and three older network composition/XRD subjects add
exploratory native probes. The first six subjects had zero matching Upbound
bases. All 21 matched provider bases come from the three network subjects added
after that result. Their release versions were manually chosen as known legacy
`spec.resources` examples, with no preregistered version rule. All first-six
subjects remain. `calibration/oracles/crossplane/pins-r1.json` records the pins
and the peeled legacy tag. These are not holdouts.

## Native schema evidence

**Measured, 2026-10-26, before and after.** Crossplane CLI 2.5.0 `resource validate`
consumed local core CRDs, XRDs and provider CRDs with its image source disabled.
This is the current command corresponding to `beta validate`. Source archives for
Upbound AWS 2.0.0, Azure 1.10.0 and GCP 1.10.0 supplied 3,019 CRD files. Their
content does not establish OCI package equivalence. The pinned sources, tool
hashes and nine scopes are in `calibration/oracles/crossplane/pins-r1.json`;
the public counts and hashes are in the [calibration receipt](data/2026-10-26-crossplane-r1.json).
The [R2 receipt](data/2026-10-26-crossplane-r2.json) records the full-population
grade and bounded owner-tool probes.

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

This source inventory is separate from `lang-oracle` discovery. The native probe
inspected 393 YAML files; the canonical grade scored 244 Crossplane files from
the original three subjects. Native schema rows do not become scored tier-A
definition, reference or cross-file evidence.

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
generated XR schema, so this is not a native invalid-field result. The owner CLI's
[`crossplane xrd convert`](https://docs.crossplane.io/cli/v2.5/command-reference/#crossplane-xrd-convert)
can generate that XR CRD and is an untried route to resolve this one diagnostic. Twenty-two
patch forms were unsupported; one patchSet definition is counted separately.
Mixed files supply neither native input nor XRD schemas to this diagnostic. Neither the 41
diagnostics nor that counter proves a Function's runtime binding.

## Native oracle search, 2026-10-26

| Source group | Search / primary source | Result and disposition |
|---|---|---|
| Own toolchain | [CLI 2.5 command reference](https://docs.crossplane.io/cli/v2.5/command-reference/); [owner validation API](https://pkg.go.dev/github.com/crossplane/cli/v2@v2.5.0/pkg/validate) | `resource validate` gives schema-instance results, no referent or patch-binding certificate. `composition render` rejects all three exact legacy `spec.resources` Compositions as requiring Pipeline mode before starting a Function or container. `xpkg build` succeeds on both actual package roots and fails on two non-package workflow roots; package metadata is therefore not categorically ungradeable. |
| SCIP / LSIF | [SCIP producer list](https://github.com/scip-code/scip#tools-using-scip), [Sourcegraph indexer guide](https://sourcegraph.com/docs/code-navigation/writing-an-indexer), [LSIF indexers](https://lsif.dev/); query `github scip-crossplane indexer` | No listed Crossplane producer found. GitHub repository search cache inaccessible. Bounded search miss, not proof of absence. |
| Language servers | [up xpls 0.34.2](https://pkg.go.dev/github.com/upbound/up@v0.34.2/cmd/up/xpls), [upstream feature issue](https://github.com/crossplane/crossplane/issues/4674) | Initialized for all nine subjects and opened 393 YAML files; 180 diagnostic entries were observed. Initialization advertised only `textDocumentSync: 2`, with no definition or binding capability. Diagnostics do not certify L2/L3 edges. |
| Package registry / sibling APIs | [patch-and-transform input API](https://pkg.go.dev/github.com/crossplane-contrib/function-patch-and-transform/input/v1beta1), [0.8.2 native patch implementation](https://github.com/crossplane-contrib/function-patch-and-transform/blob/v0.8.2/patches.go) | The Function runtime was not started: exact legacy CLI inputs fail before it. The owner's [Crossplane 1.20.0 `beta convert pipeline-composition`](https://docs.crossplane.io/cli/v2.2/command-reference/#beta-convert-pipeline-composition) can translate those inputs before local Pipeline rendering; its [official Darwin arm64 checksum](https://releases.crossplane.io/stable/v1.20.0/bin/darwin_arm64/crank.sha256) is `8ea4ee891e3f76db262c10964dfe539026975af47511f1c1ac92cb812072ee4d`. This route is untried and would yield tier B at most for the executed population. A compatible local Pipeline controller plus Function evaluation remains open. |
| Upbound CLI | `up` 0.34.2 and 0.38.0, `up composition render` / `up test` | In 0.34.2 the requested render/test commands are absent. In 0.38.0, render on all three exact legacy subjects stops for missing `upbound.yaml`; a pinned modern configuration renderer panics on nil metadata. `up test` dry-run exits 0 but prints `No test files`, so it is not a passing test. |
| Runtime tracing / evaluation | Official CLI reference, `composition render` Development runtime and `resource trace` | Development runtime can use a local process; exact legacy inputs fail before it. Cluster trace is outside the no-cluster probe. Neither route supplies a binding certificate here. |

## Canonical tier-C subjects

| Subject | Files | Definitions matched / truth | References found / truth | Cross-file references found / truth | Tiers |
|---|---:|---:|---:|---:|---|
| crossplane/crossplane | 238 | 414 / 414 | 80 / 80 | 31 / 31 | C |
| upbound/platform-ref-aws | 3 | 7 / 7 | 3 / 3 | 1 / 1 | C |
| upbound/configuration-aws-network | 3 | 7 / 7 | 3 / 3 | 1 / 1 | C |
| **Total** | **244** | **428 / 428** | **86 / 86** | **33 / 33** | **C** |

Every subject has precision and recall 1 in the six measured accuracy metrics,
with no extraction faults. Each pooled L3 denominator is 33, above D19's floor
of 20. Dropping the core subject leaves only two cross-file references, so this
is concentrated reference-example evidence, not an independent broad holdout.

## Open L2/L3 question

The full tier-C walk measures L2/L3 good. The native validator grades schema
instances but does not resolve XRD or Function referents or execute directional
patch expressions. The bounded probes above found no native binding certificate;
they also do not establish an exhaustive ceiling. A compatible local Pipeline
controller and Function evaluation, with independent population and truth
controls, remains a concrete untried route. Owner conversion by Crossplane
1.20.0 would be owner-translated input, rather than an authored rewrite, but
no conversion or Function execution is claimed here. The L2/L3 tier question is open.

IAC-13's historical assumption that function runtimes require containers is too
broad: Development runtime can use a local process. This work did not execute
that process or claim runtime evidence.

## Reproduction and controls

See `calibration/oracles/crossplane/README.md` for committed reproduction commands.
Both reports include source and tool hashes. Archive-derived CRD digests match
the local provider manifests. All nine exact tracked-set manifests, including
symlinks and gitlinks, agree before and after; all untracked/ignored inventories
are empty. The public receipt contains only counts, hashes and repository pins.

The product control removed one actual CrossplaneStep while retaining one root
CrossplaneResource and seven composed resources. Subject bytes were unchanged.
The two native receipts and raw-report digests agree exactly. This proves response
to the chosen perturbation without claiming a native binding certificate. The
before/after verification entries identify one combined pipeline log; the
product's own single-line stdout has a separate digest in the receipt. These
are not three independent command logs.

The harness ran `lang-oracle` three times at extraction commit `1b4c93a84`,
without `-tier-a` or a validator-based file filter. All 244 canonical files and
composed elements enter the independent tier-C walk. Each run matches 428/428
definitions, 86/86 references and 33/33 cross-file references. L1/L2/L3 are
good, with all six accuracy metrics at 1. CPU throughput was 2.86, 2.90 and
2.98 MB/CPU-s; the harness selected run 2 as the median. This changes the measured
population, counts and throughput; it is not a metadata-only regrade. Only the
Crossplane reading changes among 187 reading files; the other 186 and all 31
baseline/parity files remain byte-identical.

The population-matching mutation failed as expected: bypassing required report
identity made `test_report_requires_identity_and_reconciles` fail with
`AssertionError: Refusal not raised`. The restored suite passes. The oracle-tier
regression failed before the correction with `tier = "A", want C`; the full Go
oracle race suite then passed with the native offline integration test enabled.
The mixed-file schema regression also failed before its fix and passes afterward.
Twenty-five Python tests, strict reading round-trip, exact X-R3, release and merge-debris
checks pass. The public receipt retains controls, commands and statuses; raw-log digests
are withdrawn because private paths can enter tool output.

Native and Go jobs ran serially at verified nice 10, with launch load at most 9,
Go 1.27.3, GOMAXPROCS=2 and GOFLAGS=-p=2 on the shared Apple Silicon helper.
Concurrent host jobs were not recorded. The controller owns full merged-head
`task validate`, verification, Linux CI and integration.

Round 3 preserves LB-F157's original tier-A deliverable and contract, but marks
that claim withdrawn (`[-]`, disposition `refused`) under P-T1 because schema
acceptance did not enumerate nested definitions. It no longer reuses that ID for the P-T1 correction. The replacement
`LB-IAC-crossplane-pt1` has its own full-population evidence contract and remains
unchecked for the controller. The earlier one-row lexical verification in the
R2 receipt is historical and does not certify this replacement.

The archive-pin reconciliation independently fetched every subject/provider archive from its own repository
and peeled commit: all twelve now match their recorded digest and byte count.
Azure/GCP network archive metadata had been copied from other versions; both
are corrected. The legacy AWS archive pin also changed from its annotated tag
object to its peeled commit; all 29 relative entries and contents are identical.
The reproducible setup checker rejects a copied digest or wrong commit root.
The fresh-tree reproduction found six errors without `workspace/`;
the test modules now create the required scratch directory. All 27 tests pass,
including two new archive-identity checks. The [R3 receipt](data/2026-10-26-crossplane-r3.json)
records these results. Neither the canonical reading nor the native results
changed. The change note explicitly records the published L1 excellent-to-good downgrade.

The private R2 scratch ingest stored source text but failed typed Go loading for
340 of 1,266 packages because its importer supports export-data v4 while Go
1.27.3 emits v5. O1508 records that limitation; no full plan-gate pass is claimed.

## R-D19 reconciliation with main

Main `5d41806d0` is merged once. Its clean harness regenerates the canonical
reading from the frozen full-population lane counts through `lang-regrade`;
there is no native/extractor rerun and no hand merge of reading fields.
Precision and recall denominators are each 33, above D19's 20-reference floor;
this manifest sets no additional subject bar. Tier C still caps L1/L2/L3 at good.
No non-note value changes against the lane parent. Against main, the lane's
full 244-file walk preserves 428 definitions, 86 references and 33 cross-file
references, with all six accuracy tiers C; the reading note enumerates each
changed field. Frozen subjects, timings, raw counts, extraction identity and
required kinds are conserved. The new
[regrade receipt](data/2026-10-26-crossplane-d19-regrade.json) binds current
canonical bytes to exact main-harness output; earlier reading hashes bind only
the historical extraction snapshots.

Historical raw stream lengths/digests, private manifest hashes, local package
output hashes and raw-log digests have been withdrawn from public receipts.
They can encode private root identities even without literal paths. Existing
source pins, actual measured counts and command statuses remain. Private raw
owner-tool outputs stay ignored; they must not be copied into public evidence.

## Reading qualification contract

Under I-R7, `LB-IAC-crossplane-pt1` reads
`docs/bench/readings/lang/crossplane.json` directly. Its exact-match contract
requires the recorded grade block and level, all six accuracy values and tier-C
labels, pooled counts, all three subject pins/file counts and the required and
measured kinds. It does not certify a hand-authored qualification
sentence in a receipt. The production row remains unchecked.

The [contract controls](data/2026-10-27-crossplane-qualification-contract.json)
come from the actual main verifier and grader, through
`calibration/oracles/crossplane/qualification_contract.py`. Only an isolated
scratch copy is claimed. Grade-only, accuracy-only, tier-only and count-only
changes must fail strict verification. Dropping a file, subject or required/
measured element kind must also fail. The real regrade control removes all 33
found cross-file references from the subjects' counts, gives L3 unmet, and must
also fail. Original and restored inputs must pass. The canonical reading and
production plan are never written by the driver. The
[reproduction recipe](../../calibration/oracles/crossplane/README.md#reading-qualification-contract)
keeps its scratch store and ledger outside the canonical evidence population.

Public references to the archive-pin corrections, fresh-tree scratch setup,
queue oracle and withdrawn tier-A claim now name their substance or P-T1 and
`LB-F157`/`LB-IAC-crossplane-pt1`. Historical commands and measurements are
retained. G18's typed layer remains deferred and NP-Q5 remains the owner's call.

The driver also accepts a checked qualification row and keeps refusing skipped
rows. It normalizes the checkbox to `[ ]` before hashing the contract and records
its actual value as `authored_state`. The checked-row regression runs all controls
on a separate checked copy and reproduces every committed receipt field except
that explicit state field. IAC-19 now binds the existing "Open L2/L3 question"
heading above, keeping the native-oracle question open rather than asserting a
ceiling. Neither production checkbox is changed by these controls.
