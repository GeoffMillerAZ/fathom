package deps_test

import (
	"context"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"reflect"
	"slices"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/geoffmilleraz/fathom/pkg/lang/deps"
)

func readTSJSFixture(t *testing.T, repo fstest.MapFS, file, dialect, src string, opts ...deps.Option) deps.FileDeps {
	t.Helper()
	reader, err := deps.New(opts...)
	if err != nil {
		t.Fatal(err)
	}
	got, err := reader.ReadFile(context.Background(), repo, deps.Input{Path: file, Dialect: dialect}, []byte(src))
	if err != nil {
		t.Fatal(err)
	}
	return got
}

func tsjsTargets(got deps.FileDeps, kind deps.Kind) []string {
	var out []string
	for _, edge := range got.Edges {
		if edge.Kind == kind {
			out = append(out, edge.To)
		}
	}
	return out
}

func TestTSJSDependencyForms(t *testing.T) {
	deps.TSJSUseCandidateAdmissionForTest(t)
	for _, tc := range []struct {
		dialect, fixture string
		targets          []string
		dynamicLines     []int
	}{
		{"typescript", "forms.ts", []string{"side.ts", "types.ts", "namespace.ts", "equals.ts", "named.ts", "all.ts", "barrel.ts", "type-import.ts", "common.ts", "dynamic.ts", "template.ts", "nested.ts"}, []int{12, 13}},
		{"javascript", "forms.js", []string{"side.js", "named.js", "exported.js", "barrel.js", "namespace.js", "common.cjs", "resolved.cjs", "template.js", "nested.js"}, []int{9, 10}},
	} {
		t.Run(tc.dialect, func(t *testing.T) {
			repo := fstest.MapFS{}
			if tc.dialect == "typescript" {
				repo["tsconfig.json"] = &fstest.MapFile{Data: []byte(`{"compilerOptions":{"module":"commonjs"}}`)}
			}
			for _, name := range tc.targets {
				repo[name] = &fstest.MapFile{}
			}
			src, err := os.ReadFile("testdata/tsjs/" + tc.fixture)
			if err != nil {
				t.Fatal(err)
			}
			got := readTSJSFixture(t, repo, tc.fixture, "", string(src), deps.TrustUnmeasured(tc.dialect))
			if got.Dialect != tc.dialect || got.Complete || !reflect.DeepEqual(tsjsTargets(got, deps.KindImport), tc.targets) {
				t.Fatalf("imports: %+v", got)
			}
			var lines []int
			var grammarGap bool
			for _, unknown := range got.Unknowns {
				if tc.dialect == "typescript" && unknown.Code == deps.UnknownParseError && unknown.Line == 7 {
					grammarGap = true // The pinned grammar recovers the source of export type *.
					continue
				}
				if unknown.Code != deps.UnknownDynamicImport || unknown.Raw != "***" {
					t.Fatalf("unexpected unknown: %+v", unknown)
				}
				lines = append(lines, unknown.Line)
			}
			if !slices.Equal(lines, tc.dynamicLines) || tc.dialect == "typescript" && !grammarGap {
				t.Fatalf("computed import lines: %v", lines)
			}
		})
	}
}

func TestTSJSResolutionFixtures(t *testing.T) {
	deps.TSJSUseCandidateAdmissionForTest(t)
	repo := fstest.MapFS{
		"tsconfig.json": {Data: []byte(`{
		  // At equal prefix specificity, the first applicable mapping wins.
		  "compilerOptions": {
		    "moduleResolution": "bundler", "module": "esnext",
		    "paths": {"@ui/*": ["./missing/*", "./src/ui/*"], "@exact": ["./src/exact"]},
		  },
		}`)},
		"package.json": {Data: []byte(`{"name":"sample","type":"module","exports": {
		  "./feature":{"types":"./src/feature.d.ts","import":"./src/feature.mjs","default":"./src/fallback.mjs"}
		}}`)},
		"src/ui/button/index.ts": {},
		"src/exact.ts":           {},
		"src/widget.ts":          {},
		"src/widget.js":          {},
		"src/feature.d.ts":       {},
		"src/feature.mjs":        {},
		"src/fallback.mjs":       {},
	}
	got := readTSJSFixture(t, repo, "src/main.ts", "typescript", `import '@ui/button';
import '@exact';
import './widget.js';
export * from 'sample/feature';`, deps.TrustUnmeasured("typescript"))
	if !got.Complete || !slices.Equal(tsjsTargets(got, deps.KindImport), []string{"src/ui/button/index.ts", "src/exact.ts", "src/widget.ts", "src/feature.d.ts"}) {
		t.Fatalf("config, index, substitution and export precedence: %+v", got)
	}
	if !slices.Equal(tsjsTargets(got, deps.KindConfigRef), []string{"package.json", "tsconfig.json"}) {
		t.Fatalf("config inputs: %+v", got)
	}
}

func TestTSJSDependencyDisclosures(t *testing.T) {
	for _, tc := range []struct {
		name, file, src, config string
		code                    deps.UnknownCode
	}{
		{"malformed TS", "x.ts", "import { x from './target';", "", deps.UnknownParseError},
		{"malformed JS", "x.js", "require('./target'", "", deps.UnknownParseError},
		{"unsupported config", "x.ts", "import './target';", `{"extends":"./base.json"}`, deps.UnknownUnsupportedConstruct},
		{"malformed config", "x.ts", "import './target';", `{"compilerOptions":`, deps.UnknownParseError},
		{"symlink shadows fallback", "x.ts", "import './bad';", "", deps.UnknownUnsupportedConstruct},
		{"missing file", "x.js", "import './missing';", "", deps.UnknownUnresolvedAlias},
		{"relative escape", "x.js", "import '../../host';", "", deps.UnknownUnsupportedConstruct},
		{"require escape", "x.js", "const load = require; load('./target')", "", deps.UnknownUnsupportedConstruct},
		{"triple reference", "x.ts", "/// <reference path='./target.ts' />", "", deps.UnknownUnsupportedConstruct},
		{"JSDoc module", "x.js", "/** @type {import('./target').T} */ let a;", "", deps.UnknownUnsupportedConstruct},
	} {
		t.Run(tc.name, func(t *testing.T) {
			repo := fstest.MapFS{
				"target.ts": {},
				"bad.ts":    {Mode: fs.ModeSymlink, Data: []byte("target.ts")},
				"bad.js":    {},
			}
			if tc.config != "" {
				repo["tsconfig.json"] = &fstest.MapFile{Data: []byte(tc.config)}
			}
			got := readTSJSFixture(t, repo, tc.file, "", tc.src, deps.TrustUnmeasured("typescript"), deps.TrustUnmeasured("javascript"))
			if got.Complete || !slices.ContainsFunc(got.Unknowns, func(u deps.Unknown) bool { return u.Code == tc.code }) {
				t.Fatalf("want %s: %+v", tc.code, got)
			}
			if tc.name == "symlink shadows fallback" && len(tsjsTargets(got, deps.KindImport)) > 0 {
				t.Fatalf("unsafe higher-priority candidate bypassed: %+v", got)
			}
		})
	}
}

func TestTSJSPackageLock(t *testing.T) {
	repo := fstest.MapFS{
		"package.json": {Data: []byte(`{"dependencies":{"@sample/lib":"^1.2.0","other":"*"}}`)},
		"package-lock.json": {Data: []byte(`{"lockfileVersion":3,"packages":{
		  "":{"dependencies":{"@sample/lib":"^1.2.0","other":"*"}},
		  "node_modules/@sample/lib":{"version":"1.2.3"},
		  "node_modules/other":{"version":"2.0.0","link":true}
		}}`)},
	}
	got := readTSJSFixture(t, repo, "main.js", "", "require('@sample/lib'); require('other');", deps.TrustUnmeasured("javascript"))
	var pinned, unpinned bool
	for _, e := range got.Edges {
		pinned = pinned || e.To == "npm:@sample/lib" && e.Version == "1.2.3"
		unpinned = unpinned || e.To == "npm:other" && e.Version == ""
	}
	if !pinned || !unpinned || got.Complete || !slices.Contains(tsjsTargets(got, deps.KindConfigRef), "package-lock.json") {
		t.Fatalf("lock evidence: %+v", got)
	}
}

func TestTSJSLoaderUncertainty(t *testing.T) {
	for _, tc := range []struct {
		name, file, src string
	}{
		{"JavaScript shadow", "main.js", "function use(require) { return require('./target.js'); }"},
		{"TypeScript shadow", "main.ts", "const use = (require: (value: string) => unknown) => require('./target.js');"},
		{"assigned loader", "main.js", "require = other; require('./target.js');"},
		{"escaped loader", "main.js", `requ\u0069re('./target.js');`},
		{"module subscript", "main.js", `module["require"]("./target.js");`},
		{"computed global", "main.js", `globalThis[loader]("./target.js");`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			repo := fstest.MapFS{"target.js": {}}
			got := readTSJSFixture(t, repo, tc.file, "", tc.src, deps.TrustUnmeasured("typescript"), deps.TrustUnmeasured("javascript"))
			if got.Complete || len(tsjsTargets(got, deps.KindImport)) != 0 ||
				!slices.ContainsFunc(got.Unknowns, func(u deps.Unknown) bool { return u.Code == deps.UnknownUnsupportedConstruct }) {
				t.Fatalf("unproven loader yielded a binding: %+v", got)
			}
		})
	}
}

func TestTSJSCompilerPackageAmbiguity(t *testing.T) {
	repo := fstest.MapFS{
		"package.json":                          {Data: []byte(`{"dependencies":{"sample":"1.2.3","fs":"0.0.1"}}`)},
		"node_modules/@types/sample/index.d.ts": {},
	}
	got := readTSJSFixture(t, repo, "main.js", "", "require('sample');\nrequire('fs');", deps.TrustUnmeasured("javascript"))
	if got.Complete || !slices.Equal(tsjsTargets(got, deps.KindImport), []string{"node:fs"}) ||
		!slices.ContainsFunc(got.Unknowns, func(u deps.Unknown) bool { return u.Code == deps.UnknownUnsupportedConstruct }) {
		t.Fatalf("unproven package or built-in version: %+v", got)
	}
}

func TestTSJSResolutionCaps(t *testing.T) {
	t.Run("paths", func(t *testing.T) {
		paths := make(map[string][]string)
		for i := range 4097 {
			paths[fmt.Sprintf("input%d", i)] = []string{"./target"}
		}
		config, err := json.Marshal(map[string]any{"compilerOptions": map[string]any{"paths": paths}})
		if err != nil {
			t.Fatal(err)
		}
		repo := fstest.MapFS{"tsconfig.json": {Data: config}, "target.ts": {}}
		got := readTSJSFixture(t, repo, "main.ts", "", "import 'input1';", deps.TrustUnmeasured("typescript"))
		if got.Complete || len(tsjsTargets(got, deps.KindImport)) != 0 ||
			!slices.ContainsFunc(got.Unknowns, func(u deps.Unknown) bool { return u.Code == deps.UnknownUnsupportedConstruct && u.Line == 0 }) {
			t.Fatalf("oversize alias map: %+v", got)
		}
	})
	t.Run("exports", func(t *testing.T) {
		branch := strings.Repeat(`{"default":`, 16) + `"./target.js"` + strings.Repeat("}", 16)
		repo := fstest.MapFS{
			"package.json": {Data: []byte(`{"name":"sample","exports":` + branch + "}")},
			"target.js":    {},
		}
		got := readTSJSFixture(t, repo, "main.js", "", "require('sample');", deps.TrustUnmeasured("javascript"))
		if got.Complete || len(tsjsTargets(got, deps.KindImport)) != 0 ||
			!slices.ContainsFunc(got.Unknowns, func(u deps.Unknown) bool { return u.Code == deps.UnknownReadLimit }) {
			t.Fatalf("oversize exports nesting: %+v", got)
		}
	})
}

func TestTSJSDialectAdmission(t *testing.T) {
	repo := fstest.MapFS{}
	js := readTSJSFixture(t, repo, "main.js", "", "")
	if !js.Measured || js.Trusted || !js.Complete {
		t.Fatalf("JavaScript admission: %+v", js)
	}
	ts := readTSJSFixture(t, repo, "main.ts", "", "")
	if !ts.Measured || ts.Trusted || !ts.Complete || len(ts.Unknowns) != 0 {
		t.Fatalf("TypeScript admission: %+v", ts)
	}
	tsUnknown := readTSJSFixture(t, repo, "main.ts", "", "import(name);")
	if !tsUnknown.Measured || tsUnknown.Trusted || tsUnknown.Complete ||
		!slices.ContainsFunc(tsUnknown.Unknowns, func(u deps.Unknown) bool { return u.Code == deps.UnknownDynamicImport }) {
		t.Fatalf("TypeScript dynamic import completeness: %+v", tsUnknown)
	}
	resolver := tsjsTypedResolver(func(_ context.Context, request deps.ResolveRequest) deps.ResolveResult {
		if request.From != "main.js" || request.Raw != "./target.cjs" ||
			request.Kind != deps.KindImport || request.Line != 2 || request.Repo == nil {
			t.Fatalf("bad resolver request: %+v", request)
		}
		return deps.ResolveResult{Edges: []deps.Edge{{Kind: request.Kind, Resolution: deps.ResolvedFile, To: "other.cjs"}}}
	})
	repo["other.cjs"] = &fstest.MapFile{}
	custom := readTSJSFixture(t, repo, "main.js", "", "\nrequire('./target.cjs');", deps.WithResolver("javascript", resolver))
	if custom.Measured || custom.Trusted || custom.Complete ||
		!slices.Equal(tsjsTargets(custom, deps.KindImport), []string{"other.cjs"}) {
		t.Fatalf("override inherited measurement: %+v", custom)
	}
}

type tsjsTypedResolver func(context.Context, deps.ResolveRequest) deps.ResolveResult

func (f tsjsTypedResolver) Resolve(ctx context.Context, req deps.ResolveRequest) deps.ResolveResult {
	return f(ctx, req)
}
