package langbench

import (
	"slices"
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/core"
)

// TestAdjudicate (owner decision D3): a dialect-only definition the oracle
// is known to miss is admitted to the adjudicated set only by a
// deterministic check — fathom emitted it at exactly that file:line, the
// oracle matched nothing there, and the definition's source text matches
// its name (and the class's pattern, when the class gives one). Admitted
// entries leave sym_precision's denominator and are listed; the rest are
// refused with the reason, never admitted by anyone's judgment.
func TestAdjudicate(t *testing.T) {
	m, err := ParseManifest([]byte(`{"id":"java","kind":"language",
		"kinds":{"class":{"source_kind":"JavaClass","canonical":"class","container":true},
		         "method":{"source_kind":"JavaMethod","canonical":"method","callable":true}},
		"required_kinds":["class","method"],
		"oracle_kinds":{"ctags":{"class":"class","method":"method"}}}`))
	if err != nil {
		t.Fatal(err)
	}
	src := map[string]string{"T.java": strings.Join([]string{
		"class T {",                   // 1
		"  void test() {",             // 2
		"    class Local {",           // 3
		"      int f() { return 1; }", // 4
		"    }",                       // 5
		"  }",                         // 6
		"}",                           // 7
	}, "\n")}
	nodes := []core.SymbolNode{
		{Name: "T", Kind: "JavaClass", FilePath: "T.java", Line: 1, EndLine: 7},
		{Name: "T.test", Kind: "JavaMethod", FilePath: "T.java", Line: 2, EndLine: 6, Signature: "void test()"},
		{Name: "Local", Kind: "JavaClass", FilePath: "T.java", Line: 3, EndLine: 5},
		{Name: "Local.f", Kind: "JavaMethod", FilePath: "T.java", Line: 4, EndLine: 4, Signature: "int f()"},
	}
	defs := []OracleDef{{Path: "T.java", Name: "T", Line: 1, Kind: "class", End: 7}, {Path: "T.java", Name: "test", Line: 2, Kind: "method", End: 6}}
	l1 := ScoreL1(m, "ctags", nodes, defs)
	adj := Adjudication{
		Classes: map[string]AdjudicationClass{
			"local-class": {Why: "ctags 6.2.1 does not tag a class declared in a method body", Pattern: `\bclass\s+{name}\b`},
		},
		Definitions: []AdjudicatedDef{
			{Path: "T.java", Line: 3, Name: "Local", Class: "local-class"}, // admitted
			{Path: "T.java", Line: 4, Name: "f", Class: "local-class"},     // text has no "class f"
			{Path: "T.java", Line: 2, Name: "test", Class: "local-class"},  // the oracle matched it
			{Path: "T.java", Line: 9, Name: "Ghost", Class: "local-class"}, // fathom emitted nothing there
			{Path: "T.java", Line: 4, Name: "f", Class: "no-such-class"},   // unknown class
		},
	}
	accepted, refused := Adjudicate(&l1, nodes, adj, func(p string) []byte { return []byte(src[p]) })
	if !slices.Equal(accepted, []string{"T.java:3 Local (local-class)"}) {
		t.Fatalf("accepted = %v", accepted)
	}
	if len(refused) != 4 {
		t.Fatalf("refused = %v, want 4", refused)
	}
	if l1.Counts[CntAdjudicated] != 1 {
		t.Fatalf("counts = %v", l1.Counts)
	}
	r := NewReading(m, "L1")
	MergeSubject(&r, Subject{Name: "s", Files: 1, Bytes: 1 << 20, Counts: l1.Counts, Families: map[string]string{FamilyL1: "B"},
		KindIntersection: l1.Intersection, Adjudicated: accepted, Tenancy: &SubjectTenancy{ExtractMS: 1, ExtractCPUMS: 1, MaxFileMS: 1}})
	// 2 matched of 4 fathom definitions, one adjudicated: 2/3.
	if got := *r.Metrics[MSymPrecision]; got != 0.6667 {
		t.Fatalf("sym_precision = %v, want 0.6667", got)
	}
	if !slices.ContainsFunc(r.Notes, func(n string) bool { return strings.Contains(n, "adjudicated") }) {
		t.Errorf("notes %v do not disclose the adjudicated set", r.Notes)
	}
	if err := r.Validate(); err != nil {
		t.Fatal(err)
	}
}
