package codexcli

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
	// deadline). The healthy outcome is a sub-second LOCAL error (verified
	// live — see testdata/PROBES.md, probe 13); a Ping that is still running
	// after this long means the installed CLI did not perform the expected
	// local validation and may be attempting real work — kill it and report
	// the degradation.
	pingTimeout = 15 * time.Second

	// pingHealthyMarker is the stderr text the installed CLI (0.144.5)
	// emits — locally, pre-API, before even reading stdin — when
	// model_instructions_file points at an empty file (verified live twice:
	// testdata/PROBES.md probes 8 and 13). Its presence proves the CLI still
	// recognizes and validates the undocumented config key this adapter's
	// Request.System mapping and token-floor depend on.
	pingHealthyMarker = "model instructions file is empty"
)

// Ping implements llmclient.Pinger as the liveness tripwire for this
// adapter's undocumented-config-key dependence (see the WARNING on Config).
// Call it at construction or deploy time, and after any codex-cli upgrade.
//
// Mechanism — deliberately inverted from a normal health probe: Ping runs
// the exact pinned production argv (buildArgs) with a deliberately EMPTY
// model_instructions_file and an empty stdin prompt. Verified live
// (testdata/PROBES.md, probe 13): a CLI that still honors the key rejects
// the empty file with a hard LOCAL error — "model instructions file is
// empty" on stderr, exit 1, sub-second, zero JSONL events on stdout, before
// stdin is even read, so no API call and no tokens are ever spent. That
// error is the HEALTHY outcome: Ping returns nil.
//
// If a future codex-cli version silently drops or renames the key (an
// unrecognised -c key=value is verified to be tolerated with exit 0 — the
// exact silent-degradation hazard the Config WARNING describes), the
// empty-file validation stops firing and the process behaves some other way
// — proceeds toward a real turn (with an empty prompt, and killed by
// pingTimeout in any case), exits clean, or fails differently. Every one of
// those outcomes returns a typed *llmclient.InvalidRequestError naming the
// silent-degradation risk, so a deploy-time ProbeAll fails loud BEFORE the
// first compile call silently loses its System prompt and reverts to the
// CLI's ~13k-token built-in persona.
//
// Error taxonomy (per the Pinger contract): caller ctx cancellation and
// caller deadlines pass through untyped; every degradation outcome —
// including Ping's own timeout — is a non-retryable
// *llmclient.InvalidRequestError, because retrying cannot fix an installed
// CLI that no longer honors the pinned config keys; re-verification per
// testdata/PROBES.md and an adapter re-pin are the only fixes.
//
// A nil Ping proves the model_instructions_file validation seam is alive in
// the installed CLI. Like every Pinger, it does NOT prove credentials,
// model access, or quota — those failures surface on the first real call.
func (c *Client) Ping(ctx context.Context) error {
	// A deliberately empty instructions file — the probe's trigger.
	path, cleanup, err := writeScratchFile(c.scratchDir, "ping-instructions-*.txt", nil)
	if err != nil {
		return err
	}
	defer cleanup()

	args := c.buildArgs(llmclient.Request{}, path, "")

	pingCtx, cancel := context.WithTimeout(ctx, pingTimeout)
	defer cancel()

	res, err := c.runner.Run(pingCtx, boundedexec.Spec{
		Path: c.bin,
		Args: args,
		// Stdin deliberately empty (immediate EOF): even a degraded CLI that
		// skips the empty-file validation has no prompt to send.
		Stdin: nil,
	})
	if err != nil {
		// Caller cancellation (or a start/wait failure) — pass through
		// untyped per the Pinger contract.
		return fmt.Errorf("codexcli: ping: %w", err)
	}
	if res.TimedOut {
		if ctx.Err() != nil {
			// The CALLER's deadline fired, not pingTimeout — pass through
			// untyped so the caller can tell its own stop from a failure.
			return fmt.Errorf("codexcli: ping: %w", ctx.Err())
		}
		return &llmclient.InvalidRequestError{
			Message: fmt.Sprintf(
				"codexcli: ping: the codex CLI ran for %s without the expected local empty-instructions-file rejection — the undocumented model_instructions_file config key this adapter's Request.System mapping and token floor depend on may have been silently dropped by a CLI upgrade; re-verify per foundations/llmclient/codexcli/testdata/PROBES.md before trusting this adapter",
				pingTimeout,
			),
		}
	}

	if res.ExitCode != 0 && bytes.Contains(res.Stderr, []byte(pingHealthyMarker)) {
		return nil
	}

	return &llmclient.InvalidRequestError{
		Message: fmt.Sprintf(
			"codexcli: ping: the codex CLI did not reject an empty model_instructions_file (exit code %d, stderr: %q) — the undocumented config keys this adapter depends on (Request.System mapping, ambient-context suppression) may have been silently dropped by a CLI upgrade: System prompts would be silently lost and the CLI's ~13k-token built-in persona would silently return on every call; re-verify per foundations/llmclient/codexcli/testdata/PROBES.md and re-pin before trusting this adapter",
			res.ExitCode, tail(res.Stderr, maxErrorTailBytes),
		),
	}
}
