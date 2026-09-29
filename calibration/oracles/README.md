# Bench-only oracle instruments (language breadth, ADR-048 D2/D5)

Each directory is one oracle a reading may name, with its tier, install
commands and exact invocation. None of them is a product dependency; all
run as separate processes against clean clones, and their outputs stay out
of the repository. The neutral output shape is `indexbench lang-oracle
-tool-oracle`'s: `{"tool","version","tier","definitions":[{path,name,line,
end,kind}],"calls":[{path,from,to}],"imports":[{path,line,to_path,module}]}`
(repo-relative slash paths, 1-based lines, bare names; `imports` are import
statements resolved to files of the subject, scored as `import_resolution`,
LB-F40).

| Directory | Tier | Languages |
|---|---|---|
| scip-typescript/ | A | TypeScript, JavaScript |
| scip-dotnet/ | A (def_mapping, typed_recall) | C# |
| scip-php/ | A (calls, def_mapping, file edges) | PHP |
| scip-ruby/ | A (diagnostic: typed receivers only) | Ruby |
| scip-dart/ | A (definitions, calls, def_mapping, file edges; typed_recall diagnostic) | Dart |
| swift-parse/ | B (definitions, calls) | Swift |
| rust-analyzer/ | A (definitions, calls, def_mapping, typed_recall, file edges) | Rust |
| roslyn/ | B | C# |
| javaparser/ | B | Java |
| kotlin-psi/ | B | Kotlin |
| powershell-ast/ | B | PowerShell |
| php-parser/ | B | PHP |
| luaparse/ | B | Lua (definitions) |
| protoc-descriptors/ | A (definitions, references, imports) | Protocol Buffers |
| opa-parse/ | A (definitions, calls, imports) | Rego |
| ts-tags/ | C | Bash, Lua and Ruby (calls) |
| sqlglot/ | C | SQL (foreign-key calls) |
| make/ | A (references), B (definitions) | Makefile |
| ruby-require/ | B (imports) | Ruby |
| clang-includes/ | B (imports) | C, C++ |

## Tool-format oracles (D4, LB-F30)

Small programs that write `indexbench lang-oracle`'s tool-oracle JSON (`-tool-oracle`,
design §10) for dialects no tier-A/B tool covers (and, for docker-compose, to convert a tier-A
tool's own JSON, which carries no source lines, into that shape). Each is its own Go module — never
imported by fathom, never a product dependency — and is run from its own directory:

| Dialect | Tier | Parser | Emits | Run |
|---|---|---|---|---|
| `github-actions` | C | `github.com/rhysd/actionlint` v1.7.12 (workflows); `gopkg.in/yaml.v3` (action.yml) | definitions, `needs:` references, `cross_references` for local `uses: ./path` resolved from the repository root to the workflow file or the directory's `action.yml`/`action.yaml` (file granularity) | `cd calibration/oracles/github-actions && go run . -subject <checkout> > workspace/tmp/oracle/github-actions-<subject>.json` |
| `gitlab-ci` | A (`definitions`); C (`-refs`) | `gitlab-ci-local --list-json` 4.75.1 (npm) run per pipeline root, with its state and download cache kept outside the subject; `gopkg.in/yaml.v3` for each job's file and key line, the local includes a pipeline reaches, and the `-refs` walk | definitions (a pipeline's visible jobs at their key line, the tool's matrix and parallel names folded back to the declared name); with `-refs`, `references` (every `needs`, `dependencies` and `extends` a job or hidden template writes, over the pipelines the tool accepted) and `cross_references` (the subset whose target is declared in exactly one other file of the pipeline, entity granularity) | `cd calibration/oracles/gitlab-ci && go run . -subject <checkout> -state <cache dir> > workspace/tmp/oracle/gitlab-ci-<subject>.json`, and again with `-refs` into `gitlab-ci-<subject>-refs.json`; pass both to `lang-oracle -tool-oracle` (definitions file first) |
| `kubernetes` | C | `gopkg.in/yaml.v3` walk | definitions (`Kind/[namespace/]name` resources, container images), ServiceAccount/ConfigMap/Secret references, `cross_references` for a Service's `spec.selector` joined to every workload whose pod-template labels (a bare Pod's own labels) include it, same namespace (entity granularity) | `cd calibration/oracles/kubernetes && go run . -subject <checkout> > workspace/tmp/oracle/kubernetes-<subject>.json` |
| `docker-compose` | A (`definitions`, `references`); C (`-xref`) | `docker compose config --format json` v5.1.4 run per file with `--profile '*' --no-interpolate --no-env-resolution --no-path-resolution --no-consistency --no-normalize`; `gopkg.in/yaml.v3` for each service's key line, the names a file itself declares, and the `-xref` walk | definitions (a file's own services, at their key line), `depends_on` references as the tool evaluated them; with `-xref`, `cross_references` for `extends.file` and `build:` (the Dockerfile the context and `dockerfile:` name, when it exists) at file granularity | `cd calibration/oracles/docker-compose && go run . -subject <checkout> > workspace/tmp/oracle/docker-compose-<subject>.json`, and again with `-xref` into `docker-compose-<subject>-xref.json`; pass both to `lang-oracle -tool-oracle` |
| `helm` | C | Go `text/template/parse` (templates); `gopkg.in/yaml.v3` (Chart.yaml, values.yaml) | definitions (charts, dependencies, values keys, named templates, `.Values.a.b` accessors, `include`/`template` calls), `references` (a named template's `include`/`template` calls), `cross_references` (a chart's `dependencies[].name` to the one other chart of that name) | `cd calibration/oracles/helm && go run . -subject <checkout> [-helm <helm>] > workspace/tmp/oracle/helm-<subject>.json` |
| `maven` | A (`definitions`); C (`-refs`) | `mvn help:effective-pom` (Apache Maven 3.9.11, maven-help-plugin 3.5.2, JDK 21) run once at the subject's root POM, offline; `encoding/xml` for each POM's file and `<project>` line, the modules a walk reaches (profile modules included) and the `-refs` decode | definitions (one `package` per reactor project, at its `<project>` start tag); with `-refs`, `references` (the parent's artifactId and every direct dependency's, per POM, cross-checked against the effective POM) and `cross_references` (a `<module>` to the POM Maven's module rule names, a `<parent>` and a declared dependency to the reactor project whose effective groupId:artifactId equals it, entity granularity) | `cd calibration/oracles/maven && JAVA_HOME=<jdk> go run . -subject <checkout> -mvn <mvn> -m2 <local repo outside the subject> -offline [-profiles release] -effective <scratch>/maven-<subject>-eff.xml [-files-out <file>] > workspace/tmp/oracle/maven-<subject>.json`, and again with `-refs` (same `-effective` file, which the first run wrote) into `maven-<subject>-refs.json`; pass both to `lang-oracle -tool-oracle` (definitions file first) |
| `cargo` | A | `cargo metadata --no-deps --format-version 1` and `cargo metadata --all-features --format-version 1` (cargo 1.98.1, rustc 1.98.1), run at each workspace root; `go-toml/v2` for the raw cross-check | definitions (one `package` per workspace member, at line 1: the package entity is the manifest itself), `references` (every dependency of each package as the tool lists it, whatever its kind or target table, by real package name), `cross_references` (a resolved dependency edge between two packages of the subject, `[patch]` applied, and the `workspace` entity to each member the tool lists, entity granularity) | `cd calibration/oracles/cargo && PATH=<rustc bin>:$PATH go run . -subject <checkout> -cargo <cargo> -home <scratch CARGO_HOME outside the subject> [-root <manifest>]... [-offline] [-files-out <file>] > workspace/tmp/oracle/cargo-<subject>.json`; pass it to `lang-oracle -tool-oracle` |
| `go-mod` | A | `go mod edit -json`, `go work edit -json` and `go list -e -m -json all` (go1.26.3), run at each go.mod and go.work of the subject; standard library only | definitions (a `module` per go.mod, at its module directive; a `package` per require; a `key` per replace of a go.mod or go.work, `replace/<old>[@<version>]`; per go.work a `namespace`, `workspace`, at its first directive, and a `key` per use, `use/<directory as written>`), `references` (a module to each module it requires), `cross_references` (a require the tool resolves to a directory of the subject holding a go.mod, module to module; a replace to a local directory the tool evaluated as the module's replacement, replace entity to that module; a go.work use the tool lists as a workspace module, use entity to that module; entity granularity) | `cd calibration/oracles/go-mod && go run . -subject <checkout> -modcache <scratch GOMODCACHE outside the subject> > workspace/tmp/oracle/go-mod-<subject>.json`; pass it to `lang-oracle -tool-oracle` |
| `msbuild` | A | `dotnet msbuild` (MSBuild 18.9.11 in .NET SDK 10.0.401): `-getItem:PackageReference,ProjectReference -getProperty:TargetFramework,TargetFrameworks` per project, framework and configuration, and `-preprocess` for the files an evaluation imported; `encoding/xml` for each file's `<Project>` line and the cross-check decode | definitions (a `package` per evaluated project file and per file of the subject an evaluation imported, at its `<Project>` start tag), `references` (a `PackageReference` item, from the file that defined it), `cross_references` (a `ProjectReference` item, from the file that defined it to the project MSBuild resolves it to, when that is a file of the subject; entity granularity) | `cd calibration/oracles/msbuild && go build -o <scratch>/msbuild-oracle . && DOTNET_ROOT=<sdk> <scratch>/msbuild-oracle -subject <checkout> [-under <dir>] -dotnet <sdk>/dotnet -scratch <scratch dir outside the subject> > workspace/tmp/oracle/msbuild-<subject>.json` |
| `kustomize` | A | `kubectl kustomize` (kubectl v1.36.1, Kustomize v5.8.1) run once per `kustomization.yaml` / `kustomization.yml` of a scratch copy of the subject, with `buildMetadata: [originAnnotations]` and a dead proxy, and again with each entry removed; `yq` v4 (an independent YAML reader with source lines) proposes the entries | definitions (a `package` per kustomization the tool built, at its first key, named by the file; a `resource` per resource, base, component, patch and generator and a `data_source` per image, at its line, each one the tool evidences: the build fails without it or its output differs), `references` (the kustomization to each evidenced entry), `cross_references` (a resource, base, component or patch entry that names a file of the subject, a directory naming the kustomization it holds; file granularity) | `cd calibration/oracles/kustomize && go build -o <scratch>/kustomize-oracle . && <scratch>/kustomize-oracle -subject <checkout> -scratch <dir outside the subject> -files-out <file> > workspace/tmp/oracle/kustomize-<subject>.json`; pass it to `lang-oracle -tool-oracle` with `-only "$(paste -sd, <file>)"` |
| `taskfile` | A (`definitions`, `cross_references`); C (`-refs-out`) | `task --list-all --json --taskfile <file>` (go-task 3.52.0) run on every Taskfile of a scratch copy of the subject as the root of its own run, again with each `internal: true` line blanked; `yq` v4 (an independent YAML reader) for the `-refs-out` decode | definitions (a `package` per Taskfile the tool accepted, at its first key, named by the file; a `task` per task the tool lists whose location is that file, at the tool's line, named by the key written there), `cross_references` (an `includes:` entry whose file, or whose directory's Taskfile, contributes a task to the tool's listing; file granularity); with `-refs-out`, `references` (a task to each task its `deps`, `cmds` `task:` calls and `defer` name, tier C) | `cd calibration/oracles/taskfile && go build -o <scratch>/taskfile-oracle . && <scratch>/taskfile-oracle -subject <checkout> -scratch <dir outside the subject> -files-out <file> -refs-out workspace/tmp/oracle/taskfile-<subject>-refs.json > workspace/tmp/oracle/taskfile-<subject>.json`; pass both to `lang-oracle -tool-oracle` (either order) with `-only "$(paste -sd, <file>)"` |

`cargo` is tier A throughout: the tool itself evaluates each workspace (which manifests are members, each
package's name, each package's dependencies with `workspace = true` inheritance, renames and target
tables resolved) and, in the resolving run, draws the graph between packages with every `[patch]`
applied. The tool writes no line, so a package sits at line 1, where the dialect's package entity (the
manifest's root table) does. Name a workspace the root leaves out with `-root` (tokio's two fuzz crates
are each a workspace of their own); the first resolving run needs the network for the workspace's
registry dependencies and a `rustc` on `PATH` (`cargo metadata` without `--no-deps` runs `rustc -vV`),
every later run can use `-offline` on the warm `-home`, and the home is scratch to delete after
measuring. Features and explicit targets are not scored: the tool lists the implicit ones too (a feature
per optional dependency, a target for `src/main.rs`), so what would be scored is what the manifest
declares. The oracle cross-checks them instead (declared-not-in-tool counts on stderr, zero on the
subjects). A tree with fixture manifests (`rust-lang/cargo`: 614 of its 641 `Cargo.toml` under
`tests/testsuite` and the test containers) is measured with `lang-oracle -only
'Cargo.toml,benches/*/Cargo.toml,crates/*/Cargo.toml,credential/*/Cargo.toml'`: the tool builds a
workspace, and the fixtures are manifests it does not attest.

`go-mod` is tier A because the go command itself parses each file and evaluates each module: module mode
(`GOWORK=off`, the module's own replaces applied) and, where a go.work reaches the module, workspace mode, both
with `-e` so a requirement no proxy serves and nothing redirects (three in kubernetes) is reported unresolved and
the rest is still evaluated. The parse is `golang.org/x/mod/modfile`, the package the dialect reads with, so the
definitions attest what the dialect models (every directive, every block form) and not the parser; the tool prints
no position, so each entry's line comes from an independent line scan (its own tokenizer, no modfile), aligned
entry for entry with the tool's list and cross-checked (scan-mismatch count on stderr, zero on both subjects). The
first run needs the network for the `.mod` files of the modules a go.mod names (3 MB for kubernetes, 12 MB with
cli/cli), every later run can reuse the module cache; the cache is scratch to delete after measuring (it is written
with `-modcacherw`), and both subjects are sparse checkouts of the go.mod family (`calibration/corpus.d/go-mod.yaml`),
so no Go source is read and the tool never builds anything. The tool writes go.sum entries it is missing into the
subject: the checkout is scratch too.

`maven` is tier A for definitions because the tool itself evaluates the reactor (module discovery,
parent chain, interpolation, profile activation) and rejects one it cannot; the first run needs the
network for the help plugin and the parents a POM names, every later run is offline. The tool writes
no file and no line, so a walk of `<modules>` supplies them and the two project sets are compared
(walk-only and tool-only counts on stderr). A POM outside the reactor (a test project under
`src/test`, an invoker integration test under `src/it`) is not a project the tool builds and is not in
the truth: measure a reactor's own files with `lang-oracle -only` (`apache/maven`:
`'pom.xml,*/pom.xml'`), and pass `-profiles` for a subject whose modules sit in an inactive profile
(`apache/dubbo`: `release`). `-refs` is tier C: the effective POM merges inheritance, so what a file
declares comes from an independent struct decode of the raw POM, kept honest by the cross-checks
(`declared-not-effective`, `parent-mismatch`, the `build/plugins` count); its cross-file edges join by
groupId:artifactId where the dialect joins by artifactId, so the xref metrics measure that shortcut.

`msbuild` is tier A throughout: MSBuild itself evaluates each project (its Sdk, every import, every
condition and property) and names, for each item it lists, the file that defined it
(`DefiningProjectFullPath`), so what a file declares is the tool's word and a `Directory.Build.props` is
credited with the references it writes and not the projects that import it. One evaluation sees one
configuration and one target framework, so each project is evaluated for Debug and Release, for its default
framework and for each framework its `TargetFrameworks` lists, and the items are unioned; a condition on
anything else (a Windows-only framework on a macOS run, an environment variable) stays false, and the
independent decode inside the oracle prints what a file writes that no evaluation listed
(`declared-not-in-tool`; 3 over Polly, 0 over aspnetcore) and what the tool lists that no file writes
(`tool-not-declared`), and checks every `<Target Name>` the files declare against the tool's preprocessed
projects. `-preprocess` brackets each imported file with `<Import …>` and `</Import>` markers, which is where
the imported props and targets files (also definitions) come from. The SDK is installed inside the scratch
directory (`dotnet-install.sh --version 10.0.401`, with `DOTNET_ROOT`, `NUGET_PACKAGES` and
`DOTNET_CLI_HOME` under it and `DOTNET_ROLL_FORWARD=Major`; a `global.json` that pins another SDK band does not
stop `dotnet msbuild` run from outside the checkout) and deleted with it. A project the tool cannot evaluate is
listed on stderr and left out (its file is then a dialect-only definition); every project evaluated in both
subjects. `dotnet/aspnetcore` cannot be evaluated from a clean checkout: its root `Directory.Build.props` and
`.targets` import `artifacts/bin/GenerateFiles/Directory.Build.{props,targets}`, which its own first build step
writes, so the run creates both as empty `<Project />` files in the scratch clone (outside git; the corpus
shard's `sparse:` list is the MSBuild files only, plus `eng/` and the internal-testing props a test project
imports) and measures `src/Http` with `lang-oracle -only 'src/Http/**'`. Its Arcade SDK turns `<Reference>`
items into ProjectReferences in `eng/targets/ResolveReferences.targets`, so those edges are that file's,
outside the measured tree. Imports are read but not scored (the dialect draws no Import edge); `.sln` files
are not decoded, so `dotnet sln list` is not an oracle here.

`kustomize` is tier A because the tool itself is the judge of every entry. Kustomize lists nothing of what a
kustomization declares, and its `originAnnotations` name the file a resource came from but not the kustomization
that named it (an overlay of an overlay of a base reads as the base's resources), so the oracle asks the tool one
question of each entry: does the build change when this entry is removed? The entries are proposed by a decode
(`yq`, a reader of its own with line numbers; the dialect's walk over the doc engine is not reused) and each is
removed in turn from a copy of the kustomization (`yq 'del(.resources[3])'`) and the build run again with
`buildMetadata: [originAnnotations]` on. An entry is in the truth when the build fails without it or its output
differs. One that changes nothing (an image that matches no container, a patch whose target matches nothing) is
withheld from the truth and counted on stderr as `unevidenced`, so the dialect that emits it pays in precision; on
both subjects every one of 70 entries was evidenced. A kustomization the tool cannot build contributes nothing and
is left out of `-files-out` with its reason on stderr: a remote base or patch (the run sets a dead proxy and treats
a reference that leaves the subject as cross-repository, never fetched), an entry that names a file the checkout
does not hold (`examples/transformerconfigs/images`, `plugins/kustomized-helm`), an undecodable file, and a
`kind: Component` (not a root the tool builds; none in the subjects). Measure with `lang-oracle -only "$(paste -sd,
<files-out>)"`, and run `lang-parity` on a copy of the subject without the excluded files (it has no `-only`). The
harness scores only edges between files the dialect owns, so a resource that names a manifest is dropped, and
counted, as out of scope: the reading's notes carry the number.

`taskfile` is tier A for what the tool lists: go-task evaluates each Taskfile (its version, its includes, its tasks) and reports every
task with the file, line and column of its key, so a definition is the tool's word and its name is the key written at that position (the tool
lists a task under its `label:` and an included task under its namespace). `--list-all` leaves an `internal: true` task out; the oracle
lists it by running the tool again on a copy with that line blanked, every other line where it was, so the tool's lines are the file's. An
`includes:` entry is in the truth when the tool's listing has a task from the file it names (or from the Taskfile its directory holds, by
the tool's search order), which is the tool attesting that it loaded that file; a file with no tasks and a templated one are counted on stderr and left out, and a Taskfile
with a remote include (`https://`) is rejected by the tool as a whole (remote Taskfiles are an experiment, off by default) and excluded. Its `-refs-out` file is tier
C: the tool lists nothing of what a task depends on, and `--summary` renders (so evaluates) a task, dynamic `sh:` variables included, so the
`deps`, `cmds` `task:` and `defer` entries are decoded (`yq`, anchors and merge keys expanded) and each name is checked against the tool's
(a listed task, an alias, a namespaced task, a wildcard match; the ones it does not know are counted on stderr). A Taskfile the tool rejects is
excluded from `-files-out` and counted; the walk skips `testdata`, `vendor`, `node_modules`, `generated`, `.git` and the editor directories, as
the connector does. A Taskfile's global `sh:` variable may be evaluated by the listing (the subjects' are `pwd` and `git describe`): read a
subject's Taskfiles before pointing the oracle at it.

`gitlab-ci` is tier A for definitions because the tool itself evaluates each pipeline (includes,
remote templates, `extends`, matrix expansion) and rejects one it cannot; the tool is not offline
(it downloads the remote includes a pipeline names, and a 429 from gitlab.com is retried), and what
it downloads only adds jobs no local file declares. Its `-refs` output is tier C: the tool merges a
pipeline and erases `extends`, so those edges come from the raw YAML, gated by the tool's
acceptance of the pipeline, and its cross-file rule is the resolver's own. A pipeline the tool rejects
(`gitlab-org/gitlab-runner`: a `needs:` naming a matrix expansion; `omnibus-gitlab`: a `rules:`
expression it cannot evaluate) contributes nothing and is counted on stderr.

`helm` is tier C, not A: the oracle's rules are its own. It reads templates with Go's own parser (the one Helm renders with), so a named template, a call and an accessor are the parse tree's; Chart.yaml and values.yaml with the doc engine's own YAML library; and the L3 join re-implements the dialect's unique-chart-name rule, so it attests the implementation, not the rule. With `-helm`, `helm show chart` loads every chart and its name and dependency names are compared with the walk. The stderr summary counts the include/template calls inside and outside a named template and the repository shape (`file://`, `oci://`, URL, none) of every edge.

`docker-compose` is tier A because the tool itself evaluates each file (anchors, merge keys
and `extends:` resolved, the same load `docker compose up` does); a file it rejects (an
invalid fixture) contributes nothing, and fathom's extraction of it counts against
`sym_precision` as it stands. Its `-xref` output is tier C: `config` merges and erases
`extends:`, so those edges come from the raw YAML.

Tier C, not A (github-actions, kubernetes): both re-implement the format's rules over an independent parser; neither
evaluates the format the way the tool that consumes it does (GitHub's runner, the
Kubernetes API server). A reference whose target file has no root entity (a reusable
workflow without `name:`) is in the oracle but resolves to no fathom entity — a real,
disclosed recall miss, not an oracle error.
