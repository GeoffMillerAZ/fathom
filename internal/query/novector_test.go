package query_test

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"strings"
	"testing"
)

// TestQueryEngineHasNoVectorOption asserts, by AST scan of every non-test
// source file in this package, that no declared function, type, struct
// field, or package-level var/const is named *Vector*/*Embedding* (case
// insensitive).
//
// ADR-041 decision 2 (docs/vision/31-substrate.md §3 D5/D7) deleted
// internal/vector, the WithVector option, and the SymbolIndexer-driven
// hybridRank branch as dead code: no production caller ever wired a
// SymbolIndexer into the engine. The keyword+tier ranking (scoreAndRank)
// is the whole of Query's ranking now. Revisit only through vision 31 §5's
// kill criterion: a controlled result on a labelled concept-query set that
// lexical plus graph cannot match at <= 2x tokens — until then this test is
// the guard against a quiet reintroduction.
func TestQueryEngineHasNoVectorOption(t *testing.T) {
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatal(err)
	}

	banned := []string{"vector", "embedding"}
	nameIsBanned := func(name string) bool {
		lower := strings.ToLower(name)
		for _, b := range banned {
			if strings.Contains(lower, b) {
				return true
			}
		}
		return false
	}

	fset := token.NewFileSet()
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		f, err := parser.ParseFile(fset, name, nil, 0)
		if err != nil {
			t.Fatalf("parse %s: %v", name, err)
		}
		for _, decl := range f.Decls {
			switch d := decl.(type) {
			case *ast.FuncDecl:
				if nameIsBanned(d.Name.Name) {
					t.Errorf("%s: func %q — no Vector/Embedding-named option or function belongs in the query engine (ADR-041 decision 2; revisit bar: vision 31 §5)", name, d.Name.Name)
				}
			case *ast.GenDecl:
				for _, spec := range d.Specs {
					switch s := spec.(type) {
					case *ast.TypeSpec:
						if nameIsBanned(s.Name.Name) {
							t.Errorf("%s: type %q — no Vector/Embedding-named type belongs in the query engine (ADR-041 decision 2; revisit bar: vision 31 §5)", name, s.Name.Name)
						}
						if st, ok := s.Type.(*ast.StructType); ok && st.Fields != nil {
							for _, field := range st.Fields.List {
								for _, fname := range field.Names {
									if nameIsBanned(fname.Name) {
										t.Errorf("%s: field %q on type %q — no Vector/Embedding-named field belongs in the query engine (ADR-041 decision 2; revisit bar: vision 31 §5)", name, fname.Name, s.Name.Name)
									}
								}
							}
						}
					case *ast.ValueSpec:
						for _, vname := range s.Names {
							if nameIsBanned(vname.Name) {
								t.Errorf("%s: %q — no Vector/Embedding-named declaration belongs in the query engine (ADR-041 decision 2; revisit bar: vision 31 §5)", name, vname.Name)
							}
						}
					}
				}
			}
		}
	}
}
