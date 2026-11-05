package readlimit

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"
)

func TestRunReturnsTheResult(t *testing.T) {
	var g Group
	v, out, err := Run(context.Background(), &g, time.Second, func() (int, error) { return 7, nil })
	if v != 7 || out != Finished || err != nil || g.Abandoned() != 0 {
		t.Fatalf("got %d %v %v abandoned=%d", v, out, err, g.Abandoned())
	}
	want := errors.New("boom")
	_, out, err = Run(context.Background(), nil, 0, func() (int, error) { return 0, want })
	if out != Finished || !errors.Is(err, want) {
		t.Fatalf("a function's error comes back: %v %v", out, err)
	}
}

func TestRunRecoversAPanic(t *testing.T) {
	var g Group
	_, out, err := Run(context.Background(), &g, time.Second, func() (int, error) { panic("bad") })
	var pe *PanicError
	if out != Finished || !errors.As(err, &pe) || g.Abandoned() != 0 {
		t.Fatalf("a panic is the function's error: %v %v abandoned=%d", out, err, g.Abandoned())
	}
}

func TestDeadlineAbandonsAndCountsUntilItReturns(t *testing.T) {
	var g Group
	release := make(chan struct{})
	started := make(chan struct{})
	v, out, err := Run(context.Background(), &g, 30*time.Millisecond, func() (int, error) {
		close(started)
		<-release
		return 9, nil
	})
	<-started
	if out != DeadlineExceeded || v != 0 || err != nil {
		t.Fatalf("got %d %v %v", v, out, err)
	}
	if g.Abandoned() != 1 || !g.Saturated(1) || g.Saturated(2) {
		t.Fatalf("abandoned=%d", g.Abandoned())
	}
	close(release)
	waitFor(t, func() bool { return g.Abandoned() == 0 })
}

func TestCancelAbandons(t *testing.T) {
	var g Group
	ctx, cancel := context.WithCancel(context.Background())
	release := make(chan struct{})
	defer close(release)
	go func() { time.Sleep(20 * time.Millisecond); cancel() }()
	_, out, err := Run(ctx, &g, time.Minute, func() (int, error) { <-release; return 1, nil })
	if out != Canceled || !errors.Is(err, context.Canceled) || g.Abandoned() != 1 {
		t.Fatalf("got %v %v abandoned=%d", out, err, g.Abandoned())
	}
}

// Many Runs that race their own deadline: the count must settle to zero.
func TestTheCountSettlesUnderRacingDeadlines(t *testing.T) {
	var g Group
	var wg sync.WaitGroup
	for i := 0; i < 200; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			Run(context.Background(), &g, time.Duration(i%5)*time.Millisecond+time.Microsecond, func() (int, error) {
				time.Sleep(time.Duration(i%5) * time.Millisecond)
				return i, nil
			})
		}(i)
	}
	wg.Wait()
	waitFor(t, func() bool { return g.Abandoned() == 0 })
}

func TestNoDeadlineWaits(t *testing.T) {
	v, out, _ := Run(context.Background(), nil, 0, func() (int, error) { time.Sleep(30 * time.Millisecond); return 3, nil })
	if v != 3 || out != Finished {
		t.Fatalf("got %d %v", v, out)
	}
}

func waitFor(t *testing.T, ok func() bool) {
	t.Helper()
	for i := 0; i < 200; i++ {
		if ok() {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("condition not reached")
}
