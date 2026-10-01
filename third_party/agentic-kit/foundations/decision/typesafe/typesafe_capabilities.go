package typesafe

import decision "github.com/geoffmilleraz/agentic-kit/foundations/decision"

// The vendor limits TypeSafe declares, as docs/fast-judgment.md section 1
// documents them [vendor]: a choice among up to 255 options, a score on 2 to
// 10 ordered levels.
const (
	// typesafeMaxOptions is the most Options a TypeSafe choice carries.
	typesafeMaxOptions = 255
	// typesafeMinScoreLevels is the fewest levels a TypeSafe score carries.
	typesafeMinScoreLevels = 2
	// typesafeMaxScoreLevels is the most levels a TypeSafe score carries.
	typesafeMaxScoreLevels = 10
)

// Capabilities declares what TypeSafe answers, from the vendor facts
// docs/fast-judgment.md records (dated 2026-09-29, tagged [vendor]):
//
//   - every kind natively: choice among up to 255 options, score on 2 to 10
//     ordered levels, and the yes/no kind;
//   - ONE POST per Request carrying every question keyed by ID; the vendor
//     documents token limits (64k per request, 32k for the state plus the
//     longest question), not a question count, so MaxQuestionsPerCall is 0;
//   - sub-second latency (the vendor's claim and the public demonstrations);
//   - low cost: US$0.042 per million input tokens with free output, about
//     US$0.000021 for a 500-token call;
//   - native probabilities: the model outputs the distribution itself. The
//     vendor does not document them as calibrated, and a confidence is a
//     claim, not a measurement, until a reading on the caller's data says so
//     (docs/fast-judgment.md section 3).
//
// Capabilities makes no call and reads no field, so it is safe on a nil
// *TypeSafe.
func (t *TypeSafe) Capabilities() decision.Capabilities {
	return typesafeCapabilities()
}

// typesafeCapabilities is TypeSafe's one declaration of its limits. Both
// Capabilities (what a caller reads) and typesafeRenderQuestion (what refuses
// a question outside them, via outsideDeclaredLimits) take it from here, so
// the constants above are the single source: the limit a caller sees is the
// limit the adapter enforces.
func typesafeCapabilities() decision.Capabilities {
	return decision.Capabilities{
		Declared:            true,
		Kinds:               []decision.Kind{decision.KindChoice, decision.KindScore, decision.KindPredicate},
		MaxOptions:          typesafeMaxOptions,
		MinScoreLevels:      typesafeMinScoreLevels,
		MaxScoreLevels:      typesafeMaxScoreLevels,
		SingleRoundTrip:     true,
		Latency:             decision.LatencySubSecond,
		Cost:                decision.CostLow,
		ConfidenceSemantics: decision.ConfidenceNativeProbability,
	}
}
