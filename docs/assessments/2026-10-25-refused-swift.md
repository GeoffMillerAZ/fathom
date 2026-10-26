# Swift refusal: macOS harness retake

Swift remains refused. The committed WASM grammar at alex-pinkus/tree-sitter-swift
35245fbfee2fccf16273c6f4299438fb76875970 leaves **123 of 642 files (19.16%)** with an
ERROR or MISSING node, above the unchanged 5% line. The historical 36/642 (5.6%)
recorded count did not reproduce and does not represent the current harness gate. No grammar, dialect,
dispatch, generated board, or numeric oracle metric changed.

## Evidence

Measured 2026-10-25 at fathom 3b77ae9c4 on macOS arm64, locally without a daemon.
The exact calibration/corpus.d/swift.yaml subject pins were cloned clean, with no
file exclusions. The production indexbench lang-oracle path measured the dialect's
own extractor and counted ERROR or MISSING with filesWithError after timing.

| Subject | Commit | Files | Bytes | Files with error | Faults |
|---|---|---:|---:|---:|---:|
| Alamofire/Alamofire 5.9.1 | f455c2975872ccd2d9c81594c658af65716e9b9a | 92 | 1646045 | 5 | 0 |
| apple/swift-nio 2.99.0 | f71c8d2a5e74a2c6d11a0fbe324774b5d6084237 | 550 | 7850080 | 118 | 0 |
| Pooled | | 642 | 9496125 | 123 | 0 |

The embedded blob's lock SHA is
4e464eadd1f93f6a619ea2d5e1c26573a777cc7a37e7aa146495a82d3c3b04b4.
Both file and byte totals match the original reading. This confirms the native
CLI count recorded on 2026-10-24. Native newer-pin screening reached 119/642,
still above 5%, and upstream HEAD remains 9e3b87f66f1fd2f3b931376fd3723ab788e337c3.
No newer pin was built or regraded in this retake.

Reproduction, with a locally built indexbench and lane-local Go caches:

```sh
indexbench lang-oracle -dialect swift -subject workspace/calibration/alamofire -subject-name Alamofire/Alamofire -out workspace/artifacts/swift-parse-screen.json
indexbench lang-oracle -dialect swift -subject workspace/calibration/swift-nio -subject-name apple/swift-nio -out workspace/artifacts/swift-parse-screen.json
```

These are parse/tenancy screens without an oracle input. Their scratch output
claims no level. Historical L1/L2 oracle grades remain historical. This pass
claims neither a new accuracy grade nor a throughput improvement.

## Existing work and ceiling

The compiler parser adapter (calibration/oracles/swift-parse) remains the
validated tier-B L1/L2 oracle. SourceKit-LSP and IndexStoreDB expose build index
information. Contrary to the old absolute absence claim, a SCIP producer now
exists: [scip-swift](https://github.com/jarvis-intelligence/scip-swift/tree/4ad1ce9e4826785604aed5fd529913efb1733df7),
Apache-2.0. Its documented path requires macOS 14+, a subject build with an index
store, and a pinned compatible Swift toolchain. It was inspected, not installed,
built, run or validated against these subjects. It supplies a candidate for later
measurement, not a typed or import-resolution claim today.

The parse ceiling holds. Retry when an allowlisted grammar passes the same
pinned pooled corpus at at most 5% under the WASM harness. Then remeasure all
levels and evaluate the build-index candidate. Do not dispatch or retarget Swift
on these parse counts alone. No plan box is flipped.
