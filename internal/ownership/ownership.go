// Package ownership parses GitHub-style CODEOWNERS files and resolves file
// ownership via standard CODEOWNERS semantics: the last matching rule wins.
//
// Patterns follow GitHub's CODEOWNERS semantics, which are gitignore's
// anchoring rules:
//   - A leading slash anchors the pattern at the repository root:
//     "/docs/" is the root docs directory and nothing nested.
//   - A slash inside the pattern anchors it too: "docs/*" and
//     "internal/auth/token.go" match from the root only.
//   - A pattern with no slash, or only a trailing one, matches at any depth:
//     "*.go", "README.md", "apps/" (a directory named apps anywhere).
//   - A trailing slash (or a literal name) owns everything under that
//     directory; a trailing wildcard segment ("docs/*") owns that level only.
//   - "**" matches any run of directories: "docs/**", "a/**/b.txt".
//   - "*" alone matches every path.
//
// Syntax GitHub does not honor (a "!" negation, "[...]" character ranges,
// backslash escapes) and GitLab's "[Section]" headers is not approximated: the
// line is skipped and reported by Unsupported with its line number.
//
// Inline comments (# …) are stripped from rule lines.
// Full-line comments and blank lines are skipped.
package ownership

import (
	"bufio"
	"bytes"
	"path"
	"sort"
	"strings"
)

// rule is a single parsed CODEOWNERS entry.
type rule struct {
	pattern string   // original pattern token (after stripping inline comments)
	owners  []string // e.g. ["@alice", "@bob"]
}

// Unsupported is a CODEOWNERS line whose syntax this package does not
// evaluate. It matches nothing; it is named rather than approximated.
type Unsupported struct {
	Line   int    // 1-based line in the CODEOWNERS file
	Text   string // the line, trimmed, inline comment removed
	Reason string // what is not supported
}

// Owners holds the parsed rules from a CODEOWNERS file.
// The zero value is not useful; always obtain via Parse.
type Owners struct {
	rules       []rule // rules in file order; last match wins
	unsupported []Unsupported
}

// Parse parses a CODEOWNERS file from content.
//
// Lines beginning with # (after trimming whitespace) are treated as comments.
// Blank lines are skipped.  Inline comments (text from # onward, after owners)
// are stripped from rule lines.
//
// Parse never returns a non-nil error for syntactically valid CODEOWNERS files;
// the error return is reserved for future strict-mode use.
func Parse(content []byte) (*Owners, error) {
	o := &Owners{}
	if len(content) == 0 {
		return o, nil
	}

	scanner := bufio.NewScanner(bytes.NewReader(content))
	lineNo := 0
	for scanner.Scan() {
		lineNo++
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		// Strip inline comment.  A '#' that appears after at least one
		// space separating it from the preceding token is treated as a
		// comment delimiter.
		if idx := strings.Index(line, " #"); idx >= 0 {
			line = strings.TrimSpace(line[:idx])
		}
		if line == "" {
			continue
		}

		fields := strings.Fields(line)
		if len(fields) == 0 {
			continue
		}
		if reason := unsupportedReason(fields[0]); reason != "" {
			o.unsupported = append(o.unsupported, Unsupported{Line: lineNo, Text: line, Reason: reason})
			continue
		}
		r := rule{
			pattern: fields[0],
		}
		if len(fields) > 1 {
			r.owners = fields[1:]
		}
		o.rules = append(o.rules, r)
	}

	return o, nil
}

// Unsupported returns the lines Parse skipped because their syntax is not
// evaluated (negation, character ranges, backslash escapes, GitLab section
// headers), in file order. The slice is a copy.
func (o *Owners) Unsupported() []Unsupported {
	return append([]Unsupported(nil), o.unsupported...)
}

// unsupportedReason names why the first token of a rule line is not a
// pattern this package evaluates, or returns "" when it is.
func unsupportedReason(token string) string {
	switch {
	case strings.HasPrefix(token, "[") || strings.HasPrefix(token, "^["):
		return "GitLab section header"
	case strings.HasPrefix(token, "!"):
		return "negation (!) is not supported by CODEOWNERS"
	case strings.Contains(token, "\\"):
		return "backslash escapes are not supported by CODEOWNERS"
	case strings.Contains(token, "["):
		return "character ranges ([...]) are not supported by CODEOWNERS"
	}
	return ""
}

// For returns the owners for the given file path according to CODEOWNERS
// semantics: the last rule that matches the path wins.
//
// Returns an empty (non-nil) slice when no rule matches.
func (o *Owners) For(filePath string) []string {
	// Normalise the path: remove leading "./" or "/" if present, use forward slashes.
	filePath = strings.TrimPrefix(strings.TrimPrefix(filePath, "./"), "/")

	var matched []string
	for _, r := range o.rules {
		if matchPattern(r.pattern, filePath) {
			matched = r.owners
		}
	}
	if matched == nil {
		return []string{}
	}
	// Return a copy so callers cannot mutate internal state.
	out := make([]string, len(matched))
	copy(out, matched)
	return out
}

// Rollup returns a map from owner to the number of rules in which that owner
// appears.  The map is freshly allocated on each call (safe to mutate).
//
// The map itself is a Go map; key ordering is non-deterministic in a range loop,
// but the counts are deterministic.  Callers that need a stable iteration order
// should sort the keys themselves.
func (o *Owners) Rollup() map[string]int {
	result := make(map[string]int)
	for _, r := range o.rules {
		// Use a local set so a repeated owner in one rule is counted once per rule.
		seen := make(map[string]struct{}, len(r.owners))
		for _, owner := range r.owners {
			if _, dup := seen[owner]; dup {
				continue
			}
			seen[owner] = struct{}{}
			result[owner]++
		}
	}
	return result
}

// SortedOwners returns the owners from Rollup sorted lexicographically.
// This is a convenience helper for callers that need stable output.
func (o *Owners) SortedOwners() []string {
	m := o.Rollup()
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// ─── pattern matching ─────────────────────────────────────────────────────────

// matchPattern reports whether filePath (repo-relative, slash-separated, a
// file) is matched by pattern under GitHub's CODEOWNERS semantics; see the
// package comment for the rules.
//
// The pattern is split into segments, each matched with path.Match; "**" is
// a segment of its own that spans any number of directories. An unanchored
// pattern (no leading or inner slash) is the same pattern behind "**/", so
// it matches at any depth. A pattern that is spent while path segments
// remain matched a directory, and owns what is under it — unless its last
// segment is a wildcard and it has no trailing slash ("docs/*"), which owns
// that level's files only, as GitHub documents.
func matchPattern(pattern, filePath string) bool {
	segs, dirOnly := compilePattern(pattern)
	return matchSegments(segs, pathSegments(filePath), dirOnly, cascades(segs, dirOnly))
}

// compilePattern returns pattern's segments (anchored patterns as written,
// unanchored ones behind a leading "**") and whether it named a directory
// by a trailing slash.
func compilePattern(pattern string) (segs []string, dirOnly bool) {
	dirOnly = strings.HasSuffix(pattern, "/")
	body := strings.Trim(pattern, "/")
	anchored := strings.HasPrefix(pattern, "/") || strings.Contains(body, "/")
	for _, s := range strings.Split(body, "/") {
		if s == "" || (s == "**" && len(segs) > 0 && segs[len(segs)-1] == "**") {
			continue
		}
		segs = append(segs, s)
	}
	if !anchored && len(segs) > 0 {
		segs = append([]string{"**"}, segs...)
	}
	return segs, dirOnly
}

// cascades reports whether a pattern that matched a directory owns the
// files under it: a trailing slash or a literal last segment does, a
// wildcard last segment does not.
func cascades(segs []string, dirOnly bool) bool {
	if dirOnly || len(segs) == 0 {
		return true
	}
	return !strings.ContainsAny(segs[len(segs)-1], "*?")
}

// matchSegments matches the pattern segments ps against the path segments
// xs, the pattern anchored at xs[0].
func matchSegments(ps, xs []string, dirOnly, cascade bool) bool {
	if len(ps) == 0 {
		if len(xs) > 0 {
			return cascade // the pattern named a directory above the file
		}
		return !dirOnly // the pattern named the file itself
	}
	if ps[0] == "**" {
		if len(ps) == 1 {
			return len(xs) > 0 // "docs/**": everything inside
		}
		for i := 0; i <= len(xs); i++ {
			if matchSegments(ps[1:], xs[i:], dirOnly, cascade) {
				return true
			}
		}
		return false
	}
	if len(xs) == 0 {
		return false
	}
	if ok, _ := path.Match(ps[0], xs[0]); !ok {
		return false
	}
	return matchSegments(ps[1:], xs[1:], dirOnly, cascade)
}

// pathSegments splits a slash-separated path into non-empty segments.
func pathSegments(p string) []string {
	parts := strings.Split(p, "/")
	out := make([]string, 0, len(parts))
	for _, s := range parts {
		if s != "" {
			out = append(out, s)
		}
	}
	return out
}
