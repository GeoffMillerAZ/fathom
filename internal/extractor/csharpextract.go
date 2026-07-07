package extractor

import (
	"sort"

	ts "github.com/tree-sitter/go-tree-sitter"
	tscsharp "github.com/tree-sitter/tree-sitter-c-sharp/bindings/go"

	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/extractor/treesitter"
)

// csharpLang is initialised once and shared across all CSharpExtractor calls.
// tree-sitter Language objects are immutable and safe for concurrent use;
// only the Parser (created fresh per call) carries mutable state.
var csharpLang = ts.NewLanguage(tscsharp.Language())

// CSharpExtractor implements Extractor for C# source files using
// tree-sitter CGO bindings (ADR-013). It emits:
//
//   - KindCSharpNamespace for namespace declarations.
//   - KindCSharpClass for class and record declarations.
//   - KindCSharpInterface for interface declarations.
//   - KindCSharpStruct for struct declarations.
//   - KindCSharpEnum for enum declarations.
//   - KindCSharpMethod for method and constructor declarations, qualified
//     as "EnclosingType.method" when inside a type declaration.
//
// All symbols carry TierVerified.
type CSharpExtractor struct{}

// NewCSharpExtractor returns a new CSharpExtractor.
func NewCSharpExtractor() *CSharpExtractor { return &CSharpExtractor{} }

// Extract parses C# source bytes with tree-sitter and returns SymbolNodes
// for namespaces, classes, interfaces, structs, enums, methods, and
// constructors. Output is sorted by Line then Name for determinism.
func (c *CSharpExtractor) Extract(service, relPath string, src []byte) ([]core.SymbolNode, error) {
	var nodes []core.SymbolNode

	pkg := pkgFromPath(relPath)

	err := treesitter.WithTree(csharpLang, src, func(root *ts.Node) {
		walkCSharp(root, src, service, pkg, relPath, nil, &nodes)
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

// walkCSharp recursively descends the tree-sitter CST.
//
// scopeStack tracks the qualified names of enclosing type declarations
// (class, interface, struct, enum, record) so that nested types and methods
// can be qualified correctly (e.g., "Outer.Inner", "Outer.method").
func walkCSharp(
	n *ts.Node,
	src []byte,
	service, pkg, relPath string,
	scopeStack []string,
	out *[]core.SymbolNode,
) {
	if n == nil {
		return
	}

	kind := n.Kind()

	switch kind {
	case "namespace_declaration", "file_scoped_namespace_declaration":
		name := treesitter.FieldText(n, "name", src)
		if name != "" {
			*out = append(*out, core.SymbolNode{
				Service:  service,
				Package:  pkg,
				Name:     name,
				Kind:     core.KindCSharpNamespace,
				FilePath: relPath,
				Line:     treesitter.Line(n),
				Tier:     core.TierVerified,
			})
		}
		// Recurse into namespace body; namespace does not push onto type scope.
		body := n.ChildByFieldName("body")
		if body != nil {
			walkCSharp(body, src, service, pkg, relPath, scopeStack, out)
		}
		return

	case "class_declaration", "record_declaration":
		name := treesitter.FieldText(n, "name", src)
		if name == "" {
			break
		}
		qualName := qualifyCSharp(scopeStack, name)
		*out = append(*out, core.SymbolNode{
			Service:  service,
			Package:  pkg,
			Name:     qualName,
			Kind:     core.KindCSharpClass,
			FilePath: relPath,
			Line:     treesitter.Line(n),
			Tier:     core.TierVerified,
		})
		// Recurse into body with this type name pushed onto the scope stack.
		newScope := append(append([]string{}, scopeStack...), qualName)
		body := n.ChildByFieldName("body")
		if body != nil {
			walkCSharp(body, src, service, pkg, relPath, newScope, out)
		}
		return

	case "interface_declaration":
		name := treesitter.FieldText(n, "name", src)
		if name == "" {
			break
		}
		qualName := qualifyCSharp(scopeStack, name)
		*out = append(*out, core.SymbolNode{
			Service:  service,
			Package:  pkg,
			Name:     qualName,
			Kind:     core.KindCSharpInterface,
			FilePath: relPath,
			Line:     treesitter.Line(n),
			Tier:     core.TierVerified,
		})
		// Recurse into body with the interface name on the scope stack.
		newScope := append(append([]string{}, scopeStack...), qualName)
		body := n.ChildByFieldName("body")
		if body != nil {
			walkCSharp(body, src, service, pkg, relPath, newScope, out)
		}
		return

	case "struct_declaration":
		name := treesitter.FieldText(n, "name", src)
		if name == "" {
			break
		}
		qualName := qualifyCSharp(scopeStack, name)
		*out = append(*out, core.SymbolNode{
			Service:  service,
			Package:  pkg,
			Name:     qualName,
			Kind:     core.KindCSharpStruct,
			FilePath: relPath,
			Line:     treesitter.Line(n),
			Tier:     core.TierVerified,
		})
		newScope := append(append([]string{}, scopeStack...), qualName)
		body := n.ChildByFieldName("body")
		if body != nil {
			walkCSharp(body, src, service, pkg, relPath, newScope, out)
		}
		return

	case "enum_declaration":
		name := treesitter.FieldText(n, "name", src)
		if name == "" {
			break
		}
		qualName := qualifyCSharp(scopeStack, name)
		*out = append(*out, core.SymbolNode{
			Service:  service,
			Package:  pkg,
			Name:     qualName,
			Kind:     core.KindCSharpEnum,
			FilePath: relPath,
			Line:     treesitter.Line(n),
			Tier:     core.TierVerified,
		})
		// Do not recurse into enum bodies — enum member declarations are not
		// symbols we extract at this level.
		return

	case "method_declaration", "constructor_declaration":
		name := treesitter.FieldText(n, "name", src)
		if name == "" {
			break
		}
		qualName := qualifyCSharp(scopeStack, name)
		*out = append(*out, core.SymbolNode{
			Service:  service,
			Package:  pkg,
			Name:     qualName,
			Kind:     core.KindCSharpMethod,
			FilePath: relPath,
			Line:     treesitter.Line(n),
			Tier:     core.TierVerified,
			Calls:    treesitter.CollectCalls(n, src, map[string]string{"invocation_expression": "function"}),
		})
		// Do not recurse into method/constructor bodies — inner types there
		// are unusual and not worth complicating the scope.
		return
	}

	// Generic descent for all other node types (compilation_unit, declaration_list, …).
	count := n.NamedChildCount()
	for i := uint(0); i < count; i++ {
		child := n.NamedChild(i)
		walkCSharp(child, src, service, pkg, relPath, scopeStack, out)
	}
}

// qualifyCSharp returns "Outer.name" when scopeStack is non-empty, or just name
// at top level. The outermost scope entry is already a fully qualified name.
func qualifyCSharp(scopeStack []string, name string) string {
	if len(scopeStack) == 0 {
		return name
	}
	return scopeStack[len(scopeStack)-1] + "." + name
}
