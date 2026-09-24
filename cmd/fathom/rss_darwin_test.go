//go:build darwin && cgo

package main

import "testing"

// TestProcessResidentMemoryBytes_ReturnsPositiveValue is DG-16's plan-row
// contract test: proc_pid_rusage must answer with a positive, plausible
// value for this live process — never a zero standing in for "unknown."
func TestProcessResidentMemoryBytes_ReturnsPositiveValue(t *testing.T) {
	v, ok := processResidentMemoryBytes()
	if !ok {
		t.Fatal("processResidentMemoryBytes() ok=false on darwin+cgo; proc_pid_rusage should always answer for this process")
	}
	const hundredGiB = 100 << 30
	if v <= 0 || v >= hundredGiB {
		t.Errorf("processResidentMemoryBytes() = %d, want a plausible positive value (0, 100GiB)", v)
	}
}

func TestProcessPhysFootprintBytes_ReturnsPositiveValue(t *testing.T) {
	v, ok := processPhysFootprintBytes()
	if !ok {
		t.Fatal("processPhysFootprintBytes() ok=false on darwin+cgo; proc_pid_rusage should always answer for this process")
	}
	const hundredGiB = 100 << 30
	if v <= 0 || v >= hundredGiB {
		t.Errorf("processPhysFootprintBytes() = %d, want a plausible positive value (0, 100GiB)", v)
	}
}
