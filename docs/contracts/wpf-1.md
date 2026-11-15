# WPF-1 — phase-separated wiring conformance schema

Status: draft for independent contract criticism; not an implementation or approval receipt.
Scope: `docs/plans/wiring-proof.md`, WPF-1. Governing design:
`docs/design/wiring-proof.md` WP-D1, WP-D3, WP-D6 (WP-D2 clarifies admission).
ADR-110 remains proposed; this contract specifies the assigned work, not its acceptance.

## Intent and decision points

Consulted: `docs/intent/README.md`, `scope.md` and `scope/non-goals.md`,
`principles.md`, `invariants.md`, `futures.md`, `decisions/README.md`,
ADR-110 and ADR-105 decision 9. Applicable constraints: P2/P3/P17/P18 require
computable checks, visible unknowns and computed status; I9/I12/I35/I53 preserve
fail-open hooks and existing payload/deadline bounds; I13 preserves receipt-owned
writes; I62 excludes judgment from proof. These invariants have enforcement notes
in `invariants.md`; mentioning them here does not extend their enforcement.
No dispatch loop, runner, sampler, table, MCP verb, production override, default
change, paid experiment or future map feature is authorized. No matching future
promotion or revisit trigger is evidenced by this document-only work. A harness
unable to expose a required observation would fire ADR-110's first revisit trigger;
no live harness is evaluated here. Compatibility: new file-only shapes, no change
to an existing schema or published API. Free option: separate records preserve
later adapters without building them. No intent record needs changing.

| Decision | Settled choice and governing text |
|---|---|
| D1 Ownership | Four new implementation files below, exactly as WPF-1 names; no second package or validator API. |
| D2 Admission boundary | `#Preflight` contains only W0/W1/W2/W3/W6; `#Outcome` owns W4/W5 and delivery bars. `#Conformance` is a reporting envelope, never a gate argument. WP-D1/D2, WPF-1. |
| D3 Closed wire shapes | Definitions and nested structs are closed, explicit lower_snake_case keys, finite vocabularies, no extension map. W3-4; LAB-1's definitions and required-field conventions in `schema/labs/schema.cue`. No Go exported types are needed. |
| D4 Evidence and identity | Explicit phase, source kind, range, clock, pins, revisions and digests; sorted evidence; no transcript text. WP-D3/D6. |
| D5 Validation mechanism | Existing CUE Go API, independent mutations of validated positives, field-specific diagnostics; `schema/labs/schema_test.go` and WPF-1. No shell CUE process in Go tests, no new dependency. |
| D6 Defaults and absence | No implicit PASS, missing samples stay UNKNOWN, N/A needs a preregistered absence proof, daemon-free arms have no invented stamp. WP-D1/D2, ADR-105 decision 9. |
| D7 Windows and cloud | Actual harness plus environment, explicit WSL witness fields; fixtures never certify installation. WP-D5/D6, WPF-7/8. |
| D8 Routine encodings | Exact definitions below use LAB-1 SHA/text/arm vocabulary, W3-4 closed JSON, and WP-D3's canonical effective data. Encoding choices impose no extra eligibility policy. |

W2-3 approval still requires an independent LAND verdict and citations for every
decision; W3-2 requires implementing tests to match every assertion below. Exact
field names and conservative mechanics are settled here under D3/D5/D8, not
escalated for lack of literal schema prose. No unresolved policy decision was
identified. A missing downstream interface is a dependency, not permission to invent it.

## Files and implementation boundary

The implementing lane owns only these new files:

- `schema/wiring-proof/schema.cue`
- `schema/wiring-proof/testdata/valid.cue`
- `schema/wiring-proof/testdata/invalid.cue`
- `internal/wiring/conformance_schema_test.go`

This authoring lane changes only this contract and its local handback. No plan box
is checked and no changelog fragment is required for a draft. Schema package name:
`wiringproof`. The test reads the schema and fixtures using repository-relative
paths from `internal/wiring`; no embed package, production Go type, exported
function, context parameter, typed error or new runtime dependency is necessary.
Existing `schema/labs` is vocabulary precedent only and is not extended or imported
through a fabricated CUE module mapping. Copy only the scalar constraints needed.

## Exact wire definitions

Notation in the tables is normative: every listed field is required unless marked
`?`; every object is closed, including list elements. Lists are present even when
empty, never JSON null. Lists described as sets are strictly increasing in their
stated key (thus unique). Required nonempty lists have length >= 1. No default
values. Implement these constraints in CUE definitions, including cross-field
relations; the tests exercise CUE itself, not a parallel Go validator.

Scalar definitions:

- `#Text`: string matching `\S`, as LAB-1; `#LabID`: `^lab-[a-z0-9]+(-[a-z0-9]+)*$`.
- `#SHA`: `^[0-9a-f]{64}$`, as LAB-1. `#Commit`: exact lowercase Git object ID,
  `^([0-9a-f]{40}|[0-9a-f]{64})$`; no branch, short SHA or ancestor substitution.
- `#Time`: integer >= 0, UTC Unix nanoseconds from an injected fixture clock.
- `#EnvRung`: `"W0" | "W1" | "W2" | "W3" | "W6"`.
- `#Verdict`: `"PASS" | "FAIL" | "UNKNOWN" | "NOT-APPLICABLE"`.
- `#Phase`: `"preflight" | "agent"`; `#Harness`: `"claude-code" | "codex-cli"`.
  These are actual harness families, separate from LAB-1's bench instrument
  `#Harness` vocabulary; cloud and WSL are environments, not new transcript families.
- `#EnvironmentKind`: `"local" | "cloud" | "wsl"`; `#HostClass`:
  `"macos" | "linux" | "windows"`; `#SourceKind`:
  `"registration" | "negotiation" | "call_result" | "startup_delivery" |
  "hook_trigger" | "hook_event" | "beat" | "revision" | "resource_sample" |
  "channel_inventory" | "eligibility" | "action"`.
- `#BenchHarness`: LAB-1's exact enum `"replay" | "fork" | "scenario" |
  "hook-path" | "decide" | "indexbench" | "reviewbench" | "contextbench" |
  "langbench"`. `#Channel`: `"surface" | "stanza" | "descriptions" | "hooks" |
  "skills" | "prime" | "maps" | "readouts"`.

| Definition | Fields and constraints |
|---|---|
| `#Pins` | `harness: #Harness`, `harness_pin: #Text`, `model_pin: #Text`, `transcript_contract_digest: #SHA`, `product_commit: #Commit`, `subject_commit: #Commit`, `plugin_digest: #SHA`, `stanza_sha: #SHA`, `description_sha: #SHA`. LAB-1 pin keys retain their spelling; model pin is data, not authority for a call. Product commit remains the preregistered build even when absent from a baseline. |
| `#Environment` | `kind: #EnvironmentKind`, `host_class: #HostClass`, `harness_pin: #Text`, `permitted_tools: [...#Text]` sorted by value, `startup_instructions_digest: #SHA`, `action_capture_digest: #SHA`; WSL only: required `guest_revision: #Text`, `path_translation: [...#Translation]` nonempty and sorted by `guest_path`; otherwise these two keys forbidden. WSL requires `host_class: "windows"`, `harness: codex-cli` via parent pins. |
| `#Translation` | `guest_path: #Text`, `fixture_path: #Text`; both relative slash paths with no leading slash, drive prefix, backslash, empty segment, `.` or `..` segment. No machine path in public data. |
| `#ChannelSpec` | `channel: #Channel`, `enabled: bool`, `content_digest: #SHA`; empty/off contents still hash canonical empty data. |
| `#HookSpec` | `id: #Text`, `enabled: bool`, `trigger_digest: #SHA`, `event: #Text`, `timeout_ns: int & >0`. Inventory includes disabled and unsupported declared hooks, not only observed successes. |
| `#Requirement` | `rung: #EnvRung`, `applicable: bool`, `reason: #Text`. Exactly five entries, ordered W0,W1,W2,W3,W6. All applicable rungs are required; no separate skip flag. |
| `#Bound` | `metric: #Metric`, `maximum: number & >=0`. Sorted unique by metric; registration chooses bounds before data. |
| `#Metric` | `"index_age_ns" | "payload_age_ns" | "hook_p95_ns" | "phys_footprint_bytes" | "resident_bytes" | "cpu_seconds_per_ingest"`. Units are in names, never interchangeable heap measurements. |
| `#Budget` | `tenant_policy_digest: #SHA`, `bounds: [...#Bound]` nonempty. Must contain index/payload ages and hook p95 when their channels are enabled; daemon-free profiles require the host resident/footprint bound but do not acquire daemon CPU/age requirements. Apple hosts use footprint, Linux/WSL use resident; exact limits are preregistered inputs, not invented global bars. FT-D4 and WP-D1 W6. |
| `#Profile` | `id: #Text`, `lab: #LabID`, `bench_harness: #BenchHarness`, `pins: #Pins`, `environment: #Environment`, `channels: [...#ChannelSpec]` exactly all eight in lexical channel order, `hooks: [...#HookSpec]` sorted by id, `surface_tools: [...#Text]` sorted unique, `requirements: [...#Requirement]`, `workload_digest: #SHA`, `budget: #Budget`, `budget_digest: #SHA`, `source_digests: [...#SourceDigest]` nonempty sorted by id (must include `delivery_parameters`), `profile_digest: #SHA`, `environment_digest: #SHA`, `frozen_at: #Time`. No delivery bars or k. |
| `#SourceDigest` | `id: #Text`, `digest: #SHA`; references existing labs/knob, skill, primer, map and readout source contents; no path-derived substitute digest. |
| `#Evidence` | `id: #Text`, `source_kind: #SourceKind`, `phase: #Phase`, `session_id: #Text`, `call_id: #Text`, `range: {artifact_digest: #SHA, start: int & >=0, end: int & >start}`, `observed_at: #Time`, `product_commit: #Commit`, `subject_commit: #Commit`, `profile_digest: #SHA`, `harness_pin: #Text`, `reason: #Text`. Range is half-open byte offsets into a local source artifact identified by hash. `call_id` is the correlated scripted/agent operation id even for non-tool events; it does not imply an agent call. |
| `#RungResult` | `verdict: #Verdict`, `reason: #Text`, `evidence_ids: [...#Text]` sorted unique; nonempty except UNKNOWN, which may have no observation. |
| `#Absence` | `channel: #Channel`, `evidence_ids: [...#Text]` nonempty sorted unique. No bare boolean constitutes absence evidence. |
| `#HookObservation` | `hook_id: #Text`, `trigger_evidence_id: #Text`, `event_evidence_id: #Text`, `beat_evidence_id: #Text`. |
| `#Sample` | `metric: #Metric`, `value: number & >=0`, `evidence_ids: [...#Text]` nonempty sorted unique. One entry per observed bound metric, sorted by metric; no sample means unknown, not zero. |

Digest algorithm (D8, WP-D3/D6): SHA-256 over UTF-8 compact canonical JSON;
object keys lexical, no insignificant whitespace, integers decimal, other numbers in non-exponent decimal with no redundant
leading/trailing zeros (zero is `0`), all numbers finite, arrays in the declared orders. Reject duplicate keys before hashing.
`budget_digest` hashes `budget`; `environment_digest` hashes `environment`;
`profile_digest` hashes the entire effective profile excluding only
`profile_digest` and `frozen_at` (includes the budget/environment/source digests,
pins, requirements and workload). Workload digest hashes the fixed script content
plus its fixture/input manifest; a profile name is never hash input by itself.
CUE constrains syntax and binding equality, not cryptographic recomputation.
WPF-9 owns effective profile hashing and undeclared-difference detection; WPF-1
must not claim to prove a supplied hash matches unseen source content.

### Preflight: admission input only

`#Preflight` is a closed object with:

| Field | Type / rule |
|---|---|
| `session_id`, `cell_id`, `arm` | `#Text`, retaining LAB-1's assigned arm verbatim |
| `phase` | literal `"preflight"` |
| `profile` | `#Profile` |
| `started_at`, `finished_at`, `decision_at`, `agent_start_at` | `#Time`; `profile.frozen_at <= started_at <= finished_at <= decision_at < agent_start_at` |
| `startup_environment_digest`, `startup_profile_digest`, `workload_digest`, `budget_digest` | `#SHA`; equal profile environment/profile/workload/budget digest respectively |
| `rungs` | closed `{w0: #RungResult, w1: #RungResult, w2: #RungResult, w3: #RungResult, w6: #RungResult}`; no w4/w5 key |
| `served_product_commit?`, `installed_product_commit?`, `index_commit?` | `#Commit`; for W0 PASS both product fields required and equal `profile.pins.product_commit`; for W6 PASS with treatment surface enabled, index commit required and equal subject commit; baseline omits index_commit |
| `primer_digest?` | `#SHA`; W2 PASS requires equality to prime channel content digest |
| `negotiated_tools`, `answered_probe_ids` | sorted unique text lists; W1 PASS requires negotiated tools equal profile surface tools and >=1 answered probe reference |
| `hook_observations` | sorted unique `#HookObservation` list by hook_id; W3 PASS covers exactly all enabled declared hooks, no extra ids |
| `baseline_checks?` | Closed `{harness_evidence_id: #Text, tools_evidence_id: #Text, startup_evidence_id: #Text, action_capture_evidence_id: #Text}`; required only when surface is off, forbidden otherwise. References resolve to revision, channel_inventory, startup_delivery and scripted action evidence respectively. |
| `absences` | `#Absence` list sorted unique by channel; every disabled channel has exactly one absence entry, no enabled channel may appear |
| `samples` | `#Sample` list; W6 PASS requires all applicable bound metrics present and <= registered maximum |
| `evidence` | `#Evidence` list sorted unique by id; all phases preflight (including seeded scripted actions, never agent actions), all session/pin/revision/digest bindings match parent, all observed times within started_at..finished_at |

The environment harness_pin equals profile.pins.harness_pin. Every evidence reference resolves exactly once in that receipt. W1 answered probes
reference `call_result`, W2 references `startup_delivery`, hook triples reference
`hook_trigger`, `hook_event`, `beat` respectively with a shared call id and
nondecreasing times. Samples reference `resource_sample` (age samples may reference
`revision`); absence entries reference `channel_inventory`. Applicable rungs cannot
be NOT-APPLICABLE. Inapplicable rungs must be NOT-APPLICABLE with reason and cited
inventory evidence; this does not excuse absent baseline checks. For PASS, W0 cites
revision evidence, W1 negotiation and answered probe evidence, W2 startup evidence,
W3 all hook triple evidence, W6 its sample evidence. FAIL and UNKNOWN are valid
records, never admission success. Schema validity alone is not an admission verdict.

Baseline: treatment surface off implies W0/W1/W2/W3 inapplicable; product identity
fields and primer digest are absent, negotiated_tools/answered_probe_ids and hook
observations are empty. W6 remains applicable with its baseline budget. The actual
harness pin, permitted tools, startup instructions and action capture require
baseline_checks evidence as well as remaining bound
by environment digest. All off channels, including treatment hooks/prime, require
cited inventory absence; enabled treatment hooks are forbidden in this baseline.
Missing inventory cannot be represented as proved absence. Unsupported hook sets
are declared disabled with evidence, never silently skipped. No daemon wire stamp
or RT-M15 field is required by this shape. WP-D1/D2, WP-R3, ADR-105 decision 9.

### Delivery outcome: never an admission input

`#Outcome` fields: `session_id`, `cell_id`, `arm: #Text`; `phase: "agent"`;
`pins: #Pins`; `profile_digest`, `environment_digest: #SHA`;
`agent_start_at: #Time`; `delivery_parameters_digest: #SHA`; `k: int & >0`; `k_frozen_at: #Time` strictly before
agent_start_at; `capabilities: [...#CapabilityOutcome]` sorted unique by capability;
`evidence: [...#Evidence]` sorted unique by id, phase agent and observed_at >=
agent_start_at with matching pins/session/profile. No preflight verdict is copied
into this object. `k=3` is the starting fixture value from WP-D1/LAB-11, no schema
default and no post-collection tuning. The k and per-capability bars are the
companion delivery parameters of the wiring profile, outside the admission shape.
Their canonical digest binds `{k, bars}` where bars is a capability-sorted list of
`{capability, use_minimum, acted_minimum}` objects. It equals the profile
source_digests entry `delivery_parameters` when joined in #Conformance. WPF-9
recomputes that digest before collection; CUE checks the binding only. Thus the
effective profile digest commits to delivery parameters without exposing W4/W5
to admission. k counts distinct subsequent agent calls,
excluding the delivery call and all scripted probes.

`#CapabilityOutcome` fields: `capability: #Text`, `eligible: int & >=0`,
`successful_calls: int & >=0`, `acted: int & >=0`, `should_not_calls: int & >=0`,
`unknown: int & >=0`, `denominator: #Denominator`, `w4: #Rate`, `w5: #Rate`,
`bars: {use_minimum: number & >=0 & <=1, acted_minimum: number & >=0 & <=1}`.
Rates count opportunities with successful use and deliveries acted on, respectively;
raw call counts are not pooled rates. `#Denominator` fields: `classifier_digest`,
`validation_reading_digest`, `opportunity_set_digest`, `delivery_set_digest: #SHA`,
`eligible_opportunities`, `delivered_targets: int & >=0`, `evidence_ids` nonempty
sorted unique text list. `eligible` equals eligible_opportunities; these denominators
come from LAB-9/LAB-M1a and LAB-11/BEN-4, never inference from calls alone.

`#Rate` is the closed union `{state: "measured", numerator: int & >=0,
denominator: int & >0, value: number & >=0 & <=1}` (numerator <= denominator and
value * denominator == numerator), or `{state: "not_measured", denominator: 0,
reason: #Text}`. W4 denominator equals eligible_opportunities; W5 denominator equals
delivered_targets. `acted` equals W5 numerator or zero when unmeasured. Zero eligible
opportunities gives not_measured; positive opportunities with zero successful use
gives measured zero. Unknown count is separate from either denominator. Later
resource diagnostics are outside WPF-1, not an open attributes field. No task score,
void decision, cost rewrite or invented BEN ledger type belongs here. WP-D1/D6.

`#Conformance` fields: `preflight: #Preflight`, `outcome?: #Outcome`. When outcome
exists session/cell/arm, pins, agent_start_at and profile/environment digests must
match preflight; delivery_parameters_digest matches its named profile source digest. A receipt without outcome is concrete and valid; adding a valid
zero-use or zero-action outcome never changes its preflight representation.

## Tests and fixtures handed to implementation

All functions live in `internal/wiring/conformance_schema_test.go`; names are exact.
The implementing lane writes tests first, witnesses intended assertion RED, then
adds schema/fixtures. No RED is claimed by this authoring lane. Proof command:
`go test -race ./internal/wiring -run '^TestConformanceSchema'`.

1. `TestConformanceSchemaValid`: compile schema with valid.cue using cuecontext;
   check each named value against its definition with `Validate(cue.Concrete(true))`,
   then marshal JSON successfully. Fixtures: `wired_preflight`, `baseline_preflight`,
   `unknown_preflight`, `failed_preflight`, `wsl_preflight` against #Preflight;
   `zero_opportunities`, `zero_calls`, `zero_actions`, `acted_at_k` against #Outcome;
   `preflight_only`, `with_zero_calls`, `with_zero_actions` against #Conformance.
   Wired has all rungs PASS, baseline four N/A plus W6 PASS and proved absence;
   unknown/failed have applicable UNKNOWN/FAIL respectively, not masquerading as PASS.
   Assert empty lists encode `[]`, never null; no fixture asserts real installation.
2. `TestConformanceSchemaNegatives`: invalid.cue is entirely commented, LAB-1 style
   `// cases: [` data. Each case has unique `name`, `fixture`, `definition`, `path`,
   and either replacement `value` or `remove: true`. Independently deep-copy the
   validated positive, apply exactly one mutation (never unify a conflicting value
   into an already constrained positive), compile mutated JSON, unify with named
   definition, and require concrete validation failure. Diagnostic must name the
   mutated field (or its nearest constrained parent for a cross-field comparison).
   Fail on zero cases, missing fixture/path, duplicate name, uncommented line or
   absent expected case. Exact required cases and their isolated assertions follow.
3. `TestConformanceSchemaPhaseSeparation`: #Preflight accepts wired and baseline
   without any outcome; inject each of `w4`, `w5`, `k`, `bars`, `successful_calls`,
   `acted` at preflight root and under profile, and w4/w5 under rungs: each rejected
   as unknown field. Agent evidence in preflight and preflight evidence in outcome
   each rejected. Valid zero-call/zero-action envelopes retain byte-identical
   canonical preflight JSON. This proves structural separation, not WPF-4 scoring.
4. `TestConformanceSchemaOutcomeBoundaries`: W4 0/0 and W5 0/0 are not_measured;
   W4 0/2 and W5 0/2 are measured zero; 1/2 is 0.5; 2/2 is 1. Test inconsistent
   rates independently rejected. k=1 and k=3 accepted, k=0/-1 rejected. No schema
   claim to match targets or count calls: exact k-1/k/k+1 action joins, wrong targets
   and duplicate calls are WPF-3 tests, awaiting LAB/BEN events.

Negative case inventory (each semicolon-separated name is a separate subtest of
`TestConformanceSchemaNegatives`, not a collection of faults in one fixture):

| Names | Single mutation and exact refusal |
|---|---|
| `unknown_field`; `unknown_verdict`; `unknown_harness`; `unknown_environment`; `unknown_source`; `unknown_channel`; `unknown_metric` | Add `extra: true` to receipt; replace corresponding enum with `"other"`; each fails at that field. |
| `missing_pin`; `short_product_commit`; `branch_subject_commit`; `bad_digest`; `profile_name_as_digest` | Remove model_pin; use 12-hex product; subject `"main"`; uppercase SHA; profile digest `"profile-a"`; each fails, independently. |
| `w4_requirement`; `w5_requirement`; `missing_requirement`; `duplicate_requirement` | Replace one requirement rung with W4/W5; remove W6; duplicate W3 in W6 slot; reject closed five-rung mask. |
| `applicable_na`; `inapplicable_pass`; `na_without_reason`; `absence_without_inventory`; `missing_off_channel`; `enabled_absence`; `baseline_enabled_hook`; `missing_baseline_checks`; `wrong_baseline_capture` | Contradict applicability/verdict; remove reason; replace absence reference with existing non-inventory evidence id; drop an off-channel absence; cite enabled channel; enable treatment hook in baseline; remove baseline_checks; point action_capture_evidence_id to an existing non-action item; each rejected. |
| `ancestor_served`; `wrong_installed`; `missing_served`; `recent_wrong_index`; `wrong_primer`; `unnegotiated_tool`; `no_answered_probe`; `missing_hook`; `unknown_hook`; `wrong_beat_kind`; `unjoined_beat` | Change served/installed to another full commit; remove served on PASS; change index while retaining fresh age; change primer digest; change negotiated tool; empty probe ids; remove hook triple; replace hook_id; change beat evidence kind; change its call_id. Reject each claimed PASS independently. |
| `missing_sample`; `over_budget`; `wrong_footprint_metric` | Drop a required sample; exceed one maximum; replace required host footprint metric with resident metric; reject W6 PASS. UNKNOWN fixture with missing samples remains valid. |
| `post_agent_decision`; `decision_at_start`; `reverse_window`; `late_freeze`; `changed_environment`; `changed_profile`; `changed_workload`; `changed_budget` | Set decision after/equal startup; finish before start; freeze after start; change each binding digest separately; reject at ordered/binding field. |
| `late_evidence`; `wrong_session`; `wrong_evidence_product`; `wrong_evidence_subject`; `wrong_evidence_profile`; `wrong_evidence_harness`; `empty_range`; `dangling_reference`; `duplicate_evidence`; `unsorted_evidence` | Change one observation outside window, binding, range end==start or reference; duplicate/reverse evidence entries; reject independently. |
| `agent_in_preflight`; `probe_in_outcome`; `null_list`; `outcome_binding`; `delivery_parameters_binding`; `environment_pin_binding` | Replace phase of one evidence item; reverse phase in outcome; replace empty list with null; change envelope outcome cell_id; change outcome delivery_parameters_digest; change environment harness_pin; reject each. |
| `zero_measured_denominator`; `negative_count`; `numerator_exceeds_denominator`; `wrong_rate`; `wrong_denominator`; `missing_denominator_provenance`; `zero_k`; `late_k` | Set measured denominator to 0; count -1; numerator > denominator; change only rate value; change denominator provenance count; remove validation_reading_digest; k=0; k_frozen_at=start; reject each. |

No multi-fault fixture may stand in for any named case. Additional independent
mutations may strengthen coverage; weaker assertions are FIX under W3-2. The
schema fixture harness may use private helpers only, and must validate the
unmutated source immediately before each negative to exclude unrelated incompleteness.

## Dependencies and limits of this contract

LAB-1 is existing vocabulary, not a newly invented result container. LAB-7/9/11/12,
LAB-M1a and BEN-4 still own event decoding, classifier validation, deduplication,
target matching, denominator computation and ledger emission. WPF-1 constrains
reported counts and provenance, not their truth against transcripts. WPF-2/4 own
actual checks and whole-pair admission; they must validate both preflights before
either arm starts. WPF-1's agent_start_at is the registered boundary; downstream
must compare it to the actual first action, not trust a forged timestamp.

No frozen BEN-1 registration seam, BEN-2 RunReadiness signature, WR-1/2 admission
API, or WR-4/5/12 stamp/firing output contract was supplied here. Their exact
interfaces remain downstream blockers for WPF-2/4/5/6/9/10 and their dependent rows;
this document invents none and does not claim those rows proven. WPF-9 owns hash
recomputation; WPF-3 owns actual action windows; WPF-M1 owns witnessed environments.
Independent criticism and any approval receipt remain the controller's work.

## Decision models

None: these are deterministic schema constraints with provider `none`; a judgment
cannot fill missing evidence or alter admission. No new calibration site or paid
call is introduced (WP-D6 and design section 5, I62).
