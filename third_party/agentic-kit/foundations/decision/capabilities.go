package decision

import (
	"reflect"
	"slices"
)

// LatencyClass is a Decider's declared typical latency per Decide call, as
// a coarse class rather than a number: a caller orders or gates deciders by
// it (a hook deadline, a cheap-first cascade) without a live probe. It is
// the adapter's documented claim, not a measurement of this caller's
// workload.
type LatencyClass string

// The latency classes, in increasing order after LatencyNone.
// LatencyUnknown is explicit: a Decider that declares nothing, or declares
// no latency, reports it rather than "".
const (
	// LatencyUnknown means the Decider does not know or does not declare its
	// latency (for example LLM, whose wrapped client may be any model).
	LatencyUnknown LatencyClass = "unknown"
	// LatencyNone means Decide makes no provider round trip at all.
	LatencyNone LatencyClass = "none"
	// LatencySubSecond means a typical call answers in under one second.
	LatencySubSecond LatencyClass = "sub-second"
	// LatencySeconds means a typical call takes one second or more.
	LatencySeconds LatencyClass = "seconds"
)

// CostClass is a Decider's declared list-price cost per typical Decide call
// (about 500 input tokens), as a coarse class rather than a price: a caller
// puts the cheaper decider first without knowing any vendor's price sheet.
// It is the adapter's documented list-price claim, not a spend ledger.
type CostClass string

// The cost classes, in increasing order after CostNone. CostUnknown is
// explicit: a Decider that declares nothing, or declares no cost, reports
// it rather than "".
const (
	// CostUnknown means the Decider does not know or does not declare its
	// cost (for example LLM, whose wrapped client may be any model).
	CostUnknown CostClass = "unknown"
	// CostNone means Decide bills nothing: no provider is called.
	CostNone CostClass = "none"
	// CostLow means a typical call lists under US$0.0001 (a hundredth of a
	// cent).
	CostLow CostClass = "low"
	// CostMedium means a typical call lists from US$0.0001 to US$0.01.
	CostMedium CostClass = "medium"
	// CostHigh means a typical call lists above US$0.01.
	CostHigh CostClass = "high"
)

// ConfidenceSemantics declares where a Decider's answer probabilities come
// from, and so what Answer.Confidence (always recomputed by this package's
// Confidence formula) summarizes. No value is a calibration reading: per
// decision 0017, calibration is per (site, question version, provider,
// resolved model) and is measured by the caller, never declared by an
// adapter.
type ConfidenceSemantics string

// The confidence semantics. ConfidenceUnknown is explicit: a Decider that
// declares nothing, or declares no semantics, reports it rather than "".
const (
	// ConfidenceUnknown means the Decider does not declare where its
	// probabilities come from.
	ConfidenceUnknown ConfidenceSemantics = "unknown"
	// ConfidenceNone means the Decider reports no probabilities at all (it
	// answers nothing, like None).
	ConfidenceNone ConfidenceSemantics = "none"
	// ConfidenceCalibratedProbability means the provider documents its
	// probabilities as calibrated. It is the provider's claim, still to be
	// read on the caller's own data before an answer changes behavior.
	ConfidenceCalibratedProbability ConfidenceSemantics = "calibrated-probability"
	// ConfidenceLogprob means the probabilities are derived from the
	// model's token log-probabilities.
	ConfidenceLogprob ConfidenceSemantics = "logprob"
	// ConfidenceNativeProbability means the model outputs the probability
	// distribution natively as its answer, not as generated text, and the
	// provider does not document it as calibrated.
	ConfidenceNativeProbability ConfidenceSemantics = "native-probability"
	// ConfidenceSelfReported means a generative model wrote the numbers as
	// text when asked to emulate the contract: a self-report, calibrated
	// separately from any native provider.
	ConfidenceSelfReported ConfidenceSemantics = "self-reported"
)

// Capabilities is what a Decider declares it can answer and at what cost,
// so a caller, a decorator or a conformance suite can route, split or
// refuse a Request before it reaches a provider. Obtain it with
// CapabilitiesOf, never by type-asserting a Decider directly: CapabilitiesOf
// returns one canonical undeclared value for a Decider that declares
// nothing and a copy a caller may mutate.
//
// A zero limit means the Decider declares no limit of its own; Validate's
// structural rules still apply to every Request.
type Capabilities struct {
	// Declared is true when the Decider declared its capabilities. When it
	// is false every other field is its zero value except the three classes,
	// which are their explicit unknown values.
	Declared bool
	// Kinds lists the question kinds the Decider answers, in the fixed
	// order KindChoice, KindScore, KindPredicate (any other kind after them,
	// sorted), each once; CapabilitiesOf folds the alias "noul" into
	// KindPredicate. Empty means it answers no kind.
	Kinds []Kind
	// MaxOptions is the most Options a KindChoice question may carry; 0
	// means no limit of the Decider's own.
	MaxOptions int
	// MinScoreLevels and MaxScoreLevels bound the number of ordered levels
	// a KindScore question may have (explicit Levels or an integral
	// [Min, Max] range); 0 means no limit on that side.
	MinScoreLevels, MaxScoreLevels int
	// SingleRoundTrip is true when a whole Request is answered in one
	// provider round trip, so batching questions per State saves calls.
	SingleRoundTrip bool
	// MaxQuestionsPerCall is the most Questions one call carries, when the
	// provider documents a count; 0 means unbounded or undocumented.
	MaxQuestionsPerCall int
	// Latency is the declared latency class.
	Latency LatencyClass
	// Cost is the declared cost class.
	Cost CostClass
	// ConfidenceSemantics is where the Decider's probabilities come from.
	ConfidenceSemantics ConfidenceSemantics
}

// CapabilityDeclarer is the optional interface a Decider implements to
// declare its Capabilities. It is deliberately not a method on Decider:
// adding one would break every Decider implemented outside this package.
// A Decider that does not implement it is undeclared (see CapabilitiesOf).
// Implementations must be safe for concurrent use and must not call out.
type CapabilityDeclarer interface {
	// Capabilities returns the Decider's declaration. It performs no work
	// and makes no provider call.
	Capabilities() Capabilities
}

// CapabilitiesOf returns d's declared Capabilities, or the undeclared value
// (Declared false; Latency, Cost and ConfidenceSemantics their explicit
// unknown values; every other field zero) when d is nil, a nil pointer,
// does not implement CapabilityDeclarer, or declares Declared false.
//
// The result is a copy in canonical form: Kinds in the fixed order with
// duplicates removed, and an empty class reported as its unknown value.
// Mutating it never changes what d declares.
func CapabilitiesOf(d Decider) Capabilities {
	if d == nil {
		return undeclaredCapabilities()
	}
	v := reflect.ValueOf(d)
	switch v.Kind() {
	case reflect.Pointer, reflect.Interface, reflect.Func, reflect.Map, reflect.Slice, reflect.Chan:
		if v.IsNil() {
			return undeclaredCapabilities()
		}
	}
	cd, ok := d.(CapabilityDeclarer)
	if !ok {
		return undeclaredCapabilities()
	}
	c := cd.Capabilities()
	if !c.Declared {
		return undeclaredCapabilities()
	}
	c.Kinds = canonicalKinds(c.Kinds)
	if c.Latency == "" {
		c.Latency = LatencyUnknown
	}
	if c.Cost == "" {
		c.Cost = CostUnknown
	}
	if c.ConfidenceSemantics == "" {
		c.ConfidenceSemantics = ConfidenceUnknown
	}
	return c
}

// undeclaredCapabilities is the one value CapabilitiesOf returns for a
// Decider that declares nothing.
func undeclaredCapabilities() Capabilities {
	return Capabilities{
		Latency:             LatencyUnknown,
		Cost:                CostUnknown,
		ConfidenceSemantics: ConfidenceUnknown,
	}
}

// kindOrder is the fixed order Capabilities.Kinds is reported in.
var kindOrder = []Kind{KindChoice, KindScore, KindPredicate}

// canonicalKinds returns a fresh slice holding in's kinds once each, each
// folded to its canonical Kind (the alias "noul" is KindPredicate): the
// known kinds in kindOrder, then any other kind sorted. It returns nil for
// an empty input.
func canonicalKinds(in []Kind) []Kind {
	if len(in) == 0 {
		return nil
	}
	canon := make([]Kind, len(in))
	for i, k := range in {
		canon[i] = canonicalKind(k)
	}
	seen := make(map[Kind]bool, len(canon))
	out := make([]Kind, 0, len(canon))
	for _, k := range kindOrder {
		if !seen[k] && slices.Contains(canon, k) {
			seen[k] = true
			out = append(out, k)
		}
	}
	var rest []Kind
	for _, k := range canon {
		if !seen[k] {
			seen[k] = true
			rest = append(rest, k)
		}
	}
	slices.Sort(rest)
	return slices.Clip(append(out, rest...))
}
