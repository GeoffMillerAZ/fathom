package main

// guard.go — the local-only half of guarded edits (docs/design/
// guarded-edits.md): loading a repo's .fathom/guard.cue and matching it
// against a file path or a shell command string, BEFORE any daemon call.
// This is deliberately a pure, offline layer — edithook.go's --pre path
// calls it first and only proceeds to a daemon round-trip on a match,
// which is what makes the "instant silence on a non-matching file: one
// local glob check, zero HTTP calls" contract (design §2.2) provable.

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/geoffmilleraz/fathom/internal/filter"
	"github.com/geoffmilleraz/fathom/internal/guardspec"
)

// loadGuardSet reads <root>/.fathom/guard.cue. ok is false when the file is
// absent (the overwhelmingly common case) OR malformed — a broken
// guard.cue fails OPEN on this hot path exactly like modserve.Bind fails
// open on a broken modules.cue; `fathom doctor` is the loud, non-hot-path
// place a malformed file gets surfaced (design §2.4).
func loadGuardSet(root string) (guardspec.GuardSet, bool) {
	path := filepath.Join(root, ".fathom", "guard.cue")
	if _, err := os.Stat(path); err != nil {
		return guardspec.GuardSet{}, false
	}
	gs, err := guardspec.LoadGuards(path)
	if err != nil {
		return guardspec.GuardSet{}, false
	}
	return gs, true
}

// loadUserGuardSet reads ~/.fathom/guard.cue (docs/design/scoped-config.md
// deliverable 2) via guardspec.LoadUserGuards — the restricted, note+
// command_scopes-only loader. ok is false when $HOME cannot be resolved,
// the file is absent (the overwhelmingly common case), or malformed — the
// same fail-open discipline loadGuardSet uses; `fathom doctor`'s
// user_scope_guard check is the loud surface for a malformed file.
func loadUserGuardSet() (guardspec.GuardSet, bool) {
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return guardspec.GuardSet{}, false
	}
	path := filepath.Join(home, ".fathom", "guard.cue")
	if _, err := os.Stat(path); err != nil {
		return guardspec.GuardSet{}, false
	}
	gs, err := guardspec.LoadUserGuards(path)
	if err != nil {
		return guardspec.GuardSet{}, false
	}
	return gs, true
}

// matchingFileGuard returns the first guard (file order — deterministic)
// whose scopes match repoRelFile, ok=false when none does. A guard with no
// scopes configured (command-only) is skipped here, not an error.
func matchingFileGuard(gs guardspec.GuardSet, repoRelFile string) (guardspec.Guard, bool) {
	for _, g := range gs.Guards {
		if len(g.Scopes) == 0 {
			continue
		}
		if filter.New(g.Scopes).Match(repoRelFile) {
			return g, true
		}
	}
	return guardspec.Guard{}, false
}

// matchingCommandGuard returns the first guard whose command_scopes match
// command, ok=false when none does (design §2.5).
func matchingCommandGuard(gs guardspec.GuardSet, command string) (guardspec.Guard, bool) {
	for _, g := range gs.Guards {
		if len(g.CommandScopes) == 0 {
			continue
		}
		if commandMatches(g.CommandScopes, command) {
			return g, true
		}
	}
	return guardspec.Guard{}, false
}

// commandMatches reports whether command matches any of patterns, where
// each pattern is a simple shell-style glob ("terraform apply*",
// "*/migrate *") with '*' matching ANY sequence of characters, including
// spaces and slashes — this is NOT internal/filter's gitignore-style
// matcher (path.Match/filepath.Match would also be wrong here: their '*'
// stops at a path separator, which would silently break a pattern like
// "kubectl *" against a command containing a '/' argument, e.g.
// "kubectl apply -f infra/prod.yaml"). '?' matches exactly one character,
// mirroring the same shell-glob convention. Case-sensitive: command
// patterns name literal binaries/subcommands.
func commandMatches(patterns []string, command string) bool {
	for _, p := range patterns {
		if globToRegexp(p).MatchString(command) {
			return true
		}
	}
	return false
}

// globToRegexp compiles a shell-glob pattern (only '*' and '?' are
// special) into an anchored regexp. Compilation cannot fail: every
// non-glob rune is escaped via regexp.QuoteMeta before glob runes are
// substituted in, so the result is always a valid pattern.
func globToRegexp(pattern string) *regexp.Regexp {
	var sb strings.Builder
	sb.WriteString("^")
	for _, r := range pattern {
		switch r {
		case '*':
			sb.WriteString(".*")
		case '?':
			sb.WriteString(".")
		default:
			sb.WriteString(regexp.QuoteMeta(string(r)))
		}
	}
	sb.WriteString("$")
	return regexp.MustCompile(sb.String())
}
