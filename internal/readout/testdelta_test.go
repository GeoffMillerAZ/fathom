package readout

import "testing"

func TestGoTestFilterMatchFollowsGoTestRules(t *testing.T) {
	for _, c := range []struct {
		pattern, name string
		skip          bool
		match, ok     bool
	}{
		{"TestA", "TestA", false, true, true},
		{"TestA", "TestAB", false, true, true}, // unanchored, as go test matches
		{"^TestA$", "TestAB", false, false, true},
		{"TestB|TestA", "TestA", false, true, true},
		{"TestA/one", "TestA/two", false, false, true},
		{"TestA/one", "TestA", false, true, true}, // the parent runs to reach its subtests
		{"TestA/one", "TestA", true, false, true}, // a -skip needs every level
		{"TestA", "TestA/one", true, true, true},
		{"Test[|]A", "Test|A", false, true, true}, // a bar in brackets is no alternative
		{"Test(", "TestA", false, false, false},
	} {
		m, ok := goTestFilterMatch(c.pattern, c.name, c.skip)
		if m != c.match || ok != c.ok {
			t.Errorf("goTestFilterMatch(%q, %q, skip=%v) = %v, %v; want %v, %v", c.pattern, c.name, c.skip, m, ok, c.match, c.ok)
		}
	}
	if run, skip := scopeFilters("./a/,./b/ run=Test A skip=Slow"); run != "Test A" || skip != "Slow" {
		t.Errorf("scopeFilters = %q, %q", run, skip)
	}
}

func TestRenderTestDeltaDoubtNeverReadsFixed(t *testing.T) {
	const pkg = "example.com/m/money"
	row := func(head, tree string, failing ...string) TestRunRow {
		return TestRunRow{Invocation: "go test", Result: "fail", Failing: failing, Head: head, Tree: tree, Scope: "./..."}
	}
	now := func(status string, complete, filters, failfast, panicked bool, failing ...string) TestRunNow {
		return TestRunNow{
			Invocation: "go test", Failing: failing, Packages: []TestRunPackage{{Name: pkg, Status: status, Panicked: panicked}},
			Complete: complete, FiltersKnown: filters, FailFast: failfast, Head: "h2", Tree: "t2", Scope: "./...",
		}
	}
	for _, c := range []struct {
		name    string
		prev    []TestRunRow
		now     TestRunNow
		want    string
		outcome string
	}{
		{"a build-failed package ran nothing", []TestRunRow{row("h1", "t1", pkg+".TestA")}, now("build-failed", true, true, false, false),
			"delta vs the last run: not run money.TestA", DeltaCompared},
		{"an unread filter is doubt", []TestRunRow{row("h1", "t1", pkg+".TestA", pkg+".TestB")}, now("fail", true, false, false, false, pkg+".TestB"),
			"delta vs the last run: still failing money.TestB (since h1); 1 not compared", DeltaCompared},
		{"-failfast in a failing package is doubt", []TestRunRow{row("h1", "t1", pkg+".TestA", pkg+".TestB")}, now("fail", true, true, true, false, pkg+".TestB"),
			"delta vs the last run: still failing money.TestB (since h1); 1 not compared", DeltaCompared},
		{"a panic in the package is doubt", []TestRunRow{row("h1", "t1", pkg+".TestA", pkg+".TestB")}, now("fail", true, true, false, true, pkg+".TestB"),
			"delta vs the last run: still failing money.TestB (since h1); 1 not compared", DeltaCompared},
		{"a piped output never says not run", []TestRunRow{row("h1", "t1", "example.com/m/cart.TestC")}, now("fail", false, true, false, false),
			"", DeltaSilent},
		{"a capped earlier row leaves the streak open", []TestRunRow{
			{Invocation: "go test", Result: "fail", Failing: []string{"x"}, Capped: true, Head: "h0", Tree: "t0", Scope: "./..."},
			row("h1", "t1", pkg+".TestB"),
		}, now("fail", true, true, false, false, pkg+".TestB"),
			"delta vs the last run: still failing money.TestB (since h1 or earlier)", DeltaCompared},
		{"this run missing its tree is not compared", []TestRunRow{row("h1", "t1", pkg+".TestA")}, TestRunNow{Invocation: "go test", Head: "h2", Scope: "./..."},
			"", DeltaNotCompared},
	} {
		got, outcome := renderTestDelta(c.prev, c.now)
		if got != c.want || outcome != c.outcome {
			t.Errorf("%s:\n got %q (%s)\nwant %q (%s)", c.name, got, outcome, c.want, c.outcome)
		}
	}
}
