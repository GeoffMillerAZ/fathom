// Package identityquota defines the schema for identity-quota configuration.
// These constraints mirror the Go Config type and are evaluated with `cue vet ./...`.
package identityquota

// #Config holds the per-identity rolling-day limits for an identity quota guard.
#Config: {
	// MaxReqPerDay is the maximum requests a single identity may make per
	// rolling day. Must be at least 1.
	MaxReqPerDay: >=1 & int

	// MaxUSDPerDay is the maximum estimated USD a single identity may consume
	// per rolling day. Must be strictly positive.
	MaxUSDPerDay: >0 & number

	// MaxIdentities bounds how many identities are tracked at once — a memory
	// bound, since a public endpoint sees address rotation. Must be at least 1.
	MaxIdentities: >=1 & int

	// IdentityTTL is how long an idle identity's counters are retained, as a
	// Go duration string. Should be at least the rolling day so a quota cannot
	// be reset early by going quiet.
	IdentityTTL: string & !=""
}
