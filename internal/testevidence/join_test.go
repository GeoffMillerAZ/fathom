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

// TestJoinCasesToSymbols_DisambiguatesByPackage pins CV-72 (design §3.5,
// CV-M13's control measurement: "34 of 566 cases" never joined because a
// bare name was shared across packages). Case ids use surveyor's real
// go-test-json shape (control/r1/envelope.json: "<module import
// path>/<dir>.<TestFunc>") — the fixture reuses that exact id from the
// calibration data.
func TestJoinCasesToSymbols_DisambiguatesByPackage(t *testing.T) {
	symbols := []core.SymbolNode{
		{Service: "svc", Package: "canon", Name: "TestParse", Kind: core.KindGoFunc, FilePath: "internal/canon/canon_test.go", Line: 4},
		{Service: "svc", Package: "widgets", Name: "TestParse", Kind: core.KindGoFunc, FilePath: "internal/widgets/widgets_test.go", Line: 9},
	}

	cases := []Case{
		// A real surveyor-shaped id (CV-M13 control/r1/envelope.json's own
		// convention): unambiguously names the "canon" package.
		{ID: "github.com/geoffmilleraz/conjure/internal/canon.TestParse", Name: "TestParse", Status: "pass"},
		// Same bare name, the OTHER package.
		{ID: "github.com/geoffmilleraz/conjure/internal/widgets.TestParse", Name: "TestParse", Status: "pass"},
		// A subtest under the widgets package's TestParse still resolves
		// to the SAME symbol, by its own (parent) id's package.
		{ID: "github.com/geoffmilleraz/conjure/internal/widgets.TestParse", Name: "TestParse/sub", Status: "pass"},
		// An id naming a package neither candidate's directory matches —
		// stays unjoined, never guessed.
		{ID: "github.com/geoffmilleraz/conjure/internal/other.TestParse", Name: "TestParse", Status: "pass"},
		// An id with no "." at all (no package to read) — stays unjoined.
		{ID: "TestParse", Name: "TestParse", Status: "pass"},
	}

	joined, stats := JoinCasesToSymbols(cases, "unit", symbols)
	if stats.Total != 5 {
		t.Fatalf("Total = %d, want 5", stats.Total)
	}
	if stats.Joined != 3 {
		t.Fatalf("Joined = %d, want 3 (canon TestParse, widgets TestParse, widgets TestParse/sub)", stats.Joined)
	}

	wantCanon := core.SymbolNode{Service: "svc", Package: "canon", Name: "TestParse", FilePath: "internal/canon/canon_test.go"}.Key()
	wantWidgets := core.SymbolNode{Service: "svc", Package: "widgets", Name: "TestParse", FilePath: "internal/widgets/widgets_test.go"}.Key()

	if joined[0].TestSymbolKey != wantCanon {
		t.Errorf("canon case joined to %q, want %q", joined[0].TestSymbolKey, wantCanon)
	}
	if joined[1].TestSymbolKey != wantWidgets {
		t.Errorf("widgets case joined to %q, want %q", joined[1].TestSymbolKey, wantWidgets)
	}
	if joined[2].TestSymbolKey != wantWidgets {
		t.Errorf("widgets subtest case joined to %q, want %q", joined[2].TestSymbolKey, wantWidgets)
	}
	if joined[3].TestSymbolKey != "" {
		t.Errorf("unmatched-package case joined to %q, want unjoined", joined[3].TestSymbolKey)
	}
	if joined[4].TestSymbolKey != "" {
		t.Errorf("id-with-no-package case joined to %q, want unjoined", joined[4].TestSymbolKey)
	}
}
