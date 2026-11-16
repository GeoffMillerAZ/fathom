# IaC replay and shared bench plan reader: #310 / HZ-M1 and #302 / HZ-17

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

- D1 Recommended: one bench-only plan reader owned by #310, then immutable
  read-only reuse by #302. Alternative: duplicate it or add a product reader,
  rejected by HZ-M1/HZ-17, design §9 E10 and I56.
- D2 Recommended: native head plan over synthesized base state, all failures
  and costs retained. Alternative: empty-state plan or source fixtures as
  replay truth, rejected by HZ-M1 and design §10 M1.
- D3 Recommended: preregister module-stratified random sampling and grade both
  readers against the same native values, reporting soundness with decidability.
  Alternative: compare scanner finding counts, rejected by HZ-17/design M4.
- D4 Recommended: keep H2/H5 advice and separate replay noise from acceptance.
  Alternative: serve a drafted check on replay alone, rejected by W3-1 U4 and
  ADR-106. The 0.9 bar does not authorize any surface without its other gates.
- D5 Settled W12-2: the unit of admission is the hazard kind, with at least30
  distinct positive public-history pairs and its own interval/outcome. Alternative
  pooled admission is rejected; another kind's success cannot admit this kind.
- D6 Settled W12-3: exactly90 roots,30 per module stratum, seeded without
  replacement. Alternative120/40 is superseded, not an open sample-size choice.
- D7 Settled W12-4: the four exact extra paths below are granted to #310, one
  writer per turn; #302 reads. Alternative separate writer is superseded.

## Exact paths and ownership

#310's new directory grant narrows to
`docs/bench/harnesses/hazard-plan/{run.py,plan_reader.py,score.py,manifest.py,test_plan_reader.py,test_score.py,test_manifest.py,replay.json,pins.json}`
and `docs/bench/readings/change-hazards/m1-replay.md`.
#302 owns only `docs/bench/readings/change-hazards/m4-decidability.md`.
W12-4 grants these exact extra shared helpers exclusively to #310:
`docs/bench/harnesses/hazard-plan/decidability.py`,
`docs/bench/harnesses/hazard-plan/test_decidability.py`,
`docs/bench/harnesses/hazard-plan/decidability.json`,
`docs/bench/harnesses/hazard-plan/trivy-resolution.md` (native adapter build/run
handoff recipe, not a product package/dependency).
Permission is settled by W12-4; activation still requires an exact incoming
commit/base and outgoing owner's handback before #310 is the sole writer.
The #310 outgoing commit records the reader/helper/data/recipe handback; #302
consumes those exact immutable files read-only and receives no write turn.
Reject concurrent writers, unlisted paths or an inferred product-file grant.
The grant is not implementation, actual input data or proof.
No product parser, public API, config, hook, provider cache or catalog is
changed. No new dependency in go.mod/go.sum or vendored kit. Temporary native
builds require their own explicitly frozen upstream environment; this
contract does not authorize installing them now.

## Pinned native inputs and evidence boundary

Reference native primary sources:
https://github.com/hashicorp/terraform/blob/v1.9.8/website/docs/internals/json-format.mdx
and the matching show command documentation. Pin Terraform 1.9.8 archive
SHA-256 (official release checksum list read during drafting):
linux_amd64 `186e0145f5e5f2eb97cbd785bc78f21bae4ef15119349f6ad4fa535b83b10df8`;
darwin_arm64 `873d7b925d08578fb6bb9c12c7cd92ae73e289e07c360f2fdd69f9036b7baaab`.
https://releases.hashicorp.com/terraform/1.9.8/terraform_1.9.8_SHA256SUMS .
One engine per cohort; do not pool Terraform and OpenTofu output. Optional
separately preregistered control: OpenTofu 1.8.8, official archive SHA-256
linux_amd64 `9e889633bc177b1d266552658020fe8ceb839445fcac82aaa7622952fd9c81bb`;
darwin_arm64 `b21e7a5d7de20265aa6a47c5d3baf9ee8e131af9c852cd47574e53ecafdd4118`.
https://github.com/opentofu/opentofu/releases/download/v1.8.8/tofu_1.8.8_SHA256SUMS .
No binary was downloaded/executed; checksum-list receipt is not a verified
executable identity. At run time hash the actual executable, record version,
platform, native archive digest, provider source/version/binary digests,
lockfile digest and native JSON format versions before accepting a plan.

Trivy resolution source pin: v0.76.2 at commit
`3f6334d223b52188324af3b9923b4249cd206652`.
Official native archive checksums: linux_amd64
`bc0fc726e017b8927a62b2d3531d7a84ee01769daf5c99ed866dced54089fdbf`;
darwin_arm64 `ae7ea358326960406b703c77b2ac490190797d80b306b5b50c2a27221da0a5b8`.
https://github.com/aquasecurity/trivy/releases/download/v0.76.2/trivy_0.76.2_checksums.txt .
Checksum list and exact source API were inspected, no native result earned.

A stock Trivy config finding report is NOT a resolution population. The
external bench adapter uses the pinned source APIs:
`parser.New(fs.FS, string, ...Option) *Parser`, `ParseFS(context.Context,string)
error`, `EvaluateAll(context.Context) (terraform.Modules,error)` in
`pkg/iac/scanners/terraform/parser`; options `OptionWithDownloads(false)` and
`OptionStopOnHCLError(true)`. Enumerate Modules -> GetBlocks -> GetAttributes;
record structural address/span/path, `IsResolvable()`, exact typed `Value()`
and native unknown marks in scratch only. `IsResolvable` alone is a claim,
not soundness: this pin has fallback values for missing variables in
parser.go, so compare every claimed definite value to native plan truth,
including a missing-variable fallback trap. No silent adapter coercion or
source patch. Prepared adapter source/build manifest/executable digests and
complete module FS mapping must be handed back before #302. Its external
build environment remains outside product dependencies; no source installation
or module fetch is authorized by this draft. An unavailable exact adapter is
INCOMPLETE, never substituted with a finding-count proxy.

## Frozen manifests and replay protocol

Both manifests have schema_version=1, source_frame digest, selection seed,
selection rule, freeze commit, native pins, product binary/source/API snapshot
identities and sorted cases. Each replay case has case_id, public repository
URL, exact base/head commit, root module path, file-list+content digests,
provider and catalog pins, state synthesis recipe digest, explicit fake
credential/skip flags, environment-variable names (no credential values),
expected cohort and exclusion policy. History/base/head bytes are raw inputs,
not sourced from a model-chosen result pool. A complete nominated frame is
required, not an assertion that some successful roots were found. W12-2 sizes
admission evidence by kind, not a fixed60/100 overall replay cohort. No real
frame/case/state recipe or positive-pair population is delivered today.

Freeze the intended subset of H1/H2/H4/H5, complete PUBLIC HISTORY candidate
frame, exact revisions/root paths, independent label rules, selection seed,
initial look ids and reserved extension look ids before ANY run. A positive
precision pair is a distinct registered base/head public-history pair on which
that kind makes a positive prediction; both correct and incorrect predictions
count toward its positive-pair denominator. This prevents selecting only true
positives or using independent truth to exclude false positives. Public history
and outside labels precede evaluation; no model-chosen/hand-picked success pool.

Within each preregistered kind-eligible frame, rank candidates by SHA-256 of
seed20261115 and a length-prefixed tuple (kind,canonical public repository id,
base commit,head commit,root). Ties use the tuple. Freeze the entire ordered
initial/reserve roster and its hash before either arm, including negative
controls and independently labelled causal cases; no replacement of failed or
disagreeing pairs. Hashes bind both memberships and the finite stopping plan.
A pair can contribute to different kind strata, but only once within a kind.
Its repeated resources/instances cannot manufacture30 positive pairs.

Target at least30 positive pairs for each kind sought for admission (W12-2).
Start with kinds whose frozen public-history frame can supply that evidence;
unknown yield/frame size is missing input, not invented data. Evaluate every
case in the frozen look, rather than stopping after a favorable thirtieth
prediction or adding cases chosen after a score. Initial/reserve candidate
limits must be registered from actual pre-data evidence. W12 supplies no
numerical extension horizon; U5 below remains explicit. The CFN60-to100 ruling
is not imported as a Terraform extension.

For each case prepare isolated base/head directories under workspace from
those revisions. Reuse only operator-prepared provider binaries/modules;
no remote backend, credential discovery, cloud call, init download or apply.
Run native init with locked local mirror/lockfile in offline mode only if
needed, then validate base/head. Failed init/validate remains a failed case.
The bench state synthesizer produces native state v4 resources/instances
with source/version schema_version and attributes matching base, deterministic
fake ids/serial/lineage, all indexed instances and provider binding. Recipe
and generated state digest are frozen; native `show -json` must accept it.
Unknown required fields, invalid state shape or a provider requiring cloud
state is a named failure, not a fabricated valid state or empty-state fallback.
Before reading head scores, require a base-over-synthetic-state control plan
with no spurious delete/create/replacement; publish residual actions. A failed
control makes the case ungradable and blocks valid evidence/admission for
all participating kinds declared for that shared pair in the frozen manifest; it does not become a negative or a global pooled gate.

Head plan uses explicit synthesized state, `-input=false`, `-lock=false`,
`-refresh=false`, saved `-out`, fake credentials and only provider-specific
skip flags proven by that case's pinned source. These are proposals to validate
in shakedown, not observed working invocations. `show -json <saved-plan>`
must succeed at this pin: if refresh=false or synthetic state prevents JSON
output, record INCOMPLETE rather than changing the protocol after seeing scores.
No native apply/import/cloud is permitted as a workaround. Capture command,
exit, duration, provider CPU/wall time and every native diagnostic in scratch;
publish sanitized typed failure categories, resource counts and cost totals.
State/plan JSON may carry secrets, so it never lands in docs or product output.

Invoke the approved LIB-7 binary contract exactly:
`fathom-lang hazards --base <base-root> --head <head-root> --json`.
Record its binary digest, source commit and schema_version=1. Reuse CTR-10's
hazard envelope/address/enum definitions at
26ffbfedae78c6486229b4558c14849421d52664, not another public shape.
Failure to hand over this executable cannot be cured by unit-fixture output.

## Exact bench reader port and schemas

Proposed local Python signatures in plan_reader.py:
`read_plan(raw: bytes) -> PlanTruth`,
`join_attribute(truth: PlanTruth, address: Address, path: tuple[str|int,...]) -> AttributeTruth`.
These are private bench structures, never a public door or MCP input.
PlanTruth holds sorted resource_changes and planned-value attribute population;
Address uses the five comparable CTR-10 identity components, with instance
key separate. Resource changes retain complete native actions, replace_paths,
action_reason, before/after unknown and sensitive masks. AttributeTruth status
is named closed known/unknown/absent/sensitive/unjoined; exact typed values
are private scratch input only. Plan format major other than 1, missing required
structure, duplicate resource instance, wrong type or unknown action sequence
is a typed bench error; allow unknown optional action_reason as unspecified
and retain its raw code in scratch. Never use that display hint alone as truth.

Replacement is exactly actions [delete,create] OR [create,delete]; standalone
delete/create/update/no-op/read are distinct. Keep every replace_path including
numeric steps; missing replace_paths does not negate replacement (taint or
operator request can cause it). Such causal confounders are named and excluded
from the hazard-causal comparison according to frozen rules, not dropped from
cohort coverage. Missing output or unknown value is not no-op/false/null.
Walk planned_values.root_module and recursive child_modules plus matching
resource_changes; absent section or disagreement is an incomplete join.
Never regex-strip quoted instance keys. Use native address components, complete
module hierarchy and separate typed instance keys. Declaration-level joins
aggregate all instances; divergent values/unknown instance sets are unjoined,
not one arbitrarily selected instance. Attr paths use parsed segments and
wildcards from the existing iac Attr.Path contract, with all matched native
leaves accounted for. Empty root, dynamic blocks, modules, map keys and absent
attributes have explicit denominator entries. Preserve absent vs known null.
Respect sensitive masks: no raw value, raw source or reversible value hash in
published rows, errors, manifests or JSON goldens.

## Exact replay scorer

Keep the existing raw (case_id,hazard id,declaration address) witnesses, native
instances, spans and citations. For admission aggregate once per (kind,pair_id):
a positive pair has at least one positive prediction of that kind; TP requires
EVERY positive declaration claim of that kind on that pair to agree with its
independent labels/native evidence, otherwise FP. Mixed correct/incorrect
claims count as one FP, never several TP that dilute the mistake. Negative
controls, FN/TN, abstentions, unlabelled/unjoined/failed/excluded cases remain
visible per kind; a known-positive pair with no prediction contributes FN,
not a positive precision trial. Ambiguous/missing labels are not guessed.

For each kind, positive_pairs=TP+FP over distinct pair ids and precision is
TP/(TP+FP); recall TP/(TP+FN) is descriptive, undefined at zero denominator.
W12-2 requires positive_pairs>=30 AND that kind's own precision0.9 or better,
with the interval published. Exact point comparison10*TP >= 9*(TP+FP) is
necessary, never sufficient by itself. The per-kind interval/outcome rule
below decides whether its evidence confirms the0.9 bar. A kind with fewer
than30 positive pairs is INCONCLUSIVE and UNSERVED, whatever its point,
interval, resource count, another kind or the pooled summary says.

The unit of admission is the KIND, never the whole cohort or pooled figure.
Publish each planned kind's own counts, interval, outcome and explicit
admission status; unplanned/missing kinds have no admission receipt and stay
unserved. Pooled counts/precision are descriptive only: they contain NO
outcome/admission field and cannot be an input to kind admission. A weak or
unattempted sibling does not erase another kind's independently sufficient
reading, and a strong sibling never rescues it. Actual serving still requires
all authored consumer/ADR-106 acceptance, evidence-class and entry gates;
"eligible-by-replay" is not an automatic served/default state or row proof.
H2/H5 remain advice even after a precision reading; no kernel is commissioned.
A failed control/native read blocks evidence for its affected kind and stays
in full run accounting, never a clean negative or a silently omitted case.

H1 label needs native replacement AND independently person-accepted stateful
identity; plan replacement alone cannot label data loss or certify LIB-9.
H2 needs an independently confirmed old/new declaration pairing plus native
destroy/recreate consequence, never output-inferred rename truth. H4 needs a
preregistered repetition/key transition and native instance destruction or
replacement, with reason/replace_path and independent attribution retained.
H5 is loss of static proof, NOT replacement: independently labelled source
transition and native known/unknown context are recorded; a resolvable head
expression can still be an honest loss of static proof. Never call H5 a false
replacement merely because it caused none. Blind outside labels for these
causal/transition properties, labeler/source identity and adjudicated
conflicts must precede product output. Without them the affected kind evidence
is INCOMPLETE and unserved; complete sibling evidence cannot fill that gap.
Replay is a noise reading; ADR-106 still requires outside labels, strength and
person acceptance before a drafted check may serve, with H2/H5 advice retained.

## W12-2 per-kind interval, outcomes and extension assertions

Reuse the settled Wilson score method, TWO-SIDED95%, with the fixed source
z=1.959963984540054, as specified by W7-4 and the exact oracle contract
127db6b214c58393ad59bfe3dd5bec2b08dfecfc. For each kind separately, k=TP pair
count and n=TP+FP distinct positive pair count, NOT all candidate pairs, raw
addresses, another kind's counts or pooled n. Use the existing private
wilson95 formula in third_party/agentic-kit/foundations/decision/reading.go:
q=z*z; h=z*sqrt(q+4*k*(n-k)/n); d=2*(n+q);
lower=max(0,(2*k+q-h)/d), upper=min(1,(2*k+q+h)/d), for n>0 and0<=k<=n.
Set lower exactly0 at k=0 and upper exactly1 at k=n; invalid counts refuse.
At n=0 point and interval are undefined, never invented[0,1] evidence.
Repository grouping/paired witnesses remain in accounting; they never change
this estimator or manufacture an effective n. This is a per-kind per-look95%
statement, not a pooled/joint/simultaneous confidence claim. Precision is the
one confirmatory primary for each kind's admission; pooled/recall are summary,
not additional admission primaries. No method/confidence approval is renewed.

After valid complete registered evidence, FIRST apply W12-2's minimum: n<30 ->
INCONCLUSIVE/unserved, even if a point/interval looks favorable or unfavorable.
At n>=30: unrounded lower>0.9 -> CONFIRMED/eligible-by-replay; upper<0.9 ->
REFUTED/unserved; otherwise INCONCLUSIVE/unserved. A bound touching0.9 is
INCONCLUSIVE. The necessary point floor remains>=0.9, exact integer comparison;
point0.9 alone never admits. Publish that kind's point AND full interval beside
outcome, with exact n and source-manifest identity. Six-place half-even display
never chooses an outcome. Operationally incomplete input grants no statistical
outcome/admission and cannot be repaired by a sibling's result or a default.

BN-D6 three outcomes (docs/design/proof-program.md) and W12-2 require a
preregistered extension for each INCONCLUSIVE kind. At a permitted inconclusive
look, evaluate only that kind's already frozen reserve/look; retain the initial
result and report cumulative evidence without outcome-driven exclusions.
A confirmed/refuted kind does not gain a new look to improve its headline.
At a preregistered final inconclusive look stop unserved; no open-ended trials,
newly selected easier commits, backfilling positives or accepting a later good
pooled score. U5 is the still-missing numerical horizon, not permission to run
before the initial/reserve/stop registration exists. Only evidence from actual
admissible public-history pairs can count; contract/golden presence does not.

Add test_score.py::test_kind_admission_w12, exact prospective assertions:
- KindA TP35/FP0 with valid complete inputs has lower0.901099007676,
  upper1 and CONFIRMED; it is only eligible-by-replay, subject to other gates.
- KindB TP29/FP0 is INCONCLUSIVE/unserved because29<30, even with a perfect
  point; TP30/FP0 gives lower0.886486606826/upper1 and is still INCONCLUSIVE.
  Neither minimum30 nor point1 is automatic confirmation.
- TP27/FP3 has point0.9 and Wilson interval[0.743789174208,0.965400111253],
  INCONCLUSIVE; TP20/FP10 has[0.487800516445,0.807695019163], REFUTED.
  TP0/FP29 remains INCONCLUSIVE due minimum30 even though its point is low.
  Bounds touching0.9 classify INCONCLUSIVE. Source-value equality unchanged.
- KindA35/35 plus KindB29/29 yields a perfect pooled64/64 summary; KindB stays
  INCONCLUSIVE/unserved and its result is byte-identical whether KindA/pooled
  counts are present, absent or arbitrarily large. No pooled gate is consulted.
- Thirty declaration/instance flags in ONE pair count as one positive pair,
  not30. Duplicating/reordering the same pair id cannot increase n. A pair
  with one true and one false declaration claim is exactly one FP, no TP;
  mixed claims cannot hide behind duplicated true addresses.
- Missing kind receipt, failed control, unjoined native labels or mismatched
  roster never defaults to eligible/served. H2/H5 remain advice. Unserved
  siblings do not erase valid KindA evidence. #302 HZ-M1 proof still required.
- Undefined ratios/intervals and invalid counts have explicit states/refusal;
  identical per-kind counts under order permutation yield identical Wilson
  bounds/outcomes. No display-rounded ratio or resource total substitutes n.

Add test_manifest.py::test_w12_extension_registration: every intended kind has
its own immutable public-history initial/reserve membership and finite look/
stop registration with hash frozen before ANY run; missing horizon/limit or
changed hash/selection after data refuses dispatch. INCONCLUSIVE activates only
its registered next reserve, final inconclusive stops unserved, no other-kind
or pooled rescue. Test with named synthetic finite look plans; no numeric plan
in such a fixture is a real approved extension or operator input. All native
pairs, labels, reserves and scores remain absent in this authoring session.

The numerical vectors above are deterministic arithmetic checks only, never
a chosen extension size:35 is NOT an authorized reserve/target. Implementing
critic must check each assertion against actual tests; none ran RED/GREEN here.

## Decidability and soundness scorer (#302)

Entry requires BOTH HZ-3 and the complete HZ-M1 reading handed back PROVEN,
plus the actual #310 reader handback. Source: #302 Contract handed explicitly
requires HZ-M1 proven; its Dispatch state also lists HZ-M1. HZ-17's narrower
plan needs do not amend that handed contract. A parser/fixture/executable digest
is necessary but cannot stand in for the replay reading, full acceptance
assertions, independent review and actual row-proof receipt. No kernel/OQ3 yes
is required. W1..W12 contain no amendment removing #302's HZ-M1 prerequisite.
W12 per-kind admission does not itself prove the complete HZ-M1 reading or
waive this handed #302 proof/manifest join.
W12-3 settles exactly90 roots,30 in each module stratum, seeded without
replacement. Freeze the independent public module frame, exact revisions and
file rosters; strata are no module calls / local-only modules / any vendored
remote module.
Mixed local+remote goes in the last stratum only. Rank each eligible frame key
by SHA-256 of fixed seed `20261115`, a length-prefixed repo/revision/root tuple
and stratum, then sample without replacement within each stratum. Frame and
selected ids freeze before evaluation. Exclusions for licence, unavailable
source or native support must be applied before selection; runtime failures
stay in the selected cohort. Select exactly30 distinct roots per stratum,
90 total, using the unchanged seed/ranking rule. Root identity is canonical
(repository,root path), with one pinned revision per root before sampling;
multiple revisions of one root cannot manufacture distinct roots. A frame short of30 in any
stratum is HOLD, never a smaller/replacement/duplicated stratum or an imported
root from another stratum. No real frame is supplied here. W12-3 settles the
old90-versus120 question; membership/fingerprint must freeze before runs.

Each declaration attribute in the complete source/native population gets one
row keyed by source identity, declaration address, path and instance set. Union
both readers and native known attributes; report source-only/native-only and
unjoined rows instead of intersecting to successes. Defaults-only files count.
Same root/vars/workspace/native plan and vendored module FS go to both arms;
Trivy downloads disabled. Binary resources output can opt into values only in
the isolated bench; the product never does. A sensitive/withheld value remains
ungraded even when its class is definite.

For each arm, D is definite literal/defaulted (fathom) or resolvable (Trivy),
N is the full registered in-scope attribute population, G is definite rows with
known nonsensitive native truth and complete instance join, C is those G rows
whose typed value equals every corresponding native leaf. Report D/N and C/G,
D-G ungraded, all unknowns by HZ-3 reason, strata, class and arm, with module
weights and pooled raw counts. Both arms use identical N, publish each extra
unjoined population. Exact numbers, maps/lists, null/absence and booleans are
not string-coerced. C/G is undefined at G=0. Never call ungraded definite rows
sound or report soundness=1 merely because they have no counterexample.

Independent source-feature labels map unresolved contributors to E2 member
opacity, E4 bounded pure-function expressions, E5 local-module expansion.
Show overlaps and exclusive-only counts, and native/Trivy resolution support;
rank by exclusive attributable unknowns at equal measured soundness, then id
for a deterministic tie. Coarse reason codes alone cannot establish a cause.
Do not implement E2/E4/E5 or claim a hypothetical uplift measured. HZ-M4 is a
complete descriptive reading even when results disfavor fathom. 'Trivy decides
far more' has no authored numerical threshold; report exact differences and
return that trigger interpretation to architect, never invent a new floor.

## Prospective tests: every assertion

| Exact owned test | Required assertions |
|---|---|
| test_plan_reader.py::test_native_change_truth | Both replacement action orders positive; create/delete/update/no-op/read distinct; absent replace_paths still replacement; numeric/string path segments exact; unknown action/format/malformed/duplicate resource refuses; new optional reason unspecified; unknown/null/absent/sensitive distinct; nested module/quoted instance joins exact; divergent instance values unjoined; truncated plans refuse. |
| test_manifest.py::test_registered_replay | Unique ids/pins/revisions/state recipe required; PUBLIC HISTORY frame, per-kind scope/labels/seed/initial+reserve roster/hash and finite look plan frozen before any run; changed membership/hash, post-score selection, failed-case substitution or missing extension plan refuses; all selected cases accounted; bad state/nonclean base control cannot score; canary absent; exact binary schema. W12-2, no invented extension n. |
| test_score.py::test_replay_floor | TP9/FP1 point0.9 but10<30 positive pairs => INCONCLUSIVE/unserved; TP8/FP1 below point floor; injected FP/FN correct cells; zero positives undefined/inconclusive on otherwise valid data; missing H5 label/unjoined H2/bad control makes that kind incomplete/unserved; H5 not replacement; accepted-stateful absence cannot label H1; reorder identical. W12-2. |
| test_decidability.py::test_sample_and_scores | W12-3 exactly90 distinct roots/30 per stratum, seeded without replacement; insufficient stratum/duplicate root/cross-stratum substitution/changed count refuses; same seed/frame yields identical unique samples independent of order; mixed modules assigned once; changed frame invalidates freeze; union N unchanged across arms; N=10 D=6 G=5 C=4 yields 0.6 decidability, 0.8 soundness, 1 ungraded definite; G=0 undefined; sensitive/missing/dynamic/default-only rows retained; exact typed mismatch counts; missing-variable Trivy fallback cannot become sound without native equality; E2/E4/E5 overlaps never double-count exclusive ranking; no future engine work. |
| test_decidability.py::test_shared_reader_handoff | W12-4 exact four extra helper/data/recipe paths have one #310 writer then immutable #302 read-only handback; concurrent writer/unlisted path/missing incoming/outgoing commit refuses; no product permission inferred. #302 still requires proven HZ-3 AND the complete HZ-M1 reading/proof handback, plus exact #310 reader source/executable/fixture digests; parser-only handback, missing/failed/inconclusive/unproven HZ-M1, dropped native join or scanner finding-count substitution each blocks entry; a complete matching HZ-M1 proof plus other inputs admits it; plan reader reused without copy; no product values. |

Prospective command: `python3 -I -m unittest discover -s docs/bench/harnesses/hazard-plan -p 'test_*.py'`.
All additional W12 assertion rows below apply to the same owned test files;
none is implemented by this document. Actual RED belongs to #310 implementing
lane, then independent assertion check
and GREEN; native shakedown has no-claim status. Heavy work waits for the shared
lease and absence of GATE-RUNNING. No tests, synthetic state, native commands or
reading ran here. Both row verifications need independent evidence after a
scratch ingest; artifact/lexical presence alone cannot claim either row.

## W12 settled choices, remaining extension hold and dispatch

W12-2 supersedes former REPLAY/U1 and U2: >=30 positive public-history pairs
for each kind sought for admission, own0.9+interval, kind is the admission unit,
pooled is summary only, <30 INCONCLUSIVE/unserved, preregistered extension.
No fixed60/100 overall replay sample or pooled admission requirement remains.
W12-3 supersedes U3: exactly90 roots/30 per module stratum, seeded without
replacement. W12-4 supersedes U4: the four exact extra paths are granted,
one #310 writer per turn and #302 read-only. Those choices are closed, not
renewed approval questions. Grants still need real activation handbacks.

U5 unresolved numerical extension horizon/finite stopping plan: W12-2 requires
an extension when a kind is INCONCLUSIVE but gives no added/final pair count,
look limit or candidate cap. Recommend a finite per-kind reserved roster/next
look justified by disjoint pre-data power/yield evidence; alternative one
larger fixed final look. Neither numerical choice has a governing basis here.
Do NOT import CFN's100, invent35 as an extension, stop at the first favorable
precision, or choose a horizon after data. The actual initial/reserve ids,
roster hash, cumulative candidate limits, maximum looks and final stop policy
must be frozen before ANY confirmatory run. Missing that complete registration
is a dispatch HOLD. At an INCONCLUSIVE permitted look use only its predeclared
reserve for that kind; at the registered final look stop unserved if still
INCONCLUSIVE. No outcome or criterion is rewritten after data. No raw data,
look counts, numerical extension or authorization is supplied by this draft.

Required missing inputs: real public-history frames/pairs/outside labels,
90-root frame, provider binaries, state recipes, catalogs, native/runtime/
adapter digests, actual incoming/outgoing grant handbacks and frozen extension
registration. #310 still needs proven HZ-9/HZ-10/LIB-7; #302 still needs proven
HZ-3 AND complete matching HZ-M1 proof, plus actual immutable reader. A per-kind
admission result cannot waive these authored prerequisites or scratch proof.
Official critic recheck and W2-3 determination remain pending. No default,
product serving, row claim or self-approval follows from contract presence.

## Decision models

None. Deterministic native truth and independent labels; provider none.
Opt out by leaving an unearned reading INCOMPLETE, never by relabelling unknown.

## Normative private plan-reader records and wire schema

This closes the local port's record shape, not a new exported Go API. All fields
below are mandatory unless explicitly optional. Lists initialize empty, never
null. No raw native object escapes through a result/error rendered to the reading.

| Private record | Exact fields and types |
|---|---|
| Address | tool: AddressTool (terraform here), module: str, mode: AddressMode, type: str, name: str; frozen/comparable tuple with the CTR-10 field-order equality, never joined sentinel text. |
| ResourceTruth | address: Address; instance_key: str or int or None; actions: tuple[NativeAction,...]; replace_paths: tuple[tuple[str or int,...],...]; action_reason: str or None (native optional open hint). |
| AttributeInstance | instance_key: str or int or None; status: TruthStatus; value: private exact JSON value or None. status=known with value=None means known null, distinguished from absent; sensitive/unknown/unjoined carry no value. |
| AttributeTruth | address: Address; path: tuple[str or int,...]; status: TruthStatus; instances: tuple[AttributeInstance,...]. Aggregate known only if the complete nonempty instance set is known and equal; otherwise retain individual statuses and aggregate unknown/unjoined as specified above. |
| PlanTruth | format_version: str; terraform_version: str; changes: tuple[ResourceTruth,...]; attributes: tuple[AttributeTruth,...]. Parsing stores only the required changes and complete normalized attribute rows, not a provider/raw-plan handle. |
| PlanReadError | kind: PlanErrorKind; locator: str (JSON field path only, no input value/source); raised, not returned as a successful empty PlanTruth. |

Named local closed enums/constants:
NativeAction NO_OP=no-op, CREATE=create, READ=read, UPDATE=update, DELETE=delete.
TruthStatus KNOWN=known, UNKNOWN=unknown, ABSENT=absent, SENSITIVE=sensitive,
UNJOINED=unjoined. PlanErrorKind INPUT=input, PIN=pin, FORMAT=format, PARSE=parse,
LIMIT=limit, DUPLICATE=duplicate, JOIN=join, NATIVE=native. New optional native
reason hints remain open text and do not become an enum error. Resource action
arrays still accept only the exact known sequences stated above.

Numbers parse with exact integer/decimal semantics using Python's standard
library Decimal for fractions/exponents, never binary-float coercion. The
reader refuses nonfinite numbers and duplicate JSON object keys. Typed native
null/boolean/string/list/object remain distinct. A source value that cannot be
represented and joined exactly is ungraded, never coerced to fit the oracle.
The private value exists only to compare bench truth, not as a public field.

Replay score JSON has exactly schema_version=1, admission_unit="kind",
evidence_status (complete/incomplete), cases_total,cases_completed,cases_failed,
cases_excluded, kinds (sorted array), pooled, failures:[], exclusions:[]. No
global PASS/admitted flag exists. Each kind record has id,evidence_status,counts_are_lower_bounds,
positive_pairs,tp,fp,fn,tn,abstained,unlabelled,unjoined,precision,recall,
precision_interval,outcome,admission,reason. tp/fp are the distinct pair-level
counts defined above, not raw address counts; positive_pairs=tp+fp.
Outcome is CONFIRMED/REFUTED/INCONCLUSIVE for valid complete evidence only;
for operationally incomplete evidence omit precision,recall,outcome,precision_interval,
mark counts_are_lower_bounds=true, preserve observed coverage/counts and its
explicit reason, admission unserved. Complete evidence has that flag false.
Do not fabricate zero counts or complete totals for unread inputs. This is
not a fourth statistical outcome. Admission is named closed unserved/eligible-by-replay, never served.
The only eligible-by-replay value requires W12's kind gates AND a valid matching
preregistration; actual serving still needs separate acceptance/consumer proof.
Pooled has only descriptive counts/ratios, NO outcome, interval-admission gate
or admission field. Source/address-level details remain private witnesses.
Failures are sorted case_id,kind,locator; exclusions sorted case_id,reason.
Each ratio has numerator,denominator,status,display; status defined/undefined,
display six fractional digits half-even or empty if undefined. Exact integers
provide the necessary point-bar comparison, Wilson bounds provide the
three-outcome decision, and minimum30 takes precedence; display never grades.
No null ratio masquerades as a zero. No raw literal/source payload is emitted.
precision_interval has method="wilson-score", confidence=0.95, status in
{defined,undefined}; defined includes finite numeric lower,upper in[0,1] at
full calculation precision, undefined omits both bounds (no invented zero/null
interval). Method/status are named local closed enums/constants. Admission
uses named local KindAdmission constants Unserved="unserved" and
EligibleByReplay="eligible-by-replay"; outcomes use named KindOutcome constants
Confirmed/Refuted/Inconclusive with the uppercase wire values above. No Served
constant or default exists. An invalid/missing kind result cannot grant a consumer
an eligibility receipt. No top-level or pooled acceptance is retained.
The existing prospective test_kind_admission_w12 also rejects an unknown/global
admission field, a pooled outcome/admission field, a Served enum/default,
undefined interval with invented bounds, and complete-looking zero counts for
unread input; missing/incomplete evidence is explicitly unserved.
Decidability score rows are stratum,arm,n,definite,graded,correct,
ungraded_definite,decidability,soundness,unknown_by_reason; the same ratio object
and mandatory arrays apply. Separate class/population/feature tables use these
counts, never source literal payloads. These private wire schemas require no
new public fields and no live benchmark value output.

Additional exact assertions in test_plan_reader.py::test_native_change_truth:
duplicate JSON keys and nonfinite numbers raise PlanReadError; decimals/exponents
and integers compare exactly without string coercion; known null differs from
absent, boolean differs from integer, list order preserved, object key order
irrelevant. test_score.py::test_replay_floor must assert every undefined ratio
has status undefined and denominator zero, every empty list is [], and every
score JSON key matches this closed envelope without raw value fields.


## Critic FIX round 1: prerequisite and ratio corrections

F1: Restored #302's actual handed HZ-M1 PROVEN requirement at entry, the shared
reader assertion and final readiness list. The source is #302 Contract handed /
Dispatch state, not a guessed operator receipt; no receipt is supplied here.
The HZ-M1 source/head, registered manifest, complete reading verdict and its
independent/proof receipts must identify the same completed replay. A real
provenance mismatch blocks #302. Neither artifact presence nor a parser digest
establishes that reading. No issue or plan is edited by this draft.

F2: Ratio serialization uses exactly six fractional digits, nearest rounding
with ties to even. For nonnegative integer numerator n and denominator d>0,
round n/d to that display precision without binary-float conversion. It is a
presentation value only. Status undefined requires d=0 and empty display;
negative counts/denominators are typed input refusals. The exact rational n/d
and integer floor comparisons remain authoritative (HZ-M1 10*TP >= 9*(TP+FP)).
This is serialization compliance under I38/P22, not a floor amendment. Alternative
an exact fraction display is representable but not selected for this decimal
wire field. Exact Decimal semantics for private source-value comparisons remain
unchanged; a rounded score display never rounds a source value.

Additional exact test_score.py::test_replay_floor assertions:
1/3 displays "0.333333" and retains n=1,d=3; 2/3 displays "0.666667";
9/10 displays "0.900000" and meets the integer point0.9 floor but is
INCONCLUSIVE/unserved because10 positive pairs is below30 (W12-2). With
TP=2699999, FP=300001, precision=2699999/3000000 displays "0.900000" but
FAILS: 10*2699999=26999990 < 9*3000000=27000000. Zero denominator remains
undefined/empty, not "0.000000". Half-even tie witnesses 1/2000000 ->
"0.000000" and 3/2000000 -> "0.000002". All are prospective scorer assertions,
not executed implementation tests or measured replay data.

## W12-2..4 amendment receipt

This append-only amendment to reviewed1c101a88 changes only this contract.
W12-2 settles former U1/U2 (kind admission,>=30 positive public-history pairs,
own0.9 plus published interval, pooled summary only, short kinds unserved,
preregistered extension); W12-3 settles U3 (exact90/30 seeded roots); W12-4
settles U4 (four exact extra files,#310 one writer,#302 read-only). Source:
ctx/RULINGS-W12.md, applied to ownership, population, scorer, assertions, wire
and dispatch. U5 records the numerical extension/stopping basis not supplied
by the ruling. No data/measurement, tests, source implementation, row/default
acceptance, self-approval or root-board change is commissioned here. Official
critic main will recheck the exact new READY commit; prior LAND is historical.
