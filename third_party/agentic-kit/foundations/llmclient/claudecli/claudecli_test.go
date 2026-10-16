package claudecli_test

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/geoffmilleraz/fathom/third_party/agentic-kit/foundations/llmclient"
	"github.com/geoffmilleraz/fathom/third_party/agentic-kit/foundations/llmclient/claudecli"
)

// fakeClaudeScript is a hermetic stand-in for the `claude` binary. It never
// touches the network or any real credential store: it records its argv and
// stdin to files, then emits a canned response controlled entirely by
// environment variables, so every test below is deterministic and offline.
//
// FAKE_CLAUDE_ARGV_FILE  — one argv element per line is appended here.
// FAKE_CLAUDE_STDIN_FILE — the child's stdin is copied here verbatim.
// FAKE_CLAUDE_RESPONSE   — written to stdout verbatim.
// FAKE_CLAUDE_STDERR     — written to stderr verbatim.
// FAKE_CLAUDE_EXIT_CODE  — the process exit code (default 0).
// FAKE_CLAUDE_SLEEP_SECONDS — sleep this long before responding (for the
//
//	ctx-timeout test).
const fakeClaudeScript = `#!/bin/sh
if [ -n "$FAKE_CLAUDE_ARGV_FILE" ]; then
  : > "$FAKE_CLAUDE_ARGV_FILE"
  for a in "$@"; do
    printf '%s\n' "$a" >> "$FAKE_CLAUDE_ARGV_FILE"
  done
fi
if [ -n "$FAKE_CLAUDE_STDIN_FILE" ]; then
  cat > "$FAKE_CLAUDE_STDIN_FILE"
else
  cat > /dev/null
fi
if [ -n "$FAKE_CLAUDE_SLEEP_SECONDS" ]; then
  sleep "$FAKE_CLAUDE_SLEEP_SECONDS"
fi
if [ -n "$FAKE_CLAUDE_STDERR" ]; then
  printf '%s' "$FAKE_CLAUDE_STDERR" >&2
fi
if [ -n "$FAKE_CLAUDE_RESPONSE" ]; then
  printf '%s' "$FAKE_CLAUDE_RESPONSE"
fi
exit "${FAKE_CLAUDE_EXIT_CODE:-0}"
`

// newFakeClaude writes fakeClaudeScript into t.TempDir() and returns its
// absolute path, ready to use as claudecli.Config.Bin.
func newFakeClaude(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "claude")
	if err := os.WriteFile(path, []byte(fakeClaudeScript), 0o755); err != nil {
		t.Fatalf("write fake claude script: %v", err)
	}
	return path
}

// readArgv reads the argv captured by the fake shim at path, one element per
// line (trailing newline stripped). An absent file (the shim was never
// invoked) is a test failure — call this only after a Complete call that
// should have spawned the shim.
func readArgv(t *testing.T, path string) []string {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read captured argv: %v", err)
	}
	s := strings.TrimSuffix(string(raw), "\n")
	if s == "" {
		return nil
	}
	return strings.Split(s, "\n")
}

// newHappyPathConfig wires the fake shim, plus the argv/stdin capture file
// paths, as environment variables read by fakeClaudeScript. Returns the
// Client and the paths so the caller can assert on captured argv/stdin.
func newHappyPathConfig(t *testing.T, cfg claudecli.Config, response string) (*claudecli.Client, string, string) {
	t.Helper()
	argvFile := filepath.Join(t.TempDir(), "argv.txt")
	stdinFile := filepath.Join(t.TempDir(), "stdin.txt")
	t.Setenv("FAKE_CLAUDE_ARGV_FILE", argvFile)
	t.Setenv("FAKE_CLAUDE_STDIN_FILE", stdinFile)
	t.Setenv("FAKE_CLAUDE_RESPONSE", response)
	t.Setenv("FAKE_CLAUDE_EXIT_CODE", "0")

	if cfg.Bin == "" {
		cfg.Bin = newFakeClaude(t)
	}
	c, err := claudecli.New(cfg)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	return c, argvFile, stdinFile
}

const successEnvelope = `{"type":"result","subtype":"success","is_error":false,"api_error_status":null,"result":"pong","stop_reason":"end_turn","session_id":"s1","total_cost_usd":0.01,"usage":{"input_tokens":10,"output_tokens":5,"cache_creation_input_tokens":100,"cache_read_input_tokens":0}}`

func TestNew_ResolvesBin(t *testing.T) {
	bin := newFakeClaude(t)
	c, err := claudecli.New(claudecli.Config{Bin: bin})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if c == nil {
		t.Fatal("New returned a nil Client with a nil error")
	}
}

func TestNew_BinNotFound(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "does-not-exist-claude")
	_, err := claudecli.New(claudecli.Config{Bin: missing})
	if err == nil {
		t.Fatal("expected an error resolving a nonexistent Bin, got nil")
	}
}

func TestComplete_HappyPath_ArgvStdinAndResponseMapping(t *testing.T) {
	c, argvFile, stdinFile := newHappyPathConfig(t, claudecli.Config{Model: "haiku"}, successEnvelope)

	resp, err := c.Complete(context.Background(), llmclient.Request{Prompt: "ping"})
	if err != nil {
		t.Fatalf("Complete: %v", err)
	}

	wantArgv := []string{"-p", "--output-format", "json", "--safe-mode", "--tools", "", "--model", "haiku"}
	if got := readArgv(t, argvFile); !equalSlices(got, wantArgv) {
		t.Errorf("argv = %q, want %q", got, wantArgv)
	}

	stdin, err := os.ReadFile(stdinFile)
	if err != nil {
		t.Fatalf("read captured stdin: %v", err)
	}
	if string(stdin) != "ping" {
		t.Errorf("stdin = %q, want %q", stdin, "ping")
	}

	if resp.Text != "pong" {
		t.Errorf("Text = %q, want %q", resp.Text, "pong")
	}
	if len(resp.Content) != 1 || resp.Content[0].Type != llmclient.BlockText || resp.Content[0].Text != "pong" {
		t.Errorf("Content = %+v, want a single BlockText(%q)", resp.Content, "pong")
	}
	if resp.StopReason != llmclient.StopEndTurn {
		t.Errorf("StopReason = %q, want %q", resp.StopReason, llmclient.StopEndTurn)
	}
	wantUsage := llmclient.Usage{InputTokens: 10, OutputTokens: 5, CacheCreationInputTokens: 100, CacheReadInputTokens: 0}
	if resp.Usage != wantUsage {
		t.Errorf("Usage = %+v, want %+v", resp.Usage, wantUsage)
	}
}

func TestComplete_RequestModelOverridesConfigModel(t *testing.T) {
	c, argvFile, _ := newHappyPathConfig(t, claudecli.Config{Model: "haiku"}, successEnvelope)

	if _, err := c.Complete(context.Background(), llmclient.Request{Prompt: "ping", Model: "opus"}); err != nil {
		t.Fatalf("Complete: %v", err)
	}

	argv := readArgv(t, argvFile)
	if !containsPair(argv, "--model", "opus") {
		t.Errorf("argv %q does not contain --model opus (Request.Model should override Config.Model)", argv)
	}
	if containsPair(argv, "--model", "haiku") {
		t.Errorf("argv %q contains --model haiku; Config.Model should have been overridden", argv)
	}
}

func TestComplete_NoModel_OmitsModelFlag(t *testing.T) {
	c, argvFile, _ := newHappyPathConfig(t, claudecli.Config{}, successEnvelope)

	if _, err := c.Complete(context.Background(), llmclient.Request{Prompt: "ping"}); err != nil {
		t.Fatalf("Complete: %v", err)
	}

	argv := readArgv(t, argvFile)
	for _, a := range argv {
		if a == "--model" {
			t.Errorf("argv %q contains --model even though neither Request.Model nor Config.Model was set", argv)
		}
	}
}

func TestComplete_SystemPromptFlagOmittedWhenEmpty(t *testing.T) {
	c, argvFile, _ := newHappyPathConfig(t, claudecli.Config{}, successEnvelope)

	if _, err := c.Complete(context.Background(), llmclient.Request{Prompt: "ping"}); err != nil {
		t.Fatalf("Complete: %v", err)
	}

	argv := readArgv(t, argvFile)
	for _, a := range argv {
		if a == "--system-prompt" {
			t.Errorf("argv %q contains --system-prompt even though Request.System was empty", argv)
		}
	}
}

func TestComplete_SystemPromptPassedThrough(t *testing.T) {
	c, argvFile, _ := newHappyPathConfig(t, claudecli.Config{}, successEnvelope)

	if _, err := c.Complete(context.Background(), llmclient.Request{Prompt: "ping", System: "be terse"}); err != nil {
		t.Fatalf("Complete: %v", err)
	}

	argv := readArgv(t, argvFile)
	if !containsPair(argv, "--system-prompt", "be terse") {
		t.Errorf("argv %q does not contain --system-prompt \"be terse\"", argv)
	}
}

func TestComplete_EffortPassthrough(t *testing.T) {
	c, argvFile, _ := newHappyPathConfig(t, claudecli.Config{}, successEnvelope)

	if _, err := c.Complete(context.Background(), llmclient.Request{Prompt: "ping", Effort: "high"}); err != nil {
		t.Fatalf("Complete: %v", err)
	}

	argv := readArgv(t, argvFile)
	if !containsPair(argv, "--effort", "high") {
		t.Errorf("argv %q does not contain --effort high", argv)
	}
}

func TestComplete_OutputSchema_PassesJSONSchemaFlagAndRemapsToolUse(t *testing.T) {
	envelope := `{"type":"result","subtype":"success","is_error":false,"api_error_status":null,"result":"{\"name\":\"a\"}","stop_reason":"tool_use","usage":{"input_tokens":1,"output_tokens":1,"cache_creation_input_tokens":0,"cache_read_input_tokens":0}}`
	c, argvFile, _ := newHappyPathConfig(t, claudecli.Config{}, envelope)

	schema := []byte(`{"type":"object"}`)
	resp, err := c.Complete(context.Background(), llmclient.Request{Prompt: "ping", OutputSchema: schema})
	if err != nil {
		t.Fatalf("Complete: %v", err)
	}

	argv := readArgv(t, argvFile)
	if !containsPair(argv, "--json-schema", string(schema)) {
		t.Errorf("argv %q does not contain --json-schema %s", argv, schema)
	}

	// The raw envelope's stop_reason is "tool_use" (the CLI's internal
	// structured-output mechanism), which must be remapped to StopEndTurn —
	// see the package doc and envelope.go's mapStopReason.
	if resp.StopReason != llmclient.StopEndTurn {
		t.Errorf("StopReason = %q, want %q (tool_use must be remapped when OutputSchema was requested)", resp.StopReason, llmclient.StopEndTurn)
	}
}

func TestComplete_StopReason_PassthroughWithoutOutputSchema(t *testing.T) {
	envelope := `{"type":"result","subtype":"success","is_error":false,"api_error_status":null,"result":"cut off","stop_reason":"max_tokens","usage":{"input_tokens":1,"output_tokens":1,"cache_creation_input_tokens":0,"cache_read_input_tokens":0}}`
	c, _, _ := newHappyPathConfig(t, claudecli.Config{}, envelope)

	resp, err := c.Complete(context.Background(), llmclient.Request{Prompt: "ping"})
	if err != nil {
		t.Fatalf("Complete: %v", err)
	}
	if resp.StopReason != llmclient.StopMaxTokens {
		t.Errorf("StopReason = %q, want %q", resp.StopReason, llmclient.StopMaxTokens)
	}
}

func TestComplete_Messages_TextOnlyFlattened(t *testing.T) {
	c, _, stdinFile := newHappyPathConfig(t, claudecli.Config{}, successEnvelope)

	req := llmclient.Request{
		Messages: []llmclient.Message{
			{Role: llmclient.RoleUser, Content: []llmclient.ContentBlock{llmclient.TextBlock("hello")}},
			{Role: llmclient.RoleAssistant, Content: []llmclient.ContentBlock{llmclient.TextBlock("hi")}},
		},
	}
	if _, err := c.Complete(context.Background(), req); err != nil {
		t.Fatalf("Complete: %v", err)
	}

	stdin, err := os.ReadFile(stdinFile)
	if err != nil {
		t.Fatalf("read captured stdin: %v", err)
	}
	want := "user: hello\n\nassistant: hi"
	if string(stdin) != want {
		t.Errorf("stdin = %q, want %q", stdin, want)
	}
}

func TestComplete_Messages_NonTextBlockRejected(t *testing.T) {
	c, _, _ := newHappyPathConfig(t, claudecli.Config{}, successEnvelope)

	req := llmclient.Request{
		Messages: []llmclient.Message{
			{Role: llmclient.RoleAssistant, Content: []llmclient.ContentBlock{llmclient.ToolUseBlock("id1", "tool", nil)}},
		},
	}
	_, err := c.Complete(context.Background(), req)
	var invalidErr *llmclient.InvalidRequestError
	if !errors.As(err, &invalidErr) {
		t.Fatalf("Complete err = %v (%T), want *llmclient.InvalidRequestError", err, err)
	}
}

func TestComplete_UnsupportedFeaturesRejected(t *testing.T) {
	c, _, _ := newHappyPathConfig(t, claudecli.Config{}, successEnvelope)

	cases := []struct {
		name string
		req  llmclient.Request
	}{
		{"Tools", llmclient.Request{Prompt: "p", Tools: []llmclient.ToolDef{{Name: "t"}}}},
		{"Thinking", llmclient.Request{Prompt: "p", Thinking: &llmclient.ThinkingConfig{Type: "enabled"}}},
		{"Temperature", llmclient.Request{Prompt: "p", Temperature: 0.5}},
		{"StopSequences", llmclient.Request{Prompt: "p", StopSequences: []string{"STOP"}}},
		{"ProviderParams", llmclient.Request{Prompt: "p", ProviderParams: []byte(`{"foo":"bar"}`)}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := c.Complete(context.Background(), tc.req)
			var invalidErr *llmclient.InvalidRequestError
			if !errors.As(err, &invalidErr) {
				t.Fatalf("Complete err = %v (%T), want *llmclient.InvalidRequestError", err, err)
			}
		})
	}
}

func TestComplete_IsErrorMappedByAPIErrorStatus(t *testing.T) {
	cases := []struct {
		name     string
		status   int
		wantType string
	}{
		{"unauthorized", 401, "auth"},
		{"forbidden", 403, "auth"},
		{"rate limited", 429, "rateLimit"},
		{"server error", 500, "server"},
		{"bad request", 400, "invalidRequest"},
		{"not found", 404, "invalidRequest"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			envelope := fmt.Sprintf(`{"type":"result","subtype":"success","is_error":true,"api_error_status":%d,"result":"boom","stop_reason":"stop_sequence","usage":{"input_tokens":0,"output_tokens":0,"cache_creation_input_tokens":0,"cache_read_input_tokens":0}}`, tc.status)
			argvFile := filepath.Join(t.TempDir(), "argv.txt")
			t.Setenv("FAKE_CLAUDE_ARGV_FILE", argvFile)
			t.Setenv("FAKE_CLAUDE_RESPONSE", envelope)
			t.Setenv("FAKE_CLAUDE_EXIT_CODE", "1")
			c, err := claudecli.New(claudecli.Config{Bin: newFakeClaude(t)})
			if err != nil {
				t.Fatalf("New: %v", err)
			}

			_, err = c.Complete(context.Background(), llmclient.Request{Prompt: "ping"})
			if err == nil {
				t.Fatal("expected an error, got nil")
			}

			var matched bool
			switch tc.wantType {
			case "auth":
				var e *llmclient.AuthError
				matched = errors.As(err, &e)
			case "rateLimit":
				var e *llmclient.RateLimitError
				matched = errors.As(err, &e)
			case "server":
				var e *llmclient.ServerError
				matched = errors.As(err, &e)
			case "invalidRequest":
				var e *llmclient.InvalidRequestError
				matched = errors.As(err, &e)
			}
			if !matched {
				t.Errorf("Complete err = %v (%T), want it to be a typed %q error", err, err, tc.wantType)
			}
			if !strings.Contains(err.Error(), "boom") {
				t.Errorf("Complete err = %v, want it to contain the envelope's result text %q", err, "boom")
			}
		})
	}
}

func TestComplete_IsErrorWithoutAPIErrorStatus_PlainError(t *testing.T) {
	envelope := `{"type":"result","subtype":"success","is_error":true,"api_error_status":null,"result":"local failure","stop_reason":"","usage":{"input_tokens":0,"output_tokens":0,"cache_creation_input_tokens":0,"cache_read_input_tokens":0}}`
	t.Setenv("FAKE_CLAUDE_RESPONSE", envelope)
	t.Setenv("FAKE_CLAUDE_EXIT_CODE", "1")
	c, err := claudecli.New(claudecli.Config{Bin: newFakeClaude(t)})
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	_, err = c.Complete(context.Background(), llmclient.Request{Prompt: "ping"})
	if err == nil {
		t.Fatal("expected an error, got nil")
	}
	var authErr *llmclient.AuthError
	if errors.As(err, &authErr) {
		t.Errorf("Complete err = %v, should NOT be a typed llmclient error when api_error_status is absent", err)
	}
	if !strings.Contains(err.Error(), "local failure") {
		t.Errorf("Complete err = %v, want it to contain %q", err, "local failure")
	}
}

func TestComplete_NonZeroExit_NonJSONStdout_UsesStderrTail(t *testing.T) {
	t.Setenv("FAKE_CLAUDE_RESPONSE", "not json at all")
	t.Setenv("FAKE_CLAUDE_STDERR", "permission denied opening credential store")
	t.Setenv("FAKE_CLAUDE_EXIT_CODE", "1")
	c, err := claudecli.New(claudecli.Config{Bin: newFakeClaude(t)})
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	_, err = c.Complete(context.Background(), llmclient.Request{Prompt: "ping"})
	if err == nil {
		t.Fatal("expected an error, got nil")
	}
	if !strings.Contains(err.Error(), "permission denied opening credential store") {
		t.Errorf("Complete err = %v, want it to contain the stderr tail", err)
	}
}

func TestComplete_MalformedJSON_ExitZero(t *testing.T) {
	t.Setenv("FAKE_CLAUDE_RESPONSE", "{not valid json")
	t.Setenv("FAKE_CLAUDE_EXIT_CODE", "0")
	c, err := claudecli.New(claudecli.Config{Bin: newFakeClaude(t)})
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	_, err = c.Complete(context.Background(), llmclient.Request{Prompt: "ping"})
	if err == nil {
		t.Fatal("expected a malformed-envelope error, got nil")
	}
	if !strings.Contains(err.Error(), "malformed response envelope") {
		t.Errorf("Complete err = %v, want it to mention a malformed envelope", err)
	}
}

func TestComplete_CtxTimeoutKillsChild(t *testing.T) {
	t.Setenv("FAKE_CLAUDE_SLEEP_SECONDS", "30")
	t.Setenv("FAKE_CLAUDE_RESPONSE", successEnvelope)
	c, err := claudecli.New(claudecli.Config{Bin: newFakeClaude(t), Timeout: 200 * time.Millisecond})
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	start := time.Now()
	_, err = c.Complete(context.Background(), llmclient.Request{Prompt: "ping"})
	elapsed := time.Since(start)

	if err == nil {
		t.Fatal("expected a timeout error, got nil")
	}
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("Complete err = %v, want it to wrap context.DeadlineExceeded", err)
	}
	if elapsed > 10*time.Second {
		t.Errorf("Complete took %s, want it to return promptly once Timeout (200ms) elapsed — the child was not killed", elapsed)
	}
}

func TestComplete_CallerCtxCancelKillsChild(t *testing.T) {
	t.Setenv("FAKE_CLAUDE_SLEEP_SECONDS", "30")
	t.Setenv("FAKE_CLAUDE_RESPONSE", successEnvelope)
	c, err := claudecli.New(claudecli.Config{Bin: newFakeClaude(t)})
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		time.Sleep(100 * time.Millisecond)
		cancel()
	}()

	start := time.Now()
	_, err = c.Complete(ctx, llmclient.Request{Prompt: "ping"})
	elapsed := time.Since(start)

	if err == nil {
		t.Fatal("expected a cancellation error, got nil")
	}
	if !errors.Is(err, context.Canceled) {
		t.Errorf("Complete err = %v, want it to wrap context.Canceled", err)
	}
	if elapsed > 10*time.Second {
		t.Errorf("Complete took %s, want it to return promptly once ctx was cancelled — the child was not killed", elapsed)
	}
}

func TestStream_DelegatesToComplete_HappyPath(t *testing.T) {
	c, _, _ := newHappyPathConfig(t, claudecli.Config{}, successEnvelope)

	ch, err := c.Stream(context.Background(), llmclient.Request{Prompt: "ping"})
	if err != nil {
		t.Fatalf("Stream: %v", err)
	}

	var text string
	var sawTerminal bool
	for chunk := range ch {
		if chunk.Err != nil {
			t.Fatalf("unexpected chunk error: %v", chunk.Err)
		}
		if chunk.TextDelta != "" {
			text += chunk.TextDelta
		}
		if chunk.StopReason != "" {
			sawTerminal = true
			if chunk.Usage == nil {
				t.Error("terminal chunk missing Usage")
			}
		}
	}
	if text != "pong" {
		t.Errorf("assembled text = %q, want %q", text, "pong")
	}
	if !sawTerminal {
		t.Error("never saw a terminal chunk")
	}
}

func TestStream_DelegatesToComplete_Error(t *testing.T) {
	t.Setenv("FAKE_CLAUDE_RESPONSE", "not json")
	t.Setenv("FAKE_CLAUDE_EXIT_CODE", "1")
	c, err := claudecli.New(claudecli.Config{Bin: newFakeClaude(t)})
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	ch, err := c.Stream(context.Background(), llmclient.Request{Prompt: "ping"})
	if err != nil {
		t.Fatalf("Stream: %v", err)
	}

	var sawErr bool
	for chunk := range ch {
		if chunk.Err != nil {
			sawErr = true
		}
	}
	if !sawErr {
		t.Error("expected an error chunk, got none")
	}
}

func TestPing_Healthy_NoInputRejectedLocally(t *testing.T) {
	// The healthy shape, pinned from live probes A-F (testdata/PROBES.md): a
	// CLI given the exact pinned argv, empty stdin, and no prompt argument
	// rejects the invocation with a local stderr error and exit 1 before
	// doing anything else.
	argvFile := filepath.Join(t.TempDir(), "argv.txt")
	stdinFile := filepath.Join(t.TempDir(), "stdin.txt")
	t.Setenv("FAKE_CLAUDE_ARGV_FILE", argvFile)
	t.Setenv("FAKE_CLAUDE_STDIN_FILE", stdinFile)
	t.Setenv("FAKE_CLAUDE_RESPONSE", "")
	t.Setenv("FAKE_CLAUDE_STDERR", "Error: Input must be provided either through stdin or as a prompt argument when using --print")
	t.Setenv("FAKE_CLAUDE_EXIT_CODE", "1")

	c, err := claudecli.New(claudecli.Config{Bin: newFakeClaude(t)})
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	if err := c.Ping(context.Background()); err != nil {
		t.Fatalf("Ping = %v, want nil (the local no-input rejection IS the healthy outcome)", err)
	}

	// Ping runs the exact pinned production argv for a zero-valued Request
	// (no -m, no --system-prompt, no --effort, no --json-schema).
	argv := readArgv(t, argvFile)
	want := []string{"-p", "--output-format", "json", "--safe-mode", "--tools", ""}
	if !equalSlices(argv, want) {
		t.Errorf("Ping argv = %q, want %q", argv, want)
	}

	// No prompt is ever sent.
	stdin, err := os.ReadFile(stdinFile)
	if err != nil {
		t.Fatalf("read captured stdin: %v", err)
	}
	if len(stdin) != 0 {
		t.Errorf("Ping stdin = %q, want empty", stdin)
	}
}

func TestPing_Degraded_NoLocalRejection(t *testing.T) {
	// The degradation shapes: a future CLI that silently started accepting
	// empty-stdin/no-prompt input would NOT reject it — it would proceed
	// (exit 0 with a normal envelope) or fail some other, unrelated way.
	// Both must surface as a typed InvalidRequestError.
	cases := []struct {
		name     string
		response string
		stderr   string
		exitCode string
	}{
		{"cli proceeds with a normal response", successEnvelope, "", "0"},
		{"cli fails for an unrelated reason", "", "some unrelated failure", "1"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("FAKE_CLAUDE_RESPONSE", tc.response)
			t.Setenv("FAKE_CLAUDE_STDERR", tc.stderr)
			t.Setenv("FAKE_CLAUDE_EXIT_CODE", tc.exitCode)

			c, err := claudecli.New(claudecli.Config{Bin: newFakeClaude(t)})
			if err != nil {
				t.Fatalf("New: %v", err)
			}

			err = c.Ping(context.Background())
			if err == nil {
				t.Fatal("Ping = nil, want a typed degradation error when the CLI does not reject the empty-stdin, no-prompt-argument invocation")
			}
			var invalidErr *llmclient.InvalidRequestError
			if !errors.As(err, &invalidErr) {
				t.Fatalf("Ping err = %v (%T), want *llmclient.InvalidRequestError", err, err)
			}
			if !strings.Contains(err.Error(), "PROBES.md") {
				t.Errorf("Ping err = %v, want it to point at testdata/PROBES.md for re-verification", err)
			}
		})
	}
}

func TestPing_Degraded_HangKilledByTimeout(t *testing.T) {
	// A degraded CLI that hangs (e.g. waiting on the network despite empty
	// stdin) is killed by the ping bound and reported as the same typed
	// degradation error — never as a silent pass and never as an unbounded
	// wait.
	t.Setenv("FAKE_CLAUDE_SLEEP_SECONDS", "30")
	t.Setenv("FAKE_CLAUDE_RESPONSE", "")
	c, err := claudecli.New(claudecli.Config{Bin: newFakeClaude(t), Timeout: 200 * time.Millisecond})
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	start := time.Now()
	err = c.Ping(context.Background())
	elapsed := time.Since(start)

	if err == nil {
		t.Fatal("Ping = nil, want a typed degradation error on timeout")
	}
	var invalidErr *llmclient.InvalidRequestError
	if !errors.As(err, &invalidErr) {
		t.Fatalf("Ping err = %v (%T), want *llmclient.InvalidRequestError", err, err)
	}
	if elapsed > 10*time.Second {
		t.Errorf("Ping took %s, want it to return promptly once the bound elapsed", elapsed)
	}
}

func TestPing_CallerCtxCancelPassesThroughUntyped(t *testing.T) {
	// Per the Pinger contract, caller-chosen stops pass through untyped so
	// callers can distinguish their own cancellation from a provider failure.
	t.Setenv("FAKE_CLAUDE_SLEEP_SECONDS", "30")
	t.Setenv("FAKE_CLAUDE_RESPONSE", "")
	c, err := claudecli.New(claudecli.Config{Bin: newFakeClaude(t)})
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		time.Sleep(100 * time.Millisecond)
		cancel()
	}()

	err = c.Ping(ctx)
	if err == nil {
		t.Fatal("Ping = nil, want a cancellation error")
	}
	if !errors.Is(err, context.Canceled) {
		t.Errorf("Ping err = %v, want it to wrap context.Canceled", err)
	}
	var invalidErr *llmclient.InvalidRequestError
	if errors.As(err, &invalidErr) {
		t.Errorf("Ping err = %v, must NOT be typed for a caller-chosen stop", err)
	}
}

// equalSlices reports whether a and b contain the same elements in the same
// order.
func equalSlices(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// containsPair reports whether argv contains flag immediately followed by
// value.
func containsPair(argv []string, flag, value string) bool {
	for i := 0; i+1 < len(argv); i++ {
		if argv[i] == flag && argv[i+1] == value {
			return true
		}
	}
	return false
}
