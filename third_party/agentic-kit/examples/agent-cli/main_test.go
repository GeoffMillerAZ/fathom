package main

// Hermetic tests for the testable main (the cmd/endpoint-guard pattern): no
// network, no API key, no real environment. Mock and replay modes run
// end-to-end through run(); the tools are additionally exercised directly
// through the registry for the confinement contract.

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/third_party/agentic-kit/foundations/llmclient"
)

// noEnv is the getenv used in every test: hermetic — no real environment
// variable (and in particular no real API key) ever reaches the code under
// test.
func noEnv(string) string { return "" }

// runCLI executes the testable main with captured stdio.
func runCLI(t *testing.T, args ...string) (code int, stdout, stderr string) {
	t.Helper()
	var out, errOut bytes.Buffer
	code = run(args, &out, &errOut, noEnv)
	return code, out.String(), errOut.String()
}

const wantFinalText = "testdata/sample.txt contains 9 words."

func TestMockModeEndToEnd(t *testing.T) {
	code, stdout, stderr := runCLI(t, "-mode", "mock")
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr:\n%s", code, stderr)
	}
	for _, want := range []string{"stop=completed", "turns=3", "tool_calls=2", wantFinalText} {
		if !strings.Contains(stdout, want) {
			t.Errorf("stdout missing %q:\n%s", want, stdout)
		}
	}
}

// runResultJSON mirrors the fields of agentharness.RunResult asserted on in
// the -json tests.
type runResultJSON struct {
	FinalText string
	Stop      string
	Turns     int
	ToolCalls int
	Messages  []struct{ Role string }
}

func decodeResult(t *testing.T, stdout string) runResultJSON {
	t.Helper()
	var got runResultJSON
	if err := json.Unmarshal([]byte(stdout), &got); err != nil {
		t.Fatalf("stdout is not RunResult JSON: %v\n%s", err, stdout)
	}
	return got
}

func assertSessionShape(t *testing.T, got runResultJSON) {
	t.Helper()
	if got.Stop != "completed" {
		t.Errorf("Stop = %q, want %q", got.Stop, "completed")
	}
	if got.Turns != 3 {
		t.Errorf("Turns = %d, want 3", got.Turns)
	}
	if got.ToolCalls != 2 {
		t.Errorf("ToolCalls = %d, want 2", got.ToolCalls)
	}
	if got.FinalText != wantFinalText {
		t.Errorf("FinalText = %q, want %q", got.FinalText, wantFinalText)
	}
	// user, assistant(tool_use), user(tool_result), assistant(tool_use),
	// user(tool_result), assistant(final) — six messages, strict alternation.
	if len(got.Messages) != 6 {
		t.Fatalf("len(Messages) = %d, want 6", len(got.Messages))
	}
	for i, m := range got.Messages {
		want := "user"
		if i%2 == 1 {
			want = "assistant"
		}
		if m.Role != want {
			t.Errorf("Messages[%d].Role = %q, want %q", i, m.Role, want)
		}
	}
}

func TestMockModeJSON(t *testing.T) {
	code, stdout, stderr := runCLI(t, "-mode", "mock", "-json")
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr:\n%s", code, stderr)
	}
	assertSessionShape(t, decodeResult(t, stdout))
}

// TestReplayMode replays the committed cassette end-to-end. The replay
// client matches every request by hash, so this also proves fidelity: the
// live conversation (prompt, system, tool schemas, and the tool results the
// re-executed tools produce) is byte-identical to the recorded session.
func TestReplayMode(t *testing.T) {
	code, stdout, stderr := runCLI(t,
		"-mode", "replay", "-cassette", "testdata/mock-session.jsonl", "-json")
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr:\n%s", code, stderr)
	}
	assertSessionShape(t, decodeResult(t, stdout))
}

// TestReplayDivergenceIsLoud proves the other half of the fidelity contract:
// a run that drifts from the recording (different prompt, here) must fail
// loudly with a hash mismatch, never silently serve the wrong entry.
func TestReplayDivergenceIsLoud(t *testing.T) {
	code, _, stderr := runCLI(t,
		"-mode", "replay", "-cassette", "testdata/mock-session.jsonl",
		"-prompt", "A different question entirely.")
	if code != 1 {
		t.Fatalf("exit code = %d, want 1; stderr:\n%s", code, stderr)
	}
	if !strings.Contains(stderr, "request hash mismatch") {
		t.Errorf("stderr missing %q:\n%s", "request hash mismatch", stderr)
	}
}

// TestCommittedCassetteIsCurrent re-records the mock session and compares it
// byte-for-byte with the committed cassette, keeping the go:generate
// artifact honest.
func TestCommittedCassetteIsCurrent(t *testing.T) {
	tmp := filepath.Join(t.TempDir(), "session.jsonl")
	code, _, stderr := runCLI(t, "-mode", "mock", "-record", "-cassette", tmp)
	if code != 0 {
		t.Fatalf("recording run failed: exit %d; stderr:\n%s", code, stderr)
	}
	got, err := os.ReadFile(tmp)
	if err != nil {
		t.Fatalf("read recorded cassette: %v", err)
	}
	want, err := os.ReadFile(filepath.Join("testdata", "mock-session.jsonl"))
	if err != nil {
		t.Fatalf("read committed cassette: %v", err)
	}
	if !bytes.Equal(got, want) {
		t.Errorf("testdata/mock-session.jsonl is stale: regenerate with `go generate .` (or `go run . -mode mock -record -cassette testdata/mock-session.jsonl`)")
	}
}

func TestVerboseStreamsTelemetryAndCostReport(t *testing.T) {
	code, _, stderr := runCLI(t, "-mode", "mock", "-verbose")
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr:\n%s", code, stderr)
	}
	// One llm_call event per model call, emitted by the instrument
	// middleware through the JSONL sink.
	if got := strings.Count(stderr, `"Kind":"llm_call"`); got != 3 {
		t.Errorf("llm_call events = %d, want 3; stderr:\n%s", got, stderr)
	}
	// Per-turn events from the harness share the same sink.
	if !strings.Contains(stderr, `"Source":"kits/agent-harness"`) {
		t.Errorf("stderr missing harness events:\n%s", stderr)
	}
	// The cost report aggregates the script's synthetic usage.
	for _, want := range []string{"cost report", "calls=3 input=2153 output=125"} {
		if !strings.Contains(stderr, want) {
			t.Errorf("stderr missing %q:\n%s", want, stderr)
		}
	}
}

// TestToolConfinement drives the registry directly (the same Dispatch path
// the harness uses) and checks the sandbox contract of every tool: absolute
// paths and ".." traversal are rejected as is_error tool_results.
func TestToolConfinement(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "hello.txt"), []byte("one two three\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	root, err := os.OpenRoot(dir)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { root.Close() })
	reg := newRegistry(root)

	tests := []struct {
		name    string
		tool    string
		input   string
		wantErr bool
		want    string // substring of the tool_result content
	}{
		{"read_file rejects absolute path", "read_file", `{"path":"/etc/passwd"}`, true, "absolute"},
		{"read_file rejects traversal", "read_file", `{"path":"../outside.txt"}`, true, "traversal"},
		{"read_file rejects nested traversal", "read_file", `{"path":"sub/../../outside.txt"}`, true, "traversal"},
		{"read_file rejects empty path", "read_file", `{"path":""}`, true, "empty"},
		{"read_file reads inside the root", "read_file", `{"path":"hello.txt"}`, false, "one two three"},
		{"list_dir rejects absolute path", "list_dir", `{"path":"/"}`, true, "absolute"},
		{"list_dir rejects traversal", "list_dir", `{"path":".."}`, true, "traversal"},
		{"list_dir defaults to the working dir", "list_dir", `{}`, false, "hello.txt"},
		{"word_count is pure", "word_count", `{"text":"a b   c"}`, false, "3"},
		{"word_count requires text", "word_count", `{}`, true, "text"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			res := reg.Dispatch(context.Background(),
				llmclient.ToolUseBlock("toolu_test", tc.tool, json.RawMessage(tc.input)))
			if res.IsError != tc.wantErr {
				t.Fatalf("IsError = %v, want %v (content: %s)", res.IsError, tc.wantErr, res.Content)
			}
			if !strings.Contains(res.Content, tc.want) {
				t.Errorf("result %q missing %q", res.Content, tc.want)
			}
		})
	}
}

// TestReadFileCap checks the byte cap: a file past maxReadBytes comes back
// truncated with an explicit marker, never whole.
func TestReadFileCap(t *testing.T) {
	dir := t.TempDir()
	big := bytes.Repeat([]byte("x"), maxReadBytes+100)
	if err := os.WriteFile(filepath.Join(dir, "big.txt"), big, 0o644); err != nil {
		t.Fatal(err)
	}
	root, err := os.OpenRoot(dir)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { root.Close() })
	reg := newRegistry(root)

	res := reg.Dispatch(context.Background(),
		llmclient.ToolUseBlock("toolu_test", "read_file", json.RawMessage(`{"path":"big.txt"}`)))
	if res.IsError {
		t.Fatalf("unexpected tool error: %s", res.Content)
	}
	if !strings.Contains(res.Content, "[output truncated at") {
		t.Errorf("result missing truncation marker (len=%d)", len(res.Content))
	}
	if len(res.Content) > maxReadBytes+100 {
		t.Errorf("result length %d exceeds the cap plus marker", len(res.Content))
	}
}

func TestUsageErrors(t *testing.T) {
	tests := []struct {
		name string
		args []string
	}{
		{"unknown flag", []string{"-definitely-not-a-flag"}},
		{"invalid mode", []string{"-mode", "bogus"}},
		{"replay without cassette", []string{"-mode", "replay"}},
		{"record in replay mode", []string{"-mode", "replay", "-cassette", "testdata/mock-session.jsonl", "-record"}},
		{"record without cassette", []string{"-mode", "mock", "-record"}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			code, _, stderr := runCLI(t, tc.args...)
			if code != 2 {
				t.Errorf("exit code = %d, want 2; stderr:\n%s", code, stderr)
			}
		})
	}
}

// TestLiveModeRefusesWithoutKey checks the fail-closed startup path: live
// mode with no key never builds a client (and never touches the network) —
// it exits 1 with a message naming exactly what to set.
func TestLiveModeRefusesWithoutKey(t *testing.T) {
	code, _, stderr := runCLI(t, "-mode", "live")
	if code != 1 {
		t.Fatalf("exit code = %d, want 1; stderr:\n%s", code, stderr)
	}
	if !strings.Contains(stderr, apiKeyEnv) {
		t.Errorf("stderr does not name %s:\n%s", apiKeyEnv, stderr)
	}
}
