package extractor

import (
	"sort"

	ts "github.com/tree-sitter/go-tree-sitter"
	tsc "github.com/tree-sitter/tree-sitter-c/bindings/go"

	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/extractor/treesitter"
)

// cLang is initialised once and shared across all CExtractor calls.
// tree-sitter Language objects are immutable and safe for concurrent use;
// only the Parser (created fresh per call) carries mutable state.
var cLang = ts.NewLanguage(tsc.Language())

// CExtractor implements Extractor for C source files (.c, .h) using
// tree-sitter CGO bindings (ADR-013). It emits:
//
//   - KindCFunc for function definitions (name resolved via cDeclName).
//   - KindCStruct for struct specifiers with a body (struct Foo { … }).
//   - KindCEnum for enum specifiers with a body (enum Color { … }).
//   - KindCTypedef for typedef declarations (the typedef'd alias name).
//
// Bare declaration prototypes (no body) are skipped to avoid noise.
// All symbols carry TierVerified.
type CExtractor struct{}

// NewCExtractor returns a new CExtractor.
func NewCExtractor() *CExtractor { return &CExtractor{} }

// Extract parses C source bytes with tree-sitter and returns SymbolNodes.
// Output is sorted by Line then Name for determinism.
func (c *CExtractor) Extract(service, relPath string, src []byte) ([]core.SymbolNode, error) {
	var nodes []core.SymbolNode

	pkg := pkgFromPath(relPath)

	err := treesitter.WithTree(cLang, src, func(root *ts.Node) {
		walkC(root, src, service, pkg, relPath, &nodes)
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

// walkC recursively descends the tree-sitter CST for C source.
func walkC(
	n *ts.Node,
	src []byte,
	service, pkg, relPath string,
	out *[]core.SymbolNode,
) {
	if n == nil {
		return
	}

	switch n.Kind() {
	case "function_definition":
		decl := n.ChildByFieldName("declarator")
		name := cDeclName(decl, src)
		if name != "" {
			*out = append(*out, core.SymbolNode{
				Service:  service,
				Package:  pkg,
				Name:     name,
				Kind:     core.KindCFunc,
				FilePath: relPath,
				Line:     treesitter.Line(n),
				Tier:     core.TierVerified,
				Calls:    treesitter.CollectCalls(n, src, map[string]string{"call_expression": "function"}),
			})
		}
		// Do not recurse into function bodies — local structs/enums are unusual
		// and not worth complicating the model.
		return

	case "struct_specifier":
		// Only emit structs that have a body (field_declaration_list); forward
		// declarations like "struct Foo;" are skipped.
		if n.ChildByFieldName("body") == nil {
			return
		}
		name := treesitter.FieldText(n, "name", src)
		if name != "" {
			*out = append(*out, core.SymbolNode{
				Service:  service,
				Package:  pkg,
				Name:     name,
				Kind:     core.KindCStruct,
				FilePath: relPath,
				Line:     treesitter.Line(n),
				Tier:     core.TierVerified,
			})
		}
		// Do not descend into the struct body.
		return

	case "enum_specifier":
		// Only emit enums that have a body (enumerator_list).
		if n.ChildByFieldName("body") == nil {
			return
		}
		name := treesitter.FieldText(n, "name", src)
		if name != "" {
			*out = append(*out, core.SymbolNode{
				Service:  service,
				Package:  pkg,
				Name:     name,
				Kind:     core.KindCEnum,
				FilePath: relPath,
				Line:     treesitter.Line(n),
				Tier:     core.TierVerified,
			})
		}
		return

	case "type_definition":
		// The "declarator" field holds the alias name introduced by typedef.
		// For simple typedefs (typedef int MyInt; typedef struct Foo { } Foo_t;)
		// the declarator is a type_identifier whose text is the alias.
		// For complex typedefs (function pointers etc.) the declarator is a
		// function_declarator or pointer_declarator; we still attempt to resolve
		// a name via cDeclName so that "typedef void (*Cb)(int);" yields "Cb".
		decl := n.ChildByFieldName("declarator")
		if decl == nil {
			return
		}
		name := ""
		if decl.Kind() == "type_identifier" {
			name = decl.Utf8Text(src)
		} else {
			name = cDeclName(decl, src)
		}
		if name != "" {
			*out = append(*out, core.SymbolNode{
				Service:  service,
				Package:  pkg,
				Name:     name,
				Kind:     core.KindCTypedef,
				FilePath: relPath,
				Line:     treesitter.Line(n),
				Tier:     core.TierVerified,
			})
		}
		// Also recurse into the underlying type in case it is an anonymous struct
		// or enum with a body (so the struct body symbols are NOT emitted here —
		// the struct is already named by the typedef).
		return
	}

	// Generic descent for all other node types.
	count := n.NamedChildCount()
	for i := uint(0); i < count; i++ {
		child := n.NamedChild(i)
		walkC(child, src, service, pkg, relPath, out)
	}
}

// cDeclName resolves the function or variable name from a declarator node.
// In the C/C++ tree-sitter grammar, function_definition does not have a direct
// "name" field; the name is nested inside the declarator chain:
//
//	function_definition
//	  declarator → function_declarator
//	                 declarator → identifier          ("foo")
//	                 declarator → pointer_declarator
//	                                declarator → function_declarator
//	                                               declarator → identifier
//
// This helper descends through:
//
//   - function_declarator → its "declarator" child
//   - pointer_declarator  → its "declarator" child
//   - reference_declarator → its "declarator" child
//   - parenthesized_declarator → its "declarator" child
//
// until it reaches a terminal leaf:
//
//   - identifier / field_identifier → UTF-8 text
//   - qualified_identifier → UTF-8 text (e.g. "MyClass::method")
//   - destructor_name → UTF-8 text (e.g. "~MyClass")
//   - operator_name → UTF-8 text (e.g. "operator=")
//   - type_identifier → UTF-8 text (typedef alias)
func cDeclName(n *ts.Node, src []byte) string {
	for n != nil {
		switch n.Kind() {
		case "identifier", "field_identifier", "qualified_identifier",
			"destructor_name", "operator_name", "type_identifier",
			"namespace_identifier":
			return n.Utf8Text(src)
		case "function_declarator", "pointer_declarator", "reference_declarator":
			n = n.ChildByFieldName("declarator")
		case "parenthesized_declarator":
			// parenthesized_declarator wraps its inner declarator as a positional
			// (non-field) child, not via the "declarator" field.
			// e.g. "(*Callback)" → first named child is pointer_declarator.
			if n.NamedChildCount() > 0 {
				n = n.NamedChild(0)
			} else {
				return ""
			}
		default:
			// Unknown declarator kind — stop.
			return ""
		}
	}
	return ""
}
