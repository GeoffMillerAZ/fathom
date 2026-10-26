# Go field-only Q2 cost and calibration protocol

Ruling C-R2 (#20) authorizes top-level struct field definitions without native reference edges.
The field extractor is 61131a969, built on main 5b84aa5b4. The measured candidate also
excludes fields from bare plan-anchor matching while retaining exact Owner.Field lookup. Interface-method definitions and native
field read/write/reference edges are unchanged. I54 governs level claims, I23 call truth,
and K17/K26 allow at most +15% wall time and +20% store bytes without another owner ruling.

Before measurements, freeze a source snapshot of main 5b84aa5b4, preserving every tracked file.
Keep it outside the checkout's gitignored workspace tree so ancestor gitignore rules cannot
silently suppress discovery. Both binaries ingest this same source tree and logical service,
into a new empty local SQLite store for every run. No daemon, SCIP index, paid API, toolchain
fetch, source exclusion, typed-pass disable or previous typed cache participates in self-ingest.

Build the baseline before editing the extractor. Build the candidate from the frozen field-only
commit. Run baseline and candidate alternately, three runs each, serially under nice 10. Check
uptime before every heavy run and wait while one-minute load exceeds 9. Use the same local
Go module/build caches, GOWORK=off, GOFLAGS=-p=2, GOMAXPROCS=2 and GOTOOLCHAIN=local. Record
source SHA, binary digests, all wall times and store bytes, discovery/extraction counts,
read failures, typed packages/edges and skipped/failed work. Store row counts use immutable read-only access only after the process exits and no
nonempty WAL remains. An initial collector failed its read-only WAL initialization after a
healthy baseline ingest; that interrupted sequence is excluded and all six measured runs
are repeated with fresh stores. A first complete sequence then exposed 25 lost plan-anchor
edges: fields competed with functions for bare anchors. Its +15.9% wall/+6.3% store result
is not accepted as equivalent work. After a failing regression test and the narrow anchor
repair, repeat all six controls again. Thresholds and source controls are unchanged.

Compare medians. A baseline that
skipped the changed work is invalid under K27. Any unavailable or mismatched work blocks a
passing cost claim until explained.

Separately, run universal-ctags 6.2.1 and scip-go 0.2.7 on the calibration subjects at the source
SHAs in calibration/corpus.d/go.yaml and the independent
docs/bench/go-fields-holdout.yaml record, which is never canonical corpus input. Reproduce the historical baseline with the unchanged
binary where possible, then read the candidate and every affected Go reading. Keep the
independent Gin holdout frozen, index/read once, and never tune against it. Take three timing
runs on each final reading. Grades come from the existing harness and oracle tiers. A native
Go call-graph coverage diagnostic is not the graded SCIP-import typed_recall. Report both.

If the field-only lever remains below excellent, keep the measured cause in LB-Q-go-rest.
If it exceeds a cost gate, retain the receipt, revert the runtime change, and stop at the
written ceiling unless the owner rules otherwise. Do not add interface methods or reference
edges to rescue this experiment. The controller owns generated boards, plan checkboxes,
extractor-epoch numbering and the integration gate.

C-R5 outcome: the completed equivalent-work sequence measured +26.7% wall and +6.3% store.
One baseline launch was9.01 after hashing; this deviation is retained, and the stricter repeat
was interrupted by machine load above190. No passing cost claim is made. The owner directs
closing this lever and reverting the runtime. Portable commands are in
calibration/oracles/go-fields-cost/README.md. Gin remains unspent and outside canonical corpus.
