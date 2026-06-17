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
