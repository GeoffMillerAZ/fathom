package main

// Advice readiness is local bookkeeping, never another daemon query. A
// missing fact or a read error preserves the hook's existing behavior (I9).

import (
	"time"

	"github.com/geoffmilleraz/fathom/internal/harness"
	"github.com/geoffmilleraz/fathom/internal/semantic"
)

type readinessInput struct {
	root, harnessID   string
	now               time.Time
	rules             []semantic.Obligation
	instructionsKnown bool
	uncompiled        int
}

type readinessResult struct {
	ready      bool
	reason     string
	uncompiled int
}

type readinessEntry struct {
	name  string
	check func(readinessInput) readinessResult
}

// Every hook-inject vehicle has an explicit entry. Checks without a
// prerequisite are deliberately ready; adding an injecting capability
// without deciding its prerequisite fails the registry coverage test.
func hookReadinessRegistry() map[harness.Capability]readinessEntry {
	return map[harness.Capability]readinessEntry{
		harness.CapPrime:                {"session primer", readinessAlways},
		harness.CapObligationsPostEdit:  {"post-edit obligations", readinessInstructions},
		harness.CapObligationsPostShell: {"post-shell obligations", readinessInstructions},
		harness.CapGuardPreEdit:         {"edit guards", readinessAlways},
		harness.CapGuardPreShell:        {"shell guards", readinessAlways},
		harness.CapStopVerify:           {"stop verification", readinessAlways},
		harness.CapClaimGate:            {"claim gate", readinessAlways},
		harness.CapSubagentPrime:        {"exploration maps", readinessAlways},
		harness.CapToolFailureAdvice:    {"tool failure advice", readinessAlways},
		harness.CapWorktreeAdvice:       {"worktree advice", readinessAlways},
		harness.CapPromptObligations:    {"prompt obligations", readinessAlways},
		harness.CapTestRunLine:          {"test-run advice", readinessBinding(harness.CapTestRunLine)},
		harness.CapClaimCheck:           {"claim checks", readinessBinding(harness.CapClaimCheck)},
		harness.CapReadOutlineHint:      {"read outline hints", readinessBinding(harness.CapReadOutlineHint)},
		// The descriptor calls readouts hook-silent: observe mode never speaks,
		// but rich mode can, and shares the same opt-in registration check.
		harness.CapReadout: {"readouts", readinessBinding(harness.CapReadout)},
	}
}

func readinessAlways(readinessInput) readinessResult { return readinessResult{ready: true} }

func readinessBinding(cap harness.Capability) func(readinessInput) readinessResult {
	return func(in readinessInput) readinessResult {
		d, ok := harness.Lookup(in.harnessID)
		if ok {
			for _, b := range d.Hooks {
				if b.Capability == cap {
					return readinessResult{ready: true}
				}
			}
		}
		return readinessResult{reason: "unsupported-here"}
	}
}

func readinessInstructions(in readinessInput) readinessResult {
	r := readinessResult{ready: true, uncompiled: in.uncompiled}
	if !in.instructionsKnown {
		return r
	}
	for _, rule := range in.rules {
		if rule.Kind == "finding" {
			continue
		}
		if rule.Tier != "raw" || rawHasInstruction(rule.Rule) {
			return r
		}
	}
	r.ready, r.reason = false, "no-instructions"
	return r
}

// rawHasInstruction applies the daemon's own imperative test
// (semantic.IsInstruction, ADR-080 d2), so the hook never second-guesses a
// raw row with a different word list; a heading-only row from an older
// daemon still reads as no instruction.
func rawHasInstruction(text string) bool {
	return semantic.IsInstruction(text)
}

// An invocation itself proves that its entry point is registered. The
// primer additionally checks installation, since a missing hook cannot
// report its own absence. No hook is disabled by a stamp failure.
func channelReady(cap harness.Capability, in readinessInput, channel, session string) bool {
	entry, ok := hookReadinessRegistry()[cap]
	if !ok {
		return true
	}
	r := entry.check(in)
	if r.ready {
		return true
	}
	reason := "not-ready:" + r.reason
	_ = updateHookHealth(in.root, in.harnessID, cap, false, reason, in.now)
	if h := activeAdviceHealth; h != nil && h.cap == cap {
		h.recorded = true
	}
	emitBeat(channel, "skipped", firingMeta(map[string]string{"emitted": "false", "reason": reason, "capability": string(cap)}, session, ""))
	return false
}
