package main

import (
	"encoding/json"
	"fmt"
	"go/ast"
	"go/types"
	"golang.org/x/tools/go/packages"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

type Row struct {
	Symbol  string   `json:"symbol"`
	Calls   int      `json:"calls"`
	Modules []string `json:"modules"`
	Sites   []string `json:"sites"`
}

func main() {
	root, _ := filepath.Abs(os.Args[1])
	cfg := &packages.Config{Dir: root, Mode: packages.NeedName | packages.NeedFiles | packages.NeedCompiledGoFiles | packages.NeedImports | packages.NeedDeps | packages.NeedSyntax | packages.NeedTypes | packages.NeedTypesInfo, Tests: true}
	pkgs, err := packages.Load(cfg, "./...")
	if err != nil {
		panic(err)
	}
	if packages.PrintErrors(pkgs) > 0 {
		os.Exit(2)
	}
	sites := map[string]map[string]bool{}
	mods := map[string]map[string]bool{}
	for _, p := range pkgs {
		if strings.HasSuffix(p.PkgPath, ".test") {
			continue
		}
		for _, f := range p.Syntax {
			ast.Inspect(f, func(n ast.Node) bool {
				call, ok := n.(*ast.CallExpr)
				if !ok {
					return true
				}
				var obj types.Object
				switch fun := call.Fun.(type) {
				case *ast.Ident:
					obj = p.TypesInfo.Uses[fun]
				case *ast.SelectorExpr:
					obj = p.TypesInfo.Uses[fun.Sel]
				}
				fn, ok := obj.(*types.Func)
				if !ok || fn.Pkg() == nil {
					return true
				}
				def := p.Fset.Position(fn.Pos())
				if !strings.HasPrefix(def.Filename, root+string(filepath.Separator)) {
					return true
				}
				signature := fn.Type().(*types.Signature)
				symbol := fn.Pkg().Path() + "." + fn.Name()
				if signature.Recv() != nil {
					symbol = fn.Pkg().Path() + "." + types.TypeString(signature.Recv().Type(), func(*types.Package) string { return "" }) + "." + fn.Name()
				}
				pos := p.Fset.Position(call.Pos())
				rel, _ := filepath.Rel(root, pos.Filename)
				site := fmt.Sprintf("%s:%d:%d", rel, pos.Line, pos.Column)
				if sites[symbol] == nil {
					sites[symbol] = map[string]bool{}
					mods[symbol] = map[string]bool{}
				}
				sites[symbol][site] = true
				mods[symbol][p.PkgPath] = true
				return true
			})
		}
	}
	var out []Row
	for sym, ss := range sites {
		row := Row{Symbol: sym, Calls: len(ss)}
		for m := range mods[sym] {
			row.Modules = append(row.Modules, m)
		}
		for s := range ss {
			row.Sites = append(row.Sites, s)
		}
		sort.Strings(row.Modules)
		sort.Strings(row.Sites)
		out = append(out, row)
	}
	sort.Slice(out, func(i, j int) bool {
		if len(out[i].Modules) != len(out[j].Modules) {
			return len(out[i].Modules) > len(out[j].Modules)
		}
		if out[i].Calls != out[j].Calls {
			return out[i].Calls > out[j].Calls
		}
		return out[i].Symbol < out[j].Symbol
	})
	e := json.NewEncoder(os.Stdout)
	e.SetIndent("", "  ")
	e.Encode(out)
}
