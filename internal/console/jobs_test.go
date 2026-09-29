package console

import (
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// TestJobs_BoundedSingleFlightDropsSuperseded (RT-117, design §14): the
// console's background work runs in one pool — at most Workers at once, at
// most Queue waiting — with one flight per key (a key queued or running is
// joined, never run twice), a queued job superseded by a newer key of the
// same group dropped before it runs, a full queue refused by name, and no
// goroutine left once the queue drains.
func TestJobs_BoundedSingleFlightDropsSuperseded(t *testing.T) {
	j := NewJobs(2, 3)
	release := make(chan struct{})
	var running, peak, ran atomic.Int32
	var mu sync.Mutex
	order := []string{}
	work := func(name string) func() {
		return func() {
			n := running.Add(1)
			for {
				p := peak.Load()
				if n <= p || peak.CompareAndSwap(p, n) {
					break
				}
			}
			<-release
			running.Add(-1)
			ran.Add(1)
			mu.Lock()
			order = append(order, name)
			mu.Unlock()
		}
	}
	a, err := j.SubmitJob("plan:a", "a@1", work("a@1"))
	if err != nil {
		t.Fatal(err)
	}
	b, _ := j.SubmitJob("plan:b", "b@1", work("b@1"))
	waitFor(t, func() bool { return running.Load() == 2 })
	// Single flight: the running key is joined.
	again, err := j.SubmitJob("plan:a", "a@1", work("a@1 twice"))
	if err != nil || again != a {
		t.Fatalf("a running key must be joined: %v, %v", again, err)
	}
	// Two queued, then a newer key of c's group supersedes c@1.
	c1, _ := j.SubmitJob("plan:c", "c@1", work("c@1"))
	d, _ := j.SubmitJob("plan:d", "d@1", work("d@1"))
	c2, err := j.SubmitJob("plan:c", "c@2", work("c@2"))
	if err != nil {
		t.Fatal(err)
	}
	select {
	case <-c1.Done():
	case <-time.After(waitSafety):
		t.Fatal("a superseded queued job must finish (dropped) at once")
	}
	if !c1.Dropped() || c2.Dropped() {
		t.Fatalf("c@1 dropped=%v, c@2 dropped=%v", c1.Dropped(), c2.Dropped())
	}
	// A queued key is joined too.
	if dd, _ := j.SubmitJob("plan:d", "d@1", work("d@1 twice")); dd != d {
		t.Fatal("a queued key must be joined")
	}
	// The queue holds d@1, c@2 and one more; the next is refused by name.
	if _, err := j.SubmitJob("plan:e", "e@1", work("e@1")); err != nil {
		t.Fatal(err)
	}
	if _, err := j.SubmitJob("plan:f", "f@1", work("f@1")); !errors.Is(err, ErrJobsFull) {
		t.Fatalf("a full queue = %v, want ErrJobsFull", err)
	}
	if st := j.Stats(); st.Running != 2 || st.Queued != 3 {
		t.Fatalf("stats = %+v", st)
	}
	close(release)
	for _, job := range []*Job{a, b, c2, d} {
		select {
		case <-job.Done():
		case <-time.After(waitSafety):
			t.Fatalf("job %s never finished", job.Key)
		}
	}
	waitFor(t, func() bool { st := j.Stats(); return st.Running == 0 && st.Queued == 0 && st.Workers == 0 })
	if p := peak.Load(); p > 2 {
		t.Errorf("peak concurrency %d > 2 workers", p)
	}
	if n := ran.Load(); n != 5 {
		t.Errorf("%d jobs ran (%v); want a, b, d, c@2, e — once each", n, order)
	}
	for _, name := range order {
		if name == "c@1" || name == "a@1 twice" || name == "d@1 twice" {
			t.Errorf("%s ran", name)
		}
	}
	// A panicking job finishes (with its error) and frees its worker.
	p, _ := j.SubmitJob("plan:p", "p@1", func() { panic("boom") })
	<-p.Done()
	if p.Err() == nil {
		t.Error("a panicking job must report its panic")
	}
	// A finished key runs again when submitted again (single flight is
	// per flight, not a cache).
	a2, _ := j.SubmitJob("plan:a", "a@1", func() {})
	<-a2.Done()
	if a2 == a {
		t.Error("a finished key must start a new flight")
	}
}

func waitFor(t *testing.T, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(waitSafety)
	for !cond() {
		if time.Now().After(deadline) {
			t.Fatal("condition not reached")
		}
		time.Sleep(5 * time.Millisecond)
	}
}
