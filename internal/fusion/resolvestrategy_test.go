package fusion

import (
	"path"
	"reflect"
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
// today JavaScript/TypeScript (one extractor group), Python, and CSS
// (LB-Q-css, relative-path via its own @import captures).
func TestFileResolveLangsFromManifests(t *testing.T) {
	langs, err := extractor.FileResolveLangs()
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]string{}
	for _, l := range langs {
		got[l.Lang] = l.Strategy
	}
	want := map[string]string{"javascript": "relative-path", "python": "dotted-module", "css": "relative-path"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("FileResolveLangs = %v, want %v", got, want)
	}
	for path, lang := range map[string]string{"a.ts": "javascript", "a.tsx": "javascript", "a.mjs": "javascript", "a.cts": "javascript", "a.py": "python", "a.css": "css", "a.rb": "", "a.kt": "", "a.go": ""} {
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
	specs := map[string][]extractor.ImportSpec{
		"lib/a.rb": {
			{LocalName: "b", Module: "b", Kind: extractor.ImportWhole},
			{LocalName: "sub/c", Module: "sub/c", Kind: extractor.ImportWhole},
		},
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
