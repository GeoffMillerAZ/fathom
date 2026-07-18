package tokenlifecycle

import "time"

// Classify reports the health of cred at instant now. Pure; the caller
// supplies the clock reading.
//
// Rules, in order:
//   - the zero credential (cred.IsZero()) → StateNotFound
//   - ExpiresAt non-zero and now strictly before it → StateValid
//   - a refresh token is held → StateRefreshable
//   - otherwise → StateReauthNeeded
//
// A zero ExpiresAt means the expiry is unknown and is treated as already
// expired — an access token with unknown expiry is never StateValid. At
// exactly now == ExpiresAt the token is expired (validity requires now to be
// strictly before the expiry).
//
// No refresh skew is applied here: skew belongs to the refresh decision, see
// NextAction.
func Classify(cred Credential, now time.Time) State {
	if cred.IsZero() {
		return StateNotFound
	}
	if !cred.ExpiresAt.IsZero() && now.Before(cred.ExpiresAt) {
		return StateValid
	}
	if cred.RefreshToken != "" {
		return StateRefreshable
	}
	return StateReauthNeeded
}

// NextAction decides what the caller should do with cred at instant now under
// policy. Pure; the caller supplies the clock reading.
//
// Rules, in order:
//   - ExpiresAt non-zero and now strictly before ExpiresAt−skew → ActionUse
//   - a refresh token is held → ActionRefresh
//   - otherwise → ActionReauth (including the zero credential — with nothing
//     stored, full authentication is the only path)
//
// A zero ExpiresAt means unknown/already expired, so the credential is
// refreshed when possible even if the access token might still work. At the
// exact skew boundary (now == ExpiresAt−skew) the decision is already
// ActionRefresh: use requires now to be strictly before the boundary.
//
// The skew comes from policy.RefreshSkew (zero value → DefaultRefreshSkew,
// negative → no early-refresh window; see Policy).
func NextAction(cred Credential, now time.Time, policy Policy) Action {
	skew := policy.effectiveSkew()
	if !cred.ExpiresAt.IsZero() && now.Before(cred.ExpiresAt.Add(-skew)) {
		return ActionUse
	}
	if cred.RefreshToken != "" {
		return ActionRefresh
	}
	return ActionReauth
}

// MergeRefreshed merges a refresh response into the previously stored
// credential, encoding the preserve-refresh-token rule: token endpoints MAY
// omit the refresh token from a refresh response, in which case the old
// refresh token remains valid and is carried forward. When refreshed carries
// its own refresh token, it wins.
//
// Every other field — access token, expiry, scopes — is taken from refreshed
// as-is (donor parity: a refresh response that omits scopes yields empty
// scopes; nothing else is back-filled from old).
func MergeRefreshed(old, refreshed Credential) Credential {
	merged := refreshed
	if merged.RefreshToken == "" {
		merged.RefreshToken = old.RefreshToken
	}
	return merged
}

// RevocationTarget selects which token the caller should send to a revocation
// endpoint: the refresh token when present (revoking the refresh token
// invalidates the whole grant, including derived access tokens), otherwise
// the access token. ok is false when the credential holds no token at all,
// in which case there is nothing to revoke.
func RevocationTarget(cred Credential) (token string, ok bool) {
	if cred.RefreshToken != "" {
		return cred.RefreshToken, true
	}
	if cred.AccessToken != "" {
		return cred.AccessToken, true
	}
	return "", false
}
