# gradle — provenance

- Grammar: `murtaza64/tree-sitter-groovy` at commit `2a6ddd558b6aa39c5b77d8db9fe9baf817486b2c`
  (`main`, 2026-09-27; MIT, top-level `LICENSE`, read at the pin with `indexbench lang-pin -tree`;
  `package.json`, `Cargo.toml` and `tree-sitter.json` declare MIT too and nothing under
  `queries/` or `src/` disagrees), built into `internal/extractor/tsengine/grammars/groovy.wasm.gz`
  (171,362 bytes gzip, 761,792 raw, ABI 15, no scanner; pins and digests in
  `internal/extractor/tsengine/grammars/PROVENANCE.md`/`grammars.lock.json`). The repository has
  no release past its `initial` tag, so the lock's `tag` field carries the commit, as for
  `gotemplate` and `kotlin`. `main` ships its generated `src/parser.c`, so `"generate": false`.
- Why this grammar: three others were measured on the same files with the tree-sitter 0.27 CLI
  (a file counts when its tree holds an ERROR or MISSING node). On the Gradle scripts of the
  subjects (spring-projects/spring-boot v4.1.1, 725 files; apache/kafka 4.1.0, 4) this pin reads
  22 of 729 with an error (3.0%); `dekobon/tree-sitter-groovy` v0.3.0 (`654e4c2d`, Apache-2.0 or
  MIT) 37 (5.1%); `Decodetalkers/tree-sitter-groovy` (`679459be`, MIT, generated with the CLI)
  85 (11.7%); `amaanq/tree-sitter-groovy` v0.1.2 (`70efb0b9`, MIT declared) 84 (11.5%). The catalogue's own pin of
  Decodetalkers (`a6cf8f80`) carries no licence at that commit and `lang-pin -tree` refuses it.
  No candidate parses Groovy sources under the 5% line, which is why LB-Q-groovy is refused
  (LB-Q-groovy-rest); the blob ships for the Gradle scripts it does parse.
- `tags.scm` and `imports.scm`: written from scratch in this repository (design §13 step 11),
  never generated or copied. The grammar's `queries/` holds `highlights.scm`, `locals.scm` (a
  parameter rule only), `folds.scm`, `indents.scm` and `injections.scm`, no `tags.scm`, read for
  node vocabulary only; aider and nvim-treesitter carry no Gradle query, so `lang-draft` had
  nothing to draft from. Node and field shapes came from `indexbench lang-probe` and the
  tree-sitter CLI over `testdata/*.gradle` and the subjects' scripts, never from
  `grammar.js`/`node-types.json`. Each file's header states what is and is not a definition, a
  reference or an import, and the grammar's readings the patterns follow (a command call whose
  argument is a call, `api project(':x')`, is two adjacent siblings; `task x(type: T) { }` is the
  identifier `task` followed by a call; after a syntax error elsewhere, `dependencies { }` and
  `plugins { }` can read as an identifier followed by a closure, 7 of 1,329 such blocks on the
  subjects; `tasks.register('x').configure { }` after another statement of a block reads as a
  command call followed by its closure).
- `testdata/`: fixtures written in this repository (none copied from a subject); goldens written
  by hand from reading each fixture before any query existed (red first). Four of the first five
  matched the first queries entry for entry; `broken.gradle` did not: after its syntax error the
  grammar reads the `plugins { }` block as the identifier and the closure, which the patterns then
  matched. Three fixtures were added later, each red first, for shapes the subjects showed:
  `chained.gradle` (`tasks.register('x').configure { }`, two kafka tasks), `split.gradle` (the
  statement after a comment and `project(path: ':x')`, 103 spring-boot dependencies) and
  `errorroot.gradle` (a file the grammar wraps whole in an ERROR node, reduced from the shape of
  spring-boot's dependency-management script and rewritten with neutral names).
- Kinds: `GradleInclude` (module: a settings include's project path), `GradleDependency`
  (package: a dependencies-block notation or project path), `GradlePlugin` (resource: a plugins
  block id or an `apply plugin:` id), `GradleTask` (task). Include, dependency and task are
  callable: a task's `dependsOn`/`finalizedBy` names are its `Calls`, and an include or a project
  dependency calls the path it names, which the file resolver binds through `imports.scm`
  (namespace-dir over `:` from the repository root, suffixes `/build.gradle` and
  `/build.gradle.kts`) when the named project's build script declares exactly one top-level
  symbol (the rule every whole-module import resolves under; LB-Q-gradle-rest).
- Files: every `*.gradle` script (a build script, a settings script, a script plugin), which no
  dialect read before (they fell through to "no extractor registered"), so an ExtractorEpoch bump
  is needed before the dispatch reaches already-fingerprinted files. A Kotlin DSL script
  (`*.gradle.kts`) stays with the kotlin dialect.
- Measurement (`docs/bench/readings/lang/gradle.json`): against Gradle's own evaluation of each
  build (`calibration/oracles/gradle`, tier A), whose README records what the oracle does and does
  not claim. The oracle is bench-only: never a product dependency.
