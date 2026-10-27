# C++ producer and parser audit (2026-10-26)

The current audit establishes a parser admission failure, not new C++ support. The canonical reading remains the historical contract. No new holdout was selected, read or used for tuning. The committed parser lock and C++ blob are unchanged after both temporary grammar experiments. The isolated foreign-runtime experiment remains diagnostic.

The historical reading at landing base `6db920e72` has SHA-256 `4d4949cddb6fe4f95c6b8aa49e18c31ad9555af18e98a6e344f5a38c0262348b`: definition mapping .8988 (5242/5832), typed recall .9656, file-edge precision .9023 and recall .5354. Its restricted source scope and filtered 34-entry spdlog database differ from the new audit. CUDA and HLSL are landed, so their old scheduling dependency has cleared.

## Completed controlled measurements

`calibration/oracles/scip-clang/cpp-audit.py` at `0deba117abaadee0343a58a1645f9232a92b6167` produced six indexes, nine native families and four median-of-three readings from fresh dependency-attested inputs. Its SHA-256 is `baf94aaa397277fff0bd78cfefa7a384142aefba0665c08ea187ebafda348461`.

| Receipt | SHA-256 |
|---|---|
| Six indexes | `77da8d59282b222cb65d34a5af3d0844409560733660a89db4fb0052081733c3` |
| Nine native families | `2dfce1ac07bf43a35f7d673341d3211cf764f496282497f874dc487812b02c20` |
| Four readings | `dae9e402c004732dd3692b2c4176ba5b4a1b57db2e428a741e504e747dca0c28` |

Legacy means **legacy file scope / full compiler database**, not an exact historical replay. Full means every file the harness discovers and routes to C++. Both use the same compilation databases, native companions, Ctags, extractor and grammar. SCIP 0.4.0 and 0.4.1 produce identical numerators, denominators, grades, tiers and source populations. Only diagnostic counts `scip_file_edges_every_definer` and `scip_edges_macro_generated` differ.

| Scope | Files / bytes | Definition mapping | Typed recall | File precision / recall | Admission |
|---|---:|---:|---:|---:|---|
| Legacy | 222 / 2475212 | .7816 | .9268 | .9757 / .4741 | 65 selected syntax-error files, above 5% |
| Full | 679 / 9092459 | .8075 | .8143 | .9577 / .1863 | 163 selected syntax-error files and one fault; level none |

Numeric grades are diagnostic: legacy computes L1/L2 excellent, L3 good and L4 unmet; full computes L1 good, L2 excellent and L3/L4 unmet. These do not override parser admission. Median CPU throughput is 1.15/1.13 MB/CPU-s for legacy and 1.09/1.11 for full (0.4.0/0.4.1). Full fault rate 1/679 exceeds the 0.1% ceiling independently of syntax admission. L1/range metrics remain tier B because Ctags enumerates their population. Native AST ranges do not raise that population to tier A.

The [sanitized summary](../bench/readings/cpp-audit-2026-10-26/summary.json) is produced by committed `cpp-report.py`, with SHA-256 `5a70a48a00042c3f205d427cb406357e78e1e5d98b3653cb91f8b7e40be5e386`. It binds all six raw indexes, nine native files, four readings and six original screen receipts, plus the exact baseline screen binary and producer source. Both SCIP versions have the following per-subject numerators/denominators (ratios rounded to four decimals). No source content, subject file lists or fault paths are published.

**Legacy population**

| Metric / tier | fmt | json | spdlog |
|---|---:|---:|---:|
| call_precision / A | 2493/2588 = 0.9633 | 1036/1076 = 0.9628 | 977/982 = 0.9949 |
| call_recall / A | 2493/2700 = 0.9233 | 1036/1115 = 0.9291 | 977/1077 = 0.9071 |
| def_mapping / A | 3381/3916 = 0.8634 | 2756/4115 = 0.6697 | 1179/1329 = 0.8871 |
| file_edge_precision / A | 157/157 = 1.0000 | 211/214 = 0.9860 | 474/492 = 0.9634 |
| file_edge_recall / A | 157/458 = 0.3428 | 211/233 = 0.9056 | 474/1085 = 0.4369 |
| import_resolution / A | 89/89 = 1.0000 | 50/50 = 1.0000 | 118/118 = 1.0000 |
| kind_agreement / B | 3581/3581 = 1.0000 | 1729/1729 = 1.0000 | 1270/1270 = 1.0000 |
| range_agreement / B | 3314/3326 = 0.9964 | 1428/1436 = 0.9944 | 1190/1190 = 1.0000 |
| signature_coverage / B | 2363/2363 = 1.0000 | 789/789 = 1.0000 | 758/758 = 1.0000 |
| sym_precision / B | 3581/3629 = 0.9868 | 1729/1851 = 0.9341 | 1270/1279 = 0.9930 |
| sym_recall / B | 3581/3716 = 0.9637 | 1729/1798 = 0.9616 | 1270/1277 = 0.9945 |
| typed_recall / A | 33403/34701 = 0.9626 | 19122/22352 = 0.8555 | 6659/6804 = 0.9787 |

**Full population**

| Metric / tier | fmt | json | spdlog |
|---|---:|---:|---:|
| call_precision / A | 4794/5060 = 0.9474 | 2616/2748 = 0.9520 | 2286/2345 = 0.9748 |
| call_recall / A | 4794/5363 = 0.8939 | 2616/2825 = 0.9260 | 2286/2470 = 0.9255 |
| def_mapping / A | 7215/8347 = 0.8644 | 5807/8184 = 0.7096 | 3079/3409 = 0.9032 |
| file_edge_precision / A | 198/199 = 0.9950 | 569/597 = 0.9531 | 501/528 = 0.9489 |
| file_edge_recall / A | 198/536 = 0.3694 | 569/5074 = 0.1121 | 501/1198 = 0.4182 |
| import_resolution / A | 109/109 = 1.0000 | 229/229 = 1.0000 | 124/124 = 1.0000 |
| kind_agreement / B | 7267/7267 = 1.0000 | 7263/7263 = 1.0000 | 3376/3376 = 1.0000 |
| range_agreement / B | 6730/6756 = 0.9962 | 6005/6054 = 0.9919 | 3051/3059 = 0.9974 |
| signature_coverage / B | 4522/4522 = 1.0000 | 3781/3781 = 1.0000 | 2070/2070 = 1.0000 |
| sym_precision / B | 7267/7418 = 0.9796 | 7263/7876 = 0.9222 | 3376/3416 = 0.9883 |
| sym_recall / B | 7267/7634 = 0.9519 | 7263/8210 = 0.8847 | 3376/3467 = 0.9738 |
| typed_recall / A | 118900/130825 = 0.9088 | 39812/67665 = 0.5884 | 18415/19019 = 0.9682 |


## Population and dependency accounting

Exact recipe §13.3.1 tracked-set manifests are unchanged before and after configuration, production and grading. Each subject has zero tracked symlinks/gitlinks, one ignored non-language compilation-database symlink, and no added generated language files. File lists remain private.

| Subject | Tracked files / bytes | Tracked-set SHA-256 | C++ legacy / full files | CompDB entries / own unique / outside |
|---|---:|---|---:|---:|
| fmt | 142 / 3093051 | `5805a5fe1abf223ee7dffc409d74f6e44e909c06353b4b442744597b039b675f` | 60 / 69 | 51 / 31 / 0 |
| json | 1163 / 15958746 | `4111e73b93fc872ea8b53b91cd0d5eecc83cba1f9243dce087df8e862ba40bba` | 46 / 475 | 93 / 78 / 0 |
| spdlog | 177 / 1210549 | `61cee59630f5a71436abafddc625992b2288c5693ecf81f2189ab3dd3775a61d` | 116 / 135 | 141 / 34 / 107 |

Extension candidates number 71/476/147; the 2/1/12 remaining headers route to C. No directory exclusion or file-population trim explains that difference. The external spdlog units belong to its declared Catch2 v3.5.0 dependency, pinned clean at `53d0d913a422d356b23dd927547febdf69ee9081`, Boost Software License 1.0, licence SHA-256 `c9bff75738922193e67fa726fa225535870d2aa1059f91452c411736284ad566`. Full build input trees, fetched pins, generated includes and compilation databases are hashed at every phase. The OS SDK has a separate toolchain identity, not a whole-SDK manifest claim.

SCIP document records are not unique source coverage. Fmt has 60 records / 54 unique paths, attesting 48/60 legacy and 52/69 full files. Json attests 46/46 and 132/475. Spdlog attests 101/116 and 108/135. Full unenumerated files number 17/343/27. They remain disclosed source-coverage gaps; compiled translation units are not a claim that the producer enumerates every source file. No subject crosses the per-subject 10% oracle-blind file-pair cap in these readings. Blind file pairs are 1/8/2 legacy and 1/10/3 full; blind call pairs are zero.

Native preprocessor/call command counts are 36/93/141. Range collection uses first-source-command deduplication: 31/78/141 dumps, with 20/15/0 repeated entries skipped. All completed families report zero stopped or errored units.

## Historical reconciliation

The older `f2b852ab` scorer against the same frozen audit-r3 json index, native companions and legacy source scope produces every subject count identically to the current scorer, including 2756 mapped / 4115 SCIP definitions. The current canonical artifact instead records 1224/1382 for json. Thus the old oracle/input artifact differs; this audit does not identify which historical input changed. It does not attribute the delta to a new scorer denominator rule. The detached historical checkout's own reading differs again (1225/1383) and is not substituted for the canonical artifact.

The old syntax total 151/222 exactly matches fresh raw trees (32 fmt + 44 json + 75 spdlog). The selected-tree total is 65 (17 + 14 + 34). The historical scorer reported raw trees; the current scorer reports selected trees after preparse. That reporting-definition change is not a grammar improvement, and both counts fail admission.

## Parser candidates and fault disclosure

The committed `cpp-screen` helper separately counts ERROR, MISSING, their intersection, exclusive portions and union for raw and selected trees over exact reading tenancy. It checks clean compiled/live revision, subject pin, source/population/input hashes, bounded paths and output exclusivity. Candidate mode keeps the baseline reading identity separate and admits only explicitly audited changes.

| Candidate | Immutable identity | Result / concrete limit |
|---|---|---|
| Incumbent official grammar | `f41e1a044c8a84ea9fa8577fdd2eab92ec96de02`, blob `b5ca4b0cd346bc1cd1060fdc2151125a11ad7695da595157667a5e8b30b03e34` | Legacy selected 65/222. Full fmt 20/69 and spdlog 42/135 complete; json stops after 344/475, leaving 131 unresolved. |
| Official HEAD, MIT, ABI15 | `c009222808634c1014f82438d4883753516a2c24`, blob `fde7035227a488422e95cc1a408fecbbc5b3440dceebf109e42d483d31270ed2` | Legacy59/222. Full fmt 19/69 + spdlog 41/135 already exceeds pooled 33-file allowance; json stops after 276/475, leaving 199 unresolved. Reverted. |
| Unreal fork, MIT, ABI15 | `920e9810cc64a47fa05cfcf30c7caecb85b5f2c3`, blob `613af35868891df920a1e5c12fb67b2c2e83eb6a7c9cab6d3a1247acd20d22df` | Legacy61/222. Full fmt 19/69 + spdlog 43/135 exceeds allowance; same json stop/unresolved count. Reverted. |
| WillBooster grammar 6.0.2 + runtime 7.0.5, MIT | Crate SHA-256s `8c892195d9e44d816b4cc5ffa9f8cf53d2d1c00b3030feb0d35e612ae5f34bcc` / `166cf42202927500081763e65e63d3c0bc9273e05ba998496756fff7569de9c5` | ABI0x57420006 requires its own core. Two builds are byte-identical. Legacy50/222. Full spdlog 37/135 alone exceeds 33-file allowance; fmt/json selected traps leave27/199 unresolved. Diagnostic rejected; no product pin change. |

O1508 records the unresolved incumbent full-json parser fault. A committed helper replay reproduces it at raw ordinal 345; the isolated file probe passes. The exact-order diagnostic traps in 31.767 ms with `wasm error: unreachable` and prior peak 1000 WASM pages. Neither an out-of-memory cause nor a timeout is established. The two ABI15 candidates trap at selected ordinal 277. WillBooster traps at selected ordinal 43 for fmt and277 for json, with prior peaks957/901 pages; memory exhaustion remains unproven. Incomplete screen counters are not population rates, and their unresolved files are never dropped.

The foreign-runtime audit is C++-only and behind `fathom_grammar_audit`. Its lock must match exact source/tool/licence/module hashes; arbitrary self-consistent module/lock pairs are refused. The production loader, trust policy, grammar lock and default parser are unchanged. The measured module is `e39bd1cf09c15b52b9495b8e19e6b0627f41312f1e60453f4427695baf29e684` (3852285 bytes), with pinned Zig 0.16.0 and wasm-opt 129. This route does not require moving every other language to a fork runtime.

## X-R1/X-R4 source ledger (read-only triage)

Sources checked 2026-10-26. Measured and untried routes are distinguished below; a source search does not establish whole-population truth.

| Route | Primary source and result | Status / next check |
|---|---|---|
| Own compiler | [Clang command reference](https://clang.llvm.org/docs/ClangCommandLineReference.html), [JSON compilation database](https://clang.llvm.org/docs/JSONCompilationDatabase.html); local `/usr/bin/clang` is Apple clang 21.0.0 (`clang-2100.3.34.2`). Existing `clang-calls`, `clang-ast-defs`, and build-flag `preproc` adapters cover written calls, definition ranges, includes and active lines. | Measured in audit-r3: all nine native families completed without stopped or errored units. Range collection deduplicates each source to its first command; it does not cover every configuration. |
| SCIP/LSIF | [Sourcegraph's indexer list](https://sourcegraph.com/docs/code-navigation/writing-an-indexer) names scip-clang for C/C++; [scip-clang repository](https://github.com/sourcegraph/scip-clang) documents JSON compilation databases and SCIP output. The existing [oracle README](../../calibration/oracles/scip-clang/README.md) pins 0.4.0, Apache-2.0, arm64-darwin SHA-256 `ff042fbc8a029f09f4b69fc7692e290e21c52923593207ee52d4e7439473ec64`. [v0.4.1 release](https://github.com/sourcegraph/scip-clang/releases/tag/v0.4.1) arm64-darwin asset SHA-256 `34a2eb7a5ef9bbe9849cded59f4eb22a2331bd5b64a5cec2b17ee09b8aa515de` was downloaded and matched against release metadata in private preparation; both binaries' version guards passed, and both tag license files were Apache-2.0 with SHA-256 `96b8b909d18f94921214e3995e4c12483826b12ca64dd93fc710b87e1bc5a08a`. The public GitHub repository query `scip-cpp in:name` was retried successfully: five hits, all numeric SCIP or SciPy examples rather than a code-intelligence producer. [LSIF.dev](https://lsif.dev/) lists [lsif-clang](https://github.com/sourcegraph/lsif-clang), whose own README deprecates it in favor of scip-clang. The archived v0.1.0 tag was resolved through the public tag API; its output is LSIF, not SCIP. This audit has not tried a legacy LSIF adapter. | Both 0.4.0 and 0.4.1 completed all entries of the same full compiler databases with zero errored translation units. Four completed comparisons have equal graded metrics; diagnostic count differences remain. Do not silently edit the 0.4.0-guarded `run.sh`. |
| Language servers | [clangd index design](https://clangd.llvm.org/design/indexing) has background and static indexes with references, driven by the compilation database; [ccls](https://github.com/MaskRay/ccls) offers definitions, references and call hierarchy, Apache-2.0. The local Apple clangd is 21.0.0 (`clang-2100.3.34.2`). The public [ccls 0.20261004 release](https://github.com/MaskRay/ccls/releases/tag/0.20261004) was pinned by that named release and its public tag API; its tag license is Apache-2.0, SHA-256 `c71d239df91726fc519c6eb72d318ec65820627232b2f796219e87dcf35d0ab4`. Both remain unmeasured alternate producers. | Untried. Check exportable location/range and type facts, completeness and cost. An interactive LSP response is not automatically whole-population truth. |
| Registry parsers/bindings | [PyPI libclang 18.1.1](https://pypi.org/project/libclang/18.1.1/) bundles a macOS arm64 library; [PyPI clang bindings 21.1.7](https://pypi.org/project/clang/21.1.7/) are bindings and do not bundle it. LLVM's Clang AST dump is already present in the compiler route. | Untried as alternate adapters. A version-matched libclang cursor walk could test ranges and definitions, but needs its own pinned binary and whole-unit coverage; bindings alone are not a producer. |
| Runtime call tracing | [LLVM XRay](https://llvm.org/docs/XRay.html) instruments executed functions and needs a built runnable program; its documented platform list does not include macOS. | Out of scope for static, complete call-site truth on these macOS subject sources. It cannot prove that unexecuted template or macro sites are absent. |

Source-search recovery on 2026-10-26: public requests to `GET /search/repositories?q=scip-cpp%20in%3Aname`, `GET /repos/MaskRay/ccls/releases/latest`, `GET /repos/MaskRay/ccls/git/ref/tags/0.20261004`, and the pinned raw ccls license succeeded after the earlier sandbox access failures. `GET /search/repositories?q=lsif%20clang%20in%3Aname` returned four repositories including the official archived lsif-clang; `GET /repos/sourcegraph/lsif-clang/tags?per_page=1` supplied its v0.1.0 pin. These searches close access gaps, not candidate measurements. The [SCIP producer list](https://github.com/scip-code/scip#tools-using-scip), [Sourcegraph indexer list](https://sourcegraph.com/docs/code-navigation/writing-an-indexer), and [LSIF list](https://lsif.dev/) were checked. No claim that all producers are exhausted is made.

The parser sources checked include the [official grammar](https://github.com/tree-sitter/tree-sitter-cpp), [Unreal fork](https://github.com/taku25/tree-sitter-unreal-cpp), [WillBooster grammar 6.0.2](https://crates.io/crates/willbooster-tree-sitter-cpp/6.0.2), and [matching runtime 7.0.5](https://crates.io/crates/willbooster-tree-sitter/7.0.5). Crates.io registry API access failed, but the sparse index and exact static archives succeeded. The grammar crate records a dirty source tree, so its archive checksum supplies its immutable identity; a Git commit is not substituted. Grammar licence hashes are `2e0110e07abef7c2548b26ec9d6969775617ca539a0dc8dbeeb14d6452c711d1` (official HEAD), `ef02f1077281d63262946e50ec461ba72efda41e51084c887379b59397b96a6d` (Unreal), and `2bb45b424e2a497412ab9eb903554eebb9847e4f70bc81eecd2609817301679f` (WillBooster); runtime licence hash is `f1e1e62c6e1119d27f5ba38a0cbdae0788fc00f1d0bdf71cb7710ca5d2421d1c`. [Committed parser receipts](../bench/readings/cpp-audit-2026-10-26/) preserve complete/failed status and every source/population/reading identity. Compiler, LSP and tracing alternatives cannot cure the independently measured parser-admission failure. Their untried metric adapters are not classified as measured failures.

The experimental `O1504` in `1922c8606` is a withdrawn draft unrelated to main's `O1504` (graph-cache eviction). Controller review m1 authorizes the parser-fault record as O1508, allocated against origin/main `4e7eea67a` (highest O1507). No history is rewritten.

ENGINE-history note (M1): `95c59df54` and `b9d22930d` are experiment-only product-swap commits, each reverted immediately by `1283b60ed` and `807922cf4`. They must never be cherry-picked or treated as shipping product states. The current product lock/blob remain byte-exact. Their official/Unreal receipts are being replaced by isolated audit-tag replays; no new production swap is permitted.

## Controls and reproducible commands

Collector controls cover all 17 phase/provenance tests, optional `ranges:A` without raising the metric's tier, complete dependency attestation, legacy receipt refusal and failing-phase publication. Streaming entry/fanout/byte/log/depth caps refuse whole units. Native JSON/log caps are 256/64 MiB. Each command's load wait is outside its execution cap. A first exploratory run lacked dependency provenance and a first grade validator rejected an optional range family; those outputs remain diagnostic and were replaced by fresh production, never rewritten receipts.

Screen controls cover raw/selected ERROR/MISSING partitions, symlinks/traversal/duplicates, revision and output-replacement refusal, JSON/token/depth and source/file/total caps. Production caps are 256 MiB JSON,20000 files,16 MiB/source,512 MiB total,2 million tokens,depth 64. Mutation replacing preparse with an empty target fails `TestExactPopulationAndSyntaxPartitions`; restored tests pass. `TestSCIPTruthIndependentOfExtractorNodes` changes mapping while raw SCIP truth stays fixed, then changes truth when the SCIP definition changes.

Use committed commands with explicit pinned workspace inputs, never a private driver as evidence:

```sh
python3 calibration/oracles/scip-clang/cpp-report_test.py
python3 calibration/oracles/scip-clang/cpp-report.py --audit-root "$AUDIT_ROOT" --screen-binary "$SCREEN_BINARY" --out "$REPORT_OUT"
python3 calibration/oracles/scip-clang/cpp-audit_test.py
bash scripts/fathom-go.sh test -race -count=1 ./calibration/oracles/scip-clang/cpp-screen
bash scripts/fathom-go.sh test -race -count=1 -tags=fathom_grammar_audit -run TestGrammarAudit ./internal/extractor/tsengine
bash scripts/fathom-go.sh test -race -count=1 -tags=fathom_grammar_audit ./calibration/oracles/scip-clang/cpp-screen
python3 calibration/oracles/scip-clang/build-willbooster_test.py
python3 calibration/oracles/scip-clang/build-willbooster.py --work-dir "$BUILD_WORK" --out-dir "$MODULE_OUT" --gzip
```

The original production uses a clean checkout at `0deba117abaadee0343a58a1645f9232a92b6167`; the indexbench binary must be compiled there. Pinned subject/build/tool inputs must already be prepared as above. `WORK`, `INDEXES`, `READINGS` and `LOCK` are disjoint ignored-workspace paths. Run the three phases sequentially (load gating and nice are enforced internally):

```sh
python3 calibration/oracles/scip-clang/cpp-audit.py index --subjects-root "$SUBJECTS" --builds-root "$BUILDS" --work-dir "$WORK" --lock-dir "$LOCK" --indexes-dir "$INDEXES" --scip-binaries-root "$SCIP_BINARIES"
python3 calibration/oracles/scip-clang/cpp-audit.py native --subjects-root "$SUBJECTS" --builds-root "$BUILDS" --work-dir "$WORK" --lock-dir "$LOCK" --indexes-dir "$INDEXES" --clang "$CLANG"
python3 calibration/oracles/scip-clang/cpp-audit.py grade --subjects-root "$SUBJECTS" --builds-root "$BUILDS" --work-dir "$WORK" --lock-dir "$LOCK" --indexes-dir "$INDEXES" --readings-dir "$READINGS" --indexbench "$INDEXBENCH" --ctags "$CTAGS" --clang "$CLANG"
```

The WillBooster screens use a separate clean checkout and tagged binary at `8f90d5e597eb88413b68b327c830b40788ef46c9`. A later documentation tip is not that measured source revision. Fmt/json/spdlog each run with each frozen legacy/full reading; `UNMATCHED` is that subject's v0.4.1 grade receipt. The exact command is:

```sh
bash scripts/fathom-go.sh build -tags=fathom_grammar_audit -o "$SCREEN" ./calibration/oracles/scip-clang/cpp-screen
"$SCREEN" -candidate -candidate-lock "$MODULE_OUT/grammars.lock.json" -candidate-module "$MODULE_OUT/cpp.wasm" -subject "$SUBJECT" -subject-name "$SUBJECT_NAME" -reading "$READING" -unmatched "$UNMATCHED" -out "$SCREEN_OUT"
```

Successful screens exit 0. Full fmt/json fail with `parse_fault` receipts and nonzero exits; keep those failures. Replay refuses dirty/stale binaries and existing output rather than changing their identities to fit. The report producer then runs from its committed source against the immutable six original 0de screen receipts and exact baseline binary.

The external loader refuses forged self-consistent locks, wrong tool hashes, symlinks and changed production lock/blob paths. `TestGrammarAuditWholeModuleCap` supplies an otherwise valid identity: 32 MiB reaches WASM compilation, while 32 MiB+1 is refused before compilation. The full tagged tsengine race suite passes; the final helper policy/cap delta passes focused tagged-engine and both tagged/default screen race suites.

Report controls: 13 tests pass and the real frozen-input replay completes. Mutation removing the screen-binary identity comparison fails `ReportTests.test_screen_binary_and_grammar_identity_refuse`; restored bytes pass all 13. An initial mutation command named the wrong test class and failed at discovery; it is not counted as mutation evidence. The corrected run requires the expected assertion failure.

`AUDIT_ROOT` is a fresh replay of the audit-r3 directory layout, `SCREEN_BINARY` is the original clean 0de screen (SHA-256 `f7c4c351006b4b59cd76eedef27f03ba093e0ec4eae088f15e8b972fb496cdd4`), and `REPORT_OUT` is a new ignored-workspace file outside that input directory. Producer inputs and the existing report must not be rewritten. Module build tools are workspace-local pinned Zig/wasm-opt binaries, exposed through PATH; both Zig caches are explicitly set under the fresh build work directory and build commands run there (m4).

## Disposition

Measured parser limit: the incumbent and all three named alternatives fail the unchanged 5% admission gate. Keep canonical support and readings unchanged. Retry when a source-pinned grammar or per-dialect prepass completes the full 679-file population with at most 33 selected ERROR-or-MISSING files and no parser faults; then rerun affected readings, throughput/cost gates and a newly frozen holdout. Any prepass must preserve every oracle-graded fact. No extractor/runtime change is adopted by this audit, and no claim that all future producers are exhausted follows. The historical holdout remains SPENT: its pin/criterion now live in `docs/bench/cpp-historical-holdout.yaml`, outside canonical aggregation; no subject was opened or revalidated by this move. Frozen baseline regrades still need the owner scope ruling required by recipe E2-R1/D20.

## Binding review R1

M1 and M2 plus m1 through m9 are tracked explicitly. The canonical reading and product grammar stay unchanged.

| ID | Repair / verification |
|---|---|
| M1 | Isolated source-pinned builds and official/Unreal re-screens replace product-swap-dependent receipts; historical commits disclosed above. Replay pending. |
| M2 | LB-F473 is restored verbatim. The spent pin follows the existing docs/bench convention, with its URL and a shard pointer. No new validation. |
| m1 | Withdrawn draft O1504 distinguished from main's record; next free O1508 allocated under the controller ruling. |
| m2 | Optional lang-retime load log brackets all three runs and records clean measuring-binary identity. Four fresh copies will publish rates and loads; original unknown loads remain unknown. Replay pending. |
| m3 | Every phase and publication refuse dirty source trees; the summary binds historical grade/driver commit and all tool hashes separately from current collector identity. |
| m4 | Both Zig caches and cwd are explicitly under each fresh work directory. Rebuild pending. |
| m5 | Real outside-workspace file checks the exact containment error; valid metadata reaches module-byte refusal; instantiation counts distinguish identity from ABI refusal. |
| m6 | Python capped reads consume at most cap+1; tests cover equality/overflow/growth, input count, tenancy, SCIP bytes, aggregate budget and output bytes. Test artifacts stay in workspace. |
| m7 | task test now vets both audit-tag packages; the optional code is compiled by normal CI. |
| m8 | Exact aggregate counts, pooled ratios and tier labels are checked against the readings. |
| m9 | Durable assessment and trigger spacing repaired. |
