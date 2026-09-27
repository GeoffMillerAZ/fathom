//go:build !(darwin && cgo)

package main

// host_other.go — the governor's host reader off macOS (design §2): the
// footprint is the resident set (Linux /proc/self/status), and the child
// tree, pressure, swap and volume readings are Unknown — disclosed, never
// a trigger (RT-32). PSI, cgroup budgets and a systemd unit wait for a
// Linux adopter (RT-D8). An unknown footprint (no reader at all) turns the
// ladder off, disclosed.

import "github.com/geoffmilleraz/fathom/internal/tenancy"

type otherHost struct{}

func newTenancyHost(string) tenancy.Host { return otherHost{} }

func (otherHost) ReadHost(s *tenancy.Sample) {
	if rss, ok := processResidentMemoryBytes(); ok {
		s.Footprint = rss
	}
	s.Pressure = tenancy.PressureUnknown
}

func childTreeFootprint(int) (int64, int, bool) { return tenancy.Unknown, 0, false }

func killChildTree(int) int { return 0 }

func hostRAM() int64 { return 0 }
