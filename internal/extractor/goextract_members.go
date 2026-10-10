package extractor

import (
	"go/ast"
	"go/token"
	"go/types"
	"strings"

	"github.com/geoffmilleraz/fathom/internal/core"
)

// goextract_members.go — K17 (wave lf2, LB-F342, closes O606): Go's member
// pass. A top-level struct type's fields become symbols named Owner.name,
// core.KindGoField (canonical "field"), an embedded field named by its type
// (`*pkg.T[X]` is T, as Go names it); a top-level interface's own methods
// become Iface.Method, core.KindGoMethod. Like a method, a member's Package
// is its owner type's name, so its key is the owner-qualified
// "svc::<dir>/Owner.Owner.name" the typed pass targets (field references,
// LB-F343). Universal-ctags tags the same members (member, anonMember,
// methodSpec) and scip-go defines each as a term of its type.
//
// The K17 cost gate (wave lf2, O676): a struct's fields are members only when
// the struct is exported. Every field of every struct cost fathom's own
// self-ingest +34% store and +30% wall time, over the gate's +20% / +15%;
// an exported struct's fields are the API other packages read and write.
// Every interface's own methods stay members (a few hundred per service).
//
// Not members: the blank field (padding, no name to reference), a nested
// anonymous struct's own fields (finer than its owner's field), an interface's
// embedded interfaces and type-set terms (no method of its own), and the
// fields of an alias of a struct literal (no declared type owns them).
//
// Signatures are cheap on purpose (types.ExprString, never go/format): a
// field reads "name Type" ("Type" alone when embedded), an interface method
// "func Name(params) results", the shape renderFuncSignature gives a
// method.

// memberNodes returns ts's member symbols in source order.
func memberNodes(service, relPath string, fset *token.FileSet, ts *ast.TypeSpec) []core.SymbolNode {
	if ts.Name == nil || ts.Assign.IsValid() {
		return nil
	}
	owner := ts.Name.Name
	member := func(name string, kind core.SourceKind, at, end token.Pos, sig string) core.SymbolNode {
		return core.SymbolNode{
			Service:   service,
			Package:   owner,
			Name:      owner + "." + name,
			Kind:      kind,
			FilePath:  relPath,
			Line:      fset.Position(at).Line,
			EndLine:   fset.Position(end).Line,
			Signature: sig,
			Tier:      core.TierVerified,
		}
	}
	var out []core.SymbolNode
	switch t := ts.Type.(type) {
	case *ast.StructType:
		if t.Fields == nil || !ast.IsExported(owner) {
			return nil // K17 cost gate: an unexported struct's fields are no members
		}
		for _, f := range t.Fields.List {
			typ := types.ExprString(f.Type)
			if len(f.Names) == 0 {
				if name := embeddedFieldName(f.Type); name != "" {
					out = append(out, member(name, core.KindGoField, f.Type.Pos(), f.End(), typ))
				}
				continue
			}
			for _, id := range f.Names {
				if id == nil || id.Name == "_" {
					continue
				}
				out = append(out, member(id.Name, core.KindGoField, id.Pos(), f.End(), id.Name+" "+typ))
			}
		}
	case *ast.InterfaceType:
		if t.Methods == nil {
			return nil
		}
		for _, f := range t.Methods.List {
			ft, ok := f.Type.(*ast.FuncType)
			if !ok {
				continue // an embedded interface or a type-set term
			}
			sig := strings.TrimPrefix(types.ExprString(ft), "func")
			for _, id := range f.Names {
				if id == nil || id.Name == "_" {
					continue
				}
				out = append(out, member(id.Name, core.KindGoMethod, id.Pos(), f.End(), "func "+id.Name+sig))
			}
		}
	}
	return out
}

// embeddedFieldName is the name Go gives an embedded field: its type's own
// name, without pointer, package qualifier or type arguments. "" for a type
// expression no embedded field can have.
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
