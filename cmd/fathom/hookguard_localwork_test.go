package main

import (
	"strings"
	"testing"
	"time"

	"github.com/geoffmilleraz/fathom/internal/hookcache"
	"github.com/geoffmilleraz/fathom/internal/hookguard"
)

// spentHookGuard activates a hook guard whose deadline has already passed:
// the state a hook is in after its one RPC waited out the whole budget on
// a hung daemon.
func spentHookGuard(t *testing.T) {
	t.Helper()
	g := &hookguard.Guard{Deadline: hookguard.HookDeadline(time.Now().Add(-time.Minute), 5)}
	if !g.Deadline.Spent() {
		t.Fatal("fixture: deadline not spent")
	}
	restore := activateHookGuard(g)
	t.Cleanup(restore)
}

// TestDoctorBriefStopsAtTheHookDeadline is RT-7's local-work half (§11.1):
// `doctor --brief` runs as the harness Setup hook, and a hung daemon spends
// its whole deadline on the reachability probe. The ~40 local checks after
// it used to run anyway — ~100 ms idle, 350-420 ms at background priority
// on a loaded machine (the pre-push gate), past the budget. Once the
// deadline is spent doctor stops, and says how many checks it did not run
// — a cut brief never reads as a clean one.
func TestDoctorBriefStopsAtTheHookDeadline(t *testing.T) {
	full := runDoctorChecks(t.TempDir(), "", "claude", "http://127.0.0.1:1")
	for _, c := range full {
		if c.Name == doctorDeadlineRow {
			t.Fatalf("no hook guard active, yet doctor reported a deadline cut: %s", c.Reason)
		}
	}

	spentHookGuard(t)
	checks := runDoctorChecks(t.TempDir(), "", "claude", "http://127.0.0.1:1")
	if len(checks) != 1 || checks[0].Name != doctorDeadlineRow {
		names := make([]string, 0, len(checks))
		for _, c := range checks {
			names = append(names, c.Name)
		}
		t.Fatalf("with the hook deadline spent doctor ran %v; want only the %s row", names, doctorDeadlineRow)
	}
	cut := checks[0]
	if cut.Status != doctorSkipped || !strings.Contains(cut.Reason, itoa(len(full))+" check(s) not run") {
		t.Fatalf("deadline row = %+v; want skipped, naming the %d checks not run", cut, len(full))
	}
	brief := doctorBrief(checks)
	if !strings.Contains(brief, "not run") || !strings.Contains(brief, "fathom doctor") {
		t.Fatalf("brief hides the cut:\n%q", brief)
	}
}

// TestIdentityLookupSkipsItsLocalWorkWhenTheDeadlineIsSpent: the identity
// lookup's git toplevel read only feeds its /identity RPC, which the guard
// refuses once the deadline is spent — so the git exec is skipped too
// (session-hook and SessionStart's stamp ran it after the hung healthz,
// for an RPC that could not be sent).
func TestIdentityLookupSkipsItsLocalWorkWhenTheDeadlineIsSpent(t *testing.T) {
	repo := hookRepo(t)
	prevDir, prevExec := hookcache.ToplevelDir, gitToplevelExec
	hookcache.ToplevelDir = t.TempDir()
	calls := 0
	gitToplevelExec = func(dir string) string { calls++; return prevExec(dir) }
	resetToplevelMemo()
	t.Cleanup(func() {
		hookcache.ToplevelDir, gitToplevelExec = prevDir, prevExec
		resetToplevelMemo()
	})

	spentHookGuard(t)
	if _, err := lookupIdentity(nil, "http://127.0.0.1:1", repo); err == nil {
		t.Fatal("lookupIdentity with a spent deadline returned no error")
	}
	if calls != 0 {
		t.Fatalf("git toplevel ran %d time(s) after the hook deadline was spent", calls)
	}
}
