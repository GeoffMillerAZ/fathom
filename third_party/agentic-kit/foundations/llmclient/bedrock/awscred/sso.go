package awscred

import (
	"context"
	"crypto/sha1" //nolint:gosec // AWS-defined, non-secret cache-key derivation — see ssoCacheKey.
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"time"
)

// ssoRequestTimeout bounds the GetRoleCredentials HTTPS call — a real
// network round trip to the SSO portal, unlike IMDS's local-only timeout.
const ssoRequestTimeout = 10 * time.Second

// maxSSOBodyBytes caps how much of a GetRoleCredentials response is read.
const maxSSOBodyBytes = 1 << 16

// ssoTokenCache is the on-disk JSON shape AWS CLI/SDK tools write to
// ~/.aws/sso/cache/<sha1-hex>.json after `aws sso login`. Only the fields
// this package reads are declared; the real cache file also carries
// clientId/clientSecret/refreshToken, ignored here.
type ssoTokenCache struct {
	AccessToken string    `json:"accessToken"`
	ExpiresAt   time.Time `json:"expiresAt"`
	Region      string    `json:"region"`
	StartURL    string    `json:"startUrl"`
}

// ssoCacheKey hashes s (a session name or a start URL) into the lowercase
// hex SHA-1 the AWS CLI/SDK use as the cache filename stem. SHA-1 is not a
// security boundary here — it is the AWS-defined, non-secret cache-key
// derivation every SSO-aware tool must reproduce to find the same cache
// entry `aws sso login` wrote; it is never used for anything
// security-sensitive in this package.
func ssoCacheKey(s string) string {
	sum := sha1.Sum([]byte(s)) //nolint:gosec // see doc comment above
	return hex.EncodeToString(sum[:])
}

// ssoTokenCachePath returns the on-disk path for the cached SSO token keyed
// by key (already the sha1-hashed cache key — see ssoCacheKey).
func ssoTokenCachePath(home, key string) string {
	return filepath.Join(home, ".aws", "sso", "cache", key+".json")
}

// readSSOToken reads and parses the cached SSO token at home for key. It
// returns (zero, false, nil) when the cache file is simply absent — that is
// the common case (never logged in, or logged in under a different key),
// not an error; the caller decides what "not found" means (try the next
// lookup, or surface *ExpiredSSOTokenError).
func readSSOToken(home, key string) (ssoTokenCache, bool, error) {
	data, err := os.ReadFile(ssoTokenCachePath(home, key))
	if err != nil {
		if os.IsNotExist(err) {
			return ssoTokenCache{}, false, nil
		}
		return ssoTokenCache{}, false, err
	}
	var tok ssoTokenCache
	if err := json.Unmarshal(data, &tok); err != nil {
		return ssoTokenCache{}, false, fmt.Errorf("awscred: parsing SSO token cache %s: %w", ssoTokenCachePath(home, key), err)
	}
	return tok, true, nil
}

// resolveSSO resolves credentials for a profile that names SSO, in either
// the modern sso_session form or the legacy inline sso_start_url form (see
// ssoProfile).
//
// Token-cache lookup order: when sso.SessionName is set (modern form), the
// SHA-1 of the SESSION NAME is tried first — the AWS CLI/SDK's own
// derivation for sso_session-form profiles — falling back to the SHA-1 of
// sso.StartURL (the legacy derivation) as a second attempt, since a modern
// profile's referenced [sso-session] section also carries the start URL and
// some tool versions cache under that key instead. A legacy profile
// (SessionName empty) has only the start-URL hash to try.
//
// The fallback fires only when the FIRST path has no cache file at all:
// first match wins, and a token FOUND under the session-name hash but
// expired does NOT fall through to the start-URL hash. Both filenames name
// the same logical SSO session, so when the primary cache entry is expired
// the correct outcome is the re-login error below — silently adopting a
// second, at-best-equally-stale copy of the same session's token would at
// worst mask the real fix (`aws sso login`) behind a token that dies
// moments later.
//
// An absent token at every path tried, an expired token at the first found
// path, or a portal 401/403 (the token was valid at cache-read time but the
// portal has since revoked it) all return *ExpiredSSOTokenError naming the
// exact fix. Any other portal failure — a non-200 outside 401/403, or a 200
// whose roleCredentials payload carries no usable credential — is a typed
// *SSOPortalError (never a partial Credentials with a nil error, per
// Resolve's contract).
//
// A live token is exchanged for role credentials via one HTTPS GET
// (GetRoleCredentials) against the SSO portal — Config.SSOPortalBaseURL
// overrides the origin, normally
// "https://portal.sso.<sso.SSORegion>.amazonaws.com".
func resolveSSO(ctx context.Context, cfg Config, profile string, sso ssoProfile, now func() time.Time) (Credentials, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return Credentials{}, fmt.Errorf("awscred: resolving home directory for SSO token cache: %w", err)
	}

	var tok ssoTokenCache
	var found bool
	if sso.SessionName != "" {
		tok, found, err = readSSOToken(home, ssoCacheKey(sso.SessionName))
		if err != nil {
			return Credentials{}, err
		}
	}
	if !found && sso.StartURL != "" {
		tok, found, err = readSSOToken(home, ssoCacheKey(sso.StartURL))
		if err != nil {
			return Credentials{}, err
		}
	}
	if !found || tok.AccessToken == "" || !now().Before(tok.ExpiresAt) {
		return Credentials{}, &ExpiredSSOTokenError{Profile: profile}
	}

	ssoRegion := firstNonEmpty(sso.SSORegion, cfg.Region)
	base := firstNonEmpty(cfg.SSOPortalBaseURL, "https://portal.sso."+ssoRegion+".amazonaws.com")
	reqURL := base + "/federation/credentials?role_name=" + url.QueryEscape(sso.RoleName) + "&account_id=" + url.QueryEscape(sso.AccountID)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return Credentials{}, fmt.Errorf("awscred: building SSO GetRoleCredentials request: %w", err)
	}
	req.Header.Set("x-amz-sso_bearer_token", tok.AccessToken)
	req.Header.Set("Accept", "application/json")

	client := &http.Client{Timeout: ssoRequestTimeout}
	resp, err := client.Do(req)
	if err != nil {
		return Credentials{}, fmt.Errorf("awscred: SSO GetRoleCredentials request: %w", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, maxSSOBodyBytes))

	if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
		return Credentials{}, &ExpiredSSOTokenError{Profile: profile}
	}
	if resp.StatusCode != http.StatusOK {
		return Credentials{}, &SSOPortalError{Profile: profile, Reason: fmt.Sprintf("HTTP %d: %s", resp.StatusCode, snippet(body, 256))}
	}

	var out struct {
		RoleCredentials struct {
			AccessKeyId     string `json:"accessKeyId"`
			SecretAccessKey string `json:"secretAccessKey"`
			SessionToken    string `json:"sessionToken"`
			Expiration      int64  `json:"expiration"` // epoch millis
		} `json:"roleCredentials"`
	}
	if err := json.Unmarshal(body, &out); err != nil {
		return Credentials{}, fmt.Errorf("awscred: parsing SSO GetRoleCredentials response: %w", err)
	}
	if out.RoleCredentials.AccessKeyId == "" || out.RoleCredentials.SecretAccessKey == "" {
		// A 200 with no usable credential must be an error, never a partial
		// Credentials with a nil error — Resolve's contract.
		return Credentials{}, &SSOPortalError{Profile: profile, Reason: "200 response carries no roleCredentials.accessKeyId/secretAccessKey"}
	}

	return Credentials{
		AccessKeyID:     out.RoleCredentials.AccessKeyId,
		SecretAccessKey: out.RoleCredentials.SecretAccessKey,
		SessionToken:    out.RoleCredentials.SessionToken,
		Source:          SourceSSO,
		Expiry:          time.UnixMilli(out.RoleCredentials.Expiration).UTC(),
	}, nil
}

// snippet truncates b to at most n bytes for inclusion in error messages.
func snippet(b []byte, n int) string {
	if len(b) <= n {
		return string(b)
	}
	return string(b[:n]) + "..."
}
