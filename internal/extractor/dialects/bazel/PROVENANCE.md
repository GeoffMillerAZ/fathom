# bazel — provenance

LB-Q-bazel, design `docs/design/language-breadth.md` §13. A tags-engine tool format, new (no incumbent).

- Grammar: `tree-sitter-grammars/tree-sitter-starlark` tag `v1.3.0`, commit `a453dbf3ba433db0e5ec621a38a7e59d72e4dc69` (MIT;
  the top-level `LICENSE`, `indexbench lang-pin -tree` reads it as MIT), ABI 14, external scanner (`src/scanner.c`),
  `src/parser.c` shipped (no generate step), grammar at the repository root (`tree_sitter_starlark`), built once into
  `internal/extractor/tsengine/grammars/starlark.wasm.gz` (433,933 bytes raw, 107,943 gzip) and shared with the tiltfile
  dialect. The embedded grammars now total 8,593,670 gzip bytes of LB-M4's 40 MiB cap. The catalogue
  (`docs/plans/language-breadth.grammars.json`) pre-filled the commit `a453dbf` that v1.3.0 resolves to.
- `tags.scm`: written in this repository; the grammar ships `queries/` with folds, highlights, indents, injections and
  locals only. Node shapes came from `indexbench lang-probe` over `testdata/`, never from `grammar.js`. A top-level call
  with a literal `name = "x"` is the target x (BUILD files; `selects.config_setting_group(name = ...)` too); the same shape
  with `bazel_dep`, `module`, `workspace`, `http_archive` and the other repository rules is a dependency, `package_group`
  a package group, `x = rule(...)` (also `repository_rule`, `module_extension`, `aspect`, `macro`, `provider`,
  `tag_class`) a rule and `def` a function. A statement's callee (`cc_library`, `my_macro`, `native.cc_library`) is its
  call, so a `load()` binds it; a label spelled `":y"` in a top-level call's arguments (a string, a list, a list joined
  with `+`, a `select()` branch) is a reference to the target y, its name read without the colon (`names.strip_prefix`).
- Kinds: `BazelTarget`, `BazelFunction`, `BazelRule`, `BazelPackageGroup` (policy), `BazelDependency` (import). `.bzl`,
  `BUILD`, `BUILD.bazel`, `MODULE.bazel`, `WORKSPACE`, `WORKSPACE.bazel` and `WORKSPACE.bzlmod` are classified (connector.go)
  as language `bazel`, typed-edge level `syntax` (ingest's file-resolve pass is keyed by extension, O379).
- Imports: `load("//pkg:f.bzl", "sym", alias = "other")` binds each symbol; the manifest's `file-symbol` strategy with
  `labels: "bazel"` reads `//pkg:f.bzl` (from the workspace root) and `:f.bzl` (the importer's package); `@repo//...` is
  external and resolves to nothing.
- Oracle: `calibration/oracles/bazel-query` (tier A), subjects in `calibration/corpus.d/bazel.yaml`. Measured
  2026-10-19, `docs/bench/readings/lang/bazel.json`. Limits: O1347 (the oracle's scope and network), O1348
  (macros that create no target read as targets; no edge is stored at ingest for a basename-keyed file).
