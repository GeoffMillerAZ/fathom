package extractor

import (
	"sort"

	ts "github.com/tree-sitter/go-tree-sitter"
	tsjava "github.com/tree-sitter/tree-sitter-java/bindings/go"

	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/extractor/treesitter"
)

// javaLang is initialised once and shared across all JavaExtractor calls.
// tree-sitter Language objects are immutable and safe for concurrent use;
// only the Parser (created fresh per call) carries mutable state.
var javaLang = ts.NewLanguage(tsjava.Language())

// JavaExtractor implements Extractor for Java source files using
// tree-sitter CGO bindings (ADR-013 — stays on this engine deliberately;
// ADR-021 §4's CGO->wazero migration for the remaining incumbents is XT-6b,
// a separate, not-yet-triggered slice). It emits:
//
//   - KindJavaClass for class and static nested class declarations.
//   - KindJavaInterface for interface declarations.
//   - KindJavaEnum for enum declarations.
//   - KindJavaRecord for record declarations.
//   - KindJavaMethod for method declarations and constructor declarations,
//     qualified as "EnclosingType.methodName".
//   - KindJavaField for field declarations (one symbol per declarator, so
//     "int a, b;" emits two), qualified as "EnclosingType.fieldName" (wave
//     P2, O81).
//   - KindJavaImport for import declarations (static and wildcard included;
//     a wildcard import's "*" is dropped, e.g. "import java.util.*;" emits
//     "java.util" — mirrors kotlin.scm's import handling), unqualified
//     (wave P2, O81).
//
// All symbols carry TierVerified.
type JavaExtractor struct{}

// NewJavaExtractor returns a new JavaExtractor.
func NewJavaExtractor() *JavaExtractor { return &JavaExtractor{} }

// Extract parses Java source bytes with tree-sitter and returns SymbolNodes
// for classes, interfaces, enums, records, methods, and constructors. Output
// is sorted by Line then Name for determinism.
func (j *JavaExtractor) Extract(service, relPath string, src []byte) ([]core.SymbolNode, error) {
	nodes, err := j.extract(service, relPath, src, nil)
	return nodes, err
}

// SpecLang is the file-resolve language ExtractWithFileSpecs reads specs for.
func (j *JavaExtractor) SpecLang() string { return "java" }

// ExtractWithFileSpecs is Extract plus the file specs the dialect's queries
// read from the same native tree (EW-53): no second parse.
func (j *JavaExtractor) ExtractWithFileSpecs(service, relPath string, src []byte) ([]core.SymbolNode, FileSpecs, bool, error) {
	var out nativeSpecsOut
	nodes, err := j.extract(service, relPath, src, &out)
	return nodes, out.specs, out.ok && err == nil, err
}

func (j *JavaExtractor) extract(service, relPath string, src []byte, out *nativeSpecsOut) ([]core.SymbolNode, error) {
	var nodes []core.SymbolNode

	pkg := pkgFromPath(relPath)

	err := treesitter.WithTree(javaLang, src, func(root *ts.Node) {
		out.take("java", javaLang, root, relPath, src)
		walkJava(root, src, service, pkg, relPath, nil, &nodes)
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

// walkJava recursively descends the tree-sitter CST.
//
// scopeStack tracks the qualified names of enclosing type declarations (class,
// interface, enum, record) so that nested types and methods can be qualified
// correctly (e.g., "Outer.Inner", "Outer.method").
func walkJava(
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
	case "class_declaration", "interface_declaration", "enum_declaration", "record_declaration":
		name := treesitter.FieldText(n, "name", src)
		if name == "" {
			break
		}
		// Qualify with enclosing type for nested declarations.
		qualName := qualifyJava(scopeStack, name)

		symKind := javaTypeKind(kind)
		*out = append(*out, core.SymbolNode{
			Service:  service,
			Package:  pkg,
			Name:     qualName,
			Kind:     symKind,
			FilePath: relPath,
			Line:     treesitter.Line(n),
			EndLine:  treesitter.EndLine(n),
			Tier:     core.TierVerified,
		})

		// Recurse into the body with this type name pushed onto the scope stack.
		newScope := append(append([]string{}, scopeStack...), qualName)
		body := typeBodyNode(n, kind)
		if body != nil {
			walkJava(body, src, service, pkg, relPath, newScope, out)
		}
		return

	case "method_declaration", "constructor_declaration":
		name := treesitter.FieldText(n, "name", src)
		if name == "" {
			break
		}
		qualName := qualifyJava(scopeStack, name)
		*out = append(*out, core.SymbolNode{
			Service:  service,
			Package:  pkg,
			Name:     qualName,
			Kind:     core.KindJavaMethod,
			FilePath: relPath,
			Line:     treesitter.Line(n),
			EndLine:  treesitter.EndLine(n),
			Tier:     core.TierVerified,
			Calls:    treesitter.CollectCalls(n, src, map[string]string{"method_invocation": "name"}),
		})
		// Do not recurse into method/constructor bodies — inner types there
		// are unusual and not worth complicating the scope.
		return

	case "field_declaration":
		// The grammar's _variable_declarator_list is commaSep1(field
		// ("declarator", variable_declarator)) — "int a, b;" has TWO
		// variable_declarator children sharing the same field name, so we
		// walk named children by KIND rather than ChildByFieldName (which
		// only ever returns the first match for a repeated field).
		count := n.NamedChildCount()
		for i := uint(0); i < count; i++ {
			child := n.NamedChild(i)
			if child == nil || child.Kind() != "variable_declarator" {
				continue
			}
			name := treesitter.FieldText(child, "name", src)
			if name == "" {
				continue
			}
			*out = append(*out, core.SymbolNode{
				Service:  service,
				Package:  pkg,
				Name:     qualifyJava(scopeStack, name),
				Kind:     core.KindJavaField,
				FilePath: relPath,
				Line:     treesitter.Line(n),
				EndLine:  treesitter.EndLine(n),
				Tier:     core.TierVerified,
			})
		}
		return

	case "import_declaration":
		// The imported name ($._name: identifier | scoped_identifier) is
		// NOT field-tagged at the import_declaration level — it is always
		// the first named child, whether or not a trailing ".*" wildcard
		// follows (the wildcard's "asterisk" node, if present, is a later
		// named sibling).
		nameNode := n.NamedChild(0)
		if nameNode == nil {
			return
		}
		name := nameNode.Utf8Text(src)
		if name == "" {
			return
		}
		*out = append(*out, core.SymbolNode{
			Service:  service,
			Package:  pkg,
			Name:     name,
			Kind:     core.KindJavaImport,
			FilePath: relPath,
			Line:     treesitter.Line(n),
			EndLine:  treesitter.EndLine(n),
			Tier:     core.TierVerified,
		})
		return
	}

	// Generic descent for all other node types (program, class_body, block, …).
	count := n.NamedChildCount()
	for i := uint(0); i < count; i++ {
		child := n.NamedChild(i)
		walkJava(child, src, service, pkg, relPath, scopeStack, out)
	}
}

// javaTypeKind maps a tree-sitter node kind to the appropriate core.SourceKind.
func javaTypeKind(nodeKind string) core.SourceKind {
	switch nodeKind {
	case "interface_declaration":
		return core.KindJavaInterface
	case "enum_declaration":
		return core.KindJavaEnum
	case "record_declaration":
		return core.KindJavaRecord
	default: // "class_declaration"
		return core.KindJavaClass
	}
}

// typeBodyNode returns the body child of a Java type declaration. The field
// name differs by node kind in the tree-sitter-java grammar.
func typeBodyNode(n *ts.Node, nodeKind string) *ts.Node {
	switch nodeKind {
	case "interface_declaration":
		return n.ChildByFieldName("body") // interface_body
	case "enum_declaration":
		return n.ChildByFieldName("body") // enum_body
	default:
		return n.ChildByFieldName("body") // class_body
	}
}

// qualifyJava returns "Outer.name" when scopeStack is non-empty, or just name
// at top level. The outermost scope entry is already a fully qualified name.
func qualifyJava(scopeStack []string, name string) string {
	if len(scopeStack) == 0 {
		return name
	}
	return scopeStack[len(scopeStack)-1] + "." + name
}
