// Package authenticity defines the CUE-first contract for authenticity and
// integrity assessment of evidence records. Go types in types.go, assess.go,
// and tier.go mirror these definitions; any drift is a bug.
//
// Every signal carries the detector's full provenance (Author from
// aiprovenance) so assessments are reproducible and auditable. The Assessment
// aggregate can cap the evidence tier of any span derived from a suspect record
// via the MaxEvidenceTier mapping documented in tier.go.
package authenticity

import aiprovenance "github.com/geoffmilleraz/agentic-kit/foundations/provenance/ai-provenance:aiprovenance"

// _nonEmpty is a reusable constraint for non-empty strings.
_nonEmpty: string & !=""

// #Method identifies the detection or verification technique used.
//
// Ordered here for documentation; no numeric rank — use Verdict adversity
// rank for ordering, not Method.
#Method:
	"hash_match" |
	"c2pa" |
	"exif_forensics" |
	"ai_text_detector" |
	"deepfake_detector" |
	"metadata_forensics" |
	"manual_review" |
	// dkim: cryptographic DKIM-Signature verification against DNS-published keys,
	// applied to frozen RFC 822 bytes. DNS lookup is metadata-only
	// (selector._domainkey.domain TXT). Can raise documentary confidence.
	// Ceilings (Go: Method.ScoreCeiling/VerdictCeiling in ceiling.go):
	// ScoreCeiling 0.95, VerdictCeiling authentic (uncapped) — verified, but key
	// rotation and DNS trust leave residual uncertainty.
	"dkim" |
	// auth_results_header: receiver-asserted SPF/DKIM/DMARC/ARC verdicts from
	// Authentication-Results headers (RFC 7601). Not independently re-verified
	// from frozen bytes; grade lower than dkim signals.
	// Ceilings (Go: Method.ScoreCeiling/VerdictCeiling in ceiling.go):
	// ScoreCeiling 0.55, VerdictCeiling likely_authentic — an asserted verdict
	// can never on its own establish full authenticity. All other methods:
	// ScoreCeiling 1.0, VerdictCeiling authentic (uncapped). Ceilings are
	// caller-opt-in guidance (ClampToCeiling); Assess never applies them.
	"auth_results_header"

// #Verdict is the outcome of a single detection or verification pass.
//
// Adversity ordering (most-authentic → most-adversarial):
//   authentic(1) → likely_authentic(2) → inconclusive(3) →
//   likely_synthetic(4) → synthetic(5) = tampered(5)
//
// synthetic and tampered share adversity rank 5: both are equally
// disqualifying for legal evidence purposes.
#Verdict:
	"authentic" |
	"likely_authentic" |
	"inconclusive" |
	"likely_synthetic" |
	"synthetic" |
	"tampered"

// #Signal is a single authenticity/integrity finding produced by one detector
// or reviewer for one record.
//
// Author (from aiprovenance.#Author) carries full detector provenance. It is
// required in ledger-stored signals (enforced by Signal.ValidateStrict in Go).
// The CUE schema marks it as optional with "?" so that in-progress
// construction values are representable; tooling that stores signals should
// additionally assert the human/ai disjunction is resolved.
#Signal: {
	// ID is a non-empty identifier unique within the assessment context.
	ID: _nonEmpty

	// RecordID identifies the evidence record being assessed.
	RecordID: _nonEmpty

	// Method is the detection/verification technique used.
	Method: #Method

	// Verdict is the outcome of this signal's assessment.
	Verdict: #Verdict

	// Score is the detector's confidence in its own verdict, in [0, 1].
	Score: >=0 & <=1

	// Detail is a human-readable explanation. May be empty.
	Detail: string

	// Author records who or what produced this signal. Required for ledger
	// storage (Signal.ValidateStrict). Optional here for construction flows.
	Author?: aiprovenance.#Author

	// At is the RFC 3339 timestamp of signal production. Must be non-empty.
	At: _nonEmpty
}

// #Assessment is the aggregated authenticity verdict for a single record,
// derived from a set of Signals by the Assess function.
//
// Conservative rule summary (full documentation in assess.go):
//   - Any credible adverse verdict (likely_synthetic, synthetic, tampered with
//     Score > 0) drives the Overall toward the worst such verdict, even amid
//     many authentic signals.
//   - Multiple independent authentic signals raise Confidence via the
//     complementary-product rule (1 − ∏(1 − Scoreᵢ)).
//   - Inconclusive-only inputs yield Overall: "inconclusive".
//   - Conservative is true when an adverse signal dominated the result.
#Assessment: {
	// RecordID is copied from the input signals.
	RecordID: _nonEmpty

	// Overall is the aggregate verdict after conservative aggregation.
	Overall: #Verdict

	// Confidence is the aggregate confidence in Overall, in [0, 1].
	Confidence: >=0 & <=1

	// Signals is a snapshot of the input signals that produced this Assessment.
	Signals: [...#Signal]

	// Conservative is true when the overall verdict was driven toward a more
	// adversarial outcome by the conservative aggregation rule.
	Conservative: bool
}
