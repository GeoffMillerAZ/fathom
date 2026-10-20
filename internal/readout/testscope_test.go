package readout

import "testing"

func TestScopeAdmitsReadsPatternsAndFilters(t *testing.T) {
	cases := []struct {
		scope, dir, name string
		admits, ok       bool
	}{
		{"./...", "money", "TestRound", true, true},
		{"./...", ".", "TestRoot", true, true},
		{"./money/", "money", "TestRound", true, true},
		{"./money", "money/sub", "TestRound", false, true},
		{"./money/...", "money/sub", "TestRound", true, true},
		{"./cart/,./money/", "money", "TestRound", true, true},
		{"./cart/", "money", "TestRound", false, true},
		{"example.com/shop/money", "money", "TestRound", true, true},
		{"example.com/shop/...", "money", "TestRound", false, true},
		{"../shop/money", "money", "TestRound", false, true},
		{"run=TestRound", ".", "TestRound", true, true},
		{"./money/ run=TestRound skip=Slow", "money", "TestRound", true, true},
		{"./money/ run=TestFloor", "money", "TestRound", false, true},
		{"./money/ skip=Round", "money", "TestRound", false, true},
		{"./money/ run=(", "money", "TestRound", false, false},
		{"", "money", "TestRound", false, false},
	}
	for _, c := range cases {
		admits, ok := ScopeAdmits(c.scope, c.dir, c.name)
		if admits != c.admits || ok != c.ok {
			t.Errorf("ScopeAdmits(%q, %q, %q) = %v, %v; want %v, %v", c.scope, c.dir, c.name, admits, ok, c.admits, c.ok)
		}
	}
}
