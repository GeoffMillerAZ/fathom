package callgraph

// cache_memory_test.go — O108 (.fathom/observations.md): the always-on
// daemon's memory grew ~0.9 GB/hour over 9.4h. The watermark-keyed
// call-graph memo (cache.go) was candidate (a) — one graph retained per
// (service, watermark) with an LRU eviction that, IF it merely dropped
// entries from the logical slice without actually releasing them (a
// classic Go footgun: re-slicing off the front of a growing backing array
// does not by itself make evicted elements collectible), would explain
// exactly this shape: entries/hits/misses stay bounded while heap keeps
// climbing.
//
// These two tests reproduce the incident's operating conditions directly
// against the cache (no HTTP, no daemon process) at a scale well beyond
// what the synthetic HTTP driver could run in a reasonable time: hundreds
// of distinct (service, watermark) misses against graphs the size of a
// real large service (10k nodes, comparable to this repo's own ~10.6k
// symbols), sequential AND under concurrent load (many simultaneous
// requests missing the same key — this daemon's own real operating
// condition, serving many parallel agent sessions at once). Both prove
// the memo is NOT the leak: entries never exceed the configured capacity,
// and heap_inuse plateaus rather than climbing without bound.
//
// This is the regression guard docs/plans/daemon-footprint.md's DF-6 row
// claims: it fails loudly if a future change to the eviction path
// reintroduces genuine unbounded retention.

import (
	"fmt"
	"runtime"
	"sync"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/core"
)

// cacheMemTestNodes builds n synthetic SymbolNodes for service at
// watermark — each watermark's names are distinct (mimicking real content
// change), with a simple linear call chain so Build has real edge-resolve
// work to do, not just node registration.
func cacheMemTestNodes(service string, watermark, n int) []core.SymbolNode {
	nodes := make([]core.SymbolNode, 0, n)
	for i := 0; i < n; i++ {
		pkg := fmt.Sprintf("pkg%d", i%50)
		name := fmt.Sprintf("Func%d_%d", i, watermark)
		var calls []string
		if i > 0 {
			calls = []string{core.SymbolKey(service, fmt.Sprintf("pkg%d", (i-1)%50), fmt.Sprintf("Func%d_%d", i-1, watermark))}
		}
		nodes = append(nodes, core.SymbolNode{
			Service: service, Package: pkg, Name: name,
			FilePath: pkg + "/file.go", Line: i + 1, Calls: calls,
		})
	}
	return nodes
}

// cacheMemTestHeapInuse forces two GC cycles (the second catches any
// finalizer-queued garbage the first cycle's sweep released) and reports
// runtime.MemStats.HeapInuse — live reachable bytes, not the OS-reservation
// high-water mark (HeapSys), which by design never shrinks even in a
// perfectly healthy process (see memscavenge.go's own doc comment).
func cacheMemTestHeapInuse() uint64 {
	runtime.GC()
	runtime.GC()
	var ms runtime.MemStats
	runtime.ReadMemStats(&ms)
	return ms.HeapInuse
}

// TestCachedCallGraph_SequentialChurnDoesNotLeak drives 300 sequential
// misses (roughly 6x the daemon's own 47-refreshes-in-9.4h incident rate,
// run back to back with no idle time) across 15 distinct services against
// a cap-8 cache, using 10k-node graphs — a real large service's scale.
// heap_inuse must plateau once the cache fills, not climb steadily
// afterward.
func TestCachedCallGraph_SequentialChurnDoesNotLeak(t *testing.T) {
	if testing.Short() {
		t.Skip("allocates ~300 x 10k-node graphs; skipped under -short")
	}
	const (
		nodesPerGraph = 10000
		numServices   = 15 // > cacheCap, forces real eviction every miss
		cacheCap      = 8
		cyclesTotal   = 300
	)
	cache := NewCachedCallGraph(cacheCap)

	var afterFill uint64 // heap_inuse once the cache first reaches cacheCap entries
	for cycle := 1; cycle <= cyclesTotal; cycle++ {
		svc := fmt.Sprintf("svc%d", cycle%numServices)
		wm := fmt.Sprintf("wm%d", cycle)
		nodes := cacheMemTestNodes(svc, cycle, nodesPerGraph)
		_ = cache.Graph(svc+"\x00both", wm, func() *Graph { return Build(nodes) })

		if entries, _, _ := cache.Stats(); entries == cacheCap && afterFill == 0 {
			afterFill = cacheMemTestHeapInuse()
			t.Logf("cache first full at cycle %d: heap_inuse = %.2f MiB", cycle, float64(afterFill)/(1<<20))
		}
	}

	entries, hits, misses := cache.Stats()
	t.Logf("final: entries=%d hits=%d misses=%d", entries, hits, misses)
	if entries > cacheCap {
		t.Fatalf("entries = %d, want <= %d (the memo must never exceed its configured capacity)", entries, cacheCap)
	}

	finalHeap := cacheMemTestHeapInuse()
	// KeepAlive: cache's own Stats() call above is its last syntactic use,
	// so Go's precise stack-liveness analysis is free to treat cache (and
	// everything it holds) as dead BEFORE this point — cacheMemTestHeapInuse's
	// runtime.GC() would then correctly, but misleadingly, collect the
	// still-configured-to-be-cached graphs and report a heap that looks
	// tiny regardless of whether the memo leaks. Keeping cache reachable
	// through the measurement is what makes this a real test of ITS
	// footprint rather than an artifact of dead-variable elimination.
	runtime.KeepAlive(cache)
	t.Logf("final heap_inuse = %.2f MiB (%.2fx of after-fill)", float64(finalHeap)/(1<<20), float64(finalHeap)/float64(afterFill))
	// Generous margin (2x): GC timing alone oscillates heap_inuse by a wide
	// band even in a genuinely bounded cache (observed ~145-240MB across
	// repeated runs at this scale) — the assertion is against SUSTAINED
	// growth (a leak), not against GC's own normal variance.
	if maxAllowed := 2.0 * float64(afterFill); float64(finalHeap) > maxAllowed {
		t.Fatalf("heap_inuse grew from %.2f MiB (cache full) to %.2f MiB after %d more misses — more than 2x, looks like a leak, not GC variance",
			float64(afterFill)/(1<<20), float64(finalHeap)/(1<<20), cyclesTotal-cacheCap)
	}
}

// TestCachedCallGraph_ConcurrentMissesDoNotLeak models many simultaneous
// agent sessions all missing the SAME (service, watermark) key at once —
// this daemon's own real operating condition (many parallel Claude Code
// sessions sharing one daemon), which every HTTP-level synthetic driver
// (sequential, one request at a time) could not exercise. Graph()'s
// build() call runs OUTSIDE the lock by design (a concurrent caller may
// insert the same key while build runs — see cache.go's own comment), so
// N concurrent misses for the same key each run their own full build()
// before only one gets published: a real "thundering herd" that
// transiently needs up to N times one graph's memory. This test proves
// that transient spike does not become a permanent floor.
func TestCachedCallGraph_ConcurrentMissesDoNotLeak(t *testing.T) {
	if testing.Short() {
		t.Skip("allocates many x 10k-node graphs concurrently; skipped under -short")
	}
	const (
		nodesPerGraph = 10000
		herdSize      = 12 // concurrent goroutines per round, > cacheCap
		rounds        = 50
		cacheCap      = 8
	)
	cache := NewCachedCallGraph(cacheCap)

	var afterFill uint64
	for r := 1; r <= rounds; r++ {
		svc := fmt.Sprintf("herd%d", r)
		wm := fmt.Sprintf("wm%d", r)
		nodes := cacheMemTestNodes(svc, r, nodesPerGraph)
		var wg sync.WaitGroup
		for g := 0; g < herdSize; g++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				_ = cache.Graph(svc+"\x00both", wm, func() *Graph { return Build(nodes) })
			}()
		}
		wg.Wait()

		if entries, _, _ := cache.Stats(); entries == cacheCap && afterFill == 0 {
			afterFill = cacheMemTestHeapInuse()
			t.Logf("cache first full at round %d: heap_inuse = %.2f MiB", r, float64(afterFill)/(1<<20))
		}
	}

	entries, hits, misses := cache.Stats()
	t.Logf("final: entries=%d hits=%d misses=%d", entries, hits, misses)
	if entries > cacheCap {
		t.Fatalf("entries = %d, want <= %d (the memo must never exceed its configured capacity under concurrent load)", entries, cacheCap)
	}

	finalHeap := cacheMemTestHeapInuse()
	runtime.KeepAlive(cache) // see the sequential test's own comment on this call
	t.Logf("final heap_inuse = %.2f MiB (%.2fx of after-fill)", float64(finalHeap)/(1<<20), float64(finalHeap)/float64(afterFill))
	if maxAllowed := 2.0 * float64(afterFill); float64(finalHeap) > maxAllowed {
		t.Fatalf("heap_inuse grew from %.2f MiB (cache full) to %.2f MiB after %d more herd rounds — more than 2x, looks like a leak, not GC variance",
			float64(afterFill)/(1<<20), float64(finalHeap)/(1<<20), rounds-cacheCap)
	}
}
