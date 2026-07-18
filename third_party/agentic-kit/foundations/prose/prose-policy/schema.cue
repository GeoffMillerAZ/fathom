// Package prosepolicy defines the CUE-first contract for DLP posture levels
// and the policy evaluation types. Go types in types.go mirror these
// definitions; any drift is a bug.
package prosepolicy

// _nonEmpty is a reusable constraint for non-empty strings.
_nonEmpty: string & !=""

// #Posture enumerates the valid DLP posture levels in increasing restrictiveness.
#Posture: "open" | "internal" | "restricted" | "secret"

// #PostureRule describes a set of terms prohibited at or above a posture threshold.
#PostureRule: {
	// Threshold is the minimum posture level at which this rule activates.
	Threshold: #Posture

	// Terms is the list of literal substrings that must be redacted.
	// Each term must be non-empty.
	Terms: [..._nonEmpty]
}

// #Policy holds the active DLP posture and the rules governing evaluation.
#Policy: {
	// Posture is the active DLP posture level.
	Posture: #Posture

	// Rules is the set of posture rules consulted during evaluation.
	Rules: [...#PostureRule]
}

// #Decision is the result of evaluating a prose segment against a Policy.
#Decision: {
	// Allowed is true when no redactions were triggered.
	Allowed: bool

	// Redactions is the list of terms found in the segment that must be
	// removed or replaced before publication.
	Redactions: [...string]
}
