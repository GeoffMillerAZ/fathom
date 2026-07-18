package authenticity

// This file defines per-Method confidence and verdict ceilings, plus the
// opt-in ClampToCeiling helper. Assess deliberately does NOT apply these
// ceilings — see the package doc's asserted-vs-verified section for why
// clamping is caller-opt-in.

// ScoreCeiling returns the maximum Score a Signal produced by this method
// should claim.
//
// Ceilings (RFC-protocol facts, mirrored in schema.cue):
//
//	dkim                → 0.95 (cryptographic verification, but key rotation
//	                            and DNS trust leave residual uncertainty)
//	auth_results_header → 0.55 (receiver-asserted, not re-verified from
//	                            frozen bytes)
//	all other methods   → 1.0  (no protocol-imposed ceiling)
//
// ScoreCeiling is guidance: it is NOT applied by Assess or Signal.Validate.
// Callers that want it enforced apply ClampToCeiling before assessment.
func (m Method) ScoreCeiling() float64 {
	switch m {
	case MethodDKIM:
		return 0.95
	case MethodAuthResultsHeader:
		return 0.55
	default:
		return 1.0
	}
}

// VerdictCeiling returns the most-authentic Verdict a signal produced by this
// method may claim.
//
// MethodAuthResultsHeader is capped at VerdictLikelyAuthentic: a relayed
// assertion can never, on its own, establish full authenticity. All other
// methods are uncapped (ceiling VerdictAuthentic — the most-authentic verdict,
// so no effective cap).
//
// The ceiling bounds only the authentic direction. Adverse verdicts
// (likely_synthetic, synthetic, tampered) are never affected: a method with a
// low ceiling can still report strong adverse findings.
//
// Like ScoreCeiling, this is guidance — not applied by Assess. Callers opt in
// via ClampToCeiling.
func (m Method) VerdictCeiling() Verdict {
	if m == MethodAuthResultsHeader {
		return VerdictLikelyAuthentic
	}
	return VerdictAuthentic
}

// ClampToCeiling returns a copy of s with its Score and Verdict clamped to
// the ceilings of s.Method:
//
//   - Score is reduced to Method.ScoreCeiling() when it exceeds it.
//   - Verdict is demoted to Method.VerdictCeiling() when it is a recognised
//     verdict more authentic (lower adversity rank) than the ceiling. Adverse
//     and inconclusive verdicts pass through unchanged — clamping never lifts
//     a verdict toward authentic — and an unrecognised Verdict (adversity
//     rank 0) is left untouched rather than "promoted" to the ceiling; it will
//     still fail Signal.Validate.
//
// ClampToCeiling is opt-in: Assess does not call it, and it does not retro-fix
// stored assessments. Signals already aggregated into an Assessment (or
// persisted in a ledger) keep whatever Score/Verdict they were recorded with;
// callers that adopt clamping must apply it at signal-production time and, if
// desired, re-assess historical signals themselves.
func ClampToCeiling(s Signal) Signal {
	if ceiling := s.Method.ScoreCeiling(); s.Score > ceiling {
		s.Score = ceiling
	}
	if rank := s.Verdict.AdversityRank(); rank >= 1 {
		if ceiling := s.Method.VerdictCeiling(); rank < ceiling.AdversityRank() {
			s.Verdict = ceiling
		}
	}
	return s
}
