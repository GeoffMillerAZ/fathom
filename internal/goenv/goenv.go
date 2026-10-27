// Package goenv is the environment every go command fathom runs on its own
// hands the toolchain (ADR-083): the typed pass at ingest and in the daemon
// (internal/fusion TypedEdgeEnv, and the toolchain shim that fronts every
// `go` the daemon execs), ask's module loads (internal/ask) and the
// layer-rules import graph (internal/guardspec/layerrules).
//
// Offline: GOPROXY=off and GOSUMDB=off, so `go list` never downloads a
// module a cloned repository names (a cold module cache fails with the go
// command's own refusal, which the callers name with its remedy);
// GOTELEMETRY=off; GOTOOLCHAIN=local last (K22, no toolchain a go.mod names
// is fetched or run); and an inherited -mod=mod dropped from GOFLAGS, since
// it lets `go list` look up missing imports and rewrite the adopter's
// go.mod and go.sum. -mod=vendor, -mod=readonly and every other flag are
// kept: they never reach the network or write the repository.
//
// The one opt-out is the user's own `[ingest] go_network = true` in
// ~/.fathom/config.toml (internal/config, user scope only): it passes the
// inherited GOPROXY and GOSUMDB through. The toolchain pin and the -mod=mod
// strip stay.
//
// GOTELEMETRY=off is a declaration, not a switch: the go command (go1.26)
// reads its telemetry mode from `go telemetry` and calls the variable
// non-settable. Uploading happens only in mode "on", the user's own choice.
//
// The explicit verbs a user types keep their own environments: `fathom
// receipt verify` and `receipt certify` already run offline
// (internal/receiptverify), and `fathom doctor --stranger` runs the
// repository's own script.
package goenv

import (
	"strings"

	"github.com/geoffmilleraz/fathom/internal/config"
)

// LookupRefused is the go command's own words for a module it may not fetch
// under GOPROXY=off.
const LookupRefused = "module lookup disabled by GOPROXY=off"

// toolchainLocal pins the go command to the toolchain on PATH (K22).
const toolchainLocal = "GOTOOLCHAIN=local"

// Offline returns env with the module network closed (see the package
// doc). Later entries win in exec's environment, so the overrides are
// appended after env, which is kept entry for entry; GOTOOLCHAIN=local is
// the last entry. network true leaves GOPROXY and GOSUMDB as env has them.
// A GOFLAGS entry is appended only when the inherited one carried -mod=mod.
func Offline(env []string, network bool) []string {
	out := make([]string, 0, len(env)+5)
	out = append(out, env...)
	flags := ""
	for _, kv := range env {
		if v, ok := strings.CutPrefix(kv, "GOFLAGS="); ok {
			flags = v
		}
	}
	if kept, stripped := StripModMod(flags); stripped {
		out = append(out, "GOFLAGS="+kept)
	}
	if !network {
		out = append(out, "GOPROXY=off", "GOSUMDB=off")
	}
	return append(out, "GOTELEMETRY=off", toolchainLocal)
}

// Apply is Offline with the user's `[ingest] go_network` setting
// (config.UserGoNetwork), read each call so a changed setting needs no
// restart.
func Apply(env []string) []string {
	return Offline(env, config.UserGoNetwork())
}

// StripModMod returns GOFLAGS' flags without -mod=mod (or --mod=mod), every
// other flag kept in order, and whether it removed one.
func StripModMod(flags string) (string, bool) {
	fields := strings.Fields(flags)
	kept := fields[:0:0]
	for _, f := range fields {
		if f == "-mod=mod" || f == "--mod=mod" {
			continue
		}
		kept = append(kept, f)
	}
	if len(kept) == len(fields) {
		return flags, false
	}
	return strings.Join(kept, " "), true
}

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
