# OpenCL C triage and implementation proposal

G-R1 permits triage now and implementation after GLSL lands, then Slang. This branch adds no dialect, grammar blob, canonical corpus, reading or support level. The corrected [queue proposal](../../calibration/oracles/opencl-c/queue-proposal.json) reuses the existing CUDA/clang pipeline and withdraws the outcome-selected SDK+HandsOn pool. The dedicated grammar fails natively on the seven previously screened OSI-licensed primary populations: 133/378 ERROR-or-MISSING files (35.19%). The new outcome-blind pool rule is frozen before any further screen or grammar-patch measurement; no passing gate or support level is claimed.

## Frozen inputs and subject search

[Grammar and original subject pins](../../calibration/oracles/opencl-c/pins.json) describe the original SDK/hashcat screen. [The first subject search](../../calibration/oracles/opencl-c/subject-search.json) was committed at `c3ab954d9` before parsing its six candidates. [The additional search](../../calibration/oracles/opencl-c/subject-search-r3.json) was committed at `ee60410a0` before parsing its three candidates. Both retain search sources, zero-population misses and access gaps. Every result covers **every tracked regular lowercase `.cl`**, including include fragments, with no sampling or file exclusions. Public evidence contains counts/hashes rather than subject content, authors or inventories.

| Grammar | Pin | License and scope |
|---|---|---|
| lefp/tree-sitter-opencl | `8e1d24a57066b3cd1bb9685bbc1ca9de5c1b78fb` | MIT from pinned LICENSE; package metadata says ISC. Three-commit repository, 2023 HEAD, incomplete-coverage disclaimer. |
| tree-sitter/tree-sitter-c v0.24.2 | `b780e47fc780ddc8da13afa35a3f4ed5c157823d` | MIT; maintained unmodified C baseline, without an OpenCL layer. |

PyPI `tree-sitter-opencl` 0.0.2 is traceable to Time-Coder's `f1aabdf2` release. Its parser matches that repository and its grammar matches the lefp pin, so it is a repackaging rather than a third grammar. The untried alternative is a **P-R1 patch over pinned tree-sitter-c**, not an unnamed maintained fork. No grammar is selected for shipping and no ceiling is asserted.

## Complete native screens

The table reports files with ERROR **or** MISSING, not just ERROR. Exact ERROR-only/MISSING-only/both splits, bytes, full tracked/untracked before/after manifests and native-library hashes are in the [new receipts](data/opencl-c-triage-correction-2026-10-26/subject-outcomes.json) and original [SDK](data/opencl-c-triage-2026-10-26/native-sdk.json)/[hashcat](data/opencl-c-triage-2026-10-26/native-hashcat.json) receipts. Every screen has zero native faults.

| Whole population | Files | Distinct blobs | Dedicated ERROR-or-MISSING | C baseline ERROR-or-MISSING | Disposition |
|---|---:|---:|---:|---:|---|
| Khronos SDK | 10 | 9 | 1/10 (10.00%) | 10/10 (100%) | Primary under the licence/role rule |
| Hashcat | 1,349 | 1347 | 1,344/1,349 (99.63%) | 1,344/1,349 (99.63%) | Diagnostic macro stress, outside proposed pool |
| darktable | 46 | 46 | 3/46 (6.52%) | 46/46 (100%) | Primary, failure retained |
| OpenCV | 110 | 110 | 75/110 (68.18%) | 110/110 (100%) | Primary, failure retained |
| ArrayFire | 103 | 103 | 8/103 (7.77%) | 98/103 (95.15%) | Primary, failure retained |
| clBLAS | 40 | 40 | 40/40 (100%) | 40/40 (100%) | Primary, failure retained |
| Rodinia | 31 | 28 | 3/31 (9.68%) | 31/31 (100%) | Primary, failure retained |
| pocl | 1,016 | 1011 | 793/1,016 (78.05%) | 847/1,016 (83.37%) | Builtin/generated/negative mixture, diagnostic role frozen before parsing |
| SHOC | 38 | 38 | 3/38 (7.89%) | 38/38 (100%) | Primary, failure retained |
| HandsOnOpenCL Exercises-Solutions | 16 | 12 | 0/16 (0%) | 16/16 (100%) | Diagnostic only: CC-BY-3.0 is not OSI-approved |
| NVIDIA legacy examples | 41 | 41 | 5/41 (12.20%) | 41/41 (100%) | Diagnostic only: unresolved licence |
| PyOpenCL | 8 | 8 | 5/8 (62.50%) | 7/8 (87.50%) | Primary, failure retained |
| clinfo | 0 | 0 | n/a | n/a | Zero-population diagnostic, not calibration |

The original SDK+hashcat proposal was infeasible: dedicated `(1+1344)/(10+1349) = 1345/1359 = 98.97%`, C `(10+1344)/1359 = 99.63%`. Separate reporting never fixes that pooled gate. Hashcat's diagnostic role was assigned after its round-1 screen because computed includes and qualifier macros require a compiler-evidenced context; round-1 review accepted that structural exception. Applied without the exception, the historical inclusive pool fails at **1,482/1,735 (85.42%)** dedicated. Any prepass must remove nothing the oracle grades. No evidenced compiler context or prepass exists here. Every future patch measurement must report hashcat's dedicated and C rates beside the fixed eight-project pool.

The **SDK+HandsOn pairing is withdrawn**. Its numerical 1/26 (3.85%) was selected after the candidate outcomes were known and establishes no gate pass. HandsOn is CC-BY-3.0, which is not an OSI-approved software licence. Design §13.3 step 16 requires an OSI-approved subject licence; [the Vue corpus precedent](../../calibration/corpus.d/vue.yaml) already excludes CC-BY documentation on that basis. HandsOn remains diagnostic only, alongside the unresolved NVIDIA EULA subject. No licence exception is requested or assumed.

[pool-rule.json](../../calibration/oracles/opencl-c/pool-rule.json) closes the primary membership using repository identity, fixed pin, OSI licence eligibility, nonempty exact-lowercase `.cl` population, diagnostic roles recorded before candidate screens and the named hashcat exception described above. It never reads parse results, compiler success, oracle coverage or duplicate counts to select any other subject or file, and it forbids future post-screen reclassification. The initial membership rule was registered after seven primary outcomes were public and before the PyOpenCL screen. The R3 amendment clarifies the hashcat exception after those baselines and the first implementation trials; membership is unchanged. Its new digest governs subsequent patch measurements, without relabelling earlier receipts. It does not pretend the historical pool was preregistered. Every future grammar candidate must use this complete unchanged pool.

The already measured primary projects are the SDK, darktable, OpenCV, ArrayFire, clBLAS, Rodinia and SHOC: **133/378 (35.19%)** dedicated ERROR-or-MISSING files. C is **373/378 (98.68%)**. PyOpenCL joined under the same licence rule, pinned in commit `3e56917af` before its first screen. It has 5/8 dedicated and 7/8 C files in error, with zero faults. The complete fixed eight-project pool is now **138/386 (35.75%)** dedicated and **380/386 (98.45%)** C, so both fail. The [native pool receipt](data/opencl-c-pool-2026-10-26/native-pool.json) binds the original rule SHA-256 and the pre-PyOpenCL commit; its historical policy identity is retained after the R3 amendment. The original nine search outcomes remain visible, including every failure. Duplicate contents stay in file denominators. The [file/blob audit](data/opencl-c-pool-2026-10-26/pool-counts.json) reports 386 primary files, 382 distinct blobs and 4,256,615 bytes; each subject's counts are shown above. SDK has 10 files/9 blobs, Rodinia 31/28, HandsOn 16/12, hashcat 1,349/1,347 and pocl 1,016/1,011. No duplicate is removed. No subject is selected because it happens to bring a ratio under 5%.

The route is **P-R1 grammar repair**, not another cleaner subject. General address-space-qualified casts, statement attributes, `__inline`, preprocessor recovery and inline assembly need authored regression coverage. The patch must be evaluated on the frozen production pool, retaining all files. G-R6 currently authorizes no template or macro prepass; any future prepass requires independent oracle validation. This triage does not begin the implementation before G-R1 opens it.

These are native diagnostics, not WASM tenancy, throughput or accuracy. Both grammars use their committed parser/header without regeneration. Raw Git blob identities and captured immutable bytes protect the complete population. Symlink targets and gitlinks are recorded without traversal, with submodule contents uncovered. OpenCV's first clean checkout failed the strict raw-byte guard because 68 repository attributes requested CRLF conversion. A separate checkout with private info-attributes disabling text/filter/encoding conversion reproduced exact index blobs for all 7,372 regular files; zero case collisions, symlinks or gitlinks. The original checkout was untouched. The strict screen was not relaxed.

## Reuse CUDA's compiler oracle pipeline

The format owner's [OpenCL-Guide at 1269ba61](https://github.com/KhronosGroup/OpenCL-Guide/blob/1269ba618ecab9f4feb6f8a083bbb0f4c63eb56d/chapters/os_tooling.md) endorses Clang and its SPIR flow. The in-tree [CUDA database generator](../../calibration/oracles/scip-clang/cuda-compdb.py) already labels reconstructed compiler inputs tier B. OpenCL's [compdb.py](../../calibration/oracles/opencl-c/compdb.py) follows that pattern and feeds the **unmodified existing** tools:

- [clang-calls](../../calibration/oracles/clang-calls): written spelling/expansion mapping and call-site classes.
- [clang-ast-defs](../../calibration/oracles/clang-ast-defs): definition ranges and compiler-derived stand-ins, never stand-ins inferred from fathom output.
- [scip-clang/preproc](../../calibration/oracles/scip-clang/preproc): `active_lines` and include facts for the same compile context.

The corrected [SDK replay](../../calibration/oracles/opencl-c/replay-sdk.py) uses the pinned Apple Clang 21.0.0/CLT 27.0 compiler with a minimal snapshot of its two pinned OpenCL resource headers. All ten SDK units complete at tier B: **23 function ranges, 173 classified written sites = 147 resolved + 26 declarations**, and 10 active-line roots (89 ranges, 546 lines). The [fresh pipeline receipt](data/opencl-c-pool-2026-10-26/pipeline-sdk.json) publishes aggregate facts and groups the path-bound database and capture-receipt hashes as diagnostic identities. It was run into a previously absent nested output directory; portable raw oracle-output hashes match the historical replay. The historical receipts now label those digest limitations explicitly. Raw outputs remain private. Compiler/header pins are checked around each consumer, and input snapshot/database identity is checked throughout. This is pipeline capability evidence, not a fathom accuracy grade.

The original [compiler_probe.py](../../calibration/oracles/opencl-c/compiler_probe.py) is explicitly a **historical throwaway**, never to extend into a grading oracle. Its 23 bodies/19 kernels remain capability facts, but its 155 raw CallExpr nodes (6 source + 149 external) include eight repeated macro expansions. One written macro body call expands nine times. The existing `clang-calls` already resolves that distinction, yielding 147 written resolved sites. The historical [receipt](data/opencl-c-triage-2026-10-26/compiler.json) is retained without relabeling raw AST counts as written truth. Its AST hashes vary across processes.

The [host-context record](../../calibration/oracles/opencl-c/sdk-contexts.json) records SDK CL2.0/CL3.0 choices, reduce's work-group/sub-group defines, blur's subgroup exchange defines and reduce's appended generated helper source. The chosen baseline is CL3.0/SPIR64 with no defines, not a claim that host context is absent. Implementation must filter **both oracle truth and extractor sites** with the same `preproc.active_lines`, rather than scoring inactive regions as false positives. No all-device/configuration union is claimed.

[CUDA's driver](../../calibration/oracles/scip-clang/cuda-samples.sh) also provides the link-unit precedent: source grouping comes from independent build facts. OpenCL grouping must come from evidenced `clCreateProgramWithSource`/`clBuildProgram` assembly, including appended sources, never one inferred program per file. No link-unit truth or L3 claim exists in this proposal. The lowest reconstructed tier remains B and caps L2 at good.

The existing scip-clang runner deliberately pins 0.4.0 (`1860adf8`); current 0.4.1 (`14423f07`) is recorded rather than silently substituted. `-x cl` indexing is the cheap next probe, still untried. clangd's compile-database route, the Khronos guide, package provenance, SCIP/LSIF searches, language servers and runtime source/options capture are covered in [SEARCH.md](../../calibration/oracles/opencl-c/SEARCH.md), including misses and untried routes. No ceiling follows from those search misses.

## Implementation gates and review dispositions

ADR-048 data-only dialect/independent reading rules, I19 evidence privacy, I23 function-reference truth, I24 disclosed dropped targets and I25 readable-file accounting apply. No applicable revisit trigger is shown to have fired. No runtime service or decision model is added. The controller owns queue activation and plan proof.

After GLSL lands, repair and reproduce the frozen production pool in WASM, select the grammar, prove independent truth/explicit zero scope, and measure per-subject coverage, pooled errors, three-run median throughput, faults and per-file latency. `.cl` also belongs to Cool and is deliberately unclaimed by Common Lisp: require positive OpenCL/C evidence, Lisp/Cool negatives and ambiguous refusal. Keep include fragments in tenancy even without standalone-unit truth. Extra suffixes, C++ for OpenCL and embedded host strings stay outside this initial proposal.

The Apple compiler remains a historical host-specific triage pin. [llvm-plan.json](../../calibration/oracles/opencl-c/llvm-plan.json) pins official LLVM 21.1.8 archive SHA-256 values for macOS ARM64 and Linux X64/ARM64. These archives were not downloaded or run. Implementation must verify the archive, pin its actual compiler/resource bytes and replay all oracle facts, without assuming Apple/upstream version parity.

| Finding | Correction |
|---|---|
| F1 major | Existing CUDA/clang pipeline selected and replayed; old AST counter explicitly historical, never graded or extended. |
| F2 major (round1) | Two pre-parse pin commits and complete screens retained; the round2 correction below withdraws the invalid replacement pairing. |
| F3 minor | Host options/source assembly recorded; shared active-line policy for truth and extractor. |
| F4 minor | Owner guide, traceable PyPI repackaging, clangd, current versus pinned scip-clang, named C-patch alternative. |
| F5 minor | README states lowercase `.cl` scope, rather than claiming an absent uppercase counter. |
| F6 minor | Portable discovery commands replace system paths; upstream archive plan remains an explicit implementation gate. |
| F7 minor | Assessment index entry is a table row with controller-owned activation. |

The [README](../../calibration/oracles/opencl-c/README.md) provides committed runnable screens, pipeline replay and focused tests. No product Go code is changed. Harness tests cover raw Git identity, optimized-Python pin refusal, immutable source copies, minimal resource isolation and context-override refusal. The new pipeline does not claim the independent-extractor mutation proof needed for a future support reading. Historical focused tests passed 12+4+10+7 at `8d186389b`. Round-2 corrections passed **44 tests** (12 triage, 4 historical probe, 10 compdb, 7 replay, 11 pool audit). The pre-fix replay failed all seven new guard tests and the corrected replay passed. Restoring the private-layout routing guard raises the current suite to **45 tests** (8 replay), all passing. Current X-R2 release-notes and full/staged merge-debris checks pass; X-R3 TestNoOrgSpecificNames passes (2.348s). The earlier scope compatibility race checks passed after the main merge (3.760s); this Python/docs delta does not change that engine. The X-R5 review finds only repository identifiers and synthetic fixtures; no subject people or private paths are in evidence. No subject content or inventories are published. Generated aggregate files and plan checkboxes remain untouched.

## Round 2 review corrections

R2-F1: outcome-selected pooling withdrawn. The fixed licence/role/pin rule retains every eligible production subject and routes the native 35.75% full-pool failure to P-R1. R2-F2: CC-BY-3.0 teaching code is diagnostic only under §13.3 and the Vue precedent, with no calibration admission. R2-F3: the replay creates missing output parents and labels path-dependent database/receipt digests as diagnostic run identities. R2-F4: the two unavailable repository names are recorded as named misses; PyOpenCL and clinfo are pinned in [the follow-up search](../../calibration/oracles/opencl-c/subject-search-r4.json), and the unreplayed source receipt has been dropped.

The pool audit’s isolated CLI first failed with `ModuleNotFoundError`; the sibling import was fixed and its entrypoint test passes. The replay’s nested-output test first failed with `FileNotFoundError`; the corrected entrypoint and full native compiler replay pass. These are harness integrity regressions, not product grading proofs. The original policy stayed unchanged through those baseline measurements. R3 records its later amendment separately and leaves every earlier native receipt bound to its original digest.

## Round-3 review corrections

R3-F1: the PHP assessment is restored byte-for-byte from `origin/main` in a new
commit. The normal secret scanner accepts the restoration with zero findings
and no baseline. The branch diff contains only OpenCL files and one OpenCL
assessment-index row. No other lane owns any remaining changed file.

R3-F2: hashcat's post-screen structural exception and the historical inclusive
1,482/1,735 failure are explicit. The amended policy was committed before further
production patch measurements, retains all eight primary members, forbids future
post-screen reclassification, and requires hashcat's dedicated/C rates beside
each future patch result. The original baseline receipts are not rewritten.

R3-F3: the private-layout routing guard is restored beside the nested-output
test. It checks that only `private/` is created at the work root, every layout
path stays below it, and the database directory is not pre-created. All 45
focused tests pass. R3-F4: public spacing is restored and the README accurately
describes a rule registered after seven outcomes, before PyOpenCL.

R4-F1: the nested-output test creates its ignored `workspace/` parent, so all
eight replay tests are reproducible from a fresh checkout.
