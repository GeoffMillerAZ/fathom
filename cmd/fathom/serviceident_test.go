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

// TestResolveFromServicesDeterministic pins the O1 fix: when two indexed
// services record the SAME corpus root (the duplicate-corpus-root class that
// silently zeroed obligations on 36% of edits), resolution must be
// deterministic — richer service first, then basename affinity, then
// lexicographic — never Go map iteration order.
func TestResolveFromServicesDeterministic(t *testing.T) {
	root := t.TempDir()
	repo := filepath.Join(root, "sextant")
	if err := os.MkdirAll(repo, 0o755); err != nil {
		t.Fatal(err)
	}

	t.Run("richer_service_wins_every_time", func(t *testing.T) {
		svcs := map[string]svcInfo{
			"sextant": {symbols: 0, path: repo},    // the stale shadow
			"fathom":  {symbols: 6956, path: repo}, // the real service
		}
		for i := 0; i < 200; i++ {
			cand, ok := resolveFromServices(svcs, repo)
			if !ok || cand.name != "fathom" {
				t.Fatalf("iteration %d: resolved %q ok=%v, want fathom every time", i, cand.name, ok)
			}
		}
	})

	t.Run("symbol_tie_breaks_to_basename_affinity", func(t *testing.T) {
		svcs := map[string]svcInfo{
			"zzz-twin": {symbols: 10, path: repo},
			"sextant":  {symbols: 10, path: repo}, // matches the dir basename
		}
		for i := 0; i < 200; i++ {
			cand, ok := resolveFromServices(svcs, repo)
			if !ok || cand.name != "sextant" {
				t.Fatalf("iteration %d: resolved %q ok=%v, want basename-affine sextant", i, cand.name, ok)
			}
		}
	})

	t.Run("full_tie_breaks_lexicographically", func(t *testing.T) {
		svcs := map[string]svcInfo{
			"bbb": {symbols: 10, path: repo},
			"aaa": {symbols: 10, path: repo},
		}
		for i := 0; i < 200; i++ {
			cand, ok := resolveFromServices(svcs, repo)
			if !ok || cand.name != "aaa" {
				t.Fatalf("iteration %d: resolved %q ok=%v, want lexicographic aaa", i, cand.name, ok)
			}
		}
	})
}

// TestMergeCandidatesNeverCollapses pins O1 fix (c): the daemon-resolved
// identity is PREPENDED and exact duplicates dropped, but dedup must never
// shrink the fallback list — a name-equal candidate with a DIFFERENT root
// survives, so a wrong daemon answer degrades instead of totalling the list.
func TestMergeCandidatesNeverCollapses(t *testing.T) {
	resolved := serviceCandidate{name: "sextant", root: "/idx/sextant-shadow"}
	base := []serviceCandidate{
		{name: "sextant", root: "/real/sextant"},
		{name: "fathom", root: "/real/fathom"},
	}
	out := mergeCandidates(resolved, base)
	if len(out) != 3 {
		t.Fatalf("got %d candidates %v, want 3 (resolved + both base entries)", len(out), out)
	}
	if out[0] != resolved {
		t.Errorf("resolved candidate must be first, got %v", out[0])
	}

	t.Run("exact_duplicate_is_dropped", func(t *testing.T) {
		dup := serviceCandidate{name: "fathom", root: "/real/fathom"}
		out := mergeCandidates(dup, base)
		if len(out) != 2 {
			t.Fatalf("got %d candidates %v, want 2 (exact dup dropped)", len(out), out)
		}
	})
}
