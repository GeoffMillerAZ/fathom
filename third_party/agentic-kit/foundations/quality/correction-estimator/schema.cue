// Package correctionestimator defines the CUE schema for the
// correction-estimator foundation. These constraints mirror the Go types and
// are evaluated with `cue vet ./...`.
package correctionestimator

// #Band enumerates the coarse correction-burden categories.
#Band: "negligible" | "low" | "medium" | "high" | "critical"

// #FindingStats summarises finding counts by severity.
#FindingStats: {
	// Critical is the count of critical-severity findings; >= 0.
	Critical: int & >=0

	// Major is the count of major-severity findings; >= 0.
	Major: int & >=0

	// Minor is the count of minor-severity findings; >= 0.
	Minor: int & >=0

	// Informational is the count of informational findings; >= 0.
	Informational: int & >=0
}

// #Estimate is the output of the correction estimator.
#Estimate: {
	// Score is a composite correction-burden score in [0, 1].
	Score: number & >=0 & <=1

	// Band is the coarse category derived from Score.
	Band: #Band
}
