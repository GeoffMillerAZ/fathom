# cpp-toolchain — full C++/CMake/clang/LLVM/Ninja support

> **Archived** 2026-10-10T00:48:51Z · HEAD `8415dfd60d7a59f964220ed53617941694bc69be` · 13 tasks total; 9 claimed (9 verified, 0 failing); 4 skipped

> Owner directive 2026-08-08: "I want to make sure we have full support for
> cpp, cmake, clang, llvm and ninja." This fires IO-27's deferral trigger
> (docs/plans/impact-observatory-activation.md) with one redirect: the
> toolchain of record is CMake + clang/LLVM + Ninja, not Conan — Conan
> stays deferred on its own trigger below.
>
> Starting state, surveyed: CppExtractor (tree-sitter CGO, ADR-013) already
> emits namespaces/classes/structs/enums/functions with call collection;
> the call graph is language-agnostic. The gaps are spelling (`::` vs `.`),
> `.h` routing, CMake, toolchain awareness, and the languages field.
>
> **Covenant (ADR-0070 class):** failing test first; `task validate` green
> before every commit; dated CHANGELOG entry; boxes flip only on
> `fathom verify`.

## A — C++ extraction closes its gaps

- [x] one spelling per symbol: out-of-class definitions and collected callees normalize C++ scope `::` to `.` so graph edges join inline and out-of-class definitions {id: CT-1, symbol: cppCanonicalName, package: internal/extractor, reachable_from: TestCppCanonicalNameWiredDirect}
- [x] templates, operators, destructors, and nested namespaces extract and are pinned by tests {id: CT-2, symbol: walkCpp, package: internal/extractor, reachable_from: TestWalkCppWiredDirect}
- [x] polyglot .h routing: a content-sniffing header extractor sends C++-signal headers to the C++ grammar and plain C headers to the C grammar — LLVM-style codebases keep their classes {id: CT-3, symbol: NewCHeaderExtractor, package: internal/extractor, reachable_from: TestCHeaderRouting}

## B — CMake is a dialect

- [x] CMakeLists.txt and *.cmake extract: projects, targets (executable/library/custom), functions/macros, options, find_package dependencies; target_link_libraries becomes target-to-target call edges {id: CT-4, symbol: NewCMakeExtractor, package: internal/extractor, reachable_from: TestCMakeExtractor_TargetsAndLinks}
- [x] CMake is wired end-to-end: pipeline dispatch by basename and extension, connector classification, and the languages field reports cmake {id: CT-5, symbol: LanguageForPath, package: internal/connector, reachable_from: TestLanguageForPath_CMake}
- [x] the languages field stops lying for C-family services: kindToLang maps cpp, c, cmake, rust, csharp, cue {id: CT-6, symbol: kindToLang, package: internal/mcp, reachable_from: TestKindToLang_CFamilyAndCMake}

## C — Toolchain awareness (clang / LLVM / Ninja)

- [x] doctor gains a cpp_toolchain check: on a CMake repo it verifies compile_commands.json exists (root or build/) and names the exact remedy — cmake -S . -B build -G Ninja -DCMAKE_EXPORT_COMPILE_COMMANDS=ON — on a non-CMake repo it skips honestly {id: CT-7, symbol: checkCppToolchain, package: cmd/fathom, reachable_from: TestCheckCppToolchain}
- [x] init's .fathomignore template excludes build trees: build/, out/, cmake-build-*/ — a configured build tree never double-indexes copied headers and generated sources {id: CT-8, symbol: writeFathomIgnore, package: cmd/fathom, reachable_from: TestFathomIgnoreDefaults_BuildTrees}
- [-] a build.ninja extractor {id: CT-9, disposition: refused, reason: "build.ninja is generated output — the intent it encodes already lives in CMakeLists.txt, which we index; indexing artifacts violates the same doctrine that keeps generated YAML out of source control"}

## D — Calibration (the honesty gate)

- [x] calibrated against a real CMake plus C++ corpus (fmt): C++ classes inside .h headers present post-CT-3, CMake targets and link edges present, symbol and edge counts recorded; the throwaway service is removed from the registry after {id: CT-10, oracle: lexical, path: CHANGELOG.md, pattern: "calibrated live against fmtlib/fmt: 5,470 symbols"}

## E — Deferred with triggers

- [-] precise clang mode: compile_commands.json-driven libclang or scip-clang extraction for typed C++ edges {id: CT-11, disposition: deferred, reason: "the heuristic tier must prove insufficient before we take a toolchain-heavy dependency", trigger: "calibration or a real consumer shows heuristic edge quality below the decision bar"}
- [-] C++ on the wazero/WASM extraction engine per ADR-021 section 4 {id: CT-12, disposition: deferred, reason: "the CGO extractor works today; migration is an engine-consistency wave, not a capability gap", trigger: "next extraction-engine wave or a CGO-hostile deploy target"}
- [-] Conan dependency adapter from vision 15 {id: CT-13, disposition: deferred, reason: "trigger FIRED 2026-10-01 by owner directive ('especially well in c and cpp and Conan'): the format work moves to language-breadth.md LB-Q-conan (with LB-Q-vcpkg, LB-Q-meson, LB-Q-compile-commands); earlier: owner redirect 2026-08-08, CMake-first, no Conan consumer in the fleet", trigger: "fired - see language-breadth.md LB-Q-conan"}

## Calibration record

CT-10 live run, 2026-08-08, corpus fmtlib/fmt @ shallow HEAD (142 files),
isolated scratch store (never the fleet registry):

- 5,470 symbols total; C++ kinds: CppFunc 4,004 · CppStruct 457 ·
  CppClass 424 · CppNamespace 181 · CppEnum 40 — the classes and
  namespaces come from `.h` headers (fmt keeps ALL headers in .h), which
  the C grammar previously flattened to functions/structs. CFunc 15
  (genuinely C-flavored headers, correctly routed).
- CMake: 24 targets, 12 options, 10 functions; target_link_libraries
  edges land normalized (a target Calls ["fmt.fmt"], no "::" anywhere:
  the contamination query returned 0 rows).
- languages reported: [c, cmake, cpp, python] — CT-5/CT-6 live.
- Friction filed while running it: the ingest CLI's -store default is
  cwd-relative and silently created workspace/db/ inside the corpus
  clone (observation O12, the O11 class).
