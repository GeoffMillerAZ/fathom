package scenariobench

import (
	"errors"
	"sync"
	"testing"
	"time"
)

// TestRunArmsConcurrently_PreservesOrderNotCompletionOrder is RC-16's own
// proof: the slower function is given first, so a completion-order result
// would misplace it — RunArmsConcurrently must not.
func TestRunArmsConcurrently_PreservesOrderNotCompletionOrder(t *testing.T) {
	fns := []func() (string, error){
		func() (string, error) { time.Sleep(30 * time.Millisecond); return "slow", nil },
		func() (string, error) { return "fast", nil },
	}
	results, errs := RunArmsConcurrently(fns)
	if len(results) != 2 || results[0] != "slow" || results[1] != "fast" {
		t.Fatalf("results = %v, want [slow fast] (index order, not completion order)", results)
	}
	for i, err := range errs {
		if err != nil {
			t.Errorf("errs[%d] = %v, want nil", i, err)
		}
	}
}

// TestRunArmsConcurrently_ReportsEachErrorAtItsOwnIndex covers a mixed
// batch: one function fails, the other succeeds, and each result lands at
// its own index regardless.
func TestRunArmsConcurrently_ReportsEachErrorAtItsOwnIndex(t *testing.T) {
	boom := errors.New("boom")
	fns := []func() (int, error){
		func() (int, error) { return 0, boom },
		func() (int, error) { return 7, nil },
	}
	results, errs := RunArmsConcurrently(fns)
	if errs[0] != boom {
		t.Errorf("errs[0] = %v, want boom", errs[0])
	}
	if errs[1] != nil {
		t.Errorf("errs[1] = %v, want nil", errs[1])
	}
	if results[1] != 7 {
		t.Errorf("results[1] = %d, want 7", results[1])
	}
}

// TestRunArmsConcurrently_ActuallyRunsConcurrently proves the "concurrent"
// half of the name: two functions that each block until the other has
// started must both complete — a sequential caller would deadlock.
func TestRunArmsConcurrently_ActuallyRunsConcurrently(t *testing.T) {
	var wg sync.WaitGroup
	wg.Add(2)
	fns := []func() (struct{}, error){
		func() (struct{}, error) { wg.Done(); wg.Wait(); return struct{}{}, nil },
		func() (struct{}, error) { wg.Done(); wg.Wait(); return struct{}{}, nil },
	}
	done := make(chan struct{})
	go func() {
		RunArmsConcurrently(fns)
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(waitSafety):
		t.Fatal("RunArmsConcurrently did not run its functions concurrently (deadlocked)")
	}
}
