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
	"fmt"
	"runtime/metrics"
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

// passHeap is what sampleProcessHeapDuringPass saw.
type passHeap struct {
	Peak        uint64 // the process's heap in use (objects + unused spans), highest seen
	MaxInFlight int    // the most in-flight tickets registered at once during the pass
}

// sampleProcessHeapDuringPass runs fn while sampling the PROCESS's heap in
// use from runtime/metrics (/memory/classes/heap/objects + …/heap/unused —
// the old HeapInuse, read without ReadMemStats' stop-the-world) and the
// number of in-flight tickets (inflight; nil = none) on a 100 ms ticker,
// including a sample just before fn starts and one just after it returns.
//
// RT-46 (design §2.2; hazard typed-pass-peak-misattributes-process-heap):
// this is the whole process's heap while the pass ran — every concurrent
// request's allocation is in it — so the typed-pass line labels it
// "process heap during pass" with the in-flight tickets beside it, never as
// the pass's own cost. Sampling DURING fn is still what makes it
// meaningful (O108: heap can be back to ~10 MiB by the time a pass returns,
// while the transient peak happened in the middle).
func sampleProcessHeapDuringPass(fn func(), inflight func() int) passHeap {
	var h passHeap
	ms := [2]metrics.Sample{{Name: "/memory/classes/heap/objects:bytes"}, {Name: "/memory/classes/heap/unused:bytes"}}
	sample := func() {
		metrics.Read(ms[:])
		var heap uint64
		for _, m := range ms {
			if m.Value.Kind() == metrics.KindUint64 {
				heap += m.Value.Uint64()
			}
		}
		if heap > h.Peak {
			h.Peak = heap
		}
		if inflight != nil {
			if n := inflight(); n > h.MaxInFlight {
				h.MaxInFlight = n
			}
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
	<-done // synchronizes-before the final sample: no concurrent access to ms/h
	sample()
	return h
}

// typedPassLine is the typed-edge pass's summary line (RT-46).
func typedPassLine(service string, rep TypedEdgeReport, h passHeap) string {
	packages := rep.PackagesLoaded + rep.PackagesCached
	edges := rep.EdgesDirect + rep.EdgesIfaceDispatch + rep.EdgesUnresolved + rep.EdgesFuncValue
	// "edges" is what this pass WROTE: a package whose content hash is
	// unchanged keeps its stored edges and contributes none here, so an
	// all-cached pass reads "0 edges written", never a bare "0 edges" over
	// a graph that still holds every edge.
	return fmt.Sprintf("fusion: typed-edge pass %q: %d packages (%d re-resolved, %d unchanged and reused), %d edges written, process heap during pass %.1f MiB with up to %d in-flight tickets (the whole process, not the pass's own cost), %s\n",
		service, packages, rep.PackagesLoaded, rep.PackagesCached, edges, float64(h.Peak)/(1024*1024), h.MaxInFlight, rep.Duration)
}
