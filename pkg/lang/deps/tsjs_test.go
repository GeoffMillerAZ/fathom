package deps_test

import (
	"context"
	"io/fs"
	"os"
	"reflect"
	"slices"
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
