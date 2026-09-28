package langbench

import (
	"testing"

	"google.golang.org/protobuf/proto"

	"github.com/geoffmilleraz/fathom/internal/core"
	fscip "github.com/geoffmilleraz/fathom/internal/scip"
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
