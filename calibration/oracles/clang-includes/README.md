# clang-includes — tier-B C/C++ import oracle (LB-F40)

A small Go program (its own module, never imported by fathom) that writes `indexbench lang-oracle
-tool-oracle`'s neutral `imports` for a C or C++ subject, scored as L3 `import_resolution` (design §1.2,
§10). The tool is the compiler's own preprocessor: `clang -E -w -x c|c++ [-I <root>]... <file>` from the
subject root, one process per file. Its line markers say which file each `#include` entered (flag 1)
and the line the includer resumes at (flag 2), so an include written in the measured file itself is the
line before the marker that returns to it, and its target is the file the compiler opened. A
conditional include the preprocessor skipped is no include; a computed one (`#include FOO_H`) is,
resolved as the compiler resolves it. Only targets inside the subject are written; `stderr` counts
`includes_in_subject`, `includes_outside` (system headers) and `files_stopped` (a missing header is a
fatal error: that file's later includes are not seen).

Tier B, not A: the include roots are this run's (`-I`), not the build's own flags. A run driven by the
subject's `compile_commands.json` (the build's evaluation, tier A) is not implemented.

    cd calibration/oracles/clang-includes && go run . -subject <checkout> [-I include]... [-files <list>] [-lang c|c++] > workspace/tmp/oracle/cpp-<subject>-imports.json

`-files` takes the measured file list (one subject-relative path per line) so the oracle reads exactly
what the harness reads; by default it walks every C/C++ source and header, skipping hidden, `vendor`,
`third_party` and `node_modules` directories. `.c` files are preprocessed as C, every other file as C++
unless `-lang` says otherwise. Apple clang 21.0.0 runs it; `cmd/indexbench/testdata/clang-includes-cppfixture.json`
is its output on `internal/fusion/testdata/cppfixture` (`files 2 includes_in_subject 1
includes_outside 1`: `#include <vector>` opens the SDK's header). `go test .` checks the marker parser.
Bench-only; never a product dependency.

Measured 2026-09-29 (`docs/bench/readings/lang/cpp.json`, the L3 re-grade): fmtlib/fmt 12.2.0 (1be298e1) with
`-I include -I test/gtest -files <include/, src/, test/*.cc, test/*.h, test/fuzzing/**>` (`files 62 files_stopped 2
includes_in_subject 144 includes_outside 103`) and nlohmann/json v3.12.0 (55f93686) with `-I include` (`files 46
includes_in_subject 94 includes_outside 83`). `import_resolution` 218 of 218 in-repo imports; 79 of 777 imports resolve where
the oracle names no file, because an include the compiler does not enter (a guarded re-include, an `#ifdef`-skipped one) writes
no line marker (`.fathom/observations.md` O146).
