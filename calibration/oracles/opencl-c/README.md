# OpenCL C triage

This package measures native grammar diagnostics and replays the existing CUDA/clang oracle pipeline in reconstructed OpenCL C context. It registers no dialect, ships no grammar, adds no canonical corpus, and claims no language level. Implementation follows GLSL landing under G-R1. Pins and content digests are in [pins.json](pins.json).

Use an isolated Python environment with `tree-sitter==0.25.2`. All fetched sources, builds, raw ASTs, path manifests and logs stay under ignored `workspace/`. The caller must enforce the shared-machine load limit and nice 10. The examples use an existing `workspace/tools/python/bin/python`.

## Inputs and native grammar builds

Fetch the exact `archive_url` for each grammar in `pins.json`, verify its `archive_sha256` before extracting, and retain the supplied `src/parser.c` and header. Do not regenerate either candidate: this compares their committed upstream parsers. The dedicated grammar's top-level LICENSE is MIT; its package metadata says ISC, so the license conclusion uses the actual license file, whose digest is pinned. No Node package or binding is used.

Clone the two exact subject commits into `workspace/triage/opencl-r1/subjects/sdk` and `workspace/triage/opencl-r1/subjects/hashcat`. The SDK's `.reuse/dep5` applies Apache-2.0 to its files. Hashcat's license file is MIT. Full upstream source and attribution stay in the external checkouts, never in public receipts. The population rule was frozen before parsing: every tracked regular lowercase `.cl`, including include fragments, with no sampling or outcome exclusions. Only lowercase `.cl` files enter this triage population. Other suffixes and uppercase variants remain outside it; the complete tracked manifest still accounts for their bytes. Symlinks and unpopulated gitlinks have separate counters.

For grammar IDs `opencl` and `c`, unpack under `workspace/triage/opencl-r1/grammars/<id>` and build:

```sh
clang -O2 -dynamiclib -fPIC \
  -I workspace/triage/opencl-r1/grammars/opencl/src \
  workspace/triage/opencl-r1/grammars/opencl/src/parser.c \
  -o workspace/triage/opencl-r1/grammars/opencl.dylib
clang -O2 -dynamiclib -fPIC \
  -I workspace/triage/opencl-r1/grammars/c/src \
  workspace/triage/opencl-r1/grammars/c/src/parser.c \
  -o workspace/triage/opencl-r1/grammars/c.dylib
```

These local dylibs are triage artifacts. Native success is not a WASM tenancy, fault or throughput proof.

## Complete-population parse screen

Run one subject at a time, with a fresh output directory:

```sh
workspace/tools/python/bin/python -I -B calibration/oracles/opencl-c/triage.py \
  --subject workspace/triage/opencl-r1/subjects/sdk \
  --commit 6090378a113db8dab20ae278c8fa57ab10b92300 \
  --grammar opencl=tree_sitter_opencl=workspace/triage/opencl-r1/grammars/opencl.dylib \
  --grammar c=tree_sitter_c=workspace/triage/opencl-r1/grammars/c.dylib \
  --out workspace/triage/opencl-r1/native-sdk
```

Repeat with subject `workspace/triage/opencl-r1/subjects/hashcat`, commit `6a7e3b5a96abcd6373c2e4112bc358abb24dc538`, and output `workspace/triage/opencl-r1/native-hashcat`.

The screen verifies HEAD and a clean tracked tree, captures exact `git ls-files --stage -z` manifests before and after, hashes regular bytes and symlink targets without following symlinks, and records gitlink commits without traversal. It also records untracked and ignored path identities before/after. The parser consumes the immutable bytes verified during capture, rather than reopening subject files during measurement. Every node is checked for ERROR and MISSING. Faulting files remain in the denominator. Only aggregate counts, rates, pins and hashes go in `summary.json`. Paths and diagnostic messages remain under `private/`. A native process crash is a failed screen, not zero faults or a publishable summary.

## Existing clang pipeline, following CUDA

The design reuses [cuda-compdb.py](../scip-clang/cuda-compdb.py)'s synthesized-database pattern, [clang-calls](../clang-calls) for written spelling/expansion sites, [clang-ast-defs](../clang-ast-defs) for ranges and compiler-derived stand-ins, and [preproc](../scip-clang/preproc) for `active_lines`. Reconstructed inputs are **tier B**, including any future scip-clang consumer. The in-tree runner pins scip-clang 0.4.0; `-x cl` indexing remains an untried next probe.

[compdb.py](compdb.py) verifies the pinned compiler, headers and complete tracked source tree, snapshots regular files and synthesizes one CL3.0/SPIR64 entry per tracked lowercase `.cl`. It parses no AST. Symlinks and gitlinks are counted and omitted from this isolated compile snapshot, not silently resolved. A future subject needing those includes must evidence them before grading. Only the two pinned OpenCL resource headers enter the minimal resource snapshot. Unsupported non-UTF-8 paths are refused explicitly. The baseline accepts no extra command flags.

Run the committed SDK replay with a fresh output directory, under the shared-machine nice/load guard. The caller supplies `GOWORK=off`, `GOFLAGS=-p=2`, `GOMAXPROCS=2` and cache/temp locations under its ignored workspace or approved cache:

```sh
compiler=$(xcrun -f clang)
workspace/tools/python/bin/python -I -B calibration/oracles/opencl-c/replay-sdk.py \
  --sdk workspace/triage/opencl-r1/subjects/sdk \
  --clang "$compiler" --resource-dir "$("$compiler" -print-resource-dir)" \
  --out workspace/triage/opencl-r2/pipeline-sdk-replay
```

The runner builds and invokes the three unmodified in-tree tools with `-tier B` and an explicit pinned `-clang`. It verifies compiler/resource pins before and after each consumer and checks the source snapshot and database remain unchanged. Its aggregate receipt requires 10 units, zero errors/skips, 23 function ranges, 147 resolved written sites, 26 declarations and active-line coverage for all ten roots. Raw outputs and paths stay private. The database byte/tree and capture-receipt digests are path-bound diagnostic run identities: absolute checkout/output/compiler/resource paths make them differ across hosts or output locations. Compare the portable source/header and normalized oracle facts separately. Missing output parents are created automatically.

[sdk-contexts.json](sdk-contexts.json) records host-derived CL2.0/CL3.0 and define branches, plus reduce's appended generated helper source. The chosen no-define baseline does not cover every runtime variant. Implementation must filter **both truth and extractor sites** through the same `preproc.active_lines`. A future link-unit companion must follow [CUDA's build-derived grouping](../scip-clang/cuda-samples.sh), using evidenced OpenCL program source assembly, never one inferred program per file. No L3 grouping is claimed here.

The Apple binary pin is historical and host-specific. [llvm-plan.json](llvm-plan.json) pins official upstream LLVM 21.1.8 archives for macOS ARM64 and Linux X64/ARM64. They have not been downloaded or run: implementation must verify the chosen archive and replay the oracle evidence before READY.

## Historical throwaway capability probe

[compiler_probe.py](compiler_probe.py) is retained to reproduce the original capability receipt, **never to extend into a grading oracle**. Its 155 SDK raw AST CallExpr nodes include eight repeated macro expansions, whereas `clang-calls` reports 147 written resolved sites. Its 6 source plus 149 external counts are not written-site truth. Serialized AST IDs and paths vary across runs; raw AST hashes identify private runs rather than promise cross-process equality.

```sh
compiler=$(xcrun -f clang)
workspace/tools/python/bin/python -I -B calibration/oracles/opencl-c/compiler_probe.py \
  --clang "$compiler" --resource-dir "$("$compiler" -print-resource-dir)" \
  --sdk workspace/triage/opencl-r1/subjects/sdk \
  --out workspace/triage/opencl-r1/compiler-replay
```

## Subject search and tests

[subject-search.json](subject-search.json) and [subject-search-r3.json](subject-search-r3.json) freeze nine whole-project populations in separate commits before their screens. Hashcat is diagnostic stress only. Its role was assigned after the round-1 screen because computed includes and qualifier macros require a compiler-evidenced context, and round-1 review accepted that structural exception. Without the exception, the historical inclusive pool fails at 1,482/1,735 (85.42%). Pocl's mixed builtin/negative/generated population was designated diagnostic-only before parsing. [pool-rule.json](pool-rule.json) now fixes all eligible OSI-licensed whole populations by metadata, prior role and the named hashcat exception. HandsOn CC-BY-3.0 and the unresolved NVIDIA EULA are diagnostic-only. The former SDK+HandsOn pairing is withdrawn as outcome-selected; it establishes no gate pass. The already measured primary set fails at 133/378 (35.19%), so P-R1 grammar repair must retain that full set plus PyOpenCL, which was pinned before its screen. All outcomes remain disclosed; no ceiling is asserted. Every future patch measurement must report hashcat's dedicated and C rates beside the fixed pool. The assessment reports the actual pooled gate, not an average of subject percentages.

For raw-blob fidelity, checkout conversions must be suppressed before running `triage.py`. OpenCV's attributes request CRLF for 68 files. In a separate fresh clone created with `git clone --no-checkout`, write `* -text -filter -ident -working-tree-encoding` to `.git/info/attributes` before checking out the pinned commit. This changes only private Git metadata. The unchanged screen still requires each raw filesystem blob to match its index OID. The original checkout stays untouched.

Run the focused tests under the same resource guard:

```sh
workspace/tools/python/bin/python -I -B calibration/oracles/opencl-c/test_triage.py
workspace/tools/python/bin/python -I -B calibration/oracles/opencl-c/test_compiler_probe.py
workspace/tools/python/bin/python -I -B calibration/oracles/opencl-c/test_compdb.py
workspace/tools/python/bin/python -I -B calibration/oracles/opencl-c/test_replay_sdk.py
```

The frozen eight-project native pool, whose rule was registered after seven outcomes and before PyOpenCL, fails at **138/386 (35.75%)** for the dedicated grammar and **380/386 (98.45%)** for C, with zero faults. Its 386 files and 382 distinct blobs are never deduplicated for the gate. [The counts receipt](../../../docs/assessments/data/opencl-c-pool-2026-10-26/pool-counts.json) separates primary and diagnostic counts. Use the committed `pool_audit.py --subject id=checkout` once for every primary ID in `pool-rule.json`, optionally adding `--diagnostic id=checkout=full-pin`, with `--out workspace/triage/opencl-r3/pool-counts.json`. The CLI requires every and only the frozen primary set, verifies source identities and emits counts plus the policy digest. All checkout paths and raw inventories remain private.
