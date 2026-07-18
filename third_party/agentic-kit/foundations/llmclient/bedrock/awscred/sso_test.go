package awscred

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// writeSSOTokenCache writes a cached SSO token JSON file for cacheKey (the
// already-hashed sha1 hex) under home/.aws/sso/cache/.
func writeSSOTokenCache(t *testing.T, home, cacheKey string, tok ssoTokenCache) {
	t.Helper()
	data, err := json.Marshal(tok)
	if err != nil {
		t.Fatalf("marshal token cache: %v", err)
	}
	path := filepath.Join(home, ".aws", "sso", "cache", cacheKey+".json")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

func fixedNow() func() time.Time {
	t := time.Date(2026, 7, 16, 12, 0, 0, 0, time.UTC)
	return func() time.Time { return t }
}

func TestSSOCacheKey_MatchesKnownAWSVector(t *testing.T) {
	// Pinned digest: sha1("https://d-abc123.awsapps.com/start") — the worked
	// start-URL example used across AWS SSO tooling docs. The exact lowercase
	// hex is asserted (computed once via `printf ... | shasum -a 1` and
	// pinned) so any drift in the derivation — hash function, casing,
	// encoding — fails loudly instead of only length-checking.
	const want = "40a89917e3175433e361b710a9d43528d7f1890a"
	if got := ssoCacheKey("https://d-abc123.awsapps.com/start"); got != want {
		t.Fatalf("ssoCacheKey = %q, want %q", got, want)
	}
}

func TestResolveSSO_ModernForm_HappyPath(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	sessionName := "my-sso"
	writeSSOTokenCache(t, home, ssoCacheKey(sessionName), ssoTokenCache{
		AccessToken: "cached-access-token",
		ExpiresAt:   time.Date(2026, 7, 16, 18, 0, 0, 0, time.UTC),
		Region:      "us-east-1",
		StartURL:    "https://example.awsapps.com/start",
	})

	var gotRole, gotAccount, gotBearer string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotRole = r.URL.Query().Get("role_name")
		gotAccount = r.URL.Query().Get("account_id")
		gotBearer = r.Header.Get("x-amz-sso_bearer_token")
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"roleCredentials":{"accessKeyId":"AKIDSSO","secretAccessKey":"secretsso","sessionToken":"tokensso","expiration":1784304000000}}`))
	}))
	defer srv.Close()

	cfg := Config{SSOPortalBaseURL: srv.URL, Now: fixedNow()}
	sso := ssoProfile{
		SessionName: sessionName,
		StartURL:    "https://example.awsapps.com/start",
		SSORegion:   "us-east-1",
		AccountID:   "111122223333",
		RoleName:    "MyRole",
	}

	creds, err := resolveSSO(context.Background(), cfg, "sso-user", sso, cfg.clock())
	if err != nil {
		t.Fatalf("resolveSSO: %v", err)
	}
	if creds.AccessKeyID != "AKIDSSO" || creds.SecretAccessKey != "secretsso" || creds.SessionToken != "tokensso" {
		t.Errorf("creds = %+v", creds)
	}
	if creds.Source != SourceSSO {
		t.Errorf("Source = %q, want sso", creds.Source)
	}
	if gotRole != "MyRole" || gotAccount != "111122223333" {
		t.Errorf("role/account query = %q/%q", gotRole, gotAccount)
	}
	if gotBearer != "cached-access-token" {
		t.Errorf("bearer header = %q", gotBearer)
	}
}

func TestResolveSSO_LegacyForm_HashesStartURL(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	startURL := "https://legacy.awsapps.com/start"
	writeSSOTokenCache(t, home, ssoCacheKey(startURL), ssoTokenCache{
		AccessToken: "legacy-token",
		ExpiresAt:   time.Date(2026, 7, 16, 18, 0, 0, 0, time.UTC),
	})

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"roleCredentials":{"accessKeyId":"AKIDLEG","secretAccessKey":"s","sessionToken":"t","expiration":1784304000000}}`))
	}))
	defer srv.Close()

	cfg := Config{SSOPortalBaseURL: srv.URL, Now: fixedNow()}
	sso := ssoProfile{StartURL: startURL, SSORegion: "us-west-2", AccountID: "1", RoleName: "R"}

	creds, err := resolveSSO(context.Background(), cfg, "legacy-user", sso, cfg.clock())
	if err != nil {
		t.Fatalf("resolveSSO: %v", err)
	}
	if creds.AccessKeyID != "AKIDLEG" {
		t.Errorf("AccessKeyID = %q", creds.AccessKeyID)
	}
}

func TestResolveSSO_ModernForm_FallsBackToStartURLHash(t *testing.T) {
	// The token was cached under the start-URL hash (legacy-style caching)
	// even though the profile uses the modern sso_session form — the second
	// lookup documented in resolveSSO's doc comment.
	home := t.TempDir()
	t.Setenv("HOME", home)

	startURL := "https://fallback.awsapps.com/start"
	writeSSOTokenCache(t, home, ssoCacheKey(startURL), ssoTokenCache{
		AccessToken: "fallback-token",
		ExpiresAt:   time.Date(2026, 7, 16, 18, 0, 0, 0, time.UTC),
	})

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"roleCredentials":{"accessKeyId":"AKIDFALLBACK","secretAccessKey":"s","sessionToken":"t","expiration":1784304000000}}`))
	}))
	defer srv.Close()

	cfg := Config{SSOPortalBaseURL: srv.URL, Now: fixedNow()}
	sso := ssoProfile{SessionName: "not-cached-under-this-name", StartURL: startURL, SSORegion: "us-east-1", AccountID: "1", RoleName: "R"}

	creds, err := resolveSSO(context.Background(), cfg, "sso-user", sso, cfg.clock())
	if err != nil {
		t.Fatalf("resolveSSO: %v", err)
	}
	if creds.AccessKeyID != "AKIDFALLBACK" {
		t.Errorf("AccessKeyID = %q, want fallback to succeed", creds.AccessKeyID)
	}
}

func TestResolveSSO_NoCachedToken_ReturnsExpiredSSOTokenError(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	cfg := Config{Now: fixedNow()}
	sso := ssoProfile{SessionName: "never-logged-in", StartURL: "https://x.awsapps.com/start", SSORegion: "us-east-1"}

	_, err := resolveSSO(context.Background(), cfg, "myprofile", sso, cfg.clock())
	var expErr *ExpiredSSOTokenError
	if !errors.As(err, &expErr) {
		t.Fatalf("err = %T %v, want *ExpiredSSOTokenError", err, err)
	}
	if expErr.Profile != "myprofile" {
		t.Errorf("Profile = %q", expErr.Profile)
	}
	if got := err.Error(); !containsAll(got, "aws sso login", "--profile myprofile") {
		t.Errorf("error text %q must name the fix", got)
	}
}

func TestResolveSSO_ExpiredCachedToken_ReturnsExpiredSSOTokenError(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	sessionName := "expired-session"
	writeSSOTokenCache(t, home, ssoCacheKey(sessionName), ssoTokenCache{
		AccessToken: "stale-token",
		ExpiresAt:   time.Date(2026, 7, 16, 6, 0, 0, 0, time.UTC), // before fixedNow's 12:00
	})

	cfg := Config{Now: fixedNow()}
	sso := ssoProfile{SessionName: sessionName, StartURL: "https://x.awsapps.com/start", SSORegion: "us-east-1"}

	_, err := resolveSSO(context.Background(), cfg, "myprofile", sso, cfg.clock())
	var expErr *ExpiredSSOTokenError
	if !errors.As(err, &expErr) {
		t.Fatalf("err = %T %v, want *ExpiredSSOTokenError for an expired cached token", err, err)
	}
}

func TestResolveSSO_EmptyRoleCredentials_TypedError(t *testing.T) {
	// A 200 whose roleCredentials carries no accessKeyId must be a typed
	// error, never a partial Credentials with a nil error — Resolve's
	// contract says "never a partial Credentials."
	home := t.TempDir()
	t.Setenv("HOME", home)

	sessionName := "empty-creds-session"
	writeSSOTokenCache(t, home, ssoCacheKey(sessionName), ssoTokenCache{
		AccessToken: "live-token",
		ExpiresAt:   time.Date(2026, 7, 16, 18, 0, 0, 0, time.UTC),
	})

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"roleCredentials":{}}`))
	}))
	defer srv.Close()

	cfg := Config{SSOPortalBaseURL: srv.URL, Now: fixedNow()}
	sso := ssoProfile{SessionName: sessionName, StartURL: "https://x.awsapps.com/start", SSORegion: "us-east-1", AccountID: "1", RoleName: "R"}

	_, err := resolveSSO(context.Background(), cfg, "myprofile", sso, cfg.clock())
	var spe *SSOPortalError
	if !errors.As(err, &spe) {
		t.Fatalf("err = %T %v, want *SSOPortalError", err, err)
	}
	if spe.Profile != "myprofile" {
		t.Errorf("Profile = %q", spe.Profile)
	}
}

func TestResolveSSO_PortalUnauthorized_ReturnsExpiredSSOTokenError(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	sessionName := "revoked-session"
	writeSSOTokenCache(t, home, ssoCacheKey(sessionName), ssoTokenCache{
		AccessToken: "revoked-token",
		ExpiresAt:   time.Date(2026, 7, 16, 18, 0, 0, 0, time.UTC),
	})

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer srv.Close()

	cfg := Config{SSOPortalBaseURL: srv.URL, Now: fixedNow()}
	sso := ssoProfile{SessionName: sessionName, StartURL: "https://x.awsapps.com/start", SSORegion: "us-east-1"}

	_, err := resolveSSO(context.Background(), cfg, "myprofile", sso, cfg.clock())
	var expErr *ExpiredSSOTokenError
	if !errors.As(err, &expErr) {
		t.Fatalf("err = %T %v, want *ExpiredSSOTokenError on portal 401", err, err)
	}
}

// containsAll reports whether s contains every substring in subs.
func containsAll(s string, subs ...string) bool {
	for _, sub := range subs {
		if !strings.Contains(s, sub) {
			return false
		}
	}
	return true
}
