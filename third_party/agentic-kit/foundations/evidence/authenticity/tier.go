package authenticity

import (
	evidencegrade "github.com/geoffmilleraz/fathom/third_party/agentic-kit/foundations/evidence/evidence-grade"
)

// MaxEvidenceTier returns the strongest evidence tier that a span derived
// from a record with the given authenticity verdict may claim.
//
// This is the bridge between authenticity assessment and evidence grading:
// when a record is suspected to be synthetic or tampered, any span extracted
// from it must have its tier capped to reflect the record's reliability.
//
// Mapping (most-authentic → most-adversarial):
//
//	authentic        → direct       (no cap; full tier available)
//	likely_authentic → documentary  (minor uncertainty; contemporaneous-record level)
//	inconclusive     → circumstantial (requires an inference step to trust the record)
//	likely_synthetic → testimonial  (treated like a human statement: possibly true but unreliable)
//	synthetic        → inferred     (AI-generated or fabricated; weakest tier — cannot support claims)
//	tampered         → inferred     (modified after creation; cannot be trusted as direct evidence)
//
// Rationale: synthetic and tampered records share the inferred cap because
// both represent strong evidence that the record's content cannot be
// corroborated independently. A span from a tampered record, even if it
// contains true information, was produced under false pretences and cannot
// serve as anything stronger than AI-inferred evidence until independently
// verified.
//
// For an unknown verdict (not one of the six defined values), MaxEvidenceTier
// returns TierInferred as the safe conservative default.
func MaxEvidenceTier(overall Verdict) evidencegrade.Tier {
	switch overall {
	case VerdictAuthentic:
		return evidencegrade.TierDirect
	case VerdictLikelyAuthentic:
		return evidencegrade.TierDocumentary
	case VerdictInconclusive:
		return evidencegrade.TierCircumstantial
	case VerdictLikelySynthetic:
		return evidencegrade.TierTestimonial
	case VerdictSynthetic, VerdictTampered:
		return evidencegrade.TierInferred
	default:
		// Conservative safe default for any unrecognised verdict.
		return evidencegrade.TierInferred
	}
}
