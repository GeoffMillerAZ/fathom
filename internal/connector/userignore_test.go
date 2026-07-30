package connector_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/connector"
	"github.com/geoffmilleraz/fathom/internal/core"
)

// withHome points $HOME at a fresh t.TempDir() so UserIgnorePath resolves
// to a hermetic, test-owned location instead of the real machine $HOME.
func withHome(t *testing.T) string {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	return home
}

func writeUserIgnore(t *testing.T, home, content string) {
	t.Helper()
	dir := filepath.Join(home, ".fathom")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "ignore"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestLoadIgnoreScoped_NeitherFile_NilMatcher(t *testing.T) {
	withHome(t)
	root := t.TempDir()
	m, counts, err := connector.LoadIgnoreScoped(root)
	if err != nil {
		t.Fatal(err)
	}
	if m != nil {
		t.Errorf("expected nil matcher, got %v", m)
	}
	if counts.Project != 0 || counts.User != 0 {
		t.Errorf("counts = %+v, want zero", counts)
	}
}

func TestLoadIgnoreScoped_UnionsProjectAndUser(t *testing.T) {
	home := withHome(t)
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, ".fathomignore"), []byte("vendor/\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	writeUserIgnore(t, home, ".DS_Store\n*.wasm\n")

	m, counts, err := connector.LoadIgnoreScoped(root)
	if err != nil {
		t.Fatal(err)
	}
	if counts.Project != 1 || counts.User != 2 {
		t.Fatalf("counts = %+v, want {Project:1 User:2}", counts)
	}
	if !m.Match("vendor/foo.go") {
		t.Error("expected project pattern to match")
	}
	if !m.Match(".DS_Store") {
		t.Error("expected user pattern to match")
	}
	if !m.Match("gen/blob.wasm") {
		t.Error("expected user glob pattern to match")
	}
}

func TestLoadIgnoreScoped_UserReinclude_RefusedLoudly(t *testing.T) {
	home := withHome(t)
	root := t.TempDir()
	writeUserIgnore(t, home, "*.log\n!keep-this.log\n")

	_, _, err := connector.LoadIgnoreScoped(root)
	if err == nil {
		t.Fatal("expected an error for a user-scope re-include pattern")
	}
	var reinclude *connector.ErrUserScopeReinclude
	if !errors.As(err, &reinclude) {
		t.Fatalf("err = %v (%T), want *ErrUserScopeReinclude", err, err)
	}
	if reinclude.Pattern != "!keep-this.log" || reinclude.Line != 2 {
		t.Errorf("reinclude = %+v", reinclude)
	}
}

func TestLoadIgnoreScoped_ProjectReinclude_StillAllowed(t *testing.T) {
	home := withHome(t)
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, ".fathomignore"), []byte("*.go\n!main.go\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	// no user-scope file at all
	_ = home

	m, _, err := connector.LoadIgnoreScoped(root)
	if err != nil {
		t.Fatalf("project-scope re-include must remain legal: %v", err)
	}
	if m.Match("main.go") {
		t.Error("project's own negation must still re-include main.go")
	}
	if !m.Match("other.go") {
		t.Error("expected other.go to remain ignored")
	}
}

// TestLoadIgnoreScoped_ProjectWinsOnConflict proves law 13: when a project
// re-includes something a user-scope pattern would otherwise exclude, the
// project's own choice wins (patterns are unioned user-then-project, and
// filter.Matcher is last-match-wins).
func TestLoadIgnoreScoped_ProjectWinsOnConflict(t *testing.T) {
	home := withHome(t)
	writeUserIgnore(t, home, "*.log\n")
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, ".fathomignore"), []byte("!important.log\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	m, _, err := connector.LoadIgnoreScoped(root)
	if err != nil {
		t.Fatal(err)
	}
	if m.Match("important.log") {
		t.Error("project scope must win over a user-scope default on conflict (law 13)")
	}
	if !m.Match("other.log") {
		t.Error("expected other.log to remain excluded by the user-scope default")
	}
}

// TestLocalConnector_Discover_UserReinclude_RefusesIngest proves the loud
// enforcement point: Discover (ingest's own discovery pass) fails outright
// on a violating user-scope file rather than silently indexing anyway.
func TestLocalConnector_Discover_UserReinclude_RefusesIngest(t *testing.T) {
	home := withHome(t)
	writeUserIgnore(t, home, "!widen-everything\n")
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "main.go"), []byte("package main\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	c := connector.NewLocalConnector()
	_, err := c.Discover(context.Background(), core.ServiceRef{ID: "svc", Path: root})
	if err == nil {
		t.Fatal("expected Discover to refuse a violating user-scope ignore file")
	}
	var reinclude *connector.ErrUserScopeReinclude
	if !errors.As(err, &reinclude) {
		t.Fatalf("err = %v (%T), want *ErrUserScopeReinclude", err, err)
	}
}

// TestLoadIgnore_DegradesGracefullyOnUserViolation proves the read-time
// convenience (internal/mcp's read_source) never breaks on a violating
// user-scope file — it falls back to project-scope only rather than
// erroring a hot serving path.
func TestLoadIgnore_DegradesGracefullyOnUserViolation(t *testing.T) {
	home := withHome(t)
	writeUserIgnore(t, home, "!oops\n")
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, ".fathomignore"), []byte("vendor/\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	m := connector.LoadIgnore(root)
	if m == nil {
		t.Fatal("expected the project-scope matcher to still be returned")
	}
	if !m.Match("vendor/foo.go") {
		t.Error("expected project pattern to still apply on graceful degrade")
	}
}

func TestUserIgnorePath_UnresolvableHome(t *testing.T) {
	t.Setenv("HOME", "")
	// On most platforms os.UserHomeDir falls back to other env vars; this
	// test only asserts UserIgnorePath never panics and returns a sane
	// value either way.
	_ = connector.UserIgnorePath()
}
