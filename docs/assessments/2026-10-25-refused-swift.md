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
ERROR-or-MISSING count. The committed screen in
`calibration/oracles/swift-parse/grammar-screen` runs both queries on each file: `(ERROR) @error` and `[(ERROR) (MISSING)] @error`. A file in
the second set but not the first is MISSING-only. This retake reproduces both
measures, including the original ERROR-node 1/35 subject split.

Reproduce the split from the repository root with the two corpus subjects at
the pins above:

```sh
go run ./calibration/oracles/swift-parse/grammar-screen
```

The command emits file/byte counts and both measures for each subject, then
writes the per-file sets to `workspace/artifacts/swift-error-measures.json`.
ERROR includes files that also hold MISSING; MISSING-only is the union minus
the ERROR set.

The production harness independently reports the ERROR-or-MISSING total:

```sh
go run ./cmd/indexbench lang-oracle -dialect swift -subject workspace/calibration/alamofire -subject-name Alamofire/Alamofire -out workspace/artifacts/swift-parse-screen.json
go run ./cmd/indexbench lang-oracle -dialect swift -subject workspace/calibration/swift-nio -subject-name apple/swift-nio -out workspace/artifacts/swift-parse-screen.json
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

## Candidate ledger (X-R1)

The measured ceiling is the product parser gate, not the absence of a Swift oracle.
A working build-index producer cannot repair that parser. Inspected candidates are
kept distinct from candidates actually run:

| Candidate and version or pin | Check and concrete limit |
|---|---|
| alex-pinkus/tree-sitter-swift 35245fb; native survey 0.7.3, 5f435e2, b42acc4, 0.7.4/171fa3b, 0c5444e, 4bdb925, 12dd8a3, 2907592, d36d452, 3589c87, 9e3b87f | Committed WASM 35245fb reproduces 123/642 ERROR-or-MISSING, including 36 ERROR files. The recorded native survey's best total is 119/642. None clears the 5% gate. Only 35245fb was measured under the WASM harness here. |
| Swift compiler parser, Apple Swift 6.4, `swiftc -frontend -parse-as-library -dump-parse` | Existing validated tier-B L1/L2 oracle. Its syntax dump is not resolved import or typed-edge truth, and invoking a compiler is not the admitted WASM product parser. |
| SourceKit-LSP / IndexStoreDB, Swift 6.4 toolchain route | Inspected build-index route, not run on these subjects here. It needs a compatible toolchain and subject build/index store. No L3 or L4 grade is claimed from inspection, and this route does not change the product grammar's 123/642 gate. |
| jarvis-intelligence/scip-swift 4ad1ce9e4826785604aed5fd529913efb1733df7, Apache-2.0 | Inspected README and licence, not run. macOS 14+, compatible pinned toolchain and a subject build with index-store data are required. This is an existing candidate to evaluate after the parse gate, not a missing producer or a failed oracle measurement. |
| In-tree grammar patch route: [owner ruling P-R1 (#15)](https://github.com/GeoffMillerAZ/fathom/issues/15#issuecomment-6406376301), published mechanism [PR #36](https://github.com/GeoffMillerAZ/fathom/pull/36), `gpu-sci/grammar-patches-r1` | Authorized route; the mechanism passed controller review and is published as PR #36, pending integration. No Swift patch has been measured. A patch must leave at most floor(0.05 * 642) = 32 gate-error files: at least 123 - 32 = 91 must clear overall. Clearing all 87 MISSING-only files leaves 36 ERROR files, so at least 36 - 32 = 4 ERROR-node files must also clear. Use the published mechanism; no unpublished implementation commit is cited. |

No holdout was added to the canonical Swift corpus shard.

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


## Five-source audit (X-R4)

Checked 2026-10-25. Source inspection only; no new corpus run or grade.


1. **Own compiler/runtime dump or introspection.** Queries checked: `swiftc --version`, `swiftc -help | rg 'dump-parse|dump-ast|index-store|emit-sil|print-ast'`, and `swiftc -frontend -help` with the same filter. Local toolchain is Apple Swift 6.4 (`swiftlang-6.4.0.34.1`); both help views list `-dump-parse`, `-dump-ast`, `-print-ast`, and `-index-store-path`; `swiftc -help` warns `-dump-ast-format` stability is not guaranteed. The Swift compiler [compilation model, PDF p. 39](https://download.swift.org/docs/assets/generics.pdf) confirms the dump phases; the [IndexStoreDB format guide](https://github.com/swiftlang/indexstore-db/blob/main/Sources/IndexStore/Index%20Store.md) confirms the index flag plus symbol USRs, declaration/reference/call roles and relations. Both URLs were reopened and contents verified. These are real oracle/index sources; dumping an AST does not supply a tree-sitter/WASM product grammar. No corpus run.
2. **SCIP and LSIF lists plus GitHub search.** Queries checked: Sourcegraph's [SCIP recommended indexer table](https://sourcegraph.com/docs/code-navigation/writing-an-indexer) (no Swift row), Sourcegraph-maintained archived [LSIF.dev indexer table](https://lsif.dev/) (no Swift row), and GitHub connector `search_repositories({query:"scip-swift",per_page:10})` (five repositories: `jarvis-intelligence/scip-swift`, `Fostonger/SwiftSCIPIndex`, `alex2481kobe/scip-swift`, `himanshu14/scip-swift`, `phuongddx/homebrew-scip-swift`). [`jarvis-intelligence/scip-swift` `4ad1ce9e4826`](https://github.com/jarvis-intelligence/scip-swift/tree/4ad1ce9e4826785604aed5fd529913efb1733df7) builds SwiftPM or Xcode project index stores via `swift build --enable-index-store` / `xcodebuild COMPILER_INDEX_STORE_ENABLE=YES`, then maps IndexStoreDB occurrences to genuine SCIP protobuf. Requires successful subject builds; Apple SDK projects require macOS/Xcode. [`alex2481kobe/scip-swift`](https://github.com/alex2481kobe/scip-swift) points at that exact `4ad1ce9` commit and README blob; [`himanshu14/scip-swift` `f177700b42cf`](https://github.com/himanshu14/scip-swift/commit/f177700b42cf5f5016745baea191cd2db793e410) has the same README blob and a later callable-kind mapping change, so it is a derivative branch, not an independent producer lineage. [`phuongddx/homebrew-scip-swift`](https://github.com/phuongddx/homebrew-scip-swift) is packaging, not an indexer. **All inspected, unrun on the subjects.** Sourcegraph list absence is not “no SCIP producer.”

   [`Fostonger/SwiftSCIPIndex` `88c222d17c36`](https://github.com/Fostonger/SwiftSCIPIndex/commit/88c222d17c3649083eb226b4459643d59dfb3d40) is a *distinct* IndexStoreDB reader (different package, command, source tree; 8 commits, no GitHub forks). Its [README](https://github.com/Fostonger/SwiftSCIPIndex) says Xcode DerivedData, macOS 13+, Swift 5.9+, and outputs SQLite `.db` or JSON `.json`. The [package manifest](https://github.com/Fostonger/SwiftSCIPIndex/blob/88c222d17c3649083eb226b4459643d59dfb3d40/Package.swift) depends on IndexStoreDB and ArgumentParser, with no SCIP protobuf dependency; [IndexCommand](https://github.com/Fostonger/SwiftSCIPIndex/blob/88c222d17c3649083eb226b4459643d59dfb3d40/Sources/SwiftSCIPIndexer/Commands/IndexCommand.swift) chooses only SQLite or JSON, and [SCIPJSONWriter](https://github.com/Fostonger/SwiftSCIPIndex/blob/88c222d17c3649083eb226b4459643d59dfb3d40/Sources/SwiftSCIPIndexer/SCIP/SCIPJSONWriter.swift) defines custom `Codable` records. Thus it is a potential native index/JSON oracle route, **not currently a demonstrated `.scip` protobuf producer** despite its name/README. The pinned repository tree has no LICENSE file, so reuse rights are unconfirmed. It also requires a successful Xcode build with DerivedData, so it does not clear the grammar gate; no subject run.
3. **Language server.** Query checked: `swiftlang sourcekit-lsp index references background indexing`. [SourceKit-LSP background-indexing design](https://github.com/swiftlang/sourcekit-lsp/blob/main/Contributor%20Documentation/Background%20Indexing.md) says target preparation resolves imports and compiler index records permit references/call hierarchy; definitions can work without an index. [Known issues](https://github.com/swiftlang/sourcekit-lsp/blob/main/Documentation/Enable%20Experimental%20Background%20Indexing.md) include SwiftPM-only background indexing and reference invalidation when USRs change. Candidate: toolchain-matched SourceKit-LSP / IndexStoreDB, unrun here; a build-index route, not a grammar repair.
4. **Package registry parser/typechecker/AST dumper.** Literal Swift Package Index registry query checked: [`search?query=swift-syntax`](https://swiftpackageindex.com/search?query=swift-syntax); first result is `swiftlang/swift-syntax`. Its [package record](https://swiftpackageindex.com/swiftlang/swift-syntax) and [actual 604.0.0 release](https://github.com/swiftlang/swift-syntax/releases/tag/604.0.0) confirm the version; [SwiftParser 604.0.0 API](https://swiftpackageindex.com/swiftlang/swift-syntax/604.0.0/documentation/swiftparser) accepts source and returns a SwiftSyntax tree, marking missing tokens. Candidate for a native AST oracle or reader, **unrun**. Its package API does not itself establish an admitted WASM parser, source-resolved import graph, or SCIP data.
5. **Runtime call/require tracing.** Queries checked: `Swift runtime trace calls LLDB`, `SWIFT_BACKTRACE`. Swift's [debugging docs](https://docs.swift.org/latest/documentation/userdocs/debugging/) offer LLDB launch/attach; its [container guide](https://docs.swift.org/latest/documentation/swiftlinux/packaging/) describes `swift-backtrace` on crashes. Neither is a general static all-call or import-edge index. Runtime observation would cover executed paths only; no corpus run.

**Swift action:** retain grammar dispatch refusal at measured pin, while preserving the viable *unrun* build-index route. A build-index/SCIP experiment with `scip-swift` and SourceKit-LSP can assess L4 and typed edges independently; it cannot silently clear the parse gate. The assessment and corrected plan row name the producer; no absent-producer ceiling is retained.
