package config

// decidekey.go — where a remote [decide] provider's key comes from
// (DP-28..DP-30, docs/plans/decision-port.md section H): a file path, a
// variable name or a credential helper, never the key itself. Everything
// here names the source; nothing here prints, logs or returns the key.

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// The transports a remote [decide] provider speaks over (schema.cue
// #Decide.transport).
const (
	// DecideTransportTypeSafe is TypeSafe's own API, provider "typesafe"'s
	// default.
	DecideTransportTypeSafe = "typesafe"
	// DecideTransportOpenRouter is the same System One wire through
	// OpenRouter's Decisions router (provider "typesafe" only).
	DecideTransportOpenRouter = "openrouter"
	// DecideTransportOpenAI is OpenAI's own API, provider "openai"'s only
	// transport.
	DecideTransportOpenAI = "openai"
)

// KeyTransportNone is Decide.KeyTransport when the config layer that named
// the key source set no remote provider: a key named for no provider is
// sent to none.
const KeyTransportNone = "none"

// EffectiveTransport returns the transport d's provider speaks over:
// d.Transport when set, else the provider's own API ("typesafe" for
// provider typesafe, "openai" for provider openai), and "" for a provider
// that sends nothing off-host by itself (none, llm).
func (d Decide) EffectiveTransport() string {
	switch d.Provider {
	case "typesafe":
		if d.Transport != "" {
			return d.Transport
		}
		return DecideTransportTypeSafe
	case "openai":
		if d.Transport != "" {
			return d.Transport
		}
		return DecideTransportOpenAI
	}
	return ""
}

// EffectiveBaseURL returns BaseURL, else its older name Endpoint, else ""
// (the adapter's own default for the transport).
func (d Decide) EffectiveBaseURL() string {
	if d.BaseURL != "" {
		return d.BaseURL
	}
	return d.Endpoint
}

// DefaultDecideKeyEnv is the well-known variable a transport's key is read
// from when [decide] names no source: OPENROUTER_API_KEY for openrouter,
// OPENAI_API_KEY for openai, and TYPESAFE_API_KEY otherwise (the variable
// provider typesafe has always read).
func DefaultDecideKeyEnv(transport string) string {
	switch transport {
	case DecideTransportOpenRouter:
		return "OPENROUTER_API_KEY"
	case DecideTransportOpenAI:
		return "OPENAI_API_KEY"
	}
	return "TYPESAFE_API_KEY"
}

// KeySource names where a remote [decide] provider's key comes from: at
// most one of Cmd, File and Env is set. It never holds the key.
type KeySource struct {
	// Cmd is the api_key_cmd argv; its trimmed stdout is the key.
	Cmd []string
	// File is the api_key_file path ("~/" expands to the home directory).
	File string
	// Env is the variable's name: api_key_env, or the transport's
	// well-known variable when Default is true.
	Env string
	// Default is true when Env is DefaultDecideKeyEnv's, not a configured
	// name.
	Default bool
}

// KeySource returns the one source d's remote provider reads its key from,
// in precedence order: api_key_cmd, api_key_file, api_key_env, then the
// transport's well-known variable (DefaultDecideKeyEnv).
func (d Decide) KeySource() KeySource {
	switch {
	case len(d.APIKeyCmd) > 0:
		return KeySource{Cmd: append([]string(nil), d.APIKeyCmd...)}
	case d.APIKeyFile != "":
		return KeySource{File: d.APIKeyFile}
	case d.APIKeyEnv != "":
		return KeySource{Env: d.APIKeyEnv}
	}
	return KeySource{Env: DefaultDecideKeyEnv(d.EffectiveTransport()), Default: true}
}

// String names the source, never the key: `api_key_cmd "op"`, `key file
// "~/keys/openai"`, `environment variable "OPENAI_API_KEY"`.
func (s KeySource) String() string {
	switch {
	case len(s.Cmd) > 0:
		return fmt.Sprintf("api_key_cmd %q", s.Cmd[0])
	case s.File != "":
		return fmt.Sprintf("key file %q", s.File)
	case s.Env != "":
		return fmt.Sprintf("environment variable %q", s.Env)
	}
	return "no API key source"
}

// Probe reports whether s resolves, without reading a key out of a file:
// a file must exist, be a regular non-empty file and grant no access to
// group or other users ("present, mode 0600"); a variable must be set and
// not blank ("set"); a credential helper must exit 0 with output ("returned
// a key", the output discarded). The error names the source and what is
// wrong with it (a too-open file names the chmod that fixes it). Neither
// the detail nor the error ever carries the key.
func (s KeySource) Probe(ctx context.Context) (string, error) {
	switch {
	case len(s.Cmd) > 0:
		key, err := runAPIKeyCmd(ctx, s.Cmd)
		if err != nil {
			return "", err
		}
		if key == "" {
			return "", fmt.Errorf("%s printed nothing", s)
		}
		return "returned a key", nil
	case s.File != "":
		return probeKeyFile(s.File)
	case s.Env != "":
		v, ok := os.LookupEnv(s.Env)
		if !ok {
			return "", fmt.Errorf("%s is not set", s)
		}
		if strings.TrimSpace(v) == "" {
			return "", fmt.Errorf("%s is empty", s)
		}
		return "set", nil
	}
	return "", fmt.Errorf("no API key source: set api_key_file or api_key_env")
}

// probeKeyFile stats the key file at path ("~/" expanded) and checks what
// the kit's adapters check at first use, short of reading it.
func probeKeyFile(path string) (string, error) {
	expanded := path
	if path == "~" || strings.HasPrefix(path, "~/") {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("key file %q: expanding ~: %w", path, err)
		}
		expanded = filepath.Join(home, strings.TrimPrefix(path, "~"))
	}
	info, err := os.Stat(expanded)
	if err != nil {
		if os.IsNotExist(err) {
			return "", fmt.Errorf("key file %q not found", path)
		}
		return "", fmt.Errorf("key file %q: %w", path, unwrapPathErr(err))
	}
	if !info.Mode().IsRegular() {
		return "", fmt.Errorf("key file %q is not a regular file", path)
	}
	perm := info.Mode().Perm()
	if perm&0o077 != 0 {
		return "", fmt.Errorf("key file %q grants access to group or other users (mode %#o); fix: chmod 600 %s", path, perm, expanded)
	}
	if info.Size() == 0 {
		return "", fmt.Errorf("key file %q is empty", path)
	}
	return fmt.Sprintf("present, mode %#o", perm), nil
}

// unwrapPathErr keeps an *os.PathError's operation and cause and drops its
// copy of the path (the caller's message names it already).
func unwrapPathErr(err error) error {
	if pe, ok := err.(*os.PathError); ok {
		return fmt.Errorf("%s: %w", pe.Op, pe.Err)
	}
	return err
}
