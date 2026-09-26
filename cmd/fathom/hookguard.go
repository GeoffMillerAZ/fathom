package main

// hookguard.go — the cmd/fathom side of internal/hookguard
// (docs/design/responsible-tenant.md §9, §11; plan rows RT-7..RT-10).
//
// Every hook verb calls beginHookGuard right after it knows its harness and
// daemon URL. That computes the invocation's one monotonic deadline from
// the harness descriptor, stats the managed daemon's kill switch (a
// disabled or paused daemon silences the hook before any RPC), and
// activates the guard for the rest of the process. The guard is applied
// by a pass-through wrapper around http.DefaultTransport installed once at
// package init, so every `&http.Client{Timeout: …}` on a hook path —
// there are dozens — derives its context from the hook's deadline, sends
// X-Fathom-Deadline, and passes the shared per-URL breaker, without each
// call site having to remember to. Outside a hook (the daemon, CLI verbs)
// no guard is active and the wrapper is a pointer load and a tail call.

import (
	"net/http"
	"sort"
	"strings"
	"sync/atomic"
	"time"

	"github.com/geoffmilleraz/fathom/internal/harness"
	"github.com/geoffmilleraz/fathom/internal/hookguard"
)

var activeHookGuardPtr atomic.Pointer[hookguard.Guard]

func activeHookGuard() *hookguard.Guard { return activeHookGuardPtr.Load() }

// hookBudgetScale shrinks every hook deadline in tests
// (TestNoHookRPCOutlivesItsDescriptorBudget drives every descriptor
// through a hanging daemon at a tenth of real time). Always 1 in the
// binary.
var hookBudgetScale = 1.0

func init() {
	// Installed before main or any test runs, so no goroutine can race the
	// assignment. The wrapper is inert until a hook activates a guard.
	http.DefaultTransport = &hookguard.Transport{Base: http.DefaultTransport, Active: activeHookGuard}
}

// activateHookGuard makes g the process's hook guard and returns the
// restore func (tests run hooks in process, one at a time).
func activateHookGuard(g *hookguard.Guard) (restore func()) {
	prev := activeHookGuardPtr.Swap(g)
	return func() { activeHookGuardPtr.Store(prev) }
}

// hookTimeoutSec is the harness descriptor timeout for the hook whose
// wired marker is marker ("fathom edit-hook --pre"), under harness id
// ("" = claude-code). Several bindings can share a marker (edit-hook --pre
// guards Edit and Bash); the smallest timeout wins. 0 when no descriptor
// wires it (the git post-commit hook): no deadline beyond each client's.
func hookTimeoutSec(harnessID, marker string) int {
	if marker == "" {
		return 0
	}
	if harnessID == "" {
		harnessID = harness.ClaudeCode
	}
	minTimeout := func(ds []harness.Descriptor) int {
		best := 0
		for _, d := range ds {
			for _, b := range d.Hooks {
				if b.Marker != marker || b.TimeoutSec <= 0 {
					continue
				}
				if best == 0 || b.TimeoutSec < best {
					best = b.TimeoutSec
				}
			}
		}
		return best
	}
	if d, ok := harness.Lookup(harnessID); ok {
		if t := minTimeout([]harness.Descriptor{d}); t > 0 {
			return t
		}
	}
	// A verb the named harness does not wire (doctor --brief under
	// --harness auto): the smallest timeout any harness gives it.
	return minTimeout(harness.All())
}

// beginHookGuard starts a hook invocation's guard. silent reports the
// managed daemon disabled or paused (§9): the caller returns 0 at once,
// with no output and no RPC. end restores the previous guard and records
// a beat when the breaker or kill switch refused any RPC (§11.2: beats
// record breaker_open and busy).
func beginHookGuard(channel, marker, harnessID, baseURL string) (end func(), silent bool) {
	start := time.Now()
	g := &hookguard.Guard{Deadline: hookguard.HookDeadline(start, hookTimeoutSec(harnessID, marker)).Scale(hookBudgetScale)}
	d := hookguard.BreakerAllows(g, baseURL)
	if d.Verdict == hookguard.Disabled || d.Verdict == hookguard.Paused {
		return func() {}, true
	}
	restore := activateHookGuard(g)
	return func() {
		restore()
		skips := g.Skips()
		if len(skips) == 0 {
			return
		}
		meta := map[string]string{}
		var kinds []string
		for v, n := range skips {
			kinds = append(kinds, string(v))
			meta[string(v)] = itoa(n)
		}
		sort.Strings(kinds)
		meta["verdicts"] = strings.Join(kinds, ",")
		emitBeat(channel, "guard_skip", meta)
	}, false
}
