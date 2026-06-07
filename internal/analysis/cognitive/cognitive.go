// Package cognitive computes gocognit-style cognitive complexity for Go
// functions. Cognitive complexity differs from cyclomatic complexity by
// weighting nested control structures more heavily: each level of nesting adds
// an extra +1 to every control-flow increment inside it.
//
// Increments are added for:
//   - if / else if / else  (+1 + nesting for if/else-if, +1 for else)
//   - for / range          (+1 + nesting)
//   - switch               (+1 + nesting)
//   - select               (+1 + nesting)
//   - &&, ||               (+1 each, no nesting)
//   - goto                 (+1)
//
// The Score function parses a Go source string and returns the total cognitive
// complexity summed across all functions.  Analyze returns per-function detail.
//
// All exported functions are pure, deterministic, and safe for concurrent use.
package cognitive

import (
	"go/ast"
	"go/parser"
	"go/token"
	"sort"
)

// FuncCognitive is the cognitive-complexity result for a single function.
type FuncCognitive struct {
	Func  string // function / method name
	File  string // source file path (as provided to Analyze)
	Line  int    // 1-based line of the func declaration
	Score int    // cognitive complexity score
}

// Analyze parses the Go source in src (filename is used for error messages and
// stored in FuncCognitive.File) and returns cognitive complexity for every
// function/method, sorted descending by Score then ascending by Func name.
func Analyze(filename, src string) ([]FuncCognitive, error) {
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, filename, src, 0)
	if err != nil {
		return nil, err
	}

	var results []FuncCognitive
	ast.Inspect(f, func(n ast.Node) bool {
		fd, ok := n.(*ast.FuncDecl)
		if !ok {
			return true
		}
		name := fd.Name.Name
		if fd.Recv != nil && len(fd.Recv.List) > 0 {
			// prepend receiver type name
			if t := receiverName(fd.Recv.List[0].Type); t != "" {
				name = t + "." + name
			}
		}
		pos := fset.Position(fd.Pos())
		score := funcScore(fd.Body)
		results = append(results, FuncCognitive{
			Func:  name,
			File:  filename,
			Line:  pos.Line,
			Score: score,
		})
		return true
	})

	sort.Slice(results, func(i, j int) bool {
		if results[i].Score != results[j].Score {
			return results[i].Score > results[j].Score
		}
		if results[i].Line != results[j].Line {
			return results[i].Line < results[j].Line
		}
		return results[i].Func < results[j].Func
	})
	return results, nil
}

// Score parses src as Go source and returns the total cognitive complexity
// summed across all functions. Returns 0 on parse error.
func Score(src string) int {
	results, err := Analyze("<input>", src)
	if err != nil {
		return 0
	}
	total := 0
	for _, r := range results {
		total += r.Score
	}
	return total
}

// receiverName extracts the base type name from a receiver expression.
func receiverName(expr ast.Expr) string {
	switch t := expr.(type) {
	case *ast.StarExpr:
		return receiverName(t.X)
	case *ast.Ident:
		return t.Name
	case *ast.IndexExpr: // generic receiver T[P]
		return receiverName(t.X)
	}
	return ""
}

// funcScore computes the cognitive complexity of a function body.
func funcScore(body *ast.BlockStmt) int {
	if body == nil {
		return 0
	}
	v := &cogVisitor{}
	v.walkBlock(body, 0)
	return v.score
}

type cogVisitor struct {
	score int
}

// walkStmt visits a statement, tracking nesting depth.
func (v *cogVisitor) walkStmt(s ast.Stmt, depth int) {
	if s == nil {
		return
	}
	switch st := s.(type) {
	case *ast.IfStmt:
		v.score += 1 + depth
		// init
		if st.Init != nil {
			v.walkStmt(st.Init, depth)
		}
		// condition (boolean operators)
		v.walkExprBoolOps(st.Cond)
		v.walkBlock(st.Body, depth+1)
		// else / else-if
		if st.Else != nil {
			switch el := st.Else.(type) {
			case *ast.IfStmt:
				// else if: +1, no nesting increment (structural penalty already done)
				v.score++
				v.walkExprBoolOps(el.Cond)
				v.walkBlock(el.Body, depth+1)
				if el.Else != nil {
					v.walkElseChain(el.Else, depth)
				}
			case *ast.BlockStmt:
				// plain else: +1
				v.score++
				v.walkBlock(el, depth+1)
			}
		}

	case *ast.ForStmt:
		v.score += 1 + depth
		if st.Init != nil {
			v.walkStmt(st.Init, depth)
		}
		if st.Cond != nil {
			v.walkExprBoolOps(st.Cond)
		}
		if st.Post != nil {
			v.walkStmt(st.Post, depth)
		}
		v.walkBlock(st.Body, depth+1)

	case *ast.RangeStmt:
		v.score += 1 + depth
		v.walkBlock(st.Body, depth+1)

	case *ast.SwitchStmt:
		v.score += 1 + depth
		if st.Init != nil {
			v.walkStmt(st.Init, depth)
		}
		if st.Tag != nil {
			v.walkExprBoolOps(st.Tag)
		}
		v.walkBlock(st.Body, depth+1)

	case *ast.TypeSwitchStmt:
		v.score += 1 + depth
		if st.Init != nil {
			v.walkStmt(st.Init, depth)
		}
		v.walkBlock(st.Body, depth+1)

	case *ast.SelectStmt:
		v.score += 1 + depth
		v.walkBlock(st.Body, depth+1)

	case *ast.CaseClause:
		for _, e := range st.List {
			v.walkExprBoolOps(e)
		}
		for _, s2 := range st.Body {
			v.walkStmt(s2, depth)
		}

	case *ast.CommClause:
		for _, s2 := range st.Body {
			v.walkStmt(s2, depth)
		}

	case *ast.BlockStmt:
		v.walkBlock(st, depth)

	case *ast.BranchStmt:
		if st.Tok == token.GOTO {
			v.score++
		}

	case *ast.ExprStmt:
		v.walkExprBoolOps(st.X)

	case *ast.AssignStmt:
		for _, e := range st.Rhs {
			v.walkExprBoolOps(e)
		}

	case *ast.ReturnStmt:
		for _, e := range st.Results {
			v.walkExprBoolOps(e)
		}

	case *ast.SendStmt:
		v.walkExprBoolOps(st.Value)

	case *ast.IncDecStmt:
		// no increment

	case *ast.DeclStmt:
		if gd, ok := st.Decl.(*ast.GenDecl); ok {
			for _, spec := range gd.Specs {
				if vs, ok2 := spec.(*ast.ValueSpec); ok2 {
					for _, val := range vs.Values {
						v.walkExprBoolOps(val)
					}
				}
			}
		}

	case *ast.GoStmt:
		v.walkExprBoolOps(st.Call)

	case *ast.DeferStmt:
		v.walkExprBoolOps(st.Call)
	}
}

func (v *cogVisitor) walkElseChain(else_ ast.Stmt, depth int) {
	switch el := else_.(type) {
	case *ast.IfStmt:
		v.score++
		v.walkExprBoolOps(el.Cond)
		v.walkBlock(el.Body, depth+1)
		if el.Else != nil {
			v.walkElseChain(el.Else, depth)
		}
	case *ast.BlockStmt:
		v.score++
		v.walkBlock(el, depth+1)
	}
}

func (v *cogVisitor) walkBlock(b *ast.BlockStmt, depth int) {
	if b == nil {
		return
	}
	for _, s := range b.List {
		v.walkStmt(s, depth)
	}
}

// walkExprBoolOps adds +1 for each && or || in an expression (no nesting
// penalty — gocognit only counts operator changes, but for simplicity we count
// each occurrence which gives the same result for simple chains).
func (v *cogVisitor) walkExprBoolOps(e ast.Expr) {
	if e == nil {
		return
	}
	ast.Inspect(e, func(n ast.Node) bool {
		be, ok := n.(*ast.BinaryExpr)
		if !ok {
			return true
		}
		if be.Op == token.LAND || be.Op == token.LOR {
			v.score++
		}
		return true
	})
}
