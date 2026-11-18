package deps_test

import (
	"context"
	"embed"
	"errors"
	"io/fs"
	"reflect"
	"slices"
	"testing"
	"testing/fstest"

	"github.com/geoffmilleraz/fathom/pkg/lang"
	"github.com/geoffmilleraz/fathom/pkg/lang/deps"
)

//go:embed testdata/extension/dialects
var extensionPack embed.FS

func packFS() fs.FS {
	pack, err := fs.Sub(extensionPack, "testdata/extension")
	if err != nil {
		panic(err)
	}
	return pack
}

func TestDependencyPack(t *testing.T) {
	parser, err := lang.New(lang.WithDialects(packFS()))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = parser.Close() })
	kinds := map[string]deps.Kind{
		"loads": deps.KindImport, "includes": deps.KindInclude, "embeds": deps.KindEmbed,
		"inputs": deps.KindBuildInput, "runs": deps.KindExec, "reads": deps.KindConfigRef,
	}
	opt := deps.WithPack(parser, "task-spec", kinds)
	delete(kinds, "loads") // The option owns its configuration.
	path := "app/main.task.json"
	src := []byte(`{
		"name": "prepare",
		"imports": ["module.txt"],
		"includes": ["local.txt"],
		"embeds": ["picture.txt"],
		"sources": ["inputs"],
		"commands": ["run.sh"],
		"configs": ["settings.txt"]
	}`)
	repo := fstest.MapFS{
		path:                  {Data: src},
		"app/module.txt":      {},
		"app/local.txt":       {},
		"app/picture.txt":     {},
		"app/inputs/file.txt": {},
		"app/run.sh":          {},
		"app/settings.txt":    {},
		"app/outside.txt":     {Mode: fs.ModeSymlink, Data: []byte("module.txt")},
	}
	in := deps.Input{Path: path, Dialect: "task-spec"}
	for _, trust := range []bool{false, true} {
		opts := []deps.Option{opt}
		if trust {
			opts = append(opts, deps.TrustUnmeasured(in.Dialect))
		}
		reader, err := deps.New(opts...)
		if err != nil {
			t.Fatal(err)
		}
		file, err := reader.ReadFile(t.Context(), repo, in, src)
		if err != nil || file.Measured || file.Trusted != trust || file.Complete != trust {
			t.Fatalf("pack admission: %+v, %v", file, err)
		}
		if trust && len(file.Unknowns) != 0 || !trust && (len(file.Unknowns) != 1 || file.Unknowns[0].Code != deps.UnknownUnmeasuredReader) {
			t.Fatalf("pack unknowns: %+v", file.Unknowns)
		}
		rows, err := reader.ReadFiles(t.Context(), repo, []deps.Input{in})
		if err != nil || len(rows) != 1 || !reflect.DeepEqual(file, rows[0]) {
			t.Fatalf("batch disagrees: %+v, %v", rows, err)
		}
		want := []deps.Edge{
			{From: path, Kind: deps.KindImport, Raw: "module.txt", To: "app/module.txt", Resolution: deps.ResolvedFile, Line: 3, Provenance: deps.ProvenancePack},
			{From: path, Kind: deps.KindInclude, Raw: "local.txt", To: "app/local.txt", Resolution: deps.ResolvedFile, Line: 4, Provenance: deps.ProvenancePack},
			{From: path, Kind: deps.KindEmbed, Raw: "picture.txt", To: "app/picture.txt", Resolution: deps.ResolvedFile, Line: 5, Provenance: deps.ProvenancePack},
			{From: path, Kind: deps.KindBuildInput, Raw: "inputs", To: "app/inputs", Resolution: deps.ResolvedDirectory, Line: 6, Provenance: deps.ProvenancePack},
			{From: path, Kind: deps.KindExec, Raw: "run.sh", To: "app/run.sh", Resolution: deps.ResolvedFile, Line: 7, Provenance: deps.ProvenancePack},
			{From: path, Kind: deps.KindConfigRef, Raw: "settings.txt", To: "app/settings.txt", Resolution: deps.ResolvedFile, Line: 8, Provenance: deps.ProvenancePack},
		}
		if !slices.Equal(file.Edges, want) {
			t.Fatalf("pack kinds, locations or provenance: %+v", file.Edges)
		}
	}
	trusted, err := deps.New(opt, deps.TrustUnmeasured("task-spec"))
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name, src string
		code      deps.UnknownCode
	}{
		{"missing", `{"imports":["missing.txt"]}`, deps.UnknownUnresolvedAlias},
		{"symlink", `{"imports":["outside.txt"]}`, deps.UnknownUnresolvedAlias},
		{"pack unresolved", `{"imports":["../../outside.txt"]}`, deps.UnknownUnresolvedAlias},
		{"unsupported strategy", `{"service":"module.txt"}`, deps.UnknownUnsupportedConstruct},
		{"unclassified relation", `{"extra":"module.txt"}`, deps.UnknownUnsupportedConstruct},
		{"invalid syntax", `{"imports":`, deps.UnknownParseError},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := trusted.ReadFile(t.Context(), repo, in, []byte(tc.src))
			if err != nil || !got.Trusted || got.Measured || got.Complete || len(got.Edges) != 0 ||
				len(got.Unknowns) != 1 || got.Unknowns[0].Code != tc.code {
				t.Fatalf("trusted pack concealed evidence: %+v, %v", got, err)
			}
		})
	}
	rows := trusted.Dialects()
	if i := slices.IndexFunc(rows, func(row deps.DialectInfo) bool { return row.ID == "task-spec" }); i == -1 ||
		rows[i].Measured || !slices.Equal(rows[i].Kinds, []deps.Kind{
		deps.KindBuildInput, deps.KindConfigRef, deps.KindEmbed, deps.KindExec, deps.KindImport, deps.KindInclude,
	}) {
		t.Fatalf("pack metadata: %+v", rows)
	}
}

func TestDependencyPackRegistration(t *testing.T) {
	parser, err := lang.New(lang.WithDialects(packFS()))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = parser.Close() })
	kinds := map[string]deps.Kind{"includes": deps.KindInclude}
	pack := deps.WithPack(parser, "task-spec", kinds)
	for _, tc := range []struct {
		name string
		opts []deps.Option
	}{
		{"nil reader", []deps.Option{deps.WithPack(nil, "task-spec", kinds)}},
		{"unregistered dialect", []deps.Option{deps.WithPack(parser, "missing-pack", kinds)}},
		{"dependency dialect", []deps.Option{deps.WithPack(parser, "dockerfile", kinds)}},
		{"no classification", []deps.Option{deps.WithPack(parser, "task-spec", nil)}},
		{"unknown kind", []deps.Option{deps.WithPack(parser, "task-spec", map[string]deps.Kind{"includes": "other"})}},
		{"duplicate dialect", []deps.Option{pack, pack}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := deps.New(tc.opts...); !errors.Is(err, deps.ErrInput) {
				t.Fatalf("registration error: %v", err)
			}
		})
	}
	for _, info := range parser.Dialects() {
		if info.Engine != "doc" || info.Level != lang.LevelUnmeasured || info.ID == "task-spec" {
			continue
		}
		if _, err := deps.New(deps.WithPack(parser, info.ID, kinds)); !errors.Is(err, deps.ErrInput) {
			t.Errorf("built-in language dialect %q accepted as a pack: %v", info.ID, err)
		}
	}
}

type taskResolver func(context.Context, deps.ResolveRequest) deps.ResolveResult

func (r taskResolver) Resolve(ctx context.Context, req deps.ResolveRequest) deps.ResolveResult {
	return r(ctx, req)
}

func TestDependencyPackResolver(t *testing.T) {
	parser, err := lang.New(lang.WithDialects(packFS()))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = parser.Close() })
	in := deps.Input{Path: "app.task.json", Dialect: "task-spec"}
	src := []byte(`{"service":"local"}`)
	ctx := t.Context()
	resolver := taskResolver(func(gotCtx context.Context, req deps.ResolveRequest) deps.ResolveResult {
		if gotCtx != ctx || req.From != in.Path || req.Raw != "local" || req.Kind != deps.KindImport || req.Line != 1 {
			t.Errorf("pack resolver request: %+v", req)
		}
		return deps.ResolveResult{Edges: []deps.Edge{{Kind: req.Kind, To: "target.txt", Resolution: deps.ResolvedFile}}}
	})
	reader, err := deps.New(deps.WithResolver(in.Dialect, resolver),
		deps.WithPack(parser, in.Dialect, map[string]deps.Kind{"loads": deps.KindImport}),
		deps.TrustUnmeasured(in.Dialect))
	if err != nil {
		t.Fatal(err)
	}
	got, err := reader.ReadFile(ctx, fstest.MapFS{"target.txt": {}}, in, src)
	if err != nil || got.Measured || !got.Trusted || !got.Complete || len(got.Edges) != 1 {
		t.Fatalf("pack resolver result: %+v, %v", got, err)
	}
	if e := got.Edges[0]; e.To != "target.txt" || e.Provenance != deps.ProvenanceExtension {
		t.Errorf("pack resolver edge: %+v", e)
	}
}
