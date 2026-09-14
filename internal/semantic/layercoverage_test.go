package semantic

// layercoverage_test.go — O54 deliverable 2, unit-level: ObligationsLayerCoverage
// and NearestUncompiledGuide pinned directly, ahead of the fuller Compile()
// integration proof in compile_test.go and the MCP-level wiring test.

import (
	"context"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/store"
)

func TestObligationsLayerCoverage_AbsentWhenNeverCompiled(t *testing.T) {
	ctx := context.Background()
	st := store.NewMemoryStore()
	cov, found, err := ObligationsLayerCoverage(ctx, st, "svc", LayerObligations)
	if err != nil {
		t.Fatalf("ObligationsLayerCoverage: %v", err)
	}
	if found {
		t.Fatalf("found = true, want false (no compile_runs row at all) — cov=%+v", cov)
	}
}

func TestObligationsLayerCoverage_ReadsLatestRun(t *testing.T) {
	ctx := context.Background()
	st := store.NewMemoryStore()
	older := core.CompileRun{
		RunID: "r1", ServiceID: "svc", Layer: LayerObligations,
		RanAt: fixedNow().Add(-1), SectionsTotal: 10, SectionsCompiled: 10,
	}
	newer := core.CompileRun{
		RunID: "r2", ServiceID: "svc", Layer: LayerObligations,
		RanAt: fixedNow(), SectionsTotal: 60, SectionsCompiled: 18,
		BudgetStopped: true, UncompiledScopes: []string{"ADR.md", "docs/plans/README.md"},
	}
	if err := st.RecordCompileRun(ctx, older); err != nil {
		t.Fatal(err)
	}
	if err := st.RecordCompileRun(ctx, newer); err != nil {
		t.Fatal(err)
	}
	cov, found, err := ObligationsLayerCoverage(ctx, st, "svc", LayerObligations)
	if err != nil || !found {
		t.Fatalf("found=%v err=%v", found, err)
	}
	if cov.SectionsTotal != 60 || cov.SectionsCompiled != 18 || !cov.BudgetStopped {
		t.Fatalf("cov = %+v, want the NEWER run's numbers", cov)
	}
	if len(cov.UncompiledScopes) != 2 || cov.UncompiledScopes[0] != "ADR.md" {
		t.Fatalf("cov.UncompiledScopes = %v", cov.UncompiledScopes)
	}
}

func TestNearestUncompiledGuide(t *testing.T) {
	cov := LayerCoverage{
		SectionsTotal: 60, SectionsCompiled: 18, BudgetStopped: true,
		UncompiledScopes: []string{"ADR.md", "internal/CLAUDE.md", "internal/extractor/CLAUDE.md"},
	}
	cases := []struct {
		name      string
		file      string
		wantGuide string
		wantOK    bool
	}{
		{"root-scope call has no file", "", "", false},
		{"a file under the deepest uncompiled guide gets the DEEPEST match", "internal/extractor/goextract.go", "internal/extractor/CLAUDE.md", true},
		{"a file only under the shallower guide gets that one", "internal/mcp/tools.go", "internal/CLAUDE.md", true},
		{"a file under no uncompiled guide at all", "cmd/fathom/compile.go", "", false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			guide, ok := NearestUncompiledGuide(cov, c.file)
			if ok != c.wantOK || guide != c.wantGuide {
				t.Errorf("NearestUncompiledGuide(file=%q) = (%q, %v), want (%q, %v)", c.file, guide, ok, c.wantGuide, c.wantOK)
			}
		})
	}
}

// TestNearestUncompiledGuide_RootTailNeverCountsAsAGuide: when the only
// uncompiled scope is a root document (never a nested guide), NO file ever
// gets a coverage note from it — a root tail is not "the nearest guide"
// for anything.
func TestNearestUncompiledGuide_RootTailNeverCountsAsAGuide(t *testing.T) {
	cov := LayerCoverage{SectionsTotal: 3, SectionsCompiled: 2, BudgetStopped: true, UncompiledScopes: []string{"ADR.md"}}
	if guide, ok := NearestUncompiledGuide(cov, "internal/extractor/goextract.go"); ok {
		t.Fatalf("NearestUncompiledGuide = (%q, true), want ok=false", guide)
	}
}
