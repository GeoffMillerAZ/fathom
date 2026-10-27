package impact_test

import (
	"testing"

	"github.com/geoffmilleraz/fathom/internal/analysis/callgraph"
	"github.com/geoffmilleraz/fathom/internal/analysis/impact"
	"github.com/geoffmilleraz/fathom/internal/core"
)

// Field references are not indexed, so no test reaching a changed Go struct
// field is ever recorded: a field is never reported as an untested gap.
func TestTestGapNeverReportsGoField(t *testing.T) {
	nodes := []core.SymbolNode{
		{Service: "svc", Package: "p", Name: "Run", Kind: core.KindGoFunc, FilePath: "p/run.go", Line: 5},
		{Service: "svc", Package: "Opts", Name: "Opts.Run", Kind: core.KindGoField, FilePath: "p/run.go", Line: 3},
		{Service: "svc", Package: "p", Name: "TestRun", Kind: core.KindGoFunc, FilePath: "p/run_test.go", Line: 3, Calls: []string{"Run"}},
	}
	res := impact.TestGapOn(callgraph.Build(nodes), nodes, []string{"p/run.go"})
	if len(res.Gap) != 0 {
		t.Fatalf("test gap = %+v, want none: Run is tested and the field Opts.Run's references are not indexed", res.Gap)
	}
	if len(res.AffectedTests) != 1 {
		t.Fatalf("affected tests = %+v, want TestRun", res.AffectedTests)
	}
}
