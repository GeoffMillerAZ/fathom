# IaC CloudFormation oracle: #271 / HZ-1

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

## Decisions and alternatives

- D1 Recommended: bench adapter calls the existing live check, then independently
  runs CDK's template diff on identical bytes. Alternative: catalog-to-catalog
  agreement is IF-CM3, a separate reading. Governing: HZ-1 and design
  `change-hazards.md` §10 M2, ADR-099's 0.85 trigger.
- D2 Recommended: score definite replacement at resource declaration identity;
  report conditional/opaque results separately. Alternative: count may-replace
  as proven replacement, rejected by design §8 replacement semantics.
- D3 Recommended: implementing lane supplies actual failing scorer/adapter tests.
  Alternative: treat missing files as observed RED, forbidden by W3-2.
- D4 Recommended: pin CDK's packaged CLI, compare exactly one offline assembly
  stack against the base template. Alternative: default cloud change-set mode,
  rejected by design §2 U4 and ADR-091 decision 4.

## Exact implementation ownership and native boundary

#271's directory grant narrows to these new files only:
`docs/bench/harnesses/hazard-cfn/run.py`, `score.py`, `native.js`,
`adapter/main.go`, `adapter/main_test.go`, `pairs.json`, `pins.json`,
`test_score.py`, `test_manifest.py`, `test_native.py`, and
`docs/bench/readings/change-hazards/m2-cdk-diff.md`.
Raw templates, cloud assemblies, catalog archives, stderr and native results
stay under workspace, referenced by digest. No product file is writable.
These exact additions require the integrator's lane-record handoff, not a
blanket directory grant. No fixture licenses or native files are copied here.

Current production signatures, at the source base:

```go
// internal/iacfacts, reused without changes
func ReadCloudFormationFile(filePath string, src []byte, opts ...Option) (*ResourceSet, error)
func CheckCloudFormationChange(catalogs []IaCCatalogInfo, region string,
    base, head Resource, baseSrc, headSrc []byte) CFNChange
```

`adapter/main.go` is a bench-only main package under this module and can import
internal/iacfacts. Proposed local entry is `run(in io.Reader, out io.Writer)
error`. Request JSON has exactly `catalog_paths`, `region`, `base_path`,
`head_path`. It calls ListIaCCatalogs, both readers with no WithValues, rejects
syntax errors/dropped sections, indexes matching managed declarations by
(tool,module,mode,type,name), and calls the existing comparator for each
same-type logical id. Addition/deletion/type-change are named exclusions,
not unchanged negatives. Missing/duplicate identities are typed bench refusal.
It returns `schema_version:1`, `resources:[]`, `exclusions:[]`. A resource has
`address` as CTR-10's five-key object, `prediction` in the named closed local
vocabulary definite/conditional/none/unknown, and `citations:[]` with the
existing finding's pointer/path/digest/span. `Notes` never becomes a clean
negative. No source, literal value, snippet or value hash is emitted. Local
Go request/result types have explicit JSON tags; local error kinds include
input/pin/catalog/parse/native/limit. Errors never echo template contents.

Read anchors: `internal/iacfacts/cfn_replace.go` CFNChange/CFNChangeFinding,
`read_cloudformation.go`, `catalog.go` IaCCatalogInfo/ListIaCCatalogs;
existing synthetic `TestCheckCloudFormationChange` is a reuse fixture, not
an independent reading of real resource schemas.

## Native pin and actual invocation

Freeze `aws-cdk` 2.177.0 npm tarball SHA-256
`dbdd27cd98a53bc0ec1ebd89cd6db53a5df1ce90643c3706ecc5ef08edf44d04`.
Primary metadata: https://registry.npmjs.org/aws-cdk/2.177.0 . Archive bytes
were checked against npm's SHA-512 integrity and hashed during drafting;
no package was installed or executed. Its bundled `lib/cli/cdk-toolkit.js`
fixed-template branch calls `printStackDiff`; packaged `lib/diff.js` calls
`fullDiff` and owns its bundled specification. The whole CLI tarball digest
pins that specification. Do not resolve a floating service-spec dependency.

Bench command contract (paths are manifest-bound local inputs):
`node <pinned-cli>/bin/cdk diff Pair --app <head-assembly> --template <base.json> --exclusively --no-change-set --strict --lookups=false --no-color --no-notices`.
Use the 2.177.0 `--template` spelling; do not silently substitute newer CLI
flags. The assembly contains exactly one Pair stack artifact with head JSON,
no executable app, context lookup, asset or nested-stack deployment. Freeze
assembly manifest bytes and its schema version with the inputs. Run in an
isolated bench environment without credentials or network. Keep stdout,
stderr, argv and exit. A diff exit is interpreted according to that pin,
not assumed to mean tool failure. The implementing native adapter test must
prove this command is offline and maps the pinned rendered will/may-replace
and update/no-change outputs. Unrecognized output is INCOMPLETE.

For a machine-readable crosscheck only, primary pinned declarations in
`@aws-cdk/cloudformation-diff` 2.177.0 expose `fullDiff(base,head)` and
`TemplateDiff.resources.changes[id].changeImpact`, with WILL_REPLACE,
MAY_REPLACE and WILL_UPDATE. Tarball SHA-256:
`50d0f417bbed04f24097324c93691d6ea9489ba51e1219b43d1a7727445354ac`.
https://registry.npmjs.org/@aws-cdk%2fcloudformation-diff/2.177.0 .
It is not a replacement for the requested CLI witness, and its floating
transitive dependencies are not authorized installs. CLI/adapter disagreement
is retained and stops a scored PASS until explained.

## Frozen manifest contract and missing raw inputs

`pins.json` requires exact runtime version and executable digest, CLI archive
digest above, catalog archive digest, region, manifest source/line/date/version
and Inputs snapshot digest from IaCCatalogManifest. CFN version is the exact
`sha256-<12hex>` snapshot label, never a date pretending to be a version.
`pairs.json` requires schema_version=1, source roster digest, selection method,
freeze commit and sorted `pairs:[]`. Every pair requires unique pair_id,
source URL+revision+relative path, base/head template SHA-256, catalog digest,
logical ids and inclusion/exclusion reason. No aliases, absolute host paths,
duplicate pair ids or digest mismatch. Known synthetic fixtures are marked
scorer-only and excluded from the live precision denominator.

No valid real catalog, pair manifest, native runtime digest or assembly bytes
are supplied in this clone. They are REQUIRED INPUTS, not empty accepted pins.
Preregistration is append-only and complete before either arm runs. Changing
an input after scores starts a newly labelled reading; failures cannot be
replaced by easier pairs. Pair minimum/selection is U1 below.

## Exact scorer and acceptance

For the same eligible resource keys: oracle WILL_REPLACE is positive;
WILL_UPDATE or verified unchanged is negative. MAY_REPLACE/unsupported are
oracle-unknown. A definite product replacement is a positive prediction;
conditional/unknown is abstention, never TN. Record TP, FP, FN, TN, product
abstentions and oracle unknowns. FN includes definite oracle replacements for
which the product did not issue a definite flag. Publish precision TP/(TP+FP),
recall TP/(TP+FN), eligible/scored/failed/excluded counts and each reason.
Precision is undefined for TP+FP=0; it can never PASS. Compare exact integers
100*TP >= 85*(TP+FP), not rounded displays. All preregistered executions must
be accounted for. Missing native/catalog/pair or truncated output is
INCOMPLETE, with no observed precision claim. With complete coverage, below
0.85 is FAIL, at or above is PASS. HZ-M2 verdict opens the reading. A FAIL
stops further CloudFormation hazard work and requires advice wording on the
live flag; that product edit remains an architect-owned separate lane.
No reading can lift Terraform's separate 0.9 gate.

## Prospective proof tests, exact assertions

| Owned test | Every required assertion |
|---|---|
| test_manifest.py::test_pins_and_population | Correct fixed manifest accepted; missing catalog/runtime/source digest, changed template byte, duplicate id, cross-root path, unfrozen roster and empty population each refuse; roster changes invalidate a run. |
| adapter/main_test.go::TestCFNBenchAdapter | Exact reader/comparator route above; one changed replacement pointer produces definite with matching address/span/digest; conditional remains conditional; opacity/unbound/syntax/drop is unknown/refusal; additions/deletions/type changes excluded; result arrays nonnil; canary absent; order permutations give identical JSON. |
| test_native.py::test_pinned_cli_pair | Exact pinned CLI argv and one-stack assembly; will-replace, may-replace, update and unchanged parsed separately; credentials/network attempted, unknown output, signal, timeout or wrong pin refuse; CLI output and machine-readable crosscheck agree by resource. |
| test_score.py::test_confusion_and_floor | TP=17 FP=3 gives 0.85 PASS with complete inputs; TP=16 FP=3 FAIL; TP=1 FP=1 FN=1 TN=1 gives precision=recall=0.5; injected FP and FN increment their own cells; TP=FP=0 undefined/INCOMPLETE; missing row/failed arm remains INCOMPLETE; conditional is never TN; reordered rows identical. |

Implementer first runs `python3 -I -m unittest discover -s docs/bench/harnesses/hazard-cfn -p 'test_*.py'`
and `go test -race ./docs/bench/harnesses/hazard-cfn/adapter -run '^TestCFNBenchAdapter$' -count=1`
under the shared gate lease. Capture actual failing assertions, then GREEN.
Run `run.py --pairs .../pairs.json --output workspace/tmp/hz-1-results.json`
only with the pinned input handoff, and independent critic checks all rows above.
Scratch verify of change-hazards is separate, never presence-based acceptance.

## Unsettled point for foreman batch

U1 (#271): Recommend 60 preregistered template pairs, balanced replacement,
nonreplacement and conditional/opaque, independent source roster frozen before
scores. Alternative 100. HZ-1 fixes 0.85 but not cohort/minimum. Raw pair identities
and catalog/runtime digests still require input delivery, not a fabricated ruling.

## Decision models

None. Both arms and scorer are deterministic; provider none is the entire
protocol. Opt out by not running a reading. A model supplies no truth label.
