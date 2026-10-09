package decision

import (
	"fmt"
	"strings"
)

// Validate checks req against the fast-decision contract:
//
//   - State must be non-empty.
//   - Every Question.ID must be non-empty and unique within req.
//   - Every Question.Kind must be a known Kind (KindChoice, KindScore, or
//     KindPredicate). A Kind built without ParseKind that holds the alias
//     "noul" is accepted as KindPredicate; the adapters answer it as one.
//   - A KindChoice question must have 2..255 Options with unique,
//     non-empty names.
//   - A KindScore question must have Max strictly greater than Min, UNLESS
//     Levels is set, in which case Levels must have at least 2 entries and
//     Min/Max must be either both zero (unset) or exactly 0 and
//     len(Levels)-1.
//
// Validate enforces the structure every Decider shares, not any one
// provider's limits. In particular it puts no upper bound on Levels: the
// 2..10 levels of TypeSafe's Score primitive is that vendor's limit, which
// TypeSafe declares in its Capabilities and enforces itself, refusing an
// out-of-limit question into Response.Invalid by ID (DP-13). At least 2
// Levels stays a domain rule: one level is not an ordered score, just as
// the range form needs Max greater than Min.
//
// Images are optional; each one set must have a MediaType beginning with
// "image/" and non-empty Data. Whether a Decider takes images at all is its
// declared Capabilities.Images, not a Validate rule (see
// UnsupportedImages).
//
// Validate does not require req.Questions to be non-empty: an empty batch
// is a degenerate but harmless request.
func Validate(req Request) error {
	if req.State == "" {
		return fmt.Errorf("decision: Request.State must not be empty")
	}
	for i, im := range req.Images {
		if !strings.HasPrefix(im.MediaType, "image/") || len(im.MediaType) == len("image/") {
			return fmt.Errorf("decision: image %d: MediaType %q must be an image media type such as \"image/png\"", i, im.MediaType)
		}
		if len(im.Data) == 0 {
			return fmt.Errorf("decision: image %d: Data must not be empty", i)
		}
	}

	seen := make(map[string]bool, len(req.Questions))
	for i, q := range req.Questions {
		if q.ID == "" {
			return fmt.Errorf("decision: question %d: ID must not be empty", i)
		}
		if seen[q.ID] {
			return fmt.Errorf("decision: duplicate question ID %q", q.ID)
		}
		seen[q.ID] = true

		switch canonicalKind(q.Kind) {
		case KindChoice:
			if err := validateChoiceOptions(q); err != nil {
				return err
			}
		case KindScore:
			if err := validateScoreRange(q); err != nil {
				return err
			}
		case KindPredicate:
			// No additional structural constraints.
		default:
			return fmt.Errorf("decision: question %q: unknown Kind %q", q.ID, q.Kind)
		}
	}
	return nil
}

// validateScoreRange checks q's Min/Max/Levels against the score-kind
// contract: with Levels unset, Max must be strictly greater than Min (the
// original rule); with Levels set, Levels must have at least 2 entries (no
// upper bound: an adapter's own level limit is its Capabilities' business,
// not the core's — see Validate) and Min/Max must be either both zero
// (unset) or exactly 0 and len(Levels)-1 — see Question.Levels.
func validateScoreRange(q Question) error {
	if len(q.Levels) == 0 {
		if !(q.Max > q.Min) {
			return fmt.Errorf("decision: question %q: score Max (%v) must be greater than Min (%v)", q.ID, q.Max, q.Min)
		}
		return nil
	}
	if len(q.Levels) < 2 {
		return fmt.Errorf("decision: question %q: score Levels must have at least 2 entries, got %d", q.ID, len(q.Levels))
	}
	unset := q.Min == 0 && q.Max == 0
	matching := q.Min == 0 && q.Max == float64(len(q.Levels)-1)
	if !unset && !matching {
		return fmt.Errorf("decision: question %q: score Min/Max must be unset (0,0) or (0,%d) matching Levels when Levels is set, got (%v,%v)", q.ID, len(q.Levels)-1, q.Min, q.Max)
	}
	return nil
}

// validateChoiceOptions checks q.Options against the choice-kind contract:
// 2..255 options with unique, non-empty names.
func validateChoiceOptions(q Question) error {
	if len(q.Options) < 2 || len(q.Options) > 255 {
		return fmt.Errorf("decision: question %q: choice must have 2..255 options, got %d", q.ID, len(q.Options))
	}
	names := make(map[string]bool, len(q.Options))
	for _, o := range q.Options {
		if o.Name == "" {
			return fmt.Errorf("decision: question %q: option name must not be empty", q.ID)
		}
		if names[o.Name] {
			return fmt.Errorf("decision: question %q: duplicate option name %q", q.ID, o.Name)
		}
		names[o.Name] = true
	}
	return nil
}
