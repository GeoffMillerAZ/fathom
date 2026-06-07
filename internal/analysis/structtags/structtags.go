// Package structtags audits Go struct tags via go/ast.
//
// Two classes of issue are reported:
//   - Malformed struct tags (not parseable by reflect.StructTag.Lookup).
//   - Missing `json` tags on exported struct fields.
//
// All exported functions are pure, deterministic, and safe for concurrent use.
package structtags

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"reflect"
	"sort"
	"strings"
	"unicode"
)

// Issue is a single struct-tag violation.
type Issue struct {
	File    string
	Line    int
	Field   string
	Message string
}

// Scan parses Go source in src (with filename path for error attribution) and
// returns a deterministic list of struct-tag Issues. src must be a complete Go
// source file.
func Scan(path, src string) []Issue {
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, path, src, 0)
	if err != nil {
		return []Issue{{
			File:    path,
			Line:    0,
			Field:   "",
			Message: fmt.Sprintf("parse error: %v", err),
		}}
	}

	var issues []Issue

	ast.Inspect(f, func(n ast.Node) bool {
		st, ok := n.(*ast.StructType)
		if !ok {
			return true
		}
		if st.Fields == nil {
			return true
		}
		for _, field := range st.Fields.List {
			if field.Tag == nil {
				// No tag at all — check if exported fields need json.
				for _, ident := range field.Names {
					if isExported(ident.Name) {
						line := fset.Position(field.Pos()).Line
						issues = append(issues, Issue{
							File:    path,
							Line:    line,
							Field:   ident.Name,
							Message: fmt.Sprintf("exported field %q is missing a json struct tag", ident.Name),
						})
					}
				}
				continue
			}

			rawTag := strings.Trim(field.Tag.Value, "`")
			tag := reflect.StructTag(rawTag)

			// Check for malformed tag: a well-formed tag must be parseable.
			// reflect.StructTag.Lookup silently ignores malformed content, so we
			// use the stricter check: every key:value pair must be well-formed.
			if isMalformed(rawTag) {
				for _, ident := range field.Names {
					line := fset.Position(field.Pos()).Line
					issues = append(issues, Issue{
						File:    path,
						Line:    line,
						Field:   ident.Name,
						Message: fmt.Sprintf("field %q has malformed struct tag %q", ident.Name, rawTag),
					})
				}
				continue
			}

			// Check for missing json tag on exported fields.
			for _, ident := range field.Names {
				if isExported(ident.Name) {
					if jsonTag, _ := tag.Lookup("json"); jsonTag == "" {
						line := fset.Position(field.Pos()).Line
						issues = append(issues, Issue{
							File:    path,
							Line:    line,
							Field:   ident.Name,
							Message: fmt.Sprintf("exported field %q is missing a json struct tag", ident.Name),
						})
					}
				}
			}
		}
		return true
	})

	sort.Slice(issues, func(i, j int) bool {
		if issues[i].Line != issues[j].Line {
			return issues[i].Line < issues[j].Line
		}
		return issues[i].Field < issues[j].Field
	})
	return issues
}

// isExported returns true when the identifier name starts with an upper-case letter.
func isExported(name string) bool {
	if name == "" {
		return false
	}
	return unicode.IsUpper([]rune(name)[0])
}

// isMalformed returns true when rawTag cannot be interpreted as a valid
// key:"value" struct tag sequence.
//
// A valid struct tag is a sequence of optionally space-separated key:"value"
// pairs (per the reflect package). We do a minimal validation: each token
// must match `key:"..."`.
func isMalformed(raw string) bool {
	s := strings.TrimSpace(raw)
	if s == "" {
		return false
	}
	// Walk through the tag the same way reflect does internally.
	for s != "" {
		// Skip leading spaces.
		i := 0
		for i < len(s) && s[i] == ' ' {
			i++
		}
		s = s[i:]
		if s == "" {
			break
		}

		// Key must be non-empty and consist of non-control, non-space, non-quote chars.
		i = 0
		for i < len(s) && s[i] != ' ' && s[i] != ':' && s[i] != '"' {
			i++
		}
		if i == 0 {
			return true // empty key
		}
		// After key, expect :"
		if i >= len(s) || s[i] != ':' {
			return true
		}
		s = s[i+1:]
		if s == "" || s[0] != '"' {
			return true
		}
		// Consume the quoted value.
		s = s[1:]
		i = 0
		for i < len(s) && s[i] != '"' {
			if s[i] == '\\' {
				i++ // skip escaped char
			}
			i++
		}
		if i >= len(s) {
			return true // unterminated quote
		}
		s = s[i+1:]
	}
	return false
}
