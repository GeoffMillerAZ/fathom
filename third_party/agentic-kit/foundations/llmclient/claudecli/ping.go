package claudecli

import (
	"bytes"
	"context"
	"fmt"
	"time"

	boundedexec "github.com/geoffmilleraz/agentic-kit/foundations/agent/bounded-exec"
	"github.com/geoffmilleraz/agentic-kit/foundations/llmclient"
)

var _ llmclient.Pinger = (*Client)(nil)

const (
	// pingTimeout bounds one Ping call independently of Config.Timeout (the
	// effective bound is the smaller of the two, and of the caller's ctx
	// deadline). The healthy outcome is a LOCAL error within a few seconds
	// of pure CLI/Node startup overhead (verified live, repeatedly —
	// testdata/PROBES.md probes A/B/F: 1.0-3.2s across six runs, versus a
	// real completion's 5.6s wall time WITH a network round trip — see
	// probe E); a Ping that is still running after this long means the
	// installed CLI did not perform the expected local input-presence check
	// and may be attempting real work — kill it and report the degradation.
	pingTimeout = 15 * time.Second

	// pingHealthyMarker is the stderr text the installed CLI (2.1.212)
	// emits — locally, before any network activity — when run in print mode
	// (implied by --output-format/-p) with neither a prompt argument nor
	// any stdin content (verified live, stable across six runs and every
	// flag combination tried: testdata/PROBES.md probes A-F). Its presence
	// proves the CLI still refuses to proceed without SOME input, which is
	// what makes Ping's empty-stdin invocation structurally unable to reach
	// the API: there is no prompt for it to send.
	pingHealthyMarker = "Input must be provided either through stdin or as a prompt argument"
)

// Ping implements llmclient.Pinger as a LOCAL, pre-API, zero-token liveness
// probe for this adapter. Call it at construction or deploy time (e.g. via
// llmclient.ProbeAll), and after any claude-cli upgrade.
//
// Mechanism: Ping runs the exact pinned production argv Complete would build
// for an empty Request (buildArgs(llmclient.Request{}) — no --model,
// --system-prompt, --effort, or --json-schema beyond whatever Config itself
// pins), with EMPTY stdin and — per this adapter's invocation shape, see the
// package doc — never a positional prompt argument either. Verified live
// against the installed CLI (testdata/PROBES.md, probes A-F, and re-verified
// under the exact pinned argv shape in probe B): with no prompt supplied
// through either channel, the CLI rejects the invocation with a hard LOCAL
// error — "Input must be provided either through stdin or as a prompt
// argument..." on stderr, exit 1, EMPTY stdout, 1.0-3.2s (pure Node startup,
// no model resolution or network activity: probe D shows the identical
// error even with a deliberately bogus --model value, proving the
// input-presence check runs before model resolution). That error is the
// HEALTHY outcome: Ping returns nil.
//
// Contrast with a real call: a live "reply pong" probe against the same
// pinned argv shape took 5.6s wall time and returned a full JSON result
// envelope carrying real usage/cost fields (testdata/PROBES.md, probe E) —
// nothing resembling that envelope, and no API-shaped cost, ever appears on
// the healthy Ping path; probe C additionally confirms that even
// WHITESPACE-only stdin (as opposed to truly empty stdin) is accepted as a
// "provided" prompt and proceeds past this local gate, which is exactly why
// Ping's Stdin is left nil rather than merely blank.
//
// If a future claude-cli version drops or reorders this input-presence
// check (e.g. treats empty stdin as an empty-but-valid prompt and proceeds
// to call the API, or changes the stderr wording), Ping cannot silently
// pass: any outcome other than the exact healthy signature — including
// Ping's own timeout — returns a typed *llmclient.InvalidRequestError, so a
// deploy-time ProbeAll fails loud rather than trusting an adapter whose
// pre-flight assumptions have quietly shifted underneath it.
//
// Error taxonomy (per the Pinger contract): caller ctx cancellation and
// caller deadlines pass through untyped; every degradation outcome —
// including Ping's own timeout — is a non-retryable
// *llmclient.InvalidRequestError, because retrying cannot fix an installed
// CLI whose local input-gate no longer behaves as pinned; re-verification
// per testdata/PROBES.md is the only fix.
//
// A nil Ping proves the resolved binary launches, accepts the exact pinned
// argv shape, and still enforces its local "no input, no request" gate.
// Like every Pinger, it does NOT prove credentials, model access, or quota
// — those failures surface on the first real Complete call.
func (c *Client) Ping(ctx context.Context) error {
	args := c.buildArgs(llmclient.Request{})

	pingCtx, cancel := context.WithTimeout(ctx, pingTimeout)
	defer cancel()

	res, err := c.runner.Run(pingCtx, boundedexec.Spec{
		Path: c.bin,
		Args: args,
		// Stdin deliberately nil (immediate EOF), and buildArgs never emits
		// a positional prompt argument (see the package doc, "Pinned
		// invocation shape"): the probe's trigger is the total absence of
		// any prompt, which the healthy CLI rejects before touching the
		// network. A merely-blank (whitespace) stdin does NOT trigger the
		// same rejection (testdata/PROBES.md probe C) — nil is required.
		Stdin: nil,
	})
	if err != nil {
		// Caller cancellation (or a start/wait failure) — pass through
		// untyped per the Pinger contract.
		return fmt.Errorf("claudecli: ping: %w", err)
	}
	if res.TimedOut {
		if ctx.Err() != nil {
			// The CALLER's deadline fired, not pingTimeout — pass through
			// untyped so the caller can tell its own stop from a failure.
			return fmt.Errorf("claudecli: ping: %w", ctx.Err())
		}
		return &llmclient.InvalidRequestError{
			Message: fmt.Sprintf(
				"claudecli: ping: the claude CLI ran for %s without the expected local no-input rejection — it may have proceeded toward a real API call instead of rejecting the empty-stdin, no-prompt-argument invocation; re-verify per foundations/llmclient/claudecli/testdata/PROBES.md before trusting this adapter",
				pingTimeout,
			),
		}
	}

	if res.ExitCode != 0 && bytes.Contains(res.Stderr, []byte(pingHealthyMarker)) {
		return nil
	}

	return &llmclient.InvalidRequestError{
		Message: fmt.Sprintf(
			"claudecli: ping: the claude CLI did not reject an empty-stdin, no-prompt-argument invocation the expected way (exit code %d, stderr: %q) — the installed CLI's local no-input gate this adapter's zero-token Ping depends on may have changed or been silently dropped; re-verify per foundations/llmclient/claudecli/testdata/PROBES.md before trusting this adapter",
			res.ExitCode, tail(res.Stderr, maxErrorTailBytes),
		),
	}
}
