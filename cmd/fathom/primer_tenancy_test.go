package main

import (
	"strings"
	"testing"
	"time"

	"github.com/geoffmilleraz/fathom/internal/tenancy"
)

// RT-51 (§13.3): the session primer carries one tenancy attention line —
// at most 160 B, inside I12's 3,200 B cap — only when the daemon is not
// normal: shed or refusing, in safe mode, crash-looping, stopped by the
// breaker, or holding a quarantined service. A normal daemon adds nothing.
func TestPrimerTenancyLineOnlyWhenNotNormal(t *testing.T) {
	line := func(t *testing.T, f *resourcesFixture, env resourcesEnv) string {
		t.Helper()
		l := tenancyAttentionLine(buildResources(env, "", "", time.Hour), f.now)
		if len(l) > tenancyAttentionBudget {
			t.Fatalf("line is %d B, over the %d B budget: %q", len(l), tenancyAttentionBudget, l)
		}
		return l
	}
	fresh := func(f *resourcesFixture) { f.now = f.now.Add(-8 * time.Minute) } // the ring's last sample is 2 min old

	t.Run("normal: no line", func(t *testing.T) {
		f := newResourcesFixture(t)
		f.writeRing(t, 3400*tenancy.MiB, tenancy.LevelNormal, false)
		fresh(f)
		if l := line(t, f, f.env()); l != "" {
			t.Fatalf("normal daemon produced %q", l)
		}
	})
	t.Run("no files at all: no line (the identity line already speaks for a missing daemon)", func(t *testing.T) {
		f := newResourcesFixture(t)
		if l := line(t, f, f.env()); l != "" {
			t.Fatalf("got %q", l)
		}
	})
	t.Run("shed: level, since, footprint vs budget, top tool", func(t *testing.T) {
		f := newResourcesFixture(t)
		f.writeRing(t, 3400*tenancy.MiB, tenancy.LevelShed, false)
		fresh(f)
		l := line(t, f, f.env())
		for _, w := range []string{"fathom: shed", "of 4 GiB budget", "top: obligations", "fathom resources"} {
			if !strings.Contains(l, w) {
				t.Errorf("line lacks %q: %q", w, l)
			}
		}
	})
	t.Run("a stale ring does not claim a level", func(t *testing.T) {
		f := newResourcesFixture(t)
		f.writeRing(t, 3400*tenancy.MiB, tenancy.LevelShed, false) // last sample 10 min old
		if l := line(t, f, f.env()); l != "" {
			t.Fatalf("a 10-minute-old sample claimed a level: %q", l)
		}
	})
	t.Run("safe mode after over-budget exits", func(t *testing.T) {
		f := newResourcesFixture(t)
		f.writeRing(t, 3400*tenancy.MiB, tenancy.LevelNormal, true)
		fresh(f)
		f.ledger(t,
			`{"row":"exit","at":"`+at(f.now.Add(-4*time.Minute))+`","reason":"over_budget","code":70}`,
			`{"row":"exit","at":"`+at(f.now.Add(-9*time.Minute))+`","reason":"over_budget","code":70}`,
			`{"row":"exit","at":"`+at(f.now.Add(-14*time.Minute))+`","reason":"unclean"}`)
		l := line(t, f, f.env())
		for _, w := range []string{"fathom: safe mode", "3 counted exits/30m", "top: obligations", "fathom resources"} {
			if !strings.Contains(l, w) {
				t.Errorf("line lacks %q: %q", w, l)
			}
		}
	})
	t.Run("stopped by the breaker", func(t *testing.T) {
		f := newResourcesFixture(t)
		env := f.env()
		since := time.Date(2026, 9, 26, 21, 20, 0, 0, time.UTC)
		env.killSw = func(string, time.Time) tenancy.KillSwitchVerdict {
			return tenancy.KillSwitchVerdict{Action: tenancy.ActionExitStay, Record: tenancy.SwitchRecord{Reason: "breaker", Since: since}}
		}
		l := line(t, f, env)
		if !strings.HasPrefix(l, "fathom stopped by breaker 21:20") || !strings.Contains(l, "operator: fathom daemon start") {
			t.Fatalf("got %q", l)
		}
	})
	t.Run("quarantine, with a long service name cut to the budget", func(t *testing.T) {
		f := newResourcesFixture(t)
		f.writeRing(t, 3400*tenancy.MiB, tenancy.LevelNormal, false)
		fresh(f)
		svc := strings.Repeat("very-long-service-name-", 10)
		f.ledger(t, `{"row":"quarantine","at":"`+at(f.now.Add(-time.Minute))+`","service":"`+svc+`","reason":"exceeded budget during ingest"}`)
		l := line(t, f, f.env())
		if !strings.HasPrefix(l, "fathom: quarantined ") || !strings.HasSuffix(l, "…") {
			t.Fatalf("got %q", l)
		}
	})

	// The primer carries the line after identity, and nothing when normal.
	restore := primerTenancyLine
	defer func() { primerTenancyLine = restore }()
	primerTenancyLine = func(time.Time) string {
		return "fathom: shed since 21:07 (3.2 GiB of 4 GiB budget; top: obligations) — fathom resources"
	}
	p, _, _ := composeContinuityAndCustodian("identity", t.TempDir(), time.Now())
	if !strings.HasPrefix(p, "identity\nfathom: shed since 21:07") {
		t.Fatalf("primer lacks the tenancy line after identity: %q", p)
	}
	primerTenancyLine = func(time.Time) string { return "" }
	if p, _, _ := composeContinuityAndCustodian("identity", t.TempDir(), time.Now()); strings.Contains(p, "fathom:") {
		t.Fatalf("a normal daemon added a line: %q", p)
	}
}
