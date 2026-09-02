package main

import "testing"

// TestRunA reaches cmd/a's run — and must never be credited to cmd/b's.
func TestRunA(t *testing.T) {
	run()
}
