# Languages and dialects — what fathom extracts, and how far

A dialect is an extractor: the knowledge of how to read a language or tool format
(`docs/design/pack-taxonomy.md`). This table is the tracker ADR-041 names: extraction
approach, whether a tree-sitter grammar is in the tree, where typed edges come from, and
what is left. Typed-edge order is the owner's (2026-09-19): Go, CUE, TypeScript,
Python, then the delegate's. `TestLanguagesTrackerMatchesRegistry` (SU-13) keeps the
rows honest against the extractor registry.

**The breadth program (ADR-048, vision 37).** Support levels are L1 symbols, L2 syntax call
edges, L3 imports and file-resolved edges, L4 typed edges, each claimed only when a committed
reading under `docs/bench/readings/lang/` proves it against an independent oracle (ctags or
SCIP; design `docs/design/language-breadth.md` §1). The "Excellence board" and "Support levels" sections at the end of
this file are **generated** (LB-F17, `go run ./cmd/indexbench lang-tracker > docs/languages.md`)
from the queue (`docs/plans/language-breadth.queue.json`; the excellence board's totals count
its languages and tool formats), the registered dialects and the readings; `TestLanguagesTrackerIsGenerated` fails when this
file differs from the rendering and when a row claims a level above its reading (I54). The
typed-edge table below is the hand-kept template part, held true against the registry by
`TestLanguagesTrackerMatchesRegistry`; the "heuristic" label on Java, Scala, Rust, C, C++, C#
and PHP means hand-written Go walkers over CGO tree-sitter — real ASTs, never measured
against an oracle until their queue items land.

Typed-edge levels: **native** (fathom resolves types itself), **scip** (imported from a
language-native SCIP indexer), **file** (per-file resolver, tier pre-scan, labelled
`file-resolved`), **syntax** (call names by syntax only).

| Language / dialect | Extraction | Grammar in tree | Calibration subjects | Typed edges | Next |
|---|---|---|---|---|---|
| Go | `go/parser` + `go/types` | n/a | cobra, cli/cli, prometheus | **native** (done; selector-method and interface gaps O39/O40 open) | close O39/O40 |
| CUE | `cueextract.go` + `cue/load` | n/a | this repo's `.fathom/`, `schema/` | **native** (SU-10) | — |
| TypeScript | tree-sitter (wasm) | yes | nest, zod, typeorm | **scip** — `fathom ingest --scip` imports scip-typescript's own output (SU-9, `internal/scip`; the L0 oracle SU-12 is also done); against the real, only-published scip-typescript@0.4.0 it emits zero `DisplayName`s, which broke the import until O115's fix (2026-09-24): `ImportIndex` now maps each definition to fathom's own symbol table by position instead, measured on zod at 82.0% of raw SCIP definitions unmapped (no fathom-level counterpart — SCIP's finer granularity, expected) and the rest correctly keyed; **the file-resolved tier shipped (SU-11, `internal/fusion/fileresolve.go`) remains the fallback** when no scip index is imported, now with a real precision/recall read against the fixed scip truth: 0.87 precision / 0.03 recall (CV-M7, `docs/bench/readings/l0-ts-py-2026-09-24.md`) | CV-M7 done; a `PutTypedEdges` replace-granularity gap the re-run surfaced (a package with zero NEW edges keeps its stale old rows) is unfixed, its own thread; lookups corpus (SU-23) |
| JavaScript (`.js .jsx .mjs .cjs`) | tree-sitter (wasm) | yes | express, axios, lodash | **scip** — same `--scip` import mechanism as TypeScript (SU-9, SU-12 done), same O115 fix (2026-09-24: position-based mapping, fixed); **the file-resolved tier shipped (SU-11) remains the fallback** when no scip index is imported | — |
| Python | tree-sitter (wasm) | yes | fastapi, flask, django | **scip** — `fathom ingest --scip` imports scip-python's own output (SU-9, SU-12 done); against the real, only-published scip-python@0.6.6 it emits zero `DisplayName`s, which broke the import until O115's fix (2026-09-24: position-based mapping), measured on flask at 65.5% of raw SCIP definitions unmapped (expected — SCIP's finer granularity) and the rest correctly keyed; **the file-resolved tier shipped (SU-11) remains the fallback** when no scip index is imported, now with a real precision/recall read against the fixed scip truth: 0.95 precision / 0.12 recall (CV-M7, `docs/bench/readings/l0-ts-py-2026-09-24.md`) | CV-M7 done; lookups corpus (SU-23) |
| Java | heuristic (`javaextract.go`) | no | guava, retrofit | **scip** via scip-java (SU-9) | grammar; oracle |
| Kotlin | tree-sitter (wasm, fwcd/tree-sitter-kotlin commit 1852ea17; the 0.3.8 tag left 16.6% of the subject files in error) | yes | okhttp, Exposed | **scip** via scip-java (SU-9), once its Kotlin plugin writes occurrence ranges under K2 (LB-Q-kotlin-rest) | a SCIP index with ranges, to measure `import_resolution` |
| Scala | heuristic (`scalaextract.go`) | no | cats, zio, http4s | **scip** via scip-java | grammar |
| Rust | heuristic (`rustextract.go`) dispatched; a tree-sitter dialect (`dialects/rust`) is built and measured at L2 excellent, not yet dispatched | yes (tree-sitter-rust v0.24.2) | ripgrep, clap | **scip** via rust-analyzer | L3/L4 (LB-Q-rust-rest), then the dispatch switch (an `ExtractorEpoch` bump) |
| C | heuristic (`cextract.go`, `cheaderextract.go`) | no | curl, libuv, sqlite | **scip** via scip-clang | grammar |
| C++ | heuristic (`cppextract.go`) dispatched; a tree-sitter dialect (`dialects/cpp`) is built and measured at L1 excellent, not yet dispatched | yes (tree-sitter-cpp v0.23.4) | fmt, nlohmann/json | **scip** via scip-clang | L2 call precision, L3 file edges (`import_resolution` reads 1 against the build's own preprocessor, tier A) and L4 (LB-Q-cpp-rest), then the dispatch switch (an `ExtractorEpoch` bump) |
| Ruby | tree-sitter (wasm) | yes | sinatra, jekyll | **scip** via scip-ruby, which needs a Sorbet-typed subject (on untyped code it sees only typed receivers); meanwhile **file**: `require`, `require_relative`, `load` and `autoload` resolve via the file-symbol strategy, and a constant is looked up lexically through its enclosing modules (LB-Q-ruby at L3 good, LB-F76) | L3 excellent and L4 need a Sorbet-typed subject for scip-ruby (tier B and C oracles grade good at most; LB-Q-ruby-rest) |
| HTML | tree-sitter (wasm) | yes | html5-boilerplate, bootstrap (visual tests) | **file** — `src`/`href` resolve via the relative-path strategy (LB-F11); no SCIP producer exists for HTML, so this is the ceiling until one does | — |
| CSS | tree-sitter (wasm) | yes | normalize.css, pico | **file** — `@import` resolves via the relative-path strategy (LB-F11); no SCIP producer exists for CSS, so this is the ceiling until one does | BareName's separator set mismatches CSS's own `.`/`#`/`:` selector-marker punctuation (internal/langbench/score.go, Opus); tree-sitter-css > v0.25.0 for `:not(A, B)` |
| C# | heuristic (`csharpextract.go`) | no | (corpus.yaml) | **scip** via scip-dotnet | grammar |
| PHP | heuristic (`phpextract.go`) dispatched; a tree-sitter dialect (`dialects/php`) is built and measured at L1 excellent, not yet dispatched | yes (tree-sitter-php v0.24.2) | guzzle, monolog | **syntax** (no mature exporter) | L2 call precision against scip-php, L3/L4 (LB-Q-php-rest), then the dispatch switch (an `ExtractorEpoch` bump) |
| Swift | **none** — refused (LB-Q-swift-rest, design §14.2): a tree-sitter dialect (`dialects/swift`) is built and measured (L1 excellent, L2 good against the compiler's parser), but 5.6% of its calibration files parse with an error under the pinned grammar, over the 5% line, so it is not dispatched and Swift stays disclosed as unsupported, named per service by `list_services` | yes (alex-pinkus/tree-sitter-swift commit 35245fb, not dispatched) | Alamofire, swift-nio | **none** | a tree-sitter-swift that parses Swift 6 syntax under the 5% line, then re-measure and re-plan (L3 needs a Swift import oracle for `import_resolution` and a typed oracle) |
| Lua | tree-sitter (wasm) | yes | Kong, telescope.nvim | **syntax** — call names by syntax only (a name, a field or a method callee); no SCIP producer exists for Lua | the `require` graph (L3, a file-symbol strategy); a function in a computed slot has no name |
| Bash | heuristic (`bashextract.go`) | no | (corpus.yaml) | **syntax** | — |
| PowerShell | tree-sitter (wasm) | yes | PSScriptAnalyzer, Pester | **syntax** — call names by syntax only (a command name or a member invocation); no SCIP producer exists for PowerShell | class properties and script/global variables (a `$`-sigilled token the tags engine cannot strip to the bare name) |
| Dart | tree-sitter (wasm) | yes | bloc, http (eight pure-Dart packages) | **scip** via scip-dart, which indexes one package at a time (LB-Q-dart: def_mapping .9995, typed_recall .8464 once a reference to a parameter or a local variable, which maps to no symbol, leaves its denominator (LB-F55), .0036 under the .85 bar); meanwhile **file**: `import` resolves through `pubspec.yaml`'s package name and relative paths (namespace-dir) | file_edge_recall .50 for L3 excellent (barrel exports and type references are followed since LB-F74/LB-F75; L3 good); attributing an edge from a parameter or a type parameter to its enclosing definition (L4) |
| Ada | tree-sitter (wasm) | yes | gnatcoll-core, alire | **syntax** — call names by syntax only (a procedure call statement or a function call, named by the callee's last identifier); no SCIP producer exists for Ada | L1 is measured at good (universal-ctags writes no Ada end line and loses sync after a `declare` block, so `range_agreement` is not measured); the `with` graph (L3, a strategy over child-unit paths) |
| VHDL | tree-sitter (wasm) | yes | neorv32, VUnit | **syntax** — call names by syntax only; the grammar cannot tell a function call from an index, so both are recorded; no SCIP producer exists for VHDL | ports, signals and generics are not emitted; the `library`/`use` graph (L3) |
| Makefile | tree-sitter (wasm) | yes | prometheus, cosmopolitan, valkey | **syntax** — a rule's literal prerequisite words by syntax only; no SCIP producer exists for make, and a prerequisite spelled with a variable is a value the dialect does not evaluate | `include` edges (L3), tab-indented assignments inside conditionals and unspaced `VAR?=x` (tree-sitter-make limits) |
| Gradle builds (Groovy DSL: `*.gradle`) | tree-sitter (wasm, murtaza64/tree-sitter-groovy commit 2a6ddd55) | yes | spring-boot, kafka | **syntax** — a task's `dependsOn`/`finalizedBy` names by syntax only; an include or a project dependency resolves to the named project's build script only when that script declares a single top-level symbol (LB-Q-gradle-rest); no SCIP producer exists for Gradle | project edges (L3); the Kotlin DSL (`*.gradle.kts` stays Kotlin's) |
| Emacs Lisp | tree-sitter (wasm; the grammar is generated at build time, its repository commits no `parser.c`) | yes | magit, lsp-mode | **syntax** — no call is captured yet (a head symbol is a call, a `let` binding pair, a `dolist` spec and a `cond` clause in the same syntax); no SCIP producer exists for Emacs Lisp | call edges (L2), which must tell a call from a binding form; `transient-define-*`, `defvar-keymap`, `define-error` and `ert-deftest` (ctags has no kind for them) |
| Common Lisp | tree-sitter (wasm) | yes | lem, clack | **syntax** — no call is captured yet (a head symbol is a call, a `let` binding pair, a `cond` clause and a `loop` clause in the same syntax); no SCIP producer exists for Common Lisp | call edges (L2), which must tell a call from a binding form; `define-condition`, `define-command` and a `(:method ...)` option of a `defgeneric`; the grammar reads a `loop :with` list of four or more variables and some `format` control strings (`~/`, `~{~A~^ ~}`, `~:*`) as errors, which leaves 4 of the 232 calibration files (1.7%) with a parse error |
| SQL | heuristic (`sqlextract.go`) | no | — | **syntax** | — |
| Terraform/HCL | pure Go (`hclextract.go`) | n/a | (corpus.yaml) | **syntax** (resource references) | — |
| Kubernetes YAML, Helm, Kustomize, docker-compose | pure Go | n/a | (corpus.yaml) | **syntax** (`service_ref` cross-repo edges) | — |
| Dockerfile, CMake, GitHub Actions, proto, GraphQL, OpenAPI, JSON, TOML, templ, Markdown, go.mod, go.work, package.json, pyproject.toml, pom.xml, Cargo.toml, .csproj, .fsproj, .vbproj, .props, .targets, Taskfile.yml, lefthook.yml, .pre-commit-config.yaml, CloudFormation and SAM templates (by content) | pure Go | n/a | (corpus.yaml where listed) | **syntax** | — |
| nginx, HAProxy and Caddyfile configuration (nginx.conf, haproxy.cfg, Caddyfile, .caddyfile; a .conf and a .cfg by content, sites-enabled/* and Caddyfile.* by glob) | pure Go (native dialects, `confsyntax.go`) | n/a | (calibration/corpus.d) | **syntax** — includes and imports are file-tier doc-xref edges; a proxy's upstream is a `route_ref` route target, resolved to a deploy key by LB-F96 | include and import globs (fusion.ResolveDocXrefs matches a glob against directories); a `.conf` whose first KiB is only comments |
| Rego / OPA policy (.rego) | pure Go (native dialect, `regoextract.go`; no grammar reads Rego under the 5% line, ADR-048 wave-pc amendment) | n/a | regal, opa | **file** — `import data.x.y` resolves by the rego-package strategy to the files declaring the package, and a call or reference through the import is a file-resolved edge to the file defining the rule (LB-Q-rego at L3 good, LB-F132) | L3 excellent: an import names every file of a package split across files, an edge only the one defining what is used (LB-Q-rego-rest); no Rego SCIP producer (L4) |

Disclosed as unsupported (named per service in `list_services`, never silently skipped; `internal/connector/unsupported_language.go`): Clojure, Crystal, Elixir, Elm, Erlang, F#, Groovy (refused, LB-Q-groovy-rest), Haskell, Julia, Nim, OCaml, Swift (refused, LB-Q-swift-rest), Zig.

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
| T1 | language | 20 | 7 | 5 | 4 | 0 | 1 | 1 | 1 | 1 | 3 | 0 |
| T2 | tool format | 23 | 0 | 2 | 9 | 0 | 0 | 0 | 2 | 10 | 11 | 5 |
| T3 | tool format | 15 | 0 | 1 | 3 | 0 | 0 | 0 | 0 | 11 | 3 | 1 |
| T4 | language | 3 | 0 | 0 | 1 | 0 | 0 | 1 | 0 | 1 | 1 | 0 |
| T4 | tool format | 7 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 7 | 0 | 0 |
| T5 | tool format | 8 | 0 | 5 | 1 | 0 | 0 | 0 | 0 | 2 | 1 | 0 |
| T6 | language | 1 | 0 | 0 | 0 | 0 | 0 | 0 | 1 | 0 | 0 | 0 |
| T6 | tool format | 4 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 4 | 0 | 0 |
| **Total** | language | 24 | 7 | 5 | 5 | 0 | 1 | 2 | 2 | 2 | 4 | 0 |
| **Total** | tool format | 57 | 0 | 8 | 13 | 0 | 0 | 0 | 2 | 34 | 15 | 6 |

### T1

| Item | Kind | Target | Level met | Next gap |
|---|---|---|---|---|
| TypeScript (.ts .tsx .mts .cts) | language | L4 | L1 (L1 excellent) | L2 not met: call_precision 0.5186 < 0.85 |
| JavaScript (.js .jsx .mjs .cjs) | language | L4 | L1 (L1 excellent) | L2 not met: call_precision 0.1919 < 0.85 |
| Python (.py .pyi) | language | L4 | L3 (L1 excellent, L2 good, L3 excellent) | L4 not met: def_mapping 0.7619 < 0.85 |
| HTML (.html .htm) | language | L3 | L1 (L1 good) | L2 not met: call_recall not measured |
| CSS (.css) | language | L3 | none (measured) | L1 not met: sym_recall 0.1423 < 0.85 |
| SQL (.sql; schema and migrations) | language | L2 | L2 (L1 good, L2 good) | L1 good: range_agreement not measured |
| Shell / Bash (.sh .bash, shebangs) | language | L2 | L2 (L1 good, L2 good) | L1 good: range_agreement not measured |
| Java (.java) | language | L4 | L3 (L1 excellent, L2 good, L3 excellent) | L4 not met: typed_recall 0.669 < 0.85 |
| C# (.cs) | language | L4 | L1 (L1 excellent) | L2 not met: call_precision 0.7776 < 0.85 |
| C (.c, .h by sniff) | language | L4 | L2 (L1 good, L2 excellent) | L3 not met: file_edge_recall 0.0678 < 0.25 |
| C++ (.cc .cpp .cxx .hpp .hh .hxx, .h by sniff) | language | L4 | L1 (L1 excellent) | L2 not met: call_precision 0.3208 < 0.85 |
| Go (.go) | language | L4 | L1 (L1 excellent) | L2 not met: call_precision 0.8206 < 0.85 |
| PHP (.php) | language | L4 | L1 (L1 excellent) | L2 not met: call_precision 0.6924 < 0.85 |
| Rust (.rs) | language | L4 | L2 (L1 excellent, L2 excellent) | L3 not met: import_resolution 0.4732 < 0.85 |
| Kotlin (.kt .kts) | language | L4 | L2 (L1 excellent, L2 good) | L3 not met: import_resolution not measured |
| Ruby (.rb, Rakefile, Gemfile by basename) | language | L3 | L3 (L1 excellent, L2 good, L3 good) | L2 good: call_recall 0.8688 < 0.9 |
| Dart (.dart) | language | L4 | L3 (L1 excellent, L2 excellent, L3 good) | L4 not met: typed_recall 0.8464 < 0.85 |
| Swift (.swift) | language | L3 | refused | re-plan (LB-Q-swift-rest refused) |
| SCSS / Sass (.scss .sass) | language | L3 | unsupported | no dialect (LB-Q-scss) |
| Scala (.scala .sc) | language | L4 | tolerated | no reading (LB-Q-scala) |

### T2

| Item | Kind | Target | Level met | Next gap |
|---|---|---|---|---|
| GitHub Actions workflows and composite actions | tool format | L3 | L3 (L1 good, L2 good, L3 good) | L1 good: oracle tier C caps at good |
| npm/pnpm/yarn package manifests (package.json, workspaces) | tool format | L3 | L3 (L1 excellent, L2 excellent, L3 good) | L3 good: oracle tier C caps at good |
| Python packaging (pyproject.toml, requirements*.txt, setup.cfg) | tool format | L2 | L2 (L1 good, L2 good) | L1 good: oracle tier C caps at good |
| Makefile / GNU make (Makefile, *.mk, GNUmakefile) | tool format | L2 | L2 (L1 excellent, L2 excellent) | excellent at target |
| GitLab CI (.gitlab-ci.yml, included files) | tool format | L3 | L3 (L1 excellent, L2 good, L3 good) | L2 good: oracle tier C caps at good |
| Maven POMs (pom.xml) | tool format | L3 | L3 (L1 excellent, L2 good, L3 good) | L2 good: oracle tier C caps at good |
| Cargo manifests (Cargo.toml, workspaces) | tool format | L3 | L3 (L1 excellent, L2 excellent, L3 excellent) | excellent at target |
| Go modules (go.mod, go.work) | tool format | L3 | L3 (L1 excellent, L2 excellent, L3 excellent) | excellent at target |
| MSBuild projects (.csproj .fsproj .vbproj .props .targets, .sln) | tool format | L3 | L3 (L1 excellent, L2 excellent, L3 excellent) | excellent at target |
| Composer manifests (composer.json) | tool format | L2 | unsupported | no dialect (LB-Q-composer) |
| Bazel / Starlark (BUILD, BUILD.bazel, *.bzl, WORKSPACE, MODULE.bazel) | tool format | L3 | unsupported | no dialect (LB-Q-bazel) |
| CMake (CMakeLists.txt, *.cmake) | tool format | L3 | tolerated | no reading (LB-Q-cmake) |
| Protocol Buffers (.proto) | tool format | L3 | L3 (L1 excellent, L2 excellent, L3 excellent) | excellent at target |
| GraphQL schemas and operations (.graphql .gql) | tool format | L3 | tolerated | no reading (LB-Q-graphql) |
| JSON Schema documents | tool format | L3 | unsupported | no dialect (LB-Q-json-schema) |
| Taskfile (go-task: Taskfile.yml) | tool format | L3 | L3 (L1 excellent, L2 good, L3 excellent) | L2 good: oracle tier C caps at good |
| Jenkins pipelines (Jenkinsfile, *.jenkinsfile) | tool format | L2 | unsupported | no dialect (LB-Q-jenkinsfile) |
| Azure Pipelines (azure-pipelines.yml, templates) | tool format | L3 | unsupported | no dialect (LB-Q-azure-pipelines) |
| CircleCI (.circleci/config.yml) | tool format | L2 | unsupported | no dialect (LB-Q-circleci) |
| Conan recipes and lockfiles (conanfile.txt, conanfile.py, conan.lock, conandata.yml) | tool format | L3 | unsupported | no dialect (LB-Q-conan) |
| vcpkg manifests (vcpkg.json, vcpkg-configuration.json) | tool format | L3 | unsupported | no dialect (LB-Q-vcpkg) |
| Meson builds (meson.build, meson.options, meson_options.txt) | tool format | L3 | unsupported | no dialect (LB-Q-meson) |
| Clang compilation databases (compile_commands.json) | tool format | L2 | unsupported | no dialect (LB-Q-compile-commands) |

### T3

| Item | Kind | Target | Level met | Next gap |
|---|---|---|---|---|
| Kubernetes manifests (incl. CRDs) | tool format | L3 | L3 (L1 good, L2 good, L3 good) | L1 good: oracle tier C caps at good |
| Helm charts (Chart.yaml, values.yaml, templates/*.yaml, *.tpl) | tool format | L3 | L3 (L1 good, L2 good, L3 good) | L1 good: oracle tier C caps at good |
| Kustomize (kustomization.yaml) | tool format | L3 | L3 (L1 excellent, L2 excellent, L3 excellent) | excellent at target |
| Argo CD Applications and Argo Workflows | tool format | L3 | unsupported | no dialect (LB-Q-argo) |
| Ansible playbooks, roles and inventories | tool format | L3 | unsupported | no dialect (LB-Q-ansible) |
| AWS CloudFormation / SAM templates | tool format | L3 | L2 (L1 excellent, L2 excellent) | L3 not met: xref_precision not measured |
| Terragrunt (terragrunt.hcl, root.hcl) | tool format | L3 | unsupported | no dialect (LB-Q-terragrunt) |
| HashiCorp HCL tool configs (Packer *.pkr.hcl, Nomad jobspecs, Vault/Consul configs, docker-bake.hcl) | tool format | L2 | unsupported | no dialect (LB-Q-hcl-tools) |
| Pulumi YAML programs and project files (Pulumi.yaml, Pulumi.<stack>.yaml) | tool format | L2 | unsupported | no dialect (LB-Q-pulumi-yaml) |
| Crossplane compositions and XRDs | tool format | L3 | unsupported | no dialect (LB-Q-crossplane) |
| Serverless Framework (serverless.yml) | tool format | L2 | unsupported | no dialect (LB-Q-serverless) |
| Azure Bicep (.bicep, .bicepparam) | tool format | L3 | unsupported | no dialect (LB-Q-bicep) |
| Azure Resource Manager templates (azuredeploy.json shape, by content) | tool format | L3 | unsupported | no dialect (LB-Q-arm-templates) |
| AWS CDK apps (cdk.json; programs ride their language) | tool format | L2 | unsupported | no dialect (LB-Q-aws-cdk) |
| Flux CD resources (Kustomization, HelmRelease, GitRepository, OCIRepository) | tool format | L3 | unsupported | no dialect (LB-Q-flux) |

### T4

| Item | Kind | Target | Level met | Next gap |
|---|---|---|---|---|
| Rego / OPA policy (.rego) | language | L3 | L3 (L1 excellent, L2 excellent, L3 good) | L3 good: file_edge_recall 0.3858 < 0.5 |
| Kyverno policies (ClusterPolicy, Policy, PolicyException) | tool format | L2 | unsupported | no dialect (LB-Q-kyverno) |
| OPA Gatekeeper ConstraintTemplates and constraints | tool format | L3 | unsupported | no dialect (LB-Q-gatekeeper) |
| Conftest policies (policy/*.rego, conftest.toml) | tool format | L2 | unsupported | no dialect (LB-Q-conftest) |
| Cedar policies and schemas (.cedar .cedarschema) | language | L3 | refused | re-plan (LB-Q-cedar-rest refused) |
| AWS IAM policy documents (JSON by content, inline in CloudFormation and Terraform) | tool format | L2 | unsupported | no dialect (LB-Q-aws-iam-policy) |
| Azure Policy definitions and initiatives (policyRule JSON) | tool format | L2 | unsupported | no dialect (LB-Q-azure-policy) |
| Cloud Custodian policies (policies YAML) | tool format | L2 | unsupported | no dialect (LB-Q-cloud-custodian) |
| Checkov custom policies (YAML graph checks, Python checks) | tool format | L2 | unsupported | no dialect (LB-Q-checkov-policies) |
| HashiCorp Sentinel policies (.sentinel) | language | L2 | unsupported | no dialect (LB-Q-sentinel) |

### T5

| Item | Kind | Target | Level met | Next gap |
|---|---|---|---|---|
| nginx configuration (nginx.conf, .conf by sniff, sites-enabled/* by glob) | tool format | L3 | L3 (L1 good, L2 good, L3 good) | L1 good: oracle tier C caps at good |
| HAProxy configuration (haproxy.cfg, .cfg by sniff) | tool format | L3 | L2 (L1 excellent, L2 excellent) | L3 not met: xref_precision not measured |
| Caddyfile (Caddyfile, Caddyfile.* by glob, .caddyfile) | tool format | L3 | L2 (L1 excellent, L2 excellent) | L3 not met: xref_precision not measured |
| Envoy proxy configuration (bootstrap, xDS resource files) | tool format | L3 | L2 (L1 excellent, L2 excellent) | L3 not met: xref_precision not measured |
| Kong declarative configuration (decK, DB-less) | tool format | L3 | L2 (L1 excellent, L2 good) | L3 not met: xref_precision not measured |
| Traefik static and file-provider configuration | tool format | L3 | L2 (L1 excellent, L2 good) | L3 not met: xref_precision not measured |
| PgBouncer configuration (pgbouncer.ini) | tool format | L2 | unsupported | no dialect (LB-Q-pgbouncer) |
| ProxySQL configuration (proxysql.cnf) | tool format | L2 | unsupported | no dialect (LB-Q-proxysql) |

### T6

| Item | Kind | Target | Level met | Next gap |
|---|---|---|---|---|
| CUE (.cue) | language | L4 | tolerated | no reading (LB-Q-cue) |
| fathom configuration (.fathom/config.toml) | tool format | L2 | unsupported | no dialect (LB-Q-fathom-config) |
| fathom concerns (.fathom/concerns.cue) | tool format | L2 | unsupported | no dialect (LB-Q-fathom-concerns) |
| fathom layer packs (.fathom/layers/*/layer.cue, .fathom/rules/*.cue) | tool format | L2 | unsupported | no dialect (LB-Q-fathom-layer-packs) |
| fathom plan grammar v2 rows (docs/plans/*.md) | tool format | L3 | unsupported | no dialect (LB-Q-fathom-plan-rows) |

Untiered entries (outside the excellence target set): 89 (abap, ada, android-manifest, apex, assembly, astro, asyncapi, avro, blade, buf, clojure, cobol, codeowners, commonlisp, crystal, cuda, d, dbt, dependency-bots, devcontainer, dhall, docker-compose, dockerfile, dotenv, earthfile, elixir, elm, emacs-lisp, erlang, fortran, fsharp, gdscript, gemfile-podfile, git-hooks-config, gleam, glsl, gradle, groovy, haskell, hlsl, json-yaml-toml, jsonnet, julia, jupyter, justfile, kcl, latex, less, lua, luau, markdown, matlab, mojo, nim, nix, nushell, objective-c, ocaml, odin, openapi, pascal, perl, pkl, powershell, prisma, prolog, purescript, r, scheme-racket, skaffold, solidity, sql-migrations, svelte, systemd, tcl, tekton, templ, terraform, thrift, tiltfile, typst, vba, vbnet, verilog, vhdl, vimscript, vue, wgsl, zig).

## Support levels (generated)

Generated by `go run ./cmd/indexbench lang-tracker > docs/languages.md` from the queue,
the registered dialects and the committed readings; never hand-edit it. A level is the
reading's computed `level_met` with each met level's grade; `tolerated` is registered with
no reading yet; `unsupported (disclosed)` is queued, not registered, and named per service
by `list_services`; `refused` is refused by its plan row (design §14.2): it claims no level,
is not dispatched and stays disclosed, whatever its reading measured.

Milestone M1 progress: languages at L1 excellent 16/20, at L2 12/12, at L3 5/8, at L4 0/6; tool formats at target 19/15

| Language or format | Kind | Engine | Grammar / parser | Target | Level met | Oracles | Typed edges | Reading | Next |
|---|---|---|---|---|---|---|---|---|---|
| TypeScript (.ts .tsx .mts .cts) | language | tags | tree-sitter/tree-sitter-typescript (MIT) | L4 | L1 (L1 excellent) | A scip-typescript 0.4.0 | scip:scip-typescript | 2026-10-01 | LB-Q-typescript |
| JavaScript (.js .jsx .mjs .cjs) | language | tags | tree-sitter/tree-sitter-javascript (MIT) | L4 | L1 (L1 excellent) | A scip-typescript 0.4.0 | scip:scip-typescript | 2026-10-01 | LB-Q-javascript |
| Dockerfile / Containerfile | tool format | tags | camdencheek/tree-sitter-dockerfile (Apache-2.0) | L3 | L2 (L1 excellent, L2 excellent) | A buildkit-dockerfile-parser github.com/moby/buildkit v0.33.0 frontend/dockerfile (docker buildx build --print=outline substitute — no Docker daemon in this sandbox) | syntax | 2026-09-28 | LB-Q-dockerfile |
| Python (.py .pyi) | language | tags | tree-sitter/tree-sitter-python (MIT) | L4 | L3 (L1 excellent, L2 good, L3 excellent) | A scip-python 0.6.6; B universal-ctags 6.2.1 | scip:scip-python | 2026-10-01 | LB-Q-python |
| HTML (.html .htm) | language | tags | tree-sitter/tree-sitter-html (MIT) | L3 | L1 (L1 good) | B universal-ctags 6.2.1 | file | 2026-09-28 | LB-Q-html |
| GitHub Actions workflows and composite actions | tool format | doc | pure-go:gopkg.in/yaml.v3 (n/a) | L3 | L3 (L1 good, L2 good, L3 good) | C github-actions-oracle (actionlint AST + yaml.v3 for action metadata) github.com/rhysd/actionlint v1.7.12 | — | 2026-09-28 | at target (LB-Q-github-actions) |
| CSS (.css) | language | tags | tree-sitter/tree-sitter-css (MIT) | L3 | none (measured) | B universal-ctags 6.2.1 | file | 2026-09-28 | LB-Q-css |
| SQL (.sql; schema and migrations) | language | tags | DerekStride/tree-sitter-sql (MIT) | L2 | L2 (L1 good, L2 good) | C sqlglot 27.8.0; B universal-ctags 6.2.1 | syntax | 2026-09-28 | at target (LB-Q-sql) |
| npm/pnpm/yarn package manifests (package.json, workspaces) | tool format | doc | pure-go:encoding/json (n/a) | L3 | L3 (L1 excellent, L2 excellent, L3 good) | A npm query ':root' (each manifest as npm loads it) 11.16.0; C npm query + npm ls --workspaces (a scratch copy with the workspace links npm install makes) 11.16.0 | syntax | 2026-10-01 | at target (LB-Q-package-json) |
| Shell / Bash (.sh .bash, shebangs) | language | tags | tree-sitter/tree-sitter-bash (MIT) | L2 | L2 (L1 good, L2 good) | C tree-sitter-tags:bash-tags.scm py-tree-sitter 0.25.2, tree-sitter-bash 0.25.1; B universal-ctags 6.2.1 | — | 2026-09-28 | at target (LB-Q-bash) |
| Java (.java) | language | tags | tree-sitter/tree-sitter-java (MIT) | L4 | L3 (L1 excellent, L2 good, L3 excellent) | B javaparser 3.28.2; A scip-java 0.12.3 | scip:scip-java | 2026-10-01 | LB-Q-java |
| Python packaging (pyproject.toml, requirements*.txt, setup.cfg) | tool format | doc | pure-go:github.com/pelletier/go-toml/v2 (n/a) | L2 | L2 (L1 good, L2 good) | C tomllib reference parse tomllib(python3.14) | syntax | 2026-09-28 | at target (LB-Q-python-packaging) |
| C# (.cs) | language | tags | tree-sitter/tree-sitter-c-sharp (MIT) | L4 | L1 (L1 excellent) | B roslyn-syntax Microsoft.CodeAnalysis.CSharp 5.9.0; A scip-dotnet 0.1.0-SNAPSHOT | — | 2026-10-01 | LB-Q-csharp |
| C (.c, .h by sniff) | language | tags | tree-sitter/tree-sitter-c (MIT) | L4 | L2 (L1 good, L2 excellent) | A clang-preprocessor Apple clang version 21.0.0 (clang-2100.3.34.2); A scip-clang 0.4.0 | scip:scip-clang | 2026-10-01 | LB-Q-c |
| Terraform / OpenTofu (.tf .tfvars .tf.json) | tool format | native | native:github.com/hashicorp/hcl/v2 (MPL-2.0 (library dependency, unmodified)) | L3 | L3 (L1 excellent, L2 excellent, L3 good) | A opentofu (tofu show -json -config, tofu graph -type=plan) 1.13.0 | syntax | 2026-10-01 | at target (LB-Q-terraform) |
| C++ (.cc .cpp .cxx .hpp .hh .hxx, .h by sniff) | language | tags | tree-sitter/tree-sitter-cpp (MIT) | L4 | L1 (L1 excellent) | A clang-preprocessor Apple clang version 21.0.0 (clang-2100.3.34.2); A scip-clang 0.4.0; B universal-ctags 6.2.1 | scip:scip-clang | 2026-10-01 | LB-Q-cpp |
| Go (.go) | language | native | native:go/parser+go/types (BSD-3-Clause) | L4 | L1 (L1 excellent) | A scip-go 0.2.7; B universal-ctags 6.2.1 | native | 2026-10-01 | LB-Q-go |
| Kubernetes manifests (incl. CRDs) | tool format | doc | pure-go:gopkg.in/yaml.v3 (n/a) | L3 | L3 (L1 good, L2 good, L3 good) | C kubernetes-oracle (independent gopkg.in/yaml.v3 walk) gopkg.in/yaml.v3 v3.0.1 | — | 2026-10-01 | at target (LB-Q-kubernetes) |
| PowerShell (.ps1 .psm1 .psd1) | language | tags | airbus-cert/tree-sitter-powershell (MIT (verify at pin)) | L2 | L2 (L1 excellent, L2 good) | B powershell-ast PowerShell 7.5.4 System.Management.Automation.Language.Parser | syntax | 2026-09-28 | at target (LB-Q-powershell) |
| Docker Compose files | tool format | doc | pure-go:gopkg.in/yaml.v3 (n/a) | L3 | L3 (L1 excellent, L2 excellent, L3 excellent) | A docker compose config --format json 5.1.4 | — | 2026-10-01 | at target (LB-Q-docker-compose) |
| PHP (.php) | language | tags | tree-sitter/tree-sitter-php (MIT) | L4 | L1 (L1 excellent) | B php-parser 5.7.0.0; A scip-php 0.0.1 | syntax | 2026-10-01 | LB-Q-php |
| Helm charts (Chart.yaml, values.yaml, templates/*.yaml, *.tpl) | tool format | tags+doc | ngalaiko/tree-sitter-go-template (MIT (verify at pin)) | L3 | L3 (L1 good, L2 good, L3 good) | C helm-oracle (Go text/template/parse for templates, gopkg.in/yaml.v3 for Chart.yaml and values.yaml) text/template/parse (go1.26); gopkg.in/yaml.v3 v3.0.1 | — | 2026-09-28 | at target (LB-Q-helm) |
| Rust (.rs) | language | tags | tree-sitter/tree-sitter-rust (MIT) | L4 | L2 (L1 excellent, L2 excellent) | A rust-analyzer 0.0.0 (03fcb77246 2026-09-27) | scip:rust-analyzer | 2026-10-01 | LB-Q-rust |
| Makefile / GNU make (Makefile, *.mk, GNUmakefile) | tool format | tags | tree-sitter-grammars/tree-sitter-make (MIT) | L2 | L2 (L1 excellent, L2 excellent) | A make -pn 4.4.1; B universal-ctags 6.2.1 | syntax | 2026-09-29 | at target (LB-Q-makefile) |
| Kotlin (.kt .kts) | language | tags | fwcd/tree-sitter-kotlin (MIT) | L4 | L2 (L1 excellent, L2 good) | B kotlin-psi 2.3.20 | scip:scip-java | 2026-09-29 | LB-Q-kotlin |
| Lua (.lua) | language | tags | tree-sitter-grammars/tree-sitter-lua (MIT) | L2 | L2 (L1 excellent, L2 good) | B luaparse 0.3.1 (luaVersion LuaJIT); C tree-sitter-tags:lua-tags.scm py-tree-sitter 0.25.2, tree-sitter-lua 0.5.0 | syntax | 2026-09-29 | at target (LB-Q-lua) |
| GitLab CI (.gitlab-ci.yml, included files) | tool format | doc | pure-go:gopkg.in/yaml.v3 (n/a) | L3 | L3 (L1 excellent, L2 good, L3 good) | A gitlab-ci-local --list-json gitlab-ci-local 4.75.1; C gitlab-ci-refs-oracle (independent yaml.v3 walk of needs, dependencies and extends, over the pipelines gitlab-ci-local accepts) gopkg.in/yaml.v3 v3.0.1 | — | 2026-09-29 | at target (LB-Q-gitlab-ci) |
| Ruby (.rb, Rakefile, Gemfile by basename) | language | tags | tree-sitter/tree-sitter-ruby (MIT) | L3 | L3 (L1 excellent, L2 good, L3 good) | B ruby-require ruby 2.6.10; C tree-sitter-tags:ruby-tags.scm py-tree-sitter 0.25.2, tree-sitter-ruby 0.23.1; B universal-ctags 6.2.1 | scip:scip-ruby | 2026-10-01 | at target (LB-Q-ruby) |
| Maven POMs (pom.xml) | tool format | doc | pure-go:encoding/xml (n/a) | L3 | L3 (L1 excellent, L2 good, L3 good) | C maven-pom-walk (independent encoding/xml decode of each POM's own parent and dependencies, checked against mvn help:effective-pom) go1.26.3 encoding/xml; Maven 3.9.11; maven-help-plugin 3.5.2; -P release; A mvn help:effective-pom Maven 3.9.11; maven-help-plugin 3.5.2; -P release | syntax | 2026-09-29 | at target (LB-Q-maven) |
| Dart (.dart) | language | tags | UserNobody14/tree-sitter-dart (MIT) | L4 | L3 (L1 excellent, L2 excellent, L3 good) | A scip-dart 1.7.0 | scip:scip-dart | 2026-10-01 | LB-Q-dart |
| Cargo manifests (Cargo.toml, workspaces) | tool format | doc | pure-go:github.com/pelletier/go-toml/v2 (n/a) | L3 | L3 (L1 excellent, L2 excellent, L3 excellent) | A cargo metadata (--no-deps, and --all-features for the resolved graph) cargo 1.98.1 (797e8a9bc 2026-08-05) | syntax | 2026-09-29 | at target (LB-Q-cargo) |
| Swift (.swift) | language | tags | alex-pinkus/tree-sitter-swift (MIT) | L3 | refused; measured L2 (L1 excellent, L2 good) | B swiftc -frontend -dump-parse 6.4 | — | 2026-09-29 | re-plan (LB-Q-swift-rest refused) |
| Go modules (go.mod, go.work) | tool format | native | native:golang.org/x/mod (BSD-3-Clause) | L3 | L3 (L1 excellent, L2 excellent, L3 excellent) | A go mod edit -json, go work edit -json, go list -e -m -json all go1.26.3 darwin/arm64 | syntax | 2026-09-29 | at target (LB-Q-go-mod) |
| SCSS / Sass (.scss .sass) | language | tags | tree-sitter-grammars/tree-sitter-scss (MIT (verify at pin)) | L3 | unsupported (disclosed) | — | — | — | LB-Q-scss |
| Less (.less) | language | tags | jimliang/tree-sitter-less (verify at pin) | L3 | unsupported (disclosed) | — | — | — | LB-Q-less |
| MSBuild projects (.csproj .fsproj .vbproj .props .targets, .sln) | tool format | doc | pure-go:encoding/xml (n/a) | L3 | L3 (L1 excellent, L2 excellent, L3 excellent) | A dotnet msbuild (-getItem, -getProperty and -preprocess) MSBuild 18.9.11.42413; .NET SDK 10.0.401 | syntax | 2026-09-29 | at target (LB-Q-msbuild) |
| Kustomize (kustomization.yaml) | tool format | doc | pure-go:gopkg.in/yaml.v3 (n/a) | L3 | L3 (L1 excellent, L2 excellent, L3 excellent) | A kustomize (kubectl-embedded) v5.8.1 | syntax | 2026-09-29 | at target (LB-Q-kustomize) |
| Assembly (.s .S .asm) | language | tags | RubixDev/tree-sitter-asm (MIT) | L1 | refused | — | — | — | re-plan (LB-Q-assembly-rest refused) |
| Argo CD Applications and Argo Workflows | tool format | doc | pure-go:gopkg.in/yaml.v3 (n/a) | L3 | unsupported (disclosed) | — | — | — | LB-Q-argo |
| R (.R .r) | language | tags | r-lib/tree-sitter-r (MIT) | L2 | unsupported (disclosed) | — | — | — | LB-Q-r |
| Ansible playbooks, roles and inventories | tool format | doc | pure-go:gopkg.in/yaml.v3 (n/a) | L3 | unsupported (disclosed) | — | — | — | LB-Q-ansible |
| Groovy (.groovy .gvy) | language | tags | murtaza64/tree-sitter-groovy (verify at pin) | L2 | refused | — | — | — | re-plan (LB-Q-groovy-rest refused) |
| Composer manifests (composer.json) | tool format | doc | pure-go:encoding/json (n/a) | L2 | unsupported (disclosed) | — | — | — | LB-Q-composer |
| Visual Basic .NET (.vb) | language | tags | govindbanura/tree-sitter-vbnet (weak, 2026) (verify at pin) | L1 | unsupported (disclosed) | — | — | — | LB-Q-vbnet |
| Bazel / Starlark (BUILD, BUILD.bazel, *.bzl, WORKSPACE, MODULE.bazel) | tool format | tags | tree-sitter-grammars/tree-sitter-starlark (MIT) | L3 | unsupported (disclosed) | — | — | — | LB-Q-bazel |
| MATLAB (.m by sniff) | language | tags | acristoffers/tree-sitter-matlab (MIT) | L1 | tolerated | — | — | — | LB-Q-matlab |
| CMake (CMakeLists.txt, *.cmake) | tool format | tags | uyha/tree-sitter-cmake (MIT (verify at pin)) | L3 | tolerated | — | syntax | — | LB-Q-cmake |
| Perl (.pl .pm .t by sniff) | language | tags | tree-sitter-perl/tree-sitter-perl (MIT) | L2 | unsupported (disclosed) | — | — | — | LB-Q-perl |
| Protocol Buffers (.proto) | tool format | tags | coder3101/tree-sitter-proto (MIT (verify at pin)) | L3 | L3 (L1 excellent, L2 excellent, L3 excellent) | A protoc 34.1 | syntax | 2026-10-01 | at target (LB-Q-protobuf) |
| GDScript (.gd) | language | tags | PrestonKnopp/tree-sitter-gdscript (MIT) | L2 | unsupported (disclosed) | — | — | — | LB-Q-gdscript |
| GraphQL schemas and operations (.graphql .gql) | tool format | tags | bkegley/tree-sitter-graphql (MIT (verify at pin)) | L3 | tolerated | — | syntax | — | LB-Q-graphql |
| Elixir (.ex .exs) | language | tags | elixir-lang/tree-sitter-elixir (Apache-2.0) | L3 | unsupported (disclosed) | — | — | — | LB-Q-elixir |
| OpenAPI / Swagger documents | tool format | doc | pure-go:gopkg.in/yaml.v3 (n/a) | L3 | L3 (L1 excellent, L2 excellent, L3 excellent) | A @apidevtools/swagger-parser 13.1.0; A @apidevtools/swagger-parser ($refs.paths) 13.1.0 | — | 2026-10-01 | at target (LB-Q-openapi) |
| JSON Schema documents | tool format | doc | pure-go:encoding/json (n/a) | L3 | unsupported (disclosed) | — | — | — | LB-Q-json-schema |
| Scala (.scala .sc) | language | tags | tree-sitter/tree-sitter-scala (MIT) | L4 | tolerated | — | scip:scip-java | — | LB-Q-scala |
| Nix (.nix, flake.nix) | tool format | tags | nix-community/tree-sitter-nix (MIT) | L3 | unsupported (disclosed) | — | — | — | LB-Q-nix |
| Objective-C (.m .mm, .h by sniff) | language | tags | tree-sitter-grammars/tree-sitter-objc (MIT) | L3 | unsupported (disclosed) | — | — | — | LB-Q-objective-c |
| Jupyter notebooks (.ipynb) | tool format | tags+doc | pure-go:encoding/json + python dialect (n/a) | L2 | unsupported (disclosed) | — | — | — | LB-Q-jupyter |
| Luau (.luau) | language | tags | tree-sitter-grammars/tree-sitter-luau (MIT) | L2 | unsupported (disclosed) | — | — | — | LB-Q-luau |
| Taskfile (go-task: Taskfile.yml) | tool format | doc | pure-go:gopkg.in/yaml.v3 (n/a) | L3 | L3 (L1 excellent, L2 good, L3 excellent) | A task --list-all --json go-task 3.52.0; C task --list-all --json (names) with a decode of deps, cmds and defer go-task 3.52.0 | syntax | 2026-09-29 | at target (LB-Q-taskfile) |
| Rego / OPA policy (.rego) | language | native | native:pure-go (n/a) | L3 | L3 (L1 excellent, L2 excellent, L3 good) | A opa parse 1.18.2 | file | 2026-10-02 | at target (LB-Q-rego) |
| Git hook managers (lefthook.yml, .pre-commit-config.yaml) | tool format | doc | pure-go:gopkg.in/yaml.v3 (n/a) | L1 | L1 (L1 good) | A lefthook dump 1.13.6; C ruby psych 3.1.0 (ruby 2.6.10) | syntax | 2026-09-29 | at target (LB-Q-git-hooks-config) |
| Astro components (.astro) | language | tags | virchau13/tree-sitter-astro (MIT) | L2 | unsupported (disclosed) | — | — | — | LB-Q-astro |
| Gradle builds (build.gradle, build.gradle.kts, settings.gradle*) | tool format | tags | murtaza64/tree-sitter-groovy + fwcd/tree-sitter-kotlin (MIT) | L3 | L2 (L1 excellent, L2 excellent) | A gradle (help, init-script model) 9.7.0 (spring-boot), 8.14.1 (kafka) | syntax | 2026-10-01 | LB-Q-gradle |
| Vue single-file components (.vue) | language | tags | tree-sitter-grammars/tree-sitter-vue (MIT) | L3 | unsupported (disclosed) | — | — | — | LB-Q-vue |
| Jenkins pipelines (Jenkinsfile, *.jenkinsfile) | tool format | tags | murtaza64/tree-sitter-groovy (verify at pin) | L2 | unsupported (disclosed) | — | — | — | LB-Q-jenkinsfile |
| Svelte components (.svelte) | language | tags | tree-sitter-grammars/tree-sitter-svelte (MIT) | L3 | unsupported (disclosed) | — | — | — | LB-Q-svelte |
| Azure Pipelines (azure-pipelines.yml, templates) | tool format | doc | pure-go:gopkg.in/yaml.v3 (n/a) | L3 | unsupported (disclosed) | — | — | — | LB-Q-azure-pipelines |
| CUE (.cue) | language | native | native:cuelang.org/go (Apache-2.0) | L4 | tolerated | — | syntax | — | LB-Q-cue |
| CircleCI (.circleci/config.yml) | tool format | doc | pure-go:gopkg.in/yaml.v3 (n/a) | L2 | unsupported (disclosed) | — | — | — | LB-Q-circleci |
| Delphi / Object Pascal (.pas .dpr, .pp by sniff) | language | tags | Isopod/tree-sitter-pascal (MIT) | L1 | unsupported (disclosed) | — | — | — | LB-Q-pascal |
| Fortran (.f90 .f95 .f03 .f .for) | language | tags | stadelmanma/tree-sitter-fortran (MIT) | L2 | unsupported (disclosed) | — | — | — | LB-Q-fortran |
| AWS CloudFormation / SAM templates | tool format | doc | pure-go:gopkg.in/yaml.v3 (n/a) | L3 | L2 (L1 excellent, L2 excellent) | A cfn-lint 1.52.1 | — | 2026-09-29 | LB-Q-cloudformation |
| Ada (.adb .ads) | language | tags | briot/tree-sitter-ada (MIT (verify at pin)) | L1 | L1 (L1 good) | B universal-ctags 6.2.1 | syntax | 2026-09-29 | at target (LB-Q-ada) |
| COBOL (.cob .cbl .cpy) | language | tags | yutaro-sakamoto/tree-sitter-cobol (MIT) | L1 | unsupported (disclosed) | — | — | — | LB-Q-cobol |
| Terragrunt (terragrunt.hcl, root.hcl) | tool format | tags | tree-sitter-grammars/tree-sitter-hcl (MIT) | L3 | unsupported (disclosed) | — | — | — | LB-Q-terragrunt |
| Zig (.zig) | language | tags | tree-sitter-grammars/tree-sitter-zig (MIT) | L3 | unsupported (disclosed) | — | — | — | LB-Q-zig |
| Erlang (.erl .hrl) | language | tags | WhatsApp/tree-sitter-erlang (Apache-2.0) | L2 | unsupported (disclosed) | — | — | — | LB-Q-erlang |
| HashiCorp HCL tool configs (Packer *.pkr.hcl, Nomad jobspecs, Vault/Consul configs, docker-bake.hcl) | tool format | tags | tree-sitter-grammars/tree-sitter-hcl (MIT) | L2 | unsupported (disclosed) | — | — | — | LB-Q-hcl-tools |
| F# (.fs .fsi .fsx, .fs by sniff) | language | tags | ionide/tree-sitter-fsharp (MIT) | L2 | unsupported (disclosed) | — | — | — | LB-Q-fsharp |
| OCaml (.ml .mli) | language | tags | tree-sitter/tree-sitter-ocaml (MIT) | L2 | unsupported (disclosed) | — | — | — | LB-Q-ocaml |
| Pulumi YAML programs and project files (Pulumi.yaml, Pulumi.<stack>.yaml) | tool format | doc | pure-go:gopkg.in/yaml.v3 (n/a) | L2 | unsupported (disclosed) | — | — | — | LB-Q-pulumi-yaml |
| Gleam (.gleam) | language | tags | gleam-lang/tree-sitter-gleam (Apache-2.0) | L3 | unsupported (disclosed) | — | — | — | LB-Q-gleam |
| Prolog (.pl by sniff, .pro) | language | tags | foxyseta/tree-sitter-prolog (verify at pin) | L1 | unsupported (disclosed) | — | — | — | LB-Q-prolog |
| Tekton pipelines and tasks | tool format | doc | pure-go:gopkg.in/yaml.v3 (n/a) | L2 | unsupported (disclosed) | — | — | — | LB-Q-tekton |
| Haskell (.hs) | language | tags | tree-sitter-grammars/tree-sitter-haskell (MIT) | L3 | unsupported (disclosed) | — | — | — | LB-Q-haskell |
| Julia (.jl) | language | tags | tree-sitter-grammars/tree-sitter-julia (MIT) | L2 | unsupported (disclosed) | — | — | — | LB-Q-julia |
| Crossplane compositions and XRDs | tool format | doc | pure-go:gopkg.in/yaml.v3 (n/a) | L3 | unsupported (disclosed) | — | — | — | LB-Q-crossplane |
| Clojure (.clj .cljs .cljc .edn) | language | tags | sogaiu/tree-sitter-clojure (CC0-1.0) | L2 | unsupported (disclosed) | — | — | — | LB-Q-clojure |
| Common Lisp (.lisp .lsp .asd) | language | tags | tree-sitter-grammars/tree-sitter-commonlisp (MIT) | L1 | L1 (L1 good) | B universal-ctags 6.2.1 | syntax | 2026-09-29 | at target (LB-Q-commonlisp) |
| Skaffold (skaffold.yaml) | tool format | doc | pure-go:gopkg.in/yaml.v3 (n/a) | L2 | unsupported (disclosed) | — | — | — | LB-Q-skaffold |
| Scheme and Racket (.scm .ss .rkt) | language | tags | 6cdh/tree-sitter-scheme + 6cdh/tree-sitter-racket (MIT) | L1 | unsupported (disclosed) | — | — | — | LB-Q-scheme-racket |
| Solidity (.sol) | language | tags | JoranHonig/tree-sitter-solidity (MIT) | L2 | unsupported (disclosed) | — | — | — | LB-Q-solidity |
| Tiltfile (Starlark) | tool format | tags | tree-sitter-grammars/tree-sitter-starlark (MIT) | L2 | unsupported (disclosed) | — | — | — | LB-Q-tiltfile |
| Elm (.elm) | language | tags | elm-tooling/tree-sitter-elm (MIT) | L2 | unsupported (disclosed) | — | — | — | LB-Q-elm |
| Nim (.nim .nims) | language | tags | alaviss/tree-sitter-nim (MPL-2.0 (unmodified; verify at pin)) | L2 | unsupported (disclosed) | — | — | — | LB-Q-nim |
| just (justfile, .justfile) | tool format | tags | IndianBoy42/tree-sitter-just (MIT (verify at pin)) | L2 | unsupported (disclosed) | — | — | — | LB-Q-justfile |
| Crystal (.cr) | language | tags | crystal-lang-tools/tree-sitter-crystal (MIT (verify at pin)) | L2 | unsupported (disclosed) | — | — | — | LB-Q-crystal |
| D (.d) | language | tags | gdamore/tree-sitter-d (MIT) | L2 | unsupported (disclosed) | — | — | — | LB-Q-d |
| Earthly (Earthfile) | tool format | tags | glehmann/tree-sitter-earthfile (verify at pin) | L2 | unsupported (disclosed) | — | — | — | LB-Q-earthfile |
| Markdown (.md .mdx) | tool format | native | native:github.com/yuin/goldmark (MIT) | L3 | tolerated | — | syntax | — | LB-Q-markdown |
| VBA (.bas .cls .frm by sniff) | language | tags | harumiWeb/tree-sitter-vba (weak) (verify at pin) | L1 | unsupported (disclosed) | — | — | — | LB-Q-vba |
| Generic JSON, YAML and TOML documents | tool format | doc | pure-go:encoding/json + gopkg.in/yaml.v3 + go-toml/v2 (n/a) | L1 | tolerated | — | syntax | — | LB-Q-json-yaml-toml |
| Verilog / SystemVerilog (.v by sniff, .sv .svh .vh) | language | tags | gmlarumbe/tree-sitter-systemverilog (MIT) | L1 | unsupported (disclosed) | — | — | — | LB-Q-verilog |
| templ components (.templ) | tool format | tags | vrischmann/tree-sitter-templ (MIT (verify at pin)) | L2 | tolerated | — | syntax | — | LB-Q-templ |
| VHDL (.vhd .vhdl) | language | tags | jpt13653903/tree-sitter-vhdl (simplified by design) (verify at pin) | L1 | L1 (L1 excellent) | B universal-ctags 6.2.1 | syntax | 2026-09-29 | at target (LB-Q-vhdl) |
| Dotenv files (.env, .env.example, *.env) | tool format | doc | pure-go:line parser (n/a) | L1 | unsupported (disclosed) | — | — | — | LB-Q-dotenv |
| GLSL shaders (.glsl .vert .frag .comp, .fs by sniff) | language | tags | tree-sitter-grammars/tree-sitter-glsl (MIT) | L1 | unsupported (disclosed) | — | — | — | LB-Q-glsl |
| nginx configuration (nginx.conf, .conf by sniff, sites-enabled/* by glob) | tool format | native | native:pure-go (n/a) | L3 | L3 (L1 good, L2 good, L3 good) | C crossplane 0.5.8 | syntax | 2026-10-01 | at target (LB-Q-nginx) |
| HAProxy configuration (haproxy.cfg, .cfg by sniff) | tool format | native | native:pure-go (n/a) | L3 | L2 (L1 excellent, L2 excellent) | A haproxy 3.4.6-56332c5 | syntax | 2026-10-01 | LB-Q-haproxy |
| Caddyfile (Caddyfile, Caddyfile.* by glob, .caddyfile) | tool format | native | native:pure-go (n/a) | L3 | L2 (L1 excellent, L2 excellent) | A caddy v2.11.6 | syntax | 2026-10-01 | LB-Q-caddyfile |
| systemd units (*.service *.timer *.socket …) | tool format | doc | pure-go:INI parser (n/a) | L2 | unsupported (disclosed) | — | — | — | LB-Q-systemd |
| HLSL shaders (.hlsl .hlsli .fx) | language | tags | tree-sitter-grammars/tree-sitter-hlsl (MIT) | L1 | unsupported (disclosed) | — | — | — | LB-Q-hlsl |
| Dev containers (devcontainer.json, devcontainer-feature.json) | tool format | doc | pure-go:JSONC (n/a) | L2 | unsupported (disclosed) | — | — | — | LB-Q-devcontainer |
| Envoy proxy configuration (bootstrap, xDS resource files) | tool format | doc | pure-go:yaml.v3 (n/a) | L3 | L2 (L1 excellent, L2 excellent) | A envoy 1.39.1 | syntax | 2026-10-01 | LB-Q-envoy |
| Kong declarative configuration (decK, DB-less) | tool format | doc | pure-go:yaml.v3 (n/a) | L3 | L2 (L1 excellent, L2 good) | C deck v1.68.0 (34b752e) | syntax | 2026-10-01 | LB-Q-kong |
| Traefik static and file-provider configuration | tool format | doc | pure-go:yaml.v3 + go-toml/v2 (n/a) | L3 | L2 (L1 excellent, L2 good) | A traefik 3.7.13 | syntax | 2026-10-01 | LB-Q-traefik |
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
| Laravel Blade templates (.blade.php) | language | tags | EmranMR/tree-sitter-blade (MIT (verify at pin)) | L2 | unsupported (disclosed) | — | — | — | LB-Q-blade |
| Serverless Framework (serverless.yml) | tool format | doc | pure-go:gopkg.in/yaml.v3 (n/a) | L2 | unsupported (disclosed) | — | — | — | LB-Q-serverless |
| Conan recipes and lockfiles (conanfile.txt, conanfile.py, conan.lock, conandata.yml) | tool format | tags+doc | tree-sitter/tree-sitter-python + pure-go:INI, encoding/json, gopkg.in/yaml.v3 (verify at pin) | L3 | unsupported (disclosed) | — | — | — | LB-Q-conan |
| vcpkg manifests (vcpkg.json, vcpkg-configuration.json) | tool format | doc | pure-go:encoding/json (n/a) | L3 | unsupported (disclosed) | — | — | — | LB-Q-vcpkg |
| Meson builds (meson.build, meson.options, meson_options.txt) | tool format | tags | candidate tree-sitter-grammars/tree-sitter-meson (verify at pin) (verify at pin) | L3 | unsupported (disclosed) | — | — | — | LB-Q-meson |
| Clang compilation databases (compile_commands.json) | tool format | doc | pure-go:encoding/json (n/a) | L2 | unsupported (disclosed) | — | — | — | LB-Q-compile-commands |
| Azure Bicep (.bicep, .bicepparam) | tool format | tags | candidate tree-sitter-grammars/tree-sitter-bicep (verify at pin) (verify at pin) | L3 | unsupported (disclosed) | — | — | — | LB-Q-bicep |
| Azure Resource Manager templates (azuredeploy.json shape, by content) | tool format | doc | pure-go:encoding/json (n/a) | L3 | unsupported (disclosed) | — | — | — | LB-Q-arm-templates |
| AWS CDK apps (cdk.json; programs ride their language) | tool format | doc | pure-go:encoding/json (n/a) | L2 | unsupported (disclosed) | — | — | — | LB-Q-aws-cdk |
| Flux CD resources (Kustomization, HelmRelease, GitRepository, OCIRepository) | tool format | doc | pure-go:gopkg.in/yaml.v3 (n/a) | L3 | unsupported (disclosed) | — | — | — | LB-Q-flux |
| Kyverno policies (ClusterPolicy, Policy, PolicyException) | tool format | doc | yaml.v3 (n/a) | L2 | unsupported (disclosed) | — | — | — | LB-Q-kyverno |
| OPA Gatekeeper ConstraintTemplates and constraints | tool format | doc | yaml.v3 (n/a) | L3 | unsupported (disclosed) | — | — | — | LB-Q-gatekeeper |
| Conftest policies (policy/*.rego, conftest.toml) | tool format | tags+doc | rides LB-Q-rego's grammar + pure-go:go-toml/v2 (verify at pin) | L2 | unsupported (disclosed) | — | — | — | LB-Q-conftest |
| Cedar policies and schemas (.cedar .cedarschema) | language | tags | chrnorm/tree-sitter-cedar 5092204d or cedar-go v1.8.0, both refused at pin (LB-Q-cedar-rest) (MIT (tree-sitter-cedar LICENCE); Apache-2.0 (cedar-go)) | L3 | refused | — | — | — | re-plan (LB-Q-cedar-rest refused) |
| AWS IAM policy documents (JSON by content, inline in CloudFormation and Terraform) | tool format | doc | encoding/json (n/a) | L2 | unsupported (disclosed) | — | — | — | LB-Q-aws-iam-policy |
| Azure Policy definitions and initiatives (policyRule JSON) | tool format | doc | encoding/json (n/a) | L2 | unsupported (disclosed) | — | — | — | LB-Q-azure-policy |
| Cloud Custodian policies (policies YAML) | tool format | doc | yaml.v3 (n/a) | L2 | unsupported (disclosed) | — | — | — | LB-Q-cloud-custodian |
| Checkov custom policies (YAML graph checks, Python checks) | tool format | tags+doc | yaml.v3 + tree-sitter/tree-sitter-python (n/a) | L2 | unsupported (disclosed) | — | — | — | LB-Q-checkov-policies |
| HashiCorp Sentinel policies (.sentinel) | language | tags | none open (verify at pin) (verify at pin) | L2 | unsupported (disclosed) | — | — | — | LB-Q-sentinel |
| PgBouncer configuration (pgbouncer.ini) | tool format | doc | pure-go:INI parser (n/a) | L2 | unsupported (disclosed) | — | — | — | LB-Q-pgbouncer |
| ProxySQL configuration (proxysql.cnf) | tool format | doc | pure-go:libconfig parser (verify at pin) (n/a) | L2 | unsupported (disclosed) | — | — | — | LB-Q-proxysql |
| fathom configuration (.fathom/config.toml) | tool format | doc | pure-go:go-toml/v2 (n/a) | L2 | unsupported (disclosed) | — | — | — | LB-Q-fathom-config |
| fathom concerns (.fathom/concerns.cue) | tool format | native | native:cuelang.org/go (n/a) | L2 | unsupported (disclosed) | — | — | — | LB-Q-fathom-concerns |
| fathom layer packs (.fathom/layers/*/layer.cue, .fathom/rules/*.cue) | tool format | native | native:cuelang.org/go (n/a) | L2 | unsupported (disclosed) | — | — | — | LB-Q-fathom-layer-packs |
| fathom plan grammar v2 rows (docs/plans/*.md) | tool format | doc | pure-go:plan row grammar (n/a) | L3 | unsupported (disclosed) | — | — | — | LB-Q-fathom-plan-rows |
