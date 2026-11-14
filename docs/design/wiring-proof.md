# Wiring proof before a reading

Status: Proposed 2026-11-14 (architect ruling on owner direction).
Decision draft: ADR-110. Work order: `docs/plans/wiring-proof.md`.
This document specifies work. It reports no new conformance reading.

## 1. Findings

- WP-F1 Doctor already checks registration, daemon reachability, binary freshness, installed hooks and observed firing. Those are separate checks, not a session ladder (`cmd/fathom/doctor.go:248`, `cmd/fathom/doctor.go:632`, `cmd/fathom/doctor.go:812`, `cmd/fathom/doctor.go:1543`). Configuration evidence is weaker than a transcript showing the negotiated surface.
- WP-F2 Session start and pull priming share product code (`cmd/fathom/sessionhook.go:40`, `cmd/fathom/sessionhook.go:326`). Hook beats stamp version and harness (`cmd/fathom/beats.go:114`). Adoption reads call logs and groups version eras (`cmd/fathom/instrument_adoption.go:113`, `cmd/fathom/instrument_adoption.go:313`). None of those counts alone proves eligible use.
- WP-F3 Readiness already distinguishes supported bindings and absent instructions (`cmd/fathom/hookreadiness.go:35`). Doctor has per-harness installation and firing checks (`cmd/fathom/doctor_harness_firing.go:108`, `cmd/fathom/doctor_harness_firing.go:182`) and channel projections (`cmd/fathom/doctor_channels.go:66`). Preserve these distinctions rather than translating skipped into pass.
- WP-F4 Receipt work already owns session stamps, beat joins, doctor snapshots and the firing diagnosis (`docs/plans/wiring-receipts.md#WR-4`, `WR-5`, `WR-7`, `WR-12`). `realignment.md#WIR-1` names the same integration goal. These remain dependencies, not new implementations here.
- WP-F5 Labs owns transcript decoding, eligibility classification, funnel reading and result writing (`docs/plans/feature-labs.md#LAB-7`, `LAB-9`, `LAB-11`, `LAB-12`). The proof program owns forward delivery logging and scripted readiness (`docs/plans/proof-program.md#BEN-4`, `BEN-2`). This design supplies conformance contracts to those owners.
- WP-F6 Codex cloud loading and runs remain unverified in the adoption guide (`docs/adoption/cloud-sessions.md:152`). A demonstrated WSL conformance result is unknown. A Linux result cannot stand in for a Windows host through WSL.
- WP-F7 There is an explicit daemon-free exception in ADR-105 decision 9 and `docs/design/proof-program.md#BN-D14`. There is also a Claude-only labs boundary (`docs/plans/feature-labs.md#LAB-D7`). Both need an architect reconciliation, specified below.

Intent: retain deterministic checks, explicit unknowns and fail-open product hooks (I9, I12, I35, I53, I62 in `docs/intent/invariants.md`). Keep configuration changes through owned wiring receipts (I13). Preserve the dispatch-loop prohibition (`docs/intent/scope/non-goals.md:27`). This design adds no MCP verb. No map-future trigger is asserted fired. Current pins and observed harness coverage must be read, never inferred from a guide.

## 2. Design decisions

### WP-D1 A deterministic conformance ladder per harness

Evaluate each cell against its declared profile and harness pin. Preserve a separate result for every rung. W0-W3 and W6 are environment checks proven by a scripted preflight before the agent acts. W4 and W5 are treatment outcomes, never admission checks. Do not derive validity from a highest contiguous rung, because unmeasured W4/W5 cannot block a proven W6. Environment verdicts are PASS, FAIL, UNKNOWN or declared NOT-APPLICABLE. A declared absence passes only when preflight evidence proves that absence. Applicability and probe workload are frozen before collecting data.

| Rung | Evidence required | Existing input and missing join |
|---|---|---|
| W0 Installed and serving HEAD | Before agent actions, installed binary and answering process identify the preregistered product commit. Subject HEAD and index commit are separate fields. | Reuse doctor reachability and binary checks. Add exact identity comparison because reachability is insufficient. |
| W1 Surface registered | Preflight registration, negotiated tool inventory and an answered scripted call agree with the profile in that cell's environment. No agent call is required. | Reuse registration check. Add preflight negotiation and call-result evidence. |
| W2 Primed at session start | Preflight proves startup delivery of the expected service, subject revision and profile primer digest before agent actions. | Reuse sessionhook and prime. Join delivered context, not merely a process exit. |
| W3 Every declared hook observed firing | Before agent actions, a scripted eligible trigger for each declared hook yields its named event and correlated beat. | Reuse harness descriptors, readiness and WR-12. Missing supported hooks fail preflight. Unsupported hooks must be declared absent. |
| W4 Use at eligible moments | During agent actions, per-capability eligible opportunities and successful tool calls form an outcome rate. Report should-not calls separately. | Reuse LAB-9, LAB-11 and BEN-4. Add validated adapters beyond the current transcript boundary. Never exclude low-use cells. |
| W5 Acted on | During agent actions, a cited file, symbol or test is touched within the next k calls after delivery. This is an outcome only. | Reuse BEN-4 targets and LAB-11. Require a session join, stable target and explicit action event. Never exclude non-action cells. |
| W6 Fresh and inside budget | The fixed preflight workload proves index revision/age, payload age, hook p95 and resource readings within the declared bounds before agent actions. | Reuse resources and tenant readings. Correlate the scripted window and separate later answer/resource diagnostics from admission. |

W0 is the product revision under test, not whatever the live fleet happens to serve. A baseline without fathom declares product installation and surface rungs inapplicable, then proves its own harness version, permitted tools, startup instructions, absence of treatment hooks, action capture and host budget. It never fabricates a daemon identity or a wire stamp.

W4 reports each capability's rate and denominator, with no pooled rate hiding non-use. A zero eligible count is unmeasured, while zero calls with eligible opportunities is measured non-use. Neither excludes a cell. Classifier validation follows LAB-M1a before rates steer choices. W5 uses k = 3 as the proposed starting window, matching LAB-11's existing next-three-calls rule. It is a profile field fixed before collection, not tuned after seeing actions. Matching a target proves action proximity, never causation or task success. Compare it with the held-out delivery base rate (BN-D2).

W6 binds a fixed preflight window to the tenant policy and budget digest. It does not copy a rolling fleet PASS into a short probe. The resource card in `feature-tuning.md#FT-D4` supplies the measures, while sustained tenant gates retain their own windows. Missing preflight samples yield UNKNOWN. Answer-time freshness and resources during agent actions remain reported diagnostics or registered outcomes, never retroactive reasons to void a preflight-valid pair. Age and revision mismatch are separate: a recent ingest may still contradict HEAD.

### WP-D2 Preflight gates validity, while use remains an outcome

Preserve correction 1 (`docs/design/bench-prep.md:371`) and BX-40 (`docs/plans/bench-prep.md#BX-40`). Every registration declares environment requirements over W0-W3 and W6, their applicability mask, hook inventory, fixed probe workload, freshness bound and resource envelope. Eligible-use bars and k belong to the delivery-outcome record. The driver completes both arms' scripted preflights before either agent acts, binding each receipt to its cell environment and profile digest. The validity function accepts preflight inputs only and cannot read agent calls, W4 or W5. A failed or unknown required environment check makes the whole pair VOID with reason `instrument_defect` before agent execution.

Reuse WR-1 and WR-2 for receipt admission and broken positive controls. Extend their receipt with preflight evidence rather than introducing another validity gate. A treatment with a valid preflight and no agent surface call keeps its actual assigned-arm outcome, including task failures and all spent tokens. Low W4 or W5 is a finding against the profile, not a void, exclusion or automatic task failure. Report non-use in the delivery funnel and zero-call line. The headline estimate always includes every admitted cell under its assigned arm. A use-conditioned estimate is secondary only, labelled "biased toward the product: selects on tool use", and supports neither a headline nor a ruling. "Surface registered and called" means the preflight scripted call answered in that cell's environment. Historical evidence of registration and reachability can substantiate W1 for old readings, without requiring agent use or inventing missing preflight records.

Resolve ADR-105 decision 9 narrowly: daemon-free arms still need no daemon stamp or RT-M15, but must prove their applicable baseline environment checks and BEN-2 readiness. No spend authorization changes. Native Windows support is outside this design. WSL is a Linux guest on a Windows host, with that environment recorded explicitly.

### WP-D3 The wiring profile is data

A profile references the labs card and existing knob map. Fields cover default or extended surface, stanza text, tool descriptions, hook set, skill digests, prime contents, map injection and in-band anchors. Include harness and model pins, product and subject commits, source digests, channel applicability, budgets and expected hook inventory. Hash canonical effective data, not a profile name or an unsalted local path.

Extend LAB-28 and LAB-29 for description and stanza overrides. Reuse LAB-36, BX-28 and TUN-1 for the knob record, readouts for in-band delivery, and BN-D8 for selection. WIR-2 owns claim-gate parity, WIR-3 owns managed Linux installation and WIR-5 owns the published harness matrix. A profile may declare extended maps without enlarging the default surface (`docs/plans/exploration-maps.md#XM-10`). No production override is implied.

### WP-D4 Delivery experiments precede paid outcomes

Use recorded fixtures and forward logs at $0. Per profile and capability, report eligible, fired, delivered within deadline and acted on, plus missing inputs, false fires and void cells. Historical replay is labelled replay and cannot select a live knob before LAB-M2a validates it. No new model session is needed to validate deterministic event joins.

Freeze the criterion, splits and stop rule before collecting the forward window. Rank development profiles by acted-on lift over held-out delivery, subject to fact precision and preflight W6 bounds. Keep all preflight-valid cells in their assigned arms, including zero-call and zero-action cells. Report W4 and W5 with their denominators and the non-use share, so profile ranking cannot condition on successful use. Break ties by lower delivered tokens, then canonical profile id. Select at most two profiles. A null or incomplete reading selects none. Paid outcomes wait on BN-D12, RT-M15 where applicable, a committed preregistration and a separately approved spend row. This specification performs no paid run.

Firing is a diagnostic, never a selection objective. LAB-32 and BX-29 retain split ownership. TUN-3 retains release confirmation. Profile selection does not turn any capability on by default, which remains TUN-2's outcome-proof gate.

### WP-D5 Scripted proof for every harness kind

Extend BEN-2's scripted wired and scripted grep controls. Have separate adapters for Claude Code, Codex CLI, cloud sessions keyed by actual harness, and Codex CLI inside WSL. Cloud execution is an environment dimension, not a fictional shared transcript format. Each adapter replays a pinned transcript contract and also records one operator-run scripted local session without a model call.

Before agent actions, the fixed preflight script negotiates the surface, primes, exercises every declared hook with a seeded target and checks an answered surface probe plus freshness and resource bounds. Its probe calls and fixture actions are labelled preflight and excluded from agent W4/W5 counts. It seeds absent registration, stale binary, missing primer, silent hook and stale index faults from WR-3. Every injected environment fault must be detected, while the healthy twin passes its applicable environment checks. A separate healthy-preflight fixture with zero agent calls and no target action must remain admitted. Wrong agent targets affect W5 only. A script lacking a real observation channel reports UNKNOWN rather than writing synthetic proof of installation.

Codex/cloud adapters are extensions to LAB-7, not a second decoder. This proposes a bounded amendment to ADR-029 decision 6 and LAB-D7 for deterministic conformance and delivery accounting only. Paid fork or stanza arms remain under their existing entry conditions. WSL proof records guest revision, host class and path translation using relative fixture paths. A Linux CI run does not claim WSL proof.

### WP-D6 One evidence trail and one report

Each rung retains source kind, session and call ids, evidence range, observed time, product and subject commits, profile digest, harness pin and reason. Label preflight and agent phases explicitly, with the preflight decision time preceding the first agent action. An unchanged environment/profile digest binds the probe to agent startup. Environment readiness does not wait for W4/W5 observations. Sort evidence and inject the clock. Do not retain transcript text in aggregate ledgers. Raw artifacts stay in local `workspace/` and public fixtures contain synthetic or public data only.

Extend doctor and `instrument --adoption` through the current report paths. A report distinguishes configured, observed, eligible, acted on and unknown. No new daemon table, periodic sampler, product command runner or dashboard is required. WR-14 and WR-15 keep their existing storage conditions. A missing historical stamp stays wiring-unknown, never backfilled.

## 3. Risks

- WP-R1 Conditioning admission on use would select favourable cells. Accept only pre-agent environment evidence, preserve correction 1/BX-40 and test that changing W4/W5 with recorded task results held fixed never changes admission or assigned-arm totals.
- WP-R2 Transcript adapters can silently drop events. Include malformed, absent and duplicate events as negatives, expose coverage and use WR-3 fault labels as independent construction truth.
- WP-R3 Hook absence can look healthy on a limited harness. Freeze the applicability mask and require negative evidence for channels declared off.
- WP-R4 Action proximity can reward irrelevant edits. Match exact anchored targets and retain oracle correctness and the held-out base rate.
- WP-R5 Cloud or WSL evidence may be unavailable. Keep that matrix cell UNKNOWN and make no parity claim.

## 4. Sequence

1. Land the conformance schema and receipt extension. Reuse WR-4, WR-5, WR-12 and WIR-1 rather than implementing them twice.
2. Add transcript joins and harness adapters. Run the seeded fault controls at $0 through BEN-2.
3. Bind cell admission to the preflight evaluator and doctor/adoption to separate environment and outcome fields. File the per-harness conformance reading without excluding non-use.
4. Run the forward delivery profile reading, then nominate at most two profiles for independently authorized outcomes.

Each plan row owns named files and one bounded contract. Shared registry or command edits are L-S work under the architect. Nothing in this sequence claims the existing dependency rows are proven.

## 5. Decision models

No new judged site applies. Rungs, applicability, target matching and cell validity are deterministic checks over pinned evidence. Provider `none` produces the same verdicts and reports. There is no calibration band because no model verdict is used. Opt-out means disabling the optional experiment profile, while keeping its measured absence. A model suggestion cannot fill an unknown event or promote a rung. The proposed `judge_wiring` residual stays on its existing deferred row (`docs/plans/wiring-receipts.md#WR-17`).
