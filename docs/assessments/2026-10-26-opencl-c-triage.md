# OpenCL C triage and implementation proposal

G-R1 permits triage now and implementation after GLSL lands, then Slang. This branch adds no dialect, grammar blob, canonical corpus, reading or support level. The corrected [queue proposal](../../calibration/oracles/opencl-c/queue-proposal.json) reuses the existing CUDA/clang pipeline and proposes two complete reference populations whose native pooled ERROR-or-MISSING rate is 1/26 (3.85%). Product WASM validation and independent oracle coverage remain implementation gates.

## Frozen inputs and subject search

[Grammar and original subject pins](../../calibration/oracles/opencl-c/pins.json) describe the original SDK/hashcat screen. [The first subject search](../../calibration/oracles/opencl-c/subject-search.json) was committed at `c3ab954d9` before parsing its six candidates. [The additional search](../../calibration/oracles/opencl-c/subject-search-r3.json) was committed at `ee60410a0` before parsing its three candidates. Both retain search sources, zero-population misses and access gaps. Every result covers **every tracked regular lowercase `.cl`**, including include fragments, with no sampling or file exclusions. Public evidence contains counts/hashes rather than subject content, authors or inventories.

| Grammar | Pin | License and scope |
|---|---|---|
| lefp/tree-sitter-opencl | `8e1d24a57066b3cd1bb9685bbc1ca9de5c1b78fb` | MIT from pinned LICENSE; package metadata says ISC. Three-commit repository, 2023 HEAD, incomplete-coverage disclaimer. |
| tree-sitter/tree-sitter-c v0.24.2 | `b780e47fc780ddc8da13afa35a3f4ed5c157823d` | MIT; maintained unmodified C baseline, without an OpenCL layer. |

PyPI `tree-sitter-opencl` 0.0.2 is traceable to Time-Coder's `f1aabdf2` release. Its parser matches that repository and its grammar matches the lefp pin, so it is a repackaging rather than a third grammar. The untried alternative is a **P-R1 patch over pinned tree-sitter-c**, not an unnamed maintained fork. No grammar is selected for shipping and no ceiling is asserted.

## Complete native screens

The table reports files with ERROR **or** MISSING, not just ERROR. Exact ERROR-only/MISSING-only/both splits, bytes, full tracked/untracked before/after manifests and native-library hashes are in the [new receipts](data/opencl-c-triage-correction-2026-10-26/subject-outcomes.json) and original [SDK](data/opencl-c-triage-2026-10-26/native-sdk.json)/[hashcat](data/opencl-c-triage-2026-10-26/native-hashcat.json) receipts. Every screen has zero native faults.

| Whole population | Files | Dedicated ERROR-or-MISSING | C baseline ERROR-or-MISSING | Disposition |
|---|---:|---:|---:|---|
| Khronos SDK | 10 | 1/10 (10.00%) | 10/10 (100%) | Complete reference candidate |
| Hashcat | 1,349 | 1,344/1,349 (99.63%) | 1,344/1,349 (99.63%) | Diagnostic macro stress, outside proposed pool |
| darktable | 46 | 3/46 (6.52%) | 46/46 (100%) | Retained failed candidate |
| OpenCV | 110 | 75/110 (68.18%) | 110/110 (100%) | Retained failed candidate |
| ArrayFire | 103 | 8/103 (7.77%) | 98/103 (95.15%) | Retained failed candidate |
| clBLAS | 40 | 40/40 (100%) | 40/40 (100%) | Retained failed candidate |
| Rodinia | 31 | 3/31 (9.68%) | 31/31 (100%) | Retained failed candidate |
| pocl | 1,016 | 793/1,016 (78.05%) | 847/1,016 (83.37%) | Builtin/generated/negative mixture, diagnostic role frozen before parsing |
| SHOC | 38 | 3/38 (7.89%) | 38/38 (100%) | Retained failed candidate |
| HandsOnOpenCL Exercises-Solutions | 16 | 0/16 (0%) | 16/16 (100%) | Complete reference candidate |
| NVIDIA legacy examples | 41 | 5/41 (12.20%) | 41/41 (100%) | Retained failed candidate, license agreement unresolved |

The original SDK+hashcat proposal was infeasible: dedicated `(1+1344)/(10+1349) = 1345/1359 = 98.97%`, C `(10+1344)/1359 = 99.63%`. Separate reporting never fixes that pooled gate. Hashcat's computed includes and qualifier macros require a compiler-evidenced context and any prepass must remove nothing the oracle grades. Neither exists here, so hashcat is diagnostic-only and no prepass is claimed.

The revised **whole SDK + whole HandsOnOpenCL** proposal combines the owner's SDK examples with a complete teaching/exercise collection. Dedicated `(1+0)/(10+16) = 1/26 = 3.85%`, C `26/26 = 100%`. All ten SDK files remain, including its errored file. This is candidate selection after an explicit project search, with every failed project retained in the evidence, rather than selection of clean files inside a subject. The small educational population is not broad production coverage. HandsOnOpenCL compiler coverage is **unmeasured** and must meet the per-subject coverage gate before grading; the native screen alone grants no support.

These are native diagnostics, not WASM tenancy, throughput or accuracy. Both grammars use their committed parser/header without regeneration. Raw Git blob identities and captured immutable bytes protect the complete population. Symlink targets and gitlinks are recorded without traversal, with submodule contents uncovered. OpenCV's first clean checkout failed the strict raw-byte guard because 68 repository attributes requested CRLF conversion. A separate checkout with private info-attributes disabling text/filter/encoding conversion reproduced exact index blobs for all 7,372 regular files; zero case collisions, symlinks or gitlinks. The original checkout was untouched. The strict screen was not relaxed.

## Reuse CUDA's compiler oracle pipeline

The format owner's [OpenCL-Guide at 1269ba61](https://github.com/KhronosGroup/OpenCL-Guide/blob/1269ba618ecab9f4feb6f8a083bbb0f4c63eb56d/chapters/os_tooling.md) endorses Clang and its SPIR flow. The in-tree [CUDA database generator](../../calibration/oracles/scip-clang/cuda-compdb.py) already labels reconstructed compiler inputs tier B. OpenCL's [compdb.py](../../calibration/oracles/opencl-c/compdb.py) follows that pattern and feeds the **unmodified existing** tools:

- [clang-calls](../../calibration/oracles/clang-calls): written spelling/expansion mapping and call-site classes.
- [clang-ast-defs](../../calibration/oracles/clang-ast-defs): definition ranges and compiler-derived stand-ins, never stand-ins inferred from fathom output.
- [scip-clang/preproc](../../calibration/oracles/scip-clang/preproc): `active_lines` and include facts for the same compile context.

The committed [SDK replay](../../calibration/oracles/opencl-c/replay-sdk.py) uses the pinned Apple Clang 21.0.0/CLT 27.0 compiler with a minimal snapshot of its two pinned OpenCL resource headers. All ten SDK units complete at tier B: **23 function ranges, 173 classified written sites = 147 resolved + 26 declarations**, and 10 active-line roots (89 ranges, 546 lines). The [pipeline receipt](data/opencl-c-triage-correction-2026-10-26/pipeline-sdk.json) and [database capture receipt](data/opencl-c-triage-correction-2026-10-26/compdb-sdk.json) publish aggregate facts. Raw outputs remain private. Compiler/header pins are checked around each consumer, and input snapshot/database identity is checked throughout. This is pipeline capability evidence, not a fathom accuracy grade.

The original [compiler_probe.py](../../calibration/oracles/opencl-c/compiler_probe.py) is explicitly a **historical throwaway**, never to extend into a grading oracle. Its 23 bodies/19 kernels remain capability facts, but its 155 raw CallExpr nodes (6 source + 149 external) include eight repeated macro expansions. One written macro body call expands nine times. The existing `clang-calls` already resolves that distinction, yielding 147 written resolved sites. The historical [receipt](data/opencl-c-triage-2026-10-26/compiler.json) is retained without relabeling raw AST counts as written truth. Its AST hashes vary across processes.

The [host-context record](../../calibration/oracles/opencl-c/sdk-contexts.json) records SDK CL2.0/CL3.0 choices, reduce's work-group/sub-group defines, blur's subgroup exchange defines and reduce's appended generated helper source. The chosen baseline is CL3.0/SPIR64 with no defines, not a claim that host context is absent. Implementation must filter **both oracle truth and extractor sites** with the same `preproc.active_lines`, rather than scoring inactive regions as false positives. No all-device/configuration union is claimed.

[CUDA's driver](../../calibration/oracles/scip-clang/cuda-samples.sh) also provides the link-unit precedent: source grouping comes from independent build facts. OpenCL grouping must come from evidenced `clCreateProgramWithSource`/`clBuildProgram` assembly, including appended sources, never one inferred program per file. No link-unit truth or L3 claim exists in this proposal. The lowest reconstructed tier remains B and caps L2 at good.

The existing scip-clang runner deliberately pins 0.4.0 (`1860adf8`); current 0.4.1 (`14423f07`) is recorded rather than silently substituted. `-x cl` indexing is the cheap next probe, still untried. clangd's compile-database route, the Khronos guide, package provenance, SCIP/LSIF searches, language servers and runtime source/options capture are covered in [SEARCH.md](../../calibration/oracles/opencl-c/SEARCH.md), including misses and untried routes. No ceiling follows from those search misses.

## Implementation gates and review dispositions

ADR-048 data-only dialect/independent reading rules, I19 evidence privacy, I23 function-reference truth, I24 disclosed dropped targets and I25 readable-file accounting apply. No applicable revisit trigger is shown to have fired. No runtime service or decision model is added. The controller owns queue activation and plan proof.

After GLSL lands, reproduce the proposed pool in WASM, select the grammar, prove independent truth/explicit zero scope, and measure per-subject coverage, pooled errors, three-run median throughput, faults and per-file latency. `.cl` also belongs to Cool and is deliberately unclaimed by Common Lisp: require positive OpenCL/C evidence, Lisp/Cool negatives and ambiguous refusal. Keep include fragments in tenancy even without standalone-unit truth. Extra suffixes, C++ for OpenCL and embedded host strings stay outside this initial proposal.

The Apple compiler remains a historical host-specific triage pin. [llvm-plan.json](../../calibration/oracles/opencl-c/llvm-plan.json) pins official LLVM 21.1.8 archive SHA-256 values for macOS ARM64 and Linux X64/ARM64. These archives were not downloaded or run. Implementation must verify the archive, pin its actual compiler/resource bytes and replay all oracle facts, without assuming Apple/upstream version parity.

| Finding | Correction |
|---|---|
| F1 major | Existing CUDA/clang pipeline selected and replayed; old AST counter explicitly historical, never graded or extended. |
| F2 major | Two pre-parse pin commits, nine new whole populations, complete failures retained; hashcat stress role and explicit old/new pooled arithmetic. |
| F3 minor | Host options/source assembly recorded; shared active-line policy for truth and extractor. |
| F4 minor | Owner guide, traceable PyPI repackaging, clangd, current versus pinned scip-clang, named C-patch alternative. |
| F5 minor | README states lowercase `.cl` scope, rather than claiming an absent uppercase counter. |
| F6 minor | Portable discovery commands replace system paths; upstream archive plan remains an explicit implementation gate. |
| F7 minor | Assessment index entry is a table row with controller-owned activation. |

The [README](../../calibration/oracles/opencl-c/README.md) provides committed runnable screens, pipeline replay and focused tests. No product Go code is changed. Harness tests cover raw Git identity, optimized-Python pin refusal, immutable source copies, minimal resource isolation and context-override refusal. The new pipeline does not claim the independent-extractor mutation proof needed for a future support reading. Focused tests pass 12+4+10+7. The pre-fix replay fails all seven new guard tests and the restored replay passes. X-R2 release-notes and full tracked merge-debris checks pass; X-R3 TestNoOrgSpecificNames passes (2.300s). The X-R5 scan inspected all 37 changed/new files. Its seven lexical hits are repository pin identifiers and synthetic email/path fixtures, with no subject people or private paths in evidence. Generated aggregate files and plan checkboxes remain untouched.
