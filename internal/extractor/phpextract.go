package extractor

import (
	"sort"

	ts "github.com/tree-sitter/go-tree-sitter"
	tsphp "github.com/tree-sitter/tree-sitter-php/bindings/go"

	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/extractor/treesitter"
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
//   - KindPhpProperty for a declared or constructor-promoted property
//     ("Owner.$name"), KindPhpConst for a class/interface/trait/enum
//     constant ("Owner.NAME") and KindPhpEnumCase for an enum case
//     ("Owner.Case") — the definitions scip-php writes (LB-F81).
//
// All symbols carry TierVerified.
type PHPExtractor struct{}

// NewPhpExtractor returns a new PHPExtractor.
func NewPhpExtractor() *PHPExtractor { return &PHPExtractor{} }

// Extract parses src with tree-sitter and returns SymbolNodes for PHP
// classes, interfaces, traits, enums, functions, and methods. Output is
// sorted by Line then Name for determinism.
func (e *PHPExtractor) Extract(service, relPath string, src []byte) ([]core.SymbolNode, error) {
	nodes, err := e.extract(service, relPath, src, nil)
	return nodes, err
}

// SpecLang is the file-resolve language ExtractWithFileSpecs reads specs for.
func (e *PHPExtractor) SpecLang() string { return "php" }

// ExtractWithFileSpecs is Extract plus the file specs the dialect's queries
// read from the same native tree (EW-53): no second parse.
func (e *PHPExtractor) ExtractWithFileSpecs(service, relPath string, src []byte) ([]core.SymbolNode, FileSpecs, bool, error) {
	var out nativeSpecsOut
	nodes, err := e.extract(service, relPath, src, &out)
	return nodes, out.specs, out.ok && err == nil, err
}

func (e *PHPExtractor) extract(service, relPath string, src []byte, out *nativeSpecsOut) ([]core.SymbolNode, error) {
	var nodes []core.SymbolNode

	pkg := pkgFromPath(relPath)

	err := treesitter.WithTree(phpLang, src, func(root *ts.Node) {
		out.take("php", phpLang, root, relPath, src)
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
				EndLine:  treesitter.EndLine(n),
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
				EndLine:  treesitter.EndLine(n),
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
				EndLine:  treesitter.EndLine(n),
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
				EndLine:  treesitter.EndLine(n),
				Tier:     core.TierVerified,
			})
		}
		// The enum body holds cases, constants and methods (LB-F81).
		if body := n.ChildByFieldName("body"); body != nil {
			walkPHP(body, src, service, pkg, relPath, name, out)
		}
		return

	case "property_declaration", "const_declaration", "enum_case":
		if scopeName != "" {
			walkPHPMember(n, src, service, pkg, relPath, scopeName, out)
		}
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
				EndLine:  treesitter.EndLine(n),
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
				EndLine:  treesitter.EndLine(n),
				Tier:     core.TierVerified,
				Calls: treesitter.CollectCalls(n, src, map[string]string{
					"function_call_expression": "function",
					"member_call_expression":   "name",
					"scoped_call_expression":   "name",
				}),
			})
		}
		// A constructor's promoted parameters are properties (LB-F81).
		if name == "__construct" && scopeName != "" {
			walkPHPPromoted(n, src, service, pkg, relPath, scopeName, out)
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

// walkPHPMember emits one definition per element of a property_declaration
// (`public int $a, $b;`), a const_declaration (`const A = 1, B = 2;`) or the
// single enum_case n, each named "owner.name" (a property keeps its `$`, as
// scip-php writes it) and positioned at its own element so a grouped
// declaration maps one-to-one onto the index's definitions (LB-F81).
func walkPHPMember(n *ts.Node, src []byte, service, pkg, relPath, owner string, out *[]core.SymbolNode) {
	emit := func(el *ts.Node, name string, kind core.SourceKind) {
		if name == "" {
			return
		}
		*out = append(*out, core.SymbolNode{
			Service:  service,
			Package:  pkg,
			Name:     owner + "." + name,
			Kind:     kind,
			FilePath: relPath,
			Line:     treesitter.Line(el),
			EndLine:  treesitter.EndLine(el),
			Tier:     core.TierVerified,
		})
	}
	switch n.Kind() {
	case "enum_case":
		emit(n, treesitter.FieldText(n, "name", src), core.KindPhpEnumCase)
		return
	case "property_declaration":
		for i := uint(0); i < n.NamedChildCount(); i++ {
			if el := n.NamedChild(i); el.Kind() == "property_element" {
				emit(el, treesitter.FieldText(el, "name", src), core.KindPhpProperty)
			}
		}
	case "const_declaration":
		for i := uint(0); i < n.NamedChildCount(); i++ {
			el := n.NamedChild(i)
			if el.Kind() != "const_element" {
				continue
			}
			for j := uint(0); j < el.NamedChildCount(); j++ {
				if nm := el.NamedChild(j); nm.Kind() == "name" {
					emit(el, nm.Utf8Text(src), core.KindPhpConst)
					break
				}
			}
		}
	}
}

// walkPHPPromoted emits a property for each promoted parameter (one carrying a
// visibility or readonly modifier) of the constructor m.
func walkPHPPromoted(m *ts.Node, src []byte, service, pkg, relPath, owner string, out *[]core.SymbolNode) {
	params := m.ChildByFieldName("parameters")
	if params == nil {
		return
	}
	for i := uint(0); i < params.NamedChildCount(); i++ {
		p := params.NamedChild(i)
		if p.Kind() != "property_promotion_parameter" {
			continue
		}
		name := treesitter.FieldText(p, "name", src)
		if name == "" {
			continue
		}
		*out = append(*out, core.SymbolNode{
			Service:  service,
			Package:  pkg,
			Name:     owner + "." + name,
			Kind:     core.KindPhpProperty,
			FilePath: relPath,
			Line:     treesitter.Line(p),
			EndLine:  treesitter.EndLine(p),
			Tier:     core.TierVerified,
		})
	}
}
