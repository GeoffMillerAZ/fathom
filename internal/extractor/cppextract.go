package extractor

import (
	"sort"
	"strings"

	ts "github.com/tree-sitter/go-tree-sitter"
	tscpp "github.com/tree-sitter/tree-sitter-cpp/bindings/go"

	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/extractor/treesitter"
)

// cppLang is initialised once and shared across all CppExtractor calls.
// tree-sitter Language objects are immutable and safe for concurrent use;
// only the Parser (created fresh per call) carries mutable state.
var cppLang = ts.NewLanguage(tscpp.Language())

// CppExtractor implements Extractor for C++ source files (.cc, .cpp, .cxx,
// .hpp, .hh) using tree-sitter CGO bindings (ADR-013). It emits:
//
//   - KindCppNamespace for namespace definitions.
//   - KindCppClass for class specifiers.
//   - KindCppStruct for struct specifiers with a body.
//   - KindCppEnum for enum specifiers with a body.
//   - KindCppFunc for function definitions — both free functions and inline
//     methods defined inside a class body.  Methods are qualified as
//     "ClassName.method" using the scope stack.
//
// All symbols carry TierVerified.
type CppExtractor struct{}

// NewCppExtractor returns a new CppExtractor.
func NewCppExtractor() *CppExtractor { return &CppExtractor{} }

// Extract parses C++ source bytes with tree-sitter and returns SymbolNodes.
// Output is sorted by Line then Name for determinism.
func (c *CppExtractor) Extract(service, relPath string, src []byte) ([]core.SymbolNode, error) {
	nodes, err := c.extract(service, relPath, src, nil)
	return nodes, err
}

// SpecLang is the file-resolve language ExtractWithFileSpecs reads specs for.
func (c *CppExtractor) SpecLang() string { return "cpp" }

// ExtractWithFileSpecs is Extract plus the file specs the dialect's queries
// read from the same native tree (EW-53): no second parse.
func (c *CppExtractor) ExtractWithFileSpecs(service, relPath string, src []byte) ([]core.SymbolNode, FileSpecs, bool, error) {
	var out nativeSpecsOut
	nodes, err := c.extract(service, relPath, src, &out)
	return nodes, out.specs, out.ok && err == nil, err
}

func (c *CppExtractor) extract(service, relPath string, src []byte, out *nativeSpecsOut) ([]core.SymbolNode, error) {
	var nodes []core.SymbolNode

	pkg := pkgFromPath(relPath)

	err := treesitter.WithTree(cppLang, src, func(root *ts.Node) {
		out.take("cpp", cppLang, root, relPath, src)
		walkCpp(root, src, service, pkg, relPath, nil, &nodes)
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

// walkCpp recursively descends the tree-sitter CST for C++ source.
//
// scopeStack tracks the names of enclosing class/struct specifiers so that
// methods defined inline inside a class body are qualified as "Class.method".
// Namespace names are NOT pushed onto the stack — out-of-class function
// definitions already carry the fully qualified name in a qualified_identifier.
func walkCpp(
	n *ts.Node,
	src []byte,
	service, pkg, relPath string,
	scopeStack []string,
	out *[]core.SymbolNode,
) {
	if n == nil {
		return
	}

	switch n.Kind() {
	case "namespace_definition":
		// "name" field → namespace_identifier (or a nested_namespace_specifier
		// for C++17 "namespace a::b", which normalizes to "a.b").
		name := cppCanonicalName(treesitter.FieldText(n, "name", src))
		if name != "" {
			*out = append(*out, core.SymbolNode{
				Service:  service,
				Package:  pkg,
				Name:     name,
				Kind:     core.KindCppNamespace,
				FilePath: relPath,
				Line:     treesitter.Line(n),
				EndLine:  treesitter.EndLine(n),
				Tier:     core.TierVerified,
			})
		}
		// Recurse into body; namespace name does NOT enter the method scope
		// (methods in anonymous namespaces are unusual; named ones carry their
		// own fully qualified names via qualified_identifier).
		body := n.ChildByFieldName("body")
		if body != nil {
			walkCpp(body, src, service, pkg, relPath, scopeStack, out)
		}
		return

	case "class_specifier":
		// "name" field → type_identifier
		name := treesitter.FieldText(n, "name", src)
		if name == "" {
			break
		}
		qualName := qualifyCpp(scopeStack, name)
		*out = append(*out, core.SymbolNode{
			Service:  service,
			Package:  pkg,
			Name:     qualName,
			Kind:     core.KindCppClass,
			FilePath: relPath,
			Line:     treesitter.Line(n),
			EndLine:  treesitter.EndLine(n),
			Tier:     core.TierVerified,
		})
		// Recurse into body with this class name on the scope stack so inline
		// methods are qualified as "ClassName.method".
		newScope := append(append([]string{}, scopeStack...), qualName)
		body := n.ChildByFieldName("body")
		if body != nil {
			walkCpp(body, src, service, pkg, relPath, newScope, out)
		}
		return

	case "struct_specifier":
		// Only emit structs that have a body; forward declarations are skipped.
		if n.ChildByFieldName("body") == nil {
			return
		}
		name := treesitter.FieldText(n, "name", src)
		if name == "" {
			break
		}
		qualName := qualifyCpp(scopeStack, name)
		*out = append(*out, core.SymbolNode{
			Service:  service,
			Package:  pkg,
			Name:     qualName,
			Kind:     core.KindCppStruct,
			FilePath: relPath,
			Line:     treesitter.Line(n),
			EndLine:  treesitter.EndLine(n),
			Tier:     core.TierVerified,
		})
		// Recurse into body with the struct name on the scope stack.
		newScope := append(append([]string{}, scopeStack...), qualName)
		body := n.ChildByFieldName("body")
		if body != nil {
			walkCpp(body, src, service, pkg, relPath, newScope, out)
		}
		return

	case "enum_specifier":
		// Only emit enums that have a body.
		if n.ChildByFieldName("body") == nil {
			return
		}
		name := treesitter.FieldText(n, "name", src)
		if name != "" {
			qualName := qualifyCpp(scopeStack, name)
			*out = append(*out, core.SymbolNode{
				Service:  service,
				Package:  pkg,
				Name:     qualName,
				Kind:     core.KindCppEnum,
				FilePath: relPath,
				Line:     treesitter.Line(n),
				EndLine:  treesitter.EndLine(n),
				Tier:     core.TierVerified,
			})
		}
		return

	case "function_definition":
		decl := n.ChildByFieldName("declarator")
		raw := cDeclName(decl, src)
		if raw == "" {
			return
		}
		// Qualify with the enclosing class name when inside a class body.
		// Out-of-class definitions (e.g. "void MyClass::method() { }") already
		// carry the class name via a qualified_identifier returned by cDeclName,
		// so we only prepend the scope when there is no "::" in the raw name.
		symName := cppCanonicalName(raw)
		if len(scopeStack) > 0 && !containsScope(raw) {
			symName = scopeStack[len(scopeStack)-1] + "." + raw
		}
		*out = append(*out, core.SymbolNode{
			Service:  service,
			Package:  pkg,
			Name:     symName,
			Kind:     core.KindCppFunc,
			FilePath: relPath,
			Line:     treesitter.Line(n),
			EndLine:  treesitter.EndLine(n),
			Tier:     core.TierVerified,
			Calls:    cppCanonicalCalls(treesitter.CollectCalls(n, src, map[string]string{"call_expression": "function"})),
		})
		// Do not recurse into function bodies.
		return
	}

	// Generic descent for all other node types (translation_unit,
	// declaration_list, field_declaration_list, declaration, …).
	count := n.NamedChildCount()
	for i := uint(0); i < count; i++ {
		child := n.NamedChild(i)
		walkCpp(child, src, service, pkg, relPath, scopeStack, out)
	}
}

// qualifyCpp prepends the innermost scope name when inside a class/struct body.
func qualifyCpp(scopeStack []string, name string) string {
	if len(scopeStack) == 0 {
		return name
	}
	return scopeStack[len(scopeStack)-1] + "." + name
}

// containsScope reports whether a function name already contains a C++ scope
// separator ("::"), meaning it is an out-of-class definition that carries its
// own qualification (e.g. "MyClass::method").
func containsScope(name string) bool {
	for i := 0; i < len(name)-1; i++ {
		if name[i] == ':' && name[i+1] == ':' {
			return true
		}
	}
	return false
}

// cppCanonicalName normalizes C++ scope qualification ("::") to the graph's
// one true separator (".") so a symbol has exactly one spelling regardless of
// where it was defined: an inline method extracts as "Widget.render" via the
// scope stack, and the out-of-class "void Widget::render()" must land on the
// same key (docs/plans/cpp-toolchain.md CT-1). Callee references normalize
// identically so edges join.
func cppCanonicalName(name string) string {
	return strings.ReplaceAll(name, "::", ".")
}

// cppCanonicalCalls normalizes every collected callee and re-sorts: "::"
// (0x3A) and "." (0x2E) order differently, and Calls is contractually sorted.
func cppCanonicalCalls(calls []string) []string {
	if len(calls) == 0 {
		return calls
	}
	out := make([]string, len(calls))
	for i, c := range calls {
		out[i] = cppCanonicalName(c)
	}
	sort.Strings(out)
	return out
}
