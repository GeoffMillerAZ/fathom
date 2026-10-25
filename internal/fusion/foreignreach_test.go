package fusion

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"sync"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/connector"
	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/extractor"
	"github.com/geoffmilleraz/fathom/internal/store"
)

// foreignReachCorpus is a C file whose include closure passes through two
// headers the router sends to C++ (a class forward declaration in code: since
// LB-F195 the sniff reads code only, so a keyword in a comment no longer
// routes a header), fa.h and fb.h, and a third, fc.h, that no file reaches.
// deep() is declared in fb.h and defined in deep.c: main.c binds it only when
// fa.h's and fb.h's includes are read.
var foreignReachCorpus = map[string]string{
	"c/main.c": "#include \"own.h\"\nint run(void) { return deep(); }\n",
	"c/own.h":  "#include \"fa.h\"\n",
	"c/fa.h":   "class Fa;\n#include \"fb.h\"\n",
	"c/fb.h":   "class Fb;\nint deep(void);\n",
	"c/fc.h":   "class Fc;\n#include \"fb.h\"\nint unused(void);\n",
	"c/deep.c": "int deep(void) { return 1; }\n",
}

// foreignIngest ingests the corpus and returns the sorted file-resolved edges
// and the paths the file-resolved pass parsed through extractImports.
func foreignIngest(t *testing.T, dir string) (edges, parsed string) {
	t.Helper()
	var mu sync.Mutex
	var paths []string
	orig := extractImports
	extractImports = func(ext extractor.FileSpecExtractor, path string, src []byte) (extractor.FileSpecs, error) {
		mu.Lock()
		paths = append(paths, path)
		mu.Unlock()
		return orig(ext, path, src)
	}
	t.Cleanup(func() { extractImports = orig })
	st := store.NewMemoryStore()
	p := New(connector.NewLocalConnector(), st, Config{})
	t.Cleanup(p.Close)
	ctx := context.Background()
	if _, err := p.Ingest(ctx, core.ServiceRef{ID: "fr", Path: dir}); err != nil {
		t.Fatal(err)
	}
	all, _ := st.TypedEdges(ctx, "fr")
	var es []string
	for _, e := range all {
		if e.Kind == core.EdgeFileResolved {
			es = append(es, e.FromFile+"|"+e.FromSymbol+"->"+e.ToSymbol)
		}
	}
	sort.Strings(es)
	sort.Strings(paths)
	return strings.Join(es, "\n"), strings.Join(paths, ",")
}

// TestForeignHeadersParsedOnlyWhenReachable (LB-F179, LB-M5): the file
// resolver reads a header the router sent to the other language only when an
// include in the language's own files, or in a header already reached, names
// it; an unreached one is neither parsed nor admitted, and the edges are
// those the eager read drew. fmt's C resolver read all 25 of its C++ headers
// with the C grammar for one C file that includes none of them.
func TestForeignHeadersParsedOnlyWhenReachable(t *testing.T) {
	dir := t.TempDir()
	writeCorpus(t, dir, foreignReachCorpus)

	foreignPruneOff = true
	eagerEdges, eagerParsed := foreignIngest(t, dir)
	foreignPruneOff = false
	t.Cleanup(func() { foreignPruneOff = false })
	edges, parsed := foreignIngest(t, dir)

	if !strings.Contains(eagerEdges, "c/main.c") || !strings.Contains(eagerEdges, "c/deep.deep") {
		t.Fatalf("the fixture draws no main.c -> deep.c edge through the foreign headers:\n%s", eagerEdges)
	}
	if edges != eagerEdges {
		t.Errorf("file-resolved edges differ\n pruned %s\n eager  %s", edges, eagerEdges)
	}
	if !strings.Contains(eagerParsed, "c/fc.h") {
		t.Fatalf("the eager read parsed %q, want it to include the unreached c/fc.h", eagerParsed)
	}
	if strings.Contains(parsed, "c/fc.h") {
		t.Errorf("an unreached foreign header was parsed: %q", parsed)
	}
	for _, h := range []string{"c/fa.h", "c/fb.h"} {
		if !strings.Contains(parsed, h) {
			t.Errorf("a reached foreign header %s was not parsed: %q", h, parsed)
		}
	}
}

// TestReachableForeignScansIncludesGenerously: the scan reads every spelling
// of an include a parse would (spaces after the #, a continuation, an angled
// or directory-qualified operand, a commented one counting as reached) and
// follows a chain of headers; a header no operand names by base is left out.
func TestReachableForeignScansIncludesGenerously(t *testing.T) {
	own := map[string]extractor.FileSpecs{
		"a.c": {Imports: []extractor.ImportSpec{{Module: `"sub/one.h"`}, {Module: `<two.h>`}}},
	}
	hs := []foreignHeader{
		{"x/one.h", []byte("#  include \\\n   \"three.h\"\n")},
		{"y/two.h", []byte("")},
		{"z/three.h", []byte("/* #include <four.h> */\n")},
		{"z/four.h", []byte("#include \"nowhere.h\"\n")},
		{"z/five.h", []byte("#include \"one.h\"\n")},
	}
	var got []string
	for _, h := range reachableForeign(own, hs) {
		got = append(got, h.path)
	}
	want := []string{"x/one.h", "y/two.h", "z/three.h", "z/four.h"}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Errorf("reachable = %v, want %v (five.h includes a reached header but nothing includes it)", got, want)
	}
}

// TestCUDABindsIncludedHeaders (LB-Q-cuda, ruling C-R4): a .h is C's or C++'s
// through the header router, never CUDA's, yet a .cu includes it and calls
// what it defines (cuda-samples' helper_cuda.h). The CUDA resolver reads the
// reached headers for their includes and binds their stored definitions, so
// k.cu draws an edge to helper.h's helper and, through it, to deep.h's deep;
// an unreached header binds nothing, and the edges stay the CUDA files'.
func TestCUDABindsIncludedHeaders(t *testing.T) {
	dir := t.TempDir()
	writeCorpus(t, dir, map[string]string{
		"k.cu":     "#include \"helper.h\"\n__global__ void k(int *p) { p[0] = deep(); }\nint main() { k<<<1, 1>>>(0); return helper(); }\n",
		"helper.h": "#include \"deep.h\"\ninline int helper(void) { return 1; }\n",
		"deep.h":   "class Deep;\ninline int deep(void) { return 2; }\n",
		"other.h":  "inline int other(void) { return 3; }\n",
		"use.c":    "int other(void);\nint use(void) { return other(); }\n",
	})
	edges, _ := foreignIngest(t, dir)
	for _, want := range []string{"k.cu|fr::k.main->fr::helper.helper", "k.cu|fr::k.k->fr::deep.deep"} {
		if !strings.Contains(edges, want) {
			t.Errorf("no %s edge:\n%s", want, edges)
		}
	}
	for _, line := range strings.Split(edges, "\n") {
		if strings.HasPrefix(line, "k.cu|") && strings.HasSuffix(line, "->fr::other.other") || strings.HasPrefix(line, "helper.h|") || strings.HasPrefix(line, "deep.h|") {
			t.Errorf("unexpected edge %s:\n%s", line, edges)
		}
	}
}

// TestReachableForeignReadsOnlyReachable (round 3 F6): the header pass prunes
// by base name before it reads: with one .cu and many unrelated headers, only
// the headers an include can reach are read at all, and what it returns is
// what reachableForeign returns from the eager read.
func TestReachableForeignReadsOnlyReachable(t *testing.T) {
	own := map[string]extractor.FileSpecs{"k.cu": {Imports: []extractor.ImportSpec{{Module: `"helper.h"`}}}}
	src := map[string][]byte{"Common/helper.h": []byte("#include \"deep.h\"\n"), "Common/deep.h": []byte("")}
	paths := []string{"Common/deep.h", "Common/helper.h"}
	for i := 0; i < 50; i++ {
		p := fmt.Sprintf("other/u%02d.h", i)
		src[p] = []byte("#include \"helper.h\"\n")
		paths = append(paths, p)
	}
	var read []string
	got := reachableForeignRead(own, paths, func(p string) ([]byte, bool) {
		read = append(read, p)
		b, ok := src[p]
		return b, ok
	})
	sort.Strings(read)
	if want := "Common/deep.h,Common/helper.h"; strings.Join(read, ",") != want {
		t.Errorf("read %v, want only %s", read, want)
	}
	var hs []foreignHeader
	for _, p := range paths {
		hs = append(hs, foreignHeader{p, src[p]})
	}
	var a, b []string
	for _, h := range got {
		a = append(a, h.path)
	}
	for _, h := range reachableForeign(own, hs) {
		b = append(b, h.path)
	}
	if strings.Join(a, ",") != strings.Join(b, ",") {
		t.Errorf("lazy %v, eager %v", a, b)
	}
}
