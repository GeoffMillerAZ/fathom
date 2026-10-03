package fusion

// LB-F223..LB-F227 (wave lh): TypeScript's module resolution with the
// importer's project in view, over the fixture repositories under
// testdata/ts* (ADR-011-neutral; the compiler's behaviour is the reference,
// written fresh).

import (
	"io/fs"
	"path/filepath"
	"reflect"
	"sort"
	"testing"
)

// tsFixtureEnv is a resolve over a fixture repository on disk, as
// ResolveFileEdges builds it: its TS/JS files the universe, its manifests read
// from disk, the checkout walked for the manifests no universe file sits below.
func tsFixtureEnv(t *testing.T, name string) *resolveEnv {
	t.Helper()
	dir, err := filepath.Abs(filepath.Join("testdata", name))
	if err != nil {
		t.Fatal(err)
	}
	var paths []string
	err = filepath.WalkDir(dir, func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		if scriptExt(p) != "" {
			rel, _ := filepath.Rel(dir, p)
			paths = append(paths, filepath.ToSlash(rel))
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	sort.Strings(paths)
	env := newResolveEnv(dir, paths, diskManifestReader(dir))
	env.walkRoot = diskListedRoot(dir)
	return env
}

type tsCase struct {
	importer, spec string
	want           []string
}

func checkTSCases(t *testing.T, env *resolveEnv, cases []tsCase) {
	t.Helper()
	l := dialectImports(t, "javascript")
	for _, c := range cases {
		got := resolveByStrategy(l.Strategy, l.Params, env, c.importer, c.spec)
		if !reflect.DeepEqual(got, c.want) {
			t.Errorf("%q from %s = %q, want %q", c.spec, c.importer, got, c.want)
		}
	}
}

// TestResolveTSModule_PathsBaseUrlRootDirs is LB-F223 and LB-F224: the
// importer's project (the nearest config that includes it, a solution's
// reference, an `extends` chain through an array and an in-repo package's
// config), its `paths` (an exact key, the longest prefix, ordered
// fallbacks, relative to the declaring config, `${configDir}` to the
// importer's), `baseUrl` alone and `rootDirs`.
func TestResolveTSModule_PathsBaseUrlRootDirs(t *testing.T) {
	env := tsFixtureEnv(t, "tspaths")
	main := "packages/app/src/main.ts"
	checkTSCases(t, env, []tsCase{
		// paths from the later array base, relative to its directory
		{main, "@app/core", []string{"packages/core/src/index.ts"}},
		// the longer prefix wins over @app/*
		{main, "@app/core/math", []string{"packages/core/src/math.ts"}},
		// an exact key wins over the pattern
		{main, "@app/special", []string{"packages/special/lib/entry.ts"}},
		// the first substitution is missing, the second resolves
		{main, "@fallback/core", []string{"packages/core/src/index.ts"}},
		// a bare spec that leaves the repository
		{main, "left-pad", nil},
		// extends "@acme/tsconfig/shared.json": a workspace package's config
		{"apps/web/src/page.ts", "@shared/util", []string{"shared/util.ts"}},
		{"apps/web/src/page.ts", "@local/thing", []string{"apps/web/local/thing.ts"}},
		// baseUrl with no paths
		{"baseonly/src/main.ts", "lib/math", []string{"baseonly/src/lib/math.ts"}},
		{"baseonly/src/main.ts", "lib/nope", nil},
		// rootDirs: src and templates are one directory
		{"rootdirs/src/view.ts", "./template", []string{"rootdirs/templates/template.ts"}},
		// a solution config's reference holds the importer
		{"solution/src/a.ts", "~lib/x", []string{"solution/src/lib/x.ts"}},
	})
	// The stale base's mapping lost to the later base, and the root project
	// does not hold apps/web (its own config does).
	if c := env.tsConfigFor(main); c == nil || c.path != "tsconfig.json" {
		t.Fatalf("project of %s = %+v, want tsconfig.json", main, c)
	}
	if c := env.tsConfigFor("solution/src/a.ts"); c == nil || c.path != "solution/tsconfig.lib.json" {
		t.Fatalf("project of solution/src/a.ts = %+v, want solution/tsconfig.lib.json", c)
	}
}

// TestResolveTSModule_WorkspaceExports is LB-F225 and LB-F226 on an npm
// workspace ({packages} form): a workspace package by name, its exports'
// conditions in key order against the importer's set (customConditions,
// import for an ES module, require for a .cts), subpath, pattern and null
// targets, its imports (`#internal/*`, a bare target), a package importing
// its own name, a declaration-only package, `./x.js` naming x.ts and a
// directory's index; and LB-F227: a target under outDir names its source.
func TestResolveTSModule_WorkspaceExports(t *testing.T) {
	env := tsFixtureEnv(t, "tsnpmws")
	app := "packages/app/src/main.ts"
	core := "packages/core/src/index.ts"
	checkTSCases(t, env, []tsCase{
		{app, "@ws/core", []string{core}},
		{app, "@ws/core/feature", []string{"packages/core/src/feature.ts"}},
		{app, "@ws/core/mode", []string{"packages/core/src/mode-esm.ts"}},
		{"packages/app/src/legacy.cts", "@ws/core/mode", []string{"packages/core/src/mode-cjs.ts"}},
		{app, "@ws/core/locales/fr.js", []string{"packages/core/src/locales/fr.ts"}},
		{app, "@ws/core/internal/secret", nil},
		{app, "@ws/core/missing", nil},
		{app, "@ws/types", []string{"packages/types/index.d.ts"}},
		{app, "./local.js", []string{"packages/app/src/local.ts"}},
		{app, "./dir", []string{"packages/app/src/dir/index.ts"}},
		{app, "left-pad", nil},
		{app, "#internal/secret", nil}, // the app's own package has no imports
		{core, "#internal/secret", []string{"packages/core/src/internal/secret.ts"}},
		{core, "#types", []string{"packages/types/index.d.ts"}},
		{core, "@ws/core/feature", []string{"packages/core/src/feature.ts"}},
	})
}

// TestResolveTSModule_PnpmWorkspace is LB-F225 on a pnpm workspace with a
// negated glob, under node10 (no config: typings, types, main, then the
// index; a subpath as a file), and LB-F227 for a main/types output.
func TestResolveTSModule_PnpmWorkspace(t *testing.T) {
	env := tsFixtureEnv(t, "tspnpm")
	beta := "libs/beta/index.ts"
	checkTSCases(t, env, []tsCase{
		{beta, "alpha", []string{"libs/alpha/src/index.ts"}},
		{beta, "alpha/src/util", []string{"libs/alpha/src/util.ts"}},
		{beta, "beta/other", []string{"libs/beta/other.ts"}},
		{"tools/script.ts", "alpha", []string{"libs/alpha/src/index.ts"}},
		{"tools/script.ts", "ignored", nil},
	})
}

// TestResolveTSModule_InMemoryRules: the rules no fixture above needs a tree
// for — lerna.json packages (and its default), a workspace seen only below
// the root that declares it, a name two members share, a config whose
// include or allowJs leaves the importer out, `#` and self-names under
// node10, and a config that does not parse.
func TestResolveTSModule_InMemoryRules(t *testing.T) {
	paths := []string{
		"packages/common/index.ts", "packages/common/util.ts", "packages/core/index.ts",
		"sample/app/src/main.ts", "sub/ws/a/index.ts", "sub/ws/b/index.ts", "other/x.ts",
		"dup/one/index.ts", "dup/two/index.ts", "js/lib.js", "js/main.js", "js/helper/index.js",
		"broken/src/a.ts", "self/src/index.ts", "self/src/b.ts",
	}
	files := map[string]string{
		"lerna.json":                   `{"version": "1.0.0"}`,
		"package.json":                 `{"name": "@root/core"}`,
		"packages/common/package.json": `{"name": "@acme/common"}`,
		"packages/core/package.json":   `{"name": "@root/core"}`,
		"sub/package.json":             `{"name": "sub-root", "workspaces": ["ws/*"]}`,
		"sub/ws/a/package.json":        `{"name": "ws-a"}`,
		"sub/ws/b/package.json":        `{"name": "ws-b"}`,
		"dup/package.json":             `{"workspaces": ["*"]}`,
		"dup/one/package.json":         `{"name": "twin"}`,
		"dup/two/package.json":         `{"name": "twin"}`,
		"js/tsconfig.json":             `{"compilerOptions": {"paths": {"h": ["./helper"]}}}`,
		"js/jsconfig.json":             `{"compilerOptions": {"paths": {"h": ["./helper"]}}}`,
		"broken/tsconfig.json":         `{ "compilerOptions": { "paths": { "x": ["./src/a.ts"] } `,
		"self/package.json":            `{"name": "self-pkg", "exports": {"./b": "./src/b.ts"}}`,
		"self/tsconfig.json":           `{"compilerOptions": {"module": "commonjs"}}`,
	}
	env := newTestEnv("/repo", paths, files)
	checkTSCases(t, env, []tsCase{
		// lerna.json with no packages: packages/* (the root package is not a member)
		{"sample/app/src/main.ts", "@acme/common", []string{"packages/common/index.ts"}},
		{"sample/app/src/main.ts", "@acme/common/util", []string{"packages/common/util.ts"}},
		{"sample/app/src/main.ts", "@root/core", []string{"packages/core/index.ts"}},
		// sub's workspace is seen only below sub/
		{"sub/ws/a/index.ts", "ws-b", []string{"sub/ws/b/index.ts"}},
		{"other/x.ts", "ws-b", nil},
		// a name two members share binds nothing
		{"dup/one/index.ts", "twin", nil},
		// a .js importer: tsconfig.json without allowJs does not hold it,
		// the jsconfig.json beside it does
		{"js/main.js", "h", []string{"js/helper/index.js"}},
		// an unparsable config is no config
		{"broken/src/a.ts", "x", nil},
		// node10 (module commonjs): no self-name, no # imports
		{"self/src/index.ts", "self-pkg/b", nil},
		{"self/src/index.ts", "#x", nil},
	})
	if c := env.tsConfigFor("js/main.js"); c == nil || c.path != "js/jsconfig.json" {
		t.Errorf("project of js/main.js = %+v, want js/jsconfig.json", c)
	}
}

// TestTSSpecRegexp pins the include/exclude glob dialect.
func TestTSSpecRegexp(t *testing.T) {
	for _, tc := range []struct {
		spec    string
		exclude bool
		path    string
		want    bool
	}{
		{"src", false, "src/a.ts", true},
		{"src", false, "src/deep/a.ts", true},
		{"src", false, "srcx/a.ts", false},
		{"pkg/**/*", false, "pkg/a.ts", true},
		{"pkg/**/*", false, "pkg/x/y/a.ts", true},
		{"**/*.ts", false, "a.ts", true},
		{"**/*.ts", false, "a/b.tsx", false},
		{"pkg/*.ts", false, "pkg/x/a.ts", false},
		{"pkg/a?.ts", false, "pkg/ab.ts", true},
		{"", false, "any/where.ts", true},
		{"node_modules", true, "node_modules/x/a.ts", true},
		{"pkg/test/**/*", true, "pkg/test/a.spec.ts", true},
		{"pkg/*.spec.ts", true, "pkg/a.spec.ts", true},
		{"pkg/*.spec.ts", true, "pkg/x/a.spec.ts", false},
		{"pkg/dist", true, "pkg/dist/index.d.ts", true},
		{"pkg/dist", true, "pkg/distx/a.ts", false},
	} {
		re := tsSpecRegexp(tc.spec, tc.exclude)
		if got := re != nil && re.MatchString(tc.path); got != tc.want {
			t.Errorf("tsSpecRegexp(%q, exclude=%v) on %q = %v, want %v", tc.spec, tc.exclude, tc.path, got, tc.want)
		}
	}
}

// TestParseOrderedJSON: JSONC (comments, trailing commas) parses, and an
// object's keys keep the order they are written in.
func TestParseOrderedJSON(t *testing.T) {
	n, ok := parseOrderedJSON([]byte(`{
  // a comment, "with a quote"
  "b": {"z": "1", "a": ["x", "y",],},
  /* block */ "a": "s // not a comment",
}`))
	if !ok {
		t.Fatal("JSONC did not parse")
	}
	if !reflect.DeepEqual(n.keys, []string{"b", "a"}) || !reflect.DeepEqual(n.get("b").keys, []string{"z", "a"}) {
		t.Fatalf("keys out of order: %v / %v", n.keys, n.get("b").keys)
	}
	if got := n.str2("a"); got != "s // not a comment" {
		t.Fatalf("string with // = %q", got)
	}
	if got := n.get("b").get("a").strings(); !reflect.DeepEqual(got, []string{"x", "y"}) {
		t.Fatalf("array = %v", got)
	}
	if _, ok := parseOrderedJSON([]byte(`{"a": `)); ok {
		t.Fatal("malformed JSON parsed")
	}
}

// TestComparePatternKeys: the longer part before `*` first, then the longer
// key; an exact key after a pattern of the same base.
func TestComparePatternKeys(t *testing.T) {
	keys := []string{"./*", "./a/*", "./a/*.js", "./a/b/", "./a/b/*"}
	sort.SliceStable(keys, func(i, j int) bool { return comparePatternKeys(keys[i], keys[j]) < 0 })
	want := []string{"./a/b/*", "./a/b/", "./a/*.js", "./a/*", "./*"}
	if !reflect.DeepEqual(keys, want) {
		t.Fatalf("order = %v, want %v", keys, want)
	}
}

// TestResolveTSModule_KeepsRelativeRule: the TS/JS rule's relative
// resolution (LB-F43, LB-F44) is unchanged by the project, and the JS
// dialect's parameters still select it.
func TestResolveTSModule_KeepsRelativeRule(t *testing.T) {
	l := dialectImports(t, "javascript")
	if l.Strategy != "relative-path" || l.Params.Relative || len(l.Params.Suffixes) > 0 || len(l.Params.IndexFiles) > 0 {
		t.Fatalf("javascript imports = %+v, want relative-path with the TS/JS rule", l)
	}
}
