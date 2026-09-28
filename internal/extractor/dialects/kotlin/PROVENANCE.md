# kotlin — provenance

- Grammar: `fwcd/tree-sitter-kotlin 0.3.8` (MIT), built into `internal/extractor/tsengine/grammars/kotlin.wasm.gz`;
  pins and digests in `internal/extractor/tsengine/grammars/PROVENANCE.md`.
- `tags.scm`: moved unchanged from `internal/extractor/tsqueries/kotlin.scm` (LB-F1). Written in this
  repository during the tree-sitter migration (ADR-021 §4); the definition shapes follow the
  grammar's own upstream `queries/tags.scm` (MIT), with documentation predicates dropped.
- `testdata/`: fixtures written in this repository; the goldens were written by the
  pre-manifest engine at fdbc2ee4 (TestTagsDialectParity).
