# meson — provenance

LB-Q-meson, design `docs/design/language-breadth.md` §13. A new tags-engine tool format (no incumbent): `meson.build`,
`meson.options` and `meson_options.txt` were unclassified and skipped.

- Grammar: `tree-sitter-grammars/tree-sitter-meson` tag `v1.3.0`, commit `12edaf0dfef0a43bc74336a4432b2db1c65788d6` (MIT;
  the top-level `LICENSE`, Decodertalkers 2023), ABI 14, no external scanner, `src/parser.c` shipped (no generate step),
  built into `internal/extractor/tsengine/grammars/meson.wasm.gz` (233,708 bytes raw, 78,038 gzip). The master branch is
  24 commits ahead, all dependency bumps (checked 2026-10-01).
- `tags.scm`: written in this repository; the grammar ships no tags query and neither aider nor nvim-treesitter carries
  Meson. Node shapes came from `indexbench lang-probe`, never `grammar.js`. A definition is one function call, named
  as Meson names it: `project('name')`, a target call (`executable`, the library calls, `custom_target`, `jar`,
  `build_target`, `run_target`) by its first string argument (what `meson introspect --targets` lists, never the
  variable it is assigned to), `subproject('name')`, `option('name')` in the options files, and a dependency object
  (`v = dependency(...)`, `v = declare_dependency(...)`) by the variable that holds it, since `declare_dependency` names
  nothing. The variable a target call is assigned to (`libfoo = static_library('foo', ...)`) is a second symbol, kind
  `MesonVariable`, because a later `link_with : libfoo` spells the variable, not `foo`. A target whose first argument is
  not a lone string (`'t_' + name`) has no name to read. `cmake.subproject('x')` is a method of the cmake module, not
  the function: `subproject` is matched under each parent a statement can have, never under an `expression_statement`.
- References: the identifiers in a call's `link_with`, `link_whole`, `dependencies` and `depends` arguments (a single
  identifier or a list of them), owned by the smallest enclosing definition: the target, or the `declare_dependency`.
- `imports.scm`: `subdir('dir')` is a whole-file import (and wildcard) of `dir/meson.build` through
  `imports: {strategy: relative-path, relative: true, index_files: ["meson.build"]}`.
- Kinds: `MesonProject`, `MesonTarget`, `MesonVariable`, `MesonDependency`, `MesonSubproject`, `MesonOption`.
- Grammar limits, measured and not worked around: the grammar does not read `a*b` or a `?:` ternary (an ERROR node
  follows and, in a long file, can swallow the statements after it: harfbuzz's root `meson.build` loses 19 `dependency`
  assignments and 2 `subproject`/`project` calls, 21 of the 163 definitions Meson's own parser reads there), and
  `testcase ... endtestcase` (Meson's own test-suite clause, readable only under `MESON_RUNNING_IN_PROJECT_TESTS`).
- `testdata/`: fixtures written in this repository (`targets`, `links`, `imports`, `options`, `broken`), each golden
  written by hand from reading the fixture before the queries were final; adding `MesonVariable` added one entry per
  assigned target to four goldens and changed no other line.
- **Measurement (docs/bench/readings/lang/meson.json, calibration/corpus.d/meson.yaml):** see the reading's
  `disposition`.
