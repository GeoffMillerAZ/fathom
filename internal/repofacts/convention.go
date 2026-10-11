package repofacts

import (
	_ "embed"
	"encoding/json"
	"path"
	"strings"
)

// The convention basis: the core table below (universal, K15: no framework
// and no Linguist data) plus the dialect-pack data in packs.json (each
// ecosystem's own test conventions). Its rules are path facts, applied in
// conventionRole's order, first match wins. IsTest, IsFixture and
// IsVendored are the same rules read one at a time: the eight product test
// rules and the two vendor classifiers RF-2 retired call them.

//go:embed packs.json
var packsJSON []byte

// pack is one ecosystem's framework conventions (packs.json).
type pack struct {
	Dialect         string   `json:"dialect"`
	Exts            []string `json:"exts"`
	TestSuffixes    []string `json:"test_suffixes"`
	TestPrefixes    []string `json:"test_prefixes"`
	TestInfixes     []string `json:"test_infixes"`
	TestDirs        []string `json:"test_dirs"`
	TestDirPrefixes []string `json:"test_dir_prefixes"`
	FixtureNames    []string `json:"fixture_names"`
	SupportDirs     []string `json:"support_dirs"`
}

// packByExt maps a lower-case extension to its pack. Built once at init
// from embedded data, read-only after.
var packByExt = loadPacks()

func loadPacks() map[string]*pack {
	var doc struct {
		Packs []*pack `json:"packs"`
	}
	if err := json.Unmarshal(packsJSON, &doc); err != nil {
		panic("repofacts: packs.json: " + err.Error())
	}
	m := map[string]*pack{}
	for _, p := range doc.Packs {
		for _, e := range p.Exts {
			m[strings.ToLower(e)] = p
		}
	}
	return m
}

// The core table's directory segments, matched case-sensitively unless
// noted. A segment names a directory, never the file's own base name.
var (
	vendorDirs    = map[string]bool{"vendor": true, "node_modules": true, "third_party": true}
	generatedDirs = map[string]bool{"generated": true, "__generated__": true}
	fixtureDirs   = map[string]bool{"testdata": true, "fixtures": true, "fixture": true, "__fixtures__": true, "__snapshots__": true}
	// testDirs are matched case-insensitively (test/, tests/, Tests/).
	testDirs      = map[string]bool{"test": true, "tests": true}
	migrationDirs = map[string]bool{"migrations": true, "migration": true, "migrate": true}
	scriptDirs    = map[string]bool{"scripts": true, "script": true, "hack": true}
	docsDirs      = map[string]bool{"docs": true, "doc": true}
	schemaDirs    = map[string]bool{"schema": true, "schemas": true}
	iacDirs       = map[string]bool{"terraform": true, "infra": true, "deploy": true, "k8s": true, "kubernetes": true, "helm": true, "charts": true, "manifests": true, "ansible": true}
)

// dirSegments returns rel's directory segments (every segment but the
// last). A path with a trailing slash names a directory: all its segments.
func dirSegments(rel string) []string {
	rel = strings.TrimPrefix(rel, "./")
	if strings.HasSuffix(rel, "/") {
		return strings.Split(strings.TrimSuffix(rel, "/"), "/")
	}
	i := strings.LastIndexByte(rel, '/')
	if i < 0 {
		return nil
	}
	return strings.Split(rel[:i], "/")
}

func anyDir(rel string, set map[string]bool, fold bool) bool {
	for _, s := range dirSegments(rel) {
		if fold {
			s = strings.ToLower(s)
		}
		if set[s] {
			return true
		}
	}
	return false
}

func hasDir(rel string, dirs []string) bool {
	for _, s := range dirSegments(rel) {
		for _, d := range dirs {
			if s == d {
				return true
			}
		}
	}
	return false
}

// IsVendored reports whether rel lies in a vendored tree by the core
// convention: a directory segment vendor, node_modules or third_party, at
// any depth. A bare segment ("vendor") is read as that directory itself, so
// a caller testing one directory name at a time gets the same answer.
func IsVendored(rel string) bool {
	if rel != "" && !strings.Contains(rel, "/") {
		return vendorDirs[rel]
	}
	return anyDir(rel, vendorDirs, false)
}

// IsFixture reports whether rel is test-support material by convention:
// under a fixture directory (testdata, fixtures, __snapshots__ ...), a
// pack's fixture file (conftest.py), or a file under a test directory
// (test/, tests/, a pack's support directory) that is not itself a test.
func IsFixture(rel string) bool {
	return conventionRole(rel, "", Attributes{}) == Fixture
}

// IsTest reports whether rel is a test file by convention: its base name
// matches its ecosystem's pack pattern (_test.go, test_*.py, *.test.ts,
// FooTest.java ...) or it is a pack file under a pack test directory
// (__tests__/, a crate's tests/), and it lies in no vendored, generated or
// fixture tree. It is the one test rule product code calls (RF-2).
func IsTest(rel string) bool {
	return conventionRole(rel, "", Attributes{}) == Test
}

// IsTestMaterial reports whether rel is a test file or test-support
// material (IsTest or IsFixture): code and data no product path runs, never
// a risk target, a served route or production search noise.
func IsTestMaterial(rel string) bool {
	r := conventionRole(rel, "", Attributes{})
	return r == Test || r == Fixture
}

// IsTestEntry reports whether a symbol is a test entry point: a Test-named
// function or method (Go's convention, which callgraph's test reach also
// reads) declared in a test file. A production TestHelper is not one.
func IsTestEntry(name, file string) bool {
	if i := strings.LastIndexAny(name, ".:"); i >= 0 {
		name = name[i+1:]
	}
	return strings.HasPrefix(name, "Test") && IsTest(file)
}

// packTest reports whether base, a file of pack p at rel, is a test by the
// pack's own patterns.
func packTest(p *pack, rel, base string) bool {
	for _, s := range p.TestSuffixes {
		if strings.HasSuffix(base, s) && len(base) > len(s) {
			return true
		}
	}
	for _, s := range p.TestPrefixes {
		if strings.HasPrefix(base, s) {
			return true
		}
	}
	for _, s := range p.TestInfixes {
		if strings.Contains(base, s) {
			return true
		}
	}
	if hasDir(rel, p.TestDirs) {
		return true
	}
	if len(p.TestDirPrefixes) > 0 && anyDir(rel, testDirs, true) {
		for _, s := range p.TestDirPrefixes {
			if strings.HasPrefix(base, s) && len(base) > len(s)+len(path.Ext(base)) {
				return true
			}
		}
	}
	return false
}

// conventionRole is the convention basis: the core table and the pack data,
// first match wins. attrs carries any explicit `-linguist-*` the attribute
// basis read, so a convention role the repository declared false is
// skipped.
func conventionRole(rel, dialect string, attrs Attributes) RoleName {
	rel = strings.TrimPrefix(rel, "./")
	base := path.Base(rel)
	ext := strings.ToLower(path.Ext(base))
	lowerBase := strings.ToLower(base)
	p := packByExt[ext]

	if attrs.allows(Vendored) && anyDir(rel, vendorDirs, false) {
		return Vendored
	}
	if attrs.allows(Generated) && anyDir(rel, generatedDirs, false) {
		return Generated
	}
	if anyDir(rel, fixtureDirs, false) {
		return Fixture
	}
	if p != nil {
		if packTest(p, rel, base) {
			return Test
		}
		for _, n := range p.FixtureNames {
			if base == n {
				return Fixture
			}
		}
		if hasDir(rel, p.SupportDirs) {
			return Fixture
		}
	}
	if anyDir(rel, testDirs, true) {
		return Fixture
	}
	if anyDir(rel, migrationDirs, false) || strings.Contains(rel, "alembic/versions/") {
		return Migration
	}
	if ciPath(rel, base) {
		return CI
	}
	if buildFile(base, lowerBase, ext) {
		return Build
	}
	dataFormat := dataExts[ext]
	if iacFile(base, lowerBase, ext) || (dataFormat && anyDir(rel, iacDirs, false)) {
		return IaC
	}
	if schemaExts[ext] || schemaName(lowerBase) || (dataFormat && anyDir(rel, schemaDirs, false)) {
		return Schema
	}
	if attrs.allows(Docs) && (docExts[ext] || docName(base) || (!codeFile(dialect, ext) && anyDir(rel, docsDirs, false))) {
		return Docs
	}
	if scriptExts[ext] || scriptLangs[dialect] || anyDir(rel, scriptDirs, false) {
		return Script
	}
	if configFile(base, lowerBase, ext) {
		return Config
	}
	if codeFile(dialect, ext) {
		return Source
	}
	return Unknown
}

// ciPath: a CI system's own configuration location.
func ciPath(rel, base string) bool {
	for _, pre := range []string{".github/workflows/", ".github/actions/", ".gitlab/ci/", ".circleci/", ".buildkite/", ".woodpecker/", ".husky/"} {
		if strings.HasPrefix(rel, pre) {
			return true
		}
	}
	switch base {
	case ".gitlab-ci.yml", ".gitlab-ci.yaml", "Jenkinsfile", ".travis.yml", "azure-pipelines.yml", "azure-pipelines.yaml",
		"bitbucket-pipelines.yml", ".drone.yml", "appveyor.yml", ".appveyor.yml", "cloudbuild.yaml", "cloudbuild.yml",
		".woodpecker.yml", "lefthook.yml", "lefthook.yaml", ".lefthook.yml", ".pre-commit-config.yaml", "codecov.yml", ".codecov.yml":
		return true
	}
	return false
}

// buildFile: a build tool's or package manager's own file.
func buildFile(base, lowerBase, ext string) bool {
	switch base {
	case "Makefile", "GNUmakefile", "makefile", "Justfile", "justfile", "Taskfile.yml", "Taskfile.yaml", "taskfile.yml", "taskfile.yaml",
		"Dockerfile", "Containerfile", "CMakeLists.txt", "build.gradle", "build.gradle.kts", "settings.gradle", "settings.gradle.kts",
		"gradle.properties", "pom.xml", "go.mod", "go.sum", "go.work", "go.work.sum", "package.json", "package-lock.json", "yarn.lock",
		"pnpm-lock.yaml", "pnpm-workspace.yaml", "Cargo.toml", "Cargo.lock", "pyproject.toml", "setup.py", "setup.cfg", "Pipfile",
		"Pipfile.lock", "poetry.lock", "uv.lock", "composer.json", "composer.lock", "Gemfile", "Gemfile.lock", "meson.build", "BUILD",
		"BUILD.bazel", "WORKSPACE", "WORKSPACE.bazel", "MODULE.bazel", "Rakefile", "build.sbt", "Directory.Build.props",
		"Directory.Build.targets", "configure.ac", "Makefile.am", "vcpkg.json", "conanfile.txt", "conanfile.py", "flake.nix", ".goreleaser.yml",
		".goreleaser.yaml", "tsconfig.json", "deno.json", "bun.lockb", "mix.exs", "pubspec.yaml", "Package.swift":
		return true
	}
	if strings.HasPrefix(base, "Dockerfile.") || strings.HasSuffix(base, ".Dockerfile") || strings.HasSuffix(lowerBase, ".dockerfile") ||
		strings.HasPrefix(base, "requirements") && ext == ".txt" || strings.HasPrefix(base, "tsconfig.") && ext == ".json" {
		return true
	}
	switch ext {
	case ".mk", ".cmake", ".bzl", ".gemspec", ".csproj", ".fsproj", ".vbproj", ".sln", ".props", ".targets", ".podspec":
		return true
	}
	return false
}

// iacFile: an infrastructure definition by its own file name or extension.
func iacFile(base, lowerBase, ext string) bool {
	switch ext {
	case ".tf", ".tfvars", ".bicep", ".tpl":
		return true
	}
	switch base {
	case "terragrunt.hcl", "docker-compose.yml", "docker-compose.yaml", "compose.yml", "compose.yaml", "kustomization.yaml",
		"kustomization.yml", "Chart.yaml", "Pulumi.yaml", "serverless.yml", "serverless.yaml", "Vagrantfile", "skaffold.yaml", "Tiltfile":
		return true
	}
	return strings.HasPrefix(lowerBase, "docker-compose.") && dataExts[ext]
}

var (
	dataExts   = map[string]bool{".yaml": true, ".yml": true, ".json": true, ".hcl": true, ".toml": true, ".cue": true}
	schemaExts = map[string]bool{".proto": true, ".graphql": true, ".graphqls": true, ".gql": true, ".avsc": true, ".avdl": true,
		".thrift": true, ".fbs": true, ".capnp": true, ".xsd": true, ".dtd": true, ".jsonschema": true, ".prisma": true}
	docExts = map[string]bool{".md": true, ".mdx": true, ".markdown": true, ".rst": true, ".adoc": true, ".asciidoc": true,
		".org": true, ".rdoc": true, ".pod": true, ".texi": true}
	scriptExts = map[string]bool{".sh": true, ".bash": true, ".zsh": true, ".fish": true, ".ps1": true, ".psm1": true,
		".bat": true, ".cmd": true, ".bats": true}
	// scriptLangs are the shell languages (a dialect id or language tag) a
	// file with no script extension is read as a script by: a shebang file.
	scriptLangs = map[string]bool{"bash": true, "sh": true, "powershell": true}
	configExts  = map[string]bool{".toml": true, ".ini": true, ".cfg": true, ".conf": true, ".properties": true, ".yaml": true,
		".yml": true, ".json": true, ".jsonc": true, ".json5": true, ".xml": true, ".plist": true, ".env": true, ".hcl": true,
		".cue": true, ".editorconfig": true}
	// codeExts are the programming-language extensions the source rule
	// knows when the caller gives no language: each pack's, plus these.
	codeExts = map[string]bool{".h": true, ".hpp": true, ".hh": true, ".m": true, ".mm": true, ".vue": true, ".svelte": true,
		".html": true, ".htm": true, ".css": true, ".scss": true, ".sass": true, ".less": true, ".templ": true, ".sql": true,
		".r": true, ".jl": true, ".zig": true, ".nim": true, ".cr": true, ".groovy": true, ".pl": true, ".pm": true, ".tcl": true,
		".rego": true, ".ex": true, ".erl": true, ".hs": true, ".ml": true, ".clj": true, ".cljs": true, ".el": true, ".lisp": true,
		".adb": true, ".ads": true, ".vhd": true, ".vhdl": true, ".fs": true, ".vb": true, ".kts": true, ".asd": true, ".lsp": true}
	// codeLangs are the language tags (connector.LanguageForPath's and the
	// dialect ids of kind language) the source rule reads as code.
	codeLangs = map[string]bool{"go": true, "py": true, "js": true, "ts": true, "java": true, "cs": true, "ruby": true,
		"rust": true, "php": true, "scala": true, "kotlin": true, "c": true, "cpp": true, "lua": true, "dart": true, "ada": true,
		"vhdl": true, "commonlisp": true, "emacs-lisp": true, "html": true, "css": true, "templ": true, "sql": true, "rego": true,
		"swift": true, "matlab": true, "tcl": true, "vim": true, "python": true, "javascript": true, "typescript": true,
		"csharp": true}
)

func schemaName(lowerBase string) bool {
	if strings.HasSuffix(lowerBase, ".schema.json") || strings.HasSuffix(lowerBase, ".schema.yaml") || strings.HasSuffix(lowerBase, ".schema.yml") {
		return true
	}
	for _, p := range []string{"openapi.", "openapi-", "swagger."} {
		if strings.HasPrefix(lowerBase, p) {
			ext := path.Ext(lowerBase)
			return ext == ".yaml" || ext == ".yml" || ext == ".json"
		}
	}
	return false
}

func docName(base string) bool {
	upper := strings.ToUpper(strings.TrimSuffix(base, path.Ext(base)))
	switch upper {
	case "README", "LICENSE", "LICENCE", "CHANGELOG", "CHANGES", "CONTRIBUTING", "AUTHORS", "NOTICE", "COPYING", "HISTORY",
		"CODE_OF_CONDUCT", "SECURITY", "MAINTAINERS", "SUPPORT":
		return true
	}
	return false
}

func configFile(base, lowerBase, ext string) bool {
	if configExts[ext] {
		return true
	}
	if strings.HasPrefix(base, ".") && !strings.Contains(base[1:], "/") {
		return true
	}
	for _, s := range []string{".config.js", ".config.ts", ".config.mjs", ".config.cjs", ".config.mts", ".config.cts"} {
		if strings.HasSuffix(lowerBase, s) {
			return true
		}
	}
	switch base {
	case "CODEOWNERS", "OWNERS", "Procfile", "Brewfile":
		return true
	}
	return false
}

// codeFile: a programming-language file, by the caller's language when it
// gave one, else by extension.
func codeFile(dialect, ext string) bool {
	if dialect != "" && codeLangs[dialect] {
		return true
	}
	return packByExt[ext] != nil || codeExts[ext]
}
