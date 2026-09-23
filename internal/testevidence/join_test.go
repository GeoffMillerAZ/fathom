package testevidence

import (
	"testing"

	"github.com/geoffmilleraz/fathom/internal/core"
)

func TestJoinCasesToSymbols_ExactNameAndSubtest(t *testing.T) {
	symbols := []core.SymbolNode{
		{Service: "svc", Package: "cart", Name: "TestAdd", Kind: core.KindGoFunc, FilePath: "cart/cart_test.go", Line: 10},
		{Service: "svc", Package: "cart", Name: "Add", Kind: core.KindGoFunc, FilePath: "cart/cart.go", Line: 5},
		// TestMain declared in two different packages — an ambiguous bare
		// name that must never be guessed at.
		{Service: "svc", Package: "cart", Name: "TestMain", Kind: core.KindGoFunc, FilePath: "cart/main_test.go", Line: 1},
		{Service: "svc", Package: "checkout", Name: "TestMain", Kind: core.KindGoFunc, FilePath: "checkout/main_test.go", Line: 1},
	}

	cases := []Case{
		{ID: "1", Name: "TestAdd", Status: "pass"},
		{ID: "2", Name: "TestAdd/subtest_one", Status: "pass"},
		{ID: "3", Name: "TestAdd/subtest_one/nested", Status: "pass"},
		{ID: "4", Name: "TestMain", Status: "pass"}, // ambiguous — must stay unjoined
		{ID: "5", Name: "TestNeverDeclared", Status: "fail"},
	}

	joined, stats := JoinCasesToSymbols(cases, "unit", symbols)

	if stats.Total != 5 {
		t.Fatalf("Total = %d, want 5", stats.Total)
	}
	if stats.Joined != 3 {
		t.Fatalf("Joined = %d, want 3 (TestAdd + its two subtests)", stats.Joined)
	}

	wantKey := core.SymbolNode{Service: "svc", Package: "cart", Name: "TestAdd", FilePath: "cart/cart_test.go"}.Key()
	for _, jc := range joined[:3] {
		if jc.TestSymbolKey != wantKey {
			t.Errorf("case %q joined to %q, want %q", jc.Name, jc.TestSymbolKey, wantKey)
		}
		if jc.SuiteName != "unit" {
			t.Errorf("case %q suite = %q, want unit", jc.Name, jc.SuiteName)
		}
	}
	if joined[3].TestSymbolKey != "" {
		t.Errorf("ambiguous TestMain case joined to %q, want unjoined (\"\")", joined[3].TestSymbolKey)
	}
	if joined[4].TestSymbolKey != "" {
		t.Errorf("never-declared case joined to %q, want unjoined (\"\")", joined[4].TestSymbolKey)
	}
}

func TestJoinCasesToSymbols_EmptyCasesOrSymbols(t *testing.T) {
	joined, stats := JoinCasesToSymbols(nil, "unit", nil)
	if len(joined) != 0 || stats.Total != 0 || stats.Joined != 0 {
		t.Fatalf("want zero-value result for no cases/symbols, got joined=%v stats=%+v", joined, stats)
	}
}
