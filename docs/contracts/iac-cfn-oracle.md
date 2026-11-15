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
`read_cloudformation.go`, `catalog_dir.go` IaCCatalogInfo/ListIaCCatalogs;
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
replaced by easier pairs. Pair minimum/selection is settled by W6-3 below, not an open U1.

## Exact scorer and acceptance

Internal bench scoring, before the W6 publication projection below:
For the same eligible resource keys: oracle WILL_REPLACE is positive;
WILL_UPDATE or verified unchanged is negative. MAY_REPLACE/unsupported are
oracle-unknown. A definite product replacement is a positive prediction;
conditional/unknown is abstention, never TN. Record TP, FP, FN, TN, product
abstentions and oracle unknowns. FN includes definite oracle replacements for
which the product did not issue a definite flag. Record privately precision TP/(TP+FP),
recall TP/(TP+FN), eligible/scored/failed/excluded counts and each reason;
public export is restricted by the W6-3 licence projection below.
Precision is undefined for TP+FP=0; it cannot PASS. Exact point comparison
100*TP >= 85*(TP+FP) remains descriptive, never the sole acceptance rule.
All preregistered executions must be accounted for. Missing native/catalog/pair
or truncated output is INCOMPLETE, an operationally invalid reading with no
statistical claim. A valid reading uses W6-3 / BN-D6's interval-based
CONFIRMED/REFUTED/INCONCLUSIVE outcomes below; threshold remains 0.85.
HZ-M2 verdict opens the reading. REFUTED (the valid below-floor FAIL outcome)
stops all further hazard computation: fix HZ-1 before any other hazard row,
including Terraform computation rows (issue #271 Stop conditions, design
change-hazards §14 and ADR-099 Revisit). The live IF-C4 flag requires advice
wording until fixed; that edit remains an architect-owned separate lane.
This ordering stop is distinct from per-tool serving gates: HZ-1 is the
CloudFormation 0.85 acceptance gate, never Terraform's independent 0.9 gate.
Neither passing HZ-1 nor restoring its floor lifts Terraform's own gate.

## Prospective proof tests, exact assertions

| Owned test | Every required assertion |
|---|---|
| test_manifest.py::test_pins_and_population | Correct fixed manifest accepted; missing catalog/runtime/source digest, changed template byte, duplicate id, cross-root path, unfrozen roster and empty population each refuse; roster changes invalidate a run. |
| adapter/main_test.go::TestCFNBenchAdapter | Exact reader/comparator route above; one changed replacement pointer produces definite with matching address/span/digest; conditional remains conditional; opacity/unbound/syntax/drop is unknown/refusal; additions/deletions/type changes excluded; result arrays nonnil; canary absent; order permutations give identical JSON. |
| test_native.py::test_pinned_cli_pair | Exact pinned CLI argv and one-stack assembly; will-replace, may-replace, update and unchanged parsed separately; credentials/network attempted, unknown output, signal, timeout or wrong pin refuse; CLI output and machine-readable crosscheck agree by resource. |
| test_score.py::test_below_floor_handoff | REFUTED below-0.85 interval stops all other hazard computation and requires live IF-C4 advice wording until fixed; INCONCLUSIVE grants no continuation/serving permission and triggers only the preregistered extension; CONFIRMED cannot lift Terraform own 0.9. This tests the handoff decision, never an unowned flag edit. |
| test_score.py::test_confusion_and_floor | TP=17 FP=3 gives point0.85 but not automatic CONFIRMED; TP=16 FP=3 point below floor also not automatic REFUTED if interval crosses0.85; TP=1 FP=1 FN=1 TN=1 gives precision=recall0.5; seeded FP/FN alter their cells; zero positives undefined/INCONCLUSIVE on valid input; failed/missing arm INCOMPLETE; conditional never TN; interval/outcome/stopping/publication assertions below all mandatory; reorder identical. |

Implementer first runs `python3 -I -m unittest discover -s docs/bench/harnesses/hazard-cfn -p 'test_*.py'`
and `go test -race ./docs/bench/harnesses/hazard-cfn/adapter -run '^TestCFNBenchAdapter$' -count=1`
under the shared gate lease. Capture actual failing assertions, then GREEN.
Run `run.py --pairs .../pairs.json --output workspace/tmp/hz-1-results.json`
only with the pinned input handoff, and independent critic checks all rows above.
Scratch verify of change-hazards is separate, never presence-based acceptance.

## W6-3 settled cohort, exact selection and preregistered extension

W6-3 replaces former U1. Accepted n is 60 pairs from PUBLIC HISTORY, never
hand-picked, balanced across hazard-case classes, with the roster hash frozen
before ANY run. A valid INCONCLUSIVE reading triggers the already preregistered
extension to 100 total pairs; 0.85 is unchanged. The alternative 100-only first
look is no longer an open choice. No raw roster or interval is supplied here.

Freeze a complete nominated public-repository frame, time window, exact heads,
first-parent commit walk and source-file predicate before evaluating either arm.
Enumerate every adjacent first-parent base/head pair in that frame which changes
a CloudFormation template (.json/.yaml/.yml with Resources). Do not sample on
product flags, oracle output, success of native execution or a favorable score.
Record every excluded merge/non-template/invalid-source candidate by opaque
case id and a fixed exclusion code; do not replace failed selected cases.

Freeze independent pre-run case labels from that public history into the three
existing #271 candidate strata: replacement-candidate, nonreplacement-candidate,
conditional-or-opaque-candidate. They are intended hazard-case coverage labels,
NOT observed truth or guesses at measured outcomes; the oracle still labels
replacement independently. Label provenance and the closed labeling rules must
be registered before scores; a disputed/unclassifiable history remains named
and cannot be assigned after observing either arm. This is HZ-1's CFN case
coverage, not a new Terraform H2/H4/H5 cohort or extra confirmatory primary.

Within each stratum rank candidates by SHA-256 of the domain
"iac-cfn-w6-v1", fixed seed "20261115", and length-prefixed canonical public
repository id/base commit/head commit/template path/stratum. Ties use that
same tuple. Selection is deterministic round-robin in the fixed stratum order
above: first 60 means 20 per stratum; reserve next 40 before runs so the full
100 has counts 34/33/33. Publish the history-roster hash and opaque pair ids;
freeze all 100 entries and their two look memberships before any native/product
execution, shakedown on confirmatory pairs, or score. A distinct development
shakedown uses disjoint history ids and cannot tune this roster. Insufficient
frame, missing independent labels or unable to meet balance is an input HOLD,
never permission to hand-pick or post-result rebalance. Multiple pairs from a
repository remain identified as one statistical cluster.

The existing pairs manifest binds public source identities/bytes, class labels,
roster hash and look membership. Catalog/schema identities, catalog-derived
citations and full run pins belong only in confined workspace input receipts;
they are not emitted in public pairs.json/pins.json without redistribution
rights. No source bytes, catalog fields or fixture values are added by this
contract. Freezing hash/selection is still REQUIRED INPUT, not a claim it exists.

## W6-3 / BN-D6 exact statistical method and outcome assertions

W7-4 settles former U2. Direct source read:
docs/design/proof-program.md BN-D6, lines113-121, names NO interval method.
Therefore preregistration MUST state "Wilson score interval, two-sided 95%"
per W7-4. Method/confidence are settled; no further statistical approval question.
This replaces the prior proposed resampling estimator, replicate count,
97.5% bounds and alpha split. Neither a different interval nor a confidence
adjustment is inferred from BN-D6's general clustered/sequential language.

Task here remains the public-history template pair, with both arms paired.
Repository identity/cluster membership and per-repository counts are retained
for the BN-D6 population/accounting assertions; they do not alter W7-4's Wilson
estimator or invent an effective sample size. Sole confirmatory primary remains
precision TP/(TP+FP) at0.85; recall/stratum counts descriptive, Holm across this
one primary the identity. The only registered looks remain60 and cumulative100.
Apply two-sided95% Wilson at each permitted look, no alpha split or re-peeking;
95% is the stated per-look level, not a simultaneous two-look coverage claim.
No additional primary, sample, look or post-result method choice is permitted.

Deterministic Wilson inputs: integer k=TP, n=TP+FP, z=1.959963984540054,
z2=z*z. n is the number of definite positive predictions, NOT the60/100 cohort
size, resource total, repository count or number of native positives. For n>0,
0<=k<=n, define
h = z * sqrt(z2 + 4*k*(n-k)/n), d = 2*(n+z2),
lower = max(0,(2*k+z2-h)/d), upper = min(1,(2*k+z2+h)/d).
At k=0 set lower exactly0; at k=n set upper exactly1. These match the existing
private wilson95 formula and wilsonZ95 constant in
third_party/agentic-kit/foundations/decision/reading.go; that private helper
is source evidence, not a public callable port or permission to add a dependency.
Use a deterministic calculation with the fixed constant, compare full-precision
bounds to0.85, then six-place half-even display. No randomness, seed or cell
order influences Wilson bounds. Negative/noninteger counts or k>n are typed
input refusals. Do not clamp invalid counts into a valid result.

For otherwise valid complete input with n=0, precision and Wilson bounds are
undefined and statistical outcome INCONCLUSIVE, never a fabricated[0,1] bound
or a zero/perfect precision. Valid60 INCONCLUSIVE still triggers its frozen
extension; invalid/missing native cells remain operational INCOMPLETE. No
cluster-count/dispersion fallback overrides the Wilson result when n>0: even
all-correct/all-wrong predictions have the nondegenerate Wilson edge interval.
The disjoint shakedown still records noise/power/validity limitations; W6/W7
fix n60/100, confidence, floor, roster and stopping rule. Invalid execution
cannot be repaired by selecting a more favorable interval after results.

At each valid look compare UNROUNDED bounds: lower>0.85 -> CONFIRMED (acceptance
PASS); upper<0.85 -> REFUTED (acceptance FAIL); otherwise INCONCLUSIVE. A bound
touching0.85 is INCONCLUSIVE, never a point-only PASS. Publish point and both
bounds beside outcome. Use six fractional digits/half-even for display only;
retain exact integer point counts and full-precision computed bounds for the
comparison. The exact 0.85 floor is not raised to a new target. The interval
expresses uncertainty about meeting that unchanged target.

Stopping is fixed: at60 CONFIRMED/REFUTED stop; at60 valid INCONCLUSIVE run ONLY
the frozen remaining40 and report cumulative100 plus retained60 result; at100
INCONCLUSIVE remains INCONCLUSIVE/HOLD with no third look or extra pairs.
Operational INCOMPLETE requires a repaired same-input execution receipt or
new preregistration, never an automatic easier replacement cohort. No missing
inputs are supplied by this ruling. No surface claim on INCONCLUSIVE.

Additional exact test_score.py::test_confusion_and_floor assertions:
point0.90 with supplied interval[0.86,0.97] CONFIRMED; point0.80 with[0.70,0.84]
REFUTED; point0.85 with[0.75,0.93] INCONCLUSIVE. Supplied[0.85,0.95] and
[0.75,0.85] both INCONCLUSIVE. These are classifier boundary fixtures, not
invented observed Wilson outputs. Display rounding never decides the outcome.

Add exact test_score.py::test_wilson95_deterministic, all assertions mandatory:
with the fixed z constant, bounds agree with these arithmetic-only reference
vectors to absolute tolerance1e-12 before display (k,n -> lower,upper,outcome):
- (0,60) -> (0,0.060171852142), REFUTED; lower exactly0.
- (60,60) -> (0.939828147858,1), CONFIRMED; upper exactly1.
- (17,20) -> (0.639581135259,0.947631254104), INCONCLUSIVE.
- (16,20) -> (0.583982567748,0.919342337420), INCONCLUSIVE.
- (56,60) -> (0.840746426868,0.973771301275), INCONCLUSIVE.
- (57,60) -> (0.862994835237,0.982850497836), CONFIRMED.
- (93,100) -> (0.862504852609,0.965680738933), CONFIRMED.
- (75,100) -> (0.656955364519,0.824547886377), REFUTED.
All are hypothetical count fixtures, not native inputs, measurements or a
claimed precision reading. At17/20 in a60-pair cohort the denominator stays20,
not60. Reverse/permuted pair/resource/repository order with identical k,n gives
identical unrounded bounds and output. Changing cluster labels without changing
k,n does not substitute another estimator. k=0/n>0 and k=n/n>0 keep the exact
edge bound and the other nontrivial bound. n=0 produces undefined point/bounds
and INCONCLUSIVE on valid input; negative/noninteger/k>n refuses. No all-correct
case is demoted merely because a discarded resampling method was degenerate.
No precision rounding, confidence adjustment or seed affects classification.
The implementing critic checks every assertion and independently compares a
pinned reference on disjoint synthetic fixtures; no implementation test runs
in this contract update. The arithmetic reference check is not a native reading.

Additional test_manifest.py::test_pins_and_population assertions: public-history
frame enumeration reproducible under input reorder; fixed labels before output;
exact20/20/20 first60 and34/33/33 cumulative100; seed/ranking/first-parent and
look ids fixed; full100 hash frozen before first run; hand-picked substitution,
insufficient strata, changed hash and post-result relabel/rebalance each refuse.
Additional test_score.py::test_below_floor_handoff assertions: valid60
INCONCLUSIVE activates exactly preregistered40, cumulative100 includes original60;
60 CONFIRMED/REFUTED never activates extension; 100 INCONCLUSIVE no third look.

## W6-3 publication and licence boundary

A public reading exports only aggregate performance counts, opaque PUBLIC
HISTORY pair/repository ids, look ids, validity/outcome verdicts and the required
point estimate with interval. Roster hash is an identity of public-history input,
not a schema fingerprint. It publishes NO material derived from a schema with
no redistribution licence: no schema bytes, property/type names, pointers,
fragments, replacement annotations, catalog/blob digests, schema-derived ids,
per-resource citations, manifests, per-attribute decisions or raw native output.
Unclear licence is treated as no redistribution permission. Internal adapter
provenance/schema receipts and detailed joins stay confined to workspace;
existing citation assertions are PRIVATE correctness checks, not a public export.
Even if permission exists, this reading keeps the narrow counts/ids/verdicts
projection plus mandatory statistics; licensing does not silently widen it.

Add test_score.py::test_publication_boundary: seed private results with canary
schema type/property/pointer/digest/replacement text and raw native output;
public renderer emits only the exact allowlist above and no canary/schema-derived
field. Public ids must derive from registered public history, never schema names.
Missing licence permission never enables copying; private correctness citations
survive internally while public schema-derived fields are absent. No schema or
real operator data was read, run, published or fabricated in this update.

## Decision models

None. Both arms and scorer are deterministic; provider none is the entire
protocol. Opt out by not running a reading. A model supplies no truth label.

## Draft verification correction

The first draft commit's Verified line incorrectly claimed the scratch checker
passed: its catalog-info anchor named catalog.go instead of catalog_dir.go.
The shell continued after that assertion. This append corrects the anchor and
records a successful rerun with fail-fast shell execution. No implementation
or measured result changed.

## Independent critic correction, round 1

Restored the stronger authored ordering stop from #271, design §14 and ADR-099:
a below-floor HZ-1 stops all further hazard computation until repaired. The
per-tool serving floors remain separate. Added the exact prospective
test_below_floor_handoff assertion. No native execution or product edit.

## W6 compliance record

W6-3 settles former U1 and supersedes point-only acceptance/publication wording
throughout this draft. This is a later-ruling compliance update, not a new
critic FIX cycle; the prior resolved FIX round1 remains recorded. Independent
critic must confirm this exact immutable commit before approval.

## W7-4 compliance record

BN-D6 root lines113-121 read directly: no interval method is named. W7-4
therefore settles Wilson score, two-sided95%, stated in preregistration. Former
U2 method/confidence is closed, not re-escalated. W6-3 public-history60,
frozen100 roster/hash, balancing, three outcomes, inconclusive extension,
unchanged0.85, licence boundary and stop/serving rules remain binding.
Later-ruling compliance does not consume a critic FIX round. Same independent
critic confirms this exact READY commit; all other seven IaC choices stay held.
