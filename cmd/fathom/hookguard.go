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
	"fmt"
	"net/http"
	"os"
	"sort"
	"sync/atomic"
	"time"

	"github.com/geoffmilleraz/fathom/internal/custodian"
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
	// It is no longer a *http.Transport: nothing fathom runs type-asserts
	// it (cue's tool/http does, but only inside a tools/flow `cue cmd`
	// run, which fathom never starts), and net/http then enforces
	// Client.Timeout by cancelling the request context — which is why
	// hookguard counts a context whose deadline has passed as a failure.
	http.DefaultTransport = &hookguard.Transport{Base: http.DefaultTransport, Active: activeHookGuard}
}

// activateHookGuard makes g the process's hook guard and returns the
// restore func (tests run hooks in process, one at a time).
func activateHookGuard(g *hookguard.Guard) (restore func()) {
	prev := activeHookGuardPtr.Swap(g)
	return func() { activeHookGuardPtr.Store(prev) }
}

// hookDeadlineSpent reports that the running hook's deadline has passed
// (always false outside a hook). RT-7's local-work half (§11.1): the guard
// refuses every RPC after the deadline, and a hook that then carried on
// with its local work — git reads feeding an RPC that will not be sent,
// doctor's remaining checks, the continuity brief — ran past its budget by
// however long that work took: ~100 ms idle, 350-420 ms at background
// priority on a loaded machine. Past the deadline a hook skips that work,
// says what it skipped, and exits.
func hookDeadlineSpent() bool {
	g := activeHookGuard()
	return g != nil && g.Deadline.Spent()
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
	// RT-135: this invocation's id, stamped on every beat it emits and on
	// its latency row, so doctor tells a killed hook from a silent exit.
	inv := fmt.Sprintf("%d-%d", os.Getpid(), start.UnixNano())
	prevInv := hookInvocation.Swap(&inv)
	// RT-134 (§11.3): in every hook the custodian's config walk ends 2 s
	// under the harness timeout and a directory that is not a repository
	// is refused, named — SessionStart's attention line included.
	prevWalk := custodian.SetWalkPolicy(&custodian.WalkPolicy{Deadline: hookWalkDeadline(start, hookTimeoutSec(harnessID, marker)), RefuseNonGit: true})
	return func() {
		custodian.SetWalkPolicy(prevWalk)
		restore()
		// RT-135 (§11.3): every invocation's wall time against its
		// deadline, for doctor's hook_budget.
		recordHookLatency(channel, inv, time.Since(start), g.Deadline.Budget(), time.Now())
		hookInvocation.Store(prevInv)
		skips := g.Skips()
		if len(skips) == 0 {
			return
		}
		// One beat per verdict, named as §11.2 names them (breaker_open,
		// busy, deadline_spent), with the count of RPCs it held off.
		kinds := make([]string, 0, len(skips))
		for v := range skips {
			kinds = append(kinds, string(v))
		}
		sort.Strings(kinds)
		for _, k := range kinds {
			emitBeat(channel, k, map[string]string{"skipped": itoa(skips[hookguard.Verdict(k)])})
		}
	}, false
}

// hookWalkDeadline is the config walk's deadline for a hook that started
// at start under a timeout of timeoutSec: 2 s under the timeout, never
// less than half of it; zero (none) when the hook has no timeout.
func hookWalkDeadline(start time.Time, timeoutSec int) time.Time {
	if timeoutSec <= 0 {
		return time.Time{}
	}
	total := time.Duration(timeoutSec) * time.Second
	d := total - 2*time.Second
	if d < total/2 {
		d = total / 2
	}
	return start.Add(d)
}

// hookInvocation is the running hook invocation's id (nil outside a hook).
var hookInvocation atomic.Pointer[string]

// currentHookInvocation is that id, "" outside a hook.
func currentHookInvocation() string {
	if p := hookInvocation.Load(); p != nil {
		return *p
	}
	return ""
}
