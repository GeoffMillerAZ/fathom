package main

import (
	"os"
	"path/filepath"
	"regexp"
	"testing"
)

// releasedBlockRe matches a released CHANGELOG heading, `## [x.y.z] -- date`;
// `## [Unreleased]` never matches.
var releasedBlockRe = regexp.MustCompile(`(?m)^## \[(\d+\.\d+\.\d+)\]`)

// TestVersionMatchesNewestReleasedChangelogBlock: the release ritual renames
// [Unreleased] to a versioned block and bumps Version in the same commit
// (changes.md section 6; release 0.9.0 did both). A binary that prints an
// older version than the newest released block reads as stale to `fathom
// version`, `daemon install --reconcile` and the research bundle.
func TestVersionMatchesNewestReleasedChangelogBlock(t *testing.T) {
	root := repoRootForDocs(t)
	data, err := os.ReadFile(filepath.Join(root, "CHANGELOG.md"))
	if err != nil {
		t.Fatal(err)
	}
	m := releasedBlockRe.FindSubmatch(data)
	if m == nil {
		t.Fatal("CHANGELOG.md has no released `## [x.y.z]` block")
	}
	if want := "fathom v" + string(m[1]); Version != want {
		t.Errorf("Version = %q, newest released CHANGELOG block wants %q: bump cmd/fathom/version.go in the release commit", Version, want)
	}
}
