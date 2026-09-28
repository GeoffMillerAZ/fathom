# ruby — provenance

- Grammar: `tree-sitter/tree-sitter-ruby v0.23.1` (MIT), built into `internal/extractor/tsengine/grammars/ruby.wasm.gz`;
  pins and digests in `internal/extractor/tsengine/grammars/PROVENANCE.md`.
- `tags.scm`: moved unchanged from `internal/extractor/tsqueries/ruby.scm` (LB-F1). Written in this
  repository during the tree-sitter migration (ADR-021 §4); the definition shapes follow the
  grammar's own upstream `queries/tags.scm` (MIT), with documentation predicates dropped.
- `testdata/`: fixtures written in this repository; the goldens were written by the
  pre-manifest engine at fdbc2ee4 (TestTagsDialectParity).
