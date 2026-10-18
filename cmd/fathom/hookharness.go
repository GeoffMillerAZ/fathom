package main

// hookharness.go — the cmd/fathom side of the harness seam (ADR-029,
// docs/vision/25-harness-port.md §2): the hook table derives from the
// registry, the five hook verbs resolve their codec from an explicit
// `--harness <id>` (absent ⇒ claude-code, so every wired Claude Code
// command line and its goldens stay byte-identical), and the CLI's
// harness label and env reads come from the descriptor's env contract
// instead of `CLAUDE_*` literals. Detection never happens in a hook verb
// (vision 25 §2.2 rule 6): init and doctor detect; hooks are told.

import (
	"flag"
	"fmt"
	"io"
	"os"
	"slices"
	"strings"

	"github.com/geoffmilleraz/fathom/internal/harness"
)

// hookSpecsFor derives the hook table for a registry id from its event
// bindings: one hookSpec per binding, in binding order. For claude-code
// this reproduces the pre-seam fathomHookSpecs literal event for event
// (TestHookSpecsForClaude pins it). An unknown id, or a descriptor with
// no bindings, yields nil — there is no hook table to write. A binding
// with an OptIn key (RO-38: SubagentStop under hooks.claim_check) is in
// the table only when optIns names that key, so a caller that passes none
// gets the default table; fathomHookSpecsAt passes the keys a root's
// config switches on. With both the readout hook's key and [hooks]
// test_run named (RO-21), a test-run row whose event and matcher a readout
// row also binds is left out: the readout hook runs the test-run tier in
// its own process, so the pair is one entry per event.
func hookSpecsFor(id string, optIns ...string) []hookSpec {
	d, ok := harness.Lookup(id)
	if !ok || len(d.Hooks) == 0 {
		return nil
	}
	fold := slices.Contains(optIns, harness.OptInReadouts) && slices.Contains(optIns, hookOptInTestRun)
	out := make([]hookSpec, 0, len(d.Hooks))
	for _, b := range d.Hooks {
		if b.OptIn != "" && !slices.Contains(optIns, b.OptIn) {
			continue
		}
		if fold && b.Capability == harness.CapTestRunLine && readoutBinds(d, b) {
			continue
		}
		out = append(out, hookSpec{
			event:   b.Harness,
			matcher: b.Matcher,
			marker:  b.Marker,
			sub:     b.Verb,
			timeout: b.TimeoutSec,
			async:   d.RegisterAsync(b),
			optIn:   b.OptIn,
		})
	}
	return out
}

// readoutBinds reports whether d carries a readout row on b's event and
// matcher, the row that runs b's tier when the two fold (RO-21).
func readoutBinds(d harness.Descriptor, b harness.EventBinding) bool {
	for _, r := range d.Hooks {
		if r.Capability == harness.CapReadout && r.Event == b.Event && r.Matcher == b.Matcher {
			return true
		}
	}
	return false
}

// harnessFlagUsage is the one --harness help string the five hook verbs
// share.
const harnessFlagUsage = "harness id whose hook envelope to speak (default claude-code; 'fathom harness list' names the rest)"

// addHarnessFlag registers --harness on a hook verb's flag set. The
// default is the empty string, not "claude-code", so a wired command line
// that never spelled the flag is indistinguishable from one that did —
// resolveHookHarness maps "" to claude-code.
func addHarnessFlag(fs *flag.FlagSet) *string {
	return fs.String("harness", "", harnessFlagUsage)
}

// resolveHookHarness maps a hook verb's --harness value to its codec.
// "" ⇒ claude-code. An unknown id, or a descriptor whose codec has not
// landed, is I9 fail-open: ONE stderr line and ok=false, and the verb
// exits 0 — a hook must never fail the harness that called it, and a
// misspelled flag is an operator error the harness's debug log should
// show, not a blocked tool call.
func resolveHookHarness(flagVal string, stderr io.Writer) (harness.Codec, bool) {
	id := flagVal
	if id == "" {
		id = harness.ClaudeCode
	}
	if c, ok := harness.CodecFor(id); ok {
		// Every beat this process writes from here on carries the harness
		// it spoke for (beats.go) — the evidence doctor's hooks_firing
		// reads for a non-Claude harness.
		beatHarness = id
		return c, true
	}
	if _, known := harness.Lookup(id); known {
		fmt.Fprintf(stderr, "fathom: --harness %q has no hook codec in this build (hooks: not wired, wave 2) — exiting 0 (fail-open)\n", id)
	} else {
		fmt.Fprintf(stderr, "fathom: unknown --harness %q (known: %v) — exiting 0 (fail-open)\n", id, harness.IDs())
	}
	return nil, false
}

// harnessProjectDir reads the project-dir variable the harness id's env
// contract names (Claude Code: CLAUDE_PROJECT_DIR); "" when the
// descriptor names none or the variable is unset.
func harnessProjectDir(id string) string {
	d, ok := harness.Lookup(id)
	if !ok || d.Env.ProjectDir == "" {
		return ""
	}
	return os.Getenv(d.Env.ProjectDir)
}

// anyHarnessProjectDir reads the first set project-dir variable across
// every descriptor's env contract — for the pull-tier verbs (`prime`),
// which serve whichever harness is running them.
func anyHarnessProjectDir() string {
	for _, d := range harness.All() {
		if d.Env.ProjectDir == "" {
			continue
		}
		if v := os.Getenv(d.Env.ProjectDir); v != "" {
			return v
		}
	}
	return ""
}

// harnessLabel is the ledger label for the harness whose session variable
// is set — the registry's Label ("claude" for Claude Code), never a
// `CLAUDE*` literal — and that session id. "cli" with no id when no
// harness's SessionID variable is set. Only the descriptor's primary
// SessionID variable counts here (leases pinned that rule before the
// seam: $CLAUDE_SESSION_ID, not its aliases).
func harnessLabel() (label, sessionID string) {
	for _, d := range harness.All() {
		if d.Env.SessionID == "" {
			continue
		}
		if v := os.Getenv(d.Env.SessionID); v != "" {
			return d.Label, v
		}
	}
	return "cli", ""
}

// harnessSessionID resolves the current session id from any harness's
// env contract, aliases first (Claude Code exports CLAUDE_CODE_SESSION_ID
// to the agent's shell; CLAUDE_SESSION_ID is the older spelling).
func harnessSessionID() string {
	for _, d := range harness.All() {
		for _, k := range append(append([]string{}, d.Env.SessionIDAliases...), d.Env.SessionID) {
			if k == "" {
				continue
			}
			if v := os.Getenv(k); v != "" {
				return v
			}
		}
	}
	return ""
}

// harnessPresent reports whether any harness's env contract says a
// harness is running this process (its Present marker or a session id).
func harnessPresent() bool {
	for _, d := range harness.All() {
		if d.Env.Present != "" && os.Getenv(d.Env.Present) != "" {
			return true
		}
	}
	return harnessSessionID() != ""
}

// noteBeatHarness pre-scans a hook verb's args for --harness and stamps
// beatHarness from it, WITHOUT parsing the flag set — so a beat emitted
// before flag parsing still carries the harness that produced it. An
// absent flag means claude-code, the wired default, exactly as
// resolveHookHarness resolves it later.
func noteBeatHarness(args []string) {
	if id := hookHarnessArg(args); id != "" {
		beatHarness = id
		return
	}
	beatHarness = harness.ClaudeCode
}

// hookHarnessArg returns the value of --harness in args, or "".
func hookHarnessArg(args []string) string {
	for i, a := range args {
		if a == "--" {
			return ""
		}
		for _, name := range []string{"--harness", "-harness"} {
			if a == name && i+1 < len(args) {
				return args[i+1]
			}
			if strings.HasPrefix(a, name+"=") {
				return strings.TrimPrefix(a, name+"=")
			}
		}
	}
	return ""
}
