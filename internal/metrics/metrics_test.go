package metrics_test

import (
	"fmt"
	"strings"
	"sync"
	"testing"

	"github.com/geoffmilleraz/sextant/internal/metrics"
)

// ─── Counter tests ───────────────────────────────────────────────────────────

func TestCounter_ZeroByDefault(t *testing.T) {
	reg := metrics.NewRegistry()
	out := reg.Expose()
	// Registry starts empty — Expose must not panic and should return empty string.
	if strings.Contains(out, "my_counter 1") {
		t.Error("counter should not be 1 without Inc")
	}
}

func TestCounter_Inc(t *testing.T) {
	reg := metrics.NewRegistry()
	reg.Counter("http_requests_total").Inc()
	reg.Counter("http_requests_total").Inc()
	reg.Counter("http_requests_total").Inc()

	out := reg.Expose()
	if !strings.Contains(out, "http_requests_total 3") {
		t.Errorf("expected count 3 in output; got:\n%s", out)
	}
	if !strings.Contains(out, "# TYPE http_requests_total counter") {
		t.Errorf("expected TYPE annotation; got:\n%s", out)
	}
}

func TestCounter_MultipleNames(t *testing.T) {
	reg := metrics.NewRegistry()
	reg.Counter("alpha").Inc()
	reg.Counter("beta").Inc()
	reg.Counter("beta").Inc()

	out := reg.Expose()
	if !strings.Contains(out, "alpha 1") {
		t.Errorf("expected alpha 1; got:\n%s", out)
	}
	if !strings.Contains(out, "beta 2") {
		t.Errorf("expected beta 2; got:\n%s", out)
	}
}

// ─── Observe tests ───────────────────────────────────────────────────────────

func TestObserve_Sum_And_Count(t *testing.T) {
	reg := metrics.NewRegistry()
	reg.Observe("latency_seconds", 0.1)
	reg.Observe("latency_seconds", 0.2)
	reg.Observe("latency_seconds", 0.7)

	out := reg.Expose()
	if !strings.Contains(out, "latency_seconds_count 3") {
		t.Errorf("expected count=3; got:\n%s", out)
	}
	if !strings.Contains(out, "# TYPE latency_seconds summary") {
		t.Errorf("expected TYPE summary annotation; got:\n%s", out)
	}
	if !strings.Contains(out, "latency_seconds_sum") {
		t.Errorf("expected sum line; got:\n%s", out)
	}
}

// ─── Determinism / sorted output ─────────────────────────────────────────────

func TestExpose_Sorted(t *testing.T) {
	reg := metrics.NewRegistry()
	// Insert in reverse alphabetical order.
	reg.Counter("zzz_counter").Inc()
	reg.Counter("aaa_counter").Inc()
	reg.Counter("mmm_counter").Inc()
	reg.Observe("zzz_obs", 1.0)
	reg.Observe("aaa_obs", 2.0)

	out := reg.Expose()
	lines := strings.Split(strings.TrimSpace(out), "\n")

	// Collect non-TYPE, non-empty metric lines.
	var metricLines []string
	for _, l := range lines {
		if strings.HasPrefix(l, "#") || l == "" {
			continue
		}
		metricLines = append(metricLines, l)
	}

	aaa := indexContaining(metricLines, "aaa_counter")
	mmm := indexContaining(metricLines, "mmm_counter")
	zzz := indexContaining(metricLines, "zzz_counter")
	if aaa < 0 || mmm < 0 || zzz < 0 {
		t.Fatalf("expected counter lines in output; got:\n%s", out)
	}
	if !(aaa < mmm && mmm < zzz) {
		t.Errorf("counter lines not in sorted order (aaa=%d mmm=%d zzz=%d);\n%s",
			aaa, mmm, zzz, out)
	}

	aObs := indexContaining(metricLines, "aaa_obs")
	zObs := indexContaining(metricLines, "zzz_obs")
	if aObs < 0 || zObs < 0 {
		t.Fatalf("expected observation lines in output; got:\n%s", out)
	}
	if aObs >= zObs {
		t.Errorf("observation lines not sorted (aaa=%d zzz=%d);\n%s", aObs, zObs, out)
	}
}

func indexContaining(lines []string, sub string) int {
	for i, l := range lines {
		if strings.Contains(l, sub) {
			return i
		}
	}
	return -1
}

// ─── Concurrency / race tests ─────────────────────────────────────────────────

func TestCounter_ConcurrentInc(t *testing.T) {
	reg := metrics.NewRegistry()
	const goroutines = 50
	const incsPerGoroutine = 100

	var wg sync.WaitGroup
	wg.Add(goroutines)
	for i := 0; i < goroutines; i++ {
		go func() {
			defer wg.Done()
			for j := 0; j < incsPerGoroutine; j++ {
				reg.Counter("concurrent_counter").Inc()
			}
		}()
	}
	wg.Wait()

	out := reg.Expose()
	expected := goroutines * incsPerGoroutine
	want := fmt.Sprintf("concurrent_counter %d", expected)
	if !strings.Contains(out, want) {
		t.Errorf("expected %q in output; got:\n%s", want, out)
	}
}

func TestObserve_ConcurrentObserve(t *testing.T) {
	reg := metrics.NewRegistry()
	const goroutines = 20
	const obsPerGoroutine = 50

	var wg sync.WaitGroup
	wg.Add(goroutines)
	for i := 0; i < goroutines; i++ {
		go func() {
			defer wg.Done()
			for j := 0; j < obsPerGoroutine; j++ {
				reg.Observe("concurrent_obs", 1.0)
			}
		}()
	}
	wg.Wait()

	out := reg.Expose()
	expected := goroutines * obsPerGoroutine
	want := fmt.Sprintf("concurrent_obs_count %d", expected)
	if !strings.Contains(out, want) {
		t.Errorf("expected %q; got:\n%s", want, out)
	}
}

func TestExpose_ConcurrentExposeAndInc(t *testing.T) {
	// Expose() must not panic or race when called concurrently with Inc/Observe.
	reg := metrics.NewRegistry()

	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(2)
		go func() {
			defer wg.Done()
			for j := 0; j < 100; j++ {
				reg.Counter("x").Inc()
				reg.Observe("y", 0.5)
			}
		}()
		go func() {
			defer wg.Done()
			for j := 0; j < 10; j++ {
				_ = reg.Expose()
			}
		}()
	}
	wg.Wait()
}

// ─── Prometheus format sanity ─────────────────────────────────────────────────

func TestExpose_PrometheusFormat(t *testing.T) {
	reg := metrics.NewRegistry()
	reg.Counter("build_info").Inc()
	reg.Observe("gc_pause_seconds", 0.002)

	out := reg.Expose()
	lines := strings.Split(strings.TrimSpace(out), "\n")
	for i, l := range lines {
		if strings.HasPrefix(l, "# TYPE ") {
			if i+1 >= len(lines) {
				t.Errorf("# TYPE line at position %d has no following metric line", i)
			}
		}
	}
}
