package hypothesisranker

import (
	"fmt"
	"math"
)

// validStatuses is the exhaustive set of allowed HypothesisStatus values.
// An empty string is also accepted at the Go level (treated as StatusOpen by
// convention); explicit inclusion here avoids allocating a map at call time.
var validStatuses = map[HypothesisStatus]struct{}{
	StatusOpen:      {},
	StatusSupported: {},
	StatusFalsified: {},
	StatusClosed:    {},
	"":              {}, // zero value — treated as open; omit constraint at construction
}

// validGradeTiers is the exhaustive set of allowed GradeTierLabel values.
// An empty string is also accepted (GradeTierUnset — no grade assigned yet).
var validGradeTiers = map[GradeTierLabel]struct{}{
	GradeTierDirect:         {},
	GradeTierDocumentary:    {},
	GradeTierCircumstantial: {},
	GradeTierTestimonial:    {},
	GradeTierInferred:       {},
	GradeTierUnset:          {}, // "" — no grade assigned
}

// validAuthorKinds is the exhaustive set of allowed AuthorKind values.
var validAuthorKinds = map[AuthorKind]struct{}{
	AuthorKindHuman: {},
	AuthorKindAI:    {},
}

// Validate returns an error if the hypothesis violates any schema constraint.
// Rules enforced (mirrors schema.cue):
//
//   - ID must be non-empty.
//   - Support must be in [0, 1]; NaN is explicitly rejected.
//   - Status must be one of the #HypothesisStatus enum values (or empty).
//   - GradeTier must be one of the #GradeTierLabel enum values (or empty).
//   - Author.Kind, when Author is set, must be one of the #AuthorKind values.
//   - When Author.Kind == "ai", both Model and PromptHash must be non-empty
//     (Invariant 3: AI contributions must be labelled; unlabelled AI output
//     is a silent-corruption path for provenance).
//
// Validate does not require SpanRefs to be non-empty: a hypothesis without
// span references is valid Go data but callers should treat it as a draft
// (Invariant 6: untraceable output is flagged, not refused at construction).
func (h Hypothesis) Validate() error {
	if h.ID == "" {
		return fmt.Errorf("hypothesisranker: Hypothesis.ID must not be empty")
	}
	if math.IsNaN(h.Support) {
		return fmt.Errorf("hypothesisranker: Hypothesis %q Support is NaN; must be in [0, 1]", h.ID)
	}
	if h.Support < 0 || h.Support > 1 {
		return fmt.Errorf("hypothesisranker: Hypothesis %q Support %g is outside [0, 1]", h.ID, h.Support)
	}

	// Enum: Status
	if _, ok := validStatuses[h.Status]; !ok {
		return fmt.Errorf("hypothesisranker: Hypothesis %q Status %q is not a valid HypothesisStatus (open|supported|falsified|closed)", h.ID, h.Status)
	}

	// Enum: GradeTier
	if _, ok := validGradeTiers[h.GradeTier]; !ok {
		return fmt.Errorf("hypothesisranker: Hypothesis %q GradeTier %q is not a valid GradeTierLabel (direct|documentary|circumstantial|testimonial|inferred|\"\")", h.ID, h.GradeTier)
	}

	// Author checks
	if h.Author != nil {
		// Enum: Author.Kind
		if _, ok := validAuthorKinds[h.Author.Kind]; !ok {
			return fmt.Errorf("hypothesisranker: Hypothesis %q Author.Kind %q is not a valid AuthorKind (human|ai)", h.ID, h.Author.Kind)
		}

		// Invariant 3: AI contributions must carry full provenance labels.
		if h.Author.Kind == AuthorKindAI {
			if h.Author.Model == "" {
				return fmt.Errorf("hypothesisranker: Hypothesis %q Author.Kind is %q but Author.Model is empty; AI contributions must be labelled (Invariant 3)", h.ID, AuthorKindAI)
			}
			if h.Author.PromptHash == "" {
				return fmt.Errorf("hypothesisranker: Hypothesis %q Author.Kind is %q but Author.PromptHash is empty; AI contributions must be labelled (Invariant 3)", h.ID, AuthorKindAI)
			}
		}
	}

	return nil
}

// RankE is the validated variant of Rank. It calls Validate() on every input
// hypothesis and returns an error if any hypothesis is invalid. On success it
// returns the same ranked slice as Rank().
//
// Use RankE when hypotheses are received from external callers or user input
// and you want schema enforcement at runtime. Use Rank when you are certain
// inputs are valid (e.g. seeded fixtures) and prefer the simpler non-error API.
func RankE(hypotheses []Hypothesis) ([]Hypothesis, error) {
	for i, h := range hypotheses {
		if err := h.Validate(); err != nil {
			return nil, fmt.Errorf("hypothesisranker: RankE input[%d]: %w", i, err)
		}
	}
	return Rank(hypotheses), nil
}
