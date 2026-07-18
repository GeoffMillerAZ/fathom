// Package assertion defines the speech-act primitive that separates WHAT is
// claimed (Proposition) from WHO asserted it, how, and when (Assertion). It is
// the atom that makes "who said what when", witness story-changes, and
// impeachment representable in a legal/investigation engine.
//
// CUE contracts live in schema.cue; this file is the Go counterpart for the
// Stance type and must stay in sync with that schema.
package assertion

import "fmt"

// Stance classifies the speech-act relationship between the declarant and the
// proposition. It captures not just affirmation but also retraction and
// uncertainty, which are essential for representing witness story-changes and
// impeachment in legal proceedings.
//
// CUE mirror: #Stance in schema.cue.
type Stance string

const (
	// StanceAffirms means the declarant positively asserts the proposition is true.
	StanceAffirms Stance = "affirms"

	// StanceDenies means the declarant asserts the proposition is false.
	StanceDenies Stance = "denies"

	// StanceQualifies means the declarant partially accepts or conditions the proposition.
	StanceQualifies Stance = "qualifies"

	// StanceWithdraws means the declarant retracts a previous statement without
	// asserting it was false (e.g. "I no longer stand by that"). This is a retraction.
	StanceWithdraws Stance = "withdraws"

	// StanceRecants means the declarant explicitly states a previous statement was
	// wrong. Stronger than withdraws — admits prior falsity. This is a retraction.
	StanceRecants Stance = "recants"

	// StanceDoesNotRecall means the declarant claims inability to remember.
	StanceDoesNotRecall Stance = "does_not_recall"

	// StanceDeclines means the declarant refuses to answer (e.g. invokes privilege).
	StanceDeclines Stance = "declines"
)

// validStances is the canonical set for validation.
var validStances = map[Stance]struct{}{
	StanceAffirms:       {},
	StanceDenies:        {},
	StanceQualifies:     {},
	StanceWithdraws:     {},
	StanceRecants:       {},
	StanceDoesNotRecall: {},
	StanceDeclines:      {},
}

// Validate returns an error if s is not one of the recognised stances.
func (s Stance) Validate() error {
	if _, ok := validStances[s]; !ok {
		return fmt.Errorf("assertion: unknown stance %q; must be one of affirms|denies|qualifies|withdraws|recants|does_not_recall|declines", string(s))
	}
	return nil
}

// IsRetraction reports whether the stance represents a retraction of a prior
// statement. Only StanceWithdraws and StanceRecants are retractions.
func (s Stance) IsRetraction() bool {
	return s == StanceWithdraws || s == StanceRecants
}
