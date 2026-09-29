package tenancy

import (
	"testing"
	"time"
)

// waitSafety bounds a wait a test makes on progress: a channel a goroutine
// closes, a state a poll loop reads, a process that must exit. It is a hang
// guard sized for a machine starved for a minute (parallel gates under
// -race), never a latency the code under test is held to; the assertion is
// the progress itself, and a bound of seconds turns a slow scheduler into a
// red gate (EW-7).
const waitSafety = time.Minute

// returnsUnaided runs fn and fails the test unless it returns by itself,
// within waitSafety. Use it where the claim is "this call does not wait for
// X": X stays held (the test releases it only afterwards), so a return
// proves the call never waited, and the bound is nothing but a hang guard
// that turns a wedge into a named failure. fn must not call t.Fatal (it runs
// on its own goroutine).
func returnsUnaided(t testing.TB, what string, fn func()) {
	t.Helper()
	done := make(chan struct{})
	go func() { defer close(done); fn() }()
	select {
	case <-done:
	case <-time.After(waitSafety):
		t.Fatalf("%s: still waiting after %s although nothing was released for it", what, waitSafety)
	}
}
