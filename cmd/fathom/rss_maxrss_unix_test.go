//go:build darwin || linux

package main

import "testing"

func TestProcessMaxResidentMemoryBytes_ReturnsPositiveValue(t *testing.T) {
	v, ok := processMaxResidentMemoryBytes()
	if !ok {
		t.Fatal("processMaxResidentMemoryBytes() ok=false; getrusage should always answer for this process")
	}
	const hundredGiB = 100 << 30
	if v <= 0 || v >= hundredGiB {
		t.Errorf("processMaxResidentMemoryBytes() = %d, want a plausible positive value (0, 100GiB)", v)
	}
}
