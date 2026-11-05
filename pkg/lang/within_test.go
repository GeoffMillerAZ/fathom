package lang_test

import (
	"testing"
	"time"
)

// within runs f and fails the test when it has not returned by d. A quadratic
// reader does not stop at the deadline, but the test that finds it does.
func within(t *testing.T, d time.Duration, what string, f func()) {
	t.Helper()
	done := make(chan struct{})
	go func() { defer close(done); f() }()
	select {
	case <-done:
	case <-time.After(d):
		t.Fatalf("%s did not finish in %v", what, d)
	}
}
