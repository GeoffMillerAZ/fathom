# WPF-1 — phase-separated wiring conformance schema

Status: Draft awaiting independent critic; T4-1 applicability amendment applied.
Not an implementation, approval receipt or implementation dispatch.
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
to an existing schema or published API. T4-1 settles disabled-channel applicability;
the integrating helper must amend CTR-13 as the shared-contract dependency below.
This lane changes no frozen interface.
Free option: separate records preserve
later adapters without building them. No intent record needs changing.

| Decision | Settled choice and governing text |
|---|---|
| D1 Ownership | Four new implementation files below, exactly as WPF-1 names; no second package or validator API. |
| D2 Admission boundary | `#Preflight` contains only W0/W1/W2/W3/W6; `#Outcome` owns W4/W5 and delivery bars. `#Conformance` is a reporting envelope, never a gate argument. WP-D1/D2, WPF-1. |
| D3 Closed wire shapes | Definitions and nested structs are closed, explicit lower_snake_case keys, finite vocabularies, no extension map. W3-4; LAB-1's definitions and required-field conventions in `schema/labs/schema.cue`. No Go exported types are needed. |
| D4 Evidence and identity | Explicit phase, source kind, range, clock, pins, revisions, digests and expected service; registration, negotiation and answered call evidence are independently required for W1, startup service for W2. WP-D1 W1/W2 table rows, WP-D3/D6. |
| D5 Validation mechanism | Existing CUE Go API, independent mutations of validated positives, field-specific diagnostics; `schema/labs/schema_test.go` and WPF-1. No shell CUE process in Go tests, no new dependency. |
| D6 Defaults and absence | T4-1 allows N/A for a channel the profile declares off only with frozen preflight evidence that the channel is absent. Active checks are unchanged; surface-off alone exempts no other channel. No implicit PASS, missing samples stay UNKNOWN, daemon-free arms have no invented stamp. WP-D1 table and baseline paragraph, WP-D2 registration mask, WP-D3 channels, WP-R3 and T4-1; ADR-105 decision 9 exempts stamps/RT-M15 only. |
| D7 Windows and cloud | Actual harness plus environment, explicit WSL witness fields; fixtures never certify installation. WP-D5/D6, WPF-7/8. |
| D8 Routine encodings | Exact definitions below use LAB-1 SHA/text/arm vocabulary, W3-4 closed JSON, and WP-D3's canonical effective data. Encoding choices impose no extra eligibility policy. |
| D9 Honest outcomes | Preserve known counts independently of unavailable capture or unvalidated classifiers; no invented numerator or provenance. P3, WP-D1 rate paragraph, LAB-M1a counts-only rule; `internal/labs/firing.go` `FiringReadingRow` and `stages` preserve margins while stages may be not measured. |
| D10 Ratio encoding | Measured rates serialize integer numerator/denominator only; display is derived. WP-D1 requires per-capability rates and denominators, not exact decimal equality; D3/D8 settle encoding. |

W2-3 approval still requires an independent LAND verdict and citations for every
decision; W3-2 requires implementing tests to match every assertion below. Exact
field names and conservative mechanics are settled here under D3/D5/D8, not
escalated for lack of literal schema prose. F2 count preservation, F3 integer ratios
and F4 registration/service bindings remain unchanged. F5/F6 below preserve governed
evidence constraints; T4-1 settles F1. A missing downstream interface is a
dependency, not permission to invent it.

### F1: governed disabled-channel applicability (T4-1)

T4-1 permits NOT-APPLICABLE for a channel the profile declares off only with
frozen preflight evidence that the channel is absent. Active checks are unchanged.
This settles product-present hook-only W1 N/A and disabled prime/hooks W2/W3 N/A
under the frozen per-rung mask below. Surface-off alone changes no other rung.
Never translate N/A to PASS. Missing absence evidence cannot substantiate N/A.
W0 product identity, W6 bounds, active-channel checks and baseline checks retain
their existing requirements; scoring, whole-pair preflight, assigned-arm accounting
and outcome separation are unchanged (WP-D1/D2/D3, WP-R3, T4-1).

Shared-contract dependency: the integrating helper must amend CTR-13, section
“One preflight admission contract shared with CTR-12 (U1)”, paragraph beginning
“Each applicable rung”, to apply T4-1's disabled-channel N/A rule. T2-1 identifies
[CTR-13](ctr-13.md) and [bench consolidation](bench-consolidation.md) at
`83226aece` as the approved frozen BEN contracts; T4-1 governs this narrow
applicability amendment. This lane proceeds on that ruling and does not edit the
shared contract. The dependency belongs to the integrating helper.

The applicability paragraphs, hook_only_preflight positive and related surface-off
assertions below now follow T4-1. This draft still awaits an independent critic's
LAND verdict under W2-3; it is neither self-approved nor an implementation dispatch.
No policy question remains for F1, and no new bar or fixture cohort is introduced.

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
  "channel_inventory" | "eligibility" | "action" | "action_capture"`.
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
| `#HookSpec` | `id: #Text`, `enabled: bool`, `trigger_digest: #SHA`, `event: #Text`, `timeout_ns: int & >0`. Inventory includes disabled and unsupported declared hooks, not only observed successes; retain their trigger/event/timeout descriptors and channel_inventory evidence of disabled status. |
| `#Requirement` | `rung: #EnvRung`, `applicable: bool`, `reason: #Text`. Exactly five entries, ordered W0,W1,W2,W3,W6. All applicable rungs are required; no separate skip flag. |
| `#Bound` | `metric: #Metric`, `maximum: number & >=0`. Sorted unique by metric; registration chooses bounds before data. |
| `#Metric` | `"index_age_ns" | "payload_age_ns" | "hook_p95_ns" | "phys_footprint_bytes" | "resident_bytes" | "cpu_seconds_per_ingest"`. Units are in names, never interchangeable heap measurements. |
| `#Budget` | `tenant_policy_digest: #SHA`, `bounds: [...#Bound]` nonempty. Must contain index/payload ages and hook p95 when their channels are enabled; daemon-free profiles require the host resident/footprint bound but do not acquire daemon CPU/age requirements. Apple hosts use footprint, Linux/WSL use resident; exact limits are preregistered inputs, not invented global bars. FT-D4 and WP-D1 W6. |
| `#Profile` | `id: #Text`, `expected_service: #Text`, `product_present: bool`, `lab: #LabID`, `bench_harness: #BenchHarness`, `pins: #Pins`, `environment: #Environment`, `channels: [...#ChannelSpec]` exactly all eight in lexical channel order, `hooks: [...#HookSpec]` sorted by id, `surface_tools: [...#Text]` sorted unique, `requirements: [...#Requirement]`, `workload_digest: #SHA`, `budget: #Budget`, `budget_digest: #SHA`, `source_digests: [...#SourceDigest]` nonempty sorted by id (must include `delivery_parameters`), `profile_digest: #SHA`, `environment_digest: #SHA`, `frozen_at: #Time`. No delivery bars or k. |
| `#SourceDigest` | `id: #Text`, `digest: #SHA`; references existing labs/knob, skill, primer, map and readout source contents; no path-derived substitute digest. |
| `#Evidence` | `id: #Text`, `service?: #Text`, `source_kind: #SourceKind`, `phase: #Phase`, `session_id: #Text`, `call_id: #Text`, `range: {artifact_digest: #SHA, start: int & >=0, end: int & >start}`, `observed_at: #Time`, `product_commit: #Commit`, `subject_commit: #Commit`, `profile_digest: #SHA`, `harness_pin: #Text`, `reason: #Text`, `capture?: #CaptureSummary`. capture is required exactly when source_kind is action_capture and forbidden otherwise; action_capture evidence is agent-phase only. Range is half-open byte offsets into a local source artifact identified by hash. `call_id` is the correlated scripted/agent operation id even for non-tool events; it does not imply an agent call. |
| `#RungResult` | `verdict: #Verdict`, `reason: #Text`, `evidence_ids: [...#Text]` sorted unique; nonempty except UNKNOWN, which may have no observation. |
| `#Absence` | `channel: #Channel`, `evidence_ids: [...#Text]` nonempty sorted unique. No bare boolean constitutes absence evidence. |
| `#Registration` | `service: #Text`, `surface_tools: [...#Text]` sorted unique, `evidence_ids: [...#Text]` nonempty sorted unique; every reference has source_kind registration. |
| `#HookObservation` | `hook_id: #Text`, `trigger_evidence_id: #Text`, `event_evidence_id: #Text`, `beat_evidence_id: #Text`. |
| `#Sample` | `metric: #Metric`, `value: number & >=0`, `evidence_ids: [...#Text]` nonempty sorted unique. One entry per observed bound metric, sorted by metric; no sample means unknown, not zero. |

Hook inventory consistency (F6, WP-D1 W3, WP-D3, WP-R3): the
hooks channel.enabled equals whether any #HookSpec has enabled: true, in both
directions, regardless of verdict or product presence. A disabled hooks channel
retains all disabled and unsupported declared HookSpecs, never silently drops them.
No hooks absence proof can coexist with an enabled treatment hook. Unsupported
hooks remain disabled with channel_inventory evidence; missing evidence is not
absence. T4-1 governs the disabled-channel applicability rule below.

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
| `served_product_commit?`, `installed_product_commit?`, `index_commit?` | `#Commit`; for W0 PASS both product fields required and equal `profile.pins.product_commit`; for W6 PASS with product present, index commit required and equal subject commit; product-absent baseline omits index_commit |
| `registration?` | `#Registration`; required for W1 PASS, its service equals profile.expected_service and its surface_tools equal profile.surface_tools; forbidden when surface disabled |
| `primer_digest?` | `#SHA`; W2 PASS requires equality to prime channel content digest |
| `negotiated_tools`, `answered_probe_ids` | sorted unique text lists; W1 PASS requires negotiated tools equal profile surface tools and >=1 answered probe reference |
| `hook_observations` | sorted unique `#HookObservation` list by hook_id; W3 PASS covers exactly all enabled declared hooks, no extra ids |
| `baseline_checks?` | Closed `{harness_evidence_id: #Text, tools_evidence_id: #Text, startup_evidence_id: #Text, action_capture_evidence_id: #Text}`; required only when profile.product_present is false, forbidden otherwise. References resolve to revision, channel_inventory, startup_delivery and scripted action evidence respectively. |
| `absences` | `#Absence` list sorted unique by channel; every disabled channel has exactly one absence entry, no enabled channel may appear |
| `samples` | `#Sample` list; W6 PASS requires all applicable bound metrics present and <= registered maximum |
| `evidence` | `#Evidence` list sorted unique by id; all phases preflight (including seeded scripted actions, never agent actions), all session/pin/revision/digest bindings match parent, all observed times within started_at..finished_at |

The environment harness_pin equals profile.pins.harness_pin. Every evidence reference resolves exactly once in that receipt. W1 answered probes
reference `call_result`. For W1 PASS, registration evidence, negotiation evidence and each answered probe have service == profile.expected_service; registration inventory equals both negotiated_tools and profile.surface_tools. W2 PASS references `startup_delivery` with service == profile.expected_service and requires the matching primer_digest. All other supplied evidence service fields also equal profile.expected_service. Hook triples reference
`hook_trigger`, `hook_event`, `beat` respectively with a shared call id and
nondecreasing times. Samples reference `resource_sample` (age samples may reference
`revision`); absence entries reference `channel_inventory`. Applicable rungs cannot
be NOT-APPLICABLE. Inapplicable rungs must be NOT-APPLICABLE with reason and cited
inventory evidence; this does not excuse absent baseline checks. For PASS, W0 cites
revision evidence, W1 registration, negotiation and answered probe evidence (including every registration.evidence_ids and answered_probe_ids reference), W2 startup evidence,
W3 all hook triple evidence, W6 its sample evidence. FAIL and UNKNOWN are valid
records, never admission success. Schema validity alone is not an admission verdict.

**Governed applicability — T4-1.** A channel declared off may use N/A only with
frozen preflight evidence of its absence. Active checks are unchanged.
Applicability is frozen per rung and channel, never inferred from surface-off.
W0 is applicable iff product_present; W1 iff the surface channel is enabled;
W2 iff prime is enabled; W3 iff at least one declared hook is enabled; W6 is
always applicable. Disabled surface has empty surface_tools, negotiated_tools and
answered_probe_ids and no registration; it changes neither product_present nor
the prime/hooks channels. Disabled prime omits primer_digest. With no enabled
hooks, hook_observations is empty. Enabled surface, prime or hooks require
product_present. W0 PASS needs installed/served identities even for hook-only
profiles. W2/W3 PASS still require delivery/hook evidence when surface is off.

Baseline without fathom is explicitly product_present: false, not merely surface
off. Only this case has W0/W1/W2/W3 N/A, absent product identity fields and primer,
no enabled treatment surface/prime/hooks and empty probe/hook observations. W6
remains applicable with the baseline budget. baseline_checks proves the actual
harness pin, permitted tools, startup instructions and action capture, also bound
by environment_digest. Other declared channels retain their own enabled/absence
evidence; this flag does not fabricate their contents. Every off channel needs
inventory absence evidence. Missing inventory never proves absence. Unsupported
hooks are declared disabled with evidence, never silently skipped. The hook-only
positive has product_present: true, surface off, prime/hooks enabled, W1 N/A and
W0/W2/W3/W6 PASS. No surface-off shortcut is permitted. Authority: WP-D1 table
W0-W3 and its exact paragraph beginning “A baseline without fathom”, WP-D2's
applicability mask, WP-D3's separate channels, WP-R3 and T4-1; ADR-105 decision 9 removes
only daemon stamps/RT-M15 for daemon-free readings, not other applicable checks.

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

`#Count` is the closed union `{state: "known", count: int & >=0}` or
`{state: "unavailable", reason: #Text}`. Unavailable has no count key; known zero
and unknown are distinct. Existing LAB raw integer margins map to known counts,
never inferred rate numerators. No existing LAB API is changed by this wire shape.

`#Provenance` fields: `state: "validated" | "unvalidated" | "unavailable"`,
`reason: #Text`, `set_digest?: #SHA`, `classifier_digest?: #SHA`,
`validation_reading_digest?: #SHA`, `evidence_ids: [...#Text]` sorted unique.
Validated requires set_digest and nonempty evidence_ids. Eligibility provenance
additionally requires classifier_digest and validation_reading_digest when
validated. Unavailable/unvalidated permits absent digests and empty evidence_ids;
retain any actual available provenance without manufacturing a digest. For
observations, validated means usable captured events, not classifier validation.
Every provenance evidence_id resolves exactly once in the outcome evidence list;
eligibility references eligibility events, delivery references call_result events,
action_capture references capture-completeness evidence, never requires an action event.
For action_capture only, validated requires exactly one evidence_id of source_kind
`action_capture`, whose capture.capability equals the enclosing capability and
whose summary has complete: true. Its set_digest binds the captured artifact/set,
including an empty action set. Unavailable capture does not manufacture
evidence_ids or set_digest; retain any real incomplete capture evidence and digests
without promoting validation. Unvalidated capture may retain real evidence/digests but
cannot produce measured W5. CUE binds declared status/provenance only; downstream
owners prove validation and completeness against captured bytes.

`#CaptureSummary` is a closed object with exactly `capability: #Text`,
`complete: bool`, `action_evidence_ids: [...#Text]` sorted unique, present even when
empty. It summarizes coverage of this capability's delivery/action windows for the
outcome's frozen k, not a tool action; its evidence range points to the capture
manifest, even when no action occurred. Every action_evidence_id resolves exactly
once to agent-phase source_kind action in the same outcome, with the existing
session/pin/profile bindings. The summary lists the observed action events, not
only successful target matches; acted remains the matched-target count, never the
length of this list. A complete empty capture has action_evidence_ids: [] and can
support known acted zero without inventing an action. It cannot support positive
acted: when the validated summary's action list is empty, acted must be known zero.
A nonempty list does not by itself prove any match. WPF-3 owns actual window
coverage, decoding and target joins. This single summary shape serves the present
zero-action case; it adds no generic capture registry or alternate decoder.

`#Denominator` fields: `eligible_opportunities: #Count`, `delivered_targets: #Count`,
`eligibility: #Provenance`, `delivery: #Provenance`.
`#CapabilityOutcome` fields: `capability: #Text`, `eligible: #Count`,
`successful_calls: #Count`, `acted: #Count`, `should_not_calls: #Count`,
`unknown: #Count`, `denominator: #Denominator`, `action_capture: #Provenance`,
`w4: #Rate`, `w5: #Rate`,
`bars: {use_minimum: number & >=0 & <=1, acted_minimum: number & >=0 & <=1}`.
Eligible equals eligible_opportunities, including state/count or unavailable reason.
Keep every independently known margin even when a rate is unavailable or
unvalidated. Unknown residue is separate; it cannot replace known denominators.

`#Rate` is the closed union:

- `{state: "measured", numerator: int & >=0, denominator: int & >0}` with
  numerator <= denominator. Integers alone encode the exact ratio; no value key.
- `{state: "not_measured", denominator: #Count,
  cause: "zero_denominator" | "unavailable" | "unvalidated", reason: #Text}`.
  No numerator or value key, including when denominator.count is positive.

W4 denominator binds eligible_opportunities, W5 binds delivered_targets (the
measured integer equals the known count; the not_measured #Count equals the
corresponding count object). Measured W4 requires validated eligibility and known
successful_calls; its numerator counts eligible opportunities with successful
use, not raw successful_calls, and cannot exceed that known call count. Measured
W5 requires validated delivery and action_capture and known acted; its numerator
equals acted.count. A zero known denominator always uses zero_denominator and
not_measured; unknown denominator uses unavailable. A positive denominator with
missing required capture, count or provenance uses unavailable; with present but unvalidated
required evidence uses unvalidated. If both apply, unavailable takes precedence.
Only positive denominators with all required evidence validated and required counts known use measured;
zero successful use/action in this case is measured zero. If action capture is
unavailable, acted is unavailable, never fabricated zero. Independently known
acted counts can remain known when delivery provenance is unvalidated.

Eligibility is validated only after LAB-M1a passes for that surface.
A failed/missing LAB-M1a classifier validation leaves W4 counts only, even with
positive eligible counts. It does not erase independently observed delivery/action
counts or by itself invalidate W5. `FiringReadingRow` exposes raw margins while
`stages` in `internal/labs/firing.go:151` can return named not-measured strings;
its unimplemented acted channel is not a zero or synthetic action proof. Adapters
preserve those distinctions; this contract neither invents a validation API nor
claims existing replay implements action capture. Authority: P3, LAB-M1a,
WP-D1 rate paragraph and WP-D6's configured/observed/eligible/acted/unknown split.

Display is derived on reading a measured integer ratio; it is not serialized as a
redundant exact floating value. For fixture display assertions use Go's
strconv.FormatFloat(float64(numerator)/float64(denominator), 'g', -1, 64): 1/3
renders 0.3333333333333333, 2/3 renders 0.6666666666666666. Never validate a
rounded display by multiplying it back. The critic's decimal-equality failure is
addressed by removing that equality, with no new dependency or reproduction run.
Later resource diagnostics are outside WPF-1, not an open attributes field. No
task score, void decision, cost rewrite or invented BEN ledger type belongs here.

`#Conformance` fields: `preflight: #Preflight`, `outcome?: #Outcome`. When outcome
exists session/cell/arm, pins, agent_start_at and profile/environment digests must
match preflight; delivery_parameters_digest matches its named profile source digest. A receipt without outcome is concrete and valid; adding a valid
zero-use or zero-action outcome never changes its preflight representation.

## Tests and fixtures handed to implementation

All functions live in `internal/wiring/conformance_schema_test.go`; names are exact.
The implementing lane writes tests first, witnesses intended assertion RED, then
adds schema/fixtures. Documentary RED/GREEN checks in this authoring lane concern
this document only; no runtime RED, schema validation or package/full-gate proof
is claimed. Independent criticism and approval remain pending under W2-3.
Future implementation proof command:
`go test -race ./internal/wiring -run '^TestConformanceSchema'`.

1. `TestConformanceSchemaValid`: compile schema with valid.cue using cuecontext;
   check each named value against its definition with `Validate(cue.Concrete(true))`,
   then marshal JSON successfully. Fixtures: `wired_preflight`, `baseline_preflight`,
   `hook_only_preflight`, `unknown_preflight`, `failed_preflight`, `wsl_preflight` against #Preflight;
   `zero_opportunities`, `zero_calls`, `zero_actions`, `acted_at_k`,
   `two_deliveries_no_capture`, `two_deliveries_complete_no_actions`, `classifier_counts_only`,
   `classifier_failed_validation`, `unknown_denominator`,
   `one_third`, `two_thirds` against #Outcome;
   `preflight_only`, `with_zero_calls`, `with_zero_actions` against #Conformance.
   hook_only_preflight follows T4-1 with frozen surface-absence evidence; retain
   its exact assertions and dependent negatives. Wired has all rungs PASS; baseline has explicit product absence, four N/A plus W6 PASS and proved absence; hook-only has W1 N/A and W0/W2/W3/W6 PASS with actual product, primer and hook bindings;
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
   canonical preflight JSON, including with two_deliveries_complete_no_actions.
   Capture summaries are rejected at preflight root and under profile; action_capture
   source evidence is rejected in preflight. This proves structural separation, not WPF-4 scoring.
4. `TestConformanceSchemaOutcomeBoundaries`: W4 0/0 and W5 0/0 are not_measured
   with zero_denominator; validated W4 0/2 and W5 0/2 are measured integer ratios.
   1/2 and 2/2 derive displays 0.5 and 1. For one_third and two_thirds independently,
   validate, marshal JSON, decode and revalidate: integer numerator 1 or 2 and
   denominator 3 survive exactly, no value key appears, and derived displays equal
   the strings above. Reject a redundant value key even if numerically plausible.
   two_deliveries_complete_no_actions retains delivered_targets known 2, validated
   delivery and capture, one action_capture evidence item with complete true, no
   source_kind action evidence items, action_evidence_ids is [], acted known 0,
   and W5 measured numerator 0 and denominator 2. Validate, marshal, decode and
   revalidate; assert every listed field and empty action list, not just success.
   two_deliveries_no_capture retains delivered_targets known 2, W5 not_measured
   unavailable with known denominator 2, acted unavailable, empty capture evidence
   and no invented digests/numerator. Assert UNKNOWN capture remains unavailable,
   not known zero or a measured rate; neither complete-zero nor missing capture
   changes preflight or admission. classifier_counts_only retains known eligible
   2 and successful_calls 1 with unvalidated eligibility, W4 not_measured unvalidated,
   no numerator or validation digest. classifier_failed_validation retains the same
   counts and actual classifier/failed-reading digests with eligibility unvalidated
   and W4 not_measured unvalidated; no numerator or rate value is fabricated.
   Independently validated W5 may stay measured in both fixtures.
   Unknown denominator remains unavailable without a fabricated zero. Assert these
   fields after JSON round trip, not just validation success. Reject promotions
   to measured without each required provenance/status, and invented acted zero
   under unavailable capture. k=1 and k=3 accepted, k=0 and k=-1 rejected. No schema claim to
   match targets or count calls: k-1/k/k+1 joins, wrong targets and duplicate calls
   remain WPF-3 tests awaiting phase-labelled LAB/BEN event integration.

Negative case inventory (each semicolon-separated name is a separate subtest of
`TestConformanceSchemaNegatives`, not a collection of faults in one fixture):

| Names | Single mutation and exact refusal |
|---|---|
| `unknown_field`; `unknown_verdict`; `unknown_harness`; `unknown_environment`; `unknown_source`; `unknown_channel`; `unknown_metric` | Add `extra: true` to receipt; replace corresponding enum with `"other"`; each fails at that field. |
| `missing_pin`; `short_product_commit`; `branch_subject_commit`; `bad_digest`; `profile_name_as_digest` | Remove model_pin; use 12-hex product; subject `"main"`; uppercase SHA; profile digest `"profile-a"`; each fails, independently. |
| `w4_requirement`; `w5_requirement`; `missing_requirement`; `duplicate_requirement` | Replace one requirement rung with W4/W5; remove W6; duplicate W3 in W6 slot; reject closed five-rung mask. |
| `applicable_na`; `inapplicable_pass`; `na_without_reason`; `absence_without_inventory`; `missing_off_channel`; `enabled_absence`; `baseline_product_present_mismatch`; `baseline_enabled_hook`; `surface_off_w0_na`; `surface_off_w2_na`; `surface_off_w3_na`; `missing_baseline_checks`; `wrong_baseline_capture` | Contradict applicability/verdict; remove reason; replace absence reference with existing non-inventory evidence id; drop an off-channel absence; cite enabled channel; make baseline W0 applicable while product_present remains false; enable one declared baseline hook while product_present remains false; in validated hook-only fixture change W0, W2 or W3 requirement applicable to false separately (reject the inconsistent requirement/rung); remove baseline_checks; point action_capture_evidence_id to an existing non-action item; each rejected. |
| `hooks_channel_contradiction`; `hooks_channel_enabled_without_hook` | From wired_preflight with enabled hooks and W3 PASS, change only hooks channel.enabled to false, retaining HookSpecs, observations, verdicts and evidence; reject at channels/hooks cross-field rule. Independently start with baseline_preflight's disabled hooks inventory and complete absence proof, change only hooks channel.enabled to true; reject, without adding hooks or deleting absence evidence. No multi-field repair substitutes for either test. |
| `ancestor_served`; `wrong_installed`; `missing_served`; `recent_wrong_index`; `wrong_primer`; `unnegotiated_tool`; `no_answered_probe`; `missing_hook`; `unknown_hook`; `wrong_beat_kind`; `unjoined_beat` | Change served/installed to another full commit; remove served on PASS; change index while retaining fresh age; change primer digest; change negotiated tool; empty probe ids; remove hook triple; replace hook_id; change beat evidence kind; change its call_id. Reject each claimed PASS independently. |
| `missing_registration`; `missing_registration_evidence`; `wrong_registration_kind`; `wrong_registration_service`; `wrong_registration_tools`; `missing_registration_rung_citation`; `missing_startup_service`; `wrong_startup_service` | On wired PASS remove registration; empty only registration.evidence_ids; point its evidence_ids to existing negotiation evidence; change registration.service; change one registration.surface_tools entry; remove only its registration evidence id from W1 evidence_ids; remove startup evidence service; change only startup evidence service to another service while subject/profile/primer remain correct. Each independently rejects W1 or W2 at its changed field or constrained parent. |
| `missing_sample`; `over_budget`; `wrong_footprint_metric` | Drop a required sample; exceed one maximum; replace required host footprint metric with resident metric; reject W6 PASS. UNKNOWN fixture with missing samples remains valid. |
| `post_agent_decision`; `decision_at_start`; `reverse_window`; `late_freeze`; `changed_environment`; `changed_profile`; `changed_workload`; `changed_budget` | Set decision after/equal startup; finish before start; freeze after start; change each binding digest separately; reject at ordered/binding field. |
| `late_evidence`; `wrong_session`; `wrong_evidence_product`; `wrong_evidence_subject`; `wrong_evidence_profile`; `wrong_evidence_harness`; `empty_range`; `dangling_reference`; `duplicate_evidence`; `unsorted_evidence` | Change one observation outside window, binding, range end==start or reference; duplicate/reverse evidence entries; reject independently. |
| `agent_in_preflight`; `probe_in_outcome`; `null_list`; `outcome_binding`; `delivery_parameters_binding`; `environment_pin_binding` | Replace phase of one evidence item; reverse phase in outcome; replace empty list with null; change envelope outcome cell_id; change outcome delivery_parameters_digest; change environment harness_pin; reject each. |
| `zero_measured_denominator`; `negative_count`; `numerator_exceeds_denominator`; `redundant_rate_value`; `wrong_denominator`; `missing_denominator_provenance`; `zero_k`; `late_k` | Set measured denominator to 0; known count -1; numerator > denominator; add value to measured rate; change only rate denominator against known provenance count; remove validation_reading_digest from validated measured W4 eligibility; k=0; k_frozen_at=start; reject each. |
| `missing_expected_service`; `missing_registration_evidence_service`; `wrong_registration_evidence_service`; `missing_negotiation_service`; `wrong_negotiation_service`; `missing_probe_service`; `wrong_probe_service` | Remove profile.expected_service; remove or change only service on registration, negotiation or answered-probe evidence respectively; reject each W1 PASS independently. |
| `missing_measured_numerator`; `unavailable_numerator`; `unvalidated_numerator`; `unavailable_measured`; `unvalidated_measured`; `fabricated_acted_zero`; `unknown_as_zero`; `positive_zero_denominator_cause`; `wrong_w5_numerator`; `unsupported_w4_numerator` | Remove measured numerator; add numerator to unavailable or unvalidated rate separately; replace action_capture.state on a measured positive W5 with unavailable; replace eligibility.state on measured positive W4 with unvalidated (keep real digests); replace unavailable acted with known zero under unavailable capture; add count:0 to unavailable denominator; change cause to zero_denominator on a positive unavailable denominator; change measured W5 numerator within bounds but unequal acted.count; raise W4 numerator within denominator but above known successful_calls.count. Each single mutation rejects at rate/count/provenance comparison. |
| `missing_capture_summary`; `wrong_capture_kind`; `incomplete_capture_measured`; `dangling_captured_action`; `empty_capture_positive_acted`; `capture_in_preflight`; `capture_under_profile`; `capture_evidence_in_preflight` | From two_deliveries_complete_no_actions, remove only capture on its action_capture evidence; change only that evidence source_kind to action; change only capture.complete to false; replace only capture.action_evidence_ids with one nonexistent id; change only acted.count to 1 while keeping the empty action list and W5 numerator zero. Each independently rejects its field or constrained parent. From wired_preflight, add that valid summary at root, then separately under profile, then separately replace one existing evidence item with the valid agent-phase action_capture item; each independently rejects. |

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

BEN interfaces are approved and frozen: T2-1 identifies origin/main `83226aece`,
[CTR-13](ctr-13.md) and [bench consolidation](bench-consolidation.md). CTR-13's
shared ledger boundary supplies PreflightReceipt/PairPreflight/AdmitPair; its
six-readiness-harness section supplies Control, ControlResult, Runner and
RunReadiness. Registration/consolidation follow those exact contracts and their
serial grants, not an invented runner or a claim that BEN lacks an interface.
Approval of those contracts is not proof of BEN implementation. T4-1 governs
disabled-channel N/A; the integrating helper must amend CTR-13 as specified in F1.
That shared amendment is a dependency, not a file owned by this lane.

LAB/WR frozen contracts remain pending under T2-2: LAB-7 and WR-1/2/4/5/12 exact
adapter/admission/stamp/firing interfaces still block their dependent WPF rows.
The shared transcript decoder already exists in `internal/transcript/events.go`:
`func DecodeTranscriptEvents(path, salt string, fileIndex int) Result` (package
`transcript`). It reads Claude Code events, counts and names failures, and uses the
caller-supplied bundle salt and deterministic file ordinal; `DecodeLine` in
`internal/transcript/decode.go` supplies line decoding. WPF extends LAB-7 rather
than introducing a second decoder (WP-D5). Existing decoding is not
proof of complete action capture or of the pending frozen interfaces. This contract
invents none and does not claim dependent rows proven. WPF-9 owns hash
recomputation; WPF-3 owns actual action windows; WPF-M1 owns witnessed environments.
Independent criticism and any approval receipt remain the controller's work.

## Decision models

None: these are deterministic schema constraints with provider `none`; a judgment
cannot fill missing evidence or alter admission. No new calibration site or paid
call is introduced (WP-D6 and design section 5, I62).
