package langbench

// spec_owns_test.go — Manifest.Owns must agree with path globs, not only
// basenames and extensions (found alongside LB-Q-github-actions: a
// workflow file under .github/workflows/ is a path shape no basename or
// extension expresses, and lang-oracle's subject file-listing silently
// measured zero files for every such dialect until this fix; spec.go's
// Owns doc comment).

import "testing"

func TestManifestOwnsGlob(t *testing.T) {
	m := Manifest{Match: ManifestMatch{
		Basenames: []string{"action.yml", "action.yaml"},
		Globs:     []string{".github/workflows/*.yml", ".github/workflows/*.yaml"},
	}}
	cases := []struct {
		path string
		want bool
	}{
		{".github/workflows/ci.yml", true},
		{".github/workflows/release.yaml", true},
		{".github/actions/my-helper/action.yml", true},
		{"action.yaml", true},
		// A glob segment never crosses a directory boundary it did not name.
		{".github/workflows/nested/ci.yml", false},
		// Not under .github/workflows/ and not the action.yml/yaml basename.
		{"workflows/ci.yml", false},
		{"README.md", false},
	}
	for _, c := range cases {
		if got := m.Owns(c.path); got != c.want {
			t.Errorf("Owns(%q) = %v, want %v", c.path, got, c.want)
		}
	}
}

// TestManifestOwnsGlobAgreesWithNoGlobs: a manifest with no globs (every
// dialect before this fix) is unaffected — basename/extension behavior is
// unchanged.
func TestManifestOwnsGlobAgreesWithNoGlobs(t *testing.T) {
	m := Manifest{Match: ManifestMatch{Extensions: []string{".go"}, Basenames: []string{"go.mod"}}}
	if !m.Owns("main.go") {
		t.Error("Owns(main.go): want true (extension)")
	}
	if !m.Owns("go.mod") {
		t.Error("Owns(go.mod): want true (basename)")
	}
	if m.Owns("main.py") {
		t.Error("Owns(main.py): want false")
	}
}
