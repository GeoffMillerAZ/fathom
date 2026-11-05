// Package readlimit is the in-process read deadline: it runs a function in a
// goroutine and returns its result, or, when the deadline or the context ends
// first, returns "abandoned" while the goroutine runs on and its result is
// discarded.
//
// Go cannot interrupt a goroutine. An abandoned function keeps its CPU, and
// whatever it holds, until it returns on its own, so a Group counts the ones
// still running and a caller can stop starting more. The only complete answer
// to a function that never returns is a process boundary (cmd/fathom-lang).
//
// The package imports the standard library only, so the stand-alone reads
// (internal/iacfacts) and pkg/lang may both use it.
package readlimit

import (
	"context"
	"fmt"
	"sync/atomic"
	"time"
)

// Outcome says how a Run ended.
type Outcome int

const (
	// Finished: the function returned (or panicked) before the deadline.
	Finished Outcome = iota
	// DeadlineExceeded: the deadline passed first. The function is abandoned.
	DeadlineExceeded
	// Canceled: the context ended first. The function is abandoned.
	Canceled
)

// String names the outcome.
func (o Outcome) String() string {
	switch o {
	case Finished:
		return "finished"
	case DeadlineExceeded:
		return "deadline"
	case Canceled:
		return "canceled"
	}
	return "unknown"
}

// PanicError is the error Run returns for a function that panicked.
type PanicError struct{ Value any }

// Error describes the panic.
func (e *PanicError) Error() string { return fmt.Sprintf("panic: %v", e.Value) }

// Group counts the abandoned goroutines of the Runs that share it. The zero
// value is ready to use, and a Group is safe for concurrent use.
type Group struct {
	abandoned atomic.Int64
}

// Abandoned is how many goroutines a Run gave up on that have not returned yet.
func (g *Group) Abandoned() int {
	if g == nil {
		return 0
	}
	return int(g.abandoned.Load())
}

// Saturated reports whether max or more abandoned goroutines are still
// running. A caller that is saturated starts nothing more.
func (g *Group) Saturated(max int) bool { return g.Abandoned() >= max }

// Run calls fn in a goroutine and waits for it, for at most d (no limit when
// d is zero or negative) and until ctx ends. It returns fn's result and
// Finished, or the zero value and DeadlineExceeded or Canceled, in which case
// fn keeps running and its result is discarded. A panic in fn is recovered
// and returned as a *PanicError with the outcome Finished. g, which may be
// nil, counts the abandoned goroutines until they return.
func Run[T any](ctx context.Context, g *Group, d time.Duration, fn func() (T, error)) (T, Outcome, error) {
	type result struct {
		v   T
		err error
	}
	var zero T
	if ctx == nil {
		ctx = context.Background()
	}
	done := make(chan result, 1)
	// state is 0 while fn runs, 1 once it returned in time, 2 once Run gave up on it.
	// Whichever side moves it first wins, so the abandoned count is settled exactly once.
	var state atomic.Int32
	go func() {
		var r result
		defer func() {
			if p := recover(); p != nil {
				r = result{err: &PanicError{Value: p}}
			}
			if !state.CompareAndSwap(0, 1) && g != nil {
				g.abandoned.Add(-1) // Run gave up on this goroutine, and counted it
			}
			done <- r
		}()
		r.v, r.err = fn()
	}()
	abandon := func() (result, bool) {
		if g != nil {
			g.abandoned.Add(1)
		}
		if state.CompareAndSwap(0, 2) {
			return result{}, true
		}
		if g != nil {
			g.abandoned.Add(-1) // fn returned between the select and here
		}
		return <-done, false
	}
	var deadline <-chan time.Time
	if d > 0 {
		timer := time.NewTimer(d)
		defer timer.Stop()
		deadline = timer.C
	}
	select {
	case r := <-done:
		return r.v, Finished, r.err
	case <-deadline:
		if r, gaveUp := abandon(); !gaveUp {
			return r.v, Finished, r.err
		}
		return zero, DeadlineExceeded, nil
	case <-ctx.Done():
		if r, gaveUp := abandon(); !gaveUp {
			return r.v, Finished, r.err
		}
		return zero, Canceled, ctx.Err()
	}
}
