// Package markers scans source files for annotation comments (TODO, FIXME,
// HACK, XXX, BUG, DEPRECATED) and extracts optional ticket references embedded
// in the comment text.
//
// Supported ticket patterns:
//   - Jira-style:  [ABC-123]  (uppercase letters, dash, digits)
//   - GitHub-style: #456      (hash + digits)
//
// The scanner is language-agnostic: it treats any line containing one of the
// marker keywords (case-insensitive word boundary) as a match, regardless of
// the surrounding comment syntax.
//
// All exported functions are pure, deterministic, and safe for concurrent use.
package markers

import (
	"regexp"
	"sort"
	"strings"
)

// Kind enumerates the annotation types.
type Kind string

const (
	KindTODO       Kind = "TODO"
	KindFIXME      Kind = "FIXME"
	KindHACK       Kind = "HACK"
	KindXXX        Kind = "XXX"
	KindBUG        Kind = "BUG"
	KindDEPRECATED Kind = "DEPRECATED"
)

// Marker is a single annotation found in source text.
type Marker struct {
	File   string
	Line   int
	Kind   Kind
	Text   string // full trimmed comment text
	Ticket string // first ticket reference found, or ""
}

var (
	// markerRe matches TODO/FIXME/etc. as whole words (case-insensitive).
	markerRe = regexp.MustCompile(`(?i)\b(TODO|FIXME|HACK|XXX|BUG|DEPRECATED)\b`)

	// jiraRe matches [ABC-123] style tickets.
	jiraRe = regexp.MustCompile(`\[([A-Z]+-\d+)\]`)

	// githubRe matches #456 style issue refs.
	githubRe = regexp.MustCompile(`#(\d+)`)
)

// Scan scans the text of a source file and returns all markers found.
// filename is used to populate Marker.File.  The result is sorted by
// File, Line, Kind for determinism.
func Scan(filename, text string) []Marker {
	lines := strings.Split(text, "\n")
	var results []Marker

	for i, line := range lines {
		lineNum := i + 1
		match := markerRe.FindStringIndex(line)
		if match == nil {
			continue
		}
		k := Kind(strings.ToUpper(markerRe.FindString(line)))
		// Trim comment prefixes and surrounding whitespace.
		trimmed := strings.TrimSpace(line)
		trimmed = strings.TrimPrefix(trimmed, "//")
		trimmed = strings.TrimPrefix(trimmed, "#")
		trimmed = strings.TrimPrefix(trimmed, "--")
		trimmed = strings.TrimPrefix(trimmed, "/*")
		trimmed = strings.TrimSuffix(trimmed, "*/")
		trimmed = strings.TrimSpace(trimmed)

		ticket := extractTicket(line)

		results = append(results, Marker{
			File:   filename,
			Line:   lineNum,
			Kind:   k,
			Text:   trimmed,
			Ticket: ticket,
		})
	}

	sort.Slice(results, func(i, j int) bool {
		if results[i].File != results[j].File {
			return results[i].File < results[j].File
		}
		if results[i].Line != results[j].Line {
			return results[i].Line < results[j].Line
		}
		return string(results[i].Kind) < string(results[j].Kind)
	})
	return results
}

// extractTicket returns the first ticket reference found in line, or "".
// Jira-style [ABC-123] takes precedence over GitHub-style #456.
func extractTicket(line string) string {
	if m := jiraRe.FindStringSubmatch(line); m != nil {
		return m[1]
	}
	if m := githubRe.FindStringSubmatch(line); m != nil {
		return "#" + m[1]
	}
	return ""
}
