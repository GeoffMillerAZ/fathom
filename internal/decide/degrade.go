package decide

// degrade.go — JF-6 (docs/design/judgment-foundations.md §4, the degrade
// rule): a configured provider that fails a call (a timeout, a 429 or 529,
// the daemon's refuse rung or safe mode) or refuses a question (DP-33)
// yields exactly one disclosed line and the asking site's deterministic
// path. Built from the kit's own
// decorators (agentic-kit DP-18 NewFallback, DP-21 ComposeFromConfig): the
// provider falls back to decision.None, whose ErrNoProvider is the signal
// every site already reads as "no provider", and the fallback's required
// Observe hook writes the line. No cache, table or background job (R5).

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"sync"

	"github.com/geoffmilleraz/fathom/internal/config"
	decision "github.com/geoffmilleraz/fathom/third_party/agentic-kit/foundations/decision"
	"github.com/geoffmilleraz/fathom/third_party/agentic-kit/foundations/decision/openai"
	"github.com/geoffmilleraz/fathom/third_party/agentic-kit/foundations/decision/typesafe"
)

// Option configures Open.
type Option func(*openOptions)

type openOptions struct {
	disclose io.Writer
}

// WithDisclose names where the degrade line goes (the process's stderr: the
// daemon's log, a CLI verb's terminal). Without it the line goes to
// os.Stderr: a fallback is never silent.
func WithDisclose(w io.Writer) Option {
	return func(o *openOptions) { o.disclose = w }
}

// noneLeaf is the Registry key of the fallback's last Decider.
const noneLeaf = "none"

// degradeObserver is the Registry key of the hook that writes the line.
const degradeObserver = "disclose"

// degradePolicy is the composition policy derived from [decide]: the
// configured provider, then decision.None, as a fallback named for the
// provider (so Name, which every site and doctor print, is unchanged).
func degradePolicy(cfg config.Decide) decision.Policy {
	return decision.Policy{Fallback: &decision.FallbackPolicy{
		Name:     cfg.Provider,
		Deciders: []decision.Policy{{Provider: cfg.Provider}, {Provider: noneLeaf}},
		Observe:  degradeObserver,
	}}
}

// degrader writes one line per Decide call the provider failed.
type degrader struct {
	mu sync.Mutex
	w  io.Writer
}

func (d *degrader) observe(r decision.FallbackReport) {
	if len(r.Failures) == 0 {
		return // answered, or a Request that failed Validate (a caller bug, not a degrade)
	}
	f := r.Failures[0]
	if r.Answered >= 0 && !errors.Is(f.Err, decision.ErrDeclined) {
		return // the provider answered; a later leaf's miss is not a degrade
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	fmt.Fprintf(d.w, "warn: decision port degraded (%s): %s: %s; the site answered on its deterministic path\n",
		degradeClass(f.Err), f.Name, oneLine(f.Err.Error()))
}

// degradeClass names why the provider answered nothing.
func degradeClass(err error) string {
	var tse *typesafe.TypeSafeError
	var oae *openai.OpenAIError
	switch {
	case errors.Is(err, decision.ErrDeclined):
		return "declined" // the provider refused a question (Response.Declined)
	case errors.Is(err, decision.ErrNoProvider):
		return "closed" // the daemon's closure (safe mode, the refuse rung)
	case errors.Is(err, context.DeadlineExceeded):
		return "timeout"
	case errors.As(err, &oae) && oae.QuotaExhausted():
		return "quota exhausted" // OpenAI's 429 insufficient_quota: no credit left, not a rate limit
	case errors.As(err, &tse) && tse.Status == 429, errors.As(err, &oae) && oae.Status == 429:
		return "rate limited"
	case errors.As(err, &tse) && tse.Status == 529:
		return "overloaded"
	case errors.As(err, &oae) && oae.Retryable():
		return "unavailable" // OpenAI's 500, 502, 503 or 504
	}
	return "error"
}

// oneLine keeps the disclosure to one line and a readable length.
func oneLine(s string) string {
	s = strings.Join(strings.Fields(s), " ")
	if len(s) > 240 {
		s = s[:240] + "..."
	}
	return s
}

// DegradedError is what a site's Decide call returns when the configured
// provider answered nothing: its Error is the provider's own error text,
// unchanged (route's error field, the re-rank's rank_note and the
// decisions log read as before), and it is decision.ErrNoProvider, so every
// site takes the path it takes with no provider (the compile pre-filter
// keeps every section instead of aborting the compile).
type DegradedError struct {
	// Provider is the configured provider's name.
	Provider string
	// Err is the provider's (or the daemon gate's) own failure.
	Err error
}

func (e *DegradedError) Error() string { return e.Err.Error() }

// Unwrap exposes the provider's failure and decision.ErrNoProvider.
func (e *DegradedError) Unwrap() []error { return []error{e.Err, decision.ErrNoProvider} }

// degrading is the composed fallback seen by a site: the provider's answer
// unchanged, or a *DegradedError once the line is written.
type degrading struct {
	fb *decision.Fallback
}

func (d degrading) Name() string { return d.fb.Name() }

// A Response that declines any question is a degrade too (the kit's
// Fallback returns it with a nil error once decision.None also misses): its
// other answers are dropped and the site takes its deterministic path for
// the whole Request, so a refusal is never read as an answer, a false or a
// zero, and no site has to reason about a partly answered batch.
func (d degrading) Decide(ctx context.Context, req decision.Request) (decision.Response, error) {
	resp, report, err := d.fb.DecideReport(ctx, req)
	if err == nil && len(resp.Declined) > 0 && len(report.Failures) > 0 {
		return decision.Response{Provider: d.fb.Name()}, &DegradedError{Provider: d.fb.Name(), Err: report.Failures[0].Err}
	}
	if err == nil || len(report.Failures) == 0 {
		return resp, err
	}
	return resp, &DegradedError{Provider: d.fb.Name(), Err: report.Failures[0].Err}
}

// Capabilities declares what the fallback declares (agentic-kit DP-15).
func (d degrading) Capabilities() decision.Capabilities { return decision.CapabilitiesOf(d.fb) }

// compose builds the degrading fallback over leaf (the provider adapter,
// possibly gated) from cfg's policy.
func compose(cfg config.Decide, leaf decision.Decider, obs *degrader) (decision.Decider, error) {
	c, err := decision.ComposeFromConfig(degradePolicy(cfg), decision.Registry{
		Deciders:          map[string]decision.Decider{cfg.Provider: leaf, noneLeaf: decision.None{}},
		FallbackObservers: map[string]func(decision.FallbackReport){degradeObserver: obs.observe},
	})
	if err != nil {
		return nil, err
	}
	fb, ok := c.Root().(*decision.Fallback)
	if !ok {
		return nil, fmt.Errorf("decide: the degrade policy composed a %T, want a fallback", c.Root())
	}
	return degrading{fb: fb}, nil
}

// Gated returns the port's Decider with gate applied to the provider
// adapter inside the fallback, so a gate's refusal (serve-http's
// closingDecider in safe mode or at the refuse rung, gateDecider's
// admission skip) degrades like any provider failure: one line, the
// deterministic path. With no provider it returns the Decider unchanged
// (decision.None).
func (p *Ports) Gated(gate func(decision.Decider) decision.Decider) decision.Decider {
	if p == nil || p.Adapter == nil || p.degrade == nil {
		if p == nil {
			return nil
		}
		return p.Decider
	}
	d, err := compose(p.Cfg, gate(p.Adapter), p.degrade)
	if err != nil {
		// Unreachable: the same policy composed at Open. Never fail open
		// to the ungated adapter.
		p.degrade.mu.Lock()
		fmt.Fprintf(p.degrade.w, "warn: decision port disabled: %v\n", err)
		p.degrade.mu.Unlock()
		return decision.None{}
	}
	return d
}

func newDegrader(opts []Option) *degrader {
	o := openOptions{disclose: os.Stderr}
	for _, opt := range opts {
		opt(&o)
	}
	if o.disclose == nil {
		o.disclose = os.Stderr
	}
	return &degrader{w: o.disclose}
}
