# javascript — provenance

- Grammar: `tree-sitter/tree-sitter-javascript v0.25.0` (MIT), built into `internal/extractor/tsengine/grammars/javascript.wasm.gz`;
  pins and digests in `internal/extractor/tsengine/grammars/PROVENANCE.md`.
- `tags.scm`: moved unchanged from `internal/extractor/tsqueries/javascript.scm` (LB-F1). Written in this
  repository during the tree-sitter migration (ADR-021 §4); the definition shapes follow the
  grammar's own upstream `queries/tags.scm` (MIT), with documentation predicates dropped.
- `imports.scm`: written in this repository for SU-11 (`docs/plans/substrate.md` §B, ADR-041
  decision 5); moved unchanged from the tags query it used to share a file with.
- `testdata/`: fixtures written in this repository; the goldens were written by the
  pre-manifest engine at fdbc2ee4 (TestTagsDialectParity).
