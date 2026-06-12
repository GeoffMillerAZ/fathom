// Package filter provides deterministic ignore-pattern matching for file paths.
// It supports ** globs, directory prefixes, extension globs, and negation (!)
// patterns — the same semantics found in .gitignore and common CI tooling.
//
// Patterns are evaluated in order. The last matching pattern wins. A negation
// pattern ("!foo") re-includes a previously excluded path.
package filter

import (
	"path"
	"strings"
)

// Matcher holds a compiled set of ignore patterns and evaluates paths against
// them in order (last-match-wins, gitignore semantics).
type Matcher struct {
	rules []rule
}

// rule is a compiled single pattern entry.
type rule struct {
	pattern          string // cleaned pattern (without leading !)
	negate           bool   // true => re-include
	dirOnly          bool   // true => pattern ends with / (match dirs only)
	isAbs            bool   // true => pattern contains a / (anchored to root)
	hasGlob          bool   // true => pattern contains * or ?
	ext              string // non-empty when pattern is purely "*.ext"
	doubleStarPrefix bool   // pattern starts with **/ → match anywhere
	doubleStarSuffix bool   // pattern ends with /** → everything under dir
}

// New compiles a slice of patterns into a Matcher.
// Blank lines and lines beginning with # are ignored.
// Leading and trailing whitespace is stripped.
func New(patterns []string) *Matcher {
	m := &Matcher{}
	for _, raw := range patterns {
		raw = strings.TrimSpace(raw)
		if raw == "" || strings.HasPrefix(raw, "#") {
			continue
		}
		m.rules = append(m.rules, compile(raw))
	}
	return m
}

// compile converts a raw pattern string into a rule.
func compile(raw string) rule {
	r := rule{}
	p := raw

	if strings.HasPrefix(p, "!") {
		r.negate = true
		p = p[1:]
	}

	if strings.HasSuffix(p, "/") {
		r.dirOnly = true
		p = strings.TrimSuffix(p, "/")
	}

	// Does the pattern contain a slash (other than a trailing one we just stripped)?
	// If so it is anchored (absolute).
	if strings.Contains(p, "/") {
		r.isAbs = true
	}

	// Detect **/ prefix
	if strings.HasPrefix(p, "**/") {
		r.doubleStarPrefix = true
		p = p[3:] // remove the **/ so matching compares the remainder
	}
	// Detect /** suffix
	if strings.HasSuffix(p, "/**") {
		r.doubleStarSuffix = true
		p = strings.TrimSuffix(p, "/**")
	}

	// Pure extension glob like "*.pb.go" or "*.go"
	if !r.isAbs && !r.doubleStarPrefix && !r.doubleStarSuffix {
		if strings.HasPrefix(p, "*.") && !strings.ContainsAny(p[2:], "*/") {
			r.ext = p[1:] // e.g. ".pb.go"
		}
	}

	r.hasGlob = strings.ContainsAny(p, "*?[")
	r.pattern = p
	return r
}

// Match reports whether path should be ignored.
// path must use forward slashes and must not be absolute.
// The last matching rule wins (gitignore semantics); a match on a negation
// rule returns false (path is re-included).
//
// Callers that know they are passing a directory should append "/" before
// calling Match so that dir-only rules ("ref/") can apply to it. For file
// paths, dir-only rules are evaluated against the file's parent directories,
// mirroring gitignore: "ref/" ignores ref/foo.md but not a file named "ref".
func (m *Matcher) Match(filePath string) bool {
	// Capture directory-ness before Clean strips the trailing slash.
	isDir := strings.HasSuffix(filePath, "/")
	// Normalise: no leading slash, forward slashes.
	filePath = strings.TrimPrefix(filePath, "/")
	filePath = path.Clean(filePath)

	ignored := false
	for _, r := range m.rules {
		target := filePath
		if r.dirOnly && !isDir {
			// Dir-only rule against a file path: match the parent directory
			// chain instead, so a file whose base name equals the pattern is
			// not caught.
			dir := path.Dir(filePath)
			if dir == "." {
				continue
			}
			target = dir
		}
		if matches(r, target) {
			ignored = !r.negate
		}
	}
	return ignored
}

// matches reports whether rule r matches filePath.
func matches(r rule, filePath string) bool {
	// Extension-only shortcut: "*.pb.go" matches any file ending in ".pb.go"
	if r.ext != "" {
		return strings.HasSuffix(filePath, r.ext)
	}

	// "**/" prefix: the sub-pattern must match any path component or suffix.
	if r.doubleStarPrefix {
		// The remainder pattern r.pattern is matched against each suffix of the path.
		return matchAnywhere(r.pattern, filePath)
	}

	// "/**" suffix: everything under the directory r.pattern
	if r.doubleStarSuffix {
		return filePath == r.pattern || strings.HasPrefix(filePath, r.pattern+"/")
	}

	// Anchored pattern (contains /) — match from the root.
	// Also matches paths under the pattern as a directory prefix.
	if r.isAbs {
		if globMatch(r.pattern, filePath) {
			return true
		}
		// If filePath is under pattern as a directory, match too.
		// e.g. pattern "internal/gen" matches "internal/gen/foo.go".
		return strings.HasPrefix(filePath, r.pattern+"/") ||
			globMatch(r.pattern+"/*", filePath)
	}

	// Unanchored, no glob: match against the base name OR any path component
	// (gitignore behaviour: "vendor" matches "vendor/foo" and "a/vendor/b").
	if !r.hasGlob {
		base := path.Base(filePath)
		if base == r.pattern {
			return true
		}
		// Also match as a directory component anywhere in the path.
		for _, seg := range strings.Split(filePath, "/") {
			if seg == r.pattern {
				return true
			}
		}
		return false
	}

	// Unanchored with glob: try matching against each component suffix.
	return matchAnywhere(r.pattern, filePath)
}

// matchAnywhere tries to match pattern against:
//  1. The full path (exact glob match).
//  2. Each slash-separated component of the path (so "*.go" matches
//     "internal/foo/bar.go" and "testdata" matches "a/testdata/b.go").
//  3. Each slash-separated suffix starting at a directory boundary.
//
// This mirrors gitignore unanchored matching semantics.
func matchAnywhere(pattern, filePath string) bool {
	if globMatch(pattern, filePath) {
		return true
	}
	parts := strings.Split(filePath, "/")
	for i := 0; i < len(parts); i++ {
		// Match individual component (handles bare names like "testdata").
		if globMatch(pattern, parts[i]) {
			return true
		}
		// Match suffix starting at this boundary.
		if i > 0 {
			if globMatch(pattern, strings.Join(parts[i:], "/")) {
				return true
			}
		}
	}
	return false
}

// globMatch wraps path.Match and returns false on error (malformed patterns
// are treated as non-matching rather than panicking).
func globMatch(pattern, name string) bool {
	ok, err := path.Match(pattern, name)
	return err == nil && ok
}

// DefaultIgnore returns a Matcher pre-loaded with common ignore patterns:
// vendor, node_modules, .git, testdata, *.pb.go, dist, build.
// These are the defaults used by sextant's file-walking pipeline.
func DefaultIgnore() *Matcher {
	return New([]string{
		"vendor",
		"node_modules",
		".git",
		"testdata",
		"*.pb.go",
		"dist",
		"build",
	})
}
