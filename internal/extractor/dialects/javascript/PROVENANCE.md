# javascript — provenance

- Grammar: `tree-sitter/tree-sitter-javascript v0.25.0` (MIT), built into `internal/extractor/tsengine/grammars/javascript.wasm.gz`;
  pins and digests in `internal/extractor/tsengine/grammars/PROVENANCE.md`.
- `tags.scm`: moved unchanged from `internal/extractor/tsqueries/javascript.scm` (LB-F1). Written in this
  repository during the tree-sitter migration (ADR-021 §4); the definition shapes follow the
  grammar's own upstream `queries/tags.scm` (MIT), with documentation predicates dropped.
  Extended in this repository (LB-Q-javascript, the L4 measurement pass) with two
  assignment-based `@definition.method`/`@definition.function` patterns — a prototype
  method (`Foo.prototype.bar = function(){}`) and a single-level member assignment
  (`app.foo = function(){}`, `exports.foo = function(){}`) — the express/axios
  calibration idioms; a non-prototype multi-level member chain and a bare identifier
  assignment were deliberately left out (they collide with TestTagsDialectParity's
  frozen legacy fixtures; see `docs/plans/language-breadth.md` id `LB-Q-javascript-rest`
  and the reading's `disposition`).
  Scoped in this repository (LB-Q-javascript-rest): an arrow-function or function-expression
  binding is a definition only at module scope (a direct child of the program or of an export
  statement), a method only in a class body, an assignment only as a module-scope statement (plus
  the inner assignment of a module-scope chain), and no object-literal member is one; the header
  of `tags.scm` records the measured reason.
- `imports.scm`: written in this repository for SU-11 (`docs/plans/substrate.md` §B, ADR-041
  decision 5); moved unchanged from the tags query it used to share a file with.
- `testdata/`: fixtures written in this repository; the goldens were written by the
  pre-manifest engine at fdbc2ee4 (TestTagsDialectParity). `assign.js` added in this
  repository (LB-Q-javascript) for the two new assignment-based patterns above. `scope.js`
  added in this repository (LB-Q-javascript-rest) for the scope rule; since then the goldens are
  the full ones TestDialectGoldens checks, and TestJavaScriptDialectKeepsLegacySymbols holds
  every symbol the pre-manifest goldens listed except the enumerated locals and object-literal
  keys.
