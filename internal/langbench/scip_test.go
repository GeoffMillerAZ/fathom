package langbench

import (
	"reflect"
	"testing"

	"google.golang.org/protobuf/proto"

	"github.com/geoffmilleraz/fathom/internal/core"
	fscip "github.com/geoffmilleraz/fathom/internal/scip"
	scippb "github.com/geoffmilleraz/fathom/third_party/scip"
)

func tsFixtureNodes() []core.SymbolNode {
	return []core.SymbolNode{
		{Service: "s", Name: "add", Kind: core.KindJSFunc, FilePath: "math.ts", Line: 1},
		{Service: "s", Name: "Greeter", Kind: core.KindJSClass, FilePath: "greeter.ts", Line: 3},
		{Service: "s", Name: "Greeter.greet", Kind: core.KindJSMethod, FilePath: "greeter.ts", Line: 4, Calls: []string{"add", "console.log"}},
		{Service: "s", Name: "run", Kind: core.KindJSFunc, FilePath: "main.ts", Line: 3, Calls: []string{"g.greet", "made.up"}},
	}
}

// TestScoreSCIP (§1.2 L2–L4 rows, §1.4 calls): call truth from callable
// references inside mapped definitions; definitions mapped by position and
// name; cross-file edges at file granularity.
func TestScoreSCIP(t *testing.T) {
	raw, err := proto.Marshal(fscip.BuildTSFixtureIndexNoDisplayNames())
	if err != nil {
		t.Fatal(err)
	}
	res, err := ScoreSCIP(raw, "s", tsFixtureNodes())
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]int{
		CntScipDefs: 4, CntScipDefsMapped: 4,
		CntCallTruth: 3, CntCallTruthFound: 3, CntCallFathom: 4, CntCallFathomInTrue: 3,
	}
	for k, v := range want {
		if res.Counts[k] != v {
			t.Errorf("%s = %d, want %d (all %v)", k, res.Counts[k], v, res.Counts)
		}
	}
	if res.Counts[CntScipEdges] == 0 || res.Counts[CntScipEdgesTyped] > res.Counts[CntScipEdges] {
		t.Errorf("typed counts %v", res.Counts)
	}
	if !res.FileEdges[[2]string{"greeter.ts", "math.ts"}] || !res.FileEdges[[2]string{"main.ts", "greeter.ts"}] || len(res.FileEdges) != 2 {
		t.Errorf("file edges = %v", res.FileEdges)
	}
	if res.Tool != "scip-typescript" {
		t.Errorf("tool = %q", res.Tool)
	}

	fathomEdges := []core.Edge{
		{FromFile: "main.ts", ToSymbol: tsFixtureNodes()[1].Key()},
		{FromFile: "main.ts", ToSymbol: tsFixtureNodes()[0].Key()},
	}
	counts := map[string]int{}
	ScoreFileEdges(res.FileEdges, fathomEdges, tsFixtureNodes(), counts)
	if counts[CntFileEdgesTruth] != 2 || counts[CntFileEdgesFathom] != 2 || counts[CntFileEdgesAgree] != 1 {
		t.Errorf("file edge counts = %v", counts)
	}
}

func TestScipBareName(t *testing.T) {
	for in, want := range map[string]string{
		"scip-typescript npm p 1 math/add().":      "add",
		"scip-typescript npm p 1 greeter/Greeter#": "Greeter",
		"scip-python python p 1 `a.b`/C#m().":      "m",
		"scip-java maven g a 1 pkg/Outer#Inner#f.": "f",
		"local 3": "",
	} {
		if got, _ := scipBareName(in); got != want {
			t.Errorf("scipBareName(%q) = %q, want %q", in, got, want)
		}
	}
}

// TestScoreSCIPEnclosingFallback (owner decision D5): a producer that never
// sets occurrence.enclosing_range (scip-clang 0.4.0) still yields call truth
// — a definition mapped to a fathom definition takes that definition's
// line span as its enclosing range, and the result counts how many did.
func TestScoreSCIPEnclosingFallback(t *testing.T) {
	idx := fscip.BuildTSFixtureIndexNoDisplayNames()
	for _, d := range idx.Documents {
		for _, o := range d.Occurrences {
			o.EnclosingRange = nil
		}
	}
	raw, err := proto.Marshal(idx)
	if err != nil {
		t.Fatal(err)
	}
	nodes := tsFixtureNodes()
	for i, end := range []int{3, 8, 7, 6} {
		nodes[i].EndLine = end
	}
	res, err := ScoreSCIP(raw, "s", nodes)
	if err != nil {
		t.Fatal(err)
	}
	if res.Counts[CntCallTruth] != 3 || res.Counts[CntCallTruthFound] != 3 {
		t.Fatalf("call truth without enclosing ranges: %v", res.Counts)
	}
	if res.EnclosingFallback != 4 {
		t.Fatalf("EnclosingFallback = %d, want 4", res.EnclosingFallback)
	}
}

// TestSCIPDefs (owner decision D2): a SCIP index as a tier-A L1 oracle —
// every definition with its name, 1-based line, enclosing end line and a
// kind from SymbolInformation.kind, else the documentation's declaration
// keyword, else the descriptor suffix; scoped when its parent is a type.
func TestSCIPDefs(t *testing.T) {
	idx := fscip.BuildTSFixtureIndexNoDisplayNames()
	// greet documents itself the way scip-typescript's hover text does.
	idx.Documents[1].Symbols[1].Documentation = []string{"```ts\n(method) Greeter.greet(name: string): void\n```"}
	idx.Documents[1].Symbols[0].Kind = scippb.SymbolInformation_Class
	raw, err := proto.Marshal(idx)
	if err != nil {
		t.Fatal(err)
	}
	defs, tool, _, err := SCIPDefs(raw)
	if err != nil {
		t.Fatal(err)
	}
	if tool != "scip-typescript" {
		t.Errorf("tool %q", tool)
	}
	want := []OracleDef{
		{Path: "greeter.ts", Name: "Greeter", Line: 3, End: 8, Kind: "class"},
		{Path: "greeter.ts", Name: "greet", Line: 4, End: 7, Kind: "method", Scoped: true},
		{Path: "main.ts", Name: "run", Line: 3, End: 6, Kind: "function"},
		{Path: "math.ts", Name: "add", Line: 1, End: 3, Kind: "function"},
	}
	if !reflect.DeepEqual(defs, want) {
		t.Fatalf("SCIPDefs =\n%+v\nwant\n%+v", defs, want)
	}
	for in, want := range map[string]string{
		"```ts\nclass Foo\n```":                "class",
		"```ts\n(property) Foo.x: number\n```": "property",
		"```java\npublic interface I\n```":     "interface",
		"```ts\nconst x: 1\n```":               "const",
		"just prose, no declaration keyword":   "",
	} {
		if got := docKeyword(in); got != want {
			t.Errorf("docKeyword(%q) = %q, want %q", in, got, want)
		}
	}
}
