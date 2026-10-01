package decision

import (
	"context"
	"errors"
	"fmt"
	"slices"
)

// FallbackOptions configures NewFallback. It is plain data, so a composer
// can build it from configuration.
type FallbackOptions struct {
	// Name is what the Fallback's Name returns and the Provider of the
	// Response returned when every Decider fails. Empty means "fallback".
	Name string
	// Deciders are tried in order; the next runs only when the previous
	// one returned an error. At least one is required, and none may be nil.
	Deciders []Decider
	// Observe receives the report of every Decide call, including each
	// failure that preceded the answer. It is required: a Fallback without
	// it would switch providers silently, which docs/fast-judgment.md
	// forbids ("never fall back silently to a different provider"). It may
	// be called concurrently from concurrent Decide calls.
	Observe func(FallbackReport)
}

// FallbackFailure is one Decider's failure during a Fallback call.
type FallbackFailure struct {
	// Index is the Decider's position in FallbackOptions.Deciders.
	Index int
	// Name is the Decider's Name.
	Name string
	// Err is the error the Decider returned, unwrapped by the Fallback, so
	// errors.Is and errors.As (ErrNoProvider, *typesafe.TypeSafeError) work on
	// it.
	Err error
}

// FallbackReport discloses one Fallback call.
type FallbackReport struct {
	// Answered is the index of the Decider whose Response was returned, or
	// -1 when none answered (every Decider failed, or the Request failed
	// Validate).
	Answered int
	// Provider and Model are the returned Response's, empty when none
	// answered.
	Provider, Model string
	// Failures lists, in order, every Decider that failed before the
	// answer (or every Decider tried, when none answered).
	Failures []FallbackFailure
}

// Fallback is a Decider that tries its Deciders in order and returns the
// first Response that comes with a nil error: an offline or second provider
// used when the first fails or is unavailable.
//
// Which errors fall through: every error a Decider returns, including
// ErrNoProvider (the Decider is unavailable) and a non-Retryable
// *typesafe.TypeSafeError such as a 401 or a 422 (the next provider has its
// own key and its own wire, so it may still answer). Two things do not: a
// Request that fails Validate is refused before any Decider is tried (every
// Decider would refuse it), and a done ctx stops the Fallback before the
// next Decider. A Response with a nil error is returned as is even when
// every question is in Invalid: escalating those is Cascade's job.
//
// Never silent: the returned Response is the answering Decider's, unchanged,
// so its Provider and Model name the Decider that answered, and every
// earlier failure reaches FallbackOptions.Observe (required) and the
// FallbackReport DecideReport returns.
//
// Concurrency: a Fallback holds no mutable state after NewFallback, so it
// is safe for concurrent use when its Deciders and Observe are.
type Fallback struct {
	name     string
	deciders []Decider
	observe  func(FallbackReport)
}

// NewFallback returns a Fallback over opts.Deciders. It fails when there is
// no Decider, a Decider is nil, or Observe is nil.
func NewFallback(opts FallbackOptions) (*Fallback, error) {
	if len(opts.Deciders) == 0 {
		return nil, errors.New("decision: NewFallback: at least one Decider is required")
	}
	for i, d := range opts.Deciders {
		if isNilDecider(d) {
			return nil, fmt.Errorf("decision: NewFallback: Decider %d is nil", i)
		}
	}
	if opts.Observe == nil {
		return nil, errors.New("decision: NewFallback: Observe is required so a fallback is never silent")
	}
	name := opts.Name
	if name == "" {
		name = "fallback"
	}
	return &Fallback{name: name, deciders: slices.Clone(opts.Deciders), observe: opts.Observe}, nil
}

// Name returns FallbackOptions.Name, or "fallback".
func (f *Fallback) Name() string { return f.name }

// Decide is DecideReport without the report (Observe still receives it).
func (f *Fallback) Decide(ctx context.Context, req Request) (Response, error) {
	resp, _, err := f.DecideReport(ctx, req)
	return resp, err
}

// DecideReport answers req with the first Decider that returns a nil error
// (see Fallback) and returns its Response unchanged with the report. When
// every Decider fails, or ctx is done after a failure, it returns a
// Response naming the Fallback and an error joining every failure (and
// ctx's error), so errors.Is and errors.As see each of them.
func (f *Fallback) DecideReport(ctx context.Context, req Request) (Response, FallbackReport, error) {
	resp, report, err := f.decide(ctx, req)
	f.observe(report)
	return resp, report, err
}

// decide is DecideReport before Observe.
func (f *Fallback) decide(ctx context.Context, req Request) (Response, FallbackReport, error) {
	report := FallbackReport{Answered: -1}
	if err := Validate(req); err != nil {
		return Response{}, report, err
	}
	var errs []error
	for i, d := range f.deciders {
		if i > 0 && ctx.Err() != nil {
			errs = append(errs, ctx.Err())
			break
		}
		resp, err := d.Decide(ctx, req)
		if err == nil {
			report.Answered, report.Provider, report.Model = i, resp.Provider, resp.Model
			return resp, report, nil
		}
		report.Failures = append(report.Failures, FallbackFailure{Index: i, Name: d.Name(), Err: err})
		errs = append(errs, fmt.Errorf("%s: %w", d.Name(), err))
	}
	return Response{Provider: f.name}, report, fmt.Errorf("decision: fallback: no Decider answered: %w", errors.Join(errs...))
}

// Capabilities declares what the Fallback answers, derived from its
// Deciders as Cascade's is (see mergeCapabilities): any of them may answer,
// so it reports the union of kinds, the loosest limits and the worst
// latency and cost. It makes no call.
func (f *Fallback) Capabilities() Capabilities { return mergeCapabilities(f.deciders) }
