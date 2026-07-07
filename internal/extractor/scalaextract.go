package extractor

import (
	"sort"

	ts "github.com/tree-sitter/go-tree-sitter"
	tsscala "github.com/tree-sitter/tree-sitter-scala/bindings/go"

	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/extractor/treesitter"
)

// scalaLang is initialised once and shared across all ScalaExtractor calls.
// tree-sitter Language objects are immutable and safe for concurrent use;
// only the Parser (created fresh per call) carries mutable state.
var scalaLang = ts.NewLanguage(tsscala.Language())

// ScalaExtractor implements Extractor for Scala source files using
// tree-sitter CGO bindings (ADR-013). It emits:
//
//   - KindScalaClass for class definitions.
//   - KindScalaObject for object definitions.
//   - KindScalaTrait for trait definitions.
//   - KindScalaDef for function/method definitions (qualified as
//     "Owner.method" when inside a class, object, or trait body).
//
// All symbols carry TierVerified.
type ScalaExtractor struct{}

// NewScalaExtractor returns a new ScalaExtractor.
func NewScalaExtractor() *ScalaExtractor { return &ScalaExtractor{} }

// Extract parses src with tree-sitter and returns SymbolNodes for classes,
// objects, traits, and function definitions. Output is sorted by Line then
// Name for determinism.
func (s *ScalaExtractor) Extract(service, relPath string, src []byte) ([]core.SymbolNode, error) {
	var nodes []core.SymbolNode

	pkg := pkgFromPath(relPath)

	err := treesitter.WithTree(scalaLang, src, func(root *ts.Node) {
		walkScala(root, src, service, pkg, relPath, "", &nodes)
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

// walkScala recursively descends the tree-sitter CST.
//
// scopeName is the name of the nearest enclosing class/object/trait ("" at
// top level). Methods inside the owner are qualified as "Owner.method".
func walkScala(
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
	case "class_definition":
		name := treesitter.FieldText(n, "name", src)
		if name != "" {
			*out = append(*out, core.SymbolNode{
				Service:  service,
				Package:  pkg,
				Name:     name,
				Kind:     core.KindScalaClass,
				FilePath: relPath,
				Line:     treesitter.Line(n),
				Tier:     core.TierVerified,
			})
		}
		// Recurse into the class body with this class as the scope.
		body := n.ChildByFieldName("body")
		if body != nil {
			walkScala(body, src, service, pkg, relPath, name, out)
		}
		return

	case "object_definition":
		name := treesitter.FieldText(n, "name", src)
		if name != "" {
			*out = append(*out, core.SymbolNode{
				Service:  service,
				Package:  pkg,
				Name:     name,
				Kind:     core.KindScalaObject,
				FilePath: relPath,
				Line:     treesitter.Line(n),
				Tier:     core.TierVerified,
			})
		}
		body := n.ChildByFieldName("body")
		if body != nil {
			walkScala(body, src, service, pkg, relPath, name, out)
		}
		return

	case "trait_definition":
		name := treesitter.FieldText(n, "name", src)
		if name != "" {
			*out = append(*out, core.SymbolNode{
				Service:  service,
				Package:  pkg,
				Name:     name,
				Kind:     core.KindScalaTrait,
				FilePath: relPath,
				Line:     treesitter.Line(n),
				Tier:     core.TierVerified,
			})
		}
		body := n.ChildByFieldName("body")
		if body != nil {
			walkScala(body, src, service, pkg, relPath, name, out)
		}
		return

	case "function_definition":
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
				Kind:     core.KindScalaDef,
				FilePath: relPath,
				Line:     treesitter.Line(n),
				Tier:     core.TierVerified,
				Calls:    treesitter.CollectCalls(n, src, map[string]string{"call_expression": "function"}),
			})
		}
		// Do not recurse into function bodies — inner defs carry no useful scope.
		return
	}

	// Generic descent for all other node types.
	count := n.NamedChildCount()
	for i := uint(0); i < count; i++ {
		child := n.NamedChild(i)
		walkScala(child, src, service, pkg, relPath, scopeName, out)
	}
}
