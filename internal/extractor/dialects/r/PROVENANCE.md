# r — provenance

- Grammar: `r-lib/tree-sitter-r` v1.3.0 (MIT), commit `346d3707b8c9301f1051e8f6e32666e67529f7d2`, ABI 14, external scanner,
  grammar at the repository root (`tree_sitter_r`), built into `internal/extractor/tsengine/grammars/r.wasm.gz` (573,742 bytes
  raw, 107,258 bytes gzip). Pin and digests in `internal/extractor/tsengine/grammars/grammars.lock.json`. Licence read at the
  pinned commit: the top-level `LICENSE` (MIT).
- `tags.scm`: the grammar's own `queries/tags.scm` (r-lib/tree-sitter-r v1.3.0, MIT). Changes: its four assignment patterns
  (`<-` or `=`, an identifier or a string on the left of a `function_definition`) are one, which also takes `<<-`, names a
  string by its content and captures `@body`, the function's body, so a signature stops at the parameter list. The two call
  patterns (a plain `f(x)`, a namespaced `pkg::f(x)` or `pkg:::f(x)`) are upstream's unchanged.
- Naming: a function is named by the identifier as written, a leading dot (`.onLoad`) and backticks included, so the manifest
  compares names whole (`name_match: identity`): the harness's bare rule would cut `.onLoad` to `onLoad`. An S3 method keeps
  its dotted name (`print.point`).
- Not extracted, on purpose: a function held in a list or environment slot (`obj$method <- function`), `->` assignments, S4 and
  R6 classes (`setClass`, `R6Class`) and `setMethod` definitions, `library()` and `source()` edges (L3), variables.
- Dispatch: `.R` and `.r` (the extension lookup is exact, so both spellings are manifest keys) and the `Rscript` shebang.
- Oracles (calibration/oracles/README.md): L1 against universal-ctags 6.2.1 (tier B), L2 against the same upstream tags query
  run by py-tree-sitter (tier C, `calibration/oracles/ts-tags`). The oracle is independent in runtime, not in query shape.
