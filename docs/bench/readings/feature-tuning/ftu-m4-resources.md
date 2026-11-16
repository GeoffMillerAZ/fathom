# FTU-M4: bounded historical resource reading

This reading extracts two historical RT-M1 windows and discloses one supplied,
file-backed resource diagnostic under
`docs/contracts/track-free-readings.md` (approved 2026-11-15 by W2-3 after
independent LAND review). It provides partial historical and diagnostic evidence;
current resource evidence remains incomplete. The historical availability failures
remain dated FAILs. No current conformance, causal benefit, default/knob change or plan completion is claimed.

## Provenance and method

All five committed inputs below were resolved with `git show` at source revision
`83226aece9b0f421381373f8dafc4a912d3888e5` and checked against their complete-file
SHA-256 fingerprints. Current branch bytes were not substituted.

| Committed input | SHA-256 |
|---|---|
| `docs/bench/readings/tenancy/rt-m1-2026-10-19.md` | `ad4b7c0e27f33d9099cac384d4eb50cb6061934c88d33ae7158405093ed80340` |
| `docs/bench/readings/tenancy/rt-m1-2026-10-26.md` | `5aea6f4ac1144e9df0b65ed02d38b220140b278e677145fa72f3a9db26153d52` |
| `docs/plans/responsible-tenant.md` | `fb441f759f7a4f961341c41b97abfa1e4df6a253895f4b9234a42dad820470c2` |
| `docs/plans/feature-tuning.md` | `9a6a0df04259c990f48403695550a2c79759d91526983ad8edbb6c27dd6dbaa4` |
| `docs/design/feature-tuning.md` | `c278e637611f5370a56e21a33c3855f5ae48a6c06a84c241b255afd16a4cc266` |

The historical sources are aggregate, authored readings, not raw day logs or independent receipts.
Only each source's table under `## The seven days` is extracted, matching columns
by header and expanding month/day to 2026. Day order, original gauge text/units,
all count/minute columns and numeric emphasis are retained. Markdown emphasis is
stripped only for integer arithmetic. No missing observation is interpolated.

Authority: FTU-M4, FT-D4 and proposed ADR-110 decisions 1/2/8 preserve original
tenant windows/verdicts and unknown CPU attribution; WP-D6 governs provenance.
I7/I38/I53/I62 and P2/P3/P7 require disclosed freshness, deterministic accounting,
unknowns kept visible and no judgment substituted for telemetry. This extraction
establishes no new trigger. F50/F64/F65 remain unpromoted; the no-dispatch non-goal
remains binding.

## Window 1: extracted daily rows

Source: `docs/bench/readings/tenancy/rt-m1-2026-10-19.md` at the pinned revision.
Window: `[2026-10-12T00:00Z,2026-10-19T00:00Z)`; source read time:
2026-10-19 00:13Z, using `fathom resources --days 8 --json` on the then-deployed
binary identified in the source as `8cfb6c27`.

| Day | Peak footprint | Shed min | Refuse min | Self-caused | Host-starved | Exit rungs | Unclean exits |
|---|---|---|---|---|---|---|---|
| 2026-10-12 | 71 MB | 234 | 0 | 0 | 0 | 0 | 0 |
| 2026-10-13 | 71 MB | 728 | 216 | 0 | **216** | 0 | 0 |
| 2026-10-14 | 70 MB | 270 | 136 | 0 | **136** | 0 | 0 |
| 2026-10-15 | 2.00 GB | 678 | 547 | 0 | **547** | 0 | 0 |
| 2026-10-16 | 3.40 GB | 502 | 0 | 0 | 0 | 0 | 0 |
| 2026-10-17 | 3.47 GB | 297 | 0 | 0 | 0 | 0 | 0 |
| 2026-10-18 | 3.39 GB | 285 | 0 | 0 | 0 | 0 | 0 |

Source coverage: full day logs reported as 1,433..1,440/day; the source says the
reading is not INCOMPLETE. Individual daily sample counts are unavailable beyond
that reported range; seven calendar rows do not independently prove sample completeness.

Window 1 RT-M1 verdict: FAIL (read 2026-10-19).
Original availability failures: 2026-10-13 (216 minutes), 2026-10-14 (136),
2026-10-15 (547), each against host-starved refuse ≤60 min/day. The source's
verdict names 2026-10-13 first; all three failing days remain visible here.

## Window 2: extracted daily rows

Source: `docs/bench/readings/tenancy/rt-m1-2026-10-26.md` at the pinned revision.
Window: `[2026-10-19T00:00Z,2026-10-26T00:00Z)`; source read time:
2026-10-26 00:03Z, using `fathom resources --days 8 --json` on the then-deployed
daemon. This source does not supply a deployed binary revision for that read.

| Day | Peak combined | Shed min | Refuse min | Self-caused | Host-starved | Floor min | Critical min | Exit rungs | Unclean exits |
|---|---|---|---|---|---|---|---|---|---|
| 2026-10-19 | 2.18 GB | 28 | 58 | 0 | 58 | 58 | 0 | 0 | 0 |
| 2026-10-20 | 3.00 GB | 4 | 0 | 0 | 0 | 0 | 0 | 0 | 0 |
| 2026-10-21 | 1.69 GB | 6 | 193 | 0 | **193** | 193 | 0 | 0 | 0 |
| 2026-10-22 | 3.42 GB | 43 | 140 | 0 | **140** | 78 | 62 | 0 | 0 |
| 2026-10-23 | 3.23 GB | 51 | 179 | 0 | **179** | 178 | 1 | 0 | 0 |
| 2026-10-24 | 3.08 GB | 33 | 265 | 0 | **265** | 262 | 3 | 0 | 0 |
| 2026-10-25 | 3.02 GB | 142 | 0 | 0 | 0 | 0 | 0 | 0 | 0 |

Source coverage: 648 on 2026-10-19 (daemon redeployed that day),
1,440..1,455 on other days. The source says the reading is not INCOMPLETE;
this preserves its reported coverage and does not turn 648 into a full day's
samples or assign exact counts to other dates.

Window 2 RT-M1 verdict: FAIL (read 2026-10-26).
Original availability failures: 2026-10-21 (193 minutes), 2026-10-22 (140),
2026-10-23 (179), 2026-10-24 (265), each against host-starved refuse ≤60 min/day.
The source's verdict names 2026-10-21 first; all four failing days remain visible.

## Recomputed integer accounting

Every numeric minute/count cell is a nonnegative integer. For all 14 rows,
self-caused + host-starved = refuse. For all seven second-window rows,
floor + critical = host-starved. These are checks of the authored table cells,
not independent checks of raw trigger attribution.

- Window 1 Shed min sum: 2994
- Window 1 Refuse min sum: 899
- Window 1 Self-caused sum: 0
- Window 1 Host-starved sum: 899
- Window 1 Exit rungs sum: 0
- Window 1 Unclean exits sum: 0
- Window 1 shed share: 2994/10,080 = 29.702381%
- Window 1 Self-caused >15 min/day: none
- Window 1 Host-starved >60 min/day: 2026-10-13, 2026-10-14, 2026-10-15

- Window 2 Shed min sum: 307
- Window 2 Refuse min sum: 835
- Window 2 Self-caused sum: 0
- Window 2 Host-starved sum: 835
- Window 2 Floor min sum: 769
- Window 2 Critical min sum: 66
- Window 2 Exit rungs sum: 0
- Window 2 Unclean exits sum: 0
- Window 2 shed share: 307/10,080 = 3.045635%
- Window 2 Self-caused >15 min/day: none
- Window 2 Host-starved >60 min/day: 2026-10-21, 2026-10-22, 2026-10-23, 2026-10-24

Each share divides that window's shed minutes by its 10,080 calendar minutes;
it is not a sample-weighted estimate or a new resource verdict. No weeks are pooled
to hide daily failures. No floor/critical split was supplied for window 1.

## Unchanged bars and source assertions

RT-M1 retains footprint ≤ exit mark at every sample and ≤ budget outside refuse;
self-caused refuse ≤15 min/day; zero exit rungs and zero unclean exits; separately,
host-starved refuse ≤60 min/day. Both source windows report marks of
budget 4 GiB, shed 3 GiB, refuse 4 GiB, exit 8 GiB, with exit enabled.
The amended clause is dated 2026-10-11; these dated results do not reopen it.

Source-reported tenant compliance: both readings assert that footprint remained
below the exit mark at every sample and below budget outside refuse, with no
self-caused refuse, exit rung or unclean exit. Historical assertion, not sample revalidation:
the integer zero columns can be reproduced here, but daily peaks cannot establish
outside-refuse budget attribution. “Peak footprint” and “Peak combined” are
not assumed to be equal gauges. Rounded MB/GB peaks are not converted to exact GiB
or used to infer exact samples. Linux/WSL resident gauges are unavailable here;
these historical values cannot be extrapolated to them.

The first source reports shed on host pressure `warn`, refuse on `critical`, and
refuse below 5 GiB swap-volume free. It attributes its three availability failures
to the swap-volume floor. The second reports shed on `warn` only at or above its
1.8 GiB allowance (RT-192), with the same critical/floor refuse triggers. Thus the
two historical windows do not form a controlled comparison of equal behavior.

The second-window floor trigger retains its original disposition: the source says
the second FAIL fires the preregistered change to refuse on the floor only at
pressure `warn` or above. It reports 39 floor entries, 9 at normal and 30 at warn,
and says removing the normal-pressure entries would not save RT-M1's window.
This is the source's dated trigger and disposition, not a new retuning or causal
result. Its RT-M13 discussion is not the missing RT-M4/M8/M15 evidence set.

## Supplied diagnostic, separate from the historical windows

DiagnosticInput is the supplied stdout of `fathom resources --days 8 --json`:
3,762 bytes, exit 2, empty stderr, observed at the JSON's
`generated_at` time **2026-11-16T00:15:25Z**. SHA-256 of the exact private stdout:
`efd99d7443734a2b08f24489e968041bdcd9142595d7fe603452f840f0dbd1ad`.
The supplied companion identity is
`fathom v0.26.0 @ 16af6ca72515 [2026-10-29T05:26:44Z]`.
This binary identity is distinct from the pinned source revision; neither the
contract date nor file modification time replaces the observation timestamp.
The diagnostic was inspected without rerunning it; raw bytes remain private.

Sanitized observations: schema 1; daemon unreachable; current footprint unknown (`-1`);
zero resource-ring samples; no actual day rows.
RT-M1: INCOMPLETE; RT-M13: INCOMPLETE. Both request 2026-11-09..2026-11-15
and mark every requested date “no row”. These date requests are not observed day evidence.
The top-level `days` and `unreadable` arrays are omitted;
zero unreadable entries are reported; this does not establish complete coverage.
The exit reason is absence of a sample to judge, not a measured budget breach.
Other zero-valued diagnostic summaries do not establish measured compliance.
This provides unknown-coverage evidence and no current resource PASS/FAIL.

## Unknown components and non-use boundary

- Raw sample revalidation: unknown; historical day logs and sample/window joins are unavailable here.
- Current WPF profile/receipt: unknown; no environment-bound W0-W3/W6 proof is supplied.
- Current footprint: unknown; the supplied `-1` and zero samples are not a zero-footprint observation.
- Correlated CPU seconds per completed ingest: unknown; no per-job attribution is supplied.
- Matched ingest size/stage: unknown; no matched job inputs or graph stages are supplied.
- Hook p95 per harness/channel: unknown; no channel-specific latency observations are supplied.
- RT-M4/M8/M15 coverage: unknown; their complete clauses and original windows remain required by the pinned tenant plan.

Process-cumulative CPU is not divided by an invented job count.
No percentile is reconstructed from a percentile summary.
RT-M14 latencies are not current WPF preflight or RT-M15 readiness.
FT-D4's hook deadline remains descriptor timeout minus max(1 second, 20 percent),
with no retry after timeout; nothing here measures that compliance.

No arm estimates or arm denominator are derivable from these two historical tables.
If arm data later exists, retain all admitted cells under assigned arms, including zero calls/actions;
healthy-preflight non-use remains an outcome and cannot exclude a cell.
Replay stays separate from forward data. Any use-conditioned secondary must be labelled
“biased toward the product: selects on tool use” and supports no headline or ruling.

## Proof and disposition

The local standard-library-only checker validates InputFingerprint (including
DiagnosticInput), TableExtraction, WindowCoverage, DailyAccounting,
HistoricalVerdictPreserved, UnknownComponents and NoSelectionOrConformanceClaim
against pinned source bytes, the supplied diagnostic and this report. Both extracted
tables were inspected separately against their sources. These seven consistency
checks are not new independent labels, runtime proof or current conformance.

Operational verify was not run in this daemon-free lane; diagnostic exit 2 proves
no current tenant compliance and no row completion is proved. No plan box changes.
FTU-M4's full proof still requires complete tenant/source joins and prerequisites,
the resource command and `fathom verify --service fathom --plan docs/plans/feature-tuning.md`.
The supplied command execution does not fill the missing WPF receipt, attribution
or sustained RT-M4/M8/M15 evidence. Any future current PASS/FAIL requires a newly
reviewed input contract. This report preserves both historical FAILs and leaves
the incomplete current resource card unresolved.

FTU-M4 verdict: INCONCLUSIVE
