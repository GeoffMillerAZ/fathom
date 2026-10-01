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
`compile_commands.json` its build tool writes. Every pinned subject builds with CMake and none ships one:

    cmake -S <subject> -B <scratch-build-dir> -DCMAKE_EXPORT_COMPILE_COMMANDS=ON [options]   # nothing is built
    run.sh <tools>/bin/scip-clang <subject> <scratch-build-dir> <out.scip> [jobs]

The build dir stays outside the subject tree (the checkout is never written to) and is deleted when the readings
are taken. `run.sh` refuses any release but 0.4.0 and runs scip-clang from the subject root, so document paths
are subject-relative. libuv indexes in 4 s, curl in 5 s, fmt in 1 s, json in 2 s.

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

C reads its L1 against the index's definitions (`-scip-defs`, owner decision D2); C++ keeps universal-ctags for L1
(tier B) and takes calls, file edges and typed edges from the index.
