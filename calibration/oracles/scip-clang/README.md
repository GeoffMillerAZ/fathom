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
    run.sh <tools>/bin/scip-clang <subject> <subject>/build <out.scip> <tier> [jobs]   # tier A: the build's own database

The scratch copy is deleted when the readings are taken. `run.sh` takes the index's tier (A for the build's own
database, B for a translation) and `indexbench lang-oracle` refuses a scip-clang index without `-scip-tier` (round 4
C4-R2: no tier is read as A by default, nor any oracle file's that names none). `run.sh` refuses any release but 0.4.0, warns when the database
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

    cd preproc && go run . -subject <subject> -compdb <scratch-build-dir>/compile_commands.json -tier A > preproc.json

- `imports`: each `#include` written in a subject file and the subject file it entered, from the preprocessor's line
  markers (a guarded re-include writes no marker and is no import, as for ../clang-includes, whose tier-B dump this
  replaces here: it runs the build's own flags, where clang-includes runs the operator's `-I` roots, and it is not
  stopped by a missing generated header such as curl's `curl_config.h`);
- `active_lines`: the source lines each file keeps. scip-clang indexes one configuration: a definition in a branch
  that configuration discards (libuv's `src/win/` on a Unix build, a `#ifdef __linux__` typedef on macOS, curl's
  OpenSSL code with SSL off) has no occurrence in the index. `lang-oracle -scip-defs` scores fathom's definitions only
  in files the index holds and on lines the preprocessor keeps, and the callers of the L2 calls the same way; the
  counts are disclosed in the reading's notes.

    indexbench lang-oracle -dialect c -subject <libuv> -subject-name libuv/libuv -scip libuv.scip -scip-tier A -scip-defs \
        -tool-oracle preproc.json -unmatched unmatched.json -out docs/bench/readings/lang/c.json

`../clang-ast-defs` writes `preproc.ranges.json` for the same subject (`go run . -subject <subject> -tier A -compdb
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

## CUDA (LB-Q-cuda): the samples' own CMake database, nvcc's preprocessor (tier A), a clang translation for the index

`cuda-samples.sh` is the driver (F12) that builds every database, index and run of the reading:

    cuda-samples.sh <shared-clone> <cuda-root> <tools-dir> <work-dir> [runs]
    # cuda-root: CUDA 12.9 from conda-forge (cuda-nvcc-tools, cuda-nvvm-tools and cuda-crt 12.9.86, which carry the
    # nvcc driver and cicc the pip wheels lack, plus the -dev header packages: cudart, cccl, cublas, cufft, curand,
    # cusolver, cusparse, npp, nvjpeg, nvrtc, cupti, nvtx, cuda-driver-dev for cuda.h, and cuda-profiler-api for
    # cuda_profiler_api.h, which asyncAPI.cu and matrixMul.cu include: round 3's toolkit lacked it and dropped both
    # units from the clang translation, 97 of 114 where round 2 and round 4 compile 99); measurement only, under
    # workspace/tools, never committed. tools-dir/ctags is universal-ctags 6.2.1 built with jansson (its JSON output).

It measures cuda-samples as ONE subject, the way a user indexes it (owner ruling C-R4): CMake configures the samples
with nvcc (sources and static libraries the sparse clone or the toolkit lacks get an empty stand-in; nothing is
built), and that database is the build's own. The preprocessor oracle is nvcc's, both passes (owner ruling C-R6): the
device pass is `nvcc -E` at the lowest architecture the entry's target names (so `__CUDA_ARCH__` branches are judged,
not excluded) and the host pass is the `gcc -E` line `nvcc --dryrun` prints for the entry; their active lines and
includes are unioned (tier A). scip-clang 0.4.0 reads no nvcc entry, so each `.cu` entry is translated to a clang CUDA
host-only compile over the entry's own `-I`, `-isystem` and `-D` (`cuda-compdb.py`'s flags) at
`<work-dir>/compile_commands.json`; the index reads it, and `clang-calls` and `clang-ast-defs` over it are run at
`-tier B`, so the calls family is judged at tier B (the harness takes a family's tier from the weakest oracle that
feeds it). `cuda-compdb.py` writes the same translation from a directory walk, for a subject with no CMake build.

Round 3 (A2, B, C): the driver first configures the samples with clang++ as their CUDA compiler and compiles each unit
as written (clang-native.json): an index is tier A only when the subject's own build wrote its database untranslated,
at 90% of the units; cuda-samples v12.9 configures 114 and compiles 0 (its CMake flags carry nvcc's
`--extended-lambda` and sm_100, sm_101 and sm_120, which clang 18 rejects), so the index reads the translation and
lang-oracle runs at `-scip-tier B` (round 4 C4-R5: the 90% gate counts the nvcc database's 114 .cu units, never the
clang configure's own count, and `-DCMAKE_CUDA_ARCHITECTURES=52` has no effect: the samples' CMakeLists set it
themselves). Each unit the translation drops is listed in clang-skipped.json, its first clang error in
clang-skipped-errors.json (round 4 C4-R4). clang-calls unions the host pass with a `--cuda-device-only` pass at the first of
the target's CMake architectures that compiles (`-Xclang -target-sdk-version=12.3`: with -nocudalib clang otherwise
reads no toolkit version and wants the pre-9.2 launch API). `preproc.units.json` maps each CMake executable to its
sources from the build's own link lines (the link-unit companion of preproc.json). The driver writes its measurement
record and the median-of-runs throughput into `<work-dir>/reading.json` (no hand edit), and passes the clone's commit
as `-subject-sha` (the scratch copy has no .git). Round 5 (C5-R2): both CMake build trees are configured under
`<work-dir>`, never in the subject copy (a build tree there adds CMake's two CompilerIdCUDA sources to the measured
set), and the measured set, the subject's `.cu` and `.cuh` files, is listed at `<work-dir>/measured-files.txt`; every
run's file count and bytes must agree with it, and its count, bytes and the sha256 of the sorted list are a note of
the reading, so a re-timed clone can be proven to measure the same files.

C-4: the selected clang database and all build output stay under `<work-dir>`; scip-clang, clang-calls and
clang-ast-defs read `<work-dir>/compile_commands.json`. The subject contains no build directory. Fathom's include-path
resolver currently discovers only databases within the subject, so a cloud re-measure must check import resolution,
file-edge counts and the compile-database disclosure after this move. The committed measurements remain the earlier
run's evidence until that re-measure. The measured `.cu`/`.cuh` inventory should stay identical.

Driver note corrections without an oracle run use this documented notes-only path:

    python3 calibration/oracles/scip-clang/cuda-reading-notes.py --refresh-notes docs/bench/readings/lang/cuda.json

It preserves every field outside `notes`, checks the disclosed throughput runs against the committed median, and
uses the same note generator as the driver. Equal recorded CPU rates retain run order before selecting zero-based
position `runs // 2`. For the round-5 record that selects run 3 (0.96 per core, wall 0.97). The pinned subject's
stand-in note explains why the predicted 38th extern "C" function, in `eigenvalues/util.h`, is outside the dialect.
Offline regression tests invoke no oracle:

    PYTHONDONTWRITEBYTECODE=1 python3 calibration/oracles/scip-clang/cuda-samples_test.py

Two samples that define the same global name (`iDivUp`, `main`, a kernel) are one SCIP symbol, since scip-clang
names a global by no file or translation unit, so the index's file-edge truth over the whole set draws an edge to
every definer: the truth is scoped to link units (per CMake executable, round 3 B, scip_file_edges_link_unit).

## C++ producer audit (core-t1, canonical subjects only)

`cpp-audit.py` is a measurement driver for the pinned fmt, json and spdlog
population. It leaves the product extractor and shared grading harness unchanged.
It does not select or read a holdout. Keep all native JSON, compiler diagnostics,
SCIP indexes, unmatched pairs and file inventories under ignored `workspace/`.
Only the generated readings and sanitized count/hash receipts are candidates for
publication, after the organization-name and evidence-privacy checks.

The audit compares scip-clang 0.4.0 and 0.4.1 against the same full, untranslated
CMake databases. Their arm64-darwin binary SHA-256 values are respectively
`ff042fbc8a029f09f4b69fc7692e290e21c52923593207ee52d4e7439473ec64` and
`34a2eb7a5ef9bbe9849cded59f4eb22a2331bd5b64a5cec2b17ee09b8aa515de`.
Both tagged licenses are Apache-2.0, SHA-256
`96b8b909d18f94921214e3995e4c12483826b12ca64dd93fc710b87e1bc5a08a`.
The existing `run.sh` remains guarded at 0.4.0. For each verified binary,
the raw index command is:

```bash
(cd "$CPP_SUBJECT" && "$SCIP_CLANG_BINARY" \
  --compdb-path "$CPP_COMPILATION_DATABASE" --index-output-path "$CPP_INDEX" \
  --jobs 2 --no-progress-report --log-level warning)
```

Run that command serially under `nice -n 10`, after the one-minute load is at
most 9, while holding the shared measurement lock. Configure each subject with
its own options above. The audit's full databases contain 51, 93 and 141 entries;
their unique subject/outside units are 31/0, 78/0 and 34/107. Retain all entries,
including the 107 fetched dependency units in spdlog's database. Expose each
external database to Fathom through the subject-root `compile_commands.json`
symlink and keep all build trees outside the subjects. In each disposable subject,
add only `/compile_commands.json` to its local `.git/info/exclude` if the subject
does not already ignore that non-language link. This lets `lang-retime` enforce
a clean checkout without removing the resolver input; the manifest recipe below
still inventories the ignored link. Do not ignore any language source. The exact tracked-set
recipe in design §13.3.1 runs before and after configuration and measurement;
its private file lists must not be published.

The native phase builds the three committed Go oracle modules and runs the
preprocessor, call-site and AST-range adapters serially. The grade phase consumes
frozen indexes and complete native output. It measures both the historical
`-only` scope and the full C++ population, retaining these as distinct readings
for each SCIP version. Ctags remains L1's oracle; SCIP grades the call/typed/file
families, with native call sites and preprocessing as companions. No compiler
error, stopped unit or missing companion may become a successful partial result.

Set the explicit directory and executable arguments shown by
`python3 calibration/oracles/scip-clang/cpp-audit.py --help`. Set `TMPDIR`,
`GOTMPDIR`, `GOCACHE` and `GOMODCACHE` to the assigned isolated cache, `GOWORK=off`,
`GOFLAGS=-p=2`, and `GOMAXPROCS=2`. Use a fresh private work directory for a failed
phase. The native and grade phases share the same work directory and input
receipts. The published receipt records counts and hashes, not local tool paths
or compiler output. Synthetic controls run without subjects or oracles:

```bash
PYTHONDONTWRITEBYTECODE=1 python3 calibration/oracles/scip-clang/cpp-audit_test.py
```
