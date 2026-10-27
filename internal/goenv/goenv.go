// Package goenv is the environment every go command fathom runs on its own
// hands the toolchain (ADR-083): the typed pass at ingest and in the daemon
// (internal/fusion TypedEdgeEnv, and the toolchain shim that fronts every
// `go` the daemon execs), ask's module loads (internal/ask) and the
// layer-rules import graph (internal/guardspec/layerrules).
//
// Offline: GOPROXY=off and GOSUMDB=off, so `go list` never downloads a
// module a cloned repository names (a cold module cache fails with the go
// command's own refusal, which the callers name with its remedy);
// GONOPROXY="," so a GOPRIVATE pattern (GONOPROXY's default) cannot send a
// module straight to its version control past GOPROXY=off; GOTELEMETRY=off;
// GOTOOLCHAIN=local last (K22, no toolchain a go.mod names is fetched or
// run); and -mod=mod dropped from the effective GOFLAGS, since it lets `go
// list` look up missing imports and rewrite the adopter's go.mod and
// go.sum. The effective GOFLAGS is the process's, or else the GOENV file's
// (`go env GOFLAGS`), read the way cmd/go reads it (quoted fields
// included); the result is always set and never empty, so the GOENV file
// can never fill it back in. -mod=vendor, -mod=readonly and every other
// flag are kept: they never reach the network or write the repository.
//
// The one opt-out is the user's own `[ingest] go_network = true` in
// ~/.fathom/config.toml (internal/config, user scope only): it passes the
// inherited GOPROXY, GOSUMDB and GONOPROXY through. The toolchain pin and
// the -mod=mod strip stay.
//
// GOTELEMETRY=off is a declaration, not a switch: the go command (go1.26)
// reads its telemetry mode from `go telemetry` and calls the variable
// non-settable. Uploading happens only in mode "on", the user's own choice.
//
// The explicit verbs a user types: `fathom receipt verify` (its `go mod
// download`, go/packages load and `--run-tests` go test) runs under Offline
// with the network closed whatever go_network says, plus GOWORK=off and
// -mod=readonly (internal/receiptverify), and `fathom doctor --stranger`
// runs the repository's own script.
package goenv

import (
	"context"
	"os/exec"
	"strings"
	"time"

	"github.com/geoffmilleraz/fathom/internal/config"
)

// LookupRefused is the go command's own words for a module it may not fetch
// under GOPROXY=off.
const LookupRefused = "module lookup disabled by GOPROXY=off"

// toolchainLocal pins the go command to the toolchain on PATH (K22).
const toolchainLocal = "GOTOOLCHAIN=local"

// NoProxyNone is the GONOPROXY entry Offline sets: a non-empty list that
// matches no module, so GONOPROXY's default (GOPRIVATE) never applies and
// no module is fetched directly from its version control.
const NoProxyNone = "GONOPROXY=,"

// EmptyFlags is the GOFLAGS value Offline sets when no flag is left: a
// non-empty value cmd/go reads as no flags. An empty GOFLAGS would let the
// GOENV file's GOFLAGS apply.
const EmptyFlags = " "

// Offline returns env with the module network closed (see the package
// doc). Later entries win in exec's environment, so the overrides are
// appended after env, which is kept entry for entry; GOTOOLCHAIN=local is
// the last entry. network true leaves GOPROXY, GOSUMDB and GONOPROXY as env
// has them. A GOFLAGS entry is always appended (Flags).
func Offline(env []string, network bool) []string {
	out := make([]string, 0, len(env)+6)
	out = append(out, env...)
	out = append(out, "GOFLAGS="+Flags(env))
	if !network {
		out = append(out, "GOPROXY=off", "GOSUMDB=off", NoProxyNone)
	}
	return append(out, "GOTELEMETRY=off", toolchainLocal)
}

// Apply is Offline with the user's `[ingest] go_network` setting
// (config.UserGoNetwork), read each call so a changed setting needs no
// restart.
func Apply(env []string) []string {
	return Offline(env, config.UserGoNetwork())
}

// Flags is the GOFLAGS value a go command run under env gets: the
// effective GOFLAGS (env's own, or else the GOENV file's, ConfiguredFlags)
// without -mod=mod, and EmptyFlags when nothing is left.
func Flags(env []string) string {
	flags := ""
	for _, kv := range env {
		if v, ok := strings.CutPrefix(kv, "GOFLAGS="); ok {
			flags = v
		}
	}
	if flags == "" {
		flags = ConfiguredFlags(env)
	}
	kept, _ := StripModMod(flags)
	if strings.TrimSpace(kept) == "" {
		return EmptyFlags
	}
	return kept
}

// configuredFlagsTimeout bounds ConfiguredFlags' `go env` query. The
// environment is built before any caller's context exists (Apply and Offline
// take none), so the query cannot inherit one; a local `go env` answers in
// milliseconds, and one that hangs must not hold up the command it prepares.
const configuredFlagsTimeout = 10 * time.Second

// ConfiguredFlags is the GOFLAGS the go command reads when the environment
// sets none: `go env GOFLAGS` under env with GOFLAGS empty and
// GOTOOLCHAIN=local, which reads the GOENV file and $GOROOT/go.env. ""
// when the go command cannot answer: the caller's go command then gets
// EmptyFlags, which drops the file's flags rather than trusting them.
func ConfiguredFlags(env []string) string {
	ctx, cancel := context.WithTimeout(context.Background(), configuredFlagsTimeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, "go", "env", "GOFLAGS")
	cmd.Env = append(append(make([]string, 0, len(env)+2), env...), "GOFLAGS=", toolchainLocal)
	out, err := cmd.Output()
	if err != nil {
		return ""
	}
	return strings.TrimRight(string(out), "\r\n")
}

// StripModMod returns GOFLAGS' flags without -mod=mod (or --mod=mod),
// fields read the way cmd/go reads them (SplitFlags) and every other flag
// kept in order, and whether it removed one. flags that do not parse are
// returned as they are: the go command refuses them and runs nothing.
func StripModMod(flags string) (string, bool) {
	fields, ok := SplitFlags(flags)
	if !ok {
		return flags, false
	}
	kept := fields[:0:0]
	for _, f := range fields {
		if isModMod(f) {
			continue
		}
		kept = append(kept, f)
	}
	if len(kept) == len(fields) {
		return flags, false
	}
	return JoinFlags(kept), true
}

// isModMod reports whether one GOFLAGS field is -mod=mod or --mod=mod.
func isModMod(f string) bool {
	name, ok := strings.CutPrefix(f, "-")
	if !ok {
		return false
	}
	name = strings.TrimPrefix(name, "-")
	return name == "mod=mod"
}

// SplitFlags splits a GOFLAGS value the way cmd/go does
// (cmd/internal/quoted.Split): fields separated by spaces, tabs, newlines
// or carriage returns, a field that starts with ' or " running to the next
// same quote with no unescaping. ok is false for an unterminated quote.
func SplitFlags(s string) (fields []string, ok bool) {
	for len(s) > 0 {
		for len(s) > 0 && isSpace(s[0]) {
			s = s[1:]
		}
		if len(s) == 0 {
			break
		}
		if q := s[0]; q == '"' || q == '\'' {
			end := strings.IndexByte(s[1:], q)
			if end < 0 {
				return nil, false
			}
			fields = append(fields, s[1:1+end])
			s = s[2+end:]
			continue
		}
		i := 0
		for i < len(s) && !isSpace(s[i]) {
			i++
		}
		fields = append(fields, s[:i])
		s = s[i:]
	}
	return fields, true
}

// JoinFlags joins fields back into a GOFLAGS value SplitFlags reads as
// fields: a field that is empty, holds a separator or starts with a quote
// is quoted, with ' unless it holds one.
func JoinFlags(fields []string) string {
	out := make([]string, len(fields))
	for i, f := range fields {
		switch {
		case f != "" && !strings.ContainsAny(f, " \t\n\r") && f[0] != '\'' && f[0] != '"':
			out[i] = f
		case !strings.Contains(f, "'"):
			out[i] = "'" + f + "'"
		default:
			out[i] = `"` + f + `"`
		}
	}
	return strings.Join(out, " ")
}

func isSpace(c byte) bool { return c == ' ' || c == '\t' || c == '\n' || c == '\r' }

// MissingModule reports whether text carries the go command's refusal to
// fetch a module under GOPROXY=off, and the module it names: "path@version"
// from a whole-load refusal ("go: example.com/m@v1.0.0: module lookup
// disabled by GOPROXY=off"), or "" when the refusal is a package error,
// which names only the importing position (the caller names the package).
func MissingModule(text string) (string, bool) {
	if !strings.Contains(text, LookupRefused) {
		return "", false
	}
	for _, line := range strings.Split(text, "\n") {
		if !strings.Contains(line, LookupRefused) {
			continue
		}
		line = strings.TrimSpace(line)
		if i := strings.LastIndex(line, "go: "); i >= 0 {
			line = line[i+len("go: "):]
		}
		if mod, _, ok := strings.Cut(line, ": "); ok && strings.Contains(mod, "@") && !strings.ContainsAny(mod, " \t") {
			return mod, true
		}
	}
	return "", true
}

// Remedy is the one instruction a missing module's failure names: fill the
// module cache from the module's own directory.
func Remedy(moduleDir string) string {
	return "run go mod download in " + moduleDir
}
