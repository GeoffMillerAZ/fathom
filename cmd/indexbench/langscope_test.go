package main

import (
	"go/ast"
	"go/parser"
	"go/token"
	"reflect"
	"testing"
)

func TestReactUsesSharedScopeNote(t *testing.T) {
	f, err := parser.ParseFile(token.NewFileSet(), "langoracle_react.go", nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	var shared bool
	for _, decl := range f.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok || fn.Name.Name != "measureReact" {
			continue
		}
		ast.Inspect(fn.Body, func(n ast.Node) bool {
			if call, ok := n.(*ast.CallExpr); ok {
				if name, ok := call.Fun.(*ast.Ident); ok && name.Name == "onlyScopeNote" {
					shared = true
				}
			}
			return true
		})
	}
	if !shared {
		t.Fatal("measureReact must use onlyScopeNote for the shared disclosure format")
	}
}

func TestOnlyScopeNoteRoundTrip(t *testing.T) {
	for _, globs := range [][]string{{"src/*.lua"}, {"src/**/*.lua", "lib/*.lua", "init.lua"}} {
		note := onlyScopeNote("fixture", globs)
		got, ok := parseOnlyScopeNote("fixture", note)
		if !ok || !reflect.DeepEqual(got, globs) {
			t.Fatalf("round trip %q: %v, %v", note, got, ok)
		}
		if _, ok := parseOnlyScopeNote("other", note); ok {
			t.Fatal("accepted another subject's scope")
		}
	}
	if _, ok := parseOnlyScopeNote("fixture", "unrelated note"); ok {
		t.Fatal("accepted unrelated note")
	}
}
