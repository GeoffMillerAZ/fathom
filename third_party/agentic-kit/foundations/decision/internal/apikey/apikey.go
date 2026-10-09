// Package apikey resolves the API key a remote decision adapter sends, and
// redacts keys from provider text, for every adapter under
// foundations/decision (typesafe, openai). It is internal: adapters expose
// its three sources as configuration fields of their own (APIKey,
// APIKeyFile, APIKeyEnv), and nothing outside foundations/decision imports
// it.
//
// # Sources
//
// A Source names a key in one of three ways, tried in this order:
//
//  1. Value: the key itself, set in code by a caller that resolved it
//     elsewhere (a keychain, a secret manager). Never decoded from
//     configuration: configuration names a file or a variable instead.
//  2. File: a path to a file holding the key ("~" and "~/..." expand to the
//     user's home directory). The file is read at first use, the key is
//     whitespace-trimmed, and a file that grants any access to group or
//     other users is refused with an error naming the path and the fix
//     (chmod 600 <path>). A file wins over an environment variable when
//     both are set: a daemon started by a service manager does not inherit
//     the login shell's environment, so a file reference is the portable
//     way to hand it a key, and the key never appears in configuration.
//  3. Env: the name of an environment variable holding the key.
//
// Every error names the source (the path or the variable's name) and never
// the key or any part of it; a missing key is an error, never a panic.
//
// # Redaction
//
// Redact removes known keys, bearer tokens, Authorization header values
// and key-shaped "sk-" tokens from text a provider sent back (an HTTP error
// body may echo the request's Authorization header), so an adapter's error
// can carry the provider's message without carrying a key.
//
// Concurrency: Source and Resolver are values with no mutable state and
// are safe for concurrent use; Lazy guards its cached key with a
// sync.Mutex and is safe for concurrent use.
package apikey

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"sync"
	"unicode"
)

// maxKeyFileBytes bounds how much of a key file is read: a key file holds
// one key, so anything larger is a wrong path, not a key.
const maxKeyFileBytes = 64 << 10

// Source names where a key comes from. The zero Source names none.
type Source struct {
	// Value is the key itself, set in code. It wins over File and Env and
	// is used as given (not trimmed or checked).
	Value string
	// File is a path to a file holding the key; "~" and "~/..." expand to
	// the user's home directory. It wins over Env.
	File string
	// Env is the name of an environment variable holding the key.
	Env string
}

// Configured reports whether s names any source.
func (s Source) Configured() bool {
	return s.Value != "" || s.File != "" || s.Env != ""
}

// String describes the source that Resolve reads, never the key: "key file
// <path>", "environment variable <name>", "key set in code" or "no key
// source".
func (s Source) String() string {
	switch {
	case s.Value != "":
		return "key set in code"
	case s.File != "":
		return fmt.Sprintf("key file %q", s.File)
	case s.Env != "":
		return fmt.Sprintf("environment variable %q", s.Env)
	default:
		return "no key source"
	}
}

// Resolver reads a Source. Its zero value reads the process environment
// and home directory; tests inject both.
type Resolver struct {
	// LookupEnv reads an environment variable. Nil means os.LookupEnv.
	LookupEnv func(name string) (string, bool)
	// HomeDir returns the home directory "~" expands to. Nil means
	// os.UserHomeDir.
	HomeDir func() (string, error)
}

// Resolve returns the key s names (see the package doc for the order). The
// error names the source and, for a file whose permissions are too open,
// the fix; it never contains the key.
func (r Resolver) Resolve(s Source) (string, error) {
	switch {
	case s.Value != "":
		return s.Value, nil
	case s.File != "":
		return r.readFile(s.File)
	case s.Env != "":
		return r.readEnv(s.Env)
	default:
		return "", errors.New("apikey: no API key configured: set a key file or an environment variable name")
	}
}

// readFile reads, checks and trims the key in the file at path.
func (r Resolver) readFile(path string) (string, error) {
	expanded, err := r.expandHome(path)
	if err != nil {
		return "", fmt.Errorf("apikey: key file %q: %w", path, err)
	}
	f, err := os.Open(expanded)
	if err != nil {
		return "", fmt.Errorf("apikey: key file %q: %w", path, unwrapPathError(err))
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil {
		return "", fmt.Errorf("apikey: key file %q: %w", path, unwrapPathError(err))
	}
	if !info.Mode().IsRegular() {
		return "", fmt.Errorf("apikey: key file %q is not a regular file", path)
	}
	if perm := info.Mode().Perm(); runtime.GOOS != "windows" && perm&0o077 != 0 {
		return "", fmt.Errorf("apikey: key file %q grants access to group or other users (mode %#o); fix: chmod 600 %s", path, perm, expanded)
	}
	raw, err := io.ReadAll(io.LimitReader(f, maxKeyFileBytes+1))
	if err != nil {
		return "", fmt.Errorf("apikey: key file %q: %w", path, unwrapPathError(err))
	}
	if len(raw) > maxKeyFileBytes {
		return "", fmt.Errorf("apikey: key file %q is larger than %d bytes; a key file holds one key", path, maxKeyFileBytes)
	}
	return checkKey(string(raw), fmt.Sprintf("key file %q", path))
}

// readEnv reads, checks and trims the key in the environment variable
// name.
func (r Resolver) readEnv(name string) (string, error) {
	lookup := r.LookupEnv
	if lookup == nil {
		lookup = os.LookupEnv
	}
	v, ok := lookup(name)
	if !ok {
		return "", fmt.Errorf("apikey: environment variable %q is not set", name)
	}
	return checkKey(v, fmt.Sprintf("environment variable %q", name))
}

// checkKey trims raw and refuses an empty key or one with whitespace or a
// control character inside (two lines, say), naming source and never the
// content.
func checkKey(raw, source string) (string, error) {
	key := strings.TrimSpace(raw)
	if key == "" {
		return "", fmt.Errorf("apikey: %s is empty", source)
	}
	if strings.IndexFunc(key, func(c rune) bool { return unicode.IsSpace(c) || unicode.IsControl(c) }) >= 0 {
		return "", fmt.Errorf("apikey: %s holds whitespace or a control character inside the key; it must hold exactly one key", source)
	}
	return key, nil
}

// expandHome expands a leading "~" or "~/" in path to the home directory.
func (r Resolver) expandHome(path string) (string, error) {
	if path != "~" && !strings.HasPrefix(path, "~/") {
		return path, nil
	}
	home := r.HomeDir
	if home == nil {
		home = os.UserHomeDir
	}
	dir, err := home()
	if err != nil {
		return "", fmt.Errorf("expanding ~: %w", err)
	}
	return filepath.Join(dir, strings.TrimPrefix(path, "~")), nil
}

// unwrapPathError drops an *os.PathError's own copy of the path (the
// caller's message already names it) and keeps the operation and cause.
func unwrapPathError(err error) error {
	var pe *os.PathError
	if errors.As(err, &pe) {
		return fmt.Errorf("%s: %w", pe.Op, pe.Err)
	}
	return err
}

// Lazy resolves a Source at first use and keeps the key for the life of
// the value. A failed resolution is not kept: the next call tries again,
// so fixing a file's mode or setting a variable needs no restart.
//
// Concurrency: Lazy guards its key with a sync.Mutex and is safe for
// concurrent use.
type Lazy struct {
	src Source
	res Resolver

	mu  sync.Mutex
	key string
}

// NewLazy returns a Lazy over src, read through res.
func NewLazy(src Source, res Resolver) *Lazy {
	return &Lazy{src: src, res: res}
}

// Source returns the Source l reads.
func (l *Lazy) Source() Source { return l.src }

// Key returns the key, resolving it on the first successful call.
func (l *Lazy) Key() (string, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.key != "" {
		return l.key, nil
	}
	key, err := l.res.Resolve(l.src)
	if err != nil {
		return "", err
	}
	l.key = key
	return key, nil
}

// Redacted is what Redact puts in place of a secret.
const Redacted = "[REDACTED]"

var (
	// authorizationValue matches an Authorization header or field and its
	// value, with or without a "Bearer" scheme, as JSON or header text.
	authorizationValue = regexp.MustCompile(`(?i)(authorization["']?\s*[:=]\s*["']?)(?:bearer\s+)?[^\s"',;}\]]+`)
	// bearerToken matches a bearer token anywhere.
	bearerToken = regexp.MustCompile(`(?i)\bbearer\s+[^\s"',;}\]]+`)
	// skToken matches a key-shaped "sk-" token, including a provider's
	// masked echo of one ("sk-proj-****abcd").
	skToken = regexp.MustCompile(`\bsk-[A-Za-z0-9_*.\-]{3,}`)
)

// Redact returns text with every non-empty key in keys, every
// Authorization header or field value, every bearer token and every
// key-shaped "sk-" token replaced by Redacted. It is for provider text an
// adapter puts in an error or a log line; it does not make arbitrary text
// safe to publish.
func Redact(text string, keys ...string) string {
	for _, k := range keys {
		if k != "" {
			text = strings.ReplaceAll(text, k, Redacted)
		}
	}
	text = authorizationValue.ReplaceAllString(text, "${1}"+Redacted)
	text = bearerToken.ReplaceAllString(text, "Bearer "+Redacted)
	return skToken.ReplaceAllString(text, Redacted)
}
