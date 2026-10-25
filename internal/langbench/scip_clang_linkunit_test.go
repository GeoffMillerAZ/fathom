package langbench

import (
	"os"
	"path/filepath"
	"testing"

	"google.golang.org/protobuf/proto"

	scippb "github.com/geoffmilleraz/fathom/third_party/scip"
)

// TestScoreSCIPClangLinkUnits (round 3 B, C2): cuda-samples is many programs in one
// repository. Two samples each define an extern iDivUp and a kernel of one name, and
// scip-clang names a global by no file or translation unit, so K25 ruling 3 drew each
// sample's reference to every sample's copy. A program holds one definition of an
// extern identifier (C++ [basic.def.odr]), so the copy a reference means is one linked
// into the same executable: a document's link units are the CMake executables whose
// objects it is compiled into, and a header's are those of the sources whose includes
// reach it. A definer sharing no link unit with the referring document is left out
// (scip_file_edges_link_unit) and disclosed; a header both samples include
// (Common/helper.h) keeps its edge from each, and a reference whose link units hold no
// definer keeps ruling 3.
func TestScoreSCIPClangLinkUnits(t *testing.T) {
	const pre = "cxx . . $ "
	div, help, lone := pre+"iDivUp(1111111111111111).", pre+"helper(2222222222222222).", pre+"lone(3333333333333333)."
	idx := &scippb.Index{Metadata: &scippb.Metadata{ToolInfo: &scippb.ToolInfo{Name: "scip-clang", Version: "0.4.0"}},
		Documents: []*scippb.Document{
			{RelativePath: "Common/helper.h", Occurrences: []*scippb.Occurrence{scipCallsiteDef(help, nil, 0, 11, 17)}},
			{RelativePath: "Samples/a/a.cu", Occurrences: []*scippb.Occurrence{
				scipCallsiteDef(div, nil, 1, 4, 10), scipCallsiteRef(div, 5, 2, 8), scipCallsiteRef(help, 6, 2, 8),
			}},
			{RelativePath: "Samples/a/main.cpp", Occurrences: []*scippb.Occurrence{scipCallsiteRef(div, 3, 2, 8)}},
			{RelativePath: "Samples/b/b.cu", Occurrences: []*scippb.Occurrence{
				scipCallsiteDef(div, nil, 1, 4, 10), scipCallsiteRef(div, 5, 2, 8), scipCallsiteRef(help, 6, 2, 8), scipCallsiteRef(lone, 7, 2, 6),
			}},
			{RelativePath: "Samples/c/c.cu", Occurrences: []*scippb.Occurrence{scipCallsiteDef(lone, nil, 1, 4, 8)}},
			{RelativePath: "Samples/d/d.cu", Occurrences: []*scippb.Occurrence{scipCallsiteDef(lone, nil, 1, 4, 8)}},
		}}
	raw, err := proto.Marshal(idx)
	if err != nil {
		t.Fatal(err)
	}
	opts := SCIPOptionsFor("cuda", nil)
	if opts.ClangLinkage == nil {
		t.Fatal("CUDA's option set carries no linkage rule")
	}
	dir := t.TempDir()
	pre1 := filepath.Join(dir, "preproc.json")
	if err := os.WriteFile(pre1, []byte(`{"tool":"clang-preprocessor","tier":"A","imports":[
 {"path":"Samples/a/a.cu","line":1,"to_path":"Common/helper.h"},{"path":"Samples/b/b.cu","line":1,"to_path":"Common/helper.h"}]}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "preproc.units.json"), []byte(`{"tool":"cmake-link-lines","tier":"A","link_units":{
 "a":["Samples/a/a.cu","Samples/a/main.cpp"],"b":["Samples/b/b.cu"],"c":["Samples/c/c.cu"],"d":["Samples/d/d.cu"]}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := opts.ClangLinkage.Read([]string{pre1}, func(p string) string { return p + ".none" }); err != nil {
		t.Fatal(err)
	}
	res, err := ScoreSCIPWith(raw, "s", nil, opts)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range [][2]string{{"Samples/a/main.cpp", "Samples/a/a.cu"}, {"Samples/a/a.cu", "Common/helper.h"}, {"Samples/b/b.cu", "Common/helper.h"},
		{"Samples/b/b.cu", "Samples/c/c.cu"}, {"Samples/b/b.cu", "Samples/d/d.cu"}} {
		if !res.FileEdges[e] {
			t.Errorf("no truth edge %v: %v", e, res.FileEdges)
		}
	}
	for _, e := range [][2]string{{"Samples/a/a.cu", "Samples/b/b.cu"}, {"Samples/b/b.cu", "Samples/a/a.cu"}, {"Samples/a/main.cpp", "Samples/b/b.cu"}} {
		if res.FileEdges[e] {
			t.Errorf("truth edge %v crosses an executable boundary: %v", e, res.FileEdges)
		}
	}
	notes := opts.ClangLinkage.Finish("s", res.FileEdges, res.Counts)
	if res.Counts[CntScipFileEdgesLinkUnit] != 3 || len(notes) == 0 {
		t.Errorf("scip_file_edges_link_unit %d, notes %v; want 3 and a disclosure", res.Counts[CntScipFileEdgesLinkUnit], notes)
	}
}

// TestClangLinkageIncludeClosure (round 3 F5): the headers the measured files'
// includes reach, transitively, less the measured files themselves.
func TestClangLinkageIncludeClosure(t *testing.T) {
	l := &ClangLinkage{}
	l.Include("a.cu", "Common/helper.h")
	l.Include("Common/helper.h", "Common/deep.h")
	l.Include("b.cu", "b.cuh")
	l.Include("z.cpp", "z.h")
	got := l.IncludeClosure(map[string]bool{"a.cu": true, "b.cu": true, "b.cuh": true})
	if len(got) != 2 || !got["Common/helper.h"] || !got["Common/deep.h"] {
		t.Errorf("IncludeClosure = %v, want Common/helper.h and Common/deep.h", got)
	}
	var none *ClangLinkage
	if got := none.IncludeClosure(map[string]bool{"a.cu": true}); len(got) != 0 {
		t.Errorf("nil rule reached %v", got)
	}
}
