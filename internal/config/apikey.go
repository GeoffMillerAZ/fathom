package config

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"
)

// keyCmdCache holds, in memory for the life of the process, the result of
// each api_key_cmd that succeeded, keyed by its argv (only the user layer
// can name one, so the argv identifies the layer too). A credential helper
// is slow and may prompt; open, doctor and a smoke call would otherwise run
// it two or three times. Never written anywhere; a failure is not cached.
var (
	keyCmdMu    sync.Mutex
	keyCmdCache = map[string]string{}
)

// apiKeyCmdTimeout bounds api_key_cmd's subprocess lifetime. Credential
// helpers (`op read …`, `security find-generic-password …`) are expected to
// return near-instantly; 10s is generous headroom without letting a hung
// helper block a compile/providers/smoke invocation indefinitely.
const apiKeyCmdTimeout = 10 * time.Second

// ResolveAPIKey resolves the credential this profile names, at CALL time —
// never stored on the Profile and never resolved during Load (a successful
// api_key_cmd is reused in memory for the process: keyCmdCache) — so a
// rotated credential (a re-exported env var, a refreshed vault entry) is
// picked up without re-parsing the config file. Precedence when both are
// set: APIKeyCmd wins over APIKeyEnv (an explicit command is a stronger
// signal of operator intent than a bare env-var name).
//
// Returns ("", nil) when the profile names neither — the caller (a
// cmd/fathom build*ClientProfile function) then leaves the adapter's own
// Config.APIKey empty, letting the adapter fall back to ITS OWN default
// env var (e.g. anthropic.Config's APIKeyEnv default "ANTHROPIC_API_KEY"),
// which is what reproduces pre-profile fathom's exact behavior when no
// profile overrides the credential source.
//
// The resolved key is NEVER logged and NEVER embedded in a returned error —
// only the command's argv (which is configuration, not secret) and the
// underlying exec error (e.g. "exit status 1", which os/exec never
// populates with captured stdout/stderr) may appear.
func (p Profile) ResolveAPIKey(ctx context.Context) (string, error) {
	if len(p.APIKeyCmd) > 0 {
		return runAPIKeyCmd(ctx, p.APIKeyCmd)
	}
	if p.APIKeyEnv != "" {
		return os.Getenv(p.APIKeyEnv), nil
	}
	return "", nil
}

// ResolveAPIKey resolves the credential a remote [decide] provider uses
// when fathom itself must hand the adapter a key, at CALL time — same
// discipline as Profile.ResolveAPIKey (never cached, never resolved during
// Load). It follows KeySource: api_key_cmd runs the helper; api_key_file
// returns ("", nil), because the kit adapter reads that file itself at
// first use (TypeSafeConfig/OpenAIConfig.APIKeyFile) and fathom never
// copies its contents; api_key_env, else the transport's well-known
// variable (DefaultDecideKeyEnv: TYPESAFE_API_KEY, OPENROUTER_API_KEY or
// OPENAI_API_KEY), is read directly. Still never logged, never embedded in
// a returned error.
//
// io, when given, is the seam the key is read through (KeyIO, KS-1); none
// means OSKeyIO.
func (d Decide) ResolveAPIKey(ctx context.Context, io ...KeyIO) (string, error) {
	k := keyIOOf(io)
	src := d.KeySource()
	switch {
	case len(src.Cmd) > 0:
		return k.RunCmd(ctx, src.Cmd)
	case src.File != "":
		return "", nil
	}
	v, _ := k.LookupEnv(src.Env)
	return v, nil
}

// runAPIKeyCmd execs argv (no shell — argv[0] is resolved via PATH or as an
// absolute path by exec.LookPath semantics, exactly like exec.Command)
// under apiKeyCmdTimeout and returns its trimmed stdout. Stderr is
// deliberately never captured or surfaced: a credential helper's stderr
// could echo back partial secret material on some failure paths, and the
// secrets doctrine (docs/plans/config-experience.md) is "never logged, never
// embedded in errors" — a generic failure naming the command (not its
// output) is what callers get instead.
func runAPIKeyCmd(ctx context.Context, argv []string) (string, error) {
	if len(argv) == 0 {
		return "", fmt.Errorf("config: api_key_cmd is empty")
	}

	id := strings.Join(argv, "\x00")
	keyCmdMu.Lock()
	defer keyCmdMu.Unlock()
	if k, ok := keyCmdCache[id]; ok {
		return k, nil
	}

	cctx, cancel := context.WithTimeout(ctx, apiKeyCmdTimeout)
	defer cancel()

	cmd := exec.CommandContext(cctx, argv[0], argv[1:]...)
	var stdout bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = nil // never captured — see doc comment

	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("config: api_key_cmd %q failed (output withheld): %w", argv[0], err)
	}
	key := strings.TrimSpace(stdout.String())
	keyCmdCache[id] = key
	return key, nil
}
