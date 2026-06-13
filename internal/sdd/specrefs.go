// Package sdd — deterministic spec-reference extraction across source files.
//
// ExtractFileRefs applies a regex pattern to a map of path→content and records
// every match with its file and 1-based line number, normalised to a canonical
// spec_id form:
//
//   - Bracketed IDs: "[US-042]" → "US-042"
//   - Milestone IDs: "m18.2" or "M18.2" → "M18.2"
//   - Doc references: "doc  31" → "doc 31"
//
// The default pattern covers all three families:
//
//	\[(?:US|EP|FR|NFR)-\d+\]|\b[Mm]\d+\.\d+\b|\bdoc\s+\d+\b
package sdd

import (
	"regexp"
	"sort"
	"strings"
)

// DefaultSpecRefPattern is the default regex applied by ExtractFileRefs when
// the caller passes "". It matches:
//
//   - Bracketed spec IDs:   [US-042], [EP-013], [FR-7], [NFR-1]
//   - Milestone IDs:        M18.2, m17.4, (M17.4)
//   - Doc references:       doc 31, doc  31
const DefaultSpecRefPattern = `\[(?:US|EP|FR|NFR)-\d+\]|\b[Mm]\d+\.\d+\b|\bdoc\s+\d+\b`

// SpecRef is a single match of a spec ID inside a source file.
type SpecRef struct {
	SpecID string // normalised spec ID, e.g. "US-042", "M18.2", "doc 31"
	File   string // path as supplied by the caller
	Line   int    // 1-based line number
}

// ExtractFileRefs scans files (path→content) for spec-ID references using
// pattern. When pattern is "" the DefaultSpecRefPattern is used. If spec is
// non-empty, only refs whose normalised SpecID equals spec are returned.
//
// The result is sorted by (SpecID asc, File asc, Line asc) and is always
// non-nil (may be empty).
func ExtractFileRefs(files map[string]string, pattern string, specFilter string) ([]SpecRef, int, error) {
	if pattern == "" {
		pattern = DefaultSpecRefPattern
	}
	re, err := regexp.Compile(pattern)
	if err != nil {
		return nil, 0, err
	}

	filesScanned := len(files)
	// Collect results deterministically: iterate files in sorted order.
	paths := make([]string, 0, len(files))
	for p := range files {
		paths = append(paths, p)
	}
	sort.Strings(paths)

	refs := make([]SpecRef, 0)
	for _, path := range paths {
		content := files[path]
		lines := strings.SplitAfter(content, "\n")
		for i, line := range lines {
			matches := re.FindAllString(line, -1)
			for _, m := range matches {
				id := normaliseSpecID(m)
				if specFilter != "" && id != specFilter {
					continue
				}
				refs = append(refs, SpecRef{
					SpecID: id,
					File:   path,
					Line:   i + 1,
				})
			}
		}
	}

	// Sort: SpecID asc, File asc, Line asc.
	sort.Slice(refs, func(i, j int) bool {
		a, b := refs[i], refs[j]
		if a.SpecID != b.SpecID {
			return a.SpecID < b.SpecID
		}
		if a.File != b.File {
			return a.File < b.File
		}
		return a.Line < b.Line
	})

	return refs, filesScanned, nil
}

// ExtractRefsFromText scans a single text string for spec-ID references using
// pattern (DefaultSpecRefPattern when ""). Returns normalised spec IDs,
// deduplicated and sorted. Returns an error only for invalid pattern.
func ExtractRefsFromText(text, pattern string) ([]string, error) {
	refs, _, err := ExtractFileRefs(map[string]string{"<text>": text}, pattern, "")
	if err != nil {
		return nil, err
	}
	seen := make(map[string]bool)
	var out []string
	for _, r := range refs {
		if !seen[r.SpecID] {
			seen[r.SpecID] = true
			out = append(out, r.SpecID)
		}
	}
	sort.Strings(out)
	return out, nil
}

// normaliseSpecID converts a raw regex match into a canonical spec ID string.
//
//   - "[US-042]" → "US-042"        (strip brackets)
//   - "m18.2"   → "M18.2"         (uppercase M)
//   - "(M17.4)" → "M17.4"         (already uppercase; parentheses aren't in match)
//   - "doc  31" → "doc 31"         (collapse internal whitespace)
func normaliseSpecID(raw string) string {
	// Strip surrounding brackets.
	raw = strings.TrimLeft(raw, "[")
	raw = strings.TrimRight(raw, "]")

	// Collapse internal whitespace (handles "doc  31" → "doc 31").
	rawFields := strings.Fields(raw)
	raw = strings.Join(rawFields, " ")

	// Uppercase leading 'm' → 'M' for milestone IDs.
	if len(raw) > 0 && raw[0] == 'm' {
		raw = "M" + raw[1:]
	}

	return raw
}
