# php — provenance

- Grammar: `tree-sitter/tree-sitter-php v0.24.2` (MIT), commit `5b5627faaa290d89eb3d01b9bf47c3bb9e797dea`
  (the tag the incumbent CGO binding pins), ABI 15, external scanner; the `php/` grammar directory
  (`tree_sitter_php`: PHP with its `<?php ... ?>` and inline-HTML text) is built into
  `internal/extractor/tsengine/grammars/php.wasm.gz` (811,496 bytes raw, 191,343 bytes gzip). Pin and
  digests in `internal/extractor/tsengine/grammars/grammars.lock.json` (that directory's
  `PROVENANCE.md` regenerates its own tables from it). The `php_only/` variant (no inline text) is not
  built: a `.php` file may be a template.
- `tags.scm`: started from tree-sitter-php's own `queries/tags.scm` (MIT) and rewritten around the
  incumbent `phpextract.go`'s kinds. Upstream tags a trait as `@definition.interface`, a method as
  `@definition.function`, a namespace as `@definition.module`, has property and
  `@reference.class`/`@reference.implementation` patterns and no `@body`. This file keeps the incumbent's
  own `SourceKind` strings (`PhpClass`, `PhpInterface`, `PhpTrait`, `PhpEnum`, `PhpFunction`,
  `PhpMethod`), adds `PhpNamespace` (new: the incumbent emitted none) whose name is also the file's
  `@scope.package` (a braced namespace also scopes its own range), recurses into enum bodies (the
  incumbent stopped at `enum_declaration`, so an enum's methods were never definitions), captures
  `@body` for signatures (a bodiless interface or abstract method ends at its `;`) and drops isset, empty,
  die, eval and exit from the call pattern: the grammar parses those language constructs as function
  calls, no index counts them as callables, and PHP keywords are case-insensitive (`#not-match?` with
  `(?i:...)`).
- Calls: the callee is the called name alone (`function:` name, a qualified or `namespace\` relative
  name's last segment, or `name:` of a method, nullsafe or static call), like the incumbent's
  `CollectCalls` map. `new X(...)` is a type reference, not a call. tree-sitter-php has no node that spans
  a receiver and its method (`scoped_call_expression` and `member_call_expression` are flat), so
  `Helper::assist()` reaches the resolver as `assist`, not `Helper.assist`; see the reading.
- `imports.scm`: written in this repository (no upstream or incumbent equivalent). An unaliased
  `use A\B\C;` (also `use function`, `use const`) is a wildcard import of that name, the shape of C#'s
  `using X;` (owner decision D7); `use A\B\C as D;` is a whole import bound to `D`; a grouped
  `use A\B\{C, D as E};` is a named import of the shared prefix per clause. `imports.strategy:
  namespace-dir` with `separator: "\"` resolves a name through composer.json's PSR-4 prefixes
  (`internal/fusion/resolvestrategy.go`'s `resolvePSR4`, already tested) and, for a namespace declared by
  several files, through the files that declare it.
- Kept from the incumbent (§11 step 3, port parity): the six `SourceKind` strings above, method names
  qualified as `Owner.method`, `TierVerified`, sorted output. Added beyond it (a port may add, never drop):
  `PhpNamespace`, enum methods, `Signature` and `EndLine` on every definition, `Package` from the declared
  namespace instead of `pkgFromPath`'s directory guess (a file with no namespace keeps the directory
  rule). Not extracted, on purpose: properties, class constants and enum cases (SCIP defines them, about
  20% of its definitions, so `def_mapping` reads .7875).
- `testdata/`: fixtures written in this repository. `shapes.php` every kind, a namespace clause, a
  constructor with promoted parameters, a multi-line signature, an abstract and an interface method, an
  enum with methods, a guarded function and an anonymous class; `braced.php` braced and global
  namespaces; `calls.php` every call shape (plain, qualified, `namespace\`, method, chained, nullsafe,
  static, `parent::`, a call inside an arrow function, a variable function that is not a call);
  `imports.php` each `use` form; `legacy.php` inline HTML, a by-reference and a reference-returning
  function, a generator, attributes and magic methods; `broken.php` a syntax error the parser must survive
  (`Good` and `Survivor` still extract either side of it). `shapes.php.golden.json` was written by hand
  from reading the fixture and matched the engine's output on the first run; the others were generated with
  `-update` and reviewed line by line against their fixtures.
- **Measurement (docs/bench/readings/lang/php.json, calibration/corpus.d/php.yaml):** a php-parser 5.7.0
  syntax dump (tier B, D2; `calibration/oracles/php-parser`) for L1 and a scip-php index (HEAD
  `71a5b117`, PHP 8.5.8 static CLI, tier A) for calls, definition mapping and file edges, on guzzle 7.9.3
  and monolog 3.9.0 (291 files). `level_met` is L1, excellent (every L1 metric 1.0). universal-ctags 6.2.1
  is not the oracle because it writes no end line for PHP, has no enum kind and reports an enum's methods as
  namespace-level functions (an unscored run matched every class, interface, trait, function and method:
  2,394 of 2,394). L2 is unmet: call_recall 1.0, call_precision .6924, because scip-php leaves method calls
  on vendor-typed receivers unresolved; the same pairs score 1.0 against the php-parser dump.
  `docs/plans/language-breadth.md`'s `LB-Q-php-rest` records the dispositions.
- **Merge gate (§11 step 2):** the target (L4) is not met, so the CGO incumbent (`phpextract.go`) stays
  fusion's live `.php` dispatch (`internal/fusion/pipeline.go`); the dialect is written, tested and
  measured but not switched in. It is registered for imports only: `fileResolveLangFor(".php")` selects
  it, so an ingest's file-resolved pass resolves `use` statements through PSR-4 (the pass reads symbols from
  the store and imports from the dialect). A switch flips `pipeline.go`'s line, deletes `phpextract.go` and
  its test, and bumps `ExtractorEpoch`: the dialect adds a `PhpNamespace` symbol to every namespaced file
  and changes `Package` from the directory to the namespace, so every key of a namespaced file changes.

- Members (LB-F81): `tags.scm` adds `PhpProperty` (a `property_element`, and a promoted constructor
  parameter), `PhpConst` (a `const_element`, so a `const A = 1, B = 2;` group is two) and `PhpEnumCase`, the
  definitions scip-php writes. A property keeps its `$` (`Box.$label`). The engine qualifies a definition
  by its immediate container only, so a promoted constructor property, nested in its method, is the bare
  `$width` here and `Box.$width` in `phpextract.go` (lang-oracle matches bare names; a `member` kind flag in
  `tsextract.go`'s qualification would close it).
