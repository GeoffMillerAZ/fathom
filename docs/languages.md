# Languages and dialects — what fathom extracts, and how far

A dialect is an extractor: the knowledge of how to read a language or tool format
(`docs/design/pack-taxonomy.md`). This table is the tracker ADR-041 names: extraction
approach, whether a tree-sitter grammar is in the tree, where typed edges come from, and
what is left. Typed edges were built in this order: Go, CUE, TypeScript, Python, then
the rest. `TestLanguagesTrackerMatchesRegistry` keeps the rows honest against the
extractor registry.

**L4 outside Go and CUE needs the language's own SCIP indexer.** For TypeScript,
JavaScript, Python, Java, C#, C, C++, Rust and the other `scip` rows below, typed edges
come from `fathom ingest --scip`, which imports the output of an indexer you install and
run yourself (scip-typescript, scip-python, scip-java, and so on). A board reading of L4
for those languages was taken with that index imported. Without one, their call edges are
the per-file resolver's (`file-resolved`, tier pre-scan) or syntax-only, never typed; Go
and CUE resolve types natively and need nothing extra.

**The breadth program (ADR-048, vision 37).** Support levels are L1 symbols, L2 syntax call
edges, L3 imports and file-resolved edges, L4 typed edges, each claimed only when a committed
reading under `docs/bench/readings/lang/` proves it against an independent oracle (ctags or
SCIP; design `docs/design/language-breadth.md` §1). The "Excellence board" and "Support levels" sections at the end of
this file are **generated** (LB-F17, `go run ./cmd/indexbench lang-tracker > docs/languages.md`)
from the queue (`docs/plans/language-breadth.queue.json`; the excellence board's totals count
its languages and tool formats), the registered dialects and the readings; `TestLanguagesTrackerIsGenerated` fails when this
file differs from the rendering and when a row claims a level above its reading (I54). The
typed-edge table below is the hand-kept template part, held true against the registry by
`TestLanguagesTrackerMatchesRegistry`. The "Engine / dispatch" column is **generated**
from the dispatch registry (`fusion.DispatchTable`, LB-F356; one dispatch placeholder
per row, naming its extensions or basenames) and `TestTrackerDispatchColumnMatchesRegistry` fails when a cell
types an extractor by hand; the prose beside a placeholder carries only what the registry
cannot know (grammar pins, oracle notes, refusal reasons). A "Go walker" is a hand-written
extractor in `internal/extractor`.

Typed-edge levels: **native** (fathom resolves types itself), **scip** (imported from a
language-native SCIP indexer), **file** (per-file resolver, tier pre-scan, labelled
`file-resolved`), **syntax** (call names by syntax only).

| Language / dialect | Engine / dispatch | Grammar in tree | Calibration subjects | Typed edges | Next |
|---|---|---|---|---|---|
| Go | native Go (`go/parser` + `go/types`) | n/a | cobra, cli/cli, prometheus | **native** (`go/types`; selector-method and interface dispatch resolved). The board below reads Go at L3: L4's def_mapping is not met | L4: struct fields and interface methods, the one class left (O675); a member pass read L4 excellent and failed its cost gate, so it is reverted until a cheaper pass exists (O673, LB-Q-go-rest) |
| CUE | native CUE (`cue/load`) | n/a | this repo's `.fathom/`, `schema/` | **native** (SU-10) | — |
| TypeScript | tags dialect `javascript` | yes | nest, zod, typeorm | **scip** — `fathom ingest --scip` imports scip-typescript's own output (SU-9, `internal/scip`; the L0 oracle SU-12 is also done); against the real, only-published scip-typescript@0.4.0 it emits zero `DisplayName`s, which broke the import until O115's fix (2026-09-24): `ImportIndex` now maps each definition to fathom's own symbol table by position instead, measured on zod at 82.0% of raw SCIP definitions unmapped (no fathom-level counterpart — SCIP's finer granularity, expected) and the rest correctly keyed; **the file-resolved tier shipped (SU-11, `internal/fusion/fileresolve.go`) remains the fallback** when no scip index is imported. That no-SCIP fallback read 0.87 precision and 0.03 recall against scip truth on 2026-09-24 (`docs/bench/readings/l0-ts-py-2026-09-24.md`); the board's L4 reading is taken with the SCIP index imported | CV-M7 done; a `PutTypedEdges` replace-granularity gap the re-run surfaced (a package with zero NEW edges keeps its stale old rows) is unfixed, its own thread; lookups corpus (SU-23) |
| JavaScript (`.js .jsx .mjs .cjs`) | tags dialect `javascript` | yes | express, axios, lodash | **scip** — same `--scip` import mechanism as TypeScript (SU-9, SU-12 done), same O115 fix (2026-09-24: position-based mapping, fixed); **the file-resolved tier shipped (SU-11) remains the fallback** when no scip index is imported | — |
| Python | tags dialect `python` | yes | fastapi, flask, django | **scip** — `fathom ingest --scip` imports scip-python's own output (SU-9, SU-12 done); against the real, only-published scip-python@0.6.6 it emits zero `DisplayName`s, which broke the import until O115's fix (2026-09-24: position-based mapping), measured on flask at 65.5% of raw SCIP definitions unmapped (expected — SCIP's finer granularity) and the rest correctly keyed; **the file-resolved tier shipped (SU-11) remains the fallback** when no scip index is imported, now with a real precision/recall read against the fixed scip truth: 0.95 precision / 0.12 recall (CV-M7, `docs/bench/readings/l0-ts-py-2026-09-24.md`) | L4 excellent since K18's per-file module symbol (wave lf2, LB-F346..LB-F349); lookups corpus (SU-23) |
| Java | tags dialect `java` | yes (tree-sitter-java v0.23.5) | guava, retrofit | **scip** via scip-java (SU-9) | the file resolver's import and nested-type shadowing (O513); L4 typed_recall |
| Kotlin | tags dialect `kotlin`; grammar fwcd/tree-sitter-kotlin commit 1852ea17; the 0.3.8 tag left 16.6% of the subject files in error | yes | okhttp, Exposed | **scip** via scip-java (SU-9), once its Kotlin plugin writes occurrence ranges under K2 (LB-Q-kotlin-rest) | a SCIP index with ranges, to measure `import_resolution` |
| Scala | tags dialect `scala` | yes (tree-sitter-scala v0.26.2) | cats, http4s | **scip** via scip-java (SemanticDB, which needs the subject's sbt build) | L1 range agreement (the engine's end line, O761), then L3 and L4 (LB-Q-scala-rest) |
| Rust | tags dialect `rust` | yes (tree-sitter-rust v0.24.2) | ripgrep, clap | **scip** via rust-analyzer | O540's resolver edits (a macro's textual scope, a fully qualified path no `use` binds); typed receivers need type inference |
| C | `.c` tags dialect `c`; `.h` tags dialect `c` or `cpp` (by content sniff) | yes (tree-sitter-c v0.24.2) | curl, libuv, sqlite | **scip** via scip-clang | — |
| C++ | tags dialect `cpp` | yes (tree-sitter-cpp v0.23.4) | fmt, nlohmann/json | **scip** via scip-clang | L2 call precision, L3 file edges (`import_resolution` reads 1 against the build's own preprocessor, tier A) and L4 (LB-Q-cpp-rest) |
| Ruby | tags dialect `ruby` | yes | sinatra, jekyll | **scip** via scip-ruby, which needs a Sorbet-typed subject (on untyped code it sees only typed receivers); meanwhile **file**: `require`, `require_relative`, `load` and `autoload` resolve via the file-symbol strategy, and a constant is looked up lexically through its enclosing modules (LB-Q-ruby at L3 good, LB-F76) | L3 excellent and L4 need a Sorbet-typed subject for scip-ruby (tier B and C oracles grade good at most; LB-Q-ruby-rest) |
| HTML | tags dialect `html` | yes | html5-boilerplate, bootstrap (visual tests) | **file** — `src`/`href` resolve via the relative-path strategy (LB-F11); no SCIP producer exists for HTML, so this is the ceiling until one does | L1 range agreement (ctags writes no HTML end line); L2 is undefined for markup under §1.2 and L3 needs a tier-A link checker (LB-Q-html-rest, O799) |
| CSS | tags dialect `css` | yes | normalize.css, pico, gitea (web_src/css) | **file** — `@import` resolves via the relative-path strategy (LB-F11) and is an edge between the sheets' module symbols; a `var()` or animation name binds a custom property or `@keyframes` an imported sheet declares (LB-F459); no SCIP producer exists for CSS, so this is the ceiling until one does | L1 range agreement (neither lightningcss nor ctags writes a CSS end line; LB-Q-css-rest); L2 graded as name references within the `@import` closure (W2) |
| Vue single-file components | tags dialect `vue`; grammar tree-sitter-grammars/tree-sitter-vue ce8011a4 (0.09% of files in error); `<script>` read as TypeScript or JavaScript and `<style>` as CSS or SCSS, the file their host (F2) | yes | element-plus, vitepress | **file** — a `<script>`'s relative imports resolve via the relative-path strategy, and a component element binds the imported component file's module symbol; no SCIP producer reads `.vue` (vue-tsc writes none), so this is the ceiling until one does | L1 to L3 excellent against @vue/compiler-sfc (vue.json); a script's `let` is the script dialect's (O978), an import of a `.ts` module draws no file edge (the file resolver's universe is the host's own files, O979), and a host's script parsers churn the module pool on a mixed repository (O982) |
| Svelte components | tags dialect `svelte`; grammar tree-sitter-grammars/tree-sitter-svelte v1.0.2 (1.26% of files in error) | yes | sveltejs/kit, shadcn-svelte | **file** — as Vue: a `<script>`'s relative imports, a component element binding the imported component's module symbol; no SCIP producer reads `.svelte` | L1: the JavaScript dialect names a `let` a constant, outside the kind intersection (LB-Q-svelte-rest, O978); L2 and L3 excellent against svelte/compiler (svelte.json); `.ts` targets as Vue's |
| SCSS | tags dialect `scss` | yes | bootstrap, primer/css | **file** — `@use`, `@forward` and `@import` resolve via the relative-path strategy with Sass's partial rule (LB-F465), each an edge between module symbols; an `@import`ed partial's mixins bind an `@include`; no SCIP producer exists for Sass | L1 range agreement (Dart Sass's parser gives no end line); L2 is undefined for Sass until the owner extends W2 (LB-Q-scss-rest); the indented `.sass` syntax is not parsed |
| Astro (`.astro`) | tags dialect `astro` | yes (virchau13/tree-sitter-astro 213f6e69) | withastro/docs, withastro/astro (examples) | **file** — a component is its file's module symbol and the component tags its markup writes are its calls; its frontmatter and JavaScript scripts are TypeScript it hosts (F2), whose relative imports resolve via the relative-path strategy, an import of another component an edge between the files' module symbols; no SCIP producer reads `.astro` | an import of a TypeScript or JavaScript module draws no edge (the file resolver's universe is one language's files); a tsconfig `paths` alias is not resolved for an `.astro` importer; the JavaScript of a markup expression (`{...}`) is not read; a destructured prop with a default is no definition (O985, O986, O989) |
| Angular component views (`.html` a component's decorator names) | tags dialect `html`; refused for an Angular dialect (LB-Q-angular-rest, design §14.2): the only maintained licensed Angular grammar, dlvandenberg/tree-sitter-angular v0.9.2, leaves 13 of the 72 calibration views (18.1%) in error, over the 5% line | no (measured, not pinned) | angular-realworld-example-app, angular/components (`src/material`) | **none** for a view (a selector's use and a pipe are not extracted); the component class is TypeScript's | a tree-sitter-angular release under 5% on both subjects, then the view dialect |
| Meson (`meson.build`, `meson.options`, `meson_options.txt`) | tags dialect `meson` | yes (tree-sitter-meson v1.3.0) | mesonbuild/meson (parse only), harfbuzz, libxml2 | **syntax** — `subdir('dir')` is a relative-path import that lang-oracle's resolver follows (a name resolves from a file to the directory it imports), but ingest's file-resolve pass is keyed by file extension and `meson.build` has none (O379); and Meson's variables are one namespace across every file, so a link to a sibling or an ancestor has no import to bind it (LB-F128, O375) | a basename-keyed file-resolve pass (O379) and a build-namespace strategy (L3); arithmetic and ternary expressions the grammar does not read (O378) |
| Bazel / Starlark (`BUILD`, `BUILD.bazel`, `*.bzl`, `WORKSPACE`, `WORKSPACE.bazel`, `WORKSPACE.bzlmod`, `MODULE.bazel`) | tags dialect `bazel` | yes (tree-sitter-starlark v1.3.0) | bazelbuild/examples (rules, configurations), grpc/grpc | **syntax** — a `load()` binds the macros and rules it names to the `.bzl` file defining them (the file-symbol strategy reads `//pkg:f.bzl` and `:f.bzl` labels), and lang-oracle's resolver draws that edge, but ingest's file-resolve pass is keyed by file extension and `BUILD`, `WORKSPACE` and `MODULE.bazel` have none (O379), so only a `.bzl` file's own loads are stored | a basename-keyed file-resolve pass (O379); a label strategy for `//pkg:target` deps between BUILD files (the oracle's `rule_input` edges, not scored); `grpc_package(name = ...)`-style macros that create no target read as targets |
| Tiltfile (`Tiltfile`) | tags dialect `tiltfile` | yes (tree-sitter-starlark v1.3.0, the grammar Bazel reads) | tilt-dev/tilt-extensions, tilt-dev/tilt-example-go | **syntax** — call names by syntax only; a Tiltfile's `docker_build`, `local_resource`, `k8s_resource`, `k8s_yaml` and `config.define_*` calls are entities named by their literal string, and `load()` and `include()` are not read as imports (no L3 oracle) | an import strategy for `load("./file", "sym")` and `include()`; names built from a variable or a concatenation are no entity |
| Clang compilation database (`compile_commands.json`) | doc dialect `compile-commands`; JSON plus each entry's flags (`decode: compile-commands`) | n/a | fmt, libuv (cmake-generated databases at the checkout root) | **syntax** — a translation unit's include directories, macros and output are its own references; no cross-file selector is declared | joined `-isystem`/`-iquote`/`-idirafter` flags (O376) |
| Conan (`conanfile.py`, `conanfile.txt`, `conan.lock`, `conandata.yml`) | doc dialect `conan` | yes (tree-sitter-python) | conan-center-index (54 recipes), conan-io/examples2 | **syntax** — a recipe's requires are references to the recipe names they use; no cross-file edge is declared (the doc-xref pass reads no Conan file and no recipe is an entity it can resolve to, O370) | O370, O371 |
| C# | tags dialect `csharp` | yes (tree-sitter-c-sharp v0.23.5) | Polly, Newtonsoft.Json | **scip** via scip-dotnet | L2 call precision (scip-dotnet writes no occurrence for a generic method called with type arguments, O271), L3, L4 |
| PHP | tags dialect `php` | yes (tree-sitter-php v0.24.2) | guzzle, monolog | **syntax** (no mature exporter) | L2 call precision against scip-php, L3/L4 (LB-Q-php-rest) |
| Swift | refused (no extractor); refused (LB-Q-swift-rest, design §14.2): a tree-sitter dialect (`dialects/swift`) is built and measured (L1 excellent, L2 good against the compiler's parser), but 5.6% of its calibration files parse with an error under the pinned grammar, over the 5% line; Swift stays disclosed as unsupported, named per service by `list_services` | yes (alex-pinkus/tree-sitter-swift commit 35245fb, not dispatched) | Alamofire, swift-nio | **none** | a tree-sitter-swift that parses Swift 6 syntax under the 5% line, then re-measure and re-plan (L3 needs a Swift import oracle for `import_resolution` and a typed oracle) |
| Lua | tags dialect `lua` | yes | Kong, telescope.nvim | **syntax** — call names by syntax only (a name, a field or a method callee); a literal `require "a.b"` or `pcall(require, "a.b")` is a file edge to `a/b.lua` or `a/b/init.lua` under the repository root, `lua/`, `lib/` or `src/` (file-symbol strategy, LB-Q-lua at L3 good) from the file's `<module>` symbol; no SCIP producer exists for Lua | a computed `require` name and a rock or C module have no file; a function in a computed slot has no name |
| Luau | tags dialect `luau` | yes (tree-sitter-luau 1.2.0) | luau-lang/luau, Fusion | **syntax** — call names by syntax only (a name, a field or a method callee); no SCIP producer exists for Luau | the `require` graph (an instance path such as `script.Parent.Foo` or an alias string is no strategy of the closed set); an alias of a generic function type (`export type X = <A>(...) -> ...`) is not parsed by the grammar; a function in a computed slot has no name |
| R | tags dialect `r` | yes (tree-sitter-r v1.3.0) | dplyr, testthat | **syntax** — call names by syntax only (a plain or namespaced callee); no SCIP producer is wired for R | the `library()`/`source()` graph (L3); S4 and R6 classes (`setClass`, `R6Class`) and `setMethod` definitions; a function held in a list or environment slot (`obj$method <- function`); a `->` assignment |
| Perl | `.pm` tags dialect `perl`; `.pl` `.t` tags dialect `perl` (by content sniff) | yes (tree-sitter-perl 2.0.0) | Mojolicious, Perl::Critic | **syntax** — call names by syntax only (a function with parentheses or a term after it, a method after an arrow; a function named like a Perl builtin is no call); no SCIP producer exists for Perl | L1 range agreement (ctags writes no end line for Perl, so L1 stays good); the `use`/`require` graph (L3); a sub is named as written, so one in a `package NAME;` statement is not prefixed by it; anonymous subs, typeglob assignments and `use constant` are not definitions |
| Jupyter notebooks | Go walker `NotebookExtractor` | yes (the python dialect's grammar, per code cell) | jakevdp/PythonDataScienceHandbook, microsoft/ML-For-Beginners | **syntax** — call names by syntax only, as Python's; no import or file edge | format 3 notebooks (`worksheets`), a notebook whose kernel is not Python, markdown and raw cells and every output; cells are read in document order and parsed one at a time, so a cell that does not parse costs only its own symbols |
| Bash | tags dialect `bash` | yes (tree-sitter-bash v0.25.1) | bats-core, nvm | **syntax** — call names by syntax only (the command a function runs); no SCIP producer exists for Bash | a zsh script: no maintained grammar leaves fewer than 5% of ohmyzsh's zsh files with an ERROR node (the best 15.9%, tree-sitter-bash 66.4%), so zsh and ksh are not read (O792, LB-F454); `source` and `.` with a dynamic path (`$VAR`, `$(dirname "$0")/x`) draw no file edge, and a top-level call to an external command (`echo`, `grep`) is not stored (O971); a script with no extension is read by its shebang (D4, O756) and is its dialect's file in the file-resolve pass (O793) |
| PowerShell | tags dialect `powershell` | yes | PSScriptAnalyzer, Pester | **syntax** — call names by syntax only (a command name or a member invocation); no SCIP producer exists for PowerShell | class properties and script/global variables (a `$`-sigilled token the tags engine cannot strip to the bare name); dot-sourcing, Import-Module and a manifest's modules draw no file edges until the owner rules on their cost (O970) |
| Elixir | tags dialect `elixir` | yes (tree-sitter-elixir v0.3.5) | phoenix, ecto | **syntax** — call names by syntax only (a name, or `Alias.fun` with the receiver written out); no SCIP producer exists for Elixir | `alias`, `import`, `require` and `use` resolve to the file whose `defmodule` defines the module (namespace-dir over the enclosing module as the package), but a statement draws no edge of its own (file_edge_recall .401, `module_symbol` cannot combine with a captured package); `alias Foo.Bar` binds the full name, not `Bar`; `use` is read as an import of the module, not of the names its `__using__` injects; a call in a definition's own head or in a type expression is also listed |
| Dart | tags dialect `dart` | yes | bloc, http (eight pure-Dart packages) | **scip** via scip-dart, which indexes one package at a time (LB-Q-dart: def_mapping .9995, typed_recall .8464 once a reference to a parameter or a local variable, which maps to no symbol, leaves its denominator (LB-F55), .0036 under the .85 bar); meanwhile **file**: `import` resolves through `pubspec.yaml`'s package name and relative paths (namespace-dir) | file_edge_recall .50 for L3 excellent (barrel exports and type references are followed since LB-F74/LB-F75; L3 good); attributing an edge from a parameter or a type parameter to its enclosing definition (L4) |
| Ada | tags dialect `ada` | yes | gnatcoll-core, alire | **syntax** — call names by syntax only (a procedure call statement or a function call, named by the callee's last identifier); no SCIP producer exists for Ada | L1 is measured at good (universal-ctags writes no Ada end line and loses sync after a `declare` block, so `range_agreement` is not measured); the `with` graph (L3, a strategy over child-unit paths) |
| VHDL | tags dialect `vhdl` | yes | neorv32, VUnit | **syntax** — call names by syntax only; the grammar cannot tell a function call from an index, so both are recorded; no SCIP producer exists for VHDL | ports, signals and generics are not emitted; the `library`/`use` graph (L3) |
| Makefile | tags dialect `makefile` | yes | prometheus, cosmopolitan, valkey | **syntax** — a rule's literal prerequisite words by syntax only; no SCIP producer exists for make, and a prerequisite spelled with a variable is a value the dialect does not evaluate | `include` edges (L3), tab-indented assignments inside conditionals and unspaced `VAR?=x` (tree-sitter-make limits) |
| Gradle builds (Groovy DSL: `*.gradle`) | tags dialect `gradle`; grammar murtaza64/tree-sitter-groovy commit 2a6ddd55 | yes | spring-boot, kafka | **syntax** — a task's `dependsOn`/`finalizedBy` names by syntax only; an include or a project dependency resolves to the named project's build script only when that script declares a single top-level symbol (LB-Q-gradle-rest); no SCIP producer exists for Gradle | project edges (L3); the Kotlin DSL (`*.gradle.kts` stays Kotlin's) |
| Jenkins pipelines (`Jenkinsfile`, `*.jenkinsfile`, `vars/*.groovy`) | tags dialect `jenkinsfile`; grammar murtaza64/tree-sitter-groovy commit 2a6ddd55, the Gradle dialect's blob; vars/*.groovy by glob | yes | pipeline-examples, pipeline-library, jenkins | **syntax** — a step's name by syntax only, attached to its section or stage; a literal shell step reaches the Makefile target, task, npm script or script it runs through the GitHub Actions run-step rule, and a step call its in-repository `vars/` variable (file-tier doc-xref edges, LB-F374, LB-F375); no SCIP producer exists for Groovy | scripted pipelines and `vars/` at L2 (tolerated: 47.1% of the claimed files in error, LB-F373-rest); JCasC jobs for `build job:` (LB-F376-rest) |
| Emacs Lisp | tags dialect `emacs-lisp`; the grammar is generated at build time, its repository commits no `parser.c` | yes | magit, lsp-mode | **syntax** — no call is captured yet (a head symbol is a call, a `let` binding pair, a `dolist` spec and a `cond` clause in the same syntax); no SCIP producer exists for Emacs Lisp | call edges (L2), which must tell a call from a binding form; `transient-define-*`, `defvar-keymap`, `define-error` and `ert-deftest` (ctags has no kind for them) |
| Common Lisp | tags dialect `commonlisp` | yes | lem, clack | **syntax** — no call is captured yet (a head symbol is a call, a `let` binding pair, a `cond` clause and a `loop` clause in the same syntax); no SCIP producer exists for Common Lisp | call edges (L2), which must tell a call from a binding form; `define-condition`, `define-command` and a `(:method ...)` option of a `defgeneric`; the grammar reads a `loop :with` list of four or more variables and some `format` control strings (`~/`, `~{~A~^ ~}`, `~:*`) as errors, which leaves 4 of the 232 calibration files (1.7%) with a parse error |
| SQL | tags dialect `sql` | yes (DerekStride/tree-sitter-sql v0.3.11) | pagila, sqitch | **syntax** — a foreign key is a call by syntax only (the referenced table) | a data dump whose tree passes the module cap faults (O796); MySQL, T-SQL and PL/SQL are outside the grammar's families (D2) |
| Terraform/HCL | Go walker `HCLExtractor` | n/a | (corpus.yaml) | **syntax** (resource references) | — |
| Kubernetes YAML, Helm, Kustomize, docker-compose | `.yaml` Go walker `YAMLExtractor`; `Chart.yaml` Go walker `HelmChartExtractor`; `values.yaml` Go walker `HelmValuesExtractor`; `kustomization.yaml` doc dialect `kustomize`; `docker-compose.yml` doc dialect `docker-compose` | n/a | (corpus.yaml) | **syntax** (`service_ref` cross-repo edges) | — |
| Dockerfile, CMake, GitHub Actions, proto, GraphQL, OpenAPI, JSON, TOML, templ, Markdown, go.mod, go.work, package.json, pyproject.toml, pom.xml, Cargo.toml, vcpkg.json, vcpkg-configuration.json, .csproj, .fsproj, .vbproj, .props, .targets, Taskfile.yml, lefthook.yml, .pre-commit-config.yaml, CloudFormation and SAM templates (by content) | `Dockerfile` tags dialect `dockerfile`; `CMakeLists.txt` Go walker `CMakeExtractor`; `action.yml` doc dialect `github-actions`; `.proto` Go walker `ProtoExtractor`; `.graphql` tags dialect `graphql`; `.json` Go walker `JSONExtractor`; `.toml` Go walker `TOMLExtractor`; `.templ` Go walker `TemplExtractor`; `.md` Go walker `MarkdownExtractor`; `go.mod` `go.work` Go walker `GoModulesExtractor`; `package.json` doc dialect `package-json`; `pyproject.toml` doc dialect `python-packaging`; `pom.xml` doc dialect `maven`; `Cargo.toml` doc dialect `cargo`; `vcpkg.json` `vcpkg-configuration.json` doc dialect `vcpkg`; `.csproj` `.fsproj` `.vbproj` `.props` `.targets` doc dialect `msbuild`; `Taskfile.yml` doc dialect `taskfile`; `lefthook.yml` `.pre-commit-config.yaml` doc dialect `git-hooks-config` | n/a | (corpus.yaml where listed) | **syntax** | — |
| nginx, HAProxy and Caddyfile configuration (nginx.conf, haproxy.cfg, Caddyfile, .caddyfile; a .conf and a .cfg by content, sites-enabled/* and Caddyfile.* by glob) | `nginx.conf` Go walker `NginxExtractor`; `haproxy.cfg` Go walker `HAProxyExtractor`; `Caddyfile` `.caddyfile` Go walker `CaddyfileExtractor`; a .conf and a .cfg are read by content sniff, sites-enabled/* and Caddyfile.* by glob | n/a | (calibration/corpus.d) | **syntax** — includes and imports are file-tier doc-xref edges; a proxy's upstream is a `route_ref` route target, resolved to a deploy key by LB-F96 | include and import globs (fusion.ResolveDocXrefs matches a glob against directories); a `.conf` whose first KiB is only comments |
| Rego / OPA policy (.rego) | Go walker `RegoExtractor`; no grammar reads Rego under the 5% line (ADR-048 wave-pc amendment) | n/a | regal, opa | **file** — `import data.x.y` resolves by the rego-package strategy to the files declaring the package, and a call or reference through the import is a file-resolved edge to the file defining the rule (LB-Q-rego at L3 good, LB-F132) | L3 excellent: an import names every file of a package split across files, an edge only the one defining what is used (LB-Q-rego-rest); no Rego SCIP producer (L4) |
| Terragrunt (terragrunt.hcl, root.hcl, a .hcl by content) and HashiCorp HCL tool configs (Packer `*.pkr.hcl` and `*.pkrvars.hcl`, Nomad `*.nomad`, `*.nomad.hcl` and a .hcl whose top-level block is `job`, `docker-bake*.hcl`) | `terragrunt.hcl` `root.hcl` Go walker `TerragruntExtractor`; `docker-bake.hcl` `.nomad` Go walker `HCLToolsExtractor` | n/a | (calibration/corpus.d) | **syntax** — Terragrunt includes, dependencies, local `terraform.source` and inputs, and a Nomad task's `config.image`, are file-tier doc-xref edges | Vault and Consul configuration (unclaimed); a `locals`-only or comment-heavy .hcl whose first KiB no sniff matches; `read_terragrunt_config` reads |
| Azure Bicep (.bicep, .bicepparam) | tags dialect `bicep`; grammar tree-sitter-grammars/tree-sitter-bicep v1.1.0 | yes | Azure/bicep samples, azure-quickstart-templates | **file** — a module's path and a compile-time import resolve to the file (relative-path); a module's params keys bind the target file's parameters | a module with no params draws no file edge; a `.bicepparam`'s `using` (the grammar does not parse `param x = v`) |
| Serverless Framework, Pulumi YAML, AWS CDK and ARM templates (serverless.yml, Pulumi.yaml, cdk.json; Pulumi.<stack>.yaml by glob, ARM templates and parameters files by content) | `serverless.yml` doc dialect `serverless`; `Pulumi.yaml` doc dialect `pulumi-yaml`; `cdk.json` doc dialect `aws-cdk` | n/a | (calibration/corpus.d) | **syntax** — an ARM linked template (relativePath, a template-relative uri, through a variable) is a file-tier doc-xref edge | a handler's, a Pulumi `main`'s and a CDK app's program file (the doc-xref pass joins doc-engine files only) |

Disclosed as unsupported (named per service in `list_services`, never silently skipped; `internal/connector/unsupported_language.go`): Clojure, Crystal, Elm, Erlang, F#, Groovy (refused, LB-Q-groovy-rest), Haskell, Julia, Nim, OCaml, Swift (refused, LB-Q-swift-rest), Zig.

Bench coverage today: every scenario corpus has a Go subject; the Long Code Arena corpus
covers Python, Java and Kotlin bug localization. SU-23 (done) adds TypeScript
(`testdata/scenariobench/lookups-ts.json`, `nest`) and Python
(`testdata/scenariobench/lookups-py.json`, `fastapi`) lookups corpora, committed and
**not run** (ADR-041 decision 9) — both corpora's own `notes.typed_edges` records that
neither SU-9 nor SU-11 is wired yet, so their `full`-arm predictions are written against
today's heuristic (syntax) call edges only, never a typed tier. SU-12 adds the
free-tier typed-edge oracle per language.

## Excellence board (generated)

Generated from the queue's `tier` field (T1..T6; an entry without one is outside the
excellence target set), the committed readings and the thresholds of design §1.2; never
hand-edit it. Level counts are the level met exactly; `none` is measured with no level met;
`refused`, `tolerated` and `unsupported` claim no level. At target is a level met at or above the
item's target; the last column also needs every level up to the target graded excellent.

| Tier | Kind | Items | L1 | L2 | L3 | L4 | None | Refused | Tolerated | Unsupported | At target | At target, all excellent |
|---|---|---|---|---|---|---|---|---|---|---|---|---|
| T1 | language | 33 | 1 | 3 | 11 | 10 | 1 | 2 | 0 | 5 | 21 | 8 |
| T1 | tool format | 1 | 0 | 1 | 0 | 0 | 0 | 0 | 0 | 0 | 1 | 0 |
| T2 | tool format | 23 | 0 | 8 | 12 | 0 | 1 | 0 | 0 | 2 | 17 | 12 |
| T3 | tool format | 16 | 1 | 4 | 11 | 0 | 0 | 0 | 0 | 0 | 15 | 5 |
| T4 | language | 3 | 0 | 0 | 1 | 0 | 0 | 1 | 0 | 1 | 1 | 0 |
| T4 | tool format | 7 | 0 | 3 | 2 | 0 | 0 | 0 | 0 | 2 | 5 | 4 |
| T5 | tool format | 8 | 0 | 1 | 5 | 0 | 0 | 0 | 0 | 2 | 5 | 2 |
| T6 | language | 1 | 0 | 1 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 |
| T6 | tool format | 4 | 0 | 1 | 1 | 0 | 0 | 0 | 0 | 2 | 2 | 0 |
| T7 | tool format | 4 | 0 | 0 | 4 | 0 | 0 | 0 | 0 | 0 | 4 | 2 |
| **Total** | language | 37 | 1 | 4 | 12 | 10 | 1 | 3 | 0 | 6 | 22 | 8 |
| **Total** | tool format | 63 | 1 | 18 | 35 | 0 | 1 | 0 | 0 | 8 | 49 | 25 |

### T1

| Item | Kind | Target | Level met | Next gap |
|---|---|---|---|---|
| TypeScript (.ts .tsx .mts .cts) | language | L4 | L4 (L1 good, L2 excellent, L3 excellent, L4 excellent) | L1 good: sym_recall 0.9358 < 0.95 |
| JavaScript (.js .jsx .mjs .cjs) | language | L4 | L4 (L1 excellent, L2 excellent, L3 excellent, L4 good) | L4 good: typed_recall 0.9376 < 0.95 |
| React components, props, hooks and React Router routes (.tsx .jsx .ts .js) | language | L3 | L3 (L1 good, L2 good, L3 good) | L1 good: oracle tier C caps at good |
| Angular components and templates (.component.ts, .component.html) | language | L3 | refused | re-plan (LB-Q-angular-rest refused) |
| Python (.py .pyi) | language | L4 | L4 (L1 excellent, L2 excellent, L3 excellent, L4 excellent) | excellent at target |
| HTML (.html .htm) | language | L3 | L3 (L1 excellent, L2 excellent, L3 excellent) | excellent at target |
| CSS (.css) | language | L3 | L3 (L1 good, L2 excellent, L3 excellent) | L1 good: range_agreement not measured |
| SQL (.sql; schema and migrations) | language | L2 | L2 (L1 excellent, L2 good) | L2 good: oracle tier C caps at good |
| Shell / Bash (.sh .bash, shebangs) | language | L3 | L3 (L1 excellent, L2 good, L3 good) | L2 good: oracle tier C caps at good |
| Java (.java) | language | L4 | L4 (L1 excellent, L2 excellent, L3 excellent, L4 excellent) | excellent at target |
| C# (.cs) | language | L4 | L4 (L1 excellent, L2 excellent, L3 excellent, L4 excellent) | excellent at target |
| C (.c, .h by sniff) | language | L4 | L4 (L1 excellent, L2 excellent, L3 excellent, L4 excellent) | excellent at target |
| C++ (.cc .cpp .cxx .hpp .hh .hxx, .h by sniff) | language | L4 | L4 (L1 excellent, L2 excellent, L3 good, L4 good) | L3 good: file_edge_precision 0.9023 < 0.95 |
| Go (.go) | language | L4 | L3 (L1 excellent, L2 excellent, L3 excellent) | L4 not met: def_mapping 0.5891 < 0.85 |
| PowerShell (.ps1 .psm1 .psd1) | language | L3 | L3 (L1 excellent, L2 good, L3 excellent) | L2 good: oracle tier B caps at good |
| PHP (.php) | language | L4 | L4 (L1 excellent, L2 good, L3 excellent, L4 excellent) | L2 good: oracle tier B caps at good |
| Rust (.rs) | language | L4 | L4 (L1 excellent, L2 excellent, L3 excellent, L4 excellent) | excellent at target |
| Kotlin (.kt .kts) | language | L4 | L3 (L1 excellent, L2 good, L3 good) | L4 not met: def_mapping not measured |
| Lua (.lua) | language | L3 | L3 (L1 excellent, L2 good, L3 good) | L2 good: oracle tier C caps at good |
| Ruby (.rb, Rakefile, Gemfile by basename) | language | L3 | L3 (L1 excellent, L2 good, L3 good) | L2 good: oracle tier C caps at good |
| Dart (.dart) | language | L4 | L4 (L1 excellent, L2 excellent, L3 excellent, L4 excellent) | excellent at target |
| Swift (.swift) | language | L3 | refused | re-plan (LB-Q-swift-rest refused) |
| SCSS (.scss) | language | L3 | L1 (L1 good) | L2 not met: call_recall not measured |
| Sass (indented syntax) (.sass) | language | L3 | unsupported | no dialect (LB-Q-sass) |
| Tailwind CSS v4 at-rules and custom utilities (.css) | language | L2 | unsupported | no dialect (LB-Q-tailwind) |
| Less (.less) | language | L3 | unsupported | no dialect (LB-Q-less) |
| Perl (.pl .pm .t by sniff) | language | L2 | L2 (L1 good, L2 good) | L1 good: range_agreement not measured |
| Scala (.scala .sc) | language | L4 | L2 (L1 good, L2 good) | L3 not met: file_edge_recall 0.0112 < 0.25 |
| Jupyter notebooks (.ipynb) | tool format | L2 | L2 (L1 excellent, L2 good) | L2 good: oracle tier C caps at good |
| Astro components (.astro) | language | L3 | L3 (L1 good, L2 excellent, L3 excellent) | L1 good: sym_recall 0.9362 < 0.95 |
| Vue single-file components (.vue) | language | L3 | L3 (L1 excellent, L2 excellent, L3 excellent) | excellent at target |
| Svelte components (.svelte) | language | L3 | none (measured) | L1 not met: sym_precision 0.3816 < 0.9 |
| Laravel Blade templates (.blade.php) | language | L3 | unsupported | no dialect (LB-Q-blade) |
| Jinja2 templates (.j2 .jinja .jinja2, templates/*.html by sniff) | language | L3 | unsupported | no dialect (LB-Q-jinja2) |

### T2

| Item | Kind | Target | Level met | Next gap |
|---|---|---|---|---|
| GitHub Actions workflows and composite actions | tool format | L3 | L3 (L1 excellent, L2 excellent, L3 good) | L3 good: oracle tier C caps at good |
| npm/pnpm/yarn package manifests (package.json, workspaces) | tool format | L3 | L3 (L1 excellent, L2 excellent, L3 excellent) | excellent at target |
| Python packaging (pyproject.toml, requirements*.txt, setup.cfg) | tool format | L2 | L2 (L1 good, L2 excellent) | L1 good: oracle tier C caps at good |
| Makefile / GNU make (Makefile, *.mk, GNUmakefile) | tool format | L2 | L2 (L1 excellent, L2 excellent) | excellent at target |
| GitLab CI (.gitlab-ci.yml, included files) | tool format | L3 | L3 (L1 excellent, L2 excellent, L3 excellent) | excellent at target |
| Maven POMs (pom.xml) | tool format | L3 | L3 (L1 excellent, L2 excellent, L3 excellent) | excellent at target |
| Cargo manifests (Cargo.toml, workspaces) | tool format | L3 | L3 (L1 excellent, L2 excellent, L3 excellent) | excellent at target |
| Go modules (go.mod, go.work) | tool format | L3 | L3 (L1 excellent, L2 excellent, L3 excellent) | excellent at target |
| MSBuild projects (.csproj .fsproj .vbproj .props .targets, .sln) | tool format | L3 | L3 (L1 excellent, L2 excellent, L3 excellent) | excellent at target |
| Composer manifests (composer.json) | tool format | L2 | L2 (L1 excellent, L2 excellent) | excellent at target |
| Bazel / Starlark (BUILD, BUILD.bazel, *.bzl, WORKSPACE, MODULE.bazel) | tool format | L3 | L3 (L1 good, L2 excellent, L3 good) | L1 good: sym_precision 0.9554 < 0.98 |
| CMake (CMakeLists.txt, *.cmake) | tool format | L3 | none (measured) | L1 not met: tenancy floor |
| Protocol Buffers (.proto) | tool format | L3 | L3 (L1 excellent, L2 excellent, L3 excellent) | excellent at target |
| GraphQL schemas and operations (.graphql .gql) | tool format | L3 | L2 (L1 excellent, L2 excellent) | L3 not met: xref_precision not measured |
| JSON Schema documents | tool format | L3 | unsupported | no dialect (LB-Q-json-schema) |
| Taskfile (go-task: Taskfile.yml) | tool format | L3 | L3 (L1 excellent, L2 excellent, L3 excellent) | excellent at target |
| Jenkins pipelines (Jenkinsfile, *.jenkinsfile) | tool format | L3 | L3 (L1 good, L2 good, L3 good) | L1 good: oracle tier C caps at good |
| Azure Pipelines (azure-pipelines.yml, templates) | tool format | L3 | unsupported | no dialect (LB-Q-azure-pipelines) |
| CircleCI (.circleci/config.yml) | tool format | L2 | L2 (L1 good, L2 good) | L1 good: sym_precision 0.9796 < 0.98 |
| Conan recipes and lockfiles (conanfile.txt, conanfile.py, conan.lock, conandata.yml) | tool format | L3 | L2 (L1 excellent, L2 excellent) | L3 not met: xref_precision not measured |
| vcpkg manifests (vcpkg.json, vcpkg-configuration.json) | tool format | L3 | L3 (L1 excellent, L2 excellent, L3 excellent) | excellent at target |
| Meson builds (meson.build, meson.options, meson_options.txt) | tool format | L3 | L2 (L1 excellent, L2 excellent) | L3 not met: xref_precision not measured |
| Clang compilation databases (compile_commands.json) | tool format | L2 | L2 (L1 excellent, L2 excellent) | excellent at target |

### T3

| Item | Kind | Target | Level met | Next gap |
|---|---|---|---|---|
| Kubernetes manifests (incl. CRDs) | tool format | L3 | L3 (L1 excellent, L2 good, L3 good) | L2 good: oracle tier C caps at good |
| Helm charts (Chart.yaml, values.yaml, templates/*.yaml, *.tpl) | tool format | L3 | L3 (L1 excellent, L2 excellent, L3 good) | L3 good: oracle tier C caps at good |
| Kustomize (kustomization.yaml) | tool format | L3 | L3 (L1 excellent, L2 excellent, L3 excellent) | excellent at target |
| Argo CD Applications and Argo Workflows | tool format | L3 | L3 (L1 excellent, L2 excellent, L3 excellent) | excellent at target |
| Ansible playbooks, roles and inventories | tool format | L3 | L3 (L1 good, L2 good, L3 good) | L1 good: oracle tier C caps at good |
| AWS CloudFormation / SAM templates | tool format | L3 | L3 (L1 excellent, L2 excellent, L3 good) | L3 good: oracle tier C caps at good |
| Terragrunt (terragrunt.hcl, root.hcl, a .hcl by content sniff) | tool format | L3 | L3 (L1 good, L2 good, L3 excellent) | L1 good: oracle tier C caps at good |
| HashiCorp HCL tool configs (Packer *.pkr.hcl, Nomad jobspecs, docker-bake.hcl) | tool format | L2 | L2 (L1 good, L2 good) | L1 good: oracle tier C caps at good |
| Pulumi YAML programs and project files (Pulumi.yaml, Pulumi.<stack>.yaml) | tool format | L2 | L2 (L1 excellent, L2 excellent) | excellent at target |
| Crossplane compositions and XRDs | tool format | L3 | L3 (L1 excellent, L2 good, L3 good) | L2 good: oracle tier C caps at good |
| Serverless Framework (serverless.yml) | tool format | L2 | L2 (L1 excellent, L2 excellent) | excellent at target |
| Azure Bicep (.bicep, .bicepparam) | tool format | L3 | L3 (L1 excellent, L2 excellent, L3 excellent) | excellent at target |
| Azure Resource Manager templates (azuredeploy.json shape, by content) | tool format | L3 | L3 (L1 excellent, L2 good, L3 excellent) | L2 good: ref_recall 0.9448 < 0.95 |
| AWS CDK apps (cdk.json; programs ride their language) | tool format | L2 | L2 (L1 excellent, L2 good) | L2 good: oracle tier C caps at good |
| Flux CD resources (Kustomization, HelmRelease, GitRepository, OCIRepository) | tool format | L3 | L3 (L1 excellent, L2 good, L3 good) | L2 good: oracle tier C caps at good |
| Timoni modules and bundles (timoni.cue, values.cue, templates/*.cue, bundle.cue, runtime.cue) | tool format | L3 | L1 (L1 good) | L2 not met: ref_precision 0.1611 < 0.9 |

### T4

| Item | Kind | Target | Level met | Next gap |
|---|---|---|---|---|
| Rego / OPA policy (.rego) | language | L3 | L3 (L1 excellent, L2 excellent, L3 good) | L3 good: file_edge_recall 0.3858 < 0.5 |
| Kyverno policies (ClusterPolicy, Policy, PolicyException) | tool format | L2 | L3 (L1 excellent, L2 excellent, L3 good) | excellent at target |
| OPA Gatekeeper ConstraintTemplates and constraints | tool format | L3 | L3 (L1 excellent, L2 excellent, L3 excellent) | excellent at target |
| Conftest policies (policy/*.rego, conftest.toml) | tool format | L2 | unsupported | no dialect (LB-Q-conftest) |
| Cedar policies and schemas (.cedar .cedarschema) | language | L3 | refused | re-plan (LB-Q-cedar-rest refused) |
| AWS IAM policy documents (JSON by content, inline in CloudFormation and Terraform) | tool format | L2 | L2 (L1 excellent, L2 excellent) | excellent at target |
| Azure Policy definitions and initiatives (policyRule JSON) | tool format | L2 | L2 (L1 good, L2 good) | L1 good: oracle tier C caps at good |
| Cloud Custodian policies (policies YAML) | tool format | L2 | L2 (L1 excellent, L2 excellent) | excellent at target |
| Checkov custom policies (YAML graph checks, Python checks) | tool format | L2 | unsupported | no dialect (LB-Q-checkov-policies) |
| HashiCorp Sentinel policies (.sentinel) | language | L2 | unsupported | no dialect (LB-Q-sentinel) |

### T5

| Item | Kind | Target | Level met | Next gap |
|---|---|---|---|---|
| nginx configuration (nginx.conf, .conf by sniff, sites-enabled/* by glob) | tool format | L3 | L3 (L1 excellent, L2 good, L3 good) | L2 good: oracle tier B caps at good |
| HAProxy configuration (haproxy.cfg, .cfg by sniff) | tool format | L3 | L3 (L1 excellent, L2 excellent, L3 excellent) | excellent at target |
| Caddyfile (Caddyfile, Caddyfile.* by glob, .caddyfile) | tool format | L3 | L3 (L1 excellent, L2 excellent, L3 excellent) | excellent at target |
| Envoy proxy configuration (bootstrap, xDS resource files) | tool format | L3 | L3 (L1 excellent, L2 excellent, L3 good) | L3 good: xref_precision 0.9398 < 0.95 |
| Kong declarative configuration (decK, DB-less) | tool format | L3 | L3 (L1 excellent, L2 good, L3 excellent) | L2 good: oracle tier C caps at good |
| Traefik static and file-provider configuration | tool format | L3 | L2 (L1 excellent, L2 excellent) | L3 not met: xref_precision 0.7241 < 0.9 |
| PgBouncer configuration (pgbouncer.ini) | tool format | L2 | unsupported | no dialect (LB-Q-pgbouncer) |
| ProxySQL configuration (proxysql.cnf) | tool format | L2 | unsupported | no dialect (LB-Q-proxysql) |

### T6

| Item | Kind | Target | Level met | Next gap |
|---|---|---|---|---|
| CUE (.cue) | language | L4 | L2 (L1 excellent, L2 excellent) | L3 not met: import_resolution not measured |
| fathom configuration (.fathom/config.toml) | tool format | L2 | L2 (L1 good, L2 good) | L1 good: oracle tier C caps at good |
| fathom concerns (.fathom/concerns.cue) | tool format | L2 | unsupported | no dialect (LB-Q-fathom-concerns) |
| fathom layer packs (.fathom/layers/*/layer.cue, .fathom/rules/*.cue) | tool format | L2 | unsupported | no dialect (LB-Q-fathom-layer-packs) |
| fathom plan grammar v2 rows (docs/plans/*.md) | tool format | L3 | L3 (L1 good, L2 good, L3 good) | L1 good: oracle tier C caps at good |

### T7

| Item | Kind | Target | Level met | Next gap |
|---|---|---|---|---|
| Prometheus rules, configuration and promtool tests | tool format | L3 | L3 (L1 excellent, L2 excellent, L3 excellent) | excellent at target |
| Alertmanager configuration (routes, receivers, inhibit rules, time intervals) | tool format | L3 | L3 (L1 excellent, L2 excellent, L3 excellent) | excellent at target |
| Grafana dashboards and provisioning (dashboard JSON, v2 resources, providers, datasources, alerting) | tool format | L3 | L3 (L1 good, L2 good, L3 good) | L1 good: oracle tier C caps at good |
| OpenTelemetry Collector configuration (receivers, processors, exporters, connectors, extensions, pipelines) | tool format | L3 | L3 (L1 excellent, L2 excellent, L3 good) | L3 good: oracle tier C caps at good |

Untiered entries (outside the excellence target set): 80 (abap, ada, android-manifest, apex, assembly, asyncapi, avro, buf, clojure, cobol, codeowners, commonlisp, crystal, cuda, d, dbt, dependency-bots, devcontainer, dhall, docker-compose, dockerfile, dotenv, earthfile, elixir, elm, emacs-lisp, erlang, fortran, fsharp, gdscript, gemfile-podfile, git-hooks-config, gleam, glsl, gradle, groovy, haskell, hlsl, json-yaml-toml, jsonnet, julia, justfile, kcl, latex, luau, markdown, matlab, mojo, nim, nix, nushell, objective-c, ocaml, odin, openapi, pascal, pkl, prisma, prolog, purescript, r, scheme-racket, skaffold, solidity, sql-migrations, systemd, tcl, tekton, templ, terraform, thrift, tiltfile, typst, vba, vbnet, verilog, vhdl, vimscript, wgsl, zig).

## Support levels (generated)

Generated by `go run ./cmd/indexbench lang-tracker > docs/languages.md` from the queue,
the registered dialects and the committed readings; never hand-edit it. A level is the
reading's computed `level_met` with each met level's grade; `tolerated` is registered with
no reading yet; `unsupported (disclosed)` is queued, not registered, and named per service
by `list_services`; `refused` is refused by its plan row (design §14.2): it claims no level,
is not dispatched and stays disclosed, whatever its reading measured.

Milestone M1 reached (languages at L1 34, L1 excellent 24, L2 29, L3 23, L4 10; tool formats at target 55)
Milestone M2 progress: languages at L1 34/45, at L1 excellent 24/35, at L2 29/25, at L3 23/14, at L4 10/10; tool formats at target 55/30

| Language or format | Kind | Engine | Grammar / parser | Target | Level met | Oracles | Typed edges | Reading | Next |
|---|---|---|---|---|---|---|---|---|---|
| TypeScript (.ts .tsx .mts .cts) | language | tags | tree-sitter/tree-sitter-typescript (MIT) | L4 | L4 (L1 good, L2 excellent, L3 excellent, L4 excellent) | A scip-typescript 0.4.0 | scip:scip-typescript | 2026-10-11 | at target (LB-Q-typescript) |
| JavaScript (.js .jsx .mjs .cjs) | language | tags | tree-sitter/tree-sitter-javascript (MIT) | L4 | L4 (L1 excellent, L2 excellent, L3 excellent, L4 good) | A scip-typescript 0.4.0 | scip:scip-typescript | 2026-10-10 | at target (LB-Q-javascript) |
| React components, props, hooks and React Router routes (.tsx .jsx .ts .js) | language | tags | tree-sitter/tree-sitter-typescript, tree-sitter/tree-sitter-javascript (the TS/JS dialects' own kinds) (MIT) | L3 | L3 (L1 good, L2 good, L3 good) | C react-docgen-typescript (Parser.getComponentInfo, Storybook's options, every prop kept) + the TypeScript checker react-docgen-typescript 2.4.0, typescript 5.9.3 | — | 2026-10-10 | at target (LB-Q-react) |
| Angular components and templates (.component.ts, .component.html) | language | tags | the typescript dialect's own kinds plus an Angular template grammar (verify at pin) (MIT (verify at pin)) | L3 | refused | — | — | — | re-plan (LB-Q-angular-rest refused) |
| Dockerfile / Containerfile | tool format | tags | camdencheek/tree-sitter-dockerfile (Apache-2.0) | L3 | L2 (L1 excellent, L2 excellent) | A buildkit-dockerfile-parser github.com/moby/buildkit v0.33.0 frontend/dockerfile (docker buildx build --print=outline substitute — no Docker daemon in this sandbox) | syntax | 2026-09-28 | LB-Q-dockerfile |
| Python (.py .pyi) | language | tags | tree-sitter/tree-sitter-python (MIT) | L4 | L4 (L1 excellent, L2 excellent, L3 excellent, L4 excellent) | A scip-python 0.6.6; B universal-ctags 6.2.1 | scip:scip-python | 2026-10-10 | at target (LB-Q-python) |
| HTML (.html .htm) | language | tags | tree-sitter/tree-sitter-html (MIT) | L3 | L3 (L1 excellent, L2 excellent, L3 excellent) | A lychee 0.24.2; A parse5 8.0.1; B universal-ctags 6.2.1 | file | 2026-10-11 | at target (LB-Q-html) |
| GitHub Actions workflows and composite actions | tool format | doc | pure-go:gopkg.in/yaml.v3 (n/a) | L3 | L3 (L1 excellent, L2 excellent, L3 good) | A @actions/workflow-parser (parseWorkflow + convertWorkflowTemplate; parseAction + convertActionTemplate) @actions/workflow-parser 0.3.61; C @actions/workflow-parser (parseWorkflow + convertWorkflowTemplate; parseAction + convertActionTemplate) + local uses: directory rule @actions/workflow-parser 0.3.61 | — | 2026-10-15 | at target (LB-Q-github-actions) |
| CSS (.css) | language | tags | tree-sitter/tree-sitter-css (MIT) | L3 | L3 (L1 good, L2 excellent, L3 excellent) | A lightningcss 1.33.0 | file | 2026-10-10 | at target (LB-Q-css) |
| SQL (.sql; schema and migrations) | language | tags | DerekStride/tree-sitter-sql (MIT) | L2 | L2 (L1 excellent, L2 good) | C sqlglot 27.8.0; B universal-ctags 6.2.1 | syntax | 2026-10-10 | at target (LB-Q-sql) |
| npm/pnpm/yarn package manifests (package.json, workspaces) | tool format | doc | pure-go:encoding/json (n/a) | L3 | L3 (L1 excellent, L2 excellent, L3 excellent) | A npm query ':root' (each manifest as npm loads it) 11.19.1; A npm query (workspace members) + pnpm 10.33.0 ls -r --filter '{<project>}...' (one project's dependency maps at a time) npm 11.19.1; pnpm 10.33.0; A npm query (workspace members) + yarn 4.10.3 workspaces list -v (the subject's own release, .yarn/releases/yarn-4.10.3.cjs) npm 11.19.1; yarn 4.10.3 | syntax | 2026-10-02 | at target (LB-Q-package-json) |
| Shell / Bash (.sh .bash, shebangs) | language | tags | tree-sitter/tree-sitter-bash (MIT) | L3 | L3 (L1 excellent, L2 good, L3 good) | B bash-source v3.14.1 (mvdan.cc/sh/v3 syntax, -ln bash); bash 5 lookup, cwd the script's directory then the subject root; checked by ShellCheck 0.11.0; C tree-sitter-tags:bash-tags.scm py-tree-sitter 0.25.2, tree-sitter-bash 0.25.1; B universal-ctags 6.2.1 | — | 2026-10-11 | at target (LB-Q-bash) |
| Java (.java) | language | tags | tree-sitter/tree-sitter-java (MIT) | L4 | L4 (L1 excellent, L2 excellent, L3 excellent, L4 excellent) | B javaparser 3.28.2; A scip-java 0.12.3 | scip:scip-java | 2026-10-10 | at target (LB-Q-java) |
| Python packaging (pyproject.toml, requirements*.txt, setup.cfg) | tool format | doc | pure-go:github.com/pelletier/go-toml/v2 (n/a) | L2 | L2 (L1 good, L2 excellent) | A dependency-groups (PEP 735 reference implementation: DependencyGroupResolver.lookup) pyproject_hooks 1.3.3; dependency-groups 1.3.2; flit_core 3.12.0; pdm-backend 2.5.0; setuptools 84.0.0; python 3.14.7; C pyproject build backend (prepare_metadata_for_build_wheel via pyproject_hooks) + dependency-groups (PEP 735 reference implementation) + tomllib for [tool.*] tables pyproject_hooks 1.3.3; dependency-groups 1.3.2; flit_core 3.12.0; pdm-backend 2.5.0; setuptools 84.0.0; python 3.14.7 | syntax | 2026-10-02 | at target (LB-Q-python-packaging) |
| C# (.cs) | language | tags | tree-sitter/tree-sitter-c-sharp (MIT) | L4 | L4 (L1 excellent, L2 excellent, L3 excellent, L4 excellent) | A roslyn-semantic Microsoft.CodeAnalysis.CSharp 5.9.0 (MSBuildWorkspace); B roslyn-syntax Microsoft.CodeAnalysis.CSharp 5.9.0; A scip-dotnet 0.1.0-SNAPSHOT | — | 2026-10-09 | at target (LB-Q-csharp) |
| C (.c, .h by sniff) | language | tags | tree-sitter/tree-sitter-c (MIT) | L4 | L4 (L1 excellent, L2 excellent, L3 excellent, L4 excellent) | A clang-preprocessor Apple clang version 21.0.0 (clang-2100.3.34.2); A scip-clang 0.4.0 | scip:scip-clang | 2026-10-11 | at target (LB-Q-c) |
| Terraform / OpenTofu (.tf .tfvars .tf.json) | tool format | native | native:github.com/hashicorp/hcl/v2 (MPL-2.0 (library dependency, unmodified)) | L3 | L3 (L1 excellent, L2 excellent, L3 good) | A opentofu (tofu show -json -config, tofu graph -type=plan) 1.13.0 | syntax | 2026-10-10 | at target (LB-Q-terraform) |
| C++ (.cc .cpp .cxx .hpp .hh .hxx, .h by sniff) | language | tags | tree-sitter/tree-sitter-cpp (MIT) | L4 | L4 (L1 excellent, L2 excellent, L3 good, L4 good) | A clang-ast Apple clang version 21.0.0 (clang-2100.3.34.2); A clang-preprocessor Apple clang version 21.0.0 (clang-2100.3.34.2); A scip-clang 0.4.0; B universal-ctags 6.2.1 | scip:scip-clang | 2026-10-10 | at target (LB-Q-cpp) |
| Go (.go) | language | native | native:go/parser+go/types (BSD-3-Clause) | L4 | L3 (L1 excellent, L2 excellent, L3 excellent) | A scip-go 0.2.7; B universal-ctags 6.2.1 | native | 2026-10-10 | LB-Q-go |
| Kubernetes manifests (incl. CRDs) | tool format | doc | pure-go:gopkg.in/yaml.v3 (n/a) | L3 | L3 (L1 excellent, L2 good, L3 good) | A kubectl annotate --local --dry-run=client -o json (lines: gopkg.in/yaml.v3) kubectl v1.36.1; C kubernetes-oracle (independent gopkg.in/yaml.v3 walk) gopkg.in/yaml.v3 v3.0.1; A kubernetes-oracle selectors (labels.ValidatedSelectorFromSet, Selector.Matches; apimachinery decode) k8s.io/apimachinery v0.37.0 | — | 2026-10-15 | at target (LB-Q-kubernetes) |
| PowerShell (.ps1 .psm1 .psd1) | language | tags | airbus-cert/tree-sitter-powershell (MIT (verify at pin)) | L3 | L3 (L1 excellent, L2 good, L3 excellent) | B powershell-ast PowerShell 7.5.4 System.Management.Automation.Language.Parser; A powershell-ast-imports PowerShell 7.5.4 System.Management.Automation.Language.Parser, StaticParameterBinder, Test-ModuleManifest | syntax | 2026-10-15 | at target (LB-Q-powershell) |
| Docker Compose files | tool format | doc | pure-go:gopkg.in/yaml.v3 (n/a) | L3 | L3 (L1 excellent, L2 excellent, L3 excellent) | A docker compose config --format json 5.1.4 | — | 2026-10-09 | at target (LB-Q-docker-compose) |
| PHP (.php) | language | tags | tree-sitter/tree-sitter-php (MIT) | L4 | L4 (L1 excellent, L2 good, L3 excellent, L4 excellent) | B php-parser 5.7.0.0; A scip-php 0.0.1 | syntax | 2026-10-10 | at target (LB-Q-php) |
| Helm charts (Chart.yaml, values.yaml, templates/*.yaml, *.tpl) | tool format | tags+doc | ngalaiko/tree-sitter-go-template (MIT (verify at pin)) | L3 | L3 (L1 excellent, L2 excellent, L3 good) | A helm (helm.sh/helm/v4 loader, helm template offline with define ablation; templates by Go text/template/parse; lines: gopkg.in/yaml.v3) helm v4.3.0+gbec5b06; helm.sh/helm/v4 v4.3.0; A helm dependency list (file:// dependencies; helm dependency update --skip-refresh in a scratch copy for one not under charts/) helm v4.3.0+gbec5b06; C helm-oracle (Go text/template/parse for templates, gopkg.in/yaml.v3 for Chart.yaml and values.yaml) text/template/parse (go1.26); gopkg.in/yaml.v3 v3.0.1 | — | 2026-10-15 | at target (LB-Q-helm) |
| Rust (.rs) | language | tags | tree-sitter/tree-sitter-rust (MIT) | L4 | L4 (L1 excellent, L2 excellent, L3 excellent, L4 excellent) | A rust-analyzer 0.0.0 (03fcb77246 2026-09-27) | scip:rust-analyzer | 2026-10-10 | at target (LB-Q-rust) |
| Makefile / GNU make (Makefile, *.mk, GNUmakefile) | tool format | tags | tree-sitter-grammars/tree-sitter-make (MIT) | L2 | L2 (L1 excellent, L2 excellent) | A make -pn 4.4.1; B universal-ctags 6.2.1 | syntax | 2026-09-29 | at target (LB-Q-makefile) |
| Kotlin (.kt .kts) | language | tags | fwcd/tree-sitter-kotlin (MIT) | L4 | L3 (L1 excellent, L2 good, L3 good) | B kotlin-psi 2.3.20 | scip:scip-java | 2026-10-10 | LB-Q-kotlin |
| Lua (.lua) | language | tags | tree-sitter-grammars/tree-sitter-lua (MIT) | L3 | L3 (L1 excellent, L2 good, L3 good) | B lua-require luaparse 0.3.1 (luaVersion LuaJIT) statements; Lua 5.4.7 package.searchpath lookup; B luaparse 0.3.1 (luaVersion LuaJIT); C tree-sitter-tags:lua-tags.scm py-tree-sitter 0.25.2, tree-sitter-lua 0.5.0 | syntax | 2026-10-19 | at target (LB-Q-lua) |
| GitLab CI (.gitlab-ci.yml, included files) | tool format | doc | pure-go:gopkg.in/yaml.v3 (n/a) | L3 | L3 (L1 excellent, L2 excellent, L3 excellent) | A gitlab-ci-local --list-json gitlab-ci-local 4.75.1; A gitlab-ci-local --preview ablation (each written needs, dependencies and extends, and each cross-file target's declaration, removed in a scratch copy) gitlab-ci-local 4.75.1 | — | 2026-10-02 | at target (LB-Q-gitlab-ci) |
| Ruby (.rb, Rakefile, Gemfile by basename) | language | tags | tree-sitter/tree-sitter-ruby (MIT) | L3 | L3 (L1 excellent, L2 good, L3 good) | B ruby-require ruby 2.6.10; C tree-sitter-tags:ruby-tags.scm py-tree-sitter 0.25.2, tree-sitter-ruby 0.23.1; B universal-ctags 6.2.1 | scip:scip-ruby | 2026-10-03 | at target (LB-Q-ruby) |
| Maven POMs (pom.xml) | tool format | doc | pure-go:encoding/xml (n/a) | L3 | L3 (L1 excellent, L2 excellent, L3 excellent) | A mvn help:effective-pom Maven 3.9.16; maven-help-plugin 3.5.1; A mvn help:effective-pom -Dverbose (input locations) + <module> ablation Maven 3.9.16; maven-help-plugin 3.5.1 | syntax | 2026-10-02 | at target (LB-Q-maven) |
| Dart (.dart) | language | tags | UserNobody14/tree-sitter-dart (MIT) | L4 | L4 (L1 excellent, L2 excellent, L3 excellent, L4 excellent) | A scip-dart 1.7.0 | scip:scip-dart | 2026-10-10 | at target (LB-Q-dart) |
| Cargo manifests (Cargo.toml, workspaces) | tool format | doc | pure-go:github.com/pelletier/go-toml/v2 (n/a) | L3 | L3 (L1 excellent, L2 excellent, L3 excellent) | A cargo metadata (--no-deps, and --all-features for the resolved graph) cargo 1.98.1 (797e8a9bc 2026-08-05) | syntax | 2026-09-29 | at target (LB-Q-cargo) |
| Swift (.swift) | language | tags | alex-pinkus/tree-sitter-swift (MIT) | L3 | refused; measured L2 (L1 excellent, L2 good) | B swiftc -frontend -dump-parse 6.4 | — | 2026-09-29 | re-plan (LB-Q-swift-rest refused) |
| Go modules (go.mod, go.work) | tool format | native | native:golang.org/x/mod (BSD-3-Clause) | L3 | L3 (L1 excellent, L2 excellent, L3 excellent) | A go mod edit -json, go work edit -json, go list -e -m -json all go1.26.3 darwin/arm64 | syntax | 2026-09-29 | at target (LB-Q-go-mod) |
| SCSS (.scss) | language | tags | tree-sitter-grammars/tree-sitter-scss (MIT (verify at pin)) | L3 | L1 (L1 good) | A dart-sass sass 1.105.1, sass-parser 0.4.57 | file | 2026-10-10 | LB-Q-scss |
| Sass (indented syntax) (.sass) | language | tags | none pinned: no tree-sitter grammar parses the indented syntax (O853) (n/a (no grammar pinned)) | L3 | unsupported (disclosed) | — | — | — | LB-Q-sass |
| Tailwind CSS v4 at-rules and custom utilities (.css) | language | tags | tree-sitter/tree-sitter-css (the css dialect's grammar) (MIT) | L2 | unsupported (disclosed) | — | — | — | LB-Q-tailwind |
| Less (.less) | language | tags | jimliang/tree-sitter-less (verify at pin) | L3 | unsupported (disclosed) | — | — | — | LB-Q-less |
| MSBuild projects (.csproj .fsproj .vbproj .props .targets, .sln) | tool format | doc | pure-go:encoding/xml (n/a) | L3 | L3 (L1 excellent, L2 excellent, L3 excellent) | A dotnet msbuild (-getItem, -getProperty and -preprocess) MSBuild 18.9.11.42413; .NET SDK 10.0.401 | syntax | 2026-09-29 | at target (LB-Q-msbuild) |
| Kustomize (kustomization.yaml) | tool format | doc | pure-go:gopkg.in/yaml.v3 (n/a) | L3 | L3 (L1 excellent, L2 excellent, L3 excellent) | A kustomize (kubectl-embedded) v5.8.1 | syntax | 2026-09-29 | at target (LB-Q-kustomize) |
| Assembly (.s .S .asm) | language | tags | RubixDev/tree-sitter-asm (MIT) | L1 | refused | — | — | — | re-plan (LB-Q-assembly-rest refused) |
| Argo CD Applications and Argo Workflows | tool format | doc | pure-go:gopkg.in/yaml.v3 (n/a) | L3 | L3 (L1 excellent, L2 excellent, L3 excellent) | A argo lint --offline (acceptance, reference ablation) + argo-workflows v1alpha1 types, strict (definitions); lines: gopkg.in/yaml.v3 argo: v4.1.4+b5b4d66.dirty; github.com/argoproj/argo-workflows/v4 v4.1.4 | — | 2026-10-02 | at target (LB-Q-argo) |
| R (.R .r) | language | tags | r-lib/tree-sitter-r (MIT) | L2 | L2 (L1 excellent, L2 good) | C tree-sitter-tags:r-tags.scm py-tree-sitter 0.25.2, tree-sitter-r 1.3.0; B universal-ctags 6.2.1 | syntax | 2026-10-19 | at target (LB-Q-r) |
| Ansible playbooks, roles and inventories | tool format | doc | pure-go:gopkg.in/yaml.v3 (n/a) | L3 | L3 (L1 good, L2 good, L3 good) | C ansible-oracle (independent gopkg.in/yaml.v3 walk) gopkg.in/yaml.v3 v3.0.1 | — | 2026-10-02 | at target (LB-Q-ansible) |
| Groovy (.groovy .gvy) | language | tags | murtaza64/tree-sitter-groovy (verify at pin) | L2 | refused | — | — | — | re-plan (LB-Q-groovy-rest refused) |
| Composer manifests (composer.json) | tool format | doc | pure-go:encoding/json (n/a) | L2 | L2 (L1 excellent, L2 excellent) | A composer (Factory::create root package: getPrettyName, getScripts, getRequires, getDevRequires; EventDispatcher::isComposerScript) composer 2.10.3 | — | 2026-10-09 | at target (LB-Q-composer) |
| Visual Basic .NET (.vb) | language | tags | govindbanura/tree-sitter-vbnet (weak, 2026) (verify at pin) | L1 | unsupported (disclosed) | — | — | — | LB-Q-vbnet |
| Bazel / Starlark (BUILD, BUILD.bazel, *.bzl, WORKSPACE, MODULE.bazel) | tool format | tags | tree-sitter-grammars/tree-sitter-starlark (MIT) | L3 | L3 (L1 good, L2 excellent, L3 good) | A bazel query --output=streamed_jsonproto bazel 8.7.0 | syntax | 2026-10-19 | at target (LB-Q-bazel) |
| MATLAB (.m by sniff) | language | tags | acristoffers/tree-sitter-matlab (MIT) | L1 | tolerated | — | — | — | LB-Q-matlab |
| CMake (CMakeLists.txt, *.cmake) | tool format | tags | uyha/tree-sitter-cmake (MIT (verify at pin)) | L3 | none (measured) | A cmake 4.4.3 (File API codemodel-v2, --trace-expand json-v1); B universal-ctags 6.2.1 | syntax | 2026-10-01 | LB-Q-cmake |
| Perl (.pl .pm .t by sniff) | language | tags | tree-sitter-perl/tree-sitter-perl (MIT) | L2 | L2 (L1 good, L2 good) | C ppi-calls PPI 1.270; B universal-ctags 6.2.1 | syntax | 2026-10-19 | at target (LB-Q-perl) |
| Protocol Buffers (.proto) | tool format | tags | coder3101/tree-sitter-proto (MIT (verify at pin)) | L3 | L3 (L1 excellent, L2 excellent, L3 excellent) | A protoc 34.1 | syntax | 2026-10-01 | at target (LB-Q-protobuf) |
| GDScript (.gd) | language | tags | PrestonKnopp/tree-sitter-gdscript (MIT) | L2 | unsupported (disclosed) | — | — | — | LB-Q-gdscript |
| GraphQL schemas and operations (.graphql .gql) | tool format | tags | bkegley/tree-sitter-graphql (MIT (verify at pin)) | L3 | L2 (L1 excellent, L2 excellent) | A graphql-js 16.14.2 | syntax | 2026-10-19 | LB-Q-graphql |
| Elixir (.ex .exs) | language | tags | elixir-lang/tree-sitter-elixir (Apache-2.0) | L3 | L3 (L1 excellent, L2 good, L3 good) | B elixir-ast Elixir 1.19.6 Code.string_to_quoted/2 (OTP 29); C tree-sitter-tags:elixir-tags.scm py-tree-sitter 0.25.2, tree-sitter-elixir 0.3.5 | syntax | 2026-10-19 | at target (LB-Q-elixir) |
| OpenAPI / Swagger documents | tool format | doc | pure-go:gopkg.in/yaml.v3 (n/a) | L3 | L3 (L1 excellent, L2 excellent, L3 excellent) | A @apidevtools/swagger-parser 13.1.0; A @apidevtools/swagger-parser ($refs.paths) 13.1.0 | — | 2026-10-01 | at target (LB-Q-openapi) |
| Prometheus rules, configuration and promtool tests | tool format | doc | pure-go:gopkg.in/yaml.v3 (n/a) | L3 | L3 (L1 excellent, L2 excellent, L3 excellent) | A promtool 3.15.0 (5241a27f); A promtool+routes 3.15.0 (5241a27f) | — | 2026-10-10 | at target (LB-Q-prometheus) |
| Alertmanager configuration (routes, receivers, inhibit rules, time intervals) | tool format | doc | pure-go:gopkg.in/yaml.v3 (n/a) | L3 | L3 (L1 excellent, L2 excellent, L3 excellent) | A amtool v0.34.1 (amtool, config and dispatch packages); A amtool+dispatch v0.34.1 (amtool, config and dispatch packages) | — | 2026-10-10 | at target (LB-Q-alertmanager) |
| Grafana dashboards and provisioning (dashboard JSON, v2 resources, providers, datasources, alerting) | tool format | doc | pure-go:gopkg.in/yaml.v3 (n/a) | L3 | L3 (L1 good, L2 good, L3 good) | C grafana-model-walk 1 (prometheus/promql/parser v0.315.0 ExtractSelectors) | — | 2026-10-10 | at target (LB-Q-grafana-dashboard) |
| JSON Schema documents | tool format | doc | pure-go:encoding/json (n/a) | L3 | unsupported (disclosed) | — | — | — | LB-Q-json-schema |
| Scala (.scala .sc) | language | tags | tree-sitter/tree-sitter-scala (MIT) | L4 | L2 (L1 good, L2 good) | B scalameta 4.17.4 | scip:scip-java | 2026-10-10 | LB-Q-scala |
| Nix (.nix, flake.nix) | tool format | tags | nix-community/tree-sitter-nix (MIT) | L3 | unsupported (disclosed) | — | — | — | LB-Q-nix |
| Objective-C (.m .mm, .h by sniff) | language | tags | tree-sitter-grammars/tree-sitter-objc (MIT) | L3 | unsupported (disclosed) | — | — | — | LB-Q-objective-c |
| Jupyter notebooks (.ipynb) | tool format | native | pure-go:encoding/json + python dialect (n/a) | L2 | L2 (L1 excellent, L2 good) | B ctags on the exported cells, nbformat 5.11.1 Universal Ctags 6.2.1, Copyright (C) 2015-2025 Universal Ctags Team; C python ast on the exported cells, nbformat 5.11.1 python 3.13.5 | syntax | 2026-10-19 | at target (LB-Q-jupyter) |
| Luau (.luau) | language | tags | tree-sitter-grammars/tree-sitter-luau (MIT) | L2 | L2 (L1 excellent, L2 good) | B luau-ast luau-ast (luau-lang/luau 0.743); C tree-sitter-tags:lua-tags.scm py-tree-sitter 0.25.2, tree-sitter-luau 1.2.0 | syntax | 2026-10-19 | at target (LB-Q-luau) |
| Taskfile (go-task: Taskfile.yml) | tool format | doc | pure-go:gopkg.in/yaml.v3 (n/a) | L3 | L3 (L1 excellent, L2 excellent, L3 excellent) | A task --dry --force ablation (each deps entry and task: call cut from a scratch copy with every sh: and if: set to true) go-task 3.52.0; A task --list-all --json go-task 3.52.0 | syntax | 2026-10-02 | at target (LB-Q-taskfile) |
| Rego / OPA policy (.rego) | language | native | native:pure-go (n/a) | L3 | L3 (L1 excellent, L2 excellent, L3 good) | A opa parse 1.18.2 | file | 2026-10-02 | at target (LB-Q-rego) |
| Git hook managers (lefthook.yml, .pre-commit-config.yaml) | tool format | doc | pure-go:gopkg.in/yaml.v3 (n/a) | L1 | L1 (L1 good) | A lefthook dump 1.13.6; C ruby psych 3.1.0 (ruby 2.6.10) | syntax | 2026-09-29 | at target (LB-Q-git-hooks-config) |
| Astro components (.astro) | language | tags | virchau13/tree-sitter-astro (MIT) | L3 | L3 (L1 good, L2 excellent, L3 excellent) | A @astrojs/compiler parse + typescript createSourceFile @astrojs/compiler 4.0.0, typescript 5.9.3 | file | 2026-10-11 | at target (LB-Q-astro) |
| Gradle builds (build.gradle, build.gradle.kts, settings.gradle*) | tool format | tags | murtaza64/tree-sitter-groovy + fwcd/tree-sitter-kotlin (MIT) | L3 | L2 (L1 excellent, L2 excellent) | A gradle (help, init-script model) 9.7.0 (spring-boot), 8.14.1 (kafka) | syntax | 2026-10-01 | LB-Q-gradle |
| Vue single-file components (.vue) | language | tags | tree-sitter-grammars/tree-sitter-vue (MIT) | L3 | L3 (L1 excellent, L2 excellent, L3 excellent) | A @vue/compiler-sfc 3.5.43 | file | 2026-10-11 | at target (LB-Q-vue) |
| Jenkins pipelines (Jenkinsfile, *.jenkinsfile) | tool format | tags | murtaza64/tree-sitter-groovy (MIT) | L3 | L3 (L1 good, L2 good, L3 good) | C groovy AST (CONVERSION) + jenkinsfile walk 4.0.33 | syntax | 2026-10-15 | at target (LB-Q-jenkinsfile) |
| Svelte components (.svelte) | language | tags | tree-sitter-grammars/tree-sitter-svelte (MIT) | L3 | none (measured) | A svelte/compiler parse 5.57.2 | file | 2026-10-11 | LB-Q-svelte |
| Azure Pipelines (azure-pipelines.yml, templates) | tool format | doc | pure-go:gopkg.in/yaml.v3 (n/a) | L3 | unsupported (disclosed) | — | — | — | LB-Q-azure-pipelines |
| CUE (.cue) | language | native | native:cuelang.org/go (Apache-2.0) | L4 | L2 (L1 excellent, L2 excellent) | A cue lsp (cue vet gate) v0.16.1 | syntax | 2026-10-03 | LB-Q-cue |
| CircleCI (.circleci/config.yml) | tool format | doc | pure-go:gopkg.in/yaml.v3 (n/a) | L2 | L2 (L1 good, L2 good) | A circleci config process circleci 1.7.0 (1df22bb0b7b6); C circleci config process (workflow jobs, requires) and a yaml.v3 walk of steps and executor (declared commands and executors only) circleci 1.7.0 (1df22bb0b7b6) | — | 2026-10-19 | at target (LB-Q-circleci) |
| Delphi / Object Pascal (.pas .dpr, .pp by sniff) | language | tags | Isopod/tree-sitter-pascal (MIT) | L1 | unsupported (disclosed) | — | — | — | LB-Q-pascal |
| Fortran (.f90 .f95 .f03 .f .for) | language | tags | stadelmanma/tree-sitter-fortran (MIT) | L2 | unsupported (disclosed) | — | — | — | LB-Q-fortran |
| AWS CloudFormation / SAM templates | tool format | doc | pure-go:gopkg.in/yaml.v3 (n/a) | L3 | L3 (L1 excellent, L2 excellent, L3 good) | A cfn-lint 1.52.1; C cfn-lint (nested templates, identity joins and Lambda handlers resolved by the adapter) 1.52.1 | — | 2026-10-10 | at target (LB-Q-cloudformation) |
| Ada (.adb .ads) | language | tags | briot/tree-sitter-ada (MIT (verify at pin)) | L1 | L1 (L1 good) | B universal-ctags 6.2.1 | syntax | 2026-09-29 | at target (LB-Q-ada) |
| COBOL (.cob .cbl .cpy) | language | tags | yutaro-sakamoto/tree-sitter-cobol (MIT) | L1 | unsupported (disclosed) | — | — | — | LB-Q-cobol |
| Terragrunt (terragrunt.hcl, root.hcl, a .hcl by content sniff) | tool format | native | native:github.com/hashicorp/hcl/v2 (MPL-2.0 (library dependency, unmodified)) | L3 | L3 (L1 good, L2 good, L3 excellent) | C python-hcl2 (HCL2 parse tree) 8.1.4; A terragrunt render/find v1.1.6 | syntax | 2026-10-02 | at target (LB-Q-terragrunt) |
| Zig (.zig) | language | tags | tree-sitter-grammars/tree-sitter-zig (MIT) | L3 | unsupported (disclosed) | — | — | — | LB-Q-zig |
| Erlang (.erl .hrl) | language | tags | WhatsApp/tree-sitter-erlang (Apache-2.0) | L2 | unsupported (disclosed) | — | — | — | LB-Q-erlang |
| HashiCorp HCL tool configs (Packer *.pkr.hcl, Nomad jobspecs, docker-bake.hcl) | tool format | native | native:github.com/hashicorp/hcl/v2 (MPL-2.0 (library dependency, unmodified)) | L2 | L2 (L1 good, L2 good) | C python-hcl2 (HCL2 parse tree) 8.1.4 | syntax | 2026-10-02 | at target (LB-Q-hcl-tools) |
| F# (.fs .fsi .fsx, .fs by sniff) | language | tags | ionide/tree-sitter-fsharp (MIT) | L2 | unsupported (disclosed) | — | — | — | LB-Q-fsharp |
| OCaml (.ml .mli) | language | tags | tree-sitter/tree-sitter-ocaml (MIT) | L2 | unsupported (disclosed) | — | — | — | LB-Q-ocaml |
| Pulumi YAML programs and project files (Pulumi.yaml, Pulumi.<stack>.yaml) | tool format | doc | pure-go:gopkg.in/yaml.v3 (n/a) | L2 | L2 (L1 excellent, L2 excellent) | A Pulumi's own loaders (workspace.LoadProject, pulumiyaml.LoadYAMLBytes and its interpolation parse) + an independent YAML reader for lines pulumi-yaml v1.36.0, pulumi sdk v3.245.0, goccy/go-yaml v1.19.2 | syntax | 2026-10-02 | at target (LB-Q-pulumi-yaml) |
| Gleam (.gleam) | language | tags | gleam-lang/tree-sitter-gleam (Apache-2.0) | L3 | unsupported (disclosed) | — | — | — | LB-Q-gleam |
| Prolog (.pl by sniff, .pro) | language | tags | foxyseta/tree-sitter-prolog (verify at pin) | L1 | unsupported (disclosed) | — | — | — | LB-Q-prolog |
| Tekton pipelines and tasks | tool format | doc | pure-go:gopkg.in/yaml.v3 (n/a) | L2 | unsupported (disclosed) | — | — | — | LB-Q-tekton |
| Haskell (.hs) | language | tags | tree-sitter-grammars/tree-sitter-haskell (MIT) | L3 | unsupported (disclosed) | — | — | — | LB-Q-haskell |
| Julia (.jl) | language | tags | tree-sitter-grammars/tree-sitter-julia (MIT) | L2 | unsupported (disclosed) | — | — | — | LB-Q-julia |
| Crossplane compositions and XRDs | tool format | doc | pure-go:gopkg.in/yaml.v3 (n/a) | L3 | L3 (L1 excellent, L2 good, L3 good) | A crossplane resource validate, offline, against crossplane's own CRDs (cluster/crds at the subject's tag); lines: gopkg.in/yaml.v3 Client Version: v2.5.0; C crossplane-oracle (independent gopkg.in/yaml.v3 walk) gopkg.in/yaml.v3 v3.0.1 | — | 2026-10-02 | at target (LB-Q-crossplane) |
| Clojure (.clj .cljs .cljc .edn) | language | tags | sogaiu/tree-sitter-clojure (CC0-1.0) | L2 | unsupported (disclosed) | — | — | — | LB-Q-clojure |
| Common Lisp (.lisp .lsp .asd) | language | tags | tree-sitter-grammars/tree-sitter-commonlisp (MIT) | L1 | L1 (L1 good) | B universal-ctags 6.2.1 | syntax | 2026-09-29 | at target (LB-Q-commonlisp) |
| Skaffold (skaffold.yaml) | tool format | doc | pure-go:gopkg.in/yaml.v3 (n/a) | L2 | unsupported (disclosed) | — | — | — | LB-Q-skaffold |
| Scheme and Racket (.scm .ss .rkt) | language | tags | 6cdh/tree-sitter-scheme + 6cdh/tree-sitter-racket (MIT) | L1 | unsupported (disclosed) | — | — | — | LB-Q-scheme-racket |
| Solidity (.sol) | language | tags | JoranHonig/tree-sitter-solidity (MIT) | L2 | unsupported (disclosed) | — | — | — | LB-Q-solidity |
| Tiltfile (Starlark) | tool format | tags | tree-sitter-grammars/tree-sitter-starlark (MIT) | L2 | L2 (L1 good, L2 good) | C tree-sitter-tags:starlark-tags.scm py-tree-sitter 0.25.2, tree-sitter-starlark 1.3.0 | syntax | 2026-10-19 | at target (LB-Q-tiltfile) |
| Elm (.elm) | language | tags | elm-tooling/tree-sitter-elm (MIT) | L2 | unsupported (disclosed) | — | — | — | LB-Q-elm |
| Nim (.nim .nims) | language | tags | alaviss/tree-sitter-nim (MPL-2.0 (unmodified; verify at pin)) | L2 | unsupported (disclosed) | — | — | — | LB-Q-nim |
| just (justfile, .justfile) | tool format | tags | IndianBoy42/tree-sitter-just (MIT (verify at pin)) | L2 | unsupported (disclosed) | — | — | — | LB-Q-justfile |
| Crystal (.cr) | language | tags | crystal-lang-tools/tree-sitter-crystal (MIT (verify at pin)) | L2 | unsupported (disclosed) | — | — | — | LB-Q-crystal |
| D (.d) | language | tags | gdamore/tree-sitter-d (MIT) | L2 | unsupported (disclosed) | — | — | — | LB-Q-d |
| Earthly (Earthfile) | tool format | tags | glehmann/tree-sitter-earthfile (verify at pin) | L2 | unsupported (disclosed) | — | — | — | LB-Q-earthfile |
| Markdown (.md .mdx) | tool format | native | native:github.com/yuin/goldmark (MIT) | L3 | L3 (L1 excellent, L2 good, L3 good) | B lychee+universal-ctags lychee 0.24.2, ctags 6.2.1; B universal-ctags 6.2.1 | syntax | 2026-10-19 | at target (LB-Q-markdown) |
| VBA (.bas .cls .frm by sniff) | language | tags | harumiWeb/tree-sitter-vba (weak) (verify at pin) | L1 | unsupported (disclosed) | — | — | — | LB-Q-vba |
| Generic JSON, YAML and TOML documents | tool format | doc | pure-go:encoding/json + gopkg.in/yaml.v3 + go-toml/v2 (n/a) | L1 | tolerated | — | syntax | — | LB-Q-json-yaml-toml |
| Verilog / SystemVerilog (.v by sniff, .sv .svh .vh) | language | tags | gmlarumbe/tree-sitter-systemverilog (MIT) | L1 | unsupported (disclosed) | — | — | — | LB-Q-verilog |
| templ components (.templ) | tool format | tags | vrischmann/tree-sitter-templ (MIT (verify at pin)) | L2 | tolerated | — | syntax | — | LB-Q-templ |
| VHDL (.vhd .vhdl) | language | tags | jpt13653903/tree-sitter-vhdl (simplified by design) (verify at pin) | L1 | L1 (L1 excellent) | B universal-ctags 6.2.1 | syntax | 2026-09-29 | at target (LB-Q-vhdl) |
| Dotenv files (.env, .env.example, *.env) | tool format | doc | pure-go:line parser (n/a) | L1 | unsupported (disclosed) | — | — | — | LB-Q-dotenv |
| GLSL shaders (.glsl .vert .frag .comp, .fs by sniff) | language | tags | tree-sitter-grammars/tree-sitter-glsl (MIT) | L1 | unsupported (disclosed) | — | — | — | LB-Q-glsl |
| nginx configuration (nginx.conf, .conf by sniff, sites-enabled/* by glob) | tool format | native | native:pure-go (n/a) | L3 | L3 (L1 excellent, L2 good, L3 good) | B crossplane 0.5.8 | syntax | 2026-10-02 | at target (LB-Q-nginx) |
| HAProxy configuration (haproxy.cfg, .cfg by sniff) | tool format | native | native:pure-go (n/a) | L3 | L3 (L1 excellent, L2 excellent, L3 excellent) | A haproxy 3.4.6-56332c5; A haproxy+docker-compose 3.4.6-56332c5 | syntax | 2026-10-02 | at target (LB-Q-haproxy) |
| Caddyfile (Caddyfile, Caddyfile.* by glob, .caddyfile) | tool format | native | native:pure-go (n/a) | L3 | L3 (L1 excellent, L2 excellent, L3 excellent) | A caddy v2.11.6; A caddy+docker-compose v2.11.6 | syntax | 2026-10-02 | at target (LB-Q-caddyfile) |
| systemd units (*.service *.timer *.socket …) | tool format | doc | pure-go:INI parser (n/a) | L2 | unsupported (disclosed) | — | — | — | LB-Q-systemd |
| HLSL shaders (.hlsl .hlsli .fx) | language | tags | tree-sitter-grammars/tree-sitter-hlsl (MIT) | L1 | unsupported (disclosed) | — | — | — | LB-Q-hlsl |
| Dev containers (devcontainer.json, devcontainer-feature.json) | tool format | doc | pure-go:JSONC (n/a) | L2 | unsupported (disclosed) | — | — | — | LB-Q-devcontainer |
| Envoy proxy configuration (bootstrap, xDS resource files) | tool format | doc | pure-go:yaml.v3 (n/a) | L3 | L3 (L1 excellent, L2 excellent, L3 good) | A envoy 1.39.1 | syntax | 2026-10-02 | at target (LB-Q-envoy) |
| OpenTelemetry Collector configuration (receivers, processors, exporters, connectors, extensions, pipelines) | tool format | doc | pure-go:yaml.v3 (n/a) | L3 | L3 (L1 excellent, L2 excellent, L3 good) | A otelcol-contrib validate + collector confmap decode (component.ID, pipeline.ID) otelcol-contrib version 0.162.0; go.opentelemetry.io/collector v1.68.0; C otelcol-oracle routes (yaml.v3 walk, the Kubernetes and compose DNS rules) gopkg.in/yaml.v3 v3.0.1 | — | 2026-10-09 | at target (LB-Q-otel-collector) |
| Kong declarative configuration (decK, DB-less) | tool format | doc | pure-go:yaml.v3 (n/a) | L3 | L3 (L1 excellent, L2 good, L3 excellent) | A deck v1.68.0 (34b752e) | syntax | 2026-10-02 | at target (LB-Q-kong) |
| Traefik static and file-provider configuration | tool format | doc | pure-go:yaml.v3 + go-toml/v2 (n/a) | L3 | L2 (L1 excellent, L2 excellent) | A traefik 3.7.13 | syntax | 2026-10-02 | LB-Q-traefik |
| WGSL shaders (.wgsl) | language | tags | szebniok/tree-sitter-wgsl (or tree-sitter-grammars/tree-sitter-wgsl-bevy) (MIT (verify at pin)) | L1 | unsupported (disclosed) | — | — | — | LB-Q-wgsl |
| Renovate and Dependabot configs | tool format | doc | pure-go:JSON5 + gopkg.in/yaml.v3 (n/a) | L1 | unsupported (disclosed) | — | — | — | LB-Q-dependency-bots |
| CUDA (.cu .cuh) | language | tags | tree-sitter-grammars/tree-sitter-cuda (MIT) | L3 | unsupported (disclosed) | — | — | — | LB-Q-cuda |
| Jsonnet (.jsonnet .libsonnet) | tool format | tags | sourcegraph/tree-sitter-jsonnet (MIT) | L3 | unsupported (disclosed) | — | — | — | LB-Q-jsonnet |
| Salesforce Apex (.cls by glob, .trigger) | language | tags | aheber/tree-sitter-sfapex (MIT) | L2 | unsupported (disclosed) | — | — | — | LB-Q-apex |
| Pkl (.pkl) | tool format | tags | apple/tree-sitter-pkl (Apache-2.0) | L2 | unsupported (disclosed) | — | — | — | LB-Q-pkl |
| ABAP (.abap) | language | tags | kennyhml/tree-sitter-abap (weak, emerging) (verify at pin) | L1 | unsupported (disclosed) | — | — | — | LB-Q-abap |
| KCL (.k) | tool format | tags | kcl-lang/tree-sitter-kcl (Apache-2.0) | L2 | unsupported (disclosed) | — | — | — | LB-Q-kcl |
| Dhall (.dhall) | tool format | tags | jbellerb/tree-sitter-dhall (MIT (verify at pin)) | L1 | unsupported (disclosed) | — | — | — | LB-Q-dhall |
| Nushell (.nu) | language | tags | nushell/tree-sitter-nu (MIT) | L2 | unsupported (disclosed) | — | — | — | LB-Q-nushell |
| Apache Thrift IDL (.thrift) | tool format | tags | duskmoon314/tree-sitter-thrift (MIT (verify at pin)) | L3 | unsupported (disclosed) | — | — | — | LB-Q-thrift |
| PureScript (.purs) | language | tags | postsolar/tree-sitter-purescript (MIT (verify at pin)) | L2 | unsupported (disclosed) | — | — | — | LB-Q-purescript |
| Avro schemas (.avsc) and IDL (.avdl) | tool format | doc | pure-go:encoding/json (n/a) | L2 | unsupported (disclosed) | — | — | — | LB-Q-avro |
| Emacs Lisp (.el) | language | tags | Wilfred/tree-sitter-elisp (MIT) | L1 | L1 (L1 good) | B universal-ctags 6.2.1 | syntax | 2026-09-29 | at target (LB-Q-emacs-lisp) |
| AsyncAPI documents | tool format | doc | pure-go:gopkg.in/yaml.v3 (n/a) | L3 | unsupported (disclosed) | — | — | — | LB-Q-asyncapi |
| Vim script (.vim, vimrc) | language | tags | tree-sitter-grammars/tree-sitter-vim (MIT) | L1 | refused; measured L1 (L1 excellent) | B universal-ctags 6.2.1 | — | 2026-09-29 | re-plan (LB-Q-vimscript-rest refused) |
| Prisma schema (schema.prisma) | tool format | tags | victorhqc/tree-sitter-prisma (MIT (verify at pin)) | L3 | unsupported (disclosed) | — | — | — | LB-Q-prisma |
| Tcl (.tcl) | language | tags | tree-sitter-grammars/tree-sitter-tcl (MIT) | L1 | refused; measured none (measured) | B universal-ctags 6.2.1 | — | 2026-09-29 | re-plan (LB-Q-tcl-rest refused) |
| dbt projects (dbt_project.yml, models/*.sql with ref(), schema.yml) | tool format | tags+doc | DerekStride/tree-sitter-sql + pure-go:gopkg.in/yaml.v3 (MIT) | L3 | unsupported (disclosed) | — | — | — | LB-Q-dbt |
| Migration changelogs (Flyway V__*.sql naming, Liquibase changelogs) | tool format | doc | pure-go:gopkg.in/yaml.v3 + encoding/xml (n/a) | L2 | unsupported (disclosed) | — | — | — | LB-Q-sql-migrations |
| Mojo (.mojo, .🔥) | language | tags | none consolidated (several ~3-star community grammars) (verify at pin) | L1 | unsupported (disclosed) | — | — | — | LB-Q-mojo |
| Buf workspace and generation configs (buf.yaml, buf.gen.yaml, buf.work.yaml) | tool format | doc | pure-go:gopkg.in/yaml.v3 (n/a) | L1 | unsupported (disclosed) | — | — | — | LB-Q-buf |
| Odin (.odin) | language | tags | tree-sitter-grammars/tree-sitter-odin (MIT) | L1 | unsupported (disclosed) | — | — | — | LB-Q-odin |
| Ruby DSL manifests (Gemfile, *.gemspec, Podfile, *.podspec) | tool format | tags | tree-sitter/tree-sitter-ruby (MIT) | L2 | unsupported (disclosed) | — | — | — | LB-Q-gemfile-podfile |
| LaTeX (.tex .sty .cls by sniff) | language | tags | latex-lsp/tree-sitter-latex (MIT) | L2 | unsupported (disclosed) | — | — | — | LB-Q-latex |
| Android manifests (AndroidManifest.xml) | tool format | doc | pure-go:encoding/xml (n/a) | L2 | unsupported (disclosed) | — | — | — | LB-Q-android-manifest |
| Typst (.typ) | language | tags | uben0/tree-sitter-typst (MIT (verify at pin)) | L1 | unsupported (disclosed) | — | — | — | LB-Q-typst |
| CODEOWNERS (.github/CODEOWNERS, docs/CODEOWNERS) | tool format | doc | pure-go:line parser (n/a) | L2 | unsupported (disclosed) | — | — | — | LB-Q-codeowners |
| Laravel Blade templates (.blade.php) | language | tags | EmranMR/tree-sitter-blade (MIT (verify at pin)) | L3 | unsupported (disclosed) | — | — | — | LB-Q-blade |
| Jinja2 templates (.j2 .jinja .jinja2, templates/*.html by sniff) | language | tags | a Jinja tree-sitter grammar (verify at pin, licence allowlist) (verify at pin) | L3 | unsupported (disclosed) | — | — | — | LB-Q-jinja2 |
| Serverless Framework (serverless.yml) | tool format | doc | pure-go:gopkg.in/yaml.v3 (n/a) | L2 | L2 (L1 excellent, L2 excellent) | A serverless print 3.40.0 | syntax | 2026-10-09 | at target (LB-Q-serverless) |
| Conan recipes and lockfiles (conanfile.txt, conanfile.py, conan.lock, conandata.yml) | tool format | tags+doc | tree-sitter/tree-sitter-python + pure-go:INI, encoding/json, gopkg.in/yaml.v3 (verify at pin) | L3 | L2 (L1 excellent, L2 excellent) | A conan graph info --format json -nr (macOS, Linux, Windows and mingw host+build profiles; the versions of a recipe folder at default options; each option at each of its other values, and every boolean all True and all False, at the newest and oldest version) Conan version 2.33.0 | syntax | 2026-10-02 | LB-Q-conan |
| vcpkg manifests (vcpkg.json, vcpkg-configuration.json) | tool format | doc | pure-go:encoding/json (n/a) | L3 | L3 (L1 excellent, L2 excellent, L3 excellent) | A vcpkg x-package-info --x-json vcpkg package management program version 2026-09-26-Homebrew; A vcpkg x-package-info --x-json (dependencies joined by name to the attested manifests of the subject) vcpkg package management program version 2026-09-26-Homebrew | syntax | 2026-10-02 | at target (LB-Q-vcpkg) |
| Meson builds (meson.build, meson.options, meson_options.txt) | tool format | tags | tree-sitter-grammars/tree-sitter-meson (MIT) | L3 | L2 (L1 excellent, L2 excellent) | A meson mparser + introspect 1.12.1 | syntax | 2026-10-01 | LB-Q-meson |
| Clang compilation databases (compile_commands.json) | tool format | doc | pure-go:encoding/json (n/a) | L2 | L2 (L1 excellent, L2 excellent) | A libclang CompilationDatabase libclang 18.1.1 (pip) | syntax | 2026-10-01 | at target (LB-Q-compile-commands) |
| Azure Bicep (.bicep, .bicepparam) | tool format | tags | tree-sitter-grammars/tree-sitter-bicep v1.1.0 (MIT) | L3 | L3 (L1 excellent, L2 excellent, L3 excellent) | A bicep language server and CLI 0.47.16 | file | 2026-10-02 | at target (LB-Q-bicep) |
| Azure Resource Manager templates (azuredeploy.json shape, by content) | tool format | doc | pure-go:encoding/json (n/a) | L3 | L3 (L1 excellent, L2 good, L3 excellent) | A bicep decompile 0.47.16 | — | 2026-10-02 | at target (LB-Q-arm-templates) |
| AWS CDK apps (cdk.json; programs ride their language) | tool format | doc | pure-go:encoding/json (n/a) | L2 | L2 (L1 excellent, L2 good) | C cdk app command reference parse 2.1144.0; A cdk context -vv 2.1144.0 | syntax | 2026-10-02 | at target (LB-Q-aws-cdk) |
| Flux CD resources (Kustomization, HelmRelease, GitRepository, OCIRepository) | tool format | doc | pure-go:gopkg.in/yaml.v3 (n/a) | L3 | L3 (L1 excellent, L2 good, L3 good) | A Flux API types, strict (kustomize-controller api v1.9.6, helm-controller api v1.6.5, source-controller api v1.9.6, notification-controller api v1.9.4) and flux build kustomization --dry-run; lines: gopkg.in/yaml.v3 flux: v2.9.6; C flux-oracle (independent gopkg.in/yaml.v3 walk) gopkg.in/yaml.v3 v3.0.1 | — | 2026-10-02 | at target (LB-Q-flux) |
| Timoni modules and bundles (timoni.cue, values.cue, templates/*.cue, bundle.cue, runtime.cue) | tool format | native | native:cuelang.org/go (Apache-2.0) | L3 | L1 (L1 good) | A timoni 0.35.0 (cue 0.17.1) timoni 0.35.0 (cue 0.17.1); C timoni oracle: a CUE syntax walk gated by timoni 0.35.0 (cue 0.17.1) timoni 0.35.0 (cue 0.17.1) | — | 2026-10-03 | LB-Q-timoni |
| Kyverno policies (ClusterPolicy, Policy, PolicyException) | tool format | doc | yaml.v3 (n/a) | L2 | L3 (L1 excellent, L2 excellent, L3 good) | A Kyverno's own loaders (github.com/kyverno/kyverno v1.18.1 cmd/cli/kubectl-kyverno policy.Load and test.LoadTest) + gopkg.in/yaml.v3 for lines kyverno v1.18.1; C kyverno-oracle policy walk (independent gopkg.in/yaml.v3) over the loader-attested policies gopkg.in/yaml.v3 v3.0.1 | — | 2026-10-02 | at target (LB-Q-kyverno) |
| OPA Gatekeeper ConstraintTemplates and constraints | tool format | doc | yaml.v3 (n/a) | L3 | L3 (L1 excellent, L2 excellent, L3 excellent) | A Gatekeeper's own match (github.com/open-policy-agent/gatekeeper/v3 v3.23.1 pkg/target K8sValidationTarget.ToMatcher + Match) over the dialect's object set, the dialect's nearest-first join gatekeeper/v3 v3.23.1; A gator verify (gator version v3.23.1 (Feature State: beta), opa/v1.17.1, frameworks/v0.0.0-20260616163050-e1eaa1bf6d62) + gopkg.in/yaml.v3 for names and lines gator version v3.23.1 (Feature State: beta), opa/v1.17.1, frameworks/v0.0.0-20260616163050-e1eaa1bf6d62 | — | 2026-10-02 | at target (LB-Q-gatekeeper) |
| Conftest policies (policy/*.rego, conftest.toml) | tool format | tags+doc | rides LB-Q-rego's grammar + pure-go:go-toml/v2 (verify at pin) | L2 | unsupported (disclosed) | — | — | — | LB-Q-conftest |
| Cedar policies and schemas (.cedar .cedarschema) | language | tags | chrnorm/tree-sitter-cedar 5092204d or cedar-go v1.8.0, both refused at pin (LB-Q-cedar-rest) (MIT (tree-sitter-cedar LICENCE); Apache-2.0 (cedar-go)) | L3 | refused | — | — | — | re-plan (LB-Q-cedar-rest refused) |
| AWS IAM policy documents (JSON by content, inline in CloudFormation and Terraform) | tool format | doc | encoding/json (n/a) | L2 | L2 (L1 excellent, L2 excellent) | A aws-cdk-lib aws_iam PolicyDocument.fromJson and PolicyStatement.fromJson (AWS) + jsonc-parser for lines aws-cdk-lib 2.272.0, jsonc-parser 3.3.1 | — | 2026-10-02 | at target (LB-Q-aws-iam-policy) |
| Azure Policy definitions and initiatives (policyRule JSON) | tool format | doc | encoding/json (n/a) | L2 | L2 (L1 good, L2 good) | C azure-policy JSON walk (the definition schema's structure, an independent JSON reader) 1 | — | 2026-10-02 | at target (LB-Q-azure-policy) |
| Cloud Custodian policies (policies YAML) | tool format | doc | yaml.v3 (n/a) | L2 | L2 (L1 excellent, L2 excellent) | A c7n (custodian validate's loader) 0.9.53 | — | 2026-10-02 | at target (LB-Q-cloud-custodian) |
| Checkov custom policies (YAML graph checks, Python checks) | tool format | tags+doc | yaml.v3 + tree-sitter/tree-sitter-python (n/a) | L2 | unsupported (disclosed) | — | — | — | LB-Q-checkov-policies |
| HashiCorp Sentinel policies (.sentinel) | language | tags | none open (verify at pin) (verify at pin) | L2 | unsupported (disclosed) | — | — | — | LB-Q-sentinel |
| PgBouncer configuration (pgbouncer.ini) | tool format | doc | pure-go:INI parser (n/a) | L2 | unsupported (disclosed) | — | — | — | LB-Q-pgbouncer |
| ProxySQL configuration (proxysql.cnf) | tool format | doc | pure-go:libconfig parser (verify at pin) (n/a) | L2 | unsupported (disclosed) | — | — | — | LB-Q-proxysql |
| fathom configuration (.fathom/config.toml) | tool format | doc | pure-go:go-toml/v2 (n/a) | L2 | L2 (L1 good, L2 good) | C python tomllib (TOML decode) + a line scanner of its own python 3.14.7 tomllib | — | 2026-10-03 | at target (LB-Q-fathom-config) |
| fathom concerns (.fathom/concerns.cue) | tool format | native | native:cuelang.org/go (n/a) | L2 | unsupported (disclosed) | — | — | — | LB-Q-fathom-concerns |
| fathom layer packs (.fathom/layers/*/layer.cue, .fathom/rules/*.cue) | tool format | native | native:cuelang.org/go (n/a) | L2 | unsupported (disclosed) | — | — | — | LB-Q-fathom-layer-packs |
| fathom plan grammar v2 rows (docs/plans/*.md) | tool format | native | pure-go:plan row grammar (n/a) | L3 | L3 (L1 good, L2 good, L3 good) | C plan_rows_oracle.py: a line parser of docs/plans/README.md's row grammar python 3.14.7 | — | 2026-10-03 | at target (LB-Q-fathom-plan-rows) |
