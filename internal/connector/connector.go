// Package connector defines the Connector SPI and its implementations.
//
// The SPI separates source acquisition from the fusion pipeline: any
// Connector implementation can feed the pipeline without changes to
// extraction, storage, or query logic.
//
// Three implementations are provided:
//   - LocalConnector  — reads from a local directory tree (dev / CLI use)
//   - FauxConnector   — reads from the in-repo testdata/fauxtown fixture
//   - (GitHubConnector is a future implementation; not in this package)
package connector

import (
	"context"
	"path/filepath"
	"sort"

	"github.com/geoffmilleraz/fathom/internal/core"
)

// Connector is the source-provider SPI. Implementations must be safe for
// concurrent use by multiple goroutines.
//
// Discover returns a deterministically ordered (sorted by Path) slice of
// SourceRefs for all files the connector considers ingestible under svc.
// The Path field on each SourceRef is relative to the connector's root for
// that service — every connector, including LocalConnector, emits
// repo-relative, slash-separated paths (ADR-017: anchors must be portable;
// an ingested store never records machine-local absolute paths).
//
// Fetch returns the raw bytes of the file identified by ref. It is valid to
// call Fetch with any SourceRef returned by Discover.
//
// Watermark returns an opaque freshness token for the service's corpus:
// equal tokens mean unchanged content. Git-backed corpora return the bare
// HEAD commit SHA; others return a content-addressed manifest hash with the
// "manifest:" prefix. "" means freshness is unknown — an honest state, not
// an error (ADR-017: freshness is domain-neutral, never git-only).
type Connector interface {
	Discover(ctx context.Context, svc core.ServiceRef) ([]core.SourceRef, error)
	Fetch(ctx context.Context, ref core.SourceRef) ([]byte, error)
	Watermark(ctx context.Context, svc core.ServiceRef) (string, error)
}

// classification pairs the SourceKind bucket used when the Connector cannot
// perform deeper AST analysis with the canonical presentation-layer language
// tag for the same extension/basename key. Kind and Lang are always derived
// from the SAME registry entry (ADR-021 §1 / LT-1): there is exactly one
// ext→{kind, language} table in the whole codebase, and fusion.New's
// extractor registration is tested for exact key-parity against it (see
// internal/fusion's language-registry parity test), so a new extractor
// literally cannot be added without also declaring its language here (and
// vice versa).
type classification struct {
	Kind core.SourceKind
	Lang string
}

// extClassMap maps file extensions (or full basenames) to their
// classification. Keys not listed here have no registered extractor;
// LanguageForPath reports core.LangUnclassified for them rather than
// silently omitting them.
var extClassMap = map[string]classification{
	// Go
	".go": {core.KindGoFunc, "go"},

	// Go modules (LB-Q-go-mod, language-breadth: a native tool-format dialect
	// that upgraded the require-only line scanner). go.mod and go.work are
	// their own language tag, "go-mod", not Go's: a manifest is a tool format
	// (typed-edge level "syntax"), and the Go tag's "native" claim is about
	// type-checked source. The basename check takes precedence over any
	// extension; go.sum, go.work.sum and a backup are not claimed.
	"go.mod":  {core.KindGoModule, "go-mod"},
	"go.work": {core.KindGoWorkspace, "go-mod"},

	// Proxy configuration (LB-Q-nginx, LB-Q-haproxy, LB-Q-caddyfile: native
	// dialects, no incumbent). Only the names that settle a file alone are
	// classified here; a .conf (nginx) or .cfg (HAProxy) file is the dialect's
	// only when its content sniff matches, an extensionless sites-enabled/*
	// file and a Caddyfile.* only by their manifest glob, so those carry no
	// language tag, as a sniffed MATLAB .m does not (fusion's sniffedExtractor
	// dispatches them through connector.ClassifyDialect).
	"nginx.conf":  {core.SourceKind("NginxConfig"), "nginx"},
	"haproxy.cfg": {core.SourceKind("HAProxyBackend"), "haproxy"},
	"Caddyfile":   {core.SourceKind("CaddyfileConfig"), "caddyfile"},
	".caddyfile":  {core.SourceKind("CaddyfileConfig"), "caddyfile"},

	// Rego / OPA policy (LB-Q-rego, LB-F131: a native dialect, a reader with
	// no dependency, no incumbent).
	".rego": {core.SourceKind("RegoPackage"), "rego"},

	// Terragrunt and HashiCorp HCL tool configs (LB-Q-terragrunt, LB-Q-hcl-tools:
	// native hclsyntax dialects). Only the names that settle a file alone are
	// classified here; a .hcl file is a Terragrunt file or a Nomad jobspec only
	// by its content sniff, and a *.pkr.hcl, *.pkrvars.hcl, *.nomad.hcl or
	// docker-bake*.hcl file by its manifest glob, so those carry no language
	// tag (fusion's sniffedExtractor dispatches them through
	// connector.ClassifyDialect).
	"terragrunt.hcl":           {core.SourceKind("TerragruntConfig"), "terragrunt"},
	"root.hcl":                 {core.SourceKind("TerragruntConfig"), "terragrunt"},
	"docker-bake.hcl":          {core.SourceKind("BakeTarget"), "hcl-tools"},
	"docker-bake.override.hcl": {core.SourceKind("BakeTarget"), "hcl-tools"},
	".nomad":                   {core.SourceKind("NomadJob"), "hcl-tools"},

	// Infrastructure / config
	".tf":     {core.KindTerraformResource, "terraform"},
	".tfvars": {core.KindTerraformVarValue, "terraform"},
	".cue":    {core.KindCUEDefinition, "cue"},
	".yaml":   {core.KindK8sWorkload, "yaml"},
	".yml":    {core.KindK8sWorkload, "yaml"},

	// Documentation
	".md":  {core.KindDocChunk, "md"},
	".mdx": {core.KindDocChunk, "md"}, // LB-Q-markdown: MDX is Markdown with JSX

	// Application languages
	".java": {core.KindJavaClass, "java"},
	".py":   {core.KindPyFunc, "py"},
	".rb":   {core.KindRubyMethod, "ruby"},
	".js":   {core.KindJSFunc, "js"},
	".jsx":  {core.KindJSFunc, "js"},
	".ts":   {core.KindJSFunc, "ts"},
	".tsx":  {core.KindJSFunc, "ts"},
	// J1 (app-kit O18/O32/O33): the Node/TS module-system pairs a plain
	// .js/.ts glob never covers — same extractor (internal/extractor's
	// "javascript" TSExtractor), same Kind, split only by module system
	// (mjs/cjs -> js grammar, mts/cts -> ts grammar), matching
	// tsextract.go's own variantForExt routing.
	".mjs": {core.KindJSFunc, "js"},
	".cjs": {core.KindJSFunc, "js"},
	".mts": {core.KindJSFunc, "ts"},
	".cts": {core.KindJSFunc, "ts"},
	".rs":  {core.KindRustFn, "rust"},
	".cs":  {core.KindCSharpClass, "cs"},

	// LB-Q-ruby: Ruby's other source spellings — Rake task files, Rack config
	// (config.ru) and the Rakefile basename. Gemfile, *.gemspec, Podfile and
	// *.podspec are the Ruby DSL manifests' (LB-Q-gemfile-podfile), not claimed here.
	".rake":    {core.KindRubyMethod, "ruby"},
	".ru":      {core.KindRubyMethod, "ruby"},
	"Rakefile": {core.KindRubyMethod, "ruby"},

	// Kotlin (wave P2, ADR-021 §4 new-language breadth — closes the O69
	// gap: a real Kotlin repo previously indexed near-empty with no
	// disclosure). .kts (Kotlin script, e.g. build.gradle.kts) shares the
	// same grammar/extractor as .kt.
	".kt":  {core.KindKotlinClass, "kotlin"},
	".kts": {core.KindKotlinClass, "kotlin"},

	// HTML (LB-Q-html, language-breadth: new — a fleet gap, no incumbent).
	".html": {core.KindHTMLId, "html"},
	".htm":  {core.KindHTMLId, "html"},

	// CSS (LB-Q-css, language-breadth: new — a fleet gap, no incumbent).
	".css": {core.KindCSSSelector, "css"},

	// SCSS (LB-Q-scss, LB-F465: P233/tree-sitter-scss, wave lf5). The indented
	// .sass syntax is not classified (the grammar parses SCSS only); it is the
	// queue's own entry `sass`, disclosed unsupported in the generated table
	// until a grammar passes the 5% ERROR-file gate (O853, LB-Q-sass).
	".scss": {core.SourceKind("SCSSClass"), "scss"},

	// Vue and Svelte components (LB-Q-vue, LB-Q-svelte, wave lf7): the file is
	// its component's module symbol, its <script> TypeScript or JavaScript the
	// file hosts (F2).
	".vue":    {core.SourceKind("VueComponent"), "vue"},
	".svelte": {core.SourceKind("SvelteComponent"), "svelte"},

	// Astro (LB-Q-astro, wave lf7: virchau13/tree-sitter-astro). A component
	// is its file's module symbol; its frontmatter and scripts are TypeScript
	// the dialect hosts (F2), its styles CSS.
	".astro": {core.SourceKind("AstroComponent"), "astro"},

	// C
	".c": {core.KindCFunc, "c"},
	".h": {core.KindCFunc, "c"},

	// C++
	".cc":  {core.KindCppFunc, "cpp"},
	".cpp": {core.KindCppFunc, "cpp"},
	".cxx": {core.KindCppFunc, "cpp"},
	".hpp": {core.KindCppFunc, "cpp"},
	".hh":  {core.KindCppFunc, "cpp"},
	".hxx": {core.KindCppFunc, "cpp"},

	// CMake (docs/plans/cpp-toolchain.md CT-5; the intent layer of a C++
	// build — build.ninja is its generated output and is never indexed)
	"CMakeLists.txt": {core.KindCMakeTarget, "cmake"},
	".cmake":         {core.KindCMakeTarget, "cmake"},

	// Meson (LB-Q-meson, language-breadth: new, no incumbent). meson.build is
	// the build definition; meson.options (Meson 1.1+) and meson_options.txt
	// declare its options. Every other file named *.build is unclaimed.
	"meson.build":       {core.KindMesonTarget, "meson"},
	"meson.options":     {core.KindMesonTarget, "meson"},
	"meson_options.txt": {core.KindMesonTarget, "meson"},

	// Bazel / Starlark (LB-Q-bazel, wave lang4: a new tags dialect, no incumbent).
	// BUILD files declare targets, .bzl files define rules and macros, and
	// MODULE.bazel and WORKSPACE declare the modules and repositories.
	".bzl":             {core.SourceKind("BazelTarget"), "bazel"},
	"BUILD":            {core.SourceKind("BazelTarget"), "bazel"},
	"BUILD.bazel":      {core.SourceKind("BazelTarget"), "bazel"},
	"MODULE.bazel":     {core.SourceKind("BazelTarget"), "bazel"},
	"WORKSPACE":        {core.SourceKind("BazelTarget"), "bazel"},
	"WORKSPACE.bazel":  {core.SourceKind("BazelTarget"), "bazel"},
	"WORKSPACE.bzlmod": {core.SourceKind("BazelTarget"), "bazel"},

	// Tiltfile (LB-Q-tiltfile, wave lang4: a new tags dialect on the Starlark grammar, no incumbent).
	"Tiltfile": {core.SourceKind("TiltModule"), "tiltfile"},

	// Clang compilation database (LB-Q-compile-commands, language-breadth: new,
	// a doc-engine tool format, no incumbent). The basename check
	// (kindForFile/classify) takes precedence over ".json" (the generic JSON
	// extractor), so this entry governs every compile_commands.json at any depth.
	"compile_commands.json": {core.KindCompileUnit, "compile-commands"},

	// Templates
	".templ": {core.KindTemplComponent, "templ"},

	// Helm (YAML-family syntax; distinguished at the Kind level, not Lang)
	"Chart.yaml":  {core.KindHelmChart, "yaml"},
	"values.yaml": {core.KindHelmValue, "yaml"},
	".tpl":        {core.KindHelmTemplate, "yaml"},

	// Kustomize (ADR-021 §2 / TI-3; own basename keys — same precedent as
	// Chart.yaml/values.yaml above). LB-Q-kustomize: the doc-engine kustomize
	// dialect owns both basenames (language "kustomize", not "yaml"), so the
	// file's own entity is the kustomization; a bare `Kustomization` file (no
	// extension) is not claimed.
	"kustomization.yaml": {core.KindKustomization, "kustomize"},
	"kustomization.yml":  {core.KindKustomization, "kustomize"},

	// Docker
	"Dockerfile":          {core.KindDockerStage, "dockerfile"},
	".dockerfile":         {core.KindDockerStage, "dockerfile"},
	"docker-compose.yml":  {core.KindComposeService, "yaml"},
	"docker-compose.yaml": {core.KindComposeService, "yaml"},
	"compose.yml":         {core.KindComposeService, "yaml"},
	"compose.yaml":        {core.KindComposeService, "yaml"},

	// Protocol Buffers
	".proto": {core.KindProtoMessage, "proto"},

	// GraphQL
	".graphql": {core.KindGraphQLType, "graphql"},
	".gql":     {core.KindGraphQLType, "graphql"},

	// Prisma schema (LB-Q-prisma, language-breadth: new -- a tags-engine tool format, no incumbent).
	".prisma": {core.SourceKind("PrismaModel"), "prisma"},

	// TOML
	".toml": {core.KindTOMLTable, "toml"},

	// SQL (DDL)
	".sql": {core.KindSQLTable, "sql"},

	// JSON
	".json": {core.KindJSONKey, "json"},

	// npm/pnpm/yarn package manifest (LB-Q-package-json, language-breadth:
	// new — a doc-engine tool format, no incumbent). The basename check
	// (kindForFile/classify) takes precedence over ".json" above, so this
	// entry — not the generic JSON one — governs every "package.json" file.
	"package.json": {core.KindPackageJSONPackage, "package-json"},

	// Maven POM (LB-Q-maven, language-breadth: new — a doc-engine tool
	// format, no incumbent). The basename check (kindForFile/classify)
	// takes precedence over any ".xml" classification, so this entry
	// governs every "pom.xml" file; other XML (pom-shaded.xml, settings.xml,
	// Ant and Android files) is not claimed.
	"pom.xml": {core.KindMavenArtifact, "maven"},

	// Cargo manifest (LB-Q-cargo, language-breadth: new — a doc-engine tool
	// format). The basename check (kindForFile/classify) takes precedence
	// over ".toml" (the generic TOML extractor), so this entry governs every
	// "Cargo.toml" file at any depth; .cargo/config.toml, rust-toolchain.toml,
	// clippy.toml, rustfmt.toml, deny.toml and Cargo.lock are not claimed.
	"Cargo.toml": {core.KindCargoPackage, "cargo"},

	// vcpkg manifests (LB-Q-vcpkg, language-breadth: new — a doc-engine tool
	// format). The basename check takes precedence over ".json" (the generic
	// JSON extractor), so these entries govern every "vcpkg.json" (a port or a
	// manifest-mode project) and "vcpkg-configuration.json" at any depth.
	"vcpkg.json":               {core.KindVcpkgPackage, "vcpkg"},
	"vcpkg-configuration.json": {core.KindVcpkgPackage, "vcpkg"},

	// Conan (LB-Q-conan, language-breadth: new — a tags+doc tool format). The
	// basename check takes precedence over ".py" (python), ".txt" and ".yml"
	// and ".lock", so these entries govern every conanfile.py (a recipe or a
	// test_package recipe), conanfile.txt, conan.lock and conandata.yml at any
	// depth.
	"conanfile.py":  {core.KindConanRecipe, "conan"},
	"conanfile.txt": {core.KindConanRecipe, "conan"},
	"conan.lock":    {core.KindConanRecipe, "conan"},
	"conandata.yml": {core.KindConanRecipe, "conan"},

	// MSBuild (LB-Q-msbuild, language-breadth: new — a doc-engine tool
	// format, no incumbent). The projects of the three .NET languages and the
	// props and targets files they import: every one an MSBuild <Project>
	// document. A project's sidecar (App.csproj.user), the solution files
	// (.sln, and .slnx, whose root is a <Solution>) and other XML
	// (packages.config, app.config) are not claimed.
	".csproj":  {core.KindMSBuildProject, "msbuild"},
	".fsproj":  {core.KindMSBuildProject, "msbuild"},
	".vbproj":  {core.KindMSBuildProject, "msbuild"},
	".props":   {core.KindMSBuildProject, "msbuild"},
	".targets": {core.KindMSBuildProject, "msbuild"},

	// Taskfile (LB-Q-taskfile, language-breadth: new, a doc-engine tool format,
	// no incumbent). The eight basenames go-task looks for, in either case and
	// with the .dist marker; the basename check (kindForFile/classify) takes
	// precedence over the generic YAML handling, which never claimed them. An
	// included file under another name (Taskfile.docker.yml, tasks/build.yml) is
	// not claimed: the file's name is what says it is a Taskfile.
	"Taskfile.yml":       {core.KindTaskfile, "taskfile"},
	"Taskfile.yaml":      {core.KindTaskfile, "taskfile"},
	"taskfile.yml":       {core.KindTaskfile, "taskfile"},
	"taskfile.yaml":      {core.KindTaskfile, "taskfile"},
	"Taskfile.dist.yml":  {core.KindTaskfile, "taskfile"},
	"Taskfile.dist.yaml": {core.KindTaskfile, "taskfile"},
	"taskfile.dist.yml":  {core.KindTaskfile, "taskfile"},
	"taskfile.dist.yaml": {core.KindTaskfile, "taskfile"},

	// Git hook managers (LB-Q-git-hooks-config, language-breadth: new, a
	// doc-engine tool format, no incumbent). lefthook's config in the eight
	// spellings it reads (lefthook.yml, .lefthook.yml, lefthook.yaml,
	// .lefthook.yaml and their -local overrides), pre-commit's config and a
	// hook repository's manifest (.pre-commit-hooks.yaml, the hooks a
	// config's `hooks[*].id` names). The basename check (kindForFile/classify)
	// takes precedence over the generic YAML handling, which never claimed
	// them. lefthook.json and lefthook.toml are not claimed (the dialect
	// decodes YAML).
	"lefthook.yml":            {core.KindLefthookHook, "git-hooks-config"},
	"lefthook.yaml":           {core.KindLefthookHook, "git-hooks-config"},
	".lefthook.yml":           {core.KindLefthookHook, "git-hooks-config"},
	".lefthook.yaml":          {core.KindLefthookHook, "git-hooks-config"},
	"lefthook-local.yml":      {core.KindLefthookHook, "git-hooks-config"},
	"lefthook-local.yaml":     {core.KindLefthookHook, "git-hooks-config"},
	".lefthook-local.yml":     {core.KindLefthookHook, "git-hooks-config"},
	".lefthook-local.yaml":    {core.KindLefthookHook, "git-hooks-config"},
	".pre-commit-config.yaml": {core.KindPreCommitHook, "git-hooks-config"},
	".pre-commit-hooks.yaml":  {core.KindPreCommitHookDef, "git-hooks-config"},

	// Python packaging (LB-Q-python-packaging, language-breadth: new — a
	// doc-engine tool format, no incumbent). The basename check
	// (kindForFile/classify) takes precedence over ".toml" (the generic
	// TOML classification above), so this entry — not the generic one —
	// governs every "pyproject.toml" file. requirements*.txt and
	// setup.cfg from the same queue entry are not classified here: the
	// doc engine decodes only YAML/JSON/TOML/XML (design §4), and neither
	// is one of those (§14.2 disposition, LB-Q-python-packaging-rest).
	"pyproject.toml": {core.KindPyProjectPackage, "python-packaging"},

	// Azure and AWS app frameworks (wave iac, LB-Q-bicep, LB-Q-serverless,
	// LB-Q-pulumi-yaml, LB-Q-aws-cdk). Bicep files by extension (a tags-engine
	// dialect); the Serverless Framework's serverless.yml, a Pulumi project file
	// and a CDK app's cdk.json by basename, which takes precedence over the
	// generic YAML and JSON handling. An ARM template is a .json no name claims
	// (fusion reads it by its content), and a Pulumi stack file a glob's.
	".bicep":          {core.SourceKind("BicepResource"), "bicep"},
	".bicepparam":     {core.SourceKind("BicepParam"), "bicep"},
	"serverless.yml":  {core.SourceKind("ServerlessService"), "serverless"},
	"serverless.yaml": {core.SourceKind("ServerlessService"), "serverless"},
	"Pulumi.yaml":     {core.SourceKind("PulumiProject"), "pulumi-yaml"},
	"Pulumi.yml":      {core.SourceKind("PulumiProject"), "pulumi-yaml"},
	"cdk.json":        {core.SourceKind("CDKApp"), "aws-cdk"},

	// GitHub Actions composite and reusable action metadata (LB-F306): a
	// workflow is claimed by its .github/workflows/ glob; an action's metadata
	// file by its basename, anywhere in the tree.
	"action.yml":  {core.SourceKind("GHAActionDef"), "yaml"},
	"action.yaml": {core.SourceKind("GHAActionDef"), "yaml"},

	// Composer manifest (LB-Q-composer, language-breadth: new — a doc-engine
	// tool format, no incumbent), claimed by basename ahead of ".json" above.
	"composer.json": {core.SourceKind("ComposerPackage"), "json"},

	// Scala
	".scala": {core.KindScalaClass, "scala"},
	".sc":    {core.KindScalaClass, "scala"},

	// PHP
	".php": {core.KindPhpClass, "php"},

	// Bash
	".sh":   {core.KindBashFunction, "sh"},
	".bash": {core.KindBashFunction, "sh"},

	// PowerShell (LB-Q-powershell, language-breadth: new — a fleet gap, no
	// incumbent). .psd1 is a module manifest (a hashtable data file): it is
	// classified as PowerShell like the other two, and simply defines nothing.
	".ps1":  {core.KindPowerShellFunction, "powershell"},
	".psm1": {core.KindPowerShellFunction, "powershell"},
	".psd1": {core.KindPowerShellFunction, "powershell"},

	// Makefile (LB-Q-makefile, language-breadth: new — a fleet gap, no
	// incumbent). The three basenames GNU make reads by default and the two
	// extensions for included fragments; Makefile.common and <name>.Makefile
	// are not claimed (the queue entry names Makefile, *.mk and GNUmakefile).
	"Makefile":    {core.KindMakeTarget, "makefile"},
	"makefile":    {core.KindMakeTarget, "makefile"},
	"GNUmakefile": {core.KindMakeTarget, "makefile"},
	".mk":         {core.KindMakeTarget, "makefile"},
	".make":       {core.KindMakeTarget, "makefile"},

	// Gradle build scripts in the Groovy DSL (LB-Q-gradle: a new tree-sitter
	// dialect, no incumbent): build.gradle, settings.gradle and script
	// plugins. A Kotlin DSL script (*.gradle.kts) stays Kotlin's (.kts).
	".gradle": {core.KindGradleTask, "gradle"},

	// Jenkins pipelines (LB-Q-jenkinsfile: a tree-sitter dialect on the groovy
	// grammar, no incumbent): a Jenkinsfile by its basename and a *.jenkinsfile.
	// A shared library's vars/<name>.groovy is the dialect's by its manifest
	// glob only, so it carries no language tag (.groovy stays unsupported).
	"Jenkinsfile":  {core.SourceKind("JenkinsPipeline"), "jenkinsfile"},
	".jenkinsfile": {core.SourceKind("JenkinsPipeline"), "jenkinsfile"},

	// Lua
	".lua": {core.KindLuaFunction, "lua"},

	// Luau (LB-Q-luau, wave lang1): the Lua dialect's patterns on the Luau grammar, plus type aliases.
	".luau": {core.SourceKind("LuauFunction"), "luau"},

	// R (LB-Q-r, wave lang3): function assignments on r-lib/tree-sitter-r. Perl (LB-Q-perl): .pm by
	// extension, .pl and .t by content sniff (the dialect's match.sniff), so only .pm is listed here.
	".r":  {core.SourceKind("RFunction"), "r"},
	".R":  {core.SourceKind("RFunction"), "r"}, // the common spelling: the extension lookup is exact
	".pm": {core.SourceKind("PerlSub"), "perl"},

	// Jupyter notebooks (LB-Q-jupyter, language-breadth: new, a native dialect:
	// encoding/json over the notebook and the python dialect over its code cells).
	".ipynb": {core.SourceKind("JupyterFunc"), "jupyter"},

	// Elixir (LB-Q-elixir, wave lang3): a new tree-sitter dialect, no incumbent.
	".ex":  {core.SourceKind("ElixirModule"), "elixir"},
	".exs": {core.SourceKind("ElixirModule"), "elixir"},

	// Dart (LB-Q-dart: a new tree-sitter dialect, no incumbent).
	".dart": {core.KindDartClass, "dart"},

	// Emacs Lisp (LB-Q-emacs-lisp: a new tree-sitter dialect, no incumbent).
	".el": {core.KindEmacsLispFunction, "emacs-lisp"},

	// Common Lisp (LB-Q-commonlisp: a new tree-sitter dialect, no incumbent).
	// .asd is an ASDF system definition, a Lisp file the dialect claims and
	// extracts nothing from. .cl and .l are not claimed (OpenCL C and Lex).
	".lisp": {core.KindCommonLispFunction, "commonlisp"},
	".lsp":  {core.KindCommonLispFunction, "commonlisp"},
	".asd":  {core.KindCommonLispFunction, "commonlisp"},

	// Ada and VHDL (LB-Q-ada, LB-Q-vhdl: new tree-sitter dialects, no incumbent).
	// .ads (specification) and .adb (body) are GNAT's conventional Ada suffixes.
	".adb":  {core.KindAdaPackage, "ada"},
	".ads":  {core.KindAdaPackage, "ada"},
	".vhd":  {core.KindVhdlEntity, "vhdl"},
	".vhdl": {core.KindVhdlEntity, "vhdl"},

	// Swift (LB-Q-swift) is not classified: its dialect is built and measured
	// but refused (design §14.2, parse errors over the 5% line), so .swift
	// stays disclosed as unsupported (unsupported_language_gen.go).
}

// classify looks up base (full basename, e.g. "go.mod", "Dockerfile") first —
// so special basenames take precedence — then falls back to ext (leading-dot
// extension, e.g. ".go").
func classify(base, ext string) (classification, bool) {
	if c, ok := extClassMap[base]; ok {
		return c, true
	}
	c, ok := extClassMap[ext]
	return c, ok
}

// kindForFile returns the SourceKind for a file given its full basename and
// extension. The basename check (e.g. "go.mod") takes precedence over the
// extension check.
func kindForFile(base, ext string) core.SourceKind {
	c, _ := classify(base, ext)
	return c.Kind
}

// LanguageForPath returns the canonical language tag for path, derived from
// the exact same registry that assigns SourceKind (ADR-021 §1 / LT-1): one
// source of truth for "what language is this file", consumed by both the
// fusion pipeline's Kind assignment and presentation-layer callers like
// Client.Sources. Returns core.LangUnclassified when no extractor is
// registered for path's basename or extension — never a silent "".
//
// LanguageForPath does not inspect file content; a caller that also wants
// binary detection (content-based, overrides any extension-derived language)
// should additionally check IsBinary — see Client.Sources.
func LanguageForPath(path string) string {
	base := filepath.Base(path)
	ext := filepath.Ext(path)
	if c, ok := classify(base, ext); ok {
		return c.Lang
	}
	return core.LangUnclassified
}

// RegisteredExtensions returns the sorted set of extension/basename keys that
// have a language classification. It exists so the fusion package can assert,
// in a test, that its extractor registry (fusion.New's p.exts) matches this
// set exactly — the bijection that keeps "one registry" honest as extractors
// are added.
func RegisteredExtensions() []string {
	keys := make([]string, 0, len(extClassMap))
	for k := range extClassMap {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// skipDir reports whether a directory entry name should be skipped during
// tree walking. The list mirrors the DefaultIgnorePatterns described in the
// fusion pipeline design. .claude is tool state (agent worktrees under
// .claude/worktrees/ are full repo copies), same class as .idea/.vscode —
// indexing it duplicates every symbol in the service.
func skipDir(name string) bool {
	switch name {
	case "vendor", ".git", "testdata", "generated", "node_modules", ".idea", ".vscode", ".claude":
		return true
	}
	return false
}

// IsSkippedDir reports whether a directory NAME is one the connector always
// excludes from Discover walks (vendor, .git, testdata, generated,
// node_modules, .idea, .vscode, .claude — the same set skipDir prunes).
// Exported so corpus-reading consumers (internal/mcp's read_source) can
// honor the exact same exclusions when deciding whether a path is part of
// the indexed corpus, instead of maintaining a second list that drifts.
func IsSkippedDir(name string) bool { return skipDir(name) }
