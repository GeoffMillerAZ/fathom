// typedpass_serial.go — W-N (docs/plans/daemon-footprint.md), following
// O108's reframing (.fathom/observations.md): the always-on daemon's
// footprint is not a live leak but idle heap left behind by PEAKS —
// concurrent typed-edge passes over large services (app-kit's ~390
// packages triggered a 1.4 -> 5.2 GB climb from 19 `ingest` calls in 20
// minutes) that darwin reclaims lazily. memscavenge.go (W-L) bounds how
// long a peak's OS-visible footprint survives; this file bounds the peak
// itself by making sure only one typed-edge pass ever runs at a time,
// process-wide.
//
// The semaphore lives at package level (not on *Pipeline) because the
// resource it protects — go/packages.Load's own memory and subprocess
// use — is a property of the PROCESS, not of any one Pipeline instance;
// a daemon with more than one Pipeline (there is normally exactly one,
// but nothing enforces that) must still never run two typed-edge passes
// concurrently.
package fusion

import (
	"context"
	"runtime"
	"sync/atomic"
	"time"
)

// typedPassSem is a 1-buffered channel used as a mutex-with-cancellation:
// a plain sync.Mutex has no way to honor ctx while waiting for Lock, so a
// channel send/receive pair stands in for it (acquireTypedPassSlot below).
var typedPassSem = make(chan struct{}, 1)

// typedPassInflight is 0 or 1: whether a typed-edge pass currently holds
// typedPassSem. Exposed as the fathom_typed_pass_inflight gauge.
var typedPassInflight int64

// typedPassWaitedTotal counts passes that found typedPassSem already held
// and had to block for it — process-lifetime, like typedPassTotal.
// Exposed as the fathom_typed_pass_waited_total counter.
var typedPassWaitedTotal int64

// TypedPassInflight reports whether a typed-edge pass is currently running
// in this process (1) or not (0) — the fathom_typed_pass_inflight gauge on
// /metrics (via pkg/client.Client.TypedPassInflight).
func TypedPassInflight() int64 {
	return atomic.LoadInt64(&typedPassInflight)
}

// TypedPassWaitedTotal returns the process-lifetime count of typed-edge
// passes that had to wait for another pass to finish before they could
// start — the fathom_typed_pass_waited_total counter on /metrics (via
// pkg/client.Client.TypedPassWaitedTotal). A daemon that only ever ingests
// one service at a time keeps this at 0; a daemon whose ingest calls
// overlap (concurrent `ingest` MCP calls, or an explicit ingest racing
// auto-refresh) grows it instead of letting both passes run at once.
func TypedPassWaitedTotal() int64 {
	return atomic.LoadInt64(&typedPassWaitedTotal)
}

// acquireTypedPassSlot blocks until the process-wide typed-edge-pass
// semaphore is free or ctx is done, whichever comes first. On success it
// returns a non-nil release func that the caller MUST call exactly once
// (typically via defer) to free the slot and return the gauge to 0; on
// ctx cancellation it returns ctx.Err() and a nil release — nothing was
// acquired, so there is nothing to release.
//
// The non-blocking first attempt (the `select` with a `default`) is what
// lets typedPassWaitedTotal count only passes that ACTUALLY had to wait,
// rather than every pass — a pass that finds the slot free acquires it
// without ever touching the counter.
func acquireTypedPassSlot(ctx context.Context) (release func(), err error) {
	select {
	case typedPassSem <- struct{}{}:
		atomic.StoreInt64(&typedPassInflight, 1)
		return releaseTypedPassSlot, nil
	default:
	}
	atomic.AddInt64(&typedPassWaitedTotal, 1)
	select {
	case typedPassSem <- struct{}{}:
		atomic.StoreInt64(&typedPassInflight, 1)
		return releaseTypedPassSlot, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

// releaseTypedPassSlot is acquireTypedPassSlot's paired release func.
func releaseTypedPassSlot() {
	atomic.StoreInt64(&typedPassInflight, 0)
	<-typedPassSem
}

// samplePeakHeapInuse runs fn while sampling runtime.MemStats.HeapInuse on
// a short ticker, returning the highest value observed across the call —
// including a sample taken immediately before fn starts and one taken
// immediately after it returns. Sampling DURING fn, not only before and
// after, is what makes the result meaningful: O108's own production
// evidence is that heap_inuse can already be back down to ~10 MiB by the
// time a typed-edge pass returns (a GC ran before the caller got to
// sample), while the pass's transient peak — the thing that actually
// inflates heap_sys/RSS — happened somewhere in the middle. A before/after
// sample alone would report that 10 MiB as "the peak" and hide exactly the
// signal this metric exists to show.
//
// The 100ms ticker is cheap relative to a typed-edge pass, which for a
// service of any real size runs for at least hundreds of milliseconds
// (go/packages.Load shells out to the go toolchain) and for a large one
// (app-kit's ~390 packages) for seconds.
func samplePeakHeapInuse(fn func()) uint64 {
	var peak uint64
	var ms runtime.MemStats
	sample := func() {
		runtime.ReadMemStats(&ms)
		if ms.HeapInuse > peak {
			peak = ms.HeapInuse
		}
	}
	sample()

	stop := make(chan struct{})
	done := make(chan struct{})
	go func() {
		defer close(done)
		ticker := time.NewTicker(100 * time.Millisecond)
		defer ticker.Stop()
		for {
			select {
			case <-stop:
				return
			case <-ticker.C:
				sample()
			}
		}
	}()

	fn()

	close(stop)
	<-done // synchronizes-before the final sample: no concurrent access to ms/peak
	sample()
	return peak
}
