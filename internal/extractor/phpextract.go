package extractor

import (
	"sort"

	ts "github.com/tree-sitter/go-tree-sitter"
	tsphp "github.com/tree-sitter/tree-sitter-php/bindings/go"

	"github.com/geoffmilleraz/sextant/internal/core"
	"github.com/geoffmilleraz/sextant/internal/extractor/treesitter"
)

// phpLang is initialised once and shared across all PHPExtractor calls.
// tree-sitter Language objects are immutable and safe for concurrent use;
// only the Parser (created fresh per call) carries mutable state.
// LanguagePHP() handles the full <?php ... ?> dialect.
var phpLang = ts.NewLanguage(tsphp.LanguagePHP())

// PHPExtractor implements Extractor for PHP source files using
// tree-sitter CGO bindings (ADR-013). It emits:
//
//   - KindPhpClass for class declarations.
//   - KindPhpInterface for interface declarations.
//   - KindPhpTrait for trait declarations.
//   - KindPhpEnum for enum declarations.
//   - KindPhpFunction for top-level function definitions.
//   - KindPhpMethod for method declarations inside a class/interface/trait
//     (qualified as "Owner.method").
//
// All symbols carry TierVerified.
type PHPExtractor struct{}

// NewPhpExtractor returns a new PHPExtractor.
func NewPhpExtractor() *PHPExtractor { return &PHPExtractor{} }

// Extract parses src with tree-sitter and returns SymbolNodes for PHP
// classes, interfaces, traits, enums, functions, and methods. Output is
// sorted by Line then Name for determinism.
func (e *PHPExtractor) Extract(service, relPath string, src []byte) ([]core.SymbolNode, error) {
	var nodes []core.SymbolNode

	pkg := pkgFromPath(relPath)

	err := treesitter.WithTree(phpLang, src, func(root *ts.Node) {
		walkPHP(root, src, service, pkg, relPath, "", &nodes)
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

// walkPHP recursively descends the tree-sitter CST.
//
// scopeName is the name of the nearest enclosing class/interface/trait ("" at
// top level). Methods inside the owner are qualified as "Owner.method".
func walkPHP(
	n *ts.Node,
	src []byte,
	service, pkg, relPath string,
	scopeName string,
	out *[]core.SymbolNode,
) {
	if n == nil {
		return
	}

	kind := n.Kind()

	switch kind {
	case "class_declaration":
		name := treesitter.FieldText(n, "name", src)
		if name != "" {
			*out = append(*out, core.SymbolNode{
				Service:  service,
				Package:  pkg,
				Name:     name,
				Kind:     core.KindPhpClass,
				FilePath: relPath,
				Line:     treesitter.Line(n),
				Tier:     core.TierVerified,
			})
		}
		// Recurse into the class body to pick up method_declaration nodes.
		body := n.ChildByFieldName("body")
		if body != nil {
			walkPHP(body, src, service, pkg, relPath, name, out)
		}
		return

	case "interface_declaration":
		name := treesitter.FieldText(n, "name", src)
		if name != "" {
			*out = append(*out, core.SymbolNode{
				Service:  service,
				Package:  pkg,
				Name:     name,
				Kind:     core.KindPhpInterface,
				FilePath: relPath,
				Line:     treesitter.Line(n),
				Tier:     core.TierVerified,
			})
		}
		body := n.ChildByFieldName("body")
		if body != nil {
			walkPHP(body, src, service, pkg, relPath, name, out)
		}
		return

	case "trait_declaration":
		name := treesitter.FieldText(n, "name", src)
		if name != "" {
			*out = append(*out, core.SymbolNode{
				Service:  service,
				Package:  pkg,
				Name:     name,
				Kind:     core.KindPhpTrait,
				FilePath: relPath,
				Line:     treesitter.Line(n),
				Tier:     core.TierVerified,
			})
		}
		body := n.ChildByFieldName("body")
		if body != nil {
			walkPHP(body, src, service, pkg, relPath, name, out)
		}
		return

	case "enum_declaration":
		name := treesitter.FieldText(n, "name", src)
		if name != "" {
			*out = append(*out, core.SymbolNode{
				Service:  service,
				Package:  pkg,
				Name:     name,
				Kind:     core.KindPhpEnum,
				FilePath: relPath,
				Line:     treesitter.Line(n),
				Tier:     core.TierVerified,
			})
		}
		// Do not recurse into enum bodies — enum cases are not methods.
		return

	case "function_definition":
		// Top-level function (not inside a class/interface/trait).
		name := treesitter.FieldText(n, "name", src)
		if name != "" && scopeName == "" {
			*out = append(*out, core.SymbolNode{
				Service:  service,
				Package:  pkg,
				Name:     name,
				Kind:     core.KindPhpFunction,
				FilePath: relPath,
				Line:     treesitter.Line(n),
				Tier:     core.TierVerified,
				Calls: treesitter.CollectCalls(n, src, map[string]string{
					"function_call_expression": "function",
					"member_call_expression":   "name",
					"scoped_call_expression":   "name",
				}),
			})
		}
		// Do not recurse into function bodies.
		return

	case "method_declaration":
		// Method inside a class/interface/trait.
		name := treesitter.FieldText(n, "name", src)
		if name != "" {
			symName := name
			if scopeName != "" {
				symName = scopeName + "." + name
			}
			*out = append(*out, core.SymbolNode{
				Service:  service,
				Package:  pkg,
				Name:     symName,
				Kind:     core.KindPhpMethod,
				FilePath: relPath,
				Line:     treesitter.Line(n),
				Tier:     core.TierVerified,
				Calls: treesitter.CollectCalls(n, src, map[string]string{
					"function_call_expression": "function",
					"member_call_expression":   "name",
					"scoped_call_expression":   "name",
				}),
			})
		}
		// Do not recurse into method bodies.
		return
	}

	// Generic descent for all other node types.
	count := n.NamedChildCount()
	for i := uint(0); i < count; i++ {
		child := n.NamedChild(i)
		walkPHP(child, src, service, pkg, relPath, scopeName, out)
	}
}
