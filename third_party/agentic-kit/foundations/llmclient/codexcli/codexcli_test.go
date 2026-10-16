package codexcli_test

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/geoffmilleraz/fathom/third_party/agentic-kit/foundations/llmclient"
	"github.com/geoffmilleraz/fathom/third_party/agentic-kit/foundations/llmclient/codexcli"
)

// fakeCodexScript is a hermetic stand-in for the `codex` binary. It never
// touches the network or any real credential store: it records its argv and
// stdin to files, optionally captures the CONTENT of the instructions/schema
// temp files it was pointed at (while they still exist, mid-run — Complete's
// deferred cleanup only fires after this script exits), then emits a canned
// JSONL response controlled entirely by environment variables, so every
// test below is deterministic and offline.
//
// FAKE_CODEX_ARGV_FILE            — one argv element per line is appended here.
// FAKE_CODEX_STDIN_FILE           — the child's stdin is copied here verbatim.
// FAKE_CODEX_INSTRUCTIONS_CAPTURE — the content of the file referenced by
//
//	-c model_instructions_file="..." is copied here, only when that file
//	exists — so a test can distinguish "existed and was empty" (capture
//	present, zero bytes; the Ping shape) from "never written" (no capture).
//
// FAKE_CODEX_SCHEMA_CAPTURE       — the content of the file referenced by
//
//	--output-schema <file> is copied here.
//
// FAKE_CODEX_RESPONSE      — written to stdout verbatim (a full JSONL body).
// FAKE_CODEX_STDERR        — written to stderr verbatim.
// FAKE_CODEX_EXIT_CODE     — the process exit code (default 0).
// FAKE_CODEX_SLEEP_SECONDS — sleep this long before responding (for the
//
//	ctx-timeout test).
const fakeCodexScript = `#!/bin/sh
if [ -n "$FAKE_CODEX_ARGV_FILE" ]; then
  : > "$FAKE_CODEX_ARGV_FILE"
  for a in "$@"; do
    printf '%s\n' "$a" >> "$FAKE_CODEX_ARGV_FILE"
  done
fi

prev=""
for a in "$@"; do
  case "$a" in
    model_instructions_file=*)
      if [ -n "$FAKE_CODEX_INSTRUCTIONS_CAPTURE" ]; then
        val=$(printf '%s' "$a" | sed -e 's/^model_instructions_file=//' -e 's/^"//' -e 's/"$//')
        if [ -f "$val" ]; then
          cat "$val" > "$FAKE_CODEX_INSTRUCTIONS_CAPTURE"
        fi
      fi
      ;;
  esac
  if [ "$prev" = "--output-schema" ] && [ -n "$FAKE_CODEX_SCHEMA_CAPTURE" ]; then
    cat "$a" > "$FAKE_CODEX_SCHEMA_CAPTURE" 2>/dev/null || true
  fi
  prev="$a"
done

if [ -n "$FAKE_CODEX_STDIN_FILE" ]; then
  cat > "$FAKE_CODEX_STDIN_FILE"
else
  cat > /dev/null
fi
if [ -n "$FAKE_CODEX_SLEEP_SECONDS" ]; then
  sleep "$FAKE_CODEX_SLEEP_SECONDS"
fi
if [ -n "$FAKE_CODEX_STDERR" ]; then
  printf '%s' "$FAKE_CODEX_STDERR" >&2
fi
if [ -n "$FAKE_CODEX_RESPONSE" ]; then
  printf '%s' "$FAKE_CODEX_RESPONSE"
fi
exit "${FAKE_CODEX_EXIT_CODE:-0}"
`

// newFakeCodex writes fakeCodexScript into t.TempDir() and returns its
// absolute path, ready to use as codexcli.Config.Bin.
func newFakeCodex(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "codex")
	if err := os.WriteFile(path, []byte(fakeCodexScript), 0o755); err != nil {
		t.Fatalf("write fake codex script: %v", err)
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
// paths, as environment variables read by fakeCodexScript. Returns the
// Client and the paths so the caller can assert on captured argv/stdin.
// cfg.ScratchDir defaults to a fresh t.TempDir() when unset, so callers can
// assert on the scratch directory's contents (temp-file cleanup) too.
func newHappyPathConfig(t *testing.T, cfg codexcli.Config, response string) (*codexcli.Client, string, string) {
	t.Helper()
	argvFile := filepath.Join(t.TempDir(), "argv.txt")
	stdinFile := filepath.Join(t.TempDir(), "stdin.txt")
	t.Setenv("FAKE_CODEX_ARGV_FILE", argvFile)
	t.Setenv("FAKE_CODEX_STDIN_FILE", stdinFile)
	t.Setenv("FAKE_CODEX_RESPONSE", response)
	t.Setenv("FAKE_CODEX_EXIT_CODE", "0")

	if cfg.Bin == "" {
		cfg.Bin = newFakeCodex(t)
	}
	if cfg.ScratchDir == "" {
		cfg.ScratchDir = t.TempDir()
	}
	c, err := codexcli.New(cfg)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	return c, argvFile, stdinFile
}

// findConfigOverride returns the unquoted value of a `-c key="value"` pair
// in argv, or "" and false if key is not present. Mirrors how buildArgs
// formats a `-c` override (tomlOverride: key=strconv.Quote(value)).
func findConfigOverride(argv []string, key string) (string, bool) {
	prefix := key + "="
	for i := 0; i+1 < len(argv); i++ {
		if argv[i] != "-c" {
			continue
		}
		if strings.HasPrefix(argv[i+1], prefix) {
			v := strings.TrimPrefix(argv[i+1], prefix)
			v = strings.TrimPrefix(v, `"`)
			v = strings.TrimSuffix(v, `"`)
			return v, true
		}
	}
	return "", false
}

const singleMessageStream = `{"type":"thread.started","thread_id":"t1"}
{"type":"turn.started"}
{"type":"item.completed","item":{"id":"item_0","type":"agent_message","text":"pong"}}
{"type":"turn.completed","usage":{"input_tokens":10,"cached_input_tokens":3,"output_tokens":5,"reasoning_output_tokens":2}}
`

func TestNew_ResolvesBin(t *testing.T) {
	bin := newFakeCodex(t)
	c, err := codexcli.New(codexcli.Config{Bin: bin, ScratchDir: t.TempDir()})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if c == nil {
		t.Fatal("New returned a nil Client with a nil error")
	}
}

func TestNew_BinNotFound(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "does-not-exist-codex")
	_, err := codexcli.New(codexcli.Config{Bin: missing})
	if err == nil {
		t.Fatal("expected an error resolving a nonexistent Bin, got nil")
	}
}

func TestNew_CreatesScratchDirWhenUnset(t *testing.T) {
	bin := newFakeCodex(t)
	c, err := codexcli.New(codexcli.Config{Bin: bin})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if c == nil {
		t.Fatal("New returned a nil Client with a nil error")
	}
}

func TestComplete_HappyPath_ArgvStdinAndResponseMapping(t *testing.T) {
	scratch := t.TempDir()
	c, argvFile, stdinFile := newHappyPathConfig(t, codexcli.Config{Model: "gpt-5.4", ScratchDir: scratch}, singleMessageStream)

	resp, err := c.Complete(context.Background(), llmclient.Request{Prompt: "ping"})
	if err != nil {
		t.Fatalf("Complete: %v", err)
	}

	argv := readArgv(t, argvFile)

	instrValue, ok := findConfigOverride(argv, "model_instructions_file")
	if !ok {
		t.Fatalf("argv %q missing -c model_instructions_file=...", argv)
	}
	if !strings.HasPrefix(instrValue, scratch) {
		t.Errorf("model_instructions_file value %q is not under scratch dir %q", instrValue, scratch)
	}

	want := []string{
		"exec",
		"--json",
		"-C", scratch,
		"--skip-git-repo-check",
		"--ignore-user-config",
		"--ephemeral",
		"-s", "read-only",
		"-c", `model_instructions_file="` + instrValue + `"`,
		"-c", "include_environment_context=false",
		"-c", "include_apps_instructions=false",
		"-c", "include_collaboration_mode_instructions=false",
		"-c", "include_permissions_instructions=false",
		"-m", "gpt-5.4",
	}
	if !equalSlices(argv, want) {
		t.Errorf("argv = %q, want %q", argv, want)
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
	// input_tokens:10, cached_input_tokens:3 -> CacheReadInputTokens; output
	// (5) + reasoning (2) folded into OutputTokens; no cache-creation
	// equivalent exists in the event schema, so it stays zero.
	wantUsage := llmclient.Usage{InputTokens: 10, OutputTokens: 7, CacheCreationInputTokens: 0, CacheReadInputTokens: 3}
	if resp.Usage != wantUsage {
		t.Errorf("Usage = %+v, want %+v", resp.Usage, wantUsage)
	}

	// Both the instructions and (absent) schema temp files must be cleaned
	// up: the scratch dir holds nothing after Complete returns.
	assertDirEmpty(t, scratch)
}

func TestComplete_ArgvContract_NoModel_OmitsModelFlag(t *testing.T) {
	c, argvFile, _ := newHappyPathConfig(t, codexcli.Config{}, singleMessageStream)

	if _, err := c.Complete(context.Background(), llmclient.Request{Prompt: "ping"}); err != nil {
		t.Fatalf("Complete: %v", err)
	}

	argv := readArgv(t, argvFile)
	for _, a := range argv {
		if a == "-m" {
			t.Errorf("argv %q contains -m even though neither Request.Model nor Config.Model was set", argv)
		}
	}
}

func TestComplete_RequestModelOverridesConfigModel(t *testing.T) {
	c, argvFile, _ := newHappyPathConfig(t, codexcli.Config{Model: "gpt-5-mini"}, singleMessageStream)

	if _, err := c.Complete(context.Background(), llmclient.Request{Prompt: "ping", Model: "gpt-5.4"}); err != nil {
		t.Fatalf("Complete: %v", err)
	}

	argv := readArgv(t, argvFile)
	if !containsPair(argv, "-m", "gpt-5.4") {
		t.Errorf("argv %q does not contain -m gpt-5.4 (Request.Model should override Config.Model)", argv)
	}
	if containsPair(argv, "-m", "gpt-5-mini") {
		t.Errorf("argv %q contains -m gpt-5-mini; Config.Model should have been overridden", argv)
	}
}

func TestComplete_EffortPassthrough(t *testing.T) {
	c, argvFile, _ := newHappyPathConfig(t, codexcli.Config{}, singleMessageStream)

	if _, err := c.Complete(context.Background(), llmclient.Request{Prompt: "ping", Effort: "high"}); err != nil {
		t.Fatalf("Complete: %v", err)
	}

	argv := readArgv(t, argvFile)
	v, ok := findConfigOverride(argv, "model_reasoning_effort")
	if !ok || v != "high" {
		t.Errorf("argv %q does not contain -c model_reasoning_effort=\"high\" (got %q, ok=%v)", argv, v, ok)
	}
}

func TestComplete_NoEffort_OmitsEffortOverride(t *testing.T) {
	c, argvFile, _ := newHappyPathConfig(t, codexcli.Config{}, singleMessageStream)

	if _, err := c.Complete(context.Background(), llmclient.Request{Prompt: "ping"}); err != nil {
		t.Fatalf("Complete: %v", err)
	}

	argv := readArgv(t, argvFile)
	if _, ok := findConfigOverride(argv, "model_reasoning_effort"); ok {
		t.Errorf("argv %q contains model_reasoning_effort even though Request.Effort was empty", argv)
	}
}

func TestComplete_System_WritesRequestSystemToInstructionsFile(t *testing.T) {
	scratch := t.TempDir()
	captureFile := filepath.Join(t.TempDir(), "instr_capture.txt")
	t.Setenv("FAKE_CODEX_INSTRUCTIONS_CAPTURE", captureFile)

	c, _, _ := newHappyPathConfig(t, codexcli.Config{ScratchDir: scratch}, singleMessageStream)

	if _, err := c.Complete(context.Background(), llmclient.Request{Prompt: "ping", System: "be terse"}); err != nil {
		t.Fatalf("Complete: %v", err)
	}

	got, err := os.ReadFile(captureFile)
	if err != nil {
		t.Fatalf("read captured instructions content: %v", err)
	}
	if string(got) != "be terse" {
		t.Errorf("instructions file content = %q, want %q", got, "be terse")
	}
}

func TestComplete_System_Empty_UsesDefaultInstructions(t *testing.T) {
	scratch := t.TempDir()
	captureFile := filepath.Join(t.TempDir(), "instr_capture.txt")
	t.Setenv("FAKE_CODEX_INSTRUCTIONS_CAPTURE", captureFile)

	c, _, _ := newHappyPathConfig(t, codexcli.Config{ScratchDir: scratch}, singleMessageStream)

	if _, err := c.Complete(context.Background(), llmclient.Request{Prompt: "ping"}); err != nil {
		t.Fatalf("Complete: %v", err)
	}

	got, err := os.ReadFile(captureFile)
	if err != nil {
		t.Fatalf("read captured instructions content: %v", err)
	}
	if len(got) == 0 {
		t.Error("instructions file content is empty — codex hard-errors locally on an empty model_instructions_file (verified live); a default placeholder must always be written")
	}
}

func TestComplete_OutputSchema_PassesFlagWritesFileAndCleansUp(t *testing.T) {
	scratch := t.TempDir()
	captureFile := filepath.Join(t.TempDir(), "schema_capture.txt")
	t.Setenv("FAKE_CODEX_SCHEMA_CAPTURE", captureFile)

	envelope := `{"type":"thread.started","thread_id":"t1"}
{"type":"turn.started"}
{"type":"item.completed","item":{"id":"item_0","type":"agent_message","text":"{\"name\":\"a\"}"}}
{"type":"turn.completed","usage":{"input_tokens":1,"cached_input_tokens":0,"output_tokens":1,"reasoning_output_tokens":0}}
`
	c, argvFile, _ := newHappyPathConfig(t, codexcli.Config{ScratchDir: scratch}, envelope)

	schema := []byte(`{"type":"object"}`)
	resp, err := c.Complete(context.Background(), llmclient.Request{Prompt: "ping", OutputSchema: schema})
	if err != nil {
		t.Fatalf("Complete: %v", err)
	}
	if resp.Text != `{"name":"a"}` {
		t.Errorf("Text = %q, want %q", resp.Text, `{"name":"a"}`)
	}

	argv := readArgv(t, argvFile)
	idx := -1
	for i, a := range argv {
		if a == "--output-schema" {
			idx = i
		}
	}
	if idx == -1 || idx+1 >= len(argv) {
		t.Fatalf("argv %q does not contain --output-schema <path>", argv)
	}
	schemaPath := argv[idx+1]
	if !strings.HasPrefix(schemaPath, scratch) {
		t.Errorf("schema path %q is not under scratch dir %q", schemaPath, scratch)
	}

	got, err := os.ReadFile(captureFile)
	if err != nil {
		t.Fatalf("read captured schema content: %v", err)
	}
	if string(got) != string(schema) {
		t.Errorf("schema file content = %q, want %q", got, schema)
	}

	// Guaranteed cleanup: both the schema and instructions temp files are
	// gone once Complete has returned.
	assertDirEmpty(t, scratch)
}

func TestComplete_OutputSchema_CleanedUpEvenOnError(t *testing.T) {
	scratch := t.TempDir()
	t.Setenv("FAKE_CODEX_RESPONSE", "not json at all")
	t.Setenv("FAKE_CODEX_EXIT_CODE", "0")
	c, err := codexcli.New(codexcli.Config{Bin: newFakeCodex(t), ScratchDir: scratch})
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	_, err = c.Complete(context.Background(), llmclient.Request{Prompt: "ping", OutputSchema: []byte(`{"type":"object"}`)})
	if err == nil {
		t.Fatal("expected an error, got nil")
	}

	assertDirEmpty(t, scratch)
}

func TestComplete_Messages_TextOnlyFlattened(t *testing.T) {
	c, _, stdinFile := newHappyPathConfig(t, codexcli.Config{}, singleMessageStream)

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
	c, _, _ := newHappyPathConfig(t, codexcli.Config{}, singleMessageStream)

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
	c, _, _ := newHappyPathConfig(t, codexcli.Config{}, singleMessageStream)

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

func TestComplete_MultiItem_ConcatenatesAgentMessages(t *testing.T) {
	envelope := `{"type":"thread.started","thread_id":"t1"}
{"type":"turn.started"}
{"type":"item.completed","item":{"id":"item_0","type":"agent_message","text":"pon"}}
{"type":"item.completed","item":{"id":"item_1","type":"agent_message","text":"g"}}
{"type":"turn.completed","usage":{"input_tokens":10,"cached_input_tokens":0,"output_tokens":5,"reasoning_output_tokens":0}}
`
	c, _, _ := newHappyPathConfig(t, codexcli.Config{}, envelope)

	resp, err := c.Complete(context.Background(), llmclient.Request{Prompt: "ping"})
	if err != nil {
		t.Fatalf("Complete: %v", err)
	}
	if resp.Text != "pong" {
		t.Errorf("Text = %q, want %q (multi-item agent_message concatenation)", resp.Text, "pong")
	}
}

func TestComplete_NonAgentMessageItems_Ignored(t *testing.T) {
	envelope := `{"type":"thread.started","thread_id":"t1"}
{"type":"turn.started"}
{"type":"item.completed","item":{"id":"item_0","type":"error","text":""}}
{"type":"item.completed","item":{"id":"item_1","type":"agent_message","text":"pong"}}
{"type":"turn.completed","usage":{"input_tokens":1,"cached_input_tokens":0,"output_tokens":1,"reasoning_output_tokens":0}}
`
	c, _, _ := newHappyPathConfig(t, codexcli.Config{}, envelope)

	resp, err := c.Complete(context.Background(), llmclient.Request{Prompt: "ping"})
	if err != nil {
		t.Fatalf("Complete: %v", err)
	}
	if resp.Text != "pong" {
		t.Errorf("Text = %q, want %q (a soft item.completed error item must not corrupt the text accumulator)", resp.Text, "pong")
	}
}

func TestComplete_MissingTurnCompleted_Errors(t *testing.T) {
	envelope := `{"type":"thread.started","thread_id":"t1"}
{"type":"turn.started"}
{"type":"item.completed","item":{"id":"item_0","type":"agent_message","text":"pong"}}
`
	c, _, _ := newHappyPathConfig(t, codexcli.Config{}, envelope)

	_, err := c.Complete(context.Background(), llmclient.Request{Prompt: "ping"})
	if err == nil {
		t.Fatal("expected an error, got nil")
	}
	if !strings.Contains(err.Error(), "turn.completed or turn.failed") {
		t.Errorf("Complete err = %v, want it to mention a missing turn.completed/turn.failed", err)
	}
}

func TestComplete_EmptyStream_Errors(t *testing.T) {
	c, _, _ := newHappyPathConfig(t, codexcli.Config{}, "")

	_, err := c.Complete(context.Background(), llmclient.Request{Prompt: "ping"})
	if err == nil {
		t.Fatal("expected an error, got nil")
	}
	if !strings.Contains(err.Error(), "turn.completed or turn.failed") {
		t.Errorf("Complete err = %v, want it to mention a missing turn.completed/turn.failed", err)
	}
}

func TestComplete_MalformedLine_Errors(t *testing.T) {
	envelope := `{"type":"thread.started","thread_id":"t1"}
not valid json at all
{"type":"turn.completed","usage":{"input_tokens":1,"cached_input_tokens":0,"output_tokens":1,"reasoning_output_tokens":0}}
`
	c, _, _ := newHappyPathConfig(t, codexcli.Config{}, envelope)

	_, err := c.Complete(context.Background(), llmclient.Request{Prompt: "ping"})
	if err == nil {
		t.Fatal("expected an error, got nil")
	}
	if !strings.Contains(err.Error(), "malformed JSONL event line") {
		t.Errorf("Complete err = %v, want it to mention a malformed JSONL event line", err)
	}
}

func TestComplete_TurnFailed_MappedByStatus(t *testing.T) {
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
			nested := fmt.Sprintf(`{\"type\":\"error\",\"status\":%d,\"error\":{\"type\":\"invalid_request_error\",\"message\":\"boom\"}}`, tc.status)
			envelope := fmt.Sprintf(`{"type":"thread.started","thread_id":"t1"}
{"type":"turn.started"}
{"type":"error","message":"%s"}
{"type":"turn.failed","error":{"message":"%s"}}
`, nested, nested)
			c, _, _ := newHappyPathConfig(t, codexcli.Config{}, envelope)

			_, err := c.Complete(context.Background(), llmclient.Request{Prompt: "ping"})
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
				t.Errorf("Complete err = %v, want it to contain the nested error's message %q", err, "boom")
			}
		})
	}
}

func TestComplete_TurnFailed_PlainMessage_ClassifiedByContent(t *testing.T) {
	cases := []struct {
		name     string
		message  string
		wantType string
	}{
		{"not logged in", "Not logged in: please run codex login", "auth"},
		{"authentication keyword", "Authentication with ChatGPT failed", "auth"},
		{"rate limit keyword", "You have hit your usage limit for this plan", "rateLimit"},
		{"unrecognised", "something unexpected happened", "plain"},
		// "OAuth"/"author"-style incidental "auth" substrings must NOT
		// classify as AuthError — the keywords are full words by design
		// (see classifyByContent).
		{"oauth substring does not misfire", "OAuth callback server failed to start", "plain"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			envelope := fmt.Sprintf(`{"type":"thread.started","thread_id":"t1"}
{"type":"turn.started"}
{"type":"turn.failed","error":{"message":%q}}
`, tc.message)
			c, _, _ := newHappyPathConfig(t, codexcli.Config{}, envelope)

			_, err := c.Complete(context.Background(), llmclient.Request{Prompt: "ping"})
			if err == nil {
				t.Fatal("expected an error, got nil")
			}

			var authErr *llmclient.AuthError
			var rateErr *llmclient.RateLimitError
			switch tc.wantType {
			case "auth":
				if !errors.As(err, &authErr) {
					t.Errorf("Complete err = %v (%T), want *llmclient.AuthError", err, err)
				}
			case "rateLimit":
				if !errors.As(err, &rateErr) {
					t.Errorf("Complete err = %v (%T), want *llmclient.RateLimitError", err, err)
				}
			case "plain":
				if errors.As(err, &authErr) || errors.As(err, &rateErr) {
					t.Errorf("Complete err = %v, should NOT be a typed auth/rate-limit error for an unrecognised message", err)
				}
			}
			if !strings.Contains(err.Error(), tc.message) {
				t.Errorf("Complete err = %v, want it to contain %q", err, tc.message)
			}
		})
	}
}

func TestComplete_NonZeroExit_NonJSONStdout_UsesStderrTail(t *testing.T) {
	t.Setenv("FAKE_CODEX_RESPONSE", "not json at all")
	t.Setenv("FAKE_CODEX_STDERR", "permission denied opening credential store")
	t.Setenv("FAKE_CODEX_EXIT_CODE", "1")
	c, err := codexcli.New(codexcli.Config{Bin: newFakeCodex(t)})
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

func TestComplete_CtxTimeoutKillsChild(t *testing.T) {
	t.Setenv("FAKE_CODEX_SLEEP_SECONDS", "30")
	t.Setenv("FAKE_CODEX_RESPONSE", singleMessageStream)
	c, err := codexcli.New(codexcli.Config{Bin: newFakeCodex(t), Timeout: 200 * time.Millisecond})
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
	t.Setenv("FAKE_CODEX_SLEEP_SECONDS", "30")
	t.Setenv("FAKE_CODEX_RESPONSE", singleMessageStream)
	c, err := codexcli.New(codexcli.Config{Bin: newFakeCodex(t)})
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
	c, _, _ := newHappyPathConfig(t, codexcli.Config{}, singleMessageStream)

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
	t.Setenv("FAKE_CODEX_RESPONSE", "not json")
	t.Setenv("FAKE_CODEX_EXIT_CODE", "1")
	c, err := codexcli.New(codexcli.Config{Bin: newFakeCodex(t)})
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

func TestComplete_TurnFailedAfterPartialText_DiscardsText(t *testing.T) {
	// Failure wins: agent_message text accumulated before turn.failed must
	// be discarded — Complete returns only the error, never a partial
	// Response (see reducedTurn.text and the package doc, "Event shapes").
	envelope := `{"type":"thread.started","thread_id":"t1"}
{"type":"turn.started"}
{"type":"item.completed","item":{"id":"item_0","type":"agent_message","text":"partial answer before the failure"}}
{"type":"turn.failed","error":{"message":"something unexpected happened"}}
`
	c, _, _ := newHappyPathConfig(t, codexcli.Config{}, envelope)

	resp, err := c.Complete(context.Background(), llmclient.Request{Prompt: "ping"})
	if err == nil {
		t.Fatal("expected an error, got nil")
	}
	if !strings.Contains(err.Error(), "something unexpected happened") {
		t.Errorf("Complete err = %v, want it to carry the turn.failed message", err)
	}
	if resp.Text != "" || len(resp.Content) != 0 {
		t.Errorf("Complete returned a partial Response (%+v) alongside the error; failure must discard accumulated text", resp)
	}
}

func TestComplete_AgentMessageLineLargerThanDefaultScannerBuffer(t *testing.T) {
	// A single agent_message JSONL line larger than bufio.Scanner's 64 KiB
	// default token cap. Pins reduceEvents' explicit sc.Buffer sizing — a
	// regression deleting that call would fail this test with
	// bufio.ErrTooLong, while every other fixture in this suite would still
	// pass.
	big := strings.Repeat("a", 100_000)
	envelope := fmt.Sprintf(`{"type":"item.completed","item":{"id":"item_0","type":"agent_message","text":"%s"}}
{"type":"turn.completed","usage":{"input_tokens":1,"cached_input_tokens":0,"output_tokens":1,"reasoning_output_tokens":0}}
`, big)
	c, _, _ := newHappyPathConfig(t, codexcli.Config{}, envelope)

	resp, err := c.Complete(context.Background(), llmclient.Request{Prompt: "ping"})
	if err != nil {
		t.Fatalf("Complete: %v", err)
	}
	if resp.Text != big {
		t.Errorf("Text length = %d, want %d (a >64KiB event line must survive the scanner)", len(resp.Text), len(big))
	}
}

// fakeCodexConcurrentScript is a variant shim for the concurrency test:
// instead of the truncate-then-append shared capture files of
// fakeCodexScript (which would race between two in-flight calls), it writes
// per-invocation capture files keyed by its own PID under
// FAKE_CODEX_CAPTURE_DIR, so two concurrent Completes each leave an intact,
// separate stdin record.
const fakeCodexConcurrentScript = `#!/bin/sh
if [ -n "$FAKE_CODEX_CAPTURE_DIR" ]; then
  cat > "$FAKE_CODEX_CAPTURE_DIR/stdin.$$"
else
  cat > /dev/null
fi
printf '%s' "$FAKE_CODEX_RESPONSE"
exit 0
`

func TestComplete_ConcurrentCalls(t *testing.T) {
	// The package doc claims Client is safe for concurrent use; exercise two
	// in-flight Completes under -race and prove both prompts were delivered
	// intact through the shared scratch directory.
	captureDir := t.TempDir()
	t.Setenv("FAKE_CODEX_CAPTURE_DIR", captureDir)
	t.Setenv("FAKE_CODEX_RESPONSE", singleMessageStream)

	bin := filepath.Join(t.TempDir(), "codex")
	if err := os.WriteFile(bin, []byte(fakeCodexConcurrentScript), 0o755); err != nil {
		t.Fatalf("write concurrent fake codex script: %v", err)
	}
	c, err := codexcli.New(codexcli.Config{Bin: bin, ScratchDir: t.TempDir()})
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	prompts := []string{"ping-one", "ping-two"}
	resps := make([]llmclient.Response, len(prompts))
	errs := make([]error, len(prompts))
	var wg sync.WaitGroup
	for i := range prompts {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			resps[i], errs[i] = c.Complete(context.Background(), llmclient.Request{Prompt: prompts[i]})
		}(i)
	}
	wg.Wait()

	for i := range prompts {
		if errs[i] != nil {
			t.Fatalf("Complete[%d]: %v", i, errs[i])
		}
		if resps[i].Text != "pong" {
			t.Errorf("Complete[%d].Text = %q, want %q", i, resps[i].Text, "pong")
		}
	}

	entries, err := os.ReadDir(captureDir)
	if err != nil {
		t.Fatalf("read capture dir: %v", err)
	}
	got := map[string]bool{}
	for _, e := range entries {
		raw, err := os.ReadFile(filepath.Join(captureDir, e.Name()))
		if err != nil {
			t.Fatalf("read capture %s: %v", e.Name(), err)
		}
		got[string(raw)] = true
	}
	if len(entries) != 2 || !got["ping-one"] || !got["ping-two"] {
		t.Errorf("captured stdin set = %v (%d files), want exactly {ping-one, ping-two}", got, len(entries))
	}
}

func TestPing_Healthy_EmptyInstructionsFileRejectedLocally(t *testing.T) {
	// The healthy shape, pinned from live probe 13 (testdata/PROBES.md): a
	// CLI that still honors model_instructions_file rejects the empty file
	// with a local stderr error and exit 1 before doing anything else.
	scratch := t.TempDir()
	argvFile := filepath.Join(t.TempDir(), "argv.txt")
	stdinFile := filepath.Join(t.TempDir(), "stdin.txt")
	captureFile := filepath.Join(t.TempDir(), "instr_capture.txt")
	t.Setenv("FAKE_CODEX_ARGV_FILE", argvFile)
	t.Setenv("FAKE_CODEX_STDIN_FILE", stdinFile)
	t.Setenv("FAKE_CODEX_INSTRUCTIONS_CAPTURE", captureFile)
	t.Setenv("FAKE_CODEX_RESPONSE", "")
	t.Setenv("FAKE_CODEX_STDERR", "Error: model instructions file is empty: /ping/scratch/file.txt")
	t.Setenv("FAKE_CODEX_EXIT_CODE", "1")

	c, err := codexcli.New(codexcli.Config{Bin: newFakeCodex(t), ScratchDir: scratch})
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	if err := c.Ping(context.Background()); err != nil {
		t.Fatalf("Ping = %v, want nil (the local empty-file rejection IS the healthy outcome)", err)
	}

	// Ping runs the pinned production argv (no -m, no --output-schema, no
	// effort override).
	argv := readArgv(t, argvFile)
	instrValue, ok := findConfigOverride(argv, "model_instructions_file")
	if !ok {
		t.Fatalf("Ping argv %q missing -c model_instructions_file=...", argv)
	}
	if !strings.HasPrefix(instrValue, scratch) {
		t.Errorf("Ping instructions file %q is not under scratch dir %q", instrValue, scratch)
	}
	want := []string{
		"exec",
		"--json",
		"-C", scratch,
		"--skip-git-repo-check",
		"--ignore-user-config",
		"--ephemeral",
		"-s", "read-only",
		"-c", `model_instructions_file="` + instrValue + `"`,
		"-c", "include_environment_context=false",
		"-c", "include_apps_instructions=false",
		"-c", "include_collaboration_mode_instructions=false",
		"-c", "include_permissions_instructions=false",
	}
	if !equalSlices(argv, want) {
		t.Errorf("Ping argv = %q, want %q", argv, want)
	}

	// The probe's trigger: the instructions file existed and was EMPTY.
	capture, err := os.ReadFile(captureFile)
	if err != nil {
		t.Fatalf("instructions capture missing — Ping's instructions file did not exist when the shim ran: %v", err)
	}
	if len(capture) != 0 {
		t.Errorf("Ping instructions file content = %q, want empty (the empty file IS the probe)", capture)
	}

	// No prompt is ever sent.
	stdin, err := os.ReadFile(stdinFile)
	if err != nil {
		t.Fatalf("read captured stdin: %v", err)
	}
	if len(stdin) != 0 {
		t.Errorf("Ping stdin = %q, want empty", stdin)
	}

	// The ping temp file is cleaned up.
	assertDirEmpty(t, scratch)
}

func TestPing_Degraded_NoLocalRejection(t *testing.T) {
	// The degradation shapes: a future CLI that silently dropped
	// model_instructions_file would NOT reject the empty file — it would
	// proceed (exit 0 with a normal event stream) or fail some other way.
	// Both must surface as a typed InvalidRequestError naming the risk.
	cases := []struct {
		name     string
		response string
		stderr   string
		exitCode string
	}{
		{"cli proceeds with a normal turn", singleMessageStream, "", "0"},
		{"cli fails for an unrelated reason", "", "some unrelated failure", "1"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			scratch := t.TempDir()
			t.Setenv("FAKE_CODEX_RESPONSE", tc.response)
			t.Setenv("FAKE_CODEX_STDERR", tc.stderr)
			t.Setenv("FAKE_CODEX_EXIT_CODE", tc.exitCode)

			c, err := codexcli.New(codexcli.Config{Bin: newFakeCodex(t), ScratchDir: scratch})
			if err != nil {
				t.Fatalf("New: %v", err)
			}

			err = c.Ping(context.Background())
			if err == nil {
				t.Fatal("Ping = nil, want a typed degradation error when the CLI does not reject the empty instructions file")
			}
			var invalidErr *llmclient.InvalidRequestError
			if !errors.As(err, &invalidErr) {
				t.Fatalf("Ping err = %v (%T), want *llmclient.InvalidRequestError", err, err)
			}
			if !strings.Contains(err.Error(), "model_instructions_file") {
				t.Errorf("Ping err = %v, want it to name the model_instructions_file dependence", err)
			}

			// The ping temp file is cleaned up on the degraded path too.
			assertDirEmpty(t, scratch)
		})
	}
}

func TestPing_Degraded_HangKilledByTimeout(t *testing.T) {
	// A degraded CLI that hangs (e.g. waiting on the network) is killed by
	// the ping bound and reported as the same typed degradation error —
	// never as a silent pass and never as an unbounded wait.
	t.Setenv("FAKE_CODEX_SLEEP_SECONDS", "30")
	t.Setenv("FAKE_CODEX_RESPONSE", "")
	c, err := codexcli.New(codexcli.Config{Bin: newFakeCodex(t), ScratchDir: t.TempDir(), Timeout: 200 * time.Millisecond})
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
	t.Setenv("FAKE_CODEX_SLEEP_SECONDS", "30")
	t.Setenv("FAKE_CODEX_RESPONSE", "")
	c, err := codexcli.New(codexcli.Config{Bin: newFakeCodex(t), ScratchDir: t.TempDir()})
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

// assertDirEmpty fails the test if dir contains any entries — used to prove
// per-call scratch temp files (instructions, OutputSchema) were cleaned up.
func assertDirEmpty(t *testing.T, dir string) {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("read scratch dir %q: %v", dir, err)
	}
	if len(entries) != 0 {
		names := make([]string, len(entries))
		for i, e := range entries {
			names[i] = e.Name()
		}
		t.Errorf("scratch dir %q not empty after Complete returned: %v (temp files not cleaned up)", dir, names)
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
