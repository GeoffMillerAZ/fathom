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

// TestEnsureIngestSurvivesRequestCancelAndSerializes pins RT-149 (design
// §3.5, §7.1): ensure()'s inline ingest runs detached from its requester
// (context.WithoutCancel), so a caller that disconnects mid-ingest returns at
// once while the ingest completes uncancelled; and it holds one process-wide
// ingest ticket, so two services (here on two separate backends) never ingest
// inline at the same time.
func TestEnsureIngestSurvivesRequestCancelAndSerializes(t *testing.T) {
	var running, maxSeen int64
	release := make(chan struct{})
	started := make(chan string, 4)
	fa := &gatedIngestFake{refreshFake: refreshFake{path: "/a", indexed: "old"}, started: started, release: release, running: &running, maxSeen: &maxSeen}
	fb := &gatedIngestFake{refreshFake: refreshFake{path: "/b", indexed: "old"}, started: started, release: release, running: &running, maxSeen: &maxSeen}
	arA := newAR(fa, "new")
	arB := newAR(fb, "new")

	// Requester A: its ctx is cancelled while the ingest is running.
	ctxA, cancelA := context.WithCancel(context.Background())
	doneA := make(chan struct{})
	go func() { arA.ensure(ctxA, "svc-a"); close(doneA) }()
	select {
	case <-started:
	case <-time.After(5 * time.Second):
		t.Fatal("ingest for svc-a never started")
	}

	// Requester B asks for a different service while A's ingest holds the ticket.
	ctxB, cancelB := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancelB()
	doneB := make(chan struct{})
	go func() { arB.ensure(ctxB, "svc-b"); close(doneB) }()

	cancelA()
	select {
	case <-doneA:
	case <-time.After(5 * time.Second):
		t.Fatal("ensure did not return after its requester's ctx was cancelled")
	}
	select {
	case <-doneB:
	case <-time.After(5 * time.Second):
		t.Fatal("ensure for svc-b did not return when its ctx expired while waiting for the ticket")
	}
	if got := atomic.LoadInt64(&running); got != 1 {
		t.Fatalf("ingests running while svc-a holds the ticket = %d, want 1", got)
	}

	close(release)
	// svc-b's ingest was queued behind the ticket; it runs after svc-a's.
	select {
	case <-started:
	case <-time.After(5 * time.Second):
		t.Fatal("queued ingest for svc-b never started after the ticket was released")
	}
	deadline := time.Now().Add(5 * time.Second)
	for {
		fa.mu.Lock()
		fb.mu.Lock()
		done := fa.finished == 1 && fb.finished == 1
		fb.mu.Unlock()
		fa.mu.Unlock()
		if done {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("ingests did not finish")
		}
		time.Sleep(5 * time.Millisecond)
	}
	if got := atomic.LoadInt64(&maxSeen); got != 1 {
		t.Fatalf("peak concurrent inline ingests = %d, want 1 (one process-wide ticket)", got)
	}
	fa.mu.Lock()
	defer fa.mu.Unlock()
	if err := fa.ctxErrs[0]; err != nil {
		t.Fatalf("svc-a's ingest saw ctx error %v after its requester left, want nil (detached)", err)
	}
	fb.mu.Lock()
	defer fb.mu.Unlock()
	if err := fb.ctxErrs[0]; err != nil {
		t.Fatalf("svc-b's ingest saw ctx error %v, want nil (detached)", err)
	}
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
	ar := newAR(fake, "new")
	ctx := context.Background()

	ar.ensure(ctx, "svc")
	if fake.ingests != 1 {
		t.Fatalf("ingests after first read = %d, want 1", fake.ingests)
	}
	ar.mu.Lock()
	_, recorded := ar.seen["svc"]
	ar.mu.Unlock()
	if recorded {
		t.Fatal("seen[svc] recorded after a failed ingest; the failure would never be retried")
	}

	ar.ensure(ctx, "svc")
	if fake.ingests != 2 {
		t.Fatalf("ingests after second read = %d, want 2 (the failed ingest is retried)", fake.ingests)
	}
	ar.ensure(ctx, "svc")
	if fake.ingests != 2 {
		t.Fatalf("ingests after third read = %d, want 2 (debounced after success)", fake.ingests)
	}
}
