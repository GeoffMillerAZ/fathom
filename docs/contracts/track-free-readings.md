# Free reading contract: FTU-M4 historical resource component

Status: approved by rule W2-3 after independent LAND review, 2026-11-15. No reading report has
run under this contract; the supplied diagnostic below has run.
Source: fetched `origin/main` = `83226aece9b0f421381373f8dafc4a912d3888e5`;
historical extraction pins this source revision. Recheck fingerprints before execution.

## Authority and boundary

One reading only: `docs/plans/feature-tuning.md`, **FTU-M4**, output exactly
`docs/bench/readings/feature-tuning/ftu-m4-resources.md` (new). A later reading
lane owns only that tracked file; scratch inputs/checks belong under `workspace/tmp/`.
This contract author owns only this contract. It writes no report or implementation.

The runnable component is extraction and arithmetic over committed historical RT-M1
readings plus disclosure of the supplied file-backed diagnostic below. FTU-M4
explicitly requires reuse of tenant verdicts with their original windows; FT-D4 explicitly permits an initial INCONCLUSIVE resource reading with
unknown CPU attribution. This is substantive historical evidence, not a substitute
seven-day experiment. Missing WPF/FTU prerequisites prohibit row completion.
Authority: `docs/plans/feature-tuning.md` FTU-M4 and completion rule;
`docs/design/feature-tuning.md` FT-D4; `ADR.md` ADR-110 decisions 1, 2 and 8
(proposed, architect-directed specification).

I7 requires disclosed freshness, I38 forbids unknown coverage looking healthier,
I53 bounds hooks, and I62 forbids judgments as proof. P2/P3/P7 require deterministic
arithmetic, absent metrics rendered absent,
and source existence checks (`docs/intent/invariants.md`, `principles.md`).
No new trigger is established by an extraction. The historical RT-M1 failures and
the second window's recorded floor trigger retain their original disposition;
this contract neither retunes them nor reopens their bars. F50/F64/F65 remain
unpromoted by this work (FT-D2); the no-dispatch non-goal remains binding
(`docs/intent/scope/non-goals.md`). No product or compatibility change is proposed.

## Grounded availability at the source revision

| Rows inspected | Available component and missing inputs | Disposition here |
|---|---|---|
| WPF-M1 | Doctor/readiness code and historical observations exist, but `schema/wiring-proof/schema.cue`, `internal/benchready/conformance_{claude,codex}.go` and `docs/bench/readings/wiring-proof/` are absent. No environment-bound WPF receipt supplied. | Excluded: fixtures/configuration cannot prove actual W0-W3/W6. WP-D1/D5 and WPF-M1. |
| WPF-M2 | `internal/labs/{conformance,wiring_profile}.go` absent; no supplied BEN-4 forward log with validated eligibility, arm/split and exact target joins. | Excluded: no profile ranking from missing evidence. WP-D4 and WPF-M2. |
| FTU-M1 | `internal/labs/map_metrics.go`, `cmd/fathom/map_orientation.go`, frozen tuning labels and preregistration absent. | Excluded: no independent orientation/edit oracle. FT-D2, FTU-2/3/4 and FTU-M1. |
| FTU-M2 | Mock responses/scope checks in `internal/semantic/compile_test.go` and sweep checks exist. Frozen independent FTU labels and operator-timed matched-coverage authoring exercise not supplied. | Excluded: test assertions are not independent precision labels or cost measurements. FT-D3 and FTU-M2. |
| FTU-M3 | `internal/labs/freshness_reading.go` and preregistered latency bound absent; no supplied answer-time/HEAD/admitted/completed join. | Excluded: no invented bound or stale-answer denominator. FT-D4 and FTU-M3. |
| FTU-M4 | Two committed RT-M1 daily tables with dated FAIL verdicts and the supplied exit-2 file-backed diagnostic are available below. Correlated per-ingest CPU, current WPF receipts and complete RT-M4/M8/M15 evidence not supplied. | Selected historical component plus diagnostic disclosure only. FT-D4 and FTU-M4; I38/P3/P7. |
| FTU-M5 | No FTU frozen labels or supplied forward holdout/eligibility/target joins. Existing scope tests cannot establish forward lift. | Excluded: replay cannot keep a knob. FT-D5 and FTU-M5. |
| FTU-M6 | No FTU-M1..M5 report directory or WPF-M2 selection at this revision. | Excluded: not a runnable five-card disposition. FTU-M6. |

`calibration/labs.d/tuning/`, `internal/labs/testdata/feature-tuning/` and
`docs/bench/prereg/feature-tuning.md` are absent on this revision. Absence here
means checked committed paths or missing supplied evidence, not a claim about
other checkouts or private logs. Local log metadata alone supplies no receipt.
`rt-m13-2026-11.md` is a separate historical RT-M13 reading, not the required
RT-M1/4/8/15 evidence set; `rt-m14-2026-10-19.md` cannot become RT-M15.
Authority: FT-D4, each named plan row and WP-D6's provenance requirements.
A runnable FTU-M4 component exists, so the fallback search of unrelated plans is
unnecessary. No ENV or other track's active reading file is assigned.

## Immutable inputs

SHA-256 below covers the complete committed file bytes, not rendered Markdown.
Resolve every path at the source revision above with `git show <revision>:<path>`.

| Input | SHA-256 |
|---|---|
| `docs/bench/readings/tenancy/rt-m1-2026-10-19.md` | `ad4b7c0e27f33d9099cac384d4eb50cb6061934c88d33ae7158405093ed80340` |
| `docs/bench/readings/tenancy/rt-m1-2026-10-26.md` | `5aea6f4ac1144e9df0b65ed02d38b220140b278e677145fa72f3a9db26153d52` |
| `docs/plans/responsible-tenant.md` | `fb441f759f7a4f961341c41b97abfa1e4df6a253895f4b9234a42dad820470c2` |
| `docs/plans/feature-tuning.md` | `9a6a0df04259c990f48403695550a2c79759d91526983ad8edbb6c27dd6dbaa4` |
| `docs/design/feature-tuning.md` | `c278e637611f5370a56e21a33c3855f5ae48a6c06a84c241b255afd16a4cc266` |

The historical sources are aggregate, authored readings, not raw day logs or
independent receipts. Their unit labels, gauge definitions and reported coverage
are evidence limits. No raw private transcript, credential or external oracle is needed.
WP-D6 and FT-D4 govern this fingerprint/provenance boundary.

## Supplied diagnostic input (DiagnosticInput)

Keep this capture separate from the five committed inputs and historical windows.
The controller supplied the exact stdout bytes from `fathom resources --days 8 --json`,
with exit **2** and empty stderr; this lane inspected those bytes without rerunning
resources. Observation time is the JSON's `generated_at`: **2026-11-16T00:15:25Z**,
not the contract date or a filesystem timestamp. Supplied companion binary identity:
`fathom v0.26.0 @ 16af6ca72515 [2026-10-29T05:26:44Z]`.
That identity is supplied provenance, not a claim that the binary equals the pinned
source revision. Retain the 3,762 raw stdout bytes privately; SHA-256:
`efd99d7443734a2b08f24489e968041bdcd9142595d7fe603452f840f0dbd1ad`.

Publish only these sanitized diagnostic facts with the command, exit, timestamp,
identity and hash: schema 1; daemon unreachable; current footprint unknown (`-1`);
zero resource-ring samples; no actual day rows; RT-M1 and RT-M13 both `INCOMPLETE`.
Both verdicts request 2026-11-09..15 and explicitly mark every date “no row”;
these day-name requests are not observed day evidence. The top-level `days` and
`unreadable` arrays are omitted (both have `omitempty` in `resourcesReport`):
zero unreadable entries are reported, which does not establish complete coverage.
The exit reason is absence of a sample to judge, not a measured budget breach.
Do not turn other zero-valued diagnostic summaries into measured compliance.

`cmd/fathom/resources.go` implements file-backed reporting even without a responding
daemon; `resourcesVerdict` returns 2 for no sample/read errors, `runResources` still
emits JSON, and `attachDays` distinguishes missing rows from verdict date requests.
Under FTU-M4/FT-D4 and I38/P3/P7 this executed diagnostic is available evidence of
unknown coverage, with no current resource PASS/FAIL. It supplies neither CPU/job
attribution nor hook p95 per harness/channel nor a current WPF receipt. Raw private
identifiers, URLs and local paths must not enter the report.

## Exact method and unchanged bars

1. Validate **InputFingerprint**: both historical files and all governing files above
   exist at the source revision and match SHA-256. Stop on drift; do not silently
   substitute newer inputs. Also validate **DiagnosticInput**: hash the supplied raw
   capture, parse its fields, and reproduce the sanitized facts and provenance above.
   Missing or mismatched capture stops publication; do not substitute a repeat run.
   Confirm the output path is new before dispatch.
2. Validate **TableExtraction**: for each source, parse only the table under `## The seven days`, using its header
   to identify columns. Preserve day order, footprint text/unit, shed, refuse,
   self-caused, host-starved, exit-rung and unclean-exit columns; retain second-window
   floor/critical columns separately. Strip Markdown emphasis only for numeric parsing.
   Expand month/day with year 2026 from the source window. No interpolation.
3. Validate **WindowCoverage**: exactly seven unique consecutive UTC dates per table:
   2026-10-12..18 and 2026-10-19..25. Preserve the enclosing windows
   `[2026-10-12T00:00Z,2026-10-19T00:00Z)` and
   `[2026-10-19T00:00Z,2026-10-26T00:00Z)`, source read times and source verdicts.
   Carry reported sample coverage as reported: first 1,433..1,440/day; second 648 on
   10-19 and 1,440..1,455 on other days. Do not manufacture individual sample counts.
4. Validate **DailyAccounting**: numeric minute/count fields are nonnegative integers;
   self-caused + host-starved = refuse on every row; second-window floor + critical
   = host-starved on every row. Publish per-window sums of those minute columns,
   and the dates with self-caused >15 or host-starved >60 minutes. Divide summed shed
   minutes by 10,080 only to describe that window; never pool weeks to hide daily failures.
5. Validate **HistoricalVerdictPreserved**: both original `RT-M1 verdict: FAIL`
   results remain FAIL with their original dates. Mark the >60-minute days against
   the original availability clause. Do not recompute sampled footprint compliance
   from daily peaks alone: budget outside refuse requires the original sample/window
   attribution. Keep first-table “Peak footprint” and second-table “Peak combined”
   labels separate; do not assume equal gauges, convert rounded GB into exact GiB,
   or extrapolate to Linux/WSL resident gauges. Quote source-reported tenant compliance
   as a historical assertion, separately from recomputed integer accounting.
6. Validate **UnknownComponents**: explicitly list raw sample revalidation, current
   WPF profile/receipt, current footprint, correlated CPU seconds per completed ingest,
   matched ingest size/stage, hook p95 per harness/channel, and RT-M4/M8/M15 coverage
   as unknown/unavailable here. Disclose DiagnosticInput separately, including its
   exit-2 unknown coverage and INCOMPLETE verdicts; date requests are not day rows
   and zero unreadable entries do not cure missing samples. No process-cumulative CPU
   divided by an invented job count; no percentile reconstructed from a percentile summary. Existing RT-M14
   latencies are not current WPF preflight or RT-M15 readiness.
7. Validate **NoSelectionOrConformanceClaim**: no current conformance, causal benefit,
   default/knob change or row completion claim. If arm data later exists, retain all
   admitted cells including zero calls/actions; replay stays separate from forward
   data and any use-conditioned secondary is labelled “biased toward the product:
   selects on tool use” and supports no headline or ruling. No arm estimates are
   derivable from these two historical tables.

Steps 1..3 implement WP-D6 and FT-D4 provenance/windows, with DiagnosticInput
disclosure under FTU-M4 and I38/P3/P7. Steps 4..5 use the unchanged
RT-M1 row: footprint ≤ exit mark at every sample and ≤ budget outside refuse;
self-caused refuse ≤15 min/day; zero exit rungs and unclean exits; separately,
host-starved refuse ≤60 min/day. FTU-M4's full bar remains every applicable tenant
clause and registered budget with complete attribution; RT-M4, RT-M8 and RT-M15
retain every clause/window in the pinned responsible-tenant plan. Steps 6..7 follow
FT-D4, FTU-M4, WP-D2 and ADR-110 decisions 2/8. No new effect size, interval,
resource bound or prerequisite waiver is registered by this contract.

## Verdict law and proof

The report must publish actual extracted rows and arithmetic, source fingerprints,
component verdicts and unknowns, plus the supplied DiagnosticInput facts and
provenance. It must contain exactly one terminal line:
`FTU-M4 verdict: INCONCLUSIVE` for this bounded input set. This is the verdict on
incomplete current resource evidence, not a conversion of either historical FAIL.
A malformed source or failed accounting check stops publication pending correction;
it is not a resource failure. A later, fully evidenced current bar breach would be
FAIL and complete current evidence satisfying every bar could be PASS, but either
requires a newly reviewed input contract. This distinction follows FTU-M4/FT-D4
and P3; lexical header matching alone proves neither conformance nor row completion.

Reading-lane commands (source the controller-provided lane environment each shell):

- `git cat-file -e <revision>:<input>` and
  `git show <revision>:<input> | shasum -a 256` for each immutable input above.
- `python3 -I workspace/tmp/ftu-m4-check.py`: a local standard-library-only checker
  implementing the seven named checks above against pinned source bytes, the exact
  supplied DiagnosticInput bytes and the report (DiagnosticInput is part of
  InputFingerprint and UnknownComponents); emit one PASS/FAIL line per check, exit
  nonzero on any failed assertion.
  The reading lane writes this checker locally before extraction; no new tracked
  script or dependency. Inspect extracted cells against both source tables separately.
- `git diff --check` and path-scope review: the only tracked change in that reading
  lane is the exact report path above. Public output contains aggregate numbers,
  relative source paths and revisions only, no private identity or absolute path.
- FTU-M4's full proof remains `fathom resources --days 8 --json`, followed by
  `fathom verify --service fathom --plan docs/plans/feature-tuning.md` and tenant/source
  join review. The resources diagnostic has already executed as supplied above;
  validate and disclose it, with no repeat run needed. Operational verify remains
  unavailable in this daemon-free lane; no verification or row completion is proved
  by diagnostic exit 2. Do not contact/install/restart a daemon to fill the gaps.
  No plan box is flipped.

No package test is required for historical Markdown extraction. If a later authorized
package change needs tests, run only its named package with `-race` under
`bash scripts/gate-lease.sh go test -race <owned-package> -run '<named-tests>'`.
Never run the full gate here. The implementing lane owns real failing tests if
implementation is separately authorized (W3-2); this contract author writes none.
The arithmetic checker is evidence consistency checking, not a new independent label.
No paid calls, live-agent benchmark subjects, installs, config edits, push or deploy.

## Decision models

None applies: extraction, accounting and unknown handling are deterministic with
provider `none`; no judgment supplies missing telemetry (FT-D4; ADR-110).
