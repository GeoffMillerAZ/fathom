package langbench

import (
	"reflect"
	"strings"
	"testing"

	"google.golang.org/protobuf/proto"

	scippb "github.com/geoffmilleraz/fathom/third_party/scip"
)

// TestScoreImports (LB-F40, design §1.2 import_resolution): an oracle's
// import references are paired with fathom's import statements by line; an
// import (the name one statement binds, its alternative readings merged) is
// in-repo when the oracle resolves its statement to a file of the subject,
// and resolved when fathom's strategy reaches one of those files.
func TestScoreImports(t *testing.T) {
	sites := []ImportSite{
		{Path: "a.ts", StartLine: 1, EndLine: 4, Local: "a", Targets: []string{"x.ts"}}, // multi-line statement
		{Path: "a.ts", StartLine: 1, EndLine: 4, Local: "c", Targets: []string{"x.ts"}},
		{Path: "a.ts", StartLine: 5, EndLine: 5, Local: "d"},                            // unresolved
		{Path: "a.ts", StartLine: 6, EndLine: 6, Local: "e", Targets: []string{"w.ts"}}, // another file
		{Path: "a.ts", StartLine: 7, EndLine: 7, Local: "f"},                            // external, unresolved
		{Path: "a.ts", StartLine: 8, EndLine: 8, Local: "g", Targets: []string{"v.ts"}}, // unattested
		// A target that is the importer itself or a file not measured is
		// no resolution: the import counts, as neither resolved nor unattested.
		{Path: "a.ts", StartLine: 9, EndLine: 9, Local: "h", Targets: []string{"a.ts", "out.ts"}},
		// Two readings of one `use net::connect;` binding: one resolves.
		{Path: "b.rs", StartLine: 1, EndLine: 1, Local: "connect"},
		{Path: "b.rs", StartLine: 1, EndLine: 1, Local: "connect", Targets: []string{"net.rs"}},
	}
	truth := []OracleImport{
		{Path: "a.ts", Line: 2, ToPath: "x.ts"},
		{Path: "a.ts", Line: 4, ToPath: "x.ts"},
		{Path: "a.ts", Line: 3, ToPath: "q.ts"},
		{Path: "a.ts", Line: 5, ToPath: "y.ts"},
		{Path: "a.ts", Line: 5, ToPath: "out.ts"}, // outside the files measured
		{Path: "a.ts", Line: 6, ToPath: "z.ts"},
		{Path: "a.ts", Line: 2, ToPath: "a.ts"},                  // the importer itself
		{Path: "a.ts", Line: 12, ToPath: "x.ts"},                 // a later reference, not an import
		{Path: "a.ts", Line: 13, ToPath: "k.ts", Explicit: true}, // an import fathom did not extract
		{Path: "b.rs", Line: 1, ToPath: "net.rs"},
		{Path: "c.ts", Line: 1, ToPath: "x.ts", Explicit: true}, // a file fathom did not read
	}
	inScope := map[string]bool{}
	for _, p := range []string{"a.ts", "b.rs", "x.ts", "q.ts", "y.ts", "z.ts", "w.ts", "v.ts", "k.ts", "net.rs"} {
		inScope[p] = true
	}
	counts := map[string]int{}
	ScoreImports(truth, sites, inScope, counts)
	want := map[string]int{
		CntImports: 8, CntImportsInRepo: 5, CntImportsAgree: 3, CntImportsUnresolved: 1,
		CntImportsElsewhere: 1, CntImportsUnattested: 1,
		CntImportEdgesTruth: 5, CntImportEdgesFathom: 4, CntImportEdgesAgree: 2,
		CntImportOracleUnpaired: 1, CntImportOracleOutOfScope: 1,
	}
	if !reflect.DeepEqual(counts, want) {
		t.Errorf("counts\n got %v\nwant %v", counts, want)
	}
}

// ruFixtureIndex is a rust-analyzer-shaped index of the file resolver's rust
// fixture (internal/fusion/testdata/rustfixture): a file module is defined by
// its whole file and referenced at `mod x;` and in each `use` path.
func ruFixtureIndex() *scippb.Index {
	const pre = "rust-analyzer cargo app 0.1.0 "
	def := func(sym string, rng ...int32) *scippb.Occurrence {
		return &scippb.Occurrence{Symbol: sym, Range: rng, SymbolRoles: int32(scippb.SymbolRole_Definition)}
	}
	ref := func(sym string, roles scippb.SymbolRole, rng ...int32) *scippb.Occurrence {
		return &scippb.Occurrence{Symbol: sym, Range: rng, SymbolRoles: int32(roles)}
	}
	return &scippb.Index{Metadata: &scippb.Metadata{ToolInfo: &scippb.ToolInfo{Name: "rust-analyzer", Version: "1.98.1"}},
		Documents: []*scippb.Document{
			{RelativePath: "src/main.rs", Occurrences: []*scippb.Occurrence{
				ref(pre+"net/", 0, 0, 4, 7),    // mod net;
				ref(pre+"util/", 0, 1, 4, 8),   // mod util;
				ref(pre+"util/", 0, 3, 11, 15), // use crate::util::helper;
				ref(pre+"util/helper().", scippb.SymbolRole_Import, 3, 17, 23),
				ref(pre+"net/", 0, 4, 4, 7), // use net::connect;
				ref(pre+"net/connect().", 0, 4, 9, 16),
				def(pre+"main().", 6, 3, 7),
				ref(pre+"util/helper().", 0, 7, 4, 10), // helper(); a call, not an import
				def("local 0", 8, 8, 9),
			}},
			{RelativePath: "src/util.rs", Occurrences: []*scippb.Occurrence{
				def(pre+"util/", 0, 0, 2, 17),
				def(pre+"util/helper().", 0, 7, 13),
				def(pre+"util/other().", 2, 7, 12),
				ref("local 0", 0, 1, 0, 1), // a local is its own document's: never a cross-file edge
			}},
			{RelativePath: "src/net/mod.rs", Occurrences: []*scippb.Occurrence{
				def(pre+"net/", 0, 0, 0, 19),
				def(pre+"net/connect().", 0, 7, 14),
			}},
		}}
}

// TestSCIPImportRefs: the index's cross-file references, one per (document,
// line, defining document), carry the oracle's own Import role where the
// producer writes it; a module a file defines by its whole range is a
// target like any other definition, and a local symbol never crosses files.
func TestSCIPImportRefs(t *testing.T) {
	raw, err := proto.Marshal(ruFixtureIndex())
	if err != nil {
		t.Fatal(err)
	}
	res, err := ScoreSCIPWith(raw, "s", nil, SCIPOptions{})
	if err != nil {
		t.Fatal(err)
	}
	want := []OracleImport{
		{Path: "src/main.rs", Line: 1, ToPath: "src/net/mod.rs"},
		{Path: "src/main.rs", Line: 2, ToPath: "src/util.rs"},
		{Path: "src/main.rs", Line: 4, ToPath: "src/util.rs", Explicit: true},
		{Path: "src/main.rs", Line: 5, ToPath: "src/net/mod.rs"},
		{Path: "src/main.rs", Line: 8, ToPath: "src/util.rs"},
	}
	if !reflect.DeepEqual(res.ImportRefs, want) {
		t.Errorf("import refs\n got %v\nwant %v", res.ImportRefs, want)
	}

	// Scored against the rust fixture's resolution (the fusion test's
	// output): the whole-module reading of `use net::connect;` resolves
	// nothing, its named reading does, and the binding counts once.
	sites := []ImportSite{
		{Path: "src/main.rs", StartLine: 1, EndLine: 1, Local: "net", Targets: []string{"src/net/mod.rs"}},
		{Path: "src/main.rs", StartLine: 2, EndLine: 2, Local: "util", Targets: []string{"src/util.rs"}},
		{Path: "src/main.rs", StartLine: 4, EndLine: 4, Local: "helper", Targets: []string{"src/util.rs"}},
		{Path: "src/main.rs", StartLine: 4, EndLine: 4, Local: "helper", Targets: []string{"src/util.rs"}},
		{Path: "src/main.rs", StartLine: 5, EndLine: 5, Local: "connect", Targets: []string{"src/net/mod.rs"}},
		{Path: "src/main.rs", StartLine: 5, EndLine: 5, Local: "connect"},
	}
	in := map[string]bool{"src/main.rs": true, "src/util.rs": true, "src/net/mod.rs": true}
	counts := map[string]int{}
	ScoreImports(res.ImportRefs, sites, in, counts)
	if counts[CntImports] != 4 || counts[CntImportsInRepo] != 4 || counts[CntImportsAgree] != 4 || counts[CntImportOracleUnpaired] != 0 {
		t.Errorf("rust fixture counts = %v", counts)
	}
}

// TestSCIPImportRefsAmbiguous: a symbol the index defines in two documents
// (sample apps sharing one package name) names no file: a reference to it on
// an import statement is counted, never scored as truth.
func TestSCIPImportRefsAmbiguous(t *testing.T) {
	const sym = "scip-typescript npm sample 1.0 src/`app.module.ts`/AppModule#"
	def := func(rng ...int32) *scippb.Occurrence {
		return &scippb.Occurrence{Symbol: sym, Range: rng, SymbolRoles: int32(scippb.SymbolRole_Definition)}
	}
	idx := &scippb.Index{Documents: []*scippb.Document{
		{RelativePath: "a/src/app.module.ts", Occurrences: []*scippb.Occurrence{def(0, 13, 22)}},
		{RelativePath: "b/src/app.module.ts", Occurrences: []*scippb.Occurrence{def(0, 13, 22)}},
		{RelativePath: "b/src/main.ts", Occurrences: []*scippb.Occurrence{{Symbol: sym, Range: []int32{0, 9, 18}}}},
	}}
	raw, err := proto.Marshal(idx)
	if err != nil {
		t.Fatal(err)
	}
	res, err := ScoreSCIPWith(raw, "s", nil, SCIPOptions{})
	if err != nil {
		t.Fatal(err)
	}
	want := []OracleImport{{Path: "b/src/main.ts", Line: 1, Ambiguous: true}}
	if !reflect.DeepEqual(res.ImportRefs, want) {
		t.Fatalf("import refs %v, want %v", res.ImportRefs, want)
	}
	counts := map[string]int{}
	in := map[string]bool{"a/src/app.module.ts": true, "b/src/app.module.ts": true, "b/src/main.ts": true}
	ScoreImports(res.ImportRefs, []ImportSite{{Path: "b/src/main.ts", StartLine: 1, EndLine: 1, Local: "AppModule", Targets: []string{"b/src/app.module.ts"}}}, in, counts)
	if counts[CntImportsInRepo] != 0 || counts[CntImportsUnattested] != 1 || counts[CntImportOracleAmbiguous] != 1 {
		t.Errorf("counts = %v", counts)
	}
}

// TestRecomputeImportResolution: a subject's import counts become the L3
// import_resolution metric at the oracle's tier (the §1.2 bar: .95 excellent,
// .85 good), with one recomputed disclosure note, and the pre-LB-F40 "not
// measured" note leaves a reading once the family is measured.
func TestRecomputeImportResolution(t *testing.T) {
	base := func(tier string, agree int) Subject {
		return Subject{Name: "s", Files: 10, Bytes: 5e6,
			Counts: map[string]int{
				CntOracleDefs: 100, CntFathomDefs: 100, CntMatched: 100, CntKindEqual: 100,
				CntRangeEligible: 100, CntRangeOK: 100, CntCallables: 10, CntCallablesSigned: 10,
				CntCallTruth: 10, CntCallTruthFound: 10, CntCallFathom: 10, CntCallFathomInTrue: 10,
				CntFileEdgesTruth: 10, CntFileEdgesFathom: 10, CntFileEdgesAgree: 10,
				CntImports: 25, CntImportsInRepo: 20, CntImportsAgree: agree, CntImportsUnresolved: 20 - agree,
				CntImportEdgesTruth: 22, CntImportEdgesFathom: 21, CntImportEdgesAgree: agree,
			},
			Families: map[string]string{FamilyL1: "A", FamilyCalls: "A", FamilyFileEdges: "A", FamilyImports: tier},
			Tenancy:  &SubjectTenancy{FaultingFiles: []string{}, ExtractMS: 100, ExtractCPUMS: 100, MaxFileMS: 5, PeakModulePages: 10}}
	}
	for _, tc := range []struct {
		tier     string
		agree    int
		level    string
		gradeL3  string
		resolved float64
	}{
		{"A", 20, "L3", GradeExcellent, 1},
		{"A", 18, "L3", GradeGood, .9},
		{"B", 20, "L3", GradeGood, 1},
		{"A", 16, "L2", GradeUnmet, .8},
	} {
		r := Reading{Dialect: "rust", Kind: "language", TargetLevel: "L4",
			Notes: []string{"import_resolution not measured: in-repo spec counts arrive with the L3 strategies (LB-F11)"}}
		MergeSubject(&r, base(tc.tier, tc.agree))
		if err := r.Validate(); err != nil {
			t.Fatal(err)
		}
		v := r.Metrics[MImportResolution]
		if v == nil || *v != tc.resolved || r.MetricTiers[MImportResolution] != tc.tier {
			t.Errorf("tier %s agree %d: import_resolution %v tier %q", tc.tier, tc.agree, v, r.MetricTiers[MImportResolution])
		}
		if r.LevelMet != tc.level || r.Grade.L3 != tc.gradeL3 {
			t.Errorf("tier %s agree %d: level %s L3 %s, want %s %s", tc.tier, tc.agree, r.LevelMet, r.Grade.L3, tc.level, tc.gradeL3)
		}
		var notes []string
		for _, n := range r.Notes {
			if strings.HasPrefix(n, "import_resolution") {
				notes = append(notes, n)
			}
		}
		if len(notes) != 1 || !strings.HasPrefix(notes[0], noteImportsPrefix) || !strings.Contains(notes[0], "of 20 in-repo imports") {
			t.Errorf("import notes = %q", notes)
		}
	}
}

// TestScoreImportsEmpty: a subject with no import statement and no oracle
// import records no import count at all.
func TestScoreImportsEmpty(t *testing.T) {
	counts := map[string]int{}
	ScoreImports(nil, nil, map[string]bool{}, counts)
	if len(counts) != 0 {
		t.Errorf("counts = %v, want none", counts)
	}
}
