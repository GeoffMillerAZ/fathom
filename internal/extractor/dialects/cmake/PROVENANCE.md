# cmake — provenance

LB-Q-cmake, design `docs/design/language-breadth.md` §13. A tags-engine tool format, a port of the pure-Go
`internal/extractor/cmakeextract.go` (CT-4/CT-5), which stays the dispatched extractor of `CMakeLists.txt` and `*.cmake`
until the integrator switches it (see "Dispatch" below).

- Grammar: `uyha/tree-sitter-cmake` tag `v0.7.5`, commit `e997bd0b275ca525ce9befecedf5299031183661` (MIT; the top-level
  `LICENSE`, Uy Ha 2025; `indexbench lang-pin -tree` reads it as MIT), ABI 14, external scanner (`src/scanner.c`),
  `src/parser.c` shipped (no generate step), grammar at the repository root (`tree_sitter_cmake`), built into
  `internal/extractor/tsengine/grammars/cmake.wasm.gz` (198,511 bytes raw, 68,052 gzip). The catalogue
  (`docs/plans/language-breadth.grammars.json`) pre-filled v0.7.4 (`ca627bb…`); v0.7.5 is the release at pin time.
- `tags.scm`: written in this repository (design §13 step 11); the grammar's `queries/` holds highlights, folds and
  injections only, neither aider nor nvim-treesitter carries CMake tags. Node shapes came from `indexbench lang-probe`
  over `testdata/*.cmake`, never from `grammar.js`. A CMake file is a flat sequence of commands, so every definition
  is one command invocation named by its first argument, the command name matched case-insensitively (`ADD_EXECUTABLE`).
- Kinds, the incumbent's `SourceKind` strings byte for byte: `CMakeProject` (`project`), `CMakeTarget`
  (`add_executable`, `add_library` of every type including ALIAS, INTERFACE, IMPORTED and OBJECT, `add_custom_target`),
  `CMakeFunction` (`function` and `macro`, a definition that runs to `endfunction()`/`endmacro()`), `CMakeOption`
  (`option`), `CMakeDependency` (`find_package`); one more, `CMakeVariable`: `set(NAME ... CACHE ...)`, the cache entry
  a user configures (a plain `set` is a local the file writes, not a surface). A name spelled with a variable
  (`${PROJECT_NAME}_var`) is not a definition (it names nothing without CMake's evaluation; universal-ctags skips it
  too), and `::` in a name is written `.` (`fmt::fmt` is `fmt.fmt`, CT-1, `qualify.scope_sep`).
- Differences from the incumbent, all measured by `indexbench lang-parity`: `Signature` is the command's first line
  (`add_library(core STATIC`), the incumbent's was its flavour (`library STATIC`); a command in a function body is a
  definition of the file (the incumbent scanned it the same way); `CMakeVariable` is new; an
  `add_library(core ALIAS x)` target is named `core`, the incumbent's name for it too.
- References (`@reference.call`): every unquoted, variable-free argument after the first of `target_link_libraries` and
  `add_dependencies`, link keywords (`PRIVATE`, `PUBLIC`, `INTERFACE`, `debug`, `optimized`, ...) excluded. Those
  commands are siblings of the target's own, so the engine's `@calls.to`/`@calls.from` attachment gives a reference to
  the target of the same name that precedes it as a sibling in the same file or block, with at most four commands
  between them. One pattern per gap: a pattern that pairs every target with every later link command is quadratic,
  and the engine's scan budget (10 s) failed a flat file at 300 targets; the gap bound is the measured distribution
  (481 literally named targets across the subjects and their neighbours: 61 followed by a link command of their own in
  the same block, 57 within two commands, none beyond four). The incumbent attached a link to its target anywhere in
  the file; the dialect does not, and the reading counts what it misses.
- `imports.scm`: `add_subdirectory(dir)` and `include(file)` are whole-file imports (and wildcards, so the target's
  top-level names come into scope) through `imports: {strategy: relative-path, relative: true, index_files:
  ["CMakeLists.txt"], suffixes: [".cmake"]}`: `add_subdirectory(src)` is `src/CMakeLists.txt`, `include(cmake/x)` is
  `cmake/x.cmake`. A spelling with a variable names no file and is not an import.
- `testdata/`: fixtures written in this repository (`targets`, `functions`, `links`, `imports`, `broken`), each golden
  written by hand from reading the fixture before the queries were final. The one correction the goldens forced was the
  engine's, not the query's: a multi-line `add_library(\n name ...)` is `Line` the name's line (as universal-ctags
  reports it), `EndLine` the command's last.
- **Dispatch**: `internal/fusion/register.go` hand-wires `CMakeLists.txt` and `.cmake` to `cmakeextract.go`, so the tags
  loop does not claim them. `indexbench lang-parity -dialect cmake -incumbent cmakeextract` on the three subjects
  (2026-10-01): every L1 metric is at least the incumbent's (sym_recall .9952 = .9952, sym_precision .9952 vs .9200),
  ref_precision 1 vs .8986, kinds gained `CMakeVariable`; the verdict is REFUSED only because the dialect's `level_met`
  is `none` (target L3 not met). Retiring `cmakeextract.go` on the integrator's D1 re-run means deleting its
  `register.go` block, its test and re-anchoring `docs/plans/cpp-toolchain.md` CT-4 (symbol `NewCMakeExtractor`, test
  `TestCMakeExtractor_TargetsAndLinks`) to this dialect's symbol and `TestCMakeDialect_KeepsIncumbentSpellings`.
- **Measurement (docs/bench/readings/lang/cmake.json, calibration/corpus.d/cmake.yaml):** see the reading's
  `disposition`; the numbers and every ceiling are written there.
