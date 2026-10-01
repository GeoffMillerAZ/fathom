package decision

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"slices"
	"sort"
	"time"
)

// DefaultEscalateBelow is the confidence below which a Cascade escalates an
// answer when CascadeOptions.EscalateBelow is zero. It equals UnsureBelow
// and, like it, is an uncalibrated starting value (decision 0017 rule 3):
// calibration is per site, question version, provider and resolved model,
// so a caller that has a reading for its site sets its own threshold.
const DefaultEscalateBelow = UnsureBelow

// AnswerSource names who produced one answer a decorator returned: the
// inner Decider's position and the Provider and Model of the Response the
// answer came from. It is the decorators' per-answer provenance, kept in
// their reports because Response carries one Provider and Model per call.
type AnswerSource struct {
	// Stage is the index, in consultation order, of the inner Decider that
	// produced the answer (0 for a decorator with one inner Decider).
	Stage int
	// Provider is the Provider of the Response the answer came from.
	Provider string
	// Model is the Model of the Response the answer came from.
	Model string
	// Cached is true when the answer was served from a cache entry rather
	// than produced by a provider during this call (see Cached).
	Cached bool
}

// CascadeOptions configures NewCascade. It is plain data, so a composer
// can build it from configuration.
type CascadeOptions struct {
	// Name is what the Cascade's Name returns and the Provider of a
	// Response whose answers come from more than one stage. Empty means
	// "cascade".
	Name string
	// Stages are the Deciders to consult, cheapest first, in the order
	// given unless OrderByCost is set. At least one is required, and none
	// may be nil.
	Stages []Decider
	// OrderByCost stably sorts Stages by their declared CostClass (read
	// with CapabilitiesOf): none, low, medium, high, then unknown, so a
	// stage that declares no cost is consulted last among equals of the
	// caller's order.
	OrderByCost bool
	// EscalateBelow is the confidence below which an answer escalates to
	// the next stage. Zero means DefaultEscalateBelow; a negative value
	// never escalates on confidence (only on abstain). It must not exceed
	// 1. A score answer's Confidence is always 1, so a score question
	// escalates only on abstain.
	EscalateBelow float64
	// Observe, when set, receives the report of every Decide call. It may
	// be called concurrently from concurrent Decide calls.
	Observe func(CascadeReport)
}

// CascadeStep is one stage's part in one Cascade call.
type CascadeStep struct {
	// Stage is the stage's index in consultation order.
	Stage int
	// Name is the stage Decider's Name.
	Name string
	// Provider and Model are the stage Response's, empty when the stage
	// abstained with nothing to report.
	Provider, Model string
	// Asked lists the question IDs sent to the stage, in request order.
	Asked []string
	// Resolved lists the question IDs the stage answered at or above the
	// threshold, in request order.
	Resolved []string
	// Escalated maps each asked question the stage did not resolve to why:
	// its Invalid reason, an absent answer, an abstention, or a confidence
	// below the threshold.
	Escalated map[string]string
	// Abstained is true when the stage returned ErrNoProvider.
	Abstained bool
	// Usage is the stage Response's Usage.
	Usage Usage
	// Latency is the stage Response's Latency.
	Latency time.Duration
}

// CascadeReport discloses one Cascade call: which stage answered each
// question and why every other question moved on.
type CascadeReport struct {
	// Sources maps every question ID in the returned Response's Answers to
	// the stage that answered it.
	Sources map[string]AnswerSource
	// Steps lists every stage consulted, in order.
	Steps []CascadeStep
	// LowConfidence lists, in request order, the answered questions whose
	// returned answer is still below the threshold because no later stage
	// resolved them.
	LowConfidence []string
}

// Responses splits merged, a Response this report describes, into one
// Response per stage that answered at least one question, in stage order.
// Each carries that stage's Provider, Model, Usage and Latency and only
// its answers, so Records attributes every answer to the provider and
// model that produced it. Invalid entries are not included.
func (r CascadeReport) Responses(merged Response) []Response {
	var out []Response
	for _, step := range r.Steps {
		answers := map[string]Answer{}
		for id, src := range r.Sources {
			if src.Stage == step.Stage {
				if a, ok := merged.Answers[id]; ok {
					answers[id] = a
				}
			}
		}
		if len(answers) == 0 {
			continue
		}
		out = append(out, Response{
			Provider: step.Provider,
			Model:    step.Model,
			Answers:  answers,
			Invalid:  map[string]string{},
			Usage:    step.Usage,
			Latency:  step.Latency,
		})
	}
	return out
}

// Cascade is a Decider that consults its stages cheapest first and
// escalates, per question, only what a stage did not resolve: a question
// the stage abstained on (ErrNoProvider, named in Invalid, or absent from
// the Response) or answered below the confidence threshold. Each later
// stage gets a sub-Request with the same State and only the unresolved
// questions, in request order; the Cascade stops when every question is
// resolved or the stages run out, then merges.
//
// Provenance: an answer is always the one the stage that produced it
// returned, and when every answer comes from one stage the merged Response
// carries that stage's Provider and Model. When answers come from more
// than one stage the Response carries the Cascade's Name and an empty
// Model, so no answer is ever reported under another stage's Provider or
// Model; DecideReport's CascadeReport (also passed to
// CascadeOptions.Observe) says which stage answered each question, and
// CascadeReport.Responses splits the merge back per stage. Usage and
// Latency are totals over every stage consulted.
//
// A question still unresolved after the last stage keeps the latest valid
// answer any stage gave (listed in CascadeReport.LowConfidence) or, when
// none gave one, is named in Invalid with the last reason.
//
// Concurrency: a Cascade holds no mutable state after NewCascade, so it is
// safe for concurrent use when its stages are.
type Cascade struct {
	name    string
	stages  []Decider
	below   float64
	observe func(CascadeReport)
}

// NewCascade returns a Cascade over opts.Stages. It fails when there is no
// stage, a stage is nil, or EscalateBelow exceeds 1.
func NewCascade(opts CascadeOptions) (*Cascade, error) {
	if len(opts.Stages) == 0 {
		return nil, errors.New("decision: NewCascade: at least one stage is required")
	}
	for i, d := range opts.Stages {
		if isNilDecider(d) {
			return nil, fmt.Errorf("decision: NewCascade: stage %d is nil", i)
		}
	}
	if opts.EscalateBelow > 1 {
		return nil, fmt.Errorf("decision: NewCascade: EscalateBelow %v exceeds 1", opts.EscalateBelow)
	}
	stages := slices.Clone(opts.Stages)
	if opts.OrderByCost {
		sort.SliceStable(stages, func(i, j int) bool {
			return costRank(CapabilitiesOf(stages[i]).Cost) < costRank(CapabilitiesOf(stages[j]).Cost)
		})
	}
	below := opts.EscalateBelow
	if below == 0 {
		below = DefaultEscalateBelow
	}
	name := opts.Name
	if name == "" {
		name = "cascade"
	}
	return &Cascade{name: name, stages: stages, below: below, observe: opts.Observe}, nil
}

// Name returns CascadeOptions.Name, or "cascade".
func (c *Cascade) Name() string { return c.name }

// Stages returns the stage Names in consultation order.
func (c *Cascade) Stages() []string {
	names := make([]string, len(c.stages))
	for i, d := range c.stages {
		names[i] = d.Name()
	}
	return names
}

// Decide is DecideReport without the report (Observe still receives it).
func (c *Cascade) Decide(ctx context.Context, req Request) (Response, error) {
	resp, _, err := c.DecideReport(ctx, req)
	return resp, err
}

// DecideReport answers req through the stages (see Cascade) and returns
// the merged Response with its CascadeReport. It fails when req fails
// Validate (no stage is consulted), when a stage returns an error other
// than ErrNoProvider (that error, wrapped with the stage's Name; later
// stages are not consulted), or with an error wrapping ErrNoProvider when
// every stage abstained with ErrNoProvider.
func (c *Cascade) DecideReport(ctx context.Context, req Request) (Response, CascadeReport, error) {
	resp, report, err := c.decide(ctx, req)
	if c.observe != nil {
		c.observe(report)
	}
	return resp, report, err
}

// decide is DecideReport before Observe.
func (c *Cascade) decide(ctx context.Context, req Request) (Response, CascadeReport, error) {
	report := CascadeReport{Sources: map[string]AnswerSource{}}
	if err := Validate(req); err != nil {
		return Response{}, report, err
	}

	answers := map[string]Answer{} // resolved, or the latest valid low-confidence answer
	reasons := map[string]string{} // the latest reason a question is unanswered
	pending := slices.Clone(req.Questions)
	var usage Usage
	var latency time.Duration
	abstainedAll := true

	for i, stage := range c.stages {
		if len(pending) == 0 {
			break
		}
		step := CascadeStep{Stage: i, Name: stage.Name(), Asked: questionIDs(pending), Escalated: map[string]string{}}
		sresp, err := stage.Decide(ctx, Request{State: req.State, Questions: pending})
		if err != nil && !errors.Is(err, ErrNoProvider) {
			report.Steps = append(report.Steps, step)
			return Response{}, report, fmt.Errorf("decision: cascade stage %d (%s): %w", i, step.Name, err)
		}
		step.Provider, step.Model = sresp.Provider, sresp.Model
		step.Usage, step.Latency = sresp.Usage, sresp.Latency
		usage.InputTokens += sresp.Usage.InputTokens
		usage.OutputTokens += sresp.Usage.OutputTokens
		latency += sresp.Latency

		var next []Question
		if err != nil { // ErrNoProvider: the stage abstains on every question
			step.Abstained = true
			for _, q := range pending {
				step.Escalated[q.ID] = "abstained: " + err.Error()
				if _, kept := answers[q.ID]; !kept {
					reasons[q.ID] = step.Escalated[q.ID]
				}
			}
			report.Steps = append(report.Steps, step)
			continue
		}
		abstainedAll = false
		src := AnswerSource{Stage: i, Provider: sresp.Provider, Model: sresp.Model}
		for _, q := range pending {
			a, ok := sresp.Answers[q.ID]
			switch {
			case ok && a.Confidence >= c.below:
				answers[q.ID] = a
				report.Sources[q.ID] = src
				step.Resolved = append(step.Resolved, q.ID)
				continue
			case ok:
				answers[q.ID] = a
				report.Sources[q.ID] = src
				step.Escalated[q.ID] = fmt.Sprintf("confidence %v below %v", a.Confidence, c.below)
			default:
				why, invalid := sresp.Invalid[q.ID]
				if !invalid {
					why = "no answer"
				}
				step.Escalated[q.ID] = why
				if _, kept := answers[q.ID]; !kept {
					reasons[q.ID] = why
				}
			}
			next = append(next, q)
		}
		report.Steps = append(report.Steps, step)
		pending = next
	}

	merged := Response{Answers: map[string]Answer{}, Invalid: map[string]string{}, Usage: usage, Latency: latency}
	for _, q := range req.Questions {
		if a, ok := answers[q.ID]; ok {
			merged.Answers[q.ID] = a
			if a.Confidence < c.below {
				report.LowConfidence = append(report.LowConfidence, q.ID)
			}
			continue
		}
		merged.Invalid[q.ID] = reasons[q.ID]
	}
	merged.Provider, merged.Model = labelBySources(report.Sources, c.name)
	if abstainedAll && len(report.Steps) > 0 {
		return merged, report, fmt.Errorf("decision: cascade: every stage abstained: %w", ErrNoProvider)
	}
	return merged, report, nil
}

// Capabilities declares what the Cascade answers, derived from its stages:
// see mergeCapabilities. It makes no call.
func (c *Cascade) Capabilities() Capabilities { return mergeCapabilities(c.stages) }

// labelBySources returns the one (Provider, Model) every answer in sources
// shares, or (name, "") when the answers come from more than one source or
// there are none.
func labelBySources(sources map[string]AnswerSource, name string) (provider, model string) {
	if p, m, ok := sharedSource(sources); ok {
		return p, m
	}
	return name, ""
}

// sharedSource returns the (Provider, Model) every entry of sources shares,
// and false when they do not share one or sources is empty.
func sharedSource(sources map[string]AnswerSource) (provider, model string, ok bool) {
	first := true
	for _, src := range sources {
		if first {
			provider, model, first = src.Provider, src.Model, false
			continue
		}
		if src.Provider != provider || src.Model != model {
			return "", "", false
		}
	}
	return provider, model, !first
}

// questionIDs returns the IDs of qs, in order.
func questionIDs(qs []Question) []string {
	ids := make([]string, len(qs))
	for i, q := range qs {
		ids[i] = q.ID
	}
	return ids
}

// isNilDecider reports whether d is nil or holds a nil pointer, map,
// slice, func, channel or interface.
func isNilDecider(d Decider) bool {
	if d == nil {
		return true
	}
	v := reflect.ValueOf(d)
	switch v.Kind() {
	case reflect.Pointer, reflect.Interface, reflect.Func, reflect.Map, reflect.Slice, reflect.Chan:
		return v.IsNil()
	}
	return false
}

// costRank orders CostClass values cheapest first; an unrecognized class
// ranks with CostUnknown, last.
func costRank(c CostClass) int {
	switch c {
	case CostNone:
		return 0
	case CostLow:
		return 1
	case CostMedium:
		return 2
	case CostHigh:
		return 3
	default:
		return 4
	}
}

// latencyRank orders LatencyClass values fastest first; an unrecognized
// class ranks with LatencyUnknown, last.
func latencyRank(l LatencyClass) int {
	switch l {
	case LatencyNone:
		return 0
	case LatencySubSecond:
		return 1
	case LatencySeconds:
		return 2
	default:
		return 3
	}
}

// mergeCapabilities declares a decorator over ds: undeclared when any of
// ds is undeclared; otherwise the union of their Kinds, for each kind the
// loosest limit among the Deciders answering it (0, no limit, wins), the
// worst Latency and Cost, and, over the Deciders that answer any kind,
// SingleRoundTrip when all of them declare it, the smallest non-zero
// MaxQuestionsPerCall and their ConfidenceSemantics when they share one
// (unknown otherwise). A question beyond the limits of the Decider that
// receives it still lands in that Decider's Invalid.
func mergeCapabilities(ds []Decider) Capabilities {
	caps := make([]Capabilities, len(ds))
	for i, d := range ds {
		caps[i] = CapabilitiesOf(d)
		if !caps[i].Declared {
			return undeclaredCapabilities()
		}
	}
	out := Capabilities{Declared: true, Latency: LatencyNone, Cost: CostNone}
	var kinds []Kind
	var answering []Capabilities
	for _, c := range caps {
		kinds = append(kinds, c.Kinds...)
		if len(c.Kinds) > 0 {
			answering = append(answering, c)
		}
		if latencyRank(c.Latency) > latencyRank(out.Latency) {
			out.Latency = c.Latency
		}
		if costRank(c.Cost) > costRank(out.Cost) {
			out.Cost = c.Cost
		}
	}
	out.Kinds = canonicalKinds(kinds)
	out.MaxOptions = loosestMax(answering, KindChoice, func(c Capabilities) int { return c.MaxOptions })
	out.MaxScoreLevels = loosestMax(answering, KindScore, func(c Capabilities) int { return c.MaxScoreLevels })
	out.MinScoreLevels = loosestMin(answering, KindScore, func(c Capabilities) int { return c.MinScoreLevels })

	semanticsFrom := answering
	if len(answering) == 0 {
		semanticsFrom = caps
	}
	out.ConfidenceSemantics = semanticsFrom[0].ConfidenceSemantics
	for _, c := range semanticsFrom[1:] {
		if c.ConfidenceSemantics != out.ConfidenceSemantics {
			out.ConfidenceSemantics = ConfidenceUnknown
			break
		}
	}
	out.SingleRoundTrip = len(answering) > 0
	for _, c := range answering {
		out.SingleRoundTrip = out.SingleRoundTrip && c.SingleRoundTrip
		if c.MaxQuestionsPerCall > 0 && (out.MaxQuestionsPerCall == 0 || c.MaxQuestionsPerCall < out.MaxQuestionsPerCall) {
			out.MaxQuestionsPerCall = c.MaxQuestionsPerCall
		}
	}
	return out
}

// loosestMax returns the largest limit(c) among caps answering kind, or 0
// (no limit) when any of them declares 0 or none answers kind.
func loosestMax(caps []Capabilities, kind Kind, limit func(Capabilities) int) int {
	best := 0
	for _, c := range caps {
		if !slices.Contains(c.Kinds, kind) {
			continue
		}
		v := limit(c)
		if v == 0 {
			return 0
		}
		best = max(best, v)
	}
	return best
}

// loosestMin returns the smallest limit(c) among caps answering kind, or 0
// (no limit) when any of them declares 0 or none answers kind.
func loosestMin(caps []Capabilities, kind Kind, limit func(Capabilities) int) int {
	best := 0
	for _, c := range caps {
		if !slices.Contains(c.Kinds, kind) {
			continue
		}
		v := limit(c)
		if v == 0 {
			return 0
		}
		if best == 0 || v < best {
			best = v
		}
	}
	return best
}
