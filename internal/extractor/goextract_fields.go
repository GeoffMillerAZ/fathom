package extractor

import (
	"go/ast"
	"go/token"
	"go/types"

	"github.com/geoffmilleraz/fathom/internal/core"
)

// structFieldNodes emits fields directly owned by a named top-level struct.
// Aliases and fields inside anonymous nested structs have no owner here.
func structFieldNodes(service, relPath string, fset *token.FileSet, ts *ast.TypeSpec) []core.SymbolNode {
	if ts.Name == nil || ts.Assign.IsValid() {
		return nil
	}
	st, ok := ts.Type.(*ast.StructType)
	if !ok || st.Fields == nil {
		return nil
	}
	owner := ts.Name.Name
	var nodes []core.SymbolNode
	for _, field := range st.Fields.List {
		typ := types.ExprString(field.Type)
		add := func(name string, at token.Pos, signature string) {
			nodes = append(nodes, core.SymbolNode{
				Service: service, Package: owner, Name: owner + "." + name,
				Kind: core.KindGoField, FilePath: relPath,
				Line: fset.Position(at).Line, EndLine: fset.Position(field.End()).Line,
				Signature: signature, Tier: core.TierVerified,
			})
		}
		if len(field.Names) == 0 {
			if name := embeddedFieldName(field.Type); name != "" {
				add(name, field.Type.Pos(), typ)
			}
			continue
		}
		for _, id := range field.Names {
			if id != nil && id.Name != "_" {
				add(id.Name, id.Pos(), id.Name+" "+typ)
			}
		}
	}
	return nodes
}

// embeddedFieldName follows Go's field naming through pointers, package
// selectors and type arguments. Other expressions cannot name embedded fields.
func embeddedFieldName(expr ast.Expr) string {
	switch t := expr.(type) {
	case *ast.StarExpr:
		return embeddedFieldName(t.X)
	case *ast.Ident:
		return t.Name
	case *ast.SelectorExpr:
		return t.Sel.Name
	case *ast.IndexExpr:
		return embeddedFieldName(t.X)
	case *ast.IndexListExpr:
		return embeddedFieldName(t.X)
	case *ast.ParenExpr:
		return embeddedFieldName(t.X)
	}
	return ""
}
