package memo

import (
	"context"
	"sort"
	"testing"
)

// TestGroup_OnEvictReleasesValues (LB-F15): a group whose values hold
// resources outside the Go heap (compiled grammar code) is told of every
// value leaving the cache — budget and shed trims, the refuse drop, Reset,
// Remove, a replacement — exactly once each, so it can release them.
func TestGroup_OnEvictReleasesValues(t *testing.T) {
	reg := NewRegistry()
	g := NewGroupIn[string, string](reg, "onevict", Options{})
	var released []string
	g.SetOnEvict(func(k, v string) { released = append(released, k+"="+v) })
	ctx := context.Background()
	put := func(k, v string, size int64) {
		t.Helper()
		if _, err := g.Get(ctx, k, func(context.Context) (string, int64, error) { return v, size, nil }); err != nil {
			t.Fatal(err)
		}
	}
	want := func(w ...string) {
		t.Helper()
		sort.Strings(released)
		sort.Strings(w)
		if len(released) != len(w) {
			t.Fatalf("released %v, want %v", released, w)
		}
		for i := range w {
			if released[i] != w[i] {
				t.Fatalf("released %v, want %v", released, w)
			}
		}
		released = nil
	}

	put("a", "1", 10)
	put("b", "2", 10)
	reg.SetBudget(15, 0) // over budget: the LRU entry goes
	want("a=1")
	reg.SetBudget(0, 0)
	put("c", "3", 10)
	g.Remove("b", ReasonWatermark)
	want("b=2")
	put("d", "4", 10)
	reg.Drop(ReasonRefuse)
	want("c=3", "d=4")
	put("e", "5", 10)
	g.Reset(ReasonIdle)
	want("e=5")
	if g.Len() != 0 {
		t.Fatal("entries survived Reset")
	}
}
