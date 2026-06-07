// Package cloc counts blank lines, comment lines, and code lines in source
// files.  It supports Go, JavaScript/TypeScript, and Python comment syntax.
//
// All exported functions are pure, deterministic, and safe for concurrent use.
package cloc

import (
	"strings"
)

// Stats holds the line counts for a single file.
type Stats struct {
	Blank   int // lines that are empty or contain only whitespace
	Comment int // lines that are entirely a comment (single-line comment or inside a block)
	Code    int // all other lines (including lines with both code and inline comments)
}

// Total returns the sum of all line categories.
func (s Stats) Total() int { return s.Blank + s.Comment + s.Code }

// ─── Count ───────────────────────────────────────────────────────────────────

// Count analyses src (source text) for the given language and returns Stats.
//
// Supported lang values (case-insensitive):
//
//	"go"                         — // line comments, /* … */ block comments
//	"js", "ts", "javascript",
//	"typescript"                  — // line comments, /* … */ block comments
//	"py", "python"               — # line comments, triple-quoted """ strings
//	                               used as docstrings are treated as comments
//
// For unrecognised languages every non-blank line is counted as code.
func Count(lang, src string) Stats {
	switch strings.ToLower(lang) {
	case "go", "js", "ts", "javascript", "typescript":
		return countCStyle(src)
	case "py", "python":
		return countPython(src)
	default:
		return countGeneric(src)
	}
}

// ─── Aggregate ───────────────────────────────────────────────────────────────

// Aggregate sums []Stats slices keyed by language into a single Stats per
// language and returns the map.  The input map values are consumed but not
// mutated.
func Aggregate(m map[string][]Stats) map[string]Stats {
	out := make(map[string]Stats, len(m))
	for lang, slice := range m {
		var total Stats
		for _, s := range slice {
			total.Blank += s.Blank
			total.Comment += s.Comment
			total.Code += s.Code
		}
		out[lang] = total
	}
	return out
}

// ─── C-style (Go / JS / TS) ──────────────────────────────────────────────────

func countCStyle(src string) Stats {
	var s Stats
	lines := splitLines(src)
	inBlock := false

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)

		if inBlock {
			// We are inside a /* … */ block comment; blank lines count as comment.
			if trimmed == "" {
				s.Comment++
				continue
			}
			if idx := strings.Index(trimmed, "*/"); idx >= 0 {
				inBlock = false
				// Everything after */ on this line.
				after := strings.TrimSpace(trimmed[idx+2:])
				if after == "" || strings.HasPrefix(after, "//") {
					s.Comment++
				} else {
					s.Code++
				}
			} else {
				s.Comment++
			}
			continue
		}

		if trimmed == "" {
			s.Blank++
			continue
		}

		// Check for a block comment that starts on this line.
		if idx := strings.Index(trimmed, "/*"); idx >= 0 {
			// Text before the /* ?
			before := strings.TrimSpace(trimmed[:idx])
			if before != "" && !strings.HasPrefix(before, "//") {
				// Code precedes the block comment opener.
				s.Code++
				// Consume through */ if present.
				rest := trimmed[idx+2:]
				if end := strings.Index(rest, "*/"); end >= 0 {
					// Block opens and closes on same line; inBlock stays false.
				} else {
					inBlock = true
				}
				continue
			}
			// Block comment opens with no preceding code.
			rest := trimmed[idx+2:]
			if end := strings.Index(rest, "*/"); end >= 0 {
				// Single-line block comment like /* foo */.
				after := strings.TrimSpace(rest[end+2:])
				if after == "" || strings.HasPrefix(after, "//") {
					s.Comment++
				} else {
					s.Code++
				}
			} else {
				inBlock = true
				s.Comment++
			}
			continue
		}

		// Single-line comment.
		if strings.HasPrefix(trimmed, "//") {
			s.Comment++
			continue
		}

		s.Code++
	}

	return s
}

// ─── Python ──────────────────────────────────────────────────────────────────

func countPython(src string) Stats {
	var s Stats
	lines := splitLines(src)
	inTriple := false
	tripleDelim := ""

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)

		if inTriple {
			// Lines inside a triple-quoted docstring are always comment lines,
			// including blank-looking lines (they are part of the docstring).
			if idx := strings.Index(line, tripleDelim); idx >= 0 {
				inTriple = false
				after := strings.TrimSpace(line[idx+3:])
				if after == "" || strings.HasPrefix(after, "#") {
					s.Comment++
				} else {
					s.Code++
				}
			} else {
				s.Comment++
			}
			continue
		}

		if trimmed == "" {
			s.Blank++
			continue
		}

		// Check for triple-quoted string start.
		if delim, ok := tripleQuoteStart(trimmed); ok {
			rest := trimmed[strings.Index(trimmed, delim)+3:]
			// Is the closing delimiter also on this line?
			if strings.Contains(rest, delim) {
				// Inline triple-quoted string: treat as comment if the line
				// contains nothing else.
				before := strings.TrimSpace(trimmed[:strings.Index(trimmed, delim)])
				if before == "" || before == "#" {
					s.Comment++
				} else {
					s.Code++
				}
			} else {
				inTriple = true
				tripleDelim = delim
				before := strings.TrimSpace(trimmed[:strings.Index(trimmed, delim)])
				if before == "" {
					s.Comment++
				} else {
					s.Code++
				}
			}
			continue
		}

		// # line comment.
		if strings.HasPrefix(trimmed, "#") {
			s.Comment++
			continue
		}

		s.Code++
	}

	return s
}

// tripleQuoteStart returns the triple-quote delimiter ("\"\"\"" or "”'") if
// trimmed starts or contains one that isn't yet closed on this call.
func tripleQuoteStart(trimmed string) (string, bool) {
	for _, d := range []string{`"""`, `'''`} {
		if idx := strings.Index(trimmed, d); idx >= 0 {
			// Make sure it's not inside a #-comment that precedes it.
			before := trimmed[:idx]
			if strings.Contains(before, "#") {
				continue
			}
			return d, true
		}
	}
	return "", false
}

// ─── Generic (unknown language) ──────────────────────────────────────────────

func countGeneric(src string) Stats {
	var s Stats
	for _, line := range splitLines(src) {
		if strings.TrimSpace(line) == "" {
			s.Blank++
		} else {
			s.Code++
		}
	}
	return s
}

// ─── helpers ─────────────────────────────────────────────────────────────────

// splitLines splits src on newlines.  An empty src returns nil.
func splitLines(src string) []string {
	if src == "" {
		return nil
	}
	// strings.Split on "\n" leaves a trailing empty string if src ends with \n.
	lines := strings.Split(src, "\n")
	// Trim a trailing empty element that results from a trailing newline.
	if len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	return lines
}
