// Package fieldusage uses the Go AST to determine which fields of a named
// struct are referenced anywhere in the provided source file.
//
// Scan parses the file, finds the struct definition, then walks all selector
// expressions (x.Field) to record which field names appear as the right-hand
// side of a selector.  A FieldUse is returned for every declared field; Used is
// true when at least one selector in the file references that field name.
//
// Results are returned in declaration order and are therefore deterministic.
package fieldusage

import (
	"go/ast"
	"go/parser"
	"go/token"
	"sort"
)

// FieldUse records whether a struct field is referenced in the source file.
type FieldUse struct {
	Field string
	Used  bool
}

// Scan parses src as a Go source file (using path for error messages) and
// reports the usage of each field of the named struct.
//
// Embedded fields and anonymous fields are reported by their type name.
// If structName is not found in the file, Scan returns nil.
// Results are in field declaration order (deterministic).
func Scan(path, src string, structName string) []FieldUse {
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, path, src, 0)
	if err != nil {
		return nil
	}

	// Find the struct type definition.
	fields := structFields(f, structName)
	if fields == nil {
		return nil
	}

	// Collect all selector RHS names used anywhere in the file.
	used := usedSelectors(f)

	result := make([]FieldUse, 0, len(fields))
	for _, name := range fields {
		result = append(result, FieldUse{
			Field: name,
			Used:  used[name],
		})
	}
	return result
}

// structFields returns the declared field names of the named struct type, in
// declaration order.  Returns nil when the struct is not found.
func structFields(f *ast.File, structName string) []string {
	var names []string
	found := false

	for _, decl := range f.Decls {
		genDecl, ok := decl.(*ast.GenDecl)
		if !ok || genDecl.Tok != token.TYPE {
			continue
		}
		for _, spec := range genDecl.Specs {
			ts, ok := spec.(*ast.TypeSpec)
			if !ok || ts.Name.Name != structName {
				continue
			}
			st, ok := ts.Type.(*ast.StructType)
			if !ok {
				continue
			}
			found = true
			for _, field := range st.Fields.List {
				if len(field.Names) == 0 {
					// Embedded / anonymous field: use type name.
					name := embeddedName(field.Type)
					if name != "" {
						names = append(names, name)
					}
					continue
				}
				for _, ident := range field.Names {
					names = append(names, ident.Name)
				}
			}
		}
	}
	if !found {
		return nil
	}
	return names
}

// embeddedName extracts the field name for an embedded type expression.
func embeddedName(expr ast.Expr) string {
	switch t := expr.(type) {
	case *ast.Ident:
		return t.Name
	case *ast.StarExpr:
		return embeddedName(t.X)
	case *ast.SelectorExpr:
		return t.Sel.Name
	}
	return ""
}

// usedSelectors walks the entire file and records every selector RHS (x.Field)
// that appears in any expression.
func usedSelectors(f *ast.File) map[string]bool {
	used := make(map[string]bool)
	ast.Inspect(f, func(n ast.Node) bool {
		sel, ok := n.(*ast.SelectorExpr)
		if ok {
			used[sel.Sel.Name] = true
		}
		return true
	})
	return used
}

// ScanSorted is like Scan but returns results sorted alphabetically by field
// name instead of declaration order. Useful when callers want a stable sort
// independent of declaration order.
func ScanSorted(path, src string, structName string) []FieldUse {
	result := Scan(path, src, structName)
	sort.Slice(result, func(i, j int) bool {
		return result[i].Field < result[j].Field
	})
	return result
}
