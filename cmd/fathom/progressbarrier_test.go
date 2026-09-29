package main

import (
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// barrierSafety is the bound a progressBarrier waits for its count before it
// gives up: a hang guard sized for a machine that is starved for a minute,
// never a latency the code under test is held to (EW-2, EW-7).
const barrierSafety = time.Minute

// progressBarrier holds each of want arrivals until all of them have
// arrived, so a test proves "these run together" from a count of progress,
// not from a timer racing the scheduler. Only if the count never completes
// (the code under test is sequential, or deadlocked) does the safety bound
// release the waiters; Stalled then says so and Describe says how far the
// count got.
type progressBarrier struct {
	want    int32
	safety  time.Duration
	arrived atomic.Int32
	stalled atomic.Bool
	atStall atomic.Int32 // the count when the safety bound fired
	once    sync.Once
	all     chan struct{}
}

func newProgressBarrier(want int, safety time.Duration) *progressBarrier {
	return &progressBarrier{want: int32(want), safety: safety, all: make(chan struct{})}
}

func (b *progressBarrier) release() { b.once.Do(func() { close(b.all) }) }

// Arrive counts one arrival and blocks until the count is met. It reports
// whether the count was met (false: the safety bound released it).
func (b *progressBarrier) Arrive() bool {
	if b.arrived.Add(1) >= b.want {
		b.release()
	}
	timer := time.NewTimer(b.safety)
	defer timer.Stop()
	select {
	case <-b.all:
		return !b.stalled.Load()
	case <-timer.C:
		if b.stalled.CompareAndSwap(false, true) {
			b.atStall.Store(b.arrived.Load())
		}
		b.release() // let the rest through; the test fails once, not per waiter
		return false
	}
}

func (b *progressBarrier) Arrived() int  { return int(b.arrived.Load()) }
func (b *progressBarrier) Stalled() bool { return b.stalled.Load() }

// Describe names how far the count got, for a failure message.
func (b *progressBarrier) Describe() string {
	n := b.Arrived()
	if b.Stalled() {
		n = int(b.atStall.Load())
	}
	return fmt.Sprintf("%d of %d arrivals before the %s safety bound", n, b.want, b.safety)
}

// TestProgressBarrierReleasesOnTheCountNotATimer: want arrivals, staggered
// however the scheduler likes, all pass once the last one arrives, and the
// safety bound plays no part however late the last one is.
func TestProgressBarrierReleasesOnTheCountNotATimer(t *testing.T) {
	const want = 4
	b := newProgressBarrier(want, barrierSafety)
	released := make(chan bool, want)
	var wg sync.WaitGroup
	for i := 0; i < want; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			// Arrivals trail in one after another; none may pass early.
			time.Sleep(time.Duration(i) * 5 * time.Millisecond)
			released <- b.Arrive()
		}()
	}
	wg.Wait()
	close(released)
	for ok := range released {
		if !ok {
			t.Fatal("an arrival was released by the safety bound, want the count")
		}
	}
	if b.Stalled() || b.Arrived() != want {
		t.Fatalf("stalled=%v arrived=%d, want the barrier met at %d", b.Stalled(), b.Arrived(), want)
	}
}

// TestProgressBarrierSafetyBoundNamesTheStall: a count that never completes
// (a sequential chain) is released by the safety bound only, and the stall
// says how far the count got. The bound is small here on purpose: the count
// can never reach want, so the bound firing is not a timing assertion, and
// no timing decides which count the stall reports beyond "short of want".
func TestProgressBarrierSafetyBoundNamesTheStall(t *testing.T) {
	b := newProgressBarrier(4, 20*time.Millisecond)
	passed := make(chan bool, 3)
	var wg sync.WaitGroup
	for i := 0; i < 3; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			passed <- b.Arrive()
		}()
	}
	wg.Wait()
	close(passed)
	for ok := range passed {
		if ok {
			t.Fatal("an arrival passed although the count of 4 was never met")
		}
	}
	if !b.Stalled() {
		t.Fatal("Stalled = false after the safety bound released the arrivals")
	}
	// How far the count got when the bound fired depends on how quickly the
	// three goroutines started (1 to 3 of 4), so only "short of 4" is pinned.
	if got := b.Describe(); !strings.Contains(got, " of 4 arrivals") || strings.HasPrefix(got, "4 ") {
		t.Errorf("Describe() = %q, want it to name a count short of 4", got)
	}
}
