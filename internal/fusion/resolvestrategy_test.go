package fusion

import (
	"encoding/json"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/extractor"
)

// newTestEnv is a resolution universe over paths, with manifests (Cargo.toml,
// composer.json, Package.swift, pubspec.yaml, compile_commands.json) read
// from files.
func newTestEnv(root string, paths []string, files map[string]string) *resolveEnv {
	return newResolveEnv(root, paths, func(p string) ([]byte, bool) {
		s, ok := files[p]
		return []byte(s), ok
	})
}

// pwshParams is the powershell manifest's import parameters.
var pwshParams = extractor.ImportParams{Relative: true, SourceRoots: []string{""}, Separator: `\`, DirVars: []string{"$PSScriptRoot"}}

// TestResolveStrategies is LB-F11: every strategy of design §6's closed set
// resolves (importer, module spec, repo file set) to target files by its
// manifest parameters, and resolves nothing it cannot settle.
func TestResolveStrategies(t *testing.T) {
	type tc struct {
		name     string
		strategy string
		params   extractor.ImportParams
		paths    []string
		files    map[string]string
		importer string
		module   string
		want     []string
	}
	cases := []tc{
		// relative-path (JS/TS defaults: today's resolveTSModule rule).
		{"rel/ts-ext", "relative-path", extractor.ImportParams{}, []string{"src/a.ts", "src/b.ts"}, nil, "src/a.ts", "./b", []string{"src/b.ts"}},
		{"rel/ts-index", "relative-path", extractor.ImportParams{}, []string{"src/a.ts", "lib/index.js"}, nil, "src/a.ts", "../lib", []string{"lib/index.js"}},
		{"rel/ts-bare", "relative-path", extractor.ImportParams{}, []string{"src/a.ts", "react.ts"}, nil, "src/a.ts", "react", nil},

		// relative-path's TS/JS rule follows TypeScript's node16/bundler
		// resolution (O145): an ESM spec names the emitted .js/.mjs/.cjs, and
		// the compiler finds the .ts/.tsx/.d.ts, .mts or .cts source it came from.
		{"rel/ts-esm-js-names-ts", "relative-path", extractor.ImportParams{}, []string{"src/a.ts", "src/b.ts"}, nil, "src/a.ts", "./b.js", []string{"src/b.ts"}},
		{"rel/ts-esm-js-names-tsx", "relative-path", extractor.ImportParams{}, []string{"src/a.ts", "src/c.tsx"}, nil, "src/a.ts", "./c.js", []string{"src/c.tsx"}},
		{"rel/ts-esm-js-names-dts", "relative-path", extractor.ImportParams{}, []string{"src/a.ts", "src/types.d.ts"}, nil, "src/a.ts", "./types.js", []string{"src/types.d.ts"}},
		{"rel/ts-esm-jsx-names-tsx", "relative-path", extractor.ImportParams{}, []string{"src/a.tsx", "src/view.tsx", "src/view.ts"}, nil, "src/a.tsx", "./view.jsx", []string{"src/view.tsx"}},
		{"rel/ts-esm-mjs-names-mts", "relative-path", extractor.ImportParams{}, []string{"src/a.mts", "src/b.mts"}, nil, "src/a.mts", "./b.mjs", []string{"src/b.mts"}},
		{"rel/ts-esm-cjs-names-cts", "relative-path", extractor.ImportParams{}, []string{"src/a.cts", "src/b.cts"}, nil, "src/a.cts", "../src/b.cjs", []string{"src/b.cts"}},
		{"rel/ts-prefers-source-over-emitted-js", "relative-path", extractor.ImportParams{}, []string{"src/a.ts", "src/b.ts", "src/b.js"}, nil, "src/a.ts", "./b.js", []string{"src/b.ts"}},
		{"rel/js-keeps-its-own-js", "relative-path", extractor.ImportParams{}, []string{"src/a.js", "src/b.ts", "src/b.js"}, nil, "src/a.js", "./b.js", []string{"src/b.js"}},
		{"rel/js-esm-js-names-ts", "relative-path", extractor.ImportParams{}, []string{"src/a.js", "src/b.ts"}, nil, "src/a.js", "./b.js", []string{"src/b.ts"}},
		{"rel/ts-dotted-basename-keeps-suffixes", "relative-path", extractor.ImportParams{}, []string{"src/a.ts", "src/app.service.ts"}, nil, "src/a.ts", "./app.service", []string{"src/app.service.ts"}},

		// Node's CommonJS directory resolution: `.` and `..` are relative, a
		// trailing slash names only a directory, the repository root is one,
		// and a directory's package.json main (TypeScript: typings, types,
		// then main) precedes its index file.
		{"rel/dir-dotdot", "relative-path", extractor.ImportParams{}, []string{"index.js", "test/app.js"}, nil, "test/app.js", "..", []string{"index.js"}},
		{"rel/dir-dot", "relative-path", extractor.ImportParams{}, []string{"lib/index.js", "lib/a.js"}, nil, "lib/a.js", ".", []string{"lib/index.js"}},
		{"rel/dir-root-trailing-slash", "relative-path", extractor.ImportParams{}, []string{"index.js", "test/acceptance/x.js"}, nil, "test/acceptance/x.js", "../../", []string{"index.js"}},
		{"rel/dir-root-no-slash", "relative-path", extractor.ImportParams{}, []string{"index.js", "test/acceptance/x.js"}, nil, "test/acceptance/x.js", "../..", []string{"index.js"}},
		{"rel/dir-trailing-slash-skips-file", "relative-path", extractor.ImportParams{}, []string{"lib.js", "lib/index.js", "app.js"}, nil, "app.js", "./lib/", []string{"lib/index.js"}},
		{"rel/dir-no-slash-file-first", "relative-path", extractor.ImportParams{}, []string{"lib.js", "lib/index.js", "app.js"}, nil, "app.js", "./lib", []string{"lib.js"}},
		{"rel/dir-package-main", "relative-path", extractor.ImportParams{}, []string{"pkg/src/main.js", "pkg/index.js", "app.js"},
			map[string]string{"pkg/package.json": `{"name":"pkg","main":"src/main.js"}`}, "app.js", "./pkg", []string{"pkg/src/main.js"}},
		{"rel/dir-package-main-extensionless", "relative-path", extractor.ImportParams{}, []string{"lib/express.js", "index.js", "test/app.js"},
			map[string]string{"package.json": `{"main":"./lib/express"}`}, "test/app.js", "..", []string{"lib/express.js"}},
		{"rel/dir-package-main-directory", "relative-path", extractor.ImportParams{}, []string{"lib/index.js", "index.js", "test/app.js"},
			map[string]string{"package.json": `{"main":"lib"}`}, "test/app.js", "../", []string{"lib/index.js"}},
		{"rel/dir-package-main-absent-falls-to-index", "relative-path", extractor.ImportParams{}, []string{"pkg/index.ts", "app.ts"},
			map[string]string{"pkg/package.json": `{"main":"dist/index.js"}`}, "app.ts", "./pkg", []string{"pkg/index.ts"}},
		{"rel/dir-package-types-for-ts", "relative-path", extractor.ImportParams{}, []string{"pkg/src/index.ts", "pkg/lib/main.js", "app.ts", "app2.js"},
			map[string]string{"pkg/package.json": `{"types":"src/index.d.ts","main":"lib/main.js"}`}, "app.ts", "./pkg", []string{"pkg/src/index.ts"}},
		{"rel/dir-package-main-for-js", "relative-path", extractor.ImportParams{}, []string{"pkg/src/index.ts", "pkg/lib/main.js", "app.ts", "app2.js"},
			map[string]string{"pkg/package.json": `{"types":"src/index.d.ts","main":"lib/main.js"}`}, "app2.js", "./pkg", []string{"pkg/lib/main.js"}},
		{"rel/dir-above-root", "relative-path", extractor.ImportParams{}, []string{"index.js"}, nil, "index.js", "..", nil},
		// No over-resolution: a .js spec whose only file is that .js resolves to
		// it from a TypeScript importer too; a directory with neither a
		// package.json entry that exists nor an index file stays unresolved;
		// TypeScript reads typings before types (readPackageJsonTypesFields).
		{"rel/ts-esm-js-names-real-js", "relative-path", extractor.ImportParams{}, []string{"src/a.ts", "src/b.js"}, nil, "src/a.ts", "./b.js", []string{"src/b.js"}},
		{"rel/dir-no-index-stays-unresolved", "relative-path", extractor.ImportParams{}, []string{"lib/util.js", "app.js"}, nil, "app.js", "./lib/", nil},
		{"rel/dir-root-no-index-stays-unresolved", "relative-path", extractor.ImportParams{}, []string{"lib/x.js", "test/app.js"}, nil, "test/app.js", "..", nil},
		{"rel/dir-package-main-missing-no-index", "relative-path", extractor.ImportParams{}, []string{"pkg/src/x.js", "app.js"},
			map[string]string{"pkg/package.json": `{"main":"dist/index.js"}`}, "app.js", "./pkg", nil},
		{"rel/dir-package-typings-before-types", "relative-path", extractor.ImportParams{}, []string{"pkg/a.d.ts", "pkg/b.d.ts", "app.ts"},
			map[string]string{"pkg/package.json": `{"types":"b.d.ts","typings":"a.d.ts"}`}, "app.ts", "./pkg", []string{"pkg/a.d.ts"}},
		// A manifest that sets relative (HTML, CSS) keeps the literal rule: a
		// browser fetches what src names.
		{"rel/html-js-stays-literal", "relative-path", extractor.ImportParams{Relative: true}, []string{"site/index.html", "site/app.ts"}, nil, "site/index.html", "app.js", nil},
		{"rel/css-bare-relative", "relative-path", extractor.ImportParams{Relative: true, Suffixes: []string{".scss", ".css"}},
			[]string{"styles/main.scss", "styles/_vars.scss", "styles/base.css"}, nil, "styles/main.scss", "base", []string{"styles/base.css"}},
		{"rel/html-root-and-query", "relative-path", extractor.ImportParams{Relative: true},
			[]string{"site/index.html", "js/app.js"}, nil, "site/index.html", "/js/app.js?v=2", []string{"js/app.js"}},
		{"rel/url-never", "relative-path", extractor.ImportParams{Relative: true},
			[]string{"a.md", "example.com"}, nil, "a.md", "https://example.com", nil},

		// dotted-module.
		{"dotted/python-abs", "dotted-module", extractor.ImportParams{}, []string{"pkg/__init__.py", "pkg/mod.py", "app.py"}, nil, "app.py", "pkg.mod", []string{"pkg/mod.py"}},
		{"dotted/python-pkg", "dotted-module", extractor.ImportParams{}, []string{"pkg/__init__.py", "app.py"}, nil, "app.py", "pkg", []string{"pkg/__init__.py"}},
		{"dotted/python-rel", "dotted-module", extractor.ImportParams{}, []string{"pkg/a.py", "pkg/b.py", "c.py"}, nil, "pkg/a.py", ".b", []string{"pkg/b.py"}},
		{"dotted/python-rel-up", "dotted-module", extractor.ImportParams{}, []string{"pkg/sub/a.py", "pkg/c.py"}, nil, "pkg/sub/a.py", "..c", []string{"pkg/c.py"}},
		{"dotted/haskell-roots", "dotted-module", extractor.ImportParams{SourceRoots: []string{"src", ""}, Suffixes: []string{".hs"}},
			[]string{"src/Data/Graph.hs", "app/Main.hs"}, nil, "app/Main.hs", "Data.Graph", []string{"src/Data/Graph.hs"}},
		{"dotted/elixir-snake", "dotted-module", extractor.ImportParams{SourceRoots: []string{"lib"}, Suffixes: []string{".ex"}, Case: "snake"},
			[]string{"lib/my_app/foo_bar.ex", "lib/my_app.ex"}, nil, "lib/my_app.ex", "MyApp.FooBar", []string{"lib/my_app/foo_bar.ex"}},
		{"dotted/missing", "dotted-module", extractor.ImportParams{}, []string{"a.py"}, nil, "a.py", "os.path", nil},

		// namespace-dir.
		{"ns/java-class", "namespace-dir", extractor.ImportParams{Suffixes: []string{".java"}},
			[]string{"src/main/java/com/acme/Foo.java", "src/main/java/com/acme/Bar.java", "src/main/java/com/acme/app/App.java"}, nil,
			"src/main/java/com/acme/app/App.java", "com.acme.Foo", []string{"src/main/java/com/acme/Foo.java"}},
		{"ns/kotlin-package", "namespace-dir", extractor.ImportParams{Suffixes: []string{".kt"}},
			[]string{"src/main/kotlin/com/acme/util/Strings.kt", "src/main/kotlin/com/acme/util/Numbers.kt", "src/main/kotlin/com/acme/util/deep/X.kt"}, nil,
			"src/main/kotlin/com/acme/App.kt", "com.acme.util",
			[]string{"src/main/kotlin/com/acme/util/Numbers.kt", "src/main/kotlin/com/acme/util/Strings.kt"}},
		{"ns/php-psr4", "namespace-dir", extractor.ImportParams{Suffixes: []string{".php"}, Separator: `\`},
			[]string{"api/src/Http/Kernel.php", "api/src/App.php", "vendor/x/Kernel.php"},
			map[string]string{"api/composer.json": `{"autoload":{"psr-4":{"App\\":"src/"}}}`},
			"api/src/App.php", `\App\Http\Kernel`, []string{"api/src/Http/Kernel.php"}},
		{"ns/dart-package", "namespace-dir", extractor.ImportParams{Suffixes: []string{".dart"}},
			[]string{"app/lib/src/model.dart", "app/lib/main.dart"},
			map[string]string{"app/pubspec.yaml": "name: acme\nversion: 1.0.0\n"},
			"app/lib/main.dart", "package:acme/src/model.dart", []string{"app/lib/src/model.dart"}},
		{"ns/dart-foreign-package", "namespace-dir", extractor.ImportParams{Suffixes: []string{".dart"}},
			[]string{"app/lib/main.dart"}, map[string]string{"app/pubspec.yaml": "name: acme\n"},
			"app/lib/main.dart", "package:flutter/material.dart", nil},
		{"ns/dart-relative", "namespace-dir", extractor.ImportParams{Suffixes: []string{".dart"}},
			[]string{"app/lib/main.dart", "app/lib/widgets/card.dart"}, nil,
			"app/lib/main.dart", "widgets/card.dart", []string{"app/lib/widgets/card.dart"}},

		// include-path.
		{"inc/quoted-relative", "include-path", extractor.ImportParams{}, []string{"src/a.c", "src/b.h", "include/b.h"}, nil, "src/a.c", `"b.h"`, []string{"src/b.h"}},
		{"inc/compile-commands", "include-path", extractor.ImportParams{}, []string{"src/a.c", "third/include/foo/bar.h"},
			map[string]string{"compile_commands.json": `[{"directory":"/repo/build","command":"cc -I../third/include -isystem /usr/include -c ../src/a.c","file":"../src/a.c"}]`},
			"src/a.c", "<foo/bar.h>", []string{"third/include/foo/bar.h"}},
		{"inc/sibling-include", "include-path", extractor.ImportParams{}, []string{"src/sub/a.c", "src/include/util.h"}, nil, "src/sub/a.c", `"util.h"`, []string{"src/include/util.h"}},
		{"inc/angle-not-relative", "include-path", extractor.ImportParams{}, []string{"src/a.c", "src/stdio.h"}, nil, "src/a.c", "<stdio.h>", nil},

		// crate-mod.
		{"crate/mod-file", "crate-mod", extractor.ImportParams{}, rustTree, rustManifests, "src/lib.rs", "net", []string{"src/net.rs"}},
		{"crate/mod-dir", "crate-mod", extractor.ImportParams{}, rustTree, rustManifests, "src/lib.rs", "util", []string{"src/util/mod.rs"}},
		{"crate/nested-mod", "crate-mod", extractor.ImportParams{}, rustTree, rustManifests, "src/net.rs", "tcp", []string{"src/net/tcp.rs"}},
		{"crate/crate-path", "crate-mod", extractor.ImportParams{}, rustTree, rustManifests, "src/net/tcp.rs", "crate::util::helper", []string{"src/util/mod.rs"}},
		{"crate/super", "crate-mod", extractor.ImportParams{}, rustTree, rustManifests, "src/net/tcp.rs", "super::Conn", []string{"src/net.rs"}},
		{"crate/workspace-member", "crate-mod", extractor.ImportParams{}, rustTree, rustManifests, "src/lib.rs", "other_crate::api::Thing", []string{"crates/other/src/lib.rs"}},
		{"crate/external", "crate-mod", extractor.ImportParams{}, rustTree, rustManifests, "src/lib.rs", "std::fmt", nil},

		// swift-module.
		{"swift/target-path", "swift-module", extractor.ImportParams{}, swiftTree, swiftManifests, "Sources/App/main.swift", "Core",
			[]string{"Lib/Core/A.swift", "Lib/Core/Sub/B.swift"}},
		{"swift/default-dir", "swift-module", extractor.ImportParams{}, swiftTree, swiftManifests, "Lib/Core/A.swift", "App", []string{"Sources/App/main.swift"}},
		{"swift/submodule-kind", "swift-module", extractor.ImportParams{}, swiftTree, swiftManifests, "Sources/App/main.swift", "struct Core.Thing",
			[]string{"Lib/Core/A.swift", "Lib/Core/Sub/B.swift"}},
		{"swift/system", "swift-module", extractor.ImportParams{}, swiftTree, swiftManifests, "Sources/App/main.swift", "Foundation", nil},

		// file-symbol.
		{"file/ruby-relative", "file-symbol", extractor.ImportParams{Relative: true, Suffixes: []string{".rb"}, SourceRoots: []string{"lib", ""}},
			[]string{"lib/a/b.rb", "lib/a/c.rb"}, nil, "lib/a/b.rb", "c", []string{"lib/a/c.rb"}},
		{"file/ruby-root", "file-symbol", extractor.ImportParams{Relative: true, Suffixes: []string{".rb"}, SourceRoots: []string{"lib", ""}},
			[]string{"lib/a/b.rb", "app/x.rb"}, nil, "app/x.rb", "a/b", []string{"lib/a/b.rb"}},
		{"file/lua-dotted-init", "file-symbol", extractor.ImportParams{Separator: ".", Suffixes: []string{".lua"}, IndexFiles: []string{"init.lua"}},
			[]string{"net/http/init.lua", "main.lua"}, nil, "main.lua", "net.http", []string{"net/http/init.lua"}},
		{"file/bash-dot-slash", "file-symbol", extractor.ImportParams{},
			[]string{"scripts/run.sh", "scripts/lib.sh"}, nil, "scripts/run.sh", "./lib.sh", []string{"scripts/lib.sh"}},
		// PowerShell (LB-Q-powershell L3, wave lf6): `\` is a path separator
		// too, and $PSScriptRoot names the importing script's directory.
		{"file/pwsh-psscriptroot", "file-symbol", pwshParams,
			[]string{"tests/a.Tests.ps1", "tests/helpers/h.ps1", "helpers/h.ps1"}, nil, "tests/a.Tests.ps1", `$PSScriptRoot\helpers\h.ps1`, []string{"tests/helpers/h.ps1"}},
		{"file/pwsh-psscriptroot-braces-case", "file-symbol", pwshParams,
			[]string{"src/m.psm1", "bin/Mod.psd1"}, nil, "src/m.psm1", `${psscriptroot}/../bin/Mod.psd1`, []string{"bin/Mod.psd1"}},
		{"file/pwsh-psscriptroot-never-root", "file-symbol", pwshParams,
			[]string{"tests/a.ps1", "h.ps1"}, nil, "tests/a.ps1", `$PSScriptRoot\h.ps1`, nil},
		{"file/pwsh-dot-backslash", "file-symbol", pwshParams,
			[]string{"src/a.ps1", "src/Helper.ps1"}, nil, "src/a.ps1", `.\Helper.ps1`, []string{"src/Helper.ps1"}},
		{"file/pwsh-bare-then-root", "file-symbol", pwshParams,
			[]string{"src/a.ps1", "lib/x.psm1"}, nil, "src/a.ps1", `lib\x.psm1`, []string{"lib/x.psm1"}},
		{"file/pwsh-other-variable", "file-symbol", pwshParams,
			[]string{"src/a.ps1", "src/x.ps1"}, nil, "src/a.ps1", `$here\x.ps1`, nil},
		{"file/bazel-absolute", "file-symbol", extractor.ImportParams{Labels: "bazel"},
			[]string{"tools/defs.bzl", "pkg/BUILD"}, nil, "pkg/BUILD", "//tools:defs.bzl", []string{"tools/defs.bzl"}},
		{"file/bazel-local", "file-symbol", extractor.ImportParams{Labels: "bazel"},
			[]string{"pkg/local.bzl", "pkg/BUILD"}, nil, "pkg/BUILD", ":local.bzl", []string{"pkg/local.bzl"}},
		{"file/bazel-external", "file-symbol", extractor.ImportParams{Labels: "bazel"},
			[]string{"x/y.bzl", "pkg/BUILD"}, nil, "pkg/BUILD", "@rules//x:y.bzl", nil},

		// Outside the closed set: go-package is native (reference only).
		{"go-package/native", "go-package", extractor.ImportParams{}, []string{"a.go", "b/b.go"}, nil, "a.go", "b", nil},
		{"unknown", "stack-graphs", extractor.ImportParams{}, []string{"a.x", "b.x"}, nil, "a.x", "./b.x", nil},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			env := newTestEnv("/repo", c.paths, c.files)
			got := resolveByStrategy(c.strategy, c.params, env, c.importer, c.module)
			if !reflect.DeepEqual(got, c.want) {
				t.Errorf("resolveByStrategy(%s, %q from %s) = %q, want %q", c.strategy, c.module, c.importer, got, c.want)
			}
		})
	}
}

var rustTree = []string{
	"src/lib.rs", "src/net.rs", "src/net/tcp.rs", "src/util/mod.rs",
	"crates/other/src/lib.rs",
}

var rustManifests = map[string]string{
	"Cargo.toml":              "[workspace]\nmembers = [\"crates/other\"]\n\n[package]\nname = \"root\"\n",
	"crates/other/Cargo.toml": "[package]\nname = \"other-crate\"\nversion = \"0.1.0\"\n\n[dependencies]\nname = \"not-this\"\n",
}

var swiftTree = []string{"Lib/Core/A.swift", "Lib/Core/Sub/B.swift", "Sources/App/main.swift"}

var swiftManifests = map[string]string{
	"Package.swift": `// swift-tools-version:5.9
import PackageDescription
let package = Package(
    name: "Demo",
    targets: [
        .target(name: "Core", dependencies: [.product(name: "X", package: "x")], path: "Lib/Core"),
        .executableTarget(name: "App", dependencies: ["Core"]),
    ]
)
`,
}

// TestResolveStrategiesKeepTSAndPython pins "TS/JS/Python resolution
// unchanged": the manifest-selected relative-path and dotted-module
// strategies agree with the pre-LB-F11 rules on every case below.
func TestResolveStrategiesKeepTSAndPython(t *testing.T) {
	tsPaths := []string{"src/a.ts", "src/b.ts", "src/c.tsx", "src/d.js", "src/e/index.ts", "src/f/index.jsx", "lib/g.mjs", "lib/h.cjs", "src/i.d.ts", "src/b"}
	tsSpecs := []string{"./b", "./c", "./d", "./e", "./f", "../lib/g", "../lib/h", "./i.d", "./b.ts", "react", "@scope/x", "./nope", "../../x", "./e/index"}
	pyPaths := []string{"app.py", "pkg/__init__.py", "pkg/mod.py", "pkg/sub/__init__.py", "pkg/sub/leaf.py", "pkg/sub/deep/x.py", "top.py"}
	pySpecs := []string{"pkg", "pkg.mod", "pkg.sub", "pkg.sub.leaf", ".", ".leaf", "..mod", "...top", "os", "pkg.nope", "..", ".deep.x", "top"}
	tsDialect, pyDialect := dialectImports(t, "javascript"), dialectImports(t, "python")
	check := func(strategy string, params extractor.ImportParams, paths, specs []string, old func(string, string, map[string]bool) (string, bool)) {
		set := map[string]bool{}
		for _, p := range paths {
			set[p] = true
		}
		env := newTestEnv("/repo", paths, nil)
		for _, importer := range paths {
			for _, spec := range specs {
				wantPath, ok := old(importer, spec, set)
				var want []string
				if ok {
					want = []string{wantPath}
				}
				if got := resolveByStrategy(strategy, params, env, importer, spec); !reflect.DeepEqual(got, want) {
					t.Errorf("%s: %q from %s = %q, want %q (pre-LB-F11)", strategy, spec, importer, got, want)
				}
			}
		}
	}
	check(tsDialect.Strategy, tsDialect.Params, tsPaths, tsSpecs, preF11ResolveTS)
	check(pyDialect.Strategy, pyDialect.Params, pyPaths, pySpecs, preF11ResolvePy)
}

func dialectImports(t *testing.T, lang string) extractor.FileResolveLang {
	t.Helper()
	langs, err := extractor.FileResolveLangs()
	if err != nil {
		t.Fatal(err)
	}
	for _, l := range langs {
		if l.Lang == lang {
			return l
		}
	}
	t.Fatalf("no file-resolve language %q in %v", lang, langs)
	return extractor.FileResolveLang{}
}

// TestFileResolveLangsFromManifests: the resolver's languages are exactly
// the tags dialects whose manifest names an imports query and a strategy —
// today JavaScript/TypeScript (one extractor group), Python, HTML, CSS, C,
// Java, Kotlin, C#, C++, PHP, Rust, Ruby, Dart, Swift, Protocol Buffers and
// Gradle (a Kotlin DSL script resolves as Kotlin: the key is the extension).
func TestFileResolveLangsFromManifests(t *testing.T) {
	langs, err := extractor.FileResolveLangs()
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]string{}
	for _, l := range langs {
		got[l.Lang] = l.Strategy
	}
	want := map[string]string{"javascript": "relative-path", "python": "dotted-module", "html": "relative-path", "css": "relative-path", "c": "include-path", "cpp": "include-path", "java": "namespace-dir", "csharp": "namespace-dir", "php": "namespace-dir", "rust": "crate-mod", "kotlin": "namespace-dir", "ruby": "file-symbol", "dart": "namespace-dir", "elixir": "namespace-dir", "swift": "swift-module", "protobuf": "file-symbol", "gradle": "namespace-dir"}
	want["cmake"], want["meson"] = "relative-path", "relative-path" // LB-Q-cmake, LB-Q-meson (wave cx): a line of its own, so the wave merges stay additive
	// LB-F132 (wave pc): Rego data imports resolve by package.
	want["rego"] = "rego-package"
	want["bicep"] = "relative-path" // LB-Q-bicep (wave iac): a module path and an import resolve to the file
	want["scala"] = "namespace-dir" // LB-F438 (wave lf4): Scala's imports.scm, "." between segments
	want["scss"] = "relative-path"  // LB-F465 (wave lf5): @use, @forward and @import, with Sass's partial rule

	// Wave lf6, item shell: source and . with a literal path (LB-Q-bash); dot-sourcing,
	// Import-Module, using module and a manifest's modules (LB-Q-powershell).
	want["bash"], want["powershell"] = "file-symbol", "file-symbol"

	want["lua"] = "file-symbol" // LB-Q-lua (wave lang1): require with a literal module name

	// LB-F536, LB-F537 (wave lf7): an injection host's <script> imports (F2).
	want["vue"], want["svelte"] = "relative-path", "relative-path"
	want["astro"] = "relative-path"      // LB-F545 (wave lf7): an injection host's ranges' imports, with no imports query of its own
	want["bazel"] = "file-symbol"        // LB-Q-bazel (wave lang4): load() labels, //pkg:file.bzl and :file.bzl
	want["objective-c"] = "include-path" // LB-Q-objective-c (wave lang6): #import and #include
	// LB-F580 (wave lang7): a type name binds the one file of its scope defining it.
	want["graphql"], want["prisma"] = "global-name", "global-name"
	want["thrift"] = "file-symbol" // LB-Q-thrift (wave lang7): include "x.thrift", beside the including file first
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("FileResolveLangs = %v, want %v", got, want)
	}
	if g := fileResolveLangFor("a.scss"); g != "scss" {
		t.Errorf("fileResolveLangFor(a.scss) = %q, want scss", g)
	}
	for path, lang := range map[string]string{"a.ts": "javascript", "a.tsx": "javascript", "a.mjs": "javascript", "a.cts": "javascript", "a.py": "python", "a.rb": "ruby", "a.rake": "ruby", "a.ru": "ruby", "a.kt": "kotlin", "a.go": "", "a.html": "html", "a.htm": "html", "a.css": "css", "a.c": "c", "a.cpp": "cpp", "a.hpp": "cpp", "a.java": "java", "a.cs": "csharp", "a.php": "php", "a.rs": "rust", "a.dart": "dart", "a.swift": "swift", "a.proto": "protobuf", "build.gradle": "gradle", "build.gradle.kts": "kotlin", "a.rego": "rego"} {
		if g := fileResolveLangFor(path); g != lang {
			t.Errorf("fileResolveLangFor(%s) = %q, want %q", path, g, lang)
		}
	}
}

// TestResolveImportsWildcardStrategies: an include/require/import that
// brings every name of the target into scope (include-path, file-symbol,
// swift-module, and any @import.wildcard) resolves a bare call to the one
// target file defining it; two defining it resolve nothing.
func TestResolveImportsWildcardStrategies(t *testing.T) {
	env := newTestEnv("/repo", []string{"lib/a.rb", "lib/b.rb", "lib/sub/c.rb", "lib/d.rb"}, nil)
	r := &importResolver{strategy: "file-symbol", params: extractor.ImportParams{Relative: true, Suffixes: []string{".rb"}}, env: env}
	sym := func(file, name string, calls ...string) core.SymbolNode {
		return core.SymbolNode{Service: "s", FilePath: file, Name: name, Kind: core.SourceKind("RubyMethod"), Calls: calls}
	}
	helper := sym("lib/b.rb", "helper")
	symbols := []core.SymbolNode{
		sym("lib/a.rb", "run", "helper", "shared", "other"),
		helper,
		sym("lib/b.rb", "shared"),
		sym("lib/sub/c.rb", "shared"),
		sym("lib/d.rb", "other"), // defined, but lib/a.rb never requires lib/d.rb
	}
	specs := map[string]extractor.FileSpecs{
		"lib/a.rb": {Imports: []extractor.ImportSpec{
			{LocalName: "b", Module: "b", Kind: extractor.ImportWhole},
			{LocalName: "sub/c", Module: "sub/c", Kind: extractor.ImportWhole},
		}},
	}
	edges, rep := resolveImports(r, env.paths, specs, symbols)
	if rep.FilesScanned != 1 || rep.ImportsParsed != 2 {
		t.Fatalf("report = %+v", rep)
	}
	if len(edges) != 1 || edges[0].ToSymbol != helper.Key() || edges[0].Kind != core.EdgeFileResolved {
		t.Fatalf("edges = %+v, want only run -> %s (shared is defined by both required files: ambiguous)", edges, helper.Key())
	}
}

// preF11ResolveTS is resolveTSModule as it stood before LB-F11, kept as the
// reference the relative-path strategy must reproduce.
func preF11ResolveTS(importerPath, spec string, fileSet map[string]bool) (string, bool) {
	if !strings.HasPrefix(spec, "./") && !strings.HasPrefix(spec, "../") {
		return "", false
	}
	exts := []string{".ts", ".tsx", ".js", ".jsx", ".mjs", ".cjs"}
	candidate := path.Clean(path.Join(path.Dir(importerPath), spec))
	if fileSet[candidate] {
		return candidate, true
	}
	for _, ext := range exts {
		if p := candidate + ext; fileSet[p] {
			return p, true
		}
	}
	for _, ext := range exts {
		if p := candidate + "/index" + ext; fileSet[p] {
			return p, true
		}
	}
	return "", false
}

// preF11ResolvePy is resolvePyModule as it stood before LB-F11.
func preF11ResolvePy(importerPath, spec string, fileSet map[string]bool) (string, bool) {
	dir := ""
	dotted := spec
	if strings.HasPrefix(spec, ".") {
		dots := 0
		for dots < len(spec) && spec[dots] == '.' {
			dots++
		}
		dotted = spec[dots:]
		dir = path.Dir(importerPath)
		for n := 1; n < dots; n++ {
			dir = path.Dir(dir)
		}
	}
	rel := strings.ReplaceAll(dotted, ".", "/")
	candidate := rel
	switch {
	case dir != "" && dir != "." && rel != "":
		candidate = dir + "/" + rel
	case dir != "" && dir != "." && rel == "":
		candidate = dir
	}
	candidate = path.Clean(candidate)
	if p := candidate + ".py"; fileSet[p] {
		return p, true
	}
	if p := candidate + "/__init__.py"; fileSet[p] {
		return p, true
	}
	return "", false
}

// TestResolveImportsReferenceRules (LB-F74) drives resolveImports with the
// references the php and rust references queries capture (their manifests
// name the queries once the integrator adds queries.references; until then
// the resolver's half is proven here, the extractor's in
// internal/extractor's TestExtractFileSpecs). PHP names resolve as
// nikic/PHP-Parser's NameResolver reads them: a use alias (`U`), the file's
// own namespace for an unqualified (`Thing`) or relative qualified name
// (`Sub\R`), and a fully qualified name as written (`\App\Abs\Q`); a name
// none of those binds (`Missing`) draws nothing. Rust's `use crate::model::User`
// binds User, so a field type reaches the struct and `User::new` its method.
func TestResolveImportsReferenceRules(t *testing.T) {
	t.Run("php", func(t *testing.T) {
		paths := []string{"src/Abs/Q.php", "src/App/Service.php", "src/App/Thing.php", "src/Model/User.php", "src/Sub/R.php"}
		env := newTestEnv("/repo", paths, nil)
		r := &importResolver{strategy: "namespace-dir", params: extractor.ImportParams{Separator: `\`, Suffixes: []string{".php"}}, env: env, pkgScope: true}
		sym := func(file, pkg, name string, line, end int) core.SymbolNode {
			return core.SymbolNode{Service: "s", Package: pkg, FilePath: file, Name: name, Kind: core.SourceKind("PhpClass"), Line: line, EndLine: end}
		}
		run := sym("src/App/Service.php", `App`, "Service.run", 5, 7)
		symbols := []core.SymbolNode{
			sym("src/Abs/Q.php", `App\Abs`, "Q", 3, 3),
			sym("src/App/Service.php", `App`, "Service", 4, 8), run,
			sym("src/App/Thing.php", `App`, "Thing", 3, 3),
			sym("src/Model/User.php", `App\Model`, "User", 3, 3),
			sym("src/Sub/R.php", `App\Sub`, "R", 3, 3),
		}
		specs := map[string]extractor.FileSpecs{"src/App/Service.php": {
			Imports: []extractor.ImportSpec{{LocalName: "U", Module: `App\Model\User`, Kind: extractor.ImportWhole, Line: 3}},
			Refs: []extractor.RefSpec{
				{Name: "Missing", Lines: []int{6}}, {Name: "Thing", Lines: []int{6}}, {Name: "U", Lines: []int{6}},
				{Name: `Sub\R`, Lines: []int{6}}, {Name: `\App\Abs\Q`, Lines: []int{6}},
			},
		}}
		edges, _ := resolveImports(r, paths, specs, symbols)
		name := map[string]string{}
		for _, sy := range symbols {
			name[sy.Key()] = sy.Package + `\` + sy.Name
		}
		var got []string
		for _, e := range edges {
			if e.FromSymbol != run.Key() {
				t.Errorf("edge from %s, want every edge from Service.run", e.FromSymbol)
			}
			got = append(got, name[e.ToSymbol])
		}
		want := []string{`App\Abs\Q`, `App\Model\User`, `App\Sub\R`, `App\Thing`}
		sort.Strings(got)
		if strings.Join(got, " ") != strings.Join(want, " ") {
			t.Errorf("edges to %v, want %v", got, want)
		}
	})
	t.Run("rust", func(t *testing.T) {
		paths := []string{"src/lib.rs", "src/model.rs"}
		env := newTestEnv("/repo", paths, map[string]string{"Cargo.toml": "[package]\nname = \"acme\"\n"})
		r := &importResolver{strategy: "crate-mod", env: env}
		sym := func(file, name string, line, end int) core.SymbolNode {
			return core.SymbolNode{Service: "s", Package: "src", FilePath: file, Name: name, Kind: core.SourceKind("RustStruct"), Line: line, EndLine: end}
		}
		holder, make_ := sym("src/lib.rs", "Holder", 3, 3), sym("src/lib.rs", "make", 4, 6)
		user, newFn := sym("src/model.rs", "User", 1, 1), sym("src/model.rs", "User.new", 3, 5)
		specs := map[string]extractor.FileSpecs{"src/lib.rs": {
			Imports: []extractor.ImportSpec{{LocalName: "User", ExportedName: "User", Module: "crate::model", Kind: extractor.ImportNamed, Line: 1}},
			Refs:    []extractor.RefSpec{{Name: "User", Lines: []int{3}}, {Name: "User::new", Lines: []int{5}}},
		}}
		edges, _ := resolveImports(r, paths, specs, []core.SymbolNode{holder, make_, user, newFn})
		got := map[string]bool{}
		for _, e := range edges {
			got[e.FromSymbol+" -> "+e.ToSymbol] = true
		}
		want := []string{holder.Key() + " -> " + user.Key(), make_.Key() + " -> " + newFn.Key()}
		for _, w := range want {
			if !got[w] {
				t.Errorf("missing %s; got %v", w, got)
			}
		}
		if len(got) != len(want) {
			t.Errorf("got %v, want %v", got, want)
		}
	})
}

// TestResolveIncludePath_CompileDB is LB-Q-c-rest step 1: a compile database's
// own flags decide where an include resolves, per translation unit first, in
// the compiler's order, in every spelling of the flags.
func TestResolveIncludePath_CompileDB(t *testing.T) {
	db := func(entries ...string) map[string]string {
		return map[string]string{"build/compile_commands.json": "[" + strings.Join(entries, ",") + "]"}
	}
	cmd := func(file, flags string) string {
		return `{"directory":"/repo/build","command":"cc ` + flags + ` -c ../` + file + `","file":"../` + file + `"}`
	}
	args := func(file string, flags ...string) string {
		b, _ := json.Marshal(append(append([]string{"cc"}, flags...), "-c", "../"+file))
		return `{"directory":"/repo/build","arguments":` + string(b) + `,"file":"../` + file + `"}`
	}
	type tc struct {
		name     string
		paths    []string
		files    map[string]string
		importer string
		module   string
		want     []string
	}
	cases := []tc{
		{"joined-I", []string{"src/a.c", "inc/x.h"}, db(cmd("src/a.c", "-I../inc")), "src/a.c", `"x.h"`, []string{"inc/x.h"}},
		{"separate-I", []string{"src/a.c", "inc/x.h"}, db(cmd("src/a.c", "-I ../inc")), "src/a.c", `"x.h"`, []string{"inc/x.h"}},
		{"arguments-array", []string{"src/a.c", "inc/x.h"}, db(args("src/a.c", "-I", "../inc")), "src/a.c", `<x.h>`, []string{"inc/x.h"}},
		{"joined-isystem", []string{"src/a.c", "sys/x.h"}, db(cmd("src/a.c", "-isystem../sys")), "src/a.c", `<x.h>`, []string{"sys/x.h"}},
		{"separate-idirafter", []string{"src/a.c", "after/x.h"}, db(cmd("src/a.c", "-idirafter ../after")), "src/a.c", `<x.h>`, []string{"after/x.h"}},
		{"joined-iquote", []string{"src/a.c", "q/x.h"}, db(cmd("src/a.c", "-iquote../q")), "src/a.c", `"x.h"`, []string{"q/x.h"}},
		// A quoted include searches -iquote before -I; an angle include never
		// sees -iquote.
		{"quote-order", []string{"src/a.c", "q/x.h", "i/x.h"}, db(cmd("src/a.c", "-I../i -iquote ../q")), "src/a.c", `"x.h"`, []string{"q/x.h"}},
		{"angle-skips-iquote", []string{"src/a.c", "q/x.h", "i/x.h"}, db(cmd("src/a.c", "-I../i -iquote ../q")), "src/a.c", `<x.h>`, []string{"i/x.h"}},
		{"angle-I-before-isystem", []string{"src/a.c", "s/x.h", "i/x.h"}, db(cmd("src/a.c", "-isystem ../s -I ../i")), "src/a.c", `<x.h>`, []string{"i/x.h"}},
		// The including file's own directory comes before any flag.
		{"own-dir-first", []string{"src/a.c", "src/x.h", "inc/x.h"}, db(cmd("src/a.c", "-I../inc")), "src/a.c", `"x.h"`, []string{"src/x.h"}},
		// A translation unit's own roots come before the union of every unit's.
		{"per-entry/a", []string{"a/m.c", "b/m.c", "ia/dup.h", "ib/dup.h"}, db(cmd("a/m.c", "-I../ia"), cmd("b/m.c", "-I../ib")), "b/m.c", `"dup.h"`, []string{"ib/dup.h"}},
		{"per-entry/b", []string{"a/m.c", "b/m.c", "ia/dup.h", "ib/dup.h"}, db(cmd("a/m.c", "-I../ia"), cmd("b/m.c", "-I../ib")), "a/m.c", `"dup.h"`, []string{"ia/dup.h"}},
		// A header no entry compiles resolves through the union.
		{"header-union", []string{"a/m.c", "a/h.h", "ib/dup.h"}, db(cmd("a/m.c", "-I../ib")), "a/h.h", `"dup.h"`, []string{"ib/dup.h"}},
		// A root outside the repository names nothing.
		{"outside-repo", []string{"src/a.c", "x.h"}, db(cmd("src/a.c", "-I/usr/include -I../../elsewhere")), "src/a.c", `<x.h>`, nil},
		// A database at the repo root is read too.
		{"root-db", []string{"src/a.c", "inc/x.h"}, map[string]string{"compile_commands.json": `[{"directory":"/repo","command":"cc -Iinc -c src/a.c","file":"src/a.c"}]`}, "src/a.c", `<x.h>`, []string{"inc/x.h"}},
		// The conventional in-repo build directories.
		{"builddir", []string{"src/a.c", "inc/x.h"}, map[string]string{"builddir/compile_commands.json": `[{"directory":"/repo/builddir","command":"cc -I../inc -c ../src/a.c","file":"../src/a.c"}]`}, "src/a.c", `<x.h>`, []string{"inc/x.h"}},
		{"_build", []string{"src/a.c", "inc/x.h"}, map[string]string{"_build/compile_commands.json": `[{"directory":"/repo/_build","command":"cc -I../inc -c ../src/a.c","file":"../src/a.c"}]`}, "src/a.c", `<x.h>`, []string{"inc/x.h"}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			env := newTestEnv("/repo", c.paths, c.files)
			got := resolveByStrategy("include-path", extractor.ImportParams{}, env, c.importer, c.module)
			if !reflect.DeepEqual(got, c.want) {
				t.Errorf("include %s from %s = %q, want %q", c.module, c.importer, got, c.want)
			}
		})
	}
}

// TestResolveIncludePath_BuildDirGlobs: a compile database in a build
// directory whose name varies (cmake-build-debug, out/build/<preset>) is found
// on disk, and one beyond the 1 MiB manifest cap is still read.
func TestResolveIncludePath_BuildDirGlobs(t *testing.T) {
	for _, dbDir := range []string{"cmake-build-debug", "out/build/x64-debug", "build"} {
		t.Run(dbDir, func(t *testing.T) {
			root := t.TempDir()
			write := func(rel, s string) {
				t.Helper()
				p := filepath.Join(root, filepath.FromSlash(rel))
				if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(p, []byte(s), 0o644); err != nil {
					t.Fatal(err)
				}
			}
			dir := filepath.Join(root, filepath.FromSlash(dbDir))
			// 1.2 MiB of other entries, then the one that matters.
			var b strings.Builder
			b.WriteString("[")
			for i := 0; i < 4000; i++ {
				fmt.Fprintf(&b, `{"directory":%q,"command":"cc -I/nowhere/%s/%d -c other%d.c","file":"other%d.c"},`, dir, strings.Repeat("p", 200), i, i, i)
			}
			fmt.Fprintf(&b, `{"directory":%q,"command":"cc -I%s -c %s","file":%q}]`, dir, filepath.Join(root, "inc"), filepath.Join(root, "src", "a.c"), filepath.Join(root, "src", "a.c"))
			if b.Len() < 1<<20 {
				t.Fatalf("fixture is %d bytes, want over the manifest cap", b.Len())
			}
			write(dbDir+"/compile_commands.json", b.String())
			write("src/a.c", "int a;\n")
			write("inc/x.h", "int x;\n")
			env := newResolveEnv(root, []string{"src/a.c", "inc/x.h"}, diskManifestReader(root))
			got := resolveByStrategy("include-path", extractor.ImportParams{}, env, "src/a.c", "<x.h>")
			if !reflect.DeepEqual(got, []string{"inc/x.h"}) {
				t.Errorf("<x.h> from src/a.c = %q, want [inc/x.h]", got)
			}
		})
	}
}

// TestResolveIncludePath_AncestorRoot: with no compile database, a quoted
// include rooted at an ancestor directory of the including file resolves
// (curl's lib/curlx/base64.c includes "curlx/base64.h", which is lib/curlx/
// base64.h), repo-relative, and is counted; an angle include never takes it.
func TestResolveIncludePath_AncestorRoot(t *testing.T) {
	paths := []string{"lib/curlx/base64.c", "lib/curlx/base64.h", "lib/x.c", "lib/curl_setup.h"}
	env := newTestEnv("/repo", paths, nil)
	if got := resolveByStrategy("include-path", extractor.ImportParams{}, env, "lib/curlx/base64.c", `"curlx/base64.h"`); !reflect.DeepEqual(got, []string{"lib/curlx/base64.h"}) {
		t.Errorf("quoted ancestor-root include = %q, want [lib/curlx/base64.h]", got)
	}
	if got := resolveByStrategy("include-path", extractor.ImportParams{}, env, "lib/curlx/base64.c", `"curl_setup.h"`); !reflect.DeepEqual(got, []string{"lib/curl_setup.h"}) {
		t.Errorf("quoted include of an ancestor's file = %q, want [lib/curl_setup.h]", got)
	}
	if got := resolveByStrategy("include-path", extractor.ImportParams{}, env, "lib/curlx/base64.c", `<curlx/base64.h>`); got != nil {
		t.Errorf("angle include took the ancestor root: %q", got)
	}
	if n := env.ancestorIncludes(); n != 2 {
		t.Errorf("ancestorIncludes() = %d, want 2 (each distinct include counted once)", n)
	}
	// Asked again, still two.
	resolveByStrategy("include-path", extractor.ImportParams{}, env, "lib/curlx/base64.c", `"curlx/base64.h"`)
	if n := env.ancestorIncludes(); n != 2 {
		t.Errorf("ancestorIncludes() after a repeat = %d, want 2", n)
	}
	// A file beside the includer is no ancestor-root hit.
	env = newTestEnv("/repo", []string{"a/x.c", "a/y.h"}, nil)
	resolveByStrategy("include-path", extractor.ImportParams{}, env, "a/x.c", `"y.h"`)
	if n := env.ancestorIncludes(); n != 0 {
		t.Errorf("a same-directory include counted as an ancestor-root hit: %d", n)
	}
}
