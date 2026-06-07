// Package deprecation scans source text for usage of configured deprecated
// symbols/APIs and reports findings with file path and line number.
//
// The scanner is language-agnostic: it searches for any of the supplied
// deprecated symbol strings as substrings within each non-blank line.  Go,
// JS/TS, and Python source all work without special parsing.
//
// DefaultDeprecated contains a small, opinionated set of well-known deprecated
// Go standard-library symbols.
//
// All exported functions are pure, deterministic, and safe for concurrent use.
package deprecation

import (
	"sort"
	"strings"
)

// Finding is a single deprecated-symbol usage hit.
type Finding struct {
	File   string
	Line   int
	Symbol string
}

// DefaultDeprecated is the built-in list of deprecated symbols checked when
// the caller passes nil or an empty slice to Scan.
var DefaultDeprecated = []string{
	"ioutil.ReadFile",
	"ioutil.WriteFile",
	"ioutil.ReadAll",
	"ioutil.ReadDir",
	"ioutil.TempFile",
	"ioutil.TempDir",
	"ioutil.Discard",
	"ioutil.NopCloser",
	"os.SEEK_SET",
	"os.SEEK_CUR",
	"os.SEEK_END",
}

// Scan scans src (source text of a single file) for usage of any deprecated
// symbol in the deprecated list.
//
// If deprecated is nil or empty, DefaultDeprecated is used.
//
// Each line of src is checked for each deprecated symbol as a substring; lines
// that match produce a Finding.  Multiple matches on the same line produce one
// Finding per matched symbol (sorted deterministically).
//
// The returned slice is sorted by (File, Line, Symbol).
func Scan(path, src string, deprecated []string) []Finding {
	if len(deprecated) == 0 {
		deprecated = DefaultDeprecated
	}

	lines := strings.Split(src, "\n")
	var findings []Finding

	for lineIdx, line := range lines {
		lineNum := lineIdx + 1
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		for _, sym := range deprecated {
			if strings.Contains(line, sym) {
				findings = append(findings, Finding{
					File:   path,
					Line:   lineNum,
					Symbol: sym,
				})
			}
		}
	}

	sort.Slice(findings, func(i, j int) bool {
		if findings[i].File != findings[j].File {
			return findings[i].File < findings[j].File
		}
		if findings[i].Line != findings[j].Line {
			return findings[i].Line < findings[j].Line
		}
		return findings[i].Symbol < findings[j].Symbol
	})

	return findings
}
