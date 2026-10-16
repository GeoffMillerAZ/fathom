package evidencespan_test

import (
	"testing"
	"time"

	evidencegrade "github.com/geoffmilleraz/fathom/third_party/agentic-kit/foundations/evidence/evidence-grade"
	es "github.com/geoffmilleraz/fathom/third_party/agentic-kit/foundations/evidence/evidence-span"
	aiprovenance "github.com/geoffmilleraz/fathom/third_party/agentic-kit/foundations/provenance/ai-provenance"
)

func validSpan() es.Span {
	return es.Span{
		ID:         "span-001",
		EntityID:   "repo-payments",
		Field:      "language",
		Value:      "Go",
		Confidence: 0.95,
		SourceID:   "src-code-payments",
	}
}

// fullyProvenancedSpan returns a span that passes ValidateAsEvidence.
func fullyProvenancedSpan() es.Span {
	g := evidencegrade.Grade{Tier: evidencegrade.TierDocumentary, Confidence: 0.91}
	return es.Span{
		ID:         "span-full",
		EntityID:   "payments-service",
		Field:      "error-rate",
		Value:      "elevated-2x",
		Confidence: 0.91,
		SourceID:   "src-metrics-feed",
		OccurredAt: time.Date(2024, 6, 1, 14, 2, 0, 0, time.UTC),
		ObservedAt: time.Date(2024, 6, 1, 14, 5, 0, 0, time.UTC),
		RecordID:   "rec-metrics-feed-20240601",
		RecordHash: "a3f1b2c4d5e6a7b8c9d0e1f2a3b4c5d6e7f8a9b0c1d2e3f4a5b6c7d8e9f0a1b2",
		Locator: es.Locator{
			Kind:  "line",
			Value: "metrics.log:4821",
			Start: 4821,
			End:   4821,
		},
		Author: aiprovenance.NewAI(
			"claude-sonnet-4-6",
			"20250514",
			"sha256:promptabc123",
			"sha256:paramsxyz789",
			"sha256:inputdigest000",
			time.Date(2024, 6, 1, 14, 5, 1, 0, time.UTC),
		),
		Grade: &g,
	}
}

// TestValidateHappyPath ensures a well-formed span passes Validate.
func TestValidateHappyPath(t *testing.T) {
	if err := validSpan().Validate(); err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
}

// TestValidateConfidenceBoundaries exercises the closed [0, 1] interval.
func TestValidateConfidenceBoundaries(t *testing.T) {
	cases := []struct {
		name       string
		confidence float64
		wantErr    bool
	}{
		{"zero", 0.0, false},
		{"one", 1.0, false},
		{"mid", 0.5, false},
		{"negative", -0.001, true},
		{"above-one", 1.001, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			s := validSpan()
			s.Confidence = tc.confidence
			err := s.Validate()
			if tc.wantErr && err == nil {
				t.Errorf("confidence %g: expected error, got nil", tc.confidence)
			}
			if !tc.wantErr && err != nil {
				t.Errorf("confidence %g: unexpected error: %v", tc.confidence, err)
			}
		})
	}
}

// TestValidateEmptyFields ensures each required string field is checked.
func TestValidateEmptyFields(t *testing.T) {
	cases := []struct {
		name  string
		mutFn func(*es.Span)
	}{
		{"empty-ID", func(s *es.Span) { s.ID = "" }},
		{"empty-EntityID", func(s *es.Span) { s.EntityID = "" }},
		{"empty-Field", func(s *es.Span) { s.Field = "" }},
		{"empty-Value", func(s *es.Span) { s.Value = "" }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			s := validSpan()
			tc.mutFn(&s)
			if err := s.Validate(); err == nil {
				t.Errorf("%s: expected validation error, got nil", tc.name)
			}
		})
	}
}

// TestValidateSourceIDOptional ensures SourceID is not required.
func TestValidateSourceIDOptional(t *testing.T) {
	s := validSpan()
	s.SourceID = ""
	if err := s.Validate(); err != nil {
		t.Fatalf("empty SourceID should be allowed, got: %v", err)
	}
}

// ---------------------------------------------------------------------------
// Locator tests
// ---------------------------------------------------------------------------

// TestLocatorValidate exercises Locator.Validate.
func TestLocatorValidate(t *testing.T) {
	cases := []struct {
		name    string
		loc     es.Locator
		wantErr bool
	}{
		{"zero/empty-kind-ok", es.Locator{}, false},
		{"valid-page", es.Locator{Kind: "page", Value: "3", Start: 0, End: 0}, false},
		{"valid-line", es.Locator{Kind: "line", Value: "42", Start: 42, End: 42}, false},
		{"valid-char-range", es.Locator{Kind: "char-range", Value: "10-20", Start: 10, End: 20}, false},
		{"valid-byte-range", es.Locator{Kind: "byte-range", Value: "0-99", Start: 0, End: 99}, false},
		{"valid-cell", es.Locator{Kind: "cell", Value: "B2"}, false},
		{"valid-timestamp", es.Locator{Kind: "timestamp", Value: "2024-01-01T00:00:00Z"}, false},
		{"valid-xpath", es.Locator{Kind: "xpath", Value: "//div[@id='x']"}, false},
		{"valid-none", es.Locator{Kind: "none"}, false},
		{"bad-kind", es.Locator{Kind: "paragraph", Value: "intro"}, true},
		{"start-gt-end", es.Locator{Kind: "char-range", Value: "x", Start: 20, End: 10}, true},
		{"start-zero-end-nonzero-ok", es.Locator{Kind: "line", Value: "x", Start: 0, End: 5}, false},
		{"start-nonzero-end-zero-ok", es.Locator{Kind: "line", Value: "x", Start: 5, End: 0}, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.loc.Validate()
			if tc.wantErr && err == nil {
				t.Errorf("expected error, got nil")
			}
			if !tc.wantErr && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
}

// TestLocatorIsZero verifies IsZero.
func TestLocatorIsZero(t *testing.T) {
	zero := es.Locator{}
	if !zero.IsZero() {
		t.Error("zero Locator should be zero")
	}
	nonZero := es.Locator{Kind: "page"}
	if nonZero.IsZero() {
		t.Error("non-empty Kind should not be zero")
	}
}

// ---------------------------------------------------------------------------
// ValidateAsEvidence tests
// ---------------------------------------------------------------------------

// TestValidateAsEvidenceHappyPath passes the fully provenanced span.
func TestValidateAsEvidenceHappyPath(t *testing.T) {
	if err := fullyProvenancedSpan().ValidateAsEvidence(); err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
}

// TestValidateAsEvidenceRejectsMissingAuthor requires Author.Kind.
func TestValidateAsEvidenceRejectsMissingAuthor(t *testing.T) {
	s := fullyProvenancedSpan()
	s.Author = aiprovenance.Author{} // zero value
	if err := s.ValidateAsEvidence(); err == nil {
		t.Fatal("expected error for missing Author, got nil")
	}
}

// TestValidateAsEvidenceRejectsMissingRecordHash requires RecordHash.
func TestValidateAsEvidenceRejectsMissingRecordHash(t *testing.T) {
	s := fullyProvenancedSpan()
	s.RecordHash = ""
	if err := s.ValidateAsEvidence(); err == nil {
		t.Fatal("expected error for missing RecordHash, got nil")
	}
}

// TestValidateAsEvidenceRejectsMissingOccurredAt requires OccurredAt.
func TestValidateAsEvidenceRejectsMissingOccurredAt(t *testing.T) {
	s := fullyProvenancedSpan()
	s.OccurredAt = time.Time{}
	if err := s.ValidateAsEvidence(); err == nil {
		t.Fatal("expected error for zero OccurredAt, got nil")
	}
}

// TestValidateAsEvidenceRejectsMissingObservedAt requires ObservedAt.
func TestValidateAsEvidenceRejectsMissingObservedAt(t *testing.T) {
	s := fullyProvenancedSpan()
	s.ObservedAt = time.Time{}
	if err := s.ValidateAsEvidence(); err == nil {
		t.Fatal("expected error for zero ObservedAt, got nil")
	}
}

// TestValidateAsEvidenceRejectsMissingLocator requires non-zero Locator.
func TestValidateAsEvidenceRejectsMissingLocator(t *testing.T) {
	s := fullyProvenancedSpan()
	s.Locator = es.Locator{}
	if err := s.ValidateAsEvidence(); err == nil {
		t.Fatal("expected error for zero Locator, got nil")
	}
}

// TestValidateAsEvidenceRejectsMissingGrade requires Grade non-nil.
func TestValidateAsEvidenceRejectsMissingGrade(t *testing.T) {
	s := fullyProvenancedSpan()
	s.Grade = nil
	if err := s.ValidateAsEvidence(); err == nil {
		t.Fatal("expected error for nil Grade, got nil")
	}
}

// TestValidateAsEvidenceRejectsMissingRecordID — regression: fully-provenanced
// span with RecordID="" must be rejected by the strict evidence gate.
func TestValidateAsEvidenceRejectsMissingRecordID(t *testing.T) {
	s := fullyProvenancedSpan()
	s.RecordID = ""
	if err := s.ValidateAsEvidence(); err == nil {
		t.Fatal("expected error for empty RecordID, got nil")
	}
}

// ---------------------------------------------------------------------------
// Regression tests for audit bugs (FIX 1–4)
// ---------------------------------------------------------------------------

// TestValidateAsEvidenceRejectsMissingSourceID — FIX 1: SourceID is part of
// provenance invariant #1 and must be non-empty at the evidence gate.
func TestValidateAsEvidenceRejectsMissingSourceID(t *testing.T) {
	s := fullyProvenancedSpan()
	s.SourceID = ""
	if err := s.ValidateAsEvidence(); err == nil {
		t.Fatal("expected error for empty SourceID, got nil")
	}
}

// TestValidateAsEvidenceRejectsNoneLocator — FIX 2: Locator{Kind:"none"} is a
// no-op pointer with no click-traceability; must be rejected by the strict gate.
// The lenient Validate() must still accept it.
func TestValidateAsEvidenceRejectsNoneLocator(t *testing.T) {
	s := fullyProvenancedSpan()
	s.Locator = es.Locator{Kind: "none"}
	// Strict gate rejects.
	if err := s.ValidateAsEvidence(); err == nil {
		t.Fatal("expected error for Locator.Kind='none' at evidence gate, got nil")
	}
	// Lenient Validate() passes (back-compat: "none" is a recognised kind).
	if err := s.Validate(); err != nil {
		t.Fatalf("Validate() should accept Locator.Kind='none', got: %v", err)
	}
}

// TestLocatorValidateRejectsNegativeStart — FIX 3: negative Start/End must be
// rejected (CUE schema says int & >=0; Go must agree).
func TestLocatorValidateRejectsNegativeStart(t *testing.T) {
	loc := es.Locator{Kind: "char-range", Value: "x", Start: -1, End: 5}
	if err := loc.Validate(); err == nil {
		t.Fatal("expected error for negative Start, got nil")
	}
}

// TestLocatorValidateRejectsNegativeEnd — FIX 3 (End branch).
func TestLocatorValidateRejectsNegativeEnd(t *testing.T) {
	loc := es.Locator{Kind: "char-range", Value: "x", Start: 0, End: -5}
	if err := loc.Validate(); err == nil {
		t.Fatal("expected error for negative End, got nil")
	}
}

// ---------------------------------------------------------------------------
// Audit findings — new regression tests
// ---------------------------------------------------------------------------

// TestValidateAsEvidenceRejectsObservedBeforeOccurred — Finding 2: ObservedAt
// must be >= OccurredAt. A span where the record was supposedly observed before
// the event it describes occurred is physically impossible and a hallmark of
// tampering or data corruption (Throughline invariant: truthfulness).
//
// Exception documented in ValidateAsEvidence: equal timestamps are permitted
// (simultaneous observation; e.g. live system telemetry).
func TestValidateAsEvidenceRejectsObservedBeforeOccurred(t *testing.T) {
	s := fullyProvenancedSpan()
	// Set ObservedAt one second BEFORE OccurredAt — must be rejected.
	s.OccurredAt = time.Date(2024, 6, 1, 14, 5, 0, 0, time.UTC)
	s.ObservedAt = time.Date(2024, 6, 1, 14, 4, 59, 0, time.UTC)
	if err := s.ValidateAsEvidence(); err == nil {
		t.Fatal("expected error when ObservedAt < OccurredAt, got nil")
	}
}

// TestValidateAsEvidenceAcceptsObservedEqualOccurred — equal timestamps are
// permitted (simultaneous observation; see ValidateAsEvidence documentation).
func TestValidateAsEvidenceAcceptsObservedEqualOccurred(t *testing.T) {
	s := fullyProvenancedSpan()
	ts := time.Date(2024, 6, 1, 14, 5, 0, 0, time.UTC)
	s.OccurredAt = ts
	s.ObservedAt = ts
	if err := s.ValidateAsEvidence(); err != nil {
		t.Fatalf("equal ObservedAt == OccurredAt should be allowed, got: %v", err)
	}
}

// TestValidateAsEvidenceRejectsInvalidRecordHashFormat — architect finding:
// RecordHash must look like a sha256 hex digest: exactly 64 lowercase hex chars.
// The field carries content-integrity for the append-only ledger; a malformed
// hash silently undermines tamper detection.
func TestValidateAsEvidenceRejectsInvalidRecordHashFormat(t *testing.T) {
	cases := []struct {
		name string
		hash string
	}{
		{"too-short", "abc123"},
		{"sha256-prefix-style", "sha256:abc123def456"},
		{"uppercase", "ABCDEF1234567890ABCDEF1234567890ABCDEF1234567890ABCDEF1234567890"},
		{"non-hex", "zzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzz"},
		{"too-long", "a3f1b2c4d5e6a7b8c9d0e1f2a3b4c5d6e7f8a9b0c1d2e3f4a5b6c7d8e9f0a1b2xx"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			s := fullyProvenancedSpan()
			s.RecordHash = tc.hash
			if err := s.ValidateAsEvidence(); err == nil {
				t.Errorf("RecordHash %q: expected format error, got nil", tc.hash)
			}
		})
	}
}

// TestValidateAsEvidenceAcceptsValidRecordHash — a well-formed 64-char lowercase
// hex digest must pass.
func TestValidateAsEvidenceAcceptsValidRecordHash(t *testing.T) {
	s := fullyProvenancedSpan()
	s.RecordHash = "a3f1b2c4d5e6a7b8c9d0e1f2a3b4c5d6e7f8a9b0c1d2e3f4a5b6c7d8e9f0a1b2"
	if err := s.ValidateAsEvidence(); err != nil {
		t.Fatalf("valid sha256 hex RecordHash should pass, got: %v", err)
	}
}

// ---------------------------------------------------------------------------
// EffectiveGrade tests
// ---------------------------------------------------------------------------

// TestEffectiveGradeExplicit returns the set Grade when non-nil.
func TestEffectiveGradeExplicit(t *testing.T) {
	g := evidencegrade.Grade{Tier: evidencegrade.TierDirect, Confidence: 0.99}
	s := validSpan()
	s.Grade = &g
	got := s.EffectiveGrade()
	if got.Tier != evidencegrade.TierDirect || got.Confidence != 0.99 {
		t.Errorf("expected explicit grade, got %+v", got)
	}
}

// TestEffectiveGradeFallbackWeakestTier uses TierInferred + Confidence when Grade is nil.
func TestEffectiveGradeFallbackWeakestTier(t *testing.T) {
	s := validSpan()
	s.Grade = nil
	got := s.EffectiveGrade()
	if got.Tier != evidencegrade.TierInferred {
		t.Errorf("expected TierInferred, got %q", got.Tier)
	}
	if got.Confidence != s.Confidence {
		t.Errorf("expected confidence %g, got %g", s.Confidence, got.Confidence)
	}
}

// ---------------------------------------------------------------------------
// Author embedding tests
// ---------------------------------------------------------------------------

// TestAuthorEmbeddingValidates checks that Validate catches a bad Author when Kind is set.
func TestAuthorEmbeddingValidates(t *testing.T) {
	s := validSpan()
	// Set Kind but leave AI/Human nil — should fail Validate.
	s.Author = aiprovenance.Author{Kind: aiprovenance.KindHuman, Human: nil}
	if err := s.Validate(); err == nil {
		t.Fatal("expected Author validation error, got nil")
	}
}

// TestAuthorEmbeddingValidHuman checks that a valid human Author passes Validate.
func TestAuthorEmbeddingValidHuman(t *testing.T) {
	s := validSpan()
	s.Author = aiprovenance.NewHuman("alice@example.com")
	if err := s.Validate(); err != nil {
		t.Fatalf("valid human Author should pass Validate: %v", err)
	}
}

// TestAuthorEmbeddingValidAI checks that a valid AI Author passes Validate.
func TestAuthorEmbeddingValidAI(t *testing.T) {
	s := validSpan()
	s.Author = aiprovenance.NewAI(
		"claude-sonnet-4-6", "20250514",
		"sha256:ph", "sha256:pa", "sha256:id",
		time.Now(),
	)
	if err := s.Validate(); err != nil {
		t.Fatalf("valid AI Author should pass Validate: %v", err)
	}
}
