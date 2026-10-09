package openai

import decision "github.com/geoffmilleraz/agentic-kit/foundations/decision"

// Capabilities declares what OpenAI answers, from OpenAI's Decisions guide
// (read 2026-10-09):
//
//   - every kind natively (predicate, choice, score), mixed in ONE request
//     per Request; the guide documents no limit on questions, choices or
//     levels, so none is declared and the core's structural rules apply;
//   - images: the API takes inline base64 image data URLs beside the text;
//   - low cost: US$0.10 per million input tokens and no output charge,
//     about US$0.00005 for a 500-token call;
//   - native probabilities the guide does not document as calibrated (it
//     says to set thresholds from your own labelled examples), so a
//     confidence is a claim until a reading on the caller's data says
//     otherwise (decision 0017);
//   - no model pinning: gpt-6-luna is the one model and no dated version is
//     documented, so Response.Model is the only record of what answered;
//   - latency unknown: the guide states none.
//
// Capabilities makes no call and reads no field, so it is safe on a nil
// *OpenAI.
func (o *OpenAI) Capabilities() decision.Capabilities {
	return openaiCapabilities()
}

// openaiCapabilities is OpenAI's one declaration. Both Capabilities and
// openaiRenderQuestion (through decision.OutsideDeclaredLimits) read it, so
// the limits a caller sees are the limits the adapter enforces.
func openaiCapabilities() decision.Capabilities {
	return decision.Capabilities{
		Declared:            true,
		Kinds:               []decision.Kind{decision.KindChoice, decision.KindScore, decision.KindPredicate},
		SingleRoundTrip:     true,
		Latency:             decision.LatencyUnknown,
		Cost:                decision.CostLow,
		ConfidenceSemantics: decision.ConfidenceNativeProbability,
		Images:              true,
		ModelPinning:        false,
	}
}
