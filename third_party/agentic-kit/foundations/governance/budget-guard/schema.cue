// Package budgetguard defines the schema for budget-guard configuration.
// These constraints mirror the Go Config type and are evaluated with `cue vet ./...`.
package budgetguard

// #Config holds the multi-dimension quota limits for a budget guard.
#Config: {
	// MaxReqPerMin is the maximum number of requests allowed per rolling minute.
	// Must be at least 1.
	MaxReqPerMin: >=1 & int

	// MaxReqPerDay is the maximum number of requests allowed per rolling day.
	// Must be at least 1.
	MaxReqPerDay: >=1 & int

	// MaxUSDPerDay is the maximum estimated USD spend allowed per rolling day.
	// Must be strictly positive.
	MaxUSDPerDay: >0 & number
}
