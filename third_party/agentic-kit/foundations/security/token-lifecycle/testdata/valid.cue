// valid.cue — concrete values that satisfy the token-lifecycle schema.
// Validated by `cue vet ./testdata/` from the package directory (the kit's
// cue-vet task does this for every schema.cue package). Negative cases live
// as Go rejection/behaviour tests in tokenlifecycle_test.go.
package tokenlifecycle

import "github.com/geoffmilleraz/agentic-kit/foundations/security/token-lifecycle:tokenlifecycle"

// FullCredentialExample carries every field: both opaque tokens, a concrete
// expiry, and granted scopes.
FullCredentialExample: tokenlifecycle.#Credential & {
	AccessToken:  "opaque-access-token-1"
	RefreshToken: "opaque-refresh-token-1"
	ExpiresAt:    "2026-01-01T00:00:00Z"
	Scopes: ["mail.read", "calendar.read"]
}

// RefreshOnlyCredentialExample has no access token and no known expiry —
// classification treats the missing expiry as already expired, so this
// credential is refreshable.
RefreshOnlyCredentialExample: tokenlifecycle.#Credential & {
	AccessToken:  ""
	RefreshToken: "opaque-refresh-token-2"
}

// ReauthCredentialExample holds an expired access token and no refresh token:
// re-authentication is the only path forward.
ReauthCredentialExample: tokenlifecycle.#Credential & {
	AccessToken:  "opaque-access-token-3"
	RefreshToken: ""
	ExpiresAt:    "2020-01-01T00:00:00Z"
}

// DefaultPolicyExample pins the documented default refresh skew: 60 seconds,
// expressed in nanoseconds (the Go time.Duration unit).
DefaultPolicyExample: tokenlifecycle.#Policy & {
	RefreshSkew: 60000000000
}

// NoSkewPolicyExample disables the early-refresh window (negative skew).
NoSkewPolicyExample: tokenlifecycle.#Policy & {
	RefreshSkew: -1
}

// StateValidExample / ActionRefreshExample pin one member of each enum.
StateValidExample: tokenlifecycle.#State & "valid"

ActionRefreshExample: tokenlifecycle.#Action & "refresh"
