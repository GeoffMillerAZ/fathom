// Package datasensitivitytypes defines the core sensitivity classification types
// used across the security foundation. Values are data-driven — callers define
// what constitutes each level via configurable catalogs rather than hard-coded
// org-specific literals.
//
// Concurrency: values are immutable and safe for concurrent use.
package datasensitivitytypes

// SensitivityLevel is a strictly ordered sensitivity classification.
// Values are ordered: Public < Internal < Confidential < Restricted.
type SensitivityLevel string

const (
	// SensitivityPublic represents information safe for unrestricted sharing.
	SensitivityPublic SensitivityLevel = "public"

	// SensitivityInternal represents information intended for internal use only.
	SensitivityInternal SensitivityLevel = "internal"

	// SensitivityConfidential represents information requiring controlled access.
	SensitivityConfidential SensitivityLevel = "confidential"

	// SensitivityRestricted represents the highest level — need-to-know only.
	SensitivityRestricted SensitivityLevel = "restricted"
)

// sensitivityRank maps each level to a numeric rank for ordered comparison.
// Higher numbers indicate higher sensitivity.
var sensitivityRank = map[SensitivityLevel]int{
	SensitivityPublic:       0,
	SensitivityInternal:     1,
	SensitivityConfidential: 2,
	SensitivityRestricted:   3,
}

// LevelGTE returns true if level a is greater than or equal to level b.
func LevelGTE(a, b SensitivityLevel) bool {
	return sensitivityRank[a] >= sensitivityRank[b]
}

// Classification is the result of classifying a piece of content or an artifact.
// It carries both the level determined and a human-readable reason.
type Classification struct {
	// Level is the assigned sensitivity level.
	Level SensitivityLevel

	// Reason is a short human-readable explanation of why this level was assigned.
	// It must never be empty when Level is anything other than Public.
	Reason string
}
