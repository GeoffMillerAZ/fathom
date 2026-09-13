package mcp

import "testing"

type budItem struct{ S string }

func TestFitToBudget(t *testing.T) {
	items := []budItem{{"aaaaaaaa"}, {"bbbbbbbb"}, {"cccccccc"}, {"dddddddd"}, {"eeeeeeee"}}

	// A small budget keeps a nonzero prefix and reports truncation.
	got, trunc := fitToBudget(items, 40)
	if !trunc {
		t.Fatal("expected truncation at a small budget")
	}
	if len(got) == 0 || len(got) >= len(items) {
		t.Fatalf("got %d items, want a nonzero prefix < %d", len(got), len(items))
	}

	// A large budget keeps everything, no truncation.
	all, t2 := fitToBudget(items, 100000)
	if t2 || len(all) != len(items) {
		t.Fatalf("large budget should keep all: got %d trunc=%v", len(all), t2)
	}

	// Always keep at least the first item, even when it alone exceeds the budget.
	one, t3 := fitToBudget(items, 1)
	if len(one) != 1 || !t3 {
		t.Fatalf("tiny budget should keep 1 + truncate: got %d trunc=%v", len(one), t3)
	}

	// Empty input → empty output, no truncation, no panic.
	if e, t4 := fitToBudget([]budItem(nil), 0); len(e) != 0 || t4 {
		t.Fatalf("empty -> empty/no-trunc, got %d trunc=%v", len(e), t4)
	}
}

// ─── RC-19: fitToolBudget/elisionLine ───────────────────────────────────────

func TestFitToolBudget(t *testing.T) {
	items := []budItem{{"aaaaaaaa"}, {"bbbbbbbb"}, {"cccccccc"}, {"dddddddd"}, {"eeeeeeee"}}

	// A small budget keeps a nonzero prefix and reports how many were cut.
	got, n := fitToolBudget(items, 40)
	if n <= 0 {
		t.Fatal("expected a nonzero elided count at a small budget")
	}
	if len(got)+n != len(items) {
		t.Fatalf("kept(%d) + elided(%d) = %d, want %d", len(got), n, len(got)+n, len(items))
	}

	// A large budget keeps everything: elided count is 0.
	all, n2 := fitToolBudget(items, 100000)
	if n2 != 0 || len(all) != len(items) {
		t.Fatalf("large budget should keep all: got %d elided=%d", len(all), n2)
	}

	// Determinism (BA-3, RC-19): same input + budget cuts the same tail
	// every time.
	got2, n3 := fitToolBudget(items, 40)
	if len(got2) != len(got) || n3 != n {
		t.Fatalf("fitToolBudget is non-deterministic: (%d,%d) vs (%d,%d)", len(got), n, len(got2), n3)
	}
}

func TestElisionLine(t *testing.T) {
	if got := elisionLine(0, "package or file"); got != "" {
		t.Errorf("elisionLine(0, ...) = %q, want empty", got)
	}
	if got := elisionLine(-1, "package or file"); got != "" {
		t.Errorf("elisionLine(-1, ...) = %q, want empty", got)
	}
	got := elisionLine(7, "package or file")
	want := "elided: 7 more — refine with package or file"
	if got != want {
		t.Errorf("elisionLine(7, ...) = %q, want %q", got, want)
	}
}
