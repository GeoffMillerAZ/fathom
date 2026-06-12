package filter_test

import (
	"testing"

	"github.com/geoffmilleraz/sextant/internal/filter"
)

// ─── Match tests ─────────────────────────────────────────────────────────────

func TestMatch_SimpleDirectory(t *testing.T) {
	m := filter.New([]string{"vendor"})
	cases := []struct {
		path string
		want bool
	}{
		{"vendor/foo.go", true},
		{"vendor/sub/bar.go", true},
		{"internal/vendor/baz.go", true}, // gitignore: unanchored matches any component
		{"main.go", false},
		{"vendored.go", false}, // must be exact segment
		{"a/vendored/b", false},
	}
	for _, c := range cases {
		t.Run(c.path, func(t *testing.T) {
			if got := m.Match(c.path); got != c.want {
				t.Errorf("Match(%q) = %v, want %v", c.path, got, c.want)
			}
		})
	}
}

func TestMatch_ExtensionGlob(t *testing.T) {
	m := filter.New([]string{"*.pb.go"})
	cases := []struct {
		path string
		want bool
	}{
		{"foo.pb.go", true},
		{"internal/auth/token.pb.go", true},
		{"token.go", false},
		{"token.pb.go.bak", false},
	}
	for _, c := range cases {
		t.Run(c.path, func(t *testing.T) {
			if got := m.Match(c.path); got != c.want {
				t.Errorf("Match(%q) = %v, want %v", c.path, got, c.want)
			}
		})
	}
}

func TestMatch_DoubleStarPrefix(t *testing.T) {
	m := filter.New([]string{"**/testdata"})
	cases := []struct {
		path string
		want bool
	}{
		{"testdata/foo.go", true},
		{"internal/testdata/bar.go", true},
		{"a/b/testdata/c.go", true},
		{"notestdata/foo.go", false},
	}
	for _, c := range cases {
		t.Run(c.path, func(t *testing.T) {
			if got := m.Match(c.path); got != c.want {
				t.Errorf("Match(%q) = %v, want %v", c.path, got, c.want)
			}
		})
	}
}

func TestMatch_DoubleStarSuffix(t *testing.T) {
	m := filter.New([]string{"build/**"})
	cases := []struct {
		path string
		want bool
	}{
		{"build/out.bin", true},
		{"build/sub/a.go", true},
		{"build", true},
		{"notbuild/foo", false},
	}
	for _, c := range cases {
		t.Run(c.path, func(t *testing.T) {
			if got := m.Match(c.path); got != c.want {
				t.Errorf("Match(%q) = %v, want %v", c.path, got, c.want)
			}
		})
	}
}

func TestMatch_Negation(t *testing.T) {
	// Ignore all *.go but re-include main.go
	m := filter.New([]string{"*.go", "!main.go"})
	cases := []struct {
		path string
		want bool
	}{
		{"foo.go", true},
		{"main.go", false},     // negated
		{"main.go.bak", false}, // *.go does not match .bak suffix via path.Match
		{"foo.txt", false},
	}
	for _, c := range cases {
		t.Run(c.path, func(t *testing.T) {
			if got := m.Match(c.path); got != c.want {
				t.Errorf("Match(%q) = %v, want %v", c.path, got, c.want)
			}
		})
	}
}

func TestMatch_AnchoredPattern(t *testing.T) {
	// Pattern with a slash is anchored to the root.
	m := filter.New([]string{"internal/gen"})
	cases := []struct {
		path string
		want bool
	}{
		{"internal/gen/foo.go", true},
		{"internal/gen", true},
		{"a/internal/gen/foo.go", false}, // anchored: no match deep in tree
		{"internal/generator/foo.go", false},
	}
	for _, c := range cases {
		t.Run(c.path, func(t *testing.T) {
			if got := m.Match(c.path); got != c.want {
				t.Errorf("Match(%q) = %v, want %v", c.path, got, c.want)
			}
		})
	}
}

func TestMatch_Blank_And_Comment_Lines(t *testing.T) {
	// Blank lines and # comments must be ignored.
	m := filter.New([]string{
		"",
		"# this is a comment",
		"vendor",
	})
	if !m.Match("vendor/foo.go") {
		t.Error("expected vendor/foo.go to be matched")
	}
	if m.Match("main.go") {
		t.Error("expected main.go not to be matched")
	}
}

func TestMatch_Determinism(t *testing.T) {
	m := filter.New([]string{"vendor", "*.pb.go", "node_modules"})
	paths := []string{
		"vendor/a.go", "b.pb.go", "node_modules/x/y.js",
		"internal/foo.go", "main.go",
	}
	// Run twice, results must be identical.
	results1 := make([]bool, len(paths))
	results2 := make([]bool, len(paths))
	for i, p := range paths {
		results1[i] = m.Match(p)
		results2[i] = m.Match(p)
	}
	for i, p := range paths {
		if results1[i] != results2[i] {
			t.Errorf("non-deterministic result for %q: %v vs %v", p, results1[i], results2[i])
		}
	}
}

// ─── DefaultIgnore tests ─────────────────────────────────────────────────────

func TestDefaultIgnore(t *testing.T) {
	m := filter.DefaultIgnore()
	shouldIgnore := []string{
		"vendor/pkg/foo.go",
		"node_modules/react/index.js",
		".git/HEAD",
		"testdata/fixture.go",
		"internal/gen/token.pb.go",
		"dist/bundle.js",
		"build/output.bin",
	}
	shouldPass := []string{
		"main.go",
		"internal/auth/token.go",
		"cmd/sextant/main.go",
		"pkg/client/client.go",
	}
	for _, p := range shouldIgnore {
		if !m.Match(p) {
			t.Errorf("DefaultIgnore: expected %q to be ignored", p)
		}
	}
	for _, p := range shouldPass {
		if m.Match(p) {
			t.Errorf("DefaultIgnore: expected %q NOT to be ignored", p)
		}
	}
}

// ─── New with empty/nil slice ────────────────────────────────────────────────

func TestNew_Empty(t *testing.T) {
	m := filter.New(nil)
	if m.Match("anything/at/all.go") {
		t.Error("empty matcher should not ignore anything")
	}
}

// ─── Leading slash normalisation ─────────────────────────────────────────────

func TestMatch_LeadingSlashNormalised(t *testing.T) {
	m := filter.New([]string{"vendor"})
	// Paths with a leading slash should behave the same as without.
	if !m.Match("/vendor/foo.go") {
		t.Error("Match should normalise away leading slash")
	}
}

// ─── Dir-only pattern tests ──────────────────────────────────────────────────

func TestMatch_DirOnly_DirectoryPath(t *testing.T) {
	m := filter.New([]string{"ref/"})
	cases := []struct {
		path string
		want bool
	}{
		{"ref/", true},       // the directory itself (trailing slash = dir)
		{"ref/foo.md", true}, // file under the dir
		{"ref/sub/bar.go", true},
		{"ref", false},         // plain file named "ref" at root
		{"a/ref", false},       // plain file named "ref" in a subdir
		{"refs/foo.md", false}, // different directory
	}
	for _, c := range cases {
		t.Run(c.path, func(t *testing.T) {
			if got := m.Match(c.path); got != c.want {
				t.Errorf("Match(%q) = %v, want %v", c.path, got, c.want)
			}
		})
	}
}

func TestMatch_DirOnly_AnchoredAndNested(t *testing.T) {
	m := filter.New([]string{"internal/gen/", "tmp/"})
	cases := []struct {
		path string
		want bool
	}{
		{"internal/gen/", true},
		{"internal/gen/x.go", true},
		{"internal/genuine.go", false},
		{"tmp/scratch.md", true},
		{"a/tmp/scratch.md", true}, // unanchored dir-only matches any component
		{"tmp", false},             // file named tmp
	}
	for _, c := range cases {
		t.Run(c.path, func(t *testing.T) {
			if got := m.Match(c.path); got != c.want {
				t.Errorf("Match(%q) = %v, want %v", c.path, got, c.want)
			}
		})
	}
}

func TestMatch_DirOnly_Negation(t *testing.T) {
	m := filter.New([]string{"docs/", "!docs/keep/"})
	if !m.Match("docs/old.md") {
		t.Error("docs/old.md should be ignored")
	}
	if m.Match("docs/keep/important.md") {
		t.Error("docs/keep/important.md should be re-included by negation")
	}
}
