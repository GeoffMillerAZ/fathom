// Package statedir resolves the per-repository directory fathom's runtime
// state belongs under.
//
// O63 item 5: fathom's hooks and runtime were writing into the subject
// repository's own tree — workspace/logs/sessions.jsonl,
// workspace/logs/observed-discoveries.jsonl, workspace/artifacts/retros/*
// .json, workspace/artifacts/narrate/story-*.jsonl, .fathom/receipts.jsonl,
// MCP call logs — so a wired bench arm's diff carried fourteen to sixteen
// fathom-owned files, and a real adopter would find hook output inside
// their own repository. This package is the fix: by default, state lives
// OUTSIDE the repository entirely, under $XDG_STATE_HOME/fathom/<repo-id>/
// (default ~/.local/state/fathom/<repo-id>/) — so the only in-tree
// footprint after `fathom init` + a session hook + an edit hook is the
// CLAUDE.md/AGENTS.md stanza, .fathom/ config (capabilities/concerns/
// guard/modules.cue, owned.json) and .fathomignore.
//
// Two escape hatches keep this from breaking anything that WANTS the old,
// in-tree, versionable-by-choice layout:
//
//   - FATHOM_STATE_DIR (env var — the hooks' fixed-command-line callers
//     cannot set a flag) names the state directory directly.
//   - A repository's own `.fathom/config.toml` [daemon] state_dir =
//     "workspace" restores the pre-O63 convention (<repo>/workspace/...)
//     for that repository specifically. fathom's own repository uses
//     this: its Taskfile and docs still reference workspace/ paths for its
//     own dev loop.
package statedir

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"

	"github.com/geoffmilleraz/fathom/internal/config"
	"github.com/geoffmilleraz/fathom/internal/projectdir"
)

// EnvOverride is the env var that names the state directory directly,
// overriding everything else (including a repo's own state_dir config).
const EnvOverride = "FATHOM_STATE_DIR"

// StateDirWorkspace is the one recognised `[daemon] state_dir` value
// (schema.cue #Daemon.state_dir): restores the pre-O63 in-tree convention.
const StateDirWorkspace = "workspace"

// RepoID returns a stable, filesystem-safe identifier for the repository
// rooted at root: sha256 of root's canonicalized absolute path, hex-encoded
// and truncated to 16 bytes (32 hex characters). It is stable across
// sessions for the SAME checkout and distinct across different checkouts
// (even two same-named repos, or two clones of the same repo, on the same
// machine) — never the raw path itself, which would make the state
// directory's own name leak a full filesystem path.
func RepoID(root string) string {
	abs, err := filepath.Abs(root)
	if err != nil {
		abs = root
	}
	sum := sha256.Sum256([]byte(filepath.Clean(abs)))
	return hex.EncodeToString(sum[:16])
}

// Dir is the pure half of state-directory resolution: given root and the
// repository's own state_dir config value (as read from its
// .fathom/config.toml [daemon] section — "" when unset or the file/key
// does not exist), it returns the directory fathom's runtime state for
// this repository belongs under. No I/O beyond reading FATHOM_STATE_DIR,
// XDG_STATE_HOME and the user's home directory — never touches disk, never
// reads a config file itself (Resolve does that; this stays testable with
// plain strings).
//
// Precedence:
//
//  1. FATHOM_STATE_DIR — used verbatim if absolute, else resolved against
//     root.
//  2. stateDirConfig == "workspace" → root/workspace (the pre-O63
//     in-tree convention).
//  3. $XDG_STATE_HOME/fathom/<repo-id>/, default
//     ~/.local/state/fathom/<repo-id>/.
//  4. root/workspace as the last-resort fallback when no home directory
//     can be resolved at all — state must go somewhere, and staying
//     in-tree beats silently writing nowhere.
func Dir(root, stateDirConfig string) string {
	if v := os.Getenv(EnvOverride); v != "" {
		if filepath.IsAbs(v) {
			return filepath.Clean(v)
		}
		return filepath.Join(root, v)
	}
	if stateDirConfig == StateDirWorkspace {
		return filepath.Join(root, "workspace")
	}
	if base := xdgStateHome(); base != "" {
		return filepath.Join(base, "fathom", RepoID(root))
	}
	return filepath.Join(root, "workspace")
}

// xdgStateHome returns $XDG_STATE_HOME, or ~/.local/state when unset, or
// "" when neither can be resolved.
func xdgStateHome() string {
	if v := os.Getenv("XDG_STATE_HOME"); v != "" {
		return v
	}
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return ""
	}
	return filepath.Join(home, ".local", "state")
}

// Resolve is Dir's convenience wrapper for the common case: it reads
// root's OWN project config.toml (never the user layer — state_dir is a
// per-repository opt-out, not something one repo's choice should silently
// apply to every other repo on the machine) for [daemon] state_dir, then
// calls Dir. A repository with no .fathom/config.toml at all — the common
// case, including every fresh adopter — is detected with a bare os.Stat
// and never pays a config parse/CUE-validate at all.
func Resolve(root string) string {
	stateDirConfig := ""
	projPath := projectdir.Config.Path(root)
	if _, err := os.Stat(projPath); err == nil {
		if loaded, lerr := config.Load("", projPath); lerr == nil {
			stateDirConfig = loaded.Config.Daemon.StateDir
		}
	}
	return Dir(root, stateDirConfig)
}
