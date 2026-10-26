# Go field-only cost rerun: both gates pass

The controller-required rerun with the unchanged committed collector reports **PASS: +11.62% median wall and +6.30% store, equivalent work in all six runs**. Go L4 reopens for independent oracle validation. C-R2 selected struct-field definitions without native reference edges. The earlier failed-wall conclusion did not reproduce: one equivalent-work median-of-three sequence read +26.7% wall (CPU +20.9%) under asymmetric load; an independent rerun with the committed collector and same source/commits read +6.9% wall (CPU +10.8%). The earlier non-equivalent sequence read +15.9%. Store growth +6.3% reproduces. Those earlier noisy sequences did not establish a settled failure. The strict rerun below is the current measured gate result, not a new language grade.

C-R5 recorded the originally reported failure. Runtime, tests and epoch remain reverted under the preregistered stop rule while runtime adoption awaits a separately serialized ENGINE wave and independent language readings. The [canonical Go reading](../bench/readings/lang/go.json) remains unchanged; no new L4 grade or holdout result is claimed.

| Historical sequence | Wall result | Store result / limitation |
|---|---|---|
| First field implementation | +15.9% | Non-equivalent: 25 plan-anchor edges lost; excluded. |
| Anchor-corrected original | +26.7% (78.461 to 99.412 s medians) | +6.3%; one baseline launch 9.01, all candidate runs encountered higher peak load than baseline counterparts. |
| Independent committed-collector rerun | +6.9% (87.577 to 93.659 s medians) | +6.3%; peak load reached 22.67, so this is not quiet passing proof either. |

The fixed source snapshot is archived main 5b84aa5b4, unchanged before/after every run. All six runs of the original corrected sequence preserved 330 typed packages, 150,810 native typed edges and each persisted edge-kind count; symbols grew from 63,329 to 84,893. Original wall arrays were baseline 99.362/78.461/78.329 and candidate 110.404/89.340/99.412. The independent arrays were baseline 77.613/125.932/87.577 and candidate 112.837/93.659/93.598. Do not substitute a minimum or CPU result for the preregistered median-wall gate.

## Strict committed-collector rerun

| Arm / run | Wall seconds | CPU seconds | Launch / end / max load |
|---|---:|---:|---|
| Baseline 1 | 78.565 | 84.419 | 7.49 / 5.09 / 7.49 |
| Candidate 1 | 87.693 | 91.963 | 5.09 / 3.82 / 5.09 |
| Baseline 2 | 83.545 | 86.208 | 3.82 / 6.35 / 7.09 |
| Candidate 2 | 86.665 | 91.285 | 6.35 / 4.79 / 6.75 |
| Baseline 3 | 75.564 | 80.567 | 4.79 / 4.58 / 5.05 |
| Candidate 3 | 121.897 | 95.228 | 4.58 / 13.09 / 16.29 |

Median wall is 78.565 to 87.693 seconds (+11.62%, limit +15%). Every baseline
store is 205,312,000 bytes; every candidate store is 218,238,976 (+6.30%, limit
+20%). Median CPU is 84.419 to 91.963 seconds (+8.94%), diagnostic only. The last
candidate encountered higher active-run load; it is retained, with no filtered
runs or minimum substituted. All launches pass the post-manifest <=9 rule.
The collector preserves all workload/typed counters and persisted edge kinds:
330 typed packages, 150,810 native edges, 5,805 persisted file-resolved edges,
zero read/typed failures. Candidate field work executed (63,329 to 84,893 symbols).
Source hash remained unchanged, stores were fresh, and no WAL sidecar remained.

Both binaries were built with Go 1.27.3 darwin/arm64, `-trimpath -buildvcs=false`,
nice 10, GOMAXPROCS=2, GOFLAGS=-p=2 and the same lane caches. See the
[collector receipt](../bench/readings/go-fields-cost-2026-10-26.json) for exact
binary/source/collector hashes. No runtime, canonical grade or Gin holdout is
adopted by this documentation branch.

## Provenance correction and remeasurement

The original receipt at commit 327002766 was assembled from an earlier **uncommitted collector revision**, then annotated by hand. It was not emitted by the current committed `cost.py`; its field names and comparison schema differ. Its numerical history remains in Git, but it must not be presented as a re-runnable output of that collector. The binding rerun replaces it with the committed collector's actual schema and measured verdict. A deep comparison confirms every emitted field is unchanged; only `annotations` is added for history/build conditions. The raw collector receipt hash is retained there.

The original extractor commit is publicly reachable b9a23a7dd; the anchor-corrected candidate is 68ef5163780197afde9346bddad720b6076677d9. Its measured unpublished original was tree-identical over the paths listed in the README, but unpublished identities are not reproduction inputs. Binary hashes identify local executables, not portable builds. The original candidate's focused regressions passed but four touched-package golden/count tests were stale. Rerun candidate 4803e3d8b was frozen after the complete core/extractor/callgraph/fusion/langbench race suite passed. Only four test files differ from 68ef51637 (including main's hook regression), with no production-code change.

The [protocol](../bench/go-fields-cost-protocol.md) retains three alternating runs per arm, the same source snapshot, load <= 9 at launch after hashing, nice 10, GOMAXPROCS=2, GOFLAGS=-p=2, +15% median wall and +20% store gates. Sampled active-run loads are evidence, not a new rejection/filter rule. The controller explicitly requested remeasurement; no CPU co-gate or ABBA redesign is substituted. Gin remains unspent outside the canonical shard. The rerun passes both gates with equivalent work, so Go L4 reopens for independent oracle measurement.

## Named routes and concrete limits

| Candidate / route | Version / pin | Result / next condition |
|---|---|---|
| Historical full member pass | 5b49d9774, O673 | Historical +30% wall/+34% store; exported-only +26%/+22%. Those are separate implementations, not proof against field-only. |
| Field symbols without reference edges | b9a23a7dd, anchor repair 68ef51637 | C-R2 measured lever. Strict committed-collector rerun passes +11.62% wall/+6.30% store; earlier noisy results are historical. No new L4 grade yet. |
| Profile field-arm CPU delta | same candidate; unmeasured | Added nodes may enlarge O(nodes) call-target scans. A side index or narrower scans could change implementation cost; no intrinsic-cost ceiling is claimed. |
| Native semantic oracle | scip-go 0.2.7 | Available; unchanged Go reading already uses it. No new index or grade after original rollback. |
| Syntax oracle | Universal Ctags 6.2.1 | Available. Oracle choice does not establish runtime cost. |
| Other O673 levers | skip test references; call-only centrality; interface methods alone | Not selected by C-R2 and unmeasured; remain future experiments. |

This records a passing cost rerun, not an exhausted-tool ceiling or a language grade. Governing rows `LB-F342` and `LB-Q-go-rest` identify field-only as already measured under C-R2 and link this audit. Runtime adoption remains subject to the cost and language-proof gates.

## X-R4 source searches (2026-10-26 UTC)

| Required source searched | Named candidate / result | Concrete scope limit |
|---|---|---|
| Own compiler/toolchain | [go/ast](https://pkg.go.dev/go/ast), [go/types Info](https://pkg.go.dev/go/types#Info), frozen Go extractor at 68ef51637 | Defs/Uses/Selections expose exact typed objects. The selected lever is already implemented against native Go information; an alternative oracle cannot settle its runtime cost. |
| Sourcegraph SCIP list and GitHub search | [SCIP list](https://github.com/scip-code/scip#readme), [Sourcegraph recommendations](https://sourcegraph.com/docs/code-navigation/writing-an-indexer#sourcegraph-recommended-indexers); query `scip-go` | Maintained scip-go 0.2.7 is the existing independent oracle. No producer absence is claimed. |
| LSIF lineage | Query `sourcegraph lsif indexers Go`; [lsif-go](https://github.com/sourcegraph/lsif-go) | Archived predecessor; current recommendation is scip-go. Switching export formats does not reduce the field implementation's node/scan cost. |
| Language server | [gopls navigation](https://go.dev/gopls/features/navigation), registry gopls v0.24.0 | Build-configuration-scoped definitions/references/call hierarchy exist. No corpus export measured here; neither this interface nor a SCIP producer substitutes for self-ingest cost proof. |
| Package registry / typechecker | Direct lookup `proxy.golang.org/golang.org/x/tools/gopls/@latest` returned v0.24.0; standard go/ast and go/types APIs above | The language's maintained semantic tools are available; cost profiling and implementation choice remain separate questions. |
| Runtime call tracing | [Go diagnostics](https://go.dev/doc/diagnostics), pprof and runtime/trace | Suitable for profiling the delta; dynamic execution omits unexecuted static call sites, so tracing is not replacement whole-corpus oracle truth. Profiling remains an unmeasured next route. |

No Git history is rewritten to remove old private experiment labels; current records
use public C-R2/C-R5 ids and distinguish the stop rule from the owner's closure.
