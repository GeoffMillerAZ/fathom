# c — provenance

- Grammar: `tree-sitter/tree-sitter-c v0.24.2` (MIT), commit `b780e47fc780ddc8da13afa35a3f4ed5c157823d`,
  ABI 15, no scanner; built into `internal/extractor/tsengine/grammars/c.wasm.gz`
  (564,097 bytes raw, 136,140 bytes gzip). Pin and digests in
  `internal/extractor/tsengine/grammars/grammars.lock.json`
  (`internal/extractor/tsengine/grammars/PROVENANCE.md` regenerates its own tables from it).
- `tags.scm`: written in this repository (LB-Q-c, docs/design/language-breadth.md §13),
  a port of the incumbent CGO `cextract.go`/`cheaderextract.go` walkers. Started from
  tree-sitter-c's own `queries/tags.scm` (MIT) but replaced almost entirely: upstream tags
  struct/union as canonical "class" with no body check (so a forward declaration would tag),
  enum/typedef as canonical "type", has no macro capture and no call capture at all. This file
  instead: keeps the incumbent's own semantics (a struct/union/enum only counts with a body,
  matching `ChildByFieldName("body") == nil` skip in `cextract.go`), gives struct/union/enum/typedef
  their own canonical kinds, adds `#define` object-like and function-like macros
  (`preproc_def`/`preproc_function_def` — new, no upstream or incumbent equivalent), and adds
  `@reference.call` for the same naive same-file call collection `treesitter.CollectCalls` gave
  the incumbent's `function_definition` walker.
- `imports.scm`: written in this repository (no upstream or incumbent equivalent — the CGO
  walkers never resolved imports). `#include "local.h"` and `#include <system.h>` via the
  generic `@import.module`/`@import.stmt` vocabulary (LB-F7); `imports.strategy: include-path`
  (design §6) resolves the quoted form relative to the includer, then any `-I` root
  `compile_commands.json` names, then a sibling `include/` directory — `internal/fusion/resolvestrategy.go`'s
  `resolveIncludePath`, already generically tested (`TestResolveStrategies`, LB-F11).
- Kept from the incumbent (§11 step 3, port parity): `CFunc`, `CStruct`, `CEnum`, `CTypedef`
  source kinds, unchanged strings. Added beyond the incumbent (a port may add, never drop,
  §11 step 3): `CUnion` (the incumbent's `cextract.go` never emitted unions at all — a
  regression *fixed*, not introduced) and `CMacro` (`#define`, likewise never emitted before).
  The CT-3 `.h` router (`cheaderextract.go`) is kept as designed (LB-Q-c's own queue note):
  it still sniffs C++-only signals in an ambiguous `.h` and delegates to the C++ extractor on
  a hit; since the dispatch switch below its C branch is this dialect (`NewCHeaderRouter`).
- `testdata/`: fixtures written in this repository (`shapes.c` every required kind plus a
  multi-line signature and three call shapes — plain, function-pointer-through-a-struct-field
  and a macro-shaped call; `includes.c` a quoted and an angle-bracket include;
  `broken.c` a syntax error the parser must survive, `good`/`Survivor` still extracting either
  side of it). Goldens hand-written from `indexbench lang-probe`'s capture and tree output,
  reviewed against the fixtures before any test ran green.
- **Measurement (docs/bench/readings/lang/c.json, calibration/corpus.d/c.yaml):**
  universal-ctags 6.2.1 (tier B) and scip-clang 0.4.0 (tier A, the
  `scip-clang-arm64-darwin` release asset — no LLVM build needed) on libuv v1.53.0 (331
  `.c` files). scip-clang needs a `compile_commands.json`; libuv has no cached one and this
  worktree has no `cmake`, so one was hand-built from `CMakeLists.txt`'s own `APPLE`
  file-list branches (37 translation units: the 12 shared + 18 generic-unix + 7 Apple-only
  `.c` files `cmake -DCMAKE_EXPORT_COMPILE_COMMANDS=ON` would select on this platform) —
  every entry compiles clean (`clang -fsyntax-only`, 0 errors) and scip-clang indexed all 37
  with 0 errored TUs. `level_met` is `none`; L1 is `good` (sym_recall .8612, capped by a
  universal-ctags heuristic — libuv's `TEST_IMPL`/`BENCHMARK_IMPL` test macros tag as a
  function named by the macro itself, which a real parse correctly does not do — not a
  missed kind); L2/L4 measure 0 because scip-clang never sets `occurrence.enclosing_range`
  (verified directly: 5,716 definition occurrences, 0 with one), which
  `internal/langbench.ScoreSCIP`'s call-pair attribution requires — a foundation-level gap,
  not a query fix. Full detail in the reading's own `notes`/`disposition`.
  `docs/plans/language-breadth.md`'s `LB-Q-c-rest` records the same disposition.
- **Dispatch (ADR-048 amendment 2026-10-01, D1; LB-F119):** lang-parity against the CGO `cextract.go` on libuv and
  curl reads the dialect at least as good on every metric up to its demonstrated level, L2 (MERGE,
  `docs/bench/readings/lang/parity/c.json`): fusion serves `.c` from this dialect and the CT-3 router
  (`NewCHeaderRouter` in `cheaderextract.go`) sends a C header to it, a C++ one to the C++ extractor.
  `cextract.go` stays as lang-parity's incumbent; `ExtractorEpoch` 45 re-reads every `.c` and `.h` file.
- **References (LB-F74 mechanism, LB-F196, 2026-10-02):** `references.scm` is written in this repository against
  tree-sitter-c's grammar (no upstream query): every type name (`type_identifier`) and every all-capitals word
  (`identifier` matched `^[A-Z][A-Z0-9_]*$`: macros and conventionally upper-case enumerators) the file uses outside a
  call. `queries.references` in `dialect.json` names it; `internal/fusion` binds a name to the definition in a header
  the file reaches through its include graph (`refresolve.go` `matchRef`). A lower-case enumerator or global variable
  is no definition the tags query yields, so it is not captured. The `.h` sniff (`looksLikeCpp`) reads code only
  (comments, string and character literals and `#error` text blanked), so a C header whose comment says "class" stays C.
