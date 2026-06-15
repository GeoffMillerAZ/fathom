package extractor

import (
	"bufio"
	"bytes"
	"sort"
	"strings"

	"github.com/geoffmilleraz/sextant/internal/core"
)

// PythonExtractor implements Extractor for Python source files using
// deterministic regex/line heuristics (no tree-sitter or external deps).
// It emits KindPyFunc for module-level def, KindPyClass for class, and
// KindPyFunc (class-qualified name) for methods inside a class.
type PythonExtractor struct{}

// NewPythonExtractor returns a new PythonExtractor.
func NewPythonExtractor() *PythonExtractor { return &PythonExtractor{} }

// Extract parses Python source bytes and returns SymbolNodes for every
// class and function/method definition. Method names are qualified as
// "ClassName.method_name". Output is sorted by Line then Name.
func (p *PythonExtractor) Extract(service, relPath string, src []byte) ([]core.SymbolNode, error) {
	var nodes []core.SymbolNode

	scanner := bufio.NewScanner(bytes.NewReader(src))
	lineNo := 0

	// Track the current class context by its indentation level.
	// classStack entries: (indentLevel, className).
	type classFrame struct {
		indent int
		name   string
	}
	var classStack []classFrame

	for scanner.Scan() {
		lineNo++
		raw := scanner.Text()

		// Determine indentation of this line.
		indent := leadingSpaces(raw)
		trimmed := strings.TrimSpace(raw)

		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}

		// Pop class frames whose indent is >= current indent (we've left their body).
		for len(classStack) > 0 && classStack[len(classStack)-1].indent >= indent {
			classStack = classStack[:len(classStack)-1]
		}

		// class definition
		if strings.HasPrefix(trimmed, "class ") {
			name := extractPyIdentAfterKeyword(trimmed, "class ")
			if name != "" {
				nodes = append(nodes, core.SymbolNode{
					Service:  service,
					Package:  pkgFromPath(relPath),
					Name:     name,
					Kind:     core.KindPyClass,
					FilePath: relPath,
					Line:     lineNo,
					Tier:     core.TierPreScan,
				})
				classStack = append(classStack, classFrame{indent: indent, name: name})
			}
			continue
		}

		// function/method definition
		if strings.HasPrefix(trimmed, "def ") || strings.HasPrefix(trimmed, "async def ") {
			keyword := "def "
			if strings.HasPrefix(trimmed, "async ") {
				keyword = "async def "
			}
			name := extractPyIdentAfterKeyword(trimmed, keyword)
			if name == "" {
				continue
			}
			// Determine if this is a method (inside a class) or a module-level func.
			qualName := name
			if len(classStack) > 0 {
				qualName = classStack[len(classStack)-1].name + "." + name
			}
			nodes = append(nodes, core.SymbolNode{
				Service:  service,
				Package:  pkgFromPath(relPath),
				Name:     qualName,
				Kind:     core.KindPyFunc,
				FilePath: relPath,
				Line:     lineNo,
				Tier:     core.TierPreScan,
			})
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

// leadingSpaces counts the number of leading space characters (spaces + tabs
// where tab = 1 unit). Used only for relative indentation comparison.
func leadingSpaces(s string) int {
	count := 0
	for _, ch := range s {
		if ch == ' ' || ch == '\t' {
			count++
		} else {
			break
		}
	}
	return count
}

// extractPyIdentAfterKeyword returns the identifier that follows keyword in
// line. It strips everything after the first non-identifier rune (e.g. '(', ':').
func extractPyIdentAfterKeyword(line, keyword string) string {
	rest := strings.TrimPrefix(line, keyword)
	rest = strings.TrimSpace(rest)
	// Find end of identifier.
	end := 0
	for end < len(rest) {
		c := rest[end]
		if isIdentRune(c) {
			end++
		} else {
			break
		}
	}
	return rest[:end]
}

// isIdentRune reports whether c is a valid Python identifier character.
func isIdentRune(c byte) bool {
	return (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') ||
		(c >= '0' && c <= '9') || c == '_'
}

// pkgFromPath returns a simple package label derived from the file path
// (the base name without extension).
func pkgFromPath(relPath string) string {
	base := relPath
	// Find last slash.
	for i := len(relPath) - 1; i >= 0; i-- {
		if relPath[i] == '/' || relPath[i] == '\\' {
			base = relPath[i+1:]
			break
		}
	}
	// Strip extension.
	if dot := strings.LastIndex(base, "."); dot >= 0 {
		base = base[:dot]
	}
	return base
}
