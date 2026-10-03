package fusion_test

// LB-F196 (O369): C and C++ references. The c dialect's references.scm names
// the types, typedef names and macros a file uses outside a call; a name binds
// the definition in a header the file reaches through its include graph, by
// the include's own resolution (never by basename), and never a name the file
// defines itself.

import (
	"context"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/fusion"
)

// refEdgeStrings renders edges as "file:caller -> file:callee" through symbols.
func refEdgeStrings(edges []core.Edge, symbols []core.SymbolNode) []string {
	label := map[string]string{}
	for _, s := range symbols {
		label[s.Key()] = s.FilePath + ":" + s.Name
	}
	var out []string
	for _, e := range edges {
		if e.Kind == core.EdgeFileResolved {
			out = append(out, label[e.FromSymbol]+" -> "+label[e.ToSymbol])
		}
	}
	sort.Strings(out)
	return out
}

func TestResolveFileEdges_CReferences(t *testing.T) {
	dir, err := filepath.Abs(filepath.Join("testdata", "crefsfixture"))
	if err != nil {
		t.Fatal(err)
	}
	paths := []string{"inc/types.h", "inc/other/types.h", "src/use_item.c", "src/use_other.c", "src/shadow.c", "src/mix.c"}
	files, symbols := loadFileResolveFixture(t, dir, "c", paths)
	edges, report := fusion.ResolveFileEdges(context.Background(), "c", dir, files, symbols)
	if report.Skipped {
		t.Fatalf("skipped: %s", report.SkipReason)
	}
	got := map[string]bool{}
	for _, s := range refEdgeStrings(edges, symbols) {
		got[s] = true
	}
	for _, w := range []string{
		"src/use_item.c:total -> inc/types.h:item_t",               // a typedef name in a parameter
		"src/use_item.c:total -> inc/types.h:ITEM_MAX",             // a macro read in an expression
		"src/use_other.c:other_total -> inc/other/types.h:other_t", // the include's own resolution, not the basename
		"src/mix.c:pick -> inc/types.h:shape_t",                    // a typedef name written as a return type and a cast
		"src/mix.c:pick -> inc/types.h:item_t",                     // a sizeof of a pointer type
		"src/mix.c:pick -> inc/other/types.h:other_t",              // a local variable's type
	} {
		if !got[w] {
			t.Errorf("missing reference edge %s\ngot %v", w, got)
		}
	}
	for k := range got {
		switch k {
		case "src/use_other.c:other_total -> inc/types.h:item_t", "src/use_item.c:total -> inc/other/types.h:other_t":
			t.Errorf("bound through the wrong same-named header: %s", k)
		}
		if strings.HasPrefix(k, "src/shadow.c:shadow_total -> inc/types.h:item_t") {
			t.Errorf("a name the file defines itself bound to a header: %s", k)
		}
	}
}
