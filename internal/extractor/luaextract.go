package extractor

import (
	"sort"
	"strings"

	tslua "github.com/tree-sitter-grammars/tree-sitter-lua/bindings/go"
	ts "github.com/tree-sitter/go-tree-sitter"

	"github.com/geoffmilleraz/sextant/internal/core"
	"github.com/geoffmilleraz/sextant/internal/extractor/treesitter"
)

// luaLang is initialised once and shared across all LuaExtractor calls.
// tree-sitter Language objects are immutable and safe for concurrent use;
// only the Parser (created fresh per call) carries mutable state.
var luaLang = ts.NewLanguage(tslua.Language())

// LuaExtractor implements Extractor for Lua source files using tree-sitter
// CGO bindings (ADR-013). It emits:
//
//   - KindLuaFunction for every function_declaration node: global functions
//     (`function foo()`), local functions (`local function foo()`), dotted
//     module functions (`function M.foo()`), and colon-method functions
//     (`function M:method()`).
//
// The `name` field of function_declaration already carries the full name text
// (e.g. "M.foo" or "MyClass:new") as rendered by the grammar, so no manual
// child traversal is needed for qualified names.
//
// All symbols carry TierVerified.
type LuaExtractor struct{}

// NewLuaExtractor returns a new LuaExtractor.
func NewLuaExtractor() *LuaExtractor { return &LuaExtractor{} }

// Extract parses src with tree-sitter and returns SymbolNodes for all
// function declarations. Output is sorted by Line then Name for determinism.
func (l *LuaExtractor) Extract(service, relPath string, src []byte) ([]core.SymbolNode, error) {
	var nodes []core.SymbolNode

	pkg := pkgFromPath(relPath)

	err := treesitter.WithTree(luaLang, src, func(root *ts.Node) {
		walkLua(root, src, service, pkg, relPath, &nodes)
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

// walkLua recursively descends the tree-sitter CST collecting function
// declarations. The Lua grammar represents both `function foo()` and
// `local function foo()` as function_declaration nodes, with the function
// name accessible via the "name" field.
func walkLua(
	n *ts.Node,
	src []byte,
	service, pkg, relPath string,
	out *[]core.SymbolNode,
) {
	if n == nil {
		return
	}

	if n.Kind() == "function_declaration" {
		name := luaFuncName(n, src)
		if name != "" {
			*out = append(*out, core.SymbolNode{
				Service:  service,
				Package:  pkg,
				Name:     name,
				Kind:     core.KindLuaFunction,
				FilePath: relPath,
				Line:     treesitter.Line(n),
				Tier:     core.TierVerified,
				Calls:    treesitter.CollectCalls(n, src, map[string]string{"function_call": "name"}),
			})
		}
		// Do not recurse into the function body — nested function declarations
		// inside a body are handled by the generic descent below only if we
		// fall through, but we return early here.  Inner closures assigned via
		// variable_declaration are not function_declaration nodes so they
		// would not be emitted anyway; this keeps the surface clean.
		return
	}

	// Generic descent for all other node types (chunk, block, …).
	count := n.NamedChildCount()
	for i := uint(0); i < count; i++ {
		child := n.NamedChild(i)
		walkLua(child, src, service, pkg, relPath, out)
	}
}

// luaFuncName extracts the name of a function_declaration node. The "name"
// field of a function_declaration may be:
//
//   - identifier          → simple name ("foo")
//   - dot_index_expression → "M.method" (text already contains the dot)
//   - method_index_expression → "M:method" (text already contains the colon)
//
// In all three cases Utf8Text on the name child gives the full rendered string.
func luaFuncName(n *ts.Node, src []byte) string {
	nameNode := n.ChildByFieldName("name")
	if nameNode == nil {
		return ""
	}
	text := strings.TrimSpace(string(nameNode.Utf8Text(src)))
	return text
}
