package extractor

import (
	"sort"
	"strings"

	ts "github.com/tree-sitter/go-tree-sitter"
	tspython "github.com/tree-sitter/tree-sitter-python/bindings/go"

	"github.com/geoffmilleraz/sextant/internal/core"
	"github.com/geoffmilleraz/sextant/internal/extractor/treesitter"
)

// pyLang is initialised once and shared across all PythonExtractor calls.
// tree-sitter Language objects are immutable and safe for concurrent use;
// only the Parser (created fresh per call) carries mutable state.
var pyLang = ts.NewLanguage(tspython.Language())

// PythonExtractor implements Extractor for Python source files using
// tree-sitter CGO bindings (ADR-013). It emits:
//
//   - KindPyClass for class definitions.
//   - KindPyFunc for module-level functions, async functions, decorated
//     functions, and class methods (qualified as "ClassName.method").
//   - KindPyVar for module-level simple identifier assignments.
//
// All symbols carry TierVerified.
type PythonExtractor struct{}

// NewPythonExtractor returns a new PythonExtractor.
func NewPythonExtractor() *PythonExtractor { return &PythonExtractor{} }

// Extract parses src with tree-sitter and returns SymbolNodes for classes,
// functions/methods, and module-level variable assignments. Output is sorted
// by Line then Name for determinism.
func (p *PythonExtractor) Extract(service, relPath string, src []byte) ([]core.SymbolNode, error) {
	var nodes []core.SymbolNode

	pkg := pkgFromPath(relPath)

	err := treesitter.WithTree(pyLang, src, func(root *ts.Node) {
		// classScope tracks the nearest enclosing class name. An empty string
		// means we are at module level (or inside a function, not a class).
		walkPy(root, src, service, pkg, relPath, "", false, &nodes)
	})
	if err != nil {
		return nil, err
	}

	sort.Slice(nodes, func(i, j int) bool {
		if nodes[i].Line != nodes[j].Line {
			return nodes[i].Line < nodes[j].Line
		}
		return nodes[i].Name < nodes[j].Name
	})
	return nodes, nil
}

// walkPy recursively descends the tree-sitter CST.
//
//   - classScope: the name of the nearest enclosing class ("" at module level
//     or inside a function scope, so nested functions don't inherit class names).
//   - inFunction: true when we are inside a function body; suppresses
//     module-level variable emission for inner assignments.
func walkPy(
	n *ts.Node,
	src []byte,
	service, pkg, relPath string,
	classScope string,
	inFunction bool,
	out *[]core.SymbolNode,
) {
	if n == nil {
		return
	}

	kind := n.Kind()

	switch kind {
	case "class_definition":
		name := treesitter.FieldText(n, "name", src)
		if name != "" {
			*out = append(*out, core.SymbolNode{
				Service:  service,
				Package:  pkg,
				Name:     name,
				Kind:     core.KindPyClass,
				FilePath: relPath,
				Line:     treesitter.Line(n),
				Tier:     core.TierVerified,
			})
		}
		// Recurse into the class body with this class as the scope.
		body := n.ChildByFieldName("body")
		if body != nil {
			walkPy(body, src, service, pkg, relPath, name, false, out)
		}
		// Return: we've already walked the body above; don't fall through to
		// the generic child walk below.
		return

	case "function_definition":
		name := treesitter.FieldText(n, "name", src)
		if name != "" {
			symName := name
			if classScope != "" {
				symName = classScope + "." + name
			}
			*out = append(*out, core.SymbolNode{
				Service:  service,
				Package:  pkg,
				Name:     symName,
				Kind:     core.KindPyFunc,
				FilePath: relPath,
				Line:     treesitter.Line(n),
				Tier:     core.TierVerified,
				Calls:    treesitter.CollectCalls(n, src, map[string]string{"call": "function"}),
			})
		}
		// Recurse into the function body with NO class scope — nested functions
		// inside a method are not themselves methods.
		body := n.ChildByFieldName("body")
		if body != nil {
			walkPy(body, src, service, pkg, relPath, "", true, out)
		}
		return

	case "decorated_definition":
		// A decorated_definition wraps the actual class/function. Recurse so
		// the inner function_definition or class_definition is handled above.
		// We keep the same scope context (classScope, inFunction).

	case "expression_statement":
		// Module-level simple assignment: emit KindPyVar only at module scope
		// (not inside a function).
		if !inFunction && classScope == "" {
			// expression_statement has one named child; look for an assignment.
			child := n.NamedChild(0)
			if child != nil && child.Kind() == "assignment" {
				left := child.ChildByFieldName("left")
				if left != nil && left.Kind() == "identifier" {
					varName := left.Utf8Text(src)
					if varName != "" {
						*out = append(*out, core.SymbolNode{
							Service:  service,
							Package:  pkg,
							Name:     varName,
							Kind:     core.KindPyVar,
							FilePath: relPath,
							Line:     treesitter.Line(n),
							Tier:     core.TierVerified,
						})
					}
				}
			}
		}
		// No further descent needed for expression statements.
		return
	}

	// Generic descent for all other node types (module, block, decorated_definition, …).
	count := n.NamedChildCount()
	for i := uint(0); i < count; i++ {
		child := n.NamedChild(i)
		walkPy(child, src, service, pkg, relPath, classScope, inFunction, out)
	}
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
