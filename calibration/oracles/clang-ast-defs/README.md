# clang-ast-defs — tier-A definition ranges (C, C++)

scip-clang writes no end line for a definition, so L1 against its index cannot score `range_agreement` and
caps at good (language-breadth §1.2). The compiler's own AST has the range. This tool runs

    clang -fsyntax-only -w -Xclang -ast-dump=json

once per translation unit of a compilation database (the entry's own flags, its compile and dependency outputs
removed), and writes the subject's definitions with first and last line: a function with a body, a struct or union
with a body, an enum with constants, a typedef. It is its own Go module (`fathom.bench/oracles/clang-ast-defs`),
never imported by fathom, the way `../scip-clang/preproc` is.

    go run . -subject <subject> -compdb <subject>/build/compile_commands.json [-clang clang] [-jobs 4] > preproc.ranges.json

Output: `{tool: "clang-ast", tier: "A", version, units, errored, skipped, ranges: [{path, name, kind, line, end}]}`,
kinds in fathom's canonical names (`function`, `struct`, `union`, `enum`, `type_alias`), sorted. A file the database
builds twice (libuv's static and shared library) is dumped once (`skipped`); an entry whose file is not on disk or
whose dump clang could not produce is counted in `errored`, never silently dropped.

Facts of clang's JSON this reads: a location's `file` and `line` appear only where they differ from the previous
location in print order, so the walk keeps that state across the whole tree, subtrees it does not want included; a
macro-expanded head (`TEST_IMPL(loop_stop) {`) is placed by its expansion location; a macro itself has no AST node,
so no macro is scored (the reading says so).

## How the harness reads it

`indexbench lang-oracle` takes the file as the companion of a tool oracle: a sibling named `<tool-oracle>.ranges.json`
(`libuv-preproc.json` -> `libuv-preproc.ranges.json`). `cmd/indexbench/langoracle_imports.go` (`scoreRangeCompanion`)
pairs fathom's definitions with the companion's by `langbench.ScoreL1`'s own rule (same file and name, |Δline| <= 2,
nearest first) and takes only the range counts: `range_agreement` is the share whose last line is within one line of
the compiler's. The L1 oracle stays the index, so sym_recall, sym_precision and kind_agreement are unchanged and the
TEST_IMPL (`scip:macro-generated`) exclusion is untouched. No companion: `range_agreement` stays not measured.
