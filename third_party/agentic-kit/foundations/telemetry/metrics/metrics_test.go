package metrics_test

import (
	"fmt"
	"strings"
	"sync"
	"testing"

	"github.com/geoffmilleraz/fathom/third_party/agentic-kit/foundations/telemetry/metrics"
)

// ---- Counter: basic behaviour ------------------------------------------------

func TestCounter_IncAndAdd(t *testing.T) {
	reg := metrics.NewRegistry()
	c := reg.Counter("requests_total")
	c.Inc()
	c.Inc()
	c.Add(3)

	want := "# TYPE requests_total counter\nrequests_total 5\n"
	if got := reg.Expose(); got != want {
		t.Errorf("Expose mismatch:\n got: %q\nwant: %q", got, want)
	}
}

func TestCounter_AddNonPositiveIgnored(t *testing.T) {
	tests := []struct {
		name  string
		delta int64
	}{
		{name: "negative", delta: -5},
		{name: "zero", delta: 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reg := metrics.NewRegistry()
			c := reg.Counter("monotonic_total")
			c.Inc()
			c.Add(tt.delta)
			want := "# TYPE monotonic_total counter\nmonotonic_total 1\n"
			if got := reg.Expose(); got != want {
				t.Errorf("Add(%d) must be ignored:\n got: %q\nwant: %q", tt.delta, got, want)
			}
		})
	}
}

func TestCounter_SameNameReturnsSameInstance(t *testing.T) {
	reg := metrics.NewRegistry()
	a := reg.Counter("shared_total")
	b := reg.Counter("shared_total")
	if a != b {
		t.Error("Counter with the same name must return the same instance")
	}
	a.Inc()
	b.Inc()
	if !strings.Contains(reg.Expose(), "shared_total 2\n") {
		t.Errorf("both increments must land on one series; got:\n%s", reg.Expose())
	}
}

// ---- Counter: labels -----------------------------------------------------------

func TestCounter_LabelOrderIsCanonicalised(t *testing.T) {
	reg := metrics.NewRegistry()
	a := reg.Counter("calls_total", "tool", "search", "outcome", "ok")
	b := reg.Counter("calls_total", "outcome", "ok", "tool", "search")
	if a != b {
		t.Fatal("same label set in different order must name the same series")
	}
	a.Inc()
	b.Inc()

	want := "# TYPE calls_total counter\n" +
		`calls_total{outcome="ok",tool="search"} 2` + "\n"
	if got := reg.Expose(); got != want {
		t.Errorf("Expose mismatch:\n got: %q\nwant: %q", got, want)
	}
}

func TestCounter_DistinctLabelValuesAreDistinctSeries(t *testing.T) {
	reg := metrics.NewRegistry()
	reg.Counter("calls_total", "tool", "search").Add(2)
	reg.Counter("calls_total", "tool", "fetch").Inc()
	reg.Counter("calls_total").Add(4) // unlabeled series of the same family

	want := "# TYPE calls_total counter\n" +
		"calls_total 4\n" +
		`calls_total{tool="fetch"} 1` + "\n" +
		`calls_total{tool="search"} 2` + "\n"
	if got := reg.Expose(); got != want {
		t.Errorf("Expose mismatch:\n got: %q\nwant: %q", got, want)
	}
}

func TestCounter_OddLabelCountPanics(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Error("Counter with an odd number of label arguments must panic")
		}
	}()
	metrics.NewRegistry().Counter("bad_total", "key_without_value")
}

func TestObserve_OddLabelCountPanics(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Error("Observe with an odd number of label arguments must panic")
		}
	}()
	metrics.NewRegistry().Observe("bad_seconds", 1.0, "key_without_value")
}

func TestLabelValueEscaping(t *testing.T) {
	reg := metrics.NewRegistry()
	reg.Counter("weird_total", "q", "say \"hi\"\\\n").Inc()

	want := "# TYPE weird_total counter\n" +
		`weird_total{q="say \"hi\"\\\n"} 1` + "\n"
	if got := reg.Expose(); got != want {
		t.Errorf("Expose mismatch:\n got: %q\nwant: %q", got, want)
	}
}

// ---- Observe -------------------------------------------------------------------

func TestObserve_AccumulatesSumAndCount(t *testing.T) {
	reg := metrics.NewRegistry()
	reg.Observe("latency_seconds", 0.5)
	reg.Observe("latency_seconds", 0.25)

	want := "# TYPE latency_seconds summary\n" +
		"latency_seconds_sum 0.75\n" +
		"latency_seconds_count 2\n"
	if got := reg.Expose(); got != want {
		t.Errorf("Expose mismatch:\n got: %q\nwant: %q", got, want)
	}
}

func TestObserve_WithLabels(t *testing.T) {
	reg := metrics.NewRegistry()
	reg.Observe("tokens", 100, "model", "alpha")
	reg.Observe("tokens", 50, "model", "alpha")
	reg.Observe("tokens", 7, "model", "beta")

	want := "# TYPE tokens summary\n" +
		`tokens_sum{model="alpha"} 150` + "\n" +
		`tokens_count{model="alpha"} 2` + "\n" +
		`tokens_sum{model="beta"} 7` + "\n" +
		`tokens_count{model="beta"} 1` + "\n"
	if got := reg.Expose(); got != want {
		t.Errorf("Expose mismatch:\n got: %q\nwant: %q", got, want)
	}
}

// ---- Expose: golden + determinism ------------------------------------------------

func TestExpose_EmptyRegistry(t *testing.T) {
	if got := metrics.NewRegistry().Expose(); got != "" {
		t.Errorf("empty registry must expose empty string; got %q", got)
	}
}

// TestExpose_Golden locks the full deterministic output: counters before
// summaries, names sorted, series sorted by label block (unlabeled first).
func TestExpose_Golden(t *testing.T) {
	reg := metrics.NewRegistry()
	reg.Counter("tool_calls_total").Add(3)
	reg.Counter("tool_calls_total", "tool", "search").Inc()
	reg.Counter("tool_calls_total", "tool", "fetch").Add(2)
	reg.Counter("errors_total").Inc()
	reg.Observe("latency_seconds", 0.5)
	reg.Observe("latency_seconds", 0.25)
	reg.Observe("tokens", 100, "model", "alpha")

	want := `# TYPE errors_total counter
errors_total 1
# TYPE tool_calls_total counter
tool_calls_total 3
tool_calls_total{tool="fetch"} 2
tool_calls_total{tool="search"} 1
# TYPE latency_seconds summary
latency_seconds_sum 0.75
latency_seconds_count 2
# TYPE tokens summary
tokens_sum{model="alpha"} 100
tokens_count{model="alpha"} 1
`
	got := reg.Expose()
	if got != want {
		t.Errorf("golden mismatch:\n got:\n%s\nwant:\n%s", got, want)
	}
	// Determinism: repeated calls produce byte-identical output.
	for i := 0; i < 5; i++ {
		if again := reg.Expose(); again != got {
			t.Fatalf("Expose is not deterministic on call %d:\n got: %q\nfirst: %q", i+2, again, got)
		}
	}
}

// ---- Concurrency ------------------------------------------------------------------

// TestConcurrentUse hammers counters, observations, and Expose from many
// goroutines; run with -race. Final values must be exact (no lost updates).
func TestConcurrentUse(t *testing.T) {
	reg := metrics.NewRegistry()
	const (
		goroutines = 16
		iterations = 250
	)

	var wg sync.WaitGroup
	wg.Add(goroutines)
	for g := 0; g < goroutines; g++ {
		go func(g int) {
			defer wg.Done()
			for i := 0; i < iterations; i++ {
				reg.Counter("concurrent_total").Inc()
				reg.Counter("concurrent_total", "worker", "even").Add(2)
				reg.Observe("concurrent_seconds", 1.0)
				if i%50 == 0 {
					_ = reg.Expose() // concurrent reads must be safe
				}
			}
		}(g)
	}
	wg.Wait()

	const total = goroutines * iterations // 4000
	want := fmt.Sprintf("# TYPE concurrent_total counter\n"+
		"concurrent_total %d\n"+
		"concurrent_total{worker=\"even\"} %d\n"+
		"# TYPE concurrent_seconds summary\n"+
		"concurrent_seconds_sum %d\n"+
		"concurrent_seconds_count %d\n",
		total, 2*total, total, total)
	if got := reg.Expose(); got != want {
		t.Errorf("concurrent result mismatch:\n got: %q\nwant: %q", got, want)
	}
}

// ---- Special float values -----------------------------------------------------

func TestExpose_SpecialFloatValues(t *testing.T) {
	tests := []struct {
		name    string
		samples []float64
		wantSum string
	}{
		{name: "plus_inf", samples: []float64{1, inf(1)}, wantSum: "+Inf"},
		{name: "minus_inf", samples: []float64{inf(-1)}, wantSum: "-Inf"},
		{name: "nan", samples: []float64{nan()}, wantSum: "NaN"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reg := metrics.NewRegistry()
			for _, v := range tt.samples {
				reg.Observe("x", v)
			}
			got := reg.Expose()
			if !strings.Contains(got, "x_sum "+tt.wantSum+"\n") {
				t.Errorf("expected sum rendered as %q; got:\n%s", tt.wantSum, got)
			}
		})
	}
}

func inf(sign int) float64 {
	if sign >= 0 {
		return 1.0 / zero()
	}
	return -1.0 / zero()
}

func nan() float64 { return zero() / zero() }

// zero defeats constant folding so inf/nan build without compile errors.
func zero() float64 { var z float64; return z }
