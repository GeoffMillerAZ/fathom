package tenancy

import (
	"testing"
	"time"
)

// TestUnmanagedExitReasons (RT-21, §10.1): an unmanaged serve-http exits
// on idle, TTL, or an unlinked executable or store, naming the reason; an
// in-flight request (an /events subscriber included) holds idle off; a
// zero bound is disabled.
func TestUnmanagedExitReasons(t *testing.T) {
	t0 := time.Date(2026, 9, 26, 12, 0, 0, 0, time.UTC)
	b := DefaultUnmanagedBounds()
	if b.IdleExit != 20*time.Minute || b.TTL != 8*time.Hour {
		t.Fatalf("defaults = %+v, want idle 20m, ttl 8h", b)
	}
	cases := []struct {
		name string
		b    UnmanagedBounds
		s    UnmanagedState
		want string // "" = keep running
	}{
		{"fresh", b, UnmanagedState{Now: t0.Add(time.Minute), StartedAt: t0, LastActivity: t0}, ""},
		{"idle 20m", b, UnmanagedState{Now: t0.Add(21 * time.Minute), StartedAt: t0, LastActivity: t0}, "idle 20m0s"},
		{"idle held off by an in-flight request", b, UnmanagedState{Now: t0.Add(3 * time.Hour), StartedAt: t0, LastActivity: t0, InFlight: 1}, ""},
		{"recent activity", b, UnmanagedState{Now: t0.Add(3 * time.Hour), StartedAt: t0, LastActivity: t0.Add(3*time.Hour - time.Minute)}, ""},
		{"ttl beats a busy daemon", b, UnmanagedState{Now: t0.Add(8*time.Hour + time.Second), StartedAt: t0, LastActivity: t0.Add(8 * time.Hour), InFlight: 3}, "ttl 8h0m0s"},
		{"executable unlinked", b, UnmanagedState{Now: t0.Add(time.Minute), StartedAt: t0, LastActivity: t0, ExeUnlinked: true}, "executable unlinked"},
		{"store unlinked", b, UnmanagedState{Now: t0.Add(time.Minute), StartedAt: t0, LastActivity: t0, StoreUnlinked: true}, "store unlinked"},
		{"exe before store before ttl", b, UnmanagedState{Now: t0.Add(9 * time.Hour), StartedAt: t0, LastActivity: t0, ExeUnlinked: true, StoreUnlinked: true}, "executable unlinked"},
		{"idle disabled", UnmanagedBounds{TTL: 8 * time.Hour}, UnmanagedState{Now: t0.Add(7 * time.Hour), StartedAt: t0, LastActivity: t0}, ""},
		{"ttl disabled", UnmanagedBounds{IdleExit: 20 * time.Minute}, UnmanagedState{Now: t0.Add(100 * time.Hour), StartedAt: t0, LastActivity: t0.Add(100 * time.Hour)}, ""},
	}
	for _, c := range cases {
		reason, exit := EvaluateUnmanagedExit(c.b, c.s)
		if exit != (c.want != "") || reason != c.want {
			t.Errorf("%s: got (%q, %v), want %q", c.name, reason, exit, c.want)
		}
	}
}
