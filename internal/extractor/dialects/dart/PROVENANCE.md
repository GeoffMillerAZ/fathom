# dart — provenance

- Grammar: `UserNobody14/tree-sitter-dart` at commit `be07cf7118d3dba06236a3f19541685a68209934` (MIT; the repository publishes no
  release tags, so the lock pins the commit, the way kotlin's does), ABI 15, external scanner, `src/parser.c` shipped (7.1 MB), built
  into `internal/extractor/tsengine/grammars/dart.wasm.gz` (914,734 bytes raw, 205,433 gzipped); pin and digests in
  `internal/extractor/tsengine/grammars/grammars.lock.json` (that directory's `PROVENANCE.md` regenerates its own tables from it). On the
  225 real Dart files of the calibration subjects 219 parse without an ERROR or MISSING node; the six that do (2.7%, all in
  dart-lang/http's http2 package) still yield every definition (http2: 985 of 985). The parse is the cost of the dialect: about
  70% of an extraction's CPU time (a query that captures nothing but the root reads 1.0 MB/CPU-s on the same files under load).
- `tags.scm`: started from the grammar's own `queries/tags.scm` (MIT), which tags a class, a mixin, an extension, an enum, a typedef and a
  method but no constructor, accessor, operator, field or enum constant, has a `@reference.call` pattern that matches a call only in a
  cascade, and has no `@body`. The grammar's shape decides everything else (see the file's header): a callable's body is a *sibling* of its
  signature, so a definition is the signature node and its match also captures the `function_body` as `@body` and `@end` (the engine's
  `@end` capture, design §3.3, added for this dialect: the definition then reaches the end of the body and owns its calls); a
  statement is loose `selector` siblings, so a call is a sibling sequence (an identifier followed by an argument selector; a member selector
  followed by an argument selector; the direct-sibling forms after `super.` and inside a cascade; a `new`/`const` expression; and a
  `dot_shorthand` followed by arguments, which is how the grammar reads `Future<void>.delayed(x)`, as `Future < void > .delayed(x)`).
  A constructor's name is the identifier right before its parameter list (a named one has two), and an unnamed one is `Foo.Foo`, the
  name it is called by.
- Kinds: DartClass (classes of every modifier, `mixin class` and `class A = B with C`), DartMixin (canonical trait), DartExtension and
  DartExtensionType (canonical class; an unnamed extension has no symbol and its members are unqualified), DartEnum, DartEnumConstant
  (canonical field: a Dart enum value is a static field, which is how the producer reports it), DartTypedef (both forms), DartFunction
  (top-level, external and local), DartMethod (operators too), DartAccessor (a getter or setter, canonical property), DartConstructor,
  DartField (one per declared name, an `abstract` one too), DartVariable (a top-level `var`, `final`, `const` or typed variable). A
  field, an enum constant and a variable are `callable` so that the calls a closure or an initializer makes (`static T f = (e, m) {
  ... }`, `final s = StreamController<E>.broadcast()`) belong to it, and each has a `Signature` that is the first line of its
  declaration; they are qualified by their class like a method (`Shape.name`, `Color.red`).
- Not extracted, on purpose or by grammar limit: an unnamed extension; a local variable; a parameter; a library-private extension type is
  extracted but the index does not define it as a searchable symbol (one dialect-only definition on http). A comment between a
  signature and its `=>`/`{` body breaks the sibling adjacency and the callable is dropped (one definition of 2,064: web_socket_channel's
  `signKey`); `(comment)*` between the two anchors was tried and matches every later body, so it is not used. A bodiless member's
  `Signature` is its declaration's first line, and a member wrapped in `declaration` ends before its `;`.
- `imports.scm`: written in this repository. `import 'a.dart';` (and `hide`) is a wildcard over the library (owner decision D7's shape),
  `import 'a.dart' as p;` (and `deferred as`) the whole library bound to `p`, `show X, Y` a named import each, a conditional import
  its default uri. `export` and `part` directives are not imports. `imports.strategy: namespace-dir` resolves `package:<name>/<path>`
  through the pubspec.yaml whose `name` is `<name>` (`lib/<path>`) and a bare path relative to the importer
  (`internal/fusion/resolvestrategy.go`, already tested); `dart:` and a foreign package resolve nowhere. A barrel file (`lib/bloc.dart`
  exporting `src/*.dart`) resolves to itself, defines no symbol, and an import through it gives no file-resolved edge (`export` is not
  followed), which is most of the file-edge recall the reading misses.
- `testdata/`: fixtures written in this repository, each with a golden written by hand from reading the fixture before any query
  existed for it: `shapes.dart` every kind, a multi-line signature, an operator, a local function, an `enum` with a constructor and
  members, `mixin class`, `class A = B with C`; `calls.dart` every call shape (plain, member, `?.`, `!.`, `Foo.named`, `new`, `const`,
  cascades, `super.`, `this.`, generic, `await`, `(f)()`, chained, in a closure, in a field or variable initializer, `Foo<T>.named`);
  `imports.dart` each import form; `members.dart` annotated members, fields, accessors, an external method, `mixin ... on`, an
  extension type, an unnamed extension, an enum with members, both typedef forms; `edge.dart` an external top-level function and
  `abstract` fields; `broken.dart` two syntax errors the parser must survive (`Good`, `Survivor` and `tail` extract on both sides of
  them). The goldens matched the engine's output entry for entry on the first run except one: a generic old-style typedef
  (`typedef void Handler<E>(E e)`), which got its own pattern.
- **Measurement (docs/bench/readings/lang/dart.json, calibration/corpus.d/dart.yaml):** scip-dart 1.7.0 (tier A, Dart SDK 3.13.4,
  `calibration/oracles/scip-dart`) for definitions (`-scip-defs`), calls, definition mapping and file edges, on eight pure-Dart packages of
  felangel/bloc bloc-v9.2.1 and dart-lang/http http-v1.5.0 (225 files). universal-ctags 6.2.1 has no Dart parser, so this is the only
  L1 oracle: it is tier A, so L1 can be excellent. `level_met` is L2, L1 and L2 both excellent. L3 and L4 are not reached
  (`LB-Q-dart-rest` in `docs/plans/language-breadth.md`).
- Dispatch: Dart has no incumbent, so `fusion.New` registers `.dart` from the manifest like every dialect (no merge gate); every
  already-indexed `.dart` file was fingerprinted as "no extractor registered", so `ExtractorEpoch` is 25.
