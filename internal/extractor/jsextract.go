package extractor

import (
	"bufio"
	"bytes"
	"regexp"
	"sort"
	"strings"

	"github.com/geoffmilleraz/sextant/internal/core"
)

// JSExtractor implements Extractor for JavaScript, TypeScript, JSX, and TSX
// source files. It uses line-based heuristics to find:
//   - function declarations: function Foo(...)
//   - class declarations: class Foo / class Foo extends Bar
//   - class method definitions: methodName(...) { inside a class body
//   - exported const arrow functions: export const Foo = (...) =>
//
// It is deterministic, uses only stdlib regexp, and is safe for concurrent use.
type JSExtractor struct{}

// NewJSExtractor returns a new JSExtractor.
func NewJSExtractor() *JSExtractor { return &JSExtractor{} }

var (
	jsFuncDecl   = regexp.MustCompile(`^\s*(?:export\s+)?(?:async\s+)?function\s+([A-Za-z_$][A-Za-z0-9_$]*)`)
	jsClassDecl  = regexp.MustCompile(`^\s*(?:export\s+)?(?:default\s+)?(?:abstract\s+)?class\s+([A-Za-z_$][A-Za-z0-9_$]*)`)
	jsMethodDecl = regexp.MustCompile(`^\s+(?:(?:public|private|protected|static|async|override|readonly|abstract)\s+)*([A-Za-z_$][A-Za-z0-9_$]*)\s*(?:<[^>]*>)?\s*\(`)
	// jsArrowExport matches: export const Name = (...) => or export const Name = async (...) =>
	// It allows TypeScript return-type annotations between ) and =>.
	jsArrowExport = regexp.MustCompile(`^\s*export\s+const\s+([A-Za-z_$][A-Za-z0-9_$]*)\s*=\s*(?:async\s+)?(?:\([^)]*\)|[A-Za-z_$][A-Za-z0-9_$]*)(?:\s*:\s*[^=]+)?\s*=>`)
)

// Extract parses JS/TS/JSX/TSX source and returns SymbolNodes.
// Output is sorted by Line then Name.
func (j *JSExtractor) Extract(service, relPath string, src []byte) ([]core.SymbolNode, error) {
	var nodes []core.SymbolNode

	scanner := bufio.NewScanner(bytes.NewReader(src))
	lineNo := 0

	// Track current class context for method qualification.
	type classFrame struct {
		name       string
		braceDepth int // brace depth at which the class body opened
	}
	var classStack []classFrame
	braceDepth := 0

	pkg := pkgFromPath(relPath)

	for scanner.Scan() {
		lineNo++
		line := scanner.Text()
		stripped := strings.TrimSpace(line)

		// Track brace depth to know when we've exited a class body.
		for _, ch := range line {
			switch ch {
			case '{':
				braceDepth++
			case '}':
				braceDepth--
				for len(classStack) > 0 && classStack[len(classStack)-1].braceDepth > braceDepth {
					classStack = classStack[:len(classStack)-1]
				}
			}
		}

		if stripped == "" || strings.HasPrefix(stripped, "//") || strings.HasPrefix(stripped, "*") {
			continue
		}

		// class declaration
		if m := jsClassDecl.FindStringSubmatch(line); m != nil {
			name := m[1]
			nodes = append(nodes, core.SymbolNode{
				Service:  service,
				Package:  pkg,
				Name:     name,
				Kind:     core.KindJSClass,
				FilePath: relPath,
				Line:     lineNo,
				Tier:     core.TierVerified,
			})
			classStack = append(classStack, classFrame{name: name, braceDepth: braceDepth})
			continue
		}

		// exported const arrow function
		if m := jsArrowExport.FindStringSubmatch(line); m != nil {
			name := m[1]
			nodes = append(nodes, core.SymbolNode{
				Service:  service,
				Package:  pkg,
				Name:     name,
				Kind:     core.KindJSFunc,
				FilePath: relPath,
				Line:     lineNo,
				Tier:     core.TierVerified,
			})
			continue
		}

		// function declaration
		if m := jsFuncDecl.FindStringSubmatch(line); m != nil {
			name := m[1]
			nodes = append(nodes, core.SymbolNode{
				Service:  service,
				Package:  pkg,
				Name:     name,
				Kind:     core.KindJSFunc,
				FilePath: relPath,
				Line:     lineNo,
				Tier:     core.TierVerified,
			})
			continue
		}

		// method inside a class body (must be inside a classStack frame)
		if len(classStack) > 0 && braceDepth > classStack[len(classStack)-1].braceDepth {
			if m := jsMethodDecl.FindStringSubmatch(line); m != nil {
				methodName := m[1]
				// Skip keywords that look like method names.
				if isJSKeyword(methodName) {
					continue
				}
				qualName := classStack[len(classStack)-1].name + "." + methodName
				nodes = append(nodes, core.SymbolNode{
					Service:  service,
					Package:  pkg,
					Name:     qualName,
					Kind:     core.KindJSMethod,
					FilePath: relPath,
					Line:     lineNo,
					Tier:     core.TierVerified,
				})
			}
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

// isJSKeyword reports whether s is a JS/TS keyword that should not be treated
// as a method name when matched by the method regex.
// Note: "delete", "get", "set" are valid method names in classes, so they are
// not included here.
func isJSKeyword(s string) bool {
	switch s {
	case "if", "for", "while", "switch", "catch", "return", "new",
		"typeof", "instanceof", "import", "export", "class", "extends",
		"constructor", "static", "async", "await",
		"const", "let", "var", "function", "yield", "void",
		"throw", "try", "finally", "else", "case", "default", "break",
		"continue", "debugger", "in", "of", "do", "super", "this":
		return true
	}
	return false
}
