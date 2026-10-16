package buildinfo

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

func TestVersion(t *testing.T) {
	v := Version()
	if v == "" {
		t.Fatal("Version must never be empty — 'unknown' is the honest floor (law 3)")
	}
	if v2 := Version(); v2 != v {
		t.Errorf("Version not stable: %q then %q", v, v2)
	}
}

func TestReadIsStableAndConsistentWithVersion(t *testing.T) {
	s := Read()
	if s2 := Read(); s2 != s {
		t.Errorf("Read not stable: %+v then %+v", s, s2)
	}
	v := Version()
	if s.Revision == "" && v != "unknown" {
		t.Errorf("no revision stamped but Version() = %q, want unknown", v)
	}
	if s.Revision != "" && !SameRevision(v, s.Revision) {
		t.Errorf("Version() %q does not name Read().Revision %q", v, s.Revision)
	}
}

func TestSameRevision(t *testing.T) {
	cases := []struct {
		a, b string
		want bool
	}{
		{"", "", false},
		{"abc", "", false},
		{"6c6f4377b53a", "6c6f4377b53aad47cf6ab86224721df8592cadf0", true},
		{"6c6f4377b53a-dirty", "6c6f4377b53aad47cf6ab86224721df8592cadf0", true},
		{"3f0c3b1", "6c6f4377b53a", false},
		{"6c6f4377b53a", "6c6f4377b53a", true},
	}
	for _, c := range cases {
		if got := SameRevision(c.a, c.b); got != c.want {
			t.Errorf("SameRevision(%q,%q) = %v, want %v", c.a, c.b, got, c.want)
		}
	}
}

// repoFile reads a repo-root file from this package's directory.
func repoFile(t *testing.T, rel string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("..", "..", rel))
	if err != nil {
		t.Fatalf("read %s: %v", rel, err)
	}
	return string(b)
}

var changelogHeading = regexp.MustCompile(`(?m)^## \[(\d+\.\d+\.\d+)\]`)

// TestReleaseMatchesNewestChangelogHeading: a release that forgets to bump
// Release would ship a wrong `fathom version` and MCP serverInfo.version.
// [Unreleased] is skipped; the first versioned heading is the newest.
func TestReleaseMatchesNewestChangelogHeading(t *testing.T) {
	m := changelogHeading.FindStringSubmatch(repoFile(t, "CHANGELOG.md"))
	if m == nil {
		t.Fatal("CHANGELOG.md has no released ## [x.y.z] heading")
	}
	if want := "v" + m[1]; Release != want {
		t.Errorf("buildinfo.Release = %q, newest CHANGELOG release is %q: bump the constant with the roll", Release, want)
	}
}

// TestInstallDocsCarryCurrentRelease: the sample `fathom version` output in
// the install docs names the release; a stale one fails here.
func TestInstallDocsCarryCurrentRelease(t *testing.T) {
	want := "fathom " + Release + " @"
	for _, f := range []string{"README.md", "docs/early-testers.md"} {
		if !strings.Contains(repoFile(t, f), want) {
			t.Errorf("%s: sample `fathom version` output does not contain %q", f, want)
		}
	}
}
