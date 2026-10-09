package fusion

// register.go — New, the extractor registration table (every extension, basename and tool
// format the pipeline dispatches), sniffedExtractor, refusedDialects, RegisterExtractor and
// resolveExtractor, the per-file dispatch. Split out of pipeline.go unchanged.

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/geoffmilleraz/fathom/internal/connector"
	"github.com/geoffmilleraz/fathom/internal/extractor"
	"github.com/geoffmilleraz/fathom/internal/store"
)

// New constructs a Pipeline with extractors pre-registered for all supported
// file types. Additional extractors can be added or replaced by calling
// RegisterExtractor after construction.
func New(conn connector.Connector, st store.Store, cfg Config) *Pipeline {
	if cfg.Concurrency <= 0 {
		cfg.Concurrency = 1
	}
	if cfg.InflightBytes <= 0 {
		cfg.InflightBytes = DefaultInflightBytes
	}
	p := &Pipeline{
		conn:           conn,
		st:             st,
		cfg:            cfg,
		exts:           map[string]extractor.Extractor{},
		toolExts:       map[ToolKind]extractor.Extractor{},
		sniffExts:      map[string]extractor.Extractor{},
		sniffExt:       map[string]bool{},
		queue:          map[string][]pendingFile{},
		siblingIdx:     map[string]*siblingIndex{},
		classifiedZero: map[string]int64{},
		gitHeadFn:      realGitHead,
		gitLogFn:       realGitLog,
	}

	// Go
	p.exts[".go"] = extractor.NewGoExtractor()

	// Python — tree-sitter via wazero/WASM (ADR-021 §4), calibration-gated
	// against the incumbent CGO extractor (docs/plans/extraction-treesitter.md
	// XT-3); see PROVENANCE.md and the plan's recorded scores.
	py, err := extractor.NewTSExtractor("python")
	if err != nil {
		panic(fmt.Sprintf("fusion: tsextract python: %v", err))
	}
	p.exts[".py"] = py

	// JavaScript / TypeScript / JSX / TSX — tree-sitter via wazero/WASM
	// (ADR-021 §4), calibration-gated (XT-2).
	js, err := extractor.NewTSExtractor("javascript")
	if err != nil {
		panic(fmt.Sprintf("fusion: tsextract javascript: %v", err))
	}
	p.exts[".js"] = js
	p.exts[".jsx"] = js
	p.exts[".ts"] = js
	p.exts[".tsx"] = js
	// J1 (app-kit O18/O32/O33): .mjs/.cjs/.mts/.cts were classified as
	// "js"/"ts" (connector.extClassMap) but never DISPATCHED to an
	// extractor here — the actual root cause of find_symbol missing every
	// exported symbol in a new .mjs/.cjs file after ingest. Same extractor
	// instance; tsextract.go's own variantForExt picks the right grammar
	// per extension.
	p.exts[".mjs"] = js
	p.exts[".cjs"] = js
	p.exts[".mts"] = js
	p.exts[".cts"] = js

	// Java. The tags port (internal/extractor/dialects/java/) replaced the CGO
	// javaextract.go in ingest (LB-F301, ADR-048 amendment D1, dispatch at
	// parity): lang-parity read it at least as good on every metric up to its
	// demonstrated level, L4, on guava and retrofit once the file resolver let
	// a single-type import and a file's own type shadow the package scope
	// (MERGE, docs/bench/readings/lang/parity/java.json, O513). It adds a
	// JavaAnnotation per annotation type, signs every definition, and records
	// a constructor invocation as a call to its class (LB-F230, O268);
	// javaextract.go stays as lang-parity's incumbent.
	jv, err := extractor.NewTSExtractor("java")
	if err != nil {
		panic(fmt.Sprintf("fusion: tsextract java: %v", err))
	}
	p.exts[".java"] = jv

	// Rust. LB-Q-rust built and measured a tree-sitter dialect
	// (internal/extractor/dialects/rust/) — see docs/bench/readings/lang/rust.json
	// — that reads L2 excellent but not its target (L4): §11 step 2's merge
	// gate requires the dialect to meet its target level, so the CGO incumbent
	// stays the live dispatch here. A switch must bump ExtractorEpoch: the
	// dialect names an impl by its head type instead of the type field's
	// source text and adds trait methods, unions, aliases, consts, statics and
	// macros (dialects/rust/PROVENANCE.md).
	p.exts[".rs"] = extractor.NewRustExtractor()

	// C. The tags port (internal/extractor/dialects/c/) replaced the CGO
	// cextract.go in ingest (LB-F119, ADR-048 amendment D1, dispatch at
	// parity): lang-parity read it at least as good on every metric up to its
	// demonstrated level, L2, on libuv and curl (MERGE,
	// docs/bench/readings/lang/parity/c.json). It also emits macros and
	// unions; cextract.go stays as lang-parity's incumbent.
	cPort, err := extractor.NewTSExtractor("c")
	if err != nil {
		panic(fmt.Sprintf("fusion: tsextract c: %v", err))
	}
	p.exts[".c"] = cPort

	// C++. The tags dialect (internal/extractor/dialects/cpp/) replaced the
	// CGO cppextract.go in ingest (LB-F117, ADR-048 amendment D1, dispatch at
	// parity): once its cpp-macros pre-pass read decorator and try/catch
	// macros (O455), lang-parity read it at least as good on every metric up
	// to its demonstrated level on fmt, json and spdlog (MERGE at L3 when
	// switched, at L4 on wave lf1's final reading,
	// docs/bench/readings/lang/parity/cpp.json). The dialect makes a namespace
	// a container, so every namespaced symbol is renamed, and it adds macros,
	// aliases and unions: the switch bumps ExtractorEpoch
	// (dialects/cpp/PROVENANCE.md). cppextract.go stays as lang-parity's
	// incumbent.
	cpp, err := extractor.NewTSExtractor("cpp")
	if err != nil {
		panic(fmt.Sprintf("fusion: tsextract cpp: %v", err))
	}
	p.exts[".cc"] = cpp
	p.exts[".cpp"] = cpp
	p.exts[".cxx"] = cpp
	p.exts[".hpp"] = cpp
	p.exts[".hh"] = cpp
	p.exts[".hxx"] = cpp

	// .h is polyglot: the header router sniffs C++ signals and picks the
	// extractor per file (docs/plans/cpp-toolchain.md CT-3), the C port for a
	// C header and the C++ dialect above for a C++ one.
	p.exts[".h"] = extractor.NewCHeaderRouter(cPort, cpp)

	// CMake (full-basename key for CMakeLists.txt, extension for modules;
	// docs/plans/cpp-toolchain.md CT-4/CT-5)
	cmake := extractor.NewCMakeExtractor()
	p.exts["CMakeLists.txt"] = cmake
	p.exts[".cmake"] = cmake

	// C#. The tags port (internal/extractor/dialects/csharp/) replaced the
	// CGO csharpextract.go in ingest (LB-F232, ADR-048 amendment D1, dispatch
	// at parity): lang-parity read it at least as good on every metric up to
	// its demonstrated level, L1, on Polly and Newtonsoft.Json once the Roslyn
	// dump dated a definition by its name (MERGE,
	// docs/bench/readings/lang/parity/csharp.json, O457), and ahead on every
	// metric it measured beyond but file_edge_precision. It adds a
	// CSharpProperty per property, signs every definition, and writes the
	// dialect's call text (a generic callee without its type arguments, no
	// nameof, a member call qualified by its receiver: LB-F189..LB-F191, O462);
	// csharpextract.go stays as lang-parity's incumbent.
	cs, err := extractor.NewTSExtractor("csharp")
	if err != nil {
		panic(fmt.Sprintf("fusion: tsextract csharp: %v", err))
	}
	p.exts[".cs"] = cs

	// Ruby — tree-sitter via wazero/WASM (ADR-021 §4), calibration-gated (XT-4).
	rb, err := extractor.NewTSExtractor("ruby")
	if err != nil {
		panic(fmt.Sprintf("fusion: tsextract ruby: %v", err))
	}
	p.exts[".rb"] = rb
	p.exts[".rake"] = rb // LB-Q-ruby: one extractor (one grammar module) for every Ruby spelling
	p.exts[".ru"] = rb
	p.exts["Rakefile"] = rb

	// Kotlin — tree-sitter via wazero/WASM (ADR-021 §4 new-language
	// breadth, wave P2 / O69 / O79): no CGO incumbent, so no calibration
	// gate — this closes the Kotlin extraction gap directly.
	kt, err := extractor.NewTSExtractor("kotlin")
	if err != nil {
		panic(fmt.Sprintf("fusion: tsextract kotlin: %v", err))
	}
	p.exts[".kt"] = kt
	p.exts[".kts"] = kt

	// HTML — tree-sitter via wazero/WASM (LB-Q-html, design
	// §13/language-breadth): new; no incumbent bespoke walker existed for
	// HTML (a fleet gap, not a port), so there is no calibration gate.
	html, err := extractor.NewTSExtractor("html")
	if err != nil {
		panic(fmt.Sprintf("fusion: tsextract html: %v", err))
	}
	p.exts[".html"] = html
	p.exts[".htm"] = html

	// CSS — tree-sitter via wazero/WASM (LB-Q-css, design
	// §13/language-breadth): new; no incumbent bespoke walker existed for
	// CSS (a fleet gap, not a port), so there is no calibration gate.
	css, err := extractor.NewTSExtractor("css")
	if err != nil {
		panic(fmt.Sprintf("fusion: tsextract css: %v", err))
	}
	p.exts[".css"] = css

	// Terraform HCL
	hcl := extractor.NewHCLExtractor()
	p.exts[".tf"] = hcl
	p.exts[".tfvars"] = hcl

	// YAML: route .github/workflows/* to GHA extractor, everything else to K8s.
	k8s := extractor.NewK8sExtractor()
	gha := extractor.NewGHAExtractor()
	yamlExt := extractor.NewYAMLExtractor(k8s, gha)
	p.exts[".yaml"] = yamlExt
	p.exts[".yml"] = yamlExt

	// CUE
	p.exts[".cue"] = extractor.NewCUEExtractor()

	// Markdown
	p.exts[".md"] = extractor.NewMarkdownExtractor()

	// Templates
	p.exts[".templ"] = extractor.NewTemplExtractor()

	// go.mod and go.work (LB-Q-go-mod, language-breadth: the native go-mod
	// dialect, x/mod's parser; it retired gomodextract.go's require-only line
	// scanner). Matched by full basename (p.exts[base] runs before the
	// extension fallback); one extractor serves both, and it gives the
	// cross-file pass its DocResult (enrichDocXrefEdges).
	goModules := extractor.NewGoModulesExtractor()
	p.exts["go.mod"] = goModules
	p.exts["go.work"] = goModules

	// Helm (full-basename keys take precedence over .yaml k8s extractor)
	p.exts["Chart.yaml"] = extractor.NewHelmChartExtractor()
	p.exts["values.yaml"] = extractor.NewHelmValuesExtractor()
	p.exts[".tpl"] = extractor.NewHelmTemplateExtractor()

	// Dockerfile — tree-sitter via wazero/WASM (LB-Q-dockerfile, design
	// §13/language-breadth): retires DockerfileExtractor's line-scanner.
	// Full-basename key takes precedence over extension dispatch.
	dockerfile, err := extractor.NewTSExtractor("dockerfile")
	if err != nil {
		panic(fmt.Sprintf("fusion: tsextract dockerfile: %v", err))
	}
	p.exts["Dockerfile"] = dockerfile
	p.exts[".dockerfile"] = dockerfile

	// Docker Compose (full-basename keys take precedence over .yaml k8s
	// extractor). LB-Q-docker-compose (LB-F89): the doc-engine docker-compose
	// dialect serves the four basenames and, through classifyYAML's content
	// signal (a top-level services:) and toolExts below, every other compose
	// file, an override or an arbitrarily named fragment; it retires
	// composeextract.go at lang-parity and gives the doc-xref pass its
	// DocResult (a root entity per file; include, extends.file, image: and
	// build: references, LB-X-docker-image-ref).
	compose, err := extractor.NewDocDialectExtractor("docker-compose")
	if err != nil {
		panic(fmt.Sprintf("fusion: doc-dialect docker-compose: %v", err))
	}
	p.exts["docker-compose.yml"] = compose
	p.exts["docker-compose.yaml"] = compose
	p.exts["compose.yml"] = compose
	p.exts["compose.yaml"] = compose

	// Kustomize (full-basename keys take precedence over .yaml k8s extractor
	// — ADR-021 §2 / TI-3: kustomization.yaml is valid k8s-shaped YAML but
	// (almost) never has metadata.name, so the generic k8s extractor
	// silently produced zero symbols for it). LB-Q-kustomize: the doc-engine
	// kustomize dialect serves both basenames (the kustomization, its
	// resources, patches, images and generators, and the resource, base,
	// component and patch files that resolve to another file of the
	// repository), retiring kustomizeextract.go; the ToolKind tag stays
	// ToolKustomize (toolKindForBasename).
	kustomize, err := extractor.NewDocDialectExtractor("kustomize")
	if err != nil {
		panic(fmt.Sprintf("fusion: doc-dialect kustomize: %v", err))
	}
	p.exts["kustomization.yaml"] = kustomize
	p.exts["kustomization.yml"] = kustomize

	// ToolKind-keyed extractors (ADR-021 §2): dispatched via the pass-3
	// classifier (classifyYAML) for .yaml/.yml files whose basename is NOT
	// one of the special keys above and which are not under
	// .github/workflows/ (GHA keeps its existing, unchanged dispatch via
	// yamlExt). CRD and generic k8s manifests share the same extractor —
	// the classifier only tags CRDs distinctly for visibility.
	p.toolExts[ToolK8sManifest] = k8s
	p.toolExts[ToolK8sCRD] = k8s
	p.toolExts[ToolCompose] = compose
	p.toolExts[ToolHelmTemplateYAML] = extractor.NewHelmTemplateYAMLExtractor()

	// OpenAPI / Swagger (LB-Q-openapi): the doc-engine openapi dialect, which
	// retired openapiextract.go (lang-parity: every L1 metric >= the incumbent's,
	// level L3). A YAML document reaches it through classifyYAML's top-level
	// openapi:/swagger: signal and a JSON one through resolveExtractor's sniff
	// (openAPIJSON); ReingestFile makes the same decision.
	openAPI, err := extractor.NewDocDialectExtractor("openapi")
	if err != nil {
		panic(fmt.Sprintf("fusion: doc-dialect openapi: %v", err))
	}
	p.toolExts[ToolOpenAPI] = openAPI

	// GitLab CI (LB-Q-gitlab-ci, language-breadth: new — a doc-engine tool
	// format, no incumbent). Every pipeline file — the root .gitlab-ci.yml,
	// .gitlab/ci/**, *.gitlab-ci.yml — reaches it through classifyYAML's path
	// signal (the dialect manifest's own basenames and globs) and toolExts, the
	// way Helm templates and OpenAPI documents do; ReingestFile makes the same
	// path decision.
	gitlabCI, err := extractor.NewDocDialectExtractor("gitlab-ci")
	if err != nil {
		panic(fmt.Sprintf("fusion: doc-dialect gitlab-ci: %v", err))
	}
	p.toolExts[ToolGitLabCI] = gitlabCI

	// AWS CloudFormation / SAM (LB-Q-cloudformation, language-breadth: new, a
	// doc-engine tool format, no incumbent). A template claims no name: a
	// .yaml or .yml reaches it through classifyYAML's content signal, last of
	// all the signals, and a .json through resolveExtractor's sniff before the
	// generic JSON extractor (a basename claim, package.json, still wins);
	// ReingestFile makes the same decision.
	cloudFormation, err := extractor.NewDocDialectExtractor("cloudformation")
	if err != nil {
		panic(fmt.Sprintf("fusion: doc-dialect cloudformation: %v", err))
	}
	p.toolExts[ToolCloudFormation] = cloudFormation

	// Azure and AWS app frameworks (wave iac, LB-F146..LB-F148): the
	// Serverless Framework's serverless.yml, a Pulumi project file and a CDK
	// app's cdk.json are claimed by name, as fast-path keys read before any
	// content signal (so ReingestFile makes the same decision); an ARM
	// template (by content) and a Pulumi stack file (by glob) reach theirs
	// through appFrameworkDispatch (armdispatch.go). A Bicep file rides the
	// tags registry by its extension.
	for id, bases := range map[string][]string{
		"serverless":  {"serverless.yml", "serverless.yaml"},
		"pulumi-yaml": {"Pulumi.yaml", "Pulumi.yml"},
		"aws-cdk":     {"cdk.json"},
	} {
		x, err := extractor.NewDocDialectExtractor(id)
		if err != nil {
			panic(fmt.Sprintf("fusion: doc-dialect %s: %v", id, err))
		}
		for _, b := range bases {
			p.exts[b] = x
		}
	}

	// Ansible (LB-Q-ansible, language-breadth iac wave: new, a doc-engine tool
	// format, no incumbent). A playbook, role file, variable file or YAML
	// inventory reaches it through classifyYAML's last signal
	// (ansibleDetects); ReingestFile makes the same decision.
	ansible, err := extractor.NewDocDialectExtractor("ansible")
	if err != nil {
		panic(fmt.Sprintf("fusion: doc-dialect ansible: %v", err))
	}
	p.toolExts[ToolAnsible] = ansible

	// Protocol Buffers
	p.exts[".proto"] = extractor.NewProtoExtractor()

	// GraphQL
	gql := extractor.NewGraphQLExtractor()
	p.exts[".graphql"] = gql
	p.exts[".gql"] = gql

	// TOML
	p.exts[".toml"] = extractor.NewTOMLExtractor()

	// SQL (DDL)
	p.exts[".sql"] = extractor.NewSQLExtractor()

	// JSON
	p.exts[".json"] = extractor.NewJSONExtractor()

	// npm/pnpm/yarn package manifest (LB-Q-package-json, language-breadth:
	// new — a doc-engine tool format, no incumbent). resolveExtractor's
	// basename lookup (p.exts[base]) runs before the ".json" extension
	// fallback above, so this dialect — not the generic JSONExtractor —
	// governs every "package.json" file.
	pkgJSON, err := extractor.NewDocDialectExtractor("package-json")
	if err != nil {
		panic(fmt.Sprintf("fusion: doc-dialect package-json: %v", err))
	}
	p.exts["package.json"] = pkgJSON

	// Composer (LB-Q-composer): composer.json is claimed by basename. Nil-safe
	// while the dialect is absent from a build (wave c1 hub); the composer
	// commit makes it strict, as the dialects above.
	if composer, err := extractor.NewDocDialectExtractor("composer"); err == nil {
		p.exts["composer.json"] = composer
	}

	// Python packaging (LB-Q-python-packaging, language-breadth: new — a
	// doc-engine tool format, no incumbent). resolveExtractor's basename
	// lookup (p.exts[base]) runs before the ".toml" extension fallback
	// above, so this dialect — not the generic TOMLExtractor — governs
	// every "pyproject.toml" file. requirements*.txt and setup.cfg from
	// the same queue entry are left unclassified (LB-Q-python-packaging-
	// rest): the doc engine decodes only YAML/JSON/TOML/XML (design §4).
	pyProject, err := extractor.NewDocDialectExtractor("python-packaging")
	if err != nil {
		panic(fmt.Sprintf("fusion: doc-dialect python-packaging: %v", err))
	}
	p.exts["pyproject.toml"] = pyProject

	// Maven POM (LB-Q-maven, language-breadth: new — a doc-engine tool
	// format, no incumbent). No ".xml" extractor exists, so the basename
	// lookup (p.exts[base]) claims "pom.xml" alone: pom-shaded.xml,
	// settings.xml and every other XML file stay unclassified, as the
	// manifest's match says.
	maven, err := extractor.NewDocDialectExtractor("maven")
	if err != nil {
		panic(fmt.Sprintf("fusion: doc-dialect maven: %v", err))
	}
	p.exts["pom.xml"] = maven

	// Cargo manifest (LB-Q-cargo, language-breadth: new — a doc-engine tool
	// format). The basename lookup (p.exts[base]) runs before the ".toml"
	// extension fallback (the generic TOMLExtractor), so it governs every
	// "Cargo.toml" file; every other TOML file (rust-toolchain.toml,
	// .cargo/config.toml, rustfmt.toml) stays with the generic extractor.
	cargo, err := extractor.NewDocDialectExtractor("cargo")
	if err != nil {
		panic(fmt.Sprintf("fusion: doc-dialect cargo: %v", err))
	}
	p.exts["Cargo.toml"] = cargo

	// MSBuild (LB-Q-msbuild, language-breadth: new — a doc-engine tool format,
	// no incumbent). No extractor claims these extensions, so the extension
	// lookup (p.exts[ext]) governs every project, props and targets file;
	// .user sidecars, solutions and other XML stay unclassified, as the
	// manifest's match says.
	msbuild, err := extractor.NewDocDialectExtractor("msbuild")
	if err != nil {
		panic(fmt.Sprintf("fusion: doc-dialect msbuild: %v", err))
	}
	for _, ext := range []string{".csproj", ".fsproj", ".vbproj", ".props", ".targets"} {
		p.exts[ext] = msbuild
	}

	// vcpkg manifests (LB-Q-vcpkg, language-breadth: new — a doc-engine tool
	// format, no incumbent beside the generic JSONExtractor). The basename
	// lookup (p.exts[base]) runs before the ".json" extension fallback, so it
	// governs every "vcpkg.json" and "vcpkg-configuration.json"; every other
	// JSON file stays with the generic extractor.
	vcpkg, err := extractor.NewDocDialectExtractor("vcpkg")
	if err != nil {
		panic(fmt.Sprintf("fusion: doc-dialect vcpkg: %v", err))
	}
	p.exts["vcpkg.json"] = vcpkg
	p.exts["vcpkg-configuration.json"] = vcpkg

	// Conan (LB-Q-conan, language-breadth: new — a tags+doc tool format, no
	// incumbent beyond the python dialect's .py and the generic extractors'
	// .yml and .lock handling). The basename lookup (p.exts[base]) runs before
	// every extension fallback, so conanfile.py is a recipe (the tags engine on
	// the python grammar) and no longer a generic Python file, and conanfile.txt
	// (an INI decode), conan.lock and conandata.yml are documents; every other
	// .py, .txt, .lock and .yml is untouched. The recipe's classes and methods
	// are therefore not python symbols (dialects/conan/PROVENANCE.md).
	conan, err := extractor.NewTagsDocExtractor("conan")
	if err != nil {
		panic(fmt.Sprintf("fusion: tags+doc dialect conan: %v", err))
	}
	for _, base := range []string{"conanfile.py", "conanfile.txt", "conan.lock", "conandata.yml"} {
		p.exts[base] = conan
	}

	// Taskfile (LB-Q-taskfile, language-breadth: new, a doc-engine tool format,
	// no incumbent). A .yml or .yaml file with one of go-task's eight basenames
	// is unambiguous by name (resolveExtractor's basename lookup, before the
	// classifier that left these unclassified); an included Taskfile under
	// another name stays unclassified, as the manifest's match says.
	taskfile, err := extractor.NewDocDialectExtractor("taskfile")
	if err != nil {
		panic(fmt.Sprintf("fusion: doc-dialect taskfile: %v", err))
	}
	for _, name := range []string{
		"Taskfile.yml", "Taskfile.yaml", "taskfile.yml", "taskfile.yaml",
		"Taskfile.dist.yml", "Taskfile.dist.yaml", "taskfile.dist.yml", "taskfile.dist.yaml",
	} {
		p.exts[name] = taskfile
	}

	// Git hook managers (LB-Q-git-hooks-config, language-breadth: new, a
	// doc-engine tool format, no incumbent). Each basename is unambiguous by
	// name (resolveExtractor's basename lookup, before the classifier that left
	// these unclassified); the doc spec reads each format by its basename.
	gitHooks, err := extractor.NewDocDialectExtractor("git-hooks-config")
	if err != nil {
		panic(fmt.Sprintf("fusion: doc-dialect git-hooks-config: %v", err))
	}
	for _, name := range []string{
		"lefthook.yml", "lefthook.yaml", ".lefthook.yml", ".lefthook.yaml",
		"lefthook-local.yml", "lefthook-local.yaml", ".lefthook-local.yml", ".lefthook-local.yaml",
		".pre-commit-config.yaml", ".pre-commit-hooks.yaml",
	} {
		p.exts[name] = gitHooks
	}

	// Clang compilation database (LB-Q-compile-commands, language-breadth: new, a
	// doc-engine tool format, no incumbent). The basename lookup (p.exts[base])
	// runs before the ".json" extension fallback (the generic JSONExtractor), so
	// it governs every "compile_commands.json" at any depth: a translation unit
	// per entry, its include directories, macros and output as references. The
	// dialect's own decoder reads the flags as includeRoots (resolvestrategy.go)
	// does, so the file the include-path strategy reads is the file the index holds.
	compileCommands, err := extractor.NewDocDialectExtractor("compile-commands")
	if err != nil {
		panic(fmt.Sprintf("fusion: doc-dialect compile-commands: %v", err))
	}
	p.exts["compile_commands.json"] = compileCommands

	// Scala
	scala := extractor.NewScalaExtractor()
	p.exts[".scala"] = scala
	p.exts[".sc"] = scala

	// PHP. The tags port (internal/extractor/dialects/php/) replaced the CGO
	// phpextract.go in ingest (LB-F183, ADR-048 amendment D1, dispatch at
	// parity): lang-parity read it at least as good on every metric up to its
	// demonstrated level, L1, on guzzle and monolog (MERGE,
	// docs/bench/readings/lang/parity/php.json), and ahead or level on every
	// metric it measured beyond. It adds a PhpNamespace symbol per namespaced
	// file, takes Package from the declared namespace, signs every definition
	// and extracts enum methods (dialects/php/PROVENANCE.md); phpextract.go
	// stays as lang-parity's incumbent.
	php, err := extractor.NewTSExtractor("php")
	if err != nil {
		panic(fmt.Sprintf("fusion: tsextract php: %v", err))
	}
	p.exts[".php"] = php

	// Bash (.sh .bash, shebangs). LB-Q-bash built and measured a
	// tree-sitter dialect (internal/extractor/dialects/bash/) — see
	// docs/bench/readings/lang/bash.json — but its target (L2) is not met
	// (§11 step 2's merge gate): the call-recall/call-precision metrics
	// have no oracle at all for a language dialect (FamilyCalls is fed
	// only by ScoreSCIP, tier A, and no SCIP producer exists for Bash) —
	// L1 reads good. The CGO incumbent stays the live dispatch here; only
	// a future item that reaches L2 (or a retargeted level, or a wired
	// tier-C/tool calls oracle) switches this line.
	bash := extractor.NewBashExtractor()
	p.exts[".sh"] = bash
	p.exts[".bash"] = bash

	// Lua (.lua) is not wired here: LB-Q-lua's tree-sitter dialect
	// (internal/extractor/dialects/lua/) met its target (L2, docs/bench/readings/
	// lang/lua.json) and retired the CGO walker, so the manifest loop below
	// claims ".lua" like any other tags-engine dialect.

	// Every other embedded tags-engine dialect (design §2.1: "fusion.New
	// registers every manifest's extensions and basenames"). The five
	// lines above wire javascript/typescript/python/ruby/kotlin by hand
	// (their own grammar/variant selection lives inside TSExtractor); a
	// dialect added later by the language-breadth queue — CSS onward —
	// had no such line and, before this, silently dispatched to no
	// extractor at all (extraction returned "no-extractor" for every one
	// of its files, in both real ingest and indexbench lang-oracle). This
	// registers each remaining tags-engine manifest's extensions,
	// basenames and variant extensions that no earlier line already
	// claimed; a key already claimed above is left untouched. A manifest
	// load error here leaves nothing registered — the same fail-open
	// convention as dialectgate.go's dialectRules — since
	// TestDialectManifests already fails the build on a bad manifest.
	if ds, dErr := extractor.LoadDialects(); dErr == nil {
		for _, d := range ds {
			if d.Engine != "tags" || refusedDialects[d.ID] {
				continue
			}
			keys := append([]string(nil), d.Match.Extensions...)
			keys = append(keys, d.Match.Basenames...)
			for _, v := range d.Variants {
				keys = append(keys, v.Extensions...)
			}
			var e extractor.Extractor
			for _, k := range keys {
				if _, claimed := p.exts[k]; claimed {
					continue
				}
				if e == nil {
					var nerr error
					e, nerr = extractor.NewTSExtractor(d.ID)
					if nerr != nil {
						panic(fmt.Sprintf("fusion: tsextract %s: %v", d.ID, nerr))
					}
				}
				p.exts[k] = e
			}
			// A dialect that claims an extension only through match.sniff
			// (MATLAB's .m, shared with Objective-C, LB-Q-matlab) has no
			// p.exts key: registering the extension would claim every file
			// with it. Ingest's classifier decides per file
			// (connector.ClassifyDialect, sniffedExtractor).
			if d.Match.Sniff != nil {
				if e == nil {
					var nerr error
					e, nerr = extractor.NewTSExtractor(d.ID)
					if nerr != nil {
						panic(fmt.Sprintf("fusion: tsextract %s: %v", d.ID, nerr))
					}
				}
				p.sniffExts[d.ID] = e
				for _, x := range d.Match.Sniff.Extensions {
					p.sniffExt[strings.ToLower(x)] = true
				}
			}
		}
		registerProxyDialects(p, ds)
		registerHCLDialects(p, ds)
	}

	return p
}

// registerProxyDialects wires the native proxy dialects (LB-Q-nginx,
// LB-Q-haproxy, LB-Q-caddyfile; LB-F101): each manifest basename and
// extension (nginx.conf, haproxy.cfg, Caddyfile, .caddyfile) is a p.exts key,
// so ingest and ReingestFile find it on their fast path; every other file a
// manifest claims — by glob (sites-enabled/*, Caddyfile.*) or by content
// sniff (.conf, .cfg) — is dispatched by sniffedExtractor, which asks
// connector.ClassifyDialect exactly as the ingest gate does, so a .conf no
// sniff claims (supervisord.conf, redis.conf) is never extracted and is
// counted ambiguous.
func registerProxyDialects(p *Pipeline, ds []extractor.Dialect) {
	for _, d := range ds {
		e, ok := extractor.ProxyDialectExtractor(d.ID)
		if d.Engine != "native" || !ok {
			continue
		}
		for _, k := range append(append([]string(nil), d.Match.Basenames...), d.Match.Extensions...) {
			if _, claimed := p.exts[k]; !claimed {
				p.exts[k] = e
			}
		}
		p.sniffExts[d.ID] = e
		if sn := d.Match.Sniff; sn != nil {
			for _, x := range sn.Extensions {
				p.sniffExt[strings.ToLower(x)] = true
			}
		}
	}
}

// sniffedExtractor is the extractor of the dialect classification gave
// relPath to (design §5, LB-F9), for a file no p.exts key claims: a dialect
// that owns an extension only through match.sniff (MATLAB's .m, nginx's
// .conf, HAProxy's .cfg), and a native proxy dialect's file claimed by a
// manifest glob (sites-enabled/*, Caddyfile.*) or by an extension in another
// case (.Caddyfile). ok is false when classification settles nothing — an
// ambiguous file never reaches an extractor (the ingest gate counts it
// before dispatch).
func (p *Pipeline) sniffedExtractor(relPath string, content []byte) (extractor.Extractor, bool) {
	if !p.sniffExt[strings.ToLower(filepath.Ext(relPath))] && connector.ClassifyDialect(proxyDialectRules(), relPath, nil).Dialect == "" {
		return nil, false
	}
	v := connector.ClassifyDialect(dialectRules(), relPath, content)
	switch v.By {
	case connector.ClassifiedBySniff, connector.ClassifiedByGlob, connector.ClassifiedByExtension, connector.ClassifiedByBasename:
	default:
		return nil, false
	}
	e, ok := p.sniffExts[v.Dialect]
	return e, ok
}

// refusedDialects are tags-engine dialects built and measured but refused by
// their plan row (docs/design/language-breadth.md §14.2): no file is routed to
// them, so their language stays disclosed as unsupported (the connector's
// generated table lists it; langbench.RefusedItems reads the same refusal from
// the plan for the tracker). swift: LB-Q-swift-rest — 36 of its 642
// calibration files (5.6%) parse with an error under the pinned grammar, over
// the 5% line. tcl: LB-Q-tcl-rest — 516 of its 827 calibration files (62.4%).
// vimscript: LB-Q-vimscript-rest — 3 of its 7 (42.9%). Dispatching one later is
// an ExtractorEpoch bump.
var refusedDialects = map[string]bool{"swift": true, "tcl": true, "vimscript": true}

// RegisterExtractor registers an extractor for the given file extension
// (including leading dot, e.g. ".py"). It replaces any existing extractor for
// that extension.
func (p *Pipeline) RegisterExtractor(ext string, e extractor.Extractor) {
	p.exts[ext] = e
}

// resolveExtractor is ingest's per-file dispatch: which extractor reads
// relPath, the ToolKind its observations carry, and whether a YAML file
// was left unclassified (never extracted, counted). Shared by extraction
// and ExtractForBench so the language-breadth harness measures exactly
// the dispatch ingest runs (LB-F23).
func (p *Pipeline) resolveExtractor(relPath string, content []byte, idx *siblingIndex) (e extractor.Extractor, toolKind ToolKind, ok, unclassified bool) {
	base := filepath.Base(relPath)
	ext := filepath.Ext(relPath)
	if ce, ck, cok := p.cloudPolicyDispatch(relPath, content, idx); cok {
		// IAM policies, Azure Policy and Cloud Custodian files (LB-F138): the dialect's content sniff AND a decode check, never a file an earlier signal claims.
		return ce, ck, true, false
	}
	if fe, fk, fok := fathomFormatDispatch(relPath, content); fok {
		return fe, fk, true, false // .fathom/config.toml and docs/plans/*.md (LB-F219): a manifest glob, plans also by content
	}
	if ge, gk, gok := gatewayDispatch(relPath, content); gok {
		// Envoy, Kong and Traefik configuration (LB-F104): a manifest
		// basename or glob AND the doc spec's content sniff, never either alone.
		return ge, gk, true, false
	}
	if pe, pk, pok := policyDispatch(relPath, content); pok {
		return pe, pk, true, false // Kyverno and Gatekeeper files (LB-F134), before the Kubernetes verdict
	}
	if ce, ck, cok := crDispatch(relPath, content, idx); cok {
		// Argo, Flux and Crossplane objects (LB-F149): every document of the
		// file is one dialect's, else the file stays Kubernetes.
		return ce, ck, true, false
	}
	if te, tk, tok := timoniDispatch(relPath, content, idx); tok {
		return te, tk, true, false // Timoni modules, bundles and runtimes (LB-F216): the CUE extraction plus the Timoni entities
	}
	switch {
	case ext == ".yaml" || ext == ".yml":
		switch {
		case strings.Contains(filepath.ToSlash(relPath), ".github/workflows/"):
			// GHA dispatch was never the grounded defect — keep the
			// existing YAMLExtractor wrapper unchanged.
			e, ok = p.exts[ext]
			toolKind = ToolGHA
		default:
			if hitE, hit := p.exts[base]; hit {
				// Already unambiguous by basename (Chart.yaml,
				// values.yaml, kustomization.*, compose.*,
				// docker-compose.*) — no content sniff needed.
				e, ok = hitE, true
				toolKind = toolKindForBasename(base)
			} else if ae, ak, aok := appFrameworkDispatch(relPath, content); aok {
				e, ok, toolKind = ae, true, ak // a Pulumi stack file, by the pulumi-yaml glob (LB-F146)
			} else {
				kind := Classify(relPath, content, idx)
				toolKind = kind
				if kind == ToolUnclassified {
					return nil, kind, false, true
				}
				e, ok = p.toolExts[kind]
				if !ok {
					if le := lateYAMLExtractor(kind); le != nil {
						e, ok = le, true // a late YAML dialect (lateyamldispatch.go)
					}
				}
			}
		}
	case ext == ".cue":
		// Classify never changes which extractor runs for .cue files —
		// only the ToolKind tag recorded on the resulting observations.
		e, ok = p.exts[ext]
		toolKind = Classify(relPath, content, idx)
	default:
		// Unchanged fast path: try full basename first (e.g. "go.mod"),
		// then extension.
		e, ok = p.exts[base]
		if !ok && ext == ".json" && cloudFormationJSON(content) {
			// A CloudFormation template written as JSON (LB-Q-cloudformation):
			// its content, not its extension, says what it is.
			e, ok = p.toolExts[ToolCloudFormation]
			toolKind = ToolCloudFormation
		}
		if !ok && ext == ".json" && openAPIJSON(content) {
			// An OpenAPI or Swagger document written as JSON (LB-Q-openapi):
			// the same decision as a YAML one's top-level openapi:/swagger:.
			e, ok = p.toolExts[ToolOpenAPI]
			toolKind = ToolOpenAPI
		}
		if !ok {
			e, toolKind, ok = appFrameworkDispatch(relPath, content) // an ARM template or parameters file, by content (LB-F146)
		}
		if !ok {
			e, ok = p.exts[ext]
		}
		if !ok {
			e, ok = p.sniffedExtractor(relPath, content)
		}
	}
	return e, toolKind, ok, false
}
