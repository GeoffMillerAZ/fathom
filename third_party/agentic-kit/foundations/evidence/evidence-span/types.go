// Package evidencespan defines the atomic evidence unit used across the
// evidence foundation. Every observation extracted from a source is encoded as
// a Span before being handed to higher-level components such as evidence-graph,
// evidence-resolver, or source-linker.
//
// CUE contracts live in schema.cue; this file is the Go counterpart and must
// stay in sync with that schema.
//
// Concurrency contract: Span and Locator are plain value types with no mutable
// state; values are safe to read concurrently without additional
// synchronization.
package evidencespan

import (
	"encoding/hex"
	"fmt"
	"time"

	evidencegrade "github.com/geoffmilleraz/agentic-kit/foundations/evidence/evidence-grade"
	aiprovenance "github.com/geoffmilleraz/agentic-kit/foundations/provenance/ai-provenance"
)

// sha256HexLen is the expected byte-length of a SHA-256 hex digest string.
const sha256HexLen = 64

// locatorKinds is the canonical set of valid Locator.Kind values.
var locatorKinds = map[string]struct{}{
	"page":       {},
	"line":       {},
	"char-range": {},
	"byte-range": {},
	"cell":       {},
	"timestamp":  {},
	"xpath":      {},
	"none":       {},
}

// Locator is a structured pointer into a source document for click-traceability.
// It identifies the exact position within the source record where this span
// was extracted from.
//
// CUE mirror: #Locator in schema.cue.
type Locator struct {
	// Kind discriminates the locator type. Must be one of:
	// "page", "line", "char-range", "byte-range", "cell", "timestamp", "xpath", "none".
	// Empty string means unset (zero value).
	Kind string

	// Value is the human- and machine-readable location within the source
	// (e.g. "42" for page 42, "xpath://div[@id='content']").
	Value string

	// Start is the inclusive start offset within the kind's coordinate space
	// (e.g. character offset, byte offset).
	// 0 means "unset" — the Start<=End ordering check applies only when both
	// Start and End are nonzero.
	// Must be >= 0 (mirrors CUE: int & >=0).
	Start int

	// End is the inclusive end offset.
	// 0 means "unset" — ordering is only enforced when both Start and End are nonzero.
	// Must be >= 0 (mirrors CUE: int & >=0). Must be >= Start when both are nonzero.
	End int
}

// IsZero reports whether the Locator is the zero value (Kind is empty).
func (l Locator) IsZero() bool {
	return l.Kind == ""
}

// Validate returns an error if the Locator is invalid.
// An empty/zero Locator (Kind == "") is always valid.
// When Kind is set it must be one of the recognised values.
// Start and End must both be >= 0 (0 means "unset").
// When both Start and End are nonzero, Start must be <= End.
func (l Locator) Validate() error {
	if l.IsZero() {
		return nil
	}
	if _, ok := locatorKinds[l.Kind]; !ok {
		return fmt.Errorf("evidencespan: Locator.Kind %q is not valid; must be one of page|line|char-range|byte-range|cell|timestamp|xpath|none", l.Kind)
	}
	if l.Start < 0 {
		return fmt.Errorf("evidencespan: Locator.Start %d must be >= 0", l.Start)
	}
	if l.End < 0 {
		return fmt.Errorf("evidencespan: Locator.End %d must be >= 0", l.End)
	}
	if l.Start != 0 && l.End != 0 && l.Start > l.End {
		return fmt.Errorf("evidencespan: Locator.Start %d > End %d", l.Start, l.End)
	}
	return nil
}

// Span is the atomic evidence unit. It records a single claimed value for a
// (EntityID, Field) pair together with provenance and a confidence score.
//
// Backward compatibility: all new fields are optional. Existing code that
// builds Span{ID:..., EntityID:..., Field:..., Value:..., Confidence:...}
// continues to compile and pass Validate() unchanged.
//
// For full provenance enforcement call ValidateAsEvidence().
//
// CUE mirror: #Span in schema.cue.
type Span struct {
	// ID uniquely identifies this span. Must be non-empty.
	ID string

	// EntityID is the identifier of the real-world entity this span describes.
	// Must be non-empty.
	EntityID string

	// Field names the attribute being claimed (e.g. "owner", "language").
	// Must be non-empty.
	Field string

	// Value is the claimed value for EntityID.Field. Must be non-empty.
	Value string

	// Confidence is the certainty of this claim in the closed interval [0, 1].
	// 0 = no confidence, 1 = fully verified.
	// Kept for backward compatibility. Prefer Grade for new code.
	Confidence float64

	// SourceID identifies the originating source registered in source-registry.
	SourceID string

	// OccurredAt is the world-time: when the event or fact described by this
	// span actually happened (as opposed to when we recorded it). Optional for
	// backward compat; required by ValidateAsEvidence.
	OccurredAt time.Time

	// ObservedAt is the record-time: when this span was recorded/ingested.
	// Optional for backward compat; required by ValidateAsEvidence.
	ObservedAt time.Time

	// RecordID is the identifier of the source record this span was extracted
	// from. Pair with RecordHash to detect record changes via record-fingerprint.
	// Optional for backward compat; required by ValidateAsEvidence.
	RecordID string

	// RecordHash is the content hash of the source record from which this span
	// was extracted. Optional for backward compat; required by ValidateAsEvidence.
	// Must be a 64-character lowercase hex SHA-256 digest when set via
	// ValidateAsEvidence (the canonical format produced by hex.EncodeToString of
	// a sha256.Sum256 result).
	RecordHash string

	// Locator is a structured pointer into the source for click-traceability.
	// Optional for backward compat; required by ValidateAsEvidence.
	Locator Locator

	// Author records who or what produced this span (human or AI).
	// Optional for backward compat; required by ValidateAsEvidence.
	Author aiprovenance.Author

	// Grade is the explicit epistemic tier + confidence for this span.
	// Optional (nil means "use legacy Confidence field").
	// Required by ValidateAsEvidence.
	Grade *evidencegrade.Grade
}

// Validate returns an error if any field is invalid.
// Rules (lenient / backward-compatible):
//   - ID, EntityID, Field, Value must be non-empty.
//   - Confidence must be in [0, 1].
//   - If Grade is non-nil, it must pass Grade.Validate().
//   - If Locator is non-zero, it must pass Locator.Validate().
//   - If Author.Kind is non-empty, it must pass Author.Validate().
//
// New optional fields (OccurredAt, ObservedAt, RecordHash) are NOT required
// here — use ValidateAsEvidence for full provenance enforcement.
func (s Span) Validate() error {
	if s.ID == "" {
		return fmt.Errorf("evidencespan: ID must not be empty")
	}
	if s.EntityID == "" {
		return fmt.Errorf("evidencespan: EntityID must not be empty")
	}
	if s.Field == "" {
		return fmt.Errorf("evidencespan: Field must not be empty")
	}
	if s.Value == "" {
		return fmt.Errorf("evidencespan: Value must not be empty")
	}
	if s.Confidence < 0 || s.Confidence > 1 {
		return fmt.Errorf("evidencespan: Confidence %g is outside [0, 1]", s.Confidence)
	}
	if s.Grade != nil {
		if err := s.Grade.Validate(); err != nil {
			return fmt.Errorf("evidencespan: Grade invalid: %w", err)
		}
	}
	if !s.Locator.IsZero() {
		if err := s.Locator.Validate(); err != nil {
			return err
		}
	}
	if s.Author.Kind != "" {
		if err := s.Author.Validate(); err != nil {
			return fmt.Errorf("evidencespan: Author invalid: %w", err)
		}
	}
	return nil
}

// ValidateAsEvidence enforces the full Throughline provenance invariant (#1):
// "No provenance → not storable as evidence."
//
// It calls Validate() first (all existing rules apply), then additionally
// requires:
//   - SourceID non-empty (provenance invariant: source component required).
//   - Author valid with non-empty Kind.
//   - RecordHash non-empty and formatted as a 64-character lowercase hex SHA-256
//     digest (the canonical format produced by [encoding/hex].EncodeToString of a
//     [crypto/sha256].Sum256 result). A malformed hash silently undermines content
//     integrity detection in the append-only ledger.
//   - OccurredAt non-zero.
//   - ObservedAt non-zero and >= OccurredAt. Evidence describes events that have
//     already been observed; a record observed before the event it describes is
//     physically impossible and a hallmark of tampering or data corruption
//     (Throughline truthfulness invariant). Equal timestamps are permitted — a
//     simultaneous capture (e.g. live system telemetry) is a valid edge case.
//   - Locator non-zero, valid, and not Kind=="none" (must be a real pointer).
//   - Grade non-nil and valid (explicit epistemic tier required).
//
// Call this before persisting a span to the append-only ledger.
func (s Span) ValidateAsEvidence() error {
	if err := s.Validate(); err != nil {
		return err
	}
	if s.SourceID == "" {
		return fmt.Errorf("evidencespan: SourceID required for evidence (provenance invariant)")
	}
	if s.Author.Kind == "" {
		return fmt.Errorf("evidencespan: Author.Kind must not be empty (provenance required)")
	}
	if err := s.Author.Validate(); err != nil {
		return fmt.Errorf("evidencespan: Author invalid: %w", err)
	}
	if s.RecordHash == "" {
		return fmt.Errorf("evidencespan: RecordHash must not be empty (provenance required)")
	}
	if err := validateSHA256Hex(s.RecordHash); err != nil {
		return fmt.Errorf("evidencespan: RecordHash format invalid: %w", err)
	}
	if s.RecordID == "" {
		return fmt.Errorf("evidencespan: RecordID required for evidence (provenance invariant)")
	}
	if s.OccurredAt.IsZero() {
		return fmt.Errorf("evidencespan: OccurredAt must not be zero (world-time required)")
	}
	if s.ObservedAt.IsZero() {
		return fmt.Errorf("evidencespan: ObservedAt must not be zero (record-time required)")
	}
	// Temporal ordering: evidence describes things already observed. An observation
	// timestamp that precedes the event timestamp is physically impossible and
	// indicates tampering or data corruption. Equal timestamps are allowed (e.g.
	// live telemetry where capture and event are simultaneous).
	if s.ObservedAt.Before(s.OccurredAt) {
		return fmt.Errorf("evidencespan: ObservedAt (%s) must not precede OccurredAt (%s) — observation cannot predate the event it records",
			s.ObservedAt.Format(time.RFC3339), s.OccurredAt.Format(time.RFC3339))
	}
	if s.Locator.IsZero() {
		return fmt.Errorf("evidencespan: Locator must not be zero (source pointer required)")
	}
	if err := s.Locator.Validate(); err != nil {
		return err
	}
	if s.Locator.Kind == "none" {
		return fmt.Errorf("evidencespan: Locator.Kind 'none' is not a real source pointer (evidence requires a locatable pointer)")
	}
	if s.Grade == nil {
		return fmt.Errorf("evidencespan: Grade must not be nil (explicit epistemic tier required)")
	}
	return nil
}

// validateSHA256Hex returns an error if h is not a valid SHA-256 hex digest:
// exactly 64 lowercase hexadecimal characters. This is the canonical format
// produced by hex.EncodeToString(sha256.Sum256(...)[:]).
func validateSHA256Hex(h string) error {
	if len(h) != sha256HexLen {
		return fmt.Errorf("must be exactly %d hex characters, got %d", sha256HexLen, len(h))
	}
	// hex.DecodeString accepts both uppercase and lowercase; we enforce lowercase
	// for canonical form after verifying the character set.
	if _, err := hex.DecodeString(h); err != nil {
		return fmt.Errorf("must be lowercase hex: %w", err)
	}
	// hex.DecodeString accepts A-F; enforce lowercase explicitly.
	for i := 0; i < len(h); i++ {
		c := h[i]
		if c >= 'A' && c <= 'F' {
			return fmt.Errorf("must be lowercase hex: character %q at position %d is uppercase", c, i)
		}
	}
	return nil
}

// EffectiveGrade returns the explicit Grade when set, or synthesises a Grade
// from the legacy Confidence field using the weakest tier (TierInferred).
//
// Design note: a legacy bare-Confidence span is always treated as the weakest
// tier so it can never over-claim epistemic strength until it is explicitly
// graded. This upholds the weakest-link grading rule (docs/practices.md §2).
func (s Span) EffectiveGrade() evidencegrade.Grade {
	if s.Grade != nil {
		return *s.Grade
	}
	return evidencegrade.Grade{
		Tier:       evidencegrade.TierInferred,
		Confidence: s.Confidence,
	}
}
