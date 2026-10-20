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
	if req.URL.Path == QueuedRefreshPath {
		return g.queuedRoundTrip(base, req, target)
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
	if v := rpcDeadlineHeader(req.Context(), g.Deadline); v != "" {
		out.Header.Set(DeadlineHeader, v)
	}
	started := g.now()
	resp, err := base.RoundTrip(out)
	if err != nil {
		cancel()
		if !callerGaveUp(req.Context(), err, time.Now()) {
			g.RecordFailure(target, err)
		}
		return nil, err
	}
	switch {
	case resp.StatusCode == http.StatusServiceUnavailable && resp.Header.Get(BusyScopeHeader) == BusyScopeClass:
		// One class's refusal (its slots full, or a deadline the tool
		// cannot meet): the caller gets its named 503, and every other
		// hook call — a point or index call the daemon would serve — is
		// not silenced for Retry-After. Not a failure either.
	case resp.StatusCode == http.StatusServiceUnavailable && resp.Header.Get("Retry-After") != "":
		g.RecordBusy(target, parseRetryAfter(resp.Header.Get("Retry-After")))
	case resp.StatusCode >= 500:
		g.RecordFailure(target, fmt.Errorf("HTTP %d", resp.StatusCode))
	default:
		g.RecordSuccessSince(target, started)
	}
	resp.Body = &cancelBody{ReadCloser: resp.Body, cancel: cancel}
	return resp, nil
}

// QueuedRefreshPath is the daemon route the post-commit hook posts to. The
// daemon queues the refresh (202) and nothing waits on it, so it is exempt
// from the breaker the edit-time refresh opens (O1449).
const QueuedRefreshPath = "/refresh-request"

// queuedRoundTrip sends a queued-refresh request past an open or busy
// breaker. The kill switch (disabled.json, paused.json) still applies. Its
// outcome is not recorded: a success must not close a breaker the edit-time
// calls opened, and a failure must not deepen one.
func (g *Guard) queuedRoundTrip(base http.RoundTripper, req *http.Request, target string) (*http.Response, error) {
	if v, reason, ok := g.killSwitch(target); ok {
		return nil, g.skip(v, Decision{Verdict: v, Reason: reason})
	}
	ctx, cancel := g.Deadline.Context(req.Context())
	out := req.Clone(ctx)
	if v := rpcDeadlineHeader(req.Context(), g.Deadline); v != "" {
		out.Header.Set(DeadlineHeader, v)
	}
	resp, err := base.RoundTrip(out)
	if err != nil {
		cancel()
		return nil, err
	}
	resp.Body = &cancelBody{ReadCloser: resp.Body, cancel: cancel}
	return resp, nil
}

// callerGaveUp reports an RPC error that is the caller's own doing — its
// context cancelled before any deadline — which is not the daemon's fault
// and is not counted. Everything else is a failure: a refused or reset
// connection, the hook deadline, and the caller's http.Client.Timeout.
// The last matters most: because http.DefaultTransport is wrapped (not a
// *http.Transport), net/http enforces Client.Timeout by cancelling the
// request context, so a hung daemon usually surfaces as context.Canceled
// with the context's deadline already passed. On hook paths those client
// timeouts (edit-hook --pre's 2 s, the primer's 2 s) fire before the hook
// deadline, so trusting only DeadlineExceeded left the breaker closed on a
// hung daemon about half the time.
func callerGaveUp(ctx context.Context, err error, now time.Time) bool {
	if ctx.Err() == nil || IsTimeout(err) {
		return false
	}
	if dl, ok := ctx.Deadline(); ok && !now.Before(dl) {
		return false // the caller's own timeout ran out: the daemon was too slow
	}
	return true
}

// halfOpen runs the single probe: whoever wins the lease re-reads
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
			g.RecordProbeSuccess(target)
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
