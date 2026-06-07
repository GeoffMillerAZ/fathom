// Package erraudit audits Go error-handling patterns via go/ast.
//
// Three patterns are detected:
//   - Ignored errors: assignments of the form `_ = someCall()` where the call
//     returns an error.
//   - Log-only error handlers: `if err != nil` blocks whose body contains only
//     a logging call (no return/panic/os.Exit).
//   - Naked error returns: return statements that return an `err` variable
//     directly with no wrapping or annotation.
//
// All exported functions are pure, deterministic, and safe for concurrent use.
package erraudit

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"sort"
	"strings"
)

// Finding is a single error-handling concern.
type Finding struct {
	File    string
	Line    int
	Message string
}

// Scan parses Go source in src (filename path for attribution) and returns a
// deterministic list of error-handling Findings.
func Scan(path, src string) []Finding {
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, path, src, 0)
	if err != nil {
		return []Finding{{
			File:    path,
			Line:    0,
			Message: fmt.Sprintf("parse error: %v", err),
		}}
	}

	var findings []Finding

	ast.Inspect(f, func(n ast.Node) bool {
		switch node := n.(type) {
		case *ast.AssignStmt:
			findings = append(findings, checkIgnoredError(fset, path, node)...)
		case *ast.IfStmt:
			findings = append(findings, checkLogOnlyHandler(fset, path, node)...)
		case *ast.ReturnStmt:
			findings = append(findings, checkNakedErrReturn(fset, path, node)...)
		}
		return true
	})

	sort.Slice(findings, func(i, j int) bool {
		if findings[i].Line != findings[j].Line {
			return findings[i].Line < findings[j].Line
		}
		return findings[i].Message < findings[j].Message
	})
	return findings
}

// ── Pattern: ignored errors (`_ = call()`) ───────────────────────────────────

func checkIgnoredError(fset *token.FileSet, path string, stmt *ast.AssignStmt) []Finding {
	var out []Finding
	// Look for assignments where the LHS is the blank identifier "_".
	for i, lhs := range stmt.Lhs {
		ident, ok := lhs.(*ast.Ident)
		if !ok || ident.Name != "_" {
			continue
		}
		// Check the corresponding RHS expression — is it a call?
		if i < len(stmt.Rhs) {
			if isCall(stmt.Rhs[i]) {
				line := fset.Position(stmt.Pos()).Line
				out = append(out, Finding{
					File:    path,
					Line:    line,
					Message: fmt.Sprintf("ignored error: result of call at line %d is discarded via blank identifier", line),
				})
			}
		}
	}
	return out
}

func isCall(expr ast.Expr) bool {
	_, ok := expr.(*ast.CallExpr)
	return ok
}

// ── Pattern: log-only error handlers ─────────────────────────────────────────

// checkLogOnlyHandler detects `if err != nil { log.*(…) }` blocks.
func checkLogOnlyHandler(fset *token.FileSet, path string, stmt *ast.IfStmt) []Finding {
	if !isErrNotNil(stmt.Cond) {
		return nil
	}
	if stmt.Body == nil || len(stmt.Body.List) == 0 {
		return nil
	}
	// The block must contain only logging calls and no control-flow statements.
	if hasControlFlow(stmt.Body.List) {
		return nil
	}
	if allLoggingCalls(stmt.Body.List) {
		line := fset.Position(stmt.Pos()).Line
		return []Finding{{
			File:    path,
			Line:    line,
			Message: fmt.Sprintf("log-only error handler at line %d: error is logged but not returned or handled", line),
		}}
	}
	return nil
}

// isErrNotNil returns true for `err != nil` binary expressions.
func isErrNotNil(expr ast.Expr) bool {
	bin, ok := expr.(*ast.BinaryExpr)
	if !ok {
		return false
	}
	if bin.Op.String() != "!=" {
		return false
	}
	lhs, lok := bin.X.(*ast.Ident)
	rhs, rok := bin.Y.(*ast.Ident)
	if !lok || !rok {
		return false
	}
	return lhs.Name == "err" && rhs.Name == "nil"
}

// hasControlFlow returns true when the statement list contains a return,
// panic, goto, or break/continue statement.
func hasControlFlow(stmts []ast.Stmt) bool {
	for _, s := range stmts {
		switch s.(type) {
		case *ast.ReturnStmt, *ast.BranchStmt:
			return true
		case *ast.ExprStmt:
			// os.Exit counts as control flow.
			expr, ok := s.(*ast.ExprStmt)
			if !ok {
				continue
			}
			call, ok := expr.X.(*ast.CallExpr)
			if !ok {
				continue
			}
			if sel, ok := call.Fun.(*ast.SelectorExpr); ok {
				if pkg, ok := sel.X.(*ast.Ident); ok {
					if pkg.Name == "os" && sel.Sel.Name == "Exit" {
						return true
					}
				}
			}
			if ident, ok := call.Fun.(*ast.Ident); ok {
				if ident.Name == "panic" {
					return true
				}
			}
		}
	}
	return false
}

// allLoggingCalls returns true when every statement is a call to a logging
// function (log.*, logger.*, fmt.Print*).
func allLoggingCalls(stmts []ast.Stmt) bool {
	if len(stmts) == 0 {
		return false
	}
	for _, s := range stmts {
		expr, ok := s.(*ast.ExprStmt)
		if !ok {
			return false
		}
		call, ok := expr.X.(*ast.CallExpr)
		if !ok {
			return false
		}
		if !isLoggingCall(call) {
			return false
		}
	}
	return true
}

// isLoggingCall returns true when the call looks like a logging invocation.
func isLoggingCall(call *ast.CallExpr) bool {
	switch fn := call.Fun.(type) {
	case *ast.SelectorExpr:
		pkg, ok := fn.X.(*ast.Ident)
		if !ok {
			return false
		}
		pkgName := strings.ToLower(pkg.Name)
		methName := strings.ToLower(fn.Sel.Name)
		// Standard log package: log.Print, log.Println, log.Printf,
		//                        log.Fatal, log.Error, log.Warn, log.Info, …
		if pkgName == "log" || strings.HasSuffix(pkgName, "log") ||
			pkgName == "logger" || strings.HasSuffix(pkgName, "logger") {
			return true
		}
		// fmt.Print* variants often used for ad-hoc logging.
		if pkgName == "fmt" && (strings.HasPrefix(methName, "print") || methName == "fprintf") {
			return true
		}
		// slog
		if pkgName == "slog" {
			return true
		}
		return false
	case *ast.Ident:
		// Bare function calls named like log/print.
		name := strings.ToLower(fn.Name)
		return name == "log" || strings.HasPrefix(name, "log") || name == "print" || name == "println"
	}
	return false
}

// ── Pattern: naked error returns ─────────────────────────────────────────────

// checkNakedErrReturn detects `return err` or `return …, err` return statements
// that pass err directly (no wrapping).
func checkNakedErrReturn(fset *token.FileSet, path string, stmt *ast.ReturnStmt) []Finding {
	for _, result := range stmt.Results {
		ident, ok := result.(*ast.Ident)
		if ok && ident.Name == "err" {
			line := fset.Position(stmt.Pos()).Line
			return []Finding{{
				File:    path,
				Line:    line,
				Message: fmt.Sprintf("naked error return at line %d: consider wrapping err with fmt.Errorf or errors.New for context", line),
			}}
		}
	}
	return nil
}
