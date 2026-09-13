package mcp

import "testing"

// TestScopeRequired_UnderBudget_NoScope: a modest fan-in never trips
// scopeRequired — fitToolBudget's ordinary page-and-elide handles it.
func TestScopeRequired_UnderBudget_NoScope(t *testing.T) {
	byPkg := map[string]int{"pkg/a": 5, "pkg/b": 3}
	need, sum := scopeRequired(8, defaultCharBudget, byPkg)
	if need {
		t.Fatalf("scopeRequired(8, ...) = true, want false; sum=%+v", sum)
	}
	if sum.Count != 0 || sum.TopPackages != nil {
		t.Errorf("scopeRequired(8, ...) false case should return a zero summary, got %+v", sum)
	}
}

// TestScopeRequired_GenericName is RC-20's golden test: a generic symbol
// name (e.g. "New", "String") matched across a whole fleet — far more rows
// than any amount of paging would resolve. Proves the count, the top-8
// packages sorted by count descending then name ascending (deterministic
// ties), and that packages beyond 8 are dropped.
func TestScopeRequired_GenericName(t *testing.T) {
	byPkg := map[string]int{
		"svcA::pkg/handlers": 400,
		"svcB::pkg/handlers": 400, // tie with the row above: name breaks it
		"svcC::pkg/models":   250,
		"svcD::pkg/util":     120,
		"svcE::pkg/api":      90,
		"svcF::pkg/store":    80,
		"svcG::pkg/cli":      60,
		"svcH::pkg/cmd":      40,
		"svcI::pkg/testutil": 30, // 9th package: must be dropped by the cap
		"svcJ::pkg/misc":     10,
	}
	total := 0
	for _, n := range byPkg {
		total += n
	}
	need, sum := scopeRequired(total, defaultCharBudget, byPkg)
	if !need {
		t.Fatalf("scopeRequired(%d, ...) = false, want true (a generic-name fan-in)", total)
	}
	if sum.Count != total {
		t.Errorf("sum.Count = %d, want %d", sum.Count, total)
	}
	if len(sum.TopPackages) != 8 {
		t.Fatalf("len(TopPackages) = %d, want 8 (capped)", len(sum.TopPackages))
	}
	wantOrder := []struct {
		pkg   string
		count int
	}{
		{"svcA::pkg/handlers", 400}, // tie at 400: "svcA" < "svcB"
		{"svcB::pkg/handlers", 400},
		{"svcC::pkg/models", 250},
		{"svcD::pkg/util", 120},
		{"svcE::pkg/api", 90},
		{"svcF::pkg/store", 80},
		{"svcG::pkg/cli", 60},
		{"svcH::pkg/cmd", 40},
	}
	for i, want := range wantOrder {
		got := sum.TopPackages[i]
		if got.Package != want.pkg || got.Count != want.count {
			t.Errorf("TopPackages[%d] = %+v, want {%s %d}", i, got, want.pkg, want.count)
		}
	}
	for _, dropped := range []string{"svcI::pkg/testutil", "svcJ::pkg/misc"} {
		for _, got := range sum.TopPackages {
			if got.Package == dropped {
				t.Errorf("TopPackages contains %q, want it dropped past the cap of 8", dropped)
			}
		}
	}
}

// TestScopeRequired_NonPositiveBudgetUsesDefault mirrors fitToBudget's own
// convention: budget <= 0 falls back to defaultCharBudget rather than
// refusing or always tripping.
func TestScopeRequired_NonPositiveBudgetUsesDefault(t *testing.T) {
	byPkg := map[string]int{"pkg/a": 1}
	needZero, _ := scopeRequired(1, 0, byPkg)
	needDefault, _ := scopeRequired(1, defaultCharBudget, byPkg)
	if needZero != needDefault {
		t.Errorf("scopeRequired with budget=0 (%v) should match defaultCharBudget (%v)", needZero, needDefault)
	}
}

func TestTopPackages_EmptyMap(t *testing.T) {
	if got := topPackages(map[string]int{}, 8); len(got) != 0 {
		t.Errorf("topPackages(empty) = %v, want empty", got)
	}
}
