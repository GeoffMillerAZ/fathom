package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os/exec"
	"time"

	"github.com/geoffmilleraz/fathom/internal/config"
	"github.com/geoffmilleraz/fathom/third_party/agentic-kit/foundations/llmclient"
)

// Local checks run to completion. IO keeps its own request cap, CLI pings
// keep their adapter budgets, and Setup still supplies the run-wide deadline.
func doctorStepContext(parent context.Context) (context.Context, context.CancelFunc) {
	if g := activeHookGuard(); g != nil && !g.Deadline.IsZero() {
		return g.Deadline.Context(parent)
	}
	return context.WithCancel(parent)
}

// Bound an individual RPC/git probe without imposing its cap on local work.
func doctorProbeContext(parent context.Context) (context.Context, context.CancelFunc) {
	if g := activeHookGuard(); g != nil && !g.Deadline.IsZero() {
		ctx, cancel := g.Deadline.Context(parent)
		probe, stop := context.WithTimeout(ctx, 5*time.Second)
		return probe, func() { stop(); cancel() }
	}
	return context.WithTimeout(parent, 5*time.Second)
}

func doctorErrorStatus(err error) doctorStatus {
	var timeout net.Error
	if errors.Is(err, context.DeadlineExceeded) || (errors.As(err, &timeout) && timeout.Timeout()) {
		return doctorTimeout
	}
	if errors.Is(err, context.Canceled) {
		return doctorUnknown
	}
	return doctorFail
}

func runDoctorProbe(ctx context.Context, probe func(context.Context) doctorCheck) doctorCheck {
	row := probe(ctx)
	if err := ctx.Err(); err != nil && errors.Is(row.probeErr, err) {
		row.Status = doctorErrorStatus(err)
		row.Reason = string(row.Status) + ": probe cancelled; " + row.Reason
		row.Remedy = "re-run doctor when the probe can finish within its deadline"
	}
	return row
}

// Request contexts reach both headers and body reads, including RPC helpers
// that create their own requests. No global transport or context is mutated.
type doctorTransport struct{ ctx context.Context }

func (t doctorTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	ctx, cancel := context.WithCancel(req.Context())
	if at, ok := t.ctx.Deadline(); ok {
		cancel()
		ctx, cancel = context.WithDeadline(req.Context(), at)
	}
	stop := context.AfterFunc(t.ctx, cancel)
	if t.ctx.Err() != nil {
		cancel()
	}
	resp, err := http.DefaultTransport.RoundTrip(req.Clone(ctx))
	if err != nil {
		stop()
		cancel()
		return nil, err
	}
	resp.Body = &doctorProbeBody{ReadCloser: resp.Body, done: func() { stop(); cancel() }}
	return resp, nil
}

type doctorProbeBody struct {
	io.ReadCloser
	done func()
}

func (b *doctorProbeBody) Close() error { defer b.done(); return b.ReadCloser.Close() }

func doctorHTTPClient(ctx context.Context) *http.Client {
	return &http.Client{Timeout: 5 * time.Second, Transport: doctorTransport{ctx: ctx}}
}

func doctorGitCommand(ctx context.Context, root string, args ...string) *exec.Cmd {
	cmd := exec.CommandContext(ctx, "git", append([]string{"-C", root}, args...)...)
	cmd.WaitDelay = time.Second
	return cmd
}

func doctorProfileBuilder(ctx context.Context) profileClientBuilder {
	return func(p config.Profile) (llmclient.LLMClient, error) { return buildLLMClientForProfileContext(ctx, p) }
}

func doctorResourcesReport(ctx context.Context, baseURL string) resourcesReport {
	env := liveResourcesEnv()
	env.fetch = func(url string) (*StatuszReport, error) {
		hc := doctorHTTPClient(ctx)
		hc.Timeout = 2 * time.Second
		return fetchStatuszReportClient(hc, url)
	}
	return buildResources(env, "", baseURL, 24*time.Hour)
}

func doctorContextArg(parents []context.Context) (context.Context, context.CancelFunc) {
	if len(parents) > 0 {
		return context.WithCancel(parents[0])
	}
	return doctorStepContext(context.Background())
}

func doctorReachabilityReason(baseURL string, err error) string {
	if doctorErrorStatus(err) == doctorTimeout {
		return "daemon did not answer within 5 s: " + err.Error()
	}
	return fmt.Sprintf("GET %s/healthz: %v", baseURL, err)
}
