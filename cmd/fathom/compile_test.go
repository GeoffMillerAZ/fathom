package main

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/geoffmilleraz/agentic-kit/foundations/llmclient"
)

// fakeClaudeCLIScript is a hermetic stand-in for the `claude` binary — no
// network, no real credential store. It always emits the same canned
// success envelope regardless of argv/stdin; the claudecli package's own
// test suite (agentic-kit) already covers argv/error-mapping/timeout
// behavior exhaustively, so this is just enough to prove buildLLMClient's
// "claude-cli" branch really wires a working end-to-end Complete call.
const fakeClaudeCLIScript = `#!/bin/sh
cat > /dev/null
printf '%s' '{"type":"result","subtype":"success","is_error":false,"api_error_status":null,"result":"pong","stop_reason":"end_turn","usage":{"input_tokens":1,"output_tokens":1,"cache_creation_input_tokens":0,"cache_read_input_tokens":0}}'
exit 0
`

// newFakeClaudeCLI writes fakeClaudeCLIScript into t.TempDir() and returns
// its absolute path.
func newFakeClaudeCLI(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "claude")
	if err := os.WriteFile(path, []byte(fakeClaudeCLIScript), 0o755); err != nil {
		t.Fatalf("write fake claude script: %v", err)
	}
	return path
}

func TestBuildLLMClient_DefaultAndExplicitAnthropic_NoKey(t *testing.T) {
	t.Setenv("ANTHROPIC_API_KEY", "")

	for _, provider := range []string{"", "anthropic"} {
		t.Run("provider="+provider, func(t *testing.T) {
			t.Setenv("FATHOM_LLM_PROVIDER", provider)
			_, err := buildLLMClient()
			if err == nil {
				t.Fatal("expected an error building the anthropic client with no ANTHROPIC_API_KEY, got nil")
			}
		})
	}
}

func TestBuildLLMClient_ClaudeCLI_ConstructsAndCompletes(t *testing.T) {
	t.Setenv("FATHOM_LLM_PROVIDER", "claude-cli")
	t.Setenv("FATHOM_CLAUDE_BIN", newFakeClaudeCLI(t))

	cl, err := buildLLMClient()
	if err != nil {
		t.Fatalf("buildLLMClient: %v", err)
	}

	resp, err := cl.Complete(context.Background(), llmclient.Request{Prompt: "ping"})
	if err != nil {
		t.Fatalf("Complete: %v", err)
	}
	if resp.Text != "pong" {
		t.Errorf("Text = %q, want %q", resp.Text, "pong")
	}
	if resp.StopReason != llmclient.StopEndTurn {
		t.Errorf("StopReason = %q, want %q", resp.StopReason, llmclient.StopEndTurn)
	}
}

func TestBuildLLMClient_ClaudeCLI_BinNotFound(t *testing.T) {
	t.Setenv("FATHOM_LLM_PROVIDER", "claude-cli")
	t.Setenv("FATHOM_CLAUDE_BIN", filepath.Join(t.TempDir(), "does-not-exist-claude"))

	_, err := buildLLMClient()
	if err == nil {
		t.Fatal("expected an error resolving a nonexistent FATHOM_CLAUDE_BIN, got nil")
	}
}

func TestBuildLLMClient_UnknownProvider(t *testing.T) {
	t.Setenv("FATHOM_LLM_PROVIDER", "bogus-provider")

	_, err := buildLLMClient()
	if err == nil {
		t.Fatal("expected an error for an unknown FATHOM_LLM_PROVIDER, got nil")
	}
	if !strings.Contains(err.Error(), "bogus-provider") {
		t.Errorf("err = %v, want it to name the bad value %q", err, "bogus-provider")
	}
}

// TestBuildClaudeCLIClient_WrapsResolveError is a narrower unit check that
// buildClaudeCLIClient itself (not just the buildLLMClient switch) surfaces
// claudecli.New's resolution error rather than swallowing it.
func TestBuildClaudeCLIClient_WrapsResolveError(t *testing.T) {
	bin := filepath.Join(t.TempDir(), "does-not-exist-claude")
	t.Setenv("FATHOM_CLAUDE_BIN", bin)

	_, err := buildClaudeCLIClient()
	if err == nil {
		t.Fatal("expected an error, got nil")
	}
	if !strings.Contains(err.Error(), "construct Claude CLI client") {
		t.Errorf("err = %v, want it to be wrapped with \"construct Claude CLI client\"", err)
	}
}
