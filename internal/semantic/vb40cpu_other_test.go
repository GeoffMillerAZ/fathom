//go:build !unix

package semantic

import "time"

// processCPU is unmeasured off unix: RS-M2's latency reading runs on unix.
func processCPU() time.Duration { return 0 }
