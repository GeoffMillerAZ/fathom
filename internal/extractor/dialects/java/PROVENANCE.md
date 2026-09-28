# java — provenance

- Grammar: `tree-sitter/tree-sitter-java v0.23.5` (MIT), commit `94703d5a6bed02b98e438d7cad1136c01a60ba2c`,
  ABI 14, no scanner; built into `internal/extractor/tsengine/grammars/java.wasm.gz`
  (401,009 bytes raw, 113,109 bytes gzip — well under the 3 MiB budget). Pin and digests in
  `internal/extractor/tsengine/grammars/grammars.lock.json`
  (`internal/extractor/tsengine/grammars/PROVENANCE.md` regenerates its own tables from it).
- `tags.scm`: written in this repository (LB-Q-java, docs/design/language-breadth.md §13),
  a port of the incumbent CGO `javaextract.go` walker. Started from tree-sitter-java's own
  `queries/tags.scm` (MIT) for the class/method/interface/call shapes, kept close to
  verbatim for those, then added everything the upstream file lacks: enum, record and
  annotation-type declarations, field declarations (one symbol per `variable_declarator`,
  matching the incumbent's own per-declarator walk for `int a, b;`), import declarations as
  a plain definition listing (matching the incumbent), `@scope.package` (design §3.5's own
  worked example is this exact clause), and `@name` inside `@reference.call` on
  `method_invocation` (the incumbent's own `CollectCalls(..., "method_invocation": "name")`
  contract — the callee's simple name only, never a qualifying receiver).
- `imports.scm`: written in this repository (no upstream or incumbent equivalent — the CGO
  walker never resolved imports, only listed them as `KindJavaImport` symbols). The generic
  `@import.module`/`@import.wildcard` vocabulary (LB-F7) on one pattern (kotlin.scm's
  `(wildcard_import)? @import.wildcard` precedent, so a wildcard import is never
  double-captured): `import com.acme.Foo;` binds the whole FQN (`ImportWhole`, same
  precedent as Python's plain `import pkg.mod`); `import com.acme.*;` (or a static
  `import static java.lang.Math.*;`) additionally captures the trailing `asterisk` sibling
  as `@import.wildcard`. `imports.strategy: namespace-dir` (design §6) resolves a
  class-qualified name straight to its file (`com.acme.Foo` -> `.../com/acme/Foo.java`
  under any source root — literally the worked example in
  `internal/fusion/resolvestrategy.go`'s own `resolveNamespaceDir` doc comment) and a
  package-qualified name to every file directly in its directory; already generically
  proven for exactly this shape (`resolvestrategy_test.go`'s `"ns/java-class"` case,
  LB-F11) before this dialect existed.
- **`internal/fusion.resolveImports`'s same-file/cross-file CALL matching never fires for
  Java** (`internal/fusion/javafileresolve_test.go`'s own header comment has the detail):
  it only ever attributes a call to a symbol whose `Name` has no `"."`, and every Java
  method's parent is a container (`qualifyTSDefs`: `"ClassName.method"` always) — Java has
  no top-level functions at all. `TestResolveFileEdges_JavaImports` asserts zero edges
  explicitly, as the correct outcome, not a silently-accepted one. Since
  `cmd/indexbench/langoracle.go` calls `fusion.ResolveFileEdges` directly for the reading's
  own `file_edge_recall`/`file_edge_precision` metrics, this is a foundation-level gap
  shared by any class-based OOP tags dialect (Kotlin, C#, C++, Scala, PHP, Ruby's classes
  too), not specific to this port — the same root cause already measured as Python's
  near-zero `file_edge_recall` (`LB-Q-python-rest`).
- Kept from the incumbent (§11 step 3, port parity): `JavaClass`, `JavaInterface`,
  `JavaMethod`, `JavaEnum`, `JavaRecord`, `JavaField`, `JavaImport` source kinds, unchanged
  strings and semantics (one `JavaMethod` for both `method_declaration` and
  `constructor_declaration`, matching the incumbent's single `qualifyJava` treatment of
  both; wildcard imports drop the `"*"`, matching the incumbent). Added beyond the
  incumbent (a port may add, never drop, §11 step 3): `JavaAnnotation` for
  `annotation_type_declaration` (`@interface Foo { ... }`) — the incumbent's own switch
  never had a case for it. Not extracted (kept out of scope, not a regression — the
  incumbent didn't have these either): `enum_constant` members and
  `annotation_type_element_declaration` (an annotation's own `value()`-shaped elements).
  Fields are emitted **unqualified** (bare name, e.g. `"count"` not `"Widget.count"`): the
  shared engine's `qualifyTSDefs` only chains a container prefix onto a `container`- or
  `callable`-kind definition (§3 design), and no existing dialect's manifest marks a
  plain member field as either — a difference from the incumbent's own qualified fields,
  noted here rather than hidden, and consistent with how `universal-ctags` itself reports
  Java fields (bare, with a separate scope tag).
- `testdata/`: fixtures written in this repository (`shapes.java` every required kind plus
  nesting — a static nested class and its method — a multi-line signature, three call
  shapes (bare, through a local, a qualified static call), a wildcard/static import;
  `imports.java` the full import shape matrix (plain, wildcard, static single-member,
  static wildcard); `broken.java` a syntax error (`@#$` in a parameter list) the parser
  must survive, `Good`/`Survivor` still extracting on either side of it). Goldens hand
  computed from reading each fixture and reasoning through `tsextract.go`'s own
  qualification/signature/sort rules, then cross-checked against
  `indexbench lang-probe`'s raw capture output before the test first ran green.
- **Measurement (`docs/bench/readings/lang/java.json`, `calibration/corpus.d/java.yaml`):**
  universal-ctags 6.2.1 (tier B) on google/guava v33.7.1 (the `guava/src` subpath — 611
  `.java` files) and square/retrofit 3.0.0 (whole repo — 304 `.java` files, including its
  own `java-test`/`samples` trees). `level_met` is `none`; **L1 misses the good floor by a
  hair** — merged `sym_precision` .8964 vs the .90 floor (recall .9741, kind_agreement 1.0,
  range_agreement 1.0, signature_coverage 1.0 all clear excellent) — **root-caused directly,
  not just measured**: universal-ctags 6.2.1's Java parser substantially undercounts LOCAL
  classes (a class declared inside a method body) and their methods. Verified on
  `retrofit/java-test/.../RetrofitTest.java` (1,805 lines, a test file heavy with small
  per-test local classes like `class MyCallAdapterFactory extends CallAdapter.Factory {…}`
  declared inside individual `@Test` methods): ctags tags **1 class / 99 methods** in that
  file; this dialect's real tree-sitter parse — confirmed by hand against the source, the
  extra classes are genuine, distinct local declarations, not a duplicate-emission bug —
  finds **9 classes / 142 methods**. Corpus-wide this shows as fathom emitting 184 more
  classes and 500 more methods than ctags recognizes (retrofit: 577 vs 393 classes, 2,743
  vs 2,243 methods; guava, with far fewer local classes, shows the same direction at a much
  smaller scale) — every extra entry inspected is a real Java construct ctags simply never
  tags, so the precision loss is the ORACLE undercounting, not this dialect
  overcounting. Not query-fixable: the dialect is already more complete than the tier-B
  oracle measuring it. L2–L4 are `not-attempted`: **oracle A (scip-java) was attempted, not
  skipped**, and is unavailable in this environment for two independent, verified reasons —
  (1) retrofit's Gradle build pins `JavaLanguageVersion.of(8)` for its toolchain; Temurin/
  Adoptium has never shipped an arm64 JDK 8 (Apple Silicon postdates JDK 8's active release
  window), and Gradle's own `foojay` auto-provisioning downloads the x64 build but fails to
  unpack it into a usable Java home in this sandbox (installing an x64 JDK 8 via the
  Homebrew cask needs `sudo`, out of scope for a per-item worker); (2) guava's Maven build
  (JDK 21 running Gradle-adjacent Maven 3.9.12 via its own `mvnw`, `--release 9` — no
  toolchain download needed) compiles cleanly (1,965 `.class` files) and `scip-java index
  --build-tool=maven` runs the SAME build successfully through its injected semanticdb
  `javac` wrapper, but reports "No SCIP shards found" — the semanticdb-javac plugin
  produces no output for this JDK21-running/target-9 combination, a scip-java v0.13.1
  compatibility gap this worker's time budget did not chase further.
- **Merge gate (§11 step 2):** the target (L4) is not met — **not even L1 is cleanly met** —
  so the CGO incumbent (`javaextract.go`) stays fusion's live `.java` dispatch
  (`internal/fusion/pipeline.go`); this dialect is written, golden-tested and measured
  (dispatch was pointed at it temporarily, in an uncommitted local edit, only to produce
  this reading; reverted before commit) but not switched in. A future item that lands a
  working scip-java run, or a universal-ctags release that recognizes local classes, or
  both, re-measures this dialect against a materially better oracle before the merge
  decision is revisited.
