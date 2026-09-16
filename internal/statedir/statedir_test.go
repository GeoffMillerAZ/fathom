package statedir_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/statedir"
)

// withEnv sets key to value for the duration of the test (restored after),
// or unsets it entirely when value == "" — os.Unsetenv, not
// os.Setenv(key, "") — since Dir/Resolve treat "" and "unset" identically
// but a stray empty-string env var must never mask that.
func withEnv(t *testing.T, key, value string) {
	t.Helper()
	old, had := os.LookupEnv(key)
	if value == "" {
		os.Unsetenv(key)
	} else {
		os.Setenv(key, value)
	}
	t.Cleanup(func() {
		if had {
			os.Setenv(key, old)
		} else {
			os.Unsetenv(key)
		}
	})
}

func TestRepoID_StableAndDistinct(t *testing.T) {
	a := statedir.RepoID("/repo/a")
	a2 := statedir.RepoID("/repo/a")
	b := statedir.RepoID("/repo/b")
	if a != a2 {
		t.Fatalf("RepoID not stable: %q vs %q", a, a2)
	}
	if a == b {
		t.Fatalf("RepoID collided for distinct roots: %q", a)
	}
	if len(a) != 32 { // 16 bytes, hex-encoded
		t.Fatalf("RepoID length = %d, want 32 hex chars", len(a))
	}
}

func TestDir_EnvOverride_Absolute(t *testing.T) {
	withEnv(t, "FATHOM_STATE_DIR", "/abs/state")
	withEnv(t, "XDG_STATE_HOME", "")
	if got := statedir.Dir("/repo", ""); got != "/abs/state" {
		t.Fatalf("Dir = %q, want /abs/state", got)
	}
}

func TestDir_EnvOverride_Relative_ResolvesAgainstRoot(t *testing.T) {
	withEnv(t, "FATHOM_STATE_DIR", "scratch/state")
	if got, want := statedir.Dir("/repo", ""), filepath.Join("/repo", "scratch/state"); got != want {
		t.Fatalf("Dir = %q, want %q", got, want)
	}
}

func TestDir_EnvOverride_WinsOverWorkspaceConfig(t *testing.T) {
	withEnv(t, "FATHOM_STATE_DIR", "/abs/state")
	if got := statedir.Dir("/repo", statedir.StateDirWorkspace); got != "/abs/state" {
		t.Fatalf("Dir = %q, want the env override to win over state_dir: workspace", got)
	}
}

func TestDir_WorkspaceConfig_InTree(t *testing.T) {
	withEnv(t, "FATHOM_STATE_DIR", "")
	want := filepath.Join("/repo", "workspace")
	if got := statedir.Dir("/repo", statedir.StateDirWorkspace); got != want {
		t.Fatalf("Dir = %q, want %q", got, want)
	}
}

func TestDir_Default_UnderXDGStateHome(t *testing.T) {
	withEnv(t, "FATHOM_STATE_DIR", "")
	withEnv(t, "XDG_STATE_HOME", "/xdg/state")
	got := statedir.Dir("/repo", "")
	want := filepath.Join("/xdg/state", "fathom", statedir.RepoID("/repo"))
	if got != want {
		t.Fatalf("Dir = %q, want %q", got, want)
	}
	if strings.Contains(got, "/repo") {
		t.Fatalf("Dir leaked the repo path into the state dir: %q", got)
	}
}

func TestDir_Default_UnderHomeLocalState_WhenXDGUnset(t *testing.T) {
	home := t.TempDir()
	withEnv(t, "FATHOM_STATE_DIR", "")
	withEnv(t, "XDG_STATE_HOME", "")
	withEnv(t, "HOME", home)
	got := statedir.Dir("/repo", "")
	want := filepath.Join(home, ".local", "state", "fathom", statedir.RepoID("/repo"))
	if got != want {
		t.Fatalf("Dir = %q, want %q", got, want)
	}
}

func TestDir_NoHomeResolvable_FallsBackInTree(t *testing.T) {
	withEnv(t, "FATHOM_STATE_DIR", "")
	withEnv(t, "XDG_STATE_HOME", "")
	withEnv(t, "HOME", "")
	want := filepath.Join("/repo", "workspace")
	if got := statedir.Dir("/repo", ""); got != want {
		t.Fatalf("Dir = %q, want the in-tree fallback %q", got, want)
	}
}

func TestDir_TwoDifferentRepos_NeverShareAStateDir(t *testing.T) {
	withEnv(t, "FATHOM_STATE_DIR", "")
	withEnv(t, "XDG_STATE_HOME", "/xdg/state")
	a := statedir.Dir("/repo/a", "")
	b := statedir.Dir("/repo/b", "")
	if a == b {
		t.Fatalf("two different repos resolved to the same state dir: %q", a)
	}
}

// TestResolve_NoConfigFile_NeverPaysConfigLoad: the common case (no
// .fathom/config.toml at all — every fresh adopter) must still resolve
// correctly, going straight to the XDG default with no config parse.
func TestResolve_NoConfigFile_DefaultsToXDG(t *testing.T) {
	withEnv(t, "FATHOM_STATE_DIR", "")
	withEnv(t, "XDG_STATE_HOME", "/xdg/state")
	root := t.TempDir() // no .fathom/config.toml written
	got := statedir.Resolve(root)
	want := filepath.Join("/xdg/state", "fathom", statedir.RepoID(root))
	if got != want {
		t.Fatalf("Resolve = %q, want %q", got, want)
	}
}

// TestResolve_WorkspaceConfig_KeepsInTreeConvention: a repo whose
// .fathom/config.toml sets [daemon] state_dir = "workspace" (fathom's own
// repository's own escape hatch) keeps the old location.
func TestResolve_WorkspaceConfig_KeepsInTreeConvention(t *testing.T) {
	withEnv(t, "FATHOM_STATE_DIR", "")
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".fathom"), 0o755); err != nil {
		t.Fatal(err)
	}
	cfg := "[daemon]\nstate_dir = \"workspace\"\n"
	if err := os.WriteFile(filepath.Join(root, ".fathom", "config.toml"), []byte(cfg), 0o644); err != nil {
		t.Fatal(err)
	}
	want := filepath.Join(root, "workspace")
	if got := statedir.Resolve(root); got != want {
		t.Fatalf("Resolve = %q, want %q (state_dir: workspace)", got, want)
	}
}

// TestResolve_ConfigWithoutStateDir_DefaultsToXDG: a project config.toml
// exists (e.g. it sets a default_profile) but never mentions state_dir —
// must still default to the external XDG location, not silently fall back
// in-tree.
func TestResolve_ConfigWithoutStateDir_DefaultsToXDG(t *testing.T) {
	withEnv(t, "FATHOM_STATE_DIR", "")
	withEnv(t, "XDG_STATE_HOME", "/xdg/state")
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".fathom"), 0o755); err != nil {
		t.Fatal(err)
	}
	cfg := "default_profile = \"work\"\n\n[profiles.work]\nprovider = \"claude-cli\"\n"
	if err := os.WriteFile(filepath.Join(root, ".fathom", "config.toml"), []byte(cfg), 0o644); err != nil {
		t.Fatal(err)
	}
	want := filepath.Join("/xdg/state", "fathom", statedir.RepoID(root))
	if got := statedir.Resolve(root); got != want {
		t.Fatalf("Resolve = %q, want %q", got, want)
	}
}
