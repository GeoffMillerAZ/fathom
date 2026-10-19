package decide

// degrade.go — JF-6 (docs/design/judgment-foundations.md §4, the degrade
// rule): a configured provider that fails a call (a timeout, a 429 or 529,
// the daemon's refuse rung or safe mode) or refuses a question (DP-33)
// yields exactly one disclosed line and the asking site's deterministic
// path. Built from the kit's own
// decorators (agentic-kit DP-18 NewFallback, DP-21 ComposeFromConfig): the
// provider falls back to decision.None, whose ErrNoProvider is the signal
// every site already reads as "no provider", and the composed port writes
// the line from the fallback's report (JG-3: once per session, site and
// class, keyed by KS-14's Scope, which the kit's Observe hook cannot see).
// No cache, table or background job (R5).

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net/http"
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
	disclose   io.Writer
	keyIO      config.KeyIO
	httpClient *http.Client
	userConfig string
	userSet    bool
	killFile   string
	stat       func(string) (fs.FileInfo, error)
	laneModel  func() string
}

// WithDisclose names where the degrade line goes (the process's stderr: the
// daemon's log, a CLI verb's terminal). Without it the line goes to
// os.Stderr: a fallback is never silent.
func WithDisclose(w io.Writer) Option {
	return func(o *openOptions) { o.disclose = w }
}

// WithKeyIO names the seams a remote provider's key source reads through
// (KS-1): a test's counting fakes prove a closed port touches none. Without
// it, config.OSKeyIO.
func WithKeyIO(k config.KeyIO) Option {
	return func(o *openOptions) { o.keyIO = k }
}

// WithHTTPClient names the HTTP client a remote adapter dials through
// (KS-1): a test's counting transport proves a closed port dials nothing.
// Without it, the kit adapter's own default.
func WithHTTPClient(c *http.Client) Option {
	return func(o *openOptions) { o.httpClient = c }
}

// WithUserConfig names the user config whose [decide] enabled the open port
// re-checks before each ask (KS-2), reloading it only when its mtime or
// size changed: the path the caller loaded as the user layer ("" checks
// none). Without it, config.DefaultUserPath.
func WithUserConfig(path string) Option {
	return func(o *openOptions) { o.userConfig, o.userSet = path, true }
}

// WithLaneModel names provider llm's model by a function Open calls only
// after the lane is built, in place of Open's model argument: a port a
// live kill held closed at Open (O1132) resolves its profile, and so its
// model, only on the first ask after the kill lifts.
func WithLaneModel(model func() string) Option {
	return func(o *openOptions) { o.laneModel = model }
}

// resolveOptions applies opts over the defaults: os.Stderr, the machine's
// key seams, the kill file under the per-user fathom directory and the
// default user config.
func resolveOptions(opts []Option) openOptions {
	o := openOptions{disclose: os.Stderr, keyIO: config.OSKeyIO(), stat: os.Stat}
	for _, opt := range opts {
		opt(&o)
	}
	if o.disclose == nil {
		o.disclose = os.Stderr
	}
	if !o.userSet {
		o.userConfig = config.DefaultUserPath()
	}
	o.killFile = KillFilePath()
	return o
}

// KeyIOOf is the key seams opts name (config.OSKeyIO when none): doctor
// probes a key source through the same ones Open would.
func KeyIOOf(opts ...Option) config.KeyIO { return resolveOptions(opts).keyIO }

// noneLeaf is the Registry key of the fallback's last Decider.
const noneLeaf = "none"

// degradeObserver is the Registry key of the fallback's Observe hook (the
// kit requires one; degrading.Decide writes the line from the report).
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

// degrader writes the degrade line: once per session, site and class
// (JG-3), while every degraded ask still returns its *DegradedError, so
// each answer's own receipt names the miss (MissClass).
type degrader struct {
	mu sync.Mutex
	w  io.Writer
	// disclosed is each (scope, class) already told, in memory for the
	// process's lifetime (no table, R5).
	disclosed map[degradeKey]bool
}

// degradeKey is one degrade line's key: who asked (KS-14's Scope) and why
// the provider answered nothing.
type degradeKey struct {
	scope Scope
	class string
}

// observe writes r's degrade line for an ask from scope, unless that
// session, site and class already heard it.
func (d *degrader) observe(scope Scope, r decision.FallbackReport) {
	if d == nil || len(r.Failures) == 0 {
		return // answered, or a Request that failed Validate (a caller bug, not a degrade)
	}
	f := r.Failures[0]
	if r.Answered >= 0 && !errors.Is(f.Err, decision.ErrDeclined) {
		return // the provider answered; a later leaf's miss is not a degrade
	}
	class := degradeClass(f.Err)
	d.mu.Lock()
	defer d.mu.Unlock()
	key := degradeKey{scope: scope, class: class}
	if d.disclosed[key] {
		return
	}
	if d.disclosed == nil {
		d.disclosed = map[degradeKey]bool{}
	}
	d.disclosed[key] = true
	who := ""
	if s := scope.String(); s != "" {
		who = " for " + s
	}
	fmt.Fprintf(d.w, "warn: decision port degraded (%s)%s: %s: %s; the site answered on its deterministic path\n",
		class, who, f.Name, oneLine(f.Err.Error()))
}

// degradeClass names why the provider answered nothing.
func degradeClass(err error) string {
	var tse *typesafe.TypeSafeError
	var oae *openai.OpenAIError
	var pre *planRefusedError
	switch {
	case errors.As(err, &pre):
		return pre.class // the request plan sent nothing (requestplan.go)
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
// unchanged, or a *DegradedError once the line is written (or was already
// written for this session, site and class).
type degrading struct {
	fb  *decision.Fallback
	obs *degrader
}

func (d degrading) Name() string { return d.fb.Name() }

// A Response that declines any question is a degrade too (the kit's
// Fallback returns it with a nil error once decision.None also misses): its
// other answers are dropped and the site takes its deterministic path for
// the whole Request, so a refusal is never read as an answer, a false or a
// zero, and no site has to reason about a partly answered batch.
func (d degrading) Decide(ctx context.Context, req decision.Request) (decision.Response, error) {
	resp, report, err := d.fb.DecideReport(ctx, req)
	d.obs.observe(ScopeFrom(ctx), report)
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
// possibly gated) from cfg's policy, with plan's request plan in front of
// leaf (I61, requestplan.go): every port that reaches a provider is
// composed here, so every ask leaves through the plan. A nil plan is a
// fresh one over cfg (a Ports built in code), never none.
func compose(cfg config.Decide, leaf decision.Decider, obs *degrader, plan *requestPlanner) (decision.Decider, error) {
	if plan == nil {
		plan = newRequestPlanner(cfg, nil, nil)
	}
	c, err := decision.ComposeFromConfig(degradePolicy(cfg), decision.Registry{
		Deciders: map[string]decision.Decider{cfg.Provider: plan.guard(leaf), noneLeaf: decision.None{}},
		// The kit requires an Observe hook; the line is written by
		// degrading.Decide, which knows who asked.
		FallbackObservers: map[string]func(decision.FallbackReport){degradeObserver: func(decision.FallbackReport) {}},
	})
	if err != nil {
		return nil, err
	}
	fb, ok := c.Root().(*decision.Fallback)
	if !ok {
		return nil, fmt.Errorf("decide: the degrade policy composed a %T, want a fallback", c.Root())
	}
	return degrading{fb: fb, obs: obs}, nil
}

// Gated returns the port's Decider with gate applied to the provider
// adapter inside the fallback, so a gate's refusal (serve-http's
// closingDecider in safe mode or at the refuse rung, gateDecider's
// admission skip) degrades like any provider failure: one line, the
// deterministic path. With no provider it returns the Decider unchanged
// (decision.None).
func (p *Ports) Gated(gate func(decision.Decider) decision.Decider) decision.Decider {
	if p != nil && p.reopen != nil {
		// O1132: gate is applied to the adapter when the port finishes
		// opening, after the kill lifts.
		return &reopening{r: p.reopen, gate: gate}
	}
	if p == nil || p.Adapter == nil || p.degrade == nil {
		if p == nil {
			return nil
		}
		return p.Decider
	}
	d, err := compose(p.Cfg, gate(p.Adapter), p.degrade, p.plan)
	if err != nil {
		// Unreachable: the same policy composed at Open. Never fail open
		// to the ungated adapter.
		p.degrade.mu.Lock()
		fmt.Fprintf(p.degrade.w, "warn: decision port disabled: %v\n", err)
		p.degrade.mu.Unlock()
		return decision.None{}
	}
	if p.live != nil {
		// KS-2: every composed port re-checks the user's kill before each
		// ask, the one serve-http gates at startup included.
		return killable{inner: d, gate: p.live}
	}
	return d
}
