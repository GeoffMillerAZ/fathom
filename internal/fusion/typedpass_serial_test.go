package fusion

// typedpass_serial_test.go — white-box (package fusion, not fusion_test)
// tests for W-N's process-wide typed-pass semaphore (typedpass_serial.go,
// docs/plans/daemon-footprint.md): the second of two concurrent passes must
// wait rather than run alongside the first, the wait must be counted,
// context cancellation while waiting must return promptly, and the
// inflight gauge must return to 0 once every holder has released.

import (
	"context"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/store"
)

// typedPassFixtureDir returns the small, self-contained Go module (its own
// go.mod) that typedcalls_test.go (package fusion_test) also uses — that
// helper is unreachable from here, so this is its white-box twin.
func typedPassFixtureDir(t *testing.T) string {
	t.Helper()
	abs, err := filepath.Abs(filepath.Join("testdata", "typedfixture"))
	if err != nil {
		t.Fatalf("abs path: %v", err)
	}
	if _, statErr := os.Stat(filepath.Join(abs, "go.mod")); statErr != nil {
		t.Fatalf("typed fixture module missing go.mod at %s: %v", abs, statErr)
	}
	return abs
}

// TestAcquireTypedPassSlot_SerializesAndCounts is the semaphore-primitive
// half of Task 1's test: a second acquire that arrives while the first is
// still held must block until release, the wait must be counted (only the
// waiter counts — the first acquire took the uncontended fast path), and
// the inflight gauge must return to 0 once both sides have released.
func TestAcquireTypedPassSlot_SerializesAndCounts(t *testing.T) {
	waitedBefore := atomic.LoadInt64(&typedPassWaitedTotal)

	release1, err := acquireTypedPassSlot(context.Background())
	if err != nil {
		t.Fatalf("first acquire: %v", err)
	}
	if got := TypedPassInflight(); got != 1 {
		t.Fatalf("TypedPassInflight after first acquire = %d, want 1", got)
	}

	secondDone := make(chan struct{})
	go func() {
		release2, err := acquireTypedPassSlot(context.Background())
		if err != nil {
			t.Errorf("second acquire: %v", err)
			close(secondDone)
			return
		}
		close(secondDone)
		release2()
	}()

	// The second goroutine must not be able to acquire while the first
	// slot is still held — give it a real window to try and confirm it
	// blocks rather than merely racing ahead of the assertion below.
	select {
	case <-secondDone:
		t.Fatal("second acquire returned before the first was released — semaphore did not serialize")
	case <-time.After(100 * time.Millisecond):
	}

	release1()

	select {
	case <-secondDone:
	case <-time.After(2 * time.Second):
		t.Fatal("second acquire never completed after the first released")
	}

	if got := atomic.LoadInt64(&typedPassWaitedTotal) - waitedBefore; got != 1 {
		t.Errorf("typedPassWaitedTotal delta = %d, want 1 (exactly the contended acquire)", got)
	}

	deadline := time.Now().Add(time.Second)
	for TypedPassInflight() != 0 && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
	if got := TypedPassInflight(); got != 0 {
		t.Errorf("TypedPassInflight after both released = %d, want 0", got)
	}
}

// TestAcquireTypedPassSlot_CancelWhileWaiting proves ctx cancellation while
// blocked on a held slot returns ctx.Err() promptly instead of waiting for
// the holder to release — Task 1's "cancellation while waiting returns
// ctx.Err()" requirement — and that the cancelled waiter never acquired
// (the gauge stays owned by the original holder throughout).
func TestAcquireTypedPassSlot_CancelWhileWaiting(t *testing.T) {
	release1, err := acquireTypedPassSlot(context.Background())
	if err != nil {
		t.Fatalf("first acquire: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	result := make(chan error, 1)
	go func() {
		_, err := acquireTypedPassSlot(ctx)
		result <- err
	}()

	// acquireTypedPassSlot's own non-blocking fast-path attempt fails
	// immediately (the slot is held), so by the time this sleep elapses
	// the goroutine above is certainly parked in the blocking select —
	// cancelling before that would just as validly exercise the "ctx
	// already done" path, but this proves the WAITING case specifically.
	time.Sleep(20 * time.Millisecond)
	cancel()

	select {
	case err := <-result:
		if err != context.Canceled {
			t.Errorf("acquireTypedPassSlot after cancel = %v, want context.Canceled", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("acquireTypedPassSlot did not return after ctx cancellation")
	}

	if got := TypedPassInflight(); got != 1 {
		t.Errorf("TypedPassInflight while the first pass still holds the slot = %d, want 1 (the cancelled waiter must not have acquired)", got)
	}

	release1()
	if got := TypedPassInflight(); got != 0 {
		t.Errorf("TypedPassInflight after release = %d, want 0", got)
	}
}

// TestTypedPassSemaphore_SerializesConcurrentFixturePasses runs two real
// typed-edge passes over the small typedfixture module concurrently,
// each bracketed by the same acquire/release pair enrichTypedEdges uses
// in production, and proves their guarded windows never overlap — Task
// 1's "two concurrent passes on a small fixture ... the second waits
// (assert via the counter and ordering)" requirement. It composes the
// real acquireTypedPassSlot and the real EnrichTypedEdges rather than
// going through Pipeline.enrichTypedEdges, so the measured window brackets
// exactly the guarded section with no pipeline-side noise.
func TestTypedPassSemaphore_SerializesConcurrentFixturePasses(t *testing.T) {
	dir := typedPassFixtureDir(t)
	waitedBefore := atomic.LoadInt64(&typedPassWaitedTotal)

	type window struct{ start, end time.Time }
	runGuarded := func(serviceID string) (window, error) {
		release, acqErr := acquireTypedPassSlot(context.Background())
		if acqErr != nil {
			return window{}, acqErr
		}
		defer release()
		w := window{start: time.Now()}
		st := store.NewMemoryStore()
		_, err := EnrichTypedEdges(context.Background(), st, serviceID, dir, nil)
		w.end = time.Now()
		return w, err
	}

	ids := []string{"typedpass-serial-fixture-a", "typedpass-serial-fixture-b"}
	windows := make([]window, len(ids))
	errs := make([]error, len(ids))
	var wg sync.WaitGroup
	wg.Add(len(ids))
	for i, id := range ids {
		i, id := i, id
		go func() {
			defer wg.Done()
			w, err := runGuarded(id)
			windows[i], errs[i] = w, err
		}()
	}
	wg.Wait()

	for i, err := range errs {
		if err != nil {
			t.Fatalf("pass %d (%s): %v", i, ids[i], err)
		}
	}

	a, b := windows[0], windows[1]
	if a.start.Before(b.end) && b.start.Before(a.end) {
		t.Errorf("typed-edge passes overlapped despite the semaphore: a=[%s,%s] b=[%s,%s]",
			a.start.Format(time.RFC3339Nano), a.end.Format(time.RFC3339Nano),
			b.start.Format(time.RFC3339Nano), b.end.Format(time.RFC3339Nano))
	}

	if got := atomic.LoadInt64(&typedPassWaitedTotal) - waitedBefore; got != 1 {
		t.Errorf("typedPassWaitedTotal delta = %d, want 1 (exactly one of the two had to wait)", got)
	}

	if got := TypedPassInflight(); got != 0 {
		t.Errorf("TypedPassInflight after both passes finished = %d, want 0", got)
	}
}

// TestEnrichTypedEdges_ServiceSkipsSemaphoreGate is a guard against the
// early-return paths (DisableTypedEdges, no Go source, no go.mod) silently
// taking the semaphore and never releasing it — they must return before
// ever calling acquireTypedPassSlot.
func TestEnrichTypedEdges_ServiceSkipsSemaphoreGate(t *testing.T) {
	p := &Pipeline{st: store.NewMemoryStore(), cfg: Config{DisableTypedEdges: true}}
	svc := core.ServiceRef{ID: "typedpass-serial-skip", Path: typedPassFixtureDir(t)}
	refs := []core.SourceRef{{Service: svc.ID, Path: "direct.go"}}

	before := TypedPassInflight()
	rep, _ := p.enrichTypedEdges(context.Background(), svc, refs)
	if rep.PackagesTotal != 0 || rep.PackagesLoaded != 0 || rep.Duration != 0 {
		t.Errorf("enrichTypedEdges with DisableTypedEdges = %+v, want a zero-value report (the semaphore path never ran)", rep)
	}
	if got := TypedPassInflight(); got != before {
		t.Errorf("TypedPassInflight changed across a skipped (DisableTypedEdges) pass: before=%d after=%d", before, got)
	}
}
