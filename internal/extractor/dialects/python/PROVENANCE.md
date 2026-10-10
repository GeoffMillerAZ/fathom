# python — provenance

- Grammar: `tree-sitter/tree-sitter-python v0.25.0` (MIT), built into `internal/extractor/tsengine/grammars/python.wasm.gz`;
  pins and digests in `internal/extractor/tsengine/grammars/PROVENANCE.md`.
- `tags.scm`: moved unchanged from `internal/extractor/tsqueries/python.scm` (LB-F1). Written in this
  repository during the tree-sitter migration (ADR-021 §4); the definition shapes follow the
  grammar's own upstream `queries/tags.scm` (MIT), with documentation predicates dropped.
- `tags.scm` upgrade (LB-Q-python, 2026-09): three patterns added, written in this repository
  (not adapted from any upstream query file) — an annotated class attribute with a default
  value (`count: int = 0`, canonical `variable`, ctags kind `variable`) and a PEP 695
  `type X = ...` alias (`type_alias_statement`, canonical `type_alias`, a new `PythonTypeAlias`
  source kind, deliberately absent from `oracle_kinds` since universal-ctags 6.2.1 does not
  recognize the statement at all). A bare unannotated class attribute (`attr = 1`) and an
  annotation-only field with no value (`name: str`) were deliberately left uncaptured — see
  `tags.scm`'s own comments for why (the frozen `TestTagsDialectParity` fixture and a ctags
  precision cost respectively). `testdata/upgrade.py` is the new fixture; `shapes.py` and
  `broken.py` are untouched (`-update` confirmed byte-identical).
- `imports.scm`: written in this repository for SU-11 (`docs/plans/substrate.md` §B, ADR-041
  decision 5); moved unchanged from the tags query it used to share a file with.
- `testdata/`: fixtures written in this repository; the goldens were written by the
  pre-manifest engine at fdbc2ee4 (TestTagsDialectParity).
- `references.scm` (LB-F74, wave lc): written in this repository; no upstream query adapted. The
  file resolver's references (every identifier and dotted attribute path outside a call's text).
- `tags.scm` attributes and fields (LB-F242, wave lx, 2026-10-02): six definition patterns and one
  call attachment, written in this repository (no upstream query adapted), read by the engine's
  member pass (LB-F241, `internal/extractor/tsextract_members.go`, `@member`, `@member.strict`,
  `@member.self`). Every name scip-python 0.6.6 defines on a class or a module that the older
  patterns missed is a symbol named `Owner.name`, once per (Owner, name), at its first definition
  in source order: an unannotated class attribute, every target of an unparenthesized tuple
  assignment and the first of a chained one, and an assignment nested in a block (each tagged
  `variable` by universal-ctags 6.2.1, verified directly, so `PythonVar`); an annotation with no
  value, a chained assignment's later targets and an attribute a method sets on `self` (never
  tagged by it, so the new `field` kind, `PythonField`, canonical `field`, outside `oracle_kinds`).
  A module-level assignment nested in an `if`/`for`/`with`/`try` block is a module variable under
  its own name. The annotated class attribute with a value keeps its bare name
  (TestPythonDialectKeepsLegacySymbols). The two older call attachments (a module-level
  assignment, an annotated class attribute) are one: every assignment statement attaches its
  calls to the definition at its node, so a member's first definition owns the calls on it, as
  scip-python's call truth gives them. The goldens changed by additions only: `shapes.py` gains
  `name` and `other_name` (line 9, PythonVar), `Base.attr` (line 33, PythonVar) and `Base.value`
  (line 36, PythonField); `upgrade.py` gains `Model.name` (line 8, PythonField) and
  `Model.untyped` (line 10, PythonVar); `calls.py` is unchanged; `members.py` is the new fixture.
- `dialect.json` module symbol (K18, LB-F348, wave lf2, 2026-10-10): `module_symbol: "module"` turns
  on the tags engine's module pass (`internal/extractor/tsextract_module.go`), no query change. Each
  file gets one `<module>` symbol, kind `PythonModule` (canonical `module`, outside `oracle_kinds`),
  from line 1 to its last line, owning the calls no definition encloses or claims (a module-level
  statement's, an `if __name__ == "__main__":` block's, a module-level loop's); every `.py` file
  qualifies, an empty `__init__.py` included, as scip-python 0.6.6 defines one `<path>/__init__:`
  per document. The goldens changed by additions only (TestModuleSymbolIsAnAddition): each fixture
  gains its `<module>` at line 1, `members.py` with the module-level call `os.environ.get`;
  `module.py` is the new fixture.
