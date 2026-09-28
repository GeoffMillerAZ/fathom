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
