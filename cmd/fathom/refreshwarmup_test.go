package main

import (
	"testing"
	"time"

	"github.com/geoffmilleraz/fathom/internal/tenancy"
)

// TestManagedStartOpensRefreshWarmup pins RT-150's wiring (design §2.3): a
// managed serve-http opens auto-refresh's warm-up window at start and
// closes it when it returns; a scratch daemon opens none.
func TestManagedStartOpensRefreshWarmup(t *testing.T) {
	now := time.Date(2026, 9, 26, 21, 0, 0, 0, time.UTC)
	stop := startRefreshWarmup(false, 2*time.Minute, now, nil)
	if in, _ := tenancy.InWarmup(now.Add(time.Second)); in {
		t.Fatal("an unmanaged start opened a warm-up window")
	}
	stop()
	stop = startRefreshWarmup(true, 2*time.Minute, now, nil)
	if in, _ := tenancy.InWarmup(now.Add(119 * time.Second)); !in {
		t.Fatal("a managed start opened no warm-up window")
	}
	if got := tenancy.RefreshDeferralNow("svc", now.Add(time.Minute)); got == "" {
		t.Fatal("auto-refresh is not deferred during warm-up")
	}
	if in, _ := tenancy.InWarmup(now.Add(120 * time.Second)); in {
		t.Fatal("warm-up outlived its 120 s")
	}
	stop()
	if in, _ := tenancy.InWarmup(now.Add(time.Second)); in {
		t.Fatal("serve-http's return left the warm-up window open")
	}
}
