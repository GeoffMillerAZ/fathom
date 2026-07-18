// Package credresolve implements the credential-resolution chain used by
// agentic applications that integrate with the Anthropic API.
//
// # Resolution order
//
// Credentials are resolved in priority order:
//
//  1. Explicit key — a raw API key passed directly by the caller (e.g. from a
//     saved config file).
//  2. Environment variable — the value of the configured env var (default:
//     "ANTHROPIC_API_KEY").
//  3. Claude Code CLI auth — credentials stored by the `claude` CLI tool when
//     the user runs `/login` inside Claude Code:
//     a. macOS Keychain item "Claude Code-credentials" (the platform-native
//     store Claude Code prefers on darwin; payload is the same JSON shape
//     as the credentials file)
//     b. ~/.claude/.credentials.json   (claudeAiOauth.accessToken or apiKey)
//     c. ~/.claude.json                (legacy / alternate location)
//
// The Keychain is checked before the files because on macOS a credentials
// file, when present at all, is usually a stale leftover from an older Claude
// Code version while the Keychain holds the current token.
//
// # Expiry awareness
//
// When the active credential is the Claude Code OAuth token, ResolvedCredential
// carries ExpiresAt (parsed from expiresAt unix-milliseconds in the credentials
// file). Callers can call Expired(clock) to determine whether the token is
// still usable, and surface appropriate guidance to the user.
//
// If the token is expired the resolver still returns it (Source == SourceClaudeCLI)
// so callers can distinguish "found but expired — run /login" from "not found at all".
//
// # Injectability
//
// All I/O dependencies are injectable:
//   - HomeDir func() (string, error): provides the user home directory
//   - ReadFile func(path string) ([]byte, error): reads a file by path
//   - LookupEnv func(key string) (string, bool): looks up an environment variable
//   - Clock func() time.Time: current time (used by Expired)
//
// Pass nil for any of these to use the real OS implementations. Tests should
// inject fakes — never read the real ~/.claude files in tests.
//
// # Claude Code compatibility
//
// Claude Code (the Anthropic CLI tool) writes OAuth tokens obtained via
// `/login` into ~/.claude/.credentials.json as:
//
//	{
//	  "claudeAiOauth": {
//	    "accessToken": "...",
//	    "refreshToken": "...",
//	    "expiresAt": 1750000000000   // unix milliseconds, may be absent
//	  }
//	}
//
// Some versions write a top-level "apiKey" instead. A legacy ~/.claude.json
// may carry {"oauthToken":{"access_token":"..."}} or {"apiKey":"..."}.
//
// SECURITY: this package never logs, prints, or embeds raw token values in
// errors. Callers must observe the same discipline.
package credresolve

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// Source describes where a resolved credential came from.
type Source string

const (
	// SourceExplicit means the credential was supplied directly (e.g. from a
	// saved config file).
	SourceExplicit Source = "explicit"

	// SourceEnv means the credential came from an environment variable.
	SourceEnv Source = "env"

	// SourceClaudeCLI means the credential was read from the Claude Code CLI
	// auth files (~/.claude).
	SourceClaudeCLI Source = "claude-cli"

	// SourceNone means no credential was found.
	SourceNone Source = "none"
)

// ResolvedCredential is the outcome of credential resolution.
//
// SECURITY: the Token field contains a raw credential. Callers must not log,
// print, or persist it beyond passing it to an HTTP client.
type ResolvedCredential struct {
	// Token is the raw API key or OAuth access token.
	// Empty when Source == SourceNone.
	Token string

	// IsOAuth is true when Token is an OAuth bearer token, false when it is
	// a raw API key (x-api-key header).
	IsOAuth bool

	// Source reports which step in the resolution chain produced this credential.
	Source Source

	// ExpiresAt is the expiry time of the credential as reported by the auth
	// file. Zero when the expiry is unknown (API keys, env-var keys, or OAuth
	// files that omit expiresAt).
	ExpiresAt time.Time
}

// Available returns true when a credential was found.
func (r ResolvedCredential) Available() bool {
	return r.Source != SourceNone && r.Token != ""
}

// Expired reports whether the credential has expired according to clock.
// Returns false when ExpiresAt is zero (unknown expiry) or when the credential
// is not available.
func (r ResolvedCredential) Expired(clock func() time.Time) bool {
	if !r.Available() || r.ExpiresAt.IsZero() {
		return false
	}
	return !clock().Before(r.ExpiresAt)
}

// ExpiresIn returns the duration until expiry. Returns zero if ExpiresAt is
// zero or already past.
func (r ResolvedCredential) ExpiresIn(clock func() time.Time) time.Duration {
	if r.ExpiresAt.IsZero() {
		return 0
	}
	d := r.ExpiresAt.Sub(clock())
	if d < 0 {
		return 0
	}
	return d
}

// Resolver resolves credentials using an injectable set of I/O dependencies.
// The zero value uses real OS I/O. Construct with New to supply fakes in tests.
type Resolver struct {
	// HomeDir returns the current user's home directory.
	// Defaults to os.UserHomeDir when nil.
	HomeDir func() (string, error)

	// ReadFile reads a file's contents by path.
	// Defaults to os.ReadFile when nil.
	ReadFile func(path string) ([]byte, error)

	// LookupEnv looks up an environment variable.
	// Defaults to os.LookupEnv when nil.
	LookupEnv func(key string) (string, bool)

	// ReadKeychain reads the password payload of a generic keychain item by
	// service name. Defaults to the platform implementation (the `security`
	// CLI on darwin; a not-found stub elsewhere). Inject a fake in tests, or
	// a func returning (nil, ErrKeychainUnavailable) to disable the Keychain
	// step entirely (e.g. when pointing HomeDir at fixture directories).
	ReadKeychain func(service string) ([]byte, error)

	// KeychainService is the keychain item service name to read.
	// Defaults to "Claude Code-credentials" when empty.
	KeychainService string

	// Clock returns the current time. Used by ResolvedCredential.Expired.
	// Defaults to time.Now when nil.
	Clock func() time.Time

	// EnvKey is the environment variable name to check for an API key.
	// Defaults to "ANTHROPIC_API_KEY" when empty.
	EnvKey string
}

// New returns a Resolver with real OS I/O. Pass nil for any dep to use
// the default. Override individual fields on the returned Resolver for
// partial injection.
func New() *Resolver { return &Resolver{} }

// Resolve walks the priority chain and returns the first credential found.
//   - explicitKey: a raw API key from a saved config (empty = not set).
//   - The env var and ~/.claude files are always checked via the injected I/O.
//
// Resolve never returns an error for absent files; only hard parse failures
// on present files are returned. A missing credential returns (zero, nil).
func (r *Resolver) Resolve(explicitKey string) (ResolvedCredential, error) {
	// 1. Explicit key.
	if explicitKey != "" {
		return ResolvedCredential{
			Token:   explicitKey,
			IsOAuth: false,
			Source:  SourceExplicit,
		}, nil
	}

	// 2. Environment variable.
	envKey := r.envKey()
	if val, ok := r.lookupEnv(envKey); ok && val != "" {
		return ResolvedCredential{
			Token:   val,
			IsOAuth: false,
			Source:  SourceEnv,
		}, nil
	}

	// 3a. macOS Keychain (Claude Code's preferred store on darwin). The
	// payload is the same JSON document as ~/.claude/.credentials.json.
	if data, err := r.readKeychain(r.keychainService()); err == nil && len(data) > 0 {
		if cred, ok := parseCredentialsPayload(data); ok {
			return cred, nil
		}
	}

	// 3b/3c. Claude Code CLI files.
	home, err := r.homeDir()
	if err != nil {
		// Can't determine home dir — treat as absent.
		return ResolvedCredential{Source: SourceNone}, nil
	}

	// 3b. ~/.claude/.credentials.json.
	credPath := filepath.Join(home, ".claude", ".credentials.json")
	if cred, ok, err := r.readCredentialsJSON(credPath); err != nil {
		return ResolvedCredential{Source: SourceNone}, err
	} else if ok {
		return cred, nil
	}

	// 3c. ~/.claude.json (legacy).
	altPath := filepath.Join(home, ".claude.json")
	if cred, ok, err := r.readClaudeJSON(altPath); err != nil {
		return ResolvedCredential{Source: SourceNone}, err
	} else if ok {
		return cred, nil
	}

	return ResolvedCredential{Source: SourceNone}, nil
}

// ─── internal helpers ─────────────────────────────────────────────────────────

func (r *Resolver) homeDir() (string, error) {
	if r.HomeDir != nil {
		return r.HomeDir()
	}
	return os.UserHomeDir()
}

func (r *Resolver) readFile(path string) ([]byte, error) {
	if r.ReadFile != nil {
		return r.ReadFile(path)
	}
	return os.ReadFile(path)
}

func (r *Resolver) lookupEnv(key string) (string, bool) {
	if r.LookupEnv != nil {
		return r.LookupEnv(key)
	}
	return os.LookupEnv(key)
}

func (r *Resolver) envKey() string {
	if r.EnvKey != "" {
		return r.EnvKey
	}
	return "ANTHROPIC_API_KEY"
}

func (r *Resolver) readKeychain(service string) ([]byte, error) {
	if r.ReadKeychain != nil {
		return r.ReadKeychain(service)
	}
	return defaultReadKeychain(service)
}

func (r *Resolver) keychainService() string {
	if r.KeychainService != "" {
		return r.KeychainService
	}
	return "Claude Code-credentials"
}

// credentialsFileShape is the JSON shape of ~/.claude/.credentials.json.
type credentialsFileShape struct {
	ClaudeAiOauth *struct {
		AccessToken  string `json:"accessToken"`
		RefreshToken string `json:"refreshToken"`
		// ExpiresAt is unix milliseconds. May be absent (nil).
		ExpiresAt *int64 `json:"expiresAt"`
	} `json:"claudeAiOauth"`
	// Some versions write a top-level apiKey.
	APIKey string `json:"apiKey"`
}

// claudeJSONShape is the shape of ~/.claude.json (legacy / alternate).
type claudeJSONShape struct {
	OAuthToken *struct {
		AccessToken string `json:"access_token"`
	} `json:"oauthToken"`
	APIKey string `json:"apiKey"`
}

func (r *Resolver) readCredentialsJSON(path string) (ResolvedCredential, bool, error) {
	data, err := r.readFile(path)
	if err != nil {
		return ResolvedCredential{}, false, nil // absent — not an error
	}
	cred, ok := parseCredentialsPayload(data)
	return cred, ok, nil
}

// parseCredentialsPayload parses the Claude Code credentials JSON document.
// The same shape is used by ~/.claude/.credentials.json and the macOS
// Keychain item payload. Corrupt or empty payloads report not-found.
func parseCredentialsPayload(data []byte) (ResolvedCredential, bool) {
	data = []byte(strings.TrimSpace(string(data)))
	if len(data) == 0 {
		return ResolvedCredential{}, false
	}
	var cf credentialsFileShape
	if err := json.Unmarshal(data, &cf); err != nil {
		return ResolvedCredential{}, false
	}
	if cf.ClaudeAiOauth != nil && cf.ClaudeAiOauth.AccessToken != "" {
		var expiresAt time.Time
		if cf.ClaudeAiOauth.ExpiresAt != nil {
			// unix milliseconds → time.Time
			ms := *cf.ClaudeAiOauth.ExpiresAt
			expiresAt = time.Unix(ms/1000, (ms%1000)*int64(time.Millisecond)).UTC()
		}
		return ResolvedCredential{
			Token:     cf.ClaudeAiOauth.AccessToken,
			IsOAuth:   true,
			Source:    SourceClaudeCLI,
			ExpiresAt: expiresAt,
		}, true
	}
	if cf.APIKey != "" {
		return ResolvedCredential{
			Token:   cf.APIKey,
			IsOAuth: false,
			Source:  SourceClaudeCLI,
		}, true
	}
	return ResolvedCredential{}, false
}

func (r *Resolver) readClaudeJSON(path string) (ResolvedCredential, bool, error) {
	data, err := r.readFile(path)
	if err != nil {
		return ResolvedCredential{}, false, nil
	}
	data = []byte(strings.TrimSpace(string(data)))
	if len(data) == 0 {
		return ResolvedCredential{}, false, nil
	}
	var cf claudeJSONShape
	if err := json.Unmarshal(data, &cf); err != nil {
		return ResolvedCredential{}, false, nil
	}
	if cf.OAuthToken != nil && cf.OAuthToken.AccessToken != "" {
		return ResolvedCredential{
			Token:   cf.OAuthToken.AccessToken,
			IsOAuth: true,
			Source:  SourceClaudeCLI,
		}, true, nil
	}
	if cf.APIKey != "" {
		return ResolvedCredential{
			Token:   cf.APIKey,
			IsOAuth: false,
			Source:  SourceClaudeCLI,
		}, true, nil
	}
	return ResolvedCredential{}, false, nil
}
