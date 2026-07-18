// Package datasensitivitytypes defines CUE contracts for the core sensitivity
// classification types shared across the security foundation.
package datasensitivitytypes

// #SensitivityLevel enumerates the four recognised sensitivity levels.
// Values are strictly ordered: public < internal < confidential < restricted.
#SensitivityLevel: "public" | "internal" | "confidential" | "restricted"

// #Classification is the result of assigning a sensitivity level to a piece
// of content. Reason must be non-empty when level is anything above "public".
#Classification: {
	// Level is the assigned sensitivity level.
	Level: #SensitivityLevel

	// Reason is a short human-readable explanation of the classification.
	Reason: string & !=""
}
