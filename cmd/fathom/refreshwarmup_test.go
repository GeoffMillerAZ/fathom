package main

import (
	"testing"
	"time"

	"github.com/geoffmilleraz/fathom/internal/tenancy"
)

// TestManagedStartOpensRefreshWarmup pins RT-150's wiring onto RT-40's
// window (design §2.3: warm-up "after any start"): the one warm-up window
// serve-http opens at every start (tenancy.SetWarmup) also defers
// auto-refresh, for exactly its length, and ResetLifecycleState (deferred by
// serve-http) closes it.
func TestManagedStartOpensRefreshWarmup(t *testing.T) {
	tenancy.ResetLifecycleStateForTest()
	t.Cleanup(tenancy.ResetLifecycleStateForTest)
	now := time.Date(2026, 9, 26, 21, 0, 0, 0, time.UTC)
	if got := tenancy.RefreshDeferralNow("svc", now.Add(time.Second)); got != "" {
		t.Fatalf("no start yet, but auto-refresh deferred: %q", got)
	}
	tenancy.SetWarmup(now, 2*time.Minute)
	if !tenancy.InWarmup(now.Add(119 * time.Second)) {
		t.Fatal("a start opened no warm-up window")
	}
	if got := tenancy.RefreshDeferralNow("svc", now.Add(time.Minute)); got == "" {
		t.Fatal("auto-refresh is not deferred during warm-up")
	}
	if got := tenancy.RefreshDeferralNow("svc", now.Add(120*time.Second)); got != "" {
		t.Fatalf("warm-up outlived its 120 s: %q", got)
	}
	tenancy.ResetLifecycleState()
	if tenancy.InWarmup(now.Add(time.Second)) {
		t.Fatal("serve-http's return left the warm-up window open")
	}
}
