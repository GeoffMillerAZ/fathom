package main

// sessionhook_instructions.go — PC-23 (docs/vision/26-proof-credibility.md
// §4): `fathom session-hook --event instructions-loaded`, the harness's
// InstructionsLoaded event. It records ONE silent beat naming the file the
// harness loaded and why — the first harness-side evidence that the
// CLAUDE.md/AGENTS.md stanza reached a session (I37 reads the file; this
// reads the harness). It injects nothing: the event has no decision
// control and its stdout is discarded (hooks reference §7.2), so the beat
// is the whole product. Doctor's stanza_loaded row reads it.

import (
	"fmt"
	"io"
	"strings"

	"github.com/geoffmilleraz/fathom/internal/harness"
)

// instructionsLoadedChannel is the beat channel; kind "loaded". Its own
// channel — never "session-hook" — so the SessionStart counters doctor
// and instrument read stay session counts.
const instructionsLoadedChannel = "instructions-loaded"

// sessionHookEventArg returns the value of --event in args ("" when the
// flag is absent) without parsing the whole flag set: cmdSessionHook must
// route an event invocation BEFORE it emits its own "fired" beat, or every
// instruction load would count as a session start.
func sessionHookEventArg(args []string) string {
	for i, a := range args {
		if a == "--" {
			return ""
		}
		for _, name := range []string{"--event", "-event"} {
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

// cmdSessionHookEvent dispatches `session-hook --event <name>`. Every
// path exits 0 (hook contract); an unknown event is one stderr line.
func cmdSessionHookEvent(event string, args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	fs := newFlagSet("session-hook", stderr)
	url := fs.String("url", defaultFathomURL(), "fathom serve-http base URL")
	dir := fs.String("dir", "", "project dir (default: the event's own path, else $CLAUDE_PROJECT_DIR, else the envelope cwd)")
	fs.String("event", "", "harness event to handle instead of priming the session: "+strings.Join(sessionHookEvents, " | "))
	harnessFlag := addHarnessFlag(fs)
	if err := fs.Parse(args); err != nil {
		return 0
	}
	codec, ok := resolveHookHarness(*harnessFlag, stderr)
	if !ok {
		return 0 // I9: unknown harness id is one stderr line, never a failed hook
	}
	endGuard, silent := beginHookGuard("session-hook", "fathom session-hook --event "+event, codec.ID(), *url)
	if silent {
		return 0 // §9: the managed daemon is disabled or paused — no RPC, no output
	}
	defer endGuard()
	// The root an event is about is resolved per event (eventProjectDir);
	// --dir overrides it for tests and for an operator replaying a capture.
	root := func(in harness.HookInput) string {
		if *dir != "" {
			return *dir
		}
		return eventProjectDir(codec, in)
	}
	switch event {
	case "instructions-loaded":
		return recordInstructionsLoaded(codec, stdin)
	case "subagent-start":
		return runAdvisoryEvent(codec, harness.EventSubagentStart, "subagent-hook", stdin, stdout, func(in harness.HookInput) string {
			return subagentPrimer(*url, root(in), in.SessionID)
		})
	case "tool-failure":
		return runAdvisoryEvent(codec, harness.EventToolFailure, "tool-failure-hook", stdin, stdout, func(in harness.HookInput) string {
			return toolFailureAdvice(*url, root(in), in.ToolName, in.ToolError)
		})
	case "cwd-changed":
		return runAdvisoryEvent(codec, harness.EventCwdChanged, "cwd-hook", stdin, stdout, func(in harness.HookInput) string {
			return cwdChangedPrimer(*url, root(in))
		})
	case "test-run": // TR-2: PostToolUse(Bash), a piped or passing test run (TR-V1)
		return cmdTestRunHook(codec, harness.EventPostTool, event, *url, *dir, stdin, stdout)
	case "failed-test-run": // TR-2: PostToolUseFailure(Bash), an unpiped run that exited non-zero
		return cmdTestRunHook(codec, harness.EventToolFailure, event, *url, *dir, stdin, stdout)
	default:
		fmt.Fprintf(stderr, "fathom: session-hook --event %q: unknown event (known: %s) — exiting 0\n", event, strings.Join(sessionHookEvents, ", "))
		return 0
	}
}

// sessionHookEvents is the --event vocabulary, in help order.
var sessionHookEvents = []string{"instructions-loaded", "subagent-start", "tool-failure", "cwd-changed", "test-run", "failed-test-run"}

// recordInstructionsLoaded decodes the InstructionsLoaded envelope through
// the codec and writes one beat {file_path, memory_type, load_reason} (+
// the session join key). Absent fields are omitted, never recorded as "".
// Malformed or empty stdin records nothing and still exits 0.
func recordInstructionsLoaded(codec harness.Codec, stdin io.Reader) int {
	if stdin == nil {
		return 0
	}
	raw, err := io.ReadAll(io.LimitReader(stdin, 1<<16))
	if err != nil {
		return 0
	}
	in, err := codec.DecodeInput(harness.EventInstructionsLoaded, raw)
	if err != nil {
		return 0
	}
	meta := map[string]string{}
	if in.FilePath != "" {
		meta["file_path"] = in.FilePath
	}
	if in.MemoryType != "" {
		meta["memory_type"] = in.MemoryType
	}
	if in.LoadReason != "" {
		meta["load_reason"] = in.LoadReason
	}
	emitBeat(instructionsLoadedChannel, "loaded", firingMeta(meta, in.SessionID, ""))
	return 0
}
