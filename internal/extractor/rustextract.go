package extractor

import (
	"sort"

	ts "github.com/tree-sitter/go-tree-sitter"
	tsrust "github.com/tree-sitter/tree-sitter-rust/bindings/go"

	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/extractor/treesitter"
)

// rustLang is initialised once and shared across all RustExtractor calls.
// tree-sitter Language objects are immutable and safe for concurrent use;
// only the Parser (created fresh per call) carries mutable state.
var rustLang = ts.NewLanguage(tsrust.Language())

// RustExtractor implements Extractor for Rust source files using
// tree-sitter CGO bindings (ADR-013). It emits:
//
//   - KindRustFn for free functions and impl methods (qualified as "Type.method").
//   - KindRustStruct for struct definitions.
//   - KindRustEnum for enum definitions.
//   - KindRustTrait for trait definitions.
//   - KindRustImpl for impl blocks (named by the implementing type).
//   - KindRustMod for module declarations.
//
// All symbols carry TierVerified.
type RustExtractor struct{}

// NewRustExtractor returns a new RustExtractor.
func NewRustExtractor() *RustExtractor { return &RustExtractor{} }

// Extract parses Rust source bytes with tree-sitter and returns SymbolNodes
// for functions, structs, enums, traits, impl blocks, and module declarations.
// Output is sorted by Line then Name for determinism.
func (r *RustExtractor) Extract(service, relPath string, src []byte) ([]core.SymbolNode, error) {
	var nodes []core.SymbolNode

	pkg := pkgFromPath(relPath)

	err := treesitter.WithTree(rustLang, src, func(root *ts.Node) {
		walkRust(root, src, service, pkg, relPath, nil, &nodes)
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

// walkRust recursively descends the tree-sitter CST.
//
// scopeStack tracks enclosing impl type names so that methods inside an
// impl block can be qualified as "Type.method".
func walkRust(
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
	case "function_item":
		name := treesitter.FieldText(n, "name", src)
		if name == "" {
			break
		}
		symName := name
		if len(scopeStack) > 0 {
			symName = scopeStack[len(scopeStack)-1] + "." + name
		}
		*out = append(*out, core.SymbolNode{
			Service:  service,
			Package:  pkg,
			Name:     symName,
			Kind:     core.KindRustFn,
			FilePath: relPath,
			Line:     treesitter.Line(n),
			EndLine:  treesitter.EndLine(n),
			Tier:     core.TierVerified,
			Calls: treesitter.CollectCalls(n, src, map[string]string{
				"call_expression":  "function",
				"macro_invocation": "macro",
			}),
		})
		// Do not recurse into function bodies — inner items are unusual and
		// not worth complicating the scope.
		return

	case "struct_item":
		name := treesitter.FieldText(n, "name", src)
		if name != "" {
			*out = append(*out, core.SymbolNode{
				Service:  service,
				Package:  pkg,
				Name:     name,
				Kind:     core.KindRustStruct,
				FilePath: relPath,
				Line:     treesitter.Line(n),
				EndLine:  treesitter.EndLine(n),
				Tier:     core.TierVerified,
			})
		}
		// No descent needed: struct bodies don't contain items we extract.
		return

	case "enum_item":
		name := treesitter.FieldText(n, "name", src)
		if name != "" {
			*out = append(*out, core.SymbolNode{
				Service:  service,
				Package:  pkg,
				Name:     name,
				Kind:     core.KindRustEnum,
				FilePath: relPath,
				Line:     treesitter.Line(n),
				EndLine:  treesitter.EndLine(n),
				Tier:     core.TierVerified,
			})
		}
		return

	case "trait_item":
		name := treesitter.FieldText(n, "name", src)
		if name != "" {
			*out = append(*out, core.SymbolNode{
				Service:  service,
				Package:  pkg,
				Name:     name,
				Kind:     core.KindRustTrait,
				FilePath: relPath,
				Line:     treesitter.Line(n),
				EndLine:  treesitter.EndLine(n),
				Tier:     core.TierVerified,
			})
		}
		// Do not recurse into trait bodies — trait method signatures are
		// function_signature_item, not function_item, so they won't collide;
		// but we don't emit them as separate symbols here.
		return

	case "mod_item":
		name := treesitter.FieldText(n, "name", src)
		if name != "" {
			*out = append(*out, core.SymbolNode{
				Service:  service,
				Package:  pkg,
				Name:     name,
				Kind:     core.KindRustMod,
				FilePath: relPath,
				Line:     treesitter.Line(n),
				EndLine:  treesitter.EndLine(n),
				Tier:     core.TierVerified,
			})
		}
		// Recurse into the module body with the same (empty) scope stack —
		// mod-level functions are free functions, not methods.
		body := n.ChildByFieldName("body")
		if body != nil {
			walkRust(body, src, service, pkg, relPath, nil, out)
		}
		return

	case "impl_item":
		// The implementing type is in the "type" field (not "name").
		typeName := treesitter.FieldText(n, "type", src)
		if typeName != "" {
			*out = append(*out, core.SymbolNode{
				Service:  service,
				Package:  pkg,
				Name:     typeName,
				Kind:     core.KindRustImpl,
				FilePath: relPath,
				Line:     treesitter.Line(n),
				EndLine:  treesitter.EndLine(n),
				Tier:     core.TierVerified,
			})
		}
		// Recurse into the impl body with the type name pushed onto the scope
		// stack so nested function_items are qualified as "Type.method".
		body := n.ChildByFieldName("body")
		if body != nil {
			newScope := append(append([]string{}, scopeStack...), typeName)
			walkRust(body, src, service, pkg, relPath, newScope, out)
		}
		return
	}

	// Generic descent for all other node types (source_file, declaration_list, …).
	count := n.NamedChildCount()
	for i := uint(0); i < count; i++ {
		child := n.NamedChild(i)
		walkRust(child, src, service, pkg, relPath, scopeStack, out)
	}
}
