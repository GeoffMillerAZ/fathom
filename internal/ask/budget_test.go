package ask

import (
	"reflect"
	"testing"
)

// TestBudgetStopsBeforeTheChunkThatWouldPassTheCap is EXP-1's stop rule: a
// chunk whose estimate would take the spend past the cap is never sent,
// and nothing after it is either.
func TestBudgetStopsBeforeTheChunkThatWouldPassTheCap(t *testing.T) {
	b := &Budget{CapUSD: 2, Price: Price{InPerMTok: 1}}
	if !b.Admit(1_000_000, 0) {
		t.Fatal("first chunk ($1 of $2) refused")
	}
	if c := b.Charge(0, 0, 1_000_000, 0); c != 1 || b.Estimated != 1 || b.Reported != 0 {
		t.Fatalf("no reported usage: charged %v, basis %+v", c, b)
	}
	if !b.Admit(1_000_000, 0) {
		t.Fatal("a chunk landing exactly on the cap refused")
	}
	if c := b.Charge(500_000, 10, 1_000_000, 0); c != 0.5 || b.Reported != 1 {
		t.Fatalf("reported usage: charged %v, basis %+v", c, b)
	}
	if b.Admit(600_000, 0) {
		t.Fatal("a chunk passing the cap ($1.5 + $0.6 > $2) admitted")
	}
	if !b.Stopped || b.Admit(1, 0) {
		t.Fatal("after a refusal every later chunk must be refused")
	}
	if b.Calls != 2 || b.SpentUSD != 1.5 {
		t.Errorf("tally = %+v", b)
	}
}

func TestListedPriceAndEstimate(t *testing.T) {
	b := &Budget{CapUSD: DefaultCapUSD, Price: ListedPrice}
	if got := b.Cost(1_000_000, 1_000_000); got != 0.042 {
		t.Errorf("listed price for 1M in + 1M out = %v, want 0.042 (output free)", got)
	}
	if EstimateTokens(0) != 0 || EstimateTokens(1) != 1 || EstimateTokens(3) != 1 || EstimateTokens(4) != 2 {
		t.Error("EstimateTokens must be ceil(bytes/3)")
	}
}

func TestChunksBoundQuestionsAndBytes(t *testing.T) {
	cases := []struct {
		sizes            []int
		state, maxQ, max int
		want             [][]int
	}{
		{[]int{10, 10, 10}, 5, 2, 100, [][]int{{0, 1}, {2}}},
		{[]int{60, 50, 10}, 5, 64, 100, [][]int{{0}, {1, 2}}},
		{[]int{200}, 5, 64, 100, [][]int{{0}}},
		{nil, 5, 64, 100, nil},
	}
	for _, c := range cases {
		if got := Chunks(c.sizes, c.state, c.maxQ, c.max); !reflect.DeepEqual(got, c.want) {
			t.Errorf("Chunks(%v, %d, %d, %d) = %v, want %v", c.sizes, c.state, c.maxQ, c.max, got, c.want)
		}
	}
}
