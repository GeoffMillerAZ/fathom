# Swift refusal: reconcile the two error measures

Swift remains refused. The current WASM harness gate counts files with **ERROR
or MISSING: 123/642 (19.16%)**. The original reading counts files with an **ERROR
node: 36/642 (5.6%)**, and that count reproduces exactly. The other **87 files
have MISSING nodes only**. Both measures exceed 5%. No grammar, dialect, dispatch,
generated board or numeric oracle metric changed.

## Measured evidence

Retaken 2026-10-25 on macOS arm64, locally without a daemon, using the committed
WASM grammar at alex-pinkus/tree-sitter-swift
35245fbfee2fccf16273c6f4299438fb76875970. The blob's lock SHA is
4e464eadd1f93f6a619ea2d5e1c26573a777cc7a37e7aa146495a82d3c3b04b4.
The clean subject pins and file/byte totals match calibration/corpus.d/swift.yaml
and the original reading. No files were excluded.

| Subject | Commit | Files | Bytes | ERROR | MISSING only | ERROR or MISSING | Faults |
|---|---|---:|---:|---:|---:|---:|---:|
| Alamofire/Alamofire 5.9.1 | f455c2975872ccd2d9c81594c658af65716e9b9a | 92 | 1646045 | 1 | 4 | 5 | 0 |
| apple/swift-nio 2.99.0 | f71c8d2a5e74a2c6d11a0fbe324774b5d6084237 | 550 | 7850080 | 35 | 83 | 118 | 0 |
| Pooled | | 642 | 9496125 | 36 | 87 | 123 | 0 |

The original 2026-09-29 reading at 765a6502 explicitly says ERROR nodes and has
no files_with_error field. LB-F59 (0b604d7e8, 2026-10-01) introduced the harness's
ERROR-or-MISSING count. A scratch screen of the same committed blob ran both
queries on each file: `(ERROR) @error` and `[(ERROR) (MISSING)] @error`. A file in
the second set but not the first is MISSING-only. This retake reproduces both
measures, including the original ERROR-only 1/35 subject split.

The production harness independently reports the ERROR-or-MISSING total:

```sh
indexbench lang-oracle -dialect swift -subject workspace/calibration/alamofire -subject-name Alamofire/Alamofire -out workspace/artifacts/swift-parse-screen.json
indexbench lang-oracle -dialect swift -subject workspace/calibration/swift-nio -subject-name apple/swift-nio -out workspace/artifacts/swift-parse-screen.json
```

These commands are parse/tenancy screens without an oracle input. Their scratch
reading claims no level. Historical L1/L2 oracle grades remain historical; this
pass claims no accuracy regrade or throughput improvement.

## Native survey and existing work

The plan row preserves the 2026-10-24 native CLI pin survey: 123/642 at 35245fb,
119/642 from 5f435e2 through upstream HEAD 9e3b87f. Native ERROR-only counts were
28 and 24, respectively, unlike the committed WASM blob's 36 ERROR-node files.
Only the pooled ERROR-or-MISSING total agrees at 35245fb. No newer grammar pin
was built or regraded in this macOS retake.

The compiler parser adapter (calibration/oracles/swift-parse) remains the
validated tier-B L1/L2 oracle. SourceKit-LSP and IndexStoreDB expose build index
information. [scip-swift](https://github.com/jarvis-intelligence/scip-swift/tree/4ad1ce9e4826785604aed5fd529913efb1733df7)
exists (Apache-2.0). Its documented path requires macOS 14+, a subject build with
an index store, and a compatible pinned toolchain. It was inspected, not run or
validated against these subjects. L3 import resolution and typed edges remain
unmeasured here; the existence of a producer does not prove either.

## Retry routes and arithmetic

The ceiling holds. Retry an allowlisted upstream grammar commit **or a reviewed
in-tree grammar patch through P-R1**, measured by the same WASM harness on the
same pinned pooled corpus. At most **32 of 642** files may hold ERROR or MISSING:
floor(0.05 * 642) = 32. Clearing all 87 MISSING-only files leaves 36 ERROR-node
files, still 5.6%, so at least 4 ERROR-node files must also clear. At least 91
of the current 123 gate-error files must clear overall.

After that bar is met, remeasure all levels and evaluate the build-index oracle
candidate before dispatch or retargeting. No plan box is flipped. The generated
tracker remains frozen for the integrator, who must clarify its Swift template
as 5.6% with ERROR nodes and 19.2% with ERROR or MISSING before regeneration.
