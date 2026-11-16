# WPF adapters — dependency boundary and contract stop

Status: blocked draft for independent criticism. No approval or implementation
readiness is claimed. This document owns itself only. Source inspected:
`54bca03154ea35c39bf08acba4c3572e843da56a`.

## Intent and scope decision

Consulted: the intent router, principles, invariants, scope and non-goals,
futures and decisions indexes, ADR-029 decision 6, ADR-082, proposed ADR-110,
`docs/design/wiring-proof.md` WP-D1–WP-D6, the exact WPF-2/3/4/5/6/9 rows,
`docs/contracts/labs-decoder.md`, `wire-admission.md`, approved `ctr-12.md`,
`ctr-13.md` and `bench-consolidation.md`, and the separately approved WPF-1 contract.
Standing track rulings T2-1/2/3, T4-1 and W2-3/W3-2/W3-4 apply.

P2/P3/P7/P17/P18 require deterministic checks, visible missing evidence and
computed status. I9/I12/I35/I53 preserve fail-open hooks and existing payload
and deadline bounds; I13 preserves receipt-owned configuration writes; I38
requires honest coverage; I61 withholds retained values; I62 excludes judgments
from proof. Their enforcement remains what `docs/intent/invariants.md` names;
this document adds no enforcement. No dispatch loop, runner, sampler, store,
registry, knob writer, report owner, door package or future feature is authorized.
No existing compatibility surface changes.

No measured revisit or future-promotion condition is established. ADR-082's
Codex-arm revisit is relevant to WPF-6: WP-D5/proposed ADR-110 describe the
bounded deterministic extension; they do not approve a paid fork/stanza arm or
supply a pinned event format. No live harness inability or missed fault is
inferred from a missing software interface. F50/F64/F65 stay deferred.
Free option: retain WPF-1's phase-separated shapes and the existing owners;
no speculative adapter framework is needed to keep later harnesses possible.

**D1 — stop at the missing ports.** The earliest dependency group is WPF-2/4
(preflight production and admission), followed by its readiness consumers.
It is not presently contractable end to end. Neither WPF-3 nor WPF-9 provides
an independent complete group with the handed inputs currently available.
The narrow deliverable here is the dependency boundary below, not a partial
implementation grant. No public type, schema, signature or new named test is
invented. All six rows remain untouched. Authority: WPF plan introduction and
its Handed clauses; T2-2; `wire-admission.md` §Authority and exact missing-port
witnesses; the author issue's explicit stop instruction.

## What the freezes actually hand over

**D2 — existing source is not the future port.** `labs-decoder.md` freezes
`internal/transcript/decode.go` and `events.go`; `wire-admission.md` freezes
current scoring, stream parsing, stamps, beats and doctor rows. Their declared
source base is `83226aece9b0f421381373f8dafc4a912d3888e5`. Landing these documents
does not implement their unchecked LAB/WR rows or add the absent interfaces.
Their own draft status lines are not upgraded by this document.

Existing callable decoder:

```go
// internal/transcript/events.go:39; labs-decoder.md §Exact callable interfaces
func DecodeTranscriptEvents(path, salt string, fileIndex int) Result
```

`Event` (`events.go:19`) has Kind/Line/Cwd/GitBranch/Timestamp/ToolUseID/Name/
Shape/IsError/Bytes/HookEvent, without phase, byte range, stable cited target or
capture-completeness fields. `Result` and its failure counters are exactly the
freeze, including existing nil/string behavior. Do not retrofit W3-4 by changing
this API. `SessionResult` (`internal/scenariobench/types.go:385`) has aggregate
MCP/hook/use fields; those are not WPF-1 preflight evidence. Its RawPath and
transient text are not retained receipt fields. Private helpers in `cmd/fathom`
and scenariobench are not importable leaf ports.

**D3 — BEN is settled, not missing.** T2-1 and W5-3 settle CTR-13 and the
consolidation manifest. Use their exact final definitions, including the later
W3-4 named-enum/JSON amendments and typed disposition section, rather than the
older string shorthand earlier in that document:

```go
// internal/benchready; ctr-13.md §Six readiness harnesses
// Control.Name and ControlResult.Name use the frozen ControlName type.
type Runner func(context.Context, Control) (scenariobench.SessionResult, error)
func RunReadiness(ctx context.Context, controls []Control, run Runner) ([]ControlResult, error)
// internal/benchledger; ctr-13.md §One preflight admission contract
func AdmitPair(pairID string, treatment, control PreflightReceipt) (PairPreflight, error)
```

Until implementation and helper wiring land, WPF-5/6/9 may use fakes satisfying
the frozen BEN interfaces, with real registration as a wiring request. BEN's
absence in Go source alone is not a reason to demand a new interface or runner.
Conversely, Runner returns a SessionResult, not a phase-labelled WPF receipt;
it cannot supply the missing evidence port by reinterpretation. T2-1 permits
fakes of frozen interfaces, not invented receipt fields. CTR-13's admitted/void/
unknown disposition and its distinction between missing/late and definite
pre-agent failure stay intact; this draft does not resolve them with Reason text
or implement another classifier. BEN-1 still requires all B1–B3 proofs.

BEN-4's delivery inputs are also already frozen in approved
`ctr-12.md:103-169` (`internal/labs/delivery.go`):

- `DeliveryRow`: SessionID/LaneID/Capability/CallID/Target/At string,
  Phase CallPhase, Arm DeliveryArm, Eligible/Fired/Delivered bool, LatencyMS float64.
- `SessionCall`: SessionID/CallID string, Phase CallPhase, Ordinal int,
  Targets []string; `CallCoverage`: SessionID string,
  FirstOrdinal/LastOrdinal int, Complete bool, Reason string.
- `FunnelInput`: Rows []DeliveryRow, Calls []SessionCall,
  Coverage []CallCoverage, Pairs []benchledger.PairPreflight,
  DeadlineMS float64, NextCalls int.
- `DeliveryResult`: Rows []FiringReadingRow, VoidPairs/UnknownSessions []string;
  `func DeliveryFunnel(in FunnelInput) (DeliveryResult, error)` reuses the
  existing result/renderer, not a replacement funnel.

Apply CTR-12's final named-enum/JSON amendment (`:532-535`, `:562-577`):
CallPhase is preflight/agent; DeliveryArm is deliver/holdout/mandatory;
compact fields have the specified lower_snake_case JSON tags and result slices
are nonnil. DeliveryRow.CallID joins SessionCall.CallID. Target is an exact
existing file/symbol/test anchor; action is a later call's Targets within
(delivery.Ordinal, delivery.Ordinal+k], from the complete session call population,
including calls with no delivery decision. Ordinals are positive, unique recorded
event order. Duplicate CallID is unread evidence; duplicate/missing ordinals,
unknown event loss or noncontiguous claimed coverage yield UnknownSessions,
not zero action. Coverage must span delivery+k or a witnessed session end;
Complete requires captured-count/raw-digest verification, not an authored boolean.

CTR-12 `:599-615` removes only FLT-3/10; `:640-687` retains BEN-4 capture/archive
obligations, including exact raw bytes/digests, time ranges, complete call order
and named missing/corrupt/unreadable periods. CTR-13's older HOLD wording at
`:636-638` concerns FLT-3/10 and does not erase that retained approved scope.
No replacement contract for these frozen inputs is requested. Still needed are
the actual producer handbacks into these shapes, their WPF-1 evidence/provenance
bindings under WP-D6, and LAB-M1a's per-surface classifier-validation inputs
(`feature-labs.md:120`). Frozen types and capture obligations alone do not supply
those observations or establish a complete executable WPF-3 group; unknown
evidence cannot be faked under T2-1.

## WPF-1 dependency inspected separately

**D4 — schema contract versus implementation proof.** Approved WPF-1 contract
snapshot: `1eb42204a4b3a83e671ad7a44b6d2a3312012cab`, document
`docs/contracts/wpf-1.md`. The source schema was inspected read-only in the
separate implementation checkout at `45261e6dd18770aa95895437bb939c9010b725d0`.
These are observations of separate work, not files granted to this lane or a
claim that the implementation has integrated. The implementation checkout was
clean at inspection; later work can change these bytes.

| Inspected relative file | SHA-256 |
|---|---|
| `schema/wiring-proof/schema.cue` | `de1dfd64fcc40651bcc7b2e073b23e9c6fba2b333501f117b9ca10eab28b5711` |
| `schema/wiring-proof/testdata/valid.cue` | `a3fe2687420a6b6b763a1ebab0c9f06d2ad0ff9d605c664b6dc6beaf4c5d5278` |
| `schema/wiring-proof/testdata/invalid.cue` | `220ee694aabb31a887cb123bb1b9baee8c6f9a0b518750704ccbb4386ca3391b` |
| `internal/wiring/conformance_schema_test.go` | `9b9725aff51d655a88fda07958dfd38de5eb5ec5155c4cd3efbd2b660a321af9` |

Exact schema boundaries inspected: #Profile at line 66, #Preflight at 193,
#Outcome at 448, #Conformance at 504. WPF-1 exports no Go receipt or validator
API. Its CUE tests read source directly. It constrains declared hashes and
bindings, not cryptographic recomputation, observed installation or target joins.
No new phase receipt is needed or authorized here.

Preserve the approved contract's definitions: five environment requirements
W0/W1/W2/W3/W6; no W4/W5/k/bars in #Preflight or #Profile; agent-only #Outcome;
#CaptureSummary records complete empty capture without inventing an action;
unknown capture is not acted zero. Delivery parameters are separately hashed
canonical `{k,bars}` with capability-sorted bars, bound by the profile's
`delivery_parameters` source digest. That binding does not make outcomes admission
inputs. WPF-9 owns actual hash recomputation.

T4-1 permits disabled-channel NOT-APPLICABLE only with frozen preflight evidence
of absence. Surface-off alone exempts no other channel, N/A never becomes PASS,
active checks remain unchanged, W0 product identity and W6 host/resource floors
remain. Baselines retain their actual applicability and own environment checks,
without invented daemon stamps. The integrating helper must apply WPF-1 §F1's
narrow CTR-13 applicability amendment; this lane cannot edit it.

## Exact row readiness and missing handbacks

| Row and governing source | Current input and exact blocker |
|---|---|
| WPF-2; `wiring-proof.md:24`, WP-D1/D6 | WPF-1 specifies receipt data; doctor exposes `runDoctorChecks(root, explicitSvc, harness, baseURL string) []doctorCheck` at `cmd/fathom/doctor.go:248`. WR-12 has no `checkWiringFiring` declaration or session-join result contract; WR-4 has no `computeWireStamp`; WR-5 does not propagate a computed wire/idx join. Need their owning callable signatures and result/evidence bindings before `checkSessionConformance` can reuse them. Existing hooks_firing/rule_outcome_writer rows are not those ports. |
| WPF-3; `wiring-proof.md:25`, WP-D1/D6 | `FiringReading(points []Point, labs []Lab) []FiringReadingRow` exists at `internal/labs/firing.go:80`. It explicitly marks acted-on unmeasured (`:36`); Point/decoder events are not phase-labelled delivery/action capture. CTR-12:103-169 already freezes BEN-4's DeliveryRow, SessionCall, CallCoverage, FunnelInput and DeliveryFunnel, with retained capture/archive obligations at :640-687 (D3). Need actual LAB-11/BEN-4 producer handbacks into those frozen shapes, WPF-1 evidence/provenance bindings under WP-D6 and LAB-M1a classifier-validation inputs before fixing `EvaluateDeliveryConformance`. No replacement input contract, funnel or report model; the frozen shapes do not prove those observations or complete WPF-3 readiness. |
| WPF-4; `wiring-proof.md:26`, WP-D2 | BEN's AdmitPair contract is frozen. WR-1's `ReceiptDefect` and WR-2's `VoidWithoutPositiveControl` have no callable declarations/signatures. `internal/contextbench/report.go:192-239` decodes final JSON and scores without a receipt argument. Need those admission ports and WPF-2's production receipt handback before `DeclaredConformanceGate` can extend them. Do not create a parallel gate or scorer. |
| WPF-5; `wiring-proof.md:36`, WP-D5 | Claude decoding and frozen BEN Runner are available contract inputs. Missing: an owner-produced mapping from scripted registration/negotiation/primer/hook/beat/resource observations to WPF-1 evidence, with actual pre-agent boundary and startup bindings. Aggregate SessionResult is insufficient. Need that exact callable producer/consumer handback before fixing `RunClaudeConformanceScript` and its fixture files. Fake BEN wiring does not prove the missing mapping. |
| WPF-6; `wiring-proof.md:37`, WP-D5 | LAB-7 freezes Claude only. Need actual Codex harness pin, transcript contract bytes/digest, event mapping, malformed/unreadable coverage and phase/boundary source, plus WPF-5's observation handback. No Codex format is inferred from DecodeLine accepting unknown fields. `RunCodexConformanceScript` cannot be fixed from a fictional format. BEN may be faked as above. |
| WPF-9; `wiring-proof.md:43`, WP-D3 | WPF-1 fixes canonical hashes, but LAB-28's `loadLabToolDescriptions` and LAB-29's `replaceStanzaBlock` have no declarations. Their plan rows (`feature-labs.md:132-133`) name behavior, not complete effective-output interfaces. Need exact effective-content/override handbacks and BEN-1's call-site mapping before fixing `ValidateWiringProfile` and its registration adapter. Existing CheckArmParity at `internal/scenariobench/parity.go:164` does not expose a complete effective profile; it explicitly marks unrendered wired inputs incomplete. A hash of caller-asserted digests alone would not prove undeclared differences absent. |

Declaration inventory at the inspected source found none of the absent functions
above, nor new WPF functions, AdmitPair or RunReadiness implementations. This is
static absence, never a test RED or proof of runtime failure. Only genuinely
missing signatures and producer/evidence handbacks are requested from their owners; frozen BEN-4 input shapes are reused, not re-contracted.
Missing parameter lists are deliberately not guessed.

## Prospective files and precise wiring requests

**D5 — no implementation grant in this stop draft.** The exact new Go files
already named by the rows are listed to preserve ownership boundaries. They are
not created, reserved or handed to implementation here:

| Row | New files only, if a later complete contract is cleared |
|---|---|
| WPF-2 | `cmd/fathom/doctor_conformance.go`, `cmd/fathom/doctor_conformance_test.go` |
| WPF-3 | `internal/labs/conformance.go`, `internal/labs/conformance_test.go`, `cmd/fathom/instrument_conformance.go` |
| WPF-4 | `internal/benchledger/conformance_gate.go`, `internal/benchledger/conformance_gate_test.go`, `internal/contextbench/conformance_gate.go`, `internal/contextbench/conformance_gate_test.go` |
| WPF-5 | `internal/benchready/conformance_claude.go`, `internal/benchready/conformance_claude_test.go` |
| WPF-6 | `internal/transcript/codex_conformance.go`, `internal/transcript/codex_conformance_test.go`, `internal/benchready/conformance_codex.go`, `internal/benchready/conformance_codex_test.go` |
| WPF-9 | `internal/labs/wiring_profile.go`, `internal/labs/wiring_profile_test.go`, `internal/benchledger/wiring_profile_adapter.go`, `internal/benchledger/wiring_profile_adapter_test.go` |

WPF-5/6 name fixture directories, not exact file manifests. Those manifests must
be fixed from actual pinned observations in the resumed contract; no invented
fixture filenames or directory-wide ownership grant is issued here.

Existing-file wiring stays with the integrating helper: WPF-2 adds its check
through runDoctorChecks in `cmd/fathom/doctor.go`; WPF-4 extends the pre-score
receipt call in `internal/contextbench/report.go`; WPF-5/6 register through frozen
RunReadiness and the kept BEN command; WPF-9 is invoked by BEN-1 registration
before cell construction, using the existing override/knob owners. Exact adapter
calls await the missing input signatures, rather than a guessed command switch.

WPF-3 extends adoptionLabsSection in `cmd/fathom/instrument_adoption_labs.go:80`.
That shared edit is serial: WPF-3, then FTU-0, FTU-3, FTU-5 (`feature-tuning.md:21`).
T2-3 permits FTU-0's separate `schema/labs/tuning.go` embedding tuning.cue and
its own validator with valid/invalid fixtures; production validation wiring is
a helper request, not permission to edit schema.go/schema.cue or bypass WPF-3.
No FTU file is part of this draft.

## Acceptance obligations retained, not executable tests assigned

**D6 — preserve the rows' proof bars without fake RED.** These are the exact
plan test selectors and minimum assertions a later fully specified contract
must expand into exact named implementing tests. They are not new tests, test
runs or a claim that a prefix selects any existing declaration. W3-2 assigns
real failing tests to implementation, after inputs and exact file grants settle.

| Row / plan test selector | Governing exact acceptance obligations |
|---|---|
| WPF-2 / `^TestDoctorSessionConformance` | WPF-2/WP-D1: exact product mismatch fails even for an ancestor; missing primer, hook or W6 samples cannot pass; prove off-channel absence, startup digest binding and receipt before real agent action. |
| WPF-3 / `^TestConformance(Funnel\|Adoption)` | WPF-3/WP-D1 and WPF-1 outcome definitions: zero opportunities unmeasured; positive opportunities with zero use measured zero only with validated eligibility; zero action requires complete capture; duplicate calls do not inflate distinct-call windows; wrong targets do not count; exclude scripted probes; test k-1/k/k+1 boundaries; non-use never changes admission or task score. Report per-capability rates and unknowns through adoptionLabsSection. |
| WPF-4 / `^TestDeclaredConformanceGate` | WPF-4/WP-D2/BX-40: both preflights finish before either agent; broken controls cannot score; missing/late evidence cannot admit; definite failed environment checks void the pair as instrument_defect, while unknown remains the frozen typed unadmitted disposition. Healthy zero-use retains actual assigned-arm scores/tokens; varying W4/W5 with fixed task results changes neither admission nor assigned-arm totals. No agent fields in gate input. |
| WPF-5 / `^TestClaudeConformanceScript` | WPF-5/WP-D5: fixed registration, prime, every declared hook, answered probe and W6 checks; each construction-labelled environment fault detected against its healthy twin; missing observation UNKNOWN; healthy preflight with no later calls/actions admitted; probes labelled preflight; fixtures prove adapter behavior only. |
| WPF-6 / `^TestCodexConformance` | WPF-6/WP-D5: pinned events with explicit phases and malformed/unreadable counts; missing prime, unnegotiated tools, silent declared hooks and late preflight cannot admit; probes excluded from agent use; healthy zero later calls/actions admitted; no paid fork suite. |
| WPF-9 / `^TestWiringProfileDigest` | WPF-9/WP-D3 and WPF-1 digest definition: hash actual canonical effective data, including budget/environment and separate delivery-parameter binding; refuse undeclared differences, outcome-based admission requirements and profile names as digests; preserve separate outcome k/bars; registration adapter invoked before cell construction; no second knob writer. |

All future new exported shapes obey W3-4: named closed enums, explicit
lower_snake_case JSON, nonnil result slices encoding [], typed refusals/limits,
context only for I/O, additive changes, values withheld by default and no door
package. That rule does not silently rewrite the frozen existing decoder API.

## Remaining handbacks and undecided points

The missing code contracts above are dependencies, not new policy choices.
D1–D6 cite their governing rows/designs/rulings; no approval is issued here.
Two bounded sequencing recommendations for the controller, each under 300 characters:

- U1: Recommend the WR/preflight owner hand back exact observation and admission ports first, then resume WPF-2/4. Alternative: authorize a separate upstream port contract. Do not implement those ports in an adapter lane.
- U2: Recommend resume WPF-9 after LAB-28/29 effective outputs and BEN registration mapping are frozen. Alternative: separately authorize a hash-only subtask with a smaller proof claim; it cannot close WPF-9 or certify undeclared-difference detection.

## Static evidence and limits

Only document/source/declaration inspection, file digests, ownership checks and
`git diff --check` are author evidence. No build, test, readiness script, schema
validation, lease acquisition, full gate, plan verification, paid call or external
write is performed. No source-freeze document is an implementation receipt.
Independent criticism remains pending; no plan row or approval marker changes.

## Decision models

None. Provider none supplies deterministic checks; no model may manufacture
missing observations, fill a phase boundary or promote an unknown to proof.
