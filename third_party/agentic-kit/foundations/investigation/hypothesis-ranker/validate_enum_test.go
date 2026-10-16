package hypothesisranker_test

// validate_enum_test.go — table-driven tests for enum and AI-provenance
// enforcement added 2026-06-08 (fix: hypothesis-ranker-validate).
//
// Every sub-test was written RED (failing) before the corresponding fix was
// applied in validate.go and types.go.

import (
	"testing"
	"time"

	hr "github.com/geoffmilleraz/fathom/third_party/agentic-kit/foundations/investigation/hypothesis-ranker"
)

// baseValid returns a minimal well-formed Hypothesis that passes all existing
// and new validation rules. Tests mutate fields on top of this.
func baseValid() hr.Hypothesis {
	return hr.Hypothesis{
		ID:      "h-base",
		Support: 0.5,
	}
}

// ---------------------------------------------------------------------------
// Status enum enforcement
// ---------------------------------------------------------------------------

// TestValidateStatusEnum verifies that Validate() rejects Status values that
// are outside the CUE-defined enum and accepts every valid value.
func TestValidateStatusEnum(t *testing.T) {
	validStatuses := []hr.HypothesisStatus{
		hr.StatusOpen,
		hr.StatusSupported,
		hr.StatusFalsified,
		hr.StatusClosed,
		"", // zero value — explicitly allowed (treated as open)
	}
	for _, s := range validStatuses {
		h := baseValid()
		h.Status = s
		if err := h.Validate(); err != nil {
			t.Errorf("Status %q: expected no error, got: %v", s, err)
		}
	}

	invalidStatuses := []hr.HypothesisStatus{
		"unknown",
		"active",
		"OPEN",    // wrong case
		"pending", // not in schema
	}
	for _, s := range invalidStatuses {
		h := baseValid()
		h.Status = s
		if err := h.Validate(); err == nil {
			t.Errorf("Status %q: expected validation error, got nil", s)
		}
	}
}

// ---------------------------------------------------------------------------
// GradeTier enum enforcement
// ---------------------------------------------------------------------------

// TestValidateGradeTierEnum verifies that Validate() rejects GradeTier values
// outside the CUE #GradeTierLabel enum and accepts all valid values (including
// the empty/unset zero value).
func TestValidateGradeTierEnum(t *testing.T) {
	validTiers := []hr.GradeTierLabel{
		hr.GradeTierDirect,
		hr.GradeTierDocumentary,
		hr.GradeTierCircumstantial,
		hr.GradeTierTestimonial,
		hr.GradeTierInferred,
		hr.GradeTierUnset, // "" — explicitly allowed (no tier assigned yet)
	}
	for _, tier := range validTiers {
		h := baseValid()
		h.GradeTier = tier
		if err := h.Validate(); err != nil {
			t.Errorf("GradeTier %q: expected no error, got: %v", tier, err)
		}
	}

	invalidTiers := []hr.GradeTierLabel{
		"expert",
		"strong",
		"DIRECT",      // wrong case
		"speculative", // not in schema
	}
	for _, tier := range invalidTiers {
		h := baseValid()
		h.GradeTier = tier
		if err := h.Validate(); err == nil {
			t.Errorf("GradeTier %q: expected validation error, got nil", tier)
		}
	}
}

// ---------------------------------------------------------------------------
// AuthorKind enum enforcement
// ---------------------------------------------------------------------------

// TestValidateAuthorKindEnum verifies that Validate() rejects Author.Kind
// values outside the CUE #AuthorKind enum.
func TestValidateAuthorKindEnum(t *testing.T) {
	validKinds := []hr.AuthorKind{
		hr.AuthorKindHuman,
		hr.AuthorKindAI,
	}
	for _, k := range validKinds {
		h := baseValid()
		h.Author = &hr.HypothesisAuthor{
			Kind:       k,
			Model:      "claude-sonnet-4",
			PromptHash: "sha256:abc",
		}
		if k == hr.AuthorKindHuman {
			// Human authors do not require Model/PromptHash.
			h.Author.Model = ""
			h.Author.PromptHash = ""
		}
		if err := h.Validate(); err != nil {
			t.Errorf("Author.Kind %q: expected no error, got: %v", k, err)
		}
	}

	invalidKinds := []hr.AuthorKind{
		"robot",
		"system",
		"AI",    // wrong case
		"Human", // wrong case
	}
	for _, k := range invalidKinds {
		h := baseValid()
		h.Author = &hr.HypothesisAuthor{Kind: k, Model: "m", PromptHash: "p"}
		if err := h.Validate(); err == nil {
			t.Errorf("Author.Kind %q: expected validation error, got nil", k)
		}
	}
}

// ---------------------------------------------------------------------------
// AI author completeness (Invariant 3: AI labelled)
// ---------------------------------------------------------------------------

// TestValidateAIAuthorRequiresModel verifies that an AI author without a
// Model field is rejected (unlabelled AI contribution violates Invariant 3).
func TestValidateAIAuthorRequiresModel(t *testing.T) {
	h := baseValid()
	h.Author = &hr.HypothesisAuthor{
		Kind:       hr.AuthorKindAI,
		Model:      "", // missing — must be rejected
		PromptHash: "sha256:abc",
	}
	if err := h.Validate(); err == nil {
		t.Error("AI author without Model: expected validation error, got nil")
	}
}

// TestValidateAIAuthorRequiresPromptHash verifies that an AI author without a
// PromptHash is rejected (audit reproducibility requires a content-addressed
// prompt reference).
func TestValidateAIAuthorRequiresPromptHash(t *testing.T) {
	h := baseValid()
	h.Author = &hr.HypothesisAuthor{
		Kind:       hr.AuthorKindAI,
		Model:      "claude-sonnet-4",
		PromptHash: "", // missing — must be rejected
	}
	if err := h.Validate(); err == nil {
		t.Error("AI author without PromptHash: expected validation error, got nil")
	}
}

// TestValidateAIAuthorValid verifies a fully populated AI author passes.
func TestValidateAIAuthorValid(t *testing.T) {
	h := baseValid()
	h.Author = &hr.HypothesisAuthor{
		Kind:        hr.AuthorKindAI,
		Model:       "claude-sonnet-4",
		PromptHash:  "sha256:abc123",
		InputDigest: "sha256:def456",
	}
	if err := h.Validate(); err != nil {
		t.Errorf("fully populated AI author: expected no error, got: %v", err)
	}
}

// TestValidateHumanAuthorNoModelNoPromptHash verifies that a human author
// with empty Model and PromptHash is accepted (they are optional for humans).
func TestValidateHumanAuthorNoModelNoPromptHash(t *testing.T) {
	h := baseValid()
	h.Author = &hr.HypothesisAuthor{
		Kind:       hr.AuthorKindHuman,
		Model:      "",
		PromptHash: "",
	}
	if err := h.Validate(); err != nil {
		t.Errorf("human author with empty Model/PromptHash: expected no error, got: %v", err)
	}
}

// TestValidateNilAuthorAccepted verifies that a nil Author passes Validate()
// (author is optional; absence means unknown/unrecorded).
func TestValidateNilAuthorAccepted(t *testing.T) {
	h := baseValid()
	h.Author = nil
	if err := h.Validate(); err != nil {
		t.Errorf("nil Author: expected no error, got: %v", err)
	}
}

// ---------------------------------------------------------------------------
// CreatedAt field — additive, must not affect existing behaviour
// ---------------------------------------------------------------------------

// TestCreatedAtZeroValueAccepted verifies that the zero time.Time value for
// CreatedAt is accepted by Validate() (field is optional).
func TestCreatedAtZeroValueAccepted(t *testing.T) {
	h := baseValid()
	// zero value — should not cause a validation error
	if err := h.Validate(); err != nil {
		t.Errorf("zero CreatedAt: expected no error, got: %v", err)
	}
}

// TestCreatedAtNonZeroAccepted verifies that a non-zero CreatedAt is accepted.
func TestCreatedAtNonZeroAccepted(t *testing.T) {
	h := baseValid()
	h.CreatedAt = time.Date(2026, 6, 8, 0, 0, 0, 0, time.UTC)
	if err := h.Validate(); err != nil {
		t.Errorf("non-zero CreatedAt: expected no error, got: %v", err)
	}
}
