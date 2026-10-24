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

	"github.com/geoffmilleraz/fathom/internal/config"
	"github.com/geoffmilleraz/fathom/internal/fathomhome"
	"github.com/geoffmilleraz/fathom/internal/filter"
	"github.com/geoffmilleraz/fathom/internal/guardspec"
	"github.com/geoffmilleraz/fathom/internal/projectdir"
)

// loadGuardSet reads <root>/.fathom/guard.cue. ok is false when the file is
// absent (the overwhelmingly common case) OR malformed — a broken
// guard.cue fails OPEN on this hot path exactly like modserve.Bind fails
// open on a broken modules.cue; `fathom doctor` is the loud, non-hot-path
// place a malformed file gets surfaced (design §2.4).
func loadGuardSet(root string) (guardspec.GuardSet, bool) {
	path := projectdir.Guard.Path(root)
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
	path := fathomhome.Path("guard.cue")
	if path == "" {
		return guardspec.GuardSet{}, false
	}
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

// matchingCommandGuard uses guardspec's per-segment matcher, including the
// whole-string matches for repository and user commands.
func matchingCommandGuard(gs guardspec.GuardSet, command string) (guardspec.Guard, bool) {
	return guardspec.MatchCommandGuard(gs, command)
}

// loadCommandGuardSets keeps repository precedence and adds the shipped pack
// only at its policy root, even when no guard.cue exists. A config or pack error
// cannot enable a deny.
func loadCommandGuardSets(root string, configRoots ...string) []guardspec.GuardSet {
	var sets []guardspec.GuardSet
	if gs, ok := loadGuardSet(root); ok {
		sets = append(sets, gs)
	}
	if len(configRoots) == 0 {
		configRoots = []string{root}
	}
	if root != commandPackRoot(configRoots) || !commandGitSafetyEnabled(configRoots) {
		return sets
	}
	if gs, err := guardspec.GitSafetyPack(); err == nil {
		sets = append(sets, gs)
	}
	return sets
}

// commandPackRoot selects the nearest explicit project enforcement setting for
// the pack. Without one, the nearest service root supplies the user/default
// setting. Repository guards retain their own root's policy.
func commandPackRoot(roots []string) string {
	for _, root := range roots {
		loaded, err := config.Load(config.DefaultUserPath(), projectdir.Config.Path(root))
		if err == nil && loaded.Provenance["guards.enforce"].Source == config.SourceProject {
			return root
		}
	}
	if len(roots) == 0 {
		return ""
	}
	return roots[0]
}

// commandGitSafetyEnabled resolves the nearest explicit project setting before
// the user default. All service candidates must use this same root order, so an
// outer candidate cannot re-enable a nested opt-out. Read errors fail open.
func commandGitSafetyEnabled(roots []string) bool {
	for _, root := range roots {
		loaded, err := config.Load(config.DefaultUserPath(), projectdir.Config.Path(root))
		if err != nil {
			return false
		}
		if source := loaded.Provenance["guards.git_safety"].Source; source == config.SourceProject {
			return *loaded.Config.Guards.GitSafety
		}
	}
	loaded, err := config.Load(config.DefaultUserPath(), "")
	if err != nil {
		return false
	}
	return loaded.Config.Guards.GitSafety == nil || *loaded.Config.Guards.GitSafety
}

// commandGuardRoots deduplicates identity aliases without changing precedence.
func commandGuardRoots(candidates []serviceCandidate) []string {
	var roots []string
	for _, candidate := range candidates {
		if len(roots) == 0 || roots[len(roots)-1] != candidate.root {
			roots = append(roots, candidate.root)
		}
	}
	return roots
}
