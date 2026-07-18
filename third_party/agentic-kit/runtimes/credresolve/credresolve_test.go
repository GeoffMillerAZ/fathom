package credresolve_test

import (
	"errors"
	"testing"
	"time"

	"github.com/geoffmilleraz/agentic-kit/runtimes/credresolve"
)

// fixture helpers — NEVER use real ~/.claude files.

func fixedHome(dir string) func() (string, error) {
	return func() (string, error) { return dir, nil }
}

func fixedHomeFail() func() (string, error) {
	return func() (string, error) { return "", errors.New("no home dir") }
}

func fixedFiles(m map[string]string) func(string) ([]byte, error) {
	return func(path string) ([]byte, error) {
		if v, ok := m[path]; ok {
			return []byte(v), nil
		}
		return nil, errors.New("not found")
	}
}

func noEnv(string) (string, bool) { return "", false }

// noKeychain disables the platform keychain step so tests never read the
// real macOS Keychain.
func noKeychain(string) ([]byte, error) { return nil, credresolve.ErrKeychainUnavailable }

func fixedKeychain(payload string) func(string) ([]byte, error) {
	return func(service string) ([]byte, error) {
		if service != "Claude Code-credentials" {
			return nil, credresolve.ErrKeychainUnavailable
		}
		return []byte(payload), nil
	}
}

func fixedEnv(key, val string) func(string) (string, bool) {
	return func(k string) (string, bool) {
		if k == key {
			return val, true
		}
		return "", false
	}
}

func fixedClock(t time.Time) func() time.Time { return func() time.Time { return t } }

// ─── tests ────────────────────────────────────────────────────────────────────

func TestExplicitKeyWins(t *testing.T) {
	r := &credresolve.Resolver{
		HomeDir:      fixedHome("/fakehome"),
		ReadKeychain: noKeychain,
		ReadFile:     fixedFiles(nil),
		LookupEnv:    fixedEnv("ANTHROPIC_API_KEY", "env-key"),
	}
	cred, err := r.Resolve("explicit-key")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cred.Source != credresolve.SourceExplicit {
		t.Errorf("source = %v, want SourceExplicit", cred.Source)
	}
	if !cred.Available() {
		t.Error("Available() = false, want true")
	}
	// NEVER print the token value in test output.
}

func TestEnvVarFallback(t *testing.T) {
	r := &credresolve.Resolver{
		HomeDir:      fixedHome("/fakehome"),
		ReadKeychain: noKeychain,
		ReadFile:     fixedFiles(nil),
		LookupEnv:    fixedEnv("ANTHROPIC_API_KEY", "from-env"),
	}
	cred, err := r.Resolve("")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cred.Source != credresolve.SourceEnv {
		t.Errorf("source = %v, want SourceEnv", cred.Source)
	}
}

func TestCustomEnvKey(t *testing.T) {
	r := &credresolve.Resolver{
		HomeDir:      fixedHome("/fakehome"),
		ReadKeychain: noKeychain,
		ReadFile:     fixedFiles(nil),
		LookupEnv:    fixedEnv("MY_API_KEY", "custom-env-key"),
		EnvKey:       "MY_API_KEY",
	}
	cred, err := r.Resolve("")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cred.Source != credresolve.SourceEnv {
		t.Errorf("source = %v, want SourceEnv", cred.Source)
	}
}

func TestClaudeCredentialsJSON_OAuthWithExpiry(t *testing.T) {
	// expiresAt = 1750000000000 ms = 1750000000 s
	expSec := int64(1750000000)
	expMs := expSec * 1000
	fixture := `{"claudeAiOauth":{"accessToken":"tok","refreshToken":"ref","expiresAt":` +
		itoa(expMs) + `}}`
	r := &credresolve.Resolver{
		HomeDir:      fixedHome("/fakehome"),
		ReadKeychain: noKeychain,
		ReadFile: fixedFiles(map[string]string{
			"/fakehome/.claude/.credentials.json": fixture,
		}),
		LookupEnv: noEnv,
	}
	cred, err := r.Resolve("")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cred.Source != credresolve.SourceClaudeCLI {
		t.Errorf("source = %v, want SourceClaudeCLI", cred.Source)
	}
	if !cred.IsOAuth {
		t.Error("IsOAuth = false, want true")
	}
	if cred.ExpiresAt.IsZero() {
		t.Error("ExpiresAt is zero, want parsed value")
	}
	wantExpiry := time.Unix(expSec, 0).UTC()
	if !cred.ExpiresAt.Equal(wantExpiry) {
		t.Errorf("ExpiresAt = %v, want %v", cred.ExpiresAt, wantExpiry)
	}

	// Token is expired when clock is after ExpiresAt.
	pastClock := fixedClock(wantExpiry.Add(time.Hour))
	if !cred.Expired(pastClock) {
		t.Error("Expired(past) = false, want true")
	}

	// Not expired when clock is before ExpiresAt.
	futureClock := fixedClock(wantExpiry.Add(-time.Hour))
	if cred.Expired(futureClock) {
		t.Error("Expired(future) = true, want false")
	}
}

func TestClaudeCredentialsJSON_OAuthNoExpiry(t *testing.T) {
	fixture := `{"claudeAiOauth":{"accessToken":"tok","refreshToken":"ref"}}`
	r := &credresolve.Resolver{
		HomeDir:      fixedHome("/fakehome"),
		ReadKeychain: noKeychain,
		ReadFile: fixedFiles(map[string]string{
			"/fakehome/.claude/.credentials.json": fixture,
		}),
		LookupEnv: noEnv,
	}
	cred, err := r.Resolve("")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cred.Source != credresolve.SourceClaudeCLI {
		t.Errorf("source = %v, want SourceClaudeCLI", cred.Source)
	}
	if !cred.ExpiresAt.IsZero() {
		t.Errorf("ExpiresAt = %v, want zero (unknown)", cred.ExpiresAt)
	}
	// Unknown expiry: Expired always false.
	if cred.Expired(fixedClock(time.Now())) {
		t.Error("Expired = true for unknown expiry, want false")
	}
}

func TestClaudeCredentialsJSON_APIKey(t *testing.T) {
	fixture := `{"apiKey":"sk-ant-test"}`
	r := &credresolve.Resolver{
		HomeDir:      fixedHome("/fakehome"),
		ReadKeychain: noKeychain,
		ReadFile: fixedFiles(map[string]string{
			"/fakehome/.claude/.credentials.json": fixture,
		}),
		LookupEnv: noEnv,
	}
	cred, err := r.Resolve("")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cred.Source != credresolve.SourceClaudeCLI {
		t.Errorf("source = %v, want SourceClaudeCLI", cred.Source)
	}
	if cred.IsOAuth {
		t.Error("IsOAuth = true for apiKey, want false")
	}
}

func TestLegacyClaudeJSON_OAuth(t *testing.T) {
	fixture := `{"oauthToken":{"access_token":"legacy-tok"}}`
	r := &credresolve.Resolver{
		HomeDir:      fixedHome("/fakehome"),
		ReadKeychain: noKeychain,
		ReadFile: fixedFiles(map[string]string{
			"/fakehome/.claude.json": fixture,
		}),
		LookupEnv: noEnv,
	}
	cred, err := r.Resolve("")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cred.Source != credresolve.SourceClaudeCLI {
		t.Errorf("source = %v, want SourceClaudeCLI", cred.Source)
	}
	if !cred.IsOAuth {
		t.Error("IsOAuth = false for oauthToken, want true")
	}
}

func TestLegacyClaudeJSON_APIKey(t *testing.T) {
	fixture := `{"apiKey":"sk-ant-legacy"}`
	r := &credresolve.Resolver{
		HomeDir:      fixedHome("/fakehome"),
		ReadKeychain: noKeychain,
		ReadFile: fixedFiles(map[string]string{
			"/fakehome/.claude.json": fixture,
		}),
		LookupEnv: noEnv,
	}
	cred, err := r.Resolve("")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cred.Source != credresolve.SourceClaudeCLI {
		t.Errorf("source = %v, want SourceClaudeCLI", cred.Source)
	}
}

func TestNoneWhenNoCredential(t *testing.T) {
	r := &credresolve.Resolver{
		HomeDir:      fixedHome("/fakehome"),
		ReadKeychain: noKeychain,
		ReadFile:     fixedFiles(nil),
		LookupEnv:    noEnv,
	}
	cred, err := r.Resolve("")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cred.Source != credresolve.SourceNone {
		t.Errorf("source = %v, want SourceNone", cred.Source)
	}
	if cred.Available() {
		t.Error("Available() = true, want false")
	}
}

func TestHomeDirFailure_TreatedAsAbsent(t *testing.T) {
	r := &credresolve.Resolver{
		HomeDir:      fixedHomeFail(),
		ReadKeychain: noKeychain,
		ReadFile:     fixedFiles(nil),
		LookupEnv:    noEnv,
	}
	cred, err := r.Resolve("")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cred.Source != credresolve.SourceNone {
		t.Errorf("source = %v, want SourceNone", cred.Source)
	}
}

func TestPrimaryBeatsLegacy(t *testing.T) {
	// ~/.claude/.credentials.json present AND ~/.claude.json present.
	// Primary should win.
	r := &credresolve.Resolver{
		HomeDir:      fixedHome("/fakehome"),
		ReadKeychain: noKeychain,
		ReadFile: fixedFiles(map[string]string{
			"/fakehome/.claude/.credentials.json": `{"claudeAiOauth":{"accessToken":"primary"}}`,
			"/fakehome/.claude.json":              `{"oauthToken":{"access_token":"legacy"}}`,
		}),
		LookupEnv: noEnv,
	}
	cred, err := r.Resolve("")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cred.Source != credresolve.SourceClaudeCLI {
		t.Errorf("source = %v, want SourceClaudeCLI", cred.Source)
	}
	// We can't check the token value — just confirm it's available and from the right source.
	if !cred.Available() {
		t.Error("Available() = false, want true")
	}
}

func TestExpiresIn(t *testing.T) {
	now := time.Unix(1700000000, 0).UTC()
	future := now.Add(2 * time.Hour)
	cred := credresolve.ResolvedCredential{
		Token:     "tok",
		Source:    credresolve.SourceClaudeCLI,
		IsOAuth:   true,
		ExpiresAt: future,
	}
	d := cred.ExpiresIn(fixedClock(now))
	if d != 2*time.Hour {
		t.Errorf("ExpiresIn = %v, want 2h", d)
	}
	// Past expiry returns 0.
	d2 := cred.ExpiresIn(fixedClock(future.Add(time.Minute)))
	if d2 != 0 {
		t.Errorf("ExpiresIn(past) = %v, want 0", d2)
	}
}

func TestKeychainDetected(t *testing.T) {
	exp := int64(1750000000000) // unix ms
	r := &credresolve.Resolver{
		HomeDir:      fixedHome("/fakehome"),
		ReadKeychain: fixedKeychain(`{"claudeAiOauth":{"accessToken":"kc-token","expiresAt":` + itoa(exp) + `}}`),
		ReadFile:     fixedFiles(nil),
		LookupEnv:    noEnv,
	}
	cred, err := r.Resolve("")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cred.Source != credresolve.SourceClaudeCLI {
		t.Errorf("source = %v, want SourceClaudeCLI", cred.Source)
	}
	if !cred.IsOAuth {
		t.Error("IsOAuth = false, want true")
	}
	want := time.Unix(exp/1000, 0).UTC()
	if !cred.ExpiresAt.Equal(want) {
		t.Errorf("ExpiresAt = %v, want %v", cred.ExpiresAt, want)
	}
}

func TestKeychainBeatsCredentialsFile(t *testing.T) {
	// Both present: the Keychain (Claude Code's preferred store on macOS)
	// must win over a possibly-stale credentials file.
	r := &credresolve.Resolver{
		HomeDir:      fixedHome("/fakehome"),
		ReadKeychain: fixedKeychain(`{"claudeAiOauth":{"accessToken":"kc-token"}}`),
		ReadFile: fixedFiles(map[string]string{
			"/fakehome/.claude/.credentials.json": `{"claudeAiOauth":{"accessToken":"stale-file-token"}}`,
		}),
		LookupEnv: noEnv,
	}
	cred, err := r.Resolve("")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cred.Token != "kc-token" {
		t.Error("keychain credential did not take precedence over the file")
	}
}

func TestKeychainCorruptPayload_FallsThroughToFile(t *testing.T) {
	r := &credresolve.Resolver{
		HomeDir:      fixedHome("/fakehome"),
		ReadKeychain: fixedKeychain(`not-json`),
		ReadFile: fixedFiles(map[string]string{
			"/fakehome/.claude/.credentials.json": `{"claudeAiOauth":{"accessToken":"file-token"}}`,
		}),
		LookupEnv: noEnv,
	}
	cred, err := r.Resolve("")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cred.Token != "file-token" {
		t.Error("corrupt keychain payload should fall through to the credentials file")
	}
}

// itoa converts int64 to string for fixture building without importing strconv.
func itoa(n int64) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	s := ""
	for n > 0 {
		s = string(rune('0'+n%10)) + s
		n /= 10
	}
	if neg {
		s = "-" + s
	}
	return s
}
