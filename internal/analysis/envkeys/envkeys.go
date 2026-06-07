// Package envkeys extracts environment-variable read sites from source text.
//
// Supported patterns (across Go, JavaScript/TypeScript, and Python):
//
//	Go:         os.Getenv("KEY")
//	Go:         os.LookupEnv("KEY")
//	JavaScript: process.env.KEY  (bare identifier after the last '.')
//	JavaScript: process.env["KEY"]  or  process.env['KEY']
//	Python:     os.environ["KEY"]  or  os.environ['KEY']
//	Python:     os.environ.get("KEY")  or  os.environ.get('KEY')
//	Python:     os.getenv("KEY")
//
// The extractor operates on raw source text using simple string scanning; no
// AST parsing is required, making it language-version agnostic.
//
// All exported functions are pure, deterministic, and safe for concurrent use.
// Results are sorted by (File, Line, Name).
package envkeys

import (
	"sort"
	"strings"
	"unicode"
)

// EnvKey is a single environment-variable read site.
type EnvKey struct {
	Name string // environment variable name, e.g. "DATABASE_URL"
	File string
	Line int
}

// Scan extracts all environment-variable reads from src (source text of a
// single file at path) and returns them sorted by (File, Line, Name).
func Scan(path, src string) []EnvKey {
	var keys []EnvKey

	lines := strings.Split(src, "\n")
	for lineIdx, line := range lines {
		lineNum := lineIdx + 1
		keys = append(keys, scanLine(path, line, lineNum)...)
	}

	sort.Slice(keys, func(i, j int) bool {
		if keys[i].File != keys[j].File {
			return keys[i].File < keys[j].File
		}
		if keys[i].Line != keys[j].Line {
			return keys[i].Line < keys[j].Line
		}
		return keys[i].Name < keys[j].Name
	})

	return keys
}

// scanLine extracts EnvKey records from a single line of source text.
func scanLine(path, line string, lineNum int) []EnvKey {
	var keys []EnvKey

	// We'll scan the line multiple times, once per pattern family.  Duplicate
	// (name, line) pairs on the same line are deduplicated at the end.
	seen := make(map[string]bool)

	add := func(name string) {
		name = strings.TrimSpace(name)
		if name == "" || seen[name] {
			return
		}
		seen[name] = true
		keys = append(keys, EnvKey{Name: name, File: path, Line: lineNum})
	}

	// ── Go: os.Getenv("KEY") / os.LookupEnv("KEY") ───────────────────────
	for _, fn := range []string{"os.Getenv(", "os.LookupEnv("} {
		rest := line
		for {
			idx := strings.Index(rest, fn)
			if idx < 0 {
				break
			}
			after := rest[idx+len(fn):]
			if name, ok := extractQuotedArg(after); ok {
				add(name)
			}
			rest = rest[idx+len(fn):]
		}
	}

	// ── Python: os.environ["KEY"] / os.environ['KEY'] ─────────────────────
	for _, prefix := range []string{`os.environ["`, `os.environ['`} {
		rest := line
		for {
			idx := strings.Index(rest, prefix)
			if idx < 0 {
				break
			}
			after := rest[idx+len(prefix):]
			endQuote := strings.IndexAny(after, `"'`)
			if endQuote >= 0 {
				add(after[:endQuote])
			}
			rest = rest[idx+len(prefix):]
		}
	}

	// ── Python: os.environ.get("KEY") / os.environ.get('KEY') ────────────
	for _, fn := range []string{`os.environ.get("`, `os.environ.get('`} {
		rest := line
		for {
			idx := strings.Index(rest, fn)
			if idx < 0 {
				break
			}
			after := rest[idx+len(fn):]
			endQuote := strings.IndexAny(after, `"'`)
			if endQuote >= 0 {
				add(after[:endQuote])
			}
			rest = rest[idx+len(fn):]
		}
	}

	// ── Python: os.getenv("KEY") / os.getenv('KEY') ───────────────────────
	for _, fn := range []string{`os.getenv("`, `os.getenv('`} {
		rest := line
		for {
			idx := strings.Index(rest, fn)
			if idx < 0 {
				break
			}
			after := rest[idx+len(fn):]
			endQuote := strings.IndexAny(after, `"'`)
			if endQuote >= 0 {
				add(after[:endQuote])
			}
			rest = rest[idx+len(fn):]
		}
	}

	// ── JS/TS: process.env["KEY"] / process.env['KEY'] ───────────────────
	for _, prefix := range []string{`process.env["`, `process.env['`} {
		rest := line
		for {
			idx := strings.Index(rest, prefix)
			if idx < 0 {
				break
			}
			after := rest[idx+len(prefix):]
			endQuote := strings.IndexAny(after, `"'`)
			if endQuote >= 0 {
				add(after[:endQuote])
			}
			rest = rest[idx+len(prefix):]
		}
	}

	// ── JS/TS: process.env.KEY  (bare identifier) ─────────────────────────
	const envDot = "process.env."
	rest := line
	for {
		idx := strings.Index(rest, envDot)
		if idx < 0 {
			break
		}
		after := rest[idx+len(envDot):]
		// Collect identifier characters.
		end := 0
		for end < len(after) && isIdentRune(rune(after[end])) {
			end++
		}
		if end > 0 {
			// Avoid double-counting bracket-access patterns already captured.
			if after[0] != '[' && after[0] != '\'' && after[0] != '"' {
				add(after[:end])
			}
		}
		rest = rest[idx+len(envDot):]
	}

	return keys
}

// extractQuotedArg extracts a string literal from the beginning of s, where s
// starts right after the opening parenthesis of a call like os.Getenv(.
// Returns the string content and true on success.
func extractQuotedArg(s string) (string, bool) {
	s = strings.TrimLeft(s, " \t")
	if len(s) == 0 {
		return "", false
	}
	quote := s[0]
	if quote != '"' && quote != '\'' && quote != '`' {
		return "", false
	}
	end := strings.IndexByte(s[1:], quote)
	if end < 0 {
		return "", false
	}
	return s[1 : end+1], true
}

// isIdentRune returns true for characters valid in an environment-variable
// name: letters, digits, and underscore.
func isIdentRune(r rune) bool {
	return unicode.IsLetter(r) || unicode.IsDigit(r) || r == '_'
}
