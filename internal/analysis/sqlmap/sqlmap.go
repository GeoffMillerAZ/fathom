// Package sqlmap extracts SQL table names referenced in Go source string
// literals.
//
// Extract scans the provided source text for string literals containing SQL
// DML/DQL keywords (FROM, JOIN, INTO, UPDATE) immediately followed by a table
// name token.  It works as a line-oriented regex-style scan so it operates on
// any text file, not just valid Go.
//
// All results are sorted (by File, Line, Name) and deterministic.
package sqlmap

import (
	"regexp"
	"sort"
	"strings"
)

// Table is a table name found in a SQL expression inside a source file.
type Table struct {
	Name string
	File string
	Line int
}

// tablePattern matches SQL keywords that introduce a table name:
// FROM, JOIN (all variants), INTO, UPDATE.
// It captures the table name that follows (stopping at whitespace, comma,
// semicolon, closing paren/quote, or end of string).
var tablePattern = regexp.MustCompile(
	`(?i)\b(?:FROM|JOIN|INTO|UPDATE)\s+([a-zA-Z_][a-zA-Z0-9_]*)`,
)

// Extract scans src for SQL table references in string literals and returns a
// sorted slice of Table records.
//
// path is used as the File field of each result.  Line numbers are 1-based.
//
// The scanner is heuristic: it looks for the SQL keywords inside any string
// literal delimiters (" or ` or ') found on each line. Lines that contain these
// keywords outside string context (e.g., in comments or identifiers) may also
// produce matches — callers should pre-filter if precision is required.
func Extract(path, src string) []Table {
	lines := strings.Split(src, "\n")
	var tables []Table

	seen := make(map[string]bool) // deduplicate exact (name, line) pairs

	for lineIdx, line := range lines {
		lineNum := lineIdx + 1
		matches := tablePattern.FindAllStringSubmatch(line, -1)
		for _, m := range matches {
			name := m[1]
			// Skip SQL keywords that may appear as the "table name" in patterns
			// like "FROM SELECT" or similar nonsense.
			if isSQLKeyword(name) {
				continue
			}
			key := strings.Join([]string{name, path, string(rune(lineNum))}, "\x00")
			if seen[key] {
				continue
			}
			seen[key] = true
			tables = append(tables, Table{
				Name: name,
				File: path,
				Line: lineNum,
			})
		}
	}

	sort.Slice(tables, func(i, j int) bool {
		if tables[i].File != tables[j].File {
			return tables[i].File < tables[j].File
		}
		if tables[i].Line != tables[j].Line {
			return tables[i].Line < tables[j].Line
		}
		return tables[i].Name < tables[j].Name
	})
	return tables
}

// isSQLKeyword returns true when name is a common SQL keyword that should not
// be treated as a table name.
func isSQLKeyword(name string) bool {
	switch strings.ToUpper(name) {
	case "SELECT", "INSERT", "DELETE", "UPDATE", "WHERE", "SET",
		"VALUES", "AND", "OR", "NOT", "NULL", "AS", "ON",
		"INNER", "LEFT", "RIGHT", "OUTER", "CROSS", "FULL",
		"GROUP", "ORDER", "HAVING", "LIMIT", "OFFSET",
		"DISTINCT", "ALL", "EXISTS", "IN", "BETWEEN", "LIKE":
		return true
	}
	return false
}
