// Package ineffassign detects assignments whose value is never subsequently
// read before the variable is reassigned or the function returns.
//
// This is a best-effort, intra-statement-list analysis — it does not perform
// full data-flow analysis across branches. The algorithm walks each function
// body's statement list linearly and maintains a set of "written but unread"
// variables.  When a variable is read, it is removed from the set.  When a
// variable is written again while still in the set, the previous write is
// flagged as ineffectual.
//
// Limitations (by design):
//   - Variables used only in sub-blocks (if/for bodies) are conservatively
//     assumed to be read.
//   - Blank identifier assignments (_) are always skipped.
//   - Short variable declarations (:=) are treated as writes.
//
// All exported functions are pure, deterministic, and safe for concurrent use.
package ineffassign

import (
	"go/ast"
	"go/parser"
	"go/token"
	"sort"
)

// Finding records a single ineffectual assignment.
type Finding struct {
	File string
	Line int
	Var  string
}

// Analyze parses the Go source in src and returns ineffectual-assignment
// findings, sorted by File, Line, Var for determinism.
func Analyze(filename, src string) ([]Finding, error) {
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, filename, src, 0)
	if err != nil {
		return nil, err
	}

	var findings []Finding

	ast.Inspect(f, func(n ast.Node) bool {
		fd, ok := n.(*ast.FuncDecl)
		if !ok {
			return true
		}
		if fd.Body == nil {
			return true
		}
		ff := analyzeBlock(fd.Body, fset, filename)
		findings = append(findings, ff...)
		return true
	})

	sort.Slice(findings, func(i, j int) bool {
		if findings[i].File != findings[j].File {
			return findings[i].File < findings[j].File
		}
		if findings[i].Line != findings[j].Line {
			return findings[i].Line < findings[j].Line
		}
		return findings[i].Var < findings[j].Var
	})
	return findings, nil
}

// writeRecord tracks a write to a variable.
type writeRecord struct {
	line int
	name string
}

// analyzeBlock performs the linear scan of a block statement.
func analyzeBlock(body *ast.BlockStmt, fset *token.FileSet, filename string) []Finding {
	var findings []Finding
	// unread maps variable name -> write position (line).
	// A variable present here was written but not yet read.
	unread := make(map[string]writeRecord)

	// markRead removes var from unread (it has been read).
	markRead := func(name string) {
		delete(unread, name)
	}
	// markWrite records a write; if var already unread, flag and update.
	markWrite := func(name string, line int) []Finding {
		if name == "_" {
			return nil
		}
		if prev, exists := unread[name]; exists {
			findings = append(findings, Finding{
				File: filename,
				Line: prev.line,
				Var:  prev.name,
			})
		}
		unread[name] = writeRecord{line: line, name: name}
		return nil
	}

	for _, stmt := range body.List {
		processStmt(stmt, fset, markRead, markWrite, &findings)
	}

	return findings
}

func processStmt(
	stmt ast.Stmt,
	fset *token.FileSet,
	markRead func(string),
	markWrite func(string, int) []Finding,
	findings *[]Finding,
) {
	if stmt == nil {
		return
	}
	switch s := stmt.(type) {
	case *ast.AssignStmt:
		// First read RHS (uses)
		for _, rhs := range s.Rhs {
			collectReads(rhs, markRead)
		}
		// Then write LHS
		if s.Tok == token.ASSIGN || s.Tok == token.DEFINE {
			for _, lhs := range s.Lhs {
				if ident, ok := lhs.(*ast.Ident); ok {
					line := fset.Position(s.Pos()).Line
					markWrite(ident.Name, line)
				}
			}
		} else {
			// compound assignment (+=, etc.) — read then write
			for _, lhs := range s.Lhs {
				if ident, ok := lhs.(*ast.Ident); ok {
					markRead(ident.Name)
					line := fset.Position(s.Pos()).Line
					markWrite(ident.Name, line)
				}
			}
		}

	case *ast.IncDecStmt:
		// x++ reads and writes x
		if ident, ok := s.X.(*ast.Ident); ok {
			markRead(ident.Name)
			line := fset.Position(s.Pos()).Line
			markWrite(ident.Name, line)
		}

	case *ast.ReturnStmt:
		for _, r := range s.Results {
			collectReads(r, markRead)
		}

	case *ast.ExprStmt:
		collectReads(s.X, markRead)

	case *ast.SendStmt:
		collectReads(s.Chan, markRead)
		collectReads(s.Value, markRead)

	case *ast.DeclStmt:
		if gd, ok := s.Decl.(*ast.GenDecl); ok {
			for _, spec := range gd.Specs {
				if vs, ok2 := spec.(*ast.ValueSpec); ok2 {
					for _, val := range vs.Values {
						collectReads(val, markRead)
					}
					for _, name := range vs.Names {
						line := fset.Position(s.Pos()).Line
						markWrite(name.Name, line)
					}
				}
			}
		}

	case *ast.IfStmt:
		if s.Init != nil {
			processStmt(s.Init, fset, markRead, markWrite, findings)
		}
		collectReads(s.Cond, markRead)
		// Conservatively mark all vars written inside if/else as read in outer scope
		markSubBlockReads(s.Body, markRead)
		if s.Else != nil {
			if eb, ok := s.Else.(*ast.BlockStmt); ok {
				markSubBlockReads(eb, markRead)
			} else if ei, ok := s.Else.(*ast.IfStmt); ok {
				markSubBlockReads(ei.Body, markRead)
			}
		}

	case *ast.ForStmt:
		if s.Init != nil {
			processStmt(s.Init, fset, markRead, markWrite, findings)
		}
		if s.Cond != nil {
			collectReads(s.Cond, markRead)
		}
		if s.Post != nil {
			collectReads(s.Post, markRead)
		}
		markSubBlockReads(s.Body, markRead)

	case *ast.RangeStmt:
		collectReads(s.X, markRead)
		markSubBlockReads(s.Body, markRead)

	case *ast.SwitchStmt:
		if s.Init != nil {
			processStmt(s.Init, fset, markRead, markWrite, findings)
		}
		if s.Tag != nil {
			collectReads(s.Tag, markRead)
		}
		markSubBlockReads(s.Body, markRead)

	case *ast.TypeSwitchStmt:
		if s.Init != nil {
			processStmt(s.Init, fset, markRead, markWrite, findings)
		}
		markSubBlockReads(s.Body, markRead)

	case *ast.SelectStmt:
		markSubBlockReads(s.Body, markRead)

	case *ast.BlockStmt:
		markSubBlockReads(s, markRead)

	case *ast.GoStmt:
		collectReads(s.Call, markRead)

	case *ast.DeferStmt:
		collectReads(s.Call, markRead)
	}
}

// markSubBlockReads conservatively marks all identifiers appearing in a block
// as "read" in the parent scope, preventing false positives for variables that
// flow into sub-blocks.
func markSubBlockReads(b *ast.BlockStmt, markRead func(string)) {
	if b == nil {
		return
	}
	ast.Inspect(b, func(n ast.Node) bool {
		if id, ok := n.(*ast.Ident); ok {
			markRead(id.Name)
		}
		return true
	})
}

// collectReads walks an expression and calls markRead for every identifier
// that appears in a read position.
func collectReads(expr ast.Node, markRead func(string)) {
	if expr == nil {
		return
	}
	ast.Inspect(expr, func(n ast.Node) bool {
		if id, ok := n.(*ast.Ident); ok {
			if id.Name != "_" {
				markRead(id.Name)
			}
		}
		return true
	})
}
