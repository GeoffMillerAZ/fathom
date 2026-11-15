package callgraph

import (
	"fmt"
	"io/fs"
	"math/rand"
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

func TestCtorScopeIndexMatchesScan(t *testing.T) {
	rng := rand.New(rand.NewSource(195))
	node := func(dir, name string, kind core.SourceKind) core.SymbolNode {
		return core.SymbolNode{Service: "svc", Package: "p", Name: name, Kind: kind, FilePath: dir}
	}
	explicit := []core.SymbolNode{
		node("a/pkg/x.go", "One.Run", core.KindGoMethod),
		node("pkg/x.go", "Two.Run", core.KindGoMethod),
		node("a/pkg/x.go", "One.Unique", core.KindGoMethod),
		node("other/pkg/x.go", "Three.Run", core.KindGoMethod),
		node("a/pkg/x.go", "Field.Unique", core.KindGoField),
		node("", "Empty.Run", core.KindGoMethod),
		node("x.go", "Root.Run", core.KindGoMethod),
		node("/x.go", "Absolute.Run", core.KindGoMethod),
	}
	cases := map[string][]core.SymbolNode{"explicit": explicit}
	for seed := 0; seed < 30; seed++ {
		nodes := append([]core.SymbolNode{}, explicit...)
		for i := 0; i < 80; i++ {
			dirs := []string{"a/pkg", "pkg", "other/pkg", "nested/a/pkg", "single", ""}
			n := node(path.Join(dirs[rng.Intn(len(dirs))], fmt.Sprintf("f%d.go", i)), fmt.Sprintf("R%d.M%d", i, rng.Intn(8)), core.KindGoMethod)
			if i%7 == 0 {
				n.Kind = core.KindGoField
			}
			nodes = append(nodes, n)
		}
		cases[fmt.Sprintf("generated-%d", seed)] = nodes
	}
	var fixture []core.SymbolNode
	root := "../../connector/testdata/fauxtown"
	err := filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || !strings.HasSuffix(p, ".go") {
			return nil
		}
		b, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(root, p)
		if err != nil {
			return err
		}
		ns, err := extractor.NewGoExtractor().Extract("fixture", filepath.ToSlash(rel), b)
		fixture = append(fixture, ns...)
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(fixture) == 0 {
		t.Fatal("empty fauxtown fixture")
	}
	cases["fauxtown"] = fixture
	for name, nodes := range cases {
		t.Run(name, func(t *testing.T) {
			var calls []string
			for _, n := range nodes {
				for i := 0; i < len(n.Name); i++ {
					if n.Name[i] != '.' {
						continue
					}
					for _, prefix := range []string{"", "module/", "nested/module/"} {
						calls = append(calls, prefix+path.Dir(n.FilePath)+"#"+n.Name[i:])
					}
				}
				calls = append(calls, n.Calls...)
			}
			calls = append(calls, "a/pkg#.Unique", "a/pkg#.Run", "absent#.Run", "a/pkg#.Missing", "a/pkg#.")
			caller := node("caller/main.go", "Caller", core.KindGoFunc)
			caller.Calls = calls
			nodes = append(append([]core.SymbolNode{}, nodes...), caller)
			oracle := buildIndexes(nodes)
			oracle.buildSuffixIndex()
			idx := buildCtorScopeIndex(oracle.nodes)
			for _, raw := range calls {
				if i := strings.Index(raw, "#."); i > 0 {
					got := idx.resolve(raw[:i], raw[i+1:])
					want := oracle.resolveCtorScoped(raw[:i], raw[i+1:])
					if got != want {
						t.Fatalf("%q: index %q, scan %q", raw, got, want)
					}
				}
			}
			if name == "explicit" {
				if got := idx.resolve("a/pkg", ".Unique"); got != explicit[2].Key() {
					t.Fatalf("unique = %q", got)
				}
				if got := idx.resolve("a/pkg", ".Run"); got != "" {
					t.Fatalf("ambiguous = %q", got)
				}
			}
			var want []string
			for key, n := range oracle.nodes {
				seen := map[string]bool{}
				for _, raw := range n.Calls {
					k := oracle.resolve(n, raw)
					if k != "" && oracle.nodes[k].Kind != core.KindGoField && !seen[k] {
						want = append(want, key+" -> "+k)
						seen[k] = true
					}
				}
			}
			sort.Strings(want)
			for perm := 0; perm < 4; perm++ {
				rng.Shuffle(len(nodes), func(i, j int) { nodes[i], nodes[j] = nodes[j], nodes[i] })
				got := ctorScopeEdges(Build(nodes))
				if !reflect.DeepEqual(got, want) {
					t.Fatalf("permutation %d: edges differ\ngot %v\nwant %v", perm, got, want)
				}
			}
		})
	}
}

func ctorScopeEdges(g *Graph) []string {
	var edges []string
	for k, cs := range g.callees {
		for _, c := range cs {
			edges = append(edges, k+" -> "+c)
		}
	}
	sort.Strings(edges)
	return edges
}
