package llmnoop_test

import (
	"errors"
	"os"
	"os/exec"
	"reflect"
	"strings"
	"testing"

	"github.com/geoffmilleraz/agentic-kit/foundations/llmclient"
	llmtest "github.com/geoffmilleraz/agentic-kit/foundations/llmclient/llm-test"
	llmnoop "github.com/geoffmilleraz/agentic-kit/foundations/llmclient/noop"
)

// conformanceChildEnv marks the re-executed test binary in which
// TestNoopConformanceSuite actually runs the suite. Without it that test
// skips itself, so a plain `go test` never inherits the suite's failures —
// they are TestNoopConformance's pinned outcome, not a defect.
const conformanceChildEnv = "LLMNOOP_CONFORMANCE_CHILD"

// noopConformanceOutcome is the pinned verdict of every core check in
// llmtest.RunConformance against llmnoop.Client, and the claim this file
// makes: three of the seven core checks pass; the other four assert things a
// no-op client cannot satisfy by design — that the returned text equals the
// requested text (it is always Message) and that Usage reports more than
// zero tokens (none were consumed). Drift on either side — the suite
// relaxing or tightening, the client changing — fails TestNoopConformance.
var noopConformanceOutcome = map[string]string{
	"CompleteTextMatchesBlocks": "FAIL", // Text is Message, never the requested text
	"AsMessageRoundTrip":        "PASS",
	"StopReasonPopulated":       "PASS", // StopRefusal
	"StreamLifecycle":           "FAIL", // streamed text is Message, never the requested text
	"StreamContextCancellation": "PASS",
	"UsageOnComplete":           "FAIL", // zero tokens: nothing was consumed
	"UsageOnStream":             "FAIL", // zero tokens: nothing was consumed
}

// TestNoopConformance runs llmtest.RunConformance's core checks against
// llmnoop.Client and pins the outcome check by check (noopConformanceOutcome).
// The client does NOT pass the suite, and this test says so precisely rather
// than skipping the suite or claiming otherwise: the suite runs in a
// re-executed copy of this test binary (the os/exec helper-process pattern —
// no toolchain, no network) so its failures are observed here instead of
// inherited, then the child's per-check verdicts are compared to the pin.
func TestNoopConformance(t *testing.T) {
	t.Log("optional capabilities off — " +
		"ToolUse: no tool call is ever fabricated and Request.Tools is ignored; " +
		"MidStreamError: there is no failure mode to script; " +
		"UnknownBlock: the one content block is text; " +
		"TypedErrors: there is no transport and no status to map; " +
		"BodyInspection: there is no wire body; " +
		"PreflightValidation: the request is ignored entirely, so nothing is validated")

	exe, err := os.Executable()
	if err != nil {
		t.Fatalf("locate test binary: %v", err)
	}
	cmd := exec.Command(exe, "-test.run=^TestNoopConformanceSuite$", "-test.v", "-test.timeout=60s")
	cmd.Env = append(os.Environ(), conformanceChildEnv+"=1")
	out, runErr := cmd.CombinedOutput()

	got := parseVerdicts(string(out), "TestNoopConformanceSuite/")
	if !reflect.DeepEqual(got, noopConformanceOutcome) {
		t.Fatalf("core-check verdicts drifted from the pin\n got: %v\nwant: %v\nchild output:\n%s", got, noopConformanceOutcome, out)
	}

	// Four pinned failures: the child must have exited non-zero, and through
	// a normal test-binary exit rather than a crash or a missing binary.
	if runErr == nil {
		t.Fatalf("child exited 0 despite the pinned failures; output:\n%s", out)
	}
	var exitErr *exec.ExitError
	if !errors.As(runErr, &exitErr) {
		t.Fatalf("child did not run to completion: %v; output:\n%s", runErr, out)
	}
}

// parseVerdicts extracts the "--- PASS:" / "--- FAIL:" verdicts for the
// subtests under prefix from -test.v output, keyed by short subtest name.
func parseVerdicts(out, prefix string) map[string]string {
	got := map[string]string{}
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimSpace(line)
		var verdict string
		switch {
		case strings.HasPrefix(line, "--- PASS: "):
			verdict = "PASS"
		case strings.HasPrefix(line, "--- FAIL: "):
			verdict = "FAIL"
		default:
			continue
		}
		name := strings.TrimPrefix(line, "--- "+verdict+": ")
		if i := strings.IndexByte(name, ' '); i >= 0 {
			name = name[:i] // drop the "(0.00s)" duration
		}
		if !strings.HasPrefix(name, prefix) {
			continue
		}
		got[strings.TrimPrefix(name, prefix)] = verdict
	}
	return got
}

// TestNoopConformanceSuite is the child half of TestNoopConformance: it runs
// the real suite, every optional capability off, only inside the re-executed
// binary. In a normal `go test` run it skips.
func TestNoopConformanceSuite(t *testing.T) {
	if os.Getenv(conformanceChildEnv) != "1" {
		t.Skip("child-only: TestNoopConformance re-executes this binary and pins the verdicts")
	}
	llmtest.RunConformance(t, llmtest.Harness{
		NewText: func(t *testing.T, _ string) llmclient.LLMClient {
			// The requested text is ignored — that is the client's contract
			// and the reason CompleteTextMatchesBlocks and StreamLifecycle
			// are pinned FAIL.
			return llmnoop.New()
		},
	}, llmtest.Capabilities{
		ToolUse:             false,
		MidStreamError:      false,
		UnknownBlock:        false,
		TypedErrors:         false,
		BodyInspection:      false,
		PreflightValidation: false,
	})
}
