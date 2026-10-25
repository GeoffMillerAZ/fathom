# scip-clang — tier-A C and C++ index (def_mapping, typed_recall, calls, file edges, L1 definitions)

Pinned release, fetched without sudo into the oracle tools dir:

    version   scip-clang 0.4.0 (Based on Clang/LLVM 2078da43e25a4623cab2d0d60decddf709aaea28)
    release   sourcegraph/scip-clang v0.4.0, 2026-02-23 (tag commit 1860adf8deab7870fed2199eadc190e3965a92ff)
    asset     https://github.com/sourcegraph/scip-clang/releases/download/v0.4.0/scip-clang-arm64-darwin
    sha256    ff042fbc8a029f09f4b69fc7692e290e21c52923593207ee52d4e7439473ec64   (matches the release's own digest)
    licence   Apache-2.0 (sourcegraph/scip-clang LICENSE at v0.4.0: the plain Apache License 2.0 text, no LLVM exception
              clause of its own; the LLVM it embeds is Apache-2.0 WITH LLVM-exception). Both are on ADR-048's allowlist
              (Apache-2.0); the tool is a separate process on the measurement path, never linked, vendored or shipped.

    gh release download v0.4.0 -R sourcegraph/scip-clang -p scip-clang-arm64-darwin -O <tools>/bin/scip-clang && chmod +x <tools>/bin/scip-clang

## Run

scip-clang indexes the translation units of a compilation database, so a subject needs the
`compile_commands.json` its build tool writes. Every pinned subject builds with CMake and none ships one. Work in a
scratch copy of the pinned checkout (`git clone <shared clone> <scratch>/subjects/<name>`, never the shared checkout):
the database goes to `<subject>/build/compile_commands.json`, the conventional in-repo place fathom's include-path
resolver reads (internal/fusion/compdb.go: the repo root, `build/`, `builddir/`, `_build/`, `cmake-build-*/`,
`out/build/*/`; per translation unit its `-I`, `-iquote`, `-isystem` and `-idirafter`, joined or separate, relative
to the entry's directory). One database then serves the index, the preprocessor oracle, the AST oracle and the
harness's own file edges and imports, so they all see the flags the build used:

    cmake -S <subject> -B <subject>/build -DCMAKE_EXPORT_COMPILE_COMMANDS=ON [options]   # nothing is built
    run.sh <tools>/bin/scip-clang <subject> <subject>/build <out.scip> [jobs]

The scratch copy is deleted when the readings are taken. `run.sh` refuses any release but 0.4.0, warns when the database
is not at `<subject>/build`, and runs scip-clang from the subject root, so document paths are subject-relative. libuv
indexes in 4 s, curl in 5 s, fmt in 1 s, json in 2 s. The other two oracle files of a reading are written the same way
(`preproc/` below, and `../clang-ast-defs`, which runs `clang -Xclang -ast-dump=json` per entry for the definition end
lines the index lacks).

Subjects and the cmake options each needed (2026-10-01, Apple clang 21.0.0, macOS arm64):

    libuv v1.53.0     -DBUILD_TESTING=ON                                          477 entries, 246 files
    curl 8.22.0       -DCURL_ENABLE_SSL=OFF -DCURL_USE_LIBPSL=OFF -DUSE_LIBIDN2=OFF -DCURL_BROTLI=OFF -DCURL_ZSTD=OFF
                      -DCURL_USE_LIBSSH2=OFF -DBUILD_TESTING=ON                   495 entries; the 6 generated test
                      bundles (tests/*/units.c and the like) are not on disk and are skipped
    fmt 12.2.0        -DFMT_TEST=ON                                               51 entries (31 files)
    nlohmann/json     -DJSON_BuildTests=ON                                        93 entries, all tests/ (scip-clang is
    v3.12.0                                                                       run from the repo root, so include/ is indexed)

curl configures offline once SSL is off (Apple's SecTrust needs OpenSSL or GnuTLS); the readings are of that
configuration, so curl's lib/vtls/openssl.c, lib/vssh/libssh2.c and every other feature-gated file are preprocessed
to nothing and fall under "inactive lines" below.

## What the index carries, and what the harness does with it

The harness (`indexbench lang-oracle -scip`, internal/langbench/scip*.go) reads these facts of scip-clang 0.4.0:

- no `enclosing_range` on any definition (so `internal/scip.ImportIndex` takes a mapped definition's fathom lines as
  its span, LB-F54: typed edges are now stored from a scip-clang index, libuv 30,869) and no `SymbolInformation.kind`;
- a function is `name(<hash>).`, a type `name#`, a variable or field `name.`, a file `` `<file>/path`/ ``, and a macro
  a symbol named by its location, `` `src/inet.c:25:9`! `` (the harness names it by what is written at its range);
- a file compiled in several translation units (libuv builds a static and a shared library) is indexed once per unit:
  its definitions repeat and are collapsed;
- a macro-generated definition is reported over the macro's invocation: libuv's `TEST_IMPL(loop_stop) {` defines the
  function `run_test_loop_stop` over the range of `TEST_IMPL`. Such a definition is `scip:macro-generated`, left out
  of L1 and counted (the same disclosure rust.json makes for `scip:macro-generated`);
- a nameless record or enum is `$anonymous_type_N`, kind `anonymous`, left out of L1 and counted;
- a function passed as a value, and every call a macro's expansion makes (`abort`, `fprintf` inside an `ASSERT`), is a
  reference at the macro's own range with no call written there: L2 is call sites, so these are left out of the call
  truth and counted (`scip_calls_unwritten`); a call written inside a macro's argument, `ASSERT_OK(uv_ip4_addr(..))`,
  is referenced at `ASSERT_OK`'s range and recovered when the callee follows on that line; C records a function-like
  macro invocation as a call (`ASSERT_OK(`), so for C the macro is itself a call; C++ excludes all-capitals macros;
- no reference on an `#include` line and no occurrence with the Import role: scip-clang has no import truth, so
  `import_resolution` is `preproc/`'s, below.

## preproc — the build's own preprocessor, tier A

`preproc/` (its own Go module, never imported by fathom) runs each compilation database entry with `-E -dD -w` and
the entry's own flags, and writes a neutral oracle file with two things the index does not say:

    cd preproc && go run . -subject <subject> -compdb <scratch-build-dir>/compile_commands.json > preproc.json

- `imports`: each `#include` written in a subject file and the subject file it entered, from the preprocessor's line
  markers (a guarded re-include writes no marker and is no import, as for ../clang-includes, whose tier-B dump this
  replaces here: it runs the build's own flags, where clang-includes runs the operator's `-I` roots, and it is not
  stopped by a missing generated header such as curl's `curl_config.h`);
- `active_lines`: the source lines each file keeps. scip-clang indexes one configuration: a definition in a branch
  that configuration discards (libuv's `src/win/` on a Unix build, a `#ifdef __linux__` typedef on macOS, curl's
  OpenSSL code with SSL off) has no occurrence in the index. `lang-oracle -scip-defs` scores fathom's definitions only
  in files the index holds and on lines the preprocessor keeps, and the callers of the L2 calls the same way; the
  counts are disclosed in the reading's notes.

    indexbench lang-oracle -dialect c -subject <libuv> -subject-name libuv/libuv -scip libuv.scip -scip-defs \
        -tool-oracle preproc.json -unmatched unmatched.json -out docs/bench/readings/lang/c.json

`../clang-ast-defs` writes `preproc.ranges.json` for the same subject (`go run . -subject <subject> -compdb
<subject>/build/compile_commands.json > preproc.ranges.json`). The harness reads it as the companion of the tool oracle
named `preproc.json` (a sibling `<name>.ranges.json`) and scores range_agreement from it alone: the index stays L1's
oracle, and a missing companion leaves range_agreement not measured, as before.

C reads its L1 against the index's definitions (`-scip-defs`, owner decision D2); C++ keeps universal-ctags for L1
(tier B) and takes calls, file edges and typed edges from the index.

What L3 and L4 read from the index (wave lf1, `internal/langbench/scip_clang.go`; each count is in the reading):

- def_mapping names a macro by what is written at its range (its symbol is its location) and leaves out a document's
  own `<file>/` symbol, a nameless `$anonymous_*` type or namespace and a definition in a file the reading does not
  measure (`scip_defs_files`, `scip_defs_anonymous`, `scip_defs_out_of_scope`; LB-F292). typed_recall counts the
  references in the measured files to definitions in them (`scip_edges_out_of_scope`).
- A definition a macro writes (another identifier at its range: libuv's `TEST_IMPL(timer) {` is `run_test_timer`,
  curl's `CURLWARNING(Wcurl_...)` functions, `BIT(x);` bit-fields) is `scip_defs_macro_generated`, and the references
  in its brace body and to it are `scip_edges_macro_generated`, both out (LB-F293, LB-F249's shape: scip-clang gives
  such a definition no enclosing_range, so the body is read lexically). One the dialect names anyway (tree-sitter reads
  `CURLOPT(CURLOPT_URL, ...)` as the enumerator) counts as any other, and a body a fathom definition starts on the line
  of (gtest's `TEST(suite, name) {`) is not cut.
- A file pair fathom draws that the truth lacks, whose bound name is written in the drawing definition's lines where the
  index writes no occurrence (a type in a template argument or return type, `std::shared_ptr<logger>`), is
  `file_edges_oracle_blind`, out of file_edge_precision under LB-F203's 10% cap and listed under -unmatched's
  `oracle_blind` (LB-F296); a name on a line where a macro is invoked is judged at the macro.

## CUDA (LB-Q-cuda): a clang CUDA compilation database, tier B

The CUDA build's compiler is nvcc, which the redistributable components a Linux x86_64 host can fetch without a CUDA
Toolkit installer do not include (the headers, cudart, ptxas and the libraries ship; the nvcc driver and cicc do not).
`cuda-compdb.py` therefore rewrites each `.cu` translation unit to a clang CUDA host-only compile and writes the
database `run.sh`, `preproc/`, `../clang-ast-defs` and `../clang-calls` read:

    cuda-compdb.py <subject> <cuda-root> <subject>/build/compile_commands.json -I Common
    # cuda-root: the CUDA 12.9 headers (cuda_runtime, cccl, cublas, cufft, curand, cusolver, cusparse, npp, nvjpeg, nvrtc, cupti)
    # unpacked from NVIDIA's own redistributable wheels into one include/ tree; measurement only, never committed.

scip-clang indexes it with the clang it embeds (a TU clang cannot compile, a missing OpenMP, GL, Vulkan, D3D or Tegra
header, is dropped from the database and counted as a skip). That is not the build's compiler, so `preproc/`'s
imports, `clang-calls` and `clang-ast-defs` over this database are tier B, and their JSON is relabelled `"tier": "B"`
by hand before the harness reads it (the tools hard-code A). `clang-ast-defs` writes a 340 MB JSON dump per CUDA
translation unit: run it with `-jobs 2`.

Each cuda-samples sample is its own executable, so one index over all of them gives two samples that define the same
global name (`iDivUp`, `main`, a kernel) one SCIP symbol, and the harness then draws a file edge to every definer: 402
of the 415 truth pairs fathom missed in a single-index run were between two samples. The reading is therefore taken
per sample (a subject is one sample directory plus `Common/`, with its own database and index).
