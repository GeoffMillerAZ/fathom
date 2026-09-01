package main

// Hermetic tests for the testable main (the examples/agent-cli pattern): no
// network, no API key, no real environment.

import (
	"bytes"
	"strings"
	"testing"
)

func noEnv(string) string { return "" }

func runCLI(t *testing.T, args ...string) (code int, stdout, stderr string) {
	t.Helper()
	var out, errOut bytes.Buffer
	code = run(args, &out, &errOut, noEnv)
	return code, out.String(), errOut.String()
}

func TestRun_DemoRetriever_MockLLM_ComposesAnswer(t *testing.T) {
	code, stdout, stderr := runCLI(t, "-query", "what does the loop refuse to do?")
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr:\n%s", code, stderr)
	}
	if !strings.Contains(stdout, "Citations:") {
		t.Fatalf("stdout missing rendered citations:\n%s", stdout)
	}
}

func TestRun_JSONOutput(t *testing.T) {
	code, stdout, stderr := runCLI(t, "-query", "q", "-json")
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr:\n%s", code, stderr)
	}
	if !strings.Contains(stdout, `"citations"`) {
		t.Fatalf("stdout is not the expected JSON shape:\n%s", stdout)
	}
}

func TestRun_TierThresholdAboveDemoCorpus_Refuses(t *testing.T) {
	// The demo corpus is all "doc-claimed" tier (scoring.TierRank 3); asking
	// for "verified" (rank 5) leaves nothing eligible — exit code 3, a typed
	// refusal, not an operational error.
	code, stdout, stderr := runCLI(t, "-query", "q", "-tier-threshold", "verified")
	if code != 3 {
		t.Fatalf("exit code = %d, want 3 (refusal); stdout=%q stderr=%q", code, stdout, stderr)
	}
	if !strings.Contains(stdout, "REFUSED") {
		t.Fatalf("stdout missing REFUSED marker:\n%s", stdout)
	}
}

func TestRun_UnknownRetrieverMode_UsageError(t *testing.T) {
	code, _, stderr := runCLI(t, "-query", "q", "-retriever", "bogus")
	if code != 2 {
		t.Fatalf("exit code = %d, want 2; stderr:\n%s", code, stderr)
	}
}

func TestRun_HTTPRetrieverWithoutURL_UsageError(t *testing.T) {
	code, _, stderr := runCLI(t, "-query", "q", "-retriever", "http")
	if code != 2 {
		t.Fatalf("exit code = %d, want 2; stderr:\n%s", code, stderr)
	}
	if !strings.Contains(stderr, "-url") {
		t.Fatalf("stderr does not explain the missing -url:\n%s", stderr)
	}
}

func TestRun_LiveModeWithoutKey_OperationalError(t *testing.T) {
	code, _, stderr := runCLI(t, "-query", "q", "-llm", "live")
	if code != 1 {
		t.Fatalf("exit code = %d, want 1; stderr:\n%s", code, stderr)
	}
	if !strings.Contains(stderr, "ANTHROPIC_API_KEY") {
		t.Fatalf("stderr does not name the missing key env var:\n%s", stderr)
	}
}

func TestRun_EmptyQuery_UsageError(t *testing.T) {
	code, _, _ := runCLI(t, "-query", "")
	if code != 2 {
		t.Fatalf("exit code = %d, want 2", code)
	}
}
