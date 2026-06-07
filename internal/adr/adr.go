// Package adr parses Architecture Decision Records from Markdown files and
// provides conformance checking and active-decision filtering.
//
// ADR files follow the lightweight Markdown ADR format:
//
//	# ADR-NNN: Title
//
//	Status: accepted
//
// or the section-based variant:
//
//	## Status
//
//	accepted
//
// Parse extracts ID, Title, Status, and Path from a single file.
// Collect gathers ADRs from a map of path→content and returns them sorted by ID.
// ActiveDecisions returns accepted, non-superseded ADRs.
// Conformance flags ADRs missing required fields.
package adr

import (
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

// Status is the lifecycle state of an Architecture Decision Record.
type Status string

const (
	StatusProposed   Status = "proposed"
	StatusAccepted   Status = "accepted"
	StatusSuperseded Status = "superseded"
	StatusDeprecated Status = "deprecated"
	StatusRejected   Status = "rejected"
	// StatusUnknown is set when no recognizable Status line is found.
	StatusUnknown Status = ""
)

// ADR holds the parsed fields from a single ADR Markdown file.
type ADR struct {
	ID     string // e.g. "ADR-001"
	Title  string // text of the first "# " heading (ADR-NNN prefix stripped)
	Status Status
	Path   string // original path argument passed to Parse
}

// Issue records a single conformance problem found by Conformance.
type Issue struct {
	ADRID   string // ID of the offending ADR (may be empty when ID itself is missing)
	Path    string // file path of the ADR
	Field   string // "id", "title", or "status"
	Message string // human-readable description
}

// adrIDRe matches "ADR-NNN" at a word boundary (NNN = one or more digits).
var adrIDRe = regexp.MustCompile(`(?i)\bADR-(\d+)\b`)

// statusLineRe matches "Status: <value>" (case-insensitive, leading whitespace allowed).
var statusLineRe = regexp.MustCompile(`(?i)^\s*status\s*:\s*(\S+)`)

// statusSectionRe matches "## Status" section headings.
var statusSectionRe = regexp.MustCompile(`(?i)^#{1,6}\s+status\s*$`)

// h1Re matches a first-level Markdown heading ("# ...").
var h1Re = regexp.MustCompile(`^#\s+(.+)`)

// Parse parses a single ADR file.
//
// ID resolution order:
//  1. First ADR-NNN match in the base filename (without extension).
//  2. First ADR-NNN match in the first "# " heading.
//
// Title is taken from the first "# " heading; if the heading begins with
// "ADR-NNN:" or "ADR-NNN -" the prefix is stripped.
//
// Status is extracted from either a "Status: <value>" line or the first
// non-blank line following a "## Status" heading.  The value is lowercased and
// matched to the known Status constants; unrecognised values become StatusUnknown.
//
// Parse never returns a non-nil error for well-formed input; the error path is
// reserved for future strict-mode validation.
func Parse(path string, content []byte) (ADR, error) {
	a := ADR{Path: path}

	// Attempt to extract ID from the base filename first.
	base := filepath.Base(path)
	ext := filepath.Ext(base)
	stem := strings.TrimSuffix(base, ext)
	if m := adrIDRe.FindString(stem); m != "" {
		a.ID = strings.ToUpper(m)
	}

	lines := splitLines(content)

	// Single-pass: collect heading, status line, and section-based status.
	var (
		h1Text          string // raw text of the first "# " heading
		statusValue     string // resolved status string
		inStatusSection bool   // true immediately after a "## Status" heading
	)

	for _, line := range lines {
		// First-level heading.
		if h1Text == "" {
			if m := h1Re.FindStringSubmatch(line); m != nil {
				h1Text = strings.TrimSpace(m[1])
				// If we haven't found an ID from the filename, try the heading.
				if a.ID == "" {
					if idMatch := adrIDRe.FindString(h1Text); idMatch != "" {
						a.ID = strings.ToUpper(idMatch)
					}
				}
				inStatusSection = false
				continue
			}
		}

		// Inline "Status: <value>" on any line.
		if statusValue == "" {
			if m := statusLineRe.FindStringSubmatch(line); m != nil {
				statusValue = strings.ToLower(strings.TrimSpace(m[1]))
				inStatusSection = false
				continue
			}
		}

		// "## Status" section heading.
		if statusSectionRe.MatchString(line) {
			inStatusSection = true
			continue
		}

		// First non-blank line after "## Status".
		if inStatusSection {
			trimmed := strings.TrimSpace(line)
			if trimmed != "" && statusValue == "" {
				statusValue = strings.ToLower(trimmed)
				inStatusSection = false
			}
			continue
		}
	}

	// Resolve title from h1Text, stripping any leading "ADR-NNN[: | - ]" prefix.
	if h1Text != "" {
		a.Title = stripADRPrefix(h1Text)
	}

	// Map status string to Status constant.
	a.Status = parseStatus(statusValue)

	return a, nil
}

// stripADRPrefix removes a leading "ADR-NNN" prefix (plus optional separator)
// from a heading string, returning only the descriptive title portion.
//
// Examples:
//
//	"ADR-001: Use SQLite" → "Use SQLite"
//	"ADR-002 - Adopt gRPC" → "Adopt gRPC"
//	"Use SQLite" → "Use SQLite"
func stripADRPrefix(s string) string {
	// Find the end of the ADR-NNN token.
	loc := adrIDRe.FindStringIndex(s)
	if loc == nil {
		return s
	}
	rest := strings.TrimSpace(s[loc[1]:])
	// Strip a leading separator (: or -).
	rest = strings.TrimLeft(rest, ":- ")
	rest = strings.TrimSpace(rest)
	if rest == "" {
		// Heading was only the ID; return the ID itself as the title.
		return s
	}
	return rest
}

// parseStatus converts a lowercase status string to a Status constant.
func parseStatus(s string) Status {
	switch s {
	case "proposed":
		return StatusProposed
	case "accepted":
		return StatusAccepted
	case "superseded":
		return StatusSuperseded
	case "deprecated":
		return StatusDeprecated
	case "rejected":
		return StatusRejected
	default:
		return StatusUnknown
	}
}

// splitLines splits content into lines, stripping trailing \r.
func splitLines(content []byte) []string {
	if len(content) == 0 {
		return nil
	}
	raw := strings.Split(string(content), "\n")
	lines := make([]string, 0, len(raw))
	for _, l := range raw {
		lines = append(lines, strings.TrimRight(l, "\r"))
	}
	return lines
}

// Collect parses every file in the files map and returns all ADRs sorted by ID.
//
// Files without a recognisable ADR-NNN ID sort after those with one (lexicographic
// on the empty string, i.e. at the front — callers should rely on Conformance to
// flag these).  Within the same ID (or both empty), files sort by path.
func Collect(files map[string][]byte) ([]ADR, error) {
	adrs := make([]ADR, 0, len(files))
	for path, content := range files {
		a, err := Parse(path, content)
		if err != nil {
			return nil, err
		}
		adrs = append(adrs, a)
	}
	sort.Slice(adrs, func(i, j int) bool {
		if adrs[i].ID != adrs[j].ID {
			// Empty IDs sort last.
			if adrs[i].ID == "" {
				return false
			}
			if adrs[j].ID == "" {
				return true
			}
			return adrs[i].ID < adrs[j].ID
		}
		return adrs[i].Path < adrs[j].Path
	})
	return adrs, nil
}

// ActiveDecisions returns the subset of adrs that are accepted and not superseded.
// The input order is preserved; only accepted-status ADRs are included.
func ActiveDecisions(adrs []ADR) []ADR {
	out := make([]ADR, 0, len(adrs))
	for _, a := range adrs {
		if a.Status == StatusAccepted {
			out = append(out, a)
		}
	}
	return out
}

// Conformance checks each ADR for missing required fields and returns the
// discovered issues.  An ADR is conformant when it has a non-empty ID, a
// non-empty Title, and a known Status (not StatusUnknown).
//
// The returned slice is sorted by (Path, Field) for determinism.
func Conformance(adrs []ADR) []Issue {
	var issues []Issue
	for _, a := range adrs {
		if a.ID == "" {
			issues = append(issues, Issue{
				ADRID:   a.ID,
				Path:    a.Path,
				Field:   "id",
				Message: "ADR is missing an ID (no ADR-NNN marker found in filename or heading)",
			})
		}
		if a.Title == "" {
			issues = append(issues, Issue{
				ADRID:   a.ID,
				Path:    a.Path,
				Field:   "title",
				Message: "ADR is missing a title (no '# ' heading found)",
			})
		}
		if a.Status == StatusUnknown {
			issues = append(issues, Issue{
				ADRID:   a.ID,
				Path:    a.Path,
				Field:   "status",
				Message: "ADR is missing a recognized Status value",
			})
		}
	}
	sort.Slice(issues, func(i, j int) bool {
		if issues[i].Path != issues[j].Path {
			return issues[i].Path < issues[j].Path
		}
		return issues[i].Field < issues[j].Field
	})
	return issues
}
