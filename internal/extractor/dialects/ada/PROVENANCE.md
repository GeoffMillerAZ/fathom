# ada — provenance

- Grammar: `briot/tree-sitter-ada`, commit `dd5fa4cdb3aba91abc687aa68fb1431396fce6a6` (MIT). The repository has no release
  tag (its one tag is named `master`), so the pin is the commit, as kotlin's and swift's are; ABI 14, no external scanner,
  `src/parser.c` committed, grammar at the repository root (`tree_sitter_ada`), built into
  `internal/extractor/tsengine/grammars/ada.wasm.gz` (374,900 bytes raw, 107,979 bytes gzip). Pin and digests in
  `internal/extractor/tsengine/grammars/grammars.lock.json`. Licence read at the pinned commit: the top-level `LICENSE.txt`
  (MIT, Emmanuel Briot 2023); the repository ships no licence file under `queries/` or `src/`.
- `tags.scm`: the grammar ships no `tags.scm` (its `queries/` holds folds, highlights, locals and textobjects), so the file is
  drafted from `queries/locals.scm`'s node shapes (a specification's `name:` field, the leading identifier of a type,
  subtype, exception, task and protected declaration) and grown against the grammar's tree with `indexbench lang-probe`.
  It follows universal-ctags' Ada parser, which reports a specification and its body as two entities.
- Kinds and naming. Each entity has a body kind and a `*_spec` kind: `AdaPackage` (`package body`) and `AdaPackageSpec`
  (`package`, a generic package's inner package, a package instantiation and a package renaming), `AdaProcedure` and
  `AdaFunction` (a body, an expression function, `is null`, a generic instantiation, a renaming) and `AdaProcedureSpec`
  and `AdaFunctionSpec` (a declaration, `is abstract`, a generic subprogram declaration, named by its specification),
  `AdaTask` and `AdaTaskSpec`, `AdaProtected` and `AdaProtectedSpec`, `AdaEntry` (an entry body and an `accept` statement,
  the implementation of an entry) and `AdaEntrySpec` (an entry declaration), `AdaType` (every type declaration, private,
  incomplete and private-extension ones too), `AdaSubtype`, `AdaException` and `AdaConstant` (a constant object and a
  number declaration, one per name). A definition is named as written: `Shapes`, `GNATCOLL.Arg_Lists` (a child unit keeps
  its whole path), `"+"` (an operator keeps its quotes). A package, task or protected object qualifies what it contains
  (`Shapes.Registry.Register`); a type, subtype, exception and constant, and a subprogram nested in another, keep their
  bare name. A task body is both a container and a callable, so the calls its statements make belong to it.
- Signature and range. A body's signature stops at its `is` (an entry body's after its barrier: `entry E (X : T) when C`),
  a declaration's at its `;`, an `accept`'s at its `do` or `;` (`accept E (X : T)`), a renaming's at `renames`; a generic
  subprogram declaration is named by its specification alone, so its signature is the specification's first line. Every
  definition carries an `EndLine`.
- Not extracted, on purpose: variables, record components, enumeration literals, loop and block names, generic formals,
  a subprogram body stub (`is separate`, its body is the subunit's own definition), `use` and `with` clauses (L3).
  Calls (L2, not measured for this row): a `procedure_call_statement` or a `function_call` named by its callee's last
  identifier; the grammar reads a type conversion and an index as a call too.
- Oracle (calibration/oracles/README.md): universal-ctags 6.2.1, tier B, run with `--kinds-Ada=+ET` (entry and type
  specifications are off by default). It writes no end line for Ada, so `range_agreement` is not measured and L1 is met at
  most at good. It also loses sync on real bodies: on gnatcoll-core its tags stop after a `declare` block in many files
  (`gnatcoll-json.adb` after line 817), it tags neither a `private package` header nor the types in it
  (`gnatcoll-io.ads`), and it reads `function ... is abstract;` as a body, nesting the next declaration in it. The
  definitions the dialect finds past those points are scored as dialect-only (750 of gnatcoll-core's 881 dialect-only
  definitions, and 146 of alire's 183, come after the last structure tag ctags writes in their file), and ctags tags words of an expression
  (`access`, `declare`, `and`) as constants. It writes a child unit's name whole (`GNATCOLL.Arg_Lists`) where the harness
  compares the dialect's on its last segment, which is why `internal/langbench` compares an oracle's dotted identifier
  path on its last segment too (design §1.4, D2 Ada). An operator function (`"/"`) is the one name the harness splits
  on its own `/`, so its 20 definitions are recall misses. The reading names the counts.
- Syntax errors: 5 of 914 calibration files (0.55%) parse with an ERROR node, all in alire's test-suite fixtures
  (one-line or deliberately malformed crates). A malformed specification inside a body ends the body early, at the next
  `end`; what follows is a unit of its own (`testdata/broken.adb`).
