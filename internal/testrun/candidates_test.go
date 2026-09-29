package testrun

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"testing"
)

// fakeIndex is the Index port over fixed answers, recording what it was
// asked.
type fakeIndex struct {
	impact  map[string][]string
	located []Located
	failRef string
	asked   []string
	seeds   []LocalizeSeed
}

func (f *fakeIndex) TestImpact(ref string) ([]string, error) {
	f.asked = append(f.asked, ref)
	if ref == f.failRef {
		return nil, errors.New("daemon refused")
	}
	return f.impact[ref], nil
}

func (f *fakeIndex) Localize(seed LocalizeSeed) ([]Located, error) {
	f.seeds = append(f.seeds, seed)
	return f.located, nil
}

func writeFile(t *testing.T, root, rel, content string) {
	t.Helper()
	p := filepath.Join(root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

// TestTestRunCandidatesNameChangedSymbolsFailingTestsReach pins TR-5
// (design section 3, tier 1): the working-tree diff names the changed Go
// functions and methods; test_impact over each, intersected with the
// failing tests, names the changed symbols each failing test reaches; and
// localize runs once, from the panic's stack trace when there is one, else
// from the failing test, with its hops capped at 4.
func TestTestRunCandidatesNameChangedSymbolsFailingTestsReach(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "money/money.go", `package money

// Round rounds cents to the nearest ten.
func Round(c int) int {
	return (c / 10) * 10
}

func Unchanged() int { return 1 }

type Ledger struct{ n int }

func (l *Ledger) Add(c int) {
	l.n += Round(c)
}
`)
	writeFile(t, root, "cart/cart.go", `package cart

func Total(items []int) int { return len(items) }
`)
	writeFile(t, root, "money/money_test.go", "package money\n\nfunc TestRound() {}\n")
	diff := `diff --git a/money/money.go b/money/money.go
index 1111111..2222222 100644
--- a/money/money.go
+++ b/money/money.go
@@ -5 +5 @@ func Round(c int) int {
-	return ((c + 5) / 10) * 10
+	return (c / 10) * 10
@@ -12,0 +13 @@ func (l *Ledger) Add(c int) {
+	l.n += Round(c)
diff --git a/cart/cart.go b/cart/cart.go
--- a/cart/cart.go
+++ b/cart/cart.go
@@ -3 +3 @@
-func Total(items []int) int { return 0 }
+func Total(items []int) int { return len(items) }
diff --git a/money/money_test.go b/money/money_test.go
--- a/money/money_test.go
+++ b/money/money_test.go
@@ -3 +3 @@
-func TestRound() { }
+func TestRound() {}
diff --git a/gone.go b/gone.go
deleted file mode 100644
--- a/gone.go
+++ /dev/null
@@ -1,3 +0,0 @@
-package gone
`
	changed := ChangedGoSymbols(root, diff)
	var names []string
	for _, c := range changed {
		names = append(names, c.Name)
	}
	if want := []string{"cart.Total", "money.Round", "Ledger.Add"}; !reflect.DeepEqual(names, want) {
		t.Fatalf("changed = %q, want %q (test files and untouched functions excluded; file, then line order)", names, want)
	}
	if c := changed[1]; c.Ref != "money/money.Round" || c.File != "money/money.go" || c.Line != 4 {
		t.Errorf("money.Round = %+v", c)
	}
	if c := changed[2]; c.Ref != "money/Ledger.Ledger.Add" {
		t.Errorf("Ledger.Add ref = %q, want the index's method key shape", c.Ref)
	}

	run := ParseTestRun(fixture(t, "go-plain-fail.txt"))
	idx := &fakeIndex{
		impact: map[string][]string{
			"money/money.Round": {
				"shop::money/money.TestRound", "shop::money/money.TestRoundZero",
				"shop::cart/cart.TestTotal", "shop::other/money.TestTable",
			},
			"money/Ledger.Ledger.Add": {"shop::money/money.TestLedger"},
		},
		failRef: "cart/cart.Total",
		located: []Located{
			{Symbol: "shop::money/money.Unchanged", File: "money/money.go"},
			{Symbol: "shop::money/money.Round", File: "money/money.go"},
		},
	}
	got := testRunCandidates(run, changed, idx)

	reach := map[string][]string{}
	for _, r := range got.Reach {
		for _, c := range r.Changed {
			reach[r.Test] = append(reach[r.Test], c.Name)
		}
	}
	want := map[string][]string{
		"example.com/shop/cart.TestTotal":  {"money.Round"},
		"example.com/shop/money.TestRound": {"money.Round"},
	}
	if !reflect.DeepEqual(reach, want) {
		t.Errorf("reach = %v\nwant %v (TestTable/half's key is in another package; TestPanics reaches nothing changed)", reach, want)
	}
	if len(got.Reach) != 4 {
		t.Errorf("reach rows = %d, want one per leaf failure (4)", len(got.Reach))
	}
	sort.Strings(idx.asked)
	if !reflect.DeepEqual(idx.asked, []string{"cart/cart.Total", "money/Ledger.Ledger.Add", "money/money.Round"}) {
		t.Errorf("test_impact asked about %q", idx.asked)
	}
	if len(idx.seeds) != 1 || idx.seeds[0].Budget != LocalizeHops || LocalizeHops != 4 {
		t.Fatalf("localize seeds = %+v, want one call with 4 hops", idx.seeds)
	}
	if s := idx.seeds[0]; s.Kind != "stack_trace" || s.Value == "" {
		t.Errorf("a run with a panic localizes from its stack trace, got %+v", s)
	}
	if len(got.Localized) != 2 || got.Localized[0].Symbol != "shop::money/money.Round" || !got.Localized[0].Changed {
		t.Errorf("localized = %+v, want the changed candidate first", got.Localized)
	}

	t.Run("no panic: localize from the failing test", func(t *testing.T) {
		idx := &fakeIndex{}
		testRunCandidates(ParseTestRun(fixture(t, "junit-fail.xml")), nil, idx)
		if len(idx.seeds) != 1 || idx.seeds[0].Kind != "failing_test" || idx.seeds[0].Value != "TestMain" || idx.seeds[0].Budget != 4 {
			t.Errorf("seeds = %+v", idx.seeds)
		}
		if len(idx.asked) != 0 {
			t.Errorf("no changed symbols, yet test_impact was asked %q", idx.asked)
		}
	})

	t.Run("a passing run asks nothing", func(t *testing.T) {
		idx := &fakeIndex{}
		got := testRunCandidates(ParseTestRun(fixture(t, "go-plain-pass.txt")), changed, idx)
		if len(idx.asked)+len(idx.seeds) != 0 || len(got.Reach) != 0 {
			t.Errorf("pass: asked %q seeds %+v reach %+v", idx.asked, idx.seeds, got.Reach)
		}
	})

	t.Run("the cap", func(t *testing.T) {
		var many []ChangedSymbol
		for i := 0; i < MaxChangedQueried+5; i++ {
			many = append(many, ChangedSymbol{Ref: string(rune('a'+i)) + "/p.F", Name: "p.F"})
		}
		idx := &fakeIndex{}
		got := testRunCandidates(run, many, idx)
		if len(idx.asked) != MaxChangedQueried || got.Skipped != 5 {
			t.Errorf("asked %d, skipped %d; want %d and 5", len(idx.asked), got.Skipped, MaxChangedQueried)
		}
	})
}
