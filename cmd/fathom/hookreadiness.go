package main

// Advice readiness is local bookkeeping, never another daemon query. A
// missing fact or a read error preserves the hook's existing behavior (I9).

import (
	"bufio"
	"encoding/json"
	"os"
	"strings"
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
		harness.CapSubagentPrime:        {"exploration maps", readinessMaps},
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
		return readinessResult{reason: "hook-not-registered"}
	}
}

// Read only the log the hook actually sees, bounded to the same byte cap
// as hook state. Oversize/unreadable input is unknown, never no-maps.
func readinessMaps(in readinessInput) readinessResult {
	raw, err := readReadinessFile(threadsPath())
	if err != nil {
		if os.IsNotExist(err) {
			return readinessResult{reason: "no-maps"}
		}
		return readinessResult{ready: true}
	}
	s := bufio.NewScanner(strings.NewReader(string(raw)))
	s.Buffer(make([]byte, 4096), hookDedupeMaxBytes)
	for s.Scan() {
		var row threadItem
		if json.Unmarshal(s.Bytes(), &row) != nil {
			return readinessResult{ready: true}
		}
		if !isMapItem(row) {
			continue
		}
		at, err := time.Parse(time.RFC3339, row.FiledAt)
		if err != nil {
			return readinessResult{ready: true}
		}
		if !at.After(in.now) && !at.Before(in.now.Add(-hookDedupeAge)) {
			return readinessResult{ready: true}
		}
	}
	if s.Err() != nil {
		return readinessResult{ready: true}
	}
	return readinessResult{reason: "no-maps"}
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

// The raw extraction lane owns serving text. Here we only distinguish an
// instruction already returned by that path from its old heading-only row.
func rawHasInstruction(text string) bool {
	for _, line := range strings.Split(strings.ToLower(text), "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "#") {
			continue
		}
		words := " " + strings.Join(strings.Fields(line), " ") + " "
		for _, word := range []string{" must ", " never ", " always ", " only ", " do not ", "**decision**"} {
			if strings.Contains(words, word) {
				return true
			}
		}
	}
	return false
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
	if err := updateHookHealth(in.root, in.harnessID, cap, false, reason, r.uncompiled, in.now); err != nil {
		return true
	}
	if h := activeAdviceHealth; h != nil && h.cap == cap {
		h.recorded = true
	}
	emitBeat(channel, "skipped", firingMeta(map[string]string{"emitted": "false", "reason": reason, "capability": string(cap)}, session, ""))
	return false
}
