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
//
// An admission is valid only for a definition of the kind the reading scores
// that the oracle genuinely failed to report — never for a dialect's
// over-extraction the oracle deliberately excludes (locals, callback
// helpers, object-literal keys). The audit of the first adjudicated sets
// found 69% (TypeScript) and 29% (JavaScript) of a 45-entry sample were
// such locals, so a class must name its scope and "module" is checked
// structurally: the definition's line starts at column 0 and no definition
// fathom emitted encloses it.
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
		"class Other {}",              // 8
		"  class Stray {}",            // 9: indented, enclosed by nothing emitted (a callback's local)
	}, "\n")}
	nodes := []core.SymbolNode{
		{Name: "T", Kind: "JavaClass", FilePath: "T.java", Line: 1, EndLine: 7},
		{Name: "T.test", Kind: "JavaMethod", FilePath: "T.java", Line: 2, EndLine: 6, Signature: "void test()"},
		{Name: "Local", Kind: "JavaClass", FilePath: "T.java", Line: 3, EndLine: 5},
		{Name: "Local.f", Kind: "JavaMethod", FilePath: "T.java", Line: 4, EndLine: 4, Signature: "int f()"},
		{Name: "Other", Kind: "JavaClass", FilePath: "T.java", Line: 8, EndLine: 8},
		{Name: "Stray", Kind: "JavaClass", FilePath: "T.java", Line: 9, EndLine: 9},
	}
	defs := []OracleDef{{Path: "T.java", Name: "T", Line: 1, Kind: "class", End: 7}, {Path: "T.java", Name: "test", Line: 2, Kind: "method", End: 6}}
	l1 := ScoreL1(m, "ctags", nodes, defs)
	adj := Adjudication{
		Classes: map[string]AdjudicationClass{
			"top-class":   {Why: "the dumper skips a second top-level class", Pattern: `\bclass\s+{name}\b`, Scope: "module"},
			"local-class": {Why: "ctags 6.2.1 does not tag a class declared in a method body", Pattern: `\bclass\s+{name}\b`},
		},
		Definitions: []AdjudicatedDef{
			{Path: "T.java", Line: 8, Name: "Other", Class: "top-class"},   // admitted
			{Path: "T.java", Line: 3, Name: "Local", Class: "top-class"},   // enclosed by T.test: a local
			{Path: "T.java", Line: 9, Name: "Stray", Class: "top-class"},   // indented: not module scope
			{Path: "T.java", Line: 3, Name: "Local", Class: "local-class"}, // the class names no scope
			{Path: "T.java", Line: 4, Name: "f", Class: "local-class"},     // text has no "class f"
			{Path: "T.java", Line: 2, Name: "test", Class: "local-class"},  // the oracle matched it
			{Path: "T.java", Line: 9, Name: "Ghost", Class: "local-class"}, // fathom emitted nothing there
			{Path: "T.java", Line: 4, Name: "f", Class: "no-such-class"},   // unknown class
		},
	}
	accepted, refused := Adjudicate(&l1, nodes, adj, func(p string) []byte { return []byte(src[p]) })
	if !slices.Equal(accepted, []string{"T.java:8 Other (top-class)"}) {
		t.Fatalf("accepted = %v", accepted)
	}
	if len(refused) != 7 {
		t.Fatalf("refused = %v, want 7", refused)
	}
	for _, want := range []string{"T.java:3 Local (top-class): not at module scope", "T.java:9 Stray (top-class): not at module scope", "T.java:3 Local (local-class): the class names no scope"} {
		if !slices.ContainsFunc(refused, func(r string) bool { return strings.HasPrefix(r, want) }) {
			t.Errorf("refused %v lacks %q", refused, want)
		}
	}
	if l1.Counts[CntAdjudicated] != 1 {
		t.Fatalf("counts = %v", l1.Counts)
	}
	r := NewReading(m, "L1")
	MergeSubject(&r, Subject{Name: "s", Files: 1, Bytes: 1 << 20, Counts: l1.Counts, Families: map[string]string{FamilyL1: "B"},
		KindIntersection: l1.Intersection, Adjudicated: accepted, Tenancy: &SubjectTenancy{ExtractMS: 1, ExtractCPUMS: 1, MaxFileMS: 1}})
	// 2 matched of 6 fathom definitions, one adjudicated: 2/5.
	if got := *r.Metrics[MSymPrecision]; got != 0.4 {
		t.Fatalf("sym_precision = %v, want 0.4", got)
	}
	if !slices.ContainsFunc(r.Notes, func(n string) bool { return strings.Contains(n, "adjudicated") }) {
		t.Errorf("notes %v do not disclose the adjudicated set", r.Notes)
	}
	if err := r.Validate(); err != nil {
		t.Fatal(err)
	}
}
