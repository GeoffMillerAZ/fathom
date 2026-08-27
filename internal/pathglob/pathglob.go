// Package pathglob matches repo-relative paths against globs that may
// contain "**".
//
// EP-3. Go's path.Match has no "**" — it never crosses a separator — so
// every corner of this repo that had to match a glob like
// "internal/store/**" or "cmd/fathom/*hook*.go" grew its own matcher.
// internal/memory's anchor matcher lived here first and now delegates. Two
// other packages match paths and deliberately do NOT: internal/ownership
// speaks CODEOWNERS (directory rules, last match wins) and internal/filter
// speaks gitignore (leading-slash normalisation, dir-only patterns,
// negation). Those are different dialects, not copies of this one — folding
// them in would be a behaviour change dressed as cleanup. The router's
// sensitive-surface globs (ADR-026) are the second real consumer.
//
// Semantics, deliberately the memory package's (the most-exercised copy):
// an exact string equality always matches; a glob containing "**" splits at
// the first "**" and requires the literal prefix, then matches the suffix
// against either the remainder or its basename; a glob without "**" is
// plain path.Match. A malformed pattern matches nothing rather than
// erroring — a caller filtering paths wants a predicate, and path.Match's
// only error is ErrBadPattern.
package pathglob

import (
	"path"
	"strings"
)

// Match reports whether ref matches glob. Both are slash-separated and
// repo-relative; ref is never cleaned, so a caller comparing against
// stored anchors sees exactly what it stored.
func Match(glob, ref string) bool {
	if glob == ref {
		return true
	}
	if strings.Contains(glob, "**") {
		prefix, suffix, _ := strings.Cut(glob, "**")
		if !strings.HasPrefix(ref, prefix) {
			return false
		}
		rest := ref[len(prefix):]
		suffix = strings.TrimPrefix(suffix, "/")
		if suffix == "" {
			return true
		}
		if ok, _ := path.Match(suffix, path.Base(rest)); ok {
			return true
		}
		ok, _ := path.Match(suffix, rest)
		return ok
	}
	ok, _ := path.Match(glob, ref)
	return ok
}

// MatchAny reports whether ref matches any glob.
func MatchAny(globs []string, ref string) bool {
	for _, g := range globs {
		if Match(g, ref) {
			return true
		}
	}
	return false
}
