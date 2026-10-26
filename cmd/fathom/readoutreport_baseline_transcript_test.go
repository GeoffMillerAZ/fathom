package main

import (
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"testing"
	"time"
)

func TestReadoutBaselineSharedTranscriptWalkerAndPinnedCounts(t *testing.T) {
	rep := readoutBaselineReport(filepath.Join(roBaselineFixture, "hooks.jsonl"), filepath.Join(roBaselineFixture, "projects"), time.Time{})
	if rep.Rows != 6 || rep.Located != 3 || rep.Touched != 2 || rep.Share != 0.6667 || rep.LocatedBy.ToolUseID != 1 || rep.LocatedBy.Time != 2 || rep.NotComputable != (roBaselineNoCmp{Total: 3, TranscriptGone: 1, IDAbsent: 1, NamesNothing: 1}) {
		t.Errorf("baseline counts changed: %+v", rep)
	}
	// Numbers alone cannot prove this refactor: pin the delegation as well.
	f, err := parser.ParseFile(token.NewFileSet(), "readoutreport_baseline.go", nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	shared := false
	for _, decl := range f.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok || fn.Name.Name != "blReadFile" {
			continue
		}
		ast.Inspect(fn.Body, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok {
				return true
			}
			sel, ok := call.Fun.(*ast.SelectorExpr)
			if !ok {
				return true
			}
			pkg, ok := sel.X.(*ast.Ident)
			if ok && pkg.Name == "transcript" && (sel.Sel.Name == "WalkFile" || sel.Sel.Name == "WalkLines") {
				shared = true
			}
			if sel.Sel.Name == "ReadSlice" || sel.Sel.Name == "ReadBytes" {
				t.Error("baseline still owns a transcript read loop")
			}
			return true
		})
	}
	if !shared {
		t.Error("baseline does not delegate to the shared transcript walker")
	}
}
