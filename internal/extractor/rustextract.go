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
	nodes, err := r.extract(service, relPath, src, nil)
	return nodes, err
}

// SpecLang is the file-resolve language ExtractWithFileSpecs reads specs for.
func (r *RustExtractor) SpecLang() string { return "rust" }

// ExtractWithFileSpecs is Extract plus the file specs the dialect's queries
// read from the same native tree (EW-53): no second parse.
func (r *RustExtractor) ExtractWithFileSpecs(service, relPath string, src []byte) ([]core.SymbolNode, FileSpecs, bool, error) {
	var out nativeSpecsOut
	nodes, err := r.extract(service, relPath, src, &out)
	return nodes, out.specs, out.ok && err == nil, err
}

func (r *RustExtractor) extract(service, relPath string, src []byte, out *nativeSpecsOut) ([]core.SymbolNode, error) {
	var nodes []core.SymbolNode

	pkg := pkgFromPath(relPath)

	err := treesitter.WithTree(rustLang, src, func(root *ts.Node) {
		out.take("rust", rustLang, root, relPath, src)
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
			walkRustFields(n.ChildByFieldName("body"), src, service, pkg, relPath, name, out)
		}
		// No further descent: struct bodies hold fields only.
		return

	case "union_item":
		name := treesitter.FieldText(n, "name", src)
		if name != "" {
			*out = append(*out, core.SymbolNode{
				Service:  service,
				Package:  pkg,
				Name:     name,
				Kind:     core.KindRustUnion,
				FilePath: relPath,
				Line:     treesitter.Line(n),
				EndLine:  treesitter.EndLine(n),
				Tier:     core.TierVerified,
			})
			walkRustFields(n.ChildByFieldName("body"), src, service, pkg, relPath, name, out)
		}
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
			walkRustVariants(n.ChildByFieldName("body"), src, service, pkg, relPath, name, out)
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
		// A trait body's members (required methods, associated consts and
		// types) are qualified by the trait; a default method is a
		// function_item and takes the same path.
		if body := n.ChildByFieldName("body"); body != nil && name != "" {
			walkRust(body, src, service, pkg, relPath, append(append([]string{}, scopeStack...), name), out)
		}
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

	case "function_signature_item":
		// A trait's required method (bodiless); outside a trait or impl body
		// (an extern block's function) it keeps its bare name like a free fn.
		name := treesitter.FieldText(n, "name", src)
		if name == "" {
			break
		}
		*out = append(*out, core.SymbolNode{
			Service:  service,
			Package:  pkg,
			Name:     rustQualify(scopeStack, name),
			Kind:     core.KindRustFn,
			FilePath: relPath,
			Line:     treesitter.Line(n),
			EndLine:  treesitter.EndLine(n),
			Tier:     core.TierVerified,
		})
		return

	case "const_item", "associated_type", "type_item":
		// Associated items only: inside an impl or trait body. A module-level
		// const or alias is the dialect's own item kinds, not extracted here.
		if len(scopeStack) == 0 {
			break
		}
		name := treesitter.FieldText(n, "name", src)
		if name == "" {
			break
		}
		k := core.KindRustAssocType
		if kind == "const_item" {
			k = core.KindRustAssocConst
		}
		*out = append(*out, core.SymbolNode{
			Service:  service,
			Package:  pkg,
			Name:     rustQualify(scopeStack, name),
			Kind:     k,
			FilePath: relPath,
			Line:     treesitter.Line(n),
			EndLine:  treesitter.EndLine(n),
			Tier:     core.TierVerified,
		})
		return
	}

	// Generic descent for all other node types (source_file, declaration_list, …).
	count := n.NamedChildCount()
	for i := uint(0); i < count; i++ {
		child := n.NamedChild(i)
		walkRust(child, src, service, pkg, relPath, scopeStack, out)
	}
}

// rustQualify is "Owner.name" under the innermost impl or trait scope, and
// the bare name outside one (the way a method is qualified).
func rustQualify(scopeStack []string, name string) string {
	if len(scopeStack) == 0 {
		return name
	}
	return scopeStack[len(scopeStack)-1] + "." + name
}

// walkRustFields emits the named fields (field_declaration) of a struct,
// union or struct-like enum variant body, qualified owner.field. A tuple
// field (ordered_field_declaration_list) is not emitted: rust-analyzer's SCIP
// index defines none for a crate's own tuple struct (calibration/oracles/
// rust-analyzer/README.md), so def_mapping needs none.
func walkRustFields(body *ts.Node, src []byte, service, pkg, relPath, owner string, out *[]core.SymbolNode) {
	if body == nil {
		return
	}
	emit := func(n *ts.Node, name string) {
		*out = append(*out, core.SymbolNode{
			Service:  service,
			Package:  pkg,
			Name:     owner + "." + name,
			Kind:     core.KindRustField,
			FilePath: relPath,
			Line:     treesitter.Line(n),
			EndLine:  treesitter.EndLine(n),
			Tier:     core.TierVerified,
		})
	}
	switch body.Kind() {
	case "field_declaration_list":
		for i, c := uint(0), body.NamedChildCount(); i < c; i++ {
			f := body.NamedChild(i)
			if f.Kind() != "field_declaration" {
				continue
			}
			if name := treesitter.FieldText(f, "name", src); name != "" {
				emit(f, name)
			}
		}
	}
}

// walkRustVariants emits an enum's variants (qualified Enum.Variant) and the
// fields of a struct-like or tuple variant (qualified Variant.field).
func walkRustVariants(body *ts.Node, src []byte, service, pkg, relPath, enum string, out *[]core.SymbolNode) {
	if body == nil {
		return
	}
	for i, c := uint(0), body.NamedChildCount(); i < c; i++ {
		v := body.NamedChild(i)
		if v.Kind() != "enum_variant" {
			continue
		}
		name := treesitter.FieldText(v, "name", src)
		if name == "" {
			continue
		}
		*out = append(*out, core.SymbolNode{
			Service:  service,
			Package:  pkg,
			Name:     enum + "." + name,
			Kind:     core.KindRustVariant,
			FilePath: relPath,
			Line:     treesitter.Line(v),
			EndLine:  treesitter.EndLine(v),
			Tier:     core.TierVerified,
		})
		walkRustFields(v.ChildByFieldName("body"), src, service, pkg, relPath, name, out)
	}
}
