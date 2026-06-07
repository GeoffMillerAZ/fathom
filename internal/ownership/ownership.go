// Package ownership parses GitHub-style CODEOWNERS files and resolves file
// ownership via standard CODEOWNERS semantics: the last matching rule wins.
//
// Supported patterns (subset of gitignore/CODEOWNERS glob syntax):
//   - Literal paths: "internal/auth/token.go"
//   - Extension globs: "*.go", "*.tf"
//   - Directory prefixes: "internal/" matches any path under internal/
//   - Double-star: "docs/**" matches any path under docs/ at any depth
//   - Catch-all: "*" matches any path
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

// Owners holds the parsed rules from a CODEOWNERS file.
// The zero value is not useful; always obtain via Parse.
type Owners struct {
	rules []rule // rules in file order; last match wins
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
	for scanner.Scan() {
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

// For returns the owners for the given file path according to CODEOWNERS
// semantics: the last rule that matches the path wins.
//
// Returns an empty (non-nil) slice when no rule matches.
func (o *Owners) For(filePath string) []string {
	// Normalise the path: remove leading "./" if present, use forward slashes.
	filePath = strings.TrimPrefix(filePath, "./")

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

// matchPattern reports whether filePath is matched by pattern using
// CODEOWNERS/gitignore glob semantics.
//
// Rules:
//  1. If pattern ends with "/" it is a directory prefix: match any path that
//     starts with (or equals) pattern.
//  2. If pattern contains "/" it is anchored at the repo root and matched
//     via standard glob (path.Match) after normalisation.  "**" is expanded to
//     match any sequence of path segments.
//  3. If pattern contains no "/" (e.g. "*.go") it is matched against the
//     basename of filePath AND against every path component using standard glob
//     — mirroring git's behaviour for patterns without a slash.
//  4. The special pattern "*" matches any path.
func matchPattern(pattern, filePath string) bool {
	// Case 4: bare "*" matches everything.
	if pattern == "*" {
		return true
	}

	// Case 1: directory prefix (pattern ends with "/").
	if strings.HasSuffix(pattern, "/") {
		return strings.HasPrefix(filePath, pattern) || filePath == strings.TrimSuffix(pattern, "/")
	}

	// Case 2: pattern contains "/" — anchored glob.
	if strings.Contains(pattern, "/") {
		return matchGlob(pattern, filePath)
	}

	// Case 3: no slash — match against any path component / basename.
	// First try matching the full path (allows "*.go" to match "cmd/main.go"
	// because CODEOWNERS without a slash is applied to every file in the repo).
	if ok, _ := path.Match(pattern, path.Base(filePath)); ok {
		return true
	}
	// Also try matching the full path so patterns like "internal" catch directories.
	if ok, _ := path.Match(pattern, filePath); ok {
		return true
	}
	return false
}

// matchGlob matches a glob pattern (which may contain "**") against filePath.
// It expands "**" by trying all possible segment counts.
func matchGlob(pattern, filePath string) bool {
	// Fast path: no "**" — use path.Match directly.
	if !strings.Contains(pattern, "**") {
		ok, _ := path.Match(pattern, filePath)
		return ok
	}

	// Split around "**" and try all possible replacements.
	// We replace "**" with each possible interstitial substring.
	// Strategy: replace each "**" token recursively.
	return matchDoublestar(pattern, filePath)
}

// matchDoublestar matches pattern (which may contain "**") against target.
// "**" matches zero or more path components (including none).
func matchDoublestar(pattern, target string) bool {
	// Find the first "**".
	idx := strings.Index(pattern, "**")
	if idx == -1 {
		ok, _ := path.Match(pattern, target)
		return ok
	}

	prefix := pattern[:idx]   // everything before "**"
	suffix := pattern[idx+2:] // everything after "**"

	// The prefix (without "**") must match a prefix of the target.
	// We try all possible split points in the target path.
	//
	// "docs/**" with target "docs/adr/ADR-001.md":
	//   prefix = "docs/", suffix = ""
	//   replacement for ** can be "adr/ADR-001.md", "ADR-001.md", etc.

	// Enumerate all positions in target where prefix ends.
	// Since prefix may itself contain globs, we need to try each segment boundary.
	segments := pathSegments(target)

	for split := 0; split <= len(segments); split++ {
		candidate := strings.Join(segments[:split], "/")
		fullPrefix := prefix + candidate
		// Match the prefix part via path.Match (it must consume exactly prefix).
		// Remove trailing slash from fullPrefix for matching if prefix ended with one.
		trimmedPrefix := strings.TrimRight(fullPrefix, "/")
		prefixTarget := strings.TrimRight(prefix[:max(0, len(prefix)-1)], "/")

		var prefixOK bool
		if prefixTarget == "" {
			// Empty prefix always matches.
			prefixOK = split == 0 || true
			// Actually we need: the prefix glob matches candidate.
			// prefix = "docs/" → the literal "docs" must match the first segment.
			if prefix == "" {
				prefixOK = true
			} else {
				_ = trimmedPrefix
				prefixOK = matchLiteralPrefix(prefix, segments, split)
			}
		} else {
			prefixOK = matchLiteralPrefix(prefix, segments, split)
		}

		if !prefixOK {
			continue
		}

		// Build the remaining target for the suffix match.
		remaining := strings.Join(segments[split:], "/")
		// suffix may start with "/" — normalize.
		sfx := strings.TrimLeft(suffix, "/")
		if sfx == "" {
			// "**" consumed everything; suffix is empty → match.
			return true
		}
		if remaining == "" && sfx != "" {
			continue
		}
		// Recursively match suffix against remaining (handles nested **).
		if matchDoublestar(sfx, remaining) {
			return true
		}
	}
	return false
}

// matchLiteralPrefix reports whether the first split segments of segments,
// joined by "/", match the glob prefix (which may end with "/").
func matchLiteralPrefix(prefix string, segments []string, split int) bool {
	if prefix == "" {
		return split == 0
	}
	// Build the candidate from segments[0:split].
	candidate := strings.Join(segments[:split], "/")
	// Normalise prefix: strip trailing "/".
	p := strings.TrimRight(prefix, "/")
	if p == "" {
		return split == 0
	}
	ok, _ := path.Match(p, candidate)
	return ok
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

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
