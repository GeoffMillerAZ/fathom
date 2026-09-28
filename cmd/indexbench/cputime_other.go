//go:build !unix

package main

import "time"

// processCPU is unavailable here: 0 makes the reading fall back to wall
// clock, and it says so (throughput_basis "wall").
func processCPU() time.Duration { return 0 }
