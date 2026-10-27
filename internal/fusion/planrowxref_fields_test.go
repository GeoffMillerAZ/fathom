package fusion

import (
	"testing"

	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/extractor"
)

func TestResolvePlanSymbolsFieldDoesNotCompeteWithBareFunction(t *testing.T) {
	const plan = "docs/plans/fields.md"
	files := []DocFile{{Path: plan, Result: extractor.DocResult{
		Nodes: []core.SymbolNode{
			{Service: "svc", Name: "P-1", Kind: extractor.KindPlanRow, FilePath: plan},
			{Service: "svc", Name: "P-2", Kind: extractor.KindPlanRow, FilePath: plan},
		},
		CrossFile: []extractor.DocCrossFileRef{
			{From: "P-1", Raw: "Default", Target: "pkg", Strategy: extractor.PlanStrategySymbol},
			{From: "P-2", Raw: "Capability.Default", Target: "pkg", Strategy: extractor.PlanStrategySymbol},
		},
	}}}
	symbols := []core.SymbolNode{
		{Service: "svc", Package: "sample", Name: "Default", Kind: core.KindGoFunc, FilePath: "pkg/default.go"},
		{Service: "svc", Package: "Capability", Name: "Capability.Default", Kind: core.KindGoField, FilePath: "pkg/capability.go"},
	}
	refs := ResolvePlanSymbols(files, symbols)
	if len(refs) != 2 {
		t.Fatalf("resolved refs = %v, want both anchors", refs)
	}
	for _, ref := range refs {
		switch ref.From {
		case "P-1":
			if ref.ToKey != symbols[0].Key() {
				t.Errorf("bare Default resolved to %q, want function %q", ref.ToKey, symbols[0].Key())
			}
		case "P-2":
			if ref.ToKey != symbols[1].Key() {
				t.Errorf("qualified Capability.Default resolved to %q, want field %q", ref.ToKey, symbols[1].Key())
			}
		default:
			t.Errorf("unexpected plan row %q", ref.From)
		}
	}
}
