package cost

import (
	"math"
	"sync"
	"testing"
)

// ---------------------------------------------------------------------------
// Estimate
// ---------------------------------------------------------------------------

func TestEstimate(t *testing.T) {
	cases := []struct {
		text string
		want int
	}{
		{"", 0},
		{"a", 1},
		{"abcd", 1},
		{"abcde", 2},
		{"hello world", 3},    // 11 chars → ceil(11/4)=3
		{"1234567890", 3},     // 10 chars → ceil(10/4)=3
		{"12345678901234", 4}, // 14 chars → ceil(14/4)=4
	}
	for _, tc := range cases {
		got := Estimate(tc.text)
		if got != tc.want {
			t.Errorf("Estimate(%q) = %d, want %d", tc.text, got, tc.want)
		}
	}
}

// ---------------------------------------------------------------------------
// PriceUSD
// ---------------------------------------------------------------------------

func TestPriceUSD_ZeroRate(t *testing.T) {
	if p := PriceUSD(1000, 0); p != 0 {
		t.Fatalf("want 0 got %v", p)
	}
}

func TestPriceUSD_Basic(t *testing.T) {
	// 1000 tokens at $1.00/kTok = $1.00
	got := PriceUSD(1000, 1.0)
	if math.Abs(got-1.0) > 1e-6 {
		t.Fatalf("want 1.0 got %v", got)
	}
}

func TestPriceUSD_Fractional(t *testing.T) {
	// 500 tokens at $3.00/kTok = $1.50
	got := PriceUSD(500, 3.0)
	if math.Abs(got-1.5) > 1e-6 {
		t.Fatalf("want 1.5 got %v", got)
	}
}

// ---------------------------------------------------------------------------
// Tracker
// ---------------------------------------------------------------------------

func TestTracker_EmptyReport(t *testing.T) {
	tk := NewTracker()
	r := tk.Report()
	if len(r) != 0 {
		t.Fatalf("expected empty report, got %v", r)
	}
	tokens, usd := tk.Totals()
	if tokens != 0 || usd != 0 {
		t.Fatalf("expected zero totals, got tokens=%d usd=%v", tokens, usd)
	}
}

func TestTracker_RecordAndReport(t *testing.T) {
	tk := NewTracker()
	tk.Record("search", 100, 0.01)
	tk.Record("generate", 500, 0.05)
	tk.Record("search", 200, 0.02) // accumulate

	r := tk.Report()
	if len(r) != 2 {
		t.Fatalf("want 2 entries, got %d", len(r))
	}
	// sorted by name: generate, search
	if r[0].Tool != "generate" || r[1].Tool != "search" {
		t.Fatalf("unexpected order: %v", r)
	}
	if r[1].Tokens != 300 {
		t.Fatalf("search tokens: want 300 got %d", r[1].Tokens)
	}
	if math.Abs(r[1].USD-0.03) > 1e-9 {
		t.Fatalf("search USD: want 0.03 got %v", r[1].USD)
	}
}

func TestTracker_Totals(t *testing.T) {
	tk := NewTracker()
	tk.Record("a", 100, 0.10)
	tk.Record("b", 200, 0.20)
	tokens, usd := tk.Totals()
	if tokens != 300 {
		t.Fatalf("want 300 tokens got %d", tokens)
	}
	if math.Abs(usd-0.30) > 1e-9 {
		t.Fatalf("want 0.30 USD got %v", usd)
	}
}

func TestTracker_Concurrent(t *testing.T) {
	tk := NewTracker()
	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			tool := "tool"
			if n%2 == 0 {
				tool = "other"
			}
			tk.Record(tool, 10, 0.001)
		}(i)
	}
	wg.Wait()

	tokens, _ := tk.Totals()
	if tokens != 1000 {
		t.Fatalf("want 1000 total tokens got %d", tokens)
	}
	r := tk.Report()
	if len(r) != 2 {
		t.Fatalf("want 2 tools got %d", len(r))
	}
}

func TestReport_IsSorted(t *testing.T) {
	tk := NewTracker()
	tools := []string{"zebra", "alpha", "mango", "banana"}
	for _, tool := range tools {
		tk.Record(tool, 1, 0.001)
	}
	r := tk.Report()
	for i := 1; i < len(r); i++ {
		if r[i].Tool < r[i-1].Tool {
			t.Fatalf("report not sorted at index %d: %s > %s", i, r[i-1].Tool, r[i].Tool)
		}
	}
}
