package costtracker_test

import (
	"fmt"
	"math"
	"strings"
	"sync"
	"testing"

	costtracker "github.com/geoffmilleraz/agentic-kit/foundations/cost/cost-tracker"
	"github.com/geoffmilleraz/agentic-kit/foundations/llmclient"
)

// floatEq compares floats with a tolerance suitable for USD price math.
func floatEq(a, b float64) bool {
	return math.Abs(a-b) < 1e-9
}

// ---- EstimateTokens -----------------------------------------------------------

func TestEstimateTokens(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want int
	}{
		{name: "empty", in: "", want: 0},
		{name: "one char rounds up", in: "a", want: 1},
		{name: "exactly four chars", in: "abcd", want: 1},
		{name: "five chars rounds up", in: "abcde", want: 2},
		{name: "four hundred chars", in: strings.Repeat("a", 400), want: 100},
		{name: "runes not bytes", in: "héllo wörld!", want: 3},    // 12 runes, 14 bytes
		{name: "cjk runes counted once", in: "日本語のテキスト", want: 2}, // 8 runes, 24 bytes
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := costtracker.EstimateTokens(tt.in); got != tt.want {
				t.Errorf("EstimateTokens(%q) = %d, want %d", tt.in, got, tt.want)
			}
		})
	}
}

func TestEstimatorOrDefault(t *testing.T) {
	t.Run("nil falls back to EstimateTokens", func(t *testing.T) {
		est := costtracker.EstimatorOrDefault(nil)
		if got, want := est("abcdefgh"), costtracker.EstimateTokens("abcdefgh"); got != want {
			t.Errorf("default estimator = %d, want %d", got, want)
		}
	})
	t.Run("custom estimator is used verbatim", func(t *testing.T) {
		est := costtracker.EstimatorOrDefault(func(s string) int { return len(s) * 10 })
		if got := est("ab"); got != 20 {
			t.Errorf("custom estimator = %d, want 20", got)
		}
	})
}

// ---- PriceTable.Cost ------------------------------------------------------------

// exampleTable holds EXAMPLE prices for tests only — not real provider rates.
func exampleTable() costtracker.PriceTable {
	return costtracker.PriceTable{
		"example-model": {
			InputPerMTok:      1.0,
			OutputPerMTok:     5.0,
			CacheReadPerMTok:  0.1,
			CacheWritePerMTok: 1.25,
		},
	}
}

func TestPriceTable_Cost(t *testing.T) {
	tests := []struct {
		name   string
		model  string
		usage  llmclient.Usage
		want   float64
		wantOK bool
	}{
		{
			name:  "all four token classes priced (golden)",
			model: "example-model",
			usage: llmclient.Usage{
				InputTokens:              1_000_000,
				OutputTokens:             2_000_000,
				CacheReadInputTokens:     500_000,
				CacheCreationInputTokens: 100_000,
			},
			// 1.0 + 10.0 + 0.05 + 0.125
			want:   11.175,
			wantOK: true,
		},
		{
			name:  "cache reads priced only at the cache-read rate, never as plain input",
			model: "example-model",
			usage: llmclient.Usage{CacheReadInputTokens: 1_000_000},
			want:  0.1,
			// InputTokens excludes cache reads (Anthropic semantics), so a
			// pure cache hit costs only CacheReadPerMTok.
			wantOK: true,
		},
		{
			name:   "zero usage costs zero",
			model:  "example-model",
			usage:  llmclient.Usage{},
			want:   0,
			wantOK: true,
		},
		{
			name:   "unknown model",
			model:  "mystery-model",
			usage:  llmclient.Usage{InputTokens: 1000},
			want:   0,
			wantOK: false,
		},
		{
			name:   "empty model name absent from table",
			model:  "",
			usage:  llmclient.Usage{InputTokens: 1000},
			want:   0,
			wantOK: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := exampleTable().Cost(tt.model, tt.usage)
			if ok != tt.wantOK {
				t.Fatalf("Cost ok = %v, want %v", ok, tt.wantOK)
			}
			if !floatEq(got, tt.want) {
				t.Errorf("Cost = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestPriceTable_Cost_EmptyTable(t *testing.T) {
	got, ok := costtracker.PriceTable{}.Cost("anything", llmclient.Usage{InputTokens: 5})
	if ok || got != 0 {
		t.Errorf("empty table Cost = (%v, %v), want (0, false)", got, ok)
	}
}

// ---- Tracker --------------------------------------------------------------------

func TestTracker_RecordAndTotals(t *testing.T) {
	tr := costtracker.New()
	tr.Record("alpha", "model-b", llmclient.Usage{
		InputTokens:              100,
		OutputTokens:             20,
		CacheReadInputTokens:     5,
		CacheCreationInputTokens: 3,
	}, 0.5)
	tr.Record("alpha", "model-a", llmclient.Usage{InputTokens: 50, OutputTokens: 10}, 0.25)
	tr.Record("beta", "model-a", llmclient.Usage{InputTokens: 10, OutputTokens: 1}, 0.125)

	want := costtracker.Totals{
		Calls:            3,
		InputTokens:      160,
		OutputTokens:     31,
		CacheReadTokens:  5,
		CacheWriteTokens: 3,
		USD:              0.875,
	}
	if got := tr.Totals(); got != want {
		t.Errorf("Totals = %+v, want %+v", got, want)
	}

	byLabel := tr.ByLabel()
	if got, want := byLabel["alpha"].Calls, int64(2); got != want {
		t.Errorf(`ByLabel["alpha"].Calls = %d, want %d`, got, want)
	}
	if got := byLabel["beta"]; !floatEq(got.USD, 0.125) || got.InputTokens != 10 {
		t.Errorf(`ByLabel["beta"] = %+v, want usd=0.125 input=10`, got)
	}

	byModel := tr.ByModel()
	if got, want := byModel["model-a"].Calls, int64(2); got != want {
		t.Errorf(`ByModel["model-a"].Calls = %d, want %d`, got, want)
	}
	if got, want := byModel["model-b"].CacheWriteTokens, int64(3); got != want {
		t.Errorf(`ByModel["model-b"].CacheWriteTokens = %d, want %d`, got, want)
	}
}

func TestTracker_ByLabelReturnsIndependentCopy(t *testing.T) {
	tr := costtracker.New()
	tr.Record("alpha", "m", llmclient.Usage{InputTokens: 1}, 0)

	got := tr.ByLabel()
	got["alpha"] = costtracker.Totals{Calls: 999}
	got["injected"] = costtracker.Totals{Calls: 1}

	fresh := tr.ByLabel()
	if fresh["alpha"].Calls != 1 {
		t.Errorf("mutating the returned map leaked into the tracker: %+v", fresh["alpha"])
	}
	if _, ok := fresh["injected"]; ok {
		t.Error("inserting into the returned map leaked into the tracker")
	}
}

func TestTracker_ReportGolden(t *testing.T) {
	tr := costtracker.New()
	tr.Record("alpha", "model-b", llmclient.Usage{
		InputTokens:              100,
		OutputTokens:             20,
		CacheReadInputTokens:     5,
		CacheCreationInputTokens: 3,
	}, 0.5)
	tr.Record("alpha", "model-a", llmclient.Usage{InputTokens: 50, OutputTokens: 10}, 0.25)
	tr.Record("beta", "model-a", llmclient.Usage{InputTokens: 10, OutputTokens: 1}, 0.125)

	want := `cost report
by label:
  "alpha": calls=2 input=150 output=30 cache_read=5 cache_write=3 usd=0.750000
  "beta": calls=1 input=10 output=1 cache_read=0 cache_write=0 usd=0.125000
by model:
  "model-a": calls=2 input=60 output=11 cache_read=0 cache_write=0 usd=0.375000
  "model-b": calls=1 input=100 output=20 cache_read=5 cache_write=3 usd=0.500000
total: calls=3 input=160 output=31 cache_read=5 cache_write=3 usd=0.875000
`
	if got := tr.Report(); got != want {
		t.Errorf("Report mismatch:\n got:\n%s\nwant:\n%s", got, want)
	}

	// Deterministic: a second render is byte-identical.
	if again := tr.Report(); again != want {
		t.Error("Report is not deterministic across calls")
	}
}

func TestTracker_ReportEmptyGolden(t *testing.T) {
	want := `cost report
by label:
by model:
total: calls=0 input=0 output=0 cache_read=0 cache_write=0 usd=0.000000
`
	if got := costtracker.New().Report(); got != want {
		t.Errorf("empty Report mismatch:\n got:\n%s\nwant:\n%s", got, want)
	}
}

func TestTracker_EmptyKeysAreVisibleInReport(t *testing.T) {
	tr := costtracker.New()
	tr.Record("", "", llmclient.Usage{InputTokens: 7}, 0)
	report := tr.Report()
	if !strings.Contains(report, `  "": calls=1 input=7 output=0 cache_read=0 cache_write=0 usd=0.000000`) {
		t.Errorf("empty label/model keys must render as %q lines; got:\n%s", `""`, report)
	}
}

func TestTracker_ConcurrentRecord(t *testing.T) {
	tr := costtracker.New()
	const (
		goroutines = 8
		perG       = 200
	)
	var wg sync.WaitGroup
	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			label := fmt.Sprintf("label-%d", i%2)
			for j := 0; j < perG; j++ {
				tr.Record(label, "shared-model", llmclient.Usage{
					InputTokens:              1,
					OutputTokens:             2,
					CacheReadInputTokens:     3,
					CacheCreationInputTokens: 4,
				}, 0.0009765625) // 1/1024, exact in binary
			}
		}(i)
	}
	wg.Wait()

	const calls = goroutines * perG
	got := tr.Totals()
	want := costtracker.Totals{
		Calls:            calls,
		InputTokens:      calls,
		OutputTokens:     2 * calls,
		CacheReadTokens:  3 * calls,
		CacheWriteTokens: 4 * calls,
		USD:              calls * 0.0009765625,
	}
	if got != want {
		t.Errorf("concurrent Totals = %+v, want %+v", got, want)
	}
	if got, want := tr.ByModel()["shared-model"].Calls, int64(calls); got != want {
		t.Errorf("ByModel calls = %d, want %d", got, want)
	}
}
