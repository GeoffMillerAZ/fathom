# cpp — provenance

- Grammar: `tree-sitter/tree-sitter-cpp v0.23.4` (MIT), commit `f41e1a044c8a84ea9fa8577fdd2eab92ec96de02`,
  ABI 14, with its `src/scanner.c`; built into `internal/extractor/tsengine/grammars/cpp.wasm.gz`
  (1,935,648 bytes raw, 420,154 bytes gzip). Pin and digests in
  `internal/extractor/tsengine/grammars/grammars.lock.json`
  (`internal/extractor/tsengine/grammars/PROVENANCE.md` regenerates its own tables from it).
  The same tag the incumbent's Go binding (`go.mod`) pins, so the two walk one grammar.
- `tags.scm`: written in this repository (LB-Q-cpp, docs/design/language-breadth.md §13), a port of the
  incumbent CGO `cppextract.go` walker. Started from tree-sitter-cpp's own `queries/tags.scm` (MIT) and replaced
  almost entirely: upstream tags every `function_declarator` (a prototype and a call-shaped declarator
  included), puts unions under "class" and enums/typedefs under "type", has no body check (a forward
  declaration would tag) and no namespace, macro, alias or call capture. This file instead keeps the incumbent's
  semantics — a class/struct/union/enum/function needs a body (the incumbent skipped a bodiless struct and enum;
  it emitted a forward-declared `class`, which ctags does not tag and this dialect does not either), out-of-class
  members carry their class — and adds namespaces, `#define` macros, `typedef`/`using` aliases, unions and
  `@reference.call` (a plain, scoped, member or template-instantiated callee; C++ casts, all-capitals
  function-like macro invocations and single-capital type-as-function calls are not calls).
  Aider's `cpp-tags.scm` was not used.
- `imports.scm`: written in this repository. `#include "x"` and `#include <x>` through the generic
  `@import.module`/`@import.stmt` vocabulary, resolved by the `include-path` strategy (design §6;
  `internal/fusion.TestResolveFileEdges_CppIncludes`).
- **Naming — parity with the incumbent, and where the dialect is stricter or larger.**
  `CppFunc` (free functions, inline methods, out-of-class members), `CppClass`, `CppStruct`, `CppEnum` and
  `CppNamespace` are the incumbent's source-kind strings, unchanged; added beyond it (a port may add, never
  drop, §11 step 3): `CppUnion`, `CppTypedef` (`typedef` and `using` aliases) and `CppMacro` (`#define`).
  The CT-1/CT-2 contracts (`docs/plans/cpp-toolchain.md`) hold: `"::"` never survives in a symbol or callee
  (`Widget::render` extracts as `Widget.render`, `ns::util()` as `ns.util`), template argument lists are
  dropped from a symbol's key (`Box<T>::make` is `Box.make`), and nested namespaces and inline and out-of-class
  members qualify identically. This needed one engine field, `qualify.scope_sep` (`"::"`;
  `internal/extractor/scopesep.go`, unit-tested, validated by `schema/dialect/dialect.cue`): the engine
  took `@name` verbatim, so `Widget::render` could not have matched `Widget.render`.
  **One deliberate difference:** the dialect makes a namespace a *container*, so a definition inside
  `namespace ns` is `ns.Name` where the incumbent (whose walker never pushed a namespace onto its scope stack)
  said `Name`. Two namespaces of one file that define the same name were one symbol key under the incumbent;
  an inline member, a member defined in the namespace block and one defined as `ns::C::m()` were three
  spellings; and a call `util::helper()` could only resolve to the namespace `util`, not to `util.helper`.
  All four now agree. A switch of dispatch to this dialect must bump `ExtractorEpoch` (a re-index renames every
  namespaced symbol). `TestCppDialectKeepsIncumbentContracts` runs the incumbent's own scenarios through both
  extractors and states this one difference.
- `testdata/`: fixtures written in this repository, goldens written by hand from reading each fixture
  before they were compared to the extractor: `shapes.cpp` (every kind; containment through namespaces,
  classes and templates; operators; a multi-line signature; extern "C"; out-of-class members including an
  explicit specialisation), `templates.hpp` (class templates, a specialisation, an alias template, defaulted
  members, `[[attributes]]`, inheritance), `calls.cpp` (plain, `.`/`->`, scoped, template, cast, lambda and
  local-callable calls), `includes.cpp` and `broken.cpp` (a syntax error the parser survives).
- **Measurement (`docs/bench/readings/lang/cpp.json`, `calibration/corpus.d/cpp.yaml`):** universal-ctags
  6.2.1 (tier B) and scip-clang 0.4.0 (tier A, the `scip-clang-arm64-darwin` release asset) on
  fmtlib/fmt 12.2.0 (its bundled `test/gtest` excluded as vendored code, `-only`) and nlohmann/json v3.12.0
  (`include/`). scip-clang needs a `compile_commands.json`; neither library has one and this worktree has no
  `cmake`, so each was written by hand — fmt: `src/format.cc`, `src/os.cc` and a synthetic translation unit
  including every public header; json: one translation unit using `json.hpp`'s API — every entry indexed
  with 0 errored TUs. `.h` is no dialect's extension (design §5 leaves it to the CT-3 header router), so
  `lang-oracle` measures a C++ subject's `.h` headers when that router's sniff sends them to C++
  (`ownedSourceRefs`); without it fmt's public headers, most of the library, were measured as nothing.
  **Read `docs/bench/readings/lang/cpp.json` for the numbers**; what they say, in short: L1 excellent (the plan
  row needs L4, so it stays open and `LB-Q-cpp-rest` records the disposition); L2 call_recall .806 clears
  good but call_precision .677 does not, because scip-clang cannot resolve a call whose callee depends on a
  template parameter (`buf.size()`, `out(...)`, `it->begin()` inside a template body — the bulk of fmt and
  json) and reports no constructor call (`throw format_error(...)`, `string_view(x)`) as a callable
  reference, while fathom, structural, sees both; L3 is unmeasurable (`import_resolution` has no in-repo spec
  count yet, LB-F11) and its file-edge metrics miss; L4's typed_recall is 0.04 because
  `internal/scip.ImportIndex` attributes a reference by `enclosing_range`, which scip-clang never sets (the
  same product gap as `LB-Q-c-rest`).
- **Parity with the incumbent (`indexbench lang-parity -dialect cpp -incumbent cppextract`, same subjects,
  fmt unrestricted):** sym_recall .5318 → .9473, sym_precision .9064 → .9696, kind_agreement .9890 → 1.0,
  range_agreement .9942 → .9948, signature_coverage 0 → 1.0; kinds gained `CppMacro`, `CppTypedef`,
  `CppUnion`; none lost. Verdict REFUSED for one reason only: target L4 is not met.
- **Measured upper bound of an open follow-up.** A macro on a line of its own before a declaration
  (`FMT_BEGIN_EXPORT`, `NLOHMANN_JSON_NAMESPACE_BEGIN`, `JSON_HEDLEY_NON_NULL(3)`) makes tree-sitter open an
  ERROR node that swallows the next definitions; universal-ctags, being lexical, does not care. Blanking
  lines that hold only an all-capitals identifier (same length, offsets kept) before parsing, tried on a
  scratch copy of both subjects, lifts sym_recall .9523 → .9692 and sym_precision .9674 → .9857. It needs a
  lexical pre-pass in the engine, which this item does not add (recorded in `LB-Q-cpp-rest`).
- **Dispatch (ADR-048 amendment 2026-10-01, D1):** lang-parity against the CGO `cppextract.go` on fmt, json and
  spdlog reads every metric ahead but range_agreement (0.9921 < 0.9923, `docs/bench/readings/lang/parity/cpp.json`),
  so the incumbent stays fusion's live `.cc`/`.cpp`/`.cxx`/`.hpp`/`.hh`/`.hxx` dispatch and the C++ branch of the
  `.h` router. The range misses are the macro misparses above (FMT_TRY/FMT_CATCH, a decorator before a
  constructor); the lexical pre-pass `LB-Q-cpp-rest` names is the trigger. A switch bumps `ExtractorEpoch`
  (namespaces become containers) and may then retire `cppextract.go`.
  Re-checked 2026-10-02 (wave ld): this file and `cppextract.go`'s symbol path are unchanged since the parity run
  (only the EW-53 file specs), so the refusal stands without a re-run; O455 names the pre-pass exactly (a
  `cpp-macros` entry in the tags engine's `tsPreprocessors`, equal-length rewrites, and this dialect's `preprocess`).
