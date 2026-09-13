package extractor

import (
	"sort"

	ts "github.com/tree-sitter/go-tree-sitter"
	tsjs "github.com/tree-sitter/tree-sitter-javascript/bindings/go"
	tsts "github.com/tree-sitter/tree-sitter-typescript/bindings/go"

	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/extractor/treesitter"
)

// jsLang, tsLang, and tsxLang are initialised once and shared across all
// JSExtractor calls. tree-sitter Language objects are immutable and safe for
// concurrent use; only the Parser (created fresh per call) carries mutable
// state.
var (
	jsLang  = ts.NewLanguage(tsjs.Language())
	tsLang  = ts.NewLanguage(tsts.LanguageTypescript())
	tsxLang = ts.NewLanguage(tsts.LanguageTSX())
)

// JSExtractor implements Extractor for JavaScript, TypeScript, JSX, and TSX
// source files using tree-sitter CGO bindings (ADR-013). It emits:
//
//   - KindJSFunc for function declarations, generator function declarations,
//     and variable declarators whose initialiser is an arrow function or
//     function expression.
//   - KindJSClass for class declarations.
//   - KindJSMethod for method definitions, qualified as "ClassName.method".
//   - KindTSInterface for TypeScript interface declarations.
//   - KindTSType for TypeScript type alias declarations.
//   - KindTSEnum for TypeScript enum declarations.
//
// All symbols carry TierVerified.
type JSExtractor struct{}

// NewJSExtractor returns a new JSExtractor.
func NewJSExtractor() *JSExtractor { return &JSExtractor{} }

// langForPath selects the tree-sitter grammar based on the file extension.
func langForPath(relPath string) *ts.Language {
	// Find the last dot.
	for i := len(relPath) - 1; i >= 0; i-- {
		if relPath[i] == '.' {
			ext := relPath[i:]
			switch ext {
			case ".ts":
				return tsLang
			case ".tsx":
				return tsxLang
			}
			// .js, .jsx, .mjs, .cjs and everything else → javascript
			return jsLang
		}
	}
	return jsLang
}

// Extract parses src with tree-sitter and returns SymbolNodes for classes,
// functions/methods, and TypeScript-specific constructs. Output is sorted by
// Line then Name for determinism.
func (j *JSExtractor) Extract(service, relPath string, src []byte) ([]core.SymbolNode, error) {
	var nodes []core.SymbolNode

	lang := langForPath(relPath)
	pkg := pkgFromPath(relPath)

	err := treesitter.WithTree(lang, src, func(root *ts.Node) {
		walkJS(root, src, service, pkg, relPath, "", &nodes)
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

// walkJS recursively descends the tree-sitter CST.
//
//   - classScope: the name of the nearest enclosing class ("" at module level
//     or inside a function, so nested functions don't inherit class names).
func walkJS(
	n *ts.Node,
	src []byte,
	service, pkg, relPath string,
	classScope string,
	out *[]core.SymbolNode,
) {
	if n == nil {
		return
	}

	kind := n.Kind()

	switch kind {
	case "function_declaration", "generator_function_declaration":
		name := treesitter.FieldText(n, "name", src)
		if name != "" {
			*out = append(*out, core.SymbolNode{
				Service:  service,
				Package:  pkg,
				Name:     name,
				Kind:     core.KindJSFunc,
				FilePath: relPath,
				Line:     treesitter.Line(n),
				EndLine:  treesitter.EndLine(n),
				Tier:     core.TierVerified,
				Calls:    treesitter.CollectCalls(n, src, map[string]string{"call_expression": "function"}),
			})
		}
		// Recurse into the function body with no class scope.
		body := n.ChildByFieldName("body")
		if body != nil {
			walkJS(body, src, service, pkg, relPath, "", out)
		}
		return

	case "class_declaration":
		name := treesitter.FieldText(n, "name", src)
		if name != "" {
			*out = append(*out, core.SymbolNode{
				Service:  service,
				Package:  pkg,
				Name:     name,
				Kind:     core.KindJSClass,
				FilePath: relPath,
				Line:     treesitter.Line(n),
				EndLine:  treesitter.EndLine(n),
				Tier:     core.TierVerified,
			})
		}
		// Recurse into the class body with this class as scope.
		body := n.ChildByFieldName("body")
		if body != nil {
			walkJS(body, src, service, pkg, relPath, name, out)
		}
		return

	case "method_definition":
		name := treesitter.FieldText(n, "name", src)
		if name != "" && classScope != "" {
			*out = append(*out, core.SymbolNode{
				Service:  service,
				Package:  pkg,
				Name:     classScope + "." + name,
				Kind:     core.KindJSMethod,
				FilePath: relPath,
				Line:     treesitter.Line(n),
				EndLine:  treesitter.EndLine(n),
				Tier:     core.TierVerified,
				Calls:    treesitter.CollectCalls(n, src, map[string]string{"call_expression": "function"}),
			})
		}
		// Don't recurse into method bodies — methods aren't classes.
		return

	case "variable_declarator":
		// Emit KindJSFunc when the initialiser is an arrow_function or
		// function / function_expression.
		val := n.ChildByFieldName("value")
		if val != nil {
			vk := val.Kind()
			if vk == "arrow_function" || vk == "function" || vk == "function_expression" {
				name := treesitter.FieldText(n, "name", src)
				if name != "" {
					*out = append(*out, core.SymbolNode{
						Service:  service,
						Package:  pkg,
						Name:     name,
						Kind:     core.KindJSFunc,
						FilePath: relPath,
						Line:     treesitter.Line(n),
						EndLine:  treesitter.EndLine(n),
						Tier:     core.TierVerified,
						Calls:    treesitter.CollectCalls(val, src, map[string]string{"call_expression": "function"}),
					})
				}
			}
		}
		return

	case "export_statement":
		// Recurse so inner declarations (function, class, const, …) are
		// captured by the cases above.

	case "interface_declaration":
		name := treesitter.FieldText(n, "name", src)
		if name != "" {
			*out = append(*out, core.SymbolNode{
				Service:  service,
				Package:  pkg,
				Name:     name,
				Kind:     core.KindTSInterface,
				FilePath: relPath,
				Line:     treesitter.Line(n),
				EndLine:  treesitter.EndLine(n),
				Tier:     core.TierVerified,
			})
		}
		return

	case "type_alias_declaration":
		name := treesitter.FieldText(n, "name", src)
		if name != "" {
			*out = append(*out, core.SymbolNode{
				Service:  service,
				Package:  pkg,
				Name:     name,
				Kind:     core.KindTSType,
				FilePath: relPath,
				Line:     treesitter.Line(n),
				EndLine:  treesitter.EndLine(n),
				Tier:     core.TierVerified,
			})
		}
		return

	case "enum_declaration":
		name := treesitter.FieldText(n, "name", src)
		if name != "" {
			*out = append(*out, core.SymbolNode{
				Service:  service,
				Package:  pkg,
				Name:     name,
				Kind:     core.KindTSEnum,
				FilePath: relPath,
				Line:     treesitter.Line(n),
				EndLine:  treesitter.EndLine(n),
				Tier:     core.TierVerified,
			})
		}
		return
	}

	// Generic descent for all other node types.
	count := n.NamedChildCount()
	for i := uint(0); i < count; i++ {
		child := n.NamedChild(i)
		walkJS(child, src, service, pkg, relPath, classScope, out)
	}
}
