// Package tokenlifecycle classifies OAuth-style credential health and decides
// the next lifecycle action (use / refresh / reauth) as pure, clock-injected
// functions. It extracts the decision half of a consumer's hand-rolled token
// management; the I/O half (PKCE flows, token-endpoint HTTP, revocation POSTs,
// file-backed storage) deliberately stays with the consumer.
//
// AccessToken and RefreshToken are opaque strings with no secret semantics
// attached: this package never logs, hashes, redacts, or interprets them.
// Callers remain fully responsible for handling them as secrets.
//
// CUE contracts live in schema.cue; the Go types in this file mirror them and
// any drift is a bug.
//
// Concurrency contract: the package exposes only pure functions and plain
// value types; everything is safe for concurrent use without synchronisation.
//
// Non-guarantees: no storage, no HTTP, no retry policy. Classification and
// action decisions are only as good as the credential facts and the clock the
// caller supplies — a stale credential snapshot or a skewed clock yields a
// stale answer, and server-side revocation is undetectable from here.
package tokenlifecycle

import "time"

// State classifies the health of a credential at a point in time.
//
// CUE mirror: #State in schema.cue.
type State string

const (
	// StateValid — the access token is unexpired at the observation instant.
	StateValid State = "valid"

	// StateRefreshable — the access token is expired (or its expiry unknown)
	// but a refresh token is held; a refresh can restore access.
	StateRefreshable State = "refreshable"

	// StateReauthNeeded — no usable access token and no refresh token; the
	// user must re-authenticate.
	StateReauthNeeded State = "reauth_needed"

	// StateNotFound — no credential facts at all (the zero Credential; e.g.
	// nothing was stored for this account).
	StateNotFound State = "not_found"
)

// Action is the next lifecycle step a caller should take with a credential.
//
// CUE mirror: #Action in schema.cue.
type Action string

const (
	// ActionUse — the access token is safely usable (its remaining lifetime
	// exceeds the refresh skew).
	ActionUse Action = "use"

	// ActionRefresh — exchange the refresh token for a new access token.
	ActionRefresh Action = "refresh"

	// ActionReauth — no refresh path exists; run the full authentication flow.
	ActionReauth Action = "reauth"
)

// Credential is a point-in-time snapshot of a stored credential's facts.
// AccessToken and RefreshToken are opaque strings — no secret semantics are
// attached here; callers must treat them as secrets.
//
// CUE mirror: #Credential in schema.cue.
type Credential struct {
	// AccessToken is the opaque short-lived bearer token. May be empty.
	AccessToken string

	// RefreshToken is the opaque long-lived token used to obtain new access
	// tokens. Empty means no refresh path exists.
	RefreshToken string

	// ExpiresAt is the access-token expiry. The zero value means unknown,
	// which classification treats as already expired.
	ExpiresAt time.Time

	// Scopes are the granted scope identifiers, if any.
	Scopes []string
}

// IsZero reports whether the credential carries no facts at all: no tokens,
// no expiry, no scopes. Classify maps the zero credential to StateNotFound,
// mirroring a store that has nothing on file for the account.
func (c Credential) IsZero() bool {
	return c.AccessToken == "" &&
		c.RefreshToken == "" &&
		c.ExpiresAt.IsZero() &&
		len(c.Scopes) == 0
}

// DefaultRefreshSkew is the default early-refresh window: an access token
// within this duration of its expiry is refreshed rather than used. 60s
// preserves the donor's hard-coded refresh margin.
const DefaultRefreshSkew = 60 * time.Second

// Policy tunes lifecycle decisions.
//
// CUE mirror: #Policy in schema.cue.
type Policy struct {
	// RefreshSkew is the early-refresh window applied by NextAction.
	//   0 (the zero value) — selects DefaultRefreshSkew (60s).
	//   negative           — disables the early-refresh window entirely
	//                        (refresh only at/after the expiry instant).
	RefreshSkew time.Duration
}

// DefaultPolicy returns a Policy with the documented 60s refresh skew.
func DefaultPolicy() Policy {
	return Policy{RefreshSkew: DefaultRefreshSkew}
}

// effectiveSkew resolves the zero-means-default and negative-means-disabled
// rules documented on Policy.RefreshSkew.
func (p Policy) effectiveSkew() time.Duration {
	switch {
	case p.RefreshSkew == 0:
		return DefaultRefreshSkew
	case p.RefreshSkew < 0:
		return 0
	default:
		return p.RefreshSkew
	}
}
