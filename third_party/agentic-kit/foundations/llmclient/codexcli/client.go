package codexcli

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"

	boundedexec "github.com/geoffmilleraz/agentic-kit/foundations/agent/bounded-exec"
	"github.com/geoffmilleraz/agentic-kit/foundations/llmclient"
)

// maxErrorTailBytes bounds how much of a malformed-stream stderr capture is
// included in the returned error message, mirroring claudecli.
const maxErrorTailBytes = 4096

// defaultInstructions replaces the CLI's large built-in agentic-coding
// system preamble when Request.System is empty. See buildArgs and the
// package doc, "Request.System mapping", for why a non-empty file is always
// required (an empty model_instructions_file is a hard, local, pre-API
// error — verified live) and why this also happens to be the dominant lever
// in the ambient-context-suppression floor.
const defaultInstructions = "Respond directly to the user's message."

// Complete runs one `codex exec --json ...` invocation with the request's
// prompt on stdin, and reduces the resulting JSONL event stream to
// llmclient.Response. See the package doc for the full invocation shape, the
// event-reduction rules, and the documented degradations.
func (c *Client) Complete(ctx context.Context, req llmclient.Request) (llmclient.Response, error) {
	if err := validateRequest(req); err != nil {
		return llmclient.Response{}, err
	}
	if err := llmclient.ValidateProviderParams(req.ProviderParams, "codexcli"); err != nil {
		return llmclient.Response{}, err
	}
	if len(req.ProviderParams) > 0 {
		// Validated above for JSON-object shape; there is no wire body for a
		// CLI shell-out to merge it into (see the package doc). Fail loud
		// rather than silently drop it.
		return llmclient.Response{}, &llmclient.InvalidRequestError{
			Message: "codexcli: Request.ProviderParams is not supported — there is no outbound wire body for a CLI shell-out to merge it into",
		}
	}
	if err := llmclient.ValidateOutputSchema(req.OutputSchema, "codexcli"); err != nil {
		return llmclient.Response{}, err
	}

	prompt, err := promptText(req)
	if err != nil {
		return llmclient.Response{}, err
	}

	instructions := req.System
	if instructions == "" {
		instructions = defaultInstructions
	}
	instructionsPath, cleanupInstructions, err := writeScratchFile(c.scratchDir, "instructions-*.txt", []byte(instructions))
	if err != nil {
		return llmclient.Response{}, err
	}
	defer cleanupInstructions()

	var schemaPath string
	if len(req.OutputSchema) > 0 {
		var cleanupSchema func()
		schemaPath, cleanupSchema, err = writeScratchFile(c.scratchDir, "schema-*.json", req.OutputSchema)
		if err != nil {
			return llmclient.Response{}, err
		}
		defer cleanupSchema()
	}

	args := c.buildArgs(req, instructionsPath, schemaPath)

	res, err := c.runner.Run(ctx, boundedexec.Spec{
		Path:  c.bin,
		Args:  args,
		Stdin: []byte(prompt),
	})
	if err != nil {
		// Either ctx was canceled (err wraps context.Canceled) or the
		// subprocess failed to start/wait at all.
		return llmclient.Response{}, fmt.Errorf("codexcli: %w", err)
	}
	if res.TimedOut {
		return llmclient.Response{}, fmt.Errorf("codexcli: %w", context.DeadlineExceeded)
	}

	reduced, err := reduceEvents(res.Stdout)
	if err != nil {
		return llmclient.Response{}, malformedStreamErr(err, res)
	}
	if reduced.failure != nil {
		return llmclient.Response{}, reduced.failure
	}

	return llmclient.Response{
		Text:       reduced.text,
		Content:    []llmclient.ContentBlock{llmclient.TextBlock(reduced.text)},
		StopReason: llmclient.StopEndTurn,
		Usage:      reduced.usage,
	}, nil
}

// malformedStreamErr wraps parseErr with the exit code and (when present) a
// bounded tail of stderr — the "malformed/incomplete JSONL stream" error
// surface documented in the package doc.
func malformedStreamErr(parseErr error, res boundedexec.Result) error {
	truncNote := ""
	if res.StdoutTruncated {
		truncNote = " [stdout truncated]"
	}
	if len(res.Stderr) == 0 {
		return fmt.Errorf("%w (exit code %d)%s", parseErr, res.ExitCode, truncNote)
	}
	return fmt.Errorf("%w (exit code %d)%s; stderr: %s", parseErr, res.ExitCode, truncNote, tail(res.Stderr, maxErrorTailBytes))
}

// validateRequest rejects Request fields this adapter cannot honor — see the
// package doc, "Documented degradations", for why each fails loud instead of
// being silently ignored.
func validateRequest(req llmclient.Request) error {
	switch {
	case len(req.Tools) > 0:
		return &llmclient.InvalidRequestError{
			Message: "codexcli: Request.Tools is not supported — codex exec's own agentic tool surface (shell, apply_patch, ...) cannot be replaced or extended with caller-declared tools over this shell-out",
		}
	case req.Thinking != nil:
		return &llmclient.InvalidRequestError{
			Message: "codexcli: Request.Thinking is not supported — use Request.Effort (mapped to -c model_reasoning_effort) for this provider's reasoning-depth knob",
		}
	case req.Temperature != 0:
		return &llmclient.InvalidRequestError{
			Message: "codexcli: Request.Temperature is not supported — the installed CLI exposes no temperature flag",
		}
	case len(req.StopSequences) > 0:
		return &llmclient.InvalidRequestError{
			Message: "codexcli: Request.StopSequences is not supported — the installed CLI exposes no stop-sequence flag",
		}
	}
	return nil
}

// promptText resolves the effective stdin text for req, identically to
// claudecli's promptText (see that package doc for the full rationale): when
// Messages is empty, Prompt is used verbatim; when Messages is non-empty it
// takes precedence and is flattened to a plain "role: text" transcript, with
// any non-BlockText block rejected — a CLI shell-out over a text pipe cannot
// round-trip provider-native tool state or attach binary content.
func promptText(req llmclient.Request) (string, error) {
	if len(req.Messages) == 0 {
		return req.Prompt, nil
	}

	var b strings.Builder
	for i, m := range req.Messages {
		for _, blk := range m.Content {
			if blk.Type != llmclient.BlockText {
				return "", &llmclient.InvalidRequestError{
					Message: fmt.Sprintf("codexcli: Request.Messages[%d] contains a %q block — only BlockText is supported", i, blk.Type),
				}
			}
			if b.Len() > 0 {
				b.WriteString("\n\n")
			}
			b.WriteString(string(m.Role))
			b.WriteString(": ")
			b.WriteString(blk.Text)
		}
	}
	return b.String(), nil
}

// buildArgs builds the argv for one Complete call. See the package doc,
// "Pinned invocation shape", for the flag-by-flag ambient-context-suppression
// attribution and why each flag is present.
func (c *Client) buildArgs(req llmclient.Request, instructionsPath, schemaPath string) []string {
	args := []string{
		"exec",
		"--json",
		"-C", c.scratchDir,
		"--skip-git-repo-check",
		"--ignore-user-config",
		"--ephemeral",
		"-s", "read-only",
		"-c", tomlOverride("model_instructions_file", instructionsPath),
		"-c", "include_environment_context=false",
		"-c", "include_apps_instructions=false",
		"-c", "include_collaboration_mode_instructions=false",
		"-c", "include_permissions_instructions=false",
	}

	model := req.Model
	if model == "" {
		model = c.model
	}
	if model != "" {
		args = append(args, "-m", model)
	}

	if req.Effort != "" {
		args = append(args, "-c", tomlOverride("model_reasoning_effort", req.Effort))
	}

	if schemaPath != "" {
		args = append(args, "--output-schema", schemaPath)
	}

	return args
}

// tomlOverride formats a `-c key=value` override's value operand as a TOML
// basic (double-quoted) string, per --help: "The value portion is parsed as
// TOML. If it fails to parse as TOML, the raw string is used as a literal."
// Quoting explicitly avoids ever depending on that fallback (e.g. for a
// value that happens to parse as TOML on its own, like an unquoted "true" or
// a bare number). strconv.Quote's Go-style backslash/quote escaping is a
// compatible subset of TOML basic-string escaping for the ASCII paths and
// plain-text values (file paths, effort levels) this adapter ever quotes.
func tomlOverride(key, value string) string {
	return key + "=" + strconv.Quote(value)
}

// writeScratchFile writes content to a new file under dir named via
// pattern (see os.CreateTemp for pattern syntax) and returns its path plus a
// cleanup func that removes it. The cleanup func is always non-nil and safe
// to call even after a partial failure, so callers can unconditionally
// `defer cleanup()` — including on the error return path, where cleanup has
// already removed whatever was written (CX-5's "guaranteed cleanup,
// including on error").
func writeScratchFile(dir, pattern string, content []byte) (path string, cleanup func(), err error) {
	f, err := os.CreateTemp(dir, pattern)
	if err != nil {
		return "", func() {}, fmt.Errorf("codexcli: create scratch file: %w", err)
	}
	path = f.Name()
	cleanup = func() { os.Remove(path) }

	if _, err := f.Write(content); err != nil {
		f.Close()
		cleanup()
		return "", func() {}, fmt.Errorf("codexcli: write scratch file: %w", err)
	}
	if err := f.Close(); err != nil {
		cleanup()
		return "", func() {}, fmt.Errorf("codexcli: close scratch file: %w", err)
	}
	return path, cleanup, nil
}

// tail returns at most the last n bytes of b, for bounding how much raw
// subprocess output lands in an error message.
func tail(b []byte, n int) []byte {
	if len(b) <= n {
		return b
	}
	return b[len(b)-n:]
}
