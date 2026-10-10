# scala — provenance

- Grammar: `tree-sitter/tree-sitter-scala` at tag `v0.26.2` (commit `b931fcc338390925eb893d70ad070033f5856ccf`,
  MIT, the licence checked at the tag with `indexbench lang-pin -tree`), built into
  `internal/extractor/tsengine/grammars/scala.wasm.gz` (3,125,460 bytes, 633,687 gzipped, ABI 15, external scanner);
  pins and digests in `internal/extractor/tsengine/grammars/PROVENANCE.md`. The CGO walker `scalaextract.go` it
  replaces bound v0.26.0 through the Go module; v0.26.2 is pinned instead because it parses the calibration
  subjects far more cleanly: files with an ERROR node fall from 16 of 835 (1.9%) to 2 on typelevel/cats v2.13.0 and
  from 6 of 667 (0.9%) to 0 on http4s/http4s v0.23.38 (the kill line is 5%). The two left in cats
  (`core/src/main/scala/cats/Eval.scala`, `tests/shared/src/test/scala-3/cats/tests/FunctionKLiftSuite.scala`)
  still yield every definition outside the error.
- `tags.scm`: the definition shapes start from the grammar's own `queries/tags.scm` at that tag (MIT): classes,
  objects, traits, enums, enum cases, functions, vals, vars, givens and type definitions. Written in this repository
  for LB-Q-scala, and different from upstream: every SourceKind `scalaextract.go` emitted is kept byte-identical
  (ScalaClass for a class or case class, ScalaObject, ScalaTrait, ScalaDef); a name is any node, so an operator
  (`def +`, `object :==`, `val ->`) and a backticked name (`def `a b``, without its backticks through
  `names.trim_quotes`) are definitions; abstract members (`function_declaration`, `val_declaration`,
  `var_declaration`), package objects (ScalaObject) and a secondary constructor (`def this`, ScalaConstructor) are
  read; a val or var counts only as a member (a template body, a given's `with` body, an enum body, the file's top
  level) and never as a local; a class parameter is a field when written `val` or `var`, and any parameter of a case
  class or of an enum case with parameters is a ScalaVal; the Signature ends before `=`; and every call names its
  callee by the last identifier (upstream's `@reference.call` reads only a plain `f(x)`). Upstream's
  `class_parameter` (every parameter a property) and `package_clause` (a module definition) are not used: a plain
  constructor parameter is no member, and a package is file granularity, never a symbol (K18).
- Kinds beyond the walker's four: ScalaEnum, ScalaEnumCase (a member of its enum, `Color.Red`), ScalaGiven (a named
  given; its members qualify against it, `intShow.show`; an anonymous given names nothing and its members qualify
  against the enclosing template, as an extension's do), ScalaConstructor, ScalaVal, ScalaVar, ScalaType. Nested
  containers chain (`Outer.Inner.m`, Kotlin's and Java's rule); the walker named a nested class bare and a method
  after its nearest container only. A val, var, type or field is named bare, the engine's rule for a definition that
  is neither a container nor a callable.
- Package: the directory rule (`package.strategy` `dir`, the file's stem, as the walker had it). A file's package
  clauses are siblings in this grammar (`package cats` then `package data` means `cats.data`), and the engine's
  `capture` strategy keeps the first file-wide clause, so capturing would name `cats` for every file of
  `cats.data`; the package and imports belong to L3 (LB-Q-scala-rest). No `imports.scm` or `references.scm`: the
  dialect is measured at L1 and L2.
- Match: `.scala` and `.sc`, the walker's claim. linguist also lists `.sbt`, `.kojo` and a `scala` shebang; they
  stay unclaimed (an sbt build definition is a build tool's format, not this item's).
- `testdata/`: fixtures written in this repository for LB-Q-scala, their goldens typed by hand from reading each
  fixture before the queries existed. The engine's output matched them except for nine end lines, all of one cause:
  this grammar ends an indentation-delimited node (a body on the line after `=`, a Scala 3 template) at the next
  token's position, after the blank lines and the indentation that follow it, and the engine counts a node to its
  last byte, so `EndLine` reads the next definition's line (or a blank one) instead of the body's last. The goldens
  record the engine's output; the scalameta dump agrees with the hand-typed end lines. That is the whole of the
  reading's range_agreement gap (.8042, L1 good; .9511 when trailing whitespace is trimmed, O761).
- Oracle: `calibration/oracles/scalameta/` (tier B: definitions and calls), with the reasons universal-ctags 6.2.1
  (no Scala parser) and scip-java (an sbt build) are not; the grammar's upstream `tags.scm` run by py-tree-sitter
  (`calibration/oracles/ts-tags/scala-tags.scm`, tier C) is a disclosed calls cross-check. The reading is
  `docs/bench/readings/lang/scala.json`; lang-parity against the walker `docs/bench/readings/lang/parity/scala.json`.
