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
* internal/mcp/admission.go runAdmitted calls runHandler, which wraps dispatch under pprof.Do labels
  tool/origin. Pin runHandler and the binary symbol table in each profile; label text without handler stack identity is insufficient.
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
  internal/fusion/dg4_logger_test.go TestPipeline_SetLogger_RoutesTypedEdgeSummaryThroughLogger is not a
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

## Executable scratch handoff (critic correction)

Frozen PROPOSED interface, not implemented tooling or run evidence. W3-2 leaves
real failing tests to the implementing reader. All earlier assertions remain
binding. No product/telemetry/API grant. Synthetic test success cannot close
missing raw inputs, policies or dispatch prerequisites. READY is for document
review only. Resource-windows is final FIX round2; another unresolved finding is
HOLD. The other seven reading drafts are FIX round1 and have one round remaining.

Exact layout: workspace/tmp/reading-request-cost/checker.py (normalize/grade),
contracts.py (schema validation/declaration guard), test_checker.py (unittest
ContractTests), manifest.json, normalized.json, result.json and the three schemas
below. Fixture directory is fixtures/<exact-test-name>/ with input.json and
expected.json for EVERY assertion case in the earlier table. Implementing reader
creates these workspace-only files. Copy each JSON code block to its named file.

contracts.py implements only the closed JSON-Schema subset below (objects/
required/additionalProperties, arrays/items, enum/const, anyOf/null, minimum,
string pattern, date-time). Standard library only, no network schema fetch or
dependency. Check RFC3339 timestamps, unique artifact IDs, SHA256 and foreign
references, non-nil arrays, nonfinite numbers rejected. Quantity decimals use
canonical base10 strings; null upper bound is unbounded, never zero.
contracts.py guard parses the three Python files with ast without importing them,
requires checker normalize/grade entrypoints and ContractTests(unittest.TestCase)
with EACH exact method selected below declared once; checks the three schema
hashes pinned by the contract commit and fixtures for every case. Declaration
presence is never a substitute for running each assertion.

checker.py normalize --manifest PATH --output PATH validates manifest, reads ONLY
listed artifacts under this scratch root (reject absolute paths, traversal and
symlink escape), verifies raw hashes/lengths and uses only the named adapters.
SourceRef positions are half-open original byte offsets, record_index zero-based.
Every normalized row retains native lineage. Preserve identical distinct rows,
stack/frame/parent order and copy-overlap identity. Never synthesize missing facts.
Unreadable/absent native data appears as coverage gaps/nulls. No polling, capture,
network, provider, ingest, Compile/Sweep or live configuration mutation.

checker.py grade --manifest PATH --normalized PATH --output PATH revalidates
schemas, hashes, lineages, then recomputes all earlier criterion assertions from
native evidence, independent labels and bound policies. Declared PASS or approval
fields cannot supply evidence. Emit canonical UTF-8 JSON, sorted object keys;
criteria sorted by row_id,criterion_id,group_key and quantities by name. Every
authored clause and group appears, including missing required groups as INCOMPLETE.
Coverage includes all expected/read/malformed/torn/skipped rows and gaps.
complete=true needs independent accounting, not artifact presence. Pending policies
remain pending_policy_ids. Null/absent facts never become zero or empty success.
Unused group_key dimensions are empty strings. All result arrays are non-nil.

dispatch_state is BLOCKED for this draft interface; foreman approval plus real
input/prerequisite handoff is separate. Grade exit0 means valid computation,
including a FAIL/KILL/INCOMPLETE result, not measurement PASS. guard/validate/
normalize/grade exit1 means assertion/schema/identity/limit/refusal error; exit2
operational inability to read/write requested paths. Where output is writable,
errors have typed grader_errors kind and SourceRefs. TestCase exits nonzero on
any missing/weakened assertion. No new numerical bar or policy from this interface.

D-HANDOFF (W3-2, I38, L-R/parallel-lanes §3): recommend this exact standard-library
scratch layout, closed schemas and all tests; alternative ad-hoc reader protocol
is rejected because it leaves the handed contract unspecified. Proposed commands
below run from the clone root AFTER the implementing lane creates these files.
None ran in the author lane.

```sh
set -eu
cd "workspace/tmp/reading-request-cost"
test -f checker.py
test -f contracts.py
test -f test_checker.py
test -f manifest.schema.json
test -f normalized.schema.json
test -f result.schema.json
python3 contracts.py guard
python3 -m unittest test_checker.ContractTests.TestRTM4ProfileWindowAndPopulation
python3 -m unittest test_checker.ContractTests.TestRTM4AllocationGrader
python3 -m unittest test_checker.ContractTests.TestRTM4HandlerFrames
python3 -m unittest test_checker.ContractTests.TestRTM4ObligationsQuantiles
python3 -m unittest test_checker.ContractTests.TestRTM4IngestAndTypedPass
python3 -m unittest test_checker.ContractTests.TestRTM4TenMinuteRetention
python3 -m unittest test_checker.ContractTests.TestRTM4Conjunction
python3 contracts.py validate --kind manifest --input manifest.json
python3 checker.py normalize --manifest manifest.json --output normalized.json
python3 contracts.py validate --kind normalized --input normalized.json
python3 checker.py grade --manifest manifest.json --normalized normalized.json --output result.json
python3 contracts.py validate --kind result --input result.json
```

### Native adapters

| Adapter ID | Frozen normalization contract |
|---|---|
| native_call_log | Old/new native calls separately, explicit schema/counter accounting. |
| go_pprof_alloc_space | Pinned local go tool pprof -raw -sample_index=alloc_space PROFILE with immutable matching binary/symbol map; decode sample IDs/location/function/inline frames/values/type/unit/time; before/after separately. No60-minute capped HTTP capture. |
| profile_scope | Independent handler-frame and stack-coverage receipt maps request subtree union by sample identity; labels alone insufficient, sampling uncertainty retained. |
| operator_ingest | Pinned ingest receipts/counters/diff/OS readings at exact times; Pipeline.TypedPassTotal and independent component attribution, absent receipts missing. |

All input roles must appear in manifest.inputs. Missing roles use a named absent
Artifact with null hash/size and named coverage, never a placeholder evidence file.
Adapter IDs are dispatch keys of checker.py, not product commands. Independent
labels/derived classifications are checked against native source refs, not trusted
booleans. Prerequisite and policy authority must resolve to immutable receipts;
pending fields cannot be promoted by the reader. Unsupported native conversion
or missing capture contract keeps the corresponding consumer BLOCKED.

### manifest.schema.json

```json
{
  "$schema": "https://json-schema.org/draft/2020-12/schema",
  "type": "object",
  "properties": {"schema":{"const":"fathom.reading.scratch/manifest-v1"},"protocol":{"const":"reading-request-cost"},"source_base":{"type":"string"},"preregistered_at":{"$ref":"#/$defs/Timestamp"},"window":{"type":"object","properties":{"from":{"anyOf":[{"$ref":"#/$defs/Timestamp"},{"type":"null"}]},"to":{"anyOf":[{"$ref":"#/$defs/Timestamp"},{"type":"null"}]}},"required":["from","to"],"additionalProperties":false},"artifacts":{"type":"array","items":{"$ref":"#/$defs/Artifact"}},"inputs":{"type":"array","items":{"$ref":"#/$defs/Input"}},"policies":{"type":"array","items":{"$ref":"#/$defs/Policy"}},"prerequisites":{"type":"array","items":{"$ref":"#/$defs/Prerequisite"}}},
  "required": ["schema","protocol","source_base","preregistered_at","window","artifacts","inputs","policies","prerequisites"],
  "additionalProperties": false,
  "$defs": {"Artifact":{"type":"object","properties":{"artifact_id":{"type":"string"},"relative_path":{"type":"string"},"sha256":{"anyOf":[{"$ref":"#/$defs/Hash"},{"type":"null"}]},"size_bytes":{"anyOf":[{"type":"integer","minimum":0},{"type":"null"}]},"state":{"$ref":"#/$defs/ArtifactState"},"error":{"anyOf":[{"type":"string"},{"type":"null"}]}},"required":["artifact_id","relative_path","sha256","size_bytes","state","error"],"additionalProperties":false},"ArtifactState":{"type":"string","enum":["available","absent","unreadable"]},"Hash":{"type":"string","pattern":"^[0-9a-f]{64}$"},"Input":{"type":"object","properties":{"role":{"type":"string","enum":["historical_profile","historical_call_log","current_profile_before","current_profile_after","current_call_log","profile_symbol_map","stack_coverage","ingest_receipt","component_memory","typed_counter","commit_diff","footprint_samples","configuration","capture_manifest"]},"artifact_ids":{"type":"array","items":{"type":"string"}},"adapter":{"type":"string","enum":["native_call_log","go_pprof_alloc_space","profile_scope","operator_ingest"]}},"required":["role","artifact_ids","adapter"],"additionalProperties":false},"Policy":{"type":"object","properties":{"policy_id":{"type":"string"},"status":{"type":"string","enum":["approved","pending","not_applicable"]},"authority_refs":{"type":"array","items":{"$ref":"#/$defs/SourceRef"}},"parameters_artifact_id":{"anyOf":[{"type":"string"},{"type":"null"}]}},"required":["policy_id","status","authority_refs","parameters_artifact_id"],"additionalProperties":false},"Prerequisite":{"type":"object","properties":{"id":{"type":"string","enum":["heap_0518_identity","historical_call_population","profile_attribution","ingest_attribution"]},"state":{"type":"string","enum":["proven","missing","unproven"]},"evidence_refs":{"type":"array","items":{"$ref":"#/$defs/SourceRef"}}},"required":["id","state","evidence_refs"],"additionalProperties":false},"SourceRef":{"type":"object","properties":{"artifact_id":{"type":"string"},"record_index":{"type":"integer","minimum":0},"byte_start":{"type":"integer","minimum":0},"byte_end":{"type":"integer","minimum":0}},"required":["artifact_id","record_index","byte_start","byte_end"],"additionalProperties":false},"Timestamp":{"type":"string","format":"date-time"}}
}
```

### normalized.schema.json

```json
{
  "$schema": "https://json-schema.org/draft/2020-12/schema",
  "type": "object",
  "properties": {"schema":{"const":"fathom.reading.scratch/normalized-v1"},"protocol":{"const":"reading-request-cost"},"manifest_sha256":{"$ref":"#/$defs/Hash"},"coverage":{"type":"array","items":{"$ref":"#/$defs/Coverage"}},"records":{"type":"object","properties":{"calls":{"type":"array","items":{"type":"object","properties":{"cohort":{"type":"string","enum":["heap_0518","current"]},"call":{"$ref":"#/$defs/Call"}},"required":["cohort","call"],"additionalProperties":false}},"profiles":{"type":"array","items":{"type":"object","properties":{"source":{"$ref":"#/$defs/SourceRef"},"profile_id":{"type":"string"},"process":{"$ref":"#/$defs/ProcessKey"},"from":{"$ref":"#/$defs/Timestamp"},"to":{"$ref":"#/$defs/Timestamp"},"sample_type":{"type":"string","enum":["alloc_space"]},"unit":{"type":"string","enum":["bytes"]},"sample_rate":{"type":"integer","minimum":0},"decode_exit":{"type":"integer","minimum":0},"loss_upper_bytes":{"anyOf":[{"type":"integer","minimum":0},{"type":"null"}]}},"required":["source","profile_id","process","from","to","sample_type","unit","sample_rate","decode_exit","loss_upper_bytes"],"additionalProperties":false}},"samples":{"type":"array","items":{"type":"object","properties":{"source":{"$ref":"#/$defs/SourceRef"},"profile_id":{"type":"string"},"sample_id":{"type":"string"},"value_bytes":{"type":"integer","minimum":0},"frames":{"type":"array","items":{"type":"object","properties":{"function":{"type":"string"},"file":{"type":"string"},"line":{"type":"integer","minimum":0},"inlined":{"type":"boolean"}},"required":["function","file","line","inlined"],"additionalProperties":false}},"request_handler":{"anyOf":[{"type":"string"},{"type":"null"}]},"request_subtree":{"anyOf":[{"type":"boolean"},{"type":"null"}]}},"required":["source","profile_id","sample_id","value_bytes","frames","request_handler","request_subtree"],"additionalProperties":false}},"ingests":{"type":"array","items":{"type":"object","properties":{"source":{"$ref":"#/$defs/SourceRef"},"ingest_id":{"type":"string"},"process":{"$ref":"#/$defs/ProcessKey"},"source_commit":{"type":"string"},"from":{"$ref":"#/$defs/Timestamp"},"to":{"$ref":"#/$defs/Timestamp"},"content_extract_peak_bytes":{"anyOf":[{"type":"integer","minimum":0},{"type":"null"}]},"attribution_refs":{"type":"array","items":{"$ref":"#/$defs/SourceRef"}},"typed_enabled":{"type":"boolean"},"typed_before":{"anyOf":[{"type":"integer","minimum":0},{"type":"null"}]},"typed_after":{"anyOf":[{"type":"integer","minimum":0},{"type":"null"}]},"doc_only":{"anyOf":[{"type":"boolean"},{"type":"null"}]},"diff_refs":{"type":"array","items":{"$ref":"#/$defs/SourceRef"}}},"required":["source","ingest_id","process","source_commit","from","to","content_extract_peak_bytes","attribution_refs","typed_enabled","typed_before","typed_after","doc_only","diff_refs"],"additionalProperties":false}},"footprints":{"type":"array","items":{"type":"object","properties":{"source":{"$ref":"#/$defs/SourceRef"},"ingest_id":{"type":"string"},"process":{"$ref":"#/$defs/ProcessKey"},"at":{"$ref":"#/$defs/Timestamp"},"phase":{"type":"string","enum":["pre_ingest","end_plus_ten_min"]},"os_footprint_bytes":{"anyOf":[{"type":"integer","minimum":0},{"type":"null"}]},"children_bytes":{"anyOf":[{"type":"integer","minimum":0},{"type":"null"}]}},"required":["source","ingest_id","process","at","phase","os_footprint_bytes","children_bytes"],"additionalProperties":false}},"labels":{"type":"array","items":{"$ref":"#/$defs/Label"}}},"required":["calls","profiles","samples","ingests","footprints","labels"],"additionalProperties":false}},
  "required": ["schema","protocol","manifest_sha256","coverage","records"],
  "additionalProperties": false,
  "$defs": {"Call":{"type":"object","properties":{"source":{"$ref":"#/$defs/SourceRef"},"process":{"$ref":"#/$defs/ProcessKey"},"ts":{"$ref":"#/$defs/Timestamp"},"tool":{"type":"string"},"class":{"anyOf":[{"type":"string"},{"type":"null"}]},"origin":{"type":"string"},"outcome":{"anyOf":[{"type":"string"},{"type":"null"}]},"latency_ms":{"type":"number"},"via":{"type":"string"},"caller_test":{"anyOf":[{"type":"boolean"},{"type":"null"}]},"caller_pid":{"anyOf":[{"type":"integer","minimum":0},{"type":"null"}]},"session":{"anyOf":[{"type":"string"},{"type":"null"}]}},"required":["source","process","ts","tool","class","origin","outcome","latency_ms","via","caller_test","caller_pid","session"],"additionalProperties":false},"Coverage":{"type":"object","properties":{"input_role":{"type":"string","enum":["historical_profile","historical_call_log","current_profile_before","current_profile_after","current_call_log","profile_symbol_map","stack_coverage","ingest_receipt","component_memory","typed_counter","commit_diff","footprint_samples","configuration","capture_manifest"]},"artifact_ids":{"type":"array","items":{"type":"string"}},"expected_records":{"anyOf":[{"type":"integer","minimum":0},{"type":"null"}]},"read_records":{"type":"integer","minimum":0},"malformed_records":{"type":"integer","minimum":0},"torn_records":{"type":"integer","minimum":0},"skipped_records":{"type":"integer","minimum":0},"missing_intervals":{"type":"array","items":{"type":"object","properties":{"from":{"$ref":"#/$defs/Timestamp"},"to":{"$ref":"#/$defs/Timestamp"},"reason":{"type":"string"}},"required":["from","to","reason"],"additionalProperties":false}},"complete":{"type":"boolean"},"reason":{"anyOf":[{"type":"string"},{"type":"null"}]}},"required":["input_role","artifact_ids","expected_records","read_records","malformed_records","torn_records","skipped_records","missing_intervals","complete","reason"],"additionalProperties":false},"Hash":{"type":"string","pattern":"^[0-9a-f]{64}$"},"Label":{"type":"object","properties":{"label_id":{"type":"string"},"sources":{"type":"array","items":{"$ref":"#/$defs/SourceRef"}},"labeler_role":{"type":"string","enum":["native_oracle","independent_person","architect"]},"labelled_at":{"$ref":"#/$defs/Timestamp"},"outcome_blind":{"type":"boolean"},"value":{"type":"string"}},"required":["label_id","sources","labeler_role","labelled_at","outcome_blind","value"],"additionalProperties":false},"ProcessKey":{"type":"object","properties":{"daemon_id":{"type":"string"},"pid":{"type":"integer","minimum":0},"started_at":{"$ref":"#/$defs/Timestamp"},"binary_commit":{"type":"string"}},"required":["daemon_id","pid","started_at","binary_commit"],"additionalProperties":false},"SourceRef":{"type":"object","properties":{"artifact_id":{"type":"string"},"record_index":{"type":"integer","minimum":0},"byte_start":{"type":"integer","minimum":0},"byte_end":{"type":"integer","minimum":0}},"required":["artifact_id","record_index","byte_start","byte_end"],"additionalProperties":false},"Timestamp":{"type":"string","format":"date-time"}}
}
```

### result.schema.json

```json
{
  "$schema": "https://json-schema.org/draft/2020-12/schema",
  "type": "object",
  "properties": {"schema":{"const":"fathom.reading.scratch/result-v1"},"protocol":{"const":"reading-request-cost"},"manifest_sha256":{"$ref":"#/$defs/Hash"},"normalized_sha256":{"$ref":"#/$defs/Hash"},"state":{"$ref":"#/$defs/EvidenceState"},"dispatch_state":{"const":"BLOCKED"},"criteria":{"type":"array","items":{"$ref":"#/$defs/Criterion"}},"coverage":{"type":"array","items":{"$ref":"#/$defs/Coverage"}},"unknown_inputs":{"type":"array","items":{"$ref":"#/$defs/Unknown"}},"pending_policy_ids":{"type":"array","items":{"type":"string"}},"grader_errors":{"type":"array","items":{"type":"object","properties":{"kind":{"type":"string","enum":["schema","identity","limit","refusal","decode","coverage"]},"source_refs":{"type":"array","items":{"$ref":"#/$defs/SourceRef"}},"message":{"type":"string"}},"required":["kind","source_refs","message"],"additionalProperties":false}}},
  "required": ["schema","protocol","manifest_sha256","normalized_sha256","state","dispatch_state","criteria","coverage","unknown_inputs","pending_policy_ids","grader_errors"],
  "additionalProperties": false,
  "$defs": {"Coverage":{"type":"object","properties":{"input_role":{"type":"string","enum":["historical_profile","historical_call_log","current_profile_before","current_profile_after","current_call_log","profile_symbol_map","stack_coverage","ingest_receipt","component_memory","typed_counter","commit_diff","footprint_samples","configuration","capture_manifest"]},"artifact_ids":{"type":"array","items":{"type":"string"}},"expected_records":{"anyOf":[{"type":"integer","minimum":0},{"type":"null"}]},"read_records":{"type":"integer","minimum":0},"malformed_records":{"type":"integer","minimum":0},"torn_records":{"type":"integer","minimum":0},"skipped_records":{"type":"integer","minimum":0},"missing_intervals":{"type":"array","items":{"type":"object","properties":{"from":{"$ref":"#/$defs/Timestamp"},"to":{"$ref":"#/$defs/Timestamp"},"reason":{"type":"string"}},"required":["from","to","reason"],"additionalProperties":false}},"complete":{"type":"boolean"},"reason":{"anyOf":[{"type":"string"},{"type":"null"}]}},"required":["input_role","artifact_ids","expected_records","read_records","malformed_records","torn_records","skipped_records","missing_intervals","complete","reason"],"additionalProperties":false},"Criterion":{"type":"object","properties":{"row_id":{"type":"string","enum":["RT-M4"]},"criterion_id":{"type":"string","enum":["allocation_per_call","forbidden_handler_frames","obligations_p50","obligations_p99","ingest_content_extract","doc_only_typed_passes","ten_minute_footprint"]},"group_key":{"type":"object","properties":{"service_alias":{"type":"string"},"population_id":{"type":"string"},"dialect_id":{"type":"string"},"utc_day":{"type":"string"},"channel":{"type":"string"}},"required":["service_alias","population_id","dialect_id","utc_day","channel"],"additionalProperties":false},"state":{"$ref":"#/$defs/EvidenceState"},"quantities":{"type":"array","items":{"$ref":"#/$defs/Quantity"}},"evidence_refs":{"type":"array","items":{"$ref":"#/$defs/SourceRef"}},"unknowns":{"type":"array","items":{"$ref":"#/$defs/Unknown"}}},"required":["row_id","criterion_id","group_key","state","quantities","evidence_refs","unknowns"],"additionalProperties":false},"EvidenceState":{"type":"string","enum":["PASS","FAIL","KILL","INCOMPLETE","INCONCLUSIVE","HOLD"]},"Hash":{"type":"string","pattern":"^[0-9a-f]{64}$"},"Quantity":{"type":"object","properties":{"name":{"type":"string"},"unit":{"type":"string"},"value":{"anyOf":[{"type":"string"},{"type":"null"}]},"lower_bound":{"anyOf":[{"type":"string"},{"type":"null"}]},"upper_bound":{"anyOf":[{"type":"string"},{"type":"null"}]}},"required":["name","unit","value","lower_bound","upper_bound"],"additionalProperties":false},"SourceRef":{"type":"object","properties":{"artifact_id":{"type":"string"},"record_index":{"type":"integer","minimum":0},"byte_start":{"type":"integer","minimum":0},"byte_end":{"type":"integer","minimum":0}},"required":["artifact_id","record_index","byte_start","byte_end"],"additionalProperties":false},"Timestamp":{"type":"string","format":"date-time"},"Unknown":{"type":"object","properties":{"input_role":{"type":"string","enum":["historical_profile","historical_call_log","current_profile_before","current_profile_after","current_call_log","profile_symbol_map","stack_coverage","ingest_receipt","component_memory","typed_counter","commit_diff","footprint_samples","configuration","capture_manifest"]},"artifact_ids":{"type":"array","items":{"type":"string"}},"reason":{"type":"string"}},"required":["input_role","artifact_ids","reason"],"additionalProperties":false}}
}
```

### Pinned scratch schema digests

SHA256 of each JSON code block plus one terminal newline. contracts.py guard
compares exact file bytes to these constants, not a newly generated policy.

- manifest.schema.json: 15bf968e68fd02ba680b7196809b06ea1f512c73763d1155c04628bcf5092104
- normalized.schema.json: ab2b86815d13fc66353ba89855a057fa1b7a1526b801c970eabcf5417a45bb5e
- result.schema.json: 0048ebb5a1b2862f5de531d3dc643f660503afa50c4e94fef39a2e70952e14d3
