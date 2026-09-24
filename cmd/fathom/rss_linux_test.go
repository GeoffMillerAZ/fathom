//go:build linux

package main

import "testing"

func TestProcessResidentMemoryBytes_ReturnsPositiveValue(t *testing.T) {
	v, ok := processResidentMemoryBytes()
	if !ok {
		t.Fatal("processResidentMemoryBytes() ok=false on linux; /proc/self/status should always answer for this process")
	}
	const hundredGiB = 100 << 30
	if v <= 0 || v >= hundredGiB {
		t.Errorf("processResidentMemoryBytes() = %d, want a plausible positive value (0, 100GiB)", v)
	}
}

func TestProcessPhysFootprintBytes_AlwaysUnavailableOnLinux(t *testing.T) {
	if _, ok := processPhysFootprintBytes(); ok {
		t.Error("processPhysFootprintBytes() ok=true on linux; there is no distinct footprint concept to report there")
	}
}
