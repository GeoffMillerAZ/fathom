# RT-M4 complete request-cost input contract

Draft for gap #285. Grounding base d9eef4794afdbc200d1b8697ce121d9d688fedaf.
L-R ownership is only docs/bench/readings/tenancy/rt-m4-2026-11.md, with raw
artifacts/checker under workspace/. No implementation or live measurement grant.
RT-M0 is the first resource entry draft; this separate protocol adds profile and
ingest comparisons that a shared resource-window report cannot establish.

## Decisions

| ID | Recommended choice / governing citation | Alternative |
|---|---|---|
| D1 | Require all six RT-M4 clauses independently, AND for PASS. docs/plans/responsible-tenant.md RT-M4, vision 36 §6, ADR-047 d5/6 and amended worker trigger. | Treat the latency table as full RT-M4 proof. Rejected: it omits allocation and ingest comparisons. |
| D2 | Same-window request allocation and complete completed-call denominator, historical and current. RT-M4 explicitly normalizes the heap-0518 730.7GB numerator by that window's count. | Use 17,589 obligations calls as historical total. Rejected: vision36 §1 labels it one tool only. |
| D3 | p50 and p99 from raw obligations latency, nearest rank ceil(q*n), disclose all outcomes. I38, existing hookP99s precedent in cmd/fathom/doctor_retention.go. | Substitute metrics traffic p95 or drop failures. Rejected: changes the bar/population. |
| D4 | Existing instrument shapes, no new public types, context or command; missing attribution is INCOMPLETE. I38, parallel-lanes L-R, W3-2/4. | Reconstruct missing profiles from prose totals. Rejected. |

No applicable future requires prebuilding instrumentation. ADR-047's RT-D6 trigger
is evaluated only by actual ingest/retention data, not this contract. No new trigger
is established here. CTR-12 (91860745...) delivery ports do not cover this reading.
W4-2 REC-M3 median does not apply.

## Current code/input grounding and callers

* internal/mcp/calllog.go callLogEntry v3 logs ts at completion, tool, service,
  origin, outcome, latency_ms, class, via and caller {pid,ppid,session,test}.
  bytes_read measures producer reads, not allocations. Repeated equal-valued rows
  may be distinct calls. cmd/fathom/metrics_traffic.go cmdMetricsTraffic accepts
  --from/--to/--call-log/--by tool and displays p50/p95 only. trafficReportFrom
  calls callDataFrom then metrics.AggregateTraffic; it is a contextual cross-check.
* internal/mcp/admission.go runAdmitted path wraps dispatch under pprof.Do labels
  tool/origin. Resolve the actual function name and binary symbol table in each
  pinned profile; label text without handler stack identity is insufficient.
* cmd/fathom/pprof_guard.go mountPprof exposes heap/allocs via pprof.Index,
  profile and trace behind guardPprof; seconds capped at 30 and one profile at a
  time. Do not issue one purported 60-minute profile request that is capped.
  Existing automatic diag_capture.go writes heap.pb.gz, not a guaranteed complete
  60-minute allocation delta; heap inuse_space is not alloc_space.
* internal/tenancy/Sample and RingRecord carry footprint/children and runtime
  heap totals, not request-allocation bytes. resourcesReport includes live/ring
  footprints but not a full RT-M4 verdict. Ring retains 24h, with one minute
  samples plus transitions. HeapSys is never phys_footprint (I47).
* internal/fusion/pipeline.go (*Pipeline).TypedPassTotal() int64 is a process
  lifetime counter, not a call-log count. internal/fusion/ingest.go gates typed
  work based on changed inputs and emits typed status. Reference regressions:
  internal/fusion/dg4_logger_test.go TestTypedEdges... tests/logging are not a
  real doc-only commit reading. Use the pinned pipeline identity and its actual
  counter deltas or a complete equivalent attestation, not absence of log text.

Grounding is read-only. There is no new command integration, allocation API,
consumer field or implementation file reservation. If an adapter is missing,
request its exact contract through foreman rather than add it inside a reading.

## Frozen input manifest

Before outcomes: binary commit+hash/Go version/profile decoder identity, config,
platform, RAM, effective budget/marks, profile sample rate and loss/error metadata,
UTC window bounds, acquisition method, raw hashes+byte lengths+rotations,
checker version and independent coverage witnesses. Opaque aliases in publication,
private mapping retained locally. Each input retains absent/unreadable/malformed/
stale/partial state and uncovered spans. No artifact-presence PASS.

Historical manifest must bind heap-0518's exact start/end (62 minutes is descriptive,
not sufficient timestamps), original profile, its alloc_space request-subtree
selection and 730.7GB unit/rounding provenance, and ALL calls in that SAME window.
The row uses 730.7 GB as authored; publish raw-byte interpretation and precision,
never silently convert GB to GiB. If the raw profile refines a rounded value across
the boundary, retain both and return for ruling, never select the favorable number.
Unknown today: raw profile identity, exact bounds and complete total call count.

Current manifest selects one prospective contiguous 60-minute real-traffic window,
not a favorable 60 minutes chosen afterward. Record interrupted binaries/restarts
and include all calls. Missing call rotations or logging-loss accounting leaves
population incomplete. Profile start/end must bind the same process/window,
with allocation sample type alloc_space, subtraction and matching symbolization.
A retained cumulative profile without an initial baseline cannot isolate the window.
A decoder error, sampling loss with unbounded effect, incompatible profiles or
missing request-subtree stack mapping makes allocation unknown. Never equate
process-wide allocation including ingest/background work with request allocation.

## Exact grading

1. Allocation: A_now / N_now ≤ (730.7e9 / N_heap0518)/20 with both denominators
   positive and complete. Publish numerator in raw bytes, each denominator,
   both per-call means, ratio, sample weights and rounding. Selection is union
   of request handler stack subtrees, each sample charged once, not sum of every
   cumulative parent/child row. Publish per-tool/origin mix and batch envelope vs
   subcall treatment, identically in old/new cohorts. If historical schema cannot
   match the admitted-decision accounting, INCOMPLETE, not reweighted synthetic traffic.
2. Structural frames: zero Sources/ListAllSymbols frame under a request handler
   in that full window, including aliases/inlined frames resolved by symbol map.
   Profiles cannot prove absence in unsampled stacks; label coverage and sampling
   limit, and require complete stack/frame evidence for an unconditional zero.
   One witness fails; no profile is unknown. Tests/lint alone are not real-traffic
   profile proof. Preserve this limitation even if every baseline unit test passes.
3. obligations latency: all logged non-test obligations calls completed in the
   window. Include refused/error/partial rows and disclose outcome strata, no
   success-only headline. Sorted raw latency_ms gives nearest-rank p50 and p99.
   p50≤60ms and p99≤300ms, equality passes. No obligations exposure is unmeasured.
4. app-kit ingest: pin source commit, source manifest, binary, extraction configuration
   and ingest start/end, same declared service and corpus as the authored target.
   Measure content+extract peak ≤300*2^20 bytes, publish own and child footprint
   separately. A process-wide resident maximum while unrelated work runs cannot
   be silently relabeled content+extract. Require isolated ingest attribution or
   exact component telemetry. Missing target snapshot or attribution is INCOMPLETE.
5. Doc-only commit: pin pre/post commits and file diff proving no .go/go.mod/go.sum/
   go.work or other typed-pass input changed. Same-process TypedPassTotal before/
   after the completed incremental ingest has delta=0, backed by baseline counter
   and complete ingest receipt. Reset/restart, missing counter, or disabled typed
   extraction is unknown, not proof. Control must show typed extraction enabled.
6. Retention: same process and source ingest as the pinned ingest receipt, OS
   footprint immediately before ingest F0 and at exact ingest-end+10min F10,
   F0>0, F10≤1.25*F0. Pin timestamps and concurrent work. Nearest available
   convenient sample is not an exact ten-minute reading. Child bytes reported
   separately, no HeapSys substitution. Missing point or restart is unknown.

Known violation => FAIL for its clause; incomplete other clauses remain visible.
Otherwise any unknown => INCOMPLETE. PASS requires all six complete clauses. The
reading carries a separate RT-D6 trigger assessment from clauses 4 and 6; it cannot
commission a worker process or relax the budget. No plan row box is authorized.

## Prospective tests and critic checklist

Implementing lane authors a scratch checker, immutable manifest and synthetic tests
before grading, with no product file changes. W3-2: no observed RED is claimed here.

| Name | Exact assertions |
|---|---|
| TestRTM4ProfileWindowAndPopulation | historical obligations-only denominator rejected; old/new bounds mismatch rejected; zero denominator unknown; missing rotation, unreadable profile, symbolization failure, reset, profile loss and sample type inuse_space each prevent PASS; matching distinct equal log lines retained; batch accounting mismatch unknown. |
| TestRTM4AllocationGrader | disjoint subtree union counts one sample once; ratio exactly1/20 passes, +epsilon fails; bytes_read or heap live substituted for alloc_space rejected; raw units retained; rounding interval crossing bar unknown. |
| TestRTM4HandlerFrames | one forbidden frame under handler fails; unrelated background frame disclosed without false request failure; inline alias resolves; absent/unreadable/unsampled frame coverage does not prove zero. |
| TestRTM4ObligationsQuantiles | 100 sorted rows p50=50th/p99=99th, equality60/300 passes, either +1 fails; refused/error/partial included; no obligations unknown; p95 never substituted. |
| TestRTM4IngestAndTypedPass | 300MiB equality passes/+1 fails; unrelated process peak not component allocation; doc-only delta0 passes only enabled; delta1 fails; restart/missing before counter unknown; Go dependency input in diff invalidates doc-only cohort. |
| TestRTM4TenMinuteRetention | 1.25 equality passes/+epsilon fails; F0 zero or unknown rejected; wrong timestamp, process mismatch, missing end point or substituted HeapSys unknown; crossing RT-D6 bars reported without an action. |
| TestRTM4Conjunction | all six clauses needed; one unavailable clause prevents PASS; known violation remains FAIL beside unknowns; current two collection commands or nonempty reading cannot earn completion. |

## Decision models

None applies. Deterministic accounting over operator-supplied profiles and receipts,
provider none, no paid or live collection commissioned.
