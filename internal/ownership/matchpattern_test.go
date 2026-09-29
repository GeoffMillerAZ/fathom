package ownership

import "testing"

// TestMatchPatternGitHubLeadingSlashAndAnyDepth pins IM-F1: CODEOWNERS
// patterns follow GitHub's semantics (gitignore anchoring). A leading slash
// anchors a pattern at the repository root; a pattern with no slash but a
// trailing one (apps/) or none at all (README.md, *.js) matches at any
// depth; a slash inside the pattern (docs/*, a/b.go) anchors it too; a
// trailing wildcard segment (docs/*) owns the files at that level only,
// while a directory name owns everything under it. Before this, a leading
// slash never matched (paths carry none) and docs/ matched only at the root.
func TestMatchPatternGitHubLeadingSlashAndAnyDepth(t *testing.T) {
	cases := []struct {
		pattern, path string
		want          bool
	}{
		// leading slash anchors at the root
		{"/services/checkout/", "services/checkout/main.go", true},
		{"/services/checkout/", "services/checkout/pkg/a.go", true},
		{"/services/checkout/", "x/services/checkout/main.go", false},
		{"/docs/", "docs/a.md", true},
		{"/docs/", "pkg/docs/a.md", false},
		{"/docs", "docs/a.md", true},
		{"/docs", "pkg/docs/a.md", false},
		{"/README.md", "README.md", true},
		{"/README.md", "docs/README.md", false},
		{"/*.js", "app.js", true},
		{"/*.js", "web/app.js", false},
		{"/docs/*", "docs/a.md", true},
		{"/docs/*", "docs/sub/a.md", false},
		{"/docs/**", "docs/sub/a.md", true},
		{"/build/logs/", "build/logs/a.txt", true},
		{"/build/logs/", "x/build/logs/a.txt", false},

		// no leading slash, no inner slash: any depth
		{"apps/", "apps/a.go", true},
		{"apps/", "a/b/apps/c.go", true},
		{"apps/", "a/myapps/c.go", false},
		{"apps/", "apps", false}, // a file named apps is not the directory
		{"README.md", "README.md", true},
		{"README.md", "docs/README.md", true},
		{"README.md", "README.mdx", false},
		{"*.js", "c.js", true},
		{"*.js", "a/b/c.js", true},
		{"*.js", "a/b/c.jsx", false},
		{"internal", "internal/x.go", true},
		{"internal", "a/internal/x.go", true},
		{"internal", "internals/x.go", false},
		{"*", "a/b/c.go", true},
		{"*", "c.go", true},

		// a slash inside the pattern anchors it
		{"docs/*", "docs/a.md", true},
		{"docs/*", "docs/sub/a.md", false},
		{"docs/*", "x/docs/a.md", false},
		{"docs/guide.md", "docs/guide.md", true},
		{"docs/guide.md", "x/docs/guide.md", false},
		{"internal/auth/", "internal/auth/token.go", true},
		{"internal/auth/", "x/internal/auth/token.go", false},
		{"internal/*.go", "internal/a.go", true},
		{"internal/*.go", "internal/sub/a.go", false},
		{"src/*/", "src/a/x.go", true},
		{"src/*/", "src/x.go", false},

		// double star
		{"docs/**", "docs/a.md", true},
		{"docs/**", "docs/adr/ADR-001.md", true},
		{"docs/**", "x/docs/a.md", false},
		{"**/logs", "logs/a.txt", true},
		{"**/logs", "x/y/logs/a.txt", true},
		{"**/logs/*.txt", "x/logs/a.txt", true},
		{"**/logs/*.txt", "x/logs/sub/a.txt", false},
		{"a/**/b.txt", "a/b.txt", true},
		{"a/**/b.txt", "a/x/y/b.txt", true},
		{"a/**/b.txt", "a/x/c.txt", false},
		{"**", "any/thing.go", true},
	}
	for _, c := range cases {
		if got := matchPattern(c.pattern, c.path); got != c.want {
			t.Errorf("matchPattern(%q, %q) = %v, want %v", c.pattern, c.path, got, c.want)
		}
	}
}
