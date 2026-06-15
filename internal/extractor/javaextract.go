package extractor

import (
	"bufio"
	"bytes"
	"regexp"
	"sort"
	"strings"

	"github.com/geoffmilleraz/sextant/internal/core"
)

// JavaExtractor implements Extractor for Java source files using line-based
// heuristics. It detects:
//   - class declarations → KindJavaClass
//   - interface declarations → KindJavaInterface
//   - method declarations (heuristic; inner classes best-effort) → KindJavaMethod
//
// Method names are qualified as "ClassName.methodName" using a class-stack
// that tracks brace depth.
type JavaExtractor struct{}

// NewJavaExtractor returns a new JavaExtractor.
func NewJavaExtractor() *JavaExtractor { return &JavaExtractor{} }

var (
	// javaClassDecl matches class/interface/enum/record declarations (simplified).
	javaClassRe = regexp.MustCompile(
		`(?:^|\s)(?:public|private|protected|static|abstract|final|sealed|non-sealed)?\s*` +
			`(?:public|private|protected|static|abstract|final|sealed|non-sealed)?\s*` +
			`(class|interface|enum|record)\s+([A-Za-z_][A-Za-z0-9_]*)`)

	// javaMethodRe matches method declarations. Simplified: return-type name(
	// where return-type is a word and name is followed by '('.
	javaMethodRe = regexp.MustCompile(
		`^\s+(?:(?:public|private|protected|static|final|synchronized|native|abstract|default|override|@\w+)\s+)*` +
			`(?:<[^>]*>\s*)?` +
			`(?:[A-Za-z_][A-Za-z0-9_<>\[\],\s]*\s+)` +
			`([A-Za-z_][A-Za-z0-9_]*)\s*\(`)
)

// Extract parses Java source bytes and returns SymbolNodes.
func (j *JavaExtractor) Extract(service, relPath string, src []byte) ([]core.SymbolNode, error) {
	var nodes []core.SymbolNode

	scanner := bufio.NewScanner(bytes.NewReader(src))
	lineNo := 0

	type classFrame struct {
		name       string
		kind       core.SourceKind
		braceDepth int
	}
	var classStack []classFrame
	braceDepth := 0
	pkg := pkgFromPath(relPath)

	for scanner.Scan() {
		lineNo++
		line := scanner.Text()
		stripped := strings.TrimSpace(line)

		if stripped == "" || strings.HasPrefix(stripped, "//") || strings.HasPrefix(stripped, "*") {
			continue
		}

		// Update brace depth and pop class frames.
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

		// Class / interface / enum / record declaration.
		if m := javaClassRe.FindStringSubmatch(line); m != nil {
			keyword := m[1]
			name := m[2]
			// Qualify with enclosing class for inner classes.
			if len(classStack) > 0 {
				name = classStack[len(classStack)-1].name + "." + name
			}
			kind := core.KindJavaClass
			if keyword == "interface" {
				kind = core.KindJavaInterface
			}
			nodes = append(nodes, core.SymbolNode{
				Service:  service,
				Package:  pkg,
				Name:     name,
				Kind:     kind,
				FilePath: relPath,
				Line:     lineNo,
				Tier:     core.TierPreScan,
			})
			classStack = append(classStack, classFrame{name: name, kind: kind, braceDepth: braceDepth})
			continue
		}

		// Method declaration (must be inside a class body).
		if len(classStack) > 0 && braceDepth > classStack[len(classStack)-1].braceDepth {
			if m := javaMethodRe.FindStringSubmatch(line); m != nil {
				methodName := m[1]
				if isJavaKeyword(methodName) {
					continue
				}
				qualName := classStack[len(classStack)-1].name + "." + methodName
				nodes = append(nodes, core.SymbolNode{
					Service:  service,
					Package:  pkg,
					Name:     qualName,
					Kind:     core.KindJavaMethod,
					FilePath: relPath,
					Line:     lineNo,
					Tier:     core.TierPreScan,
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

// isJavaKeyword reports whether s is a Java keyword to filter false positives.
func isJavaKeyword(s string) bool {
	switch s {
	case "if", "for", "while", "switch", "return", "new", "throw",
		"try", "catch", "finally", "else", "do", "super", "this",
		"import", "package", "class", "interface", "enum", "record",
		"extends", "implements", "static", "final", "abstract",
		"public", "private", "protected", "void", "boolean", "int",
		"long", "double", "float", "byte", "short", "char":
		return true
	}
	return false
}
