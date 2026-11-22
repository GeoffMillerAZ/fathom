package deps_test

import (
	"encoding/json"
	"io/fs"
	"reflect"
	"testing"
	"testing/fstest"

	"github.com/geoffmilleraz/fathom/pkg/lang/deps"
	"github.com/geoffmilleraz/fathom/pkg/lang/deps/internal/depbench"
)

func covObservation(files ...deps.FileDeps) depbench.Observation {
	return depbench.Observation{Files: files, Repeat: files, Relocated: files}
}

func TestCOVCensusSynthetic(t *testing.T) {
	t.Run("external-as-local-with-native-truth", func(t *testing.T) {
		f := deps.FileDeps{Path: "a.ts", Complete: true, Edges: []deps.Edge{{Kind: deps.KindImport, Raw: "@local/core/sub", To: "npm:@local/core/sub", Resolution: deps.ResolvedExternal, Version: "1.0.0"}}}
		o := covObservation(f)
		o.Truth = []depbench.TruthFile{{Path: f.Path}}
		got := depbench.COVScore(o, map[string]bool{"npm:@local/core": true})
		if got.B1 || got.ExternalAsLocal != 1 || got.CompleteWithOracle != 1 {
			t.Fatalf("local inventory must refute even native-covered external: %+v", got)
		}
		got = depbench.COVScore(o, map[string]bool{"npm:@local/cor": true})
		if !got.B1 || got.ExternalAsLocal != 0 {
			t.Fatal("package prefix collision")
		}
	})
	t.Run("inconclusive-is-not-positive-failure", func(t *testing.T) {
		for _, f := range []deps.FileDeps{{Path: "a", Complete: true}, {Path: "a", Complete: false}} {
			got := depbench.COVScore(covObservation(f), nil)
			if got.B1 || got.B2 || got.Missing != 0 || got.Wrong != 0 {
				t.Fatalf("inconclusive: %+v", got)
			}
		}
	})
	t.Run("two-blockers-and-prerequisite", func(t *testing.T) {
		sets := map[string][]string{"one": {"a", "b"}, "two": {"a", "b"}, "three": {"c"}, "ready": {}}
		steps := depbench.COVGreedy(sets, 100, 5)
		if len(steps) != 2 || steps[0].Bucket != "a" || steps[0].Unlock != 0 || !steps[0].Prerequisite || steps[1].Bucket != "b" || steps[1].Unlock != 2 || steps[0].Marginals["b"] != 0 || steps[1].Marginals["b"] != 2 {
			t.Fatalf("greedy must count conjunction and prerequisite: %+v", steps)
		}
	})
	t.Run("identity-requires-corresponding-external-edge", func(t *testing.T) {
		u := deps.Unknown{Code: deps.UnknownExternalUnpinned, Raw: "left", Line: 4}
		f := deps.FileDeps{Unknowns: []deps.Unknown{u}, Edges: []deps.Edge{{Raw: "left", Line: 4, To: "npm:left", Resolution: deps.ResolvedExternal}}}
		if !depbench.COVIdentityOnly(f) {
			t.Fatal("identity known")
		}
		f.Edges[0].Raw = "right"
		if depbench.COVIdentityOnly(f) {
			t.Fatal("unmatched claim")
		}
		f.Edges[0].Raw = "left"
		f.Unknowns = append(f.Unknowns, deps.Unknown{Code: deps.UnknownMissingConfig})
		if depbench.COVIdentityOnly(f) {
			t.Fatal("mixed blockers")
		}
		if depbench.COVIdentityOnly(deps.FileDeps{Complete: true}) {
			t.Fatal("complete is not identity blocked")
		}
	})
	t.Run("selected-closure-cycles-direct-directory-and-blockers", func(t *testing.T) {
		edge := func(to string, r deps.Resolution) deps.Edge { return deps.Edge{To: to, Resolution: r} }
		files := map[string]deps.FileDeps{
			"a.ts":              {Path: "a.ts", Measured: true, Complete: true, Edges: []deps.Edge{edge("b.ts", deps.ResolvedFile)}},
			"b.ts":              {Path: "b.ts", Measured: true, Complete: true, Edges: []deps.Edge{edge("a.ts", deps.ResolvedFile)}},
			"c.ts":              {Path: "c.ts", Measured: true, Complete: true, Edges: []deps.Edge{edge("data", deps.ResolvedDirectory)}},
			"data/config.json":  {Path: "data/config.json"},
			"data/nested/no.ts": {Path: "data/nested/no.ts", Measured: true},
			"d.ts":              {Path: "d.ts", Measured: true, Edges: []deps.Edge{edge("data/config.json", deps.ResolvedFile)}},
		}
		got := depbench.COVClosure([]string{"a.ts", "c.ts", "d.ts"}, files)
		if got.Selected != 3 || got.Complete != 1 || got.OnlyUnmeasured != 1 || got.MeasuredAndUnmeasured != 1 {
			t.Fatalf("closure denominators or recursive-directory error: %+v", got)
		}
	})
	t.Run("seal-allowlist-and-byte-identical-replay", func(t *testing.T) {
		o := covObservation(deps.FileDeps{Path: "secret/never-expose.ts", Complete: true})
		o.Truth = []depbench.TruthFile{{Path: o.Files[0].Path}}
		score := depbench.COVScore(o, nil)
		a := depbench.COVSeal(score)
		b := depbench.COVSeal(depbench.COVScore(o, nil))
		if string(a) != string(b) {
			t.Fatal("seal replay differs")
		}
		var fields map[string]json.RawMessage
		if err := json.Unmarshal(a, &fields); err != nil {
			t.Fatal(err)
		}
		want := map[string]bool{"files": true, "oracle_covered": true, "complete": true, "b1": true, "b2": true, "b3": true, "missing": true, "external_as_local": true, "wrong": true, "b2_inconclusive": true, "complete_without_oracle": true, "complete_with_oracle": true, "unknown_by_code": true}
		if len(fields) != len(want) {
			t.Fatalf("seal field count: %d", len(fields))
		}
		for k := range fields {
			if !want[k] {
				t.Fatalf("seal leaked field %q", k)
			}
		}
		if !reflect.DeepEqual(fields["files"], json.RawMessage("1")) {
			t.Fatal("seal files denominator")
		}
	})
}

func TestCOVBucketAccounting(t *testing.T) {
	files := []deps.FileDeps{
		{Path: "a.ts", Unknowns: []deps.Unknown{{Code: deps.UnknownMissingConfig, DecidableBy: []deps.DecidableBy{deps.DecideConfigFile}}, {Code: deps.UnknownMissingConfig}, {Code: deps.UnknownDynamicImport}}},
		{Path: "b.ts", Unknowns: []deps.Unknown{{Code: deps.UnknownMissingConfig}}},
		{Path: "ready.ts", Complete: true, Edges: []deps.Edge{{Kind: deps.KindImport, Raw: "extra", To: "x.ts", Resolution: deps.ResolvedFile}}},
	}
	o := covObservation(files...)
	o.Truth = []depbench.TruthFile{{Path: "b.ts"}, {Path: "ready.ts"}}
	shapes := map[string][]string{"a.ts": {"literal-import", "literal-import", "computed-import"}, "b.ts": {"literal-import"}}
	got := depbench.COVAnalyze(o, shapes, map[string]string{"a.ts": "native-config-gap"}, nil)
	if got.Score.Files != 3 || got.Score.OracleFiles != 2 || got.Score.B4Extra != 1 || got.Score.B4ComparableEdges != 1 || got.OracleGaps["native-config-gap"] != 1 {
		t.Fatalf("grade denominators: %+v", got)
	}
	if len(got.BlockingSets["a.ts"]) != 2 || len(got.Buckets) != 2 {
		t.Fatalf("blocking sets must deduplicate occurrences: %+v", got)
	}
	var found bool
	for _, b := range got.Buckets {
		if b.Code == deps.UnknownMissingConfig {
			found = true
			if b.Occurrences != 3 || b.Files != 2 || b.OracleFiles != 1 || b.SoleBlockerFiles != 1 || b.OracleOccurrences != 1 || b.OracleGaps["native-config-gap"] != 1 || b.DecidableBy[0] != deps.DecideConfigFile {
				t.Fatalf("bucket meaning: %+v", b)
			}
		}
	}
	if !found {
		t.Fatal("missing bucket")
	}
}

func TestCOVExactShapeBoundaries(t *testing.T) {
	cases := []struct {
		src  string
		u    deps.Unknown
		want string
	}{
		{"/** @type {import('x').Thing} */", deps.Unknown{Code: deps.UnknownUnsupportedConstruct, Line: 1}, "jsdoc-import"},
		{"/* prose about imports */", deps.Unknown{Code: deps.UnknownUnsupportedConstruct, Line: 1}, "unknown-shape"},
		{"from .core import thing", deps.Unknown{Code: deps.UnknownMissingConfig, Line: 1}, "python-from-import"},
		{"import './thing'", deps.Unknown{Code: deps.UnknownUnresolvedAlias, Line: 0}, "unknown-shape"},
		{"import thing from 'other'", deps.Unknown{Code: deps.UnknownExternalUnpinned, Line: 1}, "js-module-statement"},
	}
	for _, c := range cases {
		if got := covShape([]byte(c.src), c.u); got != c.want {
			t.Fatalf("shape got %q want %q", got, c.want)
		}
	}
}

func TestCOVInventoryNames(t *testing.T) {
	tree := map[string][]byte{
		"package.json":                []byte(`{"name":"@local/core"}`),
		"packages/child/package.json": []byte(`{"name":"child"}`),
		"src/local/__init__.py":       nil,
		"src/local/sub.py":            nil,
		"other.py":                    nil,
	}
	names := depbench.COVInventory(tree, []string{".", "src"})
	for _, name := range []string{"npm:@local/core", "npm:child", "python:local", "python:local.sub", "python:other"} {
		if !names[name] {
			t.Fatalf("inventory missing %s", name)
		}
	}
	if names["npm:@local"] || names["python:local.sub.py"] {
		t.Fatal("invalid inventory identity")
	}
}

func TestCOVB3RejectsChangedReplay(t *testing.T) {
	f := deps.FileDeps{Path: "a.ts", Complete: true}
	o := covObservation(f)
	o.Truth = []depbench.TruthFile{{Path: f.Path}}
	o.Relocated = []deps.FileDeps{{Path: f.Path, Complete: false}}
	got := depbench.COVScore(o, nil)
	if got.B3 {
		t.Fatal("nonidentical relocation accepted")
	}
}

func TestCOVFrozenView(t *testing.T) {
	tree := fstest.MapFS{"a.py": {Data: []byte("from lib import child\n")}, "lib/__init__.py": {}, "lib/child.py": {Data: []byte("value=1\n")}, "link": {Data: []byte("lib"), Mode: fs.ModeSymlink}}
	view := covView(tree)
	if err := fstest.TestFS(view, "a.py", "lib/__init__.py", "lib/child.py", "link"); err != nil {
		t.Fatal(err)
	}
	r := &deps.Reader{}
	a, err := r.ReadFiles(t.Context(), tree, []deps.Input{{Path: "a.py"}})
	if err != nil {
		t.Fatal(err)
	}
	b, err := r.ReadFiles(t.Context(), view, []deps.Input{{Path: "a.py"}})
	if err != nil {
		t.Fatal(err)
	}
	if string(asJSON(t, a)) != string(asJSON(t, b)) {
		t.Fatal("frozen view changed reader evidence")
	}
}

func TestCOVGreedyMultiplePrerequisites(t *testing.T) {
	steps := depbench.COVGreedy(map[string][]string{"one": {"a", "b", "c"}, "two": {"a", "b", "c"}}, 100, 5)
	if len(steps) != 3 || steps[0].Unlock != 0 || steps[1].Unlock != 0 || steps[2].Unlock != 2 {
		t.Fatalf("three blockers require two justified prerequisites: %+v", steps)
	}
}
