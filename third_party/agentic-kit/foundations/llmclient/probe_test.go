package llmclient_test

import (
	"context"
	"errors"
	"sort"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/geoffmilleraz/agentic-kit/foundations/llmclient"
	"github.com/geoffmilleraz/agentic-kit/foundations/llmclient/anthropic"
	"github.com/geoffmilleraz/agentic-kit/foundations/llmclient/bedrock"
)

// pingerFunc is a functional Pinger for table-driven tests.
type pingerFunc func(ctx context.Context) error

func (f pingerFunc) Ping(ctx context.Context) error { return f(ctx) }

// -------------------------------------------------------------------------
// Pinger interface compile-time assertions: both adapter clients must satisfy
// the interface without any new files needing to be modified.
// -------------------------------------------------------------------------

var (
	_ llmclient.Pinger = (*anthropic.Client)(nil)
	_ llmclient.Pinger = (*bedrock.Client)(nil)
)

// -------------------------------------------------------------------------
// ProbeAll — semantics and concurrency
// -------------------------------------------------------------------------

func TestProbeAllEmptyMapReturnsEmpty(t *testing.T) {
	out := llmclient.ProbeAll(context.Background(), map[string]llmclient.Pinger{})
	if out == nil {
		t.Fatal("result must not be nil for an empty input")
	}
	if len(out) != 0 {
		t.Fatalf("got %d entries, want 0", len(out))
	}
}

func TestProbeAllAllHealthy(t *testing.T) {
	lanes := map[string]llmclient.Pinger{
		"a": pingerFunc(func(ctx context.Context) error { return nil }),
		"b": pingerFunc(func(ctx context.Context) error { return nil }),
		"c": pingerFunc(func(ctx context.Context) error { return nil }),
	}
	out := llmclient.ProbeAll(context.Background(), lanes)
	// Healthy lanes are absent from the result map (absent-means-healthy semantics).
	if len(out) != 0 {
		t.Fatalf("all-healthy probe returned %d entries, want 0: %v", len(out), out)
	}
}

func TestProbeAllAllFailing(t *testing.T) {
	errA := &llmclient.AuthError{Message: "bad key for a"}
	errB := &llmclient.ServerError{StatusCode: 500, Message: "server down for b"}
	errC := &llmclient.RateLimitError{Message: "throttled on c"}
	lanes := map[string]llmclient.Pinger{
		"a": pingerFunc(func(ctx context.Context) error { return errA }),
		"b": pingerFunc(func(ctx context.Context) error { return errB }),
		"c": pingerFunc(func(ctx context.Context) error { return errC }),
	}
	out := llmclient.ProbeAll(context.Background(), lanes)
	if len(out) != 3 {
		t.Fatalf("got %d entries, want 3: %v", len(out), out)
	}
	if !errors.Is(out["a"], errA) {
		t.Errorf("lane a: got %v, want %v", out["a"], errA)
	}
	if !errors.Is(out["b"], errB) {
		t.Errorf("lane b: got %v, want %v", out["b"], errB)
	}
	if !errors.Is(out["c"], errC) {
		t.Errorf("lane c: got %v, want %v", out["c"], errC)
	}
}

func TestProbeAllMixedResults(t *testing.T) {
	authErr := &llmclient.AuthError{Message: "no key"}
	lanes := map[string]llmclient.Pinger{
		"good":   pingerFunc(func(ctx context.Context) error { return nil }),
		"broken": pingerFunc(func(ctx context.Context) error { return authErr }),
	}
	out := llmclient.ProbeAll(context.Background(), lanes)
	if _, ok := out["good"]; ok {
		t.Error("healthy lane 'good' must not appear in result map")
	}
	if got, ok := out["broken"]; !ok {
		t.Error("failing lane 'broken' must appear in result map")
	} else if !errors.Is(got, authErr) {
		t.Errorf("lane 'broken': got %v, want %v", got, authErr)
	}
}

// TestProbeAllNeverStopsEarly proves that a fast failure on one lane does not
// prevent another slow lane from completing — all lanes are always probed.
func TestProbeAllNeverStopsEarly(t *testing.T) {
	var slowReached atomic.Bool
	barrier := make(chan struct{})

	lanes := map[string]llmclient.Pinger{
		"fast-fail": pingerFunc(func(ctx context.Context) error {
			return &llmclient.AuthError{Message: "fail fast"}
		}),
		"slow": pingerFunc(func(ctx context.Context) error {
			close(barrier) // signal that the slow lane started
			slowReached.Store(true)
			return nil
		}),
	}

	out := llmclient.ProbeAll(context.Background(), lanes)

	// ProbeAll must have waited for the slow lane.
	if !slowReached.Load() {
		t.Error("ProbeAll returned before the slow lane completed")
	}
	// fast-fail is in the result; slow is absent (healthy).
	if _, ok := out["fast-fail"]; !ok {
		t.Error("fast-fail must be in the result")
	}
	if _, ok := out["slow"]; ok {
		t.Error("healthy slow lane must not be in the result")
	}
}

// TestProbeAllContextCancellation proves context cancellation is forwarded to
// all lanes simultaneously and the map carries the context error for each.
func TestProbeAllContextCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())

	var started sync.WaitGroup
	started.Add(2)
	lanes := map[string]llmclient.Pinger{
		"x": pingerFunc(func(ctx context.Context) error {
			started.Done()
			<-ctx.Done()
			return ctx.Err()
		}),
		"y": pingerFunc(func(ctx context.Context) error {
			started.Done()
			<-ctx.Done()
			return ctx.Err()
		}),
	}

	// Cancel after both lanes are blocking on ctx.Done().
	go func() {
		started.Wait()
		cancel()
	}()

	out := llmclient.ProbeAll(ctx, lanes)
	if len(out) != 2 {
		t.Fatalf("want both lanes to report context error, got %d entries", len(out))
	}
	for name, err := range out {
		if !errors.Is(err, context.Canceled) {
			t.Errorf("lane %q: got %v, want context.Canceled", name, err)
		}
	}
}

// TestProbeAllConcurrent proves ProbeAll is race-clean under the race detector:
// a larger fan-out is driven concurrently without data races.
func TestProbeAllConcurrent(t *testing.T) {
	const n = 20
	lanes := make(map[string]llmclient.Pinger, n)
	for i := 0; i < n; i++ {
		i := i
		name := string(rune('a' + i))
		if i%3 == 0 {
			lanes[name] = pingerFunc(func(ctx context.Context) error {
				return &llmclient.ServerError{Message: "transient"}
			})
		} else {
			lanes[name] = pingerFunc(func(ctx context.Context) error {
				time.Sleep(time.Microsecond * time.Duration(i))
				return nil
			})
		}
	}
	out := llmclient.ProbeAll(context.Background(), lanes)
	// Just check we got results for the failing lanes (n/3 of them fail).
	var failCount int
	for _, err := range out {
		if err != nil {
			failCount++
		}
	}
	// i%3==0 lanes: 0,3,6,9,12,15,18 → 7 lanes
	if failCount != 7 {
		t.Errorf("got %d failing lanes, want 7", failCount)
	}
}

// TestProbeAllResultKeySet proves every lane in the input has a corresponding
// entry in the result (either present=error or absent=healthy), and no extra
// keys appear.
func TestProbeAllResultKeySet(t *testing.T) {
	errX := &llmclient.InvalidRequestError{Message: "bad config"}
	lanes := map[string]llmclient.Pinger{
		"healthy-1": pingerFunc(func(ctx context.Context) error { return nil }),
		"healthy-2": pingerFunc(func(ctx context.Context) error { return nil }),
		"broken-x":  pingerFunc(func(ctx context.Context) error { return errX }),
	}
	out := llmclient.ProbeAll(context.Background(), lanes)

	// No extra keys.
	for k := range out {
		if _, ok := lanes[k]; !ok {
			t.Errorf("result has unexpected key %q", k)
		}
	}
	// Healthy lanes absent, broken present.
	if _, ok := out["healthy-1"]; ok {
		t.Error("healthy-1 must not appear in result")
	}
	if _, ok := out["healthy-2"]; ok {
		t.Error("healthy-2 must not appear in result")
	}
	if !errors.Is(out["broken-x"], errX) {
		t.Errorf("broken-x: got %v, want errX", out["broken-x"])
	}
}

// TestProbeAllDeterministicKeys proves the output key set is stable across
// repeated calls with the same input — no non-determinism from goroutine
// scheduling leaks into the result keys.
func TestProbeAllDeterministicKeys(t *testing.T) {
	names := []string{"alpha", "beta", "gamma", "delta"}
	lanes := make(map[string]llmclient.Pinger, len(names))
	for i, n := range names {
		n := n
		i := i
		if i%2 == 0 {
			lanes[n] = pingerFunc(func(ctx context.Context) error {
				return &llmclient.AuthError{Message: "bad"}
			})
		} else {
			lanes[n] = pingerFunc(func(ctx context.Context) error { return nil })
		}
	}

	// Run many times and collect the key sets.
	for i := 0; i < 32; i++ {
		out := llmclient.ProbeAll(context.Background(), lanes)
		gotKeys := make([]string, 0, len(out))
		for k := range out {
			gotKeys = append(gotKeys, k)
		}
		sort.Strings(gotKeys)
		// "alpha" (i=0, even→fail), "gamma" (i=2, even→fail) should appear.
		wantKeys := []string{"alpha", "gamma"}
		if len(gotKeys) != len(wantKeys) {
			t.Fatalf("iter %d: got keys %v, want %v", i, gotKeys, wantKeys)
		}
		for j := range gotKeys {
			if gotKeys[j] != wantKeys[j] {
				t.Fatalf("iter %d: key[%d] = %q, want %q", i, j, gotKeys[j], wantKeys[j])
			}
		}
	}
}

// TestProbeAllSingleLane is a degenerate case: a single-lane map works the
// same as a multi-lane map.
func TestProbeAllSingleLane(t *testing.T) {
	want := &llmclient.AuthError{Message: "one bad"}
	out := llmclient.ProbeAll(context.Background(), map[string]llmclient.Pinger{
		"only": pingerFunc(func(ctx context.Context) error { return want }),
	})
	if !errors.Is(out["only"], want) {
		t.Errorf("got %v, want %v", out["only"], want)
	}
}
