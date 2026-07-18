// Package tokenlifecycle defines CUE contracts for the token-lifecycle
// foundation. Go types in types.go are the Go counterpart; any drift is a bug.
package tokenlifecycle

// #State classifies the health of a credential at a point in time.
//   valid         — access token unexpired at the observation instant.
//   refreshable   — access token expired (or expiry unknown) but a refresh
//                   token is held; a refresh can restore access.
//   reauth_needed — no usable access token and no refresh token; the user
//                   must re-authenticate.
//   not_found     — no credential facts at all (the zero credential; e.g.
//                   nothing was stored for this account).
#State: "valid" | "refreshable" | "reauth_needed" | "not_found"

// #Action is the next lifecycle step a caller should take with a credential.
//   use     — the access token is safely usable (outside the refresh skew).
//   refresh — exchange the refresh token for a new access token.
//   reauth  — no refresh path exists; run the full authentication flow.
#Action: "use" | "refresh" | "reauth"

// #Credential mirrors Go Credential. AccessToken and RefreshToken are opaque
// strings: the schema attaches no secret semantics or format to them — the
// caller is responsible for treating them as secrets.
#Credential: {
	// AccessToken is the opaque short-lived bearer token. May be empty.
	AccessToken: string

	// RefreshToken is the opaque long-lived token used to obtain new access
	// tokens. Empty means no refresh path exists.
	RefreshToken: string

	// ExpiresAt is the RFC3339 access-token expiry. Absent means unknown,
	// which is treated as already expired (Go counterpart: zero time.Time).
	ExpiresAt?: string

	// Scopes are the granted scope identifiers, if any.
	Scopes?: [...string & !=""]
}

// #Policy mirrors Go Policy. RefreshSkew is expressed in nanoseconds
// (the Go time.Duration unit).
//   0        — selects the documented default of 60s (60000000000 ns).
//   negative — disables the early-refresh window entirely.
#Policy: {
	RefreshSkew: int
}
