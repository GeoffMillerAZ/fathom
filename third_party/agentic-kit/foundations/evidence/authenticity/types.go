// Package authenticity provides typed, provenanced authenticity and integrity
// assessment for evidence records.
//
// Every signal carries the detector's full provenance (Author from
// aiprovenance) so assessments are themselves reproducible and auditable.
// Assessments aggregate into an overall Verdict that can cap the evidence
// tier of any span derived from a suspect record — bridging to
// evidence/evidence-grade.
//
// CUE contracts live in schema.cue; this file and assess.go are the Go
// counterparts and must stay in sync. CUE is authoritative for shape; Go is
// authoritative for behaviour.
//
// # Asserted vs verified: per-method ceilings
//
// Methods differ in epistemic strength. MethodDKIM is cryptographic
// verification: a passing signature is re-checked from the frozen message
// bytes against the sender's DNS-published key, independently reproducible by
// anyone holding the bytes. MethodAuthResultsHeader is a relayed assertion: an
// Authentication-Results header (RFC 7601) records what some receiving mail
// server *claims* it verified — nothing is re-verified from the frozen bytes.
// These are protocol-level facts of the RFCs involved, not an organisational
// catalog, so the ceilings live here as code: Method.ScoreCeiling (0.95 for
// dkim, 0.55 for auth_results_header, 1.0 otherwise) and Method.VerdictCeiling
// (auth_results_header capped at likely_authentic) — see ceiling.go.
//
// The ceilings are guidance, applied only via the opt-in ClampToCeiling.
// Assess is deliberately unchanged: it is a shipped pure function whose
// outputs downstream ledgers already pin, so silently clamping inside it
// would change stored-assessment reproducibility. Callers that want the
// ceilings enforced clamp each Signal before assessment; ClampToCeiling does
// not retro-fix assessments already stored.
//
// Concurrency contract: all exported types (Signal, Assessment) are plain
// value types with no mutable state; values are safe to read concurrently.
// Assess is a pure function and may be called from multiple goroutines
// simultaneously.
package authenticity

import (
	"fmt"
	"time"

	aiprovenance "github.com/geoffmilleraz/fathom/third_party/agentic-kit/foundations/provenance/ai-provenance"
)

// Method identifies the detection or verification technique used to produce
// a Signal.
//
// CUE mirror: #Method in schema.cue.
type Method string

const (
	// MethodHashMatch verifies the record hash against a known-good reference.
	MethodHashMatch Method = "hash_match"

	// MethodC2PA checks Coalition for Content Provenance and Authenticity
	// content credentials embedded in the record.
	MethodC2PA Method = "c2pa"

	// MethodEXIFForensics analyses EXIF metadata for signs of tampering or
	// inconsistency (e.g. mismatched camera model, edited timestamps).
	MethodEXIFForensics Method = "exif_forensics"

	// MethodAITextDetector runs an AI-text-detection model to assess whether
	// the record's textual content was machine-generated.
	MethodAITextDetector Method = "ai_text_detector"

	// MethodDeepfakeDetector runs a deepfake-detection model against image or
	// video content in the record.
	MethodDeepfakeDetector Method = "deepfake_detector"

	// MethodMetadataForensics analyses file-level and container metadata for
	// provenance anomalies (e.g. creation tool, embedded author, GPS coords).
	MethodMetadataForensics Method = "metadata_forensics"

	// MethodManualReview records the verdict of a human analyst who manually
	// inspected the record.
	MethodManualReview Method = "manual_review"

	// MethodDKIM verifies one or more DKIM-Signature headers against the raw
	// RFC 822 message bytes using cryptographic DNS-based key lookup. A passing
	// result is independently verifiable from the frozen bytes and the sender's
	// published DNS key; it can raise a message's documentary confidence.
	// DNS lookup is metadata-only (selector._domainkey.domain TXT records) and
	// does not expose message content to external services.
	MethodDKIM Method = "dkim"

	// MethodAuthResultsHeader records receiver-asserted verdicts from
	// Authentication-Results headers (RFC 7601) for SPF, DKIM, DMARC, and ARC.
	// These verdicts are asserted by the receiving mail server, not independently
	// re-verified from frozen bytes, so callers should grade signals produced by
	// this method lower than independently verified MethodDKIM signals.
	MethodAuthResultsHeader Method = "auth_results_header"
)

// validMethods is the canonical set for validation.
var validMethods = map[Method]struct{}{
	MethodHashMatch:         {},
	MethodC2PA:              {},
	MethodEXIFForensics:     {},
	MethodAITextDetector:    {},
	MethodDeepfakeDetector:  {},
	MethodMetadataForensics: {},
	MethodManualReview:      {},
	MethodDKIM:              {},
	MethodAuthResultsHeader: {},
}

// Validate returns an error if m is not one of the nine recognised methods.
func (m Method) Validate() error {
	if _, ok := validMethods[m]; !ok {
		return fmt.Errorf("authenticity: unknown Method %q; must be one of hash_match|c2pa|exif_forensics|ai_text_detector|deepfake_detector|metadata_forensics|manual_review|dkim|auth_results_header", string(m))
	}
	return nil
}

// Verdict is the outcome of a single detection or verification pass.
//
// CUE mirror: #Verdict in schema.cue.
//
// Ordering (most-authentic → most-adversarial):
//
//	authentic(1) < likely_authentic(2) < inconclusive(3) < likely_synthetic(4) < synthetic(5) = tampered(5)
//
// "Worse" means more adversarial/more suspect. synthetic and tampered share
// the highest adversity rank because both represent strong evidence that a
// record should not be trusted.
type Verdict string

const (
	// VerdictAuthentic: strong evidence the record is genuine and unmodified.
	VerdictAuthentic Verdict = "authentic"

	// VerdictLikelyAuthentic: moderate evidence of authenticity; minor
	// anomalies present but no strong indication of manipulation.
	VerdictLikelyAuthentic Verdict = "likely_authentic"

	// VerdictInconclusive: signals are contradictory or insufficient to
	// determine authenticity either way.
	VerdictInconclusive Verdict = "inconclusive"

	// VerdictLikelySynthetic: moderate evidence the content was generated by
	// an AI model or synthesised from other sources.
	VerdictLikelySynthetic Verdict = "likely_synthetic"

	// VerdictSynthetic: strong evidence the content is AI-generated or
	// otherwise fabricated.
	VerdictSynthetic Verdict = "synthetic"

	// VerdictTampered: strong evidence the record has been modified after its
	// claimed creation time (e.g. hash mismatch, EXIF inconsistency).
	VerdictTampered Verdict = "tampered"
)

// verdictAdversityRank maps each Verdict to a numeric adversity score.
// Higher = more adversarial. synthetic and tampered share rank 5 because
// both are equally disqualifying for legal evidence purposes.
var verdictAdversityRank = map[Verdict]int{
	VerdictAuthentic:       1,
	VerdictLikelyAuthentic: 2,
	VerdictInconclusive:    3,
	VerdictLikelySynthetic: 4,
	VerdictSynthetic:       5,
	VerdictTampered:        5,
}

// AdversityRank returns the numeric adversity of the verdict (5=most
// adversarial, 1=most authentic, 0=unknown).
func (v Verdict) AdversityRank() int {
	if r, ok := verdictAdversityRank[v]; ok {
		return r
	}
	return 0
}

// IsWorseOrEqual reports whether v is at least as adversarial as o.
// An unknown verdict is never worse than anything.
func (v Verdict) IsWorseOrEqual(o Verdict) bool {
	return v.AdversityRank() >= o.AdversityRank()
}

// Validate returns an error if v is not one of the six recognised verdicts.
func (v Verdict) Validate() error {
	if _, ok := verdictAdversityRank[v]; !ok {
		return fmt.Errorf("authenticity: unknown Verdict %q; must be one of authentic|likely_authentic|inconclusive|likely_synthetic|synthetic|tampered", string(v))
	}
	return nil
}

// Signal is a single authenticity/integrity finding produced by one detector
// or reviewer for one record.
//
// CUE mirror: #Signal in schema.cue.
//
// Author carries the full provenance of the detector (which AI model/version/
// prompt, or which human analyst). It is the "who produced this signal" record.
// ValidateStrict enforces Author is set; Validate does not, to support
// progressive construction flows. A Signal stored in an assessment ledger
// should always pass ValidateStrict.
type Signal struct {
	// ID is a non-empty identifier unique within the assessment context.
	ID string

	// RecordID identifies the evidence record being assessed.
	RecordID string

	// Method is the detection/verification technique used.
	Method Method

	// Verdict is the outcome of this signal's assessment.
	Verdict Verdict

	// Score is the detector's confidence in its own verdict, in [0, 1].
	// A Score of 0 means the detector is entirely uncertain; 1 means maximally
	// certain. Score is NOT the probability of authenticity — it is the
	// detector's self-reported confidence in whatever Verdict it returned.
	Score float64

	// Detail is a human-readable explanation of the signal. May be empty.
	Detail string

	// Author records who or what produced this signal. For AI detectors this
	// is the full model+prompt+input reproduction key. For human reviewers it
	// is the reviewer's identity.
	// ValidateStrict requires Author to pass aiprovenance.Author.Validate().
	// Validate (non-strict) allows a zero-value Author for construction flows.
	Author aiprovenance.Author

	// At is the time this signal was produced. Must be non-zero.
	At time.Time
}

// Validate returns an error if any required field is invalid.
//
// Rules enforced by Validate:
//   - ID must be non-empty.
//   - RecordID must be non-empty.
//   - Method must be one of the seven recognised values.
//   - Verdict must be one of the six recognised values.
//   - Score must be in [0, 1].
//   - At must be non-zero.
//
// Author is NOT required by Validate. Use ValidateStrict to require it.
func (s Signal) Validate() error {
	if s.ID == "" {
		return fmt.Errorf("authenticity: Signal.ID must not be empty")
	}
	if s.RecordID == "" {
		return fmt.Errorf("authenticity: Signal.RecordID must not be empty")
	}
	if err := s.Method.Validate(); err != nil {
		return err
	}
	if err := s.Verdict.Validate(); err != nil {
		return err
	}
	if s.Score < 0 || s.Score > 1 {
		return fmt.Errorf("authenticity: Signal %q Score %g is outside [0, 1]", s.ID, s.Score)
	}
	if s.At.IsZero() {
		return fmt.Errorf("authenticity: Signal %q At must not be zero", s.ID)
	}
	return nil
}

// ValidateStrict returns an error if any field is invalid, AND additionally
// requires Author to pass aiprovenance.Author.Validate().
//
// Every signal stored in a ledger or used to derive an official Assessment
// must pass ValidateStrict so that the assessment is fully auditable: a signal
// with no Author cannot be reproduced or challenged.
func (s Signal) ValidateStrict() error {
	if err := s.Validate(); err != nil {
		return err
	}
	if err := s.Author.Validate(); err != nil {
		return fmt.Errorf("authenticity: Signal %q Author is invalid (ValidateStrict requires a valid Author): %w", s.ID, err)
	}
	return nil
}

// Assessment is the aggregated authenticity verdict for a single record,
// derived from a set of Signals by Assess.
//
// CUE mirror: #Assessment in schema.cue.
type Assessment struct {
	// RecordID is the identifier of the assessed record, copied from the
	// signals (all signals must reference the same RecordID).
	RecordID string

	// Overall is the aggregate verdict after applying the conservative rule.
	Overall Verdict

	// Confidence is the aggregate confidence in the Overall verdict, in [0, 1].
	//
	// When Overall is authentic or likely_authentic, Confidence is the
	// complementary-product of the Scores of all credible (Score > 0)
	// authentic/likely_authentic signals: combined = 1 − ∏(1 − Scoreᵢ).
	// Each additional credible authentic signal reduces the probability that ALL
	// detectors are wrong, yielding diminishing returns toward 1.0.
	// Zero-Score authentic signals do not contribute to this calculation.
	//
	// When an adverse verdict dominates, Confidence is the max Score among the
	// signals that share the dominant adverse verdict. Authentic signals do not
	// inflate Confidence in the adverse case.
	//
	// When Overall is inconclusive (including when all signals have Score == 0),
	// Confidence is the mean Score of inconclusive signals, or 0 if none.
	Confidence float64

	// Signals is a defensive copy of the input signals that produced this
	// Assessment. Callers may inspect them but mutations do not affect the
	// Assessment.
	Signals []Signal

	// Conservative is true whenever the overall verdict was driven toward a
	// more adversarial outcome by the conservative aggregation rule — i.e. at
	// least one credible adverse signal was present.
	Conservative bool
}
