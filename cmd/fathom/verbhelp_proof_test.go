package main

// verbhelp_proof_test.go — the parse-first proof behind delegated verb help
// (O671, CL-6 and CL-7). Help hands a verb's registered handler exactly one
// argument list: ["-h"], or [sub, "-h"] for a subverb. This file reads the
// package's source, follows every verb's runner through its wrappers and
// subverb dispatch with that concrete list, and evaluates each statement
// the handler would run before fs.Parse: a statement is admitted only when
// it is neutral for that input (a branch the input never takes, a FlagSet,
// a flag definition, a literal) — anything else is pre-Parse work, and the
// handler is then answered from a static list (or, with no FlagSet on its
// path, the header alone), never by running it.

import (
	"bytes"
	"flag"
	"fmt"
	"go/ast"
	"go/parser"
	"go/printer"
	"go/token"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"testing"
)

// helpKind is how help answers a verb or subverb.
type helpKind int

const (
	helpDelegated  helpKind = iota // proven parse-first: help runs it with -h
	helpStatic                     // pre-Parse work: help prints verbHelpStatic
	helpHeaderOnly                 // no FlagSet on the help path: the header alone
)

func (k helpKind) String() string {
	return [...]string{"delegated", "static", "header-only"}[k]
}

// helpASTFlag is one flag definition read from source; Usage and Default
// are set only when the source spells them as literals.
type helpASTFlag struct {
	Name                     string
	Type                     string // helpType of the defining FlagSet method
	Usage, Default           string
	UsageKnown, DefaultKnown bool
}

// helpProof is the reading of the handler `fathom <Verb> [<Sub>]` reaches.
type helpProof struct {
	Verb, Sub string
	Kind      helpKind
	Chain     []string
	Flags     []helpASTFlag
	Problems  []string // the pre-Parse work that rules delegation out
}

// Key is the help-table key: "verb" or "verb sub".
func (p helpProof) Key() string { return helpKey(p.Verb, p.Sub) }

func (p helpProof) flagNames() []string {
	var out []string
	for _, f := range p.Flags {
		out = append(out, f.Name)
	}
	sort.Strings(out)
	return out
}

// helpFlagHelpers are the flag-only helpers a handler may call between its
// FlagSet and Parse: each is handed the FlagSet and only defines flags on
// it (the proof reads their flags from their bodies).
var helpFlagHelpers = map[string]bool{
	"addKnowledgeFlags": true,
	"addHarnessFlag":    true,
}

// helpPureCalls are the functions a flag default, a wrapper argument or a
// literal may call before Parse, each with the reason it runs no verb.
var helpPureCalls = map[string]string{
	"defaultStorePath":    "a default path from env, cwd and the git toplevel (RT-130 caches that answer in os.TempDir); opens no store",
	"defaultCallLogPath":  "a default path from env, cwd, the git toplevel and the state-dir config; opens no log",
	"defaultFathomURL":    "the daemon URL from $FATHOM_HTTP or the default address; contacts nothing",
	"beatLogPath":         "a default path from env and FATHOM_HOME",
	"preflightLedgerPath": "a default path from env and FATHOM_HOME",
	"supportKindNames":    "copies a package-level slice of kind names",
	"fleetSnapshotDir":    "a default path under FATHOM_HOME",
	"os.UserHomeDir":      "reads $HOME",
	"filepath.Join":       "pure",
	"resourcesEnvNow":     "liveResourcesEnv reads os.UserHomeDir and builds closures; nothing runs until the verb calls them",
	"liveReapEnv":         "builds closures and reads this process's pid and ppid; nothing runs until the verb calls them",
	"strings.Join":        "pure",
	"fmt.Sprintf":         "pure",
}

// helpRuntime evaluates, on the concrete help input, the argument helpers a
// handler consults before Parse. Each is proven pure from its body
// (pureHelper) before the proof calls it.
type helpRuntime struct {
	preds     map[string]func(args []string, lits []string) bool
	splitters map[string]func(args []string) ([]string, []string)
	parseArgs map[string]func(args []string) []string // fs.Parse(f(fs, args))
}

// helpPurePrimitives are the calls a pure helper's body may make.
var helpPurePrimitives = map[string]bool{
	"append": true, "len": true, "make": true, "cap": true, "copy": true,
}

type helpProofIndex struct {
	fset    *token.FileSet
	funcs   map[string]*ast.FuncDecl
	methods map[string]*ast.FuncDecl
	runners map[string]string // verb -> runner type name
	rt      helpRuntime
}

func loadHelpProofIndex(dir string, rt helpRuntime) (*helpProofIndex, error) {
	fset := token.NewFileSet()
	ents, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	ix := &helpProofIndex{fset: fset, funcs: map[string]*ast.FuncDecl{}, methods: map[string]*ast.FuncDecl{}, runners: map[string]string{}, rt: rt}
	for _, e := range ents {
		n := e.Name()
		if !strings.HasSuffix(n, ".go") || strings.HasSuffix(n, "_test.go") {
			continue
		}
		f, err := parser.ParseFile(fset, filepath.Join(dir, n), nil, parser.SkipObjectResolution)
		if err != nil {
			return nil, err
		}
		for _, d := range f.Decls {
			fd, ok := d.(*ast.FuncDecl)
			if !ok {
				continue
			}
			switch {
			case fd.Recv != nil && len(fd.Recv.List) == 1:
				ix.methods[hpRecvName(fd.Recv.List[0].Type)+"."+fd.Name.Name] = fd
			case fd.Name.Name == "init":
				ix.readDeclarations(fd)
			default:
				ix.funcs[fd.Name.Name] = fd
			}
		}
	}
	return ix, nil
}

func hpRecvName(e ast.Expr) string {
	switch t := e.(type) {
	case *ast.StarExpr:
		return hpRecvName(t.X)
	case *ast.Ident:
		return t.Name
	}
	return ""
}

func hpFuncName(fd *ast.FuncDecl) string {
	if fd.Recv != nil && len(fd.Recv.List) == 1 {
		return hpRecvName(fd.Recv.List[0].Type) + "." + fd.Name.Name
	}
	return fd.Name.Name
}

// readDeclarations maps each declareVerb(verb{Name: ..., Run: xRunner{}}).
func (ix *helpProofIndex) readDeclarations(fd *ast.FuncDecl) {
	ast.Inspect(fd.Body, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok || !hpIsIdent(call.Fun, "declareVerb") || len(call.Args) != 1 {
			return true
		}
		lit, ok := call.Args[0].(*ast.CompositeLit)
		if !ok {
			return true
		}
		var name, runner string
		for _, el := range lit.Elts {
			kv, ok := el.(*ast.KeyValueExpr)
			if !ok {
				continue
			}
			switch {
			case hpIsIdent(kv.Key, "Name"):
				name = hpStringLit(kv.Value)
			case hpIsIdent(kv.Key, "Run"):
				if cl, ok := kv.Value.(*ast.CompositeLit); ok {
					runner = hpRecvName(cl.Type)
				}
			}
		}
		if name != "" {
			ix.runners[name] = runner
		}
		return true
	})
}

func hpIsIdent(e ast.Expr, name string) bool {
	id, ok := e.(*ast.Ident)
	return ok && id.Name == name
}

func hpStringLit(e ast.Expr) string {
	s, _ := hpStringLitOK(e)
	return s
}

func hpStringLitOK(e ast.Expr) (string, bool) {
	switch x := e.(type) {
	case *ast.BasicLit:
		if x.Kind != token.STRING {
			return "", false
		}
		s, err := strconv.Unquote(x.Value)
		return s, err == nil
	case *ast.BinaryExpr: // "a" + "b"
		if x.Op != token.ADD {
			return "", false
		}
		a, ok1 := hpStringLitOK(x.X)
		b, ok2 := hpStringLitOK(x.Y)
		return a + b, ok1 && ok2
	case *ast.ParenExpr:
		return hpStringLitOK(x.X)
	}
	return "", false
}

func (ix *helpProofIndex) src(n ast.Node) string {
	var b bytes.Buffer
	_ = printer.Fprint(&b, ix.fset, n)
	s := strings.Join(strings.Fields(b.String()), " ")
	if len(s) > 100 {
		s = s[:100] + "…"
	}
	return s
}

func (ix *helpProofIndex) pos(n ast.Node) string {
	p := ix.fset.Position(n.Pos())
	return fmt.Sprintf("%s:%d", filepath.Base(p.Filename), p.Line)
}

func hpCallee(e ast.Expr) string {
	switch f := e.(type) {
	case *ast.Ident:
		return f.Name
	case *ast.SelectorExpr:
		if id, ok := f.X.(*ast.Ident); ok {
			return id.Name + "." + f.Sel.Name
		}
		return "?." + f.Sel.Name
	}
	return "?"
}

// impure returns "" when e evaluates without side effects: literals,
// identifiers, selectors, composite literals, closures (values; Parse runs
// a Func flag's closure only for a set flag), conversions, operators over
// those, and calls to helpPureCalls.
func (ix *helpProofIndex) impure(e ast.Expr) string {
	why := ""
	ast.Inspect(e, func(n ast.Node) bool {
		if why != "" {
			return false
		}
		switch x := n.(type) {
		case *ast.FuncLit:
			return false
		case *ast.CallExpr:
			fn := hpCallee(x.Fun)
			if _, ok := helpPureCalls[fn]; ok {
				return true
			}
			switch fn {
			case "string", "int", "int64", "time.Duration", "len":
				return true
			}
			if _, ok := x.Fun.(*ast.ArrayType); ok {
				return true
			}
			why = "calls " + fn
			return false
		}
		return true
	})
	return why
}

// pureHelper proves fn's body side-effect free: it calls only builtins,
// strings.*, other pure helpers and methods on its own values, and assigns
// only names it declares.
func (ix *helpProofIndex) pureHelper(name string, seen map[string]bool) string {
	if seen[name] {
		return ""
	}
	seen[name] = true
	fd := ix.funcs[name]
	if fd == nil {
		return name + " not found"
	}
	local := map[string]bool{}
	for _, fl := range fd.Type.Params.List {
		for _, id := range fl.Names {
			local[id.Name] = true
		}
	}
	if fd.Type.Results != nil {
		for _, fl := range fd.Type.Results.List {
			for _, id := range fl.Names {
				local[id.Name] = true
			}
		}
	}
	why := ""
	ast.Inspect(fd.Body, func(n ast.Node) bool {
		if why != "" {
			return false
		}
		switch x := n.(type) {
		case *ast.AssignStmt:
			for _, l := range x.Lhs {
				id := hpRootIdent(l)
				if id == nil {
					continue
				}
				if x.Tok == token.DEFINE {
					local[id.Name] = true
				} else if !local[id.Name] && id.Name != "_" {
					why = name + " assigns " + id.Name
				}
			}
		case *ast.RangeStmt:
			for _, e := range []ast.Expr{x.Key, x.Value} {
				if id, ok := e.(*ast.Ident); ok {
					local[id.Name] = true
				}
			}
		case *ast.ValueSpec:
			for _, id := range x.Names {
				local[id.Name] = true
			}
		case *ast.IncDecStmt:
			if id := hpRootIdent(x.X); id != nil && !local[id.Name] {
				why = name + " increments " + id.Name
			}
		case *ast.GoStmt, *ast.DeferStmt, *ast.SendStmt:
			why = name + " has a go, defer or send"
		case *ast.CallExpr:
			fn := hpCallee(x.Fun)
			switch {
			case helpPurePrimitives[fn], strings.HasPrefix(fn, "strings."):
			case ix.funcs[fn] != nil:
				if w := ix.pureHelper(fn, seen); w != "" {
					why = w
				}
			default:
				// A method on a value the helper holds (fs.Lookup,
				// bf.IsBoolFlag): reads of its own arguments.
				se, ok := x.Fun.(*ast.SelectorExpr)
				if !ok {
					why = name + " calls " + fn
					break
				}
				if id, ok := se.X.(*ast.Ident); !ok || !local[id.Name] {
					why = name + " calls " + fn
					break
				}
				switch se.Sel.Name {
				case "Lookup", "IsBoolFlag":
				default:
					why = name + " calls " + fn
				}
			}
		}
		return true
	})
	return why
}

func hpRootIdent(e ast.Expr) *ast.Ident {
	for {
		switch x := e.(type) {
		case *ast.Ident:
			return x
		case *ast.IndexExpr:
			e = x.X
		case *ast.SelectorExpr:
			e = x.X
		case *ast.StarExpr:
			e = x.X
		case *ast.ParenExpr:
			e = x.X
		default:
			return nil
		}
	}
}

// --- the walk ---------------------------------------------------------

type hval struct {
	known bool
	v     any // []string, string, int, bool
}

type hstop struct{} // a problem ends the walk

type hwalker struct {
	ix     *helpProofIndex
	p      *helpProof
	fn     *ast.FuncDecl // the function being walked
	env    map[string]hval
	fs     string // the FlagSet's name once created
	fsFn   *ast.FuncDecl
	parsed bool
}

func (w *hwalker) fail(n ast.Node, format string, a ...any) {
	w.p.Problems = append(w.p.Problems, fmt.Sprintf("%s: %s", w.ix.pos(n), fmt.Sprintf(format, a...)))
	panic(hstop{})
}

// proveHelp reads the handler `fathom verb [sub]` reaches when help hands
// it ["-h"] or [sub, "-h"].
func (ix *helpProofIndex) proveHelp(verb, sub string) (p helpProof) {
	p = helpProof{Verb: verb, Sub: sub}
	in := []string{"-h"}
	if sub != "" {
		in = []string{sub, "-h"}
	}
	fd := ix.methods[ix.runners[verb]+".execVerb"]
	if fd == nil {
		p.Problems = append(p.Problems, "no execVerb for verb "+verb)
		p.Kind = helpStatic
		return p
	}
	w := &hwalker{ix: ix, p: &p}
	func() {
		defer func() {
			if r := recover(); r != nil {
				if _, ok := r.(hstop); !ok {
					panic(r)
				}
			}
		}()
		w.enter(fd, map[string]hval{hpParamName(fd, 0): {true, in}})
	}()
	switch {
	case w.parsed && len(p.Problems) == 0:
		p.Kind = helpDelegated
		p.Flags = ix.flagDefs(w.fsFn, w.fs)
	default:
		fsFn, fs := w.fsFn, w.fs
		if fsFn == nil {
			fsFn, fs = ix.flagSetIn(w.fn)
		}
		if src, ok := helpStaticSource[p.Key()]; ok && fsFn == nil {
			fd := ix.funcs[src]
			if fd == nil {
				fd = ix.methods[src]
			}
			fsFn, fs = ix.flagSetIn(fd)
		}
		if fsFn == nil {
			p.Kind = helpHeaderOnly
		} else {
			p.Kind = helpStatic
			p.Flags = ix.flagDefs(fsFn, fs)
		}
		if len(p.Problems) == 0 {
			p.Problems = append(p.Problems, "the help path ends before Parse")
		}
	}
	return p
}

// helpStaticSource names, for a static key whose FlagSet is built in a
// method the walk does not enter, the method that builds it.
var helpStaticSource = map[string]string{
	"serve-http":      "serveRun.configure",
	"certify suggest": "runCertifySuggest",
}

func hpParamName(fd *ast.FuncDecl, i int) string {
	n := 0
	for _, f := range fd.Type.Params.List {
		for _, id := range f.Names {
			if n == i {
				return id.Name
			}
			n++
		}
	}
	return ""
}

func (w *hwalker) enter(fd *ast.FuncDecl, env map[string]hval) {
	w.p.Chain = append(w.p.Chain, hpFuncName(fd))
	if len(w.p.Chain) > 10 {
		w.fail(fd, "dispatch chain deeper than 10")
	}
	w.fn, w.env = fd, env
	if !w.block(fd.Body.List) {
		w.fail(fd, "%s ends without Parse", hpFuncName(fd))
	}
}

// block walks stmts; it reports whether the path ended (returned or parsed).
func (w *hwalker) block(stmts []ast.Stmt) bool {
	for i, s := range stmts {
		if w.parsed {
			return true
		}
		if as, ok := s.(*ast.AssignStmt); ok && w.fs != "" && len(as.Rhs) == 1 && w.isParseCall(as.Rhs[0]) {
			w.parse(as.Rhs[0])
			if i+1 >= len(stmts) {
				w.fail(s, "Parse error is not checked")
			}
			is, ok := stmts[i+1].(*ast.IfStmt)
			if !ok {
				w.fail(s, "Parse error is not checked next")
			}
			w.errBranch(is.Body)
			return true
		}
		if w.stmt(s) {
			return true
		}
	}
	return false
}

func (w *hwalker) isParseCall(e ast.Expr) bool {
	call, ok := e.(*ast.CallExpr)
	if !ok {
		return false
	}
	se, ok := call.Fun.(*ast.SelectorExpr)
	return ok && hpIsIdent(se.X, w.fs) && se.Sel.Name == "Parse"
}

// parse requires fs.Parse to be handed exactly ["-h"].
func (w *hwalker) parse(e ast.Expr) {
	call := e.(*ast.CallExpr)
	if len(call.Args) != 1 {
		w.fail(call, "Parse arity")
	}
	v := w.eval(call.Args[0])
	got, ok := v.v.([]string)
	if !v.known || !ok {
		w.fail(call, "Parse is handed %s, not a known argument list", w.ix.src(call.Args[0]))
	}
	if !reflect.DeepEqual(got, []string{"-h"}) {
		w.fail(call, "Parse is handed %q, not the help token (%s)", got, w.ix.src(call.Args[0]))
	}
	w.parsed = true
}

func (w *hwalker) errBranch(b *ast.BlockStmt) {
	if len(b.List) != 1 {
		w.fail(b, "the Parse error branch does more than return")
	}
	if _, ok := b.List[0].(*ast.ReturnStmt); !ok {
		w.fail(b, "the Parse error branch does not return")
	}
}

// stmt walks one statement; it reports whether the path ended.
func (w *hwalker) stmt(s ast.Stmt) bool {
	switch x := s.(type) {
	case *ast.ReturnStmt:
		if w.fs != "" {
			w.fail(x, "returns before Parse")
		}
		if len(x.Results) == 1 {
			if call, ok := x.Results[0].(*ast.CallExpr); ok {
				if callee := w.forwardTarget(call); callee != nil {
					w.forward(call, callee)
					return true
				}
			}
		}
		for _, r := range x.Results {
			if why := w.ix.impure(r); why != "" {
				w.fail(x, "returns %s (%s)", w.ix.src(r), why)
			}
		}
		return true // the path ends here; no FlagSet means the header alone
	case *ast.IfStmt:
		if w.fs != "" {
			if x.Init != nil {
				if as, ok := x.Init.(*ast.AssignStmt); ok && len(as.Rhs) == 1 && w.isParseCall(as.Rhs[0]) {
					w.parse(as.Rhs[0])
					w.errBranch(x.Body)
					return true
				}
			}
			if be, ok := x.Cond.(*ast.BinaryExpr); ok && w.isParseCall(be.X) {
				w.parse(be.X)
				w.errBranch(x.Body)
				return true
			}
		}
		if x.Init != nil {
			w.assign(x.Init)
		}
		c := w.eval(x.Cond)
		b, ok := c.v.(bool)
		if !c.known || !ok {
			w.fail(x, "a branch the help input does not decide: if %s", w.ix.src(x.Cond))
		}
		if b {
			return w.block(x.Body.List)
		}
		if x.Else != nil {
			return w.stmt(x.Else)
		}
		return false
	case *ast.BlockStmt:
		return w.block(x.List)
	case *ast.SwitchStmt:
		if x.Init != nil {
			w.assign(x.Init)
		}
		var tag hval
		if x.Tag != nil {
			tag = w.eval(x.Tag)
			if !tag.known {
				w.fail(x, "a switch the help input does not decide: %s", w.ix.src(x.Tag))
			}
		}
		var dflt *ast.CaseClause
		for _, c := range x.Body.List {
			cc := c.(*ast.CaseClause)
			if cc.List == nil {
				dflt = cc
				continue
			}
			for _, e := range cc.List {
				v := w.eval(e)
				if !v.known {
					w.fail(e, "a case the help input does not decide: %s", w.ix.src(e))
				}
				if (x.Tag == nil && v.v == true) || (x.Tag != nil && v.v == tag.v) {
					return w.block(cc.Body)
				}
			}
		}
		if dflt != nil {
			return w.block(dflt.Body)
		}
		return false
	case *ast.RangeStmt:
		v := w.eval(x.X)
		list, ok := v.v.([]string)
		if !v.known || !ok || x.Tok != token.DEFINE {
			w.fail(x, "a loop the help input does not decide: range %s", w.ix.src(x.X))
		}
		for i, el := range list {
			if id, ok := x.Key.(*ast.Ident); ok && id.Name != "_" {
				w.env[id.Name] = hval{true, i}
			}
			if id, ok := x.Value.(*ast.Ident); ok && id.Name != "_" {
				w.env[id.Name] = hval{true, el}
			}
			if w.block(x.Body.List) {
				return true
			}
		}
		return false
	case *ast.AssignStmt:
		w.assign(x)
		return false
	case *ast.DeclStmt:
		gd, ok := x.Decl.(*ast.GenDecl)
		if !ok {
			w.fail(x, "declaration")
		}
		for _, sp := range gd.Specs {
			vs, ok := sp.(*ast.ValueSpec)
			if !ok {
				w.fail(x, "declaration")
			}
			for i, id := range vs.Names {
				v := hval{}
				if i < len(vs.Values) {
					if why := w.ix.impure(vs.Values[i]); why != "" {
						w.fail(x, "declares %s (%s)", w.ix.src(vs.Values[i]), why)
					}
					v = w.eval(vs.Values[i])
				}
				w.env[id.Name] = v
			}
		}
		return false
	case *ast.ExprStmt:
		call, ok := x.X.(*ast.CallExpr)
		if !ok || w.fs == "" {
			w.fail(x, "work before the FlagSet: %s", w.ix.src(x))
		}
		if why := w.flagCall(call); why != "" {
			w.fail(x, "work before Parse: %s (%s)", w.ix.src(x), why)
		}
		return false
	}
	w.fail(s, "work before Parse: %s", w.ix.src(s))
	return false
}

// assign admits a FlagSet, a flag definition, fs.Usage, a split of the
// help input by a proven-pure splitter, and locals bound to literals or
// values of the help input. Any write to a name the walk did not declare
// (package state such as rpcOrigin) is pre-Parse work.
func (w *hwalker) assign(s ast.Stmt) {
	as, ok := s.(*ast.AssignStmt)
	if !ok {
		w.fail(s, "work before Parse: %s", w.ix.src(s))
	}
	if len(as.Lhs) == 1 && len(as.Rhs) == 1 {
		if call, ok := as.Rhs[0].(*ast.CallExpr); ok {
			switch n := hpCallee(call.Fun); {
			case w.fs == "" && (n == "newFlagSet" || n == "flag.NewFlagSet"):
				id, ok := as.Lhs[0].(*ast.Ident)
				if !ok {
					w.fail(as, "FlagSet is not a local")
				}
				for _, a := range call.Args {
					if why := w.ix.impure(a); why != "" {
						w.fail(as, "FlagSet argument %s", why)
					}
				}
				w.fs, w.fsFn = id.Name, w.fn
				return
			case w.fs != "":
				if se, ok := call.Fun.(*ast.SelectorExpr); ok && hpIsIdent(se.X, w.fs) || helpFlagHelpers[n] {
					if why := w.flagCall(call); why != "" {
						w.fail(as, "work before Parse: %s (%s)", w.ix.src(as), why)
					}
					return
				}
			}
		}
		if se, ok := as.Lhs[0].(*ast.SelectorExpr); ok && w.fs != "" && hpIsIdent(se.X, w.fs) && se.Sel.Name == "Usage" {
			// fs.Usage runs on -h: it may only print.
			ast.Inspect(as.Rhs[0], func(n ast.Node) bool {
				if c, ok := n.(*ast.CallExpr); ok {
					fn := hpCallee(c.Fun)
					if !strings.HasPrefix(fn, "fmt.Fprint") && fn != w.fs+".PrintDefaults" && fn != w.fs+".Output" && !strings.HasPrefix(fn, "strings.") {
						w.fail(c, "%s.Usage calls %s", w.fs, fn)
					}
				}
				return true
			})
			return
		}
	}
	if len(as.Lhs) == 2 && len(as.Rhs) == 1 && as.Tok == token.DEFINE {
		if call, ok := as.Rhs[0].(*ast.CallExpr); ok {
			if split, ok := w.ix.rt.splitters[hpCallee(call.Fun)]; ok && len(call.Args) == 1 {
				w.proveHelper(call, hpCallee(call.Fun))
				v := w.eval(call.Args[0])
				in, ok := v.v.([]string)
				if !v.known || !ok {
					w.fail(as, "splits an unknown list")
				}
				a, b := split(in)
				w.bind(as.Lhs[0], hval{true, a})
				w.bind(as.Lhs[1], hval{true, b})
				return
			}
		}
	}
	if len(as.Lhs) > 1 && len(as.Rhs) == 1 && as.Tok == token.DEFINE {
		// home, _ := os.UserHomeDir(): a pure call's results, unknown.
		if why := w.ix.impure(as.Rhs[0]); why != "" {
			w.fail(as, "work before Parse: %s (%s)", w.ix.src(as), why)
		}
		for _, l := range as.Lhs {
			w.bind(l, hval{})
		}
		return
	}
	if len(as.Lhs) != len(as.Rhs) {
		w.fail(as, "work before Parse: %s", w.ix.src(as))
	}
	vals := make([]hval, len(as.Rhs))
	for i, r := range as.Rhs {
		if why := w.ix.impure(r); why != "" {
			w.fail(as, "work before Parse: %s (%s)", w.ix.src(as), why)
		}
		vals[i] = w.eval(r)
	}
	for i, l := range as.Lhs {
		id, ok := l.(*ast.Ident)
		if !ok {
			w.fail(as, "writes %s before Parse", w.ix.src(l))
		}
		if as.Tok != token.DEFINE {
			if _, local := w.env[id.Name]; !local && id.Name != "_" {
				w.fail(as, "writes package state before Parse: %s", w.ix.src(as))
			}
		}
		w.bind(id, vals[i])
	}
}

func (w *hwalker) bind(l ast.Expr, v hval) {
	if id, ok := l.(*ast.Ident); ok && id.Name != "_" {
		w.env[id.Name] = v
	}
}

func (w *hwalker) proveHelper(call *ast.CallExpr, name string) {
	if why := w.ix.pureHelper(name, map[string]bool{}); why != "" {
		w.fail(call, "%s is not proven pure: %s", name, why)
	}
}

// forwardTarget is the package function a `return F(...)` hands the help
// input on to, when one argument is a known argument list.
func (w *hwalker) forwardTarget(call *ast.CallExpr) *ast.FuncDecl {
	id, ok := call.Fun.(*ast.Ident)
	if !ok {
		return nil
	}
	callee := w.ix.funcs[id.Name]
	if callee == nil {
		return nil
	}
	for _, a := range call.Args {
		if v := w.eval(a); v.known {
			if _, ok := v.v.([]string); ok {
				return callee
			}
		}
	}
	return nil
}

func (w *hwalker) forward(call *ast.CallExpr, callee *ast.FuncDecl) {
	env := map[string]hval{}
	for i, a := range call.Args {
		v := w.eval(a)
		if !v.known {
			if id, ok := a.(*ast.Ident); !ok || !w.isParam(id.Name) {
				if why := w.ix.impure(a); why != "" {
					w.fail(call, "hands on %s (%s)", w.ix.src(a), why)
				}
			}
		}
		if name := hpParamName(callee, i); name != "" {
			env[name] = v
		}
	}
	w.enter(callee, env)
}

func (w *hwalker) isParam(name string) bool {
	for _, f := range w.fn.Type.Params.List {
		for _, id := range f.Names {
			if id.Name == name {
				return true
			}
		}
	}
	return false
}

// flagCall admits fs.<Define>(...), fs.SetOutput and the flag helpers.
func (w *hwalker) flagCall(call *ast.CallExpr) string {
	if se, ok := call.Fun.(*ast.SelectorExpr); ok && hpIsIdent(se.X, w.fs) {
		if se.Sel.Name == "SetOutput" {
			return ""
		}
		i, ok := hpFlagDefArg[se.Sel.Name]
		if !ok {
			return w.fs + "." + se.Sel.Name + " is not a flag definition"
		}
		if len(call.Args) <= i || hpStringLit(call.Args[i]) == "" {
			return "flag name is not a string literal"
		}
		if n := hpStringLit(call.Args[i]); n == "h" || n == "help" {
			return "defines -" + n + ", so -h would not reach flag.ErrHelp"
		}
		for j, a := range call.Args {
			if why := w.ix.impure(a); why != "" && j != i {
				return why
			}
		}
		return ""
	}
	fn := hpCallee(call.Fun)
	if !helpFlagHelpers[fn] {
		return "calls " + fn
	}
	for _, a := range call.Args {
		if !hpIsIdent(a, w.fs) {
			if why := w.ix.impure(a); why != "" {
				return why
			}
		}
	}
	return ""
}

var hpFlagDefArg = map[string]int{ // FlagSet method -> index of the flag name
	"String": 0, "Bool": 0, "Int": 0, "Int64": 0, "Uint": 0, "Uint64": 0, "Float64": 0, "Duration": 0,
	"StringVar": 1, "BoolVar": 1, "IntVar": 1, "Int64Var": 1, "UintVar": 1, "Uint64Var": 1, "Float64Var": 1, "DurationVar": 1,
	"Var": 1, "TextVar": 1, "Func": 0, "BoolFunc": 0,
}

// hpFlagType is the helpFlag.Type a FlagSet method defines: the method
// without its Var suffix, lower-cased (Var and TextVar stay themselves).
func hpFlagType(method string) string {
	switch method {
	case "Var", "TextVar", "Func", "BoolFunc":
		return strings.ToLower(method)
	}
	return strings.ToLower(strings.TrimSuffix(method, "Var"))
}

// eval evaluates e over the walk's bindings; unknown when it cannot.
func (w *hwalker) eval(e ast.Expr) hval {
	switch x := e.(type) {
	case *ast.ParenExpr:
		return w.eval(x.X)
	case *ast.BasicLit:
		switch x.Kind {
		case token.STRING:
			s, err := strconv.Unquote(x.Value)
			return hval{err == nil, s}
		case token.INT:
			n, err := strconv.Atoi(x.Value)
			return hval{err == nil, n}
		}
	case *ast.Ident:
		switch x.Name {
		case "true":
			return hval{true, true}
		case "false":
			return hval{true, false}
		}
		if v, ok := w.env[x.Name]; ok {
			return v
		}
	case *ast.IndexExpr:
		l, ok := w.eval(x.X).v.([]string)
		i, ok2 := w.eval(x.Index).v.(int)
		if ok && ok2 {
			if i < 0 || i >= len(l) {
				w.fail(x, "indexes past the help input: %s", w.ix.src(x))
			}
			return hval{true, l[i]}
		}
	case *ast.SliceExpr:
		v := w.eval(x.X)
		l, ok := v.v.([]string)
		if !v.known || !ok || x.Slice3 {
			break
		}
		lo, hi := 0, len(l)
		if x.Low != nil {
			n, ok := w.eval(x.Low).v.(int)
			if !ok {
				break
			}
			lo = n
		}
		if x.High != nil {
			n, ok := w.eval(x.High).v.(int)
			if !ok {
				break
			}
			hi = n
		}
		if lo < 0 || hi > len(l) || lo > hi {
			w.fail(x, "slices past the help input: %s", w.ix.src(x))
		}
		return hval{true, append([]string(nil), l[lo:hi]...)}
	case *ast.UnaryExpr:
		if x.Op == token.NOT {
			if b, ok := w.eval(x.X).v.(bool); ok {
				return hval{true, !b}
			}
		}
	case *ast.BinaryExpr:
		if x.Op == token.LAND || x.Op == token.LOR {
			a, ok := w.eval(x.X).v.(bool)
			if !ok {
				break
			}
			if (x.Op == token.LAND && !a) || (x.Op == token.LOR && a) {
				return hval{true, a}
			}
			b, ok := w.eval(x.Y).v.(bool)
			return hval{ok, b}
		}
		a, b := w.eval(x.X), w.eval(x.Y)
		if !a.known || !b.known {
			break
		}
		switch x.Op {
		case token.EQL:
			return hval{true, a.v == b.v}
		case token.NEQ:
			return hval{true, a.v != b.v}
		}
		ai, ok1 := a.v.(int)
		bi, ok2 := b.v.(int)
		if ok1 && ok2 {
			switch x.Op {
			case token.LSS:
				return hval{true, ai < bi}
			case token.GTR:
				return hval{true, ai > bi}
			case token.LEQ:
				return hval{true, ai <= bi}
			case token.GEQ:
				return hval{true, ai >= bi}
			}
		}
	case *ast.CallExpr:
		return w.evalCall(x)
	}
	return hval{}
}

func (w *hwalker) evalCall(x *ast.CallExpr) hval {
	n := hpCallee(x.Fun)
	args := make([]hval, len(x.Args))
	for i, a := range x.Args {
		args[i] = w.eval(a)
	}
	switch n {
	case "len":
		if l, ok := args[0].v.([]string); ok {
			return hval{true, len(l)}
		}
		if s, ok := args[0].v.(string); ok {
			return hval{true, len(s)}
		}
	case "strings.HasPrefix", "strings.HasSuffix":
		a, ok1 := args[0].v.(string)
		b, ok2 := args[1].v.(string)
		if ok1 && ok2 {
			if n == "strings.HasPrefix" {
				return hval{true, strings.HasPrefix(a, b)}
			}
			return hval{true, strings.HasSuffix(a, b)}
		}
	}
	if pred, ok := w.ix.rt.preds[n]; ok && len(args) > 0 {
		in, ok := args[0].v.([]string)
		if !ok {
			return hval{}
		}
		var lits []string
		for _, a := range args[1:] {
			s, ok := a.v.(string)
			if !ok {
				return hval{}
			}
			lits = append(lits, s)
		}
		w.proveHelper(x, n)
		return hval{true, pred(in, lits)}
	}
	if f, ok := w.ix.rt.parseArgs[n]; ok && len(args) == 2 && hpIsIdent(x.Args[0], w.fs) {
		in, ok := args[1].v.([]string)
		if !ok {
			return hval{}
		}
		w.proveHelper(x, n)
		return hval{true, f(in)}
	}
	return hval{}
}

// flagSetIn finds the FlagSet a function creates among its top-level
// statements (a FlagSet inside one case of a switch belongs to that case).
func (ix *helpProofIndex) flagSetIn(fd *ast.FuncDecl) (*ast.FuncDecl, string) {
	if fd == nil {
		return nil, ""
	}
	for _, s := range fd.Body.List {
		as, ok := s.(*ast.AssignStmt)
		if !ok || len(as.Lhs) != 1 || len(as.Rhs) != 1 {
			continue
		}
		call, ok := as.Rhs[0].(*ast.CallExpr)
		if !ok {
			continue
		}
		if c := hpCallee(call.Fun); c == "newFlagSet" || c == "flag.NewFlagSet" {
			if id, ok := as.Lhs[0].(*ast.Ident); ok {
				return fd, id.Name
			}
		}
	}
	return nil, ""
}

// flagDefs reads every flag fd defines on fs, through the flag helpers.
func (ix *helpProofIndex) flagDefs(fd *ast.FuncDecl, fs string) []helpASTFlag {
	var out []helpASTFlag
	var visit func(body ast.Node, fs string)
	visit = func(body ast.Node, fs string) {
		ast.Inspect(body, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok {
				return true
			}
			if se, ok := call.Fun.(*ast.SelectorExpr); ok && hpIsIdent(se.X, fs) {
				i, ok := hpFlagDefArg[se.Sel.Name]
				if !ok || len(call.Args) <= i {
					return true
				}
				f := helpASTFlag{Name: hpStringLit(call.Args[i]), Type: hpFlagType(se.Sel.Name)}
				if f.Name == "" {
					return true
				}
				if se.Sel.Name != "Func" && se.Sel.Name != "BoolFunc" {
					f.Usage, f.UsageKnown = hpStringLitOK(call.Args[len(call.Args)-1])
					if len(call.Args) == i+3 {
						f.Default, f.DefaultKnown = hpLiteralDefault(call.Args[i+1])
					}
				} else {
					f.Usage, f.UsageKnown = hpStringLitOK(call.Args[1])
					f.DefaultKnown = true
				}
				out = append(out, f)
				return true
			}
			if fn := hpCallee(call.Fun); helpFlagHelpers[fn] && ix.funcs[fn] != nil {
				for j, a := range call.Args {
					if hpIsIdent(a, fs) {
						visit(ix.funcs[fn].Body, hpParamName(ix.funcs[fn], j))
					}
				}
			}
			return true
		})
	}
	visit(fd.Body, fs)
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

// hpLiteralDefault renders a literal default the way help prints it: "" for
// a zero value.
func hpLiteralDefault(e ast.Expr) (string, bool) {
	if s, ok := hpStringLitOK(e); ok {
		return s, true
	}
	switch x := e.(type) {
	case *ast.Ident:
		switch x.Name {
		case "false":
			return "", true
		case "true":
			return "true", true
		}
	case *ast.BasicLit:
		if x.Kind == token.INT || x.Kind == token.FLOAT {
			if x.Value == "0" {
				return "", true
			}
			return x.Value, true
		}
	}
	return "", false
}

// subverbLiterals reads the subverbs a verb's top handler dispatches: the
// string cases of a switch on args[0] (or a name bound to it) and every
// `args[0] == "lit"` comparison.
func (ix *helpProofIndex) subverbLiterals(verb string) []string {
	fd := ix.methods[ix.runners[verb]+".execVerb"]
	// Follow single-statement wrappers.
	for fd != nil && len(fd.Body.List) == 1 {
		rs, ok := fd.Body.List[0].(*ast.ReturnStmt)
		if !ok || len(rs.Results) != 1 {
			break
		}
		call, ok := rs.Results[0].(*ast.CallExpr)
		if !ok {
			break
		}
		id, ok := call.Fun.(*ast.Ident)
		if !ok || ix.funcs[id.Name] == nil {
			break
		}
		fd = ix.funcs[id.Name]
	}
	if fd == nil {
		return nil
	}
	args := hpParamName(fd, 0)
	alias := map[string]bool{}
	isArg0 := func(e ast.Expr) bool {
		if id, ok := e.(*ast.Ident); ok && alias[id.Name] {
			return true
		}
		ie, ok := e.(*ast.IndexExpr)
		return ok && hpIsIdent(ie.X, args) && hpIsIntLit(ie.Index, "0")
	}
	set := map[string]bool{}
	ast.Inspect(fd.Body, func(n ast.Node) bool {
		switch x := n.(type) {
		case *ast.FuncLit:
			return false
		case *ast.AssignStmt:
			for i, r := range x.Rhs {
				if isArg0(r) && i < len(x.Lhs) {
					if id, ok := x.Lhs[i].(*ast.Ident); ok {
						alias[id.Name] = true
					}
				}
			}
		case *ast.SwitchStmt:
			if x.Tag != nil && isArg0(x.Tag) {
				for _, c := range x.Body.List {
					for _, e := range c.(*ast.CaseClause).List {
						if s := hpStringLit(e); s != "" {
							set[s] = true
						}
					}
				}
			}
		case *ast.BinaryExpr:
			if x.Op == token.EQL && isArg0(x.X) {
				if s := hpStringLit(x.Y); s != "" {
					set[s] = true
				}
			}
		}
		return true
	})
	var out []string
	for s := range set {
		out = append(out, s)
	}
	sort.Strings(out)
	return out
}

func hpIsIntLit(e ast.Expr, v string) bool {
	bl, ok := e.(*ast.BasicLit)
	return ok && bl.Kind == token.INT && bl.Value == v
}

func helpKey(verb, sub string) string {
	if sub == "" {
		return verb
	}
	return verb + " " + sub
}

// --- the tests ---------------------------------------------------------

// helpProofRuntime wires the proof to the package's real argument helpers.
func helpProofRuntime() helpRuntime {
	return helpRuntime{
		preds: map[string]func([]string, []string) bool{
			"hasFlag":     func(a, l []string) bool { return len(l) == 1 && hasFlag(a, l[0]) },
			"hasJSONFlag": func(a, _ []string) bool { return hasJSONFlag(a) },
		},
		splitters: map[string]func([]string) ([]string, []string){
			"splitLeadIDs":         splitLeadIDs,
			"splitLeadingOperands": splitLeadingOperands,
		},
		parseArgs: map[string]func([]string) []string{
			"reorderFlagsFirst": func(a []string) []string {
				return reorderFlagsFirst(flag.NewFlagSet("help-proof", flag.ContinueOnError), a)
			},
		},
	}
}

// loadHelpIndex reads this package's source; call it before any chdir.
func loadHelpIndex(t *testing.T) *helpProofIndex {
	t.Helper()
	ix, err := loadHelpProofIndex(".", helpProofRuntime())
	if err != nil {
		t.Fatal(err)
	}
	return ix
}

type helpTarget struct{ verb, sub string }

// helpKeys is every verb and every subverb help answers for.
func helpKeys() []helpTarget {
	var out []helpTarget
	subs := verbSubverbs()
	for _, v := range verbs() {
		out = append(out, helpTarget{v.Name, ""})
		for _, s := range subs[v.Name] {
			out = append(out, helpTarget{v.Name, s})
		}
	}
	return out
}

// helpKindOf is how the runtime tables say help answers key.
func helpKindOf(key string) helpKind {
	if _, ok := verbHelpHeaderOnly()[key]; ok {
		return helpHeaderOnly
	}
	if _, ok := verbHelpStatic()[key]; ok {
		return helpStatic
	}
	return helpDelegated
}

// TestVerbHelpParseFirstProof (CL-6) holds delegation to proof: help runs a
// handler with -h only when the proof reads it parse-first for that exact
// input, answers from a static list exactly where pre-Parse work rules
// delegation out, and prints the header alone exactly where no FlagSet is
// on the help path. A new verb is delegated by default, so one that does
// work before Parse fails here until it is fixed or listed.
func TestVerbHelpParseFirstProof(t *testing.T) {
	ix := loadHelpIndex(t)
	seen := map[string]bool{}
	counts := map[helpKind]int{}
	for _, k := range helpKeys() {
		p := ix.proveHelp(k.verb, k.sub)
		key := p.Key()
		seen[key] = true
		counts[p.Kind]++
		if got := helpKindOf(key); got != p.Kind {
			t.Errorf("%s: help answers it %s, the proof reads %s (chain %s): %s",
				key, got, p.Kind, strings.Join(p.Chain, " > "), strings.Join(p.Problems, "; "))
		}
	}
	for key := range verbHelpHeaderOnly() {
		if !seen[key] {
			t.Errorf("verbHelpHeaderOnly lists %q, which is no verb or subverb", key)
		}
	}
	for key := range verbHelpStatic() {
		if !seen[key] {
			t.Errorf("verbHelpStatic lists %q, which is no verb or subverb", key)
		}
	}
	if counts[helpDelegated] < 100 {
		t.Errorf("only %d keys delegated; the proof reads too little", counts[helpDelegated])
	}
	t.Logf("help: %d delegated, %d static, %d header-only", counts[helpDelegated], counts[helpStatic], counts[helpHeaderOnly])
}

// TestVerbSubverbsMatchDispatch pins verbSubverbs to the dispatch it
// describes: every subverb a verb's handler routes on args[0] is listed,
// and nothing else.
func TestVerbSubverbsMatchDispatch(t *testing.T) {
	ix := loadHelpIndex(t)
	subs := verbSubverbs()
	for _, v := range verbs() {
		got := append([]string(nil), subs[v.Name]...)
		sort.Strings(got)
		want := ix.subverbLiterals(v.Name)
		if !reflect.DeepEqual(got, want) && !(len(got) == 0 && len(want) == 0) {
			t.Errorf("%s: verbSubverbs %v, the handler dispatches %v", v.Name, got, want)
		}
	}
}

// staticDrift lists how a static help list differs from the flags the
// handler defines: names, types, and usage and default wherever the source
// spells them as literals.
func staticDrift(key string, flags []helpFlag, p helpProof) []string {
	var out []string
	ast := map[string]helpASTFlag{}
	for _, f := range p.Flags {
		ast[f.Name] = f
	}
	var got []string
	for _, f := range flags {
		got = append(got, f.Name)
		a, ok := ast[f.Name]
		if !ok {
			continue
		}
		if a.Type != f.Type {
			out = append(out, fmt.Sprintf("%s -%s: static type %q, the handler defines %q", key, f.Name, f.Type, a.Type))
		}
		if a.UsageKnown && a.Usage != f.Usage {
			out = append(out, fmt.Sprintf("%s -%s: static usage %q, the handler says %q", key, f.Name, f.Usage, a.Usage))
		}
		if a.DefaultKnown && a.Default != f.Default {
			out = append(out, fmt.Sprintf("%s -%s: static default %q, the handler says %q", key, f.Name, f.Default, a.Default))
		}
	}
	sort.Strings(got)
	if want := p.flagNames(); !reflect.DeepEqual(got, want) {
		out = append(out, fmt.Sprintf("%s: static flags %v, the handler defines %v", key, got, want))
	}
	return out
}

// TestVerbHelpStaticListsPinned (CL-7, CL-11) compares every static help
// list to the flags its handler defines: the same names, the same types,
// and the same usage and default wherever the source spells them as
// literals.
func TestVerbHelpStaticListsPinned(t *testing.T) {
	ix := loadHelpIndex(t)
	for key, flags := range verbHelpStatic() {
		verb, sub, _ := strings.Cut(key, " ")
		for _, d := range staticDrift(key, flags, ix.proveHelp(verb, sub)) {
			t.Error(d)
		}
	}
}

// TestStaticPinRejectsTypeDrift (CL-11) proves the type pin bites: a static
// list whose type differs from the handler's definition, read directly or
// through a flag helper, is reported.
func TestStaticPinRejectsTypeDrift(t *testing.T) {
	dir := t.TempDir()
	src := `package main

import (
	"flag"
	"io"
	"time"
)

func declareVerb(v verb) {}

type verb struct {
	Name string
	Run  any
}

func newFlagSet(name string, w io.Writer) *flag.FlagSet { return flag.NewFlagSet(name, flag.ContinueOnError) }

func addHarnessFlag(fs *flag.FlagSet) *string { return fs.String("harness", "", "h") }

type typedR struct{}

func (typedR) execVerb(args []string, _ io.Reader, stdout, stderr io.Writer) int {
	return cmdTyped(args, stdout, stderr)
}

func cmdTyped(args []string, stdout, stderr io.Writer) int {
	fs := newFlagSet("typed", stderr)
	fs.Int("n", 0, "n")
	fs.Duration("wait", time.Second, "w")
	addHarnessFlag(fs)
	if err := fs.Parse(args); err != nil {
		return 1
	}
	return 0
}

func init() { declareVerb(verb{Name: "typed", Run: typedR{}}) }
`
	if err := os.WriteFile(filepath.Join(dir, "fixture.go"), []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	ix, err := loadHelpProofIndex(dir, helpProofRuntime())
	if err != nil {
		t.Fatal(err)
	}
	p := ix.proveHelp("typed", "")
	right := []helpFlag{{"harness", "string", "", "h"}, {"n", "int", "", "n"}, {"wait", "duration", "1s", "w"}}
	if d := staticDrift("typed", right, p); len(d) != 0 {
		t.Fatalf("a correctly typed list drifted: %v", d)
	}
	for _, c := range []struct{ name, typ string }{{"harness", "bool"}, {"n", "string"}, {"wait", "int"}} {
		bad := append([]helpFlag(nil), right...)
		for i := range bad {
			if bad[i].Name == c.name {
				bad[i].Type = c.typ
			}
		}
		d := staticDrift("typed", bad, p)
		if len(d) != 1 || !strings.Contains(d[0], "static type") || !strings.Contains(d[0], "-"+c.name) {
			t.Errorf("-%s typed %s: drift %v, want one type report", c.name, c.typ, d)
		}
	}
}

// TestHelpProofRejectsPreParseWork runs the proof over a fixture package so
// it cannot pass vacuously: each kind of pre-Parse work is caught.
func TestHelpProofRejectsPreParseWork(t *testing.T) {
	dir := t.TempDir()
	src := `package main

import (
	"flag"
	"io"
	"strings"
)

var rpcOrigin string

func declareVerb(v verb) {}

type verb struct {
	Name string
	Run  any
}

func newFlagSet(name string, w io.Writer) *flag.FlagSet { return flag.NewFlagSet(name, flag.ContinueOnError) }

func killed() bool { return false }

type okR struct{}

func (okR) execVerb(args []string, _ io.Reader, stdout, stderr io.Writer) int { return cmdOK(args, stdout, stderr) }

func cmdOK(args []string, stdout, stderr io.Writer) int {
	if len(args) > 0 && args[0] == "sub" {
		return cmdSub(args[1:], stdout, stderr)
	}
	if len(args) > 0 && !strings.HasPrefix(args[0], "-") {
		args = args[1:]
	}
	fs := newFlagSet("ok", stderr)
	fs.String("name", "x", "usage")
	if err := fs.Parse(args); err != nil {
		return 1
	}
	rpcOrigin = "ok"
	return 0
}

func cmdSub(args []string, stdout, stderr io.Writer) int {
	fs := newFlagSet("sub", stderr)
	fs.Bool("deep", false, "deep")
	if err := fs.Parse(args); err != nil {
		return 1
	}
	return 0
}

type originR struct{}

func (originR) execVerb(args []string, _ io.Reader, stdout, stderr io.Writer) int {
	rpcOrigin = "x"
	fs := newFlagSet("origin", stderr)
	fs.Bool("a", false, "a")
	if err := fs.Parse(args); err != nil {
		return 1
	}
	return 0
}

type killR struct{}

func (killR) execVerb(args []string, _ io.Reader, stdout, stderr io.Writer) int {
	if killed() {
		return 0
	}
	fs := newFlagSet("kill", stderr)
	fs.Bool("b", false, "b")
	if err := fs.Parse(args); err != nil {
		return 1
	}
	return 0
}

type peelR struct{}

func (peelR) execVerb(args []string, _ io.Reader, stdout, stderr io.Writer) int {
	fs := newFlagSet("peel", stderr)
	fs.Bool("c", false, "c")
	if err := fs.Parse(args[1:]); err != nil {
		return 1
	}
	return 0
}

type noisyR struct{}

func (noisyR) execVerb(args []string, _ io.Reader, stdout, stderr io.Writer) int {
	fs := newFlagSet("noisy", stderr)
	fs.Bool("d", false, "d")
	if err := fs.Parse(args); err != nil {
		io.WriteString(stderr, "error")
		return 1
	}
	return 0
}

type bareR struct{}

func (bareR) execVerb(args []string, _ io.Reader, stdout, stderr io.Writer) int {
	io.WriteString(stdout, "v1")
	return 0
}

func init() {
	declareVerb(verb{Name: "ok", Run: okR{}})
	declareVerb(verb{Name: "origin", Run: originR{}})
	declareVerb(verb{Name: "kill", Run: killR{}})
	declareVerb(verb{Name: "peel", Run: peelR{}})
	declareVerb(verb{Name: "noisy", Run: noisyR{}})
	declareVerb(verb{Name: "bare", Run: bareR{}})
}
`
	if err := os.WriteFile(filepath.Join(dir, "fixture.go"), []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	ix, err := loadHelpProofIndex(dir, helpProofRuntime())
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range []struct {
		verb, sub string
		want      helpKind
		flags     []string
		problem   string
	}{
		{"ok", "", helpDelegated, []string{"name"}, ""},
		{"ok", "sub", helpDelegated, []string{"deep"}, ""},
		{"origin", "", helpStatic, []string{"a"}, "writes package state"},
		{"kill", "", helpStatic, []string{"b"}, "does not decide"},
		{"peel", "", helpStatic, []string{"c"}, "not the help token"},
		{"noisy", "", helpStatic, []string{"d"}, "more than return"},
		{"bare", "", helpHeaderOnly, nil, "work before the FlagSet"},
	} {
		p := ix.proveHelp(c.verb, c.sub)
		if p.Kind != c.want || !reflect.DeepEqual(p.flagNames(), c.flags) {
			t.Errorf("%s: kind %s flags %v, want %s %v (%v)", p.Key(), p.Kind, p.flagNames(), c.want, c.flags, p.Problems)
		}
		if c.problem != "" && !strings.Contains(strings.Join(p.Problems, ";"), c.problem) {
			t.Errorf("%s: problems %v, want one naming %q", p.Key(), p.Problems, c.problem)
		}
	}
	if got := ix.subverbLiterals("ok"); !reflect.DeepEqual(got, []string{"sub"}) {
		t.Errorf("subverbLiterals(ok) = %v", got)
	}
}
