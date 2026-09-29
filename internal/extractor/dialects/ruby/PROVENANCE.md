# ruby — provenance

- Grammar: `tree-sitter/tree-sitter-ruby v0.23.1` (MIT, commit `71bd32fb`), built into
  `internal/extractor/tsengine/grammars/ruby.wasm.gz`; pins and digests in
  `internal/extractor/tsengine/grammars/PROVENANCE.md`. It parses every one of the 316 files of the two
  calibration subjects without an ERROR node.
- `tags.scm`: the module, class, method and singleton-method shapes follow the grammar's own upstream
  `queries/tags.scm` at that commit (MIT) and the previous version of this file (written in this
  repository during the tree-sitter migration, ADR-021 §4, moved unchanged from
  `internal/extractor/tsqueries/ruby.scm` by LB-F1). Written for LB-Q-ruby: `@body` on `def` and
  `define_method`, `alias` and `alias_method` (the upstream file tags only the `alias` keyword), `attr_*`
  properties (one per symbol argument, the symbol itself as the definition), `define_method(:x)` and
  `define_method("x")`, constants (`CONST = value`, a container so `Outer::CONST` reads as Ruby writes it
  and a `Struct.new do ... end` block's methods qualify under it), `Name = Class.new` / `Module.new`
  (and the same bound to a local), a constant as a call target, and no `def obj.x` (a singleton method on
  a local has no owner to qualify it). Two upstream call shapes are not taken: the bare-identifier
  reference (`[(identifier) (constant)] @name @reference.call` with `(#is-not? local)`) needs the locals
  query's scope analysis, which the engine does not run, so an identifier is left out — a local variable
  read cannot be told from a call — and constants are taken without the local test, as upstream takes
  them.
- `imports.scm`: written for LB-Q-ruby: `require`, `require_relative`, `load` and `autoload` with a
  literal string path and no receiver.
- `testdata/`: fixtures written in this repository. `shapes.rb` and `broken.rb` predate LB-Q-ruby (their
  goldens were written by the pre-manifest engine at fdbc2ee4; every symbol they listed is still held by
  `TestRubyDialectKeepsLegacySymbols`, the new symbols were added by hand); `members.rb`, `calls.rb`,
  `imports.rb` and `edge.rb` were written for it, with their goldens typed by hand from reading each
  fixture. The engine's output matched them except for one import `calls.rb` carries and its golden
  omitted (a slip in the golden, not the query).
- Oracles: universal-ctags 6.2.1 (tier B, L1), the grammar's own `queries/tags.scm` and `locals.scm` run by
  py-tree-sitter (`calibration/oracles/ts-tags/`, tier C, L2), and scip-ruby 0.5.0
  (`calibration/oracles/scip-ruby/`, tier A, run for L3/L4 and disclosed); the reading is
  `docs/bench/readings/lang/ruby.json`.
