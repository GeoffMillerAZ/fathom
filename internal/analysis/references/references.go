// Package references implements fleet-wide occurrence search across source
// files.  It operates entirely on pre-fetched []core.SourceFile slices — no
// disk access — and is therefore pure and deterministic.
package references

import (
	"fmt"
	"regexp"
	"sort"
	"strings"

	"github.com/geoffmilleraz/sextant/internal/core"
)

// maxTextLen is the maximum number of bytes kept from a matched line.
const maxTextLen = 200

// Occurrence is a single matching line within a source file.
type Occurrence struct {
	File string // repo-relative file path
	Line int    // 1-based line number
	Text string // trimmed matching line, capped to maxTextLen chars
}

// Options controls how query matching is performed.
type Options struct {
	Word            bool // whole-word match (overrides plain substring when Regex==false)
	Regex           bool // treat query as a regular expression
	CaseInsensitive bool // apply case-insensitive matching to all modes
}

// Find scans files line-by-line and returns all lines that match query
// according to opts.  Lines are 1-based.  Results are sorted deterministically
// by (File asc, Line asc).
//
// Returns a non-nil error only when opts.Regex is true and query is an invalid
// regular expression.
func Find(query string, files []core.SourceFile, opts Options) ([]Occurrence, error) {
	if query == "" {
		return nil, nil
	}

	// Build the matcher once before iterating files.
	match, err := buildMatcher(query, opts)
	if err != nil {
		return nil, err
	}

	var out []Occurrence
	for _, f := range files {
		lines := strings.Split(string(f.Content), "\n")
		for i, raw := range lines {
			if match(raw) {
				text := strings.TrimSpace(raw)
				if len(text) > maxTextLen {
					text = text[:maxTextLen]
				}
				out = append(out, Occurrence{
					File: f.Path,
					Line: i + 1,
					Text: text,
				})
			}
		}
	}

	sort.Slice(out, func(i, j int) bool {
		if out[i].File != out[j].File {
			return out[i].File < out[j].File
		}
		return out[i].Line < out[j].Line
	})

	return out, nil
}

// buildMatcher returns a function that reports whether a single line matches.
func buildMatcher(query string, opts Options) (func(string) bool, error) {
	switch {
	case opts.Regex:
		pat := query
		if opts.CaseInsensitive {
			pat = "(?i)" + pat
		}
		re, err := regexp.Compile(pat)
		if err != nil {
			return nil, fmt.Errorf("references: invalid regex %q: %w", query, err)
		}
		return re.MatchString, nil

	case opts.Word:
		pat := `\b` + regexp.QuoteMeta(query) + `\b`
		if opts.CaseInsensitive {
			pat = "(?i)" + pat
		}
		re, err := regexp.Compile(pat)
		if err != nil {
			return nil, fmt.Errorf("references: word-boundary regex failed for %q: %w", query, err)
		}
		return re.MatchString, nil

	default:
		// Plain substring.
		needle := query
		if opts.CaseInsensitive {
			needle = strings.ToLower(query)
			return func(line string) bool {
				return strings.Contains(strings.ToLower(line), needle)
			}, nil
		}
		return func(line string) bool {
			return strings.Contains(line, needle)
		}, nil
	}
}
