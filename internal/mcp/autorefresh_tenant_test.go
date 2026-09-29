package mcp

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// gatedIngestFake is a refreshFake whose IngestService blocks until release
// is closed, records the ctx error it saw when released, and tracks the
// maximum number of IngestService calls running at once (process-wide, via
// the shared counters the test hands it).
type gatedIngestFake struct {
	refreshFake
	started  chan string   // receives the service each time an ingest starts
	release  chan struct{} // closed to let every ingest finish
	running  *int64        // shared across fakes: ingests running now
	maxSeen  *int64        // shared across fakes: the peak of running
	mu       sync.Mutex
	ctxErrs  []error // ctx.Err() observed at the end of each ingest
	finished int
}

func (f *gatedIngestFake) IngestService(ctx context.Context, service string) error {
	n := atomic.AddInt64(f.running, 1)
	for {
		m := atomic.LoadInt64(f.maxSeen)
		if n <= m || atomic.CompareAndSwapInt64(f.maxSeen, m, n) {
			break
		}
	}
	f.started <- service
	<-f.release
	atomic.AddInt64(f.running, -1)
	f.mu.Lock()
	f.ctxErrs = append(f.ctxErrs, ctx.Err())
	f.finished++
	f.mu.Unlock()
	return nil
}

// TestEnsureIngestSurvivesRequestCancelAndSerializes pinned RT-149 (design
// §3.5, §7.1), which RT-99 supersedes: ensure() no longer runs or waits for
// an ingest at all. What survives of the contract is pinned here on the
// queue: the re-ingest a read requested completes uncancelled after that
// requester's ctx is cancelled, and two services (on two backends sharing
// the daemon's one queue) never ingest at the same time.
func TestEnsureIngestSurvivesRequestCancelAndSerializes(t *testing.T) {
	var running, maxSeen int64
	release := make(chan struct{})
	started := make(chan string, 4)
	fa := &gatedIngestFake{refreshFake: refreshFake{path: "/a", indexed: "old"}, started: started, release: release, running: &running, maxSeen: &maxSeen}
	fb := &gatedIngestFake{refreshFake: refreshFake{path: "/b", indexed: "old"}, started: started, release: release, running: &running, maxSeen: &maxSeen}
	// One queue for the process, as serve-http wires it; its jobs ingest on
	// whichever fake the service belongs to.
	both := &routingIngestFake{by: map[string]QueryInterface{"svc-a": fa, "svc-b": fb}}
	q := newTestRefreshQueue(t, both)
	arA := newARQueued(t, fa, "new", q)
	arB := newARQueued(t, fb, "new", q)

	ctxA, cancelA := context.WithCancel(context.Background())
	returned := make(chan struct{})
	go func() { arA.ensure(ctxA, "svc-a"); arB.ensure(context.Background(), "svc-b"); close(returned) }()
	select {
	case <-returned:
	case <-time.After(waitSafety):
		t.Fatal("ensure waited for an ingest; RT-99: it only requests one")
	}
	select {
	case <-started:
	case <-time.After(waitSafety):
		t.Fatal("no queued ingest started")
	}
	cancelA() // the requester goes away mid-ingest
	time.Sleep(20 * time.Millisecond)
	if got := atomic.LoadInt64(&running); got != 1 {
		t.Fatalf("ingests running at once = %d, want 1 (one heavy slot)", got)
	}
	close(release)
	settle(t, arA)
	settle(t, arB)
	if got := atomic.LoadInt64(&maxSeen); got != 1 {
		t.Fatalf("peak concurrent ingests = %d, want 1", got)
	}
	for name, f := range map[string]*gatedIngestFake{"svc-a": fa, "svc-b": fb} {
		f.mu.Lock()
		if f.finished != 1 || f.ctxErrs[0] != nil {
			t.Errorf("%s: finished %d, ctx errors %v; want one uncancelled ingest", name, f.finished, f.ctxErrs)
		}
		f.mu.Unlock()
	}
}

// routingIngestFake sends each service's IngestService to its own fake.
type routingIngestFake struct {
	refreshFake
	by map[string]QueryInterface
}

func (r *routingIngestFake) IngestService(ctx context.Context, service string) error {
	return r.by[service].IngestService(ctx, service)
}

// failingIngestFake fails its first n ingests.
type failingIngestFake struct {
	refreshFake
	failFirst int
}

func (f *failingIngestFake) IngestService(_ context.Context, _ string) error {
	f.ingests++
	if f.ingests <= f.failFirst {
		return errors.New("extract failed")
	}
	return nil
}

// TestEnsureSeenOnlyAfterSuccess pins RT-151 (design §7.1): a failed inline
// ingest does not record seen[service], so the next read retries it; once an
// ingest succeeds, later reads for the same head are debounced.
func TestEnsureSeenOnlyAfterSuccess(t *testing.T) {
	fake := &failingIngestFake{refreshFake: refreshFake{path: "/repo", indexed: "old"}, failFirst: 1}
	ar := newAR(t, fake, "new")
	now := time.Date(2026, 9, 26, 12, 0, 0, 0, time.UTC)
	ar.now = func() time.Time { return now }
	ctx := context.Background()

	ensureSettled(t, ar, ctx, "svc")
	if fake.ingests != 1 {
		t.Fatalf("ingests after first read = %d, want 1", fake.ingests)
	}
	ar.mu.Lock()
	_, recorded := ar.seen["svc"]
	ar.mu.Unlock()
	if recorded {
		t.Fatal("seen[svc] recorded after a failed ingest; the failure would never be retried")
	}

	now = now.Add(autoRefreshBackoffBase) // past the failure backoff
	ensureSettled(t, ar, ctx, "svc")
	if fake.ingests != 2 {
		t.Fatalf("ingests after second read = %d, want 2 (the failed ingest is retried)", fake.ingests)
	}
	ensureSettled(t, ar, ctx, "svc")
	if fake.ingests != 2 {
		t.Fatalf("ingests after third read = %d, want 2 (debounced after success)", fake.ingests)
	}
}

// TestEnsureJoinersServeStaleDuringInflightIngest pins the review fix to
// RT-149: only the read that starts an inline ingest waits for it (under its
// own ctx). Every other read of that service while the ingest runs — for the
// same head or a newer one — answers at once from the index as it stands, so
// a multi-minute ingest never parks one handler goroutine per concurrent read
// (the RPC ctx is still context.Background until RT-58).
func TestEnsureJoinersServeStaleDuringInflightIngest(t *testing.T) {
	var running, maxSeen int64
	release := make(chan struct{})
	started := make(chan string, 4)
	fa := &gatedIngestFake{refreshFake: refreshFake{path: "/a", indexed: "old"}, started: started, release: release, running: &running, maxSeen: &maxSeen}
	ar := newAR(t, fa, "new")
	var head atomic.Value
	head.Store("new")
	ar.headFn = func(context.Context, string) string { return head.Load().(string) }

	firstDone := make(chan struct{})
	go func() { ar.ensure(context.Background(), "svc"); close(firstDone) }()
	select {
	case <-started:
	case <-time.After(waitSafety):
		t.Fatal("ingest never started")
	}

	// The ingest stays held (release closes only after the checks) so a read
	// that waited on it would never come back on its own: every read must
	// return while the ingest is still in flight, and waitSafety is only the
	// hang guard that turns a read stuck behind it into a named failure.
	const readers = 20
	var wg sync.WaitGroup
	var returned atomic.Int32
	read := func() {
		defer wg.Done()
		ar.ensure(context.Background(), "svc")
		returned.Add(1)
	}
	for i := 0; i < readers; i++ {
		wg.Add(1)
		go read()
	}
	// A read that sees a newer head than the running ingest's also answers at once.
	head.Store("newer")
	wg.Add(1)
	go read()

	allReturned := make(chan struct{})
	go func() { wg.Wait(); close(allReturned) }()
	select {
	case <-allReturned:
	case <-time.After(waitSafety):
		got := readers + 1 - int(returned.Load())
		close(release)
		<-allReturned
		t.Fatalf("%d of %d concurrent reads blocked on another read's in-flight ingest; want 0 (serve stale)", got, readers+1)
	}
	if n := len(started); n != 0 {
		close(release)
		wg.Wait()
		t.Fatalf("%d more ingests started while one was in flight; want 0 (one per service at a time)", n)
	}
	close(release)
	wg.Wait()
	<-firstDone
}

// TestEnsureBacksOffAFailingIngest pins the review fix to RT-151: a failed
// inline ingest is retried, but not by every read — a (service, head) that
// failed waits 30 s, then 60 s, 120 s … (capped) before the next attempt, so
// a persistent failure (the TCC wedge that fails every service at once)
// cannot turn hook traffic into a continuous serialized walk loop.
func TestEnsureBacksOffAFailingIngest(t *testing.T) {
	fake := &failingIngestFake{refreshFake: refreshFake{path: "/repo", indexed: "old"}, failFirst: 1 << 30}
	ar := newAR(t, fake, "new")
	now := time.Date(2026, 9, 26, 12, 0, 0, 0, time.UTC)
	ar.now = func() time.Time { return now }
	ctx := context.Background()

	for i := 0; i < 50; i++ {
		ensureSettled(t, ar, ctx, "svc")
	}
	if fake.ingests != 1 {
		t.Fatalf("50 reads inside the backoff -> %d ingest attempts, want 1", fake.ingests)
	}
	now = now.Add(29 * time.Second)
	ensureSettled(t, ar, ctx, "svc")
	if fake.ingests != 1 {
		t.Fatalf("attempts 29 s after a failure = %d, want 1 (backoff 30 s)", fake.ingests)
	}
	now = now.Add(time.Second)
	ensureSettled(t, ar, ctx, "svc")
	if fake.ingests != 2 {
		t.Fatalf("attempts 30 s after a failure = %d, want 2 (retried after the backoff)", fake.ingests)
	}
	now = now.Add(59 * time.Second)
	ensureSettled(t, ar, ctx, "svc")
	if fake.ingests != 2 {
		t.Fatalf("attempts 59 s after the second failure = %d, want 2 (backoff doubles to 60 s)", fake.ingests)
	}
	now = now.Add(time.Second)
	ensureSettled(t, ar, ctx, "svc")
	if fake.ingests != 3 {
		t.Fatalf("attempts 60 s after the second failure = %d, want 3", fake.ingests)
	}
	if got := ar.AutoRefreshTotal(); got != 3 {
		t.Fatalf("refreshTotal = %d, want 3 (attempts, not reads)", got)
	}
	// A new head is a new drift: it is attempted at once.
	ar.headFn = func(context.Context, string) string { return "newer" }
	ensureSettled(t, ar, ctx, "svc")
	if fake.ingests != 4 {
		t.Fatalf("attempts after HEAD moved = %d, want 4 (a new drift is tried at once)", fake.ingests)
	}
	// Capped: after many failures the wait never exceeds autoRefreshBackoffMax.
	for i := 0; i < 20; i++ {
		now = now.Add(autoRefreshBackoffMax)
		ensureSettled(t, ar, ctx, "svc")
	}
	if fake.ingests != 24 {
		t.Fatalf("attempts at one per cap interval = %d, want 24", fake.ingests)
	}
}
