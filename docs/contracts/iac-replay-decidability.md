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

## Exact paths and ownership

#310's new directory grant narrows to
`docs/bench/harnesses/hazard-plan/{run.py,plan_reader.py,score.py,manifest.py,test_plan_reader.py,test_score.py,test_manifest.py,replay.json,pins.json}`
and `docs/bench/readings/change-hazards/m1-replay.md`.
#302 owns only `docs/bench/readings/change-hazards/m4-decidability.md`.
Shared helpers for this group are exclusively #310's proposed new
`docs/bench/harnesses/hazard-plan/decidability.py`, `test_decidability.py`,
`decidability.json`, and `trivy-resolution.md` (the native adapter build/run
handoff recipe, not a product package/dependency). These exact extra files
require an explicit integrator grant before implementation. #302 consumes
that executable/reader at the #310 handback commit, never edits its source.
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
required, not an assertion that some successful roots were found. U1 fixes
sample minimum below; no real frame/case/state recipe is delivered today.

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
control makes the case ungradable and prevents a complete-cohort PASS.

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

Score unique (case_id,hazard id,declaration address) once, retaining all native
instance witnesses and each prediction span/citation. Publish TP/FP/FN/TN,
abstentions, unlabelled, unjoined, failed and excluded per H1/H2/H4/H5 and pooled.
Precision TP/(TP+FP), undefined if no predicted positives. Recall TP/(TP+FN)
is descriptive; zero is undefined. Proposed bar comparisons use exact integers
10*TP >= 9*(TP+FP), never rounded output. Full HZ-M1 cannot PASS on one kind
while another lacks labels or predictions. U2 below settles per-kind versus
pooled interpretation without changing 0.9. All failed controls/native reads
remain visible and prevent a complete registered-cohort PASS.

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
conflicts must precede product output. Without them full HZ-M1 is INCOMPLETE.
Replay is a noise reading; ADR-106 still requires outside labels, strength and
person acceptance before a drafted check may serve, with H2/H5 advice retained.

## Decidability and soundness scorer (#302)

Entry requires BOTH HZ-3 and the complete HZ-M1 reading handed back PROVEN,
plus the actual #310 reader handback. Source: #302 Contract handed explicitly
requires HZ-M1 proven; its Dispatch state also lists HZ-M1. HZ-17's narrower
plan needs do not amend that handed contract. A parser/fixture/executable digest
is necessary but cannot stand in for the replay reading, full acceptance
assertions, independent review and actual row-proof receipt. No kernel/OQ3 yes
is required. W1..W5 contain no amendment removing #302's HZ-M1 prerequisite.
Freeze independent public module frame with exact revisions and file rosters;
strata are no module calls / local-only modules / any vendored remote module.
Mixed local+remote goes in the last stratum only. Rank each eligible frame key
by SHA-256 of fixed seed `20261115`, a length-prefixed repo/revision/root tuple
and stratum, then sample without replacement within each stratum. Frame and
selected ids freeze before evaluation. Exclusions for licence, unavailable
source or native support must be applied before selection; runtime failures
stay in the selected cohort. U3 fixes n below. No raw frame is supplied here.

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
| test_manifest.py::test_registered_replay | Unique ids/pins/revisions/state recipe required; mutated byte/native/provider/catalog digest refuses; all selected cases accounted; invalid/empty synthetic state and nonclean base control cannot become a head score; no failed-case substitution; credentials/source canary absent from public output; exact binary schema required. |
| test_score.py::test_replay_floor | TP=9 FP=1 at 0.9; TP=8 FP=1 below; injected FP and FN alter their own cells; zero positives undefined; missing H5 label/unjoined H2 pair/failed native control yield INCOMPLETE; H5 is not graded as replacement; accepted-stateful absence cannot label H1; input reorder byte-identical. |
| test_decidability.py::test_sample_and_scores | Same seed/frame yields identical unique per-stratum samples independent of order; mixed modules assigned once; changed frame invalidates freeze; union N unchanged across arms; N=10 D=6 G=5 C=4 yields 0.6 decidability, 0.8 soundness, 1 ungraded definite; G=0 undefined; sensitive/missing/dynamic/default-only rows retained; exact typed mismatch counts; missing-variable Trivy fallback cannot become sound without native equality; E2/E4/E5 overlaps never double-count exclusive ranking; no future engine work. |
| test_decidability.py::test_shared_reader_handoff | #302 requires proven HZ-3 AND the complete HZ-M1 reading/proof handback, plus exact #310 reader source/executable/fixture digests; parser-only handback, missing/failed/inconclusive/unproven HZ-M1, dropped native join or scanner finding-count substitution each blocks entry; a complete matching HZ-M1 proof plus other inputs admits it; plan reader reused without copy; no product values. |

Prospective command: `python3 -I -m unittest discover -s docs/bench/harnesses/hazard-plan -p 'test_*.py'`.
Actual RED belongs to #310 implementing lane, then independent assertion check
and GREEN; native shakedown has no-claim status. Heavy work waits for the shared
lease and absence of GATE-RUNNING. No tests, synthetic state, native commands or
reading ran here. Both row verifications need independent evidence after a
scratch ingest; artifact/lexical presence alone cannot claim either row.

## Foreman batch and operational holds

U1 (#310): recommend 60 frozen history pairs, source-frame census and all failures
retained; alternative 100. HZ-M1 fixes 0.9 but no cohort size.
U2 (#310): recommend 0.9 per H1/H2/H4/H5 plus pooled, each with blind outside
labels and nonzero positives; alternative pooled-only with per-kind disclosure.
Row fixes 0.9, not pooling/minima; H5 truth remains transition fidelity.
U3 (#302): recommend 90 roots, 30 per module stratum, deterministic seeded
sampling without replacement; alternative 120/40. HZ-17 requires stratification,
not n. Insufficient frame blocks, never backfills after outcomes.

Required missing inputs: real frames/pairs/modules, labels, provider binaries,
synthetic recipes, catalogs, native executable/runtime/adapter handback digests,
proven HZ-3 AND complete HZ-M1 proof for #302, proven HZ-9/HZ-10/LIB-7
entries for #310, exact added file grants and
independent critic/rulings. A complete contract protocol is not these inputs.

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

Replay score JSON has exactly schema_version=1, verdict (PASS/FAIL/INCOMPLETE),
cases_total, cases_completed, cases_failed, cases_excluded, kinds (sorted array
of id,tp,fp,fn,tn,abstained,unlabelled,unjoined,precision,recall), pooled (same
counts/ratios without id), failures (sorted array of case_id,kind,locator), and
exclusions (sorted array of case_id,reason). Each ratio is an object with
numerator,denominator,status and display; status is defined/undefined, display
is a fixed six-fractional-digit decimal rounded half-even when defined and
empty otherwise. Exact numerator/denominator are always retained; acceptance
uses their integer comparison, never display. No null numeric
ratio can masquerade as zero. Reasons/case identifiers contain no raw values.
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
9/10 displays "0.900000" and meets the integer 0.9 floor. With
TP=2699999, FP=300001, precision=2699999/3000000 displays "0.900000" but
FAILS: 10*2699999=26999990 < 9*3000000=27000000. Zero denominator remains
undefined/empty, not "0.000000". Half-even tie witnesses 1/2000000 ->
"0.000000" and 3/2000000 -> "0.000002". All are prospective scorer assertions,
not executed implementation tests or measured replay data.
