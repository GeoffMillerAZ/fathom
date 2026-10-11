# bash — provenance

- Grammar: `tree-sitter/tree-sitter-bash v0.25.1` (MIT), commit
  `a06c2e4415e9bc0346c6b86d401879ffb44058f7`, ABI 15, scanner; built into
  `internal/extractor/tsengine/grammars/bash.wasm.gz` (1,364,058 bytes raw,
  262,984 bytes gzip). The repository's only *tag* is the years-stale
  `v0.6.0` (ABI 6, below tree-sitter core v0.26.11's minimum compatible
  language version — `src/parser.c` shipped there predates the grammar's
  last several years of changes, confirmed by diffing its `grammar.js`
  against `master`'s, 1,362 lines different); `v0.25.1` is a real, current
  tag (`git ls-remote --tags` sorts it before `v0.6.0` lexically, which is
  why the first pass missed it) pointing at the same commit as `master`
  HEAD (pushed 2025-12-02, "Regenerate parser for 0.25.1"), with a
  pre-built ABI-15 `src/parser.c` — no `"generate": true` CLI step needed.
  Pin and digests in `internal/extractor/tsengine/grammars/grammars.lock.json`
  (`internal/extractor/tsengine/grammars/PROVENANCE.md` regenerates its own
  tables from it).
- `tags.scm`: written in this repository (LB-Q-bash, docs/design/language-breadth.md
  §13), a port of the incumbent CGO `bashextract.go` walker. tree-sitter-bash
  ships no `queries/tags.scm` of its own (only `highlights.scm`); started
  from Aider-AI/aider's `aider/queries/tree-sitter-language-pack/bash-tags.scm`
  @ `ff191affc6e1413f33dad914d9930a5993aad853` (Apache-2.0) for the function
  and call node shapes, rewritten onto this engine's own bare `@name`/`@body`
  capture convention (aider's own `@name.definition.<kind>` naming is a
  different tags-query dialect; `internal/extractor/tsextract.go`'s
  `splitTSMatches`/`tsCalleeName` look for a literal `@name`/`@body`, the
  same convention every other dialect here already uses — the query
  compiled and ran fine either way, so this cost a debugging pass, not a
  build failure). A `@body` capture was added per LB-F3 so `Signature`
  truncates at the function's opening brace instead of swallowing its
  block. Replaced aider's bare `(variable_assignment name: ...)` — which
  fires on every assignment anywhere, including a function's own locals
  (`local x=$1`, a loop counter) — with a pattern anchored on the literal
  `export` token inside a `declaration_command`: only `export NAME=value`
  becomes a `BashVariable` definition, matching this queue item's own note
  ("functions, exported vars"), not every assignment. `declare -x`,
  `readonly`, `typeset` and a bare top-level `NAME=value` parse to the
  identical `declaration_command`/`variable_assignment` shape with no
  distinguishing named child (the `-x`/`-r` flag is itself an anonymous
  `word` sibling, indistinguishable from any other flag without reading its
  text) — left uncaptured, a documented gap, not a missed requirement.
  `export NAME` with no `=value` (marking an already-set variable for
  export, no `variable_assignment` under it) is likewise uncaptured: it has
  no natural line/signature home as a "definition".
- Kept from the incumbent (§11 step 3, port parity): the `BashFunction`
  source kind, unchanged string, both syntaxes (`function name() { }` and
  the bare POSIX `name() { }`) — `TestBashExtractor_KeywordAndPOSIXForms`'s
  own two forms both extract. Added beyond the incumbent (a port may add,
  never drop): `BashVariable` for `export NAME=value` (the incumbent never
  looked at variables at all) and same-file call collection (`Calls`;
  the incumbent never populated it either). Also added: shebang
  classification (`match.shebangs: ["bash", "sh"]`) — the incumbent
  dispatches by `.sh`/`.bash` extension only; the queue entry's own name
  ("Shell / Bash (.sh .bash, shebangs)") asks for it. `BashVariable` is
  declared in `kinds` but deliberately **not** in `required_kinds`
  (precedent: `dialects/python/dialect.json`'s "type" kind) — universal-ctags'
  `Sh` kind list (`ctags --list-kinds-full=Sh`: `alias`, `function`,
  `heredoc`, `script`) has no variable kind, so it cannot corroborate
  `variable` via oracle B; declaring it required would refuse every
  measured reading (§1.4, `internal/langbench/reading.go` `Validate`, the
  exact mechanism `internal/langbench/reading_test.go` exercises).
- `testdata/`: fixtures written in this repository (`functions.sh` both
  function syntaxes, a top-level `export`, an `export` inside a function
  body, a `local` and a bare top-level assignment that must NOT count, and
  plain calls including a call to another function defined in the same
  file; `multiline.sh` the brace-on-its-own-line style, proving `Signature`
  collapses across the line break and `EndLine` still reads the closing
  brace; `broken.sh` a missing `fi` — tree-sitter-bash's own error recovery
  misparses the `broken` function itself as a bare command, which this
  dialect correctly does not claim as a definition, while `good` (before
  it) and `survivor` (after it) still extract). Goldens hand-reasoned from
  `indexbench lang-probe`'s captures and tree output before generating them
  with `-update`, then read back and checked line by line against that
  reasoning before the test was allowed to go green.
- **Measurement (docs/bench/readings/lang/bash.json, calibration/corpus.d/bash.yaml):**
  universal-ctags 6.2.1 (tier B) only — the queue entry names no SCIP
  producer, and none is known to exist for Bash. `level_met` and the L2 gap
  are recorded in the reading's own `notes`/`disposition`;
  `docs/plans/language-breadth.md`'s `LB-Q-bash-rest` records the same
  disposition (deferred, not refused: an oracle gap, not a query one).
- **Merge gate (§11 step 2):** the target (L2) is not met (see the L2
  disposition above), so the CGO incumbent (`bashextract.go`) stays
  fusion's live `.sh`/`.bash` dispatch (`internal/fusion/pipeline.go`) —
  this dialect is written, tested and measured but not yet switched in.
- `imports.scm` (wave lf6, LB-Q-bash L3): written in this repository. `source FILE` and `. FILE`
  (also `\.`, `builtin source`, `command .`) with a literal path, resolved by the manifest's
  file-symbol strategy (`./` and `../` against the script's directory, any other path against the
  script's directory then the repository root, `source_roots: [""]`); dynamic and glob paths are
  left out. Two patterns read around a tree-sitter-bash v0.25.1 defect: a line opening with a
  backslash is parsed as part of the statement before it (no ERROR node). The manifest's
  `module_symbol` (K18) makes a script's top level the `<module>` its source statements and
  top-level calls are made from.
