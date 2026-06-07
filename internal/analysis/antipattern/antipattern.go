// Package antipattern provides static-analysis scanners that detect common
// anti-patterns in Go source (via go/ast) and in arbitrary source text (via
// regex fallback).
//
// All exported functions are pure, deterministic, and safe for concurrent use.
// Each scanner returns a []Finding sorted by (File, Line, Rule).
package antipattern

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

// Finding is the result of a single scanner hit.
type Finding struct {
	File    string
	Line    int
	Rule    string
	Message string
}

// sortFindings sorts findings by (File, Line, Rule) for determinism.
func sortFindings(fs []Finding) {
	sort.Slice(fs, func(i, j int) bool {
		if fs[i].File != fs[j].File {
			return fs[i].File < fs[j].File
		}
		if fs[i].Line != fs[j].Line {
			return fs[i].Line < fs[j].Line
		}
		return fs[i].Rule < fs[j].Rule
	})
}

// parseFile parses Go source from src ([]byte or string) and returns the AST
// and token set. An empty fset+nil AST is returned when src cannot be parsed.
func parseFile(path string, src interface{}) (*token.FileSet, *ast.File) {
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, path, src, 0)
	if err != nil {
		return fset, nil
	}
	return fset, f
}

// isTestFile returns true when the file appears to be a Go test file.
func isTestFile(path string) bool {
	base := filepath.Base(path)
	return strings.HasSuffix(base, "_test.go")
}

// isMainFile returns true when the file is in a main package (heuristic).
func isMainFile(f *ast.File) bool {
	return f != nil && f.Name != nil && f.Name.Name == "main"
}

// ─── MagicNumbers ────────────────────────────────────────────────────────────

// MagicNumbers finds numeric integer/float literals that appear outside
// const/var declarations and whose value is not 0, 1, or -1.
func MagicNumbers(path string, src interface{}) []Finding {
	fset, f := parseFile(path, src)
	if f == nil {
		return nil
	}

	// Collect positions that are inside top-level const or var decls.
	constVarPos := map[token.Pos]bool{}
	for _, decl := range f.Decls {
		gd, ok := decl.(*ast.GenDecl)
		if !ok {
			continue
		}
		if gd.Tok == token.CONST || gd.Tok == token.VAR {
			ast.Inspect(gd, func(n ast.Node) bool {
				if n != nil {
					constVarPos[n.Pos()] = true
				}
				return true
			})
		}
	}

	// Walk the entire file; flag BasicLit numeric literals not in constVarPos
	// and not equal to 0, 1, or -1.
	var findings []Finding
	ast.Inspect(f, func(n ast.Node) bool {
		bl, ok := n.(*ast.BasicLit)
		if !ok {
			return true
		}
		if bl.Kind != token.INT && bl.Kind != token.FLOAT {
			return true
		}
		if constVarPos[bl.Pos()] {
			return true
		}
		if isTrivialNumeric(bl.Value) {
			return true
		}
		pos := fset.Position(bl.Pos())
		findings = append(findings, Finding{
			File:    path,
			Line:    pos.Line,
			Rule:    "magic-number",
			Message: fmt.Sprintf("magic number %s", bl.Value),
		})
		return true
	})

	sortFindings(findings)
	return findings
}

// isTrivialNumeric returns true when val represents 0, 1, or -1.
func isTrivialNumeric(val string) bool {
	// Try integer parse first.
	if i, err := strconv.ParseInt(val, 0, 64); err == nil {
		return i == 0 || i == 1 || i == -1
	}
	if f, err := strconv.ParseFloat(val, 64); err == nil {
		return f == 0 || f == 1 || f == -1
	}
	return false
}

// ─── PanicSites ──────────────────────────────────────────────────────────────

// PanicSites finds all panic() call sites in the given Go source.
func PanicSites(path string, src interface{}) []Finding {
	fset, f := parseFile(path, src)
	if f == nil {
		return nil
	}

	var findings []Finding
	ast.Inspect(f, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		ident, ok := call.Fun.(*ast.Ident)
		if !ok || ident.Name != "panic" {
			return true
		}
		pos := fset.Position(call.Pos())
		findings = append(findings, Finding{
			File:    path,
			Line:    pos.Line,
			Rule:    "panic-site",
			Message: "panic() call",
		})
		return true
	})

	sortFindings(findings)
	return findings
}

// ─── Goroutines ───────────────────────────────────────────────────────────────

// Goroutines finds all go statements in the given Go source.
func Goroutines(path string, src interface{}) []Finding {
	fset, f := parseFile(path, src)
	if f == nil {
		return nil
	}

	var findings []Finding
	ast.Inspect(f, func(n ast.Node) bool {
		gs, ok := n.(*ast.GoStmt)
		if !ok {
			return true
		}
		pos := fset.Position(gs.Pos())
		// Extract a short representation of what is being goroutined.
		label := "goroutine"
		if call, ok := gs.Call.Fun.(*ast.Ident); ok {
			label = fmt.Sprintf("go %s()", call.Name)
		} else if sel, ok := gs.Call.Fun.(*ast.SelectorExpr); ok {
			if x, ok := sel.X.(*ast.Ident); ok {
				label = fmt.Sprintf("go %s.%s()", x.Name, sel.Sel.Name)
			}
		}
		findings = append(findings, Finding{
			File:    path,
			Line:    pos.Line,
			Rule:    "goroutine",
			Message: label,
		})
		return true
	})

	sortFindings(findings)
	return findings
}

// ─── RegexInLoop ─────────────────────────────────────────────────────────────

// RegexInLoop finds calls to regexp.MustCompile or regexp.Compile that occur
// inside a for or range statement body.
func RegexInLoop(path string, src interface{}) []Finding {
	fset, f := parseFile(path, src)
	if f == nil {
		return nil
	}

	var findings []Finding

	// We walk the AST and track when we're inside a ForStmt or RangeStmt.
	var forDepth int
	var walk func(ast.Node) bool
	walk = func(n ast.Node) bool {
		if n == nil {
			return false
		}
		switch n.(type) {
		case *ast.ForStmt, *ast.RangeStmt:
			forDepth++
			ast.Inspect(n, func(inner ast.Node) bool {
				if inner == n {
					return true // skip the outer node itself
				}
				// Recurse through nested for loops.
				switch inner.(type) {
				case *ast.ForStmt, *ast.RangeStmt:
					forDepth++
					ast.Inspect(inner, func(deep ast.Node) bool {
						if deep == inner {
							return true
						}
						checkRegexpCall(fset, path, deep, &findings)
						return true
					})
					forDepth--
					return false
				}
				checkRegexpCall(fset, path, inner, &findings)
				return true
			})
			forDepth--
			return false
		}
		return true
	}
	ast.Inspect(f, walk)

	sortFindings(findings)
	return findings
}

func checkRegexpCall(fset *token.FileSet, path string, n ast.Node, findings *[]Finding) {
	call, ok := n.(*ast.CallExpr)
	if !ok {
		return
	}
	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok {
		return
	}
	pkg, ok := sel.X.(*ast.Ident)
	if !ok || pkg.Name != "regexp" {
		return
	}
	if sel.Sel.Name != "MustCompile" && sel.Sel.Name != "Compile" {
		return
	}
	pos := fset.Position(call.Pos())
	*findings = append(*findings, Finding{
		File:    path,
		Line:    pos.Line,
		Rule:    "regex-in-loop",
		Message: fmt.Sprintf("regexp.%s inside loop", sel.Sel.Name),
	})
}

// ─── WallClock ────────────────────────────────────────────────────────────────

// WallClock finds time.Now() calls in non-test, non-main Go files (business
// logic). Callers should pass a test/main file path to suppress the finding.
func WallClock(path string, src interface{}) []Finding {
	// Suppress for test and main files.
	fset, f := parseFile(path, src)
	if f == nil {
		return nil
	}
	if isTestFile(path) || isMainFile(f) {
		return nil
	}

	var findings []Finding
	ast.Inspect(f, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		sel, ok := call.Fun.(*ast.SelectorExpr)
		if !ok {
			return true
		}
		pkg, ok := sel.X.(*ast.Ident)
		if !ok || pkg.Name != "time" || sel.Sel.Name != "Now" {
			return true
		}
		pos := fset.Position(call.Pos())
		findings = append(findings, Finding{
			File:    path,
			Line:    pos.Line,
			Rule:    "wall-clock",
			Message: "time.Now() in business logic; inject a clock interface instead",
		})
		return true
	})

	sortFindings(findings)
	return findings
}

// ─── MutexAudit ──────────────────────────────────────────────────────────────

// MutexAudit reports:
//  1. Struct fields of type sync.Mutex (potential embedding / value-copy risk).
//  2. Lock() calls not followed by a matching Unlock() in the same function
//     (simple balance heuristic: count Lock vs Unlock calls per function).
func MutexAudit(path string, src interface{}) []Finding {
	fset, f := parseFile(path, src)
	if f == nil {
		return nil
	}

	var findings []Finding

	// 1. Struct field scan.
	ast.Inspect(f, func(n ast.Node) bool {
		st, ok := n.(*ast.StructType)
		if !ok {
			return true
		}
		for _, field := range st.Fields.List {
			sel, ok := field.Type.(*ast.SelectorExpr)
			if !ok {
				continue
			}
			pkg, ok := sel.X.(*ast.Ident)
			if !ok || pkg.Name != "sync" {
				continue
			}
			if sel.Sel.Name != "Mutex" && sel.Sel.Name != "RWMutex" {
				continue
			}
			pos := fset.Position(field.Pos())
			names := "<embedded>"
			if len(field.Names) > 0 {
				ns := make([]string, len(field.Names))
				for i, id := range field.Names {
					ns[i] = id.Name
				}
				names = strings.Join(ns, ", ")
			}
			findings = append(findings, Finding{
				File:    path,
				Line:    pos.Line,
				Rule:    "mutex-field",
				Message: fmt.Sprintf("sync.%s field %q — ensure pointer receiver to avoid value-copy", sel.Sel.Name, names),
			})
		}
		return true
	})

	// 2. Lock/Unlock balance per function declaration.
	for _, decl := range f.Decls {
		fd, ok := decl.(*ast.FuncDecl)
		if !ok || fd.Body == nil {
			continue
		}
		locks, unlocks := 0, 0
		ast.Inspect(fd.Body, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok {
				return true
			}
			sel, ok := call.Fun.(*ast.SelectorExpr)
			if !ok {
				return true
			}
			switch sel.Sel.Name {
			case "Lock", "RLock":
				locks++
			case "Unlock", "RUnlock":
				unlocks++
			}
			return true
		})
		if locks > 0 && locks != unlocks {
			pos := fset.Position(fd.Pos())
			findings = append(findings, Finding{
				File:    path,
				Line:    pos.Line,
				Rule:    "mutex-imbalance",
				Message: fmt.Sprintf("func %s: %d Lock(s) vs %d Unlock(s) — verify defer Unlock() pattern", fd.Name.Name, locks, unlocks),
			})
		}
	}

	sortFindings(findings)
	return findings
}

// ─── ScanAll ─────────────────────────────────────────────────────────────────

// ScanAll runs all anti-pattern scanners and returns a merged, sorted
// []Finding.
func ScanAll(path string, src interface{}) []Finding {
	runners := []func(string, interface{}) []Finding{
		MagicNumbers,
		PanicSites,
		Goroutines,
		RegexInLoop,
		WallClock,
		MutexAudit,
	}
	var all []Finding
	for _, run := range runners {
		all = append(all, run(path, src)...)
	}
	sortFindings(all)
	return all
}
