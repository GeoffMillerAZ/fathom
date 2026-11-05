package extractor

import (
	"go/ast"
	"go/parser"
	"go/token"
)

// GoTypeCanonicals is the canonical kind of each type declared in a Go file,
// by type name: "struct" for a struct type, "interface" for an interface type,
// "type_alias" for an alias (`type A = string`), and "type" for any other defined
// type (`type N int`, `type Fn func()`). The native Go extractor emits one
// SourceKind for all of them, so the door refines its Canonical per symbol from
// the source. Only the
// language library's door reads this: the extractor's nodes are unchanged. A
// file that does not parse has no entry, and the table row for the kind stands.
func GoTypeCanonicals(src []byte) map[string]string {
	f, err := parser.ParseFile(token.NewFileSet(), "", src, 0)
	if err != nil || f == nil {
		return nil
	}
	out := map[string]string{}
	for _, decl := range f.Decls {
		gd, ok := decl.(*ast.GenDecl)
		if !ok || gd.Tok != token.TYPE {
			continue
		}
		for _, spec := range gd.Specs {
			ts, ok := spec.(*ast.TypeSpec)
			if !ok || ts.Name == nil {
				continue
			}
			if _, dup := out[ts.Name.Name]; dup {
				continue
			}
			switch {
			case ts.Assign.IsValid():
				out[ts.Name.Name] = "type_alias"
			default:
				switch ts.Type.(type) {
				case *ast.StructType:
					out[ts.Name.Name] = "struct"
				case *ast.InterfaceType:
					out[ts.Name.Name] = "interface"
				default:
					out[ts.Name.Name] = "type"
				}
			}
		}
	}
	return out
}
