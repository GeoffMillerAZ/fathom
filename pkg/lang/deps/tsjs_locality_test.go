package deps_test

import (
	"context"
	"errors"
	"io/fs"
	"reflect"
	"slices"
	"testing"
	"testing/fstest"

	"github.com/geoffmilleraz/fathom/pkg/lang/deps"
)

func TestTSRepositoryPackageIdentity(t *testing.T) {
	const operand = "@invented/locality"
	for _, tc := range []struct {
		name, manifest, config, member, target string
		complete                               bool
	}{
		{name: "uninstalled tracked package", member: `{"name":"@invented/locality","main":"index.js"}`},
		{name: "pin cannot override tracked identity", manifest: `{"dependencies":{"@invented/locality":"1.2.3"}}`, member: `{"name":"@invented/locality"}`},
		{name: "unknown workspace target", manifest: `{"workspaces":["packages/*"],"dependencies":{"@invented/locality":"1.2.3"}}`, member: `{"name":"@invented/locality","exports":"./missing.ts"}`},
		{name: "self without supported export", manifest: `{"name":"@invented/locality","dependencies":{"@invented/locality":"1.2.3"}}`},
		{name: "self export", manifest: `{"name":"@invented/locality","exports":"./entry.ts"}`, target: "entry.ts", complete: true},
		{name: "workspace existing alias", manifest: `{"workspaces":["packages/*"]}`, config: `{"compilerOptions":{"moduleResolution":"bundler","paths":{"@invented/locality":["./packages/member/entry.ts"]}}}`, member: `{"name":"@invented/locality"}`, target: "packages/member/entry.ts", complete: true},
		{name: "absent external control", manifest: `{"dependencies":{"@invented/locality":"1.2.3"}}`, target: "npm:@invented/locality", complete: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			src := []byte("import {\n  value\n} from '" + operand + "';\n")
			tree := fstest.MapFS{"main.ts": {Data: src}}
			for name, content := range map[string]string{"package.json": tc.manifest, "tsconfig.json": tc.config, "packages/member/package.json": tc.member} {
				if content != "" {
					tree[name] = &fstest.MapFile{Data: []byte(content)}
				}
			}
			if tc.target != "" && tc.target != "npm:"+operand {
				tree[tc.target] = &fstest.MapFile{}
			}
			r := &deps.Reader{}
			got, err := r.ReadFile(t.Context(), tree, deps.Input{Path: "main.ts"}, src)
			if err != nil {
				t.Fatal(err)
			}
			if got.Complete != tc.complete {
				t.Errorf("complete = %v, want %v: %+v", got.Complete, tc.complete, got)
			}
			want := []string(nil)
			if tc.target != "" {
				want = []string{tc.target}
			}
			if !slices.Equal(tsjsTargets(got, deps.KindImport), want) {
				t.Errorf("imports = %v, want %v", tsjsTargets(got, deps.KindImport), want)
			}
			if !tc.complete && !slices.ContainsFunc(got.Unknowns, func(u deps.Unknown) bool { return u.Code == deps.UnknownUnresolvedAlias }) {
				t.Errorf("missing locality unknown: %+v", got.Unknowns)
			}
			batch, err := r.ReadFiles(t.Context(), tree, []deps.Input{{Path: "main.ts"}})
			if err != nil || len(batch) != 1 || !reflect.DeepEqual(got, batch[0]) {
				t.Fatalf("single/batch mismatch: %+v, %v", batch, err)
			}
			again, err := r.ReadFile(t.Context(), tree, deps.Input{Path: "main.ts"}, src)
			if err != nil || !reflect.DeepEqual(got, again) {
				t.Fatal("nondeterministic read")
			}
		})
	}
}

type tsLocalityFS struct {
	fstest.MapFS
	blocked       string
	cancel        context.CancelFunc
	manifestReads int
}

func (v *tsLocalityFS) ReadDir(name string) ([]fs.DirEntry, error) {
	if name == v.blocked {
		return nil, fs.ErrPermission
	}
	if name == "packages" && v.cancel != nil {
		v.cancel()
	}
	return v.MapFS.ReadDir(name)
}

func (v *tsLocalityFS) Open(name string) (fs.File, error) {
	if name == v.blocked {
		return nil, fs.ErrPermission
	}
	if name == "packages" && v.cancel != nil {
		v.cancel()
	}
	if name == "packages/member/package.json" {
		v.manifestReads++
	}
	return v.MapFS.Open(name)
}

func TestTSRepositoryInventoryRefusal(t *testing.T) {
	for _, tc := range []struct{ name, blocked, manifest string }{
		{"unreadable directory", "packages", `{}`},
		{"unreadable manifest", "packages/member/package.json", `{}`},
		{"malformed manifest", "", `{`},
		{"invalid name", "", `{"name":42}`},
		{"duplicate identity", "", `{"name":"@invented/absent","name":"@invented/other"}`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			src := []byte("import '@invented/absent';")
			view := &tsLocalityFS{MapFS: fstest.MapFS{
				"main.ts":                      {Data: src},
				"package.json":                 {Data: []byte(`{"dependencies":{"@invented/absent":"1.2.3"}}`)},
				"packages/member/package.json": {Data: []byte(tc.manifest)},
			}, blocked: tc.blocked}
			got, err := (&deps.Reader{}).ReadFile(t.Context(), view, deps.Input{Path: "main.ts"}, src)
			if err != nil || got.Complete || len(tsjsTargets(got, deps.KindImport)) != 0 || len(got.Unknowns) == 0 {
				t.Fatalf("inventory refusal: %+v, %v", got, err)
			}
		})
	}
}

func TestTSRepositoryInventoryBatchAndCancellation(t *testing.T) {
	src := []byte("import '@invented/absent'; import '@invented/other';")
	view := &tsLocalityFS{MapFS: fstest.MapFS{
		"a.ts": {Data: src}, "b.ts": {Data: src},
		"package.json":                 {Data: []byte(`{"dependencies":{"@invented/absent":"1.2.3","@invented/other":"2.3.4"}}`)},
		"packages/member/package.json": {Data: []byte(`{"name":"@invented/unrelated"}`)},
	}}
	r := &deps.Reader{}
	files, err := r.ReadFiles(t.Context(), view, []deps.Input{{Path: "b.ts"}, {Path: "a.ts"}})
	if err != nil || len(files) != 2 || !files[0].Complete || !files[1].Complete {
		t.Fatalf("batch: %+v, %v", files, err)
	}
	if view.manifestReads != 1 {
		t.Fatalf("inventory manifest read %d times, want once per batch", view.manifestReads)
	}
	// Inventory belongs to a call, never the Reader or the underlying FS identity.
	view.MapFS["packages/member/package.json"].Data = []byte(`{"name":"@invented/absent"}`)
	got, err := r.ReadFile(t.Context(), view, deps.Input{Path: "a.ts"}, src)
	if err != nil || got.Complete {
		t.Fatalf("stale batch cache: %+v, %v", got, err)
	}
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	view.cancel = cancel
	_, err = r.ReadFile(ctx, view, deps.Input{Path: "a.ts"}, src)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("cancellation = %v", err)
	}
}

func TestTSRepositoryInventoryConfined(t *testing.T) {
	for _, mode := range []fs.FileMode{fs.ModeSymlink, fs.ModeIrregular} {
		view := fstest.MapFS{
			"package.json": {Data: []byte(`{"dependencies":{"@invented/absent":"1.2.3"}}`)},
			"hidden":       {Mode: mode, Data: []byte("../outside")},
		}
		got, err := (&deps.Reader{}).ReadFile(t.Context(), view, deps.Input{Path: "main.ts"}, []byte("import '@invented/absent';"))
		if err != nil || got.Complete || len(tsjsTargets(got, deps.KindImport)) != 0 {
			t.Fatalf("uninspectable inventory entry: %+v, %v", got, err)
		}
	}
}

func TestTSLocalityDoesNotChangeJavaScript(t *testing.T) {
	view := fstest.MapFS{
		"package.json":                 {Data: []byte(`{"dependencies":{"@invented/locality":"1.2.3"}}`)},
		"packages/member/package.json": {Data: []byte(`{"name":"@invented/locality"}`)},
	}
	got, err := (&deps.Reader{}).ReadFile(t.Context(), view, deps.Input{Path: "main.js"}, []byte("require('@invented/locality');"))
	if err != nil || !got.Complete || !slices.Equal(tsjsTargets(got, deps.KindImport), []string{"npm:@invented/locality"}) {
		t.Fatalf("JavaScript product path changed: %+v, %v", got, err)
	}
}
