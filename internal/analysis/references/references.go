// Package references implements fleet-wide occurrence search across source
// files. It is pure and deterministic: it reads no disk; callers hand it
// file contents, one file at a time when they stream (RT-87,
// docs/design/responsible-tenant.md §4.5).
package references

import (
	"bytes"
	"fmt"
	"regexp"
	"sort"
	"strings"

	"github.com/geoffmilleraz/fathom/internal/core"
)

// maxTextLen is the maximum number of bytes kept from a matched line.
const maxTextLen = 200

// sniffLen is how much of a file the binary sniff reads (connector.IsBinary's
// convention: a NUL byte in the first 8 KiB).
const sniffLen = 8 << 10

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
	// Max stops the search once it has found this many occurrences (0: no
	// bound) — files are read in the order given, so the result is the
	// first Max in that order.
	Max int
}

// Matcher matches lines against one compiled query.
type Matcher struct {
	match func([]byte) bool
}

// NewMatcher compiles query under opts. The error is non-nil only for an
// invalid regular expression.
func NewMatcher(query string, opts Options) (*Matcher, error) {
	match, err := buildMatcher(query, opts)
	if err != nil {
		return nil, err
	}
	return &Matcher{match: match}, nil
}

// FindInFile returns content's matching lines, in line order, walking the
// content line by line (no whole-file split, no line-length limit). A
// binary file (a NUL in its first 8 KiB) yields nothing. limit > 0 stops
// after that many occurrences; stopped reports that it did.
func (m *Matcher) FindInFile(path string, content []byte, limit int) (occs []Occurrence, stopped bool) {
	if bytes.IndexByte(content[:min(len(content), sniffLen)], 0) >= 0 {
		return nil, false
	}
	line := 0
	for rest := content; len(rest) > 0; {
		line++
		raw := rest
		if i := bytes.IndexByte(rest, '\n'); i >= 0 {
			raw, rest = rest[:i], rest[i+1:]
		} else {
			rest = nil
		}
		if !m.match(raw) {
			continue
		}
		text := bytes.TrimSpace(raw)
		if len(text) > maxTextLen {
			text = text[:maxTextLen]
		}
		occs = append(occs, Occurrence{File: path, Line: line, Text: string(text)})
		if limit > 0 && len(occs) >= limit {
			return occs, true
		}
	}
	return occs, false
}

// Find scans files one at a time and returns the lines that match query
// according to opts, stopping at opts.Max. Lines are 1-based; results are
// sorted by (File, Line). Binary files are skipped.
//
// Returns a non-nil error only when opts.Regex is true and query is an invalid
// regular expression.
func Find(query string, files []core.SourceFile, opts Options) ([]Occurrence, error) {
	if query == "" {
		return nil, nil
	}
	m, err := NewMatcher(query, opts)
	if err != nil {
		return nil, err
	}
	var out []Occurrence
	for _, f := range files {
		limit := 0
		if opts.Max > 0 {
			limit = opts.Max - len(out)
		}
		occs, stopped := m.FindInFile(f.Path, f.Content, limit)
		out = append(out, occs...)
		if stopped {
			break
		}
	}
	SortOccurrences(out)
	return out, nil
}

// SortOccurrences orders occurrences by (File, Line).
func SortOccurrences(out []Occurrence) {
	sort.Slice(out, func(i, j int) bool {
		if out[i].File != out[j].File {
			return out[i].File < out[j].File
		}
		return out[i].Line < out[j].Line
	})
}

// buildMatcher returns a function that reports whether a single line matches.
func buildMatcher(query string, opts Options) (func([]byte) bool, error) {
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
		return re.Match, nil

	case opts.Word:
		pat := `\b` + regexp.QuoteMeta(query) + `\b`
		if opts.CaseInsensitive {
			pat = "(?i)" + pat
		}
		re, err := regexp.Compile(pat)
		if err != nil {
			return nil, fmt.Errorf("references: word-boundary regex failed for %q: %w", query, err)
		}
		return re.Match, nil

	default:
		// Plain substring.
		if opts.CaseInsensitive {
			needle := []byte(strings.ToLower(query))
			return func(line []byte) bool {
				return bytes.Contains(bytes.ToLower(line), needle)
			}, nil
		}
		needle := []byte(query)
		return func(line []byte) bool {
			return bytes.Contains(line, needle)
		}, nil
	}
}
