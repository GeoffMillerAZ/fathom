package hookguard

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"strconv"
	"sync"
	"syscall"
	"time"
)

// SkipError is returned instead of sending an RPC the guard refused: the
// kill switch, an open or busy breaker, a half-open breaker whose probe
// another hook holds, or a spent deadline. Callers treat it like any other
// daemon error — fail open (I9) — and can errors.As it to stay silent.
type SkipError struct {
	Verdict Verdict
	Until   time.Time
	Reason  string
}

func (e *SkipError) Error() string {
	msg := "fathom hook guard: " + string(e.Verdict)
	if !e.Until.IsZero() {
		msg += " until " + e.Until.UTC().Format(time.RFC3339)
	}
	if e.Reason != "" {
		msg += " (" + e.Reason + ")"
	}
	return msg
}

// VerdictDeadline marks an RPC not sent because the hook's deadline had
// already passed.
const VerdictDeadline Verdict = "deadline_spent"

// Retryable reports a connection that was refused or reset — the only
// failures worth one retry inside a hook's deadline (§11.1). A timeout is
// never retryable: the daemon is slow, and a retry only piles on.
func Retryable(err error) bool {
	if err == nil {
		return false
	}
	var se *SkipError
	if errors.As(err, &se) {
		return false
	}
	return errors.Is(err, syscall.ECONNREFUSED) || errors.Is(err, syscall.ECONNRESET)
}

// Transport wraps a base RoundTripper with the active hook guard: when
// Active returns nil (every non-hook process) it is a pure pass-through.
type Transport struct {
	Base   http.RoundTripper
	Active func() *Guard
}

// RoundTrip implements http.RoundTripper.
func (t *Transport) RoundTrip(req *http.Request) (*http.Response, error) {
	base := t.Base
	if base == nil {
		base = http.DefaultTransport
	}
	var g *Guard
	if t.Active != nil {
		g = t.Active()
	}
	if g == nil {
		return base.RoundTrip(req)
	}
	return g.roundTrip(base, req)
}

func (g *Guard) countSkip(v Verdict) {
	skipMu.Lock()
	if g.skips == nil {
		g.skips = map[Verdict]int{}
	}
	g.skips[v]++
	skipMu.Unlock()
}

var skipMu sync.Mutex

// Skips returns how many RPCs this guard refused, by verdict.
func (g *Guard) Skips() map[Verdict]int {
	skipMu.Lock()
	defer skipMu.Unlock()
	out := make(map[Verdict]int, len(g.skips))
	for k, v := range g.skips {
		out[k] = v
	}
	return out
}

func (g *Guard) skip(v Verdict, d Decision) error {
	g.countSkip(v)
	return &SkipError{Verdict: v, Until: d.Until, Reason: d.Reason}
}

func (g *Guard) roundTrip(base http.RoundTripper, req *http.Request) (*http.Response, error) {
	target := req.URL.Scheme + "://" + req.URL.Host
	if g.Deadline.Spent() {
		return nil, g.skip(VerdictDeadline, Decision{})
	}
	d := BreakerAllows(g, target)
	switch d.Verdict {
	case Allow:
	case HalfOpen:
		if err := g.halfOpen(base, target, req.Context()); err != nil {
			return nil, err
		}
	default:
		return nil, g.skip(d.Verdict, d)
	}

	ctx, cancel := g.Deadline.Context(req.Context())
	out := req.Clone(ctx)
	if v := g.Deadline.HeaderValue(); v != "" {
		out.Header.Set(DeadlineHeader, v)
	}
	resp, err := base.RoundTrip(out)
	if err != nil {
		cancel()
		if req.Context().Err() == nil || errors.Is(err, context.DeadlineExceeded) {
			// The caller did not give up on its own: the daemon refused,
			// reset, or outlasted the hook's deadline — a failure. A
			// caller's own cancellation is not the daemon's fault.
			g.RecordFailure(target, err)
		}
		return nil, err
	}
	switch {
	case resp.StatusCode == http.StatusServiceUnavailable && resp.Header.Get("Retry-After") != "":
		g.RecordBusy(target, parseRetryAfter(resp.Header.Get("Retry-After")))
	case resp.StatusCode >= 500:
		g.RecordFailure(target, fmt.Errorf("HTTP %d", resp.StatusCode))
	default:
		g.RecordSuccess(target)
	}
	resp.Body = &cancelBody{ReadCloser: resp.Body, cancel: cancel}
	return resp, nil
}

// halfOpen runs the single probe: whoever wins the O_EXCL lease re-reads
// the state (a racer that read "open" before the winner closed it must not
// probe again) and sends GET /healthz under ProbeTimeout; every loser
// fails open at once.
func (g *Guard) halfOpen(base http.RoundTripper, target string, parent context.Context) error {
	release, ok := g.acquireLease(target)
	if !ok {
		return g.skip(Open, Decision{Reason: "half-open probe held by another hook"})
	}
	defer release()
	d := BreakerAllows(g, target)
	switch d.Verdict {
	case Allow:
		return nil
	case HalfOpen:
	default:
		return g.skip(d.Verdict, d)
	}
	ctx, cancel := context.WithTimeout(parent, ProbeTimeout)
	defer cancel()
	ctx, cancelDeadline := g.Deadline.Context(ctx)
	defer cancelDeadline()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, target+"/healthz", nil)
	if err != nil {
		return g.skip(Open, d)
	}
	resp, err := base.RoundTrip(req)
	if err == nil {
		_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 4<<10))
		resp.Body.Close()
		if resp.StatusCode >= 200 && resp.StatusCode < 300 {
			g.RecordSuccess(target)
			return nil
		}
		err = fmt.Errorf("healthz HTTP %d", resp.StatusCode)
	}
	g.RecordProbeFailure(target, err)
	return g.skip(Open, Decision{Verdict: Open, Reason: errString(err)})
}

func parseRetryAfter(v string) time.Duration {
	if secs, err := strconv.Atoi(v); err == nil && secs > 0 {
		return time.Duration(secs) * time.Second
	}
	if t, err := http.ParseTime(v); err == nil {
		return time.Until(t)
	}
	return 0
}

// cancelBody releases the RPC's derived context when the caller closes the
// body, so the deadline context never leaks past the response.
type cancelBody struct {
	io.ReadCloser
	cancel context.CancelFunc
}

func (b *cancelBody) Close() error {
	err := b.ReadCloser.Close()
	b.cancel()
	return err
}

// IsTimeout reports a deadline or network timeout.
func IsTimeout(err error) bool {
	if errors.Is(err, context.DeadlineExceeded) {
		return true
	}
	var ne net.Error
	return errors.As(err, &ne) && ne.Timeout()
}
