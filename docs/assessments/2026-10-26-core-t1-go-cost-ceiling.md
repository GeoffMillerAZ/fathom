# Go field-only cost audit: wall result reopened

C-R2 selected struct-field definitions without native reference edges. The wall result is **not settled**: one equivalent-work median-of-three sequence read +26.7% wall (CPU +20.9%) under asymmetric load; an independent rerun with the committed collector and same source/commits read +6.9% wall (CPU +10.8%). The earlier non-equivalent sequence read +15.9%. Store growth +6.3% reproduces. Neither passing nor failing this lever is established by those noisy sequences.

C-R5 recorded the originally reported failure. Runtime, tests and epoch remain reverted under the preregistered stop rule while the controller-required strict remeasurement is prepared. The [canonical Go reading](../bench/readings/lang/go.json) remains unchanged; no new L4 grade or holdout result is claimed.

| Historical sequence | Wall result | Store result / limitation |
|---|---|---|
| First field implementation | +15.9% | Non-equivalent: 25 plan-anchor edges lost; excluded. |
| Anchor-corrected original | +26.7% (78.461 to 99.412 s medians) | +6.3%; one baseline launch 9.01, all candidate runs encountered higher peak load than baseline counterparts. |
| Independent committed-collector rerun | +6.9% (87.577 to 93.659 s medians) | +6.3%; peak load reached 22.67, so this is not quiet passing proof either. |

The fixed source snapshot is archived main 5b84aa5b4, unchanged before/after every run. All six runs of the original corrected sequence preserved 330 typed packages, 150,810 native typed edges and each persisted edge-kind count; symbols grew from 63,329 to 84,893. Original wall arrays were baseline 99.362/78.461/78.329 and candidate 110.404/89.340/99.412. The independent arrays were baseline 77.613/125.932/87.577 and candidate 112.837/93.659/93.598. Do not substitute a minimum or CPU result for the preregistered median-wall gate.

## Provenance correction and remeasurement

The original receipt at commit 327002766 was assembled from an earlier **uncommitted collector revision**, then annotated by hand. It was not emitted by the current committed `cost.py`; its field names and comparison schema differ. Its numerical history remains in Git, but it must not be presented as a re-runnable output of that collector. The binding rerun will replace it with the committed collector's actual schema and measured verdict, keeping editorial/historical notes only under `annotations`.

The original extractor commit is publicly reachable b9a23a7dd; the anchor-corrected candidate is 68ef5163780197afde9346bddad720b6076677d9. Its measured unpublished original was tree-identical over the paths listed in the README, but unpublished identities are not reproduction inputs. Binary hashes identify local executables, not portable builds. The original candidate's focused regressions passed but four touched-package golden/count tests were stale. The rerun must first freeze a fully green candidate, with expectation-only repairs and no production-code change.

The [protocol](../bench/go-fields-cost-protocol.md) retains three alternating runs per arm, the same source snapshot, load <= 9 at launch after hashing, nice 10, GOMAXPROCS=2, GOFLAGS=-p=2, +15% median wall and +20% store gates. Sampled active-run loads are evidence, not a new rejection/filter rule. The controller explicitly requested remeasurement; no CPU co-gate or ABBA redesign is substituted. Gin remains unspent outside the canonical shard. If the rerun passes both gates with equivalent work, Go L4 reopens for independent oracle measurement.

## Named routes and concrete limits

| Candidate / route | Version / pin | Result / next condition |
|---|---|---|
| Historical full member pass | 5b49d9774, O673 | Historical +30% wall/+34% store; exported-only +26%/+22%. Those are separate implementations, not proof against field-only. |
| Field symbols without reference edges | b9a23a7dd, anchor repair 68ef51637 | C-R2 measured lever; wall result inconclusive across sequences, reproducible +6.3% store. Strict rerun pending. |
| Profile field-arm CPU delta | same candidate; unmeasured | Added nodes may enlarge O(nodes) call-target scans. A side index or narrower scans could change implementation cost; no intrinsic-cost ceiling is claimed. |
| Native semantic oracle | scip-go 0.2.7 | Available; unchanged Go reading already uses it. No new index or grade after original rollback. |
| Syntax oracle | Universal Ctags 6.2.1 | Available. Oracle choice does not establish runtime cost. |
| Other O673 levers | skip test references; call-only centrality; interface methods alone | Not selected by C-R2 and unmeasured; remain future experiments. |

This records a measured uncertainty, not an exhausted-tool ceiling. Governing rows `LB-F342` and `LB-Q-go-rest` identify field-only as already measured under C-R2 and link this audit. Runtime adoption remains subject to the cost and language-proof gates.
