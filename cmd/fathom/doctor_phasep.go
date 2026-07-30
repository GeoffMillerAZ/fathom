package main

// doctor_phasep.go — phase-P doctor checks (production tokenomics +
// scoped config, docs/design/production-tokenomics.md /
// docs/design/scoped-config.md). House honesty discipline (doctor.go's own
// doc comment): an uncheckable state is `skipped` with a reason, never a
// guessed pass; a real, observable condition worth attention but not a
// wiring failure is `warn`, never `fail`.

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/geoffmilleraz/fathom/internal/connector"
	"github.com/geoffmilleraz/fathom/internal/filter"
	"github.com/geoffmilleraz/fathom/internal/guardspec"
)

// checkChannelsEnv reports whether FATHOM_CHANNELS=off is set in doctor's
// own environment — the honesty half of the kill switch (channels.go):
// a silent kill switch nobody can detect repeats the O15 failure mode, so
// doctor names it loudly whenever it finds hooks disabled by environment.
// doctor itself is never silenced by FATHOM_CHANNELS — it is a diagnostic,
// not a hook.
func checkChannelsEnv() doctorCheck {
	const name = "channels_env"
	if fathomChannelsDisabled() {
		return doctorCheck{
			Name:   name,
			Status: doctorWarn,
			Reason: "FATHOM_CHANNELS=off is set in this environment — every fathom hook (session-hook, edit-hook pre/post, the discovery observer, stop-hook) exits in total silence",
			Remedy: `unset FATHOM_CHANNELS (or set it to anything other than "off") to restore fathom's coding-agent channels`,
		}
	}
	return doctorCheck{Name: name, Status: doctorPass, Reason: `FATHOM_CHANNELS is not "off" — hooks run normally`}
}

// checkIdentCacheEnv is checkChannelsEnv's twin for the --no-cache escape
// (identcache.go, docs/design/production-tokenomics.md §7): honest
// disclosure when the service-identity cache is disabled by environment,
// never silent.
func checkIdentCacheEnv() doctorCheck {
	const name = "ident_cache_env"
	if identCacheDisabled() {
		return doctorCheck{
			Name:   name,
			Status: doctorWarn,
			Reason: "FATHOM_IDENT_NO_CACHE is set — every hook re-resolves service identity via the daemon on every call (higher latency/load, never a correctness issue)",
			Remedy: "unset FATHOM_IDENT_NO_CACHE to restore identity caching",
		}
	}
	return doctorCheck{Name: name, Status: doctorPass, Reason: fmt.Sprintf("service-identity caching is active (TTL %s)", identCacheTTL)}
}

// userScopeIgnoreWalkCap bounds checkUserScopeIgnore's filesystem walk — a
// doctor check must stay fast even on a very large repo; this is a
// diagnostic sample, not an exhaustive audit.
const userScopeIgnoreWalkCap = 20000

// checkUserScopeIgnore is scoped-config's disclosure check
// (docs/design/scoped-config.md §4): names ~/.fathom/ignore when it is in
// effect for root, and — the "why can't fathom see my code" pre-empt —
// warns when a USER-scope pattern, specifically, is what excludes an
// otherwise-indexable path (a file that would be indexed under
// project-scope-only exclusions, but is hidden once the user-scope
// defaults are unioned in). A malformed user-scope file (e.g. a re-include
// violation) is reported as its own warn, naming the file, rather than
// silently degrading the way the read-time LoadIgnore convenience does.
func checkUserScopeIgnore(root string) doctorCheck {
	const name = "user_scope_ignore"
	userPath := connector.UserIgnorePath()
	if userPath == "" {
		return doctorCheck{Name: name, Status: doctorPass, Reason: "no resolvable $HOME — no user-scope ignore file to check"}
	}
	if _, err := os.Stat(userPath); err != nil {
		return doctorCheck{Name: name, Status: doctorPass, Reason: "no ~/.fathom/ignore configured"}
	}

	union, counts, err := connector.LoadIgnoreScoped(root)
	if err != nil {
		return doctorCheck{
			Name: name, Status: doctorWarn,
			Reason: fmt.Sprintf("~/.fathom/ignore is invalid: %v", err),
			Remedy: "fix the reported line, or see docs/design/scoped-config.md §2 (user scope may only ADD exclusions, never re-include)",
		}
	}
	projectOnly := connector.ProjectOnlyIgnore(root)

	hidden := walkForUserHiddenFiles(root, projectOnly, union)
	if len(hidden) > 0 {
		return doctorCheck{
			Name: name, Status: doctorWarn,
			Reason: fmt.Sprintf("~/.fathom/ignore (%d pattern(s)) is excluding %d otherwise-indexable file(s) from this repo, e.g. %q — this may be why fathom can't see some of your code", counts.User, len(hidden), hidden[0]),
			Remedy: fmt.Sprintf("review %s, or add a project-scope override in .fathomignore (project scope always wins — docs/design/scoped-config.md law 13)", userPath),
		}
	}
	return doctorCheck{Name: name, Status: doctorPass, Reason: fmt.Sprintf("~/.fathom/ignore in effect (%d pattern(s)); excluding %d pattern(s) total (%d project, %d user-scope)", counts.User, counts.Project+counts.User, counts.Project, counts.User)}
}

// walkForUserHiddenFiles samples root (capped at userScopeIgnoreWalkCap
// entries) for regular files excluded by union but NOT by projectOnly —
// i.e. excluded specifically because of the user-scope union, never
// because of the project's own .fathomignore. Returns up to a handful of
// example relative paths (sorted, first-found order from a deterministic
// walk), nil when none found or the walk cannot proceed at all.
func walkForUserHiddenFiles(root string, projectOnly, union *filter.Matcher) []string {
	if union == nil {
		return nil
	}
	var hidden []string
	visited := 0
	_ = filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil //nolint:nilerr // best-effort diagnostic walk; a single unreadable entry must not abort the check
		}
		if visited >= userScopeIgnoreWalkCap || len(hidden) >= 5 {
			return filepath.SkipAll
		}
		visited++
		rel, relErr := filepath.Rel(root, path)
		if relErr != nil || rel == "." {
			return nil
		}
		rel = filepath.ToSlash(rel)
		if d.IsDir() {
			if connector.IsSkippedDir(d.Name()) {
				return filepath.SkipDir
			}
			if projectOnly != nil && projectOnly.Match(rel+"/") {
				return filepath.SkipDir // project already excludes this subtree — not the user-scope's doing
			}
			return nil
		}
		if !d.Type().IsRegular() {
			return nil
		}
		if projectOnly != nil && projectOnly.Match(rel) {
			return nil // project already excludes this file
		}
		if union.Match(rel) {
			hidden = append(hidden, rel)
		}
		return nil
	})
	return hidden
}

// checkUserScopeGuard is checkUserScopeIgnore's twin for
// ~/.fathom/guard.cue (docs/design/scoped-config.md deliverable 2/§4):
// names the file when in effect, loud on a governance-field violation
// (LoadUserGuards refuses it — see guardspec.LoadUserGuards's doc
// comment), pass on absence (the common case).
func checkUserScopeGuard() doctorCheck {
	const name = "user_scope_guard"
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return doctorCheck{Name: name, Status: doctorPass, Reason: "no resolvable $HOME — no user-scope guard file to check"}
	}
	path := filepath.Join(home, ".fathom", "guard.cue")
	if _, err := os.Stat(path); err != nil {
		return doctorCheck{Name: name, Status: doctorPass, Reason: "no ~/.fathom/guard.cue configured"}
	}
	gs, err := guardspec.LoadUserGuards(path)
	if err != nil {
		return doctorCheck{
			Name: name, Status: doctorWarn,
			Reason: fmt.Sprintf("~/.fathom/guard.cue is invalid: %v", err),
			Remedy: "fix the reported field — user scope is restricted to note/command_scopes (docs/design/scoped-config.md law 13)",
		}
	}
	return doctorCheck{Name: name, Status: doctorPass, Reason: fmt.Sprintf("~/.fathom/guard.cue in effect — %d command guard(s)", len(gs.Guards))}
}
