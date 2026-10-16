// Cross-process trace-carrier canary: proves the composition seam between
// event.AppendTraceparent and Config.Env — a caller that composes
// AppendTraceparent(cfg.Env, tc) gets a child that sees TRACEPARENT, and
// the parsed child-side value carries the caller's RunID/SpanID. Like the
// other tests in this package it uses a real subprocess and skips on hosts
// without the helper binaries (documented determinism deviation, see the
// package doc).
package boundedexec

import (
	"context"
	"strings"
	"testing"

	event "github.com/geoffmilleraz/fathom/third_party/agentic-kit/foundations/telemetry/event"
)

// TestTraceparentEnvRoundTrip runs /usr/bin/env in a child whose complete
// environment is AppendTraceparent(base, tc) and recovers the trace from
// the child's actual environ via TraceFromEnviron: the subprocess leg of
// the cross-process trace claim (roadmap #21).
func TestTraceparentEnvRoundTrip(t *testing.T) {
	t.Parallel()
	requireTool(t, envPath)

	t.Run("injected trace reaches the child", func(t *testing.T) {
		t.Parallel()
		sender := event.NewTrace(nil)
		base := []string{"AK_BASE=1"} // stands in for a scrubbed allowlist Config.Env

		cfgRun := generousCfg()
		cfgRun.Env = event.AppendTraceparent(base, sender)
		runner := mustRunner(t, cfgRun)
		res, err := runner.Run(context.Background(), Spec{Path: envPath})
		if err != nil {
			t.Fatalf("Run: %v", err)
		}
		if res.ExitCode != 0 {
			t.Fatalf("ExitCode = %d, want 0 (stderr: %q)", res.ExitCode, res.Stderr)
		}

		var childEnviron []string
		for _, line := range strings.Split(string(res.Stdout), "\n") {
			if line != "" {
				childEnviron = append(childEnviron, line)
			}
		}
		got, ok := event.TraceFromEnviron(childEnviron)
		if !ok {
			t.Fatalf("child environ carries no parseable TRACEPARENT; env = %q", childEnviron)
		}
		if got.RunID != sender.RunID {
			t.Errorf("child-side RunID = %q, want sender's %q", got.RunID, sender.RunID)
		}
		if got.SpanID != sender.SpanID {
			t.Errorf("child-side SpanID = %q, want sender's %q", got.SpanID, sender.SpanID)
		}
		if got.ParentSpanID != "" {
			t.Errorf("ParentSpanID must not cross the wire; got %q", got.ParentSpanID)
		}
	})

	t.Run("zero trace injects nothing", func(t *testing.T) {
		t.Parallel()
		cfgRun := generousCfg()
		cfgRun.Env = event.AppendTraceparent([]string{"AK_BASE=1"}, event.TraceContext{})
		runner := mustRunner(t, cfgRun)
		res, err := runner.Run(context.Background(), Spec{Path: envPath})
		if err != nil {
			t.Fatalf("Run: %v", err)
		}
		if strings.Contains(string(res.Stdout), event.TraceparentEnv+"=") {
			t.Errorf("zero trace must not inject %s; child env:\n%s", event.TraceparentEnv, res.Stdout)
		}
	})
}
