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

// TestJoinCasesToSymbols_SubtestNameContainingADot pins the real
// surveyor id shape a dotted subtest name produces: "<import path>.
// <TestFunc>/<subtest>", where <subtest> itself commonly contains a "."
// (a version string "v1.2", a table test keyed by a file path
// "internal/b.go" — both real shapes found in this repository's and
// conjure's own control envelopes). Cutting the id at its LAST "."
// lands inside the subtest name, matching no candidate at all, or —
// worse — suffix-matching the WRONG package when the subtest ends in
// "internal/<otherpkg>.go". Cutting by the case's own Name fixes both.
func TestJoinCasesToSymbols_SubtestNameContainingADot(t *testing.T) {
	symbols := []core.SymbolNode{
		{Service: "svc", Package: "a", Name: "TestParse", Kind: core.KindGoFunc, FilePath: "internal/a/a_test.go", Line: 4},
		{Service: "svc", Package: "b", Name: "TestParse", Kind: core.KindGoFunc, FilePath: "internal/b/b_test.go", Line: 4},
	}

	cases := []Case{
		// A dotted version-string subtest — id's LAST "." lands inside
		// "v1.2", not at the real package boundary.
		{ID: "example.com/m/internal/a.TestParse/v1.2", Name: "TestParse/v1.2", Status: "pass"},
		// A table-test subtest keyed by a file path landing in the OTHER
		// candidate's own directory name — the dangerous case: cutting
		// at the id's last "." reads pkgPath as ending in "/b", which
		// suffix-matches internal/b even though this case ran in a.
		{ID: "example.com/m/internal/a.TestParse/internal/b.go", Name: "TestParse/internal/b.go", Status: "pass"},
	}

	joined, stats := JoinCasesToSymbols(cases, "unit", symbols)
	if stats.Joined != 2 {
		t.Fatalf("Joined = %d, want 2; joined=%+v", stats.Joined, joined)
	}
	wantA := core.SymbolNode{Service: "svc", Package: "a", Name: "TestParse", FilePath: "internal/a/a_test.go"}.Key()
	if joined[0].TestSymbolKey != wantA {
		t.Errorf("dotted-subtest case joined to %q, want %q (internal/a)", joined[0].TestSymbolKey, wantA)
	}
	if joined[1].TestSymbolKey != wantA {
		t.Errorf("file-path-subtest case joined to %q, want %q (internal/a, NOT internal/b)", joined[1].TestSymbolKey, wantA)
	}
}

// TestJoinCasesToSymbols_RootPackageCandidateNeverMisjoinsToASubdirectory
// pins that a case sharing a bare Test* name with BOTH the repository-
// root package (a candidate with no directory at all) and a subdirectory
// package named like the module's own last path segment never gets
// silently misjoined to the subdirectory just because its import path
// happens to end in "/"+thatDirectory: this file has no go.mod, so it
// cannot tell a genuine root-package id from a genuine subdirectory id
// sharing the same trailing segment — the id "example.com/tools.TestA"
// is exactly as consistent with "root package, module example.com/
// tools" as it is with "tools/ subdirectory, module example.com". The
// PRE-FIX behaviour resolved this straight to the subdirectory candidate
// every time (a real defect: a root-package test run credited to the
// wrong symbol). The fix must never do that; staying unjoined here is
// correct and safe, exactly like any other unresolved ambiguity design
// doc §2 calls for.
func TestJoinCasesToSymbols_RootPackageCandidateNeverMisjoinsToASubdirectory(t *testing.T) {
	symbols := []core.SymbolNode{
		{Service: "svc", Package: "tools", Name: "TestA", Kind: core.KindGoFunc, FilePath: "a_test.go", Line: 1}, // root package
		{Service: "svc", Package: "tools", Name: "TestA", Kind: core.KindGoFunc, FilePath: "tools/a_test.go", Line: 1},
	}
	cases := []Case{
		{ID: "example.com/tools.TestA", Name: "TestA", Status: "fail"},
	}

	joined, stats := JoinCasesToSymbols(cases, "unit", symbols)
	if stats.Joined != 0 {
		t.Fatalf("Joined = %d, want 0 (an irreducible root/subdirectory ambiguity must stay unjoined)", stats.Joined)
	}
	wantSub := core.SymbolNode{Service: "svc", Package: "tools", Name: "TestA", FilePath: "tools/a_test.go"}.Key()
	if joined[0].TestSymbolKey == wantSub {
		t.Errorf("root-ambiguous case joined to the SUBDIRECTORY candidate %q — the dangerous misjoin this fix exists to remove", wantSub)
	}
	if joined[0].TestSymbolKey != "" {
		t.Errorf("root-ambiguous case joined to %q, want unjoined (\"\")", joined[0].TestSymbolKey)
	}
}

// TestJoinCasesToSymbols_UnambiguousSubdirectorySuffixStillJoins pins
// that an ordinary subdirectory suffix match — no root candidate sharing
// the bare name at all — still joins exactly as CV-72 always intended;
// the root-collision fix above narrows nothing about this common case.
func TestJoinCasesToSymbols_UnambiguousSubdirectorySuffixStillJoins(t *testing.T) {
	symbols := []core.SymbolNode{
		{Service: "svc", Package: "a", Name: "TestA", Kind: core.KindGoFunc, FilePath: "internal/a/a_test.go", Line: 1},
		{Service: "svc", Package: "b", Name: "TestA", Kind: core.KindGoFunc, FilePath: "internal/b/a_test.go", Line: 1},
	}
	cases := []Case{
		{ID: "example.com/m/internal/a.TestA", Name: "TestA", Status: "pass"},
	}
	joined, stats := JoinCasesToSymbols(cases, "unit", symbols)
	if stats.Joined != 1 {
		t.Fatalf("Joined = %d, want 1; joined=%+v", stats.Joined, joined)
	}
	want := core.SymbolNode{Service: "svc", Package: "a", Name: "TestA", FilePath: "internal/a/a_test.go"}.Key()
	if joined[0].TestSymbolKey != want {
		t.Errorf("joined to %q, want %q", joined[0].TestSymbolKey, want)
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

// TestJoinCasesToSymbolsNonGoTestEntries (TQ-28): the one join reaches the
// non-Go test entries repofacts.IsTestEntry names, by file and name. A
// case that matches no entry is counted and named, never dropped, and a
// Go-only symbol set joins exactly as before.
func TestJoinCasesToSymbolsNonGoTestEntries(t *testing.T) {
	symbols := []core.SymbolNode{
		{Service: "svc", Package: "src", Name: `"calc" "adds two numbers"`, Kind: "JSTest", FilePath: "src/calc.test.ts", Line: 3, Signature: `it("adds two numbers")`},
		{Service: "svc", Package: "src", Name: `"top level"`, Kind: "JSTest", FilePath: "src/calc.test.ts", Line: 7, Signature: `test("top level")`},
		{Service: "svc", Package: "src", Name: `"top level"`, Kind: "JSTest", FilePath: "src/other.test.ts", Line: 2, Signature: `test("top level")`},
		{Service: "svc", Package: "src", Name: "add", Kind: "JSFunc", FilePath: "src/calc.ts", Line: 1},
		{Service: "svc", Package: "tests", Name: "test_add", Kind: "PythonFunc", FilePath: "tests/test_calc.py", Line: 2, Signature: "def test_add()"},
		{Service: "svc", Package: "tests", Name: "TestCalc.test_zero", Kind: "PythonFunc", FilePath: "tests/test_calc.py", Line: 5, Signature: "def test_zero(self)"},
		{Service: "svc", Package: "tests", Name: "test_add", Kind: "PythonFunc", FilePath: "tests/test_other.py", Line: 2, Signature: "def test_add()"},
		{Service: "svc", Package: "com.acme", Name: "FooTest.testBar", Kind: "JavaMethod", FilePath: "src/test/java/com/acme/FooTest.java", Line: 9, Signature: "@Test public void testBar()"},
	}
	key := func(i int) string { return symbols[i].Key() }

	cases := []Case{
		{ID: "src/calc.test.ts > calc > adds two numbers", Name: "calc adds two numbers", Status: "pass"},
		{ID: "src/calc.test.ts::top level", Name: "top level", Status: "pass"},
		{ID: "tests/test_calc.py::test_add", Name: "test_add", Status: "pass"},
		{ID: "tests/test_calc.py::TestCalc::test_zero", Name: "TestCalc::test_zero", Status: "fail"},
		{ID: "tests/test_calc.py::test_add[1-2]", Name: "test_add[1-2]", Status: "pass"},
		{ID: "com.acme.FooTest.testBar", Name: "testBar", Status: "pass"},
		{ID: "src/calc.test.ts::never declared", Name: "never declared", Status: "skip"},
		{ID: "test_add", Name: "test_add", Status: "pass"}, // no file: ambiguous, never guessed
	}
	joined, stats := JoinCasesToSymbols(cases, "unit", symbols)

	want := []string{key(0), key(1), key(4), key(5), key(4), key(7), "", ""}
	for i, w := range want {
		if joined[i].TestSymbolKey != w {
			t.Errorf("case %q joined to %q, want %q", cases[i].ID, joined[i].TestSymbolKey, w)
		}
	}
	if stats.Total != 8 || stats.Joined != 6 {
		t.Fatalf("stats = %+v, want Total 8 Joined 6", stats)
	}
	wantUn := []string{"src/calc.test.ts::never declared", "test_add"}
	if len(stats.Unmatched) != 2 || stats.Unmatched[0] != wantUn[0] || stats.Unmatched[1] != wantUn[1] {
		t.Fatalf("Unmatched = %v, want %v", stats.Unmatched, wantUn)
	}

	// Go-only symbols and cases: the join is what it was.
	goSyms := []core.SymbolNode{{Service: "svc", Package: "cart", Name: "TestAdd", Kind: core.KindGoFunc, FilePath: "cart/cart_test.go"}}
	gj, gs := JoinCasesToSymbols([]Case{{ID: "TestAdd", Name: "TestAdd/sub"}, {ID: "TestNope", Name: "TestNope"}}, "unit", goSyms)
	if gs.Joined != 1 || gj[0].TestSymbolKey != goSyms[0].Key() || gj[1].TestSymbolKey != "" {
		t.Fatalf("go join changed: %+v %+v", gj, gs)
	}
}
