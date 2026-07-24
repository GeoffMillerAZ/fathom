package main

import (
	"os"
	"path/filepath"
	"testing"
)

// TestResolveFromServices pins the O15 fix: when a project directory's
// basename does NOT match any indexed service (the sextant-dir/fathom-service
// split that silently killed obligations injection for six days), the
// daemon's recorded corpus paths are the authoritative identity.
func TestResolveFromServices(t *testing.T) {
	root := t.TempDir()
	// The repo dir is named "sextant"; the indexed service is "fathom".
	repo := filepath.Join(root, "sextant")
	if err := os.MkdirAll(repo, 0o755); err != nil {
		t.Fatal(err)
	}

	t.Run("mismatched_basename_resolves_via_recorded_path", func(t *testing.T) {
		svcs := map[string]svcInfo{
			"fathom":      {symbols: 100, path: repo},
			"agentic-kit": {symbols: 50, path: filepath.Join(root, "elsewhere")},
		}
		cand, ok := resolveFromServices(svcs, repo)
		if !ok {
			t.Fatal("expected resolution; got none")
		}
		if cand.name != "fathom" || cand.root != repo {
			t.Errorf("resolved %q@%q, want fathom@%s", cand.name, cand.root, repo)
		}
	})

	t.Run("symlinked_path_still_matches", func(t *testing.T) {
		link := filepath.Join(root, "sextant-link")
		if err := os.Symlink(repo, link); err != nil {
			t.Skipf("symlink unavailable: %v", err)
		}
		svcs := map[string]svcInfo{"fathom": {path: link}}
		if cand, ok := resolveFromServices(svcs, repo); !ok || cand.name != "fathom" {
			t.Errorf("symlink-equivalent path did not resolve (ok=%v cand=%+v)", ok, cand)
		}
	})

	t.Run("no_match_falls_open", func(t *testing.T) {
		svcs := map[string]svcInfo{"other": {path: filepath.Join(root, "nope")}}
		if _, ok := resolveFromServices(svcs, repo); ok {
			t.Error("unrelated paths must not resolve")
		}
	})

	t.Run("empty_paths_ignored", func(t *testing.T) {
		svcs := map[string]svcInfo{"fathom": {path: ""}}
		if _, ok := resolveFromServices(svcs, repo); ok {
			t.Error("empty recorded path must not resolve")
		}
	})
}
