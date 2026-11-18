package depgraph

import (
	"testing"
	"time"
)

// timeoutC is the cycle test's watchdog. Five seconds is far beyond any
// honest answer over a handful of units, so a firing timer means the walk
// did not terminate rather than that the machine was busy.
func timeoutC(t *testing.T) <-chan time.Time {
	t.Helper()
	return time.After(5 * time.Second)
}
