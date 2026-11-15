# CTR-12: attribution, wire evidence, delivery and landing

Status: DRAFT W1-5/#263. Contract only, no implementation or measured RED.
Base: `468dc705813382d584926887b451cbea145c76ac`. Owns this document only.
Independent branch. No companion fixture drafts land here. Landing needs REC-1,
CTR-4, ENG-2, LIB-1; every consumer still needs its authored prerequisites.

## Intent before design

Read intent router, P2/P3/P12/P15/P18/P22, scope compatibility/non-goals, ADR-080,
ADR-081, ADR-098/100/101/102/110, parallel-lanes PL-D1..7 and §4 hooks row,
wiring-proof WP-D1..6, proof-program BN-D2, realignment ENG-8/9/10/WIR-1/CHG-5/6.
Newer ADRs are proposed design context, not proof their code exists. Keep I9/I12
hook fail-open behavior, I38 coverage, I62 judgment/proof separation. No scheduling,
provider/test execution in product, new table or inference that firing means help.
W2-1 answers LL-M1; no language-door package is added. No evidence a bundle/CI
mismatch, powered holdout or hook-latency trigger has fired. Stamp/receipt presence
is not a conformance reading. Preserve future adapters with explicit typed inputs,
not future harness fields. No backfilled historical stamps.

Settled decisions, with governing text:
- D1 one sequential hook owner: parallel-lanes §4 hooks row explicitly orders
  ENG-8 -> ENG-9 -> WIR-1 -> CHG-3 -> BEN-4 -> CHG-8 -> FLT-3. Alternative parallel
  hook edits contradict that row. The exact serial reservation is settled by W4-2.
- D2 reuse exact-tree receipts: ADR-100 d2 and PL-D7. Alternative rerun different
  tree and call it equivalent is excluded. Existing gatereceipt.Check is the law.
- D3 delivery is forward/holdout evidence, not helping: BEN-4, BN-D2/BN-K8/K9 and
  ADR-080. Alternative treating eligible/fired counts as outcome proof is excluded.
- D4 preflight before agent; W4/W5 outcomes never exclusions: ADR-110 WP-D1/2,
  R2-1/BX-40 in #263. Alternative excluding zero-call cells is forbidden.
- D5 retain verify codes and fail-open hooks: CU-K7, I9/I12. Alternative deriving
  runtime conformance from static verify is excluded.

API mechanics are settled by W3-4 and semantic ports cite D1-D5. W4-2 settles
the remaining approved-scope serial grants: one hook owner, active gate owner
hands back before capture integration. Full CI runs on EVERY bundle, no sample.
Advisory holdout is deterministic 26/256; enforcement/failure channels always
deliver. FLT-3/FLT-10 port consumers are split to a later exact contract below.
No generic shape/rate/grant question remains in this retained scope.

## Proposed attribution and lookup ports (W3-4)

```go
// internal/mcp/hook_lookup.go; uses existing backend/symbol types.
func hookLookup(ctx context.Context, backend QueryInterface,
    service, name string, limit int) ([]core.SymbolNode, error)
// cmd/fathom/hook_attribution.go
type RowAttribution struct { SessionID string `json:"session_id"`; LaneID string `json:"lane_id"`; Caller CallerKind `json:"caller"` }
func stampRowAttribution(row map[string]any, attribution RowAttribution) map[string]any
```

hookLookup invokes stored QuerySymbols using a name filter and top limit, the
same semantics as find_symbol, with limit 1..10 and caller deadline. Empty name,
invalid limit or missing service returns error, never a repository scan. MCP
find_symbol handler and hook caller share this function; query_symbols remains
an extended public verb where already registered but disappears from hook-path
requests. No new MCP registration or backend method. Hook liveness uses current
identity/status call, not a sacrificial full symbol query. Missing index/deadline
means named unknown and fail-open hook, never fabricated no-match.

stampRowAttribution returns a copied row, preserving all existing keys except
`session_id`, `lane_id`, `caller` set from explicit trusted caller context. Empty
session/lane remains empty and counted unattributed; no guessed id from cwd or
last session. Server transport receives these via existing request attribution
metadata, not model-supplied tool arguments. Serialize keys `session_id`, `lane_id`,
`caller`; caller is agent/edit-hook/gate/refresh/plumbing/unattributed. ENG-9
coverage is numerator rows with both applicable ids / all applicable rows, >=95%
as authored; report missing denominators and exemptions separately, never drop them.

## Wire stamp and doctor (W3-4)

```go
// internal/wiring/wire_stamp.go
type WireInputs struct {
    FathomVersion, ExtractorEpoch, OwnedHooksDigest, SurfaceDigest string
}
type WireStamp struct {
    SessionID string `json:"session_id"`
    Wire string `json:"wire"`
    Index IndexState `json:"idx"`
}
func ComputeWireStamp(in WireInputs) (string, error)
func WriteWireStamp(dst io.Writer, sessionID string,
    in WireInputs, index IndexState) (WireStamp, error)
// cmd/fathom/doctor_wire_stamp.go
type WiringFiring struct { State WiringState `json:"state"`; Reason string `json:"reason"`; SessionID string `json:"session_id"`; Wire string `json:"wire"` }
func wiringFiring(stamp wiring.WireStamp, observedWire string) WiringFiring
```

Compute sha256 of canonical JSON WireInputs in field declaration order. No local
paths/clock. All four inputs required. Effective owned hooks/surface canonicalized
before hashing, sorted keys/entries, include content not settings path. Index is
fresh/behind/unknown, independent from wire hash. WriteWireStamp writes one LF JSON
row to supplied writer, no hidden path/clock. SessionStart computes once and appends
wire/idx to existing sessionWindowRow via recordSessionWindow under its current
lock and statedir policy (WR-4); do not create a second sessions ledger. WIR-1
adapts this function, WR-4 remains the shared implementation owner. Doctor returns
unknown if missing; configured when stamp exists but no matching firing; observed
only when observed wire equals stamp. Never promote installed -> firing. Existing
WR-12 firing check and per-harness applicability remain, not a new ladder.

## Forward delivery schema and funnel (W3-4)

```go
// internal/labs/delivery.go
type DeliveryRow struct {
    SessionID string `json:"session_id"`
    LaneID string `json:"lane_id"`
    Capability string `json:"capability"`
    CallID string `json:"call_id"`
    Target string `json:"target"`
    At string `json:"at"`
    Phase CallPhase `json:"phase"`
    Arm DeliveryArm `json:"arm"`
    Eligible bool `json:"eligible"`
    Fired bool `json:"fired"`
    Delivered bool `json:"delivered"`
    LatencyMS float64 `json:"latency_ms"`
}
type SessionCall struct {
    SessionID string `json:"session_id"`
    CallID string `json:"call_id"`
    Phase CallPhase `json:"phase"`
    Ordinal int
    Targets []string
}
type CallCoverage struct {
    SessionID string
    FirstOrdinal, LastOrdinal int
    Complete bool
    Reason string
}
type FunnelInput struct {
    Rows []DeliveryRow
    Calls []SessionCall
    Coverage []CallCoverage
    Pairs []benchledger.PairPreflight
    DeadlineMS float64
    NextCalls int
}
type DeliveryResult struct {
    Rows []FiringReadingRow
    VoidPairs []string
    UnknownSessions []string
}
func DeliveryFunnel(in FunnelInput) (DeliveryResult, error)
func DeliveryHoldout(sessionID, capability string, channel DeliveryChannel) (bool, error)
```

After mandatory eligibility checks below, advisory-only DeliveryHoldout uses
sha256 of length-prefixed session/capability, first byte <26
(approximately 10.16%), stable within session. W4-2 approves this exact 26/256 fraction; it is not a measured policy win. Arm is assigned holdout/deliver before eligible outcome;
held-out rows keep eligible=true, fired/delivered=false. Phase preflight/agent
separates scripted probes from agent outcomes. Target is an exact existing
file/symbol/test anchor, never raw transcript. At RFC3339 UTC, supplied by caller.
Missing target/id/timestamp, duplicate CallID or negative latency fails parse and
is counted unread evidence, never a success. FiringReadingRow/Stages/Stage are
existing result types; no second funnel renderer. Acted requires an exact anchored action in the next k calls from ONE complete
session call population, including calls with no delivery decision. DeliveryRow.CallID
joins SessionCall.CallID; the action is the Targets of a later SessionCall with
ordinal in (delivery.Ordinal, delivery.Ordinal+k]. No separate action-only counter.
Session ordinals are exporter-recorded event order, positive and unique, not sorted
by timestamp or lexicographic id. Equal timestamps are safe only with distinct
recorded ordinals. Duplicate/missing ordinal, unknown event loss or noncontiguous
claimed coverage -> UnknownSessions, not zero action. Coverage must include the
whole window through delivery+k or a witnessed session end. Complete is an adapter
attestation verified against captured event counts/raw digest; merely authoring true
cannot establish it. Calls are immutable during aggregation.

Pair validity is not recomputed from use. FunnelInput.Pairs imports the exact
benchledger.PairPreflight contract from CTR-13: PairID, TreatmentSession,
ControlSession, BeforeAgent, Valid, Disposition, Reason. Required named
PreflightDisposition is admitted/void/unknown, not parsed from Reason. Valid is
derived Disposition==PreflightAdmitted; inconsistent boolean/enum inputs return
typed RefusalError. Missing/late receipt is PreflightUnknown, definite failed
scripted pre-agent W0-W3/W6 is PreflightVoid, all applicable pass before startup
is PreflightAdmitted. Both arms follow that same disposition. VoidPairs only from
PreflightVoid; unknown/missing/late pairs add both sessions to UnknownSessions and
are not admitted, never treated as scored or silently VOID from Valid=false.
A valid zero-use pair remains assigned-arm scored/non-use; no W4/W5 admission.
Benchledger owns this model; labs imports it, never the reverse. Preflight probes
are phase preflight, excluded from agent-use outcomes. Helped remains not measured
without independent oracle. Existing Stages result types/coverage remain.

Proposed TestDeliveryCallOrderCoverage cases: k=3 delivery at ordinal 2, non-delivery
calls 3/4/5 and target action 6 -> not acted; action 5 -> acted; equal timestamp
with distinct ordinals stable; missing call 4 -> unknown; duplicate ordinal ->
error; session ended at 4 with witnessed end closes short window. Proposed
TestDeliveryPairPreflight cases: valid reachable treatment with zero agent calls
remains scored/non-use, assigned arms unchanged; scripted failure before startup
voids both sessions; missing/late receipt maps both sessions unknown, not void; late receipt cannot
retrospectively admit; inconsistent disposition/bool typed refusal. Commands `go test -race ./internal/labs -run
'^TestDelivery(CallOrderCoverage|PairPreflight)$' -count=1` (one line), unrun.

`cmd/fathom/hook_delivery.go` converts current hook emissions and call-log attribution
into DeliveryRow once per decision point. It never modifies holdout arm after seeing
use. Hooks emit/log under existing limits and fail open on logging errors. Errors
appear in reading coverage; they do not silently remove rows from denominators.

## Landing bundle (W3-4)

```go
// internal/landing/bundle.go; current verify.Report and readmanifest reused.
type ReportEvidence struct {
    Plan, Tree, StoreCommit, ReportSHA256 string
    Report verify.Report
}
type SetEvidence struct {
    Tree string
    Entries []string
    Coverage readmanifest.Manifest
    SourceSHA256 string
}
type EvidenceManifest struct {
    Tree, Commit string
    Verify []ReportEvidence
    Affected, Overlap SetEvidence
}
type Bundle struct {
    Schema string `json:"schema"`
    Tree string `json:"tree"`
    GateReceiptID string `json:"gate_receipt_id"`
    Evidence EvidenceManifest `json:"evidence"`
    Digest string `json:"digest"`
}
func BuildLandingBundle(ctx context.Context, options gatereceipt.Options,
    ref string, artifacts fs.FS) (Bundle, error)
// internal/landing/accept.go
type Acceptance struct { Accepted bool; Reason string; SampleFullCI bool }
func AcceptLandingBundle(ctx context.Context, options gatereceipt.Options,
    ref string, bundle Bundle, artifacts fs.FS) (Acceptance, error)
```

Schema exactly fathom.landing/v1. JSON tags on nested proposed types are the
lower_snake_case of their fields, declaration order above. EvidenceManifest is
persisted in Bundle, not a prose-only argument. Digest is SHA256 canonical JSON of
all bundle fields except Digest itself, with LF; acceptance recomputes it.

Gate-owned capture protocol, not a lane-authored reportTrees map: before the
mandatory full gate, controller captures clean HEAD/tree and exact selector/reach
inputs. After tests/verify in that same gate, capture raw per-plan --json reports,
store watermark/commit, read-manifest coverage and selector/overlap source artifacts.
Recheck clean HEAD/tree unchanged. Write artifacts at exact paths manifest.json,
verify/<plan-basename>.json, affected.json, overlap.json under the receipt artifact
root. Hash source/report bytes and include those hashes in manifest. Capture emits
one `landing-artifacts-sha256=<manifest digest>` line INSIDE the successful gate log
before its completion. Existing gatereceipt then hashes that unchanged log. No
manifest includes receipt id, so there is no digest cycle. Controller owns capture;
product builder/acceptor execute no gate/test or arbitrary command.

Build calls existing gatereceipt.Check and reads its accepted receipt/log bytes.
Require exactly one manifest-digest line in the checked successful log. Validate
artifacts against that digest, their stored file digests, tree/commit against
checked receipt, and verify StoreCommit against the tested source identity.
verify.Report alone has no tree; ReportEvidence.Tree is captured by the gate
runner, never trusted without this log binding. Missing provenance rejects.
Require no unproven claimed row and no operational unknown promoted to pass.
SetEvidence.Coverage must show every selector/reach input read; absent/unreadable/
skipped/unknown input rejects even when Entries is empty. Canonicalize entries as
sorted unique, retain full coverage manifest and SourceSHA256. Full source artifacts
are local and available through artifacts fs.FS, not arbitrary caller maps.

Accept reruns receipt integrity on receiving clean checkout, requires exact tree,
recomputes bundle/manifest/report/source digests and complete coverage from artifact
bytes, then requires full CI for this bundle under W4-2, with SampleFullCI=true always. Altered Report/list/coverage/source bytes,
missing artifact, foreign store identity or receiving-tree mismatch rejects even
when bundle.Tree text is unchanged. The gate log binds these captured artifacts
as unsigned local evidence; this proves integrity under the existing local trust
model, not author identity. No lighter gate. A full-run disagreement opens
an incident and blocks acceptance. It is never overwritten by old receipt presence.

Proposed TestLandingEvidenceProvenance in bundle_test.go and accept_test.go seeds
one valid captured log/manifest, then separately mutates report, affected list,
overlap list, coverage, source hash and receiving checkout. Every mutation rejects;
a fresh valid unchanged bundle passes. Command: `go test -race ./internal/landing
-run '^TestLandingEvidenceProvenance$' -count=1` (one line), proposed/unrun.

## Other consumer ports, scoped to their current rows (W3-4)

```go
// internal/testrun/suspects.go: reuse FindCandidates, Run and Index.
func Suspects(run Run, changed []ChangedSymbol, idx Index) Candidates
// internal/verify/ledger_checkout.go
func recordUnderCheckout(checkoutRoot, ledgerPath string) (string, error)
// internal/metrics/hook_presence.go
type HookPresenceInput struct { Eligible, Fired, Delivered, Unknown int }
type HookPresence struct { State PresenceState `json:"state"`; Reason string `json:"reason"`; Eligible, Fired, Delivered, Unknown int }
func HookPathPresence(in HookPresenceInput) HookPresence
// internal/hookpath/proposed_file.go
type Edit struct { Start, End int; Replacement []byte }
func ReconstructProposedFile(base []byte, edits []Edit) ([]byte, error)
// internal/lint/tenancy/rules.go: existing Findings and CheckFile semantics.
func CheckRules(rel string, src []byte, rules []string) ([]Finding, error)
```

recordUnderCheckout resolves enclosing git checkout root passed by verifier,
never ambient cwd/global service root. Writes use existing verify-ledger writer,
not internal/verify's unrelated bench-ledger oracle parser. Resolve symlinks,
reject foreign checkout, return canonical local path; public rows redact host paths.
HookPathPresence state unknown for missing coverage, absent for eligible>0/fired=0,
observed when fired>0; delivered is separate, no healthy with Unknown>0. CLI fleet
health and panel consume same typed value. No eligible observations -> unknown.
Reconstruct edits use byte offsets, sorted non-overlapping on original base;
reject out-of-range/overlapping edits. Each harness adapter must decode its real
edit form into this type, no guessed common transcript. Missing whole-file input
is unknown, not a partial hypothetical file. Rules closes over current Rules() keys,
unknown key fails, no new expression language or execution. CHG-1 replay uses
internal factdelta only; no public delta API or admitted infrastructure card.

## Exact issue ownership snapshot and serial transfer

These are the current issue allowlists, not grants to this draft author. Existing
files remain with the architect until a named serial handoff. Directory grants
are reproduced as authored and must be narrowed to the fixture files below before
dispatch. Tests in a contract batch transfer once, at its landed commit, to the
consumer. No concurrent shared-file owner.

### #275 Landing bundle (CHG-5)

- `internal/landing/bundle.go` (new).
- `internal/landing/bundle_test.go` (new).

### #274 Landing accept (CHG-6)

- `internal/landing/accept.go` (new).
- `internal/landing/accept_test.go` (new).

### #273 Add bounded hook lookup over stored facts (ENG-8)

- `internal/mcp/hook_lookup.go` (new).
- `internal/mcp/hook_lookup_test.go` (new).

### #272 Attribute hook delivery to its caller and session (ENG-9)

- `cmd/fathom/hook_attribution.go` (new).
- `cmd/fathom/hook_attribution_test.go` (new).

### #298 Wire stamp (WIR-1)

- `internal/wiring/wire_stamp.go` (new).
- `internal/wiring/wire_stamp_test.go` (new).
- `cmd/fathom/doctor_wire_stamp.go` (new).

### #296 Red test suspects (CHG-3)

- `internal/testrun/suspects.go` (new).
- `internal/testrun/suspects_test.go` (new).
- `cmd/fathom/testrunhook.go` (exists).

### #267 Record forward delivery and acted-on evidence (BEN-4)

- `internal/labs/delivery.go` (new).
- `internal/labs/delivery_test.go` (new).
- `cmd/fathom/hook_delivery.go` (new).

### #297 Verify ledger (ENG-10)

- `internal/verify/ledger_checkout.go` (new).
- `internal/verify/ledger_checkout_test.go` (new).

### #276 Presence metrics (ENG-7)

- `internal/metrics/hook_presence.go` (new).
- `internal/metrics/hook_presence_test.go` (new).

### #304 Fleet ground (FLT-0, FLT-1)

- `cmd/fathom/fleet_health.go` (new).
- `cmd/fathom/fleet_health_test.go` (new).
- `cmd/fathom/observatory_fleet_health.go` (new).
- `cmd/fathom/observatory_fleet_health_test.go` (new).
- `docs/bench/readings/fleet-picture/browser-launch.md` (new).

### #322 Whatif delta (CHG-1, LIB-13)

- `internal/factdelta/` (new).
- `cmd/fathom/whatif_replay_test.go` (new).
- `docs/bench/readings/realignment/chg-1-replay-cards.md` (new).
- `docs/bench/readings/library-roadmap/lib-13-fact-delta.md` (new).

### #306 Guardrail deletion (CHG-9)

- `docs/bench/readings/realignment/chg-9-guardrail-deletion.md` (new).

### #314 Hook edit rebuild (CHG-8)

- `internal/hookpath/proposed_file.go` (new).
- `internal/hookpath/proposed_file_test.go` (new).
- `cmd/fathom/hook_proposed_file.go` (new).



### #320 Rule port (RL-9)

- `internal/lint/tenancy/**` (exists, including allowed new rule-data files).
- `internal/lint/tenancy/rules_test.go` (new).
- `calibration/rules/tenancy/` (new).

## Exact proposed shared-file reservations (W4-2)

Approve this exclusive manifest before dispatch; it does not claim #164 owns hooks.
#164's observed command surface is cmd/fathom/iac.go/iac_test.go only. Its legacy
package-wide wording is replaced for these paths, never interpreted as all cmd/.
MCP #167/#168 receive only their named IaC turns, then #273 hook lookup handback.

| Exact path(s) | Exclusive order/responsibility |
|---|---|
| internal/mcp/hook_lookup.go, internal/mcp/hook_lookup_test.go | #273 only; existing QueryInterface consumed without adding method |
| internal/mcp/tools.go | #167/#168 IaC context turns -> #273 existing find_symbol registration/handler adapter at tools.go:403+, same QueryInterface |
| cmd/fathom/sessionhook.go | #273 lookup transport -> #272 attribution -> #298 stamp -> #296 suspects -> #267 delivery -> #314 reconstruction |
| cmd/fathom/edithook.go, cmd/fathom/sessionhook_events.go | #272 -> #296 -> #267 -> #314 each turn |
| cmd/fathom/beats.go, cmd/fathom/discoveries.go, internal/mcp/calllog.go | #272 attribution -> #298 stamp/session record -> #267 delivery log |
| cmd/fathom/doctor.go, cmd/fathom/doctor_harness_firing.go, cmd/fathom/doctor_wire_stamp.go | #298 only, retains WR-12 applicability |
| cmd/fathom/verify.go, cmd/fathom/verifyledger.go, cmd/fathom/verifyledger_rows.go | #297 ledger checkout attribution after current gate owner; appendVerifyLedger remains existing writer |
| scripts/landing-capture.sh, Taskfile.yml | landing integration owner for proposed gate artifact capture only, after active gate owner handback; no gate removed |
| cmd/fathom/fleetreport.go, cmd/fathom/fleetreport_test.go, internal/console/fleet.go, cmd/fathom/console_fleet.go, cmd/fathom/console_test.go | #276 metrics wiring -> #304 CLI/panel parity; explicit integration grants |
| cmd/fathom/edithook.go, cmd/fathom/edithook_test.go, cmd/fathom/whatif_replay_test.go | #322 replay test after #314/BEN-4; hooks owner integrates code-card adapter in cmdEditHook/renderPostEdit, sole shared-file writer |
| internal/lint/tenancy/tenancy.go, internal/lint/tenancy/rules.go | #320 only, semantic parity to existing Rules/CheckFile |

Grounded sites: internal/mcp/tools.go find_symbol handler; cmd/fathom/fleetreport.go
cmdFleetReport/buildFleetReport -> cmd/fathom/console_fleet.go buildFleetPage at /fleet;
cmd/fathom/edithook.go cmdEditHook/renderPostEdit. No whatif.go or
observatory_fleet.go exists or is granted. New fleet_health.go remains #304's adapter
file only, called from existing buildFleetReport, never a substitute product entry.
Additional route tests: cmd/fathom/fleet_health_test.go::TestFleetReportPanelPresence
calls real fleet report CLI and registered /fleet page and compares the same
HookPathPresence values/unknown reasons; cmd/fathom/whatif_replay_test.go::TestEditHookCodeCardReplay
calls cmdEditHook with actual harness edit fixture, consumes reconstructed full file
and asserts only an admitted code card is rendered/logged. Missing factdelta/card
or reconstruction evidence stays unknown. No nonexistent what-if verb is assumed.
Reserve cmd/fathom/edithook_teststorun.go and edithook_teststorun_test.go to #296's
serial suspects turn after #272 and before #267. All new tests/fixtures transfer
at the same frozen owner handoff, not concurrent adapter edits. Batch test files remain
CTR-12-owned until its U3 RED mechanism lands, then transfer at exact commit.
No hook turn can overlap another turn merely because new adapter filenames differ.

## Exact proposed fixtures and failing proofs

Fixtures reserved to CTR-12 until transfer: internal/landing/testdata/contracts/
`valid-bundle.json`, `wrong-tree.json`, `missing-log.json`; internal/wiring/testdata/
contracts/`wire-valid.json`, `wire-missing.json`; internal/labs/testdata/contracts/
`delivery.jsonl`, `actions.jsonl`, `preflight-failed.jsonl`, `zero-use.jsonl`.
All synthetic. Negative CUE stays commented if later included in schema packages.
No active broken default build is committed by this draft.

| Consumer test path/name | Required assertion / package for exact command |
|---|---|
| internal/landing/bundle_test.go::TestLaneLandingBundle | clean same-tree gate/log/verify attestation builds; wrong/missing report binding refuses; affected/overlap order stable; ./internal/landing |
| internal/landing/accept_test.go::TestLaneLandingAccept | changed tree/command/log/bundle bytes refuse; full CI on every bundle and mismatch incident; ./internal/landing |
| internal/mcp/hook_lookup_test.go::TestLaneHookLookup | stored exact name/limit/deadline, no hook query_symbols/off-surface calls, liveness does not query graph; ./internal/mcp |
| cmd/fathom/hook_attribution_test.go::TestLaneHookAttribution | real hook and call-log rows carry explicit session/lane; missing remains unattributed, caller context wins model args; ./cmd/fathom |
| internal/wiring/wire_stamp_test.go::TestLaneWireStamp | each four inputs changes digest, stable order, idx separate, once/session integration and absent doctor unknown; ./internal/wiring |
| internal/testrun/suspects_test.go::TestLaneRedTestSuspects | existing Go/JUnit failure fixtures map to FindCandidates with unresolved tests retained, wrapped test hook integration observed; ./internal/testrun |
| internal/labs/delivery_test.go::TestLaneDeliveryLog | holdout stable; exact target/next-call/deadline; missing coverage unknown; zero-use admitted/assigned arm retained, failed preflight voids whole pair before agent; ./internal/labs |
| internal/verify/ledger_checkout_test.go::TestLaneVerifyLedger | gate clone ledger under verified checkout, parent cwd/symlink cannot move it to another checkout; ./internal/verify |
| internal/metrics/hook_presence_test.go::TestLanePresenceMetrics | missing coverage unknown, eligible/nonfiring absent, fired vs delivered distinct, zero eligibility not healthy; ./internal/metrics |
| cmd/fathom/fleet_health_test.go::TestLaneFleetGround | actual fleet health verb and existing fleet panel same typed presence/unknown values, launch witness retained; ./cmd/fathom |
| cmd/fathom/whatif_replay_test.go::TestLaneWhatifDelta | real hook replay for admitted code card, no public delta/new infra card, missing source unknown; ./cmd/fathom |
| internal/hookpath/proposed_file_test.go::TestLaneHookEditRebuild | whole-file and fragment edit forms per actual harness, invalid offsets/overlaps/unknown adapter refuse; ./internal/hookpath |
| internal/lint/tenancy/rules_test.go::TestLaneRulePort | every current rule matches CheckFile fixtures and unknown rule refuses, no provider/spawn; ./internal/lint/tenancy |

For each table entry run exactly `go test -race PACKAGE -run '^TESTNAME$' -count=1`
with that row's literal PACKAGE/TESTNAME, first assert its declaration exists via
`rg -n '^func TESTNAME\(' PATH`. Missing declaration is absence, never observed RED.
Batch entry: internal/landing/lane_contract_test.go::TestLaneContractsDelivery;
`go test -race ./internal/landing -run '^TestLaneContractsDelivery$' -count=1`.
Also proposed internal/wiring/lane_contract_test.go and
cmd/fathom/hook_lane_contract_test.go exercise cross-package fixture serialization.
All are proposed future runnable commands, none ran on this draft.

CHG-9 deletion reading #306 remains a separate preregistered history cohort,
independent labels and prior/self-declared I45 classifier; “rare/mostly declared”
needs an explicit pre-data decision, not fixture presence or a new invented bar.
No paid session, deploy, full gate or product execution is authorized here.

## Evidence and limits

Existing gatereceipt.Receipt/Options/Check, verify.Report, testrun.Run/FindCandidates,
labs.FiringReadingRow/Stages/Stage, wiring receipts/ledger, cmd sessionWindowRow/
recordSessionWindow/stampBeatMeta, metrics.CallEntry and tenancy.Rules/CheckFile
are reuse anchors. None implies the new row exists. Proposed fields are only for
current attributed delivery/landing inputs. Draft verified by diff check and commit
hook only. No RED/race/gate/conformance/coverage/latency claim. Rows stay unchecked.
Decision models: none. Deterministic joins and exact-tree integrity need no model;
provider none is complete. Manual review opts out of automation, not evidence bars.

## Mandatory holdout exclusions (ADR-080 d6, final F4)

DeliveryHoldout accepts a trusted DeliveryChannel from the actual emission path,
not a model argument or a capability name guessed from prose. BEFORE hashing:
only ChannelAdvisory is eligible. ChannelBlockingGuard/ChannelGate/ChannelVerify/
ChannelReadiness/ChannelFailureTriggered always return false,nil. Failure-triggered
advice is ChannelFailureTriggered even if that capability otherwise serves advice.
Unknown/invalid channel returns false, typed RefusalError; caller emits the FULL
line and reports unknown eligibility. Missing advisory session/capability also
returns false/error and full delivery. Logging failure never authorizes suppression.
Mandatory lines record ArmMandatory, not a heldout arm or false-firing row.
Only explicitly eligible advisory lines enter holdout. No automatic feature switch.

Proposed TestDeliveryHoldoutMandatoryChannels in internal/labs/delivery_test.go:
each mandatory class, sessions s0..s1023 and two capability ids, including hashes
below/above proposed threshold, is never held out. Unknown/invalid -> false plus
errors.As RefusalError. Advisory same input repeats its assignment; one known
below-threshold case holds out, above-threshold does not. Proposed
cmd/fathom/hook_lane_contract_test.go::TestMandatoryHookLinesNeverHeldOut invokes
actual hook output for blocking guard/gate/verify/readiness/failure fixtures and
asserts complete payload for both hash cases and log-write failure. Caller model
arguments cannot change channel class. Commands, each one line:
go test -race ./internal/labs -run '^TestDeliveryHoldoutMandatoryChannels$' -count=1
go test -race ./cmd/fathom -run '^TestMandatoryHookLinesNeverHeldOut$' -count=1
Proposed/unrun. W3-2 critic compares every assertion. W4-2 settles the 26/256 advisory rate;
ADR-080 d6 still governs exclusions and determinism.

## W3-4 exported vocabularies and exact wire map

Local declarations in their existing owning packages, not a new shared package:
~~~go
// labs/delivery.go
type DeliveryChannel string
const (
    ChannelAdvisory DeliveryChannel = "advisory"
    ChannelBlockingGuard DeliveryChannel = "blocking-guard"
    ChannelGate DeliveryChannel = "gate"
    ChannelVerify DeliveryChannel = "verify"
    ChannelReadiness DeliveryChannel = "readiness"
    ChannelFailureTriggered DeliveryChannel = "failure-triggered"
    ChannelUnknown DeliveryChannel = "unknown"
)
type CallPhase string
const ( PhasePreflight CallPhase = "preflight"; PhaseAgent CallPhase = "agent" )
type DeliveryArm string
const ( ArmDeliver DeliveryArm = "deliver"; ArmHoldout DeliveryArm = "holdout"; ArmMandatory DeliveryArm = "mandatory" )
// cmd/fathom/hook_attribution.go
type CallerKind string
const (
    CallerAgent CallerKind = "agent"
    CallerEditHook CallerKind = "edit-hook"
    CallerGate CallerKind = "gate"
    CallerRefresh CallerKind = "refresh"
    CallerPlumbing CallerKind = "plumbing"
    CallerUnattributed CallerKind = "unattributed"
)
// wiring/wire_stamp.go
type IndexState string
const ( IndexFresh IndexState = "fresh"; IndexBehind IndexState = "behind"; IndexUnknown IndexState = "unknown" )
// cmd/fathom/doctor_wire_stamp.go
type WiringState string
const ( WiringUnknown WiringState = "unknown"; WiringConfigured WiringState = "configured"; WiringObserved WiringState = "observed" )
// metrics/hook_presence.go
type PresenceState string
const ( PresenceUnknown PresenceState = "unknown"; PresenceAbsent PresenceState = "absent"; PresenceObserved PresenceState = "observed" )
// Each owning package, typed errors for its refusal/limit results:
type RefusalError struct { Reason string `json:"reason"` }
func (e *RefusalError) Error() string
type LimitError struct { Limit int64 `json:"limit"`; Observed int64 `json:"observed"` }
func (e *LimitError) Error() string
~~~

Every compact new field has explicit snake_case JSON tags, exact mapping:
WireInputs fathom_version/extractor_epoch/owned_hooks_digest/surface_digest;
SessionCall session_id/call_id/phase/ordinal/targets; CallCoverage session_id/
first_ordinal/last_ordinal/complete/reason; FunnelInput rows/calls/coverage/pairs/
deadline_ms/next_calls; DeliveryResult rows/void_pairs/unknown_sessions;
ReportEvidence plan/tree/store_commit/report_sha256/report; SetEvidence tree/
entries/coverage/source_sha256; EvidenceManifest tree/commit/verify/affected/overlap;
Acceptance accepted/reason/sample_full_ci; HookPresenceInput and HookPresence
eligible/fired/delivered/unknown; Edit start/end/replacement.
WireStamp already uses session_id/wire/idx. Reason is free explanatory prose,
not a refusal-code substitute. New closed fields use their named type/constants,
not string; existing imported models stay unchanged.

Exported result slices are nonnil recursively: no-match lookup, delivery rows/
void_pairs/unknown_sessions, targets, verify/affected/overlap sets, suspects/findings and coverage lists. Marshal [] not null. Pure APIs have no context;
I/O builder/acceptor/lookup/writer may use it. RefusalError/LimitError support
errors.As. Additive only, no door package/import, values withheld, no future fields.
Proposed TestDeliveryContractsAPIHouseRules in internal/landing/lane_contract_test.go
plus owning-package fixture adapters asserts explicit wire names, all empty result
arrays [], typed enums/errors.As, and no context on pure APIs. Exact command:
go test -race ./internal/landing -run '^TestDeliveryContractsAPIHouseRules$' -count=1.
This is proposed/unrun, not a gate claim.

## Steward #263 proposal reconciliation

Read issuecomment-6900960506. Its batch/consumer paths plus
docs/bench/readings/contributor-model/contracts-delivery.md are proposed ownership,
not active grants or proven prerequisites. Full list is this consumer table plus
three aggregate files. Independently run each matching rg declaration guard before
its exact anchored go test, recording base/head/command/exit and actual assertion
or compile witness. Missing executable/dependency/package/test or empty -run never
counts RED. W3-2 supersedes only batch-RED placement: implementing lanes capture
failures; this contract retains all assertions and serial ownership transfers.
No forge changes or dispatch authorization. #320 still needs CHG-10/CTR-3/CTR-12.

## W4-2 exact scope/grant/rate disposition

This ruling-authorized scope change removes ONLY the incomplete FLT-3 and FLT-10
port consumers from CTR-12. No attempt to repair those ports in a third design
round: Event/ParseEvents/ExportLanes/View declarations, their event/export test rows,
and #315/#316 port implementation grants have been removed from this batch.
Transferred to later exact delivery-integration contract:
- #315 FLT-3: AppendLaneEvent with current harness/model/host-label/row-id fields,
  rotation/archive and real hook handoff, not the former ParseEvents helper.
- #316 FLT-10: ExportLaneState/ReadLaneExports local paths, per-machine newest age
  and named not-heard-from state, not the former pure ExportLanes helper.
- Transferred proof names remain TestLaneLaneEvents in
  internal/lanefile/events_test.go and TestLaneLaneViews in
  internal/lanefile/export_test.go. No assertion/RED for them is claimed here.
FLT-4's existing dependency on FLT-3 remains unchanged; mixed issue #316 is not
made dispatch-ready by this scope split. No FLT-4 or other consumer plan row is
removed or marked complete. The later contract must preserve that dependency and
own its command/projection handoff explicitly. Other CTR-12 consumers/tests/grants
are retained. Existing parallel-lanes hook order remains governing context, not
a premature event/export write grant.

W4-2 U2 approves the retained exact serial/capture grants: one hook owner and active
gate owner handback before landing-capture changes. No broad cmd/fathom grant.
W4-2 CI requires a full CI run on EVERY bundle, with SampleFullCI=true always,
not a 1/16 hash sample or optional skip. A failed/missing full CI cannot authorize
landing; controller retains that release/integration decision. Amend the existing
TestLaneLandingAccept assertions to require SampleFullCI=true for every candidate
fixture and verify a missing/failed CI witness prevents controller acceptance.
The API flag reports required work, never a user-tunable probability.

W4-2 H approves exactly 26 of 256 advisory hash assignments, after the mandatory
ADR-080 d6 channel exclusions. Existing holdout helper/real-hook output tests
preserve every forbidden channel's full delivery, including log failure and
unknown channel. No enforcement or failure-line suppression is allowed.
No generic API, scope, rate or grant question remains for this retained batch.
No other ports, future fields or implementation added in this compliance revision.

#263 steward proposal originally named the now-transferred two lanefile tests;
retain them for the later contract, not this batch's proof manifest. All retained
consumer proof guards and receipt paths still apply. W3-2 implementing-lane RED
and assertion-by-assertion critic remain. Missing prerequisites never become
ready because the contract now has ruling-backed scope. Approval-by-rule follows
exact critic LAND, and later integration queues for bundle3 after ACK2.
