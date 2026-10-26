# C++ producer audit, open (2026-10-26)

The [committed C++ reading](../bench/readings/lang/cpp.json) remains a historical baseline, not a current ceiling or a new measurement. It used scip-clang 0.4.0 and Apple clang 21.0.0 over fmt 12.2.0, nlohmann/json 3.12.0 and spdlog 1.17.0: `def_mapping` .8988 (5,242/5,832), `typed_recall` .9656, `file_edge_precision` .9023 under per-subject W3, and `file_edge_recall` .5354. The governing [LB-Q-cpp-rest row](../plans/language-breadth.md) stays open. CUDA #4 and HLSL #5 have landed, so their pending integration is no longer a reason to defer this audit. No conclusion about the C++ ceiling follows from those landings.

## Population and controls to freeze before any run

The old [corpus shard](../../calibration/corpus.d/cpp.yaml) restricts fmt, json and spdlog with `-only` globs; the json `subpath: include` is another restriction. Those rules are valid history for comparing the committed reading, but are not the full tracked C++ population. A new baseline must first inventory every tracked subject file, the `.h` router's ownership, exclusions, and generated or ignored inputs under the actual discovery rules. It must report both a legacy-scope replay and a full-population run, never pool them as one metric. Do not change a corpus shard, select or inspect a holdout, or tune on a holdout until the criterion and membership are frozen.

For each of the three pinned subject commits in the shard, use the exact tracked-set command in [recipe §13.3.1](../design/language-breadth.md) before and after CMake/oracle work, with `SUBPATH=.` and separate receipts under ignored `workspace/artifacts/lang-cpp/`. Record SHA-256, files, bytes, symlinks, gitlinks, untracked/ignored count and the reading's per-subject files/bytes. The manifest is an approximation; compare harness discovery and state every exclusion. Build trees and outputs belong in `workspace/` or a disposable subject copy, and a database placed outside a subject must be made visible to the product's documented include resolver before its result can grade L3. Keep subject file lists private.

## X-R1/X-R4 source ledger (read-only triage)

Sources checked 2026-10-26; a candidate is **untried** here until the pinned full-population run. This ledger records an option, not its fitness for the grade.

| Route | Primary source and result | Status / next check |
|---|---|---|
| Own compiler | [Clang command reference](https://clang.llvm.org/docs/ClangCommandLineReference.html), [JSON compilation database](https://clang.llvm.org/docs/JSONCompilationDatabase.html); local `/usr/bin/clang` is Apple clang 21.0.0 (`clang-2100.3.34.2`). Existing `clang-calls`, `clang-ast-defs`, and build-flag `preproc` adapters cover written calls, definition ranges, includes and active lines. | Available, historically measured. Re-run exact compilation entries, log failed units and compiler diagnostics; check special/defaulted members, macro definitions, declaration/definition ranges, and preprocessing before attributing misses. |
| SCIP/LSIF | [Sourcegraph's indexer list](https://sourcegraph.com/docs/code-navigation/writing-an-indexer) names scip-clang for C/C++; [scip-clang repository](https://github.com/sourcegraph/scip-clang) documents JSON compilation databases and SCIP output. The existing [oracle README](../../calibration/oracles/scip-clang/README.md) pins 0.4.0, Apache-2.0, arm64-darwin SHA-256 `ff042fbc8a029f09f4b69fc7692e290e21c52923593207ee52d4e7439473ec64`. [v0.4.1 release](https://github.com/sourcegraph/scip-clang/releases/tag/v0.4.1) arm64-darwin asset SHA-256 `34a2eb7a5ef9bbe9849cded59f4eb22a2331bd5b64a5cec2b17ee09b8aa515de` was downloaded and matched against release metadata in private preparation; both binaries' version guards passed, and both tag license files were Apache-2.0 with SHA-256 `96b8b909d18f94921214e3995e4c12483826b12ca64dd93fc710b87e1bc5a08a`. The public GitHub repository query `scip-cpp in:name` was retried successfully: five hits, all numeric SCIP or SciPy examples rather than a code-intelligence producer. [LSIF.dev](https://lsif.dev/) lists [lsif-clang](https://github.com/sourcegraph/lsif-clang), whose own README deprecates it in favor of scip-clang. The archived v0.1.0 tag was resolved through the public tag API; its output is LSIF, not SCIP. This audit has not tried a legacy LSIF adapter. | Both 0.4.0 and 0.4.1 completed all entries of the same full compiler databases with zero errored translation units. Grading remains **untried**; an error-free index alone proves no Fathom metric. Do not silently edit the 0.4.0-guarded `run.sh`. |
| Language servers | [clangd index design](https://clangd.llvm.org/design/indexing) has background and static indexes with references, driven by the compilation database; [ccls](https://github.com/MaskRay/ccls) offers definitions, references and call hierarchy, Apache-2.0. The local Apple clangd is 21.0.0 (`clang-2100.3.34.2`). The public [ccls 0.20261004 release](https://github.com/MaskRay/ccls/releases/tag/0.20261004) was pinned by that named release and its public tag API; its tag license is Apache-2.0, SHA-256 `c71d239df91726fc519c6eb72d318ec65820627232b2f796219e87dcf35d0ab4`. Both remain unmeasured alternate producers. | Untried. Check exportable location/range and type facts, completeness and cost. An interactive LSP response is not automatically whole-population truth. |
| Registry parsers/bindings | [PyPI libclang 18.1.1](https://pypi.org/project/libclang/18.1.1/) bundles a macOS arm64 library; [PyPI clang bindings 21.1.7](https://pypi.org/project/clang/21.1.7/) are bindings and do not bundle it. LLVM's Clang AST dump is already present in the compiler route. | Untried as alternate adapters. A version-matched libclang cursor walk could test ranges and definitions, but needs its own pinned binary and whole-unit coverage; bindings alone are not a producer. |
| Runtime call tracing | [LLVM XRay](https://llvm.org/docs/XRay.html) instruments executed functions and needs a built runnable program; its documented platform list does not include macOS. | Out of scope for static, complete call-site truth on these macOS subject sources. It cannot prove that unexecuted template or macro sites are absent. |

Source-search recovery on 2026-10-26: public requests to `GET /search/repositories?q=scip-cpp%20in%3Aname`, `GET /repos/MaskRay/ccls/releases/latest`, `GET /repos/MaskRay/ccls/git/ref/tags/0.20261004`, and the pinned raw ccls license succeeded after the earlier sandbox access failures. `GET /search/repositories?q=lsif%20clang%20in%3Aname` returned four repositories including the official archived lsif-clang; `GET /repos/sourcegraph/lsif-clang/tags?per_page=1` supplied its v0.1.0 pin. These searches close access gaps, not candidate measurements. The [SCIP producer list](https://github.com/scip-code/scip#tools-using-scip), [Sourcegraph indexer list](https://sourcegraph.com/docs/code-navigation/writing-an-indexer), and [LSIF list](https://lsif.dev/) were checked. No claim that all producers are exhausted is made.

## Bounded next measurement

1. Reserve the single heavy slot at the shared scratch path `workspace/tmp/lang-measure.lock`, per recipe §13.0; release it after the heavy phase. The private setup helper `workspace/tmp/core-t1/cpp-baseline-setup.py` completed the clone, pin, manifest and CMake preparation; its count/hash summary is retained under ignored `workspace/artifacts/lang-cpp/setup-summary.json`. The six producer runs completed; native companions and grading remain unmeasured. Confirm the three subject SHAs, isolated CMake 3.31.6 and oracle versions and hashes. CMake configure with the existing subject options in `cpp.yaml`, export `compile_commands.json` to external build dirs, and record entry counts, unique source units, missing units and out-of-subject entries. A later compiler probe must record failed compilation units. A subject-root `compile_commands.json` symlink exposes the same database to Fathom's resolver; it is an untracked non-language file and is disclosed in the after receipt. The prior spdlog database kept 34 subject entries after filtering; do not turn that filter into a file-population limit.
2. Run the exact §13.3.1 tracked-set receipts around configuration. Compare the full tracked C++ set to harness discovery and the historical `-only` sets. Record ERROR-only, MISSING-only and union file counts. Keep receipts, compiler logs, databases, indexes and raw oracle files under ignored `workspace/`.
3. Replay the 0.4.0 baseline on the identical legacy scope; then measure the full population with 0.4.0 and, after SHA/license verification, 0.4.1. Use the same compilation database, clang-calls, preprocessor, AST ranges, ctags, grammar blob and extractor revision. Report per-subject numerators/denominators and tier caps for definition mapping, typed recall, file-edge precision/recall, calls, imports and ranges. Classify unmapped definitions and blind file pairs by cause; do not commit filenames, subject content or wholesale file lists.
4. Only after those comparisons, decide whether extractor changes or an independent producer are needed. Before any ceiling, complete the missing source searches, independent-truth perturbation, cap/mutation checks and a criterion-frozen holdout under recipe §§13.0.1, 13.4.0 and 13.5.1. No holdout is selected or read in this preparation.

The next claim can be made only against a named population, immutable pins and exact runnable commands. `Verified:` must eventually cite committed recipe/tests and their results, not this workspace preparation.


## Completed preparation, no new grade

The exact tracked-set recipe gave identical before/after manifest hashes for
each pinned subject. These are discovery approximations, not harness file
counts. Configuration added one non-language compiler-database symlink per
subject and no generated language files. All database source units exist.

| Subject | Tracked-set files / bytes | Manifest SHA256 | Database entries | Unique subject / outside units |
|---|---:|---|---:|---:|
| fmt | 142 / 3093051 | `5805a5fe1abf223ee7dffc409d74f6e44e909c06353b4b442744597b039b675f` | 51 | 31 / 0 |
| json | 1163 / 15958746 | `4111e73b93fc872ea8b53b91cd0d5eecc83cba1f9243dce087df8e862ba40bba` | 93 | 78 / 0 |
| spdlog | 177 / 1210549 | `61cee59630f5a71436abafddc625992b2288c5693ecf81f2189ab3dd3775a61d` | 141 | 34 / 107 |

The 107 outside spdlog units remain disclosed. The full database is retained;
no translation-unit filter or source-population cut is adopted. Both pinned SCIP producers have now completed every entry of these databases: fmt 51, json 93 and spdlog 141, with zero errored translation units in each run. The index metadata points to its own subject and all document paths checked are subject-relative despite a warning about the enclosing checkout. The next phase compares the frozen indexes through the current harness on the legacy and full populations, with native compiler companions. No holdout has been selected or read.

The scratch subjects' local Git exclude files name only the non-language `/compile_commands.json` link in addition to their existing rules. This makes the committed retimer's nonignored-clean check compatible with the product's database resolver. The exact manifest recipe still inventories ignored entries, so the same one symlink remains disclosed; no language source is hidden or removed. Every other untracked path remains subject to the retimer's cleanliness refusal.

## Committed measurement controls, no new grade

The committed `calibration/oracles/scip-clang/cpp-audit.py` driver separates pinned
index production, native companions and grading. It attests the full compilation
databases, producer metadata, source/tool hashes and pre/post identities. It refuses
missing families per subject, stale/dirty grading binaries, changed inputs, stopped
native units, or partial output. Legacy and full populations stay in distinct files.
The committed retimer supplies all three CPU rates and machine-class disclosure.
L1 and range agreement retain tier B because Ctags enumerates that population;
the AST range companion does not raise its tier.

Synthetic process and provenance controls: 12 tests pass. Stdout, stderr and input
caps refuse exactly 101 bytes at a test cap of 100; the equality boundary is accepted
for input hashing. Native production uses 256 MiB JSON and 64 MiB logs; no cap has
been spent on a subject yet. A child holding a pipe after its leader exits is killed
on timeout. The scope mutation changed `grade_command` to apply legacy `-only`
filters to the full run. `python3 calibration/oracles/scip-clang/cpp-audit_test.py
DriverTest.test_grade_scope_and_tool_flags` failed with `FAIL:
test_grade_scope_and_tool_flags`; restoring the exact driver bytes passed.
These controls prove the orchestration checks, not a C++ support level.

## Dependency provenance repair before publication

The first six index productions remain diagnostic. They hashed the subject
checkout, compilation database, compiler and producer, but did not hash the
107 outside spdlog compilation sources or fetched dependency headers. A
source hash taken later cannot attest their index-time bytes. No source-frozen
grade or ceiling follows from those indexes. The native exploratory repeat
was interrupted after this gap was established; its receipt stays failed,
with partial counters retained rather than published as a complete run.

The only fetched source checkout is Catch2 v3.5.0, clean at commit
`53d0d913a422d356b23dd927547febdf69ee9081`, as declared by spdlog's own build.
The collector now binds the complete build input tree,
fetched source pin, generated include inputs and compiler database at every
index, native and grade boundary. It will reject old receipts without those
identities. All six indexes and native companions must run afresh from the
repaired committed collector before publication; no retrospective receipt
repair is allowed. The subject manifests still match their original hashes.

The exploratory counters are limited to what each adapter actually walks.
Fmt's preprocessor and calls use 36 normalized commands from 51 entries;
its ranges dump 31 unique files and skip 20 repeated entries. Json uses
93 commands for calls/preprocessing, with 78 range dumps and 15 repeated
entries skipped. All those completed families report zero stopped/errored
units. Spdlog's 141 preprocessor and call commands completed with zero stopped
units; its range collection was interrupted. The range adapter keeps the
first command per source file, so no all-configuration range-coverage claim
is made. Ctags remains the range metric's tier-B population.

Catch2's license is Boost Software License 1.0, license-file SHA-256
`c9bff75738922193e67fa726fa225535870d2aa1059f91452c411736284ad566`.
The dependency manifest excludes Git metadata, rejects resolved compiler
inputs and links into that excluded tree, and caps total entries as well as
regular-file bytes, directory fanout and depth. It publishes counts and hashes,
not dependency file lists or Git identities other than the pinned commit.
The compiler/SDK arguments retain their separate toolchain identity; this
manifest does not claim to hash the entire operating-system SDK.

The external-source mutation control was RED: changing an outside source
left the old index stamp unchanged. Receipt-round-trip and manifest-boundary
controls also reproduced the tuple/list and missing-entry-cap failures. After
repair, all 17 Python controls pass, including outside source/header mutation,
legacy receipt refusal, JSON replay, entry/byte/fanout caps and metadata paths.
The independent final review found no remaining material issue. No new
source-frozen index, native receipt or grade has been produced by the repaired
collector yet. These controls validate the method, not a language level.

The directory-fanout control also verifies streaming refusal: with a cap of
two entries and seven available, the old walk consumed all seven before
refusing. The repaired walk reads exactly three and refuses before sorting.
The earlier live stdout/stderr caps remain unchanged. One exploratory
control attempt detected its output cap and then received a process-group
permission denial during cleanup; the full committed suite subsequently
passes. This is retained as a failed diagnostic, not claimed as an oracle
production result. No unsupported signal-handling change was made.
