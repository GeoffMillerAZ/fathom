package langbench

import (
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/core"
)

func pySpec(t *testing.T) BenchSpec {
	t.Helper()
	s, ok := IncumbentSpec("python")
	if !ok {
		t.Fatal("no python incumbent spec")
	}
	return s
}

func TestParseCtagsJSON(t *testing.T) {
	in := `{"_type": "ptag", "name": "JSON_OUTPUT_VERSION", "path": "0.0"}
{"_type": "tag", "name": "Foo", "path": "a.py", "line": 1, "kind": "class", "end": 9}
{"_type": "tag", "name": "bar", "path": "a.py", "line": 3, "kind": "member", "scope": "Foo", "scopeKind": "class", "end": 5}
not json
`
	defs, err := ParseCtagsJSON(strings.NewReader(in))
	if err == nil {
		t.Fatal("ParseCtagsJSON accepted a malformed line")
	}
	defs, err = ParseCtagsJSON(strings.NewReader(strings.TrimSuffix(in, "not json\n")))
	if err != nil {
		t.Fatal(err)
	}
	want := []OracleDef{
		{Path: "a.py", Name: "Foo", Line: 1, End: 9, Kind: "class"},
		{Path: "a.py", Name: "bar", Line: 3, End: 5, Kind: "member", Scoped: true},
	}
	if len(defs) != 2 || defs[0] != want[0] || defs[1] != want[1] {
		t.Fatalf("defs = %+v, want %+v", defs, want)
	}
}

func TestBareName(t *testing.T) {
	for in, want := range map[string]string{
		"Foo": "Foo", "Foo.bar": "bar", "A::B::c": "c", "(*T).M": "M", "a/b.c#d": "d", "Cls.__init__": "__init__",
	} {
		if got := BareName(in); got != want {
			t.Errorf("BareName(%q) = %q, want %q", in, got, want)
		}
	}
}

// TestScoreL1 (§1.4): same file, same bare name, |Δline| ≤ 2, nearest
// wins one-to-one; kinds outside the intersection are excluded from both
// sides and reported.
func TestScoreL1(t *testing.T) {
	spec := pySpec(t)
	nodes := []core.SymbolNode{
		{Name: "Foo", Kind: core.KindPyClass, FilePath: "a.py", Line: 1, EndLine: 9},
		{Name: "Foo.bar", Kind: core.KindPyFunc, FilePath: "a.py", Line: 4, EndLine: 5, Signature: "def bar(self)"},
		{Name: "Foo.bar", Kind: core.KindPyFunc, FilePath: "a.py", Line: 20}, // a second bar, too far from any oracle bar
		{Name: "helper", Kind: core.KindPyFunc, FilePath: "b.py", Line: 1},
		{Name: "chunk", Kind: core.KindDocChunk, FilePath: "a.py", Line: 1}, // outside the intersection
	}
	oracle := []OracleDef{
		{Path: "a.py", Name: "Foo", Line: 1, End: 9, Kind: "class"},
		{Path: "a.py", Name: "bar", Line: 3, End: 5, Kind: "member", Scoped: true},
		{Path: "a.py", Name: "bar", Line: 6, End: 7, Kind: "member", Scoped: true},
		{Path: "b.py", Name: "missing", Line: 8, Kind: "function"},
		{Path: "b.py", Name: "Z", Line: 1, Kind: "namespace"}, // unmapped ctags kind
	}
	r := ScoreL1(spec.Manifest, "ctags", nodes, oracle)
	want := map[string]int{
		CntOracleDefs: 4, CntFathomDefs: 4, CntMatched: 2, CntKindEqual: 2,
		CntRangeEligible: 2, CntRangeOK: 2, CntCallables: 3, CntCallablesSigned: 1,
	}
	for k, v := range want {
		if r.Counts[k] != v {
			t.Errorf("count %s = %d, want %d (all: %v)", k, r.Counts[k], v, r.Counts)
		}
	}
	if r.Excluded["ctags:namespace"] != 1 || r.Excluded["fathom:DocChunk"] != 1 {
		t.Errorf("excluded = %v", r.Excluded)
	}
	if strings.Join(r.Intersection, ",") != "class,function,variable" {
		t.Errorf("intersection = %v", r.Intersection)
	}
}

func TestScoredOracleScopedKind(t *testing.T) {
	spec, _ := IncumbentSpec("go")
	nodes := []core.SymbolNode{{Name: "T.M", Kind: core.KindGoMethod, FilePath: "a.go", Line: 3, EndLine: 5, Signature: "func (T) M()"}}
	oracle := []OracleDef{{Path: "a.go", Name: "M", Line: 3, End: 5, Kind: "func", Scoped: true}}
	r := ScoreL1(spec.Manifest, "ctags", nodes, oracle)
	if r.Counts[CntKindEqual] != 1 {
		t.Fatalf("a scoped ctags func should be a method: %v", r.Counts)
	}
}

// TestMergeSubjects: one reading per dialect; a subject re-measured
// replaces its old entry, metrics come from summed counts, tiers are the
// worst tier any contributing subject used.
func TestMergeSubjects(t *testing.T) {
	spec := pySpec(t)
	r := NewReading(spec.Manifest, "L4")
	a := Subject{Name: "o/a", Files: 10, Bytes: 2_000_000,
		Counts:   map[string]int{CntOracleDefs: 100, CntFathomDefs: 100, CntMatched: 96, CntKindEqual: 96, CntRangeEligible: 96, CntRangeOK: 90, CntCallables: 50, CntCallablesSigned: 50},
		Families: map[string]string{FamilyL1: "B"}, KindIntersection: []string{"class", "function", "variable"},
		Tenancy: &SubjectTenancy{ExtractMS: 1000, MaxFileMS: 40, PeakModulePages: 100},
	}
	b := a
	b.Name = "o/b"
	b.Counts = map[string]int{CntOracleDefs: 100, CntFathomDefs: 100, CntMatched: 94, CntKindEqual: 94, CntRangeEligible: 94, CntRangeOK: 90, CntCallables: 50, CntCallablesSigned: 50}
	MergeSubject(&r, b)
	MergeSubject(&r, a)
	MergeSubject(&r, a) // re-measured: replaces, never double-counts
	if len(r.Subjects) != 2 || r.Subjects[0].Name != "o/a" {
		t.Fatalf("subjects = %+v", r.Subjects)
	}
	if got := *r.Metrics[MSymRecall]; got != 0.95 {
		t.Errorf("sym_recall = %v, want 0.95", got)
	}
	if r.Metrics[MCallRecall] != nil {
		t.Error("call_recall measured without a calls family")
	}
	if r.LevelMet != "L1" || r.Grade.L1 != GradeExcellent {
		t.Errorf("level %s grade %+v", r.LevelMet, r.Grade)
	}
	if r.Tenancy.ThroughputMBs != 2 || r.Tenancy.MaxFileMS != 40 {
		t.Errorf("tenancy = %+v", r.Tenancy)
	}
	c := a
	c.Name = "o/c"
	c.Families = map[string]string{FamilyL1: "C"}
	MergeSubject(&r, c)
	if r.MetricTiers[MSymRecall] != "C" || r.Grade.L1 != GradeGood {
		t.Errorf("a tier-C subject must cap the merged reading: tiers %v grade %+v", r.MetricTiers, r.Grade)
	}
	if err := r.Validate(); err != nil {
		t.Fatalf("merged reading invalid: %v", err)
	}
}

func TestIncumbentSpecsCoherent(t *testing.T) {
	specs, err := Incumbents()
	if err != nil {
		t.Fatal(err)
	}
	if len(specs) == 0 {
		t.Fatal("no incumbent specs")
	}
	for _, s := range specs {
		m := s.Manifest
		if !QueueIDPattern.MatchString(m.ID) || m.QueueID != m.ID {
			t.Errorf("%s: bad id/queue_id", m.ID)
		}
		if len(s.Ctags.Languages) == 0 {
			continue
		}
		inter := m.Intersection("ctags")
		for _, k := range m.RequiredKinds {
			found := false
			for _, i := range inter {
				found = found || i == k
			}
			if !found {
				t.Errorf("%s: required kind %s outside the ctags intersection %v", m.ID, k, inter)
			}
		}
	}
}
