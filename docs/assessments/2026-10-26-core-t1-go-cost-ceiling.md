# Go field-only cost ceiling, C-R5 and X-R1

Ruling C-R2 (#20) selected struct-field definitions without native reference edges. The measured
experiment emits owner-qualified top-level fields, no interface-method definitions, and
excludes fields from call-target lookup. A plan-anchor collision found during work-equivalence
auditing was repaired before the completed sequence below. All experimental runtime, tests
and epoch changes are reverted in the final branch. The [canonical Go reading](../bench/readings/lang/go.json) holds the unchanged grades.
No new language grade is claimed.

| Three-run control | Baseline | Field-only |
|---|---:|---:|
| Wall seconds | 99.362,78.461,78.329 | 110.404,89.340,99.412 |
| Median wall | 78.461 | 99.412 (+26.7%, gate +15%) |
| Store bytes, each run | 205312000 | 218238976 (+6.3%, gate +20%) |
| Observed symbols, each run | 63329 | 84893 |
| Typed packages / native edges | 330 /150810 | 330 /150810 |

Both arms ingest the same archived main source 5b84aa5b4 into fresh direct CLI stores with
GOMAXPROCS2, GOFLAGS-p2, nice10 and decisions off. Discovery/extraction, exclusions, read
failures, typed package/edge counts and every persisted edge-kind count match. No source
changed. All six ingests succeeded with zero read or typed-package failures.

The receipt discloses a protocol deviation: baseline1 started at load9.01 after source
hashing delayed the original <=9 check. That run is retained, not passed off as fully
compliant. A strict post-hashing-guard repeat began, but machine load rose above190 and
that sequence was stopped during its later idle load wait. It contributes no median.
Under owner C-R5, retain the measured failed wall gate and close this lever conservatively.
This is no passing performance proof. No frozen Gin holdout or new oracle reading was
consumed after rejection, preserving independent validation for a viable later candidate.

| Named candidate or route | Version / pin | Concrete outcome |
|---|---|---|
| Earlier full member pass | 5b49d9774 / O673 | L4 excellent historically, but self-ingest +30% wall/+34% store; reverted. Exported-only variant +26%/+22% also failed. |
| Struct fields, no native reference edges | measured original fe42b1e0d; source-equivalent review 68ef5163780197afde9346bddad720b6076677d9 | Completed equivalent-work receipt above misses wall gate. Its launch-load caveat is explicit. C-R5 rejects runtime adoption; no L4 uplift inferred. |
| Go semantic oracle | scip-go0.2.7 | Installed for the planned measurement; no new index run after cost rejection. Existing main reading already uses this version. It is not declared unavailable. |
| Go syntax oracle | Universal Ctags6.2.1 | Available independently. It cannot make an over-cost runtime lever acceptable; no unnecessary fresh holdout is spent. |
| Other O673 levers | skip test references; call-only centrality; interface methods alone | Not authorized as the selected C-R2 lever and not measured here. They remain explicit future experiments, not failed candidates or nonexistent tools. |

Reproduction lives in `calibration/oracles/go-fields-cost/README.md` and `cost.py`, with a
post-manifest launch guard. The receipt preserves the actual binary hashes and measured
commit identity. The review candidate has byte-identical `cmd/`, `internal/`,
`calibration/oracles/`, `go.mod` and `go.sum` trees; only review metadata/holdout placement
changed after measurement. No fresh primary or historical reading is silently rewritten.
Retry another owner-selected lever under unchanged K17 gates, with proper oracle grades
and independent validation only after it demonstrates an acceptable cost.
