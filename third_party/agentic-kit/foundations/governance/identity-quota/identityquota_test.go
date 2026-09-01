package identityquota

import (
	"fmt"
	"testing"
	"time"
)

// clock is a controllable time source; every test drives time explicitly so
// nothing depends on wall-clock behavior.
type clock struct{ t time.Time }

func (c *clock) now() time.Time      { return c.t }
func (c *clock) add(d time.Duration) { c.t = c.t.Add(d) }

func testCfg() Config {
	return Config{MaxReqPerDay: 3, MaxUSDPerDay: 0.10, MaxIdentities: 4, IdentityTTL: 48 * time.Hour}
}

func TestConfigValidateRejectsDangerousZeroValues(t *testing.T) {
	// Every zero here denies all traffic or unbounds memory; construction is
	// the right place to find that out.
	cases := map[string]Config{
		"zero requests":   {MaxUSDPerDay: 1, MaxIdentities: 1, IdentityTTL: time.Hour},
		"zero usd":        {MaxReqPerDay: 1, MaxIdentities: 1, IdentityTTL: time.Hour},
		"zero identities": {MaxReqPerDay: 1, MaxUSDPerDay: 1, IdentityTTL: time.Hour},
		"zero ttl":        {MaxReqPerDay: 1, MaxUSDPerDay: 1, MaxIdentities: 1},
	}
	for name, cfg := range cases {
		if err := cfg.Validate(); err == nil {
			t.Errorf("%s: expected Validate to reject", name)
		}
	}
	if err := testCfg().Validate(); err != nil {
		t.Errorf("valid config rejected: %v", err)
	}
}

func TestOneIdentityCannotExhaustEveryoneElse(t *testing.T) {
	// The attack this package exists to stop: with only an aggregate budget,
	// one caller drains the shared daily pool and the endpoint goes dark for
	// everybody. Here the attacker hits their own ceiling and a fresh visitor
	// is still served.
	c := &clock{t: time.Date(2026, 7, 30, 12, 0, 0, 0, time.UTC)}
	g := New(testCfg(), c.now)

	for i := 0; i < 3; i++ {
		if d := g.Charge("attacker", 0.004); !d.Allowed {
			t.Fatalf("attacker request %d should be allowed: %s", i+1, d.Reason)
		}
	}
	d := g.Charge("attacker", 0.004)
	if d.Allowed {
		t.Fatal("attacker exceeded the per-identity daily quota but was allowed")
	}
	if d.Reason == "" {
		t.Error("a denial must explain itself")
	}
	if d := g.Charge("recruiter", 0.004); !d.Allowed {
		t.Fatalf("an unrelated visitor must still be served: %s", d.Reason)
	}
}

func TestSpendQuotaIsSeparateFromRequestQuota(t *testing.T) {
	c := &clock{t: time.Now()}
	g := New(Config{MaxReqPerDay: 100, MaxUSDPerDay: 0.01, MaxIdentities: 4, IdentityTTL: time.Hour}, c.now)
	if d := g.Charge("x", 0.009); !d.Allowed {
		t.Fatal("first charge should fit")
	}
	if d := g.Charge("x", 0.009); d.Allowed {
		t.Fatal("second charge exceeds the USD ceiling and must be denied")
	}
}

func TestDeniedChargeRecordsNothing(t *testing.T) {
	// A denial must not consume quota, or a caller could be punished twice for
	// one refused request.
	c := &clock{t: time.Now()}
	g := New(Config{MaxReqPerDay: 1, MaxUSDPerDay: 1, MaxIdentities: 2, IdentityTTL: time.Hour}, c.now)
	g.Charge("x", 0.1)
	g.Charge("x", 0.1) // denied
	snap := g.Snapshot()
	if got := snap.Entries[0].ReqThisDay; got != 1 {
		t.Errorf("denied charge was recorded: reqThisDay=%d, want 1", got)
	}
}

func TestQuotaResetsAfterRollingDay(t *testing.T) {
	c := &clock{t: time.Date(2026, 7, 30, 12, 0, 0, 0, time.UTC)}
	g := New(testCfg(), c.now)
	for i := 0; i < 3; i++ {
		g.Charge("x", 0.001)
	}
	if d := g.Charge("x", 0.001); d.Allowed {
		t.Fatal("should be exhausted")
	}
	c.add(24 * time.Hour)
	if d := g.Charge("x", 0.001); !d.Allowed {
		t.Fatalf("quota should reset after the rolling day: %s", d.Reason)
	}
}

func TestIdentityRotationCannotExhaustMemory(t *testing.T) {
	// A public endpoint sees address rotation; the identity table must not
	// grow without bound.
	c := &clock{t: time.Now()}
	g := New(testCfg(), c.now) // MaxIdentities: 4
	for i := 0; i < 50; i++ {
		g.Charge(fmt.Sprintf("ip-%d", i), 0.001)
	}
	if n := g.Len(); n > 4 {
		t.Errorf("identity table grew past MaxIdentities: %d", n)
	}
}

func TestFullTableOfLiveIdentitiesFailsClosed(t *testing.T) {
	// Evicting a LIVE limiter to admit a new identity would let an attacker
	// reset their own quota by flooding fresh identities. Refuse instead.
	c := &clock{t: time.Now()}
	g := New(testCfg(), c.now) // 4 slots, 48h TTL
	for i := 0; i < 4; i++ {
		if d := g.Charge(fmt.Sprintf("live-%d", i), 0.001); !d.Allowed {
			t.Fatalf("setup charge %d denied: %s", i, d.Reason)
		}
	}
	d := g.Charge("newcomer", 0.001)
	if d.Allowed {
		t.Fatal("admitting a 5th identity by evicting a live one must be refused")
	}

	// Once entries age past the TTL they become evictable and newcomers are
	// admitted again.
	c.add(49 * time.Hour)
	if d := g.Charge("newcomer", 0.001); !d.Allowed {
		t.Fatalf("after TTL expiry a newcomer should be admitted: %s", d.Reason)
	}
}

func TestGCDropsExpiredOnly(t *testing.T) {
	c := &clock{t: time.Now()}
	g := New(testCfg(), c.now)
	g.Charge("old", 0.001)
	c.add(49 * time.Hour)
	g.Charge("fresh", 0.001)
	if n := g.GC(); n != 1 {
		t.Errorf("GC should drop exactly the expired entry, dropped %d", n)
	}
	if g.Len() != 1 {
		t.Errorf("the fresh entry should survive GC, len=%d", g.Len())
	}
}

func TestSnapshotRestoreRoundTripPreservesQuota(t *testing.T) {
	// Quota must survive a process restart, or restarting the service becomes
	// a quota reset an attacker can trigger by making it crash.
	c := &clock{t: time.Date(2026, 7, 30, 12, 0, 0, 0, time.UTC)}
	g := New(testCfg(), c.now)
	for i := 0; i < 3; i++ {
		g.Charge("x", 0.001)
	}

	g2 := New(testCfg(), c.now)
	g2.Restore(g.Snapshot())
	if d := g2.Charge("x", 0.001); d.Allowed {
		t.Fatal("an exhausted quota must survive Snapshot/Restore")
	}
}

func TestRestoreEnforcesMemoryBound(t *testing.T) {
	// A stale or tampered snapshot must not be able to exceed the bound.
	c := &clock{t: time.Now()}
	g := New(testCfg(), c.now) // MaxIdentities: 4
	var snap Snapshot
	for i := 0; i < 20; i++ {
		snap.Entries = append(snap.Entries, SnapshotEntry{
			Identity: fmt.Sprintf("ip-%d", i), ReqThisDay: 1,
			DayWindow: c.t, LastSeen: c.t.Add(-time.Duration(i) * time.Minute),
		})
	}
	g.Restore(snap)
	if n := g.Len(); n != 4 {
		t.Errorf("Restore should clamp to MaxIdentities, got %d", n)
	}
}

func TestSnapshotIsDeterministic(t *testing.T) {
	c := &clock{t: time.Now()}
	g := New(testCfg(), c.now)
	g.Charge("zeta", 0.001)
	g.Charge("alpha", 0.001)
	s1, s2 := g.Snapshot(), g.Snapshot()
	if len(s1.Entries) != 2 || s1.Entries[0].Identity != "alpha" {
		t.Fatalf("snapshot should be sorted by identity: %+v", s1.Entries)
	}
	for i := range s1.Entries {
		if s1.Entries[i] != s2.Entries[i] {
			t.Error("successive snapshots differ")
		}
	}
}
