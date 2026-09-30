package langbench

import (
	"slices"
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
{"_type": "tag", "name": "F", "path": "a.go", "line": 7, "kind": "func", "scope": "pkg", "scopeKind": "package"}
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
		{Path: "a.go", Name: "F", Line: 7, Kind: "func"},
	}
	if len(defs) != 3 || defs[0] != want[0] || defs[1] != want[1] || defs[2] != want[2] {
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

// TestBareNameOperators (LB-Q-cpp): an operator is one name however a tool
// spells it — source "operator+", universal-ctags "operator +" — and the ")"
// that ends "operator()" is part of it, not a separator.
func TestBareNameOperators(t *testing.T) {
	for in, want := range map[string]string{
		"operator+": "operator+", "operator +": "operator+", "Foo.operator+": "operator+",
		"Foo::operator +=": "operator+=", "operator ()": "operator()", "operator()": "operator()",
		"ns.Foo.operator()": "operator()", "operator []": "operator[]", "Foo.operator->": "operator->",
		"operator <=>": "operator<=>", "Foo.operator<<": "operator<<", "operator ,": "operator,",
		// named operators and conversions keep the last-segment rule
		"operator new": "new", "Foo.operator bool": "bool",
		// not operators
		"my_operator": "my_operator", "operators.x": "x", "operator": "operator", "a.operator": "operator",
		"operator.itemgetter": "itemgetter",
	} {
		if got := BareName(in); got != want {
			t.Errorf("BareName(%q) = %q, want %q", in, got, want)
		}
	}
}

// TestScoreL1OperatorSpellings: an operator's spelling is canonical on both
// sides of the L1 match ("operator+" from source, "operator +" from ctags).
func TestScoreL1OperatorSpellings(t *testing.T) {
	m := Manifest{ID: "cpp", RequiredKinds: []string{"function"},
		Kinds:       map[string]KindSpec{"function": {SourceKind: "CppFunc", Canonical: "function", Callable: true}},
		OracleKinds: map[string]map[string]string{"ctags": {"function": "function"}}}
	nodes := []core.SymbolNode{
		{FilePath: "a.hpp", Name: "Box.operator+", Kind: "CppFunc", Line: 3, EndLine: 4},
		{FilePath: "a.hpp", Name: "Box.operator()", Kind: "CppFunc", Line: 6, EndLine: 7},
	}
	defs := []OracleDef{
		{Path: "a.hpp", Name: "operator +", Line: 3, End: 4, Kind: "function"},
		{Path: "a.hpp", Name: "operator ()", Line: 6, End: 7, Kind: "function"},
	}
	res := ScoreL1(m, "ctags", nodes, defs)
	if res.Counts[CntMatched] != 2 {
		t.Fatalf("matched = %d, want 2 (dialect-only %v, oracle-only %v)", res.Counts[CntMatched], res.DialectOnly, res.OracleOnly)
	}
}

// TestNameMatchIdentity (§1.4): a dialect whose identifiers carry
// punctuation (CSS selectors, npm names, script names, export paths)
// declares name_match "identity"; its names are compared whole. The
// default keeps the last-segment rule.
func TestNameMatchIdentity(t *testing.T) {
	src := `{"id":"css","kind":"language",%s
		"kinds":{"selector":{"source_kind":"CSSSelector","canonical":"selector"}},
		"required_kinds":["selector"],"oracle_kinds":{"ctags":{"class":"selector","id":"selector","selector":"selector"}}}`
	names := []string{".a", "#b", "a:hover", "@babel/core", "codesandbox:build", "./utils"}
	var nodes []core.SymbolNode
	var defs []OracleDef
	for i, n := range names {
		nodes = append(nodes, core.SymbolNode{Name: n, Kind: "CSSSelector", FilePath: "x.css", Line: 10 * (i + 1)})
		defs = append(defs, OracleDef{Path: "x.css", Name: n, Kind: "selector", Line: 10 * (i + 1)})
	}
	for _, tc := range []struct {
		field string
		want  int
	}{{`"name_match":"identity",`, len(names)}, {``, 0}} {
		m, err := ParseManifest([]byte(strings.Replace(src, "%s", tc.field, 1)))
		if err != nil {
			t.Fatal(err)
		}
		if got := ScoreL1(m, "ctags", nodes, defs).Counts[CntMatched]; got != tc.want {
			t.Errorf("name_match %q: matched %d, want %d", tc.field, got, tc.want)
		}
	}
	m, _ := ParseManifest([]byte(strings.Replace(src, "%s", `"name_match":"identity",`, 1)))
	if got := m.MatchName("@babel/core"); got != "@babel/core" {
		t.Errorf("identity MatchName = %q", got)
	}
	if got := (Manifest{}).MatchName("a.b"); got != "b" {
		t.Errorf("default MatchName = %q", got)
	}
	pairs := m.CallPairs([]core.SymbolNode{{Name: "codesandbox:build", FilePath: "package.json", Calls: []string{"build:js"}}}, nil)
	if !pairs[CallPair{Path: "package.json", Caller: "codesandbox:build", Callee: "build:js"}] {
		t.Errorf("identity call pairs = %v", pairs)
	}
	if _, err := ParseManifest([]byte(strings.Replace(src, "%s", `"name_match":"exact",`, 1))); err == nil {
		t.Error("an unknown name_match was accepted")
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

// TestMergeRangeNotMeasured (§1.2): an L1 oracle that reports no end line
// for any matched definition leaves range_agreement not_measured — never a
// failure — L1 is met at most at good, and the reading says why.
func TestMergeRangeNotMeasured(t *testing.T) {
	spec := pySpec(t)
	r := NewReading(spec.Manifest, "L1")
	s := Subject{Name: "o/a", Files: 10, Bytes: 2_000_000,
		Counts:   map[string]int{CntOracleDefs: 100, CntFathomDefs: 100, CntMatched: 99, CntKindEqual: 99, CntCallables: 50, CntCallablesSigned: 50},
		Families: map[string]string{FamilyL1: "B"}, KindIntersection: []string{"class", "function", "variable"},
		Tenancy: &SubjectTenancy{ExtractMS: 1000, MaxFileMS: 40, PeakModulePages: 100},
	}
	MergeSubject(&r, s)
	if r.Metrics[MRangeAgreement] != nil || len(r.NotMeasured) != 1 || r.NotMeasured[0] != MRangeAgreement {
		t.Fatalf("range_agreement %v not_measured %v", r.Metrics[MRangeAgreement], r.NotMeasured)
	}
	if r.LevelMet != "L1" || r.Grade.L1 != GradeGood {
		t.Errorf("level %s grade %+v, want L1 good", r.LevelMet, r.Grade)
	}
	if !slices.Contains(r.Notes, NoteRangeNotMeasured) {
		t.Errorf("notes %v lack the disclosure", r.Notes)
	}
	if err := r.Validate(); err != nil {
		t.Fatalf("Validate: %v", err)
	}
	// Once an oracle reports ends, the metric is measured and the note goes.
	s.Counts = map[string]int{CntOracleDefs: 100, CntFathomDefs: 100, CntMatched: 99, CntKindEqual: 99, CntRangeEligible: 99, CntRangeOK: 99, CntCallables: 50, CntCallablesSigned: 50}
	MergeSubject(&r, s)
	if r.NotMeasured != nil || slices.Contains(r.Notes, NoteRangeNotMeasured) || r.Grade.L1 != GradeExcellent {
		t.Errorf("measured again: not_measured %v notes %v grade %+v", r.NotMeasured, r.Notes, r.Grade)
	}
}

// TestRequiredKindsThroughCanonical (§1.4): required_kinds are the
// manifest's own kind keys; the refusal checks them through the canonical
// mapping, so a key never has to be renamed to its canonical string.
func TestRequiredKindsThroughCanonical(t *testing.T) {
	m, err := ParseManifest([]byte(`{"id":"swift","kind":"language",
		"kinds":{"protocol":{"source_kind":"SwiftProtocol","canonical":"interface"},
		         "function":{"source_kind":"SwiftFunc","canonical":"function","callable":true}},
		"required_kinds":["protocol","function"],
		"oracle_kinds":{"ctags":{"protocol":"interface","func":"function"}}}`))
	if err != nil {
		t.Fatal(err)
	}
	if got := m.RequiredCanonical(); !slices.Equal(got, []string{"function", "interface"}) {
		t.Fatalf("RequiredCanonical = %v", got)
	}
	r := NewReading(m, "L1")
	nodes := []core.SymbolNode{{Name: "P", Kind: "SwiftProtocol", FilePath: "a.swift", Line: 1, EndLine: 3},
		{Name: "f", Kind: "SwiftFunc", FilePath: "a.swift", Line: 5, EndLine: 6, Signature: "func f()"}}
	defs := []OracleDef{{Path: "a.swift", Name: "P", Kind: "protocol", Line: 1, End: 3}, {Path: "a.swift", Name: "f", Kind: "func", Line: 5, End: 6}}
	l1 := ScoreL1(m, "ctags", nodes, defs)
	MergeSubject(&r, Subject{Name: "s", Files: 1, Bytes: 1 << 20, Counts: l1.Counts, Families: map[string]string{FamilyL1: "B"},
		KindIntersection: l1.Intersection, Tenancy: &SubjectTenancy{ExtractMS: 1, MaxFileMS: 1}})
	if err := r.Validate(); err != nil {
		t.Fatalf("a required key mapped into the intersection was refused: %v", err)
	}
	r.RequiredKinds = append(r.RequiredKinds, "enum")
	if err := r.Validate(); err == nil {
		t.Fatal("a required kind outside the intersection was accepted")
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
		for _, k := range m.RequiredCanonical() {
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

// TestComparePortParity (§11 step 2): a gated L1 metric below the
// incumbent's, or measured by the incumbent and not by the dialect, refuses
// the merge; so does an unmet target; ungated metrics only report.
func TestComparePortParity(t *testing.T) {
	mk := func(level string, m map[string]*float64) Reading {
		return Reading{Dialect: "x", Kind: "language", LevelMet: level, Metrics: m}
	}
	inc := mk("L1", map[string]*float64{MSymRecall: f(0.90), MSymPrecision: f(0.90), MRangeAgreement: f(0.80), MCallRecall: f(0.9)})
	good := mk("L2", map[string]*float64{MSymRecall: f(0.95), MSymPrecision: f(0.90), MRangeAgreement: f(0.85), MCallRecall: f(0.5)})
	if p := ComparePortParity(inc, good, "old", "L2", map[string]int{"K": 1}, map[string]int{"K": 2, "N": 1}); !p.Merge || len(p.KindsGained) != 1 || len(p.KindsLost) != 0 {
		t.Fatalf("better dialect refused: %+v", p)
	}
	worse := mk("L2", map[string]*float64{MSymRecall: f(0.89), MSymPrecision: f(0.90), MRangeAgreement: f(0.85)})
	if p := ComparePortParity(inc, worse, "old", "L2", nil, nil); p.Merge || len(p.Reasons) != 1 || !strings.Contains(p.Reasons[0], MSymRecall) {
		t.Fatalf("worse sym_recall merged: %+v", p)
	}
	unmeasured := mk("L2", map[string]*float64{MSymRecall: f(0.95), MSymPrecision: f(0.95)})
	if p := ComparePortParity(inc, unmeasured, "old", "L2", nil, nil); p.Merge {
		t.Fatalf("range measured by the incumbent only merged: %+v", p)
	}
	if p := ComparePortParity(inc, good, "old", "L3", map[string]int{"Gone": 1}, nil); p.Merge || p.KindsLost[0] != "Gone" {
		t.Fatalf("unmet target merged: %+v", p)
	}
}

// TestThroughputIsCPUTime (owner decision D1, 2026-09-28): the throughput
// floor is judged on bytes per CPU-second of the extraction process, not
// wall clock, so a reading taken while other workers contend for the CPU is
// not failed by the contention. A subject that recorded no CPU time (a
// reading measured before D1) falls back to wall clock, and the reading
// says which basis it used.
func TestThroughputIsCPUTime(t *testing.T) {
	spec := pySpec(t)
	r := NewReading(spec.Manifest, "L1")
	s := Subject{Name: "o/a", Files: 10, Bytes: 1_000_000,
		Counts:   map[string]int{CntOracleDefs: 100, CntFathomDefs: 100, CntMatched: 99, CntKindEqual: 99, CntRangeEligible: 99, CntRangeOK: 99, CntCallables: 50, CntCallablesSigned: 50},
		Families: map[string]string{FamilyL1: "B"}, KindIntersection: []string{"class", "function", "variable"},
		// 1 MB in 1.6 s of wall (0.62 MB/s: contended) but 0.5 s of CPU (2 MB/s).
		Tenancy: &SubjectTenancy{ExtractMS: 1600, ExtractCPUMS: 500, MaxFileMS: 40, PeakModulePages: 100},
	}
	MergeSubject(&r, s)
	if r.Tenancy.ThroughputMBs != 2 || r.Tenancy.ThroughputBasis != ThroughputBasisCPU || r.Tenancy.WallThroughputMBs != 0.63 {
		t.Fatalf("tenancy = %+v, want 2 MB/s on the cpu basis (wall 0.63 disclosed)", r.Tenancy)
	}
	if r.LevelMet != "L1" {
		t.Fatalf("a contended wall clock failed the floor: level %s", r.LevelMet)
	}
	// A pre-D1 subject (no CPU time) falls back to wall clock for the whole
	// reading: a mixed basis would be neither number.
	old := s
	old.Name = "o/b"
	old.Tenancy = &SubjectTenancy{ExtractMS: 1600, MaxFileMS: 40, PeakModulePages: 100}
	MergeSubject(&r, old)
	if r.Tenancy.ThroughputBasis != ThroughputBasisWall || r.Tenancy.ThroughputMBs != 0.63 || r.LevelMet != LevelNone {
		t.Fatalf("wall fallback: tenancy %+v level %s", r.Tenancy, r.LevelMet)
	}
	if err := r.Validate(); err != nil {
		t.Fatal(err)
	}
}

// TestScoreL1DottedOracleName (LB-Q-ada): a language whose own names are dotted (an Ada child unit,
// `package GNATCOLL.Arg_Lists`) has an oracle that writes the whole expanded name where the dialect's name is
// compared on its last segment (§1.4: qualified names are compared on the last segment), so the oracle's dotted
// identifier path is compared on its last segment too. A name with any other punctuation (a selector, an npm
// name, an export path) is not a dotted path and is left whole, as TestNameMatchIdentity holds.
func TestScoreL1DottedOracleName(t *testing.T) {
	m := Manifest{ID: "ada", RequiredKinds: []string{"package"},
		Kinds:       map[string]KindSpec{"package": {SourceKind: "AdaPackage", Canonical: "package", Container: true}},
		OracleKinds: map[string]map[string]string{"ctags": {"package": "package"}}}
	nodes := []core.SymbolNode{
		{FilePath: "gnatcoll-arg_lists.ads", Name: "GNATCOLL.Arg_Lists", Kind: "AdaPackage", Line: 31, EndLine: 174},
		{FilePath: "gnatcoll-arg_lists.ads", Name: "Arg_List_Vector", Kind: "AdaPackage", Line: 160, EndLine: 165},
	}
	defs := []OracleDef{
		{Path: "gnatcoll-arg_lists.ads", Name: "GNATCOLL.Arg_Lists", Line: 31, Kind: "package"},
		{Path: "gnatcoll-arg_lists.ads", Name: "Arg_List_Vector", Line: 160, Kind: "package"},
	}
	res := ScoreL1(m, "ctags", nodes, defs)
	if res.Counts[CntMatched] != 2 {
		t.Fatalf("matched = %d, want 2 (dialect-only %v, oracle-only %v)", res.Counts[CntMatched], res.DialectOnly, res.OracleOnly)
	}
	for in, want := range map[string]string{"GNATCOLL.Arg_Lists": "Arg_Lists", "A.B.C": "C", "Plain": "Plain", ".a": ".a", "a:hover": "a:hover", "./utils": "./utils", "1.5": "1.5"} {
		if got := oracleMatchName(in); got != want {
			t.Errorf("oracleMatchName(%q) = %q, want %q", in, got, want)
		}
	}
}
