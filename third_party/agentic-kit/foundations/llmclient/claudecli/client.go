package claudecli

import (
	"context"
	"fmt"
	"strings"

	boundedexec "github.com/geoffmilleraz/fathom/third_party/agentic-kit/foundations/agent/bounded-exec"
	"github.com/geoffmilleraz/fathom/third_party/agentic-kit/foundations/llmclient"
)

// maxErrorTailBytes bounds how much of a malformed-envelope stderr capture is
// included in the returned error message.
const maxErrorTailBytes = 4096

// Complete runs one `claude -p --output-format json ...` invocation with the
// request's prompt on stdin, and maps the resulting envelope to
// llmclient.Response. See the package doc for the full invocation shape,
// the error-mapping rules, and the documented degradations.
func (c *Client) Complete(ctx context.Context, req llmclient.Request) (llmclient.Response, error) {
	if err := validateRequest(req); err != nil {
		return llmclient.Response{}, err
	}
	if err := llmclient.ValidateProviderParams(req.ProviderParams, "claudecli"); err != nil {
		return llmclient.Response{}, err
	}
	if len(req.ProviderParams) > 0 {
		// Validated above for JSON-object shape; there is no wire body for a
		// CLI shell-out to merge it into (see the package doc). Fail loud
		// rather than silently drop it.
		return llmclient.Response{}, &llmclient.InvalidRequestError{
			Message: "claudecli: Request.ProviderParams is not supported — there is no outbound wire body for a CLI shell-out to merge it into",
		}
	}
	if err := llmclient.ValidateOutputSchema(req.OutputSchema, "claudecli"); err != nil {
		return llmclient.Response{}, err
	}

	prompt, err := promptText(req)
	if err != nil {
		return llmclient.Response{}, err
	}

	args := c.buildArgs(req)

	res, err := c.runner.Run(ctx, boundedexec.Spec{
		Path:  c.bin,
		Args:  args,
		Stdin: []byte(prompt),
	})
	if err != nil {
		// Either ctx was canceled (err wraps context.Canceled) or the
		// subprocess failed to start/wait at all.
		return llmclient.Response{}, fmt.Errorf("claudecli: %w", err)
	}
	if res.TimedOut {
		return llmclient.Response{}, fmt.Errorf("claudecli: %w", context.DeadlineExceeded)
	}

	env, parseErr := parseEnvelope(res.Stdout)
	if parseErr != nil {
		return llmclient.Response{}, malformedEnvelopeErr(parseErr, res)
	}

	if env.IsError {
		return llmclient.Response{}, mapEnvelopeError(env)
	}

	usedOutputSchema := len(req.OutputSchema) > 0
	return llmclient.Response{
		Text:       env.Result,
		Content:    []llmclient.ContentBlock{llmclient.TextBlock(env.Result)},
		StopReason: mapStopReason(env.StopReason, usedOutputSchema),
		Usage:      env.Usage.toUsage(),
	}, nil
}

// malformedEnvelopeErr wraps parseErr with the exit code and (when present) a
// bounded tail of stderr — the "malformed envelope" / "non-zero exit" error
// surface documented in the package doc.
func malformedEnvelopeErr(parseErr error, res boundedexec.Result) error {
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
			Message: "claudecli: Request.Tools is not supported — every call runs with --tools \"\" so no caller-declared tool can be exposed to the model",
		}
	case req.Thinking != nil:
		return &llmclient.InvalidRequestError{
			Message: "claudecli: Request.Thinking is not supported — the installed CLI exposes no extended-thinking flag",
		}
	case req.Temperature != 0:
		return &llmclient.InvalidRequestError{
			Message: "claudecli: Request.Temperature is not supported — the installed CLI exposes no temperature flag",
		}
	case len(req.StopSequences) > 0:
		return &llmclient.InvalidRequestError{
			Message: "claudecli: Request.StopSequences is not supported — the installed CLI exposes no stop-sequence flag",
		}
	}
	return nil
}

// promptText resolves the effective stdin text for req. When Messages is
// empty, Prompt is used verbatim (the llmclient v1 single-turn form). When
// Messages is non-empty it takes precedence (per the llmclient contract) and
// is flattened to a plain "role: text" transcript — every block must be
// BlockText; a tool_use/tool_result/image/unknown block returns a typed
// InvalidRequestError, since a CLI shell-out over a text pipe cannot
// round-trip provider-native tool state or attach binary content (see the
// package doc).
func promptText(req llmclient.Request) (string, error) {
	if len(req.Messages) == 0 {
		return req.Prompt, nil
	}

	var b strings.Builder
	for i, m := range req.Messages {
		for _, blk := range m.Content {
			if blk.Type != llmclient.BlockText {
				return "", &llmclient.InvalidRequestError{
					Message: fmt.Sprintf("claudecli: Request.Messages[%d] contains a %q block — only BlockText is supported", i, blk.Type),
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
// "Pinned invocation shape", for why each flag is present and in this order.
func (c *Client) buildArgs(req llmclient.Request) []string {
	args := []string{
		"-p",
		"--output-format", "json",
		"--safe-mode",
		"--tools", "",
	}

	model := req.Model
	if model == "" {
		model = c.model
	}
	if model != "" {
		args = append(args, "--model", model)
	}

	if req.System != "" {
		args = append(args, "--system-prompt", req.System)
	}

	if req.Effort != "" {
		args = append(args, "--effort", req.Effort)
	}

	if len(req.OutputSchema) > 0 {
		args = append(args, "--json-schema", string(req.OutputSchema))
	}

	return args
}

// tail returns at most the last n bytes of b, for bounding how much raw
// subprocess output lands in an error message.
func tail(b []byte, n int) []byte {
	if len(b) <= n {
		return b
	}
	return b[len(b)-n:]
}
