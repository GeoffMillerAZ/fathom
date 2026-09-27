package fusion

import (
	"runtime"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

var heapSink []byte

// TestTypedPassLogLabelsProcessHeap pins RT-46 (design §2.2; hazard
// typed-pass-peak-misattributes-process-heap): the typed pass's heap
// reading comes from runtime/metrics (no stop-the-world ReadMemStats), is
// the whole PROCESS's heap while the pass ran — every concurrent request's
// allocation included — and the log line says so, with the most in-flight
// tickets seen during the pass beside it, never "peak heap_inuse" as if it
// were the pass's own cost.
func TestTypedPassLogLabelsProcessHeap(t *testing.T) {
	var tickets atomic.Int64
	tickets.Store(1)
	const hold = 64 << 20
	h := sampleProcessHeapDuringPass(func() {
		heapSink = make([]byte, hold)
		for i := range heapSink {
			heapSink[i] = byte(i)
		}
		tickets.Store(3)
		time.Sleep(250 * time.Millisecond)
		tickets.Store(2)
	}, func() int { return int(tickets.Load()) })
	heapSink = nil
	runtime.GC()
	if h.Peak < hold {
		t.Fatalf("process heap during pass = %d, want at least the %d the pass held", h.Peak, hold)
	}
	if h.MaxInFlight != 3 {
		t.Fatalf("max in-flight tickets during pass = %d, want 3", h.MaxInFlight)
	}
	line := typedPassLine("svc", TypedEdgeReport{PackagesLoaded: 2, PackagesCached: 1, EdgesDirect: 5, Duration: time.Second}, h)
	for _, want := range []string{"process heap during pass", "3 in-flight tickets", "not the pass's own cost"} {
		if !strings.Contains(line, want) {
			t.Errorf("typed-pass line %q lacks %q", line, want)
		}
	}
	if strings.Contains(line, "peak heap_inuse") {
		t.Errorf("typed-pass line still labels the process heap as the pass's peak: %q", line)
	}
}
