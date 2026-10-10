package langbench

import (
	"strings"
	"testing"

	"google.golang.org/protobuf/proto"

	scippb "github.com/geoffmilleraz/fathom/third_party/scip"
)

// TestScoreSCIPEveryDefiningDocument (K25 ruling 3, O539, LB-F387): a symbol the index
// defines in more than one document resolves to every one of them, never to the first
// alone, for every producer (rust-analyzer's per-target copies are narrowed further,
// TestScoreSCIPRustNamespaceDefiners). O539's case: two test targets' `utils` modules
// share one symbol, so `utils/FULL_TEMPLATE.` is defined in tests/builder/utils.rs and
// tests/derive/utils.rs; a reference in tests/builder/command.rs is an edge to both, and
// the one only the second definer draws is counted (scip_file_edges_every_definer), never
// silent. A symbol one document defines draws its one edge, and no edge is drawn back
// into the referring document.
func TestScoreSCIPEveryDefiningDocument(t *testing.T) {
	const pre = "scip-x cargo clap 4.6.1 "
	idx := &scippb.Index{Metadata: &scippb.Metadata{ToolInfo: &scippb.ToolInfo{Name: "scip-x"}},
		Documents: []*scippb.Document{
			{RelativePath: "tests/derive/utils.rs", Occurrences: []*scippb.Occurrence{
				scipCallsiteDef(pre+"utils/FULL_TEMPLATE.", nil, 0, 10, 23),
			}},
			{RelativePath: "tests/builder/utils.rs", Occurrences: []*scippb.Occurrence{
				scipCallsiteDef(pre+"utils/FULL_TEMPLATE.", nil, 0, 10, 23),
				scipCallsiteDef(pre+"utils/assert_output().", nil, 1, 7, 20),
			}},
			{RelativePath: "tests/builder/command.rs", Occurrences: []*scippb.Occurrence{
				scipCallsiteDef(pre+"command/f().", []int32{0, 0, 3, 1}, 0, 3, 4),
				scipCallsiteRef(pre+"utils/FULL_TEMPLATE.", 1, 11, 24),
				scipCallsiteRef(pre+"utils/assert_output().", 2, 11, 24),
			}},
			{RelativePath: "tests/derive/help.rs", Occurrences: []*scippb.Occurrence{
				// A definer referring to its own symbol draws no edge back into itself,
				// and one to the other definer.
				scipCallsiteDef(pre+"utils/FULL_TEMPLATE.", nil, 0, 10, 23),
				scipCallsiteRef(pre+"utils/FULL_TEMPLATE.", 1, 11, 24),
			}},
		}}
	raw, err := proto.Marshal(idx)
	if err != nil {
		t.Fatal(err)
	}
	res, err := ScoreSCIPWith(raw, "s", nil, SCIPOptions{})
	if err != nil {
		t.Fatal(err)
	}
	want := map[[2]string]bool{
		{"tests/builder/command.rs", "tests/derive/utils.rs"}:  true,
		{"tests/builder/command.rs", "tests/builder/utils.rs"}: true,
		{"tests/builder/command.rs", "tests/derive/help.rs"}:   true,
		{"tests/derive/help.rs", "tests/derive/utils.rs"}:      true,
		{"tests/derive/help.rs", "tests/builder/utils.rs"}:     true,
	}
	if len(res.FileEdges) != len(want) {
		t.Errorf("file edges %v, want %v", res.FileEdges, want)
	}
	for e := range want {
		if !res.FileEdges[e] {
			t.Errorf("missing file edge %v (have %v)", e, res.FileEdges)
		}
	}
	// The first definer (tests/derive/utils.rs) drew command.rs's and help.rs's edges
	// before the rule, and assert_output (one definer) already drew command.rs ->
	// tests/builder/utils.rs: the rule adds command.rs -> tests/derive/help.rs (the third
	// definer) and help.rs -> tests/builder/utils.rs, and counts both.
	if got := res.Counts[CntScipFileEdgesEveryDefiner]; got != 2 {
		t.Errorf("%s = %d, want 2", CntScipFileEdgesEveryDefiner, got)
	}
	if note := EveryDefinerNote("clap-rs/clap", res.Counts); !strings.Contains(note, "clap-rs/clap: 2 file-edge truth pairs") || !strings.Contains(note, "K25 ruling 3") {
		t.Errorf("note %q", note)
	}
	if EveryDefinerNote("x", map[string]int{}) != "" {
		t.Error("a subject the rule drew nothing for writes no note")
	}
	// A producer-specific multiply-defined rule still wins: scip-go's package, defined at
	// every file's package clause, draws no file edge at all (LB-F298).
	goIdx := proto.Clone(idx).(*scippb.Index)
	goIdx.Metadata.ToolInfo.Name = "scip-go"
	goRaw, err := proto.Marshal(goIdx)
	if err != nil {
		t.Fatal(err)
	}
	goRes, err := ScoreSCIPWith(goRaw, "s", nil, SCIPOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if goRes.FileEdges[[2]string{"tests/builder/command.rs", "tests/derive/utils.rs"}] || goRes.Counts[CntScipFileEdgesEveryDefiner] != 0 {
		t.Errorf("scip-go: a multiply-defined symbol draws no file edge and the every-definer rule counts nothing: %v %v", goRes.FileEdges, goRes.Counts)
	}
}

// TestScoreSCIPRustNamespaceDefiners (K18, K25 ruling 3, LB-F387; reverted in part by
// wave lf4's holdout, O795): rust-analyzer names a module by package and in-crate path,
// never by crate target, so `crate/` is one symbol for every root of a package and two
// test targets' `utils/` one symbol. A crate named by its package's name (`use
// clap::Parser;`) is the library root, and the pairs every definer would add there are
// counted (scip_file_edges_duplicate_narrowed); every other reference, `crate::` written
// as `crate`, a module, an item and a reference inside a defining document, resolves to
// every defining document (ruling 3 as written). Any other producer keeps ruling 3.
func TestScoreSCIPRustNamespaceDefiners(t *testing.T) {
	const pre = "rust-analyzer cargo clap 4.6.1 "
	file := func(sym string) *scippb.Occurrence { return scipCallsiteDef(sym, nil, 0, 0, 9, 0) } // a whole-file module
	idx := &scippb.Index{Metadata: &scippb.Metadata{ToolInfo: &scippb.ToolInfo{Name: "rust-analyzer"}},
		Documents: []*scippb.Document{
			{RelativePath: "src/lib.rs", Occurrences: []*scippb.Occurrence{file(pre + "crate/")}},
			{RelativePath: "tests/builder/main.rs", Occurrences: []*scippb.Occurrence{file(pre + "crate/"), scipCallsiteRef(pre+"utils/", 0, 4, 9)}},
			{RelativePath: "tests/derive/main.rs", Occurrences: []*scippb.Occurrence{file(pre + "crate/")}},
			{RelativePath: "examples/git.rs", Occurrences: []*scippb.Occurrence{
				file(pre + "crate/"), scipCallsiteRef(pre+"crate/", 3, 4, 9),
				// two examples each declare `struct Cli`: one symbol, a copy per target
				scipCallsiteDef(pre+"Cli#", nil, 5, 7, 10), scipCallsiteRef(pre+"Cli#", 6, 4, 7),
			}},
			{RelativePath: "examples/other.rs", Occurrences: []*scippb.Occurrence{scipCallsiteDef(pre+"Cli#", nil, 0, 7, 10)}},
			{RelativePath: "tests/builder/utils.rs", Occurrences: []*scippb.Occurrence{file(pre + "utils/")}},
			{RelativePath: "tests/derive/utils.rs", Occurrences: []*scippb.Occurrence{file(pre + "utils/")}},
			{RelativePath: "tests/builder/action.rs", Occurrences: []*scippb.Occurrence{
				scipCallsiteRef(pre+"crate/", 1, 4, 9),
				scipCallsiteRef(pre+"utils/", 1, 11, 16),
				scipCallsiteRef(pre+"Cli#", 2, 4, 7), // an item from no definer: every definer (ruling 3)
			}},
			{RelativePath: "clap_complete/src/lib.rs", Occurrences: []*scippb.Occurrence{scipCallsiteRef(pre+"crate/", 1, 4, 8)}},
			// `use clap::Parser;` in an example of clap itself: the same `crate/` symbol as
			// the example's own root, written as the package's name.
			{RelativePath: "examples/derive.rs", Occurrences: []*scippb.Occurrence{file(pre + "crate/"), scipCallsiteRef(pre+"crate/", 1, 4, 8)}},
		}}
	raw, err := proto.Marshal(idx)
	if err != nil {
		t.Fatal(err)
	}
	src := map[string]string{
		"examples/git.rs":          "\n\n\n    crate::x();\n",
		"tests/builder/action.rs":  "\n    crate::utils::f();\n",
		"clap_complete/src/lib.rs": "\nuse clap::Parser;\n",
		"examples/derive.rs":       "\nuse clap::Parser;\n",
	}
	opts := SCIPOptionsFor("rust", func(p string) []byte { return []byte(src[p]) })
	res, err := ScoreSCIPWith(raw, "s", nil, opts)
	if err != nil {
		t.Fatal(err)
	}
	want := map[[2]string]bool{
		{"examples/derive.rs", "src/lib.rs"}:                true, // the package's name is its library
		{"clap_complete/src/lib.rs", "src/lib.rs"}:          true, // another package names the library
		{"tests/builder/main.rs", "tests/builder/utils.rs"}: true, // a module: every definer
		{"tests/builder/main.rs", "tests/derive/utils.rs"}:  true,
		// crate:: written as `crate` in an example: every other root of the package
		{"examples/git.rs", "src/lib.rs"}:            true,
		{"examples/git.rs", "tests/builder/main.rs"}: true,
		{"examples/git.rs", "tests/derive/main.rs"}:  true,
		{"examples/git.rs", "examples/derive.rs"}:    true,
		{"examples/git.rs", "examples/other.rs"}:     true, // its own Cli#: every definer, its copy included
		// a test file: crate::, utils:: and Cli# each to every definer
		{"tests/builder/action.rs", "src/lib.rs"}:             true,
		{"tests/builder/action.rs", "tests/builder/main.rs"}:  true,
		{"tests/builder/action.rs", "tests/derive/main.rs"}:   true,
		{"tests/builder/action.rs", "examples/git.rs"}:        true,
		{"tests/builder/action.rs", "examples/derive.rs"}:     true,
		{"tests/builder/action.rs", "tests/builder/utils.rs"}: true,
		{"tests/builder/action.rs", "tests/derive/utils.rs"}:  true,
		{"tests/builder/action.rs", "examples/other.rs"}:      true,
	}
	if len(res.FileEdges) != len(want) {
		t.Errorf("file edges %v, want %v", res.FileEdges, want)
	}
	for e := range want {
		if !res.FileEdges[e] {
			t.Errorf("missing file edge %v (have %v)", e, res.FileEdges)
		}
	}
	// Only the package-name rule narrows: every definer (5 roots) would add
	// clap_complete -> the 4 test and example roots and derive.rs -> the 3 roots beside
	// src/lib.rs and itself: 4+3, each pair counted once and only when nothing else draws it.
	if got := res.Counts[CntScipFileEdgesDuplicateNarrowed]; got != 7 {
		t.Errorf("%s = %d, want 7", CntScipFileEdgesDuplicateNarrowed, got)
	}
	// Another producer's namespace keeps ruling 3: every definer.
	other := proto.Clone(idx).(*scippb.Index)
	for _, d := range other.Documents {
		for _, o := range d.Occurrences {
			o.Symbol = strings.Replace(o.Symbol, "rust-analyzer", "scip-x", 1)
		}
	}
	otherRaw, err := proto.Marshal(other)
	if err != nil {
		t.Fatal(err)
	}
	r2, err := ScoreSCIPWith(otherRaw, "s", nil, SCIPOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if !r2.FileEdges[[2]string{"tests/builder/action.rs", "examples/git.rs"}] || r2.Counts[CntScipFileEdgesDuplicateNarrowed] != 0 {
		t.Errorf("another producer: every definer, nothing narrowed: %v %v", r2.FileEdges, r2.Counts)
	}
}
