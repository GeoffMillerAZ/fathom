package extractor

import (
	"bufio"
	"bytes"
	"regexp"
	"sort"
	"strings"

	"github.com/geoffmilleraz/sextant/internal/core"
)

// SQLExtractor implements Extractor for SQL DDL files (.sql).
// It uses compiled case-insensitive regexes anchored at statement starts to
// detect DDL object definitions without any external dependency.
//
// Detected statement types and emitted kinds:
//
//   - CREATE [OR REPLACE] [TEMP|TEMPORARY|GLOBAL|LOCAL …] TABLE [IF NOT EXISTS] <name>  → KindSQLTable
//   - CREATE [OR REPLACE] [MATERIALIZED] VIEW [IF NOT EXISTS] <name>                    → KindSQLView
//   - CREATE [UNIQUE] INDEX [IF NOT EXISTS] <name>                                       → KindSQLIndex
//   - CREATE [OR REPLACE] FUNCTION <name>                                                → KindSQLFunction
//   - CREATE [OR REPLACE] PROCEDURE <name>                                               → KindSQLFunction
//   - CREATE TYPE <name>                                                                 → KindSQLType
//
// Object names have surrounding quote characters (" ` [ ]) stripped; schema
// qualifiers (e.g. public.users) are preserved.
//
// Tier is TierPreScan — pattern-matched, not fully parsed.
// Best-effort: never returns a non-nil error.
type SQLExtractor struct {
	rules []sqlRule
}

// sqlRule pairs a compiled regex with the SourceKind it produces and a human
// readable signature label.
type sqlRule struct {
	re   *regexp.Regexp
	kind core.SourceKind
	sig  string
}

// NewSQLExtractor returns a new SQLExtractor with all DDL rules compiled.
func NewSQLExtractor() *SQLExtractor {
	// Each regex captures the object name in the first capture group.
	// Patterns are case-insensitive ((?i) prefix) and anchored to the start of
	// the trimmed statement line so comments / whitespace before CREATE are
	// handled by the line trimmer.
	rules := []sqlRule{
		{
			// CREATE [OR REPLACE] [TEMP|TEMPORARY|GLOBAL|LOCAL [TEMP|TEMPORARY]] TABLE [IF NOT EXISTS] <name>
			re:   regexp.MustCompile(`(?i)^CREATE\s+(?:OR\s+REPLACE\s+)?(?:(?:GLOBAL\s+|LOCAL\s+)?(?:TEMP(?:ORARY)?\s+))?TABLE\s+(?:IF\s+NOT\s+EXISTS\s+)?([^\s(;]+)`),
			kind: core.KindSQLTable,
			sig:  "CREATE TABLE",
		},
		{
			// CREATE [OR REPLACE] [MATERIALIZED] VIEW [IF NOT EXISTS] <name>
			re:   regexp.MustCompile(`(?i)^CREATE\s+(?:OR\s+REPLACE\s+)?(?:MATERIALIZED\s+)?VIEW\s+(?:IF\s+NOT\s+EXISTS\s+)?([^\s(;]+)`),
			kind: core.KindSQLView,
			sig:  "CREATE VIEW",
		},
		{
			// CREATE [UNIQUE] INDEX [IF NOT EXISTS] <name>
			re:   regexp.MustCompile(`(?i)^CREATE\s+(?:UNIQUE\s+)?INDEX\s+(?:IF\s+NOT\s+EXISTS\s+)?([^\s(;]+)`),
			kind: core.KindSQLIndex,
			sig:  "CREATE INDEX",
		},
		{
			// CREATE [OR REPLACE] FUNCTION <name>
			re:   regexp.MustCompile(`(?i)^CREATE\s+(?:OR\s+REPLACE\s+)?FUNCTION\s+([^\s(;]+)`),
			kind: core.KindSQLFunction,
			sig:  "CREATE FUNCTION",
		},
		{
			// CREATE [OR REPLACE] PROCEDURE <name>
			re:   regexp.MustCompile(`(?i)^CREATE\s+(?:OR\s+REPLACE\s+)?PROCEDURE\s+([^\s(;]+)`),
			kind: core.KindSQLFunction,
			sig:  "CREATE PROCEDURE",
		},
		{
			// CREATE TYPE <name>
			re:   regexp.MustCompile(`(?i)^CREATE\s+TYPE\s+([^\s(;]+)`),
			kind: core.KindSQLType,
			sig:  "CREATE TYPE",
		},
	}
	return &SQLExtractor{rules: rules}
}

// Extract scans SQL DDL source bytes line by line and returns one SymbolNode
// per detected object definition. Output is sorted by Line then Name.
func (s *SQLExtractor) Extract(service, relPath string, src []byte) ([]core.SymbolNode, error) {
	pkg := pkgFromPath(relPath)
	var nodes []core.SymbolNode

	scanner := bufio.NewScanner(bytes.NewReader(src))
	lineNum := 0
	for scanner.Scan() {
		lineNum++
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "--") || strings.HasPrefix(line, "#") {
			continue
		}

		for _, rule := range s.rules {
			m := rule.re.FindStringSubmatch(line)
			if m == nil {
				continue
			}
			name := stripQuotes(m[1])
			if name == "" {
				continue
			}
			nodes = append(nodes, core.SymbolNode{
				Service:   service,
				Package:   pkg,
				Name:      name,
				Kind:      rule.kind,
				FilePath:  relPath,
				Line:      lineNum,
				Signature: rule.sig,
				Tier:      core.TierPreScan,
			})
			break // first matching rule wins; move to next line
		}
	}

	sort.Slice(nodes, func(i, j int) bool {
		if nodes[i].Line != nodes[j].Line {
			return nodes[i].Line < nodes[j].Line
		}
		return nodes[i].Name < nodes[j].Name
	})
	return nodes, nil
}

// stripQuotes removes surrounding quote characters from an SQL identifier.
// Supported delimiters: double-quotes ("), backticks (`), and brackets ([ ]).
// Schema qualifiers (e.g. "public"."users") are preserved as-is after stripping
// individual token quotes.
func stripQuotes(s string) string {
	s = strings.TrimSpace(s)
	if len(s) == 0 {
		return s
	}

	// Handle bracket quoting: [schema].[name] or [name]
	if strings.ContainsAny(s, "[]") {
		var b strings.Builder
		inBracket := false
		for _, r := range s {
			switch r {
			case '[':
				inBracket = true
			case ']':
				inBracket = false
			default:
				if !inBracket || r != ']' {
					b.WriteRune(r)
				}
			}
		}
		return b.String()
	}

	// Handle double-quote and backtick delimited identifiers.
	// Each dot-separated token is unquoted individually.
	parts := strings.Split(s, ".")
	for i, p := range parts {
		if len(p) >= 2 {
			first, last := p[0], p[len(p)-1]
			if (first == '"' && last == '"') || (first == '`' && last == '`') {
				parts[i] = p[1 : len(p)-1]
				continue
			}
		}
		// No quoting on this token — leave as-is.
		parts[i] = p
	}
	return strings.Join(parts, ".")
}
