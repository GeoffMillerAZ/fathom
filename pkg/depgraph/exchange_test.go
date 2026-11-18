package depgraph_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/pkg/depgraph"
)

// TestDeclaredEdgeWireExchange consumes the same bytes checked by
// TestDepsJSONGolden against the language CLI's -export output.
func TestDeclaredEdgeWireExchange(t *testing.T) {
	wire, err := os.ReadFile("../../cmd/fathom-lang/testdata/golden/deps-declared.json")
	if err != nil {
		t.Fatal(err)
	}
	var declarations []depgraph.EdgeDecl
	if err := json.Unmarshal(wire, &declarations); err != nil {
		t.Fatal(err)
	}
	source := depgraph.Unit{Ecosystem: "repo", Name: "main.go"}
	manifest := depgraph.Unit{Ecosystem: "repo", Name: "go.mod"}
	library := depgraph.Unit{Ecosystem: "repo", Name: "lib"}
	want := []depgraph.Edge{
		{From: depgraph.UnitRef{Ecosystem: "repo", Name: "main.go"}, To: depgraph.UnitRef{Ecosystem: "repo", Name: "go.mod"}},
		{From: depgraph.UnitRef{Ecosystem: "repo", Name: "main.go"}, To: depgraph.UnitRef{Ecosystem: "repo", Name: "lib"}},
	}

	for _, units := range [][]depgraph.Unit{{source, manifest, library}, {library, manifest, source}} {
		got, err := depgraph.ResolveDeclaredEdges(declarations, units)
		if err != nil || !reflect.DeepEqual(got, want) {
			t.Fatalf("resolve exported declarations = %v, %v, want %v", got, err, want)
		}
	}
	for _, tc := range []struct {
		name  string
		units []depgraph.Unit
		want  string
	}{
		{"missing source", []depgraph.Unit{manifest, library}, "from unknown unit repo/main.go"},
		{"missing target", []depgraph.Unit{source, manifest}, "to unknown unit repo/lib"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := depgraph.ResolveDeclaredEdges(declarations, tc.units)
			if got != nil || err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("resolve with %s = %v, %v, want nil and %q", tc.name, got, err, tc.want)
			}
		})
	}
}

func TestDepgraphKeyAcrossRoots(t *testing.T) {
	var wantSources []depgraph.SourceRef
	var wantKey depgraph.Key
	for i := range 2 {
		root := t.TempDir()
		if err := os.WriteFile(filepath.Join(root, "source"), []byte("fixture\n"), 0o600); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink("source", filepath.Join(root, "link")); err != nil {
			t.Fatal(err)
		}
		for range 2 {
			sources, err := depgraph.Materialize(os.DirFS(root), []string{"."})
			if err != nil {
				t.Fatal(err)
			}
			if len(sources) != 2 || sources[0].Path != "link" || sources[0].Mode != depgraph.ModeSymlink || sources[1].Path != "source" {
				t.Fatalf("materialized tree %d = %+v", i, sources)
			}
			key := depgraph.InputKey(depgraph.KeyInputs{Domain: "example.com/check/v1", Sources: sources})
			if wantKey == "" {
				wantSources, wantKey = sources, key
			}
			if key != wantKey || !reflect.DeepEqual(sources, wantSources) {
				t.Fatalf("tree %d diverged: sources = %+v, key = %s", i, sources, key)
			}
		}
	}
}
