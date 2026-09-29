package main

import (
	"testing"
	"time"
)

// waitUntil polls cond until it holds, failing the test after waitSafety (a hang
// guard, not a latency bound: the assertion is the state cond reads).
func waitUntil(t *testing.T, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(waitSafety)
	for !cond() {
		if time.Now().After(deadline) {
			t.Fatal("condition never held")
		}
		time.Sleep(time.Millisecond)
	}
}
